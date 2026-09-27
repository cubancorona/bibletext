package bibletext

// The light/dark decision (appearance.go) and the reopen seam
// (sheet_reopen.go).
//
// THE DEFECT these hold: with a sheet open, a system appearance change left
// the app half in each theme — the sheet in the palette it was built with over
// stock widgets Fyne had re-lit (a dark card, dark-on-dark letters, light entry
// boxes), and the page behind it mixed until the sheet closed — because the
// theme rebuild was deferred while any sheet was up. The deferral existed so
// the app switcher's two-appearance snapshot of a backgrounding app would not
// drain the sheet the reader had left open. Now the snapshot never reaches a
// rebuild (the foreground gate), a real change rebuilds at once, and the sheet
// on top comes back in the new palette showing what it showed.
//
// Two levels. The gate is pure, so every order the listener's closures can
// run in is walked through it exhaustively. The same orders then go through
// observeAppearance against a real window with a real sheet open, because the
// gate is only half of it: the window has to be rebuilt, or not, and the
// sheet taken back, or left alone.

import (
	"fmt"
	"image/color"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// --- the orders -------------------------------------------------------------

// A step on the timeline:
//
//	X   OnExitedForeground
//	E   OnEnteredForeground
//	Vn  the driver applying a size event that sets the variant (no gate call:
//	    the settings change, and the listener's closure is queued)
//	Cn  that closure running on the UI goroutine (the gate reads the LIVE variant)
//
// orders returns every permutation of steps that keeps each (a, b) pair in
// "a before b" order. The driver's own order (X, V…, E, on one event queue) is
// a chain of such pairs; each closure's only constraint is that it runs after
// the update that queued it — before the next update, after it, before the
// return, after it, and in either order against another closure.
func orders(steps []string, before [][2]string) [][]string {
	var out [][]string
	used := make([]bool, len(steps))
	cur := make([]string, 0, len(steps))
	var walk func()
	walk = func() {
		if len(cur) == len(steps) {
			pos := map[string]int{}
			for i, s := range cur {
				pos[s] = i
			}
			for _, b := range before {
				if pos[b[0]] > pos[b[1]] {
					return
				}
			}
			out = append(out, append([]string(nil), cur...))
			return
		}
		for i, s := range steps {
			if used[i] {
				continue
			}
			used[i] = true
			cur = append(cur, s)
			walk()
			cur = cur[:len(cur)-1]
			used[i] = false
		}
	}
	walk()
	return out
}

// roundTripOrders is the snapshot: built dark; resign; light; dark; return.
func roundTripOrders() [][]string {
	return orders(
		[]string{"X", "V1", "V2", "E", "C1", "C2"},
		[][2]string{{"X", "V1"}, {"V1", "V2"}, {"V2", "E"}, {"V1", "C1"}, {"V2", "C2"}},
	)
}

var (
	dark  = theme.VariantDark
	light = theme.VariantLight
)

func other(v fyne.ThemeVariant) fyne.ThemeVariant {
	if v == dark {
		return light
	}
	return dark
}

// gateSim runs orders through the pure gate: the variant the driver has
// applied so far, and every rebuild and reopen the gate asks for.
type gateSim struct {
	g        appearanceGate
	live     fyne.ThemeVariant
	sheet    bool
	rebuilds int
	reopens  int
	reopenAt []fyne.ThemeVariant
}

func (s *gateSim) run(order []string, updates map[string]fyne.ThemeVariant) {
	for _, step := range order {
		switch {
		case step == "X":
			s.hear(appearanceExitedForeground)
		case step == "E":
			s.hear(appearanceEnteredForeground)
		case strings.HasPrefix(step, "V"):
			s.live = updates[step]
		case strings.HasPrefix(step, "C"):
			s.hear(appearanceChanged)
		}
	}
}

func (s *gateSim) hear(ev appearanceEvent) {
	act := s.g.decide(ev, s.live, s.sheet)
	if act.reopen && !act.rebuild {
		panic("a reopen without a rebuild")
	}
	if act.rebuild {
		s.rebuilds++
		if act.reopen {
			s.reopens++
			s.reopenAt = append(s.reopenAt, s.live)
		}
	}
}

// --- the window -------------------------------------------------------------

// appearanceHarness is a real test window holding CreateMainUI's tree, so
// rebuildWindow really swaps it and really drains its overlays, with the
// system variant under the test's control.
type appearanceHarness struct {
	t       *testing.T
	state   *AppState
	variant fyne.ThemeVariant
}

func newAppearanceHarness(t *testing.T, mobile bool) *appearanceHarness {
	t.Helper()
	app := test.NewApp()
	t.Cleanup(app.Quit)
	h := &appearanceHarness{t: t, variant: dark}
	prev := appearanceVariant
	appearanceVariant = func(*AppState) fyne.ThemeVariant { return h.variant }
	t.Cleanup(func() { appearanceVariant = prev })
	votdSynchronousRemeasure(t)
	h.state = deferredTestState(t, app)
	h.state.window.Resize(fyne.NewSize(900, 760))
	h.state.appearance = appearanceGate{mobile: mobile, built: h.variant, frame: h.variant}
	return h
}

// step runs one timeline step against the real window.
func (h *appearanceHarness) step(step string, updates map[string]fyne.ThemeVariant) {
	switch {
	case step == "X":
		observeAppearance(h.state, appearanceExitedForeground)
	case step == "E":
		observeAppearance(h.state, appearanceEnteredForeground)
	case strings.HasPrefix(step, "V"):
		h.variant = updates[step]
	case strings.HasPrefix(step, "C"):
		observeAppearance(h.state, appearanceChanged)
	}
}

// flip is a change heard in the foreground: the update, then its closure.
func (h *appearanceHarness) flip() {
	h.variant = other(h.variant)
	observeAppearance(h.state, appearanceChanged)
}

func (h *appearanceHarness) top() *widget.PopUp {
	h.t.Helper()
	p, _ := h.state.window.Canvas().Overlays().Top().(*widget.PopUp)
	return p
}

func (h *appearanceHarness) overlays() int {
	return len(h.state.window.Canvas().Overlays().List())
}

// openGotoWithVerse opens the Go to picker and types a verse range into it,
// so there is something of the reader's in the sheet to lose.
func (h *appearanceHarness) openGotoWithVerse(start, end string) *widget.PopUp {
	h.t.Helper()
	showGotoPicker(h.state)
	p := h.top()
	if p == nil {
		h.t.Fatal("the Go to picker did not open")
	}
	s, e := findNumberEntry(p, "verse"), findNumberEntry(p, "end")
	if s == nil || e == nil {
		h.t.Fatal("the Go to picker has no verse fields")
	}
	s.SetText(start)
	e.SetText(end)
	return p
}

// sheetTexts and sheetHas are treeTexts and treeHasText for a popup that may
// be missing: a nil *widget.PopUp handed on as a CanvasObject is not a nil
// interface, and the tree walk would dereference it — a failure would then
// panic and take every later test down with it instead of being reported.
func sheetTexts(p *widget.PopUp) []string {
	if p == nil {
		return nil
	}
	return treeTexts(p)
}

func sheetHas(p *widget.PopUp, want string) bool {
	return p != nil && treeHasText(p, want)
}

// registered reports whether popup's sheet still holds a reopen — the same
// question every take and every drain asks, pruning first.
func registered(state *AppState, popup *widget.PopUp) bool {
	pruneSheetReopens(state)
	for _, r := range state.sheetReopens {
		if r.popup == popup {
			return true
		}
	}
	return false
}

// --- a. the snapshot round trip ----------------------------------------------

// THE ROUND TRIP NETS TO NOTHING, WHATEVER ORDER THE CLOSURES RUN IN: no
// rebuild, no reopen, and the sheet the reader left open is the same sheet,
// still showing what they typed. Mutation guarded: the foreground gate off
// (a closure heard in the background compares like any other) — the gate
// then rebuilds for the away leg in every order where C1 runs between the
// updates, and the window drains the picker.
func TestAppearanceRoundTripInEveryOrder(t *testing.T) {
	all := roundTripOrders()
	// CONTROL: the enumeration must reach the orders that matter, or the
	// sweep proves less than it says.
	var bothAfterReturn, firstBeforeSecondUpdate, reversed bool
	for _, o := range all {
		pos := map[string]int{}
		for i, s := range o {
			pos[s] = i
		}
		bothAfterReturn = bothAfterReturn || (pos["C1"] > pos["E"] && pos["C2"] > pos["E"])
		firstBeforeSecondUpdate = firstBeforeSecondUpdate || pos["C1"] < pos["V2"]
		reversed = reversed || pos["C2"] < pos["C1"]
	}
	if len(all) < 8 || !bothAfterReturn || !firstBeforeSecondUpdate || !reversed {
		t.Fatalf("control: %d orders; both closures after the return %v, C1 before V2 %v, C2 before C1 %v",
			len(all), bothAfterReturn, firstBeforeSecondUpdate, reversed)
	}
	updates := map[string]fyne.ThemeVariant{"V1": light, "V2": dark}

	t.Run("gate", func(t *testing.T) {
		for _, o := range all {
			s := &gateSim{g: appearanceGate{mobile: true, built: dark}, live: dark, sheet: true}
			s.run(o, updates)
			if s.rebuilds != 0 || s.reopens != 0 {
				t.Errorf("order %v: %d rebuilds, %d reopens — the round trip must net to nothing",
					o, s.rebuilds, s.reopens)
			}
			if s.g.background {
				t.Errorf("order %v: the gate is still closed after the return", o)
			}
		}
	})

	t.Run("window", func(t *testing.T) {
		h := newAppearanceHarness(t, true)
		sheet := h.openGotoWithVerse("16", "18")
		gen := windowRebuildGen
		for _, o := range all {
			for _, step := range o {
				h.step(step, updates)
			}
			if windowRebuildGen != gen {
				t.Fatalf("order %v: the window was rebuilt %d times under the open sheet",
					o, windowRebuildGen-gen)
			}
			if h.top() != sheet || !sheet.Visible() {
				t.Fatalf("order %v: the sheet the reader left open is gone", o)
			}
		}
		if got := findNumberEntry(sheet, "verse").Text; got != "16" {
			t.Errorf("the typed verse must survive untouched, got %q", got)
		}
		if !registered(h.state, sheet) {
			t.Error("the kept sheet must still hold its reopen for the next real change")
		}
	})

	// Something else rebuilds the window between the legs — a download landing
	// with no sheet up rebuilds at once — so the window is built in the AWAY
	// variant. The return must see that and repaint it: the gate compares with
	// what rebuildWindow recorded, not with the last change it acted on.
	// Mutation guarded: rebuildWindow no longer recording the variant it built
	// with (the gate still believes dark, so the reconcile finds nothing and
	// the window stays light under a dark system).
	t.Run("a rebuild between the legs", func(t *testing.T) {
		h := newAppearanceHarness(t, true)
		gen := windowRebuildGen
		for _, step := range []string{"X", "V1", "C1"} {
			h.step(step, updates)
		}
		rebuildWindow(h.state) // not the gate's: the window is now built light
		if windowRebuildGen != gen+1 || h.state.appearance.built != light {
			t.Fatalf("control: the away-leg rebuild must record light; rebuilds %d, built %v",
				windowRebuildGen-gen, h.state.appearance.built)
		}
		for _, step := range []string{"V2", "C2", "E"} {
			h.step(step, updates)
		}
		if windowRebuildGen != gen+2 || h.state.appearance.built != dark {
			t.Errorf("the window built light during the away leg must be rebuilt dark on the return; "+
				"rebuilds %d, built %v", windowRebuildGen-gen, h.state.appearance.built)
		}
	})
}

// --- b. a real change while away ---------------------------------------------

// A REAL CHANGE MADE WHILE THE APP WAS AWAY rebuilds exactly once, and the
// sheet comes back exactly once, in the new variant — whether the closure runs
// before the return or after it, and with the switcher's round trips around
// the change as well. Mutation guarded: the reconcile off (entering the
// foreground only opens the gate) — the closure that ran in the background was
// ignored and nothing ever reads the change, so the window stays in the old
// palette.
func TestAppearanceRealChangeWhileAwayReopensOnce(t *testing.T) {
	simple := orders(
		[]string{"X", "V1", "E", "C1"},
		[][2]string{{"X", "V1"}, {"V1", "E"}, {"V1", "C1"}},
	)
	// Android's order for a change made while away: the redraw that brings the
	// app back queues the lifecycle event BEFORE the size event carrying the
	// new appearance, so the reconcile runs against the old variant and the
	// update, with its closure, comes after it (appearance.go).
	simple = append(simple, []string{"X", "E", "V1", "C1"})
	// Round trips on both sides of the real change: resign, snapshot (light,
	// dark), the scheduled change to light, return — with every closure free.
	busy := orders(
		[]string{"X", "V1", "V2", "V3", "E", "C1", "C2", "C3"},
		[][2]string{{"X", "V1"}, {"V1", "V2"}, {"V2", "V3"}, {"V3", "E"},
			{"V1", "C1"}, {"V2", "C2"}, {"V3", "C3"}},
	)
	if len(simple) != 3 || len(busy) < 30 {
		t.Fatalf("control: %d simple orders, %d busy ones", len(simple), len(busy))
	}

	t.Run("gate", func(t *testing.T) {
		cases := []struct {
			orders  [][]string
			updates map[string]fyne.ThemeVariant
		}{
			{simple, map[string]fyne.ThemeVariant{"V1": light}},
			{busy, map[string]fyne.ThemeVariant{"V1": light, "V2": dark, "V3": light}},
		}
		for _, c := range cases {
			for _, o := range c.orders {
				s := &gateSim{g: appearanceGate{mobile: true, built: dark}, live: dark, sheet: true}
				s.run(o, c.updates)
				if s.rebuilds != 1 || s.reopens != 1 {
					t.Errorf("order %v: %d rebuilds, %d reopens — want exactly one of each",
						o, s.rebuilds, s.reopens)
					continue
				}
				if s.reopenAt[0] != light || s.g.built != light {
					t.Errorf("order %v: reopened at %v, built %v — want the new variant", o, s.reopenAt[0], s.g.built)
				}
			}
		}
	})

	t.Run("window", func(t *testing.T) {
		for _, o := range simple {
			h := newAppearanceHarness(t, true)
			sheet := h.openGotoWithVerse("16", "18")
			gen := windowRebuildGen
			for _, step := range o {
				h.step(step, map[string]fyne.ThemeVariant{"V1": light})
			}
			if windowRebuildGen != gen+1 {
				t.Fatalf("order %v: want exactly one rebuild, got %d", o, windowRebuildGen-gen)
			}
			again := h.top()
			if again == nil || again == sheet || sheet.Visible() || !again.Visible() {
				t.Fatalf("order %v: the drained picker must be replaced by a new one", o)
			}
			if h.overlays() != 1 {
				t.Fatalf("order %v: want exactly one sheet after the reopen, have %d", o, h.overlays())
			}
			if got := findNumberEntry(again, "verse").Text; got != "16" {
				t.Errorf("order %v: the reopened picker lost the typed verse (%q)", o, got)
			}
			if h.state.appearance.built != light {
				t.Errorf("order %v: the window must be recorded as built light", o)
			}
		}
	})
}

// --- c. a change heard in the foreground ---------------------------------------

// A CHANGE THE READER MAKES WITH THE APP UP — the quick-settings tile on
// Android, a desktop, or (the gate's other route to the same answer) Control
// Centre on iOS, which takes the app out of the foreground while it is open —
// rebuilds once and reopens the sheet once. Mutation guarded: the reopen call
// off (the gate asks for it, the glue drops it) — the rebuild drains the
// picker and nothing brings it back.
func TestAppearanceForegroundChangeReopensTheSheet(t *testing.T) {
	controlCentre := orders(
		[]string{"X", "V1", "E", "C1"},
		[][2]string{{"X", "V1"}, {"V1", "E"}, {"V1", "C1"}},
	)
	t.Run("gate", func(t *testing.T) {
		s := &gateSim{g: appearanceGate{mobile: true, built: dark}, live: dark, sheet: true}
		s.run([]string{"V1", "C1"}, map[string]fyne.ThemeVariant{"V1": light})
		if s.rebuilds != 1 || s.reopens != 1 {
			t.Errorf("in the foreground: %d rebuilds, %d reopens, want 1 and 1", s.rebuilds, s.reopens)
		}
		for _, o := range controlCentre {
			s := &gateSim{g: appearanceGate{mobile: true, built: dark}, live: dark, sheet: true}
			s.run(o, map[string]fyne.ThemeVariant{"V1": light})
			if s.rebuilds != 1 || s.reopens != 1 {
				t.Errorf("Control Centre order %v: %d rebuilds, %d reopens, want 1 and 1", o, s.rebuilds, s.reopens)
			}
		}
	})
	t.Run("window", func(t *testing.T) {
		h := newAppearanceHarness(t, true)
		sheet := h.openGotoWithVerse("3", "")
		gen := windowRebuildGen
		h.flip()
		if windowRebuildGen != gen+1 {
			t.Fatalf("want one rebuild, got %d", windowRebuildGen-gen)
		}
		again := h.top()
		if again == nil || again == sheet || !again.Visible() {
			t.Fatal("the sheet must come back, a new one")
		}
		if !sheetHas(again, "Go to") {
			t.Errorf("the reopened sheet is not the Go to picker: %v", sheetTexts(again))
		}
		if c := textColour(again, "Go to"); c != lightPalette.Text {
			t.Errorf("the reopened picker's title is %v, want the light palette's %v", c, lightPalette.Text)
		}
	})
}

// --- d. no sheet --------------------------------------------------------------

// WITH NO SHEET OPEN a real change rebuilds once and reopens nothing, and a
// round trip in the background does nothing at all. Mutations guarded: the
// gate off (the round trip rebuilds) and the reconcile off (the change made
// while away is never applied).
func TestAppearanceWithoutASheet(t *testing.T) {
	t.Run("gate", func(t *testing.T) {
		for _, o := range roundTripOrders() {
			s := &gateSim{g: appearanceGate{mobile: true, built: dark}, live: dark}
			s.run(o, map[string]fyne.ThemeVariant{"V1": light, "V2": dark})
			if s.rebuilds != 0 {
				t.Errorf("round trip, order %v: %d rebuilds", o, s.rebuilds)
			}
		}
		for _, o := range [][]string{{"V1", "C1"}, {"X", "V1", "C1", "E"}, {"X", "V1", "E", "C1"}} {
			s := &gateSim{g: appearanceGate{mobile: true, built: dark}, live: dark}
			s.run(o, map[string]fyne.ThemeVariant{"V1": light})
			if s.rebuilds != 1 || s.reopens != 0 {
				t.Errorf("real change, order %v: %d rebuilds, %d reopens, want 1 and 0", o, s.rebuilds, s.reopens)
			}
		}
	})
	t.Run("window", func(t *testing.T) {
		h := newAppearanceHarness(t, true)
		gen := windowRebuildGen
		for _, step := range []string{"X", "V1", "C1", "V2", "C2", "E"} {
			h.step(step, map[string]fyne.ThemeVariant{"V1": light, "V2": dark})
		}
		if windowRebuildGen != gen {
			t.Fatalf("the round trip rebuilt the window %d times", windowRebuildGen-gen)
		}
		for _, step := range []string{"X", "V1", "C1", "E"} {
			h.step(step, map[string]fyne.ThemeVariant{"V1": light})
		}
		if windowRebuildGen != gen+1 {
			t.Fatalf("the change made while away must rebuild once, got %d", windowRebuildGen-gen)
		}
		if h.overlays() != 0 {
			t.Error("nothing was open, so nothing may be opened")
		}
	})
}

// --- e. desktop ---------------------------------------------------------------

// DESKTOP HAS NO SNAPSHOT ROUND TRIP, and a window behind other windows is
// still on screen: a change heard while the window is not focused rebuilds
// there and then, with the sheet reopened; the return of focus owes nothing.
// Mutation guarded: the gate applied on every platform (mobile ignored) — the
// unfocused window keeps the old palette until focus returns, and the reopen
// waits with it.
func TestAppearanceDesktopRebuildsWhileUnfocused(t *testing.T) {
	t.Run("gate", func(t *testing.T) {
		s := &gateSim{g: appearanceGate{mobile: false, built: dark}, live: dark, sheet: true}
		s.run([]string{"X", "V1", "C1"}, map[string]fyne.ThemeVariant{"V1": light})
		if s.rebuilds != 1 || s.reopens != 1 {
			t.Fatalf("unfocused: %d rebuilds, %d reopens before focus returns, want 1 and 1", s.rebuilds, s.reopens)
		}
		s.run([]string{"E"}, nil)
		if s.rebuilds != 1 {
			t.Errorf("focus returning must not rebuild again (%d)", s.rebuilds)
		}
		if s.g.background {
			t.Error("the gate never closes on desktop")
		}
	})
	t.Run("window", func(t *testing.T) {
		h := newAppearanceHarness(t, false)
		sheet := h.openGotoWithVerse("1", "")
		gen := windowRebuildGen
		observeAppearance(h.state, appearanceExitedForeground)
		h.flip()
		if windowRebuildGen != gen+1 {
			t.Fatalf("a desktop window behind others must repaint on the change, got %d rebuilds", windowRebuildGen-gen)
		}
		if again := h.top(); again == nil || again == sheet || !again.Visible() {
			t.Fatal("the sheet must be reopened on desktop too")
		}
		observeAppearance(h.state, appearanceEnteredForeground)
		if windowRebuildGen != gen+1 {
			t.Error("focus returning owes nothing on desktop")
		}
	})
}

// --- f. the registration's life -------------------------------------------------

// A SHEET THE READER CLOSED NEVER COMES BACK. Its registration is cleared by
// every close path a test can drive — its close button, its own action, Escape,
// a sheet opened over it, a rebuild's drain — and a reopened sheet registers
// again, so a second change brings it back a second time. Mutation guarded:
// clearing on close off (pruning keeps every registration) — each closed
// sheet's closure, and the whole drained sheet it holds, stays registered.
func TestSheetReopenIsClearedOnEveryClose(t *testing.T) {
	votd := func(t *testing.T, h *appearanceHarness) *widget.PopUp {
		t.Helper()
		showVerseOfDay(h.state)
		p := h.top()
		if p == nil || !sheetHas(p, "Verse of the day") {
			t.Fatal("the verse of the day card did not open")
		}
		if !registered(h.state, p) {
			t.Fatal("control: the open card must hold a reopen")
		}
		return p
	}
	// afterwards: nothing registered, and a change reopens nothing.
	nothingComesBack := func(t *testing.T, h *appearanceHarness, closed *widget.PopUp) {
		t.Helper()
		if registered(h.state, closed) {
			t.Error("the closed sheet still holds its reopen")
		}
		if n := len(h.state.sheetReopens); n != 0 {
			t.Errorf("%d registrations outlive the sheets they belong to", n)
		}
		h.flip()
		if h.overlays() != 0 {
			t.Errorf("a change brought back a sheet the reader closed: %v", sheetTexts(h.top()))
		}
	}

	t.Run("its close button", func(t *testing.T) {
		h := newAppearanceHarness(t, true)
		p := votd(t, h)
		test.Tap(findTreeButton(p, "Close"))
		nothingComesBack(t, h, p)
	})
	t.Run("its own action", func(t *testing.T) {
		h := newAppearanceHarness(t, true)
		p := votd(t, h)
		test.Tap(findTreeButton(p, "Read in context"))
		nothingComesBack(t, h, p)
	})
	t.Run("Escape", func(t *testing.T) {
		h := newAppearanceHarness(t, true)
		p := votd(t, h)
		if h.state.dismissSheet == nil {
			t.Fatal("control: the card must install the Escape hook")
		}
		h.state.dismissSheet()
		nothingComesBack(t, h, p)
	})
	t.Run("a sheet replacing it", func(t *testing.T) {
		h := newAppearanceHarness(t, true)
		// Closed, and another opened in its place.
		picker := h.openGotoWithVerse("16", "")
		test.Tap(findTreeButton(picker, "Go"))
		if registered(h.state, picker) {
			t.Error("the committed picker still holds its reopen")
		}
		card := votd(t, h)
		h.flip()
		again := h.top()
		if h.overlays() != 1 || again == card || !sheetHas(again, "Verse of the day") {
			t.Fatalf("only the card may come back; overlays %d, texts %v", h.overlays(), sheetTexts(again))
		}
		// Opened OVER it: only the top comes back, and the one beneath is gone.
		under := h.openGotoWithVerse("7", "")
		_ = under
		showVersionPicker(h.state)
		over := h.top()
		if h.overlays() != 3 { // the reopened card, the picker, the translation list
			t.Fatalf("control: want three stacked sheets, have %d", h.overlays())
		}
		h.flip()
		if h.overlays() != 1 || h.top() == over || !sheetHas(h.top(), "Translation") {
			t.Fatalf("only the top sheet may come back; overlays %d, texts %v", h.overlays(), sheetTexts(h.top()))
		}
		if len(h.state.sheetReopens) != 1 {
			t.Errorf("the sheets beneath were drained, so only the top's reopen may remain; have %d", len(h.state.sheetReopens))
		}
	})
	t.Run("a rebuild's drain", func(t *testing.T) {
		h := newAppearanceHarness(t, true)
		p := votd(t, h)
		rebuildWindow(h.state) // a rotation, a translation switch — not a light/dark change
		// Read straight after the rebuild, before anything that prunes: the
		// drain itself must let the drained card's closure go.
		if n := len(h.state.sheetReopens); n != 0 {
			t.Errorf("the drain left %d registrations holding drained sheets", n)
		}
		if registered(h.state, p) {
			t.Error("a drained sheet's reopen must go with it")
		}
		// A sheet with no reopen of its own on top: the drained card's closure
		// must not stand in for it.
		bare := widget.NewModalPopUp(widget.NewLabel("no reopen"), h.state.window.Canvas())
		bare.Show()
		h.flip()
		if h.overlays() != 0 {
			t.Errorf("the change reopened a sheet the rebuild had closed: %v", sheetTexts(h.top()))
		}
	})
	// A sheet with no way back ON TOP of one that has: only the top answers,
	// so both close — the one beneath must not come back without the choice
	// being made over it (the model picker over Settings). Mutation guarded:
	// the take answering with the most recent live registration, whatever is
	// on top.
	t.Run("an unregistered sheet over it", func(t *testing.T) {
		h := newAppearanceHarness(t, true)
		votd(t, h)
		over := widget.NewModalPopUp(widget.NewLabel("a choice being made"), h.state.window.Canvas())
		over.Show()
		if h.overlays() != 2 {
			t.Fatalf("control: want the card and the sheet over it, have %d", h.overlays())
		}
		h.flip()
		if h.overlays() != 0 {
			t.Errorf("the sheet beneath came back without the one on top: %v", sheetTexts(h.top()))
		}
	})
	t.Run("the reopened sheet registers again", func(t *testing.T) {
		h := newAppearanceHarness(t, true)
		first := votd(t, h)
		h.flip()
		second := h.top()
		if second == nil || second == first || !registered(h.state, second) {
			t.Fatal("the reopened card must register its own reopen")
		}
		if registered(h.state, first) {
			t.Error("the drained card's reopen must be gone")
		}
		h.flip()
		third := h.top()
		if third == nil || third == second || !third.Visible() || !sheetHas(third, "Verse of the day") {
			t.Fatal("a second change must bring the card back a second time")
		}
		if h.overlays() != 1 || len(h.state.sheetReopens) != 1 {
			t.Errorf("one card, one registration; have %d and %d", h.overlays(), len(h.state.sheetReopens))
		}
	})
}

// --- g. the same thing, shown again ---------------------------------------------

// THE GO TO PICKER COMES BACK AS THE READER LEFT IT: the same book selected
// (not the one they are reading), the navigator at the same letter, the same
// chapter selected, the verse fields' text — and live, so Go commits it.
// Mutation guarded: the reopen call off (nothing comes back).
func TestGotoPickerReopensAsTheReaderLeftIt(t *testing.T) {
	h := newAppearanceHarness(t, true)
	showGotoPicker(h.state)
	p := h.top()
	test.Tap(findTreeButton(p, "P"))
	test.Tap(findTreeButton(p, "Psalms"))
	test.Tap(findTreeButton(p, "42"))
	findNumberEntry(p, "verse").SetText("1")
	findNumberEntry(p, "end").SetText("5")
	if h.state.CurrentBook != "John" {
		t.Fatal("control: selecting in the picker must not navigate")
	}

	h.flip()
	again := h.top()
	if again == nil || again == p || !again.Visible() {
		t.Fatal("the picker must come back, rebuilt")
	}
	if b := findTreeButton(again, "Psalms"); b == nil || b.Importance != widget.HighImportance {
		t.Error("the navigator must be back at P with Psalms selected")
	}
	if !sheetHas(again, "Psalms · 2 chapters") {
		t.Errorf("the chapter grid must show Psalms; texts %v", sheetTexts(again))
	}
	if b := findTreeButton(again, "42"); b == nil || b.Importance != widget.HighImportance {
		t.Error("chapter 42 must still be selected")
	}
	if b := findTreeButton(again, "23"); b != nil && b.Importance == widget.HighImportance {
		t.Error("only the chapter the reader chose may be selected")
	}
	if s, e := findNumberEntry(again, "verse").Text, findNumberEntry(again, "end").Text; s != "1" || e != "5" {
		t.Errorf("the verse fields read %q and %q, want 1 and 5", s, e)
	}
	test.Tap(findTreeButton(again, "Go"))
	if h.state.CurrentBook != "Psalms" || h.state.CurrentChapter != 42 {
		t.Errorf("Go on the reopened picker landed on %s %d, want Psalms 42", h.state.CurrentBook, h.state.CurrentChapter)
	}

	// The book-list flavour: the book selected in the list.
	showChapterPicker(h.state)
	list := h.top()
	var books *widget.List
	walkTree(list, func(o fyne.CanvasObject) {
		if l, ok := o.(*widget.List); ok && books == nil {
			books = l
		}
	})
	for i, b := range h.state.Bible.Books {
		if b == "Genesis" {
			books.Select(i)
		}
	}
	if !sheetHas(list, "Genesis · 2 chapters") {
		t.Fatal("control: the list must select Genesis")
	}
	h.flip()
	if again := h.top(); again == list || !sheetHas(again, "Genesis · 2 chapters") {
		t.Errorf("the chapter picker must come back on Genesis; texts %v", sheetTexts(again))
	}
}

// VERSE OF THE DAY COMES BACK WITH THE SAME VERSE, even when the day has
// turned while it was open — the overnight change is exactly when a card left
// up meets a light/dark switch. Mutation guarded: the reopen asking the
// calendar again (showVerseOfDay instead of the card's own passage) — the card
// comes back with the next day's verse.
func TestVerseOfDayReopensWithTheSameVerse(t *testing.T) {
	h := newAppearanceHarness(t, true)
	// Two days whose picks differ on this edition.
	var dayA, dayB int64 = -1, -1
	var refA, refB string
	for d := int64(20000); d < 20400 && dayB < 0; d++ {
		v, ok := verseOfTheDayAt(h.state, d)
		if !ok {
			continue
		}
		switch {
		case dayA < 0:
			dayA, refA = d, v.reference()
		case v.reference() != refA:
			dayB, refB = d, v.reference()
		}
	}
	if dayA < 0 || dayB < 0 {
		t.Fatal("control: the sample edition must give two different verses of the day")
	}
	at := func(day int64) func() time.Time {
		return func() time.Time { return time.Unix(day*86400+12*3600, 0).UTC() }
	}
	prev := verseOfDayNow
	t.Cleanup(func() { verseOfDayNow = prev })
	cite := func(ref string) string { return fmt.Sprintf("%s · %s", ref, h.state.currentVersion().Abbrev) }

	verseOfDayNow = at(dayA)
	showVerseOfDay(h.state)
	card := h.top()
	if !sheetHas(card, cite(refA)) {
		t.Fatalf("control: the card must show %s; texts %v", refA, sheetTexts(card))
	}
	verseOfDayNow = at(dayB) // the night passes with the card up
	h.flip()
	again := h.top()
	if again == nil || again == card {
		t.Fatal("the card must come back, rebuilt")
	}
	if !sheetHas(again, cite(refA)) || sheetHas(again, cite(refB)) {
		t.Errorf("the reopened card must show %s, the verse the reader was reading; texts %v", refA, sheetTexts(again))
	}
}

// --- every sheet that registers, and one that must not ------------------------------

// EVERY SHEET THAT REGISTERS COMES BACK: opened for real, one light/dark change
// in the foreground, and the same sheet is on top again — a new popup, in the
// new palette. The sheets whose opening needs a network, a timer or a native
// view (cross-references, the AI answer, the audio menu) register the same
// one-line way and are covered by reading, not here.
func TestEveryRegisteredSheetComesBack(t *testing.T) {
	target := ShareTarget{VersionID: defaultVersionID, Book: "John", Chapter: 3, VerseLo: 16, Note: "a note"}
	for _, c := range []struct {
		name string
		open func(*AppState)
		mark string
	}{
		{"Go to", showGotoPicker, "Go to"},
		{"the chapter picker", showChapterPicker, "Go to"},
		{"Verse of the day", showVerseOfDay, "Verse of the day"},
		{"the translation picker", showVersionPicker, "Translation"},
		{"Settings", showAISettings, "Settings"},
		{"a link notice", func(s *AppState) { showLinkVersionUnavailable(s, "New King James Version") }, "Shared in New King James Version"},
		{"the download error", func(s *AppState) { showVersionLoadError(s, "World English Bible") }, "OK"},
		{"the note-link offer", func(s *AppState) { offerNoteLinkChoice(s, "https://bibletext.co.uk/web/john/3/#v16", target) }, "Someone added a note"},
		{"Ask", func(s *AppState) { promptAskQuestion(s, "For God so loved the world") }, "Ask about this passage"},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := newAppearanceHarness(t, true)
			c.open(h.state)
			p := h.top()
			if p == nil || !sheetHas(p, c.mark) {
				t.Fatalf("control: the sheet did not open showing %q", c.mark)
			}
			h.flip()
			again := h.top()
			if again == nil || again == p || !again.Visible() || h.overlays() != 1 {
				t.Fatalf("the sheet must come back, rebuilt, alone (overlays %d)", h.overlays())
			}
			if !sheetHas(again, c.mark) {
				t.Errorf("a different sheet came back: %v", sheetTexts(again))
			}
		})
	}
}

