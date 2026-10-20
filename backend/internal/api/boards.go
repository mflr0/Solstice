package api

// Boards show a saved filter's issues in columns of statuses (Kanban), or
// the active sprint's (Scrum) with a backlog of what comes next. Moving a
// card to another column moves the issue along its workflow; dragging it up
// or down changes its rank, the board order shared by all boards.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"slices"

	"solstice/internal/audit"
	"solstice/internal/events"
	"solstice/internal/i18n"
	"solstice/internal/issue"
	"solstice/internal/jql"
	"solstice/internal/perm"
	"solstice/internal/search"
	"solstice/internal/sqlq"
	"strconv"
	"strings"
	"time"
)

type sprintRef struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"` // future, active, closed
	Board int64  `json:"board,omitempty"`
}

type sprintOut struct {
	ID        int64      `json:"id"`
	Name      string     `json:"name"`
	State     string     `json:"state"`
	Goal      *string    `json:"goal"`
	Start     *time.Time `json:"start"`
	End       *time.Time `json:"end"`
	Completed *time.Time `json:"completed"`
}

type boardRow struct {
	ID        int64
	Name      string
	Kind      string
	FilterID  int64
	Query     string
	SubQuery  sql.NullString
	OwnerID   sql.NullInt64
	Swimlanes string
	Estimate  sql.NullString
	Cutoff    sql.NullString
}

const boardColumns = `b.id, b.name, b.kind, b.filter_id, f.query, b.sub_query, b.owner_id, b.swimlanes, b.estimate,
	b.done_cutoff`

func scanBoard(sc scanner) (*boardRow, error) {
	var b boardRow
	err := sc.Scan(&b.ID, &b.Name, &b.Kind, &b.FilterID, &b.Query, &b.SubQuery, &b.OwnerID, &b.Swimlanes, &b.Estimate,
		&b.Cutoff)
	return &b, err
}

// visibleFilterIDs is the set of saved filters the user sees; a board is
// visible exactly when its filter is, as in Jira.
func (s *Server) visibleFilterIDs(ctx context.Context, sub *perm.Subject) (map[int64]bool, error) {
	rows, err := s.loadFilters(ctx, sub, 0)
	if err != nil {
		return nil, err
	}
	out := map[int64]bool{}
	for _, f := range rows {
		out[f.out.ID] = true
	}
	return out, nil
}

func (s *Server) board(r *http.Request, sub *perm.Subject) (*boardRow, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return nil, errNotFound
	}
	return s.boardByID(r.Context(), sub, id)
}

