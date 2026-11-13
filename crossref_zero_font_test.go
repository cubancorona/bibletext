//go:build !race

package bibletext

import (
	"image/color"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
)

// PLAIN ZERO IN REFERENCES AND PRINTED LINES (S20 Item 21). Standard Atkinson
// Hyperlegible draws '0' with a diagonal bar across its counter; the derived
// plain-zero faces keep the outer oval, stroke weight, and advance widths
// identical while opening the counter cleanly, and the cross-references panel
// applies them to its header citation, card references, muted own-passage
// references, and printed apparatus lines.
func TestCrossRefPlainZeroFacesAndPanelReferences(t *testing.T) {
	th := appThemeForTest(t)

	for _, tc := range []struct {
		name           string
		slashed, plain fyne.Resource
		style          fyne.TextStyle
	}{
		{"regular", th.uiFonts.regular, crossRefPlainZeroRegular(), fyne.TextStyle{}},
		{"bold", th.uiFonts.bold, crossRefPlainZeroBold(), fyne.TextStyle{Bold: true}},
	} {
		if tc.slashed == nil || tc.plain == nil {
			t.Fatalf("%s: slashed=%v plain=%v", tc.name, tc.slashed != nil, tc.plain != nil)
		}
		// Advance widths of digits and citation strings match to the subpixel.
		drv := fyne.CurrentApp().Driver()
		for _, sample := range []string{"0", "0123456789", "Prov. 12:4; 31:10–31", "Psalms 102:25"} {
			sSz, sBase := drv.RenderedTextSize(sample, 16, tc.style, tc.slashed)
			pSz, pBase := drv.RenderedTextSize(sample, 16, tc.style, tc.plain)
			if sSz != pSz || sBase != pBase {
				t.Errorf("%s %q metrics: plain=(%v, %v), slashed=(%v, %v)", tc.name, sample, pSz, pBase, sSz, sBase)
			}
		}

		// Render '0' large enough that its counter center is unambiguous, and
		// verify the standard face has ink across the center while the plain
		// face has ink on all four sides of the ring and none in the center.
		centerHasInk := func(face fyne.Resource) (centerInk, ringInk bool) {
			bg := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
			fg := color.NRGBA{R: 0, G: 0, B: 0, A: 255}
			rect := canvas.NewRectangle(bg)
			tx := canvas.NewText("0", fg)
			tx.TextSize = 96
			tx.FontSource = face
			w := test.NewWindow(container.NewStack(rect, container.NewCenter(tx)))
			w.Resize(fyne.NewSize(160, 160))
			img := w.Canvas().Capture()
			w.Close()

			b := img.Bounds()
			minX, minY, maxX, maxY := b.Max.X, b.Max.Y, b.Min.X, b.Min.Y
			isInk := func(x, y int) bool {
				r, g, bl, _ := img.At(x, y).RGBA()
				return r < 0xC000 || g < 0xC000 || bl < 0xC000
			}
			for y := b.Min.Y; y < b.Max.Y; y++ {
				for x := b.Min.X; x < b.Max.X; x++ {
					if isInk(x, y) {
						if x < minX {
							minX = x
						}
						if x > maxX {
							maxX = x
						}
						if y < minY {
							minY = y
						}
						if y > maxY {
							maxY = y
						}
					}
				}
			}
			if minX >= maxX || minY >= maxY {
				t.Fatalf("%s: rendered '0' produced no ink", tc.name)
			}
			cx, cy := (minX+maxX)/2, (minY+maxY)/2
			// Check a 3x3 patch around the geometric center of the glyph bbox.
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if isInk(cx+dx, cy+dy) {
						centerInk = true
					}
				}
			}
			topInk, botInk, leftInk, rightInk := false, false, false, false
			for y := minY; y <= minY+(maxY-minY)/4; y++ {
				if isInk(cx, y) {
					topInk = true
				}
			}
			for y := maxY - (maxY-minY)/4; y <= maxY; y++ {
				if isInk(cx, y) {
					botInk = true
				}
			}
			for x := minX; x <= minX+(maxX-minX)/4; x++ {
				if isInk(x, cy) {
					leftInk = true
				}
			}
			for x := maxX - (maxX-minX)/4; x <= maxX; x++ {
				if isInk(x, cy) {
					rightInk = true
				}
			}
			return centerInk, topInk && botInk && leftInk && rightInk
		}

		sCenter, sRing := centerHasInk(tc.slashed)
		pCenter, pRing := centerHasInk(tc.plain)
		if !sRing || !sCenter {
			t.Errorf("control (%s slashed '0'): ring=%v center=%v; want true, true", tc.name, sRing, sCenter)
		}
		if !pRing || pCenter {
			t.Errorf("%s plain '0': ring=%v center=%v; want true, false (unslashed counter)", tc.name, pRing, pCenter)
		}
	}

	// Mount a panel with a subtitle citation ("John 3:10"), a card ("Romans 10:9"),
	// an own-passage row ("John 3:10"), and printed lines ("John 3:10; Rom. 10:9; (1 Cor. 10:1)"),
	// and verify every reference and printed line carries its plain-zero face.
	bd := xrefBible(map[string]map[int]int{
		"John": {3: 36}, "Romans": {10: 21}, "1 Corinthians": {10: 33},
	})
	noteText := "John 3:10; Rom. 10:9; (1 Cor. 10:1); Is. 40:10"
	v := &bd.Verses["John"][3][9]
	v.Footnotes = []Footnote{{
		Anchor: 0, Text: noteText, Kind: footnoteKindCrossref, Caller: "-",
		Refs: []NoteRef{
			noteRefAt(t, noteText, "John 3:10", "JHN.3.10"),
			noteRefAt(t, noteText, "Rom. 10:9", "ROM.10.9"),
		},
	}}
	st := &AppState{Bible: bd, CurrentBook: "John", CurrentChapter: 3, CurrentVersion: "nkjv"}
	p := mountSectionPanelAt(t, st, 10, "", fyne.NewSize(402, 874))

	sawSubtitle, sawCardRef, sawSelfRef, sawPrinted := false, false, false, false
	walkTree(p.top, func(o fyne.CanvasObject) {
		if pr, ok := o.(*crossRefPrinted); ok {
			for _, tx := range test.WidgetRenderer(pr).(*crossRefPrintedRenderer).rows {
				if strings.Contains(tx.Text, "1 Cor. 10:1") || strings.Contains(tx.Text, "Is. 40:10") {
					sawPrinted = true
					if tx.FontSource != crossRefPlainZeroRegular() {
						t.Errorf("printed line %q FontSource = %v, want plain-zero regular", tx.Text, tx.FontSource)
					}
				}
			}
			return
		}
		tx, ok := o.(*canvas.Text)
		if !ok {
			return
		}
		switch tx.Text {
		case "John 3:10":
			if tx.TextStyle.Bold {
				sawSubtitle = true
				if tx.FontSource != crossRefPlainZeroBold() {
					t.Errorf("panel subtitle John 3:10 FontSource = %v, want plain-zero bold", tx.FontSource)
				}
			} else {
				sawSelfRef = true
				if tx.FontSource != crossRefPlainZeroRegular() {
					t.Errorf("own-passage row John 3:10 FontSource = %v, want plain-zero regular", tx.FontSource)
				}
			}
		case "Romans 10:9":
			sawCardRef = true
			if tx.FontSource != crossRefPlainZeroBold() {
				t.Errorf("card reference Romans 10:9 FontSource = %v, want plain-zero bold", tx.FontSource)
			}
		}
	})
	if !sawSubtitle || !sawCardRef || !sawSelfRef || !sawPrinted {
		t.Errorf("saw subtitle=%v card=%v self=%v printed=%v; want all four", sawSubtitle, sawCardRef, sawSelfRef, sawPrinted)
	}
}

