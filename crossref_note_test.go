package bibletext

import (
	"fmt"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
)

// pieceNames names pieces in order, each by its kind and what it shows: a
// card or the reader's own passage by its label and tag, words as printed,
// and a group by its printed text and its own pieces.
func pieceNames(pieces []publisherPiece) []string {
	var out []string
	for _, p := range pieces {
		switch {
		case p.Group != nil:
			out = append(out, "group:"+p.Group.Text+"["+strings.Join(pieceNames(p.Group.Pieces), " ")+"]")
		case p.Target != nil:
			kind := "card:"
			if p.Target.Current {
				kind = "self:"
			}
			name := kind + p.Target.label()
			if tag := p.Target.tag(); tag != "" {
				name += " [" + tag + "]"
			}
			out = append(out, name)
		default:
			out = append(out, "words:"+p.Words)
		}
	}
	return out
}

// noteGapProblems is every stretch of a note, and of its groups, that stands
// outside its pieces and is not a separator the edition sets between them.
func noteGapProblems(n publisherCrossRef) []string {
	var out []string
	runes := []rune(n.Text)
	check := func(where string, pieces []publisherPiece, from, to int) {
		gaps := pieceGaps(runes, pieces, from, to)
		for i, g := range gaps {
			ok := g == "; " || g == ", "
			if i == 0 || i == len(gaps)-1 {
				ok = g == ""
			}
			if !ok {
				out = append(out, fmt.Sprintf("%s: %q stands outside every piece", where, g))
			}
		}
	}
	check("the note", n.Pieces, 0, len(runes))
	for _, p := range n.Pieces {
		if p.Group != nil {
			check("the group "+p.Group.Text, p.Group.Pieces, p.Group.Start+1, p.Group.End-1)
		}
	}
	return out
}

// noteShape names the row a note is drawn as.
func noteShape(n publisherCrossRef) string {
	if _, ok := n.alone(); ok {
		return "card"
	}
	if _, ok := n.groupAlone(); ok {
		return "group"
	}
	if _, ok := n.wordsAlone(); ok {
		return "words"
	}
	return "fold"
}

