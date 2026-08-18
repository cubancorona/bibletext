package bibletext

import (
	"sort"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func TestNoteReference(t *testing.T) {
	for _, tc := range []struct {
		n    StoredNote
		want string
	}{
		{StoredNote{Book: "John", Chapter: 11, VerseLo: 35, VerseHi: 35}, "John 11:35"},
		{StoredNote{Book: "John", Chapter: 11, VerseLo: 35}, "John 11:35"},
		{StoredNote{Book: "Psalms", Chapter: 23, VerseLo: 1, VerseHi: 4}, "Psalms 23:1-4"},
		{StoredNote{Book: "Psalms", Chapter: 23}, "Psalms 23"}, // whole chapter
	} {
		if got := noteReference(tc.n); got != tc.want {
			t.Errorf("noteReference(%+v) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

// Reading order, not the alphabetical-by-key order the blob is written in.
func TestSortedNotesUsesCanonOrder(t *testing.T) {
	order := map[string]int{"Genesis": 0, "Psalms": 18, "John": 42, "Revelation": 65}
	notes := map[string]StoredNote{
		"a": {Book: "Revelation", Chapter: 22, Text: "x"},
		"b": {Book: "Genesis", Chapter: 1, Text: "x"},
		"c": {Book: "John", Chapter: 11, Text: "x"},
		"d": {Book: "John", Chapter: 3, Text: "x"},
		"e": {Book: "Psalms", Chapter: 23, VerseLo: 5, Text: "x"},
		"f": {Book: "Psalms", Chapter: 23, VerseLo: 1, Text: "x"},
	}
	got := sortedNotes(notesSlice(notes), order, sortBook)
	want := []string{"Genesis 1", "Psalms 23:1", "Psalms 23:5", "John 3", "John 11", "Revelation 22"}
	if len(got) != len(want) {
		t.Fatalf("got %d notes, want %d", len(got), len(want))
	}
	for i, w := range want {
		if ref := noteReference(got[i]); ref != w {
			t.Errorf("position %d: got %q, want %q", i, ref, w)
		}
	}
}

// A note taken in a wider canon must still be listed when the reader is in a
// narrower one — sorted to the end, never dropped.
func TestSortedNotesKeepsBooksTheCanonLacks(t *testing.T) {
	order := map[string]int{"Genesis": 0, "John": 42}
	notes := map[string]StoredNote{
		"a": {Book: "Tobit", Chapter: 4, Text: "x"},
		"b": {Book: "John", Chapter: 3, Text: "x"},
	}
	got := sortedNotes(notesSlice(notes), order, sortBook)
	if len(got) != 2 {
		t.Fatalf("a note was dropped: %d of 2", len(got))
	}
	if got[0].Book != "John" || got[1].Book != "Tobit" {
		t.Errorf("expected the unknown book last, got %s then %s", got[0].Book, got[1].Book)
	}
}

func TestMatchNotes(t *testing.T) {
	notes := []StoredNote{
		{Book: "John", Chapter: 11, VerseLo: 35, Text: "Read this at the hospital"},
		{Book: "Psalms", Chapter: 23, VerseLo: 1, VerseHi: 4, Text: "Got me through the night"},
	}
	for _, tc := range []struct {
		query string
		want  int
		why   string
	}{
		{"", 2, "an empty query browses everything"},
		{"hospital", 1, "matches the message"},
		{"HOSPITAL", 1, "case-insensitive"},
		{"john", 1, "matches the reference's book"},
		{"psalms 23", 1, "matches a full reference"},
		{"23:1-4", 1, "matches a verse range as written"},
		{"  night  ", 1, "query is trimmed"},
		{"leviticus", 0, "no match is no match"},
	} {
		if got := len(matchNotes(notes, tc.query)); got != tc.want {
			t.Errorf("matchNotes(%q) = %d, want %d — %s", tc.query, got, tc.want, tc.why)
		}
	}
}

// The mode resolver is the single place that knows the precedence, and it must
// validate each flag against whether its feature is actually on — otherwise a
// mode left over from before the reader switched something off strands the tab
// in a mode whose control is no longer on screen.
func TestSearchModeOf(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := psalm23State()
	st.aiKeys = newKeyStoreWith(newFakePrefs())

	setNotesEnabled(true)
	st.aiKeys.setAIEnabled(true)

	if got := searchModeOf(st); got != modeKeyword {
		t.Errorf("default should be keyword, got %v", got)
	}
	st.aiSearchMode = true
	if got := searchModeOf(st); got != modeFind {
		t.Errorf("aiSearchMode should give Find, got %v", got)
	}
	st.NotesMode = true
	if got := searchModeOf(st); got != modeNotes {
		t.Errorf("Notes outranks Find when both flags are set, got %v", got)
	}

	// Turn notes off with the flag still set: must fall back, not strand.
	setNotesEnabled(false)
	if got := searchModeOf(st); got != modeFind {
		t.Errorf("with notes off it should fall back to Find, got %v", got)
	}
	st.aiKeys.setAIEnabled(false)
	if got := searchModeOf(st); got != modeKeyword {
		t.Errorf("with both off it should fall back to keyword, got %v", got)
	}
	setNotesEnabled(true)
}

// With notes off the browser yields nothing, whatever the flag says — the same
// "off means not shown, not gone" rule the bubble follows.
func TestBrowsableNotesRespectsTheSwitch(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	p := fyne.CurrentApp().Preferences()
	addNote(p, StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "John", Chapter: 3, VerseLo: 16, Text: "hi"})

	st := psalm23State()
	st.NotesMode = true

	setNotesEnabled(true)
	if got, total := browsableNotes(st); len(got) != 1 || total != 1 {
		t.Errorf("expected the stored note, got %d of %d", len(got), total)
	}
	setNotesEnabled(false)
	if got, total := browsableNotes(st); len(got) != 0 || total != 0 {
		t.Errorf("notes are off; expected none, got %d of %d", len(got), total)
	}
	setNotesEnabled(true)
	if got, _ := browsableNotes(st); len(got) != 1 {
		t.Errorf("switching back on must bring it back, got %d", len(got))
	}
}

// Opening a note lands on its passage with the WHOLE range highlighted — the
// bug this pins is a range note opening with only its first verse marked.
func TestOpenNoteHighlightsTheWholeRange(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	defer deleteAllNotes(appPrefs())

	stored, ok := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
		Book: "Psalms", Chapter: 23, VerseLo: 1, VerseHi: 4, Text: "n"})
	if !ok {
		t.Fatal("the note was not stored")
	}
	st := psalm23State()
	st.CurrentBook, st.CurrentChapter = "John", 3
	openNote(st, stored)

	if st.CurrentBook != "Psalms" || st.CurrentChapter != 23 {
		t.Errorf("did not navigate: %s %d", st.CurrentBook, st.CurrentChapter)
	}
	if !st.hlOn() || st.hlLo() != 1 || st.hlHi() != 4 {
		t.Errorf("expected 1-4 highlighted, got has=%v %d-%d",
			st.hlOn(), st.hlLo(), st.hlHi())
	}
	// The mark is the NOTE'S OWN, and the note lands OPEN. The old route went
	// through openSearchResultRange, whose hlSearch mark was FOREIGN to the
	// note — the plan stood the chosen note down and the reader landed on the
	// pill (field report). Choosing a note IS the Show verb, everywhere.
	if !st.mark.fromNote() {
		t.Error("the arrival's mark must belong to the note, not to a search")
	}
	if st.NoteID != stored.ID || st.ActiveNote == "" {
		t.Errorf("the chosen note must be on the sticker: id=%d active=%q", st.NoteID, st.ActiveNote)
	}
	if notesSuppressed(st) {
		t.Error("the chosen note must not arrive suppressed to a pill")
	}
}

// The owner's report, exactly: a MINIMIZED note tapped in the list while a
// search mark is still standing must land on the reading pane EXPANDED — not
// as the pill. Both halves of the "sometimes" are here: the stored minimize
// (the Show verb clears it) and the leftover foreign mark (the choice
// displaces it).
func TestBrowserTapAlwaysLandsOpen(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	defer deleteAllNotes(appPrefs())

	stored, ok := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
		Book: "Psalms", Chapter: 23, VerseLo: 2, VerseHi: 3, Text: "n", Minimized: true})
	if !ok {
		t.Fatal("the note was not stored")
	}
	st := psalm23State()
	// A search the reader ran earlier still owns the page.
	goToVerseRange(st, "Psalms", 23, 1, 1)
	if !notesSuppressed(st) {
		t.Fatal("precondition: the foreign mark should stand the notes down")
	}

	openNote(st, stored)

	if st.NoteMinimized || notesSuppressed(st) || st.ActiveNote == "" {
		t.Fatalf("the tapped note must land OPEN: min=%v suppressed=%v active=%q",
			st.NoteMinimized, notesSuppressed(st), st.ActiveNote)
	}
	if st.NoteID != stored.ID {
		t.Errorf("the sticker holds note %d, want the tapped %d", st.NoteID, stored.ID)
	}
}

