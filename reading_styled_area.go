package bibletext

// The styled pane's scroll area — the milestone-4 assembly that replaces
// chapterText's readingScrollArea on Windows/Linux (gated by useStyledPane;
// the per-platform constants live in reading_styled_platform_*.go). Untagged
// so the whole assembly — column layout, highlight scroll-to, anchor
// capture/arm/apply — unit-tests on the dev machine; on iOS/macOS/Android it
// is dead code behind a false constant and nothing references the shipping
// panes' behaviour.
//
// The wiring mirrors reading_scroll_fyne.go's semantics exactly (carry-over
// on same-chapter re-render, explicit-restore precedence, OnScrolled disarm +
// flush, restore staying armed across resizes) with one improvement: the
// pane's line geometry is exact, so verse anchors restore to the pixel.

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// useStyledPane reports whether THIS platform's desktop reading pane is the
// styled one. Constant per build: true on Windows/Linux, false on the
// native-overlay platforms and the Android Fyne fallback — their paths stay
// byte-identical.
// useStyledPane is a var, not a call to the constant, for the same reason
// nativeNoteSticker is (notes_banner.go:32): the platform answer is a build-tag
// constant, so on any one machine only one of the two reading surfaces exists to
// be tested. Tests select the surface they mean to exercise; the default is the
// exact constant the call site used before, and nothing in the app assigns it.
//
// This matters for view tests specifically. On darwin the verses are drawn by a
// native NSTextView floating above the canvas, so there is no Fyne text to find
// and "can the reader see the verses?" cannot be asked of the object tree at
// all. Pointing this at the styled pane asks it of the surface Windows and Linux
// actually ship.
var useStyledPane = func() bool { return styledPaneEnabledOnPlatform }

// --- Live registration (one styled pane at a time, UI goroutine only) --------

var (
	styledScroll *container.Scroll
	styledPane   *styledReadingPane
	styledState  *AppState
	styledFP     string

	styledRestoreVerse int
	styledRestoreDelta float64
	styledRestoreFrac  float64
	styledRestoreArmed bool

	styledApplyingScroll bool

	// styledUserScrolled latches once the reader scrolls this wiring's pane;
	// until then the highlight scroll-to re-asserts across layout passes (a
	// resize between the first pass and the settled one moves the highlight —
	// the old one-shot latch kept a stale offset).
	styledUserScrolled bool
)

// styledAnchorActive reports whether the styled pane owns scroll persistence.
func styledAnchorActive() bool { return styledScroll != nil && styledPane != nil }

func styledPaneFP(state *AppState) string {
	return fmt.Sprintf("%s\x00%d\x00%s", state.CurrentBook, state.CurrentChapter, state.CurrentVersion)
}

// styledReadingScrollArea is the styled twin of readingScrollArea.
func styledReadingScrollArea(state *AppState, verses []Verse, pal palette) fyne.CanvasObject {
	col := &styledColumn{maxWidth: 760}
	var child fyne.CanvasObject
	var pane *styledReadingPane
	if len(verses) == 0 {
		msg := widget.NewLabel("No verses are available for this chapter yet.")
		msg.Wrapping = fyne.TextWrapWord
		child = msg
	} else {
		pane = newStyledReadingPane(state, verses)
		col.pane = pane
		child = pane
	}

	scroll := container.NewVScroll(container.New(col, child))
	col.scroll = scroll

	wireStyledReadingScroll(state, scroll, pane)

	// A narration already playing for THIS chapter re-asserts its read-along
	// state onto the fresh pane (the styled twin of Android's afterRebuild
	// reassert): the tint returns immediately instead of waiting for the next
	// verse boundary, and the "Follow narration" pill survives a theme flip.
	if pane != nil && gAudio.playingFingerprint() == chapterAudioFingerprint(state) {
		styledReassertReadAlong()
	}

	return container.NewStack(
		readingGround(container.NewPadded(scroll), pal.Background),
		styledFollowPillLayer(),
	)
}

