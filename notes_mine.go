package bibletext

import (
	"encoding/json"
	"strings"
	"time"
)

// The notes YOU send.
//
// "Share with note" used to keep nothing: the moment the share sheet closed,
// your own words existed only in the messenger thread. They are kept now — but
// deliberately NOT in the same place as the notes people send you, and
// deliberately not shown in the scripture text (owner directive: stored, and
// visible in the notes list, but the reading page stays the other person's).
//
// WHY A SEPARATE STORE, AND WHY A LIST.
//
// The received-notes store is a MAP keyed by version|book|chapter, and that key
// holds exactly one note. Writing your own notes into it would have meant two
// different kinds of loss, both silent and both of the one kind of data that
// exists nowhere else:
//
//   - sharing a note on a chapter where a FRIEND'S note already lives would
//     overwrite their message with yours;
//   - sharing a second note on the same chapter would overwrite your first.
//
// A list has no key to collide on. Nothing here is ever looked up by passage —
// own notes are never derived onto a reading page — so the map's whole purpose
// is absent and its cost is not. Appending is also the shape the scrapbook
// store takes in S5 (docs/NOTES_SCRAPBOOK.md), so this is the direction of
// travel rather than a detour.
const prefMyNotes = "notes.mine"

// myNotesMax bounds the list. Unlike the received store's cap this one is not a
// data-loss risk in the same way — these are your own words and the message you
// sent them in still has them — but it is still a real limit and the oldest go
// first, which is the least surprising rule when the alternative is unbounded
// growth in a preferences blob.
const myNotesMax = 500

// readMyNotes returns the notes you have sent, oldest first.
//
// Unparseable content yields NOTHING rather than an error, and — importantly —
// the caller must not then write, or the bad blob becomes a good empty one.
// Every writer below re-reads and appends, so a blob that will not parse is
// left alone rather than replaced.
func readMyNotes(p prefStore) ([]SharedNote, bool) {
	if p == nil {
		return nil, true
	}
	raw := strings.TrimSpace(p.String(prefMyNotes))
	if raw == "" {
		return nil, true
	}
	var out []SharedNote
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, false
	}
	kept := out[:0]
	for _, n := range out {
		if n.Book == "" || n.Chapter < 1 || strings.TrimSpace(n.Text) == "" {
			continue
		}
		n.Mine = true // whatever the blob says, everything here is yours
		kept = append(kept, n)
	}
	return kept, true
}

// saveMyNote appends a note you just sent.
//
// DUPLICATES ARE MEASURED BY EVERYTHING BEING IDENTICAL (owner). Two notes are
// the same note when the passage, the translation and the words all match —
// deliberately NOT the timestamp, which the sending device stamps and which
// would therefore make every re-share a new note. Sharing the same words on the
// same verse twice is one note; sharing different words is two, even on the
// same verse, which is the whole reason this is a list.
func saveMyNote(p prefStore, n SharedNote) {
	if p == nil || n.Book == "" || n.Chapter < 1 || strings.TrimSpace(n.Text) == "" {
		return
	}
	existing, ok := readMyNotes(p)
	if !ok {
		return // unreadable: stand down rather than overwrite it
	}
	n.Mine = true
	if n.Received == 0 {
		n.Received = time.Now().Unix()
	}
	for _, e := range existing {
		if sameNoteContent(e, n) {
			return
		}
	}
	existing = append(existing, n)
	if len(existing) > myNotesMax {
		existing = existing[len(existing)-myNotesMax:]
	}
	blob, err := json.Marshal(existing)
	if err != nil {
		return
	}
	p.SetString(prefMyNotes, string(blob))
}

// sameNoteContent is the duplicate test: everything about the note itself,
// nothing about when it happened to arrive or how the reader has since chosen
// to display it.
func sameNoteContent(a, b SharedNote) bool {
	return a.VersionID == b.VersionID &&
		a.Book == b.Book &&
		a.Chapter == b.Chapter &&
		a.VerseLo == b.VerseLo &&
		a.VerseHi == b.VerseHi &&
		strings.TrimSpace(a.Text) == strings.TrimSpace(b.Text)
}

// deleteMyNotes empties the list. Settings → delete all notes clears BOTH
// stores, because to the reader there is one thing called "your notes" and a
// control that left half of them behind would be lying (owner directive).
func deleteMyNotes(p prefStore) {
	if p == nil {
		return
	}
	p.SetString(prefMyNotes, "")
}

// allNotesForBrowsing returns every note the app holds — received and sent —
// newest first, for the notes list.
//
// One list, mixed, because a scrapbook records an EXCHANGE: "I sent her Psalm
// 34 in March, she sent me Romans 8 in June" is the story, and splitting the
// two halves into separate screens leaves two lists that each answer half a
// question. They are told apart by the byline, not by their location. See
// docs/NOTES_SCRAPBOOK.md.
func allNotesForBrowsing(p prefStore) []SharedNote {
	var out []SharedNote
	for _, n := range readNotes(p) {
		out = append(out, n)
	}
	mine, _ := readMyNotes(p)
	out = append(out, mine...)
	return out // the browser owns the ordering (sortedNotes)
}

// noteByline is who the note is from, for display. Untrusted names do not
// appear here yet: there is no name field on the share sheet, so a received
// note can only say "Friend". SenderName is carried and stored and simply not
// read — see docs/NOTES_SCRAPBOOK.md, "Identity".
func noteByline(n SharedNote) string {
	if n.Mine {
		return "From you"
	}
	return "From Friend"
}
