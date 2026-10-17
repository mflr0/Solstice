package api

// Creating and configuring projects in Solstice (until now they only came
// from a Jira import), archiving and deleting them, and their categories.

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"solstice/internal/audit"
	"solstice/internal/i18n"
	"solstice/internal/issue"
	"solstice/internal/perm"
	"solstice/internal/sqlq"
)

// projectKeyPattern is Jira's default rule: an upper-case letter, then
// upper-case letters or digits, 2 to 10 characters in all.
var projectKeyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,9}$`)

// checkProjectKey tells whether a key may be given to a new project: well
// formed, and used by no project, now or formerly.
func (s *Server) checkProjectKey(ctx context.Context, key string) error {
	if !projectKeyPattern.MatchString(key) {
		return fail(http.StatusBadRequest, i18n.ProjectKeyInvalid)
	}
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM projects WHERE pkey = ?)
		+ (SELECT COUNT(*) FROM project_keys WHERE pkey = ?)`, key, key).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return failf(http.StatusConflict, i18n.ProjectKeyTaken, key)
	}
	return nil
}

// projectKeyCheck answers the creation form as the key is typed.
func (s *Server) projectKeyCheck(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	key := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("key")))
	out := map[string]any{"key": key, "available": true}
	if err := s.checkProjectKey(r.Context(), key); err != nil {
		var he *httpError
		if !errors.As(err, &he) {
			return err
		}
		out["available"], out["reason"] = false, he.msg.In(locale(r))
	}
	return writeJSON(w, http.StatusOK, out)
}

// projectTemplates are the workflows a new project can start with: its
// statuses in order. Any status can be reached from any other.
var projectTemplates = map[string]struct {
	kind     string
	statuses []templateStatus
}{
	"software": {"software", []templateStatus{{"To Do", issue.Todo}, {"In Progress", issue.Doing},
		{"In Review", issue.Doing}, {"Done", issue.Done}}},
	"simple": {"business", []templateStatus{{"To do", issue.Todo}, {"In progress", issue.Doing},
		{"Done", issue.Done}}},
}

type templateStatus struct {
	name     string
	category issue.StatusCategory
}

// memberPermissions are what the people given access to a new project may
// do; the lead may do everything.
var memberPermissions = []string{perm.Browse, perm.Create, perm.Edit, perm.Comment, perm.Transition, perm.Assign,
	perm.Assignable, perm.Attach, perm.Link, perm.Worklog}

type accessInput struct {
	Holder string `json:"holder"` // everyone, group, role
	ID     int64  `json:"id"`
}

// activeUser checks that an id names an active account.
func (s *Server) activeUser(ctx context.Context, id int64) error {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE id = ? AND active`, id).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return fail(http.StatusBadRequest, i18n.ProjectLeadInactive)
	}
	return nil
}

func (s *Server) checkCategory(ctx context.Context, id *int64) error {
	if id == nil {
		return nil
	}
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM project_categories WHERE id = ?`, *id).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return fail(http.StatusBadRequest, i18n.ProjectCategoryNotFound)
	}
	return nil
}

