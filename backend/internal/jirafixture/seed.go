package jirafixture

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"time"
)

const (
	dirInternal int64 = 1
	dirLDAP     int64 = 10000

	typeBug         = "1"
	typeTask        = "3"
	typeImprovement = "4"
	typeSubtask     = "5"
	typeEpic        = "10000"
	typeStory       = "10001"

	roleUsers      int64 = 10000
	roleDevelopers int64 = 10001
	roleAdmins     int64 = 10002

	linkSubtask int64 = 10100

	cfStoryPoints = 10100
	cfEnvironment = 10101
	cfTeams       = 10102
	cfLocation    = 10103
	cfAcceptance  = 10104
	cfOwner       = 10105
	cfNotes       = 10106
	cfMockup      = 10107
	cfClientCode  = 10108
	cfThemes      = 10109
	cfEpicLink    = 10110
)

type user struct {
	key, name, first, last, email string
	active                        bool
	deleted                       bool // an app_user row without any cwd_user row
	ldapOnly                      bool
}

func (u *user) display() string {
	if u.deleted {
		return "Ancien collègue"
	}
	return u.first + " " + u.last
}

type status struct {
	id, name string
	category int // Jira status category: 2 new, 4 in progress, 3 done
}

func (s status) done() bool { return s.category == 3 }

// Status ids follow Jira's defaults for the system workflow statuses.
var statuses = []status{
	{"1", "Open", 2},
	{"3", "In Progress", 4},
	{"4", "Reopened", 2},
	{"5", "Resolved", 3},
	{"6", "Closed", 3},
	{"10000", "To Do", 2},
	{"10001", "Done", 3},
	{"10002", "In Review", 4},
}

type workflowStep struct {
	step     int
	osStatus string
}

// workflowSteps maps a status to the step that carries it, per workflow:
// the same status ("In Progress") is a different step in each workflow.
var workflowSteps = map[string]map[string]workflowStep{
	"jira": {
		"1": {1, "Open"}, "3": {3, "Underway"}, "5": {4, "Resolved"}, "4": {5, "Reopened"}, "6": {6, "Closed"},
	},
	"Logiciel": {
		"10000": {1, "open"}, "3": {2, "underway"}, "10002": {3, "review"}, "10001": {4, "done"},
	},
}

func statusByID(id string) status {
	for _, s := range statuses {
		if s.id == id {
			return s
		}
	}
	panic("jirafixture: unknown status " + id)
}

var issueTypes = []struct{ id, name, desc, style string }{
	{typeBug, "Bug", "Un problème qui empêche le produit de fonctionner normalement.", ""},
	{typeTask, "Task", "Une tâche à réaliser.", ""},
	{typeImprovement, "Improvement", "L'amélioration d'une fonctionnalité existante.", ""},
	{typeSubtask, "Sub-task", "Une partie d'un ticket plus large.", "jira_subtask"},
	{typeEpic, "Epic", "Un chantier qui regroupe plusieurs tickets.", ""},
	{typeStory, "Story", "Un besoin exprimé du point de vue de l'utilisateur.", ""},
}

var priorities = []struct{ id, name, color string }{
	{"1", "Highest", "#d04437"},
	{"2", "High", "#f15c75"},
	{"3", "Medium", "#f79232"},
	{"4", "Low", "#707070"},
	{"5", "Lowest", "#999999"},
}

func priorityName(id string) string {
	for _, p := range priorities {
		if p.id == id {
			return p.name
		}
	}
	return ""
}

var resolutions = []struct{ id, name string }{
	{"1", "Fixed"}, {"2", "Won't Fix"}, {"3", "Duplicate"}, {"4", "Incomplete"},
	{"5", "Cannot Reproduce"}, {"10000", "Done"},
}

func resolutionName(id string) string {
	for _, r := range resolutions {
		if r.id == id {
			return r.name
		}
	}
	return ""
}

type linkType struct {
	id                    int64
	name, inward, outward string
	style                 string
}

var linkTypes = []linkType{
	{10000, "Blocks", "is blocked by", "blocks", ""},
	{10001, "Cloners", "is cloned by", "clones", ""},
	{10002, "Duplicate", "is duplicated by", "duplicates", ""},
	{10003, "Relates", "relates to", "relates to", ""},
	{linkSubtask, "jira_subtask_link", "jira_subtask_inward", "jira_subtask_outward", "jira_subtask"},
}

type component struct {
	id       int64
	name     string
	archived bool
}

type version struct {
	id                 int64
	name               string
	released, archived bool
}

type project struct {
	id                   int64
	key, origKey, name   string
	software             bool
	lead                 *user
	assigneeType         int64
	category             int64
	count                int64 // issue counter (pcounter)
	components           []component
	versions             []version
	reporters, assignees []*user
}

func (p *project) initialStatus() string {
	if p.software {
		return "10000"
	}
	return "1"
}

func (p *project) workflow() string {
	if p.software {
		return "Logiciel"
	}
	return "jira" // Jira's built-in workflow, never stored in jiraworkflows
}

