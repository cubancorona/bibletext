package bibletext

// Read-along timing tables: per-chapter verse start/end times from forced alignment
// of the recorded narration (scripts/audio-align/). Drives highlighting the verse
// being narrated + auto-scroll. Bundled tiny (~0.5 MB for the whole Bible).
//
// Only the BSB (Barry Hays / openbible) recording is bundled today, because that's
// the recording the app streams. The complete WEB (David Williams) recording is
// aligned too, but we'd host that audio ourselves, so its timings ship once hosting
// lands — chapterTimings just returns nil for versions without a bundled table, so
// those chapters simply don't highlight (recorded audio still plays; TTS read-along
// is a separate, timing-free path via the speech synthesizer's word callback).

import (
	_ "embed"
	"encoding/json"
	"strconv"
	"sync"
)

//go:embed assets/timings/bsb.json
var bsbTimingsJSON []byte

// verseTiming is one verse's span within its chapter's recording (seconds).
type verseTiming struct {
	verse      int
	start, end float64
}

var (
	timingsOnce sync.Once
	bsbTimings  map[string]map[string][]verseTiming // book -> chapter(str) -> verses
)

func loadTimings() {
	timingsOnce.Do(func() {
		var raw map[string]map[string][][]float64 // [[verse,start,end], ...]
		if err := json.Unmarshal(bsbTimingsJSON, &raw); err != nil {
			return
		}
		bsbTimings = make(map[string]map[string][]verseTiming, len(raw))
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
			bsbTimings[book] = m
		}
	})
}

// chapterTimings returns the verse timing table for a chapter (sorted by start), or
// nil when the version's recording has no bundled timings.
func chapterTimings(version, book string, chapter int) []verseTiming {
	if version != "bsb" {
		return nil
	}
	loadTimings()
	if bsbTimings == nil {
		return nil
	}
	if m, ok := bsbTimings[book]; ok {
		return m[strconv.Itoa(chapter)]
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
