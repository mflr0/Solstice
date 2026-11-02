package api

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

type visibility struct {
	Group string `json:"group,omitempty"`
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
}

// visibleComments returns an issue's comments that the user may read.
func (s *Server) visibleComments(ctx context.Context, sub *perm.Subject, iss *issueRow) ([]commentRow, error) {
	var out []commentRow
	err := sqlq.Each(ctx, s.db, `SELECT c.id, c.author_id, c.updated_by_id, c.body, c.created_at, c.updated_at,
		c.visibility_group FROM comments c
		WHERE c.issue_id = ? ORDER BY c.created_at, c.id`, func(rows *sql.Rows) error {
		var c commentRow
		if err := rows.Scan(&c.id, &c.author, &c.updatedBy, &c.body, &c.created, &c.updated, &c.group); err != nil {
			return err
		}
		if sub.SeesRestricted(iss.ProjectID, c.group.String) {
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
		if c.group.Valid {
			o.Visibility = &visibility{Group: c.group.String}
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
	var group any
	var eventGroup string
	if v := in.Visibility; v != nil && v.Group != "" {
		// One can only restrict a comment to a group one is in, in the
		// project, otherwise one could not read one's own comment.
		eventGroup = strings.ToLower(v.Group)
		if !sub.SeesRestricted(iss.ProjectID, eventGroup) {
			return fail(http.StatusBadRequest, i18n.CommentGroupNotMember)
		}
		group = eventGroup
	}
	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `INSERT INTO comments (issue_id, author_id, body, created_at, updated_at,
		visibility_group) VALUES (?, ?, ?, ?, ?, ?)`,
		iss.ID, sub.UserID, in.Body, now, now, group)
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
		Body: in.Body, Group: eventGroup, Created: now, Updated: now}))
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
	var body string
	var created time.Time
	err = s.db.QueryRowContext(r.Context(), `SELECT issue_id, author_id, visibility_group, body,
		created_at FROM comments WHERE id = ?`, id).Scan(&issueID, &author, &group, &body, &created)
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
	if !sub.CanSee(pi) || !sub.SeesRestricted(iss.ProjectID, group.String) {
		return nil, errNotFound
	}
	own := author.Valid && author.Int64 == sub.UserID && sub.Can(perm.Comment, iss.ProjectID, &pi)
	if !own && !sub.Can(perm.Moderate, iss.ProjectID, &pi) {
		return nil, errForbidden
	}
	return &editableComment{issue: iss, id: id, group: group.String, author: author.Int64, body: body,
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
		UpdatedBy: sub.UserID, Body: in.Body, OldBody: c.body, Group: c.group, Created: c.created,
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
		Body: c.body, Group: c.group, Created: c.created, Updated: c.created}))
	w.WriteHeader(http.StatusNoContent)
	return nil
}