// projectNameTaken tells whether another project has this name (any case).
func (s *Server) projectNameTaken(ctx context.Context, name string, except int64) error {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects WHERE LOWER(name) = LOWER(?) AND id <> ?`,
		name, except).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return failf(http.StatusConflict, i18n.ProjectNameTaken, name)
	}
	return nil
}

// checkIssueTypes validates the issue types offered in a project: known,
// and at least one that is not a sub-task.
func checkIssueTypes(cat *catalog, ids []string) ([]string, error) {
	var out []string
	standard := false
	for _, id := range ids {
		t, ok := cat.issueType[id]
		if !ok {
			return nil, failf(http.StatusBadRequest, i18n.IssueTypeUnknown, id)
		}
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
		standard = standard || !t.Subtask
	}
	if !standard {
		return nil, fail(http.StatusBadRequest, i18n.ProjectIssueTypeRequired)
	}
	return out, nil
}

func (s *Server) createProject(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	var in struct {
		Key             string       `json:"key"`
		Name            string       `json:"name"`
		Description     *string      `json:"description"`
		Lead            *int64       `json:"lead"`
		Category        *int64       `json:"category"`
		Template        string       `json:"template"`
		Access          *accessInput `json:"access"`
		IssueTypes      []string     `json:"issueTypes"`
		DefaultAssignee *string      `json:"defaultAssignee"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	ctx := r.Context()
	me := currentUser(r)
	key := strings.ToUpper(strings.TrimSpace(in.Key))
	if err := s.checkProjectKey(ctx, key); err != nil {
		return err
	}
	name, err := cleanName(in.Name, 255)
	if err != nil {
		return err
	}
	if err := s.projectNameTaken(ctx, name, 0); err != nil {
		return err
	}
	lead := me.ID
	if in.Lead != nil {
		lead = *in.Lead
	}
	if err := s.activeUser(ctx, lead); err != nil {
		return err
	}
	if err := s.checkCategory(ctx, in.Category); err != nil {
		return err
	}
	if in.Template == "" {
		in.Template = "software"
	}
	tmpl, ok := projectTemplates[in.Template]
	if !ok {
		return fail(http.StatusBadRequest, i18n.ProjectTemplateUnknown)
	}
	assignTo := "unassigned"
	if in.DefaultAssignee != nil {
		if *in.DefaultAssignee != "unassigned" && *in.DefaultAssignee != "lead" {
			return fail(http.StatusBadRequest, i18n.ProjectAssigneeTypeUnknown)
		}
		assignTo = *in.DefaultAssignee
	}
	if a := in.Access; a != nil {
		switch a.Holder {
		case perm.Everyone:
		case perm.Group:
			if err := s.checkGroups(ctx, []int64{a.ID}); err != nil {
				return err
			}
		case perm.Role:
			if _, err := s.roleByID(ctx, a.ID); err != nil {
				return fail(http.StatusBadRequest, i18n.RoleNotFound)
			}
		default:
			return failf(http.StatusBadRequest, i18n.ProjectGranteeUnknown, a.Holder)
		}
	}
	if err := s.seedCatalogues(ctx); err != nil {
		return err
	}
	cat, err := s.catalogue(ctx)
	if err != nil {
		return err
	}
	types := in.IssueTypes
	if types == nil {
		for _, t := range cat.IssueTypes {
			types = append(types, t.ID)
		}
	}
	if types, err = checkIssueTypes(cat, types); err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var statuses []string
	for _, st := range tmpl.statuses {
		id, err := ensureStatus(ctx, tx, st.name, st.category)
		if err != nil {
			return err
		}
		statuses = append(statuses, id)
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO projects (pkey, name, description, lead_id, category_id, kind,
		default_assignee) VALUES (?, ?, ?, ?, ?, ?, ?)`, key, name, optDescription(in.Description), lead, in.Category,
		tmpl.kind, assignTo)
	if isDuplicate(err) {
		return failf(http.StatusConflict, i18n.ProjectKeyTaken, key)
	}
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	for i, t := range types {
		if _, err := tx.ExecContext(ctx, `INSERT INTO project_issue_types (project_id, issue_type_id, position)
			VALUES (?, ?, ?)`, id, t, i); err != nil {
			return err
		}
	}
	res, err = tx.ExecContext(ctx, `INSERT INTO workflows (project_id, name, is_default, initial_status_id)
		VALUES (?, ?, TRUE, ?)`, id, "Workflow "+key, statuses[0])
	if err != nil {
		return err
	}
	wf, _ := res.LastInsertId()
	for i, st := range statuses {
		if _, err := tx.ExecContext(ctx, `INSERT INTO workflow_statuses (workflow_id, status_id, position) VALUES (?, ?, ?)`,
			wf, st, i); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO workflow_transitions (workflow_id, name, from_status_id, to_status_id,
			position) VALUES (?, ?, NULL, ?, ?)`, wf, tmpl.statuses[i].name, st, i); err != nil {
			return err
		}
	}
	// Grants, stored as the import stores them: the lead may do everything,
	// the people given access work on issues.
	grant := func(p, holder string, user, group, role any) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO project_permissions (project_id, permission, holder, user_id,
			group_id, role_id) VALUES (?, ?, ?, ?, ?, ?)`, id, p, holder, user, group, role)
		return err
	}
	for _, p := range perm.All {
		if err := grant(p, perm.Lead, nil, nil, nil); err != nil {
			return err
		}
	}
	if a := in.Access; a != nil {
		var group, role any
		switch a.Holder {
		case perm.Group:
			group = a.ID
		case perm.Role:
			role = a.ID
			if _, err := tx.ExecContext(ctx, `INSERT INTO project_role_members (project_id, role_id, user_id)
				VALUES (?, ?, ?)`, id, a.ID, lead); err != nil {
				return err
			}
		}
		for _, p := range memberPermissions {
			if err := grant(p, a.Holder, nil, group, role); err != nil {
				return err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.perms.Invalidate()
	s.forgetCatalogue()
	details := map[string]any{"name": name, "template": in.Template, "lead": s.userNames(ctx, []int64{lead}),
		"issueTypes": types}
	if a := in.Access; a != nil {
		details["access"] = map[string]any{"holder": a.Holder, "id": a.ID}
	}
	s.auditLog(r, audit.ProjectCreate, audit.Target{Type: "project", ID: strconv.FormatInt(id, 10), Label: key}, details)
	return writeJSON(w, http.StatusCreated, map[string]any{"id": id, "key": key})
}

// ensureStatus returns the status of that name (any case), created in the
// category given if there is none.
func ensureStatus(ctx context.Context, tx *sql.Tx, name string, category issue.StatusCategory) (string, error) {
	var id string
	err := tx.QueryRowContext(ctx, `SELECT id FROM statuses WHERE LOWER(name) = LOWER(?) ORDER BY position, id LIMIT 1`,
		name).Scan(&id)
	if err == nil || !errors.Is(err, sql.ErrNoRows) {
		return id, err
	}
	next, err := nextCatalogID(ctx, tx, "statuses")
	if err != nil {
		return "", err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO statuses (id, name, category, position)
		VALUES (?, ?, ?, (SELECT p FROM (SELECT COALESCE(MAX(position), 0) + 1 AS p FROM statuses) x))`,
		next, name, category)
	return next, err
}

