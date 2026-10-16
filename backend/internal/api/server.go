// Package api is Solstice's HTTP API, consumed by the Svelte interface.
//
// Every route under /api answers JSON. Errors come back as
// {"error": "message"} with the message ready to show, in the language of
// the request (see locale).
package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/netip"
	"sync"
	"time"

	"solstice/internal/auth"
	"solstice/internal/i18n"
	"solstice/internal/issue"
	"solstice/internal/live"
	"solstice/internal/perm"
	"solstice/internal/sqlq"
)

// Server holds what handlers share.
type Server struct {
	db      *sql.DB
	perms   *perm.Store
	hub     *live.Hub
	dataDir string
	log     *slog.Logger
	tz      *time.Location // for dates typed without a time zone (JQL)
	limiter *auth.Limiter
	baseURL string         // public address (SOLSTICE_BASE_URL), "" when not set; see SetBaseURL
	proxies []netip.Prefix // reverse proxies to believe (SOLSTICE_TRUSTED_PROXIES); see SetTrustedProxies
	out     *outgoing      // event bus, e-mail and webhooks (notifications.go)

	// Set by New; tests replace them to run without a database or a real wait.
	verify func(ctx context.Context, username, password string) (*auth.User, error) // checks a password
	sleep  func(ctx context.Context, d time.Duration)                               // holds back a wrong answer

	catMu   sync.Mutex
	catalog *catalog
	catAt   time.Time

	acc accountMail // whether links can be sent by e-mail (account_links.go)

	adm adminState // instance settings, system page, imports (admin_*.go)
	upd updater    // release checks and the update helper (updates.go)

	streamsDone  chan struct{} // closed by CloseStreams
	closeStreams sync.Once
}

// New builds the API server. dataDir holds attachment files; tz is the time
// zone of dates typed in searches.
func New(db *sql.DB, dataDir string, tz *time.Location, log *slog.Logger) *Server {
	s := &Server{db: db, perms: perm.NewStore(db), hub: live.NewHub(), dataDir: dataDir, tz: tz, log: log,
		limiter: auth.NewLimiter(), streamsDone: make(chan struct{})}
	s.verify = func(ctx context.Context, username, password string) (*auth.User, error) {
		return auth.Authenticate(ctx, s.db, username, password)
	}
	s.sleep = func(ctx context.Context, d time.Duration) {
		t := time.NewTimer(d)
		defer t.Stop()
		select {
		case <-t.C:
		case <-ctx.Done():
		}
	}
	s.adm.started = time.Now()
	s.startOutgoing()
	return s
}

// CloseStreams ends the live-update streams. Called when the HTTP server shuts
// down, which does not cancel requests and would wait for them forever.
func (s *Server) CloseStreams() { s.closeStreams.Do(func() { close(s.streamsDone) }) }

