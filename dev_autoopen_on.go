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

// devAutoReadAlong drives the narration wash across a noted verse, so the one
// thing the S2 wash model exists for can actually be SEEN in a simulator.
//
// The scenario is three states of one verse: a note's wash on it (BEFORE), the
// narration sitting on top of it (DURING), and the narration gone past with the
// note's wash back (AFTER). None of it is reachable from a script otherwise —
// simctl cannot tap, and the recorded audio the read-along normally rides on
// does not stream in a simulator run. So this opens a real shared link (the same
// HandleShareLink an OS-delivered universal link calls) and then steps
// readAlongHighlight by hand, which is exactly what the audio controller's time
// observer does with a recording playing.
//
//	SIMCTL_CHILD_BIBLETEXT_DEV_READALONG=1 xcrun simctl launch <udid> uk.co.bibletext
//
// THE MARK CAN BE A RANGE, and on the defect that shipped it had to be: set the
// variable to "lo-hi" (e.g. BIBLETEXT_DEV_READALONG=35-37) and the shared link
// carries a MULTI-verse mark, with the narration walking through its middle. A
// single-verse mark is the one shape that cannot show a band whose INTERIOR is
// painted wrong — no interior — which is exactly how the wash range that
// swallowed the inter-verse break passed its testing. "1" keeps the single-verse
// scenario.
//
// The steps are on a fixed clock, ten seconds apart, so a screenshot burst can
// be correlated against the device log's own timestamps: the narration lands two
// verses BEFORE the noted one, then ON it, then past it, then stops altogether.
// Each move prints "bibletext-perf: readalong-verse N" when BIBLETEXT_PERF is
// set, which is what pins a capture to a step.
func devAutoReadAlong(state *AppState) {
	spec := strings.TrimSpace(os.Getenv("BIBLETEXT_DEV_READALONG"))
	if spec == "" || state == nil {
		return
	}
	const (
		book  = "John"
		chap  = 11
		verse = 35
	)
	lo, hi := verse, verse
	if a, b, ok := strings.Cut(spec, "-"); ok {
		if x, err := strconv.Atoi(strings.TrimSpace(a)); err == nil {
			if y, err := strconv.Atoi(strings.TrimSpace(b)); err == nil && y >= x && x > 0 {
				lo, hi = x, y
			}
		}
	}
	at := func(d time.Duration, f func()) { time.AfterFunc(d, func() { fyne.Do(f) }) }
	at(1500*time.Millisecond, func() {
		// The link names the version already loaded, so the scenario exercises
		// the wash and not a translation download.
		HandleShareLink(state, ShareLinkURLWithNote(state.currentVersion().ID,
			book, chap, lo, hi,
			"Read this at the hospital this morning and thought of you."))
	})
	at(6*time.Second, func() { readAlongHighlight(lo-2, true) })
	at(16*time.Second, func() { readAlongHighlight(lo, true) })
	// The last marked verse: over a RANGE this leaves the run's interior verses
	// restored by the live mutation while the ones the import painted are
	// untouched, which is the frame where a wrong repaint shows as one band in
	// two golds (or with a full-width tail on the interior verse).
	at(26*time.Second, func() { readAlongHighlight(hi, true) })
	// A BODY REBUILD WITH THE NARRATION LIVE — what hiding or deleting a note,
	// flipping the theme or changing the text size does mid-playback. The import
	// starts from a fresh attributed string with no wash anywhere on it, and the
	// narration has to come back with it rather than wait for the next verse tick.
	at(30*time.Second, func() { devForceBodyRebuild(state) })
	at(36*time.Second, func() { readAlongHighlight(hi+1, true) })
	at(46*time.Second, func() { readAlongClear() })
	// And the other half of the seam, on the same screen: clearing the mark is
	// what the native "Clear highlight" tap does, and it is now a live attribute
	// mutation — the note's gold goes, the text and the reader's scroll position
	// do not move, and the log says tint-mutate rather than html-import.
	at(56*time.Second, func() {
		clearHighlightedVerse(state)
		state.refreshReadingOnly()
	})
}

