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

	"github.com/go-sql-driver/mysql"

	"solstice/internal/audit"
	"solstice/internal/auth"
	"solstice/internal/i18n"
	"solstice/internal/perm"
	"solstice/internal/sqlq"
)

// Remote links -----------------------------------------------------------------

// remoteLinkIn is a remote link as Jira clients write one. Solstice keeps
// the globalId, the address, the title and the relationship; the rest
// (application, icon, summary, status) is accepted and ignored.
type remoteLinkIn struct {
	GlobalID     string `json:"globalId"`
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
	in.Relationship, in.GlobalID = strings.TrimSpace(in.Relationship), strings.TrimSpace(in.GlobalID)
	switch {
	case in.Object.URL == "":
		return fieldError("object.url", failf(http.StatusBadRequest, i18n.FieldRequired, "object.url"))
	case !remoteLinkOK(in.Object.URL):
		return invalidValue("object.url")
	case len(in.Object.URL) > 2000: // length in bytes, not characters
		return fieldError("object.url", failf(http.StatusBadRequest, i18n.FieldTooLong, 2000))
	case in.Object.Title == "":
		return fieldError("object.title", failf(http.StatusBadRequest, i18n.FieldRequired, "object.title"))
	case len([]rune(in.Object.Title)) > 255:
		return fieldError("object.title", failf(http.StatusBadRequest, i18n.FieldTooLong, 255))
	case len([]rune(in.Relationship)) > 255:
		return fieldError("relationship", failf(http.StatusBadRequest, i18n.FieldTooLong, 255))
	case len([]rune(in.GlobalID)) > 255:
		return fieldError("globalId", failf(http.StatusBadRequest, i18n.FieldTooLong, 255))
	}
	return nil
}

// remoteLinkRow is a stored remote link with its globalId.
type remoteLinkRow struct {
	remoteLink
	GlobalID string
}

const remoteLinkColumns = `id, url, title, COALESCE(relationship, ''), COALESCE(global_id, '')`

func (v *remoteLinkRow) scan(rows interface{ Scan(...any) error }) error {
	return rows.Scan(&v.ID, &v.URL, &v.Title, &v.Relationship, &v.GlobalID)
}

func (o *restOut) remoteLink(key string, v remoteLinkRow) map[string]any {
	out := map[string]any{"id": v.ID, "self": o.api + "/issue/" + key + "/remotelink/" + o.id(v.ID),
		"object": map[string]any{"url": v.URL, "title": v.Title}}
	if v.GlobalID != "" {
		out["globalId"] = v.GlobalID
	}
	if v.Relationship != "" {
		out["relationship"] = v.Relationship
	}
	return out
}

// remoteLinkText is how the history names a remote link, as Jira writes it.
// The English wording is Jira's on purpose, not a message to translate.
func remoteLinkText(title string) string { return `This issue links to "` + title + `"` }

// restLinkableIssue returns the issue of the path, which the user must be
// allowed to link, as Jira asks for "Link issues" on remote links.
func (s *Server) restLinkableIssue(r *http.Request) (*perm.Subject, *issueRow, error) {
	sub, iss, err := s.restSubjectIssue(r)
	if err != nil {
		return nil, nil, err
	}
	pi := iss.perm()
	if !sub.Can(perm.Link, iss.ProjectID, &pi) {
		return nil, nil, errForbidden
	}
	return sub, iss, nil
}

// pathRemoteLink loads the remote link of iss that the request names: by the
// id of the path, or on the routes without one by the globalId of the query.
func (s *Server) pathRemoteLink(r *http.Request, iss *issueRow) (remoteLinkRow, error) {
	var v remoteLinkRow
	where, arg := `id = ?`, any(nil)
	if raw := r.PathValue("linkId"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return v, errNotFound
		}
		arg = id
	} else if global := r.URL.Query().Get("globalId"); global != "" {
		where, arg = `global_id = ?`, global
	} else { // DELETE on the collection: Jira asks for the globalId
		return v, fieldError("globalId", failf(http.StatusBadRequest, i18n.FieldRequired, "globalId"))
	}
	err := v.scan(s.db.QueryRowContext(r.Context(), `SELECT `+remoteLinkColumns+` FROM remote_links
		WHERE `+where+` AND issue_id = ?`, arg, iss.ID))
	if errors.Is(err, sql.ErrNoRows) {
		return v, errNotFound
	}
	return v, err
}

