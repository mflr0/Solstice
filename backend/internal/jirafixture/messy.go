package jirafixture

import (
	"fmt"
	"hash/fnv"
	"math/rand/v2"
	"slices"
	"strings"
)

// The quirks of old Jira instances, generated with Options.Messy. They come
// after everything else: each one adds rows to the finished data, or rewrites
// a few rows, so without the option nothing changes. Each quirk draws from a
// random source of its own, derived from the seed and its name: the same seed
// gives the same quirks, and a quirk can change without moving the others.

// Custom fields whose type changed.
const (
	cfLegacyNumber = 10114 // was a text field, is a number field now
	cfLegacyText   = 10115 // was a date field, is a text field now
)

// missingID is the id of rows that point to something Jira no longer has.
// Ids of the generated rows stay far below.
const missingID int64 = 9_000_000

// MessySummary counts what the quirks added, for the importer checks. What
// the importer is expected to copy is also counted in the main Summary
// fields; what it cannot import is counted here only.
type MessySummary struct {
	GhostUsers   int `json:"ghost_users"`  // user keys without any user row, used by rows the importer copies
	GhostIssues  int `json:"ghost_issues"` // issues reported or assigned by one of them
	GhostWatches int `json:"ghost_watches"`

	RenamedKeys   int `json:"renamed_keys"`   // moved_issue_key rows under a former project key
	MovedIssues   int `json:"moved_issues"`   // issues moved to another project
	DuplicateKeys int `json:"duplicate_keys"` // an old issue key given to a second issue

	MisfitFieldValues int `json:"misfit_field_values"` // values in the column of the field's former type
	OddTexts          int `json:"odd_texts"`

	OrphanChangeItems    int `json:"orphan_change_items"`
	OrphanWorklogs       int `json:"orphan_worklogs"`
	OrphanComments       int `json:"orphan_comments"`
	OrphanIssueLinks     int `json:"orphan_issue_links"`
	OrphanComponentLinks int `json:"orphan_component_links"`
	OrphanMovedKeys      int `json:"orphan_moved_keys"`
}

func (g *generator) seedMessy() {
	g.sum.Messy = &MessySummary{}
	for _, q := range []struct {
		name string
		fn   func()
	}{
		{"deleted-users", g.messyDeletedUsers},
		{"renamed-project", g.messyRenamedProject},
		{"moved-issues", g.messyMovedIssues},
		{"field-type-change", g.messyFieldTypeChange},
		{"odd-text", g.messyOddText},
		{"orphans", g.messyOrphans},
	} {
		g.quirk(q.name, q.fn)
	}
}

// quirk runs fn with the generator's random source replaced by the quirk's own.
func (g *generator) quirk(name string, fn func()) {
	h := fnv.New64a()
	h.Write([]byte(name))
	saved := g.rng
	g.rng = rand.New(rand.NewPCG(g.opt.Seed, h.Sum64()))
	defer func() { g.rng = saved }()
	fn()
}

func anyIssue(*issue) bool { return true }

// messyIssues draws up to n distinct issues for which ok is true. The
// checkpoints are never drawn: the importer tests know them by their key.
func (g *generator) messyIssues(n int, ok func(*issue) bool) []*issue {
	checkpoint := map[string]bool{}
	for _, c := range g.sum.Checkpoints {
		checkpoint[c.Key] = true
	}
	var candidates []*issue
	for _, is := range g.issues {
		if !checkpoint[is.key()] && ok(is) {
			candidates = append(candidates, is)
		}
	}
	g.rng.Shuffle(len(candidates), func(a, b int) { candidates[a], candidates[b] = candidates[b], candidates[a] })
	return candidates[:min(n, len(candidates))]
}

// --- 1. deleted users -----------------------------------------------------------

// Users deleted so long ago that not even an app_user row is left: the rows
// that name them hold a user name (Jira before 6) or a key nobody knows.
var ghostUsers = []string{"j.dupond", "ex.stagiaire", "RLEBLANC"}

// ghostWatcher only watches and votes. The importer gives an inactive account
// to the ghosts that issues, comments or history name, and then copies their
// watches and votes; this one is named by nothing else, so it has no account
// and its rows cannot be imported.
const ghostWatcher = "ancien.observateur"

var ghostComments = []string{
	"Je reprends le dossier, l'ancien responsable n'est plus là.",
	"Validé de mon côté, voir la dernière version déployée.",
	"Même problème chez un autre client, à regrouper.",
}

