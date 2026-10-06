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
		headingText,                   // the whole heading
		"Falls on the Gentiles",       // a drag that began part-way along it
		"pirit Falls on the Gentiles", // and one that began mid-word
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
		"zzz qqq", // nothing in the chapter or the heading
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

// A heading at the TOP of a chapter, selected on its own, quotes nothing
// either. It has no verse above it, so the span test the rule above relies on
// has nothing to name: the native panes report a drag inside it as no span at
// all (the last verse number before it is none), or as the first verse when
// the drag reaches that verse's number (selSpanFromNative's clamp). Each of
// those shapes, and the heading followed by the number of the verse it stands
// above (a drag that ran one word too far), cites the verse beneath and quotes
// nothing. Most chapters of the licensed editions open with a heading, so this
// is the common case, not an edge.
func openingHeadingState() *AppState {
	bd := &BibleData{
		Books: []string{"Genesis"},
		Verses: map[string]map[int][]Verse{"Genesis": {1: {
			{BookName: "Genesis", Book: "Genesis", Chapter: 1, Verse: 1,
				Text: "In the beginning the heavens and the earth were made."},
			{BookName: "Genesis", Book: "Genesis", Chapter: 1, Verse: 2,
				Text: "The earth was without form, and the deep was dark."},
		}}},
		Headings: map[string]map[int][]Heading{"Genesis": {1: {
			{Text: "The Making of All Things", Style: "s", BeforeVerse: 1},
		}}},
	}
	return &AppState{Bible: bd, CurrentBook: "Genesis", CurrentChapter: 1}
}

func TestAHeadingAtTheTopOfAChapterSelectedOnItsOwnQuotesNothing(t *testing.T) {
	st := openingHeadingState()

	// The control: the first verse's own words, with the span a drag in them
	// gets, are quoted and cited as ever.
	if quote, cite, _, _ := prepareShareQuote(st, "In the beginning the heavens", selSpanFromNative(1, 1)); !strings.HasPrefix(quote, "In the beginning") || cite != "Genesis 1:1" {
		t.Fatalf("control: the first verse's words shared as (%q, %q)", quote, cite)
	}

	for _, c := range []struct {
		name, sel string
		span      selSpan
	}{
		{"the whole heading, no span", "The Making of All Things", selSpan{}},
		{"a drag begun part-way along it", "Making of All Things", selSpan{}},
		{"reported as the first verse", "The Making of All Things", selSpanFromNative(0, 1)},
		{"run on to the first verse's number", "The Making of All Things 1", selSpanFromNative(0, 1)},
	} {
		quote, cite, _, _ := prepareShareQuote(st, c.sel, c.span)
		if quote != "" {
			t.Errorf("%s: the heading was quoted as scripture: %q", c.name, quote)
		}
		if cite != "Genesis 1:1" {
			t.Errorf("%s: cited %q, want Genesis 1:1, the verse the heading stands above", c.name, cite)
		}
	}

	// The same rule in the middle of a chapter: the heading and the number of
	// the verse beneath it, reported against the verse above.
	mid := headingChapterState()
	quote, cite, _, _ := prepareShareQuote(mid, headingText+" 44", selSpanFromNative(43, 44))
	if quote != "" || cite != "Acts 10:44" {
		t.Errorf("a heading run on to its verse's number shared as (%q, %q), want (\"\", Acts 10:44)", quote, cite)
	}

	// A span that names a verse further down is not a drag inside the top
	// heading, whatever the words: the selection stays where the span puts it.
	if _, cite, _, _ := prepareShareQuote(st, "Making of All", selSpanFromNative(2, 2)); cite != "Genesis 1:2" {
		t.Errorf("an unlocatable selection reported against verse 2 cited %q; only a selection the top heading "+
			"could have produced may be moved to the verse beneath it", cite)
	}
}

