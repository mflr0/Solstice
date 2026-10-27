package api

// The personal dashboard on the home page: an ordered list of gadgets, each
// on a saved filter (its results, or its issues counted by a field) or built
// in (assigned to me, reported by me, recently updated). Until the user edits
// it, the dashboard holds the built-in gadgets that the home page used to show.

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"solstice/internal/i18n"
	"solstice/internal/perm"
	"solstice/internal/sqlq"
)

const (
	gadgetResults  = "results"
	gadgetCount    = "count"
	gadgetAssigned = "assigned"
	gadgetReported = "reported"
	gadgetRecent   = "recent"

	maxGadgets    = 20
	maxGadgetSize = 50
	maxGadgetCols = 30
)

// gadgetConfig is a gadget's settings: columns and size for a list, field for
// a count.
type gadgetConfig struct {
	Columns []string `json:"columns,omitempty"`
	Size    int      `json:"size,omitempty"`
	Field   string   `json:"field,omitempty"`
}

type gadgetFilter struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Query string `json:"query"`
}

type gadgetOut struct {
	ID     int64         `json:"id"`
	Kind   string        `json:"kind"`
	Filter *gadgetFilter `json:"filter"`
	// The gadget's filter is deleted or no longer shared with the user.
	Unavailable bool         `json:"unavailable"`
	Config      gadgetConfig `json:"config"`
}

type dashboardOut struct {
	Customised bool        `json:"customised"`
	Gadgets    []gadgetOut `json:"gadgets"`
}

type gadgetIn struct {
	// An existing gadget's id, 0 for a new one.
	ID     int64        `json:"id"`
	Kind   string       `json:"kind"`
	Filter *int64       `json:"filter"`
	Config gadgetConfig `json:"config"`
}

type dashboardIn struct {
	Gadgets []gadgetIn `json:"gadgets"`
}

func onFilter(kind string) bool { return kind == gadgetResults || kind == gadgetCount }

// defaultDashboard is what a user who never edited the dashboard sees.
func defaultDashboard() []gadgetOut {
	return []gadgetOut{
		{Kind: gadgetAssigned, Config: gadgetConfig{Size: 15}},
		{Kind: gadgetReported, Config: gadgetConfig{Size: 15}},
		{Kind: gadgetRecent, Config: gadgetConfig{Size: 12}},
	}
}

type gadgetRow struct {
	id     int64
	kind   string
	filter sql.NullInt64
	config gadgetConfig
}

func (s *Server) gadgetRows(ctx context.Context, q sqlq.Querier, user int64) ([]gadgetRow, error) {
	var out []gadgetRow
	err := sqlq.Each(ctx, q, `SELECT id, kind, filter_id, config FROM dashboard_gadgets
		WHERE user_id = ? ORDER BY position, id`, func(rows *sql.Rows) error {
		var g gadgetRow
		var config string
		if err := rows.Scan(&g.id, &g.kind, &g.filter, &config); err != nil {
			return err
		}
		// A config that no longer reads gives the defaults rather than a broken page.
		_ = json.Unmarshal([]byte(config), &g.config)
		out = append(out, g)
		return nil
	}, user)
	return out, err
}

