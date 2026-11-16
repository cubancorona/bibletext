package bibletext

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// The panel keeps publisher notes, Gospel episodes and Treasury origins in
// separate labelled sections. Shared destinations remain under every source
// that contributes them; publisher notes retain their own printed structure.

// withPublisherCrossRefs flips the registry flag for one edition for the
// test's duration, so both enabled and disabled paths have explicit controls.
func withPublisherCrossRefs(t *testing.T, id string, on bool) {
	t.Helper()
	for i := range registeredVersions {
		if registeredVersions[i].ID == id {
			wasRefs := registeredVersions[i].PublisherCrossRefs
			registeredVersions[i].PublisherCrossRefs = on
			t.Cleanup(func() {
				registeredVersions[i].PublisherCrossRefs = wasRefs
			})
			return
		}
	}
	t.Fatalf("no registered version %q", id)
}

// textsIn collects every visible string in a tree: canvas texts, labels,
// rich-text segments and what a card holds.
func textsIn(o fyne.CanvasObject) []string {
	var out []string
	var walk func(o fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		switch x := o.(type) {
		case *canvas.Text:
			out = append(out, x.Text)
		case *widget.Label:
			out = append(out, x.Text)
		case *widget.RichText:
			var b strings.Builder
			for _, s := range x.Segments {
				b.WriteString(s.Textual())
			}
			out = append(out, b.String())
		case *crossRefPrinted:
			out = append(out, x.Text)
		case *tapCard:
			walk(x.content)
		case *crossRefGroupRow:
			walk(x.box)
		case *crossRefNoteRow:
			walk(x.box)
		case *crossRefSourceGroup:
			walk(x.box)
		case *fyne.Container:
			for _, c := range x.Objects {
				walk(c)
			}
		}
	}
	walk(o)
	return out
}

func joinedTexts(objs []fyne.CanvasObject) string {
	var b strings.Builder
	for _, o := range objs {
		for _, s := range textsIn(o) {
			b.WriteString(s)
			b.WriteString("\n")
		}
	}
	return b.String()
}

// cardsIn is every card in objs, in list order: the cards of the list's own
// run, with those under an open note among them. A note's own row and a
// group's are not cards. Source groups contribute their mounted bodies;
// nested publisher groups remain separate from the list's direct cards.
func cardsIn(objs []fyne.CanvasObject) []*tapCard {
	var out []*tapCard
	var walk func(o fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		switch x := o.(type) {
		case *tapCard:
			out = append(out, x)
		case *crossRefNoteRow:
			walk(x.body)
		case *crossRefSourceGroup:
			walk(x.body)
		case *fyne.Container:
			for _, c := range x.Objects {
				walk(c)
			}
		}
	}
	for _, o := range objs {
		walk(o)
	}
	return out
}

// cardReference is a card's reference line: its bold upright text. A note's
// letter, which stands in the margin beside the reference of a card that is
// its whole note, is bold and italic.
func cardReference(c *tapCard) string {
	ref := ""
	walkTree(c.content, func(o fyne.CanvasObject) {
		if ref != "" {
			return
		}
		switch t := o.(type) {
		case *canvas.Text:
			if t.TextStyle.Bold && !t.TextStyle.Italic {
				ref = t.Text
			}
		case *crossRefPrinted:
			if t.style.Bold && !t.style.Italic {
				ref = t.Text
			}
		}
	})
	return ref
}

// cardLetter is the note's letter a card carries, "" for none.
func cardLetter(c *tapCard) string {
	letter := ""
	walkTree(c.content, func(o fyne.CanvasObject) {
		if t, ok := o.(*canvas.Text); ok && letter == "" && t.TextStyle.Bold && t.TextStyle.Italic {
			letter = t.Text
		}
	})
	return letter
}

// cardLabelsIn is the reference line of every card in objs, in list order.
func cardLabelsIn(objs []fyne.CanvasObject) []string {
	var out []string
	for _, c := range cardsIn(objs) {
		if ref := cardReference(c); ref != "" {
			out = append(out, ref)
		}
	}
	return out
}

// noteRowsIn is every note's own row in objs, in list order: the notes that
// are more than one piece.
func noteRowsIn(objs []fyne.CanvasObject) []*crossRefNoteRow {
	var out []*crossRefNoteRow
	var walk func(o fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		switch x := o.(type) {
		case *crossRefNoteRow:
			out = append(out, x)
		case *fyne.Container:
			for _, c := range x.Objects {
				walk(c)
			}
		}
	}
	for _, o := range objs {
		walk(o)
	}
	return out
}

// passageIn is the passage that heads objs, nil when the list has none.
func passageIn(objs []fyne.CanvasObject) *readingParagraph {
	var out *readingParagraph
	var walk func(o fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		switch x := o.(type) {
		case *readingParagraph:
			if out == nil {
				out = x
			}
		case *fyne.Container:
			for _, c := range x.Objects {
				walk(c)
			}
		}
	}
	for _, o := range objs {
		walk(o)
	}
	return out
}

// sectionLabelsIn is every section label in objs, in list order: the bold
// monospace texts, which nothing else in the list is.
func sectionLabelsIn(objs []fyne.CanvasObject) []string {
	var out []string
	var walk func(o fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		switch x := o.(type) {
		case *canvas.Text:
			if x.TextStyle.Monospace && x.TextStyle.Bold {
				out = append(out, x.Text)
			}
		case *fyne.Container:
			for _, c := range x.Objects {
				walk(c)
			}
		}
	}
	for _, o := range objs {
		walk(o)
	}
	return out
}

// crossRefListMisfits is everything in objs the list may not hold: an object
// whose MinSize is wider than limit, and a widget of a kind whose width
// cannot wrap (bannedIn). Every object is measured, not only the top level,
// so a wide child inside a block is named by its path. It stands in a file
// every build compiles: the render harness uses it as the width tests do.
func crossRefListMisfits(objs []fyne.CanvasObject, limit float32) []string {
	var out []string
	var walk func(o fyne.CanvasObject, path string)
	walk = func(o fyne.CanvasObject, path string) {
		if w := o.MinSize().Width; w > limit {
			out = append(out, fmt.Sprintf("%s %T wants %.0f points, over %.0f", path, o, w, limit))
		}
		switch x := o.(type) {
		case *tapCard:
			walk(x.content, path+"/card")
		case *crossRefGroupRow:
			walk(x.box, path+"/group")
		case *crossRefNoteRow:
			walk(x.box, path+"/note")
		case *crossRefSourceGroup:
			walk(x.box, path+"/source")
		case *fyne.Container:
			for i, c := range x.Objects {
				walk(c, path+"/"+strconv.Itoa(i))
			}
		}
	}
	for i, o := range objs {
		walk(o, strconv.Itoa(i))
	}
	for _, b := range bannedIn(objs) {
		out = append(out, "a "+b+" in the list")
	}
	return out
}

