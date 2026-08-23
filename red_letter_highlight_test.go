package bibletext

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
)

// john11State builds a chapter where verse 25 is BOTH highlighted and words of
// Christ — the overlap that was rendering wrong. John 11:25 is used because it
// is a real entry in wordsOfChristRanges (the table is keyed by book/chapter/
// verse, so a fabricated book would never be red and the test would pass while
// proving nothing), and because it is the verse the defect was found on.
func john11State(t *testing.T) *AppState {
	t.Helper()
	if !isWordsOfChrist("John", 11, 25) {
		t.Fatal("fixture: John 11:25 is not in the words-of-Christ table")
	}
	if isWordsOfChrist("John", 11, 24) {
		t.Fatal("fixture: John 11:24 should NOT be words of Christ — it is the control")
	}
	bd := NewBibleData()
	bd.Verses = map[string]map[int][]Verse{
		"John": {11: {
			{BookName: "John", Chapter: 11, Verse: 24, Text: "Martha replied, I know that he will rise again."},
			{BookName: "John", Chapter: 11, Verse: 25, Text: "I am the resurrection and the life."},
			{BookName: "John", Chapter: 11, Verse: 26, Text: "Everyone who lives and believes in Me will never die."},
		}},
	}
	bd.Books = []string{"John"}
	bd.PrepareSearchIndex()
	st := &AppState{Bible: bd, CurrentBook: "John", CurrentChapter: 11, CurrentVersion: "web"}
	st.setHL(hlSearch, "John", 11, 25, 25)
	return st
}

// A highlight is a BAND, not a recolour — so a highlighted words-of-Christ verse
// has to keep its red. Both native dialects used to emit the highlight OR the
// red, never both, so the verse somebody had just shared or searched for was the
// one place the red silently went missing.
func TestHighlightedWordsOfChristKeepTheirRed(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Preferences().SetBool(prefRedLetter, true)

	st := john11State(t)
	ch := st.Bible.GetChapter("John", 11)

	t.Run("apple", func(t *testing.T) {
		html := buildChapterHTML(st, ch)
		// v25 is highlighted AND red: it must wear both classes.
		if !strings.Contains(html, `<span class="hl wj">`) {
			t.Errorf("highlighted words-of-Christ verse is missing one of its classes:\n%s", html)
		}
		// v26 is red but NOT highlighted — the control. If this were absent the
		// test above could pass on a build that had simply stopped emitting .wj.
		if !strings.Contains(html, `<span class="wj">`) {
			t.Errorf("the unhighlighted words-of-Christ verse lost its red:\n%s", html)
		}
	})

	t.Run("android", func(t *testing.T) {
		html := buildChapterHTMLAndroid(st, ch)
		pal := st.pal()
		red, band := nrgbaToHex(pal.RedLetter), nrgbaToHex(pal.Highlight)

		// The EXACT paired emission, named in full. Scanning for "a band span
		// that contains some <font color>" is not good enough and was wrong here:
		// the verse NUMBER is emitted in its own background-color span and always
		// carries a <font color> of its own, so a looser check passed even with
		// the pairing arm removed entirely.
		want := `<span style="background-color:` + band + `"><font color="` + red + `">`
		if !strings.Contains(html, want) {
			t.Errorf("highlighted words-of-Christ body is not painted red inside the band.\nwant %s\ngot:\n%s", want, html)
		}
		// Control: v26 is red but unhighlighted, so a bare coloured run must exist
		// too — otherwise the check above could pass on a build emitting red
		// everywhere.
		if !strings.Contains(html, `<font color="`+red+`">Everyone who lives`) {
			t.Errorf("the unhighlighted words-of-Christ verse lost its red:\n%s", html)
		}
	})
}

// With red-letter mode OFF, a highlight is just a highlight — the pairing must
// not smuggle colour in.
func TestHighlightAloneWhenRedLetterIsOff(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Preferences().SetBool(prefRedLetter, false)

	st := john11State(t)
	ch := st.Bible.GetChapter("John", 11)

	// MARKUP, not the stylesheet. The Apple dialect always DEFINES .wj in its
	// <style> block, so a bare search for "wj" matches the rule and fails on a
	// perfectly correct document — the same trap TestNoHighlightMeansNoBand
	// documents for the band.
	if html := buildChapterHTML(st, ch); strings.Contains(html, `class="wj"`) ||
		strings.Contains(html, `class="hl wj"`) {
		t.Error("apple: red-letter is off but a verse is still wearing the wj class")
	}

	// Android has no stylesheet — the colour is inline — but the verse NUMBER is
	// also a <font color>, so the test has to name the colour it means rather
	// than looking for the tag.
	red := nrgbaToHex(st.pal().RedLetter)
	if html := buildChapterHTMLAndroid(st, ch); strings.Contains(html, `<font color="`+red+`"`) {
		t.Errorf("android: red-letter is off but body text is still painted %s:\n%s", red, html)
	}
}
