package api

// Loading what the REST API and the CSV export show of issues, for a whole
// page of issues at once: a few queries per kind of data rather than a few
// per issue. Related issues (links, sub-tasks, parent, epic), restricted
// comments and work logs follow the same visibility rules as the issue page.

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"

	"solstice/internal/perm"
	"solstice/internal/sqlq"
)

// issueData is an issue with what was loaded about it.
type issueData struct {
	row          *issueRow
	labels       []string
	components   []idName
	fixVersions  []versionData
	versions     []versionData
	links        []linkData
	subtasks     []*issueRow
	parent       *issueRow
	epic         *issueRow
	attachments  []attachmentData
	custom       map[int64][]customRaw // field id → stored values
	customLabels map[int64][]string    // labels-type custom fields
	sprints      []sprintData
	watches      counter
	votes        counter
	comments     []commentRow
	worklogs     []worklogData
}

type versionData struct {
	ID       int64
	Name     string
	Released bool
	Archived bool
	Release  sql.NullTime
}

type linkData struct {
	id, typeID int64
	outward    bool // this issue is the link's source: "this issue <outward> other"
	other      *issueRow
}

type attachmentData struct {
	id      int64
	name    string
	mime    string
	size    int64
	created time.Time
	author  sql.NullInt64
}

type customRaw struct {
	parentKey, str, text sql.NullString
	num                  sql.NullFloat64
	date                 sql.NullTime
}

type sprintData struct {
	sprintOut
	board    sql.NullInt64
	position int64
}

type worklogData struct {
	id                   int64
	author, updateAuthor sql.NullInt64
	body                 sql.NullString
	started              time.Time
	created, updated     time.Time
	seconds              int64
	group, roleName      sql.NullString
}

type customFieldDef struct {
	ID      int64
	Name    string
	TypeKey string
	Kind    string // last part of the type key: float, select, gh-sprint…
}

type optionDef struct {
	value  string
	parent sql.NullInt64
}

// issueSet is a page of issues with their data, and what rendering them needs.
type issueSet struct {
	list         []*issueData
	byID         map[int64]*issueData
	fields       []customFieldDef
	options      map[int64]optionDef
	levels       map[int64]string // security level names
	projectKinds map[int64]string
}

// Kinds of data loadIssues can fetch.
const (
	loadLabels      = "labels"
	loadComponents  = "components"
	loadVersions    = "versions"
	loadLinks       = "links"
	loadSubtasks    = "subtasks"
	loadParent      = "parent"
	loadEpic        = "epic"
	loadAttachments = "attachments"
	loadCustom      = "custom"
	loadSprints     = "sprints"
	loadWatches     = "watches"
	loadVotes       = "votes"
	loadComments    = "comments"
	loadWorklogs    = "worklogs"
)

// customFields lists the custom fields, by name.
func (s *Server) customFields(ctx context.Context) ([]customFieldDef, error) {
	var out []customFieldDef
	err := sqlq.Each(ctx, s.db, `SELECT id, name, type_key FROM custom_fields ORDER BY name, id`, func(rows *sql.Rows) error {
		var f customFieldDef
		if err := rows.Scan(&f.ID, &f.Name, &f.TypeKey); err != nil {
			return err
		}
		f.Kind = fieldKind(f.TypeKey)
		out = append(out, f)
		return nil
	})
	return out, err
}

// fieldOfKind returns the first custom field of a kind (gh-sprint…), if any.
func fieldOfKind(fields []customFieldDef, kind string) (customFieldDef, bool) {
	for _, f := range fields {
		if f.Kind == kind {
			return f, true
		}
	}
	return customFieldDef{}, false
}

// scanIn runs a query on chunks of ids: query receives the IN list to use
// ("(?, ?, …)").
func (s *Server) scanIn(ctx context.Context, ids []int64, query func(in string) string, scan func(*sql.Rows) error,
	args ...any) error {
	for len(ids) > 0 {
		chunk := ids[:min(len(ids), 500)]
		ids = ids[len(chunk):]
		all := append([]any{}, args...)
		for _, id := range chunk {
			all = append(all, id)
		}
		if err := sqlq.Each(ctx, s.db, query("("+sqlq.Marks(len(chunk))+")"), scan, all...); err != nil {
			return err
		}
	}
	return nil
}

