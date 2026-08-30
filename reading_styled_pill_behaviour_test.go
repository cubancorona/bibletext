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
