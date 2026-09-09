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
// platform shell lives in ui_desktop.go and ui_mobile.go, selected by build tag;
// both compose the shared Read / Books / Search layout. `CreateMainUI` is
// defined in exactly one of them per build.

func buildHeader(state *AppState) fyne.CanvasObject {
	pal := state.pal()

	// The chrome should defer to the reading text — small serif title, muted
	// subtitle, no in-app theme toggle (light vs. dark follows the system
	// appearance via the variant Fyne hands bibleTheme.Color).
	title := canvas.NewText("BibleText", pal.Text)
	title.TextSize = 17
	title.TextStyle = fyne.TextStyle{Bold: true}

	// Dev builds running the platform mimic (BIBLETEXT_MIMIC, dev_mimic_on.go)
	// carry a loud badge beside the title, so a screenshot from this mode can
	// never masquerade as the real platform. devMimicLabel() is a constant ""
	// in release builds — this whole block is dead code there.
	titleRow := fyne.CanvasObject(title)
	if label := devMimicLabel(); label != "" {
		badge := canvas.NewText(label, pal.Accent)
		badge.TextSize = 13
		badge.TextStyle = fyne.TextStyle{Bold: true}
		titleRow = container.NewHBox(title, badge)
	}

	// The subtitle doubles as the translation switcher (WEB / BSB / NKJV), with a
	// TESTING badge when a version is showing placeholder text (see versions.go).
	// The title and the version line read as one two-line block, so they sit
	// tight against each other rather than a padding apart.
	left := container.New(layout.NewCustomPaddedVBoxLayout(0), titleRow, versionSelector(state))

	// Retained hook for the former regular iPad layout. The current classifier
	// never selects it; shared mobile navigation is built by buildCompactUI.
	if state.layoutClass() == layoutRegular {
		sidebarBtn := widget.NewButtonWithIcon("", iconSidebarLeft, func() {
			state.sidebarCollapsed = !state.sidebarCollapsed
			rebuildWindow(state)
		})
		sidebarBtn.Importance = widget.LowImportance
		// Vertically centre the toggle against the title + version stack.
		btnCol := container.NewVBox(layout.NewSpacer(), sidebarBtn, layout.NewSpacer())
		left = container.NewHBox(btnCol, left)
	}

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
	// SYMMETRIC top/bottom padding, and tight, because this band is chrome above
	// the reading text and should take as little of the screen as it can.
	//
	// It used to be 9 above and 2 below, deliberately: the argument was that the
	// title is large and bold while the version line is small and muted, so equal
	// numeric margins read as bottom-heavy and biasing the top evens them by eye.
	// That argument holds for the title column alone. It breaks once the "Go to"
	// chip is in the same band — a bounded shape with two crisp horizontal edges
	// makes the bias legible as an error rather than reading as balance, and
	// measured on a phone the chip sat 24.3pt below the band's top edge and only
	// 16.7pt above its bottom. Judged on screenshots, not in the abstract.
	//
	// The rule is still stacked with a ZERO inter-element gap: a plain
	// VBox(row, rule) inserts a full theme.Padding() (~7pt) between the row and
	// the rule, which pooled empty band under the version line.
	rowWrap := container.New(layout.NewCustomPaddedLayout(3, 3, theme.Padding(), theme.Padding()), row)
	content := container.New(layout.NewCustomPaddedVBoxLayout(0), rowWrap, rule)
	return container.NewStack(bg, content)
}

// incompleteBibleBanner is the status strip shown above the book lists while the app is
// still on the embedded Gospels seed (seedOnly — NOT merely a stale-epoch boot, which
// serves the reader's complete previous-epoch canon) — the full default-version Bible is downloading in the
// background (triggerFullDownload, which self-retries). It returns nil once the full text
// has landed (or the reader is on a different, complete version); the Books tab / sidebar
// rebuild after the swap drops it automatically.
func incompleteBibleBanner(state *AppState) fyne.CanvasObject {
	if state == nil || !state.seedOnly || state.CurrentVersion != defaultVersionID {
		return nil
	}
	pal := state.pal()
	icon := canvas.NewImageFromResource(theme.NewColoredResource(theme.DownloadIcon(), colorNameMuted))
	icon.FillMode = canvas.ImageFillContain
	icon.SetMinSize(fyne.NewSize(15, 15))
	msg := caption("Downloading the full Bible… showing the Gospels for now.")
	row := container.NewBorder(nil, nil, container.NewPadded(icon), nil, msg)
	return surface(container.NewPadded(row), pal.SurfaceAlt, pal.Border, fyne.Size{})
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
	// Assign AFTER stopLoadingBar — it nils loadingMsg too, and assigning first
	// meant the per-book download progress line never rendered on any build.
	state.loadingMsg = msg // loadProgressFn updates this with per-book download progress
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

	msg := widget.NewLabel("Something went wrong while loading the Bible. Check your connection and try again.")
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
		wrappedParagraph(msg, 300),
	)
	// A LINK TAPPED ON THIS SCREEN IS BEING HELD, AND THE READER CANNOT TELL.
	//
	// HandleShareLink parks for any phase other than loadReady, including this
	// one — and nothing consumes that park until a Retry SUCCEEDS. So a reader
	// who taps a shared verse while this view is up gets no acknowledgement of
	// any kind, and tapping again silently replaces what is held. Naming the
	// passage here is the whole fix: it turns "the app ignored me" into "the app
	// has it, and Retry is what opens it".
	//
	// Deliberately no promise of WHEN. The reader is the one who taps Retry, and
	// the load may fail again.
	if waiting := linkParkedMessage(state, false); waiting != "" {
		note := widget.NewLabel(waiting)
		note.Wrapping = fyne.TextWrapWord
		note.Alignment = fyne.TextAlignCenter
		col.Add(spacer(8))
		col.Add(wrappedParagraph(note, 300))
	}
	// Surface the actual cause (timeout, rate-limit, DNS, decode error) so the failure
	// is diagnosable rather than a generic guess.
	if state.loadErr != nil {
		detail := widget.NewLabel(state.loadErr.Error())
		detail.Wrapping = fyne.TextWrapWord
		detail.Alignment = fyne.TextAlignCenter
		detail.Importance = widget.LowImportance
		col.Add(spacer(6))
		col.Add(wrappedParagraph(detail, 300))
	}
	col.Add(spacer(14))
	col.Add(container.NewCenter(retry))

	base := canvas.NewRectangle(pal.Background)
	return container.NewStack(base, container.NewCenter(col))
}
