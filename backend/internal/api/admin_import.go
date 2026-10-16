package api

// Importing from Jira from the administration: the setup wizard's import,
// replacing Solstice's data (its own configuration is kept, see
// importer/keep.go). While it runs Solstice is in maintenance: requests
// wait outside, except the progress page of the administrator who started
// it, recognised by their session without the database.

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"solstice/internal/audit"
	"solstice/internal/auth"
	"solstice/internal/i18n"
	"solstice/internal/importer"
)

// adminImport is an import started from the administration.
type adminImport struct {
	job      *importJob
	database string     // the database name, typed to confirm
	operator string     // who started it
	keys     []string   // hashes of their credentials (session cookie, token)
	me       meResponse // their /api/me while the database is busy
	ip       string
}

// currentImport returns the running or last import.
func (s *Server) currentImport() *adminImport {
	s.adm.impMu.Lock()
	defer s.adm.impMu.Unlock()
	return s.adm.imp
}

// credentialKeys are the hashes of what a request signs in with.
func credentialKeys(r *http.Request) []string {
	var out []string
	if c, err := r.Cookie(auth.CookieName); err == nil && c.Value != "" {
		out = append(out, tokenKey(c.Value))
	}
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		out = append(out, tokenKey(strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))))
	}
	return out
}

func (imp *adminImport) isOperator(r *http.Request) bool {
	for _, k := range credentialKeys(r) {
		for _, mine := range imp.keys {
			if k == mine {
				return true
			}
		}
	}
	return false
}

// importRunning tells whether a replacing import is running. The background
// workers (e-mail, notifications, webhooks) skip their tick meanwhile.
func (s *Server) importRunning() bool {
	imp := s.currentImport()
	return imp != nil && imp.job.running()
}

// maintenance answers the requests held back while an import runs, and
// tells whether it did.
func (s *Server) maintenance(w http.ResponseWriter, r *http.Request) bool {
	imp := s.currentImport()
	if imp == nil || !imp.job.running() {
		return false
	}
	p := r.URL.Path
	if !strings.HasPrefix(p, "/api/") && !strings.HasPrefix(p, "/rest/") {
		return false
	}
	if r.Method == http.MethodGet && p == "/api/settings/public" {
		return false
	}
	if r.Method == http.MethodGet && imp.isOperator(r) {
		switch p {
		case "/api/admin/import":
			writeJSON(w, http.StatusOK, map[string]any{"confirm": imp.database, "job": imp.job.view(locale(r))})
			return true
		case "/api/me":
			writeJSON(w, http.StatusOK, imp.me)
			return true
		}
	}
	writeError(w, r, &httpError{status: http.StatusServiceUnavailable, extra: map[string]any{"maintenance": true},
		msg: i18n.M(i18n.ImportMaintenance)})
	return true
}

func (s *Server) databaseName(ctx context.Context) (string, error) {
	var name string
	err := s.db.QueryRowContext(ctx, `SELECT DATABASE()`).Scan(&name)
	return name, err
}

func (s *Server) adminImportState(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	name, err := s.databaseName(r.Context())
	if err != nil {
		return err
	}
	var job any
	if imp := s.currentImport(); imp != nil {
		job = imp.job.view(locale(r))
	}
	return writeJSON(w, http.StatusOK, map[string]any{"confirm": name, "job": job})
}

func (s *Server) adminImportTest(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	var f sourceForm
	if err := readJSON(r, &f); err != nil {
		return err
	}
	p, err := probeSource(r.Context(), &f, locale(r))
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, p)
}

// adminImportStart replaces Solstice's data with a Jira instance's, in the
// background.
func (s *Server) adminImportStart(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	var in struct {
		sourceForm
		Confirm string `json:"confirm"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	ctx := r.Context()
	name, err := s.databaseName(ctx)
	if err != nil {
		return err
	}
	if strings.TrimSpace(in.Confirm) != name {
		return failf(http.StatusBadRequest, i18n.ImportConfirmDatabase, name)
	}
	f := in.sourceForm
	if _, err := probeSource(ctx, &f, locale(r)); err != nil {
		return err
	}
	u := currentUser(r)
	loc, _ := time.LoadLocation(f.TZ)
	imp := &adminImport{job: &importJob{state: "running", started: time.Now()}, database: name,
		operator: u.Username, keys: credentialKeys(r), me: meOf(u), ip: clientIP(r, s.proxies)}
	s.adm.impMu.Lock()
	if s.adm.imp != nil && s.adm.imp.job.running() {
		s.adm.impMu.Unlock()
		return fail(http.StatusConflict, i18n.ImportRunning)
	}
	s.adm.imp = imp
	s.adm.impMu.Unlock()

	source := sourceLabel(f.URL)
	s.auditLog(r, audit.ImportStart, audit.Target{Type: "import", Label: source},
		map[string]any{"source": source, "jiraHome": f.JiraHome, "tz": f.TZ})
	s.log.Info("import from Jira started from the administration", "by", u.Username, "source", source)
	job := imp.job
	go func() {
		rep, err := importer.Run(context.Background(), s.db, importer.Options{
			SourceURL: f.URL, SourceTZ: loc, JiraHome: f.JiraHome, DataDir: s.dataDir, Replace: true,
			Operator: u.Username, Log: s.log,
			Progress: func(p importer.Progress) {
				job.mu.Lock()
				job.progress = p
				job.mu.Unlock()
			},
		})
		// Everything cached describes the data that was replaced.
		s.perms.Invalidate()
		s.forgetCatalogue()
		s.forgetGeneral()
		s.out.notify.Invalidate()

		job.mu.Lock()
		job.finished = time.Now()
		if rep != nil {
			job.progress.Steps, job.warnings = rep.Steps, rep.Warnings
		}
		action, details := audit.ImportDone, map[string]any{"source": source,
			"seconds": int(job.finished.Sub(job.started).Seconds())}
		if err != nil {
			job.state, job.err = "failed", err
			action, details["error"] = audit.ImportFailed, err.Error()
			s.log.Error("import from Jira failed", "err", err)
		} else {
			job.state = "done"
			s.log.Info("import from Jira done", "duration", job.finished.Sub(job.started).Round(time.Second))
		}
		job.mu.Unlock()
		// The operator's account id changed with the import.
		var uid int64
		s.db.QueryRow(`SELECT id FROM users WHERE username = ?`, u.Username).Scan(&uid)
		if err := audit.Record(context.Background(), s.db, audit.Entry{UserID: uid, Username: u.Username, IP: imp.ip,
			Action: action, Target: audit.Target{Type: "import", Label: source}, Details: details}); err != nil {
			s.log.Error("audit log write failed", "action", action, "err", err)
		}
	}()
	return writeJSON(w, http.StatusAccepted, job.view(locale(r)))
}

// sourceLabel names a Jira database without its credentials.
func sourceLabel(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Scheme + "://" + u.Host + u.Path
}
