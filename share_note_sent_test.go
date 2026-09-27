package bibletext

// A note the reader sends is shown on the passage at once, as opening it from
// the notes browser shows it (showSentNote, notes_mine.go). Each send here runs
// for real: the composer opened through promptShareNote, the note typed, Share
// tapped (or Return pressed), the message caught at shareTextOut, the record
// written to the test app's own store. The phone path's timers are held
// (holdNoteSheetTimers), as the composer's own tests hold them.

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// sentNoteState is the composer tests' Psalm 23 with Psalm 24 beside it, so
// the reader has somewhere to go, in the default translation.
func sentNoteState() *AppState {
	st := psalm23NoteState()
	ps24 := []Verse{
		{Verse: 1, Text: "The earth is the Lord's, and the fulness thereof; the world, and they that dwell therein."},
		{Verse: 2, Text: "For he hath founded it upon the seas, and established it upon the floods."},
	}
	for i := range ps24 {
		ps24[i].BookName, ps24[i].Book, ps24[i].Chapter = "Psalms", "Psalms", 24
	}
	st.Bible.Verses["Psalms"][24] = ps24
	st.CurrentVersion = defaultVersionID
	st.RecentChapters = []ChapterVisit{{Book: "Psalms", Chapter: 23}}
	return st
}

// catchShares replaces the platform's share sheet with a list the test reads.
func catchShares(t *testing.T) *[]string {
	t.Helper()
	sent := &[]string{}
	prev := shareTextOut
	shareTextOut = func(s string) { *sent = append(*sent, s) }
	t.Cleanup(func() { shareTextOut = prev })
	return sent
}

// composerField is the Fyne note field on an open composer.
func composerField(p *widget.PopUp) *searchKeyEntry {
	var found *searchKeyEntry
	if p == nil {
		return nil
	}
	walkTree(p, func(o fyne.CanvasObject) {
		if e, ok := o.(*searchKeyEntry); ok && found == nil {
			found = e
		}
	})
	return found
}

// myNotes is every Kind=mine record in the app's store.
func myNotes(t *testing.T) []StoredNote {
	t.Helper()
	var out []StoredNote
	for _, n := range readNoteStore(appPrefs()).notes {
		if n.Kind == noteKindMine {
			out = append(out, n)
		}
	}
	return out
}

// styledNoteDrawn builds a Windows and Linux reading pane from st, as a
// rebuild would build it, lays it out and reports what its note band draws:
// present, collapsed to a pill, the byline and the words. It answers what the
// page draws from this state, not whether the pane in a window was ever
// rebuilt to draw it; paneOnScreen answers that.
func styledNoteDrawn(t *testing.T, st *AppState) (present, pill bool, who, body string) {
	t.Helper()
	verses := st.Bible.GetChapter(st.CurrentBook, st.CurrentChapter)
	pane := newStyledReadingPane(st, verses)
	rend, ok := pane.CreateRenderer().(*styledPaneRenderer)
	if !ok {
		t.Fatal("unexpected renderer type")
	}
	rend.Layout(fyne.NewSize(420, 1400))
	g := pane.noteGeom
	if g.present && !g.pill && (rend.noteCard == nil || !inDrawList(rend, rend.noteCard)) {
		t.Error("the note was measured as an open card that is never painted")
	}
	return g.present, g.pill, g.senderTx, strings.Join(g.body, " ")
}

// paneOnScreen reports what the Windows and Linux reading pane that st's
// window holds draws in its note band, as the window laid it out: the pane on
// screen, found in the window's own tree, and nothing built for the question.
// The window must be CreateMainUI's (newAppearanceHarness) showing the styled
// pane, which sentNoteWindow pins on darwin.
func paneOnScreen(t *testing.T, st *AppState) (present, pill bool, who, body string) {
	t.Helper()
	pane := findStyledPane(st.window.Canvas().Content())
	if pane == nil {
		t.Fatal("control: the window holds no Windows and Linux reading pane")
	}
	if pane.Size().Width <= 0 {
		t.Fatal("control: the window never laid its reading pane out")
	}
	g := pane.noteGeom
	if g.present && !g.pill {
		rend, ok := test.WidgetRenderer(pane).(*styledPaneRenderer)
		if !ok || rend.noteCard == nil || !inDrawList(rend, rend.noteCard) {
			t.Error("the pane on screen measured an open card it does not paint")
		}
	}
	return g.present, g.pill, g.senderTx, strings.Join(g.body, " ")
}

