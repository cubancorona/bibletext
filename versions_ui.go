package bibletext

// Translation switcher UI: a quiet selector in the shared header (so it appears
// on both desktop and iOS) that opens a modal version picker. Switching swaps
// AppState.Bible and rebuilds the window (see switchVersion in versions.go).

import (
	"fmt"
	"image/color"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// versionSelector is the header subtitle: the active translation name + a caret,
// with a TESTING badge when it is serving placeholder text. Tapping opens the
// picker.
func versionSelector(state *AppState) fyne.CanvasObject {
	pal := state.pal()
	v := state.currentVersion()

	anchor := newVersionPickerAnchor(state, v.Name+"  ▾", pal.TextMuted, 10)
	if state.currentMode != modeTesting {
		return anchor
	}

	badge := canvas.NewText("TESTING", pal.Accent)
	badge.TextSize = 9
	badge.TextStyle = fyne.TextStyle{Bold: true}
	return container.NewHBox(anchor, hgap(6), container.NewCenter(badge))
}

// versionPickerAnchor is a small tappable bit of muted text (the header
// subtitle) that opens the version picker. Like chapterPickerAnchor it pins the
// text inside a fixed-size GridWrap cell so it has a solid hit rectangle — a
// bare canvas.Text renderer is not reliably matched by Fyne's mobile hit-test.
type versionPickerAnchor struct {
	widget.BaseWidget
	state *AppState
	text  string
	tint  color.NRGBA
	size  float32
}

func newVersionPickerAnchor(state *AppState, text string, tint color.NRGBA, size float32) *versionPickerAnchor {
	a := &versionPickerAnchor{state: state, text: text, tint: tint, size: size}
	a.ExtendBaseWidget(a)
	return a
}

func (a *versionPickerAnchor) CreateRenderer() fyne.WidgetRenderer {
	lbl := canvas.NewText(a.text, a.tint)
	lbl.TextSize = a.size
	w := fyne.MeasureText(a.text, a.size, lbl.TextStyle).Width
	box := container.NewGridWrap(fyne.NewSize(w, a.size+12), container.NewCenter(lbl))
	return widget.NewSimpleRenderer(box)
}

func (a *versionPickerAnchor) Tapped(*fyne.PointEvent) { showVersionPicker(a.state) }

var _ fyne.Tappable = (*versionPickerAnchor)(nil)

// showVersionPicker presents the list of translations. It hides the native
// reading overlay while open (same as the chapter picker / AI panels).
func showVersionPicker(state *AppState) {
	if state == nil || state.window == nil {
		return
	}
	cnv := state.window.Canvas()
	if cnv == nil {
		return
	}
	pal := state.pal()

	if state.hideReadingOverlay != nil {
		state.hideReadingOverlay()
	}
	restore := func() {
		if state.showReadingOverlay != nil {
			state.showReadingOverlay()
		}
	}

	var popup *widget.PopUp
	closePicker := func() {
		if popup != nil {
			popup.Hide()
		}
		restore()
	}

	title := canvas.NewText("Translation", pal.Text)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 18
	intro := widget.NewLabel("Choose a Bible version.")
	intro.Wrapping = fyne.TextWrapWord
	header := container.NewVBox(title, intro, widget.NewSeparator())

	rows := container.NewVBox()
	for _, v := range bibleVersions() {
		ver := v // capture
		rows.Add(versionRow(state, ver, func() {
			closePicker()
			switchVersionInteractive(state, ver.ID)
		}))
	}

	note := widget.NewLabel("NRSV and LSB are under evaluation and not yet selectable; they unlock once licensing is complete.")
	note.Wrapping = fyne.TextWrapWord
	closeBtn := widget.NewButton("Close", closePicker)
	footer := container.NewVBox(
		widget.NewSeparator(),
		note,
		container.NewBorder(nil, nil, nil, closeBtn),
	)

	body := container.NewVScroll(container.NewPadded(rows))
	content := container.NewBorder(header, footer, nil, nil, body)

	popup = widget.NewModalPopUp(
		surface(container.NewPadded(content), pal.SurfaceAlt, pal.Border, fyne.Size{}),
		cnv,
	)
	popup.Show()

	// Size to content, capped to the screen.
	cs := cnv.Size()
	w := cs.Width - 48
	if w > 460 {
		w = 460
	}
	if w < 280 {
		w = 280
	}
	h := header.MinSize().Height + rows.MinSize().Height + footer.MinSize().Height + 64
	if maxH := cs.Height - 80; h > maxH {
		h = maxH
	}
	popup.Resize(fyne.NewSize(w, h))
}

// versionRow is one card in the picker: name + abbreviation, the publisher/license
// note, a status line, and a check on the active one. A background rectangle gives
// it a solid hit rectangle (mobile) and a subtle highlight for the current version.
//
// Selectable versions (real text available) are tappable. A version that isn't yet
// licensed is rendered de-emphasized and NON-tappable with a formal "evaluation in
// progress" note — it is never wrapped in a tapBox, so users cannot reach its
// placeholder text. (When BIBLETEXT_ENABLE_TESTING=1, such a version becomes
// selectable and instead carries the internal TESTING placeholder tag.)
func versionRow(state *AppState, v BibleVersion, onTap func()) fyne.CanvasObject {
	pal := state.pal()
	selectable := v.canSelect()
	current := v.ID == state.CurrentVersion

	nameColor := pal.Text
	if !selectable {
		nameColor = pal.TextMuted // greyed: present but not available
	}
	name := canvas.NewText(v.Name+"  ("+v.Abbrev+")", nameColor)
	name.TextStyle = fyne.TextStyle{Bold: true}
	name.TextSize = 15

	publisher := canvas.NewText(v.Publisher, pal.TextMuted)
	publisher.TextSize = 11

	lines := container.NewVBox(name, publisher)
	switch {
	case !selectable:
		// Copyrighted translation we don't yet have rights to ship: shown, but not
		// selectable, with a formal status note instead of any placeholder text.
		tag := canvas.NewText("Evaluation in progress — not yet available", pal.TextMuted)
		tag.TextSize = 11
		tag.TextStyle = fyne.TextStyle{Italic: true}
		lines.Add(tag)
	case v.isTesting():
		// Selectable only because internal testing mode is on (BIBLETEXT_ENABLE_TESTING).
		tag := canvas.NewText("TESTING — placeholder text, not the real translation", pal.Accent)
		tag.TextSize = 11
		tag.TextStyle = fyne.TextStyle{Italic: true}
		lines.Add(tag)
	}

	var right fyne.CanvasObject = layout.NewSpacer()
	if current {
		check := canvas.NewText("✓", pal.Accent)
		check.TextStyle = fyne.TextStyle{Bold: true}
		check.TextSize = 18
		right = container.NewCenter(check)
	}

	inner := container.NewBorder(nil, nil, nil, right, lines)

	bg := canvas.NewRectangle(color.Transparent)
	if current {
		bg.FillColor = pal.SurfaceAlt
	}
	bg.CornerRadius = 8

	card := container.NewStack(bg, container.NewPadded(inner))
	if !selectable {
		// No tapBox wrapper → genuinely inert; the row is informational only.
		return card
	}
	return newTapBox(card, onTap)
}

// tapBox wraps arbitrary content in a tappable widget with a solid hit area.
type tapBox struct {
	widget.BaseWidget
	content fyne.CanvasObject
	onTap   func()
}

func newTapBox(content fyne.CanvasObject, onTap func()) *tapBox {
	b := &tapBox{content: content, onTap: onTap}
	b.ExtendBaseWidget(b)
	return b
}

func (b *tapBox) Tapped(*fyne.PointEvent) {
	if b.onTap != nil {
		b.onTap()
	}
}

func (b *tapBox) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(b.content)
}

