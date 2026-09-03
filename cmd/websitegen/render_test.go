package main

// Rendering rules the web page shares with the app. These are the invariants a
// careless refactor would silently break — a psalm flattened to prose, red
// letters lost, a verse anchor renamed (which would break every shared link),
// or verse text escaping out of its span.

import (
	"regexp"
	"strings"
	"testing"

	bibletext "github.com/cubancorona/bibletext"
)

func testVersion(books []string, verses map[string]map[int][]bibletext.Verse) loadedVersion {
	return loadedVersion{
		webVersion: webVersion{ID: "web", Name: "World English Bible"},
		bible:      &bibletext.BibleData{Books: books, Verses: verses},
	}
}

func v(book string, ch, num int, text string) bibletext.Verse {
	return bibletext.Verse{BookName: book, Book: book, Chapter: ch, Verse: num, Text: text}
}

// TestChapterBodyKeepsPoemLines: authored "\n" must become a real <br>, and the
// join between two poetic verses must break too — the same rule the reading
// pane and a shared quote use. Flattening here would turn a psalm into a
// paragraph on the one surface strangers see first.
func TestChapterBodyKeepsPoemLines(t *testing.T) {
	verses := []bibletext.Verse{
		v("Psalms", 23, 1, "The LORD is my shepherd;\nI shall not want."),
		v("Psalms", 23, 2, "He makes me lie down in green pastures;\nHe leads me beside quiet waters."),
	}
	got := chapterBody("web", "Psalms", verses)
	if strings.Contains(got, "\n") {
		t.Error("a literal newline survived into the HTML — it renders as a space, flattening the poem")
	}
	if n := strings.Count(got, "<br>"); n != 3 {
		t.Errorf("want 3 <br> (one inside each verse, one for the poetic join), got %d in:\n%s", n, got)
	}
}

// TestChapterBodyProseDoesNotBreak is the other half: prose must NOT gain line
// breaks, or every chapter would render as ragged one-line-per-verse.
func TestChapterBodyProseDoesNotBreak(t *testing.T) {
	verses := []bibletext.Verse{
		v("John", 3, 16, "For God so loved the world."),
		v("John", 3, 17, "For God didn't send his Son to judge the world."),
	}
	got := chapterBody("web", "John", verses)
	if strings.Contains(got, "<br>") {
		t.Errorf("prose verses must join with a space, not a break:\n%s", got)
	}
}

// TestChapterBodyVerseAnchors: the id is the deep-link contract. #v16 works with
// no JavaScript only because this element exists and is named exactly "v16".
func TestChapterBodyVerseAnchors(t *testing.T) {
	got := chapterBody("web", "John", []bibletext.Verse{v("John", 3, 16, "For God so loved the world.")})
	if !strings.Contains(got, `id="v16"`) {
		t.Errorf(`missing id="v16" — every shared link to this verse would fail to highlight:\n%s`, got)
	}
}

// TestChapterBodyRedLetters: words of Christ carry the class the stylesheet
// colours. John 3:16 is inside a red-letter range; John 3:1 is not.
func TestChapterBodyRedLetters(t *testing.T) {
	got := chapterBody("web", "John", []bibletext.Verse{v("John", 3, 16, "For God so loved the world.")})
	if !strings.Contains(got, `class="wj"`) {
		t.Errorf("John 3:16 should be red-letter:\n%s", got)
	}
	plain := chapterBody("web", "John", []bibletext.Verse{v("John", 3, 1, "Now there was a man of the Pharisees.")})
	if strings.Contains(plain, `class="wj"`) {
		t.Errorf("John 3:1 is narration, not words of Christ:\n%s", plain)
	}
}

// TestChapterBodyEscapes: verse text is data, never markup. (Scripture contains
// no angle brackets today; a future decoder change must not be able to inject.)
func TestChapterBodyEscapes(t *testing.T) {
	got := chapterBody("web", "John", []bibletext.Verse{v("John", 1, 1, `a <script>x</script> & "quote"`)})
	if strings.Contains(got, "<script>") {
		t.Errorf("verse text escaped out of its span:\n%s", got)
	}
	if !strings.Contains(got, "&lt;script&gt;") || !strings.Contains(got, "&amp;") {
		t.Errorf("expected escaped entities:\n%s", got)
	}
}

