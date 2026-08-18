//go:build bibletextdev && darwin

package bibletext

// devNoteNextTap posts the sticker's next-tap through the SAME //export the
// native count-region button targets (bibleTextNoteNextTapped,
// ai_menu_darwin.go — the ObjC action btNoteNext: is one selector away), so
// the headless proofs exercise the real callback path: the simulator has no
// tap command, and the macOS capture run has no pointer either.
func devNoteNextTap(*AppState) { bibleTextNoteNextTapped() }
