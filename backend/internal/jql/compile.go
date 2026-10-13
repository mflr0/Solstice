package jql

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Field is a custom field known to the compiler.
type Field struct {
	ID      int64
	Name    string
	TypeKey string
}

// Env is what compiling needs to know about the user and the instance.
type Env struct {
	Me  int64
	Now time.Time
	Loc *time.Location // time zone of dates typed without one
	// CustomField finds a custom field by name (case-insensitive) or id.
	CustomField func(nameOrID string) (Field, bool)
}

// Compile turns a query into a SQL condition on issues (alias i, projects
// as p) with its arguments, and an ORDER BY clause.
func Compile(q *Query, env Env) (where string, args []any, order string, err error) {
	if env.Loc == nil {
		env.Loc = time.UTC
	}
	if env.Now.IsZero() {
		env.Now = time.Now()
	}
	c := &compiler{env: env}
	where = "TRUE"
	if q.Where != nil {
		where, err = c.node(q.Where)
		if err != nil {
			return "", nil, "", err
		}
	}
	order, err = c.order(q.Order)
	return where, c.args, order, err
}

type compiler struct {
	env  Env
	args []any
}

func (c *compiler) arg(v ...any) { c.args = append(c.args, v...) }

func fail(format string, a ...any) error { return &Error{Msg: fmt.Sprintf(format, a...)} }

func (c *compiler) node(n Node) (string, error) {
	switch n := n.(type) {
	case And:
		l, err := c.node(n.Left)
		if err != nil {
			return "", err
		}
		r, err := c.node(n.Right)
		return "(" + l + " AND " + r + ")", err
	case Or:
		l, err := c.node(n.Left)
		if err != nil {
			return "", err
		}
		r, err := c.node(n.Right)
		return "(" + l + " OR " + r + ")", err
	case Not:
		inner, err := c.node(n.Inner)
		// NOT of an unknown (NULL) comparison keeps the row.
		return "NOT COALESCE(" + inner + ", FALSE)", err
	case Clause:
		return c.clause(n)
	}
	return "", fail("Requête invalide.")
}

func (c *compiler) clause(cl Clause) (string, error) {
	switch strings.ToLower(cl.Field) {
	case "project":
		return c.lookup(cl, "i.project_id", "SELECT id FROM projects WHERE pkey IN (%s) OR name IN (%s) OR id IN (%s)", upper)
	case "status":
		return c.lookup(cl, "i.status_id", "SELECT id FROM statuses WHERE id IN (%s) OR name IN (%s) OR id IN (%s)", nil)
	case "statuscategory":
		return c.statusCategory(cl)
	case "type", "issuetype":
		return c.lookup(cl, "i.issue_type_id", "SELECT id FROM issue_types WHERE id IN (%s) OR name IN (%s) OR id IN (%s)", nil)
	case "priority":
		return c.lookup(cl, "i.priority_id", "SELECT id FROM priorities WHERE id IN (%s) OR name IN (%s) OR id IN (%s)", nil)
	case "resolution":
		return c.resolution(cl)
	case "assignee":
		return c.user(cl, "i.assignee_id")
	case "reporter":
		return c.user(cl, "i.reporter_id")
	case "creator":
		return c.user(cl, "i.creator_id")
	case "labels", "label":
		return c.multi(cl, "SELECT 1 FROM labels l WHERE l.issue_id = i.id AND l.field_id IS NULL",
			"LOWER(l.label) IN (%s)", lower)
	case "component":
		return c.multi(cl, "SELECT 1 FROM issue_components ic JOIN components co ON co.id = ic.component_id WHERE ic.issue_id = i.id",
			"(co.name IN (%[1]s) OR co.id IN (%[1]s))", nil)
	case "fixversion":
		return c.multi(cl, "SELECT 1 FROM issue_versions iv JOIN versions v ON v.id = iv.version_id WHERE iv.issue_id = i.id AND iv.kind = 'fix'",
			"(v.name IN (%[1]s) OR v.id IN (%[1]s))", nil)
	case "affectedversion":
		return c.multi(cl, "SELECT 1 FROM issue_versions iv JOIN versions v ON v.id = iv.version_id WHERE iv.issue_id = i.id AND iv.kind = 'affects'",
			"(v.name IN (%[1]s) OR v.id IN (%[1]s))", nil)
	case "watcher":
		return c.userSet(cl, "SELECT 1 FROM watchers w WHERE w.issue_id = i.id", "w.user_id")
	case "voter":
		return c.userSet(cl, "SELECT 1 FROM votes vo WHERE vo.issue_id = i.id", "vo.user_id")
	case "created", "createddate":
		return c.date(cl, "i.created_at", false)
	case "updated", "updateddate":
		return c.date(cl, "i.updated_at", false)
	case "resolved", "resolutiondate":
		return c.date(cl, "i.resolved_at", false)
	case "due", "duedate":
		return c.date(cl, "i.due_date", true)
	case "key", "issue", "issuekey", "id":
		return c.key(cl)
	case "parent":
		return c.parent(cl)
	case "summary":
		return c.like(cl, "i.summary")
	case "description":
		return c.like(cl, "i.description")
	case "environment":
		return c.like(cl, "i.environment")
	case "text":
		return c.text(cl)
	case "comment":
		return c.comment(cl)
	case "sprint", "rank", "epic name", "epic status", "epic color":
		return "", fail("Le champ « %s » (Jira Software) n'est pas encore pris en charge.", cl.Field)
	}
	if c.env.CustomField != nil {
		name := cl.Field
		if m := cfRef.FindStringSubmatch(name); m != nil {
			name = m[1]
		}
		if f, ok := c.env.CustomField(name); ok {
			return c.custom(cl, f)
		}
	}
	return "", fail("Champ inconnu : « %s ».", cl.Field)
}

