package importer_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"solstice/internal/config"
	"solstice/internal/db"
	"solstice/internal/importer"
	"solstice/internal/jirafixture"
)

var update = flag.Bool("update", false, "rewrite testdata/characterization.json")

const goldenPath = "testdata/characterization.json"

// Not written by the import (keep.go), or changing at each run.
var skippedTables = []string{"schema_migrations", "sessions", "settings", "webhooks", "audit_log"}

// Set to the time of the import: Jira does not store them (as in scripts/e2e-import.sh).
var skippedColumns = map[string][]string{
	"users":   {"created_at"},
	"filters": {"created_at", "updated_at"},
	"boards":  {"created_at", "updated_at"},
}

// characterization is what an import of the synthetic Jira wrote.
type characterization struct {
	Tables   map[string]digest `json:"tables"`
	Files    digest            `json:"files"` // copied attachments
	Steps    []importer.Step   `json:"steps"`
	Warnings []string          `json:"warnings"`
}

type digest struct {
	Count  int    `json:"count"`
	SHA256 string `json:"sha256"`
}

// TestCharacterization imports the synthetic Jira of internal/jirafixture
// (the seed and size of scripts/e2e-import.sh) and compares what it wrote
// with testdata/characterization.json, rewritten with -update.
//
// It runs once per source named by SOLSTICE_TEST_JIRA_PG and
// SOLSTICE_TEST_JIRA_MY, whose Jira tables it recreates, and imports into
// SOLSTICE_TEST_IMPORT_DB, whose tables it drops first. JIRA_DIST is the Jira
// distribution, by default the one next to the repository.
//
//	SOLSTICE_TEST_JIRA_PG=postgres://jira:jira@localhost:15432/jira \
//	SOLSTICE_TEST_JIRA_MY=mysql://jira:jira@localhost:23306/jira \
//	SOLSTICE_TEST_IMPORT_DB=mysql://solstice:solstice@localhost:13306/solstice_import_t \
//	go test ./internal/importer -run Characterization -count=1
func TestCharacterization(t *testing.T) {
	sources := map[string]string{}
	for engine, env := range map[string]string{"pg": "SOLSTICE_TEST_JIRA_PG", "my": "SOLSTICE_TEST_JIRA_MY"} {
		if u := os.Getenv(env); u != "" {
			sources[engine] = u
		}
	}
	if len(sources) == 0 {
		t.Skip("neither SOLSTICE_TEST_JIRA_PG nor SOLSTICE_TEST_JIRA_MY set: characterization test skipped")
	}
	target := os.Getenv("SOLSTICE_TEST_IMPORT_DB")
	if target == "" {
		t.Fatal("a Jira source is set but SOLSTICE_TEST_IMPORT_DB is not")
	}
	dist := jiraDist(t)

	got := map[string]*characterization{}
	for _, engine := range slices.Sorted(maps.Keys(sources)) {
		t.Run(engine, func(t *testing.T) {
			c := characterize(t, sources[engine], target, dist)
			got[engine] = c
			if !*update {
				compare(t, c)
			}
		})
	}
	if *update && !t.Failed() {
		write(t, got)
	}
}

func jiraDist(t *testing.T) string {
	t.Helper()
	if d := os.Getenv("JIRA_DIST"); d != "" {
		return d
	}
	found, _ := filepath.Glob("../../../atlassian-jira-software-*-standalone")
	if len(found) == 0 {
		t.Fatal("no Jira distribution next to the repository: set JIRA_DIST")
	}
	return found[0]
}

