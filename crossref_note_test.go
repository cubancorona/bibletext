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

// noteShape names what a note is: one citation, one parenthesised group,
// words alone, or several pieces. The first is drawn as a card (or, where it
// cites the reader's own verse, as its reference); the others as a row that
// folds, or as their words where nothing in them can be followed.
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
		{"a pericope range on its opening verse", "Matthew", 6, 9, "Matt. 6:9–13; Luke 11:2–4", nil,
			"card:Matthew 6:9–13|card:Luke 11:2–4", "fold"},
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
			"card:Psalms 135:1–3|card:Psalms 136:1", "fold"},
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
			t.Errorf("%s: the note is a %s, want a %s", c.name, got, c.shape)
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
	// Each letter directly before its word, continuously across the title and
	// verses; the verses numbered, since the passage holds more than one verse.
	if got := joined(markedWords(runs)); got != "a>when|1>Have|b>mercy|c>According|d>fixture's|2>Synthetic" {
		t.Errorf("letters and their words: %s", got)
	}
	// The Psalm's title and the supplied word are set in italic, as the
	// page sets them, and nothing else is.
	italic := ""
	for _, r := range runs {
		if r.Italic {
			italic += r.Text
		}
	}
	if italic != title.Text+"the" {
		t.Errorf("the italic runs read %q, want the title and the supplied word", italic)
	}
	// The list is headed by it, and each note carries the letter its word has,
	// all under one NKJV REFERENCES label.
	lst := buildCrossRefList(st, sel, nil, nil, lightPalette, func(crossRef) {})
	p := passageIn(lst.Objects[:1])
	if !lst.Passage || p == nil {
		t.Fatalf("the passage does not head the list")
	}
	if got := passagePlainText(p.runs); got != want {
		t.Errorf("the list's passage without its letters is %q", got)
	}
	if got := sectionLabelsIn(lst.Objects); joined(got) != "NKJV REFERENCES" {
		t.Errorf("labels = %q", got)
	}
	var letters []string
	for _, c := range cardsIn(lst.Objects) {
		if l := cardLetter(c); l != "" {
			letters = append(letters, l+" "+cardReference(c))
		}
	}
	if got := joined(letters); got != "a 2 Samuel 12:1|b Romans 5:8" {
		t.Errorf("the cards that are a whole note carry %q", got)
	}
	if got := noteHeadsIn(lst.Objects); joined(got) != "d  Is. 43:25; 9:6" {
		t.Errorf("the note of two citations reads %q; want it under d:\n%s", got, joinedTexts(lst.Objects))
	}
	if groups := groupRowsIn(lst.Objects); len(groups) != 1 || groups[0].open || groups[0].letter == nil || groups[0].letter.Text+"  "+joined(textsIn(groups[0].label)) != "c  (Is. 9:6)" {
		t.Errorf("the note that is one group: %d group rows; want c  (Is. 9:6), closed", len(groups))
	}
	// Verse 1 selected alone with its title: one bare NKJV REFERENCES label,
	// verse 1 numbered after the title, and continuous letters a..d across both.
	alone := buildCrossRefList(st, sel[:1], nil, nil, lightPalette, func(crossRef) {})
	if got := sectionLabelsIn(alone.Objects); joined(got) != "NKJV REFERENCES" {
		t.Errorf("Psalm 51:1 alone labels = %q, want NKJV REFERENCES", got)
	}
	if ap := passageIn(alone.Objects[:1]); ap == nil || joined(markedWords(ap.runs)) != "a>when|1>Have|b>mercy|c>According|d>fixture's" {
		t.Errorf("Psalm 51:1 alone passage marks = %v", ap)
	}

	// Both keys use the source groups' accent, with distinct upright numeric
	// and bold-italic letter faces, and a raised baseline measured below.
	if p.marks == nil || p.marks.number == p.marks.letter || p.number != lightPalette.Accent || p.mark != lightPalette.Accent {
		t.Fatalf("the passage's markers lack distinct faces or matching accent")
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
			case tx.FontSource == p.marks.number && tx.Color == p.number:
				kind = "number"
			case tx.FontSource == p.marks.letter && tx.Color == p.mark:
				kind = "letter"
			}
			drawn = append(drawn, kind+" "+tx.Text)
			if word == nil {
				t.Errorf("the mark %q stands on row %d with no word", tx.Text, ri)
				continue
			}
			_, base := drv.RenderedTextSize(tx.Text, tx.TextSize, fyne.TextStyle{}, tx.FontSource)
			lift := (word.Position().Y + bodyBase) - (tx.Position().Y + base)
			if want := p.marks.lift; tx.TextSize != p.marks.size || lift < want-0.01 || lift > want+0.01 {
				t.Errorf("the mark %q is %v points with its baseline %v above the words': want %v points and %v", tx.Text, tx.TextSize, lift, p.marks.size, want)
			}
		}
	}
	if got := joined(drawn); got != "letter a|number 1|letter b|letter c|letter d|number 2" {
		t.Errorf("the passage's marks are drawn as %s", got)
	}

	// CONTROL: one verse and no title: no number, and the letters start at a.
	bd.Superscriptions = nil
	one := sel[:1]
	runs = passageRuns(publisherPassageUnits(st, "Psalms", 51, one, publisherCrossRefsFor(bd, "Psalms", 51, one)))
	if got := joined(markedWords(runs)); got != "a>mercy|b>According|c>fixture's" {
		t.Errorf("control: letters and their words: %s", got)
	}
	// CONTROL: a selection with no publisher note retains its plain excerpt.
	none := versesBetween(bd, "Psalms", 51, 2, 3)
	lst = buildCrossRefList(st, none, []crossRef{{Book: "Romans", Chapter: 5, Verse: 8, Votes: 3}}, nil, lightPalette, func(crossRef) {})
	if !lst.Passage {
		t.Fatal("control: a selection with no publisher note lost its passage")
	}
	requirePlainCrossRefExcerpt(t, passageIn(lst.Objects), none)
}

