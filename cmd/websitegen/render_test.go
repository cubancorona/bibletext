package main

// Rendering rules the web page shares with the app. These are the invariants a
// careless refactor would silently break — a psalm flattened to prose, red
// letters lost, a verse anchor renamed (which would break every shared link),
// or verse text escaping out of its span.

import (
	"strings"
	"testing"

	bibletext "bibletext"
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
	got := chapterBody("Psalms", verses)
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
	got := chapterBody("John", verses)
	if strings.Contains(got, "<br>") {
		t.Errorf("prose verses must join with a space, not a break:\n%s", got)
	}
}

// TestChapterBodyVerseAnchors: the id is the deep-link contract. #v16 works with
// no JavaScript only because this element exists and is named exactly "v16".
func TestChapterBodyVerseAnchors(t *testing.T) {
	got := chapterBody("John", []bibletext.Verse{v("John", 3, 16, "For God so loved the world.")})
	if !strings.Contains(got, `id="v16"`) {
		t.Errorf(`missing id="v16" — every shared link to this verse would fail to highlight:\n%s`, got)
	}
}

// TestChapterBodyRedLetters: words of Christ carry the class the stylesheet
// colours. John 3:16 is inside a red-letter range; John 3:1 is not.
func TestChapterBodyRedLetters(t *testing.T) {
	got := chapterBody("John", []bibletext.Verse{v("John", 3, 16, "For God so loved the world.")})
	if !strings.Contains(got, `class="wj"`) {
		t.Errorf("John 3:16 should be red-letter:\n%s", got)
	}
	plain := chapterBody("John", []bibletext.Verse{v("John", 3, 1, "Now there was a man of the Pharisees.")})
	if strings.Contains(plain, `class="wj"`) {
		t.Errorf("John 3:1 is narration, not words of Christ:\n%s", plain)
	}
}

// TestChapterBodyEscapes: verse text is data, never markup. (Scripture contains
// no angle brackets today; a future decoder change must not be able to inject.)
func TestChapterBodyEscapes(t *testing.T) {
	got := chapterBody("John", []bibletext.Verse{v("John", 1, 1, `a <script>x</script> & "quote"`)})
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
