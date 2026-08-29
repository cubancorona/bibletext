package bibletext

// M3, the refresh machine — and the TRAJECTORY harness. See
// docs/VERSION_STATES.md.
//
// TWO KINDS OF DEFECT, AND WHY ONE HARNESS CANNOT FIND BOTH.
//
// A cross-product enumeration walks (state, event) pairs and finds STATE
// incoherence: one question, one wrong answer. V1, V2 and D1 were all that
// shape, and the two enumerations already in the suite are built for it.
//
// The other shape is FLOW incoherence: every step individually correct, the
// COMPOSITION wrong. No cell can see one, because no cell is a sequence. The
// refresh machine is where they live, because its state outlives the events
// that set it: a flag raised on a fresh install is still raised four events
// later when something else reads it and draws the wrong conclusion.
//
// So this file carries both — the cross-product for M3's own states, and a
// walk over JOURNEYS, asserting the invariants after EVERY step rather than
// once at the end. A journey is what the reader actually does.

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
)

// --- the refresh machine's own state ----------------------------------------

// refreshFacts is the part of AppState this machine owns, plus the one field
// it is coupled to (which version is on screen).
type refreshFacts struct {
	pending     bool
	seedOnly    bool
	downloading bool
	retryDelay  time.Duration
	current     string // CurrentVersion
}

func (f refreshFacts) String() string {
	s := "settled"
	switch {
	case f.downloading:
		s = "downloading"
	case f.pending && f.retryDelay > 0:
		s = "backoff"
	case f.pending:
		s = "pending"
	}
	if f.seedOnly {
		s += "+seed"
	}
	if f.current != defaultVersionID {
		s += "+away"
	}
	return s
}

func (f refreshFacts) toState(t *testing.T) *AppState {
	t.Helper()
	st := &AppState{
		CurrentVersion:  f.current,
		fullPending:     f.pending,
		seedOnly:        f.seedOnly,
		fullDownloading: f.downloading,
		fullRetryDelay:  f.retryDelay,
		loadedVersions:  map[string]*BibleData{},
	}
	return st
}

func factsOf(st *AppState) refreshFacts {
	return refreshFacts{
		pending:     st.fullPending,
		seedOnly:    st.seedOnly,
		downloading: st.fullDownloading,
		retryDelay:  st.fullRetryDelay,
		current:     st.CurrentVersion,
	}
}

// --- the events -------------------------------------------------------------

type refreshEvent int

const (
	reDownloadLands refreshEvent = iota // applyFullDownload with the full text
	reSwitchAway                        // the reader picks another translation
	reSwitchBack                        // ...and returns to the default
	rePickerOpen                        // the picker's manual retry + its notice
	reFetchFails                        // the download tail's failure branch
)

func (e refreshEvent) String() string {
	return [...]string{"download-lands", "switch-away", "switch-back", "picker-open", "fetch-fails"}[e]
}

// apply drives ONE event through the app's real logic and returns the state
// after it. Where the production path is a goroutine (triggerFullDownload's
// tail) the synchronous half is reproduced exactly and cited, because a test
// cannot await a goroutine deterministically.
func (e refreshEvent) apply(t *testing.T, st *AppState) {
	t.Helper()
	def, _ := versionByID(defaultVersionID)
	switch e {
	case reDownloadLands:
		applyFullDownload(st, def, fullValidBible(), modeReal)
	case reSwitchAway:
		st.CurrentVersion = "bsb"
	case reSwitchBack:
		st.CurrentVersion = defaultVersionID
	case rePickerOpen:
		// showVersionPicker's head, verbatim (versions_ui.go): the manual
		// retry, then the notice computed after it.
		if st.fullPending && !st.fullDownloading {
			st.fullRetryDelay = 0
			if !st.stopping.Load() {
				st.fullDownloading = true // triggerFullDownload sets this synchronously
			}
		}
	case reFetchFails:
		// The failure branch of the download tail (app.go), synchronous half.
		st.fullDownloading = false
		switch {
		case st.fullRetryDelay <= 0:
			st.fullRetryDelay = 20 * time.Second
		case st.fullRetryDelay < 10*time.Minute:
			st.fullRetryDelay *= 2
		}
	}
}

// --- what the reader is told, and whether it is true -------------------------

// refreshObs is the state plus what every surface says about it.
type refreshObs struct {
	facts  refreshFacts
	notice string // the picker footer
	banner bool   // the "showing the Gospels" banner would draw
	// truth
	onSeed bool // is the text on screen ACTUALLY the seed?
}

func observe(st *AppState, onSeed bool) refreshObs {
	return refreshObs{
		facts:  factsOf(st),
		notice: fullPendingNotice(st),
		banner: st.seedOnly && st.CurrentVersion == defaultVersionID,
		onSeed: onSeed,
	}
}

type pinnedRefreshDefect struct {
	name   string
	what   string
	covers func(o refreshObs) bool
}

