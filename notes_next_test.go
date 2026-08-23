package bibletext

// The Apple sticker's NEXT-NOTE selection (S10): the who-line's count region
// is a control, and a tap advances focus to the next note in the plan's
// stable order, wrapping — the Apple answer to "I see the pill with two notes
// but how do I select which one?". These tests hold the rotation itself
// (nextNoteFocusID), the verb's chip-tap parity (advanceNoteFocus carries the
// EXACT semantics of noteBannerChip's tap: un-minimize by ID, foreign mark
// stands aside, focus + re-projection), and the cost contract: a next-tap
// repaints the sticker through the native compare-and-refresh and the wash
// through the tint mutation — the BODY fingerprint must not move, or every
// selection tap would re-import the whole NSAttributedString.

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
)

// nextTestState seeds three received notes on John 3 (distinct verses,
// distinct Received times) and lands the reader on the chapter. Returns the
// state and the notes OLDEST FIRST as stored; the plan's stable order is the
// reverse (Received descending).
func nextTestState(t *testing.T) (*AppState, []StoredNote) {
	t.Helper()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())

	notes := make([]StoredNote, 0, 3)
	for i, text := range []string{"first", "second", "third"} {
		n, ok := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
			Book: "John", Chapter: 3, VerseLo: 16 + i, Text: text})
		if !ok {
			t.Fatalf("seeding note %q failed", text)
		}
		notes = append(notes, n)
	}
	st := planTestState(t)
	addRecentChapter(st, "John", 3)
	return st, notes
}

// The rotation: 1 → 2 → 3 → 1 over the plan's stable order (newest first),
// asserted on the note identity the verbs address AND on the who line's own
// count, which is what the reader watches move.
func TestNextTapRotatesThePlanWithWrap(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer deleteAllNotes(appPrefs())

	origNow := noteNow
	now := int64(1_700_000_000)
	noteNow = func() int64 { now++; return now }
	defer func() { noteNow = origNow }()

	st, notes := nextTestState(t)
	newest, middle, oldest := notes[2], notes[1], notes[0]

	wantOrder := []struct {
		id  uint64
		pos string
	}{
		{newest.ID, "1 of 3"}, // the landing default: newest first
		{middle.ID, "2 of 3"},
		{oldest.ID, "3 of 3"},
		{newest.ID, "1 of 3"}, // the wrap
	}
	for i, want := range wantOrder {
		if st.NoteID != want.id {
			t.Fatalf("step %d: sticker holds note %d, want %d", i, st.NoteID, want.id)
		}
		_, who, pill, next := appleStickerPush(st, buildChapterPlan(st, appPrefs(), st.Bible))
		if pill || !next {
			t.Fatalf("step %d: expanded multi-note sticker must be nextable (pill=%v next=%v)", i, pill, next)
		}
		if !strings.Contains(who, want.pos+" on this passage") {
			t.Fatalf("step %d: who = %q, want the count %q", i, who, want.pos)
		}
		advanceNoteFocus(st)
	}
}

// Chip-tap parity: the advance clears a foreign mark (selecting a note is the
// reader choosing it as the page's reason), un-minimizes the tapped-to note by
// its own ID and no other (the tap IS the Show verb) — and CYCLES IN PLACE:
// no forceReposition on any platform (the selection swaps where it is, the
// viewport stays; the far-off-note tradeoff is recorded on advanceNoteFocus).
func TestNextTapMatchesChipTapSemantics(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer deleteAllNotes(appPrefs())

	origNow := noteNow
	now := int64(1_700_000_000)
	noteNow = func() int64 { now++; return now }
	defer func() { noteNow = origNow }()

	st, notes := nextTestState(t)
	middle := notes[1] // the next-in-order after the newest

	// The reader stored-minimized the note the tap will land on, and a foreign
	// mark owns the page (a Go-to result on the same chapter).
	setNoteMinimizedByID(appPrefs(), middle.ID, true)
	goToVerseRange(st, "John", 3, 1, 1)
	if !notesSuppressed(st) {
		t.Fatal("precondition: the foreign mark should stand the notes down")
	}
	st.forceReposition = false
	// A same-chapter re-render leaves a captured scroll restore standing
	// (pushChapterHTML); the advance is an explicit arrival and must outrank
	// it, exactly as applyShareTarget and openSearchResultRange do — a
	// standing restore forces the push down the slow re-import path.
	st.restore = &restoreAnchor{Book: "John", Chapter: 3, Verse: 1}

	advanceNoteFocus(st)

	if st.restore != nil {
		t.Error("the advance must drop the pending restore — an explicit arrival outranks \"where you left off\"")
	}

	if st.NoteID != middle.ID {
		t.Fatalf("the tap should land on the next note %d, got %d", middle.ID, st.NoteID)
	}
	if notesSuppressed(st) {
		t.Error("the foreign mark must stand aside — the tap is the reader's new choice")
	}
	if st.mark.live() && !st.mark.fromNote() {
		t.Error("the mark on the page must now be the note's own")
	}
	for _, n := range allNotesForBrowsing(appPrefs()) {
		wantMin := false // middle was restored by the tap; the others never flipped
		if n.Minimized != wantMin {
			t.Errorf("note %q Minimized=%v after the tap — the Show verb must reach exactly the tapped-to note", n.Text, n.Minimized)
		}
	}
	if st.forceReposition {
		t.Error("the cycle must stay IN PLACE: a next-tap is a selection, not an arrival — " +
			"forceReposition must stay false on every platform")
	}
}

