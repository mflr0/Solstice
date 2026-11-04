package themes

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type zipEntry struct {
	name string
	data []byte
	mode fs.FileMode
}

func zipOf(t *testing.T, entries ...zipEntry) []byte {
	t.Helper()
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		if e.mode != 0 {
			h.SetMode(e.mode)
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(e.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func named(name string) zipEntry { return zipEntry{name: name, data: []byte("x")} }

func TestUnzipRefuses(t *testing.T) {
	var many []zipEntry
	for i := range MaxPackageFiles + 1 {
		many = append(many, named(fmt.Sprintf("assets/a%d.svg", i)))
	}
	zeros := func(n int) []byte { return make([]byte, n) }
	for _, c := range []struct {
		name    string
		entries []zipEntry
		file    string
		message string
	}{
		{"parent path", []zipEntry{named("../x")}, "../x", "relative path"},
		{"parent path inside", []zipEntry{named("assets/../../x.svg")}, "assets/../../x.svg", "relative path"},
		{"absolute path", []zipEntry{named("/etc/x")}, "/etc/x", "relative path"},
		{"backslash", []zipEntry{named(`assets\..\..\x.svg`)}, `assets\..\..\x.svg`, "relative path"},
		{"drive", []zipEntry{named("C:/x.svg")}, "C:/x.svg", "not allowed"},
		{"control character", []zipEntry{named("assets/a\x00.svg")}, "assets/a\x00.svg", "file name"},
		{"symlink", []zipEntry{{name: "assets/a.svg", data: []byte("/etc/passwd"), mode: fs.ModeSymlink | 0o777}}, "assets/a.svg", "links"},
		{"device", []zipEntry{{name: "assets/a.svg", mode: fs.ModeDevice | 0o600}}, "assets/a.svg", "regular"},
		{"html", []zipEntry{named("index.html")}, "index.html", "not allowed"},
		{"html in assets", []zipEntry{named("assets/a.html")}, "assets/a.html", "not allowed"},
		{"css in assets", []zipEntry{named("assets/a.css")}, "assets/a.css", "not allowed"},
		{"png in fonts", []zipEntry{named("fonts/a.png")}, "fonts/a.png", "not allowed"},
		{"old font format", []zipEntry{named("fonts/a.ttf")}, "fonts/a.ttf", "not allowed"},
		{"subfolder", []zipEntry{named("assets/sub/a.svg")}, "assets/sub/a.svg", "not allowed"},
		{"other folder", []zipEntry{{name: "notes/"}}, "notes/", "folders"},
		{"wrapping folder", []zipEntry{named("retro/theme.json")}, "retro/theme.json", "not allowed"},
		{"name twice", []zipEntry{named("assets/a.svg"), named("assets/a.svg")}, "assets/a.svg", "twice"},
		{"name twice, in another case", []zipEntry{named("assets/a.svg"), named("assets/A.svg")}, "assets/A.svg", "twice"},
		{"too many files", many, ".", "the most a package may hold"},
		{"zip bomb", []zipEntry{{name: "theme.css", data: zeros(MaxPackageBytes + 1)}}, ".", "bytes once unpacked"},
		{"over the size in total", []zipEntry{{name: "theme.css", data: zeros(MaxPackageBytes)}, {name: "theme.json", data: []byte("x")}}, ".", "bytes once unpacked"},
		{"compression ratio", []zipEntry{{name: "theme.css", data: zeros(ratioFloor + 1)}}, "theme.css", "compressed over"},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "new")
			findings, err := unzipPackage(zipOf(t, c.entries...), dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(findings) == 0 {
				t.Fatal("no finding")
			}
			f := findings[0]
			if f.Rule != "zip" || f.File != c.file || !strings.Contains(f.Message, c.message) {
				t.Errorf("got %+v, want %q on %q", f, c.message, c.file)
			}
		})
	}
}

func TestUnzipRefusesWhatIsNotAZip(t *testing.T) {
	findings, err := unzipPackage([]byte("not a zip"), filepath.Join(t.TempDir(), "new"))
	if err != nil || len(findings) != 1 || findings[0].Rule != "zip" {
		t.Fatalf("%v, %v", findings, err)
	}
}

// A zip's headers can lie: what counts is what comes out.
func TestUnzipRefusesAHeaderThatUnderstatesTheSize(t *testing.T) {
	var deflated bytes.Buffer
	fw, err := flate.NewWriter(&deflated, flate.BestCompression)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(make([]byte, MaxPackageBytes+1)); err != nil {
		t.Fatal(err)
	}
	if err := fw.Close(); err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	w, err := zw.CreateRaw(&zip.FileHeader{Name: "theme.css", Method: zip.Deflate,
		CompressedSize64: uint64(deflated.Len()), UncompressedSize64: 10})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(deflated.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	findings, err := unzipPackage(b.Bytes(), filepath.Join(t.TempDir(), "new"))
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].Rule != "zip" || findings[0].File != "theme.css" {
		t.Fatalf("findings %v", findings)
	}
}

func TestUnzipWritesAPackage(t *testing.T) {
	entries := []zipEntry{{name: "assets/"}, {name: "fonts/"}}
	for _, name := range []string{"theme.json", "theme.css", "LICENSE", "licence.txt", "assets/a.svg", "assets/a.png", "assets/a.webp", "assets/a.JPG", "assets/CREDITS.txt", "fonts/a.woff2", "fonts/a-OFL.txt"} {
		entries = append(entries, zipEntry{name: name, data: []byte("content of " + name)})
	}
	// Repetitive and small: the ratio doesn't count under its floor.
	entries = append(entries, zipEntry{name: "assets/flat.svg", data: make([]byte, ratioFloor)})
	dir := filepath.Join(t.TempDir(), "new")
	findings, err := unzipPackage(zipOf(t, entries...), dir)
	if err != nil || len(findings) > 0 {
		t.Fatalf("%v, %v", findings, err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.name, "/") {
			continue
		}
		got, err := os.ReadFile(filepath.Join(dir, e.name))
		if err != nil || !bytes.Equal(got, e.data) {
			t.Errorf("%s: %d bytes, %v", e.name, len(got), err)
		}
	}
}

// What #540 will export must come back in: every built-in fits the import's
// rules and passes the validator once unzipped.
func TestBuiltinPackagesCanBeImported(t *testing.T) {
	root, err := fs.Sub(builtin, "builtin")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range Builtin().list {
		t.Run(p.Manifest.ID, func(t *testing.T) {
			pkg, err := fs.Sub(root, p.Manifest.ID)
			if err != nil {
				t.Fatal(err)
			}
			var entries []zipEntry
			err = fs.WalkDir(pkg, ".", func(name string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return err
				}
				data, err := fs.ReadFile(pkg, name)
				entries = append(entries, zipEntry{name: name, data: data})
				return err
			})
			if err != nil {
				t.Fatal(err)
			}
			m, raw, findings, err := openPackage(zipOf(t, entries...), filepath.Join(t.TempDir(), "new"))
			if err != nil || len(findings) > 0 {
				t.Fatalf("%v, %v", findings, err)
			}
			if m.ID != p.Manifest.ID || raw == "" {
				t.Errorf("manifest %q, raw %d bytes", m.ID, len(raw))
			}
		})
	}
}

func TestFreeID(t *testing.T) {
	long := strings.Repeat("a", maxIDLen)
	for _, c := range []struct {
		want  string
		taken []string
		id    string
	}{
		{"retro", nil, "retro"},
		{"retro", []string{"retro"}, "retro-2"},
		{"retro", []string{"retro", "retro-2", "retro-3"}, "retro-4"},
		{"retro", []string{"retro-2"}, "retro"},
		{long, []string{long}, strings.Repeat("a", maxIDLen-2) + "-2"},
	} {
		got := freeID(c.want, func(id string) bool { return strings.Contains(" "+strings.Join(c.taken, " ")+" ", " "+id+" ") })
		if got != c.id {
			t.Errorf("freeID(%q, %v) = %q, want %q", c.want, c.taken, got, c.id)
		}
		if !idPattern.MatchString(got) {
			t.Errorf("%q is not a valid id", got)
		}
	}
}

func TestMergedLeavesOutDisabledBuiltin(t *testing.T) {
	idx := merged(testSet(t), []string{"fixture"}, nil)
	if len(idx.Themes) != 1 || idx.Themes[0].ID != "night-owl" {
		t.Fatalf("themes %+v", idx.Themes)
	}
}
