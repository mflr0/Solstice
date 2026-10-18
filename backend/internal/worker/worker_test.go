package worker

import (
	"bytes"
	"io"
	"log/slog"
	"strings"
	"testing"
)

func TestPanicDoesNotStopTheNextRun(t *testing.T) {
	var log bytes.Buffer
	runs := 0
	Guard(slog.New(slog.NewTextHandler(&log, nil)), "test", func() {
		runs++
		panic("one bad run")
	})
	Guard(slog.New(slog.NewTextHandler(io.Discard, nil)), "test", func() { runs++ })
	if runs != 2 {
		t.Fatalf("%d runs, want 2: the panic stopped the worker", runs)
	}
	if line := log.String(); !strings.Contains(line, "worker panic") || !strings.Contains(line, "worker=test") {
		t.Errorf("the panic was not logged as a worker panic:\n%s", line)
	}
}
