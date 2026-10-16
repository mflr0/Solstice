package api

// REST API v2: issues, their comments, work logs, watchers, votes,
// attachments and links, and searching.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/url"

	"solstice/internal/i18n"
	"solstice/internal/jql"
	"solstice/internal/perm"
	"solstice/internal/sqlq"
	"strconv"
	"strings"
)

// restIssue loads the issue named in the path, by key or id, if the user
// sees it.
func (s *Server) restIssue(r *http.Request, sub *perm.Subject) (*issueRow, error) {
	v := strings.TrimSpace(r.PathValue("key"))
	var iss *issueRow
	var err error
	if id, perr := strconv.ParseInt(v, 10, 64); perr == nil {
		iss, err = s.findIssueByID(r.Context(), id)
		if errors.Is(err, sql.ErrNoRows) {
			err = errNotFound
		}
	} else {
		iss, err = s.findIssue(r.Context(), v)
	}
	if err != nil {
		return nil, err
	}
	if !sub.CanSee(iss.perm()) {
		return nil, errNotFound
	}
	return iss, nil
}

// restSubjectIssue returns the permission subject and the issue of the path.
func (s *Server) restSubjectIssue(r *http.Request) (*perm.Subject, *issueRow, error) {
	sub, err := s.subject(r)
	if err != nil {
		return nil, nil, err
	}
	iss, err := s.restIssue(r, sub)
	return sub, iss, err
}

func (s *Server) restGetIssue(w http.ResponseWriter, r *http.Request) error {
	sub, iss, err := s.restSubjectIssue(r)
	if err != nil {
		return err
	}
	q := r.URL.Query()
	list, names, err := s.jiraIssues(r.Context(), s.restOut(r), sub, []*issueRow{iss},
		parseFieldSelection(q["fields"], "*all"), expandSet(q["expand"]...), false)
	if err != nil {
		return err
	}
	if names != nil {
		list[0]["names"] = names
	}
	return writeJSON(w, http.StatusOK, list[0])
}

func (s *Server) restCreateIssue(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	var in jiraEditInput
	if err := readRESTJSON(r, &in); err != nil {
		return err
	}
	pr, ok := parseRef(in.Fields["project"])
	if !ok || pr.null {
		return fieldError("project", failf(http.StatusBadRequest, i18n.FieldRequired, "project"))
	}
	var projectID int64
	if pr.key == "" && pr.name == "" {
		projectID, _ = intID(pr.id)
	} else if projectID, err = s.projectByKey(ctx, pr.text()); err != nil {
		projectID = 0
	}
	if projectID == 0 || !sub.CanBrowseProject(projectID) {
		return unknownValue("project", pr.text())
	}
	p, err := s.editProjectOf(ctx, projectID)
	if err != nil {
		return err
	}
	cat, err := s.catalogue(ctx)
	if err != nil {
		return err
	}
	edit, err := s.translateEdit(ctx, sub, cat, p, nil, in)
	if err != nil {
		return err
	}
	for jira, ui := range map[string]string{"issuetype": "type", "summary": "summary"} {
		if _, ok := edit.ui[ui]; !ok {
			return fieldError(jira, failf(http.StatusBadRequest, i18n.FieldRequired, jira))
		}
	}
	// Check what can still fail before the issue exists. Custom fields and
	// the sprint take the permission to edit the issue, as its reporter.
	if len(edit.custom) > 0 || (edit.sprint != nil && *edit.sprint != 0) {
		if !sub.Can(perm.Edit, projectID, &perm.Issue{ProjectID: projectID, ReporterID: sub.UserID}) {
			return errForbidden
		}
	}
	if edit.sprint != nil && *edit.sprint != 0 {
		if _, err := s.sprintBoard(ctx, sub, *edit.sprint); err != nil {
			return fieldError("sprint", err)
		}
	}
	var typeID string
	json.Unmarshal(edit.ui["type"], &typeID)
	if err := s.checkCustomContext(ctx, projectID, typeID, edit.custom); err != nil {
		return err
	}
	var created struct{ Key string }
	if err := s.uiCall(r, s.createIssue, http.MethodPost, "/api/issues", nil, edit.ui, &created); err != nil {
		return err
	}
	iss, err := s.findIssue(ctx, created.Key)
	if err != nil {
		return err
	}
	o := s.restOut(r)
	if err := s.applyCustom(ctx, sub, o, iss, edit.custom); err != nil {
		return err
	}
	if edit.sprint != nil && *edit.sprint != 0 {
		if err := s.applySprint(ctx, sub, iss, *edit.sprint); err != nil {
			return err
		}
	}
	for _, c := range edit.comments {
		if err := s.uiCall(r, s.addComment, http.MethodPost, "/api/issues/"+iss.Key()+"/comments",
			map[string]string{"key": iss.Key()}, c, nil); err != nil {
			return err
		}
	}
	return writeJSON(w, http.StatusCreated, map[string]string{"id": o.id(iss.ID), "key": iss.Key(),
		"self": o.api + "/issue/" + o.id(iss.ID)})
}

