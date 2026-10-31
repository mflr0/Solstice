package devseed

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"solstice/internal/issue"
)

// Days are relative to today: -3 is three days ago.

// stage is where a board issue goes on a given day.
type stage int

const (
	inProgress stage = iota
	inReview         // the second in-progress status, which software projects have
	done
)

type step struct {
	day int
	to  stage
}

// boardIssue is an issue of a board: created two days before it joins its
// sprint (or on that day, outside sprints), then moved along its steps by
// its assignee.
type boardIssue struct {
	project, summary, assignee string
	points                     float64 // Story Points; 0 for none
	day                        int     // the day it joins its sprint, or is created
	steps                      []step
}

type seedSprint struct {
	name, goal string
	start, end int
	state      issue.SprintState
	issues     []boardIssue
}

// StoryPoints is the number field the Scrum board estimates with.
const StoryPoints = "Story Points"

// ScrumBoard and KanbanBoard are the names of the boards the seed makes.
const (
	ScrumBoard  = "Demo Scrum"
	KanbanBoard = "Mobile and office Kanban"
)

var sprints = []seedSprint{
	{"Sprint 1", "Sign-in and sessions", -40, -27, issue.SprintClosed, []boardIssue{
		{Project, "Session timeout warning", "alice", 3, -41, []step{{-39, inProgress}, {-35, done}}},
		{Project, "Password reset e-mail", "hans", 5, -41, []step{{-38, inProgress}, {-34, inReview}, {-31, done}}},
		{Project, "Lock the account after failed sign-ins", "marie", 8, -41, []step{{-36, inProgress}}},
		{Project, "Show the last sign-in time", "giulia", 2, -41, []step{{-33, inProgress}, {-30, done}}},
		{Project, "Sign-in audit log", "priya", 3, -37, []step{{-36, inProgress}, {-29, done}}},
	}},
	{"Sprint 2", "Notifications", -26, -13, issue.SprintClosed, []boardIssue{
		{Project, "Daily digest e-mail", "tom", 5, -27, []step{{-25, inProgress}, {-20, done}}},
		{Project, "Mute a project's notifications", "alice", 3, -27, []step{{-24, inProgress}, {-21, inReview}, {-18, done}}},
		{Project, "Notification settings page", "marie", 8, -27, []step{{-22, inProgress}, {-15, done}}},
		{Project, "Push notifications on mobile", "hans", 13, -27, nil},
		{Project, "Mention autocomplete", "giulia", 2, -22, []step{{-20, inProgress}, {-17, done}}},
		{Project, "Unsubscribe link in e-mails", "priya", 1, -19, []step{{-17, inProgress}, {-14, done}}},
	}},
	{"Sprint 3", "Search", -5, 9, issue.SprintActive, []boardIssue{
		{Project, "Search by component", "alice", 3, -6, []step{{-4, inProgress}, {-2, done}}},
		{Project, "Saved searches in the sidebar", "hans", 5, -6, []step{{-3, inProgress}, {-1, inReview}}},
		{Project, "Highlight matches in the results", "marie", 3, -6, []step{{-2, inProgress}}},
		{Project, "Search help page", "tom", 2, -6, nil},
		{Project, "Fuzzy search for usernames", "priya", 5, -3, nil},
	}},
	{"Sprint 4", "Reports", 0, 0, issue.SprintFuture, []boardIssue{
		{Project, "Export the velocity chart", "giulia", 3, -2, nil},
		{Project, "Burndown in hours", "hans", 5, -2, nil},
	}},
}

// scrumBacklog are the Scrum board's estimated issues in no sprint.
var scrumBacklog = []boardIssue{
	{Project, "Dark mode for e-mails", "marie", 2, -10, nil},
	{Project, "Bulk edit labels", "alice", 8, -9, nil},
	{Project, "Audit log retention setting", "tom", 5, -8, nil},
}

