// Command solstice runs the Solstice server and its maintenance tasks.
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"solstice/internal/api"
	"solstice/internal/auth"
	"solstice/internal/db"
	"solstice/internal/importer"
	"solstice/internal/web"
)

const usage = `Usage : solstice <commande> [options]

Commandes :
  serve     démarre le serveur web
  migrate   met à jour le schéma de la base
  import    importe une base Jira (PostgreSQL ou MySQL)
  user      crée un compte ou change un mot de passe (solstice user add|passwd)

« solstice <commande> -h » détaille les options d'une commande.
Environnement : SOLSTICE_DB (URL MariaDB), SOLSTICE_DATA (dossier de données), SOLSTICE_ADDR (adresse d'écoute).
`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

func dbFlag(fs *flag.FlagSet) *string {
	return fs.String("db", env("SOLSTICE_DB", "mysql://solstice:solstice@localhost:13306/solstice"), "MariaDB URL")
}

func runMigrate(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("migrate", flag.ExitOnError)
	dbURL := dbFlag(fs)
	fs.Parse(args)

	conn, err := db.Open(ctx, *dbURL)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		return err
	}
	slog.Info("database is up to date")
	return nil
}

func runServe(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	dbURL := dbFlag(fs)
	dataDir := dataFlag(fs)
	addr := fs.String("addr", env("SOLSTICE_ADDR", ":8080"), "listen address")
	tzName := fs.String("tz", env("SOLSTICE_TZ", "Local"), "time zone of dates typed in searches, e.g. Europe/Paris")
	fs.Parse(args)
	tz, err := time.LoadLocation(*tzName)
	if err != nil {
		return fmt.Errorf("unknown time zone %q: %w", *tzName, err)
	}

	conn, err := db.Open(ctx, *dbURL)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		return err
	}
	log := slog.Default()
	srv := &http.Server{
		Addr:              *addr,
		Handler:           api.New(conn, *dataDir, tz, log).Handler(web.Handler()),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
		// No write timeout: live updates and large downloads stream for long.
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	go func() {
		// Expired sessions are useless; clear them hourly.
		tick := time.NewTicker(time.Hour)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				if _, err := conn.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < ?`, time.Now().UTC()); err != nil {
					log.Warn("session cleanup failed", "err", err)
				}
			}
		}
	}()
	log.Info("Solstice is listening", "addr", *addr)
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdown)
	}
}

func runUser(ctx context.Context, args []string) error {
	if len(args) == 0 || (args[0] != "add" && args[0] != "passwd") {
		return errors.New("usage: solstice user add -username … [-name …] [-email …] [-admin] | solstice user passwd -username …")
	}
	action := args[0]
	fs := flag.NewFlagSet("user "+action, flag.ExitOnError)
	dbURL := dbFlag(fs)
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
	generated := *password == ""
	if generated {
		raw := make([]byte, 12)
		if _, err := rand.Read(raw); err != nil {
			return err
		}
		*password = base64.RawURLEncoding.EncodeToString(raw)
	}
	conn, err := db.Open(ctx, *dbURL)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		return err
	}
	login := strings.ToLower(*username)
	switch action {
	case "add":
		hash, err := auth.HashPassword(*password)
		if err != nil {
			return err
		}
		display := *name
		if display == "" {
			display = login
		}
		if _, err := conn.ExecContext(ctx, `INSERT INTO users (username, display_name, email, password_hash, is_admin)
			VALUES (?, ?, ?, ?, ?)`, login, display, *email, hash, *admin); err != nil {
			return err
		}
		fmt.Printf("Compte « %s » créé.\n", login)
	case "passwd":
		var id int64
		if err := conn.QueryRowContext(ctx, `SELECT id FROM users WHERE username = ?`, login).Scan(&id); err != nil {
			return fmt.Errorf("unknown user %q", login)
		}
		if err := auth.SetPassword(ctx, conn, id, *password); err != nil {
			return err
		}
		// Sessions opened with the old password end now.
		if _, err := conn.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, id); err != nil {
			return err
		}
		if *admin {
			if _, err := conn.ExecContext(ctx, `UPDATE users SET is_admin = TRUE WHERE id = ?`, id); err != nil {
				return err
			}
		}
		fmt.Printf("Mot de passe de « %s » modifié.\n", login)
	}
	if generated {
		fmt.Printf("Mot de passe : %s\n", *password)
	}
	return nil
}

func dataFlag(fs *flag.FlagSet) *string {
	return fs.String("data", env("SOLSTICE_DATA", "data"), "data directory (attachments)")
}

func runImport(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	dbURL := dbFlag(fs)
	dataDir := dataFlag(fs)
	from := fs.String("from", "", "Jira database URL: postgres://user:pass@host/db or mysql://user:pass@host/db")
	tz := fs.String("tz", "", "time zone of the Jira server, e.g. Europe/Paris (see Jira › System info)")
	jiraHome := fs.String("jira-home", "", "Jira home directory, to copy attachment files")
	replace := fs.Bool("replace", false, "wipe Solstice's data before importing")
	asJSON := fs.Bool("json", false, "print the report as JSON")
	fs.Parse(args)
	if *from == "" || *tz == "" {
		fs.Usage()
		return errors.New("-from and -tz are required")
	}
	loc, err := time.LoadLocation(*tz)
	if err != nil {
		return fmt.Errorf("unknown time zone %q: %w", *tz, err)
	}

	conn, err := db.Open(ctx, *dbURL)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		return err
	}
	start := time.Now()
	rep, err := importer.Run(ctx, conn, importer.Options{
		SourceURL: *from,
		SourceTZ:  loc,
		JiraHome:  *jiraHome,
		DataDir:   *dataDir,
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
	fmt.Printf("\nImport terminé en %s.\n\n", time.Since(start).Round(time.Second))
	for _, s := range rep.Steps {
		line := fmt.Sprintf("  %-38s %8d", s.Name, s.Imported)
		if s.Skipped > 0 {
			line += fmt.Sprintf("   (%d ignorés : références orphelines ou non reprises)", s.Skipped)
		}
		fmt.Println(line)
	}
	if len(rep.Warnings) > 0 {
		fmt.Println("\nÀ vérifier :")
		for _, w := range rep.Warnings {
			fmt.Println("  - " + w)
		}
	}
	return nil
}
