package bibletext

// The chapter plan (S7, notes_plan.go): the model is plural, the view is not,
// and NOTHING VISIBLE CHANGED. These tests hold the plan's own contract — the
// one gate, derived suppression, the cap as a view rule with zero store
// residue, the stable order — and the fingerprint's two duties: fold what
// changes pixels, and NEVER flap run-to-run.

import (
	"fmt"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func planTestState(t *testing.T) *AppState {
	t.Helper()
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	return &AppState{
		Bible: bd, CurrentBook: "John", CurrentChapter: 3,
		CurrentVersion: "web", loadPhase: loadReady,
		loadedVersions: map[string]*BibleData{"web": bd},
	}
}

// The fingerprint MUST NOT flap: built 50 times over a many-note store —
// several translations, several chapters, an unplaced note, a minimized one —
// it answers one string. A flap here would repaint the native panes on every
// push and rewrite nothing visible; the old store's map-range ordering is
// exactly the shape this pins out.
func TestChapterPlanFingerprintDoesNotFlap(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	origNow := noteNow
	noteNow = func() int64 { return 1_700_000_000 }
	defer func() { noteNow = origNow }()

	// Many notes: same passage under three translations, one minimized, other
	// chapters of the same book, another book, and (via Esther under webc,
	// read in web… kept simple: an absent-verse note) the unplaced group.
	for i, vid := range []string{"web", "bsb", "webc"} {
		addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: vid, Book: "John", Chapter: 3,
			VerseLo: 16, Text: fmt.Sprintf("john 3 under %s", vid), Minimized: i == 1})
	}
	for c := 1; c <= 4; c++ {
		addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "John", Chapter: c,
			VerseLo: 1, Text: fmt.Sprintf("john %d", c)})
	}
	addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "Psalms", Chapter: 23,
		VerseLo: 1, Text: "elsewhere entirely"})
	// A verse far past the sample chapter: resolves placedNative regardless —
	// resolution trusts the anchor for the note's own translation — while a
	// FOREIGN one would need real verse data; the point here is bulk, not arms.
	addNote(appPrefs(), StoredNote{Kind: noteKindMine, VersionID: "web", Book: "John", Chapter: 3,
		VerseLo: 2, Text: "my own note, never drawn"})

	st := planTestState(t)
	addRecentChapter(st, "John", 3)

	first := buildChapterPlan(st, appPrefs(), st.Bible)
	if len(first.Notes) < 3 {
		t.Fatalf("precondition: the plan should carry the passage's notes, got %d", len(first.Notes))
	}
	for i := 0; i < 50; i++ {
		again := buildChapterPlan(st, appPrefs(), st.Bible)
		if again.Fingerprint != first.Fingerprint {
			t.Fatalf("fingerprint flapped on build %d:\n  %q\nvs %q", i, again.Fingerprint, first.Fingerprint)
		}
	}
}

// The one gate: notesFeatureOn()==false yields an EMPTY plan — an empty PLAN,
// not merely an empty bubble — with the Notice still carried and the passage's
// own tint untouched.
func TestFeatureOffYieldsAnEmptyPlanWithTheNoticeCarried(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())
	defer setNotesEnabled(true)

	addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "John", Chapter: 3,
		VerseLo: 16, Text: "a note"})
	st := planTestState(t)
	addRecentChapter(st, "John", 3)
	st.NoteNotice = "This note could not be read."
	setNotesEnabled(false)

	// A search mark: the passage view is unaffected by the gate.
	st.setMark(hlSearch, VerseSpan{VersionID: "web", Book: "John", Chapter: 3, Lo: 1, Hi: 1})

	plan := buildChapterPlan(st, appPrefs(), st.Bible)
	if len(plan.Notes) != 0 || len(plan.Unplaced) != 0 {
		t.Errorf("feature off must empty the PLAN: %d notes, %d unplaced", len(plan.Notes), len(plan.Unplaced))
	}
	if plan.Notice != "This note could not be read." {
		t.Errorf("the notice must still be carried: %q", plan.Notice)
	}
	if plan.Tints.of(Verse{BookName: "John", Chapter: 3, Verse: 1}) == tintNone {
		t.Error("the search mark's tint must survive the notes gate — the passage view is unaffected")
	}
}

