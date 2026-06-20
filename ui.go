package bibletext

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Shared UI helpers used by both the desktop and mobile entry points. The
// platform-specific layout (HSplit + keyboard shortcuts vs. bottom tabs + drawer
// with touch-sized rows) lives in ui_desktop.go and ui_mobile.go, selected by
// build tag — `CreateMainUI` is defined in exactly one of them per build.

func buildHeader(state *AppState) fyne.CanvasObject {
	pal := state.pal()

	// The chrome should defer to the reading text — small serif title, muted
	// subtitle, no in-app theme toggle (light vs. dark follows the system
	// appearance via the variant Fyne hands bibleTheme.Color).
	title := canvas.NewText("BibleText", pal.Text)
	title.TextSize = 17
	title.TextStyle = fyne.TextStyle{Bold: true}

	// The subtitle doubles as the translation switcher (WEB / NRSV / LSB), with a
	// TESTING badge when a version is showing placeholder text (see versions.go).
	left := container.NewVBox(title, versionSelector(state))

	// Settings gear (AI study: pick a provider + paste your key) sits beside the
	// subtle verse-of-the-day sparkle. Both are low-importance so the chrome stays
	// quiet next to the reading text.
	gear := widget.NewButtonWithIcon("", theme.SettingsIcon(), func() { showAISettings(state) })
	gear.Importance = widget.LowImportance
	controls := container.NewHBox(verseOfDayButton(state), gear)
	right := container.NewVBox(layout.NewSpacer(), controls, layout.NewSpacer())

	// A single centered "Go to" button opens the citation popup (showGotoPopup). It
	// sits in the Border's center slot — shorter than the title+subtitle column, so
	// it never grows the header — instead of an inline row that reserved layout space.
	center := container.NewVBox(layout.NewSpacer(), gotoButton(state), layout.NewSpacer())
	row := container.NewBorder(nil, nil, left, right, center)

	rule := canvas.NewLine(pal.Border)
	rule.StrokeWidth = 1

	bg := canvas.NewRectangle(pal.SurfaceAlt)
	// Tight top/bottom padding (vs the theme's full inset) keeps the app header
	// compact so more of the screen is reading text.
	content := container.NewVBox(container.New(layout.NewCustomPaddedLayout(2, 2, theme.Padding(), theme.Padding()), row), rule)
	return container.NewStack(bg, content)
}

// buildLoadingView is the startup screen shown while the Bible loads on a
// background goroutine (state.loadPhase == loadPending). It is pure Fyne — no
// native reading overlay — so it renders identically on every platform and never
// competes with the iOS UITextView (which CreateMainUI keeps detached while
// loading). Kept deliberately calm: the app title and a quiet indeterminate bar.
func buildLoadingView(state *AppState) fyne.CanvasObject {
	pal := state.pal()

	title := canvas.NewText("BibleText", pal.Text)
	title.TextSize = 22
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.Alignment = fyne.TextAlignCenter

	msg := canvas.NewText("Preparing the Bible…", pal.TextMuted)
	msg.TextSize = 13
	msg.Alignment = fyne.TextAlignCenter

	// Stop any previous spinner before replacing it. A ProgressBarInfinite runs a
	// RepeatForever animation that calls canvas.Refresh every ~50ms; if the loading
	// view is rebuilt while still loading (the system light/dark watcher is the one
	// rebuild that can fire during the 5-10s background load), overwriting loadingBar
	// without stopping the old bar ORPHANS its animation — it keeps repainting the
	// whole canvas at ~20fps off-screen, pinning the GPU/main thread and making even
	// short text scroll laggy until GC reclaims it (a force-quit is what cleared it).
	// Stopping first guarantees at most one live loading animation.
	state.stopLoadingBar()
	bar := widget.NewProgressBarInfinite()
	state.loadingBar = bar // so stopLoadingBar can halt it once loading finishes

	// A fixed-width column keeps the bar from stretching edge-to-edge on a wide
	// desktop window while still centering on a phone.
	col := container.NewVBox(
		container.NewCenter(title),
		spacer(6),
		container.NewCenter(msg),
		spacer(14),
		container.NewGridWrap(fyne.NewSize(220, bar.MinSize().Height), bar),
	)

	base := canvas.NewRectangle(pal.Background)
	return container.NewStack(base, container.NewCenter(col))
}

// buildLoadErrorView is shown when the first-ever load fails with no cache to
// fall back on (offline first run). It explains the problem and offers Retry,
// which restarts the background load. Replaces the old fatal os.Exit path.
func buildLoadErrorView(state *AppState) fyne.CanvasObject {
	pal := state.pal()

	title := canvas.NewText("Couldn’t load the Bible", pal.Text)
	title.TextSize = 18
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.Alignment = fyne.TextAlignCenter

	msg := widget.NewLabel("The first run needs an internet connection to download the text. Check your connection and try again.")
	msg.Wrapping = fyne.TextWrapWord
	msg.Alignment = fyne.TextAlignCenter

	retry := widget.NewButton("Retry", func() {
		state.loadPhase = loadPending
		state.loadErr = nil
		rebuildWindow(state)
		StartBackgroundLoad(state.app, state.window, state)
	})
	retry.Importance = widget.HighImportance

	col := container.NewVBox(
		container.NewCenter(title),
		spacer(8),
		container.NewGridWrap(fyne.NewSize(300, msg.MinSize().Height), msg),
		spacer(14),
		container.NewCenter(retry),
	)

	base := canvas.NewRectangle(pal.Background)
	return container.NewStack(base, container.NewCenter(col))
}