// Handler returns the API routes, with static serving of the interface for
// everything outside /api.
func (s *Server) Handler(static http.Handler) http.Handler {
	mux := http.NewServeMux()
	route := func(pattern string, h handler, needUser bool) {
		mux.Handle(pattern, s.wrap(h, needUser))
	}

	route("POST /api/login", s.login, false)
	route("POST /api/logout", s.logout, false)

	// Forgotten passwords and invitations: links sent by e-mail.
	s.accountLinkRoutes(route)
	route("GET /api/me", s.me, true)
	route("PATCH /api/me", s.updateMe, true)
	route("GET /api/meta", s.meta, true)
	route("GET /api/users", s.searchUsers, true)
	route("GET /api/labels", s.labels, true)
	route("GET /api/sprints", s.sprints, true)
	route("GET /api/epics", s.epics, true)
	route("GET /api/fields", s.fields, true)
	route("GET /api/versions", s.projectParts("versions"), true)
	route("GET /api/components", s.projectParts("components"), true)
	route("GET /api/issues/facets", s.facets, true)
	route("GET /api/people/{username}", s.person, true)
	route("POST /api/refs", s.refs, true)
	route("GET /api/events", s.events, true)

	route("GET /api/projects", s.listProjects, true)
	route("GET /api/projects/{key}", s.getProject, true)
	route("GET /api/projects/{key}/assignable", s.assignableUsers, true)
	route("GET /api/projects/{key}/permissions", s.getPermissions, true)
	route("PUT /api/projects/{key}/permissions", s.putPermissions, true)
	route("GET /api/projects/{key}/workflows", s.listWorkflows, true)
	route("POST /api/projects/{key}/workflows", s.createWorkflow, true)
	route("PUT /api/workflows/{id}", s.updateWorkflow, true)
	route("DELETE /api/workflows/{id}", s.deleteWorkflow, true)
	route("POST /api/statuses", s.createStatus, true)
	route("PATCH /api/statuses/{id}", s.updateStatus, true)
	route("DELETE /api/statuses/{id}", s.deleteStatus, true)

	route("GET /api/issues", s.listIssues, true)
	route("POST /api/issues", s.createIssue, true)
	route("GET /api/issues/{key}", s.getIssue, true)
	route("PATCH /api/issues/{key}", s.updateIssue, true)
	route("DELETE /api/issues/{key}", s.deleteIssue, true)
	route("GET /api/issues/{key}/comments", s.listComments, true)
	route("POST /api/issues/{key}/comments", s.addComment, true)
	route("PATCH /api/comments/{id}", s.editComment, true)
	route("DELETE /api/comments/{id}", s.deleteComment, true)
	route("GET /api/issues/{key}/history", s.history, true)
	route("GET /api/issues/{key}/worklogs", s.listWorklogs, true)
	route("POST /api/issues/{key}/worklogs", s.addWorklog, true)
	route("PUT /api/issues/{key}/watch", s.setWatch(true), true)
	route("DELETE /api/issues/{key}/watch", s.setWatch(false), true)
	route("PUT /api/issues/{key}/vote", s.setVote(true), true)
	route("DELETE /api/issues/{key}/vote", s.setVote(false), true)
	route("POST /api/issues/{key}/links", s.addLink, true)
	route("DELETE /api/links/{id}", s.deleteLink, true)
	route("POST /api/issues/{key}/attachments", s.upload, true)
	route("GET /api/attachments/{id}", s.download, true)
	route("DELETE /api/attachments/{id}", s.deleteAttachment, true)

	route("GET /api/boards", s.listBoards, true)
	route("POST /api/boards", s.createBoard, true)
	route("GET /api/boards/{id}", s.getBoard, true)
	route("PUT /api/boards/{id}", s.updateBoard, true)
	route("DELETE /api/boards/{id}", s.deleteBoard, true)
	route("GET /api/boards/{id}/cards", s.boardCards, true)
	route("GET /api/boards/{id}/backlog", s.backlog, true)
	route("POST /api/boards/{id}/move", s.moveCard, true)
	route("POST /api/boards/{id}/sprints", s.createSprint, true)
	route("POST /api/sprints/{id}/start", s.startSprint, true)
	route("POST /api/sprints/{id}/complete", s.completeSprint, true)

	route("GET /api/filters", s.listFilters, true)
	route("POST /api/filters", s.createFilter, true)
	route("PATCH /api/filters/{id}", s.updateFilter, true)
	route("DELETE /api/filters/{id}", s.deleteFilter, true)
	route("PUT /api/filters/{id}/favourite", s.setFavourite(true), true)
	route("DELETE /api/filters/{id}/favourite", s.setFavourite(false), true)

	route("GET /api/admin/users", s.adminUsers, true)
	route("POST /api/admin/users", s.adminCreateUser, true)
	route("PATCH /api/admin/users/{id}", s.adminUpdateUser, true)

	// Personal access tokens, audit log, CSV export, and the Jira-compatible
	// REST API (/rest/…) for scripts and tools.
	s.integrationRoutes(mux, route)

	// E-mail notifications, filter subscriptions, webhooks.
	s.notificationRoutes(route)

	// Instance settings, projects, catalogues, custom fields, groups, roles,
	// system information and Jira import, configured in the interface.
	s.adminRoutes(route)

	// Release checks and updates through the root helper.
	s.updateRoutes(route)

	// The setup wizard is over: it only learns so.
	route("GET /api/setup/status", s.setupDone, false)
	mux.Handle("/api/setup/", s.wrap(func(http.ResponseWriter, *http.Request) error { return errInstalled }, false))

	mux.Handle("/api/", s.wrap(func(http.ResponseWriter, *http.Request) error {
		return fail(http.StatusNotFound, i18n.RequestUnknownAddress)
	}, false))
	mux.Handle("/", static)
	return securityHeaders(s.instanceMiddleware(mux))
}