// A verse number selected on its own is the page's apparatus, as a heading is:
// a double-tap or long-press on the superscript selects it as a word. It
// quotes nothing, and the reference names the verse the number labels, with
// the span the pane reports and without one, as ordinary digits (the Apple and
// Android panes) or as the superscript characters the styled pane draws. The
// link and the verses every other verb resolves agree with it. Before this the
// digits went out as the quote, and showed on the note sheet as the selected
// words.
func TestAVerseNumberSelectedOnItsOwnQuotesNothing(t *testing.T) {
	st := psalm23NoteState()

	// The control: the number with its verse's first words is the verse.
	if quote, cite, _, _ := prepareShareQuote(st, "2 He maketh me", selSpanFromNative(2, 2)); !strings.HasPrefix(quote, "He maketh me") || cite != "Psalms 23:2" {
		t.Fatalf("control: a number with its verse's words shared as (%q, %q)", quote, cite)
	}

	for _, sel := range []string{"2", "²"} {
		for _, span := range []selSpan{selSpanFromNative(2, 2), {}} {
			quote, cite, _, _ := prepareShareQuote(st, sel, span)
			if quote != "" {
				t.Errorf("%q, span %+v: the verse number was quoted as scripture: %q", sel, span, quote)
			}
			if cite != "Psalms 23:2" {
				t.Errorf("%q, span %+v: cited %q, want Psalms 23:2, the verse the number labels", sel, span, cite)
			}
			if lo, hi := linkVersesForSelection(st, sel, span); lo != 2 || hi != 2 {
				t.Errorf("%q, span %+v: the link names verses %d-%d, want 2-2", sel, span, lo, hi)
			}
		}
	}
	if vs := selectionVersesIn(st, "Psalms", 23, "2", selSpanFromNative(2, 2)); len(vs) != 1 || vs[0].Verse != 2 {
		t.Errorf("the verse number resolved to %s, want just 2", verseRunString(vs))
	}
}

// A psalm's title is editorial matter, as a heading is: the publisher's line
// above the psalm, not a verse of it. The Apple and Android panes draw it in
// the same selectable text as the verses, above verse 1 and any heading
// there (the Windows and Linux pane draws it outside the text, where no drag
// reaches it), but their app verbs start at verse 1's number: a selection
// inside the title gets none, and one that runs on into the psalm is cut at
// that number before Go sees it. The page that does hand the title over is
// the Mac's plain-text fallback, where nothing has a span, and these tests
// stand for it and for any caller that hands over raw text. The text is the
// World English Bible's Psalm 3; the heading under the title is invented, as
// that edition sets none, and the small capitals on the title are the
// fixture's own, so the title is matched as the page draws it.
const (
	psalmTitle   = "A Psalm by David, when he fled from Absalom his son."
	psalmHeading = "A Morning Prayer"
)

func psalmTitleState(withHeading bool) *AppState {
	bd := &BibleData{
		Books: []string{"Psalms"},
		Verses: map[string]map[int][]Verse{"Psalms": {3: {
			{BookName: "Psalms", Book: "Psalms", Chapter: 3, Verse: 1,
				Text: "Yahweh, how my adversaries have increased!\nMany are those who rise up against me."},
			{BookName: "Psalms", Book: "Psalms", Chapter: 3, Verse: 2,
				Text: "Many there are who say of my soul,\n“There is no help for him in God.” Selah."},
			{BookName: "Psalms", Book: "Psalms", Chapter: 3, Verse: 3,
				Text: "But you, Yahweh, are a shield around me,\nmy glory, and the one who lifts up my head."},
		}}},
		Superscriptions: map[string]map[int]Superscription{"Psalms": {3: {
			Text: psalmTitle, SmallCaps: []TextSpan{{Start: 2, End: 7}}}}},
	}
	if withHeading {
		bd.Headings = map[string]map[int][]Heading{"Psalms": {3: {
			{Text: psalmHeading, Style: "s", BeforeVerse: 1}}}}
	}
	return &AppState{Bible: bd, CurrentBook: "Psalms", CurrentChapter: 3}
}