// nextCatalogID gives the next identifier of a catalogue with text ids:
// numeric, continuing Jira's sequence for custom items.
func nextCatalogID(ctx context.Context, tx *sql.Tx, table string) (string, error) {
	var next int64
	err := tx.QueryRowContext(ctx, `SELECT GREATEST(COALESCE(MAX(CAST(id AS UNSIGNED)), 0) + 1, 10000) FROM `+table+`
		WHERE id REGEXP '^[0-9]+$'`).Scan(&next)
	return strconv.FormatInt(next, 10), err
}

// seedCatalogues fills the empty catalogues of a fresh instance (no
// project yet) with defaults, in the instance's language, so that the first
// project can be used at once.
func (s *Server) seedCatalogues(ctx context.Context) error {
	var projects int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects`).Scan(&projects); err != nil || projects > 0 {
		return err
	}
	lang := s.defaultLanguage()
	t := func(m i18n.Message) string { return m.In(lang) }
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	empty := func(table string) (bool, error) {
		var n int
		err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table).Scan(&n)
		return n == 0, err
	}
	insert := func(table, cols string, rows [][]any) error {
		if ok, err := empty(table); err != nil || !ok {
			return err
		}
		for i, row := range rows {
			args := row
			if table != "link_types" && table != "project_roles" {
				id, err := nextCatalogID(ctx, tx, table)
				if err != nil {
					return err
				}
				args = append([]any{id}, row...)
			}
			args = append(args, i+1)
			if _, err := tx.ExecContext(ctx, `INSERT INTO `+table+` (`+cols+`) VALUES (`+sqlq.Marks(len(args))+`)`,
				args...); err != nil {
				return err
			}
		}
		return nil
	}
	steps := []struct {
		table, cols string
		rows        [][]any
	}{
		{"issue_types", "id, name, subtask, position", [][]any{
			{t(i18n.M(i18n.DefaultTypeTask)), false}, {t(i18n.M(i18n.DefaultTypeBug)), false}, {t(i18n.M(i18n.DefaultTypeStory)), false},
			{t(i18n.M(i18n.DefaultTypeEpic)), false}, {t(i18n.M(i18n.DefaultTypeSubtask)), true}}},
		{"priorities", "id, name, color, is_default, position", [][]any{
			{t(i18n.M(i18n.DefaultPriorityVeryHigh)), "#c9372c", false}, {t(i18n.M(i18n.DefaultPriorityHigh)), "#e2483d", false},
			{t(i18n.M(i18n.DefaultPriorityMedium)), "#e98d23", true}, {t(i18n.M(i18n.DefaultPriorityLow)), "#2a7ab0", false},
			{t(i18n.M(i18n.DefaultPriorityVeryLow)), "#6b778c", false}}},
		{"resolutions", "id, name, is_default, position", [][]any{
			{t(i18n.M(i18n.DefaultResolutionDone)), true}, {t(i18n.M(i18n.DefaultResolutionWontDo)), false}, {t(i18n.M(i18n.DefaultResolutionDuplicate)), false},
			{t(i18n.M(i18n.DefaultResolutionCannotReproduce)), false}}},
		{"link_types", "name, inward, outward, position", [][]any{
			{t(i18n.M(i18n.DefaultLinkBlocksName)), t(i18n.M(i18n.DefaultLinkBlocksInward)), t(i18n.M(i18n.DefaultLinkBlocksOutward))},
			{t(i18n.M(i18n.DefaultLinkRelatesName)), t(i18n.M(i18n.DefaultLinkRelatesDirection)), t(i18n.M(i18n.DefaultLinkRelatesDirection))},
			{t(i18n.M(i18n.DefaultLinkDuplicatesName)), t(i18n.M(i18n.DefaultLinkDuplicatesInward)), t(i18n.M(i18n.DefaultLinkDuplicatesOutward))},
			{t(i18n.M(i18n.DefaultLinkClonesName)), t(i18n.M(i18n.DefaultLinkClonesInward)), t(i18n.M(i18n.DefaultLinkClonesOutward))}}},
	}
	for _, st := range steps {
		if err := insert(st.table, st.cols, st.rows); err != nil {
			return err
		}
	}
	if ok, err := empty("project_roles"); err != nil {
		return err
	} else if ok {
		for _, name := range []string{t(i18n.M(i18n.DefaultRoleAdministrators)), t(i18n.M(i18n.DefaultRoleDevelopers)), t(i18n.M(i18n.DefaultRoleUsers))} {
			if _, err := tx.ExecContext(ctx, `INSERT INTO project_roles (name) VALUES (?)`, name); err != nil {
				return err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.forgetCatalogue()
	return nil
}

// Settings of one project ------------------------------------------------------------

type projectSettingsOut struct {
	ID              int64     `json:"id"`
	Key             string    `json:"key"`
	Name            string    `json:"name"`
	Description     *string   `json:"description"`
	Kind            string    `json:"kind"`
	Lead            *userRef  `json:"lead"`
	Category        *idName   `json:"category"`
	DefaultAssignee string    `json:"defaultAssignee"`
	Archived        bool      `json:"archived"`
	IssueTypes      []typeRef `json:"issueTypes"`
	Issues          int       `json:"issues"`
}

func (s *Server) projectSettings(ctx context.Context, id int64) (*projectSettingsOut, error) {
	p := &projectSettingsOut{IssueTypes: []typeRef{}}
	var desc, category sql.NullString
	var lead, categoryID sql.NullInt64
	err := s.db.QueryRowContext(ctx, `SELECT p.id, p.pkey, p.name, p.description, p.kind, p.lead_id, c.id, c.name,
		p.default_assignee, p.archived, (SELECT COUNT(*) FROM issues i WHERE i.project_id = p.id)
		FROM projects p LEFT JOIN project_categories c ON c.id = p.category_id WHERE p.id = ?`, id).
		Scan(&p.ID, &p.Key, &p.Name, &desc, &p.Kind, &lead, &categoryID, &category, &p.DefaultAssignee, &p.Archived,
			&p.Issues)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}
	p.Description = sqlq.String(desc)
	if categoryID.Valid {
		p.Category = &idName{categoryID.Int64, category.String}
	}
	users, err := s.users(ctx, lead.Int64)
	if err != nil {
		return nil, err
	}
	p.Lead = refOf(users, lead)
	cat, err := s.catalogue(ctx)
	if err != nil {
		return nil, err
	}
	err = sqlq.Each(ctx, s.db, `SELECT issue_type_id FROM project_issue_types WHERE project_id = ? ORDER BY position`,
		func(rows *sql.Rows) error {
			var t string
			if err := rows.Scan(&t); err != nil {
				return err
			}
			p.IssueTypes = append(p.IssueTypes, cat.typeOf(t))
			return nil
		}, id)
	return p, err
}

func (s *Server) getProjectSettings(w http.ResponseWriter, r *http.Request) error {
	id, err := s.configurableProject(r)
	if err != nil {
		return err
	}
	p, err := s.projectSettings(r.Context(), id)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, p)
}

// updateProject changes a project's settings: project administrators.
func (s *Server) updateProject(w http.ResponseWriter, r *http.Request) error {
	id, err := s.configurableProject(r)
	if err != nil {
		return err
	}
	var in struct {
		Name            *string  `json:"name"`
		Description     *string  `json:"description"`
		Lead            *int64   `json:"lead"`
		Category        *int64   `json:"category"`
		DefaultAssignee *string  `json:"defaultAssignee"`
		IssueTypes      []string `json:"issueTypes"`
	}
	raw, err := readJSONFields(r, &in) // null clears description and category
	if err != nil {
		return err
	}
	ctx := r.Context()
	before, err := s.projectSettings(ctx, id)
	if err != nil {
		return err
	}
	sets := map[string]any{}
	if in.Name != nil {
		name, err := cleanName(*in.Name, 255)
		if err != nil {
			return err
		}
		if err := s.projectNameTaken(ctx, name, id); err != nil {
			return err
		}
		sets["name"] = name
	}
	if _, ok := raw["description"]; ok {
		sets["description"] = optDescription(in.Description)
	}
	if in.Lead != nil {
		if err := s.activeUser(ctx, *in.Lead); err != nil {
			return err
		}
		sets["lead_id"] = *in.Lead
	}
	if _, ok := raw["category"]; ok {
		if err := s.checkCategory(ctx, in.Category); err != nil {
			return err
		}
		sets["category_id"] = in.Category
	}
	if in.DefaultAssignee != nil {
		if *in.DefaultAssignee != "unassigned" && *in.DefaultAssignee != "lead" {
			return fail(http.StatusBadRequest, i18n.ProjectAssigneeTypeUnknown)
		}
		sets["default_assignee"] = *in.DefaultAssignee
	}
	var types []string
	if in.IssueTypes != nil {
		cat, err := s.catalogue(ctx)
		if err != nil {
			return err
		}
		if types, err = checkIssueTypes(cat, in.IssueTypes); err != nil {
			return err
		}
		inUse := []map[string]any{}
		err = sqlq.Each(ctx, s.db, `SELECT issue_type_id, COUNT(*) FROM issues WHERE project_id = ? GROUP BY issue_type_id`,
			func(rows *sql.Rows) error {
				var t string
				var n int
				if err := rows.Scan(&t, &n); err != nil {
					return err
				}
				if !slices.Contains(types, t) {
					inUse = append(inUse, map[string]any{"type": cat.typeOf(t), "issues": n})
				}
				return nil
			}, id)
		if err != nil {
			return err
		}
		if len(inUse) > 0 {
			return &httpError{status: http.StatusConflict, extra: map[string]any{"inUse": inUse},
				msg: i18n.M(i18n.ProjectRemovedTypesInUse)}
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for column, v := range sets {
		if _, err := tx.ExecContext(ctx, `UPDATE projects SET `+column+` = ? WHERE id = ?`, v, id); err != nil {
			return err
		}
	}
	if types != nil {
		if _, err := tx.ExecContext(ctx, `DELETE FROM project_issue_types WHERE project_id = ?`, id); err != nil {
			return err
		}
		for i, t := range types {
			if _, err := tx.ExecContext(ctx, `INSERT INTO project_issue_types (project_id, issue_type_id, position)
				VALUES (?, ?, ?)`, id, t, i); err != nil {
				return err
			}
		}
		// Types that left keep no workflow of their own.
		if _, err := tx.ExecContext(ctx, `DELETE FROM workflow_issue_types WHERE project_id = ?
			AND issue_type_id NOT IN (SELECT issue_type_id FROM project_issue_types WHERE project_id = ?)`, id, id); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.perms.Invalidate()
	after, err := s.projectSettings(ctx, id)
	if err != nil {
		return err
	}
	if changes := projectChanges(before, after); len(changes) > 0 {
		s.auditLog(r, audit.ProjectUpdate, audit.Target{Type: "project", ID: strconv.FormatInt(id, 10), Label: after.Key},
			changes)
	}
	return writeJSON(w, http.StatusOK, after)
}

// projectChanges lists what a change to a project's settings touched.
func projectChanges(a, b *projectSettingsOut) map[string]any {
	out := map[string]any{}
	pair := func(name string, from, to any) {
		if from != to {
			out[name] = map[string]any{"from": from, "to": to}
		}
	}
	str := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	who := func(u *userRef) string {
		if u == nil {
			return ""
		}
		return u.Username
	}
	cat := func(c *idName) string {
		if c == nil {
			return ""
		}
		return c.Name
	}
	types := func(list []typeRef) string {
		var ids []string
		for _, t := range list {
			ids = append(ids, t.ID)
		}
		return strings.Join(ids, ",")
	}
	pair("name", a.Name, b.Name)
	pair("description", str(a.Description), str(b.Description))
	pair("lead", who(a.Lead), who(b.Lead))
	pair("category", cat(a.Category), cat(b.Category))
	pair("defaultAssignee", a.DefaultAssignee, b.DefaultAssignee)
	pair("issueTypes", types(a.IssueTypes), types(b.IssueTypes))
	return out
}

// adminProject loads a project by key for a global administrator, who
// need not see it.
func (s *Server) adminProject(r *http.Request) (int64, string, error) {
	if err := requireAdmin(r); err != nil {
		return 0, "", err
	}
	id, err := s.projectByKey(r.Context(), r.PathValue("key"))
	if err != nil {
		return 0, "", err
	}
	var key string
	err = s.db.QueryRowContext(r.Context(), `SELECT pkey FROM projects WHERE id = ?`, id).Scan(&key)
	return id, key, err
}

// archiveProject hides a project and makes it read-only, or brings it back.
func (s *Server) archiveProject(archived bool) handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		id, key, err := s.adminProject(r)
		if err != nil {
			return err
		}
		res, err := s.db.ExecContext(r.Context(), `UPDATE projects SET archived = ? WHERE id = ?`, archived, id)
		if err != nil {
			return err
		}
		s.perms.Invalidate()
		if n, _ := res.RowsAffected(); n > 0 {
			action := audit.ProjectArchive
			if !archived {
				action = audit.ProjectUnarchive
			}
			s.auditLog(r, action, audit.Target{Type: "project", ID: strconv.FormatInt(id, 10), Label: key}, nil)
		}
		p, err := s.projectSettings(r.Context(), id)
		if err != nil {
			return err
		}
		return writeJSON(w, http.StatusOK, p)
	}
}

// renameProject gives a project another key. Issue keys are not stored: they
// are the project key and the issue number, so the new key shows at once on
// every issue. The old key goes to project_keys, which findIssue and
// projectByKey read, so OLD-12 keeps finding its issue. A key the project
// had before can be taken again: its row in project_keys is then removed.
func (s *Server) renameProject(w http.ResponseWriter, r *http.Request) error {
	id, key, err := s.adminProject(r)
	if err != nil {
		return err
	}
	var in struct {
		NewKey  string `json:"newKey"`
		Confirm string `json:"confirm"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	if strings.TrimSpace(in.Confirm) != key {
		return fail(http.StatusConflict, i18n.ProjectRenameConfirm)
	}
	ctx := r.Context()
	newKey := strings.ToUpper(strings.TrimSpace(in.NewKey))
	// A key the project once had may come back; any other taken key is refused.
	var back int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM project_keys WHERE pkey = ? AND project_id = ?`,
		newKey, id).Scan(&back); err != nil {
		return err
	}
	if back == 0 {
		if err := s.checkProjectKey(ctx, newKey); err != nil {
			return err
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM project_keys WHERE pkey = ? AND project_id = ?`, newKey, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE projects SET pkey = ? WHERE id = ?`, newKey, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO project_keys (pkey, project_id) VALUES (?, ?)`, key, id); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.perms.Invalidate()
	s.auditLog(r, audit.ProjectRename, audit.Target{Type: "project", ID: strconv.FormatInt(id, 10), Label: newKey},
		map[string]any{"oldKey": key, "newKey": newKey})
	p, err := s.projectSettings(ctx, id)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, p)
}

