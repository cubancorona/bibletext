package bibletext

import (
	"maps"
	"testing"
)

// The cases below are the ones a reader can actually reach, and each is a real
// passage rather than a synthetic reference: the Romans doxology, the eleven
// textual-critical omissions, the Textus Receptus verses the NKJV carries, the
// Song of the Three pushing Daniel 3's tail down, and the Greek Esther, which
// the shipping build cannot map at all and the next major release maps verse
// for verse, since it keeps the Hebrew numbers and adds verses of its own
// (greek_esther_current_test.go, greek_esther_next_test.go).
//
// They are pinned here rather than left to the generator because a regeneration
// (a translation's cache epoch bumps, a translation is added) must not be able
// to change what the app does to a reader's link without someone noticing.

func TestMapVerseKnownDivergences(t *testing.T) {
	for _, tc := range []struct {
		name           string
		from, to, book string
		chapter, verse int
		wantCh, wantV  int
		want           verseMapResult
	}{
		// The Romans doxology: the WEB is the odd one out, and the BSB and NKJV
		// agree with each other.
		{"WEB doxology into the BSB", "web", "bsb", "Romans", 14, 24, 16, 25, verseMapMoved},
		{"WEB doxology into the NKJV", "web", "nkjv", "Romans", 14, 26, 16, 27, verseMapMoved},
		{"BSB doxology back into the WEB", "bsb", "web", "Romans", 16, 25, 14, 24, verseMapMoved},
		{"BSB doxology into the NKJV — two moves that cancel", "bsb", "nkjv", "Romans", 16, 25, 16, 25, verseMapExact},

		// Verses the BSB omits on textual-critical grounds. A link to one of
		// these names nothing there, and the caller must be told so rather than
		// silently shown a neighbour.
		{"Mark 9:44 is not in the BSB", "web", "bsb", "Mark", 9, 44, 0, 0, verseMapAbsent},
		{"Matthew 17:21 is not in the BSB", "web", "bsb", "Matthew", 17, 21, 0, 0, verseMapAbsent},
		{"John 5:4 is not in the BSB", "web", "bsb", "John", 5, 4, 0, 0, verseMapAbsent},
		{"Romans 16:24 is not in the BSB", "web", "bsb", "Romans", 16, 24, 0, 0, verseMapAbsent},

		// ...but the NKJV keeps them, so the same link works there.
		{"Mark 9:44 IS in the NKJV", "web", "nkjv", "Mark", 9, 44, 9, 44, verseMapExact},

		// Verses the NKJV has and the WEB does not.
		{"Acts 8:37 has nowhere to go in the WEB", "nkjv", "web", "Acts", 8, 37, 0, 0, verseMapAbsent},
		{"Luke 17:36 has nowhere to go in the BSB", "nkjv", "bsb", "Luke", 17, 36, 0, 0, verseMapAbsent},

		// The Song of the Three lands in Daniel 3 in the Catholic edition and
		// pushes the chapter's last seven verses down by 67.
		{"Daniel 3:24 in WEBC numbering", "web", "webc", "Daniel", 3, 24, 3, 91, verseMapMoved},
		{"Daniel 3:30 in WEBC numbering", "web", "webc", "Daniel", 3, 30, 3, 97, verseMapMoved},
		{"and back again", "webc", "web", "Daniel", 3, 91, 3, 24, verseMapMoved},
		{"Daniel 1:1 is untouched", "web", "webc", "Daniel", 1, 1, 1, 1, verseMapExact},

		// Two adjacent verses in the opposite order under the same numbers —
		// invisible to any comparison of verse-number sets, which is how the
		// table missed them. The WEB's Matthew 23:13 is "you devour widows'
		// houses" and its 23:14 "you shut up the Kingdom"; the NKJV has them
		// the other way round, and the BSB has only the kingdom woe, as 23:13.
		{"the widows' woe into the NKJV", "web", "nkjv", "Matthew", 23, 13, 23, 14, verseMapMoved},
		{"the kingdom woe into the NKJV", "web", "nkjv", "Matthew", 23, 14, 23, 13, verseMapMoved},
		{"the NKJV's kingdom woe back into the WEB", "nkjv", "web", "Matthew", 23, 13, 23, 14, verseMapMoved},
		{"WEB Catholic follows the WEB", "webc", "nkjv", "Matthew", 23, 14, 23, 13, verseMapMoved},
		{"the kingdom woe into the BSB", "web", "bsb", "Matthew", 23, 14, 23, 13, verseMapMoved},
		{"the widows' woe is not in the BSB", "web", "bsb", "Matthew", 23, 13, 0, 0, verseMapAbsent},
		{"the BSB's 23:13 is the WEB's 23:14", "bsb", "web", "Matthew", 23, 13, 23, 14, verseMapMoved},
		{"BSB and NKJV agree on the kingdom woe", "bsb", "nkjv", "Matthew", 23, 13, 23, 13, verseMapExact},
		{"the NKJV's widows' woe is not in the BSB", "nkjv", "bsb", "Matthew", 23, 14, 0, 0, verseMapAbsent},
		// The BSB's Philippians 1:16 is "the latter do so in love", which the
		// WEB, the NKJV and WEB Catholic number 1:17, and its 1:17 is their
		// 1:16, "out of selfish ambition".
		{"the love verse into the BSB", "web", "bsb", "Philippians", 1, 17, 1, 16, verseMapMoved},
		{"the ambition verse into the BSB", "web", "bsb", "Philippians", 1, 16, 1, 17, verseMapMoved},
		{"the BSB's love verse in the NKJV", "bsb", "nkjv", "Philippians", 1, 16, 1, 17, verseMapMoved},
		{"NKJV and WEB agree there", "nkjv", "web", "Philippians", 1, 16, 1, 16, verseMapExact},

		// The Greek Esther's cases are the switch's: greek_esther_current_test.go
		// and greek_esther_next_test.go.

		// The ordinary case, which is ~31,000 verses.
		{"John 3:16 is John 3:16 everywhere", "web", "bsb", "John", 3, 16, 3, 16, verseMapExact},
		{"Psalm 23:1 likewise", "nkjv", "webc", "Psalms", 23, 1, 23, 1, verseMapExact},
		{"same translation is always exact", "bsb", "bsb", "Mark", 9, 44, 9, 44, verseMapExact},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ch, v, res := MapVerse(tc.from, tc.to, tc.book, tc.chapter, tc.verse)
			if res != tc.want || ch != tc.wantCh || v != tc.wantV {
				t.Errorf("%s %s %d:%d -> %s = %d:%d (%s), want %d:%d (%s)",
					tc.from, tc.book, tc.chapter, tc.verse, tc.to, ch, v, res, tc.wantCh, tc.wantV, tc.want)
			}
		})
	}
}

