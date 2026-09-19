package bibletext

// A selection that STARTS at a publisher's section heading must be attributed
// to the verse the heading stands above, not to the verse before it.
//
// The reported symptom: selecting a heading and the verses under it, then
// sending a note, highlights the verse ABOVE the heading as well. The chain is
// that a heading sits between the previous verse's text and the next verse's
// number, so the native side -- which resolves an offset to a verse by the last
// verse marker before it -- hands over a span starting at the previous verse.
// normalizeShareSelectionIn is the safety net for that: it locates the
// selection's words in the chapter's prose and re-derives the range from where
// they land. But the corpus it searches is built from verse text alone
// (chapterProseIn), so a selection that OPENS with heading words cannot be
// located at all, the normalize declines, and selectionVersesIn keeps the raw
// span verbatim -- previous verse included.
//
// The control below is the point of the pair: the same verses selected WITHOUT
// the heading attribute correctly. Only the heading makes the difference, which
// is what identifies the cause rather than merely recording the symptom.

import (
	"strings"
	"testing"
)

// A chapter shaped like the report: a heading between two verses, with verses
// either side of it. The wording is invented -- the edition in the report is
// under licence and none of its text belongs in a fixture.
func headingChapterState() *AppState {
	bd := &BibleData{
		Books: []string{"Acts"},
		Verses: map[string]map[int][]Verse{"Acts": {10: {
			{BookName: "Acts", Book: "Acts", Chapter: 10, Verse: 42,
				Text: "He commanded us to preach to the people and to testify."},
			{BookName: "Acts", Book: "Acts", Chapter: 10, Verse: 43,
				Text: "To him all the prophets witness that whoever believes in him will receive remission of sins."},
			{BookName: "Acts", Book: "Acts", Chapter: 10, Verse: 44,
				Text: "While Peter was still speaking these words, the Spirit fell on all those who heard the word."},
			{BookName: "Acts", Book: "Acts", Chapter: 10, Verse: 45,
				Text: "Those of the circumcision who believed were astonished, as many as came with Peter."},
			{BookName: "Acts", Book: "Acts", Chapter: 10, Verse: 46,
				Text: "For they heard them speaking with tongues and magnifying God."},
			{BookName: "Acts", Book: "Acts", Chapter: 10, Verse: 47,
				Text: "Can anyone forbid the water for these to be baptized?"},
		}}},
		Headings: map[string]map[int][]Heading{"Acts": {10: {
			{Text: "The Holy Spirit Falls on the Gentiles", Style: "s", BeforeVerse: 44},
		}}},
	}
	return &AppState{Bible: bd, CurrentBook: "Acts", CurrentChapter: 10}
}

// The verses under the heading, as the native side hands them over: verse
// numbers survive as bare tokens in the selected string (stripVerseMarkers
// exists for exactly that), and the heading's own words lead the selection.
const (
	headingUnderVerses = "44 While Peter was still speaking these words, the Spirit fell on all those who heard the word. " +
		"45 Those of the circumcision who believed were astonished, as many as came with Peter. " +
		"46 For they heard them speaking with tongues and magnifying God. " +
		"47 Can anyone forbid the water for these to be baptized?"
	headingText = "The Holy Spirit Falls on the Gentiles"
)

func TestASelectionLedByAHeadingIsNotAttributedToTheVerseAbove(t *testing.T) {
	st := headingChapterState()

	// The control FIRST. The same verses without the heading, and with the
	// span the native side would report for them, must attribute to 44-47. If
	// this fails, the fixture or the harness is wrong and the case below proves
	// nothing about headings.
	if got := shareNoteReference(st, headingUnderVerses, selSpanFromNative(44, 47)); !strings.Contains(got, "44") || strings.Contains(got, "43") {
		t.Fatalf("control: selecting 44-47 WITHOUT the heading cited %q; the fixture is not sound, so the heading case below says nothing", got)
	}

	// The reported case. The heading leads the selection, and the native span
	// opens at 43 because the heading sits after verse 43's text and before
	// verse 44's number.
	sel := headingText + " " + headingUnderVerses
	got := shareNoteReference(st, sel, selSpanFromNative(43, 47))
	if strings.Contains(got, "43") {
		t.Errorf("a selection opening at the heading cited %q -- it reaches back over verse 43, "+
			"which is above the heading and was never selected. The note's highlight covers it.", got)
	}
	if !strings.Contains(got, "44") {
		t.Errorf("a selection opening at the heading cited %q, which does not name verse 44 -- "+
			"the first verse actually under the heading", got)
	}
}

