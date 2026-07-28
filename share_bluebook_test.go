package bibletext

// Published Bluebook Rule 5.3(b) examples, asserted VERBATIM. The wider rule
// coverage lives in bluebook_test.go (already sourced to the Indigo Book,
// Georgetown, Baron of the Bluebook, monmouth.edu, and ubalt.edu); these tests
// add the canonical worked pair from the Bluebook's own R5 illustrations as
// published at:
//
//	[Baron] https://baronofthebluebook.wordpress.com/2011/08/25/bb-rule-5-3b-or-who-knew/
//
// so the formatter is checked against real published strings, not only our
// paraphrases of the rules.

import (
	"strings"
	"testing"
)

// [Baron] the canonical pair, exactly as published.
//
//	original: "National borders are less of a barrier to economic exchange
//	           now than at almost any other time in history."
//
// Beginning omitted (5.3(b)(i)) — bracketed capital, never a leading ellipsis:
//
//	"[B]orders are less of a barrier to economic exchange now than at almost
//	 any other time in history."
//
// End omitted (5.3(b)(iii)) — the four-dot form:
//
//	"National borders are less of a barrier to economic exchange now than at
//	 almost any other time . . . ."
func TestBluebookPublishedRule53Examples(t *testing.T) {
	if got, want := bracketStartCapital("borders are less of a barrier to economic exchange now than at almost any other time in history."),
		"[B]orders are less of a barrier to economic exchange now than at almost any other time in history."; got != want {
		t.Errorf("5.3(b)(i) published example:\n got %q\nwant %q", got, want)
	}
	if got, want := addEndOmission("National borders are less of a barrier to economic exchange now than at almost any other time", '.'),
		"National borders are less of a barrier to economic exchange now than at almost any other time . . . ."; got != want {
		t.Errorf("5.3(b)(iii) published example:\n got %q\nwant %q", got, want)
	}
	// Both treatments composed through the real pipeline (a selection cut at
	// BOTH ends), plus Rule 5.1's inline wrapping:
	if got, want := formatBibleQuote("borders are less of a barrier to economic exchange now than at almost any other time", '.'),
		"“[B]orders are less of a barrier to economic exchange now than at almost any other time . . . .”"; got != want {
		t.Errorf("composed 5.3(b) example:\n got %q\nwant %q", got, want)
	}
}

// Yale Style Rule 5.1(a) gloss (implementing Rule 5.1): "[o]mitted words and
// ellipses should not be considered in the word count" — so a 49-word
// mid-sentence cut stays INLINE even though the added four-dot mark visually
// pushes it past fifty tokens. The count must happen before the mark is added.
func TestBluebookEllipsisNotCountedTowardFifty(t *testing.T) {
	cut49 := strings.TrimSpace(strings.Repeat("word ", 49)) // no terminal → will take the four-dot
	got := formatBibleQuote(cut49, '.')
	if !strings.Contains(got, ". . . .") {
		t.Fatalf("mid-sentence cut must take the four-dot mark: %q", firstRunes(got, 40))
	}
	if !strings.HasPrefix(got, "“") {
		t.Errorf("49 quoted words + ellipsis must remain INLINE (marks excluded from the count): %q…", firstRunes(got, 30))
	}
}
