package bibletext

// The styled pane's IN-TEXT note sticker: the band, the bubble, the verbs, and
// the four properties the other three platforms paid to learn.
//
// This is the one platform where every part of the sticker is host-testable —
// the layout engine is pure Go, the pane draws with Fyne canvas objects, and
// the whole thing lays out in a software window. So the assertions here are the
// ones iOS, macOS and Android could only make by eye:
//
//   - the band EXISTS above the anchor verse's line, and is the ONLY space the
//     chapter gained (lesson 1 and 2, as properties over the whole chapter);
//   - no washed run — chapter tint or narration — intersects it;
//   - the bubble's text, byline, counts and verbs are SEEN on the flattened
//     screen, not merely built;
//   - clicking the counts advances focus and the verbs fire their verbs;
//   - selection, the scroll anchor and read-along still agree with the geometry
//     the band shifted.

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	fyneTheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// --- fixtures ----------------------------------------------------------------

// bandFixtureState is three prose verses long enough to wrap at a book width,
// so a band above verse 2 has real lines above and below it.
func bandFixtureState() *AppState {
	bd := &BibleData{
		Books: []string{"Ruth"},
		Verses: map[string]map[int][]Verse{"Ruth": {1: {
			{BookName: "Ruth", Book: "Ruth", Chapter: 1, Verse: 1,
				Text: "Now it happened in the days when the judges judged that there was a famine in the land."},
			{BookName: "Ruth", Book: "Ruth", Chapter: 1, Verse: 2,
				Text: "The name of the man was Elimelech, and the name of his wife Naomi, and they came into the country of Moab and continued there."},
			{BookName: "Ruth", Book: "Ruth", Chapter: 1, Verse: 3,
				Text: "Elimelech, Naomi's husband, died, and she was left with her two sons, and they took wives of the women of Moab."},
		}}},
	}
	return &AppState{Bible: bd, CurrentBook: "Ruth", CurrentChapter: 1, CurrentVersion: "web"}
}

// styledNoteFixture seeds received notes on Ruth 1 and lands the mirror on the
// newest, exactly as a navigation would. `verses` are the verses to file the
// notes on, OLDEST FIRST; the returned notes are in that same order.
func styledNoteFixture(t *testing.T, verses []int, texts []string) (*AppState, []StoredNote) {
	t.Helper()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	t.Cleanup(func() {
		setNotesEnabled(true)
		deleteAllNotes(appPrefs())
	})
	origNow := noteNow
	now := int64(1_700_000_000)
	noteNow = func() int64 { now++; return now }
	t.Cleanup(func() { noteNow = origNow })

	var notes []StoredNote
	for i, v := range verses {
		n, ok := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
			Book: "Ruth", Chapter: 1, VerseLo: v, Text: texts[i]})
		if !ok {
			t.Fatalf("seeding note %q failed", texts[i])
		}
		notes = append(notes, n)
	}
	st := bandFixtureState()
	applyNoteForCurrentChapter(st)
	if st.ActiveNote == "" {
		t.Fatal("precondition: the fixture must leave a note on the mirror")
	}
	return st, notes
}

// bandBox is the reserved band as a painted rectangle would occupy it.
func bandBox(p *styledReadingPane) paintedBox {
	return paintedBox{0, p.lay.BandY, 100000, p.lay.BandY + p.lay.BandH}
}

// --- the band, in the layout engine ------------------------------------------

// The band exists, sits immediately above the anchor verse's first line, and
// grows the chapter by exactly its own height.
func TestStyledLayoutReservesTheNoteBand(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := psalm23State()
	verses := st.Bible.GetChapter("Psalms", 23)
	p := testLayoutParams
	p.Width = 300
	p.BandVerse, p.BandH = 2, 77

	lay := layoutChapter(st, verses, p, fixedMeasure)
	if lay.BandLine < 0 {
		t.Fatal("no band was reserved for the anchor verse")
	}
	// THE PARAGRAPH RULE: the band opens above the whole
	// paragraph carrying the verse — never between two of its lines, which
	// would break the passage in half. So BandLine is the paragraph's FIRST
	// line, at or above the anchor verse's own line, and the anchor verse
	// lives inside [BandLine, BandLastLine].
	var verseLine int
	for _, vl := range lay.VerseLines {
		if vl.verse == 2 {
			verseLine = vl.line
		}
	}
	if lay.BandLine > verseLine {
		t.Errorf("BandLine = %d is BELOW verse 2's line %d — the band must open above the paragraph",
			lay.BandLine, verseLine)
	}
	if verseLine > lay.BandLastLine {
		t.Errorf("verse 2's line %d falls outside the band's paragraph [%d..%d]",
			verseLine, lay.BandLine, lay.BandLastLine)
	}
	// And the band abuts the paragraph's first line: nothing sits between.
	if got := lay.BandY + lay.BandH; got != lay.Lines[lay.BandLine].Y {
		t.Errorf("band bottom %.1f != anchor line top %.1f — the band must abut the line it opens",
			got, lay.Lines[lay.BandLine].Y)
	}
	base := testLayoutParams
	base.Width = 300
	plain := layoutChapter(st, verses, base, fixedMeasure)
	if got, wantH := lay.Height, plain.Height+77; got != wantH {
		t.Errorf("chapter height %.1f, want %.1f (grown by exactly the band)", got, wantH)
	}
	if plain.BandLine != -1 {
		t.Errorf("a layout with no band must report BandLine -1, got %d", plain.BandLine)
	}
}

// THE DENSITY CHECK. The band is the ONLY space the chapter gained: the text
// model is byte-identical, the wrap is identical, every run keeps its X/W, every
// line above the band keeps its Y, and every line from the band on is shifted by
// exactly BandH. Nothing else about the passage moves.
// laterParagraphVerse is the first verse of the second paragraph — an anchor
// that puts the note's band below the chapter's opening lines.
// twoParagraphState is long enough that groupVersesIntoParagraphs really
// splits it: the density test needs lines ABOVE the band to prove they do not
// move, and a one-paragraph chapter puts the band at line 0.
func twoParagraphState() *AppState {
	long := "And she said to them, Do not call me Naomi, call me Mara, for the Almighty " +
		"has dealt very bitterly with me, and I went out full and the LORD has brought " +
		"me home again empty, so why call me Naomi seeing the LORD has testified against me. "
	vs := make([]Verse, 0, 8)
	for i := 1; i <= 8; i++ {
		vs = append(vs, Verse{BookName: "Ruth", Book: "Ruth", Chapter: 2, Verse: i, Text: long})
	}
	bd := &BibleData{Books: []string{"Ruth"}, Verses: map[string]map[int][]Verse{"Ruth": {2: vs}}}
	return &AppState{Bible: bd, CurrentBook: "Ruth", CurrentChapter: 2, CurrentVersion: "web"}
}

