package bibletext

import (
	"fmt"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// twoNotedParagraphs stores one note on the first paragraph's opening verse and
// n on the last paragraph's, all minimized, and returns the state plus the two
// band verses. Everything here is the COLLAPSED state, which is the only state
// the pills draw in.
func twoNotedParagraphs(t *testing.T, lastCount int) (*AppState, []Verse, int, int, []uint64) {
	t.Helper()
	st := psalm23State()
	verses := longEnoughForTwoParagraphs()
	st.Bible.Verses["John"] = map[int][]Verse{3: verses}
	st.CurrentBook, st.CurrentChapter = "John", 3
	paras := groupVersesIntoParagraphs(verses)
	if len(paras) < 2 {
		t.Fatalf("fixture must break into 2+ paragraphs, got %d", len(paras))
	}
	first, last := paras[0][0].Verse, paras[len(paras)-1][0].Verse

	var ids []uint64
	store := func(v int) {
		// Distinct text per note ON PURPOSE: addNote dedupes by content, so a
		// fixture that reuses one string silently stores a single note and the
		// test then proves nothing about counting.
		n, ok := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
			Book: "John", Chapter: 3, VerseLo: v,
			Text: fmt.Sprintf("fixture note %d on v%d", len(ids), v)})
		if !ok {
			t.Fatalf("could not store a note on v%d", v)
		}
		setNoteMinimizedByID(appPrefs(), n.ID, true)
		ids = append(ids, n.ID)
	}
	store(first)
	for i := 0; i < lastCount; i++ {
		store(last)
	}
	applyNoteForCurrentChapter(st)
	return st, verses, first, last, ids
}

func withPillsOn(t *testing.T) {
	t.Helper()
	prev := notesPillPerParagraph
	notesPillPerParagraph = true
	t.Cleanup(func() { notesPillPerParagraph = prev })
}

// The whole reason the pills are per paragraph: tapping ONE opens THAT
// paragraph's note. The single pill could only ever open whichever note the
// plan had chosen, so a reader with notes on two passages could not reach the
// second one from the text at all.
func TestTappingAParagraphPillOpensThatParagraphsNote(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())
	withPillsOn(t)

	st, verses, first, last, ids := twoNotedParagraphs(t, 1)
	firstID, lastID := ids[0], ids[1]

	groups := chapterNoteGroups(st, buildChapterPlan(st, appPrefs(), st.Bible), verses)
	if len(groups) != 2 {
		t.Fatalf("fixture must give two noted paragraphs, got %d", len(groups))
	}

	// Tap the LAST paragraph's pill.
	pressPillOnParagraphOf(t, st, last)
	if st.NoteID != lastID {
		t.Errorf("tapping the last paragraph's pill opened note %d, want %d — a "+
			"reader tapping one paragraph must not be shown another's note", st.NoteID, lastID)
	}
	if st.NoteMinimized {
		t.Errorf("tapping a pill must OPEN the note, not leave it collapsed")
	}

	// And back the other way, or the pills only work in one direction.
	pressPillOnParagraphOf(t, st, first)
	if st.NoteID != firstID {
		t.Errorf("tapping the first paragraph's pill opened note %d, want %d", st.NoteID, firstID)
	}
}

// A pill that opens a note must be reachable in the paragraph it names: the
// band verse the pill is drawn at is the group focusNoteAtGroup is called with,
// so the two must agree exactly.
func TestEveryPillsBandVerseOpensANote(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())
	withPillsOn(t)

	st, verses, _, _, _ := twoNotedParagraphs(t, 3)

	pane := newStyledReadingPane(st, verses)
	pane.Resize(fyne.NewSize(320, 900))
	if len(pane.pillGeoms) != 2 {
		t.Fatalf("want 2 pills, got %d", len(pane.pillGeoms))
	}
	for i, g := range pane.pillGeoms {
		st.NoteID = 0
		focusNoteAtGroup(st, g.groupKey)
		if st.NoteID == 0 {
			t.Errorf("pill %d is drawn at v%d but tapping it opens nothing — a "+
				"dead pill is worse than no pill", i, g.anchorVerse)
		}
	}
	_ = verses
}

// Counts are per paragraph and honest: one note reads "Note", four read
// "Notes · 4". A pill that reported the CHAPTER's total on each paragraph
// would be the very confusion the pills exist to remove.
func TestEachPillCountsOnlyItsOwnParagraph(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())
	withPillsOn(t)

	st, verses, _, _, _ := twoNotedParagraphs(t, 4)

	pane := newStyledReadingPane(st, verses)
	pane.Resize(fyne.NewSize(320, 900))
	if len(pane.pillGeoms) != 2 {
		t.Fatalf("want 2 pills, got %d", len(pane.pillGeoms))
	}
	got := []string{pane.pillGeoms[0].pillText, pane.pillGeoms[1].pillText}
	if got[0] != "Note" {
		t.Errorf("a paragraph with one note reads %q, want %q", got[0], "Note")
	}
	if got[1] != "Notes · 4" {
		t.Errorf("a paragraph with four notes reads %q, want %q", got[1], "Notes · 4")
	}
	for _, s := range got {
		if strings.Contains(s, "5") {
			t.Errorf("pill %q reports the chapter total; each pill must count only "+
				"its own paragraph", s)
		}
	}
}

// Two notes on the SAME verse are one paragraph's business: one pill, count 2.
func TestTwoNotesOnOneVerseShareOnePill(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())
	withPillsOn(t)

	st := psalm23State()
	verses := longEnoughForTwoParagraphs()
	st.Bible.Verses["John"] = map[int][]Verse{3: verses}
	st.CurrentBook, st.CurrentChapter = "John", 3
	paras := groupVersesIntoParagraphs(verses)
	for i := 0; i < 2; i++ {
		n, _ := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
			Book: "John", Chapter: 3, VerseLo: paras[0][0].Verse,
			Text: fmt.Sprintf("same verse %d", i)})
		setNoteMinimizedByID(appPrefs(), n.ID, true)
	}
	n, _ := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
		Book: "John", Chapter: 3, VerseLo: paras[len(paras)-1][0].Verse, Text: "elsewhere"})
	setNoteMinimizedByID(appPrefs(), n.ID, true)
	applyNoteForCurrentChapter(st)

	groups := chapterNoteGroups(st, buildChapterPlan(st, appPrefs(), st.Bible), verses)
	if len(groups) != 2 {
		t.Fatalf("two notes on one verse plus one elsewhere is two paragraphs, got %d", len(groups))
	}
	if len(groups[0].Notes) != 2 {
		t.Errorf("the shared verse's paragraph carries %d notes, want 2", len(groups[0].Notes))
	}
}

