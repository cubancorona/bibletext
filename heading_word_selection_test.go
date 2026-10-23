package bibletext

// A word selected inside a publisher's heading means the section the heading
// introduces: it is cited by the verse below the heading and quotes nothing,
// as a heading selected whole on its own is answered (decision 23, A). The
// iPhone, iPad, Mac and Android panes draw a heading between two verses, in
// the text a reader selects, and resolved a selection to the verse whose
// number is the last before it, the verse above. The share pipeline then
// searched for the words in that verse, and a heading's word that the verse
// above also holds was quoted from it and cited by it.
//
// Each of those panes now tests the selection against the heading range it
// keeps and hands it over as the verse below with the heading flag
// (selSpan.heading); heading_word_selection_contract_test.go holds the panes'
// code to that. These tests hold Go's half, through every verb, on a page
// drawn as those panes draw it (drawnHeadingPage), with the pane's rule
// modelled on it (paneHeadingSpan).

import (
	"os"
	"strings"
	"testing"
	"unicode"
)

// paneHeadingSpan is the span a native pane sends for the selection
// page[a:b] of a drawnHeadingPage: where the selection, less the white space
// at either end, lies wholly in the headings drawn between a verse's end and
// the next verse's number, the verse below with the heading flag
// (btIOSHeadingSelectionVerse, btMacHeadingSelectionVerse,
// BtBridge.headingSelectionVerse); anywhere else the verses and lead the
// panes have sent since the lead was added (headingPageSpan).
func paneHeadingSpan(page string, pvs []headingPageVerse, a, b int) selSpan {
	for a < b && unicode.IsSpace(rune(page[a])) {
		a++
	}
	for b > a && unicode.IsSpace(rune(page[b-1])) {
		b--
	}
	if k := headingPageVerseAt(pvs, a); a < b && k >= 0 && k+1 < len(pvs) {
		matter := page[pvs[k].end:pvs[k+1].num]
		if strings.TrimSpace(matter) != "" && a >= pvs[k].end && b <= pvs[k+1].num {
			return selSpanInHeading(pvs[k+1].verse)
		}
	}
	return headingPageSpan(page, pvs, a, b)
}

// headingWordAnswer is what every verb gives one selection.
type headingWordAnswer struct {
	quote, cite string // the share's quotation, before its framing, and its citation
	linkLo      int    // the share link's verses, which a note is kept against
	linkHi      int
	resolved    []Verse // the cross-references' verses
	aiRef       string  // the study prompt's citation
	aiPreview   string  // and the words the study panel shows
	studied     bool    // whether the desktop menu offers the study verbs
	noteWords   string  // the note composer's excerpt
	noteRef     string
}

func headingWordAnswers(st *AppState, book string, ch int, sel string, span selSpan) headingWordAnswer {
	var a headingWordAnswer
	a.quote, a.cite, _, _ = prepareShareQuoteIn(st, book, ch, sel, span)
	a.linkLo, a.linkHi = linkVersesForSelectionIn(st, book, ch, sel, span)
	a.resolved = selectionVersesIn(st, book, ch, sel, span)
	p := aiPassageFor(st, sel, span)
	a.aiRef, a.aiPreview = p.Ref, p.PreviewText
	a.studied, _, _, _ = studyMenuItems(st, sel, span)
	a.noteWords, a.noteRef = shareNoteQuote(st, sel, span, selectionPassage(st, span))
	return a
}

// sectionBelow reports whether every verb answered the selection as the
// section that verse below opens: cited by it alone, with nothing quoted,
// and offered no study verb, since it holds no word of Scripture
// (no_scripture_study_test.go).
func (a headingWordAnswer) sectionBelow(book string, ch, below int) bool {
	cite := verseRangeCitation(book, ch, below, below)
	return a.quote == "" && a.cite == cite && a.linkLo == below && a.linkHi == below &&
		len(a.resolved) == 1 && a.resolved[0].Verse == below &&
		a.aiRef == cite && a.aiPreview == "" && !a.studied &&
		a.noteWords == "" && a.noteRef == cite
}

