package tests

import "testing"

// Solstice has no project types: the REST API calls every project a software
// one, RH too, a business project in Jira (chloe.simon sees it).
func TestRESTProjectTypeKey(t *testing.T) {
	rest := restBasicAs(t, "chloe.simon")
	get := func(path string) any {
		t.Helper()
		status, v := rest("GET", path, nil)
		if status != 200 {
			t.Fatalf("GET %s: %d", path, status)
		}
		return v
	}
	check := func(what string, p any) {
		t.Helper()
		if got := str(p, "projectTypeKey"); got != "software" {
			t.Errorf("%s: projectTypeKey %q, want software", what, got)
		}
	}
	seen := false
	for _, p := range list(get("/rest/api/2/project")) {
		check("/project "+str(p, "key"), p)
		seen = seen || str(p, "key") == "RH"
	}
	if !seen {
		t.Error("/project lacks RH")
	}
	check("/project/RH", get("/rest/api/2/project/RH"))
	key := rhKey(t)
	check("issue "+key, at(get("/rest/api/2/issue/"+key+"?fields=project"), "fields", "project"))
	meta := list(at(get("/rest/api/2/issue/createmeta?projectKeys=RH"), "projects"))
	if len(meta) != 1 {
		t.Fatalf("createmeta: %d projects, want RH", len(meta))
	}
	check("createmeta", meta[0])
}
