//go:build ios || android

package bibletext

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
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

// CreateMainUI (mobile) lays the app out as three full-screen tabs across the
// bottom: Read, Books, Search. Phones don't have room for the desktop's HSplit,
// and iOS users don't expect a persistent sidebar — tapping a book or a search
// hit selects it and switches to the Read tab automatically.
//
// Like the desktop layout, navigation swaps the Read tab's content rather than
// rebuilding the chrome, so the search field never loses focus mid-keystroke.
func CreateMainUI(app fyne.App, state *AppState, window fyne.Window) fyne.CanvasObject {
	state.app = app
	state.window = window
	registerAIState(state)
	if state.theme == nil {
		state.theme = &bibleTheme{fonts: loadBookFonts(), uiFonts: loadUIFonts()}
	}
	applyTheme(app, state)
	pal := state.pal()

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

	// Distraction-free reading mode: the entire window becomes the reading
	// pane plus a small exit affordance — no top header, no bottom tabs.
	// On iOS the native UITextView overlay therefore fills nearly the whole
	// screen. (Layout-agnostic: a tablet in full-screen reading looks the same
	// as a phone, so there is no sidebar to add here.)
	if state.IsFullScreen {
		// The state hooks MUST be rewired to THIS tree (implementation verification): with
		// narration playing, a chapter's natural end calls state.refresh() from
		// advanceAndContinue — if showReading still pointed at the previous
		// build's detached host, the fresh nativeReadingHost would be built into
		// that dead tree, steal the currentHost singleton, and leave the visible
		// overlay unframeable (pushFrame bails on currentHost != h) with a stale
		// corner label. Desktop installs its hooks before its full-screen branch
		// for the same reason.
		readingHost := container.NewStack(buildReadingViewMobile(state))
		state.showReading = func() {
			readingHost.Objects = []fyne.CanvasObject{buildReadingViewMobile(state)}
			readingHost.Refresh()
			notifyReadingOverlay(overlayShouldShow(state))
		}
		state.syncSidebar = func() {}
		base := canvas.NewRectangle(pal.Background)
		return container.NewStack(base, readingHost)
	}

	// Pick the layout from the live canvas width: a wide-enough tablet gets the
	// regular sidebar+split layout (ui_regular.go); everything else (phones, and
	// a tablet squeezed into a narrow multitasking column) gets the compact
	// bottom-tab layout below.
	var root fyne.CanvasObject
	if state.layoutClass() == layoutRegular {
		root = buildRegularWidthUI(state)
	} else {
		root = buildCompactUI(state)
	}

	// Wrap the root wherever the layout could change at runtime: iPads (static
	// idiom), and ALL of Android — there the tablet test reads live window
	// dimensions, which are 0×0 before the first layout, so the watcher must be
	// armed to catch the real size (it stays inert on phones: the class never
	// changes). iPhone paths remain unwrapped and byte-for-byte unchanged.
	if layoutMayChange() {
		return newLayoutWatcher(state, root)
	}
	return root
}

// compactReadingPane is the per-platform half of the shared compact layout: the
// search-results view when a search is active, otherwise this platform's reading
// view. iOS and Android have a native overlay pane; the desktop twin in
// ui_compact_desktop.go returns the ordinary Fyne/native desktop pane.
//
// This seam is the whole reason the compact layout could leave the mobile build
// tag. Everything else in it — the tab bar, the books grid, the search tab — is
// plain Fyne that never needed to be mobile-only, and keeping it there is what
// forced a second layout to exist for the desktop to have tabs at all.
func compactReadingPane(state *AppState) fyne.CanvasObject {
	if state.IsSearching {
		return buildSearchResultsView(state)
	}
	return buildReadingViewMobile(state)
}

// compactNavRail: A TABLET IN LANDSCAPE PUTS ITS NAVIGATION ON THE LEADING EDGE.
//
// The same reasoning as the desktop's (tab_rail.go): in landscape the scarce
// axis is vertical, and a bottom bar spends a full strip of it on three icons
// while the horizontal axis has room to spare. Rotate back to portrait and the
// bar returns, because there the trade runs the other way.
//
// Phones never do this at any orientation. A landscape phone has less height
// still, but it has no width to spare either — the rail would take it out of a
// reading measure that is already the tightest on any device.
//
// Orientation comes from the CANVAS, not from a layout pass: the soft keyboard
// shrinks the laid-out height and would otherwise read as a rotation. See the
// note on layoutWatcher.Resize, which is the same trap and cost 3,000 rebuilds
// a minute when it was got wrong.
func compactNavRail(state *AppState) bool {
	return deviceIsTablet() && state.canvasIsLandscape()
}