// restEdit applies an edit (fields, custom fields, sprint, comments) to an
// issue. Permissions that would only fail midway are checked first.
func (s *Server) restEdit(r *http.Request, sub *perm.Subject, iss *issueRow, edit *jiraEdit) error {
	ctx := r.Context()
	pi := iss.perm()
	if (len(edit.custom) > 0 || edit.sprint != nil) && !sub.Can(perm.Edit, iss.ProjectID, &pi) {
		return errForbidden
	}
	if len(edit.comments) > 0 && !sub.Can(perm.Comment, iss.ProjectID, &pi) {
		return errForbidden
	}
	typeID := iss.TypeID
	if raw, ok := edit.ui["type"]; ok {
		json.Unmarshal(raw, &typeID)
	}
	if err := s.checkCustomContext(ctx, iss.ProjectID, typeID, edit.custom); err != nil {
		return err
	}
	if edit.sprint != nil && *edit.sprint != 0 {
		if _, err := s.sprintBoard(ctx, sub, *edit.sprint); err != nil {
			return fieldError("sprint", err)
		}
	}
	if len(edit.ui) > 0 {
		if err := s.applyUpdate(ctx, sub, iss, edit.ui); err != nil {
			return err
		}
	}
	if err := s.applyCustom(ctx, sub, s.restOut(r), iss, edit.custom); err != nil {
		return err
	}
	if edit.sprint != nil {
		if err := s.applySprint(ctx, sub, iss, *edit.sprint); err != nil {
			return err
		}
	}
	for _, c := range edit.comments {
		if err := s.uiCall(r, s.addComment, http.MethodPost, "/api/issues/"+iss.Key()+"/comments",
			map[string]string{"key": iss.Key()}, c, nil); err != nil {
			return err
		}
	}
	return nil
}

// translateForIssue translates an edit of an existing issue.
func (s *Server) translateForIssue(ctx context.Context, sub *perm.Subject, iss *issueRow, in jiraEditInput) (*jiraEdit, error) {
	cat, err := s.catalogue(ctx)
	if err != nil {
		return nil, err
	}
	p, err := s.editProjectOf(ctx, iss.ProjectID)
	if err != nil {
		return nil, err
	}
	p.issueType = iss.TypeID
	set, err := s.loadIssues(ctx, sub, []*issueRow{iss}, func(k string) bool {
		return k == loadLabels || k == loadComponents || k == loadVersions
	})
	if err != nil {
		return nil, err
	}
	return s.translateEdit(ctx, sub, cat, p, set.list[0], in)
}

