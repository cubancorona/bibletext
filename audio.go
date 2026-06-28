package bibletext

// Per-chapter audio. Each chapter can be played two ways:
//
//   - RECORDED: a public-domain MP3 streamed from bible.helloao.org's sibling,
//     eBible.org (https://ebible.org/webaudio). These are World English Bible
//     recordings, so they line up only with WEB text — the WEB version and the
//     WEB-Catholic's 66 protocanonical books (same text). They stream with HTTP
//     range support, so the native player can seek (the ±15s skip).
//   - TTS: on-device text-to-speech of the chapter's own verses. Always available
//     and always matches the displayed version exactly (BSB, the deuterocanon, any
//     future translation, and the WEB chapters eBible hasn't recorded).
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

const ebibleAudioBase = "https://ebible.org/webaudio/"

// ebibleAudio is one book's eBible audio file naming: code is the book's file prefix,
// pad the chapter zero-pad width, and single marks a whole-book one-file recording with
// no chapter number (the tiny one-chapter epistles).
type ebibleAudio struct {
	code   string
	pad    int
	single bool
}

// ebibleAudioBooks is the set of books eBible streams clean per-chapter public-domain
// WEB recordings for (verified by probing https://ebible.org/webaudio). It is a partial,
// irregular subset of the canon — the rest falls back to TTS — and can grow as more
// clean sources are wired without touching anything else.
var ebibleAudioBooks = map[string]ebibleAudio{
	"Matthew":  {code: "Mat", pad: 2},
	"Mark":     {code: "Mark", pad: 2},
	"John":     {code: "John", pad: 2},
	"Romans":   {code: "Romans", pad: 2},
	"Hebrews":  {code: "Heb", pad: 2},
	"Psalms":   {code: "Psalm", pad: 3},
	"Philemon": {code: "Philemon", single: true},
	"Jude":     {code: "Jude", single: true},
	"2 John":   {code: "2John", single: true},
	"3 John":   {code: "3John", single: true},
}

// ebibleAudioURL returns the recorded WEB-audio URL for a book + chapter and whether one
// is mapped. The caller must also confirm the active version's text is the WEB (see
// versionUsesEBibleAudio) before treating it as a match.
func ebibleAudioURL(book string, chapter int) (string, bool) {
	e, ok := ebibleAudioBooks[book]
	if !ok {
		return "", false
	}
	if e.single {
		return ebibleAudioBase + e.code + ".mp3", true
	}
	return fmt.Sprintf("%s%s%0*d.mp3", ebibleAudioBase, e.code, e.pad, chapter), true
}

// versionUsesEBibleAudio reports whether a version's text is the World English Bible, so
// the eBible WEB recordings line up with it: the WEB itself and the WEB-Catholic (whose
// 66 protocanonical books are the same WEB text). The BSB is a different translation, and
// the deuterocanon isn't recorded — both take the TTS path.
func versionUsesEBibleAudio(versionID string) bool {
	return versionID == "web" || versionID == "webc"
}

// chapterHasRecording reports whether the current chapter has a recorded WEB MP3 (vs.
// TTS), so the reader can pick the right button icon.
func chapterHasRecording(state *AppState) bool {
	if state == nil || !versionUsesEBibleAudio(state.CurrentVersion) {
		return false
	}
	_, ok := ebibleAudioURL(state.CurrentBook, state.CurrentChapter)
	return ok
}

// audioForChapter resolves how to play the current chapter's audio.
func audioForChapter(state *AppState) chapterAudio {
	title := fmt.Sprintf("%s %d", state.CurrentBook, state.CurrentChapter)
	sub := state.CurrentVersion
	if v, ok := versionByID(state.CurrentVersion); ok {
		sub = v.Name
	}
	if chapterHasRecording(state) {
		url, _ := ebibleAudioURL(state.CurrentBook, state.CurrentChapter)
		return chapterAudio{Kind: audioRecorded, URL: url, Title: title, Subtitle: sub}
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