// Mutating requests must carry this header. Browsers refuse to send custom
// headers cross-site without a CORS preflight, which Solstice never grants, so
// this blocks cross-site request forgery even for multipart uploads.
const csrfHeader = "X-Solstice"

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("X-Frame-Options", "DENY")
		// 'unsafe-inline' stays for styles: skins add a <style> block, Svelte
		// sets style attributes and wiki colours use <span style>.
		h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; "+
			"style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self'; "+
			"connect-src 'self'; object-src 'none'; frame-src 'none'; base-uri 'self'; "+
			"form-action 'self'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

type handler func(http.ResponseWriter, *http.Request) error

type ctxKey int

const (
	userKey ctxKey = iota
	subjectKey
)

// httpError is an error meant for the user. Its message keeps its key and
// values, so the answer can render it in the request's language.
type httpError struct {
	status int
	msg    i18n.Message
	extra  map[string]any // more fields for the JSON answer
}

func (e *httpError) Error() string { return e.msg.String() }

// fail is an error for the user with a fixed message; failf inserts values
// into it.
func fail(status int, key i18n.Key) error { return &httpError{status: status, msg: i18n.M(key)} }

func failf(status int, key i18n.Key, args ...any) error {
	return &httpError{status: status, msg: i18n.M(key, args...)}
}

var (
	errNotFound  = fail(http.StatusNotFound, i18n.RequestNotFound)
	errForbidden = fail(http.StatusForbidden, i18n.RequestForbidden)
)

func (s *Server) wrap(h handler, needUser bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Header.Get(csrfHeader) == "" {
			writeError(w, r, failf(http.StatusForbidden, i18n.RequestMissingHeader, csrfHeader))
			return
		}
		if needUser {
			u, err := s.authenticate(r)
			if err != nil {
				writeError(w, r, fail(http.StatusUnauthorized, i18n.RequestSignIn))
				return
			}
			r = r.WithContext(context.WithValue(r.Context(), userKey, u))
		}
		if err := h(w, r); err != nil {
			var he *httpError
			if !errors.As(err, &he) {
				if errors.Is(err, context.Canceled) {
					return
				}
				s.log.Error("request failed", "method", r.Method, "path", r.URL.Path, "err", err)
				err = fail(http.StatusInternalServerError, i18n.RequestInternalError)
			}
			writeError(w, r, err)
		}
	})
}

// authenticate reads the session cookie, or the credentials of scripts and
// API clients: a personal access token, or HTTP basic (see credentials).
func (s *Server) authenticate(r *http.Request) (*auth.User, error) {
	if c, err := r.Cookie(auth.CookieName); err == nil {
		return auth.Lookup(r.Context(), s.db, c.Value)
	}
	u, _, err := s.credentials(r)
	return u, err
}