func laterParagraphVerse(verses []Verse) int {
	paras := groupVersesIntoParagraphs(verses)
	if len(paras) > 1 && len(paras[1]) > 0 {
		return paras[1][0].Verse
	}
	if len(verses) > 0 {
		return verses[len(verses)-1].Verse
	}
	return 0
}

func TestStyledNoteBandIsTheOnlyInsertedSpace(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := twoParagraphState()
	verses := st.Bible.GetChapter("Ruth", 2)
	base := testLayoutParams
	base.Width = 320
	banded := base
	// Anchor in the SECOND paragraph, so the test keeps its teeth: with lines
	// above the band, "every line above keeps its Y" is a real assertion.
	// (The band opens above a whole paragraph now, so a first-paragraph
	// anchor would put it at line 0 with nothing above to check.)
	banded.BandVerse, banded.BandH = laterParagraphVerse(verses), 64

	plain := layoutChapter(st, verses, base, fixedMeasure)
	lay := layoutChapter(st, verses, banded, fixedMeasure)

	if lay.Text != plain.Text {
		t.Fatal("the band changed the selection text model — copy, selection offsets and share all read that string")
	}
	if len(lay.Lines) != len(plain.Lines) {
		t.Fatalf("the band re-wrapped the chapter: %d lines vs %d", len(lay.Lines), len(plain.Lines))
	}
	// The fixture's anchor verse (20) is in a LATER paragraph, so the band
	// still opens below the first line — now at that paragraph's top.
	if lay.BandLine < 1 {
		t.Fatalf("fixture must put the band below the first line, got BandLine %d", lay.BandLine)
	}
	for i := range plain.Lines {
		a, b := plain.Lines[i], lay.Lines[i]
		wantY := a.Y
		if i >= lay.BandLine {
			wantY += lay.BandH
		}
		if b.Y != wantY {
			t.Errorf("line %d Y = %.1f, want %.1f", i, b.Y, wantY)
		}
		if b.H != a.H {
			t.Errorf("line %d H = %.1f, want %.1f — the band must not inflate a line box", i, b.H, a.H)
		}
		if len(a.Runs) != len(b.Runs) {
			t.Fatalf("line %d run count moved: %d vs %d", i, len(b.Runs), len(a.Runs))
		}
		for j := range a.Runs {
			if a.Runs[j].X != b.Runs[j].X || a.Runs[j].W != b.Runs[j].W ||
				a.Runs[j].Offset != b.Runs[j].Offset {
				t.Errorf("line %d run %d moved horizontally or in the model", i, j)
			}
		}
	}
}

// LESSON 1 AND 2, as a property over the whole chapter: the band intersects NO
// line box. Every wash this pane paints is bounded to a line box, so this is
// what makes "a verse's wash can never reach into the band" structural rather
// than guarded.
func TestStyledNoteBandIsOutsideEveryLineBox(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	for _, tc := range []struct {
		name  string
		state *AppState
		book  string
		ch    int
		verse int
	}{
		{"prose", acts4ShareState(), "Acts", 4, 20},
		{"poetry", psalm23State(), "Psalms", 23, 2},
		{"mixed", exodus15State(), "Exodus", 15, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := testLayoutParams
			p.Width = 280
			p.BandVerse, p.BandH = tc.verse, 55
			lay := layoutChapter(tc.state, tc.state.Bible.GetChapter(tc.book, tc.ch), p, fixedMeasure)
			if lay.BandLine < 0 {
				t.Fatalf("no band reserved for verse %d", tc.verse)
			}
			lo, hi := lay.BandY, lay.BandY+lay.BandH
			for i, ln := range lay.Lines {
				if ln.Y < hi && lo < ln.Y+ln.H {
					t.Errorf("line %d [%.1f,%.1f) intersects the band [%.1f,%.1f)",
						i, ln.Y, ln.Y+ln.H, lo, hi)
				}
			}
		})
	}
}

// Verse 1 opens the chapter, so its band sits above line 0 and pushes it down —
// no collapse (the iOS first-paragraph special case has no equivalent here, and
// this pins that it does not need one). An anchor verse the chapter does not
// contain reserves nothing at all.
func TestStyledNoteBandFirstVerseAndMissingAnchor(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := psalm23State()
	verses := st.Bible.GetChapter("Psalms", 23)

	p := testLayoutParams
	p.Width = 300
	p.BandVerse, p.BandH = 1, 40
	lay := layoutChapter(st, verses, p, fixedMeasure)
	if lay.BandLine != 0 {
		t.Errorf("a band on verse 1 must open above line 0, got %d", lay.BandLine)
	}
	if lay.BandY != 0 || lay.Lines[0].Y != 40 {
		t.Errorf("band Y=%.1f, line 0 Y=%.1f — want 0 and 40 (the band cannot collapse at the top)",
			lay.BandY, lay.Lines[0].Y)
	}

	p.BandVerse = 999
	if miss := layoutChapter(st, verses, p, fixedMeasure); miss.BandLine != -1 || miss.BandH != 0 {
		t.Errorf("an absent anchor verse reserved a band anyway: line=%d h=%.1f", miss.BandLine, miss.BandH)
	}
}

// The PANE resolves an absent or chapter-level anchor to the chapter's first
// verse — one rule, so the layout engine keeps no special cases.
func TestStyledPaneParksAnOrphanAnchorAtTheTop(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st, _ := styledNoteFixture(t, []int{2}, []string{"anchored words"})
	st.NoteVerseLo = 0 // a chapter-level note: nothing to point at
	p := newStyledReadingPane(st, st.Bible.GetChapter("Ruth", 1))
	p.Resize(fyne.NewSize(520, 400))
	if p.lay.BandLine != 0 {
		t.Errorf("a chapter-level note must park the band above line 0, got %d", p.lay.BandLine)
	}
	if !p.noteGeom.present {
		t.Error("the sticker vanished instead of parking at the top")
	}
}

// --- the band and the washes --------------------------------------------------

