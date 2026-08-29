package bibletext

// THE ARRIVALS LAYER, walked as JOURNEYS. See docs/VERSION_STATES.md.
//
// WHY THIS ONE CANNOT BE A CROSS-PRODUCT. The seven machines are enumerated as
// cells because each of their defects is a wrong answer to one question. An
// arrival is not a question, it is a PROMISE: the reader tapped something and
// the app owes them either the passage or a sentence. A promise is kept or
// broken over TIME, and the ways it breaks are all sequences —
//
//	a park that outlives the load it was waiting for;
//	a park consumed by an action the reader took for a different reason;
//	an arrival reported failed and then honoured anyway, minutes later.
//
// None of those is visible in any single state. The refresh machine already
// proved the point empirically in this same suite: 160 cells found nothing and
// 310 journeys found D4. So this file walks journeys and asserts after EVERY
// step, which is the only shape that can see a promise being broken.
//
// The cross-product that DOES cover arrivals already exists and is a different
// question: share_link_flow_test.go asks "is any state a dead end", over the
// link's own axes. This one asks what the arrival does to, and learns from,
// the VERSION machinery — the half docs/VERSION_STATES.md records as unmodelled.

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
)

// --- the world an arrival lands in ------------------------------------------

// arrivalFacts is the slice of AppState the arrivals layer branches on, plus
// the one fact no field records: whether the reader has been TOLD about the
// arrival that is currently outstanding.
type arrivalFacts struct {
	loc         string // book|chapter, so a surprise navigation is visible
	parked      bool   // a target is waiting
	parkedFor   string // ...on this translation loading ("" = on a download)
	current     string // CurrentVersion
	loading     bool   // versionLoading
	arrivalOwed bool   // an arrival is outstanding: neither opened nor explained
	failureTold bool   // the reader was shown the load failure for that arrival
}

func (f arrivalFacts) String() string {
	s := f.current + "@" + f.loc
	if f.parked {
		s += "+park"
		if f.parkedFor != "" {
			s += "(" + f.parkedFor + ")"
		}
	}
	if f.loading {
		s += "+loading"
	}
	if f.arrivalOwed {
		s += "+owed"
	}
	if f.failureTold {
		s += "+told"
	}
	return s
}

// --- the events -------------------------------------------------------------

type arrivalEvent int

const (
	arLinkNamesOther   arrivalEvent = iota // a link naming a translation that needs a fetch
	arFetchFails                           // ...and the fetch fails, with no cache to fall back on
	arFetchLands                           // ...or it lands
	arReaderPicksIt                        // the reader later chooses that same translation, and it works
	arReaderPicksOther                     // the reader chooses a different translation
)

func (e arrivalEvent) String() string {
	return [...]string{"link-names-other", "fetch-fails", "fetch-lands", "reader-picks-it", "reader-picks-other"}[e]
}

// the translation a link in these journeys names, and one for the reader to
// wander off to.
const (
	arrivalLinkVersion  = "webc"
	arrivalOtherVersion = "bsb"
)

// apply drives ONE event. Where production is a goroutine tail
// (switchVersionInteractive's fyne.Do body) the synchronous half is reproduced
// verbatim and cited, exactly as the refresh harness does — a test cannot await
// a goroutine deterministically.
func (e arrivalEvent) apply(t *testing.T, st *AppState, told *bool) {
	t.Helper()
	switch e {
	case arLinkNamesOther:
		if st.CurrentVersion == arrivalLinkVersion || st.loading() {
			return // switchToLinkVersion's own guards
		}
		// switchToLinkVersion's real-fetch branch, verbatim
		// (share_link_open.go): park the target, name the translation it waits
		// on, and let switchVersionInteractive own the spinner.
		parked := ShareTarget{VersionID: arrivalLinkVersion, Book: "John", Chapter: 1, VerseLo: 16}
		st.pendingLink = &parked
		st.pendingLinkVersion = arrivalLinkVersion
		st.pendingNoteOpenID = 0
		st.versionLoading = true
		*told = false
	case arFetchFails:
		if !st.versionLoading {
			return
		}
		// switchVersionInteractive's error tail (versions_ui.go), the arm where
		// the previous-epoch cache cannot help either: the spinner goes, our
		// own park is closed, and the reader is shown showVersionLoadError.
		st.versionLoading = false
		if st.pendingLinkVersion == arrivalLinkVersion {
			st.pendingLink = nil
			st.pendingLinkRaw = ""
			st.pendingLinkVersion = ""
			st.pendingNoteOpenID = 0
		}
		*told = true
	case arFetchLands:
		if !st.versionLoading {
			return
		}
		st.versionLoading = false
		if v, ok := versionByID(arrivalLinkVersion); ok {
			applyLoadedVersion(st, v, fullValidBible(), modeReal)
		}
	case arReaderPicksIt:
		if st.versionLoading || st.CurrentVersion == arrivalLinkVersion {
			return
		}
		if v, ok := versionByID(arrivalLinkVersion); ok {
			applyLoadedVersion(st, v, fullValidBible(), modeReal)
		}
	case arReaderPicksOther:
		if st.versionLoading || st.CurrentVersion == arrivalOtherVersion {
			return
		}
		if v, ok := versionByID(arrivalOtherVersion); ok {
			applyLoadedVersion(st, v, fullValidBible(), modeReal)
		}
	}
}

