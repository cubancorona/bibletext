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

	// --- who it is from -----------------------------------------------------
	//
	// Mine marks a note YOU sent rather than one you were sent. It is the only
	// one of these three the app reads today; the other two are carried,
	// stored and never shown, so that adding a name later is additive rather
	// than a migration. See [redacted-retired-private-reference], "Identity".
	//
	// Own notes live in their own list (notes_mine.go), NOT in this map — the
	// key here holds one note per version|book|chapter, so filing yours beside
	// a friend's would overwrite one of them.
	Mine bool `json:"me,omitempty"`

	// SenderName is what the sender called themselves. RESERVED: there is no
	// name field on the share sheet yet, nothing writes this, and nothing
	// displays it. When it is shown it will be UNTRUSTED text — quoted, length
	// capped, bidi isolated, and never allowed to imitate the app's own voice.
	SenderName string `json:"sn,omitempty"`

	// SenderID is an opaque per-install value. RESERVED. It can group notes
	// from ONE install and can never survive a reinstall or reach a second
	// device, which is why linking two senders is a reader action and not
	// something the app infers.
	SenderID string `json:"sid,omitempty"`
}

func noteKey(versionID, book string, chapter int) string {
	return strings.ToLower(versionID) + "|" + book + "|" + strconv.Itoa(chapter)
}

func (n SharedNote) key() string { return noteKey(n.VersionID, n.Book, n.Chapter) }

// notesMax bounds the store. Notes are tiny, but the preferences blob is read
// and written on ordinary navigation, so it must not grow without limit if
// somebody opens hundreds of shared links.
//
// WHICH notes are dropped is arbitrary, not oldest-first: writeNotes sorts by
// storage key (version|book|chapter, for a byte-stable blob) and keeps the
// TAIL, so the discards are whichever keys sort lowest — every "bsb|…" note
// goes before any "web|…" one, and a note that arrived seconds ago can be
// evicted while a months-old one survives. Received is not consulted here.
// (This comment used to claim oldest-first; it never did that.)
const notesMax = 200

// readNotesChecked returns the stored notes AND whether the stored blob could be
// read at all. The second value is the difference between "you have no notes"
// and "I could not tell what you have", and every writer below depends on it.
//
// WHY IT MATTERS SO MUCH. Every mutation here is a read-modify-write: read the
// whole set, change one entry, write the whole set back. So if a failed read
// answered "no notes" — which is what this function used to do on a parse error
// — the very next save, delete or minimize would serialise that emptiness over
// the top of the reader's entire collection. One unreadable read would become
// permanent, silent, total loss of messages that only ever existed here, and the
// reader's next action would be the thing that destroyed them.
//
// Returning ok=false instead lets the writers stand down and leave the bytes
// alone. A blob we cannot parse today may be perfectly readable to a future
// build, or by hand; an overwritten one is gone. Nothing is shown from an
// unreadable store either way, so refusing to write costs the reader nothing.
func readNotesChecked(p prefStore) (map[string]SharedNote, bool) {
	out := map[string]SharedNote{}
	if p == nil {
		return out, true // no store at all: there is nothing to overwrite
	}
	raw := p.String(prefSharedNotes)
	if raw == "" {
		return out, true // genuinely empty, and safe to write to
	}
	var list []SharedNote
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		return out, false // UNREADABLE — callers must not persist over it
	}
	for _, n := range list {
		if n.Book == "" || n.Chapter < 1 || strings.TrimSpace(n.Text) == "" {
			continue // a half-written or hand-edited blob must not resurrect junk
		}
		out[n.key()] = n
	}
	return out, true
}

// readNotes is the read-only view, for callers that only display or count.
// Anything that goes on to WRITE must use readNotesChecked and honour ok.
func readNotes(p prefStore) map[string]SharedNote {
	notes, _ := readNotesChecked(p)
	return notes
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
	notes, ok := readNotesChecked(p)
	if !ok {
		return // unreadable store: saving one note must not erase the rest
	}
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
	notes := readNotes(p)
	if n, ok := notes[noteKey(versionID, book, chapter)]; ok {
		return n, ok
	}
	return noteFromAnotherTranslation(notes, versionID, book, chapter)
}

