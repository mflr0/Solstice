package api

import (
	"context"
	"net/http"
	"time"
)

// TestHooks lets the integration tests (backend/tests) drive by hand what
// Run does on timers, and choose what the update check sees of the machine.
// Production code never uses it, and its methods must not run alongside Run.
type TestHooks struct{ s *Server }

// TestHooks returns the server's test controls.
func (s *Server) TestHooks() TestHooks { return TestHooks{s} }

// Settle returns once every event published so far has been handled.
func (h TestHooks) Settle() { h.s.out.bus.Wait() }

// FlushNotifications writes the notification batches due at now into the
// outbox.
func (h TestHooks) FlushNotifications(ctx context.Context, now time.Time) error {
	n := h.s.out.notify
	prev := n.Now
	n.Now = func() time.Time { return now }
	defer func() { n.Now = prev }()
	return n.FlushDue(ctx)
}

// SendMail sends the queued e-mails that are due.
func (h TestHooks) SendMail(ctx context.Context) error {
	_, err := h.s.out.outbox.SendDue(ctx)
	return err
}

// DeliverWebhooks sends the webhook deliveries that are due.
func (h TestHooks) DeliverWebhooks(ctx context.Context) error {
	_, err := h.s.out.hooks.DeliverDue(ctx)
	return err
}

// RunSchedules runs the filter subscriptions and due-date reminders whose
// time has come.
func (h TestHooks) RunSchedules(ctx context.Context) error { return h.s.out.notify.RunSchedules(ctx) }

// SkipDelays stops holding back wrong passwords, so throttling tests run
// at full speed.
func (h TestHooks) SkipDelays() { h.s.sleep = func(context.Context, time.Duration) {} }

// SetOAuthFetcher replaces the HTTP client that fetches OAuth client
// metadata documents, which refuses private addresses such as a test
// server's.
func (h TestHooks) SetOAuthFetcher(c *http.Client) { h.s.oauthFetch = c }

// ForgotFloor returns the least time the answer to "forgot your password"
// takes.
func (h TestHooks) ForgotFloor() time.Duration { return forgotFloor }

// UpdateEnv is what the update check sees of the outside world.
type UpdateEnv struct {
	GitHubAPI    string // GitHub's API base URL
	HelperDir    string // the update helper's directory
	DockerEnv    string // the file Docker puts in containers
	GOOS, GOARCH string
	Install      string // SOLSTICE_INSTALL
	Now          func() time.Time
}

// SetUpdateEnv replaces what the update check sees. Call it before the
// server takes requests.
func (h TestHooks) SetUpdateEnv(e UpdateEnv) {
	u := &h.s.upd
	u.apiBase, u.dir, u.dockerEnv = e.GitHubAPI, e.HelperDir, e.DockerEnv
	u.goos, u.goarch, u.installed, u.now = e.GOOS, e.GOARCH, e.Install, e.Now
}