// messyDeletedUsers makes deleted users appear as reporter or assignee of
// issues, and as author of comments, work logs and history; another one
// only watches and votes.
func (g *generator) messyDeletedUsers() {
	m := g.sum.Messy
	for i, is := range g.messyIssues(max(3, len(g.issues)/25), anyIssue) {
		ghost := ghostUsers[i%len(ghostUsers)]
		mode := g.rng.IntN(3) // 0 reporter, 1 assignee, 2 both
		if i < len(ghostUsers) {
			mode = 0 // every deleted user reports at least one issue
		}
		var set []any
		if mode != 1 {
			set = append(set, "reporter", ghost, "creator", ghost)
		}
		if mode != 0 {
			set = append(set, "assignee", pick(g, ghostUsers))
		}
		g.w.Update("jiraissue", is.id, set...)
		m.GhostIssues++
	}
	m.GhostUsers = min(len(ghostUsers), m.GhostIssues)

	comment := g.w.Table("jiraaction", "id", "issueid", "author", "actiontype", "actionlevel", "rolelevel",
		"actionbody", "created", "updateauthor", "updated", "actionnum")
	worklog := g.w.Table("worklog", "id", "issueid", "author", "grouplevel", "rolelevel", "worklogbody",
		"created", "updateauthor", "updated", "startdate", "timeworked")
	group := g.w.Table("changegroup", "id", "issueid", "author", "created")
	item := g.w.Table("changeitem", "id", "groupid", "fieldtype", "field", "oldvalue", "oldstring",
		"newvalue", "newstring")
	targets := g.messyIssues(3*len(ghostUsers), func(is *issue) bool { return is.typ != typeEpic })
	for i, is := range targets {
		ghost := ghostUsers[i%len(ghostUsers)]
		at := g.after(is.created, 30)
		switch i / len(ghostUsers) {
		case 0:
			comment.Add(g.next("jiraaction"), is.id, ghost, "comment", nil, nil, pick(g, ghostComments), at,
				pick(g, ghostUsers), at, nil)
			g.sum.Comments++
		case 1:
			worklog.Add(g.next("worklog"), is.id, ghost, nil, nil, "Reprise de l'intervention", at, ghost, at, at,
				int64(3600))
			g.sum.Worklogs++
		default:
			gid := g.next("changegroup")
			group.Add(gid, is.id, ghost, at)
			item.Add(g.next("changeitem"), gid, "jira", "assignee", nil, nil, ghost, ghost)
			g.sum.ChangeGroups++
			g.sum.ChangeItems++
		}
	}
	// A deleted user's watches and votes cannot be imported.
	assoc := g.w.Table("userassociation", "source_name", "sink_node_id", "sink_node_entity", "association_type",
		"sequence", "created")
	if len(targets) > 0 {
		at := g.after(targets[0].created, 5)
		assoc.Add(ghostWatcher, targets[0].id, "Issue", "WatchIssue", nil, at)
		assoc.Add(ghostWatcher, targets[0].id, "Issue", "VoteIssue", nil, at)
		m.GhostWatches = 2
	}
}

// --- 2. a renamed project --------------------------------------------------------

// formerProjectKey is the key WEB had before SITE.
const formerProjectKey = "PORTAL"

