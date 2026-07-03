package bibletext

// Per-chapter audio. Each chapter can be played two ways:
//
//   - RECORDED: a public-domain MP3 streamed from the project's own audio host
//     (github.com/cubancorona/bibletext-audio, GitHub release assets with HTTP
//     range support, so the native player can seek — the ±15s skip). Each
//     translation plays a narration made from its own text: the BSB (Barry Hays)
//     and the WEB (David Williams) are both complete. Self-hosting pins the exact
//     audio bytes the bundled read-along verse timings were aligned against, so
//     the highlight can never drift out of sync with the recording.
//   - TTS: on-device text-to-speech of the chapter's own verses. Always available
//     and always matches the displayed version exactly (the deuterocanon and any
//     future translation without a recording).
//
// audioForChapter resolves which applies; the reader shows a play icon for recorded
// audio and a "voice" icon for TTS. The native players (AVPlayer / AVSpeechSynthesizer
// + AVAudioSession + MPNowPlayingInfoCenter + MPRemoteCommandCenter) live in the
// per-platform cgo files and are driven from this data.

import (
	"fmt"
	"strings"
)

// audioKind distinguishes a streamed recording from on-device text-to-speech.
type audioKind int

const (
	audioRecorded audioKind = iota
	audioTTS
)

// chapterAudio is everything needed to play one chapter. Kind selects the player:
// recorded streams URL (seekable); TTS speaks Text. Title + Subtitle feed the
// lock-screen / Control Center Now Playing info.
type chapterAudio struct {
	Kind     audioKind
	URL      string // recorded: the MP3 URL
	Text     string // TTS: the text to speak
	Title    string // "John 20"
	Subtitle string // version name, e.g. "World English Bible"
}

// audioHostBase is the project's own audio host: GitHub release assets on the
// companion bibletext-audio repo (public domain MP3s, HTTP range-seekable). Each
// recording is split across TWO releases — Old Testament (929 chapters) and New
// Testament (260) — because GitHub caps a release at 1000 assets; audioReleaseTag
// picks the right one by canonical book number.
const audioHostBase = "https://github.com/cubancorona/bibletext-audio/releases/download/"

// audioReleaseTag returns the release tag holding a book's recording: <corpus>-ot-v1
// for books 1–39, <corpus>-nt-v1 for 40–66.
func audioReleaseTag(corpus string, bookNum int) string {
	if bookNum >= 40 {
		return corpus + "-nt-v1"
	}
	return corpus + "-ot-v1"
}

// webAudioURL returns the WEB recorded-narration MP3 URL for a book + chapter and
// whether one is mapped (all 66 canonical books are — a COMPLETE public-domain
// narration by David Williams, via audiotreasure.com; it replaced the partial
// eBible.org set the app launched with, so no WEB chapter falls back to TTS
// anymore). The caller must also confirm the active version's text is the WEB (see
// versionUsesWEBAudio) before treating it as a match. File scheme:
// WEB_{book:02d}_{chapter:03d}.mp3, e.g. WEB_43_003.mp3 (John 3) — the canonical
// 1–66 numbering shared with the BSB set, so no per-book naming table is needed.
func webAudioURL(book string, chapter int) (string, bool) {
	b, ok := bsbAudioBooks[book]
	if !ok || chapter < 1 {
		return "", false
	}
	return fmt.Sprintf("%s%s/WEB_%02d_%03d.mp3", audioHostBase, audioReleaseTag("web-williams", b.num), b.num, chapter), true
}

// versionUsesWEBAudio reports whether a version's text is the World English Bible, so
// the Williams WEB narration lines up with it: the WEB itself and the WEB-Catholic
// (whose 66 protocanonical books are the same WEB text). The BSB is a different
// translation, and the deuterocanon isn't recorded — both take the TTS path.
func versionUsesWEBAudio(versionID string) bool {
	return versionID == "web" || versionID == "webc"
}

// recordedURLFor returns the recorded-narration MP3 URL for the current chapter
// and whether one exists, dispatching by translation so each version plays a
// recording made from its own text:
//   - BSB: a COMPLETE CC0 narration (Barry Hays) — all 66 books.
//   - WEB / WEB-Catholic: a COMPLETE public-domain narration (David Williams) —
//     all 66 books (the deuterocanon isn't recorded; it falls back to TTS).
//
// Any other version has no matching recording and uses TTS.
func recordedURLFor(state *AppState) (string, bool) {
	if state == nil {
		return "", false
	}
	if state.CurrentVersion == "bsb" {
		return bsbAudioURL(state.CurrentBook, state.CurrentChapter)
	}
	if versionUsesWEBAudio(state.CurrentVersion) {
		return webAudioURL(state.CurrentBook, state.CurrentChapter)
	}
	return "", false
}

// chapterHasRecording reports whether the current chapter has a recorded MP3 (vs.
// TTS), so the reader can pick the right source glyph.
func chapterHasRecording(state *AppState) bool {
	_, ok := recordedURLFor(state)
	return ok
}

// audioForChapter resolves how to play the current chapter's audio.
func audioForChapter(state *AppState) chapterAudio {
	title := fmt.Sprintf("%s %d", state.CurrentBook, state.CurrentChapter)
	sub := state.CurrentVersion
	if v, ok := versionByID(state.CurrentVersion); ok {
		sub = v.Name
	}
	if url, ok := recordedURLFor(state); ok {
		return chapterAudio{Kind: audioRecorded, URL: url, Title: title, Subtitle: sub}
	}
	return chapterAudio{Kind: audioTTS, Text: chapterSpeechText(state), Title: title, Subtitle: sub}
}

// ttsAudioForChapter forces a text-to-speech rendering of the current chapter
// regardless of whether a recording exists — used when the reader picks "Read
// aloud" from the source menu on a chapter that also has a recording.
func ttsAudioForChapter(state *AppState) chapterAudio {
	title := fmt.Sprintf("%s %d", state.CurrentBook, state.CurrentChapter)
	sub := state.CurrentVersion
	if v, ok := versionByID(state.CurrentVersion); ok {
		sub = v.Name
	}
	return chapterAudio{Kind: audioTTS, Text: chapterSpeechText(state), Title: title, Subtitle: sub}
}

// chapterSpeechText is the plain text fed to TTS: the current chapter's verses in order,
// joined into flowing prose (no spoken verse numbers), matching what's on screen.
func chapterSpeechText(state *AppState) string {
	if state == nil || state.Bible == nil {
		return ""
	}
	var b strings.Builder
	for _, v := range state.Bible.Verses[state.CurrentBook][state.CurrentChapter] {
		t := strings.TrimSpace(v.Text)
		if t == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(t)
	}
	return b.String()
}

// chapterAudioFingerprint identifies the audio appropriate for the reader's
// current position: version + book + chapter. Theme, highlight and red-letter
// (which chapterRenderFingerprint folds in) don't change the audio, so they're
// deliberately excluded — a light/dark flip must NOT count as "the chapter
// changed" and stop playback. Used to tell whether the loaded audio still matches
// where the reader is.
func chapterAudioFingerprint(state *AppState) string {
	if state == nil {
		return ""
	}
	return fmt.Sprintf("%s|%s|%d", state.CurrentVersion, state.CurrentBook, state.CurrentChapter)
}
