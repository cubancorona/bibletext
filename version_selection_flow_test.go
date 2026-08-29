package bibletext

// M4, the active-selection machine. See docs/VERSION_STATES.md.
//
// WHY ACTIVE SELECTION IS DIFFERENT FROM THE REST. Every other machine here describes something
// on disk or something the app was told. This one is the only place where the
// app keeps a SECOND copy of a translation — the in-memory loadedVersions map
// — and a second copy is a second source of truth. Everything downstream reads
// through it: the text on screen, the citation that names the translation to
// someone else, the notice that says whether the edition is current. So the
// question is not "does the map work" but "can the map and the disk ever
// disagree about the same translation, and does anything notice".

import (
	"fmt"
	"sort"
	"testing"

	"fyne.io/fyne/v2/test"
)

// --- the axes ---------------------------------------------------------------

// memoryShape is what loadedVersions holds for the version being selected.
type memoryShape int

const (
	memAbsent      memoryShape = iota // never loaded this session
	memCurrentGen                     // the current epoch's decode
	memPreviousGen                    // a PREVIOUS epoch's decode, from the offline fallback
)

func (m memoryShape) String() string {
	return [...]string{"mem-absent", "mem-current", "mem-previous"}[m]
}

// diskShape is what is on disk for that same version, at the same moment.
type diskShape int

const (
	diskAbsent diskShape = iota
	diskCurrentGen
	diskPreviousGen
)

func (d diskShape) String() string {
	return [...]string{"disk-absent", "disk-current", "disk-previous"}[d]
}

type selectObs struct {
	mem  memoryShape
	disk diskShape

	switched  bool   // did the switch take?
	onScreen  string // CurrentVersion after
	served    string // the generation stamp of the text actually in state.Bible
	mapAgrees bool   // is state.Bible exactly loadedVersions[CurrentVersion]?
	staleMark bool   // is the version recorded as showing a previous edition?
	notice    string // what the picker footer would say
}

type pinnedSelectDefect struct {
	name   string
	what   string
	covers func(o selectObs) bool
}

// knownSelectIncoherent — every incoherent state of the selection machine
// reachable TODAY, by the name docs/VERSION_STATES.md gives it. Set equality
// asserted, so a fix that leaves a pin behind fails the suite.
var knownSelectIncoherent = []pinnedSelectDefect{}

// The selection machine's invariants.
//
//	S-A  The text on screen IS the map's entry for the version on screen.
//	     Two copies, one truth.
//	S-B  A reader shown a previous edition is recorded as being shown one.
//	     The record is what every notice is computed from, so a record that
//	     disagrees with the screen is a notice that lies.
//	S-C  A switch that cannot be served leaves the reader where they were,
//	     never blank.
func checkSelectInvariants(o selectObs) []string {
	var bad []string
	if o.switched && !o.mapAgrees {
		bad = append(bad, "S-A: the text on screen is not the map's entry for the version on screen")
	}
	if o.switched && o.served == "previous" && !o.staleMark {
		bad = append(bad, "S-B: a previous edition is on screen and nothing records it")
	}
	if o.switched && o.served == "current" && o.staleMark {
		bad = append(bad, "S-B: the current edition is on screen and the app still calls it stale")
	}
	if o.onScreen == "" {
		bad = append(bad, "S-C: the switch left the reader with no translation")
	}
	return bad
}

// --- enumeration E: the selection cross-product ------------------------------

func TestVersionSelectionStateSpace(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	var unexplained []string
	seen := map[string]bool{}
	cells, skipped := 0, 0

	for mem := memAbsent; mem <= memPreviousGen; mem++ {
		for disk := diskAbsent; disk <= diskPreviousGen; disk++ {
			name := fmt.Sprintf("%s/%s", mem, disk)
			t.Run(name, func(t *testing.T) {
				obs, ok := runSelectCell(t, mem, disk)
				if !ok {
					skipped++
					return
				}
				cells++
				for _, bad := range checkSelectInvariants(obs) {
					explained := false
					for _, d := range knownSelectIncoherent {
						if d.covers(obs) {
							seen[d.name] = true
							explained = true
							break
						}
					}
					if !explained {
						unexplained = append(unexplained, fmt.Sprintf("%s: %s", name, bad))
					}
				}
			})
		}
	}

	if len(unexplained) > 0 {
		sort.Strings(unexplained)
		t.Errorf("%d selection cells; %d incoherent states with no entry in the register:\n  %v",
			cells, len(unexplained), unexplained)
	}
	for _, d := range knownSelectIncoherent {
		if !seen[d.name] {
			t.Errorf("%s is pinned as reachable but no cell reached it — if it is fixed, strike it from knownSelectIncoherent and from docs/VERSION_STATES.md: %s", d.name, d.what)
		}
	}
	t.Logf("%d selection cells enumerated (%d unserveable, skipped); %d pinned incoherent states reached", cells, skipped, len(seen))
}

