package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"solstice/internal/events"
	"solstice/internal/i18n"
	"solstice/internal/issue"
	"solstice/internal/jql"
	"solstice/internal/perm"
	"solstice/internal/search"
	"solstice/internal/sqlq"
)

var keyPattern = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9_]*)-(\d+)$`)

// issueRow is an issue as stored.
type issueRow struct {
	ID, ProjectID, Num       int64
	ProjectKey, ProjectName  string
	Summary                  string
	Description, Environment sql.NullString
	TypeID, StatusID         string
	PriorityID, ResolutionID sql.NullString
	ReporterID, AssigneeID   sql.NullInt64
	CreatorID, LevelID       sql.NullInt64
	ParentID                 sql.NullInt64
	Created, Updated         time.Time
	Resolved, Due            sql.NullTime
	Original, Remaining      sql.NullInt64
	Spent                    sql.NullInt64
	EpicID                   sql.NullInt64
	Rank                     sql.NullString
}

const issueColumns = `i.id, i.project_id, i.num, p.pkey, p.name, i.summary, i.description, i.environment,
	i.issue_type_id, i.status_id, i.priority_id, i.resolution_id, i.reporter_id, i.assignee_id, i.creator_id,
	i.security_level_id, i.parent_id, i.created_at, i.updated_at, i.resolved_at, i.due_date,
	i.original_estimate, i.remaining_estimate, i.time_spent, i.epic_id, i.rank_key`

type scanner interface{ Scan(...any) error }

func scanIssue(sc scanner) (*issueRow, error) {
	var i issueRow
	err := sc.Scan(&i.ID, &i.ProjectID, &i.Num, &i.ProjectKey, &i.ProjectName, &i.Summary, &i.Description,
		&i.Environment, &i.TypeID, &i.StatusID, &i.PriorityID, &i.ResolutionID, &i.ReporterID, &i.AssigneeID,
		&i.CreatorID, &i.LevelID, &i.ParentID, &i.Created, &i.Updated, &i.Resolved, &i.Due,
		&i.Original, &i.Remaining, &i.Spent, &i.EpicID, &i.Rank)
	return &i, err
}

func (i *issueRow) Key() string { return i.ProjectKey + "-" + strconv.FormatInt(i.Num, 10) }

func (i *issueRow) perm() perm.Issue {
	return perm.Issue{ProjectID: i.ProjectID, ReporterID: i.ReporterID.Int64, AssigneeID: i.AssigneeID.Int64,
		SecurityLevel: i.LevelID.Int64}
}

// findIssue loads an issue by key. Keys from before a project was renamed
// and keys of issues moved to another project still resolve.
func (s *Server) findIssue(ctx context.Context, key string) (*issueRow, error) {
	m := keyPattern.FindStringSubmatch(strings.TrimSpace(key))
	if m == nil {
		return nil, errNotFound
	}
	projectKey, num := strings.ToUpper(m[1]), m[2]
	from := `SELECT ` + issueColumns + ` FROM issues i JOIN projects p ON p.id = i.project_id `
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{from + `WHERE p.pkey = ? AND i.num = ?`, []any{projectKey, num}},
		{from + `JOIN project_keys k ON k.project_id = p.id WHERE k.pkey = ? AND i.num = ?`, []any{projectKey, num}},
		{from + `JOIN moved_issue_keys k ON k.issue_id = i.id WHERE k.old_key = ?`, []any{projectKey + "-" + num}},
	} {
		iss, err := scanIssue(s.db.QueryRowContext(ctx, q.sql, q.args...))
		if err == nil {
			return iss, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
	}
	return nil, errNotFound
}

// visibleIssue loads the issue named in the path, if the user may see it.
func (s *Server) visibleIssue(r *http.Request, sub *perm.Subject) (*issueRow, error) {
	iss, err := s.findIssue(r.Context(), r.PathValue("key"))
	if err != nil {
		return nil, err
	}
	if !sub.CanSee(iss.perm()) {
		return nil, errNotFound
	}
	return iss, nil
}

// issueSummary is an issue in a list.
type issueSummary struct {
	Key          string       `json:"key"`
	Summary      string       `json:"summary"`
	Type         typeRef      `json:"type"`
	Status       statusRef    `json:"status"`
	Priority     *priorityRef `json:"priority"`
	Resolution   *namedRef    `json:"resolution"`
	Assignee     *userRef     `json:"assignee"`
	Reporter     *userRef     `json:"reporter"`
	Created      time.Time    `json:"created"`
	Updated      time.Time    `json:"updated"`
	Due          *string      `json:"due"`
	Confidential bool         `json:"confidential"`
}

func (c *catalog) statusOf(id string) statusRef {
	if v, ok := c.status[id]; ok {
		return v
	}
	return statusRef{ID: id, Name: id, Category: issue.Todo}
}

func (c *catalog) typeOf(id string) typeRef {
	if v, ok := c.issueType[id]; ok {
		return v
	}
	return typeRef{ID: id, Name: id}
}

func (c *catalog) priorityOf(id sql.NullString) *priorityRef {
	if !id.Valid {
		return nil
	}
	if v, ok := c.priority[id.String]; ok {
		return &v
	}
	return &priorityRef{ID: id.String, Name: id.String}
}

func (c *catalog) resolutionOf(id sql.NullString) *namedRef {
	if !id.Valid {
		return nil
	}
	if v, ok := c.resolution[id.String]; ok {
		return &v
	}
	return &namedRef{ID: id.String, Name: id.String}
}

func summarize(i *issueRow, c *catalog, users map[int64]userRef) issueSummary {
	return issueSummary{
		Key: i.Key(), Summary: i.Summary, Type: c.typeOf(i.TypeID), Status: c.statusOf(i.StatusID),
		Priority: c.priorityOf(i.PriorityID), Resolution: c.resolutionOf(i.ResolutionID),
		Assignee: refOf(users, i.AssigneeID), Reporter: refOf(users, i.ReporterID),
		Created: i.Created, Updated: i.Updated, Due: dateOnly(i.Due), Confidential: i.LevelID.Valid,
	}
}

func (s *Server) listIssues(w http.ResponseWriter, r *http.Request) error {
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	cat, err := s.catalogue(r.Context())
	if err != nil {
		return err
	}
	params := r.URL.Query()
	page := min(10000, max(1, atoi(params.Get("page"), 1)))
	size := min(200, max(1, atoi(params.Get("size"), 50)))
	where, args, sort, mode, err := s.searchCondition(r.Context(), sub, params)
	if err != nil {
		return err
	}
	from := ` FROM issues i JOIN projects p ON p.id = i.project_id LEFT JOIN priorities pr ON pr.id = i.priority_id
		WHERE ` + where

	var total int
	if err := s.db.QueryRowContext(r.Context(), `SELECT COUNT(*)`+from, args...).Scan(&total); err != nil {
		return err
	}
	rows, err := s.db.QueryContext(r.Context(), `SELECT `+issueColumns+from+` ORDER BY `+sort+`, i.id DESC
		LIMIT ? OFFSET ?`, append(args, size, (page-1)*size)...)
	if err != nil {
		return err
	}
	var list []*issueRow
	var ids []int64
	for rows.Next() {
		iss, err := scanIssue(rows)
		if err != nil {
			rows.Close()
			return err
		}
		list = append(list, iss)
		ids = append(ids, iss.AssigneeID.Int64, iss.ReporterID.Int64)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	users, err := s.users(r.Context(), ids...)
	if err != nil {
		return err
	}
	out := make([]issueSummary, 0, len(list))
	for _, iss := range list {
		out = append(out, summarize(iss, cat, users))
	}
	return writeJSON(w, http.StatusOK, map[string]any{"total": total, "page": page, "size": size, "mode": mode,
		"issues": out})
}

func atoi(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}

// Issue detail ---------------------------------------------------------------

type idName struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type issueBrief struct {
	Key      string    `json:"key"`
	Summary  string    `json:"summary"`
	Type     typeRef   `json:"type"`
	Status   statusRef `json:"status"`
	Assignee *userRef  `json:"assignee"`
}

type issueLink struct {
	ID        int64       `json:"id"`
	Label     string      `json:"label"` // e.g. "blocks" or "is blocked by", from this issue's side
	Direction string      `json:"direction"`
	Type      linkTypeRef `json:"type"`
	Issue     issueBrief  `json:"issue"`
}

type remoteLink struct {
	ID           int64  `json:"id"`
	URL          string `json:"url"`
	Title        string `json:"title"`
	Relationship string `json:"relationship,omitempty"`
}

type attachmentOut struct {
	ID        int64     `json:"id"`
	Filename  string    `json:"filename"`
	MimeType  string    `json:"mimeType"`
	Size      int64     `json:"size"`
	Created   time.Time `json:"created"`
	Author    *userRef  `json:"author"`
	URL       string    `json:"url"`
	Missing   bool      `json:"missing"`
	CanDelete bool      `json:"canDelete"`
}

type customValue struct {
	ID     int64     `json:"id"`
	Name   string    `json:"name"`
	Kind   string    `json:"kind"` // text, wiki, number, date, datetime, url, option, user, issue, label
	Values []string  `json:"values"`
	Users  []userRef `json:"users,omitempty"`
}

type counter struct {
	Count int  `json:"count"`
	Me    bool `json:"me"`
}

type issueDetail struct {
	issueSummary
	ID              int64           `json:"id"`
	Project         projectBrief    `json:"project"`
	Description     *string         `json:"description"`
	Environment     *string         `json:"environment"`
	Creator         *userRef        `json:"creator"`
	Resolved        *time.Time      `json:"resolved"`
	Time            timeTracking    `json:"time"`
	SecurityLevel   *idName         `json:"securityLevel"`
	Parent          *issueBrief     `json:"parent"`
	Subtasks        []issueBrief    `json:"subtasks"`
	Labels          []string        `json:"labels"`
	Components      []idName        `json:"components"`
	AffectsVersions []idName        `json:"affectsVersions"`
	FixVersions     []idName        `json:"fixVersions"`
	Links           []issueLink     `json:"links"`
	RemoteLinks     []remoteLink    `json:"remoteLinks"`
	Attachments     []attachmentOut `json:"attachments"`
	CustomFields    []customValue   `json:"customFields"`
	Watchers        counter         `json:"watchers"`
	Votes           counter         `json:"votes"`
	Can             map[string]bool `json:"can"`
	Statuses        []statusRef     `json:"statuses"`
	Transitions     []transitionOut `json:"transitions"`
	SecurityLevels  []idName        `json:"securityLevels"`
	Epic            *issueBrief     `json:"epic"`
	Sprints         []sprintRef     `json:"sprints"`
	CommentCount    int             `json:"commentCount"`
}

type projectBrief struct {
	ID   int64  `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

