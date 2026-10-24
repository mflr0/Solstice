package notify

import (
	"testing"

	"solstice/internal/events"
	"solstice/internal/mail"
)

// eligible owns the eligibility rule: an e-mail goes only to an active user
// with a usable address.
func TestEligible(t *testing.T) {
	for _, c := range []struct {
		name string
		u    *events.User
		want bool
	}{
		{"nobody", nil, false},
		{"inactive", &events.User{Active: false, Email: "paul.martin@exemple.fr"}, false},
		{"no address", &events.User{Active: true}, false},
		{"not an address", &events.User{Active: true, Email: "Paul Martin"}, false},
		{"active with address", &events.User{Active: true, Email: "paul.martin@exemple.fr"}, true},
	} {
		if got := eligible(c.u); got != c.want {
			t.Errorf("%s: eligible() = %v, want %v", c.name, got, c.want)
		}
	}
}

// A batch showing several issues (a bulk change) is titled by its count and
// threads under none; one showing a single issue keeps that issue's thread.
func TestComposeBulk(t *testing.T) {
	s := &Service{BaseURL: func() string { return "" }}
	cfg := mail.Config{From: "solstice@exemple.fr"}
	rc := &recipient{user: &events.User{ID: 7, Email: "hugo.robert@exemple.fr"}, locale: "en"}
	facts := batchFacts{reasons: []string{reasonWatcher}, threadIssue: 12, actor: "Paul Martin"}
	sections := []section{{Key: "WEB-1", Summary: "Un"}, {Key: "WEB-2", Summary: "Deux"}, {Key: "WEB-3", Summary: "Trois"}}

	msg := s.compose(cfg, rc, false, &page{Lang: "en", Sections: sections}, facts)
	if msg.Subject != "Paul Martin changed 3 issues" || msg.InReplyTo != "" {
		t.Errorf("several issues: subject %q, In-Reply-To %q", msg.Subject, msg.InReplyTo)
	}
	msg = s.compose(cfg, rc, false, &page{Lang: "en", Sections: sections[:1]}, facts)
	if msg.Subject != "[WEB-1] Un" || msg.InReplyTo != "<issue.12@exemple.fr>" {
		t.Errorf("one issue: subject %q, In-Reply-To %q", msg.Subject, msg.InReplyTo)
	}
}
