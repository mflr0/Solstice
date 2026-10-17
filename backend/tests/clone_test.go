package tests

// Cloning an issue: POST /api/issues/{key}/clone.

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"slices"
	"sort"
	"strings"
	"testing"
)

type cloneAnswer struct {
	Key      string   `json:"key"`
	Warnings []string `json:"warnings"`
}

func dbInt(t *testing.T, query string, args ...any) int64 {
	t.Helper()
	needDB(t)
	var id int64
	if err := testDB.QueryRow(query, args...).Scan(&id); err != nil {
		t.Fatalf("fixture lookup failed: %v\n%s", err, query)
	}
	return id
}

func issueIDOf(t *testing.T, key string) int64 {
	t.Helper()
	return dbInt(t, `SELECT i.id FROM issues i JOIN projects p ON p.id = i.project_id WHERE `+keyExpr+` = ?`, key)
}

// cloneOf clones an issue in a language and returns the answer.
func cloneOf(t *testing.T, c *client, lang, key string, body map[string]any) cloneAnswer {
	t.Helper()
	status, raw := c.request(lang, "POST", "/api/issues/"+key+"/clone", body)
	if status != 201 {
		t.Fatalf("clone %s: %d %s", key, status, raw)
	}
	var out cloneAnswer
	if err := json.Unmarshal(raw, &out); err != nil || out.Key == "" || out.Warnings == nil {
		t.Fatalf("clone %s: unexpected answer %s", key, raw)
	}
	return out
}

