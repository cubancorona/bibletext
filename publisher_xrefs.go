package bibletext

// The publisher's own cross references — docs/SCRIPTURE_WORKLIST.md, S20.
//
// The NKJV feed carries the edition's cross-reference apparatus as notes, and
// the app has captured every one since its first epoch: 32,517 across the
// canon (32,473 on verses, 43 on Psalm titles, one on a heading), each a
// citation string exactly as the print edition's centre column sets it
// ("Rom. 5:8; Eph. 2:4; (1 John 4:9, 10)"), with the targets the source tagged
// kept as NoteRef spans (apiBibleNote). This file resolves those notes for the
// cross-references panel (crossref_list.go), which draws each note whole, in
// the order it is printed (Pieces; decided 11 November 2026):
//
//   - a citation whose passage is in the loaded text becomes a card — the
//     card the Treasury's rows are, labelled from its id ("John 19:39", never
//     the continuation "19:39"), previewed from the loaded edition and
//     followed by a tap — unless it names the passage on screen (Current),
//     which is shown in its place and not followed;
//   - a parenthesised group, the edition's "compare" citations, is one piece
//     of its note, in its place, and holds its own pieces, read from the
//     print whether or not the feed tagged them;
//   - what the loaded text lacks, or the shorthand does not read, is kept as
//     its words, in its place;
//   - nothing is sorted, capped, merged or mapped through the versification
//     tables, and no citation is dropped because another note has it.
//     The targets are in the edition's own numbering, which is the numbering
//     on screen, so the set the reader sees is the set the publisher printed.
//     The Treasury of Scripture Knowledge rows from crossRefsForSelection are
//     ranked, capped and mapped, and are credited to their own source; the two
//     stand in separate runs of the list, never interleaved.
//
// Three shapes of the apparatus need more than the id, by the census of the
// whole feed taken on 6 November 2026: 47 citations of a Psalm's TITLE, a
// whole-chapter id followed by the words ":title" ("Ps. 59:title"), which cite
// the superscription and not the psalm; 117 whole-chapter ids ("LEV.11",
// "2KI.18-2KI.20"), labelled by chapter; and the three citations the edition
// qualifies by the ancient text they rest on, ", LXX" and ", LXX, DSS", the
// feed's xta spans, which ride their citation as a tag. Those three tokens —
// title, LXX, DSS — are the ONLY words in the apparatus besides citations, and
// the live canon census pins that (TestLiveAPIBibleFullCanon), so a feed that
// starts saying more is caught rather than captioned.
//
// Whether any of it DISPLAYS is BibleVersion.PublisherCrossRefs, true for the
// NKJV only in the file behind the nkjvxrefs build tag: displaying a licensed
// publisher's apparatus is the subject of a licensing enquiry that has not
// been answered, and a store build must not be able to do it before the answer
// comes. The resolver itself is plain code over data every build already holds.

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// publisherCrossRef is one note of the publisher's apparatus, verbatim.
type publisherCrossRef struct {
	Verse int // the verse the note belongs to; 0 for the Psalm's title
	// Heading marks a note that hangs from the section heading set before
	// Verse and not from the verse itself: one in the NKJV, on the heading
	// before Numbers 4:21.
	Heading bool
	// Index is the note's place among the notes of its verse number, from 0
	// in the order they stand: a title's among the title's, and a heading's
	// before the verse's it precedes, counted with them. Anchor is the rune
	// offset in the note's own text — the verse, the title, the heading —
	// where the note stands: directly before the word it belongs to, in
	// print the word that carries the note's letter. A heading's note has 0,
	// the decoder keeping no place within a heading.
	Index, Anchor int
	// Mark is where the note's letter is drawn in that text: Anchor, moved
	// across the white space that stands at it so that the letter stands
	// directly before the note's word (noteMarkAt), and to the head of the
	// word in the three places where the feed sets a caller inside one
	// (wordSplitCallers).
	Mark int
	// Unkeyed marks a note whose caller the feed lacks: it is shown with its
	// verse, after the verse's other notes, and carries no letter, since no
	// word of the verse is known to be its own (publisher_xref_displaced.go).
	Unkeyed bool
	Text    string            // the citation text exactly as delivered
	Targets []publisherTarget // the resolved citations the feed tagged, in text order; none = words only
	Groups  []publisherGroup  // the note's parenthesised groups, in text order
	// Pieces is the whole note in the order the edition prints it: every
	// citation, every parenthesised group and anything else it prints, each
	// with its place in Text, so that nothing between the first rune and the
	// last is outside a piece but the separators the edition sets between
	// them (pieceGaps). The panel draws a note from its pieces.
	Pieces []publisherPiece
}

