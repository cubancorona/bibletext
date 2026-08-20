package bibletext

import (
	"fyne.io/fyne/v2/widget"
	"sort"
	"strings"
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
		{Book: "John", Chapter: 11, VerseLo: 35, Text: "Read this synthetic note"},
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
	// pill (verification). Choosing a note IS the Show verb, everywhere.
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


// always show it (it seems often still minimized pill)". A browser tap must
// ALWAYS answer, through EVERY route — this is the route table. Each route is
// a way the verify pass confirmed a tap could end at the pill, at nothing, or
// on an invalid chapter; each row asserts the route now lands the way the
// reader asked. Common to all: a browser arrival raises NO results trail

// hit — the way back to the list is the Search tab's Notes mode).
func TestBrowserTapAlwaysLandsOpen(t *testing.T) {
	t.Run("minimized under a leftover foreign mark", func(t *testing.T) {
		// Both halves of the "sometimes": the stored minimize (the Show verb
		// clears it) and the leftover foreign mark (the choice displaces it).
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
		if st.CanReturnToSearchResults {
			t.Error("a browser arrival must raise no results trail")
		}
	})

	t.Run("parked behind a download, then the translation lands", func(t *testing.T) {
		// Report A mechanism 2: the park used to be consumed as a LINK arrival,
		// whose bare hlLinkSpan is foreign to the note — the reader waited out
		// the download and got the pill. The park now remembers the Show
		// intent (pendingNoteOpenID) and the arrival re-runs openNote.
		app := test.NewApp()
		defer app.Quit()
		setNotesEnabled(true)
		deleteAllNotes(appPrefs())
		defer deleteAllNotes(appPrefs())

		stored, ok := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "bsb",
			Book: "John", Chapter: 3, VerseLo: 16, Text: "bsb words", Minimized: true})
		if !ok {
			t.Fatal("the note was not stored")
		}
		st := planTestState(t)
		st.versionLoading = true // another load owns the spinner: openNote parks behind it

		openNote(st, stored)

		if st.pendingLink == nil || st.pendingLinkVersion != "bsb" {
			t.Fatal("precondition: the target should be parked behind the running load")
		}
		if st.pendingNoteOpenID != stored.ID {
			t.Fatalf("the park must remember the Show intent: pendingNoteOpenID=%d, want %d",
				st.pendingNoteOpenID, stored.ID)
		}

		// The download lands. applyLoadedVersion's tail consumes the park —
		// and must open THAT note, not a link-arrival's suppressed pill.
		st.versionLoading = false
		v, vok := versionByID("bsb")
		if !vok {
			t.Skip("bsb not registered")
		}
		bd := NewBibleData()
		bd.PopulateWithSampleVerses()
		applyLoadedVersion(st, v, bd, modeReal)

		if st.CurrentBook != "John" || st.CurrentChapter != 3 {
			t.Fatalf("the arrival did not land on the passage: %s %d", st.CurrentBook, st.CurrentChapter)
		}
		if st.NoteID != stored.ID || st.ActiveNote != "bsb words" {
			t.Fatalf("the arrival must open the tapped note: id=%d active=%q", st.NoteID, st.ActiveNote)
		}
		if st.NoteMinimized || notesSuppressed(st) {
			t.Fatalf("the note arrived collapsed after the wait: min=%v suppressed=%v — "+

		}
		if !st.mark.fromNote() {
			t.Error("the arrival's mark must be the note's own, never a link span")
		}
		if st.pendingNoteOpenID != 0 {
			t.Error("the Show intent must be consumed with the park")
		}
		if st.CanReturnToSearchResults {
			t.Error("a browser arrival must raise no results trail")
		}
	})

	t.Run("a park dropped by another translation drops the intent too", func(t *testing.T) {
		app := test.NewApp()
		defer app.Quit()
		setNotesEnabled(true)
		deleteAllNotes(appPrefs())
		defer deleteAllNotes(appPrefs())

		stored, _ := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "bsb",
			Book: "John", Chapter: 3, VerseLo: 16, Text: "bsb words"})
		st := planTestState(t)
		st.versionLoading = true
		openNote(st, stored)
		if st.pendingNoteOpenID != stored.ID {
			t.Fatal("precondition: the park should carry the Show intent")
		}

		// WEBC arrives instead of the BSB the park was waiting for: the stale
		// target is dropped — and the note id must die with it, or a LATER
		// link's park would open a note nobody just tapped.
		st.versionLoading = false
		other, vok := versionByID("webc")
		if !vok {
			t.Skip("webc not registered")
		}
		bd := NewBibleData()
		bd.PopulateWithSampleVerses()
		applyLoadedVersion(st, other, bd, modeReal)

		if st.pendingNoteOpenID != 0 {
			t.Error("the Show intent survived the park it rode on")
		}
	})

	t.Run("your own note answers with its verses lit", func(t *testing.T) {
		// Report A mechanism 1: mine notes are excluded from the chapter plan

		// no plan surfaces — the reader landed on the other notes' pill, or on
		// nothing. The tap now navigates and lights the note's own range as a
		// Go-to arrival; no received-note bubble is faked.
		app := test.NewApp()
		defer app.Quit()
		setNotesEnabled(true)
		deleteAllNotes(appPrefs())
		defer deleteAllNotes(appPrefs())

		mine, ok := addNote(appPrefs(), StoredNote{Kind: noteKindMine, VersionID: "web",
			Book: "Psalms", Chapter: 23, VerseLo: 1, VerseHi: 2, Text: "sent to neutral contact"})
		if !ok {
			t.Fatal("the note was not stored")
		}
		st := planTestState(t)
		st.forceReposition = false

		openNote(st, mine)

		if st.CurrentBook != "Psalms" || st.CurrentChapter != 23 {
			t.Fatalf("did not navigate: %s %d", st.CurrentBook, st.CurrentChapter)
		}
		sp, live := st.markSpan()
		if !live || sp.Lo != 1 || sp.Hi != 2 {
			t.Fatalf("your note's verses must light on arrival: live=%v span=%+v", live, sp)
		}

		// that a mine note raises hlVerseOfDay and that no bubble is drawn. That
		// was the honest answer while the plan could not draw an own note at
		// all — but it had a consequence nobody had traced. hlVerseOfDay is not
		// fromNote, so notesSuppressed was TRUE, and tapping your own row stood
		// down every note on the chapter: a FRIEND's open note collapsed to a
		// pill because you touched your own. The reader's report ("takes you to
		// the reading pane with the highlight only and no note — a bit
		// misleading") was the smaller half of it.
		//
		// Now your own note is drawn while focus names it, in the plan's own
		// slot, and hidden again when you navigate away. So the mark is the
		// NOTE's, like any other note's, and the bubble is real rather than
		// faked — it carries your own words under "Note from you".
		if !st.mark.fromNote() {
			t.Errorf("your own note must raise its OWN mark now, got origin %v — "+
				"a foreign mark here suppresses every other note on the chapter", st.mark.Origin)
		}
		if st.ActiveNote != mine.Text {
			t.Errorf("your own note must be drawn on the passage the requested behavior see it on: "+
				"active=%q want %q", st.ActiveNote, mine.Text)
		}
		if st.NoteID != mine.ID {
			t.Errorf("the mirror must carry the note's own identity, so the verbs "+
				"address the right record: got %d want %d", st.NoteID, mine.ID)
		}
		if !st.forceReposition {
			t.Error("the arrival must place the view on the lit verses")
		}
		if st.CanReturnToSearchResults {
			t.Error("a browser arrival must raise no results trail")
		}
	})

	t.Run("a chapter this canon lacks is clamped, never landed on raw", func(t *testing.T) {
		app := test.NewApp()
		defer app.Quit()
		setNotesEnabled(true)
		deleteAllNotes(appPrefs())
		defer deleteAllNotes(appPrefs())

		stored, _ := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
			Book: "John", Chapter: 99, VerseLo: 1, Text: "future chapter"})
		st := planTestState(t)

		openNote(st, stored)

		nums := st.Bible.GetChapterNumbersForBook("John")
		valid := false
		for _, c := range nums {
			if c == st.CurrentChapter {
				valid = true
			}
		}
		if !valid {
			t.Fatalf("landed on chapter %d, which %v does not contain — the raw assignment "+
				"persisted an invalid chapter and broke the next launch's restore", st.CurrentChapter, nums)
		}
	})
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

