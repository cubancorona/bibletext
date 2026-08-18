package bibletext

// The positional selection→verse contract (selection_span.go). The fixture is
// the defect's own shape: a refrain repeated verbatim across verses (Psalm 136
// carries one in all 26), which text matching cannot attribute — the probe
// matches every copy, and strings.Index always finds the first. The span is the
// selection's POSITION carried through from the pane, and these tests pin both
// halves of the contract: a valid span decides the verses by itself, and the
// zero span still takes the old matching path — including its misattribution,
// recorded here on purpose as the reason the span exists.

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func refrainChapterState() *AppState {
	bd := &BibleData{
		Books: []string{"Psalms"},
		Verses: map[string]map[int][]Verse{"Psalms": {136: {
			{BookName: "Psalms", Book: "Psalms", Chapter: 136, Verse: 1,
				Text: "Give thanks to the LORD, for he is good; for his loving kindness endures forever."},
			{BookName: "Psalms", Book: "Psalms", Chapter: 136, Verse: 2,
				Text: "Give thanks to the God of gods; for his loving kindness endures forever."},
			{BookName: "Psalms", Book: "Psalms", Chapter: 136, Verse: 3,
				Text: "Give thanks to the Lord of lords; for his loving kindness endures forever."},
		}}},
	}
	return &AppState{Bible: bd, CurrentBook: "Psalms", CurrentChapter: 136}
}

const refrain = "for his loving kindness endures forever."

// A span names its verses positionally — the repeated refrain selected IN VERSE
// 2 cites verse 2, never the identical copy in verse 1.
func TestSelectionSpanRepeatedWordingCitesSelectedVerse(t *testing.T) {
	st := refrainChapterState()
	span := selSpanFromNative(2, 2)

	vs := selectionVerses(st, refrain, span)
	if len(vs) != 1 || vs[0].Verse != 2 {
		t.Fatalf("selectionVerses with span (2,2) = %v, want exactly verse 2", vs)
	}
	if got := citationForSelection(st, refrain, span); got != "Psalms 136:2" {
		t.Errorf("citation = %q, want Psalms 136:2", got)
	}
	// The share pipeline's primary (normalizeShareSelection) path must agree:
	// without the span it located the refrain at verse 1's copy.
	if _, cite, _, _ := prepareShareQuote(st, refrain, span); cite != "Psalms 136:2" {
		t.Errorf("share citation = %q, want Psalms 136:2", cite)
	}
}

// A selection under the matching probe's 8-rune floor resolves when a span
// rides with it — the floor left it citing nothing.
func TestSelectionSpanShortSelectionResolves(t *testing.T) {
	st := refrainChapterState()
	if vs := selectionVerses(st, "gods;", selSpanFromNative(2, 2)); len(vs) != 1 || vs[0].Verse != 2 {
		t.Fatalf("short selection with span = %v, want verse 2", vs)
	}
	// The old floor, pinned: the same selection without a span resolves to
	// nothing, so the citation degrades to chapter-only.
	if vs := selectionVerses(st, "gods;", selSpan{}); vs != nil {
		t.Errorf("short selection without span = %v, want nil (the 8-rune floor)", vs)
	}
	if got := citationForSelection(st, "gods;", selSpan{}); got != "Psalms 136" {
		t.Errorf("floor citation = %q, want the chapter-only fallback", got)
	}
	if got := citationForSelection(st, "gods;", selSpanFromNative(2, 2)); got != "Psalms 136:2" {
		t.Errorf("short-selection citation with span = %q, want Psalms 136:2", got)
	}
}

// A span crossing a verse boundary cites both verses, and an over-long span is
// clamped to the verses that exist.
func TestSelectionSpanCrossingVerseBoundary(t *testing.T) {
	st := refrainChapterState()
	sel := "gods; " + refrain + " 3 Give thanks to the Lord of lords;"
	if got := citationForSelection(st, sel, selSpanFromNative(2, 3)); got != "Psalms 136:2–3" {
		t.Errorf("boundary citation = %q, want Psalms 136:2–3", got)
	}
	if vs := selectionVerses(st, sel, selSpanFromNative(2, 99)); len(vs) != 2 || vs[0].Verse != 2 || vs[1].Verse != 3 {
		t.Errorf("over-long span = %v, want clamped to verses 2–3", vs)
	}
	// lo=0 is "the selection starts above verse 1's number" (chapter heading):
	// it clamps to verse 1 rather than losing the whole span.
	if s := selSpanFromNative(0, 2); s != (selSpan{lo: 1, hi: 2}) {
		t.Errorf("selSpanFromNative(0,2) = %+v, want {1 2}", s)
	}
	if s := selSpanFromNative(0, 0); s.valid() {
		t.Errorf("selSpanFromNative(0,0) must be the invalid zero span")
	}
}

