package jql

// History clauses (WAS, CHANGED) read the change log: change_groups (who,
// when) and change_items (field, old and new value). A value held by an
// issue is an interval: from the change that set it (or the issue's
// creation) to the next change of the field (or now).
//
// The log is read through uncorrelated subqueries ("i.id IN (SELECT …)"),
// which the database runs once rather than once per issue; intervals come
// from window functions over the log.

import (
	"regexp"
	"strings"
	"time"

	"solstice/internal/i18n"
)

// frag is a piece of SQL with its arguments, in placeholder order.
type frag struct {
	sql  string
	args []any
}

// join concatenates strings and frags into one frag.
func join(parts ...any) frag {
	var sb strings.Builder
	var args []any
	for _, p := range parts {
		switch p := p.(type) {
		case string:
			sb.WriteString(p)
		case frag:
			sb.WriteString(p.sql)
			args = append(args, p.args...)
		}
	}
	return frag{sb.String(), args}
}

// capture runs f with an empty argument list and returns what it added,
// leaving the compiler's own list as it was.
func (c *compiler) capture(f func() (string, error)) (frag, error) {
	saved := c.args
	c.args = nil
	sql, err := f()
	out := frag{sql, c.args}
	c.args = saved
	return out, err
}

// histField is a field whose changes Jira (and Solstice) record.
type histField struct {
	name    string // change_items.field
	current string // column holding the current value; "" for fix versions
	table   string // where values are looked up by id or name; "" for users
}

var histFields = map[string]histField{
	"status":     {"status", "i.status_id", "statuses"},
	"priority":   {"priority", "i.priority_id", "priorities"},
	"resolution": {"resolution", "i.resolution_id", "resolutions"},
	"assignee":   {"assignee", "i.assignee_id", ""},
	"reporter":   {"reporter", "i.reporter_id", ""},
	"fixversion": {"Fix Version", "", "versions"},
}

func (h histField) multi() bool { return h.current == "" }

func (h histField) user() bool { return h.table == "" }

// changes is the FROM … WHERE of all changes to the field (aliases g, ci).
func (h histField) changes() string {
	return "FROM change_groups g JOIN change_items ci ON ci.group_id = g.id WHERE ci.field = '" + h.name + "'"
}

// intervals is a derived table (alias h) of the values a single-valued field
// took, joined to its change item (alias hv): each change opens one
// (start_at, by author_id, value hv.new_value) that the next change ends
// (end_at, NULL while held); n = 1 marks the first change, whose
// hv.old_value is the value the issue was created with.
func (h histField) intervals() string {
	order := "OVER (PARTITION BY g.issue_id ORDER BY g.created_at, g.id)"
	// Only ids go through the window (values, LONGTEXT, would push the
	// temporary table to disk); the item is joined back as hv.
	return "(SELECT g.issue_id, g.created_at AS start_at, g.author_id, ci.id AS item_id, " +
		"LEAD(g.created_at) " + order + " AS end_at, ROW_NUMBER() " + order + " AS n " + h.changes() + ") h " +
		"JOIN change_items hv ON hv.id = h.item_id"
}

// versionEvents is a derived table (alias h) of versions added to (added = 1)
// or removed from an issue, each with the time of the next event for the
// same version (next_at) and its rank among them (n).
func (h histField) versionEvents() string {
	events := "SELECT g.issue_id, g.created_at AS at, g.id AS gid, g.author_id, CAST(ci.new_value AS CHAR(64)) AS v, " +
		"1 AS added " + h.changes() + " AND ci.new_value IS NOT NULL UNION ALL " +
		"SELECT g.issue_id, g.created_at, g.id, g.author_id, CAST(ci.old_value AS CHAR(64)), 0 " + h.changes() +
		" AND ci.old_value IS NOT NULL"
	order := "OVER (PARTITION BY e.issue_id, e.v ORDER BY e.at, e.gid, e.added)"
	return "(SELECT e.*, LEAD(e.at) " + order + " AS next_at, ROW_NUMBER() " + order + " AS n FROM (" + events + ") e) h"
}

