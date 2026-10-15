package events

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"

	"solstice/internal/perm"
)

// Named is a catalogue value: an id and its name.
type Named struct {
	ID   string
	Name string
}

// IssueType is an issue's type.
type IssueType struct {
	Named
	Subtask bool
}

// Status is an issue's status and its category (todo, doing, done).
type Status struct {
	Named
	Category string
}

// User is a person as consumers need them.
type User struct {
	ID       int64
	Username string
	Key      string // Jira's user key (the user name for accounts made in Solstice)
	Name     string
	Email    string
	Locale   string
	Active   bool
}

// Ref names another issue.
type Ref struct {
	ID  int64
	Key string
}

// Issue is an issue's state with names resolved.
type Issue struct {
	ID          int64
	Key         string
	ProjectID   int64
	ProjectKey  string
	ProjectName string
	Summary     string
	Description *string
	Environment *string
	Type        IssueType
	Status      Status
	Priority    *Named
	Resolution  *Named
	Reporter    *User
	Assignee    *User
	Creator     *User
	Security    *Named
	Created     time.Time
	Updated     time.Time
	Resolved    *time.Time
	Due         *string // YYYY-MM-DD
	Original    *int64  // estimates and time spent, in seconds
	Remaining   *int64
	Spent       *int64
	Labels      []string
	Components  []Named
	FixVersions []Named
	Versions    []Named // affects versions
	Parent      *Ref
	Watchers    []int64
	Access      perm.Issue
}

// LoadIssue reads an issue's current state; sql.ErrNoRows if it is gone.
func LoadIssue(ctx context.Context, db *sql.DB, id int64) (*Issue, error) {
	var iss Issue
	var desc, env, priID, priName, resID, resName, parentKey sql.NullString
	var typeID, typeName, statusID, statusName, statusCat sql.NullString
	var subtask sql.NullBool
	var reporter, assignee, creator, levelID, parentID sql.NullInt64
	var levelName sql.NullString
	var resolved, due sql.NullTime
	var original, remaining, spent sql.NullInt64
	var num int64
	err := db.QueryRowContext(ctx, `SELECT i.id, p.id, p.pkey, p.name, i.num, i.summary, i.description, i.environment,
		i.issue_type_id, t.name, t.subtask, i.status_id, st.name, st.category, pr.id, pr.name, rs.id, rs.name,
		i.reporter_id, i.assignee_id, i.creator_id, sl.id, sl.name, i.created_at, i.updated_at, i.resolved_at, i.due_date,
		i.original_estimate, i.remaining_estimate, i.time_spent, par.id, CONCAT(pp.pkey, '-', par.num)
		FROM issues i JOIN projects p ON p.id = i.project_id
		LEFT JOIN issue_types t ON t.id = i.issue_type_id
		LEFT JOIN statuses st ON st.id = i.status_id
		LEFT JOIN priorities pr ON pr.id = i.priority_id
		LEFT JOIN resolutions rs ON rs.id = i.resolution_id
		LEFT JOIN security_levels sl ON sl.id = i.security_level_id
		LEFT JOIN issues par ON par.id = i.parent_id
		LEFT JOIN projects pp ON pp.id = par.project_id
		WHERE i.id = ?`, id).Scan(&iss.ID, &iss.ProjectID, &iss.ProjectKey, &iss.ProjectName, &num, &iss.Summary,
		&desc, &env, &typeID, &typeName, &subtask, &statusID, &statusName, &statusCat, &priID, &priName, &resID,
		&resName, &reporter, &assignee, &creator, &levelID, &levelName, &iss.Created, &iss.Updated, &resolved, &due,
		&original, &remaining, &spent, &parentID, &parentKey)
	if err != nil {
		return nil, err
	}
	iss.Key = iss.ProjectKey + "-" + strconv.FormatInt(num, 10)
	iss.Description, iss.Environment = optString(desc), optString(env)
	iss.Type = IssueType{Named: Named{typeID.String, orID(typeName, typeID)}, Subtask: subtask.Bool}
	iss.Status = Status{Named: Named{statusID.String, orID(statusName, statusID)}, Category: statusCat.String}
	if iss.Status.Category == "" {
		iss.Status.Category = "todo"
	}
	if priID.Valid {
		iss.Priority = &Named{priID.String, priName.String}
	}
	if resID.Valid {
		iss.Resolution = &Named{resID.String, resName.String}
	}
	if levelID.Valid {
		iss.Security = &Named{strconv.FormatInt(levelID.Int64, 10), levelName.String}
	}
	if resolved.Valid {
		t := resolved.Time
		iss.Resolved = &t
	}
	if due.Valid {
		d := due.Time.Format(time.DateOnly)
		iss.Due = &d
	}
	iss.Original, iss.Remaining, iss.Spent = optInt(original), optInt(remaining), optInt(spent)
	if parentID.Valid {
		iss.Parent = &Ref{ID: parentID.Int64, Key: parentKey.String}
	}
	iss.Access = perm.Issue{ProjectID: iss.ProjectID, ReporterID: reporter.Int64, AssigneeID: assignee.Int64,
		SecurityLevel: levelID.Int64}

	users, err := LoadUsers(ctx, db, reporter.Int64, assignee.Int64, creator.Int64)
	if err != nil {
		return nil, err
	}
	iss.Reporter, iss.Assignee, iss.Creator = userOf(users, reporter), userOf(users, assignee), userOf(users, creator)

	if err := scan(ctx, db, `SELECT label FROM labels WHERE issue_id = ? AND field_id IS NULL ORDER BY label`,
		func(rows *sql.Rows) error {
			var l string
			err := rows.Scan(&l)
			iss.Labels = append(iss.Labels, l)
			return err
		}, id); err != nil {
		return nil, err
	}
	if err := scan(ctx, db, `SELECT c.id, c.name FROM issue_components ic JOIN components c ON c.id = ic.component_id
		WHERE ic.issue_id = ? ORDER BY c.name`, func(rows *sql.Rows) error {
		var cid int64
		var name string
		err := rows.Scan(&cid, &name)
		iss.Components = append(iss.Components, Named{strconv.FormatInt(cid, 10), name})
		return err
	}, id); err != nil {
		return nil, err
	}
	if err := scan(ctx, db, `SELECT v.id, v.name, iv.kind FROM issue_versions iv JOIN versions v ON v.id = iv.version_id
		WHERE iv.issue_id = ? ORDER BY v.position, v.name`, func(rows *sql.Rows) error {
		var vid int64
		var name, kind string
		if err := rows.Scan(&vid, &name, &kind); err != nil {
			return err
		}
		v := Named{strconv.FormatInt(vid, 10), name}
		if kind == "fix" {
			iss.FixVersions = append(iss.FixVersions, v)
		} else {
			iss.Versions = append(iss.Versions, v)
		}
		return nil
	}, id); err != nil {
		return nil, err
	}
	if err := scan(ctx, db, `SELECT user_id FROM watchers WHERE issue_id = ? ORDER BY user_id`, func(rows *sql.Rows) error {
		var uid int64
		err := rows.Scan(&uid)
		iss.Watchers = append(iss.Watchers, uid)
		return err
	}, id); err != nil {
		return nil, err
	}
	return &iss, nil
}

