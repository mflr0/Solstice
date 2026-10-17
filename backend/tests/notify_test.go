package tests

// Integration tests of e-mail notifications, filter subscriptions, due-date
// reminders and webhooks. Each runs its own server with e-mail switched on
// (a fake SMTP server), and drives the background work by hand: events
// handled, batches written as if their time had come, e-mails sent.

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"solstice/internal/api"
	"solstice/internal/mail/mailtest"
)

type notifyEnv struct {
	t     *testing.T
	srv   *api.Server
	hooks api.TestHooks
	base  string
	smtp  *mailtest.Server
}

// newNotifyEnv starts a server with e-mail on, sent to a fake SMTP server.
func newNotifyEnv(t *testing.T) *notifyEnv {
	needDB(t)
	mailtest.Lock(t, testDB)
	srv := api.New(testDB, t.TempDir(), time.UTC, quiet())
	srv.SetBaseURL("https://tickets.exemple.fr")
	ts := httptest.NewServer(srv.Handler(http.NotFoundHandler()))
	env := &notifyEnv{t: t, srv: srv, hooks: srv.TestHooks(), base: ts.URL, smtp: mailtest.Start(t)}
	reset := func() {
		for _, q := range []string{`DELETE FROM settings WHERE name = 'mail'`, `DELETE FROM notification_batches`,
			`DELETE FROM mail_outbox`, `DELETE FROM notification_prefs`, `DELETE FROM project_notifications`,
			`DELETE FROM project_notification_recipients`, `DELETE FROM filter_subscriptions`, `DELETE FROM webhooks`,
			`UPDATE users SET locale = NULL`} {
			if _, err := testDB.Exec(q); err != nil {
				t.Fatal(err)
			}
		}
	}
	reset()
	t.Cleanup(func() {
		ts.Close()
		srv.Close(context.Background())
		reset()
	})
	env.login("admin", "admin").call("PUT", "/api/admin/mail", map[string]any{"enabled": true, "host": env.smtp.Host,
		"port": env.smtp.Port, "security": "none", "from": "solstice@exemple.fr", "fromName": "Solstice"}, 200, nil)
	return env
}

func (e *notifyEnv) login(username, password string) *client {
	e.t.Helper()
	return loginAt(e.t, e.base, username, password)
}

// deliverAt handles the events published so far, writes the batches due
// within d (as if that time had come) and sends the e-mails. It returns the
// e-mails received since the last call.
func (e *notifyEnv) deliverAt(d time.Duration) []mailtest.Message {
	e.t.Helper()
	ctx := context.Background()
	e.hooks.Settle()
	if err := e.hooks.FlushNotifications(ctx, time.Now().Add(d)); err != nil {
		e.t.Fatal(err)
	}
	if err := e.hooks.SendMail(ctx); err != nil {
		e.t.Fatal(err)
	}
	msgs := e.smtp.Messages()
	e.smtp.Reset()
	return msgs
}

// deliver: what immediate notifications would send within their window.
func (e *notifyEnv) deliver() []mailtest.Message { return e.deliverAt(11 * time.Minute) }

func to(msgs []mailtest.Message, addr string) []mailtest.Message {
	var out []mailtest.Message
	for _, m := range msgs {
		if m.To == addr {
			out = append(out, m)
		}
	}
	return out
}

func recipients(msgs []mailtest.Message) []string {
	var out []string
	for _, m := range msgs {
		out = append(out, m.To)
	}
	return out
}

