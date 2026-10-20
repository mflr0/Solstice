package api

import (
	"testing"

	"solstice/internal/mail"
)

// validEmail and mail.ValidAddress differ only by ValidAddress's CR/LF check:
// net/mail already refuses CR and LF, so both give the same answer.
func TestEmailValidatorsAgree(t *testing.T) {
	for _, c := range []struct {
		in   string
		want bool
	}{
		{"", false},
		{" ", false},
		{"marie@example.org", true},
		{"Marie@Example.org", true},
		{"marie@localhost", true},
		{"marie@[192.0.2.1]", true},
		{"mårie@exämple.org", true},

		{"marie@example.org\r", false},
		{"marie@example.org\n", false},
		{"marie@example.org\r\n", false},
		{"\r\nmarie@example.org", false},
		{"ma\rrie@example.org", false},
		{"ma\nrie@example.org", false},
		{"marie@exa\r\nmple.org", false},
		{"marie@example.org\r\nBcc: paul@example.org", false},
		{"marie@example.org\r\n ", false},
		{"marie@\r\n example.org", false},
		{"marie\r\n @example.org", false},
		{"\"ma\r\nrie\"@example.org", false},
		{"\"ma\\\rrie\"@example.org", false},
		{"Marie\r\n <marie@example.org>", false},

		{`"marie"@example.org`, false},
		{`"ma rie"@example.org`, false},
		{`"ma\"rie"@example.org`, false},

		{"Marie <marie@example.org>", false},
		{`"Marie" <marie@example.org>`, false},
		{"=?utf-8?q?Marie?= <marie@example.org>", false},
		{"<marie@example.org>", false},
		{"marie@example.org (Marie)", false},
		{"(Marie)marie@example.org", false},

		{" marie@example.org", false},
		{"marie@example.org ", false},
		{"\tmarie@example.org", false},
		{"marie@example.org\t", false},
		{"marie@example.org x", false},
		{"marie@example.org>", false},
		{"marie@example.org, paul@example.org", false},

		{"marie", false},
		{"@example.org", false},
		{"marie@", false},
		{"marie@@example.org", false},
		{".marie@example.org", false},
		{"marie@example..org", false},
		{"ma\x00rie@example.org", false},
	} {
		got, mailGot := validEmail(c.in), mail.ValidAddress(c.in)
		if got != mailGot {
			t.Errorf("%q: validEmail %v, mail.ValidAddress %v", c.in, got, mailGot)
		} else if got != c.want {
			t.Errorf("%q: both %v, want %v", c.in, got, c.want)
		}
	}
}
