package bibletext

// What it costs to OPEN the notes browser, and what a keystroke in its search
// box costs once it is open.
//
// The browser is the one notes surface whose cost scales with the size of the
// scrapbook, so it is the one that needs a measured floor rather than an
// opinion. These benchmarks build a realistic store — hundreds of notes, both
// kinds, spread across the canon, carrying the anchor runs and preserved wire
// bytes a received note really holds — and then run the two paths a reader
// actually waits on:
//
//   - OPEN: buildNotesBrowseView from cold, which is what the speech-bubble
//     button in the sidebar runs.
//   - KEYSTROKE: the same build again with a query set, which is what every
//     character typed into "Search your notes…" runs.
//
// Run them with:
//
//	go test -run '^$' -bench 'BenchmarkNotes' -benchmem .
//
// WHAT THEY CAUGHT. The row used to be rebuilt on every list update, and a
// row carries a container.ThemeOverride whose constructor clears the
// process-wide font measurement cache (browseRow, notes_browse.go). Measured
// on one machine, before and after refilling pooled rows instead:
//
//	OpenAndLayout/100    729ms  616MB  5.44M allocs  ->  37ms  31MB  314k
//	OpenAndLayout/500    746ms  651MB  5.66M allocs  ->  38ms  33MB  326k
//	OpenAndLayout/2000   684ms  611MB  5.28M allocs  ->  42ms  39MB  342k
//	Scroll (500 notes)  58.5ms   42MB   363k allocs  -> 3.1ms 2.4MB   23k
//
// The absolute numbers are machine-specific and not worth pinning in an
// assertion; the SHAPE is the contract, and it is the thing to re-measure
// after touching the row or the list: opening stays in the tens of
// milliseconds, and it does not scale with the stored note count.

import (
	"strconv"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// benchCanonBooks is a 66-name canon for the sort's book-order map. Synthetic
// names, because the sort only cares about the ordering, not the spelling.
func benchCanonBooks() []string {
	books := make([]string, 0, 66)
	for i := 1; i <= 66; i++ {
		books = append(books, "Book "+strconv.Itoa(i))
	}
	return books
}

// benchNotesState is an AppState with a full-length canon loaded, so
// bookOrderOf builds a real map and the Bible-order sort does real lookups.
func benchNotesState() *AppState {
	return &AppState{
		Bible:          &BibleData{Books: benchCanonBooks()},
		CurrentBook:    "Book 19",
		CurrentChapter: 23,
		NotesMode:      true,
	}
}

// benchNoteText is a message of a plausible length — long enough that the row
// preview has something to cut and the search has something to scan.
func benchNoteText(i int) string {
	return "fixture note " + strconv.Itoa(i) + ": " +
		strings.Repeat("a sentence about this passage that someone actually wrote. ", 3)
}

// seedBenchNotes fills the store with n notes without going through addNote n
// times (which is a read-modify-write each, and would make setup quadratic).
// The bytes are the real serialisation, so the read path under measurement
// parses exactly what it would in the app.
func seedBenchNotes(p prefStore, n int) {
	s := &noteStore{ok: true}
	for i := 1; i <= n; i++ {
		note := StoredNote{
			ID:        uint64(i),
			Kind:      noteKindReceived,
			VersionID: "web",
			Book:      "Book " + strconv.Itoa(i%66+1),
			Chapter:   i%40 + 1,
			VerseLo:   i%20 + 1,
			VerseHi:   i%20 + 3,
			Text:      benchNoteText(i),
			Received:  1700000000 + int64(i)*3600,
			AnchorRuns: []anchorRun{
				{Lo: i%20 + 1, Hi: i%20 + 3},
				{Lo: i%20 + 6, Hi: i%20 + 7},
			},
			// A received note keeps what its link carried that this build could
			// not use. Every fifth note carries a payload of the size the store
			// really sees, so the benchmark's records are not artificially thin.
			WireSkipped: benchWireBytes(i),
		}
		if i%4 == 0 {
			note.Kind = noteKindMine
			note.WireSkipped = nil
		}
		s.notes = append(s.notes, note)
	}
	p.SetString(prefNotesStore, serializeNoteStore(s))
	p.SetString(prefNotesNextID, strconv.Itoa(n))
}

func benchWireBytes(i int) []byte {
	if i%5 != 0 {
		return nil
	}
	b := make([]byte, 4096)
	for j := range b {
		b[j] = byte('a' + (i+j)%26)
	}
	return b
}

// benchNotesApp starts a test app, seeds n notes, and returns a state that
// browses them. The store cache is dropped so the first measured iteration
// pays a cold parse, exactly as the first open after a navigation would.
func benchNotesApp(b *testing.B, n int) (*AppState, func()) {
	b.Helper()
	app := test.NewApp()
	setNotesEnabled(true)
	seedBenchNotes(app.Preferences(), n)
	noteStoreCacheRaw, noteStoreCache = "", nil
	return benchNotesState(), func() { app.Quit() }
}

var benchSizes = []int{100, 500, 2000}

// BenchmarkNotesBrowseOpen is the whole open path: read the store, filter,
// sort, and build the view the notes button shows.
func BenchmarkNotesBrowseOpen(b *testing.B) {
	for _, n := range benchSizes {
		b.Run(strconv.Itoa(n)+"notes", func(b *testing.B) {
			st, done := benchNotesApp(b, n)
			defer done()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sinkObject = buildNotesBrowseView(st)
			}
		})
	}
}

