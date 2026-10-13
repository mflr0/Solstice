// Package auth handles passwords and sign-in sessions.
package auth

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters (OWASP's baseline: 19 MiB, 2 passes, 1 lane).
const (
	argonMemory  = 19 * 1024
	argonTime    = 2
	argonThreads = 1
	argonKeyLen  = 32
)

// HashPassword returns an Argon2id hash in PHC string format.
func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	enc := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads, enc.EncodeToString(salt), enc.EncodeToString(key)), nil
}

// CheckPassword verifies a password against a stored hash: Solstice's Argon2id
// or Jira's "{PKCS5S2}" (PBKDF2-HMAC-SHA1, 10 000 iterations, 16-byte salt
// followed by the 32-byte key, base64). rehash is true when the hash should
// be replaced by a fresh Argon2id one after a successful check.
func CheckPassword(stored, password string) (ok, rehash bool) {
	switch {
	case strings.HasPrefix(stored, "$argon2id$"):
		return checkArgon2(stored, password), false
	case strings.HasPrefix(stored, "{PKCS5S2}"):
		ok := checkPKCS5S2(strings.TrimPrefix(stored, "{PKCS5S2}"), password)
		return ok, ok
	}
	return false, false
}

func checkArgon2(stored, password string) bool {
	parts := strings.Split(stored, "$")
	if len(parts) != 6 {
		return false
	}
	var version int
	var memory uint32
	var passes uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &passes, &threads); err != nil {
		return false
	}
	enc := base64.RawStdEncoding
	salt, err := enc.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := enc.DecodeString(parts[5])
	if err != nil || len(want) == 0 {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, passes, memory, threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

func checkPKCS5S2(encoded, password string) bool {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(raw) != 48 {
		return false
	}
	salt, want := raw[:16], raw[16:]
	got, err := pbkdf2.Key(sha1.New, password, salt, 10000, 32)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}
