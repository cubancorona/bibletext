package bibletext

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

// A link almost always arrives while the Bible is still loading — share_link_open.go
// calls that the COMMON case, because the OS delivers the link within milliseconds
// of launch. So every branch of HandleShareLink has to survive state.Bible == nil.
//
// The one that did not was the note-offer: it ran BEFORE the still-loading check, so
// with shared notes switched off a note-bearing link put a card over the loading
// spinner whose only working answer was "read it in the browser". Both in-app answers
// called applyShareTarget, which returns immediately on a nil Bible — the reader was
// left on the spinner, nothing was parked, and the sender's note was never stored.
func TestColdStartParksBeforeAskingAboutNotes(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(false) // notes OFF is what triggers the offer
	defer setNotesEnabled(true)

	for _, tc := range []struct {
		name string
		url  string
	}{
		{"link carrying a note", ShareLinkURLWithNote("web", "John", 3, 16, 0, "fixture link message")},
		{"plain link", ShareLinkURLWithNote("web", "John", 3, 16, 0, "")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := &AppState{loadPhase: loadPending} // Bible still nil, as at cold start
			if !HandleShareLink(st, tc.url) {
				t.Fatal("the link was not claimed at all")
			}
			if st.pendingLink == nil {
				t.Fatalf("nothing parked: the reader is left on the loading screen and the link is lost "+
					"(loadPhase=%v Bible==nil:%v)", st.loadPhase, st.Bible == nil)
			}
			if st.pendingLinkRaw != tc.url {
				t.Errorf("parked raw URL = %q, want the original %q", st.pendingLinkRaw, tc.url)
			}
		})
	}
}

// And the question must still be asked — parking first must not smuggle a note
// past a reader who has notes switched off. consumePendingLink re-asks once the
// data lands, which is where the offer belongs.
func TestNoteOfferStillHappensOnceLoaded(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(false)
	defer setNotesEnabled(true)

	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	st := &AppState{
		Bible: bd, CurrentBook: "Genesis", CurrentChapter: 1,
		CurrentVersion: "web", loadPhase: loadPending,
	}
	url := ShareLinkURLWithNote("web", "John", 3, 16, 0, "fixture queued message")
	HandleShareLink(st, url)
	if st.pendingLink == nil {
		t.Fatal("precondition: the link should be parked")
	}

	// The data lands. consumePendingLink must NOT store the note or open the
	// passage silently — notes are off, so it has to ask.
	st.loadPhase = loadReady
	consumePendingLink(st)

	if st.ActiveNote != "" {
		t.Errorf("a note was shown although shared notes are switched off: %q", st.ActiveNote)
	}
	if n := storedNoteCount(appPrefs()); n != 0 {
		t.Errorf("a note was stored although shared notes are switched off (%d stored)", n)
	}
}

// With notes ON, a parked note-bearing link opens the passage and keeps the note —
// the ordering change must not cost the normal path anything.
func TestColdStartNoteLinkOpensNormallyWhenNotesAreOn(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)

	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	st := &AppState{
		Bible: bd, CurrentBook: "Genesis", CurrentChapter: 1,
		CurrentVersion: "web", loadPhase: loadPending,
		loadedVersions: map[string]*BibleData{"web": bd},
	}
	HandleShareLink(st, ShareLinkURLWithNote("web", "John", 3, 16, 0, "fixture cold-start message"))
	if st.pendingLink == nil {
		t.Fatal("precondition: the link should be parked")
	}
	st.loadPhase = loadReady
	consumePendingLink(st)

	if st.CurrentBook != "John" || st.CurrentChapter != 3 {
		t.Errorf("landed on %s %d, want John 3", st.CurrentBook, st.CurrentChapter)
	}
	if st.ActiveNote != "fixture cold-start message" {
		t.Errorf("the note did not survive the park: %q", st.ActiveNote)
	}
}
