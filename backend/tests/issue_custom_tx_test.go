package tests

// Custom field values written with the rest of an issue: one transaction,
// one history group, and the creation announced once they exist.

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"testing"
)

// txField creates a custom field of DEMO, deleted when the test ends.
func txField(t *testing.T, admin *client, kind string, options ...string) (id int64, name string) {
	t.Helper()
	name = unique("Champ transaction ")
	in := map[string]any{"name": name, "type": kind, "context": map[string]any{"projects": []string{"DEMO"}}}
	if len(options) > 0 {
		in["options"] = options
	}
	var f fieldDetail
	admin.call("POST", "/api/admin/fields", in, 201, &f)
	t.Cleanup(func() {
		admin.do("DELETE", fmt.Sprintf("/api/admin/fields/%d", f.ID), map[string]string{"confirm": name})
	})
	return f.ID, name
}

// historyGroups lists an issue's history groups, oldest first, each as its
// "field_type:field" lines.
func historyGroups(t *testing.T, key string) [][]string {
	t.Helper()
	rows, err := testDB.Query(`SELECT g.id, ci.field_type, ci.field FROM change_groups g
		JOIN change_items ci ON ci.group_id = g.id WHERE g.issue_id = ? ORDER BY g.id, ci.id`, issueID(t, key))
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out [][]string
	last := int64(-1)
	for rows.Next() {
		var group int64
		var fieldType, field string
		if err := rows.Scan(&group, &fieldType, &field); err != nil {
			t.Fatal(err)
		}
		if group != last {
			out = append(out, nil)
			last = group
		}
		out[len(out)-1] = append(out[len(out)-1], fieldType+":"+field)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func deleteAfter(t *testing.T, key string) {
	t.Cleanup(func() { call(t, nil, testURL, "DELETE", "/rest/api/2/issue/"+key, nil, basic("admin", "admin")) })
}

func TestCreateWritesCustomFieldsWithTheIssue(t *testing.T) {
	admin := login(t, "admin", "admin")
	paul := login(t, "paul.martin", "motdepasse")
	field, name := txField(t, admin, "select", "Bas", "Haut")
	demo := `(SELECT id FROM projects WHERE pkey = 'DEMO')`
	component := dbInt(t, `SELECT id FROM components WHERE project_id = `+demo+` ORDER BY id LIMIT 1`)
	version := dbInt(t, `SELECT id FROM versions WHERE project_id = `+demo+` ORDER BY id LIMIT 1`)
	epic := pick(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		JOIN issue_types t ON t.id = i.issue_type_id WHERE p.pkey = 'DEMO' AND t.name = 'Epic' ORDER BY i.id LIMIT 1`)

	var c struct{ Key string }
	paul.call("POST", "/api/issues", map[string]any{"project": "DEMO", "type": pick(t, `SELECT id FROM issue_types WHERE name = 'Story'`),
		"summary": "Tout à la création", "securityLevel": levelID(t, "Équipe"), "components": []int64{component},
		"fixVersions": []int64{version}, "affectsVersions": []int64{version}, "epic": epic,
		"customFields": map[string]any{strconv.FormatInt(field, 10): "Haut"}}, 201, &c)
	deleteAfter(t, c.Key)

	var d issueDetail
	paul.get("/api/issues/"+c.Key, &d)
	if got := levelName(t, paul, c.Key); got != "Équipe" {
		t.Errorf("level: %q", got)
	}
	if len(d.Components) != 1 || d.Components[0].ID != component {
		t.Errorf("components: %+v", d.Components)
	}
	if len(d.FixVersions) != 1 || d.FixVersions[0].ID != version || len(d.AffectsVersions) != 1 ||
		d.AffectsVersions[0].ID != version {
		t.Errorf("versions: fix %+v, affects %+v", d.FixVersions, d.AffectsVersions)
	}
	if d.Epic == nil || d.Epic.Key != epic {
		t.Errorf("epic: %+v, want %s", d.Epic, epic)
	}
	if !slices.ContainsFunc(d.CustomFields, func(v customValue) bool {
		return v.ID == field && slices.Equal(v.Values, []string{"Haut"})
	}) {
		t.Errorf("custom values: %+v", d.CustomFields)
	}
	if got, want := historyGroups(t, c.Key), [][]string{{"custom:" + name}}; !slices.EqualFunc(got, want, slices.Equal) {
		t.Errorf("history: %v, want %v", got, want)
	}
}

func TestEditWritesCustomFieldsInOneHistoryGroup(t *testing.T) {
	admin := login(t, "admin", "admin")
	paul := login(t, "paul.martin", "motdepasse")
	field, name := txField(t, admin, "select", "Bas", "Haut")
	key := scratchIssue(t, paul, "Édition mêlée")

	paul.call("PATCH", "/api/issues/"+key, map[string]any{"summary": "Édition mêlée, bis",
		"customFields": map[string]any{strconv.FormatInt(field, 10): "Bas"}}, 200, nil)
	paul.call("PATCH", "/api/issues/"+key, map[string]any{"summary": "Édition simple"}, 200, nil)
	paul.call("PATCH", "/api/issues/"+key, map[string]any{"customFields": map[string]any{strconv.FormatInt(field, 10): "Haut"}}, 200, nil)
	want := [][]string{{"jira:summary", "custom:" + name}, {"jira:summary"}, {"custom:" + name}}
	if got := historyGroups(t, key); !slices.EqualFunc(got, want, slices.Equal) {
		t.Errorf("history: %v, want %v", got, want)
	}
}

func TestCustomFieldFailureWritesNothing(t *testing.T) {
	admin := login(t, "admin", "admin")
	paul := login(t, "paul.martin", "motdepasse")
	field, _ := txField(t, admin, "text")
	// The database refuses this field's values: the write fails after
	// everything else was checked and written.
	trigger := unique("refuse_custom_")
	if _, err := testDB.Exec(fmt.Sprintf(`CREATE TRIGGER %s BEFORE INSERT ON custom_field_values FOR EACH ROW
		BEGIN IF NEW.field_id = %d THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'refused by the test'; END IF; END`,
		trigger, field)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { testDB.Exec(`DROP TRIGGER IF EXISTS ` + trigger) })
	refused := map[string]any{strconv.FormatInt(field, 10): "refusé"}

	summary := unique("Création refusée ")
	if status, body := paul.do("POST", "/api/issues", map[string]any{"project": "DEMO", "type": standardType(t, paul, "DEMO"),
		"summary": summary, "customFields": refused}); status != 500 {
		t.Errorf("create: %d %s, want 500", status, body)
	}
	if n := dbInt(t, `SELECT COUNT(*) FROM issues WHERE summary = ?`, summary); n != 0 {
		t.Errorf("%d issues created by a failed creation", n)
	}

	key := scratchIssue(t, paul, "Édition refusée")
	if status, body := paul.do("PATCH", "/api/issues/"+key, map[string]any{"summary": "Ne doit pas rester",
		"customFields": refused}); status != 500 {
		t.Errorf("edit: %d %s, want 500", status, body)
	}
	var d issueDetail
	paul.get("/api/issues/"+key, &d)
	if d.Summary != "Édition refusée" {
		t.Errorf("summary after a failed edit: %q", d.Summary)
	}
	if got := historyGroups(t, key); len(got) != 0 {
		t.Errorf("history after a failed edit: %v", got)
	}
}

func TestCreationEventSeesCustomFields(t *testing.T) {
	env := newNotifyEnv(t)
	admin := env.login("admin", "admin")
	paul := env.login("paul.martin", "motdepasse")
	field, _ := txField(t, admin, "select", "Bas", "Haut")
	rc := newHookReceiver(t)
	admin.call("POST", "/api/admin/webhooks", map[string]any{"name": unique("Champs "), "url": rc.srv.URL + "/hook",
		"events": []string{"jira:issue_created", "jira:issue_updated"}, "jql": fmt.Sprintf("cf[%d] = Haut", field)}, 201, nil)

	var c struct{ Key string }
	paul.call("POST", "/api/issues", map[string]any{"project": "DEMO", "type": standardType(t, paul, "DEMO"),
		"summary": "Annoncé avec ses champs", "customFields": map[string]any{strconv.FormatInt(field, 10): "Haut"}}, 201, &c)
	deleteAfter(t, c.Key)
	env.hooks.Settle()
	if err := env.hooks.DeliverWebhooks(context.Background()); err != nil {
		t.Fatal(err)
	}
	var events []string
	for _, h := range rc.take() {
		var p struct {
			WebhookEvent string `json:"webhookEvent"`
			Issue        struct{ Key string }
		}
		json.Unmarshal(h.body, &p)
		events = append(events, p.WebhookEvent+" "+p.Issue.Key)
	}
	if want := []string{"jira:issue_created " + c.Key}; !slices.Equal(events, want) {
		t.Errorf("deliveries: %v, want %v", events, want)
	}
}