// devAutoNotesS8 drives the sticker proofs headlessly — the simulator cannot
// tap, so the app does the arriving (and, for S9, the minimizing and the
// searching) itself, on a fixed clock a screenshot burst can be correlated
// against. Every arrival is a real HandleShareLink (the entry point a tapped
// universal link uses) and every verb is the same function the sticker's own
// buttons post through the //export callbacks.
//
//	SIMCTL_CHILD_BIBLETEXT_DEV_NOTES=<scenario> xcrun simctl launch <udid> uk.co.bibletext
//
//	s8         three arrivals on one passage — the S8 count proof, kept
//	s9who      three arrivals → the WHO line reads "Note from Friend · 1 of 3
//	           on this passage" with the words alone in the bubble
//	s9pill     three arrivals, then the reader's minimize → the pill reads
//	           "Notes · 3" instead of hiding the set
//	s9unplaced a WEB Esther note, then a switch to WEBC (Greek Esther, the
//	           incommensurable numbering) → the unplaced-only pill: "1 note
//	           cannot be shown in this translation", no empty bubble
//	s9suppress an arrival, then a foreign mark on the same chapter (goToVerse,
//	           the verse-of-day/Go-to writer) → the sticker stands down to
//	           the pill; then the mark clears → the bubble restores
//	s9trunc    105 arrivals on one verse of BSB Mark 9 plus 9 WEB notes on
//	           Mark 9:44 (a verse the BSB does not have) → the longest
//	           realistic WHO line ("· K of 105 on this passage · 9 not shown
//	           here", 3-digit count) so the fit rule is visible: the FRIEND
//	           half tail-truncates, the counts never do
func devAutoNotesS8(state *AppState) {
	scenario := strings.ToLower(strings.TrimSpace(os.Getenv("BIBLETEXT_DEV_NOTES")))
	if scenario == "" || state == nil {
		return
	}
	link := func(text string) {
		HandleShareLink(state, ShareLinkURLWithNote(state.currentVersion().ID,
			"John", 11, 35, 35, text))
	}
	at := func(d time.Duration, f func()) { time.AfterFunc(d, func() { fyne.Do(f) }) }
	switch scenario {
	case "s9who", "s9pill":
		at(1500*time.Millisecond, func() { link("First note: read this at the hospital this morning.") })
		at(6*time.Second, func() { link("Second note: a second voice on the same verse.") })
		at(12*time.Second, func() { link("Third note: and a third, so the who line has to say 1 of 3.") })
		if scenario == "s9pill" {
			// The reader's minimize — the same verb the sticker's "–" button
			// posts (bibleTextNoteHidden → hideCurrentNote).
			at(18*time.Second, func() { hideCurrentNote(state); state.refreshReadingOnly() })
		}
	case "s9unplaced":
		at(1500*time.Millisecond, func() {
			HandleShareLink(state, ShareLinkURLWithNote("web", "Esther", 4, 1, 1,
				"A note on Esther in the WEB numbering."))
		})
		at(8*time.Second, func() { switchVersion(state, "webc") })
	case "s9suppress":
		at(1500*time.Millisecond, func() { link("This note will stand down while a search result is lit.") })
		// A foreign mark on the same chapter — goToVerseRange is what the
		// verse of the day, cross-references and the Go-to box all call. Verses
		// beside the note's own, so the standing-down pill and the foreign
		// mark share one screenshot frame.
		at(8*time.Second, func() { goToVerseRange(state, "John", 11, 30, 31); state.refreshReadingOnly() })
		// The mark clears (the native "Clear highlight" tap): released, the
		// sticker restores by derivation alone.
		at(16*time.Second, func() { clearHighlightedVerse(state); state.refreshReadingOnly() })
	case "s9trunc":
		// The longest realistic WHO line, from REAL arrivals: a 3-digit
		// placed count and an unplaced tail on one book. Mark 9:44 exists in
		// the WEB and not in the BSB (the span-with-a-hole case), so the WEB
		// notes on it are unplaced while the BSB ones place.
		for i := 0; i < 105; i++ {
			text := "Truncation note " + strconv.Itoa(i+1) + " — filler so every arrival is distinct."
			at(1500*time.Millisecond+time.Duration(i)*120*time.Millisecond, func() {
				HandleShareLink(state, ShareLinkURLWithNote("bsb", "Mark", 9, 43, 43, text))
			})
		}
		for i := 0; i < 9; i++ {
			text := "Unplaced note " + strconv.Itoa(i+1) + " on the verse the BSB does not carry."
			at(16*time.Second+time.Duration(i)*300*time.Millisecond, func() {
				HandleShareLink(state, ShareLinkURLWithNote("web", "Mark", 9, 44, 44, text))
			})
		}
		// End on the BSB chapter so the placed set is on screen with the
		// unplaced tail in the WHO line.
		at(22*time.Second, func() {
			HandleShareLink(state, ShareLinkURLWithNote("bsb", "Mark", 9, 43, 43,
				"The final note — the one the who line counts from."))
		})
	default: // "s8", kept as the arrival-count proof
		at(1500*time.Millisecond, func() { link("First note: read this at the hospital this morning.") })
		at(8*time.Second, func() { link("Second note: a second voice on the same verse.") })
		at(16*time.Second, func() { link("Third note: and a third, so the count has to move again.") })
	}
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
		" id:" + strconv.FormatUint(state.NoteID, 10) +
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
