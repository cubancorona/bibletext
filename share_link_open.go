package bibletext

// Opening a shared link inside the app.
//
// A tapped bibletext.co.uk verse link arrives here from the platform (iOS
// Universal Link, Android App Link) already parsed by ParseShareLink. This file
// is the platform-independent half: what the app DOES with it.
//
// Two rules govern the design, both learned the hard way:
//
//  1. A link almost always arrives while the Bible is still loading. The app
//     shows its window immediately and parses ~6.4 MB on a background
//     goroutine, and the OS delivers the link within milliseconds of launch —
//     so "arrives before the data" is the COMMON case, not an edge case. A link
//     that arrives early is therefore parked, not dropped, and consumed the
//     moment the data lands.
//
//  2. A cold deep link collides with the saved reading position. Startup arms a
//     one-shot scroll target for wherever the reader left off; if that is still
//     armed when the link's chapter is shown, the app opens the shared verse and
//     then silently scrolls somewhere else. Clearing it is not optional — see
//     applyShareTarget.

import (
	"fyne.io/fyne/v2"
)

// HandleShareLink is the single entry point for a tapped link, called from the
// platform glue on the UI goroutine. It returns false when the URL is not one
// of ours, so the caller can let the system open it in a browser instead.
func HandleShareLink(state *AppState, rawURL string) bool {
	if state == nil {
		return false
	}
	target, ok := ParseShareLink(rawURL)
	if !ok {
		return false
	}
	// A link carrying a note, with notes switched off, is not ours to open
	// unasked — but nor is it ours to silently throw away. Ask: read it in the
	// browser, read the passage without it, or turn notes back on and read it
	// here. Only note-bearing links reach this; a plain shared verse belongs in
	// the app whatever the setting says.
	if target.Note != "" && !notesFeatureOn(state) {
		offerNoteLinkChoice(state, rawURL, target)
		return true
	}
	if state.loadPhase != loadReady {
		// Park it. StartBackgroundLoad consumes this the instant the Bible is
		// ready — before its rebuild, so there is exactly one rebuild and no
		// flash of the wrong chapter.
		state.pendingLink = &target
		state.pendingLinkRaw = rawURL
		return true
	}
	applyShareTarget(state, target)
	return true
}

// consumePendingLink applies a link that arrived during startup. Called once
// the loaded data is in place and BEFORE the startup rebuild, so the link's
// chapter is what that rebuild paints.
func consumePendingLink(state *AppState) {
	if state == nil || state.pendingLink == nil {
		return
	}
	t := *state.pendingLink
	raw := state.pendingLinkRaw
	state.pendingLink = nil
	state.pendingLinkRaw = ""
	// The setting can have changed between parking and consuming (a cold start
	// reads preferences after the link arrives), so ask again here.
	if t.Note != "" && !notesFeatureOn(state) {
		offerNoteLinkChoice(state, raw, t)
		return
	}
	applyShareTarget(state, t)
}

// applyShareTarget navigates to the passage a shared link names.
//

