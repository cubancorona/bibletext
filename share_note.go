package bibletext

// Shared notes — the codec.
//
// A shared link may carry a short note from the sender, which the reader sees as
// a bubble beside the passage. The note travels INSIDE the link (see
// docs/SHARED_NOTES.md): there is no server, and a fragment is never transmitted,
// so the note is seen by the sender, the recipient, and whatever messenger
// carried the link — and by nobody else, ever, including us.
//
// THE PAYLOAD FORMAT IS FROZEN — the normative statement of every rule below,
// and the argument for each, is docs/NOTE_WIRE_FORMAT.md. Read it before
// changing anything here. The shape:
//
//	byte 0     framing discriminator (NOT a semantics version):
//	             'r'  record stream, raw
//	             'd'  record stream, DEFLATE (emitted only when it comes out smaller)
//	             'p'  legacy bare text, plain      } DECODE-ONLY, never emitted
//	             'z'  legacy bare text, DEFLATE    } again, kept forever
//	             'A'-'Z'  RESERVED: "a newer BibleText format"
//	bytes 1..  a record stream
//
//	record:    <tag: 1 byte> <len: uvarint> <value: len bytes>
//
// EMITTER RULES (canonical form): records ascending by tag, at most one of
// each, minimal uvarints. Emitted today: 'a' (verse runs), 'b' (book index into
// the frozen canon order, share_note_canon.go), 'c' (chapter), 't' (text,
// required), 'v' (sender's translation id, required when known). 'f', 'i' and
// 's' are reserved on paper and never emitted — sender identity is a later,
// deliberate step ([redacted-retired-private-reference]).
//
// DECODER RULES (tolerant; reject ONLY framing failures):
//   - any record order accepted; duplicate tag → first occurrence wins
//   - non-minimal uvarints accepted (binary.Uvarint accepts 0x80 0x00 as 0)
//   - trailing garbage after a complete DEFLATE stream accepted
//   - unknown LOWERCASE tags are skipped by their length and PRESERVED verbatim
//     on the decoded result, so a future forward/re-share can re-emit them
//   - an unknown UPPERCASE tag, or byte 0 in 'A'-'Z', means "a newer BibleText
//     format": the note is not rendered and the reader is told so
//   - 0xFF stops parsing; what was read renders, the rest is preserved verbatim
//   - everything else — not base64, fewer than 2 bytes, an unknown lowercase
//     byte 0, a record length past the end, a truncated record, a missing or
//     empty 't' after normalisation, invalid UTF-8 in 't', a refused inflation
//     or an oversize stream — is DAMAGED, and the reader is told that instead.
//
// Padding is dropped because '=' is the key/value separator in the fragment
// grammar (share_link.go); base64url's alphabet is otherwise A-Z a-z 0-9 - _,
// which keeps the payload unambiguous inside it.

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"strings"
	"unicode/utf8"
)

// NoteMaxRunes caps a note. It bounds the URL (a 280-rune note lands at roughly
// 290 characters of link, which messengers carry without wrapping or
// truncating) and it bounds the abuse surface of text we will render.
//
// ENCODER-SIDE ONLY: the decoder must not enforce a rune cap and refuse a note
// over it (docs/NOTE_WIRE_FORMAT.md, the limits table) — normalizeNote
// truncates instead.
const NoteMaxRunes = 280

var errNoteTooLarge = errors.New("note expands past the size a note may be")

// The four format bytes. 'p'/'z' were never in a shipped build and are kept

// reader.js of that era.
const (
	noteFormatPlain    = 'p' // legacy: bare text — DECODE-ONLY
	noteFormatDeflate  = 'z' // legacy: bare DEFLATE'd text — DECODE-ONLY
	noteFormatRecords  = 'r' // record stream, raw
	noteFormatRecordsZ = 'd' // record stream, DEFLATE (only when smaller)
)