// The same claim one layer down, at the resolver every verb shares, so a fix
// is pinned where it belongs rather than only at the citation string.
func TestSelectionVersesSkipsTheVerseAboveAHeading(t *testing.T) {
	st := headingChapterState()

	control := selectionVersesIn(st, "Acts", 10, headingUnderVerses, selSpanFromNative(44, 47))
	if len(control) == 0 || control[0].Verse != 44 {
		t.Fatalf("control: selecting 44-47 without the heading resolved to %s; the fixture is not sound",
			verseRunString(control))
	}

	sel := headingText + " " + headingUnderVerses
	got := selectionVersesIn(st, "Acts", 10, sel, selSpanFromNative(43, 47))
	if len(got) == 0 {
		t.Fatal("a selection opening at the heading resolved to no verses at all")
	}
	if got[0].Verse != 44 {
		t.Errorf("a selection opening at the heading resolved to %s, want it to start at 44: "+
			"verse %d is above the heading and was not selected",
			verseRunString(got), got[0].Verse)
	}
	if last := got[len(got)-1].Verse; last != 47 {
		t.Errorf("a selection opening at the heading resolved to %s, want it to end at 47",
			verseRunString(got))
	}
}

// A drag does not have to begin at the heading's first letter. Starting
// part-way along it, or in the middle of one of its words, leaves a fragment at
// the front of the selection that is a byte suffix of the heading rather than a
// word-aligned one — and each of those shapes reached back over verse 43 too.
func TestADragBegunInsideAHeadingIsAlsoAttributedForward(t *testing.T) {
	st := headingChapterState()
	for _, c := range []struct{ name, lead string }{
		{"begun at a word boundary inside the heading", "Falls on the Gentiles"},
		{"begun in the middle of one of its words", "pirit Falls on the Gentiles"},
		{"begun at its last word", "Gentiles"},
	} {
		got := selectionVersesIn(st, "Acts", 10, c.lead+" "+headingUnderVerses, selSpanFromNative(43, 47))
		if len(got) == 0 || got[0].Verse != 44 {
			t.Errorf("%s: resolved to %s, want it to start at 44", c.name, verseRunString(got))
		}
	}
}

// The repair must be reachable ONLY by a selection that cannot be located as it
// stands. These are the shapes that locate perfectly well today, and a heading
// repair that touched any of them would quietly move the reader's note to the
// wrong verse — a far worse defect than the one being fixed, and one the
// existing suite would not have noticed.
func TestTheHeadingRepairNeverTouchesASelectionThatAlreadyResolves(t *testing.T) {
	st := headingChapterState()
	for _, c := range []struct {
		name, sel string
		lo, hi    int
		want      int
	}{
		{"a two-letter fragment that is also heading matter", "th", 45, 45, 45},
		{"a whole word that is also heading matter", "the", 46, 46, 46},
		{"the word the heading ends with", "Gentiles", 45, 45, 45},
		{"an ordinary verse drag", "Those of the circumcision who believed", 45, 45, 45},
		{"a drag from the verse above, through the heading, into the one below",
			"To him all the prophets witness that whoever believes in him will receive remission of sins. " +
				headingText + " 44 While Peter was still speaking these words, the Spirit fell on all those who heard the word.",
			43, 44, 43},
	} {
		got := selectionVersesIn(st, "Acts", 10, c.sel, selSpanFromNative(c.lo, c.hi))
		if len(got) == 0 || got[0].Verse != c.want {
			t.Errorf("%s: resolved to %s, want it to start at %d — the heading repair has reached a selection it must not touch",
				c.name, verseRunString(got), c.want)
		}
	}
}

