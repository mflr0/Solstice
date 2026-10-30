package importer

// Check is an import that writes nothing. It runs the steps of Run, the same
// reading and mapping code, against a target that takes every write and keeps
// none, then reports what the import would do: how much there is, what would
// be left out and why. Only the Jira database (and, to look for attachment
// files, the Jira home) is touched.

import (
	"context"
	"database/sql"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"solstice/internal/i18n"
	"solstice/internal/jiradb"
)

// CheckReport is what Check finds in a Jira database.
type CheckReport struct {
	Source CheckSource `json:"source"`
	// Refused is why the import would not run on this source, "" if it would.
	// The figures then cover the steps that ran before the refusal.
	Refused  string    `json:"refused,omitempty"`
	Counts   Counts    `json:"counts"`
	Steps    []Step    `json:"steps"` // the lines of an import's report
	LeftOut  []LeftOut `json:"leftOut"`
	Warnings []string  `json:"warnings"` // those of an import
	// Anonymous tells that the report holds no content from the Jira
	// database: no name, key or id chosen by people, no text of an error, and
	// not the database server's version.
	Anonymous bool `json:"anonymous,omitempty"`
}

// CheckSource names the database that was read.
type CheckSource struct {
	Database string `json:"database"` // postgres or mysql
	Server   string `json:"server"`   // the database server's own version text
	// Jira's version and build number, "" when the database does not record them.
	Jira  string `json:"jira"`
	Build string `json:"build"`
}

// Counts are the records Jira holds that an import would copy.
type Counts struct {
	Projects               int            `json:"projects"`
	Issues                 int            `json:"issues"`
	Comments               int            `json:"comments"`
	Worklogs               int            `json:"worklogs"`
	Attachments            int            `json:"attachments"`
	AttachmentsWithFile    int            `json:"attachmentsWithFile"`
	AttachmentsWithoutFile int            `json:"attachmentsWithoutFile"`
	FilesChecked           bool           `json:"filesChecked"` // false without a Jira home
	Users                  int            `json:"users"`
	UsersWithPassword      int            `json:"usersWithPassword"`
	Groups                 int            `json:"groups"`
	Roles                  int            `json:"roles"`
	Workflows              int            `json:"workflows"`
	CustomFields           int            `json:"customFields"`
	CustomFieldTypes       map[string]int `json:"customFieldTypes"`
	Boards                 int            `json:"boards"`
	Sprints                int            `json:"sprints"`
	Filters                int            `json:"filters"`
}

// LeftOut is something an import would not carry over, and why.
type LeftOut struct {
	What   string `json:"what"`
	Count  int    `json:"count"`
	Reason string `json:"reason"`
}

// Check reads the Jira database of opt.SourceURL as an import would, and
// writes nothing: opt.DataDir and Solstice's database are not used. A source
// the import would refuse for its schema gives a report with Refused set; any
// other error means the source could not be read.
func Check(ctx context.Context, opt Options) (*CheckReport, error) {
	if opt.SourceTZ == nil {
		opt.SourceTZ = time.UTC // dates only: no count depends on it
	}
	if opt.Log == nil {
		opt.Log = slog.Default()
	}
	src, engine, err := jiradb.Open(opt.SourceURL, opt.SourceTZ)
	if err != nil {
		return nil, err
	}
	defer src.Close()
	if err := src.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("cannot reach the Jira database: %w", err)
	}
	return check(ctx, src, engine, opt)
}

func check(ctx context.Context, src *sql.DB, engine jiradb.Engine, opt Options) (*CheckReport, error) {
	r := &run{
		ctx:    ctx,
		src:    src,
		engine: engine,
		tz:     opt.SourceTZ,
		dst:    counter{},
		opt:    opt,
		rep:    &Report{},
		check:  &checkData{sprints: map[int64]bool{}, sprintRefs: map[[2]int64]bool{}},
	}
	rep := &CheckReport{Source: r.describe()}
	err := r.runSteps() // importPermissions refuses an old schema, as in an import
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if errors.Is(err, errTooOld) {
		rep.Refused = err.Error()
	} else if err != nil {
		return nil, err
	}
	r.summarise(rep)
	if opt.Anonymous {
		rep.Source.Server = ""
		rep.Anonymous = true
	}
	return rep, nil
}

