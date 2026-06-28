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

//go:embed assets/icons/headphones.svg
var headphonesIconSVG []byte

// iconSpeak is the "read aloud / voice" source glyph (a person + sound bars) used
// to mark a chapter played via on-device text-to-speech. Fyne has no
// person-speaking icon, so it's a bundled SVG; falls back to VolumeUpIcon if the
// asset is somehow missing, so the tag is never blank.
var iconSpeak fyne.Resource = func() fyne.Resource {
	if len(speakIconSVG) == 0 {
		return theme.VolumeUpIcon()
	}
	return fyne.NewStaticResource("speak.svg", speakIconSVG)
}()

// iconHeadphones is the source glyph for a streamed recording — distinct from the
// read-aloud voice glyph so the audio source is legible at a glance. Falls back to
// VolumeUpIcon if the asset is missing.
var iconHeadphones fyne.Resource = func() fyne.Resource {
	if len(headphonesIconSVG) == 0 {
		return theme.VolumeUpIcon()
	}
	return fyne.NewStaticResource("headphones.svg", headphonesIconSVG)
}()
