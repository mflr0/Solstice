package api

// Editing and deleting work logs. Authors change their own (when they may
// still log work); those who may moderate a project change anyone's, as
// with comments. The issue's time spent and remaining estimate follow, and
// the change is written to its history and announced.

import (
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

// editableWorklog is a work log the current user may edit or delete.
type editableWorklog struct {
	issue   *issueRow
	id      int64
	author  sql.NullInt64
	seconds int64
	started time.Time
	created time.Time
	body    sql.NullString
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
	if w.issue, err = s.findIssueByID(r.Context(), issueID); err != nil {
		return nil, err
	}
	pi := w.issue.perm()
	if !sub.CanSee(pi) || !sub.SeesRestricted(w.issue.ProjectID, group.String, role.Int64) {
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
	return &events.Worklog{ID: w.id, AuthorID: w.author.Int64, Body: w.body.String, Started: w.started,
		Seconds: w.seconds, Created: w.created}
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
