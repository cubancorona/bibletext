package bibletext

// CROSS-REFERENCES MUST BE READ IN THE READER'S NUMBERING.
//
// The dataset (Treasury of Scripture Knowledge, and the embedded Gospel
// synopsis) is keyed in ONE numbering — the reference, versification.go. The
// panel used to key it with whatever numbering was on screen and look the
// target up the same way, which was wrong on both sides. Both operations must
// go through the same verse-mapping rules across versions.
//
// Three shapes of defect, one cause. These tests are the mapping arithmetic
// itself, at representative mapped addresses — the panel needs a loaded
// Bible and a downloaded dataset, so what can be pinned here is the translation
// of an address, which is the half that was missing.

import (
	"fmt"
	"testing"
)

// THE ONE THAT SHOWED WRONG TEXT. WEB Catholic carries the Song of the Three as
// Daniel 3:24-90, pushing the Hebrew 3:24-30 down to 3:91-97. A cross-reference
// to "Daniel 3:25" — one of the most referenced verses in scripture, "the fourth
// is like a son of the gods" — previewed and jumped to Azariah's prayer instead,
// under a label that looked right. Nothing told the reader.
func TestDanielThreeTargetsLandOnTheRightPassageInWEBCatholic(t *testing.T) {
	for _, tc := range []struct{ ref, want int }{
		{24, 91}, {25, 92}, {26, 93}, {27, 94}, {28, 95}, {29, 96}, {30, 97},
	} {
		c, ok := crossRefTargetIn("webc", crossRef{Book: "Daniel", Chapter: 3, Verse: tc.ref})
		if !ok {
			t.Errorf("Daniel 3:%d cannot be shown in WEB Catholic at all — it is there, at 3:%d",
				tc.ref, tc.want)
			continue
		}
		if c.Verse != tc.want {
			t.Errorf("a cross-reference to Daniel 3:%d resolves to 3:%d in WEB Catholic, want 3:%d.\n"+
				"Unmapped, this row previews and jumps to the Song of the Three under a label "+
				"that reads like Nebuchadnezzar's astonishment.", tc.ref, c.Verse, tc.want)
		}
	}
	// And the reader's own numbering keys the lookup correctly in reverse: a
	// WEBC reader selecting 3:92 must fetch the references filed under 3:25.
	ch, vs, ok := crossRefSourceRef("webc", Verse{BookName: "Daniel", Chapter: 3, Verse: 92})
	if !ok || ch != 3 || vs != 25 {
		t.Errorf("selecting WEB Catholic Daniel 3:92 keys the index at %d:%d (ok=%v), want 3:25 — "+
			"otherwise the reader gets the cross-references of a different passage", ch, vs, ok)
	}
}

// THE ONE THAT SHOWED NOTHING. The Romans doxology sits at 16:25-27 in the BSB
// and NKJV and at 14:24-26 in the WEB and WEB Catholic. A single number-keyed
// table can only be filed under one of them, so readers of the other half were
// told "No cross-references for this selection" on a passage that has many.
func TestTheRomansDoxologyIsReachableFromEveryTranslation(t *testing.T) {
	for _, tc := range []struct {
		vid            string
		chapter, verse int
	}{
		{"web", 14, 24}, {"webc", 14, 24}, {"bsb", 16, 25}, {"nkjv", 16, 25},
	} {
		ch, vs, ok := crossRefSourceRef(tc.vid, Verse{BookName: "Romans", Chapter: tc.chapter, Verse: tc.verse})
		if !ok {
			t.Errorf("%s Romans %d:%d maps to nothing in the reference, so its cross-references "+
				"can never be found", tc.vid, tc.chapter, tc.verse)
			continue
		}
		// Whatever the reference calls it, all four must agree on ONE address —
		// that agreement is what makes the doxology reachable from all of them.
		if ch != 14 && ch != 16 {
			t.Errorf("%s Romans %d:%d keyed at %d:%d, which is neither placement",
				tc.vid, tc.chapter, tc.verse, ch, vs)
		}
	}
	// The four must land on the SAME key, or the table still only serves some.
	var keys []string
	for _, tc := range []struct {
		vid            string
		chapter, verse int
	}{{"web", 14, 24}, {"webc", 14, 24}, {"bsb", 16, 25}, {"nkjv", 16, 25}} {
		ch, vs, _ := crossRefSourceRef(tc.vid, Verse{BookName: "Romans", Chapter: tc.chapter, Verse: tc.verse})
		keys = append(keys, crossRefKey("Romans", ch, vs))
	}
	for i := range keys {
		if keys[i] != keys[0] {
			t.Errorf("the doxology keys differently per translation (%v) — one number-keyed table "+
				"cannot serve them all, which is the defect", keys)
			break
		}
	}
}

