package api

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"solstice/internal/i18n"
)

func TestPlanSprintEdit(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	day := func(d int) *time.Time { v := now.AddDate(0, 0, d); return &v }
	str := func(s string) *string { return &s }
	goal := "Livrer le panier"
	future := &sprintOut{ID: 1, Name: "Sprint 1", State: "future"}
	active := &sprintOut{ID: 2, Name: "Sprint 2", State: "active", Goal: &goal, Start: day(-3), End: day(11)}
	closed := &sprintOut{ID: 3, Name: "Sprint 0", State: "closed", Start: day(-30), End: day(-16), Completed: day(-16)}
	given := func(keys ...string) map[string]bool {
		m := map[string]bool{}
		for _, k := range keys {
			m[k] = true
		}
		return m
	}
	status := func(err error) int {
		var he *httpError
		if errors.As(err, &he) {
			return he.status
		}
		return 0
	}

	// A completed sprint is history.
	if _, err := planSprintEdit(closed, sprintInput{Name: str("x")}, given("name"), now); status(err) != http.StatusConflict {
		t.Errorf("closed: %v", err)
	}
	// A future sprint: rename, goal, planned dates.
	p, err := planSprintEdit(future, sprintInput{Name: str("  Sprint A "), Goal: &goal, Start: day(1), End: day(15)},
		given("name", "goal", "start", "end"), now)
	if err != nil || len(p.sets) != 4 || p.changed["name"] != "Sprint A" {
		t.Errorf("future edit: %+v, %v", p, err)
	}
	// Nothing really changes: nothing is written.
	p, err = planSprintEdit(active, sprintInput{Name: str("Sprint 2"), Goal: &goal, Start: active.Start, End: active.End},
		given("name", "goal", "start", "end"), now)
	if err != nil || len(p.sets) != 0 {
		t.Errorf("same values: %+v, %v", p, err)
	}
	// Clearing the goal and the dates of a future sprint.
	p, err = planSprintEdit(&sprintOut{ID: 4, Name: "S", State: "future", Goal: &goal, Start: day(1), End: day(9)},
		sprintInput{}, given("goal", "start", "end"), now)
	if err != nil || len(p.sets) != 3 || p.changed["goal"] != nil {
		t.Errorf("clear: %+v, %v", p, err)
	}
	// Refusals.
	for name, c := range map[string]struct {
		sp    *sprintOut
		in    sprintInput
		given map[string]bool
		want  int
	}{
		"empty name":             {future, sprintInput{Name: str(" ")}, given("name"), 400},
		"null name":              {future, sprintInput{}, given("name"), 400},
		"name too long":          {future, sprintInput{Name: str(string(make([]rune, 256)) + "x")}, given("name"), 400},
		"end before start":       {future, sprintInput{Start: day(5), End: day(2)}, given("start", "end"), 400},
		"end equals start":       {future, sprintInput{Start: day(5), End: day(5)}, given("start", "end"), 400},
		"end before known start": {&sprintOut{State: "future", Start: day(5)}, sprintInput{End: day(1)}, given("end"), 400},
		"active without start":   {active, sprintInput{}, given("start"), 400},
		"active without end":     {active, sprintInput{}, given("end"), 400},
		"active starts later":    {active, sprintInput{Start: day(1)}, given("start"), 400},
		"active ends before":     {active, sprintInput{End: day(-5)}, given("end"), 400},
	} {
		if _, err := planSprintEdit(c.sp, c.in, c.given, now); status(err) != c.want {
			t.Errorf("%s: %v", name, err)
		}
	}
	// An active sprint may move its end, even into the past (it is then late).
	if p, err := planSprintEdit(active, sprintInput{End: day(20)}, given("end"), now); err != nil || len(p.sets) != 1 {
		t.Errorf("active end: %+v, %v", p, err)
	}
	// The messages are keys, not sentences.
	_, err = planSprintEdit(future, sprintInput{Name: str("")}, given("name"), now)
	var he *httpError
	if !errors.As(err, &he) || he.msg.Key != i18n.SprintNameRequired {
		t.Errorf("message: %v", err)
	}
}