// runSelectCell puts one version into the given memory/disk shapes and drives
// the REAL switch. It reports ok=false for the cells where no text exists at
// all — an offline switch to a version with nothing anywhere is M1's question,
// answered there, not a selection state.
func runSelectCell(t *testing.T, mem memoryShape, disk diskShape) (selectObs, bool) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("BIBLETEXT_CACHE_PATH", dir+"/bibletext-cache.json")

	// A version whose fetch always fails, so nothing in this cell can reach
	// the network and every answer comes from memory or disk.
	sel := BibleVersion{
		ID: "sel", Name: "Selection Fixture", Abbrev: "SEL",
		Publisher: "Public Domain", PublicDomain: true,
		cacheEpoch: 2,
		source:     &fakeVersionSource{avail: true, err: fmt.Errorf("offline")},
	}
	withRegisteredVersion(t, sel)

	base := stampedBible("base")
	state := &AppState{
		Bible:          base,
		CurrentVersion: defaultVersionID,
		currentMode:    modeReal,
		loadedVersions: map[string]*BibleData{defaultVersionID: base},
		loadPhase:      loadReady,
	}

	switch disk {
	case diskCurrentGen:
		mustCache(t, cachePathForVersion(sel.ID), stampedBible("current"))
	case diskPreviousGen:
		paths := supersededCachePaths(sel)
		if len(paths) == 0 {
			t.Fatal("the fixture must have a previous epoch, or this axis is empty")
		}
		mustCache(t, paths[0], stampedBible("previous"))
	}

	switch mem {
	case memCurrentGen:
		state.loadedVersions[sel.ID] = stampedBible("current")
	case memPreviousGen:
		state.loadedVersions[sel.ID] = stampedBible("previous")
		// This is how the app itself arrives here: the offline fallback served
		// a previous epoch and recorded that it did (D3).
		markVersionStale(state, sel.ID)
	}

	switchVersion(state, sel.ID)

	obs := selectObs{mem: mem, disk: disk, onScreen: state.CurrentVersion}
	obs.switched = state.CurrentVersion == sel.ID
	if !obs.switched && mem == memAbsent && disk == diskAbsent {
		return obs, false // nothing to serve anywhere: not a selection state
	}
	// The stamp is read DIRECTLY, not through M1's servedFrom vocabulary:
	// that helper maps unknown stamps to "none", so routing this axis through
	// it silently turned every previous-edition assertion below into a
	// tautology. A check that cannot fail is not a check.
	obs.served = bibleStamp(state.Bible)
	obs.mapAgrees = state.Bible == state.loadedVersions[state.CurrentVersion]
	obs.staleMark = state.staleVersions[sel.ID]
	obs.notice = fullPendingNotice(state)
	return obs, true
}

// TestTheSelectionInvariantsCanActuallyFail is the control for the enumeration
// above. Its S-B arms compare a stamp against two literals, and a mismatch
// between the fixture's vocabulary and the invariant's would make every one of
// those arms unfireable while the suite stayed green — which is precisely how
// this file read before the stamp was taken directly. So: hand the invariants
// the two shapes they exist to catch, and require them to complain.
func TestTheSelectionInvariantsCanActuallyFail(t *testing.T) {
	stale := selectObs{switched: true, onScreen: "sel", served: "previous", mapAgrees: true}
	if len(checkSelectInvariants(stale)) == 0 {
		t.Fatal("S-B does not fire on a previous edition served with no record of it — every previous-edition cell in the enumeration is a tautology")
	}
	lying := selectObs{switched: true, onScreen: "sel", served: "current", mapAgrees: true, staleMark: true}
	if len(checkSelectInvariants(lying)) == 0 {
		t.Fatal("S-B does not fire on a stale record left over the current edition")
	}
	split := selectObs{switched: true, onScreen: "sel", served: "current", mapAgrees: false}
	if len(checkSelectInvariants(split)) == 0 {
		t.Fatal("S-A does not fire when the screen and the map disagree")
	}
}