// Mapping a verse out and back must return the verse it started from, or the
// feature is worse than useless — it would walk a reader's note a little further
// from home on every translation switch.
func TestMapVerseRoundTrips(t *testing.T) {
	// Samples are given in the REFERENCE's numbering and the round trip always
	// starts there. Starting from an arbitrary translation would mean inventing
	// references that do not exist in it — the WEB has no Romans 16:27 at all,
	// and asking the map about one is a question with no honest answer.
	versions := []string{"bsb", "webc", "nkjv"}
	samples := []verseRef{
		{"Genesis", 1, 1}, {"Psalms", 23, 1}, {"Isaiah", 53, 5},
		{"Matthew", 5, 3}, {"John", 3, 16}, {"Romans", 8, 28},
		{"Romans", 14, 23}, {"Romans", 16, 23}, {"Daniel", 3, 24}, {"Mark", 9, 43},
		{"Esther", 1, 1}, {"Esther", 4, 17}, {"Esther", 8, 13}, {"Esther", 10, 3},
		{"Matthew", 23, 13}, {"Matthew", 23, 14}, {"Philippians", 1, 16}, {"Philippians", 1, 17},
		{"Revelation", 22, 21},
	}
	const from = "web"
	{
		for _, to := range versions {
			for _, s := range samples {
				ch, v, res := MapVerse(from, to, s.Book, s.Chapter, s.Verse)
				if res == verseMapAbsent || res == verseMapIncommensurable {
					continue // nothing to come back from; that is a real answer
				}
				backCh, backV, backRes := MapVerse(to, from, s.Book, ch, v)
				if backRes == verseMapAbsent || backRes == verseMapIncommensurable {
					t.Errorf("%s->%s %s %d:%d mapped to %d:%d, but %s->%s says that is %s",
						from, to, s.Book, s.Chapter, s.Verse, ch, v, to, from, backRes)
					continue
				}
				if backCh != s.Chapter || backV != s.Verse {
					t.Errorf("round trip %s->%s->%s moved %s %d:%d to %d:%d",
						from, to, from, s.Book, s.Chapter, s.Verse, backCh, backV)
				}
			}
		}
	}
}

