package bibletext

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestNoteRoundTrip(t *testing.T) {
	for _, note := range []string{
		"synthetic note today — this one carried me.",
		"Read this morning and synthetic note.\n\nPraying, and I'll call Sunday.",
		"Pensé en ti hoy 🙏 — este versículo me sostuvo toda la semana. ¡Te quiero!",
		"日本語のメモ",
		"a",
		strings.Repeat("word ", 55), // long enough that deflate wins
	} {
		payload := EncodeNote(note)
		if payload == "" {
			t.Errorf("EncodeNote(%q) produced nothing", note)
			continue
		}
		rec, outcome := DecodeNote(payload)
		if outcome != NoteOutcomeOK {
			t.Errorf("DecodeNote failed for %q (payload %q, outcome %d)", note, payload, outcome)
			continue
		}
		if want := normalizeNote(note); rec.Text != want {
			t.Errorf("round trip changed the note:\n got %q\nwant %q", rec.Text, want)
		}
		// The payload has to survive a URL fragment untouched.
		if strings.ContainsAny(payload, "&=#?/ ") {
			t.Errorf("payload %q contains a character the fragment grammar uses", payload)
		}
	}
}

// The full wire record — text plus the note's own anchor — must round-trip
// with every field intact.
func TestNoteWireRoundTrip(t *testing.T) {
	w := NoteWire{
		Text:    "Read this one slowly.",
		Version: "nkjv",
		Book:    "John",
		Chapter: 3,
		VerseLo: 16,
		VerseHi: 18,
	}
	rec, outcome := DecodeNote(EncodeNoteWire(w))
	if outcome != NoteOutcomeOK {
		t.Fatalf("outcome = %d, want ok", outcome)
	}
	if rec.Text != w.Text || rec.Version != "nkjv" || rec.Book != "John" || rec.Chapter != 3 {
		t.Errorf("fields lost: %+v", rec)
	}
	if len(rec.Runs) != 1 || rec.Runs[0] != (NoteVerseRun{Lo: 16, Hi: 18}) {
		t.Errorf("runs lost: %+v", rec.Runs)
	}
	if len(rec.Skipped) != 0 || rec.Opaque != nil {
		t.Errorf("nothing should be skipped in our own payload: %+v", rec)
	}
}

// Deflate must be used only when it actually helps, and the format byte must
// say which so the decoder never guesses. 'p'/'z' are never emitted again.
func TestNoteCompressionOnlyWhenSmaller(t *testing.T) {
	short := "synthetic note."
	long := strings.Repeat("the same words over and over ", 9)

	if got := decodeTag(t, EncodeNote(short)); got != noteFormatRecords {
		t.Errorf("a short note should stay raw ('r'), got tag %q", got)
	}
	if got := decodeTag(t, EncodeNote(long)); got != noteFormatRecordsZ {
		t.Errorf("a long repetitive note should deflate ('d'), got tag %q", got)
	}
	// And deflating must genuinely shrink it.
	if len(EncodeNote(long)) >= len(long) {
		t.Errorf("deflated payload (%d) is not smaller than the note (%d)",
			len(EncodeNote(long)), len(long))
	}
}

// decodeTag reads the format byte the encoder chose, which is the first byte
// under the base64 — the thing that tells the decoder whether to inflate.
func decodeTag(t *testing.T, payload string) byte {
	t.Helper()
	if payload == "" {
		t.Fatal("empty payload")
	}
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil || len(raw) == 0 {
		t.Fatalf("cannot read the tag from %q: %v", payload, err)
	}
	return raw[0]
}

// A note is text from a URL anyone can write. Nothing here may panic, and
// nothing may come back that a caller would be surprised to render.
func TestNoteRejectsHostileInput(t *testing.T) {
	for _, payload := range []string{
		"", " ", "!!!!", "////", "=", "==",
		"AAAA",                         // decodes, but byte 0 is unknown
		"cA",                           // tag 'p' with no body
		"eg",                           // tag 'z' with no body
		"e" + strings.Repeat("A", 400), // tag 'z' with garbage
		"cg",                           // tag 'r' with no body
		"ZA",                           // tag 'd' with no body
		strings.Repeat("A", 5000),
	} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("DecodeNote(%.20q) panicked: %v", payload, r)
				}
			}()
			if rec, outcome := DecodeNote(payload); outcome == NoteOutcomeOK && rec.Text == "" {
				t.Errorf("DecodeNote(%.20q) reported ok with an empty note", payload)
			}
		}()
	}
}

