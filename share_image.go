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
	_ "embed"
	"fmt"
	"hash/fnv"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"

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
//
// The count (13) is deliberately COPRIME with the typeface count (7): both
// indices step by one per Regenerate, so the reader walks all 13×7 = 91
// scheme×typeface pairings before any repeats.
var shareSchemes = []shareScheme{
	{color.NRGBA{251, 247, 238, 255}, color.NRGBA{238, 228, 210, 255}, color.NRGBA{42, 38, 32, 255}, color.NRGBA{138, 106, 51, 255}}, // parchment
	{color.NRGBA{27, 42, 74, 255}, color.NRGBA{12, 22, 44, 255}, color.NRGBA{233, 240, 255, 255}, color.NRGBA{201, 214, 255, 255}},   // dusk blue
	{color.NRGBA{20, 36, 28, 255}, color.NRGBA{10, 20, 15, 255}, color.NRGBA{232, 242, 234, 255}, color.NRGBA{183, 224, 194, 255}},   // forest
	{color.NRGBA{42, 27, 51, 255}, color.NRGBA{22, 14, 28, 255}, color.NRGBA{243, 234, 250, 255}, color.NRGBA{224, 201, 255, 255}},   // plum
	{color.NRGBA{36, 31, 27, 255}, color.NRGBA{20, 17, 14, 255}, color.NRGBA{239, 230, 215, 255}, color.NRGBA{215, 179, 119, 255}},   // warm dark
	{color.NRGBA{46, 27, 34, 255}, color.NRGBA{26, 14, 19, 255}, color.NRGBA{251, 234, 240, 255}, color.NRGBA{240, 201, 214, 255}},   // rose
	{color.NRGBA{16, 20, 32, 255}, color.NRGBA{6, 8, 16, 255}, color.NRGBA{244, 238, 224, 255}, color.NRGBA{212, 175, 109, 255}},     // midnight gold
	{color.NRGBA{58, 22, 32, 255}, color.NRGBA{32, 10, 18, 255}, color.NRGBA{250, 238, 236, 255}, color.NRGBA{232, 180, 168, 255}},   // burgundy
	{color.NRGBA{44, 52, 64, 255}, color.NRGBA{24, 29, 38, 255}, color.NRGBA{235, 240, 246, 255}, color.NRGBA{166, 190, 214, 255}},   // slate
	{color.NRGBA{16, 50, 52, 255}, color.NRGBA{8, 26, 28, 255}, color.NRGBA{228, 244, 242, 255}, color.NRGBA{150, 214, 200, 255}},    // deep teal
	{color.NRGBA{70, 40, 28, 255}, color.NRGBA{40, 22, 16, 255}, color.NRGBA{250, 238, 226, 255}, color.NRGBA{236, 178, 128, 255}},   // terracotta dusk
	{color.NRGBA{240, 243, 246, 255}, color.NRGBA{219, 226, 233, 255}, color.NRGBA{30, 36, 44, 255}, color.NRGBA{90, 110, 140, 255}}, // mist (light)
	{color.NRGBA{36, 32, 84, 255}, color.NRGBA{18, 15, 44, 255}, color.NRGBA{238, 238, 252, 255}, color.NRGBA{178, 172, 255, 255}},   // indigo
}

// schemeForRef picks a colour treatment from a stable hash of the reference, so a
// given verse looks consistent — offset by variant, which the share preview bumps
// on each "Regenerate" to cycle through the other treatments.
func schemeForRef(ref string, variant int) shareScheme {
	h := fnv.New32a()
	_, _ = h.Write([]byte(ref))
	idx := (int(h.Sum32()) + variant) % len(shareSchemes)
	if idx < 0 {
		idx += len(shareSchemes)
	}
	return shareSchemes[idx]
}

// --- Share typefaces ---------------------------------------------------------
//
// A small library of elegant, highly readable book serifs for the card, all
// SIL OFL 1.1 (licences: assets/fonts/share/OFL-LICENSES.txt) and EMBEDDED so
// the cards look identical on every platform (the phones have no system serif
// the renderer could reach — previously they fell back to Fyne's default sans).
// Regenerate cycles the typeface together with the colour scheme.

//go:embed assets/fonts/share/Cardo-Regular.ttf
var shareFontCardo []byte

//go:embed assets/fonts/share/Cardo-Bold.ttf
var shareFontCardoBold []byte

//go:embed assets/fonts/share/CrimsonText-Regular.ttf
var shareFontCrimson []byte

//go:embed assets/fonts/share/CrimsonText-SemiBold.ttf
var shareFontCrimsonSemi []byte

//go:embed assets/fonts/share/Spectral-Regular.ttf
var shareFontSpectral []byte

//go:embed assets/fonts/share/Spectral-Bold.ttf
var shareFontSpectralBold []byte

//go:embed assets/fonts/share/LibreBaskerville-Regular.ttf
var shareFontBaskerville []byte

