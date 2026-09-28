package bibletext

// The notes YOU send.
//
// "Share with note" used to keep nothing: the moment the share sheet closed,
// your own words existed only in the messenger thread. They are kept, and
// listed in the notes browser. On the reading page an own note is never a
// chapter's default note, counted with or standing in for the notes others
// sent; it is drawn only while the session's focus names it: opened from the
// browser, arriving on the reader's own link, or just sent (showSentNote,
// below). The next navigation puts it away again.
//
// They live in the ONE scrapbook store as Kind=mine records (notes_store.go).
// They used to be a separate list precisely because the received store was a
// passage-keyed map that would have let your note overwrite a friend's; the
// scrapbook store has no key to collide on, so the separate list folded in —
// saveMyNote / readMyNotes are the Kind=mine reads and writes of that store
// and live beside it.

// noteByline is who the note is from, for the Fyne surfaces (the banner and
// the browser). The PERSON half routes through senderName (notes_byline.go)
// with every other surface, so the dormant name path is one constant away on
// all of them at once — today it can only say "Friend", because
// senderNamesEnabled is false and there is no name field on the share sheet.
func noteByline(n StoredNote) string {
	if n.Kind == noteKindMine {
		return "From you"
	}
	return "From " + senderName(n)
}

// notePassage is the passage a note is written on: the translation, the book
// and the chapter the reader had in front of them when the composer opened.
type notePassage struct {
	versionID string
	book      string
	chapter   int
}

// version is the passage's translation. The id was the reader's own when the
// composer recorded it, so it is always one this build knows; the reader's
// translation stands in only if it somehow is not.
func (p notePassage) version(state *AppState) BibleVersion {
	if v, ok := versionByID(p.versionID); ok {
		return v
	}
	return state.currentVersion()
}

// readerPassage is the passage the reader is on now.
func readerPassage(state *AppState) notePassage {
	return notePassage{
		versionID: state.currentVersion().ID,
		book:      state.CurrentBook,
		chapter:   state.CurrentChapter,
	}
}

// showSentNote puts the note the reader has just sent on the passage, as
// opening it from the notes browser does (openNote): a foreign mark stands
// aside, focus names the note, the projection draws it with its own wash and
// the view is placed on it. Sending is the one moment a reader has no other
// sign that their words were kept, and the browser is several taps away.
// stored is the record the send kept; a zero record, nothing kept, shows
// nothing.
//
// Nothing is stored. Focus is session state, so the note goes away on the
// next navigation exactly as a note opened from the browser does
// (addRecentChapter resets focus), and a window rebuild, which rebuilds the
// page from state and never resets focus, keeps it.
//
// ONLY ON THE PASSAGE IT WAS WRITTEN ON. at is where the composer opened, and
// the send shares and files the note against it (shareVerseLinkWithNote). A
// link arriving while the composer is open can move the reader in between;
// the note is then kept on its own passage, and the plan for the chapter the
// reader was moved to does not hold it. The passage check below refuses what
// the plan would not: the same chapter in another translation, into which the
// plan resolves a note. No send meets that today, since a translation switch
// rebuilds the window and closes the composer; the check holds the rule for
// any call that does. Nor is focus moved to a note this chapter's plan
// does not draw (notes off, or an anchor the chapter cannot place): focus
// naming an absent note falls to the default rule, which would reopen a note
// the reader had closed here (N3). Both are asked when this runs, not when
// the note was saved, so the answer holds however late the call comes.
//
// It reports whether the note was shown.
func showSentNote(state *AppState, stored StoredNote, at notePassage) bool {
	if state == nil || stored.ID == 0 || readerPassage(state) != at {
		return false
	}
	before := state.noteFocus
	state.focusNote(stored.ID)
	if plan := buildChapterPlan(state, appPrefs(), state.Bible); !plan.HasOwn || plan.Own.Note.ID != stored.ID {
		state.noteFocus = before
		return false
	}
	// The note is now the page's reason, as a browser tap, a chip or the pill
	// declares it: a search result's or a link's mark stands aside and the
	// suppression it caused ends with it. Left up, it would stand the note
	// down to its pill, where the browser shows the card.
	if state.mark.live() && !state.mark.fromNote() {
		state.clearMark()
		state.suppressionTookOpen = false
	}
	applyNoteForCurrentChapter(state)
	openedNotePlacesTheView(state)
	// The chapter is the one on screen, so this is a note verb's repaint, not
	// a navigation's, and it is the only thing that puts the note on screen:
	// the composer closed before the send, and nothing after it repaints the
	// reading pane. The styled pane and Android rebuild the pane. The Apple
	// panes refuse the in-place push here: the body fingerprint folds a
	// mirror naming any note but the chapter's display note (the mirror
	// clause in chapterFingerprint, reading.go), and an own note is never
	// that, so they are rebuilt too and the chapter re-imported. The sticker
	// and the import are queued on the main queue behind the share sheet.
	refreshNoteOnly(state)
	return true
}
