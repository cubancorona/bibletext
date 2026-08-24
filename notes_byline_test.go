package bibletext

// The dormant name path (S9). senderNamesEnabled is false and nothing sets
// it, so every assertion about NAMES runs through senderNameWithFlag with the
// gate forced open — the rules are tested NOW, before any name can ever be
// shown, so flipping the constant later changes no behaviour that was not
// already pinned here.

import (
	"fyne.io/fyne/v2/test"
	"strings"
	"testing"
)

// The flag is OFF. This is the one test that would fail the moment somebody
// flips the constant, which is exactly what it is for: the flip must be a
// deliberate decision that lands together with this line changing.
func TestSenderNamesAreDormant(t *testing.T) {
	if senderNamesEnabled {
		t.Fatal("senderNamesEnabled is on — sender names would display; this is a deliberate decision " +
			"(docs/NOTES_SPEC.md#sender-and-attribution-contract). If it is intended, update this test and revalidate " +
			"every byline surface.")
	}
	n := StoredNote{Kind: noteKindReceived, SenderName: "Fixture Sender"}
	if got := senderName(n); got != "Friend" {
		t.Errorf("flag off: senderName = %q, want Friend whatever the record carries", got)
	}
	if got := senderByline(n); got != "Note from Friend" {
		t.Errorf("flag off: senderByline = %q, want the exact literal it replaced", got)
	}
	if got := noteByline(n); got != "From Friend" {
		t.Errorf("flag off: noteByline = %q, want the banner's exact spelling", got)
	}
	if got := noteByline(StoredNote{Kind: noteKindMine, SenderName: "Fixture Sender"}); got != "From you" {
		t.Errorf("flag off, mine: noteByline = %q", got)
	}
	if got := senderByline(StoredNote{Kind: noteKindMine}); got != "Note from you" {
		t.Errorf("mine: senderByline = %q", got)
	}
}

// iso wraps the expected display form in the isolates the app adds.
func iso(s string) string { return "⁦" + s + "⁩" }

