package mail

import (
	"context"
	"io"
	"log/slog"
	"mime"
	"net/mail"
	"strings"
	"testing"
	"time"

	"solstice/internal/mail/mailtest"
)

func TestMessageBytes(t *testing.T) {
	m := Message{
		To: "lea@exemple.fr", ToName: "Léa Richard",
		Subject:   "[RH-3] Entretien annuel\r\nBcc: espion@exemple.fr — à préparer",
		Text:      "Bonjour,\nligne deux",
		HTML:      "<p>Bonjour</p>",
		MessageID: "<abc@exemple.fr>", InReplyTo: "<issue.10.1@exemple.fr>",
	}
	raw, err := m.Bytes(mail.Address{Name: "Solstice", Address: "solstice@exemple.fr"}, time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(raw), "\r\n") {
		if strings.HasPrefix(line, "Bcc:") {
			t.Fatalf("header injected through the subject:\n%s", raw)
		}
		if len(line) > 998 {
			t.Fatalf("line longer than 998 characters")
		}
	}
	msg, err := mail.ReadMessage(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	subject, _ := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject"))
	if !strings.HasPrefix(subject, "[RH-3] Entretien annuel") || !strings.Contains(subject, "à préparer") {
		t.Errorf("subject = %q", subject)
	}
	if msg.Header.Get("In-Reply-To") != "<issue.10.1@exemple.fr>" || msg.Header.Get("References") != "<issue.10.1@exemple.fr>" {
		t.Errorf("threading headers missing: %v", msg.Header)
	}
	if msg.Header.Get("Auto-Submitted") != "auto-generated" {
		t.Error("Auto-Submitted missing")
	}
	to, _ := msg.Header.AddressList("To")
	if len(to) != 1 || to[0].Name != "Léa Richard" || to[0].Address != "lea@exemple.fr" {
		t.Errorf("To = %v", to)
	}

	// A long subject in Devanagari folds into several encoded words.
	m.Subject = strings.Repeat("टिकट का सारांश ", 40)
	raw, _ = m.Bytes(mail.Address{Address: "s@exemple.fr"}, time.Now())
	for _, line := range strings.Split(string(raw), "\r\n") {
		if len(line) > 998 {
			t.Fatal("long subject not folded")
		}
	}
}

func TestSend(t *testing.T) {
	srv := mailtest.Start(t)
	cfg := Config{Enabled: true, Host: srv.Host, Port: srv.Port, Security: SecurityNone, Username: "relai",
		Password: "mot de passe", From: "solstice@exemple.fr", FromName: "Solstice"}
	ctx := context.Background()
	if err := Send(ctx, cfg, Message{To: "paul@exemple.fr", Subject: "Essai", Text: "texte", HTML: "<p>html</p>"}); err != nil {
		t.Fatal(err)
	}
	got := srv.Messages()
	if len(got) != 1 || got[0].To != "paul@exemple.fr" || got[0].Subject != "Essai" || got[0].Text != "texte" ||
		got[0].HTML != "<p>html</p>" {
		t.Fatalf("received %+v", got)
	}
	if auth := srv.Auth(); len(auth) != 1 || auth[0] != "relai:mot de passe" {
		t.Errorf("credentials received: %v", auth)
	}

	err := Send(ctx, cfg, Message{To: "reject@exemple.fr", Subject: "x"})
	if !Permanent(err) {
		t.Errorf("refused recipient: %v, want a permanent error", err)
	}
	srv.FailWith("451 4.3.0 try later")
	err = Send(ctx, cfg, Message{To: "paul@exemple.fr", Subject: "x"})
	var me *Error
	if err == nil || Permanent(err) || !asError(err, &me) || me.Stage != StageSender {
		t.Errorf("temporary failure: %v", err)
	}
	srv.FailWith("")

	cfg.Security = SecurityStartTLS
	if err := Send(ctx, cfg, Message{To: "paul@exemple.fr"}); !asError(err, &me) || me.Stage != StageNoTLS {
		t.Errorf("STARTTLS required but not offered: %v", err)
	}
	cfg.Security, cfg.Port = SecurityNone, 1
	if err := Send(ctx, cfg, Message{To: "paul@exemple.fr"}); !asError(err, &me) || me.Stage != StageConnect {
		t.Errorf("unreachable server: %v", err)
	}
}

func asError(err error, target **Error) bool {
	e, ok := err.(*Error)
	if ok {
		*target = e
	}
	return ok
}

// While a replacing import runs, the outbox does not even read its
// settings: the tick is skipped.
func TestSendDueSkippedWhilePaused(t *testing.T) {
	read := 0
	o := NewOutbox(nil, slog.New(slog.NewTextHandler(io.Discard, nil)), func(context.Context) (Config, error) {
		read++
		return Config{}, nil
	})
	paused := true
	o.Paused = func() bool { return paused }
	if n, err := o.SendDue(context.Background()); n != 0 || err != nil || read != 0 {
		t.Fatalf("paused: sent %d, err %v, settings read %d times", n, err, read)
	}
	paused = false
	o.SendDue(context.Background())
	if read != 1 {
		t.Errorf("not paused: settings read %d times, want 1", read)
	}
}

func TestLoopback(t *testing.T) {
	for host, want := range map[string]bool{
		"localhost": true, "LocalHost": true, "127.0.0.1": true, "127.12.0.9": true, "::1": true,
		"::ffff:127.0.0.1": true, "smtp.exemple.fr": false, "192.0.2.10": false, "0.0.0.0": false,
		"2001:db8::1": false, "": false, "127.0.0.1.exemple.fr": false,
	} {
		if got := Loopback(host); got != want {
			t.Errorf("Loopback(%q) = %v, want %v", host, got, want)
		}
	}
}

// Credentials are not sent over a plain connection to a server that is not
// local. 0.0.0.0 is no loopback address but reaches the test server.
func TestSendRefusesCredentialsInClear(t *testing.T) {
	srv := mailtest.Start(t)
	cfg := Config{Enabled: true, Host: "0.0.0.0", Port: srv.Port, Security: SecurityNone, Username: "relai",
		Password: "mot de passe", From: "solstice@exemple.fr"}
	err := Send(context.Background(), cfg, Message{To: "paul@exemple.fr", Subject: "Essai"})
	var me *Error
	if asError(err, &me) && me.Stage == StageConnect {
		t.Skipf("0.0.0.0 does not reach the local machine here: %v", err)
	}
	if !asError(err, &me) || me.Stage != StageInsecure {
		t.Fatalf("credentials over a plain connection: %v, want stage %s", err, StageInsecure)
	}
	if auth, msgs := srv.Auth(), srv.Messages(); len(auth) != 0 || len(msgs) != 0 {
		t.Errorf("the server received credentials %v and messages %v", auth, msgs)
	}
	cfg.Username = ""
	if err := Send(context.Background(), cfg, Message{To: "paul@exemple.fr", Subject: "Essai"}); err != nil {
		t.Errorf("no credentials, plain connection: %v", err)
	}
}