func (s *Server) restUpdateIssue(w http.ResponseWriter, r *http.Request) error {
	sub, iss, err := s.restSubjectIssue(r)
	if err != nil {
		return err
	}
	var in jiraEditInput
	if err := readRESTJSON(r, &in); err != nil {
		return err
	}
	edit, err := s.translateForIssue(r.Context(), sub, iss, in)
	if err != nil {
		return err
	}
	if err := s.restEdit(r, sub, iss, edit); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) restDeleteIssue(w http.ResponseWriter, r *http.Request) error {
	_, iss, err := s.restSubjectIssue(r)
	if err != nil {
		return err
	}
	if r.URL.Query().Get("deleteSubtasks") != "true" {
		var n int
		if err := s.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM issues WHERE parent_id = ?`, iss.ID).
			Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return fail(http.StatusBadRequest, i18n.IssueDeleteSubtasks)
		}
	}
	if err := s.uiCall(r, s.deleteIssue, http.MethodDelete, "/api/issues/"+iss.Key(),
		map[string]string{"key": iss.Key()}, nil, nil); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// restAssign assigns an issue: {"name": "jdupont"}, {"name": null} for
// nobody, {"name": "-1"} for the project's default assignee.
func (s *Server) restAssign(w http.ResponseWriter, r *http.Request) error {
	sub, iss, err := s.restSubjectIssue(r)
	if err != nil {
		return err
	}
	var in struct {
		Name *string `json:"name"`
		Key  *string `json:"key"`
	}
	if err := readRESTJSON(r, &in); err != nil {
		return err
	}
	who := ref{null: true}
	for _, v := range []*string{in.Name, in.Key} {
		if v != nil && *v != "" {
			who = ref{name: *v}
		}
	}
	p, err := s.editProjectOf(r.Context(), iss.ProjectID)
	if err != nil {
		return err
	}
	id, err := s.assigneeRef(r.Context(), p, who)
	if err != nil {
		return err
	}
	raw, _ := json.Marshal(id)
	if err := s.applyUpdate(r.Context(), sub, iss, fields{"assignee": raw}); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) restTransitions(w http.ResponseWriter, r *http.Request) error {
	sub, iss, err := s.restSubjectIssue(r)
	if err != nil {
		return err
	}
	cat, err := s.catalogue(r.Context())
	if err != nil {
		return err
	}
	q := r.URL.Query()
	list, err := s.jiraTransitions(r.Context(), s.restOut(r), sub, cat, iss, expandSet(q["expand"]...)["transitions.fields"])
	if err != nil {
		return err
	}
	if id := q.Get("transitionId"); id != "" {
		kept := []map[string]any{}
		for _, t := range list {
			if t["id"] == id {
				kept = append(kept, t)
			}
		}
		list = kept
	}
	return writeJSON(w, http.StatusOK, map[string]any{"expand": "transitions", "transitions": list})
}

// restTransition moves an issue along its workflow, with the fields and
// comment given for the transition.
func (s *Server) restTransition(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, iss, err := s.restSubjectIssue(r)
	if err != nil {
		return err
	}
	var in struct {
		Transition struct {
			ID   json.RawMessage `json:"id"`
			Name string          `json:"name"`
		} `json:"transition"`
		Fields map[string]json.RawMessage              `json:"fields"`
		Update map[string][]map[string]json.RawMessage `json:"update"`
	}
	if err := readRESTJSON(r, &in); err != nil {
		return err
	}
	id, _ := parseRef(in.Transition.ID)
	cat, err := s.catalogue(ctx)
	if err != nil {
		return err
	}
	list, _, err := s.issueTransitions(ctx, cat, iss)
	if err != nil {
		return err
	}
	target := ""
	for _, t := range list {
		if (id.text() != "" && transitionID(t) == id.text()) || (id.text() == "" && strings.EqualFold(t.Name, in.Transition.Name)) {
			target = t.To.ID
			break
		}
	}
	if target == "" {
		return fieldError("transition", fail(http.StatusBadRequest, i18n.IssueTransitionUnavailable))
	}
	edit, err := s.translateForIssue(ctx, sub, iss, jiraEditInput{Fields: in.Fields, Update: in.Update})
	if err != nil {
		return err
	}
	edit.ui["status"], _ = json.Marshal(target)
	if err := s.restEdit(r, sub, iss, edit); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// Comments ---------------------------------------------------------------------

func (s *Server) restComments(w http.ResponseWriter, r *http.Request) error {
	sub, iss, err := s.restSubjectIssue(r)
	if err != nil {
		return err
	}
	rows, err := s.visibleComments(r.Context(), sub, iss)
	if err != nil {
		return err
	}
	sortComments(rows, r.URL.Query().Get("orderBy"))
	startAt, maxResults := paging(r, 5000, 5000)
	page := rows[min(startAt, len(rows)):min(startAt+maxResults, len(rows))]
	o := s.restOut(r)
	var ids []int64
	for _, c := range page {
		ids = append(ids, c.author.Int64, c.updatedBy.Int64)
	}
	users, err := s.jiraUsers(r.Context(), o, ids...)
	if err != nil {
		return err
	}
	list := []any{}
	for _, c := range page {
		list = append(list, o.comment(iss, c, users))
	}
	return writeJSON(w, http.StatusOK, map[string]any{"startAt": startAt, "maxResults": maxResults,
		"total": len(rows), "comments": list})
}

// pathComment finds the comment of the path among those of the path's issue
// that the user may read.
func (s *Server) pathComment(r *http.Request, sub *perm.Subject, iss *issueRow) (commentRow, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return commentRow{}, errNotFound
	}
	rows, err := s.visibleComments(r.Context(), sub, iss)
	if err != nil {
		return commentRow{}, err
	}
	for _, c := range rows {
		if c.id == id {
			return c, nil
		}
	}
	return commentRow{}, errNotFound
}

func (s *Server) writeJiraComment(w http.ResponseWriter, r *http.Request, sub *perm.Subject, iss *issueRow, id int64,
	status int) error {
	r.SetPathValue("id", strconv.FormatInt(id, 10))
	c, err := s.pathComment(r, sub, iss)
	if err != nil {
		return err
	}
	o := s.restOut(r)
	users, err := s.jiraUsers(r.Context(), o, c.author.Int64, c.updatedBy.Int64)
	if err != nil {
		return err
	}
	return writeJSON(w, status, o.comment(iss, c, users))
}

func (s *Server) restComment(w http.ResponseWriter, r *http.Request) error {
	sub, iss, err := s.restSubjectIssue(r)
	if err != nil {
		return err
	}
	c, err := s.pathComment(r, sub, iss)
	if err != nil {
		return err
	}
	return s.writeJiraComment(w, r, sub, iss, c.id, http.StatusOK)
}

type jiraCommentInput struct {
	Body       string          `json:"body"`
	Visibility json.RawMessage `json:"visibility"`
}

func (s *Server) restAddComment(w http.ResponseWriter, r *http.Request) error {
	sub, iss, err := s.restSubjectIssue(r)
	if err != nil {
		return err
	}
	var in jiraCommentInput
	if err := readRESTJSON(r, &in); err != nil {
		return err
	}
	c, err := s.commentVisibility(r.Context(), iss.ProjectID, in.Body, in.Visibility)
	if err != nil {
		return err
	}
	var out commentOut
	if err := s.uiCall(r, s.addComment, http.MethodPost, "/api/issues/"+iss.Key()+"/comments",
		map[string]string{"key": iss.Key()}, c, &out); err != nil {
		return err
	}
	return s.writeJiraComment(w, r, sub, iss, out.ID, http.StatusCreated)
}

func (s *Server) restEditComment(w http.ResponseWriter, r *http.Request) error {
	sub, iss, err := s.restSubjectIssue(r)
	if err != nil {
		return err
	}
	c, err := s.pathComment(r, sub, iss)
	if err != nil {
		return err
	}
	var in jiraCommentInput
	if err := readRESTJSON(r, &in); err != nil {
		return err
	}
	if len(in.Visibility) > 0 {
		// Restricting or opening an existing comment is not offered.
		v, err := s.commentVisibility(r.Context(), iss.ProjectID, in.Body, in.Visibility)
		if err != nil {
			return err
		}
		var group string
		var role int64
		if v.Visibility != nil {
			group, role = v.Visibility.Group, v.Visibility.Role
		}
		if group != c.group.String || role != c.roleID.Int64 {
			return fieldError("visibility", fail(http.StatusBadRequest, i18n.CommentVisibilityFixed))
		}
	}
	id := strconv.FormatInt(c.id, 10)
	if err := s.uiCall(r, s.editComment, http.MethodPatch, "/api/comments/"+id, map[string]string{"id": id},
		map[string]string{"body": in.Body}, nil); err != nil {
		return err
	}
	return s.writeJiraComment(w, r, sub, iss, c.id, http.StatusOK)
}

func (s *Server) restDeleteComment(w http.ResponseWriter, r *http.Request) error {
	sub, iss, err := s.restSubjectIssue(r)
	if err != nil {
		return err
	}
	c, err := s.pathComment(r, sub, iss)
	if err != nil {
		return err
	}
	id := strconv.FormatInt(c.id, 10)
	if err := s.uiCall(r, s.deleteComment, http.MethodDelete, "/api/comments/"+id, map[string]string{"id": id},
		nil, nil); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// Work logs --------------------------------------------------------------------

// issueWorklogs loads the work logs of an issue the user may read, with
// their authors.
func (s *Server) issueWorklogs(ctx context.Context, o *restOut, sub *perm.Subject, iss *issueRow) ([]worklogData,
	map[int64]*jiraUser, error) {
	set, err := s.loadIssues(ctx, sub, []*issueRow{iss}, func(k string) bool { return k == loadWorklogs })
	if err != nil {
		return nil, nil, err
	}
	users, err := s.jiraUsers(ctx, o, set.userIDs()...)
	return set.list[0].worklogs, users, err
}

func (s *Server) restWorklogs(w http.ResponseWriter, r *http.Request) error {
	sub, iss, err := s.restSubjectIssue(r)
	if err != nil {
		return err
	}
	o := s.restOut(r)
	logs, users, err := s.issueWorklogs(r.Context(), o, sub, iss)
	if err != nil {
		return err
	}
	startAt, maxResults := paging(r, 5000, 5000)
	list := []any{}
	for _, wl := range logs[min(startAt, len(logs)):min(startAt+maxResults, len(logs))] {
		list = append(list, o.worklog(iss, wl, users))
	}
	return writeJSON(w, http.StatusOK, map[string]any{"startAt": startAt, "maxResults": maxResults,
		"total": len(logs), "worklogs": list})
}

// restWorklogInput is Jira's body for adding or changing a work log.
type restWorklogInput struct {
	Comment          *string         `json:"comment"`
	Started          string          `json:"started"`
	TimeSpent        string          `json:"timeSpent"`
	TimeSpentSeconds *int64          `json:"timeSpentSeconds"`
	Visibility       json.RawMessage `json:"visibility"`
}

// uiBody translates it into the interface API's body, with only what the
// request gave. Jira's adjustEstimate options are not supported: the
// remaining estimate always adjusts automatically.
func (in restWorklogInput) uiBody() (map[string]any, error) {
	if len(in.Visibility) > 0 && string(in.Visibility) != "null" {
		return nil, fieldError("visibility", fail(http.StatusBadRequest, i18n.WorkLogRestricted))
	}
	body := map[string]any{}
	switch {
	case in.TimeSpentSeconds != nil:
		body["seconds"] = *in.TimeSpentSeconds
	case strings.TrimSpace(in.TimeSpent) != "":
		v, ok := jql.Seconds(in.TimeSpent)
		if !ok {
			return nil, fieldError("timeLogged", fail(http.StatusBadRequest, i18n.WorkLogDurationInvalid))
		}
		body["seconds"] = int64(math.Round(v))
	}
	if in.Comment != nil {
		body["body"] = *in.Comment
	}
	if in.Started != "" {
		t, ok := parseJiraTime(in.Started)
		if !ok {
			return nil, fieldError("started", failf(http.StatusBadRequest, i18n.FieldValueInvalid, "started"))
		}
		body["started"] = t
	}
	return body, nil
}

func (s *Server) restAddWorklog(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, iss, err := s.restSubjectIssue(r)
	if err != nil {
		return err
	}
	var in restWorklogInput
	if err := readRESTJSON(r, &in); err != nil {
		return err
	}
	body, err := in.uiBody()
	if err != nil {
		return err
	}
	if err := s.uiCall(r, s.addWorklog, http.MethodPost, "/api/issues/"+iss.Key()+"/worklogs",
		map[string]string{"key": iss.Key()}, body, nil); err != nil {
		return err
	}
	var id int64
	if err := s.db.QueryRowContext(ctx, `SELECT MAX(id) FROM worklogs WHERE issue_id = ? AND author_id = ?`,
		iss.ID, sub.UserID).Scan(&id); err != nil {
		return err
	}
	return s.restRespondWorklog(w, r, sub, iss, id, http.StatusCreated)
}

// restRespondWorklog answers with a work log of an issue, as Jira renders it.
func (s *Server) restRespondWorklog(w http.ResponseWriter, r *http.Request, sub *perm.Subject, iss *issueRow, id int64,
	status int) error {
	o := s.restOut(r)
	logs, users, err := s.issueWorklogs(r.Context(), o, sub, iss)
	if err != nil {
		return err
	}
	for _, wl := range logs {
		if wl.id == id {
			return writeJSON(w, status, o.worklog(iss, wl, users))
		}
	}
	return errNotFound
}

// restWorklogOfIssue checks that the work log of the path belongs to the
// issue of the path, and returns its id.
func (s *Server) restWorklogOfIssue(r *http.Request, iss *issueRow) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return 0, errNotFound
	}
	var n int
	if err := s.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM worklogs WHERE id = ? AND issue_id = ?`,
		id, iss.ID).Scan(&n); err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, errNotFound
	}
	return id, nil
}

