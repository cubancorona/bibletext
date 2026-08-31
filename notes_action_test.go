package bibletext

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

// EVERY VERB MUST REACH THE OBJECT THE READER AIMED IT AT.
//
// That is the notes state machine's second rule, and a parameterless verb can
// only keep it while there is one card on the page: "the note" then has one
// meaning. With a card per paragraph the press has to name its target, and the
// order inside performNoteAction is the whole point — focusing AFTER the verb
// would delete one note and open another.
func TestAKeyedVerbActsOnTheNoteItNames(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer deleteAllNotes(appPrefs())
	defer setNotesEnabled(true)

	orig := notesPillPerParagraph
	notesPillPerParagraph = true
	defer func() { notesPillPerParagraph = orig }()

	// Two notes in two paragraphs. The plan chooses one; the reader aims at the
	// other.
	setup := func(t *testing.T) (*AppState, []noteParagraphGroup, []Verse) {
		t.Helper()
		deleteAllNotes(appPrefs())
		st := planTestState(t)
		verses := enumerationChapter()
		st.Bible.Verses["John"][3] = verses
		paras := groupVersesIntoParagraphs(verses)
		for i, v := range []int{paras[0][0].Verse, paras[1][0].Verse} {
			n, ok := addNote(appPrefs(), StoredNote{
				Kind: noteKindReceived, VersionID: "web", Book: "John", Chapter: 3,
				VerseLo: v, Text: "fixture note " + string(rune('a'+i)),
			})
			if !ok {
				t.Fatalf("fixture note %d refused", i)
			}
			setNoteMinimizedByID(appPrefs(), n.ID, true)
		}
		applyNoteForCurrentChapter(st)
		plan := buildChapterPlan(st, appPrefs(), st.Bible)
		groups := chapterNoteGroups(st, plan, verses)
		if len(groups) != 2 {
			t.Fatalf("fixture produced %d groups, want 2", len(groups))
		}
		return st, groups, verses
	}

	t.Run("restore opens the group named, not the plan's choice", func(t *testing.T) {
		st, groups, _ := setup(t)
		// Whichever the plan would choose, aim at the OTHER one.
		st.resetNoteFocus()
		performNoteAction(st, noteActionRestore, groups[0].Key)
		first := st.NoteID
		st.resetNoteFocus()
		performNoteAction(st, noteActionRestore, groups[1].Key)
		second := st.NoteID
		if first == 0 || second == 0 {
			t.Fatalf("a keyed restore opened nothing (%d, %d)", first, second)
		}
		if first == second {
			t.Error("both keys opened the same note — the key is not reaching the target")
		}
	})

	t.Run("delete removes the note named, and leaves the other", func(t *testing.T) {
		st, groups, _ := setup(t)
		// AIM AT THE ONE THE PLAN DID NOT CHOOSE. Aiming at the plan's own
		// choice cannot see the ordering inside performNoteAction: focusing
		// after the verb would then delete the same note by coincidence, and
		// the test would pass against the bug it exists for.
		applyNoteForCurrentChapter(st)
		chosen := st.NoteID
		var target, other noteParagraphGroup
		for _, g := range groups {
			if g.Notes[0].Note.ID == chosen {
				other = g
			} else {
				target = g
			}
		}
		if len(target.Notes) == 0 || len(other.Notes) == 0 {
			t.Fatalf("the plan's chosen note %d is not one of the two groups", chosen)
		}
		targetNote, otherNote := target.Notes[0].Note, other.Notes[0].Note
		if targetNote.ID == chosen {
			t.Fatal("the target IS the plan's choice; this cell proves nothing")
		}

		performNoteAction(st, noteActionDelete, target.Key)

		after := allNotesForBrowsing(appPrefs())
		stillThere := func(id uint64) bool {
			for _, n := range after {
				if n.ID == id {
					return true
				}
			}
			return false
		}
		if stillThere(targetNote.ID) {
			t.Error("the note the reader aimed at survived the delete")
		}
		if !stillThere(otherNote.ID) {
			t.Error("a note the reader did NOT aim at was deleted — the verb reached " +
				"the wrong object, which is the whole failure this rule exists for")
		}
	})

	t.Run("the focused key means what the parameterless verbs mean", func(t *testing.T) {
		st, _, _ := setup(t)
		applyNoteForCurrentChapter(st)
		chosen := st.NoteID
		if chosen == 0 {
			t.Fatal("the plan chose no note")
		}
		performNoteAction(st, noteActionHide, noteKeyFocused)
		if st.NoteID != chosen {
			t.Errorf("a focused verb moved the target from %d to %d; the existing "+
				"entry points mean 'the note the plan chose' and must keep meaning it",
				chosen, st.NoteID)
		}
	})
}

// noteKeyFocused must not collide with a real group key. Zero would: the
// chapter's first group is keyed 0, so a caller that forgot to pass a key would
// silently address it.
func TestTheFocusedKeyIsNotARealKey(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer deleteAllNotes(appPrefs())
	deleteAllNotes(appPrefs())

	st := planTestState(t)
	verses := enumerationChapter()
	st.Bible.Verses["John"][3] = verses
	paras := groupVersesIntoParagraphs(verses)
	addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
		Book: "John", Chapter: 3, VerseLo: paras[0][0].Verse, Text: "a note"})
	applyNoteForCurrentChapter(st)

	plan := buildChapterPlan(st, appPrefs(), st.Bible)
	for _, g := range chapterNoteGroups(st, plan, verses) {
		if g.Key == noteKeyFocused {
			t.Errorf("group key %d collides with noteKeyFocused; a press that "+
				"forgot its key would address this group instead of the focused note",
				g.Key)
		}
	}
	if noteKeyFocused >= chapterTopGroup {
		t.Errorf("noteKeyFocused (%d) is not below chapterTopGroup (%d)",
			noteKeyFocused, chapterTopGroup)
	}
}