type timeTracking struct {
	Original  *int64 `json:"original"`
	Remaining *int64 `json:"remaining"`
	Spent     *int64 `json:"spent"`
}

func (s *Server) getIssue(w http.ResponseWriter, r *http.Request) error {
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	iss, err := s.visibleIssue(r, sub)
	if err != nil {
		return err
	}
	d, err := s.issueDetail(r.Context(), sub, iss)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, d)
}

func (s *Server) issueDetail(ctx context.Context, sub *perm.Subject, iss *issueRow) (*issueDetail, error) {
	cat, err := s.catalogue(ctx)
	if err != nil {
		return nil, err
	}
	pi := iss.perm()
	d := &issueDetail{
		ID:          iss.ID,
		Project:     projectBrief{ID: iss.ProjectID, Key: iss.ProjectKey, Name: iss.ProjectName},
		Description: sqlq.String(iss.Description),
		Environment: sqlq.String(iss.Environment),
		Time:        timeTracking{sqlq.Int(iss.Original), sqlq.Int(iss.Remaining), sqlq.Int(iss.Spent)},
		Labels:      []string{}, Components: []idName{}, AffectsVersions: []idName{}, FixVersions: []idName{},
		Subtasks: []issueBrief{}, Links: []issueLink{}, RemoteLinks: []remoteLink{},
		Attachments: []attachmentOut{}, CustomFields: []customValue{}, SecurityLevels: []idName{},
		Can: map[string]bool{},
	}
	if iss.Resolved.Valid {
		d.Resolved = &iss.Resolved.Time
	}
	for _, p := range perm.All {
		d.Can[p] = sub.Can(p, iss.ProjectID, &pi)
	}
	userIDs := []int64{iss.ReporterID.Int64, iss.AssigneeID.Int64, iss.CreatorID.Int64}

	// Parent, sub-tasks and linked issues, keeping only those the user sees.
	related, err := s.relatedIssues(ctx, sub, iss)
	if err != nil {
		return nil, err
	}
	for _, rel := range related {
		userIDs = append(userIDs, rel.row.AssigneeID.Int64)
	}

	if err := sqlq.Each(ctx, s.db, `SELECT label FROM labels WHERE issue_id = ? AND field_id IS NULL ORDER BY label`,
		func(rows *sql.Rows) error {
			var l string
			if err := rows.Scan(&l); err != nil {
				return err
			}
			d.Labels = append(d.Labels, l)
			return nil
		}, iss.ID); err != nil {
		return nil, err
	}
	if err := sqlq.Each(ctx, s.db, `SELECT c.id, c.name FROM issue_components ic JOIN components c ON c.id = ic.component_id
		WHERE ic.issue_id = ? ORDER BY c.name`, func(rows *sql.Rows) error {
		var v idName
		if err := rows.Scan(&v.ID, &v.Name); err != nil {
			return err
		}
		d.Components = append(d.Components, v)
		return nil
	}, iss.ID); err != nil {
		return nil, err
	}
	if err := sqlq.Each(ctx, s.db, `SELECT v.id, v.name, iv.kind FROM issue_versions iv JOIN versions v ON v.id = iv.version_id
		WHERE iv.issue_id = ? ORDER BY v.position, v.name`, func(rows *sql.Rows) error {
		var v idName
		var kind string
		if err := rows.Scan(&v.ID, &v.Name, &kind); err != nil {
			return err
		}
		if kind == "fix" {
			d.FixVersions = append(d.FixVersions, v)
		} else {
			d.AffectsVersions = append(d.AffectsVersions, v)
		}
		return nil
	}, iss.ID); err != nil {
		return nil, err
	}
	if err := sqlq.Each(ctx, s.db, `SELECT id, url, title, COALESCE(relationship, '') FROM remote_links
		WHERE issue_id = ? ORDER BY id`, func(rows *sql.Rows) error {
		var v remoteLink
		if err := rows.Scan(&v.ID, &v.URL, &v.Title, &v.Relationship); err != nil {
			return err
		}
		d.RemoteLinks = append(d.RemoteLinks, v)
		return nil
	}, iss.ID); err != nil {
		return nil, err
	}

	type att struct {
		out    attachmentOut
		author sql.NullInt64
	}
	var atts []att
	if err := sqlq.Each(ctx, s.db, `SELECT id, filename, mime_type, size, created_at, author_id, file_missing
		FROM attachments WHERE issue_id = ? ORDER BY created_at, id`, func(rows *sql.Rows) error {
		var a att
		if err := rows.Scan(&a.out.ID, &a.out.Filename, &a.out.MimeType, &a.out.Size, &a.out.Created, &a.author,
			&a.out.Missing); err != nil {
			return err
		}
		a.out.URL = "/api/attachments/" + strconv.FormatInt(a.out.ID, 10)
		a.out.CanDelete = d.Can[perm.Moderate] || (d.Can[perm.Attach] && a.author.Int64 == sub.UserID)
		atts = append(atts, a)
		userIDs = append(userIDs, a.author.Int64)
		return nil
	}, iss.ID); err != nil {
		return nil, err
	}

	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(user_id = ?), 0) FROM watchers WHERE issue_id = ?`,
		sub.UserID, iss.ID).Scan(&d.Watchers.Count, &d.Watchers.Me); err != nil {
		return nil, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(user_id = ?), 0) FROM votes WHERE issue_id = ?`,
		sub.UserID, iss.ID).Scan(&d.Votes.Count, &d.Votes.Me); err != nil {
		return nil, err
	}

	custom, customUsers, err := s.customValues(ctx, sub, iss.ID)
	if err != nil {
		return nil, err
	}
	userIDs = append(userIDs, customUsers...)

	users, err := s.users(ctx, userIDs...)
	if err != nil {
		return nil, err
	}
	d.issueSummary = summarize(iss, cat, users)
	d.Creator = refOf(users, iss.CreatorID)
	for _, a := range atts {
		a.out.Author = refOf(users, a.author)
		d.Attachments = append(d.Attachments, a.out)
	}
	for _, rel := range related {
		brief := issueBrief{Key: rel.row.Key(), Summary: rel.row.Summary, Type: cat.typeOf(rel.row.TypeID),
			Status: cat.statusOf(rel.row.StatusID), Assignee: refOf(users, rel.row.AssigneeID)}
		switch rel.kind {
		case "parent":
			d.Parent = &brief
		case "subtask":
			d.Subtasks = append(d.Subtasks, brief)
		default:
			lt := cat.linkType[rel.linkType]
			label := lt.Outward
			if rel.kind == "inward" {
				label = lt.Inward
			}
			d.Links = append(d.Links, issueLink{ID: rel.linkID, Label: label, Direction: rel.kind, Type: lt, Issue: brief})
		}
	}
	for i := range custom {
		for _, id := range custom[i].userIDs {
			if u, ok := users[id]; ok {
				custom[i].out.Users = append(custom[i].out.Users, u)
			}
		}
		d.CustomFields = append(d.CustomFields, custom[i].out)
	}

	if iss.LevelID.Valid {
		var name string
		if err := s.db.QueryRowContext(ctx, `SELECT name FROM security_levels WHERE id = ?`, iss.LevelID.Int64).
			Scan(&name); err == nil {
			d.SecurityLevel = &idName{ID: iss.LevelID.Int64, Name: name}
		}
	}
	if d.Can[perm.Security] {
		if err := sqlq.Each(ctx, s.db, `SELECT id, name FROM security_levels WHERE project_id = ? ORDER BY name`,
			func(rows *sql.Rows) error {
				var v idName
				if err := rows.Scan(&v.ID, &v.Name); err != nil {
					return err
				}
				d.SecurityLevels = append(d.SecurityLevels, v)
				return nil
			}, iss.ProjectID); err != nil {
			return nil, err
		}
	}
	d.Transitions, d.Statuses, err = s.issueTransitions(ctx, cat, iss)
	if err != nil {
		return nil, err
	}
	count, err := s.visibleComments(ctx, sub, iss)
	if err != nil {
		return nil, err
	}
	d.CommentCount = len(count)

	if iss.EpicID.Valid {
		if epic, err := s.findIssueByID(ctx, iss.EpicID.Int64); err == nil && sub.CanSee(epic.perm()) {
			eu, _ := s.users(ctx, epic.AssigneeID.Int64)
			d.Epic = &issueBrief{Key: epic.Key(), Summary: epic.Summary, Type: cat.typeOf(epic.TypeID),
				Status: cat.statusOf(epic.StatusID), Assignee: refOf(eu, epic.AssigneeID)}
		}
	}
	d.Sprints = []sprintRef{}
	if err := sqlq.Each(ctx, s.db, `SELECT s.id, s.name, s.state, s.board_id FROM sprint_issues si
		JOIN sprints s ON s.id = si.sprint_id WHERE si.issue_id = ? ORDER BY s.position, s.id`, func(rows *sql.Rows) error {
		var sp sprintRef
		var board sql.NullInt64
		if err := rows.Scan(&sp.ID, &sp.Name, &sp.State, &board); err != nil {
			return err
		}
		sp.Board = board.Int64
		d.Sprints = append(d.Sprints, sp)
		return nil
	}, iss.ID); err != nil {
		return nil, err
	}
	return d, nil
}

