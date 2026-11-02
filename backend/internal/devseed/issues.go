package devseed

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"solstice/internal/i18n"
	"solstice/internal/issue"
)

// demoIssue is a demo issue: who reports it, who works on it, how far it got,
// and what people did on it. Its fields are set when it is created; what is
// around it (comments, work logs, attachments, votes, watchers) is added
// whenever it is missing.
type demoIssue struct {
	ref                string   // names it for the parent, epic and links of other demo issues
	project            string   // "" for Project
	typ                i18n.Key // a DefaultType key; "" for a task
	parent, epic       string   // refs
	reporter, assignee string
	summary            string
	status             issue.StatusCategory // where it ends; "" leaves it in the first status
	resolution         i18n.Key             // with a done status; "" for the default one
	priority           i18n.Key             // "" for the default one
	labels             []string
	due                string
	estimate           string
	components         []string
	fixVersions        []string
	custom             map[string]any // values by field name, as the REST API takes them
	level              string         // a security level of its project, by name
	comments           []note
	worklogs           []worklog
	groupComments      []groupNote
	groupWorklogs      []groupWorklog
	attachments        []attachment
	votes, watchers    []string
}

type note struct{ author, body string }

// groupNote is a comment only a group sees.
type groupNote struct {
	group string
	note
}

// worklog is time someone logged, daysAgo days ago.
type worklog struct {
	author  string
	daysAgo int
	spent   time.Duration
	body    string
}

// groupWorklog is a work log only a group sees.
type groupWorklog struct {
	group string
	worklog
}

type attachment struct{ author, name string }

// demoLink links two demo issues (refs), made by a user.
type demoLink struct {
	by   string
	from string
	typ  i18n.Key // a DefaultLink…Name key
	to   string
}

// day is the date n days from today.
func day(n int) string { return time.Now().AddDate(0, 0, n).Format(time.DateOnly) }

