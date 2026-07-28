package bibletext

// The share pipeline's handling of ragged drag selections (field-reported from
// two real shares of Acts 4:19–20, BSB). The governing principles, in the
// order they were settled:
//   1. The ONLY content edit is the mid-word repair: Bluebook has no notation
//      for a fragment of a word (Rule 5.3's ellipsis omits WORDS; Rule 5.2's
//      empty brackets — "judgment[]" — presuppose a real root word), so a
//      selection cut mid-word is trimmed to the whole-word boundary.
//   2. Verse-number markers are apparatus, not scripture: stripped no matter
//      how little of their verse was selected (the legacy 12-rune probe let
//      "21 Af" leak a marker into a shared card).
//   3. The citation is a PROVENANCE record: it names exactly the verses that
//      contribute at least one surviving word — any word from a verse puts it
//      in the range; a verse whose only contribution was a trimmed-away
//      partial word (or a bare marker) drops out of both text and range.

import (
	"strings"
	"testing"
)

func acts4ShareState() *AppState {
	bd := &BibleData{
		Books: []string{"Acts"},
		Verses: map[string]map[int][]Verse{"Acts": {4: {
			{BookName: "Acts", Book: "Acts", Chapter: 4, Verse: 19,
				Text: "But Peter and John replied, “Judge for yourselves whether it is right in God’s sight to listen to you rather than God."},
			{BookName: "Acts", Book: "Acts", Chapter: 4, Verse: 20,
				Text: "For we cannot stop speaking about what we have seen and heard.”"},
			{BookName: "Acts", Book: "Acts", Chapter: 4, Verse: 21,
				Text: "After further threats they let them go. They could not find a way to punish them, because all the people were glorifying God for what had happened."},
		}}},
	}
	return &AppState{Bible: bd, CurrentBook: "Acts", CurrentChapter: 4}
}

func TestShareSelectionMidWordEndTrim(t *testing.T) {
	// IMG_0618: the drag stopped inside "heard" — the orphan "h" must go, the
	// rest stands, and the citation covers 19–20.
	st := acts4ShareState()
	raw := "replied, “Judge for yourselves whether it is right in God’s sight to listen to you rather than God. 20 For we cannot stop speaking about what we have seen and h"
	text, cite := prepareShareQuote(st, raw)
	if strings.HasSuffix(text, " h") || strings.Contains(text, " h ") {
		t.Errorf("partial word survived: %q", text)
	}
	if !strings.HasSuffix(text, "seen and") {
		t.Errorf("trim should stop after the last whole word; got %q", text)
	}
	if cite != "Acts 4:19–20" {
		t.Errorf("citation = %q, want Acts 4:19–20", cite)
	}
}

func TestShareSelectionDanglingMarkerDropped(t *testing.T) {
	// IMG_0619: two characters into verse 21 — the partial word goes (rule 1),
	// which leaves the marker introducing nothing, so it goes too (rule 3) and
	// the citation stays 19–20. The result ends on a complete sentence, so the
	// formatted quote must carry NO omission mark.
	st := acts4ShareState()
	raw := "“Judge for yourselves whether it is right in God’s sight to listen to you rather than God. 20 For we cannot stop speaking about what we have seen and heard.” 21 Af"
	text, cite := prepareShareQuote(st, raw)
	if strings.Contains(text, "21") || strings.Contains(text, "Af") {
		t.Errorf("dangling marker/fragment survived: %q", text)
	}
	if !strings.HasSuffix(text, "heard.”") {
		t.Errorf("should end on the complete sentence; got %q", text)
	}
	if cite != "Acts 4:19–20" {
		t.Errorf("citation = %q, want Acts 4:19–20", cite)
	}
	if q := formatBibleQuote(text, originalSentenceTerminal(st, text)); strings.Contains(q, ". . .") {
		t.Errorf("complete sentence must carry no omission mark: %q", q)
	}
}

func TestShareSelectionWholeWordExtendsCitation(t *testing.T) {
	// One whole word of verse 21 ("After") is a real contribution: it stays,
	// its marker is stripped, and the citation extends to 19–21.
	st := acts4ShareState()
	raw := "“Judge for yourselves whether it is right in God’s sight to listen to you rather than God. 20 For we cannot stop speaking about what we have seen and heard.” 21 After"
	text, cite := prepareShareQuote(st, raw)
	if !strings.HasSuffix(text, "heard.” After") {
		t.Errorf("whole word from v21 must be kept, marker stripped; got %q", text)
	}
	if cite != "Acts 4:19–21" {
		t.Errorf("citation = %q, want Acts 4:19–21 (any quoted word from a verse cites it)", cite)
	}
	if q := formatBibleQuote(text, originalSentenceTerminal(st, text)); !strings.Contains(q, ". . . .") {
		t.Errorf("mid-sentence cut must carry the four-dot omission: %q", q)
	}
}

func TestShareSelectionMidWordStartTrim(t *testing.T) {
	// A drag that STARTS mid-word gets the symmetric repair.
	st := acts4ShareState()
	raw := "eplied, “Judge for yourselves whether it is right in God’s sight"
	text, cite := prepareShareQuote(st, raw)
	if strings.HasPrefix(text, "eplied") {
		t.Errorf("leading partial word survived: %q", text)
	}
	if !strings.HasPrefix(text, "“Judge") {
		t.Errorf("should start at the next whole word; got %q", text)
	}
	if cite != "Acts 4:19" {
		t.Errorf("citation = %q, want Acts 4:19", cite)
	}
}

func TestShareSelectionLegitimateNumberUntouched(t *testing.T) {
	// A number that is verse PROSE (not a marker) must never be stripped, even
	// when a verse with that number exists in the chapter.
	bd := &BibleData{
		Books: []string{"Acts"},
		Verses: map[string]map[int][]Verse{"Acts": {4: {
			{BookName: "Acts", Book: "Acts", Chapter: 4, Verse: 21,
				Text: "After further threats they let them go."},
			{BookName: "Acts", Book: "Acts", Chapter: 4, Verse: 22,
				Text: "For the man on whom this sign of healing was performed was over 21 years old."},
		}}},
	}
	st := &AppState{Bible: bd, CurrentBook: "Acts", CurrentChapter: 4}
	raw := "the man on whom this sign of healing was performed was over 21 years old."
	text, cite := prepareShareQuote(st, raw)
	if !strings.Contains(text, "over 21 years") {
		t.Errorf("legitimate in-verse number was stripped: %q", text)
	}
	if cite != "Acts 4:22" {
		t.Errorf("citation = %q, want Acts 4:22", cite)
	}
}

func TestShareSelectionFullVersesUnchanged(t *testing.T) {
	// The everyday case must be untouched by the new pipeline: whole verses,
	// markers stripped, full range cited.
	st := acts4ShareState()
	raw := "But Peter and John replied, “Judge for yourselves whether it is right in God’s sight to listen to you rather than God. 20 For we cannot stop speaking about what we have seen and heard.”"
	text, cite := prepareShareQuote(st, raw)
	if strings.Contains(text, "20 For") {
		t.Errorf("marker not stripped in the full-verse case: %q", text)
	}
	if cite != "Acts 4:19–20" {
		t.Errorf("citation = %q, want Acts 4:19–20", cite)
	}
	if !strings.HasPrefix(text, "But Peter") || !strings.HasSuffix(text, "heard.”") {
		t.Errorf("full-verse text altered: %q", text)
	}
}
