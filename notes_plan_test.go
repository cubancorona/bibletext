package bibletext

// The chapter plan (S7, notes_plan.go): the model is plural, the view is not,
// and NOTHING VISIBLE CHANGED. These tests hold the plan's own contract — the
// one gate, derived suppression, the cap as a view rule with zero store
// residue, the stable order — and the fingerprint's two duties: fold what
// changes pixels, and NEVER flap run-to-run.

import (
	"fmt"
	"strings"
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

// The single-note banner, pinned string-for-string. Until S8 this test held
// the S7 promise that nothing visible changed; S8 changed the surface ON
// PURPOSE — the shared tailed bubble with the byline outside and the citation
// heading, and a chip that names its note instead of a bare "Show note" — so
// the strings below pin the S8 shape: bubble ↔ chip round-trips through the
// real verbs must land exactly here.
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
		VerseLo: 1, Text: "fixture plan message alpha"})
	applyNoteForCurrentChapter(st)

	got := seenText(t, buildNoteBanner(st), fyne.NewSize(700, 400))
	want := "psalms 23:1 fixture plan message alpha from friend"
	if got != want {
		t.Errorf("expanded banner changed:\n got %q\nwant %q", got, want)
	}

	hideCurrentNote(st)
	got = seenText(t, buildNoteBanner(st), fyne.NewSize(700, 400))
	if want = "psalms 23:1 · today · hidden"; got != want {
		t.Errorf("chip changed:\n got %q\nwant %q", got, want)
	}

	restoreCurrentNote(st)
	got = seenText(t, buildNoteBanner(st), fyne.NewSize(700, 400))
	want = "psalms 23:1 fixture plan message alpha from friend"
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

// The reader's explicit CLOSE closes. Removing the focus-none branch from
// buildChapterPlan does not fail the state-space enumeration, so this focused
// test holds the close invariant directly.
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

// Navigation resets focus to the default. The state-space enumeration does not
// detect removal of resetNoteFocus from addRecentChapter, so this focused test
// holds that transition directly.
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

// S8 sprang the recorded S7 trap: Open is FOLDED into the plan's Fingerprint
// (the banner draws it now), while the note half (noteFP) — the Apple BODY
// fingerprint's input — deliberately ignores it: the sticker draws the mirror
// plus the counts, none of which depend on Open, so a suppression flip or an
// explicit close repaints the Fyne banner without forcing a native
// NSAttributedString re-import.
func TestFingerprintFoldsOpenButTheBodyHalfIgnoresIt(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())
	addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "John", Chapter: 3,
		VerseLo: 16, Text: "the open one"})

	st := planTestState(t)
	addRecentChapter(st, "John", 3)
	base := buildChapterPlan(st, appPrefs(), st.Bible)
	if _, ok := base.openNote(); !ok {
		t.Fatal("precondition: the note should be open at rest")
	}

	// The reader explicitly closes it: Open flips, nothing else does.
	st.focusNone()
	closed := buildChapterPlan(st, appPrefs(), st.Bible)
	if _, ok := closed.openNote(); ok {
		t.Fatal("precondition: focus none should close the note")
	}
	if closed.Fingerprint == base.Fingerprint {
		t.Error("Open is not folded: the plan's Fingerprint did not move when the open note closed — " +
			"the S7 trap (a field the fingerprint forgot) is sprung open again")
	}
	if closed.noteFP != base.noteFP {
		t.Errorf("the note half must ignore Open — folding it there re-imports the whole "+
			"NSAttributedString on every focus flip:\n %q\nvs %q", closed.noteFP, base.noteFP)
	}
}

// The Apple push (S9): the bubble's body is the sender's words ALONE, the WHO
// line is the app's chrome carrying the byline and the counts, and everything
// the push depends on EXCEPT the derived suppression is folded by the body
// fingerprint. A suppression flip changes only the pushed presentation (pill)
// and must leave the body half alone — that repaint is the native side's own
// compare-and-refresh, never a chapter re-import.
func TestAppleStickerPushIsFoldedByTheBodyFingerprint(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "John", Chapter: 3,
		VerseLo: 16, Text: "the first note"})
	st := planTestState(t)
	addRecentChapter(st, "John", 3)

	snap := func() (string, string, bool, string) {
		text, who, pill, _ := appleStickerPush(st, buildChapterPlan(st, appPrefs(), st.Bible))
		return text, who, pill, chapterBodyFingerprint(st)
	}

	text1, who1, pill1, body1 := snap()
	if text1 != "the first note" {
		t.Fatalf("the bubble must hold the sender's words alone, got %q", text1)
	}
	if who1 != "Note from Friend" {
		t.Fatalf("one note: the who line is the plain byline, got %q", who1)
	}
	if pill1 {
		t.Fatal("an open note must not push the pill")
	}

	// A second note arrives on the passage: the count joins the WHO line (the
	// body stays the sender's words), and the body fingerprint must move with
	// it or the native pane would skip the repaint.
	addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "bsb", Book: "John", Chapter: 3,
		VerseLo: 16, Text: "the second note"})
	applyNoteForCurrentChapter(st)
	text2, who2, _, body2 := snap()
	if text2 == "" || strings.Contains(text2, "more note") {
		t.Fatalf("the count must never ride in the body: %q", text2)
	}
	if !strings.Contains(who2, "of 2 on this passage") {
		t.Errorf("the who line should carry the honest count: %q", who2)
	}
	if body2 == body1 {
		t.Error("the who line changed and the body fingerprint did not — the native pane would skip the repaint")
	}

	// A suppression flip: the pushed presentation becomes the pill with the
	// set's count, the words and the body half do not move.
	openSearchResultRange(st, Verse{BookName: "John", Chapter: 3, Verse: 1}, 0)
	text3, who3, pill3, body3 := snap()
	if !pill3 {
		t.Error("a foreign mark must stand the sticker down to the pill")
	}
	if who3 != "Notes · 2" {
		t.Errorf("the suppressed pill should carry the set's count, got %q", who3)
	}
	if text3 != text2 {
		t.Errorf("a suppression flip changed the pushed words:\n %q\nvs %q", text3, text2)
	}
	if body3 != body2 {
		t.Error("a suppression flip moved the body fingerprint — every search arrival would " +
			"re-import the whole NSAttributedString")
	}

	// Releasing the mark restores the expanded push, by derivation alone.
	st.clearMark()
	_, who4, pill4, _ := snap()
	if pill4 || !strings.Contains(who4, "of 2 on this passage") {
		t.Errorf("clearing the mark must restore the expanded sticker, got pill=%v who=%q", pill4, who4)
	}
}

