package bibletext

// The styled pane's scroll area — the milestone-4 assembly that replaces
// chapterText's readingScrollArea on Windows/Linux (gated by useStyledPane;
// the per-platform constants live in reading_styled_platform_*.go). Untagged
// so the whole assembly — column layout, highlight scroll-to, anchor
// capture/arm/apply — unit-tests on the the development environment; on iOS/macOS/Android it
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
func useStyledPane() bool { return styledPaneEnabledOnPlatform }

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

	return surface(container.NewPadded(scroll), pal.Background, pal.Border, fyne.Size{})
}

// wireStyledReadingScroll registers the freshly built pane for scroll
// persistence — the styled twin of wireFyneReadingScroll, same precedence:
// explicit restore > same-chapter carry-over > top.
func wireStyledReadingScroll(state *AppState, scroll *container.Scroll, pane *styledReadingPane) {
	if pane == nil {
		styledScroll, styledPane, styledState, styledFP = nil, nil, nil, ""
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

	switch {
	case state.restore != nil:
		armPendingRestore(state) // shared: matches chapter, drops stale targets
	case carry:
		armStyledRestore(carryVerse, carryDelta, carryFrac)
	default:
		armStyledRestore(0, 0, 0)
	}

	scroll.OnScrolled = func(fyne.Position) {
		if styledApplyingScroll || styledScroll != scroll {
			return
		}
		styledRestoreArmed = false
		styledUserScrolled = true
		state.restore = nil
		flushReadingStateAsync(state)
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
	if col.pane.highlightOwnsScroll() {
		styledRestoreArmed = false // a search/xref jump owns the position
		return
	}
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
		return
	}
	styledApplyingScroll = true
	col.scroll.Offset = fyne.NewPos(0, y)
	col.scroll.Refresh()
	styledApplyingScroll = false
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

	if l.scroll != nil && l.pane != nil && l.pane.highlightOwnsScroll() && !styledUserScrolled {
		y := l.pane.highlightY() - 24
		if y < 0 {
			y = 0
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
