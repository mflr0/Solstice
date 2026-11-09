package importer

import (
	"database/sql"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"solstice/internal/issue"
	"solstice/internal/sqlq"
)

// importFilters copies saved searches with their shares, and favourites as
// views of My space. JQL
// is kept as written: Solstice understands it. A share Solstice cannot express is
// dropped, so a filter is never shown to more people than in Jira.
func (r *run) importFilters() error {
	filters := map[int64]bool{}
	r.filters = map[int64]string{}
	ins := r.inserter("filters", "id", "owner_id", "name", "description", "query", "created_at", "updated_at")
	now := time.Now().UTC()
	if err := r.each(`SELECT id, filtername, description, username, authorname, reqcontent FROM searchrequest`, func(rows *sql.Rows) error {
		var id int64
		var name, desc, owner, author, query sql.NullString
		if err := rows.Scan(&id, &name, &desc, &owner, &author, &query); err != nil {
			return err
		}
		if !owner.Valid || owner.String == "" {
			owner = author
		}
		ownerID, err := r.user(owner)
		if err != nil {
			return err
		}
		filters[id] = true
		r.filters[id] = query.String
		return ins.add(id, ownerID, truncate(orDefault(name, "Filter "+strconv.FormatInt(id, 10)), sqlq.VarcharWidth), sqlq.Value(desc),
			query.String, now, now)
	}); err != nil {
		return err
	}
	if err := ins.flush(); err != nil {
		return err
	}
	r.rep.step("saved filters", ins.total, 0)

	shares := r.inserter("filter_shares", "filter_id", "kind", "group_id", "project_id", "user_id")
	skipped := 0
	if err := r.each(`SELECT entityid, sharetype, param1, param2 FROM sharepermissions
		WHERE entitytype = 'SearchRequest'`, func(rows *sql.Rows) error {
		var filter sql.NullInt64
		var kind, p1, p2 sql.NullString
		if err := rows.Scan(&filter, &kind, &p1, &p2); err != nil {
			return err
		}
		if !filters[filter.Int64] {
			skipped++
			return nil
		}
		switch kind.String {
		case "global", "loggedin":
			return shares.add(filter.Int64, issue.ShareEveryone, nil, nil, nil)
		case "group":
			if gid, ok := r.groups[strings.ToLower(p1.String)]; ok {
				return shares.add(filter.Int64, issue.ShareGroup, gid, nil, nil)
			}
		case "project":
			pid, err := strconv.ParseInt(p1.String, 10, 64)
			if err == nil && r.projects[pid] != nil {
				// A share with a project role goes to that role's group in the project.
				var group any
				if p2.String != "" {
					rid, err := strconv.ParseInt(p2.String, 10, 64)
					rg, ok := r.roles[rid]
					if err != nil || !ok {
						skipped++
						return nil
					}
					group = rg.id
				}
				return shares.add(filter.Int64, issue.ShareProject, group, pid, nil)
			}
		case "user":
			if uid, ok := r.users[p1.String]; ok {
				return shares.add(filter.Int64, issue.ShareUser, nil, nil, uid)
			}
		}
		skipped++
		return nil
	}); err != nil {
		return err
	}
	if err := shares.flush(); err != nil {
		return err
	}
	r.rep.step("filter shares", shares.total, skipped)

	return r.importFavourites(filters)
}

// The views of My space, as the API's defaultDashboard and maxGadgets have them.
var builtinGadgets = []struct{ kind, config string }{
	{"assigned", `{"size":15}`}, {"reported", `{"size":15}`}, {"recent", `{"size":12}`},
}

const maxGadgets = 20

// importFavourites turns each user's favourite filters into views of their
// My space, after the built-in ones and in Jira's order. Favourites beyond
// maxGadgets are left out.
func (r *run) importFavourites(filters map[int64]bool) error {
	favs := map[int64][]int64{}
	seen := map[[2]int64]bool{}
	skipped := 0
	if err := r.each(`SELECT username, entityid FROM favouriteassociations WHERE entitytype = 'SearchRequest'
		ORDER BY sequence, id`, func(rows *sql.Rows) error {
		var user sql.NullString
		var filter sql.NullInt64
		if err := rows.Scan(&user, &filter); err != nil {
			return err
		}
		uid, ok := r.users[user.String]
		k := [2]int64{filter.Int64, uid}
		if !ok || !filters[filter.Int64] || seen[k] {
			skipped++
			return nil
		}
		seen[k] = true
		favs[uid] = append(favs[uid], filter.Int64)
		return nil
	}); err != nil {
		return err
	}
	now := time.Now().UTC()
	dashboards := r.inserter("dashboards", "user_id", "updated_at")
	gadgets := r.inserter("dashboard_gadgets", "id", "user_id", "position", "kind", "filter_id", "config")
	var id int64
	views := 0
	for _, uid := range slices.Sorted(maps.Keys(favs)) {
		if err := dashboards.add(uid, now); err != nil {
			return err
		}
		for i, g := range builtinGadgets {
			id++
			if err := gadgets.add(id, uid, i, g.kind, nil, g.config); err != nil {
				return err
			}
		}
		fit := favs[uid][:min(len(favs[uid]), maxGadgets-len(builtinGadgets))]
		skipped += len(favs[uid]) - len(fit)
		for i, f := range fit {
			id++
			if err := gadgets.add(id, uid, len(builtinGadgets)+i, "results", f, "{}"); err != nil {
				return err
			}
		}
		views += len(fit)
	}
	if err := dashboards.flush(); err != nil {
		return err
	}
	if err := gadgets.flush(); err != nil {
		return err
	}
	r.rep.step("favorite filters", views, skipped)
	return nil
}