// EVERYTHING A NOTE PRINTS IS A PIECE, IN THE ORDER IT IS PRINTED. A citation
// of the reader's own verse, a citation the loaded text lacks and one the
// shorthand cannot read used to be gathered after the note's cards as one
// caption; a card another note had was dropped. Each now has its place.
func TestANoteIsItsPiecesInTheOrderItPrintsThem(t *testing.T) {
	bd := xrefBible(map[string]map[int]int{
		"Judges": {9: 57}, "1 Samuel": {16: 23, 18: 30, 19: 24}, "1 Kings": {22: 53},
		"Mark": {4: 41, 13: 37}, "Revelation": {14: 20}, "Isaiah": {15: 9}, "Jeremiah": {25: 38},
		"Psalms": {45: 17, 59: 17, 135: 21, 136: 26}, "Luke": {11: 54}, "Matthew": {6: 34},
		"John": {2: 25, 10: 42}, "1 Corinthians": {1: 31, 2: 16},
	})
	oddPrint := "Ps. 135:1-3; 136:1" // a hyphen the shorthand does not read: the feed's tags stand in
	wrongTag := "John 10:20; (1 Cor. 1:23; 2:13)"
	cases := []struct {
		name, book string
		ch, v      int
		text       string
		refs       []NoteRef
		want       string
		shape      string
	}{
		{"the reader's own verse among others", "1 Samuel", 16, 16, "Judg. 9:23; 1 Sam. 16:15, 16; 18:10; 19:9; 1 Kin. 22:19–22", nil,
			"card:Judges 9:23|card:1 Samuel 16:15|self:1 Samuel 16:16|card:1 Samuel 18:10|card:1 Samuel 19:9|card:1 Kings 22:19–22", "fold"},
		{"the reader's own passage first", "Matthew", 6, 9, "Matt. 6:9–13; Luke 11:2–4", nil,
			"self:Matthew 6:9–13|card:Luke 11:2–4", "fold"},
		{"a verse the loaded text lacks, inside a group", "Mark", 4, 29, "(Mark 13:30, 39); Rev. 14:15", nil,
			"group:(Mark 13:30, 39)[card:Mark 13:30 words:39]|card:Revelation 14:15", "fold"},
		{"a citation the shorthand does not read, first", "Matthew", 6, 1, "Is. 15:1—16; Jer. 25:21", nil,
			"words:Is. 15:1—16|card:Jeremiah 25:21", "fold"},
		{"a basis after two verses", "Revelation", 14, 1, "Ps. 45:4, 5, LXX", nil,
			"card:Psalms 45:4 [LXX]|card:Psalms 45:5 [LXX]", "fold"},
		{"a Psalm's title alone", "1 Samuel", 19, 12, "Ps. 59:title", nil,
			"card:Psalms 59 [Title]", "card"},
		{"a group alone", "Matthew", 6, 2, "(Is. 15:1)", nil,
			"group:(Is. 15:1)[card:Isaiah 15:1]", "group"},
		{"a citation the loaded text lacks, alone", "Matthew", 6, 3, "Ezek. 8:21", nil,
			"words:Ezek. 8:21", "words"},
		{"the reader's own verse alone", "Revelation", 14, 15, "Rev. 14:15", nil,
			"self:Revelation 14:15", "card"},
		{"a print the shorthand cannot read, tagged", "Psalms", 135, 1, oddPrint,
			[]NoteRef{noteRefAt(t, oddPrint, "Ps. 135:1-3", "PSA.135.1-PSA.135.3"), noteRefAt(t, oddPrint, "136:1", "PSA.136.1")},
			"self:Psalms 135:1–3|card:Psalms 136:1", "fold"},
		{"a tag the feed set wrong inside a group", "Mark", 4, 1, wrongTag,
			[]NoteRef{noteRefAt(t, wrongTag, "John 10:20", "JHN.10.20"), noteRefAt(t, wrongTag, "2:13", "JHN.2.13")},
			"card:John 10:20|group:(1 Cor. 1:23; 2:13)[card:1 Corinthians 1:23 card:1 Corinthians 2:13]", "fold"},
	}
	for _, c := range cases {
		verses := versesBetween(bd, c.book, c.ch, c.v, c.v)
		if len(verses) != 1 {
			t.Fatalf("%s: control: the fixture has no %s %d:%d", c.name, c.book, c.ch, c.v)
		}
		verses[0].Footnotes = []Footnote{{Anchor: 3, Text: c.text, Kind: footnoteKindCrossref, Caller: "-", Refs: c.refs}}
		notes := publisherCrossRefsFor(bd, c.book, c.ch, verses)
		if len(notes) != 1 {
			t.Fatalf("%s: %d notes", c.name, len(notes))
		}
		n := notes[0]
		if got := joined(pieceNames(n.Pieces)); got != c.want {
			t.Errorf("%s: pieces\n got %s\nwant %s", c.name, got, c.want)
		}
		if got := noteShape(n); got != c.shape {
			t.Errorf("%s: drawn as a %s, want a %s", c.name, got, c.shape)
		}
		if n.Anchor != 3 || n.Index != 0 || n.Heading {
			t.Errorf("%s: the note stands at %d, index %d, heading %v; want offset 3, the verse's first", c.name, n.Anchor, n.Index, n.Heading)
		}
		for _, problem := range noteGapProblems(n) {
			t.Errorf("%s: %s", c.name, problem)
		}
	}

	// CONTROL: the gaps can speak. A note with one of its pieces taken away
	// has that piece's words outside every piece.
	verses := versesBetween(bd, "1 Samuel", 16, 16, 16)
	verses[0].Footnotes = []Footnote{{Text: cases[0].text, Kind: footnoteKindCrossref, Caller: "-"}}
	n := publisherCrossRefsFor(bd, "1 Samuel", 16, verses)[0]
	n.Pieces = append(append([]publisherPiece(nil), n.Pieces[:2]...), n.Pieces[3:]...)
	if problems := noteGapProblems(n); len(problems) != 1 || !strings.Contains(problems[0], `", 16; "`) {
		t.Errorf("control: with the third piece removed the gaps report %q", problems)
	}
}

