package bibletext

import (
	"strings"
	"testing"
)

// suffixNumberChapter is an invented chapter whose verse 6 opens with the
// same twelve runes as verse 26 ("God said, “L"), with verse 25 between
// them, the shape Genesis 1:6 and 1:26 have in the WEB. v6 is verse 6's
// text, so a control can give it an opening of its own.
func suffixNumberChapter(v6 string) *AppState {
	bd := NewBibleData()
	bd.Books = []string{"Probe"}
	bd.Verses["Probe"] = map[int][]Verse{1: {
		{BookName: "Probe", Book: "Probe", Chapter: 1, Verse: 6, Text: v6},
		{BookName: "Probe", Book: "Probe", Chapter: 1, Verse: 25, Text: "God made the animals of the earth after their kind."},
		{BookName: "Probe", Book: "Probe", Chapter: 1, Verse: 26, Text: "God said, “Let’s make man in our image, after our likeness.”"},
	}}
	return &AppState{Bible: bd, CurrentBook: "Probe", CurrentChapter: 1}
}

// A drag begun on the second digit of verse 25's number, through verse 25
// and over verse 26's number: the leading "5 " is in no verse, so the
// positional locate declines and the selection takes the fallback's text
// pass, in the share and in the Ask sheet's and answer panel's preview.
const suffixNumberDrag = "5 God made the animals of the earth after their kind. 26 God said, “Let’s make man"

// The fallback's marker strip removes a verse number only where it stands
// as a token of its own. It used to remove "N " plus the verse's first
// twelve runes wherever that string stood, so verse 6's "6 God said, “L"
// matched inside "26 God said, “L" and the share and the AI preview both
// read "2God said". 339 verses of the WEB and the BSB have that shape.
// Control: verse 6 opening differently, which the old strip got right too.
func TestFallbackMarkerStripLeavesALaterNumberWhole(t *testing.T) {
	const want = "5 God made the animals of the earth after their kind. God said, “Let’s make man"
	for _, c := range []struct {
		name string
		v6   string
	}{
		{"same opening as verse 26", "God said, “Let there be an expanse in the middle of the waters.”"},
		{"control: an opening of its own", "God spoke, “Let there be an expanse in the middle of the waters.”"},
	} {
		state := suffixNumberChapter(c.v6)
		if got := cleanQuoteTextIn(state, "Probe", 1, suffixNumberDrag); got != want {
			t.Errorf("%s: the fallback strip gives %q, want %q", c.name, got, want)
		}
		for _, span := range []selSpan{selSpanFromNative(25, 26), {}} {
			if _, _, _, _, ok := normalizeShareSelectionIn(state, "Probe", 1, suffixNumberDrag, span); ok {
				t.Fatalf("%s %+v: the positional path located the drag, so the fallback is not under test", c.name, span)
			}
			text, _, at, _ := prepareShareQuoteIn(state, "Probe", 1, suffixNumberDrag, span)
			if at != -1 {
				t.Fatalf("%s %+v: the share took the located path (at=%d)", c.name, span, at)
			}
			if strings.Contains(text, "2God") || !strings.Contains(text, "God said, “Let’s make man") {
				t.Errorf("%s %+v: the share's quote %q lost verse 26's number from inside it", c.name, span, text)
			}
			if got := aiPassageFor(state, suffixNumberDrag, span).previewText(); got != want {
				t.Errorf("%s %+v: the AI preview reads %q, want %q", c.name, span, got, want)
			}
		}
	}
}
