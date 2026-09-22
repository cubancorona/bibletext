package bibletext

import (
	"strings"
	"testing"
)

// SHARING A VERSE WHOSE DIVINE NAME THE EDITION SETS IN SMALL CAPITALS.
//
// The pane draws "Lᴏʀᴅ" — the app's own Unicode small capitals, chosen for the
// page (small_caps_draw.go) — where the publisher stored "Lord". Two things
// have to hold on the way out, and they are separate:
//
//   - WHAT A SHARE SENDS is what the page shows: the small capitals, as drawn.
//     That is the account holder's choice (sharedText, outbound_text.go); it
//     replaced sending the capitals, "LORD", which 1.2.13 did.
//
//   - The share pipeline LOCATES the selection among the verses. A selection
//     carrying small capitals cannot be found in text built from the stored
//     "Lord", so the verses are drawn the same way before they are searched;
//     when they were not, normalizeShareSelection returned ok=false and the
//     share silently dropped to the legacy probe path — losing the positional
//     citation on exactly the verses the divine-name work exists for.
//
// Both halves are asserted here because either could regress without the other.
func psalm23SmallCapsState() *AppState {
	bd := &BibleData{
		Books: []string{"Psalms"},
		Verses: map[string]map[int][]Verse{"Psalms": {23: {
			{BookName: "Psalms", Book: "Psalms", Chapter: 23, Verse: 1,
				Text:      "The Lord is my shepherd; I shall not want.",
				SmallCaps: []TextSpan{{Start: 4, End: 8}}},
			{BookName: "Psalms", Book: "Psalms", Chapter: 23, Verse: 2,
				Text: "He makes me to lie down in green pastures."},
		}}},
	}
	return &AppState{Bible: bd, CurrentBook: "Psalms", CurrentChapter: 23}
}

// drawnVerse is what the reading pane actually puts on screen, and therefore
// what a selection hands the share path.
func drawnVerse(t *testing.T, v Verse) string {
	t.Helper()
	runs := applySmallCaps(v, []verseRun{{Text: v.Text}})
	if len(runs) == 0 {
		t.Fatal("applySmallCaps returned no runs")
	}
	var b strings.Builder
	for _, r := range runs {
		b.WriteString(r.Text)
	}
	return b.String()
}

func TestSharingADivineNameVerseSendsTheNameAsDrawn(t *testing.T) {
	st := psalm23SmallCapsState()
	v := st.Bible.Verses["Psalms"][23][0]

	drawn := drawnVerse(t, v)
	// The premise: the pane really is drawing the app's own characters. If this
	// ever stops being true the rest of the test proves nothing.
	if !strings.Contains(drawn, "Lᴏʀᴅ") {
		t.Fatalf("the pane is not drawing small capitals any more: %q", drawn)
	}
	if strings.Contains(drawn, "LORD") {
		t.Fatalf("drawn text already carries the publisher's letters: %q", drawn)
	}

	// HALF ONE: the selection must locate itself in the publisher's prose.
	text, lo, hi, _, ok := normalizeShareSelection(st, drawn, selSpan{})
	if !ok {
		t.Fatal("normalizeShareSelection could not locate a selection carrying small capitals, " +
			"so the share falls back to the legacy probe path and loses its positional citation")
	}
	if lo != 1 || hi != 1 {
		t.Errorf("attributed to verses %d-%d, want 1-1", lo, hi)
	}

	// HALF TWO: what actually goes out is the name as the page drew it.
	if !strings.Contains(text, "Lᴏʀᴅ") {
		t.Errorf("the normalized share text does not carry the small capitals as drawn: %q", text)
	}
	if strings.Contains(text, "LORD") || strings.Contains(text, "Lord") {
		t.Errorf("the normalized share text carries the name in another spelling: %q", text)
	}

	// And through the caller a share actually uses.
	quote, cite, _, _ := prepareShareQuote(st, drawn, selSpan{})
	if !strings.Contains(quote, "Lᴏʀᴅ") {
		t.Errorf("the shared quote does not carry the small capitals as drawn: %q", quote)
	}
	if strings.Contains(quote, "LORD") || strings.Contains(quote, "Lord") {
		t.Errorf("the shared quote carries the name in another spelling: %q", quote)
	}
	if cite != "Psalms 23:1" {
		t.Errorf("citation = %q, want %q — the positional path was not used", cite, "Psalms 23:1")
	}
}

// A selection with no small capitals in it must be unaffected: the fix must not
// change what every other share already produces.
func TestSharingAnOrdinaryVerseIsUnchanged(t *testing.T) {
	st := psalm23SmallCapsState()
	raw := "He makes me to lie down in green pastures."

	text, lo, hi, _, ok := normalizeShareSelection(st, raw, selSpan{})
	if !ok {
		t.Fatal("an ordinary selection stopped locating")
	}
	if lo != 2 || hi != 2 {
		t.Errorf("attributed to verses %d-%d, want 2-2", lo, hi)
	}
	if text != raw {
		t.Errorf("an ordinary selection was altered:\n got  %q\n want %q", text, raw)
	}
}