// TAPPING YOUR OWN NOTE MUST NOT TAKE A FRIEND'S AWAY.
//
// The worst part of the old mine branch was invisible in the bug report. It
// raised an hlVerseOfDay mark, whose origin is not fromNote (mark.go), so
// notesSuppressed was true (notes_plan.go) and the Open loop stood down every
// note on the chapter. A reader with a friend's note open on Psalm 23 who
// tapped their OWN note in the list watched their friend's message collapse
// into a pill — caused by the tap, explained by nothing.
//
// This pins the property rather than the mechanism: after tapping your own
// note, a received note on the same passage is still openable.
func TestOwnNoteTapDoesNotSuppressAFriendsNote(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	friend, ok := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
		Book: "Psalms", Chapter: 23, VerseLo: 4, Text: "synthetic note"})
	if !ok {
		t.Fatal("the friend's note was not stored")
	}
	mine, ok := addNote(appPrefs(), StoredNote{Kind: noteKindMine, VersionID: "web",
		Book: "Psalms", Chapter: 23, VerseLo: 1, VerseHi: 2, Text: "sent to neutral contact"})
	if !ok {
		t.Fatal("your note was not stored")
	}

	st := planTestState(t)
	openNote(st, mine)

	// The mark your own note raised is the NOTE's, so nothing is suppressed.
	if notesSuppressed(st) {
		t.Fatal("tapping your own note suppressed the chapter's notes — a friend's " +
			"open note would collapse to a pill because you touched your own")
	}
	// And the friend's note is still there, still openable: focus it and it draws.
	st.focusNote(friend.ID)
	applyNoteForCurrentChapter(st)
	if st.ActiveNote != friend.Text {
		t.Errorf("the friend's note did not come back after your own note was tapped: "+
			"active=%q want %q", st.ActiveNote, friend.Text)
	}
}

