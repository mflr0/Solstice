package tests

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"solstice/internal/jirafixture"
	"solstice/internal/mail/mailtest"
)

var updateGolden = flag.Bool("update", false, "rewrite the golden files of the tests that have them")

// TestNotifyGolden pins whole e-mails, subject, text and HTML, byte for byte
// once the values that change from run to run are scrubbed. The other
// notification tests check pieces of them.
func TestNotifyGolden(t *testing.T) {
	t.Run("created-updated", func(t *testing.T) {
		env := newNotifyEnv(t)
		paul := env.login("paul.martin", jirafixture.Password)
		hugo := env.login("hugo.robert", jirafixture.Password)
		hugo.call("PATCH", "/api/me", map[string]any{"locale": "en"}, 200, nil)
		a := createIssue(t, paul, "WEB", "Export CSV <tronqué>", "À voir avec [~hugo.robert].\nDeuxième ligne.")
		g := newGolden(t, a)
		g.add("creation", to(env.deliver(), "hugo.robert@exemple.fr"))

		hugo.call("PUT", "/api/issues/"+a+"/watch", nil, 200, nil)
		paul.call("PATCH", "/api/issues/"+a, map[string]any{"description": "Nouvelle description\nsur deux lignes.",
			"due": "2031-03-15", "environment": "Firefox 140"}, 200, nil)
		paul.call("PATCH", "/api/issues/"+a, map[string]any{"environment": ""}, 200, nil)
		g.add("update", to(env.deliver(), "hugo.robert@exemple.fr"))
		g.check()
	})

	t.Run("relations", func(t *testing.T) {
		env := newNotifyEnv(t)
		paul := env.login("paul.martin", jirafixture.Password)
		hugo := env.login("hugo.robert", jirafixture.Password)
		chloe := env.login("chloe.simon", jirafixture.Password) // sees RH, which paul.martin does not
		paul.call("PATCH", "/api/me", map[string]any{"locale": "en"}, 200, nil)
		a := createIssue(t, paul, "WEB", "Lien source", "")
		b := createIssue(t, paul, "WEB", "Lien cible", "")
		env.deliver()
		linkType := json.Number(pick(t, `SELECT id FROM link_types ORDER BY position LIMIT 1`))
		chloe.call("POST", "/api/issues/"+a+"/links", map[string]any{"type": linkType, "direction": "outward",
			"target": rhKey(t)}, 201, nil)
		hugo.call("POST", "/api/issues/"+a+"/links", map[string]any{"type": linkType, "direction": "outward",
			"target": b}, 201, nil)
		g := newGolden(t, a, b)
		g.add("links", to(env.deliver(), "paul.martin@exemple.fr"))
		g.check()
	})

	t.Run("restricted-comment-fr", func(t *testing.T) {
		env := newNotifyEnv(t)
		paul := env.login("paul.martin", jirafixture.Password)
		hugo := env.login("hugo.robert", jirafixture.Password)  // developpeurs
		chloe := env.login("chloe.simon", jirafixture.Password) // not a developer
		hugo.call("PATCH", "/api/me", map[string]any{"locale": "fr"}, 200, nil)
		a := createIssue(t, paul, "WEB", "Commentaire réservé", "")
		env.deliver()
		hugo.call("PUT", "/api/issues/"+a+"/watch", nil, 200, nil)
		chloe.call("PUT", "/api/issues/"+a+"/watch", nil, 200, nil)
		paul.call("POST", "/api/issues/"+a+"/comments", map[string]any{"body": "Trace interne : <b>pile</b>\nligne 2",
			"visibility": map[string]string{"group": "developpeurs"}}, 201, nil)
		msgs := env.deliver()
		if got := to(msgs, "chloe.simon@exemple.fr"); len(got) != 0 {
			t.Fatalf("a comment restricted to developpeurs reached chloe.simon: %d e-mails", len(got))
		}
		g := newGolden(t, a)
		g.add("comment", to(msgs, "hugo.robert@exemple.fr"))
		g.check()
	})

	t.Run("digest-de", func(t *testing.T) {
		env := newNotifyEnv(t)
		paul := env.login("paul.martin", jirafixture.Password)
		chloe := env.login("chloe.simon", jirafixture.Password)
		a := createIssue(t, paul, "WEB", "Erster Eintrag", "")
		b := createIssue(t, paul, "WEB", "Zweiter Eintrag", "")
		env.deliver()
		chloe.call("PATCH", "/api/me", map[string]any{"locale": "de"}, 200, nil)
		chloe.call("PUT", "/api/me/notifications", map[string]any{"delivery": "digest", "digestHour": 7}, 200, nil)
		for _, k := range []string{b, a} {
			chloe.call("PUT", "/api/issues/"+k+"/watch", nil, 200, nil)
			paul.call("POST", "/api/issues/"+k+"/comments", map[string]any{"body": "Für die Zusammenfassung"}, 201, nil)
		}
		paul.call("PATCH", "/api/issues/"+b, map[string]any{"summary": "Zweiter Eintrag (neu)"}, 200, nil)
		if got := to(env.deliver(), "chloe.simon@exemple.fr"); len(got) != 0 {
			t.Fatal("a digest reader got an immediate e-mail")
		}
		g := newGolden(t, a, b)
		g.add("digest", to(env.deliverAt(25*time.Hour), "chloe.simon@exemple.fr"))
		g.check()
	})
}

