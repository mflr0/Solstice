// Command jira-fixture fills a PostgreSQL or MySQL database with a
// synthetic Jira Data Center instance, for testing Solstice's importer.
//
//	go run ./cmd/jira-fixture -url postgres://jira:jira@localhost:15432/jira \
//	    -jira-dist ../atlassian-jira-software-11.3.12-standalone -jira-home /tmp/jira-home
//
// It prints a JSON summary of what it generated on stdout.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"solstice/internal/jirafixture"
)

func main() {
	var opt jirafixture.Options
	flag.StringVar(&opt.URL, "url", "", "target database: postgres://… or mysql://user:pass@host:port/db")
	flag.StringVar(&opt.JiraDist, "jira-dist", "", "local Jira distribution, read for its schema")
	flag.StringVar(&opt.JiraHome, "jira-home", "", "directory that receives data/attachments/…")
	flag.IntVar(&opt.Issues, "issues", 300, "number of issues")
	flag.Uint64Var(&opt.Seed, "seed", 42, "random seed; the same seed gives the same data")
	tz := flag.String("tz", "Europe/Paris", "time zone of the simulated Jira server")
	flag.Parse()

	loc, err := time.LoadLocation(*tz)
	if err != nil {
		fail(err)
	}
	opt.TZ = loc
	opt.Log = os.Stderr

	sum, err := jirafixture.Run(context.Background(), opt)
	if err != nil {
		fail(err)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(sum); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "jira-fixture:", err)
	os.Exit(1)
}
