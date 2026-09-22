package bibletext

import (
	"strings"
	"testing"
)

// THE SHAPE A REAL SHARE HAS, AND THE ONE THE FIRST TESTS LEFT OUT.
//
// share_small_caps_test.go feeds the drawn verse on its own. A reader's drag
// does not arrive like that: the native side hands over the verse NUMBER as a
// bare token in front of the text ("1 The Lᴏʀᴅ is…"), which is why
// stripVerseMarkers exists at all. The marker strip confirms each token by
// comparing what follows it with the verse body -- and if the two are in
// different forms, the drawn one with small capitals and the stored one with
// the publisher's letters, the comparison fails at the first small capital,
// the token survives, the locate misses, and the share falls off the
// positional path on exactly the verses the divine-name work is for.
//
// Every case here carries the token, or a heading, or a gap mark -- something
// the plain-verse tests do not -- and asserts the same two things: the
// selection locates, and nothing the app invented for its own page goes out
// EXCEPT the divine name's small capitals, which a share keeps as drawn
// (sharedText, outbound_text.go).

func TestAWholeVerseDragWithItsNumberStillLocatesADivineNameVerse(t *testing.T) {
	st := psalm23SmallCapsState()
	v := st.Bible.Verses["Psalms"][23][0]
	raw := "1 " + drawnVerse(t, v)
	if !strings.Contains(raw, "Lᴏʀᴅ") {
		t.Fatalf("premise broken: the pane is not drawing small capitals: %q", raw)
	}

	text, lo, hi, _, ok := normalizeShareSelection(st, raw, selSpan{})
	if !ok {
		t.Fatal("a whole-verse drag that includes the verse number could not be located: " +
			"the marker strip compared drawn text against the publisher's letters, " +
			"the token survived, and the share fell back to the legacy probe path")
	}
	if lo != 1 || hi != 1 {
		t.Errorf("attributed to verses %d-%d, want 1-1", lo, hi)
	}
	if strings.HasPrefix(text, "1 ") {
		t.Errorf("the verse-number token survived into the quote: %q", text)
	}
	if !strings.Contains(text, "Lᴏʀᴅ") || strings.Contains(text, "LORD") {
		t.Errorf("the divine name did not go out as drawn: %q", text)
	}
}

func TestTwoVersesWithBothNumbersCiteTheRange(t *testing.T) {
	st := psalm23SmallCapsState()
	vs := st.Bible.Verses["Psalms"][23]
	raw := "1 " + drawnVerse(t, vs[0]) + " 2 " + vs[1].Text

	text, lo, hi, _, ok := normalizeShareSelection(st, raw, selSpan{})
	if !ok {
		t.Fatal("two numbered verses, the first holding a divine name, could not be located")
	}
	if lo != 1 || hi != 2 {
		t.Errorf("attributed to verses %d-%d, want 1-2", lo, hi)
	}
	for _, tok := range []string{"1 The", " 2 He"} {
		if strings.Contains(text, tok) {
			t.Errorf("a verse-number token survived: %q in %q", tok, text)
		}
	}
	if !strings.Contains(text, "Lᴏʀᴅ") {
		t.Errorf("the divine name did not go out as drawn: %q", text)
	}
}

// The legacy fallback is the path a selection takes when it cannot be located
// at all -- a lone partial word, for instance. It is still a way out of the
// app, so it follows the same rule as the located path: the name as drawn, and
// none of the page's other typography.
func TestADeclinedSelectionFollowsTheSameRuleAsALocatedOne(t *testing.T) {
	st := psalm23SmallCapsState()
	quote, _, at, _ := prepareShareQuote(st, "Lᴏʀ", selSpan{})
	if at >= 0 {
		t.Fatalf("premise broken: a lone partial word was located (at=%d); this test is about the fallback", at)
	}
	if !strings.Contains(quote, "Lᴏʀ") || strings.Contains(quote, "LOR") {
		t.Errorf("the fallback changed the drawn small capitals: %q", quote)
	}
	// And the fallback's typography strip: a declined selection carrying an
	// omitted verse's mark.
	gap, _, gapAt, _ := prepareShareQuote(st, "zz "+verseGapMark(3)+" qq", selSpan{})
	if gapAt >= 0 {
		t.Fatalf("premise broken: the gap-mark selection was located (at=%d), so the fallback was not exercised", gapAt)
	}
	if strings.Contains(gap, "[") {
		t.Errorf("the fallback shipped the page's own gap mark: %q", gap)
	}
}

