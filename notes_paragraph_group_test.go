package bibletext

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

// The case that started this: five notes on one chapter, two paragraphs.
// One pill today, labelled with the chapter's count and standing over one of
// the two paragraphs. Grouped, each paragraph can carry its own pill and its
// own number.
func TestNotesGroupUnderTheParagraphThatCarriesThem(t *testing.T) {
	paras := [][]Verse{
		{{Verse: 1}, {Verse: 2}, {Verse: 3}},
		{{Verse: 4}, {Verse: 5}, {Verse: 6}},
	}
	note := func(lo int) drawnNote {
		return drawnNote{Placement: placement{Kind: placedNative,
			Here: []anchorRun{{Lo: lo, Hi: lo}}}}
	}
	// Two in the first paragraph, three in the second — deliberately shuffled,
	// because the plan hands notes over newest-first, not in reading order.
	got := groupNotesByParagraph(paras, []drawnNote{
		note(5), note(2), note(6), note(3), note(4),
	})
	if len(got) != 2 {
		t.Fatalf("want 2 paragraph groups, got %d", len(got))
	}
	if got[0].ParaIndex != 0 || got[1].ParaIndex != 1 {
		t.Fatalf("groups must come out in reading order, got paragraphs %d then %d",
			got[0].ParaIndex, got[1].ParaIndex)
	}
	if len(got[0].Notes) != 2 {
		t.Errorf("first paragraph carries 2 notes, grouped %d", len(got[0].Notes))
	}
	if len(got[1].Notes) != 3 {
		t.Errorf("second paragraph carries 3 notes, grouped %d", len(got[1].Notes))
	}
	// The band opens above the paragraph, found by the EARLIEST noted verse in
	// it — not by whichever note the plan happened to hand over first.
	if got[0].BandVerse != 2 {
		t.Errorf("first group's band verse = %d, want 2 (the earlier of vv2,3)", got[0].BandVerse)
	}
	if got[1].BandVerse != 4 {
		t.Errorf("second group's band verse = %d, want 4 (the earliest of vv4,5,6)", got[1].BandVerse)
	}
}

// A note the chapter cannot place has no band to open. Forcing it into a
// neighbouring paragraph would stand a sticker over a passage the note is not
// about, so it is dropped from the grouping instead.
// A note anchored at NO verse joins the chapter-top group; one anchored at a
// verse this chapter does not contain joins nothing.
//
// The two used to be one case — both were dropped — and dropping the first is
// what left the pills' counts short of the chapter's total. An anchorless note
// is a whole-chapter note: it belongs to the chapter, so it belongs at the top
// of it. A note anchored at verse 99 of a two-verse chapter belongs to a
// paragraph that is not here, and there is nothing truthful to say about it.
func TestAnAnchorlessNoteJoinsTheTopGroupAndAnAbsentVerseJoinsNothing(t *testing.T) {
	paras := [][]Verse{{{Verse: 1}, {Verse: 2}}}
	anchorless := drawnNote{Placement: placement{Kind: unplacedAbsent}}
	beyond := drawnNote{Placement: placement{Kind: placedNative,
		Here: []anchorRun{{Lo: 99, Hi: 99}}}}

	got := groupNotesByParagraph(paras, []drawnNote{anchorless, beyond})
	if len(got) != 1 {
		t.Fatalf("the anchorless note belongs to the chapter-top group and the "+
			"absent-verse one to nothing, so one group; got %d", len(got))
	}
	if got[0].ParaIndex != chapterTopGroup {
		t.Errorf("the surviving group is at paragraph %d, want the chapter-top group (%d)",
			got[0].ParaIndex, chapterTopGroup)
	}
	if len(got[0].Notes) != 1 {
		t.Errorf("the top group holds %d notes, want just the anchorless one — the "+
			"note anchored beyond the chapter must not have joined it", len(got[0].Notes))
	}
	if got[0].BandVerse != 1 {
		t.Errorf("the top group's band verse is %d, want the chapter's first (1) so "+
			"the band opens above everything", got[0].BandVerse)
	}
}

// The gate must default to the shipped model, and no release surface may write
// it. If this ever fails, readers are getting an unfinished collapsed state.
// OFF until the pills reach every surface, and this pins it so the flip is a
// deliberate edit rather than a drift.
//
// The flag reaches ONE surface: iOS, macOS and Android have a single sticker and
// no pill row. Turning it on was tried and reverted, because it split the
// collapsed model across platforms — per-paragraph counts on desktop, the
// chapter-wide chip on the phone — which is worse than either model applied
// everywhere. The port is what unblocks the flip; see docs/BACKLOG.md.
func TestPillPerParagraphIsOffByDefault(t *testing.T) {
	if notesPillPerParagraph {
		t.Fatal("notesPillPerParagraph must default to false until every surface " +
			"draws the groups: on is a split collapsed model, not a shipped one")
	}
}

