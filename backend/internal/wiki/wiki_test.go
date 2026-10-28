package wiki

import (
	"strings"
	"testing"
	"time"
)

// The exact HTML of renderWiki for the edge cases of its grammar.
func TestRenderWikiExact(t *testing.T) {
	for _, c := range []struct{ name, in, want string }{
		{"empty", "", ""},
		{"blank lines", "  \t\n\n", ""},
		{"crlf", "a\r\nb", "<p>a<br/>b</p>"},
		{"trailing spaces", "a   \nb\t", "<p>a<br/>b</p>"},
		{"paragraphs", "a\nb\n\nc", "<p>a<br/>b</p><p>c</p>"},
		{"rule", "a\n----\nb", "<p>a</p><hr/><p>b</p>"},
		{"five dashes", "-----", "<p>-----</p>"},
		{"heading", "h3.  *T*  ", "<h3><b>T</b></h3>"},
		{"not a heading", "h7. x", "<p>h7. x</p>"},
		{"bq", "bq. *b*", "<blockquote><p><b>b</b></p></blockquote>"},
		{"bq without space", "bq.x", "<p>bq.x</p>"},
		{"code unclosed", "{code}\nx < y\nmore", "<pre class=\"code\">x &lt; y\nmore</pre>"},
		{"code param and blank edges", "{code:java}\n\nint a;\n\n{code}", "<pre class=\"code\">int a;</pre>"},
		{"noformat same line, tail dropped", "{noformat}a{noformat} tail\nnext", "<pre>a</pre><p>next</p>"},
		{"indented quote", "  {quote}q{quote}", "<blockquote><p>q</p></blockquote>"},
		{"nested quote", "{quote}\nh1. T\n* a\n{quote}\nafter", "<blockquote><h1>T</h1><ul><li>a</li></ul></blockquote><p>after</p>"},
		{"block closes list", "* a\n{code}x{code}\n* b", "<ul><li>a</li></ul><pre class=\"code\">x</pre><ul><li>b</li></ul>"},
		{"block closes para", "p\n{noformat}\n*x*\n{noformat}", "<p>p</p><pre>*x*</pre>"},
		{"depth clamp", "*** deep\n** two\n* top", "<ul><li>deep<ul><li>two</li></ul></li><li>top</li></ul>"},
		{"ul then ol", "* a\n# b", "<ul><li>a</li></ul><ol><li>b</li></ol>"},
		{"mixed nesting", "* a\n*# b\n*# c\n* d", "<ul><li>a<ol><li>b</li><li>c</li></ol></li><li>d</li></ul>"},
		{"dash list", "- a\n- b", "<ul><li>a</li><li>b</li></ul>"},
		{"para closes list", "* a\ntext", "<ul><li>a</li></ul><p>text</p>"},
		{"list closes para", "p1\np2\n* a", "<p>p1<br/>p2</p><ul><li>a</li></ul>"},
		{"table last line", "|a|b|", "<table><tr><td>a</td><td>b</td></tr></table>"},
		{"table then text", "||h||g||\n|c|d|\nafter", "<table><tr><th>h</th><th>g</th></tr><tr><td>c</td><td>d</td></tr></table><p>after</p>"},
		{"table in list", "* a\n|c|", "<ul><li>a</li></ul><table><tr><td>c</td></tr></table>"},
		{"table cells", "|a||b|\n| x |y", "<table><tr><td>a</td><th>b</th></tr><tr><td>x</td><td>y</td></tr></table>"},
		{"nul", "a\x00b \x000\x00", "<p>ab 0</p>"},
		{"color", "{color:red}r{color}", "<p>r</p>"},
		{"line break", "a \\\\ b", "<p>a <br/> b</p>"},
		{"adjacent effects", "*a* *b* _c_ *d*_e_", "<p><b>a</b> <b>b</b> <em>c</em> <b>d</b><em>e</em></p>"},
		{"mid-word", "x*y*z snake_case_name a-b-c", "<p>x*y*z snake_case_name a-b-c</p>"},
		{"all effects", "??cite?? ^sup^ ~sub~ +ins+ -del-", "<p><cite>cite</cite> <sup>sup</sup> <sub>sub</sub> <ins>ins</ins> <del>del</del></p>"},
		{"mono keeps markup", "{{*x* <y>}} *z*", "<p><code>*x* &lt;y&gt;</code> <b>z</b></p>"},
		{"link without label", "[https://x.org/?a=1&b=2]", "<p><a href=\"https://x.org/?a=1&amp;b=2\">https://x.org/?a=1&amp;b=2</a></p>"},
		{"link label effects", "[*b*|https://x.org]", "<p><a href=\"https://x.org\">*b*</a></p>"},
		{"mention", "[~bob] [ ~ann ]", "<p><span class=\"user-mention\">@bob</span> <span class=\"user-mention\">@ann</span></p>"},
		{"unsafe link", "[a|javascript:x] [b|//h] [c|ftp://h]", "<p>[a|javascript:x] [b|//h] [c|ftp://h]</p>"},
		{"relative and mailto", "[a|/p] [b|mailto:x@y.org] [c|HTTPS://X.ORG]", "<p><a href=\"/p\">a</a> <a href=\"mailto:x@y.org\">b</a> <a href=\"HTTPS://X.ORG\">c</a></p>"},
		{"escape", "<b>\"x\" & 'y'</b>", "<p>&lt;b&gt;&#34;x&#34; &amp; &#39;y&#39;&lt;/b&gt;</p>"},
		{"mono in a label", "[{{m}} <x>|https://x.org]", "<p><a href=\"https://x.org\"><code>m</code> &lt;x&gt;</a></p>"},
		{"mono in a refused link", "[{{m}}|javascript:x] [{{n}}]", "<p>[<code>m</code>|javascript:x] [<code>n</code>]</p>"},
		{"mono in a mention", "[~{{bob}}]", "<p><span class=\"user-mention\">@{{bob}}</span></p>"},
		{"mono in a target", "[x|https://x.org/{{a}}] [{{b}}|javascript:{{c}}]", "<p><a href=\"https://x.org/{{a}}\">x</a> [<code>b</code>|javascript:<code>c</code>]</p>"},
	} {
		got := HTML(c.in, Options{})
		if got != c.want {
			t.Errorf("%s: renderWiki(%q)\n got %q\nwant %q", c.name, c.in, got, c.want)
		}
		if strings.Contains(got, "\x00") {
			t.Errorf("%s: renderWiki(%q) holds a NUL: %q", c.name, c.in, got)
		}
	}
}