// boardByID loads a board the user may see.
func (s *Server) boardByID(ctx context.Context, sub *perm.Subject, id int64) (*boardRow, error) {
	b, err := scanBoard(s.db.QueryRowContext(ctx, `SELECT `+boardColumns+` FROM boards b
		JOIN filters f ON f.id = b.filter_id WHERE b.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}
	filters, err := s.loadFilters(ctx, sub, b.FilterID)
	if err != nil {
		return nil, err
	}
	if len(filters) == 0 {
		return nil, errNotFound
	}
	return b, nil
}

// boardProjects returns the projects a board belongs to.
func (s *Server) boardProjects(ctx context.Context, board int64) ([]projectBrief, error) {
	var out []projectBrief
	err := sqlq.Each(ctx, s.db, `SELECT p.id, p.pkey, p.name FROM board_projects bp JOIN projects p ON p.id = bp.project_id
		WHERE bp.board_id = ? ORDER BY p.pkey`, func(rows *sql.Rows) error {
		var p projectBrief
		if err := rows.Scan(&p.ID, &p.Key, &p.Name); err != nil {
			return err
		}
		out = append(out, p)
		return nil
	}, board)
	return out, err
}

// canManage tells whether the user may run the board's sprints and change
// it: its owner, an administrator, or someone who administers one of its
// projects.
func (s *Server) canManage(ctx context.Context, sub *perm.Subject, b *boardRow) (bool, error) {
	if sub.Admin || (b.OwnerID.Valid && b.OwnerID.Int64 == sub.UserID) {
		return true, nil
	}
	projects, err := s.boardProjects(ctx, b.ID)
	if err != nil {
		return false, err
	}
	for _, p := range projects {
		if sub.Can(perm.Admin, p.ID, nil) {
			return true, nil
		}
	}
	return false, nil
}

type boardSummary struct {
	ID       int64          `json:"id"`
	Name     string         `json:"name"`
	Kind     string         `json:"kind"`
	Projects []projectBrief `json:"projects"`
	Owner    *userRef       `json:"owner"`
	Active   *sprintRef     `json:"activeSprint"`
}

func (s *Server) listBoards(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	visible, err := s.visibleFilterIDs(ctx, sub)
	if err != nil {
		return err
	}
	project := strings.ToUpper(r.URL.Query().Get("project"))
	var list []*boardSummary
	owners := map[int64]sql.NullInt64{}
	var ownerIDs []int64
	err = sqlq.Each(ctx, s.db, `SELECT b.id, b.name, b.kind, b.filter_id, b.owner_id FROM boards b ORDER BY b.name`,
		func(rows *sql.Rows) error {
			var b boardSummary
			var filter int64
			var owner sql.NullInt64
			if err := rows.Scan(&b.ID, &b.Name, &b.Kind, &filter, &owner); err != nil {
				return err
			}
			if !visible[filter] {
				return nil
			}
			owners[b.ID] = owner
			ownerIDs = append(ownerIDs, owner.Int64)
			b.Projects = []projectBrief{}
			list = append(list, &b)
			return nil
		})
	if err != nil {
		return err
	}
	byID := map[int64]*boardSummary{}
	for _, b := range list {
		byID[b.ID] = b
	}
	if err := sqlq.Each(ctx, s.db, `SELECT bp.board_id, p.id, p.pkey, p.name FROM board_projects bp
		JOIN projects p ON p.id = bp.project_id ORDER BY p.pkey`, func(rows *sql.Rows) error {
		var board int64
		var p projectBrief
		if err := rows.Scan(&board, &p.ID, &p.Key, &p.Name); err != nil {
			return err
		}
		if b, ok := byID[board]; ok && sub.CanBrowseProject(p.ID) {
			b.Projects = append(b.Projects, p)
		}
		return nil
	}); err != nil {
		return err
	}
	if err := sqlq.Each(ctx, s.db, `SELECT id, name, state, board_id FROM sprints WHERE state = 'active'
		ORDER BY position`, func(rows *sql.Rows) error {
		var sp sprintRef
		var board sql.NullInt64
		if err := rows.Scan(&sp.ID, &sp.Name, &sp.State, &board); err != nil {
			return err
		}
		if b, ok := byID[board.Int64]; ok && b.Active == nil {
			sp.Board = board.Int64
			b.Active = &sp
		}
		return nil
	}); err != nil {
		return err
	}
	users, err := s.users(ctx, ownerIDs...)
	if err != nil {
		return err
	}
	out := []*boardSummary{}
	for _, b := range list {
		b.Owner = refOf(users, owners[b.ID])
		if project != "" && !slices.ContainsFunc(b.Projects, func(p projectBrief) bool { return p.Key == project }) {
			continue
		}
		out = append(out, b)
	}
	return writeJSON(w, http.StatusOK, out)
}

type columnOut struct {
	ID       int64       `json:"id"`
	Name     string      `json:"name"`
	Min      *int        `json:"min"`
	Max      *int        `json:"max"`
	Statuses []statusRef `json:"statuses"`
}

type namedQuery struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Query       *string `json:"query"`
	Description *string `json:"description,omitempty"`
	Default     bool    `json:"default,omitempty"`
}

func (s *Server) boardColumns(ctx context.Context, cat *catalog, board int64) ([]columnOut, error) {
	var cols []columnOut
	index := map[int64]int{}
	if err := sqlq.Each(ctx, s.db, `SELECT id, name, min_issues, max_issues FROM board_columns WHERE board_id = ?
		ORDER BY position, id`, func(rows *sql.Rows) error {
		var c columnOut
		var lo, hi sql.NullInt64
		if err := rows.Scan(&c.ID, &c.Name, &lo, &hi); err != nil {
			return err
		}
		if lo.Valid {
			v := int(lo.Int64)
			c.Min = &v
		}
		if hi.Valid {
			v := int(hi.Int64)
			c.Max = &v
		}
		c.Statuses = []statusRef{}
		index[c.ID] = len(cols)
		cols = append(cols, c)
		return nil
	}, board); err != nil {
		return nil, err
	}
	err := sqlq.Each(ctx, s.db, `SELECT cs.column_id, cs.status_id FROM board_column_statuses cs
		JOIN board_columns c ON c.id = cs.column_id WHERE c.board_id = ? ORDER BY cs.position`, func(rows *sql.Rows) error {
		var col int64
		var status string
		if err := rows.Scan(&col, &status); err != nil {
			return err
		}
		if i, ok := index[col]; ok {
			cols[i].Statuses = append(cols[i].Statuses, cat.statusOf(status))
		}
		return nil
	}, board)
	return cols, err
}

func (s *Server) getBoard(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	b, err := s.board(r, sub)
	if err != nil {
		return err
	}
	cat, err := s.catalogue(ctx)
	if err != nil {
		return err
	}
	columns, err := s.boardColumns(ctx, cat, b.ID)
	if err != nil {
		return err
	}
	quick := []namedQuery{}
	if err := sqlq.Each(ctx, s.db, `SELECT id, name, query, description FROM board_quick_filters WHERE board_id = ?
		ORDER BY position, id`, func(rows *sql.Rows) error {
		var q namedQuery
		var query, desc sql.NullString
		if err := rows.Scan(&q.ID, &q.Name, &query, &desc); err != nil {
			return err
		}
		q.Query, q.Description = sqlq.String(query), sqlq.String(desc)
		quick = append(quick, q)
		return nil
	}, b.ID); err != nil {
		return err
	}
	lanes := []namedQuery{}
	if err := sqlq.Each(ctx, s.db, `SELECT id, name, query, is_default FROM board_swimlanes WHERE board_id = ?
		ORDER BY position, id`, func(rows *sql.Rows) error {
		var q namedQuery
		var query sql.NullString
		if err := rows.Scan(&q.ID, &q.Name, &query, &q.Default); err != nil {
			return err
		}
		q.Query = sqlq.String(query)
		lanes = append(lanes, q)
		return nil
	}, b.ID); err != nil {
		return err
	}
	sprints, err := s.boardSprints(ctx, b.ID, "active", "future")
	if err != nil {
		return err
	}
	projects, err := s.boardProjects(ctx, b.ID)
	if err != nil {
		return err
	}
	if projects == nil {
		projects = []projectBrief{}
	}
	manage, err := s.canManage(ctx, sub, b)
	if err != nil {
		return err
	}
	// Number fields a board may estimate with (story points…).
	estimateFields := []namedRef{}
	if err := sqlq.Each(ctx, s.db, `SELECT id, name FROM custom_fields WHERE type_key LIKE '%:float' ORDER BY name`,
		func(rows *sql.Rows) error {
			var id int64
			var f namedRef
			if err := rows.Scan(&id, &f.Name); err != nil {
				return err
			}
			f.ID = "customfield_" + strconv.FormatInt(id, 10)
			estimateFields = append(estimateFields, f)
			return nil
		}); err != nil {
		return err
	}
	estimate := ""
	if b.Estimate.Valid {
		estimate = "points"
		if b.Estimate.String == "timeoriginalestimate" {
			estimate = "time"
		}
	}
	return writeJSON(w, http.StatusOK, map[string]any{
		"board": map[string]any{
			"id": b.ID, "name": b.Name, "kind": b.Kind, "filterId": b.FilterID, "query": b.Query,
			"subQuery": sqlq.String(b.SubQuery), "swimlanes": b.Swimlanes, "estimate": estimate,
			"estimateField": sqlq.String(b.Estimate), "doneCutoff": sqlq.String(b.Cutoff),
		},
		"estimateFields": estimateFields,
		"projects":       projects, "columns": columns, "quickFilters": quick, "swimlanes": lanes, "sprints": sprints,
		"can": map[string]bool{"manage": manage},
	})
}

// backlogCond narrows a board's issues to its backlog: sub-tasks travel with
// their parent, done issues have left it.
const backlogCond = ` AND i.parent_id IS NULL AND i.status_id NOT IN (SELECT id FROM statuses WHERE category = 'done')`

const sprintColumns = `id, name, state, goal, start_at, end_at, completed_at`

func scanSprint(sc scanner) (sprintOut, error) {
	var sp sprintOut
	var goal sql.NullString
	var start, end, done sql.NullTime
	if err := sc.Scan(&sp.ID, &sp.Name, &sp.State, &goal, &start, &end, &done); err != nil {
		return sp, err
	}
	sp.Goal = sqlq.String(goal)
	sp.Start, sp.End, sp.Completed = sqlq.Time(start), sqlq.Time(end), sqlq.Time(done)
	return sp, nil
}

// boardSprints lists the sprints of a board in the given states, or in any
// state for none, the active one first.
func (s *Server) boardSprints(ctx context.Context, board int64, states ...string) ([]sprintOut, error) {
	q := `SELECT ` + sprintColumns + ` FROM sprints WHERE board_id = ?`
	args := []any{board}
	if len(states) > 0 {
		q += ` AND state IN (` + sqlq.Marks(len(states)) + `)`
		for _, st := range states {
			args = append(args, st)
		}
	}
	out := []sprintOut{}
	err := sqlq.Each(ctx, s.db, q+` ORDER BY state = 'active' DESC, position, id`, func(rows *sql.Rows) error {
		sp, err := scanSprint(rows)
		if err != nil {
			return err
		}
		out = append(out, sp)
		return nil
	}, args...)
	return out, err
}

// sprintByID loads a sprint of the board.
func (s *Server) sprintByID(ctx context.Context, board, id int64) (*sprintOut, error) {
	sp, err := scanSprint(s.db.QueryRowContext(ctx, `SELECT `+sprintColumns+` FROM sprints WHERE board_id = ? AND id = ?`,
		board, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}
	return &sp, nil
}

// queryCondition compiles a saved query (JQL or the simple syntax) into a
// condition on issues, ignoring its ORDER BY: boards order by rank.
func (s *Server) queryCondition(ctx context.Context, sub *perm.Subject, q string) (string, []any, error) {
	if strings.TrimSpace(q) == "" {
		return "TRUE", nil, nil
	}
	if jql.LooksLike(q) {
		cond, args, _, err := s.compileJQL(ctx, sub, q)
		return cond, args, err
	}
	parsed := search.Parse(q)
	cond, args := parsed.SQL(sub.UserID)
	return cond, args, nil
}

var cutoffPattern = regexp.MustCompile(`^(\d+)\s*([dwm])$`)

// boardWhere builds the condition selecting a board's issues: the filter,
// the Kanban sub-filter and done cutoff, the chosen quick filters, statuses
// mapped to a column, and what the user may see.
func (s *Server) boardWhere(ctx context.Context, sub *perm.Subject, b *boardRow, quick []int64) (string, []any, error) {
	vis, args := sub.VisibleSQL("i")
	conds := []string{vis}
	add := func(q string) error {
		c, a, err := s.queryCondition(ctx, sub, q)
		if err != nil {
			return err
		}
		conds = append(conds, "("+c+")")
		args = append(args, a...)
		return nil
	}
	if err := add(b.Query); err != nil {
		return "", nil, err
	}
	if b.Kind == "kanban" && b.SubQuery.Valid {
		if err := add(b.SubQuery.String); err != nil {
			return "", nil, err
		}
	}
	for _, id := range quick {
		var q string
		err := s.db.QueryRowContext(ctx, `SELECT query FROM board_quick_filters WHERE id = ? AND board_id = ?`, id, b.ID).Scan(&q)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return "", nil, err
		}
		if err := add(q); err != nil {
			return "", nil, err
		}
	}
	conds = append(conds, `i.status_id IN (SELECT cs.status_id FROM board_column_statuses cs
		JOIN board_columns c ON c.id = cs.column_id WHERE c.board_id = ?)`)
	args = append(args, b.ID)
	if b.Kind == "kanban" && b.Cutoff.Valid {
		if m := cutoffPattern.FindStringSubmatch(b.Cutoff.String); m != nil {
			n, _ := strconv.Atoi(m[1])
			days := map[string]int{"d": 1, "w": 7, "m": 30}[m[2]] * n
			// Done issues leave the board once they have been done a while.
			conds = append(conds, `NOT (i.status_id IN (SELECT cs.status_id FROM board_column_statuses cs
				JOIN board_columns c ON c.id = cs.column_id
				WHERE c.board_id = ? AND c.position = (SELECT MAX(position) FROM board_columns WHERE board_id = ?))
				AND COALESCE(i.resolved_at, i.updated_at) < ?)`)
			args = append(args, b.ID, b.ID, time.Now().UTC().AddDate(0, 0, -days))
		}
	}
	return strings.Join(conds, " AND "), args, nil
}

type card struct {
	issueSummary
	ID       int64       `json:"-"`
	Rank     string      `json:"-"`
	Epic     *issueBrief `json:"epic"`
	Parent   *issueBrief `json:"parent"`
	Estimate *float64    `json:"estimate"`
	Column   int64       `json:"column"`
	Lane     *int64      `json:"lane,omitempty"`
	Sprint   *int64      `json:"sprint,omitempty"`
	row      *issueRow
}

const maxCards = 1000

// loadCards returns the issues matching where, in board order.
func (s *Server) loadCards(ctx context.Context, b *boardRow, where string, args []any, cat *catalog) ([]*card, bool, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+issueColumns+` FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE `+where+` ORDER BY i.rank_key IS NULL, i.rank_key, i.id LIMIT ?`, append(args, maxCards+1)...)
	if err != nil {
		return nil, false, err
	}
	var list []*card
	var userIDs, related []int64
	for rows.Next() {
		iss, err := scanIssue(rows)
		if err != nil {
			rows.Close()
			return nil, false, err
		}
		list = append(list, &card{ID: iss.ID, Rank: iss.Rank.String, row: iss})
		userIDs = append(userIDs, iss.AssigneeID.Int64, iss.ReporterID.Int64)
		related = append(related, iss.EpicID.Int64, iss.ParentID.Int64)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	truncated := len(list) > maxCards
	if truncated {
		list = list[:maxCards]
	}
	users, err := s.users(ctx, userIDs...)
	if err != nil {
		return nil, false, err
	}
	briefs, err := s.briefs(ctx, cat, related)
	if err != nil {
		return nil, false, err
	}
	statusColumn := map[string]int64{}
	if err := sqlq.Each(ctx, s.db, `SELECT cs.status_id, cs.column_id FROM board_column_statuses cs
		JOIN board_columns c ON c.id = cs.column_id WHERE c.board_id = ?`, func(rows *sql.Rows) error {
		var st string
		var col int64
		if err := rows.Scan(&st, &col); err != nil {
			return err
		}
		statusColumn[st] = col
		return nil
	}, b.ID); err != nil {
		return nil, false, err
	}
	estimates, err := s.estimates(ctx, b, list)
	if err != nil {
		return nil, false, err
	}
	for _, c := range list {
		c.issueSummary = summarize(c.row, cat, users)
		c.Column = statusColumn[c.row.StatusID]
		if c.row.EpicID.Valid {
			c.Epic = briefs[c.row.EpicID.Int64]
		}
		if c.row.ParentID.Valid {
			c.Parent = briefs[c.row.ParentID.Int64]
		}
		if v, ok := estimates[c.ID]; ok {
			c.Estimate = &v
		}
	}
	return list, truncated, nil
}

// briefs loads short descriptions of related issues (epics, parents). Only
// issues the caller already sees are linked from cards, and epics and
// parents share their project, so no further check is needed for display.
func (s *Server) briefs(ctx context.Context, cat *catalog, ids []int64) (map[int64]*issueBrief, error) {
	out := map[int64]*issueBrief{}
	var args []any
	seen := map[int64]bool{}
	for _, id := range ids {
		if id != 0 && !seen[id] {
			seen[id] = true
			args = append(args, id)
		}
	}
	for chunk := range slices.Chunk(args, 500) {
		err := sqlq.Each(ctx, s.db, `SELECT i.id, p.pkey, i.num, i.summary, i.issue_type_id, i.status_id FROM issues i
			JOIN projects p ON p.id = i.project_id WHERE i.id IN (`+sqlq.Marks(len(chunk))+`)`, func(rows *sql.Rows) error {
			var id, num int64
			var key, summary, typeID, statusID string
			if err := rows.Scan(&id, &key, &num, &summary, &typeID, &statusID); err != nil {
				return err
			}
			out[id] = &issueBrief{Key: key + "-" + strconv.FormatInt(num, 10), Summary: summary,
				Type: cat.typeOf(typeID), Status: cat.statusOf(statusID)}
			return nil
		}, chunk...)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

var customFieldRef = regexp.MustCompile(`^customfield_(\d+)$`)

// estimates reads each card's estimate: story points (a number field) or the
// original time estimate in hours.
func (s *Server) estimates(ctx context.Context, b *boardRow, cards []*card) (map[int64]float64, error) {
	out := map[int64]float64{}
	if !b.Estimate.Valid || len(cards) == 0 {
		return out, nil
	}
	if b.Estimate.String == "timeoriginalestimate" {
		for _, c := range cards {
			if c.row.Original.Valid {
				out[c.ID] = float64(c.row.Original.Int64) / 3600
			}
		}
		return out, nil
	}
	m := customFieldRef.FindStringSubmatch(b.Estimate.String)
	if m == nil {
		return out, nil
	}
	field, _ := strconv.ParseInt(m[1], 10, 64)
	ids := make([]any, len(cards))
	for i, c := range cards {
		ids[i] = c.ID
	}
	for chunk := range slices.Chunk(ids, 500) {
		err := sqlq.Each(ctx, s.db, `SELECT issue_id, number_value FROM custom_field_values
			WHERE field_id = ? AND number_value IS NOT NULL AND issue_id IN (`+sqlq.Marks(len(chunk))+`)`,
			func(rows *sql.Rows) error {
				var id int64
				var v float64
				if err := rows.Scan(&id, &v); err != nil {
					return err
				}
				out[id] = v
				return nil
			}, append([]any{field}, chunk...)...)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// assignLanes puts each card in the first custom swimlane whose query
// matches it, or the default lane.
func (s *Server) assignLanes(ctx context.Context, sub *perm.Subject, b *boardRow, cards []*card) error {
	if b.Swimlanes != "custom" || len(cards) == 0 {
		return nil
	}
	type lane struct {
		id    int64
		query sql.NullString
		def   bool
	}
	var lanes []lane
	if err := sqlq.Each(ctx, s.db, `SELECT id, query, is_default FROM board_swimlanes WHERE board_id = ?
		ORDER BY position, id`, func(rows *sql.Rows) error {
		var l lane
		if err := rows.Scan(&l.id, &l.query, &l.def); err != nil {
			return err
		}
		lanes = append(lanes, l)
		return nil
	}, b.ID); err != nil {
		return err
	}
	ids := make([]any, len(cards))
	byID := map[int64]*card{}
	for i, c := range cards {
		ids[i] = c.ID
		byID[c.ID] = c
	}
	var fallback *int64
	for _, l := range lanes {
		if l.def || !l.query.Valid {
			id := l.id
			fallback = &id
			continue
		}
		cond, args, err := s.queryCondition(ctx, sub, l.query.String)
		if err != nil {
			continue // a lane whose query no longer compiles keeps no card
		}
		rows, err := s.db.QueryContext(ctx, `SELECT i.id FROM issues i JOIN projects p ON p.id = i.project_id
			WHERE i.id IN (`+sqlq.Marks(len(ids))+`) AND (`+cond+`)`, append(slices.Clone(ids), args...)...)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			if c := byID[id]; c != nil && c.Lane == nil {
				lid := l.id
				c.Lane = &lid
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
	}
	for _, c := range cards {
		if c.Lane == nil {
			c.Lane = fallback
		}
	}
	return nil
}

func parseIDs(s string) []int64 {
	var out []int64
	for part := range strings.SplitSeq(s, ",") {
		if id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64); err == nil {
			out = append(out, id)
		}
	}
	return out
}

// boardCards lists the cards of the board: on a Scrum board, those of the
// active sprint (or ?sprint=id).
func (s *Server) boardCards(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	b, err := s.board(r, sub)
	if err != nil {
		return err
	}
	cat, err := s.catalogue(ctx)
	if err != nil {
		return err
	}
	where, args, err := s.boardWhere(ctx, sub, b, parseIDs(r.URL.Query().Get("quick")))
	if err != nil {
		return err
	}
	var sprint *sprintOut
	if b.Kind == "scrum" {
		if id, err := strconv.ParseInt(r.URL.Query().Get("sprint"), 10, 64); err == nil {
			if sprint, err = s.sprintByID(ctx, b.ID, id); err != nil && !errors.Is(err, errNotFound) {
				return err
			}
		} else {
			active, err := s.boardSprints(ctx, b.ID, "active")
			if err != nil {
				return err
			}
			if len(active) > 0 {
				sprint = &active[0]
			}
		}
		if sprint == nil {
			return writeJSON(w, http.StatusOK, map[string]any{"cards": []*card{}, "sprint": nil, "truncated": false})
		}
		where += " AND EXISTS (SELECT 1 FROM sprint_issues si WHERE si.issue_id = i.id AND si.sprint_id = ?)"
		args = append(args, sprint.ID)
	}
	cards, truncated, err := s.loadCards(ctx, b, where, args, cat)
	if err != nil {
		return err
	}
	if err := s.assignLanes(ctx, sub, b, cards); err != nil {
		return err
	}
	if cards == nil {
		cards = []*card{}
	}
	return writeJSON(w, http.StatusOK, map[string]any{"cards": cards, "sprint": sprint, "truncated": truncated})
}

// backlog lists, for a Scrum board, the open sprints with their issues and
// then the backlog: issues not yet in an open sprint and not done.
func (s *Server) backlog(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	b, err := s.board(r, sub)
	if err != nil {
		return err
	}
	cat, err := s.catalogue(ctx)
	if err != nil {
		return err
	}
	where, args, err := s.boardWhere(ctx, sub, b, parseIDs(r.URL.Query().Get("quick")))
	if err != nil {
		return err
	}
	where += backlogCond
	cards, truncated, err := s.loadCards(ctx, b, where, args, cat)
	if err != nil {
		return err
	}
	sprints, err := s.boardSprints(ctx, b.ID, "active", "future")
	if err != nil {
		return err
	}
	inSprint := map[int64]int64{}
	if len(cards) > 0 && len(sprints) > 0 {
		ids := make([]any, 0, len(cards))
		for _, c := range cards {
			ids = append(ids, c.ID)
		}
		if err := sqlq.Each(ctx, s.db, `SELECT si.issue_id, si.sprint_id FROM sprint_issues si JOIN sprints s ON s.id = si.sprint_id
			WHERE s.board_id = ? AND s.state <> 'closed' AND si.issue_id IN (`+sqlq.Marks(len(ids))+`)`, func(rows *sql.Rows) error {
			var issue, sprint int64
			if err := rows.Scan(&issue, &sprint); err != nil {
				return err
			}
			inSprint[issue] = sprint
			return nil
		}, append([]any{b.ID}, ids...)...); err != nil {
			return err
		}
	}
	type section struct {
		Sprint sprintOut `json:"sprint"`
		Cards  []*card   `json:"cards"`
	}
	sections := make([]section, len(sprints))
	index := map[int64]int{}
	for i, sp := range sprints {
		sections[i] = section{Sprint: sp, Cards: []*card{}}
		index[sp.ID] = i
	}
	rest := []*card{}
	for _, c := range cards {
		if id, ok := inSprint[c.ID]; ok {
			sid := id
			c.Sprint = &sid
			sections[index[id]].Cards = append(sections[index[id]].Cards, c)
			continue
		}
		rest = append(rest, c)
	}
	return writeJSON(w, http.StatusOK, map[string]any{"sprints": sections, "backlog": rest, "truncated": truncated})
}

// move handles a card dropped somewhere: another column (a workflow
// transition), another place in the order (rank), another sprint.
func (s *Server) moveCard(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	b, err := s.board(r, sub)
	if err != nil {
		return err
	}
	var in struct {
		Issue  string  `json:"issue"`
		Column *int64  `json:"column"`
		After  *string `json:"after"`  // the card now just above, if any
		Before *string `json:"before"` // the card now just below, if any
		Sprint *int64  `json:"sprint"` // 0: back to the backlog
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	iss, err := s.findIssue(ctx, in.Issue)
	if err != nil || !sub.CanSee(iss.perm()) {
		return errNotFound
	}
	pi := iss.perm()
	// Refuse before the column change commits, not halfway through the move.
	if (in.After != nil || in.Before != nil || in.Sprint != nil) && !sub.Can(perm.Edit, iss.ProjectID, &pi) {
		return errForbidden
	}

	if in.Column != nil {
		target, err := s.columnTarget(ctx, b, iss, *in.Column)
		if err != nil {
			return err
		}
		if target != "" && target != iss.StatusID {
			raw, _ := json.Marshal(target)
			if err := s.applyUpdate(ctx, sub, iss, fields{"status": raw}); err != nil {
				return err
			}
		}
	}

	if in.After != nil || in.Before != nil {
		var lower, upper string
		if in.After != nil {
			if a, err := s.findIssue(ctx, *in.After); err == nil {
				lower = a.Rank.String
			}
		}
		if in.Before != nil {
			if bf, err := s.findIssue(ctx, *in.Before); err == nil {
				upper = bf.Rank.String
			}
		}
		if lower != "" && upper != "" && lower >= upper {
			upper = "" // neighbours out of order (unranked issues): just go after the one above
		}
		if lower == "" && upper == "" {
			if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(rank_key), '') FROM issues`).Scan(&lower); err != nil {
				return err
			}
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE issues SET rank_key = ? WHERE id = ?`,
			rankBetween(lower, upper), iss.ID); err != nil {
			return err
		}
	}

	if in.Sprint != nil {
		if err := s.setSprint(ctx, sub, b, iss, *in.Sprint, false); err != nil {
			return err
		}
	}
	s.publish(iss, "issue", sub.UserID)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// columnTarget picks the status an issue takes when dropped in a column: the
// column's first status the workflow lets it reach (or its own, if it is
// already in that column).
func (s *Server) columnTarget(ctx context.Context, b *boardRow, iss *issueRow, column int64) (string, error) {
	var statuses []string
	if err := sqlq.Each(ctx, s.db, `SELECT cs.status_id FROM board_column_statuses cs JOIN board_columns c ON c.id = cs.column_id
		WHERE c.id = ? AND c.board_id = ? ORDER BY cs.position`, func(rows *sql.Rows) error {
		var st string
		if err := rows.Scan(&st); err != nil {
			return err
		}
		statuses = append(statuses, st)
		return nil
	}, column, b.ID); err != nil {
		return "", err
	}
	if len(statuses) == 0 {
		return "", fail(http.StatusBadRequest, i18n.BoardColumnUnknown)
	}
	if slices.Contains(statuses, iss.StatusID) {
		return iss.StatusID, nil
	}
	cat, err := s.catalogue(ctx)
	if err != nil {
		return "", err
	}
	_, reachable, err := s.issueTransitions(ctx, cat, iss)
	if err != nil {
		return "", err
	}
	for _, st := range statuses {
		if slices.ContainsFunc(reachable, func(r statusRef) bool { return r.ID == st }) {
			return st, nil
		}
	}
	return "", fail(http.StatusConflict, i18n.BoardMoveNotAllowed)
}

// visibleBoardIDs lists the boards the user sees.
func (s *Server) visibleBoardIDs(ctx context.Context, sub *perm.Subject) ([]any, error) {
	visible, err := s.visibleFilterIDs(ctx, sub)
	if err != nil {
		return nil, err
	}
	var out []any
	err = sqlq.Each(ctx, s.db, `SELECT id, filter_id FROM boards`, func(rows *sql.Rows) error {
		var id, filter int64
		if err := rows.Scan(&id, &filter); err != nil {
			return err
		}
		if visible[filter] {
			out = append(out, id)
		}
		return nil
	})
	return out, err
}

// issuesOnBoard checks, with one query, that the board lists every issue: they
// match its filter as the board view reads it.
func (s *Server) issuesOnBoard(ctx context.Context, sub *perm.Subject, b *boardRow, issues []*issueRow) error {
	where, args, err := s.boardWhere(ctx, sub, b, nil)
	if err != nil {
		return err
	}
	ids := map[int64]bool{}
	for _, iss := range issues {
		if !ids[iss.ID] {
			ids[iss.ID] = true
			args = append(args, iss.ID)
		}
	}
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE `+where+` AND i.id IN (`+sqlq.Marks(len(ids))+`)`, args...).Scan(&n); err != nil {
		return err
	}
	if n != len(ids) {
		return fail(http.StatusBadRequest, i18n.SprintNotOnBoard)
	}
	return nil
}

