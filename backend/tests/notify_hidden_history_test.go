package tests

// E-mail and page each decide which history lines name an issue the reader
// cannot see: link and epic lines, with the other issue under its current
// project key or under a key its project had before a rename.

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNotifyHidesInvisibleIssues(t *testing.T) {
	env := newNotifyEnv(t)
	admin := env.login("admin", "admin")
	lea := env.login("lea.richard", "motdepasse")   // rh: makes the changes
	paul := env.login("paul.martin", "motdepasse")  // not rh
	chloe := env.login("chloe.simon", "motdepasse") // rh
	linkType := pick(t, `SELECT id FROM link_types ORDER BY position LIMIT 1`)

	for _, rename := range []bool{false, true} {
		name := "current key"
		if rename {
			name = "former key"
		}
		t.Run(name, func(t *testing.T) {
			// A project only rh sees, with an epic and a task.
			hidden := newProject(t, admin, map[string]any{"access": map[string]any{"holder": "group", "id": groupID(t, "rh")}})
			var p struct {
				IssueTypes []typeRef `json:"issueTypes"`
			}
			lea.call("GET", "/api/projects/"+hidden, nil, 200, &p)
			var epicType string
			for _, it := range p.IssueTypes {
				if strings.EqualFold(it.Name, "epic") {
					epicType = it.ID
				}
			}
			if epicType == "" {
				t.Fatalf("%s has no epic type", hidden)
			}
			var created struct{ Key string }
			lea.call("POST", "/api/issues", map[string]any{"project": hidden, "type": epicType, "summary": "Recrutement"},
				201, &created)
			epic := created.Key
			task := createIssue(t, lea, hidden, "Entretien confidentiel", "")

			// A WEB issue both watch: a link and an epic naming the hidden
			// issues, and a summary change everyone may read.
			visible := createIssue(t, lea, "WEB", "Page carrières", "")
			paul.call("PUT", "/api/issues/"+visible+"/watch", nil, 200, nil)
			chloe.call("PUT", "/api/issues/"+visible+"/watch", nil, 200, nil)
			env.deliver()
			lea.call("POST", "/api/issues/"+visible+"/links", map[string]any{"type": json.Number(linkType),
				"direction": "outward", "target": task}, 201, nil)
			lea.call("PATCH", "/api/issues/"+visible, map[string]any{"summary": "Page carrières v2", "epic": epic}, 200, nil)

			if rename {
				fresh := newProjectKey()
				admin.call("POST", "/api/projects/"+hidden+"/rename", map[string]any{"newKey": fresh, "confirm": hidden}, 200, nil)
				t.Cleanup(func() { admin.do("DELETE", "/api/projects/"+fresh, map[string]string{"confirm": fresh}) })
			}

			msgs := env.deliver()
			about := func(addr string) string {
				t.Helper()
				var got []string
				for _, m := range to(msgs, addr) {
					if strings.Contains(m.Subject, "["+visible+"]") {
						got = append(got, m.Text+m.HTML)
					}
				}
				if len(got) != 1 {
					t.Fatalf("%s got %d e-mails about %s, want 1", addr, len(got), visible)
				}
				return got[0]
			}
			pm := about("paul.martin@exemple.fr")
			if !strings.Contains(pm, "Page carri") {
				t.Errorf("paul.martin's e-mail lacks the summary change:\n%s", pm)
			}
			cm := about("chloe.simon@exemple.fr")
			for _, key := range []string{task, epic} {
				if strings.Contains(pm, key) {
					t.Errorf("paul.martin's e-mail names %s, which he cannot see", key)
				}
				if !strings.Contains(cm, key) {
					t.Errorf("chloe.simon's e-mail lacks %s", key)
				}
			}

			_, raw := paul.do("GET", "/api/issues/"+visible+"/history", nil)
			_, chloeRaw := chloe.do("GET", "/api/issues/"+visible+"/history", nil)
			for _, key := range []string{task, epic} {
				if strings.Contains(string(raw), key) {
					t.Errorf("paul.martin's history of %s names %s", visible, key)
				}
				if !strings.Contains(string(chloeRaw), key) {
					t.Errorf("chloe.simon's history of %s lacks %s", visible, key)
				}
			}
		})
	}
}