// bannedIn names every object in objs of a kind the list may not hold: the
// widgets whose width cannot wrap, which widened the scroll's content past
// the panel (crossref_list.go), and the override that clears the font cache.
func bannedIn(objs []fyne.CanvasObject) []string {
	var out []string
	var walk func(o fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		switch x := o.(type) {
		case *widget.Accordion:
			out = append(out, "Accordion")
		case *widget.Button:
			out = append(out, "Button")
		case *widget.Hyperlink:
			out = append(out, "Hyperlink")
		case *container.ThemeOverride:
			out = append(out, "ThemeOverride")
		case *widget.RichText:
			for _, s := range x.Segments {
				if _, ok := s.(*widget.HyperlinkSegment); ok {
					out = append(out, "HyperlinkSegment")
				}
			}
		case *tapCard:
			walk(x.content)
		case *crossRefGroupRow:
			walk(x.box)
		case *crossRefNoteRow:
			walk(x.box)
		case *crossRefSourceGroup:
			walk(x.box)
		case *fyne.Container:
			for _, c := range x.Objects {
				walk(c)
			}
		}
	}
	for _, o := range objs {
		walk(o)
	}
	return out
}

// groupRowsIn is every parenthesised group's row in objs, in list order.
// cardsIn does not look inside one: a group's label and the cards it opens
// into are the group's, not cards of the list's run.
func groupRowsIn(objs []fyne.CanvasObject) []*crossRefGroupRow {
	var out []*crossRefGroupRow
	var walk func(o fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		switch x := o.(type) {
		case *crossRefGroupRow:
			out = append(out, x)
		case *crossRefNoteRow:
			walk(x.body) // a group among a note's pieces
		case *crossRefSourceGroup:
			walk(x.body)
		case *fyne.Container:
			for _, c := range x.Objects {
				walk(c)
			}
		}
	}
	for _, o := range objs {
		walk(o)
	}
	return out
}

// licenseNKJVForTest makes the NKJV's licensed source available, as the
// integration tests do, so the edition counts as licensed and serving text.
func licenseNKJVForTest(t *testing.T) {
	t.Helper()
	t.Setenv("BIBLE_API_KEY", "test-key")
	t.Setenv("BIBLETEXT_LICENSE_NKJV", "1")
	t.Setenv("BIBLETEXT_PROVIDER_ID_NKJV", "test-bible")
}

func listFixtureState() (*AppState, []Verse) {
	bd := publisherFixture()
	st := &AppState{Bible: bd, CurrentBook: "John", CurrentChapter: 3, CurrentVersion: "nkjv"}
	return st, bd.GetChapter("John", 3)
}

// Treasury rows and a parallel row as crossRefsForSelection would hand them
// over: parallels first, then the ranked Treasury rows.
func listFixtureRefs() []crossRef {
	return []crossRef{
		{Book: "Matthew", Chapter: 3, Verse: 1, Parallel: true, Title: "A synopsis probe"},
		{Book: "John", Chapter: 7, Verse: 50, Votes: 40},
		{Book: "Galatians", Chapter: 6, Verse: 15, Votes: 12},
	}
}

func joined(ss []string) string { return strings.Join(ss, "|") }

// noteHeadsIn is each note row's own row as a reader reads it: the letter in
// its margin, then the note as printed.
func noteHeadsIn(objs []fyne.CanvasObject) []string {
	var out []string
	for _, n := range noteRowsIn(objs) {
		out = append(out, n.letter.Text+"  "+joined(textsIn(n.label)))
	}
	return out
}

// hasLine reports whether some text in objs is exactly s.
func hasLine(objs []fyne.CanvasObject, s string) bool {
	for _, o := range objs {
		for _, line := range textsIn(o) {
			if line == s {
				return true
			}
		}
	}
	return false
}

func TestCrossRefListWithoutThePublisherKeepsLabelledSources(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	licenseNKJVForTest(t)
	withPublisherCrossRefs(t, "nkjv", false)
	st, sel := listFixtureState()

	lst := buildCrossRefList(st, sel, listFixtureRefs(), nil, lightPalette, func(crossRef) {})
	if lst.PublisherCards != 0 || lst.PublisherLines != 0 || lst.Sections.Publisher {
		t.Errorf("the publisher's run appeared with the flag off: %+v", lst)
	}
	if lst.Parallels != 1 || lst.TSKRows != 2 {
		t.Errorf("list = %d parallels and %d Treasury rows; want 1 and 2", lst.Parallels, lst.TSKRows)
	}
	if got := cardLabelsIn(lst.Objects); joined(got) != "Matthew 3:1|John 7:50|Galatians 6:15" {
		t.Errorf("cards = %q", got)
	}
	if labels := sectionLabelsIn(lst.Objects); joined(labels) != parallelsLabel+"|"+treasuryLabel {
		t.Errorf("source section labels = %q", labels)
	}
	all := joinedTexts(lst.Objects)
	if hasLine(lst.Objects, "PARALLEL") || !hasLine(lst.Objects, "A synopsis probe") {
		t.Error("the Gospel source must identify its episode without repeating a per-card pill")
	}
	for _, banned := range []string{"REFERENCES", "Thomas Nelson", "John 7:50; 19:39", "(John 1:13", "No NKJV"} {
		if strings.Contains(all, banned) {
			t.Errorf("a publisher's note, label or credit reached the plain list (%q):\n%s", banned, all)
		}
	}
	// The footer names both displayed sources, and the licensed edition
	// retains its notice under the previews of its verse text.
	sources, notice := crossRefFooterCredit(st, lst.Sections)
	if sources != treasuryCredit+"\n"+parallelsCredit {
		t.Errorf("footer sources = %q", sources)
	}
	if !strings.Contains(notice, "Thomas Nelson") {
		t.Errorf("a licensed edition's previews are shown without its notice: %q", notice)
	}
	// Without a parallel row the synopsis is not named.
	lst = buildCrossRefList(st, sel, listFixtureRefs()[1:], nil, lightPalette, func(crossRef) {})
	if sources, _ := crossRefFooterCredit(st, lst.Sections); sources != treasuryCredit {
		t.Errorf("footer sources without a parallel row = %q, want the Treasury alone", sources)
	}
	// CONTROL: a public-domain edition carries no notice.
	st.CurrentVersion = "web"
	if _, notice := crossRefFooterCredit(st, lst.Sections); notice != "" {
		t.Errorf("a public-domain edition grew a notice: %q", notice)
	}
}

