package devseed

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// category is a project category.
type category struct{ name, description string }

var categories = []category{
	{"Products", "What customers use."},
	{"Internal", "The company's own work."},
}

// seedProject is a project of the seed, with its components and versions.
type seedProject struct {
	key, name, description string
	template               string // software or simple (business)
	lead                   string
	category               string // "" for none
	archived               bool
	components             []seedComponent
	versions               []seedVersion
}

type seedComponent struct{ name, lead string }

// seedVersion is a version; release is its release day in days from today.
type seedVersion struct {
	name               string
	release            int
	released, archived bool
}

var projects = []seedProject{
	{Project, "Demo", "Sample project of the development database.", "software", "alice", "Products", false,
		[]seedComponent{{"Backend", "hans"}, {"Frontend", "marie"}, {"Documentation", ""}},
		[]seedVersion{{"0.9", -150, true, true}, {"1.0", -60, true, false}, {"1.1", 30, false, false}}},
	{"MOB", "Mobile app", "The phone application.", "software", "hans", "Products", false,
		[]seedComponent{{"iOS", "giulia"}, {"Android", "priya"}},
		[]seedVersion{{"2.0", -20, true, false}, {"2.1", 45, false, false}}},
	{"OPS", "Office operations", "Purchases, rooms and the office's daily running.", "simple", "marie", "Internal", false,
		[]seedComponent{{"Purchasing", "marie"}, {"Facilities", "tom"}}, nil},
	{"OLD", "Legacy portal", "The customer portal before the rewrite, kept for its history.", "software", "tom", "", true,
		[]seedComponent{{"Web", "tom"}},
		[]seedVersion{{"3.4", -400, true, false}}},
}

// ensureProjects creates the categories and the projects that are missing,
// then the components and versions an active project lacks (by name), then
// archives the projects to archive. Parts are added before the archiving: an
// archived project is read-only.
func ensureProjects(ctx context.Context, conn *sql.DB, admin *client, ids map[string]int64, say func(string, ...any)) error {
	cats := map[string]int64{}
	for _, c := range categories {
		id, err := ensureCategory(ctx, conn, admin, c, say)
		if err != nil {
			return fmt.Errorf("category %s: %w", c.name, err)
		}
		cats[c.name] = id
	}
	for _, p := range projects {
		if err := ensureSeedProject(ctx, conn, admin, p, ids, cats, say); err != nil {
			return fmt.Errorf("project %s: %w", p.key, err)
		}
	}
	return nil
}

func ensureCategory(ctx context.Context, conn *sql.DB, admin *client, c category, say func(string, ...any)) (int64, error) {
	var id int64
	err := conn.QueryRowContext(ctx, `SELECT id FROM project_categories WHERE name = ?`, c.name).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	var out struct{ ID int64 }
	if err := admin.call("POST", "/api/admin/project-categories",
		map[string]any{"name": c.name, "description": c.description}, &out); err != nil {
		return 0, err
	}
	say("created the project category %s", c.name)
	return out.ID, nil
}

func ensureSeedProject(ctx context.Context, conn *sql.DB, admin *client, p seedProject, ids, cats map[string]int64,
	say func(string, ...any)) error {
	var id int64
	var archived bool
	err := conn.QueryRowContext(ctx, `SELECT id, archived FROM projects WHERE pkey = ?`, p.key).Scan(&id, &archived)
	if errors.Is(err, sql.ErrNoRows) {
		in := map[string]any{"key": p.key, "name": p.name, "description": p.description, "template": p.template,
			"lead": ids[p.lead], "access": map[string]any{"holder": "everyone"}}
		if p.category != "" {
			in["category"] = cats[p.category]
		}
		if err := admin.call("POST", "/api/projects", in, nil); err != nil {
			return err
		}
		say("created the project %s", p.key)
		err = conn.QueryRowContext(ctx, `SELECT id FROM projects WHERE pkey = ?`, p.key).Scan(&id)
	}
	if err != nil {
		return err
	}
	if archived {
		return nil
	}
	components, versions := 0, 0
	for _, c := range p.components {
		made, err := ensurePart(ctx, conn, admin, "components", id, p.key, c.name, func() map[string]any {
			in := map[string]any{"name": c.name}
			if c.lead != "" {
				in["lead"] = ids[c.lead]
			}
			return in
		})
		if err != nil {
			return fmt.Errorf("component %s: %w", c.name, err)
		}
		if made {
			components++
		}
	}
	for _, v := range p.versions {
		made, err := ensurePart(ctx, conn, admin, "versions", id, p.key, v.name, func() map[string]any {
			return map[string]any{"name": v.name, "releaseDate": time.Now().AddDate(0, 0, v.release).Format(time.DateOnly),
				"released": v.released, "archived": v.archived}
		})
		if err != nil {
			return fmt.Errorf("version %s: %w", v.name, err)
		}
		if made {
			versions++
		}
	}
	var made []string
	for _, n := range []struct {
		count int
		what  string
	}{{components, "component"}, {versions, "version"}} {
		switch {
		case n.count == 1:
			made = append(made, "1 "+n.what)
		case n.count > 1:
			made = append(made, fmt.Sprintf("%d %ss", n.count, n.what))
		}
	}
	if len(made) > 0 {
		say("created %s in the project %s", strings.Join(made, " and "), p.key)
	}
	if p.archived {
		if err := admin.call("POST", "/api/projects/"+p.key+"/archive", nil, nil); err != nil {
			return err
		}
		say("archived the project %s", p.key)
	}
	return nil
}

// ensurePart creates a component or a version (table) of a project unless
// one has that name.
func ensurePart(ctx context.Context, conn *sql.DB, admin *client, table string, projectID int64, key, name string,
	body func() map[string]any) (bool, error) {
	var n int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table+` WHERE project_id = ? AND name = ?`,
		projectID, name).Scan(&n); err != nil {
		return false, err
	}
	if n > 0 {
		return false, nil
	}
	return true, admin.call("POST", "/api/projects/"+key+"/"+table, body(), nil)
}
