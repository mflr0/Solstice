// Package api is Solstice's HTTP API, consumed by the Svelte interface.
//
// Every route under /api answers JSON. Errors come back as
// {"error": "message"} with the message in French, ready to show.
package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"solstice/internal/auth"
	"solstice/internal/live"
	"solstice/internal/perm"
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

	catMu   sync.Mutex
	catalog *catalog
	catAt   time.Time
}

// New builds the API server. dataDir holds attachment files; tz is the time
// zone of dates typed in searches.
func New(db *sql.DB, dataDir string, tz *time.Location, log *slog.Logger) *Server {
	return &Server{db: db, perms: perm.NewStore(db), hub: live.NewHub(), dataDir: dataDir, tz: tz, log: log,
		limiter: auth.NewLimiter()}
}

// Handler returns the API routes, with static serving of the interface for
// everything outside /api.
func (s *Server) Handler(static http.Handler) http.Handler {
	mux := http.NewServeMux()
	route := func(pattern string, h handler, needUser bool) {
		mux.Handle(pattern, s.wrap(h, needUser))
	}

	route("POST /api/login", s.login, false)
	route("POST /api/logout", s.logout, false)
	route("GET /api/me", s.me, true)
	route("GET /api/meta", s.meta, true)
	route("GET /api/users", s.searchUsers, true)
	route("GET /api/people/{username}", s.person, true)
	route("POST /api/refs", s.refs, true)
	route("GET /api/events", s.events, true)

	route("GET /api/projects", s.listProjects, true)
	route("GET /api/projects/{key}", s.getProject, true)
	route("GET /api/projects/{key}/assignable", s.assignableUsers, true)
	route("GET /api/projects/{key}/permissions", s.getPermissions, true)
	route("PUT /api/projects/{key}/permissions", s.putPermissions, true)

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

	route("GET /api/filters", s.listFilters, true)
	route("POST /api/filters", s.createFilter, true)
	route("PATCH /api/filters/{id}", s.updateFilter, true)
	route("DELETE /api/filters/{id}", s.deleteFilter, true)
	route("PUT /api/filters/{id}/favourite", s.setFavourite(true), true)
	route("DELETE /api/filters/{id}/favourite", s.setFavourite(false), true)

	route("GET /api/admin/users", s.adminUsers, true)
	route("POST /api/admin/users", s.adminCreateUser, true)
	route("PATCH /api/admin/users/{id}", s.adminUpdateUser, true)

	mux.Handle("/api/", s.wrap(func(http.ResponseWriter, *http.Request) error {
		return fail(http.StatusNotFound, "Adresse inconnue.")
	}, false))
	mux.Handle("/", static)
	return securityHeaders(mux)
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
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: blob:; "+
			"style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; "+
			"frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		next.ServeHTTP(w, r)
	})
}

type handler func(http.ResponseWriter, *http.Request) error

type ctxKey int

const (
	userKey ctxKey = iota
	subjectKey
)

// httpError is an error meant for the user.
type httpError struct {
	status int
	msg    string
}

func (e *httpError) Error() string { return e.msg }

func fail(status int, msg string) error { return &httpError{status, msg} }

var (
	errNotFound  = fail(http.StatusNotFound, "Introuvable, ou vous n'y avez pas accès.")
	errForbidden = fail(http.StatusForbidden, "Vous n'avez pas le droit de faire cela.")
)

func (s *Server) wrap(h handler, needUser bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Header.Get(csrfHeader) == "" {
			writeError(w, fail(http.StatusForbidden, "Requête refusée (en-tête "+csrfHeader+" manquant)."))
			return
		}
		if needUser {
			u, err := s.authenticate(r)
			if err != nil {
				writeError(w, fail(http.StatusUnauthorized, "Connectez-vous pour continuer."))
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
				err = fail(http.StatusInternalServerError, "Erreur interne, l'incident a été journalisé.")
			}
			writeError(w, err)
		}
	})
}

