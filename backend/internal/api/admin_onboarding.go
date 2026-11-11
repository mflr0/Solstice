package api

// The setup steps a new instance still lacks, for the administration's
// overview and the Getting started card, and whether an administrator hid
// that card.

import (
	"net/http"

	"solstice/internal/audit"
	"solstice/internal/i18n"
)

const onboardingDismissed = "onboarding.dismissed"

type onboarding struct {
	Projects   bool `json:"projects"`
	OtherUsers bool `json:"otherUsers"`
	Mail       bool `json:"mail"`
	Boards     bool `json:"boards"`
	Issues     bool `json:"issues"`
	Dismissed  bool `json:"dismissed"`
}

func (s *Server) getOnboarding(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	ctx := r.Context()
	var o onboarding
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM projects),
		EXISTS(SELECT 1 FROM users WHERE active AND id <> ?), EXISTS(SELECT 1 FROM boards),
		EXISTS(SELECT 1 FROM issues)`, currentUser(r).ID).Scan(&o.Projects, &o.OtherUsers, &o.Boards, &o.Issues); err != nil {
		return err
	}
	cfg, err := s.mailConfig(ctx)
	o.Mail = err == nil && cfg.Enabled
	if _, err := s.out.settings.GetJSON(ctx, onboardingDismissed, &o.Dismissed); err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, o)
}

func (s *Server) putOnboarding(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	var in struct {
		Dismissed *bool `json:"dismissed"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	if in.Dismissed == nil {
		return failf(http.StatusBadRequest, i18n.FieldValueInvalid, "dismissed")
	}
	ctx := r.Context()
	var before bool
	if _, err := s.out.settings.GetJSON(ctx, onboardingDismissed, &before); err != nil {
		return err
	}
	if err := s.out.settings.SetJSON(ctx, onboardingDismissed, *in.Dismissed); err != nil {
		return err
	}
	changes := map[string]any{}
	if diff(changes, "dismissed", before, *in.Dismissed) {
		s.auditLog(r, audit.SettingsGeneral, audit.Target{Type: audit.TargetSettings, ID: "onboarding"}, changes)
	}
	return s.getOnboarding(w, r)
}