// knownRefreshIncoherent — every incoherent state of the refresh machine
// reachable TODAY, by the name docs/VERSION_STATES.md gives it.
// D4 was struck when applyFullDownload learned to clear seedOnly on the
// switched-away path too, and D5 when the picker read its notice before
// firing the manual retry. No trajectory reaches an incoherent state today.
var knownRefreshIncoherent = []pinnedRefreshDefect{}

// The refresh machine's invariants.
//
//	R-A  No surface claims the seed while the complete text is on screen.
//	R-B  A reader waiting offline is told they are waiting, not that work is
//	     in progress.
//	R-C  Nothing is owed once the download has landed.
func checkRefreshInvariants(o refreshObs) []string {
	var bad []string
	if o.banner && !o.onSeed {
		bad = append(bad, "R-A: the seed banner draws over the complete text")
	}
	// The seed wording legitimately wins while the reader IS on the seed: it
	// carries the more important fact (the text is partial), so a backoff
	// under the seed is not a violation.
	if !o.facts.seedOnly && o.facts.pending && o.facts.retryDelay > 0 && !o.facts.downloading &&
		o.notice != "" && !strings.Contains(o.notice, "waiting for a connection") {
		bad = append(bad, "R-B: a waiting reader is told the update is in progress")
	}
	return bad
}

// --- enumeration C: the refresh cross-product --------------------------------

func TestVersionRefreshStateSpace(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	var unexplained []string
	seen := map[string]bool{}
	cells := 0

	for _, pending := range []bool{true, false} {
		for _, seed := range []bool{true, false} {
			for _, downloading := range []bool{true, false} {
				for _, delay := range []time.Duration{0, 20 * time.Second} {
					for _, current := range []string{defaultVersionID, "bsb"} {
						f := refreshFacts{pending, seed, downloading, delay, current}
						for ev := reDownloadLands; ev <= reFetchFails; ev++ {
							name := fmt.Sprintf("%s/%s", f, ev)
							t.Run(name, func(t *testing.T) {
								st := f.toState(t)
								// The text on screen is the seed exactly while
								// seedOnly is set AND nothing has replaced it.
								onSeed := f.seedOnly
								ev.apply(t, st)
								if ev == reDownloadLands {
									// applyFullDownload writes the full text into
									// loadedVersions[version.ID] unconditionally, so
									// the seed is gone from that slot even when the
									// reader is away — switching back serves the
									// complete text.
									onSeed = false
								}
								obs := observe(st, onSeed)
								cells++

								for _, bad := range checkRefreshInvariants(obs) {
									explained := false
									for _, d := range knownRefreshIncoherent {
										if d.covers(obs) {
											seen[d.name] = true
											explained = true
											break
										}
									}
									if !explained {
										unexplained = append(unexplained,
											fmt.Sprintf("%s: %s", name, bad))
									}
								}
							})
						}
					}
				}
			}
		}
	}

	sort.Strings(unexplained)
	for _, u := range dedupe(unexplained) {
		t.Errorf("unpinned incoherent state — %s\n"+
			"Name it in docs/VERSION_STATES.md and pin it in knownRefreshIncoherent, "+
			"or the invariant is wrong.", u)
	}
	t.Logf("refresh space: %d cells, %d pinned defects reached", cells, len(seen))
}

