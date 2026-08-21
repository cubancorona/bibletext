//go:build !ios && !android

package bibletext

// EVERY VALUE THAT PICKS A PREVIEW MUST ALSO TURN THE PREVIEW ON.
//
// This is the test the bug needed. BIBLETEXT_DESKTOP_TABS had two independent
// readers: one deciding whether the shared compact layout ran at all, one
// deciding how it drew its navigation. "rail" satisfied the second and failed
// the first, so the app fell back to the shipped sidebar while every rendered
// gallery showed the rail — the gallery calls buildCompactUI directly and never
// crosses the gate, so nothing on the render path could catch it.
//
// The assertion below is therefore not "rail maps to rail". It is the coupling:
// a value that selects a navigation style must not leave the desktop on the
// sidebar.

import "testing"

func TestDesktopNavPreviewValues(t *testing.T) {
	for _, tc := range []struct {
		env      string
		want     desktopNavPreview
		wantRail bool
	}{
		{"", desktopNavSidebar, false},
		{"0", desktopNavSidebar, false},
		{"1", desktopNavBar, false},
		{"bar", desktopNavBar, false},
		{"rail", desktopNavRail, true},
	} {
		t.Run("env="+tc.env, func(t *testing.T) {
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
	for _, env := range []string{"", "0", "1", "bar", "rail", "nonsense"} {
		t.Setenv("BIBLETEXT_DESKTOP_TABS", env)
		if compactNavRail(nil) && desktopNav() == desktopNavSidebar {
			t.Errorf("with %q the navigation draws as a rail but the desktop keeps the "+
				"sidebar layout — the rail is unreachable", env)
		}
	}
}
