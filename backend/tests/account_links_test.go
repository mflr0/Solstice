package tests

// Integration tests of the links sent by e-mail: forgotten passwords,
// resets sent by an administrator, invitations. Each test runs its own
// server with e-mail on (the fake SMTP server of notify_test.go).

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"solstice/internal/auth"
	"solstice/internal/config"
	"solstice/internal/mail/mailtest"
)

const linkPassword = "correct horse battery"

var linkPattern = regexp.MustCompile(`https://tickets\.exemple\.fr/reset/([A-Za-z0-9_-]+)`)

// linkEnv is a notification environment for the account link tests.
type linkEnv struct {
	*notifyEnv
	admin *client
}

// newLinkEnv starts an environment whose answers skip the floor.
func newLinkEnv(t *testing.T) *linkEnv {
	e := newTimedLinkEnv(t)
	e.hooks.SkipDelays()
	return e
}

// newTimedLinkEnv starts an environment whose answers keep the floor.
func newTimedLinkEnv(t *testing.T) *linkEnv {
	e := newNotifyEnv(t)
	clean := func() {
		testDB.Exec(`DELETE FROM users WHERE username LIKE 'wp7-%'`)
		testDB.Exec(`DELETE FROM audit_log WHERE target_label LIKE 'wp7-%' OR username LIKE 'wp7-%'`)
	}
	clean()
	t.Cleanup(clean)
	return &linkEnv{notifyEnv: e, admin: e.login("admin", "admin")}
}

func (e *linkEnv) anon() *client { return anonymousAt(e.t, e.base) }

// account creates an account with a password and an address.
func (e *linkEnv) account(name, email string) int64 {
	e.t.Helper()
	var out struct {
		ID int64 `json:"id"`
	}
	e.admin.call("POST", "/api/admin/users", map[string]any{"username": name, "name": "Test " + name, "email": email,
		"password": linkPassword}, 201, &out)
	return out.ID
}

// sent sends what the outbox holds and returns the e-mails received.
func (e *linkEnv) sent() []mailtest.Message {
	e.t.Helper()
	if err := e.hooks.SendMail(context.Background()); err != nil {
		e.t.Fatal(err)
	}
	msgs := e.smtp.Messages()
	e.smtp.Reset()
	return msgs
}

// forgot asks for a reset link and returns the status and body.
func (e *linkEnv) forgot(c *client, who string) (int, string) {
	status, body := c.do("POST", "/api/auth/forgot", map[string]string{"identifier": who})
	return status, string(body)
}

// tokenIn returns the token of the link in an e-mail.
func tokenIn(t *testing.T, m mailtest.Message) string {
	t.Helper()
	got := linkPattern.FindStringSubmatch(m.Text)
	if got == nil {
		t.Fatalf("no link in the e-mail:\n%s", m.Text)
	}
	if h := linkPattern.FindStringSubmatch(m.HTML); h == nil || h[1] != got[1] {
		t.Fatalf("the HTML part has not the same link as the text part:\n%s", m.HTML)
	}
	return got[1]
}

func (e *linkEnv) mustMailOne(who string) string {
	e.t.Helper()
	msgs := e.sent()
	if len(msgs) != 1 {
		e.t.Fatalf("%s: %d e-mails received, want 1", who, len(msgs))
	}
	return tokenIn(e.t, msgs[0])
}

func (e *linkEnv) can(c *client, token, password string, want int) {
	e.t.Helper()
	c.call("POST", "/api/auth/reset", map[string]string{"token": token, "password": password}, want, nil)
}

func (e *linkEnv) canSignIn(name, password string) bool {
	status, _ := e.anon().do("POST", "/api/login", map[string]string{"username": name, "password": password})
	return status == 200
}

