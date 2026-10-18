package notify

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// While a replacing import runs, the notification tick does nothing: it does
// not touch the database (there is none here, so touching it would panic).
func TestTicksSkippedWhilePaused(t *testing.T) {
	s := New(nil, nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)
	s.Paused = func() bool { return true }
	if err := s.FlushDue(context.Background()); err != nil {
		t.Errorf("FlushDue while paused: %v", err)
	}
	if err := s.RunSchedules(context.Background()); err != nil {
		t.Errorf("RunSchedules while paused: %v", err)
	}
	s.Paused = func() bool { return false }
	defer func() {
		if recover() == nil {
			t.Error("not paused: FlushDue did not reach the database")
		}
	}()
	s.FlushDue(context.Background())
}
