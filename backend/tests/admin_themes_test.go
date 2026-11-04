package tests

// Theme packages imported by administrators: refused, listed, served, replaced,
// turned on and off, deleted.

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"testing"

	"solstice/internal/jirafixture"
)

type themeEntry struct {
	ID         string `json:"id"`
	ManifestID string `json:"manifestId"`
	Version    string `json:"version"`
	Origin     string `json:"origin"`
	Enabled    bool   `json:"enabled"`
	ImportedBy string `json:"importedBy"`
}

// themeZip is the built-in Carnet as a package of its own, under id and
// version. change, if any, edits its stylesheet.
func themeZip(t *testing.T, id, version string, change func(css string) string) []byte {
	t.Helper()
	root := os.DirFS("../internal/themes/builtin/carnet")
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	err := fs.WalkDir(root, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(root, name)
		if err != nil {
			return err
		}
		switch name {
		case "theme.css":
			css := strings.ReplaceAll(string(data), "[data-skin='carnet']", "[data-skin='"+id+"']")
			if change != nil {
				css = change(css)
			}
			data = []byte(css)
		case "theme.json":
			var m map[string]any
			if err := json.Unmarshal(data, &m); err != nil {
				return err
			}
			m["id"], m["version"] = id, version
			if data, err = json.Marshal(m); err != nil {
				return err
			}
		}
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func sendTheme(t *testing.T, c *client, method, path string, data []byte) (int, []byte) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "theme.zip")
	fw.Write(data)
	mw.Close()
	req, _ := http.NewRequest(method, c.base+path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set(csrfHeader, "1")
	resp, err := c.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, out
}

// importTheme imports a package, and deletes it when the test ends.
func importTheme(t *testing.T, admin *client, data []byte) themeEntry {
	t.Helper()
	status, raw := sendTheme(t, admin, "POST", "/api/admin/themes", data)
	if status != 201 {
		t.Fatalf("import: %d\n%s", status, raw)
	}
	var e themeEntry
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.do("DELETE", "/api/admin/themes/"+e.ID, nil) })
	return e
}

func publicThemeIDs(t *testing.T) []string {
	t.Helper()
	var idx struct {
		Themes []struct{ ID string }
	}
	anonymous(t).call("GET", "/api/themes", nil, 200, &idx)
	ids := make([]string, len(idx.Themes))
	for i, m := range idx.Themes {
		ids[i] = m.ID
	}
	return ids
}

func themeListed(t *testing.T, id string) bool {
	t.Helper()
	for _, have := range publicThemeIDs(t) {
		if have == id {
			return true
		}
	}
	return false
}

func themeServes(t *testing.T, id string) (int, string) {
	t.Helper()
	status, raw := anonymous(t).do("GET", "/api/themes/"+id+"/theme.css", nil)
	return status, string(raw)
}

func adminTheme(t *testing.T, admin *client, id string) (themeEntry, bool) {
	t.Helper()
	var out struct{ Themes []themeEntry }
	admin.call("GET", "/api/admin/themes", nil, 200, &out)
	for _, e := range out.Themes {
		if e.ID == id {
			return e, true
		}
	}
	return themeEntry{}, false
}

func TestThemeImportListedAndServed(t *testing.T) {
	admin := login(t, "admin", "admin")
	id := unique("chk")
	before, _ := latestAudit(t, "theme.import")

	e := importTheme(t, admin, themeZip(t, id, "1.0.0", nil))
	if e.ID != id || e.ManifestID != id || e.Origin != "imported" || !e.Enabled || e.ImportedBy != "admin" {
		t.Errorf("imported %+v", e)
	}
	if !themeListed(t, id) {
		t.Errorf("%s is not among %v", id, publicThemeIDs(t))
	}
	status, css := themeServes(t, id)
	if status != 200 || !strings.Contains(css, "[data-skin='"+id+"']") {
		t.Errorf("theme.css: %d", status)
	}
	if status, _ := anonymous(t).do("GET", "/api/themes/"+id+"/theme.json", nil); status != 404 {
		t.Errorf("theme.json: %d, want 404", status)
	}
	if n, details := latestAudit(t, "theme.import"); n != before+1 || !strings.Contains(details, "1.0.0") {
		t.Errorf("audit: %d entries (was %d), details %s", n, before, details)
	}
}

