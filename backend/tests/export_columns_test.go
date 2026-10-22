package tests

import (
	"maps"
	"net/url"
	"slices"
	"strings"
	"testing"
)

// TestCSVExportEveryColumn exports an issue in every system field of the
// REST field list, plus epic and sprint, and checks each cell.
func TestCSVExportEveryColumn(t *testing.T) {
	paul := login(t, "paul.martin", "motdepasse")
	admin := login(t, "admin", "admin")
	key := createIssue(t, paul, "DEMO", "Every column", "Line 1\nLine 2")
	sub := createIssue(t, paul, "DEMO", "Every column, sub-task", "")
	other := createIssue(t, paul, "DEMO", "Every column, epic and link", "")
	t.Cleanup(func() {
		for _, k := range []string{sub, key, other} {
			admin.do("DELETE", "/api/issues/"+k, nil)
		}
	})
	id, subID, otherID := issueID(t, key), issueID(t, sub), issueID(t, other)
	paulID, chloeID, marieID := userID(t, "paul.martin"), userID(t, "chloe.simon"), userID(t, "marie.durand")
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := testDB.Exec(query, args...); err != nil {
			t.Fatalf("%v\n%s", err, query)
		}
	}
	demo := func(table, name string) string {
		return pick(t, `SELECT x.id FROM `+table+` x JOIN projects p ON p.id = x.project_id
			WHERE p.pkey = 'DEMO' AND x.name = ?`, name)
	}

	// Times are UTC in the database and exported in Europe/Paris (UTC+1 in March).
	exec(`UPDATE issues SET priority_id = (SELECT id FROM priorities WHERE name = 'High'),
		resolution_id = (SELECT id FROM resolutions WHERE name = 'Fixed'), resolved_at = '2024-03-05 16:45:00',
		assignee_id = ?, reporter_id = ?, creator_id = ?, environment = 'Firefox 125',
		security_level_id = ?, due_date = '2024-04-15', original_estimate = 9000, remaining_estimate = 5400,
		time_spent = 3600, epic_id = ? WHERE id = ?`,
		chloeID, paulID, marieID, demo("security_levels", "Restreint"), otherID, id)
	exec(`UPDATE issues SET parent_id = ? WHERE id = ?`, id, subID)
	exec(`INSERT INTO labels (issue_id, label) VALUES (?, 'zeta'), (?, 'alpha')`, id, id)
	exec(`INSERT INTO issue_components (issue_id, component_id) VALUES (?, ?), (?, ?)`,
		id, demo("components", "Paiement"), id, demo("components", "API"))
	exec(`INSERT INTO issue_versions (issue_id, version_id, kind) VALUES (?, ?, 'fix'), (?, ?, 'fix'), (?, ?, 'affects')`,
		id, demo("versions", "2.0"), id, demo("versions", "1.0"), id, demo("versions", "1.1"))
	exec(`INSERT INTO issue_links (type_id, source_id, target_id)
		SELECT id, ?, ? FROM link_types WHERE name = 'Blocks'`, id, otherID)
	exec(`INSERT INTO issue_links (type_id, source_id, target_id)
		SELECT id, ?, ? FROM link_types WHERE name = 'Duplicate'`, otherID, id)
	exec(`DELETE FROM watchers WHERE issue_id = ?`, id)
	exec(`INSERT INTO watchers (issue_id, user_id, created_at) VALUES (?, ?, NOW()), (?, ?, NOW())`,
		id, paulID, id, chloeID)
	exec(`INSERT INTO votes (issue_id, user_id, created_at) VALUES (?, ?, NOW())`, id, chloeID)
	exec(`INSERT INTO attachments (issue_id, author_id, filename, mime_type, size, created_at, file_missing)
		VALUES (?, ?, 'b.pdf', 'application/pdf', 10, '2024-03-01 10:00:00', 1),
		(?, ?, 'a.png', 'image/png', 10, '2024-03-02 10:00:00', 1)`, id, paulID, id, paulID)
	exec(`INSERT INTO comments (issue_id, author_id, body, created_at, updated_at)
		VALUES (?, ?, 'Second', '2024-03-02 08:00:00', '2024-03-02 08:00:00'),
		(?, ?, 'First', '2024-03-01 08:00:00', '2024-03-01 08:00:00')`, id, chloeID, id, paulID)
	exec(`INSERT INTO worklogs (issue_id, author_id, body, started_at, seconds, created_at, updated_at)
		VALUES (?, ?, '', '2024-03-02 13:00:00', 9000, NOW(), NOW()),
		(?, ?, '', '2024-03-01 13:00:00', 1800, NOW(), NOW())`, id, chloeID, id, paulID)
	exec(`INSERT INTO sprint_issues (sprint_id, issue_id) SELECT id, ? FROM sprints
		WHERE name IN ('Sprint 2', 'Sprint 1')`, id)
	// Last, so that nothing above moves them.
	exec(`UPDATE issues SET created_at = '2024-03-01 09:30:00', updated_at = '2024-03-06 07:05:00' WHERE id = ?`, id)

	typ := pick(t, `SELECT it.name FROM issues i JOIN issue_types it ON it.id = i.issue_type_id WHERE i.id = ?`, id)
	status := pick(t, `SELECT s.name FROM issues i JOIN statuses s ON s.id = i.status_id WHERE i.id = ?`, id)
	want := map[string]string{
		"issuekey":             key,
		"summary":              "Every column",
		"issuetype":            typ,
		"project":              "DEMO",
		"status":               status,
		"priority":             "High",
		"resolution":           "Fixed",
		"resolutiondate":       "2024-03-05 17:45",
		"assignee":             "Chloé Simon",
		"reporter":             "Paul Martin",
		"creator":              "Marie Durand",
		"created":              "2024-03-01 10:30",
		"updated":              "2024-03-06 08:05",
		"duedate":              "2024-04-15",
		"description":          "Line 1\nLine 2",
		"environment":          "Firefox 125",
		"labels":               "alpha, zeta",
		"components":           "API, Paiement",
		"fixVersions":          "1.0, 2.0",
		"versions":             "1.1",
		"issuelinks":           "blocks " + other + ", is duplicated by " + other,
		"subtasks":             sub,
		"parent":               "",
		"security":             "Restreint",
		"watches":              "2",
		"votes":                "1",
		"timeoriginalestimate": "2.5",
		"timeestimate":         "1.5",
		"timespent":            "1",
		"timetracking":         "",
		"attachment":           "b.pdf, a.png",
		"comment":              "2024-03-01 09:00; Paul Martin; First\n\n2024-03-02 09:00; Chloé Simon; Second",
		"worklog":              "2024-03-01 14:00; Paul Martin; 30m\n2024-03-02 14:00; Chloé Simon; 2h 30m",
		"epic":                 other,
		"sprint":               "Sprint 1, Sprint 2",
	}

	var fields []struct {
		ID     string
		Name   string
		Custom bool
	}
	paul.call("GET", "/rest/api/2/field", nil, 200, &fields)
	columns := []string{"epic", "sprint"}
	headers := []string{"Epic", "Sprint"}
	for _, f := range fields {
		if !f.Custom {
			columns = append(columns, f.ID)
			headers = append(headers, f.Name)
		}
	}
	for _, c := range columns {
		if _, ok := want[c]; !ok {
			t.Errorf("system field %s has no expected cell", c)
		}
	}
	for c := range want {
		if !slices.Contains(columns, c) {
			t.Errorf("%s is expected but not a column", c)
		}
	}

	q := "jql=" + url.QueryEscape("key in ("+key+", "+sub+")") + "&columns=" + strings.Join(columns, ",") +
		"&tz=Europe/Paris"
	code, _, raw := exportCSVFor(t, paul, q, "en")
	if code != 200 {
		t.Fatalf("export: %d %s", code, raw)
	}
	records := readCSV(t, raw)
	if len(records) != 3 {
		t.Fatalf("%d records, want a header and 2 rows: %q", len(records), records)
	}
	if !slices.Equal(records[0], headers) {
		t.Errorf("headers: %q, want %q", records[0], headers)
	}
	row, subRow := records[1], records[2]
	if row[slices.Index(columns, "issuekey")] != key {
		row, subRow = subRow, row
	}
	for i, c := range columns {
		if row[i] != want[c] {
			t.Errorf("%s: %q, want %q", c, row[i], want[c])
		}
	}
	if got := subRow[slices.Index(columns, "parent")]; got != key {
		t.Errorf("sub-task parent: %q, want %q", got, key)
	}
}

