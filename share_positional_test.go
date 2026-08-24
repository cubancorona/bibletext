package bibletext

// The share pipeline's LATER stages must read the selected copy too. The span
// fix taught the citation which copy of repeated wording was selected; these
// pin the same discipline through the three stages that used to re-find the
// text first-match (sweep findings share.go:332/:565/:846): the trailing-
// sentence completion, the poem/paragraph line-break restore, and the original
// sentence terminal. Each test's fixture puts the SAME words in two places
// with different surroundings, selects the SECOND copy by position, and
// demands the second copy's surroundings — then pins the zero-span path's
// old first-match answer as the recorded reason the offset is threaded.

import (
	"strings"
	"testing"
)

// twoCopyState: the refrain's first copy sits mid-sentence (words follow it),
// the second copy ends its sentence — and in verse 3 the copies differ in
// TERMINAL too (a question), for the terminal test.
func twoCopyState() *AppState {
	bd := &BibleData{
		Books: []string{"Psalms"},
		Verses: map[string]map[int][]Verse{"Psalms": {136: {
			{BookName: "Psalms", Book: "Psalms", Chapter: 136, Verse: 1,
				Text: "Sing to the LORD with thanksgiving and with joy."},
			{BookName: "Psalms", Book: "Psalms", Chapter: 136, Verse: 2,
				Text: "Sing to the LORD with thanksgiving."},
			{BookName: "Psalms", Book: "Psalms", Chapter: 136, Verse: 3,
				Text: "Will you not sing to him? Sing to the LORD with thanksgiving?"},
		}}},
	}
	return &AppState{Bible: bd, CurrentBook: "Psalms", CurrentChapter: 136}
}

// The completion: the selection is every word of verse 2's sentence, missing
// only its period. Located at verse 2 the completion appends it; located at
// verse 1's copy (where "and with joy." follows) a word intervenes and the
// completion must refuse — which is exactly what the zero-span path does,
// pinned below as the old defect.
func TestCompleteTrailingSentenceReadsTheSelectedCopy(t *testing.T) {
	st := twoCopyState()
	sel := "Sing to the LORD with thanksgiving"

	text, cite, _, _ := prepareShareQuote(st, sel, selSpanFromNative(2, 2))
	if text != sel+"." {
		t.Errorf("positional: completion = %q, want the period appended (verse 2's copy is sentence-final)", text)
	}
	if cite != "Psalms 136:2" {
		t.Errorf("positional: cite = %q, want Psalms 136:2", cite)
	}

	// The zero span locates verse 1's copy, where "and with joy" follows — no
	// completion, and the citation names verse 1: the recorded old behavior.
	text, cite, _, _ = prepareShareQuote(st, sel, selSpan{})
	if text != sel {
		t.Errorf("zero span: completion = %q, want the selection unchanged (a word follows the first copy)", text)
	}
	if cite != "Psalms 136:1" {
		t.Errorf("zero span: cite = %q — the misattribution the span exists to fix", cite)
	}
}

// The terminal: verse 3's copy of the words ends its sentence with a question
// mark; verse 2's identical words end with a period. The four-dot slot must
// carry the SELECTED copy's terminal.
func TestOriginalSentenceTerminalReadsTheSelectedCopy(t *testing.T) {
	st := twoCopyState()
	sel := "Sing to the LORD with"

	_, _, at, baseLen := prepareShareQuote(st, sel, selSpanFromNative(3, 3))
	if at < 0 {
		t.Fatal("precondition: the span locate should pin the selection in verse 3")
	}
	if got := originalSentenceTerminal(st, sel, at, baseLen); got != '?' {
		t.Errorf("positional terminal = %q, want '?' (verse 3's sentence)", got)
	}
	// Unlocated, the scan runs from verse 1's copy: a period. The old answer.
	if got := originalSentenceTerminal(st, sel, -1, 0); got != '.' {
		t.Errorf("zero span terminal = %q, want the first copy's '.'", got)
	}
}

