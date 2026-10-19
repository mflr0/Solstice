// Package settings keeps instance-wide settings in the database, one value
// per name: plain text or JSON. Features own their names ("mail"…); secrets
// inside a value are sealed by the caller (package secret).
package settings

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// Store reads and writes settings.
type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store { return &Store{db: db} }

// Get returns a setting's value; ok is false when it was never set.
func (s *Store) Get(ctx context.Context, name string) (value string, ok bool, err error) {
	err = s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE name = ?`, name).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return value, err == nil, err
}

// Set writes a setting.
func (s *Store) Set(ctx context.Context, name, value string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO settings (name, value, updated_at) VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE value = VALUES(value), updated_at = VALUES(updated_at)`, name, value, time.Now().UTC())
	return err
}

// GetJSON decodes a JSON setting into v; ok is false (and v untouched) when
// it was never set.
func (s *Store) GetJSON(ctx context.Context, name string, v any) (bool, error) {
	raw, ok, err := s.Get(ctx, name)
	if err != nil || !ok {
		return false, err
	}
	return true, json.Unmarshal([]byte(raw), v)
}

// SetJSON writes v as a JSON setting.
func (s *Store) SetJSON(ctx context.Context, name string, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return s.Set(ctx, name, string(raw))
}
