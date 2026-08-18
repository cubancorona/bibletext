package bibletext

// What a reader is TOLD when a link's note payload cannot be rendered
// (docs/NOTE_WIRE_FORMAT.md rule 5): the passage always opens, the two
// messages appear in the note's place, attributed to nobody, with no call to
// action and no link — and never silently.

import (
	"encoding/base64"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func noticeState() *AppState {
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	return &AppState{
		Bible: bd, CurrentBook: "Genesis", CurrentChapter: 1,
		CurrentVersion: "web", loadPhase: loadReady,
		loadedVersions: map[string]*BibleData{"web": bd},
	}
}

// newerPayload is a payload from a future BibleText: byte 0 in 'A'-'Z'.
func newerPayload() string {
	return base64.RawURLEncoding.EncodeToString([]byte{'B', 1, 2, 3, 4})
}

func TestArrivalTellsTheReaderAboutAnUnreadableNote(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())

	for _, tc := range []struct {
		name, fragment, want string
	}{
		{"damaged payload", "#v16&n=!!!not-base64!!!", noteDamagedMessage},
		{"empty payload", "#v16&n=", noteDamagedMessage},
		{"newer format", "#v16&n=" + newerPayload(), noteNewerFormatMessage},
	} {
		st := noticeState()
		target, ok := ParseShareLink("https://bibletext.co.uk/web/john/3/" + tc.fragment)
		if !ok {
			t.Fatalf("%s: link did not parse", tc.name)
		}
		if target.Note != "" {
			t.Fatalf("%s: an unreadable payload produced note text %q", tc.name, target.Note)
		}
		applyShareTarget(st, target)

		// The passage ALWAYS opens.
		if st.CurrentBook != "John" || st.CurrentChapter != 3 {
			t.Errorf("%s: passage did not open: %s %d", tc.name, st.CurrentBook, st.CurrentChapter)
		}
		// The reader is told, in the note's place.
		if st.NoteNotice != tc.want {
			t.Errorf("%s: notice %q, want %q", tc.name, st.NoteNotice, tc.want)
		}
		// Nothing is rendered as a note and nothing is stored.
		if st.ActiveNote != "" {
			t.Errorf("%s: ActiveNote = %q, want none", tc.name, st.ActiveNote)
		}
		if n := storedNoteCount(appPrefs()); n != 0 {
			t.Errorf("%s: %d notes stored from an unreadable payload", tc.name, n)
		}
		// And the next navigation clears it — the notice belongs to the arrival.
		addRecentChapter(st, "Genesis", 1)
		if st.NoteNotice != "" {
			t.Errorf("%s: notice survived a navigation: %q", tc.name, st.NoteNotice)
		}
	}
}

func TestArrivalWithAReadableNoteRaisesNoNotice(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())

	st := noticeState()
	url := ShareLinkURLWithNote("web", "John", 3, 16, 18, "an ordinary note")
	target, ok := ParseShareLink(url)
	if !ok || target.Note != "an ordinary note" || target.NoteOutcome != NoteOutcomeOK {
		t.Fatalf("our own link did not decode: %+v ok=%v", target, ok)
	}
	applyShareTarget(st, target)
	if st.NoteNotice != "" {
		t.Errorf("a readable note raised a notice: %q", st.NoteNotice)
	}
	if st.ActiveNote != "an ordinary note" {
		t.Errorf("note not shown: %q", st.ActiveNote)
	}
}

// With notes switched off the app has nothing to say about a payload either
// way: the passage opens plainly, exactly as for a decoded note.
func TestNoNoticeWhenNotesAreOff(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(false)
	defer setNotesEnabled(true)

	st := noticeState()
	target, _ := ParseShareLink("https://bibletext.co.uk/web/john/3/#v16&n=!!!broken!!!")
	applyShareTarget(st, target)
	if st.NoteNotice != "" {
		t.Errorf("notice raised with notes off: %q", st.NoteNotice)
	}
	if st.CurrentBook != "John" || st.CurrentChapter != 3 {
		t.Errorf("passage did not open: %s %d", st.CurrentBook, st.CurrentChapter)
	}
}

