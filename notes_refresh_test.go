package bibletext

// THE APPLE NOTE VERBS MUST ASK FOR THE NARROW REPAINT, held by parsing the
// source.
//
// ai_menu_darwin.go is a file of //export callbacks reached from Objective-C on
// the native UI thread. Driving one from a Go test means standing up an
// activeAIState and a live overlay, which is exactly what the host cannot do —
// the same problem notes_spacing_spec_test.go and reading_ios_menu_guard_test.go
// solve the same way, for the same reason: a rule nothing checks is a rule that
// quietly stops being true.
//
// WHAT IS BEING GUARDED. Each of these verbs changes which note is drawn and
// which verses are washed, and nothing else. Both are live mutations on the
// native pane already on screen. Ending them in state.refreshReadingOnly()
// instead throws away and rebuilds the whole Fyne reading column, and because
// the native view's frame TRACKS a widget in that column, the rebuild moves the
// pane and the correction rides a 60 ms timer — the "delay and a flash of what

// 19 Aug 2026. refreshNoteOnly (notes_refresh.go) does the two mutations and
// leaves the tree alone, falling back to the rebuild wherever that is not a
// true substitute.
//
// This test is deliberately UNTAGGED so Linux and Windows CI parse the darwin
// file too — the flash is invisible on those platforms, so their CI is the only
// place a regression here would otherwise go unnoticed.

import (
	"regexp"
	"strings"
	"testing"
)

func TestAppleNoteVerbsTakeTheNarrowRepaint(t *testing.T) {
	src := readNativeSource(t, "ai_menu_darwin.go")

	// The verbs, and what each one changes — all of it note/wash presentation
	// on a chapter that is not itself changing.
	verbs := map[string]string{
		"bibleTextNoteHidden":       `the note and its highlight come down together`,
		"bibleTextNoteRestored":     `the collapsed marker is pressed and the note comes back`,
		"bibleTextNoteNextTapped":   `focus advances to the next note in the group`,
		"bibleTextNoteDeleted":      `the note goes for good and the highlight with it`,
		"bibleTextHighlightCleared": `the wash is cleared, which may re-open a note it was suppressing`,
	}

	for name, what := range verbs {
		body := funcBody(t, src, name)
		if strings.Contains(body, "state.refreshReadingOnly()") {
			t.Errorf("%s still ends in state.refreshReadingOnly() — %s.\n"+
				"That rebuilds the entire Fyne reading column, which moves the native "+
				"overlay's frame and brings back the ~60ms misplaced-pane flash. Use "+
				"refreshNoteOnly(state), which falls back to exactly this when it must.",
				name, what)
		}
		if !strings.Contains(body, "refreshNoteOnly(state)") {
			t.Errorf("%s no longer calls refreshNoteOnly(state) — %s.\n"+
				"Every one of these verbs is a note-selection change and must take the "+
				"narrow repaint.", name, what)
		}
	}
}

// THE FALLBACK MUST STAY REACHABLE. refreshNoteOnly is only safe to put in
// front of the verbs because it ends in the old behaviour whenever the in-place
// push refuses. A refactor that dropped that tail would turn every refusal —
// a stale body, a pending restore, a standing notice — into a verb that changes
// state and repaints nothing.
func TestNarrowNoteRepaintStillFallsBackToTheRebuild(t *testing.T) {
	src := readNativeSource(t, "notes_refresh.go")
	if !strings.Contains(src, "state.refreshReadingOnly()") {
		t.Error("refreshNoteOnly no longer falls back to state.refreshReadingOnly().\n" +
			"Without that tail a refused in-place push is a verb that mutates state and " +
			"leaves the screen showing the note before it.")
	}
	if !strings.Contains(src, "refreshNoteInPlace(state)") {
		t.Error("refreshNoteOnly no longer tries the in-place push at all — it is now just " +
			"a slower spelling of state.refreshReadingOnly(), and the flash is back.")
	}
}

// funcBody returns the source text of one top-level func, from its signature to
// the closing brace in column 1.
func funcBody(t *testing.T, src, name string) string {
	t.Helper()
	re := regexp.MustCompile(`(?ms)^func ` + regexp.QuoteMeta(name) + `\(.*?^}`)
	m := re.FindString(src)
	if m == "" {
		t.Fatalf("cannot find func %s — it was renamed or removed, and this test can no "+
			"longer say anything about the repaint it asks for", name)
	}
	return m
}
