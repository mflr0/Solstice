package api

import "testing"

func TestLocalPath(t *testing.T) {
	for in, want := range map[string]string{
		"/browse/X-1?tab=1":     "/browse/X-1?tab=1",
		"":                      "/",
		"//evil.example":        "/",
		`/\evil.example`:        "/",
		"https://evil.example/": "/",
		"browse":                "/",
	} {
		if got := localPath(in); got != want {
			t.Errorf("localPath(%q) = %q, want %q", in, got, want)
		}
	}
}
