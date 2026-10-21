package api

// The shared catalogues, for global administrators: issue types,
// priorities, resolutions and link types (statuses have their own
// endpoints, in admin_statuses.go). An item still in use is only deleted
// when its issues (or links) are moved to another item, with history.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"solstice/internal/audit"
	"solstice/internal/i18n"
	"solstice/internal/sqlq"
)

// catalogueKind describes one catalogue and what sets it apart.
type catalogueKind struct {
	table     string
	audit     string   // audit action prefix
	fields    []string // fields a request may set, besides name
	required  []string // fields a creation must set, besides name
	numericID bool     // AUTO_INCREMENT ids, written as numbers
	query     string   // SELECT of the items as t, without WHERE
	scan      func(*sql.Rows) (catalogueItem, error)
	move      catalogueMove
	// checkDelete refuses deleting any item, before the replacement is read.
	checkDelete func(ctx context.Context, db *sql.DB) error
	// checkReplacement refuses moving what uses from to to.
	checkReplacement func(from, to catalogueItem) error
}

// catalogueMove points what uses from at to instead, by the user by, and
// counts what moved.
type catalogueMove func(ctx context.Context, tx *sql.Tx, by int64, from, to catalogueItem) (int, error)

var catalogueKinds = map[string]catalogueKind{
	"issue-types": {table: "issue_types", audit: "issuetype", fields: []string{"description", "subtask"},
		query: `SELECT t.id, t.name, t.description, t.position, t.subtask,
			(SELECT COUNT(*) FROM issues i WHERE i.issue_type_id = t.id),
			(SELECT COUNT(*) FROM project_issue_types p WHERE p.issue_type_id = t.id) FROM issue_types t`,
		scan: scanIssueType, move: moveIssueType, checkDelete: keepLastIssueType, checkReplacement: sameSubtask},
	"priorities": {table: "priorities", audit: "priority", fields: []string{"description", "color", "default"},
		query: `SELECT t.id, t.name, t.description, t.position, t.color, t.is_default,
			(SELECT COUNT(*) FROM issues i WHERE i.priority_id = t.id) FROM priorities t`,
		scan: scanPriority, move: moveIssues("priority_id", "priority")},
	"resolutions": {table: "resolutions", audit: "resolution", fields: []string{"description", "default"},
		query: `SELECT t.id, t.name, t.description, t.position, t.is_default,
			(SELECT COUNT(*) FROM issues i WHERE i.resolution_id = t.id) FROM resolutions t`,
		scan: scanResolution, move: moveIssues("resolution_id", "resolution")},
	"link-types": {table: "link_types", audit: "linktype", fields: []string{"inward", "outward"},
		required: []string{"inward", "outward"}, numericID: true,
		query: `SELECT t.id, t.name, t.inward, t.outward, t.position,
			(SELECT COUNT(*) FROM issue_links l WHERE l.type_id = t.id) FROM link_types t`,
		scan: scanLinkType, move: moveLinks},
}