// counter is the target of a check: it takes every write and keeps none.
type counter struct{}

func (counter) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	return noRows{}, nil
}

type noRows struct{}

func (noRows) LastInsertId() (int64, error) { return 0, nil }
func (noRows) RowsAffected() (int64, error) { return 0, nil }

// checkData is what a check gathers on the way, beyond the report's steps. Its
// methods may be called on nil, which is what an import has: they do nothing.
// It also answers the three questions an import puts to its target, which a
// check cannot: how many rows an update or a delete touched.
type checkData struct {
	passwords                           int
	attachmentsWith, attachmentsWithout int
	unreadableFiles                     int   // attachment paths that could not be looked at
	unreadableFirst                     error // the first of them
	workflows                           []workflowFacts
	broken                              map[string]error
	changeGroups                        []int64 // ids of the changes that would be imported
	orphanItems                         int64   // details of a change that would not be
	sprints                             map[int64]bool
	sprintRefs                          map[[2]int64]bool // (sprint, issue) pairs of the Sprint field
}

type workflowFacts struct {
	name  string
	logic workflowLogic
}

// password counts a user whose Jira password hash is kept.
func (c *checkData) password() {
	if c != nil {
		c.passwords++
	}
}

func (c *checkData) attachment(withFile bool) {
	if c == nil {
		return
	}
	if withFile {
		c.attachmentsWith++
	} else {
		c.attachmentsWithout++
	}
}

func (c *checkData) unreadable(err error) {
	if c == nil {
		return
	}
	if c.unreadableFiles++; c.unreadableFirst == nil {
		c.unreadableFirst = err
	}
}

// workflow notes a workflow that would be imported, and what is not carried
// over from its descriptor.
func (c *checkData) workflow(w *workflow, descriptor string) {
	if c != nil {
		c.workflows = append(c.workflows, workflowFacts{w.name, countWorkflowLogic(descriptor)})
	}
}

func (c *checkData) brokenWorkflow(name string, err error) {
	if c == nil {
		return
	}
	if c.broken == nil {
		c.broken = map[string]error{}
	}
	c.broken[name] = err
}

func (c *checkData) changeGroup(id int64) {
	if c != nil {
		c.changeGroups = append(c.changeGroups, id)
	}
}

func (c *checkData) sortChangeGroups() {
	if c != nil {
		slices.Sort(c.changeGroups)
	}
}

// changeItem notes a detail of a change; one whose change is not imported is
// an orphan.
func (c *checkData) changeItem(group int64) {
	if c == nil {
		return
	}
	if _, ok := slices.BinarySearch(c.changeGroups, group); !ok {
		c.orphanItems++
	}
}

func (c *checkData) sprint(id int64) {
	if c != nil {
		c.sprints[id] = true
	}
}

// sprintMember notes a value of the Sprint field as the import's SQL reads it:
// a string of digits is the sprint's id, even one too long to be an id; any
// other string leaves the number, rounded like CAST AS UNSIGNED does.
func (c *checkData) sprintMember(issue int64, str sql.NullString, num sql.NullFloat64) {
	if c == nil {
		return
	}
	var id int64
	if isDigits(str.String) {
		n, err := strconv.ParseUint(str.String, 10, 63)
		if err != nil { // too big for a sprint id
			return
		}
		id = int64(n)
	} else if f := math.Round(num.Float64); num.Valid && f >= 0 && f < 1<<63 {
		id = int64(f)
	} else {
		return
	}
	c.sprintRefs[[2]int64{id, issue}] = true
}

// isDigits tells whether s is made of ASCII digits only, and not empty.
func isDigits(s string) bool {
	return s != "" && strings.IndexFunc(s, func(r rune) bool { return r < '0' || r > '9' }) < 0
}

