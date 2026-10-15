// Package jql understands Jira Query Language, so filters and habits carried
// over from Jira keep working. Parse turns a query into a syntax tree;
// Compile (compile.go) turns the tree into SQL for Solstice's schema.
//
// Supported: AND, OR, NOT, parentheses; the operators = != ~ !~ > >= < <=
// IN, NOT IN, IS EMPTY, IS NOT EMPTY; functions such as currentUser(),
// membersOf(), now(), startOfDay(); relative dates (-7d, 2w); ORDER BY;
// the history operators WAS, WAS NOT, WAS IN, WAS NOT IN and CHANGED, with
// their predicates AFTER, BEFORE, ON, DURING, BY, FROM and TO.
package jql

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// Node is a boolean expression.
type Node interface{ node() }

type And struct{ Left, Right Node }
type Or struct{ Left, Right Node }
type Not struct{ Inner Node }

// Clause is "field operator value(s)".
type Clause struct {
	Field  string
	Op     string // = != ~ !~ > >= < <= in "not in" is "is not" was "was not" "was in" "was not in" changed
	Values []Value
	Preds  []Predicate // history clauses only
	Pos    int         // where the clause starts in the query, in characters
}

// Predicate narrows a history clause: AFTER, BEFORE, ON, DURING, BY, and for
// CHANGED, FROM and TO.
type Predicate struct {
	Kind   string // after, before, on, during, by, from, to
	Values []Value
}

func (And) node()    {}
func (Or) node()     {}
func (Not) node()    {}
func (Clause) node() {}

// Value is a literal, a function call or EMPTY.
type Value struct {
	Text  string
	Func  string // lower-case name when the value is a function call
	Args  []string
	Empty bool
}

// Sort is one ORDER BY term.
type Sort struct {
	Field string
	Desc  bool
}

// Query is a parsed JQL query. Where is nil when the query only sorts.
type Query struct {
	Where Node
	Order []Sort
}

// Error is a syntax or meaning error, with a French message for the user.
// Template and Args are what Msg is made of: the French template is the key
// of its translations (package i18n), so callers can show it in other
// languages.
type Error struct {
	Pos      int
	Msg      string
	Template string
	Args     []any
}

func (e *Error) Error() string { return e.Msg }

// errAt builds an Error at a position (in characters) of the query. The
// format must be a literal, to be found by the translation tests.
func errAt(pos int, format string, args ...any) *Error {
	return &Error{Pos: pos, Msg: fmt.Sprintf(format, args...), Template: format, Args: args}
}

type tokKind int

const (
	tEOF    tokKind = iota
	tWord           // identifier, number, keyword
	tString         // quoted
	tOp             // = != ~ !~ > >= < <=
	tLParen
	tRParen
	tComma
)

type token struct {
	kind tokKind
	text string
	pos  int
}

func lex(s string) ([]token, error) {
	var out []token
	r := []rune(s)
	for i := 0; i < len(r); {
		c := r[i]
		switch {
		case unicode.IsSpace(c):
			i++
		case c == '(':
			out = append(out, token{tLParen, "(", i})
			i++
		case c == ')':
			out = append(out, token{tRParen, ")", i})
			i++
		case c == ',':
			out = append(out, token{tComma, ",", i})
			i++
		case c == '"' || c == '\'':
			start := i
			var sb strings.Builder
			i++
			for i < len(r) && r[i] != c {
				if r[i] == '\\' && i+1 < len(r) {
					i++
				}
				sb.WriteRune(r[i])
				i++
			}
			if i >= len(r) {
				return nil, errAt(start, "Guillemet non refermé.")
			}
			i++
			out = append(out, token{tString, sb.String(), start})
		case strings.ContainsRune("=!~<>", c):
			start := i
			op := string(c)
			if i+1 < len(r) && (r[i+1] == '=' || (c == '!' && r[i+1] == '~')) {
				op += string(r[i+1])
			}
			if op == "!" {
				return nil, errAt(start, "Opérateur « ! » incomplet.")
			}
			i += len([]rune(op))
			out = append(out, token{tOp, op, start})
		default:
			start := i
			for i < len(r) && !unicode.IsSpace(r[i]) && !strings.ContainsRune(`()",'=!~<>`, r[i]) {
				// cf[10100] keeps its brackets.
				i++
			}
			if i == start {
				return nil, errAt(start, "Caractère inattendu « %c ».", c)
			}
			out = append(out, token{tWord, string(r[start:i]), start})
		}
	}
	return append(out, token{tEOF, "", len(r)}), nil
}

