package bibletext

// Milestone-4 assembly tests: the styled scroll area's anchor wiring must
// reproduce reading_scroll_fyne.go's semantics — capture maps the offset to a
// verse anchor, an armed restore applies once sizes are real and stays armed
// across passes, a same-chapter rewire carries the position over, and the
// reader's own scroll disarms.

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// longPsalmState builds a chapter long enough to scroll in a short window.
func longPsalmState() *AppState {
	verses := make([]Verse, 0, 30)
	for v := 1; v <= 30; v++ {
		verses = append(verses, Verse{
			BookName: "Psalms", Book: "Psalms", Chapter: 119, Verse: v,
			Text: "Blessed are those whose ways are blameless,\nwho walk according to the law of the LORD.",
		})
	}
	bd := &BibleData{
		Books:  []string{"Psalms"},
		Verses: map[string]map[int][]Verse{"Psalms": {119: verses}},
	}
	return &AppState{Bible: bd, CurrentBook: "Psalms", CurrentChapter: 119}
}

func resetStyledWiring() {
	styledScroll, styledPane, styledState, styledFP = nil, nil, nil, ""
	styledRestoreArmed, styledUserScrolled, styledApplyingScroll = false, false, false
	styledRestoreVerse, styledRestoreDelta, styledRestoreFrac = 0, 0, 0
}

func TestStyledAreaCaptureAndRestore(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer resetStyledWiring()

	st := longPsalmState()
	area := styledReadingScrollArea(st, st.Bible.GetChapter("Psalms", 119), lightPalette)
	w := test.NewWindow(area)
	defer w.Close()
	w.Resize(fyne.NewSize(420, 300))

	if !styledAnchorActive() {
		t.Fatal("wiring must register the styled pane")
	}

	// Scroll partway (programmatically, as a reader would land mid-chapter).
	styledScroll.Offset = fyne.NewPos(0, 400)
	v, d, f, ok := captureStyledAnchor()
	if !ok || v <= 1 {
		t.Fatalf("capture = v%d d%.1f f%.2f ok=%v — want a mid-chapter verse", v, d, f, ok)
	}

	// A fresh area for the same chapter with an armed restore must return to
	// the same verse's line.
	resetStyledWiring()
	area2 := styledReadingScrollArea(st, st.Bible.GetChapter("Psalms", 119), lightPalette)
	w2 := test.NewWindow(area2)
	defer w2.Close()
	w2.Resize(fyne.NewSize(420, 300))
	armStyledRestore(v, d, f)

	// Drive layout passes until the restore applies (sizes settle async).
	for i := 0; i < 5 && styledScroll.Offset.Y == 0; i++ {
		w2.Canvas().Content().Refresh()
		w2.Resize(fyne.NewSize(420, 300+float32(i))) // nudge a layout pass
	}
	wantY, okY := styledPane.yForVerse(v)
	if !okY {
		t.Fatalf("verse %d missing from restored layout", v)
	}
	got := styledScroll.Offset.Y
	if got < wantY+float32(d)-2 || got > wantY+float32(d)+2 {
		t.Errorf("restore offset = %.1f, want %.1f (verse %d + delta %.1f)", got, wantY+float32(d), v, d)
	}

	// The reader's own scroll disarms the target.
	styledScroll.OnScrolled(fyne.NewPos(0, 50))
	if styledRestoreArmed {
		t.Error("a user scroll must disarm the pending restore")
	}
}

func TestStyledAreaCarryOverOnRewire(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer resetStyledWiring()

	st := longPsalmState()
	area := styledReadingScrollArea(st, st.Bible.GetChapter("Psalms", 119), lightPalette)
	w := test.NewWindow(area)
	defer w.Close()
	w.Resize(fyne.NewSize(420, 300))
	styledScroll.Offset = fyne.NewPos(0, 350)
	wantV, _, _, _ := captureStyledAnchor()

	// Same-chapter rebuild (a theme flip): the new wiring must carry the
	// position over as an armed restore.
	area2 := styledReadingScrollArea(st, st.Bible.GetChapter("Psalms", 119), lightPalette)
	_ = area2
	if !styledRestoreArmed || styledRestoreVerse != wantV {
		t.Errorf("carry-over: armed=%v verse=%d, want armed at verse %d",
			styledRestoreArmed, styledRestoreVerse, wantV)
	}

	// A DIFFERENT chapter must not carry over.
	resetStyledWiring()
	area3 := styledReadingScrollArea(st, st.Bible.GetChapter("Psalms", 119), lightPalette)
	w3 := test.NewWindow(area3)
	defer w3.Close()
	w3.Resize(fyne.NewSize(420, 300))
	styledScroll.Offset = fyne.NewPos(0, 350)
	st2 := psalm23State()
	styledReadingScrollArea(st2, st2.Bible.GetChapter("Psalms", 23), lightPalette)
	if styledRestoreArmed {
		t.Error("a different chapter must start at the top, not carry the old anchor")
	}
}

func TestStyledAreaHighlightScrollsOnce(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer resetStyledWiring()

	st := longPsalmState()
	st.HasHighlightedVerse = true
	st.HighlightedBook = "Psalms"
	st.HighlightedChapter = 119
	st.HighlightedVerse = 20
	area := styledReadingScrollArea(st, st.Bible.GetChapter("Psalms", 119), lightPalette)
	w := test.NewWindow(area)
	defer w.Close()
	w.Resize(fyne.NewSize(420, 300))
	w.Canvas().Content().Refresh()

	wantY, _ := styledPane.yForVerse(20)
	got := styledScroll.Offset.Y
	if got < wantY-30 || got > wantY {
		t.Errorf("highlight scroll-to: offset %.1f, want ≈%.1f-24", got, wantY)
	}

	// And a highlight supersedes any armed restore.
	armStyledRestore(3, 0, 0.1)
	applyStyledReadingRestore(&styledColumn{scroll: styledScroll, pane: styledPane})
	if styledRestoreArmed {
		t.Error("a highlight jump must drop the pending restore")
	}
}

func TestStyledAreaEmptyChapter(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer resetStyledWiring()

	st := psalm23State()
	area := styledReadingScrollArea(st, nil, lightPalette)
	if area == nil {
		t.Fatal("empty chapter must still yield a placeholder area")
	}
	if styledAnchorActive() {
		t.Error("an empty chapter must clear the wiring, not serve stale captures")
	}
}
