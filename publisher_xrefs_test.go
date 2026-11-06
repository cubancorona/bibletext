package bibletext

import (
	"strings"
	"testing"
)

// The publisher's notes resolve verbatim, in the publisher's order, with a
// target only where the source tagged a passage the loaded text has; split
// then divides each note into the citations that become cards and the words
// that stay. Every check carries a control: the untagged citation, the absent
// passage, the selection that does not reach verse 1, the group without its
// parentheses.

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
	// The labels the panel would show for a range: an en dash, as the header
	// spells the selection.
	if r, _ := parseUSFMRefID("MAT.3.1-MAT.3.12"); r.label() != "Matthew 3:1–12" {
		t.Errorf("range label = %q", r.label())
	}
	if r, _ := parseUSFMRefID("MRK.8.34-MRK.9.1"); r.label() != "Mark 8:34–9:1" {
		t.Errorf("cross-chapter range label = %q", r.label())
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

// A small NKJV-shaped canon, invented words throughout: John 3 with notes of
// every plain shape, the passages some of them cite, a titled psalm, and one
// verse for each of the three shapes the census found beyond plain citations
// — a citation of a Psalm's title, a citation of whole chapters, and a
// citation the edition qualifies by its basis.
func publisherFixture() *BibleData {
	x := footnoteKindCrossref
	xr := func(text string, refs ...NoteRef) Footnote {
		return Footnote{Text: text, Kind: x, Caller: "-", Refs: refs}
	}
	verse := func(book string, ch, v int, text string, notes ...Footnote) Verse {
		return Verse{BookName: book, Book: book, Chapter: ch, Verse: v, Text: text, Footnotes: notes}
	}
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
		Books: []string{"Genesis", "Leviticus", "Deuteronomy", "1 Samuel", "2 Kings", "Job", "Psalms", "Matthew", "Mark", "John", "Romans", "Galatians", "Ephesians", "2 Thessalonians", "Hebrews"},
		Verses: map[string]map[int][]Verse{
			"John":      {3: john3, 7: {verse("John", 7, 50, "Nicodemus said to them.")}},
			"Galatians": {6: {verse("Galatians", 6, 15, "A new creation.")}},
			"Psalms": {
				3: {
					verse("Psalms", 3, 1, "LORD, how they have increased who trouble me!"),
					verse("Psalms", 3, 2, "Many are they who say of me."),
				},
				45: {verse("Psalms", 45, 4, "Ride on for the cause of the fixture."), verse("Psalms", 45, 5, "The arrows of the fixture are sharp.")},
				59: {verse("Psalms", 59, 1, "Rescue me from the invented foes, my God."), verse("Psalms", 59, 2, "Lift me above the fixture's workers.")},
				97: {verse("Psalms", 97, 7, "Let every carved thing be put to shame.")},
			},
			"Matthew":         {3: {verse("Matthew", 3, 1, "In those days John the Baptist came.")}},
			"Mark":            {1: {verse("Mark", 1, 3, "A voice in the invented wilderness.")}},
			"Romans":          {5: {verse("Romans", 5, 8, "The fixture's love is shown here.")}},
			"Ephesians":       {2: {verse("Ephesians", 2, 4, "Rich in the fixture's mercy.")}},
			"2 Thessalonians": {2: {verse("2 Thessalonians", 2, 16, "Everlasting fixture comfort.")}},
			"Job":             {13: {verse("Job", 13, 16, "This also shall be the fixture's salvation.")}},
			"Deuteronomy":     {32: {verse("Deuteronomy", 32, 43, "Rejoice, O fixture nations, with His people.")}},
			// A citation of a Psalm's title ("Ps. 59:title") beside one the
			// fixture lacks (Judges).
			"1 Samuel": {19: {verse("1 Samuel", 19, 12, "So they let him down through the fixture window.",
				xr("Judg. 16:2; Ps. 59:title", NoteRef{"JDG.16.2", 0, 10}, NoteRef{"PSA.59", 12, 18}))}},
			// A citation of a whole chapter, and of a run of chapters.
			"Genesis": {7: {verse("Genesis", 7, 2, "Of every clean fixture animal take seven pairs.",
				xr("Lev. 11", NoteRef{"LEV.11", 0, 7}))}},
			"Leviticus": {11: {verse("Leviticus", 11, 1, "The fixture's clean and unclean."), verse("Leviticus", 11, 2, "These are the fixture animals you may eat.")}},
			"2 Kings":   {18: {verse("2 Kings", 18, 1, "The fixture king began to reign.")}, 20: {verse("2 Kings", 20, 1, "In those fixture days he was sick."), verse("2 Kings", 20, 2, "He turned his face to the fixture wall.")}},
			// A citation the edition qualifies by its basis, then a plain one.
			"Hebrews": {1: {verse("Hebrews", 1, 6, "Let all the fixture messengers bow to Him.",
				xr("Deut. 32:43, LXX, DSS; Ps. 97:7", NoteRef{"DEU.32.43", 0, 11}, NoteRef{"PSA.97.7", 23, 31}))}},
		},
		Superscriptions: map[string]map[int]Superscription{"Psalms": {
			3: {
				Text:      "A Psalm of David when he fled from Absalom his son.",
				Footnotes: []Footnote{{Anchor: 0, Text: "2 Sam. 15:13–17", Kind: x, Caller: "-", Refs: []NoteRef{{"2SA.15.13-2SA.15.17", 0, 15}}}},
			},
			59: {Text: "A fixture song of David, when the watchers were set at his house."},
		}},
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
		rows[1].Targets[0].Start != 0 || rows[1].Targets[0].End != 9 || rows[1].Targets[0].Cut != 9 {
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
	// A plain citation carries no tag and no basis.
	if tg := rows[1].Targets[0]; tg.Title || tg.Chapters || tg.Basis != "" || tg.tag() != "" || tg.label() != "John 7:50" {
		t.Errorf("a plain citation resolved as %+v", tg)
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
	if len(rows[0].Targets) != 1 || rows[0].Targets[0].Ref.label() != "2 Samuel 15:13–17" || rows[0].Targets[0].Current {
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

// A citation of whole chapters is a card for them, labelled by chapter. The
// NKJV tags 117 of its citations by chapter alone ("LEV.11", "2KI.18-2KI.20"),
// and the parser, which needs a verse, left every one as words.
func TestPublisherCrossRefsLinkWholeChapters(t *testing.T) {
	for _, tc := range []struct {
		id       string
		book     string
		from, to int
		ok       bool
	}{
		{"LEV.11", "Leviticus", 11, 11, true},
		{"2KI.18-2KI.20", "2 Kings", 18, 20, true},
		{"PSA.22", "Psalms", 22, 22, true},
		{"PSA.22-PSA.21", "Psalms", 22, 22, true}, // backwards: the first chapter alone
		{"JHN.7.50", "", 0, 0, false},             // a verse id is parseUSFMRefID's
		{"ZZZ.1", "", 0, 0, false},
		{"JHN.0", "", 0, 0, false},
	} {
		book, from, to, ok := parseUSFMChapterID(tc.id)
		if ok != tc.ok || book != tc.book || from != tc.from || to != tc.to {
			t.Errorf("parseUSFMChapterID(%q) = %q %d-%d %v, want %q %d-%d %v",
				tc.id, book, from, to, ok, tc.book, tc.from, tc.to, tc.ok)
		}
	}

	bd := publisherFixture()
	v := &bd.Verses["John"][3][1]
	v.Footnotes[0].Refs = []NoteRef{{"JHN.7", 0, 9}}
	rows := publisherCrossRefsFor(bd, "John", 3, bd.GetChapter("John", 3)[1:2])
	if len(rows) != 2 || len(rows[0].Targets) != 1 {
		t.Fatalf("a whole-chapter citation did not become a target: %+v", rows)
	}
	if got := rows[0].Targets[0]; got.Ref != (crossRef{Book: "John", Chapter: 7, Verse: 50}) || !got.Chapters || got.Title || got.label() != "John 7" {
		t.Errorf("John 7 resolved to %+v, want the chapter's verses as loaded (7:50 alone), labelled by chapter", got)
	}
	// Genesis 7:2's "Lev. 11": the chapter's first to last verse, one card.
	rows = publisherCrossRefsFor(bd, "Genesis", 7, bd.GetChapter("Genesis", 7))
	if len(rows) != 1 || len(rows[0].Targets) != 1 {
		t.Fatalf("Genesis 7:2 resolved to %+v", rows)
	}
	if tg := rows[0].Targets[0]; tg.Ref != (crossRef{Book: "Leviticus", Chapter: 11, Verse: 1, EndV: 2}) || tg.label() != "Leviticus 11" || tg.tag() != "" {
		t.Errorf("Lev. 11 resolved to %+v (label %q, tag %q)", tg.Ref, tg.label(), tg.tag())
	}
	// A run of chapters is labelled as one: "2 Kings 18–20".
	v.Footnotes[0].Refs = []NoteRef{{"2KI.18-2KI.20", 0, 9}}
	rows = publisherCrossRefsFor(bd, "John", 3, bd.GetChapter("John", 3)[1:2])
	if tg := rows[0].Targets[0]; tg.label() != "2 Kings 18–20" || tg.Ref.EndCh != 20 || tg.Ref.EndV != 2 {
		t.Errorf("2 Kings 18–20 resolved to %+v, labelled %q", tg.Ref, tg.label())
	}
	// CONTROL: a chapter the text does not have stays words.
	v.Footnotes[0].Refs = []NoteRef{{"JHN.8", 0, 9}}
	if rows := publisherCrossRefsFor(bd, "John", 3, bd.GetChapter("John", 3)[1:2]); len(rows[0].Targets) != 0 {
		t.Errorf("a chapter the text lacks became a target: %+v", rows[0].Targets)
	}
}

// A CITATION OF A PSALM'S TITLE IS THE TITLE, NOT THE PSALM. The feed tags
// "Ps. 59:title" as the whole chapter PSA.59 and sets ":title" after it in
// words; the resolver used to read that as the whole psalm, so the card would
// have washed fifty verses for a superscription. Now the target is the
// chapter's first verse, where the title stands, labelled by chapter, tagged
// "Title", and the ":title" goes with it when the card is cut out.
func TestATitleCitationResolvesToTheTitle(t *testing.T) {
	bd := publisherFixture()
	rows := publisherCrossRefsFor(bd, "1 Samuel", 19, bd.GetChapter("1 Samuel", 19))
	if len(rows) != 1 || len(rows[0].Targets) != 1 {
		t.Fatalf("1 Samuel 19:12 resolved to %+v; want one target (Judges is not in the fixture)", rows)
	}
	tg := rows[0].Targets[0]
	if !tg.Title || tg.Chapters || tg.Ref != (crossRef{Book: "Psalms", Chapter: 59, Verse: 1}) {
		t.Errorf("the title citation resolved to %+v", tg)
	}
	if tg.Start != 12 || tg.End != 18 || tg.Cut != 24 {
		t.Errorf("the title citation spans %d..%d, cut %d; want 12..18 cut 24 (the words, then \":title\")", tg.Start, tg.End, tg.Cut)
	}
	if tg.label() != "Psalms 59" || tg.tag() != "Title" {
		t.Errorf("label %q tag %q, want Psalms 59 / Title", tg.label(), tg.tag())
	}
	// CONTROL: the same id without the suffix is the whole chapter.
	v := &bd.Verses["1 Samuel"][19][0]
	v.Footnotes[0].Text = "Judg. 16:2; Ps. 59"
	rows = publisherCrossRefsFor(bd, "1 Samuel", 19, bd.GetChapter("1 Samuel", 19))
	if tg := rows[0].Targets[0]; tg.Title || !tg.Chapters || tg.Ref.EndV != 2 || tg.Cut != 18 || tg.tag() != "" {
		t.Errorf("without \":title\" the citation resolved to %+v", tg)
	}
}

// THE BASIS RIDES ITS CITATION. The three citations the edition qualifies by
// the ancient text they rest on — ", LXX" and ", LXX, DSS", the feed's xta
// spans — are read off the words after the citation and kept as its tag, and
// the card's cut runs over them, so they never stand in a caption as words
// nothing explains.
func TestABasisRidesItsCitation(t *testing.T) {
	bd := publisherFixture()
	rows := publisherCrossRefsFor(bd, "Hebrews", 1, bd.GetChapter("Hebrews", 1))
	if len(rows) != 1 || len(rows[0].Targets) != 2 {
		t.Fatalf("Hebrews 1:6 resolved to %+v; want two targets", rows)
	}
	first, second := rows[0].Targets[0], rows[0].Targets[1]
	if first.Basis != "LXX, DSS" || first.tag() != "LXX, DSS" || first.End != 11 || first.Cut != 21 {
		t.Errorf("the qualified citation = %+v; want basis LXX, DSS over 11..21", first)
	}
	if second.Basis != "" || second.Cut != second.End {
		t.Errorf("CONTROL: the plain citation after it gained a basis: %+v", second)
	}
	for _, tc := range []struct {
		text  string
		end   int
		basis string
		cut   int
	}{
		{"Job 13:16, LXX", 9, "LXX", 14},
		{"Job 13:16, LXX; Ps. 1:1", 9, "LXX", 14},
		{"Deut. 32:43, LXX, DSS; Ps. 97:7", 11, "LXX, DSS", 21},
		{"Ps. 45:4, 5, LXX", 11, "LXX", 16},
		{"Deut. 32:43, DSS, LXX", 11, "DSS, LXX", 21}, // in the feed's order, whichever that is
		{"Job 13:16; LXX", 9, "", 9},                  // only ", <token>" is the apparatus's form
		{"Job 13:16, LXXX", 9, "", 9},                 // a word that merely begins with a token
		{"Job 13:16, 17", 9, "", 9},                   // a continuation is not a basis
	} {
		basis, cut := basisAfter([]rune(tc.text), tc.end)
		if basis != tc.basis || cut != tc.cut {
			t.Errorf("basisAfter(%q, %d) = %q, %d; want %q, %d", tc.text, tc.end, basis, cut, tc.basis, tc.cut)
		}
	}
}

// THE SPANS INDEX THE TEXT THE ROW STORES. The decoder trims a note before it
// takes the spans, and the row stores the trimmed text, which split cuts by
// them; the resolver used to read the spans against the text as delivered,
// so a note with leading whitespace had its ":title" and basis read from the
// wrong offset.
func TestNoteSpansIndexTheStoredText(t *testing.T) {
	bd := publisherFixture()
	note := resolvedNote(t, bd, "1 Samuel", 19, 12, "  Ps. 59:title", NoteRef{"PSA.59", 0, 6})
	if note.Text != "Ps. 59:title" {
		t.Fatalf("the row stores %q, want the trimmed text", note.Text)
	}
	if len(note.Targets) != 1 || !note.Targets[0].Title || note.Targets[0].Cut != 12 {
		t.Errorf("against the stored text the citation is the title, cut at 12; got %+v", note.Targets)
	}
	if cards, remainder := note.split(); len(cards) != 1 || remainder != "" {
		t.Errorf("split = %d cards, remainder %q; want one card and nothing left", len(cards), remainder)
	}
	// CONTROL: the same note without the whitespace resolves to the same target.
	plain := resolvedNote(t, bd, "1 Samuel", 19, 12, "Ps. 59:title", NoteRef{"PSA.59", 0, 6})
	if len(plain.Targets) != 1 || plain.Targets[0] != note.Targets[0] {
		t.Errorf("the note resolved differently with and without leading whitespace: %+v / %+v", plain.Targets, note.Targets)
	}
}

// resolvedNote resolves one note's text and spans against the fixture, as a
// note on book ch:verse, and returns it as the panel receives it.
func resolvedNote(t *testing.T, bd *BibleData, book string, ch, verse int, text string, refs ...NoteRef) publisherCrossRef {
	t.Helper()
	v := Verse{BookName: book, Book: book, Chapter: ch, Verse: verse, Text: "Invented words of the fixture.",
		Footnotes: []Footnote{{Text: text, Kind: footnoteKindCrossref, Caller: "-", Refs: refs}}}
	rows := publisherCrossRefsFor(bd, book, ch, []Verse{v})
	if len(rows) != 1 {
		t.Fatalf("%q resolved to %d notes, want 1", text, len(rows))
	}
	return rows[0]
}

func cardLabels(cards []publisherTarget) []string {
	var out []string
	for _, c := range cards {
		out = append(out, c.label())
	}
	return out
}

// SPLIT: the cards and the words. Each shape the apparatus takes, with the
// remainder it leaves, and a control for each rule the split rests on.
func TestSplitDividesANoteIntoCardsAndRemainder(t *testing.T) {
	bd := publisherFixture()
	for _, tc := range []struct {
		name            string
		book            string
		ch, verse       int
		text            string
		refs            []NoteRef
		cards           []string
		remainder       string
		tags            []string // per card, "" for none
		titleOfLastCard bool
	}{
		{"John 3:16: three cards and the group",
			"John", 3, 16, "Rom. 5:8; Eph. 2:4; 2 Thess. 2:16; (1 John 4:9, 10; Rev. 1:5)",
			[]NoteRef{{"ROM.5.8", 0, 8}, {"EPH.2.4", 10, 18}, {"2TH.2.16", 20, 33}},
			[]string{"Romans 5:8", "Ephesians 2:4", "2 Thessalonians 2:16"}, "(1 John 4:9, 10; Rev. 1:5)", []string{"", "", ""}, false},
		{"John 3:2: a continuation the text lacks stays words",
			"John", 3, 2, "John 7:50; 19:39",
			[]NoteRef{{"JHN.7.50", 0, 9}, {"JHN.19.39", 11, 16}},
			[]string{"John 7:50"}, "19:39", []string{""}, false},
		{"a note whose citations are all cards has no remainder",
			"John", 3, 2, "John 7:50; Gal. 6:15",
			[]NoteRef{{"JHN.7.50", 0, 9}, {"GAL.6.15", 11, 20}},
			[]string{"John 7:50", "Galatians 6:15"}, "", []string{"", ""}, false},
		{"the mixed group: no card for the tagged span inside it, the group whole",
			"John", 3, 2, "(John 1:13; Gal. 6:15; 1 John 3:9)",
			[]NoteRef{{"GAL.6.15", 12, 21}},
			nil, "(John 1:13; Gal. 6:15; 1 John 3:9)", nil, false},
		{"CONTROL: the same citations without the parentheses card the tagged one",
			"John", 3, 2, "John 1:13; Gal. 6:15; 1 John 3:9",
			[]NoteRef{{"GAL.6.15", 11, 20}},
			[]string{"Galatians 6:15"}, "John 1:13; 1 John 3:9", []string{""}, false},
		{"a card before a group keeps the group as printed",
			"John", 3, 16, "Rom. 5:8; (John 1:13; Gal. 6:15)",
			[]NoteRef{{"ROM.5.8", 0, 8}, {"GAL.6.15", 22, 31}},
			[]string{"Romans 5:8"}, "(John 1:13; Gal. 6:15)", []string{""}, false},
		{"a title citation: the chapter, tagged Title, its suffix cut with it",
			"1 Samuel", 19, 12, "Judg. 16:2; Ps. 59:title",
			[]NoteRef{{"JDG.16.2", 0, 10}, {"PSA.59", 12, 18}},
			[]string{"Psalms 59"}, "Judg. 16:2", []string{"Title"}, true},
		{"a basis: tagged on its card, cut with it",
			"Hebrews", 1, 6, "Job 13:16, LXX",
			[]NoteRef{{"JOB.13.16", 0, 9}},
			[]string{"Job 13:16"}, "", []string{"LXX"}, false},
		{"two bases, then a plain citation",
			"Hebrews", 1, 6, "Deut. 32:43, LXX, DSS; Ps. 97:7",
			[]NoteRef{{"DEU.32.43", 0, 11}, {"PSA.97.7", 23, 31}},
			[]string{"Deuteronomy 32:43", "Psalms 97:7"}, "", []string{"LXX, DSS", ""}, false},
		{"a basis after a continuation rides the continuation's card",
			"Hebrews", 1, 6, "Ps. 45:4, 5, LXX",
			[]NoteRef{{"PSA.45.4", 0, 8}, {"PSA.45.5", 10, 11}},
			[]string{"Psalms 45:4", "Psalms 45:5"}, "", []string{"", "LXX"}, false},
		{"a whole-chapter citation",
			"Genesis", 7, 2, "Lev. 11",
			[]NoteRef{{"LEV.11", 0, 7}},
			[]string{"Leviticus 11"}, "", []string{""}, false},
		{"a pericope note on its opening verse: the self-range is words, the rest cards",
			"Matthew", 3, 1, "Matt. 3:1–12; Mark 1:3–8",
			[]NoteRef{{"MAT.3.1-MAT.3.12", 0, 12}, {"MRK.1.3-MRK.1.8", 14, 24}},
			[]string{"Mark 1:3–8"}, "Matt. 3:1–12", []string{""}, false},
		{"a wholly untagged note is its own remainder",
			"John", 3, 1, "(Acts 10:38)", nil, nil, "(Acts 10:38)", nil, false},
		{"a note with no colon", "John", 3, 1, "(Jude 6)", nil, nil, "(Jude 6)", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			note := resolvedNote(t, bd, tc.book, tc.ch, tc.verse, tc.text, tc.refs...)
			cards, remainder := note.split()
			if got := cardLabels(cards); strings.Join(got, "|") != strings.Join(tc.cards, "|") {
				t.Errorf("cards = %q, want %q", got, tc.cards)
			}
			if remainder != tc.remainder {
				t.Errorf("remainder = %q, want %q", remainder, tc.remainder)
			}
			for i, c := range cards {
				if i < len(tc.tags) && c.tag() != tc.tags[i] {
					t.Errorf("card %d tag = %q, want %q", i, c.tag(), tc.tags[i])
				}
			}
			if len(cards) > 0 && cards[len(cards)-1].Title != tc.titleOfLastCard {
				t.Errorf("last card Title = %v, want %v", cards[len(cards)-1].Title, tc.titleOfLastCard)
			}
			// The remainder is made of the note's own characters: every
			// piece of it stands in the text, in order.
			at := 0
			for _, piece := range strings.Split(remainder, "; ") {
				i := strings.Index(tc.text[at:], piece)
				if piece == "" || i < 0 {
					if remainder != "" {
						t.Errorf("remainder piece %q is not in the note %q after %d", piece, tc.text, at)
					}
					continue
				}
				at += i + len(piece)
			}
		})
	}
	// Parenthesis depth, which the card rule rests on.
	for _, tc := range []struct {
		text string
		at   int
		want int
	}{
		{"(John 1:13; Gal. 6:15)", 0, 0}, {"(John 1:13; Gal. 6:15)", 1, 1}, {"(John 1:13; Gal. 6:15)", 12, 1},
		{"Rom. 5:8; (John 1:13)", 0, 0}, {"Rom. 5:8; (John 1:13)", 11, 1}, {"a) b", 3, 0}, {"((a) b", 5, 1},
	} {
		if got := parenDepthAt([]rune(tc.text), tc.at); got != tc.want {
			t.Errorf("parenDepthAt(%q, %d) = %d, want %d", tc.text, tc.at, got, tc.want)
		}
	}
}
