package api

import (
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestSummaryCarriesResolved(t *testing.T) {
	at := time.Date(2026, 10, 27, 13, 32, 0, 0, time.UTC)
	for _, c := range []struct {
		resolved sql.NullTime
		want     string
	}{
		{sql.NullTime{Time: at, Valid: true}, `"resolved":"2026-10-27T13:32:00Z"`},
		{sql.NullTime{}, `"resolved":null`},
	} {
		row := &issueRow{ProjectKey: "SOL", Num: 1, Resolved: c.resolved}
		out, err := json.Marshal(summarize(row, &catalog{}, nil))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(out), c.want) {
			t.Errorf("summary %s lacks %s", out, c.want)
		}
	}
}