// wireStyledReadingScroll registers the freshly built pane for scroll
// persistence — the styled twin of wireFyneReadingScroll, same precedence:
// explicit restore > same-chapter carry-over > top.
func wireStyledReadingScroll(state *AppState, scroll *container.Scroll, pane *styledReadingPane) {
	if pane == nil {
		styledScroll, styledPane, styledState, styledFP = nil, nil, nil, ""
		styledFollowPill = nil
		styledRAFollowPending = false
		return
	}
	newFP := styledPaneFP(state)

	carryVerse, carryDelta, carryFrac := 0, 0.0, 0.0
	carry := false
	if styledAnchorActive() && styledFP == newFP {
		if v, d, f, ok := captureStyledAnchor(); ok && (v > 0 || f > 0) {
			carryVerse, carryDelta, carryFrac, carry = v, d, f, true
		}
	}

	styledScroll, styledPane, styledState = scroll, pane, state
	styledFP = newFP
	styledUserScrolled = false
	styledHighlightCeded = false

	switch {
	case state.restore != nil:
		armPendingRestore(state) // shared: matches chapter, drops stale targets
	case carry:
		armStyledRestore(carryVerse, carryDelta, carryFrac)
	default:
		armStyledRestore(0, 0, 0)
	}

	lastView := scroll.Size()
	lastContentH := float32(0)
	if pane != nil {
		lastContentH = pane.MinSize().Height
	}
	scroll.OnScrolled = func(fyne.Position) {
		if styledApplyingScroll || styledScroll != scroll {
			return
		}
		// IS THIS THE READER, OR FYNE CORRECTING ITSELF? fyne fires OnScrolled
		// for its own offset CLAMPS — a window resize, a re-wrap, anything that
		// moves the maximum offset — and a clamp is not the reader taking over.
		// This check existed before but ran three lines too late: by then
		// styledUserScrolled was already true and state.restore already nil, so
		// a plain window resize cancelled the arrival scroll a shared link had
		// just armed and threw the saved reading position away with it. The
		// comment said clamps "must not" count as the reader; the code counted
		// them and then returned.
		//
		// Moving it up is necessary but not sufficient, because "the geometry
		// changed" alone cannot separate a clamp from a reader who scrolls
		// immediately after resizing the window. What separates them is WHERE
		// the offset ends up: a clamp is forced onto a BOUND — the new maximum,
		// or zero — while a reader lands wherever they please. So both must
		// hold: the geometry moved, and the offset is sitting on a bound.
		v, ch := scroll.Size(), pane.MinSize().Height
		geometryMoved := v != lastView || ch != lastContentH
		if geometryMoved {
			lastView, lastContentH = v, ch
			maxY := ch - v.Height
			if maxY < 0 {
				maxY = 0
			}
			y := scroll.Offset.Y
			const onTheBound = 0.5
			if y <= onTheBound || y >= maxY-onTheBound {
				return // fyne's own correction, not the reader's intent
			}
		}
		styledRestoreArmed = false
		styledUserScrolled = true
		state.restore = nil
		flushReadingStateAsync(state)
		// A reader's own scroll during read-along stops the follow (the tint
		// keeps tracking; the pill offers the way back).
		styledRAFollowPending = false // the reader's intent wins over a queued follow
		gAudio.onReadAlongUserScroll()
	}
}

// captureStyledAnchor is the styled pane's capture half.
func captureStyledAnchor() (verse int, delta, frac float64, ok bool) {
	scroll, pane := styledScroll, styledPane
	if scroll == nil || pane == nil {
		return 0, 0, 0, false
	}
	viewH := scroll.Size().Height
	contentH := pane.MinSize().Height
	if viewH <= 0 || contentH <= 0 {
		return 0, 0, 0, false
	}
	y := scroll.Offset.Y
	if y <= 0 {
		return 0, 0, 0, true
	}
	if max := contentH - viewH; max > 0 {
		frac = float64(y / max)
		if frac > 1 {
			frac = 1
		}
	}
	verse, delta = pane.verseAtY(y)
	return verse, delta, frac, true
}

// armStyledRestore arms (all-zero disarms) the styled restore target.
func armStyledRestore(verse int, delta, frac float64) {
	styledRestoreVerse, styledRestoreDelta, styledRestoreFrac = verse, delta, frac
	styledRestoreArmed = verse > 0 || frac > 0
}

