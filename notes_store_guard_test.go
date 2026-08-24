package bibletext

import (
	"strings"
	"testing"
)

// notesBlobStore is a prefStore that holds exactly what a real one would: the
// raw string under the store key. The point of these tests is what ends up IN
// that string, so nothing here may inspect a parsed side-copy — a fake that
// stored notes as a map would pass even if the production code wrote rubbish.
type notesBlobStore struct {
	raw    string
	writes int
}

func (s *notesBlobStore) String(key string) string {
	if key != prefNotesStore {
		return ""
	}
	return s.raw
}

func (s *notesBlobStore) SetString(key, v string) {
	if key != prefNotesStore {
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

// A store whose value yields NOTHING recognisable must survive every mutation
// untouched — the whole-store stand-down. (Per-LINE damage beside readable
// records is the quarantine's business instead, and is writable; see
// TestJunkLinesAreQuarantinedNotShownAndNotDestroyed.) Before the guard, a
// failed read answered "no notes", and the reader's own next tap serialised
// that emptiness over their entire collection.
func TestUnreadableNotesStoreIsNeverOverwritten(t *testing.T) {
	const corrupt = `{"id":1,"k":"received","v":"web","b":"John","c":3,"t":"truncated mid-w`

	for _, tc := range []struct {
		name string
		act  func(p prefStore)
	}{
		{"addNote", func(p prefStore) {
			addNote(p, StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "Psalms", Chapter: 23, Text: "new one"})
		}},
		{"deleteNoteByID", func(p prefStore) { deleteNoteByID(p, 1) }},
		{"setNoteMinimizedByID", func(p prefStore) { setNoteMinimizedByID(p, 1, true) }},
		{"deleteAllNotes", func(p prefStore) { deleteAllNotes(p) }},
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
	addNote(s, StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "Psalms", Chapter: 23, Text: "first note"})
	if s.writes != 1 {
		t.Fatalf("writes = %d, want 1 — an empty store must be writable", s.writes)
	}
	notes := readNoteStore(s).notes
	if len(notes) != 1 || notes[0].Book != "Psalms" {
		t.Fatalf("stored %+v, want the one Psalms note (blob %q)", notes, s.raw)
	}
}

// And a readable store with real notes must still round-trip: the guard is
// about refusing BAD reads, not about refusing to work.
func TestReadableNotesStoreRoundTrips(t *testing.T) {
	s := &notesBlobStore{}
	one, _ := addNote(s, StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "John", Chapter: 3, Text: "one"})
	addNote(s, StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "Psalms", Chapter: 23, Text: "two"})
	if got := len(readNoteStore(s).notes); got != 2 {
		t.Fatalf("stored %d notes, want 2 (blob %q)", got, s.raw)
	}
	deleteNoteByID(s, one.ID)
	notes := readNoteStore(s).notes
	if len(notes) != 1 {
		t.Fatalf("after deleting one, %d remain, want 1", len(notes))
	}
	if notes[0].Book != "Psalms" {
		t.Error("deleting John 3 also removed Psalms 23")
	}
	if strings.Contains(s.raw, `"John"`) {
		t.Errorf("the deleted note is still in the stored blob: %q", s.raw)
	}
}

// The pre-S5 stores migrate into the scrapbook store at first read: the old
// map's entries become Kind=received records, the old own-notes list becomes
// Kind=mine records, and the legacy keys are cleared ONLY after the new
// store's write is verified by reading it back.
func TestLegacyStoresMigrateOnFirstRead(t *testing.T) {
	p := newNotePrefs()
	p.m[prefLegacySharedNotes] = `[{"v":"web","b":"John","c":3,"lo":16,"t":"fixture received alpha","ts":100,"m":true,"sn":"Fixture Sender","sid":"010203040506"},` +
		`{"v":"bsb","b":"Psalms","c":23,"t":"fixture received beta","ts":200}]`
	p.m[prefLegacyMyNotes] = `[{"v":"web","b":"Romans","c":8,"lo":28,"t":"fixture outgoing alpha","ts":300,"me":true}]`

	all := allNotesForBrowsing(p)
	if len(all) != 3 {
		t.Fatalf("migrated %d notes, want 3: %+v", len(all), all)
	}
	var friend, own StoredNote
	for _, n := range all {
		switch n.Text {
		case "fixture received alpha":
			friend = n
		case "fixture outgoing alpha":
			own = n
		}
	}
	if friend.Kind != noteKindReceived || !friend.Minimized || friend.Received != 100 ||
		friend.SenderName != "Fixture Sender" || friend.SenderID != "010203040506" || friend.VerseLo != 16 {
		t.Errorf("the received note lost fields in migration: %+v", friend)
	}
	if own.Kind != noteKindMine || own.Received != 300 {
		t.Errorf("the own note did not migrate as Kind=mine: %+v", own)
	}
	if friend.ID == 0 || own.ID == 0 {
		t.Error("migrated notes were not given identities")
	}
	if p.m[prefLegacySharedNotes] != "" || p.m[prefLegacyMyNotes] != "" {
		t.Error("the legacy keys were not cleared after a verified write")
	}
	// Idempotent: a restored backup re-introducing the old blobs must not
	// duplicate what already migrated (dedup: content tuple + kind).
	p.m[prefLegacySharedNotes] = `[{"v":"web","b":"John","c":3,"lo":16,"t":"fixture received alpha","ts":100}]`
	if got := len(allNotesForBrowsing(p)); got != 3 {
		t.Errorf("re-migration duplicated notes: %d, want 3", got)
	}
}

// A legacy blob that will not parse is quarantined into the new store
// VERBATIM — best-effort migration never drops bytes it cannot read — and the
// old key is still cleared, because the bytes now live in the quarantine.
func TestUnreadableLegacyBlobIsQuarantinedNotDropped(t *testing.T) {
	p := newNotePrefs()
	corrupt := `[{"v":"web","b":"John","c":3,"t":"truncated mid-w`
	p.m[prefLegacySharedNotes] = corrupt

	if got := len(allNotesForBrowsing(p)); got != 0 {
		t.Fatalf("an unreadable legacy blob produced %d notes", got)
	}
	if !strings.Contains(p.m[prefNotesStore], corrupt) {
		t.Errorf("the unreadable legacy bytes were dropped, not quarantined:\n%q", p.m[prefNotesStore])
	}
	if p.m[prefLegacySharedNotes] != "" {
		t.Error("the legacy key was left behind after its bytes were quarantined")
	}
	// And the quarantined bytes survive later writes.
	addNote(p, StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "Psalms", Chapter: 23, Text: "new"})
	if !strings.Contains(p.m[prefNotesStore], corrupt) {
		t.Error("a later write destroyed the quarantined legacy bytes")
	}
}