// Off, the groups are withheld entirely — a surface that asks gets nil and
// keeps drawing what it draws today, so wiring one up cannot change shipped
// behaviour until the flag is deliberately flipped.
func TestGroupsAreWithheldWhileTheGateIsOff(t *testing.T) {
	verses := []Verse{{Verse: 1, Text: "a"}, {Verse: 2, Text: "b"}}
	plan := chapterPlan{Notes: []drawnNote{{Placement: placement{
		Kind: placedNative, Here: []anchorRun{{Lo: 1, Hi: 1}}}}}}
	state := &AppState{}

	prev := notesPillPerParagraph
	defer func() { notesPillPerParagraph = prev }()

	// Set explicitly in BOTH directions rather than leaning on the default: the
	// default has changed once and the behaviour under each setting is what this
	// test is about.
	notesPillPerParagraph = false
	if got := chapterNoteGroups(state, plan, verses); got != nil {
		t.Fatalf("gate off must withhold the groups, got %d", len(got))
	}
	notesPillPerParagraph = true
	if got := chapterNoteGroups(state, plan, verses); len(got) != 1 {
		t.Fatalf("gate on must yield the paragraph's group, got %d", len(got))
	}
}

// The behaviour itself, driven through the projection rather than inferred
// from the grouping: two notes in different paragraphs, both minimized, must
// leave the pill at chapter scope (VerseLo 0 — the anchorless placement that
// opens the band at the top), because the pill's label counts both.
func TestTheAggregatePillAnchorsAtChapterScope(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	st := planTestState(t)
	verses := longEnoughForTwoParagraphs()
	st.Bible.Verses["John"] = map[int][]Verse{3: verses}
	st.CurrentBook, st.CurrentChapter = "John", 3
	paras := groupVersesIntoParagraphs(verses)
	if len(paras) < 2 {
		t.Fatalf("fixture must break into 2+ paragraphs, got %d", len(paras))
	}
	last := paras[len(paras)-1][0].Verse

	for _, v := range []int{1, last} {
		n, ok := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
			Book: "John", Chapter: 3, VerseLo: v, Text: "fixture note"})
		if !ok {
			t.Fatalf("could not store the note on v%d", v)
		}
		setNoteMinimizedByID(appPrefs(), n.ID, true)
	}
	applyNoteForCurrentChapter(st)

	if !st.NoteMinimized {
		t.Fatalf("both notes are minimized; the projection reports open")
	}
	if st.NoteVerseLo != 0 {
		t.Errorf("the pill counts notes in %d paragraphs but anchors at v%d; a "+
			"chapter-wide count must sit at chapter scope (VerseLo 0), or it points "+
			"at one passage while describing several", len(paras), st.NoteVerseLo)
	}
}

// One paragraph carrying every note is not a chapter-wide fact: count and
// position already agree, so the true anchor is kept.
func TestASingleNotedParagraphKeepsItsAnchor(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	st := planTestState(t)
	verses := longEnoughForTwoParagraphs()
	st.Bible.Verses["John"] = map[int][]Verse{3: verses}
	st.CurrentBook, st.CurrentChapter = "John", 3

	for _, v := range []int{1, 2} { // both in the first paragraph
		n, _ := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
			Book: "John", Chapter: 3, VerseLo: v, Text: "fixture note"})
		setNoteMinimizedByID(appPrefs(), n.ID, true)
	}
	applyNoteForCurrentChapter(st)

	if st.NoteVerseLo == 0 {
		t.Error("every note is in one paragraph, so the pill still describes that " +
			"paragraph: moving it to chapter scope drops a true anchor for nothing")
	}
}

// A chapter whose verses are long enough that groupVersesIntoParagraphs really
// breaks it — the 320-character rule, at a sentence end.
func longEnoughForTwoParagraphs() []Verse {
	long := "This verse is deliberately long so that the paragraph splitter reaches its " +
		"character threshold and breaks at the next sentence ending, which is here."
	out := make([]Verse, 0, 8)
	for i := 1; i <= 8; i++ {
		out = append(out, Verse{BookName: "John", Chapter: 3, Verse: i, Text: long})
	}
	return out
}

// Pressing − must leave the collapsed state derived from the store, not from
// wherever the expanded note happened to be. The verb used to set Minimized by
// hand and leave the anchor alone, so minimizing a note in a chapter whose
// notes span paragraphs left the pill over that note's paragraph while
// labelling it with the chapter's whole count.
func TestMinimizingRederivesTheCollapsedAnchor(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	st := planTestState(t)
	verses := longEnoughForTwoParagraphs()
	st.Bible.Verses["John"] = map[int][]Verse{3: verses}
	st.CurrentBook, st.CurrentChapter = "John", 3
	paras := groupVersesIntoParagraphs(verses)
	if len(paras) < 2 {
		t.Fatalf("fixture must break into 2+ paragraphs, got %d", len(paras))
	}
	deep := paras[len(paras)-1][0].Verse

	for _, v := range []int{1, deep} {
		if _, ok := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
			Book: "John", Chapter: 3, VerseLo: v, Text: "fixture note"}); !ok {
			t.Fatalf("could not store the note on v%d", v)
		}
	}
	applyNoteForCurrentChapter(st)
	if st.NoteMinimized {
		t.Fatal("a fresh chapter should open its display note; nothing to minimize")
	}
	opened := st.NoteVerseLo

	hideCurrentNote(st)

	if !st.NoteMinimized {
		t.Fatal("pressing − must collapse the note")
	}
	if st.NoteVerseLo == opened && opened != 0 {
		t.Errorf("the pill kept the expanded note's anchor (v%d) after −; collapsed, it "+
			"counts both paragraphs and must be re-derived to chapter scope", opened)
	}
	if st.NoteVerseLo != 0 {
		t.Errorf("collapsed across %d paragraphs, the pill must sit at chapter scope "+
			"(VerseLo 0), got v%d", len(paras), st.NoteVerseLo)
	}
}