// kanbanIssues fill every column of the Kanban board, with done issues over
// the last weeks for its flow report.
var kanbanIssues = []boardIssue{
	{"MOB", "Crash when rotating the sign-in screen", "giulia", 0, -28, []step{{-26, inProgress}, {-24, inReview}, {-22, done}}},
	{"MOB", "Dark theme for the settings", "priya", 0, -25, []step{{-23, inProgress}, {-19, done}}},
	{"MOB", "Offline mode for the issue list", "hans", 0, -24, []step{{-20, inProgress}, {-15, inReview}, {-12, done}}},
	{"MOB", "Biometric sign-in", "giulia", 0, -20, []step{{-16, inProgress}, {-9, done}}},
	{"MOB", "Compress uploaded images", "priya", 0, -18, []step{{-12, inProgress}, {-6, inReview}}},
	{"MOB", "Pull to refresh on boards", "hans", 0, -15, []step{{-4, inProgress}}},
	{"MOB", "Widget for my assigned issues", "giulia", 0, -10, []step{{-2, inProgress}}},
	{"MOB", "Tablet layout", "priya", 0, -8, nil},
	{"MOB", "Haptic feedback on drag", "hans", 0, -3, nil},
	{"OPS", "Order new chairs for room B", "marie", 0, -22, []step{{-20, inProgress}, {-14, done}}},
	{"OPS", "Renew the coffee machine contract", "tom", 0, -12, []step{{-7, inProgress}, {-3, done}}},
	{"OPS", "Fix the badge reader at the entrance", "tom", 0, -6, []step{{-1, inProgress}}},
	{"OPS", "Plan the summer party", "marie", 0, -4, nil},
}

// boardSeed makes the boards: through the API, then dated by history.
type boardSeed struct {
	ctx   context.Context
	conn  *sql.DB
	admin *client
	users *sessions
	ids   map[string]int64
	h     *history
	types map[string]string // project key → its standard issue type
	field string            // the Story Points field, as customfield_<id>
	keys  map[string]string // project/summary → key of the issues made, for walk
}

