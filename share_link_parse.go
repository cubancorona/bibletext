package bibletext

// The inverse of ShareLinkURL: turn a shared link back into a passage.
//
// This is what a deep link (iOS Universal Link / Android App Link) hands the
// app when someone taps a shared verse on a phone that has BibleText installed.
// It is deliberately a pure function with no platform code, so the parsing
// rules are testable and identical everywhere the link can arrive.
//
// It is FORGIVING BY DESIGN. A link may be years old, may have been mangled by
// a messenger, or may name a chapter this reader's translation doesn't have.
// The rule is: extract as much as is unambiguous, and never fail in a way that
// leaves the reader worse off than tapping a normal link. Callers treat a
// false result as "let the web page handle it".

import (
	"strconv"
	"strings"
)

// ShareTarget is a passage named by a shared link.
type ShareTarget struct {
	VersionID string // always one of the published ids: web, bsb, webc
	Book      string // canonical book name, e.g. "1 Corinthians"
	Chapter   int    // >= 1
	VerseLo   int    // 0 when the link names no verse (a chapter link)
	VerseHi   int    // 0 or == VerseLo for a single verse
}

// ParseShareLink parses a BibleText web-reader URL into the passage it names.
// ok is false for anything that is not one of our reader links — including the
// landing, privacy and support pages, which must always stay in the browser.
//
// Accepts (all of these appear in the wild):
//   - the canonical form   https://bibletext.co.uk/web/john/3/#v16-18
//   - no trailing slash    https://bibletext.co.uk/web/john/3#v16
//   - the ?v= alias        https://bibletext.co.uk/web/john/3/?v=16-18
//   - http://, www., and any capitalisation a messenger introduced
func ParseShareLink(raw string) (ShareTarget, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ShareTarget{}, false
	}

	// Split off the fragment and query before touching the path: the verse may
	// live in either, and neither is part of the path grammar.
	var frag, query string
	if i := strings.IndexByte(s, '#'); i >= 0 {
		frag, s = s[i+1:], s[:i]
	}
	if i := strings.IndexByte(s, '?'); i >= 0 {
		query, s = s[i+1:], s[:i]
	}

	// Strip the scheme and host. Hosts are case-insensitive; paths on GitHub
	// Pages are not, but a reader who hand-types "John" deserves to land
	// somewhere sensible, so the path is lowercased too.
	s = strings.ToLower(s)
	for _, p := range []string{"https://", "http://", "//"} {
		s = strings.TrimPrefix(s, p)
	}
	s = strings.TrimPrefix(s, "www.")
	if !strings.HasPrefix(s, shareLinkHost) {
		return ShareTarget{}, false
	}
	s = strings.TrimPrefix(s, shareLinkHost)

	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '/' })
	if len(parts) < 3 {
		// A version or book index (or the landing page) — not a passage. The
		// browser handles those; the app has its own navigation.
		return ShareTarget{}, false
	}
	version, slug, chapterStr := parts[0], parts[1], parts[2]
	if !webVersionIDs[version] {
		return ShareTarget{}, false
	}
	book, ok := BookFromSlug(slug)
	if !ok {
		return ShareTarget{}, false
	}
	chapter, err := strconv.Atoi(chapterStr)
	if err != nil || chapter < 1 {
		return ShareTarget{}, false
	}

	lo, hi := parseVersePayload(frag, query)
	return ShareTarget{VersionID: version, Book: book, Chapter: chapter, VerseLo: lo, VerseHi: hi}, true
}

const shareLinkHost = "bibletext.co.uk"

// parseVersePayload reads the verse span from the fragment ("v16", "v16-18")
// or, failing that, the ?v= alias ("16-18"). Anything unparseable yields no
// verse rather than an error: the chapter is still the right place to land.
func parseVersePayload(frag, query string) (lo, hi int) {
	payload := strings.TrimPrefix(strings.TrimSpace(frag), "v")
	if payload == "" || payload == frag {
		// No "v" prefix in the fragment — try the query alias.
		payload = ""
		for _, kv := range strings.Split(query, "&") {
			if v, found := strings.CutPrefix(kv, "v="); found {
				payload = strings.TrimPrefix(strings.TrimSpace(v), "v")
				break
			}
		}
	}
	if payload == "" {
		return 0, 0
	}
	loStr, hiStr, ranged := strings.Cut(payload, "-")
	lo, err := strconv.Atoi(loStr)
	if err != nil || lo < 1 {
		return 0, 0
	}
	if !ranged {
		return lo, 0
	}
	hi, err = strconv.Atoi(hiStr)
	if err != nil || hi <= lo {
		return lo, 0 // a broken or backwards range still names its first verse
	}
	return lo, hi
}
