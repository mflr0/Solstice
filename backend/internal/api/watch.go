package api

import (
	"net/http"
	"time"
)

func (s *Server) setWatch(on bool) handler {
	return s.toggle("watchers", on)
}

func (s *Server) setVote(on bool) handler {
	return s.toggle("votes", on)
}

func (s *Server) toggle(table string, on bool) handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		sub, err := s.subject(r)
		if err != nil {
			return err
		}
		iss, err := s.visibleIssue(r, sub)
		if err != nil {
			return err
		}
		if on {
			_, err = s.db.ExecContext(r.Context(), `INSERT IGNORE INTO `+table+` (issue_id, user_id, created_at)
				VALUES (?, ?, ?)`, iss.ID, sub.UserID, time.Now().UTC())
		} else {
			_, err = s.db.ExecContext(r.Context(), `DELETE FROM `+table+` WHERE issue_id = ? AND user_id = ?`,
				iss.ID, sub.UserID)
		}
		if err != nil {
			return err
		}
		var c counter
		if err := s.db.QueryRowContext(r.Context(), `SELECT COUNT(*), COALESCE(SUM(user_id = ?), 0) FROM `+table+`
			WHERE issue_id = ?`, sub.UserID, iss.ID).Scan(&c.Count, &c.Me); err != nil {
			return err
		}
		return writeJSON(w, http.StatusOK, c)
	}
}
