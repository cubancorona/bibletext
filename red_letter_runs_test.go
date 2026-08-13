package bibletext

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
)

// The BSB pane must colour only the words that are Christ's. Before spans, the
// whole verse went red — including the other speaker's reply.
func TestApplePaneRedensOnlyChristsWordsInTheBSB(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Preferences().SetBool(prefRedLetter, true)

	key := verseKeyFor("Mark", 8, 5)
	text := bsbVerseFixture[key]
	bd := NewBibleData()
	bd.Verses = map[string]map[int][]Verse{
		"Mark": {8: {{BookName: "Mark", Chapter: 8, Verse: 5, Text: text}}},
	}
	bd.Books = []string{"Mark"}
	bd.PrepareSearchIndex()
	st := &AppState{Bible: bd, CurrentBook: "Mark", CurrentChapter: 8, CurrentVersion: "bsb"}

	html := buildChapterHTML(st, bd.GetChapter("Mark", 8))
	red := redTextOf(html)
	if !strings.Contains(red, "How many loaves do you have") {
		t.Errorf("His question is not red:\n%s", html)
	}
	if strings.Contains(red, "Seven") {
		t.Errorf("the disciples' answer was reddened:\n%s", html)
	}
	if strings.Contains(red, "Jesus asked") {
		t.Errorf("the narration was reddened:\n%s", html)
	}
}

// With the switch off, the pane must go back to reddening the whole verse —
// otherwise "turn it off" is not something we can actually do.
func TestApplePaneFallsBackWhenTheSwitchIsOff(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Preferences().SetBool(prefRedLetter, true)
	t.Setenv("BIBLETEXT_BSB_RED_LETTER", "0")

	key := verseKeyFor("Mark", 8, 5)
	bd := NewBibleData()
	bd.Verses = map[string]map[int][]Verse{
		"Mark": {8: {{BookName: "Mark", Chapter: 8, Verse: 5, Text: bsbVerseFixture[key]}}},
	}
	bd.Books = []string{"Mark"}
	bd.PrepareSearchIndex()
	st := &AppState{Bible: bd, CurrentBook: "Mark", CurrentChapter: 8, CurrentVersion: "bsb"}

	red := redTextOf(buildChapterHTML(st, bd.GetChapter("Mark", 8)))
	if !strings.Contains(red, "Seven") {
		t.Error("switched off, the whole verse should be red again — the old behaviour")
	}
}

// redTextOf returns the concatenated contents of every wj span.
func redTextOf(html string) string {
	var out strings.Builder
	for _, open := range []string{`<span class="wj">`, `<span class="hl wj">`} {
		rest := html
		for {
			i := strings.Index(rest, open)
			if i < 0 {
				break
			}
			rest = rest[i+len(open):]
			j := strings.Index(rest, "</span>")
			if j < 0 {
				break
			}
			out.WriteString(rest[:j])
			rest = rest[j:]
		}
	}
	return out.String()
}

// Android renders its own HTML dialect, so it needs its own proof.
func TestAndroidPaneRedensOnlyChristsWordsInTheBSB(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Preferences().SetBool(prefRedLetter, true)

	text := bsbVerseFixture[verseKeyFor("Mark", 8, 5)]
	bd := NewBibleData()
	bd.Verses = map[string]map[int][]Verse{
		"Mark": {8: {{BookName: "Mark", Chapter: 8, Verse: 5, Text: text}}},
	}
	bd.Books = []string{"Mark"}
	bd.PrepareSearchIndex()
	st := &AppState{Bible: bd, CurrentBook: "Mark", CurrentChapter: 8, CurrentVersion: "bsb"}

	html := buildChapterHTMLAndroid(st, bd.GetChapter("Mark", 8))
	red := nrgbaToHex(st.pal().RedLetter)
	var coloured strings.Builder
	rest := html
	open := `<font color="` + red + `">`
	for {
		i := strings.Index(rest, open)
		if i < 0 {
			break
		}
		rest = rest[i+len(open):]
		j := strings.Index(rest, "</font>")
		if j < 0 {
			break
		}
		coloured.WriteString(rest[:j])
		rest = rest[j:]
	}
	got := coloured.String()
	if !strings.Contains(got, "How many loaves do you have") {
		t.Errorf("His question is not red:\n%s", html)
	}
	if strings.Contains(got, "Seven") {
		t.Errorf("the disciples' answer was reddened:\n%s", html)
	}
}

// The NKJV's spans are the PUBLISHER's own words-of-Jesus marks, not ours, so
// the same verse must colour the same way — and the disciples' reply must not.
func TestNKJVUsesItsOwnSpans(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Preferences().SetBool(prefRedLetter, true)

	key := verseKeyFor("Mark", 8, 5)
	spans, ok := nkjvRedLetterSpans[key]
	if !ok || len(spans) == 0 {
		t.Fatal("no NKJV span data for Mark 8:5")
	}
	// The NKJV's own table must be independent of the BSB's — same verse, but
	// different translations, so identical offsets would mean one overwrote the
	// other.
	if bsb, ok := bsbRedLetterSpans[key]; ok && len(bsb) == len(spans) && bsb[0] == spans[0] {
		t.Error("the NKJV spans are identical to the BSB's; one table is feeding the other")
	}
	if n := nkjvRedLetterRunes[key]; n == 0 {
		t.Error("no rune length recorded, so the guard cannot fire")
	}
}

// Every edition we hold spans for must go through one lookup, and editions we
// do not hold must still get the whole-verse answer.
func TestRedLetterSpanLookupCoversTheRightEditions(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	text := bsbVerseFixture[verseKeyFor("Mark", 8, 5)]

	if _, ok := redLetterSpansFor("bsb", "Mark", 8, 5, text); !ok {
		t.Error("bsb: expected span data")
	}
	for _, vid := range []string{"web", "webc", "kjv", ""} {
		if _, ok := redLetterSpansFor(vid, "Mark", 8, 5, text); ok {
			t.Errorf("%q: got span data, but no table exists for it — it must fall back to the whole verse", vid)
		}
	}
	t.Setenv("BIBLETEXT_BSB_RED_LETTER", "0")
	if _, ok := redLetterSpansFor("nkjv", "Mark", 8, 5, text); ok {
		t.Error("nkjv: the switch is off but spans were still handed out")
	}
}
