package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"regexp"

	"solstice/internal/events"
	"solstice/internal/i18n"
	"solstice/internal/perm"
	"solstice/internal/sqlq"
	"strconv"
	"strings"
	"time"
)

// Comments -------------------------------------------------------------------

type visibility struct {
	Group string `json:"group,omitempty"`
	Role  string `json:"role,omitempty"`
}

type commentOut struct {
	ID         int64       `json:"id"`
	Author     *userRef    `json:"author"`
	Body       string      `json:"body"`
	Created    time.Time   `json:"created"`
	Updated    time.Time   `json:"updated"`
	UpdatedBy  *userRef    `json:"updatedBy"`
	Visibility *visibility `json:"visibility"`
	CanEdit    bool        `json:"canEdit"`
}

type commentRow struct {
	id                int64
	author, updatedBy sql.NullInt64
	body              string
	created, updated  time.Time
	group             sql.NullString
	roleID            sql.NullInt64
	roleName          sql.NullString
}

// visibleComments returns an issue's comments that the user may read.
func (s *Server) visibleComments(ctx context.Context, sub *perm.Subject, iss *issueRow) ([]commentRow, error) {
	var out []commentRow
	err := sqlq.Each(ctx, s.db, `SELECT c.id, c.author_id, c.updated_by_id, c.body, c.created_at, c.updated_at,
		c.visibility_group, c.visibility_role_id, r.name FROM comments c
		LEFT JOIN project_roles r ON r.id = c.visibility_role_id
		WHERE c.issue_id = ? ORDER BY c.created_at, c.id`, func(rows *sql.Rows) error {
		var c commentRow
		if err := rows.Scan(&c.id, &c.author, &c.updatedBy, &c.body, &c.created, &c.updated, &c.group, &c.roleID,
			&c.roleName); err != nil {
			return err
		}
		if sub.SeesRestricted(iss.ProjectID, c.group.String, c.roleID.Int64) {
			out = append(out, c)
		}
		return nil
	}, iss.ID)
	return out, err
}

func (s *Server) commentsOut(ctx context.Context, sub *perm.Subject, iss *issueRow, rows []commentRow) ([]commentOut, error) {
	var ids []int64
	for _, c := range rows {
		ids = append(ids, c.author.Int64, c.updatedBy.Int64)
	}
	users, err := s.users(ctx, ids...)
	if err != nil {
		return nil, err
	}
	pi := iss.perm()
	moderate := sub.Can(perm.Moderate, iss.ProjectID, &pi)
	comment := sub.Can(perm.Comment, iss.ProjectID, &pi)
	out := make([]commentOut, 0, len(rows))
	for _, c := range rows {
		o := commentOut{ID: c.id, Author: refOf(users, c.author), Body: c.body, Created: c.created, Updated: c.updated,
			UpdatedBy: refOf(users, c.updatedBy), CanEdit: moderate || (comment && c.author.Int64 == sub.UserID)}
		if c.group.Valid || c.roleID.Valid {
			o.Visibility = &visibility{Group: c.group.String, Role: c.roleName.String}
		}
		out = append(out, o)
	}
	return out, nil
}

func (s *Server) listComments(w http.ResponseWriter, r *http.Request) error {
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	iss, err := s.visibleIssue(r, sub)
	if err != nil {
		return err
	}
	rows, err := s.visibleComments(r.Context(), sub, iss)
	if err != nil {
		return err
	}
	out, err := s.commentsOut(r.Context(), sub, iss, rows)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

type commentInput struct {
	Body       string `json:"body"`
	Visibility *struct {
		Group string `json:"group"`
		Role  int64  `json:"role"`
	} `json:"visibility"`
}

func (s *Server) addComment(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	iss, err := s.visibleIssue(r, sub)
	if err != nil {
		return err
	}
	pi := iss.perm()
	if !sub.Can(perm.Comment, iss.ProjectID, &pi) {
		return errForbidden
	}
	var in commentInput
	if err := readJSON(r, &in); err != nil {
		return err
	}
	if strings.TrimSpace(in.Body) == "" {
		return fail(http.StatusBadRequest, i18n.CommentEmpty)
	}
	var group, role any
	var eventGroup string
	var eventRole int64
	if v := in.Visibility; v != nil {
		// One can only restrict a comment to a group or role one belongs to,
		// otherwise one could not read one's own comment.
		if v.Group != "" {
			if !sub.SeesRestricted(iss.ProjectID, strings.ToLower(v.Group), 0) {
				return fail(http.StatusBadRequest, i18n.CommentGroupNotMember)
			}
			group = strings.ToLower(v.Group)
			eventGroup = strings.ToLower(v.Group)
		}
		if v.Role != 0 {
			if !sub.SeesRestricted(iss.ProjectID, "", v.Role) {
				return fail(http.StatusBadRequest, i18n.CommentRoleNotHeld)
			}
			role = v.Role
			eventRole = v.Role
		}
	}
	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `INSERT INTO comments (issue_id, author_id, body, created_at, updated_at,
		visibility_group, visibility_role_id) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		iss.ID, sub.UserID, in.Body, now, now, group, role)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE issues SET updated_at = ? WHERE id = ?`, now, iss.ID); err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	if err := tx.Commit(); err != nil {
		return err
	}
	s.emit(commentEvent(events.CommentCreated, iss, sub.UserID, &events.Comment{ID: id, AuthorID: sub.UserID,
		Body: in.Body, Group: eventGroup, RoleID: eventRole, Created: now, Updated: now}))
	return s.respondComment(w, r, sub, iss, id, http.StatusCreated)
}

