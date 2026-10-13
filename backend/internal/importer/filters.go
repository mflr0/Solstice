package importer

import (
	"database/sql"
	"strconv"
	"strings"
	"time"
)

// importFilters copies saved searches with their shares and favourites. JQL
// is kept as written: Solstice understands it. A share Solstice cannot express is
// dropped, so a filter is never shown to more people than in Jira.
func (r *run) importFilters() error {
	filters := map[int64]bool{}
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
		return ins.add(id, ownerID, truncate(orDefault(name, "Filtre "+strconv.FormatInt(id, 10)), 255), nstr(desc),
			query.String, now, now)
	}); err != nil {
		return err
	}
	if err := ins.flush(); err != nil {
		return err
	}
	r.rep.step("filtres enregistrés", ins.total, 0)

	shares := r.inserter("filter_shares", "filter_id", "kind", "group_id", "project_id", "role_id", "user_id")
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
			return shares.add(filter.Int64, "everyone", nil, nil, nil, nil)
		case "group":
			if gid, ok := r.groups[strings.ToLower(p1.String)]; ok {
				return shares.add(filter.Int64, "group", gid, nil, nil, nil)
			}
		case "project":
			pid, err := strconv.ParseInt(p1.String, 10, 64)
			if err == nil && r.projects[pid] != nil {
				var role any
				if p2.String != "" {
					rid, err := strconv.ParseInt(p2.String, 10, 64)
					if err != nil || !r.roles[rid] {
						skipped++
						return nil
					}
					role = rid
				}
				return shares.add(filter.Int64, "project", nil, pid, role, nil)
			}
		case "user":
			if uid, ok := r.users[p1.String]; ok {
				return shares.add(filter.Int64, "user", nil, nil, nil, uid)
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
	r.rep.step("partages de filtres", shares.total, skipped)

	favs := r.inserter("filter_favourites", "filter_id", "user_id")
	seen := map[[2]int64]bool{}
	skipped = 0
	if err := r.each(`SELECT username, entityid FROM favouriteassociations WHERE entitytype = 'SearchRequest'`, func(rows *sql.Rows) error {
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
		return favs.add(filter.Int64, uid)
	}); err != nil {
		return err
	}
	if err := favs.flush(); err != nil {
		return err
	}
	r.rep.step("filtres favoris", favs.total, skipped)
	return nil
}
