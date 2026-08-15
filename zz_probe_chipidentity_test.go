package bibletext

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

// ---- P1: struct shape -------------------------------------------------------

func TestZZProbeStructShape(t *testing.T) {
	rt := reflect.TypeOf(SharedNote{})
	t.Logf("SharedNote NumField=%d", rt.NumField())
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		t.Logf("  %-10s %-8s json=%q", f.Name, f.Type, f.Tag.Get("json"))
	}
}

// ---- P2: the versification facts the derive rides on ------------------------

func TestZZProbeMapVerseRomans(t *testing.T) {
	for _, c := range []struct {
		from, to string
		ch, v    int
	}{
		{"bsb", "web", 16, 25},
		{"bsb", "web", 16, 26},
		{"bsb", "web", 16, 27},
		{"bsb", "web", 14, 23},
		{"bsb", "web", 14, 1},
		{"webc", "web", 16, 25},
		{"nkjv", "web", 16, 25},
	} {
		ch, v, res := MapVerse(c.from, c.to, "Romans", c.ch, c.v)
		t.Logf("MapVerse(%s->%s, Romans %d:%d) = %d:%d (%s)", c.from, c.to, c.ch, c.v, ch, v, res)
	}
}

// ---- P3: THE CLAIM UNDER TEST ----------------------------------------------
// "VersionID is the only free coordinate in the key, so it is unique within a
// chapter by type, not by luck." / "a chapter can show at most four chips today"
//
// Driven through the REAL noteFromAnotherTranslation, by asking it repeatedly
// and removing the winner each round — which is exactly the enumeration a
// set-shaped derive would do over the same accept test (notes_store.go:209-246).
func TestZZProbeTwoNotesSameTranslationOneChapter(t *testing.T) {
	notes := map[string]SharedNote{}
	add := func(n SharedNote) { notes[n.key()] = n }

	// Two notes, BOTH stored under bsb, in DIFFERENT chapters of Romans.
	add(SharedNote{VersionID: "bsb", Book: "Romans", Chapter: 14, VerseLo: 23,
		Text: "note A, stored bsb Romans 14", Received: 2000})
	add(SharedNote{VersionID: "bsb", Book: "Romans", Chapter: 16, VerseLo: 25,
		Text: "note B, stored bsb Romans 16", Received: 1000})
	// A third under webc, also in Romans 16, to test the ceiling.
	add(SharedNote{VersionID: "webc", Book: "Romans", Chapter: 16, VerseLo: 25,
		Text: "note C, stored webc Romans 16", Received: 500})
	// And a fourth under nkjv on Romans 14.
	add(SharedNote{VersionID: "nkjv", Book: "Romans", Chapter: 14, VerseLo: 23,
		Text: "note D, stored nkjv Romans 14", Received: 400})

	t.Logf("stored keys = %d", len(notes))
	for k := range notes {
		t.Logf("  key %q", k)
	}

	work := map[string]SharedNote{}
	for k, v := range notes {
		work[k] = v
	}
	placed := 0
	for {
		n, ok := noteFromAnotherTranslation(work, "web", "Romans", 14)
		if !ok {
			break
		}
		placed++
		t.Logf("PLACED #%d into WEB Romans 14: VersionID=%q storedChapter(key)=%q outChapter=%d VerseLo=%d text=%q",
			placed, n.VersionID, "", n.Chapter, n.VerseLo, n.Text)
		// remove by ORIGINAL key (VersionID untouched by the derive; Chapter is
		// rewritten, so rebuild the key from the source entry we can identify).
		var delKey string
		for k, v := range work {
			if v.Text == n.Text {
				delKey = k
			}
		}
		delete(work, delKey)
		if placed > 10 {
			t.Fatal("runaway")
		}
	}
	t.Logf("TOTAL placeable in ONE chapter (WEB Romans 14) = %d", placed)
}

// Same thing through the shipping public entry point loadNote + the real store,
// to rule out "you drove a helper the app never calls this way".
func TestZZProbeLoadNoteRealStore(t *testing.T) {
	p := newFakePrefs()
	saveNote(p, SharedNote{VersionID: "bsb", Book: "Romans", Chapter: 14, VerseLo: 23,
		Text: "note A (bsb Romans 14)", Received: 2000})
	saveNote(p, SharedNote{VersionID: "bsb", Book: "Romans", Chapter: 16, VerseLo: 25,
		Text: "note B (bsb Romans 16)", Received: 1000})
	t.Logf("blob = %s", p.String(prefSharedNotes))

	n1, ok1 := loadNote(p, "web", "Romans", 14)
	t.Logf("loadNote(web, Romans 14) #1 -> ok=%v version=%q ch=%d lo=%d text=%q", ok1, n1.VersionID, n1.Chapter, n1.VerseLo, n1.Text)

	deleteNote(p, "bsb", "Romans", 14) // bin the one in front
	n2, ok2 := loadNote(p, "web", "Romans", 14)
	t.Logf("loadNote(web, Romans 14) #2 -> ok=%v version=%q ch=%d lo=%d text=%q", ok2, n2.VersionID, n2.Chapter, n2.VerseLo, n2.Text)

	t.Logf("BOTH under version %q? %v", "bsb", n1.VersionID == "bsb" && n2.VersionID == "bsb")
}

