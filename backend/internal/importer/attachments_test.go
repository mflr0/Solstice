package importer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSafePathPart(t *testing.T) {
	for _, tc := range []struct {
		in string
		ok bool
	}{
		{"ABC", true},
		{"AB_C2", true},
		{"../x", false},
		{"a/b", false},
		{`a\b`, false},
		{"..", false},
		{".", false},
		{"", false},
		{"a\x00b", false},
	} {
		if got := safePathPart(tc.in); got != tc.ok {
			t.Errorf("safePathPart(%q) = %v, want %v", tc.in, got, tc.ok)
		}
	}
}

func TestWithin(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "jira")
	for path, want := range map[string]bool{
		filepath.Join(root, "data", "a"):  true,
		filepath.Join(root, "..", "etc"):  false,
		filepath.Join(root, "..", "jirb"): false,
	} {
		if got := within(root, path); got != want {
			t.Errorf("within(%q, %q) = %v, want %v", root, path, got, want)
		}
	}
}

func TestCopyAttachmentSkipsAnUnsafeProjectKey(t *testing.T) {
	home, data := t.TempDir(), t.TempDir()
	// A file a "../" key would reach from data/attachments.
	outside := filepath.Join(home, "data", "x", "1")
	if err := os.MkdirAll(filepath.Dir(outside), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	r := &run{opt: Options{JiraHome: home}, rep: &Report{}, projects: map[int64]*project{1: {key: "../x", originalKey: "../x"}}}
	for range 2 {
		found, err := r.copyAttachment(1, issueRef{project: 1, num: 1}, data)
		if found || err != nil {
			t.Fatalf("found = %v, err = %v, want neither", found, err)
		}
	}
	if len(r.rep.Warnings) != 1 {
		t.Errorf("warnings = %q, want one", r.rep.Warnings)
	}
	if _, err := os.Stat(filepath.Join(data, "1")); !os.IsNotExist(err) {
		t.Errorf("the file was copied: %v", err)
	}
}

func TestCopyAttachmentCopiesAFileWithASafeKey(t *testing.T) {
	home, data := t.TempDir(), t.TempDir()
	src := filepath.Join(home, "data", "attachments", "ABC", "10000", "ABC-1", "7")
	if err := os.MkdirAll(filepath.Dir(src), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("hi"), 0o600); err != nil {
		t.Fatal(err)
	}
	r := &run{opt: Options{JiraHome: home}, rep: &Report{}, projects: map[int64]*project{1: {key: "ABC", originalKey: "ABC"}}}
	found, err := r.copyAttachment(7, issueRef{project: 1, num: 1}, data)
	if !found || err != nil {
		t.Fatalf("found = %v, err = %v", found, err)
	}
	if len(r.rep.Warnings) != 0 {
		t.Errorf("warnings = %q, want none", r.rep.Warnings)
	}
}
