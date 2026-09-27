package bibletext

// WHEN A LIGHT/DARK CHANGE REBUILDS THE WINDOW, AND WHEN IT IS IGNORED.
//
// A system appearance change has to rebuild the window: Fyne re-lights its
// stock widgets and every colour resolved through a theme colour name, but the
// canvas objects this app fills from state.pal() at build time, and the HTML
// the native reading panes hold, keep the palette they were built with until
// they are built again. A sheet is the same, only worse: rebuilding the page
// behind an open sheet leaves the sheet in the old palette over re-lit stock
// widgets — a dark card with dark-on-dark letters and light entry boxes — so
// the rebuild drains it (rebuildWindow) and the sheet comes back new in the
// new palette through the reopen seam (sheet_reopen.go).
//
// Not every change the settings listener hears is one the reader made. When
// iOS moves the app to the background it renders the app in BOTH appearances
// for the app switcher, so the variant flips away and flips back while the
// reader is not looking. Rebuilding for that round trip is what used to take
// the sheet the reader had left open: two rebuilds, each draining the overlay
// stack, on a change that nets to nothing. The first answer to that was to
// defer every theme rebuild until the sheet closed, which kept the sheet but
// left the whole app half in each theme for as long as the sheet stayed up —
// the sheet in the palette it was built with, the page behind it re-lit only
// where Fyne does it by itself.
//
// So on a phone or tablet the decision is gated on the foreground:
//
//   - A change heard while the app is NOT in the foreground is ignored at the
//     time. The gate closes at the very start of OnExitedForeground, before the
//     app can be snapshotted, and opens at OnEnteredForeground.
//   - Entering the foreground reconciles ONCE: the settled variant against the
//     variant the window was last built with. Equal — a snapshot round trip, or
//     nothing at all — does nothing, and the open sheet stays exactly as it
//     was. Different is a real change (made overnight, or from Control Centre,
//     which takes the app out of the foreground while it is open) and rebuilds.
//   - A change heard in the foreground is real by construction and rebuilds at
//     once, whatever is open.
//
// A light/dark rebuild takes the reopen closure of the sheet on top, rebuilds
// (which drains), then reopens it on the UI goroutine. Nothing is deferred.
//
// WHY THE ORDER THE CLOSURES RUN IN CANNOT FOOL IT. The variant itself moves in
// the driver's own order. On iOS the size event that carries the appearance and
// the lifecycle event that crosses the foreground travel on ONE event queue,
// handled on the UI goroutine, and the snapshot is taken between resigning and
// returning, so every variant update iOS makes while the app is away is applied
// before the entered-foreground hook runs. Android is the other way round: a
// change made while away usually reaches Go only with the redraw that brings
// the app back, and that redraw queues the lifecycle event (the foreground)
// BEFORE the size event carrying the new appearance — so there the hook runs
// first, against the old variant, and the update lands after it. (The
// configuration-change branch the Android night-mode patch adds may deliver it
// sooner; whether it does is a race between the Go loop and the Java side's
// write of the flag, so neither order can be assumed.) What travels out of
// order on both is the listener's closures: each settings change goes out on
// a channel (by a goroutine of its own when the channel is full), across this
// package's listener goroutine and back in through fyne.Do, whose queue the UI
// loop drains in no fixed order against the event queue. So a closure can run
// before or after the variant update that follows its own, and before or
// after the entered-foreground hook. It therefore carries nothing: it reads
// the LIVE variant when it runs and compares that with the variant the window
// was BUILT with. For the snapshot round trip, which is iOS's (built B;
// resign; variant A; variant B; return), every ordering of its two closures
// lands on one of three answers:
//
//   - a closure that runs while the app is out of the foreground is ignored,
//     whatever variant it reads;
//   - the reconcile runs after both updates (one queue), reads B, and B is
//     what the window was built with: nothing;
//   - a closure that runs after the reconcile reads B too (the round trip is
//     over), and B is still what the window was built with: nothing.
//
// That holds for either closure before the second update, both between the
// second update and the return, either or both after the return, and the two
// in either order among themselves; TestAppearanceRoundTripInEveryOrder walks
// every one of those orders through the gate and through the real window with
// a sheet open. It holds too when something else rebuilds the window between
// the two legs (a download landing with no sheet up): rebuildWindow records
// the variant it built with, A, so the return reads B against A and rebuilds
// once more, back into the palette the reader is actually in.
//
// A real change (built B; resign; variant A; return) has one answer in every
// order, the Android one included: whichever of the reconcile and a
// foreground closure reads A first rebuilds and records A as built, and
// everything after it reads A against A. On iOS that is the reconcile; on
// Android, where the return runs before the update, the reconcile reads B
// against B and does nothing, and the closure the update queues rebuilds —
// still exactly one rebuild and one reopen. The one ordering this depends on
// is the OS's — that the snapshot's second leg arrives before the app returns
// to the foreground, which is what "the app switcher's snapshot" means. Were
// a platform ever to deliver it after, the cost would be a second rebuild and
// a second reopen: the sheet reappears in the right palette each time, and
// never stays wrong.
//
// Desktop (macOS, Windows, Linux) has no snapshot round trip, and a window
// behind other windows is still on screen, so the gate never closes there: a
// change rebuilds when it is heard, focused or not. The rebuild-and-reopen is
// the same on every platform.

