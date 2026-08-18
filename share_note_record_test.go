package bibletext

// The record form of the note wire (docs/NOTE_WIRE_FORMAT.md): the emitter's
// canonical-form rules, the decoder's tolerances, and the two guards that keep
// the format honest forever — the byte-0 sweep and the frozen book order.

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"strings"
	"testing"
)

// rawNotePayload base64s a hand-built blob, for vectors the encoder refuses to
// produce (out-of-order records, duplicates, non-minimal varints...).
func rawNotePayload(blob []byte) string {
	return base64.RawURLEncoding.EncodeToString(blob)
}

// rec frames one record by hand.
func rec(tag byte, val []byte) []byte {
	var buf [binary.MaxVarintLen64]byte
	out := []byte{tag}
	out = append(out, buf[:binary.PutUvarint(buf[:], uint64(len(val)))]...)
	return append(out, val...)
}

// The emitter MUST write canonical form: ascending tag, at most one of each,
// minimal uvarints (docs/NOTE_WIRE_FORMAT.md rule 2 — an emitter rule, not a
// decoder rule).
func TestEncoderEmitsCanonicalForm(t *testing.T) {
	payload := EncodeNoteWire(NoteWire{
		Text: "hello", Version: "bsb", Book: "John", Chapter: 3, VerseLo: 16, VerseHi: 18,
	})
	blob, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		t.Fatal(err)
	}
	if blob[0] != noteFormatRecords {
		t.Fatalf("byte 0 = %q, want 'r'", blob[0])
	}
	var tags []byte
	s := blob[1:]
	for i := 0; i < len(s); {
		tag := s[i]
		tags = append(tags, tag)
		i++
		length, n := binary.Uvarint(s[i:])
		if n <= 0 {
			t.Fatalf("bad uvarint at %d", i)
		}
		// Minimal: re-encoding must reproduce the same bytes.
		var buf [binary.MaxVarintLen64]byte
		if m := binary.PutUvarint(buf[:], length); m != n || !bytes.Equal(buf[:m], s[i:i+n]) {
			t.Errorf("non-minimal uvarint emitted for tag %q", tag)
		}
		i += n + int(length)
	}
	want := []byte{'a', 'b', 'c', 't', 'v'}
	if !bytes.Equal(tags, want) {
		t.Errorf("tags = %q, want %q (ascending, one each)", tags, want)
	}
}

// A golden of the encoder's exact bytes. If this changes, the wire format
// changed — which after the first shipped build is forbidden, and before it is
// a decision to record in docs/NOTE_WIRE_FORMAT.md.
func TestEncoderGolden(t *testing.T) {
	got := EncodeNoteWire(NoteWire{
		Text: "hi", Version: "web", Book: "Genesis", Chapter: 1, VerseLo: 1, VerseHi: 2,
	})
	// r | a 3 [1 1 2] | b 1 [0] | c 1 [1] | t 2 "hi" | v 3 "web"
	want := rawNotePayload([]byte{
		'r',
		'a', 3, 1, 1, 2,
		'b', 1, 0,
		'c', 1, 1,
		't', 2, 'h', 'i',
		'v', 3, 'w', 'e', 'b',
	})
	if got != want {
		t.Errorf("encoder bytes moved:\n got %s\nwant %s", got, want)
	}
}

