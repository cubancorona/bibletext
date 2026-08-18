package bibletext

// Real-world sweep for the Bluebook share formatter: instead of hand-authored
// fragments, run THOUSANDS of realistic reader selections over the actual
// embedded World English Bible Gospels (assets/seed/web-gospels.json — real
// nested speech, questions, red-letter passages, long discourses) and assert
// the Rule 5 invariants on every output. This is the closest a unit test gets
// to what readers actually select: word-boundary spans of genuine scripture,
// with and without the verse-number markers the reading view leaks into a
// selection.
//
// Invariants checked on every generated share:
//   I1  no single-glyph "…" is ever introduced
//   I2  double quotation marks are balanced and never close before opening
//   I3  the quote ends on terminal punctuation (inside any closing marks) —
//       either the source's own or a correctly-spaced four-dot mark; a bare
//       three-dot ending never appears (Rule 5.3(b)(iii))
//   I4  the quote never opens with an ellipsis; a mid-sentence (lowercase)
//       start gets its bracketed capital (Rule 5.3(b)(i))
//   I5  the 50-word threshold counts QUOTED words (marks excluded): 49-word
//       selections are inline-quoted, 50+ are unmarked blocks (Rule 5.1)
//   I6  the share is verbatim: letters and digits of the output equal those of
//       the cleaned selection (case-folded for the one disclosed [X] change)
//   I7  whole-verse selections carrying "N " verse-number markers clean to
//       exactly the verses' own text
//   I8  originalSentenceTerminal only ever reports . ! ?

