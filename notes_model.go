package bibletext

// What a note IS, in the scrapbook store (S5 — docs/NOTES_SCRAPBOOK.md).
//
// A StoredNote is one record in an append-friendly store built for years of
// growth. Three properties here are load-bearing for the "Long-term foundation"
// section of docs/NOTES_SCRAPBOOK.md and cannot be retrofitted later:
//
//  1. IDENTITY IS CARRIED, NEVER REBUILT. A note's ID is minted once from a
//     persisted monotonic counter and never reused — deletion must never free
//     an ID, so later features (threading, replies, tags) always have something
//     stable to point at, and a verb can never reach the wrong note by
//     reconstructing a key from where the reader happens to be standing (the
//     mechanism behind X1, X2, X5 and X13 in docs/NOTES_STATE.md).
//  2. UNKNOWN FIELDS SURVIVE A READ-MODIFY-WRITE. Go's json.Unmarshal drops
//     what it does not know, so an older build rewriting the store would
//     destroy fields a newer build wrote. Extra keeps the unparsed remainder
//     and the custom (Un)MarshalJSON re-emits it.
//  3. KIND IS A STRING, NOT A BOOL. A received note is one kind of record
//     alongside your own notes; highlights and bookmarks are kinds this build
//     has never heard of, and a record whose kind it does not recognise is
//     carried through a rewrite untouched rather than judged.

import (
	"encoding/json"
	"sort"
	"strings"
)

// NoteKind says what sort of record this is. Extensible: a build that meets a
// kind it does not know keeps the record and simply does not draw it.
type NoteKind string

const (
	noteKindReceived NoteKind = "received" // somebody else's message, arrived on a link
	noteKindMine     NoteKind = "mine"     // a note the reader composed and sent
)

// StoredNote is one record of the scrapbook store.
//
// The anchor is VersionID/Book/Chapter plus a run set: AnchorRuns where the
// wire carried one, else the VerseLo/VerseHi span. Resolving it into the
// translation on screen is notes_anchor.go's job (S6), never an inline probe.
type StoredNote struct {
	// ID is this note's identity in the reader's scrapbook, and the ONLY thing
	// a verb ever addresses. Minted from a persisted monotonic counter
	// (prefNotesNextID) — NOT max(existing)+1, because deletion must never free
	// an ID for reuse.
	ID uint64

	// Kind: "received" | "mine". See NoteKind.
	Kind NoteKind

	// The anchor. VersionID is the translation the note was written against —
	// where it is FILED, not necessarily what is on screen.
	VersionID string
	Book      string
	Chapter   int
	VerseLo   int
	VerseHi   int

	// AnchorRuns is the FULL anchor as a run set (S6, docs/NOTES_SCRAPBOOK.md)
	// — what the wire's 'a' record carried, when it carried one. A resolution
	// is a set, not a span: WEB Mark 9:43-46 lands in the BSB as [43,43] and
	// [45,45], which VerseLo/VerseHi cannot say. Empty for a note whose link
	// carried no 'a' record; VerseLo/VerseHi always hold the FIRST run, so
	// everything already reading them keeps working. Additive ("ar",
	// omitempty): a store written before this field serialises byte-identically
	// after it.
	AnchorRuns []anchorRun

	// Text is the message. UNTRUSTED: rendered as text on every surface, never
	// as markup.
	Text string

	// Minimized has ONE meaning: the reader closed this note
	// (docs/NOTES_SCRAPBOOK.md). Written only by a reader's press.
	Minimized bool

	// Received is this device's clock at arrival, unix seconds.
	Received int64

	// SenderName / SenderID are RESERVED: carried, stored, never shown — as
	// SharedNote carried them before this store existed. When a name is shown
	// it will be untrusted text with its own display rules.
	SenderName string
	SenderID   string

	// WireSkipped / WireOpaque preserve what the link's payload carried that
	// the decoding build could not use (share_note.go DecodedNote.Skipped /
	// .Opaque): WireSkipped is the unknown records concatenated verbatim
	// (each is self-framing tag+len+value, so the stream splits again), and
	// WireOpaque is the stop byte and everything after it. Kept so a future
	// forward/re-share can re-emit them instead of silently destroying the
	// sender's data on its way through us (docs/NOTE_WIRE_FORMAT.md rule 3).
	WireSkipped []byte
	WireOpaque  []byte

	// Extra holds every JSON field of this record that this build does not
	// know, verbatim. A newer build's fields pass through an older build's
	// read-modify-write untouched. Never inspected, only re-emitted.
	Extra map[string]json.RawMessage
}

// storedNoteJSON is the known-field wire shape. Field order here IS the byte
// order in the store, so it must stay stable: an unchanged store must
// serialise to identical bytes or preferences.json churns on every write.
type storedNoteJSON struct {
	ID          uint64      `json:"id"`
	Kind        NoteKind    `json:"k"`
	VersionID   string      `json:"v,omitempty"`
	Book        string      `json:"b,omitempty"`
	Chapter     int         `json:"c,omitempty"`
	VerseLo     int         `json:"lo,omitempty"`
	VerseHi     int         `json:"hi,omitempty"`
	AnchorRuns  []anchorRun `json:"ar,omitempty"`
	Text        string      `json:"t,omitempty"`
	Minimized   bool        `json:"m,omitempty"`
	Received    int64       `json:"ts,omitempty"`
	SenderName  string      `json:"sn,omitempty"`
	SenderID    string      `json:"sid,omitempty"`
	WireSkipped []byte      `json:"ws,omitempty"`
	WireOpaque  []byte      `json:"wo,omitempty"`
}

