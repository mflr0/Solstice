package api

// Refused requests are logged: probing other projects, a missing
// X-Solstice header, a bad session cookie or a non-administrator calling
// an admin route leave a trace. Every 403 is, a 401 only when the request
// sent credentials: a visitor who is not signed in, or a page checking
// for a session, is no one to watch. They come in floods, so at most
// refusedCap lines are logged per client address (an IPv6 address
// counting as its /64) per refusedPeriod, then one line saying the rest
// are dropped until the period passes. All addresses together get
// refusedTotal lines per period, then one line likewise.

import (
	"log/slog"
	"net/http"
	"net/netip"
	"sync"
	"time"

	"solstice/internal/auth"
)

const (
	refusedCap    = 20  // lines logged per client address per period
	refusedTotal  = 500 // lines logged for all addresses per period
	refusedPeriod = time.Minute
)

// refuser caps how many refused requests get logged for one client
// address per period. now is a field so tests can move time; log is nil
// for the process default, which is the logger the server is built with
// (see cmd/solstice/main.go).
type refuser struct {
	log *slog.Logger

	mu sync.Mutex
	// Refused answers logged per client address this period. An entry is
	// only made when a line is logged, so there are at most refusedTotal.
	count map[string]int
	total int       // lines logged for all addresses this period
	until time.Time // end of the current period
	now   func() time.Time
}

// refused is the cap shared by the interface's API and the REST API, so
// that one address probing both fills a single budget. writeError is a
// plain function the setup wizard calls, so the cap cannot hang off a
// server; the trusted proxies are passed to logRefused instead.
var refused = &refuser{now: time.Now}

func (rf *refuser) logger() *slog.Logger {
	if rf.log != nil {
		return rf.log
	}
	return slog.Default()
}

// logRefused logs one refused request at Warn level — never its query
// string, headers, cookies or body — unless it is a 401 without
// credentials (see sentCredentials), or its address has already had
// refusedCap of them this period or all addresses refusedTotal, in which
// case one line says the rest are dropped until the period passes.
func (rf *refuser) logRefused(r *http.Request, status int, proxies []netip.Prefix) {
	if status == http.StatusUnauthorized && !sentCredentials(r) {
		return
	}
	client := clientIP(r, proxies)
	switch rf.allow(refusedKey(client)) {
	case verdictLog:
		user := ""
		if u := currentUser(r); u != nil {
			user = u.Username
		}
		rf.logger().Warn("request refused", "status", status, "method", r.Method,
			"path", r.URL.Path, "user", user, "client", client)
	case verdictAddressNotice:
		rf.logger().Warn("further refused requests from this address are not logged for a minute",
			"client", refusedKey(client))
	case verdictTotalNotice:
		rf.logger().Warn("further refused requests are not logged for a minute")
	}
}

// sentCredentials tells whether a request carries a session cookie or an
// Authorization header, whatever their value.
func sentCredentials(r *http.Request) bool {
	if r.Header.Get("Authorization") != "" {
		return true
	}
	_, err := r.Cookie(auth.CookieName)
	return err == nil
}

// refusedKey is what a client address is counted as: an IPv6 address by
// its /64, as one host holds a whole /64 and could change address at will.
func refusedKey(client string) string {
	if a, err := netip.ParseAddr(client); err == nil && a.Is6() {
		if p, err := a.Prefix(64); err == nil {
			return p.String()
		}
	}
	return client
}

// What allow decides for one refused request.
const (
	verdictLog           = iota // log the request
	verdictAddressNotice        // log the one line saying the address is capped
	verdictTotalNotice          // log the one line saying all addresses are capped
	verdictDrop                 // log nothing
)

// allow decides what to log for a refused request from this key.
func (rf *refuser) allow(key string) int {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	now := rf.now()
	if !now.Before(rf.until) { // the period has passed: start a new one
		rf.count = map[string]int{}
		rf.total = 0
		rf.until = now.Add(refusedPeriod)
	}
	if rf.total >= refusedTotal {
		if rf.total == refusedTotal {
			rf.total++
			return verdictTotalNotice
		}
		return verdictDrop
	}
	c := rf.count[key]
	switch {
	case c < refusedCap:
		rf.count[key] = c + 1
		rf.total++
		return verdictLog
	case c == refusedCap:
		rf.count[key] = c + 1
		rf.total++
		return verdictAddressNotice
	}
	return verdictDrop
}