// noteFromAnotherTranslation finds a note left on this same PASSAGE under a
// different translation, and returns it renumbered into this one.
//
// A note is a remark about a passage, and the passage is the same passage in
// every translation — but the store is keyed version|book|chapter, so before
// this a note simply vanished when the reader changed translation. That is not
// an exotic case: two people sharing a link routinely read different
// translations, and a reader sharing from a licensed translation gets their own
// note back under a different id, because the URL may only name a published one
// (owner-reported: NKJV note, opened, then invisible on returning to NKJV).
//
// The verse is MAPPED rather than copied. Chapter and verse numbers do not mean
// the same thing across translations — the Romans doxology moves, the Song of
// the Three pushes Daniel 3's tail down by 67 — so carrying a raw number over
// would anchor the note to whatever happened to sit at that number. MapVerse is
// the table built for exactly this, and its verseMapAbsent / verseMapIncommensurable
// answers are the cases where the honest thing is to show nothing: Greek Esther
// is a different book, not a renumbering, and a note on it means nothing here.
func noteFromAnotherTranslation(notes map[string]SharedNote, versionID, book string, chapter int) (SharedNote, bool) {
	// Deterministic order. Two translations can both hold a note on the same
	// passage — the reader was sent one link in the BSB and another in the WEB —
	// and ranging a Go map would then show a different one on different runs,
	// which is the kind of bug that never reproduces when you look for it.
	// Newest wins, because that is what "newest first" already promises the
	// reader everywhere else; the version id breaks ties so the answer is total.
	candidates := make([]SharedNote, 0, len(notes))
	for _, n := range notes {
		candidates = append(candidates, n)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Received != candidates[j].Received {
			return candidates[i].Received > candidates[j].Received
		}
		return candidates[i].VersionID < candidates[j].VersionID
	})

	for _, n := range candidates {
		if n.VersionID == versionID || n.Book != book {
			continue
		}
		// The note's own verse decides which chapter it lands in — mapping the
		// CHAPTER alone is not enough, because a move can cross a chapter
		// boundary (Romans 14:24 becomes 16:25).
		probe := n.VerseLo
		if probe <= 0 {
			probe = 1 // a chapter-level note: ask about the chapter's first verse
		}
		ch, v, res := MapVerse(n.VersionID, versionID, n.Book, n.Chapter, probe)
		if res == verseMapAbsent || res == verseMapIncommensurable || ch != chapter {
			continue
		}
		out := n
		// VersionID is deliberately NOT rewritten to the reader's translation. It
		// says where the note is STORED, and that is the only handle Hide and
		// Delete have on it: a note displayed in the WEB but keyed under the NKJV
		// was being deleted under "web", which deleted nothing — the reader binned
		// somebody's message, watched it disappear, and had it come back on the next
		// navigation. Only the LOCATION is renumbered here.
		out.Chapter = ch
		if n.VerseLo > 0 {
			out.VerseLo = v
			// The span's end maps on its own; a span that loses its end
			// degrades to the single verse it starts at rather than guessing.
			out.VerseHi = v
			if n.VerseHi > n.VerseLo {
				if _, hv, hres := MapVerse(n.VersionID, versionID, n.Book, n.Chapter, n.VerseHi); hres == verseMapExact || hres == verseMapMoved {
					if hv > v {
						out.VerseHi = hv
					}
				}
			}
		}
		return out, true
	}
	return SharedNote{}, false
}

// deleteNote removes it for good.
func deleteNote(p prefStore, versionID, book string, chapter int) {
	notes, ok := readNotesChecked(p)
	if !ok {
		return // see readNotesChecked: deleting one must not delete all
	}
	delete(notes, noteKey(versionID, book, chapter))
	writeNotes(p, notes)
}