func TestThemeImportTakesAFreeID(t *testing.T) {
	admin := login(t, "admin", "admin")
	id := unique("chk")
	importTheme(t, admin, themeZip(t, id, "1.0.0", nil))

	second := importTheme(t, admin, themeZip(t, id, "1.0.0", nil))
	if second.ID != id+"-2" || second.ManifestID != id {
		t.Errorf("second import %+v", second)
	}
	if status, css := themeServes(t, second.ID); status != 200 || !strings.Contains(css, "[data-skin='"+id+"']") {
		t.Errorf("%s: %d: the file should keep the anchor of its package", second.ID, status)
	}

	// A built-in's id is taken too.
	builtin := importTheme(t, admin, themeZip(t, "carnet", "1.0.0", nil))
	if builtin.ID != "carnet-2" || builtin.ManifestID != "carnet" {
		t.Errorf("import of a built-in's id %+v", builtin)
	}
}

func TestThemeImportRefused(t *testing.T) {
	admin := login(t, "admin", "admin")
	id := unique("chk")
	before, _ := latestAudit(t, "theme.import")

	broken := themeZip(t, id, "1.0.0", func(css string) string { return css + "\n@import 'https://example.com/a.css';\n" })
	status, raw := sendTheme(t, admin, "POST", "/api/admin/themes", broken)
	var answer struct {
		Error    string
		Findings []struct {
			Rule, Message, File string
			Line                int
		}
	}
	if err := json.Unmarshal(raw, &answer); err != nil {
		t.Fatal(err)
	}
	if status != 400 || answer.Error == "" || len(answer.Findings) == 0 {
		t.Fatalf("%d\n%s", status, raw)
	}
	f := answer.Findings[0]
	if f.Rule != "import" || f.File != "theme.css" || f.Line == 0 {
		t.Errorf("finding %+v", f)
	}

	var zipped bytes.Buffer
	zw := zip.NewWriter(&zipped)
	w, _ := zw.Create("../evil.css")
	w.Write([]byte("x"))
	zw.Close()
	status, raw = sendTheme(t, admin, "POST", "/api/admin/themes", zipped.Bytes())
	if status != 400 || !strings.Contains(string(raw), `"rule":"zip"`) {
		t.Errorf("a path out of the package: %d\n%s", status, raw)
	}

	if status, raw := admin.do("POST", "/api/admin/themes", nil); status != 400 {
		t.Errorf("no file: %d\n%s", status, raw)
	}
	if themeListed(t, id) {
		t.Error("a refused package is listed")
	}
	if n, _ := latestAudit(t, "theme.import"); n != before {
		t.Error("a refused import was recorded")
	}
}

func TestThemeRoutesRefuseOrdinaryUsers(t *testing.T) {
	admin := login(t, "admin", "admin")
	user := login(t, "paul.martin", jirafixture.Password)
	data := themeZip(t, unique("chk"), "1.0.0", nil)
	if status, raw := sendTheme(t, user, "POST", "/api/admin/themes", data); status != 403 {
		t.Errorf("import: %d\n%s", status, raw)
	}
	e := importTheme(t, admin, data)
	if status, _ := sendTheme(t, user, "PUT", "/api/admin/themes/"+e.ID, data); status != 403 {
		t.Errorf("replace: %d", status)
	}
	for _, call := range [][2]string{{"DELETE", ""}, {"POST", "/disable"}, {"POST", "/enable"}} {
		if status, _ := user.do(call[0], "/api/admin/themes/"+e.ID+call[1], nil); status != 403 {
			t.Errorf("%v: %d", call, status)
		}
	}
}

