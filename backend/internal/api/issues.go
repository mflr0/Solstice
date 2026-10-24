package api

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"solstice/internal/audit"
	"solstice/internal/events"
	"solstice/internal/i18n"
	"solstice/internal/issue"
	"solstice/internal/perm"
	"solstice/internal/sqlq"
)

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

// scanIssue scans the issueColumns, then the columns after them into extra.
func scanIssue(sc scanner, extra ...any) (*issueRow, error) {
	var i issueRow
	err := sc.Scan(append([]any{&i.ID, &i.ProjectID, &i.Num, &i.ProjectKey, &i.ProjectName, &i.Summary, &i.Description,
		&i.Environment, &i.TypeID, &i.StatusID, &i.PriorityID, &i.ResolutionID, &i.ReporterID, &i.AssigneeID,
		&i.CreatorID, &i.LevelID, &i.ParentID, &i.Created, &i.Updated, &i.Resolved, &i.Due,
		&i.Original, &i.Remaining, &i.Spent, &i.EpicID, &i.Rank}, extra...)...)
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
	projectKey, num, ok := issue.ParseKey(strings.TrimSpace(key))
	if !ok {
		return nil, errNotFound
	}
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
	// Values holds the columns asked for, as text (as in the CSV export).
	Values map[string]string `json:"values,omitempty"`
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

// listIssues answers GET /api/issues: a page of a search. columns may name
// more fields, as for the export, whose values come back as text in the time
// zone tz.
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
	var custom []customFieldDef
	var columns []exportColumn
	if v := strings.TrimSpace(params.Get("columns")); v != "" {
		if custom, err = s.customFields(r.Context()); err != nil {
			return err
		}
		if columns, err = parseColumns(v, locale(r), custom); err != nil {
			return err
		}
	}
	o := s.restOut(r)
	if len(columns) > 0 {
		if o.tz, err = s.columnZone(r); err != nil {
			return err
		}
	}
	page := min(10000, max(1, atoi(params.Get("page"), 1)))
	size := min(200, max(1, atoi(params.Get("size"), 50)))
	where, args, order, err := s.searchCondition(r.Context(), sub, params)
	if err != nil {
		return err
	}
	list, total, err := s.searchRows(r.Context(), where, args, order, (page-1)*size, size)
	if err != nil {
		return err
	}
	var ids []int64
	for _, iss := range list {
		ids = append(ids, iss.AssigneeID.Int64, iss.ReporterID.Int64)
	}
	users, err := s.users(r.Context(), ids...)
	if err != nil {
		return err
	}
	out := make([]issueSummary, 0, len(list))
	for _, iss := range list {
		out = append(out, summarize(iss, cat, users))
	}
	if len(columns) > 0 && len(list) > 0 {
		set, jusers, err := s.loadColumns(r.Context(), sub, o, custom, list, columns)
		if err != nil {
			return err
		}
		for i, d := range set.list {
			out[i].Values = make(map[string]string, len(columns))
			for _, c := range columns {
				out[i].Values[c.id] = o.exportCell(cat, set, jusers, d, c)
			}
		}
	}
	return writeJSON(w, http.StatusOK, map[string]any{"total": total, "page": page, "size": size, "issues": out})
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
	d := detailBase(sub, iss)
	refs, err := s.detailLists(ctx, sub, iss, d)
	if err != nil {
		return nil, err
	}
	custom, customUsers, err := s.customValues(ctx, sub, iss.ID)
	if err != nil {
		return nil, err
	}
	refs.userIDs = append(refs.userIDs, customUsers...)
	users, err := s.detailPeople(ctx, sub, cat, iss, d, refs)
	if err != nil {
		return nil, err
	}
	detailCustom(d, custom, users)
	if err := s.detailAccess(ctx, sub, cat, iss, d); err != nil {
		return nil, err
	}
	return d, nil
}

// detailRefs is what detailLists read that names users, rendered once they are loaded.
type detailRefs struct {
	related []relatedIssue
	atts    []detailAttachment
	userIDs []int64
}

type detailAttachment struct {
	out    attachmentOut
	author sql.NullInt64
}

// detailBase fills the fields that come from the issue row alone.
func detailBase(sub *perm.Subject, iss *issueRow) *issueDetail {
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
		Sprints: []sprintRef{}, Can: map[string]bool{},
	}
	d.Resolved = sqlq.Time(iss.Resolved)
	for _, p := range perm.All {
		d.Can[p] = sub.Can(p, iss.ProjectID, &pi)
	}
	return d
}

