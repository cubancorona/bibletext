package bibletext

import (
	"strings"
	"testing"
)

// fallbackSpanChapter is an invented chapter in which "ove" stands in verse
// 3 before it stands in verse 4's "moved". verse3 is verse 3's text, so a
// control can take away the full stop after its copy.
func fallbackSpanChapter(verse3 string) *AppState {
	bd := NewBibleData()
	bd.Books = []string{"Probe"}
	bd.Verses["Probe"] = map[int][]Verse{1: {
		{BookName: "Probe", Book: "Probe", Chapter: 1, Verse: 1, Text: "He said peace to them."},
		{BookName: "Probe", Book: "Probe", Chapter: 1, Verse: 2, Text: "Give thanks to the Lord, for he is good."},
		{BookName: "Probe", Book: "Probe", Chapter: 1, Verse: 3, Text: verse3},
		{BookName: "Probe", Book: "Probe", Chapter: 1, Verse: 4, Text: "They moved on and came to the city."},
	}}
	return &AppState{Bible: bd, CurrentBook: "Probe", CurrentChapter: 1}
}

// A drag inside verse 4's "moved" hands over "ove" with the span 4–4. The
// normalize declines a piece of a word, and the share then placed the piece
// at its first copy in the chapter, verse 3's "Love.", and took that copy's
// full stop: "[O]ve." for words that run on in verse 4. Placed in the span's
// verses, the share ends with the omission its own copy needs. Controls: a
// verse 3 with no full stop after its copy, where both placements agree,
// and the same drag with no span, which has no verses to place it in and
// keeps the chapter's first copy.
func TestDeclinedSelectionIsPlacedInItsSpan(t *testing.T) {
	const drag = "ove"
	span := selSpanFromNative(4, 4)
	for _, c := range []struct {
		name, verse3 string
	}{
		{"an earlier copy ends a sentence", "Love."},
		{"control: the earlier copy runs on", "Loved them."},
	} {
		state := fallbackSpanChapter(c.verse3)
		if _, _, _, _, ok := normalizeShareSelectionIn(state, "Probe", 1, drag, span); ok {
			t.Fatalf("%s: the normalize located a piece of a word, so the fallback is not under test", c.name)
		}
		if quote, cite := shareQuoteIn(state, "Probe", 1, drag, span); quote != "“[O]ve . . . .”" || cite != "Probe 1:4" {
			t.Errorf("%s: the share quotes %q citing %q, want “[O]ve . . . .” citing Probe 1:4", c.name, quote, cite)
		}
	}
	state := fallbackSpanChapter("Love.")
	corpus, spans := chapterProseIn(state, "Probe", 1)
	v4 := spans[len(spans)-1]
	want := v4.start + strings.Index(corpus[v4.start:v4.end], drag)
	if text, _, at, baseLen := prepareShareQuoteIn(state, "Probe", 1, drag, span); at != want || baseLen != len(drag) || text != drag {
		t.Errorf("the fallback gives %q at %d (base %d), want %q at verse 4's copy, %d (base %d)", text, at, baseLen, drag, want, len(drag))
	}
	if text, cite, at, _ := prepareShareQuoteIn(state, "Probe", 1, drag, selSpan{}); at != -1 || text != "ove." || cite != "Probe 1" {
		t.Errorf("control, no span: got %q at %d citing %q, want the chapter's first copy, \"ove.\" at -1, citing Probe 1", text, at, cite)
	}
}
