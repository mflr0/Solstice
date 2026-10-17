package api

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func testPicture(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 120, 255})
		}
	}
	return img
}

func encodePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, testPicture(w, h)); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// pngChunk is a PNG chunk with its length and checksum.
func pngChunk(kind string, data []byte) []byte {
	out := binary.BigEndian.AppendUint32(nil, uint32(len(data)))
	out = append(out, kind...)
	out = append(out, data...)
	return binary.BigEndian.AppendUint32(out, crc32.ChecksumIEEE(append([]byte(kind), data...)))
}

// statusOf is the HTTP status of an error for the user.
func statusOf(t *testing.T, err error) int {
	t.Helper()
	var he *httpError
	if !errors.As(err, &he) {
		t.Fatalf("not an error for the user: %v", err)
	}
	return he.status
}

func TestReencodeAvatarPNG(t *testing.T) {
	out, err := reencodeAvatar(encodePNG(t, 64, 32))
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 64 || b.Dy() != 32 {
		t.Errorf("size = %v", b)
	}
	if _, err := reencodeAvatar(encodePNG(t, 512, 512)); err != nil {
		t.Errorf("512×512 refused: %v", err)
	}
}

func TestReencodeAvatarJPEG(t *testing.T) {
	var in bytes.Buffer
	if err := jpeg.Encode(&in, testPicture(40, 40), nil); err != nil {
		t.Fatal(err)
	}
	out, err := reencodeAvatar(in.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := png.Decode(bytes.NewReader(out)); err != nil {
		t.Errorf("a JPEG is not stored as a PNG: %v", err)
	}
}

// What follows the picture, or sits beside it, is not stored as it came.
func TestReencodeAvatarDropsExtras(t *testing.T) {
	plain := encodePNG(t, 16, 16)
	trailer := []byte("<script>alert(1)</script>")
	meta := append([]byte{}, plain[:33]...) // signature and IHDR
	meta = append(meta, pngChunk("tEXt", []byte("Comment\x00secret"))...)
	meta = append(meta, plain[33:]...)
	for name, in := range map[string][]byte{
		"trailing bytes": append(append([]byte{}, plain...), trailer...),
		"text chunk":     meta,
	} {
		out, err := reencodeAvatar(in)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if bytes.Contains(out, trailer) || bytes.Contains(out, []byte("tEXt")) || bytes.Contains(out, []byte("secret")) {
			t.Errorf("%s: stored as is", name)
		}
		if !bytes.HasSuffix(out, plain[len(plain)-12:]) {
			t.Errorf("%s: does not end with the PNG end chunk", name)
		}
	}
}

func TestReencodeAvatarRefusals(t *testing.T) {
	ihdr := binary.BigEndian.AppendUint32(binary.BigEndian.AppendUint32(nil, 100000), 100000)
	bomb := append([]byte("\x89PNG\r\n\x1a\n"), pngChunk("IHDR", append(ihdr, 8, 2, 0, 0, 0))...)
	full := encodePNG(t, 64, 64)
	for name, c := range map[string]struct {
		in   []byte
		want int
	}{
		"too many bytes":    {make([]byte, avatarMaxBytes+1), http.StatusRequestEntityTooLarge},
		"too wide":          {encodePNG(t, 513, 8), http.StatusBadRequest},
		"too high":          {encodePNG(t, 8, 513), http.StatusBadRequest},
		"huge declared":     {bomb, http.StatusBadRequest},
		"text":              {[]byte("not a picture"), http.StatusBadRequest},
		"empty":             {nil, http.StatusBadRequest},
		"gif":               {[]byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;"), http.StatusBadRequest},
		"svg":               {[]byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), http.StatusBadRequest},
		"truncated picture": {full[:len(full)/2], http.StatusBadRequest},
	} {
		_, err := reencodeAvatar(c.in)
		if err == nil {
			t.Errorf("%s: accepted", name)
			continue
		}
		if got := statusOf(t, err); got != c.want {
			t.Errorf("%s: status %d, want %d", name, got, c.want)
		}
	}
}

// The same cases as initials() in the interface, so a person's letters do not
// change when their picture is removed.
func TestInitialsOf(t *testing.T) {
	for in, want := range map[string]string{
		"Marie Durand":               "MD",
		"Jean-Pierre de la Fontaine": "JF",
		"admin":                      "AD",
		"marie.durand":               "MD",
		"élodie":                     "ÉL",
		"":                           "?",
		" - ":                        "?",
	} {
		if got := initialsOf(in); got != want {
			t.Errorf("initialsOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestInitialsSVG(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/avatars/1?size=small", nil)
	svg := string(initialsSVG(req, "Marie <Durand"))
	for _, want := range []string{`width="24"`, `fill="#faf8f3"`, `fill="#4a4740"`, `font-size="9.5"`, `>M&lt;<`} {
		if !strings.Contains(svg, want) {
			t.Errorf("missing %s in %s", want, svg)
		}
	}
}

// Uploads of the same picture at once each use a temporary file of their own:
// the stored picture is one of them, whole, and nothing is left behind.
func TestSetAvatarConcurrent(t *testing.T) {
	s := &Server{dataDir: t.TempDir()}
	const uploads = 12
	var wg sync.WaitGroup
	errs := make(chan error, uploads)
	for i := range uploads {
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		fw, _ := mw.CreateFormFile("file", "avatar.png")
		fw.Write(encodePNG(t, 100+i, 100+i))
		mw.Close()
		r := httptest.NewRequest("PUT", "/api/me/avatar", &body)
		r.Header.Set("Content-Type", mw.FormDataContentType())
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- s.setAvatar(httptest.NewRecorder(), r, 7)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Error(err)
		}
	}
	f, err := os.Open(s.avatarPath(7))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("stored picture: %v", err)
	}
	if b := img.Bounds(); b.Dx() != b.Dy() || b.Dx() < 100 || b.Dx() >= 100+uploads {
		t.Errorf("stored picture is none of the uploads: %v", b)
	}
	entries, err := os.ReadDir(filepath.Dir(s.avatarPath(7)))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("%d files in the directory, want only the picture", len(entries))
	}
}
