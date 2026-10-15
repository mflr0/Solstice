// Package secret encrypts the secrets Solstice keeps in its database (SMTP
// password, webhook secrets…) so that a copy of the database alone does not
// reveal them.
//
// The key comes from SOLSTICE_SECRET_KEY (32 bytes, hex or base64, from the
// environment or the configuration file) when it is set, otherwise from
// <data dir>/secret.key, created on first use with mode 0600. Losing the key
// only loses the secrets: they must be typed again.
package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// KeyFile is the key's file name in the data directory.
const KeyFile = "secret.key"

// prefix marks sealed values, with the format version.
const prefix = "v1:"

// Box seals and opens secrets with AES-256-GCM.
type Box struct {
	aead cipher.AEAD
}

// New builds a box from a 32-byte key.
func New(key []byte) (*Box, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("secret key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead: aead}, nil
}

// Load returns the box of this installation: the configured key
// (SOLSTICE_SECRET_KEY) when not empty, otherwise the one in
// dataDir/secret.key, generated if missing.
func Load(dataDir, configured string) (*Box, error) {
	if v := strings.TrimSpace(configured); v != "" {
		key, err := decodeKey(v)
		if err != nil {
			return nil, fmt.Errorf("SOLSTICE_SECRET_KEY: %w", err)
		}
		return New(key)
	}
	key, err := keyFile(filepath.Join(dataDir, KeyFile))
	if err != nil {
		return nil, err
	}
	return New(key)
}

// decodeKey reads a 32-byte key written in hex or base64 (standard or URL
// alphabet, padded or not).
func decodeKey(s string) ([]byte, error) {
	if b, err := hex.DecodeString(s); err == nil && len(b) == 32 {
		return b, nil
	}
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding,
		base64.RawURLEncoding} {
		if b, err := enc.DecodeString(s); err == nil && len(b) == 32 {
			return b, nil
		}
	}
	return nil, errors.New("expected 32 bytes in hex (64 characters) or base64, e.g. from: openssl rand -base64 32")
}

// keyFile reads the key file, creating it if it does not exist yet. The file
// holds the key in base64, so it can be copied into SOLSTICE_SECRET_KEY.
func keyFile(path string) ([]byte, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- path built by the server, not from user input
	if err == nil {
		key, err := decodeKey(strings.TrimSpace(string(raw)))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		return key, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	// O_EXCL: two processes starting together must not each write their own key.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) // #nosec G304 -- path built by the server, not from user input
	if errors.Is(err, os.ErrExist) {
		return keyFile(path)
	}
	if err != nil {
		return nil, err
	}
	_, werr := f.WriteString(base64.StdEncoding.EncodeToString(key) + "\n")
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		os.Remove(path)
		return nil, werr
	}
	return key, nil
}

// Seal encrypts a secret. The label names what the secret is for (e.g.
// "smtp.password"): a sealed value only opens under the same label, so one
// cannot be swapped for another in the database.
func (b *Box) Seal(label string, plain []byte) (string, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	out := b.aead.Seal(nonce, nonce, plain, []byte(label))
	return prefix + base64.RawStdEncoding.EncodeToString(out), nil
}

// Open decrypts a value sealed under the same label.
func (b *Box) Open(label, sealed string) ([]byte, error) {
	body, ok := strings.CutPrefix(sealed, prefix)
	if !ok {
		return nil, errors.New("not a sealed secret")
	}
	raw, err := base64.RawStdEncoding.DecodeString(body)
	if err != nil {
		return nil, errors.New("sealed secret is corrupt")
	}
	n := b.aead.NonceSize()
	if len(raw) < n {
		return nil, errors.New("sealed secret is corrupt")
	}
	plain, err := b.aead.Open(nil, raw[:n], raw[n:], []byte(label))
	if err != nil {
		return nil, errors.New("cannot decrypt the secret: was the secret key changed?")
	}
	return plain, nil
}

// SealString and OpenString work on text secrets.
func (b *Box) SealString(label, plain string) (string, error) { return b.Seal(label, []byte(plain)) }

func (b *Box) OpenString(label, sealed string) (string, error) {
	plain, err := b.Open(label, sealed)
	return string(plain), err
}
