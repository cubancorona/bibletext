//go:build !ios && !darwin && !android

package bibletext

// Within-chapter scroll capture/restore for the Fyne reading pane — the
// Linux/Windows twin of the native overlays' anchor plumbing, so reading
// position and history-tap restore reach feature parity with macOS/iOS/Android.
//
// The anchor is verse-based, like the native ones: chapterText records each
// verse's first wrapped line during rewrap (reading.go, verseLines), so capture
// maps the scroll offset to (top-visible verse, px past its top) and restore
// maps it back through the CURRENT wrap — surviving window resizes, text-size
// changes, and restarts. ScrollFrac (0..1 of scrollable height) rides along as
// the cross-layout fallback, exactly as reading_state.go documents.
//
// Wiring: readingScrollArea calls wireFyneReadingScroll on every chapter render
// (a fresh container.Scroll each time — the native views persist, this pane
// does not). Three arming sources, in precedence order:
//   1. state.restore — an explicit target (launch restore, history tap), armed
//      via the shared armPendingRestore;
//   2. carry-over — a SAME-chapter re-render (theme flip, settings change,
//      search-clear, focus toggle) captures the outgoing pane's live position
//      and re-arms it, so a rebuild never yanks the reader to the top;
//   3. nothing — a plain navigation lands at the top.
// readingColumn.Layout applies the armed target once sizes are real
// (applyFyneReadingRestore) and KEEPS it armed so later layout passes (window
// resizes) re-anchor through the new wrap — native overlays re-assert their
// target the same way — until the reader's own scroll disarms it.
//
// OnScrolled fires for user scrolling (wheel, scrollbar) and ALSO when a resize
// clamps the offset; our own programmatic apply is latched out, and a clamp is
// treated like a user move (the position genuinely changed, and any pending
// target has already been applied by then).

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
)

// The live reading pane. One exists at a time — each chapter render builds a
// fresh scroll + chapterText — mirroring the native gReadingTV singletons. All
// access is on the Fyne UI goroutine (capture runs from OnScrolled or the
// lifecycle flush; arm/apply from build/layout).
var (
	fyneReadingScroll  *container.Scroll
	fyneReadingChapter *chapterText
	fyneReadingState   *AppState
	fyneReadingFP      string // book/chapter/version the wired pane shows

	// Restore target armed by armReadingRestore (via the shared
	// armPendingRestore) or the same-chapter carry-over, applied by
	// applyFyneReadingRestore on layout passes with real sizes. Stays armed
	// until the reader scrolls (re-asserting across resizes, like native).
	fyneRestoreVerse int
	fyneRestoreDelta float64
	fyneRestoreFrac  float64
	fyneRestoreArmed bool

	// Latched while WE move the offset (the restore apply), so the OnScrolled
	// handler ignores our own programmatic set.
	fyneApplyingScroll bool
)

// fynePaneFP identifies what the pane is showing, for the carry-over check.
func fynePaneFP(state *AppState) string {
	return fmt.Sprintf("%s\x00%d\x00%s", state.CurrentBook, state.CurrentChapter, state.CurrentVersion)
}

// wireFyneReadingScroll registers the freshly built pane and hooks user scrolls.
// A nil chapter (an empty chapter's placeholder) clears the registration so a
// stale pane can't serve captures.
func wireFyneReadingScroll(state *AppState, scroll *container.Scroll, chapter *chapterText) {
	if chapter == nil {
		fyneReadingScroll, fyneReadingChapter, fyneReadingState, fyneReadingFP = nil, nil, nil, ""
		return
	}
	newFP := fynePaneFP(state)

	// Same-chapter re-render (theme flip, settings change, search-clear, …):
	// capture the OUTGOING pane's live position before overwriting the
	// registration, so the rebuild doesn't silently reset the reader to the top
	// (and the next flush doesn't persist that top over a good anchor).
	carryVerse, carryDelta, carryFrac := 0, 0.0, 0.0
	carry := false
	if fyneReadingScroll != nil && fyneReadingChapter != nil && fyneReadingFP == newFP {
		if v, d, f, ok := captureReadingAnchor(); ok && (v > 0 || f > 0) {
			carryVerse, carryDelta, carryFrac, carry = v, d, f, true
		}
	}

	fyneReadingScroll, fyneReadingChapter, fyneReadingState = scroll, chapter, state
	fyneReadingFP = newFP

	switch {
	case state.restore != nil:
		// An explicit target (launch restore / history tap) wins. Shared logic:
		// matches the chapter, drops stale targets.
		armPendingRestore(state)
	case carry:
		armReadingRestore(carryVerse, carryDelta, carryFrac)
	default:
		armReadingRestore(0, 0, 0)
	}

	scroll.OnScrolled = func(fyne.Position) {
		// Ignore our own programmatic apply, and any late event from a pane
		// that has since been replaced (its clamp must not clear the new
		// chapter's pending restore).
		if fyneApplyingScroll || fyneReadingScroll != scroll {
			return
		}
		// Native parity (bibleTextReadingScrolled): the reader's own scroll
		// obsoletes any pending restore target, and continuously persists the
		// position — the native platforms flush on scroll-end; fyne has no end
		// event, so every scroll notification flushes instead. That stays cheap:
		// capture is a few struct reads, and flushReadingStateAsync drops
		// overlapping writes (readingStateWriting single-flight, latest-wins seq).
		fyneRestoreArmed = false
		state.restore = nil
		flushReadingStateAsync(state)
	}
}

