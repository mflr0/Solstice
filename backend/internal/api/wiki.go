package api

// A small renderer of Jira's wiki markup into HTML, for the REST API's
// renderedFields: headings, lists, quotes, code and preformatted blocks,
// tables, rules, links, mentions and the usual text effects. Everything is
// escaped first; only the tags written here come out, and links only to
// http(s), mailto or the same site. (The interface renders wiki text itself.)

import (
	"html"
	"regexp"
	"strconv"
	"strings"
)

var (
	wikiHeading = regexp.MustCompile(`^h([1-6])\.\s+(.*)$`)
	wikiList    = regexp.MustCompile(`^([*#-]+)\s+(.*)$`)
	wikiBlock   = regexp.MustCompile(`^\{(code|noformat|quote)(?::[^}]*)?\}(.*)$`)
	wikiLink    = regexp.MustCompile(`\[([^\[\]|]*?)\|?([^\[\]|]+)\]`)
	wikiMono    = regexp.MustCompile(`\{\{(.+?)\}\}`)
	wikiColor   = regexp.MustCompile(`\{color(?::[^}]*)?\}`)
	wikiKept    = regexp.MustCompile("\x00[0-9]+\x00")
	wikiEffects = []struct {
		re  *regexp.Regexp
		tag string
	}{
		{regexp.MustCompile(`(^|[^\pL\pN*])\*([^*\s](?:[^*]*[^*\s])?)\*($|[^\pL\pN*])`), "b"},
		{regexp.MustCompile(`(^|[^\pL\pN_])_([^_\s](?:[^_]*[^_\s])?)_($|[^\pL\pN_])`), "em"},
		{regexp.MustCompile(`(^|[^\pL\pN+])\+([^+\s](?:[^+]*[^+\s])?)\+($|[^\pL\pN+])`), "ins"},
		{regexp.MustCompile(`(^|[^\pL\pN-])-([^-\s](?:[^-]*[^-\s])?)-($|[^\pL\pN-])`), "del"},
		{regexp.MustCompile(`(^|[^\pL\pN^])\^([^^\s](?:[^^]*[^^\s])?)\^($|[^\pL\pN^])`), "sup"},
		{regexp.MustCompile(`(^|[^\pL\pN~])~([^~\s](?:[^~]*[^~\s])?)~($|[^\pL\pN~])`), "sub"},
		{regexp.MustCompile(`\?\?(.+?)\?\?`), "cite"},
	}
)

// maxWiki bounds the text rendered as markup; longer text comes out as
// preformatted plain text.
const maxWiki = 256 << 10

func renderWiki(src string) string {
	if len(src) > maxWiki {
		return "<pre>" + html.EscapeString(src) + "</pre>"
	}
	var w wikiWriter
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	for i := 0; i < len(lines); {
		trimmed := strings.TrimSpace(lines[i])
		next := i + 1
		if m := wikiBlock.FindStringSubmatch(trimmed); m != nil {
			w.endBlock()
			var block string
			block, next = wikiBlockAt(lines, i, m[1], m[2])
			w.out.WriteString(block)
		} else if trimmed == "" {
			w.endBlock()
		} else if trimmed == "----" {
			w.endBlock()
			w.out.WriteString("<hr/>")
		} else if m := wikiHeading.FindStringSubmatch(trimmed); m != nil {
			w.endBlock()
			w.out.WriteString("<h" + m[1] + ">" + wikiInline(m[2]) + "</h" + m[1] + ">")
		} else if text, ok := strings.CutPrefix(trimmed, "bq. "); ok {
			w.endBlock()
			w.out.WriteString("<blockquote><p>" + wikiInline(text) + "</p></blockquote>")
		} else if strings.HasPrefix(trimmed, "|") {
			w.endBlock()
			var table string
			table, next = wikiTableAt(lines, i)
			w.out.WriteString(table)
		} else if m := wikiList.FindStringSubmatch(trimmed); m != nil {
			w.flushPara()
			w.listItem(m[1], m[2])
		} else {
			w.closeLists(0)
			w.para = append(w.para, wikiInline(trimmed))
		}
		i = next
	}
	w.endBlock()
	return w.out.String()
}

// wikiWriter is the HTML renderWiki writes, with the paragraph and the lists
// still open.
type wikiWriter struct {
	out   strings.Builder
	para  []string // rendered lines of the open paragraph
	lists []string // open list tags, outermost first
}

func (w *wikiWriter) flushPara() {
	if len(w.para) > 0 {
		w.out.WriteString("<p>" + strings.Join(w.para, "<br/>") + "</p>")
		w.para = nil
	}
}

func (w *wikiWriter) closeLists(depth int) {
	for len(w.lists) > depth {
		w.out.WriteString("</li></" + w.lists[len(w.lists)-1] + ">")
		w.lists = w.lists[:len(w.lists)-1]
	}
}

// endBlock closes the open paragraph and lists before a block of its own.
func (w *wikiWriter) endBlock() {
	w.flushPara()
	w.closeLists(0)
}

// listItem writes an item whose marks (*, # or -, one per level) give its
// depth and, by the last one, its list type. An item at most one level
// deeper than the open lists opens a list.
func (w *wikiWriter) listItem(marks, text string) {
	tag := "ul"
	if marks[len(marks)-1] == '#' {
		tag = "ol"
	}
	depth := min(len(marks), len(w.lists)+1)
	if depth <= len(w.lists) {
		w.closeLists(depth)
		if w.lists[depth-1] == tag {
			w.out.WriteString("</li><li>" + wikiInline(text))
			return
		}
		w.closeLists(depth - 1)
	}
	w.out.WriteString("<" + tag + "><li>" + wikiInline(text))
	w.lists = append(w.lists, tag)
}

