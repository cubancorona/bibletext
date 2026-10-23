package painter_test

// Proof for patches/fyne-2.7.4-fallback-space.patch. setup-fyne-patch.sh
// copies it into internal/painter as bt_fallback_space_test.go; it lives in
// patches/testdata/ so the go tool ignores it here.

import (
	"bytes"
	"image"
	"image/color"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/go-text/typesetting/di"
	"github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/math/fixed"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/internal/painter"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// btFaces resolves a rune to the first face that has it, else the first face,
// as the toolkit's own font map does before it looks at system fonts. The
// faces are all bundled, so the result does not depend on the host.
type btFaces []*font.Face

func (f btFaces) ResolveFace(r rune) *font.Face {
	for _, face := range f {
		if _, ok := face.NominalGlyph(r); ok {
			return face
		}
	}
	return f[0]
}

func btFace(t *testing.T, res fyne.Resource) *font.Face {
	t.Helper()
	face, err := font.ParseTTF(bytes.NewReader(res.Content()))
	if err != nil {
		t.Fatal(err)
	}
	return face
}

// btTextEmoji is the default text face, then the bundled emoji face.
func btTextEmoji(t *testing.T) (btFaces, *font.Face) {
	t.Helper()
	emoji := btFace(t, theme.DefaultEmojiFont())
	return btFaces{btFace(t, theme.DefaultTextFont()), emoji}, emoji
}

// btWithSymbols adds the bundled symbol face, which has U+2318 (place of
// interest sign) and no U+0020: a fallback face that cannot draw a space.
func btWithSymbols(t *testing.T) btFaces {
	t.Helper()
	faces, _ := btTextEmoji(t)
	symbols := btFace(t, theme.DefaultSymbolFont())
	if _, ok := symbols.NominalGlyph(' '); ok {
		t.Fatal("the symbol face has a space; this test needs a face without one")
	}
	for _, face := range faces {
		if _, ok := face.NominalGlyph('\u2318'); ok {
			t.Fatal("an earlier face has U+2318; the symbol face would not be reached")
		}
	}
	if _, ok := symbols.NominalGlyph('\u2318'); !ok {
		t.Fatal("the symbol face lacks U+2318")
	}
	return append(faces, symbols)
}

// btWithMono adds the bundled monospace face, which has U+E0A0 (a powerline
// symbol the earlier faces lack) and a space much wider than the text face's:
// a fallback face whose own space is the wrong width for the text.
func btWithMono(t *testing.T) btFaces {
	t.Helper()
	faces, _ := btTextEmoji(t)
	mono := btFace(t, theme.DefaultTextMonospaceFont())
	for _, face := range faces {
		if _, ok := face.NominalGlyph('\ue0a0'); ok {
			t.Fatal("an earlier face has U+E0A0; the monospace face would not be reached")
		}
	}
	if _, ok := mono.NominalGlyph('\ue0a0'); !ok {
		t.Fatal("the monospace face lacks U+E0A0")
	}
	return append(faces, mono)
}

// btLastResort adds macOS's LastResort face, which the system's font lookup
// gives a character no other face has. It has a glyph for every character,
// a space among them, and draws each as a placeholder.
func btLastResort(t *testing.T) btFaces {
	t.Helper()
	data, err := os.ReadFile("/System/Library/Fonts/LastResort.otf")
	if err != nil {
		t.Skip("no LastResort face on this host")
	}
	last, err := font.ParseTTF(bytes.NewReader(data))
	if err != nil {
		t.Skip("the LastResort face does not parse here")
	}
	faces, _ := btTextEmoji(t)
	return append(faces, last)
}

func btWidth(faces shaping.Fontmap, s string, size float32) float32 {
	w, _ := painter.MeasureString(faces, s, size, fyne.TextStyle{})
	return w.Width
}

func btNear(a, b float32) bool { return math.Abs(float64(a-b)) < 0.01 }

func TestBTFontSpaceAfterEmojiKeepsItsWidth(t *testing.T) {
	faces, emoji := btTextEmoji(t)
	const size = 18
	space := btWidth(faces, "a b", size) - btWidth(faces, "ab", size)
	if space <= 1 {
		t.Fatalf("a space in the text face measures %v", space)
	}
	// The cause: the emoji face has a space glyph with no bitmap, and the
	// shaper gives a glyph it cannot measure no advance.
	if gid, ok := emoji.NominalGlyph(' '); !ok || gid == 0 {
		t.Fatal("the emoji face has no space; the case below would be the fallback case")
	}
	for _, c := range []struct{ with, without string }{
		{"a \U0001F64F b", "a \U0001F64Fb"},
		{"\U0001F64F \U0001F4D6", "\U0001F64F\U0001F4D6"},
		{"Amen \U0001F64F \U0001F4D6 end", "Amen \U0001F64F\U0001F4D6end"},
	} {
		got := btWidth(faces, c.with, size) - btWidth(faces, c.without, size)
		want := space * float32(strings.Count(c.with, " ")-strings.Count(c.without, " "))
		if !btNear(got, want) {
			t.Errorf("%q is %v wider than %q, want %v (one text-face space per space)", c.with, got, c.without, want)
		}
	}
	nbsp := btWidth(faces, "a\u00a0b", size) - btWidth(faces, "ab", size)
	if got := btWidth(faces, "a \U0001F64F\u00a0b", size) - btWidth(faces, "a \U0001F64Fb", size); !btNear(got, nbsp) {
		t.Errorf("a no-break space after an emoji measures %v, want %v", got, nbsp)
	}
	// A space the emoji face is not asked for is untouched.
	if got := btWidth(faces, "a \U0001F64Fb", size) - btWidth(faces, "ab", size); got <= space {
		t.Errorf("the emoji and its leading space measure %v", got)
	}
}

// The same through the toolkit's own font map and widgets: fyne.MeasureText,
// and a Label, as a note's text is shown in the notes browser.
func TestBTFontMeasureTextKeepsTheSpaceAfterAnEmoji(t *testing.T) {
	test.NewTempApp(t)
	size := theme.TextSize()
	width := func(s string) float32 { return fyne.MeasureText(s, size, fyne.TextStyle{}).Width }
	space := width("a b") - width("ab")
	if space <= 1 {
		t.Fatalf("a space measures %v", space)
	}
	if got := width("a \U0001F64F b") - width("a \U0001F64Fb"); !btNear(got, space) {
		t.Errorf("MeasureText: \"a \\U0001F64F b\" is %v wider than \"a \\U0001F64Fb\", want %v", got, space)
	}
	with := widget.NewLabel("Amen \U0001F64F \U0001F4D6 end")
	without := widget.NewLabel("Amen \U0001F64F\U0001F4D6end")
	if got := with.MinSize().Width - without.MinSize().Width; !btNear(got, 2*space) {
		t.Errorf("a label with two spaces after emoji is %v wider than one without, want %v", got, 2*space)
	}
}

func TestBTFontSpaceAfterFallbackFaceIsASpace(t *testing.T) {
	faces := btWithSymbols(t)
	const size = 18
	space := btWidth(faces, "c d", size) - btWidth(faces, "cd", size)
	if got := btWidth(faces, "c \u2318 d", size) - btWidth(faces, "c \u2318d", size); !btNear(got, space) {
		t.Errorf("a space after a symbol-face character measures %v, want %v", got, space)
	}

	mono := btWithMono(t)
	if own := btWidth(btFaces{mono[2]}, "c d", size) - btWidth(btFaces{mono[2]}, "cd", size); math.Abs(float64(own-space)) < 1 {
		t.Fatalf("the monospace face's space (%v) is too close to the text face's (%v) to tell apart", own, space)
	}
	if got := btWidth(mono, "c \ue0a0 d", size) - btWidth(mono, "c \ue0a0d", size); !btNear(got, space) {
		t.Errorf("a space after a monospace-face character measures %v, want the text face's %v", got, space)
	}
}

// btAlone is the advance of r shaped alone in face.
func btAlone(face *font.Face, r rune, size float32) float32 {
	out := (&shaping.HarfbuzzShaper{}).Shape(shaping.Input{
		Text: []rune{r}, RunEnd: 1, Direction: di.DirectionLTR,
		Face: face, Size: fixed.Int26_6(size * 64),
	})
	return float32(out.Advance) / 64
}

// btInkCount is the number of pixels that hold ink.
func btInkCount(img *image.NRGBA) int {
	n := 0
	b := img.Bounds()
	for x := b.Min.X; x < b.Max.X; x++ {
		for y := b.Min.Y; y < b.Max.Y; y++ {
			if img.NRGBAAt(x, y).A > 24 {
				n++
			}
		}
	}
	return n
}

// btInkEnd is one past the last column that holds ink, or 0.
func btInkEnd(img *image.NRGBA) int {
	b := img.Bounds()
	for x := b.Max.X; x > b.Min.X; x-- {
		if btInk(img, x-1, x) {
			return x
		}
	}
	return 0
}

// A glyph no face has is drawn as U+FFFD from the face the font map gives
// that character. It advances by that glyph, once, so the box ends where the
// next glyph starts, inside the measured width.
func TestBTFontMissingGlyphAdvancesByTheBoxItDraws(t *testing.T) {
	const size = 40
	text := btFace(t, theme.DefaultTextFont())
	symbols := btFace(t, theme.DefaultSymbolFont())
	if _, ok := symbols.NominalGlyph(0xfffd); ok {
		t.Fatal("the symbol face has U+FFFD; the second case needs a face without it")
	}
	for _, c := range []struct {
		name  string
		faces btFaces
	}{
		{"in the text face", btFaces{text}},
		// U+E000 resolves to the first face, which has no U+FFFD.
		{"in a face without U+FFFD", btFaces{symbols, text}},
	} {
		t.Run(c.name, func(t *testing.T) {
			for _, face := range c.faces {
				if _, ok := face.NominalGlyph('\ue000'); ok {
					t.Fatal("a face has U+E000; it would not be a missing glyph")
				}
			}
			box := c.faces.ResolveFace(0xfffd)
			if _, ok := box.NominalGlyph(0xfffd); !ok {
				t.Fatal("no face has U+FFFD")
			}
			want := btAlone(box, 0xfffd, size)
			if got := btWidth(c.faces, "\ue000", size); !btNear(got, want) {
				t.Errorf("a missing glyph measures %v, the U+FFFD drawn for it %v", got, want)
			}
			// Each string ends with a box: the next glyph would start at its
			// measured width.
			for _, s := range []string{"\ue000", "a\ue000", "a\ue000\ue000"} {
				w := btWidth(c.faces, s, size)
				if end := btInkEnd(btDraw(c.faces, s, size)); float32(end) > w+0.5 {
					t.Errorf("%q: the box's ink runs to column %d, past the next glyph's start at %v", s, end, w)
				}
			}
		})
	}
}

// A combining mark after a space shares the space's cluster. In a run of a
// fallback face the space is not measured again, so the mark is drawn on it
// and the cluster keeps its shaped width.
func TestBTFontMarkOnASpaceInAFallbackRunIsDrawn(t *testing.T) {
	const size = 40
	text := btFace(t, theme.DefaultTextFont())
	mono := btFace(t, theme.DefaultTextMonospaceFont())
	for _, c := range []struct {
		name          string
		faces         btFaces
		run           *font.Face
		with, without string
	}{
		// U+01C4 and U+0342 are in the text face, not the monospace face,
		// whose space comes first.
		{"left to right", btFaces{mono, text}, text, "\u01c4 \u0342", "\u01c4 "},
		// Arabic and its marks are in the monospace face, not the text face.
		{"right to left", btFaces{text, mono}, mono, "\u0628 \u064e\u0628", "\u0628 \u0628"},
	} {
		t.Run(c.name, func(t *testing.T) {
			for _, r := range c.with {
				if c.faces.ResolveFace(r) != c.run && r != ' ' {
					t.Fatalf("U+%04X does not resolve to the run's face", r)
				}
			}
			if c.faces.ResolveFace(' ') == c.run {
				t.Fatal("the space resolves to the run's face; the case needs another")
			}
			runes := []rune(c.with)
			segs := (&shaping.Segmenter{}).Split(shaping.Input{
				Text: runes, RunEnd: len(runes), Direction: di.DirectionLTR,
				Size: fixed.Int26_6(size * 64),
			}, c.faces)
			if len(segs) != 1 {
				t.Fatalf("%q splits into %d runs, want one", c.with, len(segs))
			}
			shaped := (&shaping.HarfbuzzShaper{}).Shape(segs[0])
			if got, want := btWidth(c.faces, c.with, size), float32(shaped.Advance)/64; !btNear(got, want) {
				t.Errorf("%q measures %v, its shaped width is %v", c.with, got, want)
			}
			with, without := btDraw(c.faces, c.with, size), btDraw(c.faces, c.without, size)
			if btInkCount(with) <= btInkCount(without) {
				t.Errorf("%q draws no more ink than %q: the mark was not drawn", c.with, c.without)
			}
		})
	}
}

// The advances FixDrawnAdvances gives a cut are the ones the painter draws
// with.
func TestBTFontFixDrawnAdvancesMatchesThePainter(t *testing.T) {
	const size = 40
	emojiFaces, _ := btTextEmoji(t)
	symbolFaces := btWithSymbols(t)
	text := btFace(t, theme.DefaultTextFont())
	mono := btFace(t, theme.DefaultTextMonospaceFont())
	for _, c := range []struct {
		faces btFaces
		s     string
	}{
		{emojiFaces, "Amen \U0001F64F \U0001F4D6 end"},
		{symbolFaces, "c \u2318 d"},
		{btFaces{text}, "a\ue000b \ue000\ue000"},
		{btFaces{btFace(t, theme.DefaultSymbolFont()), text}, "a\ue000b"},
		{btFaces{mono, text}, "x \u01c4 \u0342 y"},
	} {
		runes := []rune(c.s)
		shaper := &shaping.HarfbuzzShaper{}
		var sum fixed.Int26_6
		for _, in := range (&shaping.Segmenter{}).Split(shaping.Input{
			Text: runes, RunEnd: len(runes), Direction: di.DirectionLTR,
			Size: fixed.Int26_6(size * 64),
		}, c.faces) {
			out := shaper.Shape(in)
			painter.FixDrawnAdvances(shaper, c.faces, in, &out)
			sum += out.Advance
		}
		if got, want := float32(sum)/64, btWidth(c.faces, c.s, size); math.Abs(float64(got-want)) > 0.05 {
			t.Errorf("%q: the cut's advances total %v, the painter's %v", c.s, got, want)
		}
	}
}

// A space after an emoji is shaped alone once per string, however many there
// are: a string with forty measures with about the allocations of one with one.
func TestBTFontSpacesAfterEmojiAreShapedOncePerString(t *testing.T) {
	faces, _ := btTextEmoji(t)
	one := "\U0001F64F " + strings.Repeat("\U0001F64F", 39)
	forty := strings.Repeat("\U0001F64F ", 40)
	allocs := func(s string) float64 {
		return testing.AllocsPerRun(20, func() { btWidth(faces, s, 18) })
	}
	a1, a40 := allocs(one), allocs(forty)
	t.Logf("allocations: one space %v, forty %v", a1, a40)
	if a40-a1 >= 39 {
		t.Errorf("forty spaces after emoji take %v allocations, one takes %v: each space is shaped again", a40, a1)
	}
}

// btInk reports whether any pixel in columns [x0, x1) holds ink.
func btInk(img *image.NRGBA, x0, x1 int) bool {
	b := img.Bounds()
	if x0 < b.Min.X {
		x0 = b.Min.X
	}
	if x1 > b.Max.X {
		x1 = b.Max.X
	}
	for x := x0; x < x1; x++ {
		for y := b.Min.Y; y < b.Max.Y; y++ {
			if img.NRGBAAt(x, y).A > 24 {
				return true
			}
		}
	}
	return false
}

// btGap is the run of ink-free columns that starts at or after x.
func btGap(img *image.NRGBA, x int) (start, end int) {
	b := img.Bounds()
	start = x
	for start < b.Max.X && btInk(img, start, start+1) {
		start++
	}
	end = start
	for end < b.Max.X && !btInk(img, end, end+1) {
		end++
	}
	return start, end
}

func btDraw(faces shaping.Fontmap, s string, size float32) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 400, 80))
	painter.DrawString(img, s, color.Black, faces, size, 1, fyne.TextStyle{})
	return img
}

