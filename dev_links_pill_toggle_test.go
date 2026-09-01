//go:build bibletextdev

package bibletext

import (
	"strings"
	"testing"
)

// The pill toggle means the same thing on every surface now: the styled pane
// draws the pill row itself, and iOS, macOS and Android draw it through their
// band-spec pushes (bibleTextSetNoteBands and its twins). The old disclaimer
// ("desktop only; this surface draws one sticker") described the years the
// natives held a single note in their sticker ABI — resurfacing it would call
// a working control broken.
func TestThePillToggleSaysWhereItWorks(t *testing.T) {
	orig := useStyledPane
	defer func() { useStyledPane = orig }()

	for _, styled := range []bool{true, false} {
		useStyledPane = func() bool { return styled }
		got := devPillToggleLabel()
		if strings.Contains(got, "desktop only") {
			t.Errorf("styled=%v: every surface honours the toggle now, so it must not disclaim: %q", styled, got)
		}
		if !strings.Contains(got, "Pill per paragraph") {
			t.Errorf("styled=%v: the label must still name the thing it toggles, got %q", styled, got)
		}
	}
}