// THE ASK SHEET KEEPS THE QUESTION TYPED SO FAR.
func TestAskSheetReopensWithTheQuestion(t *testing.T) {
	h := newAppearanceHarness(t, true)
	question := func(p *widget.PopUp) *searchKeyEntry {
		var found *searchKeyEntry
		if p == nil {
			return nil
		}
		walkTree(p, func(o fyne.CanvasObject) {
			if e, ok := o.(*searchKeyEntry); ok && found == nil {
				found = e
			}
		})
		return found
	}
	promptAskQuestion(h.state, "For God so loved the world")
	entry := question(h.top())
	if entry == nil {
		t.Fatal("control: no question field")
	}
	entry.SetText("Who is speaking?")
	h.flip()
	again := question(h.top())
	if again == nil || again == entry || again.Text != "Who is speaking?" {
		t.Errorf("the reopened sheet must hold the typed question")
	}
}

// A SHEET WITH NO WAY BACK SIMPLY CLOSES — the rebuild still happens, at once,
// and nothing stale is left on the canvas. The share preview is one of the
// sheets that deliberately registers nothing (sheet_reopen.go).
func TestAnUnregisteredSheetCloses(t *testing.T) {
	h := newAppearanceHarness(t, true)
	showShareImagePreview(h.state, "For God so loved the world", "John 3:16", "WEB")
	if p := h.top(); p == nil || !sheetHas(p, "Share as image") {
		t.Fatal("control: the preview did not open")
	}
	gen := windowRebuildGen
	h.flip()
	if windowRebuildGen != gen+1 {
		t.Fatalf("the change must rebuild at once, sheet or no sheet; got %d", windowRebuildGen-gen)
	}
	if h.overlays() != 0 {
		t.Errorf("the preview must close, not come back: %v", sheetTexts(h.top()))
	}
}

