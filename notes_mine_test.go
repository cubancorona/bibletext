package bibletext

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
)

// Your own notes survive being sent, which they did not before: "Share with
// note" built a URL, handed it to the share sheet, and kept nothing.
func TestMyNoteSurvivesARoundTrip(t *testing.T) {
	p := newNotePrefs()
	saveMyNote(p, StoredNote{VersionID: "web", Book: "Psalms", Chapter: 34,
		VerseLo: 18, Text: "thinking of you today"})

	mine, ok := readMyNotes(p)
	if !ok {
		t.Fatal("the store did not read back")
	}
	if len(mine) != 1 {
		t.Fatalf("expected one note of my own, got %d", len(mine))
	}
	if mine[0].Text != "thinking of you today" || mine[0].Kind != noteKindMine {
		t.Errorf("came back changed: %+v", mine[0])
	}
	if mine[0].Received == 0 {
		t.Error("a note you sent should be stamped with when you sent it")
	}
}

// Own notes and a friend's note share ONE store now, with no passage key to
// collide on — so sending a note on a chapter where a friend's note lives can
// never overwrite their message, and the reading page still shows theirs.
func TestMyNoteCannotOverwriteAFriendsNote(t *testing.T) {
	p := newNotePrefs()
	addNote(p, StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "John", Chapter: 3, VerseLo: 16,
		Text: "a friend sent me this"})
	saveMyNote(p, StoredNote{VersionID: "web", Book: "John", Chapter: 3, VerseLo: 16,
		Text: "and I sent one back"})

	got, ok := noteForChapter(p, "web", "John", 3, nil)
	if !ok {
		t.Fatal("the friend's note is gone entirely")
	}
	if got.Text != "a friend sent me this" {
		t.Errorf("my note overwrote theirs: %q", got.Text)
	}
	if got.Kind == noteKindMine {
		t.Error("the note on the passage must be the one that was sent to me")
	}
	if mine, _ := readMyNotes(p); len(mine) != 1 {
		t.Errorf("my own note was not kept: %d", len(mine))
	}
}

// ...and the second reason: two of your own notes on one passage are two notes.
// A keyed store would have kept only the newer, which is why there is no key.
func TestTwoOfMyNotesOnOnePassageBothSurvive(t *testing.T) {
	p := newNotePrefs()
	saveMyNote(p, StoredNote{VersionID: "web", Book: "John", Chapter: 3, VerseLo: 16, Text: "first thought"})
	saveMyNote(p, StoredNote{VersionID: "web", Book: "John", Chapter: 3, VerseLo: 16, Text: "second thought"})

	mine, _ := readMyNotes(p)
	if len(mine) != 2 {
		t.Fatalf("expected both notes, got %d", len(mine))
	}
}

// "Duplicate is measured by everything being identical" (owner). Re-sharing the
// same words on the same passage is the same note, and must not stack up. The
// timestamp is deliberately NOT part of that comparison — the sending device
// stamps it, so including it would make every re-share a new note.
func TestResharingTheSameNoteDoesNotDuplicateIt(t *testing.T) {
	p := newNotePrefs()
	n := StoredNote{VersionID: "web", Book: "Romans", Chapter: 8, VerseLo: 28, Text: "all things"}
	saveMyNote(p, n)
	n.Received = 0 // a fresh send: the clock has moved on
	saveMyNote(p, n)

	if mine, _ := readMyNotes(p); len(mine) != 1 {
		t.Errorf("re-sharing the same words produced %d notes, want 1", len(mine))
	}
	// ...but different words on the same verse are a different note.
	saveMyNote(p, StoredNote{VersionID: "web", Book: "Romans", Chapter: 8, VerseLo: 28,
		Text: "and this one too"})
	if mine, _ := readMyNotes(p); len(mine) != 2 {
		t.Errorf("a different note on the same verse should be kept: got %d", len(mine))
	}
}

// Own notes are STORED but never drawn in the scripture text (owner directive:
// "no display in text for now"). The reading page derives Kind=received notes
// alone, so this is structural rather than a rule anyone has to remember.
func TestMyNotesNeverReachTheReadingPage(t *testing.T) {
	p := newNotePrefs()
	saveMyNote(p, StoredNote{VersionID: "web", Book: "Mark", Chapter: 4, VerseLo: 39,
		Text: "peace, be still"})

	if _, ok := noteForChapter(p, "web", "Mark", 4, nil); ok {
		t.Error("a note you sent appeared on the reading page")
	}
	// Nor by following the passage into another translation.
	if _, ok := noteForChapter(p, "bsb", "Mark", 4, nil); ok {
		t.Error("a note you sent followed the passage into another translation")
	}
	// It is still yours, and still in the list.
	if mine, _ := readMyNotes(p); len(mine) != 1 {
		t.Error("the note was not kept")
	}
}

// One control, one store. To the reader there is one thing called "your
// notes"; a Delete all that emptied half of them would be lying.
func TestDeleteAllClearsYourOwnNotesToo(t *testing.T) {
	p := newNotePrefs()
	addNote(p, StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "John", Chapter: 3, Text: "theirs"})
	saveMyNote(p, StoredNote{VersionID: "web", Book: "Psalms", Chapter: 23, Text: "mine"})

	deleteAllNotes(p)

	if storedNoteCount(p) != 0 {
		t.Error("notes survived Delete all")
	}
	if mine, _ := readMyNotes(p); len(mine) != 0 {
		t.Errorf("your own notes survived Delete all: %d left", len(mine))
	}
}