type parser struct {
	toks []token
	i    int
}

func (p *parser) peek() token { return p.toks[p.i] }

func (p *parser) next() token {
	t := p.toks[p.i]
	if t.kind != tEOF {
		p.i++
	}
	return t
}

// keyword reports whether the next token is one of the given keywords.
func (p *parser) keyword(words ...string) bool {
	t := p.peek()
	if t.kind != tWord {
		return false
	}
	for _, w := range words {
		if strings.EqualFold(t.text, w) {
			return true
		}
	}
	return false
}

func (p *parser) errorf(format string, args ...any) error {
	return errAt(p.peek().pos, format, args...)
}

// Parse reads a JQL query.
func Parse(s string) (*Query, error) {
	toks, err := lex(s)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks}
	q := &Query{}
	if !p.keyword("order") && p.peek().kind != tEOF {
		if q.Where, err = p.or(); err != nil {
			return nil, err
		}
	}
	if p.keyword("order") {
		p.next()
		if !p.keyword("by") {
			return nil, p.errorf("« BY » attendu après « ORDER ».")
		}
		p.next()
		for {
			t := p.next()
			if t.kind != tWord && t.kind != tString {
				return nil, p.errorf("Champ de tri attendu.")
			}
			s := Sort{Field: t.text}
			if p.keyword("asc", "desc") {
				s.Desc = strings.EqualFold(p.next().text, "desc")
			}
			q.Order = append(q.Order, s)
			if p.peek().kind != tComma {
				break
			}
			p.next()
		}
	}
	if p.peek().kind != tEOF {
		return nil, p.errorf("« %s » inattendu.", p.peek().text)
	}
	return q, nil
}

func (p *parser) or() (Node, error) {
	left, err := p.and()
	if err != nil {
		return nil, err
	}
	for p.keyword("or") {
		p.next()
		right, err := p.and()
		if err != nil {
			return nil, err
		}
		left = Or{left, right}
	}
	return left, nil
}

func (p *parser) and() (Node, error) {
	left, err := p.not()
	if err != nil {
		return nil, err
	}
	for p.keyword("and") {
		p.next()
		right, err := p.not()
		if err != nil {
			return nil, err
		}
		left = And{left, right}
	}
	return left, nil
}

func (p *parser) not() (Node, error) {
	if p.keyword("not") {
		p.next()
		inner, err := p.not()
		if err != nil {
			return nil, err
		}
		return Not{inner}, nil
	}
	if p.peek().kind == tLParen {
		p.next()
		inner, err := p.or()
		if err != nil {
			return nil, err
		}
		if p.next().kind != tRParen {
			return nil, p.errorf("Parenthèse fermante attendue.")
		}
		return inner, nil
	}
	return p.clause()
}

func (p *parser) clause() (Node, error) {
	f := p.next()
	if f.kind != tWord && f.kind != tString {
		return nil, p.errorf("Nom de champ attendu.")
	}
	c := Clause{Field: f.text, Pos: f.pos}
	switch t := p.peek(); {
	case t.kind == tOp:
		c.Op = p.next().text
	case p.keyword("in"):
		p.next()
		c.Op = "in"
	case p.keyword("not"):
		p.next()
		if !p.keyword("in") {
			return nil, p.errorf("« IN » attendu après « NOT ».")
		}
		p.next()
		c.Op = "not in"
	case p.keyword("is"):
		p.next()
		c.Op = "is"
		if p.keyword("not") {
			p.next()
			c.Op = "is not"
		}
	case p.keyword("was"):
		p.next()
		c.Op = "was"
		if p.keyword("not") {
			p.next()
			c.Op = "was not"
		}
		if p.keyword("in") {
			p.next()
			c.Op += " in"
			vals, err := p.list()
			if err != nil {
				return nil, err
			}
			c.Values = vals
		} else {
			v, err := p.value()
			if err != nil {
				return nil, err
			}
			c.Values = []Value{v}
		}
		return p.predicates(c)
	case p.keyword("changed"):
		p.next()
		c.Op = "changed"
		return p.predicates(c)
	default:
		return nil, p.errorf("Opérateur attendu après « %s ».", f.text)
	}

	if c.Op == "in" || c.Op == "not in" {
		vals, err := p.list()
		if err != nil {
			return nil, err
		}
		c.Values = vals
		return c, nil
	}
	v, err := p.value()
	if err != nil {
		return nil, err
	}
	if (c.Op == "is" || c.Op == "is not") && !v.Empty {
		return nil, p.errorf("« IS » s'emploie avec EMPTY.")
	}
	c.Values = []Value{v}
	return c, nil
}

