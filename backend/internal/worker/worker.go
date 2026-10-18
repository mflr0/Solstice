// Package worker runs background work that must survive a panic in one run.
package worker

import (
	"log/slog"
	"runtime/debug"
)

// Guard runs f once. A panic inside f is logged with its stack and does not
// escape, so a worker's loop goes on with its next run.
func Guard(log *slog.Logger, name string, f func()) {
	defer func() {
		if v := recover(); v != nil {
			log.Error("worker panic", "worker", name, "panic", v, "stack", string(debug.Stack()))
		}
	}()
	f()
}
