package api

import (
	"errors"
	"net/http"
	"testing"

	"solstice/internal/i18n"
	"solstice/internal/mail"
)

func TestMailSettingsMerge(t *testing.T) {
	str := func(s string) *string { return &s }
	stored := mail.Config{Host: "smtp.exemple.fr", Username: "relai", From: "a@exemple.fr"}
	got := mailSettingsIn{Security: str(mail.SecurityTLS), Host: str("  smtp.autre.fr "), FromName: str(" Tickets ")}.merge(stored)
	want := mail.Config{Host: "smtp.autre.fr", Port: 465, Security: mail.SecurityTLS, Username: "relai",
		From: "a@exemple.fr", FromName: "Tickets"}
	if got != want {
		t.Errorf("merge:\n got %+v\nwant %+v", got, want)
	}
	stored.Port = 2525
	if got := (mailSettingsIn{}).merge(stored); got.Security != mail.SecurityStartTLS || got.Port != 2525 {
		t.Errorf("nothing given, stored port 2525: %+v", got)
	}
}

func TestValidateMailSettings(t *testing.T) {
	ok := mail.Config{Enabled: true, Host: "smtp.exemple.fr", Port: 587, Security: mail.SecurityStartTLS,
		From: "solstice@exemple.fr"}
	if err := validateMailSettings(ok); err != nil {
		t.Fatalf("valid settings: %v", err)
	}
	for name, c := range map[string]struct {
		change func(*mail.Config)
		want   i18n.Key
	}{
		"unknown security": {func(c *mail.Config) { c.Security = "ssl" }, i18n.MailSecurityUnknown},
		"port 0":           {func(c *mail.Config) { c.Port = 0 }, i18n.MailPortInvalid},
		"port 65536":       {func(c *mail.Config) { c.Port = 65536 }, i18n.MailPortInvalid},
		"line break":       {func(c *mail.Config) { c.FromName = "a\r\nBcc: x@exemple.fr" }, i18n.FieldValueInvalid},
		"no server":        {func(c *mail.Config) { c.Host = "" }, i18n.MailServerRequired},
		"bad sender":       {func(c *mail.Config) { c.From = "pas une adresse" }, i18n.MailSenderInvalid},
		"security first":   {func(c *mail.Config) { c.Security, c.Port, c.Host = "ssl", 0, "" }, i18n.MailSecurityUnknown},
	} {
		cfg := ok
		c.change(&cfg)
		var he *httpError
		if err := validateMailSettings(cfg); !errors.As(err, &he) || he.status != http.StatusBadRequest || he.msg.Key != c.want {
			t.Errorf("%s: %v, want 400 %s", name, err, c.want)
		}
	}
	off := mail.Config{Port: 587, Security: mail.SecurityStartTLS}
	if err := validateMailSettings(off); err != nil {
		t.Errorf("disabled, no server, no sender: %v", err)
	}
}

// A kept password sent to a new destination is refused before credentials
// in clear are.
func TestMailCredentialsRule(t *testing.T) {
	before := mail.Config{Host: "smtp.exemple.fr", Port: 587, Security: mail.SecurityStartTLS, Username: "relai"}
	plain := before
	plain.Security, plain.Port = mail.SecurityNone, 25
	for name, c := range map[string]struct {
		after                     mail.Config
		keepPassword, hadPassword bool
		want                      i18n.Key
	}{
		"kept, moved":         {plain, true, true, i18n.MailPasswordRequired},
		"given, moved":        {plain, false, true, i18n.MailCredentialsNeedTLS},
		"kept, none, moved":   {plain, true, false, i18n.MailCredentialsNeedTLS},
		"kept, same place":    {before, true, true, ""},
		"given, encrypted":    {before, false, true, ""},
		"removed, same place": {before, false, false, ""},
	} {
		err := mailCredentialsRule(before, c.after, c.keepPassword, c.hadPassword)
		var he *httpError
		switch {
		case c.want == "" && err != nil:
			t.Errorf("%s: %v, want none", name, err)
		case c.want != "" && (!errors.As(err, &he) || he.status != http.StatusBadRequest || he.msg.Key != c.want):
			t.Errorf("%s: %v, want 400 %s", name, err, c.want)
		}
	}
}