// The heading's words, one at a time as a double-tap takes them, the heading
// whole, a drag begun inside one of its words, and a selection that takes the
// break before the heading or after it: every verb cites the verse below and
// quotes nothing. Verse 16 holds "Wanderer" and "the", so searched for, they
// were quoted from verse 16 and cited by it. The controls are the same words
// selected in the verses, which are quoted and cited where they stand.
func TestAWordSelectedInAHeadingMeansTheSectionBelowIt(t *testing.T) {
	const heading = "The Line of the Wanderer"
	f := headingFixture("Genesis", 4, map[int]string{
		16: "So the Wanderer went out and settled east of the garden.",
		17: "The Wanderer knew his wife, and she bore a son, and he built a city.",
	}, Heading{Text: heading, BeforeVerse: 17})
	at := strings.Index(f.page, heading)
	if at < 0 {
		t.Fatalf("fixture: the heading is not on the page:\n%s", f.page)
	}
	// A captured share sends nothing but the citation.
	shared := captureShares(t)

	var sels [][2]int
	for i := 0; i < len(heading); {
		j := strings.IndexByte(heading[i:], ' ')
		if j < 0 {
			j = len(heading) - i
		}
		sels = append(sels, [2]int{at + i, at + i + j})
		i += j + 1
	}
	sels = append(sels,
		[2]int{at, at + len(heading)},         // the heading whole
		[2]int{at + 6, at + len(heading)},     // begun inside "Line"
		[2]int{at - 1, at + 8},                // with the break before it
		[2]int{at + 9, at + len(heading) + 1}, // with the break after it
	)
	// The citation line alone: nothing is quoted, so nothing stands above it.
	want := "— Genesis 4:17 (" + f.st.currentVersion().Name + ")"
	for _, s := range sels {
		sel := f.page[s[0]:s[1]]
		span := paneHeadingSpan(f.page, f.pvs, s[0], s[1])
		if !span.heading || span.lo != 17 || span.hasLead {
			t.Fatalf("%q: the pane's model sent %+v, want verse 17 in its heading with no lead", sel, span)
		}
		a := headingWordAnswers(f.st, f.book, f.ch, sel, span)
		if !a.sectionBelow(f.book, f.ch, 17) {
			t.Errorf("%q selected in the heading: %+v, want Genesis 4:17 alone and nothing quoted", sel, a)
		}
		*shared = nil
		dispatchSelectionAction(f.st, selActionShareCite, sel, span)
		if len(*shared) != 1 || (*shared)[0] != want {
			t.Errorf("%q: Share with citation sent %q, want %q", sel, *shared, want)
		}
	}

	// The controls: the same words selected where the verses hold them, with
	// the span and the lead the panes send there, are quoted and cited there.
	for _, c := range []struct {
		word  string
		verse int
	}{
		{"Wanderer", 16},
		{"the", 16},
		{"The", 17},
		{"Wanderer", 17},
	} {
		v := f.pvs[0]
		if c.verse == 17 {
			v = f.pvs[1]
		}
		i := strings.Index(f.page[v.body:v.end], c.word)
		if v.verse != c.verse || i < 0 {
			t.Fatalf("fixture: %q not in verse %d", c.word, c.verse)
		}
		a := v.body + i
		span := paneHeadingSpan(f.page, f.pvs, a, a+len(c.word))
		if span.heading {
			t.Fatalf("control: %q in verse %d was taken for the heading's", c.word, c.verse)
		}
		got := headingWordAnswers(f.st, f.book, f.ch, c.word, span)
		cite := verseRangeCitation(f.book, f.ch, c.verse, c.verse)
		if got.quote == "" || !strings.Contains(got.quote, c.word) || got.cite != cite || got.linkLo != c.verse || !got.studied {
			t.Errorf("control: %q selected in verse %d: %+v, want it quoted, cited as %s and offered the study verbs", c.word, c.verse, got, cite)
		}
	}
}