var issues = []demoIssue{
	// DEMO: two epics, the original dozen issues, sub-tasks.
	{ref: "accounts", typ: i18n.DefaultTypeEpic, reporter: "alice", assignee: "alice",
		summary: "Sign-in and accounts", status: issue.Doing, priority: i18n.DefaultPriorityHigh,
		labels: []string{"auth"}, fixVersions: []string{"1.1"}, watchers: []string{"marie", "tom"}},
	{ref: "search", typ: i18n.DefaultTypeEpic, reporter: "tom", assignee: "hans",
		summary: "Faster search", priority: i18n.DefaultPriorityMedium, labels: []string{"search", "performance"},
		due: day(40), fixVersions: []string{"1.1"}},
	{ref: "remember", typ: i18n.DefaultTypeStory, epic: "accounts", reporter: "alice", assignee: "alice",
		summary: "Sign-in page: remember the last username", status: issue.Doing, priority: i18n.DefaultPriorityHigh,
		labels: []string{"auth", "ux"}, due: day(5), estimate: "1d", components: []string{"Frontend"},
		fixVersions: []string{"1.1"},
		custom: map[string]any{"Story points": 3, "Customer": "Acme Corp", "Impact": "Medium",
			"QA owner": "giulia", "Reviewers": []string{"marie", "hans"}, "Teams": []string{"web", "identity"},
			"Spec link": "https://example.org/specs/remember-username"},
		comments: []note{{"alice", "Working on it, the cookie is set."},
			{"marie", "Pensez à vider le cookie à la déconnexion."}},
		worklogs: []worklog{{"alice", 3, 2 * time.Hour, "Cookie and form prefill"},
			{"alice", 1, 90 * time.Minute, "Tests"}},
		votes: []string{"tom"}, watchers: []string{"giulia"}},
	{ref: "overflow", typ: i18n.DefaultTypeBug, reporter: "alice",
		summary: "Board columns overflow on small screens", priority: i18n.DefaultPriorityMedium,
		labels: []string{"ui", "mobile"}, components: []string{"Frontend"},
		custom: map[string]any{"Severity": "Major", "Browsers": []string{"Safari", "Firefox"},
			"Platforms":          []string{"Web"},
			"Steps to reproduce": "1. Open a board with five columns.\n2. Narrow the window to 390 px.\n3. The last column is cut off."},
		attachments: []attachment{{"alice", "board-overflow.png"}},
		votes:       []string{"marie", "hans", "priya"}},
	{ref: "translate", reporter: "marie", assignee: "marie", summary: "Traduire les libellés du tableau de bord",
		status: issue.Done, priority: i18n.DefaultPriorityLow, labels: []string{"i18n"}, estimate: "4h",
		fixVersions: []string{"1.0"}, components: []string{"Frontend"},
		comments: []note{{"marie", "C'est fait, à relire."}},
		worklogs: []worklog{{"marie", 20, 3 * time.Hour, "Libellés et pluriels"}}},
	{ref: "csv", typ: i18n.DefaultTypeBug, reporter: "marie", assignee: "hans",
		summary: "Export CSV : les dates sont en UTC", priority: i18n.DefaultPriorityHigh, due: day(-10),
		components: []string{"Backend"}, fixVersions: []string{"1.1"},
		custom:   map[string]any{"Severity": "Minor", "Customer": "Banque Lumière", "Target date": day(-10)},
		comments: []note{{"hans", "Ich schaue mir die Zeitzone des Exports an."}}},
	{ref: "umlaut", typ: i18n.DefaultTypeBug, epic: "search", reporter: "hans", assignee: "hans",
		summary: "Suche nach Umlauten findet nichts", status: issue.Doing, priority: i18n.DefaultPriorityVeryHigh,
		labels: []string{"search", "i18n"}, estimate: "2d", components: []string{"Backend"},
		custom:   map[string]any{"Severity": "Blocker", "Impact": "High", "Story points": 5},
		comments: []note{{"hans", "Die Sortierung der Datenbank ist schuld."}, {"alice", "Does it also break the ß?"}},
		worklogs: []worklog{{"hans", 4, 3 * time.Hour, "Kollation untersucht"},
			{"hans", 2, 150 * time.Minute, "Index neu aufgebaut"}, {"tom", 2, 30 * time.Minute, "Reproduced on staging"}},
		groupComments: []groupNote{{Developers,
			note{"hans", "Intern: der Kunde hat uns seinen Datenbank-Dump geschickt, bitte nicht weitergeben."}}},
		groupWorklogs: []groupWorklog{{Developers, worklog{"priya", 1, time.Hour, "Checked the customer's database dump"}}},
		votes:         []string{"marie", "giulia"}},
	{ref: "pdf", typ: i18n.DefaultTypeStory, reporter: "hans", assignee: "giulia",
		summary: "Attachment preview for PDF files", priority: i18n.DefaultPriorityLow, estimate: "3d",
		components: []string{"Frontend"}, custom: map[string]any{"Story points": 8},
		attachments: []attachment{{"hans", "preview-notes.txt"}}},
	{ref: "notify", typ: i18n.DefaultTypeBug, reporter: "giulia", assignee: "giulia",
		summary: "Notifiche duplicate per i commenti", status: issue.Done, priority: i18n.DefaultPriorityMedium,
		components: []string{"Backend"}, fixVersions: []string{"1.0"},
		custom:   map[string]any{"Deployed at": time.Now().AddDate(0, 0, -55).UTC().Truncate(time.Hour).Format(time.RFC3339)},
		comments: []note{{"giulia", "Risolto con il raggruppamento."}},
		worklogs: []worklog{{"giulia", 58, 5 * time.Hour, "Raggruppamento delle notifiche"}}},
	{ref: "shortcut", typ: i18n.DefaultTypeStory, reporter: "giulia", summary: "Keyboard shortcut to assign to me",
		priority: i18n.DefaultPriorityVeryLow, labels: []string{"ux", "keyboard"}},
	{ref: "hindi", reporter: "priya", assignee: "priya", summary: "हिंदी में तारीख़ का प्रारूप", status: issue.Doing,
		priority: i18n.DefaultPriorityMedium, labels: []string{"i18n"},
		comments: []note{{"priya", "अंक अभी अंग्रेज़ी में दिखते हैं।"}},
		worklogs: []worklog{{"priya", 1, time.Hour, "तारीख़ के प्रारूप की जाँच"}}},
	{ref: "overdue", typ: i18n.DefaultTypeBug, reporter: "priya", assignee: "tom",
		summary: "Due dates in the past are not highlighted", priority: i18n.DefaultPriorityMedium, due: day(-3),
		components: []string{"Frontend"}, custom: map[string]any{"Browsers": []string{"Chrome"}}},
	{ref: "checklist", reporter: "tom", assignee: "tom", summary: "Upgrade the release checklist", status: issue.Done,
		priority: i18n.DefaultPriorityLow, components: []string{"Documentation"}, fixVersions: []string{"1.0"},
		comments:    []note{{"tom", "Checklist updated in the wiki."}},
		attachments: []attachment{{"tom", "release-checklist.txt"}}},
	{ref: "slow", typ: i18n.DefaultTypeBug, epic: "search", reporter: "tom", assignee: "alice",
		summary: "Slow search on projects with many labels", priority: i18n.DefaultPriorityHigh, estimate: "1w",
		labels: []string{"search", "performance"}, components: []string{"Backend"},
		watchers: []string{"hans"}},
	{ref: "cookie", typ: i18n.DefaultTypeSubtask, parent: "remember", reporter: "alice", assignee: "alice",
		summary: "Store the username in a cookie", status: issue.Done, estimate: "3h",
		worklogs: []worklog{{"alice", 4, 3 * time.Hour, "Cookie written on sign-in"}}},
	{ref: "signout", typ: i18n.DefaultTypeSubtask, parent: "remember", reporter: "alice", assignee: "marie",
		summary: "Clear the cookie on sign-out", estimate: "1h"},
	{ref: "index", typ: i18n.DefaultTypeSubtask, parent: "slow", reporter: "alice", assignee: "hans",
		summary: "Add an index on the labels table", status: issue.Doing},
	{ref: "cookieleak", typ: i18n.DefaultTypeBug, reporter: "hans", assignee: "alice",
		summary: "Session cookie sent to other subdomains", level: Confidential, priority: i18n.DefaultPriorityHigh,
		labels: []string{"security"}, components: []string{"Backend"}, fixVersions: []string{"1.1"},
		comments: []note{{"alice", "Confirmed: the cookie has a Domain attribute, it should be host-only."}}},
	{ref: "twomails", typ: i18n.DefaultTypeBug, reporter: "tom", summary: "Two e-mails for one comment",
		status: issue.Done, resolution: i18n.DefaultResolutionDuplicate, priority: i18n.DefaultPriorityLow},
	{ref: "oldreset", reporter: "tom", assignee: "tom", summary: "Remove the old password reset page",
		status: issue.Done, resolution: i18n.DefaultResolutionWontDo, priority: i18n.DefaultPriorityVeryLow,
		comments: []note{{"alice", "Some customers still bookmark it, let's keep it."}}},

	// MOB: an epic with a story and its sub-task, a crash, a release.
	{ref: "offline", project: "MOB", typ: i18n.DefaultTypeEpic, reporter: "hans", assignee: "giulia",
		summary: "Offline mode", priority: i18n.DefaultPriorityHigh, fixVersions: []string{"2.1"},
		custom: map[string]any{"Platforms": []string{"iOS", "Android"}}},
	{ref: "sync", project: "MOB", typ: i18n.DefaultTypeStory, epic: "offline", reporter: "hans", assignee: "giulia",
		summary: "Sync issues when the network comes back", status: issue.Doing, priority: i18n.DefaultPriorityHigh,
		estimate: "3d", components: []string{"iOS", "Android"}, fixVersions: []string{"2.1"},
		custom:   map[string]any{"Story points": 13, "Platforms": []string{"iOS", "Android"}, "QA owner": "priya"},
		comments: []note{{"giulia", "La coda funziona, manca la gestione dei conflitti."}},
		worklogs: []worklog{{"giulia", 6, 4 * time.Hour, "Coda delle modifiche"},
			{"priya", 2, 2 * time.Hour, "Android test run"}},
		votes: []string{"alice"}},
	{ref: "queue", project: "MOB", typ: i18n.DefaultTypeSubtask, parent: "sync", reporter: "giulia", assignee: "priya",
		summary: "Queue edits made offline", estimate: "1d"},
	{ref: "crash", project: "MOB", typ: i18n.DefaultTypeBug, reporter: "marie", assignee: "priya",
		summary: "Crash au démarrage sur Android 14", status: issue.Done, priority: i18n.DefaultPriorityVeryHigh,
		components: []string{"Android"}, fixVersions: []string{"2.0"},
		custom: map[string]any{"Severity": "Blocker", "Platforms": []string{"Android"},
			"Steps to reproduce": "Ouvrir l'application sur un Pixel 8 sous Android 14."},
		comments:    []note{{"priya", "Fixed in 2.0.1, the splash screen read a missing setting."}},
		attachments: []attachment{{"marie", "crash-log.txt"}},
		votes:       []string{"hans", "tom"}},
	{ref: "stores", project: "MOB", reporter: "hans", assignee: "hans", summary: "Publish 2.1 to the app stores",
		due: day(45), fixVersions: []string{"2.1"}, priority: i18n.DefaultPriorityMedium},

	// OPS, a business project.
	{ref: "move", project: "OPS", typ: i18n.DefaultTypeEpic, reporter: "marie", assignee: "marie",
		summary: "Déménagement au deuxième étage", due: day(60), priority: i18n.DefaultPriorityMedium},
	{ref: "chairs", project: "OPS", epic: "move", reporter: "marie", assignee: "tom",
		summary: "Commander des chaises pour la salle 2", due: day(7), components: []string{"Purchasing"},
		custom:   map[string]any{"Customer": "Équipe produit"},
		comments: []note{{"tom", "Quote received, waiting for approval."}}},
	{ref: "projector", project: "OPS", reporter: "tom", assignee: "tom", summary: "Fix the meeting room projector",
		status: issue.Done, components: []string{"Facilities"},
		worklogs: []worklog{{"tom", 9, 45 * time.Minute, "New lamp"}}},
	{ref: "heating", project: "OPS", typ: i18n.DefaultTypeBug, reporter: "hans", assignee: "tom",
		summary: "Heizung im Büro 3 fällt aus", priority: i18n.DefaultPriorityVeryHigh, due: day(-2),
		components: []string{"Facilities"}, votes: []string{"marie", "giulia", "priya"}},
}

