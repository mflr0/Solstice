package auth

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"testing"
	"time"
)

func TestAccountTokensAreLongAndDistinct(t *testing.T) {
	seen := map[string]bool{}
	for range 200 {
		tok, err := NewAccountTokenValue()
		if err != nil {
			t.Fatal(err)
		}
		raw, err := base64.RawURLEncoding.DecodeString(tok)
		if err != nil {
			t.Fatalf("token %q is not URL-safe base64: %v", tok, err)
		}
		if len(raw) < 32 {
			t.Fatalf("token of %d bytes, want at least 32", len(raw))
		}
		if seen[tok] {
			t.Fatalf("token %q issued twice", tok)
		}
		seen[tok] = true
	}
}

func TestAccountTokenHash(t *testing.T) {
	tok := "a token"
	sum := sha256.Sum256([]byte(tok))
	if got := AccountTokenHash(tok); got != hex.EncodeToString(sum[:]) || len(got) != 64 {
		t.Fatalf("hash %q is not the SHA-256 of the token in hexadecimal", got)
	}
	if AccountTokenHash(tok) != AccountTokenHash(tok) {
		t.Fatal("the hash is not stable")
	}
	if AccountTokenHash(tok) == AccountTokenHash(tok+" ") {
		t.Fatal("two tokens share a hash")
	}
	if AccountTokenHash(tok) == tok {
		t.Fatal("the hash is the token")
	}
}

func TestAccountTokenLifetimes(t *testing.T) {
	if got := Lifetime(PurposeReset); got != time.Hour {
		t.Errorf("reset link lasts %v, want 1h", got)
	}
	if got := Lifetime(PurposeInvite); got != 7*24*time.Hour {
		t.Errorf("invitation lasts %v, want 7 days", got)
	}
	if got := Lifetime("anything else"); got != 0 {
		t.Errorf("unknown purpose lasts %v, want 0", got)
	}
}

func TestAccountTokenUsable(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	unused := sql.NullTime{}
	used := sql.NullTime{Time: now.Add(-time.Minute), Valid: true}
	for _, tc := range []struct {
		name    string
		expires time.Time
		used    sql.NullTime
		want    bool
	}{
		{"fresh", now.Add(time.Minute), unused, true},
		{"expires now", now, unused, false},
		{"expired", now.Add(-time.Second), unused, false},
		{"used", now.Add(time.Hour), used, false},
		{"used and expired", now.Add(-time.Hour), used, false},
	} {
		if got := Usable(tc.expires, tc.used, now); got != tc.want {
			t.Errorf("%s: %v, want %v", tc.name, got, tc.want)
		}
	}
}