// detailLists loads the issue's lists: those without users go into d, the
// related issues and attachments are returned with the users they name.
func (s *Server) detailLists(ctx context.Context, sub *perm.Subject, iss *issueRow, d *issueDetail) (*detailRefs, error) {
	refs := &detailRefs{userIDs: []int64{iss.ReporterID.Int64, iss.AssigneeID.Int64, iss.CreatorID.Int64}}

	// Parent, sub-tasks and linked issues, keeping only those the user sees.
	related, err := s.relatedIssues(ctx, sub, iss)
	if err != nil {
		return nil, err
	}
	refs.related = related
	for _, rel := range related {
		refs.userIDs = append(refs.userIDs, rel.row.AssigneeID.Int64)
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
	if err := sqlq.Each(ctx, s.db, `SELECT id, filename, mime_type, size, created_at, author_id, file_missing
		FROM attachments WHERE issue_id = ? ORDER BY created_at, id`, func(rows *sql.Rows) error {
		var a detailAttachment
		if err := rows.Scan(&a.out.ID, &a.out.Filename, &a.out.MimeType, &a.out.Size, &a.out.Created, &a.author,
			&a.out.Missing); err != nil {
			return err
		}
		a.out.URL = "/api/attachments/" + strconv.FormatInt(a.out.ID, 10)
		a.out.CanDelete = d.Can[perm.Moderate] || (d.Can[perm.Attach] && a.author.Int64 == sub.UserID)
		refs.atts = append(refs.atts, a)
		refs.userIDs = append(refs.userIDs, a.author.Int64)
		return nil
	}, iss.ID); err != nil {
		return nil, err
	}
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
	return refs, nil
}

// detailPeople loads the watchers and votes, then the users that refs name,
// and renders everything that shows a user.
func (s *Server) detailPeople(ctx context.Context, sub *perm.Subject, cat *catalog, iss *issueRow, d *issueDetail,
	refs *detailRefs) (map[int64]userRef, error) {
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(user_id = ?), 0) FROM watchers WHERE issue_id = ?`,
		sub.UserID, iss.ID).Scan(&d.Watchers.Count, &d.Watchers.Me); err != nil {
		return nil, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(user_id = ?), 0) FROM votes WHERE issue_id = ?`,
		sub.UserID, iss.ID).Scan(&d.Votes.Count, &d.Votes.Me); err != nil {
		return nil, err
	}

	users, err := s.users(ctx, refs.userIDs...)
	if err != nil {
		return nil, err
	}
	d.issueSummary = summarize(iss, cat, users)
	d.Creator = refOf(users, iss.CreatorID)
	for _, a := range refs.atts {
		a.out.Author = refOf(users, a.author)
		d.Attachments = append(d.Attachments, a.out)
	}
	for _, rel := range refs.related {
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
	return users, nil
}

// detailCustom adds the custom field values, with the users they name.
func detailCustom(d *issueDetail, custom []customRow, users map[int64]userRef) {
	for i := range custom {
		for _, id := range custom[i].userIDs {
			if u, ok := users[id]; ok {
				custom[i].out.Users = append(custom[i].out.Users, u)
			}
		}
		d.CustomFields = append(d.CustomFields, custom[i].out)
	}
}

// detailAccess fills what depends on the user's access: security levels,
// transitions, the visible comments and the epic.
func (s *Server) detailAccess(ctx context.Context, sub *perm.Subject, cat *catalog, iss *issueRow, d *issueDetail) error {
	if iss.LevelID.Valid {
		var name string
		err := s.db.QueryRowContext(ctx, `SELECT name FROM security_levels WHERE id = ?`, iss.LevelID.Int64).Scan(&name)
		switch {
		case err == nil:
			d.SecurityLevel = &idName{ID: iss.LevelID.Int64, Name: name}
		case !errors.Is(err, sql.ErrNoRows):
			return err
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
			return err
		}
	}
	var err error
	d.Transitions, d.Statuses, err = s.issueTransitions(ctx, cat, iss)
	if err != nil {
		return err
	}
	comments, err := s.visibleComments(ctx, sub, iss)
	if err != nil {
		return err
	}
	d.CommentCount = len(comments)

	if !iss.EpicID.Valid {
		return nil
	}
	epic, err := s.findIssueByID(ctx, iss.EpicID.Int64)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if !sub.CanSee(epic.perm()) {
		return nil
	}
	eu, err := s.users(ctx, epic.AssigneeID.Int64)
	if err != nil {
		return err
	}
	d.Epic = &issueBrief{Key: epic.Key(), Summary: epic.Summary, Type: cat.typeOf(epic.TypeID),
		Status: cat.statusOf(epic.StatusID), Assignee: refOf(eu, epic.AssigneeID)}
	return nil
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
	var out []relatedIssue
	err := sqlq.Each(ctx, s.db, q, func(rows *sql.Rows) error {
		var rel relatedIssue
		row, err := scanIssue(rows, &rel.kind, &rel.linkID, &rel.linkType)
		if err != nil {
			return err
		}
		if sub.CanSee(row.perm()) {
			rel.row = row
			out = append(out, rel)
		}
		return nil
	}, "parent", iss.ParentID.Int64, "subtask", iss.ID, iss.ID, iss.ID)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].kind == "subtask" && out[b].kind == "subtask" {
			return out[a].row.Num < out[b].row.Num
		}
		return false
	})
	return out, nil
}

