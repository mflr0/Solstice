package api

// The shared catalogues, for global administrators: issue types,
// priorities, resolutions and link types (statuses have their own
// endpoints, in workflow_admin.go). An item still in use is only deleted
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

// catalogueKind describes one catalogue.
type catalogueKind struct {
	table   string
	audit   string   // audit action prefix
	fields  []string // fields a request may set, besides name
	history string   // history field of issues moved on delete ("" for link types)
	column  string   // issues' column naming the item ("" for link types)
}

var catalogueKinds = map[string]catalogueKind{
	"issue-types": {table: "issue_types", audit: "issuetype", fields: []string{"description", "subtask"},
		history: "issuetype", column: "issue_type_id"},
	"priorities": {table: "priorities", audit: "priority", fields: []string{"description", "color", "default"},
		history: "priority", column: "priority_id"},
	"resolutions": {table: "resolutions", audit: "resolution", fields: []string{"description", "default"},
		history: "resolution", column: "resolution_id"},
	"link-types": {table: "link_types", audit: "linktype", fields: []string{"inward", "outward"}},
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
}

// MarshalJSON writes the id as Jira does: text, but a number for link types.
func (it catalogueItem) MarshalJSON() ([]byte, error) {
	type plain catalogueItem
	var id any = it.ID
	if it.Links != nil {
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

func (s *Server) catalogueItems(ctx context.Context, c catalogueKind, where string, args ...any) ([]catalogueItem, error) {
	var q string
	switch c.table {
	case "issue_types":
		q = `SELECT t.id, t.name, t.description, t.position, t.subtask,
			(SELECT COUNT(*) FROM issues i WHERE i.issue_type_id = t.id),
			(SELECT COUNT(*) FROM project_issue_types p WHERE p.issue_type_id = t.id) FROM issue_types t`
	case "priorities":
		q = `SELECT t.id, t.name, t.description, t.position, t.color, t.is_default,
			(SELECT COUNT(*) FROM issues i WHERE i.priority_id = t.id) FROM priorities t`
	case "resolutions":
		q = `SELECT t.id, t.name, t.description, t.position, t.is_default,
			(SELECT COUNT(*) FROM issues i WHERE i.resolution_id = t.id) FROM resolutions t`
	case "link_types":
		q = `SELECT t.id, t.name, t.inward, t.outward, t.position,
			(SELECT COUNT(*) FROM issue_links l WHERE l.type_id = t.id) FROM link_types t`
	}
	out := []catalogueItem{}
	err := sqlq.Each(ctx, s.db, q+` WHERE `+where+` ORDER BY t.position, t.name, t.id`, func(rows *sql.Rows) error {
		var it catalogueItem
		var desc sql.NullString
		var n int
		var err error
		switch c.table {
		case "issue_types":
			var sub bool
			var projects int
			err = rows.Scan(&it.ID, &it.Name, &desc, &it.Position, &sub, &n, &projects)
			it.Subtask, it.Issues, it.Projects = &sub, &n, &projects
		case "priorities":
			var color sql.NullString
			var def bool
			err = rows.Scan(&it.ID, &it.Name, &desc, &it.Position, &color, &def, &n)
			it.Color, it.Default, it.Issues = sqlq.String(color), &def, &n
			if it.Color == nil {
				empty := ""
				it.Color = &empty
			}
		case "resolutions":
			var def bool
			err = rows.Scan(&it.ID, &it.Name, &desc, &it.Position, &def, &n)
			it.Default, it.Issues = &def, &n
		case "link_types":
			var inward, outward string
			err = rows.Scan(&it.ID, &it.Name, &inward, &outward, &it.Position, &n)
			it.Inward, it.Outward, it.Links = &inward, &outward, &n
		}
		if err != nil {
			return err
		}
		if c.table != "link_types" {
			it.Description = sqlq.String(desc)
			if it.Description == nil {
				empty := ""
				it.Description = &empty
			}
		}
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
		var before catalogueItem
		if !creating {
			var err error
			if before, err = s.catalogueItem(ctx, c, id); err != nil {
				return err
			}
		} else if _, ok := raw["name"]; !ok {
			return failf(http.StatusBadRequest, i18n.CommonNameRequired, 255)
		}
		str := func(k string) (string, error) {
			var v string
			if json.Unmarshal(raw[k], &v) != nil {
				return "", failf(http.StatusBadRequest, i18n.FieldValueInvalid, k)
			}
			return strings.TrimSpace(v), nil
		}
		sets := map[string]any{}
		makeDefault := false
		for k := range raw {
			switch k {
			case "name":
				v, err := str(k)
				if err != nil {
					return err
				}
				name, err := cleanName(v, 255)
				if err != nil {
					return err
				}
				var n int
				if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+c.table+` WHERE LOWER(name) = LOWER(?) AND id <> ?`,
					name, id).Scan(&n); err != nil {
					return err
				}
				if n > 0 {
					return failf(http.StatusConflict, i18n.CommonAlreadyExists, name)
				}
				sets["name"] = name
			case "description":
				var v *string
				if json.Unmarshal(raw[k], &v) != nil {
					return failf(http.StatusBadRequest, i18n.FieldValueInvalid, k)
				}
				sets["description"] = optDescription(v)
			case "inward", "outward":
				v, err := str(k)
				if err != nil {
					return err
				}
				if v, err = cleanName(v, 255); err != nil {
					return err
				}
				sets[k] = v
			case "color":
				v, err := str(k)
				if err != nil {
					return err
				}
				if v != "" && !colorPattern.MatchString(v) {
					return fail(http.StatusBadRequest, i18n.CatalogColorInvalid)
				}
				sets["color"] = strings.ToLower(v)
				if v == "" {
					sets["color"] = nil
				}
			case "subtask":
				var v bool
				if json.Unmarshal(raw[k], &v) != nil {
					return failf(http.StatusBadRequest, i18n.FieldValueInvalid, k)
				}
				if !creating && v != *before.Subtask && *before.Issues > 0 {
					return fail(http.StatusConflict, i18n.CatalogTypeSubtaskLocked)
				}
				sets["subtask"] = v
			case "default":
				var v bool
				if json.Unmarshal(raw[k], &v) != nil {
					return failf(http.StatusBadRequest, i18n.FieldValueInvalid, k)
				}
				makeDefault = v
				sets["is_default"] = v
			}
		}
		if creating && c.table == "link_types" && (sets["inward"] == nil || sets["outward"] == nil) {
			return fail(http.StatusBadRequest, i18n.CatalogLinkTypeIncomplete)
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if creating {
			if c.table == "link_types" {
				res, err := tx.ExecContext(ctx, `INSERT INTO link_types (name, inward, outward, position)
					SELECT ?, ?, ?, COALESCE(MAX(position), 0) + 1 FROM link_types`, sets["name"], sets["inward"], sets["outward"])
				if err != nil {
					return err
				}
				n, _ := res.LastInsertId()
				id = strconv.FormatInt(n, 10)
			} else {
				if id, err = nextCatalogID(ctx, tx, c.table); err != nil {
					return err
				}
				if _, err := tx.ExecContext(ctx, `INSERT INTO `+c.table+` (id, name, position)
					SELECT ?, ?, COALESCE(MAX(position), 0) + 1 FROM `+c.table, id, sets["name"]); err != nil {
					return err
				}
			}
		}
		if makeDefault {
			if _, err := tx.ExecContext(ctx, `UPDATE `+c.table+` SET is_default = FALSE`); err != nil {
				return err
			}
		}
		for column, v := range sets {
			if _, err := tx.ExecContext(ctx, `UPDATE `+c.table+` SET `+column+` = ? WHERE id = ?`, v, id); err != nil {
				return err
			}
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
		if before.Name != it.Name {
			changes["name"] = map[string]string{"from": before.Name, "to": it.Name}
		}
		s.auditLog(r, c.audit+".update", target, changes)
		return writeJSON(w, http.StatusOK, it)
	}
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
		if r.ContentLength != 0 {
			if err := readJSON(r, &in); err != nil {
				return err
			}
		}
		ctx := r.Context()
		id := r.PathValue("id")
		it, err := s.catalogueItem(ctx, c, id)
		if err != nil {
			return err
		}
		used := 0
		usage := map[string]int{}
		switch {
		case it.Issues != nil:
			used, usage["issues"] = *it.Issues, *it.Issues
			if it.Projects != nil {
				// A type some project still offers is in use too.
				used += *it.Projects
				usage["projects"] = *it.Projects
			}
		case it.Links != nil:
			used, usage["links"] = *it.Links, *it.Links
		}
		if c.table == "issue_types" {
			var n int
			if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM issue_types`).Scan(&n); err != nil {
				return err
			}
			if n <= 1 {
				return fail(http.StatusConflict, i18n.CatalogLastIssueType)
			}
		}
		var to *catalogueItem
		if in.MoveTo != nil && string(*in.MoveTo) != "null" {
			var target string
			if json.Unmarshal(*in.MoveTo, &target) != nil {
				var n json.Number
				if json.Unmarshal(*in.MoveTo, &n) != nil {
					return failf(http.StatusBadRequest, i18n.FieldValueInvalid, "moveTo")
				}
				target = n.String()
			}
			t, err := s.catalogueItem(ctx, c, target)
			if errors.Is(err, errNotFound) || (err == nil && t.ID == id) {
				return fail(http.StatusBadRequest, i18n.CatalogReplacementNotFound)
			}
			if err != nil {
				return err
			}
			if c.table == "issue_types" && *t.Subtask != *it.Subtask {
				return fail(http.StatusBadRequest, i18n.CatalogReplacementKind)
			}
			to = &t
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
		moved := 0
		if to != nil {
			if c.table == "link_types" {
				res, err := tx.ExecContext(ctx, `UPDATE issue_links SET type_id = ? WHERE type_id = ?`, to.ID, id)
				if err != nil {
					return err
				}
				n, _ := res.RowsAffected()
				moved = int(n)
			} else {
				issues, err := idsOf(ctx, tx, `SELECT id FROM issues WHERE `+c.column+` = ?`, id)
				if err != nil {
					return err
				}
				now := time.Now().UTC()
				for _, issue := range issues {
					if _, err := tx.ExecContext(ctx, `UPDATE issues SET `+c.column+` = ?, updated_at = ? WHERE id = ?`,
						to.ID, now, issue); err != nil {
						return err
					}
					if err := recordChanges(ctx, tx, issue, currentUser(r).ID, now,
						[]change{{c.history, id, it.Name, to.ID, to.Name}}); err != nil {
						return err
					}
				}
				moved = len(issues)
			}
			if c.table == "issue_types" {
				// Projects offering the old type offer the new one.
				if _, err := tx.ExecContext(ctx, `INSERT IGNORE INTO project_issue_types (project_id, issue_type_id, position)
					SELECT project_id, ?, position FROM project_issue_types WHERE issue_type_id = ?`, to.ID, id); err != nil {
					return err
				}
			}
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

// Statuses ----------------------------------------------------------------------

type statusAdminOut struct {
	statusRef
	Position int            `json:"position"`
	Usage    map[string]int `json:"usage"`
}

func (s *Server) statusList(ctx context.Context) ([]statusAdminOut, error) {
	out := []statusAdminOut{}
	err := sqlq.Each(ctx, s.db, `SELECT s.id, s.name, s.category, s.position,
		(SELECT COUNT(*) FROM issues i WHERE i.status_id = s.id),
		(SELECT COUNT(DISTINCT u.workflow_id) FROM (SELECT workflow_id, status_id FROM workflow_statuses
			UNION ALL SELECT workflow_id, from_status_id FROM workflow_transitions
			UNION ALL SELECT workflow_id, to_status_id FROM workflow_transitions
			UNION ALL SELECT id, initial_status_id FROM workflows) u WHERE u.status_id = s.id),
		(SELECT COUNT(*) FROM board_column_statuses b WHERE b.status_id = s.id)
		FROM statuses s ORDER BY s.position, s.name`, func(rows *sql.Rows) error {
		var st statusAdminOut
		var issues, workflows, columns int
		if err := rows.Scan(&st.ID, &st.Name, &st.Category, &st.Position, &issues, &workflows, &columns); err != nil {
			return err
		}
		st.Usage = map[string]int{"issues": issues, "workflows": workflows, "boardColumns": columns}
		out = append(out, st)
		return nil
	})
	return out, err
}

func (s *Server) adminStatuses(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	out, err := s.statusList(r.Context())
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

// statusUsageOut tells where a status is used: the workflows (with their
// project), the board columns and, per project, the issues in it.
type statusUsageOut struct {
	Workflows []statusWorkflow `json:"workflows"`
	Boards    []statusBoard    `json:"boards"`
	Projects  []statusProject  `json:"projects"`
}

type statusWorkflow struct {
	ID      int64        `json:"id"`
	Name    string       `json:"name"`
	Project projectBrief `json:"project"`
}

type statusBoard struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Column string `json:"column"`
}

type statusProject struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	Issues int    `json:"issues"`
}

func (s *Server) adminStatusUsage(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	ctx := r.Context()
	cat, err := s.catalogue(ctx)
	if err != nil {
		return err
	}
	id := r.PathValue("id")
	if _, ok := cat.status[id]; !ok {
		return errNotFound
	}
	out := statusUsageOut{Workflows: []statusWorkflow{}, Boards: []statusBoard{}, Projects: []statusProject{}}
	if err := sqlq.Each(ctx, s.db, `SELECT w.id, w.name, p.id, p.pkey, p.name FROM workflows w
		JOIN projects p ON p.id = w.project_id WHERE w.id IN (
			SELECT workflow_id FROM workflow_statuses WHERE status_id = ?
			UNION SELECT workflow_id FROM workflow_transitions WHERE from_status_id = ? OR to_status_id = ?
			UNION SELECT id FROM workflows WHERE initial_status_id = ?)
		ORDER BY p.pkey, w.name`, func(rows *sql.Rows) error {
		var w statusWorkflow
		if err := rows.Scan(&w.ID, &w.Name, &w.Project.ID, &w.Project.Key, &w.Project.Name); err != nil {
			return err
		}
		out.Workflows = append(out.Workflows, w)
		return nil
	}, id, id, id, id); err != nil {
		return err
	}
	if err := sqlq.Each(ctx, s.db, `SELECT b.id, b.name, c.name FROM board_column_statuses bs
		JOIN board_columns c ON c.id = bs.column_id JOIN boards b ON b.id = c.board_id
		WHERE bs.status_id = ? ORDER BY b.name, c.position`, func(rows *sql.Rows) error {
		var b statusBoard
		if err := rows.Scan(&b.ID, &b.Name, &b.Column); err != nil {
			return err
		}
		out.Boards = append(out.Boards, b)
		return nil
	}, id); err != nil {
		return err
	}
	if err := sqlq.Each(ctx, s.db, `SELECT p.pkey, p.name, COUNT(*) FROM issues i
		JOIN projects p ON p.id = i.project_id WHERE i.status_id = ? GROUP BY p.id, p.pkey, p.name
		ORDER BY p.pkey`, func(rows *sql.Rows) error {
		var p statusProject
		if err := rows.Scan(&p.Key, &p.Name, &p.Issues); err != nil {
			return err
		}
		out.Projects = append(out.Projects, p)
		return nil
	}, id); err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

func (s *Server) orderStatuses(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	ids, err := readOrder(r)
	if err != nil {
		return err
	}
	ctx := r.Context()
	list, err := s.statusList(ctx)
	if err != nil {
		return err
	}
	if err := sameIDs(list, ids, func(st statusAdminOut) string { return st.ID }); err != nil {
		return err
	}
	if err := s.setPositions(ctx, "statuses", ids); err != nil {
		return err
	}
	s.forgetCatalogue()
	s.auditLog(r, audit.StatusOrder, audit.Target{Type: "status"}, map[string]any{"ids": ids})
	out, err := s.statusList(ctx)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}