// sprintMembers counts the issues in sprints that would be imported.
func (c *checkData) sprintMembers() int64 {
	var n int64
	for ref := range c.sprintRefs {
		if c.sprints[ref[0]] {
			n++
		}
	}
	return n
}

// workflowLogic is what a Jira workflow does beyond moving an issue between
// statuses: Java conditions, validators and post-functions, which an import
// does not carry over.
type workflowLogic struct {
	conditions, validators, functions int
}

// builtInFunctions are the post-functions every Jira transition has, which
// Solstice does itself (status, history, comment, index, event).
var builtInFunctions = []string{
	"UpdateIssueStatusFunction", "CreateCommentFunction", "GenerateChangeHistoryFunction",
	"IssueReindexFunction", "FireIssueEventFunction", "IssueCreateFunction",
}

// countWorkflowLogic counts the conditions, validators and post-functions of
// an OSWorkflow descriptor, Jira's built-in post-functions left out. A
// descriptor that cannot be read counts for nothing: parseWorkflow reports it.
func countWorkflowLogic(descriptor string) workflowLogic {
	var logic workflowLogic
	dec := xml.NewDecoder(strings.NewReader(descriptor))
	dec.Strict = false
	var kind string // the condition, validator or function being read
	var class strings.Builder
	inClass := false
	for {
		tok, err := dec.Token()
		if err != nil {
			return logic
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "condition", "validator", "function":
				kind = t.Name.Local
				class.Reset()
			case "arg":
				inClass = kind != "" && slices.ContainsFunc(t.Attr, func(a xml.Attr) bool {
					return a.Name.Local == "name" && a.Value == "class.name"
				})
			}
		case xml.CharData:
			if inClass {
				class.Write(t)
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "arg":
				inClass = false
			case "condition":
				logic.conditions++
				kind = ""
			case "validator":
				logic.validators++
				kind = ""
			case "function":
				name := strings.TrimSpace(class.String())
				if !slices.Contains(builtInFunctions, name[strings.LastIndexByte(name, '.')+1:]) {
					logic.functions++
				}
				kind = ""
			}
		}
	}
}

// skipReasons say why the records an import counts as skipped were.
var skipReasons = map[string]string{
	"issues":                   "project missing in Jira, or no issue number",
	"old keys of moved issues": "issue not imported, or key empty or repeated",
	"labels":                   "issue not imported, empty label, or unknown field",
	"issue versions":           "orphan: the issue, component or version is not imported",
	"issue links":              "orphan: link type unknown, or an issue not imported",
	"remote links":             "orphan: issue not imported, or no address",
	"votes":                    "orphan: user unknown, or issue not imported (watchers included)",
	"comments":                 "orphan: issue not imported",
	"worklogs":                 "orphan: issue not imported",
	"attachments":              "orphan: issue not imported",
	"custom field values":      "orphan: field or issue not imported, or user unknown",
	"changes (history)":        "orphan: issue not imported",
	"change details":           "orphan: change not imported",
	"role members":             "orphan, or an actor that is neither a user nor a group",
	"project permissions":      "grantee Solstice cannot express: access removed, never widened",
	"level members":            "grantee Solstice cannot express: fewer people see these issues, never more",
	"components":               "orphan: project not imported",
	"versions":                 "orphan: project not imported",
	"field options":            "orphan: custom field not imported",
	"boards":                   "its filter no longer exists in Jira",
	"filter shares":            "share Solstice cannot express, or its filter, project or role not imported",
	"favorite filters":         "orphan: user or filter not imported, or repeated",
	"workflows (per project)":  "workflow whose definition cannot be read",
}