// Decoder tolerances, per the spec: any order, first-occurrence-wins,
// non-minimal uvarints, trailing garbage after DEFLATE. Reject ONLY framing
// failures.
func TestDecoderTolerances(t *testing.T) {
	t.Run("out of order records", func(t *testing.T) {
		blob := append([]byte{'r'}, rec('v', []byte("bsb"))...)
		blob = append(blob, rec('t', []byte("backwards"))...)
		d, o := DecodeNote(rawNotePayload(blob))
		if o != NoteOutcomeOK || d.Text != "backwards" || d.Version != "bsb" {
			t.Errorf("got %+v outcome %d", d, o)
		}
	})
	t.Run("duplicate tag first occurrence wins", func(t *testing.T) {
		blob := append([]byte{'r'}, rec('t', []byte("first"))...)
		blob = append(blob, rec('t', []byte("second"))...)
		d, o := DecodeNote(rawNotePayload(blob))
		if o != NoteOutcomeOK || d.Text != "first" {
			t.Errorf("got %q outcome %d, want first/ok", d.Text, o)
		}
	})
	t.Run("non-minimal uvarint accepted", func(t *testing.T) {
		// len 4 spelled as 0x84 0x00.
		blob := []byte{'r', 't', 0x84, 0x00, 'n', 'o', 't', 'e'}
		d, o := DecodeNote(rawNotePayload(blob))
		if o != NoteOutcomeOK || d.Text != "note" {
			t.Errorf("got %q outcome %d", d.Text, o)
		}
	})
	t.Run("trailing garbage after a complete DEFLATE stream", func(t *testing.T) {
		stream := rec('t', []byte("compressed note that is long enough to shrink shrink shrink shrink"))
		z, err := deflateBytes(stream)
		if err != nil {
			t.Fatal(err)
		}
		blob := append([]byte{'d'}, z...)
		blob = append(blob, 0xde, 0xad, 0xbe, 0xef)
		d, o := DecodeNote(rawNotePayload(blob))
		if o != NoteOutcomeOK || !strings.HasPrefix(d.Text, "compressed") {
			t.Errorf("got %q outcome %d", d.Text, o)
		}
	})
	t.Run("backwards verse run names its first verse", func(t *testing.T) {
		blob := append([]byte{'r'}, rec('a', []byte{1, 18, 16})...)
		blob = append(blob, rec('t', []byte("x"))...)
		d, o := DecodeNote(rawNotePayload(blob))
		if o != NoteOutcomeOK || len(d.Runs) != 1 || d.Runs[0] != (NoteVerseRun{Lo: 18, Hi: 18}) {
			t.Errorf("got %+v outcome %d", d.Runs, o)
		}
	})
	t.Run("zero runs is present-but-empty", func(t *testing.T) {
		blob := append([]byte{'r'}, rec('a', []byte{0})...)
		blob = append(blob, rec('t', []byte("x"))...)
		d, o := DecodeNote(rawNotePayload(blob))
		if o != NoteOutcomeOK || d.Runs == nil || len(d.Runs) != 0 {
			t.Errorf("got runs %#v outcome %d, want empty non-nil", d.Runs, o)
		}
	})
}

// Unknown records are SKIPPED but PRESERVED verbatim — BIP 174's pass-through
// rule (docs/NOTE_WIRE_FORMAT.md rule 3) — so a future forward/re-share can
// re-emit them. 0xFF stops parsing and preserves the rest the same way.
func TestUnknownRecordsArePreservedVerbatim(t *testing.T) {
	unknown := rec('q', []byte{9, 9, 9})
	blob := append([]byte{'r'}, unknown...)
	blob = append(blob, rec('t', []byte("still shown"))...)
	d, o := DecodeNote(rawNotePayload(blob))
	if o != NoteOutcomeOK || d.Text != "still shown" {
		t.Fatalf("got %q outcome %d", d.Text, o)
	}
	if len(d.Skipped) != 1 || !bytes.Equal(d.Skipped[0], unknown) {
		t.Errorf("skipped record not preserved verbatim: %#v", d.Skipped)
	}

	// A known tag whose value cannot be used is preserved the same way, and the
	// note still shows: missing must degrade, never destroy.
	badBook := rec('b', []byte{200, 200, 1}) // index far past the canon
	blob = append([]byte{'r'}, badBook...)
	blob = append(blob, rec('t', []byte("shown"))...)
	d, o = DecodeNote(rawNotePayload(blob))
	if o != NoteOutcomeOK || d.Text != "shown" || d.Book != "" {
		t.Fatalf("got %+v outcome %d", d, o)
	}
	if len(d.Skipped) != 1 || !bytes.Equal(d.Skipped[0], badBook) {
		t.Errorf("unusable known record not preserved: %#v", d.Skipped)
	}

	// 0xFF: stop parsing, render what you have, preserve the rest verbatim.
	tail := []byte{0xFF, 0x01, 0x02, 0x03}
	blob = append([]byte{'r'}, rec('t', []byte("before the stop"))...)
	blob = append(blob, tail...)
	d, o = DecodeNote(rawNotePayload(blob))
	if o != NoteOutcomeOK || d.Text != "before the stop" {
		t.Fatalf("got %q outcome %d", d.Text, o)
	}
	if !bytes.Equal(d.Opaque, tail) {
		t.Errorf("opaque tail not preserved: %#v", d.Opaque)
	}
}

