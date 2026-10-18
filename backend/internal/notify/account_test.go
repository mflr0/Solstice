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
		for name, m := range map[string]mail.Message{
			"reset":      ResetMessage(cfg, 7, "marie@exemple.fr", "Marie <b>", `marie"<i>`, lang, link),
			"invitation": InvitationMessage(cfg, 7, "marie@exemple.fr", "Marie", "marie", lang, link),
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
	en := ResetMessage(cfg, 1, "a@exemple.fr", "A", "a", "en", link)
	fr := ResetMessage(cfg, 1, "a@exemple.fr", "A", "a", "fr", link)
	if en.Subject == fr.Subject {
		t.Error("English and French share a subject")
	}
	if got := ResetMessage(cfg, 1, "a@exemple.fr", "A", "a", "xx", link); got.Subject != en.Subject {
		t.Error("an unknown language does not fall back to English")
	}
	if inv := InvitationMessage(cfg, 1, "a@exemple.fr", "A", "a", "en", link); inv.Kind != mail.KindInvitation ||
		!strings.Contains(inv.Text, "7 days") {
		t.Errorf("invitation: %q", inv.Text)
	}
}