// The publisher's run is the edition's notes, each under its letter and whole.
func TestCrossRefListWithThePublisherIsItsNotesUnderTheirLetters(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	licenseNKJVForTest(t)
	withPublisherCrossRefs(t, "nkjv", true)
	st, sel := listFixtureState()
	var followed []crossRef
	// The Treasury's rows: one the publisher's open card carries, one inside
	// a closed lone group, and one neither carries.
	refs := append(listFixtureRefs(), crossRef{Book: "Romans", Chapter: 5, Verse: 8, Votes: 5})
	lst := buildCrossRefList(st, sel, refs, nil, lightPalette, func(c crossRef) { followed = append(followed, c) })

	// Shown and not followed at a note's own level: the continuation in
	// verse 2's first note, and verse 3's own passage. Verse 1's lone group
	// has nothing to follow and is drawn as its words, and still counts as a
	// group and never as a line.
	if lst.Parallels != 1 || lst.PublisherNotes != 4 || lst.PublisherCards != 1 || lst.PublisherGroups != 2 || lst.PublisherLines != 2 || lst.TSKRows != 3 {
		t.Fatalf("list = %+v; want one parallel, four notes showing one card, two groups and two lines that are not followed, and three Treasury cards", lst)
	}
	// The selected text heads the list.
	if !lst.Passage || passageIn(lst.Objects[:1]) == nil {
		t.Fatalf("the selected text does not head the list (Passage %v, first object %T)", lst.Passage, lst.Objects[0])
	}
	// Verse 1's group has nothing the loaded text can follow and stands as
	// its letter and words; verse 2's second note is a group with a citation
	// to follow and stands as that group's row beside its letter, closed.
	groups := groupRowsIn(lst.Objects)
	if len(groups) != 1 || groups[0].group.Text != "(John 1:13; Gal. 6:15; 1 John 3:9)" || groups[0].open || groups[0].letter == nil || groups[0].letter.Text != "c" {
		t.Fatalf("group rows = %d; want the closed lone group on verse 2 under c", len(groups))
	}
	if got := noteHeadsIn(lst.Objects); joined(got) != "b  John 7:50; 19:39" {
		t.Fatalf("note rows = %q", got)
	}
	if !lst.Sections.Parallels || !lst.Sections.Publisher || !lst.Sections.Treasury {
		t.Errorf("sections = %+v; want all three on screen", lst.Sections)
	}
	// One heading per source, never per verse: the publisher's run directly
	// under the lettered passage (a single NKJV REFERENCES label over all four
	// notes across verses 1–3), then Gospel parallels, then the Treasury.
	want := "NKJV REFERENCES|GOSPEL PARALLELS|TREASURY OF SCRIPTURE KNOWLEDGE"
	if got := sectionLabelsIn(lst.Objects); joined(got) != want {
		t.Errorf("section labels = %q\nwant %q", got, want)
	}
	// Every source retains its relationship, including the Treasury's
	// destination that the publisher also cites.
	if got := cardLabelsIn(lst.Objects); joined(got) != "John 7:50|Matthew 3:1|John 7:50|Galatians 6:15|Romans 5:8" {
		t.Errorf("cards = %q", got)
	}
	tskAt := -1
	for i, o := range lst.Objects {
		if got := sectionLabelsIn([]fyne.CanvasObject{o}); len(got) == 1 && got[0] == treasuryLabel {
			tskAt = i
			break
		}
	}
	if tskAt < 0 {
		t.Fatalf("no %q label in the list", treasuryLabel)
	}
	if before, after := joined(cardLabelsIn(lst.Objects[:tskAt])), joined(cardLabelsIn(lst.Objects[tskAt:])); before != "John 7:50|Matthew 3:1" || after != "John 7:50|Galatians 6:15|Romans 5:8" {
		t.Errorf("cards before %s = %q, after = %q; want every Treasury relationship retained", treasuryLabel, before, after)
	}
	// Verse 2's first note is more than one piece: a row that carries the
	// note as printed, open, with its pieces under it in print order — the
	// card, then the continuation the loaded text lacks, as its words.
	notes := noteRowsIn(lst.Objects)
	if len(notes) != 1 || notes[0].note.Text != "John 7:50; 19:39" || !notes[0].open || !notes[0].body.Visible() {
		t.Fatalf("note rows = %d; want the one note \"John 7:50; 19:39\", open", len(notes))
	}
	if got := notes[0].letter.Text + "  " + joined(textsIn(notes[0].label)); got != "b  John 7:50; 19:39" {
		t.Errorf("the note's row reads %q; want its letter and the note as printed", got)
	}
	// Verse 2's second note is one group: closed until tapped, and opening
	// into everything between its parentheses in print order.
	test.Tap(groups[0].header)
	if got := textsIn(groups[0].members.Objects[0]); !groups[0].open || len(groups[0].members.Objects) != 3 || joined(got) != "John 1:13" || joined(cardLabelsIn(groups[0].members.Objects)) != "Galatians 6:15" || joined(textsIn(groups[0].members.Objects[2])) != "1 John 3:9" {
		t.Errorf("the lone group opened has %d pieces, the first %q, its cards %q", len(groups[0].members.Objects), got, cardLabelsIn(groups[0].members.Objects))
	}
	test.Tap(groups[0].header)
	if n := len(notes[0].pieces); n != 2 {
		t.Fatalf("the note has %d pieces under it, want its card and its words", n)
	}
	if got := cardLabelsIn(notes[0].pieces[:1]); joined(got) != "John 7:50" {
		t.Errorf("the note's first piece is %q, want the card John 7:50", got)
	}
	if got := textsIn(notes[0].pieces[1]); joined(got) != "19:39" {
		t.Errorf("the note's second piece is %q, want the words 19:39", got)
	}
	all := joinedTexts(lst.Objects)
	// Everything the notes print stands in the list verbatim: the untagged
	// note, the note of two pieces, the group whole, and the citation of the
	// reader's own passage, which is shown in full and not followed.
	for _, want := range []string{"(Acts 10:38)", "John 7:50; 19:39", "(John 1:13; Gal. 6:15; 1 John 3:9)", "John 3:3", "A synopsis probe"} {
		if !strings.Contains(all, want) {
			t.Errorf("the list lacks %q:\n%s", want, all)
		}
	}
	// Each note under its letter, running continuously across the selection:
	// the letter is a text of its own, in the margin, directly before the note.
	var flat []string
	for _, o := range lst.Objects {
		flat = append(flat, textsIn(o)...)
	}
	for _, want := range [][2]string{{"a", "(Acts 10:38)"}, {"b", "John 7:50; 19:39"}, {"c", "(John 1:13; Gal. 6:15; 1 John 3:9)"}, {"d", "John 3:3"}} {
		found := false
		for i := 1; i < len(flat); i++ {
			if flat[i-1] == want[0] && flat[i] == want[1] {
				found = true
			}
		}
		if !found {
			t.Errorf("no letter %q directly before %q:\n%s", want[0], want[1], all)
		}
	}
	// No pill under the parallels' label (the pill is a text of its own; the
	// label's word is not it), no legend, no notice or credit in the list.
	if hasLine(lst.Objects, "PARALLEL") {
		t.Errorf("the parallel card wears its pill under the GOSPEL PARALLELS label:\n%s", all)
	}
	for _, banned := range []string{"compare", "Thomas Nelson", "OpenBible", "cross references"} {
		if strings.Contains(all, banned) {
			t.Errorf("%q is in the list:\n%s", banned, all)
		}
	}
	if b := bannedIn(lst.Objects); len(b) != 0 {
		t.Errorf("the list holds %v", b)
	}
	// A card's tap follows its passage.
	cards := cardsIn(lst.Objects)
	cards[0].Tapped(nil)
	if len(followed) != 1 || followed[0] != (crossRef{Book: "John", Chapter: 7, Verse: 50}) {
		t.Errorf("tapping the publisher's card followed %+v", followed)
	}
	// The footer names both sets, the Treasury in its label's words, one to a
	// line, and the notice once.
	sources, notice := crossRefFooterCredit(st, lst.Sections)
	if sources != treasuryCredit+"\n"+parallelsCredit {
		t.Errorf("footer sources = %q", sources)
	}
	if !strings.Contains(notice, "Thomas Nelson") {
		t.Errorf("the footer carries no notice: %q", notice)
	}
}