// A whole-chapter note (no verse) must not claim a bogus highlight on verse 0.
func TestOpenNoteWithNoVerseDoesNotHighlight(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)

	st := psalm23State()
	openNote(st, StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "Psalms", Chapter: 23, Text: "n"})
	if st.hlOn() {
		t.Errorf("a chapter-wide note highlighted verse %d", st.hlLo())
	}
}

// Tapping a minimized note in the list restores it: the reader just asked to see
// that note, so landing on a chapter showing a collapsed marker answers a
// different question.
func TestOpenNoteRestoresAMinimizedOne(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	p := fyne.CurrentApp().Preferences()
	setNotesEnabled(true)
	n := StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "Psalms", Chapter: 23,
		VerseLo: 1, VerseHi: 4, Text: "n", Minimized: true}
	stored, ok := addNote(p, n)
	if !ok {
		t.Fatal("the note was not stored")
	}

	st := psalm23State()
	openNote(st, stored)

	back, ok := noteForChapter(p, "web", "Psalms", 23, nil)
	if !ok {
		t.Fatal("the note vanished")
	}
	if back.Minimized {
		t.Error("opening a minimized note from the list should restore it")
	}
}

// openNote's abort paths write before they bail: the un-minimize (the reader
// asked to SEE this note) lands in the store before the version switch can
// park or the canon check can refuse — so both early returns must end on the
// projection and a repaint, or the reader is left standing in a list whose
// tapped row still says "Minimized in the chapter" over a store that says
// otherwise.
func TestOpenNoteAbortPathsRepaintTheList(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	p := fyne.CurrentApp().Preferences()
	setNotesEnabled(true)
	deleteAllNotes(p)
	defer deleteAllNotes(p)

	storeMinimized := func(id uint64) bool {
		for _, n := range allNotesForBrowsing(p) {
			if n.ID == id {
				return n.Minimized
			}
		}
		t.Fatalf("note %d not in the store", id)
		return false
	}

	st := planTestState(t)
	repaints := 0
	st.showReading = func() { repaints++ }
	st.syncSidebar = func() {}

	// The canon-check refusal: a note whose book the loaded translation does
	// not carry (a webc deuterocanon note read back under a 66-book canon).
	deutero, _ := addNote(p, StoredNote{Kind: noteKindReceived, VersionID: "web",
		Book: "Tobit", Chapter: 4, VerseLo: 15, Text: "deutero", Minimized: true})
	setNoteMinimizedByID(p, deutero.ID, true)
	deutero.Minimized = true

	openNote(st, deutero)

	if st.CurrentBook != "John" {
		t.Fatalf("the absent-book guard must leave the reader in place, moved to %q", st.CurrentBook)
	}
	if storeMinimized(deutero.ID) {
		t.Fatal("precondition: the tap is the Show verb — the store must be un-minimized")
	}
	if repaints == 0 {
		t.Error("the abort left the list standing with the row's stale hidden marker — " +
			"the store write must end on a repaint")
	}

	// The parked-version return: another load owns the spinner, so the target
	// parks behind it — but the un-minimize is already written.
	parked, _ := addNote(p, StoredNote{Kind: noteKindReceived, VersionID: "bsb",
		Book: "John", Chapter: 3, VerseLo: 16, Text: "parked", Minimized: true})
	setNoteMinimizedByID(p, parked.ID, true)
	parked.Minimized = true
	st.versionLoading = true
	repaints = 0

	openNote(st, parked)

	if st.pendingLink == nil || st.pendingLinkVersion != "bsb" {
		t.Fatal("precondition: the target should be parked behind the running load")
	}
	if storeMinimized(parked.ID) {
		t.Fatal("precondition: the store must be un-minimized before the park")
	}
	if repaints == 0 {
		t.Error("the parked return left the list standing with the row's stale hidden marker — " +
			"the store write must end on a repaint")
	}
}

