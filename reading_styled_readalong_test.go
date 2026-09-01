package bibletext

// Read-along on the styled pane: the amber tint spans exactly the narrated
// verse's lines, the comfort-band follow only scrolls when the verse drifts
// out (top or past 70%, re-placed at 30%), a follow-scroll disarms any pending
// verse restore, the pill toggles, and a rebuild for the playing chapter
// re-asserts. These mirror the native overlays' behaviour
// (bibleTextMacHighlightVerse) so all five platforms read along the same way.

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
)

// styledRABand digs the read-along rectangle out of the pane's renderer.
// styledRARects returns the VISIBLE narration wash rectangles, in order. The
// wash used to be one full-column rectangle over a line range; it is now one
// per line, bounded to the narrated verse's own runs, so every assertion about
// it is about a set.
func styledRARects(t *testing.T, pane *styledReadingPane) []*canvas.Rectangle {
	t.Helper()
	var out []*canvas.Rectangle
	for _, o := range test.WidgetRenderer(pane).Objects() {
		if rect, ok := o.(*canvas.Rectangle); ok && rect.Visible() &&
			rect.FillColor == color.Color(styledReadAlongTint) {
			out = append(out, rect)
		}
	}
	return out
}

func buildStyledAreaWindow(t *testing.T, st *AppState) fyne.Window {
	t.Helper()
	area := styledReadingScrollArea(st, st.Bible.GetChapter(st.CurrentBook, st.CurrentChapter), lightPalette)
	w := test.NewWindow(area)
	w.Resize(fyne.NewSize(420, 300))
	return w
}

// armControllerReadAlong puts the shared controller in "narration playing,
// follow active" state for st's chapter — follow-scrolls re-check it at apply
// time (readAlongFollowActive), so follow tests need it armed. Restores the
// controller on cleanup.
func armControllerReadAlong(t *testing.T, st *AppState) {
	t.Helper()
	gAudio.mu.Lock()
	wasLoaded, wasFP, wasRA, wasSusp := gAudio.loaded, gAudio.loadedFP, gAudio.readAlong, gAudio.followSuspended
	gAudio.loaded = true
	gAudio.loadedFP = chapterAudioFingerprint(st)
	gAudio.readAlong = []verseTiming{{verse: 1, start: 0, end: 1}}
	gAudio.followSuspended = false
	gAudio.mu.Unlock()
	t.Cleanup(func() {
		gAudio.mu.Lock()
		gAudio.loaded, gAudio.loadedFP, gAudio.readAlong, gAudio.followSuspended = wasLoaded, wasFP, wasRA, wasSusp
		gAudio.mu.Unlock()
	})
}

func TestStyledReadAlongTintSpansVerse(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer resetStyledWiring()

	st := longPsalmState()
	w := buildStyledAreaWindow(t, st)
	defer w.Close()

	styledReadAlongApply(3, false)
	pane := styledPane
	rects := styledRARects(t, pane)
	if len(rects) == 0 {
		t.Fatal("tint must show for the narrated verse")
	}
	first, last, ok := pane.verseLineSpan(3)
	if !ok {
		t.Fatal("verse 3 must have a line span")
	}
	// Poetic verses here occupy two authored lines each, so the wash is a SET
	// covering that range — top of the first rect at the verse's first line,
	// bottom of the last at the end of its last.
	if last <= first {
		t.Errorf("poetic verse should span multiple lines (got %d..%d)", first, last)
	}
	wantTop := pane.lay.Lines[first].Y
	wantBot := pane.lay.Lines[last].Y + pane.lay.Lines[last].H
	gotTop := rects[0].Position().Y
	gotBot := rects[len(rects)-1].Position().Y + rects[len(rects)-1].Size().Height
	if gotTop != wantTop || gotBot != wantBot {
		t.Errorf("wash spans %v..%v, want %v..%v", gotTop, gotBot, wantTop, wantBot)
	}
	// And the point of the change: no rect runs the full column width.
	for i, rect := range rects {
		if rect.Position().X < pane.insetX() {
			t.Errorf("rect %d starts at X=%v, left of the text column (%v)",
				i, rect.Position().X, pane.insetX())
		}
		if rect.Size().Width >= pane.lastWidth-2*pane.insetX() {
			t.Errorf("rect %d is %v wide — still a full-column band", i, rect.Size().Width)
		}
	}

	// Advancing the narration moves the wash; clearing removes it.
	styledReadAlongApply(4, false)
	moved := styledRARects(t, pane)
	if len(moved) == 0 {
		t.Fatal("wash vanished when the narration advanced")
	}
	if y4, _ := pane.yForVerse(4); moved[0].Position().Y != y4 {
		t.Errorf("wash must move to the newly narrated verse: got %v want %v",
			moved[0].Position().Y, y4)
	}
	styledReadAlongClearTint()
	if got := styledRARects(t, pane); len(got) != 0 {
		t.Errorf("clear must remove the tint: %d rects still painted", len(got))
	}
}