// checkPassword verifies credentials, throttled so that passwords cannot be
// guessed at speed and nobody can keep someone else out of their account.
// The design is described on auth.Limiter: the client address, and the pair
// (account, client address), are locked after repeated failures; the account
// alone is never locked, so the right password is always accepted, but its
// wrong answers are held back once many have come in from any address.
func (s *Server) checkPassword(r *http.Request, username, password string) (*auth.User, error) {
	userKey, ipKey := s.throttleKeys(r, username)
	if d := s.limiter.Locked(userKey, ipKey); d > 0 {
		return nil, failf(http.StatusTooManyRequests, i18n.RequestTooManyAttempts, int(d.Seconds())+1)
	}
	release, err := s.limiter.Acquire(r.Context())
	if err != nil {
		return nil, err
	}
	account := auth.CanonicalUsername(username)
	u, err := s.verify(r.Context(), account, password)
	release()
	switch {
	case errors.Is(err, auth.ErrNoSession):
		s.limiter.Failure(userKey, ipKey)
		if delay := s.limiter.AccountFailure("account:" + account); delay > 0 {
			// The account is being guessed at, from here or elsewhere: this
			// address gets half as many tries, and the answer waits (the
			// password-checking slot is already free).
			s.limiter.Failure(userKey)
			s.sleep(r.Context(), delay)
		}
	case err == nil:
		// Only the account at this address is cleared: signing into one's own
		// account must not reset the count of an address guessing other
		// passwords, nor the account's count from other addresses.
		s.limiter.Success(userKey)
	}
	return u, err
}

// throttleKeys are the limiter keys of a sign-in attempt: the account as seen
// from the client address (the canonical user name, see auth.CanonicalUsername,
// so that "Admin" and "admin " count together), and the client address.
func (s *Server) throttleKeys(r *http.Request, username string) (userKey, ipKey string) {
	ip := clientIP(r, s.proxies)
	return "user:" + auth.CanonicalUsername(username) + "|" + ip, "ip:" + ip
}

func currentUser(r *http.Request) *auth.User {
	u, _ := r.Context().Value(userKey).(*auth.User)
	return u
}

// subject returns the permission subject of the current user.
func (s *Server) subject(r *http.Request) (*perm.Subject, error) {
	u := currentUser(r)
	return s.perms.Subject(r.Context(), u.ID, u.Admin)
}

func writeJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

// locale is the language of the answer: the first supported language of
// Accept-Language, else the signed-in user's choice, else the instance's
// default language, else English.
func locale(r *http.Request) string {
	if l := i18n.Negotiate(r.Header.Get("Accept-Language")); l != "" {
		return l
	}
	if u := currentUser(r); u != nil && u.Locale != "" {
		return u.Locale
	}
	if l := requestDefaultLanguage(r); l != "" {
		return l
	}
	return i18n.Default
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	var he *httpError
	if !errors.As(err, &he) {
		he = &httpError{status: http.StatusInternalServerError, msg: i18n.Raw(err.Error())}
	}
	body := map[string]any{"error": he.msg.In(locale(r))}
	for k, v := range he.extra {
		body[k] = v
	}
	writeJSON(w, he.status, body)
}

// readJSON decodes a request body, rejecting unknown fields so typos surface.
func readJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 4<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return failf(http.StatusBadRequest, i18n.RequestInvalid, err.Error())
	}
	return nil
}

// Catalogues: statuses, priorities, resolutions, issue and link types. They
// change rarely and are read on almost every request, so they are cached.

type statusRef struct {
	ID       string               `json:"id"`
	Name     string               `json:"name"`
	Category issue.StatusCategory `json:"category"`
}

type priorityRef struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Color   string `json:"color,omitempty"`
	Default bool   `json:"default,omitempty"` // of issues created without one
}

type namedRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type typeRef struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Subtask bool   `json:"subtask"`
}

type linkTypeRef struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Inward  string `json:"inward"`
	Outward string `json:"outward"`
}

type catalog struct {
	Statuses    []statusRef   `json:"statuses"`
	Priorities  []priorityRef `json:"priorities"`
	Resolutions []namedRef    `json:"resolutions"`
	// DefaultResolution is the resolution of issues reaching a done status
	// without one; "" leaves them unresolved.
	DefaultResolution string        `json:"defaultResolution"`
	IssueTypes        []typeRef     `json:"issueTypes"`
	LinkTypes         []linkTypeRef `json:"linkTypes"`

	status     map[string]statusRef
	priority   map[string]priorityRef
	resolution map[string]namedRef
	issueType  map[string]typeRef
	linkType   map[int64]linkTypeRef
}

