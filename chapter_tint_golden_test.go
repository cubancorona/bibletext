package bibletext

// THE PROOF THAT S3 CHANGED NOTHING.
//
// S3 replaced five independent "is this verse highlighted?" questions with one
// chapterTint answer. Every one of those questions was asked inside a renderer,
// so the only honest way to show the swap is invisible is to freeze what the
// renderers emit BEFORE the swap and re-assert it byte-for-byte afterwards.
// This file is that freeze: it captures every surface that draws a tint —
// buildChapterHTML in both its layouts, buildChapterHTMLAndroid, the styled
// desktop layout's per-run tints and derived wash rectangles, the legacy Fyne
// pane's wrap + band lines, and the RichText fallback's segment colours — into
// ONE golden file (testdata/chapter_tint_golden.txt).
//
// Regenerate deliberately, never casually:
//
//	BIBLETEXT_UPDATE_GOLDEN=1 go test -run TestChapterTintGolden ./...
//
// A diff in this file is a change a reader can see. If S3's commit shows one,
// S3 is wrong.
//
// The cases are chosen for the decisions the tint feeds, not for coverage
// theatre: no mark at all; a range whose interior joins must stay inside the
// band; a single verse; a mark on ANOTHER chapter and one past the last verse
// (both must light nothing — the per-verse book/chapter test that
// isVerseHighlighted did, and that a chapter-scoped tint could quietly lose);
// a chapter-level mark with Lo=0; poetry, where joins are <br> and there is no
// space to bridge; whole-verse red letters under the band; and the BSB's
// span-level red letters under the band, which is where the .hl/.wj pairing
// lives.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
)

const chapterTintGoldenPath = "testdata/chapter_tint_golden.txt"

// tintCase is one rendered chapter: a state, and the verses it draws.
type tintCase struct {
	name   string
	state  *AppState
	verses []Verse
}

func proseChapter() []Verse {
	return []Verse{
		{BookName: "Romans", Chapter: 8, Verse: 1, Text: "Therefore, there is now no condemnation."},
		{BookName: "Romans", Chapter: 8, Verse: 2, Text: "For the law of the Spirit set you free."},
		{BookName: "Romans", Chapter: 8, Verse: 3, Text: "For what the law was powerless to do."},
		{BookName: "Romans", Chapter: 8, Verse: 4, Text: "So that the righteous standard is fulfilled."},
		{BookName: "Romans", Chapter: 8, Verse: 5, Text: "Those who live according to the flesh."},
	}
}

func poeticChapter() []Verse {
	return []Verse{
		{BookName: "Psalms", Chapter: 23, Verse: 1, Text: "The LORD is my shepherd;\nI shall not want."},
		{BookName: "Psalms", Chapter: 23, Verse: 2, Text: "He makes me lie down in green pastures;\nHe leads me beside quiet waters."},
		{BookName: "Psalms", Chapter: 23, Verse: 3, Text: "He restores my soul;\nHe guides me in paths of righteousness."},
		{BookName: "Psalms", Chapter: 23, Verse: 4, Text: "Even though I walk through the valley,\nI will fear no evil."},
	}
}

// johnChapter is words-of-Christ at VERSE level (the WEB's table): one run,
// reddened whole. John 3:16 is inside wordsOfChristRanges.
func johnChapter() []Verse {
	return []Verse{
		{BookName: "John", Chapter: 3, Verse: 15, Text: "that whoever believes in Him may have eternal life."},
		{BookName: "John", Chapter: 3, Verse: 16, Text: "For God so loved the world that He gave His one and only Son."},
		{BookName: "John", Chapter: 3, Verse: 17, Text: "For God did not send His Son into the world to condemn it."},
	}
}

// markChapter is words-of-Christ at SPAN level (the BSB's own table): the verse
// splits into runs, and only one of them is His.
func markChapter() []Verse {
	return []Verse{
		{BookName: "Mark", Chapter: 8, Verse: 4, Text: "His disciples replied to Him."},
		{BookName: "Mark", Chapter: 8, Verse: 5, Text: bsbVerseFixture[verseKeyFor("Mark", 8, 5)]},
		{BookName: "Mark", Chapter: 8, Verse: 6, Text: "And He instructed the crowd to sit down."},
	}
}

