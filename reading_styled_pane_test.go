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
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
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
	// merged segments — object count stays modest (wash rects + texts).
	if len(r.texts) == 0 || len(r.texts) > 12 {
		t.Fatalf("unexpected draw-segment count %d for 4 poem lines", len(r.texts))
	}
	// Nothing is highlighted and nothing is being narrated, so there are no
	// wash rects of either kind — only the glyphs. Both washes are now sized
	// from their span lists, so "none" costs no objects at all, where the old
	// single read-along band was always present and merely hidden.
	if len(r.tintRects) != 0 {
		t.Errorf("wash rects = %d with nothing highlighted, want 0", len(r.tintRects))
	}
	if len(r.raRects) != 0 {
		t.Errorf("narration rects = %d with nothing narrated, want 0", len(r.raRects))
	}
	if len(r.objects) != len(r.texts)+len(r.tintRects)+len(r.raRects) {
		t.Fatalf("objects = %d, want %d wash + %d narration + %d texts",
			len(r.objects), len(r.tintRects), len(r.raRects), len(r.texts))
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
	st.setHL(hlSearch, "Psalms", 23, 2, 0)
	p := newStyledReadingPane(st, st.Bible.GetChapter("Psalms", 23))
	r := p.CreateRenderer().(*styledPaneRenderer)

	// Verse 2 is two authored poem lines → one wash rect per line, each sitting
	// exactly on its own line's vertical band (NOT one rect spanning both).
	if len(r.tintRects) != 2 {
		t.Fatalf("wash rects = %d, want one per highlighted line (2)", len(r.tintRects))
	}
	for i, li := range []int{2, 3} {
		rect := r.tintRects[i]
		if !rect.Visible() {
			t.Errorf("wash rect %d must be visible for a highlighted verse", i)
		}
		ln := p.lay.Lines[li]
		if got := rect.Position().Y; got != ln.Y {
			t.Errorf("wash rect %d top = %v, want %v (line %d)", i, got, ln.Y, li)
		}
		if got := rect.Size().Height; got != ln.H {
			t.Errorf("wash rect %d height = %v, want one line (%v)", i, got, ln.H)
		}
		// X-bounded to the verse's runs, not the whole column.
		if rect.Position().X < p.insetX() {
			t.Errorf("wash rect %d starts left of the text inset (%v < %v)", i, rect.Position().X, p.insetX())
		}
		if w := rect.Size().Width; w <= 0 || w > p.lastWidth-p.insetX() {
			t.Errorf("wash rect %d width = %v, out of range for a run-bounded rect", i, w)
		}
	}

	// No highlight → no wash rects at all.
	st2 := psalm23State()
	p2 := newStyledReadingPane(st2, st2.Bible.GetChapter("Psalms", 23))
	r2 := p2.CreateRenderer().(*styledPaneRenderer)
	if len(r2.tintRects) != 0 {
		t.Errorf("wash rects = %d with no highlighted verse, want 0", len(r2.tintRects))
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

// TestStyledPaneRepresentativeSnapshots renders the pane the way SHIPPING
// Windows/Linux builds would show it: the real bibleTheme (Atkinson
// Hyperlegible face), the real palettes, on the real paper surface — both
// variants. Env-gated snapshot output for human inspection; the assertions
// keep it honest in CI (paper-coloured background, dark-on-light ink in the
// light variant and light-on-dark in the dark one).
func TestStyledPaneRepresentativeSnapshots(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setRedLetterEnabled(true)

	// The REAL app theme, exactly as CreateMainUI constructs it.
	realTheme := &bibleTheme{fonts: loadBookFonts(), uiFonts: loadUIFonts()}
	app.Settings().SetTheme(realTheme)

	for _, tc := range []struct {
		name string
		pal  palette
	}{
		{"light", lightPalette},
		{"dark", darkPalette},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := john3RedState()
			p := newStyledReadingPane(st, st.Bible.GetChapter("John", 3))
			p.pal = tc.pal // deterministic variant (bypasses OS variant plumbing)

			paper := canvas.NewRectangle(tc.pal.Surface)
			w := test.NewWindow(container.NewStack(paper, p))
			defer w.Close()
			w.Resize(fyne.NewSize(460, 300))
			p.Refresh() // renderer picks up the palette

			img := w.Canvas().Capture()

			// The paper colour must appear inside the pane region (the window
			// paints its own theme background around the stack, so corner
			// pixels are padding, not paper).
			pr, pg, pb := uint32(tc.pal.Surface.R)<<8, uint32(tc.pal.Surface.G)<<8, uint32(tc.pal.Surface.B)<<8
			foundPaper := false
			for y := 10; y < 80 && !foundPaper; y += 4 {
				for x := 10; x < 120 && !foundPaper; x += 4 {
					r0, g0, b0, _ := img.At(x, y).RGBA()
					if absDiff(r0, pr)+absDiff(g0, pg)+absDiff(b0, pb) <= 24<<8 {
						foundPaper = true
					}
				}
			}
			if !foundPaper {
				t.Errorf("the %s paper colour never appears in the pane region", tc.name)
			}
			// Red letters must still read as red on both papers.
			if n := countReddish(img); n < 50 {
				t.Errorf("red letters missing on %s paper (%d red px)", tc.name, n)
			}

			if dir := os.Getenv("BIBLETEXT_PANE_SNAPSHOT_DIR"); dir != "" {
				writePNG(t, filepath.Join(dir, "styled-pane-real-"+tc.name+".png"), img)
			}
		})
	}

	// Psalm 23 on the light paper, the everyday reading look.
	st := psalm23State()
	p := newStyledReadingPane(st, st.Bible.GetChapter("Psalms", 23))
	p.pal = lightPalette
	w := test.NewWindow(container.NewStack(canvas.NewRectangle(lightPalette.Surface), p))
	defer w.Close()
	w.Resize(fyne.NewSize(460, 340))
	p.Refresh()
	if dir := os.Getenv("BIBLETEXT_PANE_SNAPSHOT_DIR"); dir != "" {
		writePNG(t, filepath.Join(dir, "styled-pane-real-psalm23.png"), w.Canvas().Capture())
	}
}