// SENDING A NOTE SHOWS IT, on every route to Share: the desktop card's button
// and its Return, the phone sheet's button with the Fyne field (Android), and
// with the native field floated over it (iOS). After the send the note is
// kept, focus names it, the chapter's plan draws it in its own slot, the
// mirror every native sticker reads carries it with its own wash, the view is
// placed on it, and a Windows and Linux pane built from that state draws its
// card, bylined as the reader's. The share goes out first and carries the
// note. That the pane on screen is repainted with it is
// TestTheSentNoteIsOnThePaneOnScreen's to hold: the window here has no
// reading pane.
//
// Over a search result the note takes the page, as a browser tap does: the
// search mark stands aside and the card opens, where leaving the mark up would
// stand the note down to its pill.
//
// Mutations: the composer's send never calls showSentNote (every route fails);
// showSentNote leaves a foreign mark up (the search-result case fails).
func TestSendingANoteShowsItOnThePassage(t *testing.T) {
	const words = "fixture sent alpha: read this with me"
	for _, tc := range []struct {
		name          string
		phone, native bool
		overSearch    bool
		pressReturn   bool
	}{
		{name: "desktop card, Share"},
		{name: "desktop card, Return", pressReturn: true},
		{name: "phone sheet, Fyne field", phone: true},
		{name: "phone sheet, iOS native field", phone: true, native: true},
		{name: "over a search result", overSearch: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := sentNoteState()
			w, h := float32(900), float32(760)
			if tc.phone {
				w, h = 390, 844
			}
			win := noteSheetWindow(t, st, tc.phone, w, h)
			holdNoteSheetTimers(t)
			setNotesEnabled(true)
			sent := catchShares(t)
			typed := ""
			if tc.native {
				nativeNoteField(t, win)
				prev := noteEntryTyped
				noteEntryTyped = func() string { return typed }
				t.Cleanup(func() { noteEntryTyped = prev })
			}
			if tc.overSearch {
				st.setMark(hlSearch, VerseSpan{VersionID: defaultVersionID, Book: "Psalms", Chapter: 23, Lo: 5})
			}

			promptShareNote(st, drawnSelection(t, st, 1, 2), selSpanFromNative(1, 2))
			p := topPopup(t, win)
			field := composerField(p)
			if tc.native {
				// The native field is parked over a slot; the Fyne one is not
				// on the sheet at all.
				if field != nil {
					t.Fatal("control: the iOS sheet must float the native field, not draw the Fyne one")
				}
				typed = words
			} else {
				if field == nil {
					t.Fatalf("control: the composer has no field; texts %v", sheetTexts(p))
				}
				field.SetText(words)
			}
			switch {
			case tc.pressReturn:
				field.OnSubmitted(field.Text)
			default:
				test.Tap(findTreeButton(p.Content, "Share"))
			}

			if len(*sent) != 1 || !strings.HasPrefix((*sent)[0], words) {
				t.Fatalf("control: one share carrying the note must go out; got %q", *sent)
			}
			mine := myNotes(t)
			if len(mine) != 1 {
				t.Fatalf("control: the send must keep one note of the reader's own; got %d", len(mine))
			}
			n := mine[0]
			if st.noteFocus != (noteFocus{set: true, id: n.ID}) {
				t.Errorf("focus is %+v after the send, want the sent note %d", st.noteFocus, n.ID)
			}
			if plan := buildChapterPlan(st, appPrefs(), st.Bible); !plan.HasOwn || plan.Own.Note.ID != n.ID {
				t.Errorf("the chapter's plan does not draw the sent note (HasOwn=%v)", plan.HasOwn)
			}
			if st.ActiveNote != n.Text || st.NoteID != n.ID || st.NoteMinimized {
				t.Errorf("the mirror the native stickers read is %q id %d minimized %v, want the sent note open",
					st.ActiveNote, st.NoteID, st.NoteMinimized)
			}
			if !st.mark.fromNote() || st.mark.At.Lo != 1 || st.mark.At.Hi != 2 {
				t.Errorf("the wash is %v over %d–%d, want the note's own over 1–2",
					st.mark.Origin, st.mark.At.Lo, st.mark.At.Hi)
			}
			if _, _, pill, _ := appleStickerPush(st, buildChapterPlan(st, appPrefs(), st.Bible)); pill {
				t.Error("the native sticker is pushed as a pill; the browser opens the card")
			}
			if !st.forceReposition {
				t.Error("the view is not placed on the note, as every opening verb places it")
			}
			present, pill, who, body := styledNoteDrawn(t, st)
			if !present || pill {
				t.Fatalf("a Windows and Linux pane built from this state draws no open card (present=%v pill=%v)", present, pill)
			}
			if who != senderByline(n) || !strings.Contains(body, "fixture sent alpha") {
				t.Errorf("the card reads %q / %q, want the reader's own byline and words", who, body)
			}
		})
	}
}