// Tag case carries criticality (docs/NOTE_WIRE_FORMAT.md rule 1): an unknown
// UPPERCASE tag means the note is NOT rendered and the reader is told it needs
// a newer format. Framing failures still win — a truncated stream is damage
// whatever tags it holds.
func TestUppercaseTagMeansNewerFormat(t *testing.T) {
	blob := append([]byte{'r'}, rec('t', []byte("hidden"))...)
	blob = append(blob, rec('Q', []byte{1})...)
	d, o := DecodeNote(rawNotePayload(blob))
	if o != NoteOutcomeNewer || d.Text != "" {
		t.Errorf("outcome %d text %q, want newer and nothing rendered", o, d.Text)
	}

	// Truncated uppercase record: the framing is broken before the tag can
	// mean anything — damaged.
	blob = append([]byte{'r'}, 'Q', 10, 1, 2)
	if _, o := DecodeNote(rawNotePayload(blob)); o != NoteOutcomeDamaged {
		t.Errorf("truncated uppercase record: outcome %d, want damaged", o)
	}
}

// The framing failures, each of which must yield DAMAGED and no text.
func TestFramingFailuresAreDamaged(t *testing.T) {
	cases := map[string][]byte{
		"record len past end":     append([]byte{'r'}, 't', 100, 'x'),
		"truncated record":        append([]byte{'r'}, rec('t', []byte("full"))[:3]...),
		"missing t":               append([]byte{'r'}, rec('v', []byte("bsb"))...),
		"empty t after normalise": append([]byte{'r'}, rec('t', []byte("  \n "))...),
		"invalid utf-8 in t":      append([]byte{'r'}, rec('t', []byte{0xff, 0xfe})...),
		"truncated uvarint":       {'r', 't', 0x80},
		"d with garbage":          {'d', 0xde, 0xad},
	}
	for name, blob := range cases {
		if d, o := DecodeNote(rawNotePayload(blob)); o != NoteOutcomeDamaged || d.Text != "" {
			t.Errorf("%s: outcome %d text %q, want damaged", name, o, d.Text)
		}
	}
	// An oversize raw stream is a size refusal, same as a bomb.
	big := append([]byte{'r'}, rec('t', bytes.Repeat([]byte("a"), noteMaxRecordBytes+8))...)
	if _, o := DecodeNote(rawNotePayload(big)); o != NoteOutcomeDamaged {
		t.Errorf("oversize raw stream: outcome %d, want damaged", o)
	}
}

// THE BYTE-0 SWEEP — the spec's guard against ever needing a two-parses-and-a-
// tiebreak heuristic. For EVERY byte value outside {p,z,r,d}, a payload whose
// tail is a perfectly valid record stream must still refuse to parse as one:
// 'A'-'Z' is "newer format", everything else is "damaged", and NOTHING is ok.
// The moment this fails, someone could argue for "try parsing the tail anyway",
// and byte 0 stops being a discriminator.
func TestNoByteZeroOutsideTheFourParsesAsARecordTail(t *testing.T) {
	tail := append(rec('t', []byte("a perfectly ordinary note")), rec('v', []byte("web"))...)
	for b := 0; b < 256; b++ {
		b0 := byte(b)
		if b0 == noteFormatPlain || b0 == noteFormatDeflate ||
			b0 == noteFormatRecords || b0 == noteFormatRecordsZ {
			continue
		}
		payload := rawNotePayload(append([]byte{b0}, tail...))
		_, o := DecodeNote(payload)
		want := NoteOutcomeDamaged
		if b0 >= 'A' && b0 <= 'Z' {
			want = NoteOutcomeNewer
		}
		if o == NoteOutcomeOK {
			t.Fatalf("byte 0 = 0x%02x parsed a record tail as a note — the discriminator leaks", b0)
		}
		if o != want {
			t.Errorf("byte 0 = 0x%02x: outcome %d, want %d", b0, o, want)
		}
	}
}

