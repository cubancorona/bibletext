package bibletext

import (
	"fmt"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
)

// chapterRenderFingerprint is the gate that lets the native reading overlay skip
// re-importing identical chapter HTML. These tests pin the load-bearing property:
// it stays stable for the same render but changes whenever anything that affects
// buildChapterHTML's output changes — otherwise navigation would show stale text.
func TestChapterRenderFingerprintStableForSameRender(t *testing.T) {
	s := &AppState{CurrentVersion: "web", CurrentBook: "John", CurrentChapter: 3}
	if a, b := chapterRenderFingerprint(s), chapterRenderFingerprint(s); a != b {
		t.Fatalf("fingerprint not stable for identical state: %q vs %q", a, b)
	}
}

func TestChapterRenderFingerprintChangesOnNavigation(t *testing.T) {
	base := &AppState{CurrentVersion: "web", CurrentBook: "John", CurrentChapter: 3}
	baseFP := chapterRenderFingerprint(base)

	cases := []struct {
		name   string
		mutate func(*AppState)
	}{
		{"chapter", func(s *AppState) { s.CurrentChapter = 4 }},
		{"book", func(s *AppState) { s.CurrentBook = "Mark" }},
		{"version", func(s *AppState) { s.CurrentVersion = "nrsv" }},
		// A background data swap (seed→full, stale-epoch refresh) changes the
		// text under an UNCHANGED version/book/chapter — found live when the
		// epoch upgrade landed but the overlay kept showing flattened poetry.
		{"data swap", func(s *AppState) { s.Bible = &BibleData{} }},
		{"highlight on", func(s *AppState) {
			s.setHL(hlSearch, "John", 3, 16, 0)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &AppState{CurrentVersion: "web", CurrentBook: "John", CurrentChapter: 3}
			tc.mutate(s)
			if got := chapterRenderFingerprint(s); got == baseFP {
				t.Fatalf("fingerprint did not change after %s change: still %q", tc.name, got)
			}
		})
	}
}

// Clearing a highlight must change the fingerprint, or the tap-to-clear path
// (clearHighlightedVerse + refreshReadingOnly) would be skipped by the gate and the
// .hl background wash would linger on screen.
func TestChapterRenderFingerprintChangesWhenHighlightCleared(t *testing.T) {
	s := &AppState{CurrentVersion: "web", CurrentBook: "John", CurrentChapter: 3}
	s.setHL(hlSearch, "John", 3, 16, 0)
	before := chapterRenderFingerprint(s)
	clearHighlightedVerse(s)
	after := chapterRenderFingerprint(s)
	if before == after {
		t.Fatalf("fingerprint unchanged after clearing highlight (%q) — the re-render would be skipped", after)
	}
}

// Two different highlighted verses in the same chapter must differ, since the
// gate decides whether a search-jump (highlight on a specific verse) re-renders.
func TestChapterRenderFingerprintDistinguishesHighlightedVerse(t *testing.T) {
	mk := func(v int) *AppState {
		s := &AppState{CurrentVersion: "web", CurrentBook: "John", CurrentChapter: 3}
		s.setHL(hlSearch, "John", 3, v, 0)
		return s
	}
	if chapterRenderFingerprint(mk(16)) == chapterRenderFingerprint(mk(17)) {
		t.Fatal("fingerprint should differ for different highlighted verses")
	}
}

// --- The fingerprint against the TINT, not against the mark it happens to
// derive from today. ---
//
// chapterRenderFingerprint now folds chapterTints.fingerprint (tint.go). The
// property that must survive the notes rework is stated here in terms of what
// the reader sees: enumerate marks, paint the chapter with each, and require
// that two chapters that would be PAINTED DIFFERENTLY never share a
// fingerprint. Miss that and the native overlays skip a rebuild the reader
// needed — a wash that will not clear, which is the failure the note clause in
// chapterRenderFingerprint was added for.

// tintVector is what the reader would see: the tint of every verse of the
// chapter, in order. Two states with the same vector paint the same chapter.
func tintVector(s *AppState, verses []Verse) string {
	tints := chapterTint(s)
	var b strings.Builder
	for _, v := range verses {
		fmt.Fprintf(&b, "%d,", tints.of(v))
	}
	return b.String()
}

