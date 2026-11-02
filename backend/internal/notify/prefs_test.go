package notify

import (
	"testing"
	"time"
)

// Saving the preferences moves a waiting digest: a periodic one to the new
// interval after its first change, a daily one to the next digest hour.
func TestDigestDueOnSave(t *testing.T) {
	now := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	s := New(nil, nil, nil, time.UTC, nil, nil, nil)
	s.Now = func() time.Time { return now }
	first := now.Add(-20 * time.Minute)

	p := Defaults()
	p.Delivery, p.DigestInterval = Periodic, 30
	if got, want := s.digestDue(p, first), now.Add(10*time.Minute); !got.Equal(want) {
		t.Errorf("periodic, 30 min: %v, want %v", got, want)
	}
	p.DigestInterval = 15
	if got, want := s.digestDue(p, first), now.Add(-5*time.Minute); !got.Equal(want) {
		t.Errorf("periodic, shortened to 15 min: %v, want %v (past: sent at the next flush)", got, want)
	}
	p.DigestInterval = 120
	if got, want := s.digestDue(p, first), now.Add(100*time.Minute); !got.Equal(want) {
		t.Errorf("periodic, 2 h: %v, want %v", got, want)
	}

	p.Delivery, p.DigestHour = Digest, 8
	if got, want := s.digestDue(p, first), time.Date(2026, 11, 3, 8, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("periodic → digest at 8: %v, want %v", got, want)
	}
	p.DigestHour = 14
	if got, want := s.digestDue(p, first), time.Date(2026, 11, 2, 14, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("periodic → digest at 14: %v, want %v", got, want)
	}
}

func TestFormatMinutes(t *testing.T) {
	for _, c := range []struct {
		loc     string
		minutes int
		want    string
	}{{"en", 45, "45 min"}, {"en", 120, "2 h"}, {"fr", 90, "1 h 30 min"}, {"de", 90, "1 Std. 30 Min."}} {
		if got := formatMinutes(c.loc, c.minutes); got != c.want {
			t.Errorf("formatMinutes(%s, %d) = %q, want %q", c.loc, c.minutes, got, c.want)
		}
	}
}