// The gate is a gate: off, nothing about the pills runs, and the chapter is
// drawn exactly as it was before the feature existed.
func TestTheGateOffDrawsNoPillsAndKeepsTheSingleSticker(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	prev := notesPillPerParagraph
	notesPillPerParagraph = false
	defer func() { notesPillPerParagraph = prev }()

	st, verses, _, _, _ := twoNotedParagraphs(t, 3)
	if g := chapterNoteGroups(st, buildChapterPlan(st, appPrefs(), st.Bible), verses); len(g) != 0 {
		t.Errorf("gate off must not group at all, got %d groups", len(g))
	}
	pane := newStyledReadingPane(st, verses)
	pane.Resize(fyne.NewSize(320, 900))
	if len(pane.pillGeoms) != 0 {
		t.Errorf("gate off drew %d pills", len(pane.pillGeoms))
	}
	if !pane.noteGeom.present {
		t.Errorf("gate off must still draw the single sticker")
	}
}

// Notes switched off is not a note state, it is an absence: no pills, whatever
// the gate says.
func TestNotesOffDrawsNoPillsEvenWithTheGateOn(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())
	withPillsOn(t)

	st, verses, _, _, _ := twoNotedParagraphs(t, 3)
	setNotesEnabled(false)
	defer setNotesEnabled(true)
	applyNoteForCurrentChapter(st)

	pane := newStyledReadingPane(st, verses)
	pane.Resize(fyne.NewSize(320, 900))
	if len(pane.pillGeoms) != 0 {
		t.Errorf("notes are off; %d pills were drawn anyway", len(pane.pillGeoms))
	}
}

// Resize is a relayout, and a relayout must not accumulate. The pills are
// rebuilt from the store every pass, so the count is a function of the notes,
// never of how many times the reader has dragged the window.
func TestResizingDoesNotAccumulatePills(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())
	withPillsOn(t)

	st, verses, _, _, _ := twoNotedParagraphs(t, 2)
	pane := newStyledReadingPane(st, verses)
	for _, w := range []float32{320, 480, 300, 700, 320} {
		pane.Resize(fyne.NewSize(w, 900))
		if len(pane.pillGeoms) != 2 {
			t.Fatalf("width %.0f: %d pills, want 2", w, len(pane.pillGeoms))
		}
	}
}

// A chapter with no notes has no pills and no reserved space — the gate must
// not cost a blank band on every chapter the reader opens.
func TestAnUnnotedChapterReservesNothing(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())
	withPillsOn(t)

	st := psalm23State()
	verses := longEnoughForTwoParagraphs()
	st.Bible.Verses["John"] = map[int][]Verse{3: verses}
	st.CurrentBook, st.CurrentChapter = "John", 3
	applyNoteForCurrentChapter(st)

	pane := newStyledReadingPane(st, verses)
	pane.Resize(fyne.NewSize(320, 900))
	if len(pane.pillGeoms) != 0 {
		t.Errorf("no notes, yet %d pills", len(pane.pillGeoms))
	}
	if len(pane.lay.Bands) != 0 {
		t.Errorf("no notes, yet %d bands reserved — blank space on every chapter",
			len(pane.lay.Bands))
	}
}

// The disclosure the single pill makes and the pills currently do not: a note
// filed on this book whose passage this translation cannot reach is counted in
// the sticker's sentence as "N not shown". It is never dropped from the plan,
// and the reader is told it exists.
//
// This test pins the SHIPPED (gate-off) guarantee. With the gate ON and two or
// more noted paragraphs, no pill mentions it — the pills are per paragraph and
// an unplaced note belongs to no paragraph, so it currently falls out of the
// reading view entirely. That gap is recorded in docs/BACKLOG.md; it is a
// design question (where does a paragraph-less note's disclosure go?) rather
// than a mechanical fix, and the gate is off by default so nothing ships blind.
func TestTheSinglePillStillDisclosesUnplacedNotes(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	prev := notesPillPerParagraph
	notesPillPerParagraph = false
	defer func() { notesPillPerParagraph = prev }()

	st := psalm23State()
	var esther []Verse
	for i, v := range longEnoughForTwoParagraphs() {
		esther = append(esther, Verse{BookName: "Esther", Chapter: 4, Verse: i + 1, Text: v.Text})
	}
	st.Bible.Verses["Esther"] = map[int][]Verse{4: esther}
	st.CurrentBook, st.CurrentChapter = "Esther", 4

	paras := groupVersesIntoParagraphs(esther)
	for i, p := range [][]Verse{paras[0], paras[len(paras)-1]} {
		addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
			Book: "Esther", Chapter: 4, VerseLo: p[0].Verse, Text: fmt.Sprintf("placed %d", i)})
	}
	// Greek Esther: webc's numbering does not correspond — the unplaced arm.
	addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "webc",
		Book: "Esther", Chapter: 4, VerseLo: 1, Text: "greek esther"})
	for _, n := range allNotesForBrowsing(appPrefs()) {
		setNoteMinimizedByID(appPrefs(), n.ID, true)
	}
	applyNoteForCurrentChapter(st)

	plan := buildChapterPlan(st, appPrefs(), st.Bible)
	if len(plan.Unplaced) != 1 {
		t.Fatalf("fixture must produce exactly one unplaced note, got %d", len(plan.Unplaced))
	}
	_, who, _, _ := styledStickerPush(st, plan)
	if !strings.Contains(who, "not shown") {
		t.Errorf("the sticker reads %q; a note this translation cannot place must "+
			"still be disclosed to the reader, or it is silently unreachable", who)
	}
}

// A pill press is the reader choosing that note as the page's reason, so a
// foreign mark stands aside — exactly as it does for the single pill
// (restoreCurrentNote) and the counts region (advanceNoteFocus). Without it the
// press re-derives into a still-suppressed plan and NOTHING happens: the pill
// stays a pill, no bubble, no wash, no feedback. restoreCurrentNote's own
// comment records this failure being found once already.
func TestTappingAPillOpensTheNoteEvenWithASearchResultLit(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())
	withPillsOn(t)

	st, _, _, last, ids := twoNotedParagraphs(t, 1)
	lastID := ids[1]

	// A search result is lit on this chapter: a foreign mark, so notes are
	// suppressed and every note reads as collapsed.
	st.setMark(hlSearch, VerseSpan{VersionID: "web", Book: "John", Chapter: 3, Lo: 1, Hi: 1})
	// The modelled route (a tapped search result) is explicit; plain entries
	// arrive nowhere now.
	st.forceReposition = true
	applyNoteForCurrentChapter(st)
	if !notesSuppressed(st) {
		t.Fatalf("fixture must actually suppress: the mark is not foreign")
	}

	pressPillOnParagraphOf(t, st, last)

	if st.NoteMinimized {
		t.Errorf("the pill press did nothing visible while a search result was lit — "+
			"the note stayed collapsed (NoteID=%d)", st.NoteID)
	}
	if st.NoteID != lastID {
		t.Errorf("pressed pill opened note %d, want %d", st.NoteID, lastID)
	}
	if st.mark.live() && !st.mark.fromNote() {
		t.Errorf("the foreign mark survived the press; the suppression it causes " +
			"never releases and the note can never open")
	}
}

