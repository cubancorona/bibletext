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

import "fyne.io/fyne/v2"

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
// It deliberately does NOT switch translation to match the link. Switching can
// trigger a multi-megabyte download behind a modal spinner, and doing that
// unasked — possibly on cellular, possibly as someone's first sight of the app
// — is a bad trade for a wording difference: chapter and verse numbering are
// shared across our translations, so the passage itself is correct either way.
// The reader can switch in the version picker if they want the exact wording.
// (A deuterocanonical link is the one case with no fallback; those books simply
// aren't in the loaded canon, and selectBook leaves the reader where they are.)
func applyShareTarget(state *AppState, t ShareTarget) {
	if state == nil || state.Bible == nil {
		return
	}
	// A link may name a book this canon doesn't have (Tobit in a 66-book
	// translation) or a chapter beyond its end (Greek Daniel 13). Land on the
	// nearest valid thing rather than showing an error: the contract says a bad
	// payload is ignored, never fatal.
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
		state.ActiveNote = t.Note
		state.NoteMinimized = false
		state.NoteVerseLo = t.VerseLo
		if t.Note != "" {
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
	// Platforms with a native sticker draw the note beside the passage; the rest
	// fall back to a card, which is a real surface rather than nothing.
	if t.Note != "" && notesFeatureOn(state) && !notePaneHasSticker() {
		showSharedNote(state, t.Note)
	}
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