// Suppression is DERIVED and stands the notes down without emptying the plan:
// a live mark not owned by a note means zero Open, Notes still present, and
// nothing written — clear the mark and the notes come back exactly as they
// were, because there is no second copy to restore from.
func TestSuppressionMeansZeroOpenWithNotesStillPresent(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	stored, _ := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "John", Chapter: 3,
		VerseLo: 16, Text: "the note"})
	st := planTestState(t)
	addRecentChapter(st, "John", 3)

	if d, ok := buildChapterPlan(st, appPrefs(), st.Bible).openNote(); !ok || d.Note.ID != stored.ID {
		t.Fatal("precondition: the note should be open at rest")
	}

	// A search result arrives on the same chapter — the real writer.
	openSearchResultRange(st, Verse{BookName: "John", Chapter: 3, Verse: 1}, 0)
	plan := buildChapterPlan(st, appPrefs(), st.Bible)
	if _, ok := plan.openNote(); ok {
		t.Error("a foreign mark must stand every note down: something is still Open")
	}
	if len(plan.Notes) != 1 {
		t.Errorf("suppression must not empty the plan: %d notes", len(plan.Notes))
	}
	if minimizedInStore(allNotesForBrowsing(appPrefs()), "the note") {
		t.Error("suppression wrote Minimized — a forged collapse the reader never chose")
	}

	// The mark clears; the note comes back open, untouched.
	st.clearMark()
	if _, ok := buildChapterPlan(st, appPrefs(), st.Bible).openNote(); !ok {
		t.Error("clearing the foreign mark must restore the open note — suppression stores nothing, so it must cost nothing")
	}
}

// The cap is a view rule with ZERO store residue: three expanded notes on one
// passage, the plan opens at most planOpenLimit, and the store still says all
// three are un-minimized. (This replaces the pre-S7 sentinel test that the cap
// was not representable — the plan can say it now, and what must stay true
// forever is that saying it writes nothing.)
func TestTheCapOpensOneAndLeavesZeroStoreResidue(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	for _, id := range []string{"web", "bsb", "webc"} {
		addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: id, Book: "John", Chapter: 3,
			VerseLo: 16, Text: "note under " + id})
	}
	st := planTestState(t)
	addRecentChapter(st, "John", 3)

	plan := buildChapterPlan(st, appPrefs(), st.Bible)
	if len(plan.Notes) != 3 {
		t.Fatalf("precondition: all three notes should be on the plan, got %d", len(plan.Notes))
	}
	open := 0
	for _, d := range plan.Notes {
		if d.Open {
			open++
		}
	}
	if open != planOpenLimit {
		t.Errorf("the cap should open exactly %d of 3, got %d", planOpenLimit, open)
	}
	expanded := 0
	for _, n := range allNotesForBrowsing(appPrefs()) {
		if !n.Minimized {
			expanded++
		}
	}
	if expanded != 3 {
		t.Errorf("the cap left store residue: %d of 3 notes still un-minimized — a cap-by-action "+
			"minimize would be byte-identical to a genuine one and no migration could tell them apart", expanded)
	}
}

// The stable order: Received descending, ID the tiebreak — never map order —
// and the R4 group: a note for this BOOK with no home in this translation is
// carried as Unplaced with its quiet sentence, not dropped.
func TestPlanOrderAndTheUnplacedGroup(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	origNow := noteNow
	now := int64(1_700_000_000)
	noteNow = func() int64 { now++; return now }
	defer func() { noteNow = origNow }()

	older, _ := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "Esther", Chapter: 4,
		VerseLo: 1, Text: "older"})
	newer, _ := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "bsb", Book: "Esther", Chapter: 4,
		VerseLo: 1, Text: "newer"})
	// Greek Esther: webc's numbering does not correspond — the unplaced arm.
	unpl, _ := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "webc", Book: "Esther", Chapter: 4,
		VerseLo: 1, Text: "greek esther"})

	st := planTestState(t)
	// The sample canon has no Esther text; give it the chapter, or the book
	// existence test (rightly) files every foreign note as unplacedNoBook.
	st.Bible.Verses["Esther"] = map[int][]Verse{4: {{BookName: "Esther", Chapter: 4, Verse: 1, Text: "esther"}}}
	st.CurrentBook, st.CurrentChapter = "Esther", 4
	applyNoteForCurrentChapter(st)

	plan := buildChapterPlan(st, appPrefs(), st.Bible)
	if len(plan.Notes) != 2 || plan.Notes[0].Note.ID != newer.ID || plan.Notes[1].Note.ID != older.ID {
		t.Fatalf("order must be Received desc: got %+v", planIDs(plan.Notes))
	}
	if plan.Notes[0].Label == "" || plan.Notes[1].Label != "" {
		t.Errorf("labels: followed notes carry the translation, native carry none — got %q / %q",
			plan.Notes[0].Label, plan.Notes[1].Label)
	}
	if len(plan.Unplaced) != 1 || plan.Unplaced[0].Note.ID != unpl.ID {
		t.Fatalf("the incommensurable note belongs in Unplaced: %+v", planIDs(plan.Unplaced))
	}
	if s := plan.Unplaced[0].sentence(); s != "The numbering here does not correspond to the note's." {
		t.Errorf("the R4 sentence is placementCopy's, verbatim: %q", s)
	}
}

