package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"solstice/internal/i18n"
	"solstice/internal/jql"
	"solstice/internal/perm"
	"solstice/internal/search"
	"solstice/internal/sqlq"
)

// searchFrom is the FROM clause a search condition runs against: the SQL of
// compileJQL and searchCondition, conditions and ORDER BY, names its aliases
// i (issues), p (projects) and pr (priorities).
const searchFrom = ` FROM issues i JOIN projects p ON p.id = i.project_id LEFT JOIN priorities pr ON pr.id = i.priority_id`

// searchCondition turns a search's parameters (jql, or q in either syntax,
// and project) into a condition on the issues the user sees, with its ORDER
// BY, both to run against searchFrom.
func (s *Server) searchCondition(ctx context.Context, sub *perm.Subject, params url.Values) (where string, args []any,
	order string, err error) {
	// The search bar takes both syntaxes: JQL is recognised by its shape.
	var cond string
	var cargs []any
	if src := params.Get("jql"); src != "" || jql.LooksLike(params.Get("q")) {
		if src == "" {
			src = params.Get("q")
		}
		if cond, cargs, order, err = s.compileJQL(ctx, sub, src); err != nil {
			return "", nil, "", err
		}
	} else {
		q := search.Parse(params.Get("q"))
		cond, cargs = q.SQL(sub.UserID)
		order = q.Sort
	}
	vis, args := sub.VisibleSQL("i")
	where = vis + " AND " + cond
	args = append(args, cargs...)
	if p := params.Get("project"); p != "" {
		where += " AND " + sqlq.ProjectKey("p", "?")
		args = append(args, strings.ToUpper(p), strings.ToUpper(p))
	}
	return where, args, order, nil
}

// searchRows runs a search condition and returns a page of issues with the
// total.
func (s *Server) searchRows(ctx context.Context, where string, args []any, order string, offset, limit int) (
	[]*issueRow, int, error) {
	from := searchFrom + ` WHERE ` + where
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*)`+from, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if limit <= 0 || offset >= total {
		return nil, total, nil
	}
	var out []*issueRow
	err := sqlq.Each(ctx, s.db, `SELECT `+issueColumns+from+` ORDER BY `+order+`, i.id DESC LIMIT ? OFFSET ?`,
		func(rows *sql.Rows) error {
			iss, err := scanIssue(rows)
			out = append(out, iss)
			return err
		}, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// compileJQL turns a JQL query into SQL to run against searchFrom. Syntax and
// field errors come back to the user as they are (in the user's language).
func (s *Server) compileJQL(ctx context.Context, sub *perm.Subject, src string) (string, []any, string, error) {
	q, err := jql.Parse(src)
	if err != nil {
		return "", nil, "", jqlError(err)
	}
	byName := map[string]jql.Field{}
	if err := sqlq.Each(ctx, s.db, `SELECT id, name, type_key FROM custom_fields`, func(rows *sql.Rows) error {
		var f jql.Field
		if err := rows.Scan(&f.ID, &f.Name, &f.TypeKey); err != nil {
			return err
		}
		byName[foldName(f.Name)] = f
		byName[strconv.FormatInt(f.ID, 10)] = f
		return nil
	}); err != nil {
		return "", nil, "", err
	}
	where, args, order, err := jql.Compile(q, jql.Env{
		Me:  sub.UserID,
		Now: time.Now(),
		Loc: s.zone(),
		CustomField: func(name string) (jql.Field, bool) {
			f, ok := byName[foldName(name)]
			return f, ok
		},
		Query: func(q string, args []any) ([]string, error) {
			var out []string
			err := sqlq.Each(ctx, s.db, q, func(rows *sql.Rows) error {
				var v sql.NullString
				if err := rows.Scan(&v); err != nil {
					return err
				}
				if v.Valid {
					out = append(out, v.String)
				}
				return nil
			}, args...)
			return out, err
		},
	})
	if err != nil {
		return "", nil, "", jqlError(err)
	}
	return where, args, order, nil
}

// jqlError answers a query the server cannot read: 400 with the message and
// where in the query (in characters) the problem is, when known.
func jqlError(err error) error {
	var je *jql.Error
	if !errors.As(err, &je) {
		return err
	}
	return &httpError{status: http.StatusBadRequest, msg: i18n.M(i18n.JQLError, i18n.M(je.Key, je.Args...)),
		extra: map[string]any{"pos": je.Pos}}
}

// foldName compares field names the forgiving way: case and French accents
// do not matter ("Fonctionnalité" finds "Fonctionnalite").
func foldName(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case 'à', 'â', 'ä':
			return 'a'
		case 'é', 'è', 'ê', 'ë':
			return 'e'
		case 'î', 'ï':
			return 'i'
		case 'ô', 'ö':
			return 'o'
		case 'ù', 'û', 'ü':
			return 'u'
		case 'ç':
			return 'c'
		}
		return r
	}, strings.ToLower(strings.TrimSpace(s)))
}
