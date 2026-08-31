package bibletext

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// The geometry the multi-band path must hold, and it is the same property the
// single band was built around: a band is ADVANCE, never line height, so no
// line box covers it and no wash can reach into it. With several bands that
// has to be true of each one independently, and they must not overlap.
func TestEveryBandIsDisjointFromEveryLine(t *testing.T) {
	verses := longEnoughForTwoParagraphs()
	paras := groupVersesIntoParagraphs(verses)
	if len(paras) < 2 {
		t.Fatalf("fixture must break into 2+ paragraphs, got %d", len(paras))
	}
	// One band on the first paragraph, one on the last.
	first := paras[0][0].Verse
	last := paras[len(paras)-1][0].Verse

	lay := layoutForBands(t, verses, []bandRequest{
		{Verse: first, H: 40, Count: 2},
		{Verse: last, H: 55, Count: 3},
	})
	if len(lay.Bands) != 2 {
		t.Fatalf("want a band per noted paragraph, got %d", len(lay.Bands))
	}
	// Each band carries its own paragraph's count, not the chapter's total.
	if lay.Bands[0].Count != 2 || lay.Bands[1].Count != 3 {
		t.Errorf("bands must carry their own paragraph's count, got %d and %d",
			lay.Bands[0].Count, lay.Bands[1].Count)
	}
	// Disjoint from every line box.
	for bi, b := range lay.Bands {
		if b.H <= 0 {
			t.Errorf("band %d reserved no height", bi)
		}
		for li, ln := range lay.Lines {
			if b.Y < ln.Y+ln.H && ln.Y < b.Y+b.H {
				t.Errorf("band %d [%.1f,%.1f) overlaps line %d [%.1f,%.1f); a band is "+
					"advance, so no line box may cover it or a wash will reach in",
					bi, b.Y, b.Y+b.H, li, ln.Y, ln.Y+ln.H)
			}
		}
	}
	// And disjoint from each other.
	for i := 1; i < len(lay.Bands); i++ {
		a, b := lay.Bands[i-1], lay.Bands[i]
		if a.Y+a.H > b.Y {
			t.Errorf("bands %d and %d overlap: [%.1f,%.1f) then [%.1f,%.1f)",
				i-1, i, a.Y, a.Y+a.H, b.Y, b.Y+b.H)
		}
		if a.Line >= b.Line {
			t.Errorf("bands must come out in line order, got lines %d then %d", a.Line, b.Line)
		}
	}
	// Each band knows where its paragraph ends — the scroll target needs it.
	for bi, b := range lay.Bands {
		if b.LastLine < b.Line {
			t.Errorf("band %d: last line %d is before its own line %d", bi, b.LastLine, b.Line)
		}
	}
}

// Two notes in ONE paragraph share a pill: that is what grouping them means.
func TestOneParagraphReservesOneBand(t *testing.T) {
	verses := longEnoughForTwoParagraphs()
	paras := groupVersesIntoParagraphs(verses)
	a := paras[0][0].Verse
	b := paras[0][len(paras[0])-1].Verse
	if a == b {
		t.Skip("fixture's first paragraph holds one verse")
	}
	lay := layoutForBands(t, verses, []bandRequest{
		{Verse: a, H: 40, Count: 2}, {Verse: b, H: 40, Count: 2},
	})
	if len(lay.Bands) != 1 {
		t.Fatalf("both requests fall in one paragraph, so one band: got %d", len(lay.Bands))
	}
}

// An empty request leaves the single-band path exactly as it was.
func TestNoBandRequestLeavesTheSingleBandPathAlone(t *testing.T) {
	verses := longEnoughForTwoParagraphs()
	lay := layoutForBands(t, verses, nil)
	if len(lay.Bands) != 0 {
		t.Errorf("no request, yet %d multi-bands were reserved", len(lay.Bands))
	}
	if lay.BandLine != -1 {
		t.Errorf("no request and no single band, yet BandLine = %d", lay.BandLine)
	}
}

// layoutForBands lays a chapter out with the given multi-band request, using
// the same fixed measure and params the single-band tests use.
func layoutForBands(t *testing.T, verses []Verse, bands []bandRequest) *chapterLayout {
	t.Helper()
	st := psalm23State()
	st.Bible.Verses["John"] = map[int][]Verse{3: verses}
	st.CurrentBook, st.CurrentChapter = "John", 3
	p := testLayoutParams
	p.Width = 300
	p.Bands = bands
	return layoutChapter(st, verses, p, fixedMeasure)
}

