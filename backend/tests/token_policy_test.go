package tests

// Tests of who may create access tokens, and of the shortest audit
// retention. They need SOLSTICE_TEST_DB and are skipped without it.

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"solstice/internal/api"
	"solstice/internal/jirafixture"
)

func TestTokenCreationPolicy(t *testing.T) {
	admin := login(t, "admin", "admin")
	paul := login(t, "paul.martin", jirafixture.Password)
	hugo := login(t, "hugo.robert", jirafixture.Password)
	t.Cleanup(func() {
		admin.do("PUT", "/api/admin/tokens/settings", map[string]any{"who": "admins"})
	})

	var policy struct {
		Who   string
		Group *struct {
			ID   int64
			Name string
		}
	}
	canCreate := func(c *client) bool {
		t.Helper()
		var out struct{ CanCreate bool }
		if status := c.get("/api/me/tokens/policy", &out); status != 200 {
			t.Fatalf("token policy: %d", status)
		}
		return out.CanCreate
	}
	create := func(c *client, name string) (int, []byte) {
		t.Helper()
		status, body := c.do("POST", "/api/me/tokens", map[string]any{"name": name})
		if status == 201 {
			id := int64(obj(t, body)["id"].(float64))
			c.do("DELETE", fmt.Sprintf("/api/me/tokens/%d", id), nil)
		}
		return status, body
	}

	// Administrators only by default.
	if status := admin.get("/api/admin/tokens/settings", &policy); status != 200 || policy.Who != "admins" {
		t.Fatalf("default policy: %d %+v", status, policy)
	}
	if status, _ := create(paul, "politique par défaut"); status != 403 || canCreate(paul) {
		t.Errorf("a user may create a token under the default policy: %d", status)
	}

	// Everyone.
	if status, body := admin.do("PUT", "/api/admin/tokens/settings", map[string]any{"who": "everyone"}); status != 200 {
		t.Fatalf("set everyone: %d %s", status, body)
	}
	if status, body := create(paul, "politique libre"); status != 201 || !canCreate(paul) {
		t.Fatalf("everyone may create: %d %s", status, body)
	}

	// Administrators only.
	if status, body := admin.do("PUT", "/api/admin/tokens/settings", map[string]any{"who": "admins"}); status != 200 {
		t.Fatalf("set admins: %d %s", status, body)
	}
	status, body := create(paul, "politique stricte")
	if status != 403 || !strings.Contains(string(body), "not allowed for your account") {
		t.Errorf("token for a user under admins-only: %d %s", status, body)
	}
	if canCreate(paul) {
		t.Error("the policy says paul.martin may create a token")
	}
	if status, body := create(admin, "toujours permis"); status != 201 || !canCreate(admin) {
		t.Errorf("administrators always may: %d %s", status, body)
	}
	// The message follows the language of the request.
	if status, msg, _ := paul.errorIn("fr", "POST", "/api/me/tokens", map[string]any{"name": "x"}); status != 403 ||
		!strings.Contains(msg, "jetons d'accès") {
		t.Errorf("French refusal: %d %q", status, msg)
	}

	// One group.
	group := fmt.Sprintf("jetons.%d", time.Now().UnixNano()%1e9)
	status, body = admin.do("POST", "/api/admin/groups", map[string]any{"name": group})
	if status != 201 {
		t.Fatalf("create group: %d %s", status, body)
	}
	groupID := int64(obj(t, body)["id"].(float64))
	t.Cleanup(func() { admin.do("DELETE", fmt.Sprintf("/api/admin/groups/%d", groupID), nil) })
	hugoID, _ := strconv.ParseInt(pick(t, `SELECT id FROM users WHERE username = 'hugo.robert'`), 10, 64)
	if status, body := admin.do("POST", fmt.Sprintf("/api/admin/groups/%d/members", groupID),
		map[string]any{"users": []int64{hugoID}}); status != 200 {
		t.Fatalf("add member: %d %s", status, body)
	}
	if status, body := admin.do("PUT", "/api/admin/tokens/settings", map[string]any{"who": "group", "group": groupID}); status != 200 {
		t.Fatalf("set group: %d %s", status, body)
	}
	if status := admin.get("/api/admin/tokens/settings", &policy); status != 200 || policy.Who != "group" ||
		policy.Group == nil || policy.Group.ID != groupID {
		t.Errorf("group policy: %d %+v", status, policy)
	}
	if status, body := create(hugo, "membre du groupe"); status != 201 {
		t.Errorf("a member of the group: %d %s", status, body)
	}
	if status, _ := create(paul, "hors du groupe"); status != 403 {
		t.Errorf("someone outside the group: %d", status)
	}

	// A bad request changes nothing.
	for name, in := range map[string]map[string]any{
		"unknown":       {"who": "nobody"},
		"group missing": {"who": "group"},
		"no such group": {"who": "group", "group": 987654321},
	} {
		if status, _ := admin.do("PUT", "/api/admin/tokens/settings", in); status != 400 {
			t.Errorf("%s: %d", name, status)
		}
	}
	if status := admin.get("/api/admin/tokens/settings", &policy); status != 200 || policy.Who != "group" {
		t.Errorf("policy after refused changes: %d %+v", status, policy)
	}
	if status, _ := paul.do("PUT", "/api/admin/tokens/settings", map[string]any{"who": "everyone"}); status != 403 {
		t.Errorf("a user changed the policy: %d", status)
	}
	if status := paul.get("/api/admin/tokens/settings", nil); status != 403 {
		t.Errorf("a user read the policy: %d", status)
	}

	// The changes are in the audit log.
	var page auditPage
	if status := admin.get("/api/admin/audit?action=settings.tokens", &page); status != 200 || page.Total < 2 {
		t.Fatalf("audit of the policy: %d %+v", status, page)
	}
	raw, _ := json.Marshal(page.Entries[0].Details)
	if !strings.Contains(string(raw), group) {
		t.Errorf("the group is not in the last entry: %s", raw)
	}

	// An unreadable policy means administrators only, even when it decodes
	// "everyone" before failing, and the failed read is logged.
	if _, err := testDB.Exec(`UPDATE settings SET value = '{"who": "everyone", "group": "x"}' WHERE name = 'tokens.creators'`); err != nil {
		t.Fatal(err)
	}
	logs := &logBuffer{}
	ts := httptest.NewServer(api.New(testDB, t.TempDir(), time.UTC, slog.New(slog.NewTextHandler(logs, nil))).
		Handler(http.NotFoundHandler()))
	t.Cleanup(ts.Close)
	if status := loginAt(t, ts.URL, "admin", "admin").get("/api/admin/tokens/settings", &policy); status != 200 ||
		policy.Who != "admins" {
		t.Errorf("unreadable policy: %d %+v", status, policy)
	}
	if out := logs.String(); !strings.Contains(out, "level=WARN") || !strings.Contains(out, "reading the token policy failed") {
		t.Errorf("failed read not logged at Warn:\n%s", out)
	}
	if status, _ := create(paul, "politique illisible"); status != 403 || canCreate(paul) {
		t.Errorf("a user may create a token under an unreadable policy: %d", status)
	}
}

