// Package search turns the search bar's simple syntax into SQL.
//
// The syntax is forgiving and French-first, with English aliases:
//
//	panier vide            free text (summary and description)
//	"panier vide"          exact phrase
//	DEMO-12                an issue key
//	@marie  @moi  @personne        assignee (a user, me, nobody)
//	par:@paul                      reporter
//	statut:"en cours"  est:ouvert  status name, or status category
//	type:bug  projet:DEMO  priorité:haute
//	#urgent  composant:api  version:2.4
//	-statut:fini  -#bug            a leading minus excludes
//	tri:création  tri:priorité     sort order (default: last updated)
//
// Values of the same filter are alternatives (OR); different filters must
// all match (AND).
package search

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"solstice/internal/issue"
	"solstice/internal/sqlq"
)

// Field is what a filter applies to.
type Field string

const (
	Assignee  Field = "assignee"
	Reporter  Field = "reporter"
	Status    Field = "status"
	Category  Field = "category"
	Type      Field = "type"
	Project   Field = "project"
	Priority  Field = "priority"
	Label     Field = "label"
	Component Field = "component"
	Version   Field = "version"
)

// prefixes maps what users type before ':' to a field.
var prefixes = map[string]Field{
	"par": Reporter, "auteur": Reporter, "rapporteur": Reporter, "reporter": Reporter,
	"assigné": Assignee, "assigne": Assignee, "assignee": Assignee,
	"statut": Status, "status": Status,
	"est": Category, "is": Category,
	"type":   Type,
	"projet": Project, "project": Project,
	"priorité": Priority, "priorite": Priority, "priority": Priority, "prio": Priority,
	"étiquette": Label, "etiquette": Label, "label": Label,
	"composant": Component, "component": Component,
	"version": Version,
}

// categories maps est:/is: values to status categories.
var categories = map[string][]issue.StatusCategory{
	"ouvert": {issue.Todo, issue.Doing}, "open": {issue.Todo, issue.Doing},
	"à-faire": {issue.Todo}, "a-faire": {issue.Todo}, "todo": {issue.Todo},
	"en-cours": {issue.Doing}, "doing": {issue.Doing},
	"fini": {issue.Done}, "fermé": {issue.Done}, "ferme": {issue.Done}, "résolu": {issue.Done}, "resolu": {issue.Done},
	"done": {issue.Done}, "closed": {issue.Done}, "resolved": {issue.Done},
}

// Sort orders: SQL on the issues alias i, projects alias p, priorities alias pr.
var sorts = map[string]string{
	"maj": "i.updated_at DESC", "updated": "i.updated_at DESC",
	"création": "i.created_at DESC", "creation": "i.created_at DESC", "created": "i.created_at DESC",
	"priorité": "pr.position IS NULL, pr.position ASC, i.updated_at DESC",
	"priorite": "pr.position IS NULL, pr.position ASC, i.updated_at DESC",
	"priority": "pr.position IS NULL, pr.position ASC, i.updated_at DESC",
	"clé":      "p.pkey ASC, i.num ASC", "cle": "p.pkey ASC, i.num ASC", "key": "p.pkey ASC, i.num ASC",
	"échéance": "i.due_date IS NULL, i.due_date ASC", "echeance": "i.due_date IS NULL, i.due_date ASC",
	"due": "i.due_date IS NULL, i.due_date ASC",
}

const defaultSort = "i.updated_at DESC"

// Filter is one value of a filter; Not excludes it instead.
type Filter struct {
	Field Field
	Value string
	Not   bool
}

// Query is a parsed search.
type Query struct {
	Words   []string // free-text words and phrases
	Keys    []string // issue keys, e.g. DEMO-12
	Filters []Filter
	Sort    string // SQL ORDER BY
}

