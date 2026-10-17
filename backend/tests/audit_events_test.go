package tests

// The audit entries of the events added with the four new actions: issue
// deletion, CSV export, sign-in lockout and refused access tokens.

import (
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