// A note can reach this chapter through Placement.Elsewhere rather than
// Placement.Here — the Romans doxology is the shipping case: filed on WEB
// Romans 14:24-26, it lives on 16:25-27 in BSB with Here EMPTY. The plan admits
// it (placementRunOn checks BOTH lists), so the single pill counts it. A pill
// model that reads only Here drops it: no pill, and absent from every count.
func TestANoteReachingThisChapterViaElsewhereStillGetsAPill(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())
	withPillsOn(t)

	st := planTestState(t)
	st.CurrentVersion = "bsb"
	// A real-length Romans 16: the doxology maps onto vv25-27, so a chapter
	// that stops short would drop the note for want of a verse rather than for
	// the reason under test.
	long := "This verse is deliberately long so that the paragraph splitter reaches its " +
		"character threshold and breaks at the next sentence ending, which is here."
	var rom []Verse
	for i := 1; i <= 27; i++ {
		rom = append(rom, Verse{BookName: "Romans", Book: "Romans", Chapter: 16,
			Verse: i, Text: long})
	}
	st.Bible.Verses["Romans"] = map[int][]Verse{16: rom}
	st.CurrentBook, st.CurrentChapter = "Romans", 16

	addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
		Book: "Romans", Chapter: 14, VerseLo: 24, VerseHi: 26, Text: "doxology"})
	paras := groupVersesIntoParagraphs(rom)
	addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "bsb",
		Book: "Romans", Chapter: 16, VerseLo: paras[0][0].Verse, Text: "plain"})
	for _, n := range allNotesForBrowsing(appPrefs()) {
		setNoteMinimizedByID(appPrefs(), n.ID, true)
	}
	applyNoteForCurrentChapter(st)

	plan := buildChapterPlan(st, appPrefs(), st.Bible)
	if len(plan.Notes) != 2 {
		t.Fatalf("fixture must put both notes on this chapter, got %d", len(plan.Notes))
	}
	groups := groupNotesByParagraph(paras, plan.Notes)
	counted := 0
	for _, g := range groups {
		counted += len(g.Notes)
	}
	if counted != len(plan.Notes) {
		t.Errorf("the pills account for %d of the chapter's %d notes; a note the "+
			"single pill counts must not vanish from the per-paragraph model",
			counted, len(plan.Notes))
	}
	if len(groups) != 2 {
		t.Errorf("two notes in two different paragraphs is two groups, got %d", len(groups))
	}
}

// The chapter-scope parking (NoteVerseLo = 0, which opens the band at the top)
// is SHARED state, read by every surface. Only the styled pane draws pills;
// iOS, macOS and Android still draw the single sticker, and for them a
// collapsed set spanning several paragraphs is a chapter-wide fact that belongs
// at the top whatever the pill gate says. Gating the parking on the pill flag
// moved their sticker back onto an arbitrary paragraph.
//
// It costs the styled pane nothing: with pills in force its single sticker
// stands down and reserves no band, so the parked anchor is simply unused.
func TestChapterScopeParkingSurvivesThePillGate(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	for _, gateOn := range []bool{false, true} {
		deleteAllNotes(appPrefs())
		prev := notesPillPerParagraph
		notesPillPerParagraph = gateOn

		st, verses, _, _, _ := twoNotedParagraphs(t, 2)
		if st.NoteVerseLo != 0 {
			t.Errorf("gate=%v: collapsed notes span two paragraphs, so the single "+
				"sticker must park at chapter scope; NoteVerseLo=%d",
				gateOn, st.NoteVerseLo)
		}
		// And with the gate on the pane still draws its pills, unaffected.
		if gateOn {
			pane := newStyledReadingPane(st, verses)
			pane.Resize(fyne.NewSize(320, 900))
			if len(pane.pillGeoms) != 2 {
				t.Errorf("parking must not cost the pane its pills: got %d, want 2",
					len(pane.pillGeoms))
			}
		}
		notesPillPerParagraph = prev
	}
}

// Scrolling to a highlight inside a banded paragraph must land on the BAND,
// not on the verse's line — otherwise the pill that explains why the reader is
// here sits exactly above the fold. The single sticker has had this rule since
// the bubble could be clipped on arrival; its guard reads noteGeom.present and
// lay.BandLine, and BOTH are false by construction once the per-paragraph pills
// are drawn (relayout zeroes noteGeom, and the multi-band path leaves BandLine
// at -1), so the pill path inherited none of it.
func TestScrollingToAHighlightLandsOnItsParagraphsPill(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())
	withPillsOn(t)

	st, verses, _, last, _ := twoNotedParagraphs(t, 2)
	// A search result lit on a verse inside the SECOND noted paragraph.
	st.setMark(hlSearch, VerseSpan{VersionID: "web", Book: "John", Chapter: 3, Lo: last, Hi: last})
	// The modelled route (a tapped search result) is explicit; plain entries
	// arrive nowhere now.
	st.forceReposition = true
	applyNoteForCurrentChapter(st)

	pane := newStyledReadingPane(st, verses)
	pane.Resize(fyne.NewSize(320, 900))
	if len(pane.pillGeoms) != 2 {
		t.Fatalf("fixture must draw two pills, got %d", len(pane.pillGeoms))
	}
	li := pane.highlightFirstLine()
	if li < 0 {
		t.Fatalf("fixture must light a highlight")
	}

	var want float32 = -1
	for _, b := range pane.lay.Bands {
		if li >= b.Line && li <= b.LastLine {
			want = b.Y
		}
	}
	if want < 0 {
		t.Fatalf("the highlighted line %d is in no banded paragraph; fixture is wrong", li)
	}
	if got := pane.highlightY(); got != want {
		t.Errorf("highlightY() = %.1f, want the band top %.1f — the reader is scrolled "+
			"%.1fpt past the pill that explains why they are here", got, want, got-want)
	}
}