// Newest first is the default, because these are messages: the one that just
// arrived is the one you opened the list for.
func TestSortedNotesNewestFirst(t *testing.T) {
	order := map[string]int{"Genesis": 0, "John": 42}
	notes := map[string]StoredNote{
		"a": {Book: "Genesis", Chapter: 1, Text: "oldest", Received: 100},
		"b": {Book: "John", Chapter: 3, Text: "newest", Received: 300},
		"c": {Book: "John", Chapter: 1, Text: "middle", Received: 200},
	}
	got := sortedNotes(notesSlice(notes), order, sortNewest)
	want := []string{"newest", "middle", "oldest"}
	for i, w := range want {
		if got[i].Text != w {
			t.Errorf("position %d: got %q, want %q", i, got[i].Text, w)
		}
	}
	// Same data, the other order: the canon decides instead.
	got = sortedNotes(notesSlice(notes), order, sortBook)
	wantRefs := []string{"Genesis 1", "John 1", "John 3"}
	for i, w := range wantRefs {
		if ref := noteReference(got[i]); ref != w {
			t.Errorf("book order position %d: got %q, want %q", i, ref, w)
		}
	}
}

// Notes stored before Received existed have a zero stamp. They must still sort
// deterministically rather than shuffling between renders.
func TestSortedNotesIsStableWithoutTimestamps(t *testing.T) {
	order := map[string]int{"Genesis": 0, "John": 42}
	notes := map[string]StoredNote{
		"a": {Book: "John", Chapter: 3, Text: "x"},
		"b": {Book: "Genesis", Chapter: 1, Text: "x"},
		"c": {Book: "John", Chapter: 1, Text: "x"},
	}
	first := sortedNotes(notesSlice(notes), order, sortNewest)
	for i := 0; i < 5; i++ {
		again := sortedNotes(notesSlice(notes), order, sortNewest)
		for j := range first {
			if noteReference(first[j]) != noteReference(again[j]) {
				t.Fatalf("order changed between renders at %d: %q then %q",
					j, noteReference(first[j]), noteReference(again[j]))
			}
		}
	}
}

