//go:build bibletextdev

package bibletext

// A live, development-only highlight colour lab. It deliberately keeps four
// independent washes even though the shipping palettes currently have one
// Highlight per appearance: the point of the lab is to compare ordinary and
// red-letter scripture on day and night grounds, then capture the preferred
// four RGB values together for a later production change.

import (
	"fmt"
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

const devHighlightCaseCount = 4

type devHighlightCase struct {
	label        string
	summaryLabel string
	pal          palette
	red          bool
}

var devHighlightCases = [devHighlightCaseCount]devHighlightCase{
	{label: "Day / regular text", summaryLabel: "Day / regular", pal: lightPalette},
	{label: "Day / red text", summaryLabel: "Day / red", pal: lightPalette, red: true},
	{label: "Night / regular text", summaryLabel: "Night / regular", pal: darkPalette},
	{label: "Night / red text", summaryLabel: "Night / red", pal: darkPalette, red: true},
}

// The dev tab is rebuilt whenever the reader leaves it. Keep the choices for
// this process, just as devLinksScrollY keeps the tab's place in the scenario
// list. They are intentionally not preferences: this is a visual workbench,
// not a hidden way to change the shipping reading palette.
var (
	devHighlightColors = [devHighlightCaseCount]color.NRGBA{
		lightPalette.Highlight,
		lightPalette.Highlight,
		darkPalette.Highlight,
		darkPalette.Highlight,
	}
	devHighlightActive int
)

// devHighlightPreview is the small scripture specimen inside one case. The
// fields stay explicit so the tagged tests can pin the important contract:
// book font, reading colours, and text both inside and outside the wash.
type devHighlightPreview struct {
	root fyne.CanvasObject

	ground   *canvas.Rectangle
	wash     *canvas.Rectangle
	number   *canvas.Text
	prefix   *canvas.Text
	selected *canvas.Text
	suffix   *canvas.Text
}

func newDevHighlightPreview(c devHighlightCase, wash color.NRGBA) *devHighlightPreview {
	font := styledPaneFont()
	textSize := devHighlightPreviewTextSize()
	textColor := c.pal.Text
	if c.red {
		textColor = c.pal.RedLetter
	}

	text := func(value string, tint color.NRGBA, size float32) *canvas.Text {
		t := canvas.NewText(value, tint)
		t.FontSource = font
		t.TextSize = size
		return t
	}

	p := &devHighlightPreview{
		ground:   canvas.NewRectangle(c.pal.Background),
		wash:     canvas.NewRectangle(wash),
		number:   text("25", c.pal.VerseNumber, textSize*styledNumRatio),
		prefix:   text("I am", textColor, textSize),
		selected: text("the life.", textColor, textSize),
		suffix:   text("Live.", textColor, textSize),
	}
	p.ground.StrokeColor = c.pal.Border
	p.ground.StrokeWidth = 1
	p.ground.CornerRadius = 6
	p.ground.SetMinSize(fyne.NewSize(0, 44))
	p.wash.CornerRadius = 2

	// Match the native reading rule: two horizontal pixels of wash beyond the
	// selected glyphs, a 2px radius, no bolding, and no foreground override.
	selection := container.NewStack(
		p.wash,
		container.New(layout.NewCustomPaddedLayout(0, 0, 2, 2), p.selected),
	)
	line := container.NewHBox(p.number, p.prefix, selection, p.suffix)
	p.root = container.NewStack(p.ground, container.NewPadded(line))
	return p
}

// The native Apple reading pane uses a 21px body at the active reading scale.
// The Dev tab itself is Fyne, but its specimen should judge the wash against
// the same glyph size the iOS/macOS reading view imports, not the smaller UI
// theme size. The deliberately short phrase still fits a phone at Extra large.
func devHighlightPreviewTextSize() float32 {
	return 21 * float32(readingTextScale())
}

func (p *devHighlightPreview) setWash(c color.NRGBA) {
	p.wash.FillColor = c
	p.wash.Refresh()
}

// devHighlightLab keeps the live canvas references together. Slider callbacks
// mutate these objects in place; rebuilding the page would reset the controls,
// lose the scroll position, and make a live comparison needlessly jumpy.
type devHighlightLab struct {
	root fyne.CanvasObject

	previews [devHighlightCaseCount]*devHighlightPreview
	codes    [devHighlightCaseCount]*canvas.Text
	buttons  [devHighlightCaseCount]*widget.Button

	sliders      [3]*widget.Slider
	channelValue [3]*widget.Label
	editorTitle  *canvas.Text
	editorCode   *canvas.Text
	summary      *widget.Label
	editor       *fyne.Container
	editorHost   *fyne.Container
	editorSample *devHighlightPreview
	syncing      bool
}

func buildDevHighlightLab(state *AppState) fyne.CanvasObject {
	return newDevHighlightLab(state).root
}

func newDevHighlightLab(state *AppState) *devHighlightLab {
	chrome := state.pal()
	lab := &devHighlightLab{}

	title := canvas.NewText("Highlight colour lab", chrome.Text)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 20

	intro := widget.NewLabel("Tap Adjust, then drag the RGB sliders; the specimen and codes update immediately. " +
		"Hide the controls for a screenshot. These visual-only choices last for this dev run.")
	intro.Wrapping = fyne.TextWrapWord

	caseColumn := container.NewVBox()
	for i, c := range devHighlightCases {
		i, c := i, c
		lab.previews[i] = newDevHighlightPreview(c, devHighlightColors[i])

		name := canvas.NewText(c.label, chrome.Text)
		name.TextStyle = fyne.TextStyle{Bold: true}
		name.TextSize = 14
		lab.codes[i] = canvas.NewText(devHighlightHex(devHighlightColors[i]), chrome.TextMuted)
		lab.codes[i].TextSize = 13

		lab.buttons[i] = widget.NewButton("Adjust", func() {
			lab.selectCase(i)
			lab.editor.Show()
		})
		heading := container.NewBorder(nil, nil,
			container.NewHBox(name, lab.codes[i]), lab.buttons[i], nil)
		caseColumn.Add(container.NewVBox(heading, lab.previews[i].root))
	}

	lab.editorTitle = canvas.NewText("", chrome.Text)
	lab.editorTitle.TextStyle = fyne.TextStyle{Bold: true}
	lab.editorTitle.TextSize = 16
	lab.editorCode = canvas.NewText("", chrome.TextMuted)
	lab.editorCode.TextSize = 14
	lab.editorHost = container.NewStack()

	channelNames := [3]string{"Red", "Green", "Blue"}
	channelRows := make([]fyne.CanvasObject, 0, len(channelNames))
	for channel, name := range channelNames {
		channel := channel
		slider := widget.NewSlider(0, 255)
		slider.Step = 1
		slider.OnChanged = func(value float64) {
			lab.setChannel(channel, uint8(value+0.5))
		}
		lab.sliders[channel] = slider
		lab.channelValue[channel] = widget.NewLabel("000")

		left := container.NewGridWrap(fyne.NewSize(52, slider.MinSize().Height), widget.NewLabel(name))
		right := container.NewGridWrap(fyne.NewSize(40, slider.MinSize().Height), lab.channelValue[channel])
		channelRows = append(channelRows, container.NewBorder(nil, nil, left, right, slider))
	}

	reset := widget.NewButton("Reset this sample", func() {
		index := devHighlightActive
		lab.setColor(index, devHighlightCases[index].pal.Highlight)
		lab.selectCase(index)
	})
	reset.Importance = widget.LowImportance
	hideControls := widget.NewButton("Hide controls for screenshot", func() {
		lab.editor.Hide()
	})
	hideControls.Importance = widget.LowImportance
	// Keep the long active-case name on its own line. Putting title, code and
	// reset in one Border row asks for more than a phone's content width and
	// makes the two ends overlap precisely where this lab is most useful.
	editorHead := container.NewVBox(
		lab.editorTitle,
		container.NewBorder(nil, nil, lab.editorCode, reset, nil),
	)
	editorObjects := []fyne.CanvasObject{editorHead, lab.editorHost}
	editorObjects = append(editorObjects, channelRows...)
	editorObjects = append(editorObjects, hideControls)
	lab.editor = container.NewVBox(editorObjects...)

	summaryTitle := canvas.NewText("Chosen highlight colours (RGB hex)", chrome.Text)
	summaryTitle.TextStyle = fyne.TextStyle{Bold: true}
	summaryTitle.TextSize = 16
	lab.summary = widget.NewLabel(devHighlightSummary())
	lab.summary.TextStyle = fyne.TextStyle{Monospace: true}

	lab.root = container.NewVBox(
		widget.NewSeparator(),
		title,
		intro,
		caseColumn,
		widget.NewSeparator(),
		lab.editor,
		widget.NewSeparator(),
		summaryTitle,
		lab.summary,
	)
	lab.selectCase(devHighlightActive)
	lab.editor.Hide() // screenshot-ready until a case's Adjust button is tapped
	return lab
}

func (l *devHighlightLab) selectCase(index int) {
	if index < 0 || index >= len(devHighlightCases) {
		return
	}
	devHighlightActive = index
	c := devHighlightColors[index]

	l.syncing = true
	l.sliders[0].SetValue(float64(c.R))
	l.sliders[1].SetValue(float64(c.G))
	l.sliders[2].SetValue(float64(c.B))
	l.syncing = false
	l.refreshEditor(c)
	l.editorSample = newDevHighlightPreview(devHighlightCases[index], c)
	l.editorHost.Objects = []fyne.CanvasObject{l.editorSample.root}
	l.editorHost.Refresh()

	for i, button := range l.buttons {
		button.Importance = widget.MediumImportance
		if i == index {
			button.Importance = widget.HighImportance
		}
		button.Refresh()
	}
}

func (l *devHighlightLab) setChannel(channel int, value uint8) {
	if l.syncing || channel < 0 || channel >= len(l.sliders) {
		return
	}
	index := devHighlightActive
	c := devHighlightColors[index]
	switch channel {
	case 0:
		c.R = value
	case 1:
		c.G = value
	case 2:
		c.B = value
	}
	l.setColor(index, c)
}

func (l *devHighlightLab) setColor(index int, c color.NRGBA) {
	if index < 0 || index >= len(devHighlightColors) {
		return
	}
	c.A = 255
	devHighlightColors[index] = c
	l.previews[index].setWash(c)
	if index == devHighlightActive && l.editorSample != nil {
		l.editorSample.setWash(c)
	}
	l.codes[index].Text = devHighlightHex(c)
	l.codes[index].Refresh()
	l.summary.SetText(devHighlightSummary())
	if index == devHighlightActive {
		l.refreshEditor(c)
	}
}

func (l *devHighlightLab) refreshEditor(c color.NRGBA) {
	l.editorTitle.Text = "Editing: " + devHighlightCases[devHighlightActive].label
	l.editorTitle.Refresh()
	l.editorCode.Text = devHighlightHex(c)
	l.editorCode.Refresh()
	values := [3]uint8{c.R, c.G, c.B}
	for i, value := range values {
		l.channelValue[i].SetText(fmt.Sprintf("%d", value))
	}
}

func devHighlightHex(c color.NRGBA) string {
	return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
}

func devHighlightSummary() string {
	var b strings.Builder
	for i, c := range devHighlightCases {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%-15s %s", c.summaryLabel, devHighlightHex(devHighlightColors[i]))
	}
	return b.String()
}
