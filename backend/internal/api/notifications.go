package api

// What Solstice sends out: events flow from the handlers that change things
// (s.emit) to an event bus, which refreshes open pages at once and feeds
// e-mail notifications and webhooks on its own goroutine. Background workers
// (Run) write the e-mails and deliver them.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"solstice/internal/audit"
	"solstice/internal/events"
	"solstice/internal/i18n"
	"solstice/internal/live"
	"solstice/internal/mail"
	"solstice/internal/notify"
	"solstice/internal/perm"
	"solstice/internal/secret"
	"solstice/internal/settings"
	"solstice/internal/sqlq"
	"solstice/internal/webhook"
)

// outgoing holds the event bus and what it feeds.
type outgoing struct {
	bus      *events.Bus
	settings *settings.Store
	outbox   *mail.Outbox
	notify   *notify.Service
	hooks    *webhook.Service

	secretKey string // SOLSTICE_SECRETKEY; empty: <data>/secret.key
	boxMu     sync.Mutex
	box       *secret.Box
}

// startOutgoing sets up the event bus, e-mail and webhooks. Nothing is sent
// until Run is called.
func (s *Server) startOutgoing() {
	o := &outgoing{settings: settings.New(s.db)}
	s.out = o
	o.bus = events.NewBus(s.db, s.log)
	o.bus.OnPublish(s.toLive)
	o.bus.LoadCustom(s.eventCustom)
	config := func(ctx context.Context) (mail.Config, error) { return mail.LoadConfig(ctx, o.settings, s.secrets) }
	o.outbox = mail.NewOutbox(s.db, s.log, config)
	o.notify = notify.New(s.db, s.perms, o.outbox, s.tz, s.log, config, filterRunner{s})
	o.notify.BaseURL = s.base
	o.notify.Zone = s.zone
	o.notify.DefaultLocale = s.defaultLanguage
	o.hooks = webhook.New(s.db, s.log, s.secrets, s.matchJQL)
	o.hooks.BaseURL = s.base
	o.hooks.DefaultLocale = s.defaultLanguage
	o.outbox.Paused, o.notify.Paused, o.hooks.Paused = s.importRunning, s.importRunning, s.importRunning
	o.bus.Subscribe(o.notify)
	o.bus.Subscribe(o.hooks)
}

// SetSecretKey sets the key that seals stored secrets (SOLSTICE_SECRETKEY,
// 32 bytes in hex or base64). Without it, <data>/secret.key is used, and
// created if needed.
func (s *Server) SetSecretKey(key string) {
	s.out.boxMu.Lock()
	s.out.secretKey, s.out.box = key, nil
	s.out.boxMu.Unlock()
}

// secrets opens the key on first use.
func (s *Server) secrets() (*secret.Box, error) {
	o := s.out
	o.boxMu.Lock()
	defer o.boxMu.Unlock()
	if o.box == nil {
		box, err := secret.Load(s.dataDir, o.secretKey)
		if err != nil {
			return nil, err
		}
		o.box = box
	}
	return o.box, nil
}

// Run writes and sends e-mails and webhook deliveries until ctx ends.
func (s *Server) Run(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() { s.out.outbox.Run(ctx) })
	wg.Go(func() { s.out.notify.Run(ctx) })
	wg.Go(func() { s.out.hooks.Run(ctx) })
	wg.Wait()
}

// Close hands the last events over to e-mail and webhooks (they are stored
// and sent at the next start if need be). Call it once the HTTP server has
// stopped taking requests.
func (s *Server) Close(ctx context.Context) error { return s.out.bus.Close(ctx) }

// Events --------------------------------------------------------------------

// emit publishes an event. It never waits for e-mail or webhooks.
func (s *Server) emit(e events.Event) { s.out.bus.Publish(e) }

// eventCustom renders an issue's custom fields for e-mails and webhooks, as
// GET /issue does. Like the rest of a webhook's issue, the epic is not
// filtered by permission: e-mails filter it per recipient.
func (s *Server) eventCustom(ctx context.Context, issueID int64) ([]events.CustomValue, error) {
	iss, err := s.findIssueByID(ctx, issueID)
	if err != nil {
		return nil, err
	}
	set, err := s.loadIssues(ctx, nil, []*issueRow{iss}, func(k string) bool {
		return k == loadCustom || k == loadSprints
	})
	if err != nil {
		return nil, err
	}
	d := set.list[0]
	if iss.EpicID.Valid {
		if d.epic, err = s.findIssueByID(ctx, iss.EpicID.Int64); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
	}
	base := s.base()
	o := &restOut{base: base, api: base + "/rest/api/2", locale: s.defaultLanguage(), tz: s.zone()}
	users, err := s.jiraUsers(ctx, o, set.userIDs()...)
	if err != nil {
		return nil, err
	}
	out := make([]events.CustomValue, 0, len(set.fields))
	for _, f := range set.fields {
		v, err := json.Marshal(o.customValue(set, users, f, d))
		if err != nil {
			return nil, err
		}
		out = append(out, events.CustomValue{ID: f.ID, Name: f.Name, Value: v, Text: o.customText(set, users, f, d)})
	}
	return out, nil
}