// captureReadingAnchor reads the live scroll position as (top-visible verse,
// px past its top, fraction of scrollable height). ok=false only when no pane
// is registered or laid out (0 values ⇒ top of chapter, per reading_state.go).
func captureReadingAnchor() (verse int, delta, frac float64, ok bool) {
	// The styled pane (the shipping Windows/Linux pane since the milestone-4
	// swap) registers its own scroll; delegate when it is live. The
	// chapterText path below remains for the one-line fallback flip.
	if styledAnchorActive() {
		return captureStyledAnchor()
	}
	scroll, chapter := fyneReadingScroll, fyneReadingChapter
	if scroll == nil || chapter == nil {
		return 0, 0, 0, false
	}
	viewH := scroll.Size().Height
	contentH := chapter.MinSize().Height
	if viewH <= 0 || contentH <= 0 {
		return 0, 0, 0, false // not laid out yet — nothing meaningful to save
	}
	y := scroll.Offset.Y
	if y <= 0 {
		return 0, 0, 0, true // top of chapter
	}
	if max := contentH - viewH; max > 0 {
		frac = float64(y / max)
		if frac > 1 {
			frac = 1
		}
	}
	verse, delta = chapter.verseAtY(y)
	return verse, delta, frac, true
}

// armReadingRestore arms (or, with all-zero arguments, disarms) the restore
// target that applyFyneReadingRestore applies on layout passes.
func armReadingRestore(verse int, delta, frac float64) {
	if styledAnchorActive() {
		armStyledRestore(verse, delta, frac)
		return
	}
	fyneRestoreVerse, fyneRestoreDelta, fyneRestoreFrac = verse, delta, frac
	fyneRestoreArmed = verse > 0 || frac > 0
}

// applyFyneReadingRestore is called from readingColumn.Layout (reading.go) once
// per pass, after the chapter is sized. It applies the armed target through the
// CURRENT wrap: the verse anchor when the verse exists in this chapter, else
// the fraction fallback. The target STAYS armed — resizes re-anchor through the
// new wrap, native-style — until the reader's own scroll disarms it
// (OnScrolled) or a highlight jump supersedes it.
func applyFyneReadingRestore(l *readingColumn) {
	if !fyneRestoreArmed || l.scroll == nil || l.chapter == nil {
		return
	}
	if l.chapter.highlightLine >= 0 {
		// A highlight jump (search hit, cross-reference) owns the scroll — the
		// restore target is stale. Drop it.
		fyneRestoreArmed = false
		return
	}
	viewH := l.scroll.Size().Height
	contentH := l.chapter.MinSize().Height
	if viewH <= 0 || contentH <= 0 {
		return // sizes not real yet — retry on the next Layout
	}
	if l.scroll.Content.Size().Height < contentH {
		// The scroll's content box hasn't been re-laid out to the current wrap
		// yet; applying now would let the follow-up refresh clamp the offset
		// against the stale extent. The content resize triggers another Layout —
		// apply then.
		return
	}
	maxOff := contentH - viewH
	if maxOff <= 0 {
		fyneRestoreArmed = false // whole chapter fits — top is the only position
		return
	}
	var y float32
	if fyneRestoreVerse > 0 {
		if vy, ok := l.chapter.yForVerse(fyneRestoreVerse); ok {
			y = vy + float32(fyneRestoreDelta)
		} else {
			// Verse not in this chapter's wrap (translation switch trimmed it,
			// say) — fall back to the fraction anchor.
			y = float32(fyneRestoreFrac) * maxOff
		}
	} else {
		y = float32(fyneRestoreFrac) * maxOff
	}
	if y < 0 {
		y = 0
	}
	if y > maxOff {
		y = maxOff
	}
	if y == l.scroll.Offset.Y {
		return
	}
	fyneApplyingScroll = true
	l.scroll.Offset = fyne.NewPos(0, y)
	l.scroll.Refresh()
	fyneApplyingScroll = false
	// A restore is a real position: stamp it onto the current history visit so
	// a no-scroll round trip (chip A → B → back to A → away) keeps A's anchor.
	if fyneReadingState != nil {
		updateCurrentVisitAnchor(fyneReadingState, fyneRestoreVerse, fyneRestoreDelta, fyneRestoreFrac)
	}
}

// captureLastTouch / armReadingMarker back the initial-touch ("you left off
// here") marker, which is iOS-only (it needs a touch gesture and native verse
// geometry). There is nothing to record or draw on the Fyne pane.
func captureLastTouch() (verse int, delta float64, ok bool) { return 0, 0, false }

func armReadingMarker(verse int, r, g, b float64) {}