// issuesByID loads issues by id (unordered).
func (s *Server) issuesByID(ctx context.Context, ids []int64) (map[int64]*issueRow, error) {
	out := map[int64]*issueRow{}
	var want []int64
	for _, id := range ids {
		if _, ok := out[id]; !ok && id != 0 {
			out[id] = nil
			want = append(want, id)
		}
	}
	err := s.scanIn(ctx, want, func(in string) string {
		return `SELECT ` + issueColumns + ` FROM issues i JOIN projects p ON p.id = i.project_id WHERE i.id IN ` + in
	}, func(rows *sql.Rows) error {
		iss, err := scanIssue(rows)
		if err != nil {
			return err
		}
		out[iss.ID] = iss
		return nil
	})
	for id, iss := range out {
		if iss == nil {
			delete(out, id)
		}
	}
	return out, err
}

// loadIssues loads, for issues the user sees, the kinds of data want asks for.
func (s *Server) loadIssues(ctx context.Context, sub *perm.Subject, rows []*issueRow, want func(string) bool) (*issueSet, error) {
	set := &issueSet{byID: map[int64]*issueData{}, options: map[int64]optionDef{}, levels: map[int64]string{},
		projectKinds: map[int64]string{}}
	var ids []int64
	for _, r := range rows {
		d := &issueData{row: r, labels: []string{}, custom: map[int64][]customRaw{}, customLabels: map[int64][]string{}}
		set.list = append(set.list, d)
		set.byID[r.ID] = d
		ids = append(ids, r.ID)
	}
	var err error
	if set.fields, err = s.customFields(ctx); err != nil {
		return nil, err
	}
	if err := sqlq.Each(ctx, s.db, `SELECT id, kind FROM projects`, func(rows *sql.Rows) error {
		var id int64
		var kind string
		if err := rows.Scan(&id, &kind); err != nil {
			return err
		}
		set.projectKinds[id] = kind
		return nil
	}); err != nil {
		return nil, err
	}
	if err := sqlq.Each(ctx, s.db, `SELECT id, name FROM security_levels`, func(rows *sql.Rows) error {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return err
		}
		set.levels[id] = name
		return nil
	}); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return set, nil
	}
	for _, step := range []struct {
		kind string
		load func(context.Context, *perm.Subject, *issueSet, []int64) error
	}{
		{loadLabels, s.loadLabels}, {loadComponents, s.loadComponents}, {loadVersions, s.loadVersions},
		{loadLinks, s.loadLinks}, {loadSubtasks, s.loadSubtasks}, {loadParent, s.loadParents},
		{loadEpic, s.loadEpics}, {loadAttachments, s.loadAttachments}, {loadCustom, s.loadCustom},
		{loadSprints, s.loadSprints}, {loadWatches, s.loadCounter("watchers")}, {loadVotes, s.loadCounter("votes")},
		{loadComments, s.loadComments}, {loadWorklogs, s.loadWorklogs},
	} {
		if want(step.kind) {
			if err := step.load(ctx, sub, set, ids); err != nil {
				return nil, err
			}
		}
	}
	return set, nil
}

func (s *Server) loadLabels(ctx context.Context, _ *perm.Subject, set *issueSet, ids []int64) error {
	return s.scanIn(ctx, ids, func(in string) string {
		return `SELECT issue_id, label FROM labels WHERE field_id IS NULL AND issue_id IN ` + in + ` ORDER BY label`
	}, func(rows *sql.Rows) error {
		var id int64
		var label string
		if err := rows.Scan(&id, &label); err != nil {
			return err
		}
		set.byID[id].labels = append(set.byID[id].labels, label)
		return nil
	})
}

