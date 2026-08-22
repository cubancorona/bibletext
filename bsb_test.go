package bibletext

import (
	"encoding/json"
	"testing"
)

// TestBSBVerseTextSpacing locks in the verse-text spacing rules that the
// real helloao data exercises. helloao trims the whitespace around every boundary
// it introduces (dropped footnote nodes and poetry clauses all abut with nothing
// between them), so contributing pieces are joined with one synthesized space —
// except explicit lineBreak nodes, which survive as authored newlines, and closing
// punctuation/quotes, which stay attached to the preceding text.
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
			"source line break between trimmed prose runs is retained (Genesis 10:2 shape)",
			`["The sons of Japheth:",{"lineBreak":true},"Gomer, Magog, Madai, Javan, Tubal, Meshech, and Tiras."]`,
			"The sons of Japheth:\nGomer, Magog, Madai, Javan, Tubal, Meshech, and Tiras.",
		},
		{
			// REAL capture (bible.helloao.org, BSB Job 6:6, 2026-08-04): the
			// final "?" is its own poem clause — it must ABUT the line, never
			// start one, and the footnote between them synthesizes no space.
			"clause then footnote then closing punctuation abuts (Job 6:6, real)",
			`[{"text":"Is tasteless food eaten without salt,","poem":1},{"text":"or is there flavor in the white of an egg","poem":2},{"noteId":8},{"text":"?","poem":2}]`,
			"Is tasteless food eaten without salt,\nor is there flavor in the white of an egg?",
		},
		{
			"clause then footnote then closing quote abuts (Genesis 3:15 shape)",
			`[{"text":"and you will strike his heel.","poem":2},{"noteId":14},{"text":"”","poem":2}]`,
			"and you will strike his heel.”",
		},
		{
			// Prose intro + explicit lineBreak + poem clauses: the lineBreak
			// already broke the line, so the first poem clause adds no second
			// break; the next clause starts its own line.
			"prose intro then poetry clauses (Genesis 2:23 shape)",
			`["And the man said:",{"lineBreak":true},{"text":"“This is now bone of my bones","poem":1},{"text":"and flesh of my flesh;","poem":2}]`,
			"And the man said:\n“This is now bone of my bones\nand flesh of my flesh;",
		},
		{
			// REAL capture (BSB Genesis 1:27): three poem clauses, NO lineBreak
			// nodes — each clause is one source line. (The previous fixture
			// fabricated a lineBreak node and space-joined the clauses; live
			// data disproves both.)
			"poem clauses are lines (Genesis 1:27, real)",
			`[{"text":"So God created man in His own image;","poem":1},{"text":"in the image of God He created him;","poem":2},{"text":"male and female He created them.","poem":2},{"noteId":4}]`,
			"So God created man in His own image;\nin the image of God He created him;\nmale and female He created them.",
		},
		{
			// REAL capture (BSB Psalm 23:1): a footnote between two poem lines
			// must not suppress the break.
			"footnote between poem lines keeps the break (Psalm 23:1, real)",
			`[{"text":"The LORD is my shepherd;","poem":1},{"noteId":1},{"text":"I shall not want.","poem":2}]`,
			"The LORD is my shepherd;\nI shall not want.",
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
            {"text":"male and female He created them.","poem":2},
            {"noteId":4},
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

	// Poetry (real BSB shape): each {text,poem} clause is one authored line —
	// live helloao poetry has NO lineBreak nodes — and the trailing {noteId}
	// contributes nothing.
	want27 := "So God created man in His own image;\nin the image of God He created him;\nmale and female He created them."
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

	// Listed right after WEB, ahead of the licensed ones. Both evaluation
	// versions are behind build tags now, so the NKJV is the licensed version a
	// default build has to sit before.
	order := map[string]int{}
	for i, ver := range bibleVersions() {
		order[ver.ID] = i
	}
	if !(order["web"] < order["bsb"] && order["bsb"] < order["nkjv"]) {
		t.Errorf("version order = web:%d bsb:%d nkjv:%d (want web < bsb < nkjv)", order["web"], order["bsb"], order["nkjv"])
	}
}