func (s *Server) respondComment(w http.ResponseWriter, r *http.Request, sub *perm.Subject, iss *issueRow, id int64, status int) error {
	rows, err := s.visibleComments(r.Context(), sub, iss)
	if err != nil {
		return err
	}
	for _, c := range rows {
		if c.id == id {
			out, err := s.commentsOut(r.Context(), sub, iss, []commentRow{c})
			if err != nil {
				return err
			}
			return writeJSON(w, status, out[0])
		}
	}
	return errNotFound
}

// editableComment is a comment the current user may edit or delete.
type editableComment struct {
	issue   *issueRow
	id      int64
	group   string
	role    int64
	author  int64
	body    string
	created time.Time
}

// commentForEdit loads a comment the user may edit or delete.
func (s *Server) commentForEdit(r *http.Request, sub *perm.Subject) (*editableComment, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return nil, errNotFound
	}
	var issueID int64
	var author sql.NullInt64
	var group sql.NullString
	var role sql.NullInt64
	var body string
	var created time.Time
	err = s.db.QueryRowContext(r.Context(), `SELECT issue_id, author_id, visibility_group, visibility_role_id, body,
		created_at FROM comments WHERE id = ?`, id).Scan(&issueID, &author, &group, &role, &body, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}
	iss, err := s.findIssueByID(r.Context(), issueID)
	if err != nil {
		return nil, err
	}
	pi := iss.perm()
	if !sub.CanSee(pi) || !sub.SeesRestricted(iss.ProjectID, group.String, role.Int64) {
		return nil, errNotFound
	}
	own := author.Valid && author.Int64 == sub.UserID && sub.Can(perm.Comment, iss.ProjectID, &pi)
	if !own && !sub.Can(perm.Moderate, iss.ProjectID, &pi) {
		return nil, errForbidden
	}
	return &editableComment{issue: iss, id: id, group: group.String, role: role.Int64, author: author.Int64, body: body,
		created: created}, nil
}

