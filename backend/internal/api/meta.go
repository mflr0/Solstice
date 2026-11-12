package api

import (
	"database/sql"
	"net/http"
	"strings"

	"solstice/internal/issue"
	"solstice/internal/sqlq"
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
	match, args := sqlq.LikeWords(strings.ToLower(r.URL.Query().Get("q")), "username", "LOWER(display_name)")
	out := []userRef{}
	err := sqlq.Each(r.Context(), s.db, `SELECT id, username, display_name, active FROM users
		WHERE active AND `+match+` ORDER BY display_name LIMIT 20`,
		func(rows *sql.Rows) error {
			var u userRef
			if err := rows.Scan(&u.ID, &u.Username, &u.Name, &u.Active); err != nil {
				return err
			}
			out = append(out, u)
			return nil
		}, args...)
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
		WHERE `+vis+` AND i.assignee_id = ? AND st.category <> ? ORDER BY i.updated_at DESC LIMIT 50`,
		func(rows *sql.Rows) error {
			iss, err := scanIssue(rows)
			if err != nil {
				return err
			}
			list = append(list, iss)
			ids = append(ids, iss.ReporterID.Int64)
			return nil
		}, append(args, u.ID, issue.Done)...)
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
		asked := in.Users[:n]
		args := make([]any, 0, 3*n)
		for _, u := range asked {
			args = append(args, strings.ToLower(u))
		}
		for _, u := range asked {
			args = append(args, u)
		}
		args = append(args, args[:n]...)
		m := sqlq.Marks(n)
		// rank 0 is a user name, 1 a Jira key, 2 a former user name: the first
		// wins, and the name comes back as it was asked for, lowercased.
		found := map[string]int{}
		err := sqlq.Each(r.Context(), s.db, `SELECT username, 0, display_name FROM users WHERE username IN (`+m+`)
			UNION ALL SELECT user_key, 1, display_name FROM users WHERE user_key IN (`+m+`)
			UNION ALL SELECT a.name, 2, u.display_name FROM user_aliases a JOIN users u ON u.id = a.user_id
				WHERE a.name IN (`+m+`)`,
			func(rows *sql.Rows) error {
				var match, name string
				var rank int
				if err := rows.Scan(&match, &rank, &name); err != nil {
					return err
				}
				for _, u := range asked {
					if u == match || rank != 1 && strings.ToLower(u) == match {
						if prev, ok := found[u]; !ok || rank < prev {
							out.Users[strings.ToLower(u)], found[u] = name, rank
						}
					}
				}
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