import (
	"math/rand"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

// rwState builds an AppState over one real seed chapter.
func rwState(t *testing.T, bd *BibleData, book string, chapter int) *AppState {
	t.Helper()
	return &AppState{Bible: bd, CurrentBook: book, CurrentChapter: chapter}
}

// rwLetters reduces a string to its letters+digits for the verbatim check (I6).
func rwLetters(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

// rwCheckInvariants asserts I1–I6 for one formatted share.
func rwCheckInvariants(t *testing.T, label, cleaned, quote string, quotedWords int) {
	t.Helper()
	if strings.TrimSpace(cleaned) == "" {
		return
	}

	// I1 — no single-glyph ellipsis introduced.
	if !strings.Contains(cleaned, "…") && strings.Contains(quote, "…") {
		t.Errorf("%s: introduced single-glyph ellipsis: %q", label, quote)
		return
	}

	// I2 — balanced double marks, depth never negative.
	depth := 0
	for _, r := range quote {
		switch r {
		case '“':
			depth++
		case '”':
			depth--
		}
		if depth < 0 {
			t.Errorf("%s: closing mark before opener: %q", label, quote)
			return
		}
	}
	if depth != 0 {
		t.Errorf("%s: unbalanced double marks: %q", label, quote)
		return
	}

	// I3 — terminal correctness. Strip closing quotation marks, then require
	// terminal punctuation; if the four-dot mark is present it must be exactly
	// the spaced form, and a bare three-dot ending must never appear.
	core := strings.TrimRight(quote, "”’\"'")
	if core == "" {
		return
	}
	last := []rune(core)[len([]rune(core))-1]
	switch last {
	case '.', '!', '?', '…':
	default:
		t.Errorf("%s: does not end on terminal punctuation: %q", label, quote)
		return
	}
	if strings.HasSuffix(core, " . .") && !strings.HasSuffix(core, ". . . .") &&
		!strings.HasSuffix(core, ". . . !") && !strings.HasSuffix(core, ". . . ?") {
		t.Errorf("%s: malformed omission mark: %q", label, quote)
		return
	}
	// A bare three-dot ending (ellipsis with no fourth mark) is never valid.
	if strings.HasSuffix(core, " . . .") && !strings.HasSuffix(core, " . . . .") {
		t.Errorf("%s: bare three-dot ending: %q", label, quote)
		return
	}

	// I4 — beginning correctness.
	inner := strings.TrimLeft(quote, "“‘\"'")
	if strings.HasPrefix(inner, ".") {
		t.Errorf("%s: begins with an ellipsis: %q", label, quote)
		return
	}
	cleanedInner := strings.TrimLeft(cleaned, "“‘\"'")
	if cleanedInner != "" {
		first := []rune(cleanedInner)[0]
		if unicode.IsLower(first) {
			wantStart := "[" + string(unicode.ToUpper(first)) + "]"
			if !strings.HasPrefix(inner, wantStart) {
				t.Errorf("%s: lowercase start %q not bracket-capitalized: %q", label, string(first), quote)
				return
			}
		}
	}

	// I5 — block threshold on quoted words.
	if quotedWords >= blockQuoteWords {
		if strings.HasPrefix(quote, "“") && strings.HasSuffix(quote, "”") &&
			!strings.Contains(cleaned, "“") {
			t.Errorf("%s: %d-word block wrongly wrapped in outer marks: %q", label, quotedWords, quote[:40])
			return
		}
	} else if !strings.HasPrefix(quote, "“") && !strings.Contains(cleaned, "\"") {
		t.Errorf("%s: %d-word inline quote missing outer marks: %q", label, quotedWords, quote[:min(40, len(quote))])
		return
	}

	// I6 — verbatim: letters/digits preserved exactly (case-folded).
	if rwLetters(quote) != rwLetters(cleaned) {
		t.Errorf("%s: letters diverge from the selection\n sel: %q\n out: %q", label, cleaned, quote)
	}
}

func TestRealWorldGospelSweep(t *testing.T) {
	bd, err := loadSeedGospels()
	if err != nil {
		t.Fatalf("seed gospels: %v", err)
	}
	rng := rand.New(rand.NewSource(20260706)) // deterministic
	shares := 0

	for _, book := range bd.Books {
		chapters := bd.GetChapterNumbersForBook(book)
		for _, ch := range chapters {
			verses := bd.GetChapter(book, ch)
			if len(verses) == 0 {
				continue
			}
			state := rwState(t, bd, book, ch)

			// The chapter's prose, as cleanQuoteText reconstructs it.
			var parts []string
			for _, v := range verses {
				if tx := collapseSpaces(v.Text); tx != "" {
					parts = append(parts, tx)
				}
			}
			prose := strings.Join(parts, " ")
			words := strings.Fields(prose)
			if len(words) < 4 {
				continue
			}

			// (a) whole-verse ranges WITH the reading view's "N " markers (I7).
			for k := 0; k < 3; k++ {
				lo := rng.Intn(len(verses))
				hi := lo + rng.Intn(min(6, len(verses)-lo))
				var sel, want []string
				for _, v := range verses[lo : hi+1] {
					tx := collapseSpaces(v.Text)
					if tx == "" {
						continue
					}
					sel = append(sel, strconv.Itoa(v.Verse)+" "+tx)
					want = append(want, tx)
				}
				if len(sel) == 0 {
					continue
				}
				cleaned := cleanQuoteText(state, strings.Join(sel, " "))
				if cleaned != strings.Join(want, " ") {
					t.Errorf("%s %d: verse markers not cleaned\n got %q\nwant %q",
						book, ch, cleaned, strings.Join(want, " "))
					continue
				}
				term := originalSentenceTerminal(state, cleaned, -1, 0)
				if term != '.' && term != '!' && term != '?' {
					t.Errorf("%s %d: bad terminal %q", book, ch, term) // I8
				}
				q := formatBibleQuote(cleaned, term)
				rwCheckInvariants(t, book+" "+strconv.Itoa(ch)+" verses", cleaned, q, len(strings.Fields(cleaned)))
				shares++
			}

			// (b) word-boundary spans of the raw prose — short phrases through
			// 60+ word cuts, starting anywhere (lowercase mid-sentence included).
			for k := 0; k < 8; k++ {
				start := rng.Intn(len(words))
				n := 1 + rng.Intn(60)
				end := min(start+n, len(words))
				sel := strings.Join(words[start:end], " ")
				cleaned := cleanQuoteText(state, sel)
				term := originalSentenceTerminal(state, cleaned, -1, 0)
				if term != '.' && term != '!' && term != '?' {
					t.Errorf("%s %d: bad terminal %q", book, ch, term) // I8
				}
				q := formatBibleQuote(cleaned, term)
				rwCheckInvariants(t, book+" "+strconv.Itoa(ch)+" span", cleaned, q, len(strings.Fields(cleaned)))
				shares++
			}
		}
	}
	if shares < 800 {
		t.Fatalf("sweep too small to mean anything: only %d shares exercised", shares)
	}
	t.Logf("checked %d real-scripture shares", shares)
}

// TestRealWorldFamousPassages pins a handful of recognizable passages end to
// end against the REAL seed text — structural expectations, resilient to
// translation wording.
func TestRealWorldFamousPassages(t *testing.T) {
	bd, err := loadSeedGospels()
	if err != nil {
		t.Fatalf("seed gospels: %v", err)
	}

	verseText := func(book string, ch, v int) string {
		for _, vs := range bd.GetChapter(book, ch) {
			if vs.Verse == v {
				return collapseSpaces(vs.Text)
			}
		}
		t.Fatalf("%s %d:%d missing from seed", book, ch, v)
		return ""
	}

	// John 3:16 — a complete verse: inline, wrapped, no omission mark.
	{
		state := rwState(t, bd, "John", 3)
		sel := verseText("John", 3, 16)
		q := formatBibleQuote(cleanQuoteText(state, sel), originalSentenceTerminal(state, sel, -1, 0))
		if !strings.HasPrefix(q, "“") || !strings.HasSuffix(q, "”") {
			t.Errorf("John 3:16 should be inline-quoted: %q", q)
		}
		if strings.Contains(q, ". . .") {
			t.Errorf("John 3:16 is complete — no omission mark expected: %q", q)
		}
	}

	// Matthew 27:46 — contains Jesus' quoted cry: the verse's own double marks
	// must nest to singles inside the outer pair.
	{
		state := rwState(t, bd, "Matthew", 27)
		sel := verseText("Matthew", 27, 46)
		q := formatBibleQuote(cleanQuoteText(state, sel), originalSentenceTerminal(state, sel, -1, 0))
		if strings.Contains(sel, "“") && !strings.Contains(q, "‘") {
			t.Errorf("Matthew 27:46 internal quotes should nest to singles: %q", q)
		}
		if strings.Count(q, "“") != 1 {
			t.Errorf("Matthew 27:46 should carry exactly one outer opening mark: %q", q)
		}
	}

	// Matthew 5:3–12 (the Beatitudes) — 50+ words: a block, no outer marks.
	{
		state := rwState(t, bd, "Matthew", 5)
		var sel []string
		for v := 3; v <= 12; v++ {
			sel = append(sel, verseText("Matthew", 5, v))
		}
		s := strings.Join(sel, " ")
		q := formatBibleQuote(cleanQuoteText(state, s), originalSentenceTerminal(state, s, -1, 0))
		if strings.HasPrefix(q, "“") {
			t.Errorf("Beatitudes (50+ words) must be a block without outer marks: %q", q[:40])
		}
	}

	// John 11:35 — the shortest verse: complete, inline, verbatim.
	{
		state := rwState(t, bd, "John", 11)
		sel := verseText("John", 11, 35)
		q := formatBibleQuote(cleanQuoteText(state, sel), originalSentenceTerminal(state, sel, -1, 0))
		if q != "“"+sel+"”" {
			t.Errorf("John 11:35:\n got %q\nwant %q", q, "“"+sel+"”")
		}
	}
}
