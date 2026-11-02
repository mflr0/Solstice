package devseed

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"solstice/internal/i18n"
	"solstice/internal/issue"
	"solstice/internal/webhook"
)

// customWorkflow is a project workflow for some issue types: a copy of the
// project's default with a custom status, reached by a transition from any
// status.
var customWorkflow = struct {
	project, name string
	status        string
	category      issue.StatusCategory
	transition    string
	types         []i18n.Key
}{"OPS", "Purchases", "Awaiting approval", issue.Doing, "Ask for approval", []i18n.Key{i18n.DefaultTypeTask}}

// ensureWorkflow creates the custom status and the custom workflow unless
// they exist, and gives the workflow the status if it lacks it.
func ensureWorkflow(ctx context.Context, conn *sql.DB, admin *client, say func(string, ...any)) error {
	wf := customWorkflow
	var status string
	err := conn.QueryRowContext(ctx, `SELECT id FROM statuses WHERE name = ?`, wf.status).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		var out struct{ ID string }
		if err = admin.call("POST", "/api/statuses", map[string]any{"name": wf.status, "category": wf.category,
			"project": wf.project}, &out); err == nil {
			say("created the status %s", wf.status)
			status = out.ID
		}
	}
	if err != nil {
		return err
	}
	path := "/api/projects/" + wf.project + "/workflows"
	var id int64
	err = conn.QueryRowContext(ctx, `SELECT w.id FROM workflows w JOIN projects p ON p.id = w.project_id
		WHERE p.pkey = ? AND w.name = ?`, wf.project, wf.name).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		var out struct{ ID int64 }
		if err = admin.call("POST", path, map[string]any{"name": wf.name}, &out); err == nil {
			say("created the workflow %s in the project %s", wf.name, wf.project)
			id = out.ID
		}
	}
	if err != nil {
		return err
	}
	var n int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM workflow_statuses WHERE workflow_id = ? AND status_id = ?`,
		id, status).Scan(&n); err != nil || n > 0 {
		return err
	}

	type ref struct{ ID string }
	var list struct {
		Workflows []struct {
			ID          int64
			Initial     ref
			Statuses    []ref
			Transitions []struct {
				ID       int64
				Name     string
				From, To *ref
			}
		}
	}
	if err := admin.call("GET", path, nil, &list); err != nil {
		return err
	}
	cat, err := readCatalogue(admin)
	if err != nil {
		return err
	}
	for _, w := range list.Workflows {
		if w.ID != id {
			continue
		}
		in := map[string]any{"name": wf.name, "initial": w.Initial.ID}
		var statuses, types []string
		var transitions []map[string]any
		for _, s := range w.Statuses {
			statuses = append(statuses, s.ID)
		}
		for _, t := range w.Transitions {
			tr := map[string]any{"id": t.ID, "name": t.Name, "to": t.To.ID}
			if t.From != nil {
				tr["from"] = t.From.ID
			}
			transitions = append(transitions, tr)
		}
		for _, k := range wf.types {
			typ, err := lookup(cat.types, k)
			if err != nil {
				return err
			}
			types = append(types, typ)
		}
		in["statuses"] = append(statuses, status)
		in["transitions"] = append(transitions, map[string]any{"name": wf.transition, "to": status})
		in["issueTypes"] = types
		if err := admin.call("PUT", fmt.Sprintf("/api/workflows/%d", id), in, nil); err != nil {
			return err
		}
		say("added the status %s to the workflow %s", wf.status, wf.name)
		return nil
	}
	return fmt.Errorf("the project %s has no workflow %d", wf.project, id)
}

// deadHook is a webhook to a local port nothing listens on, so that its
// deliveries fail.
var deadHook = struct{ name, url string }{"Local listener (not running)", "http://127.0.0.1:9/solstice"}

func ensureWebhook(ctx context.Context, conn *sql.DB, admin *client, say func(string, ...any)) error {
	var n int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM webhooks WHERE name = ?`, deadHook.name).Scan(&n); err != nil ||
		n > 0 {
		return err
	}
	if err := admin.call("POST", "/api/admin/webhooks", map[string]any{"name": deadHook.name, "url": deadHook.url,
		"events": []string{webhook.IssueCreated, webhook.CommentCreated}}, nil); err != nil {
		return err
	}
	say("created the webhook %s", deadHook.name)
	return nil
}
