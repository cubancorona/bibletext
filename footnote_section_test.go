package bibletext

// The chapter-bottom footnote section's contract (footnote_section.go):
//
//  1. OFF is a no-op, byte for byte — a footnotes-off chapter's HTML is
//     identical whether or not the verses carry Footnotes data.
//  2. ON leaves scripture untouched: everything before the separator is
//     byte-identical to the off rendering's body, and the section sits
//     strictly after the last verse paragraph.
//  3. The section carries translator footnotes only (no crossrefs, no blank
//     bodies), keyed by verse in order, escaped, with no <sup> and no links —
//     and every run at 0.85em, the size band the native panes key on.
//  4. The toggle moves BOTH fingerprints (render + body), or the native panes
//     would skip the repaint.
//  5. The native boundary detectors this size band pacts with actually exist
//     in the Apple files (a drift alarm, in the release-scripts-test mould).

import (
	"os"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
)

func footnoteFixtureVerses() []Verse {
	return []Verse{
		{BookName: "John", Book: "John", Chapter: 3, Verse: 16,
			Text: "For God so loved the world.",
			Footnotes: []Footnote{
				{Anchor: 4, Text: "Or God loved the world in this way.", Caller: "+"},
				{Anchor: 9, Text: `A <second> & "quoted" note.`, Caller: "+"},
			}},
		{BookName: "John", Book: "John", Chapter: 3, Verse: 17,
			Text: "God did not send his Son to condemn the world.",
			Footnotes: []Footnote{
				{Anchor: 0, Text: "John 7:50; 19:39", Kind: footnoteKindCrossref},
				{Anchor: 0, Text: "   "},
			}},
		{BookName: "John", Book: "John", Chapter: 3, Verse: 18,
			Text: "Whoever believes in him is not condemned."},
	}
}

func footnoteFixtureState(verses []Verse) *AppState {
	bd := &BibleData{
		Books:  []string{"John"},
		Verses: map[string]map[int][]Verse{"John": {3: verses}},
	}
	return &AppState{Bible: bd, CurrentBook: "John", CurrentChapter: 3, CurrentVersion: "bsb"}
}

// stripFootnotes is the fixture's control twin: same verses, no side-band.
func stripFootnotes(verses []Verse) []Verse {
	out := make([]Verse, len(verses))
	for i, v := range verses {
		v.Footnotes = nil
		out[i] = v
	}
	return out
}

func TestChapterFootnoteEntries(t *testing.T) {
	entries := chapterFootnoteEntries(footnoteFixtureVerses())
	if len(entries) != 2 {
		t.Fatalf("want the 2 translator notes (crossref + blank excluded), got %d: %v", len(entries), entries)
	}
	if entries[0].Verse != 16 || entries[1].Verse != 16 {
		t.Errorf("entries must be keyed by their verse in order: %v", entries)
	}
	if entries[0].Text != "Or God loved the world in this way." {
		t.Errorf("first entry wrong: %q", entries[0].Text)
	}
	for _, e := range entries {
		if strings.Contains(e.Text, "7:50") {
			t.Errorf("crossref leaked into the section entries: %q", e.Text)
		}
	}
}

func TestFootnoteSectionOffIsByteIdentical(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	// Explicitly off (the default) — and the sharper case: off after having
	// been on, so the stored preference is exercised, not just the fallback.
	setFootnotesEnabled(true)
	setFootnotesEnabled(false)

	verses := footnoteFixtureVerses()
	st := footnoteFixtureState(verses)
	stPlain := footnoteFixtureState(stripFootnotes(verses))
	withReporterLayout(false, func() {
		if got, want := buildChapterHTML(st, verses), buildChapterHTML(stPlain, stripFootnotes(verses)); got != want {
			t.Errorf("footnotes OFF must render byte-identically to footnote-free verses:\n got: %s\nwant: %s", got, want)
		}
	})
}