func (s *Server) catalogue(ctx context.Context) (*catalog, error) {
	s.catMu.Lock()
	defer s.catMu.Unlock()
	// Short-lived, so an import run by another process shows up quickly.
	if s.catalog != nil && time.Since(s.catAt) < 30*time.Second {
		return s.catalog, nil
	}
	c := &catalog{
		status: map[string]statusRef{}, priority: map[string]priorityRef{},
		resolution: map[string]namedRef{}, issueType: map[string]typeRef{}, linkType: map[int64]linkTypeRef{},
	}
	err := sqlq.Each(ctx, s.db, `SELECT id, name, category FROM statuses ORDER BY position, name`, func(rows *sql.Rows) error {
		var v statusRef
		if err := rows.Scan(&v.ID, &v.Name, &v.Category); err != nil {
			return err
		}
		c.Statuses = append(c.Statuses, v)
		c.status[v.ID] = v
		return nil
	})
	if err != nil {
		return nil, err
	}
	err = sqlq.Each(ctx, s.db, `SELECT id, name, COALESCE(color, ''), is_default FROM priorities ORDER BY position, name`, func(rows *sql.Rows) error {
		var v priorityRef
		if err := rows.Scan(&v.ID, &v.Name, &v.Color, &v.Default); err != nil {
			return err
		}
		c.Priorities = append(c.Priorities, v)
		c.priority[v.ID] = v
		return nil
	})
	if err != nil {
		return nil, err
	}
	err = sqlq.Each(ctx, s.db, `SELECT id, name, is_default FROM resolutions ORDER BY position, name`, func(rows *sql.Rows) error {
		var v namedRef
		var def bool
		if err := rows.Scan(&v.ID, &v.Name, &def); err != nil {
			return err
		}
		c.Resolutions = append(c.Resolutions, v)
		c.resolution[v.ID] = v
		if def {
			c.DefaultResolution = v.ID
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	err = sqlq.Each(ctx, s.db, `SELECT id, name, subtask FROM issue_types ORDER BY position, name`, func(rows *sql.Rows) error {
		var v typeRef
		if err := rows.Scan(&v.ID, &v.Name, &v.Subtask); err != nil {
			return err
		}
		c.IssueTypes = append(c.IssueTypes, v)
		c.issueType[v.ID] = v
		return nil
	})
	if err != nil {
		return nil, err
	}
	err = sqlq.Each(ctx, s.db, `SELECT id, name, inward, outward FROM link_types ORDER BY position, name`, func(rows *sql.Rows) error {
		var v linkTypeRef
		if err := rows.Scan(&v.ID, &v.Name, &v.Inward, &v.Outward); err != nil {
			return err
		}
		c.LinkTypes = append(c.LinkTypes, v)
		c.linkType[v.ID] = v
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.catalog, s.catAt = c, time.Now()
	return c, nil
}

// userRef is how a user appears inside other objects.
type userRef struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	Active   bool   `json:"active"`
}

// users loads the users with the given ids (zero ids are ignored).
func (s *Server) users(ctx context.Context, ids ...int64) (map[int64]userRef, error) {
	out := map[int64]userRef{}
	var args []any
	seen := map[int64]bool{}
	for _, id := range ids {
		if id != 0 && !seen[id] {
			seen[id] = true
			args = append(args, id)
		}
	}
	if len(args) == 0 {
		return out, nil
	}
	err := sqlq.Each(ctx, s.db, `SELECT id, username, display_name, active FROM users WHERE id IN (`+sqlq.Marks(len(args))+`)`,
		func(rows *sql.Rows) error {
			var u userRef
			if err := rows.Scan(&u.ID, &u.Username, &u.Name, &u.Active); err != nil {
				return err
			}
			out[u.ID] = u
			return nil
		}, args...)
	return out, err
}

func refOf(users map[int64]userRef, id sql.NullInt64) *userRef {
	if !id.Valid {
		return nil
	}
	if u, ok := users[id.Int64]; ok {
		return &u
	}
	return nil
}

func dateOnly(t sql.NullTime) *string {
	if !t.Valid {
		return nil
	}
	s := t.Time.Format(time.DateOnly)
	return &s
}