// publisherPiece is one piece of a note, or of a parenthesised group, in the
// order the edition prints it. Exactly one of Target, Group and Words is set.
type publisherPiece struct {
	// Start and End are the piece's rune span within the note's Text: a
	// citation's to its Cut, so with a title suffix and a basis; a group's
	// with its parentheses.
	Start, End int
	// Target is a citation whose passage the loaded text has, read from the
	// print (readShorthand): Current when it is the passage the reader is on,
	// which is shown in its place and not followed.
	Target *publisherTarget
	// Group is a parenthesised group. A group holds no group.
	Group *publisherGroup
	// Words is anything else, exactly as printed: a piece the shorthand does
	// not read, or a citation whose passage the loaded text lacks.
	Words string
}

// publisherGroup is one parenthesised group of a note, the edition's
// "compare" citations, as printed, with the citations of it that the loaded
// text has.
type publisherGroup struct {
	Start, End int    // the group's rune span within the note's Text, the parentheses included
	Text       string // the group exactly as printed, the parentheses included
	// Members are the group's citations resolved against the loaded text, in
	// print order and one to a printed item, so "4:9, 10" is two. The feed
	// tags about one in eight of a group's citations, so each is read from
	// the print (readShorthand), tagged or not, and resolved as a tagged
	// citation is: a whole chapter, a title and a basis are what they are
	// outside a group. A piece the shorthand does not read, or whose passage
	// the loaded text lacks, is not a member; its words are still in Text.
	Members []publisherTarget
	// Pieces is everything between the parentheses in print order, the
	// members among them: what the opened group draws, so that a citation of
	// the reader's own verse and a piece that cannot be followed stand where
	// the edition set them and are not passed over.
	Pieces []publisherPiece
}

// cards are the members a reader can follow: all but a citation of the
// note's own verse, which is the passage on screen.
func (g publisherGroup) cards() []publisherTarget {
	var out []publisherTarget
	for _, tg := range g.Members {
		if !tg.Current {
			out = append(out, tg)
		}
	}
	return out
}

// publisherTarget is one tagged citation resolved against the loaded text.
type publisherTarget struct {
	Start, End int      // the citation's rune span within Text
	ID         string   // the passage in the feed's id form: "ROM.5.8", "MAT.3.1-MAT.3.12", "LEV.11"
	Ref        crossRef // the passage, in the on-screen edition's own numbering
	Current    bool     // the passage contains the note's own verse: shown, not followed
	// Chapters marks a citation of whole chapters ("Lev. 11", "2 Kin. 18–20"):
	// Ref runs from the first chapter's first verse to the last one's last,
	// and the card is labelled by chapter.
	Chapters bool
	// Title marks a citation of a Psalm's title: a whole-chapter id followed
	// by ":title" ("Ps. 59:title"), citing the superscription and not the
	// psalm. Ref is the chapter's first verse, where the title stands; the
	// card is labelled by chapter, tagged "Title", and previews the title.
	Title bool
	// Basis is the ancient text the edition names as the citation's basis,
	// "LXX" or "LXX, DSS" — the feed's xta spans, flattened into Text after
	// the citation as ", LXX" / ", LXX, DSS". "" for every other citation.
	Basis string
	// Cut is where the citation ends together with its ":title" suffix and
	// its basis tokens: End when it has neither. A card consumes Text up to
	// here, so the remainder keeps none of what the card already says (split).
	Cut int
}