// golden collects the e-mails of a test, scrubbed, and compares them with
// testdata/notify/<test>.golden (rewritten with -update).
type golden struct {
	t     *testing.T
	keys  []string // the test's own issues, in the order they are named KEY-1, KEY-2…
	ids   []string
	parts []string
}

func newGolden(t *testing.T, keys ...string) *golden {
	g := &golden{t: t, keys: keys}
	for _, k := range keys {
		g.ids = append(g.ids, issueID(t, k))
	}
	return g
}

var (
	// formatTime's "date HH:MM", in each language's date layout.
	goldenWhen = regexp.MustCompile(`\b(\d{4}-\d{2}-\d{2}|\d{2}[./]\d{2}[./]\d{4}) \d{2}:\d{2}\b`)
	// A date alone: the digest's day. The tests' own due dates are in 2031.
	goldenDay   = regexp.MustCompile(`\b(\d{4}-\d{2}-\d{2}|\d{2}[./]\d{2}[./]\d{4})\b`)
	goldenMsgID = regexp.MustCompile(`^<[0-9a-f]{16,}@exemple\.fr>$`)
)

func (g *golden) scrub(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = goldenWhen.ReplaceAllString(s, "<when>")
	s = goldenDay.ReplaceAllStringFunc(s, func(d string) string {
		if strings.Contains(d, "2031") {
			return d
		}
		return "<day>"
	})
	// Longest first, so that WEB-12 is not replaced inside WEB-123.
	order := make([]int, len(g.keys))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(i, j int) int { return len(g.keys[j]) - len(g.keys[i]) })
	for _, i := range order {
		s = regexp.MustCompile(`\b`+regexp.QuoteMeta(g.keys[i])+`\b`).ReplaceAllString(s, "KEY-"+strconv.Itoa(i+1))
	}
	return s
}

// threadID names a Message-ID or In-Reply-To by what it must be: an issue's
// thread root, a fresh random identifier, or nothing.
func (g *golden) threadID(v string) string {
	for i, id := range g.ids {
		if v == "<issue."+id+"@exemple.fr>" {
			return "<issue.KEY-" + strconv.Itoa(i+1) + "@exemple.fr>"
		}
	}
	if goldenMsgID.MatchString(v) {
		return "<random@exemple.fr>"
	}
	return v
}

func (g *golden) add(step string, msgs []mailtest.Message) {
	g.t.Helper()
	var out []string
	for _, m := range msgs {
		var b strings.Builder
		b.WriteString("To: " + m.To + "\n")
		b.WriteString("Subject: " + g.scrub(m.Subject) + "\n")
		b.WriteString("Message-ID: " + g.threadID(m.Header.Get("Message-ID")) + "\n")
		b.WriteString("In-Reply-To: " + g.threadID(m.Header.Get("In-Reply-To")) + "\n")
		b.WriteString("\n--- text\n" + g.scrub(m.Text) + "\n--- html\n" + g.scrub(m.HTML) + "\n")
		out = append(out, b.String())
	}
	slices.Sort(out)
	g.parts = append(g.parts, "=== "+step+" ("+strconv.Itoa(len(msgs))+" e-mails)\n"+strings.Join(out, "\n"))
}

func (g *golden) check() {
	g.t.Helper()
	got := strings.Join(g.parts, "\n")
	path := filepath.Join("testdata", "notify", strings.ReplaceAll(g.t.Name(), "/", "-")+".golden")
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			g.t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			g.t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		g.t.Fatalf("%v (run with -update to write it)", err)
	}
	if got != string(want) {
		g.t.Errorf("the e-mails differ from %s (rerun with -update and diff it to see how):\n%s", path, got)
	}
}
