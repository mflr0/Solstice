package api

import (
	"context"
	"database/sql"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"solstice/internal/i18n"
	"solstice/internal/issue"
	"solstice/internal/jql"
	"solstice/internal/perm"
	"solstice/internal/sqlq"
)

// Saved filters: a name on a query (JQL or simple syntax), private to their
// owner unless shared with everyone, a group, a project (or a group in it),
// or a person.

type filterShare struct {
	Kind  issue.ShareKind `json:"kind"`
	ID    int64           `json:"id,omitempty"`
	Group int64           `json:"group,omitempty"` // with kind project: only this group's members there
	Name  string          `json:"name,omitempty"`
}

type filterOut struct {
	ID          int64         `json:"id"`
	Name        string        `json:"name"`
	Description *string       `json:"description"`
	Query       string        `json:"query"`
	Owner       *userRef      `json:"owner"`
	Mine        bool          `json:"mine"`
	Favourite   bool          `json:"favourite"`
	CanEdit     bool          `json:"canEdit"`
	Shares      []filterShare `json:"shares"`
}

type filterRow struct {
	out    filterOut
	owner  sql.NullInt64
	shares []shareRow
}

type shareRow struct {
	kind                            issue.ShareKind
	group, project, user            sql.NullInt64
	groupName, projectKey, userName sql.NullString
}

// visible reports whether a share lets the subject see the filter.
func (sh shareRow) visible(sub *perm.Subject) bool {
	switch sh.kind {
	case issue.ShareEveryone:
		return true
	case issue.ShareGroup:
		return sub.InGroup(sh.group.Int64)
	case issue.ShareProject:
		if !sub.CanBrowseProject(sh.project.Int64) {
			return false
		}
		return !sh.group.Valid || sub.InGroupIn(sh.project.Int64, sh.group.Int64)
	case issue.ShareUser:
		return sh.user.Int64 == sub.UserID
	}
	return false
}

// out is the share as the API shows it.
func (sh shareRow) out() filterShare {
	fs := filterShare{Kind: sh.kind}
	switch sh.kind {
	case issue.ShareGroup:
		fs.ID, fs.Name = sh.group.Int64, sh.groupName.String
	case issue.ShareProject:
		fs.ID, fs.Name, fs.Group = sh.project.Int64, sh.projectKey.String, sh.group.Int64
		if sh.group.Valid {
			fs.Name += " · " + sh.groupName.String
		}
	case issue.ShareUser:
		fs.ID, fs.Name = sh.user.Int64, sh.userName.String
	}
	return fs
}

const shareQuery = `SELECT sh.filter_id, sh.kind, sh.group_id, sh.project_id, sh.user_id,
	g.name, p.pkey, u.display_name
	FROM filter_shares sh
	LEFT JOIN user_groups g ON g.id = sh.group_id
	LEFT JOIN projects p ON p.id = sh.project_id
	LEFT JOIN users u ON u.id = sh.user_id`

func scanShare(rows *sql.Rows) (filter int64, sh shareRow, err error) {
	err = rows.Scan(&filter, &sh.kind, &sh.group, &sh.project, &sh.user, &sh.groupName, &sh.projectKey, &sh.userName)
	return filter, sh, err
}

// sharesOfFilter returns a filter's shares, whoever asks.
func (s *Server) sharesOfFilter(ctx context.Context, filter int64) ([]filterShare, error) {
	out := []filterShare{}
	err := sqlq.Each(ctx, s.db, shareQuery+` WHERE sh.filter_id = ? ORDER BY sh.id`, func(rows *sql.Rows) error {
		_, sh, err := scanShare(rows)
		if err != nil {
			return err
		}
		out = append(out, sh.out())
		return nil
	}, filter)
	return out, err
}

// loadFilters returns the filters the user may see (all of them for id 0,
// or just that one).
func (s *Server) loadFilters(ctx context.Context, sub *perm.Subject, only int64) ([]*filterRow, error) {
	byID := map[int64]*filterRow{}
	var list []*filterRow
	q := `SELECT f.id, f.name, f.description, f.query, f.owner_id, fav.user_id IS NOT NULL
		FROM filters f LEFT JOIN filter_favourites fav ON fav.filter_id = f.id AND fav.user_id = ?`
	args := []any{sub.UserID}
	if only != 0 {
		q += ` WHERE f.id = ?`
		args = append(args, only)
	}
	if err := sqlq.Each(ctx, s.db, q, func(rows *sql.Rows) error {
		f := &filterRow{}
		if err := rows.Scan(&f.out.ID, &f.out.Name, &f.out.Description, &f.out.Query, &f.owner, &f.out.Favourite); err != nil {
			return err
		}
		byID[f.out.ID] = f
		list = append(list, f)
		return nil
	}, args...); err != nil {
		return nil, err
	}
	shares := shareQuery
	var shareArgs []any
	if only != 0 {
		shares += ` WHERE sh.filter_id = ?`
		shareArgs = append(shareArgs, only)
	}
	if err := sqlq.Each(ctx, s.db, shares, func(rows *sql.Rows) error {
		id, sh, err := scanShare(rows)
		if err != nil {
			return err
		}
		if f, ok := byID[id]; ok {
			f.shares = append(f.shares, sh)
		}
		return nil
	}, shareArgs...); err != nil {
		return nil, err
	}

	var out []*filterRow
	var owners []int64
	for _, f := range list {
		f.out.Mine = f.owner.Valid && f.owner.Int64 == sub.UserID
		ok := f.out.Mine
		for _, sh := range f.shares {
			ok = ok || sh.visible(sub)
		}
		if !ok {
			continue
		}
		// The owner manages a filter; administrators can tidy up filters
		// left behind by people who are gone.
		f.out.CanEdit = f.out.Mine || sub.Admin
		f.out.Shares = []filterShare{}
		for _, sh := range f.shares {
			f.out.Shares = append(f.out.Shares, sh.out())
		}
		owners = append(owners, f.owner.Int64)
		out = append(out, f)
	}
	users, err := s.users(ctx, owners...)
	if err != nil {
		return nil, err
	}
	for _, f := range out {
		f.out.Owner = refOf(users, f.owner)
	}
	return out, nil
}

