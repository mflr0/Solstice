package tests

// The password rules on every way of choosing a password.

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"solstice/internal/api"
)

// passwordEntry is a way of choosing a password, for one account.
type passwordEntry struct {
	username string
	min      int
	set      func(password string) (int, string)
}

// Every entry point applies the same rules: the minimum length in effect,
// and not the username, whatever its case and surrounding spaces.
func TestPasswordRulesOnEveryEntryPoint(t *testing.T) {
	e := newLinkEnv(t)
	clean := func() {
		testDB.Exec(`DELETE FROM users WHERE username LIKE 'pwrule-%'`)
		testDB.Exec(`DELETE FROM audit_log WHERE target_label LIKE 'pwrule-%' OR username LIKE 'pwrule-%'`)
	}
	clean()
	t.Cleanup(clean)
	account := func(name, password string) int64 {
		var out struct{ ID int64 }
		e.admin.call("POST", "/api/admin/users", map[string]any{"username": name, "email": name + "@exemple.fr",
			"password": password}, 201, &out)
		return out.ID
	}

	for _, tc := range []struct {
		name  string
		start func(t *testing.T) passwordEntry
	}{
		{"setup wizard", func(t *testing.T) passwordEntry {
			dsn := emptyDB(t)
			s, c := unlocked(t, api.SetupOptions{Done: func(api.SetupResult) {}})
			if status, body := c.call("POST", "/api/setup/database", "", dbFields(t, dsn)); status != 200 {
				t.Fatalf("database: %d %v", status, body)
			}
			// A minimum other than the default: the wizard follows the setting.
			if _, err := s.DB().Exec(`INSERT INTO settings (name, value, updated_at)
				VALUES ('general.password_min_length', '20', NOW(3))`); err != nil {
				t.Fatal(err)
			}
			username := "pwrule-setup-administrateur"
			return passwordEntry{username, 20, func(p string) (int, string) {
				status, body := c.call("POST", "/api/setup/admin", "en", map[string]string{"username": username,
					"email": username + "@exemple.fr", "password": p})
				msg, _ := body["error"].(string)
				return status, msg
			}}
		}},
		{"account creation", func(t *testing.T) passwordEntry {
			username := "pwrule-admin-creation"
			return passwordEntry{username, 12, func(p string) (int, string) {
				status, msg, _ := e.admin.errorIn("en", "POST", "/api/admin/users", map[string]any{"username": username,
					"email": username + "@exemple.fr", "password": p})
				return status, msg
			}}
		}},
		{"administrator change", func(t *testing.T) passwordEntry {
			username := "pwrule-admin-change"
			id := account(username, linkPassword)
			return passwordEntry{username, 12, func(p string) (int, string) {
				status, msg, _ := e.admin.errorIn("en", "PATCH", "/api/admin/users/"+strconv.FormatInt(id, 10),
					map[string]any{"password": p})
				return status, msg
			}}
		}},
		{"profile", func(t *testing.T) passwordEntry {
			username := "pwrule-profile-change"
			account(username, linkPassword)
			c := e.login(username, linkPassword)
			return passwordEntry{username, 12, func(p string) (int, string) {
				status, msg, _ := c.errorIn("en", "PATCH", "/api/me", map[string]any{"currentPassword": linkPassword,
					"newPassword": p})
				return status, msg
			}}
		}},
		{"reset link", func(t *testing.T) passwordEntry {
			username := "pwrule-reset-link"
			account(username, linkPassword)
			if status, body := e.forgot(e.anon(), username); status != 200 && status != 202 {
				t.Fatalf("forgot: %d %s", status, body)
			}
			token := e.mustMailOne(username)
			return passwordEntry{username, 12, func(p string) (int, string) {
				status, msg, _ := e.anon().errorIn("en", "POST", "/api/auth/reset", map[string]string{"token": token,
					"password": p})
				return status, msg
			}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := tc.start(t)
			tooShort := fmt.Sprintf("at least %d characters", p.min)
			for _, c := range []struct{ password, want string }{
				{strings.Repeat("a", p.min-1), tooShort},
				{"é" + strings.Repeat("a", p.min-2), tooShort}, // characters, not bytes
				{p.username, "cannot be the username"},
				{" " + strings.ToUpper(p.username) + " ", "cannot be the username"},
			} {
				if status, msg := p.set(c.password); status != 400 || !strings.Contains(msg, c.want) {
					t.Errorf("%q: %d %q, want 400 %q", c.password, status, msg, c.want)
				}
			}
			// Last: a good password is taken, and ends the flow or changes the password.
			if status, msg := p.set("une phrase de passe longue"); status >= 300 {
				t.Errorf("a good password: %d %q", status, msg)
			}
		})
	}
}
