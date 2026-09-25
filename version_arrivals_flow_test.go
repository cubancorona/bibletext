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
// THE DISK DECIDES ONE ARRIVAL, so the journeys walk three of them. A link's
// fetch that fails is not one outcome but two, and the reader's disk picks:
// with the translation's previous edition cached, the load serves that and
// records it as stale; with nothing, the reader is shown the error. With the
// current edition cached nothing is fetched at all. Each journey is walked on
// each of those disks, and the disk is real — applyLoadedVersion and the load
// tail read it, and a landing rewrites it.
//
// AN ARRIVAL ALSO MEETS WHAT THE READER BRINGS. Each disk is walked twice:
// with nothing remembered, and with the reader's licensed translation
// remembered, as the launch's restore records it when it cannot open that
// translation (D9). And the reader's own choice of translation can still be
// loading when a link arrives, which is the one moment the app has two
// arrivals in hand: each load carries who started it, as an argument, to its
// own landing (D19).
//
// AND THE REFRESH WORKS UNDERNEATH. A previous edition served to an arrival
// is owed its upgrade while the app runs (D17), so the walk has the upgrade
// landing as an event of its own, and asks after every step that what is owed
// is on its way.
//
// The cross-product that DOES cover arrivals already exists and is a different
// question: share_link_flow_test.go asks "is any state a dead end", over the
// link's own axes. This one asks what the arrival does to, and learns from,
// the VERSION machinery — the half docs/VERSION_STATES.md records as unmodelled.

import (
	"errors"
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
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

	// Which edition of the translation on screen IS on screen, read off the
	// text itself ("current" or "previous") rather than off any record — the
	// record is one of the things being checked.
	edition string
	stale   bool   // the translation on screen is RECORDED as showing a previous edition
	notice  string // the picker footer (fullPendingNotice), where a stale serve is told

	// preferred is the translation the reader chose when the launch had to
	// show another (preferredVersion): D9's record, which D10's sentence reads
	// and the next save writes.
	preferred string
	// landed is who put a translation on screen in the step just taken:
	// "reader" for a choice of the reader's own, "arrival" for a link's,
	// "upgrade" for the refresh's edition swap, "" when nothing landed. The
	// walk knows it from the event that started the load, never from the
	// app: who asked is exactly what the app is being checked on, and no
	// field records it (D19).
	landed string

	// owed is whether the refresh owes an upgrade (owedUpgrades), and
	// onItsWay whether one is fetching or a retry is really armed — a
	// callback the backoff handed its timer and nothing has fired yet, with
	// the delay that says so: A-H asks that the first never holds without
	// the second.
	owed     bool
	onItsWay bool

	// stuck is set by the liveness pass and nowhere else: a previous edition
	// is on screen and nothing the walk did from here brought the current one.
	stuck bool
}

func (f arrivalFacts) String() string {
	s := f.current + "@" + f.loc
	if f.edition == "previous" {
		s += "+previous"
	}
	if f.stale {
		s += "+stale"
	}
	if f.preferred != "" {
		s += "+remembers(" + f.preferred + ")"
	}
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
	if f.owed {
		s += "+upgrade-owed"
	}
	if f.onItsWay {
		s += "+upgrade-on-its-way"
	}
	if f.stuck {
		s += "+stuck"
	}
	return s
}

// --- the disk ---------------------------------------------------------------

// arrivalDisk is what is on disk for the link's translation when a journey
// starts. The default translation and the one the reader wanders off to always
// have their current edition cached; the link's is the one whose disk decides
// how an arrival ends.
type arrivalDisk int

const (
	arDiskCurrent  arrivalDisk = iota // its current edition, as a landed fetch leaves it
	arDiskPrevious                    // only the edition before it: an upgrader's, after a cacheEpoch bump
	arDiskNone                        // nothing: it has never been opened on this device
)

func (d arrivalDisk) String() string {
	return [...]string{"disk-current", "disk-previous-only", "disk-none"}[d]
}

// arrivalSource stands in for a translation's real source, which would fetch
// from the network. The network is up unless a failed fetch says otherwise.
type arrivalSource struct {
	offline bool
	fetches int
}

func (s *arrivalSource) available() bool { return true }

func (s *arrivalSource) fetch() (*BibleData, error) {
	s.fetches++
	if s.offline {
		return nil, errors.New("offline")
	}
	return stampedBible("current"), nil
}

// arrivalWorld is the disk the journeys walk, the source the link's
// translation is fetched from, what the reader brings to the journey, and the
// three doors the app's background work leaves through. A landing rewrites
// the disk, so every journey puts it back before it replays.
type arrivalWorld struct {
	disk     arrivalDisk
	src      *arrivalSource
	current  []byte // the link's translation's current edition, as the real writer leaves it
	previous []byte // its previous edition, likewise

	// preferred is the translation every journey starts out remembering, ""
	// for none: the record the restore makes when the launch cannot open the
	// reader's licensed translation (D9), which adoptLaunch carries to the live
	// state (D18). The journeys that start with it walk the app a launch that
	// fell back leaves.
	preferred string

	// The load in flight: which translation, the tail the real
	// switchVersionInteractive handed startVersionLoad — which carries the
	// cause the real call captured, so the walk never supplies one — and
	// whether the reader started it, which is the walk's own knowledge of
	// what it did and what the invariants check the app against. "" and nil
	// when nothing is loading.
	inflight       string
	inflightLand   func(*BibleData, dataMode, error)
	inflightReader bool

	// The refresh's two doors: the callback the backoff handed its timer,
	// not yet fired, and a fetch the real triggerFullDownload started, not
	// yet landed. nil when there is none.
	armed   func()
	upgrade *heldFetch
}

// heldFetch is a fetch the app started through one of its doors, held by the
// test: what it fetches, and the tail the app built to land it.
type heldFetch struct {
	v    BibleVersion
	land func(*BibleData, dataMode, error)
}

// inflightOwner is who a landing of the load in flight puts on screen.
func (w *arrivalWorld) inflightOwner() string {
	if w.inflightReader {
		return "reader"
	}
	return "arrival"
}

// newArrivalWorld builds the disk under a cache directory of the test's own
// and hands the two translations a journey can load a source the walk
// controls. The swap is also the guarantee that nothing a journey drives
// reaches the network: their real sources fetch from bible.helloao.org.
//
// And it takes the doors the app's background work leaves through, for the
// test's duration: an interactive load (startVersionLoad), the refresh's
// fetch (startUpgradeFetch) and its retry timer (upgradeRetryAfter). What the
// app starts through them is held, and the walk lands it at a step of its
// choosing through the tail the app built, so what is on the far side of a
// goroutine — the cause a load carries, what the refresh chose to fetch, what
// its timer does when it fires — is the app's own, never a copy of it.
func newArrivalWorld(t *testing.T, disk arrivalDisk) *arrivalWorld {
	t.Helper()
	t.Setenv("BIBLETEXT_CACHE_PATH", filepath.Join(t.TempDir(), cacheFileName))
	w := &arrivalWorld{disk: disk, src: &arrivalSource{}}
	withVersionSource(t, arrivalLinkVersion, w.src)
	// The other translation is only ever read from its cache, so its source
	// is never online: a journey that fetched it would fail rather than pass.
	withVersionSource(t, arrivalOtherVersion, &arrivalSource{offline: true})

	prevLoad, prevFetch, prevRetry := startVersionLoad, startUpgradeFetch, upgradeRetryAfter
	startVersionLoad = func(v BibleVersion, _ *BibleData, land func(*BibleData, dataMode, error)) {
		if w.inflightLand != nil {
			t.Errorf("a load of %s started while %s's was in flight: the single-flight guard did not hold", v.ID, w.inflight)
			return
		}
		w.inflight, w.inflightLand = v.ID, land
	}
	startUpgradeFetch = func(v BibleVersion, land func(*BibleData, dataMode, error)) {
		if w.upgrade != nil {
			t.Errorf("a fetch of %s started while %s's was in flight: the single-flight guard did not hold", v.ID, w.upgrade.v.ID)
			return
		}
		w.upgrade = &heldFetch{v: v, land: land}
	}
	upgradeRetryAfter = func(d time.Duration, f func()) {
		prevRetry(d, f) // still counted, as every test's is
		w.armed = f
	}
	t.Cleanup(func() { startVersionLoad, startUpgradeFetch, upgradeRetryAfter = prevLoad, prevFetch, prevRetry })

	for _, id := range []string{defaultVersionID, arrivalOtherVersion} {
		mustCache(t, cachePathForVersion(id), stampedBible("current"))
		if v, ok := versionByID(id); !ok || !versionCacheIsCurrent(v) {
			t.Fatalf("control: %s's seeded edition is not current, so this is not the world the journeys say they walk", id)
		}
	}
	// Both editions of the link's translation are written once, through the
	// real writer, and kept as bytes: reset lays them down again for every
	// journey without an fsync each time.
	link, _ := versionByID(arrivalLinkVersion)
	cur, prev := arrivalLinkPaths(t, link)
	mustCache(t, cur, stampedBible("current"))
	mustCache(t, prev, stampedBible("previous"))
	var err error
	if w.current, err = os.ReadFile(cur); err != nil {
		t.Fatal(err)
	}
	if w.previous, err = os.ReadFile(prev); err != nil {
		t.Fatal(err)
	}
	w.reset(t)
	return w
}

// arrivalLinkPaths is where the link's translation's current and previous
// editions live: cachePathForVersion and supersededCachePaths(v)[0], the two
// files the load and its fallback read.
func arrivalLinkPaths(t *testing.T, v BibleVersion) (current, previous string) {
	t.Helper()
	prev := supersededCachePaths(v)
	if len(prev) == 0 {
		t.Fatalf("%s has no previous edition to fall back on, so the disk axis is empty", v.ID)
	}
	return cachePathForVersion(v.ID), prev[0]
}

// reset puts the link's translation's disk back to the world's shape, the
// network back up, and nothing loading. A landing writes the current edition
// and purges the previous one, and every journey starts from the beginning.
func (w *arrivalWorld) reset(t *testing.T) {
	t.Helper()
	link, _ := versionByID(arrivalLinkVersion)
	cur, prev := arrivalLinkPaths(t, link)
	for _, p := range []string{cur, prev} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	switch w.disk {
	case arDiskCurrent:
		w.lay(t, cur, w.current)
	case arDiskPrevious:
		w.lay(t, prev, w.previous)
	}
	w.src.offline = false
	w.inflight, w.inflightLand, w.inflightReader = "", nil, false
	w.armed, w.upgrade = nil, nil
}

