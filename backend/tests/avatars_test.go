package tests

// Profile pictures: upload, re-encoding, serving and removal.

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
)

func picturePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 90, 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// putPicture sends a file as multipart, as the interface does.
func putPicture(t *testing.T, c *client, path string, data []byte) (int, []byte) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "avatar.png")
	fw.Write(data)
	mw.Close()
	req, _ := http.NewRequest("PUT", c.base+path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set(csrfHeader, "1")
	resp, err := c.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, out
}

func getPicture(t *testing.T, c *client, id int64, setup func(*http.Request)) (int, http.Header, []byte) {
	t.Helper()
	return call(t, c, c.base, "GET", fmt.Sprintf("/api/avatars/%d", id), nil, setup)
}

func TestAvatarLifecycle(t *testing.T) {
	admin := login(t, "admin", "admin")
	id, username := newAccount(t, admin, "une phrase assez longue")
	me := login(t, username, "une phrase assez longue")
	other := login(t, "paul.martin", "motdepasse")

	// Without a picture: the initials.
	status, h, body := getPicture(t, other, id, nil)
	if status != 200 || !strings.HasPrefix(h.Get("Content-Type"), "image/svg+xml") || !bytes.Contains(body, []byte("<svg")) {
		t.Errorf("no picture: %d %s %.60s", status, h.Get("Content-Type"), body)
	}
	if status, _, _ := getPicture(t, anonymous(t), id, nil); status != 401 {
		t.Errorf("signed out: %d", status)
	}
	if status, _, _ := getPicture(t, other, 999999999, nil); status != 404 {
		t.Errorf("unknown user: %d", status)
	}

	// Upload: extra bytes after the picture are not kept.
	trailer := "<script>alert(1)</script>"
	if status, body := putPicture(t, me, "/api/me/avatar", append(picturePNG(t, 120, 90), trailer...)); status != 204 {
		t.Fatalf("upload: %d %s", status, body)
	}
	status, h, body = getPicture(t, other, id, nil)
	if status != 200 || h.Get("Content-Type") != "image/png" || h.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("picture: %d %v", status, h)
	}
	if cc := h.Get("Cache-Control"); !strings.HasPrefix(cc, "private, max-age=") {
		t.Errorf("Cache-Control = %q", cc)
	}
	if bytes.Contains(body, []byte(trailer)) {
		t.Error("the trailing bytes were stored")
	}
	if img, err := png.Decode(bytes.NewReader(body)); err != nil || img.Bounds().Dx() != 120 || img.Bounds().Dy() != 90 {
		t.Errorf("stored picture: %v %v", err, img)
	}
	etag := h.Get("ETag")
	if etag == "" {
		t.Fatal("no ETag")
	}
	if status, _, _ := getPicture(t, other, id, header("If-None-Match", etag)); status != 304 {
		t.Errorf("revalidation: %d", status)
	}

	// Refusals leave the picture as it is.
	for name, c := range map[string]struct {
		data []byte
		want int
	}{
		"text":      {[]byte("not a picture"), 400},
		"too large": {picturePNG(t, 513, 513), 400},
		"too heavy": {make([]byte, 1<<20+10), 413},
	} {
		if status, body := putPicture(t, me, "/api/me/avatar", c.data); status != c.want {
			t.Errorf("%s: %d %s, want %d", name, status, body, c.want)
		}
	}
	if status, _ := me.do("PUT", "/api/me/avatar", map[string]string{"a": "b"}); status != 400 {
		t.Errorf("JSON body: %d", status)
	}
	if _, _, again := getPicture(t, other, id, nil); !bytes.Equal(again, body) {
		t.Error("a refused upload changed the picture")
	}

	// Remove: the initials come back.
	me.call("DELETE", "/api/me/avatar", nil, 204, nil)
	me.call("DELETE", "/api/me/avatar", nil, 204, nil)
	if status, h, _ := getPicture(t, other, id, nil); status != 200 || !strings.HasPrefix(h.Get("Content-Type"), "image/svg+xml") {
		t.Errorf("after removal: %d %s", status, h.Get("Content-Type"))
	}
}

func TestAdminAvatar(t *testing.T) {
	admin := login(t, "admin", "admin")
	paul := login(t, "paul.martin", "motdepasse")
	id, username := newAccount(t, admin, "une phrase assez longue")
	path := fmt.Sprintf("/api/admin/users/%d/avatar", id)

	if status, _ := putPicture(t, paul, path, picturePNG(t, 40, 40)); status != 403 {
		t.Errorf("a user set another's picture: %d", status)
	}
	paul.call("DELETE", path, nil, 403, nil)
	if status, _ := putPicture(t, admin, "/api/admin/users/999999999/avatar", picturePNG(t, 40, 40)); status != 404 {
		t.Errorf("unknown account: %d", status)
	}
	if status, body := putPicture(t, admin, path, picturePNG(t, 40, 40)); status != 204 {
		t.Fatalf("admin upload: %d %s", status, body)
	}
	if status, h, _ := getPicture(t, paul, id, nil); status != 200 || h.Get("Content-Type") != "image/png" {
		t.Errorf("picture: %d %s", status, h.Get("Content-Type"))
	}
	if d := pick(t, `SELECT COALESCE(details, '') FROM audit_log WHERE action = 'user.update' AND target_label = ?
		ORDER BY id DESC LIMIT 1`, username); !strings.Contains(d, `"avatar":"set"`) {
		t.Errorf("audit details: %q", d)
	}

	admin.call("DELETE", path, nil, 204, nil)
	if status, h, _ := getPicture(t, paul, id, nil); status != 200 || !strings.HasPrefix(h.Get("Content-Type"), "image/svg+xml") {
		t.Errorf("after removal: %d %s", status, h.Get("Content-Type"))
	}
	if d := pick(t, `SELECT COALESCE(details, '') FROM audit_log WHERE action = 'user.update' AND target_label = ?
		ORDER BY id DESC LIMIT 1`, username); !strings.Contains(d, `"avatar":"remove"`) {
		t.Errorf("audit details: %q", d)
	}
}
