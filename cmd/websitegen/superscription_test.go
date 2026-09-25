package main

import (
	"strings"
	"testing"

	bibletext "github.com/cubancorona/bibletext"
)

// S10 of docs/SCRIPTURE_WORKLIST.md. The site rendered verses and the
// publisher's section headings and nothing else beside them, so a psalm arrived
// on the web without the title Scripture gives it — while every app pane draws
// one. The title is never in Verse.Text (BibleData keeps Superscriptions
// separate on purpose), which is exactly why the page can draw it without any
// risk of it entering search, sharing or a link.

func psalmWithTitle() (*bibletext.BibleData, []bibletext.Verse) {
	verses := []bibletext.Verse{
		{BookName: "Psalms", Book: "Psalms", Chapter: 3, Verse: 1,
			Text: "Yahweh, how my adversaries have increased!"},
		{BookName: "Psalms", Book: "Psalms", Chapter: 3, Verse: 2,
			Text: "Many there are who say of my soul, there is no help."},
	}
	bd := &bibletext.BibleData{
		Verses: map[string]map[int][]bibletext.Verse{"Psalms": {3: verses}},
		Superscriptions: map[string]map[int]bibletext.Superscription{
			"Psalms": {3: {Text: "A Psalm by David, when he fled from Absalom his son."}},
		},
	}
	return bd, verses
}

func TestAPsalmPageCarriesItsTitle(t *testing.T) {
	bd, verses := psalmWithTitle()
	got := chapterBody(bd, "web", "Psalms", 3, verses)

	if !strings.Contains(got, `<p class="pst">A Psalm by David, when he fled from Absalom his son.</p>`) {
		t.Errorf("the psalm's title is not on the page:\n%s", got)
	}
	// It must stand ABOVE the scripture, which is where every app pane sets it.
	if strings.Index(got, `class="pst"`) > strings.Index(got, "adversaries") {
		t.Errorf("the title is set below the verses:\n%s", got)
	}
	// And it must not have entered a verse.
	if strings.Contains(got, "Absalom his son.</span> Yahweh") {
		t.Errorf("the title was folded into verse 1:\n%s", got)
	}
}

// CONTROL, and the property the existing chapterBody golden depends on: a
// chapter with no title renders exactly as before, and a nil BibleData — which
// nine render tests and the tint golden pass — must not panic or emit anything.
func TestAChapterWithNoTitleIsUnchanged(t *testing.T) {
	_, verses := psalmWithTitle()

	withNil := chapterBody(nil, "web", "Psalms", 3, verses)
	if strings.Contains(withNil, "pst") {
		t.Errorf("a nil BibleData produced a title line:\n%s", withNil)
	}

	bare := &bibletext.BibleData{
		Verses: map[string]map[int][]bibletext.Verse{"Psalms": {3: verses}},
	}
	if got := chapterBody(bare, "web", "Psalms", 3, verses); got != withNil {
		t.Errorf("a chapter with no title differs from the nil-data render:\n got %s\nwant %s",
			got, withNil)
	}
	if !strings.Contains(withNil, "adversaries") {
		t.Fatal("the fixture rendered no scripture; this test proves nothing")
	}
}

// Psalm 119 is the case the acrostic lift created: no superscription at all,
// and 22 acrostic headings instead. It must draw the letters and no title.
func TestPsalm119DrawsItsAcrosticLettersAndNoTitle(t *testing.T) {
	verses := []bibletext.Verse{
		{BookName: "Psalms", Book: "Psalms", Chapter: 119, Verse: 1, Text: "Blessed are those whose ways are blameless."},
		{BookName: "Psalms", Book: "Psalms", Chapter: 119, Verse: 9, Text: "How can a young man keep his way pure?"},
	}
	bd := &bibletext.BibleData{
		Verses: map[string]map[int][]bibletext.Verse{"Psalms": {119: verses}},
		Headings: map[string]map[int][]bibletext.Heading{
			"Psalms": {119: {
				{Text: "ALEPH", Style: "acrostic", BeforeVerse: 1},
				{Text: "BETH", Style: "acrostic", BeforeVerse: 9},
			}},
		},
	}
	got := chapterBody(bd, "web", "Psalms", 119, verses)
	if strings.Contains(got, "pst") {
		t.Errorf("Psalm 119 drew a title it does not have:\n%s", got)
	}
	for _, letter := range []string{"ALEPH", "BETH"} {
		if !strings.Contains(got, letter) {
			t.Errorf("the acrostic letter %s is missing from the page:\n%s", letter, got)
		}
	}
}

