package bibletext

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// S2 of docs/SCRIPTURE_WORKLIST.md. The World English editions put Psalm 119's
// acrostic letters INSIDE the verse text: ALEPH as the psalm's subtitle, and
// the other twenty-one appended to the last verse of each stanza, so Psalm
// 119:8 read "Don't utterly forsake me. BETH". They reached search, sharing,
// copying, links, the website and speech — everything that reads Verse.Text.
//
// The letters are not gone; they are drawn as stanza headings, which is what
// they are and what the NKJV decoder has always done with the equivalent (qa).

// The twenty-two, so a test can say what it is looking for.
var hebrewAcrosticLetters = []string{
	"ALEPH", "BETH", "GIMEL", "DALETH", "HE", "VAV", "ZAYIN", "HETH", "TETH",
	"YOD", "KAPH", "LAMED", "MEM", "NUN", "SAMEKH", "AYIN", "PE", "TSADHE",
	"QOPH", "RESH", "SIN", "TAV",
}

func TestATrailingDescriptiveRunLeavesTheVerseAndHeadsTheNext(t *testing.T) {
	book := helloAOBook{ID: "PSA", Order: 19}
	book.Chapters = []helloAOChapterEntry{{}}
	book.Chapters[0].Chapter.Number = 119
	for _, raw := range []string{
		`{"type":"verse","number":8,"content":[{"text":"Don’t utterly forsake me."},{"text":"BETH","descriptive":true}]}`,
		`{"type":"verse","number":9,"content":[{"text":"How can a young man keep his way pure?"}]}`,
	} {
		book.Chapters[0].Chapter.Content = append(book.Chapters[0].Chapter.Content, json.RawMessage(raw))
	}
	chapters, _, _, headings := decodeHelloAOChapters("Psalms", book, nil)

	v8 := chapters[119][0]
	if strings.Contains(v8.Text, "BETH") {
		t.Errorf("the acrostic letter is still in the verse: %q", v8.Text)
	}
	if !strings.Contains(v8.Text, "forsake me") {
		t.Errorf("the verse's own words were lost with the letter: %q", v8.Text)
	}

	var got *Heading
	for i := range headings[119] {
		if headings[119][i].Text == "BETH" {
			got = &headings[119][i]
		}
	}
	if got == nil {
		t.Fatalf("BETH was removed from the verse and not kept as a heading — it must be "+
			"drawn, not dropped. headings: %+v", headings[119])
	}
	if got.BeforeVerse != 9 {
		t.Errorf("BETH heads verse %d, want 9: a stanza letter labels what FOLLOWS it",
			got.BeforeVerse)
	}
}

// CONTROL, and the reason the rule is about POSITION rather than about words:
// the Berean's one descriptive run opens Zechariah 12:1 and is a title for the
// verse it stands in, not a label for the next. It must not move.
func TestALeadingDescriptiveRunStaysInItsVerse(t *testing.T) {
	book := helloAOBook{ID: "ZEC", Order: 38}
	book.Chapters = []helloAOChapterEntry{{}}
	book.Chapters[0].Chapter.Number = 12
	book.Chapters[0].Chapter.Content = []json.RawMessage{
		json.RawMessage(`{"type":"verse","number":1,"content":[` +
			`{"text":"This is the burden of the word of the LORD concerning Israel.","descriptive":true},` +
			`{"lineBreak":true},` +
			`{"text":"Thus declares the LORD, who stretches out the heavens."}]}`),
	}
	chapters, _, _, headings := decodeHelloAOChapters("Zechariah", book, nil)

	v1 := chapters[12][0]
	if !strings.Contains(v1.Text, "burden of the word") {
		t.Errorf("an oracle title was taken out of the verse it opens: %q", v1.Text)
	}
	for _, h := range headings[12] {
		if strings.Contains(h.Text, "burden of the word") {
			t.Errorf("an oracle title became a heading: %q", h.Text)
		}
	}
}

func TestAnAcrosticSubtitleIsAHeadingNotAPsalmTitle(t *testing.T) {
	book := helloAOBook{ID: "PSA", Order: 19}
	book.Chapters = []helloAOChapterEntry{{}}
	book.Chapters[0].Chapter.Number = 119
	book.Chapters[0].Chapter.Content = []json.RawMessage{
		json.RawMessage(`{"type":"hebrew_subtitle","content":["ALEPH"]}`),
		json.RawMessage(`{"type":"verse","number":1,"content":[{"text":"Blessed are those whose ways are blameless."}]}`),
	}
	_, _, supers, headings := decodeHelloAOChapters("Psalms", book, nil)

	if _, ok := supers[119]; ok {
		t.Errorf("ALEPH was kept as the psalm's title: %+v", supers[119])
	}
	found := false
	for _, h := range headings[119] {
		if h.Text == "ALEPH" && h.BeforeVerse == 1 {
			found = true
		}
	}
	if !found {
		t.Errorf("ALEPH is neither a title nor a heading — it was dropped. headings: %+v",
			headings[119])
	}
}