// --- the palette -------------------------------------------------------------------

// textColour is the colour of the first canvas.Text reading want under p.
func textColour(p *widget.PopUp, want string) color.Color {
	var found color.Color
	if p == nil {
		return nil
	}
	walkTree(p, func(o fyne.CanvasObject) {
		if t, ok := o.(*canvas.Text); ok && found == nil && t.Text == want {
			found = t.Color
		}
	})
	return found
}

// THE REOPENED SHEET IS IN THE NEW PALETTE — the whole point of the fix: not a
// new popup with the right words in the old colours. The harness's variant is
// the one the palette reads (isDark asks appearanceVariant), so a flip really
// re-lights what the reopen builds. Mutation guarded: a reopen that re-shows
// the drained sheet's own canvas objects in a new popup, and isDark asking
// the settings directly (the palette then never moves under the harness).
func TestTheReopenedSheetIsInTheNewPalette(t *testing.T) {
	if darkPalette.Accent == lightPalette.Accent || darkPalette.Text == lightPalette.Text {
		t.Fatal("control: the two palettes must differ where this looks")
	}
	h := newAppearanceHarness(t, true)
	showVerseOfDay(h.state)
	if c := textColour(h.top(), "Verse of the day"); c != darkPalette.Accent {
		t.Fatalf("control: the card opened in %v, want the dark palette's accent", c)
	}
	for _, want := range []struct {
		v fyne.ThemeVariant
		p palette
	}{{light, lightPalette}, {dark, darkPalette}} {
		h.flip()
		if h.variant != want.v {
			t.Fatalf("control: the harness is at %v, want %v", h.variant, want.v)
		}
		if c := textColour(h.top(), "Verse of the day"); c != want.p.Accent {
			t.Errorf("after the change to %v the card's kicker is %v, want %v", want.v, c, want.p.Accent)
		}
	}
}