// The frozen canon order behind the 'b' record: a GOLDEN of all 73 index→name
// pairs. Editing an entry fails here loudly, which is the point — an index,
// once shipped, is immutable, and the table is append-only
// (share_note_canon.go).
func TestNoteBookOrderGolden(t *testing.T) {
	golden := []string{
		0: "Genesis", 1: "Exodus", 2: "Leviticus", 3: "Numbers", 4: "Deuteronomy",
		5: "Joshua", 6: "Judges", 7: "Ruth", 8: "1 Samuel", 9: "2 Samuel",
		10: "1 Kings", 11: "2 Kings", 12: "1 Chronicles", 13: "2 Chronicles", 14: "Ezra",
		15: "Nehemiah", 16: "Esther", 17: "Job", 18: "Psalms", 19: "Proverbs",
		20: "Ecclesiastes", 21: "Song of Solomon", 22: "Isaiah", 23: "Jeremiah",
		24: "Lamentations", 25: "Ezekiel", 26: "Daniel", 27: "Hosea", 28: "Joel",
		29: "Amos", 30: "Obadiah", 31: "Jonah", 32: "Micah", 33: "Nahum",
		34: "Habakkuk", 35: "Zephaniah", 36: "Haggai", 37: "Zechariah", 38: "Malachi",
		39: "Matthew", 40: "Mark", 41: "Luke", 42: "John", 43: "Acts",
		44: "Romans", 45: "1 Corinthians", 46: "2 Corinthians", 47: "Galatians",
		48: "Ephesians", 49: "Philippians", 50: "Colossians", 51: "1 Thessalonians",
		52: "2 Thessalonians", 53: "1 Timothy", 54: "2 Timothy", 55: "Titus",
		56: "Philemon", 57: "Hebrews", 58: "James", 59: "1 Peter", 60: "2 Peter",
		61: "1 John", 62: "2 John", 63: "3 John", 64: "Jude", 65: "Revelation",
		66: "Tobit", 67: "Judith", 68: "1 Maccabees", 69: "2 Maccabees",
		70: "Wisdom", 71: "Sirach", 72: "Baruch",
	}
	if len(noteBookOrder) < len(golden) {
		t.Fatalf("noteBookOrder shrank: %d entries, golden has %d — the table is append-only",
			len(noteBookOrder), len(golden))
	}
	for i, want := range golden {
		if noteBookOrder[i] != want {
			t.Errorf("index %d = %q, want %q — a shipped index is IMMUTABLE", i, noteBookOrder[i], want)
		}
	}
	// Every book a link can name has an index, and nothing indexes a book the
	// slug table does not know.
	for name := range bookSlugs {
		if _, ok := noteBookIndexOf(name); !ok {
			t.Errorf("%q has a slug but no wire index", name)
		}
	}
	for _, name := range noteBookOrder {
		if _, ok := bookSlugs[name]; !ok {
			t.Errorf("%q has a wire index but no slug", name)
		}
	}
}

// Legacy 'p'/'z' decode forever, mapped to outcome ok with only text set.
func TestLegacyFormatsDecodeForever(t *testing.T) {
	text := "a note from a dev build"
	p := rawNotePayload(append([]byte{noteFormatPlain}, []byte(text)...))
	if d, o := DecodeNote(p); o != NoteOutcomeOK || d.Text != text || d.Version != "" {
		t.Errorf("'p': got %+v outcome %d", d, o)
	}
	z, err := deflateBytes([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	zp := rawNotePayload(append([]byte{noteFormatDeflate}, z...))
	if d, o := DecodeNote(zp); o != NoteOutcomeOK || d.Text != text {
		t.Errorf("'z': got %+v outcome %d", d, o)
	}
}