// TestSearchColumns asks the search for a custom field's column, which comes
// back as text next to the issue, and refuses an unknown column.
func TestSearchColumns(t *testing.T) {
	paul := login(t, "paul.martin", "motdepasse")
	admin := login(t, "admin", "admin")
	key := createIssue(t, paul, "DEMO", "Search columns", "")
	t.Cleanup(func() { admin.do("DELETE", "/api/issues/"+key, nil) })
	field := pick(t, `SELECT id FROM custom_fields WHERE name = 'Code client' ORDER BY id LIMIT 1`)
	if _, err := testDB.Exec(`INSERT INTO custom_field_values (issue_id, field_id, string_value) VALUES (?, ?, 'C-42')`,
		issueID(t, key), field); err != nil {
		t.Fatal(err)
	}
	if _, err := testDB.Exec(`UPDATE issues SET due_date = '2024-04-15' WHERE id = ?`, issueID(t, key)); err != nil {
		t.Fatal(err)
	}

	var page struct {
		Issues []struct {
			Key    string
			Values map[string]string
		}
	}
	jql := "&jql=" + url.QueryEscape("key = "+key)
	paul.call("GET", "/api/issues?columns=customfield_"+field+",due,labels"+jql, nil, 200, &page)
	if len(page.Issues) != 1 {
		t.Fatalf("%d issues, want 1", len(page.Issues))
	}
	want := map[string]string{"customfield_" + field: "C-42", "duedate": "2024-04-15", "labels": ""}
	if got := page.Issues[0].Values; !maps.Equal(got, want) {
		t.Errorf("values = %v, want %v", got, want)
	}

	// Without columns, the answer is as before: no values.
	var plain struct{ Issues []map[string]any }
	paul.call("GET", "/api/issues?"+jql[1:], nil, 200, &plain)
	if _, ok := plain.Issues[0]["values"]; ok {
		t.Errorf("values without columns: %v", plain.Issues[0])
	}

	paul.call("GET", "/api/issues?columns=nope"+jql, nil, 400, nil)

	var choices []struct{ ID, Name string }
	paul.call("GET", "/api/issues/columns", nil, 200, &choices)
	if !slices.ContainsFunc(choices, func(c struct{ ID, Name string }) bool {
		return c.ID == "customfield_"+field && c.Name == "Code client"
	}) || !slices.ContainsFunc(choices, func(c struct{ ID, Name string }) bool { return c.ID == "epic" }) {
		t.Errorf("columns: %v", choices)
	}
}
