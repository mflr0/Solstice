package importer

import (
	"database/sql"
	"strings"
)

// globalIDs hands out the globalIds of remote links. Solstice keeps one per
// issue and compares them exactly; a Jira link that repeats one on the same
// issue (or has none) is imported without, instead of failing the import.
// Surrounding spaces are cut, as the REST API does: the column ignores trailing
// ones when it compares, so "abc" and "abc " would collide in its unique key.
type globalIDs map[globalKey]bool

type globalKey struct {
	issue int64
	id    string
}

// take returns the globalId to store for a link of an issue: nil when it has
// none or a link of the same issue already took it.
func (g globalIDs) take(issue int64, id sql.NullString) any {
	value := strings.TrimSpace(truncate(id.String, 255))
	if !id.Valid || value == "" || g[globalKey{issue, value}] {
		return nil
	}
	g[globalKey{issue, value}] = true
	return value
}