import (
	"fmt"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// appearanceEvent is one of the three things the gate hears.
type appearanceEvent int

const (
	// appearanceChanged is the settings listener's closure running on the UI
	// goroutine. It carries no variant: the gate reads the live one.
	appearanceChanged appearanceEvent = iota
	// appearanceExitedForeground is OnExitedForeground (the app resigning:
	// backgrounded, the app switcher, Control Centre, a system sheet).
	appearanceExitedForeground
	// appearanceEnteredForeground is OnEnteredForeground: the reconcile.
	appearanceEnteredForeground
)

func (e appearanceEvent) String() string {
	switch e {
	case appearanceChanged:
		return "variant-changed"
	case appearanceExitedForeground:
		return "exited-foreground"
	case appearanceEnteredForeground:
		return "entered-foreground"
	}
	return "unknown"
}

// appearanceGate is the whole of the decision's state. It lives on AppState
// (state.appearance) and is touched only on the UI goroutine: the listener's
// closures arrive through fyne.Do, and the mobile driver runs the lifecycle
// hooks from the same loop.
type appearanceGate struct {
	// mobile closes the gate on leaving the foreground. Only a phone or a
	// tablet snapshots the app for a switcher; set once, when the listener is
	// installed, from fyne.CurrentDevice().IsMobile() — the same question
	// InstallReadingStateFlush asks to tell a phone from a desktop window.
	mobile bool
	// background is true from the start of OnExitedForeground to the start of
	// OnEnteredForeground, and only ever on mobile.
	background bool
	// built is the variant the window was last built with. rebuildWindow
	// records it on every rebuild, whatever asked for it, because the palette
	// comes from the live variant at build time; decide records it too when it
	// orders a rebuild, so the decision stands on its own.
	built fyne.ThemeVariant
}

// appearanceAction is what the gate asks for: a window rebuild, and whether
// the sheet on top should come back after it.
type appearanceAction struct {
	rebuild bool
	reopen  bool
}

// decide is the pure decision: given what happened, the live variant and
// whether a sheet is open, rebuild or not, reopen or not. It changes only the
// gate. Everything that touches the window is observeAppearance's.
func (g *appearanceGate) decide(ev appearanceEvent, now fyne.ThemeVariant, sheetOpen bool) appearanceAction {
	switch ev {
	case appearanceExitedForeground:
		if g.mobile {
			g.background = true
		}
		return appearanceAction{}
	case appearanceEnteredForeground:
		if !g.mobile {
			return appearanceAction{} // nothing was held back, so nothing is owed
		}
		g.background = false
		// The reconcile: the settled variant against the built one, below.
	case appearanceChanged:
		if g.mobile && g.background {
			return appearanceAction{} // a snapshot leg, or a change the reconcile will find
		}
	}
	if now == g.built {
		return appearanceAction{}
	}
	g.built = now
	return appearanceAction{rebuild: true, reopen: sheetOpen}
}

// appearanceVariant is the system variant as the app sees it now — for the
// gate, and for the palette every window and sheet is built from (isDark,
// theme.go), so the two can never disagree. A seam because the test driver's
// settings answer a fixed "no preference", and both the gate and the colours of
// a reopened sheet have to be provable against a variant that moves.
var appearanceVariant = func(state *AppState) fyne.ThemeVariant {
	if state != nil && state.app != nil {
		return state.app.Settings().ThemeVariant()
	}
	if a := fyne.CurrentApp(); a != nil {
		return a.Settings().ThemeVariant()
	}
	return theme.VariantLight
}

// observeAppearance feeds one event to the gate and carries out its answer:
// take the reopen of the sheet on top, rebuild (which drains every overlay),
// and reopen that sheet in the new palette. UI goroutine only — the
// listener's fyne.Do and the lifecycle hooks.
func observeAppearance(state *AppState, ev appearanceEvent) {
	if state == nil || state.stopping.Load() {
		return
	}
	now := appearanceVariant(state)
	sheetOpen := state.window != nil && state.window.Canvas().Overlays().Top() != nil
	built := state.appearance.built
	act := state.appearance.decide(ev, now, sheetOpen)
	if os.Getenv("BT_SHEET_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[sheet] appearance %s: built=%v now=%v background=%v sheet=%v -> rebuild=%v reopen=%v\n",
			ev, built, now, state.appearance.background, sheetOpen, act.rebuild, act.reopen)
	}
	if !act.rebuild {
		return
	}
	// Taken BEFORE the rebuild: the drain closes the sheet, and a closed
	// sheet's registration is gone with it (sheet_reopen.go). The take runs
	// the sheet's capture, so it also comes before the unfocus below.
	var reopen func()
	if act.reopen {
		reopen = takeTopSheetReopen(state)
	}
	// Drop the focus BEFORE the drain. The drain takes each overlay off the
	// stack along with its focus manager, and nothing then tells the field
	// that held the caret it has lost it — and on a phone the soft keyboard
	// goes down only when the canvas is unfocused (Fyne's mobile canvas hides
	// it in OnUnfocus). Left alone, the number pad or keyboard stayed up over
	// the reopened sheet typing into nothing until the reader tapped a field
	// again. Only with a sheet up: that is the drain this change added; the
	// same rule openSearchResultRange applies before its own rebuild. A
	// reopen that had the caret puts it back (the Go to picker's capture).
	if sheetOpen {
		state.window.Canvas().Unfocus()
	}
	// rebuildWindow un-suppresses the native reading overlay after the drain,
	// and the reopened sheet suppresses it again a moment later. That pair is
	// kept on purpose: the chapter the rebuild re-renders lands in a SHOWN
	// overlay, as every other rebuild's does. The Android pane places its
	// note sticker only in a laid-out view (a hidden one skips it until the
	// next refresh), and every rebuild has run the same-chapter re-render
	// that keeps the reading position against a shown one — keeping the
	// overlay down across this rebuild would put that position on a path no
	// other rebuild takes.
	// Whether the Android Dialog paints a frame between the two is a device
	// question (docs/VISUAL_TESTS.md).
	rebuildWindow(state)
	if reopen != nil {
		reopen()
	}
}
