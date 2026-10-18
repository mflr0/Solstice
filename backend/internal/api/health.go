package api

import (
	"context"
	"database/sql"
	"net/http"
	"time"
)

// Health answers GET /healthz, for container checks and load balancers,
// without signing in: 200 {"status":"ok","version":…} when the database
// answers, 503 {"status":"unavailable",…} otherwise. "setup": true is added
// while the instance waits for its setup wizard. state gives the database in
// use (nil when none is configured yet) and whether setup mode is on.
func Health(version string, state func() (*sql.DB, bool)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, setup := state()
		body := map[string]any{"status": "ok", "version": version}
		if setup {
			body["setup"] = true
		}
		status := http.StatusOK
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if conn == nil || conn.PingContext(ctx) != nil {
			status = http.StatusServiceUnavailable
			body["status"] = "unavailable"
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, status, body)
	})
}
