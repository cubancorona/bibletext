package bibletext

// Read-along timing tables: per-chapter verse start/end times from forced alignment
// of the recorded narration (scripts/audio-align/). Drives highlighting the verse
// being narrated + auto-scroll. Bundled tiny (~0.5 MB per translation).
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

// verseTiming is one verse's span within its chapter's recording (seconds).
type verseTiming struct {
	verse      int
	start, end float64
}

var (
	timingsOnce sync.Once
	allTimings  map[string]map[string]map[string][]verseTiming // recording id -> book -> chapter(str) -> verses
)

func loadTimings() {
	timingsOnce.Do(func() {
		allTimings = make(map[string]map[string]map[string][]verseTiming, 3)
		for recID, blob := range map[string][]byte{"bsb-hays": bsbTimingsJSON, "web-williams": webTimingsJSON, webbeRecordingID: webbeTimingsJSON} {
			var raw map[string]map[string][][]float64 // [[verse,start,end], ...]
			if err := json.Unmarshal(blob, &raw); err != nil {
				continue
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
			allTimings[recID] = books
		}
	})
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

// verseAtTime returns the verse being narrated at time t: the last verse whose start
// is at or before t. Returns 0 before the first verse begins (the recording's intro),
// so nothing is highlighted until the reader actually reaches verse 1.
func verseAtTime(vs []verseTiming, t float64) int {
	v := 0
	for _, vt := range vs {
		if vt.start <= t {
			v = vt.verse
		} else {
			break
		}
	}
	return v
}
