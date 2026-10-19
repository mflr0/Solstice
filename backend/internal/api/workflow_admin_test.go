package api

import (
	"errors"
	"net/http"
	"reflect"
	"testing"

	"solstice/internal/i18n"
	"solstice/internal/issue"
)

// Which issues follow a workflow once saved, and which issue types leave it
// for the default, for workflow 3.
func TestWorkflowUpdatePlan(t *testing.T) {
	for _, c := range []struct {
		name      string
		isDefault bool
		in        workflowInput
		explicit  map[string]int64
		cond      string
		args      []any
		leaving   []any
	}{
		{"default, nothing mapped elsewhere", true, workflowInput{}, map[string]int64{}, "TRUE", nil, nil},
		{"default, types of other workflows left out, sorted", true, workflowInput{},
			map[string]int64{"story": 2, "bug": 1, "task": 3},
			"issue_type_id NOT IN (?, ?)", []any{"bug", "story"}, nil},
		{"made the default, its own list ignored", false, workflowInput{Default: true, IssueTypes: []string{"task"}},
			map[string]int64{"task": 3, "bug": 2},
			"issue_type_id NOT IN (?)", []any{"bug"}, nil},
		{"types kept, added and leaving", false, workflowInput{IssueTypes: []string{"task", "story"}},
			map[string]int64{"task": 3, "epic": 3, "bug": 3, "sub": 2},
			"issue_type_id IN (?, ?)", []any{"task", "story"}, []any{"bug", "epic"}},
		{"no type left", false, workflowInput{}, map[string]int64{"task": 3, "bug": 3},
			"FALSE", nil, []any{"bug", "task"}},
	} {
		u := &workflowUpdate{id: 3, projectID: 7, isDefault: c.isDefault, in: c.in}
		u.plan(c.explicit)
		if u.followCond != c.cond || !reflect.DeepEqual(u.followArgs, c.args) || !reflect.DeepEqual(u.leaving, c.leaving) {
			t.Errorf("%s: plan = %q %v, leaving %v; want %q %v, leaving %v", c.name,
				u.followCond, u.followArgs, u.leaving, c.cond, c.args, c.leaving)
		}
	}
}

// What check refuses in an update, in order, and what it keeps.
func TestWorkflowUpdateCheck(t *testing.T) {
	cat := &catalog{status: map[string]statusRef{
		"1": {ID: "1", Name: "Open", Category: issue.Todo},
		"2": {ID: "2", Name: "Done", Category: issue.Done},
	}}
	one, two, other := "1", "2", "9"
	ok := workflowInput{Name: "  Flow ", Statuses: []string{"1", "2"}, Initial: "1",
		Transitions: []transitionInput{{Name: "Close", From: &one, To: "2"}, {Name: "Reopen", To: "1"}}}
	with := func(f func(*workflowInput)) workflowInput {
		in := ok
		in.Transitions = append([]transitionInput(nil), ok.Transitions...)
		f(&in)
		return in
	}
	for _, c := range []struct {
		name string
		in   workflowInput
		key  i18n.Key
	}{
		{"blank name", with(func(in *workflowInput) { in.Name = "  " }), i18n.WorkflowNameRequired},
		{"no status", with(func(in *workflowInput) { in.Statuses = nil }), i18n.WorkflowStatusRequired},
		{"unknown status", with(func(in *workflowInput) { in.Statuses = []string{"1", "9"} }), i18n.StatusNameUnknown},
		{"status twice", with(func(in *workflowInput) { in.Statuses = []string{"1", "1"} }), i18n.WorkflowStatusTwice},
		{"initial dropped", with(func(in *workflowInput) { in.Initial = "9" }), i18n.WorkflowInitialStatus},
		{"transition without a name", with(func(in *workflowInput) { in.Transitions[0].Name = " " }),
			i18n.WorkflowTransitionNameRequired},
		{"transition from a dropped status", with(func(in *workflowInput) { in.Transitions[0].From = &other }),
			i18n.WorkflowTransitionStatus},
		{"transition to its own status", with(func(in *workflowInput) { in.Transitions[0].From = &two }),
			i18n.WorkflowTransitionNoTarget},
	} {
		var he *httpError
		err := (&workflowUpdate{in: c.in}).check(cat)
		if !errors.As(err, &he) || he.status != http.StatusBadRequest || he.msg.Key != c.key {
			t.Errorf("%s: check = %v, want 400 %s", c.name, err, c.key)
		}
	}
	u := &workflowUpdate{in: ok}
	if err := u.check(cat); err != nil {
		t.Fatal(err)
	}
	if u.name != "Flow" || !reflect.DeepEqual(u.keep, map[string]bool{"1": true, "2": true}) {
		t.Errorf("check kept %q %v", u.name, u.keep)
	}
}