type catalogueItem struct {
	ID          string  `json:"-"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	Position    int     `json:"position"`
	// Kind-specific.
	Subtask  *bool   `json:"subtask,omitempty"`
	Color    *string `json:"color,omitempty"`
	Default  *bool   `json:"default,omitempty"`
	Inward   *string `json:"inward,omitempty"`
	Outward  *string `json:"outward,omitempty"`
	Issues   *int    `json:"issues,omitempty"`
	Projects *int    `json:"projects,omitempty"`
	Links    *int    `json:"links,omitempty"`

	numericID bool
}

// MarshalJSON writes the id as Jira does: text, but a number for link types.
func (it catalogueItem) MarshalJSON() ([]byte, error) {
	type plain catalogueItem
	var id any = it.ID
	if it.numericID {
		n, _ := strconv.ParseInt(it.ID, 10, 64)
		id = n
	}
	body, err := json.Marshal(plain(it))
	if err != nil {
		return nil, err
	}
	head, err := json.Marshal(map[string]any{"id": id})
	if err != nil {
		return nil, err
	}
	// {"id":…} + the other fields.
	if len(body) == 2 {
		return head, nil
	}
	return append(append(head[:len(head)-1], ','), body[1:]...), nil
}

func scanIssueType(rows *sql.Rows) (catalogueItem, error) {
	var it catalogueItem
	var desc sql.NullString
	var sub bool
	var issues, projects int
	err := rows.Scan(&it.ID, &it.Name, &desc, &it.Position, &sub, &issues, &projects)
	it.Description, it.Subtask, it.Issues, it.Projects = &desc.String, &sub, &issues, &projects
	return it, err
}

func scanPriority(rows *sql.Rows) (catalogueItem, error) {
	var it catalogueItem
	var desc, color sql.NullString
	var def bool
	var issues int
	err := rows.Scan(&it.ID, &it.Name, &desc, &it.Position, &color, &def, &issues)
	it.Description, it.Color, it.Default, it.Issues = &desc.String, &color.String, &def, &issues
	return it, err
}

func scanResolution(rows *sql.Rows) (catalogueItem, error) {
	var it catalogueItem
	var desc sql.NullString
	var def bool
	var issues int
	err := rows.Scan(&it.ID, &it.Name, &desc, &it.Position, &def, &issues)
	it.Description, it.Default, it.Issues = &desc.String, &def, &issues
	return it, err
}

func scanLinkType(rows *sql.Rows) (catalogueItem, error) {
	var it catalogueItem
	var inward, outward string
	var links int
	err := rows.Scan(&it.ID, &it.Name, &inward, &outward, &it.Position, &links)
	it.Inward, it.Outward, it.Links = &inward, &outward, &links
	return it, err
}

func (s *Server) catalogueItems(ctx context.Context, c catalogueKind, where string, args ...any) ([]catalogueItem, error) {
	out := []catalogueItem{}
	err := sqlq.Each(ctx, s.db, c.query+` WHERE `+where+` ORDER BY t.position, t.name, t.id`, func(rows *sql.Rows) error {
		it, err := c.scan(rows)
		if err != nil {
			return err
		}
		it.numericID = c.numericID
		out = append(out, it)
		return nil
	}, args...)
	return out, err
}

func (s *Server) catalogueItem(ctx context.Context, c catalogueKind, id string) (catalogueItem, error) {
	list, err := s.catalogueItems(ctx, c, "t.id = ?", id)
	if err != nil {
		return catalogueItem{}, err
	}
	if len(list) == 0 {
		return catalogueItem{}, errNotFound
	}
	return list[0], nil
}

func (s *Server) listCatalogue(c catalogueKind) handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		if err := requireAdmin(r); err != nil {
			return err
		}
		out, err := s.catalogueItems(r.Context(), c, "TRUE")
		if err != nil {
			return err
		}
		return writeJSON(w, http.StatusOK, out)
	}
}

var colorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// saveCatalogueItem creates an item (no id in the path) or changes one.
func (s *Server) saveCatalogueItem(c catalogueKind) handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		if err := requireAdmin(r); err != nil {
			return err
		}
		var raw map[string]json.RawMessage
		if err := readJSON(r, &raw); err != nil {
			return err
		}
		for k := range raw {
			if k != "name" && !containsStr(c.fields, k) {
				return failf(http.StatusBadRequest, i18n.FieldUnknown, k)
			}
		}
		ctx := r.Context()
		id := r.PathValue("id")
		creating := id == ""
		var before *catalogueItem
		if !creating {
			it, err := s.catalogueItem(ctx, c, id)
			if err != nil {
				return err
			}
			before = &it
		} else if _, ok := raw["name"]; !ok {
			return failf(http.StatusBadRequest, i18n.CommonNameRequired, 255)
		}
		sets, err := s.parseCatalogueFields(ctx, c, raw, id, before)
		if err != nil {
			return err
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if creating {
			if id, err = insertCatalogueItem(ctx, tx, c, sets); err != nil {
				return err
			}
		}
		if err := applyCatalogueSets(ctx, tx, c, id, sets); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		s.forgetCatalogue()
		it, err := s.catalogueItem(ctx, c, id)
		if err != nil {
			return err
		}
		target := audit.Target{Type: c.audit, ID: id, Label: it.Name}
		if creating {
			s.auditLog(r, c.audit+".create", target, nil)
			return writeJSON(w, http.StatusCreated, it)
		}
		changes := map[string]any{}
		for k, v := range sets {
			changes[k] = v
		}
		diff(changes, "name", before.Name, it.Name)
		s.auditLog(r, c.audit+".update", target, changes)
		return writeJSON(w, http.StatusOK, it)
	}
}

// parseCatalogueFields checks the fields of a request and gives the columns
// they set. before is the item changed, nil on creation.
func (s *Server) parseCatalogueFields(ctx context.Context, c catalogueKind, raw map[string]json.RawMessage, id string,
	before *catalogueItem) (map[string]any, error) {
	str := func(k string) (string, error) {
		var v string
		if json.Unmarshal(raw[k], &v) != nil {
			return "", failf(http.StatusBadRequest, i18n.FieldValueInvalid, k)
		}
		return strings.TrimSpace(v), nil
	}
	sets := map[string]any{}
	for k := range raw {
		switch k {
		case "name":
			v, err := str(k)
			if err != nil {
				return nil, err
			}
			name, err := cleanName(v, 255)
			if err != nil {
				return nil, err
			}
			if err := s.nameTaken(ctx, c.table, name, id, i18n.CommonAlreadyExists); err != nil {
				return nil, err
			}
			sets["name"] = name
		case "description":
			var v *string
			if json.Unmarshal(raw[k], &v) != nil {
				return nil, failf(http.StatusBadRequest, i18n.FieldValueInvalid, k)
			}
			sets["description"] = optDescription(v)
		case "inward", "outward":
			v, err := str(k)
			if err != nil {
				return nil, err
			}
			if v, err = cleanName(v, 255); err != nil {
				return nil, err
			}
			sets[k] = v
		case "color":
			v, err := str(k)
			if err != nil {
				return nil, err
			}
			if v != "" && !colorPattern.MatchString(v) {
				return nil, fail(http.StatusBadRequest, i18n.CatalogColorInvalid)
			}
			sets["color"] = strings.ToLower(v)
			if v == "" {
				sets["color"] = nil
			}
		case "subtask":
			var v bool
			if json.Unmarshal(raw[k], &v) != nil {
				return nil, failf(http.StatusBadRequest, i18n.FieldValueInvalid, k)
			}
			if before != nil && v != *before.Subtask && *before.Issues > 0 {
				return nil, fail(http.StatusConflict, i18n.CatalogTypeSubtaskLocked)
			}
			sets["subtask"] = v
		case "default":
			var v bool
			if json.Unmarshal(raw[k], &v) != nil {
				return nil, failf(http.StatusBadRequest, i18n.FieldValueInvalid, k)
			}
			sets["is_default"] = v
		}
	}
	if before == nil {
		for _, k := range c.required {
			if sets[k] == nil {
				return nil, fail(http.StatusBadRequest, i18n.CatalogLinkTypeIncomplete)
			}
		}
	}
	return sets, nil
}

// insertCatalogueItem adds an item, last, with its name and required
// fields, and gives its id.
func insertCatalogueItem(ctx context.Context, tx *sql.Tx, c catalogueKind, sets map[string]any) (string, error) {
	var id string
	columns, args := []string{"name"}, []any{sets["name"]}
	for _, k := range c.required {
		columns, args = append(columns, k), append(args, sets[k])
	}
	if !c.numericID {
		var err error
		if id, err = nextCatalogID(ctx, tx, c.table); err != nil {
			return "", err
		}
		columns, args = append([]string{"id"}, columns...), append([]any{id}, args...)
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO `+c.table+` (`+strings.Join(columns, ", ")+`, position)
		SELECT `+strings.Repeat("?, ", len(columns))+`COALESCE(MAX(position), 0) + 1 FROM `+c.table, args...)
	if err != nil || !c.numericID {
		return id, err
	}
	n, _ := res.LastInsertId()
	return strconv.FormatInt(n, 10), nil
}

