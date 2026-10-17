package api

// Remote links, Jira's session route and groups of the REST API.

import (
	"context"
	"database/sql"
	"errors"
	"html"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"solstice/internal/audit"
	"solstice/internal/auth"
	"solstice/internal/i18n"
	"solstice/internal/perm"
	"solstice/internal/sqlq"
)

// Remote links -----------------------------------------------------------------

// remoteLinkIn is a remote link as Jira clients write one. Solstice keeps
// the address, the title and the relationship; the rest (application, icon,
// summary, status) is accepted and ignored, as is the globalId, which the
// table has no column for.
type remoteLinkIn struct {
	Relationship string `json:"relationship"`
	Object       struct {
		URL   string `json:"url"`
		Title string `json:"title"`
	} `json:"object"`
}

// remoteLinkOK tells whether an address may become a link: absolute, and
// only the http, https and mailto schemes.
func remoteLinkOK(v string) bool { return safeHref(v) && safeURL(v) }

// clean checks and trims what a client sent.
func (in *remoteLinkIn) clean() error {
	in.Object.URL, in.Object.Title = strings.TrimSpace(in.Object.URL), strings.TrimSpace(in.Object.Title)
	in.Relationship = strings.TrimSpace(in.Relationship)
	switch {
	case in.Object.URL == "":
		return fieldError("object.url", failf(http.StatusBadRequest, i18n.FieldRequired, "object.url"))
	case !remoteLinkOK(in.Object.URL):
		return invalidValue("object.url")
	case len(in.Object.URL) > 2000:
		return fieldError("object.url", failf(http.StatusBadRequest, i18n.FieldTooLong, 2000))
	case in.Object.Title == "":
		return fieldError("object.title", failf(http.StatusBadRequest, i18n.FieldRequired, "object.title"))
	case len([]rune(in.Object.Title)) > 255:
		return fieldError("object.title", failf(http.StatusBadRequest, i18n.FieldTooLong, 255))
	case len([]rune(in.Relationship)) > 255:
		return fieldError("relationship", failf(http.StatusBadRequest, i18n.FieldTooLong, 255))
	}
	return nil
}

func (o *restOut) remoteLink(key string, v remoteLink) map[string]any {
	out := map[string]any{"id": v.ID, "self": o.api + "/issue/" + key + "/remotelink/" + o.id(v.ID),
		"object": map[string]any{"url": v.URL, "title": v.Title}}
	if v.Relationship != "" {
		out["relationship"] = v.Relationship
	}
	return out
}

// remoteLinkText is how the history names a remote link, as Jira writes it.
func remoteLinkText(title string) string { return `This issue links to "` + title + `"` }

// refuseGlobalID answers requests that name a link by its globalId.
func refuseGlobalID(r *http.Request) error {
	if r.URL.Query().Get("globalId") != "" {
		return fieldError("globalId", fail(http.StatusBadRequest, i18n.RemoteLinkGlobalIDUnsupported))
	}
	return nil
}

// restEditableIssue returns the issue of the path, which the user must be
// allowed to edit.
func (s *Server) restEditableIssue(r *http.Request) (*perm.Subject, *issueRow, error) {
	sub, iss, err := s.restSubjectIssue(r)
	if err != nil {
		return nil, nil, err
	}
	pi := iss.perm()
	if !sub.Can(perm.Edit, iss.ProjectID, &pi) {
		return nil, nil, errForbidden
	}
	return sub, iss, nil
}

// pathRemoteLink loads the remote link of the path, which belongs to iss.
func (s *Server) pathRemoteLink(r *http.Request, iss *issueRow) (remoteLink, error) {
	var v remoteLink
	id, err := strconv.ParseInt(r.PathValue("linkId"), 10, 64)
	if err != nil {
		return v, errNotFound
	}
	err = s.db.QueryRowContext(r.Context(), `SELECT id, url, title, COALESCE(relationship, '') FROM remote_links
		WHERE id = ? AND issue_id = ?`, id, iss.ID).Scan(&v.ID, &v.URL, &v.Title, &v.Relationship)
	if errors.Is(err, sql.ErrNoRows) {
		return v, errNotFound
	}
	return v, err
}

