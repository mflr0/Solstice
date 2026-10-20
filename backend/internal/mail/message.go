package mail

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"net/textproto"
	"strings"
	"time"
)

// Message is an e-mail to one recipient, in text and HTML.
type Message struct {
	Kind      string // notification, digest, subscription, reminder, test, reset, invitation
	UserID    int64  // the recipient's account, 0 if none
	To        string
	ToName    string
	Subject   string
	Text      string
	HTML      string
	MessageID string // with angle brackets; generated when empty
	InReplyTo string // the thread this message belongs to, with angle brackets
}

// NewMessageID returns a unique message id in a domain.
func NewMessageID(domain string) string {
	raw := make([]byte, 12)
	rand.Read(raw)
	return "<" + hex.EncodeToString(raw) + "@" + domain + ">"
}

// headerValue drops line breaks so a value cannot inject headers.
func headerValue(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\r' || r == '\n' || r == 0 {
			return ' '
		}
		return r
	}, s)
}

// encodeHeader encodes a header value for non-ASCII text, folding long
// values so no line passes the 998 characters SMTP allows.
func encodeHeader(s string) string {
	enc := mime.QEncoding.Encode("utf-8", headerValue(s))
	return strings.ReplaceAll(enc, "?= =?utf-8?", "?=\r\n =?utf-8?")
}

// Bytes renders the message in MIME, from a sender at a date.
func (m *Message) Bytes(from mail.Address, date time.Time) ([]byte, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	to := mail.Address{Name: headerValue(m.ToName), Address: headerValue(m.To)}
	from.Name, from.Address = headerValue(from.Name), headerValue(from.Address)
	h := []string{
		"From: " + from.String(),
		"To: " + to.String(),
		"Subject: " + encodeHeader(m.Subject),
		"Date: " + date.Format(time.RFC1123Z),
		"Message-ID: " + headerValue(m.MessageID),
	}
	if m.InReplyTo != "" && m.InReplyTo != m.MessageID {
		h = append(h, "In-Reply-To: "+headerValue(m.InReplyTo), "References: "+headerValue(m.InReplyTo))
	}
	h = append(h,
		"MIME-Version: 1.0",
		"Content-Type: multipart/alternative; boundary=\""+mw.Boundary()+"\"",
		// Out-of-office replies and auto-responders should stay quiet.
		"Auto-Submitted: auto-generated",
		"X-Auto-Response-Suppress: All",
		"Precedence: bulk",
	)
	buf.WriteString(strings.Join(h, "\r\n") + "\r\n\r\n")
	for _, part := range []struct{ kind, body string }{{"text/plain", m.Text}, {"text/html", m.HTML}} {
		w, err := mw.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {part.kind + "; charset=utf-8"},
			"Content-Transfer-Encoding": {"quoted-printable"},
		})
		if err != nil {
			return nil, err
		}
		qp := quotedprintable.NewWriter(w)
		if _, err := qp.Write([]byte(strings.ReplaceAll(part.body, "\r\n", "\n"))); err != nil {
			return nil, err
		}
		if err := qp.Close(); err != nil {
			return nil, err
		}
	}
	if err := mw.Close(); err != nil {
		return nil, fmt.Errorf("building the message: %w", err)
	}
	return buf.Bytes(), nil
}
