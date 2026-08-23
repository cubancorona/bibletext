package bibletext

// CROSS-REFERENCES MUST BE READ IN THE READER'S NUMBERING.
//
// The dataset (Treasury of Scripture Knowledge, and the embedded Gospel
// synopsis) is keyed in ONE numbering — the reference, versification.go. The
// panel used to key it with whatever numbering was on screen and look the
// target up the same way, which was wrong on both sides. Found by auditing the
// two features against the verse-mapping work that landed long after them,
// looking for consistency problems across versions.
//
// Three shapes of defect, one cause. These tests are the mapping arithmetic
// itself, at the exact addresses that were reported — the panel needs a loaded
// Bible and a downloaded dataset, so what can be pinned here is the translation
// of an address, which is the half that was missing.

import "testing"

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