func (s *Server) restWorklog(w http.ResponseWriter, r *http.Request) error {
	sub, iss, err := s.restSubjectIssue(r)
	if err != nil {
		return err
	}
	id, err := s.restWorklogOfIssue(r, iss)
	if err != nil {
		return err
	}
	return s.restRespondWorklog(w, r, sub, iss, id, http.StatusOK)
}

func (s *Server) restEditWorklog(w http.ResponseWriter, r *http.Request) error {
	sub, iss, err := s.restSubjectIssue(r)
	if err != nil {
		return err
	}
	id, err := s.restWorklogOfIssue(r, iss)
	if err != nil {
		return err
	}
	var in restWorklogInput
	if err := readRESTJSON(r, &in); err != nil {
		return err
	}
	body, err := in.uiBody()
	if err != nil {
		return err
	}
	if err := s.uiCall(r, s.editWorklog, http.MethodPut, "/api/worklogs/"+strconv.FormatInt(id, 10),
		map[string]string{"id": strconv.FormatInt(id, 10)}, body, nil); err != nil {
		return err
	}
	return s.restRespondWorklog(w, r, sub, iss, id, http.StatusOK)
}

func (s *Server) restDeleteWorklog(w http.ResponseWriter, r *http.Request) error {
	_, iss, err := s.restSubjectIssue(r)
	if err != nil {
		return err
	}
	id, err := s.restWorklogOfIssue(r, iss)
	if err != nil {
		return err
	}
	if err := s.uiCall(r, s.deleteWorklog, http.MethodDelete, "/api/worklogs/"+strconv.FormatInt(id, 10),
		map[string]string{"id": strconv.FormatInt(id, 10)}, nil, nil); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// Watchers and votes -----------------------------------------------------------

// restWatchers lists who watches an issue. Like the interface, it shows
// everyone the count; the names only to those who manage watchers.
func (s *Server) restWatchers(w http.ResponseWriter, r *http.Request) error {
	sub, iss, err := s.restSubjectIssue(r)
	if err != nil {
		return err
	}
	var ids []int64
	if err := sqlq.Each(r.Context(), s.db, `SELECT user_id FROM watchers WHERE issue_id = ? ORDER BY created_at, user_id`,
		func(rows *sql.Rows) error {
			var id int64
			if err := rows.Scan(&id); err != nil {
				return err
			}
			ids = append(ids, id)
			return nil
		}, iss.ID); err != nil {
		return err
	}
	pi := iss.perm()
	manage := sub.Can(perm.Watchers, iss.ProjectID, &pi)
	watching := false
	var shown []int64
	for _, id := range ids {
		if id == sub.UserID {
			watching = true
		}
		if manage || id == sub.UserID {
			shown = append(shown, id)
		}
	}
	o := s.restOut(r)
	users, err := s.jiraUsers(r.Context(), o, shown...)
	if err != nil {
		return err
	}
	list := []any{}
	for _, id := range shown {
		if u, ok := users[id]; ok {
			list = append(list, u)
		}
	}
	return writeJSON(w, http.StatusOK, map[string]any{"self": o.api + "/issue/" + iss.Key() + "/watchers",
		"isWatching": watching, "watchCount": len(ids), "watchers": list})
}

// restWatch adds or removes a watcher: oneself (no user given), or someone
// else, which takes the permission to manage watchers.
func (s *Server) restWatch(on bool) handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		ctx := r.Context()
		sub, iss, err := s.restSubjectIssue(r)
		if err != nil {
			return err
		}
		name := r.URL.Query().Get("username")
		if on {
			raw, err := readBody(r)
			if err != nil {
				return err
			}
			if len(raw) > 0 && json.Unmarshal(raw, &name) != nil {
				return invalidValue("username")
			}
		}
		userID := sub.UserID
		if strings.TrimSpace(name) != "" {
			id, ok := s.userByName(ctx, strings.TrimSpace(name))
			if !ok {
				return unknownValue("username", name)
			}
			userID = id
		}
		method := http.MethodPut
		if !on {
			method = http.MethodDelete
		}
		if userID == sub.UserID {
			if err := s.uiCall(r, s.setWatch(on), method, "/api/issues/"+iss.Key()+"/watch",
				map[string]string{"key": iss.Key()}, nil, nil); err != nil {
				return err
			}
			w.WriteHeader(http.StatusNoContent)
			return nil
		}
		pi := iss.perm()
		if !sub.Can(perm.Watchers, iss.ProjectID, &pi) {
			return errForbidden
		}
		if on {
			var admin, active bool
			if err := s.db.QueryRowContext(ctx, `SELECT is_admin, active FROM users WHERE id = ?`, userID).
				Scan(&admin, &active); err != nil {
				return err
			}
			watcher, err := s.perms.Subject(ctx, userID, admin)
			if err != nil {
				return err
			}
			if !active || !watcher.CanSee(pi) {
				return fieldError("username", fail(http.StatusBadRequest, i18n.IssueNotVisibleTo))
			}
			_, err = s.db.ExecContext(ctx, `INSERT IGNORE INTO watchers (issue_id, user_id, created_at) VALUES (?, ?, UTC_TIMESTAMP(3))`,
				iss.ID, userID)
			if err != nil {
				return err
			}
		} else if _, err := s.db.ExecContext(ctx, `DELETE FROM watchers WHERE issue_id = ? AND user_id = ?`,
			iss.ID, userID); err != nil {
			return err
		}
		w.WriteHeader(http.StatusNoContent)
		return nil
	}
}

