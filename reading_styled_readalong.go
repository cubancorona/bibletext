package bibletext

// Read-along for the styled desktop pane: the amber tint over the verse being
// narrated, the comfort-band follow-scroll, and the floating "Follow narration"
// pill — the styled twin of the native overlays' read-along (reading_macos.go's
// bibleTextMacHighlightVerse is the reference behaviour). The audio controller
// stays the single source of truth; these helpers only paint and scroll.
//
// Untagged so the whole feature unit-tests on the the development environment. On the
// native-overlay platforms it is dead code: the readAlong* entry points there
// are the native implementations, and nothing constructs a styled pane. The
// Windows/Linux entry points live in readalong_other.go and call these on the
// Fyne goroutine.
//
// Threading: every function here must run on the Fyne UI goroutine (they touch
// widgets and the registered wiring). readalong_other.go wraps the calls in
// fyne.Do; tests call them directly on the test goroutine.

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// styledReadAlongTint matches the native overlays' wash — macOS uses
// (1.0, 0.80, 0.30, 0.32) on both light and dark papers.
var styledReadAlongTint = color.NRGBA{R: 255, G: 204, B: 77, A: 82}

// styledFollowPill is the floating "Follow narration" button registered by the
// current styled area (nil when none). UI goroutine only, like the rest of the
// styled wiring globals.
var styledFollowPill *widget.Button

// styledReassertReadAlong re-issues the controller's read-along state after a
// pane rebuild. A seam (not a direct call) because on the darwin test host the
// controller's readAlong* entry points are the NATIVE overlay's — host tests
// stub this to observe the styled area's reassert wiring.
var styledReassertReadAlong = func() { gAudio.reassertReadAlong() }

// styledRAFollowPending marks a follow request that arrived before the pane
// had real sizes (a chapter rebuild mid-narration). styledColumn.Layout
// consumes it once geometry is real.
var styledRAFollowPending bool

// styledHighlightCeded latches once a narration follow-scroll has moved the
// view: the search/xref highlight positioned it once, and from then on the
// narration owns it — without this, styledColumn.Layout's highlight-owns-
// scroll branch re-pins the view to the highlight inside the very Refresh a
// follow-scroll issues, leaving follow inert whenever a search jump preceded
// Play. Reset on rewire (a fresh pane's highlight positions once again).
//
// IT SURVIVES S2, AND THE SCRAPBOOK'S REASON FOR RETIRING IT WAS A MISREADING.
// [redacted-retired-private-reference] files this latch under "read-along must restore, not
// erase", as the styled pane's way of reconciling two washes that want the same
// pixel — "the styled pane reconciles with a latch, not a model". It does not.
// This latch says nothing about colour: it is about who owns the SCROLL
// POSITION, and the only things that read it are styledColumn.Layout's
// highlightOwnsScroll branch and the rewire that resets it.
//
// The wash collision it was supposed to be papering over does not exist on this
// pane at all. styledReadingPane draws the narration on its own layer of
// rectangles ABOVE the verse wash (raRects over tintRects,
// reading_styled_pane.go), so both are visible at once and neither erases the
// other. That collision is a TextKit constraint — one
// NSBackgroundColorAttributeName per character — and it is confined to the two
// Apple panes, which is exactly where restoreTint/applyTint landed
// (reading_tint_apple.go). Retiring this latch would therefore not remove a
// duplicate model; it would remove the only thing that lets a follow-scroll
// survive a search jump, and read-along would go inert on Windows and Linux.
var styledHighlightCeded bool

// styledReadAlongApply tints the narrated verse (0 clears — the recording's
// intro) and, when follow is set, keeps it inside the comfortable band.
func styledReadAlongApply(verse int, follow bool) {
	pane := styledPane
	if pane == nil {
		return
	}
	pane.setReadAlongVerse(verse)
	if follow && verse > 0 {
		styledRAFollowPending = true
		styledReadAlongFollowScroll()
	}
}

// styledReadAlongClearTint removes the tint (stop / navigation / chapter end).
func styledReadAlongClearTint() {
	styledRAFollowPending = false
	pane := styledPane
	if pane == nil {
		return
	}
	pane.setReadAlongVerse(0)
}

// styledReadAlongFollowScroll moves the narrated verse back into the comfort
// band: only when its top has drifted above the viewport or below 70% of it,
// and then to 30% down — the same thresholds as the native overlays, so the
// text is never yanked on every verse. No-op until the layout has real sizes
// (styledRAFollowPending stays set and styledColumn.Layout retries).
func styledReadAlongFollowScroll() {
	scroll, pane := styledScroll, styledPane
	if scroll == nil || pane == nil || pane.raVerse <= 0 {
		return
	}
	// Re-check suspension NOW, on the UI goroutine: the follow decision was
	// snapshotted on the engine's watcher goroutine and crossed an async
	// fyne.Do hop — a user scroll processed in between must win, or the view
	// gets yanked back with the "Follow narration" pill already up.
	if !gAudio.readAlongFollowActive() {
		styledRAFollowPending = false
		return
	}
	viewH := scroll.Size().Height
	contentH := pane.MinSize().Height
	if viewH <= 0 || contentH <= 0 || scroll.Content.Size().Height < contentH {
		return // geometry not settled; the next layout pass retries
	}
	styledRAFollowPending = false
	vTop, ok := pane.yForVerse(pane.raVerse)
	if !ok {
		return
	}
	visTop := scroll.Offset.Y
	if vTop >= visTop && vTop <= visTop+viewH*0.70 {
		return // inside the comfortable band — leave the reader's view alone
	}
	y := vTop - viewH*0.30
	if max := contentH - viewH; y > max {
		y = max
	}
	if y < 0 {
		y = 0
	}
	if y == scroll.Offset.Y {
		return
	}
	// The narration owns the position now: a pending verse-anchor restore must
	// not snap the view back on the next layout pass, and a search highlight
	// must cede scroll ownership (its one-time jump already happened) — the
	// Refresh below synchronously re-enters styledColumn.Layout, whose
	// highlight branch would otherwise revert this very scroll.
	armStyledRestore(0, 0, 0)
	styledHighlightCeded = true
	styledApplyingScroll = true
	scroll.Offset = fyne.NewPos(0, y)
	scroll.Refresh()
	styledApplyingScroll = false
}

// styledReadAlongSetPill shows/hides the floating "Follow narration" button.
func styledReadAlongSetPill(show bool) {
	pill := styledFollowPill
	if pill == nil {
		return
	}
	if show {
		pill.Show()
	} else {
		pill.Hide()
	}
}

// styledFollowPillLayer builds the overlay layer holding the pill, bottom-
// centred over the reading area, and registers the pill. The layer itself
// never intercepts events — only the button hit-tests.
func styledFollowPillLayer() fyne.CanvasObject {
	pill := widget.NewButton("Follow narration", func() {
		gAudio.resumeReadAlongFollow()
	})
	pill.Importance = widget.HighImportance
	pill.Hide()
	styledFollowPill = pill
	return container.NewBorder(nil,
		container.NewPadded(container.NewCenter(pill)), nil, nil)
}
