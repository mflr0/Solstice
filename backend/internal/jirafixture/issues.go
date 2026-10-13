package jirafixture

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
)

type issue struct {
	id        int64
	proj      *project
	num       int64
	typ       string
	created   time.Time
	parent    *issue
	subtasks  int
	movedFrom string // original key of an issue moved to another project
	epic      *issue
	links     []*link // outward links, recorded in this issue's history

	// Filled by detail.
	summary    string
	status     string
	resolution string
	priority   string
	reporter   *user
	assignee   *user
	due        *time.Time
	comments   int
	attached   []string // attachment file names
}

func (is *issue) key() string { return fmt.Sprintf("%s-%d", is.proj.key, is.num) }

type link struct {
	id       int64
	typ      linkType
	src, dst *issue
	at       time.Time
}

type changeItem struct {
	fieldType, field                         string
	oldValue, oldString, newValue, newString any
}

type change struct {
	at     time.Time
	author *user
	items  []changeItem
}

// issueTables holds the batches of every per-issue table.
type issueTables struct {
	issue, wfEntry, curStep, label, comment, worklog, attachment, userAssoc,
	cfValue, changeGroup, changeItem, moved, issueLink, remoteLink *Table
}

func (g *generator) seedIssues() {
	n := g.opt.Issues
	created := make([]time.Time, n)
	for i := range created {
		created[i] = g.at(g.rng.IntN(400), dayStart+g.rng.IntN(dayEnd-dayStart))
	}
	sort.Slice(created, func(a, b int) bool { return created[a].Before(created[b]) })

	// DEMO 40 %, WEB 30 %, OPS 20 %, RH 10 %, in a shuffled order.
	keys := []string{"DEMO", "WEB", "OPS", "RH"}
	counts := []int{n * 40 / 100, n * 30 / 100, n * 20 / 100}
	counts = append(counts, n-counts[0]-counts[1]-counts[2])
	var order []*project
	for i, k := range keys {
		for range counts[i] {
			order = append(order, g.byKey[k])
		}
	}
	g.rng.Shuffle(len(order), func(a, b int) { order[a], order[b] = order[b], order[a] })

	byProject := map[string][]*issue{}
	var moved *issue
	for i := range n {
		p := order[i]
		is := &issue{id: g.next("jiraissue"), proj: p, created: created[i]}
		p.count++
		is.num = p.count
		g.chooseType(is, byProject[p.key])
		if p.key == "OPS" && is.num == 5 {
			is.typ, is.parent, is.movedFrom = typeTask, nil, "OPS-5"
			moved = is
		}
		if is.parent != nil {
			is.parent.subtasks++
		}
		byProject[p.key] = append(byProject[p.key], is)
		g.issues = append(g.issues, is)
	}
	if moved != nil {
		demo := g.byKey["DEMO"]
		demo.count++
		moved.proj, moved.num = demo, demo.count
		g.sum.MovedIssueKeys++
	}

	t := issueTables{
		issue: g.w.Table("jiraissue", "id", "pkey", "issuenum", "project", "reporter", "assignee", "creator",
			"issuetype", "summary", "description", "environment", "priority", "resolution", "issuestatus",
			"created", "updated", "duedate", "resolutiondate", "votes", "watches", "timeoriginalestimate",
			"timeestimate", "timespent", "workflow_id", "security", "fixfor", "component", "archived"),
		wfEntry: g.w.Table("OS_WFENTRY", "ID", "NAME", "INITIALIZED", "STATE"),
		curStep: g.w.Table("OS_CURRENTSTEP", "ID", "ENTRY_ID", "STEP_ID", "ACTION_ID", "OWNER", "START_DATE",
			"DUE_DATE", "FINISH_DATE", "STATUS", "CALLER"),
		label: g.w.Table("label", "id", "fieldid", "issue", "label"),
		comment: g.w.Table("jiraaction", "id", "issueid", "author", "actiontype", "actionlevel", "rolelevel",
			"actionbody", "created", "updateauthor", "updated", "actionnum"),
		worklog: g.w.Table("worklog", "id", "issueid", "author", "grouplevel", "rolelevel", "worklogbody",
			"created", "updateauthor", "updated", "startdate", "timeworked"),
		attachment: g.w.Table("fileattachment", "id", "issueid", "mimetype", "filename", "created", "filesize",
			"author", "zip", "thumbnailable"),
		userAssoc: g.w.Table("userassociation", "source_name", "sink_node_id", "sink_node_entity",
			"association_type", "sequence", "created"),
		cfValue: g.w.Table("customfieldvalue", "id", "issue", "customfield", "updated", "parentkey", "stringvalue",
			"numbervalue", "textvalue", "datevalue", "valuetype"),
		changeGroup: g.w.Table("changegroup", "id", "issueid", "author", "created"),
		changeItem: g.w.Table("changeitem", "id", "groupid", "fieldtype", "field", "oldvalue", "oldstring",
			"newvalue", "newstring"),
		moved:     g.w.Table("moved_issue_key", "id", "old_issue_key", "issue_id"),
		issueLink: g.w.Table("issuelink", "id", "linktype", "source", "destination", "sequence"),
		remoteLink: g.w.Table("remotelink", "id", "issueid", "globalid", "title", "summary", "url", "iconurl",
			"icontitle", "relationship", "resolved", "applicationtype", "applicationname"),
	}

	g.writeProjects()

	// Sub-tasks hang off their parent through a link of the jira_subtask type.
	seq := map[int64]int{}
	for _, is := range g.issues {
		if is.parent == nil {
			continue
		}
		t.issueLink.Add(g.next("issuelink"), linkSubtask, is.parent.id, is.id, seq[is.parent.id])
		seq[is.parent.id]++
		g.sum.Subtasks++
	}

	epics := map[string][]*issue{} // per project, in creation order
	for _, is := range g.issues {
		if is.typ == typeEpic {
			epics[is.proj.key] = append(epics[is.proj.key], is)
		}
	}
	for _, is := range g.issues {
		if !is.proj.software || is.typ == typeEpic || is.typ == typeSubtask || !g.chance(0.4) {
			continue
		}
		list := epics[is.proj.key]
		n := sort.Search(len(list), func(i int) bool { return !list[i].created.Before(is.created) })
		if n > 0 {
			is.epic = list[g.rng.IntN(n)]
		}
	}

	g.planLinks(t)
	g.seedRemoteLinks(t)

	for _, is := range g.issues {
		g.detail(t, is, byProject[is.proj.key])
	}
	g.sum.Issues = len(g.issues)
	g.sum.Projects = len(g.projects)
	g.checkpoints()
}