// The retained WEB verse-level API may still supply references to callers that
// subsequently map them. It must not name a book whose target versification is
// incommensurable. Rendering itself does not transfer these marks: each edition
// now uses only its own publisher table.
func TestWEBWordsOfChristReferencesAvoidIncommensurableBooks(t *testing.T) {
	unmappable := map[string][]string{}
	for book, ranges := range wordsOfChristRanges {
		for _, r := range ranges {
			for ch := r.startCh; ch <= r.endCh; ch++ {
				lo, hi := r.startV, r.endV
				if ch != r.startCh {
					lo = 1
				}
				if ch != r.endCh {
					hi = lo // one probe per chapter is enough to catch a whole-book problem
				}
				for v := lo; v <= hi && v <= lo+2; v++ {
					for _, vid := range []string{"bsb", "webc", "nkjv"} {
						if _, _, res := MapVerse("web", vid, book, ch, v); res == verseMapIncommensurable {
							unmappable[vid] = append(unmappable[vid], book)
						}
					}
				}
			}
		}
	}
	for vid, books := range unmappable {
		t.Errorf("%s: red-letter verses fall in books with no verse correspondence: %v", vid, books)
	}
}

func TestVerseExistsIn(t *testing.T) {
	for _, tc := range []struct {
		vid, book string
		ch, v     int
		want      bool
	}{
		{"bsb", "Mark", 9, 44, false},
		{"nkjv", "Mark", 9, 44, true},
		{"bsb", "John", 3, 16, true},
		// The Greek Esther's: greek_esther_current_test.go, greek_esther_next_test.go.
	} {
		if got := VerseExistsIn(tc.vid, tc.book, tc.ch, tc.v); got != tc.want {
			t.Errorf("VerseExistsIn(%s, %s %d:%d) = %v, want %v", tc.vid, tc.book, tc.ch, tc.v, got, tc.want)
		}
	}
}

// withIncommensurableBook records book as incommensurable between the
// reference and vid until the test ends. The shipping table has one book that
// is (the Greek Esther) and the next major release's none, so the arms that
// answer for one are exercised, in both states, on a book marked here.
func withIncommensurableBook(t *testing.T, vid, book string) {
	t.Helper()
	was, had := versificationDeltas[vid]
	d := was
	d.incommensurable = maps.Clone(was.incommensurable)
	if d.incommensurable == nil {
		d.incommensurable = map[string]string{}
	}
	d.incommensurable[book] = "verse numbers do not correspond"
	versificationDeltas[vid] = d
	t.Cleanup(func() {
		if had {
			versificationDeltas[vid] = was
		} else {
			delete(versificationDeltas, vid)
		}
	})
}

// A BOOK THAT DOES NOT CORRESPOND maps nowhere, either way, and says why.
func TestAnIncommensurableBookMapsNowhere(t *testing.T) {
	// CONTROL: the book maps before it is marked.
	if _, _, res := MapVerse("web", "bsb", "Ruth", 1, 16); res != verseMapExact {
		t.Fatalf("control: Ruth 1:16 maps %s into the BSB before it is marked", res)
	}
	withIncommensurableBook(t, "bsb", "Ruth")
	for _, pair := range [][2]string{{"web", "bsb"}, {"bsb", "web"}, {"bsb", "nkjv"}} {
		if ch, v, res := MapVerse(pair[0], pair[1], "Ruth", 1, 16); res != verseMapIncommensurable || ch != 0 || v != 0 {
			t.Errorf("%s->%s Ruth 1:16 = %d:%d (%s), want 0:0 (incommensurable)", pair[0], pair[1], ch, v, res)
		}
	}
	if VerseExistsIn("bsb", "Ruth", 1, 16) {
		t.Error("VerseExistsIn offers a verse of a book that does not correspond")
	}
	if IncommensurableBook("bsb", "Ruth") == "" {
		t.Error("IncommensurableBook gives no reason for a book that does not correspond")
	}
	if got := ChapterNumberingDifference("nkjv", "bsb", "Ruth", 1, 22); got != NumberingIncommensurable {
		t.Errorf("nkjv->bsb Ruth 1: got %q, want %q", got, NumberingIncommensurable)
	}
}
