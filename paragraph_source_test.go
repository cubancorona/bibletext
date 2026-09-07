package bibletext

// THE PUBLISHER'S PARAGRAPHS. Where an edition says a verse opens a
// paragraph, the app opens one there — before its own length rule is even
// consulted — and where the edition says nothing, the rule still applies.

import (
	"encoding/json"
	"testing"
)

func paraVerses(n int, text string) []Verse {
	out := make([]Verse, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, Verse{BookName: "John", Chapter: 3, Verse: i, Text: text})
	}
	return out
}

func paraShape(paras [][]Verse) []int {
	out := make([]int, 0, len(paras))
	for _, p := range paras {
		out = append(out, p[0].Verse)
	}
	return out
}

func sameShape(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Short verses that the length rule would never break: the publisher's marks
// are the only reason a paragraph ends, which is the case the app got wrong.
func TestAPublisherBreakOpensAParagraphTheRuleWouldNotFind(t *testing.T) {
	verses := paraVerses(9, "A short verse.")
	// The shape all three editions give the opening of the Nicodemus
	// dialogue: 1-2, 3, 4, 5-8, 9.
	for _, v := range []int{3, 4, 5, 9} {
		verses[v-1].ParaStart = true
	}
	got := paraShape(groupVersesIntoParagraphs(verses))
	want := []int{1, 3, 4, 5, 9}
	if !sameShape(got, want) {
		t.Errorf("paragraphs open at %v, want %v", got, want)
	}
	// Without the marks the same chapter is one paragraph.
	plain := paraShape(groupVersesIntoParagraphs(paraVerses(9, "A short verse.")))
	if !sameShape(plain, []int{1}) {
		t.Errorf("unmarked short verses should stay one paragraph, got %v", plain)
	}
}

// An edition that marks nothing is ONE paragraph, which is what a poem with no
// stanza break is in print. The app used to chop such a chapter up by
// character count; it no longer invents anything.
func TestAnUnmarkedChapterIsOneParagraph(t *testing.T) {
	long := "This verse is long enough that the old character rule would have broken the chapter several times over."
	got := paraShape(groupVersesIntoParagraphs(paraVerses(8, long)))
	if !sameShape(got, []int{1}) {
		t.Errorf("an unmarked chapter opens at %v, want one paragraph at [1]", got)
	}
}

// A marked verse breaks even mid-sentence, where the fallback never would,
// and a marked verse in a long chapter does not wait for the length floor.
func TestAPublisherBreakDoesNotWaitForTheLengthFloor(t *testing.T) {
	verses := paraVerses(4, "An unfinished clause that ends with no stop at all")
	verses[1].ParaStart = true
	got := paraShape(groupVersesIntoParagraphs(verses))
	if !sameShape(got, []int{1, 2}) {
		t.Errorf("paragraphs open at %v, want [1 2]", got)
	}
}

// Every verse survives, in order, however the two signals combine.
func TestParagraphSourcesNeverLoseAVerse(t *testing.T) {
	long := "This verse is long enough on its own to carry the running paragraph past the threshold the fallback rule uses."
	verses := paraVerses(12, long)
	for _, v := range []int{2, 3, 7, 11} {
		verses[v-1].ParaStart = true
	}
	seen, expect := 0, 1
	for _, para := range groupVersesIntoParagraphs(verses) {
		if len(para) == 0 {
			t.Fatal("an empty paragraph was emitted")
		}
		for _, v := range para {
			if v.Verse != expect {
				t.Fatalf("verse order broken: got %d want %d", v.Verse, expect)
			}
			expect++
			seen++
		}
	}
	if seen != len(verses) {
		t.Errorf("kept %d verses of %d", seen, len(verses))
	}
}

// THE FEED'S OWN BREAKS. A chapter-level line_break in the helloao feeds is
// the publisher's paragraph boundary; the decoder must put it on the verse
// that follows, and a break with no verse after it must not strand a flag.
func TestHelloAODecoderCarriesTheFeedsParagraphBreaks(t *testing.T) {
	book := helloAOBook{ID: "JHN", Order: 43}
	book.Chapters = []struct {
		Chapter struct {
			Number    int               `json:"number"`
			Content   []json.RawMessage `json:"content"`
			Footnotes []struct {
				NoteID    int    `json:"noteId"`
				Caller    string `json:"caller"`
				Text      string `json:"text"`
				Reference struct {
					Chapter int `json:"chapter"`
					Verse   int `json:"verse"`
				} `json:"reference"`
			} `json:"footnotes"`
		} `json:"chapter"`
	}{{}}
	book.Chapters[0].Chapter.Number = 3
	for _, raw := range []string{
		`{"type":"heading","content":["A heading, which is not a break"]}`,
		`{"type":"verse","number":1,"content":["There was a man of the Pharisees."]}`,
		`{"type":"verse","number":2,"content":["This man came to Jesus by night."]}`,
		`{"type":"line_break"}`,
		`{"type":"verse","number":3,"content":["Jesus answered him."]}`,
		`{"type":"line_break"}`,
		`{"type":"line_break"}`,
		`{"type":"verse","number":4,"content":["Nicodemus said to him."]}`,
		`{"type":"line_break"}`,
	} {
		book.Chapters[0].Chapter.Content = append(book.Chapters[0].Chapter.Content, json.RawMessage(raw))
	}

	chapters, _, _ := decodeHelloAOChapters("John", book)
	verses := chapters[3]
	if len(verses) != 4 {
		t.Fatalf("decoded %d verses, want 4", len(verses))
	}
	for _, tc := range []struct {
		verse int
		want  bool
		why   string
	}{
		{1, false, "the first verse carries no mark; the chapter opens a paragraph anyway"},
		{2, false, "no break precedes it"},
		{3, true, "a break precedes it"},
		{4, true, "two breaks in a row still open one paragraph"},
	} {
		if got := verses[tc.verse-1].ParaStart; got != tc.want {
			t.Errorf("verse %d ParaStart = %v, want %v (%s)", tc.verse, got, tc.want, tc.why)
		}
	}
	// The trailing break has no verse to open and must simply be dropped.
	shape := paraShape(groupVersesIntoParagraphs(verses))
	if !sameShape(shape, []int{1, 3, 4}) {
		t.Errorf("paragraphs open at %v, want [1 3 4]", shape)
	}
}

// A chapter the publisher paragraphed is theirs entirely: the app's length
// rule adds nothing inside their paragraphs. Before this, the New King James
// Genesis 1 gained five breaks the publisher did not set, in the middle of
// paragraphs they had deliberately kept whole.
func TestThePublishersParagraphingIsNotSupplemented(t *testing.T) {
	long := "This verse is long enough on its own to carry the running paragraph well past the threshold the fallback rule would otherwise use to break it."
	verses := paraVerses(9, long)
	verses[4].ParaStart = true // one publisher break, at verse 5

	got := paraShape(groupVersesIntoParagraphs(verses))
	if !sameShape(got, []int{1, 5}) {
		t.Errorf("paragraphs open at %v, want [1 5]: the publisher set one break and the rule must not add more", got)
	}

}
