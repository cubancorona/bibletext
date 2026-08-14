package bibletext

// ChapterNumberingAgrees is the predicate the web reader's NKJV notice pages ask
// once per chapter per target translation, at BUILD time, to decide whether a
// "read this verse in another translation" link may carry the verse at all.
// Getting it wrong is silent: the reader is handed a confident link to words the
// sender never pointed at. These pin the answers that matter.

import "testing"

// The overwhelmingly common case. If this ever went false the pages would drop
// the verse from ~1,189 links and the feature would be pointless.
func TestChapterNumberingAgreesOnTheOrdinaryChapter(t *testing.T) {
	for _, to := range []string{"web", "bsb", "webc"} {
		if !ChapterNumberingAgrees("nkjv", to, "John", 3, 36) {
			t.Errorf("nkjv->%s John 3 should map verse-for-verse", to)
		}
	}
}

// The moves and the omissions the deltas record. Each of these is a chapter
// where a carried verse number means a DIFFERENT passage, so the page must
// degrade to a chapter link.
func TestChapterNumberingDisagreesWhereTheDeltasSayItShould(t *testing.T) {
	for _, tc := range []struct {
		to      string
		book    string
		chapter int
		span    int
		why     string
	}{
		// The WEB's Romans 16 STOPS at 24 — it carries the doxology back in
		// chapter 14 — so the span alone never reaches the NKJV's 16:25-27.
		// This is the case the moved-To scan exists for.
		{"web", "Romans", 16, 24, "the NKJV numbers the doxology 16:25-27, the WEB 14:24-26"},
		{"bsb", "Romans", 14, 26, "the WEB's 14:24-26 are the BSB's 16:25-27"},
		{"bsb", "Mark", 9, 50, "the BSB omits 9:44 and 9:46"},
		{"bsb", "Matthew", 17, 27, "the BSB omits 17:21"},
		{"webc", "Esther", 1, 22, "WEBC's Esther is the Greek Esther — incommensurable"},
		{"webc", "Daniel", 3, 30, "WEBC inserts the Song of the Three, pushing 24-30 to 91-97"},
	} {
		if ChapterNumberingAgrees("nkjv", tc.to, tc.book, tc.chapter, tc.span) {
			t.Errorf("nkjv->%s %s %d reported as agreeing, but %s", tc.to, tc.book, tc.chapter, tc.why)
		}
	}
}

// THE CAVEAT THIS FUNCTION EXISTS FOR. Acts 8:37, Acts 15:34, Acts 24:7 and
// Luke 17:36 are in the NKJV and in none of the published three. A probe that
// walked only the reference translation's verse list would never ask about them
// and would call these chapters safe — which is exactly what the first
// measurement of this did, and why the pages would have offered a link to a
// verse that is not there.
func TestChapterNumberingCatchesVersesOnlyTheNKJVHas(t *testing.T) {
	for _, tc := range []struct {
		book    string
		chapter int
		span    int
	}{
		{"Acts", 8, 40},
		{"Acts", 15, 41},
		{"Acts", 24, 27},
		{"Luke", 17, 37},
	} {
		for _, to := range []string{"web", "bsb", "webc"} {
			if ChapterNumberingAgrees("nkjv", to, tc.book, tc.chapter, tc.span) {
				t.Errorf("nkjv->%s %s %d reported as agreeing, but the NKJV has a verse there that %s does not",
					to, tc.book, tc.chapter, to)
			}
		}
	}
}

// The other side of the doxology: the NKJV's Romans 14 has 23 verses and every
// one of them is the WEB's own 14:n, so that chapter's link MAY carry the verse
// even though the two translations disagree about where 16:25 lives. Pinned so
// nobody "fixes" the function into hedging on both chapters.
func TestChapterNumberingKeepsTheVerseWhereItReallyIsTheSame(t *testing.T) {
	if !ChapterNumberingAgrees("nkjv", "web", "Romans", 14, 26) {
		t.Error("nkjv->web Romans 14 maps verse-for-verse; the link should carry the verse")
	}
}

// The KIND of difference decides the sentence the web page shows, and telling a
// reader "the numbering differs" when the verse simply isn't in that
// translation would be false. Each of these is a chapter where the two answers
// diverge.
func TestChapterNumberingDifferenceNamesTheRightKind(t *testing.T) {
	for _, tc := range []struct {
		to      string
		book    string
		chapter int
		span    int
		want    string
	}{
		{"web", "John", 3, 36, NumberingSame},
		{"web", "Romans", 16, 24, NumberingMoved}, // the doxology's move
		{"web", "Acts", 8, 40, NumberingAbsent},   // the NKJV has 8:37, the WEB has not
		{"web", "Luke", 17, 37, NumberingAbsent},  // 17:36
		{"bsb", "Mark", 9, 50, NumberingAbsent},   // the BSB omits 9:44 and 9:46
		{"webc", "Daniel", 3, 30, NumberingMoved}, // the Song of the Three pushes 24-30 to 91-97
		{"webc", "Esther", 1, 22, NumberingIncommensurable},
	} {
		got := ChapterNumberingDifference("nkjv", tc.to, tc.book, tc.chapter, tc.span)
		if got != tc.want {
			t.Errorf("nkjv->%s %s %d: got %q, want %q", tc.to, tc.book, tc.chapter, got, tc.want)
		}
	}
}

// A translation compared with itself always agrees, and an unknown id is
// assumed to share the reference's numbering (toReference's documented
// default) — so neither can make a page hedge for no reason.
func TestChapterNumberingAgreesWithItself(t *testing.T) {
	if !ChapterNumberingAgrees("nkjv", "nkjv", "Romans", 14, 26) {
		t.Error("a translation must agree with itself")
	}
	if !ChapterNumberingAgrees("nkjv", "unknown-translation", "John", 3, 36) {
		t.Error("an unknown target must fall back to the reference's numbering")
	}
}
