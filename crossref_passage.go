package bibletext

// The selected text at the head of the publisher's notes — docs/SCRIPTURE_WORKLIST.md, S20.
//
// Decided 11 November 2026: the cross-references panel shows the selected
// text first, with a letter on each word a note of the edition hangs from,
// and the notes under it by those letters (crossref_note.go).
//
// The edition keys every cross-reference note to a word: in print a small
// raised letter stands before the word and the note's citations are listed
// under that letter. The feed keeps the place as the note's offset in its
// verse (Footnote.Anchor) and carries no letter (its caller is "-" on every
// note), so the letters here are the panel's own: a, b, c in the order the
// notes stand, restarting at each verse and at a Psalm's title
// (noteLetter). A section heading that carries a note (one in the NKJV,
// before Numbers 4:21) is shown before its verse, muted, and its notes are
// lettered with that verse's.
//
// The text is the verse's own, drawn as the page draws it (redLetterRuns):
// the divine name's small capitals, the translators' supplied words in
// italic, the words of Christ in red where the reader has that on, poem
// lines on rows of their own. A letter is set in the text and nowhere moves
// a rune of it: the passage with its letters taken out is the verses' text
// (passagePlainText, and the test that holds the two equal over a canon).
// A letter stands directly before its word, so one whose offset the decoder
// wrote ahead of a line break or a space is drawn after it (noteMarkAt).
//
// Decided 11 November 2026: the letter stands before its word, where the
// edition keys the note, and is raised above the line as the printed
// edition's letters are. It is set as the page sets a superscript, at the
// size and the lift of the page's verse numbers (readingMarkScale and
// readingMarkLift, verse_of_day.go), and a verse's number in the passage is
// the page's own: the bold cut, in the page's colour for one.

import (
	"sort"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
)

// crossRefPassageTextSize is the passage's size before the reading face's
// optical correction: the size the list's previews are set at.
const crossRefPassageTextSize = 17

// passageNumeralFace is the cut a verse's number is set in: the reading
// face's bold, as on the page (styledReadingPane.numeralFace).
func passageNumeralFace() fyne.Resource {
	if f := loadReadingFonts(); f != nil && f.bold != nil {
		return f.bold
	}
	return styledPaneFont()
}

// passageMark is one note's letter in a unit of the passage: the rune offset
// in the unit's text where it is drawn, and the letter.
type passageMark struct {
	At     int
	Letter string
}

// passageUnit is one stretch of the passage: a Psalm's title, a section
// heading that carries a note, or a verse.
type passageUnit struct {
	Number int        // the verse's number, set before it when the selection has several verses or a title stands before the verse; 0 for none
	Runs   []verseRun // the unit's text as the page draws it
	Dim    bool       // a heading: not Scripture, set muted
	Marks  []passageMark
}

// text is the unit's text as drawn, without its marks.
func (u passageUnit) text() string {
	var b strings.Builder
	for _, r := range u.Runs {
		b.WriteString(r.Text)
	}
	return b.String()
}

// publisherPassageUnits is the selection as the passage sets it: the Psalm's
// title when the notes include the title's, then each selected verse, a
// noted heading before the verse it precedes. notes are the selection's
// (publisherCrossRefsFor); each one's letter stands in its own unit.
func publisherPassageUnits(state *AppState, book string, chapter int, selected []Verse, notes []publisherCrossRef) []passageUnit {
	bd := state.Bible
	// The notes of one verse number in order: a heading's first, as
	// publisherCrossRefsFor hands them over.
	byVerse := map[int][]publisherCrossRef{}
	for _, n := range notes {
		byVerse[n.Verse] = append(byVerse[n.Verse], n)
	}
	marks := func(ns []publisherCrossRef) []passageMark {
		out := make([]passageMark, 0, len(ns))
		for _, n := range ns {
			if n.Unkeyed {
				continue // its caller is not in the feed: no word is known to be its own
			}
			out = append(out, passageMark{At: n.Mark, Letter: noteLetter(n.Index)})
		}
		return out
	}
	var units []passageUnit
	if title := byVerse[0]; len(title) > 0 {
		// A Psalm's title is set in italic, as the page sets it
		// (reading.go, p.pst; reading_styled_super.go).
		s := bd.SuperscriptionFor(book, chapter)
		units = append(units, passageUnit{Runs: []verseRun{{Text: s.DrawnText(), Italic: true}}, Marks: marks(title)})
	}
	versionID := state.currentVersion().ID
	for _, v := range selected {
		var onHeadings, onVerse []publisherCrossRef
		for _, n := range byVerse[v.Verse] {
			if n.Heading {
				onHeadings = append(onHeadings, n)
			} else {
				onVerse = append(onVerse, n)
			}
		}
		for _, h := range notedHeadingsBefore(bd, book, chapter, v.Verse) {
			// The heading's own notes are the next of the headings' notes,
			// one for each cross-reference note it carries.
			n := 0
			for _, fn := range h.Footnotes {
				if fn.Kind == footnoteKindCrossref && strings.TrimSpace(fn.Text) != "" {
					n++
				}
			}
			if n > len(onHeadings) {
				n = len(onHeadings)
			}
			units = append(units, passageUnit{Runs: []verseRun{{Text: h.DrawnText()}}, Dim: true, Marks: marks(onHeadings[:n])})
			onHeadings = onHeadings[n:]
		}
		u := passageUnit{Runs: redLetterRuns(versionID, v, redLetterEnabled()), Marks: marks(onVerse)}
		// A verse's number where the passage holds more than the one verse:
		// several verses, or a title before the first, whose letters would
		// otherwise read as that verse's.
		if len(selected) > 1 || len(units) > 0 && !units[len(units)-1].Dim {
			u.Number = v.Verse
		}
		units = append(units, u)
	}
	return units
}

