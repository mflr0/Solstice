package api

import (
	"database/sql"
	"net/http"

	"solstice/internal/sqlq"
	"strings"
)

func (s *Server) meta(w http.ResponseWriter, r *http.Request) error {
	cat, err := s.catalogue(r.Context())
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, cat)
}

// searchUsers backs user pickers: active users matching a name.
func (s *Server) searchUsers(w http.ResponseWriter, r *http.Request) error {
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	like := "%" + sqlq.EscapeLike(q) + "%"
	out := []userRef{}
	err := sqlq.Each(r.Context(), s.db, `SELECT id, username, display_name, active FROM users
		WHERE active AND (username LIKE ? OR LOWER(display_name) LIKE ?) ORDER BY display_name LIMIT 20`,
		func(rows *sql.Rows) error {
			var u userRef
			if err := rows.Scan(&u.ID, &u.Username, &u.Name, &u.Active); err != nil {
				return err
			}
			out = append(out, u)
			return nil
		}, like, like)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

// person is someone's page: who they are and what they are working on.
func (s *Server) person(w http.ResponseWriter, r *http.Request) error {
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	var u userRef
	var email string
	err = s.db.QueryRowContext(r.Context(), `SELECT id, username, display_name, active, email FROM users
		WHERE username = LOWER(?)`, r.PathValue("username")).Scan(&u.ID, &u.Username, &u.Name, &u.Active, &email)
	if err == sql.ErrNoRows {
		return errNotFound
	}
	if err != nil {
		return err
	}
	// The address is private: only an administrator and the person see it.
	if me := currentUser(r); !me.Admin && me.ID != u.ID {
		email = ""
	}
	cat, err := s.catalogue(r.Context())
	if err != nil {
		return err
	}
	vis, args := sub.VisibleSQL("i")
	var list []*issueRow
	ids := []int64{u.ID}
	err = sqlq.Each(r.Context(), s.db, `SELECT `+issueColumns+` FROM issues i
		JOIN projects p ON p.id = i.project_id JOIN statuses st ON st.id = i.status_id
		WHERE `+vis+` AND i.assignee_id = ? AND st.category <> 'done' ORDER BY i.updated_at DESC LIMIT 50`,
		func(rows *sql.Rows) error {
			iss, err := scanIssue(rows)
			if err != nil {
				return err
			}
			list = append(list, iss)
			ids = append(ids, iss.ReporterID.Int64)
			return nil
		}, append(args, u.ID)...)
	if err != nil {
		return err
	}
	users, err := s.users(r.Context(), ids...)
	if err != nil {
		return err
	}
	assigned := make([]issueSummary, 0, len(list))
	for _, iss := range list {
		assigned = append(assigned, summarize(iss, cat, users))
	}
	return writeJSON(w, http.StatusOK, map[string]any{"user": u, "email": email, "assigned": assigned})
}

// refs resolves what wiki text refers to, so the interface can render
// mentions and issue keys: display names, and which keys the user may open.
func (s *Server) refs(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Users     []string `json:"users"`
		IssueKeys []string `json:"issueKeys"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	out := struct {
		Users  map[string]string `json:"users"`
		Issues map[string]bool   `json:"issues"`
	}{map[string]string{}, map[string]bool{}}

	if n := min(len(in.Users), 200); n > 0 {
		args := make([]any, n)
		for i, u := range in.Users[:n] {
			args[i] = strings.ToLower(u)
		}
		err := sqlq.Each(r.Context(), s.db, `SELECT username, display_name FROM users WHERE username IN (`+sqlq.Marks(n)+`)`,
			func(rows *sql.Rows) error {
				var username, name string
				if err := rows.Scan(&username, &name); err != nil {
					return err
				}
				out.Users[username] = name
				return nil
			}, args...)
		if err != nil {
			return err
		}
	}
	for _, key := range in.IssueKeys[:min(len(in.IssueKeys), 200)] {
		iss, err := s.findIssue(r.Context(), key)
		if err == nil && sub.CanSee(iss.perm()) {
			out.Issues[strings.ToUpper(key)] = true
		}
	}
	return writeJSON(w, http.StatusOK, out)
}