func (s *Server) restRemoteLinks(w http.ResponseWriter, r *http.Request) error {
	if err := refuseGlobalID(r); err != nil {
		return err
	}
	_, iss, err := s.restSubjectIssue(r)
	if err != nil {
		return err
	}
	o := s.restOut(r)
	list := []any{}
	if err := sqlq.Each(r.Context(), s.db, `SELECT id, url, title, COALESCE(relationship, '') FROM remote_links
		WHERE issue_id = ? ORDER BY id`, func(rows *sql.Rows) error {
		var v remoteLink
		if err := rows.Scan(&v.ID, &v.URL, &v.Title, &v.Relationship); err != nil {
			return err
		}
		list = append(list, o.remoteLink(iss.Key(), v))
		return nil
	}, iss.ID); err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, list)
}

func (s *Server) restRemoteLink(w http.ResponseWriter, r *http.Request) error {
	_, iss, err := s.restSubjectIssue(r)
	if err != nil {
		return err
	}
	v, err := s.pathRemoteLink(r, iss)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, s.restOut(r).remoteLink(iss.Key(), v))
}

func (s *Server) restAddRemoteLink(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, iss, err := s.restEditableIssue(r)
	if err != nil {
		return err
	}
	var in remoteLinkIn
	if err := readRESTJSON(r, &in); err != nil {
		return err
	}
	if err := in.clean(); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `INSERT INTO remote_links (issue_id, url, title, relationship) VALUES (?, ?, ?, ?)`,
		iss.ID, in.Object.URL, in.Object.Title, sql.NullString{String: in.Relationship, Valid: in.Relationship != ""})
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	changes := []change{{field: "RemoteIssueLink", newValue: id, newString: remoteLinkText(in.Object.Title)}}
	if err := recordChanges(ctx, tx, iss.ID, sub.UserID, time.Now().UTC(), changes); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.changed(iss, sub.UserID, "link", changes)
	o := s.restOut(r)
	return writeJSON(w, http.StatusCreated, map[string]any{"id": id,
		"self": o.api + "/issue/" + iss.Key() + "/remotelink/" + o.id(id)})
}

func (s *Server) restEditRemoteLink(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, iss, err := s.restEditableIssue(r)
	if err != nil {
		return err
	}
	old, err := s.pathRemoteLink(r, iss)
	if err != nil {
		return err
	}
	var in remoteLinkIn
	if err := readRESTJSON(r, &in); err != nil {
		return err
	}
	if err := in.clean(); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE remote_links SET url = ?, title = ?, relationship = ? WHERE id = ?`,
		in.Object.URL, in.Object.Title, sql.NullString{String: in.Relationship, Valid: in.Relationship != ""},
		old.ID); err != nil {
		return err
	}
	var changes []change
	if in.Object.URL != old.URL || in.Object.Title != old.Title || in.Relationship != old.Relationship {
		changes = []change{{field: "RemoteIssueLink", oldValue: old.ID, oldString: remoteLinkText(old.Title),
			newValue: old.ID, newString: remoteLinkText(in.Object.Title)}}
	}
	if err := recordChanges(ctx, tx, iss.ID, sub.UserID, time.Now().UTC(), changes); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if len(changes) > 0 {
		s.changed(iss, sub.UserID, "link", changes)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) restDeleteRemoteLink(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	if err := refuseGlobalID(r); err != nil {
		return err
	}
	sub, iss, err := s.restEditableIssue(r)
	if err != nil {
		return err
	}
	old, err := s.pathRemoteLink(r, iss)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM remote_links WHERE id = ?`, old.ID); err != nil {
		return err
	}
	changes := []change{{field: "RemoteIssueLink", oldValue: old.ID, oldString: remoteLinkText(old.Title)}}
	if err := recordChanges(ctx, tx, iss.ID, sub.UserID, time.Now().UTC(), changes); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.changed(iss, sub.UserID, "link", changes)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// Session ----------------------------------------------------------------------