// NOTHING WASHES THE BAND — not the note's own mark, not the narration. This is
// the Android defect (an inflated ascent let the verse's highlight grow into the
// band and slide under the bubble) asked of the whole render.
func TestStyledNoteBandNotWashed(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st, _ := styledNoteFixture(t, []int{2}, []string{"the words the band is for"})
	p := newStyledReadingPane(st, st.Bible.GetChapter("Ruth", 1))
	p.Resize(fyne.NewSize(520, 400))
	r := p.CreateRenderer().(*styledPaneRenderer)
	p.setReadAlongVerse(2) // the narration on the very verse the band points at
	r.rebuild()
	r.Layout(fyne.NewSize(520, p.MinSize().Height))

	if p.lay.BandLine < 0 || p.lay.BandH <= 0 {
		t.Fatal("precondition: the fixture must reserve a band")
	}
	band := bandBox(p)

	hl := styledHighlightBoxes(p, r)
	if len(hl) == 0 {
		t.Fatal("precondition: the note's own mark must wash its verse")
	}
	for i, b := range hl {
		if b.overlaps(band) {
			t.Errorf("chapter wash %d (y %.1f..%.1f) reaches into the band (y %.1f..%.1f)",
				i, b.y0, b.y1, band.y0, band.y1)
		}
	}
	narration := 0
	for _, o := range r.Objects() {
		rect, ok := o.(*canvas.Rectangle)
		if !ok || !rect.Visible() || rect.FillColor != color.Color(styledReadAlongTint) {
			continue
		}
		narration++
		pos, sz := rect.Position(), rect.Size()
		b := paintedBox{pos.X, pos.Y, pos.X + sz.Width, pos.Y + sz.Height}
		if b.overlaps(band) {
			t.Errorf("narration wash (y %.1f..%.1f) reaches into the band (y %.1f..%.1f)",
				b.y0, b.y1, band.y0, band.y1)
		}
	}
	if narration == 0 {
		t.Fatal("precondition: the narration must wash the verse it is reading")
	}
}

// --- what the reader SEES -----------------------------------------------------

// styledNoteSeen lays the pane out in a real window and returns what is on the
// flattened screen (seenText, screen_seen_test.go).
func styledNoteSeen(t *testing.T, p *styledReadingPane, size fyne.Size) string {
	t.Helper()
	return seenText(t, p, size)
}

// The sender's words, the byline, the counts and both verbs are SEEN — in the
// text, on the pane itself, with no banner anywhere.
func TestStyledStickerIsSeen(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st, _ := styledNoteFixture(t,
		[]int{1, 2}, []string{"alpha words on one", "beta words on two"})
	p := newStyledReadingPane(st, st.Bible.GetChapter("Ruth", 1))
	seen := styledNoteSeen(t, p, fyne.NewSize(560, 700))

	for _, want := range []string{
		"beta words on two",      // the open note's own words
		"note from friend",       // the byline, in the app's voice
		"1 of 2 in this chapter", // the honest count
		"–",                      // the minimize verb; the delete is an ICON now, not a glyph —
		// see TestStyledClosingGlyphMatchesWhatItDoes for what it wears.
	} {
		if !strings.Contains(seen, want) {
			t.Errorf("the reader cannot see %q on the sticker.\nseen:\n%s", want, seen)
		}
	}
	if strings.Contains(seen, "alpha words on one") {
		t.Error("the other note's words are on screen; only the count stands for it")
	}
	// The passage is still there — the sticker sits in the text, it does not
	// replace it.
	if !strings.Contains(seen, "elimelech") {
		t.Error("the verses vanished behind the sticker")
	}
}

// Minimized and suppressed both collapse to the pill, and the pill carries the
// whole set's count — minimizing one note must not make the rest invisible.
func TestStyledStickerPillIsSeen(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	for _, tc := range []struct {
		name string
		act  func(st *AppState)
	}{
		{"minimized", func(st *AppState) { hideCurrentNote(st) }},
		{"suppressed by a foreign mark", func(st *AppState) {
			st.setHL(hlSearch, "Ruth", 1, 3, 3)
			applyNoteForCurrentChapter(st)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st, _ := styledNoteFixture(t, []int{1, 2, 3},
				[]string{"alpha words", "beta words", "gamma words"})
			tc.act(st)
			p := newStyledReadingPane(st, st.Bible.GetChapter("Ruth", 1))
			seen := styledNoteSeen(t, p, fyne.NewSize(560, 700))

			if !strings.Contains(seen, "notes · 3") {
				t.Errorf("the pill does not carry the store's count.\nseen:\n%s", seen)
			}
			for _, body := range []string{"alpha words", "beta words", "gamma words"} {
				if strings.Contains(seen, body) {
					t.Errorf("%q is on screen while the sticker is collapsed", body)
				}
			}
		})
	}
}

// --- the verbs ----------------------------------------------------------------

// seenPaneButtons lays the pane out in a real window and returns every VISIBLE,
// laid-out button on it — the reader's own set of controls, not the tree's.
//
// It stops the walk AT a button (seenLeaves descends through widgets into their
// renderers, so it only ever reports a button's inner label), the same shape
// seenBannerButton uses for the banner.
func seenPaneButtons(t *testing.T, p *styledReadingPane, size fyne.Size) []*widget.Button {
	t.Helper()
	w := test.NewWindow(p)
	t.Cleanup(w.Close)
	w.Resize(size)
	return visiblePaneButtons(p)
}

func visiblePaneButtons(p *styledReadingPane) []*widget.Button {
	var out []*widget.Button
	var walk func(o fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		if o == nil || !o.Visible() {
			return
		}
		if b, ok := o.(*widget.Button); ok {
			if sz := b.Size(); sz.Width > 0 && sz.Height > 0 {
				out = append(out, b)
			}
			return
		}
		if c, ok := o.(*fyne.Container); ok {
			for _, ch := range c.Objects {
				walk(ch)
			}
			return
		}
		if wdg, ok := o.(fyne.Widget); ok {
			if r := test.WidgetRenderer(wdg); r != nil {
				for _, ch := range r.Objects() {
					walk(ch)
				}
			}
		}
	}
	walk(p)
	return out
}

func seenPaneButton(t *testing.T, p *styledReadingPane, size fyne.Size, text string) *widget.Button {
	t.Helper()
	for _, b := range seenPaneButtons(t, p, size) {
		if b.Text == text {
			return b
		}
	}
	return nil
}