// readBody reads a small request body whole.
func readBody(r *http.Request) ([]byte, error) {
	var raw json.RawMessage
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<16))
	if err := dec.Decode(&raw); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		return nil, failf(http.StatusBadRequest, i18n.RequestInvalid, err.Error())
	}
	return raw, nil
}

func (s *Server) restVote(on bool) handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		_, iss, err := s.restSubjectIssue(r)
		if err != nil {
			return err
		}
		method := http.MethodPut
		if !on {
			method = http.MethodDelete
		}
		if err := s.uiCall(r, s.setVote(on), method, "/api/issues/"+iss.Key()+"/vote",
			map[string]string{"key": iss.Key()}, nil, nil); err != nil {
			return err
		}
		w.WriteHeader(http.StatusNoContent)
		return nil
	}
}

// Attachments ------------------------------------------------------------------

// restAttach adds the files of a multipart request (the field name does not
// matter) and lists them.
func (s *Server) restAttach(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, iss, err := s.restSubjectIssue(r)
	if err != nil {
		return err
	}
	var before int64
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(id), 0) FROM attachments WHERE issue_id = ?`, iss.ID).
		Scan(&before); err != nil {
		return err
	}
	r.SetPathValue("key", iss.Key())
	if err := s.upload(&recorder{header: http.Header{}}, r); err != nil {
		return err
	}
	o := s.restOut(r)
	set, err := s.loadIssues(ctx, sub, []*issueRow{iss}, func(k string) bool { return k == loadAttachments })
	if err != nil {
		return err
	}
	users, err := s.jiraUsers(ctx, o, sub.UserID)
	if err != nil {
		return err
	}
	out := []any{}
	for _, a := range set.list[0].attachments {
		if a.id > before && a.author.Int64 == sub.UserID {
			out = append(out, o.attachment(a, users))
		}
	}
	return writeJSON(w, http.StatusOK, out)
}

func (s *Server) restAttachment(w http.ResponseWriter, r *http.Request) error {
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	a, err := s.visibleAttachment(r, sub)
	if err != nil {
		return err
	}
	var d attachmentData
	if err := s.db.QueryRowContext(r.Context(), `SELECT id, filename, mime_type, size, created_at, author_id
		FROM attachments WHERE id = ?`, a.id).Scan(&d.id, &d.name, &d.mime, &d.size, &d.created, &d.author); err != nil {
		return err
	}
	o := s.restOut(r)
	users, err := s.jiraUsers(r.Context(), o, d.author.Int64)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, o.attachment(d, users))
}

// Links ------------------------------------------------------------------------

// restLink links two issues. As in Jira, the inward issue is the link's
// source: {"type": {"name": "Blocks"}, "inwardIssue": A, "outwardIssue": B}
// means "A blocks B".
func (s *Server) restLink(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	var in struct {
		Type         json.RawMessage   `json:"type"`
		InwardIssue  json.RawMessage   `json:"inwardIssue"`
		OutwardIssue json.RawMessage   `json:"outwardIssue"`
		Comment      *jiraCommentInput `json:"comment"`
	}
	if err := readRESTJSON(r, &in); err != nil {
		return err
	}
	cat, err := s.catalogue(ctx)
	if err != nil {
		return err
	}
	tr, _ := parseRef(in.Type)
	var linkType *linkTypeRef
	for _, lt := range cat.LinkTypes {
		if tr.id == strconv.FormatInt(lt.ID, 10) || (tr.text() != "" && (strings.EqualFold(lt.Name, tr.text()) ||
			strings.EqualFold(lt.Outward, tr.text()) || strings.EqualFold(lt.Inward, tr.text()))) {
			linkType = &lt
			break
		}
	}
	if linkType == nil {
		return unknownValue("type", tr.text())
	}
	var issues [2]*issueRow
	for i, raw := range []json.RawMessage{in.InwardIssue, in.OutwardIssue} {
		ir, _ := parseRef(raw)
		if issues[i], err = s.refIssue(ctx, sub, ir); err != nil {
			return fieldError([]string{"inwardIssue", "outwardIssue"}[i], err)
		}
	}
	source, target := issues[0], issues[1]
	if err := s.uiCall(r, s.addLink, http.MethodPost, "/api/issues/"+source.Key()+"/links",
		map[string]string{"key": source.Key()},
		map[string]any{"type": linkType.ID, "direction": "outward", "target": target.Key()}, nil); err != nil {
		return err
	}
	if in.Comment != nil && strings.TrimSpace(in.Comment.Body) != "" {
		c, err := s.commentVisibility(ctx, source.ProjectID, in.Comment.Body, in.Comment.Visibility)
		if err != nil {
			return err
		}
		if err := s.uiCall(r, s.addComment, http.MethodPost, "/api/issues/"+source.Key()+"/comments",
			map[string]string{"key": source.Key()}, c, nil); err != nil {
			return err
		}
	}
	w.WriteHeader(http.StatusCreated)
	return nil
}

func (s *Server) restLinkTypes(w http.ResponseWriter, r *http.Request) error {
	cat, err := s.catalogue(r.Context())
	if err != nil {
		return err
	}
	o := s.restOut(r)
	list := []any{}
	for _, lt := range cat.LinkTypes {
		list = append(list, o.linkType(lt))
	}
	return writeJSON(w, http.StatusOK, map[string]any{"issueLinkTypes": list})
}

// Search -----------------------------------------------------------------------

// searchRows runs a search condition (aliases i, p and pr) and returns a page
// of issues with the total.
func (s *Server) searchRows(ctx context.Context, where string, args []any, order string, offset, limit int) (
	[]*issueRow, int, error) {
	from := ` FROM issues i JOIN projects p ON p.id = i.project_id LEFT JOIN priorities pr ON pr.id = i.priority_id
		WHERE ` + where
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*)`+from, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if limit <= 0 || offset >= total {
		return nil, total, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+issueColumns+from+` ORDER BY `+order+`, i.id DESC LIMIT ? OFFSET ?`,
		append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []*issueRow
	for rows.Next() {
		iss, err := scanIssue(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, iss)
	}
	return out, total, rows.Err()
}

