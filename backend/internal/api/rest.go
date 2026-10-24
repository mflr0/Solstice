package api

// The Jira-compatible REST API — /rest/api/2 (also /rest/api/latest) and
// /rest/agile/1.0 — so that scripts, CI tools and IDE plugins written for
// Jira Server or Data Center keep working against Solstice.
//
// Handlers here translate requests and answers between Jira's shapes and
// Solstice's. The rules (who sees and may do what, validation, history,
// live events) stay those of the interface's API: these handlers call its
// functions, or run its handlers on a request rewritten in its format (see
// uiCall), so the two APIs cannot drift apart.
//
// Sign-in: "Authorization: Bearer <personal access token>", HTTP basic with
// the password (throttled like the sign-in form) or with a token as the
// password, or the interface's session cookie. A request signed in by cookie
// that changes something must carry X-Solstice or "X-Atlassian-Token:
// no-check", like the interface's own requests; so must a basic-auth POST in
// a form encoding a browser could send cross-site, as Jira demands.

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"mime"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"solstice/internal/auth"
	"solstice/internal/i18n"
	"solstice/internal/issue"
	"solstice/internal/sqlq"
)

// Jira's headers. Its clients read the login reason to tell a refused login
// from a lockout, and send the token header to pass the XSRF check.
const (
	loginReasonHeader = "X-Seraph-LoginReason"
	loginDenied       = "AUTHENTICATION_DENIED" // too many attempts
	loginFailed       = "AUTHENTICATED_FAILED"  // wrong credentials
	atlassianToken    = "X-Atlassian-Token"
	noCheck           = "no-check"
)

