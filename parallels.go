package bibletext

// Gospel parallels (a synopsis / harmony of the four Gospels). For a verse that
// falls within a synopsis pericope, the SAME event as told by the other Gospels is
// offered in the cross-references panel, tagged as a "parallel" (distinct from the
// Treasury-of-Scripture-Knowledge cross-references in crossrefs.go). The dataset is
// embedded in the binary (no network, works offline), parsed once on first use.

import (
	_ "embed"
	"encoding/json"
	"strconv"
	"strings"
	"sync"
)

//go:embed assets/parallels/gospel_parallels.json
var gospelParallelsJSON []byte

// gospelColumns are the four Gospels in canonical (synopsis-column) order, mapping
// the JSON's lowercase keys to the app's canonical book names.
var gospelColumns = []struct{ key, book string }{
	{"matthew", "Matthew"}, {"mark", "Mark"}, {"luke", "Luke"}, {"john", "John"},
}

// gSpan is a contiguous passage span within one Gospel: a chapter:verse start to a
// chapter:verse end (end == start for a single verse; ch2 > ch1 for a cross-chapter
// range like Mark 8:34-9:1).
type gSpan struct{ ch1, v1, ch2, v2 int }

func (s gSpan) contains(ch, v int) bool {
	afterStart := ch > s.ch1 || (ch == s.ch1 && v >= s.v1)
	beforeEnd := ch < s.ch2 || (ch == s.ch2 && v <= s.v2)
	return afterStart && beforeEnd
}

// gPericope is one synopsis row: a titled event with each Gospel's passage spans.
type gPericope struct {
	title string
	spans map[string][]gSpan // canonical book name -> spans (absent if that Gospel has none)
}

var (
	gospelOnce      sync.Once
	gospelPericopes []gPericope
)

// rawPericope mirrors the embedded JSON (id/section are ignored).
type rawPericope struct {
	Title string `json:"title"`
	Refs  struct {
		Matthew *string `json:"matthew"`
		Mark    *string `json:"mark"`
		Luke    *string `json:"luke"`
		John    *string `json:"john"`
	} `json:"refs"`
}

func loadGospelParallels() {
	var raw []rawPericope
	if err := json.Unmarshal(gospelParallelsJSON, &raw); err != nil {
		return // leave empty; parallels simply won't appear
	}
	out := make([]gPericope, 0, len(raw))
	for _, r := range raw {
		p := gPericope{title: strings.TrimSpace(r.Title), spans: map[string][]gSpan{}}
		add := func(book string, ref *string) {
			if ref == nil {
				return
			}
			if spans := parseGospelRef(*ref); len(spans) > 0 {
				p.spans[book] = spans
			}
		}
		add("Matthew", r.Refs.Matthew)
		add("Mark", r.Refs.Mark)
		add("Luke", r.Refs.Luke)
		add("John", r.Refs.John)
		if len(p.spans) > 0 {
			out = append(out, p)
		}
	}
	gospelPericopes = out
}

// parseGospelRef parses a synopsis ref string into contiguous spans. It handles
// every shape in the dataset:
//
//	"1:1-4"            one-chapter range
//	"1:14a"           single verse (the a/b verse-part letter is ignored)
//	"8:34-9:1"        cross-chapter range
//	"6:27-28,32-36"   comma-separated sub-ranges; a sub-range with no colon
//	                  inherits the chapter from the one before it
func parseGospelRef(s string) []gSpan {
	var spans []gSpan
	curCh := 0
	for _, seg := range strings.Split(s, ",") {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		startPart, endPart := seg, ""
		if i := strings.IndexByte(seg, '-'); i >= 0 {
			startPart, endPart = seg[:i], seg[i+1:]
		}
		s1ch, s1v, ok := parseChV(startPart, curCh)
		if !ok {
			continue
		}
		curCh = s1ch
		e1ch, e1v := s1ch, s1v
		if endPart != "" {
			if ec, ev, ok := parseChV(endPart, s1ch); ok {
				e1ch, e1v = ec, ev
				curCh = ec
			}
		}
		spans = append(spans, gSpan{s1ch, s1v, e1ch, e1v})
	}
	return spans
}

// parseChV parses "C:V" or a bare "V" (using fallbackCh for the latter), ignoring a
// trailing a/b verse-part letter on the verse.
func parseChV(s string, fallbackCh int) (ch, v int, ok bool) {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, ':'); i >= 0 {
		c, err := strconv.Atoi(strings.TrimSpace(s[:i]))
		if err != nil {
			return 0, 0, false
		}
		vv, ok2 := parseVerseNum(s[i+1:])
		if !ok2 {
			return 0, 0, false
		}
		return c, vv, true
	}
	if fallbackCh == 0 {
		return 0, 0, false
	}
	vv, ok2 := parseVerseNum(s)
	if !ok2 {
		return 0, 0, false
	}
	return fallbackCh, vv, true
}

func parseVerseNum(s string) (int, bool) {
	s = strings.TrimRight(strings.TrimSpace(s), "ab") // ignore verse-part letters (54a, 6b)
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return n, true
}

// gospelParallelsForVerse returns the parallel passages (the SAME event in the other
// Gospels) for a verse that falls within a synopsis pericope, each tagged as a
// parallel and carrying the pericope title. Empty for non-Gospel verses, and for
// Gospel verses with no recorded parallel. Book names are canonical; the caller
// resolves them against the loaded translation (as with TSK cross-references).
func gospelParallelsForVerse(book string, ch, v int) []crossRef {
	gospelOnce.Do(loadGospelParallels)
	if len(gospelPericopes) == 0 || !isGospelBook(book) {
		return nil
	}
	var out []crossRef
	for i := range gospelPericopes {
		p := &gospelPericopes[i]
		if !spansContain(p.spans[book], ch, v) {
			continue
		}
		for _, g := range gospelColumns {
			if g.book == book {
				continue
			}
			for _, s := range p.spans[g.book] {
				out = append(out, spanToCrossRef(g.book, s, p.title))
			}
		}
	}
	return out
}

func isGospelBook(book string) bool {
	for _, g := range gospelColumns {
		if g.book == book {
			return true
		}
	}
	return false
}

func spansContain(spans []gSpan, ch, v int) bool {
	for _, s := range spans {
		if s.contains(ch, v) {
			return true
		}
	}
	return false
}

func spanToCrossRef(book string, s gSpan, title string) crossRef {
	c := crossRef{Book: book, Chapter: s.ch1, Verse: s.v1, Parallel: true, Title: title}
	if s.ch2 != s.ch1 || s.v2 != s.v1 {
		c.EndCh, c.EndV = s.ch2, s.v2
	}
	return c
}
