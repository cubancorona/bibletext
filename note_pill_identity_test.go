package bibletext

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

// A PILL MUST OPEN ITS OWN NOTE.
//
// The chapter-top group — the notes that point at no paragraph — is given
// paragraph 0's first verse as its band verse, deliberately, so the band opens
// above the whole chapter. groupNotesByParagraph says so in a comment that ends
// "which is why bands are matched by Key rather than by verse".
//
// Placement obeys that. The verb did not: focusNoteAtVerse matched on BandVerse
// and took the first group that matched, and the sort puts chapterTopGroup (-1)
// first. So on a chapter carrying BOTH a chapter-level note and a note on
// paragraph 0, both pills opened the chapter-level note, and paragraph 0's note
// could not be reached by pressing the pill drawn for it.
func TestEachPillOpensItsOwnNote(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer deleteAllNotes(appPrefs())
	defer setNotesEnabled(true)
	deleteAllNotes(appPrefs())

	orig := notesPillPerParagraph
	notesPillPerParagraph = true
	defer func() { notesPillPerParagraph = orig }()

	st := planTestState(t)
	verses := enumerationChapter()
	st.Bible.Verses["John"][3] = verses
	paras := groupVersesIntoParagraphs(verses)
	firstVerse := paras[0][0].Verse

	// A note ON paragraph 0...
	inPara, ok := addNote(appPrefs(), StoredNote{
		Kind: noteKindReceived, VersionID: "web", Book: "John", Chapter: 3,
		VerseLo: firstVerse, Text: "the note on paragraph zero",
	})
	if !ok {
		t.Fatal("paragraph note refused")
	}
	setNoteMinimizedByID(appPrefs(), inPara.ID, true)
	// ...and a CHAPTER-LEVEL note, which has no paragraph of its own and whose
	// band is therefore given paragraph 0's verse.
	chapterWide, ok := addNote(appPrefs(), StoredNote{
		Kind: noteKindReceived, VersionID: "web", Book: "John", Chapter: 3,
		Text: "the chapter-level note",
	})
	if !ok {
		t.Fatal("chapter note refused")
	}
	setNoteMinimizedByID(appPrefs(), chapterWide.ID, true)
	applyNoteForCurrentChapter(st)

	plan := buildChapterPlan(st, appPrefs(), st.Bible)
	groups := chapterNoteGroups(st, plan, verses)
	if len(groups) < 2 {
		t.Fatalf("the fixture produced %d groups; this defect needs the chapter-top "+
			"group AND paragraph 0's own group sharing a verse", len(groups))
	}
	// The collision is the precondition. Without it the test proves nothing.
	shared := 0
	for _, g := range groups {
		if g.BandVerse == firstVerse {
			shared++
		}
	}
	if shared < 2 {
		t.Fatalf("only %d groups carry verse %d; the two groups do not collide and "+
			"this test cannot see the defect", shared, firstVerse)
	}

	// Press each pill in turn. Each must open a DIFFERENT note.
	opened := map[uint64]string{}
	for _, g := range groups {
		st.resetNoteFocus()
		focusNoteAtGroup(st, g.Key)
		if st.NoteID == 0 {
			t.Errorf("the pill for group %d (para %d) opened nothing", g.Key, g.ParaIndex)
			continue
		}
		if prev, seen := opened[st.NoteID]; seen {
			t.Errorf("the pill for group %d (para %d) opened the SAME note as %s — "+
				"one of these paragraphs has a note the reader cannot reach by "+
				"pressing the pill drawn for it", g.Key, g.ParaIndex, prev)
			continue
		}
		opened[st.NoteID] = "group " + string(rune('0'+g.Key))
	}
	if len(opened) != len(groups) {
		t.Errorf("%d pills opened only %d distinct notes", len(groups), len(opened))
	}
}

// pressPillOnParagraphOf is what the tests below mean when they say "tap the
// pill for this verse's paragraph": it resolves the verse to the group the pill
// was drawn for, then presses that group's key — the same journey the pane
// makes (geom.groupKey -> focusNoteAtGroup).
//
// It FAILS on an ambiguous verse rather than picking one, because a verse
// carried by two groups is exactly the state this file exists to pin, and a
// helper that silently chose would hide it from every other test.
func pressPillOnParagraphOf(t *testing.T, st *AppState, verse int) {
	t.Helper()
	verses := st.Bible.GetChapter(st.CurrentBook, st.CurrentChapter)
	plan := buildChapterPlan(st, appPrefs(), st.Bible)
	var keys []int
	for _, g := range chapterNoteGroups(st, plan, verses) {
		if g.BandVerse == verse && len(g.Notes) > 0 {
			keys = append(keys, g.Key)
		}
	}
	switch len(keys) {
	case 0:
		t.Fatalf("no pill is drawn for verse %d's paragraph", verse)
	case 1:
		focusNoteAtGroup(st, keys[0])
	default:
		t.Fatalf("verse %d is the band verse of %d groups, so 'the pill for this "+
			"verse' names no single pill — press a KEY here", verse, len(keys))
	}
}