// Minimizing a note must NOT move it to the top of "newest first" — saveNote is
// how minimize persists itself, so an unconditional stamp would reorder the list
// under the reader's hand.
func TestMinimizingDoesNotRestampANote(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	p := fyne.CurrentApp().Preferences()

	origNow := noteNow
	noteNow = func() int64 { return 1000 }
	defer func() { noteNow = origNow }()
	addNote(p, StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "John", Chapter: 3, VerseLo: 16, Text: "hi"})

	got, ok := noteForChapter(p, "web", "John", 3, nil)
	if !ok || got.Received != 1000 {
		t.Fatalf("expected the arrival stamped at 1000, got %+v", got)
	}

	noteNow = func() int64 { return 9999 } // time passes
	setNoteMinimizedByID(p, got.ID, true)

	got, _ = noteForChapter(p, "web", "John", 3, nil)
	if got.Received != 1000 {
		t.Errorf("minimizing restamped the note: %d, want the original 1000", got.Received)
	}
	if !got.Minimized {
		t.Error("minimize did not stick")
	}
}

// The header line is the answer to "is this everything?", so its counts have to
// be exact and its wording unambiguous.
func TestNotesHeaderLine(t *testing.T) {
	for _, tc := range []struct {
		shown, total int
		query        string
		want         string
	}{
		{0, 0, "", ""},
		{1, 1, "", "Your one note."},
		{7, 7, "", "All 7 notes."},
		{3, 7, "hosp", "3 of 7 notes match “hosp”."},
		{7, 7, "e", "All 7 notes match “e”."},
		{0, 7, "zzz", "No notes match “zzz”."},
		{0, 7, "  zzz  ", "No notes match “zzz”."},
	} {
		if got := notesHeaderLine(tc.shown, tc.total, tc.query); got != tc.want {
			t.Errorf("notesHeaderLine(%d,%d,%q) = %q, want %q",
				tc.shown, tc.total, tc.query, got, tc.want)
		}
	}
}