// Every control on the sticker ends on its verb AND on the screen (the M8
// class, on the new surface): the counts advances focus, − minimizes, ✕
// deletes, and the pill restores.
func TestStyledStickerVerbsFire(t *testing.T) {
	size := fyne.NewSize(560, 700)

	t.Run("the counts advances focus", func(t *testing.T) {
		app := test.NewApp()
		defer app.Quit()
		st, notes := styledNoteFixture(t, []int{1, 2},
			[]string{"alpha words on one", "beta words on two"})
		p := newStyledReadingPane(st, st.Bible.GetChapter("Ruth", 1))
		// The counts control is the transparent button over the accent span:
		// the one button on the sticker carrying no text of its own.
		var counts *widget.Button
		for _, b := range seenPaneButtons(t, p, size) {
			if b.Text == "" {
				counts = b
			}
		}
		if counts == nil {
			t.Fatal("no visible counts control on a passage holding two notes")
		}
		test.Tap(counts)
		if st.NoteID != notes[0].ID {
			t.Errorf("the counts tap did not advance to the next note: NoteID %d, want %d",
				st.NoteID, notes[0].ID)
		}
		if !strings.Contains(styledNoteSeen(t, newStyledReadingPane(st, st.Bible.GetChapter("Ruth", 1)), size),
			"alpha words on one") {
			t.Error("focus advanced but the screen did not follow")
		}
	})

	t.Run("– minimizes", func(t *testing.T) {
		app := test.NewApp()
		defer app.Quit()
		st, notes := styledNoteFixture(t, []int{1, 2},
			[]string{"alpha words on one", "beta words on two"})
		p := newStyledReadingPane(st, st.Bible.GetChapter("Ruth", 1))
		hide := seenPaneButton(t, p, size, "–")
		if hide == nil {
			t.Fatal("no visible minimize control on the sticker")
		}
		test.Tap(hide)
		for _, n := range allNotesForBrowsing(appPrefs()) {
			if n.ID == notes[1].ID && !n.Minimized {
				t.Error("the store did not record the minimize")
			}
		}
		seen := styledNoteSeen(t, newStyledReadingPane(st, st.Bible.GetChapter("Ruth", 1)), size)
		if !strings.Contains(seen, "notes · 2") {
			t.Errorf("the pill (with the set's count) did not replace the bubble.\nseen:\n%s", seen)
		}
		if strings.Contains(seen, "beta words on two") {
			t.Error("the minimized note's words are still on screen")
		}
	})

	t.Run("✕ deletes and the rest surfaces", func(t *testing.T) {
		app := test.NewApp()
		defer app.Quit()
		st, _ := styledNoteFixture(t, []int{1, 2},
			[]string{"alpha words on one", "beta words on two"})
		p := newStyledReadingPane(st, st.Bible.GetChapter("Ruth", 1))
		// A bin, because on a RECEIVED note this control deletes. It is an
		// icon-only button now, so it is found by having no text.
		del := seenPaneButtonIcon(t, p, size)
		if del == nil {
			t.Fatal("no visible delete control on the sticker")
		}
		test.Tap(del)
		if n := storedNoteCount(appPrefs()); n != 1 {
			t.Errorf("one delete left %d notes, want 1", n)
		}
		seen := styledNoteSeen(t, newStyledReadingPane(st, st.Bible.GetChapter("Ruth", 1)), size)
		if !strings.Contains(seen, "alpha words on one") {
			t.Errorf("the remaining note did not surface after the delete.\nseen:\n%s", seen)
		}
		if strings.Contains(seen, "beta words on two") {
			t.Error("the deleted note is still on screen")
		}
	})

	t.Run("the pill restores", func(t *testing.T) {
		app := test.NewApp()
		defer app.Quit()
		st, _ := styledNoteFixture(t, []int{2}, []string{"beta words on two"})
		hideCurrentNote(st)
		p := newStyledReadingPane(st, st.Bible.GetChapter("Ruth", 1))
		// The pill's press target carries NO text of its own — the label is
		// drawn (canvas.Text at the who size, in the muted ink) so that it
		// matches iOS's chip rather than the theme's button styling, exactly as
		// the counts control above does. A collapsed sticker has this one
		// control, so "the button with no text" names it unambiguously.
		var pill *widget.Button
		for _, b := range seenPaneButtons(t, p, size) {
			if b.Text == "" {
				pill = b
			}
		}
		if pill == nil {
			t.Fatal("no visible pill after the note was minimized")
		}
		test.Tap(pill)
		seen := styledNoteSeen(t, newStyledReadingPane(st, st.Bible.GetChapter("Ruth", 1)), size)
		if !strings.Contains(seen, "beta words on two") {
			t.Errorf("the pill press did not bring the note back.\nseen:\n%s", seen)
		}
	})
}

func TestStyledCountTapScrollsToTheNextNote(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer resetStyledWiring()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	origNow := noteNow
	now := int64(1_700_000_000)
	noteNow = func() int64 { now++; return now }
	defer func() { noteNow = origNow }()

	older, _ := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
		Book: "Psalms", Chapter: 119, VerseLo: 3, Text: "near the chapter start"})
	newer, _ := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
		Book: "Psalms", Chapter: 119, VerseLo: 25, Text: "near the chapter end"})
	st := longPsalmState()
	st.CurrentVersion = "web"
	applyNoteForCurrentChapter(st)
	if st.NoteID != newer.ID {
		t.Fatalf("precondition: newest note %d is selected, got %d", newer.ID, st.NoteID)
	}

	size := fyne.NewSize(420, 300)
	render := func() fyne.CanvasObject {
		return styledReadingScrollArea(st, st.Bible.GetChapter("Psalms", 119), lightPalette)
	}
	w := test.NewWindow(render())
	defer w.Close()
	w.Resize(size)
	w.Canvas().Content().Refresh()
	var rebuilt fyne.Window
	defer func() {
		if rebuilt != nil {
			rebuilt.Close()
		}
	}()
	st.showReading = func() {
		rebuilt = test.NewWindow(render())
		rebuilt.Resize(size)
		rebuilt.Canvas().Content().Refresh()
	}

	var counts *widget.Button
	for _, b := range visiblePaneButtons(styledPane) {
		if b.Text == "" && b.Icon == nil {
			counts = b
			break
		}
	}
	if counts == nil {
		t.Fatal("no visible note-count control")
	}
	test.Tap(counts)

	if st.NoteID != older.ID {
		t.Fatalf("count tap selected note %d, want %d", st.NoteID, older.ID)
	}
	if st.forceReposition {
		t.Error("styled rebuild did not consume the placement request")
	}
	if styledRestoreArmed {
		t.Error("the previous note's viewport remained armed after the count tap")
	}
	for i := 0; i < 5 && styledScroll.Offset.Y == 0; i++ {
		rebuilt.Resize(fyne.NewSize(size.Width, size.Height+float32(i+1)))
		rebuilt.Canvas().Content().Refresh()
	}
	wantY := styledPane.highlightY() - 24
	if wantY < 0 {
		wantY = 0
	}
	if got := styledScroll.Offset.Y; got < wantY-30 || got > wantY {
		t.Errorf("count tap offset %.1f, want near the next note at %.1f (highlight=%v first=%d user=%v ceded=%v scroll=%v content=%v)",
			got, wantY, styledPane.highlightOwnsScroll(), styledPane.highlightFirstLine(),
			styledUserScrolled, styledHighlightCeded, styledScroll.Size(), styledPane.MinSize())
	}
}

