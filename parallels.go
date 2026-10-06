package bibletext

// Gospel parallels (a synopsis / harmony of the four Gospels). For a verse that
// falls within a synopsis pericope, the SAME event as told by the other Gospels is
// offered in the cross-references panel, tagged as a "parallel" (distinct from the
// Treasury-of-Scripture-Knowledge cross-references in crossrefs.go). The dataset is
// embedded in the binary (no network, works offline), parsed once on first use.
//
// The next major release (docs/NEXT.md) adds a second kind, in a file of its
// own, gospel_occasions.json: the same saying on another occasion. The synopsis
// keeps apart what the harmonies treat as two occasions — Luke's Lord's Prayer
// is not the Sermon on the Mount's, nor his lament over Jerusalem the one
// Matthew sets in the Temple — and a pericope row would claim they are one
// event. So these are pairs of passages, each pair two occasions of one
// saying, never a pericope, and their rows say so (otherOccasionLabel). The
// pairings are taken from Stevens and Burton's harmony (1904), almost all from
// its table of sayings assigned to more than one occasion, and checked against
// Robertson's (1922); docs/TEXTUAL-DATA.md section 9 has the provenance. The
// same release places the twenty-two Gospel verses that are in no synopsis set
// (gospel_parallels_next.json). parallels_next.go embeds both files and puts
// them in place; without the next tag neither is in the binary.

import (
	_ "embed"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"sync"
)

//go:embed assets/parallels/gospel_parallels.json
var gospelParallelsJSON []byte

// gospelOccasionsJSON is gospel_occasions.json in the next major release
// (parallels_next.go), and nil in the shipping build, which lists no such rows.
var gospelOccasionsJSON []byte

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

// gPassage is one passage of one Gospel.
type gPassage struct {
	book  string
	spans []gSpan
}

// gOccasionGroup is one saying the Gospels record on more than one occasion:
// the passages that carry it, and the pairs of them that are two occasions.
// The pairs are explicit rather than every passage with every other because
// some passages of a group are ONE occasion — Matthew 13:9 and Mark 4:9 are
// the parable of the sower in both, already a synopsis parallel — and a row
// under the other-occasion label must not join those.
type gOccasionGroup struct {
	title    string
	passages []gPassage
	pairs    [][2]int // indexes into passages
}

var (
	occasionOnce   sync.Once
	occasionGroups []gOccasionGroup
)

// rawOccasions mirrors gospel_occasions.json (id, source and about are for
// the reader of the file; the panel reads the rest).
type rawOccasions struct {
	Groups []struct {
		ID       string      `json:"id"`
		Title    string      `json:"title"`
		Passages []string    `json:"passages"`
		Pairs    [][2]string `json:"pairs"`
	} `json:"groups"`
}

func loadGospelOccasions() {
	occasionGroups = parseGospelOccasions(gospelOccasionsJSON)
}

// parseGospelOccasions reads the other-occasion groups. A passage that does
// not parse, or a pair naming a passage the group does not list, is dropped
// rather than guessed at; a group left with no pair is dropped whole. The
// tests hold the embedded file to dropping nothing.
func parseGospelOccasions(data []byte) []gOccasionGroup {
	var raw rawOccasions
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	var out []gOccasionGroup
	for _, rg := range raw.Groups {
		g := gOccasionGroup{title: strings.TrimSpace(rg.Title)}
		index := map[string]int{}
		for _, ref := range rg.Passages {
			p, ok := parseGospelPassage(ref)
			if !ok {
				continue
			}
			index[ref] = len(g.passages)
			g.passages = append(g.passages, p)
		}
		for _, pr := range rg.Pairs {
			a, okA := index[pr[0]]
			b, okB := index[pr[1]]
			if okA && okB && a != b {
				g.pairs = append(g.pairs, [2]int{a, b})
			}
		}
		if len(g.pairs) > 0 {
			out = append(out, g)
		}
	}
	return out
}

// parseGospelPassage reads "Matthew 6:9-13": a Gospel's canonical name, then a
// reference in parseGospelRef's grammar.
func parseGospelPassage(s string) (gPassage, bool) {
	s = strings.TrimSpace(s)
	i := strings.LastIndexByte(s, ' ')
	if i <= 0 {
		return gPassage{}, false
	}
	book := s[:i]
	if !isGospelBook(book) {
		return gPassage{}, false
	}
	spans := parseGospelRef(s[i+1:])
	if len(spans) == 0 {
		return gPassage{}, false
	}
	return gPassage{book: book, spans: spans}, true
}

// gospelOccasionsForVerse returns the passages that carry the same saying as
// the verse on another occasion, each tagged Parallel and OtherOccasion and
// carrying the saying's title. The verse's own Gospel is NOT skipped, as the
// synopsis skips it: Luke tells the lamp under a basket at 8:16 and again at
// 11:33, and that pair is the point. Only a passage holding the verse itself
// is left out. Within a group the passages come in Gospel order; groups come
// in file order. Numbered in the reference (WEB); the caller maps them. None
// in the shipping build, which has no such data.
func gospelOccasionsForVerse(book string, ch, v int) []crossRef {
	occasionOnce.Do(loadGospelOccasions)
	if len(occasionGroups) == 0 || !isGospelBook(book) {
		return nil
	}
	var out []crossRef
	for i := range occasionGroups {
		g := &occasionGroups[i]
		var partners []int
		taken := map[int]bool{}
		for _, pr := range g.pairs {
			for side := 0; side < 2; side++ {
				here, there := g.passages[pr[side]], g.passages[pr[1-side]]
				if here.book != book || !spansContain(here.spans, ch, v) {
					continue
				}
				if there.book == book && spansContain(there.spans, ch, v) {
					continue
				}
				if !taken[pr[1-side]] {
					taken[pr[1-side]] = true
					partners = append(partners, pr[1-side])
				}
			}
		}
		sort.SliceStable(partners, func(a, b int) bool {
			return passageBefore(g.passages[partners[a]], g.passages[partners[b]])
		})
		for _, p := range partners {
			for _, s := range g.passages[p].spans {
				c := spanToCrossRef(g.passages[p].book, s, g.title)
				c.OtherOccasion = true
				out = append(out, c)
			}
		}
	}
	return out
}

// passageBefore orders passages by Gospel, then by where they start.
func passageBefore(a, b gPassage) bool {
	if ga, gb := gospelOrder(a.book), gospelOrder(b.book); ga != gb {
		return ga < gb
	}
	sa, sb := a.spans[0], b.spans[0]
	return sa.ch1 < sb.ch1 || (sa.ch1 == sb.ch1 && sa.v1 < sb.v1)
}

func gospelOrder(book string) int {
	for i, g := range gospelColumns {
		if g.book == book {
			return i
		}
	}
	return len(gospelColumns)
}
