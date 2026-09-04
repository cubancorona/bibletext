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
	entries := chapterFootnoteEntries(footnoteFixtureVerses(), nil, nil)
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

// An orphan (a note on an omitted verse) sorts into its natural place
// between its neighbours' notes, keyed by the verse number the page lacks.
func TestChapterFootnoteEntriesInterleavesOrphans(t *testing.T) {
	verses := footnoteFixtureVerses() // notes on 16 (x2), crossref-only 17, plain 18
	orphans := []OrphanFootnote{
		{Verse: 17, Text: "Some copies omit verse 17.", Caller: "+"},
		{Verse: 17, Text: "A crossref orphan stays dark.", Kind: footnoteKindCrossref},
		{Verse: 17, Text: "   "},
	}
	entries := chapterFootnoteEntries(verses, orphans, nil)
	if len(entries) != 3 {
		t.Fatalf("want 2 verse notes + 1 orphan, got %d: %v", len(entries), entries)
	}
	if entries[0].Verse != 16 || entries[1].Verse != 16 || entries[2].Verse != 17 {
		t.Errorf("orphan must sort between its neighbours by verse: %v", entries)
	}
	if entries[2].Text != "Some copies omit verse 17." {
		t.Errorf("orphan entry wrong (crossref/blank orphans must be excluded): %q", entries[2].Text)
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

// The Android dialect: byte-identical with the toggle off; on, the section
// opens with EXACTLY ONE sentinel <sup> (a no-break space — BtBridge's
// buildVerseIndex ends the last verse at it and skips it), keys are plain
// <b> text (a digit-leading sup would index as a phantom verse), and the
// separator is the same underline-over-nbsp hairline the Apple panes draw.
func TestFootnoteSectionAndroidDialect(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	verses := footnoteFixtureVerses()
	st := footnoteFixtureState(verses)
	st.Bible.OrphanFootnotes = map[string]map[int][]OrphanFootnote{
		"John": {3: {{Verse: 17, Text: "Some copies omit verse 17.", Caller: "+"}}},
	}

	setFootnotesEnabled(false)
	off := buildChapterHTMLAndroid(st, verses)
	plain := footnoteFixtureState(stripFootnotes(verses))
	if off != buildChapterHTMLAndroid(plain, stripFootnotes(verses)) {
		t.Error("footnotes OFF must render the Android dialect byte-identically to footnote-free verses")
	}

	setFootnotesEnabled(true)
	defer setFootnotesEnabled(false)
	on := buildChapterHTMLAndroid(st, verses)
	if !strings.HasPrefix(on, off[:strings.LastIndex(off, "</p>")]) {
		t.Error("scripture half of the Android dialect must be unchanged by the section")
	}
	section := on[len(off):]
	if n := strings.Count(section, `<sup>&#160;</sup>`); n != 1 {
		t.Fatalf("the section must open with exactly one sentinel sup, got %d:\n%s", n, section)
	}
	if n := strings.Count(section, "<sup"); n != 1 {
		t.Errorf("no sup but the sentinel may appear in the section (phantom-verse hazard), got %d:\n%s", n, section)
	}
	sentinelAt := strings.Index(section, `<sup>&#160;</sup>`)
	sepAt := strings.Index(section, footnoteSeparator)
	if sepAt == -1 || sepAt < sentinelAt {
		t.Errorf("the sentinel must precede the separator:\n%s", section)
	}
	for _, want := range []string{
		`<b>16</b>&#160;Or God loved the world in this way.`,
		`<b>17</b>&#160;Some copies omit verse 17.`,
		`<b>16</b>&#160;A &lt;second&gt; &amp; &quot;quoted&quot; note.`,
	} {
		if !strings.Contains(section, want) {
			t.Errorf("missing entry %q in:\n%s", want, section)
		}
	}
	if idx16 := strings.Index(section, "<b>16</b>"); idx16 > strings.Index(section, "<b>17</b>") {
		t.Errorf("entries must be in verse order (orphan 17 after 16):\n%s", section)
	}
	if strings.Contains(section, "7:50") {
		t.Errorf("crossrefs must stay out of the Android section:\n%s", section)
	}
}

// The Apple builder renders an orphan's row keyed by the verse the page
// lacks, sorted into place between its neighbours.
func TestFootnoteSectionRendersOrphans(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setFootnotesEnabled(true)
	defer setFootnotesEnabled(false)

	verses := footnoteFixtureVerses()
	st := footnoteFixtureState(verses)
	st.Bible.OrphanFootnotes = map[string]map[int][]OrphanFootnote{
		"John": {3: {{Verse: 17, Text: "Some copies omit verse 17.", Caller: "+"}}},
	}
	var html string
	withReporterLayout(false, func() { html = buildChapterHTML(st, verses) })
	want := `<p class="fn"><span class="fnv">17</span>&nbsp;Some copies omit verse 17.</p>`
	if !strings.Contains(html, want) {
		t.Fatalf("orphan row missing:\n%s", html)
	}
	if strings.Index(html, `<span class="fnv">16</span>`) > strings.Index(html, `<span class="fnv">17</span>`) {
		t.Errorf("orphan must sort after verse 16's notes:\n%s", html)
	}
	// (That the omitted verse itself never enters the TEXT is the decode
	// contract — TestFootnotesOmittedVerseNoteBecomesOrphan.)
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
			"btIOSContentStart",
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
			"btMacFindContentStart",
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

// --- superscription rendering -------------------------------------------------

func superFixtureState(verses []Verse) *AppState {
	st := footnoteFixtureState(verses)
	st.Bible.Superscriptions = map[string]map[int]Superscription{
		"John": {3: {Text: "A test title, according to Gittith.",
			Footnotes: []Footnote{{Anchor: 12, Text: "Gittith is probably a musical term.", Caller: "+"}}}},
	}
	return st
}

// The title renders as an italic unnumbered line BEFORE the first verse,
// regardless of the footnotes toggle; its note joins the section keyed
// "Title", ahead of the verse-keyed notes; a chapter without a title is
// byte-identical to the pre-title output.
func TestSuperscriptionRendersAboveTheChapter(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	verses := footnoteFixtureVerses()
	st := superFixtureState(verses)

	setFootnotesEnabled(false)
	var offHTML string
	withReporterLayout(false, func() { offHTML = buildChapterHTML(st, verses) })
	title := `<p class="pst">A test title, according to Gittith.</p>`
	if !strings.Contains(offHTML, title) {
		t.Fatalf("title must render with footnotes OFF:\n%s", offHTML)
	}
	if at := strings.Index(offHTML, title); at > strings.Index(offHTML, `<sup class="v">16</sup>`) {
		t.Error("title must precede the first verse")
	}
	if strings.Contains(offHTML, `class="fn"`) {
		t.Error("footnotes OFF must render no section even with a title note present")
	}

	setFootnotesEnabled(true)
	defer setFootnotesEnabled(false)
	var onHTML string
	withReporterLayout(false, func() { onHTML = buildChapterHTML(st, verses) })
	if !strings.Contains(onHTML, `<span class="fnv">Title</span>&nbsp;Gittith is probably a musical term.`) {
		t.Fatalf("title note must key as Title:\n%s", onHTML)
	}
	if strings.Index(onHTML, `<span class="fnv">Title</span>`) > strings.Index(onHTML, `<span class="fnv">16</span>`) {
		t.Error("the Title entry must precede the verse-keyed entries")
	}

	// No title, no trace: byte-identical to a state without the table.
	plain := footnoteFixtureState(verses)
	setFootnotesEnabled(false)
	var a, b string
	withReporterLayout(false, func() {
		a = buildChapterHTML(plain, verses)
		b = buildChapterHTML(footnoteFixtureState(verses), verses)
	})
	if a != b || strings.Contains(a, "pst") {
		t.Error("a chapter without a superscription must carry no title markup or CSS")
	}
}

func TestSuperscriptionAndroidDialect(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setFootnotesEnabled(true)
	defer setFootnotesEnabled(false)

	verses := footnoteFixtureVerses()
	st := superFixtureState(verses)

	// PIN the page. The Android dialect took the reporter page too
	// (reporter_android.go), and the seam's host default answers for the HOST —
	// true on darwin, false on Linux — so an unpinned assertion about this
	// dialect's structure would pass here and fail on CI.
	for _, page := range []struct {
		name     string
		reporter bool
		want     string
	}{
		// The reporter page has no blank line between blocks (imported
		// COMPACT), so the air under the title is an empty line inside the
		// title's own block — the Apple page's p.pst margin, in this dialect.
		{"phone page", false, `<p><i>A test title, according to Gittith.</i></p>`},
		{"reporter page", true, `<p><i>A test title, according to Gittith.</i><br></p>`},
	} {
		var html string
		withReporterLayout(page.reporter, func() { html = buildChapterHTMLAndroid(st, verses) })
		if !strings.HasPrefix(html, page.want) {
			t.Fatalf("%s: Android title must lead the chapter, italic and sup-free:\n%.200s", page.name, html)
		}
		if strings.Contains(strings.Split(html, "</i>")[0], "<sup") {
			t.Errorf("%s: the title must contain no <sup> (verse-index hazard)", page.name)
		}
		if !strings.Contains(html, `<b>Title</b>&#160;Gittith is probably a musical term.`) {
			t.Errorf("%s: Android title note must key as Title:\n%s", page.name, html)
		}
	}
}