//go:embed assets/fonts/share/Prata-Regular.ttf
var shareFontPrata []byte

//go:embed assets/fonts/share/DMSerifDisplay-Regular.ttf
var shareFontDMSerif []byte

//go:embed assets/fonts/share/Gelasio-Regular.ttf
var shareFontGelasio []byte

// shareTypeface is one card typeface: the verse face plus the citation face
// (a heavier cut where the family ships one; single-weight display families
// reuse the regular — their regular already carries display weight).
type shareTypeface struct {
	name    string
	regular *opentype.Font
	bold    *opentype.Font
}

var (
	shareTypefacesOnce sync.Once
	shareTypefaces     []shareTypeface
)

// loadShareTypefaces parses the embedded faces once. A face that fails to parse
// is skipped (never expected — the assets are fixed — but a corrupt asset must
// degrade to fewer typefaces, not a broken share sheet).
func loadShareTypefaces() []shareTypeface {
	shareTypefacesOnce.Do(func() {
		add := func(name string, reg, bold []byte) {
			r, err := opentype.Parse(reg)
			if err != nil {
				return
			}
			b := r
			if len(bold) > 0 {
				if pb, err := opentype.Parse(bold); err == nil {
					b = pb
				}
			}
			shareTypefaces = append(shareTypefaces, shareTypeface{name, r, b})
		}
		// Gelasio is the metrics-compatible OFL equivalent of Georgia — the card
		// look the app shipped with on desktop, now embedded so EVERY platform
		// has it (the phones never actually had Georgia; they fell back to a sans).
		add("Gelasio", shareFontGelasio, nil)
		add("Cardo", shareFontCardo, shareFontCardoBold)
		add("Crimson Text", shareFontCrimson, shareFontCrimsonSemi)
		add("Spectral", shareFontSpectral, shareFontSpectralBold)
		add("Libre Baskerville", shareFontBaskerville, nil)
		add("Prata", shareFontPrata, nil)
		add("DM Serif Display", shareFontDMSerif, nil)
	})
	return shareTypefaces
}

// typefaceForRef picks the card typeface the same way schemeForRef picks the
// colours — a stable per-verse default, stepped by Regenerate — but from an
// independent hash (the "|face" salt) so verses don't pair the same typeface
// with the same scheme across the whole Bible.
func typefaceForRef(ref string, variant int) (shareTypeface, bool) {
	faces := loadShareTypefaces()
	if len(faces) == 0 {
		return shareTypeface{}, false
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(ref + "|face"))
	idx := (int(h.Sum32()) + variant) % len(faces)
	if idx < 0 {
		idx += len(faces)
	}
	return faces[idx], true
}

