package bibletext

// What both builds' tests of BIBLETEXT_DEV_OPEN and BIBLETEXT_DEV_TAB share:
// the names, and handing one to a launch as the simulator does. A development
// build opens the sheet a name names, and selects the tab
// (dev_open_sheets_test.go); a release build must open nothing and select
// nothing for any of them (dev_open_guard_test.go). The names are listed
// here, untagged, because the release build has no table to read them from;
// the development tests hold these lists equal to their tables.

import (
	"testing"
	"time"

	"fyne.io/fyne/v2"
)

// devOpenNames is every name BIBLETEXT_DEV_OPEN takes in a development build.
var devOpenNames = []string{
	"settings", "goto", "chapters", "versions", "votd", "votd-one", "votd-long",
	"audio", "note", "ask", "ai-waiting", "xrefs-waiting", "share-image", "note-offer",
	"link-notice", "version-loading", "version-error", "versions-more", "xrefs",
}

// devOpenTabNames is every name BIBLETEXT_DEV_TAB takes in a development
// build.
var devOpenTabNames = []string{"read", "books", "search"}

// devOpenLaunchWait is how long a launch is watched for a sheet: past the
// 1.2 seconds the development build waits before it opens one.
const devOpenLaunchWait = 2500 * time.Millisecond

// launchOpensASheet sets BIBLETEXT_DEV_OPEN to each of names in turn and hands
// it to devAutoOpenSheet, as a launch does, then watches st's window for
// devOpenLaunchWait and reports whether any sheet came up on it.
func launchOpensASheet(t *testing.T, st *AppState, w fyne.Window, names []string) bool {
	t.Helper()
	for _, name := range names {
		t.Setenv("BIBLETEXT_DEV_OPEN", name)
		devAutoOpenSheet(st)
	}
	for end := time.Now().Add(devOpenLaunchWait); time.Now().Before(end); time.Sleep(50 * time.Millisecond) {
		if w.Canvas().Overlays().Top() != nil {
			return true
		}
	}
	return false
}

// devTabLaunchFrom is the tab a launch naming name is watched from: Books for
// read, and the Read tab for the others, so a launch that selects the tab it
// names moves the window whichever it names.
func devTabLaunchFrom(name string) int {
	if name == "read" {
		return 1
	}
	return 0
}

// launchSelectsATab puts st's window on tab from, sets BIBLETEXT_DEV_TAB to
// name and hands it to devAutoOpenSheet with no sheet named, as a launch
// does, and reports the tab the window stands on when it returns and whether
// the window was built again. A development build selects the tab before it
// returns (TestDevTabAtLaunchSelectsTheNamedTab), so what this reports is
// what the launch did.
func launchSelectsATab(t *testing.T, st *AppState, w fyne.Window, name string, from int) (tab int, rebuilt bool) {
	t.Helper()
	if st.CurrentTab != from {
		st.CurrentTab = from
		rebuildWindow(st)
	}
	before := w.Content()
	t.Setenv("BIBLETEXT_DEV_OPEN", "")
	t.Setenv("BIBLETEXT_DEV_TAB", name)
	devAutoOpenSheet(st)
	return st.CurrentTab, w.Content() != before
}
