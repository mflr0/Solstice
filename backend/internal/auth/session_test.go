package auth

import (
	"strings"
	"testing"
)

// Authenticate cannot be unit-tested without a database, so the dummy hash
// its timing equaliser relies on is pinned down here instead.
func TestDummyHashCostsArgon2id(t *testing.T) {
	if !strings.HasPrefix(dummyHash, "$argon2id$") {
		t.Fatalf("dummyHash must be an argon2id hash, got %q", dummyHash)
	}
	if ok, rehash := CheckPassword(dummyHash, "solstice-timing-equaliser"); !ok || rehash {
		t.Fatalf("dummy hash: ok=%v rehash=%v, want true false", ok, rehash)
	}
	if ok, _ := CheckPassword(dummyHash, "wrong password"); ok {
		t.Fatal("dummy hash accepted a wrong password")
	}
}
