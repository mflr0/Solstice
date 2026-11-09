package devseed

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"solstice/internal/issue"
	"solstice/internal/notify"
)

// seedFilter is a saved filter of a mock user. share is "" for a private
// filter, "everyone", or "group:", "project:" or "user:" and a name. Its
// columns leave out key and summary, which every list shows; nil for the
// default ones.
type seedFilter struct {
	owner, name, query, share string
	columns                   []string
}

var filters = []seedFilter{
	{"alice", "Open DEMO bugs", `project = DEMO AND type = Bug AND resolution is EMPTY ORDER BY priority DESC`,
		"everyone", []string{"status", "priority", "assignee"}},
	{"alice", "My open work", `assignee = currentUser() AND resolution is EMPTY ORDER BY updated DESC`,
		"", nil},
	{"hans", "Mobile – offen", `project = MOB AND resolution is EMPTY ORDER BY priority DESC`,
		"group:" + Developers, nil},
	{"hans", "Große Stories", `"Story points" > 4 ORDER BY "Story points" DESC`, "", nil},
	{"marie", "Bureau en cours", `project = OPS AND resolution is EMPTY ORDER BY created DESC`,
		"group:office", nil},
	{"priya", "Overdue", `duedate < now() AND resolution is EMPTY ORDER BY duedate ASC`,
		"project:" + Project, []string{"assignee", "duedate"}},
	{"giulia", "Etichetta auth", `labels = auth`, "user:alice", nil},
}

// seedSubscription mails a filter's results to its subscriber every day.
type seedSubscription struct {
	user, filter string
	hour         int
}

var subscriptions = []seedSubscription{{"alice", "Open DEMO bugs", 8}}

// seedGadget is a dashboard gadget, on a filter for results.
type seedGadget struct {
	kind, filter string
	config       map[string]any
}

var dashboards = map[string][]seedGadget{
	"alice": {
		{"results", "Open DEMO bugs", map[string]any{"size": 10}},
		{"results", "Overdue", map[string]any{"size": 10}},
		{"assigned", "", map[string]any{"size": 10}},
	},
	"hans": {
		{"results", "Mobile – offen", map[string]any{"size": 10}},
		{"results", "Große Stories", map[string]any{"size": 10}},
		{"results", "Open DEMO bugs", map[string]any{"size": 5}},
		{"recent", "", map[string]any{"size": 10}},
	},
	"marie": {
		{"results", "Bureau en cours", map[string]any{"size": 10}},
		{"assigned", "", map[string]any{"size": 10}},
		{"results", "Open DEMO bugs", map[string]any{}},
	},
	"giulia": withBuiltins(seedGadget{"results", "Mobile – offen", map[string]any{}}),
	"priya":  withBuiltins(seedGadget{"results", "Overdue", map[string]any{}}),
	"tom":    withBuiltins(seedGadget{"results", "Bureau en cours", map[string]any{}}),
}

// withBuiltins is the default dashboard followed by these gadgets, as an
// import makes it from Jira favourites.
func withBuiltins(gadgets ...seedGadget) []seedGadget {
	return append([]seedGadget{
		{"assigned", "", map[string]any{"size": 15}},
		{"reported", "", map[string]any{"size": 15}},
		{"recent", "", map[string]any{"size": 12}},
	}, gadgets...)
}

// notificationPrefs are the notification preferences some users set.
var notificationPrefs = map[string]map[string]any{
	"hans": {"delivery": notify.Digest, "digestHour": 9, "dueReminder": true,
		"events": map[string]bool{notify.GroupWorklogs: false, notify.GroupAttachments: false}},
}

