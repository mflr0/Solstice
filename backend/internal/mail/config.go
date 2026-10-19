// Package mail sends Solstice's e-mails: SMTP settings kept in the database,
// messages built in text and HTML, and an outbox sent by a background worker
// that retries with growing delays.
package mail

import (
	"context"
	"net/mail"
	"strings"

	"solstice/internal/secret"
	"solstice/internal/settings"
)

// Security is how the SMTP connection is protected.
const (
	SecurityNone     = "none"     // plain text (port 25, internal relays)
	SecurityStartTLS = "starttls" // upgraded after connecting (port 587)
	SecurityTLS      = "tls"      // TLS from the start (port 465)
)

// settingName is where the settings live in package settings.
const settingName = "mail"

// passwordLabel binds the sealed password to its use (package secret).
const passwordLabel = "smtp.password"

// Config is the SMTP server and the sender.
type Config struct {
	Enabled  bool   `json:"enabled"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Security string `json:"security"`
	Username string `json:"username"`
	Password string `json:"-"`
	From     string `json:"from"`     // address
	FromName string `json:"fromName"` // display name
}

// stored is Config as kept in the database, the password sealed.
type stored struct {
	Config
	SealedPassword string `json:"password,omitempty"`
}

// DefaultPort is the usual port for a kind of security.
func DefaultPort(security string) int {
	switch security {
	case SecurityTLS:
		return 465
	case SecurityStartTLS:
		return 587
	}
	return 25
}

// WithDefaults fills in what an unset configuration means: STARTTLS and the
// usual port of the security mode.
func (c Config) WithDefaults() Config {
	if c.Security == "" {
		c.Security = SecurityStartTLS
	}
	if c.Port == 0 {
		c.Port = DefaultPort(c.Security)
	}
	return c
}

// Sender is the From address, with its display name.
func (c Config) Sender() mail.Address {
	name := strings.TrimSpace(c.FromName)
	if name == "" {
		name = "Solstice"
	}
	return mail.Address{Name: name, Address: c.From}
}

// Domain is the sender's domain, used in message ids.
func (c Config) Domain() string {
	if _, d, ok := strings.Cut(c.From, "@"); ok && d != "" {
		return strings.ToLower(d)
	}
	if c.Host != "" {
		return strings.ToLower(c.Host)
	}
	return "solstice.invalid"
}

// LoadConfig reads the settings, opening the password with box (which may
// be nil when no password was stored).
func LoadConfig(ctx context.Context, st *settings.Store, box func() (*secret.Box, error)) (Config, error) {
	var s stored
	ok, err := st.GetJSON(ctx, settingName, &s)
	if err != nil || !ok {
		return Config{}.WithDefaults(), err
	}
	if s.SealedPassword != "" {
		b, err := box()
		if err != nil {
			return s.Config, err
		}
		if s.Config.Password, err = b.OpenString(passwordLabel, s.SealedPassword); err != nil {
			return s.Config, err
		}
	}
	return s.Config, nil
}

// ReadStored reads the settings as stored, without defaults and without
// opening the password, and tells whether a password is stored.
func ReadStored(ctx context.Context, st *settings.Store) (c Config, passwordSet bool, err error) {
	var s stored
	_, err = st.GetJSON(ctx, settingName, &s)
	return s.Config, s.SealedPassword != "", err
}

// SaveConfig writes the settings, sealing the password.
func SaveConfig(ctx context.Context, st *settings.Store, box func() (*secret.Box, error), c Config) error {
	s := stored{Config: c}
	if c.Password != "" {
		b, err := box()
		if err != nil {
			return err
		}
		if s.SealedPassword, err = b.SealString(passwordLabel, c.Password); err != nil {
			return err
		}
	}
	return st.SetJSON(ctx, settingName, s)
}

// ValidAddress tells whether s is a plain e-mail address (no display name).
func ValidAddress(s string) bool {
	a, err := mail.ParseAddress(s)
	return err == nil && a.Address == s && a.Name == "" && !strings.ContainsAny(s, "\r\n")
}