// always so: it used to stay in the reader's translation, because switching can
// mean a multi-megabyte download behind a spinner, possibly on cellular,
// possibly on someone's first sight of the app — and the passage was thought to
// be the same either way.
//
// It is not the same either way, which is what settled it. Measured across all
// 1189 chapters of the per-verse read-along tables (assets/timings/web.json vs
// bsb.json): books and chapters align exactly, and 1177 chapters agree on
// verses. The 12 that do not split two ways:
//
//   - Ten are verses WEB carries and the BSB omits on textual-critical grounds
//     (Matthew 17:21, 18:11, 23:14; Mark 7:16, 9:44, 9:46, 11:26, 15:28;
//     Luke 23:17; John 5:4; Acts 28:29). Every other number in those chapters
//     still names the same verse, so only a link landing ON an omitted verse
//     finds nothing.
//   - TWO are genuine renumbering, and a link across them points at the wrong
//     text rather than at nothing: the Romans doxology. WEB has Romans 14:24-26
//     and ends chapter 16 at v24; the BSB has neither and places those verses at
//     Romans 16:25-27.
//
// So opening a link in the reader's own translation is safe everywhere except
// Romans 14 and 16 — where it is not merely imprecise but points at DIFFERENT
// TEXT. Rather than special-case two chapters, the link opens where it was
// written, and the note it carries is stored under that translation too
// (rememberIncomingNote), so a note can never be filed against wording it was
// never about.
//
// The download objection is answered by deferring, not by refusing:
// switchToLinkVersion parks the target and lets switchVersionInteractive's
// spinner own the fetch, and applyLoadedVersion resumes us before its rebuild —
// one rebuild, landing on the shared passage. A translation already in memory
// switches synchronously and costs nothing.
//
// (A deuterocanonical link names webc — share_link.go forces that — so the
// switch above is what makes it openable at all. It only fails if WEBC itself
// cannot be loaded, and then the canon check below leaves the reader put.)
func applyShareTarget(state *AppState, t ShareTarget) {
	if state == nil || state.Bible == nil {
		return
	}

	// note is somebody's remark on particular wording, and the shared-link
	// contract only ever names a public-domain id (web/bsb/webc), so the target
	// is always a translation this app can show.
	//
	// This runs BEFORE the canon check below, and the order is load-bearing:
	// ShareLinkURLWithNote forces version=webc for any deuterocanonical book, so
	// a shared Tobit link is one this app can honour — by switching to WEBC.
	// Checking the book against the canon the reader HAPPENS to be in first made
	// such a link do nothing at all: no passage, no note, no message, and
	// HandleShareLink still reported success so the OS never offered the browser
	// either.
	//
	// It can still mean a download, which is why this parks rather than blocks:
	// switchVersionInteractive puts the fetch behind its own spinner, and
	// applyLoadedVersion resumes us by consuming the parked target BEFORE its
	// rebuild — so the reader sees one rebuild landing on the shared passage,
	// not a flash of the old chapter first. A translation already in memory
	// switches synchronously and falls straight through.
	if switchToLinkVersion(state, t) {
		return
	}

	// NOW check the canon — against the translation the link named, if we
	// switched to it. A link may still name a book this canon lacks (a webc
	// deuterocanon link opened where WEBC could not be loaded) or a chapter
	// beyond its end (Greek Daniel 13). Land on the nearest valid thing rather
	// than showing an error: the contract says a bad payload is ignored, never
	// fatal.
	chapters := state.Bible.GetChaptersForBook(t.Book)
	if chapters == 0 {
		return
	}
	chapter := t.Chapter
	if chapter < 1 {
		chapter = 1
	}
	if chapter > chapters {
		chapter = chapters
	}

	if state.window != nil {
		if c := state.window.Canvas(); c != nil {
			c.Unfocus()
		}
	}
	selectBook(state, t.Book, false)
	state.CurrentChapter = chapter
	addRecentChapter(state, t.Book, chapter)
	// A tapped link always places the view, even when it names the passage
	// already on screen — re-opening the same link is how a reader goes BACK to
	// the note after scrolling away (see AppState.forceReposition).
	state.forceReposition = true
	state.restore = nil // an arrival outranks "where you left off"; see openSearchResultRange

	// Highlight the shared verses. A range uses the same inclusive model the
	// app's own search highlight uses, so the web page and the app light up the
	// same verses.
	if t.VerseLo > 0 {
		state.HighlightedBook = t.Book
		state.HighlightedChapter = chapter
		state.HighlightedVerse = t.VerseLo
		state.HighlightedVerseEnd = t.VerseHi
		state.HasHighlightedVerse = true
	} else {
		state.HasHighlightedVerse = false
		state.HighlightedVerseEnd = 0
	}

	state.IsSearching = false
	state.CanReturnToSearchResults = false
	// Belt-and-braces, and the invariant matters more than this line: startup
	// arms a one-shot scroll target for last session's position, and on a cold
	// link that target is still pending in this very rebuild — the overlay would
	// show the shared chapter and then scroll away from it, silently. The
	// addRecentChapter call above ALREADY clears it (state.go, "plain
	// navigation lands at the top of the new chapter"), so this is insurance
	// against that ordering changing, not the thing doing the work.
	state.restore = nil

	// The sender's note, BEFORE the render. addRecentChapter above already asked
	// the store what note belongs here, and for a link arriving with a new one
	// that answer is "none" — so this has to overwrite it and be in place before
	// refresh(), or the first view of a freshly opened link shows no note and
	// only a later navigation reveals it.
	// With notes switched off the app still opens the passage the link names and
	// simply has nothing to do with the message attached to it: not shown, not
	// stored. (The switch cannot stop the link reaching the app at all — that is
	// settled by the entitlement and the manifest at build time.)
	if notesFeatureOn(state) {
		// Only a link that actually CARRIES a note may change what's shown: a
		// bare passage link (or one whose payload didn't decode) must not hide
		// the note already stored on this chapter — addRecentChapter above has
		// just put that stored note in place, and it stays (platform reproduction: a
		// note-less link blanked the saved note's banner).
		if t.Note != "" {
			state.ActiveNote = t.Note
			state.NoteMinimized = false
			state.NoteVerseLo = t.VerseLo
			rememberIncomingNote(state, t)
		}
	} else {
		state.ActiveNote = ""
		state.NoteMinimized = false
		state.NoteVerseLo = 0
	}

	state.refresh()
	if state.surfaceReading != nil {
		state.surfaceReading()
	}
	// Every platform now shows the note inline — iOS draws the native sticker
	// beside the passage, the rest put the banner above the reading pane — so
	// the modal arrival card (the pre-banner fallback) would only duplicate
	// what is already on screen and cover the verses it points at.
}