func (s *Server) listFilters(w http.ResponseWriter, r *http.Request) error {
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	rows, err := s.loadFilters(r.Context(), sub, 0)
	if err != nil {
		return err
	}
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	out := []filterOut{}
	for _, f := range rows {
		if q == "" || strings.Contains(strings.ToLower(f.out.Name), q) {
			out = append(out, f.out)
		}
	}
	// Favourites first, then mine, then the rest, by name.
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].Favourite != out[b].Favourite {
			return out[a].Favourite
		}
		if out[a].Mine != out[b].Mine {
			return out[a].Mine
		}
		return strings.ToLower(out[a].Name) < strings.ToLower(out[b].Name)
	})
	return writeJSON(w, http.StatusOK, out)
}

func (s *Server) visibleFilter(r *http.Request, sub *perm.Subject) (*filterRow, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	// loadFilters reads id 0 as "every filter", so 0 must not get that far.
	if err != nil || id <= 0 {
		return nil, errNotFound
	}
	rows, err := s.loadFilters(r.Context(), sub, id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, errNotFound
	}
	return rows[0], nil
}

type filterInput struct {
	Name        *string        `json:"name"`
	Description *string        `json:"description"`
	Query       *string        `json:"query"`
	Shares      *[]filterShare `json:"shares"`
}

// checkFilter validates a filter's name, query and shares. One may only share
// with a group one belongs to, a project one sees, a group one is in there.
func (s *Server) checkFilter(ctx context.Context, sub *perm.Subject, in filterInput) error {
	if in.Name != nil {
		n := strings.TrimSpace(*in.Name)
		if n == "" || utf8.RuneCountInString(n) > sqlq.VarcharWidth {
			return fail(http.StatusBadRequest, i18n.FilterNameRequired)
		}
	}
	if in.Query != nil && jql.LooksLike(*in.Query) {
		if _, _, _, err := s.compileJQL(ctx, sub, *in.Query); err != nil {
			return err
		}
	}
	if in.Shares == nil {
		return nil
	}
	for _, sh := range *in.Shares {
		switch sh.Kind {
		case issue.ShareEveryone:
		case issue.ShareGroup:
			if !sub.InGroup(sh.ID) && !sub.Admin {
				return fail(http.StatusBadRequest, i18n.FilterShareGroupNotMember)
			}
			if err := s.checkGroupExists(ctx, sh.ID); err != nil {
				return err
			}
		case issue.ShareProject:
			if !sub.CanBrowseProject(sh.ID) {
				return fail(http.StatusBadRequest, i18n.ProjectNotFound)
			}
			if sh.Group != 0 {
				if !sub.InGroupIn(sh.ID, sh.Group) && !sub.Admin {
					return fail(http.StatusBadRequest, i18n.FilterShareProjectGroupNotMember)
				}
				if err := s.checkGroupExists(ctx, sh.Group); err != nil {
					return err
				}
			}
		case issue.ShareUser:
			var n int
			if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE id = ? AND active`, sh.ID).Scan(&n); err != nil {
				return err
			}
			if n == 0 {
				return fail(http.StatusBadRequest, i18n.FilterSharePersonNotFound)
			}
		default:
			return failf(http.StatusBadRequest, i18n.FilterShareKindUnknown, sh.Kind)
		}
	}
	return nil
}

// checkGroupExists keeps an administrator, who may name any group, from
// reaching the foreign key with one that doesn't exist.
func (s *Server) checkGroupExists(ctx context.Context, group int64) error {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_groups WHERE id = ?`, group).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return fail(http.StatusBadRequest, i18n.GroupNotFound)
	}
	return nil
}

