//go:build !race

package bibletext

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func TestCrossRefHeadingsFitMinimumPanelWidth(t *testing.T) {
	th := appThemeForTest(t)
	t.Cleanup(func() { setReadingTextSizeID("normal") })
	withCrossRefIndex(t, "")
	previousRun, previousLoad := crossRefsRun, crossRefsLoad
	t.Cleanup(func() { crossRefsRun, crossRefsLoad = previousRun, previousLoad })
	crossRefsRun = func(work func()) { work() }
	crossRefsLoad = func() error { return nil }

	// This destination occurs in the shipped BSB note at 1 Corinthians 16:19.
	// Synthetic source prose also exercises the same heading inside a group.
	const label = "2 Thessalonians 3:16–18"
	windowSize := fyne.NewSize(320, 568)
	if width := aiPanelSize(windowSize).Width; width != 280 {
		t.Fatalf("minimum-width control: got %.0f, want 280", width)
	}
	for _, sizeOpt := range textSizeOptions {
		setReadingTextSizeID(sizeOpt.ID)
		for _, grouped := range []bool{false, true} {
			bd := xrefBible(map[string]map[int]int{"John": {3: 1}, "2 Thessalonians": {3: 18}})
			citation, text, start := label, label, 0
			if grouped {
				citation = "2 Thess. 3:16–18"
				text, start = "("+citation+")", 1
			}
			bd.Verses["John"][3][0].Footnotes = []Footnote{{
				Anchor: 0, Caller: "+", Kind: footnoteKindCrossref, Text: text,
				Refs: []NoteRef{{"2TH.3.16-2TH.3.18", start, start + len([]rune(citation))}},
			}}
			w := test.NewWindow(widget.NewLabel("Reading"))
			t.Cleanup(w.Close)
			w.Resize(windowSize)
			st := &AppState{Bible: bd, CurrentBook: "John", CurrentChapter: 3, CurrentVersion: "bsb", theme: th, window: w}
			showCrossRefs(st, "", selSpan{lo: 1, hi: 1})
			top := w.Canvas().Overlays().Top()
			if top == nil {
				t.Fatal("cross-reference panel did not open")
			}
			if grouped {
				opened := 0
				walkTree(top, func(o fyne.CanvasObject) {
					if group, ok := o.(*crossRefGroupRow); ok && group.header != nil {
						group.toggle()
						opened++
					}
				})
				if opened != 1 {
					t.Fatalf("group control: opened %d groups, want 1", opened)
				}
			}
			_, listBox, _ := panelListParts(top)
			if listBox == nil || listBox.Size().Width <= 0 {
				t.Fatal("rendered list has no width")
			}
			limit := listBox.Size().Width - 2*crossRefSectionPadX - crossRefPanelSizes().letterMargin
			if grouped {
				limit -= 16 // The opened well's two 8pt side insets.
			}
			if crossRefSingleLineMax > limit {
				t.Errorf("%s grouped=%v: unwrapped cutoff %.1f exceeds mounted content width %.1f", sizeOpt.ID, grouped, crossRefSingleLineMax, limit)
			}
			var printed *crossRefPrinted
			var single *canvas.Text
			walkTree(top, func(o fyne.CanvasObject) {
				switch x := o.(type) {
				case *crossRefPrinted:
					if x.refWrap && x.Text == label {
						printed = x
					}
				case *canvas.Text:
					if x.Text == label {
						single = x
					}
				}
			})
			if printed != nil {
				if width := printed.Size().Width; width > limit+0.5 {
					t.Errorf("%s grouped=%v: heading allocated %.1f beyond the %.1f content area", sizeOpt.ID, grouped, width, limit)
				}
				renderer := test.WidgetRenderer(printed).(*crossRefPrintedRenderer)
				for _, row := range renderer.rows {
					if width := renderer.measure(row.Text); width > limit+0.5 {
						t.Errorf("%s grouped=%v: heading row %q is %.1f wide in %.1f", sizeOpt.ID, grouped, row.Text, width, limit)
					}
				}
				if got := strings.Join(renderer.rowTexts(), " "); got != label {
					t.Errorf("%s grouped=%v: heading split its verse range or lost text: %q", sizeOpt.ID, grouped, got)
				}
			} else if single != nil {
				if width := single.MinSize().Width; width > limit+0.5 {
					t.Errorf("%s grouped=%v: heading is %.1f wide in %.1f", sizeOpt.ID, grouped, width, limit)
				}
			} else {
				t.Fatalf("%s grouped=%v: reference heading missing", sizeOpt.ID, grouped)
			}
			test.Tap(findTreeButton(top, "Close"))
		}
	}
}

func TestCrossRefMinimumWidthKeepsSameBookRangeWhole(t *testing.T) {
	th := appThemeForTest(t)
	p := newCrossRefRef("Psalm 119:105–176", 16, fyne.TextStyle{}, th.palette().Text)
	renderer := test.WidgetRenderer(p).(*crossRefPrintedRenderer)
	want := renderer.measure("119:105–176")
	if want <= renderer.measure("Psalm") {
		t.Fatal("control: verse range must be the widest indivisible word")
	}
	renderer.Layout(fyne.NewSize(want, 200))
	if got := renderer.MinSize().Width; got != want {
		t.Errorf("minimum width %.2f splits a same-book verse range that needs %.2f", got, want)
	}
}
