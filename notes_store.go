package bibletext

// Where shared notes live once they have arrived.
//
// A note comes in on a link and then belongs to the reader, not to the link:
// it stays on that passage until they delete it, survives relaunch, and comes
// back when they return to the chapter. That is the whole reason this file
// exists rather than the note living in AppState for the life of one screen.
//
// Storage is the same shape reading_state.go uses — one JSON blob in
// fyne.Preferences, a testable core taking a prefStore so unit tests can pass a
// fake, and a nil store meaning "no app running", which makes every call a safe
// no-op. Notes are small and few; there is no reason for anything cleverer.
//
// KEYED BY PASSAGE, one note per version+book+chapter. A second note arriving
// for the same chapter replaces the first: two people sending notes on the same
// passage is a real thing, but two bubbles competing for the same paragraph is
// not a screen anybody wants, and the newest message is the one the reader just
// opened.

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"
)

const prefSharedNotes = "shared.notes"

// SharedNote is a note the reader received, and what it is attached to.
type SharedNote struct {
	VersionID string `json:"v"`
	Book      string `json:"b"`
	Chapter   int    `json:"c"`
	VerseLo   int    `json:"lo,omitempty"`
	VerseHi   int    `json:"hi,omitempty"`
	Text      string `json:"t"`

	// Minimized: the reader collapsed it. The note is kept and so is its verse
	// range — neither the bubble nor its highlight shows until they bring it
	// back, which is what makes minimize different from delete.
	Minimized bool `json:"m,omitempty"`

	// Received is when the note arrived, as a Unix time in seconds. It exists so
	// the browser can offer "newest first", which is what a reader actually wants
	// from a list of messages. omitempty and additive: notes stored before this
	// field existed simply have 0, and sort as the oldest — a wrong position for
	// a handful of old notes is a far better outcome than a migration.
	Received int64 `json:"ts,omitempty"`
}

func noteKey(versionID, book string, chapter int) string {
	return strings.ToLower(versionID) + "|" + book + "|" + strconv.Itoa(chapter)
}

func (n SharedNote) key() string { return noteKey(n.VersionID, n.Book, n.Chapter) }

// notesMax bounds the store. Notes are tiny, but the preferences blob is read
// and written on ordinary navigation, so it must not grow without limit if
// somebody opens hundreds of shared links. The oldest are dropped first.
const notesMax = 200

func readNotes(p prefStore) map[string]SharedNote {
	out := map[string]SharedNote{}
	if p == nil {
		return out
	}
	raw := p.String(prefSharedNotes)
	if raw == "" {
		return out
	}
	var list []SharedNote
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		return out
	}
	for _, n := range list {
		if n.Book == "" || n.Chapter < 1 || strings.TrimSpace(n.Text) == "" {
			continue // a half-written or hand-edited blob must not resurrect junk
		}
		out[n.key()] = n
	}
	return out
}

func writeNotes(p prefStore, notes map[string]SharedNote) {
	if p == nil {
		return
	}
	list := make([]SharedNote, 0, len(notes))
	for _, n := range notes {
		list = append(list, n)
	}
	// A stable order keeps the stored blob byte-identical when nothing changed,
	// which keeps the preferences file from churning on every navigation.
	sort.Slice(list, func(i, j int) bool { return list[i].key() < list[j].key() })
	if len(list) > notesMax {
		list = list[len(list)-notesMax:]
	}
	data, err := json.Marshal(list)
	if err != nil {
		return
	}
	p.SetString(prefSharedNotes, string(data))
}

// saveNote stores (or replaces) the note on a passage.
// noteNow is the clock, indirected so tests can pin it.
var noteNow = func() int64 { return time.Now().Unix() }

func saveNote(p prefStore, n SharedNote) {
	if strings.TrimSpace(n.Text) == "" || n.Book == "" || n.Chapter < 1 {
		return
	}
	notes := readNotes(p)
	// Keep the ORIGINAL arrival time when re-saving a note that is already
	// stored. saveNote is how minimize and restore persist themselves, so
	// stamping unconditionally would shuffle a note to the top of "newest first"
	// every time the reader collapsed it — the list would reorder itself under
	// their hand for no reason they could see.
	if prev, ok := notes[n.key()]; ok && n.Received == 0 {
		n.Received = prev.Received
	}
	if n.Received == 0 {
		n.Received = noteNow()
	}
	notes[n.key()] = n
	writeNotes(p, notes)
}

// loadNote returns the note on a passage, if there is one.
func loadNote(p prefStore, versionID, book string, chapter int) (SharedNote, bool) {
	n, ok := readNotes(p)[noteKey(versionID, book, chapter)]
	return n, ok
}

// deleteNote removes it for good.
func deleteNote(p prefStore, versionID, book string, chapter int) {
	notes := readNotes(p)
	delete(notes, noteKey(versionID, book, chapter))
	writeNotes(p, notes)
}

// setNoteMinimized collapses or restores the note on a passage.
func setNoteMinimized(p prefStore, versionID, book string, chapter int, min bool) {
	notes := readNotes(p)
	k := noteKey(versionID, book, chapter)
	n, ok := notes[k]
	if !ok {
		return
	}
	n.Minimized = min
	notes[k] = n
	writeNotes(p, notes)
}

