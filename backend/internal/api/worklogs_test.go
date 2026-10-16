package api

import (
	"encoding/json"
	"testing"
	"time"
)

func TestRESTWorklogBody(t *testing.T) {
	comment := "Revue"
	body, err := restWorklogInput{Comment: &comment, TimeSpent: "1h 30m", Started: "2026-10-14T09:00:00.000+0200"}.uiBody()
	if err != nil || body["seconds"] != int64(5400) || body["body"] != "Revue" {
		t.Fatalf("body: %v, %v", body, err)
	}
	if started, ok := body["started"].(time.Time); !ok || !started.Equal(time.Date(2026, 10, 14, 7, 0, 0, 0, time.UTC)) {
		t.Errorf("started: %v", body["started"])
	}
	// What is not given is not sent: an edit leaves it alone.
	body, err = restWorklogInput{TimeSpentSeconds: new(int64)}.uiBody()
	if err != nil || len(body) != 1 || body["seconds"] != int64(0) {
		t.Errorf("seconds only: %v, %v", body, err)
	}
	body, err = restWorklogInput{}.uiBody()
	if err != nil || len(body) != 0 {
		t.Errorf("empty: %v, %v", body, err)
	}
	for _, in := range []restWorklogInput{{TimeSpent: "much"}, {Started: "yesterday"},
		{Visibility: json.RawMessage(`{"type":"group","value":"x"}`)}} {
		if _, err := in.uiBody(); err == nil {
			t.Errorf("accepted %+v", in)
		}
	}
}