// applyStyledReadingRestore applies the armed target once sizes are real;
// called from styledColumn.Layout each pass. Stays armed across resizes until
// the reader scrolls (native parity).
func applyStyledReadingRestore(col *styledColumn) {
	if !styledRestoreArmed || col.scroll == nil || col.pane == nil {
		return
	}
	// RESTORE BEFORE HIGHLIGHT — this pane used to do the opposite, disarming
	// the restore here because "a search/xref jump owns the position". iOS
	// states the rule the other way (reading_ios.go, bibleTextIOSScrollTV) and
	// is right: a pending restore only ever exists on a REOPEN, because every
	// explicit arrival clears it (share_link_open.go, openSearchResultRange) so
	// that it falls through to the highlight. An armed restore therefore means
	// "the reader is coming back", and coming back should land where they
	// stopped reading — not on whatever happens to be highlighted there. With
	// the old order, reopening onto a chapter carrying a note or a search hit
	// dragged the reader to it every launch.
	viewH := col.scroll.Size().Height
	contentH := col.pane.MinSize().Height
	if viewH <= 0 || contentH <= 0 {
		return
	}
	if col.scroll.Content.Size().Height < contentH {
		return // content box not re-laid-out yet; next pass applies
	}
	maxOff := contentH - viewH
	if maxOff <= 0 {
		styledRestoreArmed = false
		return
	}
	var y float32
	if styledRestoreVerse > 0 {
		if vy, ok := col.pane.yForVerse(styledRestoreVerse); ok {
			y = vy + float32(styledRestoreDelta)
		} else {
			y = float32(styledRestoreFrac) * maxOff
		}
	} else {
		y = float32(styledRestoreFrac) * maxOff
	}
	if y < 0 {
		y = 0
	}
	if y > maxOff {
		y = maxOff
	}
	if y == col.scroll.Offset.Y {
		// Already there — but still claim the placement, or the highlight block
		// in Layout takes it on the next pass.
		styledHighlightCeded = true
		styledRestoreArmed = false
		return
	}
	styledApplyingScroll = true
	col.scroll.Offset = fyne.NewPos(0, y)
	col.scroll.Refresh()
	styledApplyingScroll = false
	// The reopen has been placed. Claim it, so the highlight block in Layout
	// does not take the position back on the very next pass, and disarm so this
	// runs once.
	styledHighlightCeded = true
	styledRestoreArmed = false
	if styledState != nil {
		updateCurrentVisitAnchor(styledState, styledRestoreVerse, styledRestoreDelta, styledRestoreFrac)
	}
}

// --- Column layout ------------------------------------------------------------

// styledColumn centres the pane at a book-like measure and owns the one-shot
// highlight scroll-to plus the restore apply — the styled twin of
// readingColumn, minus the external band (the pane draws its own).
type styledColumn struct {
	maxWidth float32
	scroll   *container.Scroll
	pane     *styledReadingPane
}

func (l *styledColumn) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) == 0 {
		return
	}
	child := objects[0]

	w := size.Width
	if w > l.maxWidth {
		w = l.maxWidth
	}
	if w < 0 {
		w = 0
	}
	x := (size.Width - w) / 2
	if x < 0 {
		x = 0
	}

	// Width first so wrapping reflows, then size to the resulting height —
	// the same two-pass idiom as readingColumn.
	child.Resize(fyne.NewSize(w, child.MinSize().Height))
	child.Resize(fyne.NewSize(w, child.MinSize().Height))
	child.Move(fyne.NewPos(x, 0))

	// styledRestoreArmed joins the guard for the same reason macOS's blocks were
	// reordered: a reopen's saved position outranks whatever is highlighted on
	// the page. The restore applies a few lines below, in the same pass or a
	// later one once the sizes settle; until then the highlight must not take
	// the position, or the reader is placed twice and sees the second.
	if l.scroll != nil && l.pane != nil && l.pane.highlightOwnsScroll() &&
		!styledUserScrolled && !styledHighlightCeded && !styledRestoreArmed {
		y := l.pane.highlightY() - 24
		if y < 0 {
			y = 0
		}
		// CLAMP THE TOP END TOO. Only the floor was clamped, so a note or a
		// search hit near the END of a chapter asked for an offset past the
		// scroll's maximum. fyne then clamps it itself and reports the
		// correction through OnScrolled — which, before the fix above, was
		// indistinguishable from the reader scrolling and cancelled the very
		// arrival that caused it. Asking for a reachable offset means fyne has
		// nothing to correct and nothing to report.
		if max := l.pane.MinSize().Height - l.scroll.Size().Height; y > max {
			y = max
			if y < 0 {
				y = 0
			}
		}
		if y != l.scroll.Offset.Y {
			styledApplyingScroll = true
			l.scroll.Offset = fyne.NewPos(0, y)
			l.scroll.Refresh()
			styledApplyingScroll = false
		}
	}

	if l.scroll != nil && l.pane != nil {
		applyStyledReadingRestore(l)
	}

	// A follow request that arrived before sizes were real (chapter rebuild
	// mid-narration) applies as soon as geometry settles. After the restore
	// logic: the narration's comfort-band scroll wins over a stale anchor.
	if styledRAFollowPending && l.scroll == styledScroll {
		styledReadAlongFollowScroll()
	}
}

func (l *styledColumn) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) == 0 {
		return fyne.Size{}
	}
	min := objects[0].MinSize()
	if min.Width > l.maxWidth {
		min.Width = l.maxWidth
	}
	return min
}