var cfRef = regexp.MustCompile(`(?i)^cf\[(\d+)\]$`)

func upper(s string) string { return strings.ToUpper(s) }
func lower(s string) string { return strings.ToLower(s) }

func marks(n int) string { return strings.TrimSuffix(strings.Repeat("?, ", n), ", ") }

// split separates literal values from EMPTY.
func split(vals []Value) (texts []string, empty bool, err error) {
	for _, v := range vals {
		switch {
		case v.Empty:
			empty = true
		case v.Func != "":
			return nil, false, fail("La fonction %s() ne s'emploie pas ici.", v.Func)
		default:
			texts = append(texts, v.Text)
		}
	}
	return texts, empty, nil
}

func requireOp(cl Clause, allowed ...string) error {
	for _, a := range allowed {
		if cl.Op == a {
			return nil
		}
	}
	return fail("L'opérateur « %s » ne s'emploie pas avec « %s ».", strings.ToUpper(cl.Op), cl.Field)
}

// membership combines "col matches a set" and "col is empty" for = != in
// "not in" is "is not".
func membership(op, match, isNull string) string {
	switch op {
	case "is":
		return isNull
	case "is not":
		return "NOT " + isNull
	case "!=", "not in":
		if match == "" {
			return "NOT " + isNull
		}
		// As in Jira, != never matches an empty field: NOT (NULL) is NULL.
		return "NOT (" + match + ")"
	}
	return match
}

// lookup filters a single-valued reference column by ids or names.
// tmpl has three %s for the same value list (it matches ids, names, keys…).
func (c *compiler) lookup(cl Clause, col, tmpl string, norm func(string) string) (string, error) {
	if err := requireOp(cl, "=", "!=", "in", "not in", "is", "is not"); err != nil {
		return "", err
	}
	texts, empty, err := split(cl.Values)
	if err != nil {
		return "", err
	}
	match := ""
	if len(texts) > 0 {
		m := marks(len(texts))
		match = col + " IN (" + fmt.Sprintf(tmpl, m, m, m) + ")"
		for range 3 {
			for _, t := range texts {
				if norm != nil {
					t = norm(t)
				}
				c.arg(t)
			}
		}
	}
	isNull := col + " IS NULL"
	if empty && match != "" && (cl.Op == "=" || cl.Op == "in") {
		return "(" + match + " OR " + isNull + ")", nil
	}
	if empty && cl.Op != "is" && cl.Op != "is not" {
		if match == "" {
			return membership(map[string]string{"=": "is", "in": "is", "!=": "is not", "not in": "is not"}[cl.Op], "", isNull), nil
		}
		return "(" + membership(cl.Op, match, isNull) + " AND " + col + " IS NOT NULL)", nil
	}
	return membership(cl.Op, match, isNull), nil
}

