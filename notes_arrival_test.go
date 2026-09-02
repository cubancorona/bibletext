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
			explicit: true, wantClass: arriveBand, wantVerse: sameParaA,
			why: "the band is above this verse's paragraph",
		},
		{
			name: "ANOTHER verse of the note's paragraph", anchor: sameParaA, markVerse: sameParaB, present: true,
			explicit: true, wantClass: arriveBand, wantVerse: sameParaB,
			why: "the case Android decided by comparing verses, so the card went above the fold",
		},
		{
			name: "a verse in a different paragraph", anchor: sameParaA, markVerse: otherPara, present: true,
			explicit: true, wantClass: arriveVerse, wantVerse: otherPara,
			why: "a note about another passage must not drag the reader to it",
		},
		{
			name: "an anchorless note, arriving mid-chapter", anchor: 0, markVerse: otherPara, present: true,
			explicit: true, wantClass: arriveVerse, wantVerse: otherPara,
			why: "its band is drawn at the CHAPTER TOP, not above whatever paragraph " +
				"the reader happens to arrive at — the Apple panes only appeared to " +
				"say otherwise because their anchor range fell back to the highlight",
		},
		{
			name: "an anchorless note, arriving at the first paragraph", anchor: 0,
			markVerse: sameParaA, present: true,
			explicit: true, wantClass: arriveBand, wantVerse: sameParaA,
			why: "the chapter-top band really is above this paragraph",
		},
		{
			name: "no note at all", anchor: 0, markVerse: otherPara, present: false,
			explicit: true, wantClass: arriveVerse, wantVerse: otherPara,
			why: "the wash is the only target",
		},
		{
			name: "no mark, but a note", anchor: sameParaA, present: true,
			explicit: true, wantClass: arriveBand, wantVerse: sameParaA,
			why: "on an EXPLICIT arrival the note's own verse stands in for the mark",
		},
		{
			name: "a PLAIN entry, with a mark", anchor: sameParaA, markVerse: sameParaA, present: true,
			wantClass: arriveNothing,
			why: "arrows are browsing, not a request to be taken anywhere — the chapter " +
				"opens like any other and the wash still says where the note is",
		},
		{
			name: "a PLAIN entry, collapsed note only", anchor: sameParaA, present: true,
			wantClass: arriveNothing,
			why: "merely entering a chapter that carries a note must not drag the " +
				"reader to its pill — the report that renamed this rule",
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
			name: "an EXPLICIT jump while narration follows", anchor: sameParaA, markVerse: sameParaA, present: true,
			following: true, explicit: true, wantClass: arriveBand, wantVerse: sameParaA,
			why: "the reader asked to look somewhere; the jump places once and the " +
				"follow's own channel re-takes the view on its next tick",
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

// THE EMULATOR'S LESSON, pinned. Three noted paragraphs collapsed into the
// single chapter pill: an arrival inside one of them must be told "verse",
// because only ONE band is drawn and it is not there. Turn per-paragraph pills
// on and the same fixture answers "band", because then every noted paragraph
// really does carry one.
//
// What actually went wrong on the emulator was the ANCHORLESS branch, not the
// group list: a collapsed set parks at chapter scope with no anchor, and the
// rule read that as "the band is above whatever paragraph you are arriving at"
// — which is what the Apple panes appeared to do, and only because their anchor
// range fell back to the highlight range. The reader was sent to a band drawn
// at the top of the chapter. Nothing failed; they simply landed in the wrong
// place, which is indistinguishable from nothing happening.
//
// This holds BOTH halves so the two answers cannot quietly become one.
func TestOnlyDrawnBandsWinTheArrival(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer deleteAllNotes(appPrefs())
	defer setNotesEnabled(true)

	verses := enumerationChapter()
	paras := groupVersesIntoParagraphs(verses)
	if len(paras) < 4 {
		t.Fatalf("the fixture has %d paragraphs; this needs three noted ones plus "+
			"room to arrive away from the first", len(paras))
	}
	noted := []int{paras[0][0].Verse, paras[1][0].Verse, paras[2][0].Verse}
	// Arriving inside paragraph 1: noted, not the first (so a chapter-top
	// reservation cannot be what answers), and not the newest note's.
	arriving := paras[1][len(paras[1])-1].Verse

	orig := notesPillPerParagraph
	defer func() { notesPillPerParagraph = orig }()

	for _, tc := range []struct {
		name  string
		pills bool
		want  noteArrival
		why   string
	}{
		{
			name: "one chapter pill", pills: false, want: arriveVerse,
			why: "ONE band is reserved, above the plan's own note. This paragraph " +
				"has a note but no reservation, and being told 'band' sends the " +
				"reader where nothing is drawn — which is what an emulator did " +
				"before this guard existed",
		},
		{
			name: "one pill per paragraph", pills: true, want: arriveBand,
			why: "now this paragraph really does carry a band, and scrolling to the " +
				"verse would put its pill above the fold",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			notesPillPerParagraph = tc.pills
			deleteAllNotes(appPrefs())
			st := planTestState(t)
			st.Bible.Verses["John"][3] = verses
			for i, v := range noted {
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
			st.setMark(hlSearch, VerseSpan{
				VersionID: "web", Book: "John", Chapter: 3, Lo: arriving, Hi: arriving,
			})
			// The route being modelled — a tapped search result — is explicit
			// (goToVerseRange sets forceReposition); a PLAIN entry arrives
			// nowhere at all now, which its own table case pins.
			st.forceReposition = true

			plan := buildChapterPlan(st, appPrefs(), st.Bible)
			c := chapterNoteChrome(st, plan, verses)
			if c.Arrival != tc.want {
				t.Errorf("arrival = %v (anchor %d), want %v — %s",
					c.Arrival, c.Anchor, tc.want, tc.why)
			}
		})
	}
}
