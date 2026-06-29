package bibletext

// Bundled UI icons that Fyne's theme set doesn't provide. Embedded in the binary
// (like the fonts in fonts_embed.go) so they ship identically on every platform.
// Single-fill monochrome SVGs so theme.NewColoredResource can tint them to the
// chrome colour at the use site, exactly like the built-in theme icons.

import (
	_ "embed"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

//go:embed assets/icons/soundwave.svg
var soundwaveIconSVG []byte

// iconAudioWave is the "read aloud / text-to-speech" source glyph (a small
// equalizer-style waveform), marking a chapter played by on-device speech as
// distinct from a recorded human narration (which uses theme.AccountIcon, a
// person). Falls back to VolumeUpIcon if the asset is somehow missing.
var iconAudioWave fyne.Resource = func() fyne.Resource {
	if len(soundwaveIconSVG) == 0 {
		return theme.VolumeUpIcon()
	}
	return fyne.NewStaticResource("soundwave.svg", soundwaveIconSVG)
}()
