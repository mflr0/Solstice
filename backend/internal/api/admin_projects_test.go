package api

import (
	"errors"
	"net/http"
	"testing"

	"github.com/go-sql-driver/mysql"
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
