package bibletext

import (
	"encoding/base64"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
)

// fakePrefs is the in-memory prefStore the reading-state tests already rely on
// this package having; notes use the same seam.
type notePrefs struct{ m map[string]string }

func newNotePrefs() *notePrefs { return &notePrefs{m: map[string]string{}} }

func (p *notePrefs) String(k string) string { return p.m[k] }
func (p *notePrefs) SetString(k, v string)  { p.m[k] = v }
func (p *notePrefs) StringWithFallback(k, fallback string) string {
	if v, ok := p.m[k]; ok {
		return v
	}
	return fallback
}

// findStoredNote is the exact-filing lookup the old map key used to provide:
// the received note filed under version|book|chapter, if any. Display goes
// through noteForChapter; this asks where a note LIVES.
func findStoredNote(p prefStore, versionID, book string, chapter int) (StoredNote, bool) {
	for _, n := range readNoteStore(p).notes {
		if n.Kind == noteKindReceived && strings.EqualFold(n.VersionID, versionID) &&
			n.Book == book && n.Chapter == chapter {
			return n, true
		}
	}
	return StoredNote{}, false
}

func TestNoteSurvivesARoundTrip(t *testing.T) {
	p := newNotePrefs()
	n := StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "John", Chapter: 3, VerseLo: 16, VerseHi: 18,
		Text: "synthetic note — this one carried me. 🙏"}
	stored, ok := addNote(p, n)
	if !ok || stored.ID == 0 {
		t.Fatalf("the note was not stored: ok=%v id=%d", ok, stored.ID)
	}

	got, ok := noteForChapter(p, "web", "John", 3)
	if !ok {
		t.Fatal("the note did not come back")
	}
	if got.Text != n.Text || got.VerseLo != 16 || got.VerseHi != 18 {
		t.Errorf("came back changed: %+v", got)
	}
	if got.ID != stored.ID {
		t.Errorf("the note came back under a different identity: %d, stored as %d", got.ID, stored.ID)
	}
	if _, ok := noteForChapter(p, "web", "John", 4); ok {
		t.Error("a note leaked onto the next chapter")
	}
	// A note FOLLOWS the passage into another translation. It used to be
	// confined to the translation it arrived in, on the reasoning that a remark
	// is about particular wording — but the reader meets that rule as a note
	// that silently disappears when they change translation, and it disappears
	// in the ordinary case, not an exotic one: two people sharing a link often
	// read different translations, and a link shared FROM a licensed translation
	// comes back naming a published one, so it was the sender's own note that

	got2, ok := noteForChapter(p, "bsb", "John", 3)
	if !ok {
		t.Fatal("the note did not follow the passage into the other translation")
	}
	if got2.Text != n.Text {
		t.Errorf("the note changed on the way across: %q", got2.Text)
	}
	// VersionID keeps saying WEB — where the note actually lives — even though a
	// BSB reader is the one looking at it. Only the LOCATION is renumbered; the
	// IDENTITY rides along, which is what the verbs address.
	if got2.VersionID != "web" {
		t.Errorf("the note lost track of where it is stored: %q", got2.VersionID)
	}
	if got2.ID != stored.ID {
		t.Errorf("the followed note lost its identity: %d, want %d", got2.ID, stored.ID)
	}
}

// ...but only where the passage genuinely corresponds. Greek Esther is a
// different book from Esther, not a renumbering, so a note on one says nothing
// about the other — MapVerse calls that incommensurable and the note must stay
// where it is rather than being planted on unrelated text.
func TestANoteDoesNotFollowAnIncommensurablePassage(t *testing.T) {
	p := newNotePrefs()
	addNote(p, StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "Esther", Chapter: 4, VerseLo: 1,
		Text: "for such a time as this"})

	if _, ok := noteForChapter(p, "webc", "Esther", 4); ok {
		t.Error("a note crossed into Greek Esther, where its verse numbers mean something else")
	}
}

// Minimize must be remembered. If it is not, the note reappears on the reader's
// next visit as though they never touched it — the exact bug that makes people
// stop trusting a dismiss.
func TestMinimizeIsRemembered(t *testing.T) {
	p := newNotePrefs()
	stored, _ := addNote(p, StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "Psalms", Chapter: 23, VerseLo: 1, Text: "slowly"})

	setNoteMinimizedByID(p, stored.ID, true)
	if n, _ := noteForChapter(p, "web", "Psalms", 23); !n.Minimized {
		t.Error("minimize was not stored")
	}
	setNoteMinimizedByID(p, stored.ID, false)
	if n, _ := noteForChapter(p, "web", "Psalms", 23); n.Minimized {
		t.Error("restore was not stored")
	}
	// And the text survives both.
	if n, _ := noteForChapter(p, "web", "Psalms", 23); n.Text != "slowly" {
		t.Errorf("text lost through minimize/restore: %q", n.Text)
	}
}

func TestDeleteIsForGood(t *testing.T) {
	p := newNotePrefs()
	stored, _ := addNote(p, StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "John", Chapter: 3, Text: "x"})
	deleteNoteByID(p, stored.ID)
	if _, ok := noteForChapter(p, "web", "John", 3); ok {
		t.Error("the note came back after delete")
	}
}

