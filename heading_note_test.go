package bibletext

// A note inside a publisher's heading is set as the publisher sets it: the
// heading's words as the heading, the note as that heading's footnote in the
// chapter's footnote section (heading_note.go). The one case in the three
// helloao editions is the WEB Catholic's Greek Daniel 3, where the feed runs
// the note into the heading's words; the fixtures below are that chapter's
// public-domain text and the feed's own string for the heading.

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"fyne.io/fyne/v2/test"
)

const (
	daniel3Title = "THE SONG OF THE THREE HOLY CHILDREN"
	daniel3Note  = "The Song of the Three Holy Children is an addition to Daniel found in the Greek Septuagint " +
		"but not found in the traditional Hebrew text of Daniel. This portion is recognized as Deuterocanonical " +
		"Scripture by the Roman Catholic, Greek Orthodox, and Russian Orthodox Churches. It is found inserted " +
		"between Daniel 3:23 and Daniel 3:24 of the traditional Hebrew Bible. Here, the verses after 23 from the " +
		"Hebrew Bible are numbered starting at 91 to make room for these verses."
	// The heading as the WEB Catholic's helloao feed sends it, and as every
	// cache of that edition holds it: the note's reference and words run on
	// from the heading's last letter.
	daniel3Fused = daniel3Title + "3:24 " + daniel3Note
	daniel3V23   = "These three men, Shadrach, Meshach, and Abednego, fell down bound into the middle of the burning fiery furnace."
	daniel3V24   = "They walked in the midst of the fire, praising God, and blessing the Lord."
	daniel3V25   = "Then Azarias stood, and prayed like this. Opening his mouth in the midst of the fire he said,"
)

// daniel3Decoded runs the feed's own nodes for Daniel 3:23-25 through the
// helloao decoder, as a fresh download does.
func daniel3Decoded(t *testing.T) *BibleData {
	t.Helper()
	book := helloAOBook{ID: "DAG", Order: 27}
	book.Chapters = []helloAOChapterEntry{{}}
	book.Chapters[0].Chapter.Number = 3
	for _, raw := range []string{
		`{"type":"verse","number":23,"content":[` + jsonString(t, daniel3V23) + `]}`,
		`{"type":"heading","content":[` + jsonString(t, daniel3Fused) + `]}`,
		`{"type":"verse","number":24,"content":[` + jsonString(t, daniel3V24) + `]}`,
		`{"type":"verse","number":25,"content":[` + jsonString(t, daniel3V25) + `]}`,
	} {
		book.Chapters[0].Chapter.Content = append(book.Chapters[0].Chapter.Content, json.RawMessage(raw))
	}
	chapters, _, _, headings := decodeHelloAOChapters("Daniel", book, nil)
	return &BibleData{
		Books:    []string{"Daniel"},
		Verses:   map[string]map[int][]Verse{"Daniel": chapters},
		Headings: map[string]map[int][]Heading{"Daniel": headings},
	}
}

