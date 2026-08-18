package bibletext

// selSpan is the verse range a reading-pane selection covers, resolved
// POSITIONALLY by the pane the selection was made in — the native overlays map
// the selected character range onto their verse-number index (reading_ios.go /
// reading_macos.go / BtBridge.java), the styled pane reads it off its own runs
// (reading_styled_select.go). Position is authoritative: text matching cannot
// tell apart repeated wording (Psalm 136's refrain appears in all 26 verses)
// and its 8-rune probe floor resolves short selections to nothing, so the
// selection's position must survive the trip through the dispatch layer instead
// of being rediscovered from the words.
//
// The zero value means "position unknown" — the legacy Fyne Entry pane, or a
// native layer that could not resolve the range — and every consumer then falls
// back to the old text matching (selectionVerses, normalizeShareSelection).
type selSpan struct{ lo, hi int }

func (s selSpan) valid() bool { return s.lo >= 1 && s.hi >= s.lo }

// selSpanFromNative normalizes a lo/hi pair as the panes report it. hi<=0 is
// "no span". lo==0 with hi>0 is a selection that starts ABOVE verse 1's number
// (the chapter heading) — the first scripture it can touch is verse 1, so it
// clamps there rather than degrading the whole span to the matching fallback.
func selSpanFromNative(lo, hi int) selSpan {
	if hi <= 0 {
		return selSpan{}
	}
	if lo < 1 {
		lo = 1
	}
	if hi < lo {
		lo, hi = hi, lo
	}
	return selSpan{lo: lo, hi: hi}
}
