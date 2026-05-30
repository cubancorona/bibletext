// Command mobile is the iOS (and Android) entry point for the Holy Bible reader.
// It is built and packaged by the `fyne` CLI:
//
//	# iOS device or simulator (requires Xcode + an Apple Developer profile):
//	fyne package -os ios -appID com.willow.holybible -src ./cmd/mobile
//
//	# Android (requires the Android SDK + NDK):
//	fyne package -os android -appID com.willow.holybible -src ./cmd/mobile
//
// On phones the mobile layout (bottom tabs: Read / Books / Search) is selected
// automatically via the `//go:build ios || android` tag on ui_mobile.go.
package main

import (
	"fyne.io/fyne/v2/app"

	"holybible"
)

func main() {
	myApp := app.NewWithID("com.willow.holybible")
	state := holybible.LoadAndPrepareState()

	w := myApp.NewWindow("Holy Bible")
	// On iOS / Android the OS controls the window size; Resize is a no-op there.
	w.SetContent(holybible.CreateMainUI(myApp, state, w))
	holybible.ObserveSystemThemeChanges(myApp, state)
	w.ShowAndRun()
}