// A drag from the title into the psalm shares the psalm's words and cites
// them, and the title's words never ride along as if they were verse 1. The
// control is the same verse words dragged on their own.
func TestAPsalmTitleIsNeverQuotedAsScripture(t *testing.T) {
	for _, withHeading := range []bool{false, true} {
		st := psalmTitleState(withHeading)
		title := collapseSpaces(st.Bible.SuperscriptionFor("Psalms", 3).DrawnText())
		under := ""
		if withHeading {
			under = " " + psalmHeading
		}
		const words = "Yahweh, how my adversaries have increased!"

		wantQuote, wantCite, _, _ := prepareShareQuote(st, "1 "+words, selSpanFromNative(1, 1))
		if !strings.HasPrefix(wantQuote, "Yahweh") || wantCite != "Psalms 3:1" {
			t.Fatalf("control: verse 1's words on their own shared as (%q, %q)", wantQuote, wantCite)
		}
		for _, sel := range []string{
			title + under + " 1 " + words, // the whole title, then the verse
			"his son." + under + " 1 " + words,
			"on." + under + " 1 " + words, // begun mid-word
		} {
			quote, cite, _, _ := prepareShareQuote(st, sel, selSpanFromNative(0, 1))
			if quote != wantQuote || cite != wantCite {
				t.Errorf("heading=%v: %q shared (%q, %q), want the verse's words alone (%q, %q)",
					withHeading, sel, quote, cite, wantQuote, wantCite)
			}
		}
		// Through verse 1 into verse 2: the citation takes both.
		sel := "his son." + under + " 1 Yahweh, how my adversaries have increased! Many are those who rise up against me. 2 Many there are"
		if quote, cite, _, _ := prepareShareQuote(st, sel, selSpanFromNative(0, 2)); strings.Contains(quote, "Absalom") || cite != "Psalms 3:1–2" {
			t.Errorf("heading=%v: a drag through verse 1 into 2 shared (%q, %q)", withHeading, quote, cite)
		}
	}
}

// A title selected on its own cites verse 1 with no quotation, as a heading
// does. No pane with verse geometry offers the verbs on it (see
// psalmTitle); a page that hands it over reports it with no span at all, or,
// were it to reach verse 1's number with one, as verse 1 (selSpanFromNative's
// clamp). The verbs agree: the citation, the link's verses, which a note is
// kept against, and, given a span, the verses the cross-references resolve
// to.
func TestAPsalmTitleSelectedOnItsOwnCitesTheFirstVerse(t *testing.T) {
	for _, withHeading := range []bool{false, true} {
		st := psalmTitleState(withHeading)
		title := collapseSpaces(st.Bible.SuperscriptionFor("Psalms", 3).DrawnText())
		sels := []struct {
			name, sel string
			span      selSpan
		}{
			{"the whole title", title, selSpan{}},
			{"a piece inside it", "by David, when he", selSpan{}},
			{"its end, begun mid-word", "rom Absalom his son.", selSpan{}},
			{"run on to verse 1's number", title + " 1", selSpanFromNative(0, 1)},
			{"run on into part of verse 1's first word", "his son. 1 Yah", selSpanFromNative(0, 1)},
		}
		if withHeading {
			sels = append(sels, []struct {
				name, sel string
				span      selSpan
			}{
				{"the title and the heading", title + " " + psalmHeading, selSpan{}},
				{"the end of the title and part of the heading", "his son. A Morn", selSpan{}},
				{"the title and heading run on to verse 1's number", title + " " + psalmHeading + " 1", selSpanFromNative(0, 1)},
				{"and into part of verse 1's first word", "his son. " + psalmHeading + " 1 Ya", selSpanFromNative(0, 1)},
			}...)
		}
		for _, c := range sels {
			quote, cite, _, _ := prepareShareQuote(st, c.sel, c.span)
			if quote != "" || cite != "Psalms 3:1" {
				t.Errorf("heading=%v, %s: shared (%q, %q), want (\"\", Psalms 3:1)", withHeading, c.name, quote, cite)
			}
			if lo, hi := linkVersesForSelection(st, c.sel, c.span); lo != 1 || hi != 1 {
				t.Errorf("heading=%v, %s: the link names verses %d–%d, want 1–1", withHeading, c.name, lo, hi)
			}
			// With no span the resolver matches words, the legacy pane's
			// path, as it does for a heading at the top of a chapter.
			if !c.span.valid() {
				continue
			}
			if vs := selectionVersesIn(st, "Psalms", 3, c.sel, c.span); len(vs) != 1 || vs[0].Verse != 1 {
				t.Errorf("heading=%v, %s: resolved to %s, want just 1", withHeading, c.name, verseRunString(vs))
			}
		}
	}

	// A span that names a verse further down is not a drag inside the title,
	// whatever the words: the selection stays where the span puts it.
	st := psalmTitleState(false)
	if _, cite, _, _ := prepareShareQuote(st, "by David, when he", selSpanFromNative(2, 2)); cite != "Psalms 3:2" {
		t.Errorf("title words reported against verse 2 cited %q; only a drag the title could have produced is moved to verse 1", cite)
	}
}

