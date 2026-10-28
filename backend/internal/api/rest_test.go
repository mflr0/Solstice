package api

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestForgeryProof(t *testing.T) {
	for _, c := range []struct {
		method, via, ctype, header string
		want                       bool
	}{
		{"GET", viaCookie, "", "", true},
		{"POST", viaCookie, "application/json", "", false},
		{"POST", viaCookie, "application/json", atlassianXSRF, true},
		{"DELETE", viaCookie, "", "", false},
		{"POST", viaToken, "multipart/form-data; boundary=x", "", true},
		{"POST", viaBasic, "application/json", "", true},
		{"PUT", viaBasic, "text/plain", "", true},
		{"POST", viaBasic, "multipart/form-data; boundary=x", "", false},
		{"POST", viaBasic, "", "", false},
		{"POST", viaBasic, "text/plain;charset=utf-8", csrfHeader, true},
		{"POST", "", "application/json", "", false},
	} {
		r := httptest.NewRequest(c.method, "/rest/api/2/issue", nil)
		if c.ctype != "" {
			r.Header.Set("Content-Type", c.ctype)
		}
		switch c.header {
		case atlassianXSRF:
			r.Header.Set(c.header, noCheck)
		case csrfHeader:
			r.Header.Set(c.header, "1")
		}
		if got := forgeryProof(r, c.via); got != c.want {
			t.Errorf("%s via %q, %q, header %q: %v, want %v", c.method, c.via, c.ctype, c.header, got, c.want)
		}
	}
}

func TestFieldSelection(t *testing.T) {
	sel := parseFieldSelection(nil, "*navigable")
	if !sel.hasSystem("summary") || sel.hasSystem("comment") || !sel.has("customfield_1", true) {
		t.Error("*navigable")
	}
	sel = parseFieldSelection([]string{"summary,comment", "-summary"}, "*all")
	if sel.hasSystem("summary") || !sel.hasSystem("comment") || sel.hasSystem("status") {
		t.Error("explicit list")
	}
	sel = parseFieldSelection([]string{"-comment"}, "*all")
	if sel.hasSystem("comment") || !sel.hasSystem("worklog") {
		t.Error("exclusions only keep the default")
	}
}

func TestParseJiraTime(t *testing.T) {
	want := time.Date(2026, 10, 15, 7, 12, 0, 0, time.UTC)
	for _, v := range []string{"2026-10-15T09:12:00.000+0200", "2026-10-15T07:12:00.000Z", "2026-10-15T09:12:00+02:00",
		"2026-10-15T09:12:00+0200"} {
		if got, ok := parseJiraTime(v); !ok || !got.Equal(want) {
			t.Errorf("parseJiraTime(%q) = %v %v", v, got, ok)
		}
	}
	if _, ok := parseJiraTime("15/10/2026"); ok {
		t.Error("accepted 15/10/2026")
	}
}

func TestSafeURL(t *testing.T) {
	for _, v := range []string{"https://maquettes.exemple.fr/x", "http://x.fr", "mailto:a@b.fr"} {
		if !safeURL(v) {
			t.Errorf("safeURL(%q) = false", v)
		}
	}
	for _, v := range []string{"javascript:alert(1)", "JaVaScRiPt:alert(1)", "data:text/html,<b>",
		"vbscript:msgbox", "file:///etc/passwd", "example.com", ""} {
		if safeURL(v) {
			t.Errorf("safeURL(%q) = true", v)
		}
	}
}