// YOUR OWN NOTE IS EPHEMERAL: shown the requested behavior, gone when you move on.
// The mechanism is noteFocus, which navigation already resets (state.go), so
// this asserts the BEHAVIOUR rather than the wiring — if the reset ever moves,
// this still says what the reader is promised.
func TestOwnNoteHidesAgainOnNavigation(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	mine, ok := addNote(appPrefs(), StoredNote{Kind: noteKindMine, VersionID: "web",
		Book: "Psalms", Chapter: 23, VerseLo: 1, VerseHi: 2, Text: "sent to neutral contact"})
	if !ok {
		t.Fatal("your note was not stored")
	}
	st := planTestState(t)

	openNote(st, mine)
	if st.ActiveNote != mine.Text {
		t.Fatalf("your own note was not drawn the requested behavior for it: %q", st.ActiveNote)
	}

	// Navigate away and back. Your own note must not follow you around.
	addRecentChapter(st, "Psalms", 24)
	if st.ActiveNote != "" {
		t.Errorf("your own note followed the reader to another chapter: %q", st.ActiveNote)
	}
	addRecentChapter(st, "Psalms", 23)
	if st.ActiveNote != "" {
		t.Errorf("your own note reappeared unbidden on a later visit: %q — own notes are "+
			"drawn only when asked for, never by simply being on the passage", st.ActiveNote)
	}
}

// TAPPING YOUR OWN LINK GIVES YOU YOUR OWN NOTE — once, and as yours.
//