// THE PANE ON SCREEN IS REPAINTED WITH IT. The test above holds the state the
// send leaves and what a pane built from that state draws; this one holds
// that the pane the window shows is such a pane. The composer closes before
// the send and nothing after it repaints the reading pane, so without
// showSentNote's own repaint (refreshNoteOnly) the state would be right and
// the reader would see nothing until some unrelated repaint. The window is
// CreateMainUI's, showing the Windows and Linux pane. Android's pane is
// rebuilt by the same call (refreshReadingOnly); the macOS pane's push is
// TestASentNoteIsPushedToTheMacPane's.
//
// Mutation: showSentNote without its refreshNoteOnly.
func TestTheSentNoteIsOnThePaneOnScreen(t *testing.T) {
	h, st := sentNoteWindow(t)
	if present, _, _, _ := paneOnScreen(t, st); present {
		t.Fatal("control: the page must draw no note before the send")
	}

	promptShareNote(st, drawnSelection(t, st, 1, 1), selSpanFromNative(1, 1))
	composerField(h.top()).SetText("fixture on screen alpha")
	test.Tap(findTreeButton(h.top().Content, "Share"))

	assertSentNoteOnScreen(t, st, "fixture on screen alpha")
}

// sentNoteWindow is a real window holding CreateMainUI's tree on John 1,
// showing the Windows and Linux reading pane, with notes on, an empty store
// and the share sheet caught.
func sentNoteWindow(t *testing.T) (*appearanceHarness, *AppState) {
	t.Helper()
	prevPane := useStyledPane
	useStyledPane = func() bool { return true }
	t.Cleanup(func() { useStyledPane = prevPane })
	t.Cleanup(resetStyledWiring)
	h := newAppearanceHarness(t, false)
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	t.Cleanup(func() { deleteAllNotes(appPrefs()) })
	catchShares(t)
	return h, h.state
}

// assertSentNoteOnScreen holds that the one note kept, carrying words, is
// shown: focus names it, the chapter's plan draws it, the mirror the native
// stickers read carries it with its own wash, and the pane on screen draws
// its open card with the reader's byline and words.
func assertSentNoteOnScreen(t *testing.T, st *AppState, words string) {
	t.Helper()
	mine := myNotes(t)
	if len(mine) != 1 || mine[0].Text != words {
		t.Fatalf("control: the send must keep the one note %q; got %d notes", words, len(mine))
	}
	n := mine[0]
	if st.noteFocus != (noteFocus{set: true, id: n.ID}) {
		t.Errorf("focus is %+v, want the sent note %d", st.noteFocus, n.ID)
	}
	if plan := buildChapterPlan(st, appPrefs(), st.Bible); !plan.HasOwn || plan.Own.Note.ID != n.ID {
		t.Error("the chapter's plan does not draw the sent note")
	}
	if st.ActiveNote != n.Text || !st.mark.fromNote() {
		t.Errorf("the mirror is %q with a %v wash, want the sent note with its own", st.ActiveNote, st.mark.Origin)
	}
	present, pill, who, body := paneOnScreen(t, st)
	if !present || pill {
		t.Fatalf("the pane on screen draws no open card (present=%v pill=%v)", present, pill)
	}
	if who != senderByline(n) || !strings.Contains(body, words) {
		t.Errorf("the pane on screen reads %q / %q, want the reader's own byline and words", who, body)
	}
}

