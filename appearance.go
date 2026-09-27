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
// A light/dark rebuild takes the reopen closure of the sheet on top, drops the
// caret, rebuilds (which drains, and brings the Windows title bar into the
// variant the content was built in), then reopens the sheet on the UI
// goroutine — or, with no sheet up, puts the caret back in the page field
// that had it. Nothing is deferred.
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
	"fyne.io/fyne/v2/widget"
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
	// frame is the variant the window's own title bar was last put in: seeded
	// with built when the listener is installed, since Fyne creates the native
	// window from the same system setting, and moved only by followTitleBar.
	// The frame follows the content, not the gate — see followTitleBar.
	frame fyne.ThemeVariant
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
// take the reopen of the sheet on top, rebuild (which drains every overlay
// and brings the window's own title bar along), and reopen that sheet in the
// new palette — or, with no sheet up, put the caret back in the page field
// that had it. UI goroutine only — the listener's fyne.Do and the lifecycle
// hooks.
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
	// The page's own field with the caret — Search, Find, the notes filter,
	// the Books filter — read before the unfocus below lets it go. Only a
	// canvas with no sheet on it can answer: with a sheet up the caret, if
	// any, is the sheet's, and the sheet's capture has already read it.
	caret, hadCaret := takePageFieldCaret(state)
	// On a phone the caret comes back only if its keyboard was up: focusing a
	// field there raises the keyboard, and the reader may have closed it while
	// the field kept the caret — Done on the Books filter, Back on Android —
	// so putting the caret back would raise a keyboard nobody asked for over
	// the rebuilt page. What was typed comes back either way. Read now, before
	// the unfocus below takes the keyboard down.
	refocus := pageCaretComesBack(state.appearance.mobile, softKeyboardShown)
	// Drop the focus BEFORE the rebuild, whatever is open. The drain takes
	// each overlay off the stack along with its focus manager, and SetContent
	// swaps the page's focus manager for one that cannot find the old field
	// in the new tree; in neither case is the field that held the caret told
	// it has lost it, and on a phone the soft keyboard goes down only when the
	// canvas is unfocused (Fyne's mobile canvas hides it in OnUnfocus). Left
	// alone, the keyboard stayed up typing into nothing — over the reopened
	// sheet, or over a rebuilt Search tab whose new field had no caret — and
	// on desktop the keystrokes were dropped without a sign. It was once done
	// only with a sheet up; the page's own fields lose the caret the same way.
	// A reopen that had the caret puts it back (the Go to picker's capture),
	// and so does the page field below.
	if state.window != nil {
		state.window.Canvas().Unfocus()
	}
	// rebuildWindow un-suppresses the native reading overlay after the drain,
	// and the reopened sheet suppresses it again a moment later. On iOS and
	// Android that pair is kept on purpose: the chapter the rebuild re-renders
	// lands in a SHOWN overlay, as every other rebuild's does there. The
	// Android pane places its note sticker only in a laid-out view (a hidden
	// one skips it until the next refresh), and every rebuild there has run
	// the same-chapter re-render that keeps the reading position against a
	// shown one — keeping the overlay down across this rebuild would put that
	// position on a path no other rebuild takes. macOS is the exception, and
	// deliberately: it refuses to show a pane holding the other palette's
	// chapter, so its light/dark rebuild on Read imports into the hidden pane
	// — the launch path — and keeps the reader's place because its capture
	// reads the clip view, which a hidden pane keeps
	// (setReadingOverlayVisible, bibleTextMacCaptureAnchor).
	//
	// The cost is known and left: on iOS and Android the pane that comes up
	// still holds the OLD palette's chapter until the rebuild's own push
	// lands — a frame or so of it over the sheet before the reopen suppresses
	// it again, or, after a change made on the Books or Search tab, over the
	// new chrome when the reader next goes to Read. Holding the pane down
	// until the push lands means an import into a hidden view — the path the
	// position restore and the Android sticker skip — or a native
	// pending-generation gate that, missing a landing once, leaves the pane
	// invisible; neither can be proved without a device (docs/BACKLOG.md).
	// macOS does not flash: its UI runs on the main thread, so a rebuild's
	// hide, its import and its show drain in one pass before anything is
	// drawn, and a pane holding another palette's chapter is not shown at all.
	// Whether the Android Dialog paints a frame between the two is a device
	// question (docs/VISUAL_TESTS.md).
	rebuildWindow(state) // → followTitleBar, with every other rebuild
	if reopen != nil {
		reopen()
	} else if hadCaret {
		caret.restore(state, refocus)
	}
}

// followTitleBar brings the window's own chrome into the variant the content
// was just built in, whenever that variant is not the one the chrome is in.
// rebuildWindow calls it after every rebuild, so it answers to the variant,
// not to the reason for the rebuild.
//
// Not from the appearance gate's decision, which can miss the change for
// good. The settings listener reaches the gate through a goroutine and
// fyne.Do, after Fyne has already moved the variant, so another rebuild can
// run in between — a tab tapped in the same moment, the startup load landing,
// a resize — and build the content in the new variant. rebuildWindow records
// that variant as built, the gate's closure then reads "no change" and
// rebuilds nothing, and a frame synced only on the gate's rebuild would keep
// the old mode until the next switch. A rebuild in the same variant sends
// nothing, so no rebuild but one that moves the variant reaches the native
// call.
func followTitleBar(state *AppState) {
	if state == nil || state.window == nil || state.appearance.built == state.appearance.frame {
		return
	}
	state.appearance.frame = state.appearance.built
	syncTitleBar(state.window, state.appearance.frame)
}