func TestFootnoteSectionOnRendersAtChapterBottom(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setFootnotesEnabled(true)

	verses := footnoteFixtureVerses()
	st := footnoteFixtureState(verses)
	var onHTML, offHTML string
	withReporterLayout(false, func() {
		onHTML = buildChapterHTML(st, verses)
		setFootnotesEnabled(false)
		offHTML = buildChapterHTML(st, verses)
		setFootnotesEnabled(true)
	})

	// Exactly one separator, after the last verse paragraph.
	if n := strings.Count(onHTML, `<p class="fnsep">`); n != 1 {
		t.Fatalf("want exactly one separator paragraph, got %d:\n%s", n, onHTML)
	}
	if !strings.Contains(onHTML, `<p class="fnsep">`+footnoteSeparator+`</p>`) {
		t.Errorf("phone layout: separator must be the bare rule (no <br> gap):\n%s", onHTML)
	}
	sepAt := strings.Index(onHTML, `<p class="fnsep">`)
	if last := strings.LastIndex(onHTML[:sepAt], "</p>"); last == -1 || strings.Contains(onHTML[last:sepAt], "<p") && !strings.HasPrefix(onHTML[strings.Index(onHTML[last:sepAt], "<p")+last:], `<p class="fnsep">`) {
		t.Errorf("section must start strictly after the last verse paragraph:\n%s", onHTML)
	}

	// Scripture is untouched: the rendered BODY before the separator is
	// byte-identical to the off rendering's body (the head differs only by
	// the section's own stylesheet rules).
	marker := "</head><body>"
	onBody := onHTML[strings.Index(onHTML, marker)+len(marker):]
	offBody := offHTML[strings.Index(offHTML, marker)+len(marker):]
	if got, want := onBody[:strings.Index(onBody, `<p class="fnsep">`)], strings.TrimSuffix(offBody, "</body></html>"); got != want {
		t.Errorf("scripture body must be byte-identical with the section on:\n got: %q\nwant: %q", got, want)
	}

	section := onHTML[sepAt:]
	// Verse-keyed entries, in order, escaped.
	if !strings.Contains(section, `<p class="fn"><span class="fnv">16</span>&nbsp;Or God loved the world in this way.</p>`) {
		t.Errorf("first entry missing or malformed:\n%s", section)
	}
	if !strings.Contains(section, `<span class="fnv">16</span>&nbsp;A &lt;second&gt; &amp; &quot;quoted&quot; note.`) {
		t.Errorf("entry text must be HTML-escaped:\n%s", section)
	}
	if strings.Contains(section, "7:50") || strings.Contains(section, `<span class="fnv">17</span>`) {
		t.Errorf("crossrefs and blank bodies must stay out of the section:\n%s", section)
	}
	// The native pact: nothing in the section may register with the Apple
	// verse scans (no <sup>), nothing is a link, and the whole section renders
	// in the 0.85em band the content-end detectors key on.
	if strings.Contains(section, "<sup") || strings.Contains(section, "<a ") {
		t.Errorf("the section must contain no <sup> and no links:\n%s", section)
	}
	if strings.Count(onHTML, "font-size: 0.85em") != 2 {
		t.Errorf("both section classes must set the 0.85em pact size:\n%s", onHTML)
	}
	if !strings.Contains(offHTML, marker) || strings.Contains(offHTML, "fnsep") {
		t.Errorf("off rendering must carry no section CSS:\n%s", offHTML)
	}
}

func TestFootnoteSectionReporterGap(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setFootnotesEnabled(true)

	verses := footnoteFixtureVerses()
	st := footnoteFixtureState(verses)
	withReporterLayout(true, func() {
		html := buildChapterHTML(st, verses)
		// Reporter paragraphs carry no bottom margin and the importer zeroes
		// margin-top, so the air above the rule is a blank line inside the
		// separator paragraph.
		if !strings.Contains(html, `<p class="fnsep"><br>`+footnoteSeparator+`</p>`) {
			t.Errorf("reporter layout: separator must open with a blank line:\n%s", html)
		}
	})
}

func TestFootnoteSectionAbsentWhenChapterHasNone(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	verses := stripFootnotes(footnoteFixtureVerses())
	st := footnoteFixtureState(verses)
	var on, off string
	withReporterLayout(false, func() {
		setFootnotesEnabled(true)
		on = buildChapterHTML(st, verses)
		setFootnotesEnabled(false)
		off = buildChapterHTML(st, verses)
	})
	if on != off {
		t.Errorf("a chapter with no notes must render identically with the toggle on:\n on: %s\noff: %s", on, off)
	}
	if strings.Contains(on, "fnsep") {
		t.Errorf("no-notes chapter must carry no separator or section CSS:\n%s", on)
	}
}