func TestNotificationRecipientsAndVisibility(t *testing.T) {
	env := newNotifyEnv(t)
	paul := env.login("paul.martin", "motdepasse")
	hugo := env.login("hugo.robert", "motdepasse")  // developer
	chloe := env.login("chloe.simon", "motdepasse") // rh, not a developer
	hugo.call("PATCH", "/api/me", map[string]any{"locale": "en"}, 200, nil)

	// Creation: the person mentioned in the description hears of it; the
	// author does not hear of their own change.
	key := createIssue(t, paul, "WEB", "Le formulaire <b>plante</b>", "À voir avec [~ines.david] demain.")
	id := issueID(t, key)
	msgs := env.deliver()
	if got := recipients(msgs); len(got) != 1 || got[0] != "ines.david@exemple.fr" {
		t.Fatalf("creation: mailed to %v, want ines.david only", got)
	}
	m := msgs[0]
	if m.Subject != "["+key+"] Le formulaire <b>plante</b>" {
		t.Errorf("subject %q", m.Subject)
	}
	if strings.Contains(m.HTML, "<b>plante") || !strings.Contains(m.HTML, "&lt;b&gt;plante") {
		t.Error("the summary is not escaped in the HTML part")
	}
	if !strings.Contains(m.HTML, "https://tickets.exemple.fr/browse/"+key) ||
		!strings.Contains(m.Text, "https://tickets.exemple.fr/browse/"+key) {
		t.Error("no link to the issue")
	}
	if !strings.Contains(m.Text, "mentioned you in the description") ||
		!strings.Contains(m.Text, "because someone mentioned you") ||
		!strings.Contains(m.Text, "https://tickets.exemple.fr/settings/notifications") {
		t.Errorf("headline, reason or preferences link missing:\n%s", m.Text)
	}
	if m.Header.Get("Message-ID") != "<issue."+id+"@exemple.fr>" {
		t.Errorf("the creation e-mail should start the issue's thread: Message-ID %q", m.Header.Get("Message-ID"))
	}

	// Watchers, one of them deactivated.
	hugo.call("PUT", "/api/issues/"+key+"/watch", nil, 200, nil)
	chloe.call("PUT", "/api/issues/"+key+"/watch", nil, 200, nil)
	if _, err := testDB.Exec(`INSERT INTO watchers (issue_id, user_id) VALUES (?, ?)`, id, userID(t, "emma.morel")); err != nil {
		t.Fatal(err)
	}

	// A comment restricted to developers, then a public one, within the
	// batching window: one e-mail each, and the restricted one never
	// reaches someone outside the group.
	paul.call("POST", "/api/issues/"+key+"/comments", map[string]any{"body": "Trace interne <script>alert(1)</script>",
		"visibility": map[string]string{"group": "developpeurs"}}, 201, nil)
	paul.call("POST", "/api/issues/"+key+"/comments", map[string]any{"body": "Corrigé en recette."}, 201, nil)
	msgs = env.deliver()
	if got := to(msgs, "paul.martin@exemple.fr"); len(got) != 0 {
		t.Error("the author was notified of his own comments")
	}
	if got := to(msgs, "emma.morel@exemple.fr"); len(got) != 0 {
		t.Error("a deactivated account was notified")
	}
	hm := to(msgs, "hugo.robert@exemple.fr")
	if len(hm) != 1 {
		t.Fatalf("hugo.robert got %d e-mails, want 1 (batched)", len(hm))
	}
	if !strings.Contains(hm[0].Text, "Trace interne") || !strings.Contains(hm[0].Text, "Corrigé en recette.") ||
		!strings.Contains(hm[0].Text, "commented.") || !strings.Contains(hm[0].Text, "watching this issue") {
		t.Errorf("hugo.robert's e-mail (English) misses something:\n%s", hm[0].Text)
	}
	if strings.Contains(hm[0].HTML, "<script>") || !strings.Contains(hm[0].HTML, "&lt;script&gt;") {
		t.Error("a comment is not escaped in the HTML part")
	}
	if hm[0].Header.Get("In-Reply-To") != "<issue."+id+"@exemple.fr>" {
		t.Errorf("In-Reply-To %q", hm[0].Header.Get("In-Reply-To"))
	}
	cm := to(msgs, "chloe.simon@exemple.fr")
	if len(cm) != 1 || !strings.Contains(cm[0].Text, "Corrigé en recette.") {
		t.Fatalf("chloe.simon: %d e-mails", len(cm))
	}
	if strings.Contains(cm[0].Raw, "Trace interne") {
		t.Fatal("a comment restricted to developpeurs reached chloe.simon")
	}

	// Assignment: the new assignee is told so.
	paul.call("PATCH", "/api/issues/"+key, map[string]any{"assignee": userID(t, "hugo.robert")}, 200, nil)
	msgs = env.deliver()
	if hm = to(msgs, "hugo.robert@exemple.fr"); len(hm) != 1 || !strings.Contains(hm[0].Text, "assigned the issue to you") ||
		!strings.Contains(hm[0].Text, "assigned to you.") {
		t.Errorf("assignee's e-mail: %+v", hm)
	}
	if cm = to(msgs, "chloe.simon@exemple.fr"); len(cm) != 1 || !strings.Contains(cm[0].Text, "Hugo Robert") {
		t.Errorf("watcher's e-mail about the assignment: %+v", cm)
	}

	// An issue the watcher cannot see never notifies them.
	rhKey := pick(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE p.pkey = 'RH' AND i.security_level_id IS NULL ORDER BY i.num LIMIT 1`)
	if _, err := testDB.Exec(`INSERT IGNORE INTO watchers (issue_id, user_id) VALUES (?, ?)`, issueID(t, rhKey),
		userID(t, "paul.martin")); err != nil {
		t.Fatal(err)
	}
	chloe.call("POST", "/api/issues/"+rhKey+"/comments", map[string]any{"body": "Entretien décalé."}, 201, nil)
	msgs = env.deliver()
	for _, m := range to(msgs, "paul.martin@exemple.fr") {
		t.Errorf("paul.martin was told about %s, which he cannot see: %s", rhKey, m.Subject)
	}

	// Deletion: watchers hear of it, with the issue as it was.
	env.login("admin", "admin").call("DELETE", "/api/issues/"+key, nil, 204, nil)
	msgs = env.deliver()
	if cm = to(msgs, "chloe.simon@exemple.fr"); len(cm) != 1 || !strings.Contains(cm[0].Text, "deleted the issue") ||
		cm[0].Subject != "["+key+"] Le formulaire <b>plante</b>" {
		t.Errorf("deletion e-mail: %+v", cm)
	}
}

func TestNotificationPreferences(t *testing.T) {
	env := newNotifyEnv(t)
	paul := env.login("paul.martin", "motdepasse")
	hugo := env.login("hugo.robert", "motdepasse")
	chloe := env.login("chloe.simon", "motdepasse")

	var p prefsOut
	hugo.call("GET", "/api/me/notifications", nil, 200, &p)
	if p.Delivery != "immediate" || p.OwnChanges || p.DueReminder || !p.MailEnabled || len(p.Events) != 8 {
		t.Fatalf("defaults: %+v", p)
	}
	for _, e := range p.Events {
		if !e {
			t.Fatalf("an event is off by default: %v", p.Events)
		}
	}
	for _, bad := range []map[string]any{{"delivery": "weekly"}, {"digestHour": 24}, {"events": map[string]bool{"x": true}},
		{"reminderHour": -1}} {
		hugo.call("PUT", "/api/me/notifications", bad, 400, nil)
	}

	// No comments for hugo.robert, but mentions still reach him.
	hugo.call("PUT", "/api/me/notifications", map[string]any{"events": map[string]bool{"commented": false}}, 200, &p)
	if p.Events["commented"] || !p.Events["mentioned"] {
		t.Fatalf("after switching comments off: %v", p.Events)
	}
	key := createIssue(t, paul, "WEB", "Préférences de notification", "")
	env.deliver() // the creation, before hugo.robert watches
	hugo.call("PUT", "/api/issues/"+key+"/watch", nil, 200, nil)
	paul.call("POST", "/api/issues/"+key+"/comments", map[string]any{"body": "Un commentaire ordinaire."}, 201, nil)
	if got := to(env.deliver(), "hugo.robert@exemple.fr"); len(got) != 0 {
		t.Error("hugo.robert switched comments off but got one")
	}
	paul.call("POST", "/api/issues/"+key+"/comments", map[string]any{"body": "[~hugo.robert] peux-tu regarder ?"}, 201, nil)
	if got := to(env.deliver(), "hugo.robert@exemple.fr"); len(got) != 1 || !strings.Contains(got[0].Text, "peux-tu regarder") {
		t.Errorf("a mention must reach hugo.robert: %d e-mails", len(got))
	}

	// Own changes, when asked for.
	paul.call("PUT", "/api/me/notifications", map[string]any{"ownChanges": true}, 200, nil)
	paul.call("PATCH", "/api/issues/"+key, map[string]any{"summary": "Préférences de notification (revu)"}, 200, nil)
	if got := to(env.deliver(), "paul.martin@exemple.fr"); len(got) != 1 || !strings.Contains(got[0].Text, "(revu)") {
		t.Errorf("own change not mailed although asked for: %d", len(got))
	}

	// Digest: nothing within the batching window, one e-mail for the day.
	chloe.call("PUT", "/api/me/notifications", map[string]any{"delivery": "digest", "digestHour": 7}, 200, nil)
	other := createIssue(t, paul, "WEB", "Deuxième ticket du résumé", "")
	for _, k := range []string{key, other} {
		chloe.call("PUT", "/api/issues/"+k+"/watch", nil, 200, nil)
		paul.call("POST", "/api/issues/"+k+"/comments", map[string]any{"body": "Pour le résumé de " + k}, 201, nil)
	}
	if got := to(env.deliver(), "chloe.simon@exemple.fr"); len(got) != 0 {
		t.Fatal("a digest reader got an immediate e-mail")
	}
	got := to(env.deliverAt(25*time.Hour), "chloe.simon@exemple.fr")
	if len(got) != 1 || !strings.HasPrefix(got[0].Subject, "Your notifications of ") ||
		!strings.Contains(got[0].Text, "Pour le résumé de "+key) || !strings.Contains(got[0].Text, "Pour le résumé de "+other) {
		t.Fatalf("digest: %+v", got)
	}

	// None: nothing at all, and what was waiting is dropped.
	paul.call("POST", "/api/issues/"+key+"/comments", map[string]any{"body": "Encore un."}, 201, nil)
	env.hooks.Settle()
	chloe.call("PUT", "/api/me/notifications", map[string]any{"delivery": "none"}, 200, nil)
	if got := to(env.deliverAt(25*time.Hour), "chloe.simon@exemple.fr"); len(got) != 0 {
		t.Error("e-mail sent although delivery is none")
	}
}

func TestProjectNotificationSettings(t *testing.T) {
	env := newNotifyEnv(t)
	marie := env.login("marie.durand", "motdepasse") // administers WEB
	paul := env.login("paul.martin", "motdepasse")
	hugo := env.login("hugo.robert", "motdepasse")
	if status, _ := hugo.do("GET", "/api/projects/WEB/notifications", nil); status != 403 {
		t.Errorf("a non-administrator read the project's notifications: %d", status)
	}
	direction, _ := strconv.ParseInt(pick(t, `SELECT id FROM user_groups WHERE name = 'direction'`), 10, 64)
	for _, bad := range []map[string]any{
		{"extra": []map[string]any{{"event": "mentioned", "kind": "group", "id": direction}}},
		{"extra": []map[string]any{{"event": "created", "kind": "user", "id": 3}}},
		{"extra": []map[string]any{{"event": "created", "kind": "group", "id": 999999}}},
	} {
		marie.call("PUT", "/api/projects/WEB/notifications", bad, 400, nil)
	}
	var ps struct {
		Enabled bool
		Extra   []struct{ Event, Kind, Name string }
	}
	marie.call("PUT", "/api/projects/WEB/notifications", map[string]any{
		"extra": []map[string]any{{"event": "created", "kind": "group", "id": direction}}}, 200, &ps)
	if !ps.Enabled || len(ps.Extra) != 1 || ps.Extra[0].Name != "direction" {
		t.Fatalf("saved settings: %+v", ps)
	}
	key := createIssue(t, paul, "WEB", "Annonce à la direction", "")
	msgs := env.deliver()
	for _, who := range []string{"manon.michel", "louis.fournier", "marie.durand"} {
		got := to(msgs, who+"@exemple.fr")
		if len(got) != 1 || !strings.Contains(got[0].Text, "project WEB lists you among its recipients") {
			t.Errorf("%s: %d e-mails about the new issue", who, len(got))
		}
	}

	// Switched off: nobody hears of the project's issues.
	marie.call("PUT", "/api/projects/WEB/notifications", map[string]any{"enabled": false}, 200, nil)
	hugo.call("PUT", "/api/issues/"+key+"/watch", nil, 200, nil)
	paul.call("POST", "/api/issues/"+key+"/comments", map[string]any{"body": "Personne ne le saura."}, 201, nil)
	if msgs := env.deliver(); len(msgs) != 0 {
		t.Errorf("notifications switched off, but %d e-mails went out", len(msgs))
	}
}

func TestFilterSubscriptions(t *testing.T) {
	env := newNotifyEnv(t)
	marie := env.login("marie.durand", "motdepasse")
	paul := env.login("paul.martin", "motdepasse")
	admin := env.login("admin", "admin")
	chloe := env.login("chloe.simon", "motdepasse")

	var f filterOut
	marie.call("POST", "/api/filters", map[string]any{"name": "Abonnement RH et WEB", "query": "project in (RH, WEB) ORDER BY key ASC",
		"shares": []map[string]any{{"kind": "everyone"}}}, 201, &f)
	defer marie.do("DELETE", fmt.Sprintf("/api/filters/%d", f.ID), nil)
	base := fmt.Sprintf("/api/filters/%d/subscriptions", f.ID)

	for _, bad := range []map[string]any{{"schedule": "monthly", "hour": 7}, {"schedule": "weekly", "hour": 7},
		{"schedule": "weekly", "days": []int{8}, "hour": 7}, {"schedule": "daily", "hour": 24}} {
		paul.call("POST", base, bad, 400, nil)
	}
	rh, _ := strconv.ParseInt(pick(t, `SELECT id FROM user_groups WHERE name = 'rh'`), 10, 64)
	paul.call("POST", base, map[string]any{"group": rh, "schedule": "daily", "hour": 7}, 403, nil)

	var mine, group subscriptionOut
	paul.call("POST", base, map[string]any{"schedule": "daily", "hour": 7}, 201, &mine)
	if mine.User == nil || mine.User.Username != "paul.martin" || !mine.NextRun.After(time.Now()) || !mine.CanEdit {
		t.Fatalf("subscription: %+v", mine)
	}
	admin.call("POST", base, map[string]any{"group": rh, "schedule": "weekly", "days": []int{1, 3}, "hour": 6}, 201, &group)
	var list []subscriptionOut
	paul.call("GET", base, nil, 200, &list)
	if len(list) != 1 || list[0].ID != mine.ID {
		t.Errorf("paul.martin sees subscriptions %+v, want only his own", list)
	}
	if status, _ := chloe.do("DELETE", fmt.Sprintf("%s/%d", base, mine.ID), nil); status != 404 {
		t.Errorf("chloe.simon deleted paul.martin's subscription: %d", status)
	}

	// Each recipient gets the results they may see: paul.martin no RH issue.
	paul.call("POST", fmt.Sprintf("%s/%d/run", base, mine.ID), nil, 200, nil)
	msgs := env.deliver()
	pm := to(msgs, "paul.martin@exemple.fr")
	if len(pm) != 1 || pm[0].Subject != "Filter “Abonnement RH et WEB”" {
		t.Fatalf("paul.martin's subscription e-mail: %+v", pm)
	}
	if strings.Contains(pm[0].Raw, "/browse/RH-") || !strings.Contains(pm[0].HTML, "/browse/WEB-") {
		t.Error("paul.martin's results must list WEB issues and no RH issue")
	}
	web := pick(t, `SELECT COUNT(*) FROM issues i JOIN projects p ON p.id = i.project_id WHERE p.pkey = 'WEB'`)
	if !strings.Contains(pm[0].Text, "Matching issues: "+web+" ") {
		t.Errorf("count missing (%s WEB issues):\n%.400s", web, pm[0].Text)
	}
	admin.call("POST", fmt.Sprintf("%s/%d/run", base, group.ID), nil, 200, nil)
	msgs = env.deliver()
	if got := recipients(msgs); len(got) != 3 {
		t.Errorf("group subscription mailed %v, want the 3 members of rh", got)
	}
	if cm := to(msgs, "chloe.simon@exemple.fr"); len(cm) != 1 || !strings.Contains(cm[0].HTML, "/browse/RH-") ||
		!strings.Contains(cm[0].Text, "the group rh is subscribed") {
		t.Errorf("chloe.simon (rh) must see RH issues: %+v", cm)
	}

	// On schedule: due now, sent, and planned again.
	if _, err := testDB.Exec(`UPDATE filter_subscriptions SET next_run_at = ? WHERE id = ?`,
		time.Now().UTC().Add(-time.Minute), mine.ID); err != nil {
		t.Fatal(err)
	}
	if err := env.hooks.RunSchedules(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := to(env.deliver(), "paul.martin@exemple.fr"); len(got) != 1 {
		t.Errorf("scheduled run: %d e-mails", len(got))
	}
	var next time.Time
	testDB.QueryRow(`SELECT next_run_at FROM filter_subscriptions WHERE id = ?`, mine.ID).Scan(&next)
	if !next.After(time.Now()) {
		t.Errorf("next run not planned: %v", next)
	}

	// At most 200 issues are listed, with a link to the rest.
	var big filterOut
	paul.call("POST", "/api/filters", map[string]any{"name": "Tout", "query": "project in (DEMO, WEB, OPS) ORDER BY key ASC"}, 201, &big)
	defer paul.do("DELETE", fmt.Sprintf("/api/filters/%d", big.ID), nil)
	var s subscriptionOut
	paul.call("POST", fmt.Sprintf("/api/filters/%d/subscriptions", big.ID), map[string]any{"schedule": "daily", "hour": 7}, 201, &s)
	paul.call("POST", fmt.Sprintf("/api/filters/%d/subscriptions/%d/run", big.ID, s.ID), nil, 200, nil)
	pm = to(env.deliver(), "paul.martin@exemple.fr")
	if len(pm) != 1 {
		t.Fatal("no e-mail for the big filter")
	}
	if n := strings.Count(pm[0].HTML, `href="https://tickets.exemple.fr/browse/`); n != 200 {
		t.Errorf("%d issues listed, want 200", n)
	}
	if !strings.Contains(pm[0].Text, "Only the first 200 issues are listed") ||
		!strings.Contains(pm[0].HTML, "https://tickets.exemple.fr/issues?q=") {
		t.Error("no note nor link for the issues left out")
	}
	paul.call("DELETE", fmt.Sprintf("%s/%d", base, mine.ID), nil, 204, nil)
}

