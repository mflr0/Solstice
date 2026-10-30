package api

// OAuth clients are identified by a Client ID Metadata Document: their
// client_id is an HTTPS URL serving a JSON description of the client
// (draft-ietf-oauth-client-id-metadata-document). Solstice fetches it when
// a user is asked to authorize the client.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"solstice/internal/webhook"
)

// Limits of a client metadata document and of its fetch.
const (
	maxClientID       = 1024
	maxClientDocument = 64 << 10
	maxClientName     = 100
	clientFetchTime   = 5 * time.Second
)

// oauthClientDoc is what Solstice reads of a client metadata document.
type oauthClientDoc struct {
	ClientID     string   `json:"client_id"`
	ClientName   string   `json:"client_name"`
	ClientURI    string   `json:"client_uri"`
	RedirectURIs []string `json:"redirect_uris"`
	AuthMethod   string   `json:"token_endpoint_auth_method"`
	Registered   bool     `json:"-"` // registered at /register, not a metadata document
}

// validClientID tells whether a client_id is a URL a metadata document may
// be fetched from: https, a host, a path other than "/", no user, fragment
// or dot segment.
func validClientID(id string) bool {
	if len(id) > maxClientID {
		return false
	}
	u, err := url.Parse(id)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" || u.RawFragment != "" ||
		strings.Contains(id, "#") || u.Path == "" || u.Path == "/" {
		return false
	}
	return !slices.ContainsFunc(strings.Split(u.Path, "/"), func(seg string) bool { return seg == "." || seg == ".." })
}

// fetchClient reads and checks a client's metadata document.
func (s *Server) fetchClient(ctx context.Context, id string) (*oauthClientDoc, error) {
	ctx, cancel := context.WithTimeout(ctx, clientFetchTime)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, id, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := s.oauthFetch.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxClientDocument+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxClientDocument {
		return nil, errors.New("document too large")
	}
	var doc oauthClientDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	doc.ClientName = strings.TrimSpace(doc.ClientName)
	switch {
	case doc.ClientID != id:
		return nil, fmt.Errorf("client_id %q differs from the document's URL", doc.ClientID)
	case doc.ClientName == "":
		return nil, errors.New("no client_name")
	case len(doc.RedirectURIs) == 0:
		return nil, errors.New("no redirect_uris")
	case doc.AuthMethod != "" && doc.AuthMethod != "none":
		return nil, fmt.Errorf("token_endpoint_auth_method %q: only public clients are supported", doc.AuthMethod)
	}
	if utf8.RuneCountInString(doc.ClientName) > maxClientName {
		doc.ClientName = string([]rune(doc.ClientName)[:maxClientName])
	}
	return &doc, nil
}

// allowsRedirect tells whether the document lists a redirect URI. A loopback
// one matches on any port, as native apps pick theirs at run time (RFC 8252
// §7.3).
func (d *oauthClientDoc) allowsRedirect(uri string) bool {
	if slices.Contains(d.RedirectURIs, uri) {
		return true
	}
	got, err := url.Parse(uri)
	if err != nil || got.Scheme != "http" || !loopbackHost(got.Hostname()) {
		return false
	}
	for _, r := range d.RedirectURIs {
		want, err := url.Parse(r)
		if err == nil && want.Scheme == "http" && want.Hostname() == got.Hostname() &&
			want.EscapedPath() == got.EscapedPath() && want.RawQuery == got.RawQuery {
			return true
		}
	}
	return false
}

func loopbackHost(h string) bool {
	if h == "localhost" {
		return true
	}
	ip, err := netip.ParseAddr(h)
	return err == nil && ip.IsLoopback()
}

// cgnat is the shared address space of carrier-grade NAT (RFC 6598).
var cgnat = netip.MustParsePrefix("100.64.0.0/10")

// publicAddr tells whether a client document may be fetched from an address:
// none of the webhooks' refused ones, and nothing private or loopback.
func publicAddr(ip netip.Addr) bool {
	ip = ip.Unmap().WithZone("")
	return !webhook.Blocked(ip) && !ip.IsLoopback() && !ip.IsPrivate() && !ip.IsUnspecified() && !cgnat.Contains(ip)
}

// clientFetcher is the HTTP client of metadata documents: no redirects, no
// proxy, and every connection checked after name resolution, so a client_id
// cannot make Solstice reach an internal address.
func clientFetcher() *http.Client {
	dialer := &net.Dialer{
		Timeout: clientFetchTime,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			if ip, err := netip.ParseAddr(host); err != nil || !publicAddr(ip) {
				return fmt.Errorf("refused: %s is not a public address", host)
			}
			return nil
		},
	}
	return &http.Client{
		Timeout: clientFetchTime,
		Transport: &http.Transport{
			Proxy:                 nil,
			DialContext:           dialer.DialContext,
			ForceAttemptHTTP2:     true,
			TLSHandshakeTimeout:   clientFetchTime,
			ResponseHeaderTimeout: clientFetchTime,
			MaxIdleConns:          10,
			IdleConnTimeout:       time.Minute,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}