var categories = map[string]string{
	"to do": "todo", "todo": "todo", "new": "todo", "à faire": "todo", "a faire": "todo",
	"in progress": "doing", "indeterminate": "doing", "doing": "doing", "en cours": "doing",
	"done": "done", "complete": "done", "terminé": "done", "termine": "done",
}

func (c *compiler) statusCategory(cl Clause) (string, error) {
	if err := requireOp(cl, "=", "!=", "in", "not in"); err != nil {
		return "", err
	}
	texts, _, err := split(cl.Values)
	if err != nil {
		return "", err
	}
	var cats []any
	for _, t := range texts {
		cat, ok := categories[strings.ToLower(t)]
		if !ok {
			return "", fail("Catégorie de statut inconnue : « %s ».", t)
		}
		cats = append(cats, cat)
	}
	c.arg(cats...)
	match := "i.status_id IN (SELECT id FROM statuses WHERE category IN (" + marks(len(cats)) + "))"
	return membership(cl.Op, match, "FALSE"), nil
}

func (c *compiler) resolution(cl Clause) (string, error) {
	// "resolution = Unresolved" is Jira's way of saying EMPTY.
	vals := make([]Value, len(cl.Values))
	for i, v := range cl.Values {
		if !v.Empty && v.Func == "" && strings.EqualFold(v.Text, "unresolved") {
			v = Value{Empty: true}
		}
		vals[i] = v
	}
	cl.Values = vals
	if len(vals) == 1 && vals[0].Empty {
		switch cl.Op {
		case "=", "in":
			cl.Op = "is"
		case "!=", "not in":
			cl.Op = "is not"
		}
	}
	return c.lookup(cl, "i.resolution_id", "SELECT id FROM resolutions WHERE id IN (%s) OR name IN (%s) OR id IN (%s)", nil)
}

// users returns a subquery selecting user ids for user values: user names,
// Jira user keys, display names, currentUser(), membersOf("group").
func (c *compiler) users(vals []Value) (sub string, empty bool, err error) {
	var parts []string
	var names []string
	for _, v := range vals {
		switch {
		case v.Empty:
			empty = true
		case v.Func == "currentuser":
			parts = append(parts, "SELECT ?")
			c.arg(c.env.Me)
		case v.Func == "membersof":
			if len(v.Args) != 1 {
				return "", false, fail("membersOf() attend un nom de groupe.")
			}
			parts = append(parts, "SELECT m.user_id FROM group_members m JOIN user_groups g ON g.id = m.group_id WHERE g.name = ?")
			c.arg(strings.ToLower(v.Args[0]))
		case v.Func != "":
			return "", false, fail("La fonction %s() ne s'emploie pas avec un utilisateur.", v.Func)
		default:
			names = append(names, v.Text)
		}
	}
	if len(names) > 0 {
		m := marks(len(names))
		parts = append(parts, "SELECT id FROM users WHERE username IN ("+m+") OR user_key IN ("+m+") OR display_name IN ("+m+")")
		for _, n := range names {
			c.arg(strings.ToLower(n))
		}
		for _, n := range names {
			c.arg(n)
		}
		for _, n := range names {
			c.arg(n)
		}
	}
	return strings.Join(parts, " UNION "), empty, nil
}

func (c *compiler) user(cl Clause, col string) (string, error) {
	if err := requireOp(cl, "=", "!=", "in", "not in", "is", "is not"); err != nil {
		return "", err
	}
	sub, empty, err := c.users(cl.Values)
	if err != nil {
		return "", err
	}
	isNull := col + " IS NULL"
	if sub == "" {
		return membership(cl.Op, "", isNull), nil
	}
	match := col + " IN (" + sub + ")"
	if empty {
		if cl.Op == "=" || cl.Op == "in" {
			return "(" + match + " OR " + isNull + ")", nil
		}
		return "(" + membership(cl.Op, match, isNull) + " AND " + col + " IS NOT NULL)", nil
	}
	return membership(cl.Op, match, isNull), nil
}