// The line-break restore: the same words appear inside one poem line in verse 1
// and CROSSING an authored line break in verse 2. The share of verse 2's copy
// must break where verse 2 breaks; the located offset is what knows that.
func TestRestoreShareLineBreaksReadsTheSelectedCopy(t *testing.T) {
	bd := &BibleData{
		Books: []string{"Psalms"},
		Verses: map[string]map[int][]Verse{"Psalms": {136: {
			{BookName: "Psalms", Book: "Psalms", Chapter: 136, Verse: 1,
				Text: "praise him sun and moon in the heights above."},
			{BookName: "Psalms", Book: "Psalms", Chapter: 136, Verse: 2,
				Text: "praise him sun\nand moon in the heights beneath."},
		}}},
	}
	st := &AppState{Bible: bd, CurrentBook: "Psalms", CurrentChapter: 136}
	sel := "praise him sun and moon"

	text, _, at, baseLen := prepareShareQuote(st, sel, selSpanFromNative(2, 2))
	restored := restoreShareLineBreaks(st, text, at, baseLen)
	if !strings.Contains(restored, "sun\nand") {
		t.Errorf("positional restore = %q, want verse 2's authored break after %q", restored, "sun")
	}

	// The zero span locates verse 1's copy — one unbroken line: no break
	// restored. The recorded old behavior for the same drag.
	text, _, at, baseLen = prepareShareQuote(st, sel, selSpan{})
	if restored := restoreShareLineBreaks(st, text, at, baseLen); strings.Contains(restored, "\n") {
		t.Errorf("zero span restore = %q injected a break the first copy does not have", restored)
	}
}

// The coordinate invariant every positional stage rests on: chapterProse and
// chapterShareStructure flatten to the SAME string, so an offset located in one
// indexes the other. If these ever drift, the offset threading breaks silently
// — this is the loud version.
func TestChapterProseAndShareStructureAgree(t *testing.T) {
	for _, st := range []*AppState{twoCopyState(), refrainChapterState(), sampleState()} {
		prose, _ := chapterProse(st)
		flat, _ := chapterShareStructure(st)
		if prose != flat {
			t.Fatalf("chapterProse and chapterShareStructure diverge for %s %d:\n %q\nvs %q",
				st.CurrentBook, st.CurrentChapter, prose, flat)
		}
	}
}

// The verbs agree on the dangling-number drag: a selection swept just past the
// next verse's NUMBER spans (2,3) natively, but no word of verse 3 is quoted —
// the share cites verse 2, and the crossref resolver must name the same verse,
// not pull verse 3's references.
func TestCrossrefAndShareAgreeOnDanglingNumber(t *testing.T) {
	st := refrainChapterState()
	sel := refrain + " 3"
	span := selSpanFromNative(2, 3)

	if _, cite, _, _ := prepareShareQuote(st, sel, span); cite != "Psalms 136:2" {
		t.Fatalf("share cite = %q, want Psalms 136:2 (the dangling number introduces nothing)", cite)
	}
	vs := selectionVerses(st, sel, span)
	if len(vs) != 1 || vs[0].Verse != 2 {
		t.Errorf("selectionVerses = %v, want exactly verse 2 — the crossref panel would cite a verse the share refuses to name", vs)
	}

	// A selection that is ONLY the number still resolves to that verse: the
	// normalize declines it, and the span verbatim is the honest reading.
	if vs := selectionVerses(st, "3", selSpanFromNative(3, 3)); len(vs) != 1 || vs[0].Verse != 3 {
		t.Errorf("number-only selection = %v, want verse 3 (span verbatim when the normalize declines)", vs)
	}
}

// A single partial word: the normalize declines it (no whole word to quote) but
// the span still knows the verse — the LINK must carry it, or the message pairs
// a verse-level citation with a chapter-level URL.
func TestShareLinkKeepsSpanVersesWhenNormalizeDeclines(t *testing.T) {
	st := refrainChapterState()
	if _, _, _, _, ok := normalizeShareSelection(st, "lov", selSpanFromNative(2, 2)); ok {
		t.Fatal("precondition: a single partial word should decline to normalize")
	}
	lo, hi := linkVersesForSelection(st, "lov", selSpanFromNative(2, 2))
	if lo != 2 || hi != 2 {
		t.Errorf("link verses = %d-%d, want 2-2 from the span", lo, hi)
	}
	// And without a span it stays an honest chapter link, as ever.
	if lo, hi := linkVersesForSelection(st, "lov", selSpan{}); lo != 0 || hi != 0 {
		t.Errorf("zero span link verses = %d-%d, want 0-0", lo, hi)
	}
}
