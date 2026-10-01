//go:build ios || android

package bibletext

import (
	"fyne.io/fyne/v2"
)

// dismissKeyboard drops focus from whatever field owns the soft keyboard, which makes
// Fyne's mobile driver hide it (canvas OnUnfocus → hideVirtualKeyboard). Used after a
// search/ask is submitted from the keyboard so the results get the full pane instead of
// sitting behind the keyboard.
func dismissKeyboard(state *AppState) {
	if state != nil && state.window != nil {
		state.window.Canvas().Unfocus()
	}
}

// CreateMainUI (mobile) uses the shared Read / Books / Search layout. Navigation
// is a leading rail while the window is wider than it is tall and a bottom bar
// otherwise (mobileRailWanted). Tapping a book or search hit selects it and
// returns to Read automatically.
//
// Switching tabs rebuilds the window (ui_compact.go); within the Read tab,
// chapter navigation swaps the reading pane's content in place, so the chrome
// around it stays put.
func CreateMainUI(app fyne.App, state *AppState, window fyne.Window) fyne.CanvasObject {
	state.app = app
	state.window = window
	state.header = nil // until buildHeader makes one for this tree
	registerAIState(state)
	if state.theme == nil {
		state.theme = &bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()}
	}
	applyTheme(app, state)

	// Startup: the Bible loads on a background goroutine, so until it's ready we
	// render only the loading/error screen and keep the native UITextView overlay
	// detached (there's no chapter to show yet, and pinning it over a tree with no
	// reading view is exactly the black-rectangle hazard).
	switch state.loadPhase {
	case loadPending:
		notifyReadingOverlay(false)
		return buildLoadingView(state)
	case loadFailed:
		notifyReadingOverlay(false)
		return buildLoadErrorView(state)
	}

	// Distraction-free reading — the reader's own choice, or the phone-landscape
	// presentation on the Read tab (readingFullScreen, phone_landscape.go) — is
	// the shared layout's tree (buildCompactUI): the reading pane alone, no
	// top header, no bottom tabs, so on iOS the native UITextView overlay fills
	// nearly the whole screen. The shared branch rewires the state hooks to
	// the newly built tree, for the reason the Read case documents (a stale
	// showReading builds the pane into a dead tree).
	//
	// classifyLayout currently always selects the shared layout. Keep the
	// former regular branch explicit so restoring it would require a
	// deliberate change at the classifier rather than resurrecting hidden
	// platform logic.
	var root fyne.CanvasObject
	if state.readingFullScreen() || state.layoutClass() != layoutRegular {
		root = buildCompactUI(state)
	} else {
		root = buildRegularWidthUI(state)
	}

	// Every phone and tablet is watched: a rotation moves the navigation
	// between bottom bar and rail (mobileRailWanted), whatever the landscape
	// presentation's preference, and on a phone flips that presentation; on
	// Android the live dimensions also arrive only after the first build.
	// The watcher wraps the full-screen tree too: an iPad in chosen
	// full-screen still never rebuilds on rotation, because renderedLayout
	// zeroes its rail term while full-screen and its landscape term is
	// constant off phones — so its reading position is untouched.
	return newLayoutWatcher(state, root)
}

// compactReadingView is the per-platform half of the shared compact layout:
// this platform's reading view, which compactReadingPane (ui_compact.go)
// replaces with the search results while a search is active. iOS and Android
// have a native overlay pane; the desktop twin in ui_compact_desktop.go returns
// the ordinary Fyne/native desktop pane.
//
// This seam is the whole reason the compact layout could leave the mobile build
// tag. Everything else in it — the tab bar, the books grid, the search tab — is
// plain Fyne that never needed to be mobile-only, and keeping it there is what
// forced a second layout to exist for the desktop to have tabs at all.
func compactReadingView(state *AppState) fyne.CanvasObject {
	return buildReadingViewMobile(state)
}

// compactNavRail is the phone's and the tablet's navigation placement: the
// leading rail when the window is wider than it is tall, the bottom bar
// otherwise (mobileRailWanted, read from the live canvas by railForWindow).
// Only the edge the destinations sit on depends on it; they and the state
// behind them are the same on either.
func compactNavRail(state *AppState) bool { return railForWindow(state) }
