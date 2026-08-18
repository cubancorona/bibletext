package bibletext

// The anchor resolver, pinned on the MEASURED cases from
// docs/NOTES_SCRAPBOOK.md — each row here is a fact that was enumerated over
// the real caches, not a hypothetical. The two rows that matter most are the
// ones where MapVerse answers confidently and wrongly: Tobit "maps exactly"
// into a WEB that does not contain it, and an unknown translation id claims
// exact placement everywhere. Book existence must override the table.

import (
	"reflect"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
)

// anchorTestBible is a reading-translation fixture with just enough text for
// the existence tests: the books the cases read IN, with the chapters they
// ask about. Tobit is deliberately not here — no 66-book canon has it — and
// neither is any book the cases do not name.
func anchorTestBible() *BibleData {
	bd := NewBibleData()
	add := func(book string, chapters ...int) {
		bd.Verses[book] = map[int][]Verse{}
		for _, c := range chapters {
			bd.Verses[book][c] = []Verse{{BookName: book, Chapter: c, Verse: 1, Text: "text"}}
		}
	}
	add("Romans", 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16)
	add("Mark", 9)
	add("John", 3)
	add("Esther", 4)
	add("Daniel", 3)
	return bd
}

func TestResolveNoteAnchorMeasuredCases(t *testing.T) {
	bible := anchorTestBible()
	note := func(vid, book string, chapter, lo, hi int) StoredNote {
		return StoredNote{Kind: noteKindReceived, VersionID: vid, Book: book,
			Chapter: chapter, VerseLo: lo, VerseHi: hi, Text: "x"}
	}
	for _, tc := range []struct {
		name    string
		note    StoredNote
		reading string
		want    placement
	}{
		{
			// The only cross-chapter mapping in the shipping translations: 24
			// cases, all the Romans doxology. The WHOLE span leaves chapter 14,
			// so nothing is here and the note lives on 16.
			name: "doxology, web read in bsb",
			note: note("web", "Romans", 14, 24, 26), reading: "bsb",
			want: placement{Kind: placedOtherChapter, Elsewhere: []anchorRun{{Chapter: 16, Lo: 25, Hi: 27}}},
		},
		{
			name: "doxology, bsb read in web",
			note: note("bsb", "Romans", 16, 25, 27), reading: "web",
			want: placement{Kind: placedOtherChapter, Elsewhere: []anchorRun{{Chapter: 14, Lo: 24, Hi: 26}}},
		},
		{
			// A span with a HOLE: MapVerse(web->bsb, Mark 9:43/44/45/46) =
			// exact/absent/exact/absent. Only a set can say [43,43],[45,45];
			// the holes stay OUT of Here.
			name: "Mark 9:43-46 partial, web read in bsb",
			note: note("web", "Mark", 9, 43, 46), reading: "bsb",
			want: placement{Kind: placedPartial, Here: []anchorRun{{Chapter: 9, Lo: 43, Hi: 43}, {Chapter: 9, Lo: 45, Hi: 45}}},
		},
		{
			// THE TABLE LIES: MapVerse(webc->web, Tobit 1:1) = 1:1 EXACT,
			// because versificationDeltas has no "web" entry. Book existence
			// against the loaded canon must override it.
			name: "Tobit, webc read in web: no such book, whatever the table says",
			note: note("webc", "Tobit", 1, 1, 0), reading: "web",
			want: placement{Kind: unplacedNoBook},
		},
		{
			// An unknown translation id "maps exactly" everywhere. It must NOT
			// claim exact placement — it degrades to the unplaced arm that book
			// existence dictates.
			name: "unknown translation id degrades by book existence",
			note: note("esv", "Tobit", 1, 1, 0), reading: "web",
			want: placement{Kind: unplacedNoBook},
		},
		{
			// ...and where the book IS present, the reference-numbering default
			// stands — the right default for every translation ever shipped,
			// and the wire's 'a' record is the honest fix for future ones.
			name: "unknown translation id, book present",
			note: note("esv", "Romans", 16, 25, 0), reading: "bsb",
			want: placement{Kind: placedExact, Here: []anchorRun{{Chapter: 16, Lo: 25, Hi: 25}}},
		},
		{
			// Greek Esther is a different book, not a renumbering.
			name: "Greek Esther incommensurable",
			note: note("web", "Esther", 4, 1, 0), reading: "webc",
			want: placement{Kind: unplacedIncommensurable},
		},
		{
			// The reading translation IS the note's own: home, byte-exact, no
			// mapping, no existence test — even where the tables would call the
			// book incommensurable from anywhere else.
			name: "same version is native",
			note: note("web", "Esther", 4, 1, 0), reading: "web",
			want: placement{Kind: placedNative, Here: []anchorRun{{Chapter: 4, Lo: 1}}},
		},
		{
			name: "chapter-level note follows",
			note: note("web", "John", 3, 0, 0), reading: "bsb",
			want: placement{Kind: placedExact, Here: []anchorRun{{Chapter: 3}}},
		},
		{
			// WEBC's Daniel 13 (Susanna) does not exist in the WEB at all: a
			// chapter-level note resolves to the chapter, and the chapter is
			// not there.
			name: "chapter-level note on a chapter this translation lacks",
			note: note("webc", "Daniel", 13, 0, 0), reading: "web",
			want: placement{Kind: unplacedAbsent},
		},
		{
			// Present here under different numbers, same chapter: the Song of
			// the Three pushes WEB Daniel 3:24-30 to WEBC's 3:91-97.
			name: "renumbered in-chapter is moved",
			note: note("web", "Daniel", 3, 24, 30), reading: "webc",
			want: placement{Kind: placedMoved, Here: []anchorRun{{Chapter: 3, Lo: 91, Hi: 97}}},
		},
		{
			// A span straddling the doxology boundary: part stays on 14, part
			// moves to 16. Kind is placedMoved and the spill has somewhere to
			// go — Elsewhere — instead of degrading to the verse it starts at.
			name: "span straddling the chapter boundary keeps its spill",
			note: note("web", "Romans", 14, 23, 24), reading: "bsb",
			want: placement{Kind: placedMoved,
				Here:      []anchorRun{{Chapter: 14, Lo: 23, Hi: 23}},
				Elsewhere: []anchorRun{{Chapter: 16, Lo: 25, Hi: 25}}},
		},
		{
			name: "every verse absent",
			note: note("web", "Mark", 9, 44, 0), reading: "bsb",
			want: placement{Kind: unplacedAbsent},
		},
	} {
		got := resolveNoteAnchor(tc.note, tc.reading, bible)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s:\n got %v %+v\nwant %v %+v", tc.name, got.Kind, got, tc.want.Kind, tc.want)
		}
	}
}

