package tests

// The issue page (issueDetail) and the REST API (loadIssues) read an issue's
// relations with separate code. These tests read the same issues through both
// and compare what they share.

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"

	"solstice/internal/jirafixture"
)

// readerDiffs are the custom field kinds the two readers show differently on
// purpose: the issue page leaves them out, the REST API fills them as Jira does.
var readerDiffs = map[string]string{
	"gh-sprint":    "the issue page lists sprints on their own; REST writes Jira's sprint strings",
	"gh-lexo-rank": "the issue page hides the rank; REST shows the issue's rank column",
	"gh-epic-link": "the issue page shows the epic on its own; REST reads the issue's epic column",
}

type parityAttachment struct {
	ID       int64  `json:"id"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	MimeType string `json:"mimeType"`
}

type parityNative struct {
	Labels          []string           `json:"labels"`
	Components      []idName           `json:"components"`
	AffectsVersions []idName           `json:"affectsVersions"`
	FixVersions     []idName           `json:"fixVersions"`
	Attachments     []parityAttachment `json:"attachments"`
	CustomFields    []struct {
		ID     int64     `json:"id"`
		Values []string  `json:"values"`
		Users  []userRef `json:"users"`
	} `json:"customFields"`
}

// REST writes ids as strings.
type restIDName struct {
	ID   int64  `json:"id,string"`
	Name string `json:"name"`
}

type restAttachment struct {
	ID       int64  `json:"id,string"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	MimeType string `json:"mimeType"`
}

type parityREST struct {
	Fields struct {
		Labels      []string         `json:"labels"`
		Components  []restIDName     `json:"components"`
		Versions    []restIDName     `json:"versions"`
		FixVersions []restIDName     `json:"fixVersions"`
		Attachment  []restAttachment `json:"attachment"`
	} `json:"fields"`
}

type parityField struct {
	id   int64
	kind string
}

func TestIssueReadParity(t *testing.T) {
	paul := login(t, "paul.martin", jirafixture.Password)
	var fields []parityField
	for _, row := range column(t, `SELECT CONCAT(id, ' ', type_key) FROM custom_fields ORDER BY id`) {
		id, typeKey, _ := strings.Cut(row, " ")
		n, _ := strconv.ParseInt(id, 10, 64)
		fields = append(fields, parityField{n, typeKey[strings.LastIndexByte(typeKey, ':')+1:]})
	}

	shown := map[string]bool{} // readerDiffs kinds REST filled on some issue
	for _, key := range parityIssues(t, paul, fields) {
		var native parityNative
		paul.call("GET", "/api/issues/"+key, nil, 200, &native)
		status, _, body := call(t, paul, testURL, "GET", "/rest/api/2/issue/"+key+"?fields=*all", nil, nil)
		if status != 200 {
			t.Fatalf("REST %s: %d %s", key, status, body)
		}
		var rest parityREST
		var restAll struct {
			Fields map[string]any `json:"fields"`
		}
		if err := json.Unmarshal(body, &rest); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(body, &restAll); err != nil {
			t.Fatal(err)
		}

		sameList(t, key+" labels", native.Labels, rest.Fields.Labels)
		sameList(t, key+" components", native.Components, convert(rest.Fields.Components, toIDName))
		sameList(t, key+" affects versions", native.AffectsVersions, convert(rest.Fields.Versions, toIDName))
		sameList(t, key+" fix versions", native.FixVersions, convert(rest.Fields.FixVersions, toIDName))
		sameList(t, key+" attachments", native.Attachments,
			convert(rest.Fields.Attachment, func(a restAttachment) parityAttachment { return parityAttachment(a) }))

		nativeCustom := map[int64][]string{}
		for _, cf := range native.CustomFields {
			values := cf.Values
			for _, u := range cf.Users {
				values = append(values, u.Username)
			}
			nativeCustom[cf.ID] = values
		}
		for _, f := range fields {
			what := fmt.Sprintf("%s customfield_%d (%s)", key, f.id, f.kind)
			restValues := restCustomValues(t, what, restAll.Fields[fmt.Sprintf("customfield_%d", f.id)])
			if _, ok := readerDiffs[f.kind]; ok {
				if values, ok := nativeCustom[f.id]; ok {
					t.Errorf("%s: the issue page shows %q; drop the kind from readerDiffs", what, values)
				}
				shown[f.kind] = shown[f.kind] || len(restValues) > 0
				continue
			}
			sameList(t, what, nativeCustom[f.id], restValues)
		}
	}
	for kind, why := range readerDiffs {
		if !shown[kind] {
			t.Errorf("REST filled no %s field (%s): drop the kind from readerDiffs", kind, why)
		}
	}
}