// setNoteMinimized collapses or restores the note on a passage.
func setNoteMinimized(p prefStore, versionID, book string, chapter int, min bool) {
	notes, readable := readNotesChecked(p)
	if !readable {
		return // see readNotesChecked: collapsing a bubble must not empty the store
	}
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
		state.NoteVersionID = ""
		return
	}
	n, ok := loadNote(appPrefs(), state.currentVersion().ID, state.CurrentBook, state.CurrentChapter)
	if !ok {
		// The note goes, and so does the highlight the note put there. Clearing
		// only the note left a verse highlighted with nothing to explain it —
		// the reader saw their passage marked and the message gone, and had no
		// way to tell whether they had lost the note or imagined it
		// (owner-reported, switching back to NKJV). A highlight that arrived for
		// any OTHER reason — a search result, a shared link's verse — is not the
		// note's to clear, which is what the guard below tests for.
		// Ownership is RECORDED now, so this is an equality rather than the
		// guess it used to be: "the lit verse equals the note's verse, so the
		// note must have lit it" was true by coincidence whenever a search
		// result or a Go-to landed on the same verse, and that collateral is
		// X10. clearMarkFromNote drops the mark only if hlNote set it.
		state.clearMarkFromNote()
		state.ActiveNote = ""
		state.NoteMinimized = false
		state.NoteVerseLo = 0
		state.NoteVersionID = ""
		return
	}
	state.ActiveNote = n.Text
	state.NoteMinimized = n.Minimized
	state.NoteVerseLo = n.VerseLo
	state.NoteVersionID = n.VersionID // where it really lives; see noteStoreVersion
	if n.Minimized {
		return
	}
	// Never clobber a highlight that is already on this chapter for another
	// reason — arriving by a search result, say. That highlight is what the
	// reader just asked for; the note's is only a default. The origin makes
	// "another reason" exact: a mark the NOTE itself placed on an earlier pass
	// is not another reason, and re-asserting it is harmless.
	if _, here := state.markHere(); here && !state.mark.fromNote() {
		return
	}
	if n.VerseLo > 0 {
		state.setMark(hlNote, n.span())
	}
}

// rememberIncomingNote stores a note that just arrived on a link.
func rememberIncomingNote(state *AppState, t ShareTarget) {
	if state == nil || strings.TrimSpace(t.Note) == "" {
		return
	}
	saveNote(appPrefs(), SharedNote{
		// The LINK's translation, not whatever the reader happens to be in. A
		// note is a remark on particular wording, so it belongs to the
		// translation it was written against — and applyShareTarget now opens
		// the link in that translation, so in the ordinary case these are the
		// same thing. They differ only when the switch could not happen (an
		// unknown id, or a download already in flight), and then storing it
		// under the link's translation is still the truthful answer: the note
		// reappears when the reader is next in that translation, rather than
		// being filed under one it was never about — including the reader who has
		// no NKJV and stays where they are (they are told; see
		// showLinkVersionUnavailable), whose copy of the note is then waiting
		// correctly under nkjv if they ever unlock it.
		// The PATH id is the sender's translation now. It used to be that a note
		// written in NKJV came back on a link saying "web" (licensed ids were
		// kept out of URLs entirely), so it was filed under web and the sender's
		// own note vanished the moment they returned to NKJV — highlight still
		// there, message gone (owner-reported). A "t=" fragment hint was built as
		// the fix and deleted once /nkjv/ could say it in the path.
		VersionID: t.VersionID,
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
// noteStoreVersion is the translation the live note is STORED under — which is
// the one Hide and Delete must address. Falls back to the version being read,
// which is correct for the ordinary case where the note was written against it.
func (s *AppState) noteStoreVersion() string {
	if s == nil {
		return ""
	}
	if s.NoteVersionID != "" {
		return s.NoteVersionID
	}
	return s.currentVersion().ID
}

func hideCurrentNote(state *AppState) {
	if state == nil || state.ActiveNote == "" {
		return
	}
	state.NoteMinimized = true
	setNoteMinimized(appPrefs(), state.noteStoreVersion(), state.CurrentBook, state.CurrentChapter, true)
	// Only the note's own mark. Hiding a note used to put out whatever was lit,
	// so a reader who arrived on a search result and then collapsed a note on
	// the same chapter lost the result they had come for (X10).
	state.clearMarkFromNote()
}

func restoreCurrentNote(state *AppState) {
	if state == nil || state.ActiveNote == "" {
		return
	}
	state.NoteMinimized = false
	setNoteMinimized(appPrefs(), state.currentVersion().ID, state.CurrentBook, state.CurrentChapter, false)
	if n, ok := loadNote(appPrefs(), state.currentVersion().ID, state.CurrentBook, state.CurrentChapter); ok && n.VerseLo > 0 {
		state.setMark(hlNote, n.span())
	}
}

func dropCurrentNote(state *AppState) {
	if state == nil {
		return
	}
	deleteNote(appPrefs(), state.noteStoreVersion(), state.CurrentBook, state.CurrentChapter)
	state.ActiveNote = ""
	state.NoteMinimized = false
	state.NoteVerseLo = 0
	state.NoteVersionID = ""
	// As in hideCurrentNote: the note's mark goes, a search result's or a
	// shared link's stays.
	state.clearMarkFromNote()
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
