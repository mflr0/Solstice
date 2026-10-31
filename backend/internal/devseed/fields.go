package devseed

import (
	"context"
	"database/sql"
	"errors"
)

// seedField is a custom field: one of each type the administration offers.
type seedField struct {
	name, typ string
	options   []string
}

var fields = []seedField{
	{"Customer", "text", nil},
	{"Steps to reproduce", "textarea", nil},
	{"Story points", "number", nil},
	{"Target date", "date", nil},
	{"Deployed at", "datetime", nil},
	{"Severity", "select", []string{"Blocker", "Major", "Minor"}},
	{"Platforms", "multiselect", []string{"Web", "iOS", "Android"}},
	{"Browsers", "checkboxes", []string{"Chrome", "Firefox", "Safari"}},
	{"Impact", "radio", []string{"Low", "Medium", "High"}},
	{"QA owner", "user", nil},
	{"Reviewers", "multiuser", nil},
	{"Teams", "labels", nil},
	{"Spec link", "url", nil},
}

// ensureFields creates the custom fields that are missing (by name), for
// every project and issue type, and returns their ids by name.
func ensureFields(ctx context.Context, conn *sql.DB, admin *client, say func(string, ...any)) (map[string]int64, error) {
	ids := map[string]int64{}
	for _, f := range fields {
		var id int64
		err := conn.QueryRowContext(ctx, `SELECT id FROM custom_fields WHERE name = ?`, f.name).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			var out struct{ ID int64 }
			in := map[string]any{"name": f.name, "type": f.typ}
			if f.options != nil {
				in["options"] = f.options
			}
			if err = admin.call("POST", "/api/admin/fields", in, &out); err == nil {
				say("created the custom field %s", f.name)
				id = out.ID
			}
		}
		if err != nil {
			return nil, err
		}
		ids[f.name] = id
	}
	return ids, nil
}
