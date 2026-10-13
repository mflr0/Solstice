package api

import (
	"database/sql"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"solstice/internal/auth"
)

type adminUser struct {
	ID          int64     `json:"id"`
	Username    string    `json:"username"`
	Name        string    `json:"name"`
	Email       string    `json:"email"`
	Active      bool      `json:"active"`
	Admin       bool      `json:"admin"`
	HasPassword bool      `json:"hasPassword"`
	Created     time.Time `json:"created"`
}

func requireAdmin(r *http.Request) error {
	if !currentUser(r).Admin {
		return errForbidden
	}
	return nil
}

func (s *Server) adminUsers(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	like := "%" + escapeLike(q) + "%"
	out := []adminUser{}
	err := scanAll(r.Context(), s.db, `SELECT id, username, display_name, email, active, is_admin,
		password_hash IS NOT NULL, created_at FROM users
		WHERE username LIKE ? OR LOWER(display_name) LIKE ? OR LOWER(email) LIKE ?
		ORDER BY active DESC, display_name LIMIT 200`, func(rows *sql.Rows) error {
		var u adminUser
		if err := rows.Scan(&u.ID, &u.Username, &u.Name, &u.Email, &u.Active, &u.Admin, &u.HasPassword, &u.Created); err != nil {
			return err
		}
		out = append(out, u)
		return nil
	}, like, like, like)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

var usernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._@-]{0,99}$`)

func (s *Server) adminCreateUser(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	var in struct {
		Username string `json:"username"`
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Admin    bool   `json:"admin"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	in.Username = strings.ToLower(strings.TrimSpace(in.Username))
	if !usernamePattern.MatchString(in.Username) {
		return fail(http.StatusBadRequest, "Identifiant invalide : lettres minuscules, chiffres, . _ @ -")
	}
	if strings.TrimSpace(in.Name) == "" {
		in.Name = in.Username
	}
	if len(in.Password) < 10 {
		return fail(http.StatusBadRequest, "Le mot de passe doit faire au moins 10 caractères.")
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return err
	}
	res, err := s.db.ExecContext(r.Context(), `INSERT INTO users (username, display_name, email, password_hash, is_admin)
		VALUES (?, ?, ?, ?, ?)`, in.Username, strings.TrimSpace(in.Name), strings.TrimSpace(in.Email), hash, in.Admin)
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate") {
			return fail(http.StatusConflict, "Cet identifiant existe déjà.")
		}
		return err
	}
	id, _ := res.LastInsertId()
	return writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

func (s *Server) adminUpdateUser(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return errNotFound
	}
	var in struct {
		Name     *string `json:"name"`
		Email    *string `json:"email"`
		Active   *bool   `json:"active"`
		Admin    *bool   `json:"admin"`
		Password *string `json:"password"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	if id == currentUser(r).ID && ((in.Active != nil && !*in.Active) || (in.Admin != nil && !*in.Admin)) {
		return fail(http.StatusBadRequest, "Vous ne pouvez pas retirer vos propres droits d'administration.")
	}
	ctx := r.Context()
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE id = ?`, id).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return errNotFound
	}
	if in.Name != nil && strings.TrimSpace(*in.Name) != "" {
		if _, err := s.db.ExecContext(ctx, `UPDATE users SET display_name = ? WHERE id = ?`, strings.TrimSpace(*in.Name), id); err != nil {
			return err
		}
	}
	if in.Email != nil {
		if _, err := s.db.ExecContext(ctx, `UPDATE users SET email = ? WHERE id = ?`, strings.TrimSpace(*in.Email), id); err != nil {
			return err
		}
	}
	if in.Active != nil {
		if _, err := s.db.ExecContext(ctx, `UPDATE users SET active = ? WHERE id = ?`, *in.Active, id); err != nil {
			return err
		}
		if !*in.Active {
			if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, id); err != nil {
				return err
			}
		}
	}
	if in.Admin != nil {
		if _, err := s.db.ExecContext(ctx, `UPDATE users SET is_admin = ? WHERE id = ?`, *in.Admin, id); err != nil {
			return err
		}
	}
	if in.Password != nil {
		if len(*in.Password) < 10 {
			return fail(http.StatusBadRequest, "Le mot de passe doit faire au moins 10 caractères.")
		}
		if err := auth.SetPassword(ctx, s.db, id, *in.Password); err != nil {
			return err
		}
		// A new password signs out every session opened with the old one.
		if id != currentUser(r).ID {
			if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, id); err != nil {
				return err
			}
		}
	}
	s.perms.Invalidate()
	w.WriteHeader(http.StatusNoContent)
	return nil
}
