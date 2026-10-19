// Package mailtest is an SMTP server for tests: it accepts everything (but
// recipients containing "reject", refused with 550) and keeps the messages.
package mailtest

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"strings"
	"sync"
	"testing"
	"time"
)

// Message is a received e-mail, decoded.
type Message struct {
	From    string
	To      string
	Header  mail.Header
	Subject string
	Text    string
	HTML    string
	Raw     string
}

// Server is a running fake SMTP server.
type Server struct {
	Host string
	Port int

	mu   sync.Mutex
	fail string   // when set, the reply to MAIL FROM (e.g. "451 try later")
	auth []string // credentials received (user:pass)
	msgs []Message
	ln   net.Listener
	wg   sync.WaitGroup
}

// Start runs a server until the test ends.
func Start(t testing.TB) *Server {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().(*net.TCPAddr)
	s := &Server{Host: "127.0.0.1", Port: addr.Port, ln: ln}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			s.wg.Add(1)
			go func() {
				defer s.wg.Done()
				s.serve(c)
			}()
		}
	}()
	t.Cleanup(func() {
		ln.Close()
		s.wg.Wait()
	})
	return s
}

// FailWith makes the server refuse senders with an SMTP reply ("" to stop).
func (s *Server) FailWith(reply string) {
	s.mu.Lock()
	s.fail = reply
	s.mu.Unlock()
}

// Messages returns what was received so far.
func (s *Server) Messages() []Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Message(nil), s.msgs...)
}

// Auth returns the credentials received, as user:pass.
func (s *Server) Auth() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.auth...)
}

// Reset forgets the messages received.
func (s *Server) Reset() {
	s.mu.Lock()
	s.msgs = nil
	s.mu.Unlock()
}

func (s *Server) serve(c net.Conn) {
	defer c.Close()
	c.SetDeadline(time.Now().Add(30 * time.Second))
	r := bufio.NewReader(c)
	reply := func(line string) { io.WriteString(c, line+"\r\n") }
	reply("220 mailtest ready")
	var from string
	var to []string
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		verb := strings.ToUpper(strings.SplitN(line, " ", 2)[0])
		switch verb {
		case "EHLO", "HELO":
			reply("250-mailtest")
			reply("250-AUTH PLAIN LOGIN")
			reply("250 8BITMIME")
		case "AUTH":
			parts := strings.Fields(line)
			if len(parts) >= 3 && strings.EqualFold(parts[1], "PLAIN") {
				raw, _ := base64.StdEncoding.DecodeString(parts[2])
				f := strings.Split(string(raw), "\x00")
				if len(f) == 3 {
					s.mu.Lock()
					s.auth = append(s.auth, f[1]+":"+f[2])
					s.mu.Unlock()
				}
				reply("235 ok")
			} else {
				reply("504 unsupported")
			}
		case "MAIL":
			s.mu.Lock()
			fail := s.fail
			s.mu.Unlock()
			if fail != "" {
				reply(fail)
				continue
			}
			from = strings.Trim(strings.TrimPrefix(line[len("MAIL FROM:"):], " "), "<>")
			to = nil
			reply("250 ok")
		case "RCPT":
			addr := strings.Trim(strings.TrimSpace(line[len("RCPT TO:"):]), "<>")
			if strings.Contains(addr, "reject") {
				reply("550 5.1.1 no such user")
				continue
			}
			to = append(to, addr)
			reply("250 ok")
		case "DATA":
			reply("354 go on")
			var b strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if l == ".\r\n" || l == ".\n" {
					break
				}
				b.WriteString(strings.TrimPrefix(l, "."))
			}
			for _, rcpt := range to {
				s.store(from, rcpt, b.String())
			}
			reply("250 queued")
		case "RSET":
			from, to = "", nil
			reply("250 ok")
		case "NOOP":
			reply("250 ok")
		case "QUIT":
			reply("221 bye")
			return
		default:
			reply("502 unknown")
		}
	}
}

func (s *Server) store(from, to, raw string) {
	m := Message{From: from, To: to, Raw: raw}
	if msg, err := mail.ReadMessage(strings.NewReader(raw)); err == nil {
		m.Header = msg.Header
		dec := new(mime.WordDecoder)
		m.Subject, _ = dec.DecodeHeader(msg.Header.Get("Subject"))
		_, params, _ := mime.ParseMediaType(msg.Header.Get("Content-Type"))
		mr := multipart.NewReader(msg.Body, params["boundary"])
		for {
			p, err := mr.NextPart()
			if err != nil {
				break
			}
			var body []byte
			if p.Header.Get("Content-Transfer-Encoding") == "quoted-printable" {
				body, _ = io.ReadAll(quotedprintable.NewReader(p))
			} else {
				body, _ = io.ReadAll(p)
			}
			if strings.HasPrefix(p.Header.Get("Content-Type"), "text/html") {
				m.HTML = string(body)
			} else {
				m.Text = string(body)
			}
		}
	}
	s.mu.Lock()
	s.msgs = append(s.msgs, m)
	s.mu.Unlock()
}

// Lock serializes, across test binaries, the tests that use the outbox and
// webhook tables of the shared test database (go test runs packages in
// parallel). It is released when the test ends.
func Lock(t testing.TB, db *sql.DB) {
	t.Helper()
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var got sql.NullInt64
	if err := conn.QueryRowContext(ctx, `SELECT GET_LOCK('solstice-outgoing-tests', 300)`).Scan(&got); err != nil || got.Int64 != 1 {
		conn.Close()
		t.Fatalf("test lock not obtained: %v", err)
	}
	t.Cleanup(func() {
		conn.ExecContext(ctx, `SELECT RELEASE_LOCK('solstice-outgoing-tests')`)
		conn.Close()
	})
}
