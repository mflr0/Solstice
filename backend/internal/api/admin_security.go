package api

// The issue security levels of a project and their members. Each project has
// its own levels (the importer copies a Jira scheme into each project); an
// issue at a level is seen only by the level's members (see package perm).
// Only global administrators manage them: a project administrator could
// otherwise add themselves to a confidential level, or delete it and expose
// its issues.

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-sql-driver/mysql"

	"solstice/internal/audit"
	"solstice/internal/i18n"
	"solstice/internal/perm"
	"solstice/internal/sqlq"
)

type levelMemberOut struct {
	ID     int64    `json:"id"`
	Holder string   `json:"holder"`
	User   *userRef `json:"user"`
	Group  *idName  `json:"group"`
	Role   *idName  `json:"role"`
}

// label names a member in the audit log.
func (m levelMemberOut) label() string {
	switch {
	case m.User != nil:
		return m.User.Username
	case m.Group != nil:
		return m.Group.Name
	case m.Role != nil:
		return m.Role.Name
	}
	return m.Holder
}

type securityLevelOut struct {
	ID          int64            `json:"id"`
	Name        string           `json:"name"`
	Description *string          `json:"description"`
	Issues      int              `json:"issues"`
	MemberCount int              `json:"memberCount"`
	Members     []levelMemberOut `json:"members"`
}

// securityLevels loads the levels of a project (only the one with the id
// only points to, when it is not nil), with their members.
func (s *Server) securityLevels(ctx context.Context, projectID int64, only *int64) ([]securityLevelOut, error) {
	out := []securityLevelOut{}
	index := map[int64]int{}
	filter, args := "", []any{projectID}
	if only != nil {
		filter, args = " AND l.id = ?", append(args, *only)
	}
	err := sqlq.Each(ctx, s.db, `SELECT l.id, l.name, l.description,
		(SELECT COUNT(*) FROM issues i WHERE i.security_level_id = l.id)
		FROM security_levels l WHERE l.project_id = ?`+filter+` ORDER BY l.name, l.id`,
		func(rows *sql.Rows) error {
			var l securityLevelOut
			var desc sql.NullString
			if err := rows.Scan(&l.ID, &l.Name, &desc, &l.Issues); err != nil {
				return err
			}
			l.Description = sqlq.String(desc)
			l.Members = []levelMemberOut{}
			index[l.ID] = len(out)
			out = append(out, l)
			return nil
		}, args...)
	if err != nil {
		return nil, err
	}
	err = sqlq.Each(ctx, s.db, `SELECT m.level_id, m.id, m.holder, u.id, u.username, u.display_name, u.active,
		g.id, g.name, r.id, r.name
		FROM security_level_members m JOIN security_levels l ON l.id = m.level_id
		LEFT JOIN users u ON u.id = m.user_id LEFT JOIN user_groups g ON g.id = m.group_id
		LEFT JOIN project_roles r ON r.id = m.role_id
		WHERE l.project_id = ?`+filter+`
		ORDER BY m.holder, COALESCE(u.display_name, g.name, r.name, ''), m.id`, func(rows *sql.Rows) error {
		var level int64
		var m levelMemberOut
		var uid, gid, rid sql.NullInt64
		var username, display, gname, rname sql.NullString
		var active sql.NullBool
		if err := rows.Scan(&level, &m.ID, &m.Holder, &uid, &username, &display, &active, &gid, &gname, &rid,
			&rname); err != nil {
			return err
		}
		if uid.Valid {
			m.User = &userRef{uid.Int64, username.String, display.String, active.Bool}
		}
		if gid.Valid {
			m.Group = &idName{gid.Int64, gname.String}
		}
		if rid.Valid {
			m.Role = &idName{rid.Int64, rname.String}
		}
		i := index[level]
		out[i].Members = append(out[i].Members, m)
		out[i].MemberCount++
		return nil
	}, args...)
	return out, err
}

