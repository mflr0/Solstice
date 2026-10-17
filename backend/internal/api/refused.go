package api

// Refused requests (401 and 403 answers) are logged: probing other
// projects, a missing X-Solstice header, a bad session cookie or a
// non-administrator calling an admin route leave a trace. They come in
// floods, so at most refusedCap lines are logged per client address per
// refusedPeriod, then one line saying the rest are dropped until the
// period passes.

import (
	"log/slog"
	"net/http"
	"net/netip"
	"sync"
	"time"
)

const (
	refusedCap    = 20 // lines logged per client address per period
	refusedPeriod = time.Minute
)

// refuser caps how many refused requests get logged for one client
// address per period. now is a field so tests can move time; log is nil
// for the process default, which is the logger the server is built with
// (see cmd/solstice/main.go).
type refuser struct {
	log *slog.Logger

	mu    sync.Mutex
	count map[string]int // refused answers logged per client address this period
	until time.Time      // end of the current period
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
// string, headers, cookies or body — unless its address has already had
// refusedCap of them this period, in which case one line says the rest
// are dropped until the period passes.
func (rf *refuser) logRefused(r *http.Request, status int, proxies []netip.Prefix) {
	client := clientIP(r, proxies)
	ok, notice := rf.allow(client)
	switch {
	case ok:
		user := ""
		if u := currentUser(r); u != nil {
			user = u.Username
		}
		rf.logger().Warn("request refused", "status", status, "method", r.Method,
			"path", r.URL.Path, "user", user, "client", client)
	case notice:
		rf.logger().Warn("further refused requests from this address are not logged for a minute",
			"client", client)
	}
}

// allow reports whether a refused request from this address may be
// logged now, and whether this is the one line that says the rest are
// dropped for the period.
func (rf *refuser) allow(addr string) (ok, notice bool) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	now := rf.now()
	if !now.Before(rf.until) { // the period has passed: start a new one
		rf.count = map[string]int{}
		rf.until = now.Add(refusedPeriod)
	}
	c := rf.count[addr]
	if c < refusedCap {
		rf.count[addr] = c + 1
		return true, false
	}
	if c == refusedCap {
		rf.count[addr] = c + 1
		return false, true
	}
	return false, false
}
