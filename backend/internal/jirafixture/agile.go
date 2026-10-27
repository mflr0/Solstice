package jirafixture

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"solstice/internal/jiradb"
)

// Jira Software (formerly GreenHopper) keeps boards, sprints and ranks in
// Active Objects tables prefixed AO_60DB71_. They are not described by
// entitymodel.xml; the shapes below mirror a real Jira 7.13 database.

type aoColumn struct {
	name    string
	kind    string // bigint, int, bool, double, varchar, text
	notNull bool
}

type aoSpec struct {
	name string
	cols []aoColumn
}

func (s aoSpec) has(col string) bool {
	for _, c := range s.cols {
		if c.name == col {
			return true
		}
	}
	return false
}

var aoTables = []aoSpec{
	{"AO_60DB71_RAPIDVIEW", []aoColumn{
		{"CARD_COLOR_STRATEGY", "varchar", false}, {"ID", "bigint", true}, {"NAME", "varchar", true},
		{"OWNER_USER_NAME", "varchar", true}, {"SAVED_FILTER_ID", "bigint", true}, {"SPRINTS_ENABLED", "bool", false},
		{"SWIMLANE_STRATEGY", "varchar", false}, {"SPRINT_MARKERS_MIGRATED", "bool", false},
		{"SHOW_DAYS_IN_COLUMN", "bool", false}, {"KAN_PLAN_ENABLED", "bool", false},
		{"SHOW_EPIC_AS_PANEL", "bool", false}, {"OLD_DONE_ISSUES_CUTOFF", "varchar", false},
	}},
	{"AO_60DB71_COLUMN", []aoColumn{
		{"ID", "bigint", true}, {"MAXIM", "double", false}, {"MINIM", "double", false}, {"NAME", "varchar", false},
		{"POS", "int", true}, {"RAPID_VIEW_ID", "bigint", true},
	}},
	{"AO_60DB71_COLUMNSTATUS", []aoColumn{
		{"COLUMN_ID", "bigint", true}, {"ID", "bigint", true}, {"POS", "int", true}, {"STATUS_ID", "varchar", false},
	}},
	{"AO_60DB71_SPRINT", []aoColumn{
		{"CLOSED", "bool", true}, {"COMPLETE_DATE", "bigint", false}, {"END_DATE", "bigint", false},
		{"ID", "bigint", true}, {"NAME", "varchar", true}, {"START_DATE", "bigint", false},
		{"RAPID_VIEW_ID", "bigint", false}, {"STARTED", "bool", false}, {"SEQUENCE", "bigint", false},
		{"GOAL", "text", false},
	}},
	{"AO_60DB71_QUICKFILTER", []aoColumn{
		{"DESCRIPTION", "varchar", false}, {"ID", "bigint", true}, {"NAME", "varchar", true}, {"POS", "int", true},
		{"QUERY", "varchar", false}, {"RAPID_VIEW_ID", "bigint", true}, {"LONG_QUERY", "text", false},
	}},
	{"AO_60DB71_SUBQUERY", []aoColumn{
		{"ID", "bigint", true}, {"QUERY", "varchar", false}, {"RAPID_VIEW_ID", "bigint", false},
		{"SECTION", "varchar", true}, {"LONG_QUERY", "text", false},
	}},
	{"AO_60DB71_SWIMLANE", []aoColumn{
		{"DEFAULT_LANE", "bool", true}, {"DESCRIPTION", "varchar", false}, {"ID", "bigint", true},
		{"NAME", "varchar", true}, {"POS", "int", true}, {"QUERY", "varchar", false},
		{"RAPID_VIEW_ID", "bigint", true}, {"LONG_QUERY", "text", false},
	}},
	{"AO_60DB71_LEXORANK", []aoColumn{
		{"FIELD_ID", "bigint", true}, {"ID", "bigint", true}, {"ISSUE_ID", "bigint", true},
		{"LOCK_HASH", "varchar", false}, {"LOCK_TIME", "bigint", false}, {"RANK", "varchar", true},
		{"TYPE", "int", true}, {"BUCKET", "int", false},
	}},
	{"AO_60DB71_ESTIMATESTATISTIC", []aoColumn{
		{"FIELD_ID", "varchar", false}, {"ID", "bigint", true}, {"RAPID_VIEW_ID", "bigint", true},
		{"TYPE_ID", "varchar", true},
	}},
}

func aoTable(name string) aoSpec {
	for _, t := range aoTables {
		if t.name == name {
			return t
		}
	}
	panic("jirafixture: unknown AO table " + name)
}

