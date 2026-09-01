package bibletext

import (
	"strings"
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

	checked, mismatches, withCounts := 0, 0, 0
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

								// THE ENABLING FACT, in every reachable state: the
								// counts span is a substring of the line, it occurs
								// EXACTLY once, and it exists exactly when the
								// counts are a control. Each native finds it by
								// searching; a second occurrence would let a
								// backwards search accent the wrong one, and a
								// missing one would silently drop the affordance.
								if (c.Counts != "") != c.Next {
									t.Errorf("%s: Counts=%q but Next=%v", w.id(), c.Counts, c.Next)
								}
								if c.Counts != "" {
									withCounts++
									if n := strings.Count(c.Who, c.Counts); n != 1 {
										t.Errorf("%s: the counts span %q occurs %d times in %q",
											w.id(), c.Counts, n, c.Who)
									}
									if !strings.HasSuffix(c.Counts, noteChevron) &&
										!strings.Contains(c.Who, c.Counts+noteWhoSep) {
										t.Errorf("%s: the counts span %q is neither the line's "+
											"tail nor followed by a separator in %q — the grammar "+
											"the span was cut by has changed", w.id(), c.Counts, c.Who)
									}
								}

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
									// The ONE documented addition: the chevron is
									// part of the line now, not something four
									// natives append to it afterwards (it was four
									// literals, and Android's was two spaces wide).
									// Stated as an exact transformation of the push
									// tuple, so the shared value still cannot become
									// a fourth composition without this failing.
									if next {
										who += noteChevron
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
	if withCounts == 0 {
		t.Error("no state produced a counts control, so the substring every native " +
			"now searches for was never exercised by this sweep")
	}
	t.Logf("checked %d reachable states against three push sites (%d with counts)",
		checked, withCounts)
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
	arrivals := map[noteArrival]int{}
	// A CENSUS, not only a per-cell assertion. hasTail() reads Anchor, so the
	// per-cell line below is true by construction and proves nothing on its own.
	// The census says something the assertion cannot: on the APPLE/Android push
	// today, an expanded card ALWAYS has an anchor, because an unplaced note has
	// nothing to open and stands down to the pill. So the tail gate those three
	// natives gained changes no pixel yet — it is the styled pane's chapter-top
	// card (Anchor 0, drawn per paragraph) that needs it, and the natives get
	// that state when the bands step pushes per-paragraph placement to them.
	// Stated as a tripwire in BOTH directions: if the anchorless count ever goes
	// above zero here, this comment is stale and the natives are drawing the
	// state for real.
	var expandedAnchorless, expandedAnchored int
	for _, placement := range []notePlacement{placeNone, placeOwn, placeFollowed, placeBoth} {
		for _, collapsed := range []bool{false, true} {
			for _, foreignHL := range []bool{false, true} {
				for _, focus := range []noteFocusAxis{focusUnset, focusExactKey, focusOwnAx} {
					for _, ownNote := range []bool{false, true} {
						// The ARRIVAL axis rides along because it is what makes a
						// state explicit (applyShareTarget sets forceReposition):
						// a plain entry arrives nowhere by rule now, so without
						// it every cell here reports arriveNothing and the
						// verse/band classes cross the ABI untested.
						for _, arrival := range []bool{false, true} {
							w := notesWorld{
								featureOn: true, placement: placement, collapsed: collapsed,
								foreignHL: foreignHL, focus: focus, ownNote: ownNote,
								arrival: arrival,
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
							// The arrival class must be self-consistent with the tuple it
							// came from: a band can only be arrived at when there IS a note,
							// and a verse target is always a real verse.
							switch c.Arrival {
							case arriveBand:
								if !c.present() {
									t.Errorf("%s: arriveBand with no note on screen", w.id())
								}
								fallthrough
							case arriveVerse:
								if c.ArrivalVerse <= 0 {
									t.Errorf("%s: %v with verse %d", w.id(), c.Arrival, c.ArrivalVerse)
								}
							case arriveNothing:
							}
							arrivals[c.Arrival]++
							if c.present() && !c.collapsed() {
								if c.hasTail() {
									expandedAnchored++
								} else {
									expandedAnchorless++
								}
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
	}
	if checked == 0 {
		t.Fatal("no states reached")
	}
	if expandedAnchored == 0 {
		t.Error("no expanded card with an anchor was reached — the ordinary case " +
			"is missing from this sweep and the census below proves nothing")
	}
	if expandedAnchorless != 0 {
		t.Errorf("%d expanded cards with NO anchor: the single-card push can now "+
			"reach the anchorless state, so the natives' tail gate is live and the "+
			"note above it needs rewriting — and this cell needs a rendering proof "+
			"on a device, not only a census", expandedAnchorless)
	}
	// Every class must be REACHED, or the natives are handed a case this sweep
	// has never seen and the per-surface renderers are untested for it.
	for _, want := range []noteArrival{arriveNothing, arriveVerse, arriveBand} {
		if arrivals[want] == 0 {
			t.Errorf("no state produced %v; that case crosses the ABI untested", want)
		}
	}
	t.Logf("checked %d states; expanded cards: %d anchored, %d anchorless; arrivals %v",
		checked, expandedAnchored, expandedAnchorless, arrivals)
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