// toLive passes events on to open pages.
func (s *Server) toLive(e events.Event) {
	if e.Live == "" {
		return
	}
	ev := live.Event{Key: e.Key, What: e.Live, By: e.ActorID, Issue: e.Access}
	if e.Comment != nil {
		ev.Group, ev.Role = e.Comment.Group, e.Comment.RoleID
	}
	s.hub.Publish(ev)
	// Someone who just lost access must learn the issue changed too.
	if e.Before != nil && *e.Before != e.Access {
		ev.Issue = *e.Before
		s.hub.Publish(ev)
	}
}

// issueEvent starts an event about an issue.
func issueEvent(kind events.Kind, iss *issueRow, by int64, liveWhat string) events.Event {
	return events.Event{Kind: kind, ActorID: by, IssueID: iss.ID, Key: iss.Key(), Access: iss.perm(), Live: liveWhat}
}

// commentEvent is an event about a comment; open pages refresh for those who
// may read it.
func commentEvent(kind events.Kind, iss *issueRow, by int64, c *events.Comment) events.Event {
	e := issueEvent(kind, iss, by, "comment")
	e.Comment = c
	return e
}

// changed emits an update of an issue with its history lines.
func (s *Server) changed(iss *issueRow, by int64, liveWhat string, changes []change) {
	e := issueEvent(events.IssueUpdated, iss, by, liveWhat)
	e.Changes = eventChanges(changes)
	s.emit(e)
}

func optText(v any) *string {
	switch x := v.(type) {
	case nil:
		return nil
	case string:
		return &x
	case int64:
		return events.Int(x)
	}
	t := fmt.Sprint(v)
	return &t
}

func eventChanges(changes []change) []events.Change {
	out := make([]events.Change, 0, len(changes))
	for _, c := range changes {
		out = append(out, events.Change{Field: c.field, From: optText(c.oldValue), FromString: optText(c.oldString),
			To: optText(c.newValue), ToString: optText(c.newString)})
	}
	return out
}

func sprintEvent(sp *sprintOut, board int64) *events.Sprint {
	return &events.Sprint{ID: sp.ID, BoardID: board, Name: sp.Name, State: sp.State, Goal: sp.Goal, Start: sp.Start,
		End: sp.End, Completed: sp.Completed}
}

// emitSprint announces a sprint as it is now.
func (s *Server) emitSprint(ctx context.Context, kind events.Kind, by, board, sprint int64) {
	sp, err := s.sprintByID(ctx, board, sprint)
	if err != nil {
		s.log.Warn("sprint event lost", "sprint", sprint, "err", err)
		return
	}
	s.emit(events.Event{Kind: kind, ActorID: by, Sprint: sprintEvent(sp, board)})
}

// Preferences ---------------------------------------------------------------

type prefsOut struct {
	notify.Prefs
	TimeZone    string `json:"timeZone"`    // the hours' time zone
	MailEnabled bool   `json:"mailEnabled"` // whether this Solstice sends e-mail at all
}

func (s *Server) prefsOut(ctx context.Context, userID int64) (*prefsOut, error) {
	p, err := s.out.notify.Prefs(ctx, userID)
	if err != nil {
		return nil, err
	}
	cfg, err := mail.LoadConfig(ctx, s.out.settings, s.secrets)
	tz := s.zone()
	zone := tz.String()
	if zone == "Local" { // no time zone set: the system's, which has no name here
		zone = time.Now().In(tz).Format("MST (UTC-07:00)")
	}
	return &prefsOut{Prefs: p, TimeZone: zone, MailEnabled: err == nil && cfg.Enabled}, nil
}