// authenticate reads the session cookie, or HTTP basic credentials for
// scripts and API clients.
func (s *Server) authenticate(r *http.Request) (*auth.User, error) {
	if c, err := r.Cookie(auth.CookieName); err == nil {
		return auth.Lookup(r.Context(), s.db, c.Value)
	}
	if name, pass, ok := r.BasicAuth(); ok {
		return s.checkPassword(r, name, pass)
	}
	return nil, auth.ErrNoSession
}

// checkPassword verifies credentials, throttled per user name and per client
// address so passwords cannot be guessed at speed.
func (s *Server) checkPassword(r *http.Request, username, password string) (*auth.User, error) {
	userKey, ipKey := "user:"+strings.ToLower(username), "ip:"+clientIP(r)
	if d := s.limiter.Locked(userKey, ipKey); d > 0 {
		return nil, fail(http.StatusTooManyRequests,
			fmt.Sprintf("Trop de tentatives : réessayez dans %d s.", int(d.Seconds())+1))
	}
	release, err := s.limiter.Acquire(r.Context())
	if err != nil {
		return nil, err
	}
	u, err := auth.Authenticate(r.Context(), s.db, username, password)
	release()
	switch {
	case errors.Is(err, auth.ErrNoSession):
		s.limiter.Failure(userKey, ipKey)
	case err == nil:
		// Only the account is cleared: signing into one's own account must not
		// reset the count of an address guessing other passwords.
		s.limiter.Success(userKey)
	}
	return u, err
}

// clientIP is the address of the connection. Behind a reverse proxy, all
// clients share the proxy's address for throttling purposes.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
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

func writeError(w http.ResponseWriter, err error) {
	var he *httpError
	if !errors.As(err, &he) {
		he = &httpError{http.StatusInternalServerError, err.Error()}
	}
	writeJSON(w, he.status, map[string]string{"error": he.msg})
}

// readJSON decodes a request body, rejecting unknown fields so typos surface.
func readJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 4<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fail(http.StatusBadRequest, "Requête invalide : "+err.Error())
	}
	return nil
}

// Catalogues: statuses, priorities, resolutions, issue and link types. They
// change rarely and are read on almost every request, so they are cached.

type statusRef struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
}

type priorityRef struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
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
	IssueTypes  []typeRef     `json:"issueTypes"`
	LinkTypes   []linkTypeRef `json:"linkTypes"`

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
	err := scanAll(ctx, s.db, `SELECT id, name, category FROM statuses ORDER BY position, name`, func(rows *sql.Rows) error {
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
	err = scanAll(ctx, s.db, `SELECT id, name, COALESCE(color, '') FROM priorities ORDER BY position, name`, func(rows *sql.Rows) error {
		var v priorityRef
		if err := rows.Scan(&v.ID, &v.Name, &v.Color); err != nil {
			return err
		}
		c.Priorities = append(c.Priorities, v)
		c.priority[v.ID] = v
		return nil
	})
	if err != nil {
		return nil, err
	}
	err = scanAll(ctx, s.db, `SELECT id, name FROM resolutions ORDER BY position, name`, func(rows *sql.Rows) error {
		var v namedRef
		if err := rows.Scan(&v.ID, &v.Name); err != nil {
			return err
		}
		c.Resolutions = append(c.Resolutions, v)
		c.resolution[v.ID] = v
		return nil
	})
	if err != nil {
		return nil, err
	}
	err = scanAll(ctx, s.db, `SELECT id, name, subtask FROM issue_types ORDER BY position, name`, func(rows *sql.Rows) error {
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
	err = scanAll(ctx, s.db, `SELECT id, name, inward, outward FROM link_types ORDER BY position, name`, func(rows *sql.Rows) error {
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

func scanAll(ctx context.Context, db *sql.DB, q string, scan func(*sql.Rows) error, args ...any) error {
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := scan(rows); err != nil {
			return err
		}
	}
	return rows.Err()
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
	err := scanAll(ctx, s.db, `SELECT id, username, display_name, active FROM users WHERE id IN (`+marks(len(args))+`)`,
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

func marks(n int) string { return strings.TrimSuffix(strings.Repeat("?, ", n), ", ") }

func dateOnly(t sql.NullTime) *string {
	if !t.Valid {
		return nil
	}
	s := t.Time.Format(time.DateOnly)
	return &s
}
