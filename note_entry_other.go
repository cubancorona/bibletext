//go:build !ios && !android

package bibletext

// Desktop composers keep their existing Fyne entry. The native editor seam
// is implemented separately for iOS and Android.

import "fyne.io/fyne/v2"

func nativeNoteEntrySupported() bool { return false }

func showNativeNoteEntry(initial, placeholder string, pal palette) {}

func setNativeNoteEntryFrameFromObject(obj fyne.CanvasObject) {}

func nativeNoteEntryText() string { return "" }

func hideNativeNoteEntry() {}