type relatedIssue struct {
	kind     string // parent, subtask, outward, inward
	linkID   int64
	linkType int64
	row      *issueRow
}

func (s *Server) relatedIssues(ctx context.Context, sub *perm.Subject, iss *issueRow) ([]relatedIssue, error) {
	from := `SELECT ` + issueColumns + `, ? AS kind, 0, 0 FROM issues i JOIN projects p ON p.id = i.project_id `
	q := from + `WHERE i.id = ?
		UNION ALL ` + from + `WHERE i.parent_id = ?
		UNION ALL SELECT ` + issueColumns + `, 'outward', l.id, l.type_id FROM issue_links l
			JOIN issues i ON i.id = l.target_id JOIN projects p ON p.id = i.project_id WHERE l.source_id = ?
		UNION ALL SELECT ` + issueColumns + `, 'inward', l.id, l.type_id FROM issue_links l
			JOIN issues i ON i.id = l.source_id JOIN projects p ON p.id = i.project_id WHERE l.target_id = ?`
	rows, err := s.db.QueryContext(ctx, q, "parent", iss.ParentID.Int64, "subtask", iss.ID, iss.ID, iss.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []relatedIssue
	for rows.Next() {
		var rel relatedIssue
		var row issueRow
		err := rows.Scan(&row.ID, &row.ProjectID, &row.Num, &row.ProjectKey, &row.ProjectName, &row.Summary,
			&row.Description, &row.Environment, &row.TypeID, &row.StatusID, &row.PriorityID, &row.ResolutionID,
			&row.ReporterID, &row.AssigneeID, &row.CreatorID, &row.LevelID, &row.ParentID, &row.Created,
			&row.Updated, &row.Resolved, &row.Due, &row.Original, &row.Remaining, &row.Spent, &row.EpicID, &row.Rank,
			&rel.kind, &rel.linkID, &rel.linkType)
		if err != nil {
			return nil, err
		}
		if !sub.CanSee(row.perm()) {
			continue
		}
		rel.row = &row
		out = append(out, rel)
	}
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].kind == "subtask" && out[b].kind == "subtask" {
			return out[a].row.Num < out[b].row.Num
		}
		return false
	})
	return out, rows.Err()
}

// projectStatuses lists the statuses an issue can be moved to: those already
// in use in its project. (Per-project workflows come with the workflow editor.)
func (s *Server) projectStatuses(ctx context.Context, cat *catalog, projectID int64, current string) ([]statusRef, error) {
	used := map[string]bool{current: true}
	if err := sqlq.Each(ctx, s.db, `SELECT DISTINCT status_id FROM issues WHERE project_id = ?`, func(rows *sql.Rows) error {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		used[id] = true
		return nil
	}, projectID); err != nil {
		return nil, err
	}
	var out []statusRef
	for _, st := range cat.Statuses {
		if used[st.ID] {
			out = append(out, st)
		}
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].Category.Rank() < out[b].Category.Rank() })
	return out, nil
}

// typeStatuses are the statuses an issue of a type can have in a project.
func (s *Server) typeStatuses(ctx context.Context, cat *catalog, projectID int64, typeID string) ([]statusRef, error) {
	wf, _, ok, err := s.workflowOf(ctx, projectID, typeID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.projectStatuses(ctx, cat, projectID, "")
	}
	var out []statusRef
	err = sqlq.Each(ctx, s.db, `SELECT status_id FROM workflow_statuses WHERE workflow_id = ? ORDER BY position`,
		func(rows *sql.Rows) error {
			var id string
			if err := rows.Scan(&id); err != nil {
				return err
			}
			out = append(out, cat.statusOf(id))
			return nil
		}, wf)
	return out, err
}

// statusResolution is the resolution that goes with a new status: reaching a
// "done" status resolves the issue, leaving it reopens it.
func statusResolution(cat *catalog, resolution sql.NullString, status string) sql.NullString {
	switch done := cat.statusOf(status).Category == issue.Done; {
	case done && !resolution.Valid:
		resolution = sql.NullString{String: cat.DefaultResolution, Valid: cat.DefaultResolution != ""}
	case !done:
		resolution = sql.NullString{}
	}
	return resolution
}

type customRow struct {
	out     customValue
	userIDs []int64
}