// deleteProject removes a project and everything in it. One with issues
// needs its key typed as confirmation.
func (s *Server) deleteProject(w http.ResponseWriter, r *http.Request) error {
	id, key, err := s.adminProject(r)
	if err != nil {
		return err
	}
	var in struct {
		Confirm string `json:"confirm"`
	}
	if r.ContentLength != 0 {
		if err := readJSON(r, &in); err != nil {
			return err
		}
	}
	ctx := r.Context()
	var issues int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM issues WHERE project_id = ?`, id).Scan(&issues); err != nil {
		return err
	}
	if issues > 0 && strings.TrimSpace(in.Confirm) != key {
		return &httpError{status: http.StatusConflict, extra: map[string]any{"issues": issues},
			msg: i18n.M(i18n.ProjectDeleteConfirm)}
	}
	var files []int64
	if err := sqlq.Each(ctx, s.db, `SELECT a.id FROM attachments a JOIN issues i ON i.id = a.issue_id
		WHERE i.project_id = ?`, func(rows *sql.Rows) error {
		var a int64
		if err := rows.Scan(&a); err != nil {
			return err
		}
		files = append(files, a)
		return nil
	}, id); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Issues first: they hold restricted references (security levels) that
	// the project's own cascade would otherwise trip over.
	if _, err := tx.ExecContext(ctx, `UPDATE issues SET parent_id = NULL WHERE project_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM issues WHERE project_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM projects WHERE id = ?`, id); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	for _, a := range files {
		if err := os.Remove(s.attachmentPath(a)); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.log.Warn("attachment file not removed", "id", a, "err", err)
		}
	}
	s.perms.Invalidate()
	s.auditLog(r, audit.ProjectDelete, audit.Target{Type: "project", ID: strconv.FormatInt(id, 10), Label: key},
		map[string]any{"issues": issues, "attachments": len(files)})
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// adminProjects lists every project, seen or not, for global administrators.
func (s *Server) adminProjects(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	ctx := r.Context()
	type row struct {
		ID       int64    `json:"id"`
		Key      string   `json:"key"`
		Name     string   `json:"name"`
		Kind     string   `json:"kind"`
		Lead     *userRef `json:"lead"`
		Category *idName  `json:"category"`
		Archived bool     `json:"archived"`
		Issues   int      `json:"issues"`
		lead     sql.NullInt64
	}
	list := []row{}
	var leads []int64
	if err := sqlq.Each(ctx, s.db, `SELECT p.id, p.pkey, p.name, p.kind, p.lead_id, c.id, c.name, p.archived,
		(SELECT COUNT(*) FROM issues i WHERE i.project_id = p.id)
		FROM projects p LEFT JOIN project_categories c ON c.id = p.category_id ORDER BY p.archived, p.name`,
		func(rows *sql.Rows) error {
			var p row
			var cid sql.NullInt64
			var cname sql.NullString
			if err := rows.Scan(&p.ID, &p.Key, &p.Name, &p.Kind, &p.lead, &cid, &cname, &p.Archived, &p.Issues); err != nil {
				return err
			}
			if cid.Valid {
				p.Category = &idName{cid.Int64, cname.String}
			}
			leads = append(leads, p.lead.Int64)
			list = append(list, p)
			return nil
		}); err != nil {
		return err
	}
	users, err := s.users(ctx, leads...)
	if err != nil {
		return err
	}
	for i := range list {
		list[i].Lead = refOf(users, list[i].lead)
	}
	return writeJSON(w, http.StatusOK, list)
}

// Categories -----------------------------------------------------------------------

type categoryOut struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Projects    int     `json:"projects"`
}

func (s *Server) categories(ctx context.Context, where string, args ...any) ([]categoryOut, error) {
	out := []categoryOut{}
	err := sqlq.Each(ctx, s.db, `SELECT c.id, c.name, c.description,
		(SELECT COUNT(*) FROM projects p WHERE p.category_id = c.id) FROM project_categories c
		WHERE `+where+` ORDER BY c.name`, func(rows *sql.Rows) error {
		var c categoryOut
		var desc sql.NullString
		if err := rows.Scan(&c.ID, &c.Name, &desc, &c.Projects); err != nil {
			return err
		}
		c.Description = sqlq.String(desc)
		out = append(out, c)
		return nil
	}, args...)
	return out, err
}

func (s *Server) listCategories(w http.ResponseWriter, r *http.Request) error {
	out, err := s.categories(r.Context(), "TRUE")
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

func (s *Server) categoryNameTaken(ctx context.Context, name string, except int64) error {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM project_categories WHERE LOWER(name) = LOWER(?) AND id <> ?`,
		name, except).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return failf(http.StatusConflict, i18n.CommonAlreadyExists, name)
	}
	return nil
}

