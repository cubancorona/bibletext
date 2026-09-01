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
	"fmt"
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
// picker. TEMPORARY investigation aid for the on-device defect where
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
			"Fixture message alpha beta gamma delta epsilon."))
	})
	at(6*time.Second, func() { readAlongHighlight(lo-2, true) })
	at(16*time.Second, func() { readAlongHighlight(lo, true) })
	// The last marked verse: over a RANGE this leaves the run's interior verses
	// restored by the live mutation while the ones the import painted are
	// untouched, which is the frame where a wrong repaint shows as one band in
	// two colours (or with a full-width tail on the interior verse).
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
	// mutation — the note wash goes, the text and the reader's scroll position
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
//	           in this chapter" with the words alone in the bubble
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
//	           realistic WHO line ("· K of 105 in this chapter · 9 not shown
//	           here", 3-digit count) so the fit rule is visible: the FRIEND
//	           half tail-truncates, the counts never do
//	s10ctx     ONE note on John 11:6 (the second paragraph), so the arrival
//	           clamps near the chapter top and the picture carries the
//	           paragraph ABOVE the card and the paragraph below — the context
//	           framing; s10ctxpill is the same, minimized
//	s10ranges  three notes with DIFFERENT verse ranges inside ONE paragraph
//	           (WEB John 3:14-17; two ranges overlap at v16), then two
//	           next-taps — the wash bands only the open note's range and
//	           moves within the paragraph as the taps walk the set
//	s10next    the SELECTION proof: three arrivals on one passage, then the
//	           count region's next-tap three times through the real //export
//	           (devNoteNextTap → bibleTextNoteNextTapped) — the who line
//	           walks 1 of 3 → 2 of 3 → 3 of 3 → 1 of 3 with the bubble
//	           swapping under it, and with BIBLETEXT_PERF set the log says
//	           tint-mutate between the taps, never html-import
//	s10far     two notes far apart in John 11; cycling proves the viewport
//	           follows the newly selected note in both directions
//	s11own     ONE note the READER WROTE, opened — the verb set that had no
//	           picture. ✕ alone where a received note carries − beside the
//	           bin, and the who line runs a whole button wider because only
//	           one slot is reserved. s11mixed puts an own note and a received
//	           one on the same chapter so the two verb corners sit in one
//	           frame, a next-tap apart
//	s12pills   per-paragraph pills ON A NATIVE: flips the gate, files notes in
//	           two paragraphs of John 11 plus one at chapter scope, and
//	           collapses the set — the styled pane's pill row, drawn by the
//	           native band machinery for the first time
//	s11pill    the PILL-SCROLL case: a note late in John 11, minimized to the
//	           chapter pill, the view sent back to the top, then the pill's
//	           own Restore. The bubble must arrive IN VIEW — it used to expand
//	           hundreds of points below the reader, who saw nothing change
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
	case "s10ctx", "s10ctxpill":
		// THE CONTEXT SHOT: one note framed with the paragraph above it and
		// the paragraph below, expanded and collapsed, on every platform.
		// John 11:6 opens the SECOND paragraph, so the arrival scroll
		// clamps near the top of the chapter and the first paragraph stays on
		// screen above the card — the only way to get that framing on a
		// simulator, which has no scroll command at all.
		at(1500*time.Millisecond, func() {
			HandleShareLink(state, ShareLinkURLWithNote(state.currentVersion().ID,
				"John", 11, 6, 6,
				"A note with the paragraph above it and the paragraph below in view."))
		})
		// Minimize, then (for the expanded shot) restore. The arrival scroll
		// pins the BAND to the top of the pane — right for an arrival, wrong
		// for this picture, because it puts the paragraph above off-screen.
		// Collapsing shrinks the band and settles the scroll higher; restoring
		// leaves the card expanded with that settled position, so the frame
		// carries the paragraph above, the note, and the paragraph below. Both
		// verbs are the ones the sticker's own buttons post.
		at(7*time.Second, func() { hideCurrentNote(state); state.refreshReadingOnly() })
		if scenario == "s10ctx" {
			at(11*time.Second, func() { restoreCurrentNote(state); state.refreshReadingOnly() })
		}
		// …then SCROLL BACK to the paragraph above. The arrival pins the band
		// to the top of the pane, which is right for an arrival and wrong for
		// this picture, which needs the paragraph above the note, the note and
		// the paragraph below in one frame. armReadingRestore is the panes'
		// own one-shot scroll target (the same machinery that reopens a
		// chapter where the reader left off), so this asks for the verse two
		// above the note's own and lets the pane place it.
		at(15*time.Second, func() { armReadingRestore(4, 0, 0) })
	case "linkscroll":
		// The Links-tab state sequence (the
		// compact tab bar is CurrentTab + rebuildWindow — no widget to tap):
		// front the Links tab, open a case on ANOTHER chapter, return to
		// Links, open the noteless John 3 v16 case (the "Unknown fragment
		// keys" URL), then repeat it for the same-chapter arrival. The defect
		// this reproduces: the wash lights v16 but the view does not move to
		// it. Run with BT_SCROLL_DEBUG=1 and screenshot at 8s, 13s and 19s.
		linksTab := func() { state.CurrentTab = 3; rebuildWindow(state) }
		tapCase := func(url string) {
			// The row button's body (dev_links_on.go inApp), verbatim.
			HandleShareLink(state, url)
			state.CurrentTab = 0
			leaveSearchForRead(state, 0)
			rebuildWindow(state)
		}
		noteless := "https://bibletext.co.uk/bsb/john/3/#v16&zz=nonsense&n2=whatever"
		at(2*time.Second, linksTab)
		at(4*time.Second, func() { tapCase(ShareLinkURL("bsb", "Ruth", 1, 16, 16)) })
		at(9*time.Second, linksTab)
		at(11*time.Second, func() { tapCase(noteless) })
		at(15*time.Second, linksTab)
		at(17*time.Second, func() { tapCase(noteless) })
	case "s10ranges":
		// Three notes with DIFFERENT verse ranges inside ONE paragraph — the
		// WEB files John 3:14-17 as a single paragraph, and two of the ranges
		// overlap at verse 16. The proof this scenario captures: the wash
		// always bands the OPEN note's own range and nothing else (one lit
		// span), and the next-taps walk the set with the band moving WITHIN
		// the paragraph.
		ranged := func(lo, hi int, text string) {
			HandleShareLink(state, ShareLinkURLWithNote(state.currentVersion().ID,
				"John", 3, lo, hi, text))
		}
		at(1500*time.Millisecond, func() { ranged(14, 15, "Fixture range message 14-15.") })
		at(6*time.Second, func() { ranged(16, 16, "Fixture range message 16.") })
		at(12*time.Second, func() { ranged(16, 17, "Fixture range message 16-17.") })
		for _, d := range []time.Duration{18 * time.Second, 26 * time.Second} {
			time.AfterFunc(d, func() { devNoteNextTap(state) })
		}
	case "s10next":
		at(1500*time.Millisecond, func() { link("Fixture message alpha beta gamma delta epsilon.") })
		at(6*time.Second, func() { link("Fixture same-range message two.") })
		at(12*time.Second, func() { link("Fixture same-range message three.") })
		// The taps go through the REAL callback (bibleTextNoteNextTapped), raw
		// AfterFunc rather than at(): the export hops to Fyne's goroutine
		// itself, exactly as it does when the native button posts it.
		for _, d := range []time.Duration{18 * time.Second, 26 * time.Second, 34 * time.Second} {
			time.AfterFunc(d, func() { devNoteNextTap(state) })
		}
	case "s11own", "s11mixed":
		// The reader's OWN note, through the store's own writer, then focused
		// the way pressing its pill focuses it. Written on John 11:35 so it
		// shares the fixture passage with the received notes above.
		mine := func(verse int, text string) uint64 {
			n, _ := saveMyNote(appPrefs(), StoredNote{
				VersionID: state.currentVersion().ID,
				Book:      "John", Chapter: 11, VerseLo: verse, Text: text,
			})
			return n.ID
		}
		// Navigate FIRST. The simulator's container keeps whatever chapter the
		// last run left it on, so a note written on John 11 while the app sits
		// on John 3 is a note nobody can see — which is how this scenario's
		// first picture came back showing the previous fixture.
		at(1200*time.Millisecond, func() { navigateToReference(state, "John", 11) })
		at(2500*time.Millisecond, func() {
			id := mine(35, "A note I wrote myself, which I can only put away.")
			state.focusNote(id)
			applyNoteForCurrentChapter(state)
			state.refreshReadingOnly()
		})
		if scenario == "s11mixed" {
			// …and somebody else's, so a next-tap walks from the ✕ corner to
			// the − and bin corner without the chapter changing under it.
			at(9*time.Second, func() { link("Fixture received message beside your own.") })
			time.AfterFunc(16*time.Second, func() { devNoteNextTap(state) })
		}
	case "s12pills":
		notesPillPerParagraph = true
		at(1200*time.Millisecond, func() { navigateToReference(state, "John", 11) })
		at(2500*time.Millisecond, func() {
			HandleShareLink(state, ShareLinkURLWithNote(state.currentVersion().ID,
				"John", 11, 6, 6, "Fixture pills note in the second paragraph."))
		})
		at(6*time.Second, func() {
			HandleShareLink(state, ShareLinkURLWithNote(state.currentVersion().ID,
				"John", 11, 35, 35, "Fixture pills note far below."))
		})
		// Collapse the open note: the set has nothing open, so the specs cover
		// every noted paragraph and the natives draw one pill per band.
		at(10*time.Second, func() { hideCurrentNote(state); state.refreshReadingOnly() })
	case "s11pill":
		// The reader's report: press the pill at the top of John 11 and the
		// bubble expands, but the page does not go to it. Reproduced with the
		// note late in the chapter, the view deliberately sent back to the top
		// while it is collapsed, and then the pill's own verb.
		at(1200*time.Millisecond, func() { navigateToReference(state, "John", 11) })
		at(2500*time.Millisecond, func() {
			HandleShareLink(state, ShareLinkURLWithNote(state.currentVersion().ID,
				"John", 11, 45, 45, "A note late in the chapter, opened from the pill at the top."))
		})
		// Collapse it, then send the view back to the chapter's first verse —
		// which is where a reader who scrolled up would be, and where the pill
		// they are about to press is sitting.
		at(7*time.Second, func() { hideCurrentNote(state); state.refreshReadingOnly() })
		at(10*time.Second, func() { armReadingRestore(1, 0, 0) })
		// …and press it. This is the function the native pill posts.
		at(15*time.Second, func() { restoreCurrentNote(state); state.refreshReadingOnly() })
	case "s10far":
		far := func(verse int, text string) {
			HandleShareLink(state, ShareLinkURLWithNote(state.currentVersion().ID,
				"John", 11, verse, verse, text))
			devTraceNotePlacement(state, "arrival")
		}
		at(1500*time.Millisecond, func() { far(6, "Fixture note near the chapter start.") })
		at(6*time.Second, func() { far(54, "Fixture note near the chapter end.") })
		for _, d := range []time.Duration{12 * time.Second, 20 * time.Second} {
			time.AfterFunc(d, func() { devNoteNextTap(state) })
		}
	case "s9who", "s9pill":
		at(1500*time.Millisecond, func() { link("Fixture message alpha beta gamma delta epsilon.") })
		at(6*time.Second, func() { link("Fixture same-range message two.") })
		at(12*time.Second, func() { link("Fixture same-range message three.") })
		if scenario == "s9pill" {
			// The reader's minimize — the same verb the sticker's "–" button
			// posts (bibleTextNoteHidden → hideCurrentNote).
			at(18*time.Second, func() { hideCurrentNote(state); state.refreshReadingOnly() })
		}
	case "s9unplaced":
		at(1500*time.Millisecond, func() {
			HandleShareLink(state, ShareLinkURLWithNote("web", "Esther", 4, 1, 1,
				"Fixture unplaced message alpha."))
		})
		at(8*time.Second, func() { switchVersion(state, "webc") })
	case "s9suppress":
		at(1500*time.Millisecond, func() { link("Fixture suppressed message alpha.") })
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
			text := "Fixture truncation message " + strconv.Itoa(i+1) + " — distinct."
			at(1500*time.Millisecond+time.Duration(i)*120*time.Millisecond, func() {
				HandleShareLink(state, ShareLinkURLWithNote("bsb", "Mark", 9, 43, 43, text))
			})
		}
		for i := 0; i < 9; i++ {
			text := "Fixture unplaced message " + strconv.Itoa(i+1) + "."
			at(16*time.Second+time.Duration(i)*300*time.Millisecond, func() {
				HandleShareLink(state, ShareLinkURLWithNote("web", "Mark", 9, 44, 44, text))
			})
		}
		// End on the BSB chapter so the placed set is on screen with the
		// unplaced tail in the WHO line.
		at(22*time.Second, func() {
			HandleShareLink(state, ShareLinkURLWithNote("bsb", "Mark", 9, 43, 43,
				"Fixture final count message."))
		})
	default: // "s8", kept as the arrival-count proof
		at(1500*time.Millisecond, func() { link("Fixture message alpha beta gamma delta epsilon.") })
		at(8*time.Second, func() { link("Fixture same-range message two.") })
		at(16*time.Second, func() { link("Fixture same-range message three.") })
	}
}

func devTraceNotePlacement(state *AppState, event string) {
	if state == nil || os.Getenv("BT_SCROLL_DEBUG") == "" {
		return
	}
	fmt.Fprintf(os.Stderr, "[scroll] note %s: anchor=%d force=%v\n",
		event, state.NoteVerseLo, state.forceReposition)
}

// devAppID isolates synthetic desktop note scenarios from the installed app's
// preferences. Mobile simulators already run in disposable app containers.
func devAppID(id string) string {
	if strings.TrimSpace(os.Getenv("BIBLETEXT_DEV_NOTES")) != "" {
		if target := devMimicTarget(); target != "" {
			return id + ".devnotes." + target
		}
		return id + ".devnotes"
	}
	return id
}

// devNoteDebug reports the live note state for on-screen diagnosis. TEMPORARY:
// added while chasing the defect where switching translation to the WEB
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
