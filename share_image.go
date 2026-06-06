package bibletext

// Verse-image rendering for "Share as image". The image is intentionally
// text-on-colour only — an abstract colour field (a soft vertical gradient) with
// the verse and its citation set in the reading serif. No figures, scenes, or
// depictions of any kind: nothing that approaches a graven image (Exodus 20:4).
//
// "Dynamic" means the background gradient, the text/accent colours, and the
// font size all vary: the colour scheme is chosen deterministically from the
// reference (so a given verse looks consistent), and the type auto-sizes to fill
// the card comfortably regardless of the passage length.

import (
	"hash/fnv"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// shareScheme is one abstract colour treatment. All values are plain colours;
// there is deliberately no image content.
type shareScheme struct {
	top    color.NRGBA // gradient top
	bottom color.NRGBA // gradient bottom
	text   color.NRGBA // verse text
	accent color.NRGBA // citation
}

// shareSchemes are calm, high-contrast treatments. The chosen one is picked by a
// stable hash of the reference, so each verse keeps its own look.
var shareSchemes = []shareScheme{
	{color.NRGBA{251, 247, 238, 255}, color.NRGBA{238, 228, 210, 255}, color.NRGBA{42, 38, 32, 255}, color.NRGBA{138, 106, 51, 255}}, // parchment
	{color.NRGBA{27, 42, 74, 255}, color.NRGBA{12, 22, 44, 255}, color.NRGBA{233, 240, 255, 255}, color.NRGBA{201, 214, 255, 255}},   // dusk blue
	{color.NRGBA{20, 36, 28, 255}, color.NRGBA{10, 20, 15, 255}, color.NRGBA{232, 242, 234, 255}, color.NRGBA{183, 224, 194, 255}},   // forest
	{color.NRGBA{42, 27, 51, 255}, color.NRGBA{22, 14, 28, 255}, color.NRGBA{243, 234, 250, 255}, color.NRGBA{224, 201, 255, 255}},   // plum
	{color.NRGBA{36, 31, 27, 255}, color.NRGBA{20, 17, 14, 255}, color.NRGBA{239, 230, 215, 255}, color.NRGBA{215, 179, 119, 255}},   // warm dark
	{color.NRGBA{46, 27, 34, 255}, color.NRGBA{26, 14, 19, 255}, color.NRGBA{251, 234, 240, 255}, color.NRGBA{240, 201, 214, 255}},   // rose
}

func schemeForRef(ref string) shareScheme {
	h := fnv.New32a()
	_, _ = h.Write([]byte(ref))
	return shareSchemes[int(h.Sum32())%len(shareSchemes)]
}

// renderVerseImage writes a square share card to a temp PNG and returns its path.
func renderVerseImage(state *AppState, verseText, citation, abbrev string) (string, error) {
	const (
		dim       = 1080
		marginX   = 120
		topInset  = 150
		botInset  = 230 // room for citation + wordmark
		quoteOpen = "“"
		quoteClos = "”"
	)
	sc := schemeForRef(citation + "|" + abbrev)

	img := image.NewRGBA(image.Rect(0, 0, dim, dim))
	paintGradient(img, sc.top, sc.bottom)

	regular, err := opentype.Parse(serifFontBytes(state, fyne.TextStyle{}))
	if err != nil {
		return "", err
	}
	bold, err := opentype.Parse(serifFontBytes(state, fyne.TextStyle{Bold: true}))
	if err != nil {
		bold = regular
	}

	quoted := quoteOpen + collapseSpaces(verseText) + quoteClos
	contentW := dim - 2*marginX
	maxBlockH := dim - topInset - botInset

	// Auto-size the verse: the largest size whose wrapped block fits.
	var face font.Face
	var lines []string
	var lineH int
	for pt := 66; pt >= 26; pt -= 2 {
		f := newFace(regular, float64(pt))
		ls := wrapText(f, quoted, contentW)
		lh := int(float64(pt) * 1.42)
		if len(ls)*lh <= maxBlockH {
			face, lines, lineH = f, ls, lh
			break
		}
	}
	if face == nil { // extremely long selection: use the smallest size
		pt := 26
		face = newFace(regular, float64(pt))
		lines = wrapText(face, quoted, contentW)
		lineH = int(float64(pt) * 1.42)
	}

	// Vertically centre the verse block in the content area.
	blockH := len(lines) * lineH
	y := topInset + (maxBlockH-blockH)/2 + lineH*3/4
	for _, line := range lines {
		drawCentered(img, face, line, sc.text, dim, y)
		y += lineH
	}

	// Citation, centred a little below the verse block.
	citeFace := newFace(bold, 34)
	citeY := topInset + (maxBlockH+blockH)/2 + 70
	if citeY > dim-150 {
		citeY = dim - 150
	}
	drawCentered(img, citeFace, "— "+citation+" ("+abbrev+")", sc.accent, dim, citeY)

	// Subtle wordmark.
	markFace := newFace(regular, 26)
	markCol := blend(sc.text, sc.bottom, 0.45)
	drawCentered(img, markFace, "BibleText", markCol, dim, dim-70)

	path := filepath.Join(os.TempDir(), "bibletext-verse.png")
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

// serifFontBytes returns the reading serif's TTF bytes, falling back to Fyne's
// bundled font (e.g. on iOS, where no system serif is loaded).
func serifFontBytes(state *AppState, style fyne.TextStyle) []byte {
	if state != nil && state.theme != nil && state.theme.fonts != nil {
		if res := state.theme.fonts.face(style); res != nil {
			if b := res.Content(); len(b) > 0 {
				return b
			}
		}
	}
	return theme.DefaultTheme().Font(style).Content()
}

func newFace(ft *opentype.Font, pt float64) font.Face {
	face, err := opentype.NewFace(ft, &opentype.FaceOptions{Size: pt, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		// Size/DPI are valid constants here; parse already succeeded.
		face, _ = opentype.NewFace(ft, &opentype.FaceOptions{Size: 24, DPI: 72})
	}
	return face
}

// wrapText greedily wraps to the given pixel width using the face's metrics.
func wrapText(face font.Face, s string, maxW int) []string {
	words := strings.Fields(s)
	var lines []string
	cur := ""
	for _, w := range words {
		try := w
		if cur != "" {
			try = cur + " " + w
		}
		if font.MeasureString(face, try).Ceil() <= maxW {
			cur = try
			continue
		}
		if cur != "" {
			lines = append(lines, cur)
		}
		cur = w // a single over-long word still starts its own line
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

// drawCentered draws one line horizontally centred at baseline y.
func drawCentered(dst *image.RGBA, face font.Face, s string, col color.NRGBA, imgW, baseline int) {
	w := font.MeasureString(face, s).Ceil()
	x := (imgW - w) / 2
	d := &font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(col),
		Face: face,
		Dot:  fixed.P(x, baseline),
	}
	d.DrawString(s)
}

func paintGradient(img *image.RGBA, top, bottom color.NRGBA) {
	b := img.Bounds()
	h := b.Dy()
	for y := 0; y < h; y++ {
		t := float64(y) / float64(h-1)
		c := blend(top, bottom, t)
		for x := 0; x < b.Dx(); x++ {
			img.SetRGBA(x, y, color.RGBA{c.R, c.G, c.B, 255})
		}
	}
}

// blend linearly interpolates a->b by t in [0,1].
func blend(a, b color.NRGBA, t float64) color.NRGBA {
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	lerp := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t) }
	return color.NRGBA{lerp(a.R, b.R), lerp(a.G, b.G), lerp(a.B, b.B), 255}
}

// collapseSpaces flattens runs of whitespace (incl. newlines) to single spaces.
func collapseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
