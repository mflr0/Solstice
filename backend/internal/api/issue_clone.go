package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"solstice/internal/events"
	"solstice/internal/i18n"
	"solstice/internal/perm"
	"solstice/internal/sqlq"
)

// Cloning ------------------------------------------------------------------

// clonePlan is one issue to create: the clone of src.
type clonePlan struct {
	src      *issueRow
	summary  string
	status   string
	assignee any
	id, num  int64
}

// attachmentCopy is an attachment row of a clone whose file is still to copy.
type attachmentCopy struct {
	from, to int64
	name     string
	missing  bool
}

// cloneLink is a link of the original to copy onto the clone.
type cloneLink struct {
	typeID  int64
	other   *issueRow
	outward bool // the original is the source of the link
}

// clonesLinkType finds the "clones" / "is cloned by" link type, whatever the
// language the catalogue was seeded in.
func clonesLinkType(cat *catalog) (linkTypeRef, bool) {
	for _, lt := range cat.LinkTypes {
		for _, loc := range i18n.Locales {
			if strings.EqualFold(lt.Outward, i18n.M(i18n.DefaultLinkClonesOutward).In(loc)) &&
				strings.EqualFold(lt.Inward, i18n.M(i18n.DefaultLinkClonesInward).In(loc)) {
				return lt, true
			}
		}
	}
	return linkTypeRef{}, false
}

// cloneAssignee applies the project's assignment rule to a new issue with the
// components of src.
func (s *Server) cloneAssignee(ctx context.Context, src *issueRow, lead int64, defaultAssignee string) (any, error) {
	ids := []int64{}
	if err := sqlq.Each(ctx, s.db, `SELECT component_id FROM issue_components WHERE issue_id = ?`,
		func(rows *sql.Rows) error {
			var id int64
			err := rows.Scan(&id)
			ids = append(ids, id)
			return err
		}, src.ID); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(ids)
	if err != nil {
		return nil, err
	}
	if who, ok, err := s.componentAssignee(ctx, fields{"components": raw}, src.ProjectID, lead); err != nil || ok {
		return who, err
	}
	if defaultAssignee == "lead" && lead != 0 {
		return lead, nil
	}
	return nil, nil
}

// insertClone writes one clone of p.src and everything it copies from it.
// parent is the clone's parent issue (nil for none).
func (s *Server) insertClone(ctx context.Context, tx *sql.Tx, by int64, p *clonePlan, parent any, withAttachments bool,
	lang string, now time.Time) ([]attachmentCopy, error) {
	src := p.src
	if err := tx.QueryRowContext(ctx, `SELECT issue_counter + 1 FROM projects WHERE id = ? FOR UPDATE`, src.ProjectID).
		Scan(&p.num); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE projects SET issue_counter = ? WHERE id = ?`, p.num, src.ProjectID); err != nil {
		return nil, err
	}
	var lastRank string
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(rank_key), '') FROM issues`).Scan(&lastRank); err != nil {
		return nil, err
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO issues (project_id, num, issue_type_id, parent_id, summary,
		description, status_id, priority_id, reporter_id, assignee_id, creator_id, security_level_id,
		created_at, updated_at, rank_key) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		src.ProjectID, p.num, src.TypeID, parent, p.summary, sqlq.Value(src.Description), p.status,
		sqlq.Value(src.PriorityID), by, p.assignee, by, sqlq.Value(src.LevelID), now, now, rankBetween(lastRank, ""))
	if err != nil {
		return nil, err
	}
	if p.id, err = res.LastInsertId(); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO watchers (issue_id, user_id, created_at) VALUES (?, ?, ?)`,
		p.id, by, now); err != nil {
		return nil, err
	}
	copies := []string{
		`INSERT INTO labels (issue_id, field_id, label) SELECT ?, field_id, label FROM labels WHERE issue_id = ?`,
		`INSERT INTO issue_components (issue_id, component_id) SELECT ?, component_id FROM issue_components
			WHERE issue_id = ?`,
		`INSERT INTO issue_versions (issue_id, version_id, kind) SELECT ?, version_id, kind FROM issue_versions
			WHERE issue_id = ?`,
	}
	for _, q := range copies {
		if _, err := tx.ExecContext(ctx, q, p.id, src.ID); err != nil {
			return nil, err
		}
	}
	// Custom field values, but not the internal ones (rank, sprint…).
	hidden := make([]any, 0, len(hiddenKinds))
	for kind := range hiddenKinds {
		hidden = append(hidden, kind)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO custom_field_values (issue_id, field_id, parent_key, string_value,
		number_value, text_value, date_value) SELECT ?, v.field_id, v.parent_key, v.string_value, v.number_value,
		v.text_value, v.date_value FROM custom_field_values v JOIN custom_fields f ON f.id = v.field_id
		WHERE v.issue_id = ? AND SUBSTRING_INDEX(f.type_key, ':', -1) NOT IN (`+sqlq.Marks(len(hidden))+`)`,
		append([]any{p.id, src.ID}, hidden...)...); err != nil {
		return nil, err
	}
	origin := []change{{field: "Link", newValue: src.Key(),
		newString: i18n.M(i18n.IssueCloneHistory, src.Key()).In(lang)}}
	if err := recordChanges(ctx, tx, p.id, by, now, origin); err != nil {
		return nil, err
	}
	if !withAttachments {
		return nil, nil
	}
	var attachments []attachmentCopy
	err = sqlq.Each(ctx, tx, `SELECT id, filename, file_missing FROM attachments WHERE issue_id = ? ORDER BY id`,
		func(rows *sql.Rows) error {
			var a attachmentCopy
			a.to = p.id
			if err := rows.Scan(&a.from, &a.name, &a.missing); err != nil {
				return err
			}
			attachments = append(attachments, a)
			return nil
		}, src.ID)
	if err != nil {
		return nil, err
	}
	for i, a := range attachments {
		res, err := tx.ExecContext(ctx, `INSERT INTO attachments (issue_id, author_id, filename, mime_type, size,
			created_at, file_missing) SELECT ?, author_id, filename, mime_type, size, ?, file_missing
			FROM attachments WHERE id = ?`, p.id, now, a.from)
		if err != nil {
			return nil, err
		}
		if attachments[i].to, err = res.LastInsertId(); err != nil {
			return nil, err
		}
	}
	return attachments, nil
}

