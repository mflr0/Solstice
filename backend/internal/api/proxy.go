package api

// The client address of a request, for throttling and the audit log. Behind
// a reverse proxy every connection comes from the proxy, so the client is the
// address the proxy forwards in X-Forwarded-For. That header is only believed
// from a proxy listed in SOLSTICE_TRUSTED_PROXIES, as any client could have
// set it.

import (
	"net/http"
	"net/netip"
	"strings"
)

// SetTrustedProxies sets the addresses and CIDR ranges of the reverse proxies
// whose X-Forwarded-For header the server believes (SOLSTICE_TRUSTED_PROXIES).
func (s *Server) SetTrustedProxies(proxies []netip.Prefix) {
	s.proxies = proxies
}

// clientIP is the address of the client of r, never with a port: the address
// of the connection, unless that address is a trusted proxy, in which case
// the client is read from X-Forwarded-For, from right to left past the
// trusted addresses. A missing, empty or malformed header leaves the address
// of the connection.
func clientIP(r *http.Request, trusted []netip.Prefix) string {
	addr, ok := peerAddr(r.RemoteAddr)
	if !ok {
		return r.RemoteAddr
	}
	if !isTrusted(trusted, addr) {
		return addr.String()
	}
	chain := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
	for i := len(chain) - 1; i >= 0; i-- {
		next, ok := peerAddr(strings.TrimSpace(chain[i]))
		if !ok {
			return addr.String() // malformed: only the connection is known
		}
		if !isTrusted(trusted, next) {
			return next.String()
		}
	}
	return addr.String() // nothing but trusted proxies in the chain
}

// peerAddr reads an address as a connection or a proxy writes it (host:port,
// [v6]:port, [v6] or a bare address) without its port or IPv6 zone, an
// IPv4-mapped IPv6 address unmapped. ok is false when it is no address at
// all. The zone is dropped so that fe80::1%eth0 cannot dodge a trusted range
// or count as another client than fe80::1.
func peerAddr(s string) (netip.Addr, bool) {
	if ap, err := netip.ParseAddrPort(s); err == nil {
		return ap.Addr().WithZone("").Unmap(), true
	}
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		s = s[1 : len(s)-1]
	}
	a, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Addr{}, false
	}
	return a.WithZone("").Unmap(), true
}

// isTrusted tells whether addr belongs to one of the proxies.
func isTrusted(proxies []netip.Prefix, addr netip.Addr) bool {
	for _, p := range proxies {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}
