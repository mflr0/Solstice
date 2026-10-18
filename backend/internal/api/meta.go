package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"solstice/internal/i18n"
	"solstice/internal/live"
	"solstice/internal/perm"
	"solstice/internal/sqlq"
	"strings"
	"sync"
	"time"
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
	like := "%" + escapeLike(q) + "%"
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

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
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
	cat, err := s.catalogue(r.Context())
	if err != nil {
		return err
	}
	vis, args := sub.VisibleSQL("i")
	rows, err := s.db.QueryContext(r.Context(), `SELECT `+issueColumns+` FROM issues i
		JOIN projects p ON p.id = i.project_id JOIN statuses st ON st.id = i.status_id
		WHERE `+vis+` AND i.assignee_id = ? AND st.category <> 'done' ORDER BY i.updated_at DESC LIMIT 50`,
		append(args, u.ID)...)
	if err != nil {
		return err
	}
	var list []*issueRow
	ids := []int64{u.ID}
	for rows.Next() {
		iss, err := scanIssue(rows)
		if err != nil {
			rows.Close()
			return err
		}
		list = append(list, iss)
		ids = append(ids, iss.ReporterID.Int64)
	}
	rows.Close()
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

const (
	maxStreamsPerUser = 10
	maxStreamsTotal   = 1000
)

type streamSlots struct {
	mu      sync.Mutex
	perUser map[int64]int
	total   int
}

func (c *streamSlots) acquire(user int64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.perUser == nil {
		c.perUser = map[int64]int{}
	}
	if c.total >= maxStreamsTotal || c.perUser[user] >= maxStreamsPerUser {
		return false
	}
	c.total++
	c.perUser[user]++
	return true
}

func (c *streamSlots) release(user int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.total--
	if c.perUser[user]--; c.perUser[user] == 0 {
		delete(c.perUser, user)
	}
}

// events streams change notifications (Server-Sent Events). Permissions are
// re-read every minute so a revoked access stops the flow.
func (s *Server) events(w http.ResponseWriter, r *http.Request) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fail(http.StatusInternalServerError, i18n.RequestStreamingUnsupported)
	}
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	return s.streamEvents(w, r, flusher, sub)
}

// streamEvents ends when the client goes away, the server shuts down or the
// session behind the stream ends, so its slot is always given back.
func (s *Server) streamEvents(w http.ResponseWriter, r *http.Request, flusher http.Flusher, sub *perm.Subject) error {
	if !s.streams.acquire(sub.UserID) {
		return fail(http.StatusTooManyRequests, i18n.RequestTooManyStreams)
	}
	defer s.streams.release(sub.UserID)
	current := make(chan *perm.Subject, 1)
	current <- sub
	can := func(e live.Event) bool {
		s := <-current
		current <- s
		return s.CanSee(e.Issue) && s.SeesRestricted(e.Issue.ProjectID, e.Group, e.Role)
	}
	ch, unsubscribe := s.hub.Subscribe(can)
	defer unsubscribe()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "retry: 3000\n\n")
	flusher.Flush()

	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()
	refresh := time.NewTicker(time.Minute)
	defer refresh.Stop()
	for {
		select {
		case <-r.Context().Done():
			return nil
		case <-s.streamsDone:
			return nil
		case e := <-ch:
			data, _ := json.Marshal(e)
			fmt.Fprintf(w, "event: change\ndata: %s\n\n", data)
			flusher.Flush()
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case <-refresh.C:
			// End the stream once the session is over or the account disabled.
			u, err := s.authenticate(r)
			if err != nil || u.ID != sub.UserID {
				return nil
			}
			fresh, err := s.perms.Subject(r.Context(), u.ID, u.Admin)
			if err != nil {
				return nil
			}
			<-current
			current <- fresh
		}
	}
}
