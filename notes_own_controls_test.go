package bibletext

// ONE CONTROL ON YOUR OWN NOTE, held across all four surfaces.
//

// for a note bubble FROM me — minimize and X? But don't they do the same thing?
// Because minimize seems to hide the pill also."
//
// He was right on both halves. For an own note hideCurrentNote and
// dropCurrentNote take early returns whose bodies are IDENTICAL — focus to
// none, the note's own mark cleared, re-project — so − and ✕ were two controls
// for one verb. And "minimize hides the pill" is not a bug beside it, it is the
// reason: an own note enters the chapter plan only while focus names it
// (notes_plan.go) and is built Open, so focusNone removes it outright. There is
// no pill state for an own note to minimize INTO, which makes − a control whose
// name describes something that cannot happen.
//
// HOW TWO GOOD DECISIONS COLLIDED. − was made ephemeral for own notes so it
// would not write a durable "minimized" bit and make the notes list say
// "minimized in the chapter" about a card that is only on screen because the
// reader asked. ✕ was made non-destructive for own notes so one unconfirmed tap
// could not destroy the only copy of something the reader wrote. Each is right.
// Nobody put them side by side, and they met in the middle.
//
// ✕ is the mark that survives, because "put this away" is what the press does.

import (
	"strings"
	"testing"
)

// The two verbs must stay INTERCHANGEABLE for an own note — that equivalence is
// what justifies drawing only one control. If a later change made them differ,
// hiding − would start costing the reader something.
func TestOwnNoteHideAndDropAreTheSameVerb(t *testing.T) {
	src := readNativeSource(t, "notes_store.go")

	body := func(fn string) string {
		i := strings.Index(src, "func "+fn+"(")
		if i < 0 {
			t.Fatalf("cannot find %s — this guard can no longer say anything", fn)
		}
		rest := src[i:]
		if e := strings.Index(rest[1:], "\nfunc "); e > 0 {
			rest = rest[:e+1]
		}
		return rest
	}

	// Both own-note arms are the same three calls, in the same order.
	for _, fn := range []string{"hideCurrentNote", "dropCurrentNote"} {
		b := body(fn)
		i := strings.Index(b, "isOwnLiveNote(state)")
		if i < 0 {
			t.Errorf("%s no longer has an own-note arm. If an own note now takes the "+
				"general path, − and ✕ may have stopped being the same verb — and the "+
				"reading pane draws only ✕ on the strength of their being identical.", fn)
			continue
		}
		arm := b[i:]
		if e := strings.Index(arm, "return"); e > 0 {
			arm = arm[:e]
		}
		for _, call := range []string{"focusNone()", "clearMarkFromNote()", "applyNoteForCurrentChapter(state)"} {
			if !strings.Contains(arm, call) {
				t.Errorf("%s's own-note arm no longer calls %s — the two verbs have diverged, "+
					"so the single ✕ control in the reading pane is now hiding a real choice.",
					fn, call)
			}
		}
	}
}

// AND EVERY SURFACE DRAWS ONE. Four independent implementations of the same
// sticker — two Objective-C, one Java, one Fyne — is exactly the shape where a
// fix lands on three and the fourth keeps the defect. Checked by parsing each,
// because none of them can be exercised from a Go test on this host.
func TestEverySurfaceHidesMinimizeOnAnOwnNote(t *testing.T) {
	for _, s := range []struct{ file, gate, why string }{
		{"reading_ios.go", "if (!gNoteOwn) {",
			"the iOS sticker builds its buttons in btIOSEnsureNoteView"},
		{"reading_macos.go", "if (!gMacNoteOwn) {",
			"the macOS twin builds them the same way"},
		{"android/BtBridge.java", "if (!noteOwn) {",
			"the Android sticker floats its verbs by slot-from-right"},
		{"reading_styled_note.go", "r.noteHasHide = !p.note.Own",
			"the styled pane lays its buttons out positionally, so it must also RECORD the omission"},
	} {
		src := readNativeSource(t, s.file)
		if !strings.Contains(src, s.gate) {
			t.Errorf("%s does not gate the minimize control on the own-note flag (%s).\n"+
				"Expected to find %q. Without it this surface draws − beside ✕ on a note "+
				"the reader wrote, where both do the same thing and − names a pill that "+
				"cannot exist.", s.file, s.why, s.gate)
		}
	}
}
