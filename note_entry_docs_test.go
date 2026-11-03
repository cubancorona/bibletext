package bibletext

import (
	"strings"
	"testing"
)

// The docs record a device observation with its date and the image or device
// it was made on, as every other such record in them does, never as a hedged
// report of another checkout; and the Back paragraph names the mechanism that
// serves each Android release.
func TestNoteEntryDocsDateWhatWasSeen(t *testing.T) {
	for _, path := range []string{"docs/ANDROID.md", "docs/BACKLOG.md", "docs/PLATFORM_MATRIX.md", "docs/ROADMAP.md"} {
		doc := readNativeSource(t, path)
		for _, stale := range []string{"Seen elsewhere", "seen elsewhere", "not yet reproduced here"} {
			if strings.Contains(doc, stale) {
				t.Errorf("%s records an observation as %q, with no date, image or device", path, stale)
			}
		}
	}
	android := readNativeSource(t, "docs/ANDROID.md")
	start := strings.Index(android, "## Note and question entry")
	if start < 0 {
		t.Fatal("docs/ANDROID.md has no Note and question entry section")
	}
	section := android[start:]
	if end := strings.Index(section[3:], "\n## "); end >= 0 {
		section = section[:end+3]
	}
	if strings.Contains(section, "Android 33 and later use the window back dispatcher") {
		t.Error("docs/ANDROID.md credits the back dispatcher with Android 13 to 15, where the registration is refused without a manifest opt-in")
	}
	for _, want := range []string{"KEYCODE_BACK", "Android 16"} {
		if !strings.Contains(section, want) {
			t.Errorf("the Note and question entry section does not say where %s comes in", want)
		}
	}
}