func tintState(book string, chapter int, version string, verses []Verse) *AppState {
	bd := NewBibleData()
	bd.Verses = map[string]map[int][]Verse{book: {chapter: verses}}
	bd.Books = []string{book}
	bd.PrepareSearchIndex()
	return &AppState{Bible: bd, CurrentBook: book, CurrentChapter: chapter, CurrentVersion: version}
}

// tintCases builds every rendered case. Marks are set with hlSearch, the most
// neutral origin — the tint is the same whoever placed the mark, and pinning it
// with a note's origin would fold the note bubble into the fixture.
func tintCases() []tintCase {
	var cases []tintCase
	add := func(name, book string, chapter int, version string, verses []Verse, mark func(*AppState)) {
		st := tintState(book, chapter, version, verses)
		if mark != nil {
			mark(st)
		}
		cases = append(cases, tintCase{name: name, state: st, verses: verses})
	}

	add("prose/no-mark", "Romans", 8, "web", proseChapter(), nil)
	add("prose/range-1-4", "Romans", 8, "web", proseChapter(), func(s *AppState) {
		s.setHL(hlSearch, "Romans", 8, 1, 4)
	})
	add("prose/single-3", "Romans", 8, "web", proseChapter(), func(s *AppState) {
		s.setHL(hlSearch, "Romans", 8, 3, 0)
	})
	add("prose/mark-on-another-chapter", "Romans", 8, "web", proseChapter(), func(s *AppState) {
		s.setHL(hlSearch, "Romans", 9, 1, 4)
	})
	add("prose/mark-on-another-book", "Romans", 8, "web", proseChapter(), func(s *AppState) {
		s.setHL(hlSearch, "Mark", 8, 1, 4)
	})
	add("prose/mark-past-the-last-verse", "Romans", 8, "web", proseChapter(), func(s *AppState) {
		s.setHL(hlSearch, "Romans", 8, 90, 95)
	})
	add("prose/chapter-level-mark", "Romans", 8, "web", proseChapter(), func(s *AppState) {
		s.setHL(hlSearch, "Romans", 8, 0, 0)
	})
	add("poetry/no-mark", "Psalms", 23, "web", poeticChapter(), nil)
	add("poetry/range-2-3", "Psalms", 23, "web", poeticChapter(), func(s *AppState) {
		s.setHL(hlSearch, "Psalms", 23, 2, 3)
	})
	add("red/whole-verse-under-the-band", "John", 3, "web", johnChapter(), func(s *AppState) {
		s.setHL(hlSearch, "John", 3, 15, 16)
	})
	add("red/whole-verse-no-mark", "John", 3, "web", johnChapter(), nil)
	add("red/bsb-spans-under-the-band", "Mark", 8, "bsb", markChapter(), func(s *AppState) {
		s.setHL(hlSearch, "Mark", 8, 4, 5)
	})
	add("red/bsb-spans-no-mark", "Mark", 8, "bsb", markChapter(), nil)
	return cases
}

// renderAllSurfaces produces the whole golden body: every case through every
// surface that draws a tint.
func renderAllSurfaces(t *testing.T) string {
	t.Helper()
	var b strings.Builder
	for _, c := range tintCases() {
		for _, reporter := range []bool{false, true} {
			withReporterLayout(reporter, func() {
				fmt.Fprintf(&b, "### %s | apple-html reporter=%v\n%s\n\n",
					c.name, reporter, buildChapterHTML(c.state, c.verses))
			})
		}
		fmt.Fprintf(&b, "### %s | android-html\n%s\n\n",
			c.name, buildChapterHTMLAndroid(c.state, c.verses))
		fmt.Fprintf(&b, "### %s | styled-layout\n%s\n\n",
			c.name, dumpStyledLayout(c.state, c.verses))
		fmt.Fprintf(&b, "### %s | legacy-fyne-pane\n%s\n\n",
			c.name, dumpChapterText(c.state, c.verses))
		fmt.Fprintf(&b, "### %s | richtext-fallback\n%s\n\n",
			c.name, dumpMobileSegments(c.state, c.verses))
	}
	return b.String()
}

func withReporterLayout(on bool, fn func()) {
	orig := reporterLayout
	reporterLayout = func() bool { return on }
	defer func() { reporterLayout = orig }()
	fn()
}