var links = []demoLink{
	{"hans", "umlaut", i18n.DefaultLinkRelatesName, "slow"},
	{"alice", "index", i18n.DefaultLinkBlocksName, "slow"},
	{"marie", "csv", i18n.DefaultLinkBlocksName, "checklist"},
	{"tom", "twomails", i18n.DefaultLinkDuplicatesName, "notify"},
	{"giulia", "sync", i18n.DefaultLinkRelatesName, "remember"},
	{"hans", "stores", i18n.DefaultLinkClonesName, "checklist"},
	{"priya", "crash", i18n.DefaultLinkBlocksName, "stores"},
	{"alice", "overflow", i18n.DefaultLinkRelatesName, "crash"},
}

// catalogue maps the seeded catalogue entries to their ids.
type catalogue struct{ types, priorities, resolutions, linkTypes map[i18n.Key]string }

func readCatalogue(admin *client) (*catalogue, error) {
	type entry struct {
		ID   json.Number
		Seed i18n.Key
	}
	var meta struct{ IssueTypes, Priorities, Resolutions, LinkTypes []entry }
	if err := admin.call("GET", "/api/meta", nil, &meta); err != nil {
		return nil, err
	}
	index := func(list []entry) map[i18n.Key]string {
		m := map[i18n.Key]string{}
		for _, e := range list {
			if e.Seed != "" {
				m[e.Seed] = e.ID.String()
			}
		}
		return m
	}
	return &catalogue{index(meta.IssueTypes), index(meta.Priorities), index(meta.Resolutions), index(meta.LinkTypes)}, nil
}

