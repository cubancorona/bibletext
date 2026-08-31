package bibletext

import (
	"os"
	"strings"
	"testing"
)

func nativeFunctionSource(t *testing.T, path, signature string) string {
	t.Helper()
	src := readNativeSource(t, path)
	start := strings.Index(src, signature)
	if start < 0 {
		t.Fatalf("%s is missing %s", path, signature)
	}
	end := strings.Index(src[start:], "\n}\n")
	if end < 0 {
		t.Fatalf("%s has no stable function boundary for %s", path, signature)
	}
	return src[start : start+end]
}

func TestAppleNotePlacementRunsAfterTheNoteTuple(t *testing.T) {
	for _, tc := range []struct {
		path      string
		publicFn  string
		privateFn string
	}{
		{"reading_macos.go", "void bibleTextMacScrollToHighlight(void)", "static BOOL btMacScrollToHighlight(void)"},
		{"reading_ios.go", "void bibleTextIOSScrollToHighlight(void)", "static BOOL btIOSScrollToHighlight(void)"},
	} {
		wrapper := nativeFunctionSource(t, tc.path, tc.publicFn)
		if !strings.Contains(wrapper, "dispatch_async(dispatch_get_main_queue()") {
			t.Errorf("%s placement must queue behind SetNote on the main queue", tc.path)
		}
		if strings.Contains(wrapper, "[NSThread isMainThread]") {
			t.Errorf("%s can run placement inline before the queued note tuple", tc.path)
		}

		placement := nativeFunctionSource(t, tc.path, tc.privateFn)
		if !strings.Contains(placement, "else if (noteY >= 0)") {
			t.Errorf("%s cannot place a chapter-level note without a verse wash", tc.path)
		}
	}
}

func TestAppleNextNoteControlsHaveAnAccessibleName(t *testing.T) {
	for _, tc := range []struct {
		path  string
		label string
	}{
		{"reading_macos.go", `accessibilityLabel = @"Next note"`},
		{"reading_ios.go", `accessibilityLabel = @"Next note"`},
	} {
		if !strings.Contains(readNativeSource(t, tc.path), tc.label) {
			t.Errorf("%s note-cycle control has no accessible name", tc.path)
		}
	}
}

// The "land on the note" minimum must apply ONLY when the note's band is above
// the same paragraph the highlight is in.
//
// It exists for a link that CARRIES a note: the band sits directly above the
// washed verse, so scrolling to the verse would push the sender's message off
// the top. Taken unconditionally it does something else entirely. The DISPLAYED
// note is the newest one on the chapter (planDisplayIndex -> noteForChapter),
// which can sit anywhere — and a collapsed set spanning more than one paragraph
// is parked at CHAPTER SCOPE, whose anchor is the first paragraph, whose band is
// reserved with the container's top inset, and whose Y is therefore the top of
// the chapter. A reader with their own notes who then tapped a plain verse link
// got the wash applied correctly and the view left at the top, every time.
//
// Source-level because the placement is Objective-C inside a cgo preamble that
// only builds for its own platform: this is the same shape the contract tests
// above use, and it is what stops the guard being dropped from one surface.
func TestAppleNoteMinimumOnlyAppliesOnTheHighlightsParagraph(t *testing.T) {
	for _, tc := range []struct {
		path      string
		privateFn string
		guard     string
	}{
		{"reading_macos.go", "static BOOL btMacScrollToHighlight(void)", "btMacNoteSharesHighlightPara()"},
		{"reading_ios.go", "static BOOL btIOSScrollToHighlight(void)", "btIOSNoteSharesHighlightPara()"},
	} {
		placement := nativeFunctionSource(t, tc.path, tc.privateFn)
		if !strings.Contains(placement, "noteY - 12") {
			t.Fatalf("%s: the note minimum is gone; this test no longer guards anything", tc.path)
		}
		for _, line := range strings.Split(placement, "\n") {
			if !strings.Contains(line, "noteY - 12 <") {
				continue // the else-if arm, where a note with no wash IS the target
			}
			if !strings.Contains(line, tc.guard) {
				t.Errorf("%s: the note minimum is unguarded:\n  %s\nIt must apply only when %s, "+
					"or a chapter-scope note drags every arriving link to the top of the chapter.",
					tc.path, strings.TrimSpace(line), tc.guard)
			}
		}
		// And the guard must actually compare the two paragraphs, not merely exist.
		fn := strings.TrimSuffix(tc.guard, "()")
		body := nativeFunctionSource(t, tc.path, "static BOOL "+fn+"(void)")
		for _, want := range []string{"paragraphRangeForRange", "notePara.location == hlPara.location"} {
			if !strings.Contains(body, want) {
				t.Errorf("%s: %s does not compare paragraphs (missing %q)", tc.path, fn, want)
			}
		}
	}
}

