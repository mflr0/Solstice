// Command devseed fills a development database with an administrator, mock
// users, a demo project and e-mail sent to Mailpit. It reads the database
// like "solstice serve" (SOLSTICE_DB_URL or solstice.conf), migrates it, and
// refuses a database that has accounts but no seed administrator.
//
//	go run ./cmd/devseed
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"solstice/internal/config"
	"solstice/internal/db"
	"solstice/internal/devseed"
)

func main() {
	smtp := flag.String("smtp", "localhost:1025", "Mailpit's SMTP address, set when no mail server is; empty to skip")
	flag.Parse()
	if err := run(*smtp); err != nil {
		fmt.Fprintln(os.Stderr, "devseed:", err)
		os.Exit(1)
	}
}

func run(smtp string) error {
	cfg, err := config.Load(os.Environ())
	if err != nil {
		return err
	}
	if cfg.DB.URL == "" {
		return fmt.Errorf("no database configured: set %s or write it in %s", config.KeyDBURL, cfg.Path)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	conn, err := db.Open(ctx, cfg.DB.URL, cfg.DB.Pool())
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		return err
	}
	if err := devseed.Run(ctx, conn, devseed.Options{SMTP: smtp, Out: os.Stdout}); err != nil {
		return err
	}
	fmt.Println("development database ready")
	return nil
}