func TestRenderWiki(t *testing.T) {
	out := HTML("h2. Étapes\n# ouvrir le *panier*\n# recharger\n\nVoir _la_ page {{/panier}} et [la doc|https://exemple.org/doc?a=1&b=2].\n"+
		"* un\n** deux\n----\n{code:go}\nif a < b { panic(\"<x>\") }\n{code}\n||A||B||\n|1|2|\nbq. cité\n[~marie.durand] \\\\ fin", Options{})
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
		out := HTML(src, Options{})
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
	out := HTML(strings.Repeat("{{a}} [b|https://x.org] ", 8000), Options{})
	if d := time.Since(start); d > 2*time.Second || strings.Count(out, "<code>") != 8000 {
		t.Errorf("200 KB of markup: %s, %d spans", d, strings.Count(out, "<code>"))
	}
	if out := HTML(strings.Repeat("*x* ", maxWiki), Options{}); !strings.HasPrefix(out, "<pre>") {
		t.Error("text over the limit is parsed")
	}
}

func TestOptions(t *testing.T) {
	o := Options{
		Mention: func(u string) string { return map[string]string{"bob": "Bob <B>"}[u] },
		Local: func(p string) string {
			if p == "/hidden" {
				return ""
			}
			return "https://s.example" + p
		},
	}
	for in, want := range map[string]string{
		"[~bob] [~ann]":                     `<p><span class="user-mention">@Bob &lt;B&gt;</span> <span class="user-mention">@ann</span></p>`,
		"[a|/browse/X-1] [b|https://x.org]": `<p><a href="https://s.example/browse/X-1">a</a> <a href="https://x.org">b</a></p>`,
		"[*a* & {{c}}|/hidden]":             "<p>*a* &amp; <code>c</code></p>",
		"{quote}[~bob]{quote}":              `<blockquote><p><span class="user-mention">@Bob &lt;B&gt;</span></p></blockquote>`,
		"|[/t]|":                            `<table><tr><td><a href="https://s.example/t">/t</a></td></tr></table>`,
		"* [~bob]":                          `<ul><li><span class="user-mention">@Bob &lt;B&gt;</span></li></ul>`,
	} {
		if got := HTML(in, o); got != want {
			t.Errorf("HTML(%q)\n got %q\nwant %q", in, got, want)
		}
	}
}
