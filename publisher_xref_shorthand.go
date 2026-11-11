package bibletext

// The publisher's citation shorthand — docs/SCRIPTURE_WORKLIST.md, S20.
//
// The NKJV's cross-reference notes are citations in the edition's own
// shorthand, and the feed tags only some of them with a machine-readable
// target: every citation outside parentheses but two, and 1,001 of the 8,324
// inside the parenthesised groups (the census of 11 November 2026). The rest
// are plain text. This file reads that text, so that a group's citations can
// be opened as the tagged ones are (publisher_xrefs.go).
//
// The shorthand, as the feed prints it:
//
//   - citations are separated by "; ", and a parenthesised group stands among
//     them as one item: "Rom. 5:8; Eph. 2:4; (1 John 4:9, 10; Rev. 1:5)";
//   - a citation is a book abbreviation, a chapter and a verse ("Rom. 5:8"),
//     and an omitted book is the book of the citation before it in print
//     ("John 7:50; 19:39" is John 19:39), wherever that one stands: into a
//     group from the citation before the group ("Matt. 5:3–12; (11:5)" is
//     Matthew 11:5), and out of a group to the citation after it
//     ("(Matt. 5:21, 22); 19:18" is Matthew 19:18);
//   - ", " adds a further verse of the same chapter ("4:9, 10"), or a further
//     run of its verses ("9:18, 25–27");
//   - an en dash joins the ends of a run of verses within a chapter
//     ("3:1–12"); an em dash joins a run that crosses chapters ("5:28—10:1")
//     or a run of whole chapters ("24—26");
//   - a chapter with no verse is the whole chapter ("Lev. 11", and after it
//     "; 12"), except in the five books of one chapter, where the number is a
//     verse ("Jude 6", "Obad. 10–14");
//   - ":title" after a whole chapter makes it a citation of the Psalm's
//     title, and ", LXX" or ", DSS" after a citation names its basis. Neither
//     is part of the citation's span: the resolver reads both after it, as it
//     does after a tagged citation.
//
// Every one of these rules is attested by the feed itself. A tagged citation
// pairs its printed words with an id, and readShorthand yields the passage it
// reads in the feed's own id form ("JHN.19.39", "MAT.3.1-MAT.3.12", "LEV.11",
// "1CH.24-1CH.26"), so the reading can be held against the tag for every
// tagged citation of a canon. A shape the feed never tags is not read — a
// book after a comma, an en dash across chapters, a verse with a letter — and
// nothing is carried over a piece that was not read, so a citation is never a
// guess.

import (
	"strconv"
	"strings"
)

// shorthandBooks is the edition's abbreviation of each book, as it stands
// before a chapter number, with the book's id in the feed. The sixty-six are
// the ones the feed's tagged citations pair with an id, and the census holds
// the table equal to those pairs and finds no other abbreviation in a note.
var shorthandBooks = map[string]string{
	"Gen.": "GEN", "Ex.": "EXO", "Lev.": "LEV", "Num.": "NUM", "Deut.": "DEU",
	"Josh.": "JOS", "Judg.": "JDG", "Ruth": "RUT", "1 Sam.": "1SA", "2 Sam.": "2SA",
	"1 Kin.": "1KI", "2 Kin.": "2KI", "1 Chr.": "1CH", "2 Chr.": "2CH",
	"Ezra": "EZR", "Neh.": "NEH", "Esth.": "EST", "Job": "JOB", "Ps.": "PSA",
	"Prov.": "PRO", "Eccl.": "ECC", "Song": "SNG", "Is.": "ISA", "Jer.": "JER",
	"Lam.": "LAM", "Ezek.": "EZK", "Dan.": "DAN", "Hos.": "HOS", "Joel": "JOL",
	"Amos": "AMO", "Obad.": "OBA", "Jon.": "JON", "Mic.": "MIC", "Nah.": "NAM",
	"Hab.": "HAB", "Zeph.": "ZEP", "Hag.": "HAG", "Zech.": "ZEC", "Mal.": "MAL",
	"Matt.": "MAT", "Mark": "MRK", "Luke": "LUK", "John": "JHN", "Acts": "ACT",
	"Rom.": "ROM", "1 Cor.": "1CO", "2 Cor.": "2CO", "Gal.": "GAL", "Eph.": "EPH",
	"Phil.": "PHP", "Col.": "COL", "1 Thess.": "1TH", "2 Thess.": "2TH",
	"1 Tim.": "1TI", "2 Tim.": "2TI", "Titus": "TIT", "Philem.": "PHM",
	"Heb.": "HEB", "James": "JAS", "1 Pet.": "1PE", "2 Pet.": "2PE",
	"1 John": "1JN", "2 John": "2JN", "3 John": "3JN", "Jude": "JUD", "Rev.": "REV",
}