// Sections are for the sources only (NKJV, Gospel parallels, Treasury), never
// for verses, a Psalm's title or a heading: every note of the selection stands
// in order under a single NKJV REFERENCES label.
func TestCrossRefListHasOnePublisherSectionForTheSelection(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	licenseNKJVForTest(t)
	withPublisherCrossRefs(t, "nkjv", true)
	st, sel := listFixtureState()
	nop := func(crossRef) {}

	// One verse: one NKJV REFERENCES label.
	lst := buildCrossRefList(st, sel[1:2], nil, nil, lightPalette, nop)
	if got := sectionLabelsIn(lst.Objects); joined(got) != "NKJV REFERENCES" {
		t.Errorf("one verse: labels = %q", got)
	}
	// Multiple verses with notes: still one NKJV REFERENCES label over all of them.
	lst = buildCrossRefList(st, sel, nil, nil, lightPalette, nop)
	if got := sectionLabelsIn(lst.Objects); joined(got) != "NKJV REFERENCES" {
		t.Errorf("three verses: labels = %q, want one NKJV REFERENCES", got)
	}
	// A Psalm's title note and verse notes share the single NKJV REFERENCES
	// label on verse 1 alone and across multiple verses.
	st.CurrentBook, st.CurrentChapter = "Psalms", 3
	ps := st.Bible.GetChapter("Psalms", 3)
	if lst = buildCrossRefList(st, ps[:1], nil, nil, lightPalette, nop); joined(sectionLabelsIn(lst.Objects)) != "NKJV REFERENCES" {
		t.Errorf("1 verse of Psalm 3: labels = %q", sectionLabelsIn(lst.Objects))
	}
	if lst = buildCrossRefList(st, ps, nil, nil, lightPalette, nop); joined(sectionLabelsIn(lst.Objects)) != "NKJV REFERENCES" {
		t.Errorf("2 verses of Psalm 3: labels = %q", sectionLabelsIn(lst.Objects))
	}
	// CONTROL: a verse without a publisher note has no empty publisher section.
	lst = buildCrossRefList(st, ps[1:], []crossRef{{Book: "John", Chapter: 7, Verse: 50, Votes: 3}}, nil, lightPalette, nop)
	if got := sectionLabelsIn(lst.Objects); joined(got) != treasuryLabel {
		t.Errorf("Psalm 3:2: labels = %q", got)
	}
	// A noted section heading and surrounding verses share the one NKJV
	// REFERENCES label, with their notes in order (verse 20's a, heading's b,
	// verse 21's c).
	x := footnoteKindCrossref
	hbd := xrefBible(map[string]map[int]int{"Numbers": {3: 51, 4: 49}})
	hbd.Verses["Numbers"][4][19].Footnotes = []Footnote{{Anchor: 0, Text: "Num. 3:17", Kind: x, Caller: "-"}}
	hbd.Headings = map[string]map[int][]Heading{"Numbers": {4: {
		{Text: "Duties of the fixture's sons", Style: "s", BeforeVerse: 21, Footnotes: []Footnote{{Text: "Num. 3:22", Kind: x, Caller: "-"}}},
	}}}
	hbd.Verses["Numbers"][4][20].Footnotes = []Footnote{{Anchor: 0, Text: "Num. 3:25", Kind: x, Caller: "-"}}
	hst := &AppState{Bible: hbd, CurrentBook: "Numbers", CurrentChapter: 4, CurrentVersion: "nkjv"}
	hlst := buildCrossRefList(hst, versesBetween(hbd, "Numbers", 4, 20, 22), nil, nil, lightPalette, nop)
	if got := sectionLabelsIn(hlst.Objects); joined(got) != "NKJV REFERENCES" {
		t.Errorf("Numbers 4:20-22 with noted heading: labels = %q, want NKJV REFERENCES", got)
	}
	var letters []string
	for _, c := range cardsIn(hlst.Objects) {
		letters = append(letters, cardLetter(c)+" "+cardReference(c))
	}
	if got := joined(letters); got != "a Numbers 3:17|b Numbers 3:22|c Numbers 3:25" {
		t.Errorf("Numbers 4:20-22 note order = %q", got)
	}
}

// A NOTE IS KEPT WHOLE THOUGH ANOTHER NOTE HAS ITS PASSAGE. Two notes citing
// one passage used to give one card, the first, and a verse whose notes cited
// only passages already carded left nothing on screen and no label. The note
// is the edition's unit: each stands under its own verse and letter with all
// it prints.
func TestANoteIsKeptWholeThoughAnotherNoteHasItsPassage(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	licenseNKJVForTest(t)
	withPublisherCrossRefs(t, "nkjv", true)
	st, _ := listFixtureState()
	nop := func(crossRef) {}
	x := footnoteKindCrossref
	// John 3:3's note cites the passage 3:2's note already cards.
	v3 := &st.Bible.Verses["John"][3][2]
	v3.Footnotes = []Footnote{{Text: "John 7:50", Kind: x, Caller: "-", Refs: []NoteRef{{"JHN.7.50", 0, 9}}}}
	sel := st.Bible.GetChapter("John", 3)[1:]
	// Alone, and with a Treasury row. (Verse 2's second note is one group
	// and starts closed, so it puts no card into the run.)
	for _, refs := range [][]crossRef{nil, {{Book: "Romans", Chapter: 5, Verse: 8, Votes: 3}}} {
		lst := buildCrossRefList(st, sel, refs, nil, lightPalette, nop)
		want := "NKJV REFERENCES"
		wantCards := "John 7:50|John 7:50"
		if len(refs) > 0 {
			want += "|" + treasuryLabel
			wantCards += "|Romans 5:8"
		}
		if got := sectionLabelsIn(lst.Objects); joined(got) != want {
			t.Errorf("with %d Treasury rows: labels = %q, want %q", len(refs), got, want)
		}
		if got := cardLabelsIn(lst.Objects); lst.PublisherCards != 2 || joined(got) != wantCards {
			t.Errorf("with %d Treasury rows: cards = %q, %d publisher cards; want John 7:50 under each verse that cites it", len(refs), got, lst.PublisherCards)
		}
	}
	// CONTROL: a Treasury row for the shared passage remains beneath both
	// publisher notes, preserving all three source relationships.
	lst := buildCrossRefList(st, sel, []crossRef{{Book: "John", Chapter: 7, Verse: 50, Votes: 9}}, nil, lightPalette, nop)
	if lst.TSKRows != 1 || !lst.Sections.Treasury || joined(cardLabelsIn(lst.Objects)) != "John 7:50|John 7:50|John 7:50" {
		t.Errorf("a shared destination lost one of its source relationships: %+v", lst)
	}
}

