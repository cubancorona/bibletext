package bibletext

// The fragment is a key list, and these are the rules that make it survivable:
// a note round-trips, a link without one is byte-identical to what we emitted
// before the feature existed, and an unknown key is IGNORED rather than allowed
// to swallow the verse. That last one is the whole reason the grammar was made
// a key list before the first link-capable release shipped.

import (
	"strings"
	"testing"
)

func TestShareLinkCarriesANote(t *testing.T) {
	const note = "fixture message alpha"

	url := ShareLinkURLWithNote("web", "John", 3, 16, 18, note)
	if !strings.HasPrefix(url, "https://bibletext.co.uk/web/john/3/#v16-18&n=") {
		t.Fatalf("unexpected shape: %s", url)
	}

	got, ok := ParseShareLink(url)
	if !ok {
		t.Fatalf("our own link did not parse: %s", url)
	}
	if got.Book != "John" || got.Chapter != 3 || got.VerseLo != 16 || got.VerseHi != 18 {
		t.Errorf("passage lost: %+v", got)
	}
	if got.Note != note {
		t.Errorf("note lost:\n got %q\nwant %q", got.Note, note)
	}
}

// Adding the feature must not have changed a single link that does not use it.
func TestEmptyNoteChangesNothing(t *testing.T) {
	for _, tc := range []struct{ lo, hi int }{{0, 0}, {16, 0}, {16, 18}} {
		plain := ShareLinkURL("web", "John", 3, tc.lo, tc.hi)
		for _, empty := range []string{"", "   ", "\n\t", "‏"} {
			withNote := ShareLinkURLWithNote("web", "John", 3, tc.lo, tc.hi, empty)
			if withNote != plain {
				t.Errorf("an empty note (%q) changed the link:\n got %s\nwant %s",
					empty, withNote, plain)
			}
		}
	}
}

// A link written by a future version — carrying a key this build has never
// heard of — must still open at the right passage.
func TestUnknownFragmentKeysAreIgnored(t *testing.T) {
	for _, url := range []string{
		"https://bibletext.co.uk/web/john/3/#v16-18&x=whatever",
		"https://bibletext.co.uk/web/john/3/#v16-18&n=%%%broken%%%",
		"https://bibletext.co.uk/web/john/3/#v16-18&zzz=1&yyy=2",
		"https://bibletext.co.uk/web/john/3/#v16-18&n=",
	} {
		got, ok := ParseShareLink(url)
		if !ok {
			t.Errorf("link rejected outright: %s", url)
			continue
		}
		if got.Book != "John" || got.Chapter != 3 || got.VerseLo != 16 || got.VerseHi != 18 {
			t.Errorf("an unknown key ate the passage in %s: %+v", url, got)
		}
	}
}

// A note-bearing link with no verse is a chapter link that still carries its
// note — the fragment must not be mistaken for a verse payload.
func TestNoteWithoutVerse(t *testing.T) {
	url := ShareLinkURLWithNote("bsb", "Psalms", 23, 0, 0, "fixture chapter message")
	if !strings.Contains(url, "#n=") || strings.Contains(url, "#v") {
		t.Fatalf("unexpected shape: %s", url)
	}
	got, ok := ParseShareLink(url)
	if !ok || got.Chapter != 23 || got.VerseLo != 0 {
		t.Fatalf("chapter link broke: %+v ok=%v", got, ok)
	}
	if got.Note != "fixture chapter message" {
		t.Errorf("note lost: %q", got.Note)
	}
}

// A note must never be able to forge a different passage, however it is written.
func TestNoteCannotForgeThePassage(t *testing.T) {
	for _, hostile := range []string{
		"&v=99", "#v99", "v99&", "&&&", "=&=&=",
		"https://example.com/phish",
	} {
		url := ShareLinkURLWithNote("web", "John", 3, 16, 0, hostile)
		got, ok := ParseShareLink(url)
		if !ok {
			t.Fatalf("link rejected: %s", url)
		}
		if got.VerseLo != 16 || got.VerseHi != 0 || got.Chapter != 3 || got.Book != "John" {
			t.Errorf("note %q moved the passage: %+v", hostile, got)
		}
		if got.Note != hostile {
			t.Errorf("note %q came back as %q", hostile, got.Note)
		}
	}
}

// Every link the app can emit must parse back — including with a note, and
// including the awkward inputs a real note contains.
func TestNoteRoundTripThroughTheURL(t *testing.T) {
	for _, note := range []string{
		"plain",
		"with & ampersand = equals # hash ? query",
		"emoji 🙏 and accents é ü ñ",
		"multi\nline\nnote",
		strings.Repeat("long ", 60),
	} {
		url := ShareLinkURLWithNote("webc", "1 Maccabees", 2, 19, 22, note)
		got, ok := ParseShareLink(url)
		if !ok {
			t.Errorf("did not parse: %s", url)
			continue
		}
		if want := normalizeNote(note); got.Note != want {
			t.Errorf("note changed through the URL:\n got %q\nwant %q", got.Note, want)
		}
		if got.Book != "1 Maccabees" || got.VerseLo != 19 || got.VerseHi != 22 {
			t.Errorf("passage lost for note %q: %+v", note, got)
		}
	}
}