// BenchmarkNotesBrowseKeystroke is one character typed into the notes search
// box: the same build, with a query that keeps a fraction of the list.
func BenchmarkNotesBrowseKeystroke(b *testing.B) {
	for _, n := range benchSizes {
		b.Run(strconv.Itoa(n)+"notes", func(b *testing.B) {
			st, done := benchNotesApp(b, n)
			defer done()
			st.NotesQuery = "note 1"
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sinkObject = buildNotesBrowseView(st)
			}
		})
	}
}

// BenchmarkBrowsableNotes isolates the DATA half — store read, who-filter,
// sort, text match — from the widget half, so a regression can be attributed.
func BenchmarkBrowsableNotes(b *testing.B) {
	for _, n := range benchSizes {
		b.Run(strconv.Itoa(n)+"notes", func(b *testing.B) {
			st, done := benchNotesApp(b, n)
			defer done()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sinkNotes, sinkInt = browsableNotes(st)
			}
		})
	}
}

// BenchmarkNoteBrowseRow is one row, the unit the windowed list builds per
// visible note. It bounds what scrolling costs.
func BenchmarkNoteBrowseRow(b *testing.B) {
	st, done := benchNotesApp(b, 100)
	defer done()
	notes, _ := browsableNotes(st)
	pal := st.pal()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkObject = noteBrowseRow(st, notes[i%len(notes)], pal)
	}
}

// benchFindList walks a built view for the browser's windowed list, so a
// benchmark can scroll it the way a reader does.
func benchFindList(o fyne.CanvasObject) *widget.List {
	switch v := o.(type) {
	case *widget.List:
		return v
	case *fyne.Container:
		for _, c := range v.Objects {
			if l := benchFindList(c); l != nil {
				return l
			}
		}
	case fyne.Widget:
		for _, c := range test.WidgetRenderer(v).Objects() {
			if l := benchFindList(c); l != nil {
				return l
			}
		}
	}
	return nil
}

// BenchmarkNotesBrowseOpenAndLayout is the number a reader actually waits on:
// the view built AND laid out in a window, so the windowed list really creates
// and measures the rows for its viewport. Building alone understates the open
// because widget.List defers every row until it has a size.
func BenchmarkNotesBrowseOpenAndLayout(b *testing.B) {
	for _, n := range benchSizes {
		b.Run(strconv.Itoa(n)+"notes", func(b *testing.B) {
			st, done := benchNotesApp(b, n)
			defer done()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				w := test.NewWindow(nil)
				w.Resize(fyne.NewSize(620, 900))
				b.StartTimer()

				view := buildNotesBrowseView(st)
				w.SetContent(view)
				sinkSize = view.MinSize()

				b.StopTimer()
				w.Close()
				b.StartTimer()
			}
		})
	}
}

// BenchmarkNotesBrowseScroll is what a scroll costs once the list is open: the
// list re-runs UpdateItem for the rows it moves over.
func BenchmarkNotesBrowseScroll(b *testing.B) {
	st, done := benchNotesApp(b, 500)
	defer done()
	w := test.NewWindow(nil)
	defer w.Close()
	w.Resize(fyne.NewSize(620, 900))
	view := buildNotesBrowseView(st)
	w.SetContent(view)
	list := benchFindList(view)
	if list == nil {
		b.Fatal("no windowed list in the built view")
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		list.ScrollToOffset(float32((i % 40) * 120))
	}
}

var (
	sinkObject fyne.CanvasObject
	sinkNotes  []StoredNote
	sinkInt    int
	sinkSize   fyne.Size
)
