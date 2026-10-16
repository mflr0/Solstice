package api

// Tests of the CSV export's formula guard and the wiki renderer.

import (
	"strings"
	"testing"
	"time"
)

func TestGuardFormula(t *testing.T) {
	for in, want := range map[string]string{"=1+1": "'=1+1", "+33 6": "'+33 6", "-2+3": "'-2+3", "@SUM(A1)": "'@SUM(A1)",
		"\tx": "'\tx", "Panier": "Panier", "": "", "2026-10-15": "2026-10-15"} {
		if got := guardFormula(in); got != want {
			t.Errorf("guardFormula(%q) = %q, want %q", in, got, want)
		}
	}
}

// Wiki markup ------------------------------------------------------------------

func TestRenderWiki(t *testing.T) {
	out := renderWiki("h2. Étapes\n# ouvrir le *panier*\n# recharger\n\nVoir _la_ page {{/panier}} et [la doc|https://exemple.org/doc?a=1&b=2].\n" +
		"* un\n** deux\n----\n{code:go}\nif a < b { panic(\"<x>\") }\n{code}\n||A||B||\n|1|2|\nbq. cité\n[~marie.durand] \\\\ fin")
	for _, want := range []string{"<h2>Étapes</h2>", "<ol><li>ouvrir le <b>panier</b></li><li>recharger</li></ol>",
		"<em>la</em>", "<code>/panier</code>", `<a href="https://exemple.org/doc?a=1&amp;b=2">la doc</a>`,
		"<ul><li>un<ul><li>deux</li></ul></li></ul>", "<hr/>", `<pre class="code">if a &lt; b { panic(&#34;&lt;x&gt;&#34;) }</pre>`,
		"<tr><th>A</th><th>B</th></tr>", "<tr><td>1</td><td>2</td></tr>", "<blockquote><p>cité</p></blockquote>",
		`<span class="user-mention">@marie.durand</span>`, "<br/>"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s in\n%s", want, out)
		}
	}
	// Nothing from the text runs or links where it should not.
	for _, src := range []string{`<script>alert(1)</script>`, `<img src=x onerror=alert(1)>`, `[clic|javascript:alert(1)]`,
		`[x|//evil.example]`, `[x|/\evil.example]`, "{noformat}<b>\"'{noformat}", `[a|https://x.org" onmouseover="alert(1)]`} {
		out := renderWiki(src)
		if strings.Contains(out, "<script") || strings.Contains(out, "<img") || strings.Contains(out, `href="javascript`) ||
			strings.Contains(out, `href="//`) || strings.Contains(out, `href="/\`) || strings.Contains(out, "<b>") ||
			strings.Contains(out, `" onmouseover`) {
			t.Errorf("renderWiki(%q) = %q", src, out)
		}
	}
}

// Rendering stays linear: many links or monospaced spans must not take
// quadratic time, and very long text is not parsed at all.
func TestRenderWikiLargeInput(t *testing.T) {
	start := time.Now()
	out := renderWiki(strings.Repeat("{{a}} [b|https://x.org] ", 8000))
	if d := time.Since(start); d > 2*time.Second || strings.Count(out, "<code>") != 8000 {
		t.Errorf("200 KB of markup: %s, %d spans", d, strings.Count(out, "<code>"))
	}
	if out := renderWiki(strings.Repeat("*x* ", maxWiki)); !strings.HasPrefix(out, "<pre>") {
		t.Error("text over the limit is parsed")
	}
}
