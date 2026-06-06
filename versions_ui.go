package holybible

// Translation switcher UI: a quiet selector in the shared header (so it appears
// on both desktop and iOS) that opens a modal version picker. Switching swaps
// AppState.Bible and rebuilds the window (see switchVersion in versions.go).

import (
	"image/color"

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
			switchVersion(state, ver.ID)
		}))
	}

	note := widget.NewLabel("Licensed translations (NRSV, LSB) show clearly-marked placeholder text until a license is configured.")
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
		surface(container.NewPadded(content), pal.Surface, pal.Border, fyne.Size{}),
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

// versionRow is one tappable card in the picker: name + abbreviation, the
// publisher/license note, a TESTING tag for placeholder versions, and a check on
// the active one. A background rectangle gives it a solid hit rectangle (mobile)
// and a subtle highlight for the current version.
func versionRow(state *AppState, v BibleVersion, onTap func()) fyne.CanvasObject {
	pal := state.pal()
	current := v.ID == state.CurrentVersion

	name := canvas.NewText(v.Name+"  ("+v.Abbrev+")", pal.Text)
	name.TextStyle = fyne.TextStyle{Bold: true}
	name.TextSize = 15

	publisher := canvas.NewText(v.Publisher, pal.TextMuted)
	publisher.TextSize = 11

	lines := container.NewVBox(name, publisher)
	if v.isTesting() {
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
