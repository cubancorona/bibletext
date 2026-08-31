package bibletext

import (
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
