package bibletext

import "fyne.io/fyne/v2"

// composeAreaAboveKeyboard intersects the driver area with a native editor's
// IME limit. A driver that already excluded the IME must not lose its height twice.
func composeAreaAboveKeyboard(pos fyne.Position, size fyne.Size, canvasHeight, scale, keyboardPx float32) (fyne.Position, fyne.Size) {
	if keyboardPx <= 0 || scale <= 0 {
		return pos, size
	}
	bottom := min(pos.Y+size.Height, canvasHeight-keyboardPx/scale)
	size.Height = max(float32(0), bottom-pos.Y)
	return pos, size
}