// Two notes on the same passage are TWO notes — the store has no passage key
// to collide on. The old map silently destroyed the first (measured: 24% of a
// 200-note store, 72% of a 5,000-note one); the reading pane still shows one
// at a time (the newest), but both survive and the browser lists both.
func TestSecondNoteOnSamePassageKeepsBoth(t *testing.T) {
	p := newNotePrefs()
	origNow := noteNow
	noteNow = func() int64 { return 100 }
	defer func() { noteNow = origNow }()
	addNote(p, StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "John", Chapter: 3, Text: "first"})
	noteNow = func() int64 { return 200 }
	addNote(p, StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "John", Chapter: 3, Text: "second"})

	if got := len(allNotesForBrowsing(p)); got != 2 {
		t.Fatalf("expected both notes kept, got %d", got)
	}
	// The reading pane's arity-1 derive shows the newest.
	n, _ := noteForChapter(p, "web", "John", 3)
	if n.Text != "second" {
		t.Errorf("expected the newer note on the reading pane, got %q", n.Text)
	}
}

// ...and the SAME note arriving again is one note: dedup is the content tuple
// (sameNoteContent) plus the kind, never the timestamp — and the stored
// record's history (Received, Minimized) is preserved, not rewritten.
func TestReopeningTheSameNoteDoesNotDuplicateOrRewriteIt(t *testing.T) {
	p := newNotePrefs()
	origNow := noteNow
	noteNow = func() int64 { return 100 }
	defer func() { noteNow = origNow }()
	first, _ := addNote(p, StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "John", Chapter: 3, VerseLo: 16, Text: "same words"})
	setNoteMinimizedByID(p, first.ID, true)

	noteNow = func() int64 { return 9999 }
	again, ok := addNote(p, StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "John", Chapter: 3, VerseLo: 16, Text: "same words"})
	if !ok || again.ID != first.ID {
		t.Fatalf("the re-arrival minted a second note: %d then %d", first.ID, again.ID)
	}
	if again.Received != 100 || !again.Minimized {
		t.Errorf("the re-arrival rewrote the reader's history with the note: %+v", again)
	}
	if got := len(allNotesForBrowsing(p)); got != 1 {
		t.Errorf("expected one note, got %d", got)
	}
}

// NO CAP, NO EVICTION — eviction is a data-loss event (the old cap discarded
// by ALPHABETICAL ORDER of the storage key). And the blob must not churn when
// nothing changed: an unchanged store serialises to identical bytes.
func TestStoreIsUnboundedAndByteStable(t *testing.T) {
	p := newNotePrefs()
	for i := 1; i <= 250; i++ {
		addNote(p, StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "Psalms", Chapter: i, Text: "n"})
	}
	if got := len(allNotesForBrowsing(p)); got != 250 {
		t.Errorf("the store evicted: %d of 250 survive", got)
	}

	before := p.String(prefNotesStore)
	writeNoteStore(p, readNoteStore(p))
	if p.String(prefNotesStore) != before {
		t.Error("rewriting unchanged notes changed the blob — the file would churn on every navigation")
	}
}

// Junk in the store must not become junk on screen — and it must not be
// DESTROYED either: a bad line is quarantined verbatim and re-emitted on every
// write. The app never deletes what it cannot parse.
func TestJunkLinesAreQuarantinedNotShownAndNotDestroyed(t *testing.T) {
	p := newNotePrefs()
	good := `{"id":1,"k":"received","v":"web","b":"John","c":3,"t":"kept"}`
	junk := `{"id":2,"k":"received","v":"web","b":"John","c":3,"t":"truncated mid-w`
	p.m[prefNotesStore] = good + "\n" + junk

	if got := len(allNotesForBrowsing(p)); got != 1 {
		t.Fatalf("expected 1 readable note beside the junk, got %d", got)
	}
	// An unrelated mutation must carry the junk line through, byte for byte.
	addNote(p, StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "Psalms", Chapter: 23, Text: "new"})
	if !strings.Contains(p.m[prefNotesStore], junk) {
		t.Errorf("the quarantined line was destroyed by an unrelated write:\n%s", p.m[prefNotesStore])
	}
	if got := len(allNotesForBrowsing(p)); got != 2 {
		t.Errorf("expected 2 notes after the write, got %d", got)
	}
}

func TestNoteTextIsNotTrustedFromTheStore(t *testing.T) {
	p := newNotePrefs()
	hostile := "<script>alert(1)</script>"
	addNote(p, StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "John", Chapter: 3, Text: hostile})
	n, _ := noteForChapter(p, "web", "John", 3)
	// The store keeps text verbatim — escaping is the RENDERER's job, and this
	// pins that we are not quietly relying on the store to sanitise.
	if n.Text != hostile {
		t.Errorf("the store altered the text: %q", n.Text)
	}
	if strings.Contains(htmlEscape(n.Text), "<script>") {
		t.Error("the chapter-HTML escaper let a script tag through")
	}
}