func TestStyledCountTapOnTheSameAnchorPreservesTheTop(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer resetStyledWiring()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	origNow := noteNow
	now := int64(1_700_000_000)
	noteNow = func() int64 { now++; return now }
	defer func() { noteNow = origNow }()

	older, _ := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
		Book: "Psalms", Chapter: 119, VerseLo: 25, Text: "first same-anchor note"})
	newer, _ := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
		Book: "Psalms", Chapter: 119, VerseLo: 25, VerseHi: 26, Text: "second same-anchor note"})
	st := longPsalmState()
	st.CurrentVersion = "web"
	applyNoteForCurrentChapter(st)
	if st.NoteID != newer.ID {
		t.Fatalf("precondition: newest note %d is selected, got %d", newer.ID, st.NoteID)
	}

	size := fyne.NewSize(420, 300)
	render := func() fyne.CanvasObject {
		return styledReadingScrollArea(st, st.Bible.GetChapter("Psalms", 119), lightPalette)
	}
	w := test.NewWindow(render())
	defer w.Close()
	w.Resize(size)
	w.Canvas().Content().Refresh()
	styledScroll.Offset = fyne.NewPos(0, 0)
	var rebuilt fyne.Window
	defer func() {
		if rebuilt != nil {
			rebuilt.Close()
		}
	}()
	st.showReading = func() {
		rebuilt = test.NewWindow(render())
		rebuilt.Resize(size)
		rebuilt.Canvas().Content().Refresh()
	}

	var counts *widget.Button
	for _, b := range visiblePaneButtons(styledPane) {
		if b.Text == "" && b.Icon == nil {
			counts = b
			break
		}
	}
	if counts == nil {
		t.Fatal("no visible note-count control")
	}
	test.Tap(counts)

	if st.NoteID != older.ID {
		t.Fatalf("count tap selected note %d, want %d", st.NoteID, older.ID)
	}
	if got := styledScroll.Offset.Y; got != 0 {
		t.Fatalf("same-anchor count tap moved the top viewport to %.1f", got)
	}
	if !styledHighlightCeded {
		t.Error("same-anchor top carry did not claim the viewport from the standing wash")
	}
}

// --- the sticker and the selection --------------------------------------------

// A press inside the card must not start a selection under it, and a drag begun
// there must not select the verses the bubble is sitting over. A press one point
// outside still selects, so the guard is a rectangle and not a stand-down.
func TestStyledStickerClicksDoNotSelect(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st, _ := styledNoteFixture(t, []int{2}, []string{"the words the band is for"})
	p := newStyledReadingPane(st, st.Bible.GetChapter("Ruth", 1))
	p.Resize(fyne.NewSize(560, 700))
	if !p.noteGeom.present {
		t.Fatal("precondition: the sticker must be present")
	}
	g := p.noteGeom
	inside := fyne.NewPos(g.card.X+g.card.W/2, g.card.Y+4)

	p.MouseDown(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: inside}, Button: desktop.MouseButtonPrimary})
	if p.selStart != -1 {
		t.Errorf("a press on the card started a selection at %d", p.selStart)
	}
	p.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(inside.X, inside.Y+120)}})
	if p.selStart != -1 || p.selEnd != -1 {
		t.Errorf("a drag from the card selected [%d,%d)", p.selStart, p.selEnd)
	}
	p.DragEnd()

	outside := fyne.NewPos(g.card.X+g.card.W/2, g.card.Y+g.card.H+p.lh)
	p.MouseDown(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: outside}, Button: desktop.MouseButtonPrimary})
	if p.selStart < 0 {
		t.Error("a press below the sticker did not reach the text — the guard is too wide")
	}
}

// Selection geometry still round-trips with the band present, above it and
// below it, and the study menu's verse resolution still finds the anchor verse.
func TestStyledSelectionSurvivesTheBand(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	// Ruth 1's three verses are ONE paragraph, so the paragraph-level band
	// opens at line 0. Selection across the band is the point of the test,
	// and lines below it are what the drag crosses.
	st, _ := styledNoteFixture(t, []int{2}, []string{"the words the band is for"})
	p := newStyledReadingPane(st, st.Bible.GetChapter("Ruth", 1))
	p.Resize(fyne.NewSize(560, 700))
	if p.lay.BandLine < 0 || len(p.lay.Lines) < 3 {
		t.Fatalf("fixture must reserve a band with lines to drag across, got BandLine %d of %d",
			p.lay.BandLine, len(p.lay.Lines))
	}
	// Probe above the band when a line exists there (the band can be at line
	// 0 when its paragraph opens the chapter), on the band's own line, and at
	// the far end of the passage.
	probes := []int{p.lay.BandLine, len(p.lay.Lines) - 1}
	if p.lay.BandLine > 0 {
		probes = append([]int{p.lay.BandLine - 1}, probes...)
	}
	for _, li := range probes {
		ln := p.lay.Lines[li]
		off := ln.StartOffset + 2
		if off >= ln.EndOffset {
			continue
		}
		x := p.xForOffset(li, off)
		y := ln.Y + ln.H/2
		if got := p.offsetAtPos(fyne.NewPos(x, y)); got != off {
			t.Errorf("line %d: offsetAtPos(xForOffset(%d)) = %d", li, off, got)
		}
	}
	// And a selection over the anchor verse still resolves to it.
	p.setSelection(p.lay.Lines[p.lay.BandLine].StartOffset, p.lay.Lines[p.lay.BandLine].EndOffset)
	if sp := p.selectedVerseSpan(); !sp.valid() {
		t.Error("a selection on the banded line resolved to no verse")
	}
}

// --- scroll -------------------------------------------------------------------

// The anchor round-trips through the shifted geometry: capture and restore read
// the same line Ys, so the band cannot displace a restored reading position.
func TestStyledAnchorRoundTripsWithTheBand(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st, _ := styledNoteFixture(t, []int{2}, []string{"the words the band is for"})
	p := newStyledReadingPane(st, st.Bible.GetChapter("Ruth", 1))
	p.Resize(fyne.NewSize(560, 700))
	for _, v := range []int{1, 2, 3} {
		y, ok := p.yForVerse(v)
		if !ok {
			t.Fatalf("verse %d has no Y", v)
		}
		if got, _ := p.verseAtY(y); got != v {
			t.Errorf("verseAtY(yForVerse(%d)) = %d", v, got)
		}
	}
	// A y INSIDE the band belongs to the preceding verse — the band is not part
	// of the verse it points at.
	if got, _ := p.verseAtY(p.lay.BandY + 1); got != 1 {
		t.Errorf("a scroll offset inside the band attributed to verse %d, want the preceding verse 1", got)
	}
}