// The record tags this build understands. Lowercase = optional (an unknown one
// is skipped); UPPERCASE = critical (an unknown one means a newer format).
const (
	noteTagRuns    = 'a' // verse runs: uvarint n, then n × (uvarint lo, uvarint hi)
	noteTagBook    = 'b' // uvarint index into noteBookOrder (share_note_canon.go)
	noteTagChapter = 'c' // uvarint chapter, >= 1
	noteTagText    = 't' // UTF-8 note text — REQUIRED
	noteTagVersion = 'v' // sender's translation id, [a-z0-9-]{1,8}
	noteTagStop    = 0xFF
)

// NoteOutcome is what DecodeNote concluded about a payload. There is no silent
// arm: ok renders the note, and BOTH failure arms are TOLD to the reader
// (docs/NOTE_WIRE_FORMAT.md rule 5) — in the note's place, attributed to
// nobody, with no call to action and no link. The passage always opens.
type NoteOutcome uint8

const (
	// NoteOutcomeNone: the link carried no note payload at all. DecodeNote
	// never returns it — it exists so ShareTarget's zero value is honest.
	NoteOutcomeNone NoteOutcome = iota
	// NoteOutcomeOK: a note, decoded, plus every field that was present.
	NoteOutcomeOK
	// NoteOutcomeNewer: byte 0 in 'A'-'Z', or an unknown UPPERCASE record tag —
	// a note written in a newer note format than this build understands.
	NoteOutcomeNewer
	// NoteOutcomeDamaged: any framing failure. The one arm that rejects.
	NoteOutcomeDamaged
)

// The two sentences a reader is shown when the note cannot be rendered. Frozen
// wording (docs/NOTE_WIRE_FORMAT.md rule 5): no call to action, no link, never
// an install prompt — reader.js carries the same two strings.
const (
	noteNewerFormatMessage = "This link carries a note written in a newer note format."
	noteDamagedMessage     = "This link's note looks damaged."
)

// noteOutcomeMessage is the sentence for a failed outcome, "" for the others.
func noteOutcomeMessage(o NoteOutcome) string {
	switch o {
	case NoteOutcomeNewer:
		return noteNewerFormatMessage
	case NoteOutcomeDamaged:
		return noteDamagedMessage
	}
	return ""
}

// NoteVerseRun is one contiguous verse run of a note's anchor, inclusive.
type NoteVerseRun struct{ Lo, Hi int }

// DecodedNote is everything a payload carried that this build can read.
// Zero-valued fields were absent; Runs distinguishes absent (nil) from
// present-with-zero-runs (empty, a real assertion — see [redacted-retired-private-reference]
// on the 'a' record).
type DecodedNote struct {
	Text    string // normalized; non-empty exactly when the outcome is OK
	Version string // sender's translation id ('v'), "" when absent
	Book    string // canonical book name resolved from 'b', "" when absent
	Chapter int    // 'c', 0 when absent
	Runs    []NoteVerseRun

	// Skipped holds every record this build could not USE, verbatim
	// (tag+len+value): unknown lowercase tags, and known tags whose value did
	// not parse. Preserved, not merely skipped (docs/NOTE_WIRE_FORMAT.md rule
	// 3), so a future forward/re-share can re-emit them instead of silently
	// destroying the sender's data on its way through us.
	Skipped [][]byte
	// Opaque is the 0xFF stop byte and everything after it, verbatim, for the
	// same reason.
	Opaque []byte
}

// NoteWire is what the encoder is given. Only Text is required; everything
// else is emitted when present and meaningful.
type NoteWire struct {
	Text    string
	Version string // the SENDER's translation id — not the (lossy) link path
	Book    string // canonical book name
	Chapter int
	VerseLo int // 0 = no verse run
	VerseHi int // 0 or < VerseLo = single verse
}

