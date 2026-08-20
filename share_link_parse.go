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
	// VersionID is one of the ids a link path may name (linkPathVersionIDs).
	// That is NO LONGER the same as the ids the web reader publishes, and it is
	// no longer a promise this reader can open it: /nkjv/ is a real link path and
	// a reader without the NKJV cannot be switched to it. Code downstream must
	// therefore handle "the link names a translation we do not have" rather than
	// assume it away — see applyShareTarget.
	VersionID string
	Book      string // canonical book name, e.g. "1 Corinthians"
	Chapter   int    // >= 1
	VerseLo   int    // 0 when the link names no verse (a chapter link)
	VerseHi   int    // 0 or == VerseLo for a single verse

	// Note is the sender's message, already decoded and normalized, or "" when
	// the link carries none or carries one we could not read.
	//
	// IT IS UNTRUSTED TEXT: anyone can write a link. Render it as TEXT, never as
	// markup, and never styled as if BibleText said it — see
	// docs/SHARED_NOTES.md → Security.
	Note string

	// NoteOutcome says what became of the link's note payload:
	// NoteOutcomeNone when the fragment carried no "n" key at all, otherwise
	// DecodeNote's verdict. The two failure outcomes are TOLD to the reader in
	// the note's place (share_link_open.go) — never silently dropped, never a
	// call to action (docs/NOTE_WIRE_FORMAT.md rule 5). The passage opens in
	// every case.
	NoteOutcome NoteOutcome

	// NoteNonce is the payload's per-note identity ('n'). It answers exactly one
	// question on arrival: is this the note I sent? (share_note.go,
	// noteTagNonce.)
	//
	// A fixed ARRAY, not a slice, so ShareTarget stays comparable — seven tests
	// compare whole targets with ==, and that is a good property to keep for a
	// value type describing a parsed link.
	//
	// The zero value means "no nonce": every link made before this existed, and
	// every link whose note this build could not read. An actual all-zero nonce
	// from crypto/rand has probability 2^-48, and its only consequence would be
	// that one note does not collapse — the safe direction.
	NoteNonce [noteNonceLen]byte

	// The note's OWN anchor, from the payload's v/b/c/a records — set only on
	// NoteOutcomeOK, zero when the record was absent. These are AUTHORITATIVE
	// for the NOTE where present: the path is lossy (webc forced for the
	// deuterocanon, unknown ids falling back to web), so the wire says what the
	// sender was actually reading. They do not navigate — the fragment verse
	// span above remains what places the page.
	NoteVersion string
	NoteBook    string
	NoteChapter int
	NoteLo      int // first run of the 'a' record; 0 when absent
	NoteHi      int

	// NoteRuns is the 'a' record's FULL run set — "43-43,45-46" — where
	// NoteLo/NoteHi above carry only the first run. A resolution is a set, not
	// a span (WEB Mark 9:43-46 lands in the BSB with a hole in it), and the
	// store files the whole set (rememberIncomingNote). A string in the
	// noteRunsSpelling grammar, not a slice, so ShareTarget stays comparable;
	// "" when the record was absent or empty.
	NoteRuns string

	// What the payload carried that this build could NOT use, preserved so the
	// store can keep it and a future forward/re-share can re-emit it
	// (docs/NOTE_WIRE_FORMAT.md rule 3). NoteSkipped is DecodedNote.Skipped
	// concatenated — each skipped record is self-framing (tag+len+value), so
	// the stream splits again — and NoteOpaque is the stop byte and everything
	// after it. Strings, not byte slices, so ShareTarget stays comparable.
	NoteSkipped string
	NoteOpaque  string
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
	// A STRICT allow-list, still: /privacy.html, /support.html and the landing
	// page are ours to leave in the browser, and they are only left there because
	// an unrecognised first segment is refused outright. Widening this to "any
	// three-segment path" would swallow them.
	if !linkPathVersionIDs[version] {
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
	t := ShareTarget{
		VersionID: version, Book: book, Chapter: chapter,
		VerseLo: lo, VerseHi: hi,
	}
	// Only a fragment that actually CARRIES an "n" key gets a note verdict: a
	// plain passage link must stay NoteOutcomeNone, or every note-less link
	// would read as "damaged" and the reader would be told about a note that
	// never existed. (An "n=" with an empty payload IS a verdict — damaged.)
	if payload, present := fragmentKeyPresent(frag, "n"); present {
		rec, outcome := DecodeNote(payload)
		t.NoteOutcome = outcome
		if outcome == NoteOutcomeOK {
			t.Note = rec.Text
			if len(rec.Nonce) == noteNonceLen {
				copy(t.NoteNonce[:], rec.Nonce)
			}
			t.NoteVersion = rec.Version
			t.NoteBook = rec.Book
			t.NoteChapter = rec.Chapter
			if len(rec.Runs) > 0 {
				t.NoteLo = rec.Runs[0].Lo
				t.NoteHi = rec.Runs[0].Hi
				t.NoteRuns = noteRunsSpelling(rec.Runs)
			}
			for _, sk := range rec.Skipped {
				t.NoteSkipped += string(sk)
			}
			t.NoteOpaque = string(rec.Opaque)
		}
	}
	return t, true
}

