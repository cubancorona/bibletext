//go:build android

package bibletext

import (
	"math"

	"fyne.io/fyne/v2"
)

var androidComposeDraft nativeComposeDraft

// The native slot and passage sizing remain shared. Android additionally
// releases its keyboard when the expanded passage needs that room. Its IME
// belongs to the editor window, so its reported overlap is intersected with
// the activity area instead of assuming the driver has already counted it.
func init() {
	noteEntryKeyboard = func(owner uint64, show bool) {
		if _, _, open := androidComposeDraft.read(owner); open {
			androidNoteEntryFocus(owner, show)
		}
	}
	nativeComposeArea = func(c fyne.Canvas, pos fyne.Position, size fyne.Size) (fyne.Position, fyne.Size) {
		_, px, open := androidComposeDraft.read(noteEntryOwner)
		if !open || px <= 0 || c.Scale() <= 0 {
			return pos, size
		}
		return composeAreaAboveKeyboard(pos, size, c.Size().Height, c.Scale(), px)
	}
}

func nativeNoteEntrySupported() bool { return btaBridgePresent() }

func showNativeNoteEntry(initial, placeholder string, pal palette) {
	owner := noteEntryOwner
	androidComposeDraft.begin(owner, initial)
	scale := float32(1)
	if s := activeAIState; s != nil && s.window != nil && s.window.Canvas().Scale() > 0 {
		scale = s.window.Canvas().Scale()
	} else if h := currentHost; h != nil {
		_, _, scale, _ = nativeHostFrame(h)
	}
	androidNoteEntryShow(owner, initial, placeholder, pal, fieldTextSize*scale, scale)
}

func setNativeNoteEntryFrameFromObject(obj fyne.CanvasObject) {
	if obj == nil {
		return
	}
	pos, size, scale, ok := nativeHostFrame(obj)
	if !ok {
		return
	}
	pixel := func(v float32) int { return int(math.Round(float64(v * scale))) }
	androidNoteEntryFrame(noteEntryOwner, pixel(pos.X), pixel(pos.Y), pixel(size.Width), pixel(size.Height))
}

func nativeNoteEntryText() string {
	text, _, _ := androidComposeDraft.read(noteEntryOwner)
	return text
}

func hideNativeNoteEntry() {
	owner := noteEntryOwner
	androidComposeDraft.end(owner)
	androidNoteEntryHide(owner)
	noteSoftKeyboard(0)
}