// ---- P4: the "hard case unreachable" claim ---------------------------------

func TestZZProbeSameKeyOverwrite(t *testing.T) {
	p := newFakePrefs()
	saveNote(p, SharedNote{VersionID: "web", Book: "John", Chapter: 3, VerseLo: 16, Text: "FIRST person's note", Received: 100})
	saveNote(p, SharedNote{VersionID: "web", Book: "John", Chapter: 3, VerseLo: 16, Text: "SECOND person's note", Received: 200})
	var list []SharedNote
	_ = json.Unmarshal([]byte(p.String(prefSharedNotes)), &list)
	t.Logf("same version/book/chapter twice -> %d entries", len(list))
	for _, n := range list {
		t.Logf("  %+v", n)
	}

	// And a DIFFERENT verse in the same chapter/version?
	p2 := newFakePrefs()
	saveNote(p2, SharedNote{VersionID: "web", Book: "John", Chapter: 3, VerseLo: 16, Text: "on 16", Received: 100})
	saveNote(p2, SharedNote{VersionID: "web", Book: "John", Chapter: 3, VerseLo: 1, Text: "on 1", Received: 200})
	var list2 []SharedNote
	_ = json.Unmarshal([]byte(p2.String(prefSharedNotes)), &list2)
	t.Logf("same version/book/chapter, DIFFERENT verses -> %d entries", len(list2))
}

// ---- P5: Received stamping -------------------------------------------------

func TestZZProbeReceived(t *testing.T) {
	old := noteNow
	noteNow = func() int64 { return 1760000000 }
	defer func() { noteNow = old }()

	p := newFakePrefs()
	saveNote(p, SharedNote{VersionID: "web", Book: "John", Chapter: 3, Text: "x"})
	n, _ := loadNote(p, "web", "John", 3)
	t.Logf("saveNote with Received=0 -> stored Received=%d", n.Received)

	// legacy blob with no "ts"
	p2 := newFakePrefs()
	p2.SetString(prefSharedNotes, `[{"v":"web","b":"John","c":3,"t":"legacy"}]`)
	n2, ok := loadNote(p2, "web", "John", 3)
	t.Logf("legacy blob readable=%v Received=%d", ok, n2.Received)
	setNoteMinimized(p2, "web", "John", 3, true)
	n3, _ := loadNote(p2, "web", "John", 3)
	t.Logf("after setNoteMinimized -> Received=%d minimized=%v", n3.Received, n3.Minimized)
	saveNote(p2, SharedNote{VersionID: "web", Book: "John", Chapter: 3, Text: "legacy"})
	n4, _ := loadNote(p2, "web", "John", 3)
	t.Logf("after saveNote over legacy -> Received=%d", n4.Received)
}

// ---- P6: noteDateLabel vocabulary ------------------------------------------

func TestZZProbeDateLabel(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	for _, d := range []int{0, 1, 2, 3, 6, 7, 20, 400} {
		ts := now.AddDate(0, 0, -d).Unix()
		t.Logf("%3d days ago -> %q", d, noteDateLabel(ts, now))
	}
	t.Logf("ts=0 -> %q", noteDateLabel(0, now))
}

// ---- P7: normalizeNote -----------------------------------------------------

func TestZZProbeNormalize(t *testing.T) {
	for _, in := range []string{
		"<script>alert(1)</script>",
		"line one\nline two",
		"tab\there",
		"‮GNIHTEMOS‬",
		"a & b < c > d \"q\" 'p'",
	} {
		out := normalizeNote(in)
		t.Logf("in=%q -> out=%q  unchanged=%v", in, out, out == in)
	}
	t.Logf("NoteMaxRunes=%d", NoteMaxRunes)
}

// ---- P8: what versions can actually key the store --------------------------

func TestZZProbeVersionIDs(t *testing.T) {
	t.Logf("linkPathVersionIDs = %v", linkPathVersionIDs)
	t.Logf("noteKey lowercases: %q vs %q", noteKey("WEB", "John", 3), noteKey("web", "John", 3))
}