// typeStatuses are the statuses an issue of a type can have in a project.
func (s *Server) typeStatuses(ctx context.Context, cat *catalog, projectID int64, typeID string) ([]statusRef, error) {
	wf, _, err := s.workflowOf(ctx, projectID, typeID)
	if err != nil {
		return nil, err
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

// customRows collects the rendered values one row per field, in the order fields first appear.
type customRows struct {
	out     []customRow
	byField map[int64]int
	userIDs []int64
}

func (c *customRows) get(id int64, name, kind string) *customRow {
	if i, ok := c.byField[id]; ok {
		return &c.out[i]
	}
	c.byField[id] = len(c.out)
	c.out = append(c.out, customRow{out: customValue{ID: id, Name: name, Kind: kind, Values: []string{}}})
	return &c.out[len(c.out)-1]
}

// storedCustom is one row of custom_field_values with its field.
type storedCustom struct {
	field          int64
	name, typeKey  string
	parentKey, str sql.NullString
	num            sql.NullFloat64
	text           sql.NullString
	date           sql.NullTime
}

// customValues renders an issue's custom field values for display.
func (s *Server) customValues(ctx context.Context, sub *perm.Subject, issueID int64) ([]customRow, []int64, error) {
	vals, options, err := s.loadCustomRows(ctx, issueID)
	if err != nil {
		return nil, nil, err
	}
	rows := &customRows{byField: map[int64]int{}}
	for _, v := range vals {
		renderCustomKind(rows, v, options)
	}
	if err := s.epicLinkValues(ctx, sub, rows, vals); err != nil {
		return nil, nil, err
	}
	if err := s.labelFieldValues(ctx, rows, issueID); err != nil {
		return nil, nil, err
	}
	sort.SliceStable(rows.out, func(a, b int) bool { return rows.out[a].out.Name < rows.out[b].out.Name })
	return rows.out, rows.userIDs, nil
}

// loadCustomRows reads the issue's custom field values and the labels of the options they name.
func (s *Server) loadCustomRows(ctx context.Context, issueID int64) ([]storedCustom, map[int64]string, error) {
	var vals []storedCustom
	optionIDs := map[int64]bool{}
	if err := sqlq.Each(ctx, s.db, `SELECT v.field_id, f.name, f.type_key, v.parent_key, v.string_value,
		v.number_value, v.text_value, v.date_value FROM custom_field_values v JOIN custom_fields f ON f.id = v.field_id
		WHERE v.issue_id = ? ORDER BY f.name, v.parent_key IS NOT NULL, v.id`, func(rows *sql.Rows) error {
		var v storedCustom
		if err := rows.Scan(&v.field, &v.name, &v.typeKey, &v.parentKey, &v.str, &v.num, &v.text, &v.date); err != nil {
			return err
		}
		if id, err := strconv.ParseInt(v.str.String, 10, 64); err == nil && optionKind(issue.FieldKind(v.typeKey)) {
			optionIDs[id] = true
		}
		vals = append(vals, v)
		return nil
	}, issueID); err != nil {
		return nil, nil, err
	}
	options := map[int64]string{}
	if len(optionIDs) == 0 {
		return vals, options, nil
	}
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
	return vals, options, nil
}

// renderCustomKind adds one stored value to rows, rendered by its field's kind.
// Epic links are left to epicLinkValues.
func renderCustomKind(rows *customRows, v storedCustom, options map[int64]string) {
	kind := issue.FieldKind(v.typeKey)
	if hiddenKinds[kind] {
		return
	}
	switch {
	case optionKind(kind):
		id, _ := strconv.ParseInt(v.str.String, 10, 64)
		label, ok := options[id]
		if !ok {
			label = v.str.String
		}
		row := rows.get(v.field, v.name, "option")
		if kind == "cascadingselect" && v.parentKey.Valid && len(row.out.Values) > 0 {
			row.out.Values[len(row.out.Values)-1] += " › " + label
		} else {
			row.out.Values = append(row.out.Values, label)
		}
	case kind == "userpicker" || kind == "multiuserpicker":
		if id, err := strconv.ParseInt(v.str.String, 10, 64); err == nil {
			row := rows.get(v.field, v.name, "user")
			row.userIDs = append(row.userIDs, id)
			rows.userIDs = append(rows.userIDs, id)
		}
	case kind == "datepicker":
		if v.date.Valid {
			row := rows.get(v.field, v.name, "date")
			row.out.Values = append(row.out.Values, v.date.Time.Format(time.DateOnly))
		}
	case kind == "datetime":
		if v.date.Valid {
			row := rows.get(v.field, v.name, "datetime")
			row.out.Values = append(row.out.Values, v.date.Time.UTC().Format(time.RFC3339))
		}
	case kind == "float" || v.num.Valid && !v.str.Valid && !v.text.Valid:
		if kind == "gh-epic-link" {
			return
		}
		row := rows.get(v.field, v.name, "number")
		row.out.Values = append(row.out.Values, strconv.FormatFloat(v.num.Float64, 'f', -1, 64))
	case kind == "textarea":
		row := rows.get(v.field, v.name, "wiki")
		row.out.Values = append(row.out.Values, v.text.String)
	case kind == "url":
		row := rows.get(v.field, v.name, "url")
		row.out.Values = append(row.out.Values, v.str.String)
	default:
		val := v.str.String
		if !v.str.Valid && v.text.Valid {
			val = v.text.String
		}
		if val != "" {
			row := rows.get(v.field, v.name, "text")
			row.out.Values = append(row.out.Values, val)
		}
	}
}

// epicLinkValues shows the key of the issue an epic link points at, if the user may see it.
func (s *Server) epicLinkValues(ctx context.Context, sub *perm.Subject, rows *customRows, vals []storedCustom) error {
	for _, v := range vals {
		if issue.FieldKind(v.typeKey) != "gh-epic-link" || !v.num.Valid {
			continue
		}
		iss, err := s.findIssueByID(ctx, int64(v.num.Float64))
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		if !sub.CanSee(iss.perm()) {
			continue
		}
		row := rows.get(v.field, v.name, "issue")
		row.out.Values = append(row.out.Values, iss.Key())
	}
	return nil
}

// labelFieldValues adds the labels-type custom fields, whose values live in the labels table.
func (s *Server) labelFieldValues(ctx context.Context, rows *customRows, issueID int64) error {
	return sqlq.Each(ctx, s.db, `SELECT f.id, f.name, l.label FROM labels l JOIN custom_fields f ON f.id = l.field_id
		WHERE l.issue_id = ? ORDER BY f.name, l.label`, func(r *sql.Rows) error {
		var id int64
		var name, label string
		if err := r.Scan(&id, &name, &label); err != nil {
			return err
		}
		row := rows.get(id, name, "label")
		row.out.Values = append(row.out.Values, label)
		return nil
	}, issueID)
}

func optionKind(kind string) bool {
	switch kind {
	case "select", "multiselect", "radiobuttons", "multicheckboxes", "cascadingselect", "gh-epic-status":
		return true
	}
	return false
}

// hiddenKinds are the custom field kinds the interface does not show: Jira
// Software's internal fields (ranking, colours, version bookkeeping,
// development panel) and the sprint field, as an issue shows its sprints.
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
	return recordHistory(ctx, tx, issueID, author, at, changes, nil)
}

// recordHistory records one group of history lines: changes to the issue's
// own fields, then to its custom fields (which Jira files apart).
func recordHistory(ctx context.Context, tx *sql.Tx, issueID, author int64, at time.Time, changes, custom []change) error {
	if len(changes)+len(custom) == 0 {
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
	for _, part := range []struct {
		fieldType string
		changes   []change
	}{{"jira", changes}, {"custom", custom}} {
		for _, c := range part.changes {
			if _, err := tx.ExecContext(ctx, `INSERT INTO change_items (group_id, field_type, field, old_value,
				old_string, new_value, new_string) VALUES (?, ?, ?, ?, ?, ?, ?)`,
				group, part.fieldType, c.field, c.oldValue, c.oldString, c.newValue, c.newString); err != nil {
				return err
			}
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
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, maxJSONBody))
	if err := dec.Decode(&f); err != nil {
		return nil, badBody(err)
	}
	for k := range f {
		if !issueFields[k] {
			return nil, failf(http.StatusBadRequest, i18n.FieldUnknown, k)
		}
	}
	return f, nil
}

func (s *Server) createIssue(w http.ResponseWriter, r *http.Request) error {
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	f, err := readFields(r)
	if err != nil {
		return err
	}
	created, err := s.newIssue(r.Context(), sub, f, nil)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusCreated, map[string]string{"key": created.Key()})
}

// newIssue creates an issue from the interface's fields, with the custom
// field values of f and extra, and announces it once it is stored.
func (s *Server) newIssue(ctx context.Context, sub *perm.Subject, f fields, extra []customWrite) (*issueRow, error) {
	p, err := s.planIssue(ctx, sub, f, createOpts{extra: extra})
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := s.insertIssue(ctx, tx, sub, p); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	created, err := s.findIssueByID(ctx, p.id)
	if err != nil {
		return nil, err
	}
	s.emit(issueEvent(events.IssueCreated, created, sub.UserID, "issue"))
	return created, nil
}

// createOpts are what an import sets beyond the interface's fields.
type createOpts struct {
	extra []customWrite
	// parent and epic are issues of the same import, inserted first;
	// epicKey names the epic in messages.
	parent, epic *issuePlan
	epicKey      string
	status       string // "": the workflow's initial status
	resolution   string // with status; "": the one the status implies
	reporter     int64  // 0: the creator
}

// issuePlan is an issue creation that passed every check, ready to insert.
type issuePlan struct {
	f                                       fields
	projectID                               int64
	typeID, summary, status                 string
	typ                                     typeRef
	parentID, epicID                        any
	parent, epic                            *issuePlan
	description, environment, priority, due any
	assignee, level                         any
	resolution                              sql.NullString
	reporter                                int64
	times                                   timeEstimates
	custom                                  []customWrite
	id, num                                 int64 // once inserted
}

// planIssue checks an issue creation without writing anything.
func (s *Server) planIssue(ctx context.Context, sub *perm.Subject, f fields, o createOpts) (*issuePlan, error) {
	var projectKey, typeID, summary string
	for k, v := range map[string]*string{"project": &projectKey, "type": &typeID, "summary": &summary} {
		if !f.has(k) {
			return nil, failf(http.StatusBadRequest, i18n.FieldRequired, k)
		}
		if err := f.get(k, v); err != nil {
			return nil, err
		}
	}
	summary = strings.TrimSpace(summary)
	if summary == "" || utf8.RuneCountInString(summary) > sqlq.VarcharWidth {
		return nil, fail(http.StatusBadRequest, i18n.IssueSummaryRequired)
	}
	var projectID, leadID int64
	var defaultAssignee string
	var lead sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `SELECT p.id, p.lead_id, p.default_assignee FROM projects p WHERE `+sqlq.ProjectKey("p", "?"),
		strings.ToUpper(projectKey), strings.ToUpper(projectKey)).Scan(&projectID, &lead, &defaultAssignee); errors.Is(err, sql.ErrNoRows) {
		return nil, errNotFound
	} else if err != nil {
		return nil, err
	}
	leadID = lead.Int64
	if !sub.Can(perm.Create, projectID, nil) || !sub.CanBrowseProject(projectID) {
		return nil, errForbidden
	}
	cat, err := s.catalogue(ctx)
	if err != nil {
		return nil, err
	}
	it, ok := cat.issueType[typeID]
	if ok {
		if ok, err = s.projectHasType(ctx, projectID, typeID); err != nil {
			return nil, err
		}
	}
	if !ok {
		return nil, fail(http.StatusBadRequest, i18n.IssueTypeUnavailable)
	}
	p := &issuePlan{f: f, projectID: projectID, typeID: typeID, summary: summary, typ: it, reporter: sub.UserID,
		custom: o.extra}
	if o.reporter != 0 {
		p.reporter = o.reporter
	}

	var parentLevel sql.NullInt64
	switch {
	case o.parent != nil:
		if o.parent.parentID != nil || o.parent.parent != nil {
			return nil, fail(http.StatusBadRequest, i18n.IssueParentInvalid)
		}
		p.parent = o.parent
		parentLevel = nullInt64(o.parent.level)
	case f.has("parent") && !f.isNull("parent"):
		var parentKey string
		if err := f.get("parent", &parentKey); err != nil {
			return nil, err
		}
		parent, err := s.findIssue(ctx, parentKey)
		if err != nil || !sub.CanSee(parent.perm()) {
			return nil, fail(http.StatusBadRequest, i18n.IssueParentNotFound)
		}
		if parent.ProjectID != projectID || parent.ParentID.Valid {
			return nil, fail(http.StatusBadRequest, i18n.IssueParentInvalid)
		}
		p.parentID = parent.ID
		parentLevel = parent.LevelID
	}
	hasParent := p.parentID != nil || p.parent != nil
	if it.Subtask != hasParent {
		if it.Subtask {
			return nil, fail(http.StatusBadRequest, i18n.IssueSubtaskParentRequired)
		}
		return nil, fail(http.StatusBadRequest, i18n.IssueParentNotSubtask)
	}

	if f.has("assignee") && !f.isNull("assignee") {
		var id int64
		if err := f.get("assignee", &id); err != nil {
			return nil, err
		}
		if !sub.Can(perm.Assign, projectID, nil) && id != sub.UserID {
			return nil, fail(http.StatusForbidden, i18n.IssueAssignOthersForbidden)
		}
		if ok, err := s.isAssignable(ctx, projectID, id); err != nil {
			return nil, err
		} else if !ok {
			return nil, fail(http.StatusBadRequest, i18n.IssueAssigneeNotAssignable)
		}
		p.assignee = id
	} else {
		var components []int64
		if f.has("components") && !f.isNull("components") {
			if err := f.get("components", &components); err != nil {
				return nil, err
			}
		}
		if p.assignee, err = s.newIssueAssignee(ctx, projectID, leadID, defaultAssignee, components); err != nil {
			return nil, err
		}
	}
	custom, err := s.readCustomFields(ctx, f, projectID, typeID)
	if err != nil {
		return nil, err
	}
	p.custom = append(custom, p.custom...)
	// Custom fields take the permission to edit the issue, as its reporter.
	if len(p.custom) > 0 && !sub.Can(perm.Edit, projectID, &perm.Issue{ProjectID: projectID, ReporterID: sub.UserID}) {
		return nil, errForbidden
	}

	if p.status, err = s.initialStatusFor(ctx, projectID, typeID); err != nil {
		return nil, err
	}
	if o.status != "" && o.status != p.status {
		if !sub.Can(perm.Transition, projectID, &perm.Issue{ProjectID: projectID, ReporterID: p.reporter}) {
			return nil, errForbidden
		}
		wf, _, err := s.workflowOf(ctx, projectID, typeID)
		if err != nil {
			return nil, err
		}
		if ok, err := s.exists(ctx, `SELECT 1 FROM workflow_statuses WHERE workflow_id = ? AND status_id = ?`,
			wf, o.status); err != nil {
			return nil, err
		} else if !ok {
			return nil, failf(http.StatusBadRequest, i18n.CSVImportStatusNotInWorkflow, cat.statusOf(o.status).Name)
		}
		p.status = o.status
	}
	if o.status != "" {
		p.resolution = statusResolution(cat, sql.NullString{String: o.resolution, Valid: o.resolution != ""}, p.status)
	}

	if f.has("description") {
		var v *string
		if err := f.get("description", &v); err != nil {
			return nil, err
		}
		if v != nil {
			p.description = sqlq.NullIfBlank(*v)
		}
	}
	if f.has("environment") {
		var v *string
		if err := f.get("environment", &v); err != nil {
			return nil, err
		}
		if v != nil {
			p.environment = sqlq.NullIfBlank(*v)
		}
	}
	if f.has("priority") && !f.isNull("priority") {
		var v string
		if err := f.get("priority", &v); err != nil {
			return nil, err
		}
		if _, ok := cat.priority[v]; !ok {
			return nil, fail(http.StatusBadRequest, i18n.IssuePriorityUnknown)
		}
		p.priority = v
	} else if len(cat.Priorities) > 0 {
		// The default priority, else Jira's default: the middle one.
		p.priority = cat.Priorities[len(cat.Priorities)/2].ID
		for _, pr := range cat.Priorities {
			if pr.Default {
				p.priority = pr.ID
			}
		}
	}
	if f.has("due") && !f.isNull("due") {
		var v string
		if err := f.get("due", &v); err != nil {
			return nil, err
		}
		if _, err := time.Parse(time.DateOnly, v); err != nil {
			return nil, fail(http.StatusBadRequest, i18n.IssueDueDateInvalid)
		}
		p.due = v
	}
	estimates, err := readEstimateEdit(f)
	if err != nil {
		return nil, err
	}
	p.times = timeEstimates{}.withEstimateEdit(estimates)
	if f.has("securityLevel") && !f.isNull("securityLevel") {
		if !sub.Can(perm.Security, projectID, nil) {
			return nil, errForbidden
		}
		var id int64
		if err := f.get("securityLevel", &id); err != nil {
			return nil, err
		}
		ok, err := s.projectHasLevel(ctx, projectID, id)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fail(http.StatusBadRequest, i18n.IssueSecurityLevelUnknown)
		}
		p.level = id
	}
	// A sub-task is exactly as confidential as its parent, as in Jira.
	if hasParent {
		p.level = sqlq.Value(parentLevel)
	}

	switch {
	case o.epic != nil:
		switch {
		case hasParent:
			return nil, fail(http.StatusBadRequest, i18n.IssueSubtaskEpic)
		case isEpicType(it):
			return nil, fail(http.StatusBadRequest, i18n.IssueEpicInEpic)
		case !isEpicType(o.epic.typ):
			return nil, failf(http.StatusBadRequest, i18n.IssueNotAnEpic, o.epicKey)
		}
		p.epic = o.epic
	case f.has("epic"):
		if p.epicID, _, err = s.epicTarget(ctx, sub, cat, &issueRow{ProjectID: projectID, TypeID: typeID,
			ParentID: nullInt64(p.parentID)}, f); err != nil {
			return nil, err
		}
	}
	return p, nil
}

