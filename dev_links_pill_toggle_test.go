//go:build bibletextdev

package bibletext

import (
	"strings"
	"testing"
)

// The pill toggle must not offer itself on a surface that cannot honour it.
//
// notesPillPerParagraph reaches ONE renderer: the styled pane. iOS, macOS and
// Android hold a single note in their sticker ABI, so there is no list for a
// pill row to come from and the flag changes nothing there. Shipped as a plain
// checkbox in the shared dev panel it appeared on every platform and read as a
// feature that was broken rather than one that was absent.
func TestThePillToggleSaysWhereItWorks(t *testing.T) {
	orig := useStyledPane
	defer func() { useStyledPane = orig }()

	useStyledPane = func() bool { return true }
	on := devPillToggleLabel()
	if strings.Contains(on, "desktop only") {
		t.Errorf("on the styled pane the toggle works, so it must not disclaim: %q", on)
	}

	useStyledPane = func() bool { return false }
	off := devPillToggleLabel()
	if !strings.Contains(off, "desktop only") {
		t.Errorf("on a surface with no pill row the toggle must say so, got %q", off)
	}
	if off == on {
		t.Errorf("the label does not change with the surface: %q", off)
	}
}