// customValues renders an issue's custom field values for display.
func (s *Server) customValues(ctx context.Context, sub *perm.Subject, issueID int64) ([]customRow, []int64, error) {
	type raw struct {
		field          int64
		name, typeKey  string
		parentKey, str sql.NullString
		num            sql.NullFloat64
		text           sql.NullString
		date           sql.NullTime
	}
	var vals []raw
	optionIDs := map[int64]bool{}
	if err := sqlq.Each(ctx, s.db, `SELECT v.field_id, f.name, f.type_key, v.parent_key, v.string_value,
		v.number_value, v.text_value, v.date_value FROM custom_field_values v JOIN custom_fields f ON f.id = v.field_id
		WHERE v.issue_id = ? ORDER BY f.name, v.parent_key IS NOT NULL, v.id`, func(rows *sql.Rows) error {
		var v raw
		if err := rows.Scan(&v.field, &v.name, &v.typeKey, &v.parentKey, &v.str, &v.num, &v.text, &v.date); err != nil {
			return err
		}
		if id, err := strconv.ParseInt(v.str.String, 10, 64); err == nil && optionKind(fieldKind(v.typeKey)) {
			optionIDs[id] = true
		}
		vals = append(vals, v)
		return nil
	}, issueID); err != nil {
		return nil, nil, err
	}
	options := map[int64]string{}
	if len(optionIDs) > 0 {
		var args []any
		for id := range optionIDs {
			args = append(args, id)
		}
		if err := sqlq.Each(ctx, s.db, `SELECT id, value FROM custom_field_options WHERE id IN (`+sqlq.Marks(len(args))+`)`,
			func(rows *sql.Rows) error {
				var id int64
				var v string
				if err := rows.Scan(&id, &v); err != nil {
					return err
				}
				options[id] = v
				return nil
			}, args...); err != nil {
			return nil, nil, err
		}
	}

	var out []customRow
	byField := map[int64]int{}
	var allUsers []int64
	get := func(id int64, name, kind string) *customRow {
		if i, ok := byField[id]; ok {
			return &out[i]
		}
		byField[id] = len(out)
		out = append(out, customRow{out: customValue{ID: id, Name: name, Kind: kind, Values: []string{}}})
		return &out[len(out)-1]
	}
	for _, v := range vals {
		kind := fieldKind(v.typeKey)
		if hiddenKinds[kind] {
			continue
		}
		switch {
		case optionKind(kind):
			id, _ := strconv.ParseInt(v.str.String, 10, 64)
			label, ok := options[id]
			if !ok {
				label = v.str.String
			}
			row := get(v.field, v.name, "option")
			if kind == "cascadingselect" && v.parentKey.Valid && len(row.out.Values) > 0 {
				row.out.Values[len(row.out.Values)-1] += " › " + label
			} else {
				row.out.Values = append(row.out.Values, label)
			}
		case kind == "userpicker" || kind == "multiuserpicker":
			if id, err := strconv.ParseInt(v.str.String, 10, 64); err == nil {
				row := get(v.field, v.name, "user")
				row.userIDs = append(row.userIDs, id)
				allUsers = append(allUsers, id)
			}
		case kind == "datepicker":
			if v.date.Valid {
				get(v.field, v.name, "date").out.Values = append(get(v.field, v.name, "date").out.Values,
					v.date.Time.Format(time.DateOnly))
			}
		case kind == "datetime":
			if v.date.Valid {
				row := get(v.field, v.name, "datetime")
				row.out.Values = append(row.out.Values, v.date.Time.UTC().Format(time.RFC3339))
			}
		case kind == "float" || v.num.Valid && !v.str.Valid && !v.text.Valid:
			if kind == "gh-epic-link" {
				continue
			}
			row := get(v.field, v.name, "number")
			row.out.Values = append(row.out.Values, strconv.FormatFloat(v.num.Float64, 'f', -1, 64))
		case kind == "textarea":
			get(v.field, v.name, "wiki").out.Values = append(get(v.field, v.name, "wiki").out.Values, v.text.String)
		case kind == "url":
			get(v.field, v.name, "url").out.Values = append(get(v.field, v.name, "url").out.Values, v.str.String)
		default:
			val := v.str.String
			if !v.str.Valid && v.text.Valid {
				val = v.text.String
			}
			if val != "" {
				row := get(v.field, v.name, "text")
				row.out.Values = append(row.out.Values, val)
			}
		}
	}

	// Epic links point at an issue: show its key if the user may see it.
	for _, v := range vals {
		if fieldKind(v.typeKey) != "gh-epic-link" || !v.num.Valid {
			continue
		}
		var iss *issueRow
		iss, err := scanIssue(s.db.QueryRowContext(ctx, `SELECT `+issueColumns+` FROM issues i
			JOIN projects p ON p.id = i.project_id WHERE i.id = ?`, int64(v.num.Float64)))
		if err != nil || !sub.CanSee(iss.perm()) {
			continue
		}
		row := get(v.field, v.name, "issue")
		row.out.Values = append(row.out.Values, iss.Key())
	}

	// Labels-type custom fields keep their values in the labels table.
	if err := sqlq.Each(ctx, s.db, `SELECT f.id, f.name, l.label FROM labels l JOIN custom_fields f ON f.id = l.field_id
		WHERE l.issue_id = ? ORDER BY f.name, l.label`, func(rows *sql.Rows) error {
		var id int64
		var name, label string
		if err := rows.Scan(&id, &name, &label); err != nil {
			return err
		}
		row := get(id, name, "label")
		row.out.Values = append(row.out.Values, label)
		return nil
	}, issueID); err != nil {
		return nil, nil, err
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].out.Name < out[b].out.Name })
	return out, allUsers, nil
}

func fieldKind(typeKey string) string {
	if i := strings.LastIndexByte(typeKey, ':'); i >= 0 {
		return typeKey[i+1:]
	}
	return typeKey
}

func optionKind(kind string) bool {
	switch kind {
	case "select", "multiselect", "radiobuttons", "multicheckboxes", "cascadingselect", "gh-epic-status":
		return true
	}
	return false
}

// hiddenKinds are Jira Software's internal fields: ranking, colours, version
// bookkeeping, development panel. Sprints wait for boards to be imported.
var hiddenKinds = map[string]bool{
	"gh-lexo-rank": true, "gh-global-rank": true, "gh-epic-color": true, "gh-sprint": true,
	"greenhopper-releasedmultiversionhistory": true, "devsummary": true, "importid": true,
	"jqlFunctionsCustomFieldType": true,
}

// Changes --------------------------------------------------------------------

// change is one line of an issue's history. Field names follow Jira's so
// imported and new history read the same.
type change struct {
	field               string
	oldValue, oldString any
	newValue, newString any
}