// list reads the values after IN: a parenthesised list, or a function
// returning a list (assignee in membersOf("devs")).
func (p *parser) list() ([]Value, error) {
	if p.peek().kind != tLParen {
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	}
	p.next()
	var vals []Value
	for {
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		vals = append(vals, v)
		if p.peek().kind == tComma {
			p.next()
			continue
		}
		if p.next().kind != tRParen {
			return nil, p.errorf("Parenthèse fermante attendue.")
		}
		return vals, nil
	}
}

// predicates reads what follows WAS … or CHANGED.
func (p *parser) predicates(c Clause) (Node, error) {
	for p.keyword("after", "before", "on", "during", "by", "from", "to") {
		kind := strings.ToLower(p.next().text)
		if (kind == "from" || kind == "to") && c.Op != "changed" {
			return nil, p.errorf("« %s » ne s'emploie qu'avec CHANGED.", strings.ToUpper(kind))
		}
		var vals []Value
		var err error
		switch {
		case kind == "during":
			if p.peek().kind != tLParen {
				return nil, p.errorf("DURING attend deux dates entre parenthèses.")
			}
			if vals, err = p.list(); err == nil && len(vals) != 2 {
				err = p.errorf("DURING attend deux dates entre parenthèses.")
			}
		case kind == "by" && p.peek().kind == tLParen:
			vals, err = p.list()
		default:
			var v Value
			v, err = p.value()
			vals = []Value{v}
		}
		if err != nil {
			return nil, err
		}
		c.Preds = append(c.Preds, Predicate{Kind: kind, Values: vals})
	}
	return c, nil
}

func (p *parser) value() (Value, error) {
	t := p.next()
	switch t.kind {
	case tString:
		return Value{Text: t.text}, nil
	case tWord:
		if strings.EqualFold(t.text, "empty") || strings.EqualFold(t.text, "null") {
			return Value{Empty: true}, nil
		}
		if p.peek().kind == tLParen {
			p.next()
			v := Value{Func: strings.ToLower(t.text)}
			for p.peek().kind != tRParen {
				a := p.next()
				if a.kind != tWord && a.kind != tString {
					return Value{}, p.errorf("Argument de %s() invalide.", t.text)
				}
				v.Args = append(v.Args, a.text)
				if p.peek().kind == tComma {
					p.next()
				}
			}
			p.next()
			return v, nil
		}
		return Value{Text: t.text}, nil
	}
	return Value{}, errAt(t.pos, "Valeur attendue.")
}

// looksLike recognises JQL typed in the search bar: a known field followed by
// an operator, or an ORDER BY.
var looksLike = regexp.MustCompile(`(?i)(^|[\s(])(project|status|statuscategory|assignee|reporter|creator|type|issuetype|priority|resolution|labels?|component|fixversion|affectedversion|created|updated|resolved|due|duedate|text|summary|description|comment|key|issue|issuekey|parent|watcher|voter|commenter|participants?|involves|cf\[\d+\]|"[^"]+")\s*(=|!=|~|!~|>=?|<=?|\s+in\s*\(|\s+not\s+in\s|\s+is\s|\s+was\s|\s+changed\b)|\border\s+by\b`)

// LooksLike reports whether a search string is JQL rather than the simple
// search syntax.
func LooksLike(s string) bool { return looksLike.MatchString(s) }

// Projects returns the project keys or names a query is limited to by
// top-level "project = X" or "project in (…)" clauses joined with AND, or
// nil when it may span any project.
func Projects(q *Query) []string {
	var out []string
	var walk func(Node) bool
	walk = func(n Node) bool {
		switch n := n.(type) {
		case And:
			l, r := walk(n.Left), walk(n.Right)
			return l || r
		case Clause:
			if strings.EqualFold(n.Field, "project") && (n.Op == "=" || n.Op == "in") {
				for _, v := range n.Values {
					if v.Func == "" && !v.Empty {
						out = append(out, v.Text)
					}
				}
				return true
			}
		}
		return false
	}
	if q == nil || q.Where == nil || !walk(q.Where) {
		return nil
	}
	return out
}