// applyCatalogueSets writes the columns of an item; a new default item
// unsets the previous one.
func applyCatalogueSets(ctx context.Context, tx *sql.Tx, c catalogueKind, id string, sets map[string]any) error {
	if def, _ := sets["is_default"].(bool); def {
		if _, err := tx.ExecContext(ctx, `UPDATE `+c.table+` SET is_default = FALSE`); err != nil {
			return err
		}
	}
	for column, v := range sets {
		if _, err := tx.ExecContext(ctx, `UPDATE `+c.table+` SET `+column+` = ? WHERE id = ?`, v, id); err != nil {
			return err
		}
	}
	return nil
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// readOrder reads {"ids": [...]}: text or numbers.
func readOrder(r *http.Request) ([]string, error) {
	var in struct {
		IDs []json.RawMessage `json:"ids"`
	}
	if err := readJSON(r, &in); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(in.IDs))
	for _, raw := range in.IDs {
		var s string
		if json.Unmarshal(raw, &s) != nil {
			var n json.Number
			if json.Unmarshal(raw, &n) != nil {
				return nil, failf(http.StatusBadRequest, i18n.FieldValueInvalid, "ids")
			}
			s = n.String()
		}
		out = append(out, s)
	}
	return out, nil
}

func (s *Server) orderCatalogue(c catalogueKind) handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		if err := requireAdmin(r); err != nil {
			return err
		}
		ids, err := readOrder(r)
		if err != nil {
			return err
		}
		ctx := r.Context()
		list, err := s.catalogueItems(ctx, c, "TRUE")
		if err != nil {
			return err
		}
		if err := sameIDs(list, ids, func(it catalogueItem) string { return it.ID }); err != nil {
			return err
		}
		if err := s.setPositions(ctx, c.table, ids); err != nil {
			return err
		}
		s.forgetCatalogue()
		s.auditLog(r, c.audit+".order", audit.Target{Type: c.audit}, map[string]any{"ids": ids})
		out, err := s.catalogueItems(ctx, c, "TRUE")
		if err != nil {
			return err
		}
		return writeJSON(w, http.StatusOK, out)
	}
}