// histValues resolves the values of a history clause into two subqueries:
// the strings the change log records (ids, user keys) and the ids the
// issue's column holds now. empty tells whether EMPTY was among them.
func (c *compiler) histValues(h histField, vals []Value) (logged, ids frag, empty bool, err error) {
	if h.user() {
		ids, err = c.capture(func() (string, error) {
			sub, e, err := c.users(vals)
			empty = e
			return sub, err
		})
		if err != nil || ids.sql == "" {
			return frag{}, frag{}, empty, err
		}
		// The log keeps Jira user keys, or user names for changes made in Solstice.
		logged, err = c.literals(join("SELECT COALESCE(user_key, username) FROM users WHERE id IN (", ids,
			") UNION SELECT username FROM users WHERE id IN (", ids, ")"))
		return logged, ids, empty, err
	}
	if h.name == "resolution" {
		vals = append([]Value(nil), vals...)
		for i, v := range vals {
			if !v.Empty && v.Func == "" && strings.EqualFold(v.Text, "unresolved") {
				vals[i] = Value{Empty: true}
			}
		}
	}
	texts, empty, err := split(vals)
	if err != nil {
		return frag{}, frag{}, false, err
	}
	if empty && h.multi() {
		return frag{}, frag{}, false, fail(i18n.JQLEmptyHistory)
	}
	if len(texts) == 0 {
		return frag{}, frag{}, empty, nil
	}
	m := marks(len(texts))
	var args []any
	for range 2 {
		for _, t := range texts {
			args = append(args, t)
		}
	}
	where := " FROM " + h.table + " WHERE CAST(id AS CHAR) IN (" + m + ") OR name IN (" + m + ")"
	logged, err = c.literals(frag{"SELECT CAST(id AS CHAR)" + where, args})
	return logged, frag{"SELECT id" + where, args}, empty, err
}

// literals runs a subquery of values (Env.Query) and returns them as a list
// of placeholders; without Env.Query the subquery itself is kept.
func (c *compiler) literals(sub frag) (frag, error) {
	if c.env.Query == nil {
		return sub, nil
	}
	vals, err := c.env.Query(sub.sql, sub.args)
	if err != nil {
		return frag{}, err
	}
	if len(vals) == 0 {
		return frag{sql: "NULL"}, nil // IN (NULL) matches nothing
	}
	args := make([]any, len(vals))
	for i, v := range vals {
		args[i] = v
	}
	return frag{marks(len(vals)), args}, nil
}

// match is "col is one of the values".
func match(col string, set frag, empty bool) frag {
	var parts []any
	if set.sql != "" {
		parts = append(parts, join(col+" IN (", set, ")"))
	}
	if empty {
		if len(parts) > 0 {
			parts = append(parts, " OR ")
		}
		parts = append(parts, col+" IS NULL")
	}
	if len(parts) == 0 {
		return frag{sql: "FALSE"}
	}
	return join(append(append([]any{"("}, parts...), ")")...)
}

// currentMatch is "the issue holds one of the values now".
func (h histField) currentMatch(ids frag, empty bool) frag {
	if h.multi() {
		if ids.sql == "" {
			return frag{sql: "FALSE"}
		}
		return join("EXISTS (SELECT 1 FROM issue_versions iv WHERE iv.issue_id = i.id AND iv.kind = 'fix' AND iv.version_id IN (",
			ids, "))")
	}
	return match(h.current, ids, empty)
}

// histPreds are a clause's predicates, resolved.
type histPreds struct {
	after, before        *time.Time
	onStart, onEnd       *time.Time
	duringFrom, duringTo *time.Time
	by                   *frag
	from, to             []Value
	fromGiven, toGiven   bool
	timed                bool // any predicate other than FROM and TO
}

