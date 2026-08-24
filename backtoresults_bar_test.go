package bibletext

// The results trail is CHROME, not content: the back-to-results button took
// up too much space, where it has to read as non-intrusive, clear and
// elegant. These pin the compact composition at SCREEN level, the
// screen_seen_test.go way: what a reader can see, laid out in a real window —
// and the budget is derived from the bar's own theme sizes (trailChipTheme),
// never from magic pixels.
//
// They also pin the clarified scope of defect B: the trail belongs to
// GENUINE search-result arrivals only. A notes-browser row tap is a
// navigation — it raises no trail at all (openNote, notes_browse.go).

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// trailBarBudget is the tallest the trail may be: one compact button of the
// bar's own theme, plus two points of layout slack. Anything over it means a
// second row, a surface card, or a control that outgrew the chrome.
func trailBarBudget(t *testing.T) float32 {
	t.Helper()
	ref := widget.NewButton("‹ Results", func() {})
	ref.Importance = widget.LowImportance
	box := container.NewThemeOverride(ref, trailChipTheme{Theme: theme.DefaultTheme()})
	w := test.NewWindow(box)
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(400, 100))
	budget := box.MinSize().Height + 2

	// THE ABSOLUTE BACKSTOP: the structural budget above follows
	// trailChipTheme, so inflating the chrome's own sizes sailed through it —
	// the density suite's twice-the-rows lesson, applied here. The trail must
	// stay visibly SMALLER than ordinary chrome: under a default-theme button.
	std := widget.NewButton("‹ Results", func() {})
	sw := test.NewWindow(std)
	t.Cleanup(sw.Close)
	sw.Resize(fyne.NewSize(400, 100))
	if cap := std.MinSize().Height; budget >= cap {
		t.Fatalf("the compact trail's own budget (%.1f) has grown to ordinary-chrome height (%.1f) — the chrome sizes inflated", budget, cap)
	}
	return budget
}

func TestResultsTrailIsOneCompactRow(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	for _, tc := range []struct {
		name   string
		mobile bool
	}{
		{"desktop", false},
		{"mobile", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := psalm23State()
			st.ActiveSearchQuery = "shepherd"
			st.CanReturnToSearchResults = true
			if tc.mobile {
				st.surfaceSearch = func() {}
			}

			bar := backToResultsBar(st)
			w := test.NewWindow(bar)
			defer w.Close()
			w.Resize(fyne.NewSize(700, 100))

			if h, budget := bar.MinSize().Height, trailBarBudget(t); h > budget {
				t.Errorf("the trail is %vpt tall against a one-compact-row budget of %vpt — "+
					"it is reading as content again", h, budget)
			}

			// Composition: exactly the two verbs, and no surface card — the
			// bordered rectangle is what made the old bar read as a second
			// header.
			buttons, carded := 0, false
			var walk func(o fyne.CanvasObject)
			walk = func(o fyne.CanvasObject) {
				if o == nil {
					return
				}
				if _, ok := o.(*widget.Button); ok {
					buttons++
				}
				if r, ok := o.(*canvas.Rectangle); ok && r.StrokeWidth > 0 {
					carded = true
				}
				if c, ok := o.(*fyne.Container); ok {
					for _, ch := range c.Objects {
						walk(ch)
					}
					return
				}
				if wd, ok := o.(fyne.Widget); ok {
					if r := test.WidgetRenderer(wd); r != nil {
						for _, ch := range r.Objects() {
							walk(ch)
						}
					}
				}
			}
			walk(bar)
			if buttons != 2 {
				t.Errorf("the trail carries %d buttons, want exactly 2 (‹ Results and ✕)", buttons)
			}
			if carded {
				t.Error("the trail is drawn on a bordered surface card — it must read as chrome, not content")
			}

			// The label: phones carry no query preview (the query is waiting in
			// the Search tab the trail returns to); desktop keeps a short one.
			seen := seenText(t, backToResultsBar(st), fyne.NewSize(700, 100))
			if tc.mobile {
				if strings.Contains(seen, "shepherd") {
					t.Errorf("the phone trail must not carry the query, seen: %s", seen)
				}
				if !strings.Contains(seen, "results") {
					t.Errorf("the phone trail lost its way back, seen: %s", seen)
				}
			} else if !strings.Contains(seen, "shepherd") {
				t.Errorf("the desktop trail should preview a short query, seen: %s", seen)
			}
		})
	}
}

// A LONG query must not stretch the bar: the preview is dropped, never wrapped.
func TestResultsTrailDropsALongQuery(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := psalm23State()
	st.ActiveSearchQuery = "where does it talk about the shepherd who leaves the ninety-nine"
	st.CanReturnToSearchResults = true

	bar := backToResultsBar(st)
	w := test.NewWindow(bar)
	defer w.Close()
	w.Resize(fyne.NewSize(700, 100))
	if h, budget := bar.MinSize().Height, trailBarBudget(t); h > budget {
		t.Errorf("a long query stretched the trail to %vpt (budget %vpt)", h, budget)
	}
	if seen := seenText(t, backToResultsBar(st), fyne.NewSize(700, 100)); strings.Contains(seen, "ninety-nine") {
		t.Errorf("the long query rode onto the trail, seen: %s", seen)
	}
}

// Defect B, clarified: a search arrival shows the compact
// trail; a notes-browser arrival shows NO trail at all. Asserted at screen
// level on the shared reading view, which is where both arrivals land.
func TestTrailAppearsForSearchArrivalsOnly(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	stored, ok := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
		Book: "Psalms", Chapter: 23, VerseLo: 1, Text: "a note"})
	if !ok {
		t.Fatal("the note was not stored")
	}

	// The search arrival raises the trail.
	st := psalm23State()
	openSearchResultRange(st, Verse{BookName: "Psalms", Chapter: 23, Verse: 1}, 0)
	if !st.CanReturnToSearchResults {
		t.Fatal("a search arrival must raise the trail")
	}
	if seen := seenText(t, buildReadingView(st), fyne.NewSize(900, 700)); !strings.Contains(seen, "results") {
		t.Errorf("the reader cannot see the trail after a search arrival.\nseen:\n%s", seen)
	}

	// The browser arrival raises none.
	st = psalm23State()
	openNote(st, stored)
	if st.CanReturnToSearchResults {
		t.Fatal("a browser arrival must raise no trail")
	}
	if seen := seenText(t, buildReadingView(st), fyne.NewSize(900, 700)); strings.Contains(seen, "results") {
		t.Errorf("a browser arrival put the trail on screen.\nseen:\n%s", seen)
	}
}
