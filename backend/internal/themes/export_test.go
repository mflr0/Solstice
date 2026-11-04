package themes

import (
	"bytes"
	"io/fs"
	"os"
	"testing"
	"testing/fstest"
)

func TestExportBuiltinRoundTrip(t *testing.T) {
	r := NewRegistry(nil, t.TempDir(), nil)
	root, err := fs.Sub(builtin, "builtin")
	if err != nil {
		t.Fatal(err)
	}
	for id := range r.builtin.byID {
		t.Run(id, func(t *testing.T) {
			name, data, err := r.Export(t.Context(), id, false)
			if err != nil {
				t.Fatal(err)
			}
			if want := id + "-" + r.builtin.byID[id].Manifest.Version + ".zip"; name != want {
				t.Errorf("name %q, want %q", name, want)
			}
			if len(data) > MaxZipBytes {
				t.Errorf("zip of %d bytes: an import takes %d", len(data), MaxZipBytes)
			}
			dir := t.TempDir() + "/new"
			m, _, findings, err := openPackage(data, dir)
			if err != nil || len(findings) > 0 {
				t.Fatalf("import: %v %+v", err, findings)
			}
			if m.ID != id {
				t.Errorf("manifest id %q", m.ID)
			}
			original, err := fs.Sub(root, id)
			if err != nil {
				t.Fatal(err)
			}
			if a, b := filesOf(t, original), filesOf(t, os.DirFS(dir)); !sameFiles(a, b) {
				t.Errorf("files differ: %d exported of %d", len(b), len(a))
			}
		})
	}
}

// A big file that compresses well must still pass the import's ratio check.
func TestZipPackageImportableWhenRepetitive(t *testing.T) {
	big := bytes.Repeat([]byte("a"), 2*ratioFloor)
	data, err := zipPackage(fstest.MapFS{"assets/big.svg": {Data: big}})
	if err != nil {
		t.Fatal(err)
	}
	findings, err := unzipPackage(data, t.TempDir()+"/new")
	if err != nil || len(findings) > 0 {
		t.Errorf("unzip: %v %+v", err, findings)
	}
}

func TestZipPackageSameBytes(t *testing.T) {
	pkg := fstest.MapFS{"theme.css": {Data: []byte("a")}, "assets/a.svg": {Data: []byte("b")}}
	first, err := zipPackage(pkg)
	if err != nil {
		t.Fatal(err)
	}
	second, err := zipPackage(pkg)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Error("two exports of one package differ")
	}
}

func filesOf(t *testing.T, pkg fs.FS) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	err := fs.WalkDir(pkg, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		out[name], err = fs.ReadFile(pkg, name)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func sameFiles(a, b map[string][]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for name, data := range a {
		if other, ok := b[name]; !ok || !bytes.Equal(data, other) {
			return false
		}
	}
	return true
}
