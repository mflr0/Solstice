package directory

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"solstice/internal/auth"
	"solstice/internal/config"
)

// OIDC is an OpenID Connect provider, set up from the configuration.
type OIDC struct {
	cfg    config.OIDCSettings
	client *http.Client

	mu       sync.Mutex
	provider *oidc.Provider // discovered at the first sign-in; see discover
}

// NewOIDC returns the provider; nil when OpenID Connect is off. Nothing is
// asked of the provider yet: Solstice starts while it is down.
func NewOIDC(cfg config.OIDCSettings) *OIDC {
	if cfg.Issuer == "" {
		return nil
	}
	return &OIDC{cfg: cfg, client: &http.Client{Timeout: cfg.Timeout}}
}

// Name implements auth.SingleSignOn.
func (o *OIDC) Name() string { return strings.TrimSpace(o.cfg.Name) }

// discover reads the provider's discovery document, once it answers.
func (o *OIDC) discover(ctx context.Context) (*oidc.Provider, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.provider != nil {
		return o.provider, nil
	}
	ctx, cancel := context.WithTimeout(ctx, o.cfg.Timeout)
	defer cancel()
	p, err := oidc.NewProvider(oidc.ClientContext(ctx, o.client), o.cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("OpenID Connect discovery: %w", err)
	}
	o.provider = p
	return p, nil
}

func (o *OIDC) oauth(p *oidc.Provider, redirectURI string) *oauth2.Config {
	return &oauth2.Config{ClientID: o.cfg.ClientID, ClientSecret: o.cfg.ClientSecret, Endpoint: p.Endpoint(),
		RedirectURL: redirectURI, Scopes: strings.Fields(o.cfg.Scopes)}
}

// AuthURL implements auth.SingleSignOn.
func (o *OIDC) AuthURL(ctx context.Context, redirectURI, state, nonce, verifier string) (string, error) {
	p, err := o.discover(ctx)
	if err != nil {
		return "", err
	}
	return o.oauth(p, redirectURI).AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier)), nil
}

// Exchange implements auth.SingleSignOn.
func (o *OIDC) Exchange(ctx context.Context, redirectURI, code, verifier, nonce string) (*auth.Identity, error) {
	p, err := o.discover(ctx)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(oidc.ClientContext(ctx, o.client), o.cfg.Timeout)
	defer cancel()
	tok, err := o.oauth(p, redirectURI).Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, fmt.Errorf("OpenID Connect code exchange: %w", err)
	}
	raw, ok := tok.Extra("id_token").(string)
	if !ok {
		return nil, errors.New("OpenID Connect: the token response has no ID token")
	}
	// The verifier checks the signature, the issuer, the audience (our
	// client) and the expiry; the nonce is ours to check.
	idt, err := p.Verifier(&oidc.Config{ClientID: o.cfg.ClientID}).Verify(ctx, raw)
	if err != nil {
		return nil, fmt.Errorf("OpenID Connect ID token: %w", err)
	}
	if idt.Nonce != nonce {
		return nil, errors.New("OpenID Connect ID token: wrong nonce")
	}
	claims := map[string]any{}
	if err := idt.Claims(&claims); err != nil {
		return nil, fmt.Errorf("OpenID Connect ID token: %w", err)
	}
	if p.UserInfoEndpoint() != "" {
		// Some providers keep profile claims out of the ID token: the user
		// info fills the ones it lacks, for the same person only.
		if info, err := p.UserInfo(ctx, oauth2.StaticTokenSource(tok)); err == nil && info.Subject == idt.Subject {
			more := map[string]any{}
			if info.Claims(&more) == nil {
				for k, v := range more {
					if _, ok := claims[k]; !ok {
						claims[k] = v
					}
				}
			}
		}
	}
	id := identity(claims, o.cfg)
	id.Issuer, id.Subject = idt.Issuer, idt.Subject
	return id, auth.ApplyGroups(&id.DirectoryEntry, o.cfg.RequireGroup, o.cfg.AdminGroup)
}

// identity reads the account's fields from the claims. The address is left
// out when the provider says it is not verified: reset links go there.
func identity(claims map[string]any, cfg config.OIDCSettings) *auth.Identity {
	id := &auth.Identity{}
	id.Username = auth.CanonicalUsername(claimString(claims[cfg.UsernameClaim]))
	id.Name = claimString(claims["name"])
	if verified, ok := claims["email_verified"]; !ok || claimBool(verified) {
		id.Email = claimString(claims["email"])
	}
	switch g := claims[cfg.GroupsClaim].(type) {
	case []any:
		for _, v := range g {
			if s := claimString(v); s != "" {
				id.Groups = append(id.Groups, s)
			}
		}
	case string: // a provider may send a single group bare
		if g = strings.TrimSpace(g); g != "" {
			id.Groups = []string{g}
		}
	}
	return id
}

func claimString(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

// claimBool reads a boolean claim; some providers send "true" as a string.
func claimBool(v any) bool {
	switch b := v.(type) {
	case bool:
		return b
	case string:
		return strings.EqualFold(b, "true")
	}
	return false
}
