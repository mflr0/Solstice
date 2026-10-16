package api

import (
	"database/sql"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"solstice/internal/i18n"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"solstice/internal/perm"
)

// maxUpload caps a single upload request.
const maxUpload = 100 << 20

func (s *Server) attachmentPath(id int64) string {
	return filepath.Join(s.dataDir, "attachments", strconv.FormatInt(id, 10))
}

func (s *Server) upload(w http.ResponseWriter, r *http.Request) error {
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
	if !sub.Can(perm.Attach, iss.ProjectID, &pi) {
		return errForbidden
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload)
	reader, err := r.MultipartReader()
	if err != nil {
		return fail(http.StatusBadRequest, i18n.AttachmentUploadInvalid)
	}
	if err := os.MkdirAll(filepath.Join(s.dataDir, "attachments"), 0o750); err != nil {
		return err
	}
	var names []string
	now := time.Now().UTC()
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fail(http.StatusRequestEntityTooLarge, i18n.AttachmentTooLarge)
		}
		name := cleanFilename(part.FileName())
		if name == "" {
			continue
		}
		mimeType := part.Header.Get("Content-Type")
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		res, err := s.db.ExecContext(ctx, `INSERT INTO attachments (issue_id, author_id, filename, mime_type, size,
			created_at) VALUES (?, ?, ?, ?, 0, ?)`, iss.ID, sub.UserID, name, mimeType, now)
		if err != nil {
			return err
		}
		id, _ := res.LastInsertId()
		size, err := writeFile(s.attachmentPath(id), part)
		if err != nil {
			s.db.ExecContext(ctx, `DELETE FROM attachments WHERE id = ?`, id)
			if errors.As(err, new(*http.MaxBytesError)) {
				return fail(http.StatusRequestEntityTooLarge, i18n.AttachmentTooLarge)
			}
			return err
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE attachments SET size = ? WHERE id = ?`, size, id); err != nil {
			return err
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return fail(http.StatusBadRequest, i18n.AttachmentNoFile)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var changes []change
	for _, n := range names {
		changes = append(changes, change{field: "Attachment", newString: n})
	}
	if err := recordChanges(ctx, tx, iss.ID, sub.UserID, now, changes); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE issues SET updated_at = ? WHERE id = ?`, now, iss.ID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.changed(iss, sub.UserID, "attachment", changes)
	return s.respondIssue(w, r, sub, iss.ID)
}

func writeFile(path string, src io.Reader) (int64, error) {
	tmp := path + ".part"
	f, err := os.Create(tmp) // #nosec G304 -- path built by the server, not from user input
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(f, src)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(tmp)
		return 0, err
	}
	return n, os.Rename(tmp, path)
}

// cleanFilename keeps the base name and drops control characters.
func cleanFilename(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, `\`, "/"))
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, name)
	if name == "." || name == "/" {
		return ""
	}
	if utf8.RuneCountInString(name) > 255 {
		ext := []rune(filepath.Ext(name))
		if len(ext) > 20 {
			ext = nil
		}
		base := []rune(strings.TrimSuffix(name, string(ext)))
		name = string(base[:255-len(ext)]) + string(ext)
	}
	return name
}

// Images that browsers display safely inline. Everything else is downloaded,
// so an uploaded HTML or SVG file can never run in Solstice's origin.
var inlineTypes = map[string]bool{
	"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true, "application/pdf": true,
}

type attachmentRow struct {
	issue          *issueRow
	id             int64
	name, mimeType string
	author         sql.NullInt64
	missing        bool
}

func (s *Server) visibleAttachment(r *http.Request, sub *perm.Subject) (*attachmentRow, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return nil, errNotFound
	}
	a := attachmentRow{id: id}
	var issueID int64
	err = s.db.QueryRowContext(r.Context(), `SELECT issue_id, filename, mime_type, author_id, file_missing
		FROM attachments WHERE id = ?`, id).Scan(&issueID, &a.name, &a.mimeType, &a.author, &a.missing)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}
	a.issue, err = s.findIssueByID(r.Context(), issueID)
	if err != nil {
		return nil, err
	}
	if !sub.CanSee(a.issue.perm()) {
		return nil, errNotFound
	}
	return &a, nil
}

func (s *Server) download(w http.ResponseWriter, r *http.Request) error {
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	a, err := s.visibleAttachment(r, sub)
	if err != nil {
		return err
	}
	if a.missing {
		return fail(http.StatusNotFound, i18n.AttachmentNotImported)
	}
	f, err := os.Open(s.attachmentPath(a.id))
	if errors.Is(err, os.ErrNotExist) {
		return fail(http.StatusNotFound, i18n.AttachmentFileMissing)
	}
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	mediaType, _, _ := mime.ParseMediaType(a.mimeType)
	disposition := "attachment"
	if inlineTypes[mediaType] && r.URL.Query().Get("download") == "" {
		disposition = "inline"
		w.Header().Set("Content-Type", mediaType)
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": a.name}))
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'; img-src 'self'; style-src 'unsafe-inline'")
	// Revalidate every time, so revoked access also stops cached copies.
	w.Header().Set("Cache-Control", "private, no-cache")
	http.ServeContent(w, r, "", info.ModTime(), f)
	return nil
}

func (s *Server) deleteAttachment(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	a, err := s.visibleAttachment(r, sub)
	if err != nil {
		return err
	}
	pi := a.issue.perm()
	own := a.author.Valid && a.author.Int64 == sub.UserID && sub.Can(perm.Attach, a.issue.ProjectID, &pi)
	if !own && !sub.Can(perm.Moderate, a.issue.ProjectID, &pi) {
		return errForbidden
	}
	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM attachments WHERE id = ?`, a.id); err != nil {
		return err
	}
	removed := []change{{field: "Attachment", oldValue: strconv.FormatInt(a.id, 10), oldString: a.name}}
	if err := recordChanges(ctx, tx, a.issue.ID, sub.UserID, now, removed); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	os.Remove(s.attachmentPath(a.id))
	s.changed(a.issue, sub.UserID, "attachment", removed)
	w.WriteHeader(http.StatusNoContent)
	return nil
}
