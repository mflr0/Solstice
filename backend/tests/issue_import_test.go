package tests

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"solstice/internal/jirafixture"
)

// importCSV sends a file to a project's import, as the interface does.
func importCSV(t *testing.T, c *client, project string, check bool, data []byte) (int, []byte) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "issues.csv")
	fw.Write(data)
	mw.WriteField("tz", "UTC")
	mw.Close()
	path := "/api/projects/" + project + "/import.csv"
	if check {
		path += "?check=1"
	}
	req, _ := http.NewRequest("POST", c.base+path, &buf)
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

type importAnswer struct {
	Rows []struct {
		Line   int
		Key    string
		Errors []string
	}
	Created []string
	Import  *importAnswer // the report of a refused import
}

// exportRows exports a project's issues in the columns given, by key.
func exportRows(t *testing.T, c *client, project, columns string) (header []string, rows map[string][]string) {
	t.Helper()
	q := url.Values{"jql": {"project = " + project + " ORDER BY key"}, "columns": {"issuekey," + columns}, "tz": {"UTC"}}
	status, body := c.do("GET", "/api/issues/export.csv?"+q.Encode(), nil)
	if status != 200 {
		t.Fatalf("export %s: %d %s", project, status, body)
	}
	records, err := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(body, []byte("\xef\xbb\xbf")))).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	rows = map[string][]string{}
	for _, r := range records[1:] {
		rows[r[0]] = r[1:]
	}
	return records[0], rows
}

