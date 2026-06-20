package bibletext

import (
	"encoding/json"
	"testing"
)

// TestBSBVerseTextSpacing locks in the verse-text flattening rules that the
// real helloao data exercises. helloao trims the whitespace around every boundary
// it introduces (dropped footnote/line-break nodes and poetry clauses all abut
// with nothing between them), so every contributing piece is joined with one
// synthesized space — EXCEPT where the next piece opens with closing punctuation
// or a quote, which must stay attached to the preceding text.
func TestBSBVerseTextSpacing(t *testing.T) {
	cases := []struct{ name, contentJSON, want string }{
		{
			"footnote before closing punctuation and quote (Mark 11:17 shape)",
			`["“My house will be called a house of prayer for all the nations’",{"noteId":1},"? But you have made it ‘a den of robbers.’",{"noteId":2},"”"]`,
			"“My house will be called a house of prayer for all the nations’? But you have made it ‘a den of robbers.’”",
		},
		{
			"footnote splitting a sentence keeps a single space (John 1:1 shape)",
			`["In the beginning was the Word,",{"noteId":0}," and the Word was with God."]`,
			"In the beginning was the Word, and the Word was with God.",
		},
		{
			// Real data trims the boundary: the runs are "...Eve," + {noteId} +
			// "because..." with NO baked space, so a space must be synthesized.
			"footnote between trimmed prose runs gets a space (Genesis 3:20 shape)",
			`["And Adam named his wife Eve,",{"noteId":16},"because she would be the mother of all the living."]`,
			"And Adam named his wife Eve, because she would be the mother of all the living.",
		},
		{
			"line break between trimmed prose runs gets a space (Genesis 10:2 shape)",
			`["The sons of Japheth:",{"lineBreak":true},"Gomer, Magog, Madai, Javan, Tubal, Meshech, and Tiras."]`,
			"The sons of Japheth: Gomer, Magog, Madai, Javan, Tubal, Meshech, and Tiras.",
		},
		{
			// A footnote between a poetry clause and a clause that is pure closing
			// punctuation: the "?" / "”" must abut, not get a synthesized space.
			"clause then footnote then closing punctuation abuts (Job 6:6 shape)",
			`[{"text":"or is there flavor in the white of an egg","poem":2},{"noteId":8},{"text":"?","poem":2}]`,
			"or is there flavor in the white of an egg?",
		},
		{
			"clause then footnote then closing quote abuts (Genesis 3:15 shape)",
			`[{"text":"and you will strike his heel.","poem":2},{"noteId":14},{"text":"”","poem":2}]`,
			"and you will strike his heel.”",
		},
		{
			"prose intro then poetry clauses (Genesis 2:23 shape)",
			`["And the man said:",{"lineBreak":true},{"text":"“This is now bone of my bones","poem":1},{"text":"and flesh of my flesh;","poem":2}]`,
			"And the man said: “This is now bone of my bones and flesh of my flesh;",
		},
		{
			"pure poetry clauses single-spaced (Genesis 1:27 shape)",
			`[{"text":"So God created man in His own image;","poem":1},{"text":"in the image of God He created him;","poem":2},{"lineBreak":true},{"text":"male and female He created them.","poem":2},{"noteId":4}]`,
			"So God created man in His own image; in the image of God He created him; male and female He created them.",
		},
	}
	for _, c := range cases {
		var content []json.RawMessage
		if err := json.Unmarshal([]byte(c.contentJSON), &content); err != nil {
			t.Fatalf("%s: bad fixture: %v", c.name, err)
		}
		if got := bsbVerseText(content); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
}

// bsbSampleComplete mirrors bible.helloao.org's complete.json shape with two
// books at non-adjacent canonical orders (Genesis=1, John=43) and every node
// kind the decoder must handle: headings / line_break / hebrew_subtitle (skipped),
// plain-string verses, poetry ({text,poem}) split across {lineBreak}, and inline
// footnote markers ({noteId}).
const bsbSampleComplete = `{
  "translation": {"id":"BSB","shortName":"BSB"},
  "books": [
    {
      "id":"GEN","order":1,"name":"Genesis",
      "chapters":[
        {"chapter":{"number":1,"content":[
          {"type":"heading","content":["The Creation"]},
          {"type":"verse","number":1,"content":["In the beginning God created the heavens and the earth."]},
          {"type":"line_break"},
          {"type":"verse","number":27,"content":[
            {"text":"So God created man in His own image;","poem":1},
            {"text":"in the image of God He created him;","poem":2},
            {"lineBreak":true},
            {"text":"male and female He created them.","poem":2},
            {"noteId":4}
          ]}
        ]}}
      ]
    },
    {
      "id":"JHN","order":43,"name":"John",
      "chapters":[
        {"chapter":{"number":1,"content":[
          {"type":"hebrew_subtitle","content":["a subtitle the reader view drops"]},
          {"type":"verse","number":1,"content":["In the beginning was the Word,",{"noteId":0}," and the Word was with God."]}
        ]}}
      ]
    }
  ]
}`

func verseText(verses []Verse, n int) (string, bool) {
	for _, v := range verses {
		if v.Verse == n {
			return v.Text, true
		}
	}
	return "", false
}

func TestDecodeBSBComplete(t *testing.T) {
	appBooks := NewBibleData().Books
	bd, err := decodeBSBComplete([]byte(bsbSampleComplete), appBooks)
	if err != nil {
		t.Fatalf("decodeBSBComplete: %v", err)
	}

	// helloao's `order` (1, 43) must map to the app's own canonical book names,
	// so the decoded data slots into the shared 66-book structure.
	gen := bd.Verses["Genesis"]
	if gen == nil {
		t.Fatal("order 1 must map to Genesis")
	}
	if bd.Verses["John"] == nil {
		t.Fatal("order 43 must map to John")
	}
	if got := gen[1][0].BookName; got != "Genesis" {
		t.Errorf("verse BookName = %q, want Genesis (app name, not USFM code)", got)
	}

	// Plain-string verse passes through verbatim.
	if got, ok := verseText(gen[1], 1); !ok || got != "In the beginning God created the heavens and the earth." {
		t.Errorf("Genesis 1:1 = %q (ok=%v)", got, ok)
	}

	// Poetry: the {text} clauses join with single spaces; {lineBreak} and the
	// trailing {noteId} contribute nothing.
	want27 := "So God created man in His own image; in the image of God He created him; male and female He created them."
	if got, ok := verseText(gen[1], 27); !ok || got != want27 {
		t.Errorf("Genesis 1:27 = %q\n           want %q", got, want27)
	}

	// Non-verse nodes (hebrew_subtitle) are skipped, and an inline {noteId}
	// between two text runs collapses cleanly to a single space.
	want := "In the beginning was the Word, and the Word was with God."
	if got, ok := verseText(bd.Verses["John"][1], 1); !ok || got != want {
		t.Errorf("John 1:1 = %q\n      want %q", got, want)
	}
}

func TestDecodeBSBRejectsEmpty(t *testing.T) {
	if _, err := decodeBSBComplete([]byte(`{"books":[]}`), NewBibleData().Books); err == nil {
		t.Error("expected an error for a response with no books")
	}
	if _, err := decodeBSBComplete([]byte(`not json`), NewBibleData().Books); err == nil {
		t.Error("expected an error for invalid JSON")
	}
}

// TestBSBRegisteredAsPublicDomain guards that BSB ships as a real, selectable
// public-domain version (not a licensed/evaluation one), and is offered up front.
func TestBSBRegisteredAsPublicDomain(t *testing.T) {
	t.Setenv("BIBLETEXT_ENABLE_TESTING", "") // ensure the default (non-QA) build

	v, ok := versionByID("bsb")
	if !ok {
		t.Fatal("BSB is not registered")
	}
	if !v.PublicDomain {
		t.Error("BSB must be marked PublicDomain")
	}
	if v.isTesting() {
		t.Error("BSB must serve real text (its source is always available), not a placeholder")
	}
	if !v.canSelect() {
		t.Error("BSB (public domain) must be user-selectable")
	}
	if v.Abbrev != "BSB" || v.Name != "Berean Standard Bible" {
		t.Errorf("BSB metadata = %q / %q", v.Name, v.Abbrev)
	}

	// Listed right after WEB, before the licensed evaluation versions.
	order := map[string]int{}
	for i, ver := range bibleVersions() {
		order[ver.ID] = i
	}
	if !(order["web"] < order["bsb"] && order["bsb"] < order["nrsv"]) {
		t.Errorf("version order = web:%d bsb:%d nrsv:%d (want web < bsb < nrsv)", order["web"], order["bsb"], order["nrsv"])
	}
}
