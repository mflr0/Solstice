package jirafixture

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"path/filepath"
	"strings"
)

// attachmentContent returns deterministic, valid file content for an
// attachment, so the UI can render real thumbnails and downloads.
func attachmentContent(id int64, filename string) []byte {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".png":
		var buf bytes.Buffer
		_ = png.Encode(&buf, testCard(id))
		return buf.Bytes()
	case ".jpg", ".jpeg":
		var buf bytes.Buffer
		_ = jpeg.Encode(&buf, testCard(id), &jpeg.Options{Quality: 80})
		return buf.Bytes()
	case ".pdf":
		return minimalPDF(fmt.Sprintf("Specification - piece jointe %d", id))
	case ".csv":
		var b strings.Builder
		b.WriteString("date;client;montant\n")
		for i := range 20 + id%15 {
			fmt.Fprintf(&b, "2026-03-%02d;CLI-%04d;%d,%02d\n", i%28+1, (id*7+i)%10000, (id*13+i*17)%900+10, (id+i)%100)
		}
		return []byte(b.String())
	default:
		var b strings.Builder
		for i := range 30 + id%20 {
			fmt.Fprintf(&b, "2026-03-02 10:%02d:%02d INFO  [worker-%d] lot %d traité en %d ms\n", i%60, (i*7)%60, i%4, id+i, (id*31+i*11)%900+12)
		}
		return []byte(b.String())
	}
}

// testCard draws a small picture whose colours depend on id.
func testCard(id int64) image.Image {
	const w, h = 96, 60
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	bg := color.RGBA{uint8(200 + id%40), uint8(190 + id*3%50), uint8(170 + id*7%60), 255}
	ink := color.RGBA{uint8(id * 37 % 200), uint8(id * 53 % 120), uint8(id * 71 % 160), 255}
	for y := range h {
		for x := range w {
			c := bg
			if (x+y+int(id))%17 < 3 || y > h-8 {
				c = ink
			}
			img.Set(x, y, c)
		}
	}
	return img
}

// minimalPDF builds a one-page PDF showing text (ASCII only).
func minimalPDF(text string) []byte {
	stream := fmt.Sprintf("BT /F1 18 Tf 72 770 Td (%s) Tj ET", text)
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects))
	for i, o := range objects {
		offsets[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, off := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return b.Bytes()
}
