package tests

// The e-mail outbox: retries, giving up, and the queue's statistics.

import (
	"context"
	"testing"
	"time"

	"solstice/internal/mail"
	"solstice/internal/mail/mailtest"
)

func TestOutbox(t *testing.T) {
	needDB(t)
	conn := testDB
	mailtest.Lock(t, conn)
	srv := mailtest.Start(t)
	ctx := context.Background()
	cfg := mail.Config{Enabled: true, Host: srv.Host, Port: srv.Port, Security: mail.SecurityNone, From: "solstice@exemple.fr"}
	ob := mail.NewOutbox(conn, quiet(),
		func(context.Context) (mail.Config, error) { return cfg, nil })
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
		if err := ob.Enqueue(ctx, conn, mail.Message{Kind: "outbox-test", To: to, Subject: "Rappel", Text: "t", HTML: "h",
			MessageID: mail.NewMessageID("exemple.fr")}); err != nil {
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

	// Giving up after mail.MaxAttempts.
	conn.Exec(`UPDATE mail_outbox SET status = 'pending', attempts = ?, next_attempt_at = ? WHERE kind = 'outbox-test'`,
		mail.MaxAttempts-1, clock)
	srv.FailWith("451 later")
	ob.SendDue(ctx)
	var failed int
	conn.QueryRow(`SELECT COUNT(*) FROM mail_outbox WHERE kind = 'outbox-test' AND status = 'failed'`).Scan(&failed)
	if failed != 2 {
		t.Errorf("%d messages given up after %d attempts, want 2", failed, mail.MaxAttempts)
	}
}
