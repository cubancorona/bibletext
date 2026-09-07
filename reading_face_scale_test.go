package bibletext

// The optical scale is arithmetic on two numbers that are properties of font
// binaries, not preferences — so the binaries are what these tests read. They
// measure the OUTLINE rather than the OS/2 x-height field, because the two
// disagree: the shipped face records 0.415 in OS/2 where its 'x' actually draws
// to 0.418, and the reference face's italic is out by a similar margin. Trusting
// the recorded field would move the scale by enough to see.
//
// Each check carries a decoy, because a tolerance loose enough to pass anything
// is the failure mode these tests exist to prevent.

import (
	"math"
	"os"
	"testing"

	"fyne.io/fyne/v2/test"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// outlineXHeightEm measures the height of 'x' above the baseline as a fraction
// of the em, from the glyph outline.
func outlineXHeightEm(t *testing.T, ttf []byte) float64 {
	t.Helper()
	return outlineHeightEm(t, ttf, 'x')
}

func outlineHeightEm(t *testing.T, ttf []byte, r rune) float64 {
	t.Helper()
	f, err := sfnt.Parse(ttf)
	if err != nil {
		t.Fatalf("the shipped face does not parse: %v", err)
	}
	var b sfnt.Buffer
	upem := f.UnitsPerEm()
	idx, err := f.GlyphIndex(&b, r)
	if err != nil || idx == 0 {
		t.Fatalf("the shipped face has no glyph for %q (idx %d, err %v)", r, idx, err)
	}
	// Asking for a ppem equal to the em delivers the outline in font units.
	bounds, _, err := f.GlyphBounds(&b, idx, fixed.Int26_6(upem)<<6, 0)
	if err != nil {
		t.Fatalf("could not measure %q: %v", r, err)
	}
	// Y grows downward here, so the top of the glyph is the smallest Y.
	top := -float64(bounds.Min.Y) / 64
	return top / float64(upem)
}

// meanLowercaseAdvanceEm is the quantity the measure argument rests on: it is
// what decides how many characters fit a line of a given physical width.
func meanLowercaseAdvanceEm(t *testing.T, ttf []byte) float64 {
	t.Helper()
	f, err := sfnt.Parse(ttf)
	if err != nil {
		t.Fatalf("the shipped face does not parse: %v", err)
	}
	var b sfnt.Buffer
	upem := float64(f.UnitsPerEm())
	total := 0.0
	for r := 'a'; r <= 'z'; r++ {
		idx, err := f.GlyphIndex(&b, r)
		if err != nil || idx == 0 {
			t.Fatalf("the shipped face is missing %q — the subset dropped a letter", r)
		}
		adv, err := f.GlyphAdvance(&b, idx, fixed.Int26_6(f.UnitsPerEm())<<6, 0)
		if err != nil {
			t.Fatalf("could not measure the advance of %q: %v", r, err)
		}
		total += float64(adv) / 64 / upem
	}
	return total / 26
}

func TestOpticalScaleMatchesTheShippedFace(t *testing.T) {
	const tol = 0.002 // two thousandths of an em; the decoys below are further off

	got := outlineXHeightEm(t, readingFontRegular)
	if math.Abs(got-readingXHeightEm) > tol {
		t.Fatalf("the shipped face draws 'x' to %.4f em, but readingXHeightEm says %.4f.\n"+
			"Rebuilding the subset changed the face's proportions: re-measure and update the\n"+
			"constant, or the reading size will be wrong on every surface at once.", got, readingXHeightEm)
	}

	// The control. If the tolerance were loose enough to swallow a wrong constant
	// the check above would pass for anything, so prove it would not: the OS/2
	// field this test deliberately does not read is itself outside the band.
	const recordedInOSTwo = 0.415
	if math.Abs(got-recordedInOSTwo) <= tol {
		t.Fatalf("the outline and the recorded OS/2 x-height agree to within the tolerance, "+
			"so this test can no longer tell a measured constant from a recorded one "+
			"(outline %.4f, recorded %.4f, tolerance %.4f)", got, recordedInOSTwo, tol)
	}
}

// The four cuts must agree closely, or a size chosen for the regular would leave
// bold or italic visibly out of step with the text around them.
func TestTheCutsAgreeOnTheirXHeight(t *testing.T) {
	cuts := map[string][]byte{
		"regular":     readingFontRegular,
		"italic":      readingFontItalic,
		"bold":        readingFontBold,
		"bold italic": readingFontBoldItalic,
	}
	for name, ttf := range cuts {
		got := outlineHeightEm(t, ttf, 'x')
		if math.Abs(got-readingXHeightEm) > 0.005 {
			t.Errorf("the %s cut draws 'x' to %.4f em against the family's %.4f — "+
				"too far apart to set at one size", name, got, readingXHeightEm)
		}
	}
}

// The claim the whole design rests on: scaling the glyphs by the x-height ratio
// while holding the measure at its old physical width leaves the number of
// characters on a line where it was. That is only true because the advance ratio
// and the x-height ratio are very nearly reciprocal, which is a fact about these
// two particular faces and has to be checked rather than assumed.
func TestHoldingTheMeasureRestoresTheOldLineLength(t *testing.T) {
	georgia := referenceFaceOrSkip(t)

	advNew := meanLowercaseAdvanceEm(t, readingFontRegular)
	advOld := meanLowercaseAdvanceEm(t, georgia)
	xOld := outlineHeightEm(t, georgia, 'x')

	scale := xOld / outlineXHeightEm(t, readingFontRegular)

	// Characters on a line of fixed physical width, new against old.
	ratio := (advOld * readingBodyBase) / (advNew * readingBodyBase * scale)
	if math.Abs(ratio-1) > 0.01 {
		t.Fatalf("holding the measure would change the line length by %.1f%% — the optical "+
			"scale and the advance ratio are no longer reciprocal, so the measure can no "+
			"longer be left alone (scale %.4f, advances %.4f → %.4f)",
			(ratio-1)*100, scale, advOld, advNew)
	}

	// And the reference constant itself, while a copy of that face is in reach.
	if math.Abs(xOld-referenceXHeightEm) > 0.002 {
		t.Errorf("referenceXHeightEm says %.4f but the reference face draws 'x' to %.4f",
			referenceXHeightEm, xOld)
	}
}

// referenceFaceOrSkip returns the face the reading surfaces set before the
// change. It is a licensed system font, so it cannot be shipped or vendored and
// the check that needs it is skipped where it is absent rather than faked.
func referenceFaceOrSkip(t *testing.T) []byte {
	t.Helper()
	for _, p := range []string{
		"/System/Library/Fonts/Supplemental/Georgia.ttf",
		"/Library/Fonts/Georgia.ttf",
		"C:\\Windows\\Fonts\\georgia.ttf",
	} {
		if b, err := os.ReadFile(p); err == nil && len(b) > 0 {
			return b
		}
	}
	t.Skip("the reference face is not installed on this machine")
	return nil
}

// The scale is only ever right for the face it was measured from, so guard the
// arithmetic itself.
func TestOpticalScaleArithmetic(t *testing.T) {
	got := readingOpticalScale()
	want := referenceXHeightEm / readingXHeightEm
	if got != want {
		t.Fatalf("readingOpticalScale() = %v, want %v", got, want)
	}
	if got < 1.10 || got > 1.20 {
		t.Fatalf("the optical scale came out at %.4f, which is far enough from the measured "+
			"1.1518 that one of the two constants has been edited without the other", got)
	}
	// Geometry must not move when the face does.
	if readingReferencePx() != readingBodyBase*readingTextScale() {
		t.Fatal("readingReferencePx has picked up the optical scale — every measure, indent " +
			"and margin figured from it would widen by 15%")
	}
	if math.Abs(readingGlyphPx()-readingReferencePx()*got) > 1e-9 {
		t.Fatal("readingGlyphPx is not the reference size times the optical scale")
	}
}

// The desktop canvas pane keeps two sizes and it matters which is which: the
// type is SET at one and the column is MEASURED at the other. A pane that lost
// the distinction would still lay out and still pass the reporter gate — it
// would just quietly widen the column by 15% and hand the line back the extra
// characters the narrower face gave it.
func TestTheCanvasPaneKeepsTheMeasureOffTheOpticalScale(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Settings().SetTheme(&bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()})

	st := reporterTestState()
	p := newStyledReadingPane(st, st.Bible.GetChapter("Romans", 8))

	if p.refSize <= 0 || p.textSize <= 0 {
		t.Fatalf("the pane came up with sizes %v/%v", p.refSize, p.textSize)
	}
	want := p.refSize * float32(readingOpticalScale())
	if math.Abs(float64(p.textSize-want)) > 1e-4 {
		t.Errorf("the type is set at %v; the reference %v times the optical scale is %v",
			p.textSize, p.refSize, want)
	}
	// The control: the two must not have collapsed into the same number, or the
	// check above passes on a pane that applies no correction at all.
	if p.textSize == p.refSize {
		t.Fatal("the set size and the reference size are equal — the pane is applying no " +
			"optical correction, and every assertion about the split is vacuous")
	}
	if got := p.referenceSize(); got != p.refSize {
		t.Errorf("referenceSize() returned %v, not the stored %v", got, p.refSize)
	}

	// A pane built bare (the tests do this) must still back the reference out of
	// the set size rather than reading zero, which would put every pane into the
	// reporter layout.
	bare := &styledReadingPane{textSize: 24}
	if got, want := bare.referenceSize(), float32(24)/float32(readingOpticalScale()); math.Abs(float64(got-want)) > 1e-4 {
		t.Errorf("a bare pane's reference size is %v, want %v", got, want)
	}
	if (&styledReadingPane{}).referenceSize() <= 0 {
		t.Error("a wholly empty pane reports a zero reference size, which reads as an " +
			"infinitely narrow column")
	}
}