// The browser shows both, told apart by the byline rather than by living on
// separate screens — a scrapbook records an exchange.
func TestBrowsingShowsBothAndNamesWhoTheyAreFrom(t *testing.T) {
	p := newNotePrefs()
	addNote(p, StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "John", Chapter: 3, Text: "theirs", Received: 100})
	saveMyNote(p, StoredNote{VersionID: "web", Book: "Psalms", Chapter: 23, Text: "mine", Received: 200})

	all := allNotesForBrowsing(p)
	if len(all) != 2 {
		t.Fatalf("expected both notes in one list, got %d", len(all))
	}
	var sawMine, sawTheirs bool
	for _, n := range all {
		switch noteByline(n) {
		case "From you":
			sawMine = true
			if n.Kind != noteKindMine {
				t.Error("byline says yours but the note does not")
			}
		case "From Friend":
			sawTheirs = true
		default:
			t.Errorf("unattributed note: %+v", n)
		}
	}
	if !sawMine || !sawTheirs {
		t.Errorf("both must be present and attributed (mine=%v theirs=%v)", sawMine, sawTheirs)
	}
}

func TestWhoFilterSeparatesThem(t *testing.T) {
	notes := []StoredNote{
		{Kind: noteKindReceived, Book: "John", Chapter: 3, Text: "theirs"},
		{Kind: noteKindMine, Book: "Psalms", Chapter: 23, Text: "mine"},
	}
	if got := filterNotesByWho(notes, whoAnyone); len(got) != 2 {
		t.Errorf("Everyone = %d, want 2", len(got))
	}
	if got := filterNotesByWho(notes, whoOthers); len(got) != 1 || got[0].Kind == noteKindMine {
		t.Errorf("From others = %+v", got)
	}
	if got := filterNotesByWho(notes, whoMe); len(got) != 1 || got[0].Kind != noteKindMine {
		t.Errorf("From you = %+v", got)
	}
	// The filter must not scribble on its input: it is called on the shared,
	// sorted list and a caller behind it would otherwise see a truncated set.
	if len(notes) != 2 || notes[1].Text != "mine" {
		t.Errorf("the source list was mutated: %+v", notes)
	}
}

// A store that will not read must be left ALONE by your own sends too — same
// stand-down every writer honours (notes_store_guard_test.go pins the rest).
func TestAnUnreadableStoreIsNotOverwrittenBySaveMyNote(t *testing.T) {
	p := newNotePrefs()
	p.m[prefNotesStore] = "{not json"

	if _, ok := readMyNotes(p); ok {
		t.Error("garbage reported as readable")
	}
	saveMyNote(p, StoredNote{VersionID: "web", Book: "John", Chapter: 3, Text: "new"})
	if p.m[prefNotesStore] != "{not json" {
		t.Errorf("the unreadable blob was overwritten: %q", p.m[prefNotesStore])
	}
}

// The bubble has a TAIL, which is what makes it read as somebody speaking
// rather than as a card (owner). Its geometry mirrors the native sticker.
func TestNoteBubbleHasATail(t *testing.T) {
	pal := lightPalette
	b := noteBubble("a message", pal)
	inner, ok := b.(*fyne.Container)
	if !ok || len(inner.Objects) != 2 {
		t.Fatalf("expected a bubble and a tail, got %#v", b)
	}
	if _, isImg := inner.Objects[1].(*canvas.Image); !isImg {
		t.Fatalf("the tail is not drawn: %#v", inner.Objects[1])
	}
	// The tail adds its depth to the bubble's height, and sits inset from the
	// left edge pointing down at the passage.
	b.Resize(fyne.NewSize(240, b.MinSize().Height))
	tail := inner.Objects[1]
	if got := tail.Position().X; got != noteTailInset {
		t.Errorf("tail X = %v, want %v", got, noteTailInset)
	}
	card := inner.Objects[0]
	if tail.Position().Y >= card.Position().Y+card.Size().Height {
		t.Error("the tail must overlap the bubble's border, not sit below it")
	}
	// BY ENOUGH TO HIDE IT. "Some overlap" was one point, and one point was not
	// enough: the card's 1pt bottom border lands on a fractional device pixel at
	// 2x and 3x, so magnified it still showed as a hairline lid straight across
	// the tail's mouth — the tail read as a triangle stuck under a closed box
	// rather than as part of the bubble (owner, comparing it with the reading
	// pane). The amount is the thing to pin, not the direction.
	cardBottom := card.Position().Y + card.Size().Height
	if got := cardBottom - tail.Position().Y; got < noteTailLidOverlap {
		t.Errorf("the tail covers the card's border by %.1f, want at least %.1f — "+
			"less and the border shows through as a lid across the tail's mouth",
			got, float32(noteTailLidOverlap))
	}
	if b.MinSize().Height <= card.MinSize().Height {
		t.Error("the bubble's height must make room for the tail")
	}
}

// The translation rides in the heading, not under the bubble.
func TestVersionAbbreviations(t *testing.T) {
	for id, want := range map[string]string{
		"web": "WEB", "bsb": "BSB", "webc": "WEBC", "nkjv": "NKJV", "": "",
	} {
		if got := noteVersionAbbrev(id); got != want {
			t.Errorf("noteVersionAbbrev(%q) = %q, want %q", id, got, want)
		}
	}
	// An id the registry has never heard of still says SOMETHING — a note is
	// from somewhere, and saying so beats implying it came from the
	// translation on screen.
	if got := noteVersionAbbrev("esv"); got != "ESV" {
		t.Errorf("unknown id gave %q", got)
	}
}
