//go:build !darwin && !android && !windows

package bibletext

// Desktop share verbs for Linux, which has no system share sheet to call:
// xdg-desktop-portal has no Share portal (darwin has NSSharingServicePicker
// and UIActivityViewController, Android ACTION_SEND via BtBridge, Windows its
// Share sheet, share_windows.go). The bodies live in share_fallback.go,
// untagged for the desktop so that Windows reaches them as its Share sheet's
// fallback and the darwin platform-mimic dev mode can reach them too:
//
//   - the text verbs      → the composed text goes to the CLIPBOARD, and the
//     confirmation sheet says so and shows it (share_sheet_desktop.go) —
//     ready to paste anywhere.
//   - Share as image      → the rendered PNG is saved to ~/Downloads (falling
//     back to the temp copy) and revealed in the file manager, and the same
//     sheet says where it went.

// Each platform's share mechanism is recorded in docs/PLATFORM_MATRIX.md, Sharing.
func nativeShareText(s string) { fallbackShareText(s) }

// Each platform's share mechanism is recorded in docs/PLATFORM_MATRIX.md, Sharing.
func nativeShareImage(path string) { fallbackShareImage(path) }
