package api

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"solstice/internal/i18n"
)

func testImport() *csvImport {
	return &csvImport{
		loc: "en", tz: time.UTC, projectKey: "DEST",
		cat: &catalog{
			IssueTypes:  []typeRef{{ID: "1", Name: "Bug"}, {ID: "5", Name: "Sub-task", Subtask: true}},
			Statuses:    []statusRef{{ID: "1", Name: "Open"}, {ID: "6", Name: "Closed"}},
			Resolutions: []namedRef{{ID: "1", Name: "Fixed"}},
			Priorities:  []priorityRef{{ID: "2", Name: "High"}},
		},
		custom: []customFieldDef{
			{ID: 10, Name: "Severity", Kind: "select"},
			{ID: 11, Name: "Team", Kind: "textfield"},
			{ID: 12, Name: "Team", Kind: "textarea"},
			{ID: 13, Name: "Epic Link", Kind: "gh-epic-link"},
			{ID: 14, Name: "Sprints", Kind: "gh-sprint"},
			{ID: 15, Name: "Area", Kind: "cascadingselect"},
			{ID: 16, Name: "Rank", Kind: "gh-lexo-rank"},
		},
		parts: map[string]map[string]int64{
			"components": {"api": 3, "front, back": 4},
			"versions":   {"1.0": 7, "2.0": 8},
			"security":   {"restreint": 9},
		},
		users: map[string]importUser{},
	}
}

