//go:build !race

package bibletext

import (
	"runtime"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
)

// skipIfNativeReadingOverlay skips on macOS, where the reading pane is a native
// NSTextView overlay (reading_macos.go) rather than the Fyne chapterText widget —
// so buildReadingView builds no chapterText there. The Fyne path (reading_fyne.go,
// //go:build ios || !darwin) is exercised on Linux/Windows.
func skipIfNativeReadingOverlay(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("reading pane is a native NSTextView overlay on macOS; chapterText is the Linux/Windows path")
	}
}

// themedTestApp returns a Fyne test app with the app's real theme installed, so
// rendering tests resolve the custom colours (bibleTextMuted, …) and fonts that Fyne's
// bare "Default Test Theme" leaves undefined — which otherwise logs nil-colour errors
// and panics on a nil font during text measurement. Fonts are nil here, so bibleTheme
// falls back to Fyne's bundled default faces (enough for layout assertions).
func themedTestApp() fyne.App {
	app := test.NewApp()
	app.Settings().SetTheme(&bibleTheme{})
	return app
}

func findChapterText(o fyne.CanvasObject) *chapterText {
	switch v := o.(type) {
	case *chapterText:
		return v
	case *container.Scroll:
		return findChapterText(v.Content)
	case *container.Split:
		if c := findChapterText(v.Leading); c != nil {
			return c
		}
		return findChapterText(v.Trailing)
	case *fyne.Container:
		for _, c := range v.Objects {
			if r := findChapterText(c); r != nil {
				return r
			}
		}
	}
	return nil
}

// TestChapterTextIsSelectableReadOnlyWholeChapter verifies the chapter renders as
// one selectable, read-only block (so selection/copy spans the whole chapter) and
// uses manual wrapping (no internal scroll area).
func TestChapterTextIsSelectableReadOnlyWholeChapter(t *testing.T) {
	skipIfNativeReadingOverlay(t)
	app := themedTestApp()
	defer app.Quit()

	state := sampleState() // John 1: verses 1-3 in the sample data
	ct := findChapterText(buildReadingView(state))
	if ct == nil {
		t.Fatal("no chapterText found in the reading view")
	}
	if ct.Wrapping != fyne.TextWrapOff {
		t.Errorf("expected manual wrapping (TextWrapOff), got %v", ct.Wrapping)
	}
	// All verses share one widget, so selection can span the whole chapter.
	for _, frag := range []string{"beginning", "same", "things"} {
		if !strings.Contains(ct.Text, frag) {
			t.Errorf("chapter text is missing %q — verses should share one selectable widget", frag)
		}
	}
	before := ct.Text
	ct.TypedRune('Z')
	if ct.Text != before {
		t.Error("chapter text must be read-only")
	}
}

func TestChapterTextLocatesHighlightedVerse(t *testing.T) {
	skipIfNativeReadingOverlay(t)
	app := themedTestApp()
	defer app.Quit()

	state := sampleState()
	state.HighlightedBook = "John"
	state.HighlightedChapter = 1
	state.HighlightedVerse = 2
	state.HasHighlightedVerse = true

	ct := findChapterText(buildReadingView(state))
	if ct == nil {
		t.Fatal("no chapterText found")
	}
	if ct.highlightLine < 0 {
		t.Error("expected the highlighted verse to be located for scroll-to")
	}
}

func TestCleanCopyJoinsSoftWrapsKeepsParagraphs(t *testing.T) {
	in := "In the\nbeginning was\nthe Word.\n\nThe same\nwas here."
	want := "In the beginning was the Word.\n\nThe same was here."
	if got := cleanCopy(in); got != want {
		t.Errorf("cleanCopy = %q, want %q", got, want)
	}
}
