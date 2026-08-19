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
	styledFollowPill = nil
	styledRAFollowPending = false
	styledHighlightCeded = false
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
	st.setHL(hlSearch, "Psalms", 119, 20, 0)
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

	// AND A REOPEN OUTRANKS THE HIGHLIGHT — the rule iOS states and every
	// surface now follows (19 Aug 2026). This pane used to do the opposite:
	// applyStyledReadingRestore disarmed the restore whenever a highlight owned
	// the scroll, so reopening onto a chapter carrying a note or a search hit
	// dragged the reader to it every launch instead of back to where they had
	// stopped reading. An armed restore only ever exists on a REOPEN, because
	// every explicit arrival clears it precisely so it falls through here.
	styledUserScrolled, styledHighlightCeded = false, false
	wantRestore, ok := styledPane.yForVerse(3)
	if !ok {
		t.Fatal("verse 3 missing from the layout")
	}
	armStyledRestore(3, 0, 0.1)
	applyStyledReadingRestore(&styledColumn{scroll: styledScroll, pane: styledPane})
	if styledRestoreArmed {
		t.Error("the restore stayed armed after it should have been applied")
	}
	if got := styledScroll.Offset.Y; got < wantRestore-2 || got > wantRestore+2 {
		t.Errorf("reopen landed at %.1f, want the saved position ≈%.1f — the "+
			"highlight took a position that belonged to the restore", got, wantRestore)
	}
	if !styledHighlightCeded {
		t.Error("the restore placed the view but did not claim the placement, so " +
			"the next layout pass will hand it to the highlight")
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

// A WINDOW RESIZE IS NOT THE READER SCROLLING, and the difference is what keeps
// a shared link's arrival alive on Windows and Linux.
//
// fyne fires OnScrolled for its OWN offset clamps — a resize or a re-wrap moving
// the maximum offset reports a corrected position through the same callback a
// reader's wheel does. The handler always knew that and said so in a comment,
// but it did the damage first and checked afterwards: styledUserScrolled was set
// and state.restore nil'd three lines ABOVE the geometry check that returns. So
// resizing the window cancelled an arrival the link had just armed and threw the
// saved reading position away with it — silently, because everything still
// rendered fine, just in the wrong place.
func TestStyledResizeIsNotAReaderScroll(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer resetStyledWiring()

	st := longPsalmState()
	st.restore = &restoreAnchor{Verse: 40, Frac: 0.5}
	area := styledReadingScrollArea(st, st.Bible.GetChapter("Psalms", 119), lightPalette)
	w := test.NewWindow(area)
	defer w.Close()
	w.Resize(fyne.NewSize(420, 300))
	w.Canvas().Content().Refresh()

	styledUserScrolled = false
	st.restore = &restoreAnchor{Verse: 40, Frac: 0.5}

	// A resize: the view height changes, fyne re-clamps, OnScrolled fires.
	w.Resize(fyne.NewSize(420, 500))
	w.Canvas().Content().Refresh()
	if styledScroll.OnScrolled != nil {
		styledScroll.OnScrolled(styledScroll.Offset)
	}

	if styledUserScrolled {
		t.Error("a resize was recorded as the reader taking over the scroll — " +
			"that cancels the arrival a shared link just armed")
	}
	if st.restore == nil {
		t.Error("a resize threw away the saved reading position; only a real " +
			"reader scroll may do that")
	}

	// A REAL reader scroll still counts. Same callback, no geometry change.
	if styledScroll.OnScrolled != nil {
		styledScroll.OnScrolled(styledScroll.Offset)
	}
	if !styledUserScrolled {
		t.Error("a genuine reader scroll must still be recorded — the fix must " +
			"not make the handler inert")
	}
}

// An arrival near the END of a chapter must ask for an offset the scroll can
// actually reach. Only the floor was clamped, so a note on one of the last
// verses asked for an offset past the maximum; fyne clamped it and reported the
// correction through OnScrolled, which (before the fix above) read as the reader
// scrolling and cancelled the arrival that caused it.
func TestStyledArrivalClampsToTheScrollEnd(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer resetStyledWiring()

	st := longPsalmState()
	last := st.Bible.GetChapter("Psalms", 119)
	st.setHL(hlSearch, "Psalms", 119, last[len(last)-1].Verse, 0)

	area := styledReadingScrollArea(st, st.Bible.GetChapter("Psalms", 119), lightPalette)
	w := test.NewWindow(area)
	defer w.Close()
	w.Resize(fyne.NewSize(420, 300))
	w.Canvas().Content().Refresh()

	max := styledPane.MinSize().Height - styledScroll.Size().Height
	if max < 0 {
		t.Skip("the fixture fits the window; there is no scroll end to clamp to")
	}
	if got := styledScroll.Offset.Y; got > max+0.5 {
		t.Errorf("arrival scrolled to %.1f, past the maximum offset %.1f — fyne "+
			"will clamp that and report the correction as a reader scroll", got, max)
	}
}
