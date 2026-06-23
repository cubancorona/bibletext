package bibletext

import (
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
		{"span", "For God so loved the world, that he gave his one and only Son. For God didn't send his Son into the world to judge the world.", "John 3:16-17"},
		{"unmatched", "a phrase not present anywhere here", "John 3"},
	}
	for _, c := range cases {
		if got := citationForSelection(state, c.sel); got != c.want {
			t.Errorf("%s: citationForSelection = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestRenderVerseImage(t *testing.T) {
	path, err := renderVerseImage(&AppState{}, "For God so loved the world, that he gave his one and only Son.", "John 3:16", "WEB")
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
	path, err := renderVerseImage(&AppState{}, long, "John 3:16-18", "WEB")
	if err != nil {
		t.Fatalf("long render: %v", err)
	}
	os.Remove(path)
}

func TestFormatBibleQuote(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{
			"plain verse gets outer quotes",
			"For God so loved the world, that he gave his one and only Son.",
			"“For God so loved the world, that he gave his one and only Son.”",
		},
		{
			"balanced dialogue kept as-is, no outer quotes",
			"Jesus said to him, “I am the way, the truth, and the life.”",
			"Jesus said to him, “I am the way, the truth, and the life.”",
		},
		{
			"orphan opening quote stripped, then wrapped",
			"“Blessed are the poor in spirit, for theirs is the Kingdom of Heaven.",
			"“Blessed are the poor in spirit, for theirs is the Kingdom of Heaven.”",
		},
		{
			"orphan closing quote stripped, then wrapped",
			"why have you forsaken me?”",
			"“why have you forsaken me?”",
		},
		{
			"nested quotes within balanced dialogue left intact",
			"But he answered, “It is written, ‘Man shall not live by bread alone.’”",
			"But he answered, “It is written, ‘Man shall not live by bread alone.’”",
		},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		if got := formatBibleQuote(tc.in); got != tc.want {
			t.Errorf("%s:\n got %q\nwant %q", tc.name, got, tc.want)
		}
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

	raw := "3 “Blessed are the poor in spirit, for theirs is the Kingdom of Heaven."
	want := "“Blessed are the poor in spirit, for theirs is the Kingdom of Heaven.”"
	if got := formatBibleQuote(cleanQuoteText(state, raw)); got != want {
		t.Errorf("\n got %q\nwant %q", got, want)
	}
}
