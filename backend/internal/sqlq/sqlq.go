// Package sqlq holds small helpers around database/sql shared by the other
// packages.
package sqlq

import (
	"context"
	"database/sql"
	"strings"
)

// Marks returns n comma-separated bind markers, "?, ?, ?", for IN lists and
// VALUES tuples.
func Marks(n int) string { return strings.TrimSuffix(strings.Repeat("?, ", n), ", ") }

// ProjectKey is a condition on a project (alias pa) named by its key or by a
// key it had before it was renamed. marks is the bind markers of the keys, "?"
// for one key or Marks(n) for n; the upper-case keys are bound twice, first
// for the current key and then for the former ones.
func ProjectKey(pa, marks string) string {
	return "(" + pa + ".pkey IN (" + marks + ") OR " + pa + ".id IN (SELECT project_id FROM project_keys WHERE pkey IN (" + marks + ")))"
}

// Querier is what Each needs: a *sql.DB, *sql.Tx or *sql.Conn.
type Querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// Each runs a query and calls f for every row.
func Each(ctx context.Context, db Querier, q string, f func(*sql.Rows) error, args ...any) error {
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := f(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

// String returns the string, or nil when it is NULL.
func String(n sql.NullString) *string {
	if !n.Valid {
		return nil
	}
	return &n.String
}

// Int returns the integer, or nil when it is NULL.
func Int(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	return &n.Int64
}

// Value returns the plain value, or an untyped nil when it is NULL, so it
// compares with == against other values.
func Value[T sql.NullString | sql.NullInt64](n T) any {
	switch v := any(n).(type) {
	case sql.NullString:
		if v.Valid {
			return v.String
		}
	case sql.NullInt64:
		if v.Valid {
			return v.Int64
		}
	}
	return nil
}
