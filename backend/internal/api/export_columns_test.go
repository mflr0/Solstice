package api

import (
	"errors"
	"net/http"
	"slices"
	"testing"
)

func TestParseColumns(t *testing.T) {
	custom := []customFieldDef{{ID: 10, Name: "Story Points", Kind: "float"}, {ID: 11, Name: "Team", Kind: "select"}}
	cols, err := parseColumns(" key, customfield_10,,due , epic,sprint,customfield_11", "fr", custom)
	if err != nil {
		t.Fatal(err)
	}
	var ids, headers []string
	for _, c := range cols {
		ids = append(ids, c.id)
		headers = append(headers, c.header)
	}
	if want := []string{"issuekey", "customfield_10", "duedate", "epic", "sprint", "customfield_11"}; !slices.Equal(ids, want) {
		t.Errorf("ids = %v, want %v", ids, want)
	}
	if want := []string{"Clé", "Story Points", "Échéance", "Épopée", "Sprint", "Team"}; !slices.Equal(headers, want) {
		t.Errorf("headers = %v, want %v", headers, want)
	}
	if !cols[1].number || cols[5].number || cols[1].custom == nil || cols[1].custom.ID != 10 {
		t.Errorf("customfield_10 = %+v, customfield_11 = %+v", cols[1], cols[5])
	}
	if cols, err := parseColumns(" , ", "en", custom); err != nil || len(cols) != 0 {
		t.Errorf("blank = %v, %v; want none", cols, err)
	}
	for _, bad := range []string{"nope", "customfield_99", "key,customfield_"} {
		_, err := parseColumns(bad, "en", custom)
		var he *httpError
		if !errors.As(err, &he) || he.status != http.StatusBadRequest {
			t.Errorf("parseColumns(%q) = %v, want a 400", bad, err)
		}
	}
}
