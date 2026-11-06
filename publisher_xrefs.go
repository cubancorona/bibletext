package bibletext

// The publisher's own cross references — docs/SCRIPTURE_WORKLIST.md, S20.
//
// The NKJV feed carries the edition's cross-reference apparatus as notes, and
// the app has captured every one since its first epoch: 32,517 across the
// canon (32,473 on verses, 43 on Psalm titles, one on a heading), each a
// citation string exactly as the print edition's centre column sets it
// ("Rom. 5:8; Eph. 2:4; (1 John 4:9, 10)"), with the targets the source tagged
// kept as NoteRef spans (apiBibleNote). This file resolves those notes for the
// cross-references panel (crossref_list.go), which draws them in the one card
// idiom it has:
//
//   - a tagged citation whose passage is in the loaded text becomes a card —
//     the card the Treasury's rows are, labelled from its id ("John 19:39",
//     never the continuation "19:39"), previewed from the loaded edition and
//     followed by a tap — unless it names the passage on screen (Current) or
//     stands inside a parenthesised group, the edition's "compare" citations,
//     which stay in their group whether or not the feed tagged them;
//   - what did not become a card — the parenthesised groups, the citation of
//     the passage on screen, a citation the loaded text lacks — is kept
//     verbatim and in print order as the note's remainder, one caption under
//     the note's cards (split);
//   - nothing is sorted, capped or mapped through the versification tables.
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
)

// publisherCrossRef is one note of the publisher's apparatus, verbatim.
type publisherCrossRef struct {
	Verse   int               // the verse the note belongs to; 0 for the Psalm's title
	Text    string            // the citation text exactly as delivered
	Targets []publisherTarget // the resolved citations, in text order; none = words only
}

// publisherTarget is one tagged citation resolved against the loaded text.
type publisherTarget struct {
	Start, End int      // the citation's rune span within Text
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
// A tagged citation resolves by its id: a verse or verse range
// (parseUSFMRefID), else whole chapters (chapterTarget), and a whole-chapter
// id the text follows with ":title" is the chapter's title, placed at its
// first verse. The basis tokens after a citation (basisAfter) ride it. A
// citation whose passage the loaded text lacks is not a target: its words
// stay in the note, and the remainder shows them.
func publisherCrossRefsFor(bd *BibleData, book string, chapter int, selected []Verse) []publisherCrossRef {
	if bd == nil || len(selected) == 0 {
		return nil
	}
	var rows []publisherCrossRef
	add := func(verse int, fn Footnote) {
		if fn.Kind != footnoteKindCrossref {
			return
		}
		text := strings.TrimSpace(fn.Text)
		if text == "" {
			return
		}
		row := publisherCrossRef{Verse: verse, Text: text}
		// The spans index the text the row stores, which the decoder trims
		// before it takes them (normalizeNoteText), and split cuts that text
		// by them; read against the text as delivered, a leading space would
		// put the ":title" and basis reads and every cut one rune off.
		runes := []rune(text)
		for _, r := range fn.Refs {
			if r.Start < 0 || r.End > len(runes) || r.Start >= r.End {
				continue // a span that does not fit the text it claims to index
			}
			tg := publisherTarget{Start: r.Start, End: r.End, Cut: r.End}
			ref, ok := parseUSFMRefID(r.ID)
			if !ok {
				if ref, ok = chapterTarget(bd, r.ID); ok {
					tg.Chapters = true
					if hasRunesAt(runes, r.End, []rune(titleSuffix)) {
						// The title stands at the chapter's first verse: the
						// card goes there, not over the whole psalm.
						tg.Title, tg.Chapters = true, false
						tg.Cut = r.End + len([]rune(titleSuffix))
						ref = crossRef{Book: ref.Book, Chapter: ref.Chapter, Verse: ref.Verse}
					}
				}
			}
			if !ok || bd.GetVerse(ref.Book, ref.Chapter, ref.Verse) == nil {
				continue // words only: the passage is not in the loaded text
			}
			tg.Basis, tg.Cut = basisAfter(runes, tg.Cut)
			tg.Ref = ref
			tg.Current = verse > 0 && ref.Book == book && ref.contains(chapter, verse)
			row.Targets = append(row.Targets, tg)
		}
		rows = append(rows, row)
	}

	if ch := bd.GetChapter(book, chapter); len(ch) > 0 {
		first := ch[0].Verse
		for _, v := range selected {
			if v.Verse == first {
				for _, fn := range bd.SuperscriptionFor(book, chapter).Footnotes {
					add(0, fn)
				}
				break
			}
		}
	}
	for _, v := range selected {
		for _, fn := range v.Footnotes {
			add(v.Verse, fn)
		}
	}
	return rows
}

// split divides a resolved note into the citations that become cards and the
// remainder that stays words. A card is every target that is not Current and
// does not stand inside a parenthesised group (the depth at its Start is
// zero), in text order: a tagged citation inside a group is never a card,
// since carding it would present a compare citation in the shape of a direct
// one and then repeat it in the caption. The remainder is the note with each
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