// The note's own mark scrolls to the BAND, not to the line under it — otherwise
// the bubble explaining the mark arrives clipped above the fold.
func TestStyledHighlightScrollShowsTheBubble(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st, _ := styledNoteFixture(t, []int{2}, []string{"the words the band is for"})
	if !st.mark.fromNote() {
		t.Fatal("precondition: the note must own the mark")
	}
	// The route that lands a reader here with the note's mark lit — a link, or
	// the browser's open — is explicit; a plain entry arrives nowhere now.
	st.forceReposition = true
	p := newStyledReadingPane(st, st.Bible.GetChapter("Ruth", 1))
	p.Resize(fyne.NewSize(560, 700))
	if !p.highlightOwnsScroll() {
		t.Fatal("precondition: the note's mark must own the scroll")
	}
	if got := p.highlightY(); got != p.lay.BandY {
		t.Errorf("highlightY() = %.1f, want the band top %.1f (the bubble must arrive with its verse)",
			got, p.lay.BandY)
	}
}

// --- the shape ----------------------------------------------------------------

// ONE path for card and tail, and never an 8-digit hex colour — the two defects
// notes_bubble.go records, pinned this time rather than commented.
func TestStyledNoteBubbleIsOnePath(t *testing.T) {
	res := noteBubblePathSVG(240, 80, lightPalette.SurfaceAlt, lightPalette.Border, true)
	svg := string(res.Content())

	if n := strings.Count(svg, "<path"); n != 1 {
		t.Errorf("%d paths in the bubble — card and tail must be ONE outline, or the card's bottom border runs across the tail's mouth", n)
	}
	if regexp.MustCompile(`#[0-9A-Fa-f]{8}\b`).MatchString(svg) {
		t.Error("an 8-digit hex colour is in the SVG — Fyne's loader rejects it outright and the image silently fails to load")
	}
	// The outline really does carry both the corners and the tail apex: four
	// quadratic corners, and a point below the card's own bottom edge.
	if n := strings.Count(svg, "Q"); n != 4 {
		t.Errorf("%d corner curves in the outline, want 4", n)
	}
	// The apex is at the tail's centre (noteTailInset + noteTailWidth/2 = 33),
	// noteTailDepth below the card's bottom edge.
	if !strings.Contains(svg, "L33.0 88.5") {
		t.Errorf("the tail apex is not on the outline.\nsvg = %s", svg)
	}
}

// countColorNear counts pixels close to c inside a rect of the captured image.
func countColorNear(img image.Image, c color.NRGBA, x, y, w, h int) int {
	wantR, wantG, wantB := uint32(c.R)<<8, uint32(c.G)<<8, uint32(c.B)<<8
	b := img.Bounds()
	n := 0
	for py := y; py < y+h; py++ {
		for px := x; px < x+w; px++ {
			if px < b.Min.X || py < b.Min.Y || px >= b.Max.X || py >= b.Max.Y {
				continue
			}
			r0, g0, b0, _ := img.At(px, py).RGBA()
			if absDiff(r0, wantR)+absDiff(g0, wantG)+absDiff(b0, wantB) <= 20<<8 {
				n++
			}
		}
	}
	return n
}

// --- snapshots ------------------------------------------------------------------

// The sticker on the real paper, in both themes — env-gated PNGs for the eye,
// with assertions that keep it honest in CI: the card's own surface appears
// inside the band, and the accent appears on the counts span.
func TestStyledStickerSnapshots(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	realTheme := &bibleTheme{fonts: loadBookFonts(), uiFonts: loadUIFonts()}

	for _, tc := range []struct {
		name    string
		pal     palette
		variant fyne.ThemeVariant
	}{
		{"light", lightPalette, fyneTheme.VariantLight},
		{"dark", darkPalette, fyneTheme.VariantDark},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// The VERBS are widget.Buttons, so their label colour comes from the
			// theme rather than from p.pal. Forcing the variant (the test
			// driver's Settings has no setter — forcedVariant, icons_themed_test.go)
			// is what makes the dark snapshot honest instead of showing a light
			// theme's foreground on a dark card.
			app.Settings().SetTheme(forcedVariant{Theme: realTheme, v: tc.variant})
			st, _ := styledNoteFixture(t, []int{1, 2},
				[]string{"alpha words on one", "beta words on two"})
			p := newStyledReadingPane(st, st.Bible.GetChapter("Ruth", 1))
			p.pal = tc.pal

			w := test.NewWindow(container.NewStack(canvas.NewRectangle(tc.pal.Surface), p))
			defer w.Close()
			w.Resize(fyne.NewSize(520, 420))
			p.Refresh()
			img := w.Canvas().Capture()

			if !p.noteGeom.present {
				t.Fatal("no sticker to snapshot")
			}
			if n := countColorNear(img, tc.pal.SurfaceAlt,
				int(p.noteGeom.card.X)+6, int(p.noteGeom.card.Y)+6,
				int(p.noteGeom.card.W)-12, int(p.noteGeom.cardH)-12); n < 40 {
				t.Errorf("the card's own surface barely appears inside the band (%d px)", n)
			}
			if n := countColorNear(img, tc.pal.Accent,
				int(p.noteGeom.counts.X)-2, int(p.noteGeom.counts.Y)-2,
				int(p.noteGeom.counts.W)+4, int(p.noteGeom.counts.H)+4); n < 3 {
				t.Errorf("the counts span is not drawn in the accent (%d px) — it must look pressable", n)
			}
			if dir := os.Getenv("BIBLETEXT_PANE_SNAPSHOT_DIR"); dir != "" {
				writePNG(t, filepath.Join(dir, "styled-note-sticker-"+tc.name+".png"), img)
			}
		})
	}
}

// A chapter whose ONLY notes cannot be placed here: the sticker parks its pill
// at the top of the passage carrying the R4 sentence, and the banner keeps the
// per-note rows underneath it. The count and the sentences say the same thing
// twice; that is deliberate and named (the shared composition is not forked so
// four surfaces stay byte-equal), and it is the honest floor rather than losing
// a surface the reader had.
func TestStyledStickerUnplacedOnly(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	st := bandFixtureState()
	st.Bible.Books = append(st.Bible.Books, "Esther")
	st.Bible.Verses["Esther"] = map[int][]Verse{4: {
		{BookName: "Esther", Book: "Esther", Chapter: 4, Verse: 1, Text: "esther one two three four"}}}
	st.CurrentBook, st.CurrentChapter = "Esther", 4
	addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "webc",
		Book: "Esther", Chapter: 4, VerseLo: 1, Text: "greek esther words"})
	applyNoteForCurrentChapter(st)

	p := newStyledReadingPane(st, st.Bible.GetChapter("Esther", 4))
	seen := styledNoteSeen(t, p, fyne.NewSize(560, 500))
	if !strings.Contains(seen, "cannot be shown in this translation") {
		t.Errorf("the unplaced-only pill is not on the pane.\nseen:\n%s", seen)
	}
	if strings.Contains(seen, "greek esther words") {
		t.Error("an unplaceable note's words are drawn in a translation that cannot hold them")
	}
	if p.lay.BandLine != 0 {
		t.Errorf("an unplaced-only chapter must park the band at the top, got BandLine %d", p.lay.BandLine)
	}
}

