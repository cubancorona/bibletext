//go:build !darwin && !android

package bibletext

// Windows and Linux: no native reading overlay floats over the canvas, so no
// reading-view builder ever assigns state.hideReadingOverlay /
// state.showReadingOverlay — and with them absent there was no sheet-close
// moment to consume a deferred full rebuild (a theme flip or a background data
// swap that landed while a sheet was open). installSheetCloseConsume (app.go)
// fills exactly that gap here. See the `off` twin.
const sheetConsumeClosureOnPlatform = true
