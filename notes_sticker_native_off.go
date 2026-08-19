//go:build !darwin && !android

package bibletext

// Windows and Linux. This says the PLATFORM has no native sticker — no
// UITextView bubble, no NSTextView twin, no Android TextView overlay — and that
// remains true. It no longer means "the banner is how a note is visible here":
// as of 19 Aug the styled reading pane draws its own in-text band, card and
// tail in pure Fyne (reading_styled_note.go), and the banner stands down for
// it. That is why the seam next door reads
// `nativeNoteStickerOnPlatform || useStyledPane()` rather than this constant
// alone — the question the banner actually needs answered is "does the pane in
// front of the reader draw the note?", and here the answer is yes, just not
// natively. Flip styledPaneEnabledOnPlatform back to the legacy Entry pane and
// the banner returns on its own.
//
// Android LEFT this set on 19 Aug — it draws the native sticker in both of its
// reading modes, for parity with iOS (see the `on` twin).
const nativeNoteStickerOnPlatform = false