// --- menus, toasts and the caret ---------------------------------------------------------

// A MENU OR A TOAST OVER A SHEET DOES NOT STAND IN FOR IT. The entry's
// Cut/Copy/Paste menu over the Go to picker (a right-click, or a long-press on
// a phone) and the desktop share confirmation over Verse of the day are not
// sheets; the take looks past them, the drain closes them, and the sheet
// beneath comes back with what the reader had in it. Mutation guarded: the
// take answering only for the overlay on top (the picker and the card then
// close for good, the typed verse with them).
func TestMenusAndToastsDoNotHideTheSheetBeneath(t *testing.T) {
	for _, mobile := range []bool{false, true} {
		t.Run(fmt.Sprintf("the context menu over Go to, mobile %v", mobile), func(t *testing.T) {
			h := newAppearanceHarness(t, mobile)
			sheet := h.openGotoWithVerse("16", "18")
			test.TapSecondary(findNumberEntry(sheet, "verse"))
			if _, isSheet := h.state.window.Canvas().Overlays().Top().(*widget.PopUp); isSheet || h.overlays() != 2 {
				t.Fatalf("control: want Fyne's menu over the picker, have %T of %d",
					h.state.window.Canvas().Overlays().Top(), h.overlays())
			}
			h.flip()
			again := h.top()
			if again == nil || again == sheet || h.overlays() != 1 {
				t.Fatalf("the picker under the menu must come back, alone; overlays %d", h.overlays())
			}
			if s, e := findNumberEntry(again, "verse").Text, findNumberEntry(again, "end").Text; s != "16" || e != "18" {
				t.Errorf("the reopened picker reads %q to %q, want 16 to 18", s, e)
			}
		})
	}
	t.Run("the share confirmation over Verse of the day", func(t *testing.T) {
		h := newAppearanceHarness(t, false)
		showVerseOfDay(h.state)
		card := h.top()
		// showShareNotice's own popup, without its 1.4s timer: a timer's
		// fyne.Do runs on the timer's goroutine under the test driver and would
		// touch the overlay stack while a later test runs.
		toast := widget.NewPopUp(widget.NewLabel("Copied to the clipboard"), h.state.window.Canvas())
		shareNotice = toast
		t.Cleanup(func() { shareNotice = nil })
		toast.ShowAtPosition(fyne.NewPos(10, 10))
		if !selfDismissingOverlay(toast) || h.top() != toast {
			t.Fatal("control: the confirmation must be on top and known for what it is")
		}
		h.flip()
		again := h.top()
		if again == nil || again == card || again == toast || h.overlays() != 1 || !sheetHas(again, "Verse of the day") {
			t.Fatalf("the card beneath the confirmation must come back, alone; overlays %d, texts %v",
				h.overlays(), sheetTexts(again))
		}
		if toast.Visible() {
			t.Error("the confirmation itself must close with the drain")
		}
	})
}

