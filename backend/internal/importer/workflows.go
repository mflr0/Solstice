package importer

import (
	"database/sql"
	"encoding/xml"
	"fmt"
	"slices"
	"strings"
)

// workflow is a Jira workflow reduced to what Solstice keeps: statuses and the
// transitions between them. Jira's conditions, validators and post-functions
// are Java classes; Solstice applies its own simple rules instead (permissions,
// resolution set when reaching a "done" status).
type workflow struct {
	name        string
	initial     string   // status id of new issues
	statuses    []string // in step order
	transitions []transition
}

type transition struct {
	name string
	from string // "" = from any status
	to   string
}

// OSWorkflow descriptor, as stored in jiraworkflows.descriptor.
type osWorkflow struct {
	Initial []osAction `xml:"initial-actions>action"`
	Global  []osAction `xml:"global-actions>action"`
	Common  []osAction `xml:"common-actions>action"`
	Steps   []osStep   `xml:"steps>step"`
}

type osStep struct {
	ID      string     `xml:"id,attr"`
	Name    string     `xml:"name,attr"`
	Meta    []osMeta   `xml:"meta"`
	Actions []osAction `xml:"actions>action"`
	Common  []struct {
		ID string `xml:"id,attr"`
	} `xml:"actions>common-action"`
}

type osAction struct {
	ID      string `xml:"id,attr"`
	Name    string `xml:"name,attr"`
	Results []struct {
		Step string `xml:"step,attr"`
	} `xml:"results>unconditional-result"`
}

type osMeta struct {
	Name  string `xml:"name,attr"`
	Value string `xml:",chardata"`
}

// parseWorkflow reads an OSWorkflow descriptor. Steps are mapped to statuses
// through their jira.status.id meta attribute.
func parseWorkflow(name, descriptor string) (*workflow, error) {
	var doc osWorkflow
	dec := xml.NewDecoder(strings.NewReader(descriptor))
	dec.Strict = false
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("workflow %q: %w", name, err)
	}
	stepStatus := map[string]string{}
	w := &workflow{name: name}
	for _, s := range doc.Steps {
		for _, m := range s.Meta {
			if m.Name == "jira.status.id" {
				id := strings.TrimSpace(m.Value)
				stepStatus[s.ID] = id
				if !slices.Contains(w.statuses, id) {
					w.statuses = append(w.statuses, id)
				}
			}
		}
	}
	target := func(a osAction, from string) (string, bool) {
		for _, r := range a.Results {
			if r.Step == "-1" { // stays on the same step
				return from, from != ""
			}
			if st, ok := stepStatus[r.Step]; ok {
				return st, true
			}
		}
		return "", false
	}
	common := map[string]osAction{}
	for _, a := range doc.Common {
		common[a.ID] = a
	}
	seen := map[transition]bool{}
	add := func(t transition) {
		if t.from == t.to || seen[t] {
			return // self-loops change nothing in Solstice
		}
		seen[t] = true
		w.transitions = append(w.transitions, t)
	}
	for _, s := range doc.Steps {
		from := stepStatus[s.ID]
		if from == "" {
			continue
		}
		actions := slices.Clone(s.Actions)
		for _, c := range s.Common {
			if a, ok := common[c.ID]; ok {
				actions = append(actions, a)
			}
		}
		for _, a := range actions {
			if to, ok := target(a, from); ok {
				add(transition{name: a.Name, from: from, to: to})
			}
		}
	}
	for _, a := range doc.Global {
		if to, ok := target(a, ""); ok {
			add(transition{name: a.Name, to: to})
		}
	}
	for _, a := range doc.Initial {
		if to, ok := target(a, ""); ok {
			w.initial = to
			break
		}
	}
	if w.initial == "" && len(w.statuses) > 0 {
		w.initial = w.statuses[0]
	}
	if w.initial == "" {
		return nil, fmt.Errorf("workflow %q has no status", name)
	}
	return w, nil
}

// systemWorkflow is Jira's built-in workflow, named "jira". Jira never stores
// it in its database, so Solstice describes it itself: Open, In Progress,
// Resolved, Reopened and Closed, with the usual moves between them.
func systemWorkflow() *workflow {
	const open, progress, reopened, resolved, closed = "1", "3", "4", "5", "6"
	t := func(name, from, to string) transition { return transition{name: name, from: from, to: to} }
	return &workflow{
		name:     "jira",
		initial:  open,
		statuses: []string{open, progress, resolved, reopened, closed},
		transitions: []transition{
			t("Start progress", open, progress),
			t("Resolve", open, resolved),
			t("Close", open, closed),
			t("Stop progress", progress, open),
			t("Resolve", progress, resolved),
			t("Close", progress, closed),
			t("Close", resolved, closed),
			t("Reopen", resolved, reopened),
			t("Start progress", reopened, progress),
			t("Resolve", reopened, resolved),
			t("Close", reopened, closed),
			t("Reopen", closed, reopened),
		},
	}
}