func planIDs(list []drawnNote) []uint64 {
	out := make([]uint64, 0, len(list))
	for _, d := range list {
		out = append(out, d.Note.ID)
	}
	return out
}

// NOTHING VISIBLE CHANGES (S7 rule 5): the banner's rendered text for the
// single-note case, driven through the REAL store and derive, is exactly what
// it was before the plan existed — bubble and chip both. The strings below
// were captured against the pre-S7 tree; if this fails, the projection has
// changed a pixel the step promised not to.
func TestNoteBannerRendersIdenticallyThroughThePlan(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	pinBannerPlatform(t)
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	st := psalm23State()
	st.CurrentBook, st.CurrentChapter = "Psalms", 23
	addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "Psalms", Chapter: 23,
		VerseLo: 1, Text: "This got me through last night."})
	applyNoteForCurrentChapter(st)

	got := seenText(t, buildNoteBanner(st), fyne.NewSize(700, 400))
	want := "note from friend psalms 23 this got me through last night."
	if got != want {
		t.Errorf("expanded banner changed:\n got %q\nwant %q", got, want)
	}

	hideCurrentNote(st)
	got = seenText(t, buildNoteBanner(st), fyne.NewSize(700, 400))
	if want = "show note"; got != want {
		t.Errorf("chip changed:\n got %q\nwant %q", got, want)
	}

	restoreCurrentNote(st)
	got = seenText(t, buildNoteBanner(st), fyne.NewSize(700, 400))
	want = "note from friend psalms 23 this got me through last night."
	if got != want {
		t.Errorf("restored banner changed:\n got %q\nwant %q", got, want)
	}
}

// buildChapterPlan is READ-ONLY over the store: deriving must never write —
// not bytes, not the cache — whatever the plan contains. (The only write any
// focus change may make is Hide's Minimized, and that lives in the verb.)
func TestBuildChapterPlanDoesNotWriteTheStore(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	for _, id := range []string{"web", "bsb"} {
		addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: id, Book: "John", Chapter: 3,
			VerseLo: 16, Text: "note under " + id})
	}
	raw := appPrefs().String(prefNotesStore)
	st := planTestState(t)
	st.focusNote(1)
	for i := 0; i < 3; i++ {
		buildChapterPlan(st, appPrefs(), st.Bible)
	}
	if appPrefs().String(prefNotesStore) != raw {
		t.Error("buildChapterPlan wrote the store — the derive must be read-only")
	}
}

// The reader's explicit CLOSE closes. implementation verification M1 deleted the
// focus-none branch from buildChapterPlan and every one of 1,280 enumeration
// cells stayed green — the rule existed only in prose. Now it exists here.
func TestFocusNoneMeansNothingIsOpen(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())
	addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
		Book: "Psalms", Chapter: 23, VerseLo: 1, Text: "open by default"})

	st := psalm23State()
	plan := buildChapterPlan(st, appPrefs(), st.Bible)
	if planOpenCount(plan) != 1 {
		t.Fatalf("precondition: the default should open the one note, got %d open", planOpenCount(plan))
	}

	st.focusNone() // the reader closed it — without minimizing it
	plan = buildChapterPlan(st, appPrefs(), st.Bible)
	if got := planOpenCount(plan); got != 0 {
		t.Errorf("focus=none still shows %d open note(s) — the reader's close does not close", got)
	}
	if len(plan.Notes) != 1 {
		t.Errorf("the closed note must remain in the plan as a chip: %d notes", len(plan.Notes))
	}
}

// Navigation resets focus to the default. implementation verification M3 removed
// resetNoteFocus from addRecentChapter and the suite stayed green.
func TestNavigationResetsTheFocus(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())
	addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
		Book: "Psalms", Chapter: 23, VerseLo: 1, Text: "the psalm note"})

	st := psalm23State()
	st.focusNone() // closed on this chapter...
	addRecentChapter(st, "Psalms", 23)
	plan := buildChapterPlan(st, appPrefs(), st.Bible)
	if got := planOpenCount(plan); got != 1 {
		t.Errorf("navigation must reset a closed focus to the default: %d open, want 1", got)
	}
}

// planOpenCount is the test-side count of Open notes in a plan.
func planOpenCount(p chapterPlan) int {
	n := 0
	for _, d := range p.Notes {
		if d.Open {
			n++
		}
	}
	return n
}