// THE ONE THAT SHOWED A BLANK ROW. Where a translation omits a verse the dataset
// references, the panel rendered the label with empty space under it and a tap
// that closed the panel and went nowhere. Such a row is now dropped.
func TestTargetsAbsentFromTheReadersTranslationAreDropped(t *testing.T) {
	// Verses the BSB omits as later additions; the NKJV keeps them.
	for _, v := range []struct {
		book           string
		chapter, verse int
	}{
		{"Mark", 9, 44}, {"Mark", 9, 46}, {"Mark", 11, 26}, {"Matthew", 17, 21},
	} {
		if _, ok := crossRefTargetIn("bsb", crossRef{Book: v.book, Chapter: v.chapter, Verse: v.verse}); ok {
			t.Errorf("%s %d:%d is offered as a cross-reference in the BSB, which does not contain it — "+
				"the row renders blank and its tap goes nowhere", v.book, v.chapter, v.verse)
		}
		// The same reference in a translation that HAS the verse must survive:
		// dropping everything would be a different bug wearing this fix's face.
		if _, ok := crossRefTargetIn("nkjv", crossRef{Book: v.book, Chapter: v.chapter, Verse: v.verse}); !ok {
			t.Errorf("%s %d:%d was dropped for the NKJV, which does contain it", v.book, v.chapter, v.verse)
		}
	}
}

// AND THE COMMON CASE IS UNTOUCHED. Almost every address agrees across all four
// translations; the mapping must be an identity there, or this fix would quietly
// rewrite the 99.9% to repair the 0.08%.
func TestTheOverwhelminglyCommonCaseIsAnIdentity(t *testing.T) {
	for _, vid := range []string{"web", "webc", "bsb", "nkjv"} {
		for _, c := range []crossRef{
			{Book: "John", Chapter: 3, Verse: 16},
			{Book: "Psalms", Chapter: 23, Verse: 1},
			{Book: "Genesis", Chapter: 1, Verse: 1},
			{Book: "Isaiah", Chapter: 53, Verse: 5, EndCh: 53, EndV: 6},
		} {
			got, ok := crossRefTargetIn(vid, c)
			if !ok {
				t.Errorf("%s: %s was dropped — it exists in every translation", vid, c.label())
				continue
			}
			if got.Chapter != c.Chapter || got.Verse != c.Verse {
				t.Errorf("%s: %s was rewritten to %d:%d — the mapping must be an identity here",
					vid, c.label(), got.Chapter, got.Verse)
			}
		}
	}
}