func TestStyledReadAlongComfortBandFollow(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer resetStyledWiring()

	st := longPsalmState()
	armControllerReadAlong(t, st)
	w := buildStyledAreaWindow(t, st)
	defer w.Close()
	pane, scroll := styledPane, styledScroll
	viewH := scroll.Size().Height
	if viewH <= 0 {
		t.Fatal("viewport must have real height")
	}

	// A verse far below the fold: follow places its top 30% down the viewport.
	styledReadAlongApply(20, true)
	v20, _ := pane.yForVerse(20)
	wantY := v20 - viewH*0.30
	if got := scroll.Offset.Y; got < wantY-2 || got > wantY+2 {
		t.Fatalf("follow landed at %v, want ~%v (verse top %v)", got, wantY, v20)
	}

	// The next verse still inside the comfortable band: no scroll at all.
	before := scroll.Offset.Y
	styledReadAlongApply(21, true)
	v21, _ := pane.yForVerse(21)
	if v21 <= before+viewH*0.70 && scroll.Offset.Y != before {
		t.Errorf("verse inside the band must not scroll (offset %v -> %v)", before, scroll.Offset.Y)
	}

	// follow=false (reader scrolled away): tint moves, view stays put.
	scroll.Offset = fyne.NewPos(0, 0)
	styledReadAlongApply(25, false)
	if scroll.Offset.Y != 0 {
		t.Error("tint-only update must never move the view")
	}
}

func TestStyledReadAlongFollowDisarmsRestore(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer resetStyledWiring()

	st := longPsalmState()
	armControllerReadAlong(t, st)
	w := buildStyledAreaWindow(t, st)
	defer w.Close()

	armStyledRestore(5, 0, 0.2)
	styledReadAlongApply(20, true)
	if styledRestoreArmed {
		t.Error("a follow-scroll must disarm the pending verse restore — the narration owns the position")
	}
}

// TestStyledReadAlongSuspendedFollowIsIgnored locks the apply-time re-check:
// a follow decision snapshotted before the reader scrolled away (it crossed
// the async hop from the engine goroutine) must NOT yank the view back.
func TestStyledReadAlongSuspendedFollowIsIgnored(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer resetStyledWiring()

	st := longPsalmState()
	armControllerReadAlong(t, st)
	w := buildStyledAreaWindow(t, st)
	defer w.Close()

	gAudio.mu.Lock()
	gAudio.followSuspended = true // the reader scrolled away just before the tick's apply landed
	gAudio.mu.Unlock()

	before := styledScroll.Offset.Y
	styledReadAlongApply(20, true) // stale follow=true from the old snapshot
	if styledScroll.Offset.Y != before {
		t.Error("a stale follow must not scroll once the reader has taken over")
	}
	if styledRAFollowPending {
		t.Error("a rejected stale follow must not stay latched for the next layout pass")
	}
}

