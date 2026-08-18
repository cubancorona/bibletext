package bibletext

// The note banner: the desktop/Android rendering of a stored note.

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func bannerState(t *testing.T) *AppState {
	t.Helper()
	pinBannerPlatform(t)
	st := psalm23State()
	st.CurrentBook, st.CurrentChapter = "Psalms", 23
	return st
}

// pinBannerPlatform makes the tests ask the question they mean. The banner is
// what Windows, Linux and Android show; the host running these tests is darwin,
// where the pane draws its own in-text sticker and buildNoteBanner correctly
// returns nil. Without pinning the seam every assertion below would be testing
// the suppression, not the banner.
func pinBannerPlatform(t *testing.T) {
	t.Helper()
	orig := nativeNoteSticker
	nativeNoteSticker = func() bool { return false }
	t.Cleanup(func() { nativeNoteSticker = orig })
}

func TestNoteBannerShowsTheNote(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)

	st := bannerState(t)
	st.ActiveNote = "This got me through last night."

	b := buildNoteBanner(st)
	if b == nil {
		t.Fatal("no banner for an active note")
	}
	texts := treeTexts(b)
	joined := ""
	for _, s := range texts {
		joined += s + "|"
	}
	for _, want := range []string{"Note from Friend", "Psalms 23", "This got me through last night."} {
		found := false
		for _, s := range texts {
			if s == want {
				found = true
			}
		}
		if !found {
			t.Errorf("banner missing %q; got %s", want, joined)
		}
	}
}

func TestNoteBannerAbsentWhenThereIsNothingToShow(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := bannerState(t)
	setNotesEnabled(true)
	if b := buildNoteBanner(st); b != nil {
		t.Error("banner appeared with no note")
	}

	st.ActiveNote = "hidden by the setting"
	setNotesEnabled(false)
	if b := buildNoteBanner(st); b != nil {
		t.Error("banner appeared with notes switched off")
	}
	setNotesEnabled(true)
}

// Minimized → a chip, not the note; pressing it restores through the same
// store helper the iOS sticker uses.
func TestNoteBannerMinimizedChipRestores(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	p := fyne.CurrentApp().Preferences()
	setNotesEnabled(true)
	stored, ok := addNote(p, StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "Psalms", Chapter: 23,
		VerseLo: 1, VerseHi: 4, Text: "kept", Minimized: true})
	if !ok {
		t.Fatal("the note was not stored")
	}

	st := bannerState(t)
	st.ActiveNote = "kept"
	st.NoteMinimized = true
	st.NoteVerseLo = 1
	st.NoteID = stored.ID // the identity Show addresses

	b := buildNoteBanner(st)
	if b == nil {
		t.Fatal("no chip for a minimized note")
	}
	chip := findTreeButton(b, "Show note")
	if chip == nil {
		t.Fatalf("expected the Show note chip; texts: %v", treeTexts(b))
	}
	for _, s := range treeTexts(b) {
		if s == "kept" {
			t.Error("a minimized note's text is showing")
		}
	}
	test.Tap(chip)
	if st.NoteMinimized {
		t.Error("the chip did not restore the note")
	}
	back, _ := noteForChapter(p, "web", "Psalms", 23)
	if back.Minimized {
		t.Error("restore did not persist")
	}
}

// A pasted share link routes through HandleShareLink — this is how notes reach
// the desktop at all, so it is pinned here beside the banner that shows them.
func TestExecuteSearchOpensPastedShareLinks(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)

	// A one-chapter book: applyShareTarget clamps the chapter against the
	// book's chapter COUNT, which assumes the contiguous numbering every real
	// canon has — a sparse test fixture (only chapter 23 present) would clamp
	// 23 down to 1 and fail for fixture reasons, not product ones.
	bd := &BibleData{
		Books: []string{"Philemon"},
		Verses: map[string]map[int][]Verse{"Philemon": {1: {
			{BookName: "Philemon", Chapter: 1, Verse: 1, Text: "Paul, a prisoner of Christ Jesus."},
			{BookName: "Philemon", Chapter: 1, Verse: 4, Text: "I always thank my God."},
		}}},
	}
	bd.PrepareSearchIndex()
	st := &AppState{Bible: bd, CurrentBook: "Philemon", CurrentChapter: 1}
	link := ShareLinkURLWithNote("web", "Philemon", 1, 1, 4, "pasted note")
	executeSearch(st, link)

	if st.CurrentBook != "Philemon" || st.CurrentChapter != 1 {
		t.Fatalf("pasted link did not navigate: %s %d", st.CurrentBook, st.CurrentChapter)
	}
	if st.ActiveNote != "pasted note" {
		t.Errorf("pasted link's note not active: %q", st.ActiveNote)
	}
	if st.IsSearching {
		t.Error("a pasted link must not leave the pane in search mode")
	}
}

// A BARE passage link (no note payload) must not blank the note already stored
// on that chapter. Before the gate in applyShareTarget, arriving on a chapter
// via a plain link wrote an empty ActiveNote over the stored one, so the banner
// vanished until the reader navigated away and back (platform reproduction).
func TestBarePassageLinkKeepsTheStoredNote(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	pinBannerPlatform(t) // this one builds its own state, so it pins its own seam

	bd := &BibleData{
		Books: []string{"Philemon"},
		Verses: map[string]map[int][]Verse{"Philemon": {1: {
			{BookName: "Philemon", Chapter: 1, Verse: 1, Text: "Paul, a prisoner of Christ Jesus."},
			{BookName: "Philemon", Chapter: 1, Verse: 4, Text: "I always thank my God."},
		}}},
	}
	bd.PrepareSearchIndex()
	st := &AppState{Bible: bd, CurrentBook: "Philemon", CurrentChapter: 1}

	// A note arrives on a link and is stored.
	withNote := ShareLinkURLWithNote("web", "Philemon", 1, 1, 4, "synthetic note")
	if !HandleShareLink(st, withNote) {
		t.Fatal("the note link was not handled")
	}
	if st.ActiveNote != "synthetic note" {
		t.Fatalf("note did not arrive: %q", st.ActiveNote)
	}

	// The SAME passage is then opened by a plain link carrying no note.
	bare := ShareLinkURL("web", "Philemon", 1, 1, 4)
	if !HandleShareLink(st, bare) {
		t.Fatal("the bare link was not handled")
	}
	if st.ActiveNote != "synthetic note" {
		t.Errorf("a bare link wiped the stored note (ActiveNote=%q)", st.ActiveNote)
	}
	if buildNoteBanner(st) == nil {
		t.Error("the banner disappeared after a bare link to the same passage")
	}
}
