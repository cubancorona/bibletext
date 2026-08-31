package bibletext

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

// The shared value must equal what EVERY surface composes for itself, in every
// state the enumeration can reach. Not a sample of states: the cases come from
// the same cross-product notes_state_flow_test.go walks, because a table
// written by hand is a table of the cases somebody thought of — which is the
// failure that file's own header was written about.
//
// This is the check that makes the seam a seam. Without it, chapterNoteChrome
// is a fifth composition sitting beside four others, and the next divergence
// arrives silently.
func TestNoteChromeIsOneValueForEverySurface(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer deleteAllNotes(appPrefs())
	defer setNotesEnabled(true)

	origNow := noteNow
	noteNow = func() int64 { return 1_700_000_000 }
	defer func() { noteNow = origNow }()
	origPills := notesPillPerParagraph
	defer func() { notesPillPerParagraph = origPills }()

	checked, mismatches := 0, 0
	for _, featureOn := range []bool{true, false} {
		for _, placement := range []notePlacement{placeNone, placeOwn, placeFollowed, placeBoth} {
			for _, collapsed := range []bool{false, true} {
				for _, foreignHL := range []bool{false, true} {
					for _, focus := range []noteFocusAxis{focusUnset, focusNoneAx, focusExactKey, focusFollowedNote, focusOwnAx} {
						for _, ownNote := range []bool{false, true} {
							for _, pills := range []bool{false, true} {
								w := notesWorld{
									featureOn: featureOn, placement: placement,
									collapsed: collapsed, foreignHL: foreignHL,
									focus: focus, ownNote: ownNote, pills: pills,
								}
								obs, offered := runNotesFlow(t, w)
								if !offered || obs.st == nil {
									continue
								}
								checked++
								st := obs.st
								plan := buildChapterPlan(st, appPrefs(), st.Bible)
								verses := st.Bible.GetChapter(st.CurrentBook, st.CurrentChapter)
								c := chapterNoteChrome(st, plan, verses)

								// Every Go-side push site, byte for byte.
								for name, push := range map[string]func(*AppState, chapterPlan) (string, string, bool, bool){
									"apple":   appleStickerPush,
									"android": androidStickerPush,
									"styled":  styledStickerPush,
								} {
									text, who, pill, next := push(st, plan)
									if !notesFeatureOn(st) {
										text, who, pill, next = "", "", false, false
									}
									if c.Text != text || c.Who != who || c.Pill != pill || c.Next != next {
										mismatches++
										if mismatches <= 3 {
											t.Errorf("%s: %s composes (%q,%q,%v,%v); the shared value has (%q,%q,%v,%v)",
												w.id(), name, text, who, pill, next, c.Text, c.Who, c.Pill, c.Next)
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no states were reached; the sweep is testing nothing")
	}
	t.Logf("checked %d reachable states against three push sites", checked)
}

// The derived decisions must agree with the tuple they came from, in every
// reachable state. They are methods rather than fields precisely so a composite
// literal cannot leave one stale — that shape was tried first and it silently
// stopped the pills being drawn, because present() read a field every existing
// literal left false.
func TestDerivedChromeDecisionsAgreeWithTheirTuple(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer deleteAllNotes(appPrefs())
	defer setNotesEnabled(true)

	origNow := noteNow
	noteNow = func() int64 { return 1_700_000_000 }
	defer func() { noteNow = origNow }()
	origPills := notesPillPerParagraph
	defer func() { notesPillPerParagraph = origPills }()

	checked := 0
	for _, placement := range []notePlacement{placeNone, placeOwn, placeFollowed, placeBoth} {
		for _, collapsed := range []bool{false, true} {
			for _, foreignHL := range []bool{false, true} {
				for _, focus := range []noteFocusAxis{focusUnset, focusExactKey, focusOwnAx} {
					for _, ownNote := range []bool{false, true} {
						w := notesWorld{
							featureOn: true, placement: placement, collapsed: collapsed,
							foreignHL: foreignHL, focus: focus, ownNote: ownNote,
						}
						obs, offered := runNotesFlow(t, w)
						if !offered || obs.st == nil {
							continue
						}
						checked++
						st := obs.st
						plan := buildChapterPlan(st, appPrefs(), st.Bible)
						c := chapterNoteChrome(st, plan, st.Bible.GetChapter(st.CurrentBook, st.CurrentChapter))

						if got, want := c.present(), c.Text != "" || c.Who != ""; got != want {
							t.Errorf("%s: present()=%v, tuple says %v", w.id(), got, want)
						}
						if got, want := c.hasTail(), c.Anchor > 0; got != want {
							t.Errorf("%s: hasTail()=%v, anchor %d", w.id(), got, c.Anchor)
						}
						if got, want := c.chevron() != "", c.Next; got != want {
							t.Errorf("%s: chevron present=%v, Next=%v", w.id(), got, want)
						}
						// The verb set and the verbs must never disagree: the
						// glyph is a promise about what the press does.
						wantVerbs := noteVerbsReceived
						switch {
						case !c.present():
							wantVerbs = noteVerbsNone
						case isOwnLiveNote(st):
							wantVerbs = noteVerbsOwn
						}
						if c.verbs() != wantVerbs {
							t.Errorf("%s: verbs()=%v, the verbs branch on %v",
								w.id(), c.verbs(), wantVerbs)
						}
					}
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no states reached")
	}
	t.Logf("checked %d states", checked)
}

// The predicate itself, at its edges. It is the whole of defect 2 now, so it is
// worth stating rather than only asserting that three call sites reach it.
func TestShouldCaptureScrollRestore(t *testing.T) {
	for _, tc := range []struct {
		name                              string
		hasRestore, same, changed, arrive bool
		want                              bool
	}{
		{"a re-render the reader did not ask for", false, true, true, false, true},
		{"an explicit arrival never captures", false, true, true, true, false},
		{"a restore already armed is not replaced", true, true, true, false, false},
		{"a different chapter is a navigation", false, false, true, false, false},
		{"an unchanged body has no snap to pre-empt", false, true, false, false, false},
		{"arrival wins over every other yes", false, true, true, true, false},
	} {
		st := &AppState{}
		if tc.hasRestore {
			st.restore = &restoreAnchor{}
		}
		if got := shouldCaptureScrollRestore(st, tc.same, tc.changed, tc.arrive); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
	if shouldCaptureScrollRestore(nil, true, true, false) {
		t.Errorf("a nil state must not capture")
	}
}
