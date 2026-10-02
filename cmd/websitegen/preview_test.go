package main

import (
	"html"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	bibletext "github.com/cubancorona/bibletext"
)

// A LINK PREVIEW SAYS WHAT THE APP'S SHARE SAYS. The og:description under a
// shared chapter is plain text a messenger draws, and the app sends a verse
// with the divine name in the small capitals the page draws (sharedText); the
// preview joined the stored text, so 437 of the NKJV's chapters previewed the
// name as an ordinary "Lord". The control is the same verse with nothing
// marked, which must come out exactly as stored.
func TestChapterPreviewWritesTheDivineNameAsTheAppSharesIt(t *testing.T) {
	text := "In the beginning of the fixture the Lord spoke."
	at := utf8.RuneCountInString(text[:strings.Index(text, "Lord")])
	marked := []bibletext.Verse{{Verse: 1, Text: text, SmallCaps: []bibletext.TextSpan{{Start: at, End: at + 4}}}}
	if got, want := chapterPreview(marked), "In the beginning of the fixture the Lᴏʀᴅ spoke."; got != want {
		t.Errorf("preview = %q, want %q", got, want)
	}
	if got, want := chapterPreview(marked), bibletext.VerseSharedText(marked[0]); got != want {
		t.Errorf("preview = %q, the app's share = %q", got, want)
	}
	plain := []bibletext.Verse{{Verse: 1, Text: text}}
	if got := chapterPreview(plain); got != text {
		t.Errorf("an unmarked verse previews as %q, want it unchanged", got)
	}
}

// The preview is cut near 200 bytes. With no space to cut at, the cut backs
// off to a whole character: small capitals are two and three bytes, and a cut
// through one would put invalid UTF-8 in every messenger's preview.
func TestChapterPreviewNeverSplitsACharacter(t *testing.T) {
	long := strings.Repeat("Lord", 80)
	var caps []bibletext.TextSpan
	for i := 0; i < 80; i++ {
		caps = append(caps, bibletext.TextSpan{Start: 4 * i, End: 4*i + 4})
	}
	for lead := 0; lead < 3; lead++ {
		v := bibletext.Verse{Verse: 1, Text: strings.Repeat("x", lead) + long}
		for _, sp := range caps {
			v.SmallCaps = append(v.SmallCaps, bibletext.TextSpan{Start: sp.Start + lead, End: sp.End + lead})
		}
		got := chapterPreview([]bibletext.Verse{v})
		if !utf8.ValidString(got) {
			t.Errorf("lead %d: the preview is not valid UTF-8", lead)
		}
		if !strings.HasSuffix(got, "…") || len(got) > 200+len("…") {
			t.Errorf("lead %d: the preview is %d bytes and not cut", lead, len(got))
		}
	}
}

// metaContent is the content of the one <meta ATTR> tag a page carries.
func metaContent(t *testing.T, page, attr string) string {
	t.Helper()
	m := regexp.MustCompile(`<meta `+regexp.QuoteMeta(attr)+` content="([^"]*)">`).FindAllStringSubmatch(page, -1)
	if len(m) != 1 {
		t.Fatalf("the page has %d <meta %s> tags", len(m), attr)
	}
	return html.UnescapeString(m[0][1])
}

// psalmPage renders Psalm 3 of a one-chapter edition through renderChapter.
func psalmPage(id string, verses []bibletext.Verse, licence *webLicence) string {
	lv := loadedVersion{
		webVersion: webVersion{ID: id, Name: "Fixture Edition"},
		bible: &bibletext.BibleData{Books: []string{"Psalms"},
			Verses: map[string]map[int][]bibletext.Verse{"Psalms": {3: verses}}},
		licence: licence,
	}
	return renderChapter(lv, []loadedVersion{lv}, "Psalms", "psalms", 3, 2, 4)
}

// A PERSON READS THE SMALL CAPITALS; A MACHINE READS CAPITALS. A chapter page
// carries its opening twice. og:description is what a messenger shows a person
// under a shared link, and keeps the divine name in the small capitals a share
// sends. <meta name="description"> is what a search engine reads, and text for
// a machine takes the name in CAPITALS through outboundText, as an AI request
// does (docs/DIVINE_NAME.md). Both are read off the page renderChapter writes,
// for a licensed edition's chapter with its footer and stylesheet. The control
// is a public-domain chapter, which marks nothing and spells the name in
// literal capitals: both tags carry the stored text, unchanged.
func TestAChapterDescribesTheDivineNameToAPersonAndToAMachine(t *testing.T) {
	text := "A fixture verse in which the Lord answers \"soon\"."
	marked := bibletext.Verse{BookName: "Psalms", Book: "Psalms", Chapter: 3, Verse: 1, Text: text,
		SmallCaps: smallCapsSpan(text)}
	page := psalmPage("nkjv", []bibletext.Verse{marked},
		&webLicence{Notice: "A fixture notice.", Retrieved: londonDate(fixedRetrieval)})
	if got, want := metaContent(t, page, `property="og:description"`), `A fixture verse in which the Lᴏʀᴅ answers "soon".`; got != want {
		t.Errorf("og:description = %q, want %q: the preview a person reads keeps the small capitals", got, want)
	}
	if got, want := metaContent(t, page, `name="description"`), `A fixture verse in which the LORD answers "soon".`; got != want {
		t.Errorf("the meta description = %q, want %q: text for a machine takes capitals", got, want)
	}
	if !strings.Contains(page, `<p class="lic">A fixture notice.</p>`) || !strings.Contains(page, nkjvCSSName) {
		t.Error("the licensed chapter lost its footer or its stylesheet to the description")
	}

	stored := "A fixture verse in which the LORD answers."
	page = psalmPage("web", []bibletext.Verse{{BookName: "Psalms", Book: "Psalms", Chapter: 3, Verse: 1, Text: stored}}, nil)
	for _, attr := range []string{`property="og:description"`, `name="description"`} {
		if got := metaContent(t, page, attr); got != stored {
			t.Errorf("a public-domain chapter's %s = %q, want the stored %q", attr, got, stored)
		}
	}
}

// The description is the preview's own words, cut where the preview is cut:
// capitals are fewer bytes than small capitals, so a description cut on its own
// would run on past the word the preview ends with.
func TestTheDescriptionIsCutWhereThePreviewIs(t *testing.T) {
	var verses []bibletext.Verse
	for n := 1; n <= 12; n++ {
		text := "The fixture Lord speaks in verse " + strings.Repeat("x", n) + "."
		verses = append(verses, bibletext.Verse{BookName: "Psalms", Book: "Psalms", Chapter: 3, Verse: n,
			Text: text, SmallCaps: smallCapsSpan(text)})
	}
	page := psalmPage("nkjv", verses, nil)
	og, desc := metaContent(t, page, `property="og:description"`), metaContent(t, page, `name="description"`)
	if !strings.HasSuffix(og, "…") || !strings.Contains(og, "Lᴏʀᴅ") {
		t.Fatalf("the fixture preview was not cut or names nothing, so this test proves nothing: %q", og)
	}
	if want := strings.ReplaceAll(og, "Lᴏʀᴅ", "LORD"); desc != want {
		t.Errorf("the description is not the preview's own words in capitals:\n preview %q\n    desc %q\n    want %q", og, desc, want)
	}
}
