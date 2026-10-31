package devseed

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
)

// texts are the demo text attachments, by name.
var texts = map[string]string{
	"preview-notes.txt": "PDF preview\n\n- Render the first page as an image.\n- Fall back to a download link.\n",
	"release-checklist.txt": "Release checklist\n\n1. Freeze the branch.\n2. Run the full test suite.\n" +
		"3. Tag the version.\n4. Write the release notes.\n",
	"crash-log.txt": "FATAL EXCEPTION: main\njava.lang.NullPointerException: settings.theme\n" +
		"    at app.Splash.onCreate(Splash.kt:42)\n",
}

// attachmentFile gives a demo attachment's type and content: a text above,
// or a drawn picture for a .png.
func attachmentFile(name string) (mimeType string, data []byte, err error) {
	if t, ok := texts[name]; ok {
		return "text/plain", []byte(t), nil
	}
	if name != "board-overflow.png" {
		return "", nil, fmt.Errorf("no content for the attachment %s", name)
	}
	// Five board columns, the last one cut off by the right edge.
	img := image.NewRGBA(image.Rect(0, 0, 390, 240))
	for y := range 240 {
		for x := range 390 {
			c := color.RGBA{0xf4, 0xf5, 0xf7, 0xff}
			if col := x % 100; col >= 10 && y >= 20 && y < 220 {
				c = color.RGBA{0xdf, 0xe1, 0xe6, 0xff}
				if y >= 40 && (y-40)%45 < 35 && col >= 18 && col < 92 {
					c = color.RGBA{0xff, 0xff, 0xff, 0xff}
				}
			}
			img.Set(x, y, c)
		}
	}
	var b bytes.Buffer
	err = png.Encode(&b, img)
	return "image/png", b.Bytes(), err
}
