package bibletext

// Renderer tests for the styled reading pane (milestone 2). The last test
// renders the widget through fyne's software canvas — real pixels, no
// Windows/Linux machine required — and asserts the one thing this whole
// project exists for: the words of Christ come out RED.

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func john3RedState() *AppState {
	bd := &BibleData{
		Books: []string{"John"},
		Verses: map[string]map[int][]Verse{"John": {3: {
			{BookName: "John", Book: "John", Chapter: 3, Verse: 1,
				Text: "Now there was a man of the Pharisees named Nicodemus."},
			{BookName: "John", Book: "John", Chapter: 3, Verse: 16,
				Text: "For God so loved the world, that he gave his only Son."},
		}}},
	}
	return &AppState{Bible: bd, CurrentBook: "John", CurrentChapter: 3}
}

func TestStyledPaneMergeRuns(t *testing.T) {
	ln := styledLine{Runs: []styledRun{
		{Text: superscriptNumber(1), Kind: runVerseNum, Verse: 1},
		{Text: "In", Kind: runWord, Verse: 1},
		{Text: "the", Kind: runWord, Verse: 1},
		{Text: "beginning", Kind: runWord, Verse: 1, RedLetter: true},
		{Text: "was", Kind: runWord, Verse: 1, RedLetter: true},
		{Text: "light", Kind: runWord, Verse: 1},
	}}
	got := mergeDrawRuns(0, ln)
	want := []struct {
		text string
		kind runKind
		red  bool
	}{
		{superscriptNumber(1), runVerseNum, false},
		{"In the", runWord, false},
		{"beginning was", runWord, true},
		{"light", runWord, false},
	}
	if len(got) != len(want) {
		t.Fatalf("merged into %d segments, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i].Text != w.text || got[i].Kind != w.kind || got[i].Red != w.red {
			t.Errorf("segment %d = %+v, want %+v", i, got[i], w)
		}
	}
}

func TestStyledPaneObjects(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := psalm23State()
	p := newStyledReadingPane(st, st.Bible.GetChapter("Psalms", 23))
	r := p.CreateRenderer().(*styledPaneRenderer)

	// Wide default layout: 4 authored lines, each with a small number of
	// merged segments — object count stays modest (band + texts).
	if len(r.texts) == 0 || len(r.texts) > 12 {
		t.Fatalf("unexpected draw-segment count %d for 4 poem lines", len(r.texts))
	}
	if len(r.objects) != len(r.texts)+1 {
		t.Fatalf("objects = %d, want band + %d texts", len(r.objects), len(r.texts))
	}
	// First segment of line 0 is the verse-number label at the small size.
	if r.texts[0].TextSize >= p.textSize {
		t.Errorf("verse-number segment must render smaller than body (%v vs %v)",
			r.texts[0].TextSize, p.textSize)
	}
}

func TestStyledPaneHighlightBandGeometry(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := psalm23State()
	st.HasHighlightedVerse = true
	st.HighlightedBook = "Psalms"
	st.HighlightedChapter = 23
	st.HighlightedVerse = 2
	p := newStyledReadingPane(st, st.Bible.GetChapter("Psalms", 23))
	r := p.CreateRenderer().(*styledPaneRenderer)

	if !r.band.Visible() {
		t.Fatal("highlight band must be visible for a highlighted verse")
	}
	wantTop := p.lay.Lines[2].Y
	if got := r.band.Position().Y; got != wantTop {
		t.Errorf("band top = %v, want %v (line 2)", got, wantTop)
	}
	wantH := p.lay.Lines[3].Y + p.lay.Lines[3].H - wantTop
	if got := r.band.Size().Height; got != wantH {
		t.Errorf("band height = %v, want %v (two poem lines)", got, wantH)
	}

	// No highlight → hidden.
	st2 := psalm23State()
	p2 := newStyledReadingPane(st2, st2.Bible.GetChapter("Psalms", 23))
	r2 := p2.CreateRenderer().(*styledPaneRenderer)
	if r2.band.Visible() {
		t.Error("band must be hidden with no highlighted verse")
	}
}