// A NOTE'S LETTER, AND WHERE IT STANDS. The letters are the panel's own, in
// the order the notes stand; and a letter is drawn directly before a word, so
// an offset the decoder wrote ahead of a line break or a space moves past it.
func TestANotesLetterAndWhereItStands(t *testing.T) {
	for index, want := range map[int]string{0: "a", 1: "b", 6: "g", 25: "z", 26: "aa", 27: "ab", 51: "az", 52: "ba", -1: ""} {
		if got := noteLetter(index); got != want {
			t.Errorf("note %d has the letter %q, want %q", index, got, want)
		}
	}
	text := []rune("And Hannah prayed:\n“My heart rejoices")
	for anchor, want := range map[int]int{
		0:         0,         // before the first word
		4:         4,         // directly before a word
		3:         4,         // before the space: after it
		18:        19,        // at the end of the line: at the head of the next
		6:         6,         // inside a word, where the feed ran two together: where it is
		len(text): len(text), // the end
		-2:        0,
		99:        len(text),
	} {
		if got := noteMarkAt(text, anchor); got != want {
			t.Errorf("a note at %d is drawn at %d, want %d", anchor, got, want)
		}
	}
}

// markedWords reads a passage's runs back as its letters, each with the word
// that follows it on its row: "a>prayed". A verse's number is "1>" and its
// word. A letter with a row break between it and the next word says so.
func markedWords(runs []cardRun) []string {
	var out []string
	for i, r := range runs {
		if !r.Mark {
			continue
		}
		word := ""
		for _, next := range runs[i+1:] {
			if next.Mark {
				continue
			}
			if next.Break {
				word = "(a new row) " // the letter is left at the end of a row, away from its word
			}
			if f := strings.Fields(next.Text); len(f) > 0 {
				word += f[0]
				break
			}
		}
		out = append(out, r.Text+">"+word)
	}
	return out
}

