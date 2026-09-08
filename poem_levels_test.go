package bibletext

// THE INDENT DEPTH OF A POETRY LINE. Hebrew poetry is built of paired lines,
// and print sets the answering half indented under the opening one so the
// pairing can be seen. Every edition marks the depth; the app kept only the
// fact that a line was poetry at all, so every line drew flush left.

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// The couplet, as the publishers set it: an opening half at the first depth
// and its answer at the second.
func TestAPoemsLinesCarryTheirOwnDepth(t *testing.T) {
	const psalm = `[
  {"name":"para","type":"tag","attrs":{"style":"q1"},"items":[
    {"name":"verse","type":"tag","attrs":{"style":"v","number":"1","sid":"PSA 23:1"},"items":[{"type":"text","text":"1"}]},
    {"type":"text","text":"The LORD is my shepherd;","attrs":{"verseId":"PSA.23.1"}}
  ]},
  {"name":"para","type":"tag","attrs":{"style":"q2"},"items":[
    {"type":"text","text":"I shall not want.","attrs":{"verseId":"PSA.23.1"}}
  ]}
]`
	vs, _, _, _, err := decodeAPIBibleChapter(json.RawMessage(psalm), "Psalms", 23)
	if err != nil {
		t.Fatal(err)
	}
	v := vs[0]
	if want := "The LORD is my shepherd;\nI shall not want."; v.Text != want {
		t.Errorf("text:\n got  %q\n want %q", v.Text, want)
	}
	if len(v.PoemLevels) != 2 || v.PoemLevels[0] != 1 || v.PoemLevels[1] != 2 {
		t.Errorf("depths = %v, want [1 2]", v.PoemLevels)
	}
}

// Prose says nothing rather than saying zero: a verse with no poetry in it
// carries no depths at all.
func TestProseCarriesNoDepths(t *testing.T) {
	const prose = `[
  {"name":"para","type":"tag","attrs":{"style":"p"},"items":[
    {"name":"verse","type":"tag","attrs":{"style":"v","number":"1","sid":"JHN 1:1"},"items":[{"type":"text","text":"1"}]},
    {"type":"text","text":"In the beginning was the Word.","attrs":{"verseId":"JHN.1.1"}}
  ]}
]`
	vs, _, _, _, err := decodeAPIBibleChapter(json.RawMessage(prose), "John", 1)
	if err != nil {
		t.Fatal(err)
	}
	if got := vs[0].PoemLevels; got != nil {
		t.Errorf("prose carries depths %v", got)
	}
}

// Every edition, against the real captures: a depth list must describe the
// verse it belongs to, one entry per line, or it points at lines that are not
// there.
func TestEveryDepthListDescribesItsOwnVerse(t *testing.T) {
	for _, e := range []struct {
		label, path string
		decode      func([]byte) (*BibleData, error)
	}{
		{"BSB", "build/biblecache/bsb.json", decodeCanonical66},
		{"WEB", "build/biblecache/web.json", decodeWEB},
		{"WEB Catholic", "build/biblecache/webc.json", decodeHelloAOCatholic},
	} {
		raw, err := os.ReadFile(e.path)
		if err != nil {
			t.Skipf("no local capture at %s", e.path)
		}
		bd, err := e.decode(raw)
		if err != nil {
			t.Fatal(err)
		}
		withDepths, deepest := 0, 0
		for book, chs := range bd.Verses {
			for ch, vs := range chs {
				for _, v := range vs {
					if len(v.PoemLevels) == 0 {
						continue
					}
					withDepths++
					if lines := strings.Count(v.Text, "\n") + 1; len(v.PoemLevels) != lines {
						t.Errorf("%s %s %d:%d has %d depths for %d lines", e.label, book, ch, v.Verse, len(v.PoemLevels), lines)
					}
					for _, n := range v.PoemLevels {
						if n < 0 {
							t.Errorf("%s %s %d:%d has a negative depth", e.label, book, ch, v.Verse)
						}
						if n > deepest {
							deepest = n
						}
					}
				}
			}
		}
		if withDepths == 0 {
			t.Errorf("%s captured no depths at all", e.label)
		}
		t.Logf("%s: %d verses carry depths, deepest %d", e.label, withDepths, deepest)
	}
}

func TestDepthsSurviveTheCache(t *testing.T) {
	in := &BibleData{Verses: map[string]map[int][]Verse{"Psalms": {23: {
		{BookName: "Psalms", Chapter: 23, Verse: 1, Text: "a\nb", PoemLevels: []int{1, 2}},
	}}}}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out BibleData
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	got := out.Verses["Psalms"][23][0].PoemLevels
	if len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Errorf("round trip lost the depths: %v", got)
	}
}