// chooseType draws the issue type; a sub-task needs an earlier parent.
func (g *generator) chooseType(is *issue, earlier []*issue) {
	if is.proj.software {
		is.typ = []string{typeStory, typeBug, typeTask, typeEpic, typeSubtask}[g.weighted(40, 25, 20, 5, 10)]
	} else {
		is.typ = []string{typeTask, typeBug, typeImprovement, typeSubtask}[g.weighted(50, 20, 20, 10)]
	}
	if is.typ != typeSubtask {
		return
	}
	var parents []*issue
	for _, e := range earlier {
		if e.typ != typeSubtask && e.typ != typeEpic && e.movedFrom == "" {
			parents = append(parents, e)
		}
	}
	if len(parents) == 0 {
		is.typ = typeTask
		return
	}
	is.parent = pick(g, parents)
}

func (g *generator) writeProjects() {
	pt := g.w.Table("project", "id", "pname", "url", "lead", "description", "pkey", "pcounter", "assigneetype",
		"avatar", "originalkey", "projecttype", "created", "createdby", "updated", "updatedby")
	created := g.start.AddDate(0, 0, -30)
	for _, p := range g.projects {
		kind := "business"
		if p.software {
			kind = "software"
		}
		pt.Add(p.id, p.name, nil, p.lead.key, "Projet « "+p.name+" »", p.key, p.count, p.assigneeType, nil,
			p.origKey, kind, created, nil, created, nil)
	}
}

func (g *generator) planLinks(t issueTables) {
	want := len(g.issues) / 5
	seen := map[[2]int64]bool{}
	for tries := 0; len(seen) < want && tries < want*20+20; tries++ {
		src, dst := pick(g, g.issues), pick(g, g.issues)
		if src == dst || (src.proj.key == "RH") != (dst.proj.key == "RH") {
			continue
		}
		pair := [2]int64{src.id, dst.id}
		if seen[pair] || seen[[2]int64{dst.id, src.id}] {
			continue
		}
		seen[pair] = true
		from := src.created
		if dst.created.After(from) {
			from = dst.created
		}
		l := &link{id: g.next("issuelink"), typ: linkTypes[g.weighted(30, 10, 15, 45)], src: src, dst: dst,
			at: g.after(from, 30)}
		src.links = append(src.links, l)
		t.issueLink.Add(l.id, l.typ.id, src.id, dst.id, nil)
		g.sum.LinksNonSubtask++
	}
}

