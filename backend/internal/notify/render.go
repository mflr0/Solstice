package notify

import (
	"bytes"
	"html/template"
	"strings"
	"time"
	"unicode/utf8"

	"solstice/internal/i18n"
)

// page is an e-mail's content, rendered in HTML and in plain text. Every
// text in it is already translated; the HTML template escapes it all.
type page struct {
	Lang     string
	Title    string
	Heading  string
	Intro    []string
	Sections []section
	Tables   []table
	Why      string
	Prefs    string // link label, with PrefsURL
	PrefsURL string
}

// section is an issue and what happened to it.
type section struct {
	Key, Summary string
	URL          string
	Meta         string
	Entries      []entry
	Link         string // label of the link to the issue
}

type entry struct {
	Headline string
	When     string
	Changes  []changeRow
	Text     string // a comment or description, shown as written
	Note     string
}

type changeRow struct {
	Field, From, To string
}

// table is a list of issues (filter results, due dates).
type table struct {
	Title   string
	Columns []string
	Rows    []tableRow
	Empty   string
	More    string
	MoreURL string
}

type tableRow struct {
	Key, URL string
	Cells    []string
}

var htmlPage = template.Must(template.New("mail").Parse(`<!DOCTYPE html>
<html lang="{{.Lang}}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title></head>
<body style="margin:0;padding:0;background:#f4f1ea;">
<div style="max-width:640px;margin:0 auto;padding:24px 16px;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Helvetica,Arial,sans-serif;font-size:14px;line-height:1.5;color:#1c1b19;">
{{- if .Heading}}
<p style="margin:0 0 16px;font-family:Georgia,'Times New Roman',serif;font-size:20px;">{{.Heading}}</p>
{{- end}}
{{- range .Intro}}
<p style="margin:0 0 12px;">{{.}}</p>
{{- end}}
{{- range .Sections}}
<div style="background:#faf8f3;border:1px solid #e6e0d4;border-top:2px solid #d9381e;padding:16px;margin:0 0 16px;">
<p style="margin:0;font-family:Georgia,'Times New Roman',serif;font-size:18px;font-weight:600;line-height:1.3;">
{{- if .URL}}<a href="{{.URL}}" style="color:#1c1b19;text-decoration:none;">{{end -}}
<span style="font-family:ui-monospace,Menlo,Consolas,monospace;font-size:14px;font-weight:400;color:#c2321b;">{{.Key}}</span> {{.Summary}}
{{- if .URL}}</a>{{end}}</p>
{{- if .Meta}}
<p style="margin:4px 0 0;color:#6c685e;font-size:13px;">{{.Meta}}</p>
{{- end}}
{{- range .Entries}}
<div style="margin-top:14px;padding-top:12px;border-top:1px solid #e6e0d4;">
<p style="margin:0 0 6px;">{{.Headline}} <span style="color:#6c685e;font-size:12px;">{{.When}}</span></p>
{{- if .Changes}}
<table role="presentation" cellpadding="0" cellspacing="0" style="border-collapse:collapse;font-size:13px;">
{{- range .Changes}}
<tr><td style="padding:2px 12px 2px 0;color:#6c685e;vertical-align:top;white-space:nowrap;">{{.Field}}</td><td style="padding:2px 0;vertical-align:top;">
{{- if .From}}<span style="color:#6c685e;text-decoration:line-through;">{{.From}}</span> → {{end}}{{.To}}</td></tr>
{{- end}}
</table>
{{- end}}
{{- if .Text}}
<div style="margin-top:6px;padding:2px 0 2px 12px;border-left:2px solid #cfc7b7;white-space:pre-wrap;word-wrap:break-word;">{{.Text}}</div>
{{- end}}
{{- if .Note}}
<p style="margin:4px 0 0;color:#6c685e;font-size:12px;">{{.Note}}</p>
{{- end}}
</div>
{{- end}}
{{- if and .Link .URL}}
<p style="margin:14px 0 0;"><a href="{{.URL}}" style="color:#c2321b;">{{.Link}}</a></p>
{{- end}}
</div>
{{- end}}
{{- range .Tables}}
{{- if .Title}}
<p style="margin:16px 0 6px;font-family:Georgia,'Times New Roman',serif;font-size:16px;">{{.Title}}</p>
{{- end}}
{{- if .Rows}}
<table cellpadding="0" cellspacing="0" style="border-collapse:collapse;width:100%;font-size:13px;background:#faf8f3;border:1px solid #e6e0d4;">
<tr>{{range .Columns}}<th style="text-align:left;padding:6px 8px;border-bottom:1px solid #cfc7b7;color:#6c685e;font-weight:600;">{{.}}</th>{{end}}</tr>
{{- range .Rows}}
<tr><td style="padding:6px 8px;border-bottom:1px solid #e6e0d4;white-space:nowrap;font-family:ui-monospace,Menlo,Consolas,monospace;">
{{- if .URL}}<a href="{{.URL}}" style="color:#c2321b;text-decoration:none;">{{.Key}}</a>{{else}}{{.Key}}{{end}}</td>
{{- range .Cells}}<td style="padding:6px 8px;border-bottom:1px solid #e6e0d4;vertical-align:top;">{{.}}</td>{{end}}</tr>
{{- end}}
</table>
{{- else if .Empty}}
<p style="margin:0;color:#6c685e;">{{.Empty}}</p>
{{- end}}
{{- if .More}}
<p style="margin:8px 0 0;">{{if .MoreURL}}<a href="{{.MoreURL}}" style="color:#c2321b;">{{.More}}</a>{{else}}{{.More}}{{end}}</p>
{{- end}}
{{- end}}
<p style="margin:24px 0 0;padding-top:12px;border-top:1px solid #e6e0d4;color:#6c685e;font-size:12px;">{{.Why}}
{{- if .PrefsURL}}<br><a href="{{.PrefsURL}}" style="color:#6c685e;">{{.Prefs}}</a>{{else if .Prefs}}<br>{{.Prefs}}{{end}}</p>
</div>
</body></html>
`))