// A heading is editorial matter, not scripture, so it must never appear in the
// text that gets shared. A drag that legitimately quotes the verse above and
// reaches down into the heading used to carry the heading's words into the
// quotation — the verse range was right, so nothing that looked at verse
// numbers alone could see it.
func TestAHeadingIsNeverQuotedAsScripture(t *testing.T) {
	st := headingChapterState()
	for _, c := range []struct {
		name, sel string
		lo, hi    int
	}{
		{"a drag from the verse above that reaches into the heading",
			"To him all the prophets witness that whoever believes in him will receive remission of sins. " + headingText,
			43, 43},
		{"a drag from the verse above, through the heading, into the one below",
			"To him all the prophets witness that whoever believes in him will receive remission of sins. " +
				headingText + " 44 While Peter was still speaking these words, the Spirit fell on all those who heard the word.",
			43, 44},
		{"a drag that opens at the heading",
			headingText + " " + headingUnderVerses, 43, 47},
	} {
		quote, _, _, _ := prepareShareQuote(st, c.sel, selSpanFromNative(c.lo, c.hi))
		if strings.Contains(quote, "Falls on the Gentiles") {
			t.Errorf("%s: the shared quotation carries the publisher's heading:\n  %q", c.name, quote)
		}
	}
}

// A heading at the very top of a chapter has no verse above it to reach back
// to, so it must not shift the range forward either.
func TestAHeadingAtTheTopOfAChapterLeavesTheFirstVerseAlone(t *testing.T) {
	st := headingChapterState()
	st.Bible.Headings["Acts"][10] = []Heading{
		{Text: "Peter Speaks", Style: "s", BeforeVerse: 42},
		{Text: headingText, Style: "s", BeforeVerse: 44},
	}
	sel := "Peter Speaks 42 He commanded us to preach to the people and to testify."
	got := selectionVersesIn(st, "Acts", 10, sel, selSpanFromNative(42, 42))
	if len(got) != 1 || got[0].Verse != 42 {
		t.Errorf("a chapter-opening heading plus its first verse resolved to %s, want just 42",
			verseRunString(got))
	}
}

// The hazard the confirmation rule exists for. Psalm 23's heading is the same
// words as the opening of the verse it stands above, so a strip that matched on
// the heading's wording alone would delete the verse's own opening and leave
// the selection unlocatable — trading one misattribution for another.
func sameWordedHeadingState() *AppState {
	bd := &BibleData{
		Books: []string{"Psalms"},
		Verses: map[string]map[int][]Verse{"Psalms": {23: {
			{BookName: "Psalms", Book: "Psalms", Chapter: 23, Verse: 1,
				Text: "The LORD is my shepherd; I shall not want."},
			{BookName: "Psalms", Book: "Psalms", Chapter: 23, Verse: 2,
				Text: "He makes me to lie down in green pastures."},
		}}},
		Headings: map[string]map[int][]Heading{"Psalms": {23: {
			{Text: "The LORD Is My Shepherd", Style: "s", BeforeVerse: 1},
		}}},
	}
	return &AppState{Bible: bd, CurrentBook: "Psalms", CurrentChapter: 23}
}

func TestAHeadingWhoseWordsAreAlsoScriptureDoesNotEatTheVerse(t *testing.T) {
	st := sameWordedHeadingState()

	// The verse alone. Its opening matches the heading's wording, but no verse
	// number follows that wording here, so nothing may be removed.
	sel := "The LORD is my shepherd; I shall not want."
	if got := stripHeadings(st, "Psalms", 23, sel); got != sel {
		t.Errorf("stripHeadings ate the verse's own opening:\n  in:  %q\n  out: %q", sel, got)
	}
	vs := selectionVersesIn(st, "Psalms", 23, sel, selSpanFromNative(1, 1))
	if len(vs) != 1 || vs[0].Verse != 1 {
		t.Errorf("the verse under a same-worded heading resolved to %s, want just 1", verseRunString(vs))
	}

	// The heading AND the verse. Here the heading is followed by verse 1's
	// number, so it is confirmed and removed — and what is left still resolves.
	led := "The LORD Is My Shepherd 1 The LORD is my shepherd; I shall not want."
	vs = selectionVersesIn(st, "Psalms", 23, led, selSpanFromNative(1, 1))
	if len(vs) != 1 || vs[0].Verse != 1 {
		t.Errorf("heading plus its verse resolved to %s, want just 1", verseRunString(vs))
	}
}

