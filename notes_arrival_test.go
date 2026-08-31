package bibletext

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

// The rule at its edges, stated once instead of transcribed into five dialects.
// Every case below is a situation a reader can actually be in.
func TestChapterNoteArrival(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer deleteAllNotes(appPrefs())
	defer setNotesEnabled(true)

	verses := enumerationChapter()
	paras := groupVersesIntoParagraphs(verses)
	if len(paras) < 2 {
		t.Fatalf("the fixture has %d paragraphs; the whole rule is about which one", len(paras))
	}
	// Two verses in DIFFERENT paragraphs, and two in the SAME one — the
	// distinction Android got wrong and the reason this fixture exists.
	pA, pB := paras[0], paras[1]
	sameParaA, sameParaB := pA[0].Verse, pA[len(pA)-1].Verse
	otherPara := pB[0].Verse
	if sameParaA == sameParaB {
		t.Fatalf("paragraph 0 holds one verse; it cannot express same-paragraph-different-verse")
	}
	if noteParagraphOf(paras, sameParaA) == noteParagraphOf(paras, otherPara) {
		t.Fatal("the two 'different paragraph' verses are in one paragraph")
	}

	mark := func(st *AppState, v int) {
		st.setMark(hlNote, VerseSpan{
			VersionID: st.currentVersion().ID, Book: st.CurrentBook,
			Chapter: st.CurrentChapter, Lo: v, Hi: v,
		})
	}

	for _, tc := range []struct {
		name      string
		anchor    int
		markVerse int
		present   bool
		following bool
		restore   bool
		explicit  bool
		wantClass noteArrival
		wantVerse int
		why       string
	}{
		{
			name: "the note's own verse", anchor: sameParaA, markVerse: sameParaA, present: true,
			wantClass: arriveBand, wantVerse: sameParaA,
			why: "the band is above this verse's paragraph",
		},
		{
			name: "ANOTHER verse of the note's paragraph", anchor: sameParaA, markVerse: sameParaB, present: true,
			wantClass: arriveBand, wantVerse: sameParaB,
			why: "the case Android decided by comparing verses, so the card went above the fold",
		},
		{
			name: "a verse in a different paragraph", anchor: sameParaA, markVerse: otherPara, present: true,
			wantClass: arriveVerse, wantVerse: otherPara,
			why: "a note about another passage must not drag the reader to it",
		},
		{
			name: "an anchorless note", anchor: 0, markVerse: otherPara, present: true,
			wantClass: arriveBand, wantVerse: otherPara,
			why: "a chapter-scope note is reserved above the paragraph being arrived at",
		},
		{
			name: "no note at all", anchor: 0, markVerse: otherPara, present: false,
			wantClass: arriveVerse, wantVerse: otherPara,
			why: "the wash is the only target",
		},
		{
			name: "no mark, but a note", anchor: sameParaA, present: true,
			wantClass: arriveBand, wantVerse: sameParaA,
			why: "the note's own verse stands in for the mark",
		},
		{
			name: "nothing to place", anchor: 0, present: false,
			wantClass: arriveNothing,
			why:       "neither a mark nor a note",
		},
		{
			name: "narration is following", anchor: sameParaA, markVerse: sameParaA, present: true,
			following: true, wantClass: arriveNothing,
			why: "the read-along owns the viewport, by declaration rather than by running last",
		},
		{
			name: "reopening, with a restore armed", anchor: sameParaA, markVerse: sameParaA, present: true,
			restore: true, wantClass: arriveNothing,
			why: "a note restored on reopen must not drag the reader back every launch",
		},
		{
			name: "an EXPLICIT arrival outranks the restore", anchor: sameParaA, markVerse: sameParaA, present: true,
			restore: true, explicit: true, wantClass: arriveBand, wantVerse: sameParaA,
			why: "the reader asked for this one",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := planTestState(t)
			st.Bible.Verses["John"][3] = verses
			if tc.markVerse > 0 {
				mark(st, tc.markVerse)
			}
			c := noteChrome{Anchor: tc.anchor}
			if tc.present {
				c.Text, c.Who = "a message", "Note from Friend"
			}
			gotClass, gotVerse := chapterNoteArrival(st, c, verses, nil, tc.following, tc.restore, tc.explicit)
			if gotClass != tc.wantClass {
				t.Errorf("class = %v, want %v — %s", gotClass, tc.wantClass, tc.why)
			}
			if tc.wantClass != arriveNothing && gotVerse != tc.wantVerse {
				t.Errorf("verse = %d, want %d", gotVerse, tc.wantVerse)
			}
		})
	}
}

// The Android defect, named on its own so its fix cannot be quietly lost: the
// predicate must be SAME PARAGRAPH, and a same-VERSE reading gives a different
// answer on a real chapter.
func TestSameParagraphIsNotSameVerse(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer deleteAllNotes(appPrefs())

	verses := enumerationChapter()
	paras := groupVersesIntoParagraphs(verses)
	first := paras[0]
	if len(first) < 2 {
		t.Fatal("the fixture's first paragraph holds one verse")
	}
	anchor, arriving := first[0].Verse, first[len(first)-1].Verse

	st := planTestState(t)
	st.Bible.Verses["John"][3] = verses
	st.setMark(hlNote, VerseSpan{
		VersionID: st.currentVersion().ID, Book: st.CurrentBook,
		Chapter: st.CurrentChapter, Lo: arriving, Hi: arriving,
	})
	c := noteChrome{Text: "a message", Who: "Note from Friend", Anchor: anchor}

	class, _ := chapterNoteArrival(st, c, verses, nil, false, false, true)
	if class != arriveBand {
		t.Fatalf("arriving at verse %d with the note on verse %d of the SAME "+
			"paragraph gave %v; the band is above that paragraph, so scrolling "+
			"to the verse's own line puts the card above the fold", arriving, anchor, class)
	}
	// The control: a same-VERSE reading of the same situation says otherwise,
	// which is what makes this cell worth having.
	if anchor == arriving {
		t.Fatal("the two verses are equal, so a same-verse rule would agree here " +
			"and this test would pass against the defect it exists to catch")
	}
}