// messyRenamedProject gives WEB, renamed from SITE (see seedProjects), what a
// rename leaves behind: an older key in project_key, the old keys of its
// issues in moved_issue_key and the rename in their history; other issues
// are linked to WEB issues through the former key.
func (g *generator) messyRenamedProject() {
	m := g.sum.Messy
	web := g.byKey["WEB"]
	g.w.Table("project_key", "id", "project_id", "project_key").Add(g.next("project_key"), web.id, formerProjectKey)
	moved := g.w.Table("moved_issue_key", "id", "old_issue_key", "issue_id")
	group := g.w.Table("changegroup", "id", "issueid", "author", "created")
	item := g.w.Table("changeitem", "id", "groupid", "fieldtype", "field", "oldvalue", "oldstring",
		"newvalue", "newstring")
	admin := g.byName["admin"].key

	renamed := func(is *issue, oldKey, newKey string) {
		moved.Add(g.next("moved_issue_key"), oldKey, is.id)
		gid := g.next("changegroup")
		group.Add(gid, is.id, admin, g.after(is.created, 20))
		item.Add(g.next("changeitem"), gid, "jira", "Key", nil, oldKey, nil, newKey)
		g.sum.MovedIssueKeys++
		g.sum.ChangeGroups++
		g.sum.ChangeItems++
		m.RenamedKeys++
	}
	var onWeb []*issue
	for _, is := range g.issues {
		if is.proj == web {
			onWeb = append(onWeb, is)
		}
	}
	renamedIssues := g.messyIssues(len(onWeb)/3, func(is *issue) bool { return is.proj == web })
	for i, is := range renamedIssues {
		num := fmt.Sprint(is.num)
		if i%3 == 0 {
			renamed(is, formerProjectKey+"-"+num, web.origKey+"-"+num)
		}
		renamed(is, web.origKey+"-"+num, is.key())
	}

	// Links made while the project had its former key.
	for i, is := range g.messyIssues(3, func(is *issue) bool { return is.proj != web }) {
		if i >= len(renamedIssues) {
			break
		}
		old := fmt.Sprintf("%s-%d", web.origKey, renamedIssues[i].num)
		gid := g.next("changegroup")
		group.Add(gid, is.id, admin, g.after(is.created, 20))
		item.Add(g.next("changeitem"), gid, "jira", "Link", nil, nil, old, "This issue relates to "+old)
		g.sum.ChangeGroups++
		g.sum.ChangeItems++
	}
}

// --- 3. issues moved between projects --------------------------------------------

// messyMovedIssues moves a few issues of WEB and OPS to DEMO: new project and
// number, the old key in moved_issue_key, the move in the history. One of the
// old keys was later given to another issue.
func (g *generator) messyMovedIssues() {
	m := g.sum.Messy
	demo := g.byKey["DEMO"]
	// Without files (they stay under the old key on disk), sub-tasks or epic.
	plain := func(is *issue, project string, types ...string) bool {
		return is.proj.key == project && is.movedFrom == "" && is.parent == nil && is.subtasks == 0 &&
			is.epic == nil && len(is.attached) == 0 && (len(types) == 0 || slices.Contains(types, is.typ))
	}
	moving := g.messyIssues(2, func(is *issue) bool { return plain(is, "WEB", typeStory, typeBug, typeTask) })
	moving = append(moving, g.messyIssues(2, func(is *issue) bool { return plain(is, "OPS", typeTask, typeBug) })...)

	moved := g.w.Table("moved_issue_key", "id", "old_issue_key", "issue_id")
	group := g.w.Table("changegroup", "id", "issueid", "author", "created")
	item := g.w.Table("changeitem", "id", "groupid", "fieldtype", "field", "oldvalue", "oldstring",
		"newvalue", "newstring")
	admin := g.byName["admin"].key
	for _, is := range moving {
		oldKey, from := is.key(), is.proj
		demo.count++
		is.proj, is.num, is.movedFrom = demo, demo.count, oldKey
		g.w.Update("jiraissue", is.id, "project", demo.id, "issuenum", is.num)
		moved.Add(g.next("moved_issue_key"), oldKey, is.id)
		gid := g.next("changegroup")
		group.Add(gid, is.id, admin, g.after(is.created, 10))
		item.Add(g.next("changeitem"), gid, "jira", "Key", nil, oldKey, nil, is.key())
		item.Add(g.next("changeitem"), gid, "jira", "project", fmt.Sprint(from.id), from.name, fmt.Sprint(demo.id), demo.name)
		g.sum.MovedIssueKeys++
		g.sum.ChangeGroups++
		g.sum.ChangeItems += 2
		m.MovedIssues++
	}
	if len(moving) > 0 {
		g.w.Update("project", demo.id, "pcounter", demo.count)
		for _, other := range g.messyIssues(1, func(is *issue) bool { return is.movedFrom == "" }) {
			moved.Add(g.next("moved_issue_key"), moving[0].movedFrom, other.id)
			m.DuplicateKeys++
		}
	}
}

// --- 4. custom fields whose type changed -----------------------------------------

