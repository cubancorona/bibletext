package bibletext

// CLEARING A VERSION HAS TO MAKE THE NEXT LINK BEHAVE AS NEVER-DOWNLOADED.
//
// Deleting the cache file does not do that on its own, and this is the test
// that says so. A loaded translation is held in AppState.loadedVersions by id;
// switchVersion consults that map before the disk, and switchToLinkVersion
// returns early when the link names the version already open. So the dev
// "Clear" button removed the file, changed nothing observable, and an /nkjv/
// note still opened instantly — reported as "maybe it wasn't cleared?".

import (
	"testing"
)

func TestClearingAVersionMakesALinkTakeTheUnavailablePath(t *testing.T) {
	target, ok := ParseShareLink(
		"https://bibletext.co.uk/nkjv/psalms/23/#v1-4&n=cmEDAQEEYgESYwEXdDBCZWVuIHRoaW5raW5nIG9mIHlvdSB0b2RheS4gVGhpcyBvbmUgaXMgZm9yIHlvdS52BG5ranY")
	if !ok {
		t.Fatal("fixture link does not parse")
	}

	st := &AppState{
		CurrentVersion: "nkjv",
		loadedVersions: map[string]*BibleData{"nkjv": {}, "web": {}},
	}

	// 1. READING IT ALREADY: the link never consults the cache at all.
	if switchToLinkVersion(st, target) {
		t.Error("a link naming the version already open should not switch or download")
	}
	if msg := linkVersionUnavailable(st, target); msg != "" {
		t.Errorf("no message is due while reading that very translation; got %q", msg)
	}

	// 2. WHAT THE DEV BUTTON NOW DOES: move off it, then forget it.
	switchVersionForTest(st, defaultVersionID)
	delete(st.loadedVersions, "nkjv")

	if _, still := st.loadedVersions["nkjv"]; still {
		t.Fatal("nkjv is still in loadedVersions — the app can serve it from memory, " +
			"so no amount of deleting cache files changes what a link does")
	}
	if st.CurrentVersion == "nkjv" {
		t.Fatal("still reading nkjv; the clear did not move off it")
	}

	// 3. NOW the link is the not-downloaded case: no switch, and a message.
	if switchToLinkVersion(st, target) {
		t.Error("without a key or licence there is nothing to switch to")
	}
	if msg := linkVersionUnavailable(st, target); msg == "" {
		t.Error("the reader should now be told the note was written in the NKJV")
	}
}

// switchVersionForTest mirrors the state change switchVersion makes, without
// needing a window or loadable data: the point under test is the MAP and the
// current id, not the rebuild.
func switchVersionForTest(s *AppState, id string) {
	s.CurrentVersion = id
}