// Go's half alone. A span flagged as a heading's is answered by its verse
// with nothing quoted, whatever the words, even words the verse itself holds:
// the pane, not the text, knows where they stand. Not for a verse the chapter
// lacks, which is left to the search. The span: paneSelSpan's span marked by
// inHeading is that verse alone, in its chapter, with no lead; unmarked, or
// naming no verse, it is as it was.
func TestASelectionThePaneFoundInAHeadingIsItsVerseAlone(t *testing.T) {
	st := headingChapterState()
	for _, sel := range []string{headingText, "Spirit", "the", "the Spirit fell", "zzz"} {
		quote, lo, hi, at, ok := normalizeShareSelectionIn(st, "Acts", 10, sel, selSpanInHeading(44))
		if quote != "" || lo != 44 || hi != 44 || at != -1 || !ok {
			t.Errorf("%q, flagged in the heading above 44: (%q, %d, %d, %d, %v), want (\"\", 44, 44, -1, true)", sel, quote, lo, hi, at, ok)
		}
	}
	// The control: unflagged, verse 44's own words are quoted there.
	if quote, lo, _, _, ok := normalizeShareSelectionIn(st, "Acts", 10, "the Spirit fell", selSpanFromNative(44, 44)); quote != "the Spirit fell" || lo != 44 || !ok {
		t.Fatalf("control: verse 44's words, unflagged: (%q, %d, %v)", quote, lo, ok)
	}
	// A verse the chapter does not hold is not taken.
	if _, lo, _, _, ok := normalizeShareSelectionIn(st, "Acts", 10, "the Spirit fell", selSpanInHeading(99)); ok && lo == 99 {
		t.Errorf("a heading's flag on verse 99, which the chapter lacks, cited it")
	}

	span := paneSelSpan(44, 44, "", false, "Acts", 10).inHeading(true)
	if want := (selSpan{lo: 44, hi: 44, heading: true, book: "Acts", chapter: 10}); span != want {
		t.Errorf("inHeading(true): %+v, want %+v", span, want)
	}
	if got := paneSelSpan(43, 43, "43 To him", true, "Acts", 10).inHeading(false); got.heading || !got.hasLead || got.lo != 43 {
		t.Errorf("inHeading(false) changed the span: %+v", got)
	}
	if got := paneSelSpan(0, 0, "", false, "Acts", 10).inHeading(true); got.heading || got.valid() {
		t.Errorf("inHeading(true) on no span: %+v, want no span and no flag", got)
	}
	if got := selSpanInHeading(0); got != (selSpan{}) {
		t.Errorf("selSpanInHeading(0): %+v, want the zero span", got)
	}
}

// A text share that quotes nothing, as every heading word's does, is its
// citation line alone, with no blank lines above it; a share that quotes
// keeps the blank line that sets the quotation off from its citation.
func TestAShareThatQuotesNothingIsItsCitationLineAlone(t *testing.T) {
	const version = "Berean Standard Bible"
	if got, want := composeShareText("", "Genesis 4:17", version), "— Genesis 4:17 ("+version+")"; got != want {
		t.Errorf("nothing quoted: %q, want %q", got, want)
	}
	quote := "“The Wanderer went out . . . .”"
	if got, want := composeShareText(quote, "Genesis 4:16", version), quote+"\n\n— Genesis 4:16 ("+version+")"; got != want {
		t.Errorf("a quotation: %q, want %q", got, want)
	}
}

