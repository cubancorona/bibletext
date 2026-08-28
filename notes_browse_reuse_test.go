package bibletext

// A REFILLED ROW MUST BE INDISTINGUISHABLE FROM A FRESH ONE.
//
// The notes list stopped rebuilding a row's widget tree per note and now keeps
// a pooled row and refills it (browseRow, notes_browse.go). That is what made
// the browser fast, and it is also the one change that can go wrong quietly:
// a field the previous note set and the next one does not clear is a row
// showing somebody else's translation, date or minimized marker, and a control
// that captured a note when it was BUILT is a bin that deletes the wrong one.
//
// These tests hold the properties that make reuse safe. They are written
// against a row that has already shown a DIFFERENT note, because a row that
// has only ever shown one note cannot exhibit any of these defects.

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// visibleRowText is every string a row is actually showing, from the canvas
// texts and labels that are visible in it.
func visibleRowText(o fyne.CanvasObject) []string {
	var out []string
	var walk func(fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		if o == nil || !o.Visible() {
			return
		}
		switch v := o.(type) {
		case *canvas.Text:
			if v.Text != "" {
				out = append(out, v.Text)
			}
		case *widget.Label:
			if v.Text != "" {
				out = append(out, v.Text)
			}
		case *fyne.Container:
			for _, c := range v.Objects {
				walk(c)
			}
		case *container.Scroll:
			walk(v.Content)
		case fyne.Widget:
			for _, c := range test.WidgetRenderer(v).Objects() {
				walk(c)
			}
		}
	}
	walk(o)
	return out
}

func rowShows(o fyne.CanvasObject, want string) bool {
	for _, s := range visibleRowText(o) {
		if strings.Contains(s, want) {
			return true
		}
	}
	return false
}

// The two notes below differ in every field a row draws, so anything left over
// from the first is visible in the second.
func reuseNoteA() StoredNote {
	return StoredNote{ID: 41, Kind: noteKindReceived, VersionID: "web",
		Book: "John", Chapter: 3, VerseLo: 16, VerseHi: 18,
		Text: "fixture alpha message", Received: 1_700_000_000, Minimized: true}
}

func reuseNoteB() StoredNote {
	return StoredNote{ID: 42, Kind: noteKindMine, VersionID: "",
		Book: "Psalms", Chapter: 23, VerseLo: 1,
		Text: "fixture beta message", Received: 1_700_500_000}
}

func TestRefilledRowKeepsNothingFromThePreviousNote(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)

	st := psalm23State()
	r := newBrowseRow(st, st.pal())
	r.show(reuseNoteA())

	w := test.NewWindow(r.root)
	defer w.Close()
	w.Resize(fyne.NewSize(620, 400))

	if !rowShows(r.root, "John 3:16-18") {
		t.Fatalf("the row did not show the first note at all: %q", visibleRowText(r.root))
	}
	r.show(reuseNoteB())

	for _, stale := range []struct{ what, text string }{
		{"the previous reference", "John 3:16"},
		{"the previous message", "fixture alpha message"},
		{"the previous translation", "(WEB)"},
		{"the previous byline", "From Friend"},
		{"the previous minimized marker", "Minimized in the chapter"},
	} {
		if rowShows(r.root, stale.text) {
			t.Errorf("after refilling, %s is still on the row (%q). A pooled row that "+
				"keeps a field the next note does not set shows one person's note "+
				"wearing another's chrome.\nrow now reads: %q",
				stale.what, stale.text, visibleRowText(r.root))
		}
	}
	for _, want := range []string{"Psalms 23:1", "fixture beta message", "From you"} {
		if !rowShows(r.root, want) {
			t.Errorf("after refilling, the row does not show %q.\nrow reads: %q",
				want, visibleRowText(r.root))
		}
	}
}

