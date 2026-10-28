package api

// Editing and deleting work logs. Authors change their own (when they may
// still log work); those who may moderate a project change anyone's, as
// with comments. The issue's time spent and remaining estimate follow, and
// the change is written to its history and announced.

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"solstice/internal/events"
	"solstice/internal/i18n"
	"solstice/internal/perm"
	"solstice/internal/sqlq"
)

// maxWorklog is the longest time one work log may hold: a week.
const maxWorklog = 7 * 24 * 3600

type editableWorklog struct {
	issue   *issueRow
	id      int64
	author  sql.NullInt64
	seconds int64
	started time.Time
	created time.Time
	body    sql.NullString
	group   string
	role    int64
}

// worklogForEdit loads the work log of the path, if the user may read and
// change it.
func (s *Server) worklogForEdit(r *http.Request, sub *perm.Subject) (*editableWorklog, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return nil, errNotFound
	}
	var issueID int64
	var group sql.NullString
	var role sql.NullInt64
	w := &editableWorklog{id: id}
	err = s.db.QueryRowContext(r.Context(), `SELECT issue_id, author_id, body, started_at, seconds, created_at,
		visibility_group, visibility_role_id FROM worklogs WHERE id = ?`, id).
		Scan(&issueID, &w.author, &w.body, &w.started, &w.seconds, &w.created, &group, &role)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}
	w.group, w.role = group.String, role.Int64
	if w.issue, err = s.findIssueByID(r.Context(), issueID); err != nil {
		return nil, err
	}
	pi := w.issue.perm()
	if !sub.CanSee(pi) || !sub.SeesRestricted(w.issue.ProjectID, w.group, w.role) {
		return nil, errNotFound
	}
	own := w.author.Valid && w.author.Int64 == sub.UserID && sub.Can(perm.Worklog, w.issue.ProjectID, &pi)
	if !own && !sub.Can(perm.Moderate, w.issue.ProjectID, &pi) {
		return nil, errForbidden
	}
	return w, nil
}

// event describes the work log as it is in an announcement.
func (w *editableWorklog) event() *events.Worklog {
	return &events.Worklog{ID: w.id, AuthorID: w.author.Int64, Body: w.body.String, Group: w.group, RoleID: w.role,
		Started: w.started, Seconds: w.seconds, Created: w.created}
}

// editWorklog changes a work log's day, time or text; what the request
// leaves out stays as it is. The difference in time moves the issue's time
// spent, and its remaining estimate the other way.
func (s *Server) editWorklog(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	wl, err := s.worklogForEdit(r, sub)
	if err != nil {
		return err
	}
	var in struct {
		Started *time.Time `json:"started"`
		Seconds *int64     `json:"seconds"`
		Body    *string    `json:"body"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	next := *wl
	if in.Seconds != nil {
		if *in.Seconds <= 0 || *in.Seconds > maxWorklog {
			return fail(http.StatusBadRequest, i18n.WorkLogDurationInvalid)
		}
		next.seconds = *in.Seconds
	}
	if in.Started != nil {
		next.started = in.Started.UTC()
	}
	if in.Body != nil {
		next.body = sql.NullString{String: *in.Body, Valid: strings.TrimSpace(*in.Body) != ""}
	}
	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE worklogs SET body = ?, started_at = ?, seconds = ?, updated_at = ?,
		updated_by_id = ? WHERE id = ?`, sqlq.Value(next.body), next.started, next.seconds, now, sub.UserID, wl.id); err != nil {
		return err
	}
	cur, err := lockTimeTracking(ctx, tx, wl.issue.ID)
	if err != nil {
		return err
	}
	changes, err := storeTimeTracking(ctx, tx, wl.issue.ID, sub.UserID, now, cur, cur.withLogged(next.seconds-wl.seconds))
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	e := issueEvent(events.WorklogUpdated, wl.issue, sub.UserID, "worklog")
	e.Worklog = next.event()
	e.Worklog.UpdatedBy, e.Worklog.Updated = sub.UserID, now
	e.Changes = eventChanges(changes)
	s.emit(e)

	list, err := s.visibleWorklogs(ctx, sub, wl.issue)
	if err != nil {
		return err
	}
	for _, x := range list {
		if x.ID == wl.id {
			return writeJSON(w, http.StatusOK, x)
		}
	}
	return errNotFound
}