// parityIssues picks the issues to compare, each the first of its kind the
// user sees: one with labels, components, versions and attachments, one with
// an epic, and one per custom field that has a value somewhere.
func parityIssues(t *testing.T, c *client, fields []parityField) []string {
	t.Helper()
	from := `SELECT ` + keyExpr + ` FROM issues i JOIN projects p ON p.id = i.project_id WHERE `
	const order = ` ORDER BY p.pkey, i.num LIMIT 50`
	type want struct {
		what, where string
		args        []any
		required    bool
	}
	wants := []want{
		{"relations", `EXISTS (SELECT 1 FROM labels l WHERE l.issue_id = i.id AND l.field_id IS NULL)
			AND EXISTS (SELECT 1 FROM issue_components c WHERE c.issue_id = i.id)
			AND EXISTS (SELECT 1 FROM issue_versions v WHERE v.issue_id = i.id)
			AND EXISTS (SELECT 1 FROM attachments a WHERE a.issue_id = i.id)`, nil, true},
		{"epic", `i.epic_id IS NOT NULL`, nil, true},
	}
	for _, f := range fields {
		wants = append(wants, want{fmt.Sprintf("customfield_%d", f.id),
			`(EXISTS (SELECT 1 FROM custom_field_values v WHERE v.issue_id = i.id AND v.field_id = ?)
			OR EXISTS (SELECT 1 FROM labels l WHERE l.issue_id = i.id AND l.field_id = ?))`, []any{f.id, f.id}, false})
	}

	seen := map[string]bool{}
	var keys []string
	for _, w := range wants {
		found := false
		for _, key := range column(t, from+w.where+order, w.args...) {
			visible, ok := seen[key]
			if !ok {
				visible = c.get("/api/issues/"+key, nil) == 200
				seen[key] = visible
				if visible {
					keys = append(keys, key)
				}
			}
			if visible {
				found = true
				break
			}
		}
		if !found && w.required {
			t.Fatalf("no issue with %s that paul.martin sees", w.what)
		}
	}
	return keys
}

// restCustomValues writes a REST custom field value the way the issue page
// does: options by value, cascading options as "parent › child", users by
// name, numbers in the shortest form.
func restCustomValues(t *testing.T, what string, v any) []string {
	t.Helper()
	switch v := v.(type) {
	case nil:
		return nil
	case string:
		return []string{v}
	case float64:
		return []string{strconv.FormatFloat(v, 'f', -1, 64)}
	case []any:
		var out []string
		for _, e := range v {
			out = append(out, restCustomValues(t, what, e)...)
		}
		return out
	case map[string]any:
		switch {
		case v["child"] != nil:
			return []string{str(v, "value") + " › " + str(v, "child", "value")}
		case v["value"] != nil:
			return []string{str(v, "value")}
		case v["name"] != nil:
			return []string{str(v, "name")}
		}
	}
	t.Fatalf("%s: unexpected REST value %#v", what, v)
	return nil
}

func sameList[T comparable](t *testing.T, what string, native, rest []T) {
	t.Helper()
	if !slices.Equal(native, rest) {
		t.Errorf("%s:\nissue page %v\nREST       %v", what, native, rest)
	}
}

func convert[S, T any](in []S, f func(S) T) []T {
	out := make([]T, 0, len(in))
	for _, v := range in {
		out = append(out, f(v))
	}
	return out
}

func toIDName(v restIDName) idName { return idName(v) }
