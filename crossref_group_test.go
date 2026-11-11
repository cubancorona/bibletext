package bibletext

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// noteRefAt is the tag the feed would put on piece where it stands in text.
func noteRefAt(t *testing.T, text, piece, id string) NoteRef {
	t.Helper()
	at := strings.Index(text, piece)
	if at < 0 {
		t.Fatalf("%q is not in %q", piece, text)
	}
	start := len([]rune(text[:at]))
	return NoteRef{id, start, start + len([]rune(piece))}
}

// A GROUP STAYS WHERE THE EDITION PRINTS IT, and its citations are read from
// the print: the feed tags few of them.
func TestANotesGroupsStandWhereTheEditionPrintsThem(t *testing.T) {
	bd := xrefBible(map[string]map[int]int{
		"Acts": {10: 48}, "Matthew": {5: 48, 11: 30, 15: 39}, "Mark": {7: 37}, "Romans": {14: 23},
		"1 Corinthians": {10: 33}, "1 Timothy": {4: 16}, "Titus": {1: 16}, "Luke": {6: 49}, "James": {2: 26},
	})

	// Three groups among two direct citations, none of the groups tagged.
	text := "(Matt. 15:11; Mark 7:19); Acts 10:28; (Rom. 14:14); 1 Cor. 10:25; (1 Tim. 4:4; Titus 1:15)"
	note := resolvedNote(t, bd, "Acts", 10, 15, text,
		noteRefAt(t, text, "Acts 10:28", "ACT.10.28"), noteRefAt(t, text, "1 Cor. 10:25", "1CO.10.25"))
	got := pieceNames(note.Pieces)
	want := []string{
		"group:(Matt. 15:11; Mark 7:19)[card:Matthew 15:11 card:Mark 7:19]",
		"card:Acts 10:28",
		"group:(Rom. 14:14)[card:Romans 14:14]",
		"card:1 Corinthians 10:25",
		"group:(1 Tim. 4:4; Titus 1:15)[card:1 Timothy 4:4 card:Titus 1:15]",
	}
	if joined(got) != joined(want) {
		t.Errorf("pieces = %q\nwant  %q", got, want)
	}
	// CONTROL: the census's split still folds the same three groups into one
	// remainder after the cards, which is the order the list used to show.
	cards, remainder := note.split()
	if joined(cardLabels(cards)) != "Acts 10:28|1 Corinthians 10:25" || remainder != "(Matt. 15:11; Mark 7:19); (Rom. 14:14); (1 Tim. 4:4; Titus 1:15)" {
		t.Errorf("control: split = %q / %q", cardLabels(cards), remainder)
	}

	// The book carried into a group from the citation before it, and the
	// note's own passage in its place, outside every group.
	text = "Matt. 5:3–12; (11:5); Luke 6:20–23; (James 2:5)"
	note = resolvedNote(t, bd, "Luke", 6, 20, text,
		noteRefAt(t, text, "Matt. 5:3–12", "MAT.5.3-MAT.5.12"), noteRefAt(t, text, "Luke 6:20–23", "LUK.6.20-LUK.6.23"))
	got = pieceNames(note.Pieces)
	want = []string{"card:Matthew 5:3–12", "group:(11:5)[card:Matthew 11:5]", "self:Luke 6:20–23", "group:(James 2:5)[card:James 2:5]"}
	if joined(got) != joined(want) {
		t.Errorf("pieces = %q\nwant  %q", got, want)
	}

	// A comma's verses are one card each, as the direct citations are, and a
	// citation whose passage the loaded text lacks is its words, in its
	// place among the group's pieces.
	text = "(Rom. 14:14, 15; Jude 6)"
	note = resolvedNote(t, bd, "Acts", 10, 15, text)
	if got := pieceNames(note.Pieces); joined(got) != "group:(Rom. 14:14, 15; Jude 6)[card:Romans 14:14 card:Romans 14:15 words:Jude 6]" {
		t.Errorf("pieces = %q", got)
	}
}

// groupListState is a list fixture shaped like John 3:16: one note of three
// direct citations and a group, a second note that is a group alone, and a
// third that is a group nothing of which the fixture's text has.
func groupListState(t *testing.T) (*AppState, []Verse) {
	t.Helper()
	bd := xrefBible(map[string]map[int]int{
		"John": {3: 36}, "Romans": {5: 21}, "Ephesians": {2: 22}, "2 Thessalonians": {2: 17},
		"1 John": {4: 21}, "Revelation": {1: 20}, "Isaiah": {9: 21},
	})
	first := "Rom. 5:8; Eph. 2:4; 2 Thess. 2:16; (1 John 4:9, 10; Rev. 1:5)"
	v := &bd.Verses["John"][3][15]
	v.Footnotes = []Footnote{
		{Anchor: 0, Text: first, Kind: footnoteKindCrossref, Caller: "-", Refs: []NoteRef{
			noteRefAt(t, first, "Rom. 5:8", "ROM.5.8"), noteRefAt(t, first, "Eph. 2:4", "EPH.2.4"),
			noteRefAt(t, first, "2 Thess. 2:16", "2TH.2.16")}},
		{Anchor: 20, Text: "(Is. 9:6)", Kind: footnoteKindCrossref, Caller: "-"},
		{Anchor: 30, Text: "(Hab. 2:4)", Kind: footnoteKindCrossref, Caller: "-"}, // a book the fixture lacks
	}
	st := &AppState{Bible: bd, CurrentBook: "John", CurrentChapter: 3, CurrentVersion: "nkjv"}
	return st, []Verse{*v}
}

