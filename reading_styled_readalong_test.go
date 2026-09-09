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
	// A tapped search result is explicit; plain entries with a lit wash open
	// at the top now.
	st.forceReposition = true
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

// --- S18: the Psalm title is the read-along's verse 0 -------------------------

// titledPsalmState is longPsalmState with a superscription. The fixture's
// chapter is synthetic (the real 119 has no title), which is irrelevant here.
func titledPsalmState(title string) *AppState {
	st := longPsalmState()
	st.Bible.Superscriptions = map[string]map[int]Superscription{"Psalms": {119: {Text: title}}}
	return st
}

// The title's wash is the title's own geometry — one rect per title line,
// exactly the washes table measureStyledSuperscription filled, never a verse
// span — and it goes with the rest of the narration when the row moves on.
func TestStyledReadAlongTitleWashIsTheTitleGeometry(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer resetStyledWiring()

	st := titledPsalmState("A Psalm of David, when he fled from Absalom his son, and a line more so that it wraps.")
	w := buildStyledAreaWindow(t, st)
	defer w.Close()
	pane := styledPane
	if !pane.superGeom.present || len(pane.superGeom.washes) < 2 {
		t.Fatalf("fixture must produce a wrapped title (%d washes)", len(pane.superGeom.washes))
	}
	if got := styledRARects(t, pane); len(got) != 0 {
		t.Fatalf("a fresh titled pane opened with %d wash rects lit", len(got))
	}

	styledReadAlongApply(readAlongTitle, false)
	rects := styledRARects(t, pane)
	if len(rects) != len(pane.superGeom.washes) {
		t.Fatalf("%d wash rects for a %d-line title", len(rects), len(pane.superGeom.washes))
	}
	for i, rect := range rects {
		want := pane.superGeom.washes[i]
		if rect.Position() != want.pos() || rect.Size() != want.size() {
			t.Errorf("title wash %d at %v %v, want %v %v", i, rect.Position(), rect.Size(), want.pos(), want.size())
		}
		if rect.Position().Y+rect.Size().Height > pane.lay.Lines[0].Y {
			t.Errorf("title wash %d reaches into verse 1's line", i)
		}
	}
	if len(pane.raSpans) != 0 {
		t.Errorf("the title lit %d verse spans; it must light none", len(pane.raSpans))
	}

	// Moving on to verse 1 takes the title's wash off and lights the verse.
	styledReadAlongApply(1, false)
	moved := styledRARects(t, pane)
	if y1, _ := pane.yForVerse(1); len(moved) == 0 || moved[0].Position().Y != y1 {
		t.Errorf("verse 1's wash is not at verse 1 (%v)", moved)
	}
	for _, rect := range moved {
		if rect.Position().Y < pane.lay.Lines[0].Y {
			t.Error("a title wash rect is still lit after the narration moved to verse 1")
		}
	}
	styledReadAlongClearTint()
	if got := styledRARects(t, pane); len(got) != 0 {
		t.Errorf("clear left %d rects lit", len(got))
	}
}

// CONTROL for the wash: a chapter with no title lights nothing at verse 0 and
// leaves nothing pending — and readAlongNone, not 0, is what clears.
func TestStyledReadAlongVerseZeroLightsNothingWithoutATitle(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer resetStyledWiring()

	st := longPsalmState()
	armControllerReadAlong(t, st)
	w := buildStyledAreaWindow(t, st)
	defer w.Close()
	pane := styledPane

	styledReadAlongApply(3, true)
	if len(styledRARects(t, pane)) == 0 {
		t.Fatal("verse 3 must light")
	}
	styledReadAlongApply(readAlongTitle, true)
	if got := styledRARects(t, pane); len(got) != 0 {
		t.Errorf("verse 0 in an untitled chapter lit %d rects", len(got))
	}
	if styledRAFollowPending {
		t.Error("an untitled verse 0 left a follow pending")
	}
	if pane.raVerse != readAlongTitle || pane.raTitleLit() {
		t.Errorf("raVerse=%d lit=%v; the row is the title, but nothing is lit", pane.raVerse, pane.raTitleLit())
	}
	styledReadAlongClearTint()
	if pane.raVerse != readAlongNone {
		t.Errorf("clear left raVerse=%d, want readAlongNone", pane.raVerse)
	}
}