func (s *Server) loadComponents(ctx context.Context, _ *perm.Subject, set *issueSet, ids []int64) error {
	return s.scanIn(ctx, ids, func(in string) string {
		return `SELECT ic.issue_id, c.id, c.name FROM issue_components ic JOIN components c ON c.id = ic.component_id
			WHERE ic.issue_id IN ` + in + ` ORDER BY c.name`
	}, func(rows *sql.Rows) error {
		var id int64
		var c idName
		if err := rows.Scan(&id, &c.ID, &c.Name); err != nil {
			return err
		}
		set.byID[id].components = append(set.byID[id].components, c)
		return nil
	})
}

func (s *Server) loadVersions(ctx context.Context, _ *perm.Subject, set *issueSet, ids []int64) error {
	return s.scanIn(ctx, ids, func(in string) string {
		return `SELECT iv.issue_id, iv.kind, v.id, v.name, v.released, v.archived, v.release_date
			FROM issue_versions iv JOIN versions v ON v.id = iv.version_id
			WHERE iv.issue_id IN ` + in + ` ORDER BY v.position, v.name`
	}, func(rows *sql.Rows) error {
		var id int64
		var kind string
		var v versionData
		if err := rows.Scan(&id, &kind, &v.ID, &v.Name, &v.Released, &v.Archived, &v.Release); err != nil {
			return err
		}
		d := set.byID[id]
		if kind == "fix" {
			d.fixVersions = append(d.fixVersions, v)
		} else {
			d.versions = append(d.versions, v)
		}
		return nil
	})
}

// loadLinks loads links in both directions, keeping those to issues the
// user sees.
func (s *Server) loadLinks(ctx context.Context, sub *perm.Subject, set *issueSet, ids []int64) error {
	type raw struct {
		issue int64
		link  linkData
		other int64
	}
	var list []raw
	var others []int64
	for _, outward := range []bool{true, false} {
		this, other := "source_id", "target_id"
		if !outward {
			this, other = other, this
		}
		if err := s.scanIn(ctx, ids, func(in string) string {
			return `SELECT id, type_id, ` + this + `, ` + other + ` FROM issue_links WHERE ` + this + ` IN ` + in +
				` ORDER BY id`
		}, func(rows *sql.Rows) error {
			var l raw
			if err := rows.Scan(&l.link.id, &l.link.typeID, &l.issue, &l.other); err != nil {
				return err
			}
			l.link.outward = outward
			list = append(list, l)
			others = append(others, l.other)
			return nil
		}); err != nil {
			return err
		}
	}
	rows, err := s.issuesByID(ctx, others)
	if err != nil {
		return err
	}
	for _, l := range list {
		other, ok := rows[l.other]
		if !ok || !sub.CanSee(other.perm()) {
			continue
		}
		l.link.other = other
		d := set.byID[l.issue]
		d.links = append(d.links, l.link)
	}
	return nil
}

func (s *Server) loadSubtasks(ctx context.Context, sub *perm.Subject, set *issueSet, ids []int64) error {
	return s.scanIn(ctx, ids, func(in string) string {
		return `SELECT ` + issueColumns + ` FROM issues i JOIN projects p ON p.id = i.project_id
			WHERE i.parent_id IN ` + in + ` ORDER BY i.num`
	}, func(rows *sql.Rows) error {
		iss, err := scanIssue(rows)
		if err != nil {
			return err
		}
		if sub.CanSee(iss.perm()) {
			d := set.byID[iss.ParentID.Int64]
			d.subtasks = append(d.subtasks, iss)
		}
		return nil
	})
}

func (s *Server) loadParents(ctx context.Context, sub *perm.Subject, set *issueSet, _ []int64) error {
	var ids []int64
	for _, d := range set.list {
		ids = append(ids, d.row.ParentID.Int64)
	}
	rows, err := s.issuesByID(ctx, ids)
	if err != nil {
		return err
	}
	for _, d := range set.list {
		if p, ok := rows[d.row.ParentID.Int64]; ok && d.row.ParentID.Valid && sub.CanSee(p.perm()) {
			d.parent = p
		}
	}
	return nil
}