// integrationRoutes registers the routes of tokens, the audit log, the CSV
// export and the REST API.
func (s *Server) integrationRoutes(mux *http.ServeMux, route func(string, handler, bool)) {
	route("GET /api/me/tokens", s.myTokens, true)
	route("POST /api/me/tokens", s.createToken, true)
	route("DELETE /api/me/tokens/{id}", s.revokeMyToken, true)
	route("GET /api/me/tokens/policy", s.myTokenPolicy, true)
	route("GET /api/admin/tokens", s.adminTokens, true)
	route("DELETE /api/admin/tokens/{id}", s.adminRevokeToken, true)
	route("GET /api/admin/tokens/settings", s.adminTokenSettings, true)
	route("PUT /api/admin/tokens/settings", s.putTokenSettings, true)
	route("GET /api/admin/audit", s.listAudit, true)
	route("GET /api/admin/audit/settings", s.auditSettings, true)
	route("PUT /api/admin/audit/settings", s.putAuditSettings, true)
	route("GET /api/issues/export.csv", s.exportCSV, true)

	rest := func(pattern string, h handler, needUser bool) { mux.Handle(pattern, s.rest(h, needUser)) }
	for _, p := range []string{"/rest/api/2", "/rest/api/latest"} {
		rest("GET "+p+"/serverInfo", s.restServerInfo, false)
		rest("GET "+p+"/myself", s.restMyself, true)
		rest("GET "+p+"/user", s.restUser, true)
		rest("GET "+p+"/user/search", s.restUserSearch, true)
		rest("GET "+p+"/user/assignable/search", s.restAssignable, true)
		rest("GET "+p+"/project", s.restProjects, true)
		rest("GET "+p+"/project/{key}", s.restProject, true)
		rest("GET "+p+"/project/{key}/statuses", s.restProjectStatuses, true)
		rest("GET "+p+"/project/{key}/versions", s.restProjectParts("versions"), true)
		rest("GET "+p+"/project/{key}/components", s.restProjectParts("components"), true)
		rest("GET "+p+"/version/{id}", s.restVersion, true)
		rest("POST "+p+"/version", s.restSaveVersion, true)
		rest("PUT "+p+"/version/{id}", s.restSaveVersion, true)
		rest("DELETE "+p+"/version/{id}", s.restDeleteVersion, true)
		rest("GET "+p+"/component/{id}", s.restComponent, true)
		rest("POST "+p+"/component", s.restSaveComponent, true)
		rest("PUT "+p+"/component/{id}", s.restSaveComponent, true)
		rest("DELETE "+p+"/component/{id}", s.restDeleteComponent, true)

		rest("POST "+p+"/issue", s.restCreateIssue, true)
		rest("GET "+p+"/issue/{key}", s.restGetIssue, true)
		rest("PUT "+p+"/issue/{key}", s.restUpdateIssue, true)
		rest("DELETE "+p+"/issue/{key}", s.restDeleteIssue, true)
		rest("PUT "+p+"/issue/{key}/assignee", s.restAssign, true)
		rest("GET "+p+"/issue/{key}/transitions", s.restTransitions, true)
		rest("POST "+p+"/issue/{key}/transitions", s.restTransition, true)
		rest("GET "+p+"/issue/{key}/comment", s.restComments, true)
		rest("POST "+p+"/issue/{key}/comment", s.restAddComment, true)
		rest("GET "+p+"/issue/{key}/comment/{id}", s.restComment, true)
		rest("PUT "+p+"/issue/{key}/comment/{id}", s.restEditComment, true)
		rest("DELETE "+p+"/issue/{key}/comment/{id}", s.restDeleteComment, true)
		rest("GET "+p+"/issue/{key}/worklog", s.restWorklogs, true)
		rest("POST "+p+"/issue/{key}/worklog", s.restAddWorklog, true)
		rest("GET "+p+"/issue/{key}/worklog/{id}", s.restWorklog, true)
		rest("PUT "+p+"/issue/{key}/worklog/{id}", s.restEditWorklog, true)
		rest("DELETE "+p+"/issue/{key}/worklog/{id}", s.restDeleteWorklog, true)
		rest("GET "+p+"/issue/{key}/watchers", s.restWatchers, true)
		rest("POST "+p+"/issue/{key}/watchers", s.restWatch(true), true)
		rest("DELETE "+p+"/issue/{key}/watchers", s.restWatch(false), true)
		rest("POST "+p+"/issue/{key}/votes", s.restVote(true), true)
		rest("DELETE "+p+"/issue/{key}/votes", s.restVote(false), true)
		rest("POST "+p+"/issue/{key}/attachments", s.restAttach, true)
		rest("GET "+p+"/attachment/{id}", s.restAttachment, true)
		rest("DELETE "+p+"/attachment/{id}", s.deleteAttachment, true)
		rest("POST "+p+"/issueLink", s.restLink, true)
		rest("DELETE "+p+"/issueLink/{id}", s.deleteLink, true)
		rest("GET "+p+"/issueLinkType", s.restLinkTypes, true)
		rest("GET "+p+"/issue/createmeta", s.restCreateMeta, true)
		rest("GET "+p+"/issue/{key}/editmeta", s.restEditMeta, true)
		rest("GET "+p+"/issue/picker", s.restIssuePicker, true)

		rest("GET "+p+"/search", s.restSearch, true)
		rest("POST "+p+"/search", s.restSearch, true)
		rest("GET "+p+"/field", s.restFields, true)
		rest("GET "+p+"/priority", s.restPriorities, true)
		rest("GET "+p+"/status", s.restStatuses, true)
		rest("GET "+p+"/issuetype", s.restIssueTypes, true)
		rest("GET "+p+"/resolution", s.restResolutions, true)
		rest("GET "+p+"/filter/favourite", s.restFavouriteFilters, true)
		rest("GET "+p+"/filter/{id}", s.restFilter, true)
		rest("POST "+p+"/filter", s.restSaveFilter, true)
		rest("PUT "+p+"/filter/{id}", s.restSaveFilter, true)
		rest("DELETE "+p+"/filter/{id}", s.deleteFilter, true)
		rest("GET "+p+"/mypermissions", s.restMyPermissions, true)

		rest("GET "+p+"/issue/{key}/remotelink", s.restRemoteLinks, true)
		rest("POST "+p+"/issue/{key}/remotelink", s.restAddRemoteLink, true)
		rest("DELETE "+p+"/issue/{key}/remotelink", s.restDeleteRemoteLink, true)
		rest("GET "+p+"/issue/{key}/remotelink/{linkId}", s.restRemoteLink, true)
		rest("PUT "+p+"/issue/{key}/remotelink/{linkId}", s.restEditRemoteLink, true)
		rest("DELETE "+p+"/issue/{key}/remotelink/{linkId}", s.restDeleteRemoteLink, true)

		rest("GET "+p+"/groups/picker", s.restGroupPicker, true)
		rest("GET "+p+"/group/member", s.restGroupMembers, true)
	}
	// Jira's cookie sign-in, which old scripts still use.
	mux.Handle("GET /rest/auth/1/session", s.rest(s.restSession, true))
	mux.Handle("POST /rest/auth/1/session", s.restSessionRoute(s.restSignIn, false))
	mux.Handle("DELETE /rest/auth/1/session", s.restSessionRoute(s.restSignOut, true))
	for _, p := range []string{"/rest/agile/1.0", "/rest/agile/latest"} {
		rest("GET "+p+"/board", s.agileBoards, true)
		rest("GET "+p+"/board/{id}", s.agileBoard, true)
		rest("GET "+p+"/board/{id}/configuration", s.agileBoardConfiguration, true)
		rest("GET "+p+"/board/{id}/sprint", s.agileBoardSprints, true)
		rest("GET "+p+"/board/{id}/issue", s.agileBoardIssues, true)
		rest("GET "+p+"/board/{id}/backlog", s.agileBacklog, true)
		rest("GET "+p+"/sprint/{id}", s.agileSprint, true)
		rest("POST "+p+"/sprint/{id}", s.agileEditSprint, true)
		rest("PUT "+p+"/sprint/{id}", s.agileEditSprint, true)
		rest("DELETE "+p+"/sprint/{id}", s.agileDeleteSprint, true)
		rest("GET "+p+"/sprint/{id}/issue", s.agileSprintIssues, true)
		rest("POST "+p+"/sprint/{id}/issue", s.agileMoveToSprint, true)
		rest("POST "+p+"/backlog/issue", s.agileMoveToBacklog, true)
		rest("PUT "+p+"/issue/rank", s.agileRank, true)
		rest("POST "+p+"/issue/rank", s.agileRank, true)
		rest("GET "+p+"/epic/{key}", s.agileEpic, true)
		rest("GET "+p+"/epic/{key}/issue", s.agileEpicIssues, true)
	}
	// Jira's own addresses for files and avatars, which answers link to.
	rest("GET /secure/attachment/{id}/{filename}", s.download, true)
	rest("GET /secure/useravatar", s.userAvatar, true)
	rest("GET /secure/projectavatar", s.projectAvatar, true)
	mux.Handle("/rest/", s.rest(func(http.ResponseWriter, *http.Request) error {
		return fail(http.StatusNotFound, i18n.RequestUnknownAddress)
	}, false))
}