func TestAuditRetentionMinimum(t *testing.T) {
	admin := login(t, "admin", "admin")
	t.Cleanup(func() {
		admin.do("PUT", "/api/admin/audit/settings", map[string]int{"retentionDays": 365})
	})
	for _, days := range []int{1, 29} {
		status, body := admin.do("PUT", "/api/admin/audit/settings", map[string]int{"retentionDays": days})
		if status != 400 || !strings.Contains(string(body), "30 days") {
			t.Errorf("%d days: %d %s", days, status, body)
		}
	}
	for _, days := range []int{30, 0} {
		if status, body := admin.do("PUT", "/api/admin/audit/settings", map[string]int{"retentionDays": days}); status != 200 {
			t.Errorf("%d days: %d %s", days, status, body)
		}
	}
}

// allowTokensForEveryone lets every account create access tokens for the
// rest of the test (the default is administrators only).
func allowTokensForEveryone(t *testing.T) {
	t.Helper()
	admin := login(t, "admin", "admin")
	if status, body := admin.do("PUT", "/api/admin/tokens/settings", map[string]any{"who": "everyone"}); status != 200 {
		t.Fatalf("set everyone: %d %s", status, body)
	}
	t.Cleanup(func() {
		admin.do("PUT", "/api/admin/tokens/settings", map[string]any{"who": "admins"})
	})
}
