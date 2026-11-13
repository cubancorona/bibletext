//go:build !race

package bibletext

import (
	"os"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
)

// EVERY REAL TREASURY AND GOSPEL PARALLEL CARD FITS THE NARROWEST PHONE.
// Opt-in alongside TestEveryCrossReferenceRowOpensScriptureTheTextHas, kept in
// a !race file because measuring against the app's fonts sets the app theme
// (widgettree_test.go). Collects every distinct card shape emitted across all
// downloaded translations — single-book and cross-book Treasury rows, badged
// and unbadged Gospel parallels, and other-occasion rows — and measures both
// MinSize and rendered row edges at Normal, Large and Extra large against 283pt.
func TestEveryRealCrossReferenceCardFitsTheNarrowestPhone(t *testing.T) {
	if os.Getenv("BIBLETEXT_XREF_REALDATA") == "" {
		t.Skip("set BIBLETEXT_XREF_REALDATA=1 to measure every downloaded Treasury and parallel card")
	}
	zipBytes, err := os.ReadFile(realCachePath(crossRefCachePath()))
	if err != nil {
		t.Skipf("no Treasury zip in the machine's cache: %v", err)
	}
	idx, _, err := parseCrossRefZip(zipBytes)
	if err != nil {
		t.Fatal(err)
	}
	crossRefMu.Lock()
	was := crossRefIndex
	crossRefIndex = idx
	crossRefMu.Unlock()
	t.Cleanup(func() {
		crossRefMu.Lock()
		crossRefIndex = was
		crossRefMu.Unlock()
	})

	type cardShape struct {
		label         string
		title         string
		parallel      bool
		otherOccasion bool
		badge         bool
	}
	shapes := map[cardShape]crossRefCard{}
	walked := 0
	for _, id := range []string{"web", "bsb", "webc", "nkjv"} {
		bd, err := loadBibleFromCache(realCachePath(cachePathForVersion(id)))
		if err != nil {
			continue
		}
		walked++
		for _, book := range bd.Books {
			for _, ch := range bd.GetChapterNumbersForBook(book) {
				st := &AppState{Bible: bd, CurrentBook: book, CurrentChapter: ch, CurrentVersion: id}
				for _, v := range bd.GetChapter(book, ch) {
					for _, c := range crossRefsForSelection(st, "", selSpan{lo: v.Verse, hi: v.Verse}) {
						if c.Parallel {
							for _, badge := range []bool{false, true} {
								k := cardShape{c.label(), c.Title, true, c.otherOccasion(), badge}
								if _, ok := shapes[k]; !ok {
									shapes[k] = passageCard(c, badge)
								}
							}
						} else {
							k := cardShape{label: c.label()}
							if _, ok := shapes[k]; !ok {
								shapes[k] = passageCard(c, false)
							}
						}
					}
				}
			}
		}
	}
	if walked == 0 {
		t.Skip("no downloaded translation in the machine's cache")
	}

	th := appThemeForTest(t)
	t.Cleanup(func() { setReadingTextSizeID("normal") })
	limit := narrowestPhoneBody(t)
	empty := &AppState{Bible: &BibleData{}, theme: th}

	for _, sizeOpt := range textSizeOptions {
		setReadingTextSizeID(sizeOpt.ID)
		bad := 0
		for k, card := range shapes {
			row := crossRefRowRuled(empty, card, lightPalette, func(crossRef) {}, false)
			if problems := crossRefListMisfits([]fyne.CanvasObject{row}, limit); len(problems) != 0 {
				bad++
				if bad <= 10 {
					t.Errorf("%s %+v: %s", sizeOpt.ID, k, strings.Join(problems, "; "))
				}
				continue
			}
			row.Resize(fyne.NewSize(limit, 200))
			walkTree(row, func(o fyne.CanvasObject) {
				switch x := o.(type) {
				case *canvas.Text:
					if x.Text != "" {
						if edge := x.Position().X + x.Size().Width; edge > limit+0.5 {
							bad++
							if bad <= 10 {
								t.Errorf("%s %+v text %q reaches %.1f, over %.0f", sizeOpt.ID, k, x.Text, edge, limit)
							}
						}
					}
				case *crossRefPrinted:
					rr := test.WidgetRenderer(x).(*crossRefPrintedRenderer)
					rr.Layout(x.Size())
					for _, rtxt := range rr.rows {
						if rw := rr.measure(rtxt.Text); rw > limit+0.5 {
							bad++
							if bad <= 10 {
								t.Errorf("%s %+v printed row %q wants %.1f, over %.0f", sizeOpt.ID, k, rtxt.Text, rw, limit)
							}
						}
					}
				}
			})
		}
		t.Logf("%s: %d distinct real Treasury/parallel cards measured, %d faults", sizeOpt.ID, len(shapes), bad)
	}
}