// restSessionRoute lets scripts written for Jira's cookie sign-in call the
// session routes the way they did there: a JSON body or a DELETE needs no
// X-Atlassian-Token header. A browser cannot send either cross-site without a
// CORS preflight, which Solstice never grants; form-like POSTs still need the
// header.
func (s *Server) restSessionRoute(h handler, needUser bool) http.Handler {
	inner := s.rest(h, needUser)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete || (r.Method == http.MethodPost && !formEncoded(r)) {
			r = r.Clone(r.Context())
			r.Header.Set("X-Atlassian-Token", "no-check")
		}
		inner.ServeHTTP(w, r)
	})
}

// restLoginInfo is what Solstice knows of a user's sign-ins, from the audit
// log: how many, since when they failed, and the one before the latest.
func (s *Server) restLoginInfo(ctx context.Context, o *restOut, userID int64) (map[string]any, error) {
	info := map[string]any{"loginCount": 0, "failedLoginCount": 0}
	var successes []time.Time
	if err := sqlq.Each(ctx, s.db, `SELECT created_at FROM audit_log WHERE user_id = ? AND action = ?
		ORDER BY created_at DESC, id DESC LIMIT 2`, func(rows *sql.Rows) error {
		var at time.Time
		if err := rows.Scan(&at); err != nil {
			return err
		}
		successes = append(successes, at)
		return nil
	}, userID, audit.LoginSuccess); err != nil {
		return nil, err
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_log WHERE user_id = ? AND action = ?`,
		userID, audit.LoginSuccess).Scan(&count); err != nil {
		return nil, err
	}
	info["loginCount"] = count
	since := time.Unix(0, 0).UTC()
	if len(successes) > 0 {
		since = successes[0]
	}
	if len(successes) > 1 {
		info["previousLoginTime"] = o.time(successes[1])
	}
	var failed int
	var last sql.NullTime
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(created_at > ?), 0), MAX(created_at) FROM audit_log
		WHERE user_id = ? AND action = ?`, since, userID, audit.LoginFailure).Scan(&failed, &last); err != nil {
		return nil, err
	}
	info["failedLoginCount"] = failed
	if last.Valid {
		info["lastFailedLoginTime"] = o.time(last.Time)
	}
	return info, nil
}

// restSession tells who is signed in.
func (s *Server) restSession(w http.ResponseWriter, r *http.Request) error {
	u := currentUser(r)
	o := s.restOut(r)
	info, err := s.restLoginInfo(r.Context(), o, u.ID)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, map[string]any{"self": o.api + "/user?username=" + url.QueryEscape(u.Username),
		"name": u.Username, "loginInfo": info})
}

// restSignIn signs in as POST /api/login does and answers with the session.
func (s *Server) restSignIn(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := readRESTJSON(r, &in); err != nil {
		return err
	}
	u, err := s.signIn(w, r, in.Username, in.Password, "session")
	if err != nil {
		var he *httpError
		if errors.As(err, &he) && he.status == http.StatusUnauthorized {
			w.Header().Set("X-Seraph-LoginReason", "AUTHENTICATED_FAILED")
		} else if he != nil && he.status == http.StatusTooManyRequests {
			w.Header().Set("X-Seraph-LoginReason", "AUTHENTICATION_DENIED")
		}
		return err
	}
	var token string
	for _, c := range (&http.Response{Header: w.Header()}).Cookies() {
		if c.Name == auth.CookieName {
			token = c.Value
		}
	}
	info, err := s.restLoginInfo(r.Context(), s.restOut(r), u.ID)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, map[string]any{
		"session": map[string]string{"name": auth.CookieName, "value": token}, "loginInfo": info})
}

