package bibletext

// readingReflowWanted distinguishes a live presentation change from a saved
// placement, including an explicit history visit to the chapter already shown.
func readingReflowWanted(sameChapter, explicitArrival bool, restore *restoreAnchor) bool {
	return sameChapter && !explicitArrival && (restore == nil || restore.Reflow)
}

// androidReadingPushWanted drops a stale chapter restore before deciding whether
// a native import is needed. A no-op refresh must not send an empty restore arm:
// that would discard the native view's live semantic reflow target.
func androidReadingPushWanted(state *AppState, unchanged, explicitArrival bool) bool {
	if r := state.restore; r != nil && (r.Book != state.CurrentBook || r.Chapter != state.CurrentChapter) {
		state.restore = nil
	}
	return state.restore != nil || explicitArrival || !unchanged
}

// androidReadingIntent identifies the current reading request independently of
// anchor values: two visits to the same coordinates are still different visits.
// It and androidReadingDispatch are confined to the Fyne goroutine.
type androidReadingIntent struct {
	state   *AppState
	restore *restoreAnchor
	version string
	book    string
	chapter int
}

func androidReadingIntentFor(state *AppState) androidReadingIntent {
	if state == nil {
		return androidReadingIntent{}
	}
	return androidReadingIntent{state, state.restore, state.CurrentVersion, state.CurrentBook, state.CurrentChapter}
}

type androidReadingDispatch struct {
	next      uint64
	planned   androidReadingIntent
	plannedID uint64
	issued    androidReadingIntent
	issuedID  uint64
}

func (d *androidReadingDispatch) prepare(state *AppState) (androidReadingIntent, uint64) {
	if state != nil && state.forceReposition {
		// A fresh arrival cannot borrow prior carry permission if dispatch fails.
		d.issuedID = 0
	}
	// Every installed page owns its scroll events, even when a replacement
	// activity or a same-grammar render repeats the same reading request.
	d.next++
	d.planned, d.plannedID = androidReadingIntentFor(state), d.next
	return d.planned, d.plannedID
}

func (d *androidReadingDispatch) dispatched(intent androidReadingIntent, id uint64) {
	if id != 0 && id == d.plannedID && intent == d.planned {
		d.issued, d.issuedID = intent, id
	}
}

func (d *androidReadingDispatch) repeatsSavedRestore(state *AppState) bool {
	if state == nil || state.forceReposition || state.restore == nil || state.restore.Reflow {
		return false
	}
	intent := androidReadingIntentFor(state)
	return d.issuedID != 0 && intent == d.issued
}

// readerScrolled rejects an event queued by an older native page, including
// when a fresh restore was assigned before its replacement page was dispatched.
func (d *androidReadingDispatch) readerScrolled(state *AppState, id uint64) bool {
	if state == nil || state.forceReposition || id == 0 || id != d.issuedID || id != d.plannedID ||
		androidReadingIntentFor(state) != d.issued || d.planned != d.issued {
		return false
	}
	state.restore = nil
	// The same native page may report more genuine scrolls without a render.
	d.issued.restore = nil
	d.planned.restore = nil
	return true
}

// A recreated view may not have measured the saved target yet. Keep its
// original request; a valid parked native point can still carry live reflow.
func androidExistingRestoreCanRecover(state *AppState) bool {
	return state != nil && !state.forceReposition && state.restore != nil &&
		state.restore.Book == state.CurrentBook && state.restore.Chapter == state.CurrentChapter
}