func (s *Server) loadEpics(ctx context.Context, sub *perm.Subject, set *issueSet, _ []int64) error {
	var ids []int64
	for _, d := range set.list {
		ids = append(ids, d.row.EpicID.Int64)
	}
	rows, err := s.issuesByID(ctx, ids)
	if err != nil {
		return err
	}
	for _, d := range set.list {
		if e, ok := rows[d.row.EpicID.Int64]; ok && d.row.EpicID.Valid && sub.CanSee(e.perm()) {
			d.epic = e
		}
	}
	return nil
}

func (s *Server) loadAttachments(ctx context.Context, _ *perm.Subject, set *issueSet, ids []int64) error {
	return s.scanIn(ctx, ids, func(in string) string {
		return `SELECT issue_id, id, filename, mime_type, size, created_at, author_id FROM attachments
			WHERE issue_id IN ` + in + ` ORDER BY created_at, id`
	}, func(rows *sql.Rows) error {
		var id int64
		var a attachmentData
		if err := rows.Scan(&id, &a.id, &a.name, &a.mime, &a.size, &a.created, &a.author); err != nil {
			return err
		}
		set.byID[id].attachments = append(set.byID[id].attachments, a)
		return nil
	})
}

// loadCustom loads custom field values, the options they name, and the
// labels of labels-type fields.
func (s *Server) loadCustom(ctx context.Context, _ *perm.Subject, set *issueSet, ids []int64) error {
	optionIDs := map[int64]bool{}
	if err := s.scanIn(ctx, ids, func(in string) string {
		return `SELECT issue_id, field_id, parent_key, string_value, number_value, text_value, date_value
			FROM custom_field_values WHERE issue_id IN ` + in + ` ORDER BY parent_key IS NOT NULL, id`
	}, func(rows *sql.Rows) error {
		var id, field int64
		var v customRaw
		if err := rows.Scan(&id, &field, &v.parentKey, &v.str, &v.num, &v.text, &v.date); err != nil {
			return err
		}
		set.byID[id].custom[field] = append(set.byID[id].custom[field], v)
		if n, err := strconv.ParseInt(v.str.String, 10, 64); err == nil {
			optionIDs[n] = true
		}
		return nil
	}); err != nil {
		return err
	}
	var opts []int64
	for id := range optionIDs {
		opts = append(opts, id)
	}
	if err := s.scanIn(ctx, opts, func(in string) string {
		return `SELECT id, value, parent_id FROM custom_field_options WHERE id IN ` + in
	}, func(rows *sql.Rows) error {
		var id int64
		var o optionDef
		if err := rows.Scan(&id, &o.value, &o.parent); err != nil {
			return err
		}
		set.options[id] = o
		return nil
	}); err != nil {
		return err
	}
	return s.scanIn(ctx, ids, func(in string) string {
		return `SELECT issue_id, field_id, label FROM labels WHERE field_id IS NOT NULL AND issue_id IN ` + in +
			` ORDER BY label`
	}, func(rows *sql.Rows) error {
		var id, field int64
		var label string
		if err := rows.Scan(&id, &field, &label); err != nil {
			return err
		}
		d := set.byID[id]
		d.customLabels[field] = append(d.customLabels[field], label)
		return nil
	})
}

func (s *Server) loadSprints(ctx context.Context, _ *perm.Subject, set *issueSet, ids []int64) error {
	return s.scanIn(ctx, ids, func(in string) string {
		return `SELECT si.issue_id, s.id, s.name, s.state, s.goal, s.start_at, s.end_at, s.completed_at, s.board_id,
			s.position FROM sprint_issues si JOIN sprints s ON s.id = si.sprint_id
			WHERE si.issue_id IN ` + in + ` ORDER BY s.position, s.id`
	}, func(rows *sql.Rows) error {
		var id int64
		var sp sprintData
		var goal sql.NullString
		var start, end, done sql.NullTime
		if err := rows.Scan(&id, &sp.ID, &sp.Name, &sp.State, &goal, &start, &end, &done, &sp.board,
			&sp.position); err != nil {
			return err
		}
		sp.Goal = nullStr(goal)
		sp.Start, sp.End, sp.Completed = nullTime(start), nullTime(end), nullTime(done)
		set.byID[id].sprints = append(set.byID[id].sprints, sp)
		return nil
	})
}

