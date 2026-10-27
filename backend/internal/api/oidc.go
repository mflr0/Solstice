package api

import (
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"solstice/internal/auth"
)

// oidcCookie carries a sign-in through the provider, from start to callback.
const (
	oidcCookie     = "solstice_oidc"
	oidcCookiePath = "/api/auth/oidc"
	oidcFlowTime   = 10 * time.Minute
)

// The codes /login?error= shows after a failed OpenID Connect sign-in.
const (
	ssoFailed  = "sso-failed"  // the provider or the exchange failed
	ssoDenied  = "sso-denied"  // not allowed in: required group, deactivated account
	ssoAccount = "sso-account" // the user name belongs to an account that cannot be linked
)

// oidcFlow is what the browser keeps between start and callback.
type oidcFlow struct {
	State    string `json:"s"`
	Nonce    string `json:"n"`
	Verifier string `json:"v"`
	Next     string `json:"next"`
}

// SetSingleSignOn signs accounts in through an OpenID Connect provider. Call
// it before the server takes requests.
func (s *Server) SetSingleSignOn(sso auth.SingleSignOn) { s.sso = sso }

// ssoReady tells whether the sign-in button can work: the provider is set
// up and the public address, which the provider sends people back to, too.
func (s *Server) ssoReady() bool { return s.sso != nil && s.base() != "" }

func (s *Server) oidcRedirectURI() string {
	return strings.TrimRight(s.base(), "/") + oidcCookiePath + "/callback"
}

// oidcStart sends the browser to the provider; ?next= is where it lands
// once signed in.
func (s *Server) oidcStart(w http.ResponseWriter, r *http.Request) error {
	if !s.ssoReady() {
		return errNotFound
	}
	flow := oidcFlow{Next: localPath(r.URL.Query().Get("next"))}
	for _, v := range []*string{&flow.State, &flow.Nonce, &flow.Verifier} {
		var err error
		if *v, err = auth.RandomToken(); err != nil {
			return err
		}
	}
	to, err := s.sso.AuthURL(r.Context(), s.oidcRedirectURI(), flow.State, flow.Nonce, flow.Verifier)
	if err != nil {
		s.log.Warn("OpenID Connect sign-in failed", "err", err)
		http.Redirect(w, r, "/login?error="+ssoFailed, http.StatusSeeOther)
		return nil
	}
	raw, err := json.Marshal(flow)
	if err != nil {
		return err
	}
	s.setOIDCCookie(w, r, base64.RawURLEncoding.EncodeToString(raw), int(oidcFlowTime.Seconds()))
	http.Redirect(w, r, to, http.StatusSeeOther)
	return nil
}

// oidcCallback is where the provider sends the browser back: the code is
// exchanged for the person's identity, which signs in its account.
func (s *Server) oidcCallback(w http.ResponseWriter, r *http.Request) error {
	if s.sso == nil {
		return errNotFound
	}
	refuse := func(code, username string, err error) error {
		if err != nil {
			s.log.Warn("OpenID Connect sign-in failed", "err", err)
		}
		s.auditLogin(r, username, nil, "oidc")
		http.Redirect(w, r, "/login?error="+code, http.StatusSeeOther)
		return nil
	}
	flow, ok := readOIDCFlow(r)
	s.setOIDCCookie(w, r, "", -1) // a flow is used once
	q := r.URL.Query()
	if !ok || subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(flow.State)) != 1 {
		return refuse(ssoFailed, "", errors.New("missing or wrong state: the sign-in was not started here, or took too long"))
	}
	if e := q.Get("error"); e != "" {
		return refuse(ssoFailed, "", errors.New("the provider answered "+e))
	}
	id, err := s.sso.Exchange(r.Context(), s.oidcRedirectURI(), q.Get("code"), flow.Verifier, flow.Nonce)
	switch {
	case errors.Is(err, auth.ErrNoSession) && id != nil:
		return refuse(ssoDenied, id.Username, nil)
	case err != nil:
		return refuse(ssoFailed, "", err)
	}
	if !usernamePattern.MatchString(id.Username) {
		// Only an account already linked can sign in: no account takes this name.
		id.Username = ""
	}
	u, err := auth.IdentitySignIn(r.Context(), s.db, id)
	switch {
	case errors.Is(err, auth.ErrAccountTaken):
		return refuse(ssoAccount, id.Username, nil)
	case errors.Is(err, auth.ErrNoSession):
		return refuse(ssoDenied, id.Username, nil)
	case err != nil:
		return err
	}
	if err := s.startSession(w, r, u); err != nil {
		return err
	}
	s.auditLogin(r, u.Username, u, "oidc")
	http.Redirect(w, r, flow.Next, http.StatusSeeOther)
	return nil
}

func (s *Server) setOIDCCookie(w http.ResponseWriter, r *http.Request, value string, maxAge int) {
	// Lax: the cookie must come back with the provider's redirect, a
	// top-level navigation from another site.
	http.SetCookie(w, &http.Cookie{Name: oidcCookie, Value: value, Path: oidcCookiePath, MaxAge: maxAge,
		HttpOnly: true, Secure: secureRequest(r, s.base(), s.proxies, s.log), SameSite: http.SameSiteLaxMode})
}

func readOIDCFlow(r *http.Request) (oidcFlow, bool) {
	var flow oidcFlow
	c, err := r.Cookie(oidcCookie)
	if err != nil {
		return flow, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil || json.Unmarshal(raw, &flow) != nil || flow.State == "" {
		return flow, false
	}
	flow.Next = localPath(flow.Next)
	return flow, true
}

// localPath returns p when it is a path of this site, else "/": a link
// cannot send people elsewhere once signed in. The interface's safeNext
// does the same.
func localPath(p string) string {
	if !strings.HasPrefix(p, "/") || strings.HasPrefix(p, "//") || strings.HasPrefix(p, `/\`) {
		return "/"
	}
	return p
}