// The sort choice must survive a relaunch, or it is a control that does nothing.
func TestNotesSortPersists(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	if got := notesSortPref(); got != sortNewest {
		t.Errorf("default should be newest first, got %v", got)
	}
	setNotesSortPref(sortBook)
	if got := notesSortPref(); got != sortBook {
		t.Errorf("book order did not stick, got %v", got)
	}
	setNotesSortPref(sortNewest)
	if got := notesSortPref(); got != sortNewest {
		t.Errorf("newest first did not stick, got %v", got)
	}
}

// A view that takes over the whole results pane has to own its exit. Reaching
// the notes list from Settings → the note count leaves the sidebar's
// Search/Find/Notes control still reading "Search", so it does not look like the
// thing holding the reader there — and there was nothing else to press
// (owner-reported: "no way to go from the notes view back to the reading pane").
func TestClosingTheNotesListReturnsToReading(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)

	st := psalm23State()
	st.CurrentBook, st.CurrentChapter = "Psalms", 23

	showNotesList(st)
	if !st.NotesMode || !st.IsSearching {
		t.Fatalf("the list did not claim the pane: NotesMode=%v IsSearching=%v", st.NotesMode, st.IsSearching)
	}

	closeNotesList(st)
	if st.NotesMode {
		t.Error("Done left the reader in Notes mode")
	}
	if st.IsSearching {
		t.Error("Done left the results pane up, so the reading view never came back")
	}
}

// ...but Done must not throw away a keyword search that was running before the
// reader stepped into Notes. The pane is owed to those results, not to Notes.
func TestClosingTheNotesListKeepsALiveSearch(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)

	st := psalm23State()
	st.CurrentBook, st.CurrentChapter = "Psalms", 23
	st.ActiveSearchQuery = "shepherd"

	showNotesList(st)
	closeNotesList(st)
	if st.NotesMode {
		t.Error("Done left the reader in Notes mode")
	}
	if !st.IsSearching {
		t.Error("Done discarded the keyword results the reader still had running")
	}
}

// notesSlice adapts the map fixtures to sortedNotes, which takes the store's
// list. It assigns deterministic ids by fixture key order, because every real
// note has one and the sort's final tiebreak is the id.
func notesSlice(m map[string]StoredNote) []StoredNote {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]StoredNote, 0, len(m))
	for i, k := range keys {
		n := m[k]
		n.ID = uint64(i + 1)
		if n.Kind == "" {
			n.Kind = noteKindReceived
		}
		out = append(out, n)
	}
	return out
}