// ensureFilters creates the saved filters that are missing (by owner and
// name), with their subscriptions, then the dashboards and
// notification preferences of users who have none. What exists is left as
// it is.
func ensureFilters(ctx context.Context, conn *sql.DB, base string, ids map[string]int64, say func(string, ...any)) error {
	users := newSessions(ctx, base)
	filterIDs := map[string]int64{}
	for _, f := range filters {
		id, err := ensureFilter(ctx, conn, users, ids, f, say)
		if err != nil {
			return fmt.Errorf("filter %s: %w", f.name, err)
		}
		filterIDs[f.name] = id
	}
	for _, username := range slices.Sorted(maps.Keys(dashboards)) {
		if err := ensureDashboard(ctx, conn, users, ids[username], username, filterIDs, say); err != nil {
			return fmt.Errorf("dashboard of %s: %w", username, err)
		}
	}
	for username, prefs := range notificationPrefs {
		var n int
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_prefs WHERE user_id = ?`,
			ids[username]).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		c, err := users.as(username)
		if err != nil {
			return err
		}
		if err := c.call("PUT", "/api/me/notifications", prefs, nil); err != nil {
			return fmt.Errorf("notifications of %s: %w", username, err)
		}
		say("set the notification preferences of %s", username)
	}
	return nil
}

func ensureFilter(ctx context.Context, conn *sql.DB, users *sessions, ids map[string]int64, f seedFilter,
	say func(string, ...any)) (int64, error) {
	var id int64
	err := conn.QueryRowContext(ctx, `SELECT id FROM filters WHERE owner_id = ? AND name = ?`, ids[f.owner], f.name).
		Scan(&id)
	if err == nil || !errors.Is(err, sql.ErrNoRows) {
		return id, err
	}
	shares := []map[string]any{}
	if f.share != "" {
		kind, name, _ := strings.Cut(f.share, ":")
		sh := map[string]any{"kind": kind}
		var q string
		switch issue.ShareKind(kind) {
		case issue.ShareGroup:
			q = `SELECT id FROM user_groups WHERE name = ?`
		case issue.ShareProject:
			q = `SELECT id FROM projects WHERE pkey = ?`
		case issue.ShareUser:
			sh["id"] = ids[name]
		}
		if q != "" {
			var shareID int64
			if err := conn.QueryRowContext(ctx, q, name).Scan(&shareID); err != nil {
				return 0, fmt.Errorf("share with %s: %w", f.share, err)
			}
			sh["id"] = shareID
		}
		shares = append(shares, sh)
	}
	c, err := users.as(f.owner)
	if err != nil {
		return 0, err
	}
	var made struct{ ID int64 }
	in := map[string]any{"name": f.name, "query": f.query, "shares": shares}
	if f.columns != nil {
		in["columns"] = f.columns
	}
	if err := c.call("POST", "/api/filters", in, &made); err != nil {
		return 0, err
	}
	path := "/api/filters/" + strconv.FormatInt(made.ID, 10)
	for _, s := range subscriptions {
		if s.filter != f.name {
			continue
		}
		sc, err := users.as(s.user)
		if err != nil {
			return 0, err
		}
		if err := sc.call("POST", path+"/subscriptions", map[string]any{"schedule": "daily", "hour": s.hour},
			nil); err != nil {
			return 0, fmt.Errorf("subscription of %s: %w", s.user, err)
		}
	}
	say("created the filter %s of %s", f.name, f.owner)
	return made.ID, nil
}

func ensureDashboard(ctx context.Context, conn *sql.DB, users *sessions, userID int64, username string,
	filterIDs map[string]int64, say func(string, ...any)) error {
	var n int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM dashboards WHERE user_id = ?`, userID).Scan(&n); err != nil ||
		n > 0 {
		return err
	}
	var gadgets []map[string]any
	for _, g := range dashboards[username] {
		in := map[string]any{"kind": g.kind, "config": g.config}
		if g.filter != "" {
			id, ok := filterIDs[g.filter]
			if !ok {
				return fmt.Errorf("no filter %s", g.filter)
			}
			in["filter"] = id
		}
		gadgets = append(gadgets, in)
	}
	c, err := users.as(username)
	if err != nil {
		return err
	}
	if err := c.call("PUT", "/api/dashboard", map[string]any{"gadgets": gadgets}, nil); err != nil {
		return err
	}
	say("made the dashboard of %s", username)
	return nil
}