// THE SAME CAUSE, A SECOND FEATURE: an omitted verse's mark.
//
// Where a translation omits a verse the page draws the number in brackets —
// "[36]" in the hole Luke 17:36 leaves (verse_gaps.go). That mark is the app's
// own character too, and outboundText strips it by shape. The share path never
// reached outboundText, so a selection dragged across the hole carried "[36]"
// into the message AND could not be found in chapterProse, which is built from
// the publisher's verses and has no such token.
func TestSharingAcrossAnOmittedVerseDropsTheGapMark(t *testing.T) {
	bd := &BibleData{
		Books: []string{"Luke"},
		Verses: map[string]map[int][]Verse{"Luke": {17: {
			{BookName: "Luke", Book: "Luke", Chapter: 17, Verse: 35,
				Text: "There will be two grinding grain together. One will be taken and the other left."},
			// 36 is omitted by this translation; the page draws "[36]" here.
			{BookName: "Luke", Book: "Luke", Chapter: 17, Verse: 37,
				Text: "They answering, asked him, Where, Lord?"},
		}}},
	}
	st := &AppState{Bible: bd, CurrentBook: "Luke", CurrentChapter: 17}

	// What a reader drags across the hole.
	raw := "and the other left. " + verseGapMark(36) + " They answering, asked him"
	if !strings.Contains(raw, "[36]") {
		t.Fatalf("the gap mark is not what this test assumes: %q", raw)
	}

	text, lo, hi, _, ok := normalizeShareSelection(st, raw, selSpan{})
	if !ok {
		t.Fatal("a selection dragged across an omitted verse could not be located, " +
			"so the share drops to the legacy probe path")
	}
	if lo != 35 || hi != 37 {
		t.Errorf("attributed to verses %d-%d, want 35-37", lo, hi)
	}
	if strings.Contains(text, "[36]") {
		t.Errorf("the shared text carries the page's own omitted-verse mark: %q", text)
	}
}

// THE PASSAGE ROUTE, which builds its own "selection" rather than taking one
// from the page — the verse-of-the-day card is the caller.
//
// It synthesises the selection from the stored Verse.Text, where the name is
// "Lord". The corpus it is matched against is in the drawn, shared form, so the
// two sides have to be put in that form by the same route or a passage holding
// a divine name stops locating and the card loses its citation — and what the
// card shares must be the name as drawn, like every other share.
func TestSharingAPassageWithADivineNameLocatesAndCites(t *testing.T) {
	st := psalm23SmallCapsState()

	quote, cite, ok := shareQuoteForPassage(st, "Psalms", 23, 1, 1)
	if !ok {
		t.Fatal("shareQuoteForPassage returned nothing for a verse holding a divine name")
	}
	if cite != "Psalms 23:1" {
		t.Errorf("citation = %q, want %q", cite, "Psalms 23:1")
	}
	if !strings.Contains(quote, "Lᴏʀᴅ") {
		t.Errorf("the passage quote does not carry the small capitals as drawn: %q", quote)
	}
	if strings.Contains(quote, "Lord") || strings.Contains(quote, "LORD") {
		t.Errorf("the passage quote carries the name in another spelling: %q", quote)
	}
}

// THE POEM BREAK MUST SURVIVE THE OUTBOUND FORM.
//
// chapterProse is built in the outbound form so a selection carrying small
// capitals can be located in it. chapterShareStructure — the parallel corpus
// that carries the authored line breaks — was left on the publisher's raw
// text, so for any verse with a SmallCaps span the two corpora stopped
// agreeing character for character. restoreShareLineBreaks locates the quote
// in the structure corpus to know where to put the breaks back, and when that
// locate fails it silently returns the text unbroken.
//
// The effect is a psalm shared as one running line. It is cosmetic rather than
// an attribution error, and it lands where it is most visible: SmallCaps marks
// the divine name, which is densest in the Psalms, which are the verses whose
// line breaks matter most.
func TestAPoemBreakSurvivesAVerseWithADivineName(t *testing.T) {
	poem := "The Lord is my shepherd;\nI shall lack nothing."
	build := func(withCaps bool) *AppState {
		v := Verse{BookName: "Psalms", Book: "Psalms", Chapter: 23, Verse: 1, Text: poem}
		if withCaps {
			v.SmallCaps = []TextSpan{{Start: 4, End: 8}}
		}
		return &AppState{
			Bible: &BibleData{
				Books:  []string{"Psalms"},
				Verses: map[string]map[int][]Verse{"Psalms": {23: {v}}},
			},
			CurrentBook: "Psalms", CurrentChapter: 23,
		}
	}

	// The control: the identical verse without the divine-name span keeps its
	// break today. If this half ever fails the test is measuring the wrong thing.
	plain, _, ok := shareQuoteForPassage(build(false), "Psalms", 23, 1, 1)
	if !ok {
		t.Fatal("the control passage did not share at all")
	}
	if !strings.Contains(plain, "\n") {
		t.Fatalf("the control lost its line break, so this test cannot detect the defect: %q", plain)
	}

	withCaps, _, ok := shareQuoteForPassage(build(true), "Psalms", 23, 1, 1)
	if !ok {
		t.Fatal("the divine-name passage did not share at all")
	}
	if !strings.Contains(withCaps, "\n") {
		t.Errorf("the authored line break was dropped on a verse carrying a divine name:\n"+
			" got  %q\n want a break, as the control has: %q", withCaps, plain)
	}
}