// syncTitleBar puts the window's own chrome into a variant. SetContent
// re-lights everything inside the window and nothing outside it, so the frame
// needs a word of its own (followTitleBar says when).
//
// Only Windows has anything to do (syncNativeTitleBar, title_bar_windows.go).
// Fyne sets the title bar's immersive dark mode ONCE, when it creates the
// window, from the registry as it stands then, and never again: its settings
// listener re-applies the theme to the content and does not touch the frame,
// and Windows does not flip an attribute an app has set. So a switch made
// while the app was open left a dark page under a white title bar, or a
// parchment page under a black one, until the app was relaunched. macOS
// re-lights its title bar with the system; Linux's belongs to the window
// manager; the phones have none. A seam, so the host can prove when it is
// called and with what.
var syncTitleBar = syncNativeTitleBar

// titleBarDarkMode is the value sent for DWMWA_USE_IMMERSIVE_DARK_MODE for a
// variant: a Win32 BOOL, TRUE only for dark. The same question the palette
// asks (isDark), so "no preference" is light in the frame as it is on the page.
func titleBarDarkMode(v fyne.ThemeVariant) int32 {
	if v == theme.VariantDark {
		return 1
	}
	return 0
}

// titleBarRepaint is the WM_NCACTIVATE pair that makes Windows 10 repaint a
// caption whose immersive dark mode has just been set (syncNativeTitleBar):
// the opposite of how the caption is drawn now, then how it is drawn — so the
// pair ends where it started, and only the mode has moved. Sent the other way
// round, an active window would be left with an inactive, grey caption, and an
// inactive one with a lit caption, until the next activation change.
func titleBarRepaint(drawnActive bool) [2]uintptr {
	if drawnActive {
		return [2]uintptr{0, 1}
	}
	return [2]uintptr{1, 0}
}

// pageField names a field on the page itself — not in a sheet — whose caret a
// light/dark rebuild puts back into the field's rebuilt twin.
type pageField int

const (
	pageFieldSearch pageField = iota + 1 // the Search tab's keyword field
	pageFieldFind                        // the Search tab's AI Find field
	pageFieldNotes                       // the Search tab's notes filter
	pageFieldBooks                       // the Books tab's filter
)

// registerPageField records a page field as the build that made it lays it
// out. buildCompactUI clears the set at the start of every build, so only
// fields of the tree now on the canvas are ever registered, and a rebuilt
// tab registers its own new fields over them.
func registerPageField(state *AppState, f pageField, o fyne.Focusable) {
	if state == nil || o == nil {
		return
	}
	if state.pageFields == nil {
		state.pageFields = map[pageField]fyne.Focusable{}
	}
	state.pageFields[f] = o
}

// pageFieldCaret is what a page field held when the rebuild took it: which
// field, its text, and where the caret stood in it.
type pageFieldCaret struct {
	field    pageField
	text     string
	row, col int
}

// takePageFieldCaret reads the caret of the page field that has it, if any.
// The text is read too: the rebuilt twin is filled from state, and a field
// can be ahead of state — the Find field is written to state only when the
// question is submitted, and the keyword field only when its debounce fires.
func takePageFieldCaret(state *AppState) (pageFieldCaret, bool) {
	if state == nil || state.window == nil || len(state.pageFields) == 0 {
		return pageFieldCaret{}, false
	}
	focused := state.window.Canvas().Focused()
	if focused == nil {
		return pageFieldCaret{}, false
	}
	for f, o := range state.pageFields {
		if o != focused {
			continue
		}
		c := pageFieldCaret{field: f}
		if e := entryOf(o); e != nil {
			c.text, c.row, c.col = e.Text, e.CursorRow, e.CursorColumn
		}
		return c, true
	}
	return pageFieldCaret{}, false
}

// softKeyboardShown is whether the soft keyboard was on screen at its last
// report — iOS's keyboard-frame observer (bibleTextKeyboardChanged) and
// Android's IME insets (btaKeyboardChanged), both on the UI goroutine. False
// until a report comes, and on desktop, which has no soft keyboard, and on
// Android before API 30, which reports nothing: there a caret is simply not
// put back (pageCaretComesBack), and the reader taps the field again.
var softKeyboardShown bool

// noteSoftKeyboard records the soft keyboard's latest on-screen overlap.
func noteSoftKeyboard(overlap float32) { softKeyboardShown = overlap > 0 }

// pageCaretComesBack is whether a page field that had the caret before a
// light/dark rebuild gets it back in its rebuilt twin: always on desktop,
// where focus raises nothing, and on a phone only when the soft keyboard was
// up — Fyne's mobile canvas shows the keyboard for every Focus, and Fyne still
// counts a field focused after the reader has put its keyboard away.
func pageCaretComesBack(mobile, keyboardShown bool) bool {
	return !mobile || keyboardShown
}

// restore gives the rebuilt twin of the field the caret was taken from the
// text the reader had typed and the caret's place in it, and, when focus is
// true (pageCaretComesBack), the caret itself. A twin the rebuild did not lay
// out — the reader's tab or mode is not the one it was — gets nothing.
func (c pageFieldCaret) restore(state *AppState, focus bool) {
	if state == nil || state.window == nil {
		return
	}
	o, ok := state.pageFields[c.field]
	if !ok || o == nil {
		return
	}
	if e := entryOf(o); e != nil {
		if e.Text != c.text {
			e.SetText(c.text) // its OnChanged runs as it would for the keystrokes
		}
		e.CursorRow, e.CursorColumn = c.row, c.col
		e.Refresh()
	}
	if focus {
		state.window.Canvas().Focus(o)
	}
}

// entryOf is the widget.Entry inside a page field, whichever entry type
// carries it.
func entryOf(o fyne.Focusable) *widget.Entry {
	switch e := o.(type) {
	case *widget.Entry:
		return e
	case *searchKeyEntry:
		return &e.Entry
	}
	return nil
}