func recordChanges(ctx context.Context, tx *sql.Tx, issueID, author int64, at time.Time, changes []change) error {
	if len(changes) == 0 {
		return nil
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO change_groups (issue_id, author_id, created_at) VALUES (?, ?, ?)`,
		issueID, author, at)
	if err != nil {
		return err
	}
	group, err := res.LastInsertId()
	if err != nil {
		return err
	}
	for _, c := range changes {
		if _, err := tx.ExecContext(ctx, `INSERT INTO change_items (group_id, field_type, field, old_value, old_string,
			new_value, new_string) VALUES (?, 'jira', ?, ?, ?, ?, ?)`,
			group, c.field, c.oldValue, c.oldString, c.newValue, c.newString); err != nil {
			return err
		}
	}
	return nil
}

// publish refreshes the pages showing an issue, and nothing more (a card
// moved up a board). Changes people hear about go through s.emit.
func (s *Server) publish(iss *issueRow, what string, by int64) {
	s.emit(events.Event{ActorID: by, IssueID: iss.ID, Key: iss.Key(), Access: iss.perm(), Live: what})
}

// Create and update ----------------------------------------------------------

// fields is a request body where an absent key and a null value differ:
// absent leaves a field alone, null clears it.
type fields map[string]json.RawMessage

func (f fields) has(k string) bool { _, ok := f[k]; return ok }

func (f fields) isNull(k string) bool { return string(f[k]) == "null" }

func (f fields) get(k string, v any) error {
	if err := json.Unmarshal(f[k], v); err != nil {
		return failf(http.StatusBadRequest, i18n.FieldValueInvalid, k)
	}
	return nil
}

var issueFields = map[string]bool{
	"project": true, "type": true, "summary": true, "description": true, "environment": true, "priority": true,
	"assignee": true, "labels": true, "components": true, "affectsVersions": true, "fixVersions": true,
	"due": true, "parent": true, "status": true, "resolution": true, "securityLevel": true, "epic": true,
	"originalEstimate": true, "remainingEstimate": true, // Jira durations ("1w 2d 3h 30m"), or null
	"customFields": true, // {"<field id>": value as in Jira's REST API}
}

func readFields(r *http.Request) (fields, error) {
	var f fields
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 4<<20))
	if err := dec.Decode(&f); err != nil {
		return nil, failf(http.StatusBadRequest, i18n.RequestInvalid, err.Error())
	}
	for k := range f {
		if !issueFields[k] {
			return nil, failf(http.StatusBadRequest, i18n.FieldUnknown, k)
		}
	}
	return f, nil
}

func (s *Server) createIssue(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	f, err := readFields(r)
	if err != nil {
		return err
	}
	var projectKey, typeID, summary string
	for k, v := range map[string]*string{"project": &projectKey, "type": &typeID, "summary": &summary} {
		if !f.has(k) {
			return failf(http.StatusBadRequest, i18n.FieldRequired, k)
		}
		if err := f.get(k, v); err != nil {
			return err
		}
	}
	summary = strings.TrimSpace(summary)
	if summary == "" || utf8.RuneCountInString(summary) > 255 {
		return fail(http.StatusBadRequest, i18n.IssueSummaryRequired)
	}
	var projectID, leadID int64
	var defaultAssignee string
	var lead sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `SELECT id, lead_id, default_assignee FROM projects WHERE pkey = ?`,
		strings.ToUpper(projectKey)).Scan(&projectID, &lead, &defaultAssignee); errors.Is(err, sql.ErrNoRows) {
		return errNotFound
	} else if err != nil {
		return err
	}
	leadID = lead.Int64
	if !sub.Can(perm.Create, projectID, nil) || !sub.CanBrowseProject(projectID) {
		return errForbidden
	}
	cat, err := s.catalogue(ctx)
	if err != nil {
		return err
	}
	it, ok := cat.issueType[typeID]
	if !ok || !s.projectHasType(ctx, projectID, typeID) {
		return fail(http.StatusBadRequest, i18n.IssueTypeUnavailable)
	}

	var parentID any
	var parentLevel sql.NullInt64
	if f.has("parent") && !f.isNull("parent") {
		var parentKey string
		if err := f.get("parent", &parentKey); err != nil {
			return err
		}
		parent, err := s.findIssue(ctx, parentKey)
		if err != nil || !sub.CanSee(parent.perm()) {
			return fail(http.StatusBadRequest, i18n.IssueParentNotFound)
		}
		if parent.ProjectID != projectID || parent.ParentID.Valid {
			return fail(http.StatusBadRequest, i18n.IssueParentInvalid)
		}
		parentID = parent.ID
		parentLevel = parent.LevelID
	}
	if it.Subtask != (parentID != nil) {
		if it.Subtask {
			return fail(http.StatusBadRequest, i18n.IssueSubtaskParentRequired)
		}
		return fail(http.StatusBadRequest, i18n.IssueParentNotSubtask)
	}

	var assignee any
	if f.has("assignee") && !f.isNull("assignee") {
		var id int64
		if err := f.get("assignee", &id); err != nil {
			return err
		}
		if !sub.Can(perm.Assign, projectID, nil) && id != sub.UserID {
			return fail(http.StatusForbidden, i18n.IssueAssignOthersForbidden)
		}
		if ok, err := s.isAssignable(ctx, projectID, id); err != nil {
			return err
		} else if !ok {
			return fail(http.StatusBadRequest, i18n.IssueAssigneeNotAssignable)
		}
		assignee = id
	} else if who, ok, err := s.componentAssignee(ctx, f, projectID, leadID); err != nil {
		return err
	} else if ok {
		assignee = who
	} else if defaultAssignee == "lead" && leadID != 0 {
		assignee = leadID
	}
	custom, err := s.readCustomFields(ctx, f, projectID, typeID)
	if err != nil {
		return err
	}
	// Custom fields take the permission to edit the issue, as its reporter.
	if len(custom) > 0 && !sub.Can(perm.Edit, projectID, &perm.Issue{ProjectID: projectID, ReporterID: sub.UserID}) {
		return errForbidden
	}

	status, err := s.initialStatusFor(ctx, projectID, typeID)
	if err != nil {
		return err
	}
	var description, environment, priority, due any
	if f.has("description") {
		var v *string
		if err := f.get("description", &v); err != nil {
			return err
		}
		if v != nil && strings.TrimSpace(*v) != "" {
			description = *v
		}
	}
	if f.has("environment") {
		var v *string
		if err := f.get("environment", &v); err != nil {
			return err
		}
		if v != nil && strings.TrimSpace(*v) != "" {
			environment = *v
		}
	}
	if f.has("priority") && !f.isNull("priority") {
		var v string
		if err := f.get("priority", &v); err != nil {
			return err
		}
		if _, ok := cat.priority[v]; !ok {
			return fail(http.StatusBadRequest, i18n.IssuePriorityUnknown)
		}
		priority = v
	} else if len(cat.Priorities) > 0 {
		// The default priority, else Jira's default: the middle one.
		priority = cat.Priorities[len(cat.Priorities)/2].ID
		for _, p := range cat.Priorities {
			if p.Default {
				priority = p.ID
			}
		}
	}
	if f.has("due") && !f.isNull("due") {
		var v string
		if err := f.get("due", &v); err != nil {
			return err
		}
		if _, err := time.Parse(time.DateOnly, v); err != nil {
			return fail(http.StatusBadRequest, i18n.IssueDueDateInvalid)
		}
		due = v
	}
	estimates, err := readEstimateEdit(f)
	if err != nil {
		return err
	}
	times := timeEstimates{}.withEstimateEdit(estimates)
	var level any
	if f.has("securityLevel") && !f.isNull("securityLevel") {
		if !sub.Can(perm.Security, projectID, nil) {
			return errForbidden
		}
		var id int64
		if err := f.get("securityLevel", &id); err != nil {
			return err
		}
		if !s.projectHasLevel(ctx, projectID, id) {
			return fail(http.StatusBadRequest, i18n.IssueSecurityLevelUnknown)
		}
		level = id
	}
	// A sub-task is exactly as confidential as its parent, as in Jira.
	if parentID != nil {
		level = sqlq.Value(parentLevel)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var num int64
	if err := tx.QueryRowContext(ctx, `SELECT issue_counter + 1 FROM projects WHERE id = ? FOR UPDATE`, projectID).
		Scan(&num); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE projects SET issue_counter = ? WHERE id = ?`, num, projectID); err != nil {
		return err
	}
	now := time.Now().UTC()
	// A new issue goes to the bottom of the board order.
	var lastRank string
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(rank_key), '') FROM issues`).Scan(&lastRank); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO issues (project_id, num, issue_type_id, parent_id, summary,
		description, environment, status_id, priority_id, reporter_id, assignee_id, creator_id, security_level_id,
		created_at, updated_at, due_date, original_estimate, remaining_estimate, rank_key)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		projectID, num, typeID, parentID, summary, description, environment, status, priority, sub.UserID, assignee,
		sub.UserID, level, now, now, due, sqlq.Value(times.Original), sqlq.Value(times.Remaining),
		rankBetween(lastRank, ""))
	if err != nil {
		return err
	}
	issueID, err := res.LastInsertId()
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO watchers (issue_id, user_id, created_at) VALUES (?, ?, ?)`,
		issueID, sub.UserID, now); err != nil {
		return err
	}
	iss := &issueRow{ID: issueID, ProjectID: projectID}
	if _, err := s.applySets(ctx, tx, f, iss); err != nil {
		return err
	}
	if f.has("epic") {
		epicID, _, err := s.epicTarget(ctx, sub, cat, &issueRow{ID: issueID, ProjectID: projectID, ParentID: nullInt64(parentID)}, f)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE issues SET epic_id = ? WHERE id = ?`, epicID, issueID); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	created, err := s.findIssueByID(ctx, issueID)
	if err != nil {
		return err
	}
	s.emit(issueEvent(events.IssueCreated, created, sub.UserID, "issue"))
	if err := s.applyCustom(ctx, sub, s.restOut(r), created, custom); err != nil {
		return err
	}
	return writeJSON(w, http.StatusCreated, map[string]string{"key": created.Key()})
}

func (s *Server) findIssueByID(ctx context.Context, id int64) (*issueRow, error) {
	return scanIssue(s.db.QueryRowContext(ctx, `SELECT `+issueColumns+` FROM issues i
		JOIN projects p ON p.id = i.project_id WHERE i.id = ?`, id))
}

func (s *Server) projectHasType(ctx context.Context, projectID int64, typeID string) bool {
	var n int
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM project_issue_types WHERE project_id = ? AND issue_type_id = ?`,
		projectID, typeID).Scan(&n)
	return n > 0
}

func (s *Server) projectHasLevel(ctx context.Context, projectID, levelID int64) bool {
	var n int
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM security_levels WHERE project_id = ? AND id = ?`,
		projectID, levelID).Scan(&n)
	return n > 0
}

func (s *Server) isAssignable(ctx context.Context, projectID, userID int64) (bool, error) {
	holders, args, err := s.perms.HoldersSQL(ctx, projectID, perm.Assignable)
	if err != nil {
		return false, err
	}
	var n int
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE active AND id = ? AND id IN (`+holders+`)`,
		append([]any{userID}, args...)...).Scan(&n)
	return n > 0, err
}