func (w *arrivalWorld) lay(t *testing.T, path string, b []byte) {
	t.Helper()
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// load is loadVersionData (versions.go) with the network up or down. A fetch
// that lands has its write done first, with the bytes the real writer wrote
// for that edition, so the load reads the current edition off the disk and
// then purges the previous one exactly as it does after a fetch. The walk
// lands a translation thousands of times, and an fsync each would take it
// from seconds to minutes; a failed fetch is the source's own.
func (w *arrivalWorld) load(t *testing.T, st *AppState, v BibleVersion, online bool) (*BibleData, dataMode, error) {
	t.Helper()
	if v.ID == arrivalLinkVersion {
		if online && !versionCacheIsCurrent(v) {
			cur, _ := arrivalLinkPaths(t, v)
			w.lay(t, cur, w.current)
		}
		w.src.offline = !online
		defer func() { w.src.offline = false }()
	} else if !versionCacheIsCurrent(v) {
		t.Fatalf("%s's current edition is not on disk, so this load would fetch it", v.ID)
	}
	return loadVersionData(v, st.baseBible())
}

// landInflight ends the load in flight: the load itself, with the network up
// or down, then the tail switchVersionInteractive handed startVersionLoad, run
// as fyne.Do would run it — the real tail, with the cause the real call
// captured, ending in finishVersionLoad. The load is over either way, so
// nothing is in flight after it.
func (w *arrivalWorld) landInflight(t *testing.T, st *AppState, online bool) {
	t.Helper()
	v, ok := versionByID(w.inflight)
	land := w.inflightLand
	if !ok || land == nil {
		t.Fatalf("control: no load is in flight to land (%q)", w.inflight)
	}
	w.inflight, w.inflightLand, w.inflightReader = "", nil, false
	data, mode, err := w.load(t, st, v, online)
	land(data, mode, err)
}

// landUpgrade ends the fetch the refresh started, with the network up,
// through the tail triggerFullDownload handed startUpgradeFetch (upgradeLanded).
// It reports what was fetched.
func (w *arrivalWorld) landUpgrade(t *testing.T, st *AppState) BibleVersion {
	t.Helper()
	f := w.upgrade
	if f == nil {
		t.Fatal("control: the refresh has no fetch in flight to land")
	}
	w.upgrade = nil
	data, mode, err := w.load(t, st, f.v, true)
	f.land(data, mode, err)
	return f.v
}

// withVersionSource hands a registered translation another source for the
// test's duration and leaves everything else about it — its id, its epoch,
// and so its cache paths — exactly as registered.
func withVersionSource(t *testing.T, id string, src bibleSource) {
	t.Helper()
	prev := registeredVersions
	next := append([]BibleVersion(nil), prev...)
	found := false
	for i := range next {
		if next[i].ID == id {
			next[i].source = src
			found = true
		}
	}
	if !found {
		t.Fatalf("%s is not registered", id)
	}
	registeredVersions = next
	t.Cleanup(func() { registeredVersions = prev })
}

// --- the events -------------------------------------------------------------

type arrivalEvent int

const (
	arLinkNamesOther           arrivalEvent = iota // a link naming another translation
	arFetchFails                                   // ...and its fetch fails, with no cache to fall back on
	arFetchFailsPreviousServes                     // ...or fails with the previous edition on disk, which serves
	arFetchLands                                   // ...or the load in flight lands, whoever asked for it
	arReaderPicksIt                                // the reader later chooses that same translation, and it works
	arReaderPicksOther                             // the reader chooses a different translation, and it lands
	arReaderStartsOther                            // the reader chooses a different translation, still loading at the next step
	arReaderStartsIt                               // the reader chooses the link's translation, still loading at the next step
	arUpdateLands                                  // the refresh's next attempt fetches what it owes, and it lands
)

func (e arrivalEvent) String() string {
	return [...]string{"link-names-other", "fetch-fails", "fetch-fails-previous-serves", "fetch-lands", "reader-picks-it", "reader-picks-other", "reader-starts-other", "reader-starts-it", "update-lands"}[e]
}

// the translation a link in these journeys names, and one for the reader to
// wander off to.
const (
	arrivalLinkVersion  = "webc"
	arrivalOtherVersion = "bsb"
)

// arrivalLinkTarget is the passage every link in these journeys opens.
var arrivalLinkTarget = ShareTarget{VersionID: arrivalLinkVersion, Book: "John", Chapter: 1, VerseLo: 16}

// apply drives ONE event through the app's own entry points, and reports
// whether it did anything and who put a translation on screen, if one landed.
// A link is applyShareTarget, a choice from the picker is
// switchVersionInteractive, and the refresh's next attempt is the callback
// its backoff armed. What they start on a goroutine leaves through a door the
// world holds (newArrivalWorld), and lands at the step that ends it, through
// the tail the app built; nothing the app decides is reproduced here.
//
// An event that does nothing is not walked on from. Every journey through it
// is a journey through the state before it, one step shorter, and is walked
// from there.
func (e arrivalEvent) apply(t *testing.T, w *arrivalWorld, st *AppState, told *bool) (did bool, landed string) {
	t.Helper()
	switch e {
	case arLinkNamesOther:
		if st.CurrentVersion == arrivalLinkVersion {
			return false, "" // switchToLinkVersion's own guard
		}
		loading := st.versionLoading
		if loading && w.inflight == arrivalLinkVersion && !w.inflightReader {
			// Tapped again while its own fetch runs: the park it makes is the
			// one already there.
			return false, ""
		}
		_, inMem := st.loadedVersions[arrivalLinkVersion]
		// The real link, end to end: applyShareTarget, and the
		// switchToLinkVersion it asks who navigates.
		applyShareTarget(st, arrivalLinkTarget)
		switch {
		case loading:
			// The reader's own load is running, so the link parked behind it
			// and started no load, so it gave no cause: the reader's load
			// lands with its own (D19). If it is another translation it takes
			// the screen, and the park is dropped and said (D14); if it is this
			// one, it opens the passage.
			*told = false
			return true, ""
		case inMem:
			// Already in memory: switched synchronously, and the passage
			// opened in whatever it now holds. Nothing on this branch fetches.
			return true, "arrival"
		}
		// Parked, and the load started through the door, carrying the cause
		// the link's call gave it.
		if w.inflight != arrivalLinkVersion || w.inflightLand == nil {
			t.Fatalf("control: the link started no load of %s (in flight %q)", arrivalLinkVersion, w.inflight)
		}
		w.inflightReader = false
		*told = false
		return true, ""
	case arFetchFails, arFetchFailsPreviousServes:
		if !st.versionLoading || w.inflightLand == nil {
			return false, ""
		}
		v, _ := versionByID(w.inflight)
		// The load reads the current edition before it fetches, so with one on
		// disk nothing is fetched and nothing can fail: that load lands.
		if versionCacheIsCurrent(v) {
			return false, ""
		}
		// Which arm a failed fetch takes is the disk's decision, not the
		// event's: the tail serves the previous edition if the cache-only read
		// finds one and shows the error if it does not. Each event is the
		// failure one disk allows, and nothing on a disk that allows the other.
		if _, _, cerr := loadVersionFromCacheOnly(v); (cerr == nil) != (e == arFetchFailsPreviousServes) {
			return false, ""
		}
		owner := w.inflightOwner()
		w.landInflight(t, st, false)
		if e == arFetchFailsPreviousServes {
			return true, owner
		}
		// Nothing on disk to serve, so the tail showed the error card
		// (showVersionLoadError), which is how the reader was told.
		*told = true
		return true, ""
	case arFetchLands:
		if !st.versionLoading || w.inflightLand == nil {
			return false, ""
		}
		owner := w.inflightOwner()
		w.landInflight(t, st, true)
		return true, owner
	case arReaderPicksIt, arReaderPicksOther:
		id := arrivalLinkVersion
		if e == arReaderPicksOther {
			id = arrivalOtherVersion
		}
		// A load owns the spinner's modal, and the picker is behind it.
		if st.versionLoading || st.CurrentVersion == id {
			return false, ""
		}
		// The picker's row: switchVersionInteractive with the cause the row
		// gives (read from the source, TestTheArrivalMarkBelongsToItsLoad). A
		// translation already in memory swaps synchronously; one that is not
		// starts its load, and it works — the network is up — so it lands in
		// this same step.
		switchVersionInteractive(st, id, byReader)
		if w.inflightLand != nil {
			w.inflightReader = true
			w.landInflight(t, st, true)
		}
		if st.CurrentVersion != id {
			t.Fatalf("control: the reader chose %s and is on %s", id, st.CurrentVersion)
		}
		return true, "reader"
	case arReaderStartsOther, arReaderStartsIt:
		id := arrivalOtherVersion
		if e == arReaderStartsIt {
			id = arrivalLinkVersion
		}
		if st.versionLoading || st.CurrentVersion == id {
			return false, ""
		}
		// In memory it swaps synchronously, which is reader-picks-other or
		// reader-picks-it.
		if _, inMem := st.loadedVersions[id]; inMem {
			return false, ""
		}
		// The spinner goes up and the load leaves through the door, and lands
		// at a later step. For the link's translation it is the reader's own
		// choice made offline, which a failed fetch serves from the previous
		// edition (D17), and the reader's own load that a link to the same
		// translation parks behind (D19).
		switchVersionInteractive(st, id, byReader)
		if w.inflight != id || w.inflightLand == nil {
			t.Fatalf("control: the reader's choice of %s started no load (in flight %q)", id, w.inflight)
		}
		w.inflightReader = true
		return true, ""
	case arUpdateLands:
		// The refresh's next attempt, with the network up: the retry its
		// backoff armed fires. The callback is the app's own, so what it runs —
		// triggerFullDownload, deciding what is owed and what to fetch first —
		// is the app's too; the fetch it starts is held at the door and lands
		// through the tail it built, upgradeLanded. A-H is what says a retry is
		// armed whenever an upgrade is owed and nothing is fetching.
		if w.armed == nil || st.fullDownloading {
			return false, ""
		}
		fire := w.armed
		w.armed = nil
		fire()
		if w.upgrade == nil {
			return false, "" // the retry fetched nothing
		}
		fetched := w.landUpgrade(t, st)
		// The tail chains to the next owed upgrade by starting its fetch.
		// Nothing else in this world can be owed, so a chained fetch here is a
		// refresh that did not settle what it fetched.
		if w.upgrade != nil {
			t.Fatalf("the refresh started a fetch of %s after %s landed; owed %v", w.upgrade.v.ID, fetched.ID, owedUpgrades(st))
		}
		return true, "upgrade"
	}
	return false, ""
}

func arrivalFactsOf(w *arrivalWorld, st *AppState, owed, told bool, landed string) arrivalFacts {
	return arrivalFacts{
		loc:         fmt.Sprintf("%s|%d", st.CurrentBook, st.CurrentChapter),
		parked:      st.pendingLink != nil,
		parkedFor:   st.pendingLinkVersion,
		current:     st.CurrentVersion,
		loading:     st.versionLoading,
		arrivalOwed: owed,
		failureTold: told,
		// Read DIRECTLY, never through servedFrom: that helper maps what it
		// does not know to "none", which is how an early selection walk passed
		// without checking anything (D11 in docs/VERSION_STATES.md).
		edition:   bibleStamp(st.Bible),
		stale:     st.staleVersions[st.CurrentVersion],
		notice:    fullPendingNotice(st),
		preferred: st.preferredVersion,
		landed:    landed,
		owed:      len(owedUpgrades(st)) > 0,
		onItsWay:  st.fullDownloading || (w.armed != nil && st.fullRetryDelay > 0),
	}
}

type pinnedArrivalDefect struct {
	name   string
	what   string
	covers func(bad string, prev, now arrivalFacts, ev arrivalEvent) bool
}

// knownArrivalIncoherent — every incoherent state of the arrivals layer the
// walk reaches, by the name docs/VERSION_STATES.md gives it. Set equality
// asserted, so a fix that leaves a pin behind fails the suite. Each pin names
// the invariant it explains, so it cannot swallow another one broken in the
// same state.
//
// D17, D19 and D20 were struck on 2026-09-25, in the change that fixed D18:
// the refresh owes a previous edition its upgrade, a load's cause travels
// with it, and the footer says every true fact. No journey reaches an
// incoherent state today.
var knownArrivalIncoherent = []pinnedArrivalDefect{}

// The arrivals invariants. All of them are about a PROMISE over time, which is
// why they are checked after every step of a journey rather than once.
//
//	A-A  A park exists only while something can still consume it. Once the
//	     load it was waiting on has failed AND the reader has been told, the
//	     promise is closed and the park must be closed with it.
//	A-B  The reader's location changes only from what they just did. A park
//	     retired earlier must never move them later, and a translation they
//	     chose lands where they were — unless a live link to that same
//	     translation was parked behind their load, whose passage it opens.
//	     An edition upgrade moves nothing: not the location, not the
//	     translation on screen, not what is remembered.
//	A-C  A journey never ends owing the reader an arrival: either the passage
//	     opened or something was said. So an arrival is never owed while
//	     nothing is in flight that could still honour it.
//	A-D  The record agrees with the screen. A previous edition on screen is
//	     recorded as stale, and a current one is not: the record is what the
//	     notice is computed from, so a record that disagrees with the screen
//	     is a notice that lies.
//	A-E  A previous edition on screen is never silent: the picker footer names
//	     the translation showing it AND says it is a previous edition. Naming
//	     it is not enough, because D10's sentence names the translation shown
//	     too, and says only that it was shown instead of another.
//	A-F  A previous edition on screen is not a one-way door: something the
//	     reader can do next, with the network up, brings the current edition
//	     while the app runs. Not a property of one state, so it is checked over
//	     the walk, by checkArrivalLiveness.
//	A-G  The remembered translation is spent by the reader's own choice and by
//	     its own return, and by nothing else (D13). A translation the reader
//	     chose that takes the screen spends it, or the next save writes the
//	     remembered one over their choice; one a link brought leaves it.
//	A-H  An upgrade the refresh owes is on its way: a fetch in flight or a
//	     retry armed. Owed and not on its way is a previous edition nothing
//	     will ever replace while the app runs (D17), however long the reader
//	     is online.
func checkArrivalInvariants(prev, now arrivalFacts, _ arrivalEvent) []string {
	var bad []string
	if now.parked && now.failureTold && !now.loading {
		bad = append(bad, "A-A: the arrival was reported failed and its park is still waiting for a load that will never come")
	}
	// A translation the reader chose is not an arrival. If the location moved
	// when it landed, something else moved them — except a link to that same
	// translation, still live, parked behind the reader's own load: its landing
	// is the arrival's too, and opening the passage keeps the link's promise.
	consumedLivePark := prev.parked && prev.parkedFor == now.current && !prev.failureTold
	if now.landed == "reader" && now.loc != prev.loc && !consumedLivePark {
		bad = append(bad, "A-B: choosing a translation moved the reader to a passage they did not ask for")
	}
	if now.landed == "upgrade" && (now.loc != prev.loc || now.current != prev.current || now.preferred != prev.preferred) {
		bad = append(bad, "A-B: an edition upgrade moved the reader, changed the translation on screen, or spent what is remembered")
	}
	if now.arrivalOwed && !now.loading {
		bad = append(bad, "A-C: an arrival is owed and nothing in flight can honour it")
	}
	if now.edition == "previous" && !now.stale {
		bad = append(bad, "A-D: a previous edition is on screen and nothing records it")
	}
	if now.edition == "current" && now.stale {
		bad = append(bad, "A-D: the current edition is on screen and the app still calls it stale")
	}
	if now.edition == "previous" {
		v, _ := versionByID(now.current)
		if v.Name == "" || !strings.Contains(now.notice, v.Name) || !strings.Contains(now.notice, "previous edition") {
			bad = append(bad, "A-E: a previous edition is on screen and the picker does not say so")
		}
	}
	if now.landed == "reader" && now.preferred != "" && now.preferred != now.current {
		bad = append(bad, "A-G: the reader's own choice took the screen and did not spend the translation "+
			"the picker says is remembered, so the next save writes that over their choice")
	}
	if now.landed == "arrival" && prev.preferred != "" && now.preferred != prev.preferred && now.current != prev.preferred {
		bad = append(bad, "A-G: a link spent the reader's remembered translation")
	}
	if now.owed && !now.onItsWay {
		bad = append(bad, "A-H: an upgrade is owed and nothing is fetching it or scheduled to")
	}
	return bad
}

// arrivalLivenessHorizon is how many steps a journey must have left after a
// previous edition reaches the screen before the walk judges whether the
// current one can follow it. Two is the shortest way back any repair could
// take: the reader leaves the translation and returns, or a fetch starts and
// lands.
const arrivalLivenessHorizon = 2

// arrivalEnd is where one journey ended: its route, and the facts after its
// last step. Every prefix of a journey is itself a journey, so the ends of a
// walk are every state it reached, each with the route that reached it.
type arrivalEnd struct {
	path  []arrivalEvent
	facts arrivalFacts
}

// checkArrivalLiveness is A-F over one world's walk: for every state that has
// a previous edition on screen and the horizon still ahead of it, some
// journey through that state must later have the same translation's current
// edition on screen. It returns the states for which none does, marked stuck.
func checkArrivalLiveness(ends map[string]arrivalEnd, depth int) []arrivalEnd {
	var stuck []arrivalEnd
	for route, end := range ends {
		f := end.facts
		if f.edition != "previous" || len(end.path) > depth-arrivalLivenessHorizon {
			continue
		}
		live := false
		for other, later := range ends {
			if strings.HasPrefix(other, route+" -> ") &&
				later.facts.current == f.current && later.facts.edition == "current" {
				live = true
				break
			}
		}
		if !live {
			f.stuck = true
			stuck = append(stuck, arrivalEnd{path: end.path, facts: f})
		}
	}
	return stuck
}

// --- the trajectory walk ------------------------------------------------------

const arrivalDepth = 5

// arrivalRemembered is the translation the journeys that start remembering
// one remember. The restore records only a licensed translation it could not
// open (D9), so it is the licensed one.
const arrivalRemembered = "nkjv"

func TestArrivalJourneysKeepTheirPromise(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	// The disk an arrival lands on. The default translation and the other one
	// have their current edition cached, as the fetch that landed each left
	// it; the link's translation has, in turn, its current edition, only the
	// edition before it, and nothing. Built here, so every machine walks the
	// same worlds; the journeys used to read whatever the machine running them
	// had downloaded, and CI, with nothing, walked another.
	w := newArrivalWorld(t, arDiskCurrent)
	if _, ok := versionByID(arrivalRemembered); !ok {
		t.Fatalf("control: %s is not registered, so no journey can remember it", arrivalRemembered)
	}

	shortest := map[string]int{}
	route := map[string]string{}
	seen := map[string]bool{}
	journeys, steps, stuck := 0, 0, 0
	began := time.Now()

	report := func(world string, path []arrivalEvent, prev, now arrivalFacts, e arrivalEvent, violations []string) {
		for _, bad := range violations {
			explained := false
			for _, d := range knownArrivalIncoherent {
				if d.covers(bad, prev, now, e) {
					seen[d.name] = true
					explained = true
					break
				}
			}
			if explained {
				continue
			}
			// Keyed on the VIOLATION and the world, not the path: hundreds of
			// journeys reach the same broken promise by dozens of routes, and
			// a report that lists them all buries the handful of distinct
			// facts. The SHORTEST route is kept, because that is the one to
			// read.
			key := fmt.Sprintf("%s: %s: %s", world, e, bad)
			if old, had := shortest[key]; !had || len(path) < old {
				shortest[key] = len(path)
				route[key] = pathString(path)
			}
		}
	}

	events := []arrivalEvent{arLinkNamesOther, arFetchFails, arFetchFailsPreviousServes, arFetchLands, arReaderPicksIt, arReaderPicksOther, arReaderStartsOther, arReaderStartsIt, arUpdateLands}

	// Six worlds: each disk, starting with nothing remembered and starting
	// with the reader's licensed translation remembered.
	for _, remembered := range []string{"", arrivalRemembered} {
		for _, disk := range []arrivalDisk{arDiskCurrent, arDiskPrevious, arDiskNone} {
			w.disk, w.preferred = disk, remembered
			world := disk.String()
			if remembered != "" {
				world += "+remembers-" + remembered
			}
			worldBegan, worldJourneys, worldSteps := time.Now(), journeys, steps
			ends := map[string]arrivalEnd{}
			// What each world must show it walked, or it is not the world it
			// says it is: the controls after the walk read these.
			sawPrevious, sawTold, sawCurrentLink, sawParkBehindReader := false, false, false, false
			sawKept, sawSpent := false, false
			sawUpgrade, sawReaderPrevious := false, false

			var walk func(path []arrivalEvent, depth int)
			walk = func(path []arrivalEvent, depth int) {
				if depth == 0 {
					return
				}
				for _, ev := range events {
					next := append(append([]arrivalEvent(nil), path...), ev)

					// A FRESH state per branch, on a fresh copy of the world's
					// disk. AppState carries an atomic, so it can never be
					// copied to fork a journey — every branch is replayed from
					// the beginning instead. Every prefix of this journey was
					// walked, and checked, as a journey of its own, so only
					// its last step is new.
					w.reset(t)
					st, told := freshArrivalState(w.preferred)
					prev := arrivalFactsOf(w, st, false, told, "")
					var now arrivalFacts
					did := false
					for i, e := range next {
						var landed string
						if did, landed = e.apply(t, w, st, &told); !did {
							break
						}
						owed := st.pendingLink != nil && !told
						now = arrivalFactsOf(w, st, owed, told, landed)
						if i < len(next)-1 {
							prev = now
						}
					}
					if !did {
						continue
					}
					journeys++
					steps += len(next)
					report(world, next, prev, now, ev, checkArrivalInvariants(prev, now, ev))
					sawPrevious = sawPrevious || now.edition == "previous"
					sawTold = sawTold || now.failureTold
					sawCurrentLink = sawCurrentLink || (now.current == arrivalLinkVersion && now.edition == "current")
					sawParkBehindReader = sawParkBehindReader || (now.parked && w.inflightReader)
					sawKept = sawKept || (now.landed == "arrival" && now.preferred != "")
					sawSpent = sawSpent || (now.landed == "reader" && prev.preferred != "" && now.preferred == "")
					sawUpgrade = sawUpgrade || now.landed == "upgrade"
					sawReaderPrevious = sawReaderPrevious || (now.landed == "reader" && now.edition == "previous")
					ends[pathString(next)] = arrivalEnd{path: next, facts: now}
					walk(next, depth-1)
				}
			}
			walk(nil, arrivalDepth)

			for _, s := range checkArrivalLiveness(ends, arrivalDepth) {
				stuck++
				last := s.path[len(s.path)-1]
				report(world, s.path, s.facts, s.facts, last,
					[]string{"A-F: a previous edition is on screen and nothing the reader does next, with the network up, brings the current one"})
			}

			switch disk {
			case arDiskCurrent:
				if sawPrevious || sawTold || sawUpgrade {
					t.Errorf("control: %s put a previous edition on screen, failed a load or landed an upgrade, and with the current edition on disk none can happen", world)
				}
			case arDiskPrevious:
				if !sawPrevious || sawTold {
					t.Errorf("control: %s never served the previous edition, or failed a load it could not fail, so the arm it exists for was not walked", world)
				}
				// The two things D17's fix is for: the upgrade landing, and a
				// previous edition served to a load the reader started.
				if !sawUpgrade || !sawReaderPrevious {
					t.Errorf("control: %s never landed an upgrade (%v) or never served a previous edition to the reader's own load (%v), so the refresh was not put to either", world, sawUpgrade, sawReaderPrevious)
				}
			case arDiskNone:
				if sawPrevious || !sawTold || sawUpgrade {
					t.Errorf("control: %s served a previous edition it does not have, landed an upgrade nothing owed, or never failed a load", world)
				}
			}
			if !sawCurrentLink {
				t.Errorf("control: %s never put %s's current edition on screen, so nothing could count as a way back from a previous one", world, arrivalLinkVersion)
			}
			if !sawParkBehindReader {
				t.Errorf("control: %s never parked a link behind a load of the reader's own, so no arrival met one", world)
			}
			if remembered != "" && (!sawKept || !sawSpent) {
				t.Errorf("control: %s never kept the remembered translation through a link's landing (%v) or never spent it on a choice of the reader's (%v), so A-G was not put to either half", world, sawKept, sawSpent)
			}
			t.Logf("%s: %d journeys / %d steps in %v", world, journeys-worldJourneys, steps-worldSteps, time.Since(worldBegan).Round(time.Millisecond))
		}
	}

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
	t.Logf("%d journeys / %d steps walked over six worlds in %v; stuck on a previous edition: %d; %d pinned incoherent states reached",
		journeys, steps, time.Since(began).Round(time.Millisecond), stuck, len(seen))
}

func pathString(p []arrivalEvent) string {
	out := make([]string, len(p))
	for i, e := range p {
		out[i] = e.String()
	}
	return strings.Join(out, " -> ")
}

// freshArrivalState is the reader on the default translation's current
// edition, remembering preferred ("" for nothing), with nothing told.
func freshArrivalState(preferred string) (*AppState, bool) {
	base := stampedBible("current")
	return &AppState{
		Bible:            base,
		CurrentVersion:   defaultVersionID,
		currentMode:      modeReal,
		loadedVersions:   map[string]*BibleData{defaultVersionID: base},
		loadPhase:        loadReady,
		CurrentBook:      "Genesis",
		CurrentChapter:   1,
		preferredVersion: preferred,
	}, false
}

// TestTheArrivalInvariantsCanActuallyFail is the control for the walk above,
// for the reason TestTheSelectionInvariantsCanActuallyFail is one: a walk that
// reports nothing is evidence only if its checks can complain. Each shape here
// is one an invariant exists to catch, handed to it directly.
func TestTheArrivalInvariantsCanActuallyFail(t *testing.T) {
	link, _ := versionByID(arrivalLinkVersion)
	remembered, _ := versionByID(arrivalRemembered)
	said := link.Name + " is showing a previous edition until the update can be downloaded."
	substituted := remembered.Name + " could not be opened this time — " + link.Name +
		" is shown instead. Your choice is remembered and tried again each time the app starts."
	// A live link to the translation the reader's own load brought, parked
	// behind that load; and the same park after its failure was told.
	parkedHere := arrivalFacts{loc: "Genesis|1", parked: true, parkedFor: link.ID, loading: true}
	parkedHereTold := parkedHere
	parkedHereTold.failureTold = true
	for _, tc := range []struct {
		want      string
		prev, now arrivalFacts
	}{
		{"A-B", arrivalFacts{loc: "Genesis|1"}, arrivalFacts{loc: "John|1", current: arrivalOtherVersion, edition: "current", landed: "reader"}},
		// The exemption's loud twin: the park that moves the reader was
		// already told as failed, so it is not a live link keeping its promise.
		{"A-B", parkedHereTold, arrivalFacts{loc: "John|1", current: link.ID, edition: "current", landed: "reader"}},
		// An upgrade that moves the reader, and one that spends what is
		// remembered.
		{"A-B", arrivalFacts{loc: "Genesis|1", current: link.ID}, arrivalFacts{loc: "John|1", current: link.ID, edition: "current", landed: "upgrade"}},
		{"A-B", arrivalFacts{loc: "Genesis|1", current: link.ID, preferred: remembered.ID}, arrivalFacts{loc: "Genesis|1", current: link.ID, edition: "current", landed: "upgrade"}},
		{"A-C", arrivalFacts{}, arrivalFacts{current: link.ID, edition: "current", parked: true, arrivalOwed: true}},
		{"A-D", arrivalFacts{}, arrivalFacts{current: link.ID, edition: "previous", notice: said}},
		{"A-D", arrivalFacts{}, arrivalFacts{current: link.ID, edition: "current", stale: true, notice: said}},
		{"A-E", arrivalFacts{}, arrivalFacts{current: link.ID, edition: "previous", stale: true}},
		// D10's sentence names the translation on screen and says nothing of
		// its edition.
		{"A-E", arrivalFacts{}, arrivalFacts{current: link.ID, edition: "previous", stale: true, preferred: remembered.ID, notice: substituted}},
		{"A-G", arrivalFacts{preferred: remembered.ID}, arrivalFacts{current: arrivalOtherVersion, edition: "current", landed: "reader", preferred: remembered.ID}},
		{"A-G", arrivalFacts{preferred: remembered.ID}, arrivalFacts{current: link.ID, edition: "current", landed: "arrival"}},
		// Owed, and nothing fetching it or armed to.
		{"A-H", arrivalFacts{}, arrivalFacts{current: link.ID, edition: "previous", stale: true, notice: said, owed: true}},
	} {
		got := checkArrivalInvariants(tc.prev, tc.now, arLinkNamesOther)
		fired := false
		for _, bad := range got {
			fired = fired || strings.HasPrefix(bad, tc.want+":")
		}
		if !fired {
			t.Errorf("%s does not fire on %s after %s; got %q", tc.want, tc.now, tc.prev, got)
		}
	}
	// And the states they must leave alone, or the walk would be all noise:
	// a park with its load still running, a previous edition that is recorded,
	// said, owed and on its way, a link that lands and keeps the remembered
	// translation, a choice of the reader's that spends it, a reader's landing
	// that opens the passage of a live link parked behind it, and an upgrade
	// that swaps the edition and nothing else.
	for _, quiet := range []struct{ prev, now arrivalFacts }{
		{arrivalFacts{}, arrivalFacts{current: link.ID, edition: "current", parked: true, arrivalOwed: true, loading: true}},
		{arrivalFacts{}, arrivalFacts{current: link.ID, edition: "previous", stale: true, notice: said, owed: true, onItsWay: true}},
		{arrivalFacts{preferred: remembered.ID}, arrivalFacts{current: link.ID, edition: "current", landed: "arrival", preferred: remembered.ID, notice: substituted}},
		{arrivalFacts{preferred: remembered.ID}, arrivalFacts{current: arrivalOtherVersion, edition: "current", landed: "reader"}},
		{parkedHere, arrivalFacts{loc: "John|1", current: link.ID, edition: "current", landed: "reader"}},
		{arrivalFacts{loc: "John|1", current: link.ID, preferred: remembered.ID}, arrivalFacts{loc: "John|1", current: link.ID, edition: "current", landed: "upgrade", preferred: remembered.ID, notice: substituted}},
	} {
		if got := checkArrivalInvariants(quiet.prev, quiet.now, arLinkNamesOther); len(got) > 0 {
			t.Errorf("a coherent state was reported: %s after %s: %q", quiet.now, quiet.prev, got)
		}
	}

	// A-F: a previous edition from which some journey reaches the current one
	// is live; the same state with no such journey is stuck.
	here := []arrivalEvent{arLinkNamesOther, arFetchFailsPreviousServes}
	there := append(append([]arrivalEvent(nil), here...), arReaderPicksOther, arReaderPicksIt)
	onPrevious := arrivalFacts{current: link.ID, edition: "previous", stale: true, notice: said}
	onCurrent := arrivalFacts{current: link.ID, edition: "current"}
	ends := map[string]arrivalEnd{pathString(here): {here, onPrevious}}
	if stuck := checkArrivalLiveness(ends, arrivalDepth); len(stuck) != 1 || !stuck[0].facts.stuck {
		t.Errorf("A-F does not fire on a previous edition nothing leads on from: %v", stuck)
	}
	ends[pathString(there)] = arrivalEnd{there, onCurrent}
	if stuck := checkArrivalLiveness(ends, arrivalDepth); len(stuck) != 0 {
		t.Errorf("A-F fires on a previous edition the reader can leave: %v", stuck)
	}
}

// TestAPreviousEditionIsUpdatedWhileTheAppRuns is D17's guard. A translation
// served from its previous edition after a failed fetch is RECORDED as stale
// (D3), and the record is the refresh's work list: the upgrade is owed at
// once, waits out the first backoff step, and lands in place, on screen or
// not, with nothing else about the reader changed. Every route by which a
// previous edition reaches the screen is here — a tapped link, the reader's
// own choice made offline, and the launch — and so is what the reader does
// while it is owed.
//
// Every trigger is the app's own. The retry the backoff arms is the callback
// it handed its timer, fired; opening the picker and the launch call the real
// triggerFullDownload; and what each fetches is what the real trigger chose,
// held at its door and landed through the tail it built. A refresh that went
// back to fetching the default alone, or a timer that fired and did nothing,
// is a previous edition nothing replaces, and fails here.
//
// It used to reproduce D17, when it asserted the opposite: the picker and a
// link handed the reader the same previous edition for as long as the app
// ran, with the network up, and nothing fetched it.
func TestAPreviousEditionIsUpdatedWhileTheAppRuns(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	link, _ := versionByID(arrivalLinkVersion)
	other, _ := versionByID(arrivalOtherVersion)

	// servedPrevious walks route on the disk that holds only the link's
	// translation's previous edition, and requires what the fix promises
	// there: that edition on screen, recorded and said, owed first, and a
	// retry armed through the timer's door rather than a fetch at once.
	servedPrevious := func(t *testing.T, route ...arrivalEvent) (*arrivalWorld, *AppState) {
		t.Helper()
		w := newArrivalWorld(t, arDiskPrevious)
		st, told := freshArrivalState("")
		for _, e := range route {
			if did, _ := e.apply(t, w, st, &told); !did {
				t.Fatalf("control: %s did nothing, so the route %s is not walked", e, pathString(route))
			}
		}
		if st.CurrentVersion != link.ID || bibleStamp(st.Bible) != "previous" || !st.staleVersions[link.ID] {
			t.Fatalf("control: %s must end on the previous edition, recorded as stale; on %s with %q", pathString(route), st.CurrentVersion, bibleStamp(st.Bible))
		}
		if n := fullPendingNotice(st); !strings.Contains(n, link.Name+" is showing a previous edition until the update can be downloaded") {
			t.Fatalf("control: the picker must be promising the update; got %q", n)
		}
		if owed := owedUpgrades(st); len(owed) == 0 || owed[0].ID != link.ID {
			t.Fatalf("%s: a previous edition is on screen and the refresh does not owe it first: %v", pathString(route), owed)
		}
		if w.armed == nil || st.fullRetryDelay <= 0 || st.fullDownloading || w.upgrade != nil {
			t.Fatalf("%s: the previous edition is served and no retry is armed, or it was fetched at once (armed %v, delay %v, fetching %v)",
				pathString(route), w.armed != nil, st.fullRetryDelay, st.fullDownloading)
		}
		return w, st
	}
	// upgrade fires the retry the backoff armed, with the network up, through
	// the walk's own event, and requires what it fetched to land and change
	// nothing else.
	upgrade := func(t *testing.T, w *arrivalWorld, st *AppState) {
		t.Helper()
		loc := fmt.Sprintf("%s|%d", st.CurrentBook, st.CurrentChapter)
		current, preferred := st.CurrentVersion, st.preferredVersion
		told := false
		if did, landed := arUpdateLands.apply(t, w, st, &told); !did || landed != "upgrade" {
			t.Fatalf("the owed upgrade did not land: did %v, landed %q", did, landed)
		}
		if st.staleVersions[link.ID] || bibleStamp(st.loadedVersions[link.ID]) != "current" || !versionCacheIsCurrent(link) {
			t.Fatalf("the upgrade landed and %s is not current: still marked %v, in memory %q, on disk %v",
				link.ID, st.staleVersions[link.ID], bibleStamp(st.loadedVersions[link.ID]), versionCacheIsCurrent(link))
		}
		if got := fmt.Sprintf("%s|%d", st.CurrentBook, st.CurrentChapter); got != loc || st.CurrentVersion != current || st.preferredVersion != preferred {
			t.Fatalf("the upgrade moved the reader to %s on %s remembering %q, from %s on %s remembering %q",
				got, st.CurrentVersion, st.preferredVersion, loc, current, preferred)
		}
		if owed := owedUpgrades(st); len(owed) != 0 || st.fullRetryDelay != 0 {
			t.Fatalf("nothing is owed any more, and the refresh still says otherwise: owed %v, delay %v", owed, st.fullRetryDelay)
		}
	}

	for _, tc := range []struct {
		name  string
		route []arrivalEvent
	}{
		{"link", []arrivalEvent{arLinkNamesOther, arFetchFailsPreviousServes}},
		{"reader's offline choice", []arrivalEvent{arReaderStartsIt, arFetchFailsPreviousServes}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, st := servedPrevious(t, tc.route...)
			upgrade(t, w, st)
			if bibleStamp(st.Bible) != "current" {
				t.Fatalf("the upgrade landed and the screen still shows the %q edition", bibleStamp(st.Bible))
			}
			if n := fullPendingNotice(st); n != "" {
				t.Fatalf("the notice outlived the edition it describes: %q", n)
			}
		})
	}

	t.Run("away", func(t *testing.T) {
		w, st := servedPrevious(t, arLinkNamesOther, arFetchFailsPreviousServes)
		switchVersion(st, other.ID, byReader)
		if st.CurrentVersion != other.ID {
			t.Fatalf("control: the reader must be on %s; on %s", other.ID, st.CurrentVersion)
		}
		upgrade(t, w, st)
		// Back to it through the picker: memory now holds the current edition.
		before := w.src.fetches
		switchVersionInteractive(st, link.ID, byReader)
		if st.CurrentVersion != link.ID || bibleStamp(st.Bible) != "current" || w.src.fetches != before {
			t.Fatalf("back through the picker: on %s with %q after %d fetch(es)", st.CurrentVersion, bibleStamp(st.Bible), w.src.fetches-before)
		}
		if n := fullPendingNotice(st); n != "" {
			t.Fatalf("the notice outlived the edition it describes: %q", n)
		}
	})

	// While it is owed, the switch itself never fetches: the picker and a
	// link treat a copy in memory as loaded. The previous edition comes back
	// at once, still recorded and said, and the upgrade is still on its way.
	t.Run("picker, while stale", func(t *testing.T) {
		w, st := servedPrevious(t, arLinkNamesOther, arFetchFailsPreviousServes)
		switchVersion(st, other.ID, byReader)
		before := w.src.fetches
		switchVersionInteractive(st, link.ID, byReader)
		if st.CurrentVersion != link.ID || bibleStamp(st.Bible) != "previous" || w.src.fetches != before || st.versionLoading {
			t.Fatalf("back through the picker: on %s with %q after %d fetch(es), loading %v", st.CurrentVersion, bibleStamp(st.Bible), w.src.fetches-before, st.versionLoading)
		}
		if !st.staleVersions[link.ID] || !strings.Contains(fullPendingNotice(st), "previous edition") {
			t.Fatalf("the previous edition is back on screen unrecorded or unsaid: marked %v, notice %q", st.staleVersions[link.ID], fullPendingNotice(st))
		}
		if len(owedUpgrades(st)) == 0 || (st.fullRetryDelay <= 0 && !st.fullDownloading) {
			t.Fatalf("the upgrade is no longer on its way: owed %v, delay %v", owedUpgrades(st), st.fullRetryDelay)
		}
	})

	// Opening the picker retries what is owed at once, after reading the
	// notice (D5): the real triggerFullDownload, whose fetch is held at its
	// door and landed.
	t.Run("picker opened", func(t *testing.T) {
		w, st := servedPrevious(t, arLinkNamesOther, arFetchFailsPreviousServes)
		if n := noticeOnPickerOpen(st); !strings.Contains(n, link.Name+" is showing a previous edition") {
			t.Fatalf("the picker's footer does not say the previous edition it opened on: %q", n)
		}
		if !st.fullDownloading || w.upgrade == nil || w.upgrade.v.ID != link.ID {
			t.Fatalf("opening the picker did not fetch the owed upgrade of %s: fetching %v, fetch %v", link.ID, st.fullDownloading, w.upgrade)
		}
		w.landUpgrade(t, st)
		if bibleStamp(st.Bible) != "current" || st.staleVersions[link.ID] || fullPendingNotice(st) != "" || st.fullDownloading || st.fullRetryDelay != 0 {
			t.Fatalf("the picker's retry landed and the screen shows %q, marked %v, notice %q, fetching %v, delay %v",
				bibleStamp(st.Bible), st.staleVersions[link.ID], fullPendingNotice(st), st.fullDownloading, st.fullRetryDelay)
		}
	})

	// The launch restores the translation from its previous edition when it
	// cannot fetch it, and the live state the reader uses owes it the upgrade:
	// the launch's own trigger fetches it, and it lands.
	t.Run("launch", func(t *testing.T) {
		w := newArrivalWorld(t, arDiskPrevious)
		// The default's current edition is on disk; its source is offline all
		// the same, so nothing this launch does can reach the network.
		withVersionSource(t, defaultVersionID, &arrivalSource{offline: true})
		writeReadingState(appPrefs(), readingState{Version: link.ID, Book: "John", Chapter: 1})
		w.src.offline = true
		loaded, err := loadStateData()
		w.src.offline = false
		if err != nil {
			t.Fatalf("control: a launch with %s saved must open on its previous edition: %v", link.ID, err)
		}
		live := NewLoadingState()
		adoptLaunch(live, loaded)
		if live.CurrentVersion != link.ID || bibleStamp(live.Bible) != "previous" || !live.staleVersions[link.ID] {
			t.Fatalf("control: the launch must show %s's previous edition, recorded; on %s with %q", link.ID, live.CurrentVersion, bibleStamp(live.Bible))
		}
		// What StartBackgroundLoad's tail runs last.
		triggerFullDownload(live)
		if !live.fullDownloading || w.upgrade == nil || w.upgrade.v.ID != link.ID {
			t.Fatalf("the launch shows a previous edition and its trigger does not fetch it: fetching %v, fetch %v, owed %v", live.fullDownloading, w.upgrade, owedUpgrades(live))
		}
		w.landUpgrade(t, live)
		if bibleStamp(live.Bible) != "current" || live.staleVersions[link.ID] || fullPendingNotice(live) != "" {
			t.Fatalf("the upgrade landed and the launch's screen shows %q, marked %v, notice %q",
				bibleStamp(live.Bible), live.staleVersions[link.ID], fullPendingNotice(live))
		}
		if live.fullDownloading || live.fullRetryDelay != 0 {
			t.Fatalf("nothing is owed and the refresh is not settled: fetching %v, delay %v", live.fullDownloading, live.fullRetryDelay)
		}
		// Where the trigger is called runs in StartBackgroundLoad's fyne.Do
		// tail, which a test cannot await, so that is read from the source:
		// the launch calls triggerFullDownload under no condition of its own,
		// because owedUpgrades decides. A launch that asked fullPending first
		// would leave this translation unfetched until the reader left the app
		// or opened the picker.
		launch := parsePackageSource(t).funcs["StartBackgroundLoad"]
		if len(launch) != 1 {
			t.Fatalf("control: StartBackgroundLoad is declared %d times, so this reading of the source is wrong", len(launch))
		}
		calls, guarded := 0, 0
		var stack []ast.Node
		ast.Inspect(launch[0].Body, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			stack = append(stack, n)
			if call, ok := n.(*ast.CallExpr); ok {
				if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "triggerFullDownload" {
					calls++
					for _, outer := range stack {
						if _, ok := outer.(*ast.IfStmt); ok {
							guarded++
							break
						}
					}
				}
			}
			return true
		})
		if calls != 1 || guarded != 0 {
			t.Fatalf("the launch calls triggerFullDownload %d time(s), %d under a condition; want once, unconditionally", calls, guarded)
		}
	})

	// Two owed at once, both public-domain translations on their previous
	// editions, and the retry that fires after two failed attempts. The one on
	// screen is fetched first, and its landing starts the next, so the second
	// is on its way the moment the first lands rather than at the next
	// foreground or picker opening (A-H). The landing also restarts the
	// backoff: a failure after a success waits the first step, not the next
	// one of a streak that has ended.
	t.Run("two owed", func(t *testing.T) {
		w := newArrivalWorld(t, arDiskPrevious)
		st, _ := freshArrivalState("")
		st.Bible = stampedBible("previous")
		st.CurrentVersion = link.ID
		st.loadedVersions[link.ID] = st.Bible
		st.loadedVersions[other.ID] = stampedBible("previous")
		st.staleVersions = map[string]bool{link.ID: true, other.ID: true}
		st.fullRetryDelay = 40 * time.Second
		triggerFullDownload(st)
		if w.upgrade == nil || w.upgrade.v.ID != link.ID {
			t.Fatalf("control: the refresh must fetch %s, on screen, first; fetch %v", link.ID, w.upgrade)
		}
		w.landUpgrade(t, st)
		if bibleStamp(st.Bible) != "current" || st.staleVersions[link.ID] {
			t.Fatalf("%s's upgrade did not land: on %q, marked %v", link.ID, bibleStamp(st.Bible), st.staleVersions[link.ID])
		}
		if !st.fullDownloading || w.upgrade == nil || w.upgrade.v.ID != other.ID {
			t.Fatalf("A-H after a landing: %s is still owed and nothing is fetching it (fetching %v, fetch %v, delay %v)",
				other.ID, st.fullDownloading, w.upgrade, st.fullRetryDelay)
		}
		failed := w.upgrade
		w.upgrade = nil
		failed.land(nil, modeReal, errors.New("offline"))
		if st.fullDownloading || w.armed == nil || st.fullRetryDelay != 20*time.Second {
			t.Fatalf("the fetch after a landing failed and its retry is not the backoff's first step: fetching %v, armed %v, delay %v",
				st.fullDownloading, w.armed != nil, st.fullRetryDelay)
		}
		fire := w.armed
		w.armed = nil
		fire()
		if w.upgrade == nil || w.upgrade.v.ID != other.ID {
			t.Fatalf("the retry fired and did not fetch %s: fetch %v", other.ID, w.upgrade)
		}
		w.landUpgrade(t, st)
		if st.staleVersions[other.ID] || bibleStamp(st.loadedVersions[other.ID]) != "current" || st.CurrentVersion != link.ID || bibleStamp(st.Bible) != "current" {
			t.Fatalf("%s's upgrade did not land in memory, or moved the screen: marked %v, in memory %q, on %s with %q",
				other.ID, st.staleVersions[other.ID], bibleStamp(st.loadedVersions[other.ID]), st.CurrentVersion, bibleStamp(st.Bible))
		}
		if len(owedUpgrades(st)) != 0 || st.fullDownloading || st.fullRetryDelay != 0 || w.upgrade != nil {
			t.Fatalf("nothing is owed and the refresh is not settled: owed %v, fetching %v, delay %v", owedUpgrades(st), st.fullDownloading, st.fullRetryDelay)
		}
	})

	// With nothing owed the refresh settles its backoff at zero. A-H reads a
	// standing delay as a retry on its way, and ensureUpgradeScheduled arms
	// none while one stands, so a delay left over by a timer that fired with
	// nothing owed would claim a retry nobody armed, and the next previous
	// edition served would wait for the reader to leave the app.
	t.Run("settled", func(t *testing.T) {
		st := &AppState{CurrentVersion: defaultVersionID, fullRetryDelay: 40 * time.Second}
		fetches := upgradeFetchesStarted.Load()
		triggerFullDownload(st)
		if st.fullRetryDelay != 0 || st.fullDownloading || upgradeFetchesStarted.Load() != fetches {
			t.Fatalf("with nothing owed the refresh did not settle: delay %v, fetching %v, fetches %d",
				st.fullRetryDelay, st.fullDownloading, upgradeFetchesStarted.Load()-fetches)
		}
		armed := upgradeRetriesArmed.Load()
		markVersionStale(st, other.ID)
		ensureUpgradeScheduled(st)
		if upgradeRetriesArmed.Load() == armed || st.fullRetryDelay <= 0 {
			t.Fatalf("a previous edition served after the refresh settled armed no retry (delay %v)", st.fullRetryDelay)
		}
	})

	// Tearing down, a landing changes nothing and arms nothing. On the
	// desktop, glfw runs fyne.Do inline after the main loop drains, so a
	// fetch that ends during teardown would otherwise write the state and
	// start a timer in a closing app.
	t.Run("teardown", func(t *testing.T) {
		previous := stampedBible("previous")
		st := &AppState{
			Bible:           previous,
			CurrentVersion:  link.ID,
			loadedVersions:  map[string]*BibleData{link.ID: previous},
			staleVersions:   map[string]bool{link.ID: true},
			fullDownloading: true,
			fullRetryDelay:  20 * time.Second,
		}
		st.stopping.Store(true)
		armed := upgradeRetriesArmed.Load()
		upgradeLanded(st, link, nil, modeReal, errors.New("offline"))
		if upgradeRetriesArmed.Load() != armed || st.fullRetryDelay != 20*time.Second {
			t.Fatalf("a failed fetch ending in teardown armed a retry (%d) or moved the backoff to %v",
				upgradeRetriesArmed.Load()-armed, st.fullRetryDelay)
		}
		upgradeLanded(st, link, stampedBible("current"), modeReal, nil)
		if st.Bible != previous || !st.staleVersions[link.ID] || st.fullRetryDelay != 20*time.Second {
			t.Fatalf("a fetch landing in teardown changed the state: on %q, marked %v, delay %v",
				bibleStamp(st.Bible), st.staleVersions[link.ID], st.fullRetryDelay)
		}
	})

	// Two upgrades owed at once: the launch restored this translation on its
	// previous edition while the default's own cache was a previous edition
	// too. This one lands first, being on screen, and nothing about the
	// default's refresh is its to clear, or the default is never fetched. And
	// a landing for a translation repaired meanwhile, by its own load or D11's
	// re-read, has nothing to swap.
	t.Run("the default's refresh is its own", func(t *testing.T) {
		def, _ := versionByID(defaultVersionID)
		previous := stampedBible("previous")
		st := &AppState{
			Bible:          previous,
			CurrentVersion: link.ID,
			loadedVersions: map[string]*BibleData{def.ID: stampedBible("previous"), link.ID: previous},
			staleVersions:  map[string]bool{link.ID: true},
			fullPending:    true,
		}
		if owed := owedUpgrades(st); len(owed) != 2 || owed[0].ID != link.ID || owed[1].ID != def.ID {
			t.Fatalf("control: both must be owed, the one on screen first; owed %v", owed)
		}
		applyFullDownload(st, link, stampedBible("current"), modeReal)
		if bibleStamp(st.Bible) != "current" || st.staleVersions[link.ID] {
			t.Fatalf("%s's upgrade did not land: on %q, marked %v", link.ID, bibleStamp(st.Bible), st.staleVersions[link.ID])
		}
		if owed := owedUpgrades(st); !st.fullPending || len(owed) != 1 || owed[0].ID != def.ID {
			t.Fatalf("%s's landing cleared the default's refresh: pending %v, owed %v", link.ID, st.fullPending, owed)
		}
		repaired := st.loadedVersions[link.ID]
		applyFullDownload(st, link, stampedBible("current"), modeReal)
		if st.loadedVersions[link.ID] != repaired || st.Bible != repaired {
			t.Fatal("a landing for a translation no longer marked replaced what is in memory and on screen")
		}
	})

	// The default can carry the mark too, should a load of it ever serve its
	// previous edition. Its own landing clears it, or the refresh, which owes
	// a marked translation, would fetch it again at once and for ever.
	t.Run("default marked", func(t *testing.T) {
		def, _ := versionByID(defaultVersionID)
		st := &AppState{
			CurrentVersion: def.ID,
			loadedVersions: map[string]*BibleData{def.ID: stampedBible("previous")},
			staleVersions:  map[string]bool{def.ID: true},
		}
		if owed := owedUpgrades(st); len(owed) != 1 || owed[0].ID != def.ID {
			t.Fatalf("control: the marked default must be owed; owed %v", owed)
		}
		applyFullDownload(st, def, stampedBible("current"), modeReal)
		if st.staleVersions[def.ID] || len(owedUpgrades(st)) != 0 {
			t.Fatalf("the default's upgrade landed and it is still owed: marked %v, owed %v", st.staleVersions[def.ID], owedUpgrades(st))
		}
	})

	// Never licensed, never a placeholder: the app does not spend the
	// API.Bible quota on its own initiative, and a placeholder has nothing to
	// fetch. The licensed translation here is licensed and available, so the
	// licence is what excludes it and not its availability.
	t.Run("licensed", func(t *testing.T) {
		t.Setenv("BIBLE_API_KEY", "owed-upgrades-key")
		t.Setenv("BIBLETEXT_LICENSE_NKJV", "1")
		t.Setenv("BIBLETEXT_PROVIDER_ID_NKJV", "test-provider-id")
		prevKeys := sharedKeys
		ks := &keyStore{prefs: newFakePrefs(), secrets: emptySecretStore{}}
		sharedKeys = func() *keyStore { return ks }
		t.Cleanup(func() { sharedKeys = prevKeys })
		nk, _ := versionByID("nkjv")
		if !isLicensedSource(nk) || nk.isTesting() {
			t.Fatalf("control: nkjv must be licensed and available, or its exclusion proves nothing (licensed %v, placeholder %v)", isLicensedSource(nk), nk.isTesting())
		}
		ids := func(st *AppState) string {
			var out []string
			for _, v := range owedUpgrades(st) {
				out = append(out, v.ID)
			}
			return strings.Join(out, " ")
		}
		if got := ids(&AppState{CurrentVersion: nk.ID, staleVersions: map[string]bool{nk.ID: true, other.ID: true}}); got != other.ID {
			t.Errorf("owed %q, want only the public-domain %s", got, other.ID)
		}
		// The order the refresh fetches in: on screen, then the default, then
		// the rest in registry order.
		if got := ids(&AppState{CurrentVersion: other.ID, fullPending: true, staleVersions: map[string]bool{link.ID: true, other.ID: true}}); got != other.ID+" "+defaultVersionID+" "+link.ID {
			t.Errorf("owed %q, want the translation on screen, then the default, then the rest", got)
		}
		withVersionSource(t, other.ID, nil)
		if got := ids(&AppState{CurrentVersion: nk.ID, staleVersions: map[string]bool{other.ID: true}}); got != "" {
			t.Errorf("a placeholder is owed an upgrade: %q", got)
		}
	})
}

