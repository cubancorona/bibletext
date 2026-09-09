package bibletext

import (
	"encoding/json"
	"strings"
	"testing"
)

// The census must be silent on the styles the decoder already answers for, and
// must name the ones it does not. Its whole value is being readable on the day
// it fires, which means saying nothing on every other day.

func TestTheStyleCensusIsSilentOnStylesWithAnAnswer(t *testing.T) {
	r := &recorder{}
	c := newAPIBibleStyleCensus("Psalms", r.logf)
	for _, style := range []string{"p", "q1", "q2", "s", "qa", "d", "pc"} {
		c.para(style, true)
	}
	for _, style := range []string{"nd", "sc", "wj", "it", "add"} {
		c.char(style)
	}
	c.report()
	if len(r.lines) != 0 {
		t.Errorf("the census spoke about styles the decoder handles:\n%s", r.joined())
	}
}

// CONTROL: an unanswered paragraph style must be named, and must say whether it
// carried text — that split is what decides later whether skipping it costs an
// epoch.
func TestAnUnansweredParagraphStyleIsNamedWithItsTextSplit(t *testing.T) {
	r := &recorder{}
	c := newAPIBibleStyleCensus("Genesis", r.logf)
	c.para("iex", true) // an introduction block that carried words
	c.para("sd", false) // a semantic divider that carried none
	c.para("p", true)   // known: must not appear
	c.report()

	got := r.joined()
	if !strings.Contains(got, "Genesis") {
		t.Errorf("the report does not name the book:\n%s", got)
	}
	if !strings.Contains(got, "iex (with text)") {
		t.Errorf("a text-bearing unknown style was not reported as such:\n%s", got)
	}
	if !strings.Contains(got, "sd (no text)") {
		t.Errorf("a text-free unknown style was not reported as such:\n%s", got)
	}
	if strings.Contains(got, "p (with") {
		t.Errorf("a known style was reported as unknown:\n%s", got)
	}
}

func TestAnUnansweredCharacterStyleIsNamed(t *testing.T) {
	r := &recorder{}
	c := newAPIBibleStyleCensus("Matthew", r.logf)
	c.char("fig") // a figure caption: not Scripture, and walked into text today
	c.char("wj")  // known
	c.report()
	got := r.joined()
	if !strings.Contains(got, "fig") {
		t.Errorf("an unanswered character style was not named:\n%s", got)
	}
	if strings.Contains(got, "wj×") {
		t.Errorf("a known character style was reported:\n%s", got)
	}
}

// The text split is computed from the block's own items, nested included.
func TestBlockTextDetectionSeesNestedItems(t *testing.T) {
	nested := []apiBibleNode{{Items: []apiBibleNode{{Text: "  "}, {Items: []apiBibleNode{{Text: "words"}}}}}}
	if !apiBibleBlockHasText(nested) {
		t.Error("text nested two levels down was not seen")
	}
	blank := []apiBibleNode{{Items: []apiBibleNode{{Text: "   "}, {Text: "\n"}}}}
	if apiBibleBlockHasText(blank) {
		t.Error("whitespace was mistaken for text, which would make every empty block look text-bearing")
	}
	if apiBibleBlockHasText(nil) {
		t.Error("an empty block reported text")
	}
}

func TestANilStyleCensusIsSafe(t *testing.T) {
	var c *apiBibleStyleCensus
	c.para("p", true)
	c.char("wj")
	c.report()
	if c.styles() != nil {
		t.Error("a nil census should report no styles")
	}
}

// The census must not change a single character of what the decoder produces —
// that is the whole basis for shipping it without a cache epoch.
func TestTheCensusChangesNothingItDecodes(t *testing.T) {
	const passage = `[
	  {"name":"para","type":"tag","attrs":{"style":"p"},"items":[
	    {"name":"verse","type":"tag","attrs":{"style":"v","number":"1","sid":"GEN 1:1"},"items":[{"type":"text","text":"1"}]},
	    {"type":"text","text":"In the beginning ","attrs":{"verseId":"GEN.1.1"}},
	    {"name":"char","type":"tag","attrs":{"style":"nd"},"items":[{"type":"text","text":"God","attrs":{"verseId":"GEN.1.1"}}]},
	    {"type":"text","text":" created.","attrs":{"verseId":"GEN.1.1"}}]}
	]`
	plain, _, _, _, err := decodeAPIBiblePassage(json.RawMessage(passage), "Genesis", 1)
	if err != nil {
		t.Fatal(err)
	}
	r := &recorder{}
	c := newAPIBibleStyleCensus("Genesis", r.logf)
	censused, _, _, _, err := decodeAPIBiblePassageChecked(json.RawMessage(passage), "Genesis", 1, c)
	if err != nil {
		t.Fatal(err)
	}

	if len(plain) == 0 {
		t.Fatal("the fixture decoded to nothing; this test proves nothing")
	}
	for ch, verses := range plain {
		if len(censused[ch]) != len(verses) {
			t.Fatalf("chapter %d: %d verses without the census, %d with it",
				ch, len(verses), len(censused[ch]))
		}
		for i, v := range verses {
			if censused[ch][i].Text != v.Text {
				t.Errorf("the census changed the text of %s %d:%d\n without: %q\n    with: %q",
					v.BookName, v.Chapter, v.Verse, v.Text, censused[ch][i].Text)
			}
		}
	}

	// And it must actually have seen the styles, or the comparison above is
	// comparing two runs of the same silent code.
	if c.styles()["p"] == 0 {
		t.Error("the paragraph census saw no p block; it is not wired")
	}
	if c.charStyles["nd"] == 0 {
		t.Error("the character census saw no nd span; it is not wired")
	}
}