type generator struct {
	opt   Options
	rng   *rand.Rand
	tz    *time.Location
	now   time.Time
	start time.Time
	w     *Writer
	sum   *Summary
	ids   map[string]int64

	users    []*user
	byName   map[string]*user
	cwdIDs   map[string]int64 // "dir/name" → cwd_user.id
	projects []*project
	byKey    map[string]*project
	issues   []*issue

	nodeAssoc *Table

	devs, everyone []*user
	attachments    int // attachments generated so far
	worklogs       int
}

func newGenerator(opt Options, w *Writer, sum *Summary) *generator {
	// A fixed "now" keeps the data identical from one run to the next.
	now := time.Date(2026, 10, 1, 18, 0, 0, 0, opt.TZ)
	return &generator{
		opt:    opt,
		rng:    rand.New(rand.NewPCG(opt.Seed, opt.Seed^0x9e3779b97f4a7c15)),
		tz:     opt.TZ,
		now:    now,
		start:  now.AddDate(0, 0, -400),
		w:      w,
		sum:    sum,
		ids:    map[string]int64{},
		byName: map[string]*user{},
		cwdIDs: map[string]int64{},
		byKey:  map[string]*project{},
	}
}

// next returns the next id of a sequence; Jira ids start at 10000.
func (g *generator) next(seq string) int64 {
	v, ok := g.ids[seq]
	if !ok {
		v = 10000
	}
	g.ids[seq] = v + 1
	return v
}

func (g *generator) chance(p float64) bool { return g.rng.Float64() < p }

func pick[T any](g *generator, s []T) T { return s[g.rng.IntN(len(s))] }

// weighted returns an index drawn with the given weights.
func (g *generator) weighted(weights ...int) int {
	total := 0
	for _, w := range weights {
		total += w
	}
	n := g.rng.IntN(total)
	for i, w := range weights {
		if n < w {
			return i
		}
		n -= w
	}
	return len(weights) - 1
}

const (
	dayStart = 8 * 3600
	dayEnd   = 20 * 3600
)

// at returns a local time on the given day (offset from start) and second of
// the day. Every generated instant falls within working hours, which keeps
// clear of daylight-saving ambiguities on MySQL DATETIME columns.
func (g *generator) at(day, sec int) time.Time {
	s := g.start
	return time.Date(s.Year(), s.Month(), s.Day()+day, 0, 0, sec, 0, g.tz)
}

// after returns an instant strictly after t, at most maxDays days later,
// within working hours, and never after now.
func (g *generator) after(t time.Time, maxDays int) time.Time {
	lt := t.In(g.tz)
	d := g.rng.IntN(maxDays + 1)
	lo := dayStart
	if d == 0 {
		lo = lt.Hour()*3600 + lt.Minute()*60 + lt.Second() + 1
		if lo < dayStart {
			lo = dayStart
		}
		if lo >= dayEnd {
			d, lo = 1, dayStart
		}
	}
	sec := lo + g.rng.IntN(dayEnd-lo)
	res := time.Date(lt.Year(), lt.Month(), lt.Day()+d, 0, 0, sec, 0, g.tz)
	if res.After(g.now) {
		if t.Before(g.now) {
			return g.now
		}
		return t.Add(time.Second)
	}
	return res
}

