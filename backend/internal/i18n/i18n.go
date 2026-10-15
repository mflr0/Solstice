// Package i18n translates the messages Solstice shows its users.
//
// English is the source language: messages are written in English in the
// code, as fmt templates ("Unknown status: %s."), and that English template
// is the key of every language's catalogue. A translation may reorder the
// values with explicit argument indexes (%[2]s … %[1]s).
package i18n

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Default is the language of the source messages, used when nothing else is
// known.
const Default = "en"

// Locales are the supported languages, the default first.
var Locales = []string{"en", "fr", "de", "it", "hi"}

// catalogues holds the translations of each language but English, keyed by
// English template.
var catalogues = map[string]map[string]string{"fr": fr, "de": de, "it": it, "hi": hi}

// Supported tells whether locale is one of Locales.
func Supported(locale string) bool { return slices.Contains(Locales, locale) }

// Negotiate picks the supported language an Accept-Language header prefers:
// highest q-value first, earlier on a tie, regional variants counting as
// their language (de-CH is de). It returns "" when the header names no
// supported language, so the caller can fall back on something else.
func Negotiate(header string) string {
	best, bestQ := "", 0.0
	for part := range strings.SplitSeq(header, ",") {
		tag, params, _ := strings.Cut(part, ";")
		q := 1.0
		for p := range strings.SplitSeq(params, ";") {
			if k, v, ok := strings.Cut(p, "="); ok && strings.TrimSpace(k) == "q" {
				f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
				if err != nil {
					f = 0
				}
				q = f
			}
		}
		lang, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(tag)), "-")
		lang, _, _ = strings.Cut(lang, "_")
		if q > bestQ && Supported(lang) {
			best, bestQ = lang, q
		}
	}
	return best
}

// Message is a message not rendered yet: an English template and its values.
// Passed as a value of another message, it is rendered in the same language.
type Message struct {
	Template string
	Args     []any
}

// M builds a Message.
func M(template string, args ...any) Message { return Message{Template: template, Args: args} }

// In renders the message in a language.
func (m Message) In(locale string) string { return Sprintf(locale, m.Template, m.Args...) }

// String renders the message in the default language (English).
func (m Message) String() string { return m.In(Default) }

// Sprintf renders an English template in a language: its translation (the
// template itself in English, or when none exists) with the values inserted.
// Without values the text is returned as is, so it needs no % escaping.
func Sprintf(locale, template string, args ...any) string {
	if t, ok := catalogues[locale][template]; ok {
		template = t
	}
	if len(args) == 0 {
		return template
	}
	values := make([]any, len(args))
	for i, a := range args {
		if m, ok := a.(Message); ok {
			a = m.In(locale)
		}
		values[i] = a
	}
	return fmt.Sprintf(template, values...)
}