// oneChapterBooks are the books of a single chapter, by id. The edition cites
// their verses with no chapter ("Jude 6" is Jude 1:6), where the same shape
// in any other book is a whole chapter ("Lev. 11").
var oneChapterBooks = map[string]bool{"OBA": true, "PHM": true, "2JN": true, "3JN": true, "JUD": true}

// The two dashes of the shorthand: the en dash within a chapter, the em dash
// across chapters.
const (
	shorthandVerseDash   = '–'
	shorthandChapterDash = '—'
)

// shorthandCitation is one citation of a note as the shorthand reads it.
type shorthandCitation struct {
	// Start and End are the rune span of the citation's printed words within
	// the note, a title suffix and a basis left out, as a tag's span leaves
	// them out.
	Start, End int
	// ID is the passage in the feed's id form; "" for a piece the shorthand
	// does not read.
	ID string
	// Group is the parenthesised group the citation stands in, counted from 0
	// in print order; -1 outside every group.
	Group int
}

// shorthandGroup is one parenthesised group of a note, by its rune span, the
// parentheses included.
type shorthandGroup struct{ Start, End int }

// readShorthand reads a note's citations in print order, and finds its
// parenthesised groups. Every piece between separators is returned, the ones
// that could not be read with an empty ID; a basis token is not a piece. A
// parenthesis inside a group opens no group of its own, and one that never
// closes opens none at all: its citations stand outside every group.
func readShorthand(text string) (citations []shorthandCitation, groups []shorthandGroup) {
	runes := []rune(text)
	var carried shorthandCarry
	depth, group, comma := 0, -1, false
	for i := 0; i < len(runes); {
		switch runes[i] {
		case ' ':
			i++
		case '(':
			if depth == 0 {
				group = len(groups)
				groups = append(groups, shorthandGroup{Start: i})
			}
			depth++
			comma = false
			i++
		case ')':
			if depth > 0 {
				if depth--; depth == 0 {
					groups[group].End = i + 1
					group = -1
				}
			}
			comma = false
			i++
		case ';':
			comma = false
			i++
		case ',':
			comma = true
			i++
		default:
			next := i
			for next < len(runes) && !strings.ContainsRune(";,()", runes[next]) {
				next++
			}
			end := next
			for end > i && runes[end-1] == ' ' {
				end--
			}
			piece := string(runes[i:end])
			if comma && carried.book != "" && isBasisToken(piece) {
				i = next // the basis of the citation before it, which the resolver reads
				continue
			}
			title := strings.HasSuffix(piece, titleSuffix)
			if title {
				piece = strings.TrimSuffix(piece, titleSuffix)
				end -= len([]rune(titleSuffix))
			}
			citations = append(citations, shorthandCitation{
				Start: i, End: end, Group: group,
				ID: carried.read(piece, comma, title),
			})
			i = next
		}
	}
	if depth > 0 {
		// The last parenthesis never closed: no group, and its citations
		// stand outside every group.
		open := len(groups) - 1
		groups = groups[:open]
		for i := range citations {
			if citations[i].Group == open {
				citations[i].Group = -1
			}
		}
	}
	return citations, groups
}

// isBasisToken reports whether piece is one of the ancient texts the edition
// names after a citation as its basis (basisTokens).
func isBasisToken(piece string) bool {
	for _, tok := range basisTokens {
		if piece == tok {
			return true
		}
	}
	return false
}

// shorthandCarry is what a citation hands on to the one after it: its book,
// and its chapter while a further verse of that chapter can follow a comma.
type shorthandCarry struct {
	book    string // the book's id; "" when no book is carried
	chapter int    // the chapter a comma's verse belongs to; 0 when none is carried
}

// read reads one piece — a citation between separators, its title suffix
// already taken off — and returns its passage in the feed's id form, or ""
// for a piece that is not one of the shorthand's shapes. comma says the piece
// follows a comma, and title that ":title" followed it. A piece that is read
// sets what is carried; one that is not clears it, so the citation after an
// unread piece does not take a book or a chapter the print may not mean.
func (c *shorthandCarry) read(piece string, comma, title bool) string {
	id, book, chapter := c.interpret(piece, comma, title)
	if id == "" {
		*c = shorthandCarry{}
		return ""
	}
	c.book, c.chapter = book, chapter
	return id
}