// The eight arms split exactly four/four across "puts something on the page
// here" — and placedOtherChapter is on the FALSE side: nothing is drawn on the
// chapter the note is filed against, the note appears on the other one.
func TestPlacedPredicate(t *testing.T) {
	want := map[placementKind]bool{
		placedNative: true, placedExact: true, placedMoved: true, placedPartial: true,
		placedOtherChapter: false, unplacedAbsent: false,
		unplacedIncommensurable: false, unplacedNoBook: false,
	}
	for k, w := range want {
		if k.placed() != w {
			t.Errorf("%v.placed() = %v, want %v", k, k.placed(), w)
		}
	}
}

// Eight arms, THREE sentences (owner approved three): one per unplaced arm,
// nothing for any placed arm, and nothing for placedOtherChapter — the derive
// already shows that note on the other chapter, so there is nothing to
// apologise for. Same rules as the S4 wire notices: quiet, attributed to
// nobody, no call to action.
func TestPlacementCopyIsThreeQuietSentences(t *testing.T) {
	sentences := map[string]bool{}
	for _, k := range []placementKind{unplacedNoBook, unplacedIncommensurable, unplacedAbsent} {
		s := placementCopy(k)
		if s == "" {
			t.Errorf("%v has no sentence", k)
			continue
		}
		sentences[s] = true
		if strings.Contains(s, "\n") {
			t.Errorf("%v's sentence is not a single line: %q", k, s)
		}
		if strings.Contains(strings.ToLower(s), "bibletext") || strings.Contains(s, "http") {
			t.Errorf("%v's sentence is branded or carries a link: %q", k, s)
		}
	}
	if len(sentences) != 3 {
		t.Errorf("the three unplaced arms share sentences: %d distinct", len(sentences))
	}
	for _, k := range []placementKind{placedNative, placedExact, placedMoved, placedPartial, placedOtherChapter} {
		if s := placementCopy(k); s != "" {
			t.Errorf("%v has copy %q; a placed note needs no apology and placedOtherChapter's answer is the derive", k, s)
		}
	}
}