// The title is escaped like every other piece of text on the page.
func TestATitleIsEscaped(t *testing.T) {
	verses := []bibletext.Verse{{BookName: "Psalms", Book: "Psalms", Chapter: 7, Verse: 1, Text: "Yahweh my God."}}
	bd := &bibletext.BibleData{
		Verses: map[string]map[int][]bibletext.Verse{"Psalms": {7: verses}},
		Superscriptions: map[string]map[int]bibletext.Superscription{
			"Psalms": {7: {Text: `A <script>alert(1)</script> & "shiggaion"`}},
		},
	}
	got := chapterBody(bd, "web", "Psalms", 7, verses)
	if strings.Contains(got, "<script>") {
		t.Errorf("the title was not escaped:\n%s", got)
	}
	if !strings.Contains(got, "&lt;script&gt;") || !strings.Contains(got, "&amp;") {
		t.Errorf("the title is not escaped as expected:\n%s", got)
	}
}

// THE TITLE IS RAGGED AND WHOLE on the web: ragged as the Apple panes
// (reading.go, p.pst) and the Windows and Linux pane (reading_styled_super.go)
// set it, and unhyphenated as the Windows and Linux pane sets it. The title is
// a <p> inside .text, and .text p justifies and hyphenates, so a title with no
// rules of its own took both: one long enough to wrap, such as Psalm 18's, was
// spread to the measure and broken mid-word. Android from API 35 justifies a
// wrapped title's lines, and the Apple panes and Android hyphenate it;
// docs/READING_TYPOGRAPHY.md records both as known differences.
// Mutations: text-align:left, hyphens:none or -webkit-hyphens:none taken out
// of .text p.pst.
func TestAPsalmTitleIsSetRaggedAndUnhyphenated(t *testing.T) {
	css := testCSS()
	// The control: the prose rule still justifies and hyphenates under both
	// names, so a title rule that said nothing would do both too.
	prose := cssDecls(cssRule(t, css, ".text p{"))
	for _, d := range [][2]string{{"text-align", "justify"}, {"hyphens", "auto"}, {"-webkit-hyphens", "auto"}} {
		if prose[d[0]] != d[1] {
			t.Fatalf("the prose paragraph's %s is %q, not %q, so this test no longer shows "+
				"what the title would inherit", d[0], prose[d[0]], d[1])
		}
	}
	title := cssDecls(cssRule(t, css, ".text p.pst{"))
	for _, d := range [][2]string{{"text-align", "left"}, {"hyphens", "none"}, {"-webkit-hyphens", "none"}} {
		if title[d[0]] != d[1] {
			t.Errorf("the Psalm title's %s is %q, not %q: it would take the prose "+
				"paragraph's %q", d[0], title[d[0]], d[1], prose[d[0]])
		}
	}
}

// cssDecls reads a rule's declarations, property to value, so a check on
// hyphens cannot be met by -webkit-hyphens.
func cssDecls(rule string) map[string]string {
	decls := map[string]string{}
	_, body, _ := strings.Cut(rule, "{")
	for _, d := range strings.Split(body, ";") {
		if prop, val, ok := strings.Cut(d, ":"); ok {
			decls[strings.TrimSpace(prop)] = strings.TrimSpace(val)
		}
	}
	return decls
}