func dedupe(in []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// --- the TRAJECTORY harness --------------------------------------------------

// TestVersionRefreshTrajectories walks JOURNEYS rather than cells: every
// sequence of events up to depth 4 from the app's real starting states, with
// the invariants asserted after EVERY step.
//
// This is the instrument that sees what a cross-product cannot. D4 needs four
// events in order — fresh install on the seed, switch away, the download
// lands while away, switch back — and each of those steps is individually
// correct. Only the composition is wrong, and only a walk over compositions
// finds it.
func TestVersionRefreshTrajectories(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	// The two states a launch can really produce for this machine.
	starts := []struct {
		name  string
		facts refreshFacts
		seed  bool
	}{
		{"fresh-install-on-seed", refreshFacts{pending: true, seedOnly: true, current: defaultVersionID}, true},
		{"stale-epoch-boot", refreshFacts{pending: true, current: defaultVersionID}, false},
	}

	const depth = 4
	var unexplained []string
	seen := map[string]bool{}
	journeys := 0

	var walk func(t *testing.T, st *AppState, onSeed bool, path []string)
	walk = func(t *testing.T, st *AppState, onSeed bool, path []string) {
		if len(path) == depth {
			return
		}
		for ev := reDownloadLands; ev <= reFetchFails; ev++ {
			// Each branch gets its own state: a journey must not inherit a
			// sibling's mutations, and AppState carries an atomic so it
			// cannot simply be copied.
			nextFacts := factsOf(st)
			branch := nextFacts.toState(t)
			nextSeed := onSeed
			ev.apply(t, branch)
			if ev == reDownloadLands {
				nextSeed = false // the full text is in the slot from this moment on
			}
			here := append(append([]string{}, path...), ev.String())
			journeys++

			obs := observe(branch, nextSeed)
			for _, bad := range checkRefreshInvariants(obs) {
				explained := false
				for _, d := range knownRefreshIncoherent {
					if d.covers(obs) {
						seen[d.name] = true
						explained = true
						break
					}
				}
				if !explained {
					unexplained = append(unexplained,
						fmt.Sprintf("%s → %s", strings.Join(here, " → "), bad))
				}
			}
			walk(t, branch, nextSeed, here)
		}
	}

	for _, s := range starts {
		t.Run(s.name, func(t *testing.T) {
			walk(t, s.facts.toState(t), s.seed, []string{s.name})
		})
	}

	sort.Strings(unexplained)
	for _, u := range dedupe(unexplained) {
		t.Errorf("unpinned incoherent TRAJECTORY — %s\n"+
			"Every step is legal on its own; the composition is not. Name it in "+
			"docs/VERSION_STATES.md and pin it, or the invariant is wrong.", u)
	}
	for _, d := range knownRefreshIncoherent {
		if !seen[d.name] {
			t.Errorf("%s is pinned but no trajectory reaches it any more (%s).\n"+
				"Strike it from knownRefreshIncoherent AND from docs/VERSION_STATES.md.",
				d.name, d.what)
		}
	}
	t.Logf("trajectories: %d walked to depth %d, %d pinned defects reached", journeys, depth, len(seen))
}

// D5 is not a state that is wrong — it is a state the reader can never be
// SHOWN. The picker is the only surface that displays the notice, and the
// picker fires the manual retry before computing it, so the wording written
// for a reader waiting offline was unreachable from the one place it appears.
// A reachability property needs its own assertion: no single cell is
// incoherent, the SPACE is.
func TestTheWaitingNoticeIsReachableFromThePicker(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	// A reader waiting offline on a stale-epoch boot: pending, not on the
	// seed, backoff armed, nothing in flight.
	st := refreshFacts{pending: true, retryDelay: 20 * time.Second, current: defaultVersionID}.toState(t)
	// The manual retry inside noticeOnPickerOpen would otherwise spawn a real
	// download goroutine and reach the network. stopping is triggerFullDownload's
	// own first guard, so this exercises the ORDERING under test and nothing
	// else — the notice is captured before the retry is even attempted.
	st.stopping.Store(true)

	// The control: before the picker is involved, the wording exists.
	if before := fullPendingNotice(st); !strings.Contains(before, "waiting for a connection") {
		t.Fatalf("control: the waiting wording must exist at all, else this proves nothing: %q", before)
	}

	got := noticeOnPickerOpen(st)
	if !strings.Contains(got, "waiting for a connection") {
		t.Errorf("D5: opening the picker while waiting offline reports %q.\n"+
			"The manual retry fires before the notice is computed, so the wording "+
			"written for exactly this reader can never be shown by the only surface "+
			"that shows it.", got)
	}
}

// D3 is a COUPLING defect: M1 knows a version is serving a superseded epoch,
// M3 only ever refreshes and announces the DEFAULT one. A reader restored
// onto another translation offline read the previous decoder's output with no
// notice, no banner and no upgrade for the whole session — the same silence
// V1 was, in a place V1's fix does not reach.
func TestANonDefaultStaleVersionIsNotSilent(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := &AppState{CurrentVersion: "bsb", loadedVersions: map[string]*BibleData{}}

	// The control: with nothing stale, the picker says nothing. Otherwise a
	// notice that always fires would pass this test without meaning anything.
	if n := fullPendingNotice(st); n != "" {
		t.Fatalf("control: a settled reader must be told nothing, got %q", n)
	}

	markVersionStale(st, "bsb")
	notice := fullPendingNotice(st)
	if notice == "" {
		t.Fatal("D3: a translation serving a previous edition says nothing at all")
	}
	if !strings.Contains(notice, "Berean Standard Bible") {
		t.Errorf("the notice must name the reader's OWN translation, got %q", notice)
	}

	// And it is repaired by the only thing that repairs it — that version
	// actually loading its current epoch.
	clearVersionStale(st, "bsb")
	if n := fullPendingNotice(st); n != "" {
		t.Errorf("a repaired version must stop being announced, got %q", n)
	}
}

// The stale record must never displace the seed's own message: a reader on the
// four-book seed needs to hear about the seed first.
func TestTheSeedNoticeStillWinsForASeedReader(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := refreshFacts{pending: true, seedOnly: true, current: defaultVersionID}.toState(t)
	if n := fullPendingNotice(st); !strings.Contains(n, "starter portion") {
		t.Fatalf("control: a seed reader must hear about the seed, got %q", n)
	}
}

// And the precedence holds when BOTH are true: the seed is what the reader is
// looking at, so it is what they are told about.
func TestSeedPrecedesAStaleOtherVersion(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := refreshFacts{pending: true, seedOnly: true, current: defaultVersionID}.toState(t)
	markVersionStale(st, "bsb")
	if n := fullPendingNotice(st); !strings.Contains(n, "starter portion") {
		t.Errorf("a reader on the seed must hear about the seed first, got %q", n)
	}
}
