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

//go:embed assets/icons/sidebar_left.svg
var sidebarLeftSVG []byte

//go:embed assets/icons/note_bubble.svg
var noteBubbleSVG []byte

//go:embed assets/icons/footnote.svg
var footnoteIconSVG []byte

// iconAudioWave is the "read aloud / text-to-speech" source glyph (a small
// equalizer-style waveform), marking a chapter played by on-device speech as
// distinct from a recorded human narration (which uses theme.AccountIcon, a
// person). Falls back to VolumeUpIcon if the asset is somehow missing.
//
// THEMED, because it is handed to widget.NewButtonWithIcon (audio_menu.go). See
// the note on iconNoteBubble: a bare static resource there renders the asset's
// own fill, and every asset in this directory is fill="#000000". It sat next to
// theme.AccountIcon, which IS themed — so on the dark page the recorded-narration
// row showed a pale person and the read-aloud row a black smudge.
//
// Its other use sites (the audio card's source chip and transport buttons) wrap
// it in theme.NewColoredResource themselves, which recolours whatever it is
// given, so they are unaffected by this wrapper.
var iconAudioWave fyne.Resource = func() fyne.Resource {
	if len(soundwaveIconSVG) == 0 {
		return theme.VolumeUpIcon()
	}
	return theme.NewThemedResource(fyne.NewStaticResource("soundwave.svg", soundwaveIconSVG))
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

// iconSidebarLeft is the iPad "toggle navigation sidebar" glyph (a rounded frame
// with a filled left column — the platform sidebar.left convention). Wrapped as a
// themed resource so it tints to the chrome foreground like the built-in header
// icons (the gear, etc.). Falls back to the hamburger MenuIcon if the asset is
// missing. Retained only for the former regular-width tablet header (ui.go),
// which the current classifier does not select.
var iconSidebarLeft fyne.Resource = func() fyne.Resource {
	if len(sidebarLeftSVG) == 0 {
		return theme.MenuIcon()
	}
	return theme.NewThemedResource(fyne.NewStaticResource("sidebar_left.svg", sidebarLeftSVG))
}()

// iconNoteBubble marks the shared-notes control: the same speech bubble a note
// is drawn as, so the button and the thing it opens read as one object. It is
// deliberately NOT a magnifier or a document — notes are a different corpus from
// the scripture the Search and Find controls beside it look through, and the
// control should say so at a glance. Falls back to a mail glyph if the asset is
// missing.
//
// WRAPPED AS A THEMED RESOURCE, and that is not decoration — it is the whole
// reason the glyph is visible at night. Every SVG in assets/icons is a single
// fill="#000000" path, and Fyne only ever recolours an icon it can recognise as
// a fyne.ThemedResource: widget/button.go tints one solely when the button's
// Importance is neither Medium nor Low, and a plain StaticResource is left
// exactly as authored in every case. So this rendered PURE BLACK on the dark
// rgb(25,23,21) page — very difficult to make out on the Search tab, where the
// control is deliberately LowImportance (flat) while inactive and so had no fill
// behind it either.
//
// Themed, both states come out right: while inactive Fyne leaves the resource to
// tint itself, and NewThemedResource paints it ColorNameForeground, which this
// theme maps to the palette's Text; while ACTIVE the button is HighImportance and
// Fyne recolours it to ForegroundOnPrimary, so it reads against the accent fill
// instead of disappearing into it.
//
// The same applies at its other use site, the note banner's "Show note" button
// (notes_banner.go), which is Medium importance and was equally black.
var iconNoteBubble fyne.Resource = func() fyne.Resource {
	if len(noteBubbleSVG) == 0 {
		return theme.MailComposeIcon()
	}
	return theme.NewThemedResource(fyne.NewStaticResource("note_bubble.svg", noteBubbleSVG))
}()

// iconFootnote is the translators'-footnotes glyph, RESERVED (owner-approved,
// not currently mounted — the feature's one control is the Settings checkbox,
// with no icon, by design): alpha and omega carrying a raised footnote
// numeral, in the word-processor insert-footnote lockup — the placeholder
// text is His letters, "I am the Alpha and the Omega" (Revelation 1:8), from
// the very book whose closing lines set this feature's standard. Baked from
// font outlines into a single-fill path. Themed like iconNoteBubble so that
// whenever it mounts on a widget.NewButtonWithIcon it is visible on the dark
// page and recolours against an active accent fill. Falls back to the
// generic list glyph if the asset is somehow missing.
var iconFootnote fyne.Resource = func() fyne.Resource {
	if len(footnoteIconSVG) == 0 {
		return theme.ListIcon()
	}
	return theme.NewThemedResource(fyne.NewStaticResource("footnote.svg", footnoteIconSVG))
}()