// EncodeNote is the text-only convenience over the record encoder: a payload
// carrying just 't'. Callers that know the passage should use EncodeNoteWire.
// It returns "" for a note that is empty once trimmed — callers then emit a
// plain link rather than one carrying nothing.
func EncodeNote(note string) string {
	return EncodeNoteWire(NoteWire{Text: note})
}

// EncodeNoteWire turns a note into the payload that rides in a shared link's
// fragment: byte 0 'r' (or 'd' when DEFLATE comes out smaller), then records
// in canonical form — ascending by tag, at most one of each, minimal uvarints.
//
// The note is TRUNCATED rather than rejected if it is too long: a share is a
// gesture, and failing it outright at the last moment is worse than sending a
// slightly shortened note. Callers should hold the writer to NoteMaxRunes in
// the UI so this never fires.
func EncodeNoteWire(w NoteWire) string {
	text := normalizeNote(w.Text)
	if text == "" {
		return ""
	}

	var stream []byte
	var buf [binary.MaxVarintLen64]byte
	appendRecord := func(tag byte, val []byte) {
		// Its OWN length buffer: val is often a slice of buf above, and writing
		// the length into the same array would clobber the value before it is
		// appended.
		var lenBuf [binary.MaxVarintLen64]byte
		stream = append(stream, tag)
		stream = append(stream, lenBuf[:binary.PutUvarint(lenBuf[:], uint64(len(val)))]...)
		stream = append(stream, val...)
	}

	// Ascending tag order: a < b < c < t < v.
	if w.VerseLo >= 1 {
		hi := w.VerseHi
		if hi < w.VerseLo {
			hi = w.VerseLo
		}
		var val []byte
		val = append(val, buf[:binary.PutUvarint(buf[:], 1)]...)
		val = append(val, buf[:binary.PutUvarint(buf[:], uint64(w.VerseLo))]...)
		val = append(val, buf[:binary.PutUvarint(buf[:], uint64(hi))]...)
		appendRecord(noteTagRuns, val)
	}
	if idx, ok := noteBookIndexOf(w.Book); ok {
		appendRecord(noteTagBook, buf[:binary.PutUvarint(buf[:], uint64(idx))])
	}
	if w.Chapter >= 1 {
		appendRecord(noteTagChapter, buf[:binary.PutUvarint(buf[:], uint64(w.Chapter))])
	}
	appendRecord(noteTagText, []byte(text))
	if v := noteWireVersionID(w.Version); v != "" {
		appendRecord(noteTagVersion, []byte(v))
	}

	best := append([]byte{noteFormatRecords}, stream...)
	if z, err := deflateBytes(stream); err == nil && len(z)+1 < len(best) {
		best = append([]byte{noteFormatRecordsZ}, z...)
	}
	return base64.RawURLEncoding.EncodeToString(best)
}

// noteWireVersionID is the one shape a 'v' value may have — [a-z0-9-]{1,8} —
// applied on BOTH sides so the encoder and decoder cannot disagree. The
// encoder lowercases first (ids are case-insensitive everywhere else in the
// link grammar); the decoder does not rewrite, it only accepts or declines.
func noteWireVersionID(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	if !validNoteVersionID(id) {
		return ""
	}
	return id
}

func validNoteVersionID(id string) bool {
	if id == "" || len(id) > 8 {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return false
		}
	}
	return true
}

