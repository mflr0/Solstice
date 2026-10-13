package jirafixture

import (
	"bytes"
	"testing"
)

// Expected values computed independently with Python:
// hashlib.pbkdf2_hmac('sha1', password, salt, 10000, 32).
func TestPKCS5S2MatchesReference(t *testing.T) {
	cases := []struct {
		password string
		salt     []byte
		want     string
	}{
		{"motdepasse", []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
			"{PKCS5S2}AAECAwQFBgcICQoLDA0OD6IfoWfdYy8Ou/how1xAaKoMw3dBsoA8UcASUqpVvNsM"},
		{"été", bytes.Repeat([]byte{0xa5}, 16),
			"{PKCS5S2}paWlpaWlpaWlpaWlpaWlpaSgH9ftaJMgJCZoT4H0h4yzjpi9lkBQUIqVpx6C2Ifr"},
	}
	for _, c := range cases {
		if got := PKCS5S2(c.password, c.salt); got != c.want {
			t.Errorf("PKCS5S2(%q) = %s, want %s", c.password, got, c.want)
		}
	}
}

func TestPKCS5S2RejectsShortSalt(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic for a 4-byte salt")
		}
	}()
	PKCS5S2("x", []byte{1, 2, 3, 4})
}
