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
	// PARK BEFORE ASKING. The "still loading" check has to come first, because
	// every answer the note card offers except the browser one ends in
	// applyShareTarget, and applyShareTarget can do nothing while state.Bible is
	// still nil — it returns at its own guard. Asked in the other order, the card
	// appeared over the loading spinner and BOTH in-app answers dead-ended: the
	// reader was left on the spinner, the link was never parked, and the sender's
	// note was never stored. Nothing is lost by waiting: consumePendingLink asks
	// the same question again the moment the data lands, which it already did for
	// exactly this reason ("the setting can have changed between parking and
	// consuming").
	if state.loadPhase != loadReady {
		// Park it. StartBackgroundLoad consumes this the instant the Bible is
		// ready — before its rebuild, so there is exactly one rebuild and no
		// flash of the wrong chapter.
		state.pendingLink = &target
		state.pendingLinkRaw = rawURL
		state.pendingNoteOpenID = 0 // a real link's park displaces a browser tap's intent
		return true
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
	// A park carrying a SHOW intent came from the notes browser, not from a
	// link (openNote → switchToLinkVersion): re-run the browser's own verb now
	// that the translation is in memory, so the tapped note lands OPEN with
	// its own mark — the generic arrival below would raise a bare hlLinkSpan,
	// which is FOREIGN to the note and suppressed it to the pill. If the note
	// was deleted while the download ran, fall
	// through: the passage still opens, which is all that is left to honour.
	if id := state.pendingNoteOpenID; id != 0 {
		state.pendingNoteOpenID = 0
		for _, n := range readNoteStore(appPrefs()).notes {
			if n.ID == id {
				openNote(state, n)
				return
			}
		}
	}
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
// It SWITCHES to the translation the link names, deliberately. That was not
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
	// Open it in the translation it was WRITTEN against, deliberately. A note is
	// somebody's remark on particular wording.
	//
	// THE TARGET IS NO LONGER ALWAYS OPENABLE. This used to read "the contract
	// only ever names a public-domain id, so the target is always a translation
	// this app can show", and that sentence was the stated reason nothing below
	// had to cope. It stopped being true when /nkjv/ became a real link path: a
	// reader without the NKJV gets a translation they cannot be switched to. What
	// happens then is deliberate and is the whole point of the rule that they
	// SHOULD get a message — the passage still opens, in whatever translation
	// they have, and showLinkVersionUnavailable says so afterwards. The scripture
	// is not the licensed part; leaving them staring at the wrong wording with no
	// explanation is exactly the silent downgrade this replaced.
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
	// Asked HERE but told at the very end: the answer only depends on t and the
	// version we are in, neither of which the navigation below touches, and the
	// message must go up AFTER state.refresh()/surfaceReading — on iOS and
	// Android the native reading overlay floats above the Fyne canvas and
	// surfaceReading would paint straight over a card raised before it.
	unavailable := linkVersionUnavailable(state, t)

	// NOW check the canon — against the translation the link named, if we
	// switched to it. A link may still name a book this canon lacks (a webc
	// deuterocanon link opened where WEBC could not be loaded) or a chapter
	// beyond its end (Greek Daniel 13). Land on the nearest valid thing rather
	// than showing an error: the contract says a bad payload is ignored, never
	// fatal.
	chapters := state.Bible.GetChaptersForBook(t.Book)
	if chapters == 0 {
		// THE BOOK MAY SIMPLY NOT HAVE ARRIVED YET, and on a first install it
		// almost certainly has not. A fresh install opens on the embedded
		// four-book Gospels seed while the complete text downloads, so a link
		// naming anything outside Matthew–John reaches this guard — 62 of the 66
		// books — on precisely the flow shared links exist for: someone is sent a
		// verse, installs the app, and comes back to tap it. Returning here made
		// that tap do nothing at all: no passage, no message, and the sender's
		// note discarded, with no Safari fallback either (share_link_ios.go
		// always reports the link handled).
		//
		// So park it and let triggerFullDownload's success tail apply it when the
		// full text lands, the same way a link that arrives mid-startup waits for
		// StartBackgroundLoad.
		//
		// NOT via pendingLinkVersion: that slot means "waiting for a TRANSLATION
		// to load", and applyLoadedVersion consumes or discards whatever it finds
		// parked there on the next version load. Leaving it empty keeps this park
		// owned solely by the download that can actually satisfy it.
		//
		// LAST TAP WINS. The guard here used to be `state.pendingLink == nil`, so
		// the FIRST link kept the slot and a second tap did nothing at all
		// (B_SEED_SECOND_LINK). A reader who taps A and then B wants B, and a park
		// they cannot see is no reason to ignore them. A queue would be the wrong
		// answer too: they tapped the second link precisely because the first had
		// not opened.
		//
		// What that guard ALSO did, and what this keeps, is refuse to steal a
		// target already waiting on a translation switch — hence the test on
		// pendingLinkVersion rather than on the slot being empty. That target has a
		// different consumer (applyLoadedVersion), and displacing it would strand a
		// link nothing would ever apply.
		if state.seedOnly && state.fullPending && state.pendingLinkVersion == "" {
			replaced := state.pendingLink != nil
			parked := t
			state.pendingLink = &parked
			state.pendingNoteOpenID = 0 // this park is the link's, not a browser tap's
			// AND SAY SO. This branch used to return in silence, which is 62 of the
			// 66 books on a fresh install: the reader taps a shared verse, the app
			// opens, and nothing whatever happens. The "Shared in <translation>"
			// line was even composed above and then thrown away here, so the one
			// clue that the sender's wording differs from theirs went on the floor
			// with everything else. Both go into the card instead.
			heading := "Shared with you"
			if unavailable != "" {
				heading = "Shared in " + unavailable
			}
			showLinkNotice(state, heading, shareTargetReference(t), linkParkedMessage(state, replaced))
		} else {
			// NO PARK CAN HELP THIS ONE, so it needs the other sentence.
			//
			// The branch above is "it hasn't arrived yet"; this is "it isn't
			// coming". The book is genuinely outside the canon the reader has —
			// a deuterocanonical link when WEBC could not be loaded, or a link
			// whose translation switch was declined — and no download in flight
			// will change that, so promising one would be a lie.
			//
			// Without this the guard fell straight through to `return` and the
			// tap did nothing whatever: no passage, no message, and on iOS no
			// browser fallback either. It was the last silent state the
			// enumeration in share_link_flow_test.go could still reach, and
			// leaving it would have made the rest of this batch a half-measure —
			// the reader cannot tell "the app ignored me" from "the app has
			// nothing to show me", and both feel like the tap missed.
			showLinkNotice(state, "Shared with you", shareTargetReference(t),
				linkBookUnavailableMessage(state, t))
		}
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
	// BEFORE the navigation: what would this arrival's foreign hlLinkSpan
	// suppress? (See AppState.suppressionTookOpen — the navigation's derive
	// transiently opens the chapter's note, so a later capture would lie.
	// A link CARRYING a note ends on the note's own mark, and the capture is
	// then never consulted: the consume is guarded on a live foreign mark.)
	state.captureSuppressionTake(t.Book, chapter)
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
	// A bare link — one naming a chapter with no verse — must leave NOTHING
	// behind. The old code here set HasHighlightedVerse=false and zeroed the end
	// verse while leaving the book, chapter and start verse of the PREVIOUS mark
	// in place (X8/GHOST_LOC): inert while the flag said to ignore it, and a trap
	// for the next reader of those fields. clearMark has nothing to leave behind.
	if t.VerseLo > 0 {
		state.setMark(hlLinkSpan, VerseSpan{
			VersionID: t.VersionID,
			Book:      t.Book,
			Chapter:   chapter,
			Lo:        t.VerseLo,
			Hi:        t.VerseHi,
		})
	} else {
		state.clearMark()
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
		// the note already stored on this chapter. addRecentChapter above has
		// already restored that note, and a note-less link must leave it intact.
		if t.Note != "" {
			// Store FIRST, then focus, then the projection: the verbs address
			// the live note by its StoredNote.ID, so the mirror needs the
			// identity the store minted (or found — a re-opened link dedups to
			// the record already held, whose Received and Minimized are
			// preserved).
			// The WIRE's 'v' record outranks the path when it is present
			// (noteStorageTarget): the path is lossy — webc forced for the
			// deuterocanon, an unknown id falling back to web — and the record
			// is the sender saying what they were actually reading.
			stored, remembered := rememberIncomingNote(state, noteStorageTarget(t))
			if remembered {
				// The mark belongs to the NOTE, not to the link.
				//
				// The block above lit the link's verses as hlLinkSpan, which is
				// right for a bare passage link and wrong here: when a link
				// carries a note, the note is WHY those verses are lit. Leaving
				// the mark attributed to the link meant Delete — which clears
				// only what the note owns — walked past it, and the reader was
				// left with a lit verse and no message to explain it: ORPHAN_HL
				// by a new route, caught by the enumeration as four
				// N1-orphan-highlight cells. hlLinkSpan now means "a link's
				// range with no note attached", so the span is cleared here and
				// the projection re-raises it as the note's own.
				state.clearMark()
				// An arrival focuses the arriving note (session), and the
				// projection draws the mirror FROM the plan — the arrival is
				// no longer a second, hand-written mirror writer. One shipped
				// behaviour changes knowingly: re-opening a link whose note
				// the reader has MINIMIZED now shows the chip, not a forced
				// expansion — an explicit minimize is honoured and nothing
				// auto-expands (N5); the store's history was already being
				// preserved.
				// Tapping a link that IS this note is the Show verb, exactly as
				// tapping its chip is: the stored Minimized is cleared before
				// the projection, so the note OPENS. This reverses the S7
				// decision that a re-arrival honours the stored minimize — on
				// a real device that behaviour left the link tap doing nothing
				// visible (the note stayed a chip), which reads as a broken
				// tap, not as a respected preference. N5's rule is that nothing
				// AUTO-expands; a deliberate tap on the note's own link is not
				// automation, it is the most explicit naming of this note the
				// reader has.
				if stored.Minimized {
					setNoteMinimizedByID(appPrefs(), stored.ID, false)
				}
				state.focusNote(stored.ID)
				applyNoteForCurrentChapter(state)
				// The reader tapped a link NAMING VERSES, and the tap must
				// light them even when the projection raised no note's mark —
				// which is exactly what happens when the arriving note is one
				// the reader minimized earlier (the dedup honours the stored
				// Minimized; the chip shows, no note opens, no hlNote is set).
				// Without the fallback below, that re-arrival lights nothing.
				// The span goes on as
				// hlLinkSpan — the link's own range, exactly what that origin
				// means — so the reader's minimize stays honoured AND their
				// tap still lands somewhere visible.
				if !state.hasMark() && t.VerseLo > 0 {
					state.setMark(hlLinkSpan, VerseSpan{
						VersionID: t.VersionID,
						Book:      t.Book,
						Chapter:   chapter,
						Lo:        t.VerseLo,
						Hi:        t.VerseHi,
					})
				}
				if state.NoteID != stored.ID {
					// The note is filed on a passage OTHER than the one the
					// reader landed on (a wire whose b/c outrank the path), so
					// the plan for THIS chapter does not carry it. The reader
					// still tapped a link carrying a message: show it for this
					// session, mirror-only, exactly as before — with the
					// stored identity so the verbs reach the right record.
					state.ActiveNote = t.Note
					state.NoteMinimized = false
					state.NoteVerseLo = t.VerseLo
					state.NoteID = stored.ID
					if t.VerseLo > 0 {
						state.setMark(hlNote, VerseSpan{
							VersionID: t.VersionID,
							Book:      t.Book,
							Chapter:   chapter,
							Lo:        t.VerseLo,
							Hi:        t.VerseHi,
						})
					}
				}
			} else {
				// The store stood the write down (unreadable as a whole). The
				// plan cannot carry what the store refused, and blanking the
				// note would destroy the one copy in front of the reader — so
				// fail OPEN toward showing: the mirror alone carries it for
				// this session, NoteID 0, and the verbs quietly have nothing
				// to reach (still strictly better than the old rebuilt-key
				// delete that could reach the WRONG note, X1).
				state.ActiveNote = t.Note
				state.NoteMinimized = false
				state.NoteVerseLo = t.VerseLo
				state.NoteID = 0
				if t.VerseLo > 0 {
					state.setMark(hlNote, VerseSpan{
						VersionID: t.VersionID,
						Book:      t.Book,
						Chapter:   chapter,
						Lo:        t.VerseLo,
						Hi:        t.VerseHi,
					})
				}
			}
		} else if msg := noteOutcomeMessage(t.NoteOutcome); msg != "" {
			// The link carried a payload this build could not render — a newer
			// format, or damage. TELL the reader, in the note's place, in the
			// banner's quiet non-chrome styling, attributed to nobody, with no
			// call to action and no link (docs/NOTE_WIRE_FORMAT.md rule 5: a
			// silent drop is unrecoverable, but the sender still has the text,
			// so a message lets the recipient say "your note didn't come
			// through"). The passage has already opened above; nothing is
			// stored, and the next navigation clears it (addRecentChapter).
			state.NoteNotice = msg
		}
	} else {
		state.ActiveNote = ""
		state.NoteMinimized = false
		state.NoteVerseLo = 0
		state.NoteID = 0
	}

	state.refresh()
	if state.surfaceReading != nil {
		state.surfaceReading()
	}
	// Every platform now shows the note inline — iOS draws the native sticker
	// beside the passage, the rest put the banner above the reading pane — so
	// the modal arrival card (the pre-banner fallback) would only duplicate
	// what is already on screen and cover the verses it points at.

	// The passage is on screen; NOW say that it is not in the translation the
	// sender used. Last, so the card sits over the passage it is talking about.
	if unavailable != "" {
		showLinkVersionUnavailable(state, unavailable)
	}
}

// noteStorageTarget is the target as the NOTE should be filed, which is not
// quite the target as the PAGE navigates. The payload's own records are
// authoritative for the note's anchor wherever they are present
// (docs/NOTE_WIRE_FORMAT.md): 'v' because the path version is lossy by
// construction, 'b' and 'c' because the path names where the page opens while
// the record names what the sender wrote about, and 'a' because the runs are
// the span the sender actually selected. The fragment's verse span still
// navigates the page — that is not this function's business.
//
// The older store decoded b/c but deliberately did not apply them because it
// filed a note under the chapter the arrival navigated to, and a
// disagreeing wire chapter would have torn the anchor in half. The scrapbook
// store files the whole anchor from this target and reads nothing from the
// navigation — rememberIncomingNote — so the wire's v/b/c/a are now
// authoritative for what is filed.
func noteStorageTarget(t ShareTarget) ShareTarget {
	if t.NoteVersion != "" {
		t.VersionID = t.NoteVersion
	}
	if t.NoteBook != "" {
		t.Book = t.NoteBook
	}
	if t.NoteChapter > 0 {
		t.Chapter = t.NoteChapter
	}
	if t.NoteLo > 0 {
		t.VerseLo, t.VerseHi = t.NoteLo, t.NoteHi
		if t.VerseHi <= t.VerseLo {
			t.VerseHi = 0 // the store's single-verse spelling
		}
	} else if t.NoteBook != "" || t.NoteChapter > 0 {
		// The wire named a book or chapter but NO verse run: a chapter-level
		// note. The verse from the PATH's fragment must not be grafted onto
		// it — the fragment names where the page scrolls, and the wire's
		// silence about verses is the sender's statement that the note is
		// about the chapter. The encoder always emits 'a' alongside a verse
		// span, but a crafted wire can omit it; clear the span rather than file
		// the note under a verse its sender never named.
		t.VerseLo, t.VerseHi = 0, 0
	}
	return t
}

// linkVersionUnavailable names the translation a link asked for when this reader
// cannot be moved to it — "" when there is nothing to say. It is the question
// switchToLinkVersion deliberately does not answer: that function reports
// custody of the navigation, and every "no" it returns looks the same to the
// caller.
//
// Only ONE of those noes deserves a message: a translation this app knows about
// and this reader has not unlocked (today, the NKJV without a key — see
// BibleVersion.canSelect). The others are all silent on purpose:
//
//   - an id we do not recognise at all: a link from a future BibleText, and
//     "your app is too old" is not something we can say accurately. Degrading
//     quietly is pinned by TestUnknownTranslationLinkStillOpens.
//   - the translation we are already in, or one already loaded: nothing happened
//     that a reader needs telling about.
//   - a download in flight (versionLoading, or our own switch parked above):
//     canSelect is true there, and switchVersionInteractive owns the reporting,
//     including its own failure message.
func linkVersionUnavailable(state *AppState, t ShareTarget) string {
	if state == nil || t.VersionID == "" || t.VersionID == state.CurrentVersion {
		return ""
	}
	v, ok := versionByID(t.VersionID)
	if !ok || v.canSelect() {
		return ""
	}
	return v.Name
}

// linkBookUnavailableMessage is what to tell a reader whose link names a book
// their translation does not contain and no download in flight will supply.
//
// Pure, and separate from the rendering, for the reason linkVersionUnavailable
// and linkParkedMessage are: the card needs a canvas and a host test has none,
// so the WORDING lives where it can be asserted. That is not a stylistic
// preference — the enumeration in share_link_flow_test.go decides whether a
// state is a dead end by asking whether anything would be SAID, and it can only
// ask a function. A message that exists solely inside a showLinkNotice call is
// invisible to the proof, which is exactly the mistake that left this branch
// looking unfixed after it had been fixed.
// It answers "" when the book IS present, which makes it a question about the
// state rather than a sentence generator — the same shape linkVersionUnavailable
// has. Without that guard it returned a message for every link, the enumeration
// read every state as "something was said", and the whole test went quietly
// vacuous: 0 blocked out of 48, which is not a result anyone should have
// believed.
func linkBookUnavailableMessage(state *AppState, t ShareTarget) string {
	if state == nil || t.Book == "" || state.Bible == nil {
		return ""
	}
	if state.Bible.GetChaptersForBook(t.Book) > 0 {
		return "" // the reader has this book; nothing to explain
	}
	return t.Book + " isn't in " + state.currentVersion().Name +
		". Try another translation from the version picker — the deuterocanonical books " +
		"are in the World English Bible (Catholic)."
}

// linkParkedMessage is what to tell a reader whose tapped link has been PARKED
// rather than opened: which passage is waiting, and why it is waiting. It
// returns "" when nothing is parked, so a caller may ask unconditionally.
//
// It is a pure function of the state for the same reason linkVersionUnavailable
// is one: the card and the load-error view both need a canvas, and a host test
// has none — so the WORDING is decided here, where it can be asserted, and the
// rendering stays a thin call. (linkVersionUnavailable earned that shape by
// being the only part of the silent-downgrade fix a test could see.)
//
// replaced says a previous park was displaced by this one. It is a parameter
// rather than something read off the state because by the time this is called
// the old target is already gone — the caller is the only one who still knows.
func linkParkedMessage(state *AppState, replaced bool) string {
	if state == nil || state.pendingLink == nil {
		return ""
	}
	ref := shareTargetReference(*state.pendingLink)
	switch {
	case state.loadPhase == loadFailed:
		// Deliberately does not promise WHEN. Nothing consumes this park until a
		// Retry succeeds, and the reader is the one who taps Retry.
		return ref + " is waiting. It will open as soon as the Bible loads."
	case replaced:
		return ref + " is waiting instead — this is the link that will open when the rest of the " +
			"Bible finishes downloading. The one you tapped before it has been replaced."
	default:
		return ref + " isn't in the part of the Bible that has downloaded yet. The rest is still " +
			"arriving, and this passage will open on its own as soon as it does."
	}
}

// deliverShareLink marshals a link from a native callback onto the UI goroutine.
// The iOS delegate callback is already on the main thread; the Android JNI one
// is not, so everything goes through fyne.Do rather than assuming.
// It returns whether the URL is one of OURS — which the caller reports straight
// back to the OS.
//
// THE ANSWER IS AVAILABLE WITHOUT THE HOP, and that is the whole trick. The
// handling has to marshal onto the Fyne UI goroutine, so the native side used to
// return YES unconditionally and throw this answer away: iOS then believed the
// app had handled links it refused — /web/john/ (a book index, matched by the
// "/web/*" component of the association file), /web/psalm/23/, /web/john/three/
// — and never offered Safari. The app foregrounded on whatever chapter it was
// already showing, which reads as the tap having gone somewhere wrong.
//
// But "is this one of ours?" is answered by ParseShareLink, which is pure, cheap
// and needs no UI thread. So it is asked HERE, synchronously, and only the
// handling is dispatched. Invariant I2 in docs/NKJV_FLOW.md.
func deliverShareLink(rawURL string) bool {
	state := activeAIState // the app is single-window; this is the live state
	if state == nil {
		return false
	}
	if _, ok := ParseShareLink(rawURL); !ok {
		return false // not ours: let the OS open it in the browser
	}
	fyne.Do(func() { HandleShareLink(state, rawURL) })
	return true
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
	want := t.VersionID
	if state == nil || want == "" || want == state.CurrentVersion {
		return false
	}
	if state.CurrentVersion == "" {
		return false // we don't know what we're in; never start a download on a guess
	}
	// Two different noes, kept apart on purpose. An id we do not know is a link
	// from a future BibleText: degrade quietly and open the passage where they
	// are. A registered translation this reader has not unlocked is the case that
	// does deserve a message — but the telling belongs to applyShareTarget
	// (linkVersionUnavailable), after the passage is on screen, not to a helper
	// whose job is deciding who navigates.
	v, ok := versionByID(want)
	if !ok || !v.canSelect() {
		return false // nothing to switch to; open it in what the reader has
	}
	if state.versionLoading {
		// PARK BEHIND IT. This used to return false — "another load owns the
		// spinner; don't queue behind it" — and falling through was the whole of
		// two blocked states. For a translation the reader CAN select,
		// linkVersionUnavailable says nothing, so an /nkjv/ link tapped while any
		// other download happened to be running opened the WEB with no message
		// (B_SILENT_DOWNGRADE); and a deuterocanon link, whose book is in no
		// 66-book canon, fell through to the canon check and did nothing at all
		// (B_SILENT_NOTHING). Both looked to the reader like the tap missed.
		//
		// Nothing new is needed to make waiting safe: applyLoadedVersion's tail
		// ALREADY consumes a parked target when the arriving id matches and drops
		// it when it does not. That is exactly the rule we want here — the
		// running load either IS the translation we want (in which case it is the
		// fetch we would otherwise have started) or it is not (in which case the
		// target is stale by the time it lands). So record the id and let that
		// decide, rather than starting a second fetch or guessing now.
		//
		// The trade is the same one the doc comment above states for our own
		// fetch: if the running load fails, applyLoadedVersion never runs and the
		// target is dropped rather than applied. A failed download must not yank
		// the reader somewhere.
		parked := t
		state.pendingLink = &parked
		state.pendingLinkVersion = want
		// A fresh park starts with no Show intent: openNote re-stamps its note
		// id AFTER this returns true; any other caller's park must not inherit
		// a browser tap's stale one.
		state.pendingNoteOpenID = 0
		return true
	}
	_, inMem := state.loadedVersions[want]
	if inMem || v.isTesting() {
		switchVersion(state, want) // synchronous; fall through and apply
		return false
	}
	// A real fetch: park the target and let the load's apply tail resume it.
	parked := t
	state.pendingLink = &parked
	state.pendingLinkVersion = want
	state.pendingNoteOpenID = 0 // as above: openNote re-stamps after the return
	switchVersionInteractive(state, want)
	return true
}
