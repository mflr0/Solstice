package importer_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"maps"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"solstice/internal/importer"
	"solstice/internal/jiradb"
	"solstice/internal/jirafixture"
)

// TestCheckAnonymous checks the synthetic Jira of internal/jirafixture, with
// the quirks of old instances, twice: the anonymous report names no project,
// workflow, user or custom field of it, nor the database server's version,
// and counts what the normal one counts.
// It runs on the sources of TestCharacterization, whose Jira tables it
// recreates.
func TestCheckAnonymous(t *testing.T) {
	sources := map[string]string{}
	for engine, env := range map[string]string{"pg": "SOLSTICE_TEST_JIRA_PG", "my": "SOLSTICE_TEST_JIRA_MY"} {
		if u := os.Getenv(env); u != "" {
			sources[engine] = u
		}
	}
	if len(sources) == 0 {
		t.Skip("neither SOLSTICE_TEST_JIRA_PG nor SOLSTICE_TEST_JIRA_MY set: anonymous check test skipped")
	}
	dist := jiraDist(t)
	for _, engine := range slices.Sorted(maps.Keys(sources)) {
		t.Run(engine, func(t *testing.T) { checkAnonymous(t, sources[engine], dist) })
	}
}

func checkAnonymous(t *testing.T, source, dist string) {
	ctx := t.Context()
	home := t.TempDir()
	if _, err := jirafixture.Run(ctx, jirafixture.Options{
		URL: source, JiraDist: dist, JiraHome: home, Issues: 300, Seed: 42, TZ: time.UTC, Messy: true,
	}); err != nil {
		t.Fatalf("generating the Jira fixture: %v", err)
	}
	names := fixtureNames(t, source)

	opt := importer.Options{SourceURL: source, JiraHome: home, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	plain, err := importer.Check(ctx, opt)
	if err != nil {
		t.Fatal(err)
	}
	opt.Anonymous = true
	anon, err := importer.Check(ctx, opt)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(anon.Counts, plain.Counts) || !reflect.DeepEqual(anon.Steps, plain.Steps) {
		t.Errorf("counts differ:\nanonymous %+v\n%+v\nnormal    %+v\n%+v", anon.Counts, anon.Steps, plain.Counts, plain.Steps)
	}
	if len(anon.LeftOut) != len(plain.LeftOut) || len(anon.Warnings) != len(plain.Warnings) {
		t.Errorf("%d left-out lines and %d warnings, want %d and %d",
			len(anon.LeftOut), len(anon.Warnings), len(plain.LeftOut), len(plain.Warnings))
	}
	for i := range min(len(anon.LeftOut), len(plain.LeftOut)) {
		if anon.LeftOut[i].Count != plain.LeftOut[i].Count {
			t.Errorf("left out %q: %d, want %d", anon.LeftOut[i].What, anon.LeftOut[i].Count, plain.LeftOut[i].Count)
		}
	}

	plainText, anonText := render(t, plain), render(t, anon)
	leaked := 0
	for _, name := range names {
		re := regexp.MustCompile(wordAt(name[0]) + regexp.QuoteMeta(name) + wordAt(name[len(name)-1]))
		if re.MatchString(plainText) {
			leaked++
		}
		if loc := re.FindStringIndex(anonText); loc != nil {
			t.Errorf("the anonymous report holds %q: …%s…", name, anonText[max(loc[0]-60, 0):min(loc[1]+60, len(anonText))])
		}
	}
	if leaked == 0 {
		t.Error("the normal report names nothing of the fixture: the test proves nothing")
	}
}

// wordAt is the word boundary a name needs at an end, so that "admin" is not
// found in "administrator".
func wordAt(c byte) string {
	if c == '_' || '0' <= c && c <= '9' || 'a' <= c|0x20 && c|0x20 <= 'z' {
		return `\b`
	}
	return ""
}

// fixtureNames are the project keys and names, workflow names, user names,
// custom field names and ids of the Jira database, and its server's version.
func fixtureNames(t *testing.T, source string) []string {
	t.Helper()
	src, _, err := jiradb.Open(source, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	var names []string
	for _, q := range []string{
		"SELECT pkey FROM project", "SELECT pname FROM project",
		"SELECT workflowname FROM jiraworkflows",
		"SELECT user_name FROM cwd_user", "SELECT display_name FROM cwd_user",
		"SELECT cfname FROM customfield", "SELECT CAST(id AS CHAR(20)) FROM customfield",
		"SELECT version()",
	} {
		rows, err := src.QueryContext(t.Context(), q)
		if err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		for rows.Next() {
			var s sql.NullString
			if err := rows.Scan(&s); err != nil {
				t.Fatal(err)
			}
			if v, _, _ := strings.Cut(strings.TrimSpace(s.String), " on "); v != "" { // as the report gives the version
				names = append(names, v)
			}
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		rows.Close()
	}
	return names
}

// render gives the text report followed by the JSON one.
func render(t *testing.T, rep *importer.CheckReport) string {
	t.Helper()
	var b bytes.Buffer
	rep.WriteText(&b)
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(rep); err != nil {
		t.Fatal(err)
	}
	return b.String()
}
