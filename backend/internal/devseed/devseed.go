// Package devseed fills a development database: an administrator, mock users
// in every interface language, projects of every kind with their categories,
// components and versions, demo issues, and e-mail sent to a local Mailpit. Only cmd/devseed uses it, so its passwords never ship in
// the solstice binary.
package devseed

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"solstice/internal/api"
)

// Admin is the administrator the seed creates; its presence marks a
// database as a development one.
const (
	Admin         = "seed"
	AdminPassword = "seed-dev-only" // #nosec G101 -- development account, documented in the README
	UserPassword  = "dev-password"  // #nosec G101 -- development accounts, documented in the README
)

// Project is the key of the demo project, the one with issues.
const Project = "DEMO"

// ErrNotDev is returned for a database with accounts but no seed
// administrator: probably a real instance.
var ErrNotDev = errors.New("this database has accounts but no \"" + Admin + "\" administrator: " +
	"it does not look like a development database, nothing was changed")

// Options configures Run.
type Options struct {
	// SMTP is Mailpit's host:port, set as the mail server when none is.
	SMTP string
	// Out receives one line per thing created.
	Out io.Writer
}

// Run seeds conn, a migrated database. It only adds what is missing, so it
// can run on every start.
func Run(ctx context.Context, conn *sql.DB, opt Options) error {
	if opt.Out == nil {
		opt.Out = io.Discard
	}
	say := func(format string, args ...any) { fmt.Fprintf(opt.Out, format+"\n", args...) }
	var accounts, admins int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*), COUNT(CASE WHEN username = ? THEN 1 END) FROM users`,
		Admin).Scan(&accounts, &admins); err != nil {
		return err
	}
	if accounts > 0 && admins == 0 {
		return ErrNotDev
	}
	if admins == 0 {
		if err := createAdmin(ctx, conn); err != nil {
			return err
		}
		say("created the administrator %s / %s", Admin, AdminPassword)
	}

	dataDir, err := os.MkdirTemp("", "solstice-devseed")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dataDir)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := httptest.NewServer(api.New(conn, dataDir, time.UTC, log).Handler(http.NotFoundHandler()))
	defer srv.Close()
	admin, err := login(ctx, srv.URL, Admin, AdminPassword)
	if err != nil {
		return err
	}

	ids := map[string]int64{}
	for _, u := range Users {
		id, created, err := ensureUser(ctx, conn, admin, u)
		if err != nil {
			return fmt.Errorf("user %s: %w", u.Username, err)
		}
		ids[u.Username] = id
		if created {
			say("created the user %s / %s", u.Username, UserPassword)
		}
	}
	if opt.SMTP != "" {
		set, err := ensureMail(admin, opt.SMTP)
		if err != nil {
			return fmt.Errorf("mail settings: %w", err)
		}
		if set {
			say("e-mail goes to %s", opt.SMTP)
		}
	}
	if err := ensureProjects(ctx, conn, admin, ids, say); err != nil {
		return err
	}
	created, err := ensureIssues(ctx, conn, srv.URL, admin, ids)
	if err != nil {
		return fmt.Errorf("demo issues: %w", err)
	}
	if created > 0 {
		say("created %d issues in the project %s", created, Project)
	}
	return nil
}
