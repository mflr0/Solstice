package auth

import "testing"

// Computed independently with Python's hashlib.pbkdf2_hmac('sha1', b'motdepasse',
// bytes(range(16)), 10000, 32), salt and key concatenated and base64-encoded.
const jiraHash = "{PKCS5S2}AAECAwQFBgcICQoLDA0OD6IfoWfdYy8Ou/how1xAaKoMw3dBsoA8UcASUqpVvNsM"

func TestJiraPasswordHash(t *testing.T) {
	ok, rehash := CheckPassword(jiraHash, "motdepasse")
	if !ok || !rehash {
		t.Fatalf("Jira hash: ok=%v rehash=%v, want true true", ok, rehash)
	}
	if ok, _ := CheckPassword(jiraHash, "Motdepasse"); ok {
		t.Fatal("Jira hash accepted a wrong password")
	}
}

func TestArgon2RoundTrip(t *testing.T) {
	hash, err := HashPassword("cheval correct pile agrafe")
	if err != nil {
		t.Fatal(err)
	}
	if ok, rehash := CheckPassword(hash, "cheval correct pile agrafe"); !ok || rehash {
		t.Fatalf("argon2 hash: ok=%v rehash=%v, want true false", ok, rehash)
	}
	if ok, _ := CheckPassword(hash, "cheval correct pile"); ok {
		t.Fatal("argon2 hash accepted a wrong password")
	}
}

func TestUnusableHashes(t *testing.T) {
	for _, stored := range []string{"", "nopass", "{PKCS5S2}", "{PKCS5S2}not-base64!", "$argon2id$broken"} {
		if ok, _ := CheckPassword(stored, ""); ok {
			t.Errorf("%q accepted an empty password", stored)
		}
	}
}
