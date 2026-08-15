package bibletext

import "testing"

// Tapping Read while search results are showing must land on the READING view.
//
// The shipped behaviour (1.1.6, 1.1.7): the tab changed and IsSearching did not,
// so rebuildMobileReadingPane kept returning the results view while
// overlayShouldShow went false and took the native reading overlay down with it.
// The reader got a Read tab holding search results, no reading view, and none of
// the Search tab's chrome to explain it.
func TestReadTabLeavesTheSearchResults(t *testing.T) {
	st := &AppState{CurrentBook: "John", CurrentChapter: 3}
	st.IsSearching = true
	st.CurrentTab = 2

	// The reader taps Read.
	st.CurrentTab = 0
	leaveSearchForRead(st, 0)

	if st.IsSearching {
		t.Error("the Read tab still believes results are occupying the reading pane")
	}
	if !overlayShouldShow(st) {
		t.Error("the native reading overlay stays hidden, so the Read tab shows nothing")
	}
}

// ...and the other tabs must NOT clear it: leaving Search for Books and coming
// back should find the results where they were.
func TestOtherTabsLeaveTheSearchStateAlone(t *testing.T) {
	for _, tab := range []int{1, 2, 3} {
		st := &AppState{}
		st.IsSearching = true
		leaveSearchForRead(st, tab)
		if !st.IsSearching {
			t.Errorf("tab %d cleared the search state", tab)
		}
	}
}
