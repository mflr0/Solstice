package themes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"testing/fstest"
)

const validManifest = `{
  "id": "night-owl",
  "contract": 1,
  "version": "1.2.3-beta.1+build.5",
  "name": {"en": "Night owl", "fr": "Chouette"},
  "description": {"en": "Dark and calm."},
  "author": "Someone",
  "schemes": ["light", "dark"],
  "preview": {
    "light": {"bar": "#112233", "paper": "#FFFFFF", "ink": "#000000", "accent": "#abcdef"},
    "dark": {"bar": "#000000", "paper": "#111111", "ink": "#eeeeee", "accent": "#ff8800"}
  }
}`

// manifestWith is validManifest with change applied to its decoded form.
func manifestWith(t *testing.T, change func(m map[string]any)) []byte {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(validManifest), &m); err != nil {
		t.Fatal(err)
	}
	change(m)
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestParseManifest(t *testing.T) {
	m, err := ParseManifest([]byte(validManifest))
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != "night-owl" || m.Name["fr"] != "Chouette" || m.Preview["dark"].Accent != "#ff8800" {
		t.Errorf("parsed %+v", m)
	}
	light, err := ParseManifest(manifestWith(t, func(m map[string]any) {
		m["schemes"] = []string{"light"}
		delete(m["preview"].(map[string]any), "dark")
	}))
	if err != nil || len(light.Schemes) != 1 {
		t.Errorf("light only: %v %v", light.Schemes, err)
	}
	about, err := ParseManifest(manifestWith(t, func(m map[string]any) {
		m["about"] = map[string]string{"en": "A barn owl at dusk.", "fr": "Une chouette au crépuscule."}
	}))
	if err != nil || about.About["fr"] != "Une chouette au crépuscule." {
		t.Errorf("with about: %v %v", about.About, err)
	}
	if m.About != nil {
		t.Errorf("without about: %v", m.About)
	}
}

func TestParseManifestRefuses(t *testing.T) {
	preview := func(scheme, key string, v any) func(map[string]any) {
		return func(m map[string]any) { m["preview"].(map[string]any)[scheme].(map[string]any)[key] = v }
	}
	set := func(key string, v any) func(map[string]any) {
		return func(m map[string]any) { m[key] = v }
	}
	cases := map[string]func(map[string]any){
		"id missing":        func(m map[string]any) { delete(m, "id") },
		"id upper case":     set("id", "Night"),
		"id with a dot":     set("id", "night.owl"),
		"id too long":       set("id", "abcdefghijklmnopqrstuvwxyz0123456"),
		"contract 0":        set("contract", 0),
		"contract too new":  set("contract", Contract+1),
		"contract fraction": set("contract", 1.5),
		"contract a string": set("contract", "1"),
		"version two parts": set("version", "1.2"),
		"version with v":    set("version", "v1.2.3"),
		"version leading 0": set("version", "01.2.3"),
		"name without en":   set("name", map[string]string{"fr": "Chouette"}),
		"name empty en":     set("name", map[string]string{"en": ""}),
		"name blank text":   set("name", map[string]string{"en": "Owl", "fr": " "}),
		"name a string":     set("name", "Owl"),
		"description no en": set("description", map[string]string{"de": "Eule"}),
		"about without en":  set("about", map[string]string{"fr": "Une chouette."}),
		"about empty":       set("about", map[string]string{}),
		"about blank text":  set("about", map[string]string{"en": "An owl.", "de": "\t"}),
		"author missing":    func(m map[string]any) { delete(m, "author") },
		"author blank":      set("author", "  "),
		"schemes dark only": set("schemes", []string{"dark"}),
		"schemes reversed":  set("schemes", []string{"dark", "light"}),
		"schemes unknown":   set("schemes", []string{"light", "sepia"}),
		"schemes empty":     set("schemes", []string{}),
		"preview missing":   func(m map[string]any) { delete(m, "preview") },
		"preview no dark":   func(m map[string]any) { delete(m["preview"].(map[string]any), "dark") },
		"preview extra": func(m map[string]any) {
			m["schemes"] = []string{"light"}
		},
		"preview short colour": preview("light", "bar", "#123"),
		"preview named colour": preview("dark", "paper", "white"),
		"preview no hash":      preview("light", "ink", "112233"),
		"preview no accent": func(m map[string]any) {
			delete(m["preview"].(map[string]any)["light"].(map[string]any), "accent")
		},
		"unknown field": set("homepage", "https://example.org"),
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			if m, err := ParseManifest(manifestWith(t, change)); err == nil {
				t.Errorf("accepted %+v", m)
			}
		})
	}
	if _, err := ParseManifest([]byte(validManifest + `{}`)); err == nil {
		t.Error("accepted data after the manifest")
	}
}