func nullTime(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}

// loadCounter counts watchers or votes, and whether the user is among them.
func (s *Server) loadCounter(table string) func(context.Context, *perm.Subject, *issueSet, []int64) error {
	return func(ctx context.Context, sub *perm.Subject, set *issueSet, ids []int64) error {
		return s.scanIn(ctx, ids, func(in string) string {
			return `SELECT issue_id, COUNT(*), COALESCE(SUM(user_id = ?), 0) FROM ` + table + `
				WHERE issue_id IN ` + in + ` GROUP BY issue_id`
		}, func(rows *sql.Rows) error {
			var id int64
			var c counter
			if err := rows.Scan(&id, &c.Count, &c.Me); err != nil {
				return err
			}
			if table == "watchers" {
				set.byID[id].watches = c
			} else {
				set.byID[id].votes = c
			}
			return nil
		}, sub.UserID)
	}
}

// loadComments loads the comments each issue shows the user (one query per
// issue: comments are only loaded when asked for).
func (s *Server) loadComments(ctx context.Context, sub *perm.Subject, set *issueSet, _ []int64) error {
	for _, d := range set.list {
		rows, err := s.visibleComments(ctx, sub, d.row)
		if err != nil {
			return err
		}
		d.comments = rows
	}
	return nil
}

// loadWorklogs loads the work logs the user may read.
func (s *Server) loadWorklogs(ctx context.Context, sub *perm.Subject, set *issueSet, ids []int64) error {
	return s.scanIn(ctx, ids, func(in string) string {
		return `SELECT w.issue_id, w.id, w.author_id, w.updated_by_id, w.body, w.started_at, w.created_at, w.updated_at,
			w.seconds, w.visibility_group, w.visibility_role_id, pr.name
			FROM worklogs w LEFT JOIN project_roles pr ON pr.id = w.visibility_role_id
			WHERE w.issue_id IN ` + in + ` ORDER BY w.started_at, w.id`
	}, func(rows *sql.Rows) error {
		var id int64
		var wl worklogData
		var role sql.NullInt64
		if err := rows.Scan(&id, &wl.id, &wl.author, &wl.updateAuthor, &wl.body, &wl.started, &wl.created,
			&wl.updated, &wl.seconds, &wl.group, &role, &wl.roleName); err != nil {
			return err
		}
		d := set.byID[id]
		if sub.SeesRestricted(d.row.ProjectID, wl.group.String, role.Int64) {
			d.worklogs = append(d.worklogs, wl)
		}
		return nil
	})
}

// userIDs lists the users the loaded data mentions.
func (set *issueSet) userIDs() []int64 {
	var ids []int64
	users := map[string]bool{"userpicker": true, "multiuserpicker": true}
	kinds := map[int64]string{}
	for _, f := range set.fields {
		kinds[f.ID] = f.Kind
	}
	for _, d := range set.list {
		ids = append(ids, d.row.ReporterID.Int64, d.row.AssigneeID.Int64, d.row.CreatorID.Int64)
		for _, a := range d.attachments {
			ids = append(ids, a.author.Int64)
		}
		for _, c := range d.comments {
			ids = append(ids, c.author.Int64, c.updatedBy.Int64)
		}
		for _, w := range d.worklogs {
			ids = append(ids, w.author.Int64, w.updateAuthor.Int64)
		}
		for field, values := range d.custom {
			if users[kinds[field]] {
				for _, v := range values {
					if id, err := strconv.ParseInt(strings.TrimSpace(v.str.String), 10, 64); err == nil {
						ids = append(ids, id)
					}
				}
			}
		}
	}
	return ids
}
