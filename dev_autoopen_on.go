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
	"strconv"
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

// devAutoSwitchVersion switches translation shortly after launch, so the LIVE
// switch path can be exercised in a simulator — which cannot tap the version
// picker. TEMPORARY investigation aid for the owner's device report that
// switching translation keeps a note's highlight and loses the note itself.
//
//	SIMCTL_CHILD_BIBLETEXT_DEV_SWITCH=web xcrun simctl launch <udid> uk.co.bibletext
func devAutoSwitchVersion(state *AppState) {
	id := strings.ToLower(strings.TrimSpace(os.Getenv("BIBLETEXT_DEV_SWITCH")))
	if id == "" || state == nil {
		return
	}
	time.AfterFunc(6*time.Second, func() {
		fyne.Do(func() {
			switchVersion(state, id)
		})
	})
}

// devNoteDebug reports the live note state for on-screen diagnosis. TEMPORARY:
// added while chasing the owner's report that switching translation to the WEB
// (but not to the BSB) loses a note while keeping its highlight. It shows what
// the app BELIEVES, so a screenshot separates "the state is wrong" from "the
// state is right and the pane did not redraw".
func devNoteDebug(state *AppState) string {
	if state == nil {
		return ""
	}
	if state.ActiveNote == "" {
		return "note:none hl:" + state.mark.Origin.String()
	}
	return "note:" + strconv.Itoa(len(state.ActiveNote)) +
		" in:" + state.NoteVersionID +
		" v:" + strconv.Itoa(state.NoteVerseLo) +
		" min:" + boolMark(state.NoteMinimized) +
		" hl:" + state.mark.Origin.String()
}

func boolMark(b bool) string {
	if b {
		return "y"
	}
	return "n"
}
