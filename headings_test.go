package bibletext

// THE PUBLISHERS' HEADINGS ARE KEPT. Every edition sets them, the app used to
// drop every one, and they are the translators' own map of what a chapter is
// about. Captured here whether or not any surface draws them yet, because a
// thing a publisher sent is kept — and kept OUT of Verse.Text, because a
// heading is editorial matter rather than Scripture.

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func headingCapture(t *testing.T, path string, decode func([]byte) (*BibleData, error)) *BibleData {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("no local capture at %s", path)
	}
	bd, err := decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	return bd
}

func TestTheBereanKeepsEveryHeadingItsPublisherSets(t *testing.T) {
	bd := headingCapture(t, "build/biblecache/bsb.json", decodeCanonical66)
	n := 0
	for _, chs := range bd.Headings {
		for _, hs := range chs {
			for _, h := range hs {
				n++
				if h.Text == "" {
					t.Error("an empty heading was captured")
				}
				if h.BeforeVerse == 0 {
					t.Errorf("%q names no verse", h.Text)
				}
			}
		}
	}
	if n != 3091 {
		t.Errorf("captured %d headings, want the edition's 3,091", n)
	}
	// The shape a reader would recognise: Genesis 1 by the days of creation.
	got := bd.Headings["Genesis"][1]
	if len(got) < 2 || got[0].Text != "The Creation" || got[0].BeforeVerse != 1 {
		t.Errorf("Genesis 1 opens with %+v", got)
	}
}

// A heading is not Scripture and must never reach the text, or it would enter
// search, sharing, speech and links as though a translator had written it. A
// corpus-wide word search cannot show this — "Joseph" heads Genesis 30 and the
// chapter names him, and 1 Chronicles 23:7 opens with the very words of its own
// heading because it is a genealogy — so the fixture makes the heading text
// impossible to confuse with the verse it precedes.
func TestAHeadingNeverEntersTheText(t *testing.T) {
	book := helloAOBook{ID: "GEN", Order: 1}
	book.Chapters = []helloAOChapterEntry{{}}
	book.Chapters[0].Chapter.Number = 1
	for _, raw := range []string{
		`{"type":"heading","content":["ZZ A HEADING NO VERSE COULD CONTAIN ZZ"]}`,
		`{"type":"verse","number":1,"content":["In the beginning God created the heavens and the earth."]}`,
		`{"type":"heading","content":["ZZ A SECOND ONE ZZ"]}`,
		`{"type":"verse","number":2,"content":["Now the earth was formless and void."]}`,
	} {
		book.Chapters[0].Chapter.Content = append(book.Chapters[0].Chapter.Content, json.RawMessage(raw))
	}

	chapters, _, _, headings := decodeHelloAOChapters("Genesis", book, nil)
	for _, v := range chapters[1] {
		if strings.Contains(v.Text, "ZZ") {
			t.Errorf("verse %d carries heading text: %q", v.Verse, v.Text)
		}
	}
	if got := chapters[1][0].Text; got != "In the beginning God created the heavens and the earth." {
		t.Errorf("verse 1 = %q", got)
	}
	if len(headings[1]) != 2 {
		t.Fatalf("captured %d headings, want 2", len(headings[1]))
	}
	for i, want := range []int{1, 2} {
		if headings[1][i].BeforeVerse != want {
			t.Errorf("heading %d stands before verse %d, want %d", i, headings[1][i].BeforeVerse, want)
		}
	}
}

// The World English feed sets none, which is a fact about that edition rather
// than a failure to read them: its own published markup carries only five.
func TestTheWorldEnglishFeedSetsNoHeadings(t *testing.T) {
	bd := headingCapture(t, "build/biblecache/web.json", decodeWEB)
	for book, chs := range bd.Headings {
		for ch, hs := range chs {
			t.Errorf("%s %d unexpectedly carries %d headings", book, ch, len(hs))
		}
	}
}

// The Catholic edition's five are the names by which those passages are known,
// and they include one malformed block: a title fused to a translator's note
// with no space between them. It is captured as the publisher sent it — this
// pins the shape so a decision about drawing it can be taken on the real data.
func TestTheCatholicEditionKeepsItsFiveHeadings(t *testing.T) {
	bd := headingCapture(t, "build/biblecache/webc.json", decodeHelloAOCatholic)
	n := 0
	for _, chs := range bd.Headings {
		for _, hs := range chs {
			n += len(hs)
		}
	}
	if n != 5 {
		t.Errorf("captured %d headings, want 5", n)
	}
	if got := bd.Headings["Daniel"][13]; len(got) != 1 || got[0].Text != "THE HISTORY OF SUSANNA" {
		t.Errorf("Daniel 13 heading = %+v", got)
	}
}

// Headings survive the cache, and a cache written before they existed decodes
// without them rather than failing.
func TestHeadingsSurviveTheCache(t *testing.T) {
	in := &BibleData{
		Books:  []string{"Matthew"},
		Verses: map[string]map[int][]Verse{"Matthew": {5: {{BookName: "Matthew", Chapter: 5, Verse: 1, Text: "a"}}}},
		Headings: map[string]map[int][]Heading{"Matthew": {5: {
			{Text: "The Beatitudes", Style: "s", BeforeVerse: 1},
		}}},
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out BibleData
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	got := out.Headings["Matthew"][5]
	if len(got) != 1 || got[0].Text != "The Beatitudes" || got[0].Style != "s" || got[0].BeforeVerse != 1 {
		t.Errorf("round trip lost the heading: %+v", got)
	}
	var old BibleData
	if err := json.Unmarshal([]byte(`{"Books":["Matthew"],"Verses":{}}`), &old); err != nil {
		t.Fatal(err)
	}
	if old.Headings != nil {
		t.Error("a cache written before headings existed must decode without them")
	}
}