func TestStyledPaneResizeRelayouts(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := acts4ShareState()
	p := newStyledReadingPane(st, st.Bible.GetChapter("Acts", 4))
	wideLines := len(p.lay.Lines)
	wideH := p.MinSize().Height

	p.Resize(fyne.NewSize(260, 400))
	if len(p.lay.Lines) <= wideLines {
		t.Errorf("narrow resize must add wrapped lines: %d -> %d", wideLines, len(p.lay.Lines))
	}
	if p.MinSize().Height <= wideH {
		t.Errorf("narrow MinSize height must grow: %v -> %v", wideH, p.MinSize().Height)
	}
}

// TestStyledPaneVisualRedLetter renders the pane in software and checks the
// pixels: with red-letter on, John 3:16 must contribute clearly red pixels
// that a narration-only render lacks. Set BIBLETEXT_PANE_SNAPSHOT_DIR to also
// write PNG snapshots for human inspection.
func TestStyledPaneVisualRedLetter(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setRedLetterEnabled(true)

	st := john3RedState()
	p := newStyledReadingPane(st, st.Bible.GetChapter("John", 3))
	w := test.NewWindow(p)
	defer w.Close()
	w.Resize(fyne.NewSize(420, 320))

	img := w.Canvas().Capture()
	red := countReddish(img)
	if red < 50 {
		t.Errorf("expected clearly red pixels for John 3:16, found %d", red)
	}

	// Control: red-letter OFF → no red pixels.
	setRedLetterEnabled(false)
	p2 := newStyledReadingPane(john3RedState(), st.Bible.GetChapter("John", 3))
	w2 := test.NewWindow(p2)
	defer w2.Close()
	w2.Resize(fyne.NewSize(420, 320))
	img2 := w2.Canvas().Capture()
	if n := countReddish(img2); n >= 50 {
		t.Errorf("red-letter off must not render red text, found %d red pixels", n)
	}

	if dir := os.Getenv("BIBLETEXT_PANE_SNAPSHOT_DIR"); dir != "" {
		writePNG(t, filepath.Join(dir, "styled-pane-redletter-on.png"), img)
		writePNG(t, filepath.Join(dir, "styled-pane-redletter-off.png"), img2)
	}
}

// TestStyledPaneVisualPoetry renders Psalm 23 and snapshots it (env-gated);
// the assertion is structural — text pixels exist well below the first line,
// i.e. the poem lines really occupy separate rows.
func TestStyledPaneVisualPoetry(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := psalm23State()
	p := newStyledReadingPane(st, st.Bible.GetChapter("Psalms", 23))
	w := test.NewWindow(p)
	defer w.Close()
	w.Resize(fyne.NewSize(420, 360))

	img := w.Canvas().Capture()
	rows := textRows(img)
	if rows < 4 {
		t.Errorf("Psalm 23:1-2 must render as at least 4 text rows, found %d", rows)
	}
	if dir := os.Getenv("BIBLETEXT_PANE_SNAPSHOT_DIR"); dir != "" {
		writePNG(t, filepath.Join(dir, "styled-pane-psalm23.png"), img)
	}
}

// countReddish counts pixels that are unambiguously red-dominant.
func countReddish(img image.Image) int {
	b := img.Bounds()
	n := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			if a > 0 && r>>8 > 120 && r > g+(40<<8) && r > bl+(40<<8) {
				n++
			}
		}
	}
	return n
}

// textRows counts contiguous horizontal bands that contain non-background
// pixels — a proxy for rendered text lines.
func textRows(img image.Image) int {
	b := img.Bounds()
	bgR, bgG, bgB, _ := img.At(b.Min.X, b.Min.Y).RGBA()
	inRow, rows := false, 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		hasInk := false
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			if absDiff(r, bgR)+absDiff(g, bgG)+absDiff(bl, bgB) > 30<<8 {
				hasInk = true
				break
			}
		}
		if hasInk && !inRow {
			rows++
			inRow = true
		} else if !hasInk {
			inRow = false
		}
	}
	return rows
}

func absDiff(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Logf("snapshot: %v", err)
		return
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Logf("snapshot: %v", err)
	}
}
