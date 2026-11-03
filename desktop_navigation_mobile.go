//go:build ios || android

package bibletext

import "fyne.io/fyne/v2"

// Touch devices retain their rotation watcher and immediate layout rebuild.
func watchCompactNavigation(_ *AppState, body *fyne.Container, _, _ fyne.CanvasObject) fyne.CanvasObject {
	return body
}
