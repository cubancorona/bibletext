package bibletext

// The Windows and Linux pane sends the selection's lead with its span
// (styledReadingPane.selectedVerseSpan, selSpan.lead): the model's text from
// verse lo's number run to the selection's first rune. These tests lay out
// real chapters and select through the pane's own model, so the lead is the
// one the pane builds, and hold the share pipeline's placement of every such
// selection to the reader's own place.

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

// styledWordTokens is verse v's words as the layout set them, in order: the
// runWord runs that carry v.
func styledWordTokens(lay *chapterLayout, v int) []styledRun {
	var out []styledRun
	for _, ln := range lay.Lines {
		for _, r := range ln.Runs {
			if r.Kind == runWord && r.Verse == v {
				out = append(out, r)
			}
		}
	}
	return out
}

// letterRange is the rune range of tok's letters and digits, without the
// marks at either end; ok is false for a token with none.
func letterRange(tok string) (a, b int, ok bool) {
	rs := []rune(tok)
	a, b = -1, -1
	for i, r := range rs {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if a < 0 {
				a = i
			}
			b = i + 1
		}
	}
	return a, b, a >= 0
}

// Every selection of one and two words made letter to letter in every verse
// of the embedded Gospels, through the styled pane's own model: with the
// lead the pane sends, each is placed at its own offset in the chapter with
// its own words and cited by its own verse. Before the pane sent the lead,
// a later copy of a word its verse repeats was placed at the first.
func TestTheStyledPanesSelectionsArePlacedByTheirLead(t *testing.T) {
	bd, err := loadSeedGospels()
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	step := 1
	if testing.Short() {
		step = 7
	}
	verses, selections, laterCopies, skipped, wrong, laterWithoutLead := 0, 0, 0, 0, 0, 0
	for _, book := range bd.Books {
		for _, ch := range bd.GetChapterNumbersForBook(book) {
			st := &AppState{Bible: bd, CurrentBook: book, CurrentChapter: ch}
			corpus, spans := chapterProseIn(st, book, ch)
			lay := layoutChapter(st, bd.GetChapter(book, ch), testLayoutParams, fixedMeasure)
			for vi := 0; vi < len(spans); vi += step {
				sp := spans[vi]
				verses++
				toks := styledWordTokens(lay, sp.verse)
				texts := make([]string, len(toks))
				for i, r := range toks {
					texts[i] = r.Text
				}
				body := corpus[sp.start:sp.end]
				if strings.Join(texts, " ") != body {
					skipped++ // the layout's words are not the corpus's, word for word
					continue
				}
				// The byte offset of each token in the verse's body.
				offs := make([]int, len(toks))
				for i, o := 1, len(texts[0])+1; i < len(texts); i++ {
					offs[i] = o
					o += len(texts[i]) + 1
				}
				for n := 1; n <= 2; n++ {
					for i := 0; i+n <= len(toks); i++ {
						a, _, okA := letterRange(toks[i].Text)
						_, b, okB := letterRange(toks[i+n-1].Text)
						if !okA || !okB {
							continue
						}
						p := &styledReadingPane{lay: lay, selStart: toks[i].Offset + a, selEnd: toks[i+n-1].Offset + b}
						span := p.selectedVerseSpan()
						raw := plainSelection(p.selectedRaw())
						start := offs[i] + len(string([]rune(toks[i].Text)[:a]))
						end := offs[i+n-1] + len(string([]rune(toks[i+n-1].Text)[:b]))
						sel, pos := body[start:end], sp.start+start
						selections++
						text, lo, hi, at, ok := normalizeShareSelectionProse(st, book, ch, corpus, spans, raw, span)
						if !span.hasLead || !ok || at != pos || text != sel || lo != sp.verse || hi != sp.verse {
							wrong++
							if wrong <= 5 {
								t.Logf("%s %d:%d %q: lead %q (sent %v) placed (%q, %d–%d, at %d, ok %v), its place is %d",
									book, ch, sp.verse, sel, span.lead, span.hasLead, text, lo, hi, at, ok, pos)
							}
							continue
						}
						if first := wordAlignedIndex(body, sel); first >= 0 && first != start {
							laterCopies++
							plain := selSpanFromNative(span.lo, span.hi)
							if _, _, _, at, _ := normalizeShareSelectionProse(st, book, ch, corpus, spans, raw, plain); at != pos {
								laterWithoutLead++
							}
						}
					}
				}
			}
		}
	}
	t.Logf("%d selections through the pane, %d of them later copies (%d placed elsewhere without the lead); %d of %d verses skipped, %d wrong",
		selections, laterCopies, laterWithoutLead, skipped, verses, wrong)
	if wrong != 0 {
		t.Errorf("%d of %d selections through the styled pane were not placed at their own place by their lead", wrong, selections)
	}
	if selections == 0 || skipped*100 > verses {
		t.Errorf("%d selections, %d of %d verses skipped: the sweep no longer sees the pane's words", selections, skipped, verses)
	}
	if laterCopies == 0 || laterWithoutLead != laterCopies {
		t.Errorf("control: of %d later copies %d were placed elsewhere without the lead; all of them are expected to be", laterCopies, laterWithoutLead)
	}
}

// The lead's edges in the pane's model: a selection begun in the space
// before a verse number sends the empty lead, as one begun on the number
// does, and one begun inside a word sends the text before it. An omitted
// verse's mark, which stands ahead of the next number on the page, is drawn
// outside the model (TestTheStyledPaneDrawsTheMarkOutsideTheTextModel), so a
// selection cannot begin on it.
func TestTheStyledPaneSendsTheLeadFromTheVerseNumber(t *testing.T) {
	st := leadChapter("Acts", 8, map[int]string{
		36: "As they went on the way, they came to some water.",
		38: "He commanded the chariot to stand still.",
	})
	lay := layoutChapter(st, st.Bible.GetChapter("Acts", 8), testLayoutParams, fixedMeasure)
	text := []rune(lay.Text)
	num38 := -1
	for _, ln := range lay.Lines {
		for _, r := range ln.Runs {
			if r.Verse == 38 && r.Kind == runVerseNum {
				num38 = r.Offset
			}
		}
	}
	if num38 < 0 {
		t.Fatalf("no number run for verse 38 in %q", lay.Text)
	}
	after := string(text[num38:])
	commanded := num38 + utf8.RuneCountInString(after[:strings.Index(after, "commanded")])
	for _, c := range []struct {
		name       string
		start, end int
		lead       string
		hasLead    bool
		wantLo     int
	}{
		{"on the number", num38, len(text), "", true, 38},
		{"inside a word", commanded + 3, len(text), string(text[num38 : commanded+3]), true, 38},
	} {
		p := &styledReadingPane{lay: lay, selStart: c.start, selEnd: c.end}
		span := p.selectedVerseSpan()
		if span.lo != c.wantLo || span.hasLead != c.hasLead || span.lead != c.lead {
			t.Errorf("%s: span %d–%d lead %q (sent %v), want %d lead %q (sent %v)", c.name, span.lo, span.hi, span.lead, span.hasLead, c.wantLo, c.lead, c.hasLead)
		}
	}
	// The space before the number: the selection starts one rune before it.
	if num38 > 0 && text[num38-1] == ' ' {
		p := &styledReadingPane{lay: lay, selStart: num38 - 1, selEnd: len(text)}
		if span := p.selectedVerseSpan(); span.lo != 38 || !span.hasLead || span.lead != "" {
			t.Errorf("begun in the space before 38: span %d–%d lead %q (sent %v), want 38 with the empty lead", span.lo, span.hi, span.lead, span.hasLead)
		}
	}
}
