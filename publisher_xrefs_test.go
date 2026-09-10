package bibletext

import (
	"testing"
)

// The publisher's rows are the notes verbatim, in the publisher's order, with
// a tap target only where the source tagged a passage the loaded text has.
// Every check carries a control: the untagged citation, the absent passage,
// the selection that does not reach verse 1.

func TestParseUSFMRefID(t *testing.T) {
	for _, tc := range []struct {
		id   string
		want crossRef
		ok   bool
	}{
		{"JHN.7.50", crossRef{Book: "John", Chapter: 7, Verse: 50}, true},
		{"MAT.3.1-MAT.3.12", crossRef{Book: "Matthew", Chapter: 3, Verse: 1, EndCh: 3, EndV: 12}, true},
		{"2SA.15.13-2SA.15.17", crossRef{Book: "2 Samuel", Chapter: 15, Verse: 13, EndCh: 15, EndV: 17}, true},
		{"MRK.8.34-MRK.9.1", crossRef{Book: "Mark", Chapter: 8, Verse: 34, EndCh: 9, EndV: 1}, true},
		{"PSA.3.1", crossRef{Book: "Psalms", Chapter: 3, Verse: 1}, true},
		{"SNG.2.1", crossRef{Book: "Song of Solomon", Chapter: 2, Verse: 1}, true},
		{" jhn.7.50 ", crossRef{Book: "John", Chapter: 7, Verse: 50}, true},
		// A range into another book, or running backwards, keeps its start.
		{"GEN.50.26-EXO.1.1", crossRef{Book: "Genesis", Chapter: 50, Verse: 26}, true},
		{"JHN.7.50-JHN.7.40", crossRef{Book: "John", Chapter: 7, Verse: 50}, true},
		// Rejections.
		{"", crossRef{}, false},
		{"JHN.7", crossRef{}, false},
		{"JHN.7.50.1", crossRef{}, false},
		{"ZZZ.1.1", crossRef{}, false},
		{"JHN.a.1", crossRef{}, false},
		{"JHN.0.1", crossRef{}, false},
		{"JHN.7.0", crossRef{}, false},
		{"JHN.7.50-JHN.7.51-JHN.7.52", crossRef{}, false},
	} {
		got, ok := parseUSFMRefID(tc.id)
		if ok != tc.ok || got != tc.want {
			t.Errorf("parseUSFMRefID(%q) = %+v, %v; want %+v, %v", tc.id, got, ok, tc.want, tc.ok)
		}
	}
	// The labels the panel would show for a range.
	if r, _ := parseUSFMRefID("MAT.3.1-MAT.3.12"); r.label() != "Matthew 3:1-12" {
		t.Errorf("range label = %q", r.label())
	}
}

func TestCrossRefContains(t *testing.T) {
	single := crossRef{Book: "John", Chapter: 7, Verse: 50}
	rng := crossRef{Book: "Mark", Chapter: 8, Verse: 34, EndCh: 9, EndV: 1}
	same := crossRef{Book: "Matthew", Chapter: 3, Verse: 1, EndV: 12}
	for _, tc := range []struct {
		c     crossRef
		ch, v int
		want  bool
	}{
		{single, 7, 50, true}, {single, 7, 51, false}, {single, 8, 50, false},
		{rng, 8, 34, true}, {rng, 8, 38, true}, {rng, 9, 1, true}, {rng, 9, 2, false}, {rng, 8, 33, false},
		{same, 3, 1, true}, {same, 3, 12, true}, {same, 3, 13, false}, {same, 4, 1, false},
	} {
		if got := tc.c.contains(tc.ch, tc.v); got != tc.want {
			t.Errorf("%+v contains(%d,%d) = %v, want %v", tc.c, tc.ch, tc.v, got, tc.want)
		}
	}
}