// A resize between measure and rebuild re-places the wash from the current
// geometry table: it can never stay where the old title was.
func TestStyledReadAlongTitleWashFollowsAResize(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer resetStyledWiring()

	st := titledPsalmState("A Psalm of David, when he fled from Absalom his son, and a line more so that it wraps.")
	w := buildStyledAreaWindow(t, st)
	defer w.Close()
	pane := styledPane
	styledReadAlongApply(readAlongTitle, false)
	before := styledRARects(t, pane)
	if len(before) < 2 {
		t.Fatalf("fixture must wrap the title (%d rects)", len(before))
	}
	beforeW := before[0].Size().Width

	w.Resize(fyne.NewSize(260, 300))
	after := styledRARects(t, pane)
	if len(after) != len(pane.superGeom.washes) || len(after) <= len(before) {
		t.Fatalf("after narrowing: %d rects for %d washes (was %d)", len(after), len(pane.superGeom.washes), len(before))
	}
	for i, rect := range after {
		want := pane.superGeom.washes[i]
		if rect.Position() != want.pos() || rect.Size() != want.size() {
			t.Errorf("after the resize, wash %d at %v %v, want %v %v", i, rect.Position(), rect.Size(), want.pos(), want.size())
		}
		if rect.Size().Width >= beforeW {
			t.Errorf("wash %d is %v wide after narrowing to 260 (was %v)", i, rect.Size().Width, beforeW)
		}
	}
}

// The title follows like verse 1 does: from far below, a follow on the title
// lands the view at the top; and verse 1, inside the band, then does not yank.
func TestStyledReadAlongFollowScrollsToTheTitle(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer resetStyledWiring()

	st := titledPsalmState("A Psalm of David.")
	armControllerReadAlong(t, st)
	w := buildStyledAreaWindow(t, st)
	defer w.Close()
	pane, scroll := styledPane, styledScroll
	viewH := scroll.Size().Height
	if viewH <= 0 {
		t.Fatal("viewport must have real height")
	}

	// Park the view far down, then narrate the title.
	styledReadAlongApply(25, true)
	if scroll.Offset.Y <= 0 {
		t.Fatal("verse 25 must have scrolled the view down")
	}
	styledReadAlongApply(readAlongTitle, true)
	if scroll.Offset.Y != 0 {
		t.Errorf("follow to the title landed at %v, want 0 (the title's top is the page's top)", scroll.Offset.Y)
	}
	if top, ok := pane.readAlongTop(); !ok || top != pane.superGeom.rect.Y {
		t.Errorf("readAlongTop = %v,%v; want the title block's top %v", top, ok, pane.superGeom.rect.Y)
	}
	// Verse 1 is inside the band now: no scroll.
	styledReadAlongApply(1, true)
	if scroll.Offset.Y != 0 {
		t.Errorf("verse 1 yanked the view to %v right after the title", scroll.Offset.Y)
	}
	// CONTROL: the same follow in an untitled chapter reports nothing to
	// scroll to and leaves the view alone.
	resetStyledWiring()
	st2 := longPsalmState()
	armControllerReadAlong(t, st2)
	w2 := buildStyledAreaWindow(t, st2)
	defer w2.Close()
	styledReadAlongApply(25, true)
	parked := styledScroll.Offset.Y
	styledReadAlongApply(readAlongTitle, true)
	if styledScroll.Offset.Y != parked {
		t.Errorf("an untitled verse 0 moved the view %v -> %v", parked, styledScroll.Offset.Y)
	}
	if _, ok := styledPane.readAlongTop(); ok {
		t.Error("readAlongTop reported a top for a title the chapter does not have")
	}
}
