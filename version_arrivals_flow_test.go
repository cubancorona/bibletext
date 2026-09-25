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
// arrivals in hand and one mark to tell them apart by.
//
// The cross-product that DOES cover arrivals already exists and is a different
// question: share_link_flow_test.go asks "is any state a dead end", over the
// link's own axes. This one asks what the arrival does to, and learns from,
// the VERSION machinery — the half docs/VERSION_STATES.md records as unmodelled.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

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
	// "reader" for a choice of the reader's own, "arrival" for a link's, ""
	// when nothing landed. The walk knows it from what it did rather than
	// from any field, because the field that records it is one of the things
	// being checked.
	landed string

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
// translation is fetched from, and what the reader brings to the journey. A
// landing rewrites the disk, so every journey puts it back before it replays.
type arrivalWorld struct {
	disk     arrivalDisk
	src      *arrivalSource
	current  []byte // the link's translation's current edition, as the real writer leaves it
	previous []byte // its previous edition, likewise

	// preferred is the translation every journey starts out remembering, ""
	// for none: the record the restore makes when the launch cannot open the
	// reader's licensed translation (D9). D18 keeps it off the live state
	// today, so the journeys that start with it walk the app as it will be the
	// day D18 is fixed.
	preferred string

	// The load in flight: which translation, and whether the reader asked for
	// it. Production keeps both in switchVersionInteractive's goroutine and no
	// field records them, so the walk does. "" when nothing is loading.
	inflight       string
	inflightReader bool
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
func newArrivalWorld(t *testing.T, disk arrivalDisk) *arrivalWorld {
	t.Helper()
	t.Setenv("BIBLETEXT_CACHE_PATH", filepath.Join(t.TempDir(), cacheFileName))
	w := &arrivalWorld{disk: disk, src: &arrivalSource{}}
	withVersionSource(t, arrivalLinkVersion, w.src)
	// The other translation is only ever read from its cache, so its source
	// is never online: a journey that fetched it would fail rather than pass.
	withVersionSource(t, arrivalOtherVersion, &arrivalSource{offline: true})

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
	w.inflight, w.inflightReader = "", false
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

// loadTail is switchVersionInteractive's goroutine and the fyne.Do tail it
// ends in (versions_ui.go), verbatim from the load down but for the spinner
// and the error card, which need a window. The error card's one effect a test
// can see is recorded in told. The load is over either way, so nothing is in
// flight after it.
func (w *arrivalWorld) loadTail(t *testing.T, st *AppState, v BibleVersion, online bool, told *bool) {
	t.Helper()
	w.inflight, w.inflightReader = "", false
	data, mode, err := w.load(t, st, v, online)
	st.versionLoading = false
	if err != nil {
		if old, oldMode, cerr := loadVersionFromCacheOnly(v); cerr == nil {
			if !versionCacheIsCurrent(v) {
				markVersionStale(st, v.ID) // D3: say so, do not serve it silently
			}
			applyLoadedVersion(st, v, old, oldMode)
			return
		}
		if st.pendingLinkVersion == v.ID {
			st.pendingLink = nil
			st.pendingLinkRaw = ""
			st.pendingLinkVersion = ""
			st.pendingNoteOpenID = 0
		}
		*told = true // showVersionLoadError
		return
	}
	applyLoadedVersion(st, v, data, mode)
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
)

func (e arrivalEvent) String() string {
	return [...]string{"link-names-other", "fetch-fails", "fetch-fails-previous-serves", "fetch-lands", "reader-picks-it", "reader-picks-other", "reader-starts-other"}[e]
}

// the translation a link in these journeys names, and one for the reader to
// wander off to.
const (
	arrivalLinkVersion  = "webc"
	arrivalOtherVersion = "bsb"
)

// arrivalLinkTarget is the passage every link in these journeys opens.
var arrivalLinkTarget = ShareTarget{VersionID: arrivalLinkVersion, Book: "John", Chapter: 1, VerseLo: 16}

// apply drives ONE event, and reports whether it did anything and who put a
// translation on screen, if one landed. Where production is a goroutine tail
// (switchVersionInteractive's fyne.Do body) the synchronous half is reproduced
// verbatim and cited, exactly as the refresh harness does — a test cannot
// await a goroutine deterministically. Where production is synchronous and
// cannot reach the network, the real function runs.
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
		if st.versionLoading {
			if w.inflight == arrivalLinkVersion {
				// Tapped again while its own fetch runs: the park it makes is
				// the one already there.
				return false, ""
			}
			// The reader's own load is running, so the link parks behind it:
			// the real applyShareTarget, whose switchToLinkVersion parks,
			// marks the switch as not the reader's, and returns without a
			// load of its own. When the reader's load lands it takes the
			// screen, and the park is dropped and said (D14).
			applyShareTarget(st, arrivalLinkTarget)
			*told = false
			return true, ""
		}
		if _, inMem := st.loadedVersions[arrivalLinkVersion]; inMem {
			// Already in memory, so switchToLinkVersion switches synchronously
			// and applyShareTarget opens the passage in whatever it now holds —
			// the real functions, end to end. Nothing on this branch fetches.
			applyShareTarget(st, arrivalLinkTarget)
			return true, "arrival"
		}
		// switchToLinkVersion's real-fetch branch, verbatim
		// (share_link_open.go): park the target, name the translation it waits
		// on, mark the switch as not the reader's, and let
		// switchVersionInteractive own the spinner.
		parked := arrivalLinkTarget
		st.pendingLink = &parked
		st.pendingLinkVersion = arrivalLinkVersion
		st.versionSwitchForArrival = true
		st.pendingNoteOpenID = 0
		st.versionLoading = true
		w.inflight, w.inflightReader = arrivalLinkVersion, false
		*told = false
		return true, ""
	case arFetchFails, arFetchFailsPreviousServes:
		if !st.versionLoading {
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
		w.loadTail(t, st, v, false, told)
		if e == arFetchFailsPreviousServes {
			return true, owner
		}
		return true, ""
	case arFetchLands:
		if !st.versionLoading {
			return false, ""
		}
		v, _ := versionByID(w.inflight)
		owner := w.inflightOwner()
		w.loadTail(t, st, v, true, told)
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
		v, _ := versionByID(id)
		// switchVersionInteractive (versions_ui.go): a translation already in
		// memory swaps synchronously through the real switchVersion; one that
		// is not loads behind the spinner — and it works, the network is up.
		if _, inMem := st.loadedVersions[id]; inMem {
			switchVersion(st, id)
			return true, "reader"
		}
		st.versionLoading = true
		w.loadTail(t, st, v, true, told)
		return true, "reader"
	case arReaderStartsOther:
		if st.versionLoading || st.CurrentVersion == arrivalOtherVersion {
			return false, ""
		}
		// In memory it swaps synchronously, which is reader-picks-other.
		if _, inMem := st.loadedVersions[arrivalOtherVersion]; inMem {
			return false, ""
		}
		// switchVersionInteractive's synchronous half (versions_ui.go): the
		// spinner goes up and the load leaves on its goroutine, which lands at
		// a later step. Nothing else is recorded, which is the point: the app
		// cannot tell this load from a link's except by the arrival mark.
		st.versionLoading = true
		w.inflight, w.inflightReader = arrivalOtherVersion, true
		return true, ""
	}
	return false, ""
}

func arrivalFactsOf(st *AppState, owed, told bool, landed string) arrivalFacts {
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
// D19 and D20 are reached only from a remembered translation, which D18 keeps
// off the live state today: they are what the reader meets the day D18 is
// fixed, and are pinned so that fix cannot ship them unseen.
var knownArrivalIncoherent = []pinnedArrivalDefect{
	{
		name: "D17",
		what: "a translation shown from its previous edition is not updated while the app runs: " +
			"nothing fetches it again, however long the reader is online",
		covers: func(bad string, _, now arrivalFacts, _ arrivalEvent) bool {
			return strings.HasPrefix(bad, "A-F:") && now.stuck &&
				now.current == arrivalLinkVersion && now.edition == "previous"
		},
	},
	{
		name: "D19",
		what: "the arrival mark is spent by a load it was not set for: the reader's own choice is taken " +
			"for an arrival and does not spend the translation they are told is remembered",
		covers: func(bad string, _, now arrivalFacts, _ arrivalEvent) bool {
			return strings.HasPrefix(bad, "A-G:") && now.landed == "reader" && now.preferred != ""
		},
	},
	{
		name: "D20",
		what: "a previous edition is on screen and the picker says only that a translation was " +
			"substituted: the substitution sentence outranks the stale one",
		covers: func(bad string, _, now arrivalFacts, _ arrivalEvent) bool {
			return strings.HasPrefix(bad, "A-E:") && now.stale &&
				now.preferred != "" && now.preferred != now.current
		},
	},
}

// The arrivals invariants. All of them are about a PROMISE over time, which is
// why they are checked after every step of a journey rather than once.
//
//	A-A  A park exists only while something can still consume it. Once the
//	     load it was waiting on has failed AND the reader has been told, the
//	     promise is closed and the park must be closed with it.
//	A-B  The reader's location changes only from what they just did. A park
//	     retired earlier must never move them later, and a translation they
//	     chose lands where they were.
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
func checkArrivalInvariants(prev, now arrivalFacts, _ arrivalEvent) []string {
	var bad []string
	if now.parked && now.failureTold && !now.loading {
		bad = append(bad, "A-A: the arrival was reported failed and its park is still waiting for a load that will never come")
	}
	// A translation the reader chose is not an arrival. If the location moved
	// when it landed, something else moved them.
	if now.landed == "reader" && now.loc != prev.loc {
		bad = append(bad, "A-B: choosing a translation moved the reader to a passage they did not ask for")
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

	events := []arrivalEvent{arLinkNamesOther, arFetchFails, arFetchFailsPreviousServes, arFetchLands, arReaderPicksIt, arReaderPicksOther, arReaderStartsOther}

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
					prev := arrivalFactsOf(st, false, told, "")
					var now arrivalFacts
					did := false
					for i, e := range next {
						var landed string
						if did, landed = e.apply(t, w, st, &told); !did {
							break
						}
						owed := st.pendingLink != nil && !told
						now = arrivalFactsOf(st, owed, told, landed)
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
				if sawPrevious || sawTold {
					t.Errorf("control: %s put a previous edition on screen or failed a load, and with the current edition on disk neither can happen", world)
				}
			case arDiskPrevious:
				if !sawPrevious || sawTold {
					t.Errorf("control: %s never served the previous edition, or failed a load it could not fail, so the arm it exists for was not walked", world)
				}
			case arDiskNone:
				if sawPrevious || !sawTold {
					t.Errorf("control: %s served a previous edition it does not have, or never failed a load", world)
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
		" is shown instead. Your choice is remembered and comes back when it can."
	for _, tc := range []struct {
		want      string
		prev, now arrivalFacts
	}{
		{"A-B", arrivalFacts{loc: "Genesis|1"}, arrivalFacts{loc: "John|1", current: arrivalOtherVersion, edition: "current", landed: "reader"}},
		{"A-C", arrivalFacts{}, arrivalFacts{current: link.ID, edition: "current", parked: true, arrivalOwed: true}},
		{"A-D", arrivalFacts{}, arrivalFacts{current: link.ID, edition: "previous", notice: said}},
		{"A-D", arrivalFacts{}, arrivalFacts{current: link.ID, edition: "current", stale: true, notice: said}},
		{"A-E", arrivalFacts{}, arrivalFacts{current: link.ID, edition: "previous", stale: true}},
		// D10's sentence names the translation on screen and says nothing of
		// its edition.
		{"A-E", arrivalFacts{}, arrivalFacts{current: link.ID, edition: "previous", stale: true, preferred: remembered.ID, notice: substituted}},
		{"A-G", arrivalFacts{preferred: remembered.ID}, arrivalFacts{current: arrivalOtherVersion, edition: "current", landed: "reader", preferred: remembered.ID}},
		{"A-G", arrivalFacts{preferred: remembered.ID}, arrivalFacts{current: link.ID, edition: "current", landed: "arrival"}},
	} {
		got := checkArrivalInvariants(tc.prev, tc.now, arLinkNamesOther)
		fired := false
		for _, bad := range got {
			fired = fired || strings.HasPrefix(bad, tc.want+":")
		}
		if !fired {
			t.Errorf("%s does not fire on %s; got %q", tc.want, tc.now, got)
		}
	}
	// And the states they must leave alone, or the walk would be all noise:
	// a park with its load still running, a previous edition that is recorded
	// and said, a link that lands and keeps the remembered translation, and a
	// choice of the reader's that spends it.
	for _, quiet := range []struct{ prev, now arrivalFacts }{
		{arrivalFacts{}, arrivalFacts{current: link.ID, edition: "current", parked: true, arrivalOwed: true, loading: true}},
		{arrivalFacts{}, arrivalFacts{current: link.ID, edition: "previous", stale: true, notice: said}},
		{arrivalFacts{preferred: remembered.ID}, arrivalFacts{current: link.ID, edition: "current", landed: "arrival", preferred: remembered.ID, notice: substituted}},
		{arrivalFacts{preferred: remembered.ID}, arrivalFacts{current: arrivalOtherVersion, edition: "current", landed: "reader"}},
	} {
		if got := checkArrivalInvariants(quiet.prev, quiet.now, arLinkNamesOther); len(got) > 0 {
			t.Errorf("a coherent state was reported: %s: %q", quiet.now, got)
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

// TestAPreviousEditionIsNotUpdatedWhileTheAppRuns reproduces D17, which is
// OPEN, through the app's own entry points rather than the walk's copies of
// them — the picker's switchVersionInteractive and a tapped link's
// applyShareTarget — with the network up throughout.
//
// It asserts what the app does TODAY, and says so: the day D17 is fixed it
// fails, and should then become the fix's own test with its assertions
// turned round, and the pin struck from knownArrivalIncoherent and from
// docs/VERSION_STATES.md.
func TestAPreviousEditionIsNotUpdatedWhileTheAppRuns(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	w := newArrivalWorld(t, arDiskPrevious)
	link, _ := versionByID(arrivalLinkVersion)
	st, told := freshArrivalState("")

	// Offline, a shared link to the translation parks, its fetch fails, and
	// the previous edition serves the passage, recorded and said.
	arLinkNamesOther.apply(t, w, st, &told)
	arFetchFailsPreviousServes.apply(t, w, st, &told)
	if st.CurrentVersion != link.ID || bibleStamp(st.Bible) != "previous" || !st.staleVersions[link.ID] {
		t.Fatalf("control: the link must end on the previous edition, recorded as stale; on %s with %q", st.CurrentVersion, bibleStamp(st.Bible))
	}
	if n := fullPendingNotice(st); !strings.Contains(n, "until the update can be downloaded") {
		t.Fatalf("control: the picker must be promising the update; got %q", n)
	}

	// The network is back, and stays back. The reader goes to another
	// translation and returns to this one through the picker, and then taps
	// another link to it: the two ways the app offers.
	before := w.src.fetches
	switchVersion(st, arrivalOtherVersion)
	switchVersionInteractive(st, link.ID)
	onPicker := bibleStamp(st.Bible)
	switchVersion(st, arrivalOtherVersion)
	applyShareTarget(st, ShareTarget{VersionID: link.ID, Book: "John", Chapter: 3, VerseLo: 16})
	if st.CurrentVersion != link.ID {
		t.Fatalf("control: the link must switch to %s; on %s", link.ID, st.CurrentVersion)
	}

	fetched := w.src.fetches - before
	if onPicker != "previous" || bibleStamp(st.Bible) != "previous" || fetched != 0 {
		t.Fatalf("D17 looks fixed: the picker served %q, the link %q, after %d fetch(es). "+
			"Strike D17 from knownArrivalIncoherent and docs/VERSION_STATES.md, and turn this test round into the fix's guard.",
			onPicker, bibleStamp(st.Bible), fetched)
	}
	if n := fullPendingNotice(st); !strings.Contains(n, "until the update can be downloaded") {
		t.Fatalf("the notice changed its promise; D17's record needs re-reading: %q", n)
	}
}

// TestTheArrivalMarkIsSpentByTheWrongLoad reproduces D19, which is OPEN, by
// its two shortest routes. Both start from the state the launch's restore
// records when it cannot open the reader's licensed translation — which D18
// keeps off the live state today, so the reader meets this the day D18 is
// fixed.
//
// versionSwitchForArrival is one flag with one consumer: the next
// applyLoadedVersion reads it to decide whether a landing was the reader's,
// and clears it. It is set for a link's load and read by whichever load lands
// next. On the first route the link's load fails with nothing to fall back
// on, never reaches applyLoadedVersion, and leaves the mark for the reader's
// next choice; on the second the link parks behind a load of the reader's
// own, which lands first and takes the mark. Either way the reader's own
// choice is taken for an arrival, the remembered translation is not spent,
// the picker goes on saying it could not be opened, and the next save writes
// it over the translation the reader has just chosen.
//
// It asserts what the app does TODAY. The day D19 is fixed it fails, and
// should be turned round into the fix's guard, with the pin struck from
// knownArrivalIncoherent and from docs/VERSION_STATES.md.
func TestTheArrivalMarkIsSpentByTheWrongLoad(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	other, _ := versionByID(arrivalOtherVersion)

	for _, tc := range []struct {
		disk  arrivalDisk
		route []arrivalEvent
	}{
		{arDiskNone, []arrivalEvent{arLinkNamesOther, arFetchFails, arReaderPicksOther}},
		{arDiskCurrent, []arrivalEvent{arReaderStartsOther, arLinkNamesOther, arFetchLands}},
	} {
		w := newArrivalWorld(t, tc.disk)
		st, told := freshArrivalState(arrivalRemembered)
		for _, e := range tc.route {
			if did, _ := e.apply(t, w, st, &told); !did {
				t.Fatalf("control: %s did nothing on %s, so the route %s is not walked", e, tc.disk, pathString(tc.route))
			}
		}
		if st.CurrentVersion != other.ID || st.pendingLink != nil {
			t.Fatalf("control: %s must end on the reader's %s with no link waiting; on %s", pathString(tc.route), other.ID, st.CurrentVersion)
		}
		saved := snapshotReadingState(st, 0, 0, 0, 0, 0).Version
		if st.preferredVersion != arrivalRemembered || saved != arrivalRemembered {
			t.Fatalf("D19's route %s (%s) looks closed: remembered %q, next save %q. A fix must close both "+
				"routes; when it does, strike D19 from knownArrivalIncoherent and docs/VERSION_STATES.md, "+
				"and turn this test round into the fix's guard.",
				pathString(tc.route), tc.disk, st.preferredVersion, saved)
		}
		if n := fullPendingNotice(st); !strings.Contains(n, "could not be opened") || !strings.Contains(n, other.Name) {
			t.Fatalf("the picker no longer says the reader's choice was a substitution; D19's record needs re-reading: %q", n)
		}
	}
}

// TestAPreviousEditionIsSilentBehindASubstitution reproduces D20, which is
// OPEN, at its shortest route. Like D19 it starts from the remembered
// translation D18 keeps off the live state today.
//
// fullPendingNotice has one sentence to give and ranks the substitution (D10)
// above the previous edition (D3). So when a link to a translation holding
// only its previous edition is served that edition offline, the record is
// made and the picker says only that the remembered translation could not be
// opened and this one is shown instead. Nothing says the text on screen is a
// previous edition: D3's silence, reached through D10's sentence.
//
// It asserts what the app does TODAY. The day D20 is fixed it fails, and
// should be turned round into the fix's guard, with the pin struck from
// knownArrivalIncoherent and from docs/VERSION_STATES.md.
func TestAPreviousEditionIsSilentBehindASubstitution(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	link, _ := versionByID(arrivalLinkVersion)

	w := newArrivalWorld(t, arDiskPrevious)
	st, told := freshArrivalState(arrivalRemembered)
	arLinkNamesOther.apply(t, w, st, &told)
	arFetchFailsPreviousServes.apply(t, w, st, &told)
	if st.CurrentVersion != link.ID || bibleStamp(st.Bible) != "previous" || !st.staleVersions[link.ID] {
		t.Fatalf("control: the link must end on the previous edition, recorded as stale; on %s with %q", st.CurrentVersion, bibleStamp(st.Bible))
	}
	n := fullPendingNotice(st)
	if !strings.Contains(n, "could not be opened") || !strings.Contains(n, link.Name) {
		t.Fatalf("control: the picker must be reporting the substitution; got %q", n)
	}
	if strings.Contains(n, "previous edition") {
		t.Fatalf("D20 looks fixed: the picker says %q. Strike D20 from knownArrivalIncoherent and "+
			"docs/VERSION_STATES.md, and turn this test round into the fix's guard.", n)
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

	// A friend's link in some OTHER translation switches for them.
	st := newFallbackState()
	st.versionSwitchForArrival = true // what switchToLinkVersion sets
	other, _ := versionByID(arrivalOtherVersion)
	applyLoadedVersion(st, other, fullValidBible(), modeReal)

	if st.preferredVersion != nk.ID {
		t.Fatal("somebody else's link spent the reader's remembered translation")
	}
	if got := snapshotReadingState(st, 0, 0, 0, 0, 0).Version; got != nk.ID {
		t.Fatalf("the next save would write %q over the reader's choice", got)
	}
	if n := fullPendingNotice(st); !strings.Contains(n, nk.Name) {
		t.Fatalf("the promise went silent without being kept; notice = %q", n)
	}

	// The reader's OWN switch does spend it — that is the pinned behaviour the
	// exception must not break.
	st = newFallbackState()
	applyLoadedVersion(st, other, fullValidBible(), modeReal)
	if st.preferredVersion != "" {
		t.Fatal("an explicit switch must still spend the fallback preference")
	}

	// And so does the chosen translation finally arriving, however it arrives:
	// a link TO it is exactly the thing coming back.
	st = newFallbackState()
	st.versionSwitchForArrival = true
	st.loadedVersions[nk.ID] = fullValidBible()
	applyLoadedVersion(st, nk, fullValidBible(), modeReal)
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
