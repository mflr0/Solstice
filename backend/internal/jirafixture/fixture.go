package jirafixture

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"solstice/internal/jiradb"
)

// Options configure a fixture run.
type Options struct {
	URL      string         // postgres://… or mysql://…
	JiraDist string         // local Jira distribution, read for the schema
	JiraHome string         // receives data/attachments/…
	Issues   int            // number of issues
	Seed     uint64         // same seed ⇒ same data, on either engine
	TZ       *time.Location // time zone of the simulated Jira server
	Log      io.Writer      // progress messages; nil for none
}

// Summary describes what was generated, for the importer tests to check.
type Summary struct {
	Engine string `json:"engine"`
	Seed   uint64 `json:"seed"`
	TZ     string `json:"tz"`
	Tables int    `json:"tables"`

	AppUsers               int   `json:"app_users"`
	UsersWithPKCS5S2       int   `json:"users_with_pkcs5s2"`
	UsersLDAPNopass        int   `json:"users_ldap_nopass"`
	InactiveUsers          int   `json:"inactive_users"`
	UsersWithoutAccess     int   `json:"users_without_access"`
	DeletedUsers           int   `json:"deleted_users"`
	Groups                 int   `json:"groups"`
	NestedGroupMemberships int   `json:"nested_group_memberships"`
	Projects               int   `json:"projects"`
	Issues                 int   `json:"issues"`
	Subtasks               int   `json:"subtasks"`
	MovedIssueKeys         int   `json:"moved_issue_keys"`
	SecuredIssues          int   `json:"secured_issues"`
	Components             int   `json:"components"`
	Versions               int   `json:"versions"`
	IssueComponents        int   `json:"issue_components"`
	IssueVersionsAffects   int   `json:"issue_versions_affects"`
	IssueVersionsFix       int   `json:"issue_versions_fix"`
	LabelsSystem           int   `json:"labels_system"`
	LabelsCustom           int   `json:"labels_custom"`
	Comments               int   `json:"comments"`
	RestrictedComments     int   `json:"restricted_comments"`
	Worklogs               int   `json:"worklogs"`
	Attachments            int   `json:"attachments"`
	AttachmentFilesMissing int   `json:"attachment_files_missing"`
	AttachmentMissingID    int64 `json:"attachment_missing_id,omitempty"`
	LinksNonSubtask        int   `json:"links_non_subtask"`
	RemoteLinks            int   `json:"remote_links"`
	Watchers               int   `json:"watchers"`
	Votes                  int   `json:"votes"`
	CustomFields           int   `json:"custom_fields"`
	CustomFieldOptions     int   `json:"custom_field_options"`
	CustomFieldValues      int   `json:"custom_field_values"`
	ChangeGroups           int   `json:"change_groups"`
	ChangeItems            int   `json:"change_items"`
	Filters                int   `json:"filters"`
	FilterShares           int   `json:"filter_shares"`
	FilterFavourites       int   `json:"filter_favourites"`

	// Jira Software (AO_60DB71_* tables, Sprint field, epic links).
	AOTables            int `json:"ao_tables"`
	Boards              int `json:"boards"`
	BoardColumns        int `json:"board_columns"`
	BoardColumnStatuses int `json:"board_column_statuses"`
	QuickFilters        int `json:"quick_filters"`
	Swimlanes           int `json:"swimlanes"`
	SprintsClosed       int `json:"sprints_closed"`
	SprintsActive       int `json:"sprints_active"`
	SprintsFuture       int `json:"sprints_future"`
	SprintMemberships   int `json:"sprint_memberships"` // Sprint field values, also counted in custom_field_values
	RankedIssues        int `json:"ranked_issues"`
	EpicLinks           int `json:"epic_links"`

	Checkpoints []Checkpoint   `json:"checkpoints"`
	Rows        map[string]int `json:"rows"` // rows written per table
}

// Checkpoint is one issue as the importer should end up showing it.
type Checkpoint struct {
	Key              string  `json:"key"`
	Summary          string  `json:"summary"`
	StatusName       string  `json:"status_name"`
	AssigneeUsername *string `json:"assignee_username"`
	ReporterUsername string  `json:"reporter_username"`
	PriorityName     string  `json:"priority_name"`
	ResolutionName   *string `json:"resolution_name"`
	Created          string  `json:"created"`
	DueDate          *string `json:"due_date"`
	CommentCount     int     `json:"comment_count"`
	ParentKey        *string `json:"parent_key"`
	EpicKey          *string `json:"epic_key"`
}

// Run recreates the Jira schema in the target database and fills it.
func Run(ctx context.Context, opt Options) (*Summary, error) {
	if opt.URL == "" || opt.JiraDist == "" || opt.JiraHome == "" {
		return nil, errors.New("url, jira-dist and jira-home are required")
	}
	if opt.Issues < 1 {
		return nil, errors.New("issues must be at least 1")
	}
	if opt.TZ == nil {
		opt.TZ = time.UTC
	}
	logf := func(format string, args ...any) {
		if opt.Log != nil {
			fmt.Fprintf(opt.Log, format+"\n", args...)
		}
	}

	db, eng, err := jiradb.Open(opt.URL, opt.TZ)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("connect to %s: %w", eng, err)
	}

	model, types, err := LoadDistribution(opt.JiraDist, eng)
	if err != nil {
		return nil, err
	}
	stmts, err := model.DDL(eng, types)
	if err != nil {
		return nil, err
	}
	stmts = append(stmts, AODDL(eng)...)
	logf("recreating %d tables on %s", len(model.Entities), eng)
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			return nil, fmt.Errorf("%s: %w", s, err)
		}
	}

	if err := os.RemoveAll(filepath.Join(opt.JiraHome, "data", "attachments")); err != nil {
		return nil, err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	sum := &Summary{Engine: eng.String(), Seed: opt.Seed, TZ: opt.TZ.String(), Tables: len(model.Entities),
		AOTables: len(aoTables)}
	w := NewWriter(ctx, tx, eng, model)
	start := time.Now()
	newGenerator(opt, w, sum).run()
	if err := w.Close(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	sum.Rows = w.Rows
	total := 0
	for _, n := range w.Rows {
		total += n
	}
	logf("wrote %d rows in %s", total, time.Since(start).Round(time.Millisecond))
	return sum, nil
}