// multi filters multi-valued fields (labels, components, versions) with
// EXISTS subqueries. cond has one %s (or %[1]s) for the value list.
func (c *compiler) multi(cl Clause, exists, cond string, norm func(string) string) (string, error) {
	if err := requireOp(cl, "=", "!=", "in", "not in", "is", "is not"); err != nil {
		return "", err
	}
	texts, empty, err := split(cl.Values)
	if err != nil {
		return "", err
	}
	any := "EXISTS (" + exists + ")"
	if len(texts) == 0 || cl.Op == "is" || cl.Op == "is not" {
		if cl.Op == "is" || ((cl.Op == "=" || cl.Op == "in") && empty) {
			return "NOT " + any, nil
		}
		return any, nil
	}
	m := marks(len(texts))
	n := strings.Count(cond, "%")
	match := "EXISTS (" + exists + " AND " + fmt.Sprintf(cond, m) + ")"
	if strings.Contains(cond, "%[1]s") {
		n = strings.Count(cond, "%[1]s")
	}
	for range n {
		for _, t := range texts {
			if norm != nil {
				t = norm(t)
			}
			c.arg(t)
		}
	}
	switch cl.Op {
	case "!=", "not in":
		// As in Jira, an issue without any value does not match either.
		return "(" + any + " AND NOT " + match + ")", nil
	}
	if empty {
		return "(" + match + " OR NOT " + any + ")", nil
	}
	return match, nil
}

func (c *compiler) userSet(cl Clause, exists, col string) (string, error) {
	if err := requireOp(cl, "=", "!=", "in", "not in", "is", "is not"); err != nil {
		return "", err
	}
	if cl.Op == "is" || cl.Op == "is not" {
		cond := "EXISTS (" + exists + ")"
		if cl.Op == "is" {
			cond = "NOT " + cond
		}
		return cond, nil
	}
	sub, _, err := c.users(cl.Values)
	if err != nil || sub == "" {
		return "FALSE", err
	}
	match := "EXISTS (" + exists + " AND " + col + " IN (" + sub + "))"
	if cl.Op == "!=" || cl.Op == "not in" {
		return "(EXISTS (" + exists + ") AND NOT " + match + ")", nil
	}
	return match, nil
}

var (
	relative = regexp.MustCompile(`^([+-]?)(\d+)([ywdhmM])$`)
	absolute = []string{"2006-01-02 15:04", "2006/01/02 15:04", "2006-01-02", "2006/01/02"}
)

// instant resolves a date value: an absolute date, a relative one ("-7d",
// "2w"), or a function (now(), startOfDay("-1d")…).
func (c *compiler) instant(v Value) (time.Time, error) {
	now := c.env.Now.In(c.env.Loc)
	if v.Func != "" {
		base := now
		if len(v.Args) > 0 {
			var err error
			if base, err = shift(now, v.Args[0]); err != nil {
				return time.Time{}, err
			}
		}
		y, m, d := base.Date()
		day := time.Date(y, m, d, 0, 0, 0, 0, c.env.Loc)
		switch v.Func {
		case "now":
			return base, nil
		case "startofday":
			return day, nil
		case "endofday":
			return day.AddDate(0, 0, 1).Add(-time.Millisecond), nil
		case "startofweek":
			return day.AddDate(0, 0, -((int(day.Weekday()) + 6) % 7)), nil // weeks start on Monday
		case "endofweek":
			return day.AddDate(0, 0, 7-((int(day.Weekday())+6)%7)).Add(-time.Millisecond), nil
		case "startofmonth":
			return time.Date(y, m, 1, 0, 0, 0, 0, c.env.Loc), nil
		case "endofmonth":
			return time.Date(y, m+1, 1, 0, 0, 0, 0, c.env.Loc).Add(-time.Millisecond), nil
		case "startofyear":
			return time.Date(y, 1, 1, 0, 0, 0, 0, c.env.Loc), nil
		case "endofyear":
			return time.Date(y+1, 1, 1, 0, 0, 0, 0, c.env.Loc).Add(-time.Millisecond), nil
		}
		return time.Time{}, fail("Fonction de date inconnue : %s().", v.Func)
	}
	if relative.MatchString(v.Text) {
		return shift(now, v.Text)
	}
	for _, layout := range absolute {
		if t, err := time.ParseInLocation(layout, v.Text, c.env.Loc); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fail("Date invalide : « %s » (AAAA-MM-JJ, AAAA-MM-JJ HH:MM, ou -7d, 2w…).", v.Text)
}

func shift(t time.Time, s string) (time.Time, error) {
	m := relative.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return time.Time{}, fail("Décalage invalide : « %s ».", s)
	}
	n, _ := strconv.Atoi(m[2])
	if m[1] == "-" {
		n = -n
	}
	switch m[3] {
	case "y":
		return t.AddDate(n, 0, 0), nil
	case "M":
		return t.AddDate(0, n, 0), nil
	case "w":
		return t.AddDate(0, 0, 7*n), nil
	case "d":
		return t.AddDate(0, 0, n), nil
	case "h":
		return t.Add(time.Duration(n) * time.Hour), nil
	}
	return t.Add(time.Duration(n) * time.Minute), nil
}