// insertIssue stores a planned issue in tx and sets p.id. Its parent and
// epic of the same import must be inserted first.
func (s *Server) insertIssue(ctx context.Context, tx *sql.Tx, sub *perm.Subject, p *issuePlan) error {
	var num int64
	if err := tx.QueryRowContext(ctx, `SELECT issue_counter + 1 FROM projects WHERE id = ? FOR UPDATE`, p.projectID).
		Scan(&num); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE projects SET issue_counter = ? WHERE id = ?`, num, p.projectID); err != nil {
		return err
	}
	now := time.Now().UTC()
	// A new issue goes to the bottom of the board order.
	var lastRank string
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(rank_key), '') FROM issues`).Scan(&lastRank); err != nil {
		return err
	}
	parentID, epicID := p.parentID, p.epicID
	if p.parent != nil {
		parentID = p.parent.id
	}
	if p.epic != nil {
		epicID = p.epic.id
	}
	var resolved any
	if p.resolution.Valid {
		resolved = now
	}
	p.num = num
	res, err := tx.ExecContext(ctx, `INSERT INTO issues (project_id, num, issue_type_id, parent_id, summary,
		description, environment, status_id, priority_id, resolution_id, resolved_at, reporter_id, assignee_id,
		creator_id, security_level_id, created_at, updated_at, due_date, original_estimate, remaining_estimate, rank_key)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.projectID, num, p.typeID, parentID, p.summary, p.description, p.environment, p.status, p.priority,
		sqlq.Value(p.resolution), resolved, p.reporter, p.assignee, sub.UserID, p.level, now, now, p.due,
		sqlq.Value(p.times.Original), sqlq.Value(p.times.Remaining), rankBetween(lastRank, ""))
	if err != nil {
		return err
	}
	if p.id, err = res.LastInsertId(); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO watchers (issue_id, user_id, created_at) VALUES (?, ?, ?)`,
		p.id, sub.UserID, now); err != nil {
		return err
	}
	if _, err := s.applySets(ctx, tx, p.f, &issueRow{ID: p.id, ProjectID: p.projectID}); err != nil {
		return err
	}
	if p.f.has("epic") || p.epic != nil {
		if _, err := tx.ExecContext(ctx, `UPDATE issues SET epic_id = ? WHERE id = ?`, epicID, p.id); err != nil {
			return err
		}
	}
	// As in Jira, a creation records no history, custom fields included.
	_, err = writeCustom(ctx, tx, p.id, p.custom, nil)
	return err
}

