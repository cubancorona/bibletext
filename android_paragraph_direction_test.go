package bibletext

import (
	"regexp"
	"strings"
	"testing"
)

// A PARAGRAPH OF THE ANDROID READING PANE THAT OPENS ON A RIGHT-TO-LEFT LETTER
// IS STILL SET LEFT TO RIGHT, as the web, the Apple panes and the Windows and
// Linux pane set every paragraph.
//
// THE DEFECT THIS EXISTS FOR. The reading TextView gives each paragraph the
// direction of its first strong character. The NKJV heads each of Psalm 119's
// twenty-two stanzas with a Hebrew letter and then its name, so Android set
// every one of those headings right to left — against the right edge, the
// letter to the right of the name — where every other surface sets the letter
// first, flush left. The view's own setTextDirection cannot overrule this in
// this app (a View resolves any text direction to FIRST_STRONG unless the
// application declares supportsRtl; measured on the API 35 emulator, the
// heading stayed right to left with it set), so the dialect opens such a
// paragraph with a LEFT-TO-RIGHT MARK and the bridge drops the mark from the
// text that leaves the page.
func TestAndroidSetsAParagraphOpeningOnHebrewLeftToRight(t *testing.T) {
	const mark = "&#x200E;"
	st := sampleState()
	ch := st.Bible.GetChapter(st.CurrentBook, st.CurrentChapter)
	withHeading := func(text string) string {
		st.Bible.Headings = map[string]map[int][]Heading{
			st.CurrentBook: {st.CurrentChapter: {{Text: text, Style: "qa", BeforeVerse: ch[0].Verse}}},
		}
		return buildChapterHTMLAndroid(st, ch)
	}

	// A stanza heading: the mark, inside the <b> so the bridge still finds a
	// paragraph wholly in bold, then the letter and the name as they were.
	if html := withHeading("א Fixture"); !strings.Contains(html, "<p><b>"+mark+"א Fixture</b></p>") {
		t.Errorf("a heading that opens on a Hebrew letter must open on a left-to-right mark, "+
			"or the pane sets it right to left against the right edge; the page came out as:\n%s", html)
	}
	// The Psalm title is written by the same rule, on the phone page and on
	// the book page a wide pane takes, which write it on two different lines:
	// the book page's title keeps its own air as an empty line inside the
	// block (<br> before </p>). Each page is pinned, so neither line rests on
	// the width of the host the test runs on.
	st.CurrentBook, st.CurrentChapter = "Psalms", 150
	st.Bible.Verses["Psalms"] = map[int][]Verse{150: {{BookName: "Psalms", Chapter: 150, Verse: 1, Text: "A fixture verse."}}}
	pages := map[bool]string{false: "phone", true: "book"}
	withTitle := func(text string, book bool) (html string) {
		st.Bible.Superscriptions = map[string]map[int]Superscription{"Psalms": {150: {Text: text}}}
		withReporterLayout(book, func() { html = buildChapterHTMLAndroid(st, st.Bible.GetChapter("Psalms", 150)) })
		if book != strings.Contains(html, "</i><br></p>") {
			t.Fatalf("the title was not written on the %s page's line, so that line goes unchecked:\n%s", pages[book], html)
		}
		return html
	}
	for _, book := range []bool{false, true} {
		if html := withTitle("א fixture title.", book); !strings.Contains(html, "<i>"+mark+"א fixture title.</i>") {
			t.Errorf("on the %s page, a Psalm title that opens on a Hebrew letter must open on a "+
				"left-to-right mark:\n%s", pages[book], html)
		}
		// The control: a title that opens on a Latin letter is written
		// exactly as before, even with Hebrew later in the line.
		if html := withTitle("A fixture title, א.", book); strings.Contains(html, mark) {
			t.Errorf("on the %s page, a title that opens on a Latin letter was given a direction mark:\n%s",
				pages[book], html)
		}
	}

	// The control for a heading: one that opens on a Latin letter, and every
	// verse, is written exactly as before — no mark anywhere — even with
	// Hebrew later in the line.
	st = sampleState()
	ch = st.Bible.GetChapter(st.CurrentBook, st.CurrentChapter)
	if html := withHeading("A Fixture א Heading"); !strings.Contains(html, "<p><b>A Fixture א Heading</b></p>") ||
		strings.Contains(html, mark) {
		t.Errorf("a heading that opens on a Latin letter, or a verse, was given a direction mark:\n%s", html)
	}

	// The bridge drops the mark: the same character, and only where a
	// paragraph opens, which is the one place the dialect writes it.
	java := readNativeSource(t, "android/BtBridge.java")
	if m := regexp.MustCompile(`static final char PARA_LTR_MARK = '\\u([0-9A-Fa-f]{4})';`).FindStringSubmatch(java); m == nil ||
		!strings.EqualFold("&#x"+m[1]+";", mark) {
		t.Errorf("the bridge's PARA_LTR_MARK is not the dialect's %s (found %v)", mark, m)
	}
	drops := func(src string) bool {
		read := javaBlockAfter(t, src, "private static String readerText(")
		return inSequence(read, "if (i == p && c == PARA_LTR_MARK) continue;", "sb.append(c);")
	}
	if !drops(java) {
		t.Error("readerText must drop a paragraph's opening direction mark, or it leaves the page " +
			"in the text a selection hands on and in Copy")
	}
	if drops(strings.Replace(java, "if (i == p && c == PARA_LTR_MARK) continue;", "", 1)) {
		t.Fatal("control: the check passes a bridge that keeps the mark, so it proves nothing")
	}
}