// label is the card's reference line: the passage as crossRef.label spells
// it, or, for a citation of whole chapters or of a title, the chapter alone
// ("Leviticus 11", "2 Kings 18–20", "Psalms 59").
func (tg publisherTarget) label() string {
	if !tg.Chapters && !tg.Title {
		return tg.Ref.label()
	}
	if tg.Ref.EndCh != 0 && tg.Ref.EndCh != tg.Ref.Chapter {
		return fmt.Sprintf("%s %d–%d", tg.Ref.Book, tg.Ref.Chapter, tg.Ref.EndCh)
	}
	return fmt.Sprintf("%s %d", tg.Ref.Book, tg.Ref.Chapter)
}

// tag is the word set beside the card's reference, in the slot the parallel
// pill uses: "Title" for a title citation — the footnote section's and the
// search tag's own word for a superscription — and the basis for a citation
// the edition qualifies. "" for the rest. The feed never sets both on one
// citation; were it to, both would show.
func (tg publisherTarget) tag() string {
	var tags []string
	if tg.Title {
		tags = append(tags, "Title")
	}
	if tg.Basis != "" {
		tags = append(tags, tg.Basis)
	}
	return strings.Join(tags, ", ")
}

// titleSuffix is what follows a whole-chapter id in a citation of the
// chapter's title: "Ps. 59:title". The feed sets it in lower case, 47 times.
const titleSuffix = ":title"

// basisTokens are the ancient texts the apparatus names after a citation as
// its basis, each set as ", <token>", in the order the feed sets them.
var basisTokens = []string{"LXX", "DSS"}

// basisAfter reads the basis tokens that follow a citation ending at end —
// ", LXX", ", LXX, DSS" — and reports them joined as the feed prints them,
// with where they end. A token must end the text or be followed by a
// separator, so a word that merely begins with one is not read. Nothing
// there gives "" and end unchanged.
func basisAfter(runes []rune, end int) (string, int) {
	var found []string
	at := end
	for {
		matched := false
		for _, tok := range basisTokens {
			piece := []rune(", " + tok)
			if !hasRunesAt(runes, at, piece) {
				continue
			}
			next := at + len(piece)
			if next < len(runes) && (isLetterRune(runes[next]) || isDigitRune(runes[next])) {
				continue
			}
			found = append(found, tok)
			at = next
			matched = true
			break
		}
		if !matched {
			break
		}
	}
	return strings.Join(found, ", "), at
}

// hasRunesAt reports whether want stands in runes at offset at.
func hasRunesAt(runes []rune, at int, want []rune) bool {
	if at < 0 || at+len(want) > len(runes) {
		return false
	}
	for i, r := range want {
		if runes[at+i] != r {
			return false
		}
	}
	return true
}