// Control characters and bidi overrides let text lie about its own direction or
// hide part of itself. A note is a single run of text on a page we control, so
// they are stripped rather than rendered.
func TestNoteStripsControlAndBidi(t *testing.T) {
	hostile := "safe\u202eevil\u200ftext\a\x00 end"
	got := normalizeNote(hostile)
	for _, bad := range []rune{0x202e, 0x200f, 0x07, 0x00} {
		if strings.ContainsRune(got, bad) {
			t.Errorf("normalizeNote kept U+%04X: %q", bad, got)
		}
	}
	if !strings.Contains(got, "safe") || !strings.Contains(got, "end") {
		t.Errorf("normalizeNote destroyed the legible text: %q", got)
	}
	// Newlines and tabs survive — they are what makes a note readable.
	if n := normalizeNote("one\ntwo\tthree"); n != "one\ntwo\tthree" {
		t.Errorf("normalizeNote should keep newlines and tabs, got %q", n)
	}
}

func TestNoteCapIsCountedInRunes(t *testing.T) {
	// 400 four-byte runes: over the cap in runes, far over it in bytes.
	long := strings.Repeat("😀", 400)
	got := normalizeNote(long)
	if n := utf8.RuneCountInString(got); n != NoteMaxRunes {
		t.Errorf("cap should be %d runes, got %d", NoteMaxRunes, n)
	}
	if !utf8.ValidString(got) {
		t.Error("truncation split a rune")
	}
	round, outcome := DecodeNote(EncodeNote(long))
	if outcome != NoteOutcomeOK || utf8.RuneCountInString(round.Text) != NoteMaxRunes {
		t.Errorf("a too-long note should round-trip at the cap, got %d runes (outcome=%d)",
			utf8.RuneCountInString(round.Text), outcome)
	}
}

// A zip bomb in a link must not be able to allocate its way through a phone.
func TestNoteInflateIsBounded(t *testing.T) {
	bomb, err := deflateBytes(make([]byte, 8<<20)) // 8 MB of zeroes deflates tiny
	if err != nil {
		t.Fatal(err)
	}
	// NO SKIP. The original guard was `len(bomb) > 4096 → t.Skip`, on the belief
	// that "8 MB of zeroes deflates tiny". DEFLATE cannot do that: a length/
	// distance pair covers at most 258 bytes and costs at least 2 bits, so 8 MiB
	// has a hard floor near 8 KB (measured: 8157 bytes at level 9). The condition
	// was therefore ALWAYS true and this test always skipped — the inflate cap it
	// exists to prove was never once exercised.
	if len(bomb) == 0 {
		t.Fatal("nothing to inflate")
	}
	for _, limit := range []int{noteMaxInflatedBytes, noteMaxRecordBytes} {
		out, err := inflateBytes(bomb, limit)
		// Either refuse it or truncate it — never hand back the whole bomb.
		if err == nil && len(out) > limit {
			t.Errorf("inflate returned %d bytes, past the %d cap — the bound is not enforced",
				len(out), limit)
		}
		if err == nil && len(out) >= 8<<20 {
			t.Error("inflate expanded the bomb in full")
		}
	}
}

// A payload engineered to expand without bound must yield NO NOTE, not a
// truncated one presented as the sender's words.
//
// The guard read exactly the cap through an io.LimitReader and returned what
// came back. A LimitReader reports io.EOF at its limit, which is not an error —
// so 5,114 bytes expanding to 5 MB returned 1,121 bytes with err == nil, and
// those 1,121 bytes were rendered as a note somebody had written. The function's
// own doc comment promised the opposite: "It never returns a partially decoded
// note, because the caller's next move is to render it."
func TestADeflateBombYieldsNoNoteAtAll(t *testing.T) {
	var buf bytes.Buffer
	w, err := flate.NewWriter(&buf, flate.BestCompression)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(bytes.Repeat([]byte("A"), 5<<20)); err != nil {
		t.Fatal(err)
	}
	w.Close()

	for _, limit := range []int{noteMaxInflatedBytes, noteMaxRecordBytes} {
		if got, err := inflateBytes(buf.Bytes(), limit); err == nil {
			t.Errorf("%d bytes of payload expanded past the %d cap and came back as %d bytes "+
				"with no error — a hostile note reaching the screen truncated",
				buf.Len(), limit, len(got))
		}
	}

	// ...and the whole way in, through the real decoder, a bomb is simply no
	// note — DAMAGED, under both the legacy 'z' framing and the record 'd' one.
	for _, format := range []byte{noteFormatDeflate, noteFormatRecordsZ} {
		payload := base64.RawURLEncoding.EncodeToString(append([]byte{format}, buf.Bytes()...))
		if rec, outcome := DecodeNote(payload); outcome != NoteOutcomeDamaged || rec.Text != "" {
			t.Errorf("format %q: DecodeNote showed %d characters of a deflate bomb (outcome=%d)",
				format, len(rec.Text), outcome)
		}
	}

	// A real compressed note still decodes — the guard must not cost us the
	// feature it protects.
	long := strings.TrimSpace(strings.Repeat("a real note that compresses well. ", 8))
	if round, outcome := DecodeNote(EncodeNote(long)); outcome != NoteOutcomeOK || round.Text != long {
		t.Errorf("an ordinary long note no longer survives the round trip: %q (outcome=%d)",
			round.Text, outcome)
	}
}
