//go:build !ios && !android

package bibletext

// THE TWO QUESTIONS ABOUT THE DESKTOP NAVIGATION MUST KEEP THE SAME ANSWER.
//
// BIBLETEXT_DESKTOP_TABS once had two independent readers: one deciding whether
// the shared compact layout ran at all, one deciding how it drew its
// navigation. "rail" satisfied the second and failed the first, so the app fell
// back to the sidebar while every rendered gallery showed the rail — the
// gallery calls buildCompactUI directly and never crosses the gate, so nothing
// on the render path could catch it.
//
// The assertions below are therefore not "rail maps to rail". They are the
// coupling: a value that selects a navigation style must not leave the desktop
// on a layout that cannot draw it.
//
// The rail is now the default rather than something the variable turns on, so
// the interesting case has inverted: an unset variable must give the RAIL, and
// only an explicit opt-out returns the former sidebar.

import "testing"

func TestDesktopNavStyleValues(t *testing.T) {
	for _, tc := range []struct {
		env      string
		want     desktopNavStyle
		wantRail bool
	}{
		// Unset and unrecognised both mean "no override" — the shipped rail.
		{"", desktopNavRail, true},
		{"nonsense", desktopNavRail, true},
		{"rail", desktopNavRail, true},
		// The opt-out, both spellings.
		{"sidebar", desktopNavSidebar, false},
		{"0", desktopNavSidebar, false},
		// The bottom-bar comparison.
		{"1", desktopNavBar, false},
		{"bar", desktopNavBar, false},
	} {
		name := tc.env
		if name == "" {
			name = "unset"
		}
		t.Run("env="+name, func(t *testing.T) {
			t.Setenv("BIBLETEXT_DESKTOP_TABS", tc.env)
			if got := desktopNav(); got != tc.want {
				t.Errorf("desktopNav() = %v, want %v", got, tc.want)
			}
			if got := compactNavRail(nil); got != tc.wantRail {
				t.Errorf("compactNavRail() = %v, want %v", got, tc.wantRail)
			}
		})
	}
}

// The coupling itself, stated once: if the navigation draws as a rail, the
// compact layout must be the layout. A rail inside a sidebar layout is not a
// thing that exists, and the two questions drifting apart is exactly how the
// app came to show the old sidebar while claiming to show the rail.
func TestRailImpliesTheCompactLayoutIsOn(t *testing.T) {
	for _, env := range []string{"", "0", "1", "bar", "rail", "sidebar", "nonsense"} {
		t.Setenv("BIBLETEXT_DESKTOP_TABS", env)
		if compactNavRail(nil) && desktopNav() == desktopNavSidebar {
			t.Errorf("with %q the navigation draws as a rail but the desktop keeps the "+
				"sidebar layout — the rail is unreachable", env)
		}
	}
}

// The default is the rail, and it is reached without help. A regression that
// restored the sidebar default would leave every other test in this file
// passing, because they all set the variable.
func TestDesktopDefaultsToTheRailWithNoEnvironment(t *testing.T) {
	t.Setenv("BIBLETEXT_DESKTOP_TABS", "")
	if got := desktopNav(); got != desktopNavRail {
		t.Errorf("with no override the desktop draws %v, want the rail", got)
	}
	if !compactNavRail(nil) {
		t.Error("with no override the navigation does not draw as a rail")
	}
}