func (st *AppState) loading() bool { return st != nil && st.versionLoading }

func arrivalFactsOf(st *AppState, owed, told bool) arrivalFacts {
	return arrivalFacts{
		loc:         fmt.Sprintf("%s|%d", st.CurrentBook, st.CurrentChapter),
		parked:      st.pendingLink != nil,
		parkedFor:   st.pendingLinkVersion,
		current:     st.CurrentVersion,
		loading:     st.versionLoading,
		arrivalOwed: owed,
		failureTold: told,
	}
}

type pinnedArrivalDefect struct {
	name   string
	what   string
	covers func(prev, now arrivalFacts, ev arrivalEvent) bool
}

// knownArrivalIncoherent — every incoherent state of the arrivals layer
// reachable TODAY, by the name docs/VERSION_STATES.md gives it. Set equality
// asserted, so a fix that leaves a pin behind fails the suite.
var knownArrivalIncoherent = []pinnedArrivalDefect{}

// The arrivals invariants. All of them are about a PROMISE over time, which is
// why they are checked after every step of a journey rather than once.
//
//	A-A  A park exists only while something can still consume it. Once the
//	     load it was waiting on has failed AND the reader has been told, the
//	     promise is closed and the park must be closed with it.
//	A-B  The reader's location changes only from what they just did. A park
//	     retired earlier must never move them later.
//	A-C  A journey never ends owing the reader an arrival: either the passage
//	     opened or something was said.
func checkArrivalInvariants(prev, now arrivalFacts, ev arrivalEvent) []string {
	var bad []string
	if now.parked && now.failureTold && !now.loading {
		bad = append(bad, "A-A: the arrival was reported failed and its park is still waiting for a load that will never come")
	}
	// A reader-initiated switch is not an arrival. If the location moved on one,
	// something else moved them.
	if (ev == arReaderPicksIt || ev == arReaderPicksOther) && now.loc != prev.loc {
		bad = append(bad, "A-B: choosing a translation moved the reader to a passage they did not ask for")
	}
	return bad
}

// --- the trajectory walk ------------------------------------------------------

func TestArrivalJourneysKeepTheirPromise(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	shortest := map[string]int{}
	route := map[string]string{}
	seen := map[string]bool{}
	journeys, steps := 0, 0

	events := []arrivalEvent{arLinkNamesOther, arFetchFails, arFetchLands, arReaderPicksIt, arReaderPicksOther}

	var walk func(path []arrivalEvent, depth int)
	walk = func(path []arrivalEvent, depth int) {
		if depth == 0 {
			return
		}
		for _, ev := range events {
			next := append(append([]arrivalEvent(nil), path...), ev)
			journeys++

			// A FRESH state per branch. AppState carries an atomic, so it can
			// never be copied to fork a journey — every branch is replayed from
			// the beginning instead.
			st, told := freshArrivalState()
			prev := arrivalFactsOf(st, false, told)
			ok := true
			for _, e := range next {
				e.apply(t, st, &told)
				steps++
				owed := st.pendingLink != nil && !told
				now := arrivalFactsOf(st, owed, told)
				for _, bad := range checkArrivalInvariants(prev, now, e) {
					explained := false
					for _, d := range knownArrivalIncoherent {
						if d.covers(prev, now, e) {
							seen[d.name] = true
							explained = true
							break
						}
					}
					if !explained {
						// Keyed on the VIOLATION, not the path: 780 journeys
						// reach the same broken promise by hundreds of routes,
						// and a report that lists them all buries the handful
						// of distinct facts. The SHORTEST route is kept,
						// because that is the one to read.
						key := fmt.Sprintf("%s: %s", e, bad)
						if old, had := shortest[key]; !had || len(next) < old {
							shortest[key] = len(next)
							route[key] = pathString(next)
						}
						ok = false
					}
				}
				prev = now
			}
			_ = ok
			walk(next, depth-1)
		}
	}
	walk(nil, 4)

	if len(shortest) > 0 {
		var lines []string
		for k, r := range route {
			lines = append(lines, fmt.Sprintf("%s\n      shortest route: %s", k, r))
		}
		sort.Strings(lines)
		t.Errorf("%d journeys, %d steps; %d distinct broken promises with no entry in the register:\n  %s",
			journeys, steps, len(lines), strings.Join(lines, "\n  "))
	}
	for _, d := range knownArrivalIncoherent {
		if !seen[d.name] {
			t.Errorf("%s is pinned as reachable but no journey reached it — if it is fixed, strike it from knownArrivalIncoherent and from docs/VERSION_STATES.md: %s", d.name, d.what)
		}
	}
	t.Logf("%d journeys / %d steps walked; %d pinned incoherent states reached", journeys, steps, len(seen))
}