// --- the live view of all this, for the panes -------------------------------

// applyNoteForCurrentChapter loads whatever note belongs to where the reader is
// now and mirrors it into AppState, so the reading panes have one field to look
// at rather than a store to query. Called on every navigation.
//
// A minimized note restores its own highlight state too: the highlight belongs
// to the note, so it must not come back on its own while the note is collapsed.
func applyNoteForCurrentChapter(state *AppState) {
	if state == nil {
		return
	}
	// Off means off, but it does NOT mean gone: the stored notes stay where they
	// are unless the reader asked for them to be deleted, so switching back on
	// brings them all back.
	if !notesFeatureOn(state) {
		state.ActiveNote = ""
		state.NoteMinimized = false
		state.NoteVerseLo = 0
		return
	}
	n, ok := loadNote(appPrefs(), state.currentVersion().ID, state.CurrentBook, state.CurrentChapter)
	if !ok {
		state.ActiveNote = ""
		state.NoteMinimized = false
		state.NoteVerseLo = 0
		return
	}
	state.ActiveNote = n.Text
	state.NoteMinimized = n.Minimized
	state.NoteVerseLo = n.VerseLo
	if n.Minimized {
		return
	}
	// Never clobber a highlight that is already on this chapter for another
	// reason — arriving by a search result, say. That highlight is what the
	// reader just asked for; the note's is only a default.
	if state.HasHighlightedVerse && state.HighlightedBook == state.CurrentBook &&
		state.HighlightedChapter == state.CurrentChapter {
		return
	}
	if n.VerseLo > 0 {
		state.HighlightedBook = n.Book
		state.HighlightedChapter = n.Chapter
		state.HighlightedVerse = n.VerseLo
		state.HighlightedVerseEnd = n.VerseHi
		state.HasHighlightedVerse = true
	}
}

// rememberIncomingNote stores a note that just arrived on a link.
func rememberIncomingNote(state *AppState, t ShareTarget) {
	if state == nil || strings.TrimSpace(t.Note) == "" {
		return
	}
	saveNote(appPrefs(), SharedNote{
		VersionID: state.currentVersion().ID,
		Book:      t.Book,
		Chapter:   state.CurrentChapter,
		VerseLo:   t.VerseLo,
		VerseHi:   t.VerseHi,
		Text:      t.Note,
	})
}

// hideCurrentNote / dropCurrentNote are what the tap menu's two verbs do, in
// the store as well as on screen — otherwise the note would come back on the
// reader's next visit as though they had never touched it.
func hideCurrentNote(state *AppState) {
	if state == nil || state.ActiveNote == "" {
		return
	}
	state.NoteMinimized = true
	setNoteMinimized(appPrefs(), state.currentVersion().ID, state.CurrentBook, state.CurrentChapter, true)
	clearHighlightedVerse(state)
}

func restoreCurrentNote(state *AppState) {
	if state == nil || state.ActiveNote == "" {
		return
	}
	state.NoteMinimized = false
	setNoteMinimized(appPrefs(), state.currentVersion().ID, state.CurrentBook, state.CurrentChapter, false)
	if n, ok := loadNote(appPrefs(), state.currentVersion().ID, state.CurrentBook, state.CurrentChapter); ok && n.VerseLo > 0 {
		state.HighlightedBook = n.Book
		state.HighlightedChapter = n.Chapter
		state.HighlightedVerse = n.VerseLo
		state.HighlightedVerseEnd = n.VerseHi
		state.HasHighlightedVerse = true
	}
}

func dropCurrentNote(state *AppState) {
	if state == nil {
		return
	}
	deleteNote(appPrefs(), state.currentVersion().ID, state.CurrentBook, state.CurrentChapter)
	state.ActiveNote = ""
	state.NoteMinimized = false
	state.NoteVerseLo = 0
	clearHighlightedVerse(state)
}

// applyNoteOnResume surfaces a stored note for the chapter the app is REOPENING
// into.
//
// It exists because reopening never went through addRecentChapter. The restore
// path sets book and chapter directly (reading_state.go), so nothing called
// applyNoteForCurrentChapter and the chapter the reader last had open came back
// bare — the note only appeared once they navigated away and returned, by which
// point every OTHER chapter's note had shown up correctly. observed in practice, and
// exactly as confusing as it sounds.
//
// REOPENING IS NOT ARRIVING, though, and the difference decides the SCROLL —
// but only the scroll. Arriving on a link should land the reader on the message;
// reopening should land them where they stopped reading, which may be a long way
// past the note.
//
// The first attempt bought that by dropping the note's highlight on resume, so
// it could not capture the scroll. That was the wrong price: the note came back
// bare, a bubble pointing at nothing, and it read as a fault — reported as one.
// A note and the passage it marks are one object; showing half of it is worse
// than either alternative.
//
// So the note is restored WHOLE, and the scroll is settled where it belongs: a
// pending restore now outranks the highlight in the reading panes, and the
// explicit arrivals clear the restore so they still land on their passage. See
// AppState.restore and openSearchResultRange.
func applyNoteOnResume(state *AppState) {
	if state == nil {
		return
	}
	applyNoteForCurrentChapter(state)
}