func jsonString(t *testing.T, s string) string {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// daniel3Drawn checks what every surface reads for the chapter: the heading
// blocks, and the footnote section's rows.
func daniel3Drawn(t *testing.T, bd *BibleData) {
	t.Helper()
	verses := bd.GetChapter("Daniel", 3)
	var heads []string
	for _, b := range ChapterBlocks(bd, "Daniel", 3, verses) {
		if b.HeadingText != "" {
			heads = append(heads, b.HeadingText)
		}
	}
	if len(heads) != 1 || heads[0] != daniel3Title {
		t.Errorf("Daniel 3 draws the heading %q, want the publisher's %d-character title %q",
			heads, utf8.RuneCountInString(daniel3Title), daniel3Title)
	}
	entries := ChapterFootnoteEntries(bd, "Daniel", 3, verses)
	if len(entries) != 1 || entries[0].Key != "24" || entries[0].Text != daniel3Note {
		t.Errorf("the footnote section is %+v, want the heading's note filed under 24 (the publisher's 3:24)", entries)
	}
}

func TestTheSongOfTheThreeIsDrawnAsItsPublisherSetsIt(t *testing.T) {
	bd := daniel3Decoded(t)
	if got := bd.Headings["Daniel"][3]; len(got) != 1 || got[0].BeforeVerse != 24 {
		t.Fatalf("the decoder placed the heading as %+v, want one heading above verse 24", got)
	}
	if utf8.RuneCountInString(daniel3Title) != 35 || utf8.RuneCountInString(daniel3Fused) != 510 {
		t.Fatalf("fixture: title %d and fused heading %d characters, want 35 and 510",
			utf8.RuneCountInString(daniel3Title), utf8.RuneCountInString(daniel3Fused))
	}
	daniel3Drawn(t, bd)
	// The text is untouched: the note is apparatus, never a verse's words.
	for _, v := range bd.GetChapter("Daniel", 3) {
		if strings.Contains(v.Text, "Septuagint") || strings.Contains(v.Text, daniel3Title) {
			t.Errorf("verse %d carries heading matter: %q", v.Verse, v.Text)
		}
	}
}

// A WEB Catholic cache written before this change holds the fused string, and
// it draws correctly as it stands: nobody downloads the translation again.
// The envelope is the on-disk form, so this is the file a device already has.
func TestACacheWrittenBeforeTheHeadingNoteWasSetApartDrawsIt(t *testing.T) {
	data := fmt.Sprintf(`{"version":%d,"saved_at":"2026-09-11T12:12:27Z","data":{`+
		`"Books":["Daniel"],"Verses":{"Daniel":{"3":[`+
		`{"BookName":"Daniel","Book":"Daniel","Chapter":3,"Verse":23,"Text":%s},`+
		`{"BookName":"Daniel","Book":"Daniel","Chapter":3,"Verse":24,"Text":%s},`+
		`{"BookName":"Daniel","Book":"Daniel","Chapter":3,"Verse":25,"Text":%s}]}},`+
		`"headings":{"Daniel":{"3":[{"text":%s,"style":"heading","before_verse":24}]}}}}`,
		cacheSchemaVersion, jsonString(t, daniel3V23), jsonString(t, daniel3V24), jsonString(t, daniel3V25),
		jsonString(t, daniel3Fused))
	path := filepath.Join(t.TempDir(), "bibletext-webc-v9.json")
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	bd, err := loadBibleFromCache(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := bd.GetChapter("Daniel", 3); len(got) != 3 || got[1].Text != daniel3V24 {
		t.Fatalf("fixture: the cache's verses did not load: %+v", got)
	}
	daniel3Drawn(t, bd)
	// Drawing leaves the stored heading as the feed sent it.
	if got := bd.Headings["Daniel"][3][0]; got.Text != daniel3Fused || len(got.Footnotes) != 0 {
		t.Errorf("drawing changed the stored heading: %+v", got)
	}
}

// Every surface's builder: the iPhone, iPad and Mac pages (one builder), the
// Android page and the Windows and Linux pane. With the footnotes on, the
// heading is the title alone and the note is a row of the section under 24;
// with them off, the note is nowhere, as any other note is.
func TestEverySurfaceSetsTheHeadingNoteInTheFootnoteSection(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer setFootnotesEnabled(false)

	bd := daniel3Decoded(t)
	st := &AppState{Bible: bd, CurrentBook: "Daniel", CurrentChapter: 3, CurrentVersion: "webc"}
	verses := bd.GetChapter("Daniel", 3)
	notePrefix := "The Song of the Three Holy Children is an addition"

	for _, on := range []bool{true, false} {
		setFootnotesEnabled(on)
		var apple string
		withReporterLayout(false, func() { apple = buildChapterHTML(st, verses) })
		if !strings.Contains(apple, `<p class="sec">`+daniel3Title+`</p>`) {
			t.Errorf("footnotes %v: the Apple page does not set the title alone as the heading:\n%s", on, apple)
		}
		android := buildChapterHTMLAndroid(st, verses)
		if !strings.Contains(android, daniel3Title+"</b></p>") || strings.Contains(android, daniel3Title+"3:24") {
			t.Errorf("footnotes %v: the Android page does not set the title alone as the heading:\n%s", on, android)
		}
		pane := newTestPane(t, st, 420)
		var paneHeading []string
		for _, ln := range pane.lay.Lines {
			if ln.Heading != "" {
				paneHeading = append(paneHeading, ln.Heading)
			}
		}
		if got := strings.Join(paneHeading, " "); got != daniel3Title {
			t.Errorf("footnotes %v: the Windows and Linux pane sets the heading %q", on, got)
		}

		if !on {
			for name, page := range map[string]string{"Apple": apple, "Android": android} {
				if strings.Contains(page, notePrefix) {
					t.Errorf("footnotes off: the %s page shows the heading's note:\n%s", name, page)
				}
			}
			if pane.fnGeom.present || len(pane.fnEntries) != 0 {
				t.Error("footnotes off: the Windows and Linux pane grew a footnote section")
			}
			continue
		}
		if !strings.Contains(apple, `<span class="fnv">24</span>&nbsp;`+notePrefix) {
			t.Errorf("footnotes on: the Apple section has no row 24 with the heading's note:\n%s", apple)
		}
		if sep := strings.Index(apple, `<p class="fnsep">`); sep < 0 || strings.Index(apple, notePrefix) < sep {
			t.Errorf("footnotes on: the heading's note must stand in the section, below the text:\n%s", apple)
		}
		if !strings.Contains(android, `<b>24</b>&#160;`+notePrefix) {
			t.Errorf("footnotes on: the Android section has no row 24 with the heading's note:\n%s", android)
		}
		if len(pane.fnEntries) != 1 || pane.fnEntries[0].Verse != 24 || pane.fnEntries[0].Text != daniel3Note {
			t.Errorf("footnotes on: the Windows and Linux section is %+v, want the note under 24", pane.fnEntries)
		}
		if !pane.fnGeom.present {
			t.Error("footnotes on: the Windows and Linux pane drew no section")
		}
	}
}

// A heading's note takes the verse it stands above as its key and stands
// ahead of that verse's own notes, as the heading stands ahead of the verse.
// A heading the page does not draw contributes nothing.
func TestAHeadingsNoteIsFiledUnderTheVerseBelowAheadOfItsOwn(t *testing.T) {
	verses := []Verse{
		{Verse: 1, Text: "a", Footnotes: []Footnote{{Text: "verse one's note"}}},
		{Verse: 2, Text: "b", Footnotes: []Footnote{{Text: "verse two's note"}}},
	}
	heads := []Heading{
		{Text: "Above two", Style: feedHeadingStyle, BeforeVerse: 2, Footnotes: []Footnote{
			{Text: "the heading's note"},
			{Text: "Gen 1:1", Kind: footnoteKindCrossref},
			{Text: "  "},
		}},
		{Text: "Above a verse the page lacks", Style: feedHeadingStyle, BeforeVerse: 9, Footnotes: []Footnote{{Text: "never drawn"}}},
		{Text: "", Style: feedHeadingStyle, BeforeVerse: 1, Footnotes: []Footnote{{Text: "an empty heading is not drawn"}}},
	}
	got := chapterFootnoteEntries(verses, nil, nil, heads)
	want := []footnoteEntry{
		{Verse: 1, Text: "verse one's note"},
		{Verse: 2, Text: "the heading's note"},
		{Verse: 2, Text: "verse two's note"},
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("entries = %+v\nwant      %+v", got, want)
	}
	st := &AppState{Bible: &BibleData{
		Books:    []string{"Daniel"},
		Verses:   map[string]map[int][]Verse{"Daniel": {3: {{Verse: 24, Text: daniel3V24}}}},
		Headings: map[string]map[int][]Heading{"Daniel": {3: {{Text: daniel3Fused, Style: "heading", BeforeVerse: 24}}}},
	}, CurrentBook: "Daniel", CurrentChapter: 3}
	if !chapterHasFootnotes(st) {
		t.Error("a chapter whose only note is a heading's must report a section")
	}
}

// The notes the API.Bible decoder keeps on an NKJV heading (its USX styles,
// "s1", "r" and the rest) are not listed: only a helloao heading's notes are,
// the ones the feed ran into the heading's words. How many of the NKJV's are
// not cross-references has not been measured, and the display of its
// apparatus is an open licensing question, so the section, the gate on its
// control and the website's notes all leave them out, as they did before any
// heading's note was listed.
func TestAnNKJVHeadingsKeptNotesAreNotListed(t *testing.T) {
	bd := &BibleData{
		Books:  []string{"Mark"},
		Verses: map[string]map[int][]Verse{"Mark": {1: {{Verse: 1, Text: "synthetic verse one"}, {Verse: 2, Text: "synthetic verse two"}}}},
		Headings: map[string]map[int][]Heading{"Mark": {1: {
			{Text: "A synthetic section head", Style: "s1", BeforeVerse: 1, Footnotes: []Footnote{{Text: "a synthetic note kept on the head"}}},
			{Text: "(Synthetic 1:1)", Style: "r", BeforeVerse: 2, Footnotes: []Footnote{{Text: "another synthetic note"}}},
		}}},
	}
	verses := bd.GetChapter("Mark", 1)
	if got := ChapterFootnoteEntries(bd, "Mark", 1, verses); len(got) != 0 {
		t.Errorf("the notes kept on API.Bible headings are listed: %+v", got)
	}
	st := &AppState{Bible: bd, CurrentBook: "Mark", CurrentChapter: 1, CurrentVersion: "nkjv"}
	if chapterHasFootnotes(st) {
		t.Error("a chapter whose only notes are kept on API.Bible headings reports a footnote section")
	}
}

// The shapes a heading keeps as it is: only the feed's own run-on, a
// reference to the chapter and the verse the heading stands above straight
// after the heading's last letter, is read as a note.
func TestOnlyTheFeedsRunOnIsReadAsAHeadingNote(t *testing.T) {
	for _, c := range []struct {
		name    string
		h       Heading
		chapter int
	}{
		{"a reference after a space", Heading{Text: "Fulfilled as in 3:24 of the prophet", Style: "heading", BeforeVerse: 24}, 3},
		{"a reference after a digit", Heading{Text: "Psalm 13:24 and its echo", Style: "heading", BeforeVerse: 24}, 3},
		{"another verse's reference", Heading{Text: daniel3Title + "3:25 " + daniel3Note, Style: "heading", BeforeVerse: 24}, 3},
		{"another chapter's reference", Heading{Text: daniel3Fused, Style: "heading", BeforeVerse: 24}, 4},
		{"no words after the reference", Heading{Text: daniel3Title + "3:24 ", Style: "heading", BeforeVerse: 24}, 3},
		{"no words before it", Heading{Text: "3:24 " + daniel3Note, Style: "heading", BeforeVerse: 24}, 3},
		{"an API.Bible style", Heading{Text: daniel3Fused, Style: "s1", BeforeVerse: 24}, 3},
		{"no verse below", Heading{Text: daniel3Fused, Style: "heading"}, 3},
	} {
		if got, ok := settleHeadingNote(c.h, c.chapter); ok || got.Text != c.h.Text || len(got.Footnotes) != 0 {
			t.Errorf("%s: read as a note: %+v", c.name, got)
		}
	}

	h := Heading{Text: daniel3Fused, Style: "heading", BeforeVerse: 24,
		SmallCaps: []TextSpan{{Start: 4, End: 8}, {Start: 30, End: 40}, {Start: 60, End: 64}},
		Footnotes: []Footnote{{Text: "a note the decoder kept"}}}
	got, ok := settleHeadingNote(h, 3)
	if !ok || got.Text != daniel3Title {
		t.Fatalf("the feed's run-on was not read: %+v", got)
	}
	if len(got.Footnotes) != 2 || got.Footnotes[0].Text != "a note the decoder kept" ||
		got.Footnotes[1].Text != daniel3Note || got.Footnotes[1].Anchor != 35 {
		t.Errorf("notes = %+v, want the kept note, then the run-on one anchored at the title's end", got.Footnotes)
	}
	if want := []TextSpan{{Start: 4, End: 8}, {Start: 30, End: 35}}; fmt.Sprint(got.SmallCaps) != fmt.Sprint(want) {
		t.Errorf("small caps = %v, want %v cut to the title", got.SmallCaps, want)
	}
	if len(h.Footnotes) != 1 || h.Text != daniel3Fused {
		t.Errorf("settling changed the heading it was given: %+v", h)
	}
}

// A heading's words are compared with a selection as the page draws them. A
// drag from verse 23 through the heading into verse 24 shares neither the
// title nor the note as Scripture, and a selection that opens at the title is
// placed at verse 24, the verse it stands above.
func TestTheSongOfTheThreeIsNeverQuotedAsScripture(t *testing.T) {
	bd := daniel3Decoded(t)
	st := &AppState{Bible: bd, CurrentBook: "Daniel", CurrentChapter: 3, CurrentVersion: "webc"}

	drag := daniel3V23 + " " + daniel3Title + " 24 " + daniel3V24
	quote, _, _, _ := prepareShareQuote(st, drag, selSpanFromNative(23, 24))
	if strings.Contains(quote, "HOLY CHILDREN") || strings.Contains(quote, "Septuagint") {
		t.Errorf("a drag across the heading quotes it as Scripture:\n  %q", quote)
	}
	if !strings.Contains(quote, "furnace") || !strings.Contains(quote, "blessing the Lord") {
		t.Errorf("a drag across the heading lost the verses either side of it:\n  %q", quote)
	}

	opened := daniel3Title + " 24 " + daniel3V24 + " 25 " + daniel3V25
	got := selectionVersesIn(st, "Daniel", 3, opened, selSpanFromNative(23, 25))
	if len(got) == 0 || got[0].Verse != 24 || got[len(got)-1].Verse != 25 {
		t.Errorf("a selection opening at the heading resolved to %s, want 24-25", verseRunString(got))
	}

	// The whole heading selected with no span, as a pane that reports none
	// hands it over, alone or with the line break and the verse number that
	// follow it on the page: nothing is quoted, and it is cited at the verse
	// it stands above (wholeHeadingVerse).
	for _, sel := range []string{daniel3Title, daniel3Title + "\n", daniel3Title + "\n24"} {
		quote, cite, _, _ := prepareShareQuote(st, sel, selSpan{})
		if quote != "" || cite != "Daniel 3:24" {
			t.Errorf("the whole heading %q with no span shares as (%q, %q), want nothing quoted, cited Daniel 3:24",
				sel, quote, cite)
		}
	}
}

// Every read of a chapter's headings outside the tests goes through
// headingsFor, so what a surface draws and what the share code compares a
// selection with are the same headings. A read of the stored map anywhere
// else sees the feed's fused Daniel 3 string, 510 characters, where the page
// draws a 35-character heading, and a selection of that heading would be
// matched against words the page does not show. The decoders' writes to the
// map are not reads.
func TestEveryReadOfAChaptersHeadingsGoesThroughHeadingsFor(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	cmds, err := filepath.Glob(filepath.Join("cmd", "*", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	files = append(files, cmds...)
	fset := token.NewFileSet()
	var reads []string
	scanned := 0
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		scanned++
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil || (name == "chapter_blocks.go" && fd.Recv == nil && fd.Name.Name == "headingsFor") {
				continue
			}
			written := map[ast.Expr]bool{}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				if as, ok := n.(*ast.AssignStmt); ok {
					for _, lhs := range as.Lhs {
						written[lhs] = true
					}
				}
				return true
			})
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				ix, ok := n.(*ast.IndexExpr)
				if !ok || written[ix] {
					return true
				}
				if sel, ok := ix.X.(*ast.SelectorExpr); ok && sel.Sel.Name == "Headings" {
					reads = append(reads, fmt.Sprintf("%s in %s", fset.Position(ix.Pos()), fd.Name.Name))
				}
				return true
			})
		}
	}
	if scanned < 100 {
		t.Fatalf("scanned %d source files; the scan is not reading the package", scanned)
	}
	for _, r := range reads {
		t.Errorf("the stored headings are read directly at %s; read them through headingsFor", r)
	}
}
