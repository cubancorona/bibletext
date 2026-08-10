package bibletext

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func TestNoteReference(t *testing.T) {
	for _, tc := range []struct {
		n    SharedNote
		want string
	}{
		{SharedNote{Book: "John", Chapter: 11, VerseLo: 35, VerseHi: 35}, "John 11:35"},
		{SharedNote{Book: "John", Chapter: 11, VerseLo: 35}, "John 11:35"},
		{SharedNote{Book: "Psalms", Chapter: 23, VerseLo: 1, VerseHi: 4}, "Psalms 23:1-4"},
		{SharedNote{Book: "Psalms", Chapter: 23}, "Psalms 23"}, // whole chapter
	} {
		if got := noteReference(tc.n); got != tc.want {
			t.Errorf("noteReference(%+v) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

// Reading order, not the alphabetical-by-key order the blob is written in.
func TestSortedNotesUsesCanonOrder(t *testing.T) {
	order := map[string]int{"Genesis": 0, "Psalms": 18, "John": 42, "Revelation": 65}
	notes := map[string]SharedNote{
		"a": {Book: "Revelation", Chapter: 22, Text: "x"},
		"b": {Book: "Genesis", Chapter: 1, Text: "x"},
		"c": {Book: "John", Chapter: 11, Text: "x"},
		"d": {Book: "John", Chapter: 3, Text: "x"},
		"e": {Book: "Psalms", Chapter: 23, VerseLo: 5, Text: "x"},
		"f": {Book: "Psalms", Chapter: 23, VerseLo: 1, Text: "x"},
	}
	got := sortedNotes(notes, order)
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
	notes := map[string]SharedNote{
		"a": {Book: "Tobit", Chapter: 4, Text: "x"},
		"b": {Book: "John", Chapter: 3, Text: "x"},
	}
	got := sortedNotes(notes, order)
	if len(got) != 2 {
		t.Fatalf("a note was dropped: %d of 2", len(got))
	}
	if got[0].Book != "John" || got[1].Book != "Tobit" {
		t.Errorf("expected the unknown book last, got %s then %s", got[0].Book, got[1].Book)
	}
}

func TestMatchNotes(t *testing.T) {
	notes := []SharedNote{
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
	saveNote(p, SharedNote{VersionID: "web", Book: "John", Chapter: 3, VerseLo: 16, Text: "hi"})

	st := psalm23State()
	st.NotesMode = true

	setNotesEnabled(true)
	if got := len(browsableNotes(st)); got != 1 {
		t.Errorf("expected the stored note, got %d", got)
	}
	setNotesEnabled(false)
	if got := len(browsableNotes(st)); got != 0 {
		t.Errorf("notes are off; expected none, got %d", got)
	}
	setNotesEnabled(true)
	if got := len(browsableNotes(st)); got != 1 {
		t.Errorf("switching back on must bring it back, got %d", got)
	}
}

// Opening a note lands on its passage with the WHOLE range highlighted — the
// bug this pins is a range note opening with only its first verse marked.
func TestOpenNoteHighlightsTheWholeRange(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)

	st := psalm23State()
	st.CurrentBook, st.CurrentChapter = "John", 3
	openNote(st, SharedNote{VersionID: "web", Book: "Psalms", Chapter: 23,
		VerseLo: 1, VerseHi: 4, Text: "n"})

	if st.CurrentBook != "Psalms" || st.CurrentChapter != 23 {
		t.Errorf("did not navigate: %s %d", st.CurrentBook, st.CurrentChapter)
	}
	if !st.HasHighlightedVerse || st.HighlightedVerse != 1 || st.HighlightedVerseEnd != 4 {
		t.Errorf("expected 1-4 highlighted, got has=%v %d-%d",
			st.HasHighlightedVerse, st.HighlightedVerse, st.HighlightedVerseEnd)
	}
}

// A whole-chapter note (no verse) must not claim a bogus highlight on verse 0.
func TestOpenNoteWithNoVerseDoesNotHighlight(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)

	st := psalm23State()
	openNote(st, SharedNote{VersionID: "web", Book: "Psalms", Chapter: 23, Text: "n"})
	if st.HasHighlightedVerse {
		t.Errorf("a chapter-wide note highlighted verse %d", st.HighlightedVerse)
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
	n := SharedNote{VersionID: "web", Book: "Psalms", Chapter: 23,
		VerseLo: 1, VerseHi: 4, Text: "n", Minimized: true}
	saveNote(p, n)

	st := psalm23State()
	openNote(st, n)

	back, ok := loadNote(p, "web", "Psalms", 23)
	if !ok {
		t.Fatal("the note vanished")
	}
	if back.Minimized {
		t.Error("opening a minimized note from the list should restore it")
	}
}
