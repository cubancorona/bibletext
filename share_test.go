package bibletext

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func shareTestBible() *BibleData {
	bd := NewBibleData()
	bd.Books = []string{"John"}
	bd.Verses["John"] = map[int][]Verse{
		3: {
			{BookName: "John", Chapter: 3, Verse: 16, Text: "For God so loved the world, that he gave his one and only Son."},
			{BookName: "John", Chapter: 3, Verse: 17, Text: "For God didn't send his Son into the world to judge the world."},
		},
	}
	return bd
}

func TestCitationForSelection(t *testing.T) {
	state := &AppState{Bible: shareTestBible(), CurrentBook: "John", CurrentChapter: 3}

	cases := []struct {
		name string
		sel  string
		want string
	}{
		{"single", "For God so loved the world, that he gave his one and only Son.", "John 3:16"},
		{"span", "For God so loved the world, that he gave his one and only Son. For God didn't send his Son into the world to judge the world.", "John 3:16–17"},
		{"unmatched", "a phrase not present anywhere here", "John 3"},
	}
	for _, c := range cases {
		if got := citationForSelection(state, c.sel); got != c.want {
			t.Errorf("%s: citationForSelection = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestRenderVerseImage(t *testing.T) {
	path, err := renderVerseImage(&AppState{}, "For God so loved the world, that he gave his one and only Son.", "John 3:16", "WEB", 0)
	if err != nil {
		t.Fatalf("renderVerseImage: %v", err)
	}
	defer os.Remove(path)
	if !strings.HasSuffix(path, ".png") {
		t.Errorf("expected a .png path, got %q", path)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() < 1000 {
		t.Errorf("image suspiciously small: %d bytes", info.Size())
	}
}

func TestRenderVerseImageLongPassage(t *testing.T) {
	long := strings.Repeat("For God so loved the world that he gave his one and only Son. ", 12)
	path, err := renderVerseImage(&AppState{}, long, "John 3:16-18", "WEB", 0)
	if err != nil {
		t.Fatalf("long render: %v", err)
	}
	defer os.Remove(path)
	// Same sanity floor as the short-passage test: a degenerate/blank render
	// for the long-wrap path must not pass just because err was nil.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() < 1000 {
		t.Errorf("long-passage image suspiciously small: %d bytes", info.Size())
	}
}

func TestFormatBibleQuote(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{
			"plain verse gets outer quotes",
			"For God so loved the world, that he gave his one and only Son.",
			"“For God so loved the world, that he gave his one and only Son.”",
		},
		{
			"dialogue nests to single marks inside an outer pair (Rule 5.1(b))",
			"Jesus said to him, “I am the way, the truth, and the life.”",
			"“Jesus said to him, ‘I am the way, the truth, and the life.’”",
		},
		{
			"verse that is itself a quotation: balance the close, then nest + wrap",
			"“Blessed are the poor in spirit, for theirs is the Kingdom of Heaven.",
			"“Blessed are the poor in spirit, for theirs is the Kingdom of Heaven.”",
		},
		{
			"verse ending a quotation: balance the open, then nest + wrap",
			"why have you forsaken me?”",
			"“[W]hy have you forsaken me?”",
		},
		{
			"John 18:38 fragment: internal quotations nest to single inside the outer double",
			"“What is truth?” Pilate asked. And having said this, he went out again to the Jews and told them, “I find no basis for a charge against Him.",
			"“‘What is truth?’ Pilate asked. And having said this, he went out again to the Jews and told them, ‘I find no basis for a charge against Him.’”",
		},
		{
			"two-level nesting: the outer double drops to single (the rare 2nd level stays single)",
			"But he answered, “It is written, ‘Man shall not live by bread alone.’”",
			"“But he answered, ‘It is written, ‘Man shall not live by bread alone.’’”",
		},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		if got := formatBibleQuote(tc.in); got != tc.want {
			t.Errorf("%s:\n got %q\nwant %q", tc.name, got, tc.want)
		}
	}
}

func TestFormatBibleQuoteBlockThreshold(t *testing.T) {
	// Bluebook Rule 5: 50+ words is a block quotation — no surrounding marks.
	long49 := strings.Repeat("word ", 48) + "word." // 49 words, ends on a period
	if got := formatBibleQuote(long49); got != "“[W]"+long49[1:]+"”" {
		t.Errorf("49 words should be an inline (quoted) passage, got unquoted: %q", got[:20])
	}
	long50 := strings.Repeat("word ", 49) + "word." // 50 words
	if got := formatBibleQuote(long50); got != "[W]"+long50[1:] {
		t.Errorf("50 words should be a block quote (no outer marks):\n got %q", got[:20])
	}
}

func TestCleanQuoteTextStripsVerseNumbers(t *testing.T) {
	bd := NewBibleData()
	bd.Books = []string{"John"}
	bd.Verses["John"] = map[int][]Verse{3: {
		{BookName: "John", Chapter: 3, Verse: 16, Text: "\nFor God so loved the world, that he gave his one and only Son,\n"},
		{BookName: "John", Chapter: 3, Verse: 17, Text: "\nFor God didn’t send his Son into the world to judge the world,\n"},
	}}
	state := &AppState{Bible: bd, CurrentBook: "John", CurrentChapter: 3}

	raw := "16 For God so loved the world, that he gave his one and only Son, 17 For God didn’t send his Son into the world to judge the world,"
	want := "For God so loved the world, that he gave his one and only Son, For God didn’t send his Son into the world to judge the world,"
	if got := cleanQuoteText(state, raw); got != want {
		t.Errorf("\n got %q\nwant %q", got, want)
	}
}

func TestCleanQuoteTextKeepsNumbersInsideText(t *testing.T) {
	bd := NewBibleData()
	bd.Books = []string{"Revelation"}
	bd.Verses["Revelation"] = map[int][]Verse{7: {
		{BookName: "Revelation", Chapter: 7, Verse: 4, Text: "I heard the number of those who were sealed, 144,000,"},
	}}
	state := &AppState{Bible: bd, CurrentBook: "Revelation", CurrentChapter: 7}

	// The leading "4" is the verse number and must go; "144,000" is real text, stays.
	raw := "4 I heard the number of those who were sealed, 144,000,"
	want := "I heard the number of those who were sealed, 144,000,"
	if got := cleanQuoteText(state, raw); got != want {
		t.Errorf("\n got %q\nwant %q", got, want)
	}
}

func TestShareQuotePipelineBeatitude(t *testing.T) {
	bd := NewBibleData()
	bd.Books = []string{"Matthew"}
	bd.Verses["Matthew"] = map[int][]Verse{5: {
		{BookName: "Matthew", Chapter: 5, Verse: 3, Text: "\n“Blessed are the poor in spirit,\nfor theirs is the Kingdom of Heaven.\n"},
	}}
	state := &AppState{Bible: bd, CurrentBook: "Matthew", CurrentChapter: 5}

	// The selection includes the verse number and the opening quote; the number is
	// stripped, the quotation is balanced (its closer lies past this verse), then the
	// verse's own marks nest to single inside the outer pair (Rule 5.1(b)).
	raw := "3 “Blessed are the poor in spirit, for theirs is the Kingdom of Heaven."
	want := "“Blessed are the poor in spirit, for theirs is the Kingdom of Heaven.”"
	if got := formatBibleQuote(cleanQuoteText(state, raw)); got != want {
		t.Errorf("\n got %q\nwant %q", got, want)
	}
}

func TestCitedTextSharePreservesSourceLineBreaks(t *testing.T) {
	bd := NewBibleData()
	bd.Books = []string{"Psalms"}
	bd.Verses["Psalms"] = map[int][]Verse{19: {
		{BookName: "Psalms", Chapter: 19, Verse: 1,
			Text: "The heavens declare the glory of God.\nThe expanse shows his handiwork."},
	}}
	state := &AppState{Bible: bd, CurrentBook: "Psalms", CurrentChapter: 19}

	raw := "1 The heavens declare the glory of God. The expanse shows his handiwork."
	cleaned, cite := prepareShareQuote(state, raw)
	cleaned = restoreShareLineBreaks(state, cleaned)
	got := composeShareText(formatBibleQuote(cleaned), cite, "World English Bible")
	want := "“The heavens declare the glory of God.\nThe expanse shows his handiwork.”\n\n— Psalms 19:1 (World English Bible)"
	if got != want {
		t.Errorf("cited share must retain the source poetry line:\n got %q\nwant %q", got, want)
	}
}

func TestCitedTextSharePreservesParagraphsNotSoftWraps(t *testing.T) {
	longVerse := strings.Repeat("Alpha ", 54) + "ends."
	bd := NewBibleData()
	bd.Books = []string{"Test"}
	bd.Verses["Test"] = map[int][]Verse{1: {
		{BookName: "Test", Chapter: 1, Verse: 1, Text: longVerse},
		{BookName: "Test", Chapter: 1, Verse: 2, Text: "Second paragraph."},
	}}
	state := &AppState{Bible: bd, CurrentBook: "Test", CurrentChapter: 1}

	// The reader intentionally starts verse 2 as a new paragraph once the first
	// paragraph passes its measure; that structural break survives sharing.
	flat := longVerse + " Second paragraph."
	if got, want := restoreShareLineBreaks(state, flat), longVerse+"\n\nSecond paragraph."; got != want {
		t.Errorf("paragraph structure:\n got %q\nwant %q", got, want)
	}

	// A newline introduced only by display wrapping is absent from chapter data,
	// so the normalized share remains continuous prose.
	shortBD := NewBibleData()
	shortBD.Books = []string{"Test"}
	shortBD.Verses["Test"] = map[int][]Verse{1: {
		{BookName: "Test", Chapter: 1, Verse: 1, Text: "A line that wraps on a narrow screen."},
	}}
	shortState := &AppState{Bible: shortBD, CurrentBook: "Test", CurrentChapter: 1}
	cleaned, _ := prepareShareQuote(shortState, "A line that wraps\non a narrow screen.")
	if got, want := restoreShareLineBreaks(shortState, cleaned), "A line that wraps on a narrow screen."; got != want {
		t.Errorf("soft wrap must be flattened:\n got %q\nwant %q", got, want)
	}
}

// TestShareRestoresRealPoetryLines runs the WHOLE cited-text chain on the
// REAL captured node shape of Psalm 23:1-2 (bible.helloao.org, 2026-08-04):
// decoder (poem clauses → lines) → prepareShareQuote (flattens for Bluebook
// normalization) → restoreShareLineBreaks (re-inserts ONLY authored lines).
// This replaces the earlier fixture that hand-authored newlines the decoder
// could not produce (audit finding).
func TestShareRestoresRealPoetryLines(t *testing.T) {
	mk := func(raw string) []json.RawMessage {
		var c []json.RawMessage
		if err := json.Unmarshal([]byte(raw), &c); err != nil {
			t.Fatal(err)
		}
		return c
	}
	v1 := bsbVerseText(mk(`[{"text":"The LORD is my shepherd;","poem":1},{"noteId":1},{"text":"I shall not want.","poem":2}]`))
	v2 := bsbVerseText(mk(`[{"text":"He makes me lie down in green pastures;","poem":1},{"text":"He leads me beside quiet waters.","poem":2}]`))
	if !strings.Contains(v1, "\n") || !strings.Contains(v2, "\n") {
		t.Fatalf("decoder must produce poem lines: %q / %q", v1, v2)
	}

	bd := &BibleData{
		Books: []string{"Psalms"},
		Verses: map[string]map[int][]Verse{"Psalms": {23: {
			{BookName: "Psalms", Book: "Psalms", Chapter: 23, Verse: 1, Text: v1},
			{BookName: "Psalms", Book: "Psalms", Chapter: 23, Verse: 2, Text: v2},
		}}},
	}
	st := &AppState{Bible: bd, CurrentBook: "Psalms", CurrentChapter: 23}

	raw := "The LORD is my shepherd; I shall not want. 2 He makes me lie down in green pastures; He leads me beside quiet waters."
	text, cite := prepareShareQuote(st, raw)
	if strings.Contains(text, "\n") {
		t.Fatalf("pipeline text stays flat for Bluebook normalization: %q", text)
	}
	restored := restoreShareLineBreaks(st, text)
	want := "The LORD is my shepherd;\nI shall not want.\nHe makes me lie down in green pastures;\nHe leads me beside quiet waters."
	if restored != want {
		t.Errorf("poetry lines:\n got %q\nwant %q", restored, want)
	}
	if cite != "Psalms 23:1–2" {
		t.Errorf("citation = %q, want Psalms 23:1–2", cite)
	}
}