// Sign-in methods of a REST request.
const (
	viaCookie = "cookie"
	viaToken  = "token"
	viaBasic  = "basic"
)

// rest wraps a REST handler: sign-in, protection against cross-site request
// forgery, and errors in Jira's format.
func (s *Server) rest(h handler, needUser bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, via, err := s.restCredentials(r)
		if err != nil {
			var he *httpError
			if errors.As(err, &he) && he.status == http.StatusTooManyRequests {
				w.Header().Set(loginReasonHeader, loginDenied)
			} else if errors.Is(err, auth.ErrNoSession) {
				w.Header().Set(loginReasonHeader, loginFailed)
				err = fail(http.StatusUnauthorized, i18n.RequestCredentialsRefused)
			}
			s.writeRESTFailure(w, r, err)
			return
		}
		if u == nil {
			w.Header().Set("X-AUSERNAME", "anonymous")
			if needUser {
				s.writeRESTError(w, r, fail(http.StatusUnauthorized, i18n.RequestSignIn))
				return
			}
		}
		if !forgeryProof(r, via) {
			s.writeRESTError(w, r, fail(http.StatusForbidden, i18n.RequestXSRFHeader))
			return
		}
		if u != nil {
			r = r.WithContext(context.WithValue(r.Context(), userCtxKey, u))
			w.Header().Set("X-AUSERNAME", u.Username)
		}
		if err := h(w, r); err != nil {
			s.writeRESTFailure(w, r, err)
		}
	})
}