// deliverShareLink marshals a link from a native callback onto the UI goroutine.
// The iOS delegate callback is already on the main thread; the Android JNI one
// is not, so everything goes through fyne.Do rather than assuming.
func deliverShareLink(rawURL string) {
	state := activeAIState // the app is single-window; this is the live state
	if state == nil {
		return
	}
	fyne.Do(func() { HandleShareLink(state, rawURL) })
}

// switchToLinkVersion moves the reader to the translation a shared link names,
// and reports whether the caller should stand down and let the switch finish
// the job.
//
// It returns true when an asynchronous load is in flight and the target is
// parked for applyLoadedVersion to consume. "Custody" is the intent, not a
// guarantee: if that download FAILS, applyLoadedVersion never runs, and the
// parked target is dropped rather than applied — the reader stays where they
// are and the link does not open. That is the deliberate trade (a failed
// download must not yank them somewhere), but it does mean a tapped link can
// end in nothing visible when the network is down mid-switch.
// A synchronous switch — the translation is already in memory — returns false,
// because there is nothing to wait for and the caller should carry straight on.
// So does every reason not to switch at all: same translation already, an
// unknown or not-yet-selectable id, or a download already running for something
// else (parking behind that one would let it apply our target to the wrong
// translation).
func switchToLinkVersion(state *AppState, t ShareTarget) bool {
	if state == nil || t.VersionID == "" || t.VersionID == state.CurrentVersion {
		return false
	}
	if state.CurrentVersion == "" {
		return false // we don't know what we're in; never start a download on a guess
	}
	v, ok := versionByID(t.VersionID)
	if !ok || !v.canSelect() {
		return false // nothing to switch to; open it in what the reader has
	}
	if state.versionLoading {
		return false // another load owns the spinner; don't queue behind it
	}
	_, inMem := state.loadedVersions[t.VersionID]
	if inMem || v.isTesting() {
		switchVersion(state, t.VersionID) // synchronous; fall through and apply
		return false
	}
	// A real fetch: park the target and let the load's apply tail resume it.
	parked := t
	state.pendingLink = &parked
	state.pendingLinkVersion = t.VersionID
	switchVersionInteractive(state, t.VersionID)
	return true
}