// focusProbe is a field that counts the times it is told it lost the caret.
type focusProbe struct {
	widget.Entry
	lost int
}

func (f *focusProbe) FocusLost() { f.lost++; f.Entry.FocusLost() }

// THE CARET IS DROPPED BEFORE THE DRAIN, AND PUT BACK. The drain takes a sheet
// off the stack with its focus manager and tells nobody, and a phone's keyboard
// goes down only on an unfocus — so without the drop the keyboard stayed up
// over the reopened sheet, typing into nothing. The Go to picker, whose reader
// was typing a verse, gets the caret back in the same field. Mutations
// guarded: the unfocus off (the field is never told), and the capture not
// asking which field had the caret (the reopened picker has none).
func TestAppearanceChangeDropsTheCaretAndPutsItBack(t *testing.T) {
	t.Run("the drained field is told", func(t *testing.T) {
		h := newAppearanceHarness(t, true)
		cnv := h.state.window.Canvas()
		probe := &focusProbe{}
		probe.ExtendBaseWidget(probe)
		p := widget.NewModalPopUp(probe, cnv)
		p.Show()
		registerSheetReopen(h.state, p, func() {})
		cnv.Focus(probe)
		if cnv.Focused() != probe || probe.lost != 0 {
			t.Fatal("control: the probe must hold the caret")
		}
		h.flip()
		if probe.lost != 1 {
			t.Errorf("the drained sheet's field was told it lost the caret %d times, want 1", probe.lost)
		}
	})
	for _, field := range []string{"verse", "end"} {
		t.Run("Go to, the "+field+" field", func(t *testing.T) {
			h := newAppearanceHarness(t, true)
			cnv := h.state.window.Canvas()
			sheet := h.openGotoWithVerse("16", "18")
			cnv.Focus(findNumberEntry(sheet, field))
			h.flip()
			again := h.top()
			if again == nil || again == sheet {
				t.Fatal("the picker must come back")
			}
			if want := findNumberEntry(again, field); want == nil || cnv.Focused() != fyne.Focusable(want) {
				t.Errorf("the caret must be back in the reopened %s field; focused %v", field, cnv.Focused())
			}
		})
	}
	t.Run("Go to, no field", func(t *testing.T) {
		h := newAppearanceHarness(t, true)
		h.openGotoWithVerse("16", "")
		h.flip()
		if f := h.state.window.Canvas().Focused(); f != nil {
			t.Errorf("a picker nobody was typing in must not come back with the caret (the number pad would rise unasked): %v", f)
		}
	})
}

