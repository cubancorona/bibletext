package bibletext

// The browser's DENSITY: the browser took up far too much space, on the phone
// AND on mimic/desktop alike — one shared noteBrowseRow serves both, so one
// budget holds both. The budget is derived
// from the sizes the row is actually built from (the browse* constants and
// browseRowTheme, notes_browse.go) — never from magic pixels: shrink a size
// there and the budget follows; grow the row's STRUCTURE (a new full-height
// line, a reinstated padding layer) and this test is what says so.

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// browseRowBudget is the tallest a row showing bodyLines wrapped bubble lines
// may be, assembled from the row's own named sizes:
//
//	heading (ref/meta line) + gap + bubble (body lines + label inner padding +
//	surface padding + border + tail) + row padding + separator row.
func browseRowBudget(t *testing.T, bodyLines int) float32 {
	t.Helper()
	lineH := func(size float32, style fyne.TextStyle) float32 {
		return fyne.MeasureText("Ag", size, style).Height
	}
	heading := lineH(browseRefTextSize, fyne.TextStyle{Bold: true})
	if meta := lineH(browseMetaTextSize, fyne.TextStyle{}); meta > heading {
		heading = meta
	}
	rowTheme := browseRowTheme{Theme: theme.DefaultTheme()}
	body := float32(bodyLines)*lineH(browseBodyTextSize, fyne.TextStyle{}) +
		float32(bodyLines-1)*rowTheme.Size(theme.SizeNameLineSpacing) +
		2*rowTheme.Size(theme.SizeNameInnerPadding) // the Label's own box
	bubble := body + 2*browseBubblePad + 2 /*card stroke*/ + noteTailDepth
	sep := widget.NewSeparator().MinSize().Height + browseSepGap
	const slack = 6 // sub-pixel rounding across the stack, not a size of its own
	return heading + browseRowGap + bubble + 2*browseRowPad + sep + slack
}

// measuredBrowseRow lays one row out at a list-like width and returns its
// settled height (two passes, as the list's UpdateItem does, so wrapping
// labels report their true height).
func measuredBrowseRow(t *testing.T, st *AppState, n StoredNote) float32 {
	t.Helper()
	row := noteBrowseRow(st, n, st.pal())
	w := test.NewWindow(row)
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(600, 800))
	row.Resize(fyne.NewSize(600, row.MinSize().Height))
	row.Resize(fyne.NewSize(600, row.MinSize().Height))
	return row.MinSize().Height
}

func TestBrowserRowFitsTheDensityBudget(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)

	st := psalm23State()

	oneLine := StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "Psalms",
		Chapter: 23, VerseLo: 1, Text: "fixture short alpha", Received: 1_700_000_000}
	if h, budget := measuredBrowseRow(t, st, oneLine), browseRowBudget(t, 1); h > budget {
		t.Errorf("a one-line note's row measures %vpt against its %vpt budget — "+
			"the row has grown structure its sizes do not account for", h, budget)
	}

	// A LONG message is held to the preview cap: however much the sender
	// wrote, the row's bubble never exceeds the wrapped preview's worth of
	// lines. The cap is asserted on the string (deterministic) and the row is
	// asserted against a budget of generous wrapped lines for it.
	long := oneLine
	long.Text = strings.Repeat("He makes me lie down in green pastures. ", 30)
	preview := notePreview(long.Text)
	if r := []rune(preview); len(r) > browsePreviewMaxRunes+2 { // + the ellipsis
		t.Fatalf("the preview did not cap: %d runes", len(r))
	}
	// 220 runes at 13pt wrap to at most ~4 lines at the 600pt list width;
	// budget them at 5 so the assertion holds on any host font metrics.
	if h, budget := measuredBrowseRow(t, st, long), browseRowBudget(t, 5); h > budget {
		t.Errorf("a capped long note's row measures %vpt against its %vpt budget", h, budget)
	}
}

// The density target, measured honestly: at least TWICE the rows per screen.
// The pre-density row measured ~130pt at the default text size — roughly five
// rows on a 700pt phone viewport. The budgeted row must fit at least ten.
func TestBrowserShowsTwiceTheRowsPerScreen(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)

	st := psalm23State()
	typical := StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "Psalms",
		Chapter: 23, VerseLo: 1, Text: "fixture density message alpha", Received: 1_700_000_000}

	const phoneViewport = 700 // the fixture screen where the old layout fit about five rows
	h := measuredBrowseRow(t, st, typical)
	if rows := int(phoneViewport / h); rows < 10 {
		t.Errorf("a typical row measures %vpt — %d rows per %dpt screen, want at least 10 "+
			"(twice the ~5 the pre-density row allowed)", h, rows, phoneViewport)
	}
}

// notePreview is the deterministic cap: runes and authored lines, with the cut
// marked. The full text is one tap away on the passage.
func TestNotePreviewCaps(t *testing.T) {
	if got := notePreview("short"); got != "short" {
		t.Errorf("a short note must pass through untouched: %q", got)
	}
	longRunes := strings.Repeat("x", browsePreviewMaxRunes+50)
	if got := notePreview(longRunes); !strings.HasSuffix(got, "…") ||
		len([]rune(got)) > browsePreviewMaxRunes+2 {
		t.Errorf("rune cap failed: %d runes, ellipsis=%v", len([]rune(got)), strings.HasSuffix(got, "…"))
	}
	manyLines := strings.Repeat("line\n", browsePreviewMaxLines+3)
	got := notePreview(manyLines)
	if n := strings.Count(got, "\n"); n > browsePreviewMaxLines-1 {
		t.Errorf("line cap failed: %d breaks survive", n)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("a cut must be marked: %q", got)
	}
}
