package bibletext

// notes_action.go — ONE verb entry point, which can name WHICH note.
//
// Every press on a note card runs one of four verbs, and today each has its own
// parameterless entry point on each surface: bibleTextNoteHidden and its three
// siblings on the Apple panes, btaNoteHidden and its three on Android, closures
// on the styled pane. Parameterless is only correct while there is exactly ONE
// card, because "the note" can then only mean the focused one.
//
// With a card per paragraph a press has to say which. That is the state
// machine's own second rule — "every verb must reach the object the reader
// aimed it at" — and it is the half a parameterless verb cannot keep.
//
// So the verb and its target are one call. The existing entry points become
// this call with noteKeyFocused, which is exactly what they have always meant,
// and nothing about them changes.

type noteAction uint8

const (
	// noteActionHide: the card comes down and the note is KEPT. The reader can
	// bring it back; the highlight goes with it.
	noteActionHide noteAction = iota
	// noteActionRestore: a collapsed marker is pressed. The note comes back and
	// its highlight with it.
	noteActionRestore
	// noteActionDelete: the note goes for good — EXCEPT on the reader's own,
	// where the same press only dismisses (dropCurrentNote says why).
	noteActionDelete
	// noteActionNext: the counts region is pressed; focus advances through the
	// chapter's notes in the plan's stable order.
	noteActionNext
)

func (a noteAction) String() string {
	return [...]string{"hide", "restore", "delete", "next"}[a]
}

// noteKeyFocused is "whichever note the plan has chosen" — the only thing a
// parameterless press can mean, and what every existing entry point means.
//
// It is deliberately NOT zero: zero is a plausible group key, and a caller that
// forgot to pass one would then silently address group 0. chapterTopGroup is
// -1, so this sits below every real key.
const noteKeyFocused = -2

// performNoteAction runs one verb against one target.
//
// A key other than noteKeyFocused FOCUSES THAT GROUP FIRST, so the verb acts on
// the note the reader aimed at rather than on whatever the plan had chosen. The
// order matters and is the whole point: focusing after the verb would delete
// one note and open another.
//
// noteActionNext ignores the key. Advancing is a walk through the whole
// chapter's notes in one stable order, not an operation on a group — the reader
// pressed "next", not "next within this paragraph".
func performNoteAction(state *AppState, verb noteAction, key int) {
	if state == nil {
		return
	}
	if key != noteKeyFocused && verb != noteActionNext {
		focusNoteAtGroup(state, key)
	}
	switch verb {
	case noteActionHide:
		hideCurrentNote(state)
	case noteActionRestore:
		restoreCurrentNote(state)
	case noteActionDelete:
		dropCurrentNote(state)
	case noteActionNext:
		advanceNoteFocus(state)
	}
}