// The pill label centres on BOTH axes, exactly as the single pill's does.
// Vertically that means the TEXT'S INTRINSIC HEIGHT, not its point size: a
// canvas.Text draws from its top-left at MinSize().Height, which is taller than
// TextSize by the ascender and descender, so centring on TextSize pushes the
// word down by half the difference and the label sits low in its frame.
func TestThePillLabelIsCentredInItsFrame(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())
	withPillsOn(t)

	st, verses, _, _, _ := twoNotedParagraphs(t, 3)
	pane := newStyledReadingPane(st, verses)
	rend, ok := pane.CreateRenderer().(*styledPaneRenderer)
	if !ok {
		t.Fatalf("unexpected renderer type")
	}
	rend.Layout(fyne.NewSize(320, 900))
	if len(rend.pillLabels) == 0 {
		t.Fatalf("no pill labels were built")
	}

	for i, lbl := range rend.pillLabels {
		card := pane.pillGeoms[i].card
		h := lbl.MinSize().Height
		wantY := card.Y + (card.H-h)/2
		if got := lbl.Position().Y; got != wantY {
			t.Errorf("pill %d label at y=%.2f, want %.2f (off by %.2fpt): the label "+
				"must centre on the text's intrinsic height %.2f, not its point size %.2f",
				i, got, wantY, got-wantY, h, lbl.TextSize)
		}
		if got := lbl.Size().Height; got != h {
			t.Errorf("pill %d label sized to %.2f, want its intrinsic %.2f", i, got, h)
		}
	}
}

// The pills take every colour from the palette, so they follow the theme the
// same way the single sticker does. Pinned in BOTH themes: a pill that read a
// literal, or reused a light-only colour, would be invisible or garish on the
// other ground — and the pills are drawn on the reading page, where a wrong
// ink is at its most obvious.
func TestThePillsTakeTheirColoursFromTheActiveTheme(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())
	withPillsOn(t)

	for _, tc := range []struct {
		name string
		pal  palette
	}{{"light", lightPalette}, {"dark", darkPalette}} {
		t.Run(tc.name, func(t *testing.T) {
			deleteAllNotes(appPrefs())
			st, verses, _, _, _ := twoNotedParagraphs(t, 2)
			pane := newStyledReadingPane(st, verses)
			pane.pal = tc.pal
			rend, ok := pane.CreateRenderer().(*styledPaneRenderer)
			if !ok {
				t.Fatalf("unexpected renderer type")
			}
			rend.Layout(fyne.NewSize(320, 900))
			if len(rend.pillFrames) == 0 {
				t.Fatalf("no pills built")
			}
			for i, f := range rend.pillFrames {
				if f.FillColor != tc.pal.SurfaceAlt {
					t.Errorf("pill %d fill is %v, want the theme's SurfaceAlt %v",
						i, f.FillColor, tc.pal.SurfaceAlt)
				}
				if f.StrokeColor != tc.pal.Border {
					t.Errorf("pill %d stroke is %v, want the theme's Border %v",
						i, f.StrokeColor, tc.pal.Border)
				}
			}
			for i, l := range rend.pillLabels {
				if l.Color != tc.pal.TextMuted {
					t.Errorf("pill %d label ink is %v, want the theme's TextMuted %v",
						i, l.Color, tc.pal.TextMuted)
				}
			}
		})
	}
}

// seedOwnNote writes one of the reader's own notes through the app's own path.
func seedOwnNote(t *testing.T, verse int, text string) StoredNote {
	t.Helper()
	nonce := make([]byte, noteNonceLen)
	for i := range nonce {
		nonce[i] = byte(verse*17 + i)
	}
	n, ok := saveMyNote(appPrefs(), StoredNote{VersionID: "web", Book: "John", Chapter: 3,
		VerseLo: verse, Text: text, Nonce: nonce})
	if !ok {
		t.Fatalf("could not store an own note on v%d", verse)
	}
	return n
}

// The received set must be represented on the page EXACTLY ONCE. Opening one of
// the reader's own notes used to represent it zero times: the sticker showed the
// own note, and the pills stood down because a sticker existed — so every trace
// of the friends' notes disappeared, with nothing on the page saying to close
// your own note to get them back.
//
// An own note is not a member of that set and carries no count of it, so the
// pills stay up beside it and duplicate nothing.
func TestTheFriendsPillsSurviveOpeningYourOwnNote(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())
	withPillsOn(t)

	st, verses, first, _, _ := twoNotedParagraphs(t, 2)
	mine := seedOwnNote(t, first+1, "mine, beside a friend's paragraph")
	st.focusNote(mine.ID)
	applyNoteForCurrentChapter(st)

	pane := newStyledReadingPane(st, verses)
	pane.Resize(fyne.NewSize(320, 900))

	if len(pane.pillGeoms) != 2 {
		t.Errorf("two noted paragraphs of friends' notes, so two pills must stay "+
			"up beside the reader's own note; got %d", len(pane.pillGeoms))
	}
	if !pane.noteGeom.present {
		t.Errorf("the reader's own note must still be drawn: standing the sticker " +
			"down for the pills is what made it vanish")
	}
}

// The single-noted-paragraph shortcut does not apply while an own note is open.
// Normally one noted paragraph needs no pill because the sticker already IS
// that paragraph's collapsed form — but a sticker showing the reader's own note
// is not available to say anything about the friends' one.
func TestASingleNotedParagraphStillGetsAPillBesideYourOwnNote(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())
	withPillsOn(t)

	st := psalm23State()
	verses := longEnoughForTwoParagraphs()
	st.Bible.Verses["John"] = map[int][]Verse{3: verses}
	st.CurrentBook, st.CurrentChapter = "John", 3
	paras := groupVersesIntoParagraphs(verses)
	first := paras[0][0].Verse

	n, _ := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
		Book: "John", Chapter: 3, VerseLo: first, Text: "the only friend's note"})
	setNoteMinimizedByID(appPrefs(), n.ID, true)

	// Nothing open: one group, so the sticker speaks for it and no pill is due.
	applyNoteForCurrentChapter(st)
	bare := newStyledReadingPane(st, verses)
	bare.Resize(fyne.NewSize(320, 900))
	if len(bare.pillGeoms) != 0 {
		t.Errorf("one noted paragraph and nothing open: the sticker speaks for it, "+
			"so no pill is due; got %d", len(bare.pillGeoms))
	}

	// Own note open: the sticker is busy, so the one paragraph needs its pill.
	mine := seedOwnNote(t, paras[len(paras)-1][0].Verse, "mine, elsewhere")
	st.focusNote(mine.ID)
	applyNoteForCurrentChapter(st)
	withMine := newStyledReadingPane(st, verses)
	withMine.Resize(fyne.NewSize(320, 900))
	if len(withMine.pillGeoms) != 1 {
		t.Errorf("the sticker is showing the reader's own note, so the friends' "+
			"one noted paragraph needs its own pill; got %d", len(withMine.pillGeoms))
	}
}

