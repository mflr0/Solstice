package auth

import (
	"strings"
	"testing"
)

// TestDummyHashCostsArgon2id guards the timing equaliser of Authenticate:
// refusing a password against an imported (non-Argon2id) hash must also
// spend an Argon2id verification, so the time a failed sign-in takes does not
// reveal which hash algorithm an account uses.
//
// Authenticate itself cannot be unit-tested without a database, and the
// database tests skip when SOLSTICE_TEST_DB is unset (no database is used
// in this environment): the dummy hash it relies on is pinned down here.
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