func TestSenderNameDisplayRules(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		// The empty and near-empty names fall back.
		{"empty", "", "Friend"},
		{"whitespace only", "  \n\t ", "Friend"},
		{"controls only", "\x01\x02‮", "Friend"},

		// Ordinary names pass, bidi-isolated.
		{"plain", "Fixture Sender", iso("Fixture Sender")},
		{"spaced", "Fixture Sender Alpha", iso("Fixture Sender Alpha")},
		{"initials", "F. S. Example", iso("F. S. Example")},
		{"unicode", "Fïxture Sënder", iso("Fïxture Sënder")},
		// Homoglyphs are NOT filtered — they are also legitimate names, and
		// display (isolation, quiet styling) is the honest defence.
		{"cyrillic homoglyph", "Ѕample Sender", iso("Ѕample Sender")},

		// One line: newlines, tabs and controls become collapsed spaces.
		{"newline", "Fixture\nSender", iso("Fixture Sender")},
		{"crlf and tabs", "Fixture\r\n\tSender", iso("Fixture Sender")},
		{"run of blanks", "Fixture    Sender", iso("Fixture Sender")},

		// Bidi steering INSIDE a name is stripped before our isolates go on.
		{"rlo stripped", "‮fixture", iso("fixture")},
		{"isolates stripped", "⁦fixture⁩", iso("fixture")},

		// The 24-rune cap, counted in runes.
		{"at the cap", strings.Repeat("a", 24), iso(strings.Repeat("a", 24))},
		{"over the cap", strings.Repeat("a", 25), iso(strings.Repeat("a", 24))},
		{"cap in runes not bytes", strings.Repeat("å", 25), iso(strings.Repeat("å", 24))},

		// Chrome impersonation is refused — the fallback is Friend, and the
		// note still shows.
		{"bibletext", "BibleText", "Friend"},
		{"bibletext upper", "BIBLETEXT", "Friend"},
		{"note", "Note", "Friend"},
		{"notes", "notes", "Friend"},
		{"support", "BibleText Support", "Friend"},
		{"support spaced", "  bibletext   support ", "Friend"},
		{"embedded", "Fixture BibleText", "Friend"},
		{"split", "Bible Text security", "Friend"},
		{"newline smuggle", "Fixture\nBibleText Support", "Friend"},
		// ...but a name merely CONTAINING "note" is a name.
		{"notebook", "Notebook", iso("Notebook")},

		// URL-ish tokens are refused: a byline is never a place for a link.
		{"scheme", "https://evil.example", "Friend"},
		{"bare scheme", "http:evil", "Friend"},
		{"www", "www.evil.example", "Friend"},
		{"dotted host", "evil.com", "Friend"},
		{"host with path", "bibletext.co.uk/web", "Friend"},
		{"host inside", "read evil.com now", "Friend"},
		{"trailing dot ok", "Fixture S.", iso("Fixture S.")},
	}
	for _, c := range cases {
		n := StoredNote{Kind: noteKindReceived, SenderName: c.in}
		if got := senderNameWithFlag(n, true); got != c.want {
			t.Errorf("%s: senderNameWithFlag(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

// The composed byline under the flag: the frame is the app's, the slot is the
// (isolated) name, and refusal falls back to the exact literal.
func TestSenderBylineComposition(t *testing.T) {
	if senderNamesEnabled {
		t.Skip("composition under the flag is exercised through senderNameWithFlag while dormant")
	}
	// Route the dormant branch by hand, the way the flip would.
	n := StoredNote{Kind: noteKindReceived, SenderName: "Fixture Sender"}
	if got := "Note from " + senderNameWithFlag(n, true); got != "Note from "+iso("Fixture Sender") {
		t.Errorf("byline with a name = %q", got)
	}
	refused := StoredNote{Kind: noteKindReceived, SenderName: "BibleText Support"}
	if got := "Note from " + senderNameWithFlag(refused, true); got != "Note from Friend" {
		t.Errorf("refused byline = %q, want the Friend fallback", got)
	}
}

// Formatting and zero-width characters must not bypass the reserved-byline guard.
func TestHostileNamesCannotWearTheChrome(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
	}{
		// U+200B inside "BibleText" — renders pixel-identically to the refused
		// string, and case-folded WITH the invisible it no longer matched. Cf
		// is stripped as a CLASS now, so the fold sees the real letters.
		{"zero-width impersonation", "Bible​Text Support"},
		{"zwj impersonation", "Bible‍Text"},
		// " · " is the who-line's own separator; a name carrying it could pose
		// as the app's counts, and truncation would then PROTECT the forgery.
		{"chrome-grammar forgery", "Amy · 9 not shown here"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := senderNameWithFlag(StoredNote{SenderName: tc.in}, true)
			if strings.Contains(got, "​") || strings.Contains(got, "‍") {
				t.Errorf("an invisible survived sanitisation: %q", got)
			}
			if strings.Contains(got, " · ") {
				t.Errorf("the chrome separator survived inside a name: %q", got)
			}
			folded := strings.ToLower(got)
			if strings.Contains(folded, "bibletext") {
				t.Errorf("a chrome impersonation was displayed: %q", got)
			}
		})
	}
}

// The pill press clears a foreign mark before restoring, matching the banner
// chip's rule that the restored note becomes the active choice.
func TestRestoringANoteClearsAForeignMark(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())
	addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
		Book: "Psalms", Chapter: 23, VerseLo: 1, Text: "restore me"})

	st := psalm23State()
	applyNoteForCurrentChapter(st)
	hideCurrentNote(st)
	// A search result lights a different verse — the suppressing mark.
	st.setHL(hlSearch, "Psalms", 23, 4, 0)

	restoreCurrentNote(st)
	if sp, ok := st.markSpan(); ok && st.mark.Origin == hlSearch {
		t.Errorf("the foreign mark survived the restore: %+v", sp)
	}
	if st.NoteMinimized {
		t.Error("the note did not restore")
	}
}
