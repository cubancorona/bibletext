package bibletext

import (
	"os"
	"strings"
	"testing"
)

func nativeFunctionSource(t *testing.T, path, signature string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	src := string(raw)
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
		raw, err := os.ReadFile(tc.path)
		if err != nil {
			t.Fatalf("read %s: %v", tc.path, err)
		}
		if !strings.Contains(string(raw), tc.label) {
			t.Errorf("%s note-cycle control has no accessible name", tc.path)
		}
	}
}