// initialStatus picks the status of a new issue: the "to do" status most used
// in the project. (Per-project workflows will define it explicitly.)
func (s *Server) initialStatus(ctx context.Context, projectID int64) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT s.id FROM statuses s
		LEFT JOIN (SELECT status_id, COUNT(*) AS n FROM issues WHERE project_id = ? GROUP BY status_id) u
			ON u.status_id = s.id
		WHERE s.category = 'todo' ORDER BY u.n IS NULL, u.n DESC, s.position LIMIT 1`, projectID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fail(http.StatusConflict, i18n.IssueNoTodoStatus)
	}
	return id, err
}

// applySets writes the multi-valued fields (labels, components, versions)
// present in f and returns the history lines for them.
func (s *Server) applySets(ctx context.Context, tx *sql.Tx, f fields, iss *issueRow) ([]change, error) {
	var changes []change
	if f.has("labels") {
		var labels []string
		if !f.isNull("labels") {
			if err := f.get("labels", &labels); err != nil {
				return nil, err
			}
		}
		clean := map[string]bool{}
		for _, l := range labels {
			l = strings.Join(strings.Fields(l), "_")
			if l != "" && utf8.RuneCountInString(l) <= 255 {
				clean[l] = true
			}
		}
		var before []string
		if err := sqlq.Each(ctx, tx, `SELECT label FROM labels WHERE issue_id = ? AND field_id IS NULL ORDER BY label`,
			func(rows *sql.Rows) error {
				var l string
				err := rows.Scan(&l)
				before = append(before, l)
				return err
			}, iss.ID); err != nil {
			return nil, err
		}
		after := make([]string, 0, len(clean))
		for l := range clean {
			after = append(after, l)
		}
		sort.Strings(after)
		if !slices.Equal(before, after) {
			if _, err := tx.ExecContext(ctx, `DELETE FROM labels WHERE issue_id = ? AND field_id IS NULL`, iss.ID); err != nil {
				return nil, err
			}
			for _, l := range after {
				if _, err := tx.ExecContext(ctx, `INSERT INTO labels (issue_id, label) VALUES (?, ?)`, iss.ID, l); err != nil {
					return nil, err
				}
			}
			changes = append(changes, change{field: "labels", oldString: strings.Join(before, " "),
				newString: strings.Join(after, " ")})
		}
	}

	type set struct {
		key, field, table, column, kind string
	}
	for _, st := range []set{
		{"components", "Component", "components", "component_id", ""},
		{"affectsVersions", "Version", "versions", "version_id", "affects"},
		{"fixVersions", "Fix Version", "versions", "version_id", "fix"},
	} {
		if !f.has(st.key) {
			continue
		}
		var ids []int64
		if !f.isNull(st.key) {
			if err := f.get(st.key, &ids); err != nil {
				return nil, err
			}
		}
		names := map[int64]string{}
		if err := sqlq.Each(ctx, tx, `SELECT id, name FROM `+st.table+` WHERE project_id = ?`, func(rows *sql.Rows) error {
			var id int64
			var name string
			err := rows.Scan(&id, &name)
			names[id] = name
			return err
		}, iss.ProjectID); err != nil {
			return nil, err
		}
		want := map[int64]bool{}
		for _, id := range ids {
			if _, ok := names[id]; !ok {
				return nil, failf(http.StatusBadRequest, i18n.FieldValueNotInProject, st.key)
			}
			want[id] = true
		}
		var q string
		var args []any
		if st.kind == "" {
			q, args = `SELECT component_id FROM issue_components WHERE issue_id = ?`, []any{iss.ID}
		} else {
			q, args = `SELECT version_id FROM issue_versions WHERE issue_id = ? AND kind = ?`, []any{iss.ID, st.kind}
		}
		have := map[int64]bool{}
		if err := sqlq.Each(ctx, tx, q, func(rows *sql.Rows) error {
			var id int64
			err := rows.Scan(&id)
			have[id] = true
			return err
		}, args...); err != nil {
			return nil, err
		}
		var err error
		for id := range have {
			if want[id] {
				continue
			}
			if st.kind == "" {
				_, err = tx.ExecContext(ctx, `DELETE FROM issue_components WHERE issue_id = ? AND component_id = ?`, iss.ID, id)
			} else {
				_, err = tx.ExecContext(ctx, `DELETE FROM issue_versions WHERE issue_id = ? AND version_id = ? AND kind = ?`,
					iss.ID, id, st.kind)
			}
			if err != nil {
				return nil, err
			}
			changes = append(changes, change{field: st.field, oldValue: strconv.FormatInt(id, 10), oldString: names[id]})
		}
		for id := range want {
			if have[id] {
				continue
			}
			if st.kind == "" {
				_, err = tx.ExecContext(ctx, `INSERT INTO issue_components (issue_id, component_id) VALUES (?, ?)`, iss.ID, id)
			} else {
				_, err = tx.ExecContext(ctx, `INSERT INTO issue_versions (issue_id, version_id, kind) VALUES (?, ?, ?)`,
					iss.ID, id, st.kind)
			}
			if err != nil {
				return nil, err
			}
			changes = append(changes, change{field: st.field, newValue: strconv.FormatInt(id, 10), newString: names[id]})
		}
	}
	return changes, nil
}

func (s *Server) userName(ctx context.Context, id int64) (string, string) {
	var username, name string
	s.db.QueryRowContext(ctx, `SELECT COALESCE(user_key, username), display_name FROM users WHERE id = ?`, id).
		Scan(&username, &name)
	return username, name
}

func (s *Server) updateIssue(w http.ResponseWriter, r *http.Request) error {
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	iss, err := s.visibleIssue(r, sub)
	if err != nil {
		return err
	}
	f, err := readFields(r)
	if err != nil {
		return err
	}
	ctx := r.Context()
	typeID := iss.TypeID
	if f.has("type") {
		f.get("type", &typeID)
	}
	custom, err := s.readCustomFields(ctx, f, iss.ProjectID, typeID)
	if err != nil {
		return err
	}
	if pi := iss.perm(); len(custom) > 0 && !sub.Can(perm.Edit, iss.ProjectID, &pi) {
		return errForbidden
	}
	if err := s.applyUpdate(ctx, sub, iss, f); err != nil {
		return err
	}
	if err := s.applyCustom(ctx, sub, s.restOut(r), iss, custom); err != nil {
		return err
	}
	return s.respondIssue(w, r, sub, iss.ID)
}

// applyUpdate changes an issue's fields, checking each one's permission,
// recording history and announcing the change. The board reuses it to move
// cards between columns.
func (s *Server) applyUpdate(ctx context.Context, sub *perm.Subject, iss *issueRow, f fields) error {
	if f.has("project") {
		return fail(http.StatusBadRequest, i18n.IssueMoveUseAction)
	}
	if f.has("parent") {
		return fail(http.StatusBadRequest, i18n.IssueParentUnsupported)
	}
	cat, err := s.catalogue(ctx)
	if err != nil {
		return err
	}
	pi := iss.perm()
	can := func(p string) error {
		if !sub.Can(p, iss.ProjectID, &pi) {
			return errForbidden
		}
		return nil
	}

	sets := map[string]any{}
	var changes []change
	text := func(key, column string, required bool, maxLen int) error {
		if !f.has(key) {
			return nil
		}
		if err := can(perm.Edit); err != nil {
			return err
		}
		var v *string
		if err := f.get(key, &v); err != nil {
			return err
		}
		var newVal any
		if v != nil && strings.TrimSpace(*v) != "" {
			newVal = *v
			if key == "summary" {
				newVal = strings.TrimSpace(*v)
			}
		}
		if required && newVal == nil {
			return fail(http.StatusBadRequest, i18n.IssueSummaryEmpty)
		}
		if s, ok := newVal.(string); ok && maxLen > 0 && utf8.RuneCountInString(s) > maxLen {
			return failf(http.StatusBadRequest, i18n.FieldTooLong, maxLen)
		}
		var old any
		switch key {
		case "summary":
			old = iss.Summary
		case "description":
			old = sqlq.Value(iss.Description)
		case "environment":
			old = sqlq.Value(iss.Environment)
		}
		if old != newVal {
			sets[column] = newVal
			changes = append(changes, change{field: key, oldString: old, newString: newVal})
		}
		return nil
	}
	if err := text("summary", "summary", true, 255); err != nil {
		return err
	}
	if err := text("description", "description", false, 0); err != nil {
		return err
	}
	if err := text("environment", "environment", false, 0); err != nil {
		return err
	}

	if f.has("type") {
		if err := can(perm.Edit); err != nil {
			return err
		}
		var v string
		if err := f.get("type", &v); err != nil {
			return err
		}
		it, ok := cat.issueType[v]
		if !ok || !s.projectHasType(ctx, iss.ProjectID, v) {
			return fail(http.StatusBadRequest, i18n.IssueTypeUnavailable)
		}
		if it.Subtask != iss.ParentID.Valid {
			return fail(http.StatusBadRequest, i18n.IssueSubtaskTypeChange)
		}
		if v != iss.TypeID {
			sets["issue_type_id"] = v
			changes = append(changes, change{"issuetype", iss.TypeID, cat.typeOf(iss.TypeID).Name, v, it.Name})
		}
	}

	if f.has("priority") {
		if err := can(perm.Edit); err != nil {
			return err
		}
		var v *string
		if err := f.get("priority", &v); err != nil {
			return err
		}
		var newVal, newName any
		if v != nil {
			p, ok := cat.priority[*v]
			if !ok {
				return fail(http.StatusBadRequest, i18n.IssuePriorityUnknown)
			}
			newVal, newName = *v, p.Name
		}
		if sqlq.Value(iss.PriorityID) != newVal {
			var oldName any
			if p := cat.priorityOf(iss.PriorityID); p != nil {
				oldName = p.Name
			}
			sets["priority_id"] = newVal
			changes = append(changes, change{"priority", sqlq.Value(iss.PriorityID), oldName, newVal, newName})
		}
	}

	if f.has("assignee") {
		var v *int64
		if err := f.get("assignee", &v); err != nil {
			return err
		}
		selfAssign := v != nil && *v == sub.UserID
		if !sub.Can(perm.Assign, iss.ProjectID, &pi) && !(selfAssign && sub.Can(perm.Assignable, iss.ProjectID, &pi)) {
			return errForbidden
		}
		var newVal any
		if v != nil {
			ok, err := s.isAssignable(ctx, iss.ProjectID, *v)
			if err != nil {
				return err
			}
			if !ok {
				return fail(http.StatusBadRequest, i18n.IssueAssigneeNotAssignable)
			}
			newVal = *v
		}
		if sqlq.Value(iss.AssigneeID) != newVal {
			c := change{field: "assignee"}
			if iss.AssigneeID.Valid {
				c.oldValue, c.oldString = s.userName(ctx, iss.AssigneeID.Int64)
			}
			if v != nil {
				c.newValue, c.newString = s.userName(ctx, *v)
			}
			sets["assignee_id"] = newVal
			changes = append(changes, c)
		}
	}

	if f.has("due") {
		if err := can(perm.Edit); err != nil {
			return err
		}
		var v *string
		if err := f.get("due", &v); err != nil {
			return err
		}
		var newVal any
		if v != nil && *v != "" {
			if _, err := time.Parse(time.DateOnly, *v); err != nil {
				return fail(http.StatusBadRequest, i18n.IssueDueDateInvalid)
			}
			newVal = *v
		}
		var old any
		if d := dateOnly(iss.Due); d != nil {
			old = *d
		}
		if old != newVal {
			sets["due_date"] = newVal
			changes = append(changes, change{field: "duedate", oldValue: old, oldString: old, newValue: newVal, newString: newVal})
		}
	}

	if f.has("originalEstimate") || f.has("remainingEstimate") {
		if err := can(perm.Edit); err != nil {
			return err
		}
		edit, err := readEstimateEdit(f)
		if err != nil {
			return err
		}
		cur := timeEstimates{Original: iss.Original, Remaining: iss.Remaining, Spent: iss.Spent}
		next := cur.withEstimateEdit(edit)
		if next.Original != cur.Original {
			sets["original_estimate"] = sqlq.Value(next.Original)
		}
		if next.Remaining != cur.Remaining {
			sets["remaining_estimate"] = sqlq.Value(next.Remaining)
		}
		changes = append(changes, cur.changesTo(next)...)
	}

	if f.has("epic") {
		if err := can(perm.Edit); err != nil {
			return err
		}
		epicID, epicKey, err := s.epicTarget(ctx, sub, cat, iss, f)
		if err != nil {
			return err
		}
		if sqlq.Value(iss.EpicID) != epicID {
			c := change{field: "Epic Link", newValue: epicID, newString: epicKey}
			if iss.EpicID.Valid {
				if old, err := s.findIssueByID(ctx, iss.EpicID.Int64); err == nil {
					c.oldValue, c.oldString = iss.EpicID.Int64, old.Key()
				}
			}
			sets["epic_id"] = epicID
			changes = append(changes, c)
		}
	}

	if f.has("securityLevel") {
		if err := can(perm.Security); err != nil {
			return err
		}
		if iss.ParentID.Valid {
			return fail(http.StatusBadRequest, i18n.IssueSubtaskSecurity)
		}
		var v *int64
		if err := f.get("securityLevel", &v); err != nil {
			return err
		}
		var newVal any
		if v != nil {
			if !s.projectHasLevel(ctx, iss.ProjectID, *v) {
				return fail(http.StatusBadRequest, i18n.IssueSecurityLevelUnknown)
			}
			newVal = *v
		}
		if sqlq.Value(iss.LevelID) != newVal {
			sets["security_level_id"] = newVal
			changes = append(changes, change{field: "security", oldValue: sqlq.Value(iss.LevelID), newValue: newVal})
		}
	}

	// Status, and the resolution that goes with it: reaching a "done" status
	// resolves the issue, leaving it reopens it.
	if f.has("status") || f.has("resolution") {
		if err := can(perm.Transition); err != nil {
			return err
		}
		status := iss.StatusID
		if f.has("status") {
			if err := f.get("status", &status); err != nil {
				return err
			}
			if _, ok := cat.status[status]; !ok {
				return fail(http.StatusBadRequest, i18n.StatusUnknown)
			}
			if status != iss.StatusID {
				_, allowed, err := s.issueTransitions(ctx, cat, iss)
				if err != nil {
					return err
				}
				if !slices.ContainsFunc(allowed, func(st statusRef) bool { return st.ID == status }) {
					return fail(http.StatusConflict, i18n.IssueTransitionNotAllowed)
				}
			}
		}
		resolution := iss.ResolutionID
		if f.has("resolution") {
			var v *string
			if err := f.get("resolution", &v); err != nil {
				return err
			}
			resolution = sql.NullString{}
			if v != nil {
				if _, ok := cat.resolution[*v]; !ok {
					return fail(http.StatusBadRequest, i18n.IssueResolutionUnknown)
				}
				resolution = sql.NullString{String: *v, Valid: true}
			}
		}
		resolution = statusResolution(cat, resolution, status)
		if status != iss.StatusID {
			sets["status_id"] = status
			changes = append(changes, change{"status", iss.StatusID, cat.statusOf(iss.StatusID).Name, status,
				cat.statusOf(status).Name})
		}
		if resolution != iss.ResolutionID {
			sets["resolution_id"] = sqlq.Value(resolution)
			var oldName, newName any
			if v := cat.resolutionOf(iss.ResolutionID); v != nil {
				oldName = v.Name
			}
			if v := cat.resolutionOf(resolution); v != nil {
				newName = v.Name
			}
			changes = append(changes, change{"resolution", sqlq.Value(iss.ResolutionID), oldName, sqlq.Value(resolution), newName})
			if resolution.Valid {
				sets["resolved_at"] = time.Now().UTC()
			} else {
				sets["resolved_at"] = nil
			}
		}
	}

	if f.has("labels") || f.has("components") || f.has("affectsVersions") || f.has("fixVersions") {
		if err := can(perm.Edit); err != nil {
			return err
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	setChanges, err := s.applySets(ctx, tx, f, iss)
	if err != nil {
		return err
	}
	changes = append(changes, setChanges...)
	if len(changes) == 0 {
		return nil
	}
	now := time.Now().UTC()
	sets["updated_at"] = now
	cols := make([]string, 0, len(sets))
	for c := range sets {
		cols = append(cols, c)
	}
	sort.Strings(cols)
	var assigns []string
	var args []any
	for _, c := range cols {
		assigns = append(assigns, c+" = ?")
		args = append(args, sets[c])
	}
	if _, err := tx.ExecContext(ctx, `UPDATE issues SET `+strings.Join(assigns, ", ")+` WHERE id = ?`,
		append(args, iss.ID)...); err != nil {
		return err
	}
	if level, ok := sets["security_level_id"]; ok {
		if _, err := tx.ExecContext(ctx, `UPDATE issues SET security_level_id = ?, updated_at = ? WHERE parent_id = ?`,
			level, now, iss.ID); err != nil {
			return err
		}
	}
	if err := recordChanges(ctx, tx, iss.ID, sub.UserID, now, changes); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	updated, err := s.findIssueByID(ctx, iss.ID)
	if err != nil {
		return err
	}
	// Announce with the old visibility too: someone who just lost access
	// must learn the issue changed.
	e := issueEvent(events.IssueUpdated, updated, sub.UserID, "issue")
	e.Changes = eventChanges(changes)
	if before := iss.perm(); before != updated.perm() {
		e.Before = &before
	}
	s.emit(e)
	return nil
}

func (s *Server) respondIssue(w http.ResponseWriter, r *http.Request, sub *perm.Subject, id int64) error {
	iss, err := s.findIssueByID(r.Context(), id)
	if err != nil {
		return err
	}
	if !sub.CanSee(iss.perm()) {
		return writeJSON(w, http.StatusOK, map[string]any{"key": iss.Key(), "hidden": true})
	}
	d, err := s.issueDetail(r.Context(), sub, iss)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, d)
}

func (s *Server) deleteIssue(w http.ResponseWriter, r *http.Request) error {
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
	if !sub.Can(perm.Delete, iss.ProjectID, &pi) {
		return errForbidden
	}
	// Deleting an issue deletes its sub-tasks: each must be deletable too.
	children, err := s.db.QueryContext(ctx, `SELECT `+issueColumns+` FROM issues i
		JOIN projects p ON p.id = i.project_id WHERE i.parent_id = ?`, iss.ID)
	if err != nil {
		return err
	}
	gone := []*issueRow{iss}
	for children.Next() {
		child, err := scanIssue(children)
		if err != nil {
			children.Close()
			return err
		}
		cp := child.perm()
		if !sub.Can(perm.Delete, child.ProjectID, &cp) {
			children.Close()
			return fail(http.StatusConflict, i18n.IssueSubtasksNotDeletable)
		}
		gone = append(gone, child)
	}
	children.Close()
	if err := children.Err(); err != nil {
		return err
	}
	// Deletions are announced with the issues as they were: e-mails and
	// webhooks are written once they are gone.
	var deleted []events.Event
	for _, d := range gone {
		e := issueEvent(events.IssueDeleted, d, sub.UserID, "")
		if e.Issue, err = events.LoadIssue(ctx, s.db, d.ID); err != nil {
			return err
		}
		e.HookMatches = s.out.hooks.MatchBeforeDelete(ctx, d.ID)
		deleted = append(deleted, e)
	}
	deleted[0].Live = "deleted"
	var files []int64
	if err := sqlq.Each(ctx, s.db, `SELECT a.id FROM attachments a JOIN issues i ON i.id = a.issue_id
		WHERE i.id = ? OR i.parent_id = ?`, func(rows *sql.Rows) error {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return err
		}
		files = append(files, id)
		return nil
	}, iss.ID, iss.ID); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM issues WHERE id = ?`, iss.ID); err != nil {
		return err
	}
	for _, id := range files {
		os.Remove(filepath.Join(s.dataDir, "attachments", strconv.FormatInt(id, 10)))
	}
	for _, e := range deleted {
		s.emit(e)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// compileJQL turns a JQL query into SQL. Syntax and field errors come back
// to the user as they are (in the user's language).
// searchCondition turns a search's parameters (jql, or q in either syntax,
// and project) into a condition on the issues the user sees (aliases i and
// p), with its ORDER BY and the syntax used.
func (s *Server) searchCondition(ctx context.Context, sub *perm.Subject, params url.Values) (where string, args []any,
	sort, mode string, err error) {
	// The search bar takes both syntaxes: JQL is recognised by its shape.
	mode = "simple"
	var cond string
	var cargs []any
	if src := params.Get("jql"); src != "" || jql.LooksLike(params.Get("q")) {
		if src == "" {
			src = params.Get("q")
		}
		mode = "jql"
		if cond, cargs, sort, err = s.compileJQL(ctx, sub, src); err != nil {
			return "", nil, "", "", err
		}
	} else {
		q := search.Parse(params.Get("q"))
		cond, cargs = q.SQL(sub.UserID)
		sort = q.Sort
	}
	vis, args := sub.VisibleSQL("i")
	where = vis + " AND " + cond
	args = append(args, cargs...)
	if p := params.Get("project"); p != "" {
		where += " AND p.pkey = ?"
		args = append(args, strings.ToUpper(p))
	}
	return where, args, sort, mode, nil
}

func (s *Server) compileJQL(ctx context.Context, sub *perm.Subject, src string) (string, []any, string, error) {
	q, err := jql.Parse(src)
	if err != nil {
		return "", nil, "", jqlError(err)
	}
	fields := map[string]jql.Field{}
	if err := sqlq.Each(ctx, s.db, `SELECT id, name, type_key FROM custom_fields`, func(rows *sql.Rows) error {
		var f jql.Field
		if err := rows.Scan(&f.ID, &f.Name, &f.TypeKey); err != nil {
			return err
		}
		fields[foldName(f.Name)] = f
		fields[strconv.FormatInt(f.ID, 10)] = f
		return nil
	}); err != nil {
		return "", nil, "", err
	}
	where, args, order, err := jql.Compile(q, jql.Env{
		Me:  sub.UserID,
		Now: time.Now(),
		Loc: s.zone(),
		CustomField: func(name string) (jql.Field, bool) {
			f, ok := fields[foldName(name)]
			return f, ok
		},
		Query: func(q string, args []any) ([]string, error) {
			var out []string
			err := sqlq.Each(ctx, s.db, q, func(rows *sql.Rows) error {
				var v sql.NullString
				if err := rows.Scan(&v); err != nil {
					return err
				}
				if v.Valid {
					out = append(out, v.String)
				}
				return nil
			}, args...)
			return out, err
		},
	})
	if err != nil {
		return "", nil, "", jqlError(err)
	}
	return where, args, order, nil
}

// jqlError answers a query the server cannot read: 400 with the message and
// where in the query (in characters) the problem is, when known.
func jqlError(err error) error {
	var je *jql.Error
	if !errors.As(err, &je) {
		return err
	}
	return &httpError{status: http.StatusBadRequest, msg: i18n.M(i18n.JQLError, i18n.M(je.Key, je.Args...)),
		extra: map[string]any{"pos": je.Pos}}
}

// foldName compares field names the forgiving way: case and French accents
// do not matter ("Fonctionnalité" finds "Fonctionnalite").
func foldName(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case 'à', 'â', 'ä':
			return 'a'
		case 'é', 'è', 'ê', 'ë':
			return 'e'
		case 'î', 'ï':
			return 'i'
		case 'ô', 'ö':
			return 'o'
		case 'ù', 'û', 'ü':
			return 'u'
		case 'ç':
			return 'c'
		}
		return r
	}, strings.ToLower(strings.TrimSpace(s)))
}