func (s *Server) getMyNotifications(w http.ResponseWriter, r *http.Request) error {
	out, err := s.prefsOut(r.Context(), currentUser(r).ID)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

func checkHour(h int) error {
	if h < 0 || h > 23 {
		return fail(http.StatusBadRequest, i18n.NotifyHourInvalid)
	}
	return nil
}

// putMyNotifications changes the user's preferences; fields left out keep
// their value.
func (s *Server) putMyNotifications(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Delivery     *string         `json:"delivery"`
		DigestHour   *int            `json:"digestHour"`
		OwnChanges   *bool           `json:"ownChanges"`
		Events       map[string]bool `json:"events"`
		DueReminder  *bool           `json:"dueReminder"`
		ReminderHour *int            `json:"reminderHour"`
		// Read-only fields sent back as received are accepted.
		TimeZone    *string `json:"timeZone"`
		MailEnabled *bool   `json:"mailEnabled"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	ctx := r.Context()
	me := currentUser(r).ID
	p, err := s.out.notify.Prefs(ctx, me)
	if err != nil {
		return err
	}
	if in.Delivery != nil {
		if !slices.Contains([]string{notify.Immediate, notify.Digest, notify.None}, *in.Delivery) {
			return fail(http.StatusBadRequest, i18n.NotifyDeliveryUnknown)
		}
		p.Delivery = *in.Delivery
	}
	for _, h := range []struct {
		in  *int
		out *int
	}{{in.DigestHour, &p.DigestHour}, {in.ReminderHour, &p.ReminderHour}} {
		if h.in != nil {
			if err := checkHour(*h.in); err != nil {
				return err
			}
			*h.out = *h.in
		}
	}
	if in.OwnChanges != nil {
		p.OwnChanges = *in.OwnChanges
	}
	if in.DueReminder != nil {
		p.DueReminder = *in.DueReminder
	}
	for k, v := range in.Events {
		if !slices.Contains(notify.Groups, k) {
			return failf(http.StatusBadRequest, i18n.NotifyEventUnknown, k)
		}
		p.Events[k] = v
	}
	if err := s.out.notify.SavePrefs(ctx, me, p); err != nil {
		return err
	}
	return s.getMyNotifications(w, r)
}

// Project overrides ---------------------------------------------------------

// projectEvents are the event groups a project may send to extra people.
var projectEvents = []string{notify.GroupCreated, notify.GroupAssigned, notify.GroupCommented, notify.GroupStatus,
	notify.GroupUpdated, notify.GroupAttachments, notify.GroupDeleted}

func (s *Server) getProjectNotifications(w http.ResponseWriter, r *http.Request) error {
	id, err := s.configurableProject(r)
	if err != nil {
		return err
	}
	ps, err := s.out.notify.ProjectSettings(r.Context(), id)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, map[string]any{"enabled": ps.Enabled, "extra": ps.Extra, "events": projectEvents})
}

func (s *Server) putProjectNotifications(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	id, err := s.configurableProject(r)
	if err != nil {
		return err
	}
	var in struct {
		Enabled *bool                    `json:"enabled"`
		Extra   *[]notify.ExtraRecipient `json:"extra"`
		Events  []string                 `json:"events"` // read-only, accepted back
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	ps, err := s.out.notify.ProjectSettings(ctx, id)
	if err != nil {
		return err
	}
	if in.Enabled != nil {
		ps.Enabled = *in.Enabled
	}
	if in.Extra != nil {
		ps.Extra = nil
		for _, e := range *in.Extra {
			if !slices.Contains(projectEvents, e.Event) {
				return failf(http.StatusBadRequest, i18n.NotifyEventUnknown, e.Event)
			}
			var n int
			switch e.Kind {
			case "group":
				err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_groups WHERE id = ?`, e.ID).Scan(&n)
				if err == nil && n == 0 {
					return fail(http.StatusBadRequest, i18n.GroupNotFound)
				}
			case "role":
				err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM project_roles WHERE id = ?`, e.ID).Scan(&n)
				if err == nil && n == 0 {
					return fail(http.StatusBadRequest, i18n.RoleNotFound)
				}
			default:
				return fail(http.StatusBadRequest, i18n.NotifyRecipientUnknown)
			}
			if err != nil {
				return err
			}
			ps.Extra = append(ps.Extra, e)
		}
	}
	if err := s.out.notify.SaveProjectSettings(ctx, id, *ps); err != nil {
		return err
	}
	return s.getProjectNotifications(w, r)
}

// Mail settings (administrators) ---------------------------------------------

type mailSettingsOut struct {
	mail.Config
	PasswordSet bool    `json:"passwordSet"`
	BaseURL     *string `json:"baseUrl"`           // links in e-mails; null: not set, links left out
	Problem     string  `json:"problem,omitempty"` // what keeps the saved settings from sending, see mailProblem
}

// mailProblemCredentialsNeedTLS is the problem of settings saved before
// credentials were refused over a plain connection to a remote server.
const mailProblemCredentialsNeedTLS = "credentialsNeedTLS"

func credentialsInClear(c mail.Config) bool {
	return c.Security == mail.SecurityNone && c.Username != "" && !mail.Loopback(c.Host)
}

func (s *Server) getMailSettings(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	ctx := r.Context()
	// The password is never opened here: a lost key must not lock the page.
	cfg, set, err := mail.ReadStored(ctx, s.out.settings)
	if err != nil {
		return err
	}
	cfg = cfg.WithDefaults()
	out := mailSettingsOut{Config: cfg, PasswordSet: set}
	if credentialsInClear(cfg) {
		out.Problem = mailProblemCredentialsNeedTLS
	}
	if base := s.base(); base != "" {
		out.BaseURL = &base
	}
	return writeJSON(w, http.StatusOK, out)
}

// mailSettingsIn is the body of PUT /api/admin/mail. A field left out keeps
// its stored value.
type mailSettingsIn struct {
	Enabled  *bool           `json:"enabled"`
	Host     *string         `json:"host"`
	Port     *int            `json:"port"`
	Security *string         `json:"security"`
	Username *string         `json:"username"`
	Password json.RawMessage `json:"password"`
	From     *string         `json:"from"`
	FromName *string         `json:"fromName"`
	// Read-only fields sent back as received are accepted.
	PasswordSet *bool   `json:"passwordSet"`
	BaseURL     *string `json:"baseUrl"`
}

// putMailSettings saves the SMTP settings. The password is kept when left
// out, removed when null or empty.
func (s *Server) putMailSettings(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	var in mailSettingsIn
	if err := readJSON(r, &in); err != nil {
		return err
	}
	ctx := r.Context()
	before, hadPassword, err := mail.ReadStored(ctx, s.out.settings)
	if err != nil {
		return err
	}
	keepPassword := in.Password == nil
	password, err := s.mailPassword(ctx, in.Password)
	if err != nil {
		return err
	}
	cfg := in.merge(before)
	cfg.Password = password
	if err := validateMailSettings(cfg); err != nil {
		return err
	}
	if err := mailCredentialsRule(before, cfg, keepPassword, hadPassword); err != nil {
		return err
	}
	if err := s.saveMailSettings(ctx, cfg); err != nil {
		return err
	}
	if changes := mailChanges(before, cfg, !keepPassword, hadPassword); len(changes) > 0 {
		s.auditLog(r, audit.SettingsMail, audit.Target{Type: "settings", ID: "mail"}, changes)
	}
	s.out.notify.Invalidate()
	s.forgetEmailLinks()
	s.out.outbox.Wake()
	return s.getMailSettings(w, r)
}

// mailPassword is the password to save: the stored one when raw is nil (left
// out), none when raw is null or empty.
func (s *Server) mailPassword(ctx context.Context, raw json.RawMessage) (string, error) {
	if raw == nil {
		old, err := mail.LoadConfig(ctx, s.out.settings, s.secrets)
		if err != nil {
			s.log.Error("stored mail password not readable", "err", err)
			return "", failf(http.StatusInternalServerError, i18n.MailPasswordUnreadable, i18n.M(i18n.RequestInternalError))
		}
		return old.Password, nil
	}
	var p *string
	if err := json.Unmarshal(raw, &p); err != nil {
		return "", failf(http.StatusBadRequest, i18n.FieldValueInvalid, "password")
	}
	if p == nil {
		return "", nil
	}
	return *p, nil
}

// merge lays the fields the request gives over the stored settings c, the
// password aside, and fills in the defaults.
func (in mailSettingsIn) merge(c mail.Config) mail.Config {
	trim := func(p *string, to *string) {
		if p != nil {
			*to = strings.TrimSpace(*p)
		}
	}
	trim(in.Host, &c.Host)
	trim(in.Username, &c.Username)
	trim(in.From, &c.From)
	trim(in.FromName, &c.FromName)
	if in.Enabled != nil {
		c.Enabled = *in.Enabled
	}
	if in.Security != nil {
		c.Security = *in.Security
	}
	if in.Port != nil {
		c.Port = *in.Port
	}
	return c.WithDefaults()
}

// validateMailSettings checks merged settings on their own, before the rules
// on credentials.
func validateMailSettings(c mail.Config) error {
	if !slices.Contains([]string{mail.SecurityNone, mail.SecurityStartTLS, mail.SecurityTLS}, c.Security) {
		return fail(http.StatusBadRequest, i18n.MailSecurityUnknown)
	}
	if c.Port < 1 || c.Port > 65535 {
		return fail(http.StatusBadRequest, i18n.MailPortInvalid)
	}
	if strings.ContainsAny(c.Host+c.Username+c.FromName, "\r\n") {
		return failf(http.StatusBadRequest, i18n.FieldValueInvalid, "host")
	}
	if c.Enabled && c.Host == "" {
		return fail(http.StatusBadRequest, i18n.MailServerRequired)
	}
	if (c.Enabled || c.From != "") && !mail.ValidAddress(c.From) {
		return fail(http.StatusBadRequest, i18n.MailSenderInvalid)
	}
	return nil
}

// mailCredentialsRule refuses settings that would send credentials where
// they must not go.
func mailCredentialsRule(before, after mail.Config, keepPassword, hadPassword bool) error {
	// The stored password must not follow a change of destination: whoever
	// can change the server could otherwise read it back by pointing the
	// settings at a server of their own and sending a test e-mail.
	if keepPassword && hadPassword && mailDestinationChanged(before, after) {
		return fail(http.StatusBadRequest, i18n.MailPasswordRequired)
	}
	if credentialsInClear(after) {
		return fail(http.StatusBadRequest, i18n.MailCredentialsNeedTLS)
	}
	return nil
}

// saveMailSettings stores c, refusing before any write when a password
// cannot be sealed.
func (s *Server) saveMailSettings(ctx context.Context, c mail.Config) error {
	if c.Password != "" {
		if _, err := s.secrets(); err != nil {
			s.log.Error("mail password not encryptable", "err", err)
			return failf(http.StatusInternalServerError, i18n.MailPasswordEncryptFailed, i18n.M(i18n.RequestInternalError))
		}
	}
	return mail.SaveConfig(ctx, s.out.settings, s.secrets, c)
}

// mailDestinationChanged tells whether the settings send the password
// somewhere else: another server, port, security mode or user name.
func mailDestinationChanged(before, after mail.Config) bool {
	before, after = before.WithDefaults(), after.WithDefaults()
	return !strings.EqualFold(before.Host, after.Host) || before.Port != after.Port ||
		before.Security != after.Security || before.Username != after.Username
}

// mailChanges lists what differs between two states of the SMTP settings,
// for the audit log. The password is never written, only that it changed or
// was removed.
func mailChanges(before, after mail.Config, passwordGiven, hadPassword bool) map[string]any {
	before, after = before.WithDefaults(), after.WithDefaults()
	out := map[string]any{}
	diff(out, "enabled", before.Enabled, after.Enabled)
	diff(out, "host", before.Host, after.Host)
	diff(out, "port", before.Port, after.Port)
	diff(out, "security", before.Security, after.Security)
	diff(out, "username", before.Username, after.Username)
	diff(out, "from", before.From, after.From)
	diff(out, "fromName", before.FromName, after.FromName)
	switch {
	case passwordGiven && after.Password != "":
		out["passwordChanged"] = true
	case passwordGiven && hadPassword:
		out["passwordRemoved"] = true
	}
	return out
}

// testMail sends a test e-mail with the saved settings, now, and explains
// a failure in the administrator's language.
func (s *Server) testMail(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	var in struct {
		To string `json:"to"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	to := strings.TrimSpace(in.To)
	if to == "" {
		to = currentUser(r).Email
	}
	if !mail.ValidAddress(to) {
		return fail(http.StatusBadRequest, i18n.MailRecipientInvalid)
	}
	cfg, err := mail.LoadConfig(r.Context(), s.out.settings, s.secrets)
	if err != nil {
		s.log.Error("stored mail password not readable", "err", err)
		return failf(http.StatusInternalServerError, i18n.MailPasswordUnreadable, i18n.M(i18n.RequestInternalError))
	}
	if cfg.Host == "" || cfg.From == "" {
		return fail(http.StatusConflict, i18n.MailSettingsIncomplete)
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	if err := mail.Send(ctx, cfg, notify.TestMessage(cfg, to, locale(r))); err != nil {
		return smtpError(err)
	}
	return writeJSON(w, http.StatusOK, map[string]any{"sent": true, "to": to})
}

// smtpError explains an SMTP failure, with the server's own words.
func smtpError(err error) error {
	var me *mail.Error
	if !errors.As(err, &me) {
		return failf(http.StatusBadGateway, i18n.MailSendFailed, err.Error())
	}
	detail := me.Err.Error()
	switch me.Stage {
	case mail.StageConnect:
		return failf(http.StatusBadGateway, i18n.MailConnectFailed, detail)
	case mail.StageTLS:
		return failf(http.StatusBadGateway, i18n.MailTLSFailed, detail)
	case mail.StageNoTLS:
		return fail(http.StatusBadGateway, i18n.MailNoStartTLS)
	case mail.StageAuth:
		return failf(http.StatusBadGateway, i18n.MailAuthRefused, detail)
	case mail.StageNoAuth:
		return fail(http.StatusBadGateway, i18n.MailAuthUnsupported)
	case mail.StageInsecure:
		return fail(http.StatusBadGateway, i18n.MailCredentialsNeedTLS)
	case mail.StageSender:
		return failf(http.StatusBadGateway, i18n.MailSenderRefused, detail)
	case mail.StageRecipient:
		return failf(http.StatusBadGateway, i18n.MailRecipientRefused, detail)
	}
	return failf(http.StatusBadGateway, i18n.MailMessageRefused, detail)
}

func (s *Server) mailQueue(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	st, err := s.out.outbox.Stats(r.Context())
	if err != nil {
		return err
	}
	immediate, digests, err := s.out.notify.Pending(r.Context())
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, map[string]any{"pending": st.Pending, "failed": st.Failed,
		"sentLastDay": st.SentLastDay, "oldestPending": st.OldestPending, "recent": st.Recent, "errors": st.Errors,
		"batching": immediate, "digests": digests})
}

