//go:build !ios && !darwin && !android

package bibletext

// Integration tests for the Linux/Windows within-chapter scroll capture/restore
// (reading_scroll_fyne.go), using real widgets under the fyne test driver.
// These run in the linux and windows CI jobs — the platforms whose build this
// file belongs to.

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// buildTestPane renders the current chapter in a small window so the text
// overflows the viewport. The window is sized BEFORE the content is set, so the
// first layout pass — the one that applies a pending restore — already runs at
// the real geometry (a post-SetContent Resize would make the first pass run at
// min-size and the assertions accidental).
//
// Built via chapterTextScrollArea, NOT readingScrollArea: on this file's
// platforms the dispatcher returns the styled pane, and these tests lock the
// LEGACY pane's wiring (still shipping as the Android fallback and the desktop
// burn-in fallback). The styled globals are cleared first so the capture/arm
// delegation in reading_scroll_fyne.go stays on the legacy path.
func buildTestPane(t *testing.T, state *AppState) fyne.Window {
	t.Helper()
	resetStyledWiring()
	verses := state.Bible.GetChapter(state.CurrentBook, state.CurrentChapter)
	if len(verses) == 0 {
		t.Fatal("sample chapter is empty")
	}
	area := chapterTextScrollArea(state, verses, state.pal())
	w := fyne.CurrentApp().NewWindow("bt-scroll-test")
	w.Resize(fyne.NewSize(260, 140)) // narrow + short → lots of wrapped lines, scrollable
	w.SetContent(area)
	return w
}

func TestFyneScrollCaptureAndRestore(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	state := sampleState()
	w := buildTestPane(t, state)
	defer w.Close()

	scroll, chapter := fyneReadingScroll, fyneReadingChapter
	if scroll == nil || chapter == nil {
		t.Fatal("readingScrollArea must register the pane")
	}
	maxOff := chapter.MinSize().Height - scroll.Size().Height
	if maxOff <= 0 {
		t.Fatalf("test pane must be scrollable (content %v, viewport %v)", chapter.MinSize().Height, scroll.Size().Height)
	}

	// Reader mid-chapter.
	target := maxOff / 2
	scroll.Offset = fyne.NewPos(0, target)

	verse, delta, frac, ok := captureReadingAnchor()
	if !ok {
		t.Fatal("captureReadingAnchor must succeed with a live pane")
	}
	if verse == 0 && frac == 0 {
		t.Fatal("mid-chapter capture must produce an anchor")
	}
	if frac <= 0 || frac >= 1 {
		t.Fatalf("mid-chapter frac out of range: %v", frac)
	}

	// Fresh render of the same chapter with a pending restore (the launch /
	// history-tap path): state.restore is armed, readingScrollArea re-arms it
	// via armPendingRestore, and the layout pass applies it.
	state.restore = &restoreAnchor{
		Book:    state.CurrentBook,
		Chapter: state.CurrentChapter,
		Verse:   verse,
		Delta:   delta,
		Frac:    frac,
	}
	w2 := buildTestPane(t, state)
	defer w2.Close()

	got := fyneReadingScroll.Offset.Y
	if diff := got - target; diff < -2 || diff > 2 {
		t.Fatalf("restore landed at %v, want ~%v (anchor verse %d delta %v frac %v)", got, target, verse, delta, frac)
	}

	// The target stays armed (it re-asserts across resizes, native-style) until
	// a user scroll disarms it — covered by TestFyneUserScrollDropsRestore.
	if !fyneRestoreArmed {
		t.Fatal("restore must stay armed after applying (re-assert until a user scroll)")
	}
}

func TestFyneSameChapterRerenderKeepsPosition(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	state := sampleState()
	state.restore = nil
	w := buildTestPane(t, state)
	defer w.Close()

	scroll, chapter := fyneReadingScroll, fyneReadingChapter
	maxOff := chapter.MinSize().Height - scroll.Size().Height
	if maxOff <= 0 {
		t.Fatal("test pane must be scrollable")
	}

	// The reader scrolls (a genuine user scroll: OnScrolled fires and clears
	// any pending restore), then something re-renders the pane — a theme flip,
	// a settings change — with no state.restore. The new pane must carry the
	// position over instead of resetting to the top.
	target := maxOff * 3 / 4
	scroll.Offset = fyne.NewPos(0, target)
	scroll.OnScrolled(fyne.NewPos(0, target))

	w2 := buildTestPane(t, state) // same book/chapter/version → carry-over
	defer w2.Close()

	got := fyneReadingScroll.Offset.Y
	if diff := got - target; diff < -2 || diff > 2 {
		t.Fatalf("same-chapter re-render landed at %v, want ~%v (carry-over)", got, target)
	}
}

func TestFyneUserScrollDropsRestore(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	state := sampleState()
	state.restore = &restoreAnchor{Book: state.CurrentBook, Chapter: state.CurrentChapter, Frac: 0.4}
	w := buildTestPane(t, state)
	defer w.Close()

	// Simulate the user wheel-scrolling: fyne fires OnScrolled for user input.
	if fyneReadingScroll.OnScrolled == nil {
		t.Fatal("pane must hook OnScrolled")
	}
	fyneReadingScroll.OnScrolled(fyne.NewPos(0, 50))

	if state.restore != nil {
		t.Fatal("a user scroll must drop the pending restore (native parity)")
	}
	if fyneRestoreArmed {
		t.Fatal("a user scroll must disarm the restore target")
	}
}

func TestFyneRestoreYieldsToHighlightJump(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	state := sampleState()
	verses := state.Bible.GetChapter(state.CurrentBook, state.CurrentChapter)
	// A search-jump highlight owns the scroll; a stale restore must not fight it.
	state.setHL(hlSearch, state.CurrentBook, state.CurrentChapter, verses[len(verses)-1].Verse, 0)
	state.restore = &restoreAnchor{Book: state.CurrentBook, Chapter: state.CurrentChapter, Frac: 0.1}

	w := buildTestPane(t, state)
	defer w.Close()

	if fyneRestoreArmed {
		t.Fatal("restore must disarm when a highlight jump owns the scroll")
	}

	// Reset the highlight so later tests in this binary see clean state.
	state.clearMark()
	state.restore = nil
}