// THE PASSAGE IS THE SELECTED TEXT WITH A LETTER BEFORE EACH NOTE'S WORD, and
// setting the letters moves none of it.
func TestThePassageIsTheSelectedTextWithALetterBeforeEachNotesWord(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	licenseNKJVForTest(t)
	withPublisherCrossRefs(t, "nkjv", true)
	bd := xrefBible(map[string]map[int]int{"Psalms": {51: 3}, "Romans": {5: 21}, "Isaiah": {9: 21, 43: 28}, "2 Samuel": {12: 31}})
	x := footnoteKindCrossref
	title := Superscription{Text: "A fixture psalm when the prophet came.", Footnotes: []Footnote{
		{Anchor: 16, Text: "2 Sam. 12:1", Kind: x, Caller: "-"}, // before "when"
	}}
	bd.Superscriptions = map[string]map[int]Superscription{"Psalms": {51: title}}
	v1 := &bd.Verses["Psalms"][51][0]
	v1.Text = "Have mercy upon me, O God,\nAccording to the fixture's kindness;"
	v1.Supplied = []TextSpan{{Start: 40, End: 43}} // "the"
	v1.Footnotes = []Footnote{
		{Anchor: 5, Text: "Rom. 5:8", Kind: x, Caller: "-"},        // directly before "mercy"
		{Anchor: 26, Text: "(Is. 9:6)", Kind: x, Caller: "-"},      // at the end of the first line: before "According"
		{Anchor: 43, Text: "Is. 43:25; 9:6", Kind: x, Caller: "-"}, // before the space: before "fixture's"
		{Anchor: 54, Text: "A translator's remark.", Caller: "+"},  // not a cross-reference: no letter
	}
	st := &AppState{Bible: bd, CurrentBook: "Psalms", CurrentChapter: 51, CurrentVersion: "nkjv"}
	sel := versesBetween(bd, "Psalms", 51, 1, 2)
	notes := publisherCrossRefsFor(bd, "Psalms", 51, sel)
	if len(notes) != 4 {
		t.Fatalf("%d notes, want the title's and the verse's three", len(notes))
	}
	runs := passageRuns(publisherPassageUnits(st, "Psalms", 51, sel, notes))

	// The letters taken out, the passage is the title and the two verses,
	// rune for rune, each on rows of its own.
	want := title.Text + "\n" + v1.Text + "\n" + sel[1].Text
	if got := passagePlainText(runs); got != want {
		t.Errorf("the passage without its letters is\n%q\nwant\n%q", got, want)
	}
	// Each letter directly before its word, in order; the verses numbered,
	// since the passage holds more than the one verse.
	if got := joined(markedWords(runs)); got != "a>when|1>Have|a>mercy|b>According|c>fixture's|2>Synthetic" {
		t.Errorf("letters and their words: %s", got)
	}
	// The supplied word is set in italic, as the page sets it.
	italic := ""
	for _, r := range runs {
		if r.Italic {
			italic += r.Text
		}
	}
	if italic != "the" {
		t.Errorf("the italic runs read %q, want the supplied word alone", italic)
	}
	// The list is headed by it, and each note carries the letter its word has.
	lst := buildCrossRefList(st, sel, nil, nil, lightPalette, func(crossRef) {})
	p := passageIn(lst.Objects[:1])
	if !lst.Passage || p == nil {
		t.Fatalf("the passage does not head the list")
	}
	if got := passagePlainText(p.runs); got != want {
		t.Errorf("the list's passage without its letters is %q", got)
	}
	if got := sectionLabelsIn(lst.Objects); joined(got) != "NKJV REFERENCES · TITLE|NKJV REFERENCES · VERSE 1" {
		t.Errorf("labels = %q", got)
	}
	var letters []string
	for _, c := range cardsIn(lst.Objects) {
		if l := cardLetter(c); l != "" {
			letters = append(letters, l+" "+cardReference(c))
		}
	}
	if got := joined(letters); got != "a 2 Samuel 12:1|a Romans 5:8" {
		t.Errorf("the cards that are a whole note carry %q", got)
	}
	if !hasLine(lst.Objects, "b  (Is. 9:6)") || !hasLine(lst.Objects, "c  Is. 43:25; 9:6") {
		t.Errorf("the group and the note of two citations do not carry b and c:\n%s", joinedTexts(lst.Objects))
	}

	// DRAWN: a verse's number is the page's own, in the bold cut and the
	// page's colour for one, and a note's letter is in the text's cut and
	// the accent; both at the size and the lift of the page's superscript,
	// measured baseline to baseline.
	if readingMarkScale != styledNumRatio || readingMarkLift != styledNumLift {
		t.Errorf("a mark is %v of the text and raised %v of it; the page's verse number is %v and %v", readingMarkScale, readingMarkLift, styledNumRatio, styledNumLift)
	}
	if passageNumeralFace() == p.regular || p.number != lightPalette.VerseNumber || p.mark != lightPalette.Accent {
		t.Fatalf("the passage's number is not set apart from its text: the same cut %v, colours %v and %v", passageNumeralFace() == p.regular, p.number, p.mark)
	}
	p.Resize(fyne.NewSize(300, 600))
	r := test.WidgetRenderer(p).(*readingParagraphRenderer)
	r.Layout(p.Size())
	drv := fyne.CurrentApp().Driver()
	_, bodyBase := drv.RenderedTextSize("Ag", p.size, fyne.TextStyle{}, p.regular)
	var drawn []string
	for ri, row := range r.rows {
		var word *canvas.Text
		for _, tx := range row {
			if tx.TextSize == p.size {
				word = tx
				break
			}
		}
		for _, tx := range row {
			if tx.TextSize == p.size {
				continue
			}
			kind := "neither"
			switch {
			case tx.FontSource == passageNumeralFace() && tx.Color == p.number:
				kind = "number"
			case tx.FontSource == p.regular && tx.Color == p.mark:
				kind = "letter"
			}
			drawn = append(drawn, kind+" "+tx.Text)
			if word == nil {
				t.Errorf("the mark %q stands on row %d with no word", tx.Text, ri)
				continue
			}
			_, base := drv.RenderedTextSize(tx.Text, tx.TextSize, fyne.TextStyle{}, tx.FontSource)
			lift := (word.Position().Y + bodyBase) - (tx.Position().Y + base)
			if want := p.size * readingMarkLift; tx.TextSize != p.size*readingMarkScale || lift < want-0.01 || lift > want+0.01 {
				t.Errorf("the mark %q is %v points with its baseline %v above the words': want %v points and %v", tx.Text, tx.TextSize, lift, p.size*readingMarkScale, want)
			}
		}
	}
	if got := joined(drawn); got != "letter a|number 1|letter a|letter b|letter c|number 2" {
		t.Errorf("the passage's marks are drawn as %s", got)
	}

	// CONTROL: one verse and no title: no number, and the letters start at a.
	bd.Superscriptions = nil
	one := sel[:1]
	runs = passageRuns(publisherPassageUnits(st, "Psalms", 51, one, publisherCrossRefsFor(bd, "Psalms", 51, one)))
	if got := joined(markedWords(runs)); got != "a>mercy|b>According|c>fixture's" {
		t.Errorf("control: letters and their words: %s", got)
	}
	// CONTROL: a selection with no note has no passage.
	none := versesBetween(bd, "Psalms", 51, 2, 3)
	if lst := buildCrossRefList(st, none, []crossRef{{Book: "Romans", Chapter: 5, Verse: 8, Votes: 3}}, nil, lightPalette, func(crossRef) {}); lst.Passage || passageIn(lst.Objects) != nil {
		t.Errorf("control: a selection with no note is headed by a passage")
	}
}