func (s *Server) retryMailQueue(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	n, err := s.out.outbox.Retry(r.Context())
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, map[string]int64{"retried": n})
}

// Filter subscriptions --------------------------------------------------------

type subscriptionOut struct {
	ID       int64      `json:"id"`
	FilterID int64      `json:"filterId"`
	Owner    *userRef   `json:"owner"`
	User     *userRef   `json:"user"`
	Group    *idName    `json:"group"`
	Schedule string     `json:"schedule"` // daily, weekly
	Days     []int      `json:"days"`     // ISO weekdays (1 = Monday), for weekly
	Hour     int        `json:"hour"`     // in the server's time zone
	EmptyToo bool       `json:"emptyToo"`
	LastRun  *time.Time `json:"lastRun"`
	NextRun  time.Time  `json:"nextRun"`
	CanEdit  bool       `json:"canEdit"`
}

func (s *Server) subscriptionsOut(ctx context.Context, sub *perm.Subject, list []*notify.Subscription) ([]subscriptionOut, error) {
	var ids []int64
	for _, x := range list {
		ids = append(ids, x.OwnerID, x.UserID)
	}
	users, err := s.users(ctx, ids...)
	if err != nil {
		return nil, err
	}
	out := []subscriptionOut{}
	for _, x := range list {
		mine := x.UserID == sub.UserID || x.OwnerID == sub.UserID
		member := x.GroupID != 0 && sub.InGroup(x.GroupID)
		if !mine && !member && !sub.Admin {
			continue
		}
		o := subscriptionOut{ID: x.ID, FilterID: x.FilterID, Owner: refOf(users, sql.NullInt64{Int64: x.OwnerID, Valid: true}),
			Schedule: x.Frequency, Days: x.Days, Hour: x.Hour, EmptyToo: x.EmptyToo, LastRun: x.LastRun, NextRun: x.NextRun,
			CanEdit: mine || sub.Admin}
		if o.Days == nil {
			o.Days = []int{}
		}
		if x.GroupID != 0 {
			o.Group = &idName{ID: x.GroupID, Name: x.GroupName}
		} else {
			o.User = refOf(users, sql.NullInt64{Int64: x.UserID, Valid: true})
		}
		out = append(out, o)
	}
	return out, nil
}

