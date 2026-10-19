package jirafixture

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"solstice/internal/jiradb"
)

// Writer inserts rows in multi-row batches. Like bufio.Writer it keeps the
// first error and turns every later call into a no-op, so seeding code can
// stay linear; check Close.
type Writer struct {
	ctx    context.Context
	tx     *sql.Tx
	eng    jiradb.Engine
	model  *Model
	tables []*Table
	err    error

	// Rows counts the rows written per table (lower-case name).
	Rows map[string]int
}

// NewWriter writes into tx.
func NewWriter(ctx context.Context, tx *sql.Tx, eng jiradb.Engine, model *Model) *Writer {
	return &Writer{ctx: ctx, tx: tx, eng: eng, model: model, Rows: map[string]int{}}
}

// Table is a batch of rows for one table and a fixed list of columns.
type Table struct {
	w     *Writer
	name  string // lower-case table name, for Rows
	head  string // INSERT INTO t (c1, c2) VALUES
	ncols int
	max   int // rows per statement
	vals  []any
	n     int
}

// Table prepares inserts into table with the given columns. Unknown tables
// or columns are programming errors and panic: the fixture must stay in
// sync with the real schema.
func (w *Writer) Table(table string, cols ...string) *Table {
	e := w.model.Entity(table)
	if e == nil {
		panic(fmt.Sprintf("jirafixture: table %s is not in the entity model", table))
	}
	quoted := make([]string, len(cols))
	for i, c := range cols {
		f, ok := e.Column(c)
		if !ok {
			panic(fmt.Sprintf("jirafixture: column %s.%s is not in the entity model", e.Table, c))
		}
		quoted[i] = w.eng.QuoteFolded(f.Column)
	}
	t := &Table{
		w:     w,
		name:  strings.ToLower(e.Table),
		head:  "INSERT INTO " + w.eng.QuoteFolded(e.Table) + " (" + strings.Join(quoted, ", ") + ") VALUES ",
		ncols: len(cols),
		// Both engines accept up to 65535 bind parameters per statement.
		max: min(500, 60000/len(cols)),
	}
	w.tables = append(w.tables, t)
	return t
}

// AOTable prepares inserts into a Jira Software (Active Objects) table, which
// is not part of the entity model; see agile.go.
func (w *Writer) AOTable(table string, cols ...string) *Table {
	spec := aoTable(table)
	quoted := make([]string, len(cols))
	for i, c := range cols {
		if !spec.has(c) {
			panic(fmt.Sprintf("jirafixture: column %s.%s is not an AO column", table, c))
		}
		quoted[i] = w.eng.Quote(c)
	}
	t := &Table{
		w:     w,
		name:  strings.ToLower(table),
		head:  "INSERT INTO " + w.eng.Quote(table) + " (" + strings.Join(quoted, ", ") + ") VALUES ",
		ncols: len(cols),
		max:   min(500, 60000/len(cols)),
	}
	w.tables = append(w.tables, t)
	return t
}

// Add queues one row; values must follow the column order given to Table.
func (t *Table) Add(vals ...any) {
	if len(vals) != t.ncols {
		panic(fmt.Sprintf("jirafixture: %s: %d values for %d columns", t.name, len(vals), t.ncols))
	}
	t.vals = append(t.vals, vals...)
	t.n++
	if t.n == t.max {
		t.flush()
	}
}

func (t *Table) flush() {
	w := t.w
	if t.n == 0 || w.err != nil {
		t.vals, t.n = t.vals[:0], 0
		return
	}
	var b strings.Builder
	b.WriteString(t.head)
	p := 1
	for r := range t.n {
		if r > 0 {
			b.WriteString(", ")
		}
		b.WriteByte('(')
		for c := range t.ncols {
			if c > 0 {
				b.WriteString(", ")
			}
			b.WriteString(w.eng.Placeholder(p))
			p++
		}
		b.WriteByte(')')
	}
	if _, err := w.tx.ExecContext(w.ctx, b.String(), t.vals...); err != nil {
		w.err = fmt.Errorf("insert into %s: %w", t.name, err)
	} else {
		w.Rows[t.name] += t.n
	}
	t.vals, t.n = t.vals[:0], 0
}

// updateSQL builds an UPDATE of the row of e whose id column is id; set is a
// list of column name, value pairs.
func updateSQL(eng jiradb.Engine, e *Entity, id int64, set []any) (string, []any) {
	var b strings.Builder
	b.WriteString("UPDATE " + eng.QuoteFolded(e.Table) + " SET ")
	args := make([]any, 0, len(set)/2+1)
	for i := 0; i < len(set); i += 2 {
		f, ok := e.Column(set[i].(string))
		if !ok {
			panic(fmt.Sprintf("jirafixture: column %s.%s is not in the entity model", e.Table, set[i]))
		}
		if i > 0 {
			b.WriteString(", ")
		}
		args = append(args, set[i+1])
		b.WriteString(eng.QuoteFolded(f.Column) + " = " + eng.Placeholder(len(args)))
	}
	f, ok := e.Column("id")
	if !ok {
		panic(fmt.Sprintf("jirafixture: table %s has no id column", e.Table))
	}
	args = append(args, id)
	b.WriteString(" WHERE " + eng.QuoteFolded(f.Column) + " = " + eng.Placeholder(len(args)))
	return b.String(), args
}

// Update rewrites columns of a row written earlier, for the quirks that change
// what was generated (messy.go). Pending batches are written first.
func (w *Writer) Update(table string, id int64, set ...any) {
	e := w.model.Entity(table)
	if e == nil {
		panic(fmt.Sprintf("jirafixture: table %s is not in the entity model", table))
	}
	for _, t := range w.tables {
		t.flush()
	}
	if w.err != nil {
		return
	}
	q, args := updateSQL(w.eng, e, id, set)
	if _, err := w.tx.ExecContext(w.ctx, q, args...); err != nil {
		w.err = fmt.Errorf("update %s: %w", e.Table, err)
	}
}

// Fail records an error that did not come from the database.
func (w *Writer) Fail(err error) {
	if w.err == nil {
		w.err = err
	}
}

// Err returns the first error recorded so far.
func (w *Writer) Err() error { return w.err }

// Close flushes every pending batch and returns the first error.
func (w *Writer) Close() error {
	for _, t := range w.tables {
		t.flush()
	}
	return w.err
}