// deleteWorklog removes a work log: the time it held is no longer spent,
// and the remaining estimate grows back by as much.
func (s *Server) deleteWorklog(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	wl, err := s.worklogForEdit(r, sub)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM worklogs WHERE id = ?`, wl.id); err != nil {
		return err
	}
	cur, err := lockTimeTracking(ctx, tx, wl.issue.ID)
	if err != nil {
		return err
	}
	changes, err := storeTimeTracking(ctx, tx, wl.issue.ID, sub.UserID, now, cur, cur.withLogged(-wl.seconds))
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	e := issueEvent(events.WorklogDeleted, wl.issue, sub.UserID, "worklog")
	e.Worklog = wl.event()
	e.Changes = eventChanges(changes)
	s.emit(e)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

type worklogOut struct {
	ID         int64       `json:"id"`
	Author     *userRef    `json:"author"`
	Started    time.Time   `json:"started"`
	Seconds    int64       `json:"seconds"`
	Body       *string     `json:"body"`
	Visibility *visibility `json:"visibility"`
	CanEdit    bool        `json:"canEdit"`
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
	out, err := s.visibleWorklogs(r.Context(), sub, iss)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

// visibleWorklogs returns an issue's work logs that the user may read, and
// tells for each whether the user may edit or delete it.
func (s *Server) visibleWorklogs(ctx context.Context, sub *perm.Subject, iss *issueRow) ([]worklogOut, error) {
	type row struct {
		out    worklogOut
		author sql.NullInt64
	}
	var list []row
	var ids []int64
	err := sqlq.Each(ctx, s.db, `SELECT w.id, w.author_id, w.started_at, w.seconds, w.body, w.visibility_group,
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
		return nil, err
	}
	users, err := s.users(ctx, ids...)
	if err != nil {
		return nil, err
	}
	pi := iss.perm()
	moderate := sub.Can(perm.Moderate, iss.ProjectID, &pi)
	own := sub.Can(perm.Worklog, iss.ProjectID, &pi)
	out := make([]worklogOut, 0, len(list))
	for _, x := range list {
		x.out.Author = refOf(users, x.author)
		x.out.CanEdit = moderate || (own && x.author.Valid && x.author.Int64 == sub.UserID)
		out = append(out, x.out)
	}
	return out, nil
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
	if in.Seconds <= 0 || in.Seconds > maxWorklog {
		return fail(http.StatusBadRequest, i18n.WorkLogDurationInvalid)
	}
	now := time.Now().UTC()
	started := now
	if in.Started != nil {
		started = in.Started.UTC()
	}
	body := sqlq.NullIfBlank(in.Body)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Lock the issue before the insert: the foreign key check would take a
	// shared lock first, and two requests upgrading it deadlock.
	cur, err := lockTimeTracking(ctx, tx, iss.ID)
	if err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO worklogs (issue_id, author_id, body, started_at, seconds, created_at,
		updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, iss.ID, sub.UserID, body, started, in.Seconds, now, now)
	if err != nil {
		return err
	}
	worklogID, _ := res.LastInsertId()
	// Logged time is spent time; the remaining estimate shrinks accordingly.
	changes, err := storeTimeTracking(ctx, tx, iss.ID, sub.UserID, now, cur, cur.withLogged(in.Seconds))
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	e := issueEvent(events.WorklogCreated, iss, sub.UserID, "worklog")
	e.Worklog = &events.Worklog{ID: worklogID, AuthorID: sub.UserID, Body: in.Body, Started: started,
		Seconds: in.Seconds, Created: now}
	e.Changes = eventChanges(changes)
	s.emit(e)
	return writeJSON(w, http.StatusCreated, map[string]int64{"id": worklogID})
}