func TestChapterHasFootnotes(t *testing.T) {
	if !chapterHasFootnotes(footnoteFixtureState(footnoteFixtureVerses())) {
		t.Error("fixture chapter has translator notes; gate must say so")
	}
	// A chapter whose only apparatus is crossrefs shows no toggle — the
	// section would be empty.
	crossOnly := []Verse{{BookName: "John", Book: "John", Chapter: 3, Verse: 1,
		Text:      "t",
		Footnotes: []Footnote{{Text: "John 7:50", Kind: footnoteKindCrossref}}}}
	if chapterHasFootnotes(footnoteFixtureState(crossOnly)) {
		t.Error("crossref-only chapter must not offer the toggle")
	}
	if chapterHasFootnotes(footnoteFixtureState(stripFootnotes(footnoteFixtureVerses()))) {
		t.Error("note-free chapter must not offer the toggle")
	}
	if chapterHasFootnotes(nil) || chapterHasFootnotes(&AppState{}) {
		t.Error("nil-safety: no state / no data must gate to false")
	}
}

// TestFootnoteSectionNativeContract is the drift alarm for the cross-language
// pact (the scripts-are-asserted-not-trusted mould of dev_links_guard_test.go):
// the Go side renders the whole section at 0.85em; the Apple files must keep
// the content-end detectors that find the apparatus boundary as the first run
// in the [0.8, 0.95) font band, and the verbs that clamp to it. Delete or
// rename either half and this fails before a simulator ever shows the drift.
func TestFootnoteSectionNativeContract(t *testing.T) {
	var css strings.Builder
	writeFootnoteCSS(&css, "#000000")
	if strings.Count(css.String(), "font-size: 0.85em") != 2 {
		t.Fatalf("writeFootnoteCSS must set every section class to 0.85em:\n%s", css.String())
	}

	for file, wants := range map[string][]string{
		"reading_ios.go": {
			"btIOSFindContentEnd",
			"btIOSContentEnd(ts)",
			"maxSize * 0.95",
			// The plain-text import-failure fallback must cut the section
			// out of the HTML before stripping tags — the geometry boundary
			// cannot exist in untyped text, so the clamps would silently
			// disarm over apparatus the fallback string still carried.
			"fnsep",
		},
		"reading_macos.go": {
			"btMacFindContentEnd",
			"btMacContentEnd(ts)",
			"maxSize * 0.95",
			"hbScriptureSelection",
			"fnsep",
		},
	} {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		for _, want := range wants {
			if !strings.Contains(string(src), want) {
				t.Errorf("%s: missing %q — the native content-end pact with the 0.85em section is broken", file, want)
			}
		}
	}
}

// Poetry keeps its shape with the section on: the poem's <br> structure and
// ragged-right class are untouched, and the section trails the poem.
func TestBuildChapterHTMLPoetryWithFootnotes(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setFootnotesEnabled(true)

	st := psalm23State()
	verses := st.Bible.GetChapter("Psalms", 23)
	verses[0].Footnotes = []Footnote{{Anchor: 24, Text: "Hebrew waters of rest.", Caller: "+"}}

	var html string
	withReporterLayout(false, func() { html = buildChapterHTML(st, verses) })
	if !strings.Contains(html, "The LORD is my shepherd;<br>I shall not want.") {
		t.Errorf("poem line structure must survive the section:\n%s", html)
	}
	if !strings.Contains(html, `<p class="pm">`) {
		t.Errorf("poetic paragraph class must survive the section:\n%s", html)
	}
	sepAt := strings.Index(html, `<p class="fnsep">`)
	if sepAt == -1 || sepAt < strings.LastIndex(html, `<p class="pm">`) {
		t.Errorf("the section must trail the poem:\n%s", html)
	}
	if !strings.Contains(html[sepAt:], `<span class="fnv">1</span>&nbsp;Hebrew waters of rest.`) {
		t.Errorf("poem verse's note must key by its verse number:\n%s", html)
	}
}
