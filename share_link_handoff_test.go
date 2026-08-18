package bibletext

import (
	"fmt"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
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
	st.clearMark()

	url := "https://bibletext.co.uk/bsb/psalms/23/#v1-4&n=" + EncodeNote("handed to the browser")
	if !HandleShareLink(st, url) {
		t.Fatal("the link should still be reported as handled")
	}
	// The fixture clamps chapters, so the highlight — which only a real
	// navigation sets — is the signal that the app did NOT take the link.
	if st.hlOn() {
		t.Error("the app navigated instead of declining the link")
	}
	if st.ActiveNote != "" {
		t.Errorf("a note was surfaced: %q", st.ActiveNote)
	}
	if n := storedNoteCount(fyne.CurrentApp().Preferences()); n != 0 {
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
	st.clearMark()

	if !HandleShareLink(st, "https://bibletext.co.uk/bsb/psalms/23/#v1-4") {
		t.Fatal("link not handled")
	}
	if !st.hlOn() {
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
		if !st.hlOn() {
			t.Error("the passage did not open")
		}
		if n := storedNoteCount(fyne.CurrentApp().Preferences()); n != 0 {
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
		if n := storedNoteCount(fyne.CurrentApp().Preferences()); n != 1 {
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

// The offer's two buttons must never overlap. A Border row pins one left and one
// right at their natural widths and does not shrink them, so on a narrow canvas
// the right-hand button is drawn on top of the left one, hiding its label and
// taking its taps. Every phone under ~410pt did this until the row learned to
// stack. Only a 16 Pro Max was wide enough to look fine — which is why looking
// at it was not enough.
func TestOfferButtonsNeverOverlap(t *testing.T) {
	for _, w := range []float32{320, 375, 402, 440, 834} {
		t.Run(fmt.Sprintf("%gpt", w), func(t *testing.T) {
			app := test.NewApp()
			defer app.Quit()
			th := &bibleTheme{fonts: loadBookFonts(), uiFonts: loadUIFonts()}
			app.Settings().SetTheme(th)
			win := app.NewWindow("offer")
			win.Resize(fyne.NewSize(w, 812))
			st := psalm23State()
			st.window, st.theme = win, th

			offerNoteLinkChoice(st, "https://bibletext.co.uk/bsb/psalms/23/#v1",
				ShareTarget{Book: "Psalms", Chapter: 23, VerseLo: 1, VerseHi: 4, Note: "hi"})
			pop, _ := win.Canvas().Overlays().Top().(*widget.PopUp)
			if pop == nil {
				t.Fatal("the offer did not open")
			}
			test.WidgetRenderer(pop).Layout(pop.Size())

			type box struct {
				label    string
				lo, hi   float32
				top, bot float32
			}
			var boxes []box
			var walk func(o fyne.CanvasObject, ox, oy float32)
			walk = func(o fyne.CanvasObject, ox, oy float32) {
				x, y := ox+o.Position().X, oy+o.Position().Y
				if b, ok := o.(*widget.Button); ok && b.Text != "" {
					boxes = append(boxes, box{b.Text, x, x + b.Size().Width, y, y + b.Size().Height})
				}
				if c, ok := o.(*fyne.Container); ok {
					for _, ch := range c.Objects {
						walk(ch, x, y)
					}
					return
				}
				if wd, ok := o.(fyne.Widget); ok {
					for _, ch := range test.WidgetRenderer(wd).Objects() {
						walk(ch, x, y)
					}
				}
			}
			walk(pop.Content, 0, 0)
			if len(boxes) < 2 {
				t.Fatalf("expected both choice buttons, found %d", len(boxes))
			}
			for i := range boxes {
				for j := i + 1; j < len(boxes); j++ {
					a, b := boxes[i], boxes[j]
					if a.lo < b.hi-0.5 && b.lo < a.hi-0.5 && a.top < b.bot-0.5 && b.top < a.bot-0.5 {
						t.Errorf("%q and %q overlap (x %.0f-%.0f vs %.0f-%.0f, y %.0f-%.0f vs %.0f-%.0f)",
							a.label, b.label, a.lo, a.hi, b.lo, b.hi, a.top, a.bot, b.top, b.bot)
					}
				}
			}
		})
	}
}