func TestDueDateReminders(t *testing.T) {
	env := newNotifyEnv(t)
	paul := env.login("paul.martin", "motdepasse")
	chloe := env.login("chloe.simon", "motdepasse")
	hugo := env.login("hugo.robert", "motdepasse")
	hugo.call("PUT", "/api/me/notifications", map[string]any{"dueReminder": true, "reminderHour": 0}, 200, nil)
	today := time.Now().UTC()
	late := createIssue(t, paul, "WEB", "Échéance dépassée", "")
	due := createIssue(t, paul, "WEB", "Échéance du jour", "")
	later := createIssue(t, paul, "WEB", "Échéance lointaine", "")
	for k, d := range map[string]time.Time{late: today.AddDate(0, 0, -2), due: today, later: today.AddDate(0, 0, 5)} {
		paul.call("PATCH", "/api/issues/"+k, map[string]any{"assignee": userID(t, "hugo.robert"),
			"due": d.Format(time.DateOnly)}, 200, nil)
	}
	// Due in a project hugo.robert does not see: never listed.
	secret := createIssue(t, chloe, "RH", "Dossier confidentiel", "")
	if _, err := testDB.Exec(`UPDATE issues SET assignee_id = ?, due_date = ? WHERE id = ?`, userID(t, "hugo.robert"),
		today.Format(time.DateOnly), issueID(t, secret)); err != nil {
		t.Fatal(err)
	}
	env.deliver() // the assignment e-mails
	ctx := context.Background()
	if err := env.hooks.RunSchedules(ctx); err != nil {
		t.Fatal(err)
	}
	got := to(env.deliver(), "hugo.robert@exemple.fr")
	if len(got) != 1 || !strings.HasPrefix(got[0].Subject, "Your due dates for ") {
		t.Fatalf("reminder: %+v", got)
	}
	text := got[0].Text
	if !strings.Contains(text, "Overdue") || !strings.Contains(text, late) || !strings.Contains(text, "Due today") ||
		!strings.Contains(text, due) || strings.Contains(text, later) || strings.Contains(text, secret) {
		t.Errorf("reminder content:\n%s", text)
	}
	// Once a day.
	env.hooks.RunSchedules(ctx)
	if got := to(env.deliver(), "hugo.robert@exemple.fr"); len(got) != 0 {
		t.Error("a second reminder the same day")
	}
}