// A PSALM'S TITLE AND THE WHOLE PSALM ARE TWO CARDS. Both are labelled by the
// chapter ("Psalms 59"), and the rule that two notes citing one passage give
// one card used to key on the label, so the second of them was dropped.
func TestATitleAndTheWholePsalmAreTwoCards(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	licenseNKJVForTest(t)
	withPublisherCrossRefs(t, "nkjv", true)
	st, _ := listFixtureState()
	st.CurrentBook, st.CurrentChapter = "1 Samuel", 19
	nop := func(crossRef) {}
	x := footnoteKindCrossref
	v := &st.Bible.Verses["1 Samuel"][19][0]
	v.Footnotes = []Footnote{{Text: "Ps. 59; Ps. 59:title", Kind: x, Caller: "-", Refs: []NoteRef{{"PSA.59", 0, 6}, {"PSA.59", 8, 14}}}}
	lst := buildCrossRefList(st, st.Bible.GetChapter("1 Samuel", 19), nil, nil, lightPalette, nop)
	if got := cardLabelsIn(lst.Objects); joined(got) != "Psalms 59|Psalms 59" || lst.PublisherCards != 2 {
		t.Fatalf("cards = %q (%d publisher cards); want the psalm and its title", got, lst.PublisherCards)
	}
	cards := cardsIn(lst.Objects)
	if ts := textsIn(cards[1].content); len(ts) < 2 || ts[1] != "Title" {
		t.Errorf("the second card is not the title's: %q", ts)
	}
	if ts := textsIn(cards[0].content); len(ts) > 1 && ts[1] == "Title" {
		t.Errorf("the first card wears the Title tag: %q", ts)
	}
	// CONTROL: neither card wears the tag when the note cites the psalm
	// twice and its title not at all; and it is still two cards, since a
	// note is drawn as it is printed.
	v.Footnotes[0] = Footnote{Text: "Ps. 59; Ps. 59", Kind: x, Caller: "-", Refs: []NoteRef{{"PSA.59", 0, 6}, {"PSA.59", 8, 14}}}
	lst = buildCrossRefList(st, st.Bible.GetChapter("1 Samuel", 19), nil, nil, lightPalette, nop)
	if got := cardLabelsIn(lst.Objects); joined(got) != "Psalms 59|Psalms 59" || lst.PublisherCards != 2 {
		t.Errorf("control: cards = %q (%d publisher cards); want the psalm twice, as printed", got, lst.PublisherCards)
	}
	for i, c := range cardsIn(lst.Objects) {
		if ts := textsIn(c.content); len(ts) > 1 && ts[1] == "Title" {
			t.Errorf("control: card %d wears the Title tag: %q", i, ts)
		}
	}
}

// Each source keeps its own relationship, including matching and overlapping
// destinations. A publisher card never removes a Treasury row or its credit.
func TestCrossRefListKeepsTreasuryRelationshipsAlsoCarriedByPublisher(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	licenseNKJVForTest(t)
	withPublisherCrossRefs(t, "nkjv", true)
	st, _ := listFixtureState()
	st.CurrentBook, st.CurrentChapter = "Psalms", 3
	st.Bible.Verses["2 Samuel"] = map[int][]Verse{
		15: {{BookName: "2 Samuel", Book: "2 Samuel", Chapter: 15, Verse: 13, Text: "A fixture messenger came."}},
		16: {{BookName: "2 Samuel", Book: "2 Samuel", Chapter: 16, Verse: 1, Text: "Over the fixture hill."}},
	}
	ps := st.Bible.GetChapter("Psalms", 3)
	nop := func(crossRef) {}
	refs := []crossRef{
		{Book: "2 Samuel", Chapter: 15, Verse: 1, EndV: 18, Votes: 20}, // contains the publisher card's start
		{Book: "2 Samuel", Chapter: 15, Verse: 14, Votes: 9},           // inside the publisher passage
		{Book: "2 Samuel", Chapter: 16, Verse: 1, Votes: 5},            // another passage
		{Book: "John", Chapter: 7, Verse: 50, Votes: 3},
	}
	lst := buildCrossRefList(st, ps[:1], refs, nil, lightPalette, nop)
	if got := cardLabelsIn(lst.Objects); joined(got) != "2 Samuel 15:13–17|2 Samuel 15:1–18|2 Samuel 15:14|2 Samuel 16:1|John 7:50" {
		t.Errorf("cards = %q", got)
	}
	if lst.PublisherCards != 1 || lst.TSKRows != 4 || !lst.Sections.Treasury {
		t.Errorf("list = %+v", lst)
	}
	// CONTROL: with the flag off every Treasury row shows.
	withPublisherCrossRefs(t, "nkjv", false)
	if lst := buildCrossRefList(st, ps[:1], refs, nil, lightPalette, nop); lst.TSKRows != 4 {
		t.Errorf("flag off: %d Treasury rows, want all 4", lst.TSKRows)
	}
	withPublisherCrossRefs(t, "nkjv", true)
	// Even when every target overlaps, the Treasury remains a separate source.
	lst = buildCrossRefList(st, ps[:1], refs[:2], nil, lightPalette, nop)
	if got := sectionLabelsIn(lst.Objects); joined(got) != "NKJV REFERENCES|TREASURY OF SCRIPTURE KNOWLEDGE" {
		t.Errorf("labels with overlapping Treasury targets = %q", got)
	}
	if lst.TSKRows != 2 || !lst.Sections.Treasury {
		t.Errorf("the Treasury lost its own relationships: %+v", lst)
	}
	if sources, _ := crossRefFooterCredit(st, lst.Sections); sources != treasuryCredit {
		t.Errorf("the footer lost the displayed Treasury credit: %q", sources)
	}
}

