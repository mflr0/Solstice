package api

import (
	"errors"
	"fmt"
	"testing"

	"github.com/go-sql-driver/mysql"
)

func TestIsDuplicate(t *testing.T) {
	duplicate := &mysql.MySQLError{Number: 1062, Message: "Duplicate entry 'x' for key 'uq'"}
	for name, c := range map[string]struct {
		err  error
		want bool
	}{
		"unique key":         {duplicate, true},
		"wrapped unique key": {fmt.Errorf("insert: %w", duplicate), true},
		"other number":       {&mysql.MySQLError{Number: 1213, Message: "Deadlock found"}, false},
		"text only":          {errors.New("Duplicate entry 'x'"), false},
		"nil":                {nil, false},
	} {
		if got := isDuplicate(c.err); got != c.want {
			t.Errorf("%s: isDuplicate = %v, want %v", name, got, c.want)
		}
	}
}
