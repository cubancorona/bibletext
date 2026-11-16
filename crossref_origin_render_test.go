//go:build !race

package bibletext

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// TestRenderCrossRefOrigins captures the real modal and composed list using
// invented prose and a synthetic Treasury index. It needs no downloaded canon,
// credentials or network. Each frame also checks the mounted source identity,
// destinations and horizontal bounds; PNG generation alone is not a pass.
func TestRenderCrossRefOrigins(t *testing.T) {
	out := os.Getenv("BIBLETEXT_ORIGIN_RENDER_DIR")
	if out == "" {
		t.Skip("BIBLETEXT_ORIGIN_RENDER_DIR is not set")
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	th := appThemeForTest(t)
	t.Cleanup(func() { setReadingTextSizeID("normal") })
	withOriginPericopes(t, []gPericope{{
		id: 901, title: "A synthetic journey and two accounts",
		spans: map[string][]gSpan{"Matthew": {{5, 43, 5, 48}}, "Luke": {{6, 27, 6, 28}, {6, 32, 6, 36}}},
		refs:  map[string]string{"Matthew": "5:43-48", "Luke": "6:27-28,32-36"},
	}})
	occasionOnce.Do(loadGospelOccasions)
	oldOccasions := occasionGroups
	occasionGroups = nil
	t.Cleanup(func() { occasionGroups = oldOccasions })
	withCrossRefIndex(t, "Ps.119.99\t1Thess.5.16-1Thess.5.18\t20\n"+
		"Ps.119.99\t2Thess.3.16-2Thess.3.18\t10\n"+
		"Ps.119.100\t1Thess.5.16-1Thess.5.18\t20\n"+
		"Ps.119.100\t2Thess.3.16-2Thess.3.18\t10\n")
	var pending func()
	oldRun, oldLoad := crossRefsRun, crossRefsLoad
	crossRefsRun = func(fn func()) { pending = fn }
	crossRefsLoad = func() error { return nil }
	t.Cleanup(func() { crossRefsRun, crossRefsLoad = oldRun, oldLoad })
	oldVariant := appearanceVariant
	t.Cleanup(func() { appearanceVariant = oldVariant })

	views := []struct {
		name, size string
		canvas     fyne.Size
		variant    fyne.ThemeVariant
	}{
		{"375-light", "normal", fyne.NewSize(375, 667), theme.VariantLight},
		{"375-dark", "normal", fyne.NewSize(375, 667), theme.VariantDark},
		{"320-light", "normal", fyne.NewSize(320, 568), theme.VariantLight},
		{"320-dark-xl", "xl", fyne.NewSize(320, 568), theme.VariantDark},
	}
	frames := 0
	for _, view := range views {
		for _, kind := range []string{"web-anchor", "webc-anchor", "bsb-heading", "gospel-event", "treasury-99-100"} {
			t.Run(kind+"-"+view.name, func(t *testing.T) {
				setReadingTextSizeID(view.size)
				appearanceVariant = func(*AppState) fyne.ThemeVariant { return view.variant }
				fyne.CurrentApp().Settings().SetTheme(forcedVariant{Theme: th, v: view.variant})
				st, span := originRenderState(kind)
				w := test.NewWindow(widget.NewLabel("Synthetic reading page"))
				t.Cleanup(w.Close)
				w.Resize(view.canvas)
				st.app, st.theme, st.window = fyne.CurrentApp(), th, w
				pending = nil
				showCrossRefs(st, "", span)
				w.Canvas().Capture()
				if pending == nil {
					t.Fatal("panel did not schedule the bounded Treasury load")
				}
				pending()
				top, ok := w.Canvas().Overlays().Top().(*widget.PopUp)
				if !ok {
					t.Fatal("the real cross-reference modal did not open")
				}
				scroll, box, _ := panelListParts(top)
				if scroll == nil || box == nil {
					t.Fatal("modal has no composed source list")
				}
				groups := crossRefSourceRows(box.Objects)
				originRenderContent(t, kind, st, span, top, box, groups)
				capture := func(suffix string) {
					t.Helper()
					w.Canvas().Capture()
					originRenderFits(t, st, top, scroll, box)
					name := fmt.Sprintf("%s-%s-%s.png", kind, view.name, suffix)
					writeRenderPNG(t, filepath.Join(out, name), w.Canvas().Capture())
					frames++
				}
				initial := "direct"
				if len(groups) > 0 {
					initial = "open"
					if !groups[0].open {
						initial = "closed"
					}
				}
				capture(initial)
				if len(groups) == 0 {
					return
				}
				beforeSize, beforePos := top.Content.Size(), top.Content.Position()
				open := !groups[0].open
				for _, group := range groups {
					group.setOpen(open)
				}
				wantCards := 0
				if open {
					wantCards = 2
					if kind == "treasury-99-100" {
						wantCards = 4
					}
				}
				if got := len(originListCards(box.Objects)); got != wantCards {
					t.Fatalf("open=%v: mounted %d destination cards, want %d", open, got, wantCards)
				}
				if open && kind == "treasury-99-100" {
					want := []string{"1 Thessalonians 5:16–18", "2 Thessalonians 3:16–18", "1 Thessalonians 5:16–18", "2 Thessalonians 3:16–18"}
					if !slices.Equal(originListCardLabels(box.Objects), want) {
						t.Fatal("opening Treasury origins lost or moved a shared destination")
					}
				}
				if top.Content.Size() != beforeSize || top.Content.Position() != beforePos {
					t.Fatal("opening or closing a group moved the modal")
				}
				if open {
					capture("open")
				} else {
					capture("closed")
				}
			})
		}
	}
	if frames != 32 {
		t.Fatalf("captured %d verified frames, want 32", frames)
	}
}

func originRenderState(kind string) (*AppState, selSpan) {
	st := &AppState{CurrentBook: "Genesis", CurrentChapter: 1, CurrentVersion: "web"}
	span := selSpan{lo: 1, hi: 1}
	if kind == "gospel-event" {
		st.Bible = xrefBible(map[string]map[int]int{"Matthew": {5: 48}, "Luke": {6: 36}})
		st.CurrentBook, st.CurrentChapter = "Matthew", 5
		return st, selSpan{lo: 43, hi: 44}
	}
	if kind == "treasury-99-100" {
		st.Bible = xrefBible(map[string]map[int]int{"Psalms": {119: 176}, "1 Thessalonians": {5: 18}, "2 Thessalonians": {3: 18}})
		st.CurrentBook, st.CurrentChapter = "Psalms", 119
		return st, selSpan{lo: 99, hi: 100}
	}
	st.Bible = xrefBible(map[string]map[int]int{"Genesis": {1: 1}, "Exodus": {1: 2}})
	st.Bible.Verses["Genesis"][1][0].Text = "A synthetic passage places its marker before this phrase."
	if kind == "bsb-heading" {
		st.CurrentVersion = "bsb"
		st.Bible.Headings = map[string]map[int][]Heading{"Genesis": {1: {{
			Text: "A synthetic journey toward a distant place", Style: "heading", BeforeVerse: 1,
			Footnotes: []Footnote{{Text: "Exodus 1:1; 1:2", Kind: footnoteKindCrossref, Caller: "-", Refs: []NoteRef{{"EXO.1.1", 0, 10}, {"EXO.1.2", 12, 15}}}},
		}}}}
		return st, span
	}
	if kind == "webc-anchor" {
		st.CurrentVersion = "webc"
	}
	st.Bible.Verses["Genesis"][1][0].Footnotes = []Footnote{{
		Text: "Exodus 1:1", Anchor: len([]rune("A synthetic ")), Kind: footnoteKindCrossref,
		Caller: "+", Refs: []NoteRef{{"EXO.1.1", 0, 10}},
	}}
	return st, span
}

func originRenderContent(t *testing.T, kind string, st *AppState, span selSpan, top *widget.PopUp, box *fyne.Container, groups []*crossRefSourceGroup) {
	t.Helper()
	passage := passageIn(box.Objects)
	cards := originListCards(box.Objects)
	if strings.HasSuffix(kind, "-anchor") {
		if passage == nil || len(cards) != 1 || cardReference(cards[0]) != "Exodus 1:1" || cardLetter(cards[0]) != "a" || len(groups) != 0 {
			t.Fatal("the exact publisher anchor did not produce its directly linked a card")
		}
		selected := versesBetween(st.Bible, st.CurrentBook, st.CurrentChapter, span.lo, span.hi)
		if passagePlainText(passage.runs) != selected[0].Text {
			t.Fatal("the lettered passage changed its synthetic source text")
		}
		var prefix strings.Builder
		marks := 0
		for _, run := range passage.runs {
			if run.Mark {
				marks++
				if run.Text != "a" || prefix.String() != "A synthetic " {
					t.Fatalf("anchor %q follows %q instead of its exact source position", run.Text, prefix.String())
				}
			} else {
				prefix.WriteString(run.Text)
			}
		}
		if marks != 1 || !treeHasText(top, publisherRunLabel(st.currentVersion().Abbrev)) {
			t.Fatal("missing source label or exact anchor mark")
		}
		return
	}
	if passage != nil {
		t.Fatal("a source without an exact word anchor repeated the selected passage")
	}
	switch kind {
	case "bsb-heading":
		if len(groups) != 1 || !groups[0].open || groups[0].key != nil || len(cards) != 2 ||
			!treeHasText(top, "BSB REFERENCES") || !treeHasText(top, "A synthetic journey toward a distant place") || !treeHasText(top, "Heading before verse 1") {
			t.Fatal("BSB heading identity, placement or open destinations missing")
		}
		if !slices.Equal(originListCardLabels(box.Objects), []string{"Exodus 1:1", "Exodus 1:2"}) {
			t.Fatal("BSB heading destinations changed")
		}
	case "gospel-event":
		if len(groups) != 1 || !groups[0].open || groups[0].key != nil ||
			!treeHasText(top, parallelsLabel) || !treeHasText(top, "A synthetic journey and two accounts") || !treeHasText(top, "Matthew 5:43–48") {
			t.Fatal("Gospel group lost its actual episode or source span")
		}
		if !slices.Equal(originListCardLabels(box.Objects), []string{"Luke 6:27–28", "Luke 6:32–36"}) || !treeHasText(top, parallelsCredit) || treeHasText(top, treasuryCredit) {
			t.Fatal("Gospel spans or displayed-source credits changed")
		}
	case "treasury-99-100":
		if len(groups) != 2 || len(cards) != 0 || !treeHasText(top, treasuryLabel) || !treeHasText(top, treasuryCredit) {
			t.Fatal("multi-verse Treasury did not begin as two credited closed origins")
		}
		for i, g := range groups {
			if g.key == nil || g.key.Text != fmt.Sprint(99+i) || g.open || g.built || len(g.body.Objects) != 0 || g.key.TextStyle.Italic || !g.key.TextStyle.Bold {
				t.Fatalf("Treasury origin %d is missing, eager, or styled as a publisher letter", 99+i)
			}
		}
		if groups[0].margin != groups[1].margin {
			t.Fatal("two- and three-digit keys do not align")
		}
	}
	for _, card := range cards {
		if cardLetter(card) != "" {
			t.Fatal("a source without word anchors acquired a publisher letter")
		}
	}
}

func originRenderFits(t *testing.T, st *AppState, top *widget.PopUp, scroll *container.Scroll, box *fyne.Container) {
	t.Helper()
	inner := sheetPanelSize(st, st.window.Canvas()).Width - theme.InnerPadding()
	if top.Content.MinSize().Width > inner+0.5 || math.Abs(float64(top.Content.Size().Width-inner)) > 0.5 {
		t.Fatalf("modal width %.1f/min %.1f does not fit %.1f", top.Content.Size().Width, top.Content.MinSize().Width, inner)
	}
	if box.Size().Width <= 0 || box.Size().Width > scroll.Size().Width+0.5 || scroll.Offset.X != 0 {
		t.Fatalf("list %.1f exceeds viewport %.1f or moved horizontally", box.Size().Width, scroll.Size().Width)
	}
	if problems := crossRefListMisfits(box.Objects, box.Size().Width); len(problems) != 0 {
		t.Fatal(strings.Join(problems, "; "))
	}
	for _, card := range originListCards(box.Objects) {
		if !strings.Contains(strings.Join(treeTexts(card), " "), "Synthetic text of ") {
			t.Fatalf("destination %q has no synthetic passage preview", cardReference(card))
		}
	}
	checked := 0
	walkTree(box, func(o fyne.CanvasObject) {
		if !o.Visible() || o.Size().Width <= 0 {
			return
		}
		switch x := o.(type) {
		case *crossRefPrinted:
			checked++
			r := test.WidgetRenderer(x).(*crossRefPrintedRenderer)
			for _, row := range r.rows {
				if r.measure(row.Text) > x.Size().Width+0.5 {
					t.Errorf("printed line %q overflows %.1f points", row.Text, x.Size().Width)
				}
			}
		case *readingParagraph:
			checked++
			r := test.WidgetRenderer(x).(*readingParagraphRenderer)
			for _, width := range r.rowWidths() {
				if width > x.Size().Width+0.5 {
					t.Errorf("passage line %.1f overflows %.1f points", width, x.Size().Width)
				}
			}
		}
	})
	if checked == 0 {
		t.Fatal("no rendered content was measured")
	}
	for _, g := range crossRefSourceRows(box.Objects) {
		if g.open && g.body.Position().X+g.body.Size().Width > g.Size().Width+0.5 {
			t.Fatal("opened source cards overflow their group")
		}
		for _, label := range g.labels {
			if label.Position().X < g.margin || label.Position().X+label.Size().Width > g.fold.Position().X-crossRefSourceIconGap+0.5 {
				t.Fatal("source heading overlaps its origin key or disclosure")
			}
		}
	}
}
