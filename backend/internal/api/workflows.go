package api

import (
	"context"
	"database/sql"
	"errors"
	"slices"
)

// transitionOut is a move an issue can make from its current status.
type transitionOut struct {
	ID   int64     `json:"id"`
	Name string    `json:"name"`
	To   statusRef `json:"to"`
}

// workflowOf returns the workflow an issue type follows in a project, and the
// status new issues start in. ok is false for a project without workflows.
func (s *Server) workflowOf(ctx context.Context, projectID int64, typeID string) (id int64, initial string, ok bool, err error) {
	err = s.db.QueryRowContext(ctx, `SELECT w.id, w.initial_status_id FROM workflows w
		WHERE w.id = COALESCE(
			(SELECT workflow_id FROM workflow_issue_types WHERE project_id = ? AND issue_type_id = ?),
			(SELECT id FROM workflows WHERE project_id = ? AND is_default LIMIT 1))`,
		projectID, typeID, projectID).Scan(&id, &initial)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, "", false, nil
	}
	return id, initial, err == nil, err
}

func (s *Server) initialStatusFor(ctx context.Context, projectID int64, typeID string) (string, error) {
	if _, initial, ok, err := s.workflowOf(ctx, projectID, typeID); err != nil || ok {
		return initial, err
	}
	return s.initialStatus(ctx, projectID)
}

// issueTransitions lists the moves available from the issue's status, and the
// statuses they lead to (the current one first). An issue whose status is not
// part of its workflow — data drift from Jira — may be put back on any of the
// workflow's statuses.
func (s *Server) issueTransitions(ctx context.Context, cat *catalog, iss *issueRow) ([]transitionOut, []statusRef, error) {
	statuses := []statusRef{cat.statusOf(iss.StatusID)}
	add := func(st statusRef) {
		if !slices.ContainsFunc(statuses, func(x statusRef) bool { return x.ID == st.ID }) {
			statuses = append(statuses, st)
		}
	}
	wf, _, ok, err := s.workflowOf(ctx, iss.ProjectID, iss.TypeID)
	if err != nil {
		return nil, nil, err
	}
	out := []transitionOut{}
	if !ok {
		// No workflow (a project created before workflows existed): any status
		// already used in the project is allowed.
		used, err := s.projectStatuses(ctx, cat, iss.ProjectID, iss.StatusID)
		if err != nil {
			return nil, nil, err
		}
		for _, st := range used {
			if st.ID != iss.StatusID {
				out = append(out, transitionOut{Name: st.Name, To: st})
				add(st)
			}
		}
		return out, statuses, nil
	}

	var inWorkflow int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM workflow_statuses WHERE workflow_id = ? AND status_id = ?`,
		wf, iss.StatusID).Scan(&inWorkflow); err != nil {
		return nil, nil, err
	}
	if inWorkflow == 0 {
		err := scanAll(ctx, s.db, `SELECT status_id FROM workflow_statuses WHERE workflow_id = ? ORDER BY position`,
			func(rows *sql.Rows) error {
				var id string
				if err := rows.Scan(&id); err != nil {
					return err
				}
				st := cat.statusOf(id)
				out = append(out, transitionOut{Name: st.Name, To: st})
				add(st)
				return nil
			}, wf)
		return out, statuses, err
	}
	err = scanAll(ctx, s.db, `SELECT id, name, to_status_id FROM workflow_transitions
		WHERE workflow_id = ? AND (from_status_id = ? OR from_status_id IS NULL) AND to_status_id <> ?
		ORDER BY from_status_id IS NULL, position, id`, func(rows *sql.Rows) error {
		var t transitionOut
		var to string
		if err := rows.Scan(&t.ID, &t.Name, &to); err != nil {
			return err
		}
		t.To = cat.statusOf(to)
		out = append(out, t)
		add(t.To)
		return nil
	}, wf, iss.StatusID, iss.StatusID)
	return out, statuses, err
}
