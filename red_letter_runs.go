package bibletext

import "strings"

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
//   - red-letter off, or not a words-of-Christ verse → one run, not red.
//   - the BSB, with usable span data → runs alternating between His words and
//     the narration or other speakers around them.
//   - anything else → one run, red. That is the whole-verse behaviour every
//     pane had before spans existed, and it stays the fallback: the WEB and the
//     NKJV have no span data yet, and the BSB's is refused when the verse text
//     does not match what the offsets were computed against.
func redLetterRuns(versionID string, v Verse, redLetter bool) []verseRun {
	if !redLetter || !isWordsOfChrist(v.BookName, v.Chapter, v.Verse) {
		return []verseRun{{Text: v.Text}}
	}
	if versionID == "bsb" {
		if spans, ok := bsbRedLetterSpansFor(v.BookName, v.Chapter, v.Verse, v.Text); ok {
			return runsFromSpans(v.Text, spans)
		}
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
