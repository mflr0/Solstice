package importer

import (
	"database/sql"
	"strings"
	"testing"
)

func TestGlobalIDsTake(t *testing.T) {
	ids := globalIDs{}
	str := func(s string) sql.NullString { return sql.NullString{String: s, Valid: true} }

	if got := ids.take(1, str("gitlab:mr:7")); got != "gitlab:mr:7" {
		t.Errorf("first use: %v", got)
	}
	if got := ids.take(1, str("gitlab:mr:7")); got != nil {
		t.Errorf("same issue, same globalId: %v, want nil", got)
	}
	if got := ids.take(1, str("GITLAB:MR:7")); got != "GITLAB:MR:7" {
		t.Errorf("another case is another globalId: %v", got)
	}
	if got := ids.take(2, str("gitlab:mr:7")); got != "gitlab:mr:7" {
		t.Errorf("same globalId, other issue: %v", got)
	}
	if got := ids.take(1, str("")); got != nil {
		t.Errorf("empty: %v, want nil", got)
	}
	if got := ids.take(1, sql.NullString{}); got != nil {
		t.Errorf("NULL: %v, want nil", got)
	}
	long := strings.Repeat("é", 300)
	if got, _ := ids.take(3, str(long)).(string); len([]rune(got)) != 255 {
		t.Errorf("long globalId kept %d characters, want 255", len([]rune(got)))
	}
	if got := ids.take(3, str(strings.Repeat("é", 256))); got != nil {
		t.Errorf("globalIds equal once cut: %v, want nil", got)
	}
}