func (s *Server) editComment(w http.ResponseWriter, r *http.Request) error {
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	c, err := s.commentForEdit(r, sub)
	if err != nil {
		return err
	}
	var in struct {
		Body string `json:"body"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	if strings.TrimSpace(in.Body) == "" {
		return fail(http.StatusBadRequest, i18n.CommentEmpty)
	}
	now := time.Now().UTC()
	if _, err := s.db.ExecContext(r.Context(), `UPDATE comments SET body = ?, updated_at = ?, updated_by_id = ?
		WHERE id = ?`, in.Body, now, sub.UserID, c.id); err != nil {
		return err
	}
	s.emit(commentEvent(events.CommentUpdated, c.issue, sub.UserID, &events.Comment{ID: c.id, AuthorID: c.author,
		UpdatedBy: sub.UserID, Body: in.Body, OldBody: c.body, Group: c.group, RoleID: c.role, Created: c.created,
		Updated: now}))
	return s.respondComment(w, r, sub, c.issue, c.id, http.StatusOK)
}

func (s *Server) deleteComment(w http.ResponseWriter, r *http.Request) error {
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	c, err := s.commentForEdit(r, sub)
	if err != nil {
		return err
	}
	if _, err := s.db.ExecContext(r.Context(), `DELETE FROM comments WHERE id = ?`, c.id); err != nil {
		return err
	}
	s.emit(commentEvent(events.CommentDeleted, c.issue, sub.UserID, &events.Comment{ID: c.id, AuthorID: c.author,
		Body: c.body, Group: c.group, RoleID: c.role, Created: c.created, Updated: c.created}))
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// History --------------------------------------------------------------------

type historyItem struct {
	Field     string  `json:"field"`
	FieldType string  `json:"fieldType"`
	From      *string `json:"from"`
	To        *string `json:"to"`
	// Raw values (ids) and display strings apart, for Jira's changelog.
	fromValue, fromString, toValue, toString *string
}

type historyEntry struct {
	ID       int64         `json:"id"`
	Author   *userRef      `json:"author"`
	Created  time.Time     `json:"created"`
	Items    []historyItem `json:"items"`
	authorID sql.NullInt64
}

func (s *Server) history(w http.ResponseWriter, r *http.Request) error {
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	iss, err := s.visibleIssue(r, sub)
	if err != nil {
		return err
	}
	entries, err := s.issueHistory(r.Context(), sub, iss)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, entries)
}

// issueHistory returns the history of an issue, oldest first, without what
// the user may not read. Fields whose old and new values may reveal
// restricted information are not shown to everyone. (Jira hides nothing
// here; Solstice errs on the safe side only for the security level, whose
// ids mean nothing to readers anyway.)
func (s *Server) issueHistory(ctx context.Context, sub *perm.Subject, iss *issueRow) ([]historyEntry, error) {
	entries := []historyEntry{}
	index := map[int64]int{}
	var authors []int64
	err := sqlq.Each(ctx, s.db, `SELECT g.id, g.author_id, g.created_at, i.field_type, i.field,
		i.old_value, i.old_string, i.new_value, i.new_string
		FROM change_groups g JOIN change_items i ON i.group_id = g.id
		WHERE g.issue_id = ? ORDER BY g.created_at, g.id, i.id`, func(rows *sql.Rows) error {
		var id int64
		var author sql.NullInt64
		var created time.Time
		var it historyItem
		var fromValue, fromString, toValue, toString sql.NullString
		if err := rows.Scan(&id, &author, &created, &it.FieldType, &it.Field, &fromValue, &fromString, &toValue,
			&toString); err != nil {
			return err
		}
		if it.Field == "security" {
			return nil
		}
		it.fromValue, it.fromString, it.toValue, it.toString = sqlq.String(fromValue), sqlq.String(fromString),
			sqlq.String(toValue), sqlq.String(toString)
		it.From, it.To = it.fromString, it.toString
		if it.From == nil {
			it.From = it.fromValue
		}
		if it.To == nil {
			it.To = it.toValue
		}
		i, ok := index[id]
		if !ok {
			i = len(entries)
			index[id] = i
			entries = append(entries, historyEntry{ID: id, Created: created, authorID: author})
			authors = append(authors, author.Int64)
		}
		entries[i].Items = append(entries[i].Items, it)
		return nil
	}, iss.ID)
	if err != nil {
		return nil, err
	}
	entries = s.hideRestrictedHistory(ctx, sub, entries)
	users, err := s.users(ctx, authors...)
	if err != nil {
		return nil, err
	}
	for i := range entries {
		entries[i].Author = refOf(users, entries[i].authorID)
	}
	return entries, nil
}

// History lines that name other issues: "This issue blocks RH-12".
var relationFields = map[string]bool{
	"link": true, "parent": true, "parent issue": true, "parent link": true, "epic link": true, "epic child": true,
}

// Work log lines identify individual work logs, which may be restricted.
var worklogFields = map[string]bool{"worklogid": true, "worklogtimespent": true}

var keyInText = regexp.MustCompile(`\b[A-Z][A-Z0-9_]+-\d+\b`)

// hideRestrictedHistory drops history lines that would reveal an issue the
// user may not see, or an individual (possibly restricted) work log.
func (s *Server) hideRestrictedHistory(ctx context.Context, sub *perm.Subject, entries []historyEntry) []historyEntry {
	visible := map[string]bool{}
	canSee := func(key string) bool {
		if v, ok := visible[key]; ok {
			return v
		}
		iss, err := s.findIssue(ctx, key)
		v := err == nil && sub.CanSee(iss.perm())
		visible[key] = v
		return v
	}
	out := entries[:0]
	for _, e := range entries {
		items := e.Items[:0]
		for _, it := range e.Items {
			field := strings.ToLower(it.Field)
			if worklogFields[field] {
				continue
			}
			if relationFields[field] {
				ok := true
				for _, text := range []*string{it.From, it.To} {
					if text == nil {
						continue
					}
					for _, key := range keyInText.FindAllString(*text, -1) {
						ok = ok && canSee(key)
					}
				}
				if !ok {
					continue
				}
			}
			items = append(items, it)
		}
		if len(items) > 0 {
			e.Items = items
			out = append(out, e)
		}
	}
	return out
}

// Work logs ------------------------------------------------------------------

type worklogOut struct {
	ID         int64       `json:"id"`
	Author     *userRef    `json:"author"`
	Started    time.Time   `json:"started"`
	Seconds    int64       `json:"seconds"`
	Body       *string     `json:"body"`
	Visibility *visibility `json:"visibility"`
}

func (s *Server) listWorklogs(w http.ResponseWriter, r *http.Request) error {
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	iss, err := s.visibleIssue(r, sub)
	if err != nil {
		return err
	}
	type row struct {
		out    worklogOut
		author sql.NullInt64
	}
	var list []row
	var ids []int64
	err = sqlq.Each(r.Context(), s.db, `SELECT w.id, w.author_id, w.started_at, w.seconds, w.body, w.visibility_group,
		w.visibility_role_id, pr.name FROM worklogs w LEFT JOIN project_roles pr ON pr.id = w.visibility_role_id
		WHERE w.issue_id = ? ORDER BY w.started_at, w.id`, func(rows *sql.Rows) error {
		var x row
		var body, group, roleName sql.NullString
		var role sql.NullInt64
		if err := rows.Scan(&x.out.ID, &x.author, &x.out.Started, &x.out.Seconds, &body, &group, &role, &roleName); err != nil {
			return err
		}
		if !sub.SeesRestricted(iss.ProjectID, group.String, role.Int64) {
			return nil
		}
		x.out.Body = sqlq.String(body)
		if group.Valid || role.Valid {
			x.out.Visibility = &visibility{Group: group.String, Role: roleName.String}
		}
		list = append(list, x)
		ids = append(ids, x.author.Int64)
		return nil
	}, iss.ID)
	if err != nil {
		return err
	}
	users, err := s.users(r.Context(), ids...)
	if err != nil {
		return err
	}
	out := make([]worklogOut, 0, len(list))
	for _, x := range list {
		x.out.Author = refOf(users, x.author)
		out = append(out, x.out)
	}
	return writeJSON(w, http.StatusOK, out)
}

func (s *Server) addWorklog(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	iss, err := s.visibleIssue(r, sub)
	if err != nil {
		return err
	}
	pi := iss.perm()
	if !sub.Can(perm.Worklog, iss.ProjectID, &pi) {
		return errForbidden
	}
	var in struct {
		Started *time.Time `json:"started"`
		Seconds int64      `json:"seconds"`
		Body    string     `json:"body"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	if in.Seconds <= 0 || in.Seconds > 7*24*3600 {
		return fail(http.StatusBadRequest, i18n.WorkLogDurationInvalid)
	}
	now := time.Now().UTC()
	started := now
	if in.Started != nil {
		started = in.Started.UTC()
	}
	var body any
	if strings.TrimSpace(in.Body) != "" {
		body = in.Body
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `INSERT INTO worklogs (issue_id, author_id, body, started_at, seconds, created_at,
		updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, iss.ID, sub.UserID, body, started, in.Seconds, now, now)
	if err != nil {
		return err
	}
	worklogID, _ := res.LastInsertId()
	// Logged time is spent time; the remaining estimate shrinks accordingly.
	if _, err := tx.ExecContext(ctx, `UPDATE issues SET time_spent = COALESCE(time_spent, 0) + ?,
		remaining_estimate = CASE WHEN remaining_estimate IS NULL THEN NULL ELSE GREATEST(remaining_estimate - ?, 0) END,
		updated_at = ? WHERE id = ?`, in.Seconds, in.Seconds, now, iss.ID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	e := issueEvent(events.WorklogCreated, iss, sub.UserID, "worklog")
	e.Worklog = &events.Worklog{ID: worklogID, AuthorID: sub.UserID, Body: in.Body, Started: started,
		Seconds: in.Seconds, Created: now}
	s.emit(e)
	w.WriteHeader(http.StatusCreated)
	return nil
}

// Watching and voting --------------------------------------------------------

func (s *Server) setWatch(on bool) handler {
	return s.toggle("watchers", on)
}

func (s *Server) setVote(on bool) handler {
	return s.toggle("votes", on)
}

func (s *Server) toggle(table string, on bool) handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		sub, err := s.subject(r)
		if err != nil {
			return err
		}
		iss, err := s.visibleIssue(r, sub)
		if err != nil {
			return err
		}
		if on {
			_, err = s.db.ExecContext(r.Context(), `INSERT IGNORE INTO `+table+` (issue_id, user_id, created_at)
				VALUES (?, ?, ?)`, iss.ID, sub.UserID, time.Now().UTC())
		} else {
			_, err = s.db.ExecContext(r.Context(), `DELETE FROM `+table+` WHERE issue_id = ? AND user_id = ?`,
				iss.ID, sub.UserID)
		}
		if err != nil {
			return err
		}
		var c counter
		if err := s.db.QueryRowContext(r.Context(), `SELECT COUNT(*), COALESCE(SUM(user_id = ?), 0) FROM `+table+`
			WHERE issue_id = ?`, sub.UserID, iss.ID).Scan(&c.Count, &c.Me); err != nil {
			return err
		}
		return writeJSON(w, http.StatusOK, c)
	}
}

// Links ----------------------------------------------------------------------

func (s *Server) addLink(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	iss, err := s.visibleIssue(r, sub)
	if err != nil {
		return err
	}
	pi := iss.perm()
	if !sub.Can(perm.Link, iss.ProjectID, &pi) {
		return errForbidden
	}
	var in struct {
		Type      int64  `json:"type"`
		Direction string `json:"direction"`
		Target    string `json:"target"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	cat, err := s.catalogue(ctx)
	if err != nil {
		return err
	}
	lt, ok := cat.linkType[in.Type]
	if !ok {
		return fail(http.StatusBadRequest, i18n.LinkTypeUnknown)
	}
	target, err := s.findIssue(ctx, in.Target)
	if err != nil || !sub.CanSee(target.perm()) {
		return fail(http.StatusBadRequest, i18n.LinkTargetNotFound)
	}
	if target.ID == iss.ID {
		return fail(http.StatusBadRequest, i18n.LinkToItself)
	}
	source, dest := iss, target
	if in.Direction == "inward" {
		source, dest = target, iss
	}
	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO issue_links (type_id, source_id, target_id) VALUES (?, ?, ?)`,
		in.Type, source.ID, dest.ID); err != nil {
		return err
	}
	// Jira records links in the history of both issues.
	sourceChange := []change{{field: "Link", newValue: dest.Key(), newString: "This issue " + lt.Outward + " " + dest.Key()}}
	destChange := []change{{field: "Link", newValue: source.Key(), newString: "This issue " + lt.Inward + " " + source.Key()}}
	if err := recordChanges(ctx, tx, source.ID, sub.UserID, now, sourceChange); err != nil {
		return err
	}
	if err := recordChanges(ctx, tx, dest.ID, sub.UserID, now, destChange); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.changed(source, sub.UserID, "link", sourceChange)
	s.changed(dest, sub.UserID, "link", destChange)
	return s.respondIssue(w, r, sub, iss.ID)
}

func (s *Server) deleteLink(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return errNotFound
	}
	var sourceID, targetID, typeID int64
	if err := s.db.QueryRowContext(ctx, `SELECT source_id, target_id, type_id FROM issue_links WHERE id = ?`, id).
		Scan(&sourceID, &targetID, &typeID); errors.Is(err, sql.ErrNoRows) {
		return errNotFound
	} else if err != nil {
		return err
	}
	source, err := s.findIssueByID(ctx, sourceID)
	if err != nil {
		return err
	}
	target, err := s.findIssueByID(ctx, targetID)
	if err != nil {
		return err
	}
	sp, tp := source.perm(), target.perm()
	// Links are only shown between issues one sees: others do not exist
	// for the user.
	if !sub.CanSee(sp) || !sub.CanSee(tp) {
		return errNotFound
	}
	if !sub.Can(perm.Link, source.ProjectID, &sp) && !sub.Can(perm.Link, target.ProjectID, &tp) {
		return errForbidden
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM issue_links WHERE id = ?`, id); err != nil {
		return err
	}
	// Announced like the history lines of a new link, the other way round.
	var lt linkTypeRef
	if cat, err := s.catalogue(ctx); err == nil {
		lt = cat.linkType[typeID]
	}
	s.changed(source, sub.UserID, "link", []change{{field: "Link", oldValue: target.Key(),
		oldString: "This issue " + lt.Outward + " " + target.Key()}})
	s.changed(target, sub.UserID, "link", []change{{field: "Link", oldValue: source.Key(),
		oldString: "This issue " + lt.Inward + " " + source.Key()}})
	w.WriteHeader(http.StatusNoContent)
	return nil
}
