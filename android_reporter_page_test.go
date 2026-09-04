package bibletext

// THE ANDROID REPORTER PAGE, which reaches the reader by three different
// routes: the first-line indent is markup (android_chapter_html.go), the
// paragraph gap is closed by the importer's mode (BtBridge.setHtml, COMPACT),
// and the measure is a width the bridge centres (androidReadingMeasureDp).
// These lock the two halves the host can see, and lock the phone page against
// picking either up by accident.

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
)

// The em+en pair as characters, after the entities are read back.
const androidReporterIndent = "\u2003\u2002"

func androidReporterState() *AppState {
	prose := "In the beginning God created the heavens and the earth. Now the earth was formless and void, and darkness was over the surface of the deep. And the Spirit of God was hovering over the surface of the waters. And God said, Let there be light, and there was light. And God saw that the light was good, and He separated the light from the darkness."
	bd := &BibleData{
		Books: []string{"Genesis"},
		Verses: map[string]map[int][]Verse{"Genesis": {1: {
			{BookName: "Genesis", Book: "Genesis", Chapter: 1, Verse: 1, Text: prose},
			{BookName: "Genesis", Book: "Genesis", Chapter: 1, Verse: 2, Text: "And God said, Let there be a firmament in the midst of the waters, and let it divide the waters from the waters."},
		}}},
	}
	return &AppState{Bible: bd, CurrentBook: "Genesis", CurrentChapter: 1}
}

func androidChapterHTMLWithReporter(t *testing.T, on bool) string {
	t.Helper()
	orig := reporterLayout
	reporterLayout = func() bool { return on }
	defer func() { reporterLayout = orig }()
	st := androidReporterState()
	return buildChapterHTMLAndroid(st, st.Bible.GetChapter("Genesis", 1))
}

// The reporter page indents every prose paragraph — including the first, which
// is where the reader learns what an indent means on this page.
func TestAndroidReporterPageIndentsItsParagraphs(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	html := androidChapterHTMLWithReporter(t, true)
	if !strings.HasPrefix(html, "<p>&#8195;&#8194;") {
		t.Errorf("the first paragraph is indented too:\n%s", html)
	}
	if got := strings.Count(html, "<p>&#8195;&#8194;"); got != 2 {
		t.Errorf("both prose paragraphs must be indented, found %d:\n%s", got, html)
	}
}

// The paragraphs stay BLOCKS. The gap between them is closed by importing in
// COMPACT mode (BtBridge.setHtml), not by joining the markup — a body that
// stopped saying where a paragraph begins would put a note's band above the
// wrong passage, and would make paragraph_identity_test.go unable to see it.
func TestAndroidReporterPageKeepsParagraphsAsBlocks(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	html := androidChapterHTMLWithReporter(t, true)
	if !strings.Contains(html, "</p><p>") {
		t.Errorf("paragraph boundaries must stay in the markup:\n%s", html)
	}
	if strings.Contains(html, "<br>&#8195;&#8194;") {
		t.Errorf("an indent must not follow a hard break — that is a poem line:\n%s", html)
	}
}

// The phone page is untouched: separate blocks, no indent anywhere.
func TestAndroidPhonePageKeepsItsParagraphBlocks(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	html := androidChapterHTMLWithReporter(t, false)
	if !strings.Contains(html, "</p><p>") {
		t.Errorf("the phone page separates paragraphs with blocks:\n%s", html)
	}
	if strings.Contains(html, "&#8195;&#8194;") || strings.Contains(html, androidReporterIndent) {
		t.Errorf("the phone page must not indent:\n%s", html)
	}
}

// Poetry is never first-line indented in print, and with the gap gone the
// indent is what marks a paragraph — so the rule has to be read on a chapter
// that contains BOTH kinds, not only on one that can trivially satisfy it.
func TestAndroidReporterPageIndentsProseAndNotPoetry(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	orig := reporterLayout
	reporterLayout = func() bool { return true }
	defer func() { reporterLayout = orig }()

	// A chapter of poetry throughout: no indent anywhere.
	st := psalm23State()
	html := buildChapterHTMLAndroid(st, st.Bible.GetChapter("Psalms", 23))
	if strings.Contains(html, "&#8195;&#8194;") {
		t.Errorf("a chapter of poetry must carry no indent:\n%s", html)
	}

	// The mixed case the Apple dialect is tested on (reading_poetry_test.go):
	// a paragraph that OPENS with prose keeps the indent even though it turns
	// into poetry, because without it the paragraph would read as a
	// continuation of the one before.
	bd := &BibleData{
		Books: []string{"Exodus"},
		Verses: map[string]map[int][]Verse{"Exodus": {15: {
			{BookName: "Exodus", Book: "Exodus", Chapter: 15, Verse: 1,
				Text: "Then Moses and the Israelites sang this song to the LORD:"},
			{BookName: "Exodus", Book: "Exodus", Chapter: 15, Verse: 2,
				Text: "The LORD is my strength and my song,\nand He has become my salvation."},
		}}},
	}
	st2 := &AppState{Bible: bd, CurrentBook: "Exodus", CurrentChapter: 15}
	mixed := buildChapterHTMLAndroid(st2, bd.GetChapter("Exodus", 15))
	if !strings.HasPrefix(mixed, "<p>&#8195;&#8194;") {
		t.Errorf("a mixed paragraph opening with prose must keep its indent:\n%s", mixed)
	}
	// And the poem line inside it is still a hard break, not a new paragraph.
	if !strings.Contains(mixed, "<br>") {
		t.Errorf("the poem line must still break:\n%s", mixed)
	}
	if strings.Contains(mixed, "<br>&#8195;&#8194;") {
		t.Errorf("a poem line must not be indented like a paragraph:\n%s", mixed)
	}
}

// The measure the bridge is handed: em-based, so it widens with the text size,
// and zero for the phone page.
func TestAndroidReadingMeasureDp(t *testing.T) {
	for _, tc := range []struct {
		name     string
		reporter bool
		textDp   float32
		want     float32
	}{
		{"phone page", false, 21, 0},
		{"reporter at Normal", true, 21, 27.5 * 21},
		{"reporter at a larger size", true, 26, 27.5 * 26},
		{"no text size yet", true, 0, 0},
	} {
		if got := androidReadingMeasureDp(tc.reporter, tc.textDp); got != tc.want {
			t.Errorf("%s: measure = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// The reporter page has no blank line between blocks, so the air above the
// footnote rule is written into the separator itself — the same <br> the Apple
// page adds for the same reason (writeFootnoteSection).
func TestAndroidReporterPageOpensAirAboveTheFootnoteRule(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setFootnotesEnabled(true)
	defer setFootnotesEnabled(false)

	verses := footnoteFixtureVerses()
	st := footnoteFixtureState(verses)
	for _, tc := range []struct {
		name             string
		reporter, wantBr bool
	}{
		{"phone page keeps the importer's blank line", false, false},
		{"reporter page writes the air itself", true, true},
	} {
		var html string
		withReporterLayout(tc.reporter, func() { html = buildChapterHTMLAndroid(st, verses) })
		if !strings.Contains(html, "<sup>&#160;</sup>") {
			t.Fatalf("%s: no footnote section rendered:\n%s", tc.name, html)
		}
		after := strings.Split(html, "<sup>&#160;</sup>")[1]
		if len(after) > 60 {
			after = after[:60]
		}
		br := strings.Contains(after, "<br>")
		if br != tc.wantBr {
			t.Errorf("%s: separator carries <br> = %v, want %v:\n%s", tc.name, br, tc.wantBr, html)
		}
	}
}