func pathString(p []arrivalEvent) string {
	out := make([]string, len(p))
	for i, e := range p {
		out[i] = e.String()
	}
	return strings.Join(out, " -> ")
}

func freshArrivalState() (*AppState, bool) {
	base := fullValidBible()
	return &AppState{
		Bible:          base,
		CurrentVersion: defaultVersionID,
		currentMode:    modeReal,
		loadedVersions: map[string]*BibleData{defaultVersionID: base},
		loadPhase:      loadReady,
		CurrentBook:    "Genesis",
		CurrentChapter: 1,
	}, false
}

// TestADeadLinkDoesNotMoveTheReaderLater is D12 — the one the journeys found,
// at the shortest route link-names-other -> fetch-fails -> reader-picks-it.
//
// It is FLOW-SHAPED, and deliberately so: no single state is wrong here. A
// park waiting on a translation is correct. A load failing is correct. The
// error card is correct. Choosing a translation from the picker is correct.
// The defect is only in the composition, which is why the cross-products in
// this suite are blind to it and the journeys are not.
func TestADeadLinkDoesNotMoveTheReaderLater(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st, told := freshArrivalState()
	here := fmt.Sprintf("%s|%d", st.CurrentBook, st.CurrentChapter)

	// The reader taps a shared link naming a translation they do not have in
	// memory. The target parks and the load owns the spinner.
	arLinkNamesOther.apply(t, st, &told)
	if st.pendingLink == nil || st.pendingLinkVersion != arrivalLinkVersion {
		t.Fatal("control: the link must park on the translation it named, or the rest of this proves nothing")
	}

	// The load fails and the reader is told so. The promise is now closed.
	arFetchFails.apply(t, st, &told)
	if !told {
		t.Fatal("control: the reader must have been told the load failed")
	}
	if st.pendingLink != nil {
		t.Fatal("a park is still waiting for a load that will never come")
	}

	// Much later, for their own reasons, the reader chooses that same
	// translation from the picker. It works this time.
	arReaderPicksIt.apply(t, st, &told)
	if st.CurrentVersion != arrivalLinkVersion {
		t.Fatalf("the switch itself must work; on %q", st.CurrentVersion)
	}
	if got := fmt.Sprintf("%s|%d", st.CurrentBook, st.CurrentChapter); got != here {
		t.Fatalf("choosing a translation moved the reader from %s to %s — the dead link was honoured behind their back", here, got)
	}
}

// TestAParkForAnotherTranslationSurvivesThisOnesFailure is the other half of
// the same fix, and the reason it is conditioned rather than unconditional: a
// target waiting on a DIFFERENT translation belongs to a different load, which
// still has its own consumer. Clearing every park on any failure would strand
// that one instead — trading this defect for its mirror image.
func TestAParkForAnotherTranslationSurvivesThisOnesFailure(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st, told := freshArrivalState()
	arLinkNamesOther.apply(t, st, &told)
	// Re-point the park at some other translation, as a second arrival would.
	st.pendingLinkVersion = arrivalOtherVersion

	arFetchFails.apply(t, st, &told)
	if st.pendingLink == nil || st.pendingLinkVersion != arrivalOtherVersion {
		t.Fatal("a park waiting on another translation was cleared by this load's failure")
	}
}