// lookup finds a catalogue entry by its seed key.
func lookup(m map[i18n.Key]string, key i18n.Key) (string, error) {
	if id, ok := m[key]; ok {
		return id, nil
	}
	return "", fmt.Errorf("the catalogue has no %s", key)
}

// seeded is a demo issue as stored.
type seeded struct {
	id  int64
	key string
}

// counts are what a run added around the issues, by kind.
type counts map[string]int

func (c counts) say(say func(string, ...any)) {
	var parts []string
	for _, what := range []string{"comment", "work log", "attachment", "link", "vote", "watcher"} {
		switch n := c[what]; {
		case n == 1:
			parts = append(parts, "1 "+what)
		case n > 1:
			parts = append(parts, fmt.Sprintf("%d %ss", n, what))
		}
	}
	if len(parts) > 0 {
		say("added %s to the demo issues", strings.Join(parts, ", "))
	}
}

// ensureIssues creates the custom fields and the demo issues that are
// missing (by project and summary), each issue by its reporter, then what is
// missing around them and the links between them, each by the user it names.
func ensureIssues(ctx context.Context, conn *sql.DB, base string, admin *client, ids map[string]int64,
	levels map[[2]string]int64, say func(string, ...any)) error {
	cat, err := readCatalogue(admin)
	if err != nil {
		return err
	}
	fieldIDs, err := ensureFields(ctx, conn, admin, say)
	if err != nil {
		return fmt.Errorf("custom fields: %w", err)
	}
	clients := map[string]*client{}
	as := func(username string) (*client, error) {
		if c, ok := clients[username]; ok {
			return c, nil
		}
		c, err := login(ctx, base, username, UserPassword)
		clients[username] = c
		return c, err
	}
	parts := map[string]*projectParts{}
	byRef := map[string]seeded{}
	created := map[string]int{}
	added := counts{}
	for _, is := range issues {
		project := is.project
		if project == "" {
			project = Project
		}
		s, err := findIssue(ctx, conn, project, is.summary)
		if errors.Is(err, sql.ErrNoRows) {
			if parts[project] == nil {
				if parts[project], err = readParts(admin, project); err != nil {
					return fmt.Errorf("project %s: %w", project, err)
				}
			}
			s, err = createIssue(ctx, conn, as, cat, parts[project], fieldIDs, ids, levels, byRef, project, is)
			if err == nil {
				created[project]++
			}
		}
		if err != nil {
			return fmt.Errorf("%q: %w", is.summary, err)
		}
		byRef[is.ref] = s
		if err := ensureAround(ctx, conn, as, ids, s, is, added); err != nil {
			return fmt.Errorf("%s: %w", s.key, err)
		}
	}
	for _, l := range links {
		if err := ensureLink(ctx, conn, as, cat, byRef, l, added); err != nil {
			return fmt.Errorf("link %s %s %s: %w", l.from, l.typ, l.to, err)
		}
	}
	for _, p := range projects {
		if n := created[p.key]; n > 0 {
			say("created %d issues in the project %s", n, p.key)
		}
	}
	added.say(say)
	return nil
}