func TestMailSettings(t *testing.T) {
	env := newNotifyEnv(t)
	admin := env.login("admin", "admin")
	if status, _ := env.login("paul.martin", "motdepasse").do("GET", "/api/admin/mail", nil); status != 403 {
		t.Errorf("a non-administrator read the mail settings: %d", status)
	}
	for _, bad := range []map[string]any{{"security": "ssl"}, {"enabled": true, "host": ""}, {"from": "pas une adresse"},
		{"port": 70000}} {
		admin.call("PUT", "/api/admin/mail", bad, 400, nil)
	}
	raw := admin.call("PUT", "/api/admin/mail", map[string]any{"username": "relai", "password": "s3cret-smtp"}, 200, nil)
	var cfg mailSettingsOut
	json.Unmarshal(raw, &cfg)
	if !cfg.PasswordSet || strings.Contains(string(raw), "s3cret-smtp") || cfg.BaseURL == nil {
		t.Fatalf("settings answer: %s", raw)
	}
	stored := pick(t, `SELECT value FROM settings WHERE name = 'mail'`)
	if strings.Contains(stored, "s3cret-smtp") {
		t.Fatal("the SMTP password is stored in clear")
	}
	// Left out, the password stays.
	admin.call("PUT", "/api/admin/mail", map[string]any{"fromName": "Tickets"}, 200, &cfg)
	if !cfg.PasswordSet || cfg.FromName != "Tickets" {
		t.Errorf("after an update without password: %+v", cfg)
	}
	admin.call("POST", "/api/admin/mail/test", map[string]any{"to": "x"}, 400, nil)
	admin.call("POST", "/api/admin/mail/test", map[string]any{"to": "admin@exemple.fr"}, 200, nil)
	got := env.smtp.Messages()
	if len(got) != 1 || got[0].Subject != "Solstice test email" || got[0].To != "admin@exemple.fr" {
		t.Fatalf("test e-mail: %+v", got)
	}
	if auth := env.smtp.Auth(); len(auth) == 0 || auth[len(auth)-1] != "relai:s3cret-smtp" {
		t.Errorf("the stored password did not reach the server: %v", auth)
	}
	admin.call("PUT", "/api/admin/mail", map[string]any{"password": nil}, 200, &cfg)
	if cfg.PasswordSet {
		t.Error("password not removed")
	}
	// Failures are explained.
	admin.call("PUT", "/api/admin/mail", map[string]any{"port": 1}, 200, nil)
	_, body := admin.do("POST", "/api/admin/mail/test", map[string]any{"to": "admin@exemple.fr"})
	if !strings.Contains(string(body), "Cannot connect to the server") {
		t.Errorf("unreachable server: %s", body)
	}
	env.smtp.FailWith("550 5.7.1 relaying denied")
	admin.call("PUT", "/api/admin/mail", map[string]any{"port": env.smtp.Port}, 200, nil)
	_, body = admin.do("POST", "/api/admin/mail/test", map[string]any{"to": "admin@exemple.fr"})
	if !strings.Contains(string(body), "The server refuses the sender") || !strings.Contains(string(body), "relaying denied") {
		t.Errorf("sender refused: %s", body)
	}
	env.smtp.FailWith("")
	var queue map[string]any
	admin.call("GET", "/api/admin/mail/queue", nil, 200, &queue)
	for _, k := range []string{"pending", "failed", "sentLastDay", "recent", "errors", "batching", "digests"} {
		if _, ok := queue[k]; !ok {
			t.Errorf("queue lacks %s: %v", k, queue)
		}
	}
}