func (c *compiler) date(cl Clause, col string, dateOnly bool) (string, error) {
	switch cl.Op {
	case "is":
		return col + " IS NULL", nil
	case "is not":
		return col + " IS NOT NULL", nil
	case "=", "!=", ">", ">=", "<", "<=":
	default:
		return "", fail("L'opérateur « %s » ne s'emploie pas avec une date.", strings.ToUpper(cl.Op))
	}
	v := cl.Values[0]
	if v.Empty {
		if cl.Op == "!=" {
			return col + " IS NOT NULL", nil
		}
		return col + " IS NULL", nil
	}
	t, err := c.instant(v)
	if err != nil {
		return "", err
	}
	op := map[string]string{"=": "=", "!=": "<>", ">": ">", ">=": ">=", "<": "<", "<=": "<="}[cl.Op]
	if dateOnly {
		c.arg(t.In(c.env.Loc).Format(time.DateOnly))
	} else {
		c.arg(t.UTC())
	}
	return col + " " + op + " ?", nil
}

var keyPattern = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9_]*)-(\d+)$`)

// keysSQL is a condition matching the given issue keys or numeric ids on
// the issue under alias a (projects under pa).
func (c *compiler) keysSQL(texts []string, a, pa string) (string, error) {
	var conds []string
	for _, t := range texts {
		if m := keyPattern.FindStringSubmatch(t); m != nil {
			num, _ := strconv.ParseInt(m[2], 10, 64)
			conds = append(conds, "("+pa+".pkey = ? AND "+a+".num = ?)")
			c.arg(strings.ToUpper(m[1]), num)
			continue
		}
		if id, err := strconv.ParseInt(t, 10, 64); err == nil {
			conds = append(conds, a+".id = ?")
			c.arg(id)
			continue
		}
		return "", fail("Clé de ticket invalide : « %s ».", t)
	}
	if len(conds) == 0 {
		return "FALSE", nil
	}
	return "(" + strings.Join(conds, " OR ") + ")", nil
}

func (c *compiler) key(cl Clause) (string, error) {
	texts, _, err := split(cl.Values)
	if err != nil {
		return "", err
	}
	switch cl.Op {
	case "=", "in":
		return c.keysSQL(texts, "i", "p")
	case "!=", "not in":
		cond, err := c.keysSQL(texts, "i", "p")
		return "NOT " + cond, err
	case ">", ">=", "<", "<=":
		m := keyPattern.FindStringSubmatch(texts[0])
		if m == nil {
			return "", fail("Clé de ticket invalide : « %s ».", texts[0])
		}
		num, _ := strconv.ParseInt(m[2], 10, 64)
		c.arg(strings.ToUpper(m[1]), num)
		return "(p.pkey = ? AND i.num " + cl.Op + " ?)", nil
	}
	return "", requireOp(cl, "=")
}

func (c *compiler) parent(cl Clause) (string, error) {
	if err := requireOp(cl, "=", "!=", "in", "not in", "is", "is not"); err != nil {
		return "", err
	}
	if cl.Op == "is" {
		return "i.parent_id IS NULL", nil
	}
	if cl.Op == "is not" {
		return "i.parent_id IS NOT NULL", nil
	}
	texts, _, err := split(cl.Values)
	if err != nil {
		return "", err
	}
	cond, err := c.keysSQL(texts, "i2", "p2")
	if err != nil {
		return "", err
	}
	match := "i.parent_id IN (SELECT i2.id FROM issues i2 JOIN projects p2 ON p2.id = i2.project_id WHERE " + cond + ")"
	if cl.Op == "!=" || cl.Op == "not in" {
		return "NOT (" + match + ")", nil
	}
	return match, nil
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func (c *compiler) like(cl Clause, col string) (string, error) {
	if cl.Op == "is" || cl.Op == "is not" {
		cond := "(" + col + " IS NULL OR " + col + " = '')"
		if cl.Op == "is not" {
			cond = "NOT " + cond
		}
		return cond, nil
	}
	if err := requireOp(cl, "~", "!~"); err != nil {
		return "", err
	}
	texts, _, err := split(cl.Values)
	if err != nil || len(texts) == 0 {
		return "FALSE", err
	}
	c.arg("%" + escapeLike(strings.Trim(texts[0], "*")) + "%")
	if cl.Op == "!~" {
		return "NOT COALESCE(" + col + " LIKE ?, FALSE)", nil
	}
	return col + " LIKE ?", nil
}

// text searches the summary and description through the full-text index.
func (c *compiler) text(cl Clause) (string, error) {
	if err := requireOp(cl, "~", "!~"); err != nil {
		return "", err
	}
	texts, _, err := split(cl.Values)
	if err != nil || len(texts) == 0 {
		return "FALSE", err
	}
	var words []string
	for _, w := range strings.Fields(texts[0]) {
		w = strings.Map(func(r rune) rune {
			if strings.ContainsRune(`+-<>()~*"@`, r) {
				return -1
			}
			return r
		}, w)
		if len([]rune(w)) >= 3 {
			words = append(words, "+"+w+"*")
		}
	}
	if len(words) == 0 {
		cond, err := c.like(Clause{Field: "summary", Op: cl.Op, Values: cl.Values}, "i.summary")
		return cond, err
	}
	c.arg(strings.Join(words, " "))
	cond := "MATCH(i.summary, i.description) AGAINST (? IN BOOLEAN MODE)"
	if cl.Op == "!~" {
		return "NOT " + cond, nil
	}
	return cond, nil
}

