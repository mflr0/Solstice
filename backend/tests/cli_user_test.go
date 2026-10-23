package tests

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"solstice/internal/api"
	"solstice/internal/config"
	"solstice/internal/db"
)

// The user command as the README documents it, on a new database: the
// accounts it creates and the passwords it sets sign in through the API.
func TestUserCommand(t *testing.T) {
	needDB(t)
	dsn := emptyDB(t)
	bin := filepath.Join(t.TempDir(), "solstice")
	if out, err := exec.Command("go", "build", "-o", bin, "solstice/cmd/solstice").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	env := append(os.Environ(), config.KeyDBURL+"="+dsn, config.KeyConfig+"="+filepath.Join(t.TempDir(), "absent.conf"))
	run := func(args ...string) (string, error) {
		t.Helper()
		cmd := exec.Command(bin, append([]string{"user"}, args...)...)
		cmd.Env = env
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err := cmd.Run()
		if err != nil {
			err = fmt.Errorf("%w: %s", err, stderr.String())
		}
		return stdout.String(), err
	}
	mustRun := func(args ...string) string {
		t.Helper()
		out, err := run(args...)
		if err != nil {
			t.Fatalf("solstice user %s: %v", strings.Join(args, " "), err)
		}
		return out
	}
	printed := regexp.MustCompile(`(?m)^Password: (\S+)$`)
	generated := func(out string) string {
		t.Helper()
		m := printed.FindStringSubmatch(out)
		if m == nil {
			t.Fatalf("no generated password in %q", out)
		}
		return m[1]
	}

	// The first command creates the schema; the server opens the database after it.
	out := mustRun("add", "-username", "Me", "-name", "Me", "-admin", "-password", "me-password-1")
	if out != "Account \"me\" created.\n" {
		t.Errorf("add with a password printed %q", out)
	}
	conn, err := db.Open(context.Background(), dsn, config.Defaults().DB.Pool())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	srv := httptest.NewServer(api.New(conn, t.TempDir(), time.UTC, quiet()).Handler(http.NotFoundHandler()))
	t.Cleanup(srv.Close)
	base := srv.URL

	type account struct {
		Username, Name, Email string
		Admin                 bool
	}
	me := func(c *client) account {
		t.Helper()
		var a account
		if status := c.get("/api/me", &a); status != 200 {
			t.Fatalf("GET /api/me: %d", status)
		}
		return a
	}
	refused := func(username, password string) {
		t.Helper()
		if status, body := anonymousAt(t, base).do("POST", "/api/login",
			map[string]string{"username": username, "password": password}); status == 200 {
			t.Errorf("%s signs in with %q: %s", username, password, body)
		}
	}
	audited := func(login string, want ...string) {
		t.Helper()
		var got []string
		rows, err := conn.Query(`SELECT action, details FROM audit_log
			WHERE target_type = 'user' AND target_label = ? ORDER BY id`, login)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		for rows.Next() {
			var action, details string
			if err := rows.Scan(&action, &details); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(details, `"via":"command line"`) {
				t.Errorf("%s of %s: details %s", action, login, details)
			}
			got = append(got, action)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("audit of %s: %v, want %v", login, got, want)
		}
	}

	if got := me(loginAt(t, base, "me", "me-password-1")); got != (account{"me", "Me", "", true}) {
		t.Errorf("README add: %+v", got)
	}
	audited("me", "user.create", "admin.grant")

	bobPassword := generated(mustRun("add", "-username", "bob", "-email", "bob@example.com"))
	bob := loginAt(t, base, "bob", bobPassword)
	if got := me(bob); got != (account{"bob", "bob", "bob@example.com", false}) {
		t.Errorf("add with a generated password: %+v", got)
	}
	audited("bob", "user.create")

	if _, err := run("add", "-username", "BOB", "-password", "other-password-1"); err == nil {
		t.Error("a second bob was created")
	}

	out = mustRun("passwd", "-username", "bob", "-password", "bob-password-2")
	if out != "Password for \"bob\" changed.\n" {
		t.Errorf("passwd with a password printed %q", out)
	}
	if status := bob.get("/api/me", nil); status != 401 {
		t.Errorf("bob's session after passwd: %d, want 401", status)
	}
	refused("bob", bobPassword)
	if got := me(loginAt(t, base, "bob", "bob-password-2")); got.Admin {
		t.Error("passwd without -admin made bob an administrator")
	}
	audited("bob", "user.create", "password.change")

	bobPassword = generated(mustRun("passwd", "-username", "bob", "-admin"))
	refused("bob", "bob-password-2")
	if got := me(loginAt(t, base, "bob", bobPassword)); !got.Admin {
		t.Error("passwd -admin left bob a plain user")
	}
	audited("bob", "user.create", "password.change", "password.change", "admin.grant")

	if _, err := run("passwd", "-username", "nobody"); err == nil || !strings.Contains(err.Error(), `unknown user "nobody"`) {
		t.Errorf("passwd of an unknown user: %v", err)
	}
}