// Drawn, the space after the emoji is as wide as a space and holds no ink,
// and the space after a fallback-face character is empty: no missing-glyph box
// and no placeholder.
func TestBTFontSpaceAfterEmojiOrFallbackDrawsAsSpace(t *testing.T) {
	const size = 40
	t.Run("emoji", func(t *testing.T) {
		faces, _ := btTextEmoji(t)
		space := btWidth(faces, "a b", size) - btWidth(faces, "ab", size)
		emojiEnd := int(btWidth(faces, "a \U0001F64F", size))
		// The ink-free gap between the emoji and the letter after it.
		with := btDraw(faces, "a \U0001F64F b", size)
		without := btDraw(faces, "a \U0001F64Fb", size)
		s1, e1 := btGap(with, emojiEnd-4)
		s0, e0 := btGap(without, emojiEnd-4)
		if grown := float32((e1 - s1) - (e0 - s0)); math.Abs(float64(grown-space)) > 1.5 {
			t.Errorf("the space after the emoji widens the drawn gap by %v px, want about %v", grown, space)
		}
	})
	t.Run("fallback", func(t *testing.T) {
		faces := btWithSymbols(t)
		x0 := btWidth(faces, "c \u2318", size)
		x1 := btWidth(faces, "c \u2318 ", size)
		img := btDraw(faces, "c \u2318 d", size)
		if btInk(img, int(math.Ceil(float64(x0)))+1, int(x1)) {
			t.Errorf("ink between %v and %v: the space after U+2318 drew as a glyph", x0, x1)
		}
		if !btInk(img, int(x1), int(x1)+int(size)) {
			t.Error("nothing drawn after the space")
		}
	})
	t.Run("placeholder", func(t *testing.T) {
		faces := btLastResort(t)
		last := faces[len(faces)-1]
		if !btInk(btDraw(btFaces{last}, " ", size), 0, 400) {
			t.Skip("this LastResort face draws its space blank")
		}
		space := btWidth(faces, "c d", size) - btWidth(faces, "cd", size)
		x0 := btWidth(faces, "c \U0001D50A", size)
		x1 := btWidth(faces, "c \U0001D50A ", size)
		if !btNear(x1-x0, space) {
			t.Errorf("the space after U+1D50A measures %v, want the text face's %v", x1-x0, space)
		}
		img := btDraw(faces, "c \U0001D50A d", size)
		if btInk(img, int(math.Ceil(float64(x0)))+1, int(x1)) {
			t.Errorf("ink between %v and %v: the space after U+1D50A drew as a placeholder", x0, x1)
		}
	})
}

// A label cut with an ellipsis counts the space after an emoji as it is drawn.
func TestBTFontTruncationCountsSpaceAfterEmoji(t *testing.T) {
	test.NewTempApp(t)
	const text = "\U0001F64F \U0001F64F \U0001F64F \U0001F64F \U0001F64F \U0001F64F x"
	style := fyne.TextStyle{}
	size := theme.TextSize()
	full := fyne.MeasureText(text, size, style).Width
	rt := widget.NewRichTextWithText(text)
	rt.Truncation = fyne.TextTruncateEllipsis
	pad := theme.InnerPadding()
	rt.Resize(fyne.NewSize(full+2*pad-1, 100))
	var shown []string
	for _, o := range test.WidgetRenderer(rt).Objects() {
		if txt, ok := o.(*canvas.Text); ok {
			shown = append(shown, txt.Text)
		}
	}
	got := strings.Join(shown, "")
	if got == text {
		t.Fatalf("all of %q is shown in a width 1 px narrower than it draws", text)
	}
	if !strings.HasSuffix(got, "\u2026") {
		t.Fatalf("shown %q, want it cut with an ellipsis", got)
	}
}