// midnight is the local midnight of t's day: how Jira stores date-only values.
func (g *generator) midnight(t time.Time) time.Time {
	lt := t.In(g.tz)
	return time.Date(lt.Year(), lt.Month(), lt.Day(), 0, 0, 0, 0, g.tz)
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func keyOf(u *user) any {
	if u == nil {
		return nil
	}
	return u.key
}

func (g *generator) assoc(srcID int64, srcEntity string, sinkID int64, sinkEntity, kind string) {
	g.nodeAssoc.Add(srcID, srcEntity, sinkID, sinkEntity, kind, nil)
}

func (g *generator) run() {
	g.nodeAssoc = g.w.Table("nodeassociation", "source_node_id", "source_node_entity", "sink_node_id",
		"sink_node_entity", "association_type", "sequence")
	g.seedDirectory()
	g.seedCatalogs()
	g.seedProjects()
	g.seedSchemes()
	g.seedCustomFields()
	g.seedWorkflows()
	g.seedIssues()
	g.seedFilters()
	g.seedSequences()
}

// --- users and groups ------------------------------------------------------

var internalUsers = []struct{ name, first, last string }{
	{"admin", "Administrateur", "Jira"},
	{"marie.durand", "Marie", "Durand"},
	{"paul.martin", "Paul", "Martin"},
	{"jeanne.dupont", "Jeanne", "Dupont"},
	{"lucas.bernard", "Lucas", "Bernard"},
	{"camille.petit", "Camille", "Petit"},
	{"hugo.robert", "Hugo", "Robert"},
	{"lea.richard", "Léa", "Richard"},
	{"thomas.moreau", "Thomas", "Moreau"},
	{"chloe.simon", "Chloé", "Simon"},
	{"nicolas.laurent", "Nicolas", "Laurent"},
	{"manon.michel", "Manon", "Michel"},
	{"antoine.lefebvre", "Antoine", "Lefebvre"},
	{"sarah.leroy", "Sarah", "Leroy"},
	{"julien.roux", "Julien", "Roux"},
	{"ines.david", "Inès", "David"},
	{"maxime.bertrand", "Maxime", "Bertrand"},
	{"emma.morel", "Emma", "Morel"},
	{"louis.fournier", "Louis", "Fournier"},
	{"alice.girard", "Alice", "Girard"},
}

var ldapOnlyUsers = []struct{ name, first, last string }{
	{"karim.benali", "Karim", "Benali"},
	{"sophie.lambert", "Sophie", "Lambert"},
	{"yann.lecomte", "Yann", "Lecomte"},
}

const (
	inactiveUser   = "emma.morel"
	bothDirsUser   = "alice.girard"
	deletedUserKey = "JIRAUSER10099"
)

// Legacy keys predate Jira 6 user keys; jdoe is a user renamed since.
var specialKeys = map[string]string{"admin": "admin", "paul.martin": "paul.martin", "jeanne.dupont": "jdoe"}

var groupDefs = []struct {
	name, desc string
	dir        int64
	members    []string
}{
	{"jira-administrators", "Administrateurs de Jira", dirInternal, []string{"admin", "marie.durand"}},
	{"jira-software-users", "Utilisateurs de Jira Software", dirInternal, nil}, // every internal user
	{"developpeurs", "Équipe de développement", dirInternal, []string{"paul.martin", "jeanne.dupont",
		"lucas.bernard", "hugo.robert", "thomas.moreau", "nicolas.laurent", "julien.roux", "maxime.bertrand"}},
	{"equipe-mobile", "Équipe mobile", dirInternal, []string{"camille.petit", "antoine.lefebvre", "ines.david"}},
	{"rh", "Ressources humaines", dirInternal, []string{"lea.richard", "sarah.leroy", "chloe.simon"}},
	{"direction", "Direction", dirInternal, []string{"marie.durand", "manon.michel", "louis.fournier"}},
	{"ldap-devs", "Développeurs (annuaire LDAP)", dirLDAP, []string{"karim.benali", "sophie.lambert",
		"yann.lecomte", "alice.girard"}},
}

func (g *generator) seedDirectory() {
	t0 := g.start.AddDate(0, -6, 0)
	dirs := g.w.Table("cwd_directory", "id", "directory_name", "lower_directory_name", "created_date",
		"updated_date", "active", "description", "impl_class", "lower_impl_class", "directory_type",
		"directory_position")
	internalImpl := "com.atlassian.crowd.directory.InternalDirectory"
	ldapImpl := "com.atlassian.crowd.directory.OpenLDAP"
	dirs.Add(dirInternal, "Jira Internal Directory", "jira internal directory", t0, t0, 1,
		"Annuaire interne de Jira", internalImpl, strings.ToLower(internalImpl), "INTERNAL", 0)
	dirs.Add(dirLDAP, "LDAP", "ldap", t0, t0, 1, "Annuaire de l'entreprise", ldapImpl,
		strings.ToLower(ldapImpl), "CONNECTOR", 1)

	cwd := g.w.Table("cwd_user", "id", "directory_id", "user_name", "lower_user_name", "active",
		"created_date", "updated_date", "first_name", "lower_first_name", "last_name", "lower_last_name",
		"display_name", "lower_display_name", "email_address", "lower_email_address", "credential",
		"deleted_externally", "external_id")
	addCwd := func(u *user, dir int64, credential string, active bool) {
		id := g.next("cwd_user")
		g.cwdIDs[fmt.Sprintf("%d/%s", dir, u.name)] = id
		created := g.after(t0, 60)
		act := 0
		if active {
			act = 1
		}
		var external any
		if dir == dirLDAP {
			external = fmt.Sprintf("ldap-%08x", g.rng.Uint32())
		}
		cwd.Add(id, dir, u.name, u.name, act, created, created, u.first, strings.ToLower(u.first), u.last,
			strings.ToLower(u.last), u.display(), strings.ToLower(u.display()), u.email,
			strings.ToLower(u.email), credential, nil, external)
	}

	nextKey := int64(10000)
	newUser := func(name, first, last string) *user {
		key, ok := specialKeys[name]
		if !ok {
			key = fmt.Sprintf("JIRAUSER%d", nextKey)
			nextKey++
		}
		u := &user{key: key, name: name, first: first, last: last, email: name + "@exemple.fr", active: true}
		g.users = append(g.users, u)
		g.byName[name] = u
		return u
	}

	for _, d := range internalUsers {
		u := newUser(d.name, d.first, d.last)
		u.active = d.name != inactiveUser
		password := "motdepasse"
		if d.name == "admin" {
			password = "admin"
		}
		salt := make([]byte, 16)
		for i := range salt {
			salt[i] = byte(g.rng.IntN(256))
		}
		addCwd(u, dirInternal, PKCS5S2(password, salt), u.active)
		g.sum.UsersWithPKCS5S2++
		if !u.active {
			g.sum.InactiveUsers++
		}
	}
	// Same login in the LDAP directory: Jira uses the first directory.
	addCwd(g.byName[bothDirsUser], dirLDAP, "nopass", true)
	for _, d := range ldapOnlyUsers {
		u := newUser(d.name, d.first, d.last)
		u.ldapOnly = true
		addCwd(u, dirLDAP, "nopass", true)
		g.sum.UsersLDAPNopass++
	}
	deleted := &user{key: deletedUserKey, name: "ancien.collegue", first: "Ancien", last: "Collègue", deleted: true}
	g.users = append(g.users, deleted)
	g.byName[deleted.name] = deleted
	g.sum.DeletedUsers++

	appUser := g.w.Table("app_user", "id", "user_key", "lower_user_name")
	for _, u := range g.users {
		appUser.Add(g.next("app_user"), u.key, u.name)
		g.sum.AppUsers++
	}

	groups := g.w.Table("cwd_group", "id", "group_name", "lower_group_name", "active", "local",
		"created_date", "updated_date", "description", "lower_description", "group_type", "directory_id",
		"external_id")
	members := g.w.Table("cwd_membership", "id", "parent_id", "child_id", "membership_type", "group_type",
		"parent_name", "lower_parent_name", "child_name", "lower_child_name", "directory_id")
	groupIDs := map[string]int64{}
	for _, gd := range groupDefs {
		id := g.next("cwd_group")
		groupIDs[gd.name] = id
		groups.Add(id, gd.name, gd.name, 1, 0, t0, t0, gd.desc, strings.ToLower(gd.desc), "GROUP", gd.dir, nil)
		g.sum.Groups++
		names := gd.members
		if gd.name == "jira-software-users" {
			for _, d := range internalUsers {
				names = append(names, d.name)
			}
		}
		for _, n := range names {
			members.Add(g.next("cwd_membership"), id, g.cwdIDs[fmt.Sprintf("%d/%s", gd.dir, n)], "GROUP_USER",
				"GROUP", gd.name, gd.name, n, n, gd.dir)
		}
	}
	// equipe-mobile is nested inside developpeurs.
	members.Add(g.next("cwd_membership"), groupIDs["developpeurs"], groupIDs["equipe-mobile"], "GROUP_GROUP",
		"GROUP", "developpeurs", "developpeurs", "equipe-mobile", "equipe-mobile", dirInternal)
	g.sum.NestedGroupMemberships++

	gp := g.w.Table("globalpermissionentry", "id", "permission", "group_id")
	gp.Add(g.next("globalpermissionentry"), "ADMINISTER", "jira-administrators")
	gp.Add(g.next("globalpermissionentry"), "SYSTEM_ADMIN", "jira-administrators")
	gp.Add(g.next("globalpermissionentry"), "USE", "jira-software-users")

	// Application access: only members of jira-software-users may sign in.
	// The LDAP-only users are not members, so Jira would refuse them.
	licence := g.w.Table("licenserolesgroup", "id", "license_role_name", "group_id", "primary_group")
	licence.Add(g.next("licenserolesgroup"), "jira-software", "jira-software-users", "Y")
	g.sum.UsersWithoutAccess = g.sum.UsersLDAPNopass

	for _, n := range []string{"paul.martin", "jeanne.dupont", "lucas.bernard", "hugo.robert", "thomas.moreau",
		"nicolas.laurent", "julien.roux", "maxime.bertrand", "camille.petit", "antoine.lefebvre", "ines.david",
		"marie.durand"} {
		g.devs = append(g.devs, g.byName[n])
	}
	for _, u := range g.users {
		if !u.deleted && u.active {
			g.everyone = append(g.everyone, u)
		}
	}
}

func (g *generator) usersNamed(names ...string) []*user {
	out := make([]*user, len(names))
	for i, n := range names {
		out[i] = g.byName[n]
	}
	return out
}

// --- catalogs ----------------------------------------------------------------

func (g *generator) seedCatalogs() {
	it := g.w.Table("issuetype", "id", "sequence", "pname", "pstyle", "description", "iconurl", "avatar")
	for i, t := range issueTypes {
		it.Add(t.id, i, t.name, nullable(t.style), t.desc, nil, nil)
	}
	st := g.w.Table("issuestatus", "id", "sequence", "pname", "description", "iconurl", "statuscategory")
	for i, s := range statuses {
		st.Add(s.id, i, s.name, "Statut « "+s.name+" »", nil, s.category)
	}
	pr := g.w.Table("priority", "id", "sequence", "pname", "description", "iconurl", "status_color")
	for i, p := range priorities {
		pr.Add(p.id, i, p.name, "Priorité "+p.name, nil, p.color)
	}
	rs := g.w.Table("resolution", "id", "sequence", "pname", "description", "iconurl")
	for i, r := range resolutions {
		rs.Add(r.id, i, r.name, "Résolution "+r.name, nil)
	}
	lt := g.w.Table("issuelinktype", "id", "linkname", "inward", "outward", "pstyle", "position")
	for i, l := range linkTypes {
		lt.Add(l.id, l.name, l.inward, l.outward, nullable(l.style), i)
	}
}

// --- projects ----------------------------------------------------------------

func (g *generator) seedProjects() {
	cat := g.w.Table("projectcategory", "id", "cname", "description")
	cat.Add(int64(10000), "Produit", "Projets liés au produit vendu aux clients")
	cat.Add(int64(10001), "Interne", "Projets internes de l'entreprise")

	g.projects = []*project{
		{id: 10000, key: "DEMO", origKey: "DEMO", name: "Démo", software: true, lead: g.byName["marie.durand"],
			assigneeType: 3, category: 10000},
		{id: 10001, key: "WEB", origKey: "SITE", name: "Site web", software: true, lead: g.byName["paul.martin"],
			assigneeType: 2, category: 10000},
		{id: 10002, key: "RH", origKey: "RH", name: "Ressources humaines", lead: g.byName["lea.richard"],
			assigneeType: 3, category: 10001},
		{id: 10003, key: "OPS", origKey: "OPS", name: "Exploitation", lead: g.byName["hugo.robert"],
			assigneeType: 2, category: 10001},
	}
	reportersSoftware := g.usersNamed("marie.durand", "paul.martin", "jeanne.dupont", "lucas.bernard",
		"camille.petit", "hugo.robert", "thomas.moreau", "chloe.simon", "nicolas.laurent", "manon.michel",
		"antoine.lefebvre", "julien.roux", "ines.david", "maxime.bertrand", "emma.morel", "louis.fournier",
		"alice.girard")
	rh := g.usersNamed("lea.richard", "sarah.leroy", "chloe.simon", "admin")
	opsAssignees := g.usersNamed("hugo.robert", "nicolas.laurent", "julien.roux", "karim.benali",
		"sophie.lambert", "yann.lecomte", "thomas.moreau")
	for _, p := range g.projects {
		g.byKey[p.key] = p
		switch p.key {
		case "DEMO", "WEB":
			p.reporters, p.assignees = reportersSoftware, g.devs
		case "RH":
			p.reporters, p.assignees = rh, rh[:3]
		case "OPS":
			p.reporters = append(append([]*user{}, opsAssignees...), g.usersNamed("manon.michel", "louis.fournier",
				"marie.durand")...)
			p.assignees = opsAssignees
		}
		g.assoc(p.id, "Project", p.category, "ProjectCategory", "ProjectCategory")
	}

	pk := g.w.Table("project_key", "id", "project_id", "project_key")
	for _, p := range g.projects {
		if p.origKey != p.key {
			pk.Add(g.next("project_key"), p.id, p.origKey)
		}
		pk.Add(g.next("project_key"), p.id, p.key)
	}

	comps := map[string][]struct {
		name     string
		archived bool
	}{
		"DEMO": {{"API", false}, {"Interface", false}, {"Paiement", false}},
		"WEB":  {{"Front", false}, {"SEO", true}},
		"RH":   {{"Recrutement", false}, {"Paie", false}},
		"OPS":  {{"Serveurs", false}, {"Réseau", false}, {"Sauvegardes", false}},
	}
	ct := g.w.Table("component", "id", "project", "cname", "description", "url", "lead", "assigneetype",
		"archived", "deleted")
	for _, p := range g.projects {
		for _, c := range comps[p.key] {
			id := g.next("component")
			archived := "N"
			if c.archived {
				archived = "Y"
			}
			ct.Add(id, p.id, c.name, "Composant "+c.name, nil, pick(g, p.assignees).key, 0, archived, "N")
			p.components = append(p.components, component{id, c.name, c.archived})
			g.sum.Components++
		}
	}

	vers := map[string][]struct {
		name               string
		released, archived bool
		startDay, endDay   int
	}{
		"DEMO": {{"1.0", true, true, 0, 60}, {"1.1", true, false, 60, 150}, {"2.0", false, false, 150, 420},
			{"2.1", false, false, 300, 480}},
		"WEB": {{"2025.1", true, false, 0, 120}, {"2025.2", true, false, 120, 260}, {"2026.1", false, false, 260, 430}},
	}
	vt := g.w.Table("projectversion", "id", "project", "vname", "description", "sequence", "released",
		"archived", "url", "startdate", "releasedate")
	for _, p := range g.projects {
		for i, v := range vers[p.key] {
			id := g.next("projectversion")
			vt.Add(id, p.id, v.name, "Version "+v.name, i+1, fmt.Sprint(v.released), fmt.Sprint(v.archived), nil,
				g.midnight(g.at(v.startDay, dayStart)), g.midnight(g.at(v.endDay, dayStart)))
			p.versions = append(p.versions, version{id, v.name, v.released, v.archived})
			g.sum.Versions++
		}
	}
}

// --- roles, permissions, security, issue type schemes -----------------------

func (g *generator) seedSchemes() {
	roles := g.w.Table("projectrole", "id", "name", "description")
	roles.Add(roleUsers, "Users", "Utilisateurs du projet")
	roles.Add(roleDevelopers, "Developers", "Développeurs du projet")
	roles.Add(roleAdmins, "Administrators", "Administrateurs du projet")

	actors := g.w.Table("projectroleactor", "id", "pid", "projectroleid", "roletype", "roletypeparameter")
	const groupActor, userActor = "atlassian-group-role-actor", "atlassian-user-role-actor"
	// Default actors (pid NULL) are copied to new projects; they grant nothing.
	actors.Add(g.next("projectroleactor"), nil, roleUsers, groupActor, "jira-software-users")
	actors.Add(g.next("projectroleactor"), nil, roleAdmins, groupActor, "jira-administrators")
	for _, p := range g.projects {
		if p.key == "RH" {
			actors.Add(g.next("projectroleactor"), p.id, roleUsers, groupActor, "rh")
			continue
		}
		actors.Add(g.next("projectroleactor"), p.id, roleUsers, groupActor, "jira-software-users")
		actors.Add(g.next("projectroleactor"), p.id, roleDevelopers, groupActor, "developpeurs")
		actors.Add(g.next("projectroleactor"), p.id, roleDevelopers, userActor, g.byName["thomas.moreau"].key)
		actors.Add(g.next("projectroleactor"), p.id, roleAdmins, userActor, g.byName["admin"].key)
	}

	ps := g.w.Table("permissionscheme", "id", "name", "description")
	ps.Add(int64(0), "Default Permission Scheme", "Schéma de permissions par défaut")
	ps.Add(int64(10000), "RH", "Projet RH : visible des seules ressources humaines")
	perms := g.w.Table("schemepermissions", "id", "scheme", "permission", "perm_type", "perm_parameter",
		"permission_key")
	grant := func(scheme int64, key, typ string, param any) {
		perms.Add(g.next("schemepermissions"), scheme, nil, typ, param, key)
	}
	role := func(id int64) string { return fmt.Sprint(id) }
	grant(0, "BROWSE_PROJECTS", "applicationRole", nil)
	for _, k := range []string{"CREATE_ISSUES", "EDIT_ISSUES", "ADD_COMMENTS", "TRANSITION_ISSUES",
		"CREATE_ATTACHMENTS", "LINK_ISSUES", "WORK_ON_ISSUES", "ASSIGNABLE_USER", "VIEW_VOTERS_AND_WATCHERS"} {
		grant(0, k, "projectrole", role(roleUsers))
	}
	for _, k := range []string{"ASSIGN_ISSUES", "RESOLVE_ISSUES", "CLOSE_ISSUES", "MANAGE_WATCHERS",
		"SET_ISSUE_SECURITY"} {
		grant(0, k, "projectrole", role(roleDevelopers))
	}
	for _, k := range []string{"DELETE_ISSUES", "ADMINISTER_PROJECTS", "EDIT_ALL_COMMENTS", "DELETE_ALL_COMMENTS"} {
		grant(0, k, "projectrole", role(roleAdmins))
	}
	grant(0, "EDIT_OWN_COMMENTS", "applicationRole", nil)
	grant(0, "DELETE_OWN_COMMENTS", "applicationRole", nil)
	grant(0, "EDIT_ISSUES", "reporter", nil)
	grant(0, "TRANSITION_ISSUES", "assignee", nil)
	grant(0, "ADMINISTER_PROJECTS", "lead", nil)
	grant(0, "ADMINISTER_PROJECTS", "user", g.byName["marie.durand"].key)
	grant(0, "LINK_ISSUES", "group", "developpeurs")

	grant(10000, "BROWSE_PROJECTS", "group", "rh")
	grant(10000, "BROWSE_PROJECTS", "projectrole", role(roleAdmins))
	for _, k := range []string{"CREATE_ISSUES", "EDIT_ISSUES", "ADD_COMMENTS", "TRANSITION_ISSUES",
		"ASSIGN_ISSUES", "ASSIGNABLE_USER", "RESOLVE_ISSUES", "CLOSE_ISSUES", "CREATE_ATTACHMENTS",
		"EDIT_OWN_COMMENTS", "DELETE_OWN_COMMENTS"} {
		grant(10000, k, "group", "rh")
	}
	grant(10000, "ADMINISTER_PROJECTS", "projectrole", role(roleAdmins))
	grant(10000, "DELETE_ISSUES", "projectrole", role(roleAdmins))
	g.assoc(10000, "Project", 0, "PermissionScheme", "ProjectScheme")
	g.assoc(10001, "Project", 0, "PermissionScheme", "ProjectScheme")
	g.assoc(10002, "Project", 10000, "PermissionScheme", "ProjectScheme")
	// OPS has no association: Jira falls back to the default scheme (id 0).

	g.w.Table("issuesecurityscheme", "id", "name", "description", "defaultlevel").
		Add(int64(10000), "Confidentialité", "Tickets réservés à une partie de l'équipe", nil)
	levels := g.w.Table("schemeissuesecuritylevels", "id", "name", "description", "scheme")
	levels.Add(int64(10000), "Équipe", "Visible de l'équipe de développement", int64(10000))
	levels.Add(int64(10001), "Restreint", "Visible du rapporteur et de la direction de projet", int64(10000))
	sec := g.w.Table("schemeissuesecurities", "id", "scheme", "security", "sec_type", "sec_parameter")
	sec.Add(g.next("schemeissuesecurities"), int64(10000), int64(10000), "group", "developpeurs")
	sec.Add(g.next("schemeissuesecurities"), int64(10000), int64(10001), "reporter", nil)
	sec.Add(g.next("schemeissuesecurities"), int64(10000), int64(10001), "user", g.byName["marie.durand"].key)
	sec.Add(g.next("schemeissuesecurities"), int64(10000), int64(10001), "projectrole", role(roleAdmins))
	g.assoc(10000, "Project", 10000, "IssueSecurityScheme", "ProjectScheme")

	// Issue type schemes are stored like a custom field configuration of the
	// pseudo-field "issuetype".
	fcs := g.w.Table("fieldconfigscheme", "id", "configname", "description", "fieldid", "customfield")
	fc := g.w.Table("fieldconfiguration", "id", "configname", "description", "fieldid", "customfield")
	fcsit := g.w.Table("fieldconfigschemeissuetype", "id", "issuetype", "fieldconfigscheme", "fieldconfiguration")
	ctx := g.w.Table("configurationcontext", "id", "projectcategory", "project", "customfield", "fieldconfigscheme")
	opts := g.w.Table("optionconfiguration", "id", "fieldid", "optionid", "fieldconfig", "sequence")
	typeScheme := func(id int64, name string, types []string, projects ...int64) {
		fcs.Add(id, name, "Schéma de types de tickets « "+name+" »", "issuetype", nil)
		fc.Add(id, "Configuration de "+name, nil, "issuetype", nil)
		fcsit.Add(g.next("fieldconfigschemeissuetype"), nil, id, id)
		if len(projects) == 0 {
			ctx.Add(g.next("configurationcontext"), nil, nil, "issuetype", id)
		}
		for _, p := range projects {
			ctx.Add(g.next("configurationcontext"), nil, p, "issuetype", id)
		}
		for i, t := range types {
			opts.Add(g.next("optionconfiguration"), "issuetype", t, id, i)
		}
	}
	all := make([]string, len(issueTypes))
	for i, t := range issueTypes {
		all[i] = t.id
	}
	typeScheme(10000, "Default Issue Type Scheme", all)
	typeScheme(10001, "Logiciel", []string{typeStory, typeBug, typeTask, typeEpic, typeSubtask}, 10000, 10001)
}

// --- custom fields -------------------------------------------------------------

type customFieldDef struct {
	id                int64
	name, typ, search string
}

const cfPrefix = "com.atlassian.jira.plugin.system.customfieldtypes:"

var customFields = []customFieldDef{
	{cfStoryPoints, "Story Points", cfPrefix + "float", cfPrefix + "exactnumber"},
	{cfEnvironment, "Environnement client", cfPrefix + "select", cfPrefix + "multiselectsearcher"},
	{cfTeams, "Équipes concernées", cfPrefix + "multiselect", cfPrefix + "multiselectsearcher"},
	{cfLocation, "Lieu", cfPrefix + "cascadingselect", cfPrefix + "cascadingselectsearcher"},
	{cfAcceptance, "Date de recette", cfPrefix + "datepicker", cfPrefix + "daterange"},
	{cfOwner, "Référent métier", cfPrefix + "userpicker", cfPrefix + "userpickergroupsearcher"},
	{cfNotes, "Notes de version", cfPrefix + "textarea", cfPrefix + "textsearcher"},
	{cfMockup, "Lien maquette", cfPrefix + "url", cfPrefix + "exacttextsearcher"},
	{cfClientCode, "Code client", cfPrefix + "textfield", cfPrefix + "textsearcher"},
	{cfThemes, "Thèmes", cfPrefix + "labels", cfPrefix + "labelsearcher"},
	{cfEpicLink, "Epic Link", "com.pyxis.greenhopper.jira:gh-epic-link", "com.pyxis.greenhopper.jira:gh-epic-link-searcher"},
}

// Option ids of the select-like custom fields.
var (
	optEnvironments = []int64{10200, 10201, 10202} // Production, Recette, Développement (10203 is disabled)
	optTeams        = []int64{10210, 10211, 10212, 10213}
	optLocations    = []struct{ parent, child int64 }{{10220, 10221}, {10220, 10222}, {10223, 10224}}
)

func (g *generator) seedCustomFields() {
	cf := g.w.Table("customfield", "id", "cfkey", "customfieldtypekey", "customfieldsearcherkey", "cfname",
		"description", "defaultvalue", "fieldtype", "project", "issuetype", "issueswithvalue", "lastvalueupdate")
	fcs := g.w.Table("fieldconfigscheme", "id", "configname", "description", "fieldid", "customfield")
	fc := g.w.Table("fieldconfiguration", "id", "configname", "description", "fieldid", "customfield")
	fcsit := g.w.Table("fieldconfigschemeissuetype", "id", "issuetype", "fieldconfigscheme", "fieldconfiguration")
	ctx := g.w.Table("configurationcontext", "id", "projectcategory", "project", "customfield", "fieldconfigscheme")
	for _, d := range customFields {
		cf.Add(d.id, nil, d.typ, d.search, d.name, "Champ « "+d.name+" »", nil, nil, nil, nil, nil, nil)
		g.sum.CustomFields++
		fieldID := fmt.Sprintf("customfield_%d", d.id)
		cfg := d.id + 100 // field configuration and scheme share this id
		fcs.Add(cfg, "Default Configuration Scheme for "+d.name, nil, fieldID, d.id)
		fc.Add(cfg, "Default Configuration for "+d.name, nil, fieldID, d.id)
		fcsit.Add(g.next("fieldconfigschemeissuetype"), nil, cfg, cfg)
		ctx.Add(g.next("configurationcontext"), nil, nil, fieldID, cfg)
	}

	opt := g.w.Table("customfieldoption", "id", "customfield", "customfieldconfig", "parentoptionid", "sequence",
		"customvalue", "optiontype", "disabled")
	add := func(id, field int64, parent any, seq int, value, disabled string) {
		opt.Add(id, field, field+100, parent, seq, value, nil, disabled)
		g.sum.CustomFieldOptions++
	}
	add(10200, cfEnvironment, nil, 0, "Production", "N")
	add(10201, cfEnvironment, nil, 1, "Recette", "N")
	add(10202, cfEnvironment, nil, 2, "Développement", "N")
	add(10203, cfEnvironment, nil, 3, "Préproduction", "Y")
	add(10210, cfTeams, nil, 0, "Front", "N")
	add(10211, cfTeams, nil, 1, "Back", "N")
	add(10212, cfTeams, nil, 2, "Mobile", "N")
	add(10213, cfTeams, nil, 3, "Data", "N")
	add(10220, cfLocation, nil, 0, "France", "N")
	add(10221, cfLocation, int64(10220), 0, "Paris", "N")
	add(10222, cfLocation, int64(10220), 1, "Lyon", "N")
	add(10223, cfLocation, nil, 1, "Belgique", "N")
	add(10224, cfLocation, int64(10223), 0, "Bruxelles", "N")
}

// --- workflows -------------------------------------------------------------------

func (g *generator) seedWorkflows() {
	g.w.Table("jiraworkflows", "id", "workflowname", "creatorname", "descriptor", "islocked").
		Add(int64(10000), "Logiciel", g.byName["admin"].key, logicielWorkflow, nil)
	g.w.Table("workflowscheme", "id", "name", "description").
		Add(int64(10000), "Logiciel", "Projets logiciels")
	g.w.Table("workflowschemeentity", "id", "scheme", "workflow", "issuetype").
		Add(int64(10000), int64(10000), "Logiciel", "0")
	g.assoc(10000, "Project", 10000, "WorkflowScheme", "ProjectScheme")
	g.assoc(10001, "Project", 10000, "WorkflowScheme", "ProjectScheme")
}

// --- saved filters and sequences ---------------------------------------------

func (g *generator) seedFilters() {
	sr := g.w.Table("searchrequest", "id", "filtername", "authorname", "description", "username", "groupname",
		"projectid", "reqcontent", "fav_count", "filtername_lower")
	shares := g.w.Table("sharepermissions", "id", "entityid", "entitytype", "sharetype", "param1", "param2", "rights")
	favs := g.w.Table("favouriteassociations", "id", "username", "entitytype", "entityid", "sequence")
	// share is (sharetype, param1, param2) as Jira stores it; fans are the
	// users who marked the filter as a favourite (fav_count matches).
	add := func(name, owner, desc string, project any, jql string, share [3]any, fans ...string) {
		key := g.byName[owner].key
		id := g.next("searchrequest")
		sr.Add(id, name, key, desc, key, nil, project, jql, len(fans), strings.ToLower(name))
		g.sum.Filters++
		shares.Add(g.next("sharepermissions"), id, "SearchRequest", share[0], share[1], share[2], 1)
		g.sum.FilterShares++
		for i, fan := range fans {
			favs.Add(g.next("favouriteassociations"), g.byName[fan].key, "SearchRequest", id, i)
			g.sum.FilterFavourites++
		}
	}
	add("Mes tickets ouverts", "marie.durand", "Ce qui m'est assigné et reste à faire", nil,
		"assignee = currentUser() AND resolution = Unresolved ORDER BY priority DESC, updated DESC",
		[3]any{"loggedin", nil, nil}, "marie.durand", "paul.martin", "hugo.robert")
	add("Bugs DEMO en cours", "paul.martin", "", int64(10000),
		`project = DEMO AND issuetype = Bug AND status in ("In Progress", "In Review") ORDER BY created DESC`,
		[3]any{"project", "10000", nil}, "paul.martin", "jeanne.dupont")
	add("Créés cette semaine", "hugo.robert", "Nouveaux tickets des sept derniers jours", nil,
		"created >= -7d ORDER BY created DESC", [3]any{"group", "developpeurs", nil}, "hugo.robert")
	// Shared with the Users role of RH only: nobody outside HR may see it.
	add("RH — à traiter", "lea.richard", "", int64(10002), `project = RH AND status = Open ORDER BY duedate ASC`,
		[3]any{"project", "10002", "10000"}, "lea.richard")
}

func (g *generator) seedSequences() {
	seq := g.w.Table("SEQUENCE_VALUE_ITEM", "seq_name", "seq_id")
	for _, s := range []struct{ entity, counter string }{
		{"Issue", "jiraissue"}, {"Action", "jiraaction"}, {"ChangeGroup", "changegroup"},
		{"ChangeItem", "changeitem"}, {"FileAttachment", "fileattachment"}, {"Worklog", "worklog"},
		{"IssueLink", "issuelink"}, {"Label", "label"}, {"CustomFieldValue", "customfieldvalue"},
		{"OSWorkflowEntry", "os_wfentry"}, {"OSCurrentStep", "os_currentstep"}, {"User", "cwd_user"},
		{"ApplicationUser", "app_user"}, {"Group", "cwd_group"}, {"Membership", "cwd_membership"},
	} {
		// OFBiz hands out ids in banks; the stored value is the next bank start.
		next := g.ids[s.counter]
		if next == 0 {
			next = 10000
		}
		seq.Add(s.entity, (next/100+1)*100)
	}
}
