package bibletext

// S11a — the browser must hold a years-deep scrapbook ([redacted-retired-private-reference],
// hard case 14: "the browse list's widget-per-note VBox is the only real
// ceiling and it arrives thousands of notes before storage does").
//
// The pin here is STRUCTURAL, not a stopwatch: a windowed list builds row
// widgets for the rows a reader can see, so the number of note cards alive
// after layout must stay flat as the store grows. A timing assertion would be
// the flaky twin of the same fact. The measured build times ride along as logs
// (run with -v) so the numbers in the S11 report stay reproducible.

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// seedNoteStore writes count displayable notes straight into the store value —
// one JSONL write, the exact bytes readNoteStore parses — because addNote is a
// read-modify-write per record and would serialise a growing store count times
// just to set a fixture.
func seedNoteStore(t *testing.T, count int) {
	t.Helper()
	p := appPrefs()
	if p == nil {
		t.Fatal("no preferences in test app")
	}
	texts := []string{
		"a word for you",
		"He makes me lie down in green pastures; He leads me beside quiet waters — this one carried me through the whole of last winter.",
		"synthetic note this morning.",
		"Remember synthetic note? This is the passage. Read the whole chapter when you get a quiet minute and tell me what you see in verse four.",
	}
	versions := []string{"web", "bsb", "webc"}
	var b strings.Builder
	for i := 1; i <= count; i++ {
		n := StoredNote{
			ID:        uint64(i),
			Kind:      noteKindReceived,
			VersionID: versions[i%len(versions)],
			Book:      "Psalms",
			Chapter:   1 + i%150,
			VerseLo:   1 + i%6,
			Text:      texts[i%len(texts)],
			Received:  1_700_000_000 + int64(i),
		}
		if i%5 == 0 {
			n.Kind = noteKindMine
		}
		line, err := json.Marshal(n)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	p.SetString(prefNotesStore, b.String())
	p.SetString(prefNotesNextID, "9000000")
}

// countNoteCards walks what survives layout and counts the note rows that were
// actually BUILT — the number a windowed list keeps flat and a VBox grows one
// per stored note.
func countNoteCards(root fyne.CanvasObject) int {
	count := 0
	var walk func(o fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		if o == nil {
			return
		}
		if _, ok := o.(*searchResultCard); ok {
			count++
			// keep walking: cards do not nest, but the walk is cheap
		}
		if sc, ok := o.(*container.Scroll); ok {
			walk(sc.Content)
			return
		}
		if c, ok := o.(*fyne.Container); ok {
			for _, ch := range c.Objects {
				walk(ch)
			}
			return
		}
		if w, ok := o.(fyne.Widget); ok {
			if r := test.WidgetRenderer(w); r != nil {
				for _, ch := range r.Objects() {
					walk(ch)
				}
			}
		}
	}
	walk(root)
	return count
}

func TestNotesBrowserIsWindowedAtScrapbookScale(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	const stored = 2000
	seedNoteStore(t, stored)

	st := psalm23State()
	st.NotesMode = true

	// Warm the store cache first, so the numbers below measure the VIEW and not
	// the one-off JSONL parse the store already amortises across navigations.
	if got := storedNoteCount(appPrefs()); got != stored {
		t.Fatalf("seeded %d notes, store sees %d", stored, got)
	}

	buildStart := time.Now()
	view := buildNotesBrowseView(st)
	buildTook := time.Since(buildStart)

	layoutStart := time.Now()
	w := test.NewWindow(view)
	defer w.Close()
	w.Resize(fyne.NewSize(900, 700))
	layoutTook := time.Since(layoutStart)

	t.Logf("%d notes: buildNotesBrowseView %v, window layout %v", stored, buildTook, layoutTook)

	cards := countNoteCards(view)
	t.Logf("%d notes: %d note cards built after layout", stored, cards)
	if cards == 0 {
		t.Fatal("no note rows on screen at all — the browser is blank")
	}
	// A 700pt viewport holds well under 40 rows at the row's minimum height;
	// the pool may hold a couple of off-screen spares. 2,000 built cards is the
	// VBox-per-note shape this test exists to keep out.
	if cards > 60 {
		t.Errorf("%d note cards built for a %d-note store — the browser is building "+
			"the whole scrapbook instead of the window (want <= 60)", cards, stored)
	}

	// The list is still the list: the header line names the whole collection.
	text := seenText(t, view, fyne.NewSize(900, 700))
	if !strings.Contains(text, "2000 notes") {
		t.Errorf("the header no longer names the collection; seen:\n%.400s", text)
	}
	_ = widget.ListItemID(0) // the windowed widget this file pins
}

// S11b — the capacity NOTICE. At the threshold the browser header says, once
// and quietly, that everything is kept and that a very large scrapbook has a
// cost. It is the app's own chrome voice: a sentence, no link, no button, no
// action asked — because there is no cap, no eviction, and nothing the reader
// must do. Below the threshold it says nothing at all: a reader with a
// three-note scrapbook must never be warned about a problem they do not have.
func TestCapacityNoticeLine(t *testing.T) {
	for _, tc := range []struct {
		stored int
		want   bool
	}{
		{0, false},
		{1, false},
		{notesCapacityNoticeAt - 1, false},
		{notesCapacityNoticeAt, true},
		{notesCapacityNoticeAt + 500, true},
	} {
		got := notesCapacityLine(tc.stored)
		if (got != "") != tc.want {
			t.Errorf("notesCapacityLine(%d) = %q, want shown=%v", tc.stored, got, tc.want)
		}
	}
	line := notesCapacityLine(notesCapacityNoticeAt)
	// The two facts the sentence exists to say, in the app's own voice.
	for _, want := range []string{"kept", "ever removed"} {
		if !strings.Contains(line, want) {
			t.Errorf("the notice does not say %q — the promise comes before the cost: %q", want, line)
		}
	}
	if !strings.Contains(line, "slow") {
		t.Errorf("the notice does not name the cost (slowness on older devices): %q", line)
	}
}

// The notice must be SEEN, in the browser, at the threshold — and not before.
// Seen the way screen_seen_test.go means it: surviving layout in a real
// window, not merely present in the tree.
func TestCapacityNoticeIsSeenAtTheThreshold(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	st := psalm23State()
	st.NotesMode = true

	seedNoteStore(t, notesCapacityNoticeAt-1)
	if got := seenText(t, buildNotesBrowseView(st), fyne.NewSize(900, 700)); strings.Contains(got, "ever removed") {
		t.Errorf("the capacity notice is showing below the threshold — a warning about "+
			"nothing erodes the sentences that mean something.\nseen:\n%.400s", got)
	}

	seedNoteStore(t, notesCapacityNoticeAt)
	got := seenText(t, buildNotesBrowseView(st), fyne.NewSize(900, 700))
	if !strings.Contains(got, strings.ToLower("Every note is kept")) || !strings.Contains(got, "slow on older devices") {
		t.Errorf("the reader cannot see the capacity notice at %d stored notes.\nseen:\n%.600s",
			notesCapacityNoticeAt, got)
	}
	// One sentence in the app's chrome voice: nothing tappable rides with it.
	// The header's own controls (sort, who, Done) are buttons the notice must
	// not add to — count them with and without the notice and they must match.
	deleteAllNotes(appPrefs())
	seedNoteStore(t, 10)
	before := countButtons(t, buildNotesBrowseView(st))
	seedNoteStore(t, notesCapacityNoticeAt)
	after := countButtons(t, buildNotesBrowseView(st))
	if after != before {
		t.Errorf("the capacity notice changed the button count %d -> %d — it must carry "+
			"no action: there is nothing the reader must do", before, after)
	}
}

// countButtons counts the buttons in a view's visible tree.
func countButtons(t *testing.T, root fyne.CanvasObject) int {
	t.Helper()
	n := 0
	var walk func(o fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		if o == nil || !o.Visible() {
			return
		}
		if _, ok := o.(*widget.Button); ok {
			n++
			return
		}
		if sc, ok := o.(*container.Scroll); ok {
			walk(sc.Content)
			return
		}
		if c, ok := o.(*fyne.Container); ok {
			for _, ch := range c.Objects {
				walk(ch)
			}
			return
		}
		if w, ok := o.(fyne.Widget); ok {
			if r := test.WidgetRenderer(w); r != nil {
				for _, ch := range r.Objects() {
					walk(ch)
				}
			}
		}
	}
	walk(root)
	return n
}
