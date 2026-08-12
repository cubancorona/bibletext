package bibletext

import (
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
		got, ok := DecodeNote(payload)
		if !ok {
			t.Errorf("DecodeNote failed for %q (payload %q)", note, payload)
			continue
		}
		if want := normalizeNote(note); got != want {
			t.Errorf("round trip changed the note:\n got %q\nwant %q", got, want)
		}
		// The payload has to survive a URL fragment untouched.
		if strings.ContainsAny(payload, "&=#?/ ") {
			t.Errorf("payload %q contains a character the fragment grammar uses", payload)
		}
	}
}

// Deflate must be used only when it actually helps, and the tag must say which
// so the decoder never guesses.
func TestNoteCompressionOnlyWhenSmaller(t *testing.T) {
	short := "synthetic note."
	long := strings.Repeat("the same words over and over ", 9)

	if got := decodeTag(t, EncodeNote(short)); got != noteFormatPlain {
		t.Errorf("a short note should stay plain, got tag %q", got)
	}
	if got := decodeTag(t, EncodeNote(long)); got != noteFormatDeflate {
		t.Errorf("a long repetitive note should deflate, got tag %q", got)
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
		"AAAA",                         // decodes, but the tag is unknown
		"cA",                           // tag 'p' with no body
		"eg",                           // tag 'z' with no body
		"e" + strings.Repeat("A", 400), // tag 'z' with garbage
		strings.Repeat("A", 5000),
	} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("DecodeNote(%.20q) panicked: %v", payload, r)
				}
			}()
			if got, ok := DecodeNote(payload); ok && got == "" {
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
	round, ok := DecodeNote(EncodeNote(long))
	if !ok || utf8.RuneCountInString(round) != NoteMaxRunes {
		t.Errorf("a too-long note should round-trip at the cap, got %d runes (ok=%v)",
			utf8.RuneCountInString(round), ok)
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
	out, err := inflateBytes(bomb)
	// Either refuse it or truncate it — never hand back the whole bomb.
	if err == nil && len(out) > NoteMaxRunes*4+1 {
		t.Errorf("inflate returned %d bytes, past the %d cap — the bound is not enforced",
			len(out), NoteMaxRunes*4+1)
	}
	if err == nil && len(out) >= 8<<20 {
		t.Error("inflate expanded the bomb in full")
	}
}
