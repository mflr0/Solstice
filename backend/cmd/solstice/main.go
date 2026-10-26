// Command solstice runs the Solstice server and its maintenance tasks.
package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"maps"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	_ "time/tzdata" // time zones even on systems (containers) without a zoneinfo database

	"solstice/internal/api"
	"solstice/internal/audit"
	"solstice/internal/auth"
	"solstice/internal/db"
	"solstice/internal/directory"
	"solstice/internal/importer"
	"solstice/internal/web"
	"solstice/internal/worker"
)

// #nosec G101 -- usage text that names settings, not credentials
const usage = `Usage: solstice <command> [options]

Commands:
  serve     starts the web server
  migrate   updates the database schema
  import    imports a Jira database (PostgreSQL or MySQL); "import -check" audits it first
  user      creates an account or changes a password (solstice user add|passwd)
  version   prints the version
  health    checks that a running server answers (container health check)

"solstice <command> -h" lists a command's options.

Configuration: a file of KEY=value lines, named by SOLSTICE_CONFIG, otherwise
./solstice.conf, otherwise /etc/solstice/solstice.conf. Environment variables
of the same name win over the file. Durations are written 30s, 5m, 1h30m.
  SOLSTICE_DB_URL                MariaDB database: mysql://user:password@host:3306/database
                                 add ?tls=true (or skip-verify, preferred) to encrypt the link
  SOLSTICE_DB_MAXOPENCONNS       open connections at most (default 20)
  SOLSTICE_DB_MAXIDLECONNS       idle connections kept (default 10)
  SOLSTICE_DB_CONNMAXLIFETIME    lifetime of a connection (default 5m)
  SOLSTICE_DATA                  data directory (default ./data)
  SOLSTICE_TZ                    time zone of dates typed in searches, e.g. Europe/Paris
  SOLSTICE_SECRETKEY             key of the stored secrets (default <data>/secret.key)
  SOLSTICE_INSTALL               "docker" in a container image
  SOLSTICE_HTTP_ADDR             listen address (default :8080)
  SOLSTICE_HTTP_BASEURL          public address, e.g. https://tickets.example.org
  SOLSTICE_HTTP_TRUSTEDPROXIES   reverse proxies in front of Solstice: IP addresses and
                                 CIDR ranges, e.g. 127.0.0.1,10.0.0.0/8 (default none)
  SOLSTICE_HTTP_SHUTDOWNTIMEOUT  time given to work in progress on stop (default 10s)
  SOLSTICE_HTTP_MAXUPLOAD        size of one upload, e.g. 100MB or 1GiB (default 100MiB)
  SOLSTICE_HTTP_MAXSTORAGE       total size of all attachments, 0 for no limit, e.g. 10GiB (default 0)
  SOLSTICE_LOG_LEVEL             debug, info, warn or error (default info)
  SOLSTICE_LOG_FORMAT            text or json (default text)
  SOLSTICE_LDAP_URL              LDAP or Active Directory sign-in, off when empty: ldaps://host,
                                 or ldap://host with STARTTLS (default none)
  SOLSTICE_LDAP_STARTTLS         switch ldap:// to TLS first (default false)
  SOLSTICE_LDAP_INSECURE         allow ldap:// without TLS, passwords in clear (default false)
  SOLSTICE_LDAP_CAFILE           PEM file of certificate authorities to trust as well
  SOLSTICE_LDAP_BINDDN           service account that searches, anonymous when empty
  SOLSTICE_LDAP_BINDPASSWORD     its password
  SOLSTICE_LDAP_USERBASE         where people are searched, e.g. ou=people,dc=example,dc=org
  SOLSTICE_LDAP_USERFILTER       how; {username} is the name typed; defaults to (uid={username})
  SOLSTICE_LDAP_NAMEATTR         attribute of the account's name (default displayName)
  SOLSTICE_LDAP_EMAILATTR        attribute of its e-mail address (default mail)
  SOLSTICE_LDAP_GROUPBASE        where groups are searched (default <the user base>)
  SOLSTICE_LDAP_GROUPFILTER      how; {dn} is the person's DN; defaults to (member={dn})
  SOLSTICE_LDAP_GROUPNAMEATTR    attribute naming a group (default cn)
  SOLSTICE_LDAP_REQUIREGROUP     only its members sign in through LDAP (default none)
  SOLSTICE_LDAP_ADMINGROUP       its members are administrators, and only they (default none)
  SOLSTICE_LDAP_TIMEOUT          time a sign-in waits for the server (default 10s)

Without a configured database, or on a database without any account, "solstice serve"
opens the setup wizard at /setup.
`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// Once shutting down, a second Ctrl+C kills the process.
	go func() { <-ctx.Done(); stop() }()

	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	var err error
	switch cmd, args := os.Args[1], os.Args[2:]; cmd {
	case "serve":
		err = runServe(ctx, args)
	case "migrate":
		err = runMigrate(ctx, args)
	case "import":
		err = runImport(ctx, args)
	case "user":
		err = runUser(ctx, args)
	case "version", "-version", "--version":
		fmt.Println(version)
		return
	case "health":
		err = runHealth(ctx, args)
	case "-h", "--help", "help":
		fmt.Print(usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", cmd, usage)
		os.Exit(2)
	}
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

func runMigrate(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("migrate", flag.ExitOnError)
	fs.Parse(args)
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	conn, err := openMigrated(ctx, cfg)
	if err != nil {
		return err
	}
	conn.Close()
	slog.Info("database is up to date")
	return nil
}

func runServe(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	fs.Parse(args)
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	tz, tzName := time.Local, ""
	if cfg.TZ != nil {
		tz, tzName = cfg.TZ, cfg.TZ.String()
	}
	dataDir, err := filepath.Abs(cfg.Data)
	if err != nil {
		return err
	}
	log := slog.Default()
	log.Info("Solstice is starting", "version", version, "config", cfg.Path, "config_found", cfg.Exists, "data", dataDir)
	go auth.PrepareSignIn()
	dir, err := directory.New(cfg.LDAP)
	if err != nil {
		return err
	}
	if dir != nil {
		log.Info("LDAP sign-in is on", "url", cfg.LDAP.URL)
	}
	// Bound first: a taken port fails here, before a setup code is printed.
	ln, err := net.Listen("tcp", cfg.HTTP.Addr)
	if err != nil {
		return err
	}
	defer ln.Close()

	// Without a database, or on one without any account, the setup wizard
	// runs first; the application takes over when it is done.
	var conn *sql.DB
	if cfg.DB.URL != "" {
		if conn, err = openDB(ctx, cfg.DB.URL, cfg.DB.Pool()); err != nil {
			return err
		}
		if err := db.Migrate(ctx, conn); err != nil {
			return err
		}
	}
	setup := conn == nil
	if !setup {
		var accounts int
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&accounts); err != nil {
			return err
		}
		setup = accounts == 0
	}

	static := web.Handler()
	var (
		current atomic.Pointer[http.Handler] // the setup wizard, then the application
		appDB   atomic.Pointer[sql.DB]       // the application's database, once it runs
		app     atomic.Pointer[api.Server]   // the application, once it runs
		wizard  *api.Setup
		workers sync.WaitGroup // e-mails and webhooks
	)
	// startApp opens the application. Anything that needs the database in
	// the background starts here: after setup, it runs without a restart.
	startApp := func(conn *sql.DB, tz *time.Location, baseURL string) {
		srv := api.New(conn, dataDir, tz, log)
		srv.SetBaseURL(baseURL)
		srv.SetTrustedProxies(cfg.HTTP.TrustedProxies)
		srv.SetSecretKey(cfg.SecretKey)
		srv.SetMaxUpload(int64(cfg.HTTP.MaxUpload))
		srv.SetMaxStorage(int64(cfg.HTTP.MaxStorage))
		if dir != nil {
			srv.SetDirectory(dir)
		}
		srv.SetInstall(cfg.Install)
		srv.SetVersion(version)
		srv.SetConfigInfo(api.ConfigInfo{Version: version, Path: cfg.Path, Found: cfg.Exists, Addr: cfg.HTTP.Addr})
		h := srv.Handler(static)
		appDB.Store(conn)
		app.Store(srv)
		current.Store(&h)
		go purgeExpired(ctx, conn, log)
		workers.Go(func() { srv.Run(ctx) })
	}
	if setup {
		wizard, err = api.NewSetup(ctx, api.SetupOptions{
			DB: conn, Pool: cfg.DB.Pool(),
			DataDir:        dataDir,
			ConfigPath:     cfg.Path,
			TrustedProxies: cfg.HTTP.TrustedProxies,
			BaseURL:        strings.TrimSuffix(cfg.HTTP.BaseURL, "/"),
			TZ:             tzName,
			Version:        version, Log: log,
			Done: func(res api.SetupResult) {
				tz, err := loadTZ(res.TZ)
				if err != nil {
					tz = time.Local
				}
				startApp(res.DB, tz, res.BaseURL)
				log.Info("Solstice is set up: the application is open")
			},
		})
		if err != nil {
			return err
		}
		h := wizard.Handler(static, web.IsAsset)
		current.Store(&h)
		fmt.Fprint(os.Stderr, wizard.Banner(cfg.HTTP.Addr))
	} else {
		startApp(conn, tz, cfg.HTTP.BaseURL)
	}
	defer func() {
		if c := appDB.Load(); c != nil {
			c.Close()
		} else if wizard != nil && wizard.DB() != nil {
			wizard.DB().Close()
		}
	}()

	root := http.NewServeMux()
	root.Handle("GET /healthz", api.Health(version, func() (*sql.DB, bool) {
		if c := appDB.Load(); c != nil {
			return c, false
		}
		return wizard.DB(), true
	}))
	root.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		(*current.Load()).ServeHTTP(w, r)
	}))
	srv := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           root,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
		// No write timeout: live updates and large downloads stream for long.
	}
	srv.RegisterOnShutdown(func() {
		if a := app.Load(); a != nil {
			a.CloseStreams()
		}
	})
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	log.Info("Solstice is listening", "addr", cfg.HTTP.Addr, "setup", setup)
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
		defer cancel()
		err := srv.Shutdown(shutdown)
		// Requests are over: hand their last events to e-mail and webhooks,
		// and let the workers finish what they are sending.
		if a := app.Load(); a != nil {
			a.Close(shutdown)
		}
		stopped := make(chan struct{})
		go func() { workers.Wait(); close(stopped) }()
		select {
		case <-stopped:
		case <-shutdown.Done():
		}
		return err
	}
}