func (p *page) html() (string, error) {
	var b bytes.Buffer
	if err := htmlPage.Execute(&b, p); err != nil {
		return "", err
	}
	return b.String(), nil
}

// text renders the page as plain text.
func (p *page) text() string {
	// French puts a space before a colon.
	colon := ": "
	if p.Lang == "fr" {
		colon = " : "
	}
	var b strings.Builder
	if p.Heading != "" {
		b.WriteString(p.Heading + "\n\n")
	}
	for _, s := range p.Intro {
		b.WriteString(s + "\n\n")
	}
	for _, s := range p.Sections {
		b.WriteString(s.Key + " " + s.Summary + "\n")
		if s.Meta != "" {
			b.WriteString(s.Meta + "\n")
		}
		for _, e := range s.Entries {
			b.WriteString("\n" + e.Headline)
			if e.When != "" {
				b.WriteString(" (" + e.When + ")")
			}
			b.WriteString("\n")
			for _, c := range e.Changes {
				b.WriteString("  " + c.Field + colon)
				if c.From != "" {
					b.WriteString(c.From + " → ")
				}
				b.WriteString(c.To + "\n")
			}
			if e.Text != "" {
				for l := range strings.SplitSeq(e.Text, "\n") {
					b.WriteString("  > " + l + "\n")
				}
			}
			if e.Note != "" {
				b.WriteString("  " + e.Note + "\n")
			}
		}
		if s.URL != "" {
			b.WriteString("\n" + s.URL + "\n")
		}
		b.WriteString("\n" + strings.Repeat("—", 20) + "\n\n")
	}
	for _, t := range p.Tables {
		if t.Title != "" {
			b.WriteString(t.Title + "\n\n")
		}
		if len(t.Rows) == 0 && t.Empty != "" {
			b.WriteString(t.Empty + "\n")
		}
		for _, r := range t.Rows {
			b.WriteString("- " + r.Key + "  " + strings.Join(r.Cells, " · ") + "\n")
		}
		if t.More != "" {
			b.WriteString("\n" + t.More)
			if t.MoreURL != "" {
				b.WriteString(colon + t.MoreURL)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("--\n" + p.Why + "\n")
	if p.Prefs != "" {
		b.WriteString(p.Prefs)
		if p.PrefsURL != "" {
			b.WriteString(colon + p.PrefsURL)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// Dates as each language writes them.
var dateLayouts = map[string]string{"fr": "02/01/2006", "en": "2006-01-02", "de": "02.01.2006", "it": "02/01/2006",
	"hi": "02/01/2006"}

func formatDate(locale string, t time.Time) string {
	layout, ok := dateLayouts[locale]
	if !ok {
		layout = dateLayouts["fr"]
	}
	return t.Format(layout)
}

// formatDay formats a YYYY-MM-DD day.
func formatDay(locale, day string) string {
	t, err := time.Parse(time.DateOnly, day)
	if err != nil {
		return day
	}
	return formatDate(locale, t)
}

func formatTime(locale string, t time.Time) string {
	return formatDate(locale, t) + " " + t.Format("15:04")
}

// excerpt shortens a long text.
func excerpt(s string, n int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return strings.TrimSpace(string(r[:n])) + " …"
}

// fieldLabel names a history field in a language (Jira's field names, as
// the issue history shows them).
func fieldLabel(locale, field string) string {
	var m i18n.Message
	switch strings.ToLower(field) {
	case "status":
		m = i18n.M("Status")
	case "assignee":
		m = i18n.M("Assignee")
	case "reporter":
		m = i18n.M("Reporter")
	case "resolution":
		m = i18n.M("Resolution")
	case "priority":
		m = i18n.M("Priority")
	case "summary":
		m = i18n.M("Summary")
	case "description":
		m = i18n.M("Description")
	case "environment":
		m = i18n.M("Environment")
	case "labels":
		m = i18n.M("Labels")
	case "component":
		m = i18n.M("Component")
	case "fix version":
		m = i18n.M("Fix version")
	case "version":
		m = i18n.M("Affects version")
	case "duedate":
		m = i18n.M("Due date")
	case "issuetype":
		m = i18n.M("Issue type")
	case "attachment":
		m = i18n.M("Attachment")
	case "link":
		m = i18n.M("Link")
	case "parent":
		m = i18n.M("Parent")
	case "epic link":
		m = i18n.M("Epic")
	case "sprint":
		m = i18n.M("Sprint")
	default:
		return field
	}
	return m.In(locale)
}
