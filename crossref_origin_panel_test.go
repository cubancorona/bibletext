//go:build !race

package bibletext

import (
	"fmt"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func sourceGroupPanelState(t *testing.T) *AppState {
	t.Helper()
	bd := NewBibleData()
	bd.Verses = map[string]map[int][]Verse{}
	for _, book := range []string{"Genesis", "Psalms"} {
		bd.Verses[book] = map[int][]Verse{1: {}}
		for v := 1; v <= 12; v++ {
			bd.Verses[book][1] = append(bd.Verses[book][1], Verse{BookName: book, Chapter: 1, Verse: v, Text: "A synthetic passage for checking the study panel and wrapped reference cards."})
		}
	}
	var rows strings.Builder
	for origin := 1; origin <= 3; origin++ {
		for target := 1; target <= 8; target++ {
			fmt.Fprintf(&rows, "Gen.1.%d\tPs.1.%d\t%d\n", origin, target, 100-target)
		}
	}
	withCrossRefIndex(t, rows.String())
	return &AppState{Bible: bd, CurrentBook: "Genesis", CurrentChapter: 1, CurrentVersion: "web"}
}

func TestSourceGroupPanelRetainsIndependentFoldsThroughRebuild(t *testing.T) {
	th := appThemeForTest(t)
	st := sourceGroupPanelState(t)
	oldRun, oldLoad := crossRefsRun, crossRefsLoad
	t.Cleanup(func() { crossRefsRun, crossRefsLoad = oldRun, oldLoad })
	crossRefsRun = func(fn func()) { fn() }
	crossRefsLoad = func() error { return nil }
	live := theme.VariantLight
	oldVariant := appearanceVariant
	appearanceVariant = func(*AppState) fyne.ThemeVariant { return live }
	t.Cleanup(func() { appearanceVariant = oldVariant })
	st.app, st.theme = fyne.CurrentApp(), th
	w := test.NewWindow(nil)
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(375, 667))
	st.window = w
	w.SetContent(CreateMainUI(st.app, st, w))
	st.appearance.built = theme.VariantLight
	inspect := func() (*widget.PopUp, *container.Scroll, []*crossRefSourceGroup) {
		t.Helper()
		top, ok := w.Canvas().Overlays().Top().(*widget.PopUp)
		if !ok {
			t.Fatal("missing panel")
		}
		scroll, box, _ := panelListParts(top)
		if scroll == nil || box == nil {
			t.Fatal("missing scroll or list")
		}
		groups := crossRefSourceRows(box.Objects)
		if len(groups) != 3 {
			t.Fatalf("got %d origin groups, want 3", len(groups))
		}
		return top, scroll, groups
	}
	showCrossRefs(st, "", selSpan{lo: 1, hi: 3})
	first, scroll, groups := inspect()
	for _, g := range groups {
		if g.open || len(g.body.Objects) != 0 {
			t.Fatal("multi-verse cards eagerly opened")
		}
	}
	groups[1].toggle()
	groups[2].toggle()
	scroll.Offset = fyne.NewPos(0, 80)
	scroll.Refresh()
	live = theme.VariantDark
	observeAppearance(st, appearanceChanged)
	second, scroll, groups := inspect()
	if first == second || first.Visible() {
		t.Fatal("appearance did not replace panel")
	}
	if groups[0].open || !groups[1].open || !groups[2].open || scroll.Offset.Y != 80 {
		t.Fatal("appearance lost independent source folds or scroll position")
	}
	groups[2].toggle()
	w.Resize(fyne.NewSize(667, 375))
	rebuildForLayoutChange(st)
	third, _, groups := inspect()
	if second == third || second.Visible() || groups[0].open || !groups[1].open || groups[2].open {
		t.Fatal("landscape rebuild lost source folds")
	}
	if groups[1].id != "treasury:2" || len(groups[1].body.Objects) != 8 {
		t.Fatal("restored fold contains the wrong origin")
	}
	if len(w.Canvas().Overlays().List()) != 1 {
		t.Fatal("rebuild left an extra panel")
	}
}

