package bibletext

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// The handoff decision, without the platform half: openLinkInBrowser is a no-op
// off iOS/Android, so what these pin is that the app DECLINES the link — it does
// not navigate, and it stores nothing.
func TestNoteLinkIsDeclinedWhenNotesAreOff(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(false)

	st := psalm23State()
	st.loadPhase = loadReady
	st.HasHighlightedVerse = false

	url := "https://bibletext.co.uk/bsb/psalms/23/#v1-4&n=" + EncodeNote("handed to the browser")
	if !HandleShareLink(st, url) {
		t.Fatal("the link should still be reported as handled")
	}
	// The fixture clamps chapters, so the highlight — which only a real
	// navigation sets — is the signal that the app did NOT take the link.
	if st.HasHighlightedVerse {
		t.Error("the app navigated instead of declining the link")
	}
	if st.ActiveNote != "" {
		t.Errorf("a note was surfaced: %q", st.ActiveNote)
	}
	if n := len(readNotes(fyne.CurrentApp().Preferences())); n != 0 {
		t.Errorf("a note was stored while off (%d)", n)
	}
}

// A PLAIN shared verse belongs in the app whatever the notes setting says —
// handing every link to the browser would break sharing for everyone.
func TestPlainLinkStillOpensInTheAppWhenNotesAreOff(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(false)

	st := psalm23State()
	st.loadPhase = loadReady
	st.HasHighlightedVerse = false

	if !HandleShareLink(st, "https://bibletext.co.uk/bsb/psalms/23/#v1-4") {
		t.Fatal("link not handled")
	}
	if !st.HasHighlightedVerse {
		t.Error("a plain shared link should have opened in the app, not been handed away")
	}
}

// With notes ON, a note-bearing link is the app's business as usual.
func TestNoteLinkOpensInTheAppWhenNotesAreOn(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)

	st := psalm23State()
	st.loadPhase = loadReady
	url := "https://bibletext.co.uk/bsb/psalms/23/#v1-4&n=" + EncodeNote("stays in the app")
	if !HandleShareLink(st, url) {
		t.Fatal("link not handled")
	}
	if st.ActiveNote != "stays in the app" {
		t.Errorf("the note did not arrive: %q", st.ActiveNote)
	}
}

// A cold start parks the link and consumes it later — the raw URL has to survive
// that trip, or the handoff would have nothing to hand over.
func TestParkedLinkKeepsItsURL(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)

	st := psalm23State()
	st.loadPhase = loadPending
	url := "https://bibletext.co.uk/bsb/psalms/23/#v1-4&n=" + EncodeNote("parked")
	if !HandleShareLink(st, url) {
		t.Fatal("link not handled")
	}
	if st.pendingLinkRaw != url {
		t.Errorf("the raw URL was lost on parking: %q", st.pendingLinkRaw)
	}
	if st.pendingLink == nil || st.pendingLink.Note != "parked" {
		t.Error("the parked target lost its note")
	}
}