var _ fyne.Tappable = (*tapBox)(nil)

// switchVersionInteractive is the version picker's entry point. It keeps the UI
// thread free when a translation isn't loaded yet: versions already in memory (or
// instant testing placeholders) swap synchronously, but a first-time real fetch —
// notably the Berean Standard Bible's one-time ~7 MB download — runs on a
// goroutine behind a small loading modal. This matters most on iOS, where a
// multi-second network call on the main thread would freeze the UI and risk the
// launch/run-loop watchdog. On failure the current version is kept and a brief
// message is shown; the synchronous switchVersion remains the shared core (and is
// what unit tests drive directly).
func switchVersionInteractive(state *AppState, id string) {
	if state == nil || id == state.CurrentVersion {
		return
	}
	v, ok := versionByID(id)
	if !ok || !v.canSelect() {
		return
	}
	// Loaded earlier this session, or an instant base-derived placeholder → swap
	// synchronously; neither touches the network.
	if _, inMem := state.loadedVersions[id]; inMem || v.isTesting() {
		switchVersion(state, id)
		return
	}

	// A real source not yet in memory may hit the on-disk cache (fast) or the
	// network (slow). Load off the UI thread either way, behind a spinner.
	base := state.baseBible()
	dismiss := showVersionLoading(state, v.Name)
	go func() {
		data, mode, err := loadVersionData(v, base)
		fyne.Do(func() {
			// If the app began tearing down while the download ran, the desktop
			// (glfw) driver runs this inline on THIS goroutine after the main loop
			// drained — drop the result rather than mutate state / write Preferences
			// off the main thread during exit. (Mobile always enqueues, so this is a
			// no-op there.)
			if state.stopping.Load() {
				return
			}
			dismiss()
			if err != nil {
				fmt.Fprintf(os.Stderr, "BibleText: could not load %s: %v\n", v.Name, err)
				showVersionLoadError(state, v.Name)
				return
			}
			applyLoadedVersion(state, v, data, mode)
		})
	}()
}