// --- the sheets that promise, and the one that waits -------------------------------

// THE DOWNLOAD SPINNER COMES BACK UNTIL ITS DOWNLOAD LANDS. A change mid-download
// used to take it for good, leaving the reader free to open a sheet the
// landing's unconditional rebuild then closed. Now it reopens, the download's
// dismissal closes whichever copy is up, and the landing lands as before.
// Mutation guarded: the spinner registering nothing (after the change nothing
// is on screen while the download still runs).
func TestTheDownloadSpinnerComesBackUntilItsDownloadLands(t *testing.T) {
	h := newAppearanceHarness(t, true)
	var land func(*BibleData, dataMode, error)
	prev := startVersionLoad
	startVersionLoad = func(_ BibleVersion, _ *BibleData, l func(*BibleData, dataMode, error)) { land = l }
	t.Cleanup(func() { startVersionLoad = prev })

	switchVersionInteractive(h.state, "bsb", byReader)
	first := h.top()
	if land == nil || !sheetHas(first, "Downloading Berean Standard Bible…") {
		t.Fatalf("control: the download must be in flight behind its spinner; texts %v", sheetTexts(first))
	}
	h.flip()
	again := h.top()
	if again == nil || again == first || h.overlays() != 1 || !sheetHas(again, "Downloading Berean Standard Bible…") {
		t.Fatalf("the spinner must come back while its download runs; overlays %d, texts %v", h.overlays(), sheetTexts(again))
	}
	h.flip()
	third := h.top()
	if third == nil || third == again || !sheetHas(third, "Downloading Berean Standard Bible…") {
		t.Fatal("a second change must bring the spinner back again")
	}
	land(sampleState().Bible, modeReal, nil)
	if h.overlays() != 0 || third.Visible() {
		t.Errorf("the landing must take down the spinner that is showing; overlays %d", h.overlays())
	}
	if h.state.CurrentVersion != "bsb" || h.state.versionLoading {
		t.Errorf("the download must land as before: version %q, loading %v", h.state.CurrentVersion, h.state.versionLoading)
	}
	h.flip()
	if h.overlays() != 0 {
		t.Errorf("a spinner whose download has landed must never come back: %v", sheetTexts(h.top()))
	}
}