// into my stored notes as from me. But then if I click my own link it seems to
// get stored again a second time as from friend."
//
// The key is the per-note nonce (share_note.go, noteTagNonce), not the words: a
// friend may write the same sentence on the same verse, and collapsing on
// content would file their message as yours and lose it.
func TestOwnLinkComesHomeAsYours(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	nonce := newNoteNonce()
	if len(nonce) != noteNonceLen {
		t.Fatalf("no nonce minted: %v", nonce)
	}
	mine, ok := addNote(appPrefs(), StoredNote{Kind: noteKindMine, VersionID: "web",
		Book: "Psalms", Chapter: 23, VerseLo: 1, Text: "sent to neutral contact", Nonce: nonce})
	if !ok {
		t.Fatal("your note was not stored")
	}

	// The link you sent, parsed back exactly as tapping it would.
	url := ShareLinkURLWithNoteNonce("web", "Psalms", 23, 1, 1, "sent to neutral contact", nonce)
	target, ok := ParseShareLink(url)
	if !ok {
		t.Fatalf("your own link did not parse: %q", url)
	}
	if target.NoteNonce == ([noteNonceLen]byte{}) {
		t.Fatal("the link lost its nonce on the round trip")
	}

	st := planTestState(t)
	got, stored := rememberIncomingNote(st, target)
	if !stored {
		t.Fatal("the arrival was refused outright")
	}
	if got.ID != mine.ID {
		t.Errorf("tapping your own link created a second record (id %d, yours is %d) — "+
			"your own words would then be shown back to you as a note from a friend",
			got.ID, mine.ID)
	}
	if got.Kind != noteKindMine {
		t.Errorf("your note came home as %q; it is still yours", got.Kind)
	}
	if all := allNotesForBrowsing(appPrefs()); len(all) != 1 {
		t.Errorf("the scrapbook holds %d notes after a round trip of ONE note", len(all))
	}
}

// A FRIEND'S IDENTICAL WORDS ARE STILL THEIR OWN NOTE. The failure mode the
// nonce exists to prevent: short notes repeat ("Amen", "synthetic note"), and
// a content-keyed collapse would fold a real message from a real person into
// your record and show it as yours, with nothing to say it had arrived.
func TestAFriendsIdenticalNoteIsNotSwallowed(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	mine, ok := addNote(appPrefs(), StoredNote{Kind: noteKindMine, VersionID: "web",
		Book: "Psalms", Chapter: 23, VerseLo: 1, Text: "Amen", Nonce: newNoteNonce()})
	if !ok {
		t.Fatal("your note was not stored")
	}
	// Their link: same words, same verse, THEIR nonce.
	url := ShareLinkURLWithNoteNonce("web", "Psalms", 23, 1, 1, "Amen", newNoteNonce())
	target, ok := ParseShareLink(url)
	if !ok {
		t.Fatal("their link did not parse")
	}

	st := planTestState(t)
	got, stored := rememberIncomingNote(st, target)
	if !stored {
		t.Fatal("their note was refused")
	}
	if got.ID == mine.ID {
		t.Error("a friend's note was folded into yours because the words matched — " +
			"their message is gone and the app would call it yours")
	}
	if got.Kind != noteKindReceived {
		t.Errorf("their note is %q, not received", got.Kind)
	}
	if all := allNotesForBrowsing(appPrefs()); len(all) != 2 {
		t.Errorf("expected two notes (yours and theirs), got %d", len(all))
	}
}

// THE OWNER'S SCENARIO, END TO END: you send a note, you tap your own link, and
// the passage shows YOUR note — once, as yours, and gone when you move on.
//
// This is the one that would have caught the whole family of defects: the
// duplicate record, the "Note from Friend" attribution on your own words, and
// the arrival showing nothing at all after the collapse.
func TestTappingYourOwnLinkDrawsYourOwnNote(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	nonce := newNoteNonce()
	// John 3 — the passage planTestState's sample Bible actually has, so the
	// arrival is not clamped onto a different chapter.
	mine, ok := addNote(appPrefs(), StoredNote{Kind: noteKindMine, VersionID: "web",
		Book: "John", Chapter: 3, VerseLo: 16, Text: "sent to neutral contact", Nonce: nonce})
	if !ok {
		t.Fatal("your note was not stored")
	}

	st := planTestState(t)
	url := ShareLinkURLWithNoteNonce("web", "John", 3, 16, 16, "sent to neutral contact", nonce)
	HandleShareLink(st, url)

	// One note in the scrapbook, still yours.
	all := allNotesForBrowsing(appPrefs())
	if len(all) != 1 {
		t.Fatalf("tapping your own link left %d notes in the scrapbook, want 1", len(all))
	}
	if all[0].Kind != noteKindMine || all[0].ID != mine.ID {
		t.Errorf("your note changed identity on the round trip: %+v", all[0])
	}

	// And it is DRAWN on the passage, as yours.
	if st.ActiveNote != "sent to neutral contact" {
		t.Errorf("tapping your own link drew no note: active=%q — the reader followed a "+
			"link carrying a message and was shown a highlight and nothing else",
			st.ActiveNote)
	}
	plan := buildChapterPlan(st, appPrefs(), st.Bible)
	_, who, _, _ := appleStickerPush(st, plan)
	if !strings.Contains(who, "you") {
		t.Errorf("your own words are attributed %q — they are yours", who)
	}
	if strings.Contains(who, "Friend") {
		t.Errorf("your own note is bylined as a friend's: %q", who)
	}
	if strings.Contains(who, "of") {
		t.Errorf("your own note joined the passage's count (%q); N describes the notes "+
			"people sent you and must not change because you looked at your own", who)
	}
}

