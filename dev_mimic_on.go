//go:build bibletextdev

package bibletext

// Platform mimic: run the macOS DEV build down the Windows/Linux code paths.
// DEVELOPMENT BUILDS ONLY — see dev_mimic_off.go for why this is a build tag
// rather than a runtime flag.
//
// WHY IT EXISTS (owner task #43). The Windows VM is unusable (GL emulation)
// and the Linux ARM VM is audio-only, so there was no way to EYEBALL the
// Windows/Linux UI before the CI visual smokes. Every reading-surface
// divergence between macOS and Windows/Linux already sits behind a runtime
// seam (a var initialised from a per-platform constant — useStyledPane,
// sheetConsumeClosure, nativeNoteSticker, reporterLayout, ttsSupported,
// washIsLiveMutation); this file is the one place that flips them all to the
// target platform's answers at startup. The full covered/not-covered contract
// lives in docs/PLATFORM_MIMIC.md — the CI visual smokes and real hardware
// remain the final word.
//
// Launch:
//
//	BIBLETEXT_MIMIC=windows go run -tags bibletextdev ./cmd/desktop
//	BIBLETEXT_MIMIC=linux   go run -tags bibletextdev ./cmd/desktop
//
// The app header shows a "MIMIC: Windows" / "MIMIC: Linux" badge while active,
// so a screenshot from this mode can never masquerade as the real platform.

import (
	"os"
	"strings"
)

// devMimicTargetValue is the validated BIBLETEXT_MIMIC value — "windows",
// "linux", or "" when the mode is off. Written once by devApplyMimic before
// the UI exists; read-only after that.
var devMimicTargetValue string

// devMimicTarget reports the active mimic target ("" = off). Untagged code
// (reading_macos.go's share branch, the header badge) asks this; the release
// twin answers a constant "".
func devMimicTarget() string { return devMimicTargetValue }

// devMimicLabel is the header badge text — empty when the mode is off.
func devMimicLabel() string {
	switch devMimicTargetValue {
	case "windows":
		return "MIMIC: Windows"
	case "linux":
		return "MIMIC: Linux"
	}
	return ""
}

// devApplyMimic reads BIBLETEXT_MIMIC and, for a recognised target, flips the
// runtime seams to that platform's answers. Called once at the very top of
// Run() — before CreateMainUI installs the sheet-close consume closure and
// before loadBookFonts picks the scripture face, both of which read seams this
// sets. Unknown values are ignored (the mode simply stays off).
func devApplyMimic() {
	t := strings.ToLower(strings.TrimSpace(os.Getenv("BIBLETEXT_MIMIC")))
	if t != "windows" && t != "linux" {
		return
	}
	devMimicTargetValue = t
	devApplyMimicSeams(t)
}

// devApplyMimicSeams flips every seam the platform audit confirmed. Split from
// the env read so tests can drive a target directly.
func devApplyMimicSeams(target string) {
	// The reading surface: the styled pane Windows/Linux ship, instead of the
	// NSTextView overlay. readingScrollArea (reading_macos.go) dispatches on
	// this; setReadingOverlayVisible and the read-along entry points early-out
	// on it so the native view is never created.
	useStyledPane = func() bool { return true }

	// With the native overlay absent, its showReadingOverlay closure (which
	// ends on consumeDeferredFullRebuild) is never assigned — the Windows/Linux
	// stand-in consume point must activate, or a theme flip / background data
	// swap under an open sheet leaks the deferred rebuild.
	sheetConsumeClosure = func() bool { return true }

	// Notes surface as the Fyne banner above the pane (chips, notice path,
	// hide/delete verbs) — the exact Windows/Linux surface — instead of
	// standing down for the native in-text sticker.
	nativeNoteSticker = func() bool { return false }

	// Reporter truth follows the TARGET platform: false, so the styled pane
	// applies its own width-gated reporter typesetting (reading_styled_pane.go
	// relayout), exactly as on Windows/Linux.
	reporterLayout = func() bool { return false }

	// No read-aloud on desktop Windows/Linux: hides the "Read aloud" source
	// row and, via chapterAudioAvailable, the whole audio button on chapters
	// with no recording (licensed versions, the deuterocanon).
	ttsSupported = func() bool { return false }

	// A wash change on the styled pane is a re-render that carries its own
	// scroll — arrivals must NOT declare forceReposition (which nothing would
	// clear here; see reading_tint_apple.go).
	washIsLiveMutation = false

	// Scripture face: Windows ships Georgia (the same family macOS loads
	// first, so the list stands); Linux ships DejaVu Serif, which this Mac
	// does not have at the Linux paths — dropping the Georgia candidates lets
	// the styled pane fall back to the embedded Gelasio, the app's own
	// no-serif-found path. The doc states the Linux face is approximated.
	if target == "linux" {
		var linuxOnly [][4]string
		for _, set := range serifFontCandidates {
			if strings.Contains(strings.ToLower(set[0]), "dejavu") {
				linuxOnly = append(linuxOnly, set)
			}
		}
		serifFontCandidates = linuxOnly
	}
}
