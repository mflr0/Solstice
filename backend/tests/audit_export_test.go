package tests

// The audit log and the CSV export.

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

type auditPage struct {
	Total   int
	Entries []auditEntryOut
}

func (p auditPage) actions() []string {
	var out []string
	for _, e := range p.Entries {
		out = append(out, e.Action)
	}
	return out
}

func TestAuditLog(t *testing.T) {
	// Failed sign-ins lock the address: they go to a server of their own.
	base := ownServer(t)
	for _, c := range []struct {
		user, pass string
		want       int
	}{{"hugo.robert", "pas-le-bon-mot-de-passe", 401}, {"personne.inconnue", "x", 401}, {"hugo.robert", "motdepasse", 200}} {
		if status, _, _ := call(t, nil, base, "POST", "/api/login", map[string]string{"username": c.user, "password": c.pass},
			header(csrfHeader, "1")); status != c.want {
			t.Fatalf("login %s: %d, want %d", c.user, status, c.want)
		}
	}

	admin := login(t, "admin", "admin")
	username := fmt.Sprintf("audit.%d", time.Now().UnixNano()%1e9)
	status, body := admin.do("POST", "/api/admin/users", map[string]any{"username": username, "name": "Compte Audit",
		"email": "audit@example.org", "password": "premier-secret-123"})
	if status != 201 {
		t.Fatalf("create user: %d %s", status, body)
	}
	var created struct{ ID int64 }
	json.Unmarshal(body, &created)
	userPath := fmt.Sprintf("/api/admin/users/%d", created.ID)
	for _, change := range []map[string]any{{"name": "Compte Renommé", "admin": true, "password": "second-secret-456"},
		{"active": false}} {
		if status, body := admin.do("PATCH", userPath, change); status != 204 {
			t.Fatalf("update user: %d %s", status, body)
		}
	}

	allowTokensForEveryone(t)
	paul := login(t, "paul.martin", "motdepasse")
	status, body = paul.do("POST", "/api/me/tokens", map[string]any{"name": "jeton audité"})
	if status != 201 {
		t.Fatalf("token: %d %s", status, body)
	}
	tokenID := int64(obj(t, body)["id"].(float64))
	token := str(obj(t, body), "token")
	paul.do("DELETE", fmt.Sprintf("/api/me/tokens/%d", tokenID), nil)

	status, body = paul.do("POST", "/api/filters", map[string]any{"name": "Audit partagé", "query": "project = DEMO",
		"shares": []map[string]any{{"kind": "everyone"}}})
	if status != 201 {
		t.Fatalf("filter: %d %s", status, body)
	}

	// A grant added to OPS, then the grants put back as they were.
	var perms struct {
		Grants []grantOut `json:"grants"`
	}
	marie := login(t, "marie.durand", "motdepasse")
	marie.get("/api/projects/OPS/permissions", &perms)
	hugo, _ := strconv.ParseInt(pick(t, `SELECT id FROM users WHERE username = 'hugo.robert'`), 10, 64)
	added := append(slices.Clone(perms.Grants), grantOut{Permission: "link", Holder: "user", ID: hugo})
	for _, grants := range [][]grantOut{added, perms.Grants} {
		if status, body := marie.do("PUT", "/api/projects/OPS/permissions", map[string]any{"grants": grants}); status != 200 {
			t.Fatalf("permissions: %d %s", status, body)
		}
	}

	status, body = admin.do("POST", "/api/statuses", map[string]any{"name": "Audit en attente", "category": "todo"})
	if status != 201 {
		t.Fatalf("status: %d %s", status, body)
	}
	statusID := str(obj(t, body), "id")
	if status, _ := admin.do("DELETE", "/api/statuses/"+statusID, nil); status != 204 {
		t.Fatalf("delete status: %d", status)
	}

	list := func(query string) auditPage {
		t.Helper()
		var p auditPage
		if status := admin.get("/api/admin/audit?"+query, &p); status != 200 {
			t.Fatalf("GET audit?%s: %d", query, status)
		}
		return p
	}
	logins := list("action=login&user=hugo.robert")
	if acts := logins.actions(); !slices.Contains(acts, "login.failure") || !slices.Contains(acts, "login.success") {
		t.Errorf("hugo.robert's sign-ins: %v", acts)
	}
	for _, e := range logins.Entries {
		if e.User == nil || e.User.Username != "hugo.robert" || e.IP == nil || *e.IP == "" {
			t.Errorf("sign-in entry: %+v", e)
		}
		// A lockout is not a sign-in attempt: it has no method, but its length.
		if e.Action == "login.lockout" {
			if _, ok := e.Details["seconds"]; !ok {
				t.Errorf("lockout entry without seconds: %+v", e)
			}
		} else if e.Details["method"] != "form" {
			t.Errorf("sign-in entry: %+v", e)
		}
	}
	unknown := list("user=personne.inconnue")
	if unknown.Total == 0 || unknown.Entries[0].User != nil || *unknown.Entries[0].Username != "personne.inconnue" {
		t.Errorf("failed sign-in of an unknown name: %+v", unknown)
	}

	account := list("q=" + username)
	for _, want := range []string{"user.create", "user.update", "admin.grant", "password.change", "user.deactivate"} {
		if !slices.Contains(account.actions(), want) {
			t.Errorf("%s missing for the new account: %v", want, account.actions())
		}
	}
	raw, _ := json.Marshal(account)
	if strings.Contains(string(raw), "secret-123") || strings.Contains(string(raw), "secret-456") {
		t.Fatal("a password reached the audit log")
	}
	for _, e := range account.Entries {
		if e.Action == "user.update" {
			if name, _ := e.Details["name"].(map[string]any); name["from"] != "Compte Audit" || name["to"] != "Compte Renommé" {
				t.Errorf("user.update details: %v", e.Details)
			}
		}
		if e.User == nil || e.User.Username != "admin" {
			t.Errorf("%s by %+v, want admin", e.Action, e.User)
		}
	}

	tokens := list("action=token&user=paul.martin")
	if acts := tokens.actions(); !slices.Contains(acts, "token.create") || !slices.Contains(acts, "token.revoke") ||
		*tokens.Entries[0].Target.Label != "jeton audité" {
		t.Errorf("token entries: %+v", tokens.Entries)
	}
	if strings.Contains(fmt.Sprint(tokens), token) {
		t.Fatal("a token reached the audit log")
	}
	grants := list("action=project.permissions")
	if grants.Total < 2 || !strings.Contains(fmt.Sprint(grants.Entries[1].Details["added"]), "link: user hugo.robert") ||
		!strings.Contains(fmt.Sprint(grants.Entries[0].Details["removed"]), "link: user hugo.robert") {
		t.Errorf("permission entries: %+v", grants.Entries[:min(2, len(grants.Entries))])
	}
	if shares := list("action=filter.share&user=paul.martin"); shares.Total == 0 ||
		fmt.Sprint(shares.Entries[0].Details["to"]) != "[everyone]" {
		t.Errorf("filter share entries: %+v", shares.Entries)
	}
	if acts := list("action=status&q=" + url.QueryEscape("Audit en attente")).actions(); !slices.Contains(acts, "status.create") ||
		!slices.Contains(acts, "status.delete") {
		t.Errorf("status entries: %v", acts)
	}
	tomorrow := time.Now().AddDate(0, 0, 2).Format(time.DateOnly)
	if p := list("from=" + tomorrow); p.Total != 0 {
		t.Errorf("entries from %s: %d", tomorrow, p.Total)
	}
	if p := list("to=" + tomorrow + "&action=login"); p.Total == 0 || p.Size() != auditPageSize {
		t.Errorf("entries up to %s: %d", tomorrow, p.Total)
	}
	if status := paul.get("/api/admin/audit", nil); status != 403 {
		t.Errorf("paul.martin reads the audit log: %d", status)
	}
	if status, _ := admin.do("GET", "/api/admin/audit?from=hier", nil); status != 400 {
		t.Errorf("bad date: %d", status)
	}

	// Retention: shortening it purges what is older.
	if _, err := testDB.Exec(`INSERT INTO audit_log (created_at, action, username) VALUES (?, 'test.old', 'ancien')`,
		time.Now().UTC().AddDate(0, 0, -40)); err != nil {
		t.Fatal(err)
	}
	if status, body := admin.do("PUT", "/api/admin/audit/settings", map[string]int{"retentionDays": 30}); status != 200 {
		t.Fatalf("retention: %d %s", status, body)
	}
	if n := pick(t, `SELECT COUNT(*) FROM audit_log WHERE action = 'test.old'`); n != "0" {
		t.Errorf("old entry not purged")
	}
	if p := list("action=login"); p.Total == 0 {
		t.Error("recent entries purged")
	}
	if status, _ := admin.do("PUT", "/api/admin/audit/settings", map[string]int{"retentionDays": -1}); status != 400 {
		t.Errorf("negative retention: %d", status)
	}
	admin.do("PUT", "/api/admin/audit/settings", map[string]int{"retentionDays": 365})
	var settings struct{ RetentionDays int }
	if admin.get("/api/admin/audit/settings", &settings); settings.RetentionDays != 365 {
		t.Errorf("retention: %d", settings.RetentionDays)
	}
	if acts := list("action=settings.audit").actions(); len(acts) < 2 {
		t.Errorf("retention changes not logged: %v", acts)
	}
}