func TestSourceGroupPanelPreservesLocalEpisodeWhileTreasuryLoads(t *testing.T) {
	app := themedTestApp()
	t.Cleanup(app.Quit)
	bd := NewBibleData()
	bd.Verses = map[string]map[int][]Verse{}
	for _, book := range []string{"Matthew", "Luke"} {
		bd.Verses[book] = map[int][]Verse{}
		for _, ch := range []int{5, 6} {
			for v := 1; v <= 48; v++ {
				bd.Verses[book][ch] = append(bd.Verses[book][ch], Verse{BookName: book, Chapter: ch, Verse: v, Text: "Synthetic episode text for an offline study panel."})
			}
		}
	}
	withCrossRefIndex(t, "")
	var pending func()
	oldRun, oldLoad := crossRefsRun, crossRefsLoad
	t.Cleanup(func() { crossRefsRun, crossRefsLoad = oldRun, oldLoad })
	crossRefsRun = func(fn func()) { pending = fn }
	crossRefsLoad = func() error { return errOfflineForTest }
	w := test.NewWindow(widget.NewLabel("Reading"))
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(375, 667))
	st := &AppState{Bible: bd, CurrentBook: "Matthew", CurrentChapter: 5, CurrentVersion: "web", window: w}
	showCrossRefs(st, "", selSpan{lo: 43, hi: 44})
	top := w.Canvas().Overlays().Top()
	_, box, bar := panelListParts(top)
	if box == nil || bar == nil || pending == nil {
		t.Fatal("local episode did not appear with Treasury loading")
	}
	groups := crossRefSourceRows(box.Objects)
	if len(groups) != 1 || !groups[0].open {
		t.Fatalf("expected one initially open Gospel episode, got %d", len(groups))
	}
	id := groups[0].id
	groups[0].toggle()
	pending()
	_, box, _ = panelListParts(top)
	groups = crossRefSourceRows(box.Objects)
	if len(groups) != 1 || groups[0].id != id || groups[0].open {
		t.Fatal("failed download reset local fold")
	}
	if !treeHasText(top, parallelsCredit) || treeHasText(top, treasuryCredit) {
		t.Fatal("credits do not match displayed sources")
	}
	if findTapCardWithText(top, "Try again") == nil {
		t.Fatal("Treasury retry unavailable beside local content")
	}
}

func TestSourceGroupPanelThreeDigitOriginsFitMinimumWidth(t *testing.T) {
	th := appThemeForTest(t)
	t.Cleanup(func() { setReadingTextSizeID("normal") })
	withCrossRefIndex(t, "Ps.119.99\t1Thess.5.16-1Thess.5.18\t10\nPs.119.100\t1Thess.5.16-1Thess.5.18\t10\n")
	oldRun, oldLoad := crossRefsRun, crossRefsLoad
	t.Cleanup(func() { crossRefsRun, crossRefsLoad = oldRun, oldLoad })
	crossRefsRun = func(fn func()) { fn() }
	crossRefsLoad = func() error { return nil }
	for _, option := range textSizeOptions {
		t.Run(option.ID, func(t *testing.T) {
			setReadingTextSizeID(option.ID)
			bd := xrefBible(map[string]map[int]int{"Psalms": {119: 176}, "1 Thessalonians": {5: 28}})
			w := test.NewWindow(widget.NewLabel("Reading"))
			t.Cleanup(w.Close)
			w.Resize(fyne.NewSize(320, 568))
			st := &AppState{Bible: bd, CurrentBook: "Psalms", CurrentChapter: 119, CurrentVersion: "web", theme: th, window: w}
			showCrossRefs(st, "", selSpan{lo: 99, hi: 100})
			top := w.Canvas().Overlays().Top()
			_, box, _ := panelListParts(top)
			groups := crossRefSourceRows(box.Objects)
			if len(groups) != 2 || groups[0].key.Text != "99" || groups[1].key.Text != "100" {
				t.Fatal("wrong source origins")
			}
			if groups[0].margin != groups[1].margin {
				t.Fatal("mixed digit counts do not share an aligned margin")
			}
			groups[1].toggle()
			g := groups[1]
			limit := box.Size().Width - 2*crossRefSectionPadX - g.margin
			if g.body.Size().Width > limit+0.5 || g.body.MinSize().Width > limit+0.5 {
				t.Fatalf("body %.1f/min %.1f wider than available %.1f", g.body.Size().Width, g.body.MinSize().Width, limit)
			}
			var heading fyne.CanvasObject
			walkTree(g.body, func(o fyne.CanvasObject) {
				switch x := o.(type) {
				case *crossRefPrinted:
					if x.refWrap && x.Text == "1 Thessalonians 5:16–18" {
						heading = x
					}
				case *canvas.Text:
					if x.Text == "1 Thessalonians 5:16–18" {
						heading = x
					}
				}
			})
			if heading == nil || heading.MinSize().Width > limit+0.5 {
				t.Fatal("reference heading does not fit the three-digit group")
			}
			g.keepLetterInView(30)
			if g.pin == nil || g.pin.TextStyle != g.key.TextStyle || g.pin.FontSource != g.key.FontSource || g.pin.TextSize != g.key.TextSize {
				t.Fatal("sticky source key changed its font or size")
			}
		})
	}
}