// THE ROW: closed until tapped, its citations the list's ordinary cards when
// open, a rule between it and the row above it, none under it as the last
// of its note's rows, and the list told to lay itself out again.
func TestAGroupRowOpensIntoItsCitations(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	licenseNKJVForTest(t)
	withPublisherCrossRefs(t, "nkjv", true)
	st, sel := groupListState(t)
	var followed []crossRef
	lst := buildCrossRefList(st, sel, nil, nil, lightPalette, func(c crossRef) { followed = append(followed, c) })
	relaid := 0
	lst.Relayout.fn = func() { relaid++ }

	if lst.PublisherNotes != 3 || lst.PublisherCards != 3 || lst.PublisherGroups != 3 || lst.PublisherLines != 0 {
		t.Fatalf("list = %+v; want three notes showing three cards and holding three groups, and nothing at a note's own level that is not followed", lst)
	}
	// The cards on screen as the list is built: the first note's three direct
	// citations. Both the group inside the first note and the second note,
	// which is one group, start closed.
	if got := cardLabelsIn(lst.Objects); joined(got) != "Romans 5:8|Ephesians 2:4|2 Thessalonians 2:16" {
		t.Fatalf("cards = %q", got)
	}
	groups := groupRowsIn(lst.Objects)
	if len(groups) != 2 {
		t.Fatalf("%d group rows, want the one inside the first note and the second note's", len(groups))
	}
	first, lone := groups[0], groups[1]

	// Order: the passage and the label, then the three notes, each on a
	// ground of its own. The first is a row whose pieces are its three cards
	// and then its group, where the edition printed it; the second is a
	// group that is the whole of its note, drawn as that group's row beside
	// its letter; the third is a group nothing of which can be followed. A
	// rule closes the card above the first group, which is the rule above it;
	// neither group closes with a rule of its own, since each ends at its
	// note's ground.
	notes := noteRowsIn(lst.Objects)
	if len(notes) != 1 {
		t.Fatalf("%d note rows, want the first note's", len(notes))
	}
	note := notes[0]
	n := len(lst.Objects)
	if n < 3 {
		t.Fatalf("the list has %d objects", n)
	}
	for i, o := range lst.Objects[n-3:] {
		c, ok := o.(*fyne.Container)
		if !ok {
			t.Fatalf("note %d is a %T, want a ground with the note on it", i, o)
		}
		if _, section := c.Layout.(crossRefSectionLayout); !section {
			t.Fatalf("note %d does not stand on a ground of its own", i)
		}
	}
	unread := lst.Objects[n-1]
	if got := noteRowsIn(lst.Objects[n-3 : n-2]); len(got) != 1 || got[0] != note {
		t.Fatalf("the first of the last three grounds does not hold the first note's row")
	}
	if got := groupRowsIn(lst.Objects[n-2 : n-1]); len(got) != 1 || got[0] != lone || len(noteRowsIn(lst.Objects[n-2:n-1])) != 0 {
		t.Fatalf("the second of the last three grounds does not hold the lone group's row")
	}
	body := note.pieces
	at := len(body) - 1
	if len(body) != 4 || body[at] != fyne.CanvasObject(first) {
		t.Fatalf("the first note's pieces are %d objects; want its three cards and then its group", len(body))
	}
	above, ok := body[at-1].(*fyne.Container)
	if !ok {
		t.Fatalf("the row above the group is a %T", body[at-1])
	}
	if _, rule := above.Objects[len(above.Objects)-1].(*widget.Separator); !rule {
		t.Errorf("no rule above the first group: the block above ends in a %T", above.Objects[len(above.Objects)-1])
	}
	for _, g := range []*crossRefGroupRow{first, lone} {
		if _, rule := g.box.Objects[len(g.box.Objects)-1].(*widget.Separator); rule {
			t.Errorf("%s: closes with a rule of its own at the foot of its note's ground", g.group.Text)
		}
	}
	// The card above carries no caption now: the group is not its footnote.
	if n := len(above.Objects); n != 2 {
		t.Errorf("the card above the group is a block of %d objects; want the card and its rule", n)
	}

	// Closed: the printed text and nothing under it.
	if first.open || first.inset.Visible() || len(first.members.Objects) != 0 {
		t.Fatalf("the group is not closed when the list is built: open %v, inset visible %v, %d member cards", first.open, first.inset.Visible(), len(first.members.Objects))
	}
	if got := textsIn(first); joined(got) != "(1 John 4:9, 10; Rev. 1:5)" {
		t.Errorf("closed, the row shows %q", got)
	}
	closedHeight := first.MinSize().Height

	// Open: its citations in print order, as cards, on the shaded ground.
	test.Tap(first.header)
	if !first.open || !first.inset.Visible() {
		t.Fatalf("a tap did not open the group")
	}
	members := cardsIn(first.members.Objects)
	if got := cardLabelsIn(first.members.Objects); joined(got) != "1 John 4:9|1 John 4:10|Revelation 1:5" {
		t.Fatalf("open, the group's cards are %q", got)
	}
	if first.shade.FillColor != crossRefGroupGround(lightPalette) || crossRefGroupGround(lightPalette) == crossRefSectionGround(lightPalette) {
		t.Errorf("the group's ground is %v; want %v, which must differ from the ground of the note it stands in, %v", first.shade.FillColor, crossRefGroupGround(lightPalette), crossRefSectionGround(lightPalette))
	}
	if first.MinSize().Height <= closedHeight {
		t.Errorf("open, the row is %v tall, no taller than closed (%v)", first.MinSize().Height, closedHeight)
	}
	if relaid != 1 {
		t.Errorf("opening asked for %d layouts of the list, want 1", relaid)
	}
	// The last member has no rule of its own; the others do.
	for i, o := range first.members.Objects {
		block := o.(*fyne.Container)
		_, rule := block.Objects[len(block.Objects)-1].(*widget.Separator)
		if rule != (i < len(first.members.Objects)-1) {
			t.Errorf("member %d: rule %v", i, rule)
		}
	}
	// A member is followed as any card is.
	test.Tap(members[1])
	if len(followed) != 1 || followed[0] != (crossRef{Book: "1 John", Chapter: 4, Verse: 10}) {
		t.Errorf("tapping the group's second card followed %+v", followed)
	}
	// The list's own run is what it was: the group's cards are not its cards.
	if got := cardLabelsIn(lst.Objects); joined(got) != "Romans 5:8|Ephesians 2:4|2 Thessalonians 2:16" {
		t.Errorf("with the group open the list's cards are %q", got)
	}

	// Closed again.
	test.Tap(first.header)
	if first.open || first.inset.Visible() || first.MinSize().Height != closedHeight || relaid != 2 {
		t.Errorf("a second tap did not close the group: open %v, inset %v, height %v (closed %v), %d layouts", first.open, first.inset.Visible(), first.MinSize().Height, closedHeight, relaid)
	}

	// A note that is a group alone is drawn as that group's row beside its
	// letter: closed when the list is built, and opened onto the group's
	// shaded ground by a tap on the row.
	if lone.letter == nil || lone.letter.Text+"  "+joined(textsIn(lone.label)) != "b  (Is. 9:6)" {
		t.Fatalf("the lone group's row has letter %v and label %q", lone.letter, textsIn(lone.label))
	}
	if lone.open || lone.inset.Visible() || lone.stem == nil || lone.stem.Visible() || len(lone.members.Objects) != 0 {
		t.Fatalf("the lone group is not closed when the list is built: open %v, inset %v, stem %v, %d member cards", lone.open, lone.inset.Visible(), lone.stem != nil && lone.stem.Visible(), len(lone.members.Objects))
	}
	test.Tap(lone.header)
	if got := cardLabelsIn(lone.members.Objects); !lone.open || !lone.inset.Visible() || !lone.stem.Visible() || joined(got) != "Isaiah 9:6" || lone.shade.FillColor != crossRefGroupGround(lightPalette) || relaid != 3 {
		t.Errorf("the lone group open: open %v, inset %v, stem %v, cards %q, ground %v, %d layouts", lone.open, lone.inset.Visible(), lone.stem.Visible(), got, lone.shade.FillColor, relaid)
	}
	test.Tap(lone.header)
	if lone.open || lone.inset.Visible() || lone.stem.Visible() || relaid != 4 {
		t.Errorf("a second tap did not close the lone group: open %v, inset %v, stem %v, %d layouts", lone.open, lone.inset.Visible(), lone.stem.Visible(), relaid)
	}
	// CONTROL: a group none of whose citations the loaded text has is its
	// letter and its words and no more: nothing to tap, no row promising
	// citations.
	if got := joined(textsIn(unread)); got != "c|(Hab. 2:4)" {
		t.Errorf("a group with nothing to open reads %q", got)
	}
	if len(cardsIn([]fyne.CanvasObject{unread})) != 0 || len(noteRowsIn([]fyne.CanvasObject{unread})) != 0 || len(groupRowsIn([]fyne.CanvasObject{unread})) != 0 {
		t.Errorf("a group with nothing to open holds a tap target or a row that folds")
	}
}
