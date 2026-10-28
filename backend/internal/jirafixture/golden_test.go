package jirafixture

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"fmt"
	"hash"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"solstice/internal/jiradb"
)

// goldenRows is, per Jira version, the hash of the rows generated with the
// seed, size and time zone of scripts/e2e-import.sh. Change it only for an
// intended change of the generated data.
var goldenRows = map[string]string{
	"7.13.18": "e2f3b631fec635e4f5ae863ded7637190cded72d538568c8eece0e702482ace0",
	"8.20.30": "00b4c18967f1b15af8f7d8f1ec40f115d01630575a1f647535b2f59e139b405f",
	"9.12.25": "00b4c18967f1b15af8f7d8f1ec40f115d01630575a1f647535b2f59e139b405f",
	"10.3.12": "3b74bcf1c0ed7629b994115553c047e4110c0ca78b53f73b7172b9e85160c175",
	"11.3.12": "8e3e6019369985253754c2fa0a952b834e0c49ff31b14d1897433d92c6d4937c",
}

// TestGoldenRows runs the generator against a connection that records every
// insert and hashes the rows in write order: the same seed must give the
// same rows, on either engine, from one run to the next. It runs for each
// distribution found, as scripts/e2e-import.sh finds them.
func TestGoldenRows(t *testing.T) {
	var dists []string
	if dist := os.Getenv("JIRA_DIST"); dist != "" {
		dists = []string{dist}
	} else {
		cache := os.Getenv("JIRA_ENTITYDEFS")
		if cache == "" {
			cache = "../../../../jira-entitydefs"
		}
		dists, _ = filepath.Glob(filepath.Join(cache, distGlob))
		next, _ := filepath.Glob(filepath.Join("../../..", distGlob))
		dists = append(dists, next...)
	}
	if len(dists) == 0 {
		t.Skip("no Jira distribution: run scripts/jira-entitydefs.sh, or set JIRA_DIST")
	}
	tz, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		t.Fatal(err)
	}
	opt := Options{Issues: 300, Seed: 42, TZ: tz}

	for _, dist := range dists {
		version := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(dist), "atlassian-jira-software-"), "-standalone")
		t.Run(version, func(t *testing.T) {
			want, ok := goldenRows[version]
			if !ok {
				t.Fatalf("no golden hash for Jira %s (%s)", version, dist)
			}
			pg := generateHash(t, dist, jiradb.Postgres, opt)
			if again := generateHash(t, dist, jiradb.Postgres, opt); again != pg {
				t.Fatalf("two runs give different rows: %s, then %s", pg, again)
			}
			if my := generateHash(t, dist, jiradb.MySQL, opt); my != pg {
				t.Errorf("MySQL rows hash to %s, PostgreSQL rows to %s", my, pg)
			}
			if pg != want {
				t.Errorf("rows hash to %s, want %s", pg, want)
			}
		})
	}
}

const distGlob = "atlassian-jira-software-*-standalone"

func generateHash(t *testing.T, dist string, eng jiradb.Engine, opt Options) string {
	t.Helper()
	model, _, err := LoadDistribution(dist, eng)
	if err != nil {
		t.Fatal(err)
	}
	rec := &recorder{h: sha256.New()}
	db := sql.OpenDB(rec)
	defer db.Close()
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	opt.JiraHome = t.TempDir()
	w := NewWriter(ctx, tx, eng, model)
	newGenerator(opt, w, &Summary{}).run()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if rec.rows == 0 {
		t.Fatal("no row written")
	}
	return hex.EncodeToString(rec.h.Sum(nil))
}

// recorder is a database/sql connector whose connection hashes each inserted
// row as its table, its columns and its values, without the engine's quoting.
type recorder struct {
	h    hash.Hash
	rows int
}

func (r *recorder) Connect(context.Context) (driver.Conn, error) { return r, nil }
func (r *recorder) Driver() driver.Driver                        { return nil }
func (r *recorder) Prepare(string) (driver.Stmt, error) {
	return nil, fmt.Errorf("prepare is not recorded")
}
func (r *recorder) Close() error              { return nil }
func (r *recorder) Begin() (driver.Tx, error) { return r, nil }
func (r *recorder) Commit() error             { return nil }
func (r *recorder) Rollback() error           { return nil }

func (r *recorder) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	head, _, ok := strings.Cut(query, " VALUES ")
	table, cols, ok2 := strings.Cut(strings.TrimPrefix(head, "INSERT INTO "), " (")
	if !ok || !ok2 || !strings.HasPrefix(query, "INSERT INTO ") {
		return nil, fmt.Errorf("not an insert: %.80s", query)
	}
	unquote := strings.NewReplacer(`"`, "", "`", "")
	table = strings.ToLower(unquote.Replace(table))
	cols = strings.ToLower(unquote.Replace(strings.TrimSuffix(cols, ")")))
	ncols := strings.Count(cols, ",") + 1
	if len(args)%ncols != 0 {
		return nil, fmt.Errorf("%s: %d values for %d columns", table, len(args), ncols)
	}
	for i := 0; i < len(args); i += ncols {
		var b strings.Builder
		b.WriteString(table + "\t" + cols)
		for _, a := range args[i : i+ncols] {
			b.WriteByte('\t')
			b.WriteString(format(a.Value))
		}
		b.WriteByte('\n')
		r.h.Write([]byte(b.String()))
		r.rows++
	}
	return driver.RowsAffected(len(args) / ncols), nil
}

// format writes a driver value with its type, so that 1, "1" and 1.0 differ.
func format(v driver.Value) string {
	switch v := v.(type) {
	case nil:
		return "null"
	case string:
		return strconv.Quote(v)
	case []byte:
		return "x" + hex.EncodeToString(v)
	case time.Time:
		return "t" + v.Format(time.RFC3339Nano)
	default:
		return fmt.Sprintf("%T:%v", v, v)
	}
}
