package api

// Interface preferences stored on the account, so they follow the user
// across devices.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"unicode"
	"unicode/utf8"

	"solstice/internal/i18n"
)

const (
	maxShortcutBindings = 4  // per action
	maxBindingLength    = 32 // characters
)

// shortcutActions are the ShortcutAction names of frontend/src/core/shortcuts.ts.
var shortcutActions = []string{
	"search", "create", "help", "toggleScheme", "next", "previous", "open", "select",
	"editSummary", "assignToMe", "comment", "status", "watch",
}

// preferenceChecks check the value of each known preference.
var preferenceChecks = map[string]func(json.RawMessage) error{
	"shortcutsEnabled": checkShortcutsEnabled,
	"shortcuts":        checkShortcuts,
}

// preferenceValue checks a value for the preference name and answers it
// compacted, as stored.
func preferenceValue(name string, raw json.RawMessage) (json.RawMessage, error) {
	check, ok := preferenceChecks[name]
	if !ok {
		return nil, failf(http.StatusBadRequest, i18n.PreferenceUnknown, name)
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, failf(http.StatusBadRequest, i18n.PreferenceInvalid, name)
	}
	if err := check(raw); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := json.Compact(&out, raw); err != nil {
		return nil, failf(http.StatusBadRequest, i18n.PreferenceInvalid, name)
	}
	return out.Bytes(), nil
}

func checkShortcutsEnabled(raw json.RawMessage) error {
	var on bool
	if err := json.Unmarshal(raw, &on); err != nil {
		return failf(http.StatusBadRequest, i18n.PreferenceInvalid, "shortcutsEnabled")
	}
	return nil
}

// checkShortcuts checks the bindings that differ from the defaults: action
// name → keys. An empty list leaves the action without a key.
func checkShortcuts(raw json.RawMessage) error {
	var m map[string][]string
	if err := json.Unmarshal(raw, &m); err != nil {
		return failf(http.StatusBadRequest, i18n.PreferenceInvalid, "shortcuts")
	}
	for action, keys := range m {
		if !slices.Contains(shortcutActions, action) {
			return failf(http.StatusBadRequest, i18n.PreferenceShortcutActionUnknown, action)
		}
		if keys == nil {
			return failf(http.StatusBadRequest, i18n.PreferenceInvalid, "shortcuts")
		}
		if len(keys) > maxShortcutBindings {
			return failf(http.StatusBadRequest, i18n.PreferenceShortcutBindingsTooMany, maxShortcutBindings)
		}
		for _, k := range keys {
			if !validBinding(k) {
				return failf(http.StatusBadRequest, i18n.PreferenceShortcutBindingInvalid, maxBindingLength)
			}
		}
	}
	return nil
}

// validBinding is a key like "Alt-k" or "/": 1 to maxBindingLength
// characters, none of them blank or a control character.
func validBinding(k string) bool {
	if k == "" || utf8.RuneCountInString(k) > maxBindingLength {
		return false
	}
	for _, r := range k {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return false
		}
	}
	return true
}

// preferences are the stored preferences of a user, by name.
func (s *Server) preferences(ctx context.Context, userID int64) (map[string]json.RawMessage, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT name, value FROM user_preferences WHERE user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]json.RawMessage{}
	for rows.Next() {
		var name string
		var value []byte
		if err := rows.Scan(&name, &value); err != nil {
			return nil, err
		}
		if _, ok := preferenceChecks[name]; ok {
			out[name] = value
		}
	}
	return out, rows.Err()
}

// putPreference replaces one preference of the user's own account. Not
// audited: it changes how the interface behaves, not the account.
func (s *Server) putPreference(w http.ResponseWriter, r *http.Request) error {
	name := r.PathValue("key")
	var raw json.RawMessage
	if err := readJSON(r, &raw); err != nil {
		return err
	}
	value, err := preferenceValue(name, raw)
	if err != nil {
		return err
	}
	u := currentUser(r)
	if _, err := s.db.ExecContext(r.Context(), `INSERT INTO user_preferences (user_id, name, value) VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE value = VALUES(value)`, u.ID, name, string(value)); err != nil {
		return err
	}
	return s.writeMe(w, r, u)
}

// deletePreference puts one preference back to its default.
func (s *Server) deletePreference(w http.ResponseWriter, r *http.Request) error {
	name := r.PathValue("key")
	if _, ok := preferenceChecks[name]; !ok {
		return failf(http.StatusBadRequest, i18n.PreferenceUnknown, name)
	}
	u := currentUser(r)
	if _, err := s.db.ExecContext(r.Context(), `DELETE FROM user_preferences WHERE user_id = ? AND name = ?`,
		u.ID, name); err != nil {
		return err
	}
	return s.writeMe(w, r, u)
}