// dumpStyledLayout serialises the styled pane's tint answer: the per-run tints
// (what the text colour rules read) and the derived wash rectangles (what the
// renderer paints). A deterministic ruler, so the geometry is exact.
func dumpStyledLayout(state *AppState, verses []Verse) string {
	measure := func(text string, kind runKind) float32 {
		w := float32(len([]rune(text))) * 7
		if kind == runVerseNum {
			w = float32(len([]rune(text))) * 5
		}
		return w
	}
	lay := layoutChapter(state, verses, styledLayoutParams{
		Width: 220, LineHeight: 24, ParaGap: 12, SpaceW: 4,
	}, measure)

	var b strings.Builder
	for li, ln := range lay.Lines {
		fmt.Fprintf(&b, "line %d y=%.1f paraFirst=%v poemBreak=%v [%d,%d)\n",
			li, ln.Y, ln.ParaFirst, ln.PoemBreakAfter, ln.StartOffset, ln.EndOffset)
		for _, r := range ln.Runs {
			fmt.Fprintf(&b, "  run v%d kind=%d red=%v tint=%d override=%v x=%.1f w=%.1f off=%d %q\n",
				r.Verse, r.Kind, r.RedLetter, r.Tint, r.Tint.overridesTextColour(), r.X, r.W, r.Offset, r.Text)
		}
	}
	for _, sp := range tintSpansForLayout(lay) {
		fmt.Fprintf(&b, "wash line=%d tint=%d x0=%.1f x1=%.1f\n", sp.Line, sp.Tint, sp.LineX0, sp.LineX1)
	}
	fmt.Fprintf(&b, "height=%.1f text=%q\n", lay.Height, lay.Text)
	return b.String()
}

// dumpChapterText serialises the legacy Fyne pane: the wrapped text model and
// the band's line range, which is the only thing its highlight produces.
func dumpChapterText(state *AppState, verses []Verse) string {
	c := newChapterText(state, verses)
	c.rewrap(360)
	var b strings.Builder
	fmt.Fprintf(&b, "highlightLine=%d highlightEndLine=%d totalLines=%d\n",
		c.highlightLine, c.highlightEndLine, c.totalLines)
	fmt.Fprintf(&b, "verseLines=%v\n", c.verseLines)
	fmt.Fprintf(&b, "text=%q\n", c.Text)
	return b.String()
}

// dumpMobileSegments serialises the RichText fallback: the colour NAME per
// segment is the whole of its tint model (it has no band to paint).
func dumpMobileSegments(state *AppState, verses []Verse) string {
	var b strings.Builder
	for _, seg := range mobileParagraphSegments(state, verses) {
		fmt.Fprintf(&b, "%#v\n", seg)
	}
	return b.String()
}

func TestChapterTintGolden(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Preferences().SetBool(prefRedLetter, true)

	got := renderAllSurfaces(t)

	if os.Getenv("BIBLETEXT_UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(chapterTintGoldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(chapterTintGoldenPath, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Log("golden rewritten:", chapterTintGoldenPath)
		return
	}

	want, err := os.ReadFile(chapterTintGoldenPath)
	if err != nil {
		t.Fatalf("golden missing (%v) — regenerate with BIBLETEXT_UPDATE_GOLDEN=1", err)
	}
	if got == string(want) {
		return
	}
	// Report the FIRST divergent case, not a 200KB diff: the golden is
	// per-surface and the failing surface is the finding.
	gotBlocks, wantBlocks := goldenBlocks(got), goldenBlocks(string(want))
	for i := range gotBlocks {
		if i >= len(wantBlocks) {
			t.Fatalf("golden has %d blocks, render produced %d; first extra:\n%s",
				len(wantBlocks), len(gotBlocks), gotBlocks[i])
		}
		if gotBlocks[i] != wantBlocks[i] {
			t.Fatalf("output changed.\n--- golden ---\n%s\n--- now ---\n%s",
				wantBlocks[i], gotBlocks[i])
		}
	}
	t.Fatalf("golden has %d blocks, render produced %d", len(wantBlocks), len(gotBlocks))
}

func goldenBlocks(s string) []string {
	parts := strings.Split(s, "\n### ")
	for i := 1; i < len(parts); i++ {
		parts[i] = "### " + parts[i]
	}
	return parts
}