// The banner renders the notice bare: no byline, no buttons, no link — and it
// renders on EVERY platform, including the ones whose pane draws real notes
// natively, because the native sticker only ever draws a decoded note.
func TestNoteNoticeBannerIsBareOfChromeAndActions(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)

	st := noticeState()
	st.NoteNotice = noteDamagedMessage

	// Deliberately NOT pinning nativeNoteSticker to false: the host is darwin,
	// where real notes suppress the banner — the notice must show anyway.
	b := buildNoteBanner(st)
	if b == nil {
		t.Fatal("no banner for a notice")
	}
	texts := treeTexts(b)
	found := false
	for _, s := range texts {
		if strings.Contains(s, noteDamagedMessage) {
			found = true
		}
		if strings.Contains(s, "Note from") {
			t.Errorf("the notice is attributed: %q — it must come from nobody", s)
		}
	}
	if !found {
		t.Errorf("notice text missing; got %v", texts)
	}
	buttons, links := 0, 0
	walkTree(b, func(n fyne.CanvasObject) {
		switch n.(type) {
		case *widget.Button:
			buttons++
		case *widget.Hyperlink:
			links++
		}
	})
	if buttons != 0 || links != 0 {
		t.Errorf("the notice carries %d button(s) and %d link(s) — no call to action, no link",
			buttons, links)
	}
}

// A payload whose 'v' record names a translation the PATH could not (the path
// is lossy: an id outside linkPathVersionIDs falls back to web) must be stored
// under the WIRE's translation, and the live mirror must address the same key
// — or Hide and Delete would miss it.
func TestWireVersionIsAuthoritativeForStorage(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	st := noticeState()
	// A hand-built link: path says web, the payload's own record says nkjv —
	// which is exactly what ShareLinkURLWithNote emits for a sender whose
	// translation the path cannot carry.
	payload := EncodeNoteWire(NoteWire{
		Text: "from the nkjv", Version: "nkjv", Book: "John", Chapter: 3, VerseLo: 16,
	})
	target, ok := ParseShareLink("https://bibletext.co.uk/web/john/3/#v16&n=" + payload)
	if !ok {
		t.Fatal("link did not parse")
	}
	if target.NoteVersion != "nkjv" || target.NoteBook != "John" || target.NoteChapter != 3 {
		t.Fatalf("wire anchor lost in parsing: %+v", target)
	}
	applyShareTarget(st, target)

	if st.ActiveNote != "from the nkjv" {
		t.Fatalf("note not shown: %q", st.ActiveNote)
	}
	stored, ok := findStoredNote(appPrefs(), "nkjv", "John", 3)
	if !ok {
		t.Errorf("note not stored under the wire's translation; store: %v", allNotesForBrowsing(appPrefs()))
	}
	if st.NoteID != stored.ID {
		t.Errorf("live mirror addresses id %d, want the stored note's %d", st.NoteID, stored.ID)
	}
	if _, ok := findStoredNote(appPrefs(), "web", "John", 3); ok {
		t.Errorf("note ALSO stored under the lossy path translation")
	}
}

// Tapping a note's own link is the Show verb: a stored minimize does not make
// the tap look broken.
//
// Owner-reported from a real device: the top dev-links case "does not expand
// the note when clicking" — the note had been minimized in earlier testing,
// the dedup preserved the flag, and the re-arrival honoured it (the S7
// decision). That reading of N5 was wrong: the rule is that nothing
// AUTO-expands, and a deliberate tap on the link that IS this note is the
// most explicit naming of it the reader has — the same act as tapping its
// chip, which un-minimizes.
func TestReopeningANotesOwnLinkExpandsIt(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	st := noticeState()
	url := ShareLinkURLWithNote("web", "Genesis", 1, 1, 1, "tap me twice")
	target, ok := ParseShareLink(url)
	if !ok {
		t.Fatal("link did not parse")
	}
	applyShareTarget(st, target)
	if st.ActiveNote == "" || st.NoteMinimized {
		t.Fatalf("precondition: first arrival should show the note open (%q min=%v)",
			st.ActiveNote, st.NoteMinimized)
	}
	hideCurrentNote(st) // the reader minimizes it...

	target2, _ := ParseShareLink(url)
	applyShareTarget(st, target2) // ...and later taps the same link again

	if st.NoteMinimized || st.ActiveNote == "" {
		t.Errorf("re-tapping the note's own link left it minimized (%q min=%v) — "+
			"the tap reads as broken", st.ActiveNote, st.NoteMinimized)
	}
	if n, ok := findStoredNote(appPrefs(), "web", "Genesis", 1); !ok || n.Minimized {
		t.Error("the stored minimize must be CLEARED, as the chip tap clears it")
	}
}