// wikiBlockAt renders the {code}, {noformat} or {quote} block of the given
// kind that opens on lines[i], rest being the text after its tag, up to its
// closing tag or the end of the text. It returns the index of the line after
// the block.
func wikiBlockAt(lines []string, i int, kind, rest string) (string, int) {
	closing := "{" + kind + "}"
	var body []string
	for {
		before, _, closed := strings.Cut(rest, closing)
		body = append(body, before)
		i++
		if closed || i == len(lines) {
			break
		}
		rest = lines[i]
	}
	text := strings.Trim(strings.Join(body, "\n"), "\n")
	switch kind {
	case "quote":
		return "<blockquote>" + renderWiki(text) + "</blockquote>", i
	case "code":
		return `<pre class="code">` + html.EscapeString(text) + "</pre>", i
	default:
		return "<pre>" + html.EscapeString(text) + "</pre>", i
	}
}

// wikiTableAt renders the table whose rows start on lines[i], and returns
// the index of the first line after it.
func wikiTableAt(lines []string, i int) (string, int) {
	var out strings.Builder
	out.WriteString("<table>")
	for ; i < len(lines); i++ {
		row := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(row, "|") {
			break
		}
		out.WriteString(wikiRow(row))
	}
	out.WriteString("</table>")
	return out.String(), i
}

// wikiRow renders a table row: ||heading||heading|| or |cell|cell|.
func wikiRow(line string) string {
	var out strings.Builder
	out.WriteString("<tr>")
	for len(line) > 0 {
		tag := "td"
		switch {
		case strings.HasPrefix(line, "||"):
			tag, line = "th", line[2:]
		case strings.HasPrefix(line, "|"):
			line = line[1:]
		}
		if line == "" {
			break
		}
		end := strings.Index(line, "|")
		if end < 0 {
			end = len(line)
		}
		out.WriteString("<" + tag + ">" + wikiInline(strings.TrimSpace(line[:end])) + "</" + tag + ">")
		line = line[end:]
	}
	out.WriteString("</tr>")
	return out.String()
}

// wikiInline renders the text effects of a line. Links and monospaced text
// are set aside first, so their content is not formatted.
func wikiInline(text string) string {
	text = strings.ReplaceAll(text, "\x00", "")
	var kept, mono []string // mono: the source of each monospaced span, kept first
	keep := func(s string) string {
		kept = append(kept, s)
		return "\x00" + strconv.Itoa(len(kept)-1) + "\x00"
	}
	restore := func(s string, from []string) string {
		return wikiKept.ReplaceAllStringFunc(s, func(m string) string {
			i, _ := strconv.Atoi(m[1 : len(m)-1])
			return from[i]
		})
	}
	text = replaceSubmatch(wikiMono, text, func(m []string) string {
		mono = append(mono, m[0])
		return keep("<code>" + html.EscapeString(m[1]) + "</code>")
	})
	// A link holds the placeholders of its monospaced spans: its target gets
	// their source back, the text it shows their HTML.
	text = replaceSubmatch(wikiLink, text, func(m []string) string {
		label, target := m[1], strings.TrimSpace(restore(m[2], mono))
		if strings.HasPrefix(target, "~") {
			return keep(`<span class="user-mention">@` + html.EscapeString(target[1:]) + "</span>")
		}
		if label == "" {
			label = target
		}
		if !safeHref(target) {
			return keep(restore(html.EscapeString(m[0]), kept))
		}
		return keep(`<a href="` + html.EscapeString(target) + `">` + restore(html.EscapeString(label), kept) + "</a>")
	})
	text = html.EscapeString(wikiColor.ReplaceAllString(text, ""))
	text = strings.ReplaceAll(text, `\\`, "<br/>")
	for _, e := range wikiEffects {
		if e.tag == "cite" {
			text = e.re.ReplaceAllString(text, "<cite>$1</cite>")
			continue
		}
		// Twice: neighbouring spans share the character between them.
		for range 2 {
			text = e.re.ReplaceAllString(text, "$1<"+e.tag+">$2</"+e.tag+">$3")
		}
	}
	return restore(text, kept)
}

// replaceSubmatch is regexp's ReplaceAllStringFunc, with repl given the
// submatches of each match rather than the match alone.
func replaceSubmatch(re *regexp.Regexp, s string, repl func(m []string) string) string {
	var out strings.Builder
	last := 0
	for _, loc := range re.FindAllStringSubmatchIndex(s, -1) {
		m := make([]string, len(loc)/2)
		for k := range m {
			if loc[2*k] >= 0 {
				m[k] = s[loc[2*k]:loc[2*k+1]]
			}
		}
		out.WriteString(s[last:loc[0]])
		out.WriteString(repl(m))
		last = loc[1]
	}
	out.WriteString(s[last:])
	return out.String()
}

func safeHref(u string) bool {
	if strings.ContainsAny(u, "\\\x00") {
		return false
	}
	lower := strings.ToLower(u)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "mailto:") || (strings.HasPrefix(u, "/") && !strings.HasPrefix(u, "//"))
}
