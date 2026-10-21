package api

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"

	"solstice/internal/i18n"
	"solstice/internal/perm"
)

func (s *Server) addLink(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	iss, err := s.visibleIssue(r, sub)
	if err != nil {
		return err
	}
	pi := iss.perm()
	if !sub.Can(perm.Link, iss.ProjectID, &pi) {
		return errForbidden
	}
	var in struct {
		Type      int64  `json:"type"`
		Direction string `json:"direction"`
		Target    string `json:"target"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	if in.Direction != "outward" && in.Direction != "inward" {
		return fail(http.StatusBadRequest, i18n.LinkDirectionUnknown)
	}
	cat, err := s.catalogue(ctx)
	if err != nil {
		return err
	}
	lt, ok := cat.linkType[in.Type]
	if !ok {
		return fail(http.StatusBadRequest, i18n.LinkTypeUnknown)
	}
	target, err := s.findIssue(ctx, in.Target)
	if err != nil || !sub.CanSee(target.perm()) {
		return fail(http.StatusBadRequest, i18n.LinkTargetNotFound)
	}
	if target.ID == iss.ID {
		return fail(http.StatusBadRequest, i18n.LinkToItself)
	}
	source, dest := iss, target
	if in.Direction == "inward" {
		source, dest = target, iss
	}
	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO issue_links (type_id, source_id, target_id) VALUES (?, ?, ?)`,
		in.Type, source.ID, dest.ID); err != nil {
		return err
	}
	// Jira records links in the history of both issues.
	sourceChange := []change{{field: "Link", newValue: dest.Key(), newString: "This issue " + lt.Outward + " " + dest.Key()}}
	destChange := []change{{field: "Link", newValue: source.Key(), newString: "This issue " + lt.Inward + " " + source.Key()}}
	if err := recordChanges(ctx, tx, source.ID, sub.UserID, now, sourceChange); err != nil {
		return err
	}
	if err := recordChanges(ctx, tx, dest.ID, sub.UserID, now, destChange); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.changed(source, sub.UserID, "link", sourceChange)
	s.changed(dest, sub.UserID, "link", destChange)
	return s.respondIssue(w, r, http.StatusCreated, sub, iss.ID)
}

func (s *Server) deleteLink(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return errNotFound
	}
	var sourceID, targetID, typeID int64
	if err := s.db.QueryRowContext(ctx, `SELECT source_id, target_id, type_id FROM issue_links WHERE id = ?`, id).
		Scan(&sourceID, &targetID, &typeID); errors.Is(err, sql.ErrNoRows) {
		return errNotFound
	} else if err != nil {
		return err
	}
	source, err := s.findIssueByID(ctx, sourceID)
	if err != nil {
		return err
	}
	target, err := s.findIssueByID(ctx, targetID)
	if err != nil {
		return err
	}
	sp, tp := source.perm(), target.perm()
	// Links are only shown between issues one sees: others do not exist
	// for the user.
	if !sub.CanSee(sp) || !sub.CanSee(tp) {
		return errNotFound
	}
	if !sub.Can(perm.Link, source.ProjectID, &sp) && !sub.Can(perm.Link, target.ProjectID, &tp) {
		return errForbidden
	}
	cat, err := s.catalogue(ctx)
	if err != nil {
		return err
	}
	lt := cat.linkType[typeID]
	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM issue_links WHERE id = ?`, id); err != nil {
		return err
	}
	// The history lines of a new link, the other way round.
	sourceChange := []change{{field: "Link", oldValue: target.Key(), oldString: "This issue " + lt.Outward + " " + target.Key()}}
	targetChange := []change{{field: "Link", oldValue: source.Key(), oldString: "This issue " + lt.Inward + " " + source.Key()}}
	if err := recordChanges(ctx, tx, source.ID, sub.UserID, now, sourceChange); err != nil {
		return err
	}
	if err := recordChanges(ctx, tx, target.ID, sub.UserID, now, targetChange); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.changed(source, sub.UserID, "link", sourceChange)
	s.changed(target, sub.UserID, "link", targetChange)
	w.WriteHeader(http.StatusNoContent)
	return nil
}