// An open RECEIVED note is the one case where the pills must NOT stand up: its
// who line already reads "K of N in this chapter", so pills would say the same
// thing twice — which is the duplication the stand-down rule exists to prevent.
func TestPillsStandDownForAnOpenReceivedNote(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())
	withPillsOn(t)

	st, verses, first, _, ids := twoNotedParagraphs(t, 2)
	_ = first
	setNoteMinimizedByID(appPrefs(), ids[0], false)
	st.focusNote(ids[0])
	applyNoteForCurrentChapter(st)

	pane := newStyledReadingPane(st, verses)
	pane.Resize(fyne.NewSize(320, 900))
	if len(pane.pillGeoms) != 0 {
		t.Errorf("an open received note carries the count in its who line; %d pills "+
			"say the same set a second time", len(pane.pillGeoms))
	}
	if !pane.noteGeom.present {
		t.Errorf("the open received note must be drawn")
	}
}

// With no friends' notes at all there is nothing for the pills to say, whatever
// the reader has open of their own.
func TestNoFriendsNotesMeansNoPillsBesideYourOwn(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())
	withPillsOn(t)

	st := psalm23State()
	verses := longEnoughForTwoParagraphs()
	st.Bible.Verses["John"] = map[int][]Verse{3: verses}
	st.CurrentBook, st.CurrentChapter = "John", 3
	mine := seedOwnNote(t, groupVersesIntoParagraphs(verses)[0][0].Verse, "only mine")
	st.focusNote(mine.ID)
	applyNoteForCurrentChapter(st)

	pane := newStyledReadingPane(st, verses)
	pane.Resize(fyne.NewSize(320, 900))
	if len(pane.pillGeoms) != 0 {
		t.Errorf("no friends' notes on the chapter, yet %d pills", len(pane.pillGeoms))
	}
}

// When both land in ONE paragraph the layout must reserve BOTH bands, and in
// reading order: the pill above, the sticker below it and nearest the text,
// because the sticker's tail points at the passage and a chip between the two
// would break that line of sight.
func TestAPillAndYourOwnNoteShareAParagraphInOrder(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())
	withPillsOn(t)

	st, verses, first, _, _ := twoNotedParagraphs(t, 2)
	// Same paragraph as the first friend's note.
	mine := seedOwnNote(t, first+1, "mine, in the friend's own paragraph")
	st.focusNote(mine.ID)
	applyNoteForCurrentChapter(st)

	pane := newStyledReadingPane(st, verses)
	pane.Resize(fyne.NewSize(320, 900))
	if !pane.noteGeom.present || len(pane.pillGeoms) == 0 {
		t.Fatalf("fixture must draw both: sticker=%v pills=%d",
			pane.noteGeom.present, len(pane.pillGeoms))
	}
	if pane.lay.BandLine < 0 {
		t.Fatalf("the sticker's own band was never reserved")
	}
	// The pill for the shared paragraph sits above the sticker.
	var shared *styledNoteGeom
	for i := range pane.pillGeoms {
		if pane.pillGeoms[i].card.Y < pane.noteGeom.card.Y {
			shared = &pane.pillGeoms[i]
		}
	}
	if shared == nil {
		t.Errorf("no pill sits above the sticker; the two bands are not in reading order")
	}
	// And they must not overlap: the bands are ADVANCE, disjoint by construction.
	for i, g := range pane.pillGeoms {
		if g.card.Y < pane.noteGeom.card.Y+pane.noteGeom.card.H &&
			pane.noteGeom.card.Y < g.card.Y+g.card.H {
			t.Errorf("pill %d (y=%.1f h=%.1f) overlaps the sticker (y=%.1f h=%.1f)",
				i, g.card.Y, g.card.H, pane.noteGeom.card.Y, pane.noteGeom.card.H)
		}
	}
}

// styledNote.Pill means "the sticker is CLOSED", which is not the same question
// as "the sticker IS the received set's collapsed form". A focused own note is
// closed too whenever a foreign mark suppresses the chapter (notes_plan.go,
// appleStickerPush's own-note arm returns pill=true under notesSuppressed), so
// a rule keyed on Pill alone treats the reader's own note as the friends' chip:
// the pills replace it, the pane zeroes its geometry, and the own note is drawn
// nowhere — the very failure the pills were made to prevent, reachable by
// arriving at the chapter through a search result first.
func TestYourOwnNoteSurvivesPillsWhileASearchResultIsLit(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())
	withPillsOn(t)

	st, verses, first, _, _ := twoNotedParagraphs(t, 2)
	mine := seedOwnNote(t, first+1, "mine, opened after a search")
	// The foreign mark first, exactly as arriving through Results does.
	goToVerseRange(st, "John", 3, 1, 1)
	st.focusNote(mine.ID)
	applyNoteForCurrentChapter(st)

	if !notesSuppressed(st) {
		t.Fatalf("fixture must suppress: the mark is not foreign")
	}
	note := styledNoteFor(st)
	if !note.Pill || !note.Own {
		t.Fatalf("fixture must produce the overlapping state, got Pill=%v Own=%v",
			note.Pill, note.Own)
	}

	pane := newStyledReadingPane(st, verses)
	pane.Resize(fyne.NewSize(320, 900))
	if !pane.noteGeom.present {
		t.Errorf("the reader's own note was drawn nowhere: a closed OWN note is not " +
			"the received set's collapsed form, so the pills must not replace it")
	}
}

// And the same confusion the other way, on every surface with no pill row: a
// suppressed own note reported the set as represented by the sticker, when the
// sticker was showing "Note from you" and carried no count of the set at all.
// N9 read that as healthy and X16 undercounted its own extent.
func TestASuppressedOwnNoteDoesNotPassAsTheReceivedSetsChip(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	prev := notesPillPerParagraph
	notesPillPerParagraph = false // every shipped build, and all three native surfaces
	defer func() { notesPillPerParagraph = prev }()

	st, verses, first, _, _ := twoNotedParagraphs(t, 2)
	mine := seedOwnNote(t, first+1, "mine")
	goToVerseRange(st, "John", 3, 1, 1)
	st.focusNote(mine.ID)
	applyNoteForCurrentChapter(st)

	plan := buildChapterPlan(st, appPrefs(), st.Bible)
	note := styledNoteFor(st)
	groups := len(chapterNoteGroups(st, plan, verses))
	if got := receivedSetShownAs(plan, note, groups); got != shownAsNothing {
		t.Errorf("the sticker is showing %q and counts none of the %d received "+
			"notes, so the set is represented nowhere; receivedSetShownAs says %v",
			note.Who, len(plan.Notes), got)
	}
}