// setSprint moves an issue into an open sprint of the board, or back to the
// backlog (0). Closed sprints stay in its history. The backlog move, on a board
// or not (an empty boardRow), only leaves the open sprints of boards the user
// sees. onBoard says the caller has already checked the issue against the board.
func (s *Server) setSprint(ctx context.Context, sub *perm.Subject, b *boardRow, iss *issueRow, sprint int64, onBoard bool) error {
	scope, scopeArgs := "", []any(nil)
	if sprint == 0 {
		boards, err := s.visibleBoardIDs(ctx, sub)
		if err != nil {
			return err
		}
		if len(boards) == 0 {
			return nil
		}
		scope, scopeArgs = ` AND s.board_id IN (`+sqlq.Marks(len(boards))+`)`, boards
	}
	var oldID sql.NullInt64
	var oldName sql.NullString
	s.db.QueryRowContext(ctx, `SELECT s.id, s.name FROM sprint_issues si JOIN sprints s ON s.id = si.sprint_id
		WHERE si.issue_id = ? AND s.state <> 'closed'`+scope+` LIMIT 1`, append([]any{iss.ID}, scopeArgs...)...).Scan(&oldID, &oldName)
	var newName any
	if sprint != 0 {
		var name, state string
		err := s.db.QueryRowContext(ctx, `SELECT name, state FROM sprints WHERE id = ? AND board_id = ?`, sprint, b.ID).
			Scan(&name, &state)
		if errors.Is(err, sql.ErrNoRows) || state == "closed" {
			return fail(http.StatusBadRequest, i18n.SprintUnknown)
		}
		if err != nil {
			return err
		}
		newName = name
	}
	if (oldID.Valid && oldID.Int64 == sprint) || (sprint == 0 && !oldID.Valid) {
		return nil
	}
	if sprint != 0 && !onBoard {
		if err := s.issuesOnBoard(ctx, sub, b, []*issueRow{iss}); err != nil {
			return err
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE si FROM sprint_issues si JOIN sprints s ON s.id = si.sprint_id
		WHERE si.issue_id = ? AND s.state <> 'closed'`+scope, append([]any{iss.ID}, scopeArgs...)...); err != nil {
		return err
	}
	if sprint != 0 {
		if _, err := tx.ExecContext(ctx, `INSERT IGNORE INTO sprint_issues (sprint_id, issue_id) VALUES (?, ?)`,
			sprint, iss.ID); err != nil {
			return err
		}
	}
	now := time.Now().UTC()
	c := change{field: "Sprint", oldString: sqlq.Value(oldName), newString: newName}
	if oldID.Valid {
		c.oldValue = strconv.FormatInt(oldID.Int64, 10)
	}
	if sprint != 0 {
		c.newValue = strconv.FormatInt(sprint, 10)
	}
	if err := recordChanges(ctx, tx, iss.ID, sub.UserID, now, []change{c}); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE issues SET updated_at = ? WHERE id = ?`, now, iss.ID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.changed(iss, sub.UserID, "", []change{c}) // the caller refreshes open pages
	return nil
}

// Sprints ----------------------------------------------------------------------

func (s *Server) sprintForManage(r *http.Request, sub *perm.Subject) (*boardRow, *sprintOut, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return nil, nil, errNotFound
	}
	var board sql.NullInt64
	if err := s.db.QueryRowContext(r.Context(), `SELECT board_id FROM sprints WHERE id = ?`, id).Scan(&board); err != nil {
		return nil, nil, errNotFound
	}
	b, err := s.boardByID(r.Context(), sub, board.Int64)
	if err != nil {
		return nil, nil, err
	}
	if ok, err := s.canManage(r.Context(), sub, b); err != nil {
		return nil, nil, err
	} else if !ok {
		return nil, nil, errForbidden
	}
	sp, err := s.sprintByID(r.Context(), b.ID, id)
	if err != nil {
		return nil, nil, err
	}
	return b, sp, nil
}

func (s *Server) createSprint(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	b, err := s.board(r, sub)
	if err != nil {
		return err
	}
	if b.Kind != "scrum" {
		return fail(http.StatusBadRequest, i18n.SprintKanban)
	}
	if ok, err := s.canManage(ctx, sub, b); err != nil {
		return err
	} else if !ok {
		return errForbidden
	}
	var in struct {
		Name string `json:"name"`
		Goal string `json:"goal"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	var count, position int64
	s.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(MAX(position), 0) FROM sprints WHERE board_id = ?`, b.ID).
		Scan(&count, &position)
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = b.Name + " Sprint " + strconv.FormatInt(count+1, 10)
	}
	goal := sqlq.NullIfBlank(in.Goal)
	res, err := s.db.ExecContext(ctx, `INSERT INTO sprints (board_id, name, goal, state, position) VALUES (?, ?, ?, 'future', ?)`,
		b.ID, sqlq.Truncate(name, varcharWidth), goal, position+1)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	s.emitSprint(ctx, events.SprintCreated, sub.UserID, b.ID, id)
	return writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

func (s *Server) startSprint(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	b, sp, err := s.sprintForManage(r, sub)
	if err != nil {
		return err
	}
	if sp.State != "future" {
		return fail(http.StatusConflict, i18n.SprintStarted)
	}
	var in struct {
		Name *string    `json:"name"`
		Goal *string    `json:"goal"`
		End  *time.Time `json:"end"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	var active int
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sprints WHERE board_id = ? AND state = 'active'`, b.ID).Scan(&active)
	if active > 0 {
		return fail(http.StatusConflict, i18n.SprintAlreadyActive)
	}
	start := time.Now().UTC()
	end := start.AddDate(0, 0, 14)
	if in.End != nil {
		if !in.End.After(start) {
			return fail(http.StatusBadRequest, i18n.SprintEndPast)
		}
		end = in.End.UTC()
	}
	sets := []string{"state = 'active'", "start_at = ?", "end_at = ?"}
	args := []any{start, end}
	if in.Name != nil && strings.TrimSpace(*in.Name) != "" {
		sets, args = append(sets, "name = ?"), append(args, sqlq.Truncate(strings.TrimSpace(*in.Name), varcharWidth))
	}
	if in.Goal != nil {
		sets, args = append(sets, "goal = ?"), append(args, sqlq.NullIfBlank(*in.Goal))
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE sprints SET `+strings.Join(sets, ", ")+` WHERE id = ?`,
		append(args, sp.ID)...); err != nil {
		return err
	}
	s.emitSprint(ctx, events.SprintStarted, sub.UserID, b.ID, sp.ID)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// completeSprint closes the sprint. Its unfinished issues go to another open
// sprint of the board (moveTo) or back to the backlog; the closed sprint
// stays in their history either way.
func (s *Server) completeSprint(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	b, sp, err := s.sprintForManage(r, sub)
	if err != nil {
		return err
	}
	if sp.State != "active" {
		return fail(http.StatusConflict, i18n.SprintNotActive)
	}
	var in struct {
		MoveTo int64 `json:"moveTo"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	if in.MoveTo != 0 {
		var state string
		err := s.db.QueryRowContext(ctx, `SELECT state FROM sprints WHERE id = ? AND board_id = ?`, in.MoveTo, b.ID).Scan(&state)
		if err != nil || state != "future" || in.MoveTo == sp.ID {
			return fail(http.StatusBadRequest, i18n.SprintTargetInvalid)
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `UPDATE sprints SET state = 'closed', completed_at = ? WHERE id = ?`, now, sp.ID); err != nil {
		return err
	}
	moved := int64(0)
	if in.MoveTo != 0 {
		res, err := tx.ExecContext(ctx, `INSERT IGNORE INTO sprint_issues (sprint_id, issue_id)
			SELECT ?, si.issue_id FROM sprint_issues si JOIN issues i ON i.id = si.issue_id
			JOIN statuses st ON st.id = i.status_id WHERE si.sprint_id = ? AND st.category <> 'done'`, in.MoveTo, sp.ID)
		if err != nil {
			return err
		}
		moved, _ = res.RowsAffected()
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.emitSprint(ctx, events.SprintClosed, sub.UserID, b.ID, sp.ID)
	return writeJSON(w, http.StatusOK, map[string]int64{"moved": moved})
}

// Creating a board ---------------------------------------------------------------

// createBoard makes a board for a project: a saved filter on the project,
// shared with it, and one column per status category of its workflow.
func (s *Server) createBoard(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	var in struct {
		Name    string `json:"name"`
		Kind    string `json:"kind"`
		Project string `json:"project"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	if in.Kind != "kanban" && in.Kind != "scrum" {
		return fail(http.StatusBadRequest, i18n.BoardTypeUnknown)
	}
	projectID, err := s.projectByKey(ctx, in.Project)
	if err != nil || !sub.CanBrowseProject(projectID) {
		return errNotFound
	}
	if !sub.Admin && !sub.Can(perm.Admin, projectID, nil) {
		return errForbidden
	}
	var key string
	if err := s.db.QueryRowContext(ctx, `SELECT pkey FROM projects WHERE id = ?`, projectID).Scan(&key); err != nil {
		return err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = key + " — " + map[string]string{"kanban": "Kanban", "scrum": "Scrum"}[in.Kind]
	}
	cat, err := s.catalogue(ctx)
	if err != nil {
		return err
	}
	// The project's statuses (its workflows', or those its issues use).
	var statuses []statusRef
	if err := sqlq.Each(ctx, s.db, `SELECT DISTINCT ws.status_id FROM workflow_statuses ws JOIN workflows w ON w.id = ws.workflow_id
		WHERE w.project_id = ? UNION SELECT DISTINCT status_id FROM issues WHERE project_id = ?`, func(rows *sql.Rows) error {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		statuses = append(statuses, cat.statusOf(id))
		return nil
	}, projectID, projectID); err != nil {
		return err
	}

	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `INSERT INTO filters (owner_id, name, query, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		sub.UserID, i18n.Sprintf(locale(r), i18n.BoardFilterName, name), `project = "`+key+`" ORDER BY Rank ASC`, now, now)
	if err != nil {
		return err
	}
	filterID, _ := res.LastInsertId()
	if _, err := tx.ExecContext(ctx, `INSERT INTO filter_shares (filter_id, kind, project_id) VALUES (?, 'project', ?)`,
		filterID, projectID); err != nil {
		return err
	}
	res, err = tx.ExecContext(ctx, `INSERT INTO boards (name, kind, filter_id, owner_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`, sqlq.Truncate(name, varcharWidth), in.Kind, filterID, sub.UserID, now, now)
	if err != nil {
		return err
	}
	boardID, _ := res.LastInsertId()
	if _, err := tx.ExecContext(ctx, `INSERT INTO board_projects (board_id, project_id) VALUES (?, ?)`, boardID, projectID); err != nil {
		return err
	}
	for pos, category := range issue.StatusCategories {
		res, err := tx.ExecContext(ctx, `INSERT INTO board_columns (board_id, name, position) VALUES (?, ?, ?)`,
			boardID, category.Name().In(s.defaultLanguage()), pos)
		if err != nil {
			return err
		}
		column, _ := res.LastInsertId()
		n := 0
		for _, st := range statuses {
			if st.Category == category {
				if _, err := tx.ExecContext(ctx, `INSERT INTO board_column_statuses (column_id, status_id, position)
					VALUES (?, ?, ?)`, column, st.ID, n); err != nil {
					return err
				}
				n++
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.emit(events.Event{Kind: events.BoardCreated, ActorID: sub.UserID,
		Board: &events.Board{ID: boardID, Name: sqlq.Truncate(name, varcharWidth), Kind: in.Kind}})
	s.auditBoard(r, audit.BoardCreate, boardID, sqlq.Truncate(name, varcharWidth), map[string]any{"kind": in.Kind, "project": key})
	return writeJSON(w, http.StatusCreated, map[string]int64{"id": boardID})
}