// A RANGE WHOSE START MOVES CHAPTER. The doxology's ranges are 14:24-25 and
// 14:24-26 in the reference and 16:25-26 and 16:25-27 in the BSB and the
// NKJV. The end used to be looked up in the start's REWRITTEN chapter —
// "Romans 16:25" in the reference, which the reference does not have and the
// table therefore passes through unchanged — so the rows read "Romans
// 16:25-25" and "16:25-26", one verse short, on Matthew 24:14, Colossians
// 1:26, Jude 1:24 and fourteen other verses.
func TestADoxologyRangeKeepsItsEndInEveryTranslation(t *testing.T) {
	for _, tc := range []struct {
		vid        string
		end        int
		want, from string
	}{
		{"bsb", 25, "Romans 16:25-26", "14:24-25"},
		{"bsb", 26, "Romans 16:25-27", "14:24-26"},
		{"nkjv", 25, "Romans 16:25-26", "14:24-25"},
		{"nkjv", 26, "Romans 16:25-27", "14:24-26"},
		{"web", 25, "Romans 14:24-25", "14:24-25"},
		{"webc", 26, "Romans 14:24-26", "14:24-26"},
	} {
		got, ok := crossRefTargetIn(tc.vid, crossRef{Book: "Romans", Chapter: 14, Verse: 24, EndV: tc.end})
		if !ok {
			t.Errorf("%s: Romans %s was dropped; every translation has the doxology", tc.vid, tc.from)
			continue
		}
		if got.label() != tc.want {
			t.Errorf("%s: Romans %s reads %q, want %q", tc.vid, tc.from, got.label(), tc.want)
		}
	}
}

// And a range that collapses to one verse is labelled as one.
func TestARangeEndingOnItsStartIsLabelledAsOneVerse(t *testing.T) {
	for _, c := range []crossRef{
		{Book: "Romans", Chapter: 16, Verse: 25, EndV: 25},
		{Book: "Romans", Chapter: 16, Verse: 25, EndCh: 16, EndV: 25},
	} {
		if got := c.label(); got != "Romans 16:25" {
			t.Errorf("%+v is labelled %q, want \"Romans 16:25\"", c, got)
		}
	}
}

// TWO VERSES IN THE OPPOSITE ORDER UNDER THE SAME NUMBERS. The BSB's
// Philippians 1:16 is "the latter do so in love" and its 1:17 "the former …
// out of selfish ambition"; the WEB, WEB Catholic and the NKJV number them the
// other way round. The NKJV's Matthew 23:13 is the kingdom woe and its 23:14
// the widows' woe; the WEB's are the other way round, and the BSB has only
// the kingdom woe, as 23:13. A span that crosses such a pair must still be a
// span, and hold the verses it names.
func TestRangesThroughReorderedVersesHoldTheirVerses(t *testing.T) {
	for _, tc := range []struct {
		vid  string
		in   crossRef
		want string // "" = dropped
	}{
		{"bsb", crossRef{Book: "Philippians", Chapter: 1, Verse: 16}, "Philippians 1:17"},
		{"bsb", crossRef{Book: "Philippians", Chapter: 1, Verse: 17}, "Philippians 1:16"},
		{"bsb", crossRef{Book: "Philippians", Chapter: 1, Verse: 16, EndV: 17}, "Philippians 1:16-17"},
		{"bsb", crossRef{Book: "Philippians", Chapter: 1, Verse: 12, EndV: 17}, "Philippians 1:12-17"},
		{"bsb", crossRef{Book: "Philippians", Chapter: 1, Verse: 13, EndV: 16}, "Philippians 1:13-17"},
		{"web", crossRef{Book: "Philippians", Chapter: 1, Verse: 13, EndV: 16}, "Philippians 1:13-16"},
		{"nkjv", crossRef{Book: "Matthew", Chapter: 23, Verse: 13}, "Matthew 23:14"},
		{"nkjv", crossRef{Book: "Matthew", Chapter: 23, Verse: 13, EndV: 14}, "Matthew 23:13-14"},
		{"nkjv", crossRef{Book: "Matthew", Chapter: 23, Verse: 14, EndV: 39}, "Matthew 23:13-39"},
		{"nkjv", crossRef{Book: "Matthew", Chapter: 23, Verse: 1, EndV: 13}, "Matthew 23:1-14"},
		// The BSB does not have the widows' woe at all: a row to it is
		// dropped, as a row to Mark 9:44 is. A range that STARTS on it still
		// holds every verse after it, and begins at the BSB's 23:13 — the
		// kingdom woe, which is the reference's 23:14.
		{"bsb", crossRef{Book: "Matthew", Chapter: 23, Verse: 13}, ""},
		{"bsb", crossRef{Book: "Matthew", Chapter: 23, Verse: 14}, "Matthew 23:13"},
		{"bsb", crossRef{Book: "Matthew", Chapter: 23, Verse: 13, EndV: 36}, "Matthew 23:13-36"},
		{"bsb", crossRef{Book: "Matthew", Chapter: 23, Verse: 13, EndV: 14}, "Matthew 23:13"},
		{"web", crossRef{Book: "Matthew", Chapter: 23, Verse: 13, EndV: 36}, "Matthew 23:13-36"},
	} {
		got, ok := crossRefTargetIn(tc.vid, tc.in)
		switch {
		case tc.want == "" && ok:
			t.Errorf("%s: %s is offered as %q; this translation does not have it", tc.vid, tc.in.label(), got.label())
		case tc.want != "" && !ok:
			t.Errorf("%s: %s was dropped, want %q", tc.vid, tc.in.label(), tc.want)
		case ok && got.label() != tc.want:
			t.Errorf("%s: %s reads %q, want %q", tc.vid, tc.in.label(), got.label(), tc.want)
		}
	}
}

