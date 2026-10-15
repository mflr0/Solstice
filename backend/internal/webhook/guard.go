package webhook

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// Reasons a webhook address is refused.
var (
	ErrScheme  = errors.New("only http and https addresses are allowed")
	ErrHost    = errors.New("the address has no host")
	ErrBlocked = errors.New("link-local and cloud metadata addresses are refused")
)

// blockedNets are never reached: link-local ranges (where cloud metadata
// services answer: 169.254.169.254, fd00:ec2::254…), "this network",
// multicast, broadcast and reserved ranges, and the metadata addresses some
// clouds put elsewhere.
var blockedNets = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("224.0.0.0/4"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("100.100.100.200/32"), // Alibaba Cloud metadata
	netip.MustParsePrefix("::/128"),
	netip.MustParsePrefix("fe80::/10"),
	netip.MustParsePrefix("ff00::/8"),
	netip.MustParsePrefix("fd00:ec2::254/128"), // AWS metadata over IPv6
}

// nat64 embeds IPv4 addresses in IPv6 (64:ff9b::a9fe:a9fe is
// 169.254.169.254 through a NAT64 gateway).
var nat64 = netip.MustParsePrefix("64:ff9b::/96")

// Blocked tells whether an address must never be contacted. Private and
// loopback addresses are allowed: webhooks often target internal services.
func Blocked(ip netip.Addr) bool {
	// A zoned address (fe80::1%eth0) is in no prefix: drop the zone first.
	ip = ip.Unmap().WithZone("")
	if !ip.IsValid() {
		return true
	}
	if ip.Is6() && (nat64.Contains(ip) || isIPv4Compatible(ip)) {
		b := ip.As16()
		return Blocked(netip.AddrFrom4([4]byte{b[12], b[13], b[14], b[15]}))
	}
	for _, p := range blockedNets {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

// isIPv4Compatible is the deprecated ::a.b.c.d form (but :: and ::1).
func isIPv4Compatible(ip netip.Addr) bool {
	b := ip.As16()
	for _, x := range b[:12] {
		if x != 0 {
			return false
		}
	}
	return ip != netip.IPv6Unspecified() && ip != netip.IPv6Loopback()
}

// CheckURL validates a webhook address when it is saved: http or https, a
// host, and not a refused address when the host is one or resolves to one
// now. The check that counts happens again at each connection.
func CheckURL(ctx context.Context, raw string) error {
	u, err := url.Parse(urlVariables.ReplaceAllString(strings.TrimSpace(raw), "x"))
	if err != nil {
		return ErrHost
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ErrScheme
	}
	host := u.Hostname()
	if host == "" {
		return ErrHost
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		if Blocked(ip) {
			return ErrBlocked
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil // perhaps not resolvable from here yet; checked again at each delivery
	}
	for _, ip := range ips {
		if Blocked(ip) {
			return ErrBlocked
		}
	}
	return nil
}

// blockedError is a connection refused by the guard.
type blockedError struct{ addr string }

func (e *blockedError) Error() string {
	return fmt.Sprintf("refused: %s is a link-local or cloud metadata address", e.addr)
}

// guardedClient is the HTTP client of deliveries: 10 s per request, no
// redirects, no proxy, and every connection checked against Blocked after
// name resolution, so DNS tricks cannot reach a refused address.
func guardedClient() *http.Client {
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
		Control: func(network, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip, err := netip.ParseAddr(host)
			if err != nil || Blocked(ip) {
				return &blockedError{host}
			}
			return nil
		},
	}
	return &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			Proxy:                 nil,
			DialContext:           dialer.DialContext,
			ForceAttemptHTTP2:     true,
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			MaxIdleConns:          20,
			IdleConnTimeout:       time.Minute,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}