func findIssue(ctx context.Context, conn *sql.DB, project, summary string) (seeded, error) {
	var s seeded
	var num int64
	err := conn.QueryRowContext(ctx, `SELECT i.id, i.num FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE p.pkey = ? AND i.summary = ?`, project, summary).Scan(&s.id, &num)
	s.key = project + "-" + strconv.FormatInt(num, 10)
	return s, err
}

// projectParts are a project's components and versions by name.
type projectParts struct{ components, versions map[string]int64 }

func readParts(admin *client, key string) (*projectParts, error) {
	type part struct {
		ID   int64
		Name string
	}
	var p struct{ Components, Versions []part }
	if err := admin.call("GET", "/api/projects/"+key, nil, &p); err != nil {
		return nil, err
	}
	out := &projectParts{map[string]int64{}, map[string]int64{}}
	for _, c := range p.Components {
		out.components[c.Name] = c.ID
	}
	for _, v := range p.Versions {
		out.versions[v.Name] = v.ID
	}
	return out, nil
}

func createIssue(ctx context.Context, conn *sql.DB, as func(string) (*client, error), cat *catalogue,
	parts *projectParts, fieldIDs, ids map[string]int64, levels map[[2]string]int64, byRef map[string]seeded, project string,
	is demoIssue) (seeded, error) {
	typ := is.typ
	if typ == "" {
		typ = i18n.DefaultTypeTask
	}
	typeID, err := lookup(cat.types, typ)
	if err != nil {
		return seeded{}, err
	}
	in := map[string]any{"project": project, "type": typeID, "summary": is.summary}
	if is.assignee != "" {
		in["assignee"] = ids[is.assignee]
	}
	if is.priority != "" {
		if in["priority"], err = lookup(cat.priorities, is.priority); err != nil {
			return seeded{}, err
		}
	}
	for field, ref := range map[string]string{"parent": is.parent, "epic": is.epic} {
		if ref == "" {
			continue
		}
		s, ok := byRef[ref]
		if !ok {
			return seeded{}, fmt.Errorf("its %s %s comes later in the list", field, ref)
		}
		in[field] = s.key
	}
	if len(is.labels) > 0 {
		in["labels"] = is.labels
	}
	if is.due != "" {
		in["due"] = is.due
	}
	if is.level != "" {
		id, ok := levels[[2]string{project, is.level}]
		if !ok {
			return seeded{}, fmt.Errorf("the project has no security level %s", is.level)
		}
		in["securityLevel"] = id
	}
	if is.estimate != "" {
		in["originalEstimate"] = is.estimate
		in["remainingEstimate"] = is.estimate
	}
	for _, set := range []struct {
		key   string
		names []string
		ids   map[string]int64
	}{{"components", is.components, parts.components}, {"fixVersions", is.fixVersions, parts.versions}} {
		if len(set.names) == 0 {
			continue
		}
		var list []int64
		for _, n := range set.names {
			id, ok := set.ids[n]
			if !ok {
				return seeded{}, fmt.Errorf("the project has no %s %s", set.key, n)
			}
			list = append(list, id)
		}
		in[set.key] = list
	}
	if len(is.custom) > 0 {
		values := map[string]any{}
		for name, v := range is.custom {
			id, ok := fieldIDs[name]
			if !ok {
				return seeded{}, fmt.Errorf("no custom field %s", name)
			}
			values[strconv.FormatInt(id, 10)] = v
		}
		in["customFields"] = values
	}
	reporter, err := as(is.reporter)
	if err != nil {
		return seeded{}, err
	}
	if err := reporter.call("POST", "/api/issues", in, nil); err != nil {
		return seeded{}, err
	}
	s, err := findIssue(ctx, conn, project, is.summary)
	if err != nil {
		return seeded{}, err
	}
	if is.status != "" {
		var resolution string
		if is.resolution != "" {
			if resolution, err = lookup(cat.resolutions, is.resolution); err != nil {
				return seeded{}, err
			}
		}
		if err := moveTo(reporter, s.key, is.status, resolution); err != nil {
			return seeded{}, err
		}
	}
	return s, nil
}

