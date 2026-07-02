package bibletext

// Now-Playing artwork for the iOS lock screen / Control Center. It reuses the
// "Share as image" renderer's helpers (share_image.go: paintGradient,
// drawCentered, newFace, wrapText, schemeForRef) to draw a square card showing the
// book + chapter (e.g. "John 20") with the translation name beneath — the same
// calm text-on-colour treatment as a shared verse card, no imagery.
//
// Font bytes are passed in (not pulled from AppState) so the caller can render off
// the UI goroutine without a data race on the live state.

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

// renderChapterArtwork writes a square artwork PNG to a temp file and returns its
// path. title is "Book Chapter" (e.g. "John 20"); subtitle is the translation name.
func renderChapterArtwork(title, subtitle string, regularTTF, boldTTF []byte) (string, error) {
	const (
		dim    = 1024
		margin = 110
	)
	bold, err := opentype.Parse(boldTTF)
	if err != nil {
		return "", err
	}
	regular, err := opentype.Parse(regularTTF)
	if err != nil {
		regular = bold
	}

	sc := schemeForRef(title+"|"+subtitle, 0)
	img := image.NewRGBA(image.Rect(0, 0, dim, dim))
	paintGradient(img, sc.top, sc.bottom)

	contentW := dim - 2*margin

	// Title: the largest bold size that fits, wrapping a long book name to at most
	// two lines.
	var face font.Face
	var lines []string
	var lineH int
	for pt := 150; pt >= 48; pt -= 4 {
		f := newFace(bold, float64(pt))
		ls := wrapText(f, title, contentW)
		lh := int(float64(pt) * 1.2)
		if len(ls) <= 2 && len(ls)*lh <= dim*55/100 {
			face, lines, lineH = f, ls, lh
			break
		}
	}
	if face == nil {
		pt := 48
		face = newFace(bold, float64(pt))
		lines = wrapText(face, title, contentW)
		lineH = int(float64(pt) * 1.2)
	}

	blockH := len(lines) * lineH
	y := (dim-blockH)/2 + lineH*3/4
	for _, line := range lines {
		drawCentered(img, face, line, sc.text, dim, y)
		y += lineH
	}

	// Translation name, smaller, in the accent colour, below the title.
	if s := strings.TrimSpace(subtitle); s != "" {
		subFace := newFace(regular, 40)
		for pt := 40; pt >= 22; pt -= 2 {
			subFace = newFace(regular, float64(pt))
			if font.MeasureString(subFace, s).Ceil() <= contentW {
				break
			}
		}
		drawCentered(img, subFace, s, sc.accent, dim, y+48)
	}

	safe := strings.NewReplacer(" ", "_", "/", "_", ":", "_").Replace(title)
	path := filepath.Join(os.TempDir(), fmt.Sprintf("bibletext-artwork-%s.png", safe))
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return "", err
	}
	return path, nil
}
