package api

import (
	"testing"

	"solstice/internal/mail"
)

func TestEmailValidatorsAgree(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"plain", "marie@example.org", true},
		{"plus tag", "marie+tag@example.org", true},
		{"empty", "", false},
		{"space only", " ", false},
		{"no domain", "marie@", false},
		{"no local part", "@example.org", false},
		{"no at sign", "marie", false},
		{"leading space", " marie@example.org", false},
		{"trailing space", "marie@example.org ", false},
		{"display name", "Marie <marie@example.org>", false},
		{"quoted display name", `"Marie" <marie@example.org>`, false},
		{"angle brackets only", "<marie@example.org>", false},
		{"quoted local part", `"marie"@example.org`, false},
		{"quoted local part with space", `"ma rie"@example.org`, false},
		{"trailing text", "marie@example.org junk", false},
		{"trailing comma", "marie@example.org,", false},
		{"two addresses", "a@example.org, b@example.org", false},
		{"comment", "marie@example.org (Marie)", false},
		{"trailing CR", "marie@example.org\r", false},
		{"trailing LF", "marie@example.org\n", false},
		{"trailing CRLF", "marie@example.org\r\n", false},
		{"leading CR", "\rmarie@example.org", false},
		{"leading LF", "\nmarie@example.org", false},
		{"CR in local part", "ma\rrie@example.org", false},
		{"LF in local part", "ma\nrie@example.org", false},
		{"LF in domain", "marie@exam\nple.org", false},
		{"CRLF then header", "marie@example.org\r\nBcc: x@example.org", false},
		{"LF then header", "marie@example.org\nBcc: x@example.org", false},
		{"CR in display name", "Marie\r <marie@example.org>", false},
		{"LF in display name", "Marie\n <marie@example.org>", false},
		{"CR in quoted local part", "\"ma\rrie\"@example.org", false},
		{"LF in quoted local part", "\"ma\nrie\"@example.org", false},
		{"CRLF fold in quoted local part", "\"ma\r\n rie\"@example.org", false},
		{"CRLF fold before angle", "Marie\r\n <marie@example.org>", false},
		{"CR in comment", "marie@example.org (a\rb)", false},
		{"LF in comment", "marie@example.org (a\nb)", false},
		{"CRLF fold in comment", "marie@example.org (a\r\n b)", false},
		{"NUL", "marie\x00@example.org", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validEmail(tt.in); got != tt.want {
				t.Errorf("validEmail(%q) = %v, want %v", tt.in, got, tt.want)
			}
			if got := mail.ValidAddress(tt.in); got != tt.want {
				t.Errorf("mail.ValidAddress(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