func testSet(t *testing.T) *Set {
	t.Helper()
	s, err := Load(fstest.MapFS{
		"night-owl/theme.json":         {Data: []byte(validManifest)},
		"night-owl/theme.css":          {Data: []byte("[data-skin='night-owl'] { --accent: #abcdef; }")},
		"night-owl/assets/frame.svg":   {Data: []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)},
		"night-owl/assets/paper.png":   {Data: []byte("\x89PNG")},
		"night-owl/fonts/Owl.woff2":    {Data: []byte("wOF2")},
		"night-owl/fonts/OFL.txt":      {Data: []byte("SIL Open Font License")},
		"night-owl/notes/secret.css":   {Data: []byte("not served")},
		"night-owl/README.md":          {Data: []byte("not served")},
		"fixture/theme.json":           {Data: manifestWith(t, func(m map[string]any) { m["id"] = "fixture" })},
		"fixture/theme.css":            {Data: []byte("")},
		"fixture/assets/nested/a.webp": {Data: []byte("RIFF")},
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestIndex(t *testing.T) {
	idx := testSet(t).Index()
	if idx.Contract != Contract || len(idx.Themes) != 2 || idx.Themes[0].ID != "fixture" || idx.Themes[1].ID != "night-owl" {
		t.Errorf("index %+v", idx)
	}
}

func serve(s *Set, id, name string, setup func(*http.Request)) (bool, *httptest.ResponseRecorder) {
	r := httptest.NewRequest("GET", "/api/themes/"+id+"/"+name, nil)
	if setup != nil {
		setup(r)
	}
	w := httptest.NewRecorder()
	return s.Serve(w, r, id, name), w
}

func TestServe(t *testing.T) {
	s := testSet(t)
	for name, contentType := range map[string]string{
		"theme.css":        "text/css; charset=utf-8",
		"assets/frame.svg": "image/svg+xml",
		"assets/paper.png": "image/png",
		"fonts/Owl.woff2":  "font/woff2",
		"fonts/OFL.txt":    "text/plain; charset=utf-8",
	} {
		ok, w := serve(s, "night-owl", name, nil)
		h := w.Header()
		if !ok || w.Code != http.StatusOK {
			t.Errorf("%s: served %v, status %d", name, ok, w.Code)
			continue
		}
		if got := h.Get("Content-Type"); got != contentType {
			t.Errorf("%s: Content-Type %q, want %q", name, got, contentType)
		}
		if got := h.Get("Content-Security-Policy"); got != "default-src 'none'; style-src 'unsafe-inline'" {
			t.Errorf("%s: Content-Security-Policy %q", name, got)
		}
		if h.Get("X-Content-Type-Options") != "nosniff" || h.Get("ETag") == "" || h.Get("Cache-Control") == "" {
			t.Errorf("%s: headers %v", name, h)
		}
	}
	if ok, w := serve(s, "fixture", "assets/nested/a.webp", nil); !ok || w.Header().Get("Content-Type") != "image/webp" {
		t.Errorf("nested asset: %v %v", ok, w.Header())
	}
}

func TestServeETag(t *testing.T) {
	s := testSet(t)
	_, first := serve(s, "night-owl", "theme.css", nil)
	etag := first.Header().Get("ETag")
	_, again := serve(s, "night-owl", "theme.css", func(r *http.Request) { r.Header.Set("If-None-Match", etag) })
	if again.Code != http.StatusNotModified || again.Body.Len() != 0 {
		t.Errorf("If-None-Match: status %d, %d bytes", again.Code, again.Body.Len())
	}
	if _, other := serve(s, "night-owl", "assets/frame.svg", nil); other.Header().Get("ETag") == etag {
		t.Error("two files share an ETag")
	}
}

func TestServeRefuses(t *testing.T) {
	s := testSet(t)
	for _, c := range []struct{ id, name string }{
		{"nobody", "theme.css"},
		{"", "theme.css"},
		{"night-owl", "theme.json"},
		{"night-owl", "README.md"},
		{"night-owl", "notes/secret.css"},
		{"night-owl", "assets/missing.svg"},
		{"night-owl", "assets"},
		{"night-owl", ""},
		{"night-owl", "../fixture/theme.css"},
		{"night-owl", "assets/../theme.json"},
		{"night-owl", "assets/../../fixture/theme.css"},
		{"night-owl", "./theme.css"},
		{"night-owl", "/theme.css"},
		{"fixture", "../night-owl/fonts/Owl.woff2"},
		{"..", "night-owl/theme.css"},
	} {
		if ok, w := serve(s, c.id, c.name, nil); ok || w.Body.Len() != 0 || len(w.Header()) != 0 {
			t.Errorf("%s/%s: served %v, %v", c.id, c.name, ok, w.Header())
		}
	}
}

func TestLoadRefuses(t *testing.T) {
	cases := map[string]fstest.MapFS{
		"id is not the folder": {
			"other/theme.json": {Data: []byte(validManifest)},
			"other/theme.css":  {Data: nil},
		},
		"no theme.css": {
			"night-owl/theme.json": {Data: []byte(validManifest)},
		},
		"no theme.json": {
			"night-owl/theme.css": {Data: nil},
		},
		"invalid manifest": {
			"night-owl/theme.json": {Data: []byte(`{"id": "night-owl"}`)},
			"night-owl/theme.css":  {Data: nil},
		},
		"unknown file type": {
			"night-owl/theme.json":      {Data: []byte(validManifest)},
			"night-owl/theme.css":       {Data: nil},
			"night-owl/assets/run.html": {Data: []byte("<script>")},
		},
	}
	for name, root := range cases {
		if _, err := Load(root); err == nil {
			t.Errorf("%s: loaded", name)
		}
	}
}

func TestSortBy(t *testing.T) {
	s := testSet(t)
	if err := s.sortBy([]string{"night-owl", "fixture"}); err != nil {
		t.Fatal(err)
	}
	if idx := s.Index(); idx.Themes[0].ID != "night-owl" || idx.Themes[1].ID != "fixture" {
		t.Errorf("sorted %+v", idx.Themes)
	}
	for _, ids := range [][]string{
		{"night-owl"},
		{"night-owl", "fixture", "other"},
		{"night-owl", "other"},
		{"night-owl", "night-owl"},
	} {
		if err := testSet(t).sortBy(ids); err == nil {
			t.Errorf("%q: sorted", ids)
		}
	}
}

func TestBuiltin(t *testing.T) {
	var got []string
	for _, m := range Builtin().Index().Themes {
		got = append(got, m.ID)
		if m.About["en"] == "" {
			t.Errorf("%s: no English about", m.ID)
		}
	}
	want := []string{"carnet", "orbite", "crayon", "luna", "retro", "classique", "terminal", "plan"}
	if !slices.Equal(got, want) {
		t.Errorf("built-in themes %q, want %q", got, want)
	}
}