// A citation of a Psalm's title previews the superscription and never washes
// verse 1. Whole-chapter citations preview the opening verse. Treasury rows
// retain their own destinations in both cases, including an exact overlap.
func TestTitleAndWholeChapterCardsKeepSpecificTreasuryRows(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	licenseNKJVForTest(t)
	withPublisherCrossRefs(t, "nkjv", true)
	st, _ := listFixtureState()
	nop := func(crossRef) {}

	// 1. 1 Samuel 19:12 cites "Ps. 59:title": both Psalms 59:1 and Psalms 59:1–2 stay.
	st.CurrentBook, st.CurrentChapter = "1 Samuel", 19
	titleTSK := []crossRef{
		{Book: "Psalms", Chapter: 59, Verse: 1, Votes: 25},
		{Book: "Psalms", Chapter: 59, Verse: 1, EndV: 2, Votes: 15},
	}
	lst := buildCrossRefList(st, st.Bible.GetChapter("1 Samuel", 19), titleTSK, nil, lightPalette, nop)
	if got := cardLabelsIn(lst.Objects); joined(got) != "Psalms 59|Psalms 59:1|Psalms 59:1–2" || lst.TSKRows != 2 {
		t.Errorf("under a :title card, cards = %q (TSKRows=%d); want Psalms 59 + both Treasury verse rows", got, lst.TSKRows)
	}

	// 2. Genesis 7:2 cites whole-chapter "Lev. 11" (verses 1..2 in the fixture):
	// specific verses, a cross-chapter range, and the exact whole chapter
	// all retain their independent Treasury relationship.
	st.Bible.Verses["Leviticus"][10] = []Verse{{BookName: "Leviticus", Book: "Leviticus", Chapter: 10, Verse: 1, Text: "An earlier fixture chapter."}}
	st.CurrentBook, st.CurrentChapter = "Genesis", 7
	chapterTSK := []crossRef{
		{Book: "Leviticus", Chapter: 11, Verse: 1, Votes: 30},                    // single verse: stays
		{Book: "Leviticus", Chapter: 11, Verse: 2, Votes: 20},                    // single verse: stays
		{Book: "Leviticus", Chapter: 10, Verse: 1, EndCh: 11, EndV: 2, Votes: 9}, // wider multi-chapter: stays
		{Book: "Leviticus", Chapter: 11, Verse: 1, EndV: 2, Votes: 40},           // exact whole chapter: stays
	}
	lst = buildCrossRefList(st, st.Bible.GetChapter("Genesis", 7), chapterTSK, nil, lightPalette, nop)
	if got := cardLabelsIn(lst.Objects); joined(got) != "Leviticus 11|Leviticus 11:1|Leviticus 11:2|Leviticus 10:1–11:2|Leviticus 11:1–2" || lst.TSKRows != 4 {
		t.Errorf("under a whole-chapter card, cards = %q (TSKRows=%d); want all four Treasury relationships kept", got, lst.TSKRows)
	}
}

func TestCrossRefListOmitsEmptyPublisherSection(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	licenseNKJVForTest(t)
	withPublisherCrossRefs(t, "nkjv", true)
	st, john3 := listFixtureState()
	st.CurrentBook, st.CurrentChapter = "Psalms", 3
	ps := st.Bible.GetChapter("Psalms", 3)
	nop := func(crossRef) {}

	// Psalm 3:2 has no note: only the populated Treasury section appears.
	lst := buildCrossRefList(st, ps[1:], []crossRef{{Book: "John", Chapter: 7, Verse: 50, Votes: 3}}, nil, lightPalette, nop)
	if got := sectionLabelsIn(lst.Objects); joined(got) != treasuryLabel {
		t.Errorf("labels = %q", got)
	}
	if lst.PublisherCards != 0 || lst.PublisherLines != 0 || lst.Sections.Publisher || lst.TSKRows != 1 {
		t.Errorf("list = %+v", lst)
	}
	all := joinedTexts(lst.Objects)
	if strings.Contains(all, "No NKJV cross-references for this selection.") || strings.Contains(all, "NKJV REFERENCES") || strings.Contains(all, "Thomas Nelson") {
		t.Errorf("the absent publisher leaked a section or credit:\n%s", all)
	}
	// Nothing at all for the selection is the panel's message state, not a
	// run of one line over an empty Treasury.
	if lst := buildCrossRefList(st, ps[1:], nil, nil, lightPalette, nop); len(lst.Objects) != 0 {
		t.Errorf("nothing to list, yet %d objects: %q", len(lst.Objects), joinedTexts(lst.Objects))
	}
	// Offline with nothing of the publisher's: the message state, which the
	// panel turns into its connection error.
	if lst := buildCrossRefList(st, ps[1:], nil, errors.New("offline"), lightPalette, nop); len(lst.Objects) != 0 {
		t.Errorf("offline and no notes, yet %d objects: %q", len(lst.Objects), joinedTexts(lst.Objects))
	}
	// Offline with the publisher's notes: the run, then the Treasury's label
	// and one line, no verb, no credit.
	st.CurrentBook, st.CurrentChapter = "John", 3
	lst = buildCrossRefList(st, john3[1:2], nil, errors.New("offline"), lightPalette, nop)
	if got := sectionLabelsIn(lst.Objects); joined(got) != "NKJV REFERENCES|TREASURY OF SCRIPTURE KNOWLEDGE" {
		t.Errorf("offline labels = %q", got)
	}
	all = joinedTexts(lst.Objects)
	if !strings.Contains(all, "Couldn't load cross-references.") || strings.Contains(all, "try again") {
		t.Errorf("an offline Treasury is not one line in its place:\n%s", all)
	}
	if lst.Sections.Treasury || lst.PublisherCards != 1 {
		t.Errorf("offline list = %+v", lst)
	}
	if sources, _ := crossRefFooterCredit(st, lst.Sections); sources != "" {
		t.Errorf("the footer credits a Treasury that did not load: %q", sources)
	}
	if b := bannedIn(lst.Objects); len(b) != 0 {
		t.Errorf("the offline list holds %v", b)
	}
	// CONTROL: with the flag off the same failure and no rows is an empty
	// list, which the panel turns into its message.
	withPublisherCrossRefs(t, "nkjv", false)
	if lst := buildCrossRefList(st, john3[1:2], nil, errors.New("offline"), lightPalette, nop); len(lst.Objects) != 0 {
		t.Errorf("flag off, nothing loaded, yet %d objects", len(lst.Objects))
	}
}