func (s *Server) copyAttachmentFile(from, to int64) error {
	if err := os.MkdirAll(filepath.Dir(s.attachmentPath(to)), 0o750); err != nil {
		return err
	}
	src, err := os.Open(s.attachmentPath(from))
	if err != nil {
		return err
	}
	defer src.Close()
	_, err = writeFile(s.attachmentPath(to), src)
	return err
}

// cloneIssue creates a copy of an issue, and of its sub-tasks, links and
// attachments when asked. Files are copied once the rows are committed: a
// file that cannot be copied is reported in the answer's warnings.
func (s *Server) cloneIssue(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	orig, err := s.visibleIssue(r, sub)
	if err != nil {
		return err
	}
	var in struct {
		Summary            *string `json:"summary"`
		IncludeSubtasks    bool    `json:"includeSubtasks"`
		IncludeLinks       bool    `json:"includeLinks"`
		IncludeAttachments bool    `json:"includeAttachments"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	if !sub.Can(perm.Create, orig.ProjectID, nil) || !sub.CanBrowseProject(orig.ProjectID) {
		return errForbidden
	}
	if !s.projectHasType(ctx, orig.ProjectID, orig.TypeID) {
		return fail(http.StatusBadRequest, i18n.IssueTypeUnavailable)
	}
	lang := locale(r)
	warnings := []string{}
	summary := i18n.M(i18n.IssueCloneSummary, orig.Summary).In(lang)
	if utf8.RuneCountInString(summary) > 255 {
		summary = string([]rune(summary)[:255])
	}
	if in.Summary != nil {
		summary = strings.TrimSpace(*in.Summary)
		if summary == "" || utf8.RuneCountInString(summary) > 255 {
			return fail(http.StatusBadRequest, i18n.IssueSummaryRequired)
		}
	}
	cat, err := s.catalogue(ctx)
	if err != nil {
		return err
	}

	var lead sql.NullInt64
	var defaultAssignee string
	if err := s.db.QueryRowContext(ctx, `SELECT lead_id, default_assignee FROM projects WHERE id = ?`, orig.ProjectID).
		Scan(&lead, &defaultAssignee); err != nil {
		return err
	}
	newPlan := func(src *issueRow, text string) (*clonePlan, error) {
		status, err := s.initialStatusFor(ctx, src.ProjectID, src.TypeID)
		if err != nil {
			return nil, err
		}
		assignee, err := s.cloneAssignee(ctx, src, lead.Int64, defaultAssignee)
		return &clonePlan{src: src, summary: text, status: status, assignee: assignee}, err
	}
	clone, err := newPlan(orig, summary)
	if err != nil {
		return err
	}
	var subtasks []*clonePlan
	if in.IncludeSubtasks {
		var children []*issueRow
		if err := sqlq.Each(ctx, s.db, `SELECT `+issueColumns+` FROM issues i JOIN projects p ON p.id = i.project_id
			WHERE i.parent_id = ? ORDER BY i.num`, func(rows *sql.Rows) error {
			child, err := scanIssue(rows)
			if err == nil && sub.CanSee(child.perm()) {
				children = append(children, child)
			}
			return err
		}, orig.ID); err != nil {
			return err
		}
		for _, child := range children {
			// The sub-task is created in its own project, which may not be the original's.
			if !sub.Can(perm.Create, child.ProjectID, nil) || !sub.CanBrowseProject(child.ProjectID) ||
				!s.projectHasType(ctx, child.ProjectID, child.TypeID) {
				warnings = append(warnings, i18n.M(i18n.IssueCloneSubtaskSkipped, child.Key()).In(lang))
				continue
			}
			p, err := newPlan(child, child.Summary)
			if err != nil {
				return err
			}
			subtasks = append(subtasks, p)
		}
	}

	// Links to issues the user cannot see are left out, as everywhere else.
	var links []cloneLink
	if in.IncludeLinks {
		type linkRow struct{ typeID, source, target int64 }
		var found []linkRow
		if err := sqlq.Each(ctx, s.db, `SELECT type_id, source_id, target_id FROM issue_links
			WHERE source_id = ? OR target_id = ? ORDER BY id`, func(row *sql.Rows) error {
			var l linkRow
			err := row.Scan(&l.typeID, &l.source, &l.target)
			found = append(found, l)
			return err
		}, orig.ID, orig.ID); err != nil {
			return err
		}
		for _, l := range found {
			outward, otherID := l.source == orig.ID, l.target
			if !outward {
				otherID = l.source
			}
			other, err := s.findIssueByID(ctx, otherID)
			if err != nil {
				return err
			}
			if sub.CanSee(other.perm()) {
				links = append(links, cloneLink{typeID: l.typeID, other: other, outward: outward})
			}
		}
		if len(links) > 0 && !sub.Can(perm.Link, orig.ProjectID,
			&perm.Issue{ProjectID: orig.ProjectID, ReporterID: sub.UserID}) {
			return errForbidden
		}
	}

	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	files, err := s.insertClone(ctx, tx, sub.UserID, clone, sqlq.Value(orig.ParentID), in.IncludeAttachments, lang, now)
	if err != nil {
		return err
	}
	cloneKey := orig.ProjectKey + "-" + strconv.FormatInt(clone.num, 10)
	for _, p := range subtasks {
		more, err := s.insertClone(ctx, tx, sub.UserID, p, clone.id, in.IncludeAttachments, lang, now)
		if err != nil {
			return err
		}
		files = append(files, more...)
	}

	// The issue at the other end of a link hears about it too.
	type announced struct {
		issue   *issueRow
		changes []change
	}
	var others []announced
	// link joins the clone to other; cloneIsSource tells which way it points.
	link := func(typeID int64, other *issueRow, cloneIsSource bool) error {
		lt := cat.linkType[typeID]
		source, dest := clone.id, other.ID
		cloneText, otherText := "This issue "+lt.Outward+" "+other.Key(), "This issue "+lt.Inward+" "+cloneKey
		if !cloneIsSource {
			source, dest = other.ID, clone.id
			cloneText, otherText = "This issue "+lt.Inward+" "+other.Key(), "This issue "+lt.Outward+" "+cloneKey
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO issue_links (type_id, source_id, target_id) VALUES (?, ?, ?)`,
			typeID, source, dest); err != nil {
			return err
		}
		cloneChange := []change{{field: "Link", newValue: other.Key(), newString: cloneText}}
		otherChange := []change{{field: "Link", newValue: cloneKey, newString: otherText}}
		if err := recordChanges(ctx, tx, clone.id, sub.UserID, now, cloneChange); err != nil {
			return err
		}
		if err := recordChanges(ctx, tx, other.ID, sub.UserID, now, otherChange); err != nil {
			return err
		}
		others = append(others, announced{other, otherChange})
		return nil
	}
	// The link is written on the original too: that takes the Link permission there.
	if lt, ok := clonesLinkType(cat); ok {
		op := orig.perm()
		if sub.Can(perm.Link, orig.ProjectID, &op) {
			if err := link(lt.ID, orig, true); err != nil {
				return err
			}
		} else {
			warnings = append(warnings, i18n.M(i18n.IssueCloneLinkSkipped, orig.Key()).In(lang))
		}
	}
	for _, l := range links {
		if err := link(l.typeID, l.other, l.outward); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	for _, f := range files {
		if f.missing {
			continue // never imported: nothing to copy, the row says so
		}
		if err := s.copyAttachmentFile(f.from, f.to); err != nil {
			s.log.Error("clone: attachment not copied", "from", f.from, "to", f.to, "err", err)
			if _, err := s.db.ExecContext(ctx, `DELETE FROM attachments WHERE id = ?`, f.to); err != nil {
				s.log.Error("clone: attachment row not removed", "id", f.to, "err", err)
			}
			warnings = append(warnings, i18n.M(i18n.IssueCloneAttachmentFailed, f.name).In(lang))
		}
	}

	for _, p := range append([]*clonePlan{clone}, subtasks...) {
		created, err := s.findIssueByID(ctx, p.id)
		if err != nil {
			return err
		}
		s.emit(issueEvent(events.IssueCreated, created, sub.UserID, "issue"))
	}
	for _, o := range others {
		s.changed(o.issue, sub.UserID, "link", o.changes)
	}
	return writeJSON(w, http.StatusCreated, map[string]any{"key": cloneKey, "warnings": warnings})
}