// DecodeNote is the inverse: the decoded record plus what to do about it.
// On any outcome other than NoteOutcomeOK the DecodedNote is zero — it never
// returns a partially decoded note, because the caller's next move is to
// render it.
//
// A note arriving here is UNTRUSTED — it came from a URL that anyone can
// write. This function guarantees only that Text is well-formed, printable
// UTF-8 within the length cap. It is NOT sanitised for a markup context: every
// caller must insert it as text (textContent, an escaped HTML write, an
// attributed string), never as markup. See docs/SHARED_NOTES.md → Security.
func DecodeNote(payload string) (DecodedNote, NoteOutcome) {
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return DecodedNote{}, NoteOutcomeDamaged
	}
	// Tolerate a padded payload: we never emit one, but a messenger or a
	// hand-edited link might arrive with it.
	enc := base64.RawURLEncoding
	if strings.HasSuffix(payload, "=") {
		enc = base64.URLEncoding
	}
	blob, err := enc.DecodeString(payload)
	if err != nil || len(blob) < 2 {
		return DecodedNote{}, NoteOutcomeDamaged
	}

	switch b0 := blob[0]; {
	case b0 == noteFormatPlain:
		return decodeLegacyNoteText(blob[1:])
	case b0 == noteFormatDeflate:
		raw, err := inflateBytes(blob[1:], noteMaxInflatedBytes)
		if err != nil {
			return DecodedNote{}, NoteOutcomeDamaged
		}
		return decodeLegacyNoteText(raw)
	case b0 == noteFormatRecords:
		if len(blob)-1 > noteMaxRecordBytes {
			return DecodedNote{}, NoteOutcomeDamaged // size refusal, same as a bomb
		}
		return decodeNoteRecords(blob[1:])
	case b0 == noteFormatRecordsZ:
		stream, err := inflateBytes(blob[1:], noteMaxRecordBytes)
		if err != nil {
			return DecodedNote{}, NoteOutcomeDamaged
		}
		return decodeNoteRecords(stream)
	case b0 >= 'A' && b0 <= 'Z':
		return DecodedNote{}, NoteOutcomeNewer // reserved: a newer BibleText format
	default:
		return DecodedNote{}, NoteOutcomeDamaged // unknown byte 0: framing, fail closed
	}
}

// decodeLegacyNoteText is the 'p'/'z' tail: bare text, mapped to outcome ok
// with only Text set. Kept forever — a link is forever.
func decodeLegacyNoteText(raw []byte) (DecodedNote, NoteOutcome) {
	if !utf8.Valid(raw) {
		return DecodedNote{}, NoteOutcomeDamaged
	}
	text := normalizeNote(string(raw))
	if text == "" {
		return DecodedNote{}, NoteOutcomeDamaged
	}
	return DecodedNote{Text: text}, NoteOutcomeOK
}

// decodeNoteRecords walks a record stream under the decoder rules in the file
// header. Framing failures are the ONLY rejections; a known tag whose value
// cannot be used is preserved verbatim beside the unknown ones, so nothing the
// sender wrote is destroyed by our failure to understand it.
func decodeNoteRecords(s []byte) (DecodedNote, NoteOutcome) {
	var d DecodedNote
	var textRaw []byte
	sawText := false
	var seen [256]bool

	i := 0
	for i < len(s) {
		tag := s[i]
		if tag == noteTagStop {
			d.Opaque = append([]byte(nil), s[i:]...)
			break
		}
		start := i
		i++
		length, n := binary.Uvarint(s[i:])
		if n <= 0 {
			return DecodedNote{}, NoteOutcomeDamaged // truncated or overlong uvarint
		}
		// The length is checked against the REMAINING bytes, as uint64, BEFORE
		// any offset+len arithmetic — so a hostile length can never wrap.
		if length > uint64(len(s)-i-n) {
			return DecodedNote{}, NoteOutcomeDamaged // record len past end
		}
		i += n
		val := s[i : i+int(length)]
		i += int(length)

		if tag >= 'A' && tag <= 'Z' {
			// A critical record this build does not know. The whole stream is
			// from a newer format; nothing of it is rendered.
			return DecodedNote{}, NoteOutcomeNewer
		}

		switch tag {
		case noteTagText, noteTagVersion, noteTagBook, noteTagChapter, noteTagRuns:
			if seen[tag] {
				break // duplicate tag: first occurrence wins
			}
			seen[tag] = true
			if !applyNoteField(&d, tag, val, &textRaw, &sawText) {
				// A known tag whose value did not parse: unusable here, but
				// not ours to destroy.
				d.Skipped = append(d.Skipped, append([]byte(nil), s[start:i]...))
			}
		default:
			// An unknown lowercase (or otherwise non-critical) tag: skipped by
			// its length, preserved verbatim.
			d.Skipped = append(d.Skipped, append([]byte(nil), s[start:i]...))
		}
	}

	if !sawText {
		return DecodedNote{}, NoteOutcomeDamaged // missing 't'
	}
	if !utf8.Valid(textRaw) {
		return DecodedNote{}, NoteOutcomeDamaged
	}
	text := normalizeNote(string(textRaw))
	if text == "" {
		return DecodedNote{}, NoteOutcomeDamaged // empty 't' after normalisation
	}
	d.Text = text
	return d, NoteOutcomeOK
}

