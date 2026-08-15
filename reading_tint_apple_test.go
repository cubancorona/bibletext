//go:build darwin

package bibletext

// The Apple panes' wash model, and the split that lets a wash change skip the
// NSAttributedString re-import.
//
// These are the two halves of the S2 seam that CAN be tested off the device:
// what Go decides a verse's background should be, and whether the gate that
// decides "rebuild or mutate" can tell a wash change from a text change. What
// they cannot cover is the paint itself, which is TextKit behind cgo — that is
// what the simulator screenshots are for.

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

func tintVerses(book string, chapter, n int) []Verse {
	vs := make([]Verse, 0, n)
	for i := 1; i <= n; i++ {
		vs = append(vs, Verse{BookName: book, Chapter: chapter, Verse: i, Text: "text"})
	}
	return vs
}

func TestNativeTintRunsUnmarkedIsEmpty(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	s := &AppState{CurrentVersion: "web", CurrentBook: "John", CurrentChapter: 3}
	if runs := nativeTintRuns(s, tintVerses("John", 3, 36)); len(runs) != 0 {
		t.Fatalf("unmarked chapter produced %d runs, want none: %+v", len(runs), runs)
	}
}

func TestNativeTintRunsCoalescesTheMarkedSpan(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	s := &AppState{CurrentVersion: "web", CurrentBook: "Romans", CurrentChapter: 8}
	s.setHL(hlNote, "Romans", 8, 1, 4)
	runs := nativeTintRuns(s, tintVerses("Romans", 8, 39))
	if len(runs) != 1 {
		t.Fatalf("want one run for one span, got %d: %+v", len(runs), runs)
	}
	if runs[0].Lo != 1 || runs[0].Hi != 4 {
		t.Fatalf("run = %d..%d, want 1..4", runs[0].Lo, runs[0].Hi)
	}
	// The wash must be the SHARED table's colour, not a second copy of the
	// palette read on the native side — that divergence is what tint.go exists
	// to prevent, and the native pane is now one of the surfaces asking.
	want, _ := tintHighlight.wash(s.pal())
	if runs[0].Wash != want {
		t.Fatalf("wash = %v, want the shared table's %v", runs[0].Wash, want)
	}
}

// A mark can name verses this chapter does not have (it is numbered in another
// translation, or it simply overruns). The run table must stop at the verses
// being drawn, because the native side fills a run by walking lo..hi.
func TestNativeTintRunsStopAtTheVersesDrawn(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	s := &AppState{CurrentVersion: "web", CurrentBook: "Psalms", CurrentChapter: 117}
	s.setHL(hlSearch, "Psalms", 117, 2, 9)
	runs := nativeTintRuns(s, tintVerses("Psalms", 117, 2))
	if len(runs) != 1 || runs[0].Lo != 2 || runs[0].Hi != 2 {
		t.Fatalf("runs = %+v, want a single 2..2 run clamped to the chapter", runs)
	}
}

func TestNativeTintRunsIgnoresAMarkOnAnotherChapter(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	s := &AppState{CurrentVersion: "web", CurrentBook: "John", CurrentChapter: 3}
	s.setHL(hlSearch, "John", 11, 35, 35)
	if runs := nativeTintRuns(s, tintVerses("John", 3, 36)); len(runs) != 0 {
		t.Fatalf("a mark on another chapter produced %+v, want nothing", runs)
	}
}

// THE POINT OF THE SPLIT. A wash change must leave the body identity alone, or
// the Apple gate rebuilds the HTML and re-imports the whole chapter for a change
// that is one attribute over a known range.
func TestChapterBodyFingerprintIgnoresTheWash(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	s := &AppState{CurrentVersion: "web", CurrentBook: "Psalms", CurrentChapter: 119}
	bare := chapterBodyFingerprint(s)
	bareRender := chapterRenderFingerprint(s)

	s.setHL(hlNote, "Psalms", 119, 105, 105)
	if got := chapterBodyFingerprint(s); got != bare {
		t.Fatalf("marking a verse changed the BODY fingerprint:\n %q\n %q", bare, got)
	}
	if chapterRenderFingerprint(s) == bareRender {
		t.Fatal("marking a verse left the render fingerprint unchanged — the wash would never be pushed")
	}

	s.setHL(hlNote, "Psalms", 119, 106, 106)
	if got := chapterBodyFingerprint(s); got != bare {
		t.Fatalf("moving the mark changed the BODY fingerprint:\n %q\n %q", bare, got)
	}

	clearHighlightedVerse(s)
	if got := chapterBodyFingerprint(s); got != bare {
		t.Fatalf("clearing the mark changed the BODY fingerprint:\n %q\n %q", bare, got)
	}
}

// …and the other direction, which is the expensive one to get wrong: anything
// that changes the TEXT must still change the body fingerprint, or the pane goes
// stale. This is the same enumeration reading_fingerprint_test.go pins for the
// combined fingerprint, asserted against the half the Apple gate now uses.
func TestChapterBodyFingerprintChangesWithTheText(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	base := func() *AppState {
		return &AppState{CurrentVersion: "web", CurrentBook: "John", CurrentChapter: 3}
	}
	bare := chapterBodyFingerprint(base())
	cases := []struct {
		name   string
		mutate func(*AppState)
	}{
		{"chapter", func(s *AppState) { s.CurrentChapter = 4 }},
		{"book", func(s *AppState) { s.CurrentBook = "Mark" }},
		{"version", func(s *AppState) { s.CurrentVersion = "nrsv" }},
		{"data swap", func(s *AppState) { s.Bible = &BibleData{} }},
		{"note", func(s *AppState) { s.ActiveNote = "a message" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := base()
			tc.mutate(s)
			if got := chapterBodyFingerprint(s); got == bare {
				t.Fatalf("body fingerprint unchanged after %s changed: %q", tc.name, got)
			}
		})
	}
}

// The run table is copied into a fixed C array on the native side; Go must never
// hand it more rows than that array holds.
func TestNativeTintRunsRespectTheNativeCap(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	s := &AppState{CurrentVersion: "web", CurrentBook: "Psalms", CurrentChapter: 119}
	s.setHL(hlSearch, "Psalms", 119, 1, 176)
	runs := nativeTintRuns(s, tintVerses("Psalms", 119, 176))
	if len(runs) > nativeTintRunCap {
		t.Fatalf("%d runs exceeds the native cap of %d", len(runs), nativeTintRunCap)
	}
	// One contiguous span stays ONE run however long it is: the native side
	// paints a run with a single attribute call, so a marked psalm must not cost
	// 176 of them.
	if len(runs) != 1 {
		t.Fatalf("a whole-chapter span produced %d runs, want 1", len(runs))
	}
}