// The footer's sources line names the sets with rows on screen, and no other,
// one credit to a line: two on one line broke inside a credit on a phone.
func TestTheFooterNamesOnlyTheSetsOnScreen(t *testing.T) {
	licenseNKJVForTest(t)
	st, _ := listFixtureState()
	for _, tc := range []struct {
		flag     bool
		sections crossRefSections
		want     string
	}{
		{false, crossRefSections{Treasury: true, Parallels: true}, treasuryCredit + "\n" + parallelsCredit},
		{false, crossRefSections{Treasury: true}, treasuryCredit},
		{false, crossRefSections{Parallels: true}, "Gospel parallels: synopsis"},
		{false, crossRefSections{}, ""},
		{true, crossRefSections{Treasury: true, Parallels: true, Publisher: true}, "Treasury of Scripture Knowledge: OpenBible.info (CC-BY)\nGospel parallels: synopsis"},
		{true, crossRefSections{Treasury: true, Publisher: true}, "Treasury of Scripture Knowledge: OpenBible.info (CC-BY)"},
		{true, crossRefSections{Publisher: true}, ""},
	} {
		withPublisherCrossRefs(t, "nkjv", tc.flag)
		if sources, _ := crossRefFooterCredit(st, tc.sections); sources != tc.want {
			t.Errorf("flag %v, %+v: sources = %q, want %q", tc.flag, tc.sections, sources, tc.want)
		}
	}
}

// A title card previews the chapter's superscription and wears the Title tag;
// the same passage as an ordinary card previews the verse.
func TestATitleCardPreviewsTheSuperscription(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	st := &AppState{Bible: publisherFixture(), CurrentBook: "1 Samuel", CurrentChapter: 19, CurrentVersion: "nkjv"}
	ref := crossRef{Book: "Psalms", Chapter: 59, Verse: 1}
	nop := func(crossRef) {}
	title := textsIn(crossRefRow(st, crossRefCard{Ref: ref, Label: "Psalms 59", Tag: "Title", Title: true}, lightPalette, nop))
	got := strings.Join(title, "\n")
	if len(title) < 3 || title[0] != "Psalms 59" || title[1] != "Title" || !strings.Contains(got, "A fixture song of David") || strings.Contains(got, "Rescue me") {
		t.Errorf("the title card reads %q; want the chapter, the Title tag and the superscription", title)
	}
	// CONTROL: the ordinary card of the same verse.
	plain := textsIn(crossRefRow(st, passageCard(ref, false), lightPalette, nop))
	if got := strings.Join(plain, "\n"); plain[0] != "Psalms 59:1" || strings.Contains(got, "Title") || !strings.Contains(got, "Rescue me") {
		t.Errorf("the plain card reads %q", plain)
	}
	// A chapter whose title the loaded text lacks previews its first verse,
	// where the tap lands.
	bare := textsIn(crossRefRow(st, crossRefCard{Ref: crossRef{Book: "Psalms", Chapter: 45, Verse: 4}, Label: "Psalms 45", Tag: "Title", Title: true}, lightPalette, nop))
	if got := strings.Join(bare, "\n"); !strings.Contains(got, "Ride on") {
		t.Errorf("a title card over an untitled chapter reads %q", bare)
	}
	// A basis tag stands in the same slot.
	basis := textsIn(crossRefRow(st, crossRefCard{Ref: crossRef{Book: "Job", Chapter: 13, Verse: 16}, Label: "Job 13:16", Tag: "LXX"}, lightPalette, nop))
	if len(basis) < 2 || basis[0] != "Job 13:16" || basis[1] != "LXX" {
		t.Errorf("the qualified card reads %q", basis)
	}
}

// A ROW'S TAP WASHES WHAT THE ROW CITES. It used to wash the first verse
// only — "John 1:1–3" lit John 1:1 — though Go to and the verse of the day
// already wash a whole range. A wash is one chapter, so a range that runs on
// is washed to the end of the chapter it starts in.
func TestFollowingARowWashesTheRangeItCites(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	bd := xrefBible(map[string]map[int]int{"John": {1: 51}, "Mark": {8: 38, 9: 50}, "Ruth": {4: 22}, "1 Samuel": {1: 28}})
	for _, tc := range []struct {
		c      crossRef
		lo, hi int
	}{
		{crossRef{Book: "John", Chapter: 1, Verse: 1}, 1, 1},
		{crossRef{Book: "John", Chapter: 1, Verse: 1, EndV: 3}, 1, 3},
		{crossRef{Book: "Mark", Chapter: 8, Verse: 34, EndCh: 9, EndV: 1}, 34, 38},
		{crossRef{Book: "Ruth", Chapter: 4, Verse: 18, EndBook: "1 Samuel", EndCh: 1, EndV: 2}, 18, 22},
	} {
		st := &AppState{Bible: bd, CurrentBook: "John", CurrentChapter: 1, CurrentVersion: "web"}
		followCrossRef(st, tc.c)
		at := st.mark.At
		if st.CurrentBook != tc.c.Book || st.CurrentChapter != tc.c.Chapter {
			t.Errorf("%s: landed on %s %d", tc.c.label(), st.CurrentBook, st.CurrentChapter)
		}
		if at.Book != tc.c.Book || at.Chapter != tc.c.Chapter || at.Lo != tc.lo || at.Hi != tc.hi {
			t.Errorf("%s: washed %s %d:%d-%d, want %d-%d", tc.c.label(), at.Book, at.Chapter, at.Lo, at.Hi, tc.lo, tc.hi)
		}
	}

	// A Psalm-title citation (TitleRef) opens the Psalm at its top without
	// washing verse 1 or leaving any earlier highlight behind, whereas an
	// ordinary verse-1 citation washes verse 1.
	bd.Verses["Psalms"] = map[int][]Verse{59: {
		{BookName: "Psalms", Book: "Psalms", Chapter: 59, Verse: 1, Text: "Synthetic verse 1."},
		{BookName: "Psalms", Book: "Psalms", Chapter: 59, Verse: 2, Text: "Synthetic verse 2."},
	}}
	st := &AppState{
		Bible:                    bd,
		CurrentBook:              "Psalms",
		CurrentChapter:           142,
		CurrentVersion:           "nkjv",
		CanReturnToSearchResults: true,
		restore:                  &restoreAnchor{},
	}
	st.setMark(hlVerseOfDay, VerseSpan{Book: "Psalms", Chapter: 142, Lo: 1, Hi: 3, VersionID: "nkjv"})
	followCrossRef(st, crossRef{Book: "Psalms", Chapter: 59, Verse: 1, TitleRef: true})
	if st.CurrentBook != "Psalms" || st.CurrentChapter != 59 {
		t.Errorf("title ref landed on %s %d, want Psalms 59", st.CurrentBook, st.CurrentChapter)
	}
	if st.mark.live() || st.mark.At != (VerseSpan{}) {
		t.Errorf("title ref left highlight %+v; want none", st.mark)
	}
	if !st.forceReposition || st.restore != nil || st.CanReturnToSearchResults {
		t.Errorf("title ref state: forceReposition=%v restore=%v returnSearch=%v", st.forceReposition, st.restore, st.CanReturnToSearchResults)
	}
	followCrossRef(st, crossRef{Book: "Psalms", Chapter: 59, Verse: 1})
	if !st.mark.live() || st.mark.At.Book != "Psalms" || st.mark.At.Chapter != 59 || st.mark.At.Lo != 1 || st.mark.At.Hi != 1 {
		t.Errorf("ordinary verse-1 ref washed %+v, want Psalms 59:1", st.mark)
	}
}