// A HEADING'S NOTE STANDS WITH THE VERSE IT IS SET BEFORE. The NKJV has one,
// on the heading before Numbers 4:21, and it reached no reader while the list
// read a verse's notes alone.
func TestAHeadingsNoteStandsWithTheVerseItIsSetBefore(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	licenseNKJVForTest(t)
	withPublisherCrossRefs(t, "nkjv", true)
	bd := xrefBible(map[string]map[int]int{"Numbers": {3: 51, 4: 49}})
	x := footnoteKindCrossref
	bd.Headings = map[string]map[int][]Heading{"Numbers": {4: {
		{Text: "Duties of the fixture's sons", Style: "s", BeforeVerse: 21, Footnotes: []Footnote{{Text: "Num. 3:22", Kind: x, Caller: "-"}}},
		{Text: "A heading with no note", Style: "s", BeforeVerse: 29},
	}}}
	v21 := &bd.Verses["Numbers"][4][20]
	v21.Footnotes = []Footnote{{Anchor: 10, Text: "Num. 3:25", Kind: x, Caller: "-"}}
	st := &AppState{Bible: bd, CurrentBook: "Numbers", CurrentChapter: 4, CurrentVersion: "nkjv"}
	sel := versesBetween(bd, "Numbers", 4, 21, 21)

	notes := publisherCrossRefsFor(bd, "Numbers", 4, sel)
	if len(notes) != 2 || !notes[0].Heading || notes[0].Verse != 21 || notes[0].Index != 0 || notes[1].Heading || notes[1].Index != 1 {
		t.Fatalf("notes = %+v; want the heading's first and the verse's after it, lettered together", notes)
	}
	runs := passageRuns(publisherPassageUnits(st, "Numbers", 4, sel, notes))
	if got := passagePlainText(runs); got != "Duties of the fixture's sons\n"+v21.Text {
		t.Errorf("the passage is %q; want the heading and then the verse", got)
	}
	if got := joined(markedWords(runs)); got != "a>Duties|b>text" {
		t.Errorf("letters and their words: %s", got)
	}
	for _, r := range runs {
		if strings.Contains(r.Text, "Duties") && !r.Dim {
			t.Errorf("the heading is not set muted")
		}
	}
	lst := buildCrossRefList(st, sel, nil, nil, lightPalette, func(crossRef) {})
	if got := cardLabelsIn(lst.Objects); joined(got) != "Numbers 3:22|Numbers 3:25" || lst.PublisherNotes != 2 {
		t.Errorf("cards = %q, %d notes; want the heading's note and the verse's", got, lst.PublisherNotes)
	}
	if got := sectionLabelsIn(lst.Objects); joined(got) != "NKJV REFERENCES" {
		t.Errorf("labels = %q", got)
	}

	// CONTROL: the verse after a heading that carries no note has only its own.
	sel = versesBetween(bd, "Numbers", 4, 29, 29)
	if notes := publisherCrossRefsFor(bd, "Numbers", 4, sel); len(notes) != 0 {
		t.Errorf("control: %d notes on a verse whose heading has none", len(notes))
	}
}

