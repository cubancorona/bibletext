package bibletext

import (
	"fmt"
	"image/color"
	"math"
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
// ── THE NOTE SPACING SPEC ────────────────────────────────────────────────────
//
// ONE table, four surfaces. Before 19 Aug 2026 each of the four note surfaces
// carried its own numbers and they disagreed on every one of them — the band
// above the card was 0 (iOS), a measured line (macOS), 8dp (Android) and 10
// (this pane); the pill was 30 / 24 / ~26 / 28 for the same object; and
// Android's card used a different internal rhythm again (6 top, 4 right, 10
// bottom, and a who row whose height was set by a verb glyph). The owner asked
// for one answer — "measure the pill and note vertical spacing and margins
// visually … make sure it's the same there. Really this should be done for all
// platforms" — so the numbers live HERE and every surface reads them.
//
// The natives cannot import a Go constant into Objective-C or Java, so they
// carry named constants of their own and notes_spacing_spec_test.go PARSES
// those three sources and asserts each literal equals this table (the mechanism
// dev_links_guard_test.go uses on the release scripts). A push parameter was
// the alternative and was rejected: these are compile-time layout constants,
// not per-chapter data, and pushing them would put a wire format and a
// three-platform ABI bump in front of every future 1pt change.
//
// WHAT EACH NUMBER MEANS:
//
//	GapAbove   the air RESERVED above the card's top edge.
//	GapBelow   the air reserved below the drawn shape — the tail's apex for a
//	           card, the pill's bottom edge for a pill — to the first line of
//	           the passage. This is THE PINNED INVARIANT: it is the one
//	           distance a reader consciously reads (tail → the words it points
//	           at), so macOS places the sticker bottom-up off the passage and
//	           lets every measurement error fall into the gap above, where
//	           there is already air.
//	Pad        the card's inner padding, all four sides.
//	WhoGap     the who row's bottom edge → the message's first line.
//	WhoH       NOT a literal: ceil(whoSize × noteWhoRatio), because the correct
//	           box for an 11pt semibold system face (14) is the wrong box for a
//	           10pt one (13, macOS). A flat 14 in this table would be a bug on
//	           macOS.
//	Radius     the card's corner radius. Was 8 on this pane alone — borrowed
//	           from surface()'s chrome, never chosen for this card, while the
//	           three natives all drew 10.
//	PillH      the collapsed marker's height, spec'd INDEPENDENTLY of the verb
//	           buttons. All four used to derive it from their own button metric
//	           (30 iOS touch minimum, 24 macOS pointer, ~26 Android wrap, 28
//	           here), which is a touch-target decision leaking into a piece of
//	           content: the pill must not change height because a platform
//	           moved its tap targets.
//
// WHAT THIS TABLE DOES NOT OWN — the residual, stated rather than pretended.
// GapAbove is a RESERVATION. The air a reader SEES above the card is
// GapAbove + whatever paragraph separator the reading layout already puts
// between paragraphs, and that separator belongs to the reading page, not to
// the note: it is the same for every paragraph in the chapter, note or no note,
// and cancelling it under a note would give the note's paragraph LESS
// separation than its neighbours. It is 24px on iOS phones, 0 in the reporter
// layout (iPad/macOS/wide styled pane), one blank line on Android, and ParaGap
// on the narrow styled pane. Named per platform in docs/NOTES_SCRAPBOOK.md.
//
// THE TAIL is what makes it read as somebody speaking rather than as a card.
// Its geometry is copied from the native sticker (reading_macos.go,
// btMacNoteBubblePath): nine points deep, eighteen wide, twenty-four in from the
// left, pointing DOWN at the passage.
const (
	noteGapAbove  = 10
	noteGapBelow  = 10
	notePad       = 12
	noteWhoGap    = 4
	noteWhoRatio  = 1.27 // whoH = ceil(whoSize × this): 14 at 11pt, 13 at 10pt
	noteTailDepth = 9
	noteTailWidth = 18
	noteTailInset = 24
	// The FYNE bubble's corner — the notes browser's rows and the reading
	// banner. It is surface()'s radius on purpose (theme.go), so a bubble sits
	// inside the app's other cards without a different curve; the audio button,
	// the crossref panel, the search rows and the version picker are all 8 too.
	// The STICKER's own corner is noteStickerRad (the native panes use 10);
	// they were briefly one constant, which quietly moved the browser and the
	// banner to a curve nobody asked for (refuter finding).
	noteBubbleRad = 8
	// The in-text sticker's corner, shared by all four note surfaces.
	noteStickerRad = 10
	notePillH      = 28
)

// noteSpacing is the spec above as one value, so a consumer reads a NAMED
// field rather than a bare constant and the spec can be passed around whole.
type noteSpacing struct {
	GapAbove  float32
	GapBelow  float32
	Pad       float32
	WhoGap    float32
	TailDepth float32
	TailWidth float32
	TailInset float32
	Radius    float32
	PillH     float32
}

// noteMetrics is THE table. Every surface reads this or is held to it by
// notes_spacing_spec_test.go.
//
// A function, not a package var: a var would let any code — or any test — assign
// to the spec at runtime, and a spec that can be reassigned is not a spec
// (refuter finding).
func noteMetrics() noteSpacing { return noteSpacingTable }

var noteSpacingTable = noteSpacing{
	GapAbove:  noteGapAbove,
	GapBelow:  noteGapBelow,
	Pad:       notePad,
	WhoGap:    noteWhoGap,
	TailDepth: noteTailDepth,
	TailWidth: noteTailWidth,
	TailInset: noteTailInset,
	Radius:    noteStickerRad,
	PillH:     notePillH,
}

// WhoH is the who row's box height for a given who-line font size — derived,
// never a literal, so the same rule serves iOS's 11pt semibold (14) and macOS's
// 10pt one (13).
func (s noteSpacing) WhoH(size float32) float32 {
	return float32(math.Ceil(float64(size) * noteWhoRatio))
}

// noteSVGHex spells a colour the way Fyne's SVG loader will accept it: SIX hex
// digits, alpha as its own attribute.
//
// Fyne rejects #RRGGBBAA outright ("color string ... is not length 3 or 6"),
// and the original 8-digit spelling here meant the tail image FAILED TO LOAD on
// every build since the bubble shipped — the bubble rendered as a plain card
// and the failure was one quiet log line in a test nobody grepped. It is shared
// with the styled pane's one-path builder (noteBubblePathSVG,
// reading_styled_note.go) so the second SVG author cannot re-learn it.
func noteSVGHex(c color.Color) (string, float64) {
	r, g, b, a := c.RGBA()
	return fmt.Sprintf("#%02X%02X%02X", r>>8, g>>8, b>>8), float64(a>>8) / 255
}

// noteTailSVG draws the tail: filled, with the two SLANTED edges stroked and
// the mouth left open so it merges into the bubble above it.
//
// The top row of the fill deliberately overlaps the bubble's own bottom border
// by a point, and the tail is drawn after the bubble, so the border does not
// run visibly across the tail's mouth — which is what would make it read as a
// triangle stuck to a box rather than as one shape.
func noteTailSVG(fill, stroke color.Color) fyne.Resource {
	fillHex, fillA := noteSVGHex(fill)
	strokeHex, strokeA := noteSVGHex(stroke)
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