// A heading run on through the number of the verse it stands above into part
// of that verse's first word holds no word of scripture: the piece of a word
// cannot be quoted, so the drag is the heading's alone and is answered as
// one, as a psalm's title is in the same shape. It quoted the heading and
// cited the verse above with the one below.
func TestAHeadingRunOnIntoPartOfAWordQuotesNothing(t *testing.T) {
	st := headingChapterState()

	// The control: run on through the whole first word, it is that verse's.
	if quote, cite, _, _ := prepareShareQuote(st, headingText+" 44 While", selSpanFromNative(43, 44)); quote != "While" || cite != "Acts 10:44" {
		t.Fatalf("control: the heading and verse 44's first word shared (%q, %q)", quote, cite)
	}
	for _, sel := range []string{headingText + " 44 Wh", "Gentiles 44 W"} {
		quote, cite, _, _ := prepareShareQuote(st, sel, selSpanFromNative(43, 44))
		if quote != "" || cite != "Acts 10:44" {
			t.Errorf("%q shared (%q, %q), want (\"\", Acts 10:44)", sel, quote, cite)
		}
	}
}

// A drag from the verse above a heading, through the heading, that stops on
// the number of the verse below holds no word of that verse. The first pass
// drops such a trailing number, and the heading repair now does too: the
// shared words are the verse above's, cited by it alone. It shared the
// heading as Scripture with the number after it, cited by both verses. The
// control is the same drag without the heading, which the first pass
// answers.
func TestAHeadingRepairDropsTheNumberTheDragStoppedOn(t *testing.T) {
	st := headingChapterState()
	const above = "whoever believes in him will receive remission of sins."

	if quote, cite, _, _ := prepareShareQuote(st, above+" 44", selSpanFromNative(43, 44)); quote != above || cite != "Acts 10:43" {
		t.Fatalf("control: verse 43's words and verse 44's number shared (%q, %q)", quote, cite)
	}
	quote, cite, _, _ := prepareShareQuote(st, above+" "+headingText+" 44", selSpanFromNative(43, 44))
	if quote != above || cite != "Acts 10:43" {
		t.Errorf("through the heading to verse 44's number: shared (%q, %q), want (%q, Acts 10:43)", quote, cite, above)
	}
}

// A heading and the number of the verse it stands above are the page's
// apparatus, with no word of Scripture in them. With the heading taken off,
// the number alone is answered as the heading is, never looked for among the
// verses' words, where the same figure can stand as a word of the text: here
// verse 1's "3", which the number was found as, quoted and cited.
func TestAHeadingAndItsVerseNumberAreNotLookedForInTheText(t *testing.T) {
	bd := &BibleData{
		Books: []string{"Numbers"},
		Verses: map[string]map[int][]Verse{"Numbers": {2: {
			{BookName: "Numbers", Book: "Numbers", Chapter: 2, Verse: 1,
				Text: "These are the families, 3 in all."},
			{BookName: "Numbers", Book: "Numbers", Chapter: 2, Verse: 2,
				Text: "And they camped by their standards."},
			{BookName: "Numbers", Book: "Numbers", Chapter: 2, Verse: 3,
				Text: "The Levites were not counted among them."},
		}}},
		Headings: map[string]map[int][]Heading{"Numbers": {2: {
			{Text: "The Levites Are Set Apart", Style: "s", BeforeVerse: 3},
		}}},
	}
	st := &AppState{Bible: bd, CurrentBook: "Numbers", CurrentChapter: 2}

	// The control: the heading alone is verse 3's, with nothing quoted.
	if quote, cite, _, _ := prepareShareQuote(st, "The Levites Are Set Apart", selSpanFromNative(2, 2)); quote != "" || cite != "Numbers 2:3" {
		t.Fatalf("control: the heading alone shared (%q, %q)", quote, cite)
	}
	for _, sel := range []string{"The Levites Are Set Apart 3", "Set Apart 3"} {
		quote, cite, _, _ := prepareShareQuote(st, sel, selSpanFromNative(2, 3))
		if quote != "" || cite != "Numbers 2:3" {
			t.Errorf("%q, span 2–3: shared (%q, %q), want (\"\", Numbers 2:3)", sel, quote, cite)
		}
	}
}
