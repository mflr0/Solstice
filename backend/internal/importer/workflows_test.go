package importer

import (
	"reflect"
	"testing"
)

const descriptor = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE workflow PUBLIC "-//OpenSymphony Group//DTD OSWorkflow 2.8//EN" "http://www.opensymphony.com/osworkflow/workflow_2_8.dtd">
<workflow>
  <initial-actions>
    <action id="1" name="Create"><results><unconditional-result step="1" status="open"/></results></action>
  </initial-actions>
  <global-actions>
    <action id="9" name="Abandonner"><results><unconditional-result step="3" status="closed"/></results></action>
  </global-actions>
  <common-actions>
    <action id="5" name="Livrer"><results><unconditional-result step="3" status="closed"/></results></action>
  </common-actions>
  <steps>
    <step id="1" name="À faire">
      <meta name="jira.status.id">10000</meta>
      <actions>
        <common-action id="5"/>
        <action id="11" name="Commencer"><results><unconditional-result step="2" status="underway"/></results></action>
        <action id="12" name="Commenter"><results><unconditional-result step="-1" status="open"/></results></action>
      </actions>
    </step>
    <step id="2" name="En cours">
      <meta name="jira.status.id">3</meta>
      <actions>
        <common-action id="5"/>
      </actions>
    </step>
    <step id="3" name="Fini">
      <meta name="jira.status.id">10001</meta>
    </step>
  </steps>
</workflow>`

func TestParseWorkflow(t *testing.T) {
	w, err := parseWorkflow("Logiciel", descriptor)
	if err != nil {
		t.Fatal(err)
	}
	if w.initial != "10000" {
		t.Errorf("initial status %q", w.initial)
	}
	if !reflect.DeepEqual(w.statuses, []string{"10000", "3", "10001"}) {
		t.Errorf("statuses %v", w.statuses)
	}
	// A step's own actions come first, then the common ones it references.
	want := []transition{
		{"Commencer", "10000", "3"},
		{"Livrer", "10000", "10001"},
		{"Livrer", "3", "10001"},
		{"Abandonner", "", "10001"},
	}
	if !reflect.DeepEqual(w.transitions, want) {
		t.Errorf("transitions\n got %v\nwant %v", w.transitions, want)
	}
}

func TestParseWorkflowRejectsGarbage(t *testing.T) {
	if _, err := parseWorkflow("vide", "<workflow><steps/></workflow>"); err == nil {
		t.Error("a workflow without steps was accepted")
	}
	if _, err := parseWorkflow("cassé", "<workflow><steps>"); err == nil {
		t.Error("truncated XML was accepted")
	}
}

func TestSystemWorkflowIsConnected(t *testing.T) {
	w := systemWorkflow()
	reach := map[string]bool{w.initial: true}
	for changed := true; changed; {
		changed = false
		for _, tr := range w.transitions {
			if reach[tr.from] && !reach[tr.to] {
				reach[tr.to] = true
				changed = true
			}
		}
	}
	for _, s := range w.statuses {
		if !reach[s] {
			t.Errorf("status %s cannot be reached", s)
		}
	}
}