// A stored password that cannot be opened is logged, not sent to the client.
func TestMailTestHidesUnreadablePassword(t *testing.T) {
	env := newNotifyEnv(t)
	admin := env.login("admin", "admin")
	admin.call("PUT", "/api/admin/mail", map[string]any{"password": "s3cret-smtp"}, 200, nil)
	if _, err := testDB.Exec(`UPDATE settings SET value = JSON_SET(value, '$.password', 'garbage') WHERE name = 'mail'`); err != nil {
		t.Fatal(err)
	}
	status, msg, _ := admin.errorIn("en", "POST", "/api/admin/mail/test", map[string]any{"to": "admin@exemple.fr"})
	if status != 500 || strings.Contains(msg, "sealed secret") || !strings.Contains(msg, "Internal error") {
		t.Errorf("unreadable password: %d %q", status, msg)
	}
}

// hookReceiver records webhook deliveries.
type hookReceiver struct {
	mu   sync.Mutex
	got  []hookRequest
	srv  *httptest.Server
	code int
}

type hookRequest struct {
	header http.Header
	body   []byte
}

func newHookReceiver(t *testing.T) *hookReceiver {
	h := &hookReceiver{code: 200}
	h.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		h.mu.Lock()
		h.got = append(h.got, hookRequest{r.Header.Clone(), body})
		h.mu.Unlock()
		w.WriteHeader(h.code)
	}))
	t.Cleanup(h.srv.Close)
	return h
}