// TestAStaleVersionUpgradesInPlaceWhenTheCurrentEpochArrives is D11 — the one
// the selection cross-product found, at mem-previous/disk-current.
//
// ON REACHABILITY, PLAINLY. No path in the app today writes a NON-default
// version's current epoch while that version's previous decode is sitting in
// loadedVersions: the background refresh only ever upgrades the default
// translation, and a version already in memory is never re-read from disk. So
// this is a guard, in D7's sense, not a live defect — and it is a much shorter
// reach than D7's. The feature that makes it live is the obvious next one, and
// the notice D3 added already promises it ("until the update can be
// downloaded"). The fix also closes a liveness hole that IS live: before it, a
// non-default translation recorded as stale had no way to stop being stale
// within a session, however much of it the reader spent online.
func TestAStaleVersionUpgradesInPlaceWhenTheCurrentEpochArrives(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	t.Setenv("BIBLETEXT_CACHE_PATH", t.TempDir()+"/bibletext-cache.json")

	sel := BibleVersion{
		ID: "sel", Name: "Selection Fixture", Abbrev: "SEL",
		Publisher: "Public Domain", PublicDomain: true,
		cacheEpoch: 2,
		source:     &fakeVersionSource{avail: true, err: fmt.Errorf("offline")},
	}
	withRegisteredVersion(t, sel)

	base := stampedBible("base")
	state := &AppState{
		Bible:          base,
		CurrentVersion: defaultVersionID,
		currentMode:    modeReal,
		loadedVersions: map[string]*BibleData{
			defaultVersionID: base,
			// The offline fallback's decode, from an earlier moment in this
			// same session, with the record it left behind.
			sel.ID: stampedBible("previous"),
		},
		loadPhase: loadReady,
	}
	markVersionStale(state, sel.ID)

	// CONTROL: while the disk is ALSO on the previous epoch there is nothing to
	// upgrade to, and the reader must stay where they are with the notice
	// intact. Without this arm the test would pass on a change that simply
	// dropped the memory cache entirely.
	prevPaths := supersededCachePaths(sel)
	if len(prevPaths) == 0 {
		t.Fatal("the fixture needs a previous epoch")
	}
	mustCache(t, prevPaths[0], stampedBible("previous"))
	switchVersion(state, sel.ID)
	if got := bibleStamp(state.Bible); got != "previous" {
		t.Fatalf("control: with nothing better on disk the reader keeps the previous edition; got %q", got)
	}
	if !state.staleVersions[sel.ID] {
		t.Fatal("control: the notice must survive while the previous edition is still all there is")
	}
	if n := fullPendingNotice(state); n == "" {
		t.Fatal("control: a previous edition on screen must still be announced")
	}

	// Now the current epoch lands on disk. The fetch still fails, so the only
	// way to the new text is the cache — which is the point.
	mustCache(t, cachePathForVersion(sel.ID), stampedBible("current"))
	state.CurrentVersion = defaultVersionID
	state.Bible = base
	switchVersion(state, sel.ID)

	if got := bibleStamp(state.Bible); got != "current" {
		t.Fatalf("the current edition was on disk and the reader was left on %q", got)
	}
	if state.staleVersions[sel.ID] {
		t.Fatal("the version upgraded and is still recorded as stale")
	}
	if n := fullPendingNotice(state); n != "" {
		t.Fatalf("the notice outlived the condition it describes: %q", n)
	}
}

