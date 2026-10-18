package mail

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/smtp"
	"net/textproto"
	"os"
	"strconv"
	"strings"
	"time"
)

// Stages of an SMTP exchange, so a failure can be explained.
const (
	StageConnect   = "connect"   // reaching the server
	StageTLS       = "tls"       // TLS negotiation
	StageNoTLS     = "notls"     // STARTTLS asked but not offered
	StageAuth      = "auth"      // credentials refused
	StageNoAuth    = "noauth"    // credentials given but the server takes none
	StageInsecure  = "insecure"  // credentials given but the connection is not encrypted
	StageSender    = "sender"    // MAIL FROM refused
	StageRecipient = "recipient" // RCPT TO refused
	StageData      = "data"      // message refused
)

// Error is an SMTP failure and the stage where it happened.
type Error struct {
	Stage string
	Err   error
}

func (e *Error) Error() string { return e.Stage + ": " + e.Err.Error() }
func (e *Error) Unwrap() error { return e.Err }

// Permanent tells whether retrying cannot help: the server refused the
// recipient for good (5xx).
func Permanent(err error) bool {
	var me *Error
	var te *textproto.Error
	return errors.As(err, &me) && me.Stage == StageRecipient && errors.As(err, &te) && te.Code >= 500
}

// session is an SMTP connection reused for several messages.
type session struct {
	cfg    Config
	conn   net.Conn
	client *smtp.Client
}

func (s *session) close() {
	if s.client != nil {
		s.client.Quit()
		s.client.Close()
	}
	s.client, s.conn = nil, nil
}

func (s *session) dial(ctx context.Context) error {
	cfg := s.cfg
	port := cfg.Port
	if port == 0 {
		port = DefaultPort(cfg.Security)
	}
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(port))
	dialer := &net.Dialer{Timeout: 15 * time.Second}
	tlsConf := &tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12}
	var conn net.Conn
	var err error
	if cfg.Security == SecurityTLS {
		conn, err = (&tls.Dialer{NetDialer: dialer, Config: tlsConf}).DialContext(ctx, "tcp", addr)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return &Error{StageConnect, err}
	}
	conn.SetDeadline(time.Now().Add(time.Minute))
	c, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		conn.Close()
		return &Error{StageConnect, err}
	}
	fail := func(stage string, err error) error {
		c.Close()
		return &Error{stage, err}
	}
	if err := c.Hello(helloName()); err != nil {
		return fail(StageConnect, err)
	}
	if cfg.Security == SecurityStartTLS {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return fail(StageNoTLS, errors.New("the server does not offer STARTTLS"))
		}
		if err := c.StartTLS(tlsConf); err != nil {
			return fail(StageTLS, err)
		}
	}
	if cfg.Username != "" {
		if _, encrypted := c.TLSConnectionState(); !encrypted && !Loopback(cfg.Host) {
			return fail(StageInsecure, errors.New("credentials are not sent over an unencrypted connection"))
		}
		ok, mechanisms := c.Extension("AUTH")
		if !ok {
			return fail(StageNoAuth, errors.New("the server does not offer authentication"))
		}
		a, err := pickAuth(mechanisms, cfg.Username, cfg.Password)
		if err != nil {
			return fail(StageAuth, err)
		}
		if err := c.Auth(a); err != nil {
			return fail(StageAuth, err)
		}
	}
	s.conn, s.client = conn, c
	return nil
}

// send delivers one message, connecting first if needed. After a failure
// the connection is dropped, unless the server only refused this recipient.
func (s *session) send(ctx context.Context, from, to string, data []byte) error {
	if s.client == nil {
		if err := s.dial(ctx); err != nil {
			return err
		}
	}
	s.conn.SetDeadline(time.Now().Add(2 * time.Minute))
	c := s.client
	if err := c.Mail(from); err != nil {
		s.close()
		return &Error{StageSender, err}
	}
	if err := c.Rcpt(to); err != nil {
		if c.Reset() != nil {
			s.close()
		}
		return &Error{StageRecipient, err}
	}
	w, err := c.Data()
	if err != nil {
		s.close()
		return &Error{StageData, err}
	}
	if _, err := w.Write(data); err != nil {
		s.close()
		return &Error{StageData, err}
	}
	if err := w.Close(); err != nil {
		s.close()
		return &Error{StageData, err}
	}
	return nil
}

// Send delivers a single message over a new connection (the admin's test
// e-mail).
func Send(ctx context.Context, cfg Config, m Message) error {
	if m.MessageID == "" {
		m.MessageID = NewMessageID(cfg.Domain())
	}
	data, err := m.Bytes(cfg.Sender(), time.Now())
	if err != nil {
		return err
	}
	s := &session{cfg: cfg}
	defer s.close()
	return s.send(ctx, cfg.From, m.To, data)
}

func helloName() string {
	if h, err := os.Hostname(); err == nil && h != "" && !strings.ContainsAny(h, " \r\n") {
		return h
	}
	return "localhost"
}

// Loopback tells whether host is the local machine. Host names are not
// resolved, so only the spelling "localhost" and literal loopback addresses count.
func Loopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	a, err := netip.ParseAddr(host)
	return err == nil && a.Unmap().IsLoopback()
}

// pickAuth chooses PLAIN, LOGIN or CRAM-MD5, whichever the server offers.
// Credentials go in clear only over TLS, or to the local machine (see dial).
func pickAuth(offered, user, pass string) (smtp.Auth, error) {
	has := map[string]bool{}
	for _, m := range strings.Fields(strings.ToUpper(offered)) {
		has[m] = true
	}
	switch {
	case has["PLAIN"]:
		return &plainAuth{user: user, pass: pass}, nil
	case has["LOGIN"]:
		return &loginAuth{user: user, pass: pass}, nil
	case has["CRAM-MD5"]:
		return smtp.CRAMMD5Auth(user, pass), nil
	}
	return nil, fmt.Errorf("no supported authentication method (server offers: %s)", offered)
}

type plainAuth struct{ user, pass string }

func (a *plainAuth) Start(*smtp.ServerInfo) (string, []byte, error) {
	return "PLAIN", []byte("\x00" + a.user + "\x00" + a.pass), nil
}

func (a *plainAuth) Next(_ []byte, more bool) ([]byte, error) {
	if more {
		return nil, errors.New("unexpected challenge")
	}
	return nil, nil
}

// loginAuth is the LOGIN mechanism, which some servers offer instead of
// PLAIN: the user name, then the password, each on request.
type loginAuth struct {
	user, pass string
	step       int
}

func (a *loginAuth) Start(*smtp.ServerInfo) (string, []byte, error) { return "LOGIN", nil, nil }

func (a *loginAuth) Next(_ []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	a.step++
	switch a.step {
	case 1:
		return []byte(a.user), nil
	case 2:
		return []byte(a.pass), nil
	}
	return nil, errors.New("unexpected challenge")
}
