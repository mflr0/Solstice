package api

// Links sent by e-mail: "forgot your password", the reset an administrator
// sends, and the invitation of a new account. A link carries a random token
// (package auth keeps only its hash), works once, expires, and is replaced by
// the next one issued for the account.
//
// What the design guarantees:
//
//   - the answer to "forgot your password" is the same whether or not an
//     account matches and whether or not e-mail is set up, and is never
//     shorter than forgotFloor, so neither its content nor its speed tells;
//   - the e-mail goes to the address stored on the account, never to one
//     typed by whoever asks, and its link is built from the instance's public
//     address (a setting), never from the request's Host header;
//   - requests are counted per client address (the answer then is a refusal,
//     which depends on the address only) and per account (further requests
//     are silently not mailed), so nobody can flood a mailbox;
//   - a link is a secret: the token is never in an API answer, never in the
//     audit log, and the outbox wipes the message once it is sent.

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"solstice/internal/audit"
	"solstice/internal/auth"
	"solstice/internal/i18n"
	"solstice/internal/mail"
	"solstice/internal/notify"
	"solstice/internal/sqlq"
)

const (
	// forgotFloor is the least time the answer to "forgot your password"
	// takes: work that happens only for a matching account (a few queries)
	// then cannot be told by timing.
	forgotFloor = 400 * time.Millisecond
	// forgotSpacing is the least time between two reset e-mails for one
	// account: asking again sooner mails nothing (a stream of requests could
	// otherwise keep replacing the link its owner is about to use).
	forgotSpacing = time.Minute
	// forgotMatches bounds the accounts one identifier can match (accounts
	// may share an address).
	forgotMatches = 5
	// maxIdentifier is the longest user name or address accepted.
	maxIdentifier = 255
)

// accountMail caches whether links can be sent.
type accountMail struct {
	mu    sync.Mutex
	ready bool
	at    time.Time
}

func (s *Server) accountLinkRoutes(route func(string, handler, bool)) {
	route("POST /api/auth/forgot", s.forgotPassword, false)
	route("GET /api/auth/reset/{token}", s.checkResetLink, false)
	route("POST /api/auth/reset", s.useResetLink, false)
	route("POST /api/admin/users/{id}/invite", s.adminInviteUser, true)
}

// linkConfig returns the mail settings, and whether links can be sent: e-mail
// is on, complete, and the instance's public address is known.
func (s *Server) linkConfig(ctx context.Context) (mail.Config, bool) {
	if s.db == nil {
		return mail.Config{}, false
	}
	cfg, err := mail.LoadConfig(ctx, s.out.settings, s.secrets)
	if err != nil {
		s.log.Warn("reading the mail settings failed", "err", err)
		return cfg, false
	}
	return cfg, cfg.Enabled && cfg.Host != "" && cfg.From != "" && s.base() != ""
}

// emailLinksReady is linkConfig's answer, remembered for a few seconds: the
// sign-in page asks for it.
func (s *Server) emailLinksReady(ctx context.Context) bool {
	a := &s.acc
	a.mu.Lock()
	defer a.mu.Unlock()
	if time.Since(a.at) < 15*time.Second {
		return a.ready
	}
	_, a.ready = s.linkConfig(ctx)
	a.at = time.Now()
	return a.ready
}

// forgetEmailLinks drops the remembered answer (the mail settings changed).
func (s *Server) forgetEmailLinks() {
	s.acc.mu.Lock()
	s.acc.at = time.Time{}
	s.acc.mu.Unlock()
}

// accountLink is the address a link leads to, from the public address.
func (s *Server) accountLink(token string) string {
	return strings.TrimSuffix(s.base(), "/") + "/reset/" + url.PathEscape(token)
}

// linkTarget is an account a link is sent to.
type linkTarget struct {
	ID                                int64
	Username, Name, Email, UserLocale string
	Active                            bool
	LastLogin                         sql.NullTime
}

// linkTargetColumns are the columns of users that scanLinkTarget reads.
const linkTargetColumns = `id, username, display_name, email, active, last_login_at, COALESCE(locale, '')`