func TestFingerprintChangesWheneverTheTintDoes(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	verses := proseChapter() // Romans 8:1-5
	marks := []struct {
		name   string
		lo, hi int
		clear  bool
	}{
		{name: "no mark", clear: true},
		{name: "1", lo: 1},
		{name: "2", lo: 2},
		{name: "1-4", lo: 1, hi: 4},
		{name: "2-4", lo: 2, hi: 4},
		{name: "1-5", lo: 1, hi: 5},
		{name: "5", lo: 5},
		// The same single verse, spelled both ways the mark-setters spell it.
		// covers() reads them identically, so they PAINT identically and must
		// fold identically — the "no flap" arm of this test, and the reason
		// chapterTints.fingerprint normalises rather than printing the raw span.
		{name: "3 as Hi=0", lo: 3, hi: 0},
		{name: "3 as Hi=3", lo: 3, hi: 3},
		// A chapter-level mark covers no verse at all: it paints what no mark
		// paints, and must fold the same way.
		{name: "chapter-level", lo: 0, hi: 0},
	}

	// ONE AppState, re-marked in place. Building a state per row would make this
	// test pass no matter what the tint clause did: chapterRenderFingerprint
	// folds state.Bible's POINTER (a background data swap must not be skipped),
	// so a fresh BibleData per row gives every row a different fingerprint for a
	// reason that has nothing to do with tints. Caught by mutating
	// chapterTints.fingerprint and watching this test stay green.
	s := tintState("Romans", 8, "web", verses)

	type row struct{ name, vec, fp string }
	var rows []row
	for _, m := range marks {
		if m.clear {
			s.clearMark()
		} else {
			s.setHL(hlSearch, "Romans", 8, m.lo, m.hi)
		}
		rows = append(rows, row{m.name, tintVector(s, verses), chapterRenderFingerprint(s)})
	}
	for i := range rows {
		for j := i + 1; j < len(rows); j++ {
			if rows[i].vec != rows[j].vec && rows[i].fp == rows[j].fp {
				t.Errorf("%s and %s paint differently (%s vs %s) but share fingerprint %q — "+
					"the second render would be skipped and the wash would be wrong on screen",
					rows[i].name, rows[j].name, rows[i].vec, rows[j].vec, rows[i].fp)
			}
			if rows[i].vec == rows[j].vec && rows[i].fp != rows[j].fp {
				t.Errorf("%s and %s paint identically (%s) but differ in fingerprint (%q vs %q) — "+
					"a rebuild for no visible change",
					rows[i].name, rows[j].name, rows[i].vec, rows[i].fp, rows[j].fp)
			}
		}
	}
}

// The ONE place the fingerprint is deliberately coarser than the tint: a mark on
// a chapter that is not on screen tints nothing here and still moves the
// fingerprint. Pinned so the trade is a decision rather than an accident — the
// alternative is walking the chapter's verse list on the path whose whole
// purpose is not doing work, to save a rebuild for a state the app reaches only
// in the instant between setting a mark and navigating to it.
func TestFingerprintIsCoarserThanTheTintForOffChapterMarks(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	verses := proseChapter()
	// One state, re-marked — same reason as above: two states would differ by
	// their Bible pointer and prove nothing.
	s := tintState("Romans", 8, "web", verses)
	unmarkedVec, unmarkedFP := tintVector(s, verses), chapterRenderFingerprint(s)
	s.setHL(hlSearch, "Romans", 9, 1, 4)

	if a, b := unmarkedVec, tintVector(s, verses); a != b {
		t.Fatalf("a mark on another chapter must tint nothing here: %s vs %s", a, b)
	}
	if unmarkedFP == chapterRenderFingerprint(s) {
		t.Fatal("expected the fingerprint to fold the mark's own chapter " +
			"(if this now matches, the gate got finer — update tint.go's fingerprint comment)")
	}
}

// The reading text-size setting must be part of the render fingerprint — a size
// change with a stale fingerprint would show the old text at the old size.
func TestFingerprintIncludesTextSize(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	s := sampleState()
	a := chapterRenderFingerprint(s)
	setReadingTextSizeID("xl")
	defer setReadingTextSizeID("normal")
	if b := chapterRenderFingerprint(s); a == b {
		t.Fatal("fingerprint unchanged after text-size change")
	}
	if got := readingTextScale(); got != 1.3 {
		t.Fatalf("readingTextScale() = %v, want 1.3", got)
	}
}

// The reporter page is body content (leading, indents, paragraph gaps), so it
// must move both fingerprints: the phone-landscape mode flips it on rotation,
// and the Apple push gate would otherwise keep the portrait grammar under the
// landscape measure.
func TestFingerprintIncludesReporterLayout(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	s := sampleState()
	var onBody, offBody, onRender, offRender string
	withReporterLayout(true, func() { onBody, onRender = chapterBodyFingerprint(s), chapterRenderFingerprint(s) })
	withReporterLayout(false, func() { offBody, offRender = chapterBodyFingerprint(s), chapterRenderFingerprint(s) })
	if onBody == offBody {
		t.Fatal("body fingerprint unchanged by the reporter layout")
	}
	if onRender == offRender {
		t.Fatal("render fingerprint unchanged by the reporter layout")
	}
}

// The footnotes toggle must move BOTH fingerprints: the section is body
// content on the Apple panes (chapterBodyFingerprint gates the rebuild path
// there — miss it and toggling repaints nothing), and Android asks the
// combined question through chapterRenderFingerprint.
func TestFingerprintIncludesFootnotes(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	s := sampleState()
	renderA, bodyA := chapterRenderFingerprint(s), chapterBodyFingerprint(s)
	setFootnotesEnabled(true)
	defer setFootnotesEnabled(false)
	if renderB := chapterRenderFingerprint(s); renderA == renderB {
		t.Fatal("render fingerprint unchanged after footnotes toggle")
	}
	if bodyB := chapterBodyFingerprint(s); bodyA == bodyB {
		t.Fatal("body fingerprint unchanged after footnotes toggle — the Apple rebuild gate would skip the repaint")
	}
}
