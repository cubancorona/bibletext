package bibletext

// The settings sheet's close-out delta versus the surfaces its switches
// change. Two pins:
//
//   - The NOTES switch changes surface existence exactly the way the assistant
//     radio does — the sidebar/Search-tab mode row gates its notes bubble on
//     notesFeatureOn at BUILD time, and searchModeOf validates a leftover
//     Notes mode against the switch — so done() must rebuild the window when
//     the switch's value differs from open. Without that, flipping notes off
//     while the tab sat in Notes mode left the dead "Search your notes…"
//     field and the lit bubble over keyword results until a tab switch
//     happened to rebuild (and ON added no bubble at all).
//
//   - The assistant radio's None tap tears down a live Find context
//     (clearAISearchContext mutates IsSearching and drops the results that
//     owned the reading pane) and must repaint the pane ITSELF: done()'s
//     open-vs-close delta nets to zero on a None→provider round trip inside
//     one sheet session, so a teardown that leans on it repaints nothing and
//     the dead results sit on screen until the next navigation.

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// settingsSheetControls digs the open sheet's popup out of the overlay stack
// and finds one control in it by predicate.
func settingsPopup(t *testing.T, state *AppState) *widget.PopUp {
	t.Helper()
	popup, ok := state.window.Canvas().Overlays().Top().(*widget.PopUp)
	if !ok || popup == nil {
		t.Fatalf("expected the settings sheet on the overlay stack, got %T",
			state.window.Canvas().Overlays().Top())
	}
	return popup
}

func sheetCloseButton(t *testing.T, popup *widget.PopUp) *widget.Button {
	t.Helper()
	var closeBtn *widget.Button
	walkTree(popup, func(o fyne.CanvasObject) {
		if b, ok := o.(*widget.Button); ok && closeBtn == nil && b.Text == "" &&
			b.Icon != nil && b.Icon.Name() == theme.CancelIcon().Name() {
			closeBtn = b
		}
	})
	if closeBtn == nil {
		t.Fatal("no ✕ on the settings sheet")
	}
	return closeBtn
}

func sheetNotesCheck(t *testing.T, popup *widget.PopUp) *widget.Check {
	t.Helper()
	var check *widget.Check
	walkTree(popup, func(o fyne.CanvasObject) {
		if c, ok := o.(*widget.Check); ok && c.Text == "Show notes people share with you" {
			check = c
		}
	})
	if check == nil {
		t.Fatal("no notes switch on the settings sheet")
	}
	return check
}

func TestSettingsNotesToggleRebuildsTheWindowOnClose(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	state := deferredTestState(t, app)
	setNotesEnabled(true)
	deleteAllNotes(appPrefs()) // zero stored → the off-flip asks no question
	defer setNotesEnabled(true)

	// OFF while the sheet is open, then close: the mode row and field host are
	// built from notesFeatureOn at window build time, so the close-out owes a
	// rebuild.
	showAISettings(state)
	popup := settingsPopup(t, state)
	genBefore := windowRebuildGen
	sheetNotesCheck(t, popup).SetChecked(false)
	if notesEnabled() {
		t.Fatal("the switch did not flip")
	}
	if windowRebuildGen != genBefore {
		t.Fatal("the flip itself must not rebuild over the open sheet")
	}
	test.Tap(sheetCloseButton(t, popup))
	if windowRebuildGen != genBefore+1 {
		t.Fatalf("notes went off while the sheet was open: done() must rebuild the window "+
			"(mode row, field host), got %d rebuilds", windowRebuildGen-genBefore)
	}

	// The ON direction is the mirror: enabling notes adds the bubble only at a
	// rebuild.
	showAISettings(state)
	popup = settingsPopup(t, state)
	genBefore = windowRebuildGen
	sheetNotesCheck(t, popup).SetChecked(true)
	test.Tap(sheetCloseButton(t, popup))
	if windowRebuildGen != genBefore+1 {
		t.Fatalf("notes came on while the sheet was open: done() must rebuild, got %d rebuilds",
			windowRebuildGen-genBefore)
	}

	// And a no-change visit stays cheap: open, close, no rebuild.
	showAISettings(state)
	popup = settingsPopup(t, state)
	genBefore = windowRebuildGen
	test.Tap(sheetCloseButton(t, popup))
	if windowRebuildGen != genBefore {
		t.Errorf("nothing changed: closing must not rebuild (got %d)", windowRebuildGen-genBefore)
	}
}

func TestAssistantNoneTapRepaintsTheFindTeardown(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	state := deferredTestState(t, app)
	if !state.keys().aiEnabled() {
		state.keys().setAIEnabled(true)
	}

	// Live Find results own the reading pane.
	state.aiSearchActive = true
	state.aiSearchMode = true
	state.aiSearchQuery = "hope in suffering"
	state.IsSearching = true
	state.CanReturnToSearchResults = true

	showAISettings(state)
	popup := settingsPopup(t, state)

	// Instrument the repaint AFTER the sheet opened, so only the verb's own
	// repaint counts.
	repaints := 0
	state.showReading = func() { repaints++ }
	state.syncSidebar = func() {}

	var radio *widget.RadioGroup
	walkTree(popup, func(o fyne.CanvasObject) {
		if r, ok := o.(*widget.RadioGroup); ok && radio == nil {
			for _, opt := range r.Options {
				if opt == "None" {
					radio = r
					return
				}
			}
		}
	})
	if radio == nil {
		t.Fatal("no assistant radio on the sheet")
	}

	genBefore := windowRebuildGen
	radio.SetSelected("None")
	if state.IsSearching || state.aiSearchActive {
		t.Fatal("None must tear the Find context down")
	}
	if repaints == 0 {
		t.Error("the None tap emptied the results context that owned the pane and repainted " +
			"nothing — done()'s open-vs-close delta nets to zero on a round trip, so the " +
			"teardown must carry its own repaint")
	}

	// The round trip: provider back on, close. The pane was repainted at the
	// teardown, so the netted-to-zero delta correctly rebuilds nothing.
	radio.SetSelected(aiProviders()[0].Name)
	test.Tap(sheetCloseButton(t, popup))
	if windowRebuildGen != genBefore {
		t.Errorf("the round trip should close without a rebuild (got %d) — the mid-flight "+
			"repaint is what keeps that cheap close honest", windowRebuildGen-genBefore)
	}
}