func requirePlainCrossRefExcerpt(t *testing.T, passage *readingParagraph, selected []Verse) {
	t.Helper()
	if passage == nil {
		t.Fatal("the populated reference list lost its selected-passage excerpt")
	}
	var text, wantNumbers, numbers []string
	for _, verse := range selected {
		text = append(text, verse.Text)
		if len(selected) > 1 {
			wantNumbers = append(wantNumbers, fmt.Sprint(verse.Verse))
		}
	}
	if passagePlainText(passage.runs) != strings.Join(text, "\n") {
		t.Fatal("the plain excerpt changed or omitted selected text, or repeated a heading")
	}
	for _, run := range passage.runs {
		if run.Mark {
			if !run.Dim {
				t.Fatal("an excerpt without word anchors acquired a publisher letter")
			}
			numbers = append(numbers, run.Text)
		}
	}
	if joined(numbers) != joined(wantNumbers) {
		t.Fatalf("excerpt verse numbers = %v, want %v", numbers, wantNumbers)
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
		if strings.Contains(r.Text, "Duties") && (!r.Bold || r.Dim) {
			t.Errorf("the heading is not set bold in full colour: Bold=%v Dim=%v", r.Bold, r.Dim)
		}
	}
	lst := buildCrossRefList(st, sel, nil, nil, lightPalette, func(crossRef) {})
	if got := cardLabelsIn(lst.Objects); joined(got) != "Numbers 3:22|Numbers 3:25" || lst.PublisherNotes != 2 {
		t.Errorf("cards = %q, %d notes; want the heading's note and the verse's", got, lst.PublisherNotes)
	}
	if got := sectionLabelsIn(lst.Objects); joined(got) != "NKJV REFERENCES" {
		t.Errorf("labels = %q, want NKJV REFERENCES", got)
	}
	// Drawn: the heading line uses the bold cut and the body text colour, and
	// a single-verse selection keeps its verse unnumbered despite the heading.
	hp := passageIn(lst.Objects[:1])
	hp.Resize(fyne.NewSize(300, 300))
	hr := test.WidgetRenderer(hp).(*readingParagraphRenderer)
	hr.Layout(hp.Size())
	sawBoldHeading := false
	for _, row := range hr.rows {
		for _, tx := range row {
			if strings.Contains(tx.Text, "Duties") {
				sawBoldHeading = true
				if tx.FontSource != passageNumeralFace() || tx.Color != lightPalette.Text {
					t.Errorf("heading fragment drawn with face=%v color=%v, want bold face and %v", tx.FontSource, tx.Color, lightPalette.Text)
				}
			}
		}
	}
	if !sawBoldHeading {
		t.Errorf("heading text was not drawn in the passage")
	}
	// When only the heading has a note (as on Numbers 4:21 in the NKJV), its
	// section label is the single unkeyed NKJV REFERENCES both alone and in a range.
	v21.Footnotes = nil
	for _, span := range [][]Verse{versesBetween(bd, "Numbers", 4, 21, 21), versesBetween(bd, "Numbers", 4, 20, 22)} {
		onlyHeading := buildCrossRefList(st, span, nil, nil, lightPalette, func(crossRef) {})
		if got := sectionLabelsIn(onlyHeading.Objects); joined(got) != "NKJV REFERENCES" {
			t.Errorf("%d verses around Numbers 4:21: labels = %q, want NKJV REFERENCES", len(span), got)
		}
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
	if !row.open || !row.body.Visible() {
		t.Fatalf("the note is not open when the list is built: open %v, pieces visible %v", row.open, row.body.Visible())
	}
	if got := row.letter.Text + "  " + joined(textsIn(row.label)); got != "a  "+printed {
		t.Errorf("the row reads %q; want the letter and the note as printed", got)
	}
	// The pieces in print order: the reader's own verse third, in full, with
	// nothing to tap; the rest cards.
	var pieces []string
	for _, o := range row.pieces {
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
	// The reader's own verse is set as a line of the edition's print, muted
	// and at that size: in a card's bold it read as a heading over the
	// cards after it.
	own := 0
	walkTree(row.pieces[2], func(o fyne.CanvasObject) {
		if tx, ok := o.(*canvas.Text); ok && tx.Text == "1 Samuel 16:16" {
			own++
			if tx.TextStyle.Bold || tx.TextSize != crossRefGroupTextSize || tx.Color != lightPalette.TextMuted {
				t.Errorf("the reader's own verse is set bold %v at %v points in %v", tx.TextStyle.Bold, tx.TextSize, tx.Color)
			}
		}
	})
	if own != 1 {
		t.Errorf("the reader's own verse stands %d times in its row", own)
	}
	openHeight := row.MinSize().Height

	// A tap on the row closes it to its printed line.
	test.Tap(row.header)
	if row.open || row.body.Visible() || relaid != 1 {
		t.Fatalf("a tap did not close the note: open %v, pieces visible %v, %d layouts", row.open, row.body.Visible(), relaid)
	}
	if got := row.letter.Text + "  " + joined(textsIn(row.label)); got != "a  "+printed {
		t.Errorf("closed, the row reads %q", got)
	}
	if row.MinSize().Height >= openHeight {
		t.Errorf("closed, the row is %v tall, no shorter than open (%v)", row.MinSize().Height, openHeight)
	}
	// And a second opens it again, with the pieces it had.
	test.Tap(row.header)
	if !row.open || !row.body.Visible() || relaid != 2 || row.MinSize().Height != openHeight {
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

// CONTINUOUS LETTERS PAST Z USE SPREADSHEET ORDER AND FIT THE 18PT MARGIN.
func TestContinuousLettersPastTwentySixFitTheMargin(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	licenseNKJVForTest(t)
	withPublisherCrossRefs(t, "nkjv", true)
	bd := xrefBible(map[string]map[int]int{"Amos": {2: 4}, "Romans": {5: 21}})
	for v := 1; v <= 4; v++ {
		verse := &bd.Verses["Amos"][2][v-1]
		verse.Text = "Alpha beta gamma delta epsilon zeta eta."
		var notes []Footnote
		for j := range 7 {
			notes = append(notes, Footnote{Anchor: j * 6, Text: "Rom. 5:8", Kind: footnoteKindCrossref, Caller: "-"})
		}
		verse.Footnotes = notes
	}
	sel := versesBetween(bd, "Amos", 2, 1, 4)
	notes := publisherCrossRefsFor(bd, "Amos", 2, sel)
	if len(notes) != 28 {
		t.Fatalf("got %d notes, want 28", len(notes))
	}
	if got, want := noteLetter(notes[0].Index), "a"; got != want {
		t.Errorf("first letter = %q, want %q", got, want)
	}
	if got, want := noteLetter(notes[7].Index), "h"; got != want {
		t.Errorf("eighth letter (head of verse 2) = %q, want %q", got, want)
	}
	if got, want := noteLetter(notes[25].Index), "z"; got != want {
		t.Errorf("26th letter = %q, want %q", got, want)
	}
	if got, want := noteLetter(notes[26].Index), "aa"; got != want {
		t.Errorf("27th letter = %q, want %q", got, want)
	}
	if got, want := noteLetter(notes[27].Index), "ab"; got != want {
		t.Errorf("28th letter = %q, want %q", got, want)
	}

	single := crossRefLetter("a", lightPalette)
	double := crossRefLetter("ww", lightPalette)
	if single.TextSize != crossRefLetterSize || double.TextSize != crossRefDoubleLetterSize || double.TextSize >= single.TextSize {
		t.Errorf("single size = %v, double size = %v; want %v and %v", single.TextSize, double.TextSize, crossRefLetterSize, crossRefDoubleLetterSize)
	}
	wSize, _ := fyne.CurrentApp().Driver().RenderedTextSize(double.Text, double.TextSize, double.TextStyle, double.FontSource)
	if wSize.Width >= crossRefLetterMargin-1 {
		t.Errorf("widest double letter %q renders %.2fpt wide, leaving no gap inside %vpt margin", double.Text, wSize.Width, crossRefLetterMargin)
	}
}

// PASSAGE LETTERING TOUCHES (S20 Item 20): when holdFlush is set (the
// cross-references passage), a raised letter flush after an italic supplied
// word takes readingMarkGap before it so the slanted glyph does not touch the
// upright letter, and a standalone opening quotation mark after a raised
// letter stays attached across the space to the word that follows rather than
// stranding at the right end of a row. When holdFlush is false (the Verse of
// the Day card), neither rule alters layout.
func TestPassageLetteringTouchesItalicGapAndOpeningQuote(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	// 1. Air after an italic supplied word before a flush raised letter.
	italicRuns := []cardRun{
		{Text: "“He", Italic: true},
		{Text: "a", Mark: true},
		{Text: "is a tower of salvation", Italic: false},
	}
	pFlush := newReadingParagraph(italicRuns, 17, lightPalette.Text, lightPalette.RedLetter, styledPaneFont(), cardItalicFont())
	pFlush.holdFlush = true
	rFlush := test.WidgetRenderer(pFlush).(*readingParagraphRenderer)
	rFlush.Layout(fyne.NewSize(300, 100))
	if len(rFlush.rows) == 0 || len(rFlush.rows[0]) < 3 {
		t.Fatalf("rows = %v, want at least 3 fragments on row 0", len(rFlush.rows))
	}
	heFrag, markFrag := rFlush.rows[0][0], rFlush.rows[0][1]
	gotGap := markFrag.Position().X - (heFrag.Position().X + heFrag.Size().Width)
	if d := gotGap - readingMarkGap; d < -0.05 || d > 0.05 {
		t.Errorf("gap between italic word and flush mark = %.2f, want %.2f", gotGap, readingMarkGap)
	}
	// Lookahead accounts for readingMarkGap so a tight width wraps cleanly
	// instead of overrunning by readingMarkGap.
	prefixRuns := []cardRun{
		{Text: "Mercy "},
		{Text: "He", Italic: true},
		{Text: "a", Mark: true},
		{Text: "shows"},
	}
	pTight := newReadingParagraph(prefixRuns, 17, lightPalette.Text, lightPalette.RedLetter, styledPaneFont(), cardItalicFont())
	pTight.holdFlush = true
	rTight := test.WidgetRenderer(pTight).(*readingParagraphRenderer)
	rTight.Layout(fyne.NewSize(500, 100))
	oneLineW := rTight.rowWidths()[0]
	// Force width 0.5pt narrower than the single-line width: the cluster must
	// wrap onto row 2 rather than overrun by readingMarkGap.
	rTight.Layout(fyne.NewSize(oneLineW-0.5, 100))
	for ri, rw := range rTight.rowWidths() {
		if rw > oneLineW-0.5+0.01 {
			t.Errorf("row %d width %.2f exceeds tight limit %.2f", ri, rw, oneLineW-0.5)
		}
	}

	// Control: holdFlush == false leaves gap == 0 after italic before flush mark.
	pPlain := newReadingParagraph(italicRuns, 17, lightPalette.Text, lightPalette.RedLetter, styledPaneFont(), cardItalicFont())
	rPlain := test.WidgetRenderer(pPlain).(*readingParagraphRenderer)
	rPlain.Layout(fyne.NewSize(300, 100))
	plainGap := rPlain.rows[0][1].Position().X - (rPlain.rows[0][0].Position().X + rPlain.rows[0][0].Size().Width)
	if plainGap != 0 {
		t.Errorf("control (holdFlush=false): gap = %.2f, want 0", plainGap)
	}

	// 2. Standalone opening quote mark after a raised letter ("“ ‘So...") stays
	// with the word after its space rather than stranding at the end of row 0.
	quoteRuns := []cardRun{
		{Text: "Then Haggai answered and spoke, "},
		{Text: "a", Mark: true},
		{Text: "“ ‘So is this nation before Me,’"},
	}
	pQuote := newReadingParagraph(quoteRuns, 17, lightPalette.Text, lightPalette.RedLetter, styledPaneFont(), cardItalicFont())
	pQuote.holdFlush = true
	rQuote := test.WidgetRenderer(pQuote).(*readingParagraphRenderer)
	// Measure exact width up through `a“` on one row so `a“` alone would fit
	// while `a“ ‘So` does not.
	rQuote.Layout(fyne.NewSize(800, 100))
	var quoteEnd, nextEnd float32
	for _, tx := range rQuote.rows[0] {
		if tx.Text == "“" {
			quoteEnd = tx.Position().X + tx.Size().Width
		}
		if tx.Text == "‘So" {
			nextEnd = tx.Position().X + tx.Size().Width
		}
	}
	if quoteEnd == 0 || nextEnd <= quoteEnd {
		t.Fatalf("control: quoteEnd=%v nextEnd=%v", quoteEnd, nextEnd)
	}
	limit := (quoteEnd + nextEnd) / 2
	rQuote.Layout(fyne.NewSize(limit, 200))
	if len(rQuote.rows) < 2 {
		t.Fatalf("expected wrap at width %.1f, got %d row", limit, len(rQuote.rows))
	}
	lastOnFirst := rQuote.rows[0][len(rQuote.rows[0])-1].Text
	if lastOnFirst == "“" || lastOnFirst == "a" {
		t.Errorf("row 0 ended on %q, stranding the mark or opening quote from its word", lastOnFirst)
	}
	if len(rQuote.rows[1]) < 3 || rQuote.rows[1][0].Text != "a" || rQuote.rows[1][1].Text != "“" || rQuote.rows[1][2].Text != "‘So" {
		var got []string
		for _, tx := range rQuote.rows[1] {
			got = append(got, tx.Text)
		}
		t.Errorf("row 1 fragments = %q, want [\"a\" \"“\" \"‘So\" ...]", got)
	}
	// And the space between "“" and "‘So" is preserved on row 1.
	spaceBetween := rQuote.rows[1][2].Position().X - (rQuote.rows[1][1].Position().X + rQuote.rows[1][1].Size().Width)
	if spaceBetween <= 0.5 {
		t.Errorf("space between opening quotes on row 1 = %.2f, want > 0.5", spaceBetween)
	}
	// Even at a width narrower than `a“ ‘So` itself, `‘So` stays on the same
	// row as `a“` at the head of a row rather than breaking after `“`.
	rQuote.Layout(fyne.NewSize(35, 300))
	sawQuoteCluster := false
	for _, rw := range rQuote.rows {
		if len(rw) >= 3 && rw[0].Text == "a" && rw[1].Text == "“" && rw[2].Text == "‘So" {
			sawQuoteCluster = true
		}
	}
	if !sawQuoteCluster {
		t.Error("at narrow width 35, `‘So` broke away from `a“` at the start of its row")
	}
}