// A real title must be untouched, or every psalm loses its superscription.
func TestARealPsalmTitleIsStillATitle(t *testing.T) {
	book := helloAOBook{ID: "PSA", Order: 19}
	book.Chapters = []helloAOChapterEntry{{}}
	book.Chapters[0].Chapter.Number = 3
	book.Chapters[0].Chapter.Content = []json.RawMessage{
		json.RawMessage(`{"type":"hebrew_subtitle","content":["A Psalm by David, when he fled from Absalom his son."]}`),
		json.RawMessage(`{"type":"verse","number":1,"content":[{"text":"Yahweh, how my adversaries have increased!"}]}`),
	}
	_, _, supers, _ := decodeHelloAOChapters("Psalms", book, nil)
	if !strings.Contains(supers[3].Text, "Absalom") {
		t.Errorf("a genuine psalm title was mistaken for an acrostic letter: %+v", supers[3])
	}
}

func TestTheAcrosticLetterRuleRejectsOrdinaryTitles(t *testing.T) {
	for _, s := range hebrewAcrosticLetters {
		if !acrosticLetterLabel(s) {
			t.Errorf("%q is an acrostic letter and was not recognised", s)
		}
	}
	for _, s := range []string{
		"A Psalm by David.", "For the Chief Musician.", "", "  ",
		"A PSALM OF DAVID", // spaces: a sentence, however it is cased
		"Maskil", "SELAH.", // punctuation is not a bare letter name
	} {
		if acrosticLetterLabel(s) {
			t.Errorf("%q was mistaken for an acrostic letter", s)
		}
	}
}

// Against the real feeds: after the fix, no verse of Psalm 119 may end with an
// acrostic letter, and all twenty-two must be present as headings.
func TestPsalm119AcrosticsLeftTheTextInEveryEdition(t *testing.T) {
	for _, tc := range []struct {
		path   string
		decode func([]byte) (*BibleData, error)
	}{
		{"build/biblecache/web.json", decodeWEB},
		{"build/biblecache/webc.json", decodeHelloAOCatholic},
	} {
		body, err := os.ReadFile(tc.path)
		if err != nil {
			t.Skipf("%s not present (build/ is gitignored); skipping", tc.path)
		}
		bd, err := tc.decode(body)
		if err != nil {
			t.Fatalf("%s: %v", tc.path, err)
		}
		verses := bd.Verses["Psalms"][119]
		if len(verses) < 176 {
			t.Fatalf("%s: Psalm 119 decoded to %d verses; this test proves nothing",
				tc.path, len(verses))
		}
		for _, v := range verses {
			for _, letter := range hebrewAcrosticLetters {
				if strings.HasSuffix(strings.TrimSpace(v.Text), letter) {
					t.Errorf("%s: Psalm 119:%d still ends with %q: %q",
						tc.path, v.Verse, letter, v.Text)
				}
			}
		}
		if _, ok := bd.Superscriptions["Psalms"][119]; ok {
			t.Errorf("%s: Psalm 119 still carries a title", tc.path)
		}
		heads := bd.Headings["Psalms"][119]
		if len(heads) != 22 {
			t.Errorf("%s: Psalm 119 has %d headings, want 22 — one per acrostic letter",
				tc.path, len(heads))
		}
	}
}

// The other editions must be unchanged by all of this. The Berean has one
// descriptive run and it is not an acrostic letter.
func TestTheBereanKeepsItsOracleTitleInTheVerse(t *testing.T) {
	body, err := os.ReadFile("build/biblecache/bsb.json")
	if err != nil {
		t.Skip("build/biblecache/bsb.json not present; skipping")
	}
	bd, err := decodeCanonical66(body)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, v := range bd.Verses["Zechariah"][12] {
		if v.Verse == 1 {
			found = true
			if !strings.Contains(v.Text, "burden of the word") {
				t.Errorf("Zechariah 12:1 lost its oracle title: %q", v.Text)
			}
		}
	}
	if !found {
		t.Fatal("Zechariah 12:1 not decoded; this test proves nothing")
	}
	if n := len(bd.Superscriptions["Psalms"]); n != 116 {
		t.Errorf("the Berean has %d psalm titles, want 116 — unchanged by the acrostic rule", n)
	}
}