// The who line and the pill, composed over every S9 shape: position counts,
// the unplaced tail, the minimized pill, and the unplaced-only chapter that
// pushes no body at all. Within the ABI, a text-less who selects the pill.
func TestAppleStickerPushComposition(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	// Three placed notes plus one the translation cannot show. The reading
	// book must be one the sample bible carries; the unplaced note rides on
	// the same BOOK under a version whose numbering does not correspond.
	st := planTestState(t)
	st.Bible.Verses["Esther"] = map[int][]Verse{4: {{BookName: "Esther", Chapter: 4, Verse: 1, Text: "esther"}}}
	st.CurrentBook, st.CurrentChapter = "Esther", 4
	for _, text := range []string{"first", "second", "third"} {
		addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "Esther", Chapter: 4,
			VerseLo: 1, Text: text})
	}
	addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "webc", Book: "Esther", Chapter: 4,
		VerseLo: 1, Text: "greek esther"})
	applyNoteForCurrentChapter(st)

	text, who, pill, next := appleStickerPush(st, buildChapterPlan(st, appPrefs(), st.Bible))
	if pill {
		t.Fatal("expanded: not a pill")
	}
	if text == "" {
		t.Fatal("expanded: the bubble carries the open note's words")
	}
	if !next {
		t.Error("three placed notes expanded: the count region must be a control (next)")
	}
	wantPrefix := "Note from Friend · "
	if !strings.HasPrefix(who, wantPrefix) ||
		!strings.Contains(who, "of 3 on this passage") ||
		!strings.Contains(who, "· 1 not shown here") {
		t.Errorf("who = %q, want byline · K of 3 on this passage · 1 not shown here", who)
	}

	// Minimize: the pill carries the whole set, placed and unplaced — and the
	// pill is never the selector (tapping it opens the focused note, as ever).
	hideCurrentNote(st)
	_, who, pill, next = appleStickerPush(st, buildChapterPlan(st, appPrefs(), st.Bible))
	if !pill || who != "Notes · 3 · 1 not shown" {
		t.Errorf("minimized pill = %q (pill=%v), want %q", who, pill, "Notes · 3 · 1 not shown")
	}
	if next {
		t.Error("the pill must not carry the next control")
	}

	// The unplaced-only chapter: no sender text exists, so the push is the
	// pill presentation with the sentence — never an empty sender bubble.
	deleteAllNotes(appPrefs())
	addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "webc", Book: "Esther", Chapter: 4,
		VerseLo: 1, Text: "greek esther"})
	addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "webc", Book: "Esther", Chapter: 5,
		VerseLo: 1, Text: "greek esther again"})
	applyNoteForCurrentChapter(st)
	text, who, pill, next = appleStickerPush(st, buildChapterPlan(st, appPrefs(), st.Bible))
	if text != "" {
		t.Errorf("unplaced-only: no sender words exist to show, got %q", text)
	}
	if !pill {
		t.Error("unplaced-only: must collapse to the pill presentation")
	}
	if who != "2 notes cannot be shown in this translation" {
		t.Errorf("unplaced-only who = %q", who)
	}
	if next {
		t.Error("unplaced-only: nothing to advance through, next must be false")
	}

	// And a single-note chapter still pushes the plain pill label when
	// minimized — today's presentation, unchanged.
	deleteAllNotes(appPrefs())
	addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "Esther", Chapter: 4,
		VerseLo: 1, Text: "only note", Minimized: true})
	applyNoteForCurrentChapter(st)
	if _, who, pill, _ = appleStickerPush(st, buildChapterPlan(st, appPrefs(), st.Bible)); !pill || who != "Note" {
		t.Errorf("single minimized note: pill=%v who=%q, want the plain \"Note\"", pill, who)
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