func (s *Server) setPositions(ctx context.Context, table string, ids []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, id := range ids {
		if _, err := tx.ExecContext(ctx, `UPDATE `+table+` SET position = ? WHERE id = ?`, i+1, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// deleteCatalogueItem removes an item. One still in use needs moveTo:
// another item its issues (or links) go to.
func (s *Server) deleteCatalogueItem(c catalogueKind) handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		if err := requireAdmin(r); err != nil {
			return err
		}
		var in struct {
			MoveTo *json.RawMessage `json:"moveTo"`
		}
		if err := readOptionalJSON(r, &in); err != nil {
			return err
		}
		ctx := r.Context()
		id := r.PathValue("id")
		it, err := s.catalogueItem(ctx, c, id)
		if err != nil {
			return err
		}
		used, usage := catalogueUsage(it)
		if c.checkDelete != nil {
			if err := c.checkDelete(ctx, s.db); err != nil {
				return err
			}
		}
		to, err := s.replacement(ctx, c, it, in.MoveTo)
		if err != nil {
			return err
		}
		if used > 0 && to == nil {
			return &httpError{status: http.StatusConflict, extra: map[string]any{"usage": usage},
				msg: i18n.M(i18n.CatalogItemInUse)}
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		moved, err := moveCatalogueItems(ctx, tx, c, currentUser(r).ID, it, to)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+c.table+` WHERE id = ?`, id); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		s.forgetCatalogue()
		details := map[string]any{"moved": moved}
		if to != nil {
			details["movedTo"] = to.Name
		}
		s.auditLog(r, c.audit+".delete", audit.Target{Type: c.audit, ID: id, Label: it.Name}, details)
		w.WriteHeader(http.StatusNoContent)
		return nil
	}
}

// catalogueUsage counts what uses an item: issues, projects, links, as its
// kind counts them.
func catalogueUsage(it catalogueItem) (int, map[string]int) {
	used, usage := 0, map[string]int{}
	for k, n := range map[string]*int{"issues": it.Issues, "projects": it.Projects, "links": it.Links} {
		if n != nil {
			used += *n
			usage[k] = *n
		}
	}
	return used, usage
}

// replacement reads moveTo, text or a number: another item of the
// catalogue, or nil when none is given.
func (s *Server) replacement(ctx context.Context, c catalogueKind, from catalogueItem, moveTo *json.RawMessage) (*catalogueItem, error) {
	if moveTo == nil || string(*moveTo) == "null" {
		return nil, nil
	}
	var target string
	if json.Unmarshal(*moveTo, &target) != nil {
		var n json.Number
		if json.Unmarshal(*moveTo, &n) != nil {
			return nil, failf(http.StatusBadRequest, i18n.FieldValueInvalid, "moveTo")
		}
		target = n.String()
	}
	to, err := s.catalogueItem(ctx, c, target)
	if errors.Is(err, errNotFound) || (err == nil && to.ID == from.ID) {
		return nil, fail(http.StatusBadRequest, i18n.CatalogReplacementNotFound)
	}
	if err != nil {
		return nil, err
	}
	if c.checkReplacement != nil {
		if err := c.checkReplacement(from, to); err != nil {
			return nil, err
		}
	}
	return &to, nil
}

// moveCatalogueItems points what uses from at to, by the user by, and
// counts what moved; nothing moves without to.
func moveCatalogueItems(ctx context.Context, tx *sql.Tx, c catalogueKind, by int64, from catalogueItem,
	to *catalogueItem) (int, error) {
	if to == nil {
		return 0, nil
	}
	return c.move(ctx, tx, by, from, *to)
}

// moveIssues gives the move of a catalogue that issues name in column,
// with a history line of field per issue.
func moveIssues(column, field string) catalogueMove {
	return func(ctx context.Context, tx *sql.Tx, by int64, from, to catalogueItem) (int, error) {
		issues, err := idsOf(ctx, tx, `SELECT id FROM issues WHERE `+column+` = ?`, from.ID)
		if err != nil {
			return 0, err
		}
		now := time.Now().UTC()
		for _, issue := range issues {
			if _, err := tx.ExecContext(ctx, `UPDATE issues SET `+column+` = ?, updated_at = ? WHERE id = ?`,
				to.ID, now, issue); err != nil {
				return 0, err
			}
			if err := recordChanges(ctx, tx, issue, by, now,
				[]change{{field, from.ID, from.Name, to.ID, to.Name}}); err != nil {
				return 0, err
			}
		}
		return len(issues), nil
	}
}

// moveIssueType moves the issues, and projects offering from offer to.
func moveIssueType(ctx context.Context, tx *sql.Tx, by int64, from, to catalogueItem) (int, error) {
	moved, err := moveIssues("issue_type_id", "issuetype")(ctx, tx, by, from, to)
	if err != nil {
		return 0, err
	}
	_, err = tx.ExecContext(ctx, `INSERT IGNORE INTO project_issue_types (project_id, issue_type_id, position)
		SELECT project_id, ?, position FROM project_issue_types WHERE issue_type_id = ?`, to.ID, from.ID)
	return moved, err
}

func moveLinks(ctx context.Context, tx *sql.Tx, _ int64, from, to catalogueItem) (int, error) {
	res, err := tx.ExecContext(ctx, `UPDATE issue_links SET type_id = ? WHERE type_id = ?`, to.ID, from.ID)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// keepLastIssueType refuses deleting the only issue type.
func keepLastIssueType(ctx context.Context, db *sql.DB) error {
	var n int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM issue_types`).Scan(&n); err != nil {
		return err
	}
	if n <= 1 {
		return fail(http.StatusConflict, i18n.CatalogLastIssueType)
	}
	return nil
}

// sameSubtask keeps sub-tasks among sub-tasks.
func sameSubtask(from, to catalogueItem) error {
	if *to.Subtask != *from.Subtask {
		return fail(http.StatusBadRequest, i18n.CatalogReplacementKind)
	}
	return nil
}