// NOTHING YOU WROTE CAN BE LOST FROM THE READING PAGE.
//
// Drawing your own note made the pane's − and ✕ reachable on it for the first
// time. Both were wrong for a card that is on the passage only because you
// asked and only until you navigate away: − wrote a DURABLE "minimized" bit,
// whose only reader is a browser sentence that would then be false, and ✕
// deleted the only record of something you wrote — unconfirmed, in one press.
//
// Both now mean "put it away", which is what navigating away does a moment
// later. Deleting your own note stays an explicit act in the notes browser.
func TestReadingPageVerbsCannotDestroyYourOwnNote(t *testing.T) {
	for _, tc := range []struct {
		name string
		act  func(*AppState)
	}{
		{"the minimize verb", hideCurrentNote},
		{"the delete verb", dropCurrentNote},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := test.NewApp()
			defer app.Quit()
			setNotesEnabled(true)
			deleteAllNotes(appPrefs())
			defer deleteAllNotes(appPrefs())

			mine, ok := addNote(appPrefs(), StoredNote{Kind: noteKindMine, VersionID: "web",
				Book: "John", Chapter: 3, VerseLo: 16, Text: "sent to neutral contact"})
			if !ok {
				t.Fatal("your note was not stored")
			}
			st := planTestState(t)
			openNote(st, mine)
			if st.ActiveNote != mine.Text {
				t.Fatalf("precondition: your note must be drawn, got %q", st.ActiveNote)
			}

			tc.act(st)

			// It is off the page...
			if st.ActiveNote != "" {
				t.Errorf("the verb did not put your note away: %q", st.ActiveNote)
			}
			// ...and it still EXISTS, unchanged.
			all := allNotesForBrowsing(appPrefs())
			if len(all) != 1 {
				t.Fatalf("your note was destroyed from the reading page: %d notes remain", len(all))
			}
			if all[0].Minimized {
				t.Error("a durable Minimized was written on a note that is only ever on " +
					"the passage while focused — the browser would then say it is " +
					"'minimized in the chapter', which is not true of it")
			}
			if all[0].Text != mine.Text {
				t.Errorf("your note's text changed: %q", all[0].Text)
			}
		})
	}
}

// And the same verbs still do their real work on a note somebody SENT you —
// the point is not to make the pane inert, only to stop it destroying your own
// words without asking.
func TestReadingPageVerbsStillWorkOnAReceivedNote(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	theirs, ok := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
		Book: "John", Chapter: 3, VerseLo: 16, Text: "synthetic note"})
	if !ok {
		t.Fatal("their note was not stored")
	}
	st := planTestState(t)
	st.focusNote(theirs.ID)
	applyNoteForCurrentChapter(st)
	if st.ActiveNote != theirs.Text {
		t.Fatalf("precondition: their note must be drawn, got %q", st.ActiveNote)
	}

	hideCurrentNote(st)
	all := allNotesForBrowsing(appPrefs())
	if len(all) != 1 || !all[0].Minimized {
		t.Fatalf("minimize must still write the durable bit on a received note: %+v", all)
	}

	st.focusNote(theirs.ID)
	applyNoteForCurrentChapter(st)
	dropCurrentNote(st)
	if len(allNotesForBrowsing(appPrefs())) != 0 {
		t.Error("delete must still delete a received note")
	}
}


// height ... at all").
//