// End to end through the pane: two noted paragraphs, collapsed, gate on —
// two pills, each labelled with its own paragraph's count, each sitting in
// its own band.
func TestThePaneDrawsAPillPerNotedParagraph(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	prev := notesPillPerParagraph
	notesPillPerParagraph = true
	defer func() { notesPillPerParagraph = prev }()

	st := psalm23State()
	verses := longEnoughForTwoParagraphs()
	st.Bible.Verses["John"] = map[int][]Verse{3: verses}
	st.CurrentBook, st.CurrentChapter = "John", 3
	paras := groupVersesIntoParagraphs(verses)
	if len(paras) < 2 {
		t.Fatalf("fixture must break into 2+ paragraphs, got %d", len(paras))
	}
	// Two notes in the first paragraph, one in the last.
	for _, v := range []int{paras[0][0].Verse, paras[0][0].Verse + 1, paras[len(paras)-1][0].Verse} {
		n, ok := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
			Book: "John", Chapter: 3, VerseLo: v, Text: "fixture note"})
		if !ok {
			t.Fatalf("could not store a note on v%d", v)
		}
		setNoteMinimizedByID(appPrefs(), n.ID, true)
	}
	applyNoteForCurrentChapter(st)

	pane := newStyledReadingPane(st, verses)
	pane.Resize(fyne.NewSize(320, 900))

	if len(pane.pillGeoms) != 2 {
		t.Fatalf("two noted paragraphs, so two pills: got %d", len(pane.pillGeoms))
	}
	// Each pill carries its OWN paragraph's count, which is the whole point.
	labels := []string{pane.pillGeoms[0].pillText, pane.pillGeoms[1].pillText}
	if labels[0] == labels[1] {
		t.Errorf("both pills read %q; a paragraph with two notes and one with a "+
			"single note cannot carry the same label", labels[0])
	}
	// And they sit in different bands, in reading order.
	if pane.pillGeoms[0].card.Y >= pane.pillGeoms[1].card.Y {
		t.Errorf("pills must come out in reading order, got y=%.1f then y=%.1f",
			pane.pillGeoms[0].card.Y, pane.pillGeoms[1].card.Y)
	}
	// The single sticker stands down, or the reader sees the notes twice.
	if pane.noteGeom.present {
		t.Error("the single sticker must stand down while the pills are drawn")
	}
}

// Gate off, nothing changes: the single sticker draws and no pill exists.
func TestGateOffLeavesTheSingleStickerDrawing(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	// Explicit, now that the default is ON: this test is about the OTHER
	// setting, which the three native surfaces are still effectively in.
	prev := notesPillPerParagraph
	notesPillPerParagraph = false
	defer func() { notesPillPerParagraph = prev }()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	st := psalm23State()
	verses := longEnoughForTwoParagraphs()
	st.Bible.Verses["John"] = map[int][]Verse{3: verses}
	st.CurrentBook, st.CurrentChapter = "John", 3
	paras := groupVersesIntoParagraphs(verses)
	for _, v := range []int{paras[0][0].Verse, paras[len(paras)-1][0].Verse} {
		n, _ := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
			Book: "John", Chapter: 3, VerseLo: v, Text: "fixture note"})
		setNoteMinimizedByID(appPrefs(), n.ID, true)
	}
	applyNoteForCurrentChapter(st)

	pane := newStyledReadingPane(st, verses)
	pane.Resize(fyne.NewSize(320, 900))
	if len(pane.pillGeoms) != 0 {
		t.Fatalf("gate off must draw no pills, got %d", len(pane.pillGeoms))
	}
}

// The pills must be VISIBLE, not merely measured. This is the regression the
// geometry test above could not catch: the pills were built, placed in their
// bands, and then hidden a line later, because they had been added to the
// SINGLE sticker's object list — and positionNote hides all of those whenever
// the single sticker is absent, which is exactly when the pills are drawn. On
// screen: reserved bands with nothing in them.
func TestTheParagraphPillsAreActuallyVisible(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	prev := notesPillPerParagraph
	notesPillPerParagraph = true
	defer func() { notesPillPerParagraph = prev }()

	st := psalm23State()
	verses := longEnoughForTwoParagraphs()
	st.Bible.Verses["John"] = map[int][]Verse{3: verses}
	st.CurrentBook, st.CurrentChapter = "John", 3
	paras := groupVersesIntoParagraphs(verses)
	for _, v := range []int{paras[0][0].Verse, paras[len(paras)-1][0].Verse} {
		n, ok := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
			Book: "John", Chapter: 3, VerseLo: v, Text: "fixture note"})
		if !ok {
			t.Fatalf("could not store a note on v%d", v)
		}
		setNoteMinimizedByID(appPrefs(), n.ID, true)
	}
	applyNoteForCurrentChapter(st)

	pane := newStyledReadingPane(st, verses)
	rend, ok := pane.CreateRenderer().(*styledPaneRenderer)
	if !ok {
		t.Fatalf("unexpected renderer type")
	}
	rend.Layout(fyne.NewSize(320, 900))

	if len(rend.pillFrames) == 0 {
		t.Fatalf("gate on, two noted paragraphs, collapsed: no pills were built")
	}
	for i, f := range rend.pillFrames {
		if !f.Visible() {
			t.Errorf("pill %d was built and placed but is hidden: the reader sees "+
				"a reserved band with nothing in it", i)
		}
	}
	// And every pill is in the object list, or it is never drawn at all.
	inList := 0
	for _, o := range rend.Objects() {
		for _, f := range rend.pillFrames {
			if o == fyne.CanvasObject(f) {
				inList++
			}
		}
	}
	if inList != len(rend.pillFrames) {
		t.Errorf("%d of %d pill frames reached the object list", inList, len(rend.pillFrames))
	}
}
