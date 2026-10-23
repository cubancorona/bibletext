package bibletext

// A SHARE THE READER STARTED DOES NOT END IN SILENCE (docs/ROADMAP.md, Later,
// row 2). Where a share cannot finish, the reader is told so in one line, in
// the words the phones already used for it:
//
//   - a card the preview cannot draw, on all six (showShareImagePreview);
//   - on the desktop share sheet (Linux, the Windows Share sheet's fallback
//     and the macOS mimic), a card that could not be read at the tap and a
//     picture no folder took that the file manager did not show
//     (fallbackShareImage); and a card gone at the tap on the Mac's own share
//     (reading_macos.go), as on iOS and Android;
//   - on iPhone and iPad, a share sheet iOS still will not show once the
//     sheet before it has finished opening or closing (reading_ios.go).
//
// Email… on the desktop sheet has its own line, under its buttons
// (shareLineEmailFailed, share_sheet_desktop.go).
//
// sayShareFailed says the line each platform's own way: iOS in an alert with
// one button (bibleTextShareNotice), Android in a toast (BtBridge.shareNotice),
// and the desktops in the share sheet with the line and Done.

const (
	// shareFailedPassage is said for a text share that cannot finish.
	shareFailedPassage = "Could not share the passage."
	// shareFailedCard is said for a picture share that cannot finish.
	shareFailedCard = "Could not share the card."
)
