package bibletext

// THE APP'S ONE TRASH CAN.
//
// Every place a press destroys something wears this, at the same weight and in
// the same ink: the notes list's row control, the history bar's clear, the
// banner's delete, "Delete all notes" in Settings, and the note card on all
// four reading surfaces. A reader should never have to work out which bin means
// what, and two different bins in one app is exactly the ambiguity the glyph is
// there to remove.
//
// WHY NOT theme.DeleteIcon(). Fyne's own is a SOLID bin — a filled slab that
// reads as a heavy block at any size, and at the sizes this app uses it, as a
// blot (owner: "less... solid black", "NO HUGE TRASH CANS"). This one is drawn
// as an OUTLINE with the ridges a bin actually has, so at 16pt it still reads
// as a bin rather than as a dark rectangle, and it takes the muted ink rather
// than the foreground.
//
// WHY IT IS DRAWN HERE rather than taken from each platform's icon set. The
// same shape has to appear on Fyne, on two Objective-C panes and on Android; a
// per-platform icon would be four slightly different bins, which is what the
// note sticker spent a day converging AWAY from. The geometry lives in one
// place and every surface renders the same path.
//
// It is deliberately STROKED, never filled: a stroke keeps its weight when the
// icon shrinks, while a fill gets darker and heavier as the shape gets smaller.

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
)

// noteTrashPath is the bin, on a 24×24 viewBox, as SVG path data.
//
// Three parts, in one string so no surface can draw two of them and miss the
// third: the lid line and its handle, the can's body, and the three ridges. The
// ridges are what stop it reading as a solid block at small sizes, and they are
// the reason a reader recognises it as a bin at a glance rather than by
// deduction.
//
// The Android vector drawable and the Apple panes' bezier paths are generated
// from these same coordinates — the format differs, the geometry does not.
const (
	noteTrashBody   = "M4 6.5h16M9.5 3.5h5a1 1 0 0 1 1 1v2h-7v-2a1 1 0 0 1 1-1zM6 6.5v13.5a2 2 0 0 0 2 2h8a2 2 0 0 0 2-2V6.5"
	noteTrashRidges = "M9.7 10.2v8M12 10.2v8M14.3 10.2v8"

	// noteTrashStroke is light on purpose. At 1.5 the ridges close up into a
	// dark centre once the icon is drawn small; at 1.2 they stay separate and
	// the whole mark stays quiet beside the text it sits next to.
	noteTrashStroke = 1.2

	// noteTrashSize is the drawn size everywhere. Small enough to be furniture
	// beside a note's bubble, large enough to stay a comfortable tap target
	// (the BUTTON around it is what carries the touch area, not the drawing).
	noteTrashSize = 16
)

// noteTrashIcon is the bin as a Fyne resource, in a colour of the caller's
// choosing — normally the palette's muted ink, so it sits at the same weight as
// the byline and the date rather than competing with the message.
//
// The colour is BAKED IN rather than left to the theme: Fyne recolours only its
// own ThemedResource, and a static resource is what lets the same drawing take
// the note palette's muted ink on one surface and the chrome's on another.
func noteTrashIcon(c color.Color) fyne.Resource {
	hex, alpha := noteSVGHex(c)
	svg := fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="24" height="24">`+
			`<g fill="none" stroke="%s" stroke-opacity="%.3f" stroke-width="%.1f" `+
			`stroke-linecap="round" stroke-linejoin="round">`+
			`<path d="%s"/><path d="%s"/></g></svg>`,
		hex, alpha, noteTrashStroke, noteTrashBody, noteTrashRidges)
	return fyne.NewStaticResource("bt-trash.svg", []byte(svg))
}