func characterize(t *testing.T, source, targetURL, dist string) *characterization {
	t.Helper()
	ctx := t.Context()
	tz, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		t.Fatal(err)
	}
	home, data := t.TempDir(), t.TempDir()
	if _, err := jirafixture.Run(ctx, jirafixture.Options{
		URL: source, JiraDist: dist, JiraHome: home, Issues: 300, Seed: 42, TZ: tz,
	}); err != nil {
		t.Fatalf("generating the Jira fixture: %v", err)
	}

	target, err := db.Open(ctx, targetURL, config.Defaults().DB.Pool())
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	// A fresh database each run: Replace keeps users' rows and settings.
	if err := dropTables(ctx, target); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(ctx, target); err != nil {
		t.Fatal(err)
	}
	rep, err := importer.Run(ctx, target, importer.Options{
		SourceURL: source, SourceTZ: tz, JiraHome: home, DataDir: data,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	c := &characterization{Tables: map[string]digest{}, Steps: rep.Steps, Warnings: rep.Warnings}
	tables, err := baseTables(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range tables {
		if slices.Contains(skippedTables, table) {
			continue
		}
		if c.Tables[table], err = tableDigest(ctx, target, table); err != nil {
			t.Fatalf("%s: %v", table, err)
		}
	}
	if c.Files, err = filesDigest(filepath.Join(data, "attachments")); err != nil {
		t.Fatal(err)
	}
	return c
}

func dropTables(ctx context.Context, target *sql.DB) error {
	tables, err := baseTables(ctx, target)
	if err != nil {
		return err
	}
	conn, err := target.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "SET SESSION foreign_key_checks = 0"); err != nil {
		return err
	}
	for _, table := range tables {
		if _, err := conn.ExecContext(ctx, "DROP TABLE `"+table+"`"); err != nil {
			return err
		}
	}
	_, err = conn.ExecContext(ctx, "SET SESSION foreign_key_checks = 1")
	return err
}

func baseTables(ctx context.Context, target *sql.DB) ([]string, error) {
	return queryStrings(ctx, target, `SELECT table_name FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE' ORDER BY table_name`)
}

// tableDigest hashes the column names, then the table's rows sorted, each
// value quoted and NULL written apart.
func tableDigest(ctx context.Context, target *sql.DB, table string) (digest, error) {
	all, err := queryStrings(ctx, target, `SELECT column_name FROM information_schema.columns
		WHERE table_schema = DATABASE() AND table_name = ? ORDER BY ordinal_position`, table)
	if err != nil {
		return digest{}, err
	}
	var cols []string
	for _, c := range all {
		if !slices.Contains(skippedColumns[table], c) {
			cols = append(cols, "`"+c+"`")
		}
	}
	rows, err := target.QueryContext(ctx, "SELECT "+strings.Join(cols, ", ")+" FROM `"+table+"`")
	if err != nil {
		return digest{}, err
	}
	defer rows.Close()
	var lines []string
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return digest{}, err
		}
		parts := make([]string, len(vals))
		for i, v := range vals {
			parts[i] = encode(v)
		}
		lines = append(lines, strings.Join(parts, "|"))
	}
	if err := rows.Err(); err != nil {
		return digest{}, err
	}
	slices.Sort(lines)
	h := sha256.New()
	fmt.Fprintln(h, strings.Join(cols, "|"))
	for _, l := range lines {
		fmt.Fprintln(h, l)
	}
	return digest{Count: len(lines), SHA256: hex.EncodeToString(h.Sum(nil))}, nil
}

func encode(v any) string {
	switch v := v.(type) {
	case nil:
		return `\N`
	case []byte:
		return strconv.Quote(string(v))
	case time.Time:
		return v.UTC().Format(time.RFC3339Nano)
	default:
		return strconv.Quote(fmt.Sprint(v))
	}
}

// filesDigest hashes the relative path and contents of every file under dir.
func filesDigest(dir string) (digest, error) {
	var lines []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		lines = append(lines, filepath.ToSlash(rel)+" "+hex.EncodeToString(sum[:]))
		return nil
	})
	if err != nil {
		return digest{}, err
	}
	slices.Sort(lines)
	h := sha256.New()
	for _, l := range lines {
		fmt.Fprintln(h, l)
	}
	return digest{Count: len(lines), SHA256: hex.EncodeToString(h.Sum(nil))}, nil
}

func queryStrings(ctx context.Context, q *sql.DB, query string, args ...any) ([]string, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func compare(t *testing.T, got *characterization) {
	t.Helper()
	b, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("%v (run with -update to write it)", err)
	}
	var want characterization
	if err := json.Unmarshal(b, &want); err != nil {
		t.Fatal(err)
	}
	tables := maps.Clone(want.Tables)
	maps.Copy(tables, got.Tables)
	for _, table := range slices.Sorted(maps.Keys(tables)) {
		if g, w := got.Tables[table], want.Tables[table]; g != w {
			t.Errorf("table %s: got %+v, want %+v", table, g, w)
		}
	}
	if got.Files != want.Files {
		t.Errorf("attachment files: got %+v, want %+v", got.Files, want.Files)
	}
	if !reflect.DeepEqual(got.Steps, want.Steps) {
		t.Errorf("steps:\n got %+v\nwant %+v", got.Steps, want.Steps)
	}
	if !reflect.DeepEqual(got.Warnings, want.Warnings) {
		t.Errorf("warnings:\n got %q\nwant %q", got.Warnings, want.Warnings)
	}
}

// write records the result, once every engine gave the same one.
func write(t *testing.T, got map[string]*characterization) {
	t.Helper()
	var first *characterization
	for engine, c := range got {
		if first == nil {
			first = c
		} else if !reflect.DeepEqual(c, first) {
			t.Fatalf("the engines disagree (%s differs): nothing written", engine)
		}
	}
	b, err := json.MarshalIndent(first, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(goldenPath, append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}