// storedNoteKnownKeys is what UnmarshalJSON subtracts to find Extra. A key
// added to storedNoteJSON must be added here, or it round-trips twice (once
// parsed, once as Extra) and the marshaller refuses duplicates by skipping
// known keys — so the failure mode is a duplicate-free but noisy record, not
// corruption.
var storedNoteKnownKeys = map[string]bool{
	"id": true, "k": true, "v": true, "b": true, "c": true,
	"lo": true, "hi": true, "ar": true, "t": true, "m": true, "ts": true,
	"sn": true, "sid": true, "ws": true, "wo": true,
}

// MarshalJSON emits the known fields in fixed order, then every Extra key in
// sorted order — deterministic bytes, so an unchanged store writes unchanged.
func (n StoredNote) MarshalJSON() ([]byte, error) {
	base, err := json.Marshal(storedNoteJSON{
		ID: n.ID, Kind: n.Kind, VersionID: n.VersionID, Book: n.Book,
		Chapter: n.Chapter, VerseLo: n.VerseLo, VerseHi: n.VerseHi,
		AnchorRuns: n.AnchorRuns,
		Text:       n.Text, Minimized: n.Minimized, Received: n.Received,
		SenderName: n.SenderName, SenderID: n.SenderID,
		WireSkipped: n.WireSkipped, WireOpaque: n.WireOpaque,
	})
	if err != nil || len(n.Extra) == 0 {
		return base, err
	}
	keys := make([]string, 0, len(n.Extra))
	for k := range n.Extra {
		if !storedNoteKnownKeys[k] {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	out := base[:len(base)-1] // strip the closing brace
	for _, k := range keys {
		kb, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		out = append(out, ',')
		out = append(out, kb...)
		out = append(out, ':')
		out = append(out, n.Extra[k]...)
	}
	return append(out, '}'), nil
}

// UnmarshalJSON reads the known fields AND keeps every field it does not know
// in Extra, verbatim. This is spec rule 1 of the long-term foundation: an
// older build's rewrite must not destroy a newer build's fields.
func (n *StoredNote) UnmarshalJSON(data []byte) error {
	// Known fields are decoded INDIVIDUALLY from an exact-case key map — never
	// by unmarshalling the whole object into the struct. Go's struct decoding
	// matches keys CASE-INSENSITIVELY, so a future build's field named "ID" or
	// "Lo" or "TS" would have clobbered the known field on read while also
	// riding along in Extra: the record's identity silently corrupted by a
	// field this build was supposed to pass through untouched. Found by
	// adversarial review — splicing "ID":99999 onto a record with id 1 read
	// back as id 99999. Exact-case lookups make an unknown key exactly and
	// only an Extra key, whatever it is named.
	var all map[string]json.RawMessage
	if err := json.Unmarshal(data, &all); err != nil {
		return err
	}
	take := func(key string, dst any) error {
		raw, ok := all[key]
		if !ok {
			return nil
		}
		delete(all, key)
		return json.Unmarshal(raw, dst)
	}
	out := StoredNote{}
	for key, dst := range map[string]any{
		"id": &out.ID, "k": &out.Kind, "v": &out.VersionID, "b": &out.Book,
		"c": &out.Chapter, "lo": &out.VerseLo, "hi": &out.VerseHi,
		"ar": &out.AnchorRuns,
		"t":  &out.Text, "m": &out.Minimized, "ts": &out.Received,
		"sn": &out.SenderName, "sid": &out.SenderID,
		"ws": &out.WireSkipped, "wo": &out.WireOpaque,
	} {
		if err := take(key, dst); err != nil {
			return err
		}
	}
	if len(all) == 0 {
		all = nil
	}
	out.Extra = all
	*n = out
	return nil
}

// span builds the highlight span for a note, in the numbering the note itself
// carries — for a followed note that is the renumbered location the derive
// produced, still labelled with the translation it is stored under.
func (n StoredNote) span() VerseSpan {
	return VerseSpan{
		VersionID: n.VersionID,
		Book:      n.Book,
		Chapter:   n.Chapter,
		Lo:        n.VerseLo,
		Hi:        n.VerseHi,
	}
}

// sameNoteContent is the duplicate test: the content tuple — passage,
// translation and words — and NEVER the payload bytes (spec rule 2's
// corollary: two encodings of one note are one note). Not the timestamp, which
// the sending device stamps and which would make every re-share a new note;
// not the ID, which is local. Same tuple + same Kind = same note (dedup is
// applied per kind by the store).
func sameNoteContent(a, b StoredNote) bool {
	return a.VersionID == b.VersionID &&
		a.Book == b.Book &&
		a.Chapter == b.Chapter &&
		a.VerseLo == b.VerseLo &&
		a.VerseHi == b.VerseHi &&
		strings.TrimSpace(a.Text) == strings.TrimSpace(b.Text)
}