// restSearch runs a JQL search: GET with parameters, or POST with a JSON
// body, as Jira.
func (s *Server) restSearch(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	var query string
	var fieldList, expandList []string
	startAt, maxResults := paging(r, 50, 1000)
	if r.Method == http.MethodPost {
		var in struct {
			JQL        string          `json:"jql"`
			StartAt    int             `json:"startAt"`
			MaxResults *int            `json:"maxResults"`
			Fields     []string        `json:"fields"`
			Expand     json.RawMessage `json:"expand"`
		}
		if err := readRESTJSON(r, &in); err != nil {
			return err
		}
		query, fieldList, startAt = in.JQL, in.Fields, min(max(0, in.StartAt), 1<<31)
		maxResults = 50
		if in.MaxResults != nil && *in.MaxResults >= 0 {
			maxResults = min(*in.MaxResults, 1000)
		}
		var one string
		if json.Unmarshal(in.Expand, &one) == nil {
			expandList = []string{one}
		} else {
			json.Unmarshal(in.Expand, &expandList)
		}
	} else {
		q := r.URL.Query()
		query, fieldList, expandList = q.Get("jql"), q["fields"], q["expand"]
	}
	where, args, order, _, err := s.searchCondition(ctx, sub, url.Values{"jql": {query}})
	if err != nil {
		return err
	}
	rows, total, err := s.searchRows(ctx, where, args, order, startAt, maxResults)
	if err != nil {
		return err
	}
	issues, names, err := s.jiraIssues(ctx, s.restOut(r), sub, rows, parseFieldSelection(fieldList, "*navigable"),
		expandSet(expandList...), false)
	if err != nil {
		return err
	}
	out := map[string]any{"expand": "schema,names", "startAt": startAt, "maxResults": maxResults, "total": total,
		"issues": issues}
	if names != nil {
		out["names"] = names
	}
	return writeJSON(w, http.StatusOK, out)
}