func (s *Server) saveCategory(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	var in struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	ctx := r.Context()
	var id int64
	creating := r.PathValue("id") == ""
	if !creating {
		var err error
		if id, err = pathID(r, "id"); err != nil {
			return err
		}
		if list, err := s.categories(ctx, "c.id = ?", id); err != nil {
			return err
		} else if len(list) == 0 {
			return errNotFound
		}
	} else if in.Name == nil {
		return failf(http.StatusBadRequest, i18n.CommonNameRequired, 255)
	}
	if in.Name != nil {
		name, err := cleanName(*in.Name, 255)
		if err != nil {
			return err
		}
		if err := s.categoryNameTaken(ctx, name, id); err != nil {
			return err
		}
		if creating {
			res, err := s.db.ExecContext(ctx, `INSERT INTO project_categories (name, description) VALUES (?, ?)`,
				name, optDescription(in.Description))
			if err != nil {
				return err
			}
			id, _ = res.LastInsertId()
		} else if _, err := s.db.ExecContext(ctx, `UPDATE project_categories SET name = ? WHERE id = ?`, name, id); err != nil {
			return err
		}
	}
	if !creating && in.Description != nil {
		if _, err := s.db.ExecContext(ctx, `UPDATE project_categories SET description = ? WHERE id = ?`,
			optDescription(in.Description), id); err != nil {
			return err
		}
	}
	list, err := s.categories(ctx, "c.id = ?", id)
	if err != nil || len(list) == 0 {
		return err
	}
	action, status := audit.CategoryUpdate, http.StatusOK
	if creating {
		action, status = audit.CategoryCreate, http.StatusCreated
	}
	s.auditLog(r, action, audit.Target{Type: "category", ID: strconv.FormatInt(id, 10), Label: list[0].Name},
		map[string]any{"description": list[0].Description})
	return writeJSON(w, status, list[0])
}

func (s *Server) deleteCategory(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	ctx := r.Context()
	list, err := s.categories(ctx, "c.id = ?", id)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		return errNotFound
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM project_categories WHERE id = ?`, id); err != nil {
		return err
	}
	s.auditLog(r, audit.CategoryDelete, audit.Target{Type: "category", ID: strconv.FormatInt(id, 10), Label: list[0].Name},
		map[string]any{"projects": list[0].Projects})
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// today is the current day in the instance's time zone.
func (s *Server) today() string { return time.Now().In(s.zone()).Format(time.DateOnly) }
