package api

// The system page: what runs, where its data lives, how much there is, and
// what needs attention (mail queue, failing webhooks). Never a secret.

import (
	"context"
	"database/sql"
	"io/fs"
	"net/http"
	"path/filepath"
	"runtime"
	"time"

	"solstice/internal/config"
	"solstice/internal/mail"
	"solstice/internal/sqlq"
)

// dataDirSize measures the data directory, at most every 5 minutes.
func (s *Server) dataDirSize() (int64, string) {
	s.adm.sizeMu.Lock()
	defer s.adm.sizeMu.Unlock()
	if !s.adm.sizeAt.IsZero() && time.Since(s.adm.sizeAt) < 5*time.Minute {
		return s.adm.dataSize, s.adm.sizeErr
	}
	var total int64
	err := filepath.WalkDir(s.dataDir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			if info, err := d.Info(); err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	s.adm.dataSize, s.adm.sizeErr, s.adm.sizeAt = total, "", time.Now()
	if err != nil {
		s.adm.sizeErr = err.Error()
	}
	return s.adm.dataSize, s.adm.sizeErr
}

func (s *Server) systemInfo(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	ctx := r.Context()
	info := s.configInfo()
	version := info.Version
	if version == "" {
		version = "dev"
	}
	out := map[string]any{
		"version": version, "goVersion": runtime.Version(), "startedAt": s.adm.started.UTC(),
		"uptimeSeconds": int64(time.Since(s.adm.started).Seconds()),
	}

	server, _ := serverVersion(ctx, s.db)
	var name string
	var size sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `SELECT DATABASE(), (SELECT SUM(data_length + index_length)
		FROM information_schema.tables WHERE table_schema = DATABASE())`).Scan(&name, &size); err != nil {
		return err
	}
	out["database"] = map[string]any{"server": server, "name": name, "sizeBytes": size.Int64}

	dataSize, sizeErr := s.dataDirSize()
	dataDir := map[string]any{"path": s.dataDir, "sizeBytes": dataSize, "error": nil}
	if sizeErr != "" {
		dataDir["sizeBytes"], dataDir["error"] = nil, sizeErr
	}
	out["dataDir"] = dataDir

	var c struct {
		usersActive, usersInactive, projects, archived, issues, comments, attachments int64
		attachmentBytes                                                               sql.NullInt64
	}
	if err := s.db.QueryRowContext(ctx, `SELECT
		(SELECT COUNT(*) FROM users WHERE active), (SELECT COUNT(*) FROM users WHERE NOT active),
		(SELECT COUNT(*) FROM projects WHERE NOT archived), (SELECT COUNT(*) FROM projects WHERE archived),
		(SELECT COUNT(*) FROM issues), (SELECT COUNT(*) FROM comments),
		(SELECT COUNT(*) FROM attachments), (SELECT SUM(size) FROM attachments)`).Scan(&c.usersActive, &c.usersInactive,
		&c.projects, &c.archived, &c.issues, &c.comments, &c.attachments, &c.attachmentBytes); err != nil {
		return err
	}
	out["counts"] = map[string]any{"usersActive": c.usersActive, "usersInactive": c.usersInactive,
		"projects": c.projects, "projectsArchived": c.archived, "issues": c.issues, "comments": c.comments,
		"attachments": c.attachments, "attachmentBytes": c.attachmentBytes.Int64}

	mailOut := map[string]any{}
	cfg, err := mail.LoadConfig(ctx, s.out.settings, s.secrets)
	mailOut["enabled"] = err == nil && cfg.Enabled
	if st, err := s.out.outbox.Stats(ctx); err == nil {
		mailOut["pending"], mailOut["failed"] = st.Pending, st.Failed
		mailOut["sentLastDay"], mailOut["oldestPending"] = st.SentLastDay, st.OldestPending
	} else {
		return err
	}
	out["mail"] = mailOut

	hooks, err := s.failingWebhooks(ctx)
	if err != nil {
		return err
	}
	out["webhooks"] = hooks

	type migration struct {
		Version   string    `json:"version"`
		AppliedAt time.Time `json:"appliedAt"`
	}
	migrations := []migration{}
	if err := sqlq.Each(ctx, s.db, `SELECT version, applied_at FROM schema_migrations ORDER BY version`,
		func(rows *sql.Rows) error {
			var m migration
			if err := rows.Scan(&m.Version, &m.AppliedAt); err != nil {
				return err
			}
			migrations = append(migrations, m)
			return nil
		}); err != nil {
		return err
	}
	latest := ""
	if len(migrations) > 0 {
		latest = migrations[len(migrations)-1].Version
	}
	out["migrations"] = map[string]any{"count": len(migrations), "latest": latest, "list": migrations}

	sources := map[string]string{}
	for _, k := range config.Keys {
		sources[k] = sourceDefault
		if src := info.Sources[k]; src != "" {
			sources[k] = src
		}
	}
	g := s.general(ctx)
	if g.Sources["baseURL"] == sourceDatabase {
		sources[config.KeyBaseURL] = sourceDatabase
	}
	if g.Sources["timeZone"] == sourceDatabase {
		sources[config.KeyTZ] = sourceDatabase
	}
	out["config"] = map[string]any{"path": info.Path, "found": info.Found, "sources": sources, "addr": info.Addr}

	var imp any
	if a := s.currentImport(); a != nil {
		imp = a.job.view(locale(r))
	}
	out["import"] = imp
	return writeJSON(w, http.StatusOK, out)
}

// failingWebhooks counts the webhooks and lists those whose deliveries
// failed in the last day (given up, or being retried).
func (s *Server) failingWebhooks(ctx context.Context) (map[string]any, error) {
	type failing struct {
		ID          int64      `json:"id"`
		Name        string     `json:"name"`
		Failed      int        `json:"failed"`
		LastError   *string    `json:"lastError"`
		LastAttempt *time.Time `json:"lastAttempt"`
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM webhooks`).Scan(&total); err != nil {
		return nil, err
	}
	list := []failing{}
	since := time.Now().UTC().Add(-24 * time.Hour)
	err := sqlq.Each(ctx, s.db, `SELECT w.id, w.name, COUNT(*), MAX(d.attempted_at),
		(SELECT d2.error FROM webhook_deliveries d2 WHERE d2.webhook_id = w.id AND d2.error IS NOT NULL
			ORDER BY d2.id DESC LIMIT 1)
		FROM webhooks w JOIN webhook_deliveries d ON d.webhook_id = w.id
		WHERE d.created_at >= ? AND (d.status = 'failed' OR (d.status = 'pending' AND d.attempts > 0))
		GROUP BY w.id, w.name ORDER BY w.name`, func(rows *sql.Rows) error {
		var f failing
		var last sql.NullTime
		var lastErr sql.NullString
		if err := rows.Scan(&f.ID, &f.Name, &f.Failed, &last, &lastErr); err != nil {
			return err
		}
		if last.Valid {
			f.LastAttempt = &last.Time
		}
		f.LastError = nullStr(lastErr)
		list = append(list, f)
		return nil
	}, since)
	return map[string]any{"total": total, "failing": list}, err
}