// A small NKJV-shaped canon: John 3 with notes of every shape, the passages
// some of them cite, and a titled psalm.
func publisherFixture() *BibleData {
	x := footnoteKindCrossref
	john3 := []Verse{
		{BookName: "John", Book: "John", Chapter: 3, Verse: 1, Text: "There was a man of the Pharisees named Nicodemus.",
			Footnotes: []Footnote{{Anchor: 10, Text: "(Acts 10:38)", Kind: x, Caller: "-"}}}, // untagged: words only
		{BookName: "John", Book: "John", Chapter: 3, Verse: 2, Text: "This man came to Jesus by night and said to Him.",
			Footnotes: []Footnote{
				{Anchor: 31, Text: "John 7:50; 19:39", Kind: x, Caller: "-",
					Refs: []NoteRef{{"JHN.7.50", 0, 9}, {"JHN.19.39", 11, 16}}}, // 19:39 is not in the fixture
				{Anchor: 31, Text: "(John 1:13; Gal. 6:15; 1 John 3:9)", Kind: x, Caller: "-",
					Refs: []NoteRef{{"GAL.6.15", 12, 21}}},
				{Anchor: 40, Text: "Alpha-Text omits the fixture clause.", Caller: "+"}, // a translator note: never a row
			}},
		{BookName: "John", Book: "John", Chapter: 3, Verse: 3, Text: "Jesus answered and said to him.",
			Footnotes: []Footnote{{Anchor: 5, Text: "John 3:1–21", Kind: x, Caller: "-",
				Refs: []NoteRef{{"JHN.3.1-JHN.3.21", 0, 11}}}}}, // contains its own verse: current, not followed
	}
	return &BibleData{
		Books: []string{"Psalms", "Matthew", "John", "Galatians"},
		Verses: map[string]map[int][]Verse{
			"John":      {3: john3, 7: {{BookName: "John", Book: "John", Chapter: 7, Verse: 50, Text: "Nicodemus said to them."}}},
			"Galatians": {6: {{BookName: "Galatians", Book: "Galatians", Chapter: 6, Verse: 15, Text: "A new creation."}}},
			"Psalms": {3: {
				{BookName: "Psalms", Book: "Psalms", Chapter: 3, Verse: 1, Text: "LORD, how they have increased who trouble me!"},
				{BookName: "Psalms", Book: "Psalms", Chapter: 3, Verse: 2, Text: "Many are they who say of me."},
			}},
			"Matthew": {3: {{BookName: "Matthew", Book: "Matthew", Chapter: 3, Verse: 1, Text: "In those days John the Baptist came."}}},
		},
		Superscriptions: map[string]map[int]Superscription{"Psalms": {3: {
			Text:      "A Psalm of David when he fled from Absalom his son.",
			Footnotes: []Footnote{{Anchor: 0, Text: "2 Sam. 15:13–17", Kind: x, Caller: "-", Refs: []NoteRef{{"2SA.15.13-2SA.15.17", 0, 15}}}},
		}}},
	}
}

func TestPublisherCrossRefsAreTheNotesVerbatimInOrder(t *testing.T) {
	bd := publisherFixture()
	rows := publisherCrossRefsFor(bd, "John", 3, bd.GetChapter("John", 3))
	if len(rows) != 4 {
		t.Fatalf("got %d rows, want 4 (three notes on 3:1-2 and one on 3:3; the translator note is not a row): %+v", len(rows), rows)
	}
	// Verse order, then the order the notes stand in the verse.
	if rows[0].Verse != 1 || rows[1].Verse != 2 || rows[2].Verse != 2 || rows[3].Verse != 3 {
		t.Errorf("row order = %d %d %d %d", rows[0].Verse, rows[1].Verse, rows[2].Verse, rows[3].Verse)
	}
	if rows[1].Text != "John 7:50; 19:39" || rows[2].Text != "(John 1:13; Gal. 6:15; 1 John 3:9)" {
		t.Errorf("rows are not the notes verbatim: %q / %q", rows[1].Text, rows[2].Text)
	}
	// The untagged citation is words only.
	if len(rows[0].Targets) != 0 {
		t.Errorf("an untagged citation gained targets: %+v", rows[0].Targets)
	}
	// Of the two tagged citations on the first 3:2 note, only the passage the
	// loaded text has becomes a target; the words of the other remain.
	if len(rows[1].Targets) != 1 || rows[1].Targets[0].Ref != (crossRef{Book: "John", Chapter: 7, Verse: 50}) ||
		rows[1].Targets[0].Start != 0 || rows[1].Targets[0].End != 9 {
		t.Errorf("3:2's targets = %+v, want John 7:50 alone (19:39 is not in the fixture)", rows[1].Targets)
	}
	if rows[1].Text != "John 7:50; 19:39" {
		t.Errorf("dropping a target changed the words: %q", rows[1].Text)
	}
	// The tagged span inside the parenthesised group resolves; the untagged
	// neighbours around it stay words.
	if len(rows[2].Targets) != 1 || rows[2].Targets[0].Ref.Book != "Galatians" || rows[2].Targets[0].Start != 12 {
		t.Errorf("mixed note's targets = %+v", rows[2].Targets)
	}
	// A passage containing the note's own verse is current: shown, not followed.
	if len(rows[3].Targets) != 1 || !rows[3].Targets[0].Current {
		t.Errorf("a self-range is not marked current: %+v", rows[3].Targets)
	}
	if rows[1].Targets[0].Current {
		t.Error("an ordinary target is marked current")
	}
}