// Scrolling to a highlight must land on the TOPMOST thing reserved above that
// paragraph, not merely on the sticker's band. Once the two reservations became
// independent a paragraph can carry a pill AND the sticker, pill above — and
// highlightY's first branch returns the sticker's band unconditionally, which
// puts the pill it sits under above the fold. Same failure as scrolling past a
// pill entirely, reintroduced for the shared-paragraph case.
func TestScrollingLandsOnTheTopmostBandOfTheParagraph(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())
	withPillsOn(t)

	st, verses, first, _, _ := twoNotedParagraphs(t, 2)
	mine := seedOwnNote(t, first+1, "mine, in the friend's paragraph")
	st.focusNote(mine.ID)
	applyNoteForCurrentChapter(st)
	// Opening a note from the browser is explicit (openedNotePlacesTheView);
	// a plain entry arrives nowhere now.
	st.forceReposition = true

	pane := newStyledReadingPane(st, verses)
	pane.Resize(fyne.NewSize(320, 900))
	if !pane.noteGeom.present || pane.lay.BandLine < 0 || len(pane.lay.Bands) == 0 {
		t.Fatalf("fixture must reserve both: sticker=%v bandLine=%d bands=%d",
			pane.noteGeom.present, pane.lay.BandLine, len(pane.lay.Bands))
	}
	li := pane.highlightFirstLine()
	if li < 0 {
		t.Skip("no highlight lit in this fixture")
	}

	// The topmost band whose paragraph carries the highlighted line.
	top := float32(-1)
	if li >= pane.lay.BandLine && li <= pane.lastLineOfBandParagraph() {
		top = pane.lay.BandY
	}
	for _, b := range pane.lay.Bands {
		if li >= b.Line && li <= b.LastLine && (top < 0 || b.Y < top) {
			top = b.Y
		}
	}
	if top < 0 {
		t.Skip("the highlighted line is in no banded paragraph")
	}
	if got := pane.highlightY(); got != top {
		t.Errorf("highlightY() = %.1f, want the topmost band %.1f — the reader is "+
			"scrolled %.1fpt past what sits above the paragraph", got, top, got-top)
	}
}

// A note that points at NO passage gets no speech tail. The tail asserts "this
// is about the text directly below me", and a chapter-scope note is parked at
// the top of the chapter — so a tail there claims verse 1, which is not what the
// note is about. It also reserves no tail depth, so the card is exactly as tall
// as its content.
func TestAChapterScopeNoteHasNoSpeechTail(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	const w = 300
	anchored := measureStyledNote(styledNote{Text: "a note", Who: "Note from Friend", Anchor: 7}, w)
	chapterWide := measureStyledNote(styledNote{Text: "a note", Who: "Note from Friend", Anchor: 0}, w)

	if !anchored.hasTail {
		t.Errorf("a note anchored at a verse must keep its tail")
	}
	if chapterWide.hasTail {
		t.Errorf("a chapter-scope note must have no tail: it points at no passage")
	}
	if got := anchored.card.H - anchored.cardH; got != noteTailDepth {
		t.Errorf("anchored card reserves %.1f of tail depth, want %d", got, noteTailDepth)
	}
	if got := chapterWide.card.H - chapterWide.cardH; got != 0 {
		t.Errorf("chapter-scope card reserves %.1f of tail depth, want 0 — the space "+
			"is only there to hold a tail", got)
	}
}

// And the drawn outline actually loses the detour, while staying ONE path — the
// property notes_bubble.go records as the reason the outline is a single path at
// all (a separate rect and triangle leave the card's bottom border running
// across the tail's mouth).
func TestTheTaillessBubbleIsStillOnePathAndFlatBottomed(t *testing.T) {
	withTail := string(noteBubblePathSVG(240, 80, lightPalette.SurfaceAlt, lightPalette.Border, true).Content())
	noTail := string(noteBubblePathSVG(240, 80, lightPalette.SurfaceAlt, lightPalette.Border, false).Content())

	for name, svg := range map[string]string{"with tail": withTail, "no tail": noTail} {
		if n := strings.Count(svg, "<path"); n != 1 {
			t.Errorf("%s: %d paths, want exactly one", name, n)
		}
	}
	if !strings.Contains(withTail, `height="89"`) {
		t.Errorf("the tailed bubble should be 80+%d tall; got:\n%s", noteTailDepth, withTail[:120])
	}
	if !strings.Contains(noTail, `height="80"`) {
		t.Errorf("the tailless bubble must be exactly its card height; got:\n%s", noTail[:120])
	}
	// The apex is what the detour exists for, so the tailless path must be shorter.
	if len(noTail) >= len(withTail) {
		t.Errorf("the tailless outline is not shorter than the tailed one, so the "+
			"detour is probably still in it (%d vs %d bytes)", len(noTail), len(withTail))
	}
}

