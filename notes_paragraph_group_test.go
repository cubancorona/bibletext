package bibletext

import "testing"

// The case that started this: five notes on one chapter, two paragraphs.
// One pill today, labelled with the chapter's count and standing over one of
// the two paragraphs. Grouped, each paragraph can carry its own pill and its
// own number.
func TestNotesGroupUnderTheParagraphThatCarriesThem(t *testing.T) {
	paras := [][]Verse{
		{{Verse: 1}, {Verse: 2}, {Verse: 3}},
		{{Verse: 4}, {Verse: 5}, {Verse: 6}},
	}
	note := func(lo int) drawnNote {
		return drawnNote{Placement: placement{Kind: placedNative,
			Here: []anchorRun{{Lo: lo, Hi: lo}}}}
	}
	// Two in the first paragraph, three in the second — deliberately shuffled,
	// because the plan hands notes over newest-first, not in reading order.
	got := groupNotesByParagraph(paras, []drawnNote{
		note(5), note(2), note(6), note(3), note(4),
	})
	if len(got) != 2 {
		t.Fatalf("want 2 paragraph groups, got %d", len(got))
	}
	if got[0].ParaIndex != 0 || got[1].ParaIndex != 1 {
		t.Fatalf("groups must come out in reading order, got paragraphs %d then %d",
			got[0].ParaIndex, got[1].ParaIndex)
	}
	if len(got[0].Notes) != 2 {
		t.Errorf("first paragraph carries 2 notes, grouped %d", len(got[0].Notes))
	}
	if len(got[1].Notes) != 3 {
		t.Errorf("second paragraph carries 3 notes, grouped %d", len(got[1].Notes))
	}
	// The band opens above the paragraph, found by the EARLIEST noted verse in
	// it — not by whichever note the plan happened to hand over first.
	if got[0].BandVerse != 2 {
		t.Errorf("first group's band verse = %d, want 2 (the earlier of vv2,3)", got[0].BandVerse)
	}
	if got[1].BandVerse != 4 {
		t.Errorf("second group's band verse = %d, want 4 (the earliest of vv4,5,6)", got[1].BandVerse)
	}
}

// A note the chapter cannot place has no band to open. Forcing it into a
// neighbouring paragraph would stand a sticker over a passage the note is not
// about, so it is dropped from the grouping instead.
func TestAnUnplaceableNoteJoinsNoParagraph(t *testing.T) {
	paras := [][]Verse{{{Verse: 1}, {Verse: 2}}}
	unplaced := drawnNote{Placement: placement{Kind: unplacedAbsent}}
	beyond := drawnNote{Placement: placement{Kind: placedNative,
		Here: []anchorRun{{Lo: 99, Hi: 99}}}}
	if got := groupNotesByParagraph(paras, []drawnNote{unplaced, beyond}); len(got) != 0 {
		t.Fatalf("neither note can be placed in this chapter; got %d group(s)", len(got))
	}
}