func (s *Server) findIssueByID(ctx context.Context, id int64) (*issueRow, error) {
	return scanIssue(s.db.QueryRowContext(ctx, `SELECT `+issueColumns+` FROM issues i
		JOIN projects p ON p.id = i.project_id WHERE i.id = ?`, id))
}

// exists reports whether the query returns a row.
func (s *Server) exists(ctx context.Context, query string, args ...any) (bool, error) {
	var ok bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(`+query+`)`, args...).Scan(&ok)
	return ok, err
}

func (s *Server) projectHasType(ctx context.Context, projectID int64, typeID string) (bool, error) {
	return s.exists(ctx, `SELECT 1 FROM project_issue_types WHERE project_id = ? AND issue_type_id = ?`,
		projectID, typeID)
}

func (s *Server) projectHasLevel(ctx context.Context, projectID, levelID int64) (bool, error) {
	return s.exists(ctx, `SELECT 1 FROM security_levels WHERE project_id = ? AND id = ?`, projectID, levelID)
}

// newIssueAssignee is who a new issue with these components is assigned to
// when nobody is named: the first component that decides, else the project's
// lead if that is its rule.
func (s *Server) newIssueAssignee(ctx context.Context, projectID, lead int64, defaultAssignee string, components []int64) (any, error) {
	raw, err := json.Marshal(components)
	if err != nil {
		return nil, err
	}
	if who, ok, err := s.componentAssignee(ctx, fields{"components": raw}, projectID, lead); err != nil || ok {
		return who, err
	}
	if defaultAssignee == "lead" && lead != 0 {
		return lead, nil
	}
	return nil, nil
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

// replaceSet makes the components (kind "") or the versions of a kind of an
// issue be want, and returns the history lines for what differs from have,
// removals then additions, each in id order.
func replaceSet(ctx context.Context, tx *sql.Tx, issueID int64, field, kind string, have, want []idName) ([]change, error) {
	byID := func(a, b idName) int { return cmp.Compare(a.ID, b.ID) }
	have, want = slices.Clone(have), slices.Clone(want)
	slices.SortFunc(have, byID)
	slices.SortFunc(want, byID)
	var changes []change
	has := func(list []idName, v idName) bool {
		return slices.ContainsFunc(list, func(o idName) bool { return o.ID == v.ID })
	}
	for _, v := range have {
		if has(want, v) {
			continue
		}
		var err error
		if kind == "" {
			_, err = tx.ExecContext(ctx, `DELETE FROM issue_components WHERE issue_id = ? AND component_id = ?`, issueID, v.ID)
		} else {
			_, err = tx.ExecContext(ctx, `DELETE FROM issue_versions WHERE issue_id = ? AND version_id = ? AND kind = ?`,
				issueID, v.ID, kind)
		}
		if err != nil {
			return nil, err
		}
		changes = append(changes, change{field: field, oldValue: strconv.FormatInt(v.ID, 10), oldString: v.Name})
	}
	for _, v := range want {
		if has(have, v) {
			continue
		}
		var err error
		if kind == "" {
			_, err = tx.ExecContext(ctx, `INSERT INTO issue_components (issue_id, component_id) VALUES (?, ?)`, issueID, v.ID)
		} else {
			_, err = tx.ExecContext(ctx, `INSERT INTO issue_versions (issue_id, version_id, kind) VALUES (?, ?, ?)`,
				issueID, v.ID, kind)
		}
		if err != nil {
			return nil, err
		}
		changes = append(changes, change{field: field, newValue: strconv.FormatInt(v.ID, 10), newString: v.Name})
	}
	return changes, nil
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
			if l != "" && utf8.RuneCountInString(l) <= sqlq.VarcharWidth {
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
		var want []idName
		for _, id := range ids {
			name, ok := names[id]
			if !ok {
				return nil, failf(http.StatusBadRequest, i18n.FieldValueNotInProject, st.key)
			}
			if !slices.ContainsFunc(want, func(o idName) bool { return o.ID == id }) {
				want = append(want, idName{id, name})
			}
		}
		var q string
		var args []any
		if st.kind == "" {
			q, args = `SELECT component_id FROM issue_components WHERE issue_id = ?`, []any{iss.ID}
		} else {
			q, args = `SELECT version_id FROM issue_versions WHERE issue_id = ? AND kind = ?`, []any{iss.ID, st.kind}
		}
		var have []idName
		if err := sqlq.Each(ctx, tx, q, func(rows *sql.Rows) error {
			var id int64
			err := rows.Scan(&id)
			have = append(have, idName{id, names[id]})
			return err
		}, args...); err != nil {
			return nil, err
		}
		lines, err := replaceSet(ctx, tx, iss.ID, st.field, st.kind, have, want)
		if err != nil {
			return nil, err
		}
		changes = append(changes, lines...)
	}
	return changes, nil
}

func (s *Server) userName(ctx context.Context, id int64) (string, string, error) {
	var username, name string
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(user_key, username), display_name FROM users WHERE id = ?`, id).
		Scan(&username, &name)
	return username, name, err
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
		if err := f.get("type", &typeID); err != nil {
			return err
		}
	}
	custom, err := s.readCustomFields(ctx, f, iss.ProjectID, typeID)
	if err != nil {
		return err
	}
	if pi := iss.perm(); len(custom) > 0 && !sub.Can(perm.Edit, iss.ProjectID, &pi) {
		return errForbidden
	}
	if err := s.applyEdit(ctx, sub, s.restOut(r), iss, f, custom); err != nil {
		return err
	}
	return s.respondIssue(w, r, http.StatusOK, sub, iss.ID)
}

