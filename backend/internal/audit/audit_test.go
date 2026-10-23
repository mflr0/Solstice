package audit

import (
	"reflect"
	"testing"
)

func TestMaskHidesSecrets(t *testing.T) {
	got := mask(map[string]any{
		"host": "smtp.example.org", "password": "hunter2", "smtpPassword": "x", "apiKey": "k", "webhookSecret": "s",
		"token": "sol_x", "nested": map[string]any{"user": "u", "Passwd": "p"}, "count": 3,
	})
	want := map[string]any{
		"host": "smtp.example.org", "password": "***", "smtpPassword": "***", "apiKey": "***", "webhookSecret": "***",
		"token": "***", "nested": map[string]any{"user": "u", "Passwd": "***"}, "count": 3,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("mask:\n got %v\nwant %v", got, want)
	}
}

// A boolean flag can say that a secret changed, and a count how many ended;
// their values are not secrets. A number from JSON may be one.
func TestMaskKeepsFlags(t *testing.T) {
	got := mask(map[string]any{"passwordChanged": true, "secretRemoved": false, "password": "hunter2",
		"revokedAccessTokens": int64(2), "tokens": 3, "pin": map[string]any{"password": float64(1234)}})
	want := map[string]any{"passwordChanged": true, "secretRemoved": false, "password": "***",
		"revokedAccessTokens": int64(2), "tokens": 3, "pin": map[string]any{"password": "***"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("mask:\n got %v\nwant %v", got, want)
	}
}

func TestCut(t *testing.T) {
	if got := cut("épopée", 3); got != "épo" {
		t.Errorf("cut = %q", got)
	}
	if nullText("", 10) != nil || nullID(0) != nil {
		t.Error("empty values must be NULL")
	}
}
