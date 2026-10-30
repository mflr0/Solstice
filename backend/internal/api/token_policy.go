package api

// Who may create personal access tokens: the administrators only (the
// default), everyone, or the members of one group. Administrators always
// may. Tokens already created keep working whatever the setting says; they
// can be revoked from the administration.

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"solstice/internal/audit"
	"solstice/internal/auth"
	"solstice/internal/i18n"
)

const tokenCreatorsSetting = "tokens.creators"

const (
	creatorsEveryone = "everyone"
	creatorsAdmins   = "admins"
	creatorsGroup    = "group"
)

// tokenPolicy is the stored policy: who, and for "group" which group.
type tokenPolicy struct {
	Who   string `json:"who"`
	Group int64  `json:"group,omitempty"`
}

// tokenPolicyOut is the policy as the administration reads it, with the
// lifetime of OAuth refresh tokens (oauth.go) and whether OAuth clients may
// register (oauth_register.go).
type tokenPolicyOut struct {
	Who          string  `json:"who"`
	Group        *idName `json:"group"` // for "group", while the group exists
	RefreshDays  int     `json:"refreshDays"`
	Registration bool    `json:"registration"`
}

// tokenPolicy reads the policy; a missing or unreadable one means
// administrators only.
func (s *Server) tokenPolicy(ctx context.Context) tokenPolicy {
	p := tokenPolicy{Who: creatorsAdmins}
	if _, err := s.out.settings.GetJSON(ctx, tokenCreatorsSetting, &p); err != nil {
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			s.log.Warn("reading the token policy failed, administrators only", "err", err)
		}
		return tokenPolicy{Who: creatorsAdmins}
	}
	switch p.Who {
	case creatorsEveryone, creatorsAdmins, creatorsGroup:
		return p
	}
	return tokenPolicy{Who: creatorsAdmins}
}

// mayCreateTokens tells whether a user may create a personal access token.
// A policy naming a group that no longer exists lets administrators only.
func (s *Server) mayCreateTokens(ctx context.Context, u *auth.User) (bool, error) {
	if u.Admin {
		return true, nil
	}
	switch p := s.tokenPolicy(ctx); p.Who {
	case creatorsEveryone:
		return true, nil
	case creatorsGroup:
		var n int
		err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM group_members WHERE group_id = ? AND user_id = ?`,
			p.Group, u.ID).Scan(&n)
		return n > 0, err
	}
	return false, nil
}

// myTokenPolicy tells the interface whether the current user may create a
// token, so that it can say so instead of offering a form that fails.
func (s *Server) myTokenPolicy(w http.ResponseWriter, r *http.Request) error {
	u := currentUser(r)
	ok, err := s.mayCreateTokens(r.Context(), u)
	if err != nil {
		return err
	}
	// A token cannot make tokens, whatever the policy.
	return writeJSON(w, http.StatusOK, map[string]bool{"canCreate": ok && !u.ByToken()})
}

// groupRef names a group, nil when it is gone.
func (s *Server) groupRef(ctx context.Context, id int64) (*idName, error) {
	var g idName
	err := s.db.QueryRowContext(ctx, `SELECT id, name FROM user_groups WHERE id = ?`, id).Scan(&g.ID, &g.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &g, err
}

func (s *Server) policyOut(ctx context.Context, p tokenPolicy) (tokenPolicyOut, error) {
	out := tokenPolicyOut{Who: p.Who}
	if p.Who == creatorsGroup {
		g, err := s.groupRef(ctx, p.Group)
		if err != nil {
			return out, err
		}
		out.Group = g
	}
	return out, nil
}

func (s *Server) adminTokenSettings(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	out, err := s.policyOut(r.Context(), s.tokenPolicy(r.Context()))
	if err != nil {
		return err
	}
	out.RefreshDays, out.Registration = s.refreshDays(r.Context()), s.registrationOn(r.Context())
	return writeJSON(w, http.StatusOK, out)
}

// putTokenSettings changes who may create tokens: {"who": "everyone" |
// "admins" | "group", "group": <id, for "group">, "refreshDays": <days an
// unused OAuth refresh token lasts, optional>, "registration": <whether
// OAuth clients may register, optional>}.
func (s *Server) putTokenSettings(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	var body struct {
		tokenPolicy
		RefreshDays  *int  `json:"refreshDays"`
		Registration *bool `json:"registration"`
	}
	if err := readJSON(r, &body); err != nil {
		return err
	}
	in := body.tokenPolicy
	if d := body.RefreshDays; d != nil && (*d < 1 || *d > maxRefreshDays) {
		return failf(http.StatusBadRequest, i18n.OAuthRefreshDaysInvalid, maxRefreshDays)
	}
	ctx := r.Context()
	switch in.Who {
	case creatorsEveryone, creatorsAdmins:
		in.Group = 0
	case creatorsGroup:
		if in.Group <= 0 {
			return fail(http.StatusBadRequest, i18n.TokenPolicyInvalid)
		}
		if err := s.checkGroups(ctx, []int64{in.Group}); err != nil {
			return err
		}
	default:
		return fail(http.StatusBadRequest, i18n.TokenPolicyInvalid)
	}
	before, beforeDays, beforeReg := s.tokenPolicy(ctx), s.refreshDays(ctx), s.registrationOn(ctx)
	if err := s.out.settings.SetJSON(ctx, tokenCreatorsSetting, in); err != nil {
		return err
	}
	details := map[string]any{"who": map[string]any{"from": before.Who, "to": in.Who},
		"group": s.groupNames(ctx, nonZero(in.Group))}
	if d := body.RefreshDays; d != nil && *d != beforeDays {
		if err := s.out.settings.SetJSON(ctx, oauthRefreshSetting, *d); err != nil {
			return err
		}
		details["refreshDays"] = map[string]any{"from": beforeDays, "to": *d}
	}
	if on := body.Registration; on != nil && *on != beforeReg {
		if err := s.out.settings.SetJSON(ctx, oauthRegistrationSetting, *on); err != nil {
			return err
		}
		details["registration"] = map[string]any{"from": beforeReg, "to": *on}
	}
	s.auditLog(r, audit.SettingsTokens, audit.Target{Type: audit.TargetSettings, ID: tokenCreatorsSetting}, details)
	out, err := s.policyOut(ctx, in)
	if err != nil {
		return err
	}
	out.RefreshDays, out.Registration = s.refreshDays(ctx), s.registrationOn(ctx)
	return writeJSON(w, http.StatusOK, out)
}

func nonZero(ids ...int64) []int64 {
	var out []int64
	for _, id := range ids {
		if id != 0 {
			out = append(out, id)
		}
	}
	return out
}