// applyUpdate changes an issue's fields, checking each one's permission,
// recording history and announcing the change. The board reuses it to move
// cards between columns.
func (s *Server) applyUpdate(ctx context.Context, sub *perm.Subject, iss *issueRow, f fields) error {
	return s.applyEdit(ctx, sub, nil, iss, f, nil)
}

// applyEdit is applyUpdate that also writes custom field values, in the same
// transaction and history group. Callers check the permission of those.
func (s *Server) applyEdit(ctx context.Context, sub *perm.Subject, o *restOut, iss *issueRow, f fields,
	custom []customWrite) error {
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
		if v != nil {
			value := *v
			if key == "summary" {
				value = strings.TrimSpace(value)
			}
			newVal = sqlq.NullIfBlank(value)
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
	if err := text("summary", "summary", true, sqlq.VarcharWidth); err != nil {
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
		if ok {
			if ok, err = s.projectHasType(ctx, iss.ProjectID, v); err != nil {
				return err
			}
		}
		if !ok {
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
				if c.oldValue, c.oldString, err = s.userName(ctx, iss.AssigneeID.Int64); err != nil {
					return err
				}
			}
			if v != nil {
				if c.newValue, c.newString, err = s.userName(ctx, *v); err != nil {
					return err
				}
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
			ok, err := s.projectHasLevel(ctx, iss.ProjectID, *v)
			if err != nil {
				return err
			}
			if !ok {
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

	oldCustom, err := s.customBefore(ctx, sub, o, iss, custom)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// The checks above and the history's old values come from iss: a
	// concurrent edit since it was read would make both wrong.
	var lastUpdate time.Time
	if err := tx.QueryRowContext(ctx, `SELECT updated_at FROM issues WHERE id = ? FOR UPDATE`, iss.ID).
		Scan(&lastUpdate); err != nil {
		return err
	}
	if !lastUpdate.Equal(iss.Updated) {
		return fail(http.StatusConflict, i18n.IssueEditChanged)
	}
	setChanges, err := s.applySets(ctx, tx, f, iss)
	if err != nil {
		return err
	}
	changes = append(changes, setChanges...)
	customChanges, err := writeCustom(ctx, tx, iss.ID, custom, oldCustom)
	if err != nil {
		return err
	}
	if len(changes)+len(customChanges) == 0 {
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
	if err := recordHistory(ctx, tx, iss.ID, sub.UserID, now, changes, customChanges); err != nil {
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
	e.Changes = eventChanges(append(changes, customChanges...))
	if before := iss.perm(); before != updated.perm() {
		e.Before = &before
	}
	s.emit(e)
	return nil
}

func (s *Server) respondIssue(w http.ResponseWriter, r *http.Request, status int, sub *perm.Subject, id int64) error {
	iss, err := s.findIssueByID(r.Context(), id)
	if err != nil {
		return err
	}
	if !sub.CanSee(iss.perm()) {
		return writeJSON(w, status, map[string]any{"key": iss.Key(), "hidden": true})
	}
	d, err := s.issueDetail(r.Context(), sub, iss)
	if err != nil {
		return err
	}
	return writeJSON(w, status, d)
}

func (s *Server) deleteIssue(w http.ResponseWriter, r *http.Request) error {
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	iss, err := s.visibleIssue(r, sub)
	if err != nil {
		return err
	}
	if err := s.removeIssue(r, sub, iss); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// removeIssue deletes an issue the user sees, and its sub-tasks.
func (s *Server) removeIssue(r *http.Request, sub *perm.Subject, iss *issueRow) error {
	ctx := r.Context()
	var err error
	pi := iss.perm()
	if !sub.Can(perm.Delete, iss.ProjectID, &pi) {
		return errForbidden
	}
	// Deleting an issue deletes its sub-tasks: each must be deletable too.
	gone := []*issueRow{iss}
	if err := sqlq.Each(ctx, s.db, `SELECT `+issueColumns+` FROM issues i
		JOIN projects p ON p.id = i.project_id WHERE i.parent_id = ?`, func(rows *sql.Rows) error {
		child, err := scanIssue(rows)
		if err != nil {
			return err
		}
		cp := child.perm()
		if !sub.Can(perm.Delete, child.ProjectID, &cp) {
			return fail(http.StatusConflict, i18n.IssueSubtasksNotDeletable)
		}
		gone = append(gone, child)
		return nil
	}, iss.ID); err != nil {
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
		if e.Issue.Custom, err = s.eventCustom(ctx, d.ID); err != nil {
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
	s.log.Info("issue deleted", "key", iss.Key())
	s.auditLog(r, audit.IssueDelete, target("issue", iss.ID, iss.Key()), nil)
	return nil
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
