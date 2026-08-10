package bibletext

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// With notes off, a note-bearing link must not just open: the app has to stop
// and ask. Without a window there is nothing to ask in, so what this pins is
// that it does NOT silently navigate and does NOT store the note.
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

// The three ways out of the offer. Each is driven directly, because the popup
// itself needs a window; what matters is that each branch does the right thing
// to the passage, the note and the setting.
func TestOfferBranches(t *testing.T) {
	url := "https://bibletext.co.uk/bsb/psalms/23/#v1-4&n=" + EncodeNote("the message")
	target, ok := ParseShareLink(url)
	if !ok {
		t.Fatal("link did not parse")
	}

	t.Run("just the passage drops the note", func(t *testing.T) {
		app := test.NewApp()
		defer app.Quit()
		setNotesEnabled(false)
		st := psalm23State()
		st.loadPhase = loadReady

		bare := target
		bare.Note = ""
		applyShareTarget(st, bare)

		if st.ActiveNote != "" {
			t.Errorf("the note survived: %q", st.ActiveNote)
		}
		if !st.HasHighlightedVerse {
			t.Error("the passage did not open")
		}
		if n := len(readNotes(fyne.CurrentApp().Preferences())); n != 0 {
			t.Errorf("a dropped note was stored anyway (%d)", n)
		}
		if notesEnabled() {
			t.Error("reading the passage should not have changed the setting")
		}
	})

	t.Run("turning notes on reads it here", func(t *testing.T) {
		app := test.NewApp()
		defer app.Quit()
		setNotesEnabled(false)
		st := psalm23State()
		st.loadPhase = loadReady

		setNotesEnabled(true) // what the quiet link does
		applyShareTarget(st, target)

		if !notesEnabled() {
			t.Error("the setting did not switch on")
		}
		if st.ActiveNote != "the message" {
			t.Errorf("the note did not arrive: %q", st.ActiveNote)
		}
		if n := len(readNotes(fyne.CurrentApp().Preferences())); n != 1 {
			t.Errorf("expected the note stored once, got %d", n)
		}
	})
}

// The offer's subtitle names the passage the link points at — built from the
// target, because the app has not navigated there and may never.
func TestShareTargetReference(t *testing.T) {
	for _, c := range []struct {
		t    ShareTarget
		want string
	}{
		{ShareTarget{Book: "John", Chapter: 3, VerseLo: 16, VerseHi: 18}, "John 3:16-18"},
		{ShareTarget{Book: "John", Chapter: 3, VerseLo: 16}, "John 3:16"},
		{ShareTarget{Book: "Philippians", Chapter: 4}, "Philippians 4"},
		{ShareTarget{Book: "1 Corinthians", Chapter: 13, VerseLo: 4, VerseHi: 7}, "1 Corinthians 13:4-7"},
	} {
		if got := shareTargetReference(c.t); got != c.want {
			t.Errorf("got %q, want %q", got, c.want)
		}
	}
}
