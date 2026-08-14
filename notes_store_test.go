package bibletext

import (
	"strings"
	"testing"
)

// fakePrefs is the in-memory prefStore the reading-state tests already rely on
// this package having; notes use the same seam.
type notePrefs struct{ m map[string]string }

func newNotePrefs() *notePrefs { return &notePrefs{m: map[string]string{}} }

func (p *notePrefs) String(k string) string { return p.m[k] }
func (p *notePrefs) SetString(k, v string)  { p.m[k] = v }
func (p *notePrefs) StringWithFallback(k, fallback string) string {
	if v, ok := p.m[k]; ok {
		return v
	}
	return fallback
}

func TestNoteSurvivesARoundTrip(t *testing.T) {
	p := newNotePrefs()
	n := SharedNote{VersionID: "web", Book: "John", Chapter: 3, VerseLo: 16, VerseHi: 18,
		Text: "Thinking of you — this one carried me. 🙏"}
	saveNote(p, n)

	got, ok := loadNote(p, "web", "John", 3)
	if !ok {
		t.Fatal("the note did not come back")
	}
	if got.Text != n.Text || got.VerseLo != 16 || got.VerseHi != 18 {
		t.Errorf("came back changed: %+v", got)
	}
	if _, ok := loadNote(p, "web", "John", 4); ok {
		t.Error("a note leaked onto the next chapter")
	}
	// A note FOLLOWS the passage into another translation. It used to be
	// confined to the translation it arrived in, on the reasoning that a remark
	// is about particular wording — but the reader meets that rule as a note
	// that silently disappears when they change translation, and it disappears
	// in the ordinary case, not an exotic one: two people sharing a link often
	// read different translations, and a link shared FROM a licensed translation
	// comes back naming a published one, so it was the sender's own note that
	// vanished (owner-reported, NKJV). John 3:16 is John 3:16 in both.
	got2, ok := loadNote(p, "bsb", "John", 3)
	if !ok {
		t.Fatal("the note did not follow the passage into the other translation")
	}
	if got2.Text != n.Text {
		t.Errorf("the note changed on the way across: %q", got2.Text)
	}
	if got2.VersionID != "bsb" {
		t.Errorf("a note handed to the bsb reader still claims %q", got2.VersionID)
	}
}

// ...but only where the passage genuinely corresponds. Greek Esther is a
// different book from Esther, not a renumbering, so a note on one says nothing
// about the other — MapVerse calls that incommensurable and the note must stay
// where it is rather than being planted on unrelated text.
func TestANoteDoesNotFollowAnIncommensurablePassage(t *testing.T) {
	p := newNotePrefs()
	saveNote(p, SharedNote{VersionID: "web", Book: "Esther", Chapter: 4, VerseLo: 1,
		Text: "for such a time as this"})

	if _, ok := loadNote(p, "webc", "Esther", 4); ok {
		t.Error("a note crossed into Greek Esther, where its verse numbers mean something else")
	}
}

// Minimize must be remembered. If it is not, the note reappears on the reader's
// next visit as though they never touched it — the exact bug that makes people
// stop trusting a dismiss.
func TestMinimizeIsRemembered(t *testing.T) {
	p := newNotePrefs()
	saveNote(p, SharedNote{VersionID: "web", Book: "Psalms", Chapter: 23, VerseLo: 1, Text: "slowly"})

	setNoteMinimized(p, "web", "Psalms", 23, true)
	if n, _ := loadNote(p, "web", "Psalms", 23); !n.Minimized {
		t.Error("minimize was not stored")
	}
	setNoteMinimized(p, "web", "Psalms", 23, false)
	if n, _ := loadNote(p, "web", "Psalms", 23); n.Minimized {
		t.Error("restore was not stored")
	}
	// And the text survives both.
	if n, _ := loadNote(p, "web", "Psalms", 23); n.Text != "slowly" {
		t.Errorf("text lost through minimize/restore: %q", n.Text)
	}
}

func TestDeleteIsForGood(t *testing.T) {
	p := newNotePrefs()
	saveNote(p, SharedNote{VersionID: "web", Book: "John", Chapter: 3, Text: "x"})
	deleteNote(p, "web", "John", 3)
	if _, ok := loadNote(p, "web", "John", 3); ok {
		t.Error("the note came back after delete")
	}
}

// A second note on the same passage replaces the first, rather than stacking
// two bubbles on one paragraph.
func TestSecondNoteReplaces(t *testing.T) {
	p := newNotePrefs()
	saveNote(p, SharedNote{VersionID: "web", Book: "John", Chapter: 3, Text: "first"})
	saveNote(p, SharedNote{VersionID: "web", Book: "John", Chapter: 3, Text: "second"})
	n, _ := loadNote(p, "web", "John", 3)
	if n.Text != "second" {
		t.Errorf("expected the newer note, got %q", n.Text)
	}
	if len(readNotes(p)) != 1 {
		t.Errorf("expected one note, got %d", len(readNotes(p)))
	}
}

// The stored blob is read and written on ordinary navigation, so it must not
// grow without limit, and it must not churn when nothing changed.
func TestStoreIsBoundedAndStable(t *testing.T) {
	p := newNotePrefs()
	for i := 1; i <= notesMax+50; i++ {
		saveNote(p, SharedNote{VersionID: "web", Book: "Psalms", Chapter: i, Text: "n"})
	}
	if got := len(readNotes(p)); got > notesMax {
		t.Errorf("store grew to %d, past the %d cap", got, notesMax)
	}

	before := p.String(prefSharedNotes)
	writeNotes(p, readNotes(p))
	if p.String(prefSharedNotes) != before {
		t.Error("rewriting unchanged notes changed the blob — the file would churn on every navigation")
	}
}

// Junk in the store must not become junk on screen.
func TestStoreRejectsRubbish(t *testing.T) {
	p := newNotePrefs()
	for _, raw := range []string{
		"", "not json", "{}", "[{}]",
		`[{"b":"John","c":0,"t":"x"}]`,
		`[{"b":"","c":3,"t":"x"}]`,
		`[{"b":"John","c":3,"t":"   "}]`,
	} {
		p.m[prefSharedNotes] = raw
		if got := readNotes(p); len(got) != 0 {
			t.Errorf("raw %q produced %d notes", raw, len(got))
		}
	}
}

func TestNoteTextIsNotTrustedFromTheStore(t *testing.T) {
	p := newNotePrefs()
	hostile := "<script>alert(1)</script>"
	saveNote(p, SharedNote{VersionID: "web", Book: "John", Chapter: 3, Text: hostile})
	n, _ := loadNote(p, "web", "John", 3)
	// The store keeps text verbatim — escaping is the RENDERER's job, and this
	// pins that we are not quietly relying on the store to sanitise.
	if n.Text != hostile {
		t.Errorf("the store altered the text: %q", n.Text)
	}
	if strings.Contains(htmlEscape(n.Text), "<script>") {
		t.Error("the chapter-HTML escaper let a script tag through")
	}
}