// auditPageSize is how many entries a page of the audit log holds.
const auditPageSize = 50

func (p auditPage) Size() int { return min(p.Total, auditPageSize) }

// CSV export -------------------------------------------------------------------

func exportCSVFor(t *testing.T, c *client, query string, lang string) (int, http.Header, []byte) {
	t.Helper()
	var setup func(*http.Request)
	if lang != "" {
		setup = header("Accept-Language", lang)
	}
	return call(t, c, testURL, "GET", "/api/issues/export.csv?"+query, nil, setup)
}

func readCSV(t *testing.T, raw []byte) [][]string {
	t.Helper()
	if !bytes.HasPrefix(raw, []byte("\xEF\xBB\xBF")) {
		t.Fatalf("no byte order mark: %q", raw[:min(len(raw), 20)])
	}
	records, err := csv.NewReader(bytes.NewReader(raw[3:])).ReadAll()
	if err != nil {
		t.Fatalf("not RFC 4180: %v\n%s", err, raw)
	}
	return records
}

func TestCSVExport(t *testing.T) {
	paul := login(t, "paul.martin", "motdepasse")
	task := pick(t, `SELECT t.issue_type_id FROM project_issue_types t JOIN issue_types it ON it.id = t.issue_type_id
		JOIN projects p ON p.id = t.project_id WHERE p.pkey = 'DEMO' AND NOT it.subtask ORDER BY t.position LIMIT 1`)
	summary := `=HYPERLINK("http://exemple.org"), "devis" urgent`
	description := "Ligne 1\nLigne 2 ; avec, virgule"
	status, body := paul.do("POST", "/api/issues", map[string]any{"project": "DEMO", "type": task, "summary": summary,
		"description": description})
	if status != 201 {
		t.Fatalf("create: %d %s", status, body)
	}
	var created struct{ Key string }
	json.Unmarshal(body, &created)

	q := "jql=" + url.QueryEscape("key = "+created.Key) + "&columns=key,summary,description,status,created,customfield_10100&tz=Asia/Kolkata"
	status, h, raw := exportCSVFor(t, paul, q, "en")
	if status != 200 || !strings.HasPrefix(h.Get("Content-Type"), "text/csv") || !strings.Contains(h.Get("Content-Disposition"), ".csv") {
		t.Fatalf("export: %d %v %s", status, h, raw)
	}
	if !bytes.Contains(raw, []byte("\r\n")) || !bytes.Contains(raw, []byte(`""devis""`)) {
		t.Errorf("not RFC 4180 (CRLF, doubled quotes): %q", raw)
	}
	records := readCSV(t, raw)
	if want := []string{"Key", "Summary", "Description", "Status", "Created", "Story Points"}; !slices.Equal(records[0], want) {
		t.Errorf("headers: %v, want %v", records[0], want)
	}
	if len(records) != 2 || records[1][0] != created.Key || records[1][1] != "'"+summary || records[1][2] != description {
		t.Fatalf("row: %q", records)
	}
	createdAt, _ := time.Parse("2006-01-02 15:04:05.000", pick(t, `SELECT DATE_FORMAT(i.created_at, '%Y-%m-%d %H:%i:%s.000')
		FROM issues i JOIN projects p ON p.id = i.project_id WHERE `+keyExpr+` = ?`, created.Key))
	kolkata, _ := time.LoadLocation("Asia/Kolkata")
	if want := createdAt.In(kolkata).Format("2006-01-02 15:04"); records[1][4] != want {
		t.Errorf("created in Asia/Kolkata: %q, want %q", records[1][4], want)
	}

	// Headers follow the language; default columns.
	_, _, raw = exportCSVFor(t, paul, "jql="+url.QueryEscape("key = "+created.Key), "de")
	records = readCSV(t, raw)
	if len(records[0]) != 11 || records[0][0] != "Schlüssel" || records[0][2] != "Zusammenfassung" {
		t.Errorf("German default headers: %v", records[0])
	}
	_, _, raw = exportCSVFor(t, paul, "q="+url.QueryEscape(created.Key)+"&columns=key,summary", "fr")
	if records = readCSV(t, raw); records[0][0] != "Clé" || len(records) != 2 {
		t.Errorf("simple search, French: %v", records)
	}

	// Only what the user sees.
	_, _, raw = exportCSVFor(t, paul, "jql="+url.QueryEscape("project = RH"), "")
	if records = readCSV(t, raw); len(records) != 1 {
		t.Errorf("paul.martin exports %d RH rows", len(records)-1)
	}
	chloe := login(t, "chloe.simon", "motdepasse")
	var ui struct{ Total int }
	chloe.get("/api/issues?jql="+url.QueryEscape("project = RH"), &ui)
	_, h, raw = exportCSVFor(t, chloe, "jql="+url.QueryEscape("project = RH"), "")
	if records = readCSV(t, raw); len(records)-1 != ui.Total || ui.Total == 0 || h.Get("X-Total-Count") != strconv.Itoa(ui.Total) {
		t.Errorf("chloe.simon exports %d RH rows, sees %d", len(records)-1, ui.Total)
	}
	key := pick(t, `SELECT `+keyExpr+` FROM comments c JOIN issues i ON i.id = c.issue_id
		JOIN projects p ON p.id = i.project_id
		WHERE c.visibility_group = 'developpeurs' AND i.security_level_id IS NULL AND p.pkey <> 'RH'
		ORDER BY c.id LIMIT 1`)
	secret := pick(t, `SELECT c.body FROM comments c JOIN issues i ON i.id = c.issue_id JOIN projects p ON p.id = i.project_id
		WHERE `+keyExpr+` = ? AND c.visibility_group = 'developpeurs' ORDER BY c.id LIMIT 1`, key)
	query := "jql=" + url.QueryEscape("key = "+key) + "&columns=key,comment"
	if _, _, raw = exportCSVFor(t, paul, query, ""); !strings.Contains(string(raw), secret) {
		t.Errorf("paul.martin's export lacks the comment restricted to developpeurs")
	}
	if _, _, raw = exportCSVFor(t, chloe, query, ""); strings.Contains(string(raw), secret) {
		t.Errorf("chloe.simon's export shows a comment restricted to developpeurs")
	}

	if status, _, _ := exportCSVFor(t, paul, "columns=key,couleur", ""); status != 400 {
		t.Errorf("unknown column: %d", status)
	}
	if status, _, _ := exportCSVFor(t, paul, "jql="+url.QueryEscape(`project = "DEMO`), ""); status != 400 {
		t.Errorf("bad JQL: %d", status)
	}
	// Scripts export with a token.
	if status, _, _ := call(t, nil, testURL, "GET", "/api/issues/export.csv?q="+created.Key, nil,
		bearer(newToken(t, "paul.martin", nil))); status != 200 {
		t.Errorf("export with a token: %d", status)
	}
}
