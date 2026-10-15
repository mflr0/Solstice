package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"solstice/internal/perm"
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
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM issues i JOIN projects p ON p.id = i.project_id
		LEFT JOIN priorities pr ON pr.id = i.priority_id WHERE i.id = ? AND (`+cond+`)`,
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
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return nil, errNotFound
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
	if wh.Name == "" || utf8.RuneCountInString(wh.Name) > 255 {
		return nil, fail(http.StatusBadRequest, "Le nom est obligatoire (255 caractères au plus).")
	}
	if in.URL != nil {
		wh.URL = strings.TrimSpace(*in.URL)
	}
	if len(wh.URL) > 2000 {
		return nil, fail(http.StatusBadRequest, "Adresse trop longue (2000 caractères au plus).")
	}
	switch err := webhook.CheckURL(r.Context(), wh.URL); {
	case errors.Is(err, webhook.ErrBlocked):
		return nil, fail(http.StatusBadRequest,
			"Adresse refusée : les adresses locales au lien et de métadonnées cloud (169.254.0.0/16…) sont interdites.")
	case errors.Is(err, webhook.ErrScheme):
		return nil, fail(http.StatusBadRequest, "Adresse invalide : http:// ou https:// attendu.")
	case err != nil:
		return nil, fail(http.StatusBadRequest, "Adresse invalide : hôte manquant.")
	}
	if in.Events != nil {
		wh.Events = nil
		for _, e := range *in.Events {
			if !slices.Contains(webhook.Events, e) {
				return nil, failf(http.StatusBadRequest, "Événement inconnu : %s.", e)
			}
			if !slices.Contains(wh.Events, e) {
				wh.Events = append(wh.Events, e)
			}
		}
	}
	if len(wh.Events) == 0 {
		return nil, fail(http.StatusBadRequest, "Choisissez au moins un événement.")
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
		if utf8.RuneCountInString(*in.Secret) > 255 {
			return nil, fail(http.StatusBadRequest, "Le secret fait 255 caractères au plus.")
		}
		if *in.Secret != "" {
			if _, err := s.secrets(); err != nil {
				return nil, failf(http.StatusInternalServerError, "Impossible de chiffrer le secret : %s", err.Error())
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
	return writeJSON(w, http.StatusCreated, created)
}

func (s *Server) updateWebhook(w http.ResponseWriter, r *http.Request) error {
	wh, err := s.adminWebhook(r)
	if err != nil {
		return err
	}
	secretText, err := s.readWebhook(r, wh)
	if err != nil {
		return err
	}
	if err := s.out.hooks.Save(r.Context(), wh, secretText, 0); err != nil {
		return err
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
	w.WriteHeader(http.StatusNoContent)
	return nil
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