// The dataset's rows land on the right verse in every translation. Its
// Philippians 1:16 and 1:17 follow the KJV's order, the WEB's; its Matthew
// 23:13 as a source is the kingdom woe, the WEB's 23:14, while its rows
// pointing at 23:13 are mostly the widows' woe that its verse set has no
// 23:14 for; and its one row from 1:17 in the ESV's order, Philippians 2:3,
// belongs to the selfish-ambition verse.
func TestTheDatasetsRowsLandOnTheSamePassageInEveryTranslation(t *testing.T) {
	withCrossRefIndex(t, "Phil.1.16\t2Cor.2.17\t9\n"+
		"Phil.1.17\tActs.22.1\t9\n"+
		"Phil.1.17\tPhil.2.3\t4\n"+
		"Matt.23.13\tLuke.11.52\t20\n"+
		"Mark.12.40\tMatt.23.13\t5\n"+
		"Ezek.34.7\tMatt.23.13-Matt.23.36\t3\n")
	bd := xrefBible(map[string]map[int]int{
		"Philippians": {1: 30, 2: 30}, "2 Corinthians": {2: 17}, "Acts": {22: 30},
		"Matthew": {23: 39}, "Luke": {11: 54}, "Mark": {12: 44}, "Ezekiel": {34: 31},
	})
	for _, tc := range []struct {
		vid, book string
		ch, v     int
		want      []string
	}{
		// The selfish-ambition verse.
		{"web", "Philippians", 1, 16, []string{"2 Corinthians 2:17", "Philippians 2:3"}},
		{"nkjv", "Philippians", 1, 16, []string{"2 Corinthians 2:17", "Philippians 2:3"}},
		{"bsb", "Philippians", 1, 17, []string{"2 Corinthians 2:17", "Philippians 2:3"}},
		// The love-and-defence verse.
		{"web", "Philippians", 1, 17, []string{"Acts 22:1"}},
		{"bsb", "Philippians", 1, 16, []string{"Acts 22:1"}},
		// The kingdom woe.
		{"web", "Matthew", 23, 14, []string{"Luke 11:52"}},
		{"webc", "Matthew", 23, 14, []string{"Luke 11:52"}},
		{"nkjv", "Matthew", 23, 13, []string{"Luke 11:52"}},
		{"bsb", "Matthew", 23, 13, []string{"Luke 11:52"}},
		// The widows' woe has no rows of its own in the dataset.
		{"web", "Matthew", 23, 13, nil},
		{"nkjv", "Matthew", 23, 14, nil},
		// A row pointing at the widows' woe opens it, and the BSB, which does
		// not have it, does not offer it.
		{"web", "Mark", 12, 40, []string{"Matthew 23:13"}},
		{"nkjv", "Mark", 12, 40, []string{"Matthew 23:14"}},
		{"bsb", "Mark", 12, 40, nil},
		// The woes as a passage.
		{"web", "Ezekiel", 34, 7, []string{"Matthew 23:13-36"}},
		{"nkjv", "Ezekiel", 34, 7, []string{"Matthew 23:13-36"}},
		{"bsb", "Ezekiel", 34, 7, []string{"Matthew 23:13-36"}},
	} {
		st := &AppState{Bible: bd, CurrentBook: tc.book, CurrentChapter: tc.ch, CurrentVersion: tc.vid}
		var got []string
		for _, c := range crossRefsForSelection(st, "", selSpan{lo: tc.v, hi: tc.v}) {
			if !c.Parallel {
				got = append(got, c.label())
			}
		}
		if fmt.Sprint(got) != fmt.Sprint(tc.want) {
			t.Errorf("%s %s %d:%d lists %q, want %q", tc.vid, tc.book, tc.ch, tc.v, got, tc.want)
		}
	}
}

