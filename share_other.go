//go:build !darwin && !android

package bibletext

// Desktop share verbs for the platforms without a system share sheet
// (Linux/Windows; darwin has NSSharingServicePicker / UIActivityViewController,
// Android has ACTION_SEND via BtBridge). The bodies live in share_fallback.go
// (untagged-for-desktop, so the darwin platform-mimic dev mode can reach the
// same code); these wrappers are what keeps the Windows/Linux release path
// byte-identical to before the extraction:
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