// The cost contract: a next-tap changes the sticker (who line, words, anchor)
// and the wash (the mark moves to the next note's verse) — and NOT the body.
// The sticker repaint rides the native compare-and-refresh
// (bibleTextSetNote / bibleTextMacSetNote) and the wash rides the tint
// mutation (applyNativeTint), so the body fingerprint moving here would be a
// full NSAttributedString re-import bought for a selection tap.
func TestNextTapLeavesTheBodyFingerprintAlone(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer deleteAllNotes(appPrefs())

	origNow := noteNow
	now := int64(1_700_000_000)
	noteNow = func() int64 { now++; return now }
	defer func() { noteNow = origNow }()

	st, _ := nextTestState(t)

	bodyBefore := chapterBodyFingerprint(st)
	tintBefore := chapterTint(st).fingerprint()
	_, whoBefore, _, _ := appleStickerPush(st, buildChapterPlan(st, appPrefs(), st.Bible))

	advanceNoteFocus(st)

	if body := chapterBodyFingerprint(st); body != bodyBefore {
		t.Errorf("a next-tap moved the BODY fingerprint — the selection would re-import the whole chapter:\n %q\nvs %q",
			body, bodyBefore)
	}
	if tint := chapterTint(st).fingerprint(); tint == tintBefore {
		t.Error("the mark should have moved to the next note's verse — the tint half must carry the change")
	}
	if _, who, _, _ := appleStickerPush(st, buildChapterPlan(st, appPrefs(), st.Bible)); who == whoBefore {
		t.Error("the pushed who line should have moved to the next count — the native compare would skip the repaint")
	}

	// The same contract when the tap has to Show a stored-minimized note: the
	// store write flips presentation, not the body.
	plan := buildChapterPlan(st, appPrefs(), st.Bible)
	nextID := nextNoteFocusID(st, plan)
	setNoteMinimizedByID(appPrefs(), nextID, true)
	bodyBefore = chapterBodyFingerprint(st)
	advanceNoteFocus(st)
	if body := chapterBodyFingerprint(st); body != bodyBefore {
		t.Errorf("a next-tap onto a minimized note moved the BODY fingerprint:\n %q\nvs %q", body, bodyBefore)
	}
}

// Nothing to advance to: a single placed note pushes next=false and the verb
// is a no-op — no focus churn, no store write, no reposition request.
func TestNextTapSingleNoteIsANoOp(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	n, _ := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
		Book: "John", Chapter: 3, VerseLo: 16, Text: "alone"})
	st := planTestState(t)
	addRecentChapter(st, "John", 3)

	plan := buildChapterPlan(st, appPrefs(), st.Bible)
	if id := nextNoteFocusID(st, plan); id != 0 {
		t.Fatalf("one note: nextNoteFocusID = %d, want 0", id)
	}
	if _, _, _, next := appleStickerPush(st, plan); next {
		t.Error("one note: the push must not offer the next control")
	}
	focusBefore := st.noteFocus
	st.forceReposition = false
	advanceNoteFocus(st)
	if st.NoteID != n.ID || st.noteFocus != focusBefore || st.forceReposition {
		t.Error("advance with nothing to advance to must change nothing")
	}
}