// THE OWN-PASSAGE RULE APPLIES TO BOTH PUBLISHER AND TREASURY ROWS: only a
// citation equal to or contained within the selected passage is muted; wider
// pericope ranges stay tappable cards.
func TestOwnPassageIsMutedOnlyWithinSelection(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	licenseNKJVForTest(t)
	withPublisherCrossRefs(t, "nkjv", true)
	bd := xrefBible(map[string]map[int]int{"Matthew": {3: 15}, "Mark": {1: 10}})
	v1 := &bd.Verses["Matthew"][3][0]
	v1.Footnotes = []Footnote{{
		Anchor: 0,
		Text:   "Matt. 3:1–12; Mark 1:3–8",
		Kind:   footnoteKindCrossref,
		Caller: "-",
		Refs:   []NoteRef{{"MAT.3.1-MAT.3.12", 0, 12}, {"MRK.1.3-MRK.1.8", 14, 24}},
	}}
	st := &AppState{Bible: bd, CurrentBook: "Matthew", CurrentChapter: 3, CurrentVersion: "nkjv"}
	tsk := []crossRef{
		{Book: "Matthew", Chapter: 3, Verse: 1, EndV: 6, Votes: 12},
		{Book: "Mark", Chapter: 1, Verse: 9, Votes: 5},
	}

	// Single verse 3:1: publisher Matt. 3:1–12 and Treasury Matt. 3:1–6 are
	// both wider than 3:1, so both remain cards in their own source sections,
	// alongside Mark 1:3–8 and Mark 1:9.
	one := versesBetween(bd, "Matthew", 3, 1, 1)
	lstOne := buildCrossRefList(st, one, tsk, nil, lightPalette, func(crossRef) {})
	if got := joined(cardLabelsIn(lstOne.Objects)); got != "Matthew 3:1–12|Mark 1:3–8|Matthew 3:1–6|Mark 1:9" {
		t.Errorf("single verse 3:1 cards = %q, want Matthew 3:1–12|Mark 1:3–8|Matthew 3:1–6|Mark 1:9", got)
	}

	// Selecting the full pericope 3:1–12: publisher Matt. 3:1–12 is now equal
	// to the selection (a muted self-row), and Treasury Matt. 3:1–6 is contained in 3:1–12
	// so it also renders as a muted self-row under the Treasury section!
	span := versesBetween(bd, "Matthew", 3, 1, 12)
	lstSpan := buildCrossRefList(st, span, tsk, nil, lightPalette, func(crossRef) {})
	if got := joined(cardLabelsIn(lstSpan.Objects)); got != "Mark 1:3–8|Mark 1:9" {
		t.Errorf("pericope 3:1–12 cards = %q, want Mark 1:3–8|Mark 1:9", got)
	}
	if lstSpan.TSKRows != 2 {
		t.Errorf("TSKRows = %d, want 2 (muted self-row + Mark 1:9 card)", lstSpan.TSKRows)
	}
	all := joinedTexts(lstSpan.Objects)
	if !strings.Contains(all, "Matthew 3:1–6") {
		t.Errorf("muted Treasury self-row label missing from list:\n%s", all)
	}

	// Disabled publisher control: the own-passage rule still
	// applies, independently of whether publisher notes are displayed.
	withPublisherCrossRefs(t, "nkjv", false)
	storeSpan := buildCrossRefList(st, span, tsk, nil, lightPalette, func(crossRef) {})
	if got := joined(cardLabelsIn(storeSpan.Objects)); got != "Mark 1:9" {
		t.Errorf("store build cards = %q, want Mark 1:9", got)
	}
	if !strings.Contains(joinedTexts(storeSpan.Objects), "Matthew 3:1–6") {
		t.Error("store build lost the muted Treasury self-reference")
	}
}

// A preview ends on a whole word, and never with a space before its ellipsis.
func TestACrossReferencePreviewEndsOnAWord(t *testing.T) {
	short := "In the beginning, God created the heavens and the earth."
	if got := crossRefPreview(short); got != short {
		t.Errorf("a verse shorter than the preview is shown whole, got %q", got)
	}
	// 89 runes of words, then a space at rune 90: the old cut ended "… ".
	long := strings.Repeat("word ", 18) + "andmore text follows here"
	got := crossRefPreview(long)
	if strings.HasSuffix(got, " …") || !strings.HasSuffix(got, "…") {
		t.Errorf("preview %q must end with an ellipsis straight after a word", got)
	}
	// A cut that falls inside a word backs off to the word before it.
	mid := strings.Repeat("abcdefghi ", 8) + "unbreakableword and the rest of the verse"
	if got := crossRefPreview(mid); strings.Contains(got, "unbreak") || !strings.HasSuffix(got, "abcdefghi…") {
		t.Errorf("preview %q must not end inside a word", got)
	}
	for _, s := range []string{long, mid} {
		if n := len([]rune(crossRefPreview(s))); n > crossRefPreviewRunes+1 {
			t.Errorf("preview of %d runes is longer than the %d-rune cut", n, crossRefPreviewRunes)
		}
	}
}

// THE PREVIEW KEEPS THE DIVINE NAME'S SMALL CAPITALS. In the NKJV they are the
// only difference between the Tetragrammaton and Adonai, and the preview showed
// the raw text, which spells both "Lord".
func TestACrossReferencePreviewKeepsTheSmallCapitals(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	const text = "Thus says the Lord God: the Lord is my shepherd."
	bd := &BibleData{Books: []string{"Psalms"}, Verses: map[string]map[int][]Verse{"Psalms": {23: {{
		BookName: "Psalms", Book: "Psalms", Chapter: 23, Verse: 1, Text: text,
		// "Lord God" is Adonai YHWH: the second word alone is in small capitals.
		SmallCaps: []TextSpan{{Start: 19, End: 22}, {Start: 28, End: 32}},
	}}}}}
	st := &AppState{Bible: bd, CurrentBook: "Psalms", CurrentChapter: 23, CurrentVersion: "nkjv"}
	row := crossRefRow(st, passageCard(crossRef{Book: "Psalms", Chapter: 23, Verse: 1}, false), st.pal(), func(crossRef) {})
	got := strings.Join(textsIn(row), "\n")
	if !strings.Contains(got, "the Lord Gᴏᴅ: the Lᴏʀᴅ is my shepherd.") {
		t.Errorf("the preview reads %q; it must keep the small capitals the page draws", got)
	}
}