// NOTHING IS SHOWN WHEN NOTHING WAS KEPT. A store that stands the write down
// (an unreadable blob is never overwritten) keeps no note, and a Share with
// the field left empty writes none: either way the share still goes out and
// the page is exactly as the reader left it, the search result they arrived on
// still lit and no focus taken.
//
// Held twice over: the composer shows only a kept note, and showSentNote gives
// focus back when the chapter's plan does not draw the record it was handed,
// which a zero record never is. Mutation: both dropped (the composer's kept
// check, showSentNote's zero-record check and its giving focus back), so
// focus is taken for nothing. Either one alone keeps this green.
func TestNothingIsShownWhenTheNoteWasNotKept(t *testing.T) {
	const corrupt = `{"id":1,"k":"received","v":"web","b":"John","c":3,"t":"truncated mid-w`
	for _, tc := range []struct {
		name    string
		words   string
		corrupt bool
	}{
		{name: "the store refused it", words: "fixture refused alpha", corrupt: true},
		{name: "no note was written", words: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := sentNoteState()
			win := noteSheetWindow(t, st, false, 900, 760)
			holdNoteSheetTimers(t)
			setNotesEnabled(true)
			if tc.corrupt {
				appPrefs().SetString(prefNotesStore, corrupt)
			}
			sent := catchShares(t)
			search := VerseSpan{VersionID: defaultVersionID, Book: "Psalms", Chapter: 23, Lo: 5}
			st.setMark(hlSearch, search)

			promptShareNote(st, drawnSelection(t, st, 1, 2), selSpanFromNative(1, 2))
			p := topPopup(t, win)
			composerField(p).SetText(tc.words)
			test.Tap(findTreeButton(p.Content, "Share"))

			if len(*sent) != 1 {
				t.Fatalf("control: the share must still go out once; got %d", len(*sent))
			}
			if tc.corrupt && appPrefs().String(prefNotesStore) != corrupt {
				t.Fatal("control: the unreadable store must be left as it was")
			}
			if len(myNotes(t)) != 0 {
				t.Fatal("control: nothing may be kept here")
			}
			if st.noteFocus != (noteFocus{}) {
				t.Errorf("focus was taken for a note that was not kept: %+v", st.noteFocus)
			}
			if st.mark.Origin != hlSearch || st.mark.At != search {
				t.Errorf("the search result the reader arrived on was put out: %v %+v", st.mark.Origin, st.mark.At)
			}
			if st.ActiveNote != "" {
				t.Errorf("a note is drawn after a send that kept none: %q", st.ActiveNote)
			}
		})
	}
}

