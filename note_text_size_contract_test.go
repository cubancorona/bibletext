package bibletext

import (
	"fmt"
	"strings"
	"testing"
)

// The note card's text is one size on every surface: noteBodySize for the
// message and noteWhoSize for the byline and the pills (reading_page.go). The
// card is the app's furniture and does not follow the reader's text size, so
// each surface sets these as plain numbers; this holds every one of those
// numbers to the spec's, spelled from the constants so a changed constant
// cannot leave a surface behind. The Mac set 13 and 10, and the Windows and
// Linux pane took the toolkit's 18, until the reading page was specified once.
func TestNoteTextIsTheSpecsOnEverySurface(t *testing.T) {
	body, who := fmt.Sprintf("%g", noteBodySize), fmt.Sprintf("%g", noteWhoSize)

	for _, tc := range []struct {
		name, path, sig string
		java            bool
		want            string
	}{
		{"iOS body", "reading_ios.go", "static UIFont *btNoteBodyFont(void)", false, "systemFontOfSize:" + body + "]"},
		{"iOS byline", "reading_ios.go", "static UIFont *btNoteWhoFont(void)", false, "systemFontOfSize:" + who + " weight:UIFontWeightSemibold"},
		{"macOS body", "reading_macos.go", "static NSFont *btMacNoteBodyFont(void)", false, "systemFontOfSize:" + body + "]"},
		{"macOS byline", "reading_macos.go", "static NSFont *btMacNoteWhoFont(void)", false, "systemFontOfSize:" + who + " weight:NSFontWeightSemibold"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := readNativeSource(t, tc.path)
			i := strings.Index(src, tc.sig)
			if i < 0 {
				t.Fatalf("%s is missing %s", tc.path, tc.sig)
			}
			line := src[i:]
			if j := strings.Index(line, "\n"); j >= 0 {
				line = line[:j]
			}
			if !strings.Contains(line, tc.want) {
				t.Errorf("%s: %q does not set %q", tc.path, strings.TrimSpace(line), tc.want)
			}
		})
	}

	// Android: sp at the spec's numbers (sp follows the reader's accessibility
	// font scale there, deliberately — notes_bubble.go).
	java := readNativeSource(t, "android/BtBridge.java")
	for _, want := range []string{
		"body.setTextSize(android.util.TypedValue.COMPLEX_UNIT_SP, " + body + "f)",
		"who.setTextSize(android.util.TypedValue.COMPLEX_UNIT_SP, " + who + "f)",
		"chip.setTextSize(android.util.TypedValue.COMPLEX_UNIT_SP, " + who + "f)",
	} {
		if !strings.Contains(java, want) {
			t.Errorf("android/BtBridge.java no longer sets %q", want)
		}
	}

	// The canvas pane reads the constants.
	if got := styledUISize(); float64(got) != noteBodySize {
		t.Errorf("the canvas pane's note body is %v, want %v", got, noteBodySize)
	}
	if float64(styledNoteWhoSz) != noteWhoSize {
		t.Errorf("the canvas pane's byline is %v, want %v", styledNoteWhoSz, noteWhoSize)
	}
}