// applyNoteField decodes one known record's value into the note, reporting
// whether the value was usable. An unusable value is NOT a framing failure —
// the caller preserves it verbatim and the note still shows (missing must
// degrade, never destroy).
func applyNoteField(d *DecodedNote, tag byte, val []byte, textRaw *[]byte, sawText *bool) bool {
	switch tag {
	case noteTagText:
		*textRaw = val
		*sawText = true
		return true
	case noteTagVersion:
		if !validNoteVersionID(string(val)) {
			return false
		}
		d.Version = string(val)
		return true
	case noteTagBook:
		idx, n := binary.Uvarint(val)
		if n <= 0 || n != len(val) || idx >= uint64(len(noteBookOrder)) {
			return false
		}
		d.Book = noteBookOrder[idx]
		return true
	case noteTagChapter:
		c, n := binary.Uvarint(val)
		if n <= 0 || n != len(val) || c < 1 || c > 1<<31-1 {
			return false
		}
		d.Chapter = int(c)
		return true
	case noteTagRuns:
		runs, ok := parseNoteRuns(val)
		if !ok {
			return false
		}
		d.Runs = runs
		return true
	}
	return false
}

// parseNoteRuns reads the 'a' value: uvarint n, then n × (uvarint lo, uvarint
// hi). Zero runs is a real, meaningful state and comes back as an empty
// non-nil slice. The count cannot drive an allocation — runs are appended as
// they parse, and every run consumes bytes, so the value's own length bounds
// the loop.
func parseNoteRuns(val []byte) ([]NoteVerseRun, bool) {
	count, off := binary.Uvarint(val)
	if off <= 0 {
		return nil, false
	}
	rest := val[off:]
	runs := []NoteVerseRun{}
	for k := uint64(0); k < count; k++ {
		lo, n1 := binary.Uvarint(rest)
		if n1 <= 0 {
			return nil, false
		}
		rest = rest[n1:]
		hi, n2 := binary.Uvarint(rest)
		if n2 <= 0 {
			return nil, false
		}
		rest = rest[n2:]
		if lo < 1 || lo > 1<<31-1 || hi > 1<<31-1 {
			return nil, false
		}
		if hi < lo {
			hi = lo // a backwards run still names its first verse
		}
		runs = append(runs, NoteVerseRun{Lo: int(lo), Hi: int(hi)})
	}
	if len(rest) != 0 {
		return nil, false // the value must be exactly its runs
	}
	return runs, true
}