// A NOTE IS SHOWN ONLY ON THE PASSAGE ITS WORDS WERE SELECTED ON. The reader
// can be moved while the composer is open (a link arriving does it); the send
// then files the note against the chapter the reader is on, and the page they
// are on must not draw it. The passage is the one the composer first opened
// on, carried through a light/dark reopen. And a show that comes late, after
// the reader has gone, focuses nothing on the chapter they went to.
//
// Mutations: showSentNote without its passage check (the moved and the
// moved-then-reopened cases fail; the late show is also refused by the plan,
// which does not draw a Psalm 23 note on Psalm 24, and fails once the plan's
// giving focus back is dropped too); the light/dark reopen taking the passage
// the reader is on instead of the one it opened on (the reopened case fails).
func TestASentNoteIsNotShownOnAnotherChapter(t *testing.T) {
	t.Run("moved while the composer was open", func(t *testing.T) {
		st := sentNoteState()
		win := noteSheetWindow(t, st, false, 900, 760)
		holdNoteSheetTimers(t)
		setNotesEnabled(true)
		catchShares(t)

		promptShareNote(st, drawnSelection(t, st, 1, 2), selSpanFromNative(1, 2))
		p := topPopup(t, win)
		composerField(p).SetText("fixture moved alpha")
		if !moveChapter(st, 1) || st.CurrentChapter != 24 {
			t.Fatal("control: the reader must now be on Psalm 24")
		}
		test.Tap(findTreeButton(p.Content, "Share"))

		if len(myNotes(t)) != 1 {
			t.Fatal("control: the send keeps the note, as it always has")
		}
		assertNothingFocused(t, st)
	})

	t.Run("moved, then a light/dark reopen", func(t *testing.T) {
		h := newAppearanceHarness(t, false)
		setNotesEnabled(true)
		deleteAllNotes(appPrefs())
		t.Cleanup(func() { deleteAllNotes(appPrefs()) })
		catchShares(t)
		st := h.state
		sel := "1 In the beginning was the Word, and the Word was with God, and the Word was God."

		promptShareNote(st, sel, selSpanFromNative(1, 1))
		if !moveChapter(st, 1) || st.CurrentChapter != 2 {
			t.Fatal("control: the reader must now be on John 2")
		}
		gen := windowRebuildGen
		h.flip()
		p := h.top()
		if windowRebuildGen == gen || composerField(p) == nil {
			t.Fatal("control: the light/dark change must rebuild and bring the composer back")
		}
		composerField(p).SetText("fixture reopened alpha")
		test.Tap(findTreeButton(p.Content, "Share"))

		if len(myNotes(t)) != 1 {
			t.Fatal("control: the send keeps the note, as it always has")
		}
		assertNothingFocused(t, st)
	})

	t.Run("a late show after the reader left", func(t *testing.T) {
		test.NewTempApp(t)
		setNotesEnabled(true)
		st := sentNoteState()
		at := readerPassage(st)
		n, ok := saveMyNote(appPrefs(), StoredNote{VersionID: defaultVersionID, Book: "Psalms", Chapter: 23,
			VerseLo: 1, VerseHi: 2, Text: "fixture late alpha"})
		if !ok {
			t.Fatal("control: the note must be kept")
		}
		moveChapter(st, 1)
		if showSentNote(st, n, at) {
			t.Error("a note written on Psalm 23 was shown on Psalm 24")
		}
		assertNothingFocused(t, st)
	})
}

func assertNothingFocused(t *testing.T, st *AppState) {
	t.Helper()
	if st.noteFocus != (noteFocus{}) {
		t.Errorf("focus was taken on a chapter the note was not written on: %+v", st.noteFocus)
	}
	if plan := buildChapterPlan(st, appPrefs(), st.Bible); plan.HasOwn {
		t.Errorf("the plan for %s %d draws the sent note", st.CurrentBook, st.CurrentChapter)
	}
	if st.ActiveNote != "" {
		t.Errorf("%s %d draws %q", st.CurrentBook, st.CurrentChapter, st.ActiveNote)
	}
}

// A SENT NOTE GOES AWAY ON NAVIGATION, as a note opened from the browser
// does: gone on the next chapter, and not back when the reader returns.
//
// Mutation: navigation no longer resets focus (addRecentChapter).
func TestASentNoteGoesAwayOnNavigation(t *testing.T) {
	st := sentNoteState()
	win := noteSheetWindow(t, st, false, 900, 760)
	holdNoteSheetTimers(t)
	setNotesEnabled(true)
	catchShares(t)

	promptShareNote(st, drawnSelection(t, st, 1, 2), selSpanFromNative(1, 2))
	p := topPopup(t, win)
	composerField(p).SetText("fixture navigated alpha")
	test.Tap(findTreeButton(p.Content, "Share"))
	if st.ActiveNote == "" {
		t.Fatal("control: the sent note must be on the passage first")
	}

	moveChapter(st, 1)
	if st.ActiveNote != "" || st.noteFocus != (noteFocus{}) {
		t.Errorf("the sent note followed the reader to Psalm 24: %q focus %+v", st.ActiveNote, st.noteFocus)
	}
	moveChapter(st, -1)
	if st.CurrentChapter != 23 {
		t.Fatal("control: the reader must be back on Psalm 23")
	}
	if st.ActiveNote != "" {
		t.Errorf("the sent note came back unbidden on a later visit: %q", st.ActiveNote)
	}
	if present, _, _, _ := styledNoteDrawn(t, st); present {
		t.Error("a Windows and Linux pane built on the reader's return still draws the sent note")
	}
}

