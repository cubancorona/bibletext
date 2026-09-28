package bibletext

// A NOTE IS SHARED AND FILED ON THE PASSAGE ITS WORDS WERE SELECTED ON.
//
// The composer holds words selected on one chapter. A link arriving while it
// is open moves the reader (HandleShareLink, applyShareTarget) and leaves the
// sheet up, since the reader may be halfway through a sentence. Share then
// used to read the passage from where the reader had been moved to: the
// citation, the link's chapter and verses and the stored record all named the
// new chapter, and the note was filed there. Each send here runs for real, as
// in share_note_sent_test.go: the composer opened through promptShareNote, a
// real link handled, the note typed, Share tapped, the message caught at
// shareTextOut, the record read back from the test app's own store.

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// linkedAwayState is sentNoteState, loaded, with John 1 beside Psalms, so a
// link can take the reader to another book as well as another chapter, and a
// book with no link path, whose note can only go out as text.
func linkedAwayState() *AppState {
	st := sentNoteState()
	john := []Verse{
		{Verse: 1, Text: "In the beginning was the Word, and the Word was with God, and the Word was God."},
		{Verse: 2, Text: "The same was in the beginning with God."},
	}
	for i := range john {
		john[i].BookName, john[i].Book, john[i].Chapter = "John", "John", 1
	}
	fixture := []Verse{
		{Verse: 1, Text: "Alpha fixture verse one speaks of the quiet field."},
		{Verse: 2, Text: "Beta fixture verse two answers from the far hill."},
	}
	for i := range fixture {
		fixture[i].BookName, fixture[i].Book, fixture[i].Chapter = sluglessBook, sluglessBook, 1
	}
	st.Bible.Books = append(st.Bible.Books, "John", sluglessBook)
	st.Bible.Verses["John"] = map[int][]Verse{1: john}
	st.Bible.Verses[sluglessBook] = map[int][]Verse{1: fixture}
	st.loadPhase = loadReady
	return st
}

// sluglessBook is a book with no link path, so a note written on it goes out
// as a text share (the quote and its citation) rather than as a link.
const sluglessBook = "Fixture Book"

// sheetExcerptAndRef reads the words and the reference the composer shows.
func sheetExcerptAndRef(t *testing.T, p *widget.PopUp, wantRef string) string {
	t.Helper()
	ex, _ := noteSheetExcerpt(t, p)
	found := false
	for _, s := range sheetTexts(p) {
		found = found || s == wantRef
	}
	if !found {
		t.Errorf("the composer's reference is not %q; texts %v", wantRef, sheetTexts(p))
	}
	return ex.text
}

// wantSharedOn holds the one message and the one record a send left to the
// passage book chapter:lo–hi, in the default translation, with the note's
// words. A single verse is stored and linked with no end verse.
func wantSharedOn(t *testing.T, sent []string, words, book string, chapter, lo, hi int) {
	t.Helper()
	web, _ := versionByID(defaultVersionID)
	end := hi
	if hi == lo {
		end = 0
	}
	mine := myNotes(t)
	if len(sent) != 1 || len(mine) != 1 {
		t.Fatalf("control: one share and one kept note; got %d shares %q and %d notes", len(sent), sent, len(mine))
	}
	n := mine[0]
	if n.VersionID != defaultVersionID || n.Book != book || n.Chapter != chapter || n.VerseLo != lo || n.VerseHi != end {
		t.Errorf("the note is filed on %s %s %d:%d–%d, want %s %s %d:%d–%d",
			n.VersionID, n.Book, n.Chapter, n.VerseLo, n.VerseHi, defaultVersionID, book, chapter, lo, end)
	}
	if n.Text != words {
		t.Errorf("the note kept reads %q, want %q", n.Text, words)
	}
	msg := sent[0]
	cite := verseRangeCitation(book, chapter, lo, hi)
	head := words + "\n\n" + cite + " (" + web.Name + ")\n"
	if !strings.HasPrefix(msg, head) {
		t.Fatalf("the message is %q; want the note, then the citation %q in the %s", msg, cite, web.Name)
	}
	url := strings.TrimPrefix(msg, head)
	if want := ShareLinkURLWithNoteNonce(defaultVersionID, book, chapter, lo, hi, words, n.Nonce); url != want {
		t.Errorf("the link is %q, want %q", url, want)
	}
	target, ok := ParseShareLink(url)
	if !ok {
		t.Fatalf("the message's link %q does not parse", url)
	}
	if target.VersionID != defaultVersionID || target.Book != book || target.Chapter != chapter ||
		target.VerseLo != lo || target.VerseHi != end || target.Note != words {
		t.Errorf("the link opens %s %s %d:%d–%d with %q, want %s %s %d:%d–%d with the note",
			target.VersionID, target.Book, target.Chapter, target.VerseLo, target.VerseHi, target.Note,
			defaultVersionID, book, chapter, lo, end)
	}
}

