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
// The default follows the window; explicit values still pin either edge or
// return to the former sidebar.

import (
	"strings"
	"testing"
)

func TestDesktopNavStyleValues(t *testing.T) {
	for _, tc := range []struct {
		env      string
		want     desktopNavStyle
		wantRail bool
	}{
		// No canvas yet: adaptive navigation starts with the desktop's rail.
		{"", desktopNavAdaptive, true},
		{"nonsense", desktopNavAdaptive, true},
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

// An unsized desktop starts with the rail, then follows its real canvas.
func TestDesktopDefaultsToAdaptiveNavigationWithNoEnvironment(t *testing.T) {
	t.Setenv("BIBLETEXT_DESKTOP_TABS", "")
	if got := desktopNav(); got != desktopNavAdaptive {
		t.Errorf("with no override the desktop draws %v, want adaptive navigation", got)
	}
	if !compactNavRail(nil) {
		t.Error("with no override the navigation does not draw as a rail")
	}
}

// ARCHITECTURE.md's list of the programs gives the desktop window the
// navigation the code gives it. With BIBLETEXT_DESKTOP_TABS unset the mode is
// adaptive (desktopNav), so neither edge is the desktop's default, and the
// edge follows mobileRailWanted's boundary, the one the touch devices' line
// gives them, with a square window on the rail's side. The code's answers are
// the control: a change of default or of boundary brings both lines back to
// be read.
func TestTheArchitectureListGivesTheDesktopTheTouchDevicesRule(t *testing.T) {
	t.Setenv("BIBLETEXT_DESKTOP_TABS", "")
	if got := desktopNav(); got != desktopNavAdaptive {
		t.Fatalf("an unset BIBLETEXT_DESKTOP_TABS gives %v: read ARCHITECTURE.md's line for cmd/bibletext/main.go against it", got)
	}
	if !mobileRailWanted(false, 700, 700) || mobileRailWanted(false, 699, 700) {
		t.Fatal("mobileRailWanted no longer puts a square window on the rail's side and a taller one on the bar's: " +
			"read ARCHITECTURE.md's lines for cmd/bibletext/main.go and cmd/mobile/main.go against it")
	}
	arch := readRepoFile(t, "ARCHITECTURE.md")
	for _, program := range []string{"cmd/bibletext/main.go", "cmd/mobile/main.go"} {
		item := architectureProgramItem(t, arch, program)
		for _, want := range []string{"bottom bar while", "left rail while it is square or wider"} {
			if !strings.Contains(item, want) {
				t.Errorf("ARCHITECTURE.md's line for %s does not say %q:\n%s", program, want, item)
			}
		}
	}
	if desktop := architectureProgramItem(t, arch, "cmd/bibletext/main.go"); strings.Contains(desktop, "by default") {
		t.Errorf("ARCHITECTURE.md's line for the desktop window calls an edge its default:\n%s", desktop)
	}
}

// architectureProgramItem is ARCHITECTURE.md's list item for one program
// under cmd/, white space collapsed.
func architectureProgramItem(t *testing.T, arch, program string) string {
	t.Helper()
	start := strings.Index(arch, "- `"+program+"` — ")
	if start < 0 {
		t.Fatalf("ARCHITECTURE.md lists no %s", program)
	}
	item := arch[start+len("- "):]
	if end := strings.Index(item, "\n- "); end >= 0 {
		item = item[:end]
	}
	if end := strings.Index(item, "\n\n"); end >= 0 {
		item = item[:end]
	}
	return strings.Join(strings.Fields(item), " ")
}