var issueKey = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9_]*)-(\d+)$`)

// Parse reads a search string. It never fails: anything not understood is
// searched as text.
func Parse(s string) Query {
	q := Query{Sort: defaultSort}
	for _, tok := range tokenize(s) {
		raw := tok.text
		not := false
		if !tok.quoted && len(raw) > 1 && raw[0] == '-' {
			not, raw = true, raw[1:]
		}
		if tok.quoted {
			q.Words = append(q.Words, raw)
			continue
		}
		switch {
		case strings.HasPrefix(raw, "@") && len(raw) > 1:
			q.Filters = append(q.Filters, Filter{Assignee, raw[1:], not})
			continue
		case strings.HasPrefix(raw, "#") && len(raw) > 1:
			q.Filters = append(q.Filters, Filter{Label, raw[1:], not})
			continue
		case issueKey.MatchString(raw) && !not:
			q.Keys = append(q.Keys, strings.ToUpper(raw))
			continue
		}
		if name, value, ok := strings.Cut(raw, ":"); ok && value != "" {
			name = strings.ToLower(name)
			if name == "tri" || name == "sort" {
				if order, ok := sorts[strings.ToLower(value)]; ok {
					q.Sort = order
				}
				continue
			}
			if f, ok := prefixes[name]; ok {
				value = strings.TrimPrefix(value, "@")
				q.Filters = append(q.Filters, Filter{f, value, not})
				continue
			}
		}
		if not {
			raw = "-" + raw
		}
		q.Words = append(q.Words, raw)
	}
	return q
}

type token struct {
	text   string
	quoted bool
}

// tokenize splits on spaces, keeping "quoted phrases" and prefix:"quoted
// values" together.
func tokenize(s string) []token {
	var out []token
	var cur strings.Builder
	quoted, inQuote := false, false
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, token{cur.String(), quoted})
		}
		cur.Reset()
		quoted = false
	}
	for _, r := range s {
		switch {
		case r == '"':
			if inQuote {
				inQuote = false
				// A phrase on its own is text; prefix:"value" stays a filter.
				if !strings.Contains(cur.String(), ":") {
					quoted = true
				}
				flush()
			} else {
				inQuote = true
			}
		case unicode.IsSpace(r) && !inQuote:
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return out
}

// UsesProject tells whether SQL reads the projects alias p.
func (q Query) UsesProject() bool {
	if len(q.Keys) > 0 {
		return true
	}
	for _, f := range q.Filters {
		if f.Field == Project {
			return true
		}
	}
	return false
}

// SQL returns a condition on the issues table (alias i, with projects as p)
// and its arguments. me is the current user, for @moi.
func (q Query) SQL(me int64) (string, []any) {
	var conds []string
	var args []any

	if len(q.Keys) > 0 {
		var keys []string
		for _, k := range q.Keys {
			m := issueKey.FindStringSubmatch(k)
			num, _ := strconv.ParseInt(m[2], 10, 64)
			keys = append(keys, "(p.pkey = ? AND i.num = ?)")
			args = append(args, m[1], num)
		}
		conds = append(conds, "("+strings.Join(keys, " OR ")+")")
	}

	if c, a := textSQL(q.Words); c != "" {
		conds = append(conds, c)
		args = append(args, a...)
	}

	// Group filter values by field (and polarity): alternatives are ORed.
	type group struct {
		field Field
		not   bool
	}
	var order []group
	values := map[group][]string{}
	for _, f := range q.Filters {
		g := group{f.Field, f.Not}
		if _, ok := values[g]; !ok {
			order = append(order, g)
		}
		values[g] = append(values[g], f.Value)
	}
	for _, g := range order {
		var alts []string
		for _, v := range values[g] {
			c, a := filterSQL(g.field, v, me)
			alts = append(alts, c)
			args = append(args, a...)
		}
		c := "(" + strings.Join(alts, " OR ") + ")"
		if g.not {
			// Unassigned or unprioritised issues must stay: NOT NULL is NULL.
			c = "NOT COALESCE(" + c + ", FALSE)"
		}
		conds = append(conds, c)
	}
	if len(conds) == 0 {
		return "TRUE", nil
	}
	return strings.Join(conds, " AND "), args
}

func userSQL(column, v string, me int64) (string, []any) {
	switch strings.ToLower(v) {
	case "moi", "me", "moi-même":
		return column + " = ?", []any{me}
	case "personne", "none", "aucun", "nobody":
		return column + " IS NULL", nil
	}
	like := escapeLike(strings.ToLower(v))
	return column + ` IN (SELECT id FROM users WHERE username LIKE ? OR LOWER(display_name) LIKE ?
		OR LOWER(display_name) LIKE ?)`, []any{like + "%", like + "%", "% " + like + "%"}
}

func filterSQL(f Field, v string, me int64) (string, []any) {
	switch f {
	case Assignee:
		return userSQL("i.assignee_id", v, me)
	case Reporter:
		return userSQL("i.reporter_id", v, me)
	case Status:
		return "i.status_id IN (SELECT id FROM statuses WHERE name = ?)", []any{v}
	case Category:
		cats, ok := categories[strings.ToLower(v)]
		if !ok {
			return "FALSE", nil
		}
		return "i.status_id IN (SELECT id FROM statuses WHERE category IN (" + sqlq.Marks(len(cats)) + "))", anys(cats)
	case Type:
		return "i.issue_type_id IN (SELECT id FROM issue_types WHERE name = ?)", []any{v}
	case Project:
		return "(p.pkey = ? OR p.name = ?)", []any{strings.ToUpper(v), v}
	case Priority:
		return "i.priority_id IN (SELECT id FROM priorities WHERE name = ?)", []any{v}
	case Label:
		return "EXISTS (SELECT 1 FROM labels l WHERE l.issue_id = i.id AND l.field_id IS NULL AND LOWER(l.label) = LOWER(?))", []any{v}
	case Component:
		return `EXISTS (SELECT 1 FROM issue_components ic JOIN components c ON c.id = ic.component_id
			WHERE ic.issue_id = i.id AND c.name = ?)`, []any{v}
	case Version:
		return `EXISTS (SELECT 1 FROM issue_versions iv JOIN versions v ON v.id = iv.version_id
			WHERE iv.issue_id = i.id AND iv.kind = 'fix' AND v.name = ?)`, []any{v}
	}
	return "FALSE", nil
}

// textSQL searches words in the summary and description. Words of three
// letters or more use the full-text index; shorter ones (below InnoDB's
// minimum token size) fall back to LIKE on the summary.
func textSQL(words []string) (string, []any) {
	var boolean []string
	var conds []string
	var args []any
	for _, w := range words {
		not := strings.HasPrefix(w, "-") && len(w) > 1
		if not {
			w = w[1:]
		}
		clean := strings.Map(func(r rune) rune {
			if strings.ContainsRune(`+-<>()~*"@`, r) {
				return ' '
			}
			return r
		}, w)
		parts := strings.Fields(clean)
		if len(parts) == 0 {
			continue
		}
		short := false
		for _, p := range parts {
			if len([]rune(p)) < 3 {
				short = true
			}
		}
		if short {
			c := "(i.summary LIKE ? OR i.description LIKE ?)"
			if not {
				c = "NOT " + c
			}
			conds = append(conds, c)
			like := "%" + escapeLike(strings.Join(parts, " ")) + "%"
			args = append(args, like, like)
			continue
		}
		op := "+"
		if not {
			op = "-"
		}
		if len(parts) > 1 {
			boolean = append(boolean, op+`"`+strings.Join(parts, " ")+`"`)
		} else {
			boolean = append(boolean, op+parts[0]+"*")
		}
	}
	if len(boolean) > 0 {
		conds = append([]string{"MATCH(i.summary, i.description) AGAINST (? IN BOOLEAN MODE)"}, conds...)
		args = append([]any{strings.Join(boolean, " ")}, args...)
	}
	return strings.Join(conds, " AND "), args
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func anys[T any](xs []T) []any {
	out := make([]any, len(xs))
	for i, x := range xs {
		out[i] = x
	}
	return out
}