// normalizeNote is the one place a note's shape is decided, so the encoder and
// decoder cannot disagree about what is legal. It runs on BOTH sides: a note is
// cleaned before it is sent and again after it arrives, because the second one
// is defending against a payload we did not write.
//
// It strips control characters (which can reorder or hide text when rendered —
// bidi overrides are the classic trick) while keeping the newlines that make a
// note readable, collapses runs of blank lines, and caps the length in RUNES so
// the limit means the same thing in every script.
func normalizeNote(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '\n' || r == '\t':
			b.WriteRune(r)
		case r == utf8.RuneError:
			// Drop invalid sequences rather than rendering replacement glyphs.
		case r < 0x20 || r == 0x7f:
			// Other C0 controls and DEL.
		case r >= 0x80 && r <= 0x9f:
			// C1 controls.
		case r == 0x200e || r == 0x200f || (r >= 0x202a && r <= 0x202e) || (r >= 0x2066 && r <= 0x2069):
			// Bidi marks, embeddings and isolates: a note is a single run of
			// text in a page we control, and these only ever let it lie about
			// its own direction.
		case r == 0xfeff:
			// Zero-width no-break space / BOM.
		default:
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(b.String())

	// Collapse three-or-more newlines to a paragraph break, so a note cannot
	// push its own bubble to the height of the screen.
	for strings.Contains(out, "\n\n\n") {
		out = strings.ReplaceAll(out, "\n\n\n", "\n\n")
	}

	if n := utf8.RuneCountInString(out); n > NoteMaxRunes {
		count := 0
		for i := range out {
			if count == NoteMaxRunes {
				out = out[:i]
				break
			}
			count++
		}
		out = strings.TrimSpace(out)
	}
	return out
}

func deflateBytes(raw []byte) ([]byte, error) {
	var buf bytes.Buffer
	w, err := flate.NewWriter(&buf, flate.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(raw); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// noteMaxInflatedBytes is the most a LEGACY 'z' note may expand to. A named
// constant, not arithmetic at the call site: derived as NoteMaxRunes*4+1 it
// silently changes meaning the day the rune cap moves, and a size limit that
// drifts is the shape of Bitcoin's March 2013 fork — a Berkeley DB lock limit
// nobody had written down had quietly become part of the format.
//
// Four bytes per rune is UTF-8's maximum, so this admits every legal bare-text
// note and nothing else. It is FROZEN at the legacy value for the legacy
// formats — the record formats carry more than the text and have their own cap.
const noteMaxInflatedBytes = NoteMaxRunes*4 + 1

// noteMaxRecordBytes bounds an 'r'/'d' RECORD STREAM (raw length, or inflated
// length — same number, one limit). The maximal legal note text is 1,121 bytes;
// the rest is headroom for the anchor records, the reserved sender fields, and
// unknown fields from future builds that this build must skip-and-preserve
// without letting a hostile payload buy an unbounded allocation. This number is
// part of the wire format from the first shipped build onward
// (docs/NOTE_WIRE_FORMAT.md: whatever v1.0 does IS the spec) — it may grow in a
// later build, never shrink.
const noteMaxRecordBytes = 4096

// inflateBytes bounds its output: a hostile payload could otherwise be a zip
// bomb, a few hundred bytes of link expanding to gigabytes on a phone.
func inflateBytes(z []byte, limit int) ([]byte, error) {
	r := flate.NewReader(bytes.NewReader(z))
	defer r.Close()

	// Read ONE BYTE past the cap, and reject if we got it.
	//
	// This used to read exactly the cap through a LimitReader and return
	// whatever came back. A LimitReader reports io.EOF at its limit, which is
	// not an error, so a payload engineered to expand without bound came back
	// TRUNCATED and err == nil, and the truncation was then rendered as if the
	// sender had written it. Measured: 5,114 bytes of payload expanding to 5 MB
	// returned 1,121 bytes and no error. That contradicted this function's own
	// promise never to return a partially decoded note, and it is the one place
	// in the note path where a hostile payload reached the screen.
	//
	// Bitcoin's discipline for the same hazard (MAX_SIZE / MAX_VECTOR_ALLOCATE):
	// never let a declared or produced size drive an allocation, and validate
	// BEFORE you trust. Here that means asking for more than is allowed and
	// treating the surplus as proof of a lie.
	out, err := io.ReadAll(io.LimitReader(r, int64(limit)+1))
	if err != nil {
		return nil, err
	}
	if len(out) > limit {
		return nil, errNoteTooLarge
	}
	return out, nil
}
