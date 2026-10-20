package api

import (
	"strconv"
	"testing"

	"solstice/internal/jql"
)

// Every clause of a system field reported orderable must compile in ORDER BY,
// and none of a field reported not orderable.
func TestSystemFieldsOrderable(t *testing.T) {
	for _, f := range systemFields {
		for _, c := range f.clauses {
			q, err := jql.Parse("ORDER BY " + strconv.Quote(c))
			if err != nil {
				t.Fatalf("%s: parse %q: %v", f.id, c, err)
			}
			_, _, _, err = jql.Compile(q, jql.Env{})
			if sorts := err == nil; sorts != f.orderable() {
				t.Errorf("%s: orderable %v, but ORDER BY %q: %v", f.id, f.orderable(), c, err)
			}
		}
	}
}
