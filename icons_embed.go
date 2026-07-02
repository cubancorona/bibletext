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

//go:embed assets/icons/skip_back_15.svg
var skipBack15SVG []byte

//go:embed assets/icons/skip_fwd_15.svg
var skipFwd15SVG []byte

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

// iconSkipBack15 / iconSkipFwd15 are the ±15-second skip glyphs (a loop arrow with
// "15"), distinct from track-skip. Fall back to the fast-rewind/forward icons if
// the asset is missing.
var iconSkipBack15 fyne.Resource = func() fyne.Resource {
	if len(skipBack15SVG) == 0 {
		return theme.MediaFastRewindIcon()
	}
	return fyne.NewStaticResource("skip_back_15.svg", skipBack15SVG)
}()

var iconSkipFwd15 fyne.Resource = func() fyne.Resource {
	if len(skipFwd15SVG) == 0 {
		return theme.MediaFastForwardIcon()
	}
	return fyne.NewStaticResource("skip_fwd_15.svg", skipFwd15SVG)
}()
