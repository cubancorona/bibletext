package bibletext

// The reader's per-chapter listen control. It lives in the shared header builders
// (reading.go chapterHeader + reading_ios.go chapterHeaderMobile), so the glyph is
// recomputed on every navigation. It's plain Fyne chrome above the native text
// overlay's frame, so it's never occluded and needs no hide/restore dance.
//
// Two decoupled signals (so neither ever disappears):
//   - an accent-filled play/pause button — the one obviously-tappable control;
//   - a flat, NON-interactive source tag beside it — headphones for a streamed
//     recording, the voice glyph for read-aloud — persistent across play/pause.

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
)

// audioButton builds the [play/pause] + [source tag] cluster. It installs
// gAudio.onChange so a native state change (chapter finished, a phone-call
// interruption, or a lock-screen / Control Center toggle) re-renders the button —
// refreshReadingOnly rebuilds the cheap Fyne header without re-pushing chapter
// HTML to the overlay (the push path skips on an unchanged chapterRenderFingerprint,
// which an audio toggle never changes).
func audioButton(state *AppState, boxH float32) fyne.CanvasObject {
	playing, _ := gAudio.buttonState(chapterAudioFingerprint(state))
	glyph := theme.MediaPlayIcon()
	if playing {
		glyph = theme.MediaPauseIcon()
	}
	play := newIconTapButton(state, glyph, 15, boxH, func() {
		gAudio.playPauseCurrent(state)
	})
	play.filled = true
	gAudio.setOnChange(func() { state.refreshReadingOnly() })

	return container.NewHBox(play, hgap(3), audioSourceTag(audioSourceIcon(state), 15, boxH))
}

// audioSourceIcon is the persistent source glyph for the current chapter:
// headphones for a streamed recording, the voice glyph for read-aloud. It uses the
// cheap chapterHasRecording lookup (NOT audioForChapter, which would build the whole
// chapter's speech text) and never changes with play/pause state.
func audioSourceIcon(state *AppState) fyne.Resource {
	if chapterHasRecording(state) {
		return iconHeadphones
	}
	return iconSpeak
}

// audioSourceTag renders the source glyph flat and muted, and — deliberately — NOT
// wrapped in any Tappable widget, so a tap passes through it. Only the accent play
// control reads as pressable; this is visibly a label, not a button.
func audioSourceTag(res fyne.Resource, size, boxH float32) fyne.CanvasObject {
	img := canvas.NewImageFromResource(theme.NewColoredResource(res, colorNameMuted))
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(size, size))
	return container.NewGridWrap(fyne.NewSize(size+6, boxH), container.NewCenter(img))
}
