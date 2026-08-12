//go:build bibletextdev

package bibletext

// Opening a sheet straight from launch, for screenshots. DEVELOPMENT BUILDS
// ONLY — see dev_autoopen_off.go.
//
// WHY IT EXISTS. Verifying a sheet's LAYOUT in the simulator means getting the
// sheet on screen, and the simulator has no tap command: xcrun simctl can boot,
// install, launch and screenshot, but it cannot press anything. Driving the
// Simulator window from outside needs synthetic clicks, which depend on a
// desktop-automation tool being connected and on macOS accessibility being
// granted — neither is guaranteed, and when it went away mid-session there was
// no way to see a sheet at all.
//
// So the app can open one itself. simctl passes SIMCTL_CHILD_-prefixed
// variables through to the app, which makes a screenshot a two-liner:
//
//	SIMCTL_CHILD_BIBLETEXT_DEV_OPEN=settings xcrun simctl launch <udid> uk.co.bibletext
//	xcrun simctl io <udid> screenshot shot.png
//
// This is a screenshot aid, not a test: it proves what a sheet LOOKS like on a
// real device runtime — safe-area insets, the system font stack, the native
// overlay underneath — which is exactly what a host-side canvas capture cannot.

import (
	"os"
	"strings"
	"time"

	"fyne.io/fyne/v2"
)

// devAutoOpenSheet opens the sheet named by BIBLETEXT_DEV_OPEN, once, shortly
// after the reading view is up. The delay lets the first layout settle so the
// capture shows the sheet at its real size rather than mid-build.
func devAutoOpenSheet(state *AppState) {
	name := strings.ToLower(strings.TrimSpace(os.Getenv("BIBLETEXT_DEV_OPEN")))
	if name == "" || state == nil {
		return
	}
	time.AfterFunc(1200*time.Millisecond, func() {
		fyne.Do(func() {
			switch name {
			case "settings":
				showAISettings(state)
			case "goto":
				showGotoPicker(state)
			case "versions":
				showVersionPicker(state)
			case "votd":
				showVerseOfDay(state)
			}
		})
	})
}