// The wire's 'a' record is filed WHOLE — every run, not the first — and the
// full set round-trips the store: bytes out, bytes back, resolver consuming
// what came back. VerseLo/VerseHi still carry the first run for everything
// that already reads them.
func TestWireRunSetIsFiledWholeAndRoundTripsTheStore(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())
	st := noticeState()
	// The reading data must CONTAIN Mark 9: the resolver now verifies a
	// destination chapter against the reading translation (a review finding —
	// without it, an anchor from an unknown table classified placedExact into
	// a chapter the canon lacks), and in production a reader cannot stand on a
	// chapter their translation does not have. The sample fixture has no Mark,
	// which is a state this call site cannot reach outside a test.
	st.Bible.Books = append(st.Bible.Books, "Mark")
	if st.Bible.Verses["Mark"] == nil {
		st.Bible.Verses["Mark"] = map[int][]Verse{}
	}
	for v := 43; v <= 46; v++ {
		st.Bible.Verses["Mark"][9] = append(st.Bible.Verses["Mark"][9],
			Verse{BookName: "Mark", Book: "Mark", Chapter: 9, Verse: v, Text: "sample"})
	}
	st.Bible.PrepareSearchIndex()

	// A hand-built payload with the run set the encoder cannot yet produce:
	// two runs, 43-43 and 45-46 — the shape a projection split writes.
	idx, ok := noteBookIndexOf("Mark")
	if !ok {
		t.Fatal("Mark is not in the wire canon")
	}
	blob := []byte{'r'}
	blob = append(blob, rec('a', []byte{2, 43, 43, 45, 46})...)
	blob = append(blob, rec('b', []byte{byte(idx)})...)
	blob = append(blob, rec('c', []byte{9})...)
	blob = append(blob, rec('t', []byte("mind the hole"))...)
	blob = append(blob, rec('v', []byte("web"))...)
	target, ok := ParseShareLink("https://bibletext.co.uk/web/mark/9/#v43-46&n=" + rawNotePayload(blob))
	if !ok {
		t.Fatal("link did not parse")
	}
	if target.NoteRuns != "43,45-46" {
		t.Fatalf("the full run set did not ride the target: %q", target.NoteRuns)
	}
	if target.NoteLo != 43 || target.NoteHi != 43 {
		t.Fatalf("first-run compatibility fields wrong: %d-%d", target.NoteLo, target.NoteHi)
	}

	applyShareTarget(st, target)
	stored, ok := findStoredNote(appPrefs(), "web", "Mark", 9)
	if !ok {
		t.Fatalf("note not filed under the wire anchor; store: %v", allNotesForBrowsing(appPrefs()))
	}
	wantRuns := []anchorRun{{Chapter: 9, Lo: 43}, {Chapter: 9, Lo: 45, Hi: 46}}
	if !reflect.DeepEqual(stored.AnchorRuns, wantRuns) {
		t.Errorf("filed runs %+v, want %+v", stored.AnchorRuns, wantRuns)
	}
	if stored.VerseLo != 43 || stored.VerseHi != 0 {
		t.Errorf("VerseLo/VerseHi must stay the first run: %d-%d", stored.VerseLo, stored.VerseHi)
	}

	// The bytes round-trip: a FRESH parse of the raw value — not the cache —
	// hands the resolver the same set.
	reread := readNoteStoreRaw(appPrefs())
	if len(reread.notes) != 1 || !reflect.DeepEqual(reread.notes[0].AnchorRuns, wantRuns) {
		t.Fatalf("runs did not survive the bytes: %+v", reread.notes)
	}

	// And the resolver consumes what came back: read in the BSB, the set
	// resolves to the measured partial — [43,43] and [45,45], 46 being one of
	// the BSB's omissions — and the arity-1 derive mirrors the first run.
	pl := resolveNoteAnchor(reread.notes[0], "bsb", nil)
	wantHere := []anchorRun{{Chapter: 9, Lo: 43, Hi: 43}, {Chapter: 9, Lo: 45, Hi: 45}}
	if pl.Kind != placedPartial || !reflect.DeepEqual(pl.Here, wantHere) {
		t.Errorf("stored runs resolved to %v %+v, want partial %+v", pl.Kind, pl.Here, wantHere)
	}
	n, ok := noteForChapter(appPrefs(), "bsb", "Mark", 9, st.Bible)
	if !ok || n.VerseLo != 43 || n.VerseHi != 43 || n.ID != stored.ID {
		t.Errorf("derive mirror = lo %d hi %d id %d (ok=%v), want 43 43 %d", n.VerseLo, n.VerseHi, n.ID, ok, stored.ID)
	}
}