// A REBUILD RIGHT AFTER THE SEND KEEPS THE NOTE. A light/dark change is the
// likely one (the share sheet in front, the system appearance switched), and
// it rebuilds the whole window: the note is still focused, still in the plan,
// still in the mirror with its wash, and the pane the rebuilt window shows
// draws it, in the new palette.
//
// A light/dark change made while the composer is still open drains it and
// brings it back with what the reader had written (sheet_reopen.go), and with
// the passage it first opened on: Share on the composer that came back shows
// the note as Share on the first would have. (That the passage carried is the
// one it opened on, not the one the reader is on, is
// TestASentNoteIsNotShownOnAnotherChapter's.)
//
// Mutations: rebuildWindow resets note focus before building the new tree
// (the first case fails); the light/dark reopen drops the passage the
// composer opened on (the second fails).
func TestARebuildRightAfterSendingKeepsTheNote(t *testing.T) {
	t.Run("a light/dark change after the send", func(t *testing.T) {
		h, st := sentNoteWindow(t)
		promptShareNote(st, drawnSelection(t, st, 1, 1), selSpanFromNative(1, 1))
		composerField(h.top()).SetText("fixture rebuilt alpha")
		test.Tap(findTreeButton(h.top().Content, "Share"))
		if mine := myNotes(t); len(mine) != 1 || st.noteFocus.id != mine[0].ID {
			t.Fatal("control: the send must keep the note and show it")
		}

		gen, before := windowRebuildGen, st.pal()
		h.flip()
		if windowRebuildGen == gen || st.pal() == before {
			t.Fatal("control: the light/dark change must rebuild the window in the other palette")
		}
		assertSentNoteOnScreen(t, st, "fixture rebuilt alpha")
	})

	t.Run("a light/dark change with the composer open, then Share", func(t *testing.T) {
		h, st := sentNoteWindow(t)
		promptShareNote(st, drawnSelection(t, st, 1, 1), selSpanFromNative(1, 1))
		composerField(h.top()).SetText("fixture reopened alpha")

		gen := windowRebuildGen
		h.flip()
		p := h.top()
		if windowRebuildGen == gen || composerField(p) == nil {
			t.Fatal("control: the light/dark change must rebuild and bring the composer back")
		}
		if got := composerField(p).Text; got != "fixture reopened alpha" {
			t.Fatalf("control: the composer must come back with what was written; got %q", got)
		}
		if len(myNotes(t)) != 0 {
			t.Fatal("control: nothing is kept before Share")
		}
		test.Tap(findTreeButton(p.Content, "Share"))

		assertSentNoteOnScreen(t, st, "fixture reopened alpha")
	})
}

// FOCUS GOES ONLY TO A NOTE THE CHAPTER DRAWS. A note the plan cannot place
// here (its anchor past the chapter's end) is not focused: focus naming an
// absent note falls to the default rule, and that would reopen the friend's
// note the reader had just closed on this chapter.
//
// Mutation: showSentNote keeps the focus it set when the plan does not draw
// the note.
func TestASentNoteTheChapterCannotDrawTakesNoFocus(t *testing.T) {
	test.NewTempApp(t)
	setNotesEnabled(true)
	st := sentNoteState()
	friend, ok := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: defaultVersionID,
		Book: "Psalms", Chapter: 23, VerseLo: 3, Text: "fixture received alpha"})
	if !ok {
		t.Fatal("control: the friend's note must be kept")
	}
	applyNoteForCurrentChapter(st)
	if st.NoteID != friend.ID || st.NoteMinimized {
		t.Fatal("control: the friend's note must be open first")
	}
	st.focusNone() // the reader closes it
	applyNoteForCurrentChapter(st)
	if !st.NoteMinimized {
		t.Fatal("control: closing it must leave it closed")
	}

	beyond, ok := saveMyNote(appPrefs(), StoredNote{VersionID: defaultVersionID, Book: "Psalms",
		Chapter: 23, VerseLo: 40, Text: "fixture beyond alpha"})
	if !ok {
		t.Fatal("control: the note must be kept")
	}
	if showSentNote(st, beyond, readerPassage(st)) {
		t.Error("a note the chapter cannot place was shown")
	}
	if st.noteFocus != (noteFocus{set: true}) {
		t.Errorf("focus moved off the reader's close: %+v", st.noteFocus)
	}
	applyNoteForCurrentChapter(st)
	if !st.NoteMinimized {
		t.Error("the friend's note the reader closed opened again")
	}
}