var dateOnly = regexp.MustCompile(`^\d{4}[-/]\d{1,2}[-/]\d{1,2}$`)

func (c *compiler) preds(cl Clause) (*histPreds, error) {
	pr := &histPreds{}
	when := func(kind string, v Value) (*time.Time, error) {
		if v.Empty {
			return nil, fail(i18n.JQLDateExpected, strings.ToUpper(kind))
		}
		t, err := c.instant(v)
		return &t, err
	}
	for _, p := range cl.Preds {
		var err error
		switch p.Kind {
		case "after":
			pr.after, err = when(p.Kind, p.Values[0])
		case "before":
			pr.before, err = when(p.Kind, p.Values[0])
		case "on":
			var t *time.Time
			if t, err = when(p.Kind, p.Values[0]); err == nil {
				y, m, d := t.In(c.env.Loc).Date()
				start := time.Date(y, m, d, 0, 0, 0, 0, c.env.Loc)
				end := start.AddDate(0, 0, 1)
				pr.onStart, pr.onEnd = &start, &end
			}
		case "during":
			if pr.duringFrom, err = when(p.Kind, p.Values[0]); err == nil {
				pr.duringTo, err = when(p.Kind, p.Values[1])
			}
			if err == nil && p.Values[1].Func == "" && dateOnly.MatchString(p.Values[1].Text) {
				end := pr.duringTo.AddDate(0, 0, 1) // the whole last day
				pr.duringTo = &end
			}
		case "by":
			var none bool
			var by frag
			by, err = c.capture(func() (string, error) {
				sub, e, err := c.users(p.Values)
				none = e
				return sub, err
			})
			if err == nil && (none || by.sql == "") {
				err = fail(i18n.JQLByUser)
			}
			pr.by = &by
		case "from":
			pr.from, pr.fromGiven = p.Values, true
		case "to":
			pr.to, pr.toGiven = p.Values, true
		}
		if err != nil {
			return nil, err
		}
		if p.Kind != "from" && p.Kind != "to" {
			pr.timed = true
		}
	}
	return pr, nil
}

// window applies the time predicates to an interval [start, end); a NULL
// end means the value is still held.
func (pr *histPreds) window(start, end string) frag {
	var parts []any
	if pr.after != nil {
		parts = append(parts, frag{" AND (" + end + " IS NULL OR " + end + " > ?)", []any{pr.after.UTC()}})
	}
	if pr.before != nil {
		parts = append(parts, frag{" AND " + start + " < ?", []any{pr.before.UTC()}})
	}
	if pr.onStart != nil {
		parts = append(parts, frag{" AND " + start + " < ? AND (" + end + " IS NULL OR " + end + " >= ?)",
			[]any{pr.onEnd.UTC(), pr.onStart.UTC()}})
	}
	if pr.duringFrom != nil {
		parts = append(parts, frag{" AND " + start + " < ? AND (" + end + " IS NULL OR " + end + " >= ?)",
			[]any{pr.duringTo.UTC(), pr.duringFrom.UTC()}})
	}
	return join(parts...)
}

// byUser is the BY predicate on a user column.
func (pr *histPreds) byUser(col string) frag {
	if pr.by == nil {
		return frag{}
	}
	return join(" AND "+col+" IN (", *pr.by, ")")
}

func (c *compiler) history(cl Clause) (string, error) {
	h, ok := histFields[strings.ToLower(strings.ReplaceAll(cl.Field, " ", ""))]
	if !ok {
		return "", fail(i18n.JQLHistoryField,
			strings.ToUpper(strings.Fields(cl.Op)[0]))
	}
	pr, err := c.preds(cl)
	if err != nil {
		return "", err
	}
	var f frag
	if cl.Op == "changed" {
		f, err = c.changed(h, pr)
	} else {
		f, err = c.was(h, cl.Values, pr)
		if strings.HasPrefix(cl.Op, "was not") {
			f = join("NOT COALESCE(", f, ", FALSE)")
		}
	}
	if err != nil {
		return "", err
	}
	c.arg(f.args...)
	return f.sql, nil
}