// passageRuns is the units as the paragraph draws them: each unit on rows of
// its own, its number before it, its letters in its text at their offsets,
// and its poem lines broken where the text breaks them.
func passageRuns(units []passageUnit) []cardRun {
	var out []cardRun
	for ui, u := range units {
		first := true
		emit := func(r cardRun) {
			if first {
				r.Break = r.Break || ui > 0
				first = false
			}
			out = append(out, r)
		}
		text := func(run verseRun, piece []rune) {
			for li, line := range strings.Split(string(piece), "\n") {
				if line == "" && li == 0 {
					continue // nothing before the piece's first break
				}
				// A piece that ends on a break yields an empty last line
				// that carries it: what follows, a letter or the next run,
				// opens the next row.
				emit(cardRun{Text: line, Red: run.Red, Italic: run.Italic, Dim: u.Dim, Break: li > 0})
			}
		}
		if u.Number > 0 {
			emit(cardRun{Text: strconv.Itoa(u.Number), Mark: true, Dim: true})
		}
		marks := append([]passageMark(nil), u.Marks...)
		sort.SliceStable(marks, func(i, j int) bool { return marks[i].At < marks[j].At })
		at, next := 0, 0
		for _, run := range u.Runs {
			rs := []rune(run.Text)
			start := 0
			for next < len(marks) && marks[next].At < at+len(rs) {
				cut := marks[next].At - at
				if cut < start {
					cut = start
				}
				text(run, rs[start:cut])
				emit(cardRun{Text: marks[next].Letter, Mark: true})
				start = cut
				next++
			}
			text(run, rs[start:])
			at += len(rs)
		}
		for ; next < len(marks); next++ {
			emit(cardRun{Text: marks[next].Letter, Mark: true}) // at the unit's end
		}
	}
	return out
}

// passagePlainText is the passage's runs read back without their marks, a
// break as a newline: the units' texts, one to a line. What a test holds
// against the verses to show that setting the letters moved nothing.
func passagePlainText(runs []cardRun) string {
	var b strings.Builder
	for _, r := range runs {
		if r.Break {
			b.WriteByte('\n')
		}
		if !r.Mark {
			b.WriteString(r.Text)
		}
	}
	return b.String()
}

// newCrossRefPassage is the passage for a selection and its notes, as the
// list's first row.
func newCrossRefPassage(state *AppState, book string, chapter int, selected []Verse, notes []publisherCrossRef, pal palette) fyne.CanvasObject {
	p := newReadingParagraph(passageRuns(publisherPassageUnits(state, book, chapter, selected, notes)),
		readingGlyphSize(crossRefPassageTextSize), pal.Text, pal.RedLetter, styledPaneFont(), cardItalicFont())
	p.mark, p.dim = pal.Accent, pal.TextMuted
	p.number, p.bold = pal.VerseNumber, passageNumeralFace()
	p.holdFlush = true
	// The list's width is known before the list is laid out, so the passage
	// reports the height of its rows at that width from the first frame: at
	// the card's guess a passage of several rows could report a row too few
	// on the narrowest phone, and stand over the label under it until the
	// toolkit's next pass.
	if state != nil && state.window != nil && state.window.Canvas() != nil {
		p.guess = sheetPanelSize(state, state.window.Canvas()).Width - crossRefListChrome
	}
	return container.New(layout.NewCustomPaddedLayout(2, 6, 0, 0), p)
}
