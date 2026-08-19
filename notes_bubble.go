package bibletext

import (
	"fmt"
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// The note bubble, as the reading page draws it.
//
// ONE BUILDER, used by the reading banner and by the notes browser, because the
// owner asked for the two to match and two hand-built lookalikes drift the
// moment either is touched.
//
// The TAIL is what makes it read as somebody speaking rather than as a card.
// Its geometry is copied from the native sticker (reading_macos.go,
// btMacNoteBubblePath): nine points deep, eighteen wide, twenty-four in from the
// left, pointing DOWN at the passage. Those numbers are duplicated rather than
// shared because the other side of the pair lives in Objective-C inside a cgo
// preamble; the constants below name their source so a change to one is at
// least findable from the other.
const (
	noteTailDepth = 9
	noteTailWidth = 18
	noteTailInset = 24
	noteBubbleRad = 8 // surface()'s corner radius
)

// noteTailSVG draws the tail: filled, with the two SLANTED edges stroked and
// the mouth left open so it merges into the bubble above it.
//
// The top row of the fill deliberately overlaps the bubble's own bottom border
// by a point, and the tail is drawn after the bubble, so the border does not
// run visibly across the tail's mouth — which is what would make it read as a
// triangle stuck to a box rather than as one shape.
func noteTailSVG(fill, stroke color.Color) fyne.Resource {
	// SIX hex digits, alpha as its own attribute. Fyne's SVG loader rejects
	// #RRGGBBAA outright ("color string ... is not length 3 or 6"), and the
	// original 8-digit spelling here meant the tail image FAILED TO LOAD on
	// every build since the bubble shipped — the bubble rendered as a plain
	// card and the failure was one quiet log line in a test nobody grepped.
	hex := func(c color.Color) (string, float64) {
		r, g, b, a := c.RGBA()
		return fmt.Sprintf("#%02X%02X%02X", r>>8, g>>8, b>>8), float64(a>>8) / 255
	}
	fillHex, fillA := hex(fill)
	strokeHex, strokeA := hex(stroke)
	w, d := noteTailWidth, noteTailDepth
	svg := fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`+
			`<path d="M0 0 L%d %d L%d 0 Z" fill="%s" fill-opacity="%.3f"/>`+
			`<path d="M0 0 L%d %d L%d 0" fill="none" stroke="%s" stroke-opacity="%.3f" stroke-width="1" `+
			`stroke-linejoin="round"/></svg>`,
		w, d+1, w, d+1,
		w/2, d, w, fillHex, fillA,
		w/2, d, w, strokeHex, strokeA,
	)
	return fyne.NewStaticResource("note-tail.svg", []byte(svg))
}

// bubbleLayout puts the tail under the bubble's lower-left, overlapping its
// border by a point.
type bubbleLayout struct{}

func (bubbleLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) == 0 {
		return fyne.Size{}
	}
	m := objects[0].MinSize()
	return fyne.NewSize(m.Width, m.Height+noteTailDepth)
}

func (bubbleLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) == 0 {
		return
	}
	bodyH := size.Height - noteTailDepth
	objects[0].Move(fyne.NewPos(0, 0))
	objects[0].Resize(fyne.NewSize(size.Width, bodyH))
	if len(objects) < 2 {
		return
	}
	// One point up, so the fill covers the bubble's bottom border.
	objects[1].Move(fyne.NewPos(noteTailInset, bodyH-1))
	objects[1].Resize(fyne.NewSize(noteTailWidth, noteTailDepth+1))
}

// noteBubble draws somebody's words the way the reading page draws them.
//
// The text inside is UNTRUSTED — another person's message. It is a Label, never
// markup, never RichText, and it is never the app speaking: whatever wraps this
// must attribute it. noteBubbleWithByline does that; use it unless you have a
// reason not to.
func noteBubble(text string, pal palette) fyne.CanvasObject {
	return noteBubblePadded(text, pal, theme.Padding())
}

// noteBubblePadded is noteBubble with the card's inner padding as a parameter.
// The bubble's IDENTITY — the rounded bordered card and the tail — is the
// same everywhere (owner: list bubbles match reading bubbles); its DENSITY is
// not: the notes browser packs rows (browseBubblePad, notes_browse.go) while
// the reading banner keeps the page's own theme.Padding(), which is what the
// default above passes. surface() cannot be reused for the tight case because
// container.NewPadded reads the GLOBAL theme padding, out of reach of any
// row-scoped override.
func noteBubblePadded(text string, pal palette, pad float32) fyne.CanvasObject {
	body := widget.NewLabel(strings.TrimSpace(text))
	body.Wrapping = fyne.TextWrapWord

	frame := canvas.NewRectangle(pal.SurfaceAlt)
	frame.StrokeColor = pal.Border
	frame.StrokeWidth = 1
	frame.CornerRadius = noteBubbleRad
	card := container.NewStack(frame,
		container.New(layout.NewCustomPaddedLayout(pad, pad, pad, pad), body))
	tail := canvas.NewImageFromResource(noteTailSVG(pal.SurfaceAlt, pal.Border))
	tail.FillMode = canvas.ImageFillStretch
	return container.New(bubbleLayout{}, card, tail)
}

// noteBubbleWithByline is the bubble plus the attribution that must always
// accompany it, and the attribution sits OUTSIDE the bubble (owner directive).
//
// That is not decoration. Inside the bubble, a line saying who a note is from
// would read as part of the message — as though the sender had typed it.
// Outside, it reads as what it is: the app telling you where this came from.
// The bubble holds their words and nothing else.
//
// The TRANSLATION is not here any more: it rides in the heading beside the
// reference, in parentheses, where it belongs with the other fact about which
// passage this is (owner).
func noteBubbleWithByline(text, byline string, pal palette) fyne.CanvasObject {
	rows := []fyne.CanvasObject{noteBubble(text, pal)}
	if who := strings.TrimSpace(byline); who != "" {
		lbl := widget.NewLabel(who)
		lbl.TextStyle = fyne.TextStyle{Italic: true}
		lbl.Importance = widget.LowImportance
		rows = append(rows, lbl)
	}
	return container.NewVBox(rows...)
}

// noteVersionAbbrev is the translation a note was written in, short enough to
// sit in a heading: "WEB", "BSB", "NKJV". An id the registry does not know
// degrades to the id itself rather than to nothing — a note is still from
// SOMEWHERE, and saying so beats implying it came from the translation on
// screen.
func noteVersionAbbrev(id string) string {
	if strings.TrimSpace(id) == "" {
		return ""
	}
	if v, ok := versionByID(id); ok {
		if v.Abbrev != "" {
			return v.Abbrev
		}
		return v.Name
	}
	return strings.ToUpper(id)
}

// noteVersionName is the full translation name, for anywhere with room for it.
func noteVersionName(id string) string {
	if strings.TrimSpace(id) == "" {
		return ""
	}
	if v, ok := versionByID(id); ok {
		return v.Name
	}
	return id
}