// interpret is read without the carry's update: the id, and the book and
// chapter the citation hands on.
func (c shorthandCarry) interpret(piece string, comma, title bool) (id, book string, chapter int) {
	token, figures := "", piece
	if at := strings.LastIndexByte(piece, ' '); at >= 0 {
		token, figures = piece[:at], piece[at+1:]
	}
	f, ok := parseShorthandFigures(figures)
	if !ok {
		return "", "", 0
	}
	if comma {
		// A further verse, or run of verses, of the chapter carried: figures
		// alone, with no chapter of their own.
		if token != "" || title || c.book == "" || c.chapter == 0 || f.colon {
			return "", "", 0
		}
		switch {
		case f.dash == 0:
			return shorthandVerseID(c.book, c.chapter, f.first), c.book, c.chapter
		case f.dash == shorthandVerseDash && !f.endColon && f.end > f.first:
			return shorthandVerseID(c.book, c.chapter, f.first) + "-" + shorthandVerseID(c.book, c.chapter, f.end), c.book, c.chapter
		}
		return "", "", 0
	}
	book = c.book
	if token != "" {
		book = shorthandBooks[token]
	}
	if book == "" {
		return "", "", 0
	}
	switch {
	case title:
		// The title of a Psalm: a whole chapter and nothing more.
		if f.colon || f.dash != 0 || oneChapterBooks[book] {
			return "", "", 0
		}
		return shorthandChapterID(book, f.first), book, 0
	case f.colon && f.dash == 0:
		return shorthandVerseID(book, f.first, f.second), book, f.first
	case f.colon && f.dash == shorthandVerseDash && !f.endColon && f.end > f.second:
		return shorthandVerseID(book, f.first, f.second) + "-" + shorthandVerseID(book, f.first, f.end), book, f.first
	case f.colon && f.dash == shorthandChapterDash && f.endColon && f.end > f.first:
		// A run across chapters hands no chapter on: the feed has no comma
		// after one to say which it would be.
		return shorthandVerseID(book, f.first, f.second) + "-" + shorthandVerseID(book, f.end, f.endSecond), book, 0
	case f.colon:
		return "", "", 0
	case oneChapterBooks[book] && f.dash == 0:
		return shorthandVerseID(book, 1, f.first), book, 1
	case oneChapterBooks[book] && f.dash == shorthandVerseDash && !f.endColon && f.end > f.first:
		return shorthandVerseID(book, 1, f.first) + "-" + shorthandVerseID(book, 1, f.end), book, 1
	case oneChapterBooks[book]:
		return "", "", 0
	case f.dash == 0:
		return shorthandChapterID(book, f.first), book, 0
	case f.dash == shorthandChapterDash && !f.endColon && f.end > f.first:
		return shorthandChapterID(book, f.first) + "-" + shorthandChapterID(book, f.end), book, 0
	}
	return "", "", 0
}

func shorthandVerseID(book string, chapter, verse int) string {
	return book + "." + strconv.Itoa(chapter) + "." + strconv.Itoa(verse)
}

func shorthandChapterID(book string, chapter int) string {
	return book + "." + strconv.Itoa(chapter)
}

// shorthandFigures is the numbers of a citation: "5:8", "3:1–12",
// "5:28—10:1", "11", "24—26".
type shorthandFigures struct {
	first, second  int  // the number, and the one after its colon
	colon          bool // a colon follows the first number
	dash           rune // the dash after them: the en dash, the em dash, or 0 for none
	end, endSecond int  // the number after the dash, and the one after its colon
	endColon       bool // a colon follows the number after the dash
}

// parseShorthandFigures reads a citation's numbers. Each is one to three
// digits with no leading nought — no book has more than 150 chapters or a
// chapter more than 176 verses — and nothing else may stand among them: a
// verse with a letter, or a second dash, is not read.
func parseShorthandFigures(s string) (shorthandFigures, bool) {
	var f shorthandFigures
	runes := []rune(s)
	at := 0
	number := func() (int, bool) {
		start := at
		for at < len(runes) && isDigitRune(runes[at]) && at-start < 3 {
			at++
		}
		if at == start || runes[start] == '0' {
			return 0, false
		}
		n, err := strconv.Atoi(string(runes[start:at]))
		return n, err == nil
	}
	colon := func() bool {
		if at < len(runes) && runes[at] == ':' {
			at++
			return true
		}
		return false
	}
	var ok bool
	if f.first, ok = number(); !ok {
		return f, false
	}
	if f.colon = colon(); f.colon {
		if f.second, ok = number(); !ok {
			return f, false
		}
	}
	if at < len(runes) && (runes[at] == shorthandVerseDash || runes[at] == shorthandChapterDash) {
		f.dash = runes[at]
		at++
		if f.end, ok = number(); !ok {
			return f, false
		}
		if f.endColon = colon(); f.endColon {
			if f.endSecond, ok = number(); !ok {
				return f, false
			}
		}
	}
	return f, at == len(runes)
}
