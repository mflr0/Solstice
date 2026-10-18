package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

// A Jira database address with a driver option that is not accepted is
// refused before any connection is attempted, with a message naming the option.
func TestProbeSourceRefusesDriverOptions(t *testing.T) {
	for _, raw := range []string{
		"mysql://jira:secret@127.0.0.1:1/jira?allowAllFiles=true",
		"postgres://jira:secret@127.0.0.1:1/jira?sslkey=/etc/hostname",
	} {
		_, err := probeSource(context.Background(), &sourceForm{URL: raw, TZ: "Europe/Paris"}, "en")
		var he *httpError
		if !errors.As(err, &he) || he.status != http.StatusBadRequest {
			t.Fatalf("%s: %v, want a 400", raw, err)
		}
		msg := he.Error()
		name := "allowAllFiles"
		if strings.HasPrefix(raw, "postgres") {
			name = "sslkey"
		}
		if !strings.Contains(msg, name) || strings.Contains(msg, "secret") {
			t.Errorf("%s: message %q should name the option and not the password", raw, msg)
		}
	}
}
