package notify

import (
	"fmt"
	"testing"

	"solstice/internal/events"
)

// A creation lists the values set, but for those the meta line shows, the
// description and the hidden fields.
func TestCreatedLines(t *testing.T) {
	desc, env, due := "Texte", "Recette", "2026-11-02"
	e := &events.Event{Kind: events.IssueCreated, Issue: &events.Issue{
		Description: &desc, Environment: &env, Due: &due, Priority: &events.Named{Name: "Haute"},
		Assignee: &events.User{ID: 3}, Security: &events.Named{Name: "Équipe"},
		Components: []events.Named{{Name: "Back"}, {Name: "API"}}, Labels: []string{"a", "b"},
		Parent: &events.Ref{Key: "DEMO-1"},
		Custom: []events.CustomValue{{Name: "Environnement client", Text: "Recette"}, {Name: "Lieu"},
			{Name: "Rank", Text: "0|i0000:"}},
	}}
	got := fmt.Sprint(linesOf(e))
	want := fmt.Sprint([]line{{Group: GroupCreated},
		{Group: GroupCreated, Field: "Component", To: "Back, API"},
		{Group: GroupCreated, Field: "labels", To: "a, b"},
		{Group: GroupCreated, Field: "duedate", To: "2026-11-02"},
		{Group: GroupCreated, Field: "environment", To: "Recette"},
		{Group: GroupCreated, Field: "parent", To: "DEMO-1"},
		{Group: GroupCreated, Field: "Environnement client", To: "Recette"},
	})
	if got != want {
		t.Errorf("lines of a creation:\n got %s\nwant %s", got, want)
	}
}
