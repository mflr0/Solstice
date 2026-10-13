// Package jql understands Jira Query Language, so filters and habits carried
// over from Jira keep working. Parse turns a query into a syntax tree;
// Compile (compile.go) turns the tree into SQL for Solstice's schema.
//
// Supported: AND, OR, NOT, parentheses; the operators = != ~ !~ > >= < <=
// IN, NOT IN, IS EMPTY, IS NOT EMPTY; functions such as currentUser(),
// membersOf(), now(), startOfDay(); relative dates (-7d, 2w); ORDER BY.
// History operators (WAS, CHANGED) are not supported yet.
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
	Op     string // = != ~ !~ > >= < <= in "not in" is "is not"
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

// Error is a syntax error, with a French message for the user.
type Error struct {
	Pos int
	Msg string
}

func (e *Error) Error() string { return e.Msg }

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
				return nil, &Error{start, "Guillemet non refermé."}
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
				return nil, &Error{start, "Opérateur « ! » incomplet."}
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
				return nil, &Error{start, fmt.Sprintf("Caractère inattendu « %c ».", c)}
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
	return &Error{p.peek().pos, fmt.Sprintf(format, args...)}
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
	c := Clause{Field: f.text}
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
	case p.keyword("was", "changed"):
		return nil, p.errorf("L'historique (« %s ») n'est pas encore pris en charge.", strings.ToUpper(t.text))
	default:
		return nil, p.errorf("Opérateur attendu après « %s ».", f.text)
	}

	if c.Op == "in" || c.Op == "not in" {
		if p.peek().kind != tLParen {
			// A function returning a list: assignee in membersOf("devs").
			v, err := p.value()
			if err != nil {
				return nil, err
			}
			c.Values = []Value{v}
			return c, nil
		}
		p.next()
		for {
			v, err := p.value()
			if err != nil {
				return nil, err
			}
			c.Values = append(c.Values, v)
			if p.peek().kind == tComma {
				p.next()
				continue
			}
			if p.next().kind != tRParen {
				return nil, p.errorf("Parenthèse fermante attendue.")
			}
			break
		}
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
	return Value{}, &Error{t.pos, "Valeur attendue."}
}

// looksLike recognises JQL typed in the search bar: a known field followed by
// an operator, or an ORDER BY.
var looksLike = regexp.MustCompile(`(?i)(^|[\s(])(project|status|statuscategory|assignee|reporter|creator|type|issuetype|priority|resolution|labels?|component|fixversion|affectedversion|created|updated|resolved|due|duedate|text|summary|description|comment|key|issue|issuekey|parent|watcher|voter|cf\[\d+\]|"[^"]+")\s*(=|!=|~|!~|>=?|<=?|\s+in\s*\(|\s+not\s+in\s|\s+is\s)|\border\s+by\b`)

// LooksLike reports whether a search string is JQL rather than the simple
// search syntax.
func LooksLike(s string) bool { return looksLike.MatchString(s) }
