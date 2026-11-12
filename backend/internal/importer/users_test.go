package importer

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

// tableJira answers each query with the rows of the first table it names.
type tableJira map[string][][]driver.Value

func (j tableJira) Connect(context.Context) (driver.Conn, error) { return tableConn{j}, nil }
func (j tableJira) Driver() driver.Driver                        { return nil }

type tableConn struct{ tables tableJira }

func (tableConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not supported") }
func (tableConn) Close() error                        { return nil }
func (tableConn) Begin() (driver.Tx, error)           { return nil, errors.New("not supported") }

func (c tableConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	for table, rows := range c.tables {
		if strings.Contains(query, "FROM "+table) {
			return &tableRows{rows: rows}, nil
		}
	}
	return nil, errors.New("unexpected query: " + query)
}

type tableRows struct{ rows [][]driver.Value }

func (r *tableRows) Columns() []string {
	if len(r.rows) == 0 {
		return nil
	}
	return make([]string, len(r.rows[0]))
}
func (*tableRows) Close() error { return nil }
func (r *tableRows) Next(dest []driver.Value) error {
	if len(r.rows) == 0 {
		return io.EOF
	}
	copy(dest, r.rows[0])
	r.rows = r.rows[1:]
	return nil
}

// Two Jira users with one address in different cases: the first keeps it,
// the report names the other, imported without an address.
func TestImportUsersSharedEmail(t *testing.T) {
	src := sql.OpenDB(tableJira{
		"cwd_directory": {{int64(1), int64(0), int64(1)}},
		"cwd_user": {
			{int64(1), "alice", "Alice", "alice@example.org", "", int64(1)},
			{int64(1), "alice2", "Alice Bis", "Alice@Example.org", "", int64(1)},
			{int64(1), "bob", "Bob", "", "", int64(1)},
			{int64(1), "carol", "Carol", "", "", int64(1)},
		},
		"app_user": {{"k1", "alice"}, {"k2", "alice2"}, {"k3", "bob"}, {"k4", "carol"}},
	})
	defer src.Close()
	dst := recorder{}
	r := &run{ctx: t.Context(), src: src, dst: dst, rep: &Report{}}
	if err := r.importUsers(); err != nil {
		t.Fatal(err)
	}
	// id, username, key, name, address, password, active; bob and carol share
	// having none.
	want := []string{
		"1 alice k1 Alice alice@example.org <nil> true",
		"2 alice2 k2 Alice Bis  <nil> true",
		"3 bob k3 Bob  <nil> true",
		"4 carol k4 Carol  <nil> true",
	}
	if !reflect.DeepEqual(dst["users"], want) {
		t.Errorf("users = %q, want %q", dst["users"], want)
	}
	var got []string
	for _, w := range r.rep.Warnings {
		got = append(got, w.In("en"))
	}
	if want := "the address Alice@Example.org of alice2 already belongs to another account: imported without an address"; len(got) == 0 || !strings.Contains(strings.Join(got, "\n"), want) {
		t.Errorf("warnings = %q, want one %q", got, want)
	}
}