// purgeExpired deletes expired sessions hourly: they are useless. Audit
// log entries past their retention period, and expired or used reset and
// invitation links, go at the same time.
func purgeExpired(ctx context.Context, conn *sql.DB, log *slog.Logger) {
	tick := time.NewTicker(time.Hour)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			worker.Guard(log, "sessions", func() {
				if _, err := conn.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < ?`, time.Now().UTC()); err != nil {
					log.Warn("session cleanup failed", "err", err)
				}
				if _, err := audit.Purge(ctx, conn); err != nil {
					log.Warn("audit log purge failed", "err", err)
				}
				if _, err := auth.PurgeAccountTokens(ctx, conn, time.Now()); err != nil {
					log.Warn("expired reset and invitation links purge failed", "err", err)
				}
			})
		}
	}
}

func runUser(ctx context.Context, args []string) error {
	if len(args) == 0 || (args[0] != "add" && args[0] != "passwd") {
		return errors.New("usage: solstice user add -username … [-name …] [-email …] [-admin] | solstice user passwd -username …")
	}
	action := args[0]
	fs := flag.NewFlagSet("user "+action, flag.ExitOnError)
	username := fs.String("username", "", "login")
	name := fs.String("name", "", "display name")
	email := fs.String("email", "", "e-mail address")
	admin := fs.Bool("admin", false, "make the account an administrator")
	password := fs.String("password", "", "password (generated and printed when empty)")
	fs.Parse(args[1:])
	if *username == "" {
		fs.Usage()
		return errors.New("-username is required")
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	conn, err := openMigrated(ctx, cfg)
	if err != nil {
		return err
	}
	defer conn.Close()
	minPassword := api.StoredPasswordMin(ctx, conn)
	generated := *password == ""
	if generated {
		n := max(minPassword, 16)
		raw := make([]byte, n)
		if _, err := rand.Read(raw); err != nil {
			return err
		}
		*password = base64.RawURLEncoding.EncodeToString(raw)[:n]
	}
	acc, err := checkAccount(action, account{login: *username, name: *name, email: *email, password: *password}, minPassword)
	if err != nil {
		return err
	}
	login := acc.login
	// record writes an audit entry for a change made from the command line.
	record := func(what string, id int64, details map[string]any) error {
		d := map[string]any{"via": "command line"}
		maps.Copy(d, details)
		return audit.Record(ctx, conn, audit.Entry{
			Action:  what,
			Target:  audit.Target{Type: audit.TargetUser, ID: strconv.FormatInt(id, 10), Label: login},
			Details: d,
		})
	}
	switch action {
	case "add":
		hash, err := auth.HashPassword(acc.password)
		if err != nil {
			return err
		}
		res, err := conn.ExecContext(ctx, `INSERT INTO users (username, display_name, email, password_hash, is_admin)
			VALUES (?, ?, ?, ?, ?)`, login, acc.name, acc.email, hash, *admin)
		if api.IsDuplicate(err) {
			return fmt.Errorf("an account named %q already exists", login)
		}
		if err != nil {
			return err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		if err := record(audit.UserCreate, id, nil); err != nil {
			return err
		}
		if *admin {
			if err := record(audit.AdminGrant, id, nil); err != nil {
				return err
			}
		}
		fmt.Printf("Account \"%s\" created.\n", login)
	case "passwd":
		var id int64
		if err := conn.QueryRowContext(ctx, `SELECT id FROM users WHERE username = ?`, login).Scan(&id); err != nil {
			return fmt.Errorf("unknown user %q", login)
		}
		hash, err := auth.HashPassword(acc.password)
		if err != nil {
			return err
		}
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		revoked, err := auth.SetPasswordHash(ctx, tx, id, hash, "")
		if err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		if err := record(audit.PasswordChange, id, map[string]any{"revokedAccessTokens": revoked}); err != nil {
			return err
		}
		if *admin {
			if _, err := conn.ExecContext(ctx, `UPDATE users SET is_admin = TRUE WHERE id = ?`, id); err != nil {
				return err
			}
			if err := record(audit.AdminGrant, id, nil); err != nil {
				return err
			}
		}
		fmt.Printf("Password for \"%s\" changed.\n", login)
	}
	if generated {
		fmt.Printf("Password: %s\n", acc.password)
	}
	return nil
}

// account is what solstice user stores.
type account struct {
	login, name, email, password string
}

// checkAccount applies the API's account rules to the command's input: to
// the login, name and e-mail of a new account (add), and to the password
// against minPassword, the instance's minimum length. It returns the input
// as stored.
func checkAccount(action string, in account, minPassword int) (account, error) {
	in.login = strings.ToLower(strings.TrimSpace(in.login))
	if action == "add" {
		if !api.ValidUsername(in.login) {
			return account{}, fmt.Errorf("invalid -username %q: use at most 100 lowercase letters, digits and . _ @ -, starting with a letter or digit", in.login)
		}
		if in.name = strings.TrimSpace(in.name); in.name == "" {
			in.name = in.login
		}
		if in.email = strings.TrimSpace(in.email); in.email != "" && !api.ValidEmail(in.email) {
			return account{}, fmt.Errorf("invalid -email %q: give a bare address, such as marie@example.org", in.email)
		}
	}
	if err := api.PasswordRules(in.password, in.login, minPassword); err != nil {
		return account{}, err
	}
	return in, nil
}

func runImport(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	from := fs.String("from", "", "Jira database URL: postgres://user:pass@host/db or mysql://user:pass@host/db")
	tz := fs.String("tz", "", "time zone of the Jira server, e.g. Europe/Paris (see Jira › System info); with -check, UTC if omitted")
	jiraHome := fs.String("jira-home", "", "Jira home directory, to copy attachment files (with -check: to look for them)")
	replace := fs.Bool("replace", false, "wipe Solstice's data before importing")
	check := fs.Bool("check", false, "audit the Jira database without importing: what there is, what would be left out and why; "+
		"writes nothing, needs no Solstice database, exits with status 1 if the import would refuse the source, 2 if it cannot read it")
	asJSON := fs.Bool("json", false, "print the report as JSON")
	fs.Parse(args)
	if *from == "" || (*tz == "" && !*check) {
		fs.Usage()
		if *check {
			return errors.New("-from is required")
		}
		return errors.New("-from and -tz are required")
	}
	if *check && *replace {
		return errors.New("-replace has no meaning with -check, which writes nothing")
	}
	var loc *time.Location
	if *tz != "" {
		var err error
		if loc, err = time.LoadLocation(*tz); err != nil {
			return fmt.Errorf("unknown time zone %q: %w", *tz, err)
		}
	}
	if *check {
		rep, err := importer.Check(ctx, importer.Options{SourceURL: *from, SourceTZ: loc, JiraHome: *jiraHome})
		if err != nil { // not a refusal: the source could not be read
			slog.Error(err.Error())
			os.Exit(2)
		}
		if *asJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(rep); err != nil {
				return err
			}
		} else {
			rep.WriteText(os.Stdout)
		}
		if rep.Refused != "" {
			return errors.New("the import would refuse this source")
		}
		return nil
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	conn, err := openMigrated(ctx, cfg)
	if err != nil {
		return err
	}
	defer conn.Close()
	start := time.Now()
	rep, err := importer.Run(ctx, conn, importer.Options{
		SourceURL: *from,
		SourceTZ:  loc,
		JiraHome:  *jiraHome,
		DataDir:   cfg.Data,
		Replace:   *replace,
	})
	if err != nil {
		return err
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(rep)
	}
	fmt.Printf("\nImport finished in %s.\n\n", time.Since(start).Round(time.Second))
	for _, s := range rep.Steps {
		line := fmt.Sprintf("  %-38s %8d", s.Name, s.Imported)
		if s.Skipped > 0 {
			line += fmt.Sprintf("   (%d skipped: orphaned references or not carried over)", s.Skipped)
		}
		fmt.Println(line)
	}
	if len(rep.Warnings) > 0 {
		fmt.Println("\nCheck:")
		for _, w := range rep.Warnings {
			fmt.Println("  - " + w)
		}
	}
	return nil
}
