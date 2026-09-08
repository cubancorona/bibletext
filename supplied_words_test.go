package bibletext

// THE TRANSLATORS' SUPPLIED WORDS. The King James tradition sets in italics
// the words added for English sense that stand in no Hebrew or Greek word, and
// the New King James feed marks every one. The app flattened them all away.
// They are kept now as offsets, never as characters, so the verse a reader
// searches, shares, hears and links to is unchanged.

import (
	"encoding/json"
	"strings"
	"testing"
)

// A verse with a supplied word, a note and a poem break together: the three
// things that mark positions in a verse while it is still being assembled,
// which is where they could interfere with one another.
const suppliedFixture = `[
  {"name":"para","type":"tag","attrs":{"style":"q1"},"items":[
    {"name":"verse","type":"tag","attrs":{"style":"v","number":"30","sid":"MAT 11:30"},"items":[{"type":"text","text":"30"}]},
    {"type":"text","text":"For My yoke ","attrs":{"verseId":"MAT.11.30"}},
    {"name":"char","type":"tag","attrs":{"style":"it"},"items":[{"type":"text","text":"is","attrs":{"verseId":"MAT.11.30"}}]},
    {"type":"text","text":" easy and My burden ","attrs":{"verseId":"MAT.11.30"}},
    {"name":"char","type":"tag","attrs":{"style":"it"},"items":[{"type":"text","text":"is","attrs":{"verseId":"MAT.11.30"}}]},
    {"type":"text","text":" light.","attrs":{"verseId":"MAT.11.30"}}
  ]}
]`

func TestSuppliedWordsAreKeptAsOffsetsNotCharacters(t *testing.T) {
	vs, _, _, _, err := decodeAPIBibleChapter(json.RawMessage(suppliedFixture), "Matthew", 11)
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 1 {
		t.Fatalf("decoded %d verses, want 1", len(vs))
	}
	v := vs[0]

	// The text is exactly what it would be with nothing marked at all.
	if want := "For My yoke is easy and My burden is light."; v.Text != want {
		t.Errorf("text changed:\n got  %q\n want %q", v.Text, want)
	}
	if strings.ContainsRune(v.Text, suppliedOpen) || strings.ContainsRune(v.Text, suppliedClose) {
		t.Errorf("a bracket survived into the text: %q", v.Text)
	}

	if len(v.Supplied) != 2 {
		t.Fatalf("captured %d supplied spans, want 2: %+v", len(v.Supplied), v.Supplied)
	}
	r := []rune(v.Text)
	for i, sp := range v.Supplied {
		if sp.Start < 0 || sp.End > len(r) || sp.Start >= sp.End {
			t.Fatalf("span %d is outside the text: %+v", i, sp)
		}
		if got := string(r[sp.Start:sp.End]); got != "is" {
			t.Errorf("span %d marks %q, want the supplied \"is\"", i, got)
		}
	}
	// In order, and the second is the later one.
	if v.Supplied[0].Start >= v.Supplied[1].Start {
		t.Errorf("spans are out of order: %+v", v.Supplied)
	}
}

// A note and a supplied span in the same verse: each must land where it
// belongs once the other's marker has been taken out, which is why they are
// stripped in a single pass.
func TestASuppliedSpanAndANoteDoNotMoveEachOther(t *testing.T) {
	const both = `[
  {"name":"para","type":"tag","attrs":{"style":"p"},"items":[
    {"name":"verse","type":"tag","attrs":{"style":"v","number":"1","sid":"GEN 1:1"},"items":[{"type":"text","text":"1"}]},
    {"type":"text","text":"In the beginning ","attrs":{"verseId":"GEN.1.1"}},
    {"name":"char","type":"tag","attrs":{"style":"it"},"items":[{"type":"text","text":"was","attrs":{"verseId":"GEN.1.1"}}]},
    {"type":"text","text":" God","attrs":{"verseId":"GEN.1.1"}},
    {"name":"note","type":"tag","attrs":{"style":"f","caller":"+","verseId":"GEN.1.1"},"items":[
      {"name":"char","type":"tag","attrs":{"style":"ft"},"items":[{"type":"text","text":"Or Elohim"}]}
    ]},
    {"type":"text","text":" who made all.","attrs":{"verseId":"GEN.1.1"}}
  ]}
]`
	vs, _, _, _, err := decodeAPIBibleChapter(json.RawMessage(both), "Genesis", 1)
	if err != nil {
		t.Fatal(err)
	}
	v := vs[0]
	if want := "In the beginning was God who made all."; v.Text != want {
		t.Errorf("text:\n got  %q\n want %q", v.Text, want)
	}
	r := []rune(v.Text)
	if len(v.Supplied) != 1 {
		t.Fatalf("supplied spans: %+v", v.Supplied)
	}
	if got := string(r[v.Supplied[0].Start:v.Supplied[0].End]); got != "was" {
		t.Errorf("the supplied span marks %q, want \"was\"", got)
	}
	if len(v.Footnotes) != 1 {
		t.Fatalf("footnotes: %+v", v.Footnotes)
	}
	// The note sits after "God", which is rune 22 of the cleaned text.
	if want := len([]rune("In the beginning was God")); v.Footnotes[0].Anchor != want {
		t.Errorf("the note anchors at %d, want %d", v.Footnotes[0].Anchor, want)
	}
}

// The spans survive the cache, and a cache written before them decodes
// without them rather than failing.
func TestSuppliedSpansSurviveTheCache(t *testing.T) {
	in := &BibleData{Verses: map[string]map[int][]Verse{"Matthew": {11: {
		{BookName: "Matthew", Chapter: 11, Verse: 30, Text: "For My yoke is easy.",
			Supplied: []TextSpan{{Start: 12, End: 14}}},
	}}}}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out BibleData
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	got := out.Verses["Matthew"][11][0].Supplied
	if len(got) != 1 || got[0].Start != 12 || got[0].End != 14 {
		t.Errorf("round trip lost the span: %+v", got)
	}
}