// restCredentials signs a REST request in. Explicit credentials (an
// Authorization header) win over the session cookie; a request without
// either is anonymous (u is nil, err too).
func (s *Server) restCredentials(r *http.Request) (u *auth.User, via string, err error) {
	if r.Header.Get("Authorization") != "" {
		return s.credentials(r)
	}
	if c, err := r.Cookie(auth.CookieName); err == nil {
		u, err := auth.Lookup(r.Context(), s.db, c.Value)
		if errors.Is(err, auth.ErrNoSession) {
			return nil, "", nil // an expired session: anonymous
		}
		return u, viaCookie, err
	}
	return nil, "", nil
}

// forgeryProof tells whether a request cannot have been forged by another
// site. Reading is always safe. Browsers never add a bearer token by
// themselves, and only send custom headers cross-site after a CORS
// preflight, which Solstice never grants. A session cookie, or basic
// credentials a browser remembered, are sent with any request; a
// cross-site form can only POST url-encoded, multipart or plain text bodies.
func forgeryProof(r *http.Request, via string) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	if r.Header.Get(csrfHeader) != "" {
		return true
	}
	if v := strings.ToLower(r.Header.Get(atlassianToken)); v == noCheck || v == "nocheck" {
		return true
	}
	switch via {
	case viaToken:
		return true
	case viaBasic:
		return r.Method != http.MethodPost || !formEncoded(r)
	}
	return false
}

// formEncoded tells whether a request body has a type an HTML form (or a
// "simple" cross-site request) can send.
func formEncoded(r *http.Request) bool {
	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	switch strings.ToLower(mediaType) {
	case "", "application/x-www-form-urlencoded", "multipart/form-data", "text/plain":
		return true
	}
	return false
}

// writeRESTFailure answers an error, logging those not meant for the user.
func (s *Server) writeRESTFailure(w http.ResponseWriter, r *http.Request, err error) {
	if err := boundary(s.log, "request failed", r, err); err != nil {
		s.writeRESTError(w, r, err)
	}
}

// restError is Jira's error body: general messages, and messages per field
// of the request.
type restError struct {
	ErrorMessages []string          `json:"errorMessages"`
	Errors        map[string]string `json:"errors"`
	RetryAfter    int               `json:"retryAfter,omitempty"`
}

func (s *Server) writeRESTError(w http.ResponseWriter, r *http.Request, err error) {
	var he *httpError
	errors.As(err, &he)
	// Refused: leave a trace at Warn, capped per address (refused.go).
	if he.status == http.StatusUnauthorized || he.status == http.StatusForbidden {
		refused.logRefused(r, he.status, s.proxies)
	}
	body := restError{ErrorMessages: []string{}, Errors: map[string]string{}}
	msg := he.msg.In(locale(r))
	if field, _ := he.extra["field"].(string); field != "" {
		body.Errors[field] = msg
	} else {
		body.ErrorMessages = append(body.ErrorMessages, msg)
	}
	if he.retry > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(he.retry))
		body.RetryAfter = he.retry
	}
	writeJSON(w, he.status, body)
}

// fieldError ties an error to a field of the request (Jira's "errors" map).
func fieldError(field string, err error) error {
	var he *httpError
	if !errors.As(err, &he) {
		return err
	}
	extra := maps.Clone(he.extra)
	if extra == nil {
		extra = map[string]any{}
	}
	extra["field"] = field
	return &httpError{status: he.status, msg: he.msg, extra: extra}
}