// securityLevel loads one level of the project: errNotFound for another
// project's.
func (s *Server) securityLevel(ctx context.Context, projectID, id int64) (securityLevelOut, error) {
	if id <= 0 {
		return securityLevelOut{}, errNotFound
	}
	list, err := s.securityLevels(ctx, projectID, &id)
	if err != nil {
		return securityLevelOut{}, err
	}
	if len(list) == 0 {
		return securityLevelOut{}, errNotFound
	}
	return list[0], nil
}

// levelPathID reads an id of the path: 0 and below are no id, so not found.
func levelPathID(r *http.Request, name string) (int64, error) {
	id, err := pathID(r, name)
	if err == nil && id <= 0 {
		return 0, errNotFound
	}
	return id, err
}

// levelProject loads the project of the route for a global administrator.
func (s *Server) levelProject(r *http.Request) (int64, error) {
	if err := requireAdmin(r); err != nil {
		return 0, err
	}
	return s.projectByKey(r.Context(), r.PathValue("key"))
}

func (s *Server) listSecurityLevels(w http.ResponseWriter, r *http.Request) error {
	projectID, err := s.levelProject(r)
	if err != nil {
		return err
	}
	out, err := s.securityLevels(r.Context(), projectID, nil)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

func (s *Server) securityLevelNameTaken(ctx context.Context, projectID int64, name string, except int64) error {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM security_levels WHERE project_id = ?
		AND LOWER(name) = LOWER(?) AND id <> ?`, projectID, name, except).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return failf(http.StatusConflict, i18n.CommonAlreadyExists, name)
	}
	return nil
}

// auditLevel writes a change to a level, on the project it belongs to.
func (s *Server) auditLevel(r *http.Request, action string, projectID int64, level string, more map[string]any) {
	details := map[string]any{"securityLevel": level}
	for k, v := range more {
		details[k] = v
	}
	key := s.projectKeyOf(r.Context(), projectID)
	s.auditLog(r, action, audit.Target{Type: "project", ID: strconv.FormatInt(projectID, 10), Label: key},
		details)
}

// saveSecurityLevel creates a level (no id in the path) or changes one.
func (s *Server) saveSecurityLevel(w http.ResponseWriter, r *http.Request) error {
	projectID, err := s.levelProject(r)
	if err != nil {
		return err
	}
	var in struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
	}
	raw, err := readJSONFields(r, &in)
	if err != nil {
		return err
	}
	ctx := r.Context()
	creating := r.PathValue("id") == ""
	var id int64
	var before securityLevelOut
	if !creating {
		if id, err = levelPathID(r, "id"); err != nil {
			return err
		}
		if before, err = s.securityLevel(ctx, projectID, id); err != nil {
			return err
		}
	} else if in.Name == nil {
		return failf(http.StatusBadRequest, i18n.CommonNameRequired, 255)
	}
	sets := map[string]any{}
	if in.Name != nil {
		name, err := cleanName(*in.Name, 255)
		if err != nil {
			return err
		}
		if err := s.securityLevelNameTaken(ctx, projectID, name, id); err != nil {
			return err
		}
		sets["name"] = name
	}
	if _, ok := raw["description"]; ok {
		sets["description"] = optDescription(in.Description)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if creating {
		res, err := tx.ExecContext(ctx, `INSERT INTO security_levels (project_id, name) VALUES (?, ?)`, projectID, sets["name"])
		if err != nil {
			return err
		}
		id, _ = res.LastInsertId()
	}
	for column, v := range sets {
		if _, err := tx.ExecContext(ctx, `UPDATE security_levels SET `+column+` = ? WHERE id = ?`, v, id); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.perms.Invalidate()
	level, err := s.securityLevel(ctx, projectID, id)
	if err != nil {
		return err
	}
	if creating {
		s.auditLevel(r, audit.SecurityLevelCreate, projectID, level.Name, nil)
		return writeJSON(w, http.StatusCreated, level)
	}
	if level.Name != before.Name || derefStr(level.Description) != derefStr(before.Description) {
		more := map[string]any{}
		if level.Name != before.Name {
			more["name"] = map[string]any{"from": before.Name, "to": level.Name}
		}
		if derefStr(level.Description) != derefStr(before.Description) {
			more["description"] = map[string]any{"from": derefStr(before.Description), "to": derefStr(level.Description)}
		}
		s.auditLevel(r, audit.SecurityLevelUpdate, projectID, level.Name, more)
	}
	return writeJSON(w, http.StatusOK, level)
}

// levelIssues counts the issues at a level.
func levelIssues(ctx context.Context, tx *sql.Tx, id int64) (int, error) {
	var n int
	err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM issues WHERE security_level_id = ?`, id).Scan(&n)
	return n, err
}

