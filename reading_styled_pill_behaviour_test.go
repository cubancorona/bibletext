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
	focusNoteAtVerse(st, last)
	if st.NoteID != lastID {
		t.Errorf("tapping the last paragraph's pill opened note %d, want %d — a "+
			"reader tapping one paragraph must not be shown another's note", st.NoteID, lastID)
	}
	if st.NoteMinimized {
		t.Errorf("tapping a pill must OPEN the note, not leave it collapsed")
	}

	// And back the other way, or the pills only work in one direction.
	focusNoteAtVerse(st, first)
	if st.NoteID != firstID {
		t.Errorf("tapping the first paragraph's pill opened note %d, want %d", st.NoteID, firstID)
	}
}

// A pill that opens a note must be reachable in the paragraph it names: the
// band verse the pill is drawn at is the verse focusNoteAtVerse is called with,
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
		focusNoteAtVerse(st, g.anchorVerse)
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
	applyNoteForCurrentChapter(st)
	if !notesSuppressed(st) {
		t.Fatalf("fixture must actually suppress: the mark is not foreign")
	}

	focusNoteAtVerse(st, last)

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
