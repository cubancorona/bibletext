package bibletext

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// What the reader can actually SEE, as opposed to what got built.
//
// The distinction is the whole point. A bug shipped in 1.1.6 and 1.1.7 where
// tapping Read left the reader on a blank screen — and every widget involved was
// present in the tree the whole time, correctly constructed, simply not shown.
// A test that walks the object tree passes that bug. So these walk what SURVIVES
// layout: an object counts only if it and every ancestor is Visible() and it has
// a non-zero size once the window has laid it out.
//
// The acceptance gate for this file is scripts/view-test-gate.sh, which applies
// six mutations that each break something a reader would see and requires the
// suite to go red for every one. These tests exist to close M2, M3 and M6, which
// the suite could not feel before them.

// seenLeaves lays root out in a real window and returns the drawable leaves the
// reader can see.
//
// LAYOUT IS NOT OPTIONAL. Before Resize every object is zero-sized, so a walk
// without it reports the same answer for a full screen and an empty one — which
// is exactly the kind of test that passes while the app shows nothing.
func seenLeaves(t *testing.T, root fyne.CanvasObject, size fyne.Size) []fyne.CanvasObject {
	t.Helper()
	w := test.NewWindow(root)
	t.Cleanup(w.Close)
	w.Resize(size)

	var out []fyne.CanvasObject
	var walk func(o fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		if o == nil || !o.Visible() {
			return // an invisible parent takes its whole subtree off the screen
		}
		// Descend GENERICALLY. Type-switching on the container types you happen
		// to remember is how a walk quietly stops seeing things: this missed
		// container.ThemeOverride, which wraps the notes browser's only exit
		// control, and reported the button as unseen when a reader can see it
		// perfectly well. A widget's renderer knows its children; ask it.
		// A Scroll BEFORE the generic case: its renderer exposes a clipped
		// wrapper rather than the content, so the generic descent stops dead at
		// it and reports an empty reading pane.
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
				return
			}
		}
		// EFFECTIVE size, not Size(). A canvas.Text is drawn at its intrinsic
		// size whether or not anything resized it, so several of this app's
		// renderers only Move their text and leave Size at zero — and a rule of
		// "Size must be non-zero" then reports a full page of verses as an
		// empty screen. Falling back to MinSize is what makes the answer match
		// what is actually painted.
		sz := o.Size()
		if sz.Width <= 0 || sz.Height <= 0 {
			sz = o.MinSize()
		}
		if sz.Width > 0 && sz.Height > 0 {
			out = append(out, o)
		}
	}
	walk(root)
	return out
}

// seenText is the text the reader can read, lowercased and joined.
func seenText(t *testing.T, root fyne.CanvasObject, size fyne.Size) string {
	t.Helper()
	var b strings.Builder
	for _, o := range seenLeaves(t, root, size) {
		switch v := o.(type) {
		case *canvas.Text:
			b.WriteString(v.Text)
		case *widget.Label:
			b.WriteString(v.Text)
		case *widget.Button:
			b.WriteString(v.Text)
		case *widget.Entry:
			b.WriteString(v.Text)
		case *widget.RichText:
			b.WriteString(v.String())
		}
		b.WriteByte(' ')
	}
	// Collapse whitespace. The styled pane draws a verse as many separately
	// positioned segments -- a poetic line break splits "I shall lack nothing"
	// across two of them -- so a phrase only survives if the segments are joined
	// the way the eye joins them.
	return strings.Join(strings.Fields(strings.ToLower(b.String())), " ")
}

// M2 — the reader must be able to SEE the verses.
//
// The mutation this closes hides the reading pane and leaves it in the tree. Every
// object is still constructed, still correct, still reachable by a tree walk, and
// the reader is looking at nothing.
func TestReadingViewShowsItsVerses(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	// The FYNE reading surface — what Windows and Linux ship. On darwin the
	// verses live in a native NSTextView above the canvas, so there is no Fyne
	// text to look for and this question cannot be asked of the tree at all;
	// there, "can the reader see the verses" is answered by whether the overlay
	// is shown, which is a different assertion and belongs in its own test.
	orig := useStyledPane
	useStyledPane = func() bool { return true }
	defer func() { useStyledPane = orig }()

	st := psalm23State()
	got := seenText(t, buildReadingView(st), fyne.NewSize(900, 700))

	// Words from the passage itself, not chrome: chrome survives hiding the pane.
	for _, want := range []string{"shepherd", "green pastures"} {
		if !strings.Contains(got, want) {
			t.Errorf("the reader cannot see %q anywhere on the reading screen.\n"+
				"Everything may still be in the tree — that is the failure this test "+
				"exists for.\nseen:\n%s", want, got)
		}
	}
}

// M6 — no key view may render blank.
//
// The mutation this closes returns an empty container from a view builder: a
// valid object, correctly wired, showing nothing. "It built without error" is not
// evidence that anybody can read it.
func TestKeyViewsAreNotBlank(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())
	addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "Psalms", Chapter: 23,
		VerseLo: 1, Text: "a note to look at"})

	for _, tc := range []struct {
		name  string
		build func(*AppState) fyne.CanvasObject
		least int
	}{
		{"reading", buildReadingView, 8},
		{"notes browser", buildNotesBrowseView, 4},
		{"search results", buildSearchResultsView, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := psalm23State()
			st.NotesMode = true
			st.SearchResults = st.Bible.GetChapter("Psalms", 23)
			st.ActiveSearchQuery = "shepherd"

			seen := seenLeaves(t, tc.build(st), fyne.NewSize(900, 700))
			if len(seen) < tc.least {
				t.Errorf("%s shows %d visible things, want at least %d — the reader is "+
					"looking at a blank screen", tc.name, len(seen), tc.least)
			}
		})
	}
}

// M3 — every view that takes over the screen must show a way out of it.
//

// "in mac there's no way to go from the notes view back to the reading pane".
// The control existed in the code by then; what mattered was whether a reader
// could SEE it.
func TestNotesBrowserShowsAWayOut(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())
	addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "Psalms", Chapter: 23,
		VerseLo: 1, Text: "a note to look at"})

	st := psalm23State()
	st.NotesMode = true
	// surfaceSearch nil is the desktop case — the phones leave through the tab
	// bar, which is always on screen; desktop must carry its own exit.
	st.surfaceSearch = nil

	got := seenText(t, buildNotesBrowseView(st), fyne.NewSize(900, 700))
	if !strings.Contains(got, "done") {
		t.Errorf("the notes browser shows no way back to reading.\n"+
			"A view that takes over the screen owns its own exit.\nseen:\n%s", got)
	}
}