// was: the issue held one of the values at some point (within the time
// predicates; BY names who set it).
func (c *compiler) was(h histField, vals []Value, pr *histPreds) (frag, error) {
	logged, ids, empty, err := c.histValues(h, vals)
	if err != nil {
		return frag{}, err
	}
	now := h.currentMatch(ids, empty)
	if !pr.timed {
		// Held now, or set or left at some point.
		return join("(", now, " OR i.id IN (SELECT g.issue_id ", h.changes(), " AND (", match("ci.new_value", logged, empty),
			" OR ", match("ci.old_value", logged, empty), ")))"), nil
	}
	if !h.multi() {
		set := join("(", match("hv.new_value", logged, empty), pr.byUser("h.author_id"), pr.window("h.start_at", "h.end_at"), ")")
		// Whoever created the issue set its first value.
		first := join("(h.n = 1 AND ", match("hv.old_value", logged, empty), pr.byUser("ii.creator_id"),
			pr.window("ii.created_at", "h.start_at"), ")")
		never := join("(i.id NOT IN (SELECT g.issue_id ", h.changes(), ") AND ", now, pr.byUser("i.creator_id"),
			pr.window("i.created_at", "NULL"), ")")
		return join("(i.id IN (SELECT h.issue_id FROM ", h.intervals(), " JOIN issues ii ON ii.id = h.issue_id WHERE ",
			set, " OR ", first, ") OR ", never, ")"), nil
	}
	added := join("(h.added = 1 AND ", match("h.v", logged, false), pr.byUser("h.author_id"), pr.window("h.at", "h.next_at"), ")")
	// Removed before ever being added: there from the start.
	initial := join("(h.n = 1 AND h.added = 0 AND ", match("h.v", logged, false), pr.byUser("ii.creator_id"),
		pr.window("ii.created_at", "h.at"), ")")
	// There now, never added nor removed.
	untouched := join("(EXISTS (SELECT 1 FROM issue_versions iv WHERE iv.issue_id = i.id AND iv.kind = 'fix' ",
		"AND iv.version_id IN (", ids, ") AND (i.id, CAST(iv.version_id AS CHAR)) NOT IN (SELECT g.issue_id, ",
		"CAST(COALESCE(ci.new_value, ci.old_value) AS CHAR(64)) ", h.changes(), ")", " AND (i.id, CAST(iv.version_id AS CHAR)) ",
		"NOT IN (SELECT g.issue_id, CAST(ci.old_value AS CHAR(64)) ", h.changes(), " AND ci.old_value IS NOT NULL))",
		pr.byUser("i.creator_id"), pr.window("i.created_at", "NULL"), ")")
	return join("(i.id IN (SELECT h.issue_id FROM ", h.versionEvents(), " JOIN issues ii ON ii.id = h.issue_id WHERE ",
		added, " OR ", initial, ") OR ", untouched, ")"), nil
}

// changed: the field changed (from / to the values given, by someone, within
// the time predicates).
func (c *compiler) changed(h histField, pr *histPreds) (frag, error) {
	parts := []any{"i.id IN (SELECT g.issue_id ", h.changes()}
	for _, side := range []struct {
		given bool
		vals  []Value
		col   string
	}{{pr.fromGiven, pr.from, "ci.old_value"}, {pr.toGiven, pr.to, "ci.new_value"}} {
		if !side.given {
			continue
		}
		logged, _, empty, err := c.histValues(h, side.vals)
		if err != nil {
			return frag{}, err
		}
		parts = append(parts, " AND ", match(side.col, logged, empty))
	}
	parts = append(parts, pr.byUser("g.author_id"), pr.window("g.created_at", "g.created_at"), ")")
	return join(parts...), nil
}