// A RANGE WHOSE LAST VERSE THE TRANSLATION LACKS. The synopsis gives the
// healing of the epileptic boy as Matthew 17:14-21 and the fig tree's lesson
// as Mark 11:20-26; the BSB has neither 17:21 nor 11:26. The row used to keep
// only its start, so a BSB reader of Mark 9 or Matthew 21 was offered
// "Matthew 17:14" and "Mark 11:20" as the parallel, the first verse of a
// passage the text prints in full up to the verse before the missing one.
func TestARangeWhoseEndTheTranslationLacksEndsAtTheVerseBefore(t *testing.T) {
	for _, tc := range []struct {
		vid  string
		in   crossRef
		want string
	}{
		{"bsb", crossRef{Book: "Matthew", Chapter: 17, Verse: 14, EndV: 21}, "Matthew 17:14-20"},
		{"bsb", crossRef{Book: "Mark", Chapter: 11, Verse: 20, EndV: 26}, "Mark 11:20-25"},
		// Past two missing verses, to the one before both.
		{"bsb", crossRef{Book: "Mark", Chapter: 9, Verse: 42, EndV: 46}, "Mark 9:42-45"},
		{"bsb", crossRef{Book: "Mark", Chapter: 9, Verse: 43, EndV: 44}, "Mark 9:43"},
		{"bsb", crossRef{Book: "Luke", Chapter: 23, Verse: 16, EndV: 17}, "Luke 23:16"},
		// CONTROL: the translations that have the last verse keep it.
		{"web", crossRef{Book: "Matthew", Chapter: 17, Verse: 14, EndV: 21}, "Matthew 17:14-21"},
		{"nkjv", crossRef{Book: "Mark", Chapter: 11, Verse: 20, EndV: 26}, "Mark 11:20-26"},
	} {
		got, ok := crossRefTargetIn(tc.vid, tc.in)
		if !ok {
			t.Errorf("%s: %s was dropped, want %q", tc.vid, tc.in.label(), tc.want)
			continue
		}
		if got.label() != tc.want {
			t.Errorf("%s: %s reads %q, want %q", tc.vid, tc.in.label(), got.label(), tc.want)
		}
	}

	// And in the panel: a BSB reader of Mark 9:14 is offered Matthew's
	// telling of the healing as far as the BSB prints it.
	bd := xrefBible(map[string]map[int]int{"Matthew": {17: 27}, "Mark": {9: 50}, "Luke": {9: 62}})
	st := &AppState{Bible: bd, CurrentBook: "Mark", CurrentChapter: 9, CurrentVersion: "bsb"}
	var parallels []string
	for _, c := range crossRefsForSelection(st, "", selSpan{lo: 14, hi: 14}) {
		if c.Parallel {
			parallels = append(parallels, c.label())
		}
	}
	if fmt.Sprint(parallels) != "[Matthew 17:14-20 Luke 9:37-43]" {
		t.Errorf("BSB Mark 9:14's parallels are %q, want Matthew 17:14-20 and Luke 9:37-43", parallels)
	}
}
