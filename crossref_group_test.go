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

// partsOf names a note's parts in order: a card by its label, a group by its
// printed text and its members' labels.
func partsOf(n publisherCrossRef) (out []string, words string) {
	parts, words := n.parts()
	for _, p := range parts {
		if p.Card != nil {
			out = append(out, p.Card.label())
			continue
		}
		out = append(out, p.Group.Text+" = "+strings.Join(cardLabels(p.Group.cards()), ", "))
	}
	return out, words
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
	got, words := partsOf(note)
	want := []string{
		"(Matt. 15:11; Mark 7:19) = Matthew 15:11, Mark 7:19",
		"Acts 10:28",
		"(Rom. 14:14) = Romans 14:14",
		"1 Corinthians 10:25",
		"(1 Tim. 4:4; Titus 1:15) = 1 Timothy 4:4, Titus 1:15",
	}
	if joined(got) != joined(want) || words != "" {
		t.Errorf("parts = %q, words %q\nwant  %q and no words", got, words, want)
	}
	// CONTROL: the census's split still folds the same three groups into one
	// remainder after the cards, which is the order the list used to show.
	cards, remainder := note.split()
	if joined(cardLabels(cards)) != "Acts 10:28|1 Corinthians 10:25" || remainder != "(Matt. 15:11; Mark 7:19); (Rom. 14:14); (1 Tim. 4:4; Titus 1:15)" {
		t.Errorf("control: split = %q / %q", cardLabels(cards), remainder)
	}

	// The book carried into a group from the citation before it, and the
	// note's own passage left as words, outside every group.
	text = "Matt. 5:3–12; (11:5); Luke 6:20–23; (James 2:5)"
	note = resolvedNote(t, bd, "Luke", 6, 20, text,
		noteRefAt(t, text, "Matt. 5:3–12", "MAT.5.3-MAT.5.12"), noteRefAt(t, text, "Luke 6:20–23", "LUK.6.20-LUK.6.23"))
	got, words = partsOf(note)
	want = []string{"Matthew 5:3–12", "(11:5) = Matthew 11:5", "(James 2:5) = James 2:5"}
	if joined(got) != joined(want) || words != "Luke 6:20–23" {
		t.Errorf("parts = %q, words %q\nwant  %q and the note's own passage as words", got, words, want)
	}

	// A comma's verses are one card each, as the direct citations are, and a
	// citation whose passage the loaded text lacks is no member: its words
	// are still the group's.
	text = "(Rom. 14:14, 15; Jude 6)"
	note = resolvedNote(t, bd, "Acts", 10, 15, text)
	got, words = partsOf(note)
	if joined(got) != "(Rom. 14:14, 15; Jude 6) = Romans 14:14, Romans 14:15" || words != "" {
		t.Errorf("parts = %q, words %q", got, words)
	}
}

// groupListState is a list fixture shaped like John 3:16: one note of three
// direct citations and a group, and a second note that is a group alone.
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
// open, a rule above and below it, and the list told to lay itself out again.
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

	if lst.PublisherCards != 3 || lst.PublisherGroups != 3 || lst.PublisherLines != 0 {
		t.Fatalf("list = %+v; want three cards, three groups and no caption", lst)
	}
	if got := cardLabelsIn(lst.Objects); joined(got) != "Romans 5:8|Ephesians 2:4|2 Thessalonians 2:16" {
		t.Fatalf("cards = %q", got)
	}
	groups := groupRowsIn(lst.Objects)
	if len(groups) != 3 {
		t.Fatalf("%d group rows", len(groups))
	}
	first, lone, unread := groups[0], groups[1], groups[2]

	// Order: the label, the three cards, then the groups, each a row of the
	// list itself; and a rule closes the block above the first group, which
	// is the rule above it.
	at := -1
	for i, o := range lst.Objects {
		if o == fyne.CanvasObject(first) {
			at = i
		}
	}
	if at != 4 || lst.Objects[5] != fyne.CanvasObject(lone) || lst.Objects[6] != fyne.CanvasObject(unread) {
		t.Fatalf("the first group is object %d of %d; want the label, three cards, then the three groups", at, len(lst.Objects))
	}
	above, ok := lst.Objects[at-1].(*fyne.Container)
	if !ok {
		t.Fatalf("the row above the group is a %T", lst.Objects[at-1])
	}
	if _, rule := above.Objects[len(above.Objects)-1].(*widget.Separator); !rule {
		t.Errorf("no rule above the first group: the block above ends in a %T", above.Objects[len(above.Objects)-1])
	}
	for _, g := range groups {
		if _, rule := g.box.Objects[len(g.box.Objects)-1].(*widget.Separator); !rule {
			t.Errorf("%s: no rule below the group", g.group.Text)
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
	if first.shade.FillColor != lightPalette.Surface || lightPalette.Surface == lightPalette.SurfaceAlt {
		t.Errorf("the group's ground is %v; want the inset surface %v, which must differ from the panel's %v", first.shade.FillColor, lightPalette.Surface, lightPalette.SurfaceAlt)
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

	// A note that is a group alone opens the same way.
	test.Tap(lone.header)
	if got := cardLabelsIn(lone.members.Objects); joined(got) != "Isaiah 9:6" {
		t.Errorf("the lone group's cards are %q", got)
	}
	// CONTROL: a group none of whose citations the loaded text has is its
	// words and no more: nothing to tap, no icon promising cards.
	if unread.header != nil || unread.icon != nil || joined(textsIn(unread)) != "(Hab. 2:4)" {
		t.Errorf("a group with nothing to open has a header %v, an icon %v and the text %q", unread.header != nil, unread.icon != nil, textsIn(unread))
	}
	if len(cardsIn([]fyne.CanvasObject{unread.box})) != 0 {
		t.Errorf("a group with nothing to open holds a tap target")
	}
}