// The zero span takes today's text-matching path — and on this fixture that
// path MISATTRIBUTES, which is pinned deliberately: it is the honest record of
// why the span exists. If matching ever learns to attribute repeated wording
// correctly, this test should fail and the span's fallback story be revisited.
func TestSelectionSpanZeroFallsBackToMatching(t *testing.T) {
	st := refrainChapterState()

	// The probe matches the refrain's copy in EVERY verse, so a one-verse
	// selection comes back as all three.
	vs := selectionVerses(st, refrain, selSpan{})
	if len(vs) != 3 {
		t.Fatalf("matching fallback = %d verses, want all 3 (the misattribution)", len(vs))
	}
	// The share pipeline locates the FIRST copy, so a verse-2 selection is
	// cited as verse 1.
	if _, cite, _, _ := prepareShareQuote(st, refrain, selSpan{}); cite != "Psalms 136:1" {
		t.Errorf("fallback share citation = %q, want the (wrong) Psalms 136:1", cite)
	}
}

// The styled pane resolves the span from its own runs (every run knows its
// verse), so the desktop's Windows/Linux pane feeds the same positional
// contract the native overlays do.
func TestStyledPaneSelectedVerseSpan(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := refrainChapterState()
	p := newStyledReadingPane(st, st.Bible.GetChapter(st.CurrentBook, st.CurrentChapter))
	p.Resize(fyne.NewSize(420, 400))

	if s := p.selectedVerseSpan(); s.valid() {
		t.Fatalf("no selection must yield the zero span; got %+v", s)
	}

	runes := []rune(p.lay.Text)
	// Rune offset of verse 2's refrain: the copy after verse 2's number.
	offOf := func(needle string, from int) int {
		sub := string(runes[from:])
		i := indexRunes(sub, needle)
		if i < 0 {
			t.Fatalf("%q not found in layout text", needle)
		}
		return from + i
	}
	v2num := offOf(superscriptNumber(2), 0)
	v2refrain := offOf(refrain, v2num)

	// The refrain inside verse 2 resolves to verse 2, never verse 1's copy.
	p.setSelection(v2refrain, v2refrain+len([]rune(refrain)))
	if s := p.selectedVerseSpan(); s != (selSpan{lo: 2, hi: 2}) {
		t.Errorf("verse-2 refrain span = %+v, want {2 2}", s)
	}

	// A selection touching only verse 2's NUMBER resolves to verse 2.
	p.setSelection(v2num, v2num+len([]rune(superscriptNumber(2))))
	if s := p.selectedVerseSpan(); s != (selSpan{lo: 2, hi: 2}) {
		t.Errorf("verse-number span = %+v, want {2 2}", s)
	}

	// Crossing the 1→2 boundary carries both.
	p.setSelection(v2num-4, v2refrain+4)
	if s := p.selectedVerseSpan(); s != (selSpan{lo: 1, hi: 2}) {
		t.Errorf("boundary span = %+v, want {1 2}", s)
	}

	// And the span the study menu would dispatch survives the whole pipeline:
	// the pane's positional answer cites verse 2 for verse 2's refrain.
	p.setSelection(v2refrain, v2refrain+len([]rune(refrain)))
	if got := citationForSelection(st, plainSelection(p.selectedRaw()), p.selectedVerseSpan()); got != "Psalms 136:2" {
		t.Errorf("styled-pane citation = %q, want Psalms 136:2", got)
	}
}

// indexRunes is strings.Index in RUNE units, matching the styled pane's model
// offsets.
func indexRunes(haystack, needle string) int {
	hr := []rune(haystack)
	nr := []rune(needle)
	for i := 0; i+len(nr) <= len(hr); i++ {
		if string(hr[i:i+len(nr)]) == needle {
			return i
		}
	}
	return -1
}