// uiCall runs a handler of the interface's API on a request rewritten in its
// format: method, query, path values and JSON body. The handler applies its
// usual checks; its error is returned as is, its JSON answer decoded into out
// (when not nil).
func (s *Server) uiCall(r *http.Request, h handler, method string, query url.Values, values map[string]string, body, out any) error {
	var raw []byte
	if body != nil {
		var err error
		if raw, err = json.Marshal(body); err != nil {
			return err
		}
	}
	req := r.Clone(r.Context())
	req.Method, req.URL = method, &url.URL{RawQuery: query.Encode()}
	req.RequestURI = req.URL.RequestURI()
	req.Body, req.ContentLength = io.NopCloser(bytes.NewReader(raw)), int64(len(raw))
	req.Header = r.Header.Clone()
	req.Header.Set("Content-Type", "application/json")
	for k, v := range values {
		req.SetPathValue(k, v)
	}
	rec := &recorder{header: http.Header{}}
	if err := h(rec, req); err != nil {
		return err
	}
	if out != nil && rec.body.Len() > 0 {
		return json.Unmarshal(rec.body.Bytes(), out)
	}
	return nil
}

// recorder keeps what a handler answers, for uiCall.
type recorder struct {
	header http.Header
	body   bytes.Buffer
}

func (r *recorder) Header() http.Header { return r.header }

func (r *recorder) WriteHeader(int) {}

func (r *recorder) Write(b []byte) (int, error) { return r.body.Write(b) }

// readRESTJSON decodes a REST request body. Unlike the interface's API,
// unknown fields are ignored: Jira clients send many that Solstice has no
// use for.
func readRESTJSON(r *http.Request, v any) error {
	if err := json.NewDecoder(http.MaxBytesReader(nil, r.Body, maxJSONBody)).Decode(v); err != nil {
		return badBody(err)
	}
	return nil
}

// paging reads Jira's startAt and maxResults: def results by default, at
// most limit. maxResults=0 only counts.
func paging(r *http.Request, def, limit int) (startAt, maxResults int) {
	q := r.URL.Query()
	startAt = min(max(0, atoi(q.Get("startAt"), 0)), 1<<31)
	maxResults = atoi(q.Get("maxResults"), def)
	if maxResults < 0 {
		maxResults = def
	}
	return startAt, min(maxResults, limit)
}

// Building answers ------------------------------------------------------------

// restOut holds what building Jira-shaped answers needs: addresses, the
// language and the time zone of dates.
type restOut struct {
	base   string // public address, without a trailing slash
	api    string // base + /rest/api/2
	locale string
	tz     *time.Location
}

func (s *Server) restOut(r *http.Request) *restOut {
	base := s.base()
	if base == "" {
		scheme := "http"
		if secureRequest(r, "", s.proxies, s.log) {
			scheme = "https"
		}
		base = scheme + "://" + r.Host
	}
	return &restOut{base: base, api: base + "/rest/api/2", locale: locale(r), tz: s.zone()}
}

// zone is the time zone of dates typed or shown without one: the
// instance's setting (initially SOLSTICE_TZ).
func (s *Server) zone() *time.Location {
	if loc := s.general(context.Background()).loc; loc != nil {
		return loc
	}
	return time.UTC
}

// jiraTimeLayout is how Jira writes instants: "2026-10-15T09:12:00.000+0200".
const jiraTimeLayout = "2006-01-02T15:04:05.000-0700"

func (o *restOut) time(t time.Time) string { return t.In(o.tz).Format(jiraTimeLayout) }

func (o *restOut) timePtr(t *time.Time) any {
	if t == nil {
		return nil
	}
	return o.time(*t)
}

