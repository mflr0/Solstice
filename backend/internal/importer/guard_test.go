package importer

import (
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
)

func TestGuardTurnsAPanicIntoAnError(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	var nilMap map[string]int
	rep, err := guard(log, func() (*Report, error) {
		nilMap["x"] = 1
		return &Report{}, nil
	})
	if err == nil || !strings.Contains(err.Error(), "import stopped by an internal error") {
		t.Fatalf("err = %v, want the internal error", err)
	}
	if rep != nil {
		t.Errorf("report = %v, want nil", rep)
	}
}

func TestGuardPassesTheResultThrough(t *testing.T) {
	want := errors.New("boom")
	rep, err := guard(nil, func() (*Report, error) { return &Report{}, want })
	if err != want || rep == nil {
		t.Fatalf("got %v, %v", rep, err)
	}
}