// An ID, once issued, is NEVER reused: the counter is its own persisted key and
// deletion does not touch it, so later features always have something stable
// to point at and a verb can never reach a stranger through a recycled id.
func TestDeletedIDsAreNeverReissued(t *testing.T) {
	p := newNotePrefs()
	a, _ := addNote(p, StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "John", Chapter: 1, Text: "a"})
	b, _ := addNote(p, StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "John", Chapter: 2, Text: "b"})
	if b.ID <= a.ID {
		t.Fatalf("ids are not monotonic: %d then %d", a.ID, b.ID)
	}
	deleteNoteByID(p, b.ID)
	c, _ := addNote(p, StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "John", Chapter: 3, Text: "c"})
	if c.ID <= b.ID {
		t.Errorf("a deleted note's id was reissued: %d after deleting %d — max(existing)+1 is the bug this pins", c.ID, b.ID)
	}
}

// A record whose fields this build does not know must pass through a
// read-modify-write UNTOUCHED — spec rule 1 of the long-term foundation,
// tested with a field no current code knows.
func TestUnknownFieldsSurviveAReadModifyWrite(t *testing.T) {
	p := newNotePrefs()
	future := `{"id":7,"k":"received","v":"web","b":"John","c":3,"t":"hello","zz_future_field":{"nested":[1,2,3]},"zz_more":"kept"}`
	p.m[prefNotesStore] = future

	// An unrelated mutation: minimize a different, newly-arrived note.
	stored, _ := addNote(p, StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "Psalms", Chapter: 23, Text: "other"})
	setNoteMinimizedByID(p, stored.ID, true)

	blob := p.m[prefNotesStore]
	if !strings.Contains(blob, `"zz_future_field":{"nested":[1,2,3]}`) || !strings.Contains(blob, `"zz_more":"kept"`) {
		t.Errorf("a newer build's fields were destroyed by an older build's rewrite:\n%s", blob)
	}
	// And the known fields of that record still read.
	n, ok := noteForChapter(p, "web", "John", 3)
	if !ok || n.Text != "hello" || n.ID != 7 {
		t.Errorf("the future-field record did not read back: %+v ok=%v", n, ok)
	}
}

// deleteAllNotes writes the one-line header sentinel, never "": an empty value
// means "new reader", and a deliberate wipe must stay distinguishable from a
// value-level loss. The ID counter survives the wipe — ids are never reused.
func TestDeleteAllWritesTheWipedSentinelAndKeepsTheCounter(t *testing.T) {
	p := newNotePrefs()
	a, _ := addNote(p, StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "John", Chapter: 3, Text: "x"})

	deleteAllNotes(p)
	if got := p.m[prefNotesStore]; got != notesWipedSentinel {
		t.Errorf("delete-all wrote %q, want the sentinel %q", got, notesWipedSentinel)
	}
	if got := len(allNotesForBrowsing(p)); got != 0 {
		t.Errorf("notes survived delete-all: %d", got)
	}
	b, _ := addNote(p, StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "John", Chapter: 3, Text: "y"})
	if b.ID <= a.ID {
		t.Errorf("delete-all reset the id counter: %d after %d", b.ID, a.ID)
	}
}

// Spec rule 3, end to end: a payload record this build cannot use — an
// unknown wire tag from a future BibleText — must ride the link into the
// STORE, preserved verbatim, so a future forward/re-share can re-emit it.
func TestUnknownWireRecordsArePreservedIntoTheStore(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	// A hand-built record stream: t (text), v (version), then an unknown
	// lowercase tag 'x' — the append-only extension case.
	rec := func(tag byte, val []byte) []byte {
		out := []byte{tag, byte(len(val))}
		return append(out, val...)
	}
	var stream []byte
	stream = append(stream, rec('t', []byte("with a future field"))...)
	stream = append(stream, rec('v', []byte("web"))...)
	unknown := rec('x', []byte{0xDE, 0xAD, 0xBE, 0xEF})
	stream = append(stream, unknown...)
	payload := base64.RawURLEncoding.EncodeToString(append([]byte{'r'}, stream...))

	target, ok := ParseShareLink("https://bibletext.co.uk/web/john/3/#v16&n=" + payload)
	if !ok || target.Note != "with a future field" {
		t.Fatalf("link did not parse to the note: ok=%v %+v", ok, target)
	}
	if target.NoteSkipped != string(unknown) {
		t.Fatalf("the unknown record did not survive parsing: %q", target.NoteSkipped)
	}

	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	st := &AppState{Bible: bd, CurrentBook: "John", CurrentChapter: 3,
		CurrentVersion: "web", loadPhase: loadReady,
		loadedVersions: map[string]*BibleData{"web": bd}}
	applyShareTarget(st, target)

	stored, ok := findStoredNote(appPrefs(), "web", "John", 3)
	if !ok {
		t.Fatal("the note was not stored")
	}
	if string(stored.WireSkipped) != string(unknown) {
		t.Errorf("the unknown wire record did not reach the store verbatim: %x, want %x",
			stored.WireSkipped, unknown)
	}
	if st.NoteID != stored.ID {
		t.Errorf("the mirror does not address the stored note: %d vs %d", st.NoteID, stored.ID)
	}
}