// parseJiraTime reads an instant as Jira clients write it ("2026-10-15T09:12:00.000+0200"),
// or in RFC 3339.
func parseJiraTime(v string) (time.Time, bool) {
	for _, layout := range []string{jiraTimeLayout, "2006-01-02T15:04:05.000Z0700", "2006-01-02T15:04:05-0700",
		time.RFC3339Nano} {
		if t, err := time.Parse(layout, v); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func (o *restOut) msg(m i18n.Message) string { return m.In(o.locale) }

func (o *restOut) id(n int64) string { return strconv.FormatInt(n, 10) }

func (o *restOut) statusCategory(category issue.StatusCategory) map[string]any {
	c := category.Jira()
	return map[string]any{"self": o.api + "/statuscategory/" + strconv.Itoa(c.ID), "id": c.ID, "key": c.Key,
		"colorName": c.Colour, "name": o.msg(category.Name())}
}

func (o *restOut) status(st statusRef) map[string]any {
	return map[string]any{"self": o.api + "/status/" + st.ID, "id": st.ID, "name": st.Name,
		"untranslatedName": st.Name, "description": "", "statusCategory": o.statusCategory(st.Category)}
}

func (o *restOut) issueType(t typeRef) map[string]any {
	return map[string]any{"self": o.api + "/issuetype/" + t.ID, "id": t.ID, "name": t.Name, "description": "",
		"subtask": t.Subtask}
}

func (o *restOut) priority(p *priorityRef) any {
	if p == nil {
		return nil
	}
	out := map[string]any{"self": o.api + "/priority/" + p.ID, "id": p.ID, "name": p.Name}
	if p.Color != "" {
		out["statusColor"] = p.Color
	}
	return out
}

func (o *restOut) resolution(v *namedRef) any {
	if v == nil {
		return nil
	}
	return map[string]any{"self": o.api + "/resolution/" + v.ID, "id": v.ID, "name": v.Name, "description": ""}
}

// jiraUser is a user as Jira shows one.
type jiraUser struct {
	Self         string            `json:"self"`
	Key          string            `json:"key"`
	Name         string            `json:"name"`
	EmailAddress string            `json:"emailAddress,omitempty"`
	AvatarURLs   map[string]string `json:"avatarUrls"`
	DisplayName  string            `json:"displayName"`
	Active       bool              `json:"active"`
	TimeZone     string            `json:"timeZone"`
}

func (o *restOut) avatars(kind, owner string) map[string]string {
	u := o.base + "/secure/" + kind + "?ownerId=" + url.QueryEscape(owner)
	return map[string]string{"48x48": u, "32x32": u + "&size=medium", "24x24": u + "&size=small",
		"16x16": u + "&size=xsmall"}
}

// jiraUsers loads users by id (zero ids are ignored). E-mail addresses are
// shown as on people's pages, to every signed-in user.
func (s *Server) jiraUsers(ctx context.Context, o *restOut, ids ...int64) (map[int64]*jiraUser, error) {
	out := map[int64]*jiraUser{}
	args := sqlq.IDs(ids...)
	for chunk := range slices.Chunk(args, 500) {
		err := sqlq.Each(ctx, s.db, `SELECT id, username, COALESCE(user_key, username), display_name, email, active
			FROM users WHERE id IN (`+sqlq.Marks(len(chunk))+`)`, func(rows *sql.Rows) error {
			var id int64
			u := &jiraUser{TimeZone: o.tz.String()}
			if err := rows.Scan(&id, &u.Name, &u.Key, &u.DisplayName, &u.EmailAddress, &u.Active); err != nil {
				return err
			}
			u.Self = o.api + "/user?username=" + url.QueryEscape(u.Name)
			pic := o.base + "/api/avatars/" + o.id(id)
			u.AvatarURLs = map[string]string{"48x48": pic, "32x32": pic, "24x24": pic, "16x16": pic}
			out[id] = u
			return nil
		}, chunk...)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// userOf returns a loaded user, or nil (JSON null) for none.
func userOf(users map[int64]*jiraUser, id int64) any {
	if u, ok := users[id]; ok && id != 0 {
		return u
	}
	return nil
}

// userByName finds an account by user name, or by Jira user key.
func (s *Server) userByName(ctx context.Context, name string) (int64, bool) {
	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT id FROM users WHERE username = LOWER(?) OR user_key = ?
		ORDER BY username = LOWER(?) DESC LIMIT 1`, name, name, name).Scan(&id)
	return id, err == nil
}
