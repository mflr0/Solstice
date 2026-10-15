package importer

import (
	"database/sql"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"solstice/internal/jiradb"
	"solstice/internal/jql"
)

// Jira Software (formerly GreenHopper) keeps boards, sprints and ranks in
// "Active Objects" tables named AO_60DB71_*. PostgreSQL databases have them
// with quoted upper-case names, and some of their columns are reserved
// words on MySQL (RANK, KEY), so every identifier is quoted.

func (r *run) aoSelect(table string, cols ...string) string {
	quoted := make([]string, len(cols))
	for i, c := range cols {
		quoted[i] = r.quote(c)
	}
	return "SELECT " + strings.Join(quoted, ", ") + " FROM " + r.quote(table)
}

// hasTable tells whether the Jira database has a table (case-insensitive).
func (r *run) hasTable(table string) bool {
	var q string
	if r.engine == jiradb.MySQL {
		q = `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND LOWER(table_name) = ?`
	} else {
		q = `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = current_schema() AND LOWER(table_name) = $1`
	}
	var n int
	if err := r.src.QueryRowContext(r.ctx, q, strings.ToLower(table)).Scan(&n); err != nil {
		return false
	}
	return n > 0
}

// optional returns the column quoted if the table has it, NULL otherwise.
func (r *run) optional(table, column string) string {
	if r.hasColumn(table, column) {
		return r.quote(column)
	}
	return "NULL"
}

func millis(ms sql.NullInt64) any {
	if !ms.Valid || ms.Int64 <= 0 {
		return nil
	}
	return time.UnixMilli(ms.Int64).UTC()
}

var swimlaneStrategies = map[string]string{
	"none": "none", "custom": "custom", "assignee": "assignee", "assigneeUnassignedFirst": "assignee",
	"epic": "epic", "parentChild": "parent",
}

