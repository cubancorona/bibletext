package bibletext

import (
	"strings"
	"unicode"
)

// verseRun is a stretch of a verse that is uniformly Christ's words or not.
//
// Every reading pane renders a verse the same way in principle — walk the runs,
// colour the red ones — so the decision about WHICH words are His is made once,
// here, rather than four times in four dialects of markup.
type verseRun struct {
	Text string
	Red  bool
}

// redLetterRuns splits a verse into runs by whether the words are Christ's.
//
// Three outcomes, in order:
//
//   - red-letter off, or the edition does not mark the verse → one black run.
//   - usable publisher span data → alternating red and black runs.
//   - the edition marks the verse but its offsets are stale/unavailable → one
//     red run. This fallback stays inside the same edition's judgement.
func redLetterRuns(versionID string, v Verse, redLetter bool) []verseRun {
	if !redLetter {
		return []verseRun{{Text: v.Text}}
	}
	if spans, ok := redLetterSpansFor(versionID, v.BookName, v.Chapter, v.Verse, v.Text); ok {
		return runsFromSpans(v.Text, spans)
	}
	// A missing entry means black for that edition. A present entry with stale
	// offsets remains red at verse granularity; it never consults WEB unless the
	// selected edition itself is WEB/WEBC.
	if !redLetterVerseMarked(versionID, v.BookName, v.Chapter, v.Verse) {
		return []verseRun{{Text: v.Text}}
	}
	return []verseRun{{Text: v.Text, Red: true}}
}

// runsFromSpans turns rune offsets into alternating runs. The spans are sorted
// and non-overlapping (a generated-data invariant the tests pin), so this is a
// single walk; anything outside them is somebody else's words or the narrator's.
func runsFromSpans(text string, spans []redLetterSpan) []verseRun {
	r := []rune(text)
	out := make([]verseRun, 0, len(spans)*2+1)
	at := 0
	for _, s := range spans {
		if s.Start > at {
			out = append(out, verseRun{Text: string(r[at:s.Start])})
		}
		out = append(out, verseRun{Text: string(r[s.Start:s.End]), Red: true})
		at = s.End
	}
	if at < len(r) {
		out = append(out, verseRun{Text: string(r[at:])})
	}
	return out
}

// trimRuns trims the verse as a whole — leading space off the front, trailing
// space off the back — without disturbing the space BETWEEN runs, which belongs
// to the text. Panes previously called strings.TrimSpace on the whole verse;
// doing that per run would glue "he said" onto the quotation beside it.
func trimRuns(runs []verseRun) []verseRun {
	if len(runs) == 0 {
		return runs
	}
	runs[0].Text = strings.TrimLeft(runs[0].Text, " \t\n")
	last := len(runs) - 1
	runs[last].Text = strings.TrimRight(runs[last].Text, " \t\n")
	out := runs[:0]
	for _, r := range runs {
		if r.Text != "" {
			out = append(out, r)
		}
	}
	return out
}

// tokenSpan is one wrap token's half-open rune range within its verse's text.
type tokenSpan struct {
	Start int
	End   int
}

// verseTokenSpans locates each of verseTokens(v)'s tokens inside the verse text.
//
// The styled desktop pane draws one canvas.Text per token, so to ask the spans
// whether a token is Christ's it first has to know where that token IS; tokens
// carry no offsets of their own. verseTokens splits each authored line with
// strings.Fields, so the tokens are the verse's whitespace-delimited fields in
// order — plus "\n" sentinels between authored lines (zero-width here, since a
// sentinel is never drawn) and the superscript verse number glued to the front
// of the first one.
//
// The text is SCANNED rather than the offsets accumulated as token length plus
// one space. That shortcut is what I reached for first and it is wrong: the
// separators are not all single spaces — an authored poem line is a "\n" and
// nothing stops the supplier emitting two spaces after a sentence — and every
// such separator slides all the later offsets, which paints the red onto the
// neighbouring words instead of failing. Each token is therefore checked against
// the text it landed on, and ok=false the moment one disagrees so the caller can
// fall back to the whole-verse answer rather than colour arbitrary words.
func verseTokenSpans(v Verse, tokens []string) ([]tokenSpan, bool) {
	r := []rune(v.Text)
	spans := make([]tokenSpan, len(tokens))
	num := superscriptNumber(v.Verse)
	at := 0
	first := true
	for i, tok := range tokens {
		if tok == "\n" {
			spans[i] = tokenSpan{Start: at, End: at}
			continue
		}
		// Only the first CONTENT token carries the number, exactly as
		// layoutChapter strips it — a sentinel does not consume the prefix.
		word := tok
		if first && num != "" && strings.HasPrefix(tok, num+" ") {
			word = strings.TrimPrefix(tok, num+" ")
		}
		first = false

		for at < len(r) && unicode.IsSpace(r[at]) {
			at++
		}
		start := at
		for at < len(r) && !unicode.IsSpace(r[at]) {
			at++
		}
		if string(r[start:at]) != word {
			return nil, false
		}
		spans[i] = tokenSpan{Start: start, End: at}
	}
	return spans, true
}

// redLetterTokenFlags answers, token by token, which of verseTokens(v)'s tokens
// are Christ's words: the per-token form of redLetterRuns, for a pane that can
// only colour whole tokens because a token is the smallest thing it draws.
//
// A token counts as His as soon as ANY of its runes falls inside a red run.
// That rule is NOT hypothetical, and an earlier version of this comment was
// wrong to imply it: two shipping BSB spans end mid-token — Mark 7:34's closes
// inside "opened!”)." and Acts 20:35's inside "receive.’”". It decides toward
// red on purpose, because a span stopping one rune short would otherwise drop
// the colour off a quotation's closing mark, which reads as a rendering fault
// rather than as an editorial line.
//
// The consequence is a real cross-platform difference, recorded here because
// nothing else says it: on those two verses this pane paints 3 characters red
// that the Apple and Android panes leave in body colour, since those split at
// rune level (redLetterRuns) while this one can only colour whole tokens.
//
// Every mapped edition may yield multiple runs. A uniformly red/black verse, or
// a verse whose tokens cannot be matched back to its own text, takes the single
// whole-token fallback below.
func redLetterTokenFlags(versionID string, v Verse, redLetter bool, tokens []string) []bool {
	runs := redLetterRuns(versionID, v, redLetter)

	// The whole-verse answer — what every pane did before spans: red iff this
	// verse is marked as Christ's at all. It is read off the runs rather than
	// re-derived from isWordsOfChrist so there stays one decision, not two.
	whole := false
	for _, run := range runs {
		if run.Red {
			whole = true
			break
		}
	}
	uniform := func() []bool {
		flags := make([]bool, len(tokens))
		for i := range flags {
			flags[i] = whole
		}
		return flags
	}
	if len(runs) < 2 {
		return uniform()
	}

	// The runs partition the verse text exactly, so flattening their redness
	// rune by rune answers for any offset a token can occupy.
	red := make([]bool, 0, len(v.Text))
	for _, run := range runs {
		for range run.Text { // ranges by rune, which is what the offsets count
			red = append(red, run.Red)
		}
	}
	spans, ok := verseTokenSpans(v, tokens)
	if !ok {
		return uniform()
	}

	flags := make([]bool, len(tokens))
	for i, s := range spans {
		for j := s.Start; j < s.End && j < len(red); j++ {
			if red[j] {
				flags[i] = true
				break
			}
		}
	}
	return flags
}
