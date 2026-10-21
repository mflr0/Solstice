package api

import (
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"

	"solstice/internal/i18n"
)

// A project key taken between the check and the write comes back as the
// refusal the check gives, not as a server error.
func TestKeyConflict(t *testing.T) {
	duplicate := &mysql.MySQLError{Number: 1062, SQLState: [5]byte{'2', '3', '0', '0', '0'},
		Message: "Duplicate entry 'ABCD' for key 'uq_projects_pkey'"}
	var he *httpError
	if err := keyConflict(duplicate, "ABCD"); !errors.As(err, &he) || he.status != http.StatusConflict {
		t.Errorf("a duplicate key gives %v, want a 409", err)
	}
	other := &mysql.MySQLError{Number: 1213, SQLState: [5]byte{'4', '0', '0', '0', '1'}, Message: "Deadlock found"}
	if err := keyConflict(other, "ABCD"); err != error(other) {
		t.Errorf("another database error gives %v, want it unchanged", err)
	}
	if err := keyConflict(nil, "ABCD"); err != nil {
		t.Errorf("no error gives %v", err)
	}
}

// A template's statuses are named in the language given, each name its own:
// statuses are reused by name, whatever the case.
func TestTemplateStatuses(t *testing.T) {
	var names []string
	for _, st := range templateStatuses(projectTemplates["software"].statuses, "fr") {
		names = append(names, st.name)
	}
	if want := []string{"À faire", "En cours", "En revue", "Terminé"}; !slices.Equal(names, want) {
		t.Errorf("software template in French: %q, want %q", names, want)
	}
	for _, lang := range i18n.Locales {
		for tmpl, p := range projectTemplates {
			seen := map[string]bool{}
			for _, st := range templateStatuses(p.statuses, lang) {
				if k := strings.ToLower(st.name); seen[k] {
					t.Errorf("%s template in %s: %q twice", tmpl, lang, st.name)
				} else {
					seen[k] = true
				}
			}
		}
	}
}