func aoType(eng jiradb.Engine, kind string) string {
	mysql := map[string]string{"bigint": "BIGINT", "int": "INT", "bool": "TINYINT(1)", "double": "DOUBLE",
		"varchar": "VARCHAR(255)", "text": "LONGTEXT"}
	pg := map[string]string{"bigint": "BIGINT", "int": "INTEGER", "bool": "BOOLEAN", "double": "DOUBLE PRECISION",
		"varchar": "VARCHAR(255)", "text": "TEXT"}
	if eng == jiradb.MySQL {
		return mysql[kind]
	}
	return pg[kind]
}

// AODDL returns the statements that drop and recreate the Jira Software tables.
func AODDL(eng jiradb.Engine) []string {
	var stmts []string
	for _, t := range aoTables {
		name := eng.Quote(t.name)
		if eng == jiradb.MySQL {
			stmts = append(stmts, "DROP TABLE IF EXISTS "+name)
		} else {
			stmts = append(stmts, "DROP TABLE IF EXISTS "+name+" CASCADE")
		}
		cols := make([]string, len(t.cols))
		for i, c := range t.cols {
			cols[i] = eng.Quote(c.name) + " " + aoType(eng, c.kind)
			if c.notNull {
				cols[i] += " NOT NULL"
			}
		}
		stmt := "CREATE TABLE " + name + " (" + strings.Join(cols, ", ") + ", PRIMARY KEY (" + eng.Quote("ID") + "))"
		if eng == jiradb.MySQL {
			stmt += " ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin"
		}
		stmts = append(stmts, stmt)
	}
	return stmts
}

// --- boards ----------------------------------------------------------------------

type boardColumn struct {
	name     string
	statuses []string
	min, max any // WIP limits (AO stores them as doubles), nil when unset
}

type boardDef struct {
	name, project string
	scrum         bool
	swimlanes     string // none, custom, assignee, parentChild
	cutoff        string // kanban: hide done issues older than this
	share         [3]any // sharepermissions (sharetype, param1, param2) of the board's filter
	columns       []boardColumn
	estimate      [2]any // AO_60DB71_ESTIMATESTATISTIC (FIELD_ID, TYPE_ID)
	kanPlan       bool   // kanban: the first column is a backlog
}