func (g *generator) seedRemoteLinks(t issueTables) {
	var software []*issue
	for _, is := range g.issues {
		if is.proj.software {
			software = append(software, is)
		}
	}
	if len(software) == 0 {
		return
	}
	for i := range len(g.issues)/30 + 1 {
		is := pick(g, software)
		mr := 100 + i*7
		t.remoteLink.Add(g.next("remotelink"), is.id, fmt.Sprintf("gitlab:mr:%d", mr),
			fmt.Sprintf("MR !%d — %s", mr, pick(g, taskObjects)), nil,
			fmt.Sprintf("https://gitlab.exemple.fr/boutique/web/-/merge_requests/%d", mr), nil, nil,
			"mentionné dans", nil, "com.gitlab", "GitLab")
		g.sum.RemoteLinks++
	}
}

// statusPath draws the final status and the transitions leading to it.
func (g *generator) statusPath(p *project) []string {
	if p.software {
		switch g.weighted(30, 20, 10, 40) {
		case 0:
			if g.chance(0.2) {
				return []string{"3", "10000"}
			}
			return nil
		case 1:
			if g.chance(0.15) {
				return []string{"3", "10002", "3"}
			}
			return []string{"3"}
		case 2:
			return []string{"3", "10002"}
		default:
			return [][]string{{"3", "10002", "10001"}, {"3", "10001"}, {"10001"}}[g.weighted(70, 20, 10)]
		}
	}
	switch g.weighted(25, 15, 5, 25, 30) {
	case 0:
		return nil
	case 1:
		return []string{"3"}
	case 2:
		return [][]string{{"5", "4"}, {"3", "5", "4"}}[g.weighted(50, 50)]
	case 3:
		return [][]string{{"3", "5"}, {"5"}}[g.weighted(60, 40)]
	default:
		return [][]string{{"3", "5", "6"}, {"5", "6"}, {"6"}}[g.weighted(50, 30, 20)]
	}
}

func (g *generator) summaryFor(is *issue) string {
	p := is.proj
	switch {
	case is.typ == typeEpic:
		return pick(g, epicNames)
	case is.typ == typeSubtask:
		return pick(g, subtaskNames)
	case p.key == "RH":
		return strings.ReplaceAll(pick(g, rhSummaries), "{person}", pick(g, rhPersons))
	case p.key == "OPS":
		return strings.ReplaceAll(pick(g, opsSummaries), "{host}", pick(g, hosts))
	case is.typ == typeBug:
		return pick(g, bugSubjects) + " " + pick(g, bugProblems)
	case is.typ == typeStory:
		return "En tant que " + pick(g, storyPersonas) + ", je veux " + pick(g, storyGoals)
	default:
		return pick(g, taskVerbs) + " " + pick(g, taskObjects)
	}
}

// earlierKey returns the key of a random earlier issue of the same project,
// or "" when there is none. earlier is sorted by id.
func (g *generator) earlierKey(is *issue, earlier []*issue) string {
	n := sort.Search(len(earlier), func(i int) bool { return earlier[i].id >= is.id })
	if n == 0 {
		return ""
	}
	e := earlier[g.rng.IntN(n)]
	if e.movedFrom != "" {
		return "" // its key changed since; keep the text unambiguous
	}
	return e.key()
}