// TestTheArrivalMarkBelongsToItsLoad is D19's guard. Every route starts from
// the state the launch's restore records when it cannot open the reader's
// licensed translation, which adoptLaunch now carries to the screen (D18).
// Who asked for a translation is the cause its load was started with,
// carried by that load to its own landing: a link's failed load leaves
// nothing behind, a link parked behind the reader's own load gives that load
// nothing, and the reader's choice spends the remembered translation however
// the loads around it end. Every load starts at the app's own entry point —
// a link through applyShareTarget, a choice through switchVersionInteractive
// or the picker's row — and lands through the tail that call built, so the
// cause is the app's own from the call to the landing.
//
// It used to reproduce D19, when the mark was one flag that whichever load
// landed next read and cleared.
func TestTheArrivalMarkBelongsToItsLoad(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	link, _ := versionByID(arrivalLinkVersion)
	other, _ := versionByID(arrivalOtherVersion)

	// honoured asserts the reader's choice was taken as theirs: nothing is
	// remembered, the next save names it, and the picker no longer calls it
	// a substitution.
	honoured := func(t *testing.T, st *AppState, route, choice string) {
		t.Helper()
		if saved := snapshotReadingState(st, 0, 0, 0, 0, 0).Version; st.preferredVersion != "" || saved != choice {
			t.Fatalf("%s: the reader chose %s and it was taken for an arrival: remembered %q, next save %q",
				route, choice, st.preferredVersion, saved)
		}
		if n := fullPendingNotice(st); strings.Contains(n, "could not be opened") {
			t.Fatalf("%s: the picker still calls the reader's own choice a substitution: %q", route, n)
		}
	}

	for _, tc := range []struct {
		disk   arrivalDisk
		route  []arrivalEvent
		choice string
	}{
		// A link's load fails with nothing to fall back on; the reader then
		// chooses a translation of their own.
		{arDiskNone, []arrivalEvent{arLinkNamesOther, arFetchFails, arReaderPicksOther}, other.ID},
		// A link parks behind the reader's own load of another translation.
		{arDiskCurrent, []arrivalEvent{arReaderStartsOther, arLinkNamesOther, arFetchLands}, other.ID},
		// A link parks behind the reader's own load of the same translation,
		// whose landing is the reader's and opens the link's passage.
		{arDiskCurrent, []arrivalEvent{arReaderStartsIt, arLinkNamesOther, arFetchLands}, link.ID},
	} {
		route := fmt.Sprintf("%s (%s)", pathString(tc.route), tc.disk)
		w := newArrivalWorld(t, tc.disk)
		st, told := freshArrivalState(arrivalRemembered)
		for _, e := range tc.route {
			if did, _ := e.apply(t, w, st, &told); !did {
				t.Fatalf("control: %s did nothing in %s, so the route is not walked", e, route)
			}
		}
		if st.CurrentVersion != tc.choice || st.pendingLink != nil || st.versionLoading {
			t.Fatalf("control: %s must end on the reader's %s with nothing waiting or loading; on %s", route, tc.choice, st.CurrentVersion)
		}
		honoured(t, st, route, tc.choice)
		if tc.choice == link.ID {
			if got := fmt.Sprintf("%s|%d", st.CurrentBook, st.CurrentChapter); got != fmt.Sprintf("%s|%d", arrivalLinkTarget.Book, arrivalLinkTarget.Chapter) {
				t.Fatalf("%s: the link parked behind the reader's load of its translation did not open its passage; on %s", route, got)
			}
		}
	}

	// The evicted spinner: a rebuild takes the spinner down while a link's
	// load is in flight, and the reader picks a translation already in memory
	// from the real picker, which swaps at once. That landing is the reader's;
	// the link's, when it comes, is the link's.
	t.Run("evicted spinner", func(t *testing.T) {
		w := newArrivalWorld(t, arDiskCurrent)
		st, told := freshArrivalState(arrivalRemembered)
		st.loadedVersions[other.ID] = stampedBible("current") // read earlier this session
		win := app.NewWindow("evicted spinner")
		defer win.Close()
		st.window = win
		if did, _ := arLinkNamesOther.apply(t, w, st, &told); !did || !st.versionLoading {
			t.Fatal("control: the link must start a load of its own")
		}
		showVersionPicker(st)
		popup, ok := win.Canvas().Overlays().Top().(*widget.PopUp)
		if !ok {
			t.Fatalf("control: the picker did not open; top overlay %T", win.Canvas().Overlays().Top())
		}
		var row *tapBox
		walkTree(popup, func(n fyne.CanvasObject) {
			if tb, ok := n.(*tapBox); ok && row == nil && treeHasText(tb, other.Name+"  ("+other.Abbrev+")") {
				row = tb
			}
		})
		if row == nil {
			t.Fatalf("control: the picker has no row for %s", other.ID)
		}
		row.Tapped(&fyne.PointEvent{})
		if st.CurrentVersion != other.ID {
			t.Fatalf("control: the picker must swap to %s at once; on %s", other.ID, st.CurrentVersion)
		}
		honoured(t, st, "evicted spinner, the reader's pick", other.ID)
		if did, landed := arFetchLands.apply(t, w, st, &told); !did || landed != "arrival" {
			t.Fatalf("control: the link's load must land as the link's; did %v, landed %q", did, landed)
		}
		if st.preferredVersion != "" || strings.Contains(fullPendingNotice(st), "could not be opened") {
			t.Fatalf("after the link's landing: remembered %q, notice %q", st.preferredVersion, fullPendingNotice(st))
		}
	})

	// The reader picks a translation that is not in memory from the real
	// picker's row: its load leaves through the door, and its landing, through
	// the tail the row's call built, is the reader's.
	t.Run("the picker's row", func(t *testing.T) {
		w := newArrivalWorld(t, arDiskCurrent)
		st, _ := freshArrivalState(arrivalRemembered)
		win := app.NewWindow("the picker's row")
		defer win.Close()
		st.window = win
		showVersionPicker(st)
		popup, ok := win.Canvas().Overlays().Top().(*widget.PopUp)
		if !ok {
			t.Fatalf("control: the picker did not open; top overlay %T", win.Canvas().Overlays().Top())
		}
		var row *tapBox
		walkTree(popup, func(n fyne.CanvasObject) {
			if tb, ok := n.(*tapBox); ok && row == nil && treeHasText(tb, other.Name+"  ("+other.Abbrev+")") {
				row = tb
			}
		})
		if row == nil {
			t.Fatalf("control: the picker has no row for %s", other.ID)
		}
		row.Tapped(&fyne.PointEvent{})
		if !st.versionLoading || w.inflight != other.ID {
			t.Fatalf("control: the row must start a load of %s; loading %v, in flight %q", other.ID, st.versionLoading, w.inflight)
		}
		w.landInflight(t, st, true)
		if st.CurrentVersion != other.ID {
			t.Fatalf("control: the reader's load must land; on %s", st.CurrentVersion)
		}
		honoured(t, st, "the picker's row", other.ID)
	})

	// The cause is fixed by the call that starts the load, and the walk and
	// the routes above land every load through the tail that call built, so
	// a wrong cause anywhere from a call to its landing fails there. This
	// names the call: every switch switchToLinkVersion starts is the
	// arrival's, and every one the picker starts is the reader's.
	t.Run("the cause each entry point gives", func(t *testing.T) {
		src := parsePackageSource(t)
		for _, tc := range []struct {
			fn    string
			cause string
			calls int
		}{
			{"switchToLinkVersion", "byArrival", 2},
			{"showVersionPicker", "byReader", 1},
		} {
			decls := src.funcs[tc.fn]
			if len(decls) != 1 {
				t.Fatalf("control: %s is declared %d times, so this reading of the source is wrong", tc.fn, len(decls))
			}
			n := 0
			ast.Inspect(decls[0].Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				if id, ok := call.Fun.(*ast.Ident); !ok || (id.Name != "switchVersion" && id.Name != "switchVersionInteractive") {
					return true
				}
				n++
				if last, ok := call.Args[len(call.Args)-1].(*ast.Ident); !ok || last.Name != tc.cause {
					t.Errorf("%s starts a switch with the cause %s, want %s", tc.fn, types.ExprString(call.Args[len(call.Args)-1]), tc.cause)
				}
				return true
			})
			if n != tc.calls {
				t.Errorf("control: %s starts %d switches, where this reading expects %d", tc.fn, n, tc.calls)
			}
		}
	})
}