// fragmentKeyPresent reads one key out of the fragment's "&"-separated key
// list, and says whether the key was there at all — "" with present=true is a
// key that arrived empty, which for "n" is a verdict (damaged) rather than an
// absence. Keys we do not recognise are simply not asked for, which is how the
// grammar stays open: a link written by a future version still parses for
// everything this version does understand.
func fragmentKeyPresent(frag, key string) (string, bool) {
	for _, kv := range strings.Split(frag, "&") {
		if v, found := strings.CutPrefix(strings.TrimSpace(kv), key+"="); found {
			return v, true
		}
	}
	return "", false
}

const shareLinkHost = "bibletext.co.uk"

// noteRunsSpelling writes a decoded run set in ShareTarget.NoteRuns' grammar:
// runs joined by ",", each "lo" or "lo-hi" — the fragment's own verse-span
// spelling, chosen so ShareTarget can carry the whole set and stay comparable.
func noteRunsSpelling(runs []NoteVerseRun) string {
	var b strings.Builder
	for i, r := range runs {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.Itoa(r.Lo))
		if r.Hi > r.Lo {
			b.WriteByte('-')
			b.WriteString(strconv.Itoa(r.Hi))
		}
	}
	return b.String()
}

// noteRunsFromSpelling is the inverse. Anything malformed yields nil — the
// first run already rode across in NoteLo/NoteHi, so a damaged tail costs the
// extra runs, never the note.
func noteRunsFromSpelling(s string) []NoteVerseRun {
	if s == "" {
		return nil
	}
	var runs []NoteVerseRun
	for _, part := range strings.Split(s, ",") {
		loStr, hiStr, ranged := strings.Cut(part, "-")
		lo, err := strconv.Atoi(loStr)
		if err != nil || lo < 1 {
			return nil
		}
		hi := lo
		if ranged {
			hi, err = strconv.Atoi(hiStr)
			if err != nil || hi < lo {
				return nil
			}
		}
		runs = append(runs, NoteVerseRun{Lo: lo, Hi: hi})
	}
	return runs
}

// parseVersePayload reads the verse span from the fragment ("v16", "v16-18")
// or, failing that, the ?v= alias ("16-18"). Anything unparseable yields no
// verse rather than an error: the chapter is still the right place to land.
//
// The fragment is a KEY LIST, and the verse is read from whichever field
// carries it rather than from the first field only.
//
// It used to take just the leading field, on the reasoning that an unknown key
// would then be harmless. It is harmless either way, and the narrow version
// disagreed with the published web reader, which splits every field and accepts
// "v=" wherever it appears (cmd/websitegen/assets.go, fragKeys). Nothing we emit
// hits that today because ShareLinkURL always writes the verse first
// (share_link.go:136-142) — but the divergence is one new key away from being
// real, and a grammar whose meaning depends on field order is not a grammar.
// Both spellings are accepted at any position: the bare "v16" the reader emits,
// and the explicit "v=16" the key list implies.
func parseVersePayload(frag, query string) (lo, hi int) {
	var payload string
	for _, field := range strings.Split(frag, "&") {
		field = strings.TrimSpace(field)
		switch {
		case strings.HasPrefix(field, "v="):
			payload = field[len("v="):]
		case len(field) > 1 && field[0] == 'v' && field[1] >= '0' && field[1] <= '9':
			// A bare verse token. The digit test keeps it from swallowing any
			// future key that merely starts with "v".
			payload = field[1:]
		}
		if payload != "" {
			break
		}
	}
	if payload == "" {
		// No verse in the fragment — try the ?v= alias.
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