// comment searches unrestricted comments only: matching restricted ones
// would reveal what they say to people who may not read them.
func (c *compiler) comment(cl Clause) (string, error) {
	if err := requireOp(cl, "~", "!~"); err != nil {
		return "", err
	}
	texts, _, err := split(cl.Values)
	if err != nil || len(texts) == 0 {
		return "FALSE", err
	}
	c.arg("%" + escapeLike(strings.Trim(texts[0], "*")) + "%")
	cond := "EXISTS (SELECT 1 FROM comments cm WHERE cm.issue_id = i.id AND cm.visibility_group IS NULL " +
		"AND cm.visibility_role_id IS NULL AND cm.body LIKE ?)"
	if cl.Op == "!~" {
		return "NOT " + cond, nil
	}
	return cond, nil
}

func fieldKind(typeKey string) string {
	if i := strings.LastIndexByte(typeKey, ':'); i >= 0 {
		return typeKey[i+1:]
	}
	return typeKey
}

// custom filters on a custom field's values.
func (c *compiler) custom(cl Clause, f Field) (string, error) {
	kind := fieldKind(f.TypeKey)
	// The field id is an integer from the database: inlined, not bound.
	if kind == "labels" {
		return c.multi(cl, fmt.Sprintf("SELECT 1 FROM labels l WHERE l.issue_id = i.id AND l.field_id = %d", f.ID),
			"LOWER(l.label) IN (%s)", lower)
	}
	base := fmt.Sprintf("SELECT 1 FROM custom_field_values v WHERE v.issue_id = i.id AND v.field_id = %d", f.ID)
	exists := func() string { return "EXISTS (" + base }

	switch cl.Op {
	case "is", "is not":
		cond := exists() + ")"
		if cl.Op == "is" {
			cond = "NOT " + cond
		}
		return cond, nil
	case "~", "!~":
		texts, _, err := split(cl.Values)
		if err != nil || len(texts) == 0 {
			return "FALSE", err
		}
		cond := exists() + " AND (v.string_value LIKE ? OR v.text_value LIKE ?))"
		like := "%" + escapeLike(texts[0]) + "%"
		c.arg(like, like)
		if cl.Op == "!~" {
			return "NOT " + cond, nil
		}
		return cond, nil
	case ">", ">=", "<", "<=":
		v := cl.Values[0]
		switch kind {
		case "datepicker", "datetime":
			t, err := c.instant(v)
			if err != nil {
				return "", err
			}
			cond := exists() + " AND v.date_value " + cl.Op + " ?)"
			if kind == "datepicker" {
				y, m, d := t.In(c.env.Loc).Date()
				t = time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
			}
			c.arg(t.UTC())
			return cond, nil
		}
		n, err := strconv.ParseFloat(v.Text, 64)
		if err != nil {
			return "", fail("Nombre attendu pour « %s ».", f.Name)
		}
		cond := exists() + " AND v.number_value " + cl.Op + " ?)"
		c.arg(n)
		return cond, nil
	}

	// =, !=, in, not in
	var cond string
	switch kind {
	case "userpicker", "multiuserpicker":
		start := exists()
		sub, _, err := c.users(cl.Values)
		if err != nil || sub == "" {
			return "FALSE", err
		}
		cond = start + " AND v.string_value IN (SELECT CAST(id AS CHAR) FROM users WHERE id IN (" + sub + ")))"
	case "gh-epic-link":
		texts, _, err := split(cl.Values)
		if err != nil {
			return "", err
		}
		start := exists()
		keys, err := c.keysSQL(texts, "i2", "p2")
		if err != nil {
			return "", err
		}
		cond = start + " AND v.number_value IN (SELECT i2.id FROM issues i2 JOIN projects p2 ON p2.id = i2.project_id WHERE " + keys + "))"
	default:
		texts, _, err := split(cl.Values)
		if err != nil || len(texts) == 0 {
			return "FALSE", err
		}
		start := exists()
		m := marks(len(texts))
		cond = start + " AND (v.string_value IN (" + m + ") OR CAST(v.number_value AS CHAR) IN (" + m + ")" +
			fmt.Sprintf(" OR v.string_value IN (SELECT CAST(o.id AS CHAR) FROM custom_field_options o WHERE o.field_id = %d", f.ID) +
			" AND o.value IN (" + m + "))))"
		for range 3 {
			for _, t := range texts {
				c.arg(t)
			}
		}
	}
	if cl.Op == "!=" || cl.Op == "not in" {
		return "(" + exists() + ") AND NOT " + cond + ")", nil
	}
	return cond, nil
}