// TestAPreviousEditionIsSaidBehindASubstitution is D20's guard. The footer
// says every true fact, one per line, in the order a reader asks: which
// translation, then which edition. Ranking them let the substitution sentence
// silence the edition. The sentences are the ones the app already had; only
// their composition changed, so they are compared whole.
//
// It used to reproduce D20, when the footer gave one sentence and a previous
// edition behind a substitution was not said at all.
func TestAPreviousEditionIsSaidBehindASubstitution(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	link, _ := versionByID(arrivalLinkVersion)
	nk, _ := versionByID(arrivalRemembered)
	def, _ := versionByID(defaultVersionID)
	substituted := func(shown string) string {
		return nk.Name + " could not be opened this time — " + shown +
			" is shown instead. Your choice is remembered and tried again each time the app starts."
	}

	// The arrival: a link to a translation holding only its previous edition,
	// served that edition offline, with the reader's licensed translation
	// remembered.
	w := newArrivalWorld(t, arDiskPrevious)
	st, told := freshArrivalState(arrivalRemembered)
	arLinkNamesOther.apply(t, w, st, &told)
	arFetchFailsPreviousServes.apply(t, w, st, &told)
	if st.CurrentVersion != link.ID || bibleStamp(st.Bible) != "previous" || !st.staleVersions[link.ID] {
		t.Fatalf("control: the link must end on the previous edition, recorded as stale; on %s with %q", st.CurrentVersion, bibleStamp(st.Bible))
	}
	want := substituted(link.Name) + "\n" +
		link.Name + " is showing a previous edition until the update can be downloaded."
	if n := fullPendingNotice(st); n != want {
		t.Fatalf("the footer behind a substitution reads\n  %q\nwant the substitution, then the edition:\n  %q", n, want)
	}

	// The launch: the remembered translation could not be opened, and the
	// default serves its previous edition, waiting offline — and then with
	// its update downloading.
	st = &AppState{CurrentVersion: defaultVersionID, preferredVersion: nk.ID, fullPending: true, fullRetryDelay: 20 * time.Second}
	want = substituted(def.Name) + "\n" +
		def.Name + " has a text update waiting for a connection — the previous edition is shown meanwhile. It retries automatically."
	if n := fullPendingNotice(st); n != want {
		t.Fatalf("the launch's footer reads\n  %q\nwant\n  %q", n, want)
	}
	st.fullDownloading = true
	want = substituted(def.Name) + "\n" +
		def.Name + " is updating to its latest edition in the background — the previous edition is shown meanwhile."
	if n := fullPendingNotice(st); n != want {
		t.Fatalf("the launch's footer while downloading reads\n  %q\nwant\n  %q", n, want)
	}

	// D21: the default's own sentences describe the default on screen. On
	// another translation its previous edition is said as any other
	// translation's is, behind the substitution too.
	st = &AppState{CurrentVersion: arrivalOtherVersion, preferredVersion: nk.ID, fullPending: true, fullRetryDelay: 20 * time.Second}
	other, _ := versionByID(arrivalOtherVersion)
	want = substituted(other.Name) + "\n" +
		def.Name + " is showing a previous edition until the update can be downloaded."
	if n := fullPendingNotice(st); n != want {
		t.Fatalf("on %s the footer reads\n  %q\nwant\n  %q", arrivalOtherVersion, n, want)
	}
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

	w := newArrivalWorld(t, arDiskNone)
	st, told := freshArrivalState("")
	here := fmt.Sprintf("%s|%d", st.CurrentBook, st.CurrentChapter)

	// The reader taps a shared link naming a translation they do not have in
	// memory. The target parks and the load owns the spinner.
	arLinkNamesOther.apply(t, w, st, &told)
	if st.pendingLink == nil || st.pendingLinkVersion != arrivalLinkVersion {
		t.Fatal("control: the link must park on the translation it named, or the rest of this proves nothing")
	}

	// The load fails and the reader is told so. The promise is now closed.
	arFetchFails.apply(t, w, st, &told)
	if !told {
		t.Fatal("control: the reader must have been told the load failed")
	}
	if st.pendingLink != nil {
		t.Fatal("a park is still waiting for a load that will never come")
	}

	// Much later, for their own reasons, the reader chooses that same
	// translation from the picker. It works this time.
	arReaderPicksIt.apply(t, w, st, &told)
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

	w := newArrivalWorld(t, arDiskNone)
	st, told := freshArrivalState("")
	arLinkNamesOther.apply(t, w, st, &told)
	// Re-point the park at some other translation, as a second arrival would.
	st.pendingLinkVersion = arrivalOtherVersion

	arFetchFails.apply(t, w, st, &told)
	if !told {
		t.Fatal("control: the load must have failed, or the park was never at risk")
	}
	if st.pendingLink == nil || st.pendingLinkVersion != arrivalOtherVersion {
		t.Fatal("a park waiting on another translation was cleared by this load's failure")
	}
}