func (s *Server) restRemoteLinks(w http.ResponseWriter, r *http.Request) error {
	_, iss, err := s.restSubjectIssue(r)
	if err != nil {
		return err
	}
	o := s.restOut(r)
	if r.URL.Query().Get("globalId") != "" { // as Jira answers: the one link, not a list
		v, err := s.pathRemoteLink(r, iss)
		if err != nil {
			return err
		}
		return writeJSON(w, http.StatusOK, o.remoteLink(iss.Key(), v))
	}
	list := []any{}
	if err := sqlq.Each(r.Context(), s.db, `SELECT `+remoteLinkColumns+` FROM remote_links
		WHERE issue_id = ? ORDER BY id`, func(rows *sql.Rows) error {
		var v remoteLinkRow
		if err := v.scan(rows); err != nil {
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
	sub, iss, err := s.restLinkableIssue(r)
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
	// Two POSTs with the same new globalId at once both find nothing to update.
	// The one that loses fails on the unique key or as a deadlock victim; its
	// second try finds the link of the other and updates it, as Jira answers.
	var id int64
	var status int
	var changes []change
	for try := 0; ; try++ {
		id, status, changes, err = s.saveRemoteLink(ctx, iss, sub.UserID, in)
		if err == nil || try > 0 || !lostRace(err) {
			break
		}
	}
	if err != nil {
		return err
	}
	if len(changes) > 0 {
		s.changed(iss, sub.UserID, "link", changes)
	}
	o := s.restOut(r)
	return writeJSON(w, status, map[string]any{"id": id,
		"self": o.api + "/issue/" + iss.Key() + "/remotelink/" + o.id(id)})
}

// lostRace tells the errors of a request that met a concurrent one on the
// same globalId: a unique key violation (1062) or a deadlock (1213).
func lostRace(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && (me.Number == 1062 || me.Number == 1213)
}

// saveRemoteLink adds a remote link to an issue, or updates the one that has
// the globalId. It answers the id, 201 or 200, and the changes it recorded.
func (s *Server) saveRemoteLink(ctx context.Context, iss *issueRow, userID int64, in remoteLinkIn) (int64, int, []change, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, nil, err
	}
	defer tx.Rollback()
	// As in Jira, a globalId the issue already has makes this an update.
	var old remoteLinkRow
	updating := false
	if in.GlobalID != "" {
		// A plain read, then a lock on the row found: a locking read of a missing
		// key locks its gap, and requests that all find nothing would deadlock
		// on their inserts.
		err := old.scan(tx.QueryRowContext(ctx, `SELECT `+remoteLinkColumns+` FROM remote_links
			WHERE issue_id = ? AND global_id = ?`, iss.ID, in.GlobalID))
		if err == nil {
			err = old.scan(tx.QueryRowContext(ctx, `SELECT `+remoteLinkColumns+` FROM remote_links WHERE id = ? FOR UPDATE`, old.ID))
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return 0, 0, nil, err
		}
		updating = err == nil
	}
	id, status := old.ID, http.StatusOK
	var changes []change
	if updating {
		if changes, err = updateRemoteLink(ctx, tx, old, in); err != nil {
			return 0, 0, nil, err
		}
	} else {
		res, err := tx.ExecContext(ctx, `INSERT INTO remote_links (issue_id, url, title, relationship, global_id) VALUES (?, ?, ?, ?, ?)`,
			iss.ID, in.Object.URL, in.Object.Title, nullIfEmpty(in.Relationship), nullIfEmpty(in.GlobalID))
		if err != nil {
			return 0, 0, nil, err
		}
		if id, err = res.LastInsertId(); err != nil {
			return 0, 0, nil, err
		}
		status = http.StatusCreated
		changes = []change{{field: "RemoteIssueLink", newValue: id, newString: remoteLinkText(in.Object.Title)}}
	}
	if err := recordChanges(ctx, tx, iss.ID, userID, time.Now().UTC(), changes); err != nil {
		return 0, 0, nil, err
	}
	return id, status, changes, tx.Commit()
}

// updateRemoteLink replaces the address, title and relationship of a link and
// returns the change to record, none when nothing differs. The globalId stays.
func updateRemoteLink(ctx context.Context, tx *sql.Tx, old remoteLinkRow, in remoteLinkIn) ([]change, error) {
	if _, err := tx.ExecContext(ctx, `UPDATE remote_links SET url = ?, title = ?, relationship = ? WHERE id = ?`,
		in.Object.URL, in.Object.Title, nullIfEmpty(in.Relationship), old.ID); err != nil {
		return nil, err
	}
	if in.Object.URL == old.URL && in.Object.Title == old.Title && in.Relationship == old.Relationship {
		return nil, nil
	}
	return []change{{field: "RemoteIssueLink", oldValue: old.ID, oldString: remoteLinkText(old.Title),
		newValue: old.ID, newString: remoteLinkText(in.Object.Title)}}, nil
}

func (s *Server) restEditRemoteLink(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, iss, err := s.restLinkableIssue(r)
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
	if in.GlobalID != "" && in.GlobalID != old.GlobalID { // the stored one stays: say so, do not ignore it
		return invalidValue("globalId")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	changes, err := updateRemoteLink(ctx, tx, old, in)
	if err != nil {
		return err
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
	sub, iss, err := s.restLinkableIssue(r)
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
		if len(exclude) > 100 {
			return invalidValue("exclude")
		}
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
