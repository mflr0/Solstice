package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"solstice/internal/config"
)

// runHealth asks a running server whether it is healthy: exit status 0 when
// /healthz answers 200. Container images have no curl; this is their
// HEALTHCHECK.
func runHealth(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("health", flag.ExitOnError)
	target := fs.String("url", "", "URL of /healthz (by default derived from SOLSTICE_HTTP_ADDR)")
	_ = fs.Parse(args)

	url := *target
	if url == "" {
		cfg, err := config.Load(os.Environ())
		if err != nil {
			return err
		}
		host, port, err := net.SplitHostPort(cfg.HTTP.Addr)
		if err != nil {
			return fmt.Errorf("SOLSTICE_HTTP_ADDR %q: %w", cfg.HTTP.Addr, err)
		}
		if host == "" || host == "0.0.0.0" || host == "::" {
			host = "127.0.0.1"
		}
		url = "http://" + net.JoinHostPort(host, port) + "/healthz"
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", url, res.Status)
	}
	return nil
}
