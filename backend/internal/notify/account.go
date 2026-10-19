package notify

import (
	"solstice/internal/i18n"
	"solstice/internal/mail"
)

// Account is the account an e-mail about it goes to.
type Account struct {
	UserID int64
	Email  string
	Name   string // display name
	// Username names the account in the text: several may share an address.
	Username string
	Locale   string // of the e-mail; English when unsupported
}

// ResetMessage is the e-mail with the link to choose a new password. link is
// built by the caller from the instance's public address and carries the
// token.
func ResetMessage(cfg mail.Config, to Account, link string) mail.Message {
	return linkMessage(cfg, mail.KindReset, to, link, linkTexts{
		subject: i18n.M(i18n.MailResetSubject),
		intro:   i18n.M(i18n.MailResetIntro, to.Username),
		action:  i18n.M(i18n.MailResetAction),
		outro:   []i18n.Message{i18n.M(i18n.MailResetExpiry), i18n.M(i18n.MailResetIgnore)},
		why:     i18n.M(i18n.MailResetWhy),
	})
}

// InvitationMessage is the e-mail inviting someone to an account an
// administrator created: the link lets them choose their password.
func InvitationMessage(cfg mail.Config, to Account, link string) mail.Message {
	return linkMessage(cfg, mail.KindInvitation, to, link, linkTexts{
		subject: i18n.M(i18n.MailInviteSubject),
		intro:   i18n.M(i18n.MailInviteIntro, to.Username),
		action:  i18n.M(i18n.MailInviteAction),
		outro:   []i18n.Message{i18n.M(i18n.MailInviteExpiry)},
		why:     i18n.M(i18n.MailInviteWhy),
	})
}

// linkTexts are the texts of a message with a link, before translation.
type linkTexts struct {
	subject, intro, action, why i18n.Message
	outro                       []i18n.Message
}

func linkMessage(cfg mail.Config, kind string, to Account, link string, x linkTexts) mail.Message {
	locale := i18n.Pick(to.Locale)
	p := &page{Lang: locale, Title: x.subject.In(locale)}
	p.Heading = p.Title
	p.Intro = []string{x.intro.In(locale)}
	p.Action = action{Label: x.action.In(locale), URL: link}
	for _, m := range x.outro {
		p.Outro = append(p.Outro, m.In(locale))
	}
	p.Why = x.why.In(locale)
	return mail.Message{Kind: kind, UserID: to.UserID, To: to.Email, ToName: to.Name, Subject: p.Title, Text: p.text(),
		HTML: p.html(), MessageID: mail.NewMessageID(cfg.Domain())}
}