// importBoards copies Jira Software's boards, columns, quick filters,
// swimlanes and sprints, then the issues' epics and board ranks.
func (r *run) importBoards() error {
	if err := r.applyEpics(); err != nil {
		return err
	}
	if !r.hasTable("AO_60DB71_RAPIDVIEW") {
		return nil // Jira without Jira Software
	}

	subQueries := map[int64]string{}
	if r.hasTable("AO_60DB71_SUBQUERY") {
		if err := r.each(r.aoSelect("AO_60DB71_SUBQUERY", "RAPID_VIEW_ID", "QUERY", "LONG_QUERY", "SECTION"), func(rows *sql.Rows) error {
			var board sql.NullInt64
			var short, long, section sql.NullString
			if err := rows.Scan(&board, &short, &long, &section); err != nil {
				return err
			}
			if section.String == "board.kanban.work" {
				subQueries[board.Int64] = orDefault(long, short.String)
			}
			return nil
		}); err != nil {
			return err
		}
	}
	estimates := map[int64]string{}
	if r.hasTable("AO_60DB71_ESTIMATESTATISTIC") {
		if err := r.each(r.aoSelect("AO_60DB71_ESTIMATESTATISTIC", "RAPID_VIEW_ID", "FIELD_ID", "TYPE_ID"), func(rows *sql.Rows) error {
			var board sql.NullInt64
			var field, kind sql.NullString
			if err := rows.Scan(&board, &field, &kind); err != nil {
				return err
			}
			if kind.String == "field" && field.String != "" {
				estimates[board.Int64] = field.String
			}
			return nil
		}); err != nil {
			return err
		}
	}

	projectsByName := map[string]int64{}
	for id, p := range r.projects {
		projectsByName[strings.ToLower(p.key)] = id
	}
	if err := r.each(`SELECT id, pname FROM project`, func(rows *sql.Rows) error {
		var id int64
		var name sql.NullString
		if err := rows.Scan(&id, &name); err != nil {
			return err
		}
		if _, ok := r.projects[id]; ok && name.String != "" {
			projectsByName[strings.ToLower(name.String)] = id
		}
		return nil
	}); err != nil {
		return err
	}

	boards := map[int64]bool{}
	now := time.Now().UTC()
	bins := r.inserter("boards", "id", "name", "kind", "filter_id", "sub_query", "owner_id", "swimlanes", "estimate",
		"done_cutoff", "created_at", "updated_at")
	pins := r.inserter("board_projects", "board_id", "project_id")
	skipped := 0
	view := "AO_60DB71_RAPIDVIEW"
	q := r.aoSelect(view, "ID", "NAME", "OWNER_USER_NAME", "SAVED_FILTER_ID", "SPRINTS_ENABLED", "SWIMLANE_STRATEGY")
	q = strings.Replace(q, " FROM ", ", "+r.optional(view, "OLD_DONE_ISSUES_CUTOFF")+" FROM ", 1)
	if err := r.each(q, func(rows *sql.Rows) error {
		var id int64
		var name, owner, swimlanes, cutoff sql.NullString
		var filter sql.NullInt64
		var sprints sql.NullBool
		if err := rows.Scan(&id, &name, &owner, &filter, &sprints, &swimlanes, &cutoff); err != nil {
			return err
		}
		query, ok := r.filters[filter.Int64]
		if !ok {
			skipped++
			return nil
		}
		var ownerID any
		if uid, ok := r.users[owner.String]; ok {
			ownerID = uid
		} else if uid, ok := r.usersByName[strings.ToLower(owner.String)]; ok {
			ownerID = uid
		}
		kind := "kanban"
		if sprints.Bool {
			kind = "scrum"
		}
		lanes, ok := swimlaneStrategies[swimlanes.String]
		if !ok {
			lanes = "none"
		}
		var sub, estimate any
		if s, ok := subQueries[id]; ok && strings.TrimSpace(s) != "" {
			sub = s
		}
		if e, ok := estimates[id]; ok {
			estimate = e
		}
		boards[id] = true
		if parsed, err := jql.Parse(query); err == nil {
			seen := map[int64]bool{}
			for _, p := range jql.Projects(parsed) {
				if pid, ok := projectsByName[strings.ToLower(p)]; ok && !seen[pid] {
					seen[pid] = true
					if err := pins.add(id, pid); err != nil {
						return err
					}
				}
			}
		}
		return bins.add(id, truncate(orDefault(name, "Tableau "+strconv.FormatInt(id, 10)), 255), kind, filter.Int64,
			sub, ownerID, lanes, estimate, nstr(cutoff), now, now)
	}); err != nil {
		return err
	}
	if err := bins.flush(); err != nil {
		return err
	}
	if err := pins.flush(); err != nil {
		return err
	}
	r.rep.step("tableaux", bins.total, skipped)
	if skipped > 0 {
		r.rep.warn("%s", plural(skipped, "tableau non repris : son filtre n'existe plus dans Jira",
			"tableaux non repris : leur filtre n'existe plus dans Jira"))
	}

	columns := map[int64]bool{}
	cins := r.inserter("board_columns", "id", "board_id", "name", "position", "min_issues", "max_issues")
	if err := r.each(r.aoSelect("AO_60DB71_COLUMN", "ID", "RAPID_VIEW_ID", "NAME", "POS", "MINIM", "MAXIM"), func(rows *sql.Rows) error {
		var id int64
		var board, pos sql.NullInt64
		var name sql.NullString
		var minimum, maximum sql.NullFloat64
		if err := rows.Scan(&id, &board, &name, &pos, &minimum, &maximum); err != nil {
			return err
		}
		if !boards[board.Int64] {
			return nil
		}
		columns[id] = true
		var lo, hi any
		if minimum.Valid && minimum.Float64 >= 0 {
			lo = int(minimum.Float64)
		}
		if maximum.Valid && maximum.Float64 >= 0 {
			hi = int(maximum.Float64)
		}
		return cins.add(id, board.Int64, truncate(name.String, 255), pos.Int64, lo, hi)
	}); err != nil {
		return err
	}
	if err := cins.flush(); err != nil {
		return err
	}
	csins := r.inserter("board_column_statuses", "column_id", "status_id", "position")
	seen := map[string]bool{}
	if err := r.each(r.aoSelect("AO_60DB71_COLUMNSTATUS", "COLUMN_ID", "STATUS_ID", "POS"), func(rows *sql.Rows) error {
		var column, pos sql.NullInt64
		var status sql.NullString
		if err := rows.Scan(&column, &status, &pos); err != nil {
			return err
		}
		k := strconv.FormatInt(column.Int64, 10) + "/" + status.String
		if !columns[column.Int64] || !r.statuses[status.String] || seen[k] {
			return nil
		}
		seen[k] = true
		return csins.add(column.Int64, status.String, pos.Int64)
	}); err != nil {
		return err
	}
	if err := csins.flush(); err != nil {
		return err
	}
	r.rep.step("colonnes de tableaux", cins.total, 0)

	if r.hasTable("AO_60DB71_QUICKFILTER") {
		qins := r.inserter("board_quick_filters", "id", "board_id", "name", "query", "description", "position")
		if err := r.each(r.aoSelect("AO_60DB71_QUICKFILTER", "ID", "RAPID_VIEW_ID", "NAME", "QUERY", "LONG_QUERY", "DESCRIPTION", "POS"), func(rows *sql.Rows) error {
			var id int64
			var board, pos sql.NullInt64
			var name, short, long, desc sql.NullString
			if err := rows.Scan(&id, &board, &name, &short, &long, &desc, &pos); err != nil {
				return err
			}
			if !boards[board.Int64] {
				return nil
			}
			return qins.add(id, board.Int64, truncate(name.String, 255), orDefault(long, short.String),
				nstr(desc), pos.Int64)
		}); err != nil {
			return err
		}
		if err := qins.flush(); err != nil {
			return err
		}
		r.rep.step("filtres rapides", qins.total, 0)
	}

	if r.hasTable("AO_60DB71_SWIMLANE") {
		sins := r.inserter("board_swimlanes", "id", "board_id", "name", "query", "position", "is_default")
		if err := r.each(r.aoSelect("AO_60DB71_SWIMLANE", "ID", "RAPID_VIEW_ID", "NAME", "QUERY", "LONG_QUERY", "POS", "DEFAULT_LANE"), func(rows *sql.Rows) error {
			var id int64
			var board, pos sql.NullInt64
			var name, short, long sql.NullString
			var isDefault sql.NullBool
			if err := rows.Scan(&id, &board, &name, &short, &long, &pos, &isDefault); err != nil {
				return err
			}
			if !boards[board.Int64] {
				return nil
			}
			var query any
			if s := orDefault(long, short.String); s != "" {
				query = s
			}
			return sins.add(id, board.Int64, truncate(name.String, 255), query, pos.Int64, isDefault.Bool)
		}); err != nil {
			return err
		}
		if err := sins.flush(); err != nil {
			return err
		}
		r.rep.step("couloirs", sins.total, 0)
	}

	if err := r.importSprints(boards); err != nil {
		return err
	}
	return r.importRanks()
}

