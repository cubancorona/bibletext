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
	Kind        audioKind
	RecordingID string // recorded: which recording URL came from (timing-table key)
	URL         string // recorded: the MP3 URL
	Text        string // TTS: the text to speak
	Title       string // "John 20"
	Subtitle    string // e.g. "World English Bible · David Williams"
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
// anymore). Registered per version in recordingsFor. File scheme:
// WEB_{book:02d}_{chapter:03d}.mp3, e.g. WEB_43_003.mp3 (John 3) — the canonical
// 1–66 numbering shared with the BSB set, so no per-book naming table is needed.
func webAudioURL(book string, chapter int) (string, bool) {
	b, ok := bsbAudioBooks[book]
	if !ok || chapter < 1 {
		return "", false
	}
	return fmt.Sprintf("%s%s/WEB_%02d_%03d.mp3", audioHostBase, audioReleaseTag("web-williams", b.num), b.num, chapter), true
}

// recording is one named narration of a translation: id keys the bundled
// read-along timing tables (timings are aligned against a specific recording's
// exact audio bytes, so they belong to the recording, not the version), narrator
// is the display name for the source menu and Now Playing, and urlFor maps a
// chapter to its MP3 (reporting false where the recording has no file — the
// deuterocanon, out-of-range chapters).
type recording struct {
	id       string // "bsb-hays" — also the release-tag prefix on the audio host
	narrator string // "Barry Hays"
	urlFor   func(book string, chapter int) (string, bool)
}

// recordingsFor lists the narrations whose text matches a version, in preference
// order (the first is the default). Each version currently has exactly one; adding
// a narrator here is all it takes for it to appear as a source-menu row:
//   - BSB: complete CC0 narration by Barry Hays.
//   - WEB / WEB-Catholic: complete public-domain narration by David Williams (the
//     WEB-Catholic's 66 protocanonical books are the same WEB text; the deuterocanon
//     isn't recorded and its chapters fall back to TTS via urlFor).
//
// Any other version has no matching recording and uses TTS.
func recordingsFor(versionID string) []recording {
	switch versionID {
	case "bsb":
		return []recording{{id: "bsb-hays", narrator: "Barry Hays", urlFor: bsbAudioURL}}
	case "web", "webc":
		return []recording{{id: "web-williams", narrator: "David Williams", urlFor: webAudioURL}}
	}
	return nil
}

// chapterRecordings returns the recordings that actually have the current chapter
// (a version's recordings minus the books/chapters they don't cover).
func chapterRecordings(state *AppState) []recording {
	if state == nil {
		return nil
	}
	var out []recording
	for _, r := range recordingsFor(state.CurrentVersion) {
		if _, ok := r.urlFor(state.CurrentBook, state.CurrentChapter); ok {
			out = append(out, r)
		}
	}
	return out
}

// recordingByID resolves a version's recording by id — used to validate a
// remembered preference against the recordings the version actually has.
func recordingByID(versionID, recID string) (recording, bool) {
	for _, r := range recordingsFor(versionID) {
		if r.id == recID {
			return r, true
		}
	}
	return recording{}, false
}

// chapterHasRecording reports whether the current chapter has a recorded MP3 (vs.
// TTS), so the reader can pick the right source glyph.
func chapterHasRecording(state *AppState) bool {
	return len(chapterRecordings(state)) > 0
}

// audioForRecording resolves the current chapter as a specific recording, falling
// back to TTS when that recording doesn't cover the chapter. The Subtitle credits
// the narrator on the lock screen / Control Center.
func audioForRecording(state *AppState, rec recording) chapterAudio {
	title := fmt.Sprintf("%s %d", state.CurrentBook, state.CurrentChapter)
	sub := state.CurrentVersion
	if v, ok := versionByID(state.CurrentVersion); ok {
		sub = v.Name
	}
	if url, ok := rec.urlFor(state.CurrentBook, state.CurrentChapter); ok {
		return chapterAudio{Kind: audioRecorded, RecordingID: rec.id, URL: url,
			Title: title, Subtitle: sub + " · " + rec.narrator}
	}
	return chapterAudio{Kind: audioTTS, Text: chapterSpeechText(state), Title: title, Subtitle: sub}
}

// audioForChapter resolves how to play the current chapter's audio with the
// version's default (first-listed) recording.
func audioForChapter(state *AppState) chapterAudio {
	if recs := chapterRecordings(state); len(recs) > 0 {
		return audioForRecording(state, recs[0])
	}
	title := fmt.Sprintf("%s %d", state.CurrentBook, state.CurrentChapter)
	sub := state.CurrentVersion
	if v, ok := versionByID(state.CurrentVersion); ok {
		sub = v.Name
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

// speechVerseOffsets mirrors chapterSpeechText's construction exactly (same trim,
// same skip-empty, same single-space separator) and returns where each spoken
// verse STARTS within the utterance — in UTF-16 code units, because that's the
// unit of the NSRange the speech synthesizer's willSpeakRangeOfSpeechString
// callback reports against the NSString it was given. Reuses verseTiming with
// start holding the offset, so the recorded path's verseAtTime lookup works
// unchanged for TTS read-along.
func speechVerseOffsets(state *AppState) []verseTiming {
	if state == nil || state.Bible == nil {
		return nil
	}
	var out []verseTiming
	off := 0
	for _, v := range state.Bible.Verses[state.CurrentBook][state.CurrentChapter] {
		t := strings.TrimSpace(v.Text)
		if t == "" {
			continue
		}
		if len(out) > 0 {
			off++ // the joining space
		}
		out = append(out, verseTiming{verse: v.Verse, start: float64(off)})
		off += utf16Len(t)
	}
	return out
}

// utf16Len is a string's length in UTF-16 code units (BMP rune = 1, astral = 2) —
// how NSString counts, and therefore how speech ranges are indexed.
func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		if r > 0xFFFF {
			n += 2
		} else {
			n++
		}
	}
	return n
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