func (h *hookReceiver) take() []hookRequest {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := h.got
	h.got = nil
	return out
}

func TestWebhooks(t *testing.T) {
	env := newNotifyEnv(t)
	admin := env.login("admin", "admin")
	paul := env.login("paul.martin", "motdepasse")
	rc := newHookReceiver(t)
	ctx := context.Background()
	deliver := func() []hookRequest {
		env.hooks.Settle()
		if err := env.hooks.DeliverWebhooks(ctx); err != nil {
			t.Fatal(err)
		}
		return rc.take()
	}

	if status, _ := paul.do("GET", "/api/admin/webhooks", nil); status != 403 {
		t.Errorf("a non-administrator listed webhooks: %d", status)
	}
	var names []string
	admin.call("GET", "/api/admin/webhooks/events", nil, 200, &names)
	if len(names) == 0 || names[0] != "jira:issue_created" {
		t.Errorf("events: %v", names)
	}
	good := map[string]any{"name": "Intégration", "url": rc.srv.URL + "/hook", "events": []string{"jira:issue_created",
		"comment_created", "jira:issue_deleted"}, "jql": "project = WEB", "secret": "s3cret-hook"}
	for field, bad := range map[string]any{"url": "http://169.254.169.254/latest/meta-data", "events": []string{"jira:nope"},
		"jql": "project = = WEB", "name": ""} {
		in := map[string]any{}
		for k, v := range good {
			in[k] = v
		}
		in[field] = bad
		admin.call("POST", "/api/admin/webhooks", in, 400, nil)
	}
	in := map[string]any{}
	for k, v := range good {
		in[k] = v
	}
	in["url"] = "ftp://exemple.fr/hook"
	admin.call("POST", "/api/admin/webhooks", in, 400, nil)

	var wh struct {
		ID        int64
		HasSecret bool `json:"hasSecret"`
	}
	raw := admin.call("POST", "/api/admin/webhooks", good, 201, &wh)
	if !wh.HasSecret || strings.Contains(string(raw), "s3cret-hook") {
		t.Fatalf("created webhook: %s", raw)
	}
	path := fmt.Sprintf("/api/admin/webhooks/%d", wh.ID)

	// Only issues matching the JQL filter.
	webKey := createIssue(t, paul, "WEB", "Annoncé au webhook", "")
	createIssue(t, paul, "DEMO", "Hors du filtre du webhook", "")
	got := deliver()
	if len(got) != 1 {
		t.Fatalf("%d deliveries, want 1 (the WEB issue only)", len(got))
	}
	var p struct {
		WebhookEvent string `json:"webhookEvent"`
		EventType    string `json:"issue_event_type_name"`
		User         struct{ Name string }
		Issue        struct {
			Key, Self string
			Fields    struct{ Summary string }
		}
		Comment struct{ Body string }
	}
	json.Unmarshal(got[0].body, &p)
	if p.WebhookEvent != "jira:issue_created" || p.EventType != "issue_created" || p.Issue.Key != webKey ||
		p.User.Name != "paul.martin" || !strings.HasPrefix(p.Issue.Self, "https://tickets.exemple.fr/rest/api/2/issue/") {
		t.Errorf("payload: %s", got[0].body)
	}
	mac := hmac.New(sha256.New, []byte("s3cret-hook"))
	mac.Write(got[0].body)
	if got[0].header.Get("X-Hub-Signature") != "sha256="+hex.EncodeToString(mac.Sum(nil)) {
		t.Error("bad signature")
	}

	paul.call("POST", "/api/issues/"+webKey+"/comments", map[string]any{"body": "Pour le webhook"}, 201, nil)
	got = deliver()
	if len(got) != 1 || json.Unmarshal(got[0].body, &p) != nil || p.WebhookEvent != "comment_created" ||
		p.Comment.Body != "Pour le webhook" {
		t.Errorf("comment delivery: %d", len(got))
	}

	// A deleted issue still matches the filter it matched before.
	admin.call("DELETE", "/api/issues/"+webKey, nil, 204, nil)
	got = deliver()
	if len(got) != 1 || json.Unmarshal(got[0].body, &p) != nil || p.WebhookEvent != "jira:issue_deleted" || p.Issue.Key != webKey {
		t.Errorf("deletion delivery: %d", len(got))
	}

	var log []struct {
		Event      string
		Status     string
		StatusCode *int `json:"statusCode"`
	}
	admin.call("GET", path+"/deliveries", nil, 200, &log)
	if len(log) != 3 || log[0].Event != "jira:issue_deleted" || log[0].Status != "sent" || log[0].StatusCode == nil {
		t.Errorf("delivery log: %+v", log)
	}
	var test struct{ Status, Event string }
	admin.call("POST", path+"/test", nil, 200, &test)
	if test.Status != "sent" || test.Event != "solstice:test" || len(rc.take()) != 1 {
		t.Errorf("test delivery: %+v", test)
	}

	// Disabled: nothing goes out.
	admin.call("PUT", path, map[string]any{"enabled": false}, 200, nil)
	createIssue(t, paul, "WEB", "Webhook coupé", "")
	if got := deliver(); len(got) != 0 {
		t.Errorf("a disabled webhook delivered %d times", len(got))
	}
	admin.call("DELETE", path, nil, 204, nil)
	admin.call("GET", path, nil, 404, nil)
}
