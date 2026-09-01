package bibletext

// The styled pane's scroll area — the milestone-4 assembly that replaces
// chapterText's readingScrollArea on Windows/Linux (gated by useStyledPane;
// the per-platform constants live in reading_styled_platform_*.go). Untagged
// so the whole assembly — column layout, highlight scroll-to, anchor
// capture/arm/apply — runs in the default host test suite; on iOS/macOS/Android it
// is dead code behind a false constant and nothing references the shipping
// panes' behaviour.
//
// The wiring mirrors reading_scroll_fyne.go's semantics exactly (carry-over
// on same-chapter re-render, explicit-restore precedence, OnScrolled disarm +
// flush, restore staying armed across resizes) with one improvement: the
// pane's line geometry is exact, so verse anchors restore to the pixel.

import (
	"fmt"
	"os"

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

// styledViewportSettled records that the CURRENT pane's viewport is a real
// position: its Layout has run with real sizes at least once. captureStyledAnchor
// cannot tell "the reader is at the top" from "this pane never laid out" — both
// read as offset 0 — and carrying the second as the first is how an arrival got
// swallowed: the share-link handler surfaces the Read tab (one rebuild, which
// consumes forceReposition and arms the placement), the dev row's own
// switch-to-read rebuilds AGAIN in the same event, and the second wire captured
// the first pane's never-laid-out zero as "top", claimed the placement for it
// (carryTop) and CEDED the highlight — so the note the reader tapped never
// scrolled into view. A pane that has not settled has no position to carry.
var styledViewportSettled bool
var styledLastGateTrace string

// styledScrollTrace prints the styled scroll machinery's decisions when
// BT_SCROLL_DEBUG is set — the same switch the native panes honour. The
// placement chain has failed live while every host probe passed, twice; the
// only cure for diagnosing from a harness is the app narrating itself.
func styledScrollTrace(format string, args ...any) {
	if os.Getenv("BT_SCROLL_DEBUG") == "" {
		return
	}
	fmt.Fprintf(os.Stderr, "[styled] "+format+"\n", args...)
}

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
// persistence. An explicit placement lets the pane's highlight choose the
// viewport; otherwise a saved restore outranks same-chapter carry-over.
func wireStyledReadingScroll(state *AppState, scroll *container.Scroll, pane *styledReadingPane) {
	if pane == nil {
		styledScroll, styledPane, styledState, styledFP = nil, nil, nil, ""
		styledFollowPill = nil
		styledRAFollowPending = false
		return
	}
	newFP := styledPaneFP(state)

	carryVerse, carryDelta, carryFrac := 0, 0.0, 0.0
	carry, carryTop := false, false
	if styledAnchorActive() && styledFP == newFP && styledViewportSettled {
		if v, d, f, ok := captureStyledAnchor(); ok {
			carryVerse, carryDelta, carryFrac, carry = v, d, f, true
			carryTop = v <= 0 && f <= 0
		}
	}

	styledScroll, styledPane, styledState = scroll, pane, state
	styledFP = newFP
	styledUserScrolled = false
	styledHighlightCeded = false
	styledViewportSettled = false

	branch := "default"
	switch {
	case state.forceReposition:
		branch = "forceReposition"
		state.forceReposition = false
		state.restore = nil
		armStyledRestore(0, 0, 0)
	case state.restore != nil:
		branch = "pendingRestore"
		armPendingRestore(state) // shared: matches chapter, drops stale targets
	case carryTop:
		branch = "carryTop"
		// Top is a meaningful viewport position even though its serialized
		// anchor is all zeroes. Claim the placement so a standing note wash
		// cannot auto-scroll the replacement pane away from the top.
		armStyledRestore(0, 0, 0)
		styledHighlightCeded = true
	case carry:
		branch = "carry"
		armStyledRestore(carryVerse, carryDelta, carryFrac)
	default:
		armStyledRestore(0, 0, 0)
	}
	styledScrollTrace("wire: branch=%s carry=%v(v%d f%.2f) top=%v armed=%v", branch, carry, carryVerse, carryFrac, carryTop, styledRestoreArmed)

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
		styledScrollTrace("user scrolled (offset %.0f)", scroll.Offset.Y)
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
	styledRestoreLastContentH = -1
}

// styledRestoreLastContentH is the content height the previous layout pass
// reported while a restore was armed. The restore applies only once the
// height REPEATS — i.e. against geometry that has stopped reflowing. Applied
// on the first pass, it spent itself against the pre-wrap layout (every Y
// roughly doubled), ceded the highlight, and the reflow then left the reader
// at a position that meant nothing; the launch restore only ever survived
// that because a bottom-of-chapter fraction clamps to the bottom either way.
var styledRestoreLastContentH float32 = -1

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
	// GEOMETRY MUST HAVE STOPPED MOVING. A fresh pane's first pass lays out
	// before the wrap settles, and a Y computed there is wrong by the whole
	// reflow (see styledRestoreLastContentH).
	if contentH != styledRestoreLastContentH {
		styledScrollTrace("restore DEFER: contentH %.0f (was %.0f)", contentH, styledRestoreLastContentH)
		styledRestoreLastContentH = contentH
		return
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
	styledScrollTrace("restore APPLY y=%.0f (v%d f%.2f)", y, styledRestoreVerse, styledRestoreFrac)
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
	if os.Getenv("BT_SCROLL_DEBUG") != "" && l.scroll == styledScroll && l.pane != nil {
		gate := fmt.Sprintf("hlOwns=%v user=%v ceded=%v restoreArmed=%v hlY=%.0f off=%.0f",
			l.pane.highlightOwnsScroll(), styledUserScrolled, styledHighlightCeded,
			styledRestoreArmed, l.pane.highlightY(), l.scroll.Offset.Y)
		if gate != styledLastGateTrace {
			styledLastGateTrace = gate
			styledScrollTrace("layout gate: %s", gate)
		}
	}
	if l.scroll != nil && l.pane != nil && l.pane.highlightOwnsScroll() &&
		!styledUserScrolled && !styledHighlightCeded && !styledRestoreArmed {
		// The SHARED arrival lead (noteMetrics().Lead), not a local literal:
		// five surfaces each had their own and the same arrival sat at four
		// different heights. This pane's 24 outlived the step that was
		// supposed to remove it, which a live trace exposed.
		y := l.pane.highlightY() - noteMetrics().Lead
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
			styledScrollTrace("layout PLACE y=%.0f (was %.0f)", y, l.scroll.Offset.Y)
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

	// The viewport is a real position now — this pane laid out with real sizes,
	// so whatever its offset is, it is a fact about the reader and may be
	// carried into a same-chapter rebuild (see styledViewportSettled).
	if l.scroll == styledScroll && l.scroll != nil && l.pane != nil &&
		l.scroll.Size().Height > 0 && l.pane.MinSize().Height > 0 {
		styledViewportSettled = true
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
