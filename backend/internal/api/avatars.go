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
	"fmt"
	"html"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"solstice/internal/audit"
	"solstice/internal/i18n"
	"strconv"
	"strings"
	"time"
	"unicode"

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
	dir := filepath.Join(s.dataDir, "avatars")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	// Each upload has its own temporary file, so two at once cannot mix.
	tmp, err := os.CreateTemp(dir, strconv.FormatInt(id, 10)+"-*.part")
	if err != nil {
		return err
	}
	_, err = tmp.Write(data)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), s.avatarPath(id))
	}
	if err != nil {
		os.Remove(tmp.Name())
	}
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

// initialsOf is the letters shown for a name, as initials() in
// frontend/src/core/marks.ts finds them: "Marie Durand" is "MD", "admin" is "AD".
func initialsOf(name string) string {
	words := strings.FieldsFunc(name, func(r rune) bool { return unicode.IsSpace(r) || r == '.' || r == '_' || r == '-' })
	words = slices.DeleteFunc(words, func(w string) bool {
		return !strings.ContainsFunc(w, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsNumber(r) })
	})
	switch len(words) {
	case 0:
		return "?"
	case 1:
		rs := []rune(words[0])
		return strings.ToUpper(string(rs[:min(2, len(rs))]))
	}
	first, last := []rune(words[0]), []rune(words[len(words)-1])
	return strings.ToUpper(string(first[:1]) + string(last[:1]))
}

// initialsSVG draws initials like the interface does where a person has no
// picture: the same colours, font and proportions as a person's mark, whose
// box (18 units inside its border) the picture fills. The dark colours follow
// the system's setting, the only one an image can see.
func initialsSVG(r *http.Request, name string) []byte {
	size := avatarSize(r)
	return fmt.Appendf(nil, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 18 18">`+
		`<style>@media (prefers-color-scheme: dark) { rect { fill: #22211d } text { fill: #c4beb0 } }</style>`+
		`<rect width="18" height="18" fill="#faf8f3"/><text x="9" y="9" dy=".35em" text-anchor="middle" `+
		`font-family="'IBM Plex Mono', ui-monospace, monospace" font-size="9.5" font-weight="500" `+
		`letter-spacing=".02em" fill="#4a4740">%s</text></svg>`, size, size, html.EscapeString(initialsOf(name)))
}

// serveAvatar answers with an avatar that the browser may keep but must check
// with the server before showing, so a change is seen at once. The ETag lets
// the check end in a 304.
func serveAvatar(w http.ResponseWriter, r *http.Request, contentType, etag string, body io.ReadSeeker) {
	h := w.Header()
	h.Set("Content-Type", contentType)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
	h.Set("Cache-Control", "private, no-cache")
	h.Set("ETag", etag)
	http.ServeContent(w, r, "", time.Time{}, body)
}

// getAvatar serves a person's picture, or their initials when they have none.
func (s *Server) getAvatar(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "userId")
	if err != nil {
		return err
	}
	var name string
	err = s.db.QueryRowContext(r.Context(), `SELECT display_name FROM users WHERE id = ?`, id).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return errNotFound
	}
	if err != nil {
		return err
	}
	f, err := os.Open(s.avatarPath(id))
	if errors.Is(err, os.ErrNotExist) {
		svg := initialsSVG(r, name)
		sum := sha256.Sum256(svg)
		serveAvatar(w, r, "image/svg+xml", `"`+hex.EncodeToString(sum[:16])+`"`, bytes.NewReader(svg))
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	serveAvatar(w, r, "image/png", fmt.Sprintf(`"%x-%x"`, info.ModTime().UnixNano(), info.Size()), f)
	return nil
}