func isLetterRune(r rune) bool { return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') }
func isDigitRune(r rune) bool  { return r >= '0' && r <= '9' }

// parseUSFMRefID reads a source target id — "JHN.7.50", or the range
// "MAT.3.1-MAT.3.12" — into a crossRef in the app's book names. Anything
// malformed is rejected. A range whose end is in another book, or before its
// start, keeps only the start, so the row points at scripture the reader can
// reach rather than at nothing.
func parseUSFMRefID(id string) (crossRef, bool) {
	parts := strings.Split(strings.TrimSpace(id), "-")
	if len(parts) > 2 {
		return crossRef{}, false
	}
	book, ch, v, ok := parseUSFMRef(parts[0])
	if !ok {
		return crossRef{}, false
	}
	out := crossRef{Book: book, Chapter: ch, Verse: v}
	if len(parts) == 2 {
		if eb, ec, ev, ok := parseUSFMRef(parts[1]); ok && eb == book && (ec > ch || (ec == ch && ev > v)) {
			out.EndCh, out.EndV = ec, ev
		}
	}
	return out, true
}

// parseUSFMChapterID reads a citation of whole chapters — "LEV.11", or the
// range "2KI.18-2KI.20" — as its book and first and last chapter. The NKJV
// tags 117 of its citations so ("Lev. 11", "Ps. 22", "Is. 36–39"), and
// parseUSFMRefID, which needs a verse, rejected every one: the citation
// stayed words while its neighbours were links.
func parseUSFMChapterID(id string) (book string, from, to int, ok bool) {
	parts := strings.Split(strings.TrimSpace(id), "-")
	if len(parts) > 2 {
		return "", 0, 0, false
	}
	chapterOf := func(s string) (string, int, bool) {
		f := strings.Split(strings.TrimSpace(s), ".")
		if len(f) != 2 {
			return "", 0, false
		}
		b := apiBibleBookName(strings.ToUpper(f[0]))
		ch, err := strconv.Atoi(f[1])
		if b == "" || err != nil || ch < 1 {
			return "", 0, false
		}
		return b, ch, true
	}
	book, from, ok = chapterOf(parts[0])
	if !ok {
		return "", 0, 0, false
	}
	to = from
	if len(parts) == 2 {
		if eb, ec, ok := chapterOf(parts[1]); ok && eb == book && ec > from {
			to = ec
		}
	}
	return book, from, to, true
}

// chapterTarget resolves a whole-chapter citation against the loaded text:
// from the first verse of its first chapter to the last verse of its last.
func chapterTarget(bd *BibleData, id string) (crossRef, bool) {
	book, from, to, ok := parseUSFMChapterID(id)
	if !ok {
		return crossRef{}, false
	}
	first, last := bd.GetChapter(book, from), bd.GetChapter(book, to)
	if len(first) == 0 || len(last) == 0 {
		return crossRef{}, false
	}
	return normaliseSpanEnd(crossRef{Book: book, Chapter: from, Verse: first[0].Verse, EndCh: to, EndV: last[len(last)-1].Verse}), true
}

// parseUSFMRef reads one "BOOK.chapter.verse" id.
func parseUSFMRef(s string) (book string, ch, v int, ok bool) {
	f := strings.Split(strings.TrimSpace(s), ".")
	if len(f) != 3 {
		return "", 0, 0, false
	}
	book = apiBibleBookName(strings.ToUpper(f[0]))
	if book == "" {
		return "", 0, 0, false
	}
	ch, err1 := strconv.Atoi(f[1])
	v, err2 := strconv.Atoi(f[2])
	if err1 != nil || err2 != nil || ch < 1 || v < 1 {
		return "", 0, 0, false
	}
	return book, ch, v, true
}

// contains reports whether the passage c covers verse (ch, v) of its own book.
func (c crossRef) contains(ch, v int) bool {
	endCh, endV := c.EndCh, c.EndV
	if endV == 0 {
		return ch == c.Chapter && v == c.Verse
	}
	if endCh == 0 {
		endCh = c.Chapter
	}
	afterStart := ch > c.Chapter || (ch == c.Chapter && v >= c.Verse)
	beforeEnd := ch < endCh || (ch == endCh && v <= endV)
	return afterStart && beforeEnd
}

// publisherCrossRefsFor resolves the publisher's notes for the selected
// verses of one chapter: one entry per note, in verse order and then in the
// order the notes stand in the verse, with the Psalm's title notes first
// (keyed 0) when the selection reaches the chapter's first verse — the title
// precedes verse 1 on the page, so its references precede verse 1's.
//
// A tagged citation resolves by its id (resolveCitation). A citation whose
// passage the loaded text lacks is not a target: it is a piece of the note
// that is its words. The note's parenthesised groups are resolved beside its
// targets, each citation of a group by what the group prints. A heading's
// notes stand before the notes of the verse the heading is set before; and
// in the two stretches where the feed hangs every note one caller late, a
// caller is given the content that is its own (publisher_xref_displaced.go).
func publisherCrossRefsFor(bd *BibleData, book string, chapter int, selected []Verse) []publisherCrossRef {
	if bd == nil || len(selected) == 0 {
		return nil
	}
	var rows []publisherCrossRef
	nextIndex := 0
	// unit is the text the note stands in: the verse's, the title's or the
	// heading's.
	add := func(verse int, heading, unkeyed bool, unit string, fn Footnote) {
		if fn.Kind != footnoteKindCrossref {
			return
		}
		text := strings.TrimSpace(fn.Text)
		if text == "" {
			return
		}
		row := publisherCrossRef{Verse: verse, Heading: heading, Unkeyed: unkeyed, Anchor: fn.Anchor, Text: text}
		row.Mark = noteMarkAt([]rune(unit), wordSplitCallers.wordStart(book, chapter, verse, heading, []rune(unit), fn.Anchor))
		// One continuous run of letters across the whole selection — title,
		// noted headings and verses alike — in spreadsheet order (a..z, aa..).
		// A note keyed to no word takes no letter.
		if unkeyed {
			row.Index, row.Anchor, row.Mark = -1, 0, 0
		} else {
			row.Index = nextIndex
			nextIndex++
		}
		// The spans index the text the row stores, which the decoder trims
		// before it takes them (normalizeNoteText), and split cuts that text
		// by them; read against the text as delivered, a leading space would
		// put the ":title" and basis reads and every cut one rune off.
		runes := []rune(text)
		resolve := func(start, end int, id string) (publisherTarget, bool) {
			return resolveCitation(bd, runes, start, end, id, book, chapter, selected)
		}
		tagged := map[int]NoteRef{} // the feed's tags by where their span starts
		for _, r := range fn.Refs {
			if r.Start < 0 || r.End > len(runes) || r.Start >= r.End {
				continue // a span that does not fit the text it claims to index
			}
			tagged[r.Start] = r
			if tg, ok := resolve(r.Start, r.End, r.ID); ok {
				row.Targets = append(row.Targets, tg)
			}
		}
		// The pieces: every citation the shorthand finds in the print, in or
		// out of a group, and the groups among them. Outside a group a
		// citation the feed tagged is resolved by its tag, as it always was,
		// and by the print's reading only where the feed set none; the two
		// are the same passage wherever both exist, which the census holds
		// over a canon (publisher_xrefs_census_test.go). Inside a group the
		// print is followed, since the feed tags one citation in eight there
		// and gets twelve of those wrong.
		citations, groups := readShorthand(text)
		for _, g := range groups {
			row.Groups = append(row.Groups, publisherGroup{Start: g.Start, End: g.End, Text: string(runes[g.Start:g.End])})
		}
		piece := func(c shorthandCitation) publisherPiece {
			if r, ok := tagged[c.Start]; ok && c.Group < 0 {
				if tg, ok := resolve(r.Start, r.End, r.ID); ok {
					return publisherPiece{Start: tg.Start, End: tg.Cut, Target: &tg}
				}
			}
			if c.ID != "" {
				if tg, ok := resolve(c.Start, c.End, c.ID); ok {
					return publisherPiece{Start: tg.Start, End: tg.Cut, Target: &tg}
				}
			}
			// Words: with the title suffix the shorthand set aside, and the
			// basis of a citation it read, so the piece is all the edition
			// printed of it.
			end := c.End
			if hasRunesAt(runes, end, []rune(titleSuffix)) {
				end += len([]rune(titleSuffix))
			}
			if c.ID != "" {
				_, end = basisAfter(runes, end)
			}
			return publisherPiece{Start: c.Start, End: end, Words: string(runes[c.Start:end])}
		}
		placed := 0 // the groups already set among the note's pieces
		place := func(before int) {
			for ; placed < len(row.Groups) && row.Groups[placed].Start < before; placed++ {
				g := &row.Groups[placed]
				row.Pieces = append(row.Pieces, publisherPiece{Start: g.Start, End: g.End, Group: g})
			}
		}
		for _, c := range citations {
			p := piece(c)
			if c.Group >= 0 {
				g := &row.Groups[c.Group]
				g.Pieces = append(g.Pieces, p)
				if p.Target != nil {
					g.Members = append(g.Members, *p.Target)
				}
				continue
			}
			place(c.Start)
			row.Pieces = append(row.Pieces, p)
		}
		place(len(runes) + 1)
		spreadBasis(runes, row.Pieces)
		for _, g := range row.Groups {
			spreadBasis(runes, g.Pieces)
		}
		rows = append(rows, row)
	}

	// addCallers adds the cross-reference notes of one verse or title, each
	// caller with the content that is its own: the one delivered on it, or,
	// in the two stretches where the feed hangs every content one caller
	// late, the next one (displacedContent).
	addCallers := func(verse int, unit string, fns []Footnote) {
		for i, fn := range crossRefNotesOf(fns) {
			if own, shown := displacedContent(bd, noteCaller{book, chapter, verse, i}, fn); shown {
				add(verse, false, false, unit, own)
			}
		}
	}
	if ch := bd.GetChapter(book, chapter); len(ch) > 0 {
		first := ch[0].Verse
		for _, v := range selected {
			if v.Verse == first {
				title := bd.SuperscriptionFor(book, chapter)
				addCallers(0, title.Text, title.Footnotes)
				break
			}
		}
	}
	for _, v := range selected {
		// A heading stands before its verse on the page, so its notes
		// precede the verse's.
		for _, h := range notedHeadingsBefore(bd, book, chapter, v.Verse) {
			for _, fn := range h.Footnotes {
				add(v.Verse, true, false, h.Text, fn)
			}
		}
		addCallers(v.Verse, v.Text, v.Footnotes)
		// A content whose caller the feed lacks, after the verse's others.
		for _, fn := range displacedHomeNotes(bd, book, chapter, v.Verse) {
			add(v.Verse, false, true, v.Text, fn)
		}
	}
	return rows
}

// notedHeadingsBefore are the section headings set before a verse that carry
// a cross-reference note, in the order they stand. The NKJV has one, before
// Numbers 4:21; its note reached no reader while the panel listed a verse's
// notes alone.
func notedHeadingsBefore(bd *BibleData, book string, chapter, verse int) []Heading {
	var out []Heading
	for _, h := range headingsFor(bd, book, chapter) {
		if h.BeforeVerse != verse {
			continue
		}
		for _, fn := range h.Footnotes {
			if fn.Kind == footnoteKindCrossref && strings.TrimSpace(fn.Text) != "" {
				out = append(out, h)
				break
			}
		}
	}
	return out
}

// alone reports whether the note is one citation and nothing else, printed as
// the edition prints a citation that carries its own book. That is the one
// shape whose card says all the note says: the note's text can be rebuilt
// from the card (shorthandPrint), so the text need not stand beside it. Every
// other note keeps its printed text on screen.
func (n publisherCrossRef) alone() (publisherTarget, bool) {
	if len(n.Pieces) != 1 || n.Pieces[0].Target == nil {
		return publisherTarget{}, false
	}
	p := n.Pieces[0]
	if p.Start != 0 || p.End != len([]rune(n.Text)) {
		return publisherTarget{}, false
	}
	if printed, ok := shorthandPrint(p.Target.ID, p.Target.Title, p.Target.Basis); !ok || printed != n.Text {
		return publisherTarget{}, false
	}
	return *p.Target, true
}

// wordsAlone reports whether the note is one piece that is no citation the
// reader can follow and nothing else: its words are then the note.
func (n publisherCrossRef) wordsAlone() (string, bool) {
	if len(n.Pieces) != 1 || n.Pieces[0].Words == "" || n.Pieces[0].Words != n.Text {
		return "", false
	}
	return n.Pieces[0].Words, true
}

// groupAlone reports whether the note is one parenthesised group and nothing
// else: the note is then drawn as that group's row beside the letter's
// margin, closed until tapped (crossref_note.go).
func (n publisherCrossRef) groupAlone() (*publisherGroup, bool) {
	if len(n.Pieces) != 1 || n.Pieces[0].Group == nil {
		return nil, false
	}
	if p := n.Pieces[0]; p.Start != 0 || p.End != len([]rune(n.Text)) {
		return nil, false
	}
	return n.Pieces[0].Group, true
}

// pieceGaps is what stands in a stretch of a note outside its pieces: the
// text before the first piece, between each two, and after the last, in
// order. In a note the edition printed and the shorthand covers, every gap
// between two pieces is "; " or ", " and the ends are empty; anything else is
// text no piece carries, which the census counts and a canon must not have.
// from and to bound the stretch: a note's whole text, or the inside of a
// group's parentheses.
func pieceGaps(runes []rune, pieces []publisherPiece, from, to int) []string {
	gaps := make([]string, 0, len(pieces)+1)
	at := from
	for _, p := range pieces {
		start := p.Start
		if start < at {
			start = at // overlapping pieces: reported as an empty gap, caught by the order check
		}
		gaps = append(gaps, string(runes[at:start]))
		if p.End > at {
			at = p.End
		}
	}
	if at > to {
		at = to
	}
	return append(gaps, string(runes[at:to]))
}

// noteMarkAt is where a note's letter is drawn in its verse: the note's
// offset, moved across the white space that stands at it, so that the letter
// stands directly before the word the note belongs to. The decoder writes a
// note's offset before the line break or the space it then adds, so about two
// thousand offsets stand at the end of a poem line whose next line opens with
// the note's word.
func noteMarkAt(text []rune, anchor int) int {
	if anchor < 0 {
		anchor = 0
	}
	for anchor < len(text) && unicode.IsSpace(text[anchor]) {
		anchor++
	}
	if anchor > len(text) {
		anchor = len(text)
	}
	return anchor
}

// noteCaller names a place a cross-reference note stands: a verse, or a
// Psalm's title, and which of its notes.
type noteCaller struct {
	Book    string
	Chapter int
	Verse   int // 0 for the Psalm's title
	Index   int // among the cross-reference notes of that verse or title; for wordSplits, the note's offset
}

// wordSplits are the callers the feed sets inside a word, by where they
// stand: the three of the canon fetched on 6 November 2026, each the offset
// in its verse. Two hundred and fifty-seven other callers stand between two
// letters because the feed ran two words together there, and those stand
// where they are, between the words; these three split one word, and a
// letter is drawn before a word, never in it.
type wordSplits map[noteCaller]bool

// wordSplitCallers names them by book, chapter, verse and offset (Index).
var wordSplitCallers = wordSplits{
	{"Genesis", 42, 20, 20}: true,
	{"Exodus", 12, 25, 78}:  true,
	{"Exodus", 19, 17, 20}:  true,
}

// wordStart is anchor, or, for a caller named here that still stands
// between two letters, the offset of the first letter of the word it splits.
func (w wordSplits) wordStart(book string, chapter, verse int, heading bool, text []rune, anchor int) int {
	if heading || !w[noteCaller{book, chapter, verse, anchor}] || anchor <= 0 || anchor >= len(text) || !unicode.IsLetter(text[anchor]) {
		return anchor
	}
	start := anchor
	for start > 0 && unicode.IsLetter(text[start-1]) {
		start--
	}
	return start
}

// spreadBasis gives a basis to every verse it is printed after. The edition
// names the ancient text after a citation — "Ps. 45:4, 5, LXX" — and where
// the citation is a list of verses the basis is the whole citation's: in the
// feed the words stand after the span that holds both verses. So a piece
// that carries a basis hands it back to the pieces before it as far as the
// last "; ".
func spreadBasis(runes []rune, pieces []publisherPiece) {
	for i := len(pieces) - 1; i > 0; i-- {
		p, before := pieces[i], pieces[i-1]
		if p.Target == nil || p.Target.Basis == "" || before.Target == nil || before.Target.Basis != "" {
			continue
		}
		if string(runes[before.End:p.Start]) != ", " {
			continue
		}
		before.Target.Basis = p.Target.Basis
	}
}

// noteLetter is the letter the panel sets on a note and on its word: a for a
// selection's first note, b for its second, running continuously across the
// selection's title, noted headings and verses in spreadsheet order (a..z,
// aa, ab..). The feed carries no letters (its caller is "-" on every note),
// so these are the panel's own.
func noteLetter(index int) string {
	if index < 0 {
		return ""
	}
	letters := ""
	for {
		letters = string(rune('a'+index%26)) + letters
		index = index/26 - 1
		if index < 0 {
			return letters
		}
	}
}

// isWithinSelection reports whether c names only verses already in selected:
// an exact match to the selection or a subset of a multi-verse selection. A
// citation that runs beyond the selection is false, so it becomes a tappable
// card; a title citation cites the superscription rather than a verse.
func isWithinSelection(c crossRef, book string, chapter int, selected []Verse) bool {
	if len(selected) == 0 || c.TitleRef || c.crossBook() || c.Book != book {
		return false
	}
	start, end := c.bounds()
	if start.Chapter != chapter || end.Chapter != chapter || end.Verse < start.Verse {
		return false
	}
	for v := start.Verse; v <= end.Verse; v++ {
		in := false
		for _, sv := range selected {
			if sv.Verse == v {
				in = true
				break
			}
		}
		if !in {
			return false
		}
	}
	return true
}

// resolveCitation resolves one citation of a note against the loaded text:
// the rune span of its printed words within the note (runes) and its id, for a
// selection in book chapter. The id is a verse or a run of verses
// (parseUSFMRefID), else whole chapters (chapterTarget), and whole chapters
// the text follows with ":title" are the chapter's title, placed at its first
// verse without washing verse 1 on arrival. The basis tokens after the
// citation ride it. A citation whose passage the loaded text lacks does not
// resolve.
func resolveCitation(bd *BibleData, runes []rune, start, end int, id, book string, chapter int, selected []Verse) (publisherTarget, bool) {
	tg := publisherTarget{Start: start, End: end, ID: id, Cut: end}
	ref, ok := parseUSFMRefID(id)
	if !ok {
		if ref, ok = chapterTarget(bd, id); ok {
			tg.Chapters = true
			ref.WholeChapters = true
			if hasRunesAt(runes, end, []rune(titleSuffix)) {
				// The title stands at the chapter's first verse: the card
				// goes there, not over the whole psalm, without highlighting
				// verse 1 on arrival.
				tg.Title, tg.Chapters = true, false
				tg.Cut = end + len([]rune(titleSuffix))
				ref = crossRef{Book: ref.Book, Chapter: ref.Chapter, Verse: ref.Verse, TitleRef: true}
			}
		}
	}
	if !ok || bd.GetVerse(ref.Book, ref.Chapter, ref.Verse) == nil {
		return publisherTarget{}, false // words only: the passage is not in the loaded text
	}
	tg.Basis, tg.Cut = basisAfter(runes, tg.Cut)
	tg.Ref = ref
	tg.Current = !tg.Title && isWithinSelection(ref, book, chapter, selected)
	return tg, true
}

// split divides a resolved note into its tagged direct citations and the
// remainder: the measure the apparatus census takes of a canon by the feed's
// own tags (publisher_xrefs_census_test.go). The list is not composed from
// it: it draws a note from its Pieces, which keep every citation, group and
// word in its printed place. A card here is every target that is not Current
// and does not stand inside a parenthesised group (the depth at its Start is
// zero), in text order. The remainder is the note with each
// card's span cut out — to its Cut, so a title suffix and a basis go with
// their citation — its pieces split at ";", trimmed of " ,;", the empty ones
// dropped and the rest re-joined with "; ", the separator the feed sets
// between pieces, so a group that lost nothing reads exactly as printed. A
// note that is one parenthesised group, or wholly untagged, is its own
// remainder; a note whose citations are all cards has none.
func (n publisherCrossRef) split() (cards []publisherTarget, remainder string) {
	runes := []rune(n.Text)
	keep := make([]bool, len(runes))
	for i := range keep {
		keep[i] = true
	}
	for _, tg := range n.Targets {
		if tg.Current || parenDepthAt(runes, tg.Start) > 0 {
			continue
		}
		cards = append(cards, tg)
		for i := tg.Start; i < tg.Cut && i < len(runes); i++ {
			keep[i] = false
		}
	}
	rest := make([]rune, 0, len(runes))
	for i, r := range runes {
		if keep[i] {
			rest = append(rest, r)
		}
	}
	var pieces []string
	for _, p := range strings.Split(string(rest), ";") {
		if p = strings.Trim(p, " ,;"); p != "" {
			pieces = append(pieces, p)
		}
	}
	return cards, strings.Join(pieces, "; ")
}

// parenDepthAt is how many parentheses stand open at rune i of a note: the
// opening brackets before it less the closing ones, never below zero.
func parenDepthAt(runes []rune, i int) int {
	if i > len(runes) {
		i = len(runes)
	}
	depth := 0
	for _, r := range runes[:i] {
		switch r {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		}
	}
	return depth
}
