package bibletext

// refreshNoteOnly is the repaint a NOTE-SELECTION verb earns — Hide, Restore,
// the count region's next-tap, Delete — as opposed to the repaint a chapter
// change earns. It is the narrow end of the verb→screen rule: the smallest
// repaint that can carry the verb, and nothing wider.
//
// WHAT THE OWNER SAW (desktop, 2026-08-19): "when i click a note group with
// multiple notes to cycle to the next note there is a delay and a flash of what
// looks like a missized and misplaced reading pane before it stabilizes."
//
// WHY IT HAPPENED. All four verbs ended in state.refreshReadingOnly(), which is
// showReading() — `readingHost.Objects = []fyne.CanvasObject{buildReadingPane(state)}`
// — a WHOLE NEW Fyne widget tree for the reading column. On the Apple panes the
// scripture is not in that tree at all: it is a native NSTextView (macOS) or
// UITextView (iOS) floating above the canvas, and its frame TRACKS the tree's
// reading host (macReadingHost.Resize/Move → pushFrame → setMacFrameFromObject).
// A rebuild hands that tracker a brand-new host with no geometry yet, so the
// native pane keeps the OLD rect while the new chrome lays out around it — and
// the correction is not immediate: pushFrame's re-assert is a SHARED 60 ms timer
// (one per burst, deliberately, so a layout storm cannot flood it). A stale
// frame for ~60 ms, arriving on a press, is exactly "a delay and a flash …
// before it stabilizes".
//
// The chapter re-import was ALREADY skipped here — the body/tint fingerprint
// gate in newMacReadingHost / pushChapterHTML — which is why the flash was
// geometry and not text. Skipping the re-import INSIDE a rebuild cannot help
// when the rebuild is the thing that moves the pane.
//
// WHAT HAPPENS INSTEAD. A note-selection verb changes exactly two things a
// reader can see: which note the sticker draws, and which verses are washed.
// Both are live mutations on the pane already on screen — pushNoteToPane's
// compare-and-refresh and applyNativeTint's one attribute over a known
// character range, the same two primitives the rebuild's own fast path uses.
// Neither needs a widget. So the verb pushes those two and leaves the tree
// alone, which leaves the frame alone.
//
// IT FALLS BACK, ALWAYS, AND FALLING BACK IS THE OLD BEHAVIOUR EXACTLY.
// refreshNoteInPlace answers false wherever the Fyne tree does carry something
// about the note (the whole Windows/Linux styled-band and Android story, and on
// the Apple panes the NoteNotice banner), and wherever the native pane does not
// already hold this chapter's text. No surface can be lost to this; the worst
// case is a verb that keeps the flash.
func refreshNoteOnly(state *AppState) {
	if state == nil {
		return
	}
	if refreshNoteInPlace(state) {
		return
	}
	state.refreshReadingOnly()
}
