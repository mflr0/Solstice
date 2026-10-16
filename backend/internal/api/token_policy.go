package api

// Who may create personal access tokens: everyone (the default, as before
// the setting existed), the administrators only, or the members of one
// group. Administrators always may. Tokens already created keep working
// whatever the setting says; they can be revoked from the administration.

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"solstice/internal/audit"
	"solstice/internal/auth"
	"solstice/internal/i18n"
)

// tokenCreatorsSetting is the name of the setting that holds the policy.
const tokenCreatorsSetting = "tokens.creators"

// Who may create tokens.
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

// tokenPolicyOut is the policy as the administration reads it.
type tokenPolicyOut struct {
	Who   string  `json:"who"`
	Group *idName `json:"group"` // for "group", while the group exists
}

// tokenPolicy reads the policy; a missing or unreadable one means everyone.
func (s *Server) tokenPolicy(ctx context.Context) (tokenPolicy, error) {
	p := tokenPolicy{Who: creatorsEveryone}
	if _, err := s.out.settings.GetJSON(ctx, tokenCreatorsSetting, &p); err != nil {
		return tokenPolicy{Who: creatorsEveryone}, nil
	}
	switch p.Who {
	case creatorsEveryone, creatorsAdmins, creatorsGroup:
		return p, nil
	}
	return tokenPolicy{Who: creatorsEveryone}, nil
}

// mayCreateTokens tells whether a user may create a personal access token.
// A policy naming a group that no longer exists lets administrators only.
func (s *Server) mayCreateTokens(ctx context.Context, u *auth.User) (bool, error) {
	if u.Admin {
		return true, nil
	}
	p, err := s.tokenPolicy(ctx)
	if err != nil {
		return false, err
	}
	switch p.Who {
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
	return writeJSON(w, http.StatusOK, map[string]bool{"canCreate": ok && u.TokenID == 0})
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
	p, err := s.tokenPolicy(r.Context())
	if err != nil {
		return err
	}
	out, err := s.policyOut(r.Context(), p)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

// putTokenSettings changes who may create tokens: {"who": "everyone" |
// "admins" | "group", "group": <id, for "group">}.
func (s *Server) putTokenSettings(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	var in tokenPolicy
	if err := readJSON(r, &in); err != nil {
		return err
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
	before, err := s.tokenPolicy(ctx)
	if err != nil {
		return err
	}
	if err := s.out.settings.SetJSON(ctx, tokenCreatorsSetting, in); err != nil {
		return err
	}
	s.auditLog(r, audit.SettingsTokens, audit.Target{Type: "settings", ID: tokenCreatorsSetting},
		map[string]any{"who": map[string]any{"from": before.Who, "to": in.Who},
			"group": s.groupNames(ctx, nonZero(in.Group))})
	out, err := s.policyOut(ctx, in)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

// nonZero lists the ids that are set.
func nonZero(ids ...int64) []int64 {
	var out []int64
	for _, id := range ids {
		if id != 0 {
			out = append(out, id)
		}
	}
	return out
}