// shareableGroups lists the groups checkFilter lets the user share with: their
// own, in a project when one is named; for an administrator, every group.
func (s *Server) shareableGroups(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	belongs := sub.InGroup
	if v := r.URL.Query().Get("project"); v != "" {
		project, err := strconv.ParseInt(v, 10, 64)
		if err != nil || !sub.CanBrowseProject(project) {
			return fail(http.StatusBadRequest, i18n.ProjectNotFound)
		}
		belongs = func(group int64) bool { return sub.InGroupIn(project, group) }
	}
	out := []idName{}
	err = sqlq.Each(ctx, s.db, `SELECT id, name FROM user_groups ORDER BY name, id`, func(rows *sql.Rows) error {
		var g idName
		if err := rows.Scan(&g.ID, &g.Name); err != nil {
			return err
		}
		if sub.Admin || belongs(g.ID) {
			out = append(out, g)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

func writeShares(ctx context.Context, tx *sql.Tx, filter int64, shares []filterShare) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM filter_shares WHERE filter_id = ?`, filter); err != nil {
		return err
	}
	for _, sh := range shares {
		var group, project, user any
		switch sh.Kind {
		case issue.ShareGroup:
			group = sh.ID
		case issue.ShareProject:
			project = sh.ID
			if sh.Group != 0 {
				group = sh.Group
			}
		case issue.ShareUser:
			user = sh.ID
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO filter_shares (filter_id, kind, group_id, project_id, user_id)
			VALUES (?, ?, ?, ?, ?)`, filter, sh.Kind, group, project, user); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) createFilter(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	var in filterInput
	if err := readJSON(r, &in); err != nil {
		return err
	}
	if in.Name == nil || in.Query == nil {
		return fail(http.StatusBadRequest, i18n.FilterIncomplete)
	}
	if err := s.checkFilter(ctx, sub, in); err != nil {
		return err
	}
	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var desc any
	if in.Description != nil {
		desc = sqlq.NullIfBlank(*in.Description)
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO filters (owner_id, name, description, query, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`, sub.UserID, strings.TrimSpace(*in.Name), desc, *in.Query, now, now)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	if in.Shares != nil {
		if err := writeShares(ctx, tx, id, *in.Shares); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if in.Shares != nil {
		s.auditFilterShares(r, id, strings.TrimSpace(*in.Name), nil, *in.Shares)
	}
	rows, err := s.loadFilters(ctx, sub, id)
	if err != nil || len(rows) == 0 {
		return err
	}
	return writeJSON(w, http.StatusCreated, rows[0].out)
}

func (s *Server) updateFilter(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	f, err := s.visibleFilter(r, sub)
	if err != nil {
		return err
	}
	if !f.out.CanEdit {
		return errForbidden
	}
	var in filterInput
	if err := readJSON(r, &in); err != nil {
		return err
	}
	if err := s.checkFilter(ctx, sub, in); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	sets := []string{"updated_at = ?"}
	args := []any{time.Now().UTC()}
	if in.Name != nil {
		sets, args = append(sets, "name = ?"), append(args, strings.TrimSpace(*in.Name))
	}
	if in.Description != nil {
		sets, args = append(sets, "description = ?"), append(args, sqlq.NullIfBlank(*in.Description))
	}
	if in.Query != nil {
		sets, args = append(sets, "query = ?"), append(args, *in.Query)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE filters SET `+strings.Join(sets, ", ")+` WHERE id = ?`,
		append(args, f.out.ID)...); err != nil {
		return err
	}
	if in.Shares != nil {
		if err := writeShares(ctx, tx, f.out.ID, *in.Shares); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if in.Shares != nil {
		s.auditFilterShares(r, f.out.ID, f.out.Name, f.out.Shares, *in.Shares)
	}
	rows, err := s.loadFilters(ctx, sub, f.out.ID)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		// The owner always sees their filter; an administrator may have
		// just unshared someone else's.
		w.WriteHeader(http.StatusNoContent)
		return nil
	}
	return writeJSON(w, http.StatusOK, rows[0].out)
}

func (s *Server) deleteFilter(w http.ResponseWriter, r *http.Request) error {
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	f, err := s.visibleFilter(r, sub)
	if err != nil {
		return err
	}
	if !f.out.CanEdit {
		return errForbidden
	}
	if _, err := s.db.ExecContext(r.Context(), `DELETE FROM filters WHERE id = ?`, f.out.ID); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) setFavourite(on bool) handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		sub, err := s.subject(r)
		if err != nil {
			return err
		}
		f, err := s.visibleFilter(r, sub)
		if err != nil {
			return err
		}
		if on {
			_, err = s.db.ExecContext(r.Context(), `INSERT IGNORE INTO filter_favourites (filter_id, user_id) VALUES (?, ?)`,
				f.out.ID, sub.UserID)
		} else {
			_, err = s.db.ExecContext(r.Context(), `DELETE FROM filter_favourites WHERE filter_id = ? AND user_id = ?`,
				f.out.ID, sub.UserID)
		}
		if err != nil {
			return err
		}
		w.WriteHeader(http.StatusNoContent)
		return nil
	}
}
