package bibletext

// THE PUBLISHER'S PARAGRAPH TABLE. The World English editions' own
// paragraphing, recovered from their USFM because the runtime feed drops
// about 92% of it. These hold the table against the text it is meant for:
// every reference must name a verse that exists, or a paragraph opens
// nowhere and the reader silently loses a break.

import (
	"encoding/json"
	"os"
	"testing"
)

func webCaptureOrSkip(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("no local capture at %s", path)
	}
	return raw
}

// Every reference lands. The one hazard the generator guards is the
// publishers' zips carrying BOTH the Hebrew and the Greek Daniel and Esther,
// which the app maps to the same book names: a reference from Greek Daniel 3,
// where the Song of the Three occupies verses 24 to 90, would open paragraphs
// in the middle of a Daniel that has 30 verses there.
func TestEveryParagraphReferenceNamesARealVerse(t *testing.T) {
	raw := webCaptureOrSkip(t, "build/biblecache/web.json")
	bd, err := decodeWEB(raw)
	if err != nil {
		t.Fatal(err)
	}
	known := map[string]bool{}
	for _, b := range bd.Books {
		known[b] = true
	}
	missing, forwarded := 0, 0
	for key, verses := range webParagraphStarts {
		usfm, ch, ok := splitParagraphKey(key)
		if !ok {
			t.Errorf("unparseable key %q", key)
			continue
		}
		book := apiBibleBookName(usfm)
		if !known[book] {
			t.Errorf("%q names %q, which is not in this edition", key, book)
			continue
		}
		got := map[int]bool{}
		for _, v := range bd.Verses[book][ch] {
			got[v.Verse] = true
		}
		if len(got) == 0 {
			t.Errorf("%q names a chapter this edition does not have", key)
			continue
		}
		last := bd.Verses[book][ch][len(bd.Verses[book][ch])-1].Verse
		for _, v := range verses {
			if got[v] {
				continue
			}
			// A reference may name a verse the edition omits (Acts 8:37 and
			// kin); the applier moves the paragraph to the next verse it has.
			// A reference past the END of the chapter has nowhere to go and
			// is a real fault.
			forwarded++
			if v > last {
				missing++
				t.Errorf("%s %d:%d is past the end of the chapter", book, ch, v)
			}
		}
	}
	t.Logf("%d references moved to the next verse present (omitted verses)", forwarded)
	if missing > 0 {
		t.Errorf("%d references could not be placed at all", missing)
	}
	if forwarded > 12 {
		t.Errorf("%d references did not land on their own verse; the table and the text may have drifted", forwarded)
	}
}

// The shape readers actually see, on chapters whose paragraphing is well
// known: the days of creation, and the turns of the exchange with Nicodemus.
func TestTheWEBParagraphsWhereItsPublisherDoes(t *testing.T) {
	raw := webCaptureOrSkip(t, "build/biblecache/web.json")
	bd, err := decodeWEB(raw)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		book string
		ch   int
		want []int
	}{
		{"Genesis", 1, []int{1, 3, 6, 9, 14, 20, 24, 26, 31}},
		{"John", 3, []int{1, 3, 4, 5, 9, 10, 22, 27, 31}},
	} {
		var got []int
		for _, p := range groupVersesIntoParagraphs(bd.Verses[tc.book][tc.ch]) {
			got = append(got, p[0].Verse)
		}
		if len(got) != len(tc.want) {
			t.Errorf("%s %d opens %d paragraphs, want %d: %v", tc.book, tc.ch, len(got), len(tc.want), got)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("%s %d opens at %v, want %v", tc.book, tc.ch, got, tc.want)
				break
			}
		}
	}
}

// The Berean must not pick the table up: its own feed carries its
// paragraphing, and the two editions do not paragraph alike.
func TestTheTableIsNotAppliedToTheBerean(t *testing.T) {
	raw := webCaptureOrSkip(t, "build/biblecache/bsb.json")
	bd, err := decodeCanonical66(raw)
	if err != nil {
		t.Fatal(err)
	}
	var got []int
	for _, p := range groupVersesIntoParagraphs(bd.Verses["John"][3]) {
		got = append(got, p[0].Verse)
	}
	// The BSB's own feed opens John 3 in fifteen places, the WEB's publisher
	// in nine; if the table leaked across, the shapes would converge.
	if len(got) != 15 {
		t.Errorf("the Berean's John 3 opens %d paragraphs, want its feed's 15: %v", len(got), got)
	}
	_ = json.Marshal
}
