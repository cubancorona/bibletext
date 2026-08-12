package bibletext

import (
	"encoding/json"
	"strings"
	"testing"
)

// notesBlobStore is a prefStore that holds exactly what a real one would: the
// raw string. The point of these tests is what ends up IN that string, so
// nothing here may inspect a parsed side-copy — a fake that stored notes as a
// map would pass even if the production code wrote rubbish.
type notesBlobStore struct {
	raw    string
	writes int
}

func (s *notesBlobStore) String(key string) string {
	if key != prefSharedNotes {
		return ""
	}
	return s.raw
}

func (s *notesBlobStore) SetString(key, v string) {
	if key != prefSharedNotes {
		return
	}
	s.raw = v
	s.writes++
}

func (s *notesBlobStore) Bool(string) bool                       { return false }
func (s *notesBlobStore) SetBool(string, bool)                   {}
func (s *notesBlobStore) Int(string) int                         { return 0 }
func (s *notesBlobStore) SetInt(string, int)                     {}
func (s *notesBlobStore) Float(string) float64                   { return 0 }
func (s *notesBlobStore) SetFloat(string, float64)               {}
func (s *notesBlobStore) RemoveValue(string)                     {}
func (s *notesBlobStore) StringWithFallback(_, f string) string  { return f }
func (s *notesBlobStore) BoolWithFallback(_ string, f bool) bool { return f }
func (s *notesBlobStore) IntWithFallback(_ string, f int) int    { return f }

// A store whose blob cannot be parsed must survive every mutation untouched.
// Before the guard, each of these calls read "no notes", then serialised that
// emptiness back — one unreadable read destroyed the lot, permanently, and the
// reader's own next tap was what did it.
//
// setNoteMinimized is deliberately NOT in this table. It looks up the note first
// and returns before writing when the key is absent, so an unreadable blob left
// it inert even before the guard — a subtest for it would pass with the guard
// removed and would therefore prove nothing. It still carries the check, because
// relying on an early return elsewhere in the function is not a property anyone
// should have to re-derive when editing it.
func TestUnreadableNotesBlobIsNeverOverwritten(t *testing.T) {
	const corrupt = `[{"v":"web","b":"John","c":3,"t":"truncated mid-w`

	for _, tc := range []struct {
		name string
		act  func(p prefStore)
	}{
		{"saveNote", func(p prefStore) {
			saveNote(p, SharedNote{VersionID: "web", Book: "Psalms", Chapter: 23, Text: "new one"})
		}},
		{"deleteNote", func(p prefStore) { deleteNote(p, "web", "John", 3) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &notesBlobStore{raw: corrupt}
			tc.act(s)
			if s.writes != 0 {
				t.Errorf("wrote to an unreadable store %d time(s); the blob is now %q", s.writes, s.raw)
			}
			if s.raw != corrupt {
				t.Errorf("blob changed:\n got %q\nwant %q", s.raw, corrupt)
			}
		})
	}
}

// The guard must not seize up a store that is merely EMPTY — that is the state
// every reader starts in, and refusing to write there would mean no note could
// ever be saved at all.
func TestEmptyNotesStoreStillAcceptsWrites(t *testing.T) {
	s := &notesBlobStore{}
	saveNote(s, SharedNote{VersionID: "web", Book: "Psalms", Chapter: 23, Text: "first note"})
	if s.writes != 1 {
		t.Fatalf("writes = %d, want 1 — an empty store must be writable", s.writes)
	}
	var list []SharedNote
	if err := json.Unmarshal([]byte(s.raw), &list); err != nil {
		t.Fatalf("stored blob does not parse: %v (%q)", err, s.raw)
	}
	if len(list) != 1 || list[0].Book != "Psalms" {
		t.Fatalf("stored %+v, want the one Psalms note", list)
	}
}

// And a readable store with real notes must still round-trip: the guard is
// about refusing BAD reads, not about refusing to work.
func TestReadableNotesStoreRoundTrips(t *testing.T) {
	s := &notesBlobStore{}
	saveNote(s, SharedNote{VersionID: "web", Book: "John", Chapter: 3, Text: "one"})
	saveNote(s, SharedNote{VersionID: "web", Book: "Psalms", Chapter: 23, Text: "two"})
	if got := len(readNotes(s)); got != 2 {
		t.Fatalf("stored %d notes, want 2 (blob %q)", got, s.raw)
	}
	deleteNote(s, "web", "John", 3)
	notes := readNotes(s)
	if len(notes) != 1 {
		t.Fatalf("after deleting one, %d remain, want 1", len(notes))
	}
	if _, still := notes[noteKey("web", "John", 3)]; still {
		t.Error("the deleted note is still there")
	}
	if _, kept := notes[noteKey("web", "Psalms", 23)]; !kept {
		t.Error("deleting John 3 also removed Psalms 23")
	}
	if strings.Contains(s.raw, `"John"`) {
		t.Errorf("the deleted note is still in the stored blob: %q", s.raw)
	}
}