// messyFieldTypeChange adds two fields whose type changed after values were
// entered. Jira leaves the old values where they were, in the column of the
// former type, which the current type never reads.
func (g *generator) messyFieldTypeChange() {
	m := g.sum.Messy
	cf := g.w.Table("customfield", "id", "cfkey", "customfieldtypekey", "customfieldsearcherkey", "cfname",
		"description", "defaultvalue", "fieldtype", "project", "issuetype", "issueswithvalue", "lastvalueupdate")
	fcs := g.w.Table("fieldconfigscheme", "id", "configname", "description", "fieldid", "customfield")
	fc := g.w.Table("fieldconfiguration", "id", "configname", "description", "fieldid", "customfield")
	fcsit := g.w.Table("fieldconfigschemeissuetype", "id", "issuetype", "fieldconfigscheme", "fieldconfiguration")
	ctx := g.w.Table("configurationcontext", "id", "projectcategory", "project", "customfield", "fieldconfigscheme")
	for _, d := range []customFieldDef{
		{cfLegacyNumber, "Priorité client", cfPrefix + "float", cfPrefix + "exactnumber"},
		{cfLegacyText, "Référence de recette", cfPrefix + "textfield", cfPrefix + "textsearcher"},
	} {
		cf.Add(d.id, nil, d.typ, d.search, d.name, "Champ « "+d.name+" »", nil, nil, nil, nil, nil, nil)
		g.sum.CustomFields++
		fieldID := fmt.Sprintf("customfield_%d", d.id)
		cfg := d.id + 100
		fcs.Add(cfg, "Default Configuration Scheme for "+d.name, nil, fieldID, d.id)
		fc.Add(cfg, "Default Configuration for "+d.name, nil, fieldID, d.id)
		fcsit.Add(g.next("fieldconfigschemeissuetype"), nil, cfg, cfg)
		ctx.Add(g.next("configurationcontext"), nil, nil, fieldID, cfg)
	}

	values := g.w.Table("customfieldvalue", "id", "issue", "customfield", "updated", "parentkey", "stringvalue",
		"numbervalue", "textvalue", "datevalue", "valuetype")
	for i, is := range g.messyIssues(24, func(is *issue) bool { return is.proj.software }) {
		id := g.next("customfieldvalue")
		switch i % 4 {
		case 0: // number entered since the change
			values.Add(id, is.id, int64(cfLegacyNumber), nil, nil, nil, float64(1+g.rng.IntN(5)), nil, nil, nil)
		case 1: // text entered before it
			values.Add(id, is.id, int64(cfLegacyNumber), nil, nil, fmt.Sprintf("CLI-%04d", g.rng.IntN(10000)), nil,
				nil, nil, nil)
			m.MisfitFieldValues++
			continue
		case 2: // text entered since the change
			values.Add(id, is.id, int64(cfLegacyText), nil, nil, fmt.Sprintf("REC-%03d", g.rng.IntN(1000)), nil, nil,
				nil, nil)
		default: // date entered before it
			values.Add(id, is.id, int64(cfLegacyText), nil, nil, nil, nil, nil,
				g.midnight(is.created).AddDate(0, 0, 10+g.rng.IntN(60)), nil)
			m.MisfitFieldValues++
			continue
		}
		g.sum.CustomFieldValues++
	}
}

// --- 5. odd text -------------------------------------------------------------------

// mojibake reads the UTF-8 bytes of s as Latin-1, as a tool does that does
// not know the encoding of what it reads ("é" becomes "Ã©"). Bytes that are not
// UTF-8 cannot be stored in a UTF-8 column of either database, so this is the
// form mis-encoded Latin-1 takes in the data.
func mojibake(s string) string {
	runes := make([]rune, len(s))
	for i := range len(s) {
		runes[i] = rune(s[i])
	}
	return string(runes)
}

// oddTexts are texts that cause trouble: mixed line endings, control
// characters other than NUL (which the databases refuse), lines that are
// longer than any sensible limit, emoji, right-to-left text and mojibake.
func oddTexts() []string {
	return []string{
		"Première ligne\r\nDeuxième ligne\rTroisième ligne\nQuatrième ligne\r\n\r\nFin\r",
		"Alerte\x01 cloche\x07 tabulation\there tab verticale\x0b saut de page\x0c échappement\x1b[31m rouge\x1b[0m suppression\x7f fin",
		strings.Repeat("très longue ligne sans retour ", 4000), // 120 000 characters
		strings.Repeat("x", 70_000), // more than a TEXT column holds
		"Déploiement terminé 🎉🚀 👨‍👩‍👧‍👦 🇫🇷 👍🏽 — 𝔘𝔫𝔦𝔠𝔬𝔡𝔢",
		"Bonjour مرحبا بالعالم et שלום עולם \u202bטקסט\u202c \u200fסוף\u200e",
		mojibake("Réunion à Zürich : le café a été livré — « merci »"),
	}
}