// Every route to Share: the desktop card, the phone sheet with the Fyne field
// (Android), and with the native field floated over it (iOS). Each once with
// the reader left where the composer opened, as bec3f265a shared it, and once
// moved to John 1 by a link while the note is being written.
//
// Mutations: shareVerseLinkWithNote reading the reader's passage again
// (readerPassage(state) in place of at: every moved case fails, on the
// citation, the link and the record); the composer handing the send the
// reader's passage (the same); the record filed on the reader's chapter while
// the link takes at (the record fails); the link's verses read against the
// reader's chapter (the link fails); the text share a link cannot be built for
// read against the reader's chapter (the slugless case fails). The link's
// verses are found in two steps, the words located in the chapter and, where
// they cannot be, the span; reading both against the reader's chapter fails
// here only because this John 1 has no verses 3 and 4. Locating the words in
// the reader's chapter is held by the next test.
func TestANoteIsSharedAndFiledOnThePassageItWasWrittenOn(t *testing.T) {
	const words = "fixture passage alpha: read this with me"
	for _, route := range []struct {
		name          string
		phone, native bool
	}{
		{name: "desktop card"},
		{name: "phone sheet, Fyne field", phone: true},
		{name: "phone sheet, iOS native field", phone: true, native: true},
	} {
		for _, moved := range []bool{false, true} {
			name := route.name + ", reader left where the composer opened"
			if moved {
				name = route.name + ", reader moved by a link"
			}
			t.Run(name, func(t *testing.T) {
				st := linkedAwayState()
				w, h := float32(900), float32(760)
				if route.phone {
					w, h = 390, 844
				}
				win := noteSheetWindow(t, st, route.phone, w, h)
				holdNoteSheetTimers(t)
				setNotesEnabled(true)
				sent := catchShares(t)
				typed := ""
				if route.native {
					nativeNoteField(t, win)
					prev := noteEntryTyped
					noteEntryTyped = func() string { return typed }
					t.Cleanup(func() { noteEntryTyped = prev })
				}

				// Verses 3 and 4, which the chapter the link opens does not
				// have, so every verse read from the wrong chapter shows.
				promptShareNote(st, drawnSelection(t, st, 3, 4), selSpanFromNative(3, 4))
				p := topPopup(t, win)
				quote := sheetExcerptAndRef(t, p, "Psalms 23:3–4")
				if !strings.Contains(quote, "restoreth my soul") || !strings.Contains(quote, "thy staff") {
					t.Fatalf("control: the composer should show Psalm 23:3–4's words; it shows %q", quote)
				}
				if route.native {
					typed = words
				} else {
					composerField(p).SetText(words)
				}

				if moved {
					if !HandleShareLink(st, ShareLinkURL(defaultVersionID, "John", 1, 2, 2)) ||
						st.CurrentBook != "John" || st.CurrentChapter != 1 {
						t.Fatalf("control: the link must take the reader to John 1; they are on %s %d",
							st.CurrentBook, st.CurrentChapter)
					}
					if topPopup(t, win) != p || !p.Visible() {
						t.Fatal("control: the link must leave the composer open")
					}
					// The sheet still shows what Share will send.
					if got := sheetExcerptAndRef(t, p, "Psalms 23:3–4"); got != quote {
						t.Errorf("after the link the composer shows %q, want %q", got, quote)
					}
				}
				test.Tap(findTreeButton(p.Content, "Share"))

				wantSharedOn(t, *sent, words, "Psalms", 23, 3, 4)
				if moved {
					assertNothingFocused(t, st)
				} else if n := myNotes(t); len(n) == 1 && st.noteFocus != (noteFocus{set: true, id: n[0].ID}) {
					t.Errorf("control: on its own passage the sent note is shown; focus is %+v", st.noteFocus)
				}
			})
		}
	}
}

// THE WORDS ARE LOCATED IN THE PASSAGE THEY WERE SELECTED ON, even where the
// chapter the link opens has the same words at another verse, as psalms that
// repeat one another do (Psalm 108:1–5 takes up Psalm 57:7–11). The reader
// selects words within Psalm 23:3, without its verse number, and a link moves
// them to a fixture John 2 whose second verse carries the same words.
// Located in the reader's chapter, they would be found at verse 2, and the
// link and the record would name Psalm 23:2 under a citation of 23:3.
//
// Mutation: linkVersesForSelectionIn locating the words in the reader's
// chapter (normalizeShareSelection in place of normalizeShareSelectionIn)
// fails the link and the record.
func TestANoteKeepsItsVersesWhereTheLinkedChapterRepeatsItsWords(t *testing.T) {
	const words = "fixture repeated alpha"
	const selected = "he leadeth me in the paths of righteousness for his name's sake."
	st := linkedAwayState()
	john2 := []Verse{
		{Verse: 1, Text: "Eta fixture verse one sets the scene."},
		{Verse: 2, Text: "Theta fixture verse two repeats: " + selected},
	}
	for i := range john2 {
		john2[i].BookName, john2[i].Book, john2[i].Chapter = "John", "John", 2
	}
	st.Bible.Verses["John"][2] = john2
	win := noteSheetWindow(t, st, false, 900, 760)
	holdNoteSheetTimers(t)
	setNotesEnabled(true)
	sent := catchShares(t)
	span := selSpanFromNative(3, 3)

	promptShareNote(st, selected, span)
	p := topPopup(t, win)
	sheetExcerptAndRef(t, p, "Psalms 23:3")
	composerField(p).SetText(words)
	if !HandleShareLink(st, ShareLinkURL(defaultVersionID, "John", 2, 1, 1)) ||
		st.CurrentBook != "John" || st.CurrentChapter != 2 {
		t.Fatalf("control: the link must take the reader to John 2; they are on %s %d", st.CurrentBook, st.CurrentChapter)
	}
	// Control: the reader's chapter has the words too, at verse 2.
	if _, lo, hi, _, ok := normalizeShareSelection(st, selected, span); !ok || lo != 2 || hi != 2 {
		t.Fatalf("control: in John 2 the words should be found at verse 2; got %d–%d (%v)", lo, hi, ok)
	}
	test.Tap(findTreeButton(p.Content, "Share"))

	wantSharedOn(t, *sent, words, "Psalms", 23, 3, 3)
}

