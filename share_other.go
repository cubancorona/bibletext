//go:build !darwin && !android

package bibletext

// Desktop share verbs for the platforms without a system share sheet
// (Linux/Windows; darwin has NSSharingServicePicker / UIActivityViewController,
// Android has ACTION_SEND via BtBridge). The bodies live in share_fallback.go
// (untagged-for-desktop, so the darwin platform-mimic dev mode can reach the
// same code); these wrappers are what keeps the Windows/Linux release path
// byte-identical to before the extraction:
//
//   - Share with citation → the composed quote+citation goes to the CLIPBOARD,
//     with a brief confirmation notice — ready to paste anywhere.
//   - Share as image      → the rendered PNG is saved to ~/Downloads (falling
//     back to the temp copy) and revealed in the file manager.

func nativeShareText(s string) { fallbackShareText(s) }

func nativeShareImage(path string) { fallbackShareImage(path) }