// THE READING VIEW STAYS DOWN UNDER A SHEET THE SPINNER SAT OVER. The spinner's
// dismissal and the error card's OK used to restore the native reading overlay
// whatever was still open, putting the verses over the sheet beneath; they now
// restore only onto a clear canvas, as the watchdogs do. Mutation guarded:
// either restore unguarded.
func TestTheSpinnerAndTheErrorCardRestoreOnlyOntoAClearCanvas(t *testing.T) {
	// An empty cache of its own, so the failed download has no previous
	// edition to fall back on and takes the error card's arm for certain.
	t.Setenv("BIBLETEXT_CACHE_PATH", filepath.Join(t.TempDir(), cacheFileName))
	h := newAppearanceHarness(t, true)
	var land func(*BibleData, dataMode, error)
	prev := startVersionLoad
	startVersionLoad = func(_ BibleVersion, _ *BibleData, l func(*BibleData, dataMode, error)) { land = l }
	t.Cleanup(func() { startVersionLoad = prev })

	showVerseOfDay(h.state)
	card := h.top()
	shown := 0
	h.state.showReadingOverlay = func() { shown++ }
	h.state.hideReadingOverlay = func() {}
	switchVersionInteractive(h.state, "bsb", byReader) // a link's switch, say, over the open card
	if land == nil {
		t.Fatal("control: the download must be in flight")
	}
	land(nil, modeReal, fmt.Errorf("offline"))
	errCard := h.top()
	if !sheetHas(errCard, "OK") || !card.Visible() {
		t.Fatalf("control: the error card must be up over the card; texts %v", sheetTexts(errCard))
	}
	test.Tap(findTreeButton(errCard, "OK"))
	if h.top() != card {
		t.Fatal("control: the card must be what is left")
	}
	if shown != 0 {
		t.Errorf("the reading view was restored %d times over the open card", shown)
	}
}

// THE NOTE COMPOSER COMES BACK WITH THE NOTE, on the desktops and Android,
// where the field is Fyne's (iOS's native field takes the same reopen; its
// teardown is ownership-guarded, share_note_ui.go). Mutation guarded: the
// composer registering nothing (the note written so far is lost).
func TestTheNoteComposerComesBackWithTheNote(t *testing.T) {
	h := newAppearanceHarness(t, false)
	field := func(p *widget.PopUp) *searchKeyEntry {
		var found *searchKeyEntry
		if p == nil {
			return nil
		}
		walkTree(p, func(o fyne.CanvasObject) {
			if e, ok := o.(*searchKeyEntry); ok && found == nil {
				found = e
			}
		})
		return found
	}
	promptShareNote(h.state, "For God so loved the world", selSpan{})
	entry := field(h.top())
	if entry == nil || !sheetHas(h.top(), "Add a note") {
		t.Fatal("control: the composer did not open")
	}
	entry.SetText("Read this before Sunday")
	h.flip()
	again := field(h.top())
	if again == nil || again == entry || again.Text != "Read this before Sunday" {
		t.Fatalf("the composer must come back holding the note; overlays %d", h.overlays())
	}
	if !sheetHas(h.top(), fmt.Sprintf("%d characters left", NoteMaxRunes-len("Read this before Sunday"))) {
		t.Errorf("the counter must count the note it came back with; texts %v", sheetTexts(h.top()))
	}
	if h.state.window.Canvas().Focused() != fyne.Focusable(again) {
		t.Error("the reopened composer takes the caret, as any open does")
	}
}

// THE REOPENED TRANSLATION PICKER SAYS WHAT IS TRUE NOW, AND ASKS FOR NOTHING.
// A download that landed while the picker was up had its window rebuild
// deferred to the picker's close; the light/dark rebuild is the one that paints
// it, so the reopened picker must not still say the text is downloading. And
// the reopen is not the reader opening the picker, so it must not repeat the
// manual retry an opening performs. Mutations guarded: reopening with the
// at-open notice (still "downloading" over the full text), and reopening
// through showVersionPicker (a second fetch, the backoff zeroed).
func TestTheReopenedPickerSaysWhatIsTrueNow(t *testing.T) {
	var lands []func(*BibleData, dataMode, error)
	prev := startUpgradeFetch
	startUpgradeFetch = func(_ BibleVersion, l func(*BibleData, dataMode, error)) { lands = append(lands, l) }
	t.Cleanup(func() { startUpgradeFetch = prev })

	t.Run("a download that landed under it", func(t *testing.T) {
		lands = nil
		h := newAppearanceHarness(t, true)
		h.state.fullPending, h.state.seedOnly = true, true
		const downloading = "The full World English Bible is still downloading — a starter portion is shown meanwhile."
		showVersionPicker(h.state)
		if !sheetHas(h.top(), downloading) || len(lands) != 1 {
			t.Fatalf("control: the picker must open saying so, and retry once (%d)", len(lands))
		}
		lands[0](sampleState().Bible, modeReal, nil)
		if !h.state.fullRebuildDeferred {
			t.Fatal("control: the landing's rebuild must wait for the picker")
		}
		h.flip()
		if h.state.fullRebuildDeferred {
			t.Fatal("control: the change's rebuild must be the one that paints the landing")
		}
		if again := h.top(); again == nil || !sheetHas(again, "Translation") || sheetHas(again, downloading) {
			t.Errorf("the reopened picker still says the text is downloading over the full text; texts %v", sheetTexts(again))
		}
	})
	t.Run("no second retry", func(t *testing.T) {
		lands = nil
		h := newAppearanceHarness(t, true)
		// Waiting out the backoff offline, on a previous edition.
		h.state.fullPending, h.state.seedOnly = true, false
		h.state.fullRetryDelay = 40 * time.Second
		const waiting = "World English Bible has a text update waiting for a connection — the previous edition is shown meanwhile. It retries automatically."
		showVersionPicker(h.state)
		if !sheetHas(h.top(), waiting) || len(lands) != 1 {
			t.Fatalf("control: the picker must say the update is waiting, and retry once (%d); texts %v", len(lands), sheetTexts(h.top()))
		}
		lands[0](nil, modeReal, fmt.Errorf("offline")) // the retry fails: waiting again
		if h.state.fullDownloading || h.state.fullRetryDelay <= 0 {
			t.Fatal("control: the failed retry must leave the update waiting on the backoff")
		}
		delay := h.state.fullRetryDelay
		h.flip()
		if len(lands) != 1 {
			t.Errorf("the reopen started %d more fetches; a rebuild is not the reader asking again", len(lands)-1)
		}
		if h.state.fullRetryDelay != delay {
			t.Errorf("the reopen reset the backoff from %v to %v", delay, h.state.fullRetryDelay)
		}
		if !sheetHas(h.top(), waiting) {
			t.Errorf("the reopened picker must still say the update is waiting; texts %v", sheetTexts(h.top()))
		}
	})
}