// The measure must not depend on history. If a hidden leftover still took part
// in the layout, a reused row would be a different height from a fresh one and
// the list's own item heights would be wrong.
func TestRefilledRowMeasuresLikeAFreshOne(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)

	st := psalm23State()
	settle := func(o fyne.CanvasObject) float32 {
		w := test.NewWindow(o)
		t.Cleanup(w.Close)
		w.Resize(fyne.NewSize(620, 800))
		o.Resize(fyne.NewSize(620, o.MinSize().Height))
		o.Resize(fyne.NewSize(620, o.MinSize().Height))
		return o.MinSize().Height
	}

	for _, tc := range []struct {
		name  string
		first StoredNote
		then  StoredNote
	}{
		{"a translation, then none", reuseNoteA(), reuseNoteB()},
		{"none, then a translation", reuseNoteB(), reuseNoteA()},
		{"minimized, then not", reuseNoteA(), reuseNoteB()},
		{"a long note, then a short one", func() StoredNote {
			n := reuseNoteA()
			n.Text = strings.Repeat("fixture wrapping words for the bubble. ", 12)
			return n
		}(), reuseNoteB()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reused := newBrowseRow(st, st.pal())
			reused.show(tc.first)
			settle(reused.root)
			reused.show(tc.then)
			got := settle(reused.root)

			fresh := newBrowseRow(st, st.pal())
			fresh.show(tc.then)
			want := settle(fresh.root)

			if got != want {
				t.Errorf("a row that had shown another note measures %.2fpt for this one, "+
					"but a fresh row measures %.2fpt. The list sets its item heights from "+
					"this number, so a history-dependent measure is a list with wrong rows.",
					got, want)
			}
		})
	}
}

// The bin deletes the note the row is SHOWING, not the one it was built with.
// This is the defect the reuse could introduce and the reason the control asks
// the row for its note at the moment of the press.
func TestRefilledRowDeletesTheNoteItIsShowing(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	first, _ := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
		Book: "John", Chapter: 3, VerseLo: 16, Text: "fixture built-with"})
	shown, ok := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
		Book: "Psalms", Chapter: 23, VerseLo: 1, Text: "fixture refilled-to"})
	if !ok {
		t.Fatal("seeding failed")
	}

	st := psalm23State()
	r := newBrowseRow(st, st.pal())
	r.show(first)
	r.show(shown) // the row is recycled onto the second note

	trash := seenBannerButton(t, r.root, fyne.NewSize(620, 400), func(b *widget.Button) bool {
		return b.Text == "" && b.Icon != nil
	})
	if trash == nil {
		t.Fatal("no delete control on the refilled row")
	}
	test.Tap(trash)

	left := allNotesForBrowsing(appPrefs())
	if len(left) != 1 {
		t.Fatalf("after one delete the store holds %d notes, want 1: %+v", len(left), left)
	}
	if left[0].ID != first.ID {
		t.Errorf("the bin deleted %q — the note the row was BUILT with — instead of %q, "+
			"the note it was showing", left[0].Text, shown.Text)
	}
}

// And the tap opens the note the row is showing, for the same reason.
func TestRefilledRowOpensTheNoteItIsShowing(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	// The fixture canon holds Psalms 23 and nothing else, so the note the row
	// is REFILLED to is the only one that can navigate anywhere. A row still
	// carrying the note it was built with lands nowhere at all.
	st := psalm23State()
	st.CurrentBook, st.CurrentChapter = "John", 3

	r := newBrowseRow(st, st.pal())
	r.show(StoredNote{ID: 1, Kind: noteKindReceived, VersionID: "web",
		Book: "John", Chapter: 11, VerseLo: 35, Text: "fixture built-with"})
	r.show(StoredNote{ID: 2, Kind: noteKindReceived, VersionID: "web",
		Book: "Psalms", Chapter: 23, VerseLo: 1, Text: "fixture refilled-to"})

	r.card.Tapped(nil)

	if st.CurrentBook != "Psalms" || st.CurrentChapter != 23 {
		t.Errorf("tapping the refilled row landed on %s %d, want Psalms 23 — the row "+
			"opened the note it was built with, not the one it was showing",
			st.CurrentBook, st.CurrentChapter)
	}
}
