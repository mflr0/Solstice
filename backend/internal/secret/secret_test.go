package secret

import (
	"encoding/base64"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	box, err := New(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := box.SealString("smtp.password", "s3cret !é")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sealed, "s3cret") || !strings.HasPrefix(sealed, "v1:") {
		t.Fatalf("sealed value looks wrong: %q", sealed)
	}
	again, _ := box.SealString("smtp.password", "s3cret !é")
	if again == sealed {
		t.Error("two seals of the same secret are identical: the nonce is not random")
	}
	plain, err := box.OpenString("smtp.password", sealed)
	if err != nil || plain != "s3cret !é" {
		t.Fatalf("open: %q, %v", plain, err)
	}
	if _, err := box.OpenString("webhook.secret", sealed); err == nil {
		t.Error("a secret opened under another label")
	}
	tampered := sealed[:len(sealed)-2] + "AA"
	if _, err := box.OpenString("smtp.password", tampered); err == nil {
		t.Error("a tampered secret opened")
	}
	other, _ := New(append(make([]byte, 31), 1))
	if _, err := other.OpenString("smtp.password", sealed); err == nil {
		t.Error("a secret opened with another key")
	}
	if _, err := box.OpenString("smtp.password", "plain text"); err == nil {
		t.Error("an unsealed value opened")
	}
}

func TestKeyFile(t *testing.T) {
	dir := t.TempDir()
	box, err := Load(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, KeyFile))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("key file mode %v, want 0600", info.Mode().Perm())
	}
	sealed, _ := box.SealString("x", "valeur")
	// The same directory gives the same key.
	again, err := Load(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if v, err := again.OpenString("x", sealed); err != nil || v != "valeur" {
		t.Fatalf("reloaded key cannot open: %q %v", v, err)
	}
}

func TestConfiguredKey(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	dir := t.TempDir()
	for _, v := range []string{hex.EncodeToString(key), base64.StdEncoding.EncodeToString(key),
		base64.RawURLEncoding.EncodeToString(key)} {
		box, err := Load(dir, v)
		if err != nil {
			t.Fatalf("%q: %v", v, err)
		}
		ref, _ := New(key)
		sealed, _ := ref.SealString("x", "ok")
		if got, err := box.OpenString("x", sealed); err != nil || got != "ok" {
			t.Errorf("%q: key not used: %v", v, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, KeyFile)); !os.IsNotExist(err) {
		t.Error("a key file was written although a key is configured")
	}
	if _, err := Load(dir, "trop court"); err == nil {
		t.Error("a malformed key was accepted")
	}
}