// An explicit arrival must not have a scroll restore captured underneath it.
//
// All three reading panes capture the reader's live position into state.restore
// when the SAME chapter re-renders with a different body — the guard against a
// theme flip yanking a mid-chapter reader to the top. It must not fire on a
// render the reader ASKED for: applyShareTarget sets forceReposition and clears
// restore precisely so the placement falls through to the highlight, and
// bibleTextScrollReadingTV checks restore FIRST. Capturing there re-created the
// restore the arrival had just cleared, so a link tapped on the chapter already
// open applied its wash and left the viewport where it started.
//
// Android carried the clause from the beginning; the two Apple panes did not.
// Source-level for the same reason as the placement contracts above: two of the
// three only build for their own platform.
func TestReadingPanesDoNotCaptureARestoreOverAnArrival(t *testing.T) {
	for _, path := range []string{"reading_ios.go", "reading_macos.go", "reading_android.go"} {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		body := string(src)
		// The capture is recognised by the ANCHOR it writes, which every pane
		// must do however its condition is spelled — so the test cannot be
		// satisfied by deleting the capture, and cannot be broken by rewording
		// the condition.
		idx := strings.Index(body, "state.restore = &restoreAnchor{")
		if idx < 0 {
			t.Fatalf("%s: the same-chapter restore capture is gone; this test guards nothing", path)
		}
		// STRICTLY STRONGER than "the condition mentions the arrival": the pane
		// must ask the SHARED predicate. Mentioning it was satisfiable by three
		// separate conditions that happened to agree, which is the shape that
		// let two of these three panes drift apart in the first place.
		if !strings.Contains(body, "shouldCaptureScrollRestore(") {
			t.Errorf("%s: it captures a scroll restore without asking "+
				"shouldCaptureScrollRestore. Each pane deciding this for itself is how "+
				"two of the three came to omit the arrival clause, so a tapped link on "+
				"the chapter already open was dragged back to where the reader was.", path)
		}
	}
}

// EVERY SURFACE SPELLS THE CHROME'S OWN STRINGS THE SAME WAY.
//
// The chevron is app chrome, not a renderer's choice — and it was four separate
// literals that had already drifted: Android used two spaces where everyone else
// used one, so its counts sat a space further out than anybody had decided.
// Nothing could have caught that; there was no place the four were compared.
//
// So it is composed ONCE, in Go, as part of the who line (chapterNoteChrome),
// and no surface appends it any more. This is the inverse contract: the literal
// must not reappear in any renderer, in any spelling. A surface that spells it
// again is a surface that can drift again.
//
// Source-level because two of the four are Objective-C in a cgo preamble and one
// is Java: none can be linked into a Go test, but all four can be read.
func TestNoSurfaceSpellsTheChevronItself(t *testing.T) {
	// Every spelling seen in this repository's history, plus the current one.
	spellings := []string{
		`"` + noteChevron + `"`, `@"` + noteChevron + `"`,
		`"  ›"`, `@"  ›"`, `"›"`, `@"›"`, `'›'`,
	}
	for _, path := range []string{
		"reading_ios.go", "reading_macos.go",
		"android/BtBridge.java", "reading_styled_note.go",
	} {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		body := string(src)
		for _, sp := range spellings {
			if strings.Contains(body, sp) {
				t.Errorf("%s spells the chevron itself (%s). It is part of the who "+
					"line the Go side composes now; a renderer that appends its own "+
					"puts two on the card, or one that has drifted from the others.",
					path, sp)
			}
		}
	}

	// The control. With the literal gone from all four renderers, a test that
	// only looks for its ABSENCE passes just as well if the chevron has been
	// deleted from the app entirely — so the composer must still carry it.
	if !strings.Contains(noteCountsSpan("Amy · 1 of 2 in this chapter"+noteChevron, true), noteChevron) {
		t.Error("the composed counts span no longer carries the chevron, so the " +
			"absence checked above proves nothing")
	}
}

// The four expressions of "is there a sticker" and "is it collapsed" must stay
// the same expression. They are identical today and equivalent to the shared Go
// wherever anything is drawn; this is what makes the next edit to one of them a
// failure rather than a divergence nobody sees.
func TestEverySurfaceAsksPresenceAndCollapseTheSameWay(t *testing.T) {
	for _, tc := range []struct{ path, present, pill string }{
		{"reading_ios.go",
			"gNoteText != nil || gNoteWho != nil",
			"gNoteMinimized || gNoteText == nil"},
		{"reading_macos.go",
			"gMacNoteText != nil || gMacNoteWho != nil",
			"gMacNoteMinimized || gMacNoteText == nil"},
		{"android/BtBridge.java",
			"noteText != null || noteWho != null",
			"notePill || noteText == null"},
	} {
		src, err := os.ReadFile(tc.path)
		if err != nil {
			t.Fatalf("read %s: %v", tc.path, err)
		}
		body := string(src)
		if !strings.Contains(body, tc.present) {
			t.Errorf("%s: presence is not %q — it must match noteChrome.present() "+
				"(Text or Who non-empty)", tc.path, tc.present)
		}
		if !strings.Contains(body, tc.pill) {
			t.Errorf("%s: the collapsed test is not %q — it must match "+
				"noteChrome.collapsed() wherever a sticker is drawn", tc.path, tc.pill)
		}
	}
}