func TestThemeReplace(t *testing.T) {
	admin := login(t, "admin", "admin")
	id := unique("chk")
	e := importTheme(t, admin, themeZip(t, id, "1.0.0", nil))
	admin.call("POST", "/api/admin/themes/"+e.ID+"/disable", nil, 200, nil)
	before, _ := latestAudit(t, "theme.replace")

	marked := func(css string) string { return css + "\n/* new version */\n" }
	status, raw := sendTheme(t, admin, "PUT", "/api/admin/themes/"+e.ID, themeZip(t, id, "2.0.0", marked))
	if status != 200 {
		t.Fatalf("%d\n%s", status, raw)
	}
	got, _ := adminTheme(t, admin, e.ID)
	if got.Version != "2.0.0" || got.Enabled {
		t.Errorf("after the replacement %+v, it should stay disabled", got)
	}
	admin.call("POST", "/api/admin/themes/"+e.ID+"/enable", nil, 200, nil)
	if _, css := themeServes(t, e.ID); !strings.Contains(css, "new version") {
		t.Error("the old files are still served")
	}
	if n, details := latestAudit(t, "theme.replace"); n != before+1 || !strings.Contains(details, "2.0.0") || !strings.Contains(details, "1.0.0") {
		t.Errorf("audit: %d entries (was %d), details %s", n, before, details)
	}

	// Another manifest id, an invalid version: refused, nothing changes.
	status, raw = sendTheme(t, admin, "PUT", "/api/admin/themes/"+e.ID, themeZip(t, unique("oth"), "3.0.0", nil))
	if status != 400 || !strings.Contains(string(raw), `"rule":"manifest"`) {
		t.Errorf("another manifest id: %d\n%s", status, raw)
	}
	broken := func(css string) string { return css + "\n@import 'https://example.com/a.css';\n" }
	if status, _ := sendTheme(t, admin, "PUT", "/api/admin/themes/"+e.ID, themeZip(t, id, "3.0.0", broken)); status != 400 {
		t.Errorf("an invalid version: %d", status)
	}
	if got, _ := adminTheme(t, admin, e.ID); got.Version != "2.0.0" {
		t.Errorf("a refused version changed the theme: %+v", got)
	}
	if _, css := themeServes(t, e.ID); !strings.Contains(css, "new version") {
		t.Error("a refused version changed the files")
	}

	if status, _ := sendTheme(t, admin, "PUT", "/api/admin/themes/carnet", themeZip(t, "carnet", "9.0.0", nil)); status != 409 {
		t.Errorf("replacing a built-in: %d, want 409", status)
	}
	if status, _ := sendTheme(t, admin, "PUT", "/api/admin/themes/nothing-here", themeZip(t, id, "9.0.0", nil)); status != 404 {
		t.Errorf("replacing an unknown theme: %d, want 404", status)
	}
}

func TestThemeDisableAndDelete(t *testing.T) {
	admin := login(t, "admin", "admin")
	e := importTheme(t, admin, themeZip(t, unique("chk"), "1.0.0", nil))
	disabled, _ := latestAudit(t, "theme.disable")

	admin.call("POST", "/api/admin/themes/"+e.ID+"/disable", nil, 200, nil)
	admin.call("POST", "/api/admin/themes/"+e.ID+"/disable", nil, 200, nil)
	if n, _ := latestAudit(t, "theme.disable"); n != disabled+1 {
		t.Errorf("disabling twice recorded %d entries", n-disabled)
	}
	if themeListed(t, e.ID) {
		t.Error("a disabled theme is offered")
	}
	if status, _ := themeServes(t, e.ID); status != 404 {
		t.Errorf("a disabled theme's file: %d, want 404", status)
	}
	if got, ok := adminTheme(t, admin, e.ID); !ok || got.Enabled {
		t.Errorf("administrators should still see it, disabled: %+v, %v", got, ok)
	}

	admin.call("POST", "/api/admin/themes/"+e.ID+"/enable", nil, 200, nil)
	if !themeListed(t, e.ID) {
		t.Error("an enabled theme isn't offered")
	}
	if status, _ := themeServes(t, e.ID); status != 200 {
		t.Errorf("an enabled theme's file: %d", status)
	}

	deleted, _ := latestAudit(t, "theme.delete")
	admin.call("DELETE", "/api/admin/themes/"+e.ID, nil, 204, nil)
	if n, _ := latestAudit(t, "theme.delete"); n != deleted+1 {
		t.Error("the deletion was not recorded")
	}
	if status, _ := themeServes(t, e.ID); status != 404 {
		t.Errorf("a deleted theme's file: %d, want 404", status)
	}
	if _, ok := adminTheme(t, admin, e.ID); ok {
		t.Error("a deleted theme is listed")
	}
	admin.call("DELETE", "/api/admin/themes/"+e.ID, nil, 404, nil)
}