// ensureBoards creates the boards that are missing (by name), with their
// issues and sprints. An existing board is left as it is.
func ensureBoards(ctx context.Context, conn *sql.DB, base string, admin *client, ids map[string]int64,
	say func(string, ...any)) error {
	s := &boardSeed{ctx: ctx, conn: conn, admin: admin, users: newSessions(ctx, base), ids: ids,
		h: newHistory(conn), types: map[string]string{}, keys: map[string]string{}}
	for _, b := range []struct {
		name string
		make func() (int, error)
	}{{ScrumBoard, s.scrum}, {KanbanBoard, s.kanban}} {
		var exists int
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM boards WHERE name = ?`, b.name).Scan(&exists); err != nil {
			return err
		}
		if exists > 0 {
			continue
		}
		n, err := b.make()
		if err != nil {
			return fmt.Errorf("board %s: %w", b.name, err)
		}
		say("created the board %s with %d issues", b.name, n)
	}
	return nil
}

type boardOut struct {
	Board struct {
		FilterID int64 `json:"filterId"`
	} `json:"board"`
	Columns []struct {
		ID       int64 `json:"id"`
		Name     string
		Statuses []struct{ ID string }
	} `json:"columns"`
}

func (s *boardSeed) createBoard(name, kind, project string) (int64, *boardOut, error) {
	var made struct{ ID int64 }
	if err := s.admin.call("POST", "/api/boards", map[string]any{"name": name, "kind": kind, "project": project},
		&made); err != nil {
		return 0, nil, err
	}
	var b boardOut
	err := s.admin.call("GET", "/api/boards/"+strconv.FormatInt(made.ID, 10), nil, &b)
	return made.ID, &b, err
}

func (s *boardSeed) scrum() (int, error) {
	if err := s.ensureStoryPoints(); err != nil {
		return 0, err
	}
	id, _, err := s.createBoard(ScrumBoard, "scrum", Project)
	if err != nil {
		return 0, err
	}
	path := "/api/boards/" + strconv.FormatInt(id, 10)
	if err := s.admin.call("PUT", path, map[string]any{
		"estimate": s.field,
		"quickFilters": []map[string]any{
			{"name": "My issues", "query": "assignee = currentUser()"},
			{"name": "Unassigned", "query": "assignee is EMPTY"},
		},
	}, nil); err != nil {
		return 0, err
	}
	n := 0
	for _, sp := range sprints {
		var made struct{ ID int64 }
		if err := s.admin.call("POST", path+"/sprints", map[string]any{"name": sp.name, "goal": sp.goal}, &made); err != nil {
			return n, err
		}
		sprintPath := "/api/sprints/" + strconv.FormatInt(made.ID, 10)
		// Committed issues join before the start; the others, on their day.
		var later []boardIssue
		for _, is := range sp.issues {
			if sp.state != issue.SprintFuture && is.day >= sp.start {
				later = append(later, is)
				continue
			}
			if err := s.joinSprint(id, made.ID, is); err != nil {
				return n, err
			}
			n++
		}
		if sp.state == issue.SprintFuture {
			continue
		}
		if err := s.admin.call("POST", sprintPath+"/start", map[string]any{}, nil); err != nil {
			return n, err
		}
		start, end := s.h.at(sp.start), s.h.day0.AddDate(0, 0, sp.end).Add(17*time.Hour)
		var completed *time.Time
		if sp.state == issue.SprintClosed {
			completed = &end
		}
		if err := s.h.sprint(s.ctx, made.ID, start, end, nil); err != nil {
			return n, err
		}
		for _, is := range later {
			if err := s.joinSprint(id, made.ID, is); err != nil {
				return n, err
			}
			n++
		}
		if err := s.walk(sp.issues); err != nil {
			return n, err
		}
		if sp.state == issue.SprintClosed {
			if err := s.admin.call("POST", sprintPath+"/complete", map[string]any{}, nil); err != nil {
				return n, err
			}
			if err := s.h.sprint(s.ctx, made.ID, start, end, completed); err != nil {
				return n, err
			}
		}
	}
	for _, is := range scrumBacklog {
		if _, err := s.create(is, is.day); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// joinSprint creates the issue and adds it to the sprint, on its day.
func (s *boardSeed) joinSprint(board, sprint int64, is boardIssue) error {
	key, err := s.create(is, is.day-2)
	if err != nil {
		return err
	}
	if err := s.admin.call("POST", "/api/boards/"+strconv.FormatInt(board, 10)+"/move",
		map[string]any{"issue": key, "sprint": sprint}, nil); err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	return s.h.changed(s.ctx, key, s.h.at(is.day))
}

// create makes the issue as its assignee, dated day.
func (s *boardSeed) create(is boardIssue, day int) (string, error) {
	typ, err := s.issueType(is.project)
	if err != nil {
		return "", err
	}
	c, err := s.users.as(is.assignee)
	if err != nil {
		return "", err
	}
	fields := map[string]any{"project": is.project, "type": typ, "summary": is.summary, "assignee": s.ids[is.assignee]}
	if is.points > 0 {
		fields["customFields"] = map[string]any{s.field: is.points}
	}
	var made struct{ Key string }
	if err := c.call("POST", "/api/issues", fields, &made); err != nil {
		return "", fmt.Errorf("%q: %w", is.summary, err)
	}
	s.keys[is.project+"/"+is.summary] = made.Key
	return made.Key, s.h.created(s.ctx, made.Key, s.h.at(day))
}

// walk moves the issues along their steps, in the order of the days.
func (s *boardSeed) walk(issues []boardIssue) error {
	type move struct {
		key, assignee string
		step
	}
	var moves []move
	for _, is := range issues {
		for _, st := range is.steps {
			moves = append(moves, move{s.keys[is.project+"/"+is.summary], is.assignee, st})
		}
	}
	slices.SortStableFunc(moves, func(a, b move) int { return cmp.Compare(a.day, b.day) })
	for _, m := range moves {
		c, err := s.users.as(m.assignee)
		if err != nil {
			return err
		}
		switch m.to {
		case inProgress:
			err = moveToNth(c, m.key, issue.Doing, 0)
		case inReview:
			err = moveToNth(c, m.key, issue.Doing, 1)
		case done:
			err = moveTo(c, m.key, issue.Done)
		}
		if err != nil {
			return fmt.Errorf("%s: %w", m.key, err)
		}
		if err := s.h.changed(s.ctx, m.key, s.h.at(m.day)); err != nil {
			return err
		}
	}
	return nil
}

func (s *boardSeed) kanban() (int, error) {
	id, b, err := s.createBoard(KanbanBoard, "kanban", "MOB")
	if err != nil {
		return 0, err
	}
	// The filter spans the office project too; the board stays MOB's.
	if err := s.admin.call("PATCH", "/api/filters/"+strconv.FormatInt(b.Board.FilterID, 10),
		map[string]any{"query": `project in (MOB, OPS) ORDER BY Rank ASC`}, nil); err != nil {
		return 0, err
	}
	// To Do, In Progress (In Progress, In Review), Done: In Review gets its own
	// column, In Progress a limit, To Do the backlog page.
	if len(b.Columns) != 3 || len(b.Columns[1].Statuses) != 2 {
		return 0, errors.New("unexpected default columns")
	}
	statuses := func(i int) []string {
		var out []string
		for _, st := range b.Columns[i].Statuses {
			out = append(out, st.ID)
		}
		return out
	}
	columns := []map[string]any{
		{"id": b.Columns[0].ID, "name": b.Columns[0].Name, "statuses": statuses(0)},
		{"id": b.Columns[1].ID, "name": b.Columns[1].Name, "max": 3, "statuses": statuses(1)[:1]},
		{"name": "Review", "statuses": statuses(1)[1:]},
		{"id": b.Columns[2].ID, "name": b.Columns[2].Name, "statuses": statuses(2)},
	}
	if err := s.admin.call("PUT", "/api/boards/"+strconv.FormatInt(id, 10), map[string]any{
		"columns":       columns,
		"kanbanBacklog": true,
		"swimlanes":     "assignee",
		"quickFilters": []map[string]any{
			{"name": "Mobile", "query": "project = MOB"},
			{"name": "Office", "query": "project = OPS"},
			{"name": "My issues", "query": "assignee = currentUser()"},
		},
	}, nil); err != nil {
		return 0, err
	}
	for _, is := range kanbanIssues {
		if _, err := s.create(is, is.day); err != nil {
			return 0, err
		}
	}
	return len(kanbanIssues), s.walk(kanbanIssues)
}

// issueType is the first standard issue type of a project.
func (s *boardSeed) issueType(project string) (string, error) {
	if t, ok := s.types[project]; ok {
		return t, nil
	}
	var p struct {
		IssueTypes []struct {
			ID      string
			Subtask bool
		}
	}
	if err := s.admin.call("GET", "/api/projects/"+project, nil, &p); err != nil {
		return "", err
	}
	for _, t := range p.IssueTypes {
		if !t.Subtask {
			s.types[project] = t.ID
			return t.ID, nil
		}
	}
	return "", fmt.Errorf("the project %s has no standard issue type", project)
}

// ensureStoryPoints finds the Story Points field, or creates it.
func (s *boardSeed) ensureStoryPoints() error {
	var id int64
	err := s.conn.QueryRowContext(s.ctx, `SELECT id FROM custom_fields WHERE name = ?`, StoryPoints).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		var made struct{ ID int64 }
		if err := s.admin.call("POST", "/api/admin/fields", map[string]any{"name": StoryPoints, "type": "number"},
			&made); err != nil {
			return err
		}
		id = made.ID
	} else if err != nil {
		return err
	}
	s.field = "customfield_" + strconv.FormatInt(id, 10)
	return nil
}