// scanLinkTarget reads a linkTarget, its address trimmed: an import keeps
// the spaces a Jira address may have, and the link goes to the address.
func scanLinkTarget(sc interface{ Scan(...any) error }) (linkTarget, error) {
	var t linkTarget
	err := sc.Scan(&t.ID, &t.Username, &t.Name, &t.Email, &t.Active, &t.LastLogin, &t.UserLocale)
	t.Email = strings.TrimSpace(t.Email)
	return t, err
}

// linkTargetOf loads the account a link leads to, for its id.
func (s *Server) linkTargetOf(ctx context.Context, id int64) (*linkTarget, error) {
	t, err := scanLinkTarget(s.db.QueryRowContext(ctx, `SELECT `+linkTargetColumns+` FROM users WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNotFound
	}
	return &t, err
}

// queueAccountLink issues a token of a purpose for an account and queues the
// e-mail carrying its link, in the caller's transaction, which must commit
// before Wake is called (see sendAccountLink). Tokens and mails issued
// before for the account are replaced.
func (s *Server) queueAccountLink(ctx context.Context, tx *sql.Tx, cfg mail.Config, to *linkTarget, purpose string) error {
	token, err := auth.IssueAccountToken(ctx, tx, to.ID, purpose, time.Now())
	if err != nil {
		return err
	}
	// A link still waiting in the queue is dead now: do not send it.
	if _, err := tx.ExecContext(ctx, `DELETE FROM mail_outbox WHERE user_id = ? AND kind IN (?, ?)
		AND status = 'pending' AND claim IS NULL`, to.ID, mail.KindReset, mail.KindInvitation); err != nil {
		return err
	}
	loc := i18n.Pick(to.UserLocale, s.defaultLanguage())
	link := s.accountLink(token)
	var msg mail.Message
	if purpose == auth.PurposeInvite {
		msg = notify.InvitationMessage(cfg, to.ID, to.Email, to.Name, to.Username, loc, link)
	} else {
		msg = notify.ResetMessage(cfg, to.ID, to.Email, to.Name, to.Username, loc, link)
	}
	return s.out.outbox.Enqueue(ctx, tx, msg)
}

// sendAccountLink queues a link's e-mail in a transaction of its own and
// wakes the outbox.
func (s *Server) sendAccountLink(ctx context.Context, cfg mail.Config, to *linkTarget, purpose string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := s.queueAccountLink(ctx, tx, cfg, to, purpose); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.out.outbox.Wake()
	return nil
}

// auditAccount records an action on an account that its own person does
// without being signed in (a reset asked for or done, an invitation accepted).
func (s *Server) auditAccount(r *http.Request, action string, id int64, username string, details map[string]any) {
	e := audit.Entry{UserID: id, Username: username, IP: clientIP(r, s.proxies), Action: action,
		Target: audit.Target{Type: "user", ID: strconv.FormatInt(id, 10), Label: username}, Details: details}
	if err := audit.Record(r.Context(), s.db, e); err != nil {
		s.log.Error("audit log write failed", "action", action, "err", err)
	}
}

// hold waits until d has passed since start (or the request ends).
func (s *Server) hold(ctx context.Context, start time.Time, d time.Duration) {
	if rest := d - time.Since(start); rest > 0 {
		s.sleep(ctx, rest)
	}
}

// forgotPassword takes a user name or an e-mail address and, if it matches
// an active account with an address and links can be sent, mails a link to
// choose a new password. The answer never says which.
func (s *Server) forgotPassword(w http.ResponseWriter, r *http.Request) error {
	started := time.Now()
	var in struct {
		Identifier string `json:"identifier"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	id := auth.CanonicalUsername(in.Identifier)
	if id == "" || utf8.RuneCountInString(id) > maxIdentifier {
		return failf(http.StatusBadRequest, i18n.FieldValueInvalid, "identifier")
	}
	ipKey := "forgot-ip:" + clientIP(r, s.proxies)
	if d := s.limiter.Locked(ipKey); d > 0 {
		return tooManyAttempts(d)
	}
	// Every request counts, whatever it finds. Only the client address is
	// counted here: a name key would let anyone mute someone else's account.
	s.limiter.Failure(ipKey)
	if err := s.mailResetLinks(r, id); err != nil {
		s.log.Error("sending a password reset link failed", "err", err)
	}
	s.hold(r.Context(), started, forgotFloor)
	return writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// mailResetLinks mails a link to each active account that has an address and
// is named by id, as a user name or an address.
func (s *Server) mailResetLinks(r *http.Request, id string) error {
	ctx := r.Context()
	cfg, ok := s.linkConfig(ctx)
	if !ok {
		return nil
	}
	var targets []linkTarget
	err := sqlq.Each(ctx, s.db, `SELECT `+linkTargetColumns+` FROM users
		WHERE active AND email <> '' AND (username = ? OR LOWER(email) = ?) ORDER BY id LIMIT ?`,
		func(rows *sql.Rows) error {
			t, err := scanLinkTarget(rows)
			if err != nil {
				return err
			}
			targets = append(targets, t)
			return nil
		}, id, id, forgotMatches)
	if err != nil {
		return err
	}
	var first error
	for i := range targets {
		t := &targets[i]
		if !mail.ValidAddress(t.Email) {
			continue
		}
		var recent int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM account_tokens WHERE user_id = ? AND purpose = ?
			AND created_at > ?`, t.ID, auth.PurposeReset, time.Now().UTC().Add(-forgotSpacing)).Scan(&recent); err != nil {
			first = errors.Join(first, err)
			continue
		}
		if recent > 0 {
			continue
		}
		if err := s.sendAccountLink(ctx, cfg, t, auth.PurposeReset); err != nil {
			first = errors.Join(first, err)
			continue
		}
		s.auditAccount(r, audit.PasswordResetRequest, t.ID, t.Username, map[string]any{"method": "form"})
	}
	return first
}

// linkGuard refuses a client address that keeps presenting links that do not
// work.
func (s *Server) linkGuard(r *http.Request) (key string, err error) {
	key = "reset-ip:" + clientIP(r, s.proxies)
	if d := s.limiter.Locked(key); d > 0 {
		return key, tooManyAttempts(d)
	}
	return key, nil
}

// checkResetLink tells whether a link can still be used, so that the page
// says so before asking for a password.
func (s *Server) checkResetLink(w http.ResponseWriter, r *http.Request) error {
	key, err := s.linkGuard(r)
	if err != nil {
		return err
	}
	tok, err := auth.LookupAccountToken(r.Context(), s.db, r.PathValue("token"), time.Now())
	if errors.Is(err, auth.ErrBadToken) {
		s.limiter.Failure(key)
		return fail(http.StatusBadRequest, i18n.AccountResetLinkInvalid)
	}
	if err != nil {
		return err
	}
	t, err := s.linkTargetOf(r.Context(), tok.UserID)
	if err != nil {
		return err
	}
	w.Header().Set("Cache-Control", "no-store")
	return writeJSON(w, http.StatusOK, map[string]any{"purpose": tok.Purpose, "name": t.Name, "username": t.Username})
}

// useResetLink sets the password the person chose, once, with a valid link:
// every session and personal access token of the account ends, the other
// links die, and the person is signed in.
func (s *Server) useResetLink(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	ctx := r.Context()
	key, err := s.linkGuard(r)
	if err != nil {
		return err
	}
	bad := func() error {
		s.limiter.Failure(key)
		return fail(http.StatusBadRequest, i18n.AccountResetLinkInvalid)
	}
	tok, err := auth.LookupAccountToken(ctx, s.db, in.Token, time.Now())
	if errors.Is(err, auth.ErrBadToken) {
		return bad()
	}
	if err != nil {
		return err
	}
	target, err := s.linkTargetOf(ctx, tok.UserID)
	if err != nil {
		return err
	}
	if err := s.checkNewPassword(ctx, in.Password); err != nil {
		return err
	}
	if strings.EqualFold(strings.TrimSpace(in.Password), target.Username) {
		return fail(http.StatusBadRequest, i18n.AccountPasswordIsUsername)
	}
	release, err := s.limiter.Acquire(ctx)
	if err != nil {
		return err
	}
	hash, err := auth.HashPassword(in.Password)
	release()
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now()
	used, err := auth.UseAccountToken(ctx, tx, in.Token, now)
	if errors.Is(err, auth.ErrBadToken) {
		return bad()
	}
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, hash, used.UserID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, used.UserID); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM api_tokens WHERE user_id = ?`, used.UserID)
	if err != nil {
		return err
	}
	revoked, _ := res.RowsAffected()
	if err := auth.RevokeAccountTokens(ctx, tx, used.UserID, now); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	action := audit.PasswordResetDone
	if used.Purpose == auth.PurposeInvite {
		action = audit.UserInviteAccept
	}
	s.auditAccount(r, action, used.UserID, target.Username, map[string]any{"revokedAccessTokens": revoked})

	u, err := s.userByID(ctx, used.UserID)
	if err != nil {
		return err
	}
	if err := s.startSession(w, r, &u); err != nil {
		return err
	}
	s.auditLogin(r, u.Username, &u, "link")
	w.Header().Set("Cache-Control", "no-store")
	return writeJSON(w, http.StatusOK, meOf(&u))
}

// adminInviteUser sends an invitation again to an account that never signed
// in (a new invitation replaces the previous one).
func (s *Server) adminInviteUser(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	return s.adminSendLink(w, r, id, inviteLink)
}

// adminResetLink sends an account a link to choose its new password, instead
// of giving it one: nothing changes until the person uses the link.
func (s *Server) adminResetLink(w http.ResponseWriter, r *http.Request, id int64) error {
	return s.adminSendLink(w, r, id, resetLink)
}

// adminLink is a link an administrator sends to an account.
type adminLink struct {
	purpose string // of the token, auth.Purpose...
	action  string // audited, audit....
	details func() map[string]any
	// neverSignedIn: only for an account that never signed in.
	neverSignedIn bool
}

var (
	inviteLink = adminLink{purpose: auth.PurposeInvite, action: audit.UserInvite, neverSignedIn: true,
		details: func() map[string]any { return map[string]any{"resent": true} }}
	resetLink = adminLink{purpose: auth.PurposeReset, action: audit.PasswordResetRequest,
		details: func() map[string]any { return map[string]any{"method": "link", "by": "administrator"} }}
)

// refusal is why the link cannot go to t, or nil.
func (l adminLink) refusal(t *linkTarget) error {
	switch {
	case !t.Active:
		return fail(http.StatusConflict, i18n.AccountInactive)
	case l.neverSignedIn && t.LastLogin.Valid:
		return fail(http.StatusConflict, i18n.AccountAlreadySignedIn)
	case !mail.ValidAddress(t.Email):
		return fail(http.StatusBadRequest, i18n.AccountEmailMissing)
	}
	return nil
}

// adminSendLink sends an account the link l on an administrator's ask,
// unless l refuses the account or the mail setup cannot send, audits the ask
// and answers {"sent": true}.
func (s *Server) adminSendLink(w http.ResponseWriter, r *http.Request, id int64, l adminLink) error {
	ctx := r.Context()
	to, err := s.linkTargetOf(ctx, id)
	if err != nil {
		return err
	}
	if err := l.refusal(to); err != nil {
		return err
	}
	cfg, ok := s.linkConfig(ctx)
	if !ok {
		return fail(http.StatusConflict, i18n.AccountMailUnavailable)
	}
	if err := s.sendAccountLink(ctx, cfg, to, l.purpose); err != nil {
		return err
	}
	s.auditLog(r, l.action, audit.Target{Type: "user", ID: strconv.FormatInt(id, 10), Label: to.Username}, l.details())
	return writeJSON(w, http.StatusOK, map[string]bool{"sent": true})
}