// TestAnInMemorySwitchAlwaysTakes pins a coupling that D11's fix put weight
// on, at the far end of the codebase from where the fix lives.
//
// switchToLinkVersion treats "already in memory" as "switches synchronously
// and cannot fail": it calls switchVersion and returns FALSE, which tells
// applyShareTarget to carry on and open the passage in whatever translation is
// now current. Before D11, in-memory always meant a map read, so that was free.
// D11 made switchVersion able to take the LOAD path for an in-memory version —
// deliberately, to pick up an epoch that has since arrived on disk — and a load
// that failed there would leave the reader in their old translation while the
// link went on to open the passage as if the switch had happened. That is the
// silent downgrade share_link_open.go was written to prevent, arriving through
// a door D11 opened.
//
// It cannot happen, and the reason is worth pinning rather than remembering:
// the reload is gated on versionCacheIsCurrent, which since V1 does not stat
// the cache but LOADS it. The load switchVersion then performs reads the same
// file that check just decoded. The guard is not "probably fine", it is the
// same successful read twice.
func TestAnInMemorySwitchAlwaysTakes(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	t.Setenv("BIBLETEXT_CACHE_PATH", t.TempDir()+"/bibletext-cache.json")

	sel := BibleVersion{
		ID: "sel", Name: "Selection Fixture", Abbrev: "SEL",
		Publisher: "Public Domain", PublicDomain: true,
		cacheEpoch: 2,
		source:     &fakeVersionSource{avail: true, err: fmt.Errorf("offline")},
	}
	withRegisteredVersion(t, sel)

	base := stampedBible("base")
	for _, stale := range []bool{false, true} {
		for _, onDisk := range []bool{false, true} {
			name := fmt.Sprintf("stale=%v disk-current=%v", stale, onDisk)
			t.Run(name, func(t *testing.T) {
				t.Setenv("BIBLETEXT_CACHE_PATH", t.TempDir()+"/bibletext-cache.json")
				if onDisk {
					mustCache(t, cachePathForVersion(sel.ID), stampedBible("current"))
				}
				state := &AppState{
					Bible:          base,
					CurrentVersion: defaultVersionID,
					currentMode:    modeReal,
					loadedVersions: map[string]*BibleData{
						defaultVersionID: base,
						sel.ID:           stampedBible("previous"),
					},
					loadPhase: loadReady,
				}
				if stale {
					markVersionStale(state, sel.ID)
				}
				switchVersion(state, sel.ID)
				if state.CurrentVersion != sel.ID {
					t.Fatalf("an in-memory switch did not take — switchToLinkVersion would now open a shared link in %q with no message at all", state.CurrentVersion)
				}
			})
		}
	}
}

// TestASwitchDoesNotLeaveTheOldTranslationsSearchResults is D15.
//
// Two failures with one cause, and the second is the durable one. A search
// result list is made of Verses carrying the OLD translation's wording; it
// survives a switch, so the reader reads one translation's text under another
// translation's name. And tapping a row navigated by the old numbering into a
// canon that need not contain the book at all — a blank chapter with both
// arrows dead, written into the reading position and the recent-chapters
// history where it outlives the session.
//
// The mark beside those results was ALREADY renumbered through the anchor
// machinery on every switch, for exactly this reason. The results were the
// half nothing re-derived.
func TestASwitchDoesNotLeaveTheOldTranslationsSearchResults(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	wide := widerCanonBible()
	narrow := fullValidBible()
	web, _ := versionByID(defaultVersionID)

	newState := func() *AppState {
		return &AppState{
			Bible: wide, CurrentVersion: "webc", currentMode: modeReal,
			loadedVersions:    map[string]*BibleData{"webc": wide},
			loadPhase:         loadReady,
			CurrentBook:       "Tobit",
			CurrentChapter:    1,
			ActiveSearchQuery: "sample",
			SearchResults: []Verse{
				{BookName: "Tobit", Book: "Tobit", Chapter: 1, Verse: 1, Text: "wide Tobit 1:1 sample."},
			},
		}
	}

	// CONTROL: the fixture really does hold a result the narrow canon cannot
	// contain, and the narrow canon really lacks the book. Without both, the
	// assertions below would pass against anything.
	if narrow.GetChaptersForBook("Tobit") != 0 {
		t.Fatal("control: the narrow canon must not contain Tobit")
	}
	if len(newState().SearchResults) == 0 {
		t.Fatal("control: the fixture must hold a result")
	}

	st := newState()
	applyLoadedVersion(st, web, narrow, modeReal)
	for _, r := range st.SearchResults {
		if st.Bible.GetChaptersForBook(r.BookName) == 0 {
			t.Fatalf("a result for %s survived the switch into a translation without that book", r.BookName)
		}
	}

	// And the guard on the navigation itself, which protects every other
	// producer of a Verse — the AI list, and anything added later.
	st = newState()
	st.Bible, st.CurrentVersion = narrow, defaultVersionID
	stale := Verse{BookName: "Tobit", Book: "Tobit", Chapter: 1, Verse: 1}
	before := st.CurrentBook
	openSearchResultRange(st, stale, 0)
	if st.CurrentBook != before {
		t.Fatalf("navigated to %s, a book this translation does not contain", st.CurrentBook)
	}
	for _, h := range st.RecentChapters {
		if h.Book == "Tobit" {
			t.Fatal("a dead reference was written into the durable history")
		}
	}
	if msg := searchResultOutsideCanonMessage(st, "Tobit"); msg == "" {
		t.Fatal("the tap was refused in silence, which reads as the tap having missed")
	}
	// The question shape: nothing to say when the book IS present, or a proof
	// that asks whether anything was said reads every state as answered.
	if msg := searchResultOutsideCanonMessage(st, "Genesis"); msg != "" {
		t.Fatalf("spoke about a book the reader has: %q", msg)
	}
}