// --- the banner's residue -------------------------------------------------------

// WHAT THE STICKER CANNOT CARRY KEEPS THE BANNER. With the styled pane live the
// banner is gone for the ordinary open/minimized note — the sticker draws it —
// but the notice path and the R4 unplaced sentences still come from it. Flip the
// pane back to the legacy Entry pane (the documented one-line revert) and the
// whole banner returns, because that pane has no sticker.
func TestBannerKeepsWhatTheStickerCannot(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	// Ask the WINDOWS/LINUX question. The host is darwin, where the platform
	// constant is already true, so without pinning the composition every
	// assertion below would be testing the Apple sticker's suppression instead.
	origPane, origSticker := useStyledPane, nativeNoteSticker
	nativeNoteSticker = func() bool { return false || useStyledPane() } // the !darwin && !android composition
	useStyledPane = func() bool { return true }
	defer func() { useStyledPane, nativeNoteSticker = origPane, origSticker }()

	st, _ := styledNoteFixture(t, []int{2}, []string{"beta words on two"})

	if b := buildNoteBanner(st); b != nil {
		t.Errorf("the banner drew an ordinary note the pane's sticker already draws: %v",
			seenText(t, b, fyne.NewSize(560, 400)))
	}

	// The notice path is untouched on every platform.
	st.NoteNotice = "This note could not be read."
	if b := buildNoteBanner(st); b == nil {
		t.Error("the notice lost its only surface")
	} else if seen := seenText(t, b, fyne.NewSize(560, 200)); !strings.Contains(seen, "could not be read") {
		t.Errorf("the notice is not on screen: %s", seen)
	}
	st.NoteNotice = ""

	// The R4 group keeps the banner: its sentence ("The numbering here does not
	// correspond to the note's") does not fit a one-band sticker without
	// doubling its height, so those rows survive as the banner's whole residue.
	// Greek Esther is the incommensurable arm (notes_plan_test.go's recipe).
	deleteAllNotes(appPrefs())
	st.Bible.Books = append(st.Bible.Books, "Esther")
	st.Bible.Verses["Esther"] = map[int][]Verse{4: {
		{BookName: "Esther", Book: "Esther", Chapter: 4, Verse: 1, Text: "esther one"}}}
	st.CurrentBook, st.CurrentChapter = "Esther", 4
	addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "webc",
		Book: "Esther", Chapter: 4, VerseLo: 1, Text: "greek esther words"})
	applyNoteForCurrentChapter(st)
	plan := buildChapterPlan(st, appPrefs(), st.Bible)
	if len(plan.Unplaced) != 1 {
		t.Fatalf("fixture must produce exactly one unplaced note, got %d", len(plan.Unplaced))
	}
	b := buildNoteBanner(st)
	if b == nil {
		t.Fatal("the R4 sentences lost their only surface")
	}
	seen := seenText(t, b, fyne.NewSize(560, 300))
	if !strings.Contains(seen, "esther 4:1") || !strings.Contains(seen, "does not correspond") {
		t.Errorf("the unplaced note has no visible trace: %s", seen)
	}
	if strings.Contains(seen, "greek esther words") {
		t.Error("the residual strip drew the note's words; only the sticker draws a message")
	}

	// And the revert path brings the whole banner back.
	useStyledPane = func() bool { return false }
	deleteAllNotes(appPrefs())
	st.CurrentBook, st.CurrentChapter = "Ruth", 1
	addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
		Book: "Ruth", Chapter: 1, VerseLo: 2, Text: "beta words on two"})
	applyNoteForCurrentChapter(st)
	back := buildNoteBanner(st)
	if back == nil {
		t.Fatal("flipping the pane back left the reader with no note surface at all")
	}
	if seen := seenText(t, back, fyne.NewSize(560, 400)); !strings.Contains(seen, "beta words on two") {
		t.Errorf("the legacy pane's banner does not draw the note: %s", seen)
	}
}

// The band gives the card the SAME air above as below. iOS gets the top gap
// free from paragraphSpacingBefore (its band opens above a whole paragraph);
// this pane anchors to the verse's own line, so without this the card butted
// against the line above — 0pt against 19pt, and the mismatch was obvious at a
// glance: both the note and the pill had less air above them than on iOS.
func TestStyledNoteBandIsSymmetric(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	for _, tc := range []struct {
		name string
		pill bool
	}{{"expanded", false}, {"pill", true}} {
		t.Run(tc.name, func(t *testing.T) {
			st, _ := styledNoteFixture(t, []int{2}, []string{"A note with air on both sides."})
			if tc.pill {
				hideCurrentNote(st)
			}
			p := newStyledReadingPane(st, st.Bible.GetChapter("Ruth", 1))
			w := test.NewWindow(p)
			defer w.Close()
			w.Resize(fyne.NewSize(520, 420))
			p.Refresh()

			g := p.noteGeom
			if !g.present {
				t.Fatal("precondition: a sticker")
			}
			above := g.card.Y - p.lay.BandY
			below := (p.lay.BandY + p.lay.BandH) - (g.card.Y + g.card.H)
			if above < styledNoteGapAbv-0.6 || above > styledNoteGapAbv+0.6 {
				t.Errorf("gap above the card = %.1f, want %.1f", above, styledNoteGapAbv)
			}
			if below < styledNoteGapBlw-0.6 || below > styledNoteGapBlw+0.6 {
				t.Errorf("gap below the card = %.1f, want %.1f", below, styledNoteGapBlw)
			}
			// And the card still lands inside its own band.
			if g.card.Y < p.lay.BandY-0.1 || g.card.Y+g.card.H > p.lay.BandY+p.lay.BandH+0.1 {
				t.Errorf("card %.1f..%.1f escapes band %.1f..%.1f",
					g.card.Y, g.card.Y+g.card.H, p.lay.BandY, p.lay.BandY+p.lay.BandH)
			}
		})
	}
}