func nullInt64(v any) sql.NullInt64 {
	if id, ok := v.(int64); ok {
		return sql.NullInt64{Int64: id, Valid: true}
	}
	return sql.NullInt64{}
}

// isEpicType tells whether an issue type is an epic (by name, as Jira
// Software does not mark it otherwise).
func isEpicType(t typeRef) bool {
	n := strings.ToLower(t.Name)
	return n == "epic" || n == "épopée" || n == "epopee"
}

// epicTarget reads the "epic" field (an issue key, or null to detach) and
// checks the target: an epic the user sees. Sub-tasks follow their parent's
// epic, and an epic cannot be put in an epic.
func (s *Server) epicTarget(ctx context.Context, sub *perm.Subject, cat *catalog, iss *issueRow, f fields) (any, any, error) {
	if f.isNull("epic") {
		return nil, nil, nil
	}
	var key string
	if err := f.get("epic", &key); err != nil {
		return nil, nil, err
	}
	if iss.ParentID.Valid {
		return nil, nil, fail(http.StatusBadRequest, i18n.IssueSubtaskEpic)
	}
	if iss.TypeID != "" && isEpicType(cat.typeOf(iss.TypeID)) {
		return nil, nil, fail(http.StatusBadRequest, i18n.IssueEpicInEpic)
	}
	epic, err := s.findIssue(ctx, key)
	if err != nil || !sub.CanSee(epic.perm()) {
		return nil, nil, fail(http.StatusBadRequest, i18n.IssueEpicNotFound)
	}
	if !isEpicType(cat.typeOf(epic.TypeID)) {
		return nil, nil, failf(http.StatusBadRequest, i18n.IssueNotAnEpic, epic.Key())
	}
	return epic.ID, epic.Key(), nil
}