// order compiles ORDER BY; with none, the most recently updated come first.
func (c *compiler) order(sorts []Sort) (string, error) {
	if len(sorts) == 0 {
		return "i.updated_at DESC", nil
	}
	var parts []string
	for _, s := range sorts {
		dir := " ASC"
		if s.Desc {
			dir = " DESC"
		}
		var expr string
		switch strings.ToLower(s.Field) {
		case "key", "issuekey", "issue", "id":
			parts = append(parts, "p.pkey"+dir, "i.num"+dir)
			continue
		case "created":
			expr = "i.created_at"
		case "updated":
			expr = "i.updated_at"
		case "resolved", "resolutiondate":
			expr = "i.resolved_at"
		case "due", "duedate":
			expr = "i.due_date"
		case "priority":
			// "priority DESC" puts the highest first; the highest priority has
			// the smallest position, so the direction is reversed.
			expr = "(SELECT position FROM priorities WHERE id = i.priority_id)"
			dir = map[string]string{" ASC": " DESC", " DESC": " ASC"}[dir]
		case "status":
			expr = "(SELECT position FROM statuses WHERE id = i.status_id)"
		case "summary":
			expr = "i.summary"
		case "assignee":
			expr = "(SELECT display_name FROM users WHERE id = i.assignee_id)"
		case "reporter":
			expr = "(SELECT display_name FROM users WHERE id = i.reporter_id)"
		case "project":
			expr = "p.pkey"
		case "type", "issuetype":
			expr = "(SELECT position FROM issue_types WHERE id = i.issue_type_id)"
		case "rank":
			// Jira Software's manual ranking is not imported: fall back to keys.
			parts = append(parts, "p.pkey ASC", "i.num ASC")
			continue
		default:
			return "", fail("Tri impossible sur « %s ».", s.Field)
		}
		parts = append(parts, expr+" IS NULL", expr+dir)
	}
	return strings.Join(parts, ", "), nil
}