// loadDashboard reads the user's dashboard, resolving each filter gadget's
// filter as the user sees it now.
func (s *Server) loadDashboard(ctx context.Context, sub *perm.Subject) (dashboardOut, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM dashboards WHERE user_id = ?`, sub.UserID).Scan(&n); err != nil {
		return dashboardOut{}, err
	}
	if n == 0 {
		return dashboardOut{Gadgets: defaultDashboard()}, nil
	}
	rows, err := s.gadgetRows(ctx, s.db, sub.UserID)
	if err != nil {
		return dashboardOut{}, err
	}
	visible, err := s.visibleFilters(ctx, sub)
	if err != nil {
		return dashboardOut{}, err
	}
	out := dashboardOut{Customised: true, Gadgets: []gadgetOut{}}
	for _, g := range rows {
		o := gadgetOut{ID: g.id, Kind: g.kind, Config: g.config}
		if onFilter(g.kind) {
			if f := visible[g.filter.Int64]; g.filter.Valid && f != nil {
				o.Filter = &gadgetFilter{ID: f.ID, Name: f.Name, Query: f.Query}
			} else {
				o.Unavailable = true
			}
		}
		out.Gadgets = append(out.Gadgets, o)
	}
	return out, nil
}

func (s *Server) visibleFilters(ctx context.Context, sub *perm.Subject) (map[int64]*filterOut, error) {
	list, err := s.loadFilters(ctx, sub, 0)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]*filterOut, len(list))
	for _, f := range list {
		out[f.out.ID] = &f.out
	}
	return out, nil
}

func (s *Server) getDashboard(w http.ResponseWriter, r *http.Request) error {
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	out, err := s.loadDashboard(r.Context(), sub)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

// checkGadget validates a gadget's kind and settings. A filter gadget needs
// a filter the user sees, unless it keeps the one it already had: a gadget
// whose filter went away can still be moved or reconfigured.
func checkGadget(g *gadgetIn, before map[int64]gadgetRow, visible map[int64]*filterOut, custom []customFieldDef) error {
	switch g.Kind {
	case gadgetResults, gadgetCount:
		old, kept := before[g.ID]
		kept = kept && onFilter(old.kind) &&
			(g.Filter == nil && !old.filter.Valid || g.Filter != nil && old.filter.Valid && *g.Filter == old.filter.Int64)
		if !kept && (g.Filter == nil || visible[*g.Filter] == nil) {
			return fail(http.StatusBadRequest, i18n.DashboardFilterRequired)
		}
	case gadgetAssigned, gadgetReported, gadgetRecent:
		g.Filter = nil
	default:
		return failf(http.StatusBadRequest, i18n.DashboardGadgetKindUnknown, g.Kind)
	}
	c := &g.Config
	if g.Kind == gadgetCount {
		if c.Field != "labels" && facetColumns[c.Field] == "" {
			return failf(http.StatusBadRequest, i18n.DashboardCountFieldUnknown, c.Field)
		}
		c.Columns, c.Size = nil, 0
		return nil
	}
	c.Field = ""
	if c.Size < 0 || c.Size > maxGadgetSize {
		return failf(http.StatusBadRequest, i18n.DashboardSizeInvalid, maxGadgetSize)
	}
	if len(c.Columns) > maxGadgetCols {
		return failf(http.StatusBadRequest, i18n.DashboardColumnsTooMany, maxGadgetCols)
	}
	if len(c.Columns) > 0 {
		if _, err := parseColumns(strings.Join(c.Columns, ","), "", custom); err != nil {
			return err
		}
	}
	return nil
}

// putDashboard replaces the user's gadgets with the ordered list given.
func (s *Server) putDashboard(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	var in dashboardIn
	if err := readJSON(r, &in); err != nil {
		return err
	}
	if len(in.Gadgets) > maxGadgets {
		return failf(http.StatusBadRequest, i18n.DashboardTooManyGadgets, maxGadgets)
	}
	visible, err := s.visibleFilters(ctx, sub)
	if err != nil {
		return err
	}
	custom, err := s.customFields(ctx)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Locks the user's dashboard row, so two saves at once apply one after the other.
	if _, err := tx.ExecContext(ctx, `INSERT INTO dashboards (user_id, updated_at) VALUES (?, ?)
		ON DUPLICATE KEY UPDATE updated_at = VALUES(updated_at)`, sub.UserID, time.Now().UTC()); err != nil {
		return err
	}
	rows, err := s.gadgetRows(ctx, tx, sub.UserID)
	if err != nil {
		return err
	}
	before := make(map[int64]gadgetRow, len(rows))
	for _, g := range rows {
		before[g.id] = g
	}
	seen := map[int64]bool{}
	for i := range in.Gadgets {
		g := &in.Gadgets[i]
		if _, ok := before[g.ID]; !ok || seen[g.ID] {
			g.ID = 0
		}
		seen[g.ID] = true
		if err := checkGadget(g, before, visible, custom); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM dashboard_gadgets WHERE user_id = ?`, sub.UserID); err != nil {
		return err
	}
	for i, g := range in.Gadgets {
		config, err := json.Marshal(g.Config)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO dashboard_gadgets (id, user_id, position, kind, filter_id, config)
			VALUES (?, ?, ?, ?, ?, ?)`, sqlq.NullIf(g.ID), sub.UserID, i, g.Kind, g.Filter, config); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	out, err := s.loadDashboard(ctx, sub)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}
