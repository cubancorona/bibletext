package bibletext

// The page the Apple panes show when a chapter will not import.
//
// The iPhone, iPad and Mac draw a chapter by handing buildChapterHTML's page
// to the system's HTML importer, which fails now and then, most often right
// after the app returns to the foreground. Each pane tries three times and
// then shows this page: the chapter's text, cut before the translators'
// notes, with the markup taken out and its paragraphs and line breaks where
// the importer puts them.
//
// The importer ends a paragraph at each </p> with a line feed and breaks a
// poem line at each <br> with U+2028 (LINE SEPARATOR), which NSString does
// not take for the end of a paragraph. This page writes the same two
// characters, so a paragraph here is the paragraph an imported page has.
// The panes reserve a note's band, place its sticker, card and pills, and
// test which notes share a paragraph, by the paragraph that holds the anchor
// verse's number (paragraphRangeForRange:). A line feed at each poem line
// would make every line a paragraph and open a note between two lines of a
// psalm instead of above its stanza, the defect Android had before 1.2.22
// (AND-2).
//
// The panes find the verses a selection is placed by from the text's faces:
// a verse number is the run set below 0.8 of the largest size, a psalm's
// title the italic paragraph above verse 1, a heading a bold paragraph
// between two verses. Untyped text has none of them, so on the page that used
// to be shown here no verse could be found: every verb sent no span and
// searched the whole chapter for the selected words (Psalm 136:10's refrain
// cited 136:3; on the WEB about 60% of double-clicked words cited an earlier
// verse), and no highlight, note or search arrival could land.
//
// So the walk that takes the markup out also records where each verse
// number, the title and each heading stand, in the page's own UTF-16 code
// units, the units NSString counts in. The panes set those runs in the faces
// the importer gives them (the number at its size and weight, the title
// italic, the heading bold), and then install the page through the same
// function as an imported one (btIOSInstallPage, btMacInstallPage), so every
// range is derived from it as from an imported page, and a selection there
// sends its span and lead as anywhere else.
//
// Go builds the page with every push, from the HTML it is pushing, so the
// page and the HTML cannot be of different chapters, and the walk runs in
// host tests (reading_native_fallback_page_test.go), where the panes'
// Objective-C cannot. On a chapter the size of Psalm 119 it costs about as
// much as building the HTML, under 0.1 ms in a host benchmark.

import (
	"image/color"
	"strings"
	"unicode/utf16"
)

// appleFallbackRunKind says how a run of the page is set. The values cross
// to Objective-C (kBTFallbackNumeral and its siblings in both panes), and a
// source test holds the two lists equal.
type appleFallbackRunKind int

const (
	// A verse's number, at the size and weight the stylesheet gives sup.v.
	appleFallbackNumeral appleFallbackRunKind = 1
	// A psalm's title, the p.pst paragraph: italic.
	appleFallbackTitle appleFallbackRunKind = 2
	// A publisher's heading, a p.sec paragraph: bold, at body size.
	appleFallbackHeading appleFallbackRunKind = 3
)

// appleFallbackRun is one styled stretch of the page, in UTF-16 code units.
// A title's or heading's run is its paragraph's text without the line feed
// that ends it.
type appleFallbackRun struct {
	kind          appleFallbackRunKind
	start, length int
}

// appleFallbackPage is the page and its runs, in page order.
type appleFallbackPage struct {
	text string
	runs []appleFallbackRun
}

// appleFallbackCut is where the page ends: the opening of the translators'
// notes. The panes find the end of Scripture by the notes' size, which the
// page does not set, so the notes must not be on it at all.
const appleFallbackCut = `<p class="fnsep">`

// appleFallbackEntities are the entities buildChapterHTML writes (htmlEscape,
// and the no-break space). They are read in one pass, so an entity the text
// itself spells out ("&amp;lt;" for a verse's "&lt;") is read once, as the
// importer reads it. The no-break space becomes an ordinary space, as the
// earlier page set it.
var appleFallbackEntities = []struct{ name, char string }{
	{"&amp;", "&"},
	{"&lt;", "<"},
	{"&gt;", ">"},
	{"&quot;", `"`},
	{"&nbsp;", " "},
}

// appleFallbackWriter accumulates the page and counts it in UTF-16 units.
type appleFallbackWriter struct {
	b     strings.Builder
	units int
}

func (w *appleFallbackWriter) raw(s string) {
	w.b.WriteString(s)
	for _, r := range s {
		n := utf16.RuneLen(r)
		if n < 1 {
			n = 1
		}
		w.units += n
	}
}

// text writes s with its entities read.
func (w *appleFallbackWriter) text(s string) {
	for {
		i := strings.IndexByte(s, '&')
		if i < 0 {
			w.raw(s)
			return
		}
		w.raw(s[:i])
		s = s[i:]
		read := false
		for _, e := range appleFallbackEntities {
			if strings.HasPrefix(s, e.name) {
				w.raw(e.char)
				s = s[len(e.name):]
				read = true
				break
			}
		}
		if !read {
			w.raw("&")
			s = s[1:]
		}
	}
}