func (r *run) importSprints(boards map[int64]bool) error {
	if !r.hasTable("AO_60DB71_SPRINT") {
		return nil
	}
	table := "AO_60DB71_SPRINT"
	q := r.aoSelect(table, "ID", "NAME", "RAPID_VIEW_ID", "STARTED", "CLOSED", "START_DATE", "END_DATE", "COMPLETE_DATE", "SEQUENCE")
	q = strings.Replace(q, " FROM ", ", "+r.optional(table, "GOAL")+" FROM ", 1)
	ins := r.inserter("sprints", "id", "board_id", "name", "goal", "state", "start_at", "end_at", "completed_at", "position")
	if err := r.each(q, func(rows *sql.Rows) error {
		var id int64
		var name, goal sql.NullString
		var board, start, end, complete, sequence sql.NullInt64
		var started, closed sql.NullBool
		if err := rows.Scan(&id, &name, &board, &started, &closed, &start, &end, &complete, &sequence, &goal); err != nil {
			return err
		}
		state := "future"
		switch {
		case closed.Bool:
			state = "closed"
		case started.Bool:
			state = "active"
		}
		var boardID any
		if boards[board.Int64] {
			boardID = board.Int64
		}
		position := id
		if sequence.Valid {
			position = sequence.Int64
		}
		var g any
		if strings.TrimSpace(goal.String) != "" {
			g = goal.String
		}
		return ins.add(id, boardID, truncate(orDefault(name, "Sprint "+strconv.FormatInt(id, 10)), 255), g, state,
			millis(start), millis(end), millis(complete), position)
	}); err != nil {
		return err
	}
	if err := ins.flush(); err != nil {
		return err
	}
	r.rep.step("sprints", ins.total, 0)

	// Sprint membership is the Sprint custom field: one value per sprint the
	// issue has been in, holding the sprint id.
	res, err := r.dst.ExecContext(r.ctx, `INSERT IGNORE INTO sprint_issues (sprint_id, issue_id)
		SELECT DISTINCT s.id, v.issue_id FROM custom_field_values v
		JOIN custom_fields f ON f.id = v.field_id
		JOIN sprints s ON s.id = COALESCE(IF(v.string_value REGEXP '^[0-9]+$', CAST(v.string_value AS UNSIGNED), NULL),
			CAST(v.number_value AS UNSIGNED))
		JOIN issues i ON i.id = v.issue_id
		WHERE f.type_key LIKE '%:gh-sprint'`)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	r.rep.step("tickets dans les sprints", int(n), 0)
	return nil
}