func (g *generator) seedAgile() {
	sr := g.w.Table("searchrequest", "id", "filtername", "authorname", "description", "username", "groupname",
		"projectid", "reqcontent", "fav_count", "filtername_lower")
	shares := g.w.Table("sharepermissions", "id", "entityid", "entitytype", "sharetype", "param1", "param2", "rights")
	rapid := g.w.AOTable("AO_60DB71_RAPIDVIEW", "ID", "NAME", "OWNER_USER_NAME", "SAVED_FILTER_ID",
		"SPRINTS_ENABLED", "SWIMLANE_STRATEGY", "CARD_COLOR_STRATEGY", "SPRINT_MARKERS_MIGRATED",
		"SHOW_DAYS_IN_COLUMN", "KAN_PLAN_ENABLED", "SHOW_EPIC_AS_PANEL", "OLD_DONE_ISSUES_CUTOFF")
	column := g.w.AOTable("AO_60DB71_COLUMN", "ID", "NAME", "POS", "RAPID_VIEW_ID", "MINIM", "MAXIM")
	columnStatus := g.w.AOTable("AO_60DB71_COLUMNSTATUS", "ID", "COLUMN_ID", "POS", "STATUS_ID")
	quick := g.w.AOTable("AO_60DB71_QUICKFILTER", "ID", "NAME", "DESCRIPTION", "POS", "QUERY", "LONG_QUERY",
		"RAPID_VIEW_ID")
	sub := g.w.AOTable("AO_60DB71_SUBQUERY", "ID", "QUERY", "LONG_QUERY", "SECTION", "RAPID_VIEW_ID")
	lane := g.w.AOTable("AO_60DB71_SWIMLANE", "ID", "NAME", "DESCRIPTION", "POS", "QUERY", "LONG_QUERY",
		"DEFAULT_LANE", "RAPID_VIEW_ID")
	estimate := g.w.AOTable("AO_60DB71_ESTIMATESTATISTIC", "ID", "RAPID_VIEW_ID", "FIELD_ID", "TYPE_ID")

	// Active Objects tables have their own id sequences, starting at 1.
	seq := map[string]int64{}
	aoID := func(table string) int64 {
		seq[table]++
		return seq[table]
	}

	software := []boardColumn{
		{"To do", []string{"10000"}, nil, nil},
		{"In progress", []string{"3"}, nil, nil},
		{"En revue", []string{"10002"}, nil, nil},
		{"Done", []string{"10001"}, nil, nil},
	}
	web := slices.Clone(software)
	web[1].max = 5.0
	web[2].min = 1.0
	business := []boardColumn{
		{"Ouvert", []string{"1", "4"}, nil, nil},
		{"In progress", []string{"3"}, nil, nil},
		{"Résolu", []string{"5", "6"}, nil, nil},
	}
	boards := []boardDef{
		{"DEMO — Scrum", "DEMO", true, "none", "", [3]any{"project", "10000", nil}, software,
			[2]any{fmt.Sprintf("customfield_%d", cfStoryPoints), "field"}, false},
		{"WEB — Kanban", "WEB", false, "custom", "2w", [3]any{"project", "10001", nil}, web,
			[2]any{nil, "issueCount"}, false},
		{"OPS — Exploitation", "OPS", false, "assignee", "", [3]any{"project", "10003", nil}, business,
			[2]any{nil, "issueCount"}, true},
		// Shared with the Users role of RH only: invisible outside HR.
		{"RH", "RH", true, "none", "", [3]any{"project", "10002", "10000"}, business,
			[2]any{"timeoriginalestimate", "field"}, false},
	}

	boardOf := map[string]int64{}
	for _, b := range boards {
		p := g.byKey[b.project]
		filterID := g.next("searchrequest")
		filterName := "Filtre du tableau " + b.name
		sr.Add(filterID, filterName, p.lead.key, nil, p.lead.key, nil, p.id,
			"project = "+p.key+" ORDER BY Rank ASC", 0, strings.ToLower(filterName))
		g.sum.Filters++
		shares.Add(g.next("sharepermissions"), filterID, "SearchRequest", b.share[0], b.share[1], b.share[2], 1)
		g.sum.FilterShares++

		id := aoID("rapidview")
		boardOf[b.project] = id
		rapid.Add(id, b.name, p.lead.key, filterID, b.scrum, b.swimlanes, "issuetype", true, true, b.kanPlan, false,
			nullable(b.cutoff))
		g.sum.Boards++

		for pos, c := range b.columns {
			cid := aoID("column")
			column.Add(cid, c.name, pos, id, c.min, c.max)
			g.sum.BoardColumns++
			for spos, st := range c.statuses {
				columnStatus.Add(aoID("columnstatus"), cid, spos, st)
				g.sum.BoardColumnStatuses++
			}
		}

		quick.Add(aoID("quickfilter"), "Mes tickets", "Ce qui m'est assigné", 0, "assignee = currentUser()", nil, id)
		quick.Add(aoID("quickfilter"), "Bugs", nil, 1, "issuetype = Bug", nil, id)
		// Long queries go to LONG_QUERY, with QUERY left empty.
		quick.Add(aoID("quickfilter"), "Urgent et récent", "Priorité haute, modifié ces deux dernières semaines", 2,
			nil, "priority in (Highest, High) AND updated >= -14d", id)
		g.sum.QuickFilters += 3

		if b.project == "WEB" {
			sub.Add(aoID("subquery"), "fixVersion in unreleasedVersions() OR fixVersion is EMPTY", nil,
				"board.kanban.work", id)
		}
		if b.swimlanes == "custom" {
			lane.Add(aoID("swimlane"), "Bugs", "Les anomalies d'abord", 0, "issuetype = Bug", nil, false, id)
			lane.Add(aoID("swimlane"), "Le reste", nil, 1, nil, nil, true, id)
			g.sum.Swimlanes += 2
		}
		estimate.Add(aoID("estimatestatistic"), id, b.estimate[0], b.estimate[1])
	}

	g.seedSprints(boardOf, aoID)
	g.seedRanks(aoID)
}

// --- sprints -----------------------------------------------------------------------

type sprintDef struct {
	id                    int64
	name, goal            string
	start, end, completed *time.Time
	closed, started       bool
}