// measures rather than trusts: build the row as it ships, build it again with
// the control removed, and compare. A control that needs its own line — or one
// whose minimum height simply exceeds a short bubble's — would fail here rather
// than being noticed later on a phone.
func TestNoteRowTrashCostsNoHeight(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	st := planTestState(t)
	for _, tc := range []struct{ name, text string }{
		{"a one-word note", "Amen"},
		{"a one-line note", "synthetic note today"},
		{"a wrapping note", "synthetic note today and praying that this week is gentler " +
			"than the last one was, with love from all of us here"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := StoredNote{ID: 1, Kind: noteKindReceived, VersionID: "web",
				Book: "John", Chapter: 3, VerseLo: 16, Text: tc.text, Received: 1_700_000_000}

			withTrash := noteBrowseRow(st, n, lightPalette)
			// The same row, minus only the control.
			bubbleOnly := noteBubblePadded(notePreview(n.Text), lightPalette, browseBubblePad)

			got := withTrash.MinSize().Height
			// The row is head + gap + bubble; the control may not add to that.
			// Measured against the bubble it sits beside: if the control were
			// taller, the Border would grow and this difference would show.
			trashH := noteRowTrash(st, n, lightPalette).MinSize().Height
			bubbleH := bubbleOnly.MinSize().Height
			if trashH > bubbleH {
				t.Errorf("the delete control is %.1fpx tall beside a %.1fpx bubble — "+
					"it would push the row taller for a short note (row is %.1f)",
					trashH, bubbleH, got)
			}
		})
	}
}

// And it is REACHABLE and it DELETES — a control that costs no height and does
// nothing would pass the test above perfectly.
func TestNoteRowTrashDeletesTheNote(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	keep, _ := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
		Book: "John", Chapter: 3, VerseLo: 16, Text: "keep me"})
	drop, ok := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
		Book: "John", Chapter: 3, VerseLo: 17, Text: "delete me"})
	if !ok {
		t.Fatal("seeding failed")
	}
	st := planTestState(t)

	row := noteBrowseRow(st, drop, lightPalette)
	// The icon-only control: no text, an icon, and actually laid out on screen.
	trash := seenBannerButton(t, row, fyne.NewSize(420, 300), func(b *widget.Button) bool {
		return b.Text == "" && b.Icon != nil
	})
	if trash == nil {
		t.Fatal("no delete control on the row")
	}
	test.Tap(trash)

	all := allNotesForBrowsing(appPrefs())
	if len(all) != 1 {
		t.Fatalf("after one delete the store holds %d notes, want 1", len(all))
	}
	if all[0].ID != keep.ID {
		t.Errorf("the wrong note was deleted: %q survived", all[0].Text)
	}
}

// PUTTING A NOTE AWAY OPENS NOTHING ELSE — the rule the codebase states as N3:
// "nothing may take the closed one's place under the reader's eyes."
//
// Found by a state-transition analysis, then measured. With a friend's note
// also on the passage, putting YOUR OWN note away opened THEIRS in its place —
// fully expanded, with the wash jumping onto their verse:
//
//	before: active="my own words" id=2
//	after:  active="friend words" id=1 markLive=true lo=17
//
// The plan was right throughout (openNote() reported false); the MIRROR was
// wrong. applyNoteForCurrentChapter read the stored Minimized bit, which is
// only one of the three things that keep a note closed — the other two, focus
// none and suppression, it could not see. It now reads the plan's Open, which
// folds all three.
//
// The fixture is the whole point: the original test had only ONE note, so the
// display index had nowhere to fall and the defect could not appear.
func TestPuttingYourOwnNoteAwayOpensNothingElse(t *testing.T) {
	for _, tc := range []struct {
		name string
		act  func(*AppState)
	}{
		{"the minimize verb", hideCurrentNote},
		{"the delete verb", dropCurrentNote},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := test.NewApp()
			defer app.Quit()
			setNotesEnabled(true)
			deleteAllNotes(appPrefs())
			defer deleteAllNotes(appPrefs())

			friend, ok := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
				Book: "John", Chapter: 3, VerseLo: 17, Text: "friend words"})
			if !ok {
				t.Fatal("the friend's note was not stored")
			}
			mine, ok := addNote(appPrefs(), StoredNote{Kind: noteKindMine, VersionID: "web",
				Book: "John", Chapter: 3, VerseLo: 16, Text: "my own words"})
			if !ok {
				t.Fatal("your note was not stored")
			}

			st := planTestState(t)
			openNote(st, mine)
			if st.ActiveNote != mine.Text {
				t.Fatalf("precondition: your note must be drawn, got %q", st.ActiveNote)
			}

			tc.act(st)

			// Their note may be PRESENT — the passage does have one, and saying
			// so is honest — but it must not be OPEN, and its verse must not
			// light. Nothing opens in the place of a note you just closed.
			if st.ActiveNote == friend.Text && !st.NoteMinimized {
				t.Errorf("%s: a friend's note opened in place of the one you put away — "+
					"expanded, under the reader's eyes, unasked for", tc.name)
			}
			if _, live := st.markSpan(); live {
				t.Errorf("%s: the wash moved onto another note's verse when you put "+
					"yours away", tc.name)
			}
			// And whatever else happened, YOUR note survived untouched.
			var found bool
			for _, n := range allNotesForBrowsing(appPrefs()) {
				if n.ID == mine.ID {
					found = true
					if n.Minimized {
						t.Errorf("%s: a durable Minimized was written on your own note", tc.name)
					}
				}
			}
			if !found {
				t.Errorf("%s: your own note was destroyed", tc.name)
			}
		})
	}
}