// A mirror-only session note (the store refused the arrival) leads the count
// from outside the plan: the first advance lands on the plan's first note, and
// the wrap can never land back on the mirror-only note (it has no identity to
// focus) — the honest floor, recorded here.
func TestNextTapFromAMirrorOnlyNoteLandsOnThePlan(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	origNow := noteNow
	now := int64(1_700_000_000)
	noteNow = func() int64 { now++; return now }
	defer func() { noteNow = origNow }()

	stored, _ := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
		Book: "John", Chapter: 3, VerseLo: 16, Text: "in the plan"})
	st := planTestState(t)
	addRecentChapter(st, "John", 3)

	// The mirror holds a note the store never accepted: NoteID 0.
	st.ActiveNote = "the session-only note"
	st.NoteID = 0
	st.NoteVerseLo = 17
	st.NoteMinimized = false

	plan := buildChapterPlan(st, appPrefs(), st.Bible)
	if _, who, _, next := appleStickerPush(st, plan); !next || !strings.Contains(who, "1 of 2 on this passage") {
		t.Fatalf("mirror-only lead: want a nextable \"1 of 2\" push, got next=%v who=%q", next, who)
	}
	if id := nextNoteFocusID(st, plan); id != stored.ID {
		t.Fatalf("the advance should land on the plan's first note %d, got %d", stored.ID, id)
	}
	advanceNoteFocus(st)
	if st.NoteID != stored.ID {
		t.Errorf("after the advance the sticker should hold the stored note %d, got %d", stored.ID, st.NoteID)
	}
}

// The rotation is deterministic run-to-run: fifty advances land exactly where
// the arithmetic says — no map order anywhere in the path.
func TestNextTapRotationIsDeterministic(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer deleteAllNotes(appPrefs())

	origNow := noteNow
	now := int64(1_700_000_000)
	noteNow = func() int64 { now++; return now }
	defer func() { noteNow = origNow }()

	st, notes := nextTestState(t)
	order := []uint64{notes[2].ID, notes[1].ID, notes[0].ID}
	for i := 0; i < 50; i++ {
		want := order[i%3]
		if st.NoteID != want {
			t.Fatalf("advance %d: note %d on the sticker, want %d", i, st.NoteID, want)
		}
		advanceNoteFocus(st)
	}
}

// Deleting one note of several must SURFACE THE REST — the verification was
// "all the note pills disappear... until I navigate away and come back":
// dropCurrentNote cleared the mirror and stopped, so the pane showed nothing
// while the store still held two notes. The delete verb now ends on the same
// projection every other verb ends on. Walked to the bottom: each delete
// surfaces the next of the set, and only the LAST leaves the pane bare.
func TestDeleteOfManySurfacesTheRemaining(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer deleteAllNotes(appPrefs())

	origNow := noteNow
	now := int64(1_700_000_000)
	noteNow = func() int64 { now++; return now }
	defer func() { noteNow = origNow }()

	st, notes := nextTestState(t)
	if st.NoteID != notes[2].ID {
		t.Fatal("precondition: the newest note leads")
	}

	dropCurrentNote(st)
	if st.ActiveNote == "" || st.NoteID == 0 {
		t.Fatal("two notes remain and the pane shows NOTHING — the verification, pinned")
	}
	if st.NoteID != notes[1].ID {
		t.Fatalf("the next of the set should surface (note %d), got %d", notes[1].ID, st.NoteID)
	}
	if _, who, _, _ := appleStickerPush(st, buildChapterPlan(st, appPrefs(), st.Bible)); !strings.Contains(who, "of 2 on this passage") {
		t.Errorf("who = %q, want the honest remaining count", who)
	}

	dropCurrentNote(st)
	if st.NoteID != notes[0].ID {
		t.Fatalf("the last remaining note should surface (note %d), got %d", notes[0].ID, st.NoteID)
	}

	dropCurrentNote(st)
	if st.ActiveNote != "" || st.NoteID != 0 {
		t.Error("nothing remains: the pane must finally be bare")
	}
	if n := storedNoteCount(appPrefs()); n != 0 {
		t.Errorf("store should be empty, holds %d", n)
	}
}
