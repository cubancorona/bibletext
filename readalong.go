package bibletext

// Read-along timing tables: per-chapter verse start/end times from forced alignment
// of the recorded narration (scripts/audio-align/). Drives highlighting the verse
// being narrated + auto-scroll. Bundled tiny (~0.5 MB per translation).
//
// Both complete recordings the app streams are covered: the BSB (Barry Hays) and
// the WEB (David Williams) — the tables were aligned against the exact audio bytes
// on the project's own host (see audio.go), so they can't drift. The WEB-Catholic
// shares the WEB tables for its 66 protocanonical books (same text, same verse
// numbers); the deuterocanon has no recording. chapterTimings returns nil for
// anything without a bundled table, so those chapters simply don't highlight
// (recorded audio still plays; TTS read-along is a separate, timing-free path via
// the speech synthesizer's word callback).

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

// verseTiming is one verse's span within its chapter's recording (seconds).
type verseTiming struct {
	verse      int
	start, end float64
}

var (
	timingsOnce sync.Once
	allTimings  map[string]map[string]map[string][]verseTiming // version -> book -> chapter(str) -> verses
)

func loadTimings() {
	timingsOnce.Do(func() {
		allTimings = make(map[string]map[string]map[string][]verseTiming, 2)
		for version, blob := range map[string][]byte{"bsb": bsbTimingsJSON, "web": webTimingsJSON} {
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
			allTimings[version] = books
		}
	})
}

// chapterTimings returns the verse timing table for a chapter (sorted by start), or
// nil when the version's recording has no bundled timings.
func chapterTimings(version, book string, chapter int) []verseTiming {
	if version == "webc" {
		version = "web" // the WEB-Catholic's 66 recorded books are the same WEB text
	}
	loadTimings()
	if m, ok := allTimings[version]; ok {
		if b, ok := m[book]; ok {
			return b[strconv.Itoa(chapter)]
		}
	}
	return nil
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
