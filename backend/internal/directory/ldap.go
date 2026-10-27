// Package directory signs accounts in through an LDAP or Active Directory
// server (search the person with a service account, bind as them with the
// password typed, then read their groups) or an OpenID Connect provider.
package directory

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"

	"solstice/internal/auth"
	"solstice/internal/config"
)

// Name is what users.directory holds for LDAP accounts.
const Name = "ldap"

// LDAP is an LDAP server, set up from the configuration.
type LDAP struct {
	cfg config.LDAPSettings
	tls *tls.Config
}

// New checks the settings that need more than parsing (the CA file) and
// returns the directory; nil when LDAP is off.
func New(cfg config.LDAPSettings) (*LDAP, error) {
	if cfg.URL == "" {
		return nil, nil
	}
	u, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, err
	}
	tc := &tls.Config{ServerName: u.Hostname(), MinVersion: tls.VersionTLS12}
	if cfg.CAFile != "" {
		pem, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("LDAP CA file: %w", err)
		}
		pool, err := x509.SystemCertPool()
		if err != nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("LDAP CA file %s: no PEM certificate", cfg.CAFile)
		}
		tc.RootCAs = pool
	}
	return &LDAP{cfg: cfg, tls: tc}, nil
}

// Name implements auth.Directory.
func (d *LDAP) Name() string { return Name }

// Authenticate implements auth.Directory.
func (d *LDAP) Authenticate(ctx context.Context, username, password string) (*auth.DirectoryEntry, error) {
	// An empty password would be an unauthenticated bind, which servers
	// accept for any DN (RFC 4513, 5.1.2).
	if username == "" || password == "" {
		return nil, auth.ErrNoSession
	}
	ctx, cancel := context.WithTimeout(ctx, d.cfg.Timeout)
	defer cancel()
	conn, err := d.dial(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	// The connection does not watch ctx: closing it ends a call that waits.
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()

	if err := d.serviceBind(conn); err != nil {
		return nil, err
	}
	user, err := d.findUser(conn, username)
	if err != nil {
		return nil, err
	}
	if err := conn.Bind(user.DN, password); err != nil {
		if ldap.IsErrorWithCode(err, ldap.LDAPResultInvalidCredentials) {
			return nil, auth.ErrNoSession
		}
		return nil, fmt.Errorf("LDAP bind as the user: %w", err)
	}
	// Groups are read as the service account when there is one: a person
	// may not be allowed to read the groups.
	if err := d.serviceBind(conn); err != nil {
		return nil, err
	}
	groups, err := d.groups(conn, user.DN, username)
	if err != nil {
		return nil, err
	}
	e := &auth.DirectoryEntry{
		Name:   user.GetAttributeValue(d.cfg.NameAttr),
		Email:  user.GetAttributeValue(d.cfg.EmailAttr),
		Groups: groups,
	}
	return e, auth.ApplyGroups(e, d.cfg.RequireGroup, d.cfg.AdminGroup)
}

func (d *LDAP) dial(ctx context.Context) (*ldap.Conn, error) {
	dl := &net.Dialer{}
	if deadline, ok := ctx.Deadline(); ok {
		dl.Deadline = deadline
	}
	conn, err := ldap.DialURL(d.cfg.URL, ldap.DialWithDialer(dl), ldap.DialWithTLSConfig(d.tls))
	if err != nil {
		return nil, fmt.Errorf("LDAP connection: %w", err)
	}
	if deadline, ok := ctx.Deadline(); ok {
		conn.SetTimeout(time.Until(deadline))
	}
	if d.cfg.StartTLS {
		if err := conn.StartTLS(d.tls); err != nil {
			conn.Close()
			return nil, fmt.Errorf("LDAP StartTLS: %w", err)
		}
	}
	return conn, nil
}

// serviceBind binds as the service account; without one, the connection
// stays anonymous, or bound as the user.
func (d *LDAP) serviceBind(conn *ldap.Conn) error {
	if d.cfg.BindDN == "" {
		return nil
	}
	if err := conn.Bind(d.cfg.BindDN, d.cfg.BindPassword); err != nil {
		return fmt.Errorf("LDAP bind as the service account: %w", err)
	}
	return nil
}

// errAmbiguous means the user filter matches several entries: a setup
// mistake, which must not sign in whichever comes first.
var errAmbiguous = errors.New("LDAP: the user filter matches several entries")

func (d *LDAP) findUser(conn *ldap.Conn, username string) (*ldap.Entry, error) {
	res, err := conn.Search(ldap.NewSearchRequest(d.cfg.UserBase, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases,
		2, 0, false, UserFilter(d.cfg.UserFilter, username), []string{d.cfg.NameAttr, d.cfg.EmailAttr}, nil))
	switch {
	case ldap.IsErrorWithCode(err, ldap.LDAPResultSizeLimitExceeded):
		return nil, errAmbiguous
	case ldap.IsErrorWithCode(err, ldap.LDAPResultNoSuchObject):
		return nil, auth.ErrNoSession
	case err != nil:
		return nil, fmt.Errorf("LDAP user search: %w", err)
	case len(res.Entries) > 1:
		return nil, errAmbiguous
	case len(res.Entries) == 0:
		return nil, auth.ErrNoSession
	}
	return res.Entries[0], nil
}

func (d *LDAP) groups(conn *ldap.Conn, dn, username string) ([]string, error) {
	base := d.cfg.GroupBase
	if base == "" {
		base = d.cfg.UserBase
	}
	res, err := conn.Search(ldap.NewSearchRequest(base, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases,
		0, 0, false, GroupFilter(d.cfg.GroupFilter, dn, username), []string{d.cfg.GroupNameAttr}, nil))
	if ldap.IsErrorWithCode(err, ldap.LDAPResultNoSuchObject) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("LDAP group search: %w", err)
	}
	var out []string
	for _, e := range res.Entries {
		if n := e.GetAttributeValue(d.cfg.GroupNameAttr); n != "" {
			out = append(out, n)
		}
	}
	return out, nil
}

// UserFilter fills {username} in a filter, escaped (RFC 4515): a typed name
// cannot change what the filter matches.
func UserFilter(filter, username string) string {
	return strings.ReplaceAll(filter, "{username}", ldap.EscapeFilter(username))
}

// GroupFilter fills {dn} and {username} in a filter, escaped.
func GroupFilter(filter, dn, username string) string {
	return strings.NewReplacer("{dn}", ldap.EscapeFilter(dn), "{username}", ldap.EscapeFilter(username)).Replace(filter)
}
