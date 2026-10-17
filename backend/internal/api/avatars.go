package api

// Profile pictures. A person uploads a PNG or JPEG; the server decodes it and
// keeps a re-encoded PNG, so nothing but pixels is stored: no metadata, no
// trailing bytes. People without a picture get the initials drawn by
// writeAvatar.

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"solstice/internal/audit"
	"solstice/internal/i18n"
	"strconv"
	"time"

	_ "image/jpeg" // registers the JPEG decoder
)

const (
	avatarMaxBytes = 1 << 20
	avatarMaxSide  = 512
	// A multipart body adds boundaries and headers around the file.
	avatarMaxBody = avatarMaxBytes + 64<<10
)

func (s *Server) avatarPath(id int64) string {
	return filepath.Join(s.dataDir, "avatars", strconv.FormatInt(id, 10)+".png")
}

// reencodeAvatar checks an uploaded picture and returns it as a PNG.
func reencodeAvatar(src []byte) ([]byte, error) {
	if len(src) > avatarMaxBytes {
		return nil, fail(http.StatusRequestEntityTooLarge, i18n.AvatarTooLarge)
	}
	// The size is read first, so a small file that unpacks to a huge image is
	// refused before any pixel is decoded.
	cfg, format, err := image.DecodeConfig(bytes.NewReader(src))
	if err != nil || (format != "png" && format != "jpeg") {
		return nil, fail(http.StatusBadRequest, i18n.AvatarNotImage)
	}
	if cfg.Width > avatarMaxSide || cfg.Height > avatarMaxSide {
		return nil, fail(http.StatusBadRequest, i18n.AvatarDimensions)
	}
	img, _, err := image.Decode(bytes.NewReader(src))
	if err != nil {
		return nil, fail(http.StatusBadRequest, i18n.AvatarNotImage)
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// readAvatarUpload reads the first file of a multipart body.
func readAvatarUpload(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	r.Body = http.MaxBytesReader(w, r.Body, avatarMaxBody)
	reader, err := r.MultipartReader()
	if err != nil {
		return nil, fail(http.StatusBadRequest, i18n.AvatarUploadInvalid)
	}
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			return nil, fail(http.StatusBadRequest, i18n.AvatarNoFile)
		}
		if err != nil {
			return nil, fail(http.StatusRequestEntityTooLarge, i18n.AvatarTooLarge)
		}
		if part.FileName() == "" {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(part, avatarMaxBytes+1))
		if err != nil {
			return nil, fail(http.StatusRequestEntityTooLarge, i18n.AvatarTooLarge)
		}
		return data, nil
	}
}

// setAvatar stores a person's picture, replacing the previous one.
func (s *Server) setAvatar(w http.ResponseWriter, r *http.Request, id int64) error {
	raw, err := readAvatarUpload(w, r)
	if err != nil {
		return err
	}
	data, err := reencodeAvatar(raw)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(s.dataDir, "avatars"), 0o750); err != nil {
		return err
	}
	_, err = writeFile(s.avatarPath(id), bytes.NewReader(data))
	return err
}

// removeAvatar deletes a person's picture, if they have one.
func (s *Server) removeAvatar(id int64) error {
	if err := os.Remove(s.avatarPath(id)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *Server) putMyAvatar(w http.ResponseWriter, r *http.Request) error {
	if err := s.setAvatar(w, r, currentUser(r).ID); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) deleteMyAvatar(w http.ResponseWriter, r *http.Request) error {
	if err := s.removeAvatar(currentUser(r).ID); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// adminAvatarTarget checks that the caller is an administrator and that the
// account exists.
func (s *Server) adminAvatarTarget(r *http.Request) (int64, audit.Target, error) {
	if err := requireAdmin(r); err != nil {
		return 0, audit.Target{}, err
	}
	id, err := pathID(r, "id")
	if err != nil {
		return 0, audit.Target{}, err
	}
	u, err := s.adminUserByID(r.Context(), id)
	if err != nil {
		return 0, audit.Target{}, err
	}
	return id, audit.Target{Type: "user", ID: strconv.FormatInt(id, 10), Label: u.Username}, nil
}

// The audit log records a change of picture as an update of the account.
func (s *Server) adminPutAvatar(w http.ResponseWriter, r *http.Request) error {
	id, target, err := s.adminAvatarTarget(r)
	if err != nil {
		return err
	}
	if err := s.setAvatar(w, r, id); err != nil {
		return err
	}
	s.auditLog(r, audit.UserUpdate, target, map[string]any{"avatar": "set"})
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) adminDeleteAvatar(w http.ResponseWriter, r *http.Request) error {
	id, target, err := s.adminAvatarTarget(r)
	if err != nil {
		return err
	}
	if err := s.removeAvatar(id); err != nil {
		return err
	}
	s.auditLog(r, audit.UserUpdate, target, map[string]any{"avatar": "remove"})
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// getAvatar serves a person's picture, or their initials when they have none.
func (s *Server) getAvatar(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "userId")
	if err != nil {
		return err
	}
	var username, name string
	err = s.db.QueryRowContext(r.Context(), `SELECT username, display_name FROM users WHERE id = ?`, id).
		Scan(&username, &name)
	if errors.Is(err, sql.ErrNoRows) {
		return errNotFound
	}
	if err != nil {
		return err
	}
	data, err := os.ReadFile(s.avatarPath(id))
	if errors.Is(err, os.ErrNotExist) {
		return writeAvatar(w, r, username, name)
	}
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	h := w.Header()
	h.Set("Content-Type", "image/png")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "default-src 'none'")
	h.Set("Cache-Control", "private, max-age=300")
	h.Set("ETag", `"`+hex.EncodeToString(sum[:16])+`"`)
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
	return nil
}
