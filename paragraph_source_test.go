package bibletext

// THE PUBLISHER'S PARAGRAPHS. Where an edition says a verse opens a
// paragraph, the app opens one there — before its own length rule is even
// consulted — and where the edition says nothing, the rule still applies.

import "testing"

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
	// Without the marks the same chapter is one paragraph, which is what the
	// app drew before.
	plain := paraShape(groupVersesIntoParagraphs(paraVerses(9, "A short verse.")))
	if !sameShape(plain, []int{1}) {
		t.Errorf("unmarked short verses should stay one paragraph, got %v", plain)
	}
}

// An edition that marks nothing still gets paragraphs, from the fallback.
func TestTheFallbackRuleStillAppliesWhereNothingIsMarked(t *testing.T) {
	long := "This verse is long enough on its own to carry the running paragraph past the threshold the fallback rule uses."
	got := paraShape(groupVersesIntoParagraphs(paraVerses(8, long)))
	if len(got) < 2 {
		t.Fatalf("an unmarked chapter of long verses must still break, got %v", got)
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