// oddSummaries are short enough for a summary.
func oddSummaries() []string {
	return []string{
		"Déploiement 🎉 terminé — vérifier בדיקה",
		mojibake("Réunion à Zürich — « café »"),
		"Alerte\x07 disque\x1b[0m plein",
	}
}

// messyOddText adds one comment per odd text, rewrites a few descriptions
// and summaries with them and adds a work log.
func (g *generator) messyOddText() {
	m := g.sum.Messy
	texts, summaries := oddTexts(), oddSummaries()
	targets := g.messyIssues(len(texts)+3+len(summaries)+1, anyIssue)
	next := func() *issue {
		if len(targets) == 0 {
			return nil
		}
		is := targets[0]
		targets = targets[1:]
		return is
	}
	comment := g.w.Table("jiraaction", "id", "issueid", "author", "actiontype", "actionlevel", "rolelevel",
		"actionbody", "created", "updateauthor", "updated", "actionnum")
	for _, text := range texts {
		if is := next(); is != nil {
			author, at := pick(g, g.devs).key, g.after(is.created, 30)
			comment.Add(g.next("jiraaction"), is.id, author, "comment", nil, nil, text, at, author, at, nil)
			g.sum.Comments++
			m.OddTexts++
		}
	}
	for _, text := range []string{texts[0], texts[1], texts[2]} {
		if is := next(); is != nil {
			g.w.Update("jiraissue", is.id, "description", text)
			m.OddTexts++
		}
	}
	for _, text := range summaries {
		if is := next(); is != nil {
			g.w.Update("jiraissue", is.id, "summary", text)
			m.OddTexts++
		}
	}
	if is := next(); is != nil {
		author, at := pick(g, g.devs).key, g.after(is.created, 30)
		g.w.Table("worklog", "id", "issueid", "author", "grouplevel", "rolelevel", "worklogbody", "created",
			"updateauthor", "updated", "startdate", "timeworked").
			Add(g.next("worklog"), is.id, author, nil, nil, texts[4], at, author, at, at, int64(1800))
		g.sum.Worklogs++
		m.OddTexts++
	}
}

// --- 6. orphan rows ----------------------------------------------------------------

// messyOrphans adds rows that point to something that was deleted without
// them: change items of a missing change group, a work log, a comment and a
// link of a missing issue, a component link to a missing component, an old
// issue key of a missing issue.
func (g *generator) messyOrphans() {
	m := g.sum.Messy
	admin := g.byName["admin"].key
	at := g.at(10, dayStart)

	item := g.w.Table("changeitem", "id", "groupid", "fieldtype", "field", "oldvalue", "oldstring",
		"newvalue", "newstring")
	for i := range 3 {
		item.Add(g.next("changeitem"), missingID+int64(i), "jira", "status", "1", "Open", "3", "In Progress")
		m.OrphanChangeItems++
	}
	worklog := g.w.Table("worklog", "id", "issueid", "author", "grouplevel", "rolelevel", "worklogbody", "created",
		"updateauthor", "updated", "startdate", "timeworked")
	for i := range 2 {
		worklog.Add(g.next("worklog"), missingID+int64(i), admin, nil, nil, "Temps passé sur un ticket supprimé", at,
			admin, at, at, int64(1800))
		m.OrphanWorklogs++
	}
	g.w.Table("jiraaction", "id", "issueid", "author", "actiontype", "actionlevel", "rolelevel", "actionbody",
		"created", "updateauthor", "updated", "actionnum").
		Add(g.next("jiraaction"), missingID, admin, "comment", nil, nil, "Commentaire d'un ticket supprimé", at, admin,
			at, nil)
	m.OrphanComments++
	g.w.Table("moved_issue_key", "id", "old_issue_key", "issue_id").Add(g.next("moved_issue_key"), "OPS-404", missingID)
	m.OrphanMovedKeys++

	links := g.w.Table("issuelink", "id", "linktype", "source", "destination", "sequence")
	for _, is := range g.messyIssues(2, anyIssue) {
		links.Add(g.next("issuelink"), linkTypes[3].id, is.id, missingID+1, nil)
		m.OrphanIssueLinks++
		g.assoc(is.id, "Issue", missingID+int64(m.OrphanComponentLinks), "Component", "IssueComponent")
		m.OrphanComponentLinks++
	}
}