// The pills now account for EVERY note on the chapter. A chapter-level note
// belongs to no paragraph, so it used to be dropped and the pills' counts came
// out short of the total — the gap recorded in docs/BACKLOG.md. It rides a
// chapter-top group instead, drawn above everything, which is the scope it
// actually has.
func TestTheTopGroupCarriesTheNotesThatBelongToNoParagraph(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())
	withPillsOn(t)

	st := psalm23State()
	verses := longEnoughForTwoParagraphs()
	st.Bible.Verses["John"] = map[int][]Verse{3: verses}
	st.CurrentBook, st.CurrentChapter = "John", 3
	paras := groupVersesIntoParagraphs(verses)

	// Two on the last paragraph, one on the FIRST — which is the collision the
	// keying exists for: the chapter-top group is found by the first verse, and
	// so is paragraph 0's own group, so the two share a band verse and can only
	// be told apart by key.
	for i := 0; i < 2; i++ {
		n, _ := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
			Book: "John", Chapter: 3, VerseLo: paras[len(paras)-1][0].Verse,
			Text: fmt.Sprintf("on a paragraph %d", i)})
		setNoteMinimizedByID(appPrefs(), n.ID, true)
	}
	first, _ := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
		Book: "John", Chapter: 3, VerseLo: paras[0][0].Verse, Text: "on the first paragraph"})
	setNoteMinimizedByID(appPrefs(), first.ID, true)
	ch, _ := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
		Book: "John", Chapter: 3, VerseLo: 0, Text: "on the whole chapter"})
	setNoteMinimizedByID(appPrefs(), ch.ID, true)
	applyNoteForCurrentChapter(st)

	plan := buildChapterPlan(st, appPrefs(), st.Bible)
	groups := chapterNoteGroups(st, plan, verses)

	counted := 0
	top := -1
	for i, g := range groups {
		counted += len(g.Notes)
		if g.ParaIndex == chapterTopGroup {
			top = i
		}
	}
	if counted != len(plan.Notes) {
		t.Errorf("the pills account for %d of the chapter's %d notes; every note "+
			"must be in exactly one group", counted, len(plan.Notes))
	}
	if top < 0 {
		t.Fatalf("no chapter-top group, so the whole-chapter note is nowhere")
	}
	if top != 0 {
		t.Errorf("the chapter-top group is at position %d, want first — it is drawn "+
			"above everything", top)
	}
	if len(groups[top].Notes) != 1 {
		t.Errorf("the top group holds %d notes, want the one whole-chapter note",
			len(groups[top].Notes))
	}

	// And it draws: a pill at the top, with no tail, above the paragraph pill.
	pane := newStyledReadingPane(st, verses)
	pane.Resize(fyne.NewSize(320, 900))
	if len(pane.pillGeoms) != 3 {
		t.Fatalf("want a top pill plus one per noted paragraph, got %d", len(pane.pillGeoms))
	}
	// Each pill must be a DIFFERENT group. Distinct positions are not enough:
	// matching on the shared verse picks the same geometry for both bands and
	// places that one geometry twice, which reads as two pills at two heights
	// carrying one group's label while the other group is simply gone.
	seenKey := map[int]bool{}
	for i, g := range pane.pillGeoms {
		if seenKey[g.groupKey] {
			t.Errorf("pill %d is group %d again: two bands resolved to one group, "+
				"so a group is drawn twice and another not at all", i, g.groupKey)
		}
		seenKey[g.groupKey] = true
	}
	// And the labels are the groups' own, in reading order: the chapter-wide one
	// first, then each paragraph's count.
	want := []string{"Note", "Note", "Notes · 2"}
	for i, g := range pane.pillGeoms {
		if i < len(want) && g.pillText != want[i] {
			t.Errorf("pill %d reads %q, want %q", i, g.pillText, want[i])
		}
	}
	// No tail assertion here: a pill never carries one at any anchor (its
	// measure path returns before the tail is considered), so asserting it would
	// pass whatever the anchor said. The tail rule is about the BUBBLE and is
	// pinned by TestAChapterScopeNoteHasNoSpeechTail.
	if pane.pillGeoms[0].card.Y >= pane.pillGeoms[1].card.Y {
		t.Errorf("the top pill sits at y=%.1f, below the paragraph pill at %.1f",
			pane.pillGeoms[0].card.Y, pane.pillGeoms[1].card.Y)
	}
}

// An unplaced note has no home in this translation at all, so it belongs to no
// paragraph either. It rides the same top group and is disclosed in that pill's
// label using the app's own shipped phrasing — which is what the single sticker
// always did and the pills previously dropped.
func TestTheTopPillDisclosesUnplacedNotes(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())
	withPillsOn(t)

	st := psalm23State()
	var esther []Verse
	for i, v := range longEnoughForTwoParagraphs() {
		esther = append(esther, Verse{BookName: "Esther", Chapter: 4, Verse: i + 1, Text: v.Text})
	}
	st.Bible.Verses["Esther"] = map[int][]Verse{4: esther}
	st.CurrentBook, st.CurrentChapter = "Esther", 4
	paras := groupVersesIntoParagraphs(esther)

	for i, p := range [][]Verse{paras[0], paras[len(paras)-1]} {
		n, _ := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
			Book: "Esther", Chapter: 4, VerseLo: p[0].Verse, Text: fmt.Sprintf("placed %d", i)})
		setNoteMinimizedByID(appPrefs(), n.ID, true)
	}
	// Greek Esther: webc's numbering does not correspond — the unplaced arm.
	n, _ := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "webc",
		Book: "Esther", Chapter: 4, VerseLo: 1, Text: "greek esther"})
	setNoteMinimizedByID(appPrefs(), n.ID, true)
	applyNoteForCurrentChapter(st)

	plan := buildChapterPlan(st, appPrefs(), st.Bible)
	if len(plan.Unplaced) != 1 {
		t.Fatalf("fixture must produce one unplaced note, got %d", len(plan.Unplaced))
	}
	groups := chapterNoteGroups(st, plan, esther)
	if len(groups) == 0 || groups[0].ParaIndex != chapterTopGroup {
		t.Fatalf("the unplaced note needs a chapter-top group; groups=%d", len(groups))
	}
	if groups[0].Unplaced != 1 {
		t.Errorf("the top group reports %d unplaced, want 1", groups[0].Unplaced)
	}

	pane := newStyledReadingPane(st, esther)
	pane.Resize(fyne.NewSize(320, 900))
	if len(pane.pillGeoms) == 0 {
		t.Fatalf("no pills drawn")
	}
	if !strings.Contains(pane.pillGeoms[0].pillText, "not shown") {
		t.Errorf("the top pill reads %q; a note this translation cannot place must "+
			"still be disclosed", pane.pillGeoms[0].pillText)
	}

	// And the ANCHORLESS CARD must not point anywhere. Measured directly rather
	// than through the pane, and the reason is worth stating: no live push
	// reaches this state yet. A pill is tail-free already (measureStyledNote
	// returns before the tail term), and an EXPANDED card always has an anchor,
	// because a note with no passage here has nothing to open and stands down to
	// the pill. So this is the geometry unit under test, held to the rule ahead
	// of the state that will exercise it — the natives get anchorless expanded
	// cards when per-paragraph placement reaches them.
	top := measureStyledNote(styledNote{Who: "x", Text: "body", Anchor: 0}, 320)
	pointed := measureStyledNote(styledNote{Who: "x", Text: "body", Anchor: 1}, 320)
	if top.hasTail {
		t.Error("the chapter-top card claims a tail; it points at no passage")
	}
	if !pointed.hasTail {
		t.Fatal("the anchored control has no tail either — this comparison is " +
			"measuring nothing")
	}
	if got, want := pointed.bandH()-top.bandH(), float32(noteTailDepth); got != want {
		t.Errorf("the anchorless band is %g shorter than the anchored one, want %g "+
			"(the tail's whole depth); a gate that changes the DRAWING without "+
			"changing the RESERVATION leaves the same phantom gap behind", got, want)
	}
}

