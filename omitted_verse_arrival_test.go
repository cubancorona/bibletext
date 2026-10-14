package bibletext

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// A link names a verse in the sender's edition, and the reader's may omit
// it: the NKJV prints Matthew 17:21, the BSB does not, and draws the hole as
// a "[21]" mark before verse 22's number (gapsBefore). Arriving at 21, every
// pane found nothing to land on and opened the chapter at the top, Android
// even for a link to 21-22 whose verse 22 the others lit. The arrival lands
// on the verse after the hole on every surface; Android's push sends the
// verse the chrome resolves (android_arrival_scroll_test.go holds its source
// to it). Whether the mark itself lights is a question of its own; nothing
// here lights it.
func omittedVerseLinkState(t *testing.T, lo, hi int) (*AppState, []Verse) {
	t.Helper()
	v := func(n int, text string) Verse {
		return Verse{BookName: "Matthew", Book: "Matthew", Chapter: 17, Verse: n, Text: text}
	}
	verses := []Verse{
		v(19, "Then the friends came to him privately and asked why they could not do it."),
		v(20, "He answered that their trust was small, and told them what trust can do."),
		v(22, "When they gathered together in the hill country, he spoke to them again."),
		v(23, "And they were filled with grief at what he told them."),
	}
	st := &AppState{
		Bible:          &BibleData{Books: []string{"Matthew"}, Verses: map[string]map[int][]Verse{"Matthew": {17: verses}}},
		CurrentVersion: "bsb", CurrentBook: "Matthew", CurrentChapter: 17,
	}
	st.forceReposition = true
	st.setMark(hlLinkSpan, VerseSpan{VersionID: "nkjv", Book: "Matthew", Chapter: 17, Lo: lo, Hi: hi})
	if sp, ok := st.markHere(); !ok || sp.Lo != lo || sp.Hi != hi {
		t.Fatalf("fixture: the link's mark is %+v (ok %v), want %d-%d", sp, ok, lo, hi)
	}
	return st, verses
}

func TestALinkToAnOmittedVerseArrivesAtTheVerseAfterTheHole(t *testing.T) {
	for _, c := range []struct {
		lo, hi, want int
	}{
		{21, 21, 22}, // the omitted verse alone
		{21, 22, 22}, // a range that starts at the hole
		{22, 22, 22}, // the control: a verse the chapter has
		{99, 99, 99}, // past the chapter's last verse: nowhere to land, left as named
	} {
		st, verses := omittedVerseLinkState(t, c.lo, c.hi)
		cls, got := chapterNoteArrival(st, noteChrome{}, verses, nil, false, false, true)
		if cls != arriveVerse || got != c.want {
			t.Errorf("a link to %d-%d arrives as %v at verse %d, want a verse arrival at %d", c.lo, c.hi, cls, got, c.want)
		}
	}
}

// A NOTE sent on the omitted verse is the other case. The card's own verse
// is missing from the page too, so the canvas pane opens its band above the
// first verse (noteAnchorVerse), the bridge parks the sticker at the top of
// the text, the Apple panes reserve it at the top when the mark lights
// nothing, and the website parks it above the text and lands on it there.
// Moving the arrival on to verse 22 scrolled that card out of view, so a
// note whose own verse the page omits keeps the arrival it had: none, and
// the chapter opens at its top with the card in view. A note on a verse the
// page has is the control.
func TestANoteOnAnOmittedVerseKeepsItsCardInView(t *testing.T) {
	note := func(anchor int) noteChrome {
		return noteChrome{Text: "Invented note text.", Who: "Invented sender", Anchor: anchor}
	}
	for _, c := range []struct {
		name           string
		lo, hi, anchor int
		cls            noteArrival
		verse          int
	}{
		{"a note on the omitted verse", 21, 21, 21, arriveNothing, 0},
		{"a note on a range that starts at the hole", 21, 22, 21, arriveNothing, 0},
		{"a note anchored at the verse after the hole", 21, 22, 22, arriveBand, 22},
		{"a note on a verse the page has", 22, 22, 22, arriveBand, 22},
	} {
		st, verses := omittedVerseLinkState(t, c.lo, c.hi)
		cls, got := chapterNoteArrival(st, note(c.anchor), verses, nil, false, false, true)
		if cls != c.cls || got != c.verse {
			t.Errorf("%s (link %d-%d): the arrival is %v at verse %d, want %v at %d",
				c.name, c.lo, c.hi, cls, got, c.cls, c.verse)
		}
		// Android pushes this verse to the bridge beside the class, so the two
		// have to be one answer.
		if v := arrivalVerseOnPage(note(c.anchor), verses, c.lo); v != c.verse {
			t.Errorf("%s (link %d-%d): Android would push verse %d, want %d", c.name, c.lo, c.hi, v, c.verse)
		}
	}
	// With no note the arrival moves on past the hole, as the test above holds.
	_, verses := omittedVerseLinkState(t, 21, 21)
	if v := arrivalVerseOnPage(noteChrome{}, verses, 21); v != 22 {
		t.Errorf("a link with no note: Android would push verse %d, want 22", v)
	}
}

// The canvas pane's half of the case above: with the card parked above the
// first verse, the pane places nothing, so the chapter opens at its top
// where the card is.
func TestTheCanvasPaneOpensANoteOnAnOmittedVerseAtItsCard(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	for _, c := range []struct{ lo, hi int }{{21, 21}, {21, 22}} {
		st, verses := omittedVerseLinkState(t, c.lo, c.hi)
		p := newStyledReadingPane(st, verses)
		p.note = noteChrome{Text: "Invented note text.", Who: "Invented sender", Anchor: 21}
		p.note.Arrival, p.note.ArrivalVerse = chapterNoteArrival(st, p.note, verses, nil, false, false, true)
		p.Resize(fyne.NewSize(560, 700))
		if first := verses[0].Verse; p.noteAnchorVerse() != first {
			t.Fatalf("fixture: the card opens above verse %d, want the first verse %d", p.noteAnchorVerse(), first)
		}
		if p.highlightOwnsScroll() {
			t.Errorf("a note on %d-%d: the canvas pane scrolls to y %.1f, away from the card at the top",
				c.lo, c.hi, p.highlightY())
		}
	}
}

// Windows and Linux: the canvas pane scrolled only to a lit line, and a
// link to the omitted verse lights none. It now lands on the verse the
// arrival names, as the Apple panes fall back to it.
func TestTheCanvasPaneLandsALinkToAnOmittedVerseOnTheVerseAfterIt(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	for _, c := range []struct {
		lo, hi, land int
	}{{21, 21, 22}, {21, 22, 22}, {20, 20, 20}} {
		st, verses := omittedVerseLinkState(t, c.lo, c.hi)
		p := newStyledReadingPane(st, verses)
		p.Resize(fyne.NewSize(560, 700))
		if !p.highlightOwnsScroll() {
			t.Errorf("a link to %d-%d: the canvas pane places nothing (first lit line %d)", c.lo, c.hi, p.highlightFirstLine())
			continue
		}
		want, ok := p.yForVerse(c.land)
		if !ok {
			t.Fatalf("verse %d has no line", c.land)
		}
		if got := p.highlightY(); got != want {
			t.Errorf("a link to %d-%d lands at y %.1f, want verse %d's line at %.1f", c.lo, c.hi, got, c.land, want)
		}
	}
}
