package bibletext

// Shared notes — the codec.
//
// A shared link may carry a short note from the sender, which the reader sees as
// a bubble beside the passage. The note travels INSIDE the link (see
// docs/SHARED_NOTES.md): there is no server, and a fragment is never transmitted,
// so the note is seen by the sender, the recipient, and whatever messenger
// carried the link — and by nobody else, ever, including us.
//
// THE PAYLOAD FORMAT IS FROZEN the moment a link is sent, so it is versioned by
// its first byte and everything else is defensive:
//
//	byte 0        format tag: noteFormatPlain or noteFormatDeflate
//	bytes 1..n    UTF-8 text, raw or raw-DEFLATE'd
//	              → base64url, unpadded
//
// Deflate is used ONLY when it comes out smaller. On a 40-character note it
// costs bytes (the window never pays for itself); on a 270-character one it
// saves about a third. The tag says which, so the decoder never has to guess.
//
// Padding is dropped because '=' is the key/value separator in the fragment
// grammar (share_link.go); base64url's alphabet is otherwise A-Z a-z 0-9 - _,
// which keeps the payload unambiguous inside it.

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"io"
	"strings"
	"unicode/utf8"
)

// NoteMaxRunes caps a note. It bounds the URL (a 280-rune note lands at roughly
// 290 characters of link, which messengers carry without wrapping or
// truncating) and it bounds the abuse surface of text we will render.
const NoteMaxRunes = 280

const (
	noteFormatPlain   = 'p'
	noteFormatDeflate = 'z'
)

// EncodeNote turns a note into the payload that rides in a shared link's
// fragment. It returns "" for a note that is empty once trimmed — callers then
// emit a plain link rather than one carrying nothing.
//
// The note is TRUNCATED rather than rejected if it is too long: a share is a
// gesture, and failing it outright at the last moment is worse than sending a
// slightly shortened note. Callers should hold the writer to NoteMaxRunes in the
// UI so this never fires.
func EncodeNote(note string) string {
	text := normalizeNote(note)
	if text == "" {
		return ""
	}
	raw := []byte(text)

	best := append([]byte{noteFormatPlain}, raw...)
	if z, err := deflateBytes(raw); err == nil && len(z)+1 < len(best) {
		best = append([]byte{noteFormatDeflate}, z...)
	}
	return base64.RawURLEncoding.EncodeToString(best)
}

// DecodeNote is the inverse. ok is false for anything that is not a note we
// wrote: a corrupt payload, an unknown format tag, or text that is not valid
// UTF-8. It never returns a partially decoded note, because the caller's next
// move is to render it.
//
// A note arriving here is UNTRUSTED — it came from a URL that anyone can write.
// This function guarantees only that the result is well-formed, printable UTF-8
// within the length cap. It is NOT sanitised for a markup context: every caller
// must insert it as text (textContent, an escaped HTML write, an attributed
// string), never as markup. See docs/SHARED_NOTES.md → Security.
func DecodeNote(payload string) (string, bool) {
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return "", false
	}
	// Tolerate a padded payload: we never emit one, but a messenger or a
	// hand-edited link might arrive with it.
	enc := base64.RawURLEncoding
	if strings.HasSuffix(payload, "=") {
		enc = base64.URLEncoding
	}
	blob, err := enc.DecodeString(payload)
	if err != nil || len(blob) < 2 {
		return "", false
	}

	var raw []byte
	switch blob[0] {
	case noteFormatPlain:
		raw = blob[1:]
	case noteFormatDeflate:
		raw, err = inflateBytes(blob[1:])
		if err != nil {
			return "", false
		}
	default:
		return "", false // a format from a future we do not know: show nothing
	}

	if !utf8.Valid(raw) {
		return "", false
	}
	text := normalizeNote(string(raw))
	if text == "" {
		return "", false
	}
	return text, true
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

// inflateBytes bounds its output: a hostile payload could otherwise be a zip
// bomb, a few hundred bytes of link expanding to gigabytes on a phone. The cap
// is generous against NoteMaxRunes (4 bytes per rune is the UTF-8 maximum) and
// still refuses anything that is not plausibly a note.
func inflateBytes(z []byte) ([]byte, error) {
	r := flate.NewReader(bytes.NewReader(z))
	defer r.Close()
	limit := int64(NoteMaxRunes*4 + 1)
	out, err := io.ReadAll(io.LimitReader(r, limit))
	if err != nil {
		return nil, err
	}
	return out, nil
}