// ensureAround adds the comments, work logs, attachments, votes and watchers
// a demo issue lacks, counting them in added.
func ensureAround(ctx context.Context, conn *sql.DB, as func(string) (*client, error), ids map[string]int64,
	s seeded, is demoIssue, added counts) error {
	type step struct {
		what, user string
		exists     string // a query on the issue, the user and args
		args       []any
		do         func(*client) error
	}
	var steps []step
	path := "/api/issues/" + s.key
	var comments []groupNote
	for _, n := range is.comments {
		comments = append(comments, groupNote{"", n})
	}
	comments = append(comments, is.groupComments...)
	for _, n := range comments {
		steps = append(steps, step{"comment", n.author,
			`SELECT 1 FROM comments WHERE issue_id = ? AND author_id = ? AND body = ?`, []any{n.body},
			func(c *client) error {
				in := map[string]any{"body": n.body}
				if n.group != "" {
					in["visibility"] = map[string]any{"group": n.group}
				}
				return c.call("POST", path+"/comments", in, nil)
			}})
	}
	var worklogs []groupWorklog
	for _, w := range is.worklogs {
		worklogs = append(worklogs, groupWorklog{"", w})
	}
	worklogs = append(worklogs, is.groupWorklogs...)
	for _, w := range worklogs {
		steps = append(steps, step{"work log", w.author,
			`SELECT 1 FROM worklogs WHERE issue_id = ? AND author_id = ? AND body = ?`, []any{w.body},
			func(c *client) error {
				var out struct{ ID int64 }
				if err := c.call("POST", path+"/worklogs", map[string]any{
					"started": time.Now().AddDate(0, 0, -w.daysAgo).Truncate(time.Hour),
					"seconds": int64(w.spent / time.Second), "body": w.body}, &out); err != nil || w.group == "" {
					return err
				}
				// The API offers no restricted work logs: only a Jira import
				// makes them, and stores them this way.
				_, err := conn.ExecContext(ctx, `UPDATE worklogs SET visibility_group = ? WHERE id = ?`, w.group, out.ID)
				return err
			}})
	}
	for _, a := range is.attachments {
		steps = append(steps, step{"attachment", a.author,
			`SELECT 1 FROM attachments WHERE issue_id = ? AND author_id = ? AND filename = ?`, []any{a.name},
			func(c *client) error {
				mimeType, data, err := attachmentFile(a.name)
				if err != nil {
					return err
				}
				return c.upload(path+"/attachments", a.name, mimeType, data)
			}})
	}
	for _, u := range is.votes {
		steps = append(steps, step{"vote", u, `SELECT 1 FROM votes WHERE issue_id = ? AND user_id = ?`, nil,
			func(c *client) error { return c.call("PUT", path+"/vote", nil, nil) }})
	}
	for _, u := range is.watchers {
		steps = append(steps, step{"watcher", u, `SELECT 1 FROM watchers WHERE issue_id = ? AND user_id = ?`, nil,
			func(c *client) error { return c.call("PUT", path+"/watch", nil, nil) }})
	}
	for _, st := range steps {
		var found int
		err := conn.QueryRowContext(ctx, st.exists, append([]any{s.id, ids[st.user]}, st.args...)...).Scan(&found)
		if err == nil {
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		c, err := as(st.user)
		if err != nil {
			return err
		}
		if err := st.do(c); err != nil {
			return err
		}
		added[st.what]++
	}
	return nil
}

func ensureLink(ctx context.Context, conn *sql.DB, as func(string) (*client, error), cat *catalogue,
	byRef map[string]seeded, l demoLink, added counts) error {
	from, ok := byRef[l.from]
	to, ok2 := byRef[l.to]
	if !ok || !ok2 {
		return errors.New("no such demo issue")
	}
	typeID, err := lookup(cat.linkTypes, l.typ)
	if err != nil {
		return err
	}
	var n int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM issue_links WHERE type_id = ? AND source_id = ? AND target_id = ?`,
		typeID, from.id, to.id).Scan(&n); err != nil || n > 0 {
		return err
	}
	c, err := as(l.by)
	if err != nil {
		return err
	}
	if err := c.call("POST", "/api/issues/"+from.key+"/links",
		map[string]any{"type": json.Number(typeID), "direction": "outward", "target": to.key}, nil); err != nil {
		return err
	}
	added["link"]++
	return nil
}

// moveTo moves an issue to the first status of a category, with a
// resolution ("" for the one the status implies).
func moveTo(c *client, key string, category issue.StatusCategory, resolution string) error {
	var d struct {
		Statuses []struct{ ID, Category string }
	}
	if err := c.call("GET", "/api/issues/"+key, nil, &d); err != nil {
		return err
	}
	for _, s := range d.Statuses {
		if issue.StatusCategory(s.Category) == category {
			in := map[string]any{"status": s.ID}
			if resolution != "" {
				in["resolution"] = resolution
			}
			return c.call("PATCH", "/api/issues/"+key, in, nil)
		}
	}
	return fmt.Errorf("no %s status", category)
}
