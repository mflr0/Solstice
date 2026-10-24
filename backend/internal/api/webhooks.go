package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"unicode/utf8"

	"solstice/internal/audit"
	"solstice/internal/i18n"
	"solstice/internal/perm"
	"solstice/internal/sqlq"
	"solstice/internal/webhook"
)

// Webhooks (administrators). See package webhook.

// matchJQL tells whether an issue matches a webhook's JQL filter. The query
// runs without a user: currentUser() matches nobody, and no visibility
// applies (webhooks are set up by administrators, for every issue).
func (s *Server) matchJQL(ctx context.Context, src string, issueID int64) (bool, error) {
	cond, args, _, err := s.compileJQL(ctx, &perm.Subject{}, src)
	if err != nil {
		return false, err
	}
	var n int
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*)`+searchFrom+` WHERE i.id = ? AND (`+cond+`)`,
		append([]any{issueID}, args...)...).Scan(&n)
	return n > 0, err
}

func (s *Server) webhookEvents(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, webhook.Events)
}

func (s *Server) listWebhooks(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	list, err := s.out.hooks.List(r.Context())
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, list)
}

// adminWebhook loads the webhook named in the path, for an administrator.
func (s *Server) adminWebhook(r *http.Request) (*webhook.Webhook, error) {
	if err := requireAdmin(r); err != nil {
		return nil, err
	}
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	wh, err := s.out.hooks.Get(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNotFound
	}
	return wh, err
}

func (s *Server) getWebhook(w http.ResponseWriter, r *http.Request) error {
	wh, err := s.adminWebhook(r)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, wh)
}

type webhookInput struct {
	Name        *string   `json:"name"`
	URL         *string   `json:"url"`
	Events      *[]string `json:"events"`
	JQL         *string   `json:"jql"`
	Secret      *string   `json:"secret"` // left out: unchanged; "": removed
	Enabled     *bool     `json:"enabled"`
	ExcludeBody *bool     `json:"excludeBody"`
	// Read-only fields sent back as received are accepted.
	ID           *int64 `json:"id"`
	HasSecret    *bool  `json:"hasSecret"`
	Created      any    `json:"created"`
	Updated      any    `json:"updated"`
	LastDelivery any    `json:"lastDelivery"`
}

// readWebhook applies a request to a webhook, checking every field.
func (s *Server) readWebhook(r *http.Request, wh *webhook.Webhook) (*string, error) {
	var in webhookInput
	if err := readJSON(r, &in); err != nil {
		return nil, err
	}
	if in.Name != nil {
		wh.Name = strings.TrimSpace(*in.Name)
	}
	if wh.Name == "" || utf8.RuneCountInString(wh.Name) > sqlq.VarcharWidth {
		return nil, fail(http.StatusBadRequest, i18n.WebhookNameRequired)
	}
	if in.URL != nil {
		wh.URL = strings.TrimSpace(*in.URL)
	}
	if len(wh.URL) > 2000 {
		return nil, fail(http.StatusBadRequest, i18n.WebhookURLTooLong)
	}
	switch err := webhook.CheckURL(r.Context(), wh.URL); {
	case errors.Is(err, webhook.ErrBlocked):
		return nil, fail(http.StatusBadRequest, i18n.WebhookURLForbidden)
	case errors.Is(err, webhook.ErrScheme):
		return nil, fail(http.StatusBadRequest, i18n.WebhookURLScheme)
	case err != nil:
		return nil, fail(http.StatusBadRequest, i18n.WebhookURLNoHost)
	}
	if in.Events != nil {
		wh.Events = nil
		for _, e := range *in.Events {
			if !slices.Contains(webhook.Events, e) {
				return nil, failf(http.StatusBadRequest, i18n.NotifyEventUnknown, e)
			}
			if !slices.Contains(wh.Events, e) {
				wh.Events = append(wh.Events, e)
			}
		}
	}
	if len(wh.Events) == 0 {
		return nil, fail(http.StatusBadRequest, i18n.WebhookEventRequired)
	}
	if in.JQL != nil {
		wh.JQL = strings.TrimSpace(*in.JQL)
	}
	if wh.JQL != "" {
		if _, _, _, err := s.compileJQL(r.Context(), &perm.Subject{}, wh.JQL); err != nil {
			return nil, err
		}
	}
	if in.Enabled != nil {
		wh.Enabled = *in.Enabled
	}
	if in.ExcludeBody != nil {
		wh.ExcludeBody = *in.ExcludeBody
	}
	if in.Secret != nil {
		if utf8.RuneCountInString(*in.Secret) > sqlq.VarcharWidth {
			return nil, fail(http.StatusBadRequest, i18n.WebhookSecretTooLong)
		}
		if *in.Secret != "" {
			if _, err := s.secrets(); err != nil {
				s.log.Error("webhook secret not encryptable", "err", err)
				return nil, failf(http.StatusInternalServerError, i18n.WebhookSecretEncryptFailed, i18n.M(i18n.RequestInternalError))
			}
		}
	}
	return in.Secret, nil
}

func (s *Server) createWebhook(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	wh := &webhook.Webhook{Enabled: true}
	secretText, err := s.readWebhook(r, wh)
	if err != nil {
		return err
	}
	if err := s.out.hooks.Save(r.Context(), wh, secretText, currentUser(r).ID); err != nil {
		return err
	}
	created, err := s.out.hooks.Get(r.Context(), wh.ID)
	if err != nil {
		return err
	}
	s.auditLog(r, audit.WebhookCreate, webhookTarget(created), webhookChanges(&webhook.Webhook{}, created, secretText))
	return writeJSON(w, http.StatusCreated, created)
}

func (s *Server) updateWebhook(w http.ResponseWriter, r *http.Request) error {
	wh, err := s.adminWebhook(r)
	if err != nil {
		return err
	}
	before := *wh
	before.Events = slices.Clone(wh.Events)
	secretText, err := s.readWebhook(r, wh)
	if err != nil {
		return err
	}
	if err := s.out.hooks.Save(r.Context(), wh, secretText, 0); err != nil {
		return err
	}
	if changes := webhookChanges(&before, wh, secretText); len(changes) > 0 {
		s.auditLog(r, audit.WebhookUpdate, webhookTarget(wh), changes)
	}
	return s.getWebhook(w, r)
}

func (s *Server) deleteWebhook(w http.ResponseWriter, r *http.Request) error {
	wh, err := s.adminWebhook(r)
	if err != nil {
		return err
	}
	if err := s.out.hooks.Delete(r.Context(), wh.ID); err != nil {
		return err
	}
	s.auditLog(r, audit.WebhookDelete, webhookTarget(wh), map[string]any{
		"url": redactedHookURL(wh.URL), "events": wh.Events, "enabled": wh.Enabled})
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// webhookTarget names a webhook in the audit log.
func webhookTarget(wh *webhook.Webhook) audit.Target {
	return target(audit.TargetWebhook, wh.ID, wh.Name)
}

// redactedHookURL is a webhook address without what may be a secret: the
// user name and password, and the path and query (Slack and similar
// services put their access token there). "https://example.org/…" tells
// that there is a path.
func redactedHookURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	out := u.Scheme + "://" + u.Host
	if strings.Trim(u.Path, "/") != "" || u.RawQuery != "" {
		out += "/…"
	}
	return out
}

// webhookChanges lists what differs between two states of a webhook, for the
// audit log (a new webhook is compared with an empty one). The secret and the
// path of the address are never written, only that they changed.
func webhookChanges(before, after *webhook.Webhook, secret *string) map[string]any {
	out := map[string]any{}
	diff(out, "name", before.Name, after.Name)
	if before.URL != after.URL {
		if !diff(out, "url", redactedHookURL(before.URL), redactedHookURL(after.URL)) {
			out["urlPathChanged"] = true
		}
	}
	diff(out, "events", nonNil(before.Events), nonNil(after.Events))
	diff(out, "jql", before.JQL, after.JQL)
	diff(out, "enabled", before.Enabled, after.Enabled)
	diff(out, "excludeBody", before.ExcludeBody, after.ExcludeBody)
	switch {
	case secret != nil && *secret != "":
		out["secretChanged"] = true
	case secret != nil && before.HasSecret:
		out["secretRemoved"] = true
	}
	return out
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func (s *Server) webhookDeliveries(w http.ResponseWriter, r *http.Request) error {
	wh, err := s.adminWebhook(r)
	if err != nil {
		return err
	}
	list, err := s.out.hooks.Deliveries(r.Context(), wh.ID, 100)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, list)
}

// testWebhook sends a test event now and returns the delivery.
func (s *Server) testWebhook(w http.ResponseWriter, r *http.Request) error {
	wh, err := s.adminWebhook(r)
	if err != nil {
		return err
	}
	d, err := s.out.hooks.Test(r.Context(), wh.ID, currentUser(r).ID)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, d)
}
