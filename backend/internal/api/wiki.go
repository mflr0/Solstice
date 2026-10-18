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
	var out strings.Builder
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	var para []string
	var lists []string // open list tags, outermost first
	flushPara := func() {
		if len(para) > 0 {
			out.WriteString("<p>" + strings.Join(para, "<br/>") + "</p>")
			para = nil
		}
	}
	closeLists := func(depth int) {
		for len(lists) > depth {
			out.WriteString("</li></" + lists[len(lists)-1] + ">")
			lists = lists[:len(lists)-1]
		}
	}
	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], " \t")
		trimmed := strings.TrimSpace(line)
		if m := wikiBlock.FindStringSubmatch(trimmed); m != nil {
			flushPara()
			closeLists(0)
			// Gather the block up to its closing tag.
			var body []string
			rest := m[2]
			closing := "{" + m[1] + "}"
			for {
				if before, _, ok := strings.Cut(rest, closing); ok {
					body = append(body, before)
					break
				}
				body = append(body, rest)
				i++
				if i >= len(lines) {
					break
				}
				rest = lines[i]
			}
			text := strings.Trim(strings.Join(body, "\n"), "\n")
			switch m[1] {
			case "quote":
				out.WriteString("<blockquote>" + renderWiki(text) + "</blockquote>")
			case "code":
				out.WriteString(`<pre class="code">` + html.EscapeString(text) + "</pre>")
			default:
				out.WriteString("<pre>" + html.EscapeString(text) + "</pre>")
			}
			continue
		}
		switch {
		case trimmed == "":
			flushPara()
			closeLists(0)
		case trimmed == "----":
			flushPara()
			closeLists(0)
			out.WriteString("<hr/>")
		case wikiHeading.MatchString(trimmed):
			flushPara()
			closeLists(0)
			m := wikiHeading.FindStringSubmatch(trimmed)
			out.WriteString("<h" + m[1] + ">" + wikiInline(m[2]) + "</h" + m[1] + ">")
		case strings.HasPrefix(trimmed, "bq. "):
			flushPara()
			closeLists(0)
			out.WriteString("<blockquote><p>" + wikiInline(strings.TrimPrefix(trimmed, "bq. ")) + "</p></blockquote>")
		case strings.HasPrefix(trimmed, "|"):
			flushPara()
			closeLists(0)
			out.WriteString("<table>")
			for ; i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), "|"); i++ {
				out.WriteString(wikiRow(strings.TrimSpace(lines[i])))
			}
			i--
			out.WriteString("</table>")
		case wikiList.MatchString(trimmed):
			flushPara()
			m := wikiList.FindStringSubmatch(trimmed)
			depth := len(m[1])
			tag := "ul"
			if m[1][depth-1] == '#' {
				tag = "ol"
			}
			if depth > len(lists)+1 {
				depth = len(lists) + 1
			}
			switch {
			case depth > len(lists):
				out.WriteString("<" + tag + "><li>")
				lists = append(lists, tag)
			default:
				closeLists(depth)
				if lists[depth-1] != tag {
					closeLists(depth - 1)
					out.WriteString("<" + tag + "><li>")
					lists = append(lists, tag)
				} else {
					out.WriteString("</li><li>")
				}
			}
			out.WriteString(wikiInline(m[2]))
		default:
			closeLists(0)
			para = append(para, wikiInline(trimmed))
		}
	}
	flushPara()
	closeLists(0)
	return out.String()
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
	var kept []string
	keep := func(s string) string {
		kept = append(kept, s)
		return "\x00" + strconv.Itoa(len(kept)-1) + "\x00"
	}
	text = wikiMono.ReplaceAllStringFunc(text, func(m string) string {
		return keep("<code>" + html.EscapeString(wikiMono.FindStringSubmatch(m)[1]) + "</code>")
	})
	text = wikiLink.ReplaceAllStringFunc(text, func(m string) string {
		parts := wikiLink.FindStringSubmatch(m)
		label, target := parts[1], strings.TrimSpace(parts[2])
		if strings.HasPrefix(target, "~") {
			return keep(`<span class="user-mention">@` + html.EscapeString(target[1:]) + "</span>")
		}
		if label == "" {
			label = target
		}
		if !safeHref(target) {
			return keep(html.EscapeString(m))
		}
		return keep(`<a href="` + html.EscapeString(target) + `">` + html.EscapeString(label) + "</a>")
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
	return wikiKept.ReplaceAllStringFunc(text, func(m string) string {
		i, _ := strconv.Atoi(m[1 : len(m)-1])
		return kept[i]
	})
}

var wikiKept = regexp.MustCompile("\x00[0-9]+\x00")

func safeHref(u string) bool {
	if strings.ContainsAny(u, "\\\x00") {
		return false
	}
	lower := strings.ToLower(u)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "mailto:") || (strings.HasPrefix(u, "/") && !strings.HasPrefix(u, "//"))
}