func (s *Server) listSubscriptions(w http.ResponseWriter, r *http.Request) error {
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	f, err := s.visibleFilter(r, sub)
	if err != nil {
		return err
	}
	list, err := s.out.notify.Subscriptions(r.Context(), f.out.ID)
	if err != nil {
		return err
	}
	out, err := s.subscriptionsOut(r.Context(), sub, list)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

type subscriptionInput struct {
	Group    *int64 `json:"group"` // subscribe a group (administrators); otherwise oneself
	Schedule string `json:"schedule"`
	Days     []int  `json:"days"`
	Hour     int    `json:"hour"`
	EmptyToo bool   `json:"emptyToo"`
}

func (s *Server) readSubscription(r *http.Request, sub *perm.Subject, x *notify.Subscription) error {
	var in subscriptionInput
	if err := readJSON(r, &in); err != nil {
		return err
	}
	switch in.Schedule {
	case "daily":
		in.Days = nil
	case "weekly":
		if len(in.Days) == 0 {
			return fail(http.StatusBadRequest, i18n.SubscriptionDayRequired)
		}
		for _, d := range in.Days {
			if d < 1 || d > 7 {
				return fail(http.StatusBadRequest, i18n.SubscriptionDaysInvalid)
			}
		}
	default:
		return fail(http.StatusBadRequest, i18n.SubscriptionFrequencyUnknown)
	}
	if err := checkHour(in.Hour); err != nil {
		return err
	}
	x.Frequency, x.Days, x.Hour, x.EmptyToo = in.Schedule, in.Days, in.Hour, in.EmptyToo
	if in.Group != nil && *in.Group != 0 {
		if !sub.Admin {
			return fail(http.StatusForbidden, i18n.SubscriptionGroupAdminOnly)
		}
		var name string
		err := s.db.QueryRowContext(r.Context(), `SELECT name FROM user_groups WHERE id = ?`, *in.Group).Scan(&name)
		if errors.Is(err, sql.ErrNoRows) {
			return fail(http.StatusBadRequest, i18n.GroupNotFound)
		}
		if err != nil {
			return err
		}
		x.GroupID, x.GroupName, x.UserID = *in.Group, name, 0
	} else if x.GroupID == 0 {
		x.UserID = sub.UserID
	}
	return nil
}

func (s *Server) createSubscription(w http.ResponseWriter, r *http.Request) error {
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	f, err := s.visibleFilter(r, sub)
	if err != nil {
		return err
	}
	x := &notify.Subscription{FilterID: f.out.ID, OwnerID: sub.UserID}
	if err := s.readSubscription(r, sub, x); err != nil {
		return err
	}
	if err := s.out.notify.SaveSubscription(r.Context(), x); err != nil {
		return err
	}
	return s.respondSubscription(w, r, sub, x.ID, http.StatusCreated)
}

func (s *Server) respondSubscription(w http.ResponseWriter, r *http.Request, sub *perm.Subject, id int64, status int) error {
	x, err := s.out.notify.Subscription(r.Context(), id)
	if err != nil {
		return err
	}
	out, err := s.subscriptionsOut(r.Context(), sub, []*notify.Subscription{x})
	if err != nil || len(out) == 0 {
		return err
	}
	return writeJSON(w, status, out[0])
}

// editableSubscription loads the subscription named in the path, on a
// filter the user sees, if they may change it: theirs, or any for
// administrators.
func (s *Server) editableSubscription(r *http.Request, sub *perm.Subject) (*notify.Subscription, error) {
	f, err := s.visibleFilter(r, sub)
	if err != nil {
		return nil, err
	}
	id, err := pathID(r, "sid")
	if err != nil {
		return nil, err
	}
	x, err := s.out.notify.Subscription(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) || err == nil && x.FilterID != f.out.ID {
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}
	if x.UserID != sub.UserID && x.OwnerID != sub.UserID && !sub.Admin {
		return nil, errNotFound
	}
	return x, nil
}

func (s *Server) updateSubscription(w http.ResponseWriter, r *http.Request) error {
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	x, err := s.editableSubscription(r, sub)
	if err != nil {
		return err
	}
	if err := s.readSubscription(r, sub, x); err != nil {
		return err
	}
	if err := s.out.notify.SaveSubscription(r.Context(), x); err != nil {
		return err
	}
	return s.respondSubscription(w, r, sub, x.ID, http.StatusOK)
}

func (s *Server) deleteSubscription(w http.ResponseWriter, r *http.Request) error {
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	x, err := s.editableSubscription(r, sub)
	if err != nil {
		return err
	}
	if err := s.out.notify.DeleteSubscription(r.Context(), x.ID); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// runSubscription mails the results now (the schedule is unchanged).
func (s *Server) runSubscription(w http.ResponseWriter, r *http.Request) error {
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	x, err := s.editableSubscription(r, sub)
	if err != nil {
		return err
	}
	cfg, err := mail.LoadConfig(r.Context(), s.out.settings, s.secrets)
	if err != nil || !cfg.Enabled {
		return fail(http.StatusConflict, i18n.MailDisabled)
	}
	n, err := s.out.notify.RunSubscription(r.Context(), x)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, map[string]int{"emails": n})
}

// filterRunner runs saved filters for subscription e-mails.
type filterRunner struct{ s *Server }

func (fr filterRunner) RunFilter(ctx context.Context, sub *perm.Subject, filterID int64, limit int) (*notify.FilterResult, bool, error) {
	s := fr.s
	rows, err := s.loadFilters(ctx, sub, filterID)
	if err != nil || len(rows) == 0 {
		return nil, false, err
	}
	f := rows[0].out
	cat, err := s.catalogue(ctx)
	if err != nil {
		return nil, false, err
	}
	where, args, order, err := s.searchCondition(ctx, sub, url.Values{"q": {f.Query}})
	if err != nil {
		var he *httpError
		if errors.As(err, &he) {
			return nil, false, fmt.Errorf("filter %d no longer runs: %s", filterID, he.msg.String())
		}
		return nil, false, err
	}
	from := searchFrom + ` WHERE ` + where
	res := &notify.FilterResult{Name: f.Name, Query: f.Query}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*)`+from, args...).Scan(&res.Total); err != nil {
		return nil, false, err
	}
	var list []*issueRow
	var ids []int64
	if err := sqlq.Each(ctx, s.db, `SELECT `+issueColumns+from+` ORDER BY `+order+`, i.id DESC LIMIT ?`, func(rows *sql.Rows) error {
		iss, err := scanIssue(rows)
		if err != nil {
			return err
		}
		list = append(list, iss)
		ids = append(ids, iss.AssigneeID.Int64)
		return nil
	}, append(args, limit)...); err != nil {
		return nil, false, err
	}
	users, err := s.users(ctx, ids...)
	if err != nil {
		return nil, false, err
	}
	for _, iss := range list {
		l := notify.IssueLine{Key: iss.Key(), Summary: iss.Summary, Status: cat.statusOf(iss.StatusID).Name, Due: dateOnly(iss.Due)}
		if p := cat.priorityOf(iss.PriorityID); p != nil {
			l.Priority = p.Name
		}
		if a := refOf(users, iss.AssigneeID); a != nil {
			l.Assignee = a.Name
		}
		res.Issues = append(res.Issues, l)
	}
	return res, true, nil
}

// notificationRoutes registers the routes of e-mail and webhook settings.
func (s *Server) notificationRoutes(route func(string, handler, bool)) {
	route("GET /api/me/notifications", s.getMyNotifications, true)
	route("PUT /api/me/notifications", s.putMyNotifications, true)
	route("GET /api/projects/{key}/notifications", s.getProjectNotifications, true)
	route("PUT /api/projects/{key}/notifications", s.putProjectNotifications, true)
	route("GET /api/me/subscriptions", s.mySubscriptions, true)
	route("GET /api/filters/{id}/subscriptions", s.listSubscriptions, true)
	route("POST /api/filters/{id}/subscriptions", s.createSubscription, true)
	route("PUT /api/filters/{id}/subscriptions/{sid}", s.updateSubscription, true)
	route("DELETE /api/filters/{id}/subscriptions/{sid}", s.deleteSubscription, true)
	route("POST /api/filters/{id}/subscriptions/{sid}/run", s.runSubscription, true)
	route("GET /api/admin/mail", s.getMailSettings, true)
	route("PUT /api/admin/mail", s.putMailSettings, true)
	route("POST /api/admin/mail/test", s.testMail, true)
	route("GET /api/admin/mail/queue", s.mailQueue, true)
	route("POST /api/admin/mail/queue/retry", s.retryMailQueue, true)
	route("GET /api/admin/webhooks", s.listWebhooks, true)
	route("POST /api/admin/webhooks", s.createWebhook, true)
	route("GET /api/admin/webhooks/events", s.webhookEvents, true)
	route("GET /api/admin/webhooks/{id}", s.getWebhook, true)
	route("PUT /api/admin/webhooks/{id}", s.updateWebhook, true)
	route("DELETE /api/admin/webhooks/{id}", s.deleteWebhook, true)
	route("GET /api/admin/webhooks/{id}/deliveries", s.webhookDeliveries, true)
	route("POST /api/admin/webhooks/{id}/test", s.testWebhook, true)
}

const (
	maxStreamsPerUser = 10
	maxStreamsTotal   = 1000
)

type streamSlots struct {
	mu      sync.Mutex
	perUser map[int64]int
	total   int
}

func (c *streamSlots) acquire(user int64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.perUser == nil {
		c.perUser = map[int64]int{}
	}
	if c.total >= maxStreamsTotal || c.perUser[user] >= maxStreamsPerUser {
		return false
	}
	c.total++
	c.perUser[user]++
	return true
}

func (c *streamSlots) release(user int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.total--
	if c.perUser[user]--; c.perUser[user] == 0 {
		delete(c.perUser, user)
	}
}

// events streams change notifications (Server-Sent Events). Permissions are
// re-read every minute so a revoked access stops the flow.
func (s *Server) events(w http.ResponseWriter, r *http.Request) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fail(http.StatusInternalServerError, i18n.RequestStreamingUnsupported)
	}
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	return s.streamEvents(w, r, flusher, sub)
}

// streamEvents ends when the client goes away, the server shuts down or the
// session behind the stream ends, so its slot is always given back.
func (s *Server) streamEvents(w http.ResponseWriter, r *http.Request, flusher http.Flusher, sub *perm.Subject) error {
	if !s.streams.acquire(sub.UserID) {
		return fail(http.StatusTooManyRequests, i18n.RequestTooManyStreams)
	}
	defer s.streams.release(sub.UserID)
	current := make(chan *perm.Subject, 1)
	current <- sub
	can := func(e live.Event) bool {
		s := <-current
		current <- s
		return s.CanSee(e.Issue) && s.SeesRestricted(e.Issue.ProjectID, e.Group, e.Role)
	}
	ch, unsubscribe := s.hub.Subscribe(can)
	defer unsubscribe()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "retry: 3000\n\n")
	flusher.Flush()

	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()
	refresh := time.NewTicker(time.Minute)
	defer refresh.Stop()
	for {
		select {
		case <-r.Context().Done():
			return nil
		case <-s.streamsDone:
			return nil
		case e := <-ch:
			data, _ := json.Marshal(e)
			fmt.Fprintf(w, "event: change\ndata: %s\n\n", data)
			flusher.Flush()
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case <-refresh.C:
			// End the stream once the session is over or the account disabled.
			u, err := s.authenticate(r)
			if err != nil || u.ID != sub.UserID {
				return nil
			}
			fresh, err := s.perms.Subject(r.Context(), u.ID, u.Admin)
			if err != nil {
				return nil
			}
			<-current
			current <- fresh
		}
	}
}