func (g *generator) description(is *issue, earlier []*issue) string {
	var b strings.Builder
	b.WriteString("h3. Contexte\n" + pick(g, contextSentences) + "\n\n")
	if is.typ == typeBug {
		b.WriteString("*Étapes pour reproduire :*\n")
		steps := slices.Clone(reproSteps)
		g.rng.Shuffle(len(steps), func(a, c int) { steps[a], steps[c] = steps[c], steps[a] })
		for _, s := range steps[:3] {
			b.WriteString("# " + s + "\n")
		}
		b.WriteString("\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s'arrête sans message_.\n\n")
	} else {
		b.WriteString("h3. Critères d'acceptation\n")
		crit := slices.Clone(acceptance)
		g.rng.Shuffle(len(crit), func(a, c int) { crit[a], crit[c] = crit[c], crit[a] })
		for _, c := range crit[:2+g.rng.IntN(2)] {
			b.WriteString("* " + c + "\n")
		}
		b.WriteString("\n")
	}
	if g.chance(0.2) {
		b.WriteString(pick(g, codeSnippets) + "\n\n")
	}
	if g.chance(0.15) {
		b.WriteString(pick(g, logSnippets) + "\n\n")
	}
	var refs []string
	if g.chance(0.25) {
		refs = append(refs, "Voir "+pick(g, links)+".")
	}
	if k := g.earlierKey(is, earlier); k != "" && g.chance(0.25) {
		refs = append(refs, "Lié à "+k+".")
	}
	if g.chance(0.3) {
		refs = append(refs, "[~"+pick(g, g.devs).name+"] peux-tu valider ?")
	}
	if len(refs) > 0 {
		b.WriteString(strings.Join(refs, " ") + "\n\n")
	}
	for _, f := range is.attached {
		if (f == "capture.png" || f == "maquette.jpg") && g.chance(0.5) {
			b.WriteString("!" + f + "!\n\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

var attachmentKinds = []struct{ name, mime string }{
	{"capture.png", "image/png"}, {"logs-prod.txt", "text/plain"}, {"spec.pdf", "application/pdf"},
	{"maquette.jpg", "image/jpeg"}, {"export.csv", "text/csv"},
}

func isImage(mime string) bool { return strings.HasPrefix(mime, "image/") }

func (g *generator) authorFor(is *issue) *user {
	if is.assignee != nil && g.chance(0.5) {
		return is.assignee
	}
	return pick(g, is.proj.assignees)
}

// detail draws everything about one issue and writes its rows.
func (g *generator) detail(t issueTables, is *issue, earlier []*issue) {
	p := is.proj
	early := is.created.Before(g.start.AddDate(0, 0, 100))
	if (p.key == "DEMO" || p.key == "OPS") && early && g.chance(0.3) {
		is.reporter = g.byName["ancien.collegue"]
	} else {
		is.reporter = pick(g, p.reporters)
	}
	if !g.chance(0.15) {
		is.assignee = pick(g, p.assignees)
	}
	creator := is.reporter
	if g.chance(0.1) {
		creator = g.byName["admin"]
	}
	is.priority = priorities[g.weighted(5, 20, 50, 20, 5)].id
	is.summary = g.summaryFor(is)
	updated := is.created
	touch := func(at time.Time) {
		if at.After(updated) {
			updated = at
		}
	}

	var changes []change

	// Workflow history: every transition is a change group.
	cur := statusByID(p.initialStatus())
	var resolvedAt *time.Time
	resolutionID := ""
	chosen := ""
	last := is.created
	for _, nextID := range g.statusPath(p) {
		next := statusByID(nextID)
		at := g.after(last, 12)
		last = at
		items := []changeItem{{"jira", "status", cur.id, cur.name, next.id, next.name}}
		switch {
		case next.done() && !cur.done():
			if chosen == "" {
				if p.software {
					chosen = []string{"10000", "2", "3"}[g.weighted(80, 10, 10)]
				} else {
					chosen = []string{"1", "2", "3", "4", "5"}[g.weighted(60, 10, 10, 10, 10)]
				}
			}
			resolutionID = chosen
			resolvedAt = &at
			items = append(items, changeItem{"jira", "resolution", nil, nil, chosen, resolutionName(chosen)})
		case !next.done() && cur.done():
			items = append(items, changeItem{"jira", "resolution", resolutionID, resolutionName(resolutionID), nil, nil})
			resolutionID, resolvedAt = "", nil
		}
		changes = append(changes, change{at: at, author: g.authorFor(is), items: items})
		cur = next
	}
	is.status, is.resolution = cur.id, resolutionID
	statusSince := last

	if is.assignee != nil && g.chance(0.3) {
		var from *user
		if g.chance(0.5) {
			from = pick(g, p.assignees)
		}
		item := changeItem{"jira", "assignee", nil, nil, is.assignee.key, is.assignee.display()}
		if from != nil && from != is.assignee {
			item.oldValue, item.oldString = from.key, from.display()
		}
		changes = append(changes, change{at: g.after(is.created, 30), author: g.authorFor(is), items: []changeItem{item}})
	}
	if g.chance(0.15) {
		old := priorities[g.rng.IntN(len(priorities))]
		if old.id != is.priority {
			changes = append(changes, change{at: g.after(is.created, 20), author: g.authorFor(is),
				items: []changeItem{{"jira", "priority", old.id, old.name, is.priority, priorityName(is.priority)}}})
		}
	}
	if g.chance(0.1) {
		old := "[À qualifier] " + is.summary
		if len(old) > 255 {
			old = old[:255]
		}
		changes = append(changes, change{at: g.after(is.created, 5), author: is.reporter,
			items: []changeItem{{"jira", "summary", nil, old, nil, is.summary}}})
	}

	// Attachments come before the description, which may show the images.
	var attTimes []time.Time
	if is.movedFrom == "" && g.chance(0.15) {
		kinds := slices.Clone(attachmentKinds)
		g.rng.Shuffle(len(kinds), func(a, c int) { kinds[a], kinds[c] = kinds[c], kinds[a] })
		for _, k := range kinds[:1+g.rng.IntN(3)] {
			id := g.next("fileattachment")
			at := g.after(is.created, 20)
			author := is.reporter
			if author.deleted {
				author = pick(g, g.devs)
			}
			data := attachmentContent(id, k.name)
			thumb := 0
			if isImage(k.mime) {
				thumb = 1
			}
			t.attachment.Add(id, is.id, k.mime, k.name, at, int64(len(data)), author.key, 0, thumb)
			g.sum.Attachments++
			if g.attachments == 1 {
				g.sum.AttachmentFilesMissing++ // the second attachment is lost on disk
				g.sum.AttachmentMissingID = id
			} else {
				g.writeAttachment(is, id, data)
			}
			g.attachments++
			is.attached = append(is.attached, k.name)
			attTimes = append(attTimes, at)
			changes = append(changes, change{at: at, author: author,
				items: []changeItem{{"jira", "Attachment", nil, nil, fmt.Sprint(id), k.name}}})
		}
	}

	var description any
	if g.chance(0.85) {
		d := g.description(is, earlier)
		description = d
		if g.chance(0.1) {
			changes = append(changes, change{at: g.after(is.created, 10), author: is.reporter,
				items: []changeItem{{"jira", "description", nil, pick(g, contextSentences), nil, d}}})
		}
	}
	var environment any
	if is.typ == typeBug && g.chance(0.5) {
		environment = pick(g, environments)
	}
	var due any
	if g.chance(0.2) {
		d := g.midnight(is.created).AddDate(0, 0, 7+g.rng.IntN(54))
		is.due = &d
		due = d
	}

	// Components and versions.
	var comps []component
	for _, c := range p.components {
		if !c.archived {
			comps = append(comps, c)
		}
	}
	if len(comps) > 0 && g.chance(0.5) {
		g.rng.Shuffle(len(comps), func(a, c int) { comps[a], comps[c] = comps[c], comps[a] })
		k := 1
		if len(comps) > 1 && g.chance(0.2) {
			k = 2
		}
		for _, c := range comps[:k] {
			g.assoc(is.id, "Issue", c.id, "Component", "IssueComponent")
			g.sum.IssueComponents++
		}
	}
	if len(p.versions) > 0 {
		var released, open []version
		for _, v := range p.versions {
			if v.released {
				released = append(released, v)
			}
			if !v.archived {
				open = append(open, v)
			}
		}
		if is.typ == typeBug && len(released) > 0 && g.chance(0.5) {
			v := pick(g, released)
			g.assoc(is.id, "Issue", v.id, "Version", "IssueVersion")
			g.sum.IssueVersionsAffects++
		}
		if len(open) > 0 && g.chance(0.5) {
			v := pick(g, open)
			g.assoc(is.id, "Issue", v.id, "Version", "IssueFixVersion")
			g.sum.IssueVersionsFix++
			if g.chance(0.2) {
				changes = append(changes, change{at: g.after(is.created, 15), author: g.authorFor(is),
					items: []changeItem{{"jira", "Fix Version", nil, nil, fmt.Sprint(v.id), v.name}}})
			}
		}
	}
	// Labels.
	if g.chance(0.5) {
		labels := slices.Clone(systemLabels)
		g.rng.Shuffle(len(labels), func(a, c int) { labels[a], labels[c] = labels[c], labels[a] })
		labels = labels[:1+g.rng.IntN(3)]
		for _, l := range labels {
			t.label.Add(g.next("label"), nil, is.id, l)
			g.sum.LabelsSystem++
		}
		if g.chance(0.15) {
			changes = append(changes, change{at: g.after(is.created, 10), author: g.authorFor(is),
				items: []changeItem{{"jira", "labels", nil, "", nil, strings.Join(labels, " ")}}})
		}
	}
	if g.chance(0.15) {
		themes := slices.Clone(themeLabels)
		g.rng.Shuffle(len(themes), func(a, c int) { themes[a], themes[c] = themes[c], themes[a] })
		for _, l := range themes[:1+g.rng.IntN(2)] {
			t.label.Add(g.next("label"), int64(cfThemes), is.id, l)
			g.sum.LabelsCustom++
		}
	}

	g.customFieldValues(t, is, &changes)

	// Outward links are recorded in the source issue's history.
	for _, l := range is.links {
		changes = append(changes, change{at: l.at, author: g.authorFor(is),
			items: []changeItem{{"jira", "Link", nil, nil, l.dst.key(), "This issue " + l.typ.outward + " " + l.dst.key()}}})
	}

	if is.movedFrom != "" {
		at := g.after(is.created, 10)
		changes = append(changes, change{at: at, author: g.byName["admin"], items: []changeItem{
			{"jira", "Key", nil, is.movedFrom, nil, is.key()},
			{"jira", "project", fmt.Sprint(g.byKey["OPS"].id), "Exploitation", fmt.Sprint(p.id), p.name},
		}})
		t.moved.Add(g.next("moved_issue_key"), is.movedFrom, is.id)
	}

	// Comments.
	nComments := []int{0, 1, 2, 3, 4, 5, 6}[g.weighted(30, 20, 20, 12, 8, 5, 5)]
	at := is.created
	for range nComments {
		at = g.after(at, 15)
		author := pick(g, p.reporters)
		if is.assignee != nil && g.chance(0.3) {
			author = is.assignee
		}
		if early && (p.key == "DEMO" || p.key == "OPS") && at.Before(g.start.AddDate(0, 0, 120)) && g.chance(0.2) {
			author = g.byName["ancien.collegue"]
		}
		body := pick(g, commentBodies)
		body = strings.ReplaceAll(body, "{mention}", "[~"+pick(g, g.devs).name+"]")
		k := g.earlierKey(is, earlier)
		if k == "" {
			k = is.key()
		}
		body = strings.ReplaceAll(body, "{key}", k)
		var group, role any
		if p.key != "RH" {
			switch r := g.rng.Float64(); {
			case r < 0.07:
				group = "developpeurs"
			case r < 0.11:
				role = roleAdmins
			}
		}
		if group != nil || role != nil {
			g.sum.RestrictedComments++
		}
		updatedBy, updatedAt := author, at
		if g.chance(0.1) {
			updatedAt = g.after(at, 3)
			if g.chance(0.3) {
				updatedBy = g.byName["admin"]
			}
		}
		t.comment.Add(g.next("jiraaction"), is.id, author.key, "comment", group, role, body, at, updatedBy.key,
			updatedAt, nil)
		g.sum.Comments++
		is.comments++
		touch(updatedAt)
	}

	// Work logs; the time spent on the issue is their sum.
	var spent int64
	if is.typ != typeEpic && g.chance(0.25) {
		start := is.created
		for range 1 + g.rng.IntN(4) {
			start = g.after(start, 10)
			created := g.after(start, 0)
			seconds := int64(900 * (1 + g.rng.IntN(32)))
			spent += seconds
			author := is.assignee
			if author == nil {
				author = pick(g, p.assignees)
			}
			var group any
			if g.worklogs == 0 {
				group = "developpeurs" // exactly one restricted work log
			}
			var body any
			if g.chance(0.7) {
				body = pick(g, worklogBodies)
			}
			t.worklog.Add(g.next("worklog"), is.id, author.key, group, nil, body, created, author.key, created,
				start, seconds)
			g.worklogs++
			g.sum.Worklogs++
			touch(created)
		}
	}
	var original, remaining, spentV any
	if spent > 0 {
		spentV = spent
	}
	if g.chance(0.4) {
		o := int64(3600 * (1 + g.rng.IntN(40)))
		original = o
		remaining = max(0, o-spent)
	}

	// Watchers and votes.
	watchers := []*user{}
	addWatcher := func(u *user) {
		if u != nil && !u.deleted && !slices.Contains(watchers, u) {
			watchers = append(watchers, u)
		}
	}
	addWatcher(is.reporter)
	addWatcher(is.assignee)
	for range g.rng.IntN(3) {
		addWatcher(pick(g, g.everyone))
	}
	for _, u := range watchers {
		t.userAssoc.Add(u.key, is.id, "Issue", "WatchIssue", nil, g.after(is.created, 5))
		g.sum.Watchers++
	}
	var voters []*user
	if g.chance(0.1) {
		for range 1 + g.rng.IntN(3) {
			u := pick(g, g.everyone)
			if u != is.reporter && !slices.Contains(voters, u) {
				voters = append(voters, u)
			}
		}
	}
	for _, u := range voters {
		t.userAssoc.Add(u.key, is.id, "Issue", "VoteIssue", nil, g.after(is.created, 30))
		g.sum.Votes++
	}

	// History, in chronological order.
	sort.SliceStable(changes, func(a, b int) bool { return changes[a].at.Before(changes[b].at) })
	for _, c := range changes {
		gid := g.next("changegroup")
		t.changeGroup.Add(gid, is.id, keyOf(c.author), c.at)
		g.sum.ChangeGroups++
		for _, it := range c.items {
			t.changeItem.Add(g.next("changeitem"), gid, it.fieldType, it.field, it.oldValue, it.oldString,
				it.newValue, it.newString)
			g.sum.ChangeItems++
		}
		touch(c.at)
	}
	for _, a := range attTimes {
		touch(a)
	}

	var security any
	if p.key == "DEMO" && is.movedFrom == "" {
		switch r := g.rng.Float64(); {
		case r < 0.06:
			security = int64(10000)
		case r < 0.09:
			security = int64(10001)
		}
	}
	if security != nil {
		g.sum.SecuredIssues++
	}

	var resolved any
	if resolvedAt != nil {
		resolved = *resolvedAt
	}
	wf := g.next("os_wfentry")
	t.issue.Add(is.id, nil, is.num, p.id, keyOf(is.reporter), keyOf(is.assignee), keyOf(creator), is.typ,
		is.summary, description, environment, is.priority, nullable(is.resolution), is.status, is.created,
		updated, due, resolved, len(voters), len(watchers), original, remaining, spentV, wf, security, nil, nil,
		nil)
	step, ok := workflowSteps[p.workflow()][is.status]
	if !ok {
		panic(fmt.Sprintf("jirafixture: status %s is not in workflow %s", is.status, p.workflow()))
	}
	t.wfEntry.Add(wf, p.workflow(), 0, 1)
	t.curStep.Add(g.next("os_currentstep"), wf, step.step, 0, nil, statusSince, nil, nil, step.osStatus, nil)
}

// customFieldValues draws the custom field values of an issue, stored the
// way Jira stores them (one row per value).
func (g *generator) customFieldValues(t issueTables, is *issue, changes *[]change) {
	p := is.proj
	add := func(field int64, parentKey, str, num, text, date any) {
		t.cfValue.Add(g.next("customfieldvalue"), is.id, field, nil, parentKey, str, num, text, date, nil)
		g.sum.CustomFieldValues++
	}
	if p.software && (is.typ == typeStory || is.typ == typeBug) && g.chance(0.6) {
		points := []float64{0.5, 1, 2, 3, 5, 8, 13}
		sp := pick(g, points)
		add(cfStoryPoints, nil, nil, sp, nil, nil)
		if g.chance(0.3) {
			old := pick(g, points)
			if old != sp {
				*changes = append(*changes, change{at: g.after(is.created, 10), author: g.authorFor(is),
					items: []changeItem{{"custom", "Story Points", nil, fmt.Sprint(old), nil, fmt.Sprint(sp)}}})
			}
		}
	}
	if is.typ == typeBug && g.chance(0.7) {
		add(cfEnvironment, nil, fmt.Sprint(pick(g, optEnvironments)), nil, nil, nil)
	}
	if p.software && g.chance(0.3) {
		teams := slices.Clone(optTeams)
		g.rng.Shuffle(len(teams), func(a, c int) { teams[a], teams[c] = teams[c], teams[a] })
		for _, o := range teams[:1+g.rng.IntN(3)] {
			add(cfTeams, nil, fmt.Sprint(o), nil, nil, nil)
		}
	}
	if !p.software && g.chance(0.3) {
		loc := pick(g, optLocations)
		add(cfLocation, nil, fmt.Sprint(loc.parent), nil, nil, nil)
		add(cfLocation, "1", fmt.Sprint(loc.child), nil, nil, nil)
	}
	if p.software && g.chance(0.2) {
		add(cfAcceptance, nil, nil, nil, nil, g.midnight(is.created).AddDate(0, 0, 10+g.rng.IntN(60)))
	}
	if !p.software && g.chance(0.3) {
		add(cfOwner, nil, pick(g, p.reporters).key, nil, nil, nil)
	}
	if p.software && statusByID(is.status).done() && g.chance(0.3) {
		add(cfNotes, nil, nil, nil, "Corrigé dans cette version : "+is.summary+".\n* Aucun impact sur les données existantes", nil)
	}
	if p.key == "WEB" && g.chance(0.2) {
		add(cfMockup, nil, fmt.Sprintf("https://maquettes.exemple.fr/web/%d", is.num), nil, nil, nil)
	}
	if p.key == "DEMO" && g.chance(0.2) {
		add(cfClientCode, nil, fmt.Sprintf("CLI-%04d", g.rng.IntN(10000)), nil, nil, nil)
	}
	if is.epic != nil {
		add(cfEpicLink, nil, nil, float64(is.epic.id), nil, nil)
	}
}

// writeAttachment stores an attachment where Jira keeps it:
// <home>/data/attachments/<original project key>/<bucket>/<original key>-<num>/<id>.
func (g *generator) writeAttachment(is *issue, id int64, data []byte) {
	if g.w.Err() != nil {
		return
	}
	bucket := ((is.num-1)/10000 + 1) * 10000
	orig := is.proj.origKey
	dir := filepath.Join(g.opt.JiraHome, "data", "attachments", orig, fmt.Sprint(bucket), fmt.Sprintf("%s-%d", orig, is.num))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		g.w.Fail(err)
		return
	}
	if err := os.WriteFile(filepath.Join(dir, fmt.Sprint(id)), data, 0o644); err != nil {
		g.w.Fail(err)
	}
}

// checkpoints picks five issues whose details the importer tests can verify.
func (g *generator) checkpoints() {
	var chosen []*issue
	addFirst := func(match func(*issue) bool) {
		for _, is := range g.issues {
			if match(is) && !slices.Contains(chosen, is) {
				chosen = append(chosen, is)
				return
			}
		}
	}
	addFirst(func(is *issue) bool { return is.proj.key == "DEMO" && is.num == 1 })
	addFirst(func(is *issue) bool { return is.proj.key == "WEB" })
	addFirst(func(is *issue) bool { return is.parent != nil })
	addFirst(func(is *issue) bool { return is.movedFrom != "" })
	addFirst(func(is *issue) bool { return is.proj.key == "RH" })
	for i := len(g.issues) - 1; len(chosen) < 5 && i >= 0; i-- {
		if !slices.Contains(chosen, g.issues[i]) {
			chosen = append(chosen, g.issues[i])
		}
	}
	for _, is := range chosen {
		c := Checkpoint{
			Key:              is.key(),
			Summary:          is.summary,
			StatusName:       statusByID(is.status).name,
			ReporterUsername: is.reporter.name,
			PriorityName:     priorityName(is.priority),
			Created:          is.created.UTC().Format(time.RFC3339),
			CommentCount:     is.comments,
		}
		if is.assignee != nil {
			c.AssigneeUsername = &is.assignee.name
		}
		if is.resolution != "" {
			r := resolutionName(is.resolution)
			c.ResolutionName = &r
		}
		if is.due != nil {
			d := is.due.Format(time.DateOnly)
			c.DueDate = &d
		}
		if is.parent != nil {
			k := is.parent.key()
			c.ParentKey = &k
		}
		g.sum.Checkpoints = append(g.sum.Checkpoints, c)
	}
}
