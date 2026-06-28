package bibletext

// Bundled UI icons that Fyne's theme set doesn't provide. Embedded in the binary
// (like the fonts in fonts_embed.go) so they ship identically on every platform.
// Each is a single-fill monochrome SVG so theme.NewColoredResource can tint it to
// the chrome colour at the use site, exactly like the built-in theme icons.

import (
	_ "embed"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

//go:embed assets/icons/speak.svg
var speakIconSVG []byte

// iconSpeak is the distinct "read aloud / voice" glyph for the TTS play button,
// kept visually separate from the recorded play triangle so recorded-vs-spoken is
// legible at a glance. Fyne has no person-speaking icon, so it's a bundled SVG;
// falls back to VolumeUpIcon if the asset is somehow missing, so the button is
// never blank.
var iconSpeak fyne.Resource = func() fyne.Resource {
	if len(speakIconSVG) == 0 {
		return theme.VolumeUpIcon()
	}
	return fyne.NewStaticResource("speak.svg", speakIconSVG)
}()