// A NOTE OF SEVERAL PIECES IS A ROW THAT CARRIES IT AS PRINTED AND FOLDS.
func TestANoteOfSeveralPiecesFoldsToItsPrintedLine(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	licenseNKJVForTest(t)
	withPublisherCrossRefs(t, "nkjv", true)
	bd := xrefBible(map[string]map[int]int{"Judges": {9: 57}, "1 Samuel": {16: 23, 18: 30, 19: 24}, "1 Kings": {22: 53}})
	printed := "Judg. 9:23; 1 Sam. 16:15, 16; 18:10; 19:9; 1 Kin. 22:19–22"
	v := &bd.Verses["1 Samuel"][16][15]
	v.Footnotes = []Footnote{{Anchor: 10, Text: printed, Kind: footnoteKindCrossref, Caller: "-"}}
	st := &AppState{Bible: bd, CurrentBook: "1 Samuel", CurrentChapter: 16, CurrentVersion: "nkjv"}
	var followed []crossRef
	lst := buildCrossRefList(st, []Verse{*v}, nil, nil, lightPalette, func(c crossRef) { followed = append(followed, c) })
	relaid := 0
	lst.Relayout.fn = func() { relaid++ }

	rows := noteRowsIn(lst.Objects)
	if len(rows) != 1 || lst.PublisherNotes != 1 || lst.PublisherCards != 5 || lst.PublisherLines != 1 {
		t.Fatalf("%d note rows; list = %+v; want one note of five cards and the reader's own verse", len(rows), lst)
	}
	row := rows[0]
	// Open when the list is built, the printed line on the row.
	if !row.open || !row.body.Visible() || row.foot.Visible() {
		t.Fatalf("the note is not open when the list is built: open %v, pieces visible %v, closing rule visible %v", row.open, row.body.Visible(), row.foot.Visible())
	}
	if got := joined(textsIn(row.label)); got != "a  "+printed {
		t.Errorf("the row reads %q; want the letter and the note as printed", got)
	}
	// The pieces in print order: the reader's own verse third, in full, with
	// nothing to tap; the rest cards.
	var pieces []string
	for _, o := range row.body.Objects {
		taps := cardsIn([]fyne.CanvasObject{o})
		switch {
		case len(taps) == 1:
			pieces = append(pieces, "card:"+cardReference(taps[0]))
		case len(taps) == 0:
			pieces = append(pieces, "shown:"+joined(textsIn(o)))
		default:
			pieces = append(pieces, fmt.Sprintf("%d taps", len(taps)))
		}
	}
	if got := joined(pieces); got != "card:Judges 9:23|card:1 Samuel 16:15|shown:1 Samuel 16:16|card:1 Samuel 18:10|card:1 Samuel 19:9|card:1 Kings 22:19–22" {
		t.Errorf("pieces = %s", got)
	}
	openHeight := row.MinSize().Height

	// A tap on the row closes it to its printed line.
	test.Tap(row.header)
	if row.open || row.body.Visible() || !row.foot.Visible() || relaid != 1 {
		t.Fatalf("a tap did not close the note: open %v, pieces visible %v, closing rule visible %v, %d layouts", row.open, row.body.Visible(), row.foot.Visible(), relaid)
	}
	if got := joined(textsIn(row.label)); got != "a  "+printed {
		t.Errorf("closed, the row reads %q", got)
	}
	if row.MinSize().Height >= openHeight {
		t.Errorf("closed, the row is %v tall, no shorter than open (%v)", row.MinSize().Height, openHeight)
	}
	// And a second opens it again, with the pieces it had.
	test.Tap(row.header)
	if !row.open || !row.body.Visible() || row.foot.Visible() || relaid != 2 || row.MinSize().Height != openHeight {
		t.Errorf("a second tap did not open the note again: open %v, %d layouts, height %v (was %v)", row.open, relaid, row.MinSize().Height, openHeight)
	}
	// A card under the note is followed as any card is.
	test.Tap(cardsIn(lst.Objects)[1])
	if len(followed) != 1 || followed[0] != (crossRef{Book: "1 Samuel", Chapter: 16, Verse: 15}) {
		t.Errorf("tapping the note's second card followed %+v", followed)
	}
	if b := bannedIn(lst.Objects); len(b) != 0 {
		t.Errorf("the list holds %v", b)
	}
}