// Own-ness is ONE question, and every surface must get the same answer — above
// all the surface DRAWING the glyph, because the glyph is a promise about what
// the press will do.
//
// The three native panes push isOwnLiveNote (notes_store.go), which asks the
// store by NoteID; dropCurrentNote and hideCurrentNote branch on the same
// predicate. The styled pane asked the PLAN instead — HasOwn and the slot's id —
// and the two disagree wherever the mirror still names an own note the plan is
// no longer offering. There the styled pane drew a bin, which says destroy, on
// a note the press would only dismiss.
func TestOwnNessComesFromTheSamePredicateAsTheVerbs(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	st := planTestState(t)
	nonce := make([]byte, noteNonceLen)
	nonce[0] = 5
	mine, ok := saveMyNote(appPrefs(), StoredNote{VersionID: "web", Book: "John",
		Chapter: 3, VerseLo: 16, Text: "mine on this chapter", Nonce: nonce})
	if !ok {
		t.Fatal("could not store an own note")
	}
	st.focusNote(mine.ID)
	applyNoteForCurrentChapter(st)

	// The state where the two used to part company: the mirror still names the
	// note, the plan no longer offers it.
	st.resetNoteFocus()
	if !isOwnLiveNote(st) {
		t.Fatalf("fixture broken: the store no longer calls the live note the reader's own")
	}
	if plan := buildChapterPlan(st, appPrefs(), st.Bible); plan.HasOwn {
		t.Fatalf("fixture broken: the plan still offers the own slot, so the two cannot part")
	}

	if got := styledNoteFor(st); got.present() && !got.Own {
		t.Errorf("the styled sticker says the live note is not the reader's own while " +
			"the verbs say it is: it would draw a bin — destroy — on a note that " +
			"dropCurrentNote only dismisses")
	}
}

// The centering rule (notePillSeparatorLift): on the narrow layout a collapsed
// stack whose bottom neighbour is the passage rises half the paragraph
// separator above its band top, so the air reads the same on both sides. The
// chapter-top band (Line 0) has no separator above it and stays put, and the
// reporter layout's paraGap is 0, so nothing moves there — the same absence of
// a width branch the rule promises in notes_bubble.go.
func TestNarrowPillsCentreAcrossTheParagraphSeparator(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())
	withPillsOn(t)

	if notePillSeparatorLift(0) != 0 {
		t.Fatal("a zero separator must mean a zero lift — the reporter layouts ride on this")
	}
	if got := notePillSeparatorLift(24); got != 12 {
		t.Fatalf("the lift must be HALF the separator, got %v for 24", got)
	}

	st, verses, _, _, _ := twoNotedParagraphs(t, 1)

	check := func(t *testing.T, width float32, wantLifted bool) {
		pane := newStyledReadingPane(st, verses)
		pane.Resize(fyne.NewSize(width, 900))
		if len(pane.pillGeoms) != 2 {
			t.Fatalf("want 2 pills, got %d", len(pane.pillGeoms))
		}
		paraGap := pane.styledLineHeight() * 0.65
		if !wantLifted {
			paraGap = 0
		}
		for _, g := range pane.pillGeoms {
			var band *noteBand
			for i := range pane.lay.Bands {
				if pane.lay.Bands[i].Key == g.groupKey {
					band = &pane.lay.Bands[i]
					break
				}
			}
			if band == nil {
				t.Fatalf("pill for group %d has no band", g.groupKey)
			}
			want := band.Y + styledNoteGapAbv
			if band.Line > 0 {
				want -= notePillSeparatorLift(paraGap)
			}
			if got := g.card.Y; got < want-0.6 || got > want+0.6 {
				t.Errorf("pill on band line %d sits at %.1f, want %.1f "+
					"(band top %.1f, lift %.1f)", band.Line, got, want,
					band.Y, notePillSeparatorLift(paraGap))
			}
		}
	}

	t.Run("narrow lifts, chapter top does not", func(t *testing.T) {
		check(t, 320, true)
	})
	t.Run("reporter is untouched", func(t *testing.T) {
		check(t, 900, false)
	})
}

// The single collapsed pill (no per-paragraph specs) takes the same lift
// through the noteGeom path — and the OPEN card at the same width does not:
// its tail's distance to the passage is the pinned invariant, and centering
// the card would float the tail away from the words it points at.
func TestNarrowSinglePillCentresAndTheCardDoesNot(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	for _, tc := range []struct {
		name string
		pill bool
	}{{"pill lifts", true}, {"card stays", false}} {
		t.Run(tc.name, func(t *testing.T) {
			setNotesEnabled(true)
			deleteAllNotes(appPrefs())
			defer deleteAllNotes(appPrefs())

			st := psalm23State()
			verses := longEnoughForTwoParagraphs()
			st.Bible.Verses["John"] = map[int][]Verse{3: verses}
			st.CurrentBook, st.CurrentChapter = "John", 3
			paras := groupVersesIntoParagraphs(verses)
			last := paras[len(paras)-1][0].Verse
			if _, ok := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
				Book: "John", Chapter: 3, VerseLo: last,
				Text: "A note measured for the centering rule."}); !ok {
				t.Fatal("could not store the fixture note")
			}
			applyNoteForCurrentChapter(st)
			if tc.pill {
				hideCurrentNote(st)
			}
			p := newStyledReadingPane(st, verses)
			w := test.NewWindow(p)
			defer w.Close()
			w.Resize(fyne.NewSize(300, 900))
			p.Refresh()

			g := p.noteGeom
			if !g.present {
				t.Fatal("precondition: a sticker")
			}
			if g.pill != tc.pill {
				t.Fatalf("precondition: pill=%v, got %v", tc.pill, g.pill)
			}
			if p.lay.BandLine <= 0 {
				t.Fatalf("precondition: a mid-chapter band (line %d) — line 0 "+
					"would not exercise the separator", p.lay.BandLine)
			}
			want := p.lay.BandY + styledNoteGapAbv
			if tc.pill {
				want -= notePillSeparatorLift(p.styledLineHeight() * 0.65)
			}
			if got := g.card.Y; got < want-0.6 || got > want+0.6 {
				t.Errorf("shape sits at %.1f, want %.1f (band top %.1f)",
					got, want, p.lay.BandY)
			}
		})
	}
}
