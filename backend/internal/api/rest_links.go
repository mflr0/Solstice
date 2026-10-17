package api

// Remote links, Jira's session route and groups of the REST API.

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

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
