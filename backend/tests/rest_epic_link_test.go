package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"solstice/internal/api"
	"solstice/internal/config"
	"solstice/internal/db"
	"solstice/internal/devseed"
)

// On an instance not imported from Jira, which has no Epic Link custom
// field, the REST API still shows one, backed by the issue's epic: listed
// once, in the edit meta, set by name, read back by id or by name, cleared,
// and found by JQL.
func TestRESTEpicLinkWithoutImport(t *testing.T) {
	dsn := emptyDB(t)
	ctx := context.Background()
	conn, err := db.Open(ctx, dsn, config.Defaults().DB.Pool())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatal(err)
	}
	if err := devseed.Run(ctx, conn, devseed.Options{SMTP: "localhost:1025", Out: &strings.Builder{}}); err != nil {
		t.Fatal(err)
	}
	var imported int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM custom_fields WHERE type_key LIKE '%gh-epic-link'`).
		Scan(&imported); err != nil || imported != 0 {
		t.Fatalf("the dev seed has %d Epic Link fields (%v)", imported, err)
	}
	ts := httptest.NewServer(api.New(conn, t.TempDir(), time.UTC, quiet()).Handler(http.NotFoundHandler()))
	defer ts.Close()
	jar, _ := cookiejar.New(nil)
	alice := &client{t: t, base: ts.URL, http: &http.Client{Jar: jar, Timeout: 30 * time.Second}}
	if status, body := alice.do("POST", "/api/login", map[string]string{"username": "alice",
		"password": devseed.UserPassword}); status != 200 {
		t.Fatalf("login alice: %d %s", status, body)
	}

	var fields []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	alice.call("GET", "/rest/api/2/field", nil, 200, &fields)
	fieldID := ""
	for _, f := range fields {
		if f.Name == "Epic Link" {
			if fieldID != "" {
				t.Fatalf("Epic Link listed twice: %s and %s", fieldID, f.ID)
			}
			fieldID = f.ID
		}
	}
	if !strings.HasPrefix(fieldID, "customfield_") {
		t.Fatalf("no Epic Link custom field: %+v", fields)
	}

	create := func(fields map[string]any) string {
		fields["project"] = map[string]string{"key": devseed.Project}
		var out struct{ Key string }
		alice.call("POST", "/rest/api/2/issue", map[string]any{"fields": fields}, 201, &out)
		return out.Key
	}
	epic := create(map[string]any{"summary": "Epic link epic", "issuetype": map[string]string{"name": "Epic"}})
	task := create(map[string]any{"summary": "Epic link task", "issuetype": map[string]string{"name": "Task"},
		"Epic Link": epic})

	epicOf := func(selection string) any {
		var out struct{ Fields map[string]any }
		alice.call("GET", "/rest/api/2/issue/"+task+"?fields="+url.QueryEscape(selection), nil, 200, &out)
		return out.Fields[fieldID]
	}
	for _, sel := range []string{fieldID, "Epic Link", "*all"} {
		if got := epicOf(sel); got != epic {
			t.Errorf("fields=%s: epic %v, want %s", sel, got, epic)
		}
	}

	var search struct {
		Issues []struct{ Key string }
	}
	alice.call("GET", "/rest/api/2/search?fields=summary&jql="+url.QueryEscape(`"Epic Link" = `+epic), nil, 200, &search)
	if len(search.Issues) != 1 || search.Issues[0].Key != task {
		t.Errorf(`"Epic Link" = %s: %+v, want %s`, epic, search.Issues, task)
	}

	var meta struct {
		Fields map[string]json.RawMessage
	}
	alice.call("GET", "/rest/api/2/issue/"+task+"/editmeta", nil, 200, &meta)
	if _, ok := meta.Fields[fieldID]; !ok {
		t.Errorf("Epic Link not in the task's edit meta")
	}
	meta.Fields = nil
	alice.call("GET", "/rest/api/2/issue/"+epic+"/editmeta", nil, 200, &meta)
	if _, ok := meta.Fields[fieldID]; ok {
		t.Errorf("Epic Link in the epic's edit meta")
	}

	alice.call("PUT", "/rest/api/2/issue/"+task, map[string]any{"fields": map[string]any{fieldID: nil}}, 204, nil)
	if got := epicOf(fieldID); got != nil {
		t.Errorf("after clearing: epic %v", got)
	}
}
