package themes

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func storedManifest(t *testing.T, manifestID, name string) Manifest {
	t.Helper()
	m, err := ParseManifest(manifestWith(t, func(m map[string]any) {
		m["id"] = manifestID
		m["name"] = map[string]string{"en": name}
	}))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestMerged(t *testing.T) {
	builtin := testSet(t)
	idx := merged(builtin, nil, []storedTheme{
		{id: "zebra", manifest: storedManifest(t, "zebra", "Zebra"), enabled: true},
		{id: "off", manifest: storedManifest(t, "off", "Off"), enabled: false},
		{id: "retro-2", manifest: storedManifest(t, "retro", "apple"), enabled: true},
		{id: "same-b", manifest: storedManifest(t, "same", "Same"), enabled: true},
		{id: "same-a", manifest: storedManifest(t, "same", "Same"), enabled: true},
		{id: "fixture", manifest: storedManifest(t, "fixture", "Clash"), enabled: true},
	})
	want := []string{"fixture", "night-owl", "retro-2", "same-a", "same-b", "zebra"}
	if len(idx.Themes) != len(want) {
		t.Fatalf("themes %+v, want %v", idx.Themes, want)
	}
	for i, id := range want {
		if idx.Themes[i].ID != id {
			t.Errorf("theme %d is %q, want %q", i, idx.Themes[i].ID, id)
		}
	}
	if idx.Themes[0].Name["en"] == "Clash" {
		t.Error("a stored theme replaced the built-in of its id")
	}
	if len(builtin.Index().Themes) != 2 {
		t.Error("merging changed the built-in set")
	}
}

func storedRegistry(t *testing.T) *Registry {
	t.Helper()
	r := &Registry{builtin: testSet(t), dir: t.TempDir(), log: quietLog()}
	for name, data := range map[string]string{
		"owl-2/theme.json":                  validManifest,
		"owl-2/theme.css":                   "[data-skin='night-owl'] { --accent: #abcdef; }",
		"owl-2/assets/frame.svg":            "<svg/>",
		"owl-2/assets/folder.png/inner.png": "x",
		"owl-2/fonts/OFL.txt":               "licence",
		"owl-2/notes/secret.css":            "not served",
		"secret.css":                        "outside the theme's folder",
		"fixture/theme.css":                 "the stored copy",
	} {
		file := filepath.Join(r.dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(data), 0o640); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(r.dir, "secret.css"), filepath.Join(r.dir, "owl-2", "assets", "link.css")); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestServeStored(t *testing.T) {
	r := storedRegistry(t)
	serveOne := func(name string, setup func(*http.Request)) (bool, *httptest.ResponseRecorder) {
		t.Helper()
		req := httptest.NewRequest("GET", "/api/themes/owl-2/x", nil)
		if setup != nil {
			setup(req)
		}
		w := httptest.NewRecorder()
		ok, err := r.serveStored(w, req, "owl-2", name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return ok, w
	}

	ok, stored := serveOne("assets/frame.svg", nil)
	if !ok || stored.Code != 200 || stored.Body.String() != "<svg/>" {
		t.Fatalf("frame.svg: %v %d %q", ok, stored.Code, stored.Body)
	}
	_, builtin := serve(r.builtin, "night-owl", "assets/frame.svg", nil)
	for _, name := range []string{"Content-Type", "X-Content-Type-Options", "Content-Security-Policy", "Cache-Control"} {
		if got, want := stored.Header().Get(name), builtin.Header().Get(name); got != want || got == "" {
			t.Errorf("%s: %q, a built-in has %q", name, got, want)
		}
	}
	etag := stored.Header().Get("ETag")
	if ok, w := serveOne("assets/frame.svg", func(r *http.Request) { r.Header.Set("If-None-Match", etag) }); !ok || w.Code != http.StatusNotModified {
		t.Errorf("If-None-Match: %v %d", ok, w.Code)
	}
	for _, name := range []string{cssFile, "fonts/OFL.txt"} {
		if ok, w := serveOne(name, nil); !ok || w.Code != 200 {
			t.Errorf("%s: %v %d", name, ok, w.Code)
		}
	}

	for _, name := range []string{
		manifestFile,
		"notes/secret.css",
		"assets/missing.png",
		"assets/folder.png",
		"assets/frame.svg/inner.png",
		"assets/link.css",
		"../secret.css",
		"assets/../theme.css",
		"/theme.css",
		"",
	} {
		if ok, w := serveOne(name, nil); ok || w.Body.Len() != 0 {
			t.Errorf("%q: served %q", name, w.Body)
		}
	}
}

func TestRegistryServeBuiltinFirst(t *testing.T) {
	r := storedRegistry(t)
	w := httptest.NewRecorder()
	ok, err := r.Serve(w, httptest.NewRequest("GET", "/api/themes/fixture/theme.css", nil), "fixture", cssFile)
	if err != nil || !ok || w.Body.String() == "the stored copy" {
		t.Errorf("fixture: %v %v %q", ok, err, w.Body)
	}
}

func TestNewRegistryWithoutDatabase(t *testing.T) {
	r := NewRegistry(nil, t.TempDir(), nil)
	idx, err := r.Index(t.Context())
	if err != nil || len(idx.Themes) == 0 || idx.Themes[0].ID != "carnet" {
		t.Errorf("index %+v, %v", idx, err)
	}
	w := httptest.NewRecorder()
	if ok, err := r.Serve(w, httptest.NewRequest("GET", "/api/themes/owl-2/theme.css", nil), "owl-2", cssFile); ok || err != nil {
		t.Errorf("served a stored theme: %v %v", ok, err)
	}
}

func quietLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }
