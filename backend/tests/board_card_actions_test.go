package tests

// Board and backlog cards say what the user may do with the issue and
// whether they watch it, so the card menu needs no request per card.

import (
	"fmt"
	"testing"

	"solstice/internal/jirafixture"
)

func TestBoardCardActions(t *testing.T) {
	admin := login(t, "admin", "admin")
	paul := login(t, "paul.martin", jirafixture.Password) // developpeurs
	key := newProject(t, admin, nil)

	// The developers may see the issues but not edit them.
	var perms struct {
		Grants []grantOut `json:"grants"`
	}
	admin.call("GET", "/api/projects/"+key+"/permissions", nil, 200, &perms)
	kept := []grantOut{}
	for _, g := range perms.Grants {
		if g.Permission != "edit" || g.Holder != "group" {
			kept = append(kept, g)
		}
	}
	admin.call("PUT", "/api/projects/"+key+"/permissions", map[string]any{"grants": kept}, 200, nil)

	watched := createIssue(t, admin, key, "Suivie", "")
	other := createIssue(t, admin, key, "Pas suivie", "")
	paul.call("PUT", "/api/issues/"+watched+"/watch", nil, 200, nil)

	type cardOut struct {
		Key string
		Can struct {
			Create bool `json:"create"`
			Edit   bool `json:"edit"`
		} `json:"can"`
		Watching bool `json:"watching"`
	}
	cards := func(c *client, path string) map[string]cardOut {
		var out struct{ Cards, Backlog []cardOut }
		c.call("GET", path, nil, 200, &out)
		byKey := map[string]cardOut{}
		for _, card := range append(out.Cards, out.Backlog...) {
			byKey[card.Key] = card
		}
		return byKey
	}

	for kind, suffix := range map[string]string{"kanban": "/cards", "scrum": "/backlog"} {
		var created struct{ ID int64 }
		admin.call("POST", "/api/boards", map[string]any{"kind": kind, "project": key}, 201, &created)
		path := fmt.Sprintf("/api/boards/%d", created.ID)
		t.Cleanup(func() { admin.do("DELETE", path, nil) })

		for who, c := range map[string]*client{"admin": admin, "paul": paul} {
			got := cards(c, path+suffix)
			for _, k := range []string{watched, other} {
				card, ok := got[k]
				if !ok {
					t.Fatalf("%s %s: %s missing from %v", kind, who, k, got)
				}
				if want := who == "admin"; card.Can.Edit != want {
					t.Errorf("%s %s %s: can.edit %v, want %v", kind, who, k, card.Can.Edit, want)
				}
				if !card.Can.Create {
					t.Errorf("%s %s %s: can.create false", kind, who, k)
				}
			}
			if who == "paul" && (!got[watched].Watching || got[other].Watching) {
				t.Errorf("%s paul: watching %v and %v, want true and false",
					kind, got[watched].Watching, got[other].Watching)
			}
		}
	}
}