func TestForgotAnswersTheSameForEveryone(t *testing.T) {
	e := newLinkEnv(t)
	e.account("wp7-known", "known@exemple.fr")
	e.account("wp7-nomail", "")
	c := e.anon()

	known, knownBody := e.forgot(c, "wp7-known")
	if known != 200 {
		t.Fatalf("known account: %d %s", known, knownBody)
	}
	if msgs := e.sent(); len(msgs) != 1 || msgs[0].To != "known@exemple.fr" {
		t.Fatalf("known account: %d e-mails (%v), want one to its address", len(msgs), msgs)
	}
	for _, who := range []string{"wp7-unknown", "wp7-nomail", "nobody@exemple.fr", "WP7-KNOWN2 "} {
		status, body := e.forgot(e.anon(), who)
		if status != known || body != knownBody {
			t.Errorf("%q: %d %s, want the answer of a known account: %d %s", who, status, body, known, knownBody)
		}
	}
	if msgs := e.sent(); len(msgs) != 0 {
		t.Fatalf("%d e-mails sent for accounts that do not exist or have no address", len(msgs))
	}
	if status, _ := e.forgot(c, "  "); status != 400 {
		t.Fatalf("an empty identifier: %d, want 400", status)
	}
}

func TestForgotByAddressAndCase(t *testing.T) {
	e := newLinkEnv(t)
	e.account("wp7-byaddress", "By.Address@exemple.fr")
	if status, body := e.forgot(e.anon(), " BY.ADDRESS@exemple.fr "); status != 200 {
		t.Fatalf("%d %s", status, body)
	}
	msgs := e.sent()
	if len(msgs) != 1 || msgs[0].To != "By.Address@exemple.fr" {
		t.Fatalf("e-mails: %v, want one to the stored address", msgs)
	}
}

// With e-mail off the answer is the same and nothing is created.
func TestForgotWithoutMailSetUp(t *testing.T) {
	e := newLinkEnv(t)
	e.account("wp7-nomailsetup", "nms@exemple.fr")
	_, want := e.forgot(e.anon(), "wp7-unknown")
	e.admin.call("PUT", "/api/admin/mail", map[string]any{"enabled": false, "host": e.smtp.Host, "port": e.smtp.Port,
		"security": "none", "from": "solstice@exemple.fr", "fromName": "Solstice"}, 200, nil)
	status, body := e.forgot(e.anon(), "wp7-nomailsetup")
	if status != 200 || body != want {
		t.Fatalf("%d %s, want 200 %s", status, body, want)
	}
	var n int
	testDB.QueryRow(`SELECT COUNT(*) FROM account_tokens t JOIN users u ON u.id = t.user_id WHERE u.username = 'wp7-nomailsetup'`).Scan(&n)
	if n != 0 {
		t.Fatalf("%d tokens created while e-mail is off", n)
	}
	var pub struct {
		EmailLinks bool `json:"emailLinks"`
	}
	e.anon().call("GET", "/api/settings/public", nil, 200, &pub)
	if pub.EmailLinks {
		t.Error("the public settings announce e-mail links while e-mail is off")
	}
	// The admin options are refused too.
	e.admin.call("POST", "/api/admin/users", map[string]any{"username": "wp7-inv-off", "email": "x@exemple.fr", "invite": true},
		409, nil)
}

func TestPublicSettingsAnnounceEmailLinks(t *testing.T) {
	e := newLinkEnv(t)
	var pub struct {
		EmailLinks bool `json:"emailLinks"`
	}
	e.anon().call("GET", "/api/settings/public", nil, 200, &pub)
	if !pub.EmailLinks {
		t.Error("e-mail is on and the public address set, but the public settings say links are not available")
	}
}

// The floor makes the answer take as long for a matching account as for any
// other.
func TestForgotAnswerTakesAtLeastTheFloor(t *testing.T) {
	e := newTimedLinkEnv(t)
	e.account("wp7-slow", "slow@exemple.fr")
	for _, who := range []string{"wp7-slow", "wp7-nobody"} {
		start := time.Now()
		e.forgot(e.anon(), who)
		if d := time.Since(start); d < e.hooks.ForgotFloor() {
			t.Errorf("%s: answered in %v, under the floor of %v", who, d, e.hooks.ForgotFloor())
		}
	}
}

