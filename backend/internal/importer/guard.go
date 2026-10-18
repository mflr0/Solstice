package importer

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"runtime/debug"
)

// RunGuarded is Run, but a panic on bad source data comes back as an error
// instead of stopping the server. Use it for an import started in a goroutine.
func RunGuarded(ctx context.Context, target *sql.DB, opt Options) (*Report, error) {
	return guard(opt.Log, func() (*Report, error) { return Run(ctx, target, opt) })
}

func guard(log *slog.Logger, run func() (*Report, error)) (rep *Report, err error) {
	defer func() {
		if r := recover(); r != nil {
			if log == nil {
				log = slog.Default()
			}
			log.Error("import panicked", "panic", r, "stack", string(debug.Stack()))
			rep, err = nil, fmt.Errorf("import stopped by an internal error: %v", r)
		}
	}()
	return run()
}