// TestStyledReadAlongFollowBeatsSearchHighlight locks the ceded-scroll fix: a
// search jump positions the view once, but once narration follow-scrolls, the
// highlight must not re-pin the view — neither inside the follow's own Refresh
// nor on later layout passes.
func TestStyledReadAlongFollowBeatsSearchHighlight(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer resetStyledWiring()

	st := longPsalmState()
	st.setHL(hlSearch, st.CurrentBook, st.CurrentChapter, 25, 0)
	defer func() { st.clearMark() }()
	armControllerReadAlong(t, st)

	w := buildStyledAreaWindow(t, st)
	defer w.Close()
	w.Canvas().Content().Refresh()

	hlY := styledPane.highlightY() - noteMetrics().Lead
	if got := styledScroll.Offset.Y; got < hlY-2 || got > hlY+2 {
		t.Fatalf("search jump must position the view first (offset %v, want ~%v)", got, hlY)
	}

	// Narration starts at verse 1, far above the highlight: follow must win.
	styledReadAlongApply(1, true)
	if got := styledScroll.Offset.Y; got != 0 {
		t.Fatalf("narration follow must override the highlight pin (offset %v, want 0)", got)
	}
	// And a later layout pass must not snap back to the highlight.
	w.Canvas().Content().Refresh()
	if got := styledScroll.Offset.Y; got != 0 {
		t.Errorf("highlight re-pinned the view on a later layout pass (offset %v)", got)
	}
}

// TestStyledReadAlongResizeClampKeepsFollow locks the clamp guard: fyne fires
// OnScrolled for its own offset clamps on geometry changes — those must not
// read as the reader scrolling away (no pill, follow stays active).
func TestStyledReadAlongResizeClampKeepsFollow(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer resetStyledWiring()

	st := longPsalmState()
	armControllerReadAlong(t, st)
	w := buildStyledAreaWindow(t, st)
	defer w.Close()

	// A geometry change then OnScrolled: fyne's clamp, not a gesture.
	w.Resize(fyne.NewSize(420, 340))
	styledScroll.OnScrolled(fyne.NewPos(0, styledScroll.Offset.Y))
	if !gAudio.readAlongFollowActive() {
		t.Fatal("a resize clamp must not suspend the narration follow")
	}

	// The same event with UNCHANGED geometry is the reader: follow suspends.
	styledScroll.OnScrolled(fyne.NewPos(0, styledScroll.Offset.Y+10))
	if gAudio.readAlongFollowActive() {
		t.Error("a genuine reader scroll must suspend the follow")
	}
}

func TestStyledReadAlongPillToggleAndUserScroll(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer resetStyledWiring()

	st := longPsalmState()
	w := buildStyledAreaWindow(t, st)
	defer w.Close()

	pill := styledFollowPill
	if pill == nil {
		t.Fatal("the styled area must register the follow pill")
	}
	if pill.Visible() {
		t.Fatal("pill must start hidden")
	}
	styledReadAlongSetPill(true)
	if !pill.Visible() {
		t.Error("pill must show on demand")
	}
	styledReadAlongSetPill(false)
	if pill.Visible() {
		t.Error("pill must hide on demand")
	}

	// The wiring's OnScrolled forwards reader scrolls to the audio controller;
	// with no narration loaded that must be a harmless no-op.
	styledScroll.OnScrolled(fyne.NewPos(0, 40))
}

func TestStyledAreaReassertsReadAlongForPlayingChapter(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer resetStyledWiring()

	st := longPsalmState()

	fired := 0
	orig := styledReassertReadAlong
	styledReassertReadAlong = func() { fired++ }
	defer func() { styledReassertReadAlong = orig }()

	// Nothing playing: no reassert.
	w := buildStyledAreaWindow(t, st)
	w.Close()
	if fired != 0 {
		t.Fatalf("no narration loaded -> no reassert (fired %d)", fired)
	}

	// The controller holds this chapter as loaded: the fresh area re-asserts.
	fp := chapterAudioFingerprint(st)
	gAudio.mu.Lock()
	wasLoaded, wasFP := gAudio.loaded, gAudio.loadedFP
	gAudio.loaded, gAudio.loadedFP = true, fp
	gAudio.mu.Unlock()
	defer func() {
		gAudio.mu.Lock()
		gAudio.loaded, gAudio.loadedFP = wasLoaded, wasFP
		gAudio.mu.Unlock()
	}()

	resetStyledWiring()
	w2 := buildStyledAreaWindow(t, st)
	defer w2.Close()
	if fired != 1 {
		t.Errorf("a rebuild for the playing chapter must reassert exactly once (fired %d)", fired)
	}
}