// importRanks copies Jira Software's LexoRank board order onto the issues.
func (r *run) importRanks() error {
	if !r.hasTable("AO_60DB71_LEXORANK") {
		return nil
	}
	var rankField int64
	for id, typeKey := range r.fields {
		if fieldKind(typeKey) == "gh-lexo-rank" && (rankField == 0 || id < rankField) {
			rankField = id
		}
	}
	if _, err := r.dst.ExecContext(r.ctx, `CREATE TEMPORARY TABLE import_ranks (
		issue_id BIGINT NOT NULL PRIMARY KEY, rank_key VARCHAR(255) COLLATE utf8mb4_bin NOT NULL)`); err != nil {
		return err
	}
	defer r.dst.ExecContext(r.ctx, `DROP TEMPORARY TABLE IF EXISTS import_ranks`)
	ins := r.inserter("import_ranks", "issue_id", "rank_key")
	seen := map[int64]bool{}
	if err := r.each(r.aoSelect("AO_60DB71_LEXORANK", "ISSUE_ID", "RANK", "TYPE", "FIELD_ID"), func(rows *sql.Rows) error {
		var issue, kind, field sql.NullInt64
		var rank sql.NullString
		if err := rows.Scan(&issue, &rank, &kind, &field); err != nil {
			return err
		}
		// TYPE 1 rows rank issues; 0 and 2 are the minimum and maximum markers.
		if kind.Int64 != 1 || (rankField != 0 && field.Int64 != rankField) || !r.hasIssue(issue) || seen[issue.Int64] {
			return nil
		}
		seen[issue.Int64] = true
		return ins.add(issue.Int64, truncate(rank.String, 255))
	}); err != nil {
		return err
	}
	if err := ins.flush(); err != nil {
		return err
	}
	if _, err := r.dst.ExecContext(r.ctx, `UPDATE issues i JOIN import_ranks k ON k.issue_id = i.id
		SET i.rank_key = k.rank_key`); err != nil {
		return err
	}
	r.rep.step("tickets classés", ins.total, 0)
	return nil
}

// applyEpics sets each issue's epic, gathered from epic links (Jira Software
// 7) or the Epic Link field (older versions).
func (r *run) applyEpics() error {
	if len(r.epicOf) == 0 {
		return nil
	}
	if _, err := r.dst.ExecContext(r.ctx, `CREATE TEMPORARY TABLE import_epics (
		issue_id BIGINT NOT NULL PRIMARY KEY, epic_id BIGINT NOT NULL)`); err != nil {
		return err
	}
	defer r.dst.ExecContext(r.ctx, `DROP TEMPORARY TABLE IF EXISTS import_epics`)
	ins := r.inserter("import_epics", "issue_id", "epic_id")
	for _, issue := range slices.Sorted(maps.Keys(r.epicOf)) {
		if epic := r.epicOf[issue]; epic != issue {
			if err := ins.add(issue, epic); err != nil {
				return err
			}
		}
	}
	if err := ins.flush(); err != nil {
		return err
	}
	if _, err := r.dst.ExecContext(r.ctx, `UPDATE issues i JOIN import_epics e ON e.issue_id = i.id
		SET i.epic_id = e.epic_id`); err != nil {
		return err
	}
	r.rep.step("tickets rattachés à une épopée", ins.total, 0)
	return nil
}