// LoadUsers reads people by id (zero ids are ignored).
func LoadUsers(ctx context.Context, db *sql.DB, ids ...int64) (map[int64]*User, error) {
	out := map[int64]*User{}
	var args []any
	seen := map[int64]bool{}
	for _, id := range ids {
		if id != 0 && !seen[id] {
			seen[id] = true
			args = append(args, id)
		}
	}
	if len(args) == 0 {
		return out, nil
	}
	err := scan(ctx, db, `SELECT id, username, COALESCE(user_key, username), display_name, email, COALESCE(locale, ''),
		active FROM users WHERE id IN (`+strings.TrimSuffix(strings.Repeat("?, ", len(args)), ", ")+`)`,
		func(rows *sql.Rows) error {
			var u User
			if err := rows.Scan(&u.ID, &u.Username, &u.Key, &u.Name, &u.Email, &u.Locale, &u.Active); err != nil {
				return err
			}
			out[u.ID] = &u
			return nil
		}, args...)
	return out, err
}

func userOf(users map[int64]*User, id sql.NullInt64) *User {
	if !id.Valid {
		return nil
	}
	return users[id.Int64]
}

func scan(ctx context.Context, db *sql.DB, q string, f func(*sql.Rows) error, args ...any) error {
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := f(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

func optString(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}
	return &s.String
}

func optInt(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	return &n.Int64
}

func orID(name, id sql.NullString) string {
	if name.Valid {
		return name.String
	}
	return id.String
}