func treeContains(o fyne.CanvasObject, sub string) bool {
	return strings.Contains(strings.Join(treeTexts(o), "\n"), sub)
}

// SHORT AND LANDSCAPE WINDOWS LET CREDITS AND NOTICE SCROLL WITH THE LIST
// (S20 Item 21). On portrait phones and roomy desktop windows the credits and
// edition notice stay pinned above Close; on landscape phones and short
// desktop windows (ps.Height < crossRefPanelHeight) they move into the foot of
// the scroll so the list keeps vertical reading room, and refit cleanly when a
// desktop window changes height.
func TestShortAndLandscapePanelsScrollCreditsWithTheList(t *testing.T) {
	st, _ := groupListState(t)
	tsk := "John.3.16\tJohn.3.1\t10\n"

	// 1. Portrait phone (402x874): credits and notice stay pinned outside scroll.
	portrait := mountSectionPanelAt(t, st, 16, tsk, fyne.NewSize(402, 874))
	if treeContains(portrait.scroll, "Thomas Nelson") || treeContains(portrait.scroll, "OpenBible.info") {
		t.Error("on 402x874 portrait, credits or notice moved inside scroll; want pinned in footer")
	}
	if !treeContains(portrait.top, "Thomas Nelson") || !treeContains(portrait.top, "OpenBible.info") {
		t.Error("on 402x874 portrait, credits or notice missing from panel")
	}

	// 2. Landscape phone (874x402): credits and notice sit inside scroll, and
	// scroll gets the freed vertical room (> 150pt instead of ~100pt).
	landscape := mountSectionPanelAt(t, st, 16, tsk, fyne.NewSize(874, 402))
	if !treeContains(landscape.scroll, "Thomas Nelson") || !treeContains(landscape.scroll, "OpenBible.info") {
		t.Error("on 874x402 landscape, credits and notice did not move into scroll")
	}
	if landscape.scroll.Size().Height < 150 {
		t.Errorf("on 874x402 landscape, scroll height = %.1f, want >= 150 with unpinned captions", landscape.scroll.Size().Height)
	}
	landscape.scroll.ScrollToBottom()
	landscape.scroll.Refresh()
	if landscape.scroll.Offset.Y <= 0 {
		t.Error("on 874x402 landscape, list did not scroll down to reach credits and notice at the foot")
	}

	// 3. Desktop window resized from 1280x700 -> 1280x420 -> 1280x700 refits
	// captions into scroll when short and back into footer when roomy again.
	desktopSheets(t)
	licenseNKJVForTest(t)
	withPublisherCrossRefs(t, "nkjv", true)
	withCrossRefIndex(t, tsk)
	prevRun, prevLoad := crossRefsRun, crossRefsLoad
	t.Cleanup(func() { crossRefsRun, crossRefsLoad = prevRun, prevLoad })
	crossRefsRun = func(work func()) { work() }
	crossRefsLoad = func() error { return nil }

	dst, dw := desktopWindow(t, fyne.NewSize(1280, 700))
	dst.Bible = st.Bible
	dst.CurrentBook, dst.CurrentChapter, dst.CurrentVersion = "John", 3, "nkjv"
	popup := pickerPopup(t, dst, func(s *AppState) { showCrossRefs(s, "", selSpan{lo: 16, hi: 16}) })
	defer popup.Hide()

	scroll := findScroll(popup)
	if scroll == nil {
		t.Fatal("desktop panel has no scroll")
	}
	if treeContains(scroll, "Thomas Nelson") {
		t.Error("at 1280x700, notice should be pinned in footer, not inside scroll")
	}
	dw.Resize(fyne.NewSize(1280, 420))
	if !treeContains(scroll, "Thomas Nelson") || !treeContains(scroll, "OpenBible.info") {
		t.Error("after shrinking to 1280x420, credits and notice should scroll with the list")
	}
	dw.Resize(fyne.NewSize(1280, 700))
	if treeContains(scroll, "Thomas Nelson") || !treeContains(popup, "Thomas Nelson") {
		t.Error("after restoring 1280x700, credits and notice should return to the pinned footer")
	}
}
