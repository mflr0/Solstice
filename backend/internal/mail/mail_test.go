package mail

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"mime"
	"net/mail"
	"os"
	"strings"
	"testing"
	"time"

	"solstice/internal/db"
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

func testDB(t *testing.T) *sql.DB {
	dsn := os.Getenv("SOLSTICE_TEST_DB")
	if dsn == "" {
		t.Skip("SOLSTICE_TEST_DB not set: integration test skipped")
	}
	conn, err := db.Open(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(context.Background(), conn); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestOutbox(t *testing.T) {
	conn := testDB(t)
	mailtest.Lock(t, conn)
	srv := mailtest.Start(t)
	ctx := context.Background()
	cfg := Config{Enabled: true, Host: srv.Host, Port: srv.Port, Security: SecurityNone, From: "solstice@exemple.fr"}
	ob := NewOutbox(conn, slog.New(slog.NewTextHandler(io.Discard, nil)),
		func(context.Context) (Config, error) { return cfg, nil })
	clock := time.Now()
	ob.Now = func() time.Time { return clock }
	conn.Exec(`DELETE FROM mail_outbox WHERE kind = 'outbox-test'`)
	t.Cleanup(func() { conn.Exec(`DELETE FROM mail_outbox WHERE kind = 'outbox-test'`) })

	// Only this test's messages may be in the way.
	var others int
	conn.QueryRow(`SELECT COUNT(*) FROM mail_outbox WHERE status = 'pending'`).Scan(&others)
	if others > 0 {
		t.Skip("other messages are pending in the outbox")
	}
	for _, to := range []string{"ines@exemple.fr", "reject@exemple.fr"} {
		if err := ob.Enqueue(ctx, conn, Message{Kind: "outbox-test", To: to, Subject: "Rappel", Text: "t", HTML: "h",
			MessageID: NewMessageID("exemple.fr")}); err != nil {
			t.Fatal(err)
		}
	}
	srv.FailWith("421 4.3.2 busy")
	if n, err := ob.SendDue(ctx); err != nil || n != 0 {
		t.Fatalf("SendDue while the server fails: %d, %v", n, err)
	}
	var status string
	var attempts int
	var next time.Time
	conn.QueryRow(`SELECT status, attempts, next_attempt_at FROM mail_outbox WHERE kind = 'outbox-test' AND to_addr = ?`,
		"ines@exemple.fr").Scan(&status, &attempts, &next)
	if status != "pending" || attempts != 1 || !next.After(clock) {
		t.Fatalf("after a temporary failure: %s, %d attempts, next %v", status, attempts, next)
	}
	// Not due yet.
	srv.FailWith("")
	if n, _ := ob.SendDue(ctx); n != 0 {
		t.Fatalf("message retried before its delay: %d", n)
	}
	clock = clock.Add(2 * time.Minute)
	if n, err := ob.SendDue(ctx); err != nil || n != 1 {
		t.Fatalf("retry: %d sent, %v", n, err)
	}
	conn.QueryRow(`SELECT status FROM mail_outbox WHERE kind = 'outbox-test' AND to_addr = ?`, "reject@exemple.fr").Scan(&status)
	if status != "failed" {
		t.Errorf("refused recipient: status %s, want failed at once", status)
	}
	st, err := ob.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.Failed < 1 || len(st.Errors) == 0 || st.Errors[0].LastError == nil {
		t.Errorf("stats: %+v", st)
	}
	if n, err := ob.Retry(ctx); err != nil || n < 1 {
		t.Errorf("retry failed messages: %d, %v", n, err)
	}

	// Giving up after MaxAttempts.
	conn.Exec(`UPDATE mail_outbox SET status = 'pending', attempts = ?, next_attempt_at = ? WHERE kind = 'outbox-test'`,
		MaxAttempts-1, clock)
	srv.FailWith("451 later")
	ob.SendDue(ctx)
	var failed int
	conn.QueryRow(`SELECT COUNT(*) FROM mail_outbox WHERE kind = 'outbox-test' AND status = 'failed'`).Scan(&failed)
	if failed != 2 {
		t.Errorf("%d messages given up after %d attempts, want 2", failed, MaxAttempts)
	}
}