// Headers map by id, alias, name in any language and custom field name;
// read-only and unknown columns are not imported, and a field goes to its
// first column only.
func TestImportColumnsFromHeaders(t *testing.T) {
	im := testImport()
	header := []string{
		"Key",                                  // English name
		i18n.M(i18n.FieldNameSummary).In("fr"), // French name
		"type",                                 // alias
		i18n.M(i18n.FieldNameStatus).In("de"),  // German name
		"fixVersions",                          // id
		"Created",                              // read-only
		"Severity",                             // custom field name
		"customfield_11",                       // custom field id
		"Team",                                 // name two custom fields share
		"Epic Link",                            // the epic link field is the epic
		"Sprints",                              // read-only custom
		"Rank",                                 // custom kind that cannot be set
		"Whatever",                             // unknown
		"Summary",                              // second column of a field
	}
	cols, err := im.columns(header, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []importColumn{
		{Header: "Key", Field: "issuekey", Known: true},
		{Header: header[1], Field: "summary", Known: true},
		{Header: "type", Field: "issuetype", Known: true},
		{Header: header[3], Field: "status", Known: true},
		{Header: "fixVersions", Field: "fixVersions", Known: true},
		{Header: "Created", Known: true, ReadOnly: true},
		{Header: "Severity", Field: "customfield_10", Known: true},
		{Header: "customfield_11", Field: "customfield_11", Known: true},
		{Header: "Team"},
		{Header: "Epic Link", Field: "epic", Known: true},
		{Header: "Sprints", Known: true, ReadOnly: true},
		{Header: "Rank", Known: true, ReadOnly: true},
		{Header: "Whatever"},
		{Header: "Summary", Known: true},
	}
	if !slices.Equal(cols, want) {
		t.Errorf("columns:\n got %+v\nwant %+v", cols, want)
	}
}

// A mapping replaces the guesses; it must match the header and name each
// settable field once.
func TestImportColumnsFromMapping(t *testing.T) {
	im := testImport()
	header := []string{"Titre", "Kind", "Created"}
	cols, err := im.columns(header, []string{"summary", "issuetype", ""})
	if err != nil {
		t.Fatal(err)
	}
	if cols[0].Field != "summary" || cols[1].Field != "issuetype" || cols[2].Field != "" {
		t.Errorf("mapped: %+v", cols)
	}
	for _, c := range []struct {
		mapping []string
		want    i18n.Key
	}{
		{[]string{"summary"}, i18n.CSVImportMappingInvalid},
		{[]string{"summary", "created", ""}, i18n.CSVImportMappingInvalid},
		{[]string{"summary", "nope", ""}, i18n.CSVImportMappingInvalid},
		{[]string{"summary", "summary", ""}, i18n.CSVImportColumnTwice},
	} {
		_, err := im.columns(header, c.mapping)
		he, ok := err.(*httpError)
		if !ok || he.msg.Key != c.want {
			t.Errorf("%v: %v, want %s", c.mapping, err, c.want)
		}
	}
}

// A row becomes the fields the interface would send, and its unknown values
// errors that name the column.
func TestImportReadRow(t *testing.T) {
	im := testImport()
	header := []string{"Key", "Summary", "Issue Type", "Status", "Resolution", "Priority", "Labels", "Components",
		"Affects versions", "Due Date", "Parent", "Epic Link", "Security Level", "Original Estimate",
		"Remaining Estimate", "Severity", "Area", "Description"}
	cols, err := im.columns(header, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range cols {
		if c.Field == "" {
			t.Fatalf("column %q not mapped", header[i])
		}
	}
	row := &importRow{f: fields{}}
	rec := []string{"old-3", "'=sum", "bug", "Closed", "fixed", "High", "a, b c", "Front, back, API",
		"1.0, 2.0", "2024-04-15", "OLD-1", "OLD-2", "Restreint", "2.5", "0,5", "Major", "Web › CSS", ""}
	if err := im.read(row, cols, rec); err != nil {
		t.Fatal(err)
	}
	if len(row.Errors) > 0 {
		t.Fatalf("errors: %v", row.Errors)
	}
	got := map[string]string{}
	for k, v := range row.f {
		got[k] = string(v)
	}
	want := map[string]string{
		"project": `"DEST"`, "summary": `"=sum"`, "type": `"1"`, "priority": `"2"`, "labels": `["a","b","c"]`,
		"components": `[4,3]`, "affectsVersions": `[7,8]`, "due": `"2024-04-15"`, "securityLevel": `9`,
		"originalEstimate": `"150m"`, "remainingEstimate": `"30m"`,
		"customFields": `{"customfield_10":{"value":"Major"},"customfield_15":{"child":{"value":"CSS"},"value":"Web"}}`,
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %s, want %s", k, got[k], v)
		}
	}
	if len(got) != len(want) {
		t.Errorf("fields: %v", got)
	}
	if row.Key != "OLD-3" || row.parentKey != "OLD-1" || row.epicKey != "OLD-2" ||
		row.opts.status != "6" || row.opts.resolution != "1" || row.Summary != "=sum" {
		t.Errorf("row: %+v", row)
	}

	bad := &importRow{f: fields{}}
	if err := im.read(bad, cols[:8], []string{"", "x", "Story", "Done", "Won't do", "Huge", "", "API, Mobile"}); err != nil {
		t.Fatal(err)
	}
	wantErrs := []string{
		i18n.M(i18n.CSVImportValueUnknown, "Issue Type", "Story").In("en"),
		i18n.M(i18n.CSVImportValueUnknown, "Status", "Done").In("en"),
		i18n.M(i18n.CSVImportValueUnknown, "Resolution", "Won't do").In("en"),
		i18n.M(i18n.CSVImportValueUnknown, "Priority", "Huge").In("en"),
		i18n.M(i18n.CSVImportValueUnknown, "Components", "Mobile").In("en"),
	}
	if !slices.Equal(bad.Errors, wantErrs) {
		t.Errorf("errors:\n got %q\nwant %q", bad.Errors, wantErrs)
	}
}

// The file: a byte order mark, quoted cells over several lines, blank rows
// left out, semicolons when the header has more of them, and the caps.
func TestReadImportCSV(t *testing.T) {
	data := "\ufeffKey,Summary\r\nA-1,\"two\r\nlines\"\r\n,\r\nA-2,plain\r\n"
	header, records, lines, err := readImportCSV([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(header, []string{"Key", "Summary"}) || len(records) != 2 || records[0][1] != "two\nlines" ||
		!slices.Equal(lines, []int{2, 5}) {
		t.Errorf("header %q, records %q, lines %v", header, records, lines)
	}
	if header, _, _, _ := readImportCSV([]byte("Clé;Résumé, court\nA;b")); len(header) != 2 {
		t.Errorf("semicolons: %q", header)
	}
	for _, c := range []struct {
		data string
		want i18n.Key
	}{
		{"", i18n.CSVImportFileEmpty},
		{"a,b\n\"open", i18n.CSVImportFileInvalid},
		{"Summary\n" + strings.Repeat("x\n", maxImportRows+1), i18n.CSVImportTooManyRows},
	} {
		_, _, _, err := readImportCSV([]byte(c.data))
		he, ok := err.(*httpError)
		if !ok || he.msg.Key != c.want {
			t.Errorf("%.20q: %v, want %s", c.data, err, c.want)
		}
	}
	if _, _, _, err := readImportCSV([]byte("Summary\n" + strings.Repeat("x\n", maxImportRows))); err != nil {
		t.Errorf("%d rows: %v", maxImportRows, err)
	}
}

// A datetime custom field is read in the file's time zone.
func TestImportCustomDatetime(t *testing.T) {
	im := testImport()
	im.custom = append(im.custom, customFieldDef{ID: 20, Name: "When", Kind: "datetime"})
	im.tz, _ = time.LoadLocation("Europe/Paris")
	v, ok, err := im.customValue("customfield_20", "2024-03-01 09:30")
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	b, _ := json.Marshal(v)
	if string(b) != `"2024-03-01T09:30:00.000+0100"` {
		t.Errorf("datetime: %s", b)
	}
}