func TestBuiltinThemeDisable(t *testing.T) {
	admin := login(t, "admin", "admin")
	admin.call("POST", "/api/admin/themes/carnet/disable", nil, 409, nil)
	admin.call("DELETE", "/api/admin/themes/carnet", nil, 409, nil)
	admin.call("DELETE", "/api/admin/themes/plan", nil, 409, nil)
	if !themeListed(t, "carnet") {
		t.Error("Carnet is not offered")
	}

	admin.call("POST", "/api/admin/themes/plan/disable", nil, 200, nil)
	t.Cleanup(func() { admin.do("POST", "/api/admin/themes/plan/enable", nil) })
	if themeListed(t, "plan") {
		t.Error("a disabled built-in is offered")
	}
	if status, _ := themeServes(t, "plan"); status != 404 {
		t.Errorf("a disabled built-in's file: %d, want 404", status)
	}
	if got, ok := adminTheme(t, admin, "plan"); !ok || got.Enabled || got.Origin != "builtin" {
		t.Errorf("the administration's view %+v, %v", got, ok)
	}

	admin.call("POST", "/api/admin/themes/plan/enable", nil, 200, nil)
	if !themeListed(t, "plan") {
		t.Error("an enabled built-in isn't offered")
	}
	if status, _ := themeServes(t, "plan"); status != 200 {
		t.Errorf("an enabled built-in's file: %d", status)
	}
}

// exportedFiles downloads a theme's package, and returns its files.
func exportedFiles(t *testing.T, c *client, id string) (http.Header, map[string][]byte) {
	t.Helper()
	status, h, body := call(t, c, c.base, "GET", "/api/themes/"+id+"/export", nil, nil)
	if status != 200 {
		t.Fatalf("export %s: %d\n%s", id, status, body)
	}
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		files[f.Name], err = io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	return h, files
}

func zipFiles(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	for name, data := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestThemeExportBuiltinRoundTrip(t *testing.T) {
	admin := login(t, "admin", "admin")
	user := login(t, "paul.martin", jirafixture.Password)
	h, files := exportedFiles(t, user, "orbite")
	if h.Get("Content-Type") != "application/zip" || !strings.HasPrefix(h.Get("Content-Disposition"), `attachment; filename=orbite-`) {
		t.Errorf("headers %v", h)
	}
	root := os.DirFS("../internal/themes/builtin/orbite")
	var count int
	err := fs.WalkDir(root, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		count++
		want, err := fs.ReadFile(root, name)
		if err == nil && !bytes.Equal(files[name], want) {
			t.Errorf("%s differs in the export", name)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != count {
		t.Errorf("%d files exported, the package has %d", len(files), count)
	}

	// This instance has the id already: the import takes another, and keeps the anchor.
	e := importTheme(t, admin, zipFiles(t, files))
	if e.ID != "orbite-2" || e.ManifestID != "orbite" || e.Origin != "imported" {
		t.Errorf("import of the export %+v", e)
	}
	status, css := themeServes(t, e.ID)
	if status != 200 || css != string(files["theme.css"]) {
		t.Errorf("%s: %d: the imported stylesheet differs", e.ID, status)
	}
}

func TestThemeExportImported(t *testing.T) {
	admin := login(t, "admin", "admin")
	user := login(t, "paul.martin", jirafixture.Password)
	id := unique("chk")
	data := themeZip(t, id, "1.2.3", nil)
	importTheme(t, admin, data)
	second := importTheme(t, admin, data)

	// A second copy, served under a suffix, exports under its package's id.
	h, files := exportedFiles(t, user, second.ID)
	if got := h.Get("Content-Disposition"); !strings.Contains(got, id+"-1.2.3.zip") {
		t.Errorf("Content-Disposition %q", got)
	}
	var m struct{ ID string }
	if err := json.Unmarshal(files["theme.json"], &m); err != nil || m.ID != id {
		t.Errorf("exported manifest id %q, %v", m.ID, err)
	}

	admin.call("POST", "/api/admin/themes/"+second.ID+"/disable", nil, 200, nil)
	user.call("GET", "/api/themes/"+second.ID+"/export", nil, 404, nil)
	exportedFiles(t, admin, second.ID)
}

func TestThemeExportRefused(t *testing.T) {
	admin := login(t, "admin", "admin")
	user := login(t, "paul.martin", jirafixture.Password)
	anonymous(t).call("GET", "/api/themes/carnet/export", nil, 401, nil)
	user.call("GET", "/api/themes/nobody/export", nil, 404, nil)

	admin.call("POST", "/api/admin/themes/plan/disable", nil, 200, nil)
	t.Cleanup(func() { admin.do("POST", "/api/admin/themes/plan/enable", nil) })
	user.call("GET", "/api/themes/plan/export", nil, 404, nil)
	exportedFiles(t, admin, "plan")
}