// appleFallbackTag reads a tag's body (what stands between < and >): its
// name, lower-cased with a closing tag's slash kept ("p", "/p", "br"), and
// the words of its class attribute.
func appleFallbackTag(tag string) (name string, classes []string) {
	end := strings.IndexAny(tag, " \t\n/>")
	if strings.HasPrefix(tag, "/") {
		end = strings.IndexAny(tag[1:], " \t\n>")
		if end >= 0 {
			end++
		}
	}
	if end < 0 {
		end = len(tag)
	}
	name = strings.ToLower(tag[:end])
	if i := strings.Index(tag, `class="`); i >= 0 {
		rest := tag[i+len(`class="`):]
		if j := strings.IndexByte(rest, '"'); j >= 0 {
			classes = strings.Fields(rest[:j])
		}
	}
	return name, classes
}

func appleFallbackHasClass(classes []string, c string) bool {
	for _, x := range classes {
		if x == c {
			return true
		}
	}
	return false
}

// appleFallbackLineSeparator is what the importer writes for a <br>: a line
// break inside a paragraph, not the end of one.
const appleFallbackLineSeparator = "\u2028"

// appleFallbackPageOf is the page for a chapter's HTML, as buildChapterHTML
// writes it: cut at the translators' notes, the stylesheet taken out, a line
// separator for each <br> and a line feed at the end of each paragraph, as
// the importer writes them, every other tag taken out and the entities read.
// A "<" with no ">" after it stays as text, as a tag-stripping expression
// would leave it. Each verse number, the title and each heading are recorded
// as a run.
func appleFallbackPageOf(html string) appleFallbackPage {
	if i := strings.Index(html, appleFallbackCut); i >= 0 {
		html = html[:i]
	}
	if i := strings.Index(html, "<head>"); i >= 0 {
		if j := strings.Index(html[i:], "</head>"); j >= 0 {
			html = html[:i] + html[i+j+len("</head>"):]
		}
	}
	var w appleFallbackWriter
	w.b.Grow(len(html)) // the page is the HTML less its markup
	var runs []appleFallbackRun
	add := func(kind appleFallbackRunKind, start int) {
		if w.units > start {
			runs = append(runs, appleFallbackRun{kind: kind, start: start, length: w.units - start})
		}
	}
	para, paraStart := appleFallbackRunKind(0), 0
	number := -1
	for len(html) > 0 {
		lt := strings.IndexByte(html, '<')
		if lt < 0 {
			w.text(html)
			break
		}
		w.text(html[:lt])
		html = html[lt:]
		gt := strings.IndexByte(html, '>')
		if gt < 0 {
			w.text(html)
			break
		}
		if gt == 1 { // "<>" is no tag
			w.text("<>")
			html = html[2:]
			continue
		}
		name, classes := appleFallbackTag(html[1:gt])
		html = html[gt+1:]
		switch name {
		case "br":
			w.raw(appleFallbackLineSeparator)
		case "p":
			para, paraStart = 0, w.units
			switch {
			case appleFallbackHasClass(classes, "pst"):
				para = appleFallbackTitle
			case appleFallbackHasClass(classes, "sec"):
				para = appleFallbackHeading
			}
		case "/p":
			if para != 0 {
				add(para, paraStart)
			}
			para = 0
			w.raw("\n")
		case "sup":
			if appleFallbackHasClass(classes, "v") {
				number = w.units
			}
		case "/sup":
			if number >= 0 {
				add(appleFallbackNumeral, number)
			}
			number = -1
		}
	}
	return appleFallbackPage{text: w.b.String(), runs: runs}
}

// appleFallbackStyle is what the panes set the page in: the body's size, a
// verse number's size, and the two colours the stylesheet gives the text and
// the numbers, from the palette the HTML beside it was built in.
type appleFallbackStyle struct {
	bodyPt, numeralPt float64
	ink, numeral      color.NRGBA
}

// appleFallbackStyleFor is the page's style for the chapter being pushed.
// The sizes are the stylesheet's (buildChapterHTML): the body at the glyph
// size and a verse number at readingNumeralEm of it, which keeps a number
// below the panes' 0.8 threshold exactly as on an imported page.
func appleFallbackStyleFor(state *AppState) appleFallbackStyle {
	pal := state.pal()
	body := readingGlyphPx()
	return appleFallbackStyle{
		bodyPt:    body,
		numeralPt: body * readingNumeralEm,
		ink:       pal.Text,
		numeral:   pal.VerseNumber,
	}
}

// flatRuns is the runs as the panes read them: kind, start and length, one
// triple after another.
func (p appleFallbackPage) flatRuns() []int32 {
	out := make([]int32, 0, 3*len(p.runs))
	for _, r := range p.runs {
		out = append(out, int32(r.kind), int32(r.start), int32(r.length))
	}
	return out
}