// TestRenderChapterHasPreviewMetadata: per-chapter Open Graph tags are the whole
// reason the site is pre-rendered — unfurlers don't run JavaScript.
func TestRenderChapterHasPreviewMetadata(t *testing.T) {
	verses := map[string]map[int][]bibletext.Verse{
		"John": {3: {v("John", 3, 16, "For God so loved the world.")}},
	}
	lv := testVersion([]string{"John"}, verses)
	page := renderChapter(lv, []loadedVersion{lv}, "John", "john", 3, 2, 4)

	for _, want := range []string{
		`<meta property="og:title" content="John 3 (World English Bible)">`,
		`<link rel="canonical" href="https://bibletext.co.uk/web/john/3/">`,
		`id="v16"`,
		`href="../2/"`, // previous chapter
		`href="../4/"`, // next chapter
	} {
		if !strings.Contains(page, want) {
			t.Errorf("chapter page missing %s", want)
		}
	}
	if !strings.Contains(page, "For God so loved the world.") {
		t.Error("the chapter page must contain the scripture in the served HTML")
	}
}

// TestSiteURLsMatchTheAppsLinks is the join between the two halves of the
// feature: the path the generator writes must be exactly the path the app's
// share link points at. If these ever drift, every shared link 404s.
func TestSiteURLsMatchTheAppsLinks(t *testing.T) {
	for _, tc := range []struct{ version, book string }{
		{"web", "John"}, {"bsb", "Psalms"}, {"webc", "1 Maccabees"},
	} {
		slug, ok := bibletext.BookSlug(tc.book)
		if !ok {
			t.Fatalf("%s has no slug", tc.book)
		}
		generated := tc.version + "/" + slug + "/3/index.html"
		shared := bibletext.ShareLinkURL(tc.version, tc.book, 3, 1, 0)
		wantPath := strings.TrimPrefix(shared, "https://bibletext.co.uk/")
		wantPath = strings.TrimSuffix(wantPath, "#v1") + "index.html"
		if generated != wantPath {
			t.Errorf("generator writes %q but the app links to %q", generated, wantPath)
		}
	}
}

// --- Red letters are per translation, and per stretch of a verse -------------
//
// The page was the fifth rendering surface and the only one still asking
// IsWordsOfChrist when the app grew per-edition span tables. These pin the two
// things that were wrong on it.

// A verse where somebody ELSE speaks must not come out wholly red. John 4:9 is
// the Samaritan woman answering; the BSB's table marks Christ's words in verses
// 7 and 10 and marks nothing in 9. Rendered through the old whole-verse rule the
// page put her words in Christ's colour.
func TestChapterBodyRedensOnlyChristsWordsInTheBSB(t *testing.T) {
	verses := []bibletext.Verse{
		v("John", 4, 7, `When a Samaritan woman came to draw water, Jesus said to her, “Give Me a drink.”`),
		v("John", 4, 9, `“You are a Jew,” said the woman. “How can you ask for a drink from me, a Samaritan woman?” (For Jews do not associate with Samaritans.)`),
	}
	got := chapterBody("bsb", "John", verses)

	if !strings.Contains(got, `<span class="wj">“Give Me a drink.”</span>`) {
		t.Errorf("Christ's words in v7 were not reddened on their own:\n%s", got)
	}
	if strings.Contains(got, `<span class="wj">“You are a Jew,”`) {
		t.Errorf("the Samaritan woman's words were painted in Christ's colour:\n%s", got)
	}
	// The narration that introduces the quotation stays black.
	if strings.Contains(got, `<span class="wj">When a Samaritan woman`) {
		t.Errorf("the narration was swept into the red span:\n%s", got)
	}
}

// The SAME verse rendered for two translations must use each one's own marks.
// Version-blind rendering is what put the WEB's marks on every page.
func TestChapterBodyRedLettersFollowTheTranslation(t *testing.T) {
	// Mark 5:31 — the NKJV's publisher reddens Christ here and the WEB does not.
	verse := []bibletext.Verse{
		v("Mark", 5, 31, `But His disciples said to Him, “You see the multitude thronging You, and You say, ‘Who touched Me?’ ”`),
	}
	web := chapterBody("web", "Mark", verse)
	nkjv := chapterBody("nkjv", "Mark", verse)
	if web == nkjv {
		t.Error("the WEB and the NKJV rendered Mark 5:31 identically; the page is still version-blind")
	}
}

// Whatever the marks say, the page must still show the whole verse: the runs
// concatenate back to the text, and a rendering bug that dropped one would be
// invisible in a red/not-red assertion.
func TestChapterBodyLosesNoTextToTheRuns(t *testing.T) {
	text := `And looking up to heaven, He sighed deeply and said to him, “Ephphatha!” (which means, “Be opened!”).`
	got := chapterBody("bsb", "Mark", []bibletext.Verse{v("Mark", 7, 34, text)})
	stripped := regexp.MustCompile(`<[^>]*>`).ReplaceAllString(got, "")
	stripped = strings.ReplaceAll(stripped, "\u00a0", " ")
	for _, word := range []string{"Ephphatha", "which means", "Be opened", "sighed deeply"} {
		if !strings.Contains(stripped, word) {
			t.Errorf("the rendered verse lost %q:\n%s", word, stripped)
		}
	}
}
