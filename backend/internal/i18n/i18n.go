// Package i18n translates the messages Solstice shows its users.
//
// Code names a message by its Key (keys.go); every language's catalogue maps
// it to a fmt template ("Unknown status: %s."), English being the source
// (en.go). A translation may reorder the values with explicit argument
// indexes (%[2]s … %[1]s).
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

// Key names a message. Its value is its own name.
type Key string

// catalogues holds the templates of each language.
var catalogues = map[string]map[Key]string{"en": en, "fr": fr, "de": de, "it": it, "hi": hi}

// Supported tells whether locale is one of Locales.
func Supported(locale string) bool { return slices.Contains(Locales, locale) }

// Pick returns the first supported language among candidates, in order, or
// Default when none of them is supported.
func Pick(candidates ...string) string {
	for _, c := range candidates {
		if Supported(c) {
			return c
		}
	}
	return Default
}

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

// Message is a message not rendered yet: a key and its values, or a text
// shown as is. Passed as a value of another message, it is rendered in the
// same language.
type Message struct {
	Key  Key
	Args []any
	raw  string
}

// M builds a Message.
func M(key Key, args ...any) Message { return Message{Key: key, Args: args} }

// N builds a Message counting n: key one when n is 1, many otherwise, with n
// as its first value.
func N(n int, one, many Key, args ...any) Message {
	key := many
	if n == 1 {
		key = one
	}
	return Message{Key: key, Args: append([]any{n}, args...)}
}

// Raw builds a Message that is not translated: the text shows as is in
// every language.
func Raw(text string) Message { return Message{raw: text} }

// In renders the message in a language.
func (m Message) In(locale string) string {
	if m.Key == "" {
		return m.raw
	}
	return Sprintf(locale, m.Key, m.Args...)
}

// String renders the message in the default language (English).
func (m Message) String() string { return m.In(Default) }

// MarshalText renders the message in English, as JSON does.
func (m Message) MarshalText() ([]byte, error) { return []byte(m.String()), nil }

// Sprintf renders a message in a language: its template (the English one
// when the language has none) with the values inserted. Without values the
// template is returned as is, so it needs no % escaping.
func Sprintf(locale string, key Key, args ...any) string {
	template, ok := catalogues[locale][key]
	if !ok {
		template = en[key]
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

// Match returns the key among keys whose text, in any language, is text, or
// "" when none is. It tells a name Solstice wrote (a seeded default) from one
// someone typed.
func Match(text string, keys ...Key) Key {
	for _, k := range keys {
		for _, c := range catalogues {
			if template, ok := c[k]; ok && template == text {
				return k
			}
		}
	}
	return ""
}