// An exported search imports back into another project with the same
// values; parents and epics of the file become the new issues; a check
// writes nothing; and a file with one bad row creates nothing.
func TestCSVImportRoundTrip(t *testing.T) {
	admin := login(t, "admin", "admin")
	paul := login(t, "paul.martin", jirafixture.Password)
	src, dst := newProject(t, admin, nil), newProject(t, admin, nil)
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := testDB.Exec(query, args...); err != nil {
			t.Fatalf("%v\n%s", err, query)
		}
	}
	for _, p := range []string{src, dst} {
		exec(`INSERT INTO components (project_id, name) SELECT id, 'API' FROM projects WHERE pkey = ?`, p)
		exec(`INSERT INTO versions (project_id, name, position) SELECT id, '1.0', 0 FROM projects WHERE pkey = ?`, p)
	}
	var meta struct {
		IssueTypes []struct {
			ID, Name string
			Subtask  bool
		}
	}
	paul.call("GET", "/api/projects/"+src, nil, 200, &meta)
	types := map[string]string{}
	for _, it := range meta.IssueTypes {
		types[it.Name] = it.ID
	}
	part := func(table string) int64 {
		return dbInt(t, `SELECT x.id FROM `+table+` x JOIN projects p ON p.id = x.project_id WHERE p.pkey = ?`, src)
	}
	create := func(in map[string]any) string {
		t.Helper()
		in["project"] = src
		var out struct{ Key string }
		paul.call("POST", "/api/issues", in, 201, &out)
		return out.Key
	}
	epic := create(map[string]any{"type": types["Epic"], "summary": "Import epic"})
	story := create(map[string]any{"type": types["Story"], "summary": "=Import story, guarded",
		"description": "Line 1\nLine 2", "labels": []string{"alpha", "beta"}, "components": []int64{part("components")},
		"fixVersions": []int64{part("versions")}, "due": "2024-04-15", "assignee": userID(t, "paul.martin"),
		"epic": epic, "originalEstimate": "2h 30m"})
	create(map[string]any{"type": types["Sub-task"], "summary": "Import sub-task", "parent": story})
	// The last status of the workflow is a done one.
	exec(`UPDATE issues i JOIN projects p ON p.id = i.project_id
		SET i.status_id = (SELECT ws.status_id FROM workflow_statuses ws JOIN workflows w ON w.id = ws.workflow_id
			WHERE w.project_id = p.id ORDER BY ws.position DESC LIMIT 1),
		i.resolution_id = (SELECT id FROM resolutions WHERE name = 'Fixed')
		WHERE p.pkey = ? AND i.num = ?`, src, strings.TrimPrefix(story, src+"-"))

	columns := "summary,issuetype,status,resolution,priority,assignee,reporter,description,labels,components," +
		"fixVersions,duedate,timeoriginalestimate,parent,epic,created"
	_, srcRows := exportRows(t, admin, src, columns)
	q := url.Values{"jql": {"project = " + src + " ORDER BY key DESC"}, "columns": {"issuekey," + columns}}
	status, file := admin.do("GET", "/api/issues/export.csv?"+q.Encode(), nil)
	if status != 200 {
		t.Fatalf("export: %d", status)
	}

	// Sub-task first in the file: rows are created after the rows they name.
	countDst := func() int64 {
		return dbInt(t, `SELECT COUNT(*) FROM issues i JOIN projects p ON p.id = i.project_id WHERE p.pkey = ?`, dst)
	}
	paul.call("GET", "/api/projects/"+dst, nil, 200, nil)
	if st, _ := importCSV(t, paul, dst, true, file); st != 403 {
		t.Errorf("a member who does not administer the project: %d, want 403", st)
	}
	var checked importAnswer
	st, body := importCSV(t, admin, dst, true, file)
	if st != 200 || json.Unmarshal(body, &checked) != nil || len(checked.Rows) != 3 {
		t.Fatalf("check: %d %s", st, body)
	}
	for _, r := range checked.Rows {
		if len(r.Errors) > 0 {
			t.Errorf("check, line %d: %v", r.Line, r.Errors)
		}
	}
	if n := countDst(); n != 0 {
		t.Fatalf("the check created %d issues", n)
	}

	before, _ := latestAudit(t, "issue.import")
	var done importAnswer
	st, body = importCSV(t, admin, dst, false, file)
	if st != 200 || json.Unmarshal(body, &done) != nil || len(done.Created) != 3 {
		t.Fatalf("import: %d %s", st, body)
	}
	if n, _ := latestAudit(t, "issue.import"); n != before+1 {
		t.Errorf("audit entries: %d, want %d", n, before+1)
	}

	// Same values, the keys aside: parent and epic are matched by summary.
	_, dstRows := exportRows(t, admin, dst, columns)
	summaries := func(rows map[string][]string) map[string]string {
		out := map[string]string{"": ""}
		for k, r := range rows {
			out[k] = r[0]
		}
		return out
	}
	srcSum, dstSum := summaries(srcRows), summaries(dstRows)
	bySummary := map[string][]string{}
	for _, r := range dstRows {
		bySummary[r[0]] = r
	}
	n := len(strings.Split(columns, ","))
	for k, want := range srcRows {
		got, ok := bySummary[want[0]]
		if !ok {
			t.Errorf("%s (%q) not imported", k, want[0])
			continue
		}
		for i, col := range strings.Split(columns, ",")[:n-1] { // created differs
			w, g := want[i], got[i]
			if col == "parent" || col == "epic" {
				w, g = srcSum[w], dstSum[g]
			}
			if w != g {
				t.Errorf("%s %s: %q, want %q", k, col, g, w)
			}
		}
	}
	if pick(t, `SELECT u.username FROM issues i JOIN projects p ON p.id = i.project_id JOIN users u ON u.id = i.creator_id
		WHERE p.pkey = ? LIMIT 1`, dst) != "admin" {
		t.Errorf("the importer is not the creator")
	}

	// One bad row: nothing is created, and the report names that row only.
	bad := bytes.Replace(file, []byte("Import sub-task,Sub-task,"), []byte("Import sub-task,No such type,"), 1)
	if bytes.Equal(bad, file) {
		t.Fatalf("the file does not have the expected row:\n%s", file)
	}
	countBefore := countDst()
	var refused importAnswer
	st, body = importCSV(t, admin, dst, false, bad)
	if st != 400 || json.Unmarshal(body, &refused) != nil || refused.Import == nil {
		t.Fatalf("bad file: %d %s", st, body)
	}
	failed := 0
	for _, r := range refused.Import.Rows {
		if len(r.Errors) > 0 {
			failed++
		}
	}
	if failed != 1 {
		t.Errorf("rows with errors: %d, want 1\n%s", failed, body)
	}
	if n := countDst(); n != countBefore {
		t.Errorf("a refused import created %d issues", n-countBefore)
	}
}
