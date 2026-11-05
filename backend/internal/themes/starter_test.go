package themes

import (
	"archive/zip"
	"bytes"
	"io/fs"
	"maps"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func starterPackage(t *testing.T) *zip.Reader {
	t.Helper()
	data, err := Starter()
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	return pkg
}

func TestStarterPassesValidator(t *testing.T) {
	data, err := Starter()
	if err != nil {
		t.Fatal(err)
	}
	m, _, findings, err := openPackage(data, t.TempDir()+"/new")
	if err != nil || len(findings) > 0 {
		t.Fatalf("import: %v %+v", err, findings)
	}
	if m.ID != starterID {
		t.Errorf("manifest id %q", m.ID)
	}
}

func TestStarterDeclaresCarnetTokens(t *testing.T) {
	raw, err := fs.ReadFile(starterPackage(t), cssFile)
	if err != nil {
		t.Fatal(err)
	}
	starter := parseCSS(string(raw))
	carnet, err := fs.ReadFile(builtin, "builtin/"+FallbackID+"/"+cssFile)
	if err != nil {
		t.Fatal(err)
	}
	for _, dark := range []bool{false, true} {
		want, _, _, err := tokenBlock(parseCSS(string(carnet)), FallbackID, dark)
		if err != nil {
			t.Fatal(err)
		}
		got, _, ok, err := tokenBlock(starter, starterID, dark)
		if err != nil || !ok {
			t.Fatalf("dark %v: no block: %v", dark, err)
		}
		if a, b := slices.Sorted(maps.Keys(got)), slices.Sorted(maps.Keys(want)); !slices.Equal(a, b) {
			t.Errorf("dark %v: tokens\n%v\nwant\n%v", dark, a, b)
		}
	}
	if strings.Contains(string(raw), "Source Serif") || strings.Contains(string(raw), "IBM Plex Sans") {
		t.Error("a font token names a font the starter doesn't carry")
	}
}

func TestStarterCarriesAnatomy(t *testing.T) {
	got, err := fs.ReadFile(starterPackage(t), anatomyFile)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("ANATOMY.md")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Error("the starter's ANATOMY.md differs from this package's")
	}
}

func TestStarterHasEveryHookSection(t *testing.T) {
	raw, err := os.ReadFile("ANATOMY.md")
	if err != nil {
		t.Fatal(err)
	}
	_, hooks, _ := strings.Cut(string(raw), "\n## 3. Hooks\n")
	hooks, _, _ = strings.Cut(hooks, "\n## 4.")
	var want []string
	for _, m := range regexp.MustCompile(`(?m)^\*\*([^*]+)\*\*`).FindAllStringSubmatch(hooks, -1) {
		want = append(want, m[1])
	}
	if !slices.Equal(starterHooks, want) {
		t.Errorf("sections\n%q\nwant, from ANATOMY.md\n%q", starterHooks, want)
	}
}