// A NOTE WHOSE LINK CANNOT BE BUILT goes out as the quote and its citation,
// and those are the passage's too. No shipped book lacks a link path, so the
// passage here is a fixture book that has none.
func TestANoteSharedAsTextQuotesThePassageItWasWrittenOn(t *testing.T) {
	const words = "fixture text alpha"
	st := linkedAwayState()
	st.CurrentBook, st.CurrentChapter = sluglessBook, 1
	st.RecentChapters = []ChapterVisit{{Book: sluglessBook, Chapter: 1}}
	win := noteSheetWindow(t, st, false, 900, 760)
	holdNoteSheetTimers(t)
	setNotesEnabled(true)
	sent := catchShares(t)
	sel, span := drawnSelection(t, st, 1, 2), selSpanFromNative(1, 2)

	promptShareNote(st, sel, span)
	p := topPopup(t, win)
	composerField(p).SetText(words)
	if !HandleShareLink(st, ShareLinkURL(defaultVersionID, "John", 1, 2, 2)) || st.CurrentBook != "John" {
		t.Fatal("control: the link must take the reader to John 1")
	}
	test.Tap(findTreeButton(p.Content, "Share"))

	web, _ := versionByID(defaultVersionID)
	quote, cite := shareQuoteIn(st, sluglessBook, 1, sel, span)
	if cite != verseRangeCitation(sluglessBook, 1, 1, 2) || !strings.Contains(quote, "Alpha fixture") ||
		!strings.Contains(quote, "far hill") {
		t.Fatalf("control: the fixture passage quotes as %q, cited %q", quote, cite)
	}
	if want := composeShareText(quote, cite, web.Name); len(*sent) != 1 || (*sent)[0] != want {
		t.Errorf("the share is %q, want the fixture passage's quote and citation %q", *sent, want)
	}
	if mine := myNotes(t); len(mine) != 1 || mine[0].Book != sluglessBook || mine[0].Chapter != 1 ||
		mine[0].VerseLo != 1 || mine[0].VerseHi != 2 {
		t.Errorf("the note is not filed on %s 1:1–2: %+v", sluglessBook, mine)
	}
}

// THROUGH A LIGHT/DARK REOPEN TOO. The reader is moved by a link and then the
// appearance changes with the composer open: the rebuild brings the composer
// back, and it shows, shares and files the passage it first opened on, not
// the one the reader is now on.
//
// Mutations: shareNoteQuote reading the reader's chapter (the reopened
// composer's excerpt and reference fail); the reopen handing on the reader's
// passage (the send fails); shareVerseLinkWithNote reading the reader's
// passage (the send fails).
func TestANoteReopenedAfterALinkKeepsItsPassage(t *testing.T) {
	const words = "fixture reopened passage alpha"
	h := newAppearanceHarness(t, false)
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	t.Cleanup(func() { deleteAllNotes(appPrefs()) })
	sent := catchShares(t)
	st := h.state
	sel := "1 In the beginning was the Word, and the Word was with God, and the Word was God."

	promptShareNote(st, sel, selSpanFromNative(1, 1))
	quote := sheetExcerptAndRef(t, h.top(), "John 1:1")
	// What HandleShareLink does with a link once the app has loaded.
	applyShareTarget(st, ShareTarget{VersionID: defaultVersionID, Book: "Psalms", Chapter: 23, VerseLo: 1, VerseHi: 1})
	if st.CurrentBook != "Psalms" || st.CurrentChapter != 23 {
		t.Fatalf("control: the link must take the reader to Psalm 23; they are on %s %d", st.CurrentBook, st.CurrentChapter)
	}
	gen := windowRebuildGen
	h.flip()
	p := h.top()
	if windowRebuildGen == gen || composerField(p) == nil {
		t.Fatal("control: the light/dark change must rebuild and bring the composer back")
	}
	if got := sheetExcerptAndRef(t, p, "John 1:1"); got != quote {
		t.Errorf("the reopened composer shows %q, want %q", got, quote)
	}
	composerField(p).SetText(words)
	test.Tap(findTreeButton(p.Content, "Share"))

	wantSharedOn(t, *sent, words, "John", 1, 1, 1)
}