// TestAnArrivalDoesNotSpendTheReadersRememberedTranslation is D13.
//
// It is the D9/D10 record being deleted through a caller neither of them
// modelled, and it is the sharpest case in this document of a fix creating an
// obligation somewhere else: D10 made the app PROMISE, in writing on the
// picker, that the reader's translation is "remembered and comes back when it
// can". applyLoadedVersion then spent that record on any successful load — and
// a tapped link is a successful load the reader did not ask for.
func TestAnArrivalDoesNotSpendTheReadersRememberedTranslation(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	nk, ok := versionByID("nkjv")
	if !ok {
		t.Skip("nkjv not registered")
	}

	newFallbackState := func() *AppState {
		base := fullValidBible()
		return &AppState{
			Bible:          base,
			CurrentVersion: defaultVersionID,
			currentMode:    modeReal,
			loadedVersions: map[string]*BibleData{defaultVersionID: base},
			loadPhase:      loadReady,
			CurrentBook:    "Genesis",
			CurrentChapter: 1,
			// the state D9 records and D10 announces
			preferredVersion: nk.ID,
		}
	}

	// CONTROL: the promise is really being made in this state, and the record
	// really is what the next save would write. Without this the test could
	// pass against a build that never made the promise at all.
	if n := fullPendingNotice(newFallbackState()); !strings.Contains(n, nk.Name) {
		t.Fatalf("control: the picker must be promising the reader their translation; got %q", n)
	}
	if got := snapshotReadingState(newFallbackState(), 0, 0, 0, 0, 0).Version; got != nk.ID {
		t.Fatalf("control: the record must name the reader's translation; got %q", got)
	}

	// A friend's link in some OTHER translation switches for them: the
	// landing its load carries byArrival to, and the whole link through the
	// real entry point, with that translation already in memory so the switch
	// is synchronous.
	other, _ := versionByID(arrivalOtherVersion)
	landed := newFallbackState()
	applyLoadedVersion(landed, other, fullValidBible(), modeReal, byArrival) // what switchToLinkVersion's load carries
	tapped := newFallbackState()
	tapped.loadedVersions[other.ID] = fullValidBible()
	applyShareTarget(tapped, ShareTarget{VersionID: other.ID, Book: "John", Chapter: 1, VerseLo: 1})
	if tapped.CurrentVersion != other.ID {
		t.Fatalf("control: the link must switch to %s; on %s", other.ID, tapped.CurrentVersion)
	}
	for _, st := range []*AppState{landed, tapped} {
		if st.preferredVersion != nk.ID {
			t.Fatal("somebody else's link spent the reader's remembered translation")
		}
		if got := snapshotReadingState(st, 0, 0, 0, 0, 0).Version; got != nk.ID {
			t.Fatalf("the next save would write %q over the reader's choice", got)
		}
		if n := fullPendingNotice(st); !strings.Contains(n, nk.Name) {
			t.Fatalf("the promise went silent without being kept; notice = %q", n)
		}
	}

	// The reader's OWN switch does spend it — that is the pinned behaviour the
	// exception must not break.
	st := newFallbackState()
	applyLoadedVersion(st, other, fullValidBible(), modeReal, byReader)
	if st.preferredVersion != "" {
		t.Fatal("an explicit switch must still spend the fallback preference")
	}

	// And so does the chosen translation finally arriving, however it arrives:
	// a link TO it is exactly the thing coming back.
	st = newFallbackState()
	st.loadedVersions[nk.ID] = fullValidBible()
	applyLoadedVersion(st, nk, fullValidBible(), modeReal, byArrival)
	if st.preferredVersion != "" {
		t.Fatal("the chosen translation came back and the fallback record outlived it")
	}
	if n := fullPendingNotice(st); n != "" {
		t.Fatalf("the notice outlived the substitution it describes: %q", n)
	}
}