// The confirmation is the number of the verse the heading stands above. A
// heading followed by some other number is not the rendered shape this removes,
// and a number that merely starts with it is not a match at all.
func TestAHeadingIsOnlyStrippedWhereItsOwnVerseNumberFollows(t *testing.T) {
	st := headingChapterState()

	// BeforeVerse is 44; a different number must leave the heading in place.
	wrong := headingText + " 46 For they heard them speaking with tongues and magnifying God."
	if got := stripHeadings(st, "Acts", 10, wrong); got != wrong {
		t.Errorf("stripped a heading that was not followed by its own verse number:\n  in:  %q\n  out: %q", wrong, got)
	}

	// "44" must not be matched by a longer number that opens with it.
	st.Bible.Headings["Acts"][10] = []Heading{{Text: headingText, Style: "s", BeforeVerse: 4}}
	runOn := headingText + " 44 While Peter was still speaking these words."
	if got := stripHeadings(st, "Acts", 10, runOn); got != runOn {
		t.Errorf("matched verse 4 inside the number 44:\n  in:  %q\n  out: %q", runOn, got)
	}
}

func verseRunString(vs []Verse) string {
	if len(vs) == 0 {
		return "no verses"
	}
	var b strings.Builder
	for i, v := range vs {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(itoaVerse(v.Verse))
	}
	return b.String()
}

func itoaVerse(n int) string {
	if n == 0 {
		return "0"
	}
	var d [8]byte
	i := len(d)
	for n > 0 {
		i--
		d[i] = byte('0' + n%10)
		n /= 10
	}
	return string(d[i:])
}

// A heading selected on its OWN belongs to the verse it stands above.
//
// The other drag shapes carry verse words, so the repair can locate them and
// the range falls out of where they land. A heading alone carries none: there
// is nothing to locate, the normalize declines, and the raw span the native
// side reported survives — and that span names the verse ABOVE the heading,
// because a heading sits after that verse's text and before the next verse's
// number.
//
// The answer is the verse the heading introduces, which the model already
// records as Heading.BeforeVerse. It is the only answer that survives leaving
// this device: a note anchored to a heading would not place for a reader on a
// different translation, and shared links open the public-domain web reader,
// whose headings are not the NKJV's.
func TestAHeadingSelectedOnItsOwnBelongsToTheVerseBelowIt(t *testing.T) {
	st := headingChapterState()

	for _, sel := range []string{
		headingText,                      // the whole heading
		"Falls on the Gentiles",          // a drag that began part-way along it
		"pirit Falls on the Gentiles",    // and one that began mid-word
	} {
		got := selectionVersesIn(st, "Acts", 10, sel, selSpanFromNative(43, 43))
		if len(got) != 1 || got[0].Verse != 44 {
			t.Errorf("selecting %q alone resolved to %s, want just 44 — the verse the heading stands above",
				sel, verseRunString(got))
		}
	}
}

// And it must not put the heading's words in the quotation. The reader selected
// editorial matter, not scripture; quoting the verse instead would attribute
// words they did not choose, and quoting the heading breaks the rule that a
// heading never reaches sharing.
func TestAHeadingSelectedOnItsOwnQuotesNothing(t *testing.T) {
	st := headingChapterState()
	quote, cite, _, _ := prepareShareQuote(st, headingText, selSpanFromNative(43, 43))
	if strings.Contains(quote, "Falls on the Gentiles") {
		t.Errorf("a heading selected alone was quoted as scripture: %q", quote)
	}
	if !strings.Contains(cite, "44") {
		t.Errorf("a heading selected alone cited %q, want it to name verse 44", cite)
	}
}

// The heading answer must require that the selection IS heading text. The span
// alone is not enough: a selection that simply fails to locate, reported
// against the verse above a heading, would otherwise be handed to the heading's
// verse even though it has nothing to do with the heading.
func TestAnUnlocatableSelectionThatIsNotHeadingTextStaysWhereTheSpanPutsIt(t *testing.T) {
	st := headingChapterState()
	for _, sel := range []string{
		"zzz qqq",                 // nothing in the chapter or the heading
		"words that appear nowhere in this fixture at all",
	} {
		got := selectionVersesIn(st, "Acts", 10, sel, selSpanFromNative(43, 43))
		if len(got) != 1 || got[0].Verse != 43 {
			t.Errorf("an unlocatable non-heading selection %q resolved to %s, want it left at 43 "+
				"where the span put it — only HEADING text may be moved to the verse below",
				sel, verseRunString(got))
		}
	}
}