// importWorkflows gives every project its own copy of the workflows its Jira
// workflow scheme uses. Issue types mapped to the scheme's default workflow
// use the project's default; the others are listed in workflow_issue_types.
func (r *run) importWorkflows() error {
	defs := map[string]*workflow{"jira": systemWorkflow()}
	broken := 0
	if err := r.each(`SELECT workflowname, descriptor FROM jiraworkflows`, func(rows *sql.Rows) error {
		var name, descriptor sql.NullString
		if err := rows.Scan(&name, &descriptor); err != nil {
			return err
		}
		w, err := parseWorkflow(name.String, descriptor.String)
		if err != nil {
			broken++
			r.opt.Log.Warn("workflow not imported", "err", err)
			return nil
		}
		defs[name.String] = w
		return nil
	}); err != nil {
		return err
	}

	type entry struct{ workflow, issueType string }
	entries := map[int64][]entry{}
	if err := r.each(`SELECT scheme, workflow, issuetype FROM workflowschemeentity`, func(rows *sql.Rows) error {
		var scheme sql.NullInt64
		var wf, it sql.NullString
		if err := rows.Scan(&scheme, &wf, &it); err != nil {
			return err
		}
		entries[scheme.Int64] = append(entries[scheme.Int64], entry{wf.String, it.String})
		return nil
	}); err != nil {
		return err
	}
	schemeOf, err := r.projectAssociations("WorkflowScheme")
	if err != nil {
		return err
	}

	var nextWorkflow, nextTransition int64
	wins := r.inserter("workflows", "id", "project_id", "name", "is_default", "initial_status_id")
	sins := r.inserter("workflow_statuses", "workflow_id", "status_id", "position")
	tins := r.inserter("workflow_transitions", "id", "workflow_id", "name", "from_status_id", "to_status_id", "position")
	mins := r.inserter("workflow_issue_types", "project_id", "issue_type_id", "workflow_id")
	missing := map[string]bool{}
	for _, pid := range r.projectIDs() {
		// Issue type → workflow name; "0" is the scheme's default.
		mapping := map[string]string{"0": "jira"}
		if scheme, ok := schemeOf[pid]; ok {
			for _, e := range entries[scheme] {
				mapping[e.issueType] = e.workflow
			}
		}
		created := map[string]int64{}
		create := func(name string, isDefault bool) (int64, error) {
			if id, ok := created[name]; ok {
				return id, nil
			}
			w, ok := defs[name]
			if !ok {
				missing[name] = true
				w = defs["jira"]
			}
			nextWorkflow++
			id := nextWorkflow
			created[name] = id
			if !r.statuses[w.initial] {
				return 0, fmt.Errorf("workflow %q starts in unknown status %s", name, w.initial)
			}
			if err := wins.add(id, pid, truncate(name, 255), isDefault, w.initial); err != nil {
				return 0, err
			}
			for i, st := range w.statuses {
				if r.statuses[st] {
					if err := sins.add(id, st, i); err != nil {
						return 0, err
					}
				}
			}
			for i, t := range w.transitions {
				if !r.statuses[t.to] || (t.from != "" && !r.statuses[t.from]) {
					continue
				}
				nextTransition++
				var from any
				if t.from != "" {
					from = t.from
				}
				if err := tins.add(nextTransition, id, truncate(t.name, 255), from, t.to, i); err != nil {
					return 0, err
				}
			}
			return id, nil
		}
		defaultID, err := create(mapping["0"], true)
		if err != nil {
			return err
		}
		types := make([]string, 0, len(mapping))
		for it := range mapping {
			types = append(types, it)
		}
		slices.Sort(types)
		for _, it := range types {
			if it == "0" || !r.issueTypes[it] {
				continue
			}
			id, err := create(mapping[it], false)
			if err != nil {
				return err
			}
			if id != defaultID {
				if err := mins.add(pid, it, id); err != nil {
					return err
				}
			}
		}
	}
	for _, in := range []*inserter{wins, sins, tins, mins} {
		if err := in.flush(); err != nil {
			return err
		}
	}
	r.rep.step("workflows (per project)", wins.total, broken)
	r.rep.step("transitions", tins.total, 0)
	for name := range missing {
		r.rep.warn("workflow \"%s\" not found in Jira: replaced by the standard workflow", name)
	}
	return nil
}