func attachFile(t *testing.T, c *client, key, name, content string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", name)
	fw.Write([]byte(content))
	mw.Close()
	req, _ := http.NewRequest("POST", c.base+"/api/issues/"+key+"/attachments", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set(csrfHeader, "1")
	resp, err := c.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if body, _ := io.ReadAll(resp.Body); resp.StatusCode != 200 {
		t.Fatalf("upload to %s: %d %s", key, resp.StatusCode, body)
	}
}

func assigneeName(u *userRef) string {
	if u == nil {
		return ""
	}
	return u.Username
}

func TestCloneCopiesTheIssue(t *testing.T) {
	admin := login(t, "admin", "admin")
	paul := login(t, "paul.martin", "motdepasse")
	chloe := login(t, "chloe.simon", "motdepasse")

	component := dbInt(t, `SELECT c.id FROM components c JOIN projects p ON p.id = c.project_id
		WHERE p.pkey = 'DEMO' ORDER BY c.id LIMIT 1`)
	affects := dbInt(t, `SELECT v.id FROM versions v JOIN projects p ON p.id = v.project_id
		WHERE p.pkey = 'DEMO' ORDER BY v.id LIMIT 1`)
	fix := dbInt(t, `SELECT v.id FROM versions v JOIN projects p ON p.id = v.project_id
		WHERE p.pkey = 'DEMO' ORDER BY v.id LIMIT 1 OFFSET 1`)
	priority := pick(t, `SELECT id FROM priorities WHERE NOT is_default ORDER BY position LIMIT 1`)
	subtaskType := pick(t, `SELECT t.issue_type_id FROM project_issue_types t JOIN issue_types it ON it.id = t.issue_type_id
		JOIN projects p ON p.id = t.project_id WHERE p.pkey = 'DEMO' AND it.subtask LIMIT 1`)
	linkType := dbInt(t, `SELECT id FROM link_types WHERE outward <> 'clones' ORDER BY position LIMIT 1`)
	hidden := pick(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE p.pkey = 'RH' AND i.security_level_id IS NULL ORDER BY i.num LIMIT 1`)

	// The original is the administrator's, so that the reporter of the clone can differ.
	taskType := standardType(t, admin, "DEMO")
	var made struct{ Key string }
	admin.call("POST", "/api/issues", map[string]any{"project": "DEMO", "type": taskType,
		"summary": "Original à cloner", "description": "h3. Contexte\nun texte", "priority": priority,
		"labels": []string{"alpha", "beta"}, "components": []int64{component},
		"affectsVersions": []int64{affects}, "fixVersions": []int64{fix}}, 201, &made)
	orig := made.Key
	admin.call("POST", "/api/issues", map[string]any{"project": "DEMO", "type": subtaskType,
		"summary": "Sous-tâche à cloner", "parent": orig}, 201, nil)
	visible := createIssue(t, admin, "DEMO", "Voisin visible", "")
	// The RH issue is seen by chloe.simon (RH), not by paul.martin.
	for target, who := range map[string]*client{visible: admin, hidden: chloe} {
		who.call("POST", "/api/issues/"+orig+"/links", map[string]any{"type": linkType, "direction": "outward",
			"target": target}, 200, nil)
	}
	origID := issueIDOf(t, orig)
	// A custom field, and one of Jira Software's internal ones (the rank), which is not copied.
	for _, v := range []struct{ field, value string }{{"Code client", "C-42"}, {"Rank", "0|zzzzzz:"}} {
		if _, err := testDB.Exec(`INSERT INTO custom_field_values (issue_id, field_id, string_value)
			SELECT ?, id, ? FROM custom_fields WHERE name = ? ORDER BY id LIMIT 1`, origID, v.value, v.field); err != nil {
			t.Fatal(err)
		}
	}
	attachFile(t, admin, orig, "note.txt", "contenu du fichier")

	// What the project's assignment rule gives an issue with these components.
	var reference issueDetail
	var refMade struct{ Key string }
	paul.call("POST", "/api/issues", map[string]any{"project": "DEMO", "type": taskType, "summary": "Référence",
		"components": []int64{component}}, 201, &refMade)
	paul.call("GET", "/api/issues/"+refMade.Key, nil, 200, &reference)

	got := cloneOf(t, paul, "en", orig, map[string]any{"includeSubtasks": true, "includeLinks": true,
		"includeAttachments": true})
	if got.Key == orig || len(got.Warnings) != 0 {
		t.Fatalf("clone answer: %+v", got)
	}
	var d issueDetail
	paul.call("GET", "/api/issues/"+got.Key, nil, 200, &d)
	if d.Summary != "CLONE - Original à cloner" || d.Type.ID != taskType || d.Priority == nil || d.Priority.ID != priority ||
		d.Reporter == nil || d.Reporter.Username != "paul.martin" || d.Status.Category != "todo" ||
		assigneeName(d.Assignee) != assigneeName(reference.Assignee) || !d.Watchers.Me {
		t.Errorf("clone looks wrong: %+v", d)
	}
	sort.Strings(d.Labels)
	if !slices.Equal(d.Labels, []string{"alpha", "beta"}) || len(d.Components) != 1 || d.Components[0].ID != component ||
		len(d.AffectsVersions) != 1 || d.AffectsVersions[0].ID != affects || len(d.FixVersions) != 1 || d.FixVersions[0].ID != fix {
		t.Errorf("clone lost labels, components or versions: %+v", d)
	}
	copyID := issueIDOf(t, got.Key)
	if n := dbInt(t, `SELECT COUNT(*) FROM issues a JOIN issues b ON b.id = ? WHERE a.id = ?
		AND a.description <=> b.description AND a.issue_type_id = b.issue_type_id AND a.priority_id <=> b.priority_id
		AND a.security_level_id <=> b.security_level_id AND a.project_id = b.project_id`, copyID, origID); n != 1 {
		t.Error("the clone's description, type, priority, level or project differ from the original's")
	}
	rows, err := testDB.Query(`SELECT f.name, v.string_value FROM custom_field_values v JOIN custom_fields f ON f.id = v.field_id
		WHERE v.issue_id = ?`, copyID)
	if err != nil {
		t.Fatal(err)
	}
	var customs []string
	for rows.Next() {
		var name, value string
		rows.Scan(&name, &value)
		customs = append(customs, name+"="+value)
	}
	rows.Close()
	if !slices.Equal(customs, []string{"Code client=C-42"}) {
		t.Errorf("custom field values of the clone: %v", customs)
	}

	// Sub-tasks: a copy under the clone, the original keeps its own.
	if n := dbInt(t, `SELECT COUNT(*) FROM issues i WHERE i.parent_id = ? AND i.summary = 'Sous-tâche à cloner'
		AND i.reporter_id = ?`, copyID, userID(t, "paul.martin")); n != 1 {
		t.Errorf("the clone has %d copied sub-tasks, want 1", n)
	}
	if n := dbInt(t, `SELECT COUNT(*) FROM issues WHERE parent_id = ?`, origID); n != 1 {
		t.Errorf("the original has %d sub-tasks after the clone, want 1", n)
	}

	// Links: to the original ("clones") and to the visible issue, not to the hidden one.
	if n := dbInt(t, `SELECT COUNT(*) FROM issue_links l JOIN link_types t ON t.id = l.type_id
		WHERE l.source_id = ? AND l.target_id = ? AND t.outward = 'clones' AND t.inward = 'is cloned by'`,
		copyID, origID); n != 1 {
		t.Errorf("%d links \"clones\" the original, want 1", n)
	}
	var linked []string
	for _, l := range d.Links {
		linked = append(linked, l.Issue.Key)
	}
	want := []string{orig, visible}
	sort.Strings(linked)
	sort.Strings(want)
	if !slices.Equal(linked, want) {
		t.Errorf("links of the clone: %v, want %v", linked, want)
	}
	hiddenID := issueIDOf(t, hidden)
	if n := dbInt(t, `SELECT COUNT(*) FROM issue_links WHERE (source_id = ? AND target_id = ?)
		OR (source_id = ? AND target_id = ?)`, copyID, hiddenID, hiddenID, copyID); n != 0 {
		t.Error("the clone is linked to an issue its author cannot see")
	}
	if n := dbInt(t, `SELECT COUNT(*) FROM issue_links WHERE source_id = ? OR target_id = ?`, copyID, copyID); n != 2 {
		t.Errorf("the clone has %d links, want 2 (the original and the visible issue)", n)
	}

	// Attachments: a new row and a new file with the same content.
	var again issueDetail
	admin.call("GET", "/api/issues/"+orig, nil, 200, &again)
	if len(d.Attachments) != 1 || len(again.Attachments) != 1 || d.Attachments[0].ID == again.Attachments[0].ID {
		t.Fatalf("attachments of the clone: %+v (original: %+v)", d.Attachments, again.Attachments)
	}
	if status, content := paul.do("GET", d.Attachments[0].URL, nil); status != 200 || string(content) != "contenu du fichier" {
		t.Errorf("download of the copied attachment: %d %q", status, content)
	}

	// History says where it comes from; the original shows the link.
	var history []historyEntry
	paul.call("GET", "/api/issues/"+got.Key+"/history", nil, 200, &history)
	found := false
	for _, h := range history {
		for _, it := range h.Items {
			found = found || (it.To != nil && strings.Contains(*it.To, "created by cloning "+orig))
		}
	}
	if !found {
		t.Errorf("history of the clone does not say it was cloned from %s: %+v", orig, history)
	}
	if !slices.ContainsFunc(again.Links, func(l struct {
		ID    int64 `json:"id"`
		Issue struct {
			Key string `json:"key"`
		} `json:"issue"`
	}) bool {
		return l.Issue.Key == got.Key
	}) {
		t.Errorf("the original does not link to its clone %s", got.Key)
	}

	// Someone who sees the hidden issue keeps that link in a clone.
	full := cloneOf(t, chloe, "en", orig, map[string]any{"includeLinks": true})
	if n := dbInt(t, `SELECT COUNT(*) FROM issue_links WHERE (source_id = ? AND target_id = ?)
		OR (source_id = ? AND target_id = ?)`, issueIDOf(t, full.Key), hiddenID, hiddenID, issueIDOf(t, full.Key)); n != 1 {
		t.Errorf("chloe.simon's clone has %d links to %s, want 1", n, hidden)
	}
}

func TestCloneOptionsAndLanguage(t *testing.T) {
	paul := login(t, "paul.martin", "motdepasse")
	chloe := login(t, "chloe.simon", "motdepasse") // not a developer: does not see "Équipe" issues
	team := dbInt(t, `SELECT l.id FROM security_levels l JOIN projects p ON p.id = l.project_id
		WHERE p.pkey = 'DEMO' AND l.name = 'Équipe'`)
	subtaskType := pick(t, `SELECT t.issue_type_id FROM project_issue_types t JOIN issue_types it ON it.id = t.issue_type_id
		JOIN projects p ON p.id = t.project_id WHERE p.pkey = 'DEMO' AND it.subtask LIMIT 1`)

	var made struct{ Key string }
	paul.call("POST", "/api/issues", map[string]any{"project": "DEMO", "type": standardType(t, paul, "DEMO"),
		"summary": "Confidentiel de l'équipe", "securityLevel": team}, 201, &made)
	orig := made.Key
	paul.call("POST", "/api/issues", map[string]any{"project": "DEMO", "type": subtaskType,
		"summary": "Sous-tâche", "parent": orig}, 201, nil)
	attachFile(t, paul, orig, "note.txt", "contenu")
	origID := issueIDOf(t, orig)

	if status, _ := chloe.do("POST", "/api/issues/"+orig+"/clone", map[string]any{}); status != 404 {
		t.Errorf("someone who cannot see the issue cloned it: %d", status)
	}

	// Nothing asked: only the issue, and the link to its original.
	plain := cloneOf(t, paul, "en", orig, map[string]any{"summary": "  Mon clone  "})
	var d issueDetail
	paul.call("GET", "/api/issues/"+plain.Key, nil, 200, &d)
	plainID := issueIDOf(t, plain.Key)
	if d.Summary != "Mon clone" {
		t.Errorf("summary of the clone: %q", d.Summary)
	}
	if n := dbInt(t, `SELECT COUNT(*) FROM issues WHERE parent_id = ?`, plainID); n != 0 {
		t.Errorf("sub-tasks were cloned without being asked: %d", n)
	}
	if n := dbInt(t, `SELECT COUNT(*) FROM attachments WHERE issue_id = ?`, plainID); n != 0 {
		t.Errorf("attachments were cloned without being asked: %d", n)
	}
	if n := dbInt(t, `SELECT COUNT(*) FROM issue_links WHERE source_id = ? OR target_id = ?`, plainID, plainID); n != 1 {
		t.Errorf("the clone has %d links, want only the one to its original", n)
	}
	if n := dbInt(t, `SELECT COUNT(*) FROM issues a JOIN issues b ON b.id = ? WHERE a.id = ?
		AND a.security_level_id = b.security_level_id AND a.security_level_id = ?`, plainID, origID, team); n != 1 {
		t.Error("the clone does not keep the security level of the original")
	}
	if status := chloe.get("/api/issues/"+plain.Key, nil); status != 404 {
		t.Errorf("a non-developer sees the clone of a team-only issue: %d", status)
	}

	// The default summary is translated.
	german := cloneOf(t, paul, "de", orig, map[string]any{})
	paul.call("GET", "/api/issues/"+german.Key, nil, 200, &d)
	if d.Summary != "KLON - Confidentiel de l'équipe" {
		t.Errorf("default summary in German: %q", d.Summary)
	}

	// A clone of a sub-task is a sub-task of the same parent.
	var child struct{ Key string }
	if err := testDB.QueryRow(`SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE i.parent_id = ?`, origID).Scan(&child.Key); err != nil {
		t.Fatal(err)
	}
	sibling := cloneOf(t, paul, "en", child.Key, map[string]any{})
	if n := dbInt(t, `SELECT COUNT(*) FROM issues WHERE id = ? AND parent_id = ?`, issueIDOf(t, sibling.Key), origID); n != 1 {
		t.Error("the clone of a sub-task is not under the same parent")
	}
}

func TestCloneRefusals(t *testing.T) {
	admin := login(t, "admin", "admin")
	paul := login(t, "paul.martin", "motdepasse")
	demo := createIssue(t, paul, "DEMO", "À cloner", "")

	hidden := pick(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE p.pkey = 'RH' AND i.security_level_id IS NULL ORDER BY i.num LIMIT 1`)
	if status, _ := paul.do("POST", "/api/issues/"+hidden+"/clone", map[string]any{}); status != 404 {
		t.Errorf("clone of an issue the user cannot see: %d, want 404", status)
	}
	if status, _ := paul.do("POST", "/api/issues/"+demo+"/clone", map[string]any{"summary": "   "}); status != 400 {
		t.Errorf("clone with a blank summary: %d, want 400", status)
	}
	if status, _ := paul.do("POST", "/api/issues/"+demo+"/clone", map[string]any{"summary": strings.Repeat("x", 256)}); status != 400 {
		t.Errorf("clone with a 256-character summary: %d, want 400", status)
	}
	if status, _ := paul.do("POST", "/api/issues/"+demo+"/clone", map[string]any{"colour": "red"}); status != 400 {
		t.Errorf("clone with an unknown field: %d, want 400", status)
	}

	// Seen but not allowed to create: an archived project is read-only.
	project := newProject(t, admin, nil)
	frozen := createIssue(t, admin, project, "Dans un projet archivé", "")
	admin.call("POST", "/api/projects/"+project+"/archive", nil, 200, nil)
	if status, body := paul.do("POST", "/api/issues/"+frozen+"/clone", map[string]any{}); status != 403 {
		t.Errorf("clone in an archived project: %d %s, want 403", status, body)
	}
	if n := dbInt(t, `SELECT COUNT(*) FROM issues i JOIN projects p ON p.id = i.project_id WHERE p.pkey = ?`, project); n != 1 {
		t.Errorf("a refused clone left %d issues in the project, want 1", n)
	}
}

func TestCloneReportsAFileItCannotCopy(t *testing.T) {
	paul := login(t, "paul.martin", "motdepasse")
	orig := createIssue(t, paul, "DEMO", "Avec des pièces jointes", "")
	origID := issueIDOf(t, orig)
	// One row whose file is not in the data folder, one that was never imported.
	for _, row := range []struct {
		name    string
		missing bool
	}{{"absent.txt", false}, {"jamais-importe.txt", true}} {
		if _, err := testDB.Exec(`INSERT INTO attachments (issue_id, filename, mime_type, size, created_at, file_missing)
			VALUES (?, ?, 'text/plain', 3, NOW(3), ?)`, origID, row.name, row.missing); err != nil {
			t.Fatal(err)
		}
	}
	got := cloneOf(t, paul, "en", orig, map[string]any{"includeAttachments": true})
	if len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "absent.txt") {
		t.Errorf("warnings: %q, want one about absent.txt", got.Warnings)
	}
	cloneRowID := issueIDOf(t, got.Key)
	if n := dbInt(t, `SELECT COUNT(*) FROM attachments WHERE issue_id = ? AND filename = 'absent.txt'`, cloneRowID); n != 0 {
		t.Error("the clone keeps a row for a file that could not be copied")
	}
	if n := dbInt(t, `SELECT COUNT(*) FROM attachments WHERE issue_id = ? AND filename = 'jamais-importe.txt'
		AND file_missing`, cloneRowID); n != 1 {
		t.Error("a never-imported attachment is not carried over as such")
	}
	var d issueDetail
	paul.call("GET", "/api/issues/"+got.Key, nil, 200, &d)
	if len(d.Attachments) != 1 {
		t.Errorf("attachments of the clone: %+v", d.Attachments)
	}
}