// describe names the database and, where the database records them, Jira's
// version and build.
func (r *run) describe() CheckSource {
	s := CheckSource{Database: r.engine.String()}
	r.src.QueryRowContext(r.ctx, "SELECT version()").Scan(&s.Server)
	s.Server, _, _ = strings.Cut(s.Server, " on ") // PostgreSQL adds its platform and compiler
	if r.hasColumn("propertyentry", "property_key") && r.hasColumn("propertystring", "propertyvalue") {
		rows, err := r.src.QueryContext(r.ctx, `SELECT pe.property_key, ps.propertyvalue FROM propertyentry pe
			JOIN propertystring ps ON ps.id = pe.id
			WHERE pe.entity_name = 'jira.properties' AND pe.entity_id = 1 AND pe.property_key LIKE 'jira.version%'`)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var key, value sql.NullString
				if rows.Scan(&key, &value) != nil {
					break
				}
				switch key.String {
				case "jira.version":
					s.Jira = value.String
				case "jira.version.patched":
					s.Build = value.String
				}
			}
		}
	}
	return s
}

// summarise fills the report from what the steps found.
func (r *run) summarise(rep *CheckReport) {
	c := r.check
	count := func(step string) int { return max(r.rep.Count(step), 0) }
	rep.Counts = Counts{
		Projects:          count("projects"),
		Issues:            count("issues"),
		Comments:          count("comments"),
		Worklogs:          count("worklogs"),
		Attachments:       count("attachments"),
		FilesChecked:      r.opt.JiraHome != "",
		Users:             count("users"),
		UsersWithPassword: c.passwords,
		Groups:            count("groups"),
		Roles:             count("roles"),
		Workflows:         len(c.workflows),
		CustomFields:      len(r.fields),
		CustomFieldTypes:  map[string]int{},
		Boards:            count("boards"),
		Sprints:           count("sprints"),
		Filters:           count("saved filters"),
	}
	if rep.Counts.FilesChecked {
		rep.Counts.AttachmentsWithFile, rep.Counts.AttachmentsWithoutFile = c.attachmentsWith, c.attachmentsWithout
	}
	for _, typeKey := range r.fields {
		rep.Counts.CustomFieldTypes[typeKey]++
	}
	if c.unreadableFiles > 0 {
		r.rep.warn(i18n.N(c.unreadableFiles, i18n.ImportWarnUnreadablePathsOne, i18n.ImportWarnUnreadablePathsMany,
			c.unreadableFirst))
	}
	rep.Steps = r.rep.Steps
	rep.Warnings = make([]string, len(r.rep.Warnings))
	for i, w := range r.rep.Warnings {
		if r.opt.Anonymous {
			w = anonymise(w)
		}
		rep.Warnings[i] = w.String()
	}
	rep.LeftOut = []LeftOut{}
	for _, s := range r.rep.Steps {
		if s.Skipped > 0 {
			reason, ok := skipReasons[s.Name]
			if !ok {
				reason = "orphaned references or not carried over"
			}
			rep.LeftOut = append(rep.LeftOut, LeftOut{s.Name, s.Skipped, reason})
		}
	}
	if rep.Counts.AttachmentsWithoutFile > 0 {
		rep.LeftOut = append(rep.LeftOut, LeftOut{"attachment files", c.attachmentsWithout,
			"not found in the Jira home: only the metadata is imported"})
	}
	workflow := func(name string) string { return `workflow "` + name + `"` }
	if r.opt.Anonymous {
		numbers := map[string]int{}
		workflow = func(name string) string {
			if numbers[name] == 0 {
				numbers[name] = len(numbers) + 1
			}
			return fmt.Sprintf("workflow #%d", numbers[name])
		}
	}
	for _, w := range c.workflows {
		for _, l := range []struct {
			what string
			n    int
		}{{"conditions", w.logic.conditions}, {"validators", w.logic.validators}, {"post-functions", w.logic.functions}} {
			if l.n > 0 {
				rep.LeftOut = append(rep.LeftOut, LeftOut{workflow(w.name) + ": " + l.what, l.n,
					"Java code: Solstice applies its own rules (permissions, resolution set on a done status)"})
			}
		}
	}
	for _, name := range slices.Sorted(maps.Keys(c.broken)) {
		reason := c.broken[name].Error()
		if r.opt.Anonymous {
			reason = "its definition cannot be read"
		}
		rep.LeftOut = append(rep.LeftOut, LeftOut{workflow(name), 1, reason})
	}
}