// WHAT YOU SENT IS WHAT IS STORED — the prerequisite the collapse rests on.
//
// A mutation analysis found this path unguarded: strip normalizeNote and the
// single-verse spelling from saveMyNote and the whole suite stayed green. Both
// exist so the sent record and the link carry the SAME note; without them the
// nonce still matches but the two records describe different text, and any
// future content comparison silently misses.
func TestSaveMyNoteStoresWhatActuallyTravelled(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	for _, tc := range []struct{ name, typed string }{
		{"a blank-line run", "First thought.\n\n\n\nSecond thought."},
		{"over the rune cap", strings.Repeat("a very long thought indeed ", 20)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deleteAllNotes(appPrefs())
			saveMyNote(appPrefs(), StoredNote{VersionID: "web", Book: "John",
				Chapter: 3, VerseLo: 16, VerseHi: 16, Text: tc.typed})

			all := allNotesForBrowsing(appPrefs())
			if len(all) != 1 {
				t.Fatalf("%d notes stored, want 1", len(all))
			}
			got := all[0]
			if want := normalizeNote(tc.typed); got.Text != want {
				t.Errorf("stored %d chars, the link carries %d — the record and the "+
					"link would describe different notes", len(got.Text), len(want))
			}
			// And the store's ONE spelling for a single verse.
			if got.VerseHi != 0 {
				t.Errorf("a one-verse note stored VerseHi=%d; the arrival stores 0, so "+
					"the same passage would compare unequal", got.VerseHi)
			}
		})
	}
}

// YOUR OWN NOTE'S BYLINE SURVIVES A CHAPTER THAT CANNOT BE REACHED.
//
// Also found unguarded by mutation: delete the store lookup in
// appleStickerPush's mirror-only arm and nothing failed. That arm runs when the
// note is not in this chapter's plan — an arrival filed on a passage this canon
// lacks, so the chapter was clamped — and it built the who line from a ZERO
// StoredNote, which reads as received. Your own words, under "Note from
// Friend", on the one path where nothing downstream can correct it.
func TestOwnNoteBylineSurvivesAClampedChapter(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	// A note on a chapter the sample Bible does not reach.
	mine, ok := addNote(appPrefs(), StoredNote{Kind: noteKindMine, VersionID: "web",
		Book: "John", Chapter: 21, VerseLo: 25, Text: "the last verse"})
	if !ok {
		t.Fatal("your note was not stored")
	}
	st := planTestState(t)
	// The mirror carries it even though the plan for this chapter does not.
	st.ActiveNote = mine.Text
	st.NoteID = mine.ID
	st.NoteVerseLo = mine.VerseLo

	plan := buildChapterPlan(st, appPrefs(), st.Bible)
	_, who, _, _ := appleStickerPush(st, plan)
	if strings.Contains(who, "Friend") {
		t.Errorf("your own words are bylined %q — the mirror-only arm must ask the "+
			"store whose note it is rather than assuming a received one", who)
	}
	if !strings.Contains(who, "you") {
		t.Errorf("your own note is bylined %q, want it attributed to you", who)
	}
}