func TestAHeadingOverADivineNameVerseIsStrippedAndTheVerseLocated(t *testing.T) {
	st := psalm23SmallCapsState()
	st.Bible.Headings = map[string]map[int][]Heading{"Psalms": {23: {
		{Text: "The Lord the Shepherd of His People", Style: "s", BeforeVerse: 1},
	}}}
	v := st.Bible.Verses["Psalms"][23][0]
	raw := "The Lord the Shepherd of His People 1 " + drawnVerse(t, v)

	text, lo, hi, _, ok := normalizeShareSelection(st, raw, selSpan{})
	if !ok {
		t.Fatal("a heading followed by a divine-name verse could not be located on the heading retry")
	}
	if lo != 1 || hi != 1 {
		t.Errorf("attributed to verses %d-%d, want 1-1", lo, hi)
	}
	if strings.Contains(text, "Shepherd of His People") {
		t.Errorf("the heading rode into the quote: %q", text)
	}
	if !strings.Contains(text, "Lᴏʀᴅ") {
		t.Errorf("the divine name did not go out as drawn: %q", text)
	}
}

// A heading and an omitted-verse mark in one drag: the retry has to remove the
// heading AND resolve the mark, or it can neither locate nor cite correctly.
func TestAHeadingAndAGapMarkInOneDragLocateAndCiteOnlyTheVersesQuoted(t *testing.T) {
	bd := &BibleData{
		Books: []string{"Acts"},
		Verses: map[string]map[int][]Verse{"Acts": {10: {
			{BookName: "Acts", Book: "Acts", Chapter: 10, Verse: 43,
				Text: "All the prophets witness about him, that through his name everyone who believes in him will receive remission of sins."},
			{BookName: "Acts", Book: "Acts", Chapter: 10, Verse: 44,
				Text: "While Peter was still speaking these words, the Spirit fell on all those who heard the word."},
			// 45 is omitted by this edition; the page draws "[45]" here.
			{BookName: "Acts", Book: "Acts", Chapter: 10, Verse: 46,
				Text: "For they heard them speaking with tongues and magnifying God."},
		}}},
		Headings: map[string]map[int][]Heading{"Acts": {10: {
			{Text: "The Holy Spirit Falls on the Gentiles", Style: "s", BeforeVerse: 44},
		}}},
	}
	st := &AppState{Bible: bd, CurrentBook: "Acts", CurrentChapter: 10}
	raw := "The Holy Spirit Falls on the Gentiles 44 While Peter was still speaking these words, " +
		"the Spirit fell on all those who heard the word. " + verseGapMark(45) +
		" 46 For they heard them speaking with tongues and magnifying God."

	text, lo, hi, _, ok := normalizeShareSelection(st, raw, selSpan{})
	if !ok {
		t.Fatal("a heading plus a gap mark could not be located: the retry stripped the heading " +
			"but searched the outbound corpus with a selection still carrying the mark")
	}
	if lo != 44 || hi != 46 {
		t.Errorf("attributed to verses %d-%d, want 44-46 (43 contributed no words)", lo, hi)
	}
	if strings.Contains(text, "Falls on the Gentiles") {
		t.Errorf("the heading rode into the quote: %q", text)
	}
	if strings.Contains(text, "[45]") {
		t.Errorf("the page's own gap mark rode into the quote: %q", text)
	}
}