// levelInUse is the answer for a level that issues still use.
func levelInUse(issues int) error {
	return &httpError{status: http.StatusConflict, extra: map[string]any{"usage": map[string]int{"issues": issues}},
		msg: i18n.M(i18n.SecurityLevelInUse, issues)}
}

// deleteSecurityLevel removes a level. Issues still at it need moveTo, another
// level of the project, or clear: they then have no level (everyone who sees
// the project sees them), which is never the default.
func (s *Server) deleteSecurityLevel(w http.ResponseWriter, r *http.Request) error {
	projectID, err := s.levelProject(r)
	if err != nil {
		return err
	}
	id, err := levelPathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		MoveTo *int64 `json:"moveTo"`
		Clear  bool   `json:"clear"`
	}
	if err := readOptionalJSON(r, &in); err != nil {
		return err
	}
	ctx := r.Context()
	level, err := s.securityLevel(ctx, projectID, id)
	if err != nil {
		return err
	}
	if in.MoveTo != nil && in.Clear {
		return fail(http.StatusBadRequest, i18n.SecurityMoveOrClear)
	}
	var target *securityLevelOut
	if in.MoveTo != nil {
		other, err := s.securityLevel(ctx, projectID, *in.MoveTo)
		if errors.Is(err, errNotFound) || (err == nil && other.ID == id) {
			return fail(http.StatusBadRequest, i18n.SecurityTargetNotFound)
		}
		if err != nil {
			return err
		}
		target = &other
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// With the level row locked, no issue can be set to the level until the
	// commit: the count below holds to the delete.
	var locked int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM security_levels WHERE id = ? FOR UPDATE`, id).Scan(&locked); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errNotFound
		}
		return err
	}
	used, err := levelIssues(ctx, tx, id)
	if err != nil {
		return err
	}
	if used > 0 && target == nil && !in.Clear {
		return levelInUse(used)
	}
	moved := 0
	if used > 0 {
		var to any
		if target != nil {
			to = target.ID
		}
		issues, err := idsOf(ctx, tx, `SELECT id FROM issues WHERE security_level_id = ?`, id)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		if _, err := tx.ExecContext(ctx, `UPDATE issues SET security_level_id = ?, updated_at = ? WHERE security_level_id = ?`,
			to, now, id); err != nil {
			return err
		}
		for _, issue := range issues {
			if err := recordChanges(ctx, tx, issue, currentUser(r).ID, now,
				[]change{{field: "security", oldValue: id, newValue: to}}); err != nil {
				return err
			}
		}
		moved = len(issues)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM security_levels WHERE id = ?`, id); err != nil {
		// 1451: a row still refers to the level.
		var me *mysql.MySQLError
		if errors.As(err, &me) && me.Number == 1451 {
			if used, err := levelIssues(ctx, tx, id); err == nil && used > 0 {
				return levelInUse(used)
			}
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.perms.Invalidate()
	more := map[string]any{"issues": moved, "members": level.MemberCount}
	if target != nil {
		more["movedTo"] = target.Name
	}
	if in.Clear {
		more["cleared"] = true
	}
	s.auditLevel(r, audit.SecurityLevelDelete, projectID, level.Name, more)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// addSecurityLevelMember adds a holder to a level: everyone, a user, a group
// or a project role who sees the issues of the level, the project lead, or
// the reporter or assignee of each issue.
func (s *Server) addSecurityLevelMember(w http.ResponseWriter, r *http.Request) error {
	projectID, err := s.levelProject(r)
	if err != nil {
		return err
	}
	levelID, err := levelPathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		Holder string `json:"holder"`
		ID     *int64 `json:"id"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	ctx := r.Context()
	level, err := s.securityLevel(ctx, projectID, levelID)
	if err != nil {
		return err
	}
	var user, group, role any
	table := ""
	switch in.Holder {
	case perm.Everyone, perm.Lead, perm.Reporter, perm.Assignee:
	case perm.User:
		table = "users"
	case perm.Group:
		table = "user_groups"
	case perm.Role:
		table = "project_roles"
	default:
		return failf(http.StatusBadRequest, i18n.ProjectGranteeUnknown, in.Holder)
	}
	if table != "" {
		if in.ID == nil {
			return fail(http.StatusBadRequest, i18n.ProjectGranteeNotFound)
		}
		var n int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table+` WHERE id = ?`, *in.ID).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			return fail(http.StatusBadRequest, i18n.ProjectGranteeNotFound)
		}
		switch in.Holder {
		case perm.User:
			user = *in.ID
		case perm.Group:
			group = *in.ID
		case perm.Role:
			role = *in.ID
		}
	}
	// The level row is locked so that two submissions of the same holder
	// cannot both pass the check.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var locked int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM security_levels WHERE id = ? FOR UPDATE`, levelID).Scan(&locked); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errNotFound
		}
		return err
	}
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM security_level_members WHERE level_id = ? AND holder = ?
		AND user_id <=> ? AND group_id <=> ? AND role_id <=> ?`, levelID, in.Holder, user, group, role).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return fail(http.StatusConflict, i18n.SecurityMemberExists)
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO security_level_members (level_id, holder, user_id, group_id, role_id)
		VALUES (?, ?, ?, ?, ?)`, levelID, in.Holder, user, group, role)
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.perms.Invalidate()
	memberID, _ := res.LastInsertId()
	level, err = s.securityLevel(ctx, projectID, levelID)
	if err != nil {
		return err
	}
	member := ""
	for _, m := range level.Members {
		if m.ID == memberID {
			member = m.label()
		}
	}
	s.auditLevel(r, audit.SecurityLevelMemberAdd, projectID, level.Name, map[string]any{"holder": in.Holder, "member": member})
	return writeJSON(w, http.StatusCreated, level)
}

// removeSecurityLevelMember takes a holder off a level. A level left with no
// member hides its issues from everyone.
func (s *Server) removeSecurityLevelMember(w http.ResponseWriter, r *http.Request) error {
	projectID, err := s.levelProject(r)
	if err != nil {
		return err
	}
	levelID, err := levelPathID(r, "id")
	if err != nil {
		return err
	}
	memberID, err := levelPathID(r, "member")
	if err != nil {
		return err
	}
	ctx := r.Context()
	level, err := s.securityLevel(ctx, projectID, levelID)
	if err != nil {
		return err
	}
	var gone *levelMemberOut
	for i := range level.Members {
		if level.Members[i].ID == memberID {
			gone = &level.Members[i]
		}
	}
	if gone == nil {
		return errNotFound
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM security_level_members WHERE id = ? AND level_id = ?`,
		memberID, levelID); err != nil {
		return err
	}
	s.perms.Invalidate()
	s.auditLevel(r, audit.SecurityLevelMemberRemove, projectID, level.Name, map[string]any{"holder": gone.Holder, "member": gone.label()})
	w.WriteHeader(http.StatusNoContent)
	return nil
}