// showVersionLoading puts up a modal spinner while a translation downloads, and
// returns a dismiss func. It hides the native reading overlay for the duration
// (the overlay floats above the Fyne canvas); on success the follow-up
// rebuildWindow re-pins it, and the dismiss func restores it on the error path so
// the reader is never left blank.
func showVersionLoading(state *AppState, name string) func() {
	if state == nil || state.window == nil {
		return func() {}
	}
	cnv := state.window.Canvas()
	if cnv == nil {
		return func() {}
	}
	pal := state.pal()
	if state.hideReadingOverlay != nil {
		state.hideReadingOverlay()
	}

	title := canvas.NewText("Downloading "+name+"…", pal.Text)
	title.Alignment = fyne.TextAlignCenter
	title.TextStyle = fyne.TextStyle{Bold: true}
	sub := canvas.NewText("One-time download — it's cached after this.", pal.TextMuted)
	sub.TextSize = 11
	sub.Alignment = fyne.TextAlignCenter
	bar := widget.NewProgressBarInfinite()

	content := container.NewVBox(
		title, spacer(10),
		container.NewGridWrap(fyne.NewSize(240, bar.MinSize().Height), bar),
		spacer(8), sub,
	)
	popup := widget.NewModalPopUp(
		surface(container.NewPadded(content), pal.SurfaceAlt, pal.Border, fyne.Size{}),
		cnv,
	)
	popup.Show()

	dismissed := false
	return func() {
		if dismissed {
			return
		}
		dismissed = true
		bar.Stop()
		popup.Hide()
		if state.showReadingOverlay != nil {
			state.showReadingOverlay()
		}
	}
}

// showVersionLoadError reports a failed translation download (e.g. offline) and
// leaves the current version untouched.
func showVersionLoadError(state *AppState, name string) {
	if state == nil || state.window == nil {
		return
	}
	cnv := state.window.Canvas()
	if cnv == nil {
		return
	}
	pal := state.pal()

	// Hide the native reading overlay while the dialog is up — it floats above the
	// Fyne canvas and would otherwise paint over the message and OK button (the
	// same hide/restore dance every other modal does). Restore it on dismiss.
	if state.hideReadingOverlay != nil {
		state.hideReadingOverlay()
	}

	msg := widget.NewLabel("Couldn't download " + name + ".\nCheck your connection and try again.")
	msg.Wrapping = fyne.TextWrapWord
	msg.Alignment = fyne.TextAlignCenter

	var popup *widget.PopUp
	okBtn := widget.NewButton("OK", func() {
		if popup != nil {
			popup.Hide()
		}
		if state.showReadingOverlay != nil {
			state.showReadingOverlay()
		}
	})
	content := container.NewVBox(msg, container.NewCenter(okBtn))
	popup = widget.NewModalPopUp(
		surface(container.NewPadded(content), pal.SurfaceAlt, pal.Border, fyne.Size{}),
		cnv,
	)
	popup.Show()
}