func TestResetFlowEndToEnd(t *testing.T) {
	e := newLinkEnv(t)
	id := e.account("wp7-flow", "flow@exemple.fr")
	_ = id

	// An open session and a personal access token, both to be revoked.
	old := e.login("wp7-flow", linkPassword)
	var pat struct {
		Token string `json:"token"`
	}
	old.call("POST", "/api/me/tokens", map[string]any{"name": "script"}, 201, &pat)
	bearer := func() int {
		req, _ := http.NewRequest("GET", e.base+"/api/me", nil)
		req.Header.Set("Authorization", "Bearer "+pat.Token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if bearer() != 200 {
		t.Fatal("the token does not work before the reset")
	}

	// The link is built from the public address, whatever Host says.
	req, _ := http.NewRequest("POST", e.base+"/api/auth/forgot", strings.NewReader(`{"identifier":"wp7-flow"}`))
	req.Host = "evil.example"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(csrfHeader, "1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	msgs := e.sent()
	if len(msgs) != 1 {
		t.Fatalf("%d e-mails, want 1", len(msgs))
	}
	if strings.Contains(msgs[0].Raw, "evil.example") {
		t.Fatal("the e-mail uses the Host header")
	}
	token := tokenIn(t, msgs[0])
	if len(token) < 43 {
		t.Fatalf("token of %d characters: under 32 bytes", len(token))
	}
	if !strings.Contains(msgs[0].Subject, "Solstice") {
		t.Errorf("subject %q", msgs[0].Subject)
	}

	// Only a hash is stored, in the table and in the outbox.
	sum := sha256.Sum256([]byte(token))
	var n int
	testDB.QueryRow(`SELECT COUNT(*) FROM account_tokens WHERE token_hash = ?`, hex.EncodeToString(sum[:])).Scan(&n)
	if n != 1 {
		t.Fatalf("hash of the token found %d times", n)
	}
	testDB.QueryRow(`SELECT COUNT(*) FROM account_tokens WHERE token_hash = ?`, token).Scan(&n)
	if n != 0 {
		t.Fatal("the token itself is stored")
	}
	var body string
	testDB.QueryRow(`SELECT CONCAT(text_body, html_body) FROM mail_outbox WHERE to_addr = 'flow@exemple.fr' ORDER BY id DESC LIMIT 1`).Scan(&body)
	if body != "" {
		t.Fatal("the sent message still sits in the outbox with its link")
	}

	// The page checks the link, then a too short password is refused without
	// using it up.
	anon := e.anon()
	var info struct {
		Purpose, Name, Username string
	}
	anon.call("GET", "/api/auth/reset/"+token, nil, 200, &info)
	if info.Purpose != "reset" || info.Username != "wp7-flow" {
		t.Fatalf("link info %+v", info)
	}
	e.can(anon, token, "short", 400)
	e.can(anon, token, "wp7-flow", 400) // not the user name either (and too short)

	newPassword := "a brand new passphrase"
	var me struct {
		Username string `json:"username"`
	}
	anon.call("POST", "/api/auth/reset", map[string]string{"token": token, "password": newPassword}, 200, &me)
	if me.Username != "wp7-flow" {
		t.Fatalf("signed in as %q", me.Username)
	}
	anon.call("GET", "/api/me", nil, 200, nil) // the answer carried a session

	if status, _ := old.do("GET", "/api/me", nil); status != 401 {
		t.Errorf("the old session still works: %d", status)
	}
	if bearer() != 401 {
		t.Error("the personal access token still works")
	}
	if e.canSignIn("wp7-flow", linkPassword) {
		t.Error("the old password still works")
	}
	if !e.canSignIn("wp7-flow", newPassword) {
		t.Error("the new password does not work")
	}

	// Single use.
	e.can(e.anon(), token, "yet another passphrase", 400)
	if status, _ := e.anon().do("GET", "/api/auth/reset/"+token, nil); status != 400 {
		t.Errorf("a used link is still announced valid: %d", status)
	}
	if !e.canSignIn("wp7-flow", newPassword) {
		t.Error("a second use changed the password")
	}

	// Audited, without the token or the password.
	var details string
	err = testDB.QueryRow(`SELECT COALESCE(details, '') FROM audit_log WHERE action = 'password.reset_done' AND target_label = 'wp7-flow'
		ORDER BY id DESC LIMIT 1`).Scan(&details)
	if err != nil {
		t.Fatalf("no audit entry for the reset: %v", err)
	}
	if strings.Contains(details, token) || strings.Contains(details, newPassword) {
		t.Fatalf("the audit entry holds a secret: %s", details)
	}
	var raised int
	testDB.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action = 'password.reset_request' AND target_label = 'wp7-flow'`).Scan(&raised)
	if raised != 1 {
		t.Errorf("%d audit entries for the request, want 1", raised)
	}
}

func TestResetLinkExpires(t *testing.T) {
	e := newLinkEnv(t)
	e.account("wp7-expiry", "expiry@exemple.fr")
	e.forgot(e.anon(), "wp7-expiry")
	token := e.mustMailOne("expiry")

	var created, expires time.Time
	testDB.QueryRow(`SELECT t.created_at, t.expires_at FROM account_tokens t JOIN users u ON u.id = t.user_id
		WHERE u.username = 'wp7-expiry'`).Scan(&created, &expires)
	if d := expires.Sub(created); d != time.Hour {
		t.Fatalf("a reset link lasts %v, want 1h", d)
	}
	e.anon().call("GET", "/api/auth/reset/"+token, nil, 200, nil)
	if _, err := testDB.Exec(`UPDATE account_tokens SET expires_at = ? WHERE token_hash = ?`,
		time.Now().UTC().Add(-time.Minute), tokenHashOf(token)); err != nil {
		t.Fatal(err)
	}
	if status, _ := e.anon().do("GET", "/api/auth/reset/"+token, nil); status != 400 {
		t.Errorf("an expired link is announced valid: %d", status)
	}
	e.can(e.anon(), token, "a brand new passphrase", 400)
	if !e.canSignIn("wp7-expiry", linkPassword) {
		t.Error("an expired link changed the password")
	}
}

func tokenHashOf(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// postdate makes the tokens of an account look older, past the spacing
// between two requests.
func postdate(t *testing.T, name string) {
	t.Helper()
	if _, err := testDB.Exec(`UPDATE account_tokens SET created_at = created_at - INTERVAL 5 MINUTE
		WHERE user_id = (SELECT id FROM users WHERE username = ?)`, name); err != nil {
		t.Fatal(err)
	}
}

func TestNewerLinkReplacesTheOlder(t *testing.T) {
	e := newLinkEnv(t)
	e.account("wp7-newer", "newer@exemple.fr")
	e.forgot(e.anon(), "wp7-newer")
	first := e.mustMailOne("first")

	// Asking again at once mails nothing: a stream of requests cannot keep
	// replacing the link of its owner.
	e.forgot(e.anon(), "wp7-newer")
	if msgs := e.sent(); len(msgs) != 0 {
		t.Fatalf("%d e-mails for a second request within the minute", len(msgs))
	}
	e.anon().call("GET", "/api/auth/reset/"+first, nil, 200, nil)

	postdate(t, "wp7-newer")
	e.forgot(e.anon(), "wp7-newer")
	second := e.mustMailOne("second")
	if first == second {
		t.Fatal("two links with the same token")
	}
	if status, _ := e.anon().do("GET", "/api/auth/reset/"+first, nil); status != 400 {
		t.Errorf("the older link still works: %d", status)
	}
	e.anon().call("GET", "/api/auth/reset/"+second, nil, 200, nil)
}

func TestPasswordOrAddressChangeKillsLinks(t *testing.T) {
	e := newLinkEnv(t)
	id := e.account("wp7-change", "change@exemple.fr")
	idStr := jsonInt(id)

	issue := func() string {
		postdate(t, "wp7-change")
		e.forgot(e.anon(), "wp7-change")
		return e.mustMailOne("change")
	}
	valid := func(token string) bool {
		status, _ := e.anon().do("GET", "/api/auth/reset/"+token, nil)
		return status == 200
	}

	// The person changes their own password.
	token := issue()
	me := e.login("wp7-change", linkPassword)
	me.call("PATCH", "/api/me", map[string]any{"currentPassword": linkPassword, "newPassword": "another long passphrase"}, 200, nil)
	if valid(token) {
		t.Error("a link survives a password change")
	}
	// An administrator sets a password.
	token = issue()
	e.admin.call("PATCH", "/api/admin/users/"+idStr, map[string]any{"password": "set by the administrator"}, 204, nil)
	if valid(token) {
		t.Error("a link survives a password set by an administrator")
	}
	// The address changes: the link went to the old one.
	token = issue()
	e.admin.call("PATCH", "/api/admin/users/"+idStr, map[string]any{"email": "elsewhere@exemple.fr"}, 204, nil)
	if valid(token) {
		t.Error("a link survives a change of address")
	}
	token = issue()
	e.login("wp7-change", "set by the administrator").call("PATCH", "/api/me", map[string]any{"email": "mine@exemple.fr"}, 200, nil)
	if valid(token) {
		t.Error("a link survives a change of address by the person")
	}
}

func jsonInt(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func TestDeactivatedAccountsGetNoLink(t *testing.T) {
	e := newLinkEnv(t)
	id := e.account("wp7-off", "off@exemple.fr")
	idStr := jsonInt(id)

	// A link issued while active dies with the deactivation.
	e.forgot(e.anon(), "wp7-off")
	token := e.mustMailOne("off")
	e.admin.call("PATCH", "/api/admin/users/"+idStr, map[string]any{"active": false}, 204, nil)
	if status, _ := e.anon().do("GET", "/api/auth/reset/"+token, nil); status != 400 {
		t.Errorf("a deactivated account's link is announced valid: %d", status)
	}
	e.can(e.anon(), token, "a brand new passphrase", 400)

	postdate(t, "wp7-off")
	status, _ := e.forgot(e.anon(), "wp7-off")
	if status != 200 {
		t.Fatalf("answer %d: it must stay neutral", status)
	}
	if msgs := e.sent(); len(msgs) != 0 {
		t.Fatalf("%d e-mails to a deactivated account", len(msgs))
	}
	// Nor can an administrator send one.
	e.admin.call("POST", "/api/admin/users/"+idStr+"/reset-password", map[string]string{"method": "link"}, 409, nil)
	e.admin.call("POST", "/api/admin/users/"+idStr+"/invite", nil, 409, nil)
}

func TestAccountWithoutPasswordCanUseTheLink(t *testing.T) {
	e := newLinkEnv(t)
	e.account("wp7-nopass", "nopass@exemple.fr")
	if _, err := testDB.Exec(`UPDATE users SET password_hash = NULL WHERE username = 'wp7-nopass'`); err != nil {
		t.Fatal(err)
	}
	if e.canSignIn("wp7-nopass", linkPassword) {
		t.Fatal("an account without password signs in")
	}
	e.forgot(e.anon(), "wp7-nopass")
	token := e.mustMailOne("nopass")
	e.can(e.anon(), token, "chosen after the reset", 200)
	if !e.canSignIn("wp7-nopass", "chosen after the reset") {
		t.Error("the chosen password does not work")
	}
}

func TestInvitationFlow(t *testing.T) {
	e := newLinkEnv(t)

	// Refused without an address, or with a password too.
	e.admin.call("POST", "/api/admin/users", map[string]any{"username": "wp7-inv0", "invite": true}, 400, nil)
	e.admin.call("POST", "/api/admin/users", map[string]any{"username": "wp7-inv0", "email": "inv0@exemple.fr",
		"invite": true, "password": linkPassword}, 400, nil)

	var out struct {
		ID int64 `json:"id"`
	}
	e.admin.call("POST", "/api/admin/users", map[string]any{"username": "wp7-invited", "name": "Invited Person",
		"email": "invited@exemple.fr", "invite": true}, 201, &out)
	var hasPassword bool
	testDB.QueryRow(`SELECT password_hash IS NOT NULL FROM users WHERE username = 'wp7-invited'`).Scan(&hasPassword)
	if hasPassword {
		t.Fatal("an invited account has a password")
	}
	msgs := e.sent()
	if len(msgs) != 1 || msgs[0].To != "invited@exemple.fr" {
		t.Fatalf("invitation e-mails: %v", msgs)
	}
	first := tokenIn(t, msgs[0])
	if !strings.Contains(msgs[0].Text, "wp7-invited") {
		t.Errorf("the invitation does not name the account:\n%s", msgs[0].Text)
	}
	var created, expires time.Time
	var purpose string
	testDB.QueryRow(`SELECT created_at, expires_at, purpose FROM account_tokens WHERE token_hash = ?`, tokenHashOf(first)).
		Scan(&created, &expires, &purpose)
	if d := expires.Sub(created); d != 7*24*time.Hour || purpose != "invite" {
		t.Fatalf("invitation lasts %v (%s), want 168h", d, purpose)
	}
	if e.canSignIn("wp7-invited", "anything at all, really") {
		t.Fatal("signed in without a password")
	}

	// Resent: the new link replaces the old one.
	idStr := jsonInt(out.ID)
	e.admin.call("POST", "/api/admin/users/"+idStr+"/invite", nil, 200, nil)
	second := e.mustMailOne("resent")
	if status, _ := e.anon().do("GET", "/api/auth/reset/"+first, nil); status != 400 {
		t.Errorf("the first invitation still works: %d", status)
	}
	var info struct{ Purpose string }
	e.anon().call("GET", "/api/auth/reset/"+second, nil, 200, &info)
	if info.Purpose != "invite" {
		t.Fatalf("purpose %q", info.Purpose)
	}
	// Not an administrator: no resending.
	plain := e.account("wp7-plain", "plain@exemple.fr")
	plainSession := e.login("wp7-plain", linkPassword)
	if status, _ := plainSession.do("POST", "/api/admin/users/"+jsonInt(plain)+"/invite", nil); status != 403 {
		t.Errorf("a plain user can resend invitations: %d", status)
	}

	// Accepted: signed in, and the account has a password.
	anon := e.anon()
	anon.call("POST", "/api/auth/reset", map[string]string{"token": second, "password": "my own passphrase"}, 200, nil)
	anon.call("GET", "/api/me", nil, 200, nil)
	if !e.canSignIn("wp7-invited", "my own passphrase") {
		t.Error("the chosen password does not work")
	}
	var n int
	testDB.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action = 'user.invite_accept' AND target_label = 'wp7-invited'`).Scan(&n)
	if n != 1 {
		t.Errorf("%d audit entries for the acceptance, want 1", n)
	}
	// Signed in once: no more invitations.
	e.admin.call("POST", "/api/admin/users/"+idStr+"/invite", nil, 409, nil)
}

func TestInvitationNeedsAnAddress(t *testing.T) {
	e := newLinkEnv(t)
	id := e.account("wp7-noaddr", "")
	e.admin.call("POST", "/api/admin/users/"+jsonInt(id)+"/invite", nil, 400, nil)
	e.admin.call("POST", "/api/admin/users/"+jsonInt(id)+"/reset-password", map[string]string{"method": "link"}, 400, nil)
}

func TestAdminSendsAResetLink(t *testing.T) {
	e := newLinkEnv(t)
	id := e.account("wp7-byadmin", "byadmin@exemple.fr")
	idStr := jsonInt(id)
	session := e.login("wp7-byadmin", linkPassword)

	e.admin.call("POST", "/api/admin/users/"+idStr+"/reset-password", map[string]string{"method": "link"}, 200, nil)
	token := e.mustMailOne("byadmin")
	// Nothing changes until the person acts.
	session.call("GET", "/api/me", nil, 200, nil)
	if !e.canSignIn("wp7-byadmin", linkPassword) {
		t.Fatal("sending the link changed the password")
	}
	e.can(e.anon(), token, "chosen by the person", 200)
	if status, _ := session.do("GET", "/api/me", nil); status != 401 {
		t.Errorf("the old session survived the reset: %d", status)
	}
	if !e.canSignIn("wp7-byadmin", "chosen by the person") {
		t.Error("the chosen password does not work")
	}
	var details string
	testDB.QueryRow(`SELECT COALESCE(details, '') FROM audit_log WHERE action = 'password.reset_request' AND target_label = 'wp7-byadmin'
		AND username = 'admin' ORDER BY id DESC LIMIT 1`).Scan(&details)
	if !strings.Contains(details, "administrator") {
		t.Errorf("the request is not audited under the administrator: %q", details)
	}
	// An unknown method, and the old generated password, still behave.
	e.admin.call("POST", "/api/admin/users/"+idStr+"/reset-password", map[string]string{"method": "telepathy"}, 400, nil)
	var gen struct{ Password string }
	e.admin.call("POST", "/api/admin/users/"+idStr+"/reset-password", nil, 200, &gen)
	if len(gen.Password) < 16 || !e.canSignIn("wp7-byadmin", gen.Password) {
		t.Errorf("generated password %q", gen.Password)
	}
}

func TestForgotIsRateLimited(t *testing.T) {
	e := newLinkEnv(t)
	e.account("wp7-limit", "limit@exemple.fr")

	// One address asking about many names: refused after a few, with a 429
	// that depends on the address only.
	c := e.anon()
	for i := range 5 {
		if status, body := e.forgot(c, "wp7-limit-"+jsonInt(int64(i))); status != 200 {
			t.Fatalf("request %d: %d %s", i+1, status, body)
		}
	}
	if status, _ := e.forgot(c, "wp7-limit"); status != 429 {
		t.Fatalf("sixth request from one address: %d, want 429", status)
	}
	if msgs := e.sent(); len(msgs) != 0 {
		t.Fatalf("a refused request mailed %d messages", len(msgs))
	}
}

func TestForgotIsRateLimitedPerAccount(t *testing.T) {
	e := newLinkEnv(t)
	proxies, err := config.ParseTrustedProxies("127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	e.srv.SetTrustedProxies(proxies)
	e.account("wp7-flood", "flood@exemple.fr")

	// From many addresses, one account gets a few e-mails, then none, and
	// every answer is the neutral one.
	mails := 0
	for i := range 8 {
		postdate(t, "wp7-flood") // set the one-minute spacing aside
		req, _ := http.NewRequest("POST", e.base+"/api/auth/forgot", strings.NewReader(`{"identifier":"wp7-flood"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(csrfHeader, "1")
		req.Header.Set("X-Forwarded-For", "198.51.100."+jsonInt(int64(10+i)))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("request %d: %d, want the neutral 200", i+1, resp.StatusCode)
		}
		mails += len(e.sent())
	}
	if mails == 0 || mails > 5 {
		t.Fatalf("%d e-mails for 8 requests, want a few but at most 5", mails)
	}
}

func TestGuessingLinksIsThrottled(t *testing.T) {
	e := newLinkEnv(t)
	c := e.anon()
	for i := range 5 {
		if status, _ := c.do("GET", "/api/auth/reset/not-a-token-"+jsonInt(int64(i)), nil); status != 400 {
			t.Fatalf("attempt %d: %d, want 400", i+1, status)
		}
	}
	if status, _ := c.do("GET", "/api/auth/reset/not-a-token-again", nil); status != 429 {
		t.Fatalf("sixth wrong link from one address: %d, want 429", status)
	}
}

func TestExpiredLinksArePurged(t *testing.T) {
	e := newLinkEnv(t)
	id := e.account("wp7-purge", "purge@exemple.fr")
	now := time.Now().UTC()
	for _, tok := range []struct {
		hash          string
		expires, used any
	}{
		{"p1", now.Add(-time.Minute), nil},                   // expired
		{"p2", now.Add(time.Hour), now.Add(-48 * time.Hour)}, // used long ago
		{"p3", now.Add(time.Hour), nil},                      // good
		{"p4", now.Add(time.Hour), now.Add(-time.Hour)},      // used just now: kept a day
	} {
		if _, err := testDB.Exec(`INSERT INTO account_tokens (user_id, token_hash, purpose, created_at, expires_at, used_at)
			VALUES (?, ?, 'reset', ?, ?, ?)`, id, tok.hash, now.Add(-72*time.Hour), tok.expires, tok.used); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := auth.PurgeAccountTokens(context.Background(), testDB, time.Now()); err != nil {
		t.Fatal(err)
	}
	rows, err := testDB.Query(`SELECT token_hash FROM account_tokens WHERE user_id = ? ORDER BY token_hash`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var left []string
	for rows.Next() {
		var h string
		rows.Scan(&h)
		left = append(left, h)
	}
	if strings.Join(left, ",") != "p3,p4" {
		t.Fatalf("left after the purge: %v, want p3 and p4", left)
	}
}