// hidden stands, in an anonymous report, for a value from the Jira database.
const hidden = "[hidden]"

// schemaWarnings are the warnings whose values name a type or a table, never
// content: an anonymous report keeps them whole.
var schemaWarnings = []i18n.Key{
	i18n.ImportWarnUnknownValuesOne, i18n.ImportWarnUnknownValuesMany, // a Solstice table
	i18n.ImportWarnPermissionTypeOne, i18n.ImportWarnPermissionTypeMany, // a Jira grantee type
}

// anonymise hides the values of a warning, its counts kept. A warning not
// known to name schema only is hidden, so that a new one leaks nothing.
func anonymise(m i18n.Message) i18n.Message {
	if m.Key == "" {
		return i18n.Raw(hidden)
	}
	if slices.Contains(schemaWarnings, m.Key) {
		return m
	}
	args := slices.Clone(m.Args)
	for i, a := range args {
		if _, count := a.(int); !count {
			args[i] = hidden
		}
	}
	m.Args = args
	return m
}

// WriteText prints the report for a person.
func (c *CheckReport) WriteText(w io.Writer) {
	s := c.Source
	fmt.Fprintf(w, "\nJira database: %s", s.Database)
	if s.Server != "" {
		fmt.Fprintf(w, " (%s)", s.Server)
	}
	switch {
	case s.Jira != "" && s.Build != "":
		fmt.Fprintf(w, "\nJira version:  %s (build %s)", s.Jira, s.Build)
	case s.Jira != "" || s.Build != "":
		fmt.Fprintf(w, "\nJira version:  %s", s.Jira+s.Build)
	default:
		fmt.Fprint(w, "\nJira version:  not recorded in the database")
	}
	if c.Anonymous {
		fmt.Fprint(w, "\n\nAnonymous report: names, keys, ids and error texts from the Jira database are hidden.")
	}
	if c.Refused != "" {
		fmt.Fprintf(w, "\n\nThe import would refuse this source: %s\n", c.Refused)
		fmt.Fprintln(w, "\nFound before the refusal:")
	} else {
		fmt.Fprint(w, "\n\nThe import would work. Found:\n")
	}
	n := c.Counts
	line := func(name string, count int, more string) {
		fmt.Fprintf(w, "  %-38s %8d%s\n", name, count, more)
	}
	line("projects", n.Projects, "")
	line("issues", n.Issues, "")
	line("comments", n.Comments, "")
	line("work logs", n.Worklogs, "")
	files := "   (files not checked: no -jira-home)"
	if n.FilesChecked {
		files = fmt.Sprintf("   (%d with a file in the Jira home, %d without)", n.AttachmentsWithFile, n.AttachmentsWithoutFile)
	}
	line("attachments", n.Attachments, files)
	line("users", n.Users, fmt.Sprintf("   (%d with a usable password)", n.UsersWithPassword))
	line("groups", n.Groups, "")
	line("roles", n.Roles, "")
	line("workflows", n.Workflows, "")
	line("custom fields", n.CustomFields, "")
	for _, typeKey := range slices.Sorted(maps.Keys(n.CustomFieldTypes)) {
		fmt.Fprintf(w, "    %8d  %s\n", n.CustomFieldTypes[typeKey], typeKey)
	}
	line("boards", n.Boards, "")
	line("sprints", n.Sprints, "")
	line("saved filters", n.Filters, "")
	if len(c.LeftOut) > 0 {
		fmt.Fprintln(w, "\nLeft out:")
		for _, l := range c.LeftOut {
			fmt.Fprintf(w, "  - %s: %d (%s)\n", l.What, l.Count, l.Reason)
		}
	}
	if len(c.Warnings) > 0 {
		fmt.Fprintln(w, "\nCheck:")
		for _, warning := range c.Warnings {
			fmt.Fprintln(w, "  - "+warning)
		}
	}
}
