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

// A boolean flag can say that a secret changed; its value is not one.
func TestMaskKeepsFlags(t *testing.T) {
	got := mask(map[string]any{"passwordChanged": true, "secretRemoved": false, "password": "hunter2"})
	want := map[string]any{"passwordChanged": true, "secretRemoved": false, "password": "***"}
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