// The wire's b and c records outrank the path for what is FILED, exactly as
// the S4 commit said they would once the store could hold the full anchor:
// the path names where the page opens, the record names what the sender wrote
// about, and the note belongs to the second.
func TestWireBookAndChapterAreAuthoritativeForFiling(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())
	st := noticeState()

	payload := EncodeNoteWire(NoteWire{
		Text: "the doxology sits at the end here", Version: "bsb",
		Book: "Romans", Chapter: 16, VerseLo: 25,
	})
	target, ok := ParseShareLink("https://bibletext.co.uk/web/john/3/#v16&n=" + payload)
	if !ok {
		t.Fatal("link did not parse")
	}
	applyShareTarget(st, target)

	stored, ok := findStoredNote(appPrefs(), "bsb", "Romans", 16)
	if !ok {
		t.Fatalf("note not filed under the wire's b/c; store: %v", allNotesForBrowsing(appPrefs()))
	}
	if stored.VerseLo != 25 {
		t.Errorf("filed verse %d, want the wire run's 25", stored.VerseLo)
	}
	if _, misfiled := findStoredNote(appPrefs(), "web", "John", 3); misfiled {
		t.Error("note ALSO filed under the path's passage")
	}
	if st.NoteID != stored.ID {
		t.Errorf("live mirror addresses id %d, want %d — the verbs would miss", st.NoteID, stored.ID)
	}
}

// A hostile run cannot buy unbounded resolution work: parseNoteRuns admits
// verse numbers up to 2^31, and the walk must clamp rather than honour them —
// the inflateBytes discipline, applied to a count instead of a length.
func TestResolveClampsAHostileSpan(t *testing.T) {
	n := StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "John",
		Chapter: 3, VerseLo: 1, VerseHi: 1 << 30, Text: "x"}
	pl := resolveNoteAnchor(n, "bsb", nil) // must return promptly
	if pl.Kind != placedExact {
		t.Errorf("clamped span resolved to %v, want exact over the clamped walk", pl.Kind)
	}
	total := 0
	for _, r := range pl.Here {
		hi := r.Hi
		if hi < r.Lo {
			hi = r.Lo
		}
		total += hi - r.Lo + 1
	}
	if total > anchorWalkCap {
		t.Errorf("the walk honoured a hostile span: %d verses resolved", total)
	}
}
