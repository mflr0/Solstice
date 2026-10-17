package tests

// The audit entries of the events added with the four new actions: issue
// deletion, CSV export, sign-in lockout and refused access tokens.

import (
	"net/url"
	"strings"
	"testing"
)

// Issue deletions are audited, from the interface and from the REST API
// (which goes through the same handler).
func TestIssueDeletionIsAudited(t *testing.T) {
	admin := login(t, "admin", "admin")
	key := createIssue(t, admin, "DEMO", "Suppression auditée", "")
	before, _ := latestAudit(t, "issue.delete")
	admin.call("DELETE", "/api/issues/"+key, nil, 204, nil)
	n, _ := latestAudit(t, "issue.delete")
	if n != before+1 {
		t.Fatalf("interface deletion: %d entries (was %d)", n, before)
	}
	if label := pick(t, `SELECT target_label FROM audit_log WHERE action = 'issue.delete' ORDER BY id DESC LIMIT 1`); label != key {
		t.Errorf("target label %q, want %s", label, key)
	}
	if who := pick(t, `SELECT COALESCE(username, '') FROM audit_log WHERE action = 'issue.delete' ORDER BY id DESC LIMIT 1`); who != "admin" {
		t.Errorf("author %q, want admin", who)
	}

	// The REST API goes through the interface's handler: one entry, once.
	rest := createIssue(t, admin, "DEMO", "Suppression auditée par l'API", "")
	if status, _, _ := call(t, nil, testURL, "DELETE", "/rest/api/2/issue/"+rest, nil, basic("admin", "admin")); status != 204 {
		t.Fatalf("REST deletion: %d", status)
	}
	if n2, _ := latestAudit(t, "issue.delete"); n2 != n+1 {
		t.Errorf("REST deletion: %d entries (was %d)", n2, n)
	}
}

// A CSV export is audited with the search, never with the exported rows.
func TestCSVExportIsAudited(t *testing.T) {
	paul := login(t, "paul.martin", "motdepasse")
	key := createIssue(t, paul, "DEMO", "Export audité", "")
	before, _ := latestAudit(t, "issue.export")
	query := "jql=" + url.QueryEscape("key = "+key)
	if status, _, _ := exportCSVFor(t, paul, query, ""); status != 200 {
		t.Fatalf("export: %d", status)
	}
	n, details := latestAudit(t, "issue.export")
	if n != before+1 || !strings.Contains(details, key) {
		t.Fatalf("export: %d entries (was %d), details %s", n, before, details)
	}
	if strings.Contains(details, "Export audité") {
		t.Errorf("the exported rows reached the audit log: %s", details)
	}
	if who := pick(t, `SELECT COALESCE(username, '') FROM audit_log WHERE action = 'issue.export' ORDER BY id DESC LIMIT 1`); who != "paul.martin" {
		t.Errorf("author %q, want paul.martin", who)
	}
	// A refused export (an unknown column) leaves no entry.
	bad, _ := latestAudit(t, "issue.export")
	if status, _, _ := exportCSVFor(t, paul, "columns=key,couleur", ""); status != 400 {
		t.Fatalf("unknown column: %d", status)
	}
	if n2, _ := latestAudit(t, "issue.export"); n2 != bad {
		t.Errorf("a refused export was recorded: %d entries (was %d)", n2, bad)
	}
}