func TestPublisherCrossRefsSelectionAndTitle(t *testing.T) {
	bd := publisherFixture()
	// Only the selected verses' notes, in order.
	ch := bd.GetChapter("John", 3)
	rows := publisherCrossRefsFor(bd, "John", 3, ch[2:])
	if len(rows) != 1 || rows[0].Verse != 3 {
		t.Errorf("selecting 3:3 alone gave %+v", rows)
	}
	// A verse with no cross-reference notes gives nothing — not an empty row.
	if rows := publisherCrossRefsFor(bd, "Psalms", 3, bd.GetChapter("Psalms", 3)[1:]); len(rows) != 0 {
		t.Errorf("Psalm 3:2 has no notes but gave %+v", rows)
	}
	// The title's note leads, keyed 0, when the selection reaches verse 1 —
	// and its range resolves like any other target.
	ps := bd.GetChapter("Psalms", 3)
	rows = publisherCrossRefsFor(bd, "Psalms", 3, ps[:1])
	if len(rows) != 1 || rows[0].Verse != 0 || rows[0].Text != "2 Sam. 15:13–17" {
		t.Fatalf("with verse 1 selected the title's note is missing or misplaced: %+v", rows)
	}
	if len(rows[0].Targets) != 0 {
		t.Errorf("2 Samuel is not in the fixture, so the title's citation must be words only: %+v", rows[0].Targets)
	}
	bd.Verses["2 Samuel"] = map[int][]Verse{15: {{BookName: "2 Samuel", Book: "2 Samuel", Chapter: 15, Verse: 13, Text: "Now a messenger came."}}}
	rows = publisherCrossRefsFor(bd, "Psalms", 3, ps[:1])
	if len(rows[0].Targets) != 1 || rows[0].Targets[0].Ref.label() != "2 Samuel 15:13-17" || rows[0].Targets[0].Current {
		t.Errorf("the title's citation did not resolve once the passage was loaded: %+v", rows[0].Targets)
	}
	// CONTROL: a selection that does not reach verse 1 carries no title note.
	if rows := publisherCrossRefsFor(bd, "Psalms", 3, ps[1:]); len(rows) != 0 {
		t.Errorf("the title's note appeared for a selection of verse 2 alone: %+v", rows)
	}
	// Nothing for nothing.
	if rows := publisherCrossRefsFor(bd, "John", 3, nil); rows != nil {
		t.Errorf("an empty selection gave %+v", rows)
	}
	if rows := publisherCrossRefsFor(nil, "John", 3, ch); rows != nil {
		t.Errorf("a nil Bible gave %+v", rows)
	}
}

// A span that does not fit the text it claims to index — a cache from a
// different decoder, say — is dropped rather than sliced out of bounds.
func TestPublisherCrossRefsDropSpansThatDoNotFit(t *testing.T) {
	bd := publisherFixture()
	v := &bd.Verses["John"][3][1]
	v.Footnotes[0].Refs = []NoteRef{{"JHN.7.50", 0, 99}, {"JHN.7.50", 5, 5}, {"JHN.7.50", -1, 4}}
	rows := publisherCrossRefsFor(bd, "John", 3, bd.GetChapter("John", 3)[1:2])
	if len(rows) != 2 || len(rows[0].Targets) != 0 {
		t.Errorf("ill-fitting spans became targets: %+v", rows)
	}
	// CONTROL: a span that fits is kept.
	v.Footnotes[0].Refs = []NoteRef{{"JHN.7.50", 0, 9}}
	if rows := publisherCrossRefsFor(bd, "John", 3, bd.GetChapter("John", 3)[1:2]); len(rows[0].Targets) != 1 {
		t.Errorf("a fitting span was dropped: %+v", rows)
	}
}
