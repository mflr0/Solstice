package importer

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
)

// favouritesJira answers every query with its favouriteassociations rows,
// user key and filter id, in the order Jira's ORDER BY would give them.
type favouritesJira struct{ rows [][2]any }

func (f *favouritesJira) Connect(context.Context) (driver.Conn, error) { return favouritesConn{f}, nil }
func (f *favouritesJira) Driver() driver.Driver                        { return nil }

type favouritesConn struct{ *favouritesJira }

func (favouritesConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not supported") }
func (favouritesConn) Close() error                        { return nil }
func (favouritesConn) Begin() (driver.Tx, error)           { return nil, errors.New("not supported") }

func (c favouritesConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return &favouritesRows{rows: c.rows}, nil
}

type favouritesRows struct{ rows [][2]any }

func (*favouritesRows) Columns() []string { return []string{"username", "entityid"} }
func (*favouritesRows) Close() error      { return nil }
func (r *favouritesRows) Next(dest []driver.Value) error {
	if len(r.rows) == 0 {
		return io.EOF
	}
	dest[0], dest[1] = r.rows[0][0], r.rows[0][1]
	r.rows = r.rows[1:]
	return nil
}

// recorder keeps the rows of each INSERT, one line per row.
type recorder map[string][]string

func (r recorder) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	table := strings.Fields(query)[2]
	cols := strings.Count(query[:strings.Index(query, ")")], ",") + 1
	for i := 0; i < len(args); i += cols {
		row := args[i : i+cols]
		if table == "dashboards" {
			row = row[:1] // without the import's time
		}
		r[table] = append(r[table], strings.TrimSuffix(fmt.Sprintln(row...), "\n"))
	}
	return noRows{}, nil
}

// Favourites become views of My space, after the built-in ones and in
// Jira's order, up to maxGadgets; the others, the orphans and repeats are
// counted as left out.
func TestImportFavourites(t *testing.T) {
	jira := &favouritesJira{rows: [][2]any{
		{"bob", int64(11)}, {"ann", int64(12)}, {"ann", int64(11)}, {"ann", int64(11)},
		{"ghost", int64(11)}, {"bob", int64(99)},
	}}
	for f := int64(100); f < 120; f++ {
		jira.rows = append(jira.rows, [2]any{"cid", f})
	}
	src := sql.OpenDB(jira)
	defer src.Close()
	dst := recorder{}
	filters := map[int64]bool{11: true, 12: true}
	for f := int64(100); f < 120; f++ {
		filters[f] = true
	}
	r := &run{ctx: context.Background(), src: src, dst: dst, rep: &Report{},
		opt:   Options{Log: slog.Default()},
		users: map[string]int64{"ann": 1, "bob": 2, "cid": 3}}
	if err := r.importFavourites(filters); err != nil {
		t.Fatal(err)
	}

	if got := strings.Join(dst["dashboards"], ","); got != "1,2,3" {
		t.Errorf("dashboards of %s, want 1,2,3", got)
	}
	var want []string
	id := 0
	add := func(user, pos int, kind, filter, config string) {
		id++
		want = append(want, fmt.Sprintf("%d %d %d %s %s %s", id, user, pos, kind, filter, config))
	}
	builtins := func(user int) {
		add(user, 0, "assigned", "<nil>", `{"size":15}`)
		add(user, 1, "reported", "<nil>", `{"size":15}`)
		add(user, 2, "recent", "<nil>", `{"size":12}`)
	}
	builtins(1)
	add(1, 3, "results", "12", "{}")
	add(1, 4, "results", "11", "{}")
	builtins(2)
	add(2, 3, "results", "11", "{}")
	builtins(3)
	for i := range 17 {
		add(3, 3+i, "results", fmt.Sprint(100+i), "{}")
	}
	got := dst["dashboard_gadgets"]
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("gadgets:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if s := r.rep.Steps; len(s) != 1 || s[0] != (Step{"favorite filters", 20, 6}) {
		t.Errorf("steps = %+v, want 20 imported, 6 left out (repeat, ghost, unknown filter, 3 beyond the limit)", s)
	}
}