// TestStyledPillMatchesTheApplePill holds the collapsed marker to the SAME
// numbers and the same ink as iOS's chip, because the two are compared side by
// side and any drift shows immediately — in mimic mode the pills had stopped
// matching iOS in spacing, colouring and text colour.
//
// WHAT WENT WRONG BEFORE, and what this therefore guards: the pill was
// MEASURED at the who size (11pt semibold, matching btNoteWhoFont) and then
// DRAWN by a widget.Button, which renders its title in the THEME's size and
// foreground ink — 18pt body ink on this pane. So the text was two-thirds
// larger than the box had been sized for and shouted in a colour the note's own
// chrome never uses. Measuring and drawing had drifted apart with nothing
// holding them together; this test is that thing.
//
// It asserts the DRAWN objects, not the constants that produced them: a test
// that re-reads styledNoteWhoSz would have passed happily throughout the bug.
func TestStyledPillMatchesTheApplePill(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st, _ := styledNoteFixture(t, []int{2}, []string{"beta words on two"})
	hideCurrentNote(st)
	p := newStyledReadingPane(st, st.Bible.GetChapter("Ruth", 1))

	w := test.NewWindow(p)
	defer w.Close()
	w.Resize(fyne.NewSize(560, 700))

	r := p.CreateRenderer().(*styledPaneRenderer)
	r.Layout(fyne.NewSize(560, 700))
	g := p.noteGeom
	if !g.present || !g.pill {
		t.Fatal("the sticker is not a pill after hideCurrentNote")
	}

	// 1. THE BOX. iOS: gNoteView.frame height is kNotePill, corner radius
	//    kNotePill/2, width = title + 28 clamped to [86, column].
	if got, want := g.card.H, noteMetrics().PillH; got != want {
		t.Errorf("pill height %v, want the spec's PillH %v", got, want)
	}
	if r.notePill == nil {
		t.Fatal("no pill shape was built")
	}
	if got, want := r.notePill.CornerRadius, g.card.H/2; got != want {
		t.Errorf("pill corner radius %v, want half its height %v (iOS: kNotePill/2)", got, want)
	}
	if g.card.W < 86 {
		t.Errorf("pill width %v is under the floor iOS keeps (86)", g.card.W)
	}

	// 2. THE LABEL. iOS draws it in btNoteWhoFont (11pt semibold) in gNoteMuted.
	if len(r.noteTexts) != 1 {
		t.Fatalf("a collapsed sticker should draw exactly one text, got %d", len(r.noteTexts))
	}
	label := r.noteTexts[0]
	if got, want := label.TextSize, styledNoteWhoSz; got != want {
		t.Errorf("pill label is %vpt, want the who size %vpt — this is the exact "+
			"drift that made the label overflow a box measured for something smaller",
			got, want)
	}
	if !label.TextStyle.Bold {
		t.Error("pill label is not bold; iOS's chip is semibold (btNoteWhoFont)")
	}
	if got, want := label.Color, p.pal.TextMuted; got != want {
		t.Errorf("pill label ink is %v, want the muted ink %v — the who line is the "+
			"app's chrome and is muted throughout on every surface", got, want)
	}

	// 3. THE LABEL FITS ITS BOX. The whole point of measuring at the who size is
	//    that the drawn text then fits; assert the drawn width really does.
	if lw := label.MinSize().Width; lw > g.card.W {
		t.Errorf("pill label wants %vpx inside a %vpx pill — it would be clipped", lw, g.card.W)
	}

	// 4. THE PRESS TARGET COVERS THE WHOLE PILL, as iOS's chip does
	//    (chip.frame = gNoteView.bounds). A label that is drawn separately from
	//    its control is only safe while the two are placed from one table.
	if len(r.noteBtns) != 1 {
		t.Fatalf("a collapsed sticker should carry exactly one control, got %d", len(r.noteBtns))
	}
	btn := r.noteBtns[0]
	if btn.Text != "" {
		t.Errorf("the pill's control carries text %q; the label is drawn, so the "+
			"button must be a transparent hit target", btn.Text)
	}
	if got := btn.Position(); got != g.card.pos() {
		t.Errorf("press target at %v, pill at %v — a press would miss the label", got, g.card.pos())
	}
	if got := btn.Size(); got != g.card.size() {
		t.Errorf("press target is %v, pill is %v — iOS's chip fills its bounds", got, g.card.size())
	}
}

// THE CLOSING GLYPH SAYS WHAT THE PRESS DOES.
//
// A received note is deleted and uses a bin. An own note is transient and the
// control only dismisses it, so it uses ✕. Distinct glyphs keep the destructive
// action unambiguous.
func TestStyledClosingGlyphMatchesWhatItDoes(t *testing.T) {
	size := fyne.NewSize(560, 700)

	t.Run("a received note offers a bin", func(t *testing.T) {
		app := test.NewApp()
		defer app.Quit()
		st, _ := styledNoteFixture(t, []int{2}, []string{"fixture styled message alpha"})
		p := newStyledReadingPane(st, st.Bible.GetChapter("Ruth", 1))
		if seenPaneButtonIcon(t, p, size) == nil {
			t.Error("a note that DELETES on press must wear the bin ICON")
		}
		if seenPaneButton(t, p, size, "✕") != nil {
			t.Error("a received note must not offer ✕ — that mark means dismiss")
		}
	})

	t.Run("your own note offers a dismiss", func(t *testing.T) {
		app := test.NewApp()
		defer app.Quit()
		setNotesEnabled(true)
		deleteAllNotes(appPrefs())
		defer deleteAllNotes(appPrefs())
		mine, ok := addNote(appPrefs(), StoredNote{Kind: noteKindMine, VersionID: "web",
			Book: "Ruth", Chapter: 1, VerseLo: 2, Text: "fixture outgoing alpha"})
		if !ok {
			t.Fatal("your note was not stored")
		}
		st := bandFixtureState()
		st.focusNote(mine.ID)
		applyNoteForCurrentChapter(st)
		p := newStyledReadingPane(st, st.Bible.GetChapter("Ruth", 1))
		if seenPaneButton(t, p, size, "✕") == nil {
			t.Error("your own note's control only dismisses, so it must wear ✕")
		}
		if seenPaneButtonIcon(t, p, size) != nil {
			t.Error("your own note must not wear a bin — that press does not delete, " +
				"and a bin that does not delete is worse than no bin")
		}
	})
}

// seenPaneButtonIcon finds the sticker's icon-only control — the bin. The
// delete verb wears a drawn icon rather than a glyph now, so it is identified
// by carrying an icon and no text.
func seenPaneButtonIcon(t *testing.T, p *styledReadingPane, size fyne.Size) *widget.Button {
	t.Helper()
	want := fyneTheme.DeleteIcon().Name()
	for _, b := range seenPaneButtons(t, p, size) {
		// By NAME, not merely "has an icon": the sticker's other controls can
		// be icon-only too, and a loose match finds whichever comes first.
		if b.Text == "" && b.Icon != nil && b.Icon.Name() == want {
			return b
		}
	}
	return nil
}
