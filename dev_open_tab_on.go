//go:build bibletextdev

package bibletext

// The tab BIBLETEXT_DEV_TAB selects at launch. DEVELOPMENT BUILDS ONLY: the
// release build has no names and reads no variable (dev_autoopen_off.go).
//
// A simulator cannot tap a tab, and Books and Search are where a phone held
// sideways keeps its header and its navigation (the Read tab reads
// full-screen there), so a launch can be asked to come up on one:
//
//	SIMCTL_CHILD_BIBLETEXT_DEV_TAB=books xcrun simctl launch <udid> uk.co.bibletext
//
// read, books and search name the three destinations every build has. The
// tab is selected as a tap on it selects it: the index set, the search left
// behind only for Read, and the window rebuilt.

import (
	"fmt"
	"os"
	"slices"
	"strings"
)

// devTabNames are the names BIBLETEXT_DEV_TAB takes, each the label of a
// destination in tabDestinations, lowercased.
var devTabNames = []string{"read", "books", "search"}

// devTabIndex is the index CurrentTab gives the destination name names, read
// from tabDestinations, so the name follows the destination wherever the
// navigation puts it.
func devTabIndex(name string) (int, bool) {
	if !slices.Contains(devTabNames, name) {
		return 0, false
	}
	for i, d := range tabDestinations() {
		if strings.ToLower(d.label) == name {
			return i, true
		}
	}
	return 0, false
}

// devSelectTab selects the tab BIBLETEXT_DEV_TAB names, once, at once. A name
// that is not among devTabNames selects nothing and says so on stderr.
func devSelectTab(state *AppState) {
	name := strings.ToLower(strings.TrimSpace(os.Getenv("BIBLETEXT_DEV_TAB")))
	if name == "" || state == nil {
		return
	}
	tab, ok := devTabIndex(name)
	if !ok {
		fmt.Fprintf(os.Stderr, "bibletext: BIBLETEXT_DEV_TAB=%q names no tab; the names are %s\n",
			name, strings.Join(devTabNames, ", "))
		return
	}
	if state.CurrentTab == tab {
		return
	}
	state.CurrentTab = tab
	leaveSearchForRead(state, tab)
	rebuildWindow(state)
}