// TestADisplacedLinkIsNotDroppedInSilence is D14.
//
// A link tapped while some other translation is already downloading parks
// behind that load. When the other translation lands it takes the screen and
// the park is dropped — correctly, since the target is stale — but until now
// without a word. The reader had tapped shared scripture and nothing whatever
// happened: no passage, no message, and because the platform glue always
// reports a bibletext.co.uk link as handled, no browser fallback either. It is
// a dead end of exactly the kind share_link_flow_test.go exists to forbid,
// reached by an axis that enumeration does not have.
func TestADisplacedLinkIsNotDroppedInSilence(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	wanted, ok := versionByID(arrivalLinkVersion)
	if !ok {
		t.Skip(arrivalLinkVersion + " not registered")
	}
	target := ShareTarget{VersionID: wanted.ID, Book: "John", Chapter: 3, VerseLo: 16}

	msg := linkDisplacedMessage(&AppState{}, target, wanted.ID)
	if msg == "" {
		t.Fatal("a displaced link says nothing at all — the tap looks to the reader like it missed")
	}
	if !strings.Contains(msg, wanted.Name) {
		t.Fatalf("the message must name the translation the link opens in; got %q", msg)
	}

	// It is a question about the state, not a sentence generator: with nothing
	// displaced there is nothing to say. Without this the enumerations that ask
	// "was anything said" would read every state as answered and go vacuous —
	// the mistake linkBookUnavailableMessage records having made.
	if got := linkDisplacedMessage(&AppState{}, ShareTarget{}, ""); got != "" {
		t.Fatalf("nothing was displaced and it spoke anyway: %q", got)
	}
	if got := linkDisplacedMessage(&AppState{}, target, "not-a-version"); got != "" {
		t.Fatalf("an unknown translation is a link from a future BibleText; it degrades quietly: %q", got)
	}
}
