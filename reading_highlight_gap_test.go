package bibletext

// A highlighted range must be ONE continuous band.
//
// The bug this pins: the space joining two verses inside a paragraph was written
// bare, between the two verses' spans, so it belonged to neither. A highlighted
// range came out notched — one hole per join — and only the joins that happened
// to fall mid-line were visible, which is why it read as an intermittent
// rendering fault rather than a layout bug. observed in practice from a photo of
// Romans 8:1-4, and present in all three dialects: the iOS/macOS HTML, the
// Android HTML, and the web (fixed there in JS, see cmd/websitegen/assets.go).

import (
	"regexp"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
)

// romans8State is a prose (non-poetic) chapter with several verses in one
// paragraph — the shape that shows the defect. Poetic chapters join with <br>,
// which has no width and therefore no gap to leave.
func romans8State(t *testing.T) *AppState {
	t.Helper()
	bd := NewBibleData()
	bd.Verses = map[string]map[int][]Verse{
		"Romans": {8: {
			{BookName: "Romans", Chapter: 8, Verse: 1, Text: "Therefore, there is now no condemnation."},
			{BookName: "Romans", Chapter: 8, Verse: 2, Text: "For the law of the Spirit set you free."},
			{BookName: "Romans", Chapter: 8, Verse: 3, Text: "For what the law was powerless to do."},
			{BookName: "Romans", Chapter: 8, Verse: 4, Text: "So that the righteous standard is fulfilled."},
			{BookName: "Romans", Chapter: 8, Verse: 5, Text: "Those who live according to the flesh."},
		}},
	}
	bd.Books = []string{"Romans"}
	bd.PrepareSearchIndex()
	st := &AppState{Bible: bd, CurrentBook: "Romans", CurrentChapter: 8}
	st.HighlightedBook, st.HighlightedChapter = "Romans", 8
	st.HighlightedVerse, st.HighlightedVerseEnd = 1, 4
	st.HasHighlightedVerse = true
	return st
}

// bareJoin matches a highlighted verse ending, then an UNWRAPPED space, then the
// next verse starting — the notch.
var bareJoinIOS = regexp.MustCompile(`</span> +<sup class="v hl"`)

func TestHighlightBandHasNoGapsIOS(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := romans8State(t)
	html := buildChapterHTML(st, st.Bible.GetChapter("Romans", 8))

	if loc := bareJoinIOS.FindStringIndex(html); loc != nil {
		t.Errorf("unhighlighted space between two highlighted verses at %d:\n…%s…",
			loc[0], html[max(0, loc[0]-60):min(len(html), loc[1]+60)])
	}
	// And the join really is there, wrapped — otherwise the check above passes
	// on a chapter that simply has no joins.
	if !strings.Contains(html, `<span class="hl"> </span>`) {
		t.Errorf("expected the joining space to be inside the band:\n%s", html)
	}
	// The band must stop at the edge of the range: verse 5 is not highlighted, so
	// the join from 4 to 5 stays a plain space.
	if !strings.Contains(html, `</span> <sup class="v">5</sup>`) {
		t.Errorf("the join to the first UNhighlighted verse must stay plain:\n%s", html)
	}
}

func TestHighlightBandHasNoGapsAndroid(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := romans8State(t)
	html := buildChapterHTMLAndroid(st, st.Bible.GetChapter("Romans", 8))

	// Android wraps in an inline style rather than a class.
	bare := regexp.MustCompile(`</span> +<span style="background-color:[^"]*"><sup>`)
	if loc := bare.FindStringIndex(html); loc != nil {
		t.Errorf("unhighlighted space between two highlighted verses at %d:\n…%s…",
			loc[0], html[max(0, loc[0]-60):min(len(html), loc[1]+60)])
	}
	if !regexp.MustCompile(`<span style="background-color:[^"]*"> </span>`).MatchString(html) {
		t.Errorf("expected the joining space to be inside the band:\n%s", html)
	}
	// The verse NUMBER joins the band too — leaving it out punches the same hole.
	if !regexp.MustCompile(`<span style="background-color:[^"]*"><sup><small>`).MatchString(html) {
		t.Errorf("expected the verse number inside the band:\n%s", html)
	}
}

// With nothing highlighted, nothing is wrapped — the fix must not tint a chapter
// the reader never highlighted.
func TestNoHighlightMeansNoBand(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := romans8State(t)
	st.HasHighlightedVerse = false

	// Markup only, not the stylesheet: the iOS dialect always DEFINES .hl in its
	// <style> block, so searching the whole document for "background-color" finds
	// the rule rather than a band. What must be absent is any element wearing it.
	ios := buildChapterHTML(st, st.Bible.GetChapter("Romans", 8))
	if strings.Contains(ios, `class="hl"`) {
		t.Errorf("ios: a band appeared with nothing highlighted:\n%s", ios)
	}
	android := buildChapterHTMLAndroid(st, st.Bible.GetChapter("Romans", 8))
	if strings.Contains(android, `<span style="background-color:`) {
		t.Errorf("android: a band appeared with nothing highlighted:\n%s", android)
	}
}
