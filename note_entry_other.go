//go:build !ios

package bibletext

// The native Share-with-note compose field exists only on iOS (see
// note_entry_ios.go for why). Everywhere else the sheet keeps its Fyne Entry,
// and these stubs let the untagged sheet code ask without build tags.

import "fyne.io/fyne/v2"

func nativeNoteEntrySupported() bool { return false }

func showNativeNoteEntry(initial, placeholder string, pal palette) {}

func setNativeNoteEntryFrameFromObject(obj fyne.CanvasObject) {}

func nativeNoteEntryText() string { return "" }

func hideNativeNoteEntry() {}
