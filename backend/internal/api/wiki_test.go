package api

import "testing"

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
	} {
		if got := renderWiki(c.in); got != c.want {
			t.Errorf("%s: renderWiki(%q)\n got %q\nwant %q", c.name, c.in, got, c.want)
		}
	}
}