// seedSprints creates the sprints of the scrum boards and puts issues in
// them through the Sprint custom field, one value per sprint an issue has
// been part of.
func (g *generator) seedSprints(boardOf map[string]int64, aoID func(string) int64) {
	sprintTable := g.w.AOTable("AO_60DB71_SPRINT", "ID", "NAME", "RAPID_VIEW_ID", "STARTED", "CLOSED",
		"START_DATE", "END_DATE", "COMPLETE_DATE", "SEQUENCE", "GOAL")
	values := g.w.Table("customfieldvalue", "id", "issue", "customfield", "updated", "parentkey", "stringvalue",
		"numbervalue", "textvalue", "datevalue", "valuetype")

	day := func(offset, hour int) *time.Time {
		t := time.Date(g.now.Year(), g.now.Month(), g.now.Day()+offset, hour, 0, 0, 0, g.tz)
		return &t
	}
	ms := func(t *time.Time) any {
		if t == nil {
			return nil
		}
		return t.UnixMilli()
	}
	write := func(board int64, s *sprintDef) {
		s.id = aoID("sprint")
		var goal any
		if s.goal != "" {
			goal = s.goal
		}
		sprintTable.Add(s.id, s.name, board, s.started, s.closed, ms(s.start), ms(s.end), ms(s.completed), s.id, goal)
		switch {
		case s.closed:
			g.sum.SprintsClosed++
		case s.started:
			g.sum.SprintsActive++
		default:
			g.sum.SprintsFuture++
		}
	}
	member := func(is *issue, s *sprintDef) {
		values.Add(g.next("customfieldvalue"), is.id, cfSprint, nil, nil, strconv.FormatInt(s.id, 10), nil, nil, nil, nil)
		g.sum.CustomFieldValues++
		g.sum.SprintMemberships++
	}

	// DEMO: four closed two-week sprints, the current one, the next one.
	var closed []*sprintDef
	for i := range 4 {
		s := &sprintDef{name: fmt.Sprintf("Sprint %d", i+1), started: true, closed: true,
			start: day(-63+14*i, 10), end: day(-49+14*i, 18), completed: day(-49+14*i, 17)}
		if i == 2 {
			s.goal = "Stabiliser le paiement"
		}
		write(boardOf["DEMO"], s)
		closed = append(closed, s)
	}
	active := &sprintDef{name: "Sprint 5", started: true, start: day(-7, 10), end: day(7, 18),
		goal: "Livrer l'export des factures et fermer les anomalies bloquantes"}
	write(boardOf["DEMO"], active)
	future := &sprintDef{name: "Sprint 6"}
	write(boardOf["DEMO"], future)

	rhSprint := &sprintDef{name: "RH — Octobre", started: true, start: day(-3, 10), end: day(11, 18)}
	write(boardOf["RH"], rhSprint)

	plannable := func(key string) (done, open []*issue) {
		for _, is := range g.issues {
			if is.proj.key != key || is.typ == typeSubtask || is.typ == typeEpic {
				continue
			}
			if statusByID(is.status).done() {
				done = append(done, is)
			} else {
				open = append(open, is)
			}
		}
		return done, open
	}
	shuffle := func(s []*issue) {
		g.rng.Shuffle(len(s), func(a, b int) { s[a], s[b] = s[b], s[a] })
	}

	done, open := plannable("DEMO")
	shuffle(done)
	shuffle(open)
	membership := map[*issue][]*sprintDef{}
	// Most finished issues were finished in a closed sprint; a few in the current one.
	doneInActive := done[:min(5, len(done))]
	for _, is := range done[len(doneInActive):] {
		if g.chance(0.7) {
			membership[is] = append(membership[is], closed[g.rng.IntN(len(closed))])
		}
	}
	for _, is := range doneInActive {
		membership[is] = append(membership[is], active)
	}
	inActive := open[:min(10, len(open))]
	for i, is := range inActive {
		if i < 4 { // carried over from the previous sprint
			membership[is] = append(membership[is], closed[len(closed)-1])
		}
		membership[is] = append(membership[is], active)
	}
	for _, is := range open[len(inActive):min(len(inActive)+8, len(open))] {
		membership[is] = append(membership[is], future)
	}

	_, rhOpen := plannable("RH")
	shuffle(rhOpen)
	for _, is := range rhOpen[:min(5, len(rhOpen))] {
		membership[is] = append(membership[is], rhSprint)
	}

	for _, is := range g.issues { // issue order keeps the rows deterministic
		for _, s := range membership[is] {
			member(is, s)
		}
	}
}

// --- ranks -----------------------------------------------------------------------

// seedRanks gives every issue a LexoRank: unfinished issues first, by
// priority then age, finished ones after. As in Jira, issue rows have TYPE 1;
// one TYPE 0 row (minimum) and one TYPE 2 row (maximum) bound the range.
func (g *generator) seedRanks(aoID func(string) int64) {
	lex := g.w.AOTable("AO_60DB71_LEXORANK", "ID", "FIELD_ID", "ISSUE_ID", "RANK", "TYPE", "BUCKET",
		"LOCK_HASH", "LOCK_TIME")
	order := slices.Clone(g.issues)
	sort.SliceStable(order, func(a, b int) bool {
		da, db := statusByID(order[a].status).done(), statusByID(order[b].status).done()
		if da != db {
			return !da
		}
		if order[a].priority != order[b].priority {
			return order[a].priority < order[b].priority // "1" is Highest
		}
		return order[a].created.Before(order[b].created)
	})
	lex.Add(aoID("lexorank"), cfRank, int64(math.MinInt64), "0|000000:", 0, 0, nil, nil)
	base := int64(18) * 60466176 // "i00000" in base 36
	for i, is := range order {
		rank := strconv.FormatInt(base+int64(i)*512, 36)
		lex.Add(aoID("lexorank"), cfRank, is.id, "0|"+rank+":", 1, 0, nil, nil)
		g.sum.RankedIssues++
	}
	lex.Add(aoID("lexorank"), cfRank, int64(math.MaxInt64), "0|zzzzzz:", 2, 0, nil, nil)
}