// A LETTER IS DRAWN BEFORE A WORD, NEVER IN IT. The feed sets three callers
// inside a word; each is drawn at the head of its word. A caller between two
// words the feed ran together stands where it is.
func TestACallerInsideAWordIsDrawnBeforeTheWord(t *testing.T) {
	bd := xrefBible(map[string]map[int]int{"Genesis": {42: 38}, "Romans": {5: 21}})
	note := func(anchor int) []Footnote {
		return []Footnote{{Anchor: anchor, Text: "Rom. 5:8", Kind: footnoteKindCrossref, Caller: "-"}}
	}
	// "Synthetic text of Genesis 42:20.": offset 20 is the third letter of
	// "Genesis", which begins at 18.
	bd.Verses["Genesis"][42][19].Footnotes = note(20)
	n := publisherCrossRefsFor(bd, "Genesis", 42, versesBetween(bd, "Genesis", 42, 20, 20))[0]
	if n.Anchor != 20 || n.Mark != 18 {
		t.Errorf("the named caller stands at %d and is drawn at %d; want 20 and the head of its word, 18", n.Anchor, n.Mark)
	}
	// CONTROL: the same offset in a verse that is not named is drawn where
	// it stands: 257 callers stand between two words the feed ran together.
	bd.Verses["Genesis"][42][20].Footnotes = note(20)
	n = publisherCrossRefsFor(bd, "Genesis", 42, versesBetween(bd, "Genesis", 42, 21, 21))[0]
	if n.Anchor != 20 || n.Mark != 20 {
		t.Errorf("control: an unnamed caller stands at %d and is drawn at %d; want 20 and 20", n.Anchor, n.Mark)
	}
	// CONTROL: a named caller that no longer stands inside a word (a feed
	// put right) is drawn where it stands.
	bd.Verses["Genesis"][42][19].Text = "Synthetic text of a Genesis 42:20."
	bd.Verses["Genesis"][42][19].Footnotes = note(20)
	n = publisherCrossRefsFor(bd, "Genesis", 42, versesBetween(bd, "Genesis", 42, 20, 20))[0]
	if n.Mark != 20 {
		t.Errorf("control: a named caller before a word is drawn at %d, want where it stands, 20", n.Mark)
	}
}

// A BASIS IS ITS WHOLE CITATION'S. "Ps. 45:4, 5, LXX" names the Septuagint
// for both verses; the tag used to ride the last alone.
func TestABasisAfterAListOfVersesTagsEachOfThem(t *testing.T) {
	bd := xrefBible(map[string]map[int]int{"Psalms": {45: 17, 97: 12}, "Job": {13: 28}, "Revelation": {6: 17}})
	for _, c := range []struct{ text, want string }{
		{"Ps. 45:4, 5, LXX", "card:Psalms 45:4 [LXX]|card:Psalms 45:5 [LXX]"},
		{"Job 13:16, LXX; Ps. 97:7", "card:Job 13:16 [LXX]|card:Psalms 97:7"},
		{"Ps. 97:7; 45:4, 5, LXX, DSS", "card:Psalms 97:7|card:Psalms 45:4 [LXX, DSS]|card:Psalms 45:5 [LXX, DSS]"},
		{"(Ps. 45:4, 5, LXX); Job 13:16", "group:(Ps. 45:4, 5, LXX)[card:Psalms 45:4 [LXX] card:Psalms 45:5 [LXX]]|card:Job 13:16"},
		// CONTROL: a semicolon ends the citation: the basis does not cross it.
		{"Ps. 45:4; 45:5, LXX", "card:Psalms 45:4|card:Psalms 45:5 [LXX]"},
	} {
		v := versesBetween(bd, "Revelation", 6, 2, 2)
		v[0].Footnotes = []Footnote{{Text: c.text, Kind: footnoteKindCrossref, Caller: "-"}}
		n := publisherCrossRefsFor(bd, "Revelation", 6, v)[0]
		if got := joined(pieceNames(n.Pieces)); got != c.want {
			t.Errorf("%q\n got %s\nwant %s", c.text, got, c.want)
		}
		for _, problem := range noteGapProblems(n) {
			t.Errorf("%q: %s", c.text, problem)
		}
	}
}
