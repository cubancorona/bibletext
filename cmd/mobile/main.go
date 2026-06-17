// Command mobile is the iOS (and Android) entry point for the BibleText reader.
// It is built and packaged by the `fyne` CLI:
//
//	# iOS device or simulator (requires Xcode + an Apple Developer profile):
//	fyne package -os ios -appID com.willow.bibletext -src ./cmd/mobile
//
//	# Android (requires the Android SDK + NDK):
//	fyne package -os android -appID com.willow.bibletext -src ./cmd/mobile
//
// On phones the mobile layout (bottom tabs: Read / Books / Search) is selected
// automatically via the `//go:build ios || android` tag on ui_mobile.go.
package main

import (
	"fyne.io/fyne/v2/app"

	"bibletext"
)

func main() {
	myApp := app.NewWithID("com.willow.bibletext")
	// The Bible loads on a background goroutine so the window appears instantly
	// with a loading spinner — crucial on iOS, where a slow first-run fetch on
	// the main thread would trip the launch watchdog and get the app SIGKILLed.
	state := bibletext.NewLoadingState()

	w := myApp.NewWindow("BibleText")
	// On iOS / Android the OS controls the window size; Resize is a no-op there.
	w.SetContent(bibletext.CreateMainUI(myApp, state, w)) // shows the spinner
	bibletext.ObserveSystemThemeChanges(myApp, state)
	bibletext.InstallReadingStateFlush(myApp, w, state)
	bibletext.StartBackgroundLoad(myApp, w, state)
	w.ShowAndRun()
}
