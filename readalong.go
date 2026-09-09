package bibletext

// Read-along timing tables: per-chapter verse start/end times from forced alignment
// of the recorded narration (scripts/audio-align/). Drives highlighting the verse
// being narrated + auto-scroll. Bundled tiny (~0.5 MB per translation). A titled
// psalm's table leads with a verse-0 row for its superscription, which the
// narrators read before verse 1 — see verseTiming.
//
// Tables are keyed by RECORDING id (see recordingsFor in audio.go): timings are
// aligned against a specific recording's exact audio bytes on the project's own
// host, so they belong to the recording, not the translation. All three recordings
// the app streams are covered — "bsb-hays" (Barry Hays), "web-williams" (David
// Williams; also serves the WEB-Catholic's protocanon, the same WEB text) and
// "webbe-synthetic" (eBible.org's synthetic narration of the WEB-Catholic's Greek
// books). chapterTimings returns nil for anything without a bundled table, so those
// chapters simply don't highlight while recorded audio still plays.

import (
	_ "embed"
	"encoding/json"
	"strconv"
	"sync"
)

//go:embed assets/timings/bsb.json
var bsbTimingsJSON []byte

//go:embed assets/timings/web.json
var webTimingsJSON []byte

//go:embed assets/timings/webbe.json
var webbeTimingsJSON []byte

// verseTiming is one row of a chapter's table: the span (seconds) of one
// narrated unit within the recording. Verse 0 is the Psalm's superscription —
// present only for the titled psalms, in tables built by the title-aware
// alignment; a table without verse-0 rows is still valid and simply never
// reports the title. The TTS path reuses the struct with start holding a
// UTF-16 offset instead of a time (speechVerseOffsets).
type verseTiming struct {
	verse      int
	start, end float64
}

// The read-along's three states, on ONE wire from the model through the
// controller to every renderer: readAlongNone is "nothing narrated" (the
// recording's intro, or a seek back before the first row), readAlongTitle is
// the Psalm's superscription, and n >= 1 is verse n. Zero used to mean none,
// which is why the sentinel is negative: the title needed the one slot no
// verse number can collide with. The native panes carry the same values as
// kBTReadAlongNone/kBTReadAlongTitle (Apple) and RA_NONE/RA_TITLE (Android);
// readalong_title_native_contract_test.go holds them equal.
const (
	readAlongNone  = -1
	readAlongTitle = 0
)

var (
	timingsOnce sync.Once
	allTimings  map[string]map[string]map[string][]verseTiming // recording id -> book -> chapter(str) -> verses
)

func loadTimings() {
	timingsOnce.Do(func() {
		allTimings = make(map[string]map[string]map[string][]verseTiming, 3)
		for recID, blob := range map[string][]byte{"bsb-hays": bsbTimingsJSON, "web-williams": webTimingsJSON, webbeRecordingID: webbeTimingsJSON} {
			books, err := parseTimings(blob)
			if err != nil {
				continue // a recording whose table will not parse simply never highlights
			}
			allTimings[recID] = books
		}
	})
}

// parseTimings decodes one recording's compact table, {book: {chapter:
// [[verse, start, end], ...]}}. A row that is not exactly three numbers is
// dropped; a verse-0 row (the title) is kept like any other.
func parseTimings(blob []byte) (map[string]map[string][]verseTiming, error) {
	var raw map[string]map[string][][]float64
	if err := json.Unmarshal(blob, &raw); err != nil {
		return nil, err
	}
	books := make(map[string]map[string][]verseTiming, len(raw))
	for book, chs := range raw {
		m := make(map[string][]verseTiming, len(chs))
		for ch, rows := range chs {
			vs := make([]verseTiming, 0, len(rows))
			for _, r := range rows {
				if len(r) == 3 {
					vs = append(vs, verseTiming{int(r[0]), r[1], r[2]})
				}
			}
			m[ch] = vs
		}
		books[book] = m
	}
	return books, nil
}

// chapterTimings returns a recording's verse timing table for a chapter (sorted by
// start), or nil when that recording has no bundled timings.
func chapterTimings(recordingID, book string, chapter int) []verseTiming {
	loadTimings()
	if m, ok := allTimings[recordingID]; ok {
		if b, ok := m[book]; ok {
			return b[strconv.Itoa(chapter)]
		}
	}
	return nil
}

// recordingHasChapter reports whether a recording actually has an MP3 for a
// chapter, by consulting its bundled timing table. The tables were force-aligned
// against the released audio files themselves (66 books / 1189 chapters each), so
// they are the authority on which chapters were recorded — no hand-written
// per-book chapter count to drift. This is what keeps the urlFor builders from
// offering chapters past a book's recorded end, e.g. the WEB-Catholic's Greek
// Daniel 13–14 (rendered chapters the WEB narration doesn't have).
func recordingHasChapter(recordingID, book string, chapter int) bool {
	return len(chapterTimings(recordingID, book, chapter)) > 0
}

// verseAtTime returns the row being narrated at time t: the last row whose start
// is at or before t. Returns readAlongNone before the first row begins (the
// recording's intro) and for an empty table, so nothing is highlighted until
// the narrator actually reaches the first row — the title in a titled psalm,
// verse 1 everywhere else.
func verseAtTime(vs []verseTiming, t float64) int {
	v := readAlongNone
	for _, vt := range vs {
		if vt.start <= t {
			v = vt.verse
		} else {
			break
		}
	}
	return v
}