// Every word of every heading the public-domain translations' caches hold
// between two verses, as a double-tap selects it, and every such heading
// whole: with the span the native panes send, every verb cites the verse
// below the heading and quotes nothing, and no study verb is offered, by the
// desktop menu or by the native panes' rule on the page
// (paneHoldsScripture). Local-only and opt-in by
// BIBLETEXT_EXHAUSTIVE=1, as the other sweeps over the caches are. Headings
// at the top of a chapter are counted and left out: they stand before the
// first verse's number, where the native panes offer none of the app's verbs.
//
// The panes' span before, the verse above and the lead through the heading
// (headingPageSpan), is measured beside it and logged: the words it quoted,
// and the words it cited by the verse above.
func TestEveryWordOfEveryHeadingMeansTheSectionBelowOverLocalCaches_LocalOnly(t *testing.T) {
	if os.Getenv("BIBLETEXT_EXHAUSTIVE") == "" {
		t.Skip("local-only and opt-in: every heading word of three translation caches; run with BIBLETEXT_EXHAUSTIVE=1")
	}
	for _, c := range localPublicDomainCaches(t) {
		var headings, top, words, wholes, wrong int
		var beforeQuoted, beforeAbove, quotedHeadings, aboveHeadings int
		for _, book := range c.bd.Books {
			for _, ch := range c.bd.GetChapterNumbersForBook(book) {
				hs := headingsFor(c.bd, book, ch)
				if len(hs) == 0 {
					continue
				}
				st := &AppState{Bible: c.bd, CurrentBook: book, CurrentChapter: ch}
				corpus, spans := chapterProseIn(st, book, ch)
				if len(spans) == 0 {
					continue
				}
				page, pvs := drawnHeadingPage(corpus, spans, hs)
				for k := 0; k+1 < len(pvs); k++ {
					off := pvs[k].end
					for _, line := range strings.Split(page[pvs[k].end:pvs[k+1].num], "\n") {
						lineAt := off
						off += len(line) + 1
						if strings.TrimSpace(line) == "" {
							continue
						}
						headings++
						below, above := pvs[k+1].verse, pvs[k].verse
						var sels [][2]int
						for i := 0; i < len(line); {
							j := strings.IndexByte(line[i:], ' ')
							if j < 0 {
								j = len(line) - i
							}
							if j > 0 {
								sels = append(sels, [2]int{lineAt + i, lineAt + i + j})
							}
							i += j + 1
						}
						words += len(sels)
						wholes++
						sels = append(sels, [2]int{lineAt, lineAt + len(line)})
						quotedHere, aboveHere := false, false
						for n, s := range sels {
							sel := page[s[0]:s[1]]
							span := paneHeadingSpan(page, pvs, s[0], s[1])
							a := headingWordAnswers(st, book, ch, sel, span)
							held := paneHoldsScripture(page, pvs, s[0], s[1])
							if !span.heading || !a.sectionBelow(book, ch, below) || held {
								wrong++
								if wrong <= 5 {
									t.Logf("%s: %s %d, %q in the heading above verse %d: span %+v, answered %+v, panes' rule %v", c.id, book, ch, sel, below, span, a, held)
								}
							}
							if n == len(sels)-1 {
								continue // the whole heading: the before figures count words
							}
							q, cite, _, _ := prepareShareQuoteIn(st, book, ch, sel, headingPageSpan(page, pvs, s[0], s[1]))
							if q != "" {
								beforeQuoted++
								quotedHere = true
							}
							if cite == verseRangeCitation(book, ch, above, above) {
								beforeAbove++
								aboveHere = true
							}
						}
						if quotedHere {
							quotedHeadings++
						}
						if aboveHere {
							aboveHeadings++
						}
					}
				}
				// The headings standing above the first verse's number.
				for _, h := range hs {
					if collapseSpaces(h.DrawnText()) != "" && h.BeforeVerse > 0 && h.BeforeVerse <= pvs[0].verse {
						top++
					}
				}
			}
		}
		t.Logf("%s: %d headings between two verses (%d at a chapter's top left out), %d words and %d whole headings; answered otherwise than the section below: %d",
			c.id, headings, top, words, wholes, wrong)
		t.Logf("%s: with the span the panes sent before, %d of the %d words were quoted, under %d headings, and %d cited by the verse above, under %d",
			c.id, beforeQuoted, words, quotedHeadings, beforeAbove, aboveHeadings)
		if headings == 0 {
			continue
		}
		if wrong != 0 {
			t.Errorf("%s: %d heading selections were quoted or cited otherwise than the verse below", c.id, wrong)
		}
	}
}
