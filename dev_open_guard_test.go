//go:build !bibletextdev

package bibletext

// The guard that keeps the launch sheet-opener and tab-selector out of
// release builds, as dev_links_guard_test.go keeps the Links page out: this
// file runs in the ordinary `go test ./...`, and fails if a release-shaped
// build selects a tab, opens a sheet, or moves a seam a development build's
// opener moves, for any name BIBLETEXT_DEV_TAB or BIBLETEXT_DEV_OPEN takes.
// Its controls, the same watches seeing a development build select the tab
// and open the sheet, are TestDevTabAtLaunchSelectsTheNamedTab and
// TestDevOpenAtLaunchOpensTheNamedSheet.

import (
	"reflect"
	"testing"

	"fyne.io/fyne/v2"
)

func TestDevOpenNamesAreInertInReleaseBuilds(t *testing.T) {
	holdSheetTimers(t)
	st, w := desktopWindow(t, fyne.NewSize(1280, 800))
	study := reflect.ValueOf(aiActionRun).Pointer()
	run, load := reflect.ValueOf(crossRefsRun).Pointer(), reflect.ValueOf(crossRefsLoad).Pointer()
	versions := len(bibleVersions())
	book, chapter := st.CurrentBook, st.CurrentChapter

	for _, name := range devOpenTabNames {
		from := devTabLaunchFrom(name)
		if tab, rebuilt := launchSelectsATab(t, st, w, name, from); tab != from || rebuilt {
			t.Errorf("a release build left tab %d for tab %d (built again: %v) at launch for BIBLETEXT_DEV_TAB=%s",
				from, tab, rebuilt, name)
		}
	}
	// BIBLETEXT_DEV_TAB stays set through the sheets' launches and their
	// watch, so a tab selected late would show below too.
	tab, content := st.CurrentTab, w.Content()

	if launchOpensASheet(t, st, w, devOpenNames) {
		t.Fatalf("a release build opened %T at launch for a BIBLETEXT_DEV_OPEN name", w.Canvas().Overlays().Top())
	}
	if st.CurrentTab != tab || w.Content() != content {
		t.Errorf("a release build moved the window from tab %d to tab %d at launch", tab, st.CurrentTab)
	}
	if reflect.ValueOf(aiActionRun).Pointer() != study {
		t.Error("a release build replaced the study request at launch")
	}
	if reflect.ValueOf(crossRefsRun).Pointer() != run || reflect.ValueOf(crossRefsLoad).Pointer() != load {
		t.Error("a release build replaced the cross-references load at launch")
	}
	if n := len(bibleVersions()); n != versions {
		t.Errorf("a release build registered %d translations at launch", n-versions)
	}
	if st.CurrentBook != book || st.CurrentChapter != chapter {
		t.Errorf("a release build moved the reader from %s %d to %s %d at launch", book, chapter, st.CurrentBook, st.CurrentChapter)
	}
}