// renderVerseImage writes a square share card to a temp PNG and returns its path.
// variant selects the colour treatment (0 = the verse's default; the preview's
// Regenerate increments it).
func renderVerseImage(state *AppState, verseText, citation, version string, variant int) (string, error) {
	const (
		dim      = 1080
		marginX  = 120
		topInset = 150
		botInset = 230 // room for citation + wordmark
	)
	sc := schemeForRef(citation+"|"+version, variant)

	img := image.NewRGBA(image.Rect(0, 0, dim, dim))
	paintGradient(img, sc.top, sc.bottom)

	// The verse + citation faces: one of the embedded share serifs, cycled with
	// the colour scheme by Regenerate. The reading-serif path survives only as a
	// fallback for the never-expected case that every embedded face fails to parse.
	var regular, bold *opentype.Font
	if tf, ok := typefaceForRef(citation+"|"+version, variant); ok {
		regular, bold = tf.regular, tf.bold
	} else {
		var err error
		regular, err = opentype.Parse(serifFontBytes(state, fyne.TextStyle{}))
		if err != nil {
			return "", err
		}
		bold, err = opentype.Parse(serifFontBytes(state, fyne.TextStyle{Bold: true}))
		if err != nil {
			bold = regular
		}
	}

	// verseText is already cleaned + quoted by shareVerse (formatBibleQuote): verse
	// numbers stripped, and outer quotation marks added only when appropriate.
	// Authored POEM LINES arrive as "\n" and are structure, not wrapping — the
	// reading pane and the text share both break there (verseIsPoetic /
	// restoreShareLineBreaks), so a psalm must not be run together into a
	// paragraph here either. Each poem line is wrapped on its own; prose is a
	// single segment, so its layout is unchanged.
	segments := poemSegments(verseText)
	contentW := dim - 2*marginX
	maxBlockH := dim - topInset - botInset

	// Auto-size the verse: the largest size whose wrapped block fits.
	wrapAll := func(f font.Face) []string {
		var out []string
		for _, seg := range segments {
			out = append(out, wrapText(f, seg, contentW)...)
		}
		return out
	}
	var face font.Face
	var lines []string
	var lineH int
	for pt := 66; pt >= cardMinPt; pt -= 2 {
		f := newFace(regular, float64(pt))
		ls := wrapAll(f)
		lh := int(float64(pt) * 1.42)
		if len(ls)*lh <= maxBlockH {
			face, lines, lineH = f, ls, lh
			break
		}
	}
	if face == nil {
		// Longer than the card can hold even at the smallest readable size. The
		// old code drew the full block anyway: it bled off the top AND bottom
		// edges mid-word and the citation printed straight over the verse. Clamp
		// to what fits and MARK the cut — an unmarked truncation would present a
		// severed quotation as a whole one, which is the one thing this pipeline
		// is careful never to do (see addEndOmission).
		pt := cardMinPt
		face = newFace(regular, float64(pt))
		lineH = int(float64(pt) * 1.42)
		lines = clampLinesToCard(wrapAll(face), maxBlockH/lineH, face, contentW, verseText)
	}

	// Vertically centre the verse block in the content area.
	blockH := len(lines) * lineH
	y := topInset + (maxBlockH-blockH)/2 + lineH*3/4
	for _, line := range lines {
		drawCentered(img, face, line, sc.text, dim, y)
		y += lineH
	}

	// Citation, centred a little below the verse block. The translation is spelled
	// out in full (Bluebook style: "(World English Bible)", not "(WEB)"), so the line
	// can be long — shrink the type until it fits the content width rather than
	// overflowing the card edges.
	citeStr := citationLine(citation, version)
	var citeFace font.Face
	for pt := 34; pt >= 20; pt -= 2 {
		citeFace = newFace(bold, float64(pt))
		if font.MeasureString(citeFace, citeStr).Ceil() <= contentW {
			break
		}
	}
	citeY := topInset + (maxBlockH+blockH)/2 + 70
	if citeY > dim-110 {
		citeY = dim - 110
	}
	drawCentered(img, citeFace, citeStr, sc.accent, dim, citeY)

	// A fresh file per variant so the preview's canvas.Image reloads on Regenerate
	// (a stable path would be served from Fyne's image cache).
	path := filepath.Join(os.TempDir(), fmt.Sprintf("bibletext-verse-%d.png", variant))
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
	raw := strings.Fields(s)
	// Re-glue the Bluebook omission marks: Rule 5.3's spaced dots (" . . . ." etc.)
	// must never wrap across lines — law-review practice universally sets them with
	// non-breaking spaces. Fields split them into bare "."/"?"/"!" tokens; fold
	// those back onto the preceding word so each mark wraps as one unit.
	var words []string
	for _, w := range raw {
		if len(words) > 0 && (w == "." || w == "?" || w == "!") {
			words[len(words)-1] += " " + w
			continue
		}
		words = append(words, w)
	}
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

// cardMinPt is the smallest type the card will set. Below this the card stops
// being a shareable image and becomes a wall — and a passage that long is past
// what a square card can carry anyway, which is what clampLinesToCard handles.
const cardMinPt = 20

// clampLinesToCard trims a wrapped block to the lines that actually fit and
// marks the cut with the Bluebook end-of-quote omission (Rule 5.3's spaced
// ellipsis, four-dot form — never the single "…" glyph), restoring the closing
// quotation mark the cut removed. It re-drops words until the marked final line
// measures within the content width, so the mark can never itself overflow.
func clampLinesToCard(lines []string, maxLines int, face font.Face, contentW int, original string) []string {
	if maxLines < 1 {
		maxLines = 1
	}
	if len(lines) <= maxLines {
		return lines
	}
	closing := ""
	if strings.HasSuffix(strings.TrimRight(original, " \t\n"), "”") {
		closing = "”"
	}
	kept := append([]string(nil), lines[:maxLines]...)
	last := kept[len(kept)-1]
	for {
		trimmed := strings.TrimRight(last, " \t")
		// Don't stack the mark on punctuation the cut left dangling.
		trimmed = strings.TrimRight(trimmed, "”’\"'")
		trimmed = strings.TrimRight(trimmed, " ,;:.")
		// The four-dot form, composed exactly as addEndOmission does it: the
		// spaced ellipsis, a space, then the sentence's final punctuation.
		candidate := trimmed + endOmissionEllipsis + " ." + closing
		if trimmed == "" || font.MeasureString(face, candidate).Ceil() <= contentW {
			kept[len(kept)-1] = candidate
			return kept
		}
		// Too wide with the mark attached — give back a word and try again.
		i := strings.LastIndexByte(trimmed, ' ')
		if i <= 0 {
			kept[len(kept)-1] = candidate
			return kept
		}
		last = trimmed[:i]
	}
}

// poemSegments splits a quote into the blocks that must start their own line on
// the card: the authored poem lines. Whitespace WITHIN a line is still collapsed
// (a line may have been wrapped in the source), and blank segments are dropped,
// so prose — which carries no "\n" — yields exactly one segment and lays out
// precisely as it always has.
func poemSegments(s string) []string {
	var out []string
	for _, part := range strings.Split(s, "\n") {
		if p := collapseSpaces(part); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return []string{""}
	}
	return out
}