// THE SEED PARK'S NOTICE COMES BACK ONLY WHILE ITS PASSAGE IS STILL WAITING. It
// promises the passage "will open on its own"; a rebuild that consumes the
// deferred download keeps that promise, and the card must not then reappear
// word for word over the passage it promised. Mutation guarded: the notice
// reopening regardless.
func TestTheParkNoticeComesBackOnlyWhileTheParkWaits(t *testing.T) {
	const promise = "Psalms 23:1 isn't in the part of the Bible that has downloaded yet. The rest is still arriving, and this passage will open on its own as soon as it does."
	parked := func(t *testing.T) *appearanceHarness {
		t.Helper()
		app := test.NewApp()
		t.Cleanup(app.Quit)
		h := &appearanceHarness{t: t, variant: dark}
		prev := appearanceVariant
		appearanceVariant = func(*AppState) fyne.ThemeVariant { return h.variant }
		t.Cleanup(func() { appearanceVariant = prev })
		h.state = seedState(t)
		win := app.NewWindow("park")
		t.Cleanup(win.Close)
		h.state.app, h.state.window = app, win
		win.SetContent(CreateMainUI(app, h.state, win))
		win.Resize(fyne.NewSize(900, 760))
		h.state.appearance = appearanceGate{mobile: true, built: dark}
		applyShareTarget(h.state, ShareTarget{VersionID: defaultVersionID, Book: "Psalms", Chapter: 23, VerseLo: 1})
		if h.state.pendingLink == nil || !sheetHas(h.top(), promise) {
			t.Fatalf("control: the link must park and say so; texts %v", sheetTexts(h.top()))
		}
		return h
	}
	t.Run("still waiting", func(t *testing.T) {
		h := parked(t)
		first := h.top()
		h.flip()
		if again := h.top(); again == nil || again == first || !sheetHas(again, promise) {
			t.Errorf("the promise still stands, so the notice must come back; texts %v", sheetTexts(again))
		}
	})
	t.Run("kept by the rebuild", func(t *testing.T) {
		h := parked(t)
		version, _ := versionByID(defaultVersionID)
		applyFullDownload(h.state, version, fullBible(), modeReal)
		if !h.state.fullRebuildDeferred || h.state.pendingLink == nil {
			t.Fatal("control: the download must land under the notice and leave the park waiting")
		}
		h.flip()
		if h.state.CurrentBook != "Psalms" || h.state.CurrentChapter != 23 || h.state.pendingLink != nil {
			t.Fatalf("control: the rebuild must open the parked passage; at %s %d", h.state.CurrentBook, h.state.CurrentChapter)
		}
		if h.overlays() != 0 {
			t.Errorf("the notice came back promising a passage that is already open: %v", sheetTexts(h.top()))
		}
	})
}

// --- the wiring ----------------------------------------------------------------------

// THE GATE IS WIRED WHERE THE DECISION SAYS. Every test above calls the gate
// directly, so this holds the three places that make the app behave that way:
// the settings listener hands every change to the gate and never defers; the
// gate closes as the FIRST thing the background hook does and reconciles
// straight after the zone read in the foreground hook; and the mobile flag is
// set from the device. The hooks are then run for real — the ones
// InstallReadingStateFlush installs — through a round trip and a real change
// with a sheet open. Mutations guarded: the listener deferring again, either
// hook call removed or moved (after the flush, to the end, behind a fyne.Do),
// and the mobile flag never set.
func TestTheAppearanceGateIsWiredToTheListenerAndTheLifecycle(t *testing.T) {
	src := readSourceForShape(t, "app.go")
	body := func(from, to string) string {
		t.Helper()
		i := strings.Index(src, from)
		if i < 0 {
			t.Fatalf("app.go no longer contains %q", from)
		}
		rest := src[i+len(from):]
		if j := strings.Index(rest, to); j >= 0 {
			rest = rest[:j]
		}
		return rest
	}
	live := func(block string) []string {
		var out []string
		for _, line := range strings.Split(block, "\n") {
			if l := strings.TrimSpace(line); l != "" && !strings.HasPrefix(l, "//") {
				out = append(out, l)
			}
		}
		return out
	}

	observe := live(body("func ObserveSystemThemeChanges(", "\n}\n"))
	joined := strings.Join(observe, "\n")
	if !strings.Contains(joined, "fyne.Do(func() { observeAppearance(state, appearanceChanged) })") {
		t.Error("the settings listener no longer hands each change to the gate")
	}
	if strings.Contains(joined, "deferOrRebuild(") || strings.Contains(joined, "rebuildWindow(") {
		t.Error("the settings listener rebuilds or defers by itself; a light/dark change is the gate's to decide")
	}
	if !strings.Contains(joined, "state.appearance.mobile = fyne.CurrentDevice().IsMobile()") {
		t.Error("the gate's mobile flag is no longer set from the device, so a phone's snapshot rebuilds again")
	}

	exit := live(body("lc.SetOnExitedForeground(func() {", "\n\t})"))
	if len(exit) == 0 || exit[0] != "observeAppearance(state, appearanceExitedForeground)" {
		t.Errorf("the background hook must close the gate before anything else; its statements are %q", exit)
	}
	enter := live(body("lc.SetOnEnteredForeground(func() {", "\n\t})"))
	if len(enter) < 2 || enter[0] != "refreshLocalTimeZone()" || enter[1] != "observeAppearance(state, appearanceEnteredForeground)" {
		t.Errorf("the foreground hook must reconcile straight after the zone read; its statements are %q", enter)
	}
	// CONTROL: the shape reader must be able to see a statement that is there.
	if len(live("\t// a comment\n\tx()\n")) != 1 {
		t.Fatal("control: the statement reader is broken")
	}

	// The hooks themselves, as installed.
	h := newAppearanceHarness(t, true)
	prevCapture := captureAnchorFn
	captureAnchorFn = func() (int, float64, float64, bool) { return 0, 0, 0, false }
	t.Cleanup(func() { captureAnchorFn = prevCapture })
	InstallReadingStateFlush(h.state.app, nil, h.state)
	hooks, ok := h.state.app.Lifecycle().(interface {
		OnExitedForeground() func()
		OnEnteredForeground() func()
	})
	if !ok || hooks.OnExitedForeground() == nil || hooks.OnEnteredForeground() == nil {
		t.Fatal("control: InstallReadingStateFlush must install both foreground hooks")
	}
	sheet := h.openGotoWithVerse("16", "18")
	gen := windowRebuildGen
	hooks.OnExitedForeground()()
	for _, v := range []fyne.ThemeVariant{light, dark} { // the switcher's snapshot
		h.variant = v
		observeAppearance(h.state, appearanceChanged)
	}
	hooks.OnEnteredForeground()()
	if windowRebuildGen != gen || h.top() != sheet {
		t.Fatalf("the snapshot round trip through the real hooks rebuilt %d times; sheet kept %v",
			windowRebuildGen-gen, h.top() == sheet)
	}
	hooks.OnExitedForeground()()
	h.variant = light // a real change while away
	observeAppearance(h.state, appearanceChanged)
	if windowRebuildGen != gen {
		t.Fatal("a change heard in the background must wait for the return")
	}
	hooks.OnEnteredForeground()()
	if windowRebuildGen != gen+1 {
		t.Fatalf("the return must apply the change once, got %d rebuilds", windowRebuildGen-gen)
	}
	if again := h.top(); again == nil || again == sheet || findNumberEntry(again, "verse").Text != "16" {
		t.Error("the return's rebuild must bring the picker back as the reader left it")
	}
}
