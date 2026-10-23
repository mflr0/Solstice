package notify

import (
	"strings"
	"testing"

	"solstice/internal/i18n"
	"solstice/internal/mail"
)

// The reset and invitation e-mails carry the link in both parts, in every
// language, and escape what they were given.
func TestLinkMessages(t *testing.T) {
	cfg := mail.Config{From: "solstice@exemple.fr"}
	const link = "https://tickets.exemple.fr/reset/abc_DEF-123"
	for _, lang := range i18n.Locales {
		marie := Account{UserID: 7, Email: "marie@exemple.fr", Name: "Marie <b>", Username: `marie"<i>`, Locale: lang}
		for name, m := range map[string]mail.Message{
			"reset":      ResetMessage(cfg, marie, link),
			"invitation": InvitationMessage(cfg, marie, link),
		} {
			if !strings.Contains(m.Text, link) || !strings.Contains(m.HTML, `href="`+link+`"`) {
				t.Errorf("%s/%s: the link is missing from the text or the HTML part", lang, name)
			}
			if m.Subject == "" || m.To != "marie@exemple.fr" || m.UserID != 7 {
				t.Errorf("%s/%s: message %+v", lang, name, m)
			}
			if m.Kind != mail.KindReset && m.Kind != mail.KindInvitation {
				t.Errorf("%s/%s: kind %q", lang, name, m.Kind)
			}
			if strings.Contains(m.HTML, `<i>`) {
				t.Errorf("%s/%s: a user name is not escaped in the HTML part", lang, name)
			}
			if !strings.Contains(m.HTML, `lang="`+lang+`"`) {
				t.Errorf("%s/%s: not written in its language", lang, name)
			}
		}
	}
	in := func(locale string) Account {
		return Account{UserID: 1, Email: "a@exemple.fr", Name: "A", Username: "a", Locale: locale}
	}
	en := ResetMessage(cfg, in("en"), link)
	if fr := ResetMessage(cfg, in("fr"), link); en.Subject == fr.Subject {
		t.Error("English and French share a subject")
	}
	if got := ResetMessage(cfg, in("xx"), link); got.Subject != en.Subject {
		t.Error("an unknown language does not fall back to English")
	}
	if inv := InvitationMessage(cfg, in("en"), link); inv.Kind != mail.KindInvitation ||
		!strings.Contains(inv.Text, "7 days") {
		t.Errorf("invitation: %q", inv.Text)
	}
}
