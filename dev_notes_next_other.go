//go:build bibletextdev && !darwin

package bibletext

// devNoteNextTap drives the Apple sticker's next-tap proof; no sticker here
// (Android and the desktop styled panes carry the Fyne banner instead, whose
// chips are the selector), so the scenario step is a no-op.
func devNoteNextTap(*AppState) {}
