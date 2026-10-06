package bibletext

// A DRAG FROM WORD TO WORD SHARES THE WORDS IT TOOK. The share code places a
// selection once and never searches for it again (normalizeShareSelection),
// so the one search has to find a copy a drag from word to word could have
// made. It took the first copy of any kind, and where that stood inside a
// longer word, "the wicked" in "the wickedness", the end repair cut the
// reader's last word off; with no span the verse of that copy was cited too.

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"unicode"
)

// phraseWords is a phrase without the marks at either end, which the share
// code may drop (an orphan full stop at the front, a clause mark at the end)
// or keep (a closing question mark) without changing its words.
func phraseWords(s string) string {
	return strings.TrimFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

// sweepPhrases selects every phrase of two, three and five words inside each
// step-th verse of bd, once with the span a pane reports for a drag inside
// one verse and once with none, as the plain-text fallback the Mac and the
// iPhone and iPad show hands a selection over. Each must be placed, and
// share its own words; with the span it must cite its verse. With no span
// the citation is that of the first copy that splits no word, which for
// wording the chapter repeats need not be the reader's (BACKLOG.md, "A
// Psalm superscription can be shared as scripture"). It returns how many
// selections it made and how many were wrong, logging the first few.
func sweepPhrases(t *testing.T, bd *BibleData, books []string, step int, label string) (selections, wrong int) {
	t.Helper()
	shown := 0
	for _, book := range books {
		for _, ch := range bd.GetChapterNumbersForBook(book) {
			st := &AppState{Bible: bd, CurrentBook: book, CurrentChapter: ch}
			corpus, spans := chapterProse(st)
			for vi := 0; vi < len(spans); vi += step {
				sp := spans[vi]
				words := strings.Fields(corpus[sp.start:sp.end])
				for _, n := range []int{2, 3, 5} {
					for i := 0; i+n <= len(words); i++ {
						sel := strings.Join(words[i:i+n], " ")
						for _, span := range []selSpan{{sp.verse, sp.verse}, {}} {
							selections++
							text, lo, hi, _, ok := normalizeShareSelectionProse(st, book, ch, corpus, spans, sel, span)
							bad := ""
							switch {
							case !ok:
								bad = "not placed"
							case phraseWords(text) != phraseWords(sel):
								bad = fmt.Sprintf("shared %q", text)
							case span.valid() && (lo != sp.verse || hi != sp.verse):
								bad = fmt.Sprintf("cited %d–%d", lo, hi)
							}
							if bad == "" {
								continue
							}
							wrong++
							if shown < 5 {
								shown++
								t.Logf("%s: %s %d:%d %q span %d–%d: %s", label, book, ch, sp.verse, sel, span.lo, span.hi, bad)
							}
						}
					}
				}
			}
		}
	}
	return selections, wrong
}

// Every phrase of two, three and five words in every verse of the embedded
// Gospels, 214,115 of them, each with the span and without. Placed at the
// first copy of any kind, 59 with the span and 1,247 without were wrong: with
// the span Matthew 15:14's "blind guide" shared "blind", the verse having
// "blind guides" first, and with none Matthew 3:13's "to the" shared "to".
// None is now.
func TestEveryPhraseInAVerseSharesItsOwnWords(t *testing.T) {
	bd, err := loadSeedGospels()
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	step := 1
	if testing.Short() {
		step = 7
	}
	selections, wrong := sweepPhrases(t, bd, bd.Books, step, "Gospels")
	t.Logf("%d selections, %d wrong", selections, wrong)
	if wrong != 0 {
		t.Errorf("%d of %d phrases shared other words than the ones selected, or cited another verse with the span", wrong, selections)
	}
}

// The shape itself, on an invented verse that holds the phrase first inside
// a longer word: the drag's own words are shared, from the copy that splits
// no word, with the span and without. The control is the same verse without
// the longer word first, which the first copy of any kind answered too.
func TestAPhraseFirstFoundInsideALongerWordSharesItsOwnWords(t *testing.T) {
	for _, c := range []struct {
		name, text string
	}{
		{"the longer word first", "The proud boast of the wickedness they plan, but the wicked will fall."},
		{"control: no longer word", "The proud boast of their plans, but the wicked will fall."},
	} {
		bd := &BibleData{
			Books: []string{"Psalms"},
			Verses: map[string]map[int][]Verse{"Psalms": {7: {
				{BookName: "Psalms", Book: "Psalms", Chapter: 7, Verse: 1, Text: "Hear me, and answer my cry."},
				{BookName: "Psalms", Book: "Psalms", Chapter: 7, Verse: 2, Text: c.text},
			}}},
		}
		st := &AppState{Bible: bd, CurrentBook: "Psalms", CurrentChapter: 7}
		for _, span := range []selSpan{selSpanFromNative(2, 2), {}} {
			quote, cite, _, _ := prepareShareQuote(st, "the wicked", span)
			if quote != "the wicked" || cite != "Psalms 7:2" {
				t.Errorf("%s, span %d–%d: shared (%q, %q), want (\"the wicked\", \"Psalms 7:2\")", c.name, span.lo, span.hi, quote, cite)
			}
		}
	}
}

// The same sweep over the public-domain translations' caches the app keeps,
// every verse of every book. LOCAL-ONLY and opt-in by BIBLETEXT_EXHAUSTIVE=1,
// as TestPassageShareEqualsSelectionRouteOverLocalCaches_LocalOnly is: the
// caches are downloaded data a CI checkout does not have. Two phrases of the
// BSB, each with the span and without, are held apart: each ends on a dash
// as its verse does, and the verse has the same words earlier with a word
// straight after the dash ("herself—or" in Numbers 5:14, "truth—and" in 2
// John 1). That copy splits no word and is taken, and the end repair reads
// the letter after the dash as the rest of a word and cuts the phrase to its
// first word, as it did before the search changed. With the search as it
// was, the WEB had 12,161 wrong, the BSB 9,339 and the WEBC 13,855.
func TestEveryPhraseInAVerseSharesItsOwnWordsOverLocalCaches_LocalOnly(t *testing.T) {
	if os.Getenv("BIBLETEXT_EXHAUSTIVE") == "" {
		t.Skip("local-only and opt-in: about twelve million selections over three translation caches, half a minute; " +
			"run with BIBLETEXT_EXHAUSTIVE=1")
	}
	known := map[string]int{"bsb": 4}
	for _, c := range localPublicDomainCaches(t) {
		selections, wrong := sweepPhrases(t, c.bd, c.bd.Books, 1, c.id)
		t.Logf("%s: %d selections, %d wrong", c.id, selections, wrong)
		if wrong != known[c.id] {
			t.Errorf("%s: %d selections shared other words than the ones selected, or cited another verse with the span; want %d", c.id, wrong, known[c.id])
		}
	}
}

// localPublicDomainCaches loads the WEB, BSB and WEBC caches the app keeps,
// found as TestPassageShareEqualsSelectionRouteOverLocalCaches_LocalOnly
// finds them: the current epoch's file, else the newest superseded one. A
// translation with no cache is skipped; one whose cache will not load is an
// error. With none present the test is skipped.
func localPublicDomainCaches(t *testing.T) []struct {
	id string
	bd *BibleData
} {
	t.Helper()
	var out []struct {
		id string
		bd *BibleData
	}
	for _, id := range []string{"web", "bsb", "webc"} {
		v, ok := versionByID(id)
		if !ok {
			t.Fatalf("version %q is not in the catalogue", id)
		}
		var bd *BibleData
		for _, own := range append([]string{cachePathForVersion(id)}, supersededCachePaths(v)...) {
			path := realCachePath(own)
			if _, err := os.Stat(path); err != nil {
				continue
			}
			loaded, err := loadBibleFromCache(path)
			if err != nil {
				t.Errorf("%s: a cache is present but will not load: %v", id, err)
				continue
			}
			bd = loaded
			break
		}
		if bd == nil {
			t.Logf("%s: no cache present, skipped", id)
			continue
		}
		out = append(out, struct {
			id string
			bd *BibleData
		}{id, bd})
	}
	if len(out) == 0 {
		t.Skip("local-only: no translation cache present")
	}
	return out
}
