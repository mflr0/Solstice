package jirafixture

import (
	"crypto/pbkdf2"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
)

// PKCS5S2 encodes a password the way Jira's internal directory stores it in
// cwd_user.credential: "{PKCS5S2}" followed by base64(salt ‖ key), where key
// is PBKDF2-HMAC-SHA1 over 10 000 iterations, 32 bytes long, and salt is 16
// bytes.
func PKCS5S2(password string, salt []byte) string {
	if len(salt) != 16 {
		panic(fmt.Sprintf("jirafixture: PKCS5S2 salt must be 16 bytes, got %d", len(salt)))
	}
	key, err := pbkdf2.Key(sha1.New, password, salt, 10000, 32)
	if err != nil {
		panic(err) // only possible for invalid parameters, which are fixed here
	}
	raw := make([]byte, 0, 48)
	raw = append(raw, salt...)
	raw = append(raw, key...)
	return "{PKCS5S2}" + base64.StdEncoding.EncodeToString(raw)
}