// restSignOut signs out like the interface's logout.
func (s *Server) restSignOut(w http.ResponseWriter, r *http.Request) error {
	if err := s.signOut(w, r); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// Groups -----------------------------------------------------------------------

// restGroupPicker finds groups by part of their name. As in the interface,
// administrators see every group; the others only those they belong to.
func (s *Server) restGroupPicker(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	u := currentUser(r)
	q := r.URL.Query()
	_, limit := paging(r, 20, 1000)
	query := strings.TrimSpace(q.Get("query"))

	where, args := `WHERE g.name LIKE ?`, []any{"%" + escapeLike(strings.ToLower(query)) + "%"}
	if exclude := q["exclude"]; len(exclude) > 0 {
		where += ` AND g.name NOT IN (` + sqlq.Marks(len(exclude)) + `)`
		for _, name := range exclude {
			args = append(args, strings.TrimSpace(name))
		}
	}
	if !u.Admin {
		where += ` AND g.id IN (SELECT group_id FROM group_members WHERE user_id = ?)`
		args = append(args, u.ID)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_groups g `+where, args...).Scan(&total); err != nil {
		return err
	}
	groups := []any{}
	if err := sqlq.Each(ctx, s.db, `SELECT g.name FROM user_groups g `+where+` ORDER BY g.name LIMIT ?`,
		func(rows *sql.Rows) error {
			var name string
			if err := rows.Scan(&name); err != nil {
				return err
			}
			groups = append(groups, map[string]any{"name": name, "html": highlight(name, query), "labels": []any{}})
			return nil
		}, append(args, limit)...); err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, map[string]any{
		"header": s.restOut(r).msg(i18n.M(i18n.GroupPickerHeader, len(groups), total)), "total": total, "groups": groups})
}

// highlight is a name in HTML with the first occurrence of part of it in bold.
func highlight(name, part string) string {
	lower, needle := strings.ToLower(name), strings.ToLower(part)
	i := strings.Index(lower, needle)
	// Lower-casing some letters changes their length: the offsets would not fit.
	if needle == "" || i < 0 || len(lower) != len(name) {
		return html.EscapeString(name)
	}
	j := i + len(needle)
	return html.EscapeString(name[:i]) + "<b>" + html.EscapeString(name[i:j]) + "</b>" + html.EscapeString(name[j:])
}

// restGroupMembers lists the users of a group. As in the interface, only
// administrators see who belongs to a group.
func (s *Server) restGroupMembers(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	ctx := r.Context()
	q := r.URL.Query()
	name := strings.TrimSpace(q.Get("groupname"))
	if name == "" {
		return fieldError("groupname", failf(http.StatusBadRequest, i18n.FieldRequired, "groupname"))
	}
	var group int64
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM user_groups WHERE LOWER(name) = LOWER(?)`, name).Scan(&group); errors.Is(err, sql.ErrNoRows) {
		return fail(http.StatusNotFound, i18n.GroupNotFound)
	} else if err != nil {
		return err
	}
	inactive := q.Get("includeInactiveUsers") == "true"
	startAt, maxResults := paging(r, 50, 50)
	from := `FROM group_members m JOIN users u ON u.id = m.user_id WHERE m.group_id = ? AND (u.active OR ?)`
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) `+from, group, inactive).Scan(&total); err != nil {
		return err
	}
	var ids []int64
	if err := sqlq.Each(ctx, s.db, `SELECT u.id `+from+` ORDER BY u.username LIMIT ? OFFSET ?`, func(rows *sql.Rows) error {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
		return nil
	}, group, inactive, maxResults, startAt); err != nil {
		return err
	}
	o := s.restOut(r)
	users, err := s.jiraUsers(ctx, o, ids...)
	if err != nil {
		return err
	}
	values := []any{}
	for _, id := range ids {
		values = append(values, users[id])
	}
	page := func(at int) string {
		return o.api + "/group/member?" + url.Values{"groupname": {name}, "includeInactiveUsers": {strconv.FormatBool(inactive)},
			"startAt": {strconv.Itoa(at)}, "maxResults": {strconv.Itoa(maxResults)}}.Encode()
	}
	out := map[string]any{"self": page(startAt), "maxResults": maxResults, "startAt": startAt, "total": total,
		"isLast": startAt+len(ids) >= total, "values": values}
	if startAt+len(ids) < total {
		out["nextPage"] = page(startAt + maxResults)
	}
	return writeJSON(w, http.StatusOK, out)
}
