package tests

// The project a user scopes Work, Boards and Search to.

import (
	"database/sql"
	"testing"

	"solstice/internal/jirafixture"
)

func TestScopeProject(t *testing.T) {
	c := login(t, "paul.martin", jirafixture.Password) // developer, not in "rh"
	id := userID(t, "paul.martin")
	t.Cleanup(func() { c.do("PATCH", "/api/me", map[string]any{"project": nil}) })
	scope := func(raw []byte) any {
		t.Helper()
		v, ok := obj(t, raw)["project"]
		if !ok {
			t.Fatalf("no project in %s", raw)
		}
		if v == nil {
			return nil
		}
		return str(v, "key")
	}
	me := func() any {
		t.Helper()
		return scope(c.call("GET", "/api/me", nil, 200, nil))
	}

	if p := me(); p != nil {
		t.Fatalf("before any choice: %v", p)
	}
	if p := scope(c.call("PATCH", "/api/me", map[string]any{"project": "demo"}, 200, nil)); p != "DEMO" {
		t.Errorf("PATCH project demo answers %v", p)
	}
	if p := me(); p != "DEMO" {
		t.Errorf("read back %v, want DEMO", p)
	}
	_, body := anonymous(t).do("POST", "/api/login", map[string]string{"username": "paul.martin", "password": jirafixture.Password})
	if p := scope(body); p != "DEMO" {
		t.Errorf("login answers %v, want DEMO", p)
	}
	if status, _ := c.do("PATCH", "/api/me", map[string]any{}); status != 200 || me() != "DEMO" {
		t.Errorf("an empty PATCH changed the project (%d, %v)", status, me())
	}

	for _, bad := range []any{"RH", "NOPE", 3} {
		if status, _ := c.do("PATCH", "/api/me", map[string]any{"project": bad}); status != 400 {
			t.Errorf("PATCH project %v: %d, want 400", bad, status)
		}
	}
	if p := me(); p != "DEMO" {
		t.Errorf("after refused changes: %v, want DEMO", p)
	}

	// A project the user can no longer browse is forgotten.
	if _, err := testDB.Exec(`UPDATE users SET scope_project_id = (SELECT id FROM projects WHERE pkey = 'RH') WHERE id = ?`, id); err != nil {
		t.Fatal(err)
	}
	if p := me(); p != nil {
		t.Errorf("a project out of sight is answered: %v", p)
	}
	var stored sql.NullInt64
	if err := testDB.QueryRow(`SELECT scope_project_id FROM users WHERE id = ?`, id).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored.Valid {
		t.Errorf("scope_project_id still %d", stored.Int64)
	}

	c.call("PATCH", "/api/me", map[string]any{"project": "DEMO"}, 200, nil)
	if p := scope(c.call("PATCH", "/api/me", map[string]any{"project": nil}, 200, nil)); p != nil {
		t.Errorf("PATCH project null answers %v", p)
	}
	if p := me(); p != nil {
		t.Errorf("after clearing: %v", p)
	}
}
