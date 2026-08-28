package bibletext

// The life of a Bible VERSION, enumerated. See docs/VERSION_STATES.md.
//
// WHY AN ENUMERATION AND NOT A LIST OF CASES. What a version serves is decided
// by seven functions that each answer a slightly different question about the
// same files on disk — does a cache exist, does it load, is it current, is it
// stale, may it be served, may it be deleted — and the reader sees only the
// answer, never which function gave it. Every defect this file pins was
// reachable by a combination nobody had written a case for, and each was found
// by walking the cross-product rather than by inspection. This is the shape
// notes_state_flow_test.go has, for the same reason.
//
// HOW IT FAILS USEFULLY. Every violation reachable TODAY is pinned in
// knownVersionIncoherent below, by the name docs/VERSION_STATES.md gives it,
// and the assertion is on set EQUALITY — so it fails when a NEW incoherent
// state appears AND when a pinned one is fixed without being struck off. The
// second half is the point: a fix that leaves the ledger stale makes the next
// reader trust a document that lies.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
)

// --- the fake source (the keystone the suite could not exist without) -------
//
// The public-domain sources build their HTTP client inline against const URLs
// (bsb.go), so a version whose fetch can be made to succeed or fail on demand
// has to be registered for the cell. isLicensedSource is a TYPE assertion
// (versions.go), so a fake can never be licensed — hence the two lanes below.

type fakeVersionSource struct {
	avail bool
	data  *BibleData
	err   error
	calls *int
}

func (s *fakeVersionSource) available() bool { return s.avail }

func (s *fakeVersionSource) fetch() (*BibleData, error) {
	if s.calls != nil {
		*s.calls++
	}
	if s.err != nil {
		return nil, s.err
	}
	return s.data, nil
}

// withRegisteredVersion adds v to the registry for the duration of the test.
// Restored on cleanup: versions_test.go enumerates the catalogue exactly, and
// several suites iterate the whole registry.
func withRegisteredVersion(t *testing.T, v BibleVersion) {
	t.Helper()
	prev := registeredVersions
	registeredVersions = append(append([]BibleVersion(nil), prev...), v)
	t.Cleanup(func() { registeredVersions = prev })
}

// stampedBible builds a cacheable Bible whose verse text names the generation,
// so "which decode is on screen" is assertable by text rather than by pointer.
func stampedBible(stamp string) *BibleData {
	bd := fullValidBible()
	for _, book := range bd.Books {
		for ch, verses := range bd.Verses[book] {
			for i := range verses {
				verses[i].Text = stamp + " " + verses[i].Text
			}
			bd.Verses[book][ch] = verses
		}
	}
	bd.PrepareSearchIndex()
	return bd
}

func bibleStamp(bd *BibleData) string {
	if bd == nil {
		return "<nil>"
	}
	for _, book := range bd.Books {
		for _, verses := range bd.Verses[book] {
			if len(verses) > 0 {
				if i := len(verses[0].Text); i > 0 {
					fields := verses[0].Text
					for j := 0; j < len(fields); j++ {
						if fields[j] == ' ' {
							return fields[:j]
						}
					}
				}
			}
		}
	}
	return "<unstamped>"
}

// --- the axes ---------------------------------------------------------------

// storageShape is what is on disk for the version when the cell starts. These
// are the five distinguishable shapes the seven functions can disagree about.
type storageShape int

const (
	stoAbsent          storageShape = iota // nothing at all
	stoCurrentOnly                         // a valid cache at the current epoch
	stoSupersededOnly                      // a valid cache at epoch-1, nothing current
	stoBoth                                // valid at both
	stoCorruptCurrent                      // UNREADABLE bytes at the current epoch, valid superseded
)

func (s storageShape) String() string {
	return [...]string{"absent", "current-only", "superseded-only", "both", "corrupt-current"}[s]
}

// storageEvent is the question asked of that disk state.
type storageEvent int

const (
	evCacheOnly    storageEvent = iota // loadVersionFromCacheOnly — the startup read
	evIsCurrent                        // versionCacheIsCurrent — the refresh decision
	evPurgeOldEpoc                     // purgeSupersededCaches
)

func (e storageEvent) String() string {
	return [...]string{"cache-only-read", "is-current", "purge-superseded"}[e]
}

// versionObs is everything the cell observed, in the vocabulary the model uses.
type versionObs struct {
	shape storageShape
	event storageEvent

	served     string // the stamp of the decode that was served, "" = nothing
	servedFrom string // "current" | "superseded" | "none"
	loadErr    bool
	isCurrent  bool   // what versionCacheIsCurrent answered
	refresh    bool   // would a launch schedule the upgrade? (!isCurrent)
	notice     string // what the picker would say about it
	filesLeft  int    // after a purge event
}

// A violation is a name from docs/VERSION_STATES.md.
type pinnedVersionDefect struct {
	name   string
	what   string
	covers func(o versionObs) bool
}

// knownVersionIncoherent — every incoherent state reachable TODAY. Strike an
// entry the moment its defect is fixed; the set-equality assertion below fails
// if a pin goes stale.
// V1 was struck when versionCacheIsCurrent was made to LOAD rather than stat
// (versions.go) and saveBibleToCache learned to fsync before renaming
// (cache.go). No cell reaches an incoherent state today.
var knownVersionIncoherent = []pinnedVersionDefect{}

// invariants — what every cell must satisfy unless a pin excuses it.
//
//	V-A  Nothing is served that was not loadable.
//	V-B  A version serving a superseded epoch is scheduled for upgrade.
//	V-C  A stale-serving state is never silent (the notice exists).
//	V-D  A purge never removes the only readable copy.
func checkVersionInvariants(o versionObs) []string {
	var bad []string
	switch o.event {
	case evCacheOnly:
		if o.served != "" && o.loadErr {
			bad = append(bad, "V-A: served text while reporting a load error")
		}
	case evIsCurrent:
		// The upgrade decision must agree with what the reader is actually
		// looking at: if a superseded decode is what serves, the refresh must
		// be scheduled and the reader must be told.
		if o.servedFrom == "superseded" && !o.refresh {
			bad = append(bad, "V-B: serving a superseded epoch with no upgrade scheduled")
		}
		if o.servedFrom == "superseded" && o.notice == "" {
			bad = append(bad, "V-C: serving a superseded epoch silently")
		}
	case evPurgeOldEpoc:
		if o.filesLeft == 0 {
			bad = append(bad, "V-D: purge left no readable copy")
		}
	}
	return bad
}

// --- enumeration A: the storage space ---------------------------------------

func TestVersionStorageStateSpace(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	const testID = "vstest"
	var unexplained []string
	seen := map[string]bool{}
	cells, skipped := 0, 0

	for shape := stoAbsent; shape <= stoCorruptCurrent; shape++ {
		for event := evCacheOnly; event <= evPurgeOldEpoc; event++ {
			name := fmt.Sprintf("%s/%s", shape, event)
			t.Run(name, func(t *testing.T) {
				dir := t.TempDir()
				t.Setenv("BIBLETEXT_CACHE_PATH", filepath.Join(dir, "bibletext-cache.json"))
				withFakeSharedKeys(t)

				calls := 0
				v := BibleVersion{
					ID: testID, Name: "Version-State Test", Abbrev: "VST",
					Publisher: "Test", PublicDomain: true,
					cacheEpoch: 2,
					source:     &fakeVersionSource{avail: true, data: stampedBible("fresh"), calls: &calls},
				}
				withRegisteredVersion(t, v)

				current := cachePathForVersion(testID)
				superseded := supersededCachePaths(v)[0] // epoch 1

				// Build the disk state through the REAL writer, so a "valid"
				// cache is valid in exactly the way the app makes one.
				switch shape {
				case stoAbsent:
				case stoCurrentOnly:
					mustCache(t, current, stampedBible("current"))
				case stoSupersededOnly:
					mustCache(t, superseded, stampedBible("old"))
				case stoBoth:
					mustCache(t, current, stampedBible("current"))
					mustCache(t, superseded, stampedBible("old"))
				case stoCorruptCurrent:
					mustCache(t, superseded, stampedBible("old"))
					// The realistic corruption: the rename landed, the bytes
					// did not (see V2 in docs/VERSION_STATES.md).
					if err := os.WriteFile(current, nil, 0o644); err != nil {
						t.Fatal(err)
					}
				}

				obs := versionObs{shape: shape, event: event, servedFrom: "none"}
				switch event {
				case evCacheOnly:
					data, _, err := loadVersionFromCacheOnly(v)
					obs.loadErr = err != nil
					if err == nil {
						obs.served = bibleStamp(data)
						obs.servedFrom = servedFrom(obs.served)
					}
				case evIsCurrent:
					obs.isCurrent = versionCacheIsCurrent(v)
					obs.refresh = !obs.isCurrent
					data, _, err := loadVersionFromCacheOnly(v)
					if err == nil {
						obs.served = bibleStamp(data)
						obs.servedFrom = servedFrom(obs.served)
					}
					// What the reader would be told, on the launch this
					// decision drives.
					st := &AppState{fullPending: obs.refresh}
					obs.notice = fullPendingNotice(st)
				case evPurgeOldEpoc:
					// THROUGH THE REAL ENTRY POINT, never purgeSupersededCaches
					// directly: the app purges only from inside loadVersionData,
					// AFTER a load succeeded and wrote the current epoch
					// (versions.go — the incident comment). Calling the purge on
					// its own invents states the app does not have, and an
					// enumeration that invents states reports false defects.
					if _, _, err := loadVersionData(v, nil); err != nil {
						obs.loadErr = true
						skipped++ // the app would not have reached the purge
						return
					}
					obs.filesLeft = countReadableCaches(t, v)
				}
				cells++

				for _, bad := range checkVersionInvariants(obs) {
					explained := false
					for _, d := range knownVersionIncoherent {
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

	sort.Strings(unexplained)
	for _, u := range unexplained {
		t.Errorf("unpinned incoherent state — %s\n"+
			"If this is a real defect, name it in docs/VERSION_STATES.md and pin it "+
			"in knownVersionIncoherent. If it is not, the invariant is wrong.", u)
	}
	for _, d := range knownVersionIncoherent {
		if !seen[d.name] {
			t.Errorf("%s is pinned but no cell reaches it any more (%s).\n"+
				"Strike it from knownVersionIncoherent AND from docs/VERSION_STATES.md — "+
				"a pinned list that outlives its defect makes the document lie.", d.name, d.what)
		}
	}
	t.Logf("storage space: %d cells, %d skipped, %d pinned defects reached", cells, skipped, len(seen))
}

// servedFrom maps a decode's stamp back to the epoch it came from.
func servedFrom(stamp string) string {
	switch stamp {
	case "current", "fresh":
		return "current"
	case "old":
		return "superseded"
	default:
		return "none"
	}
}

func mustCache(t *testing.T, path string, bd *BibleData) {
	t.Helper()
	if err := saveBibleToCache(path, bd, currentUTCTime); err != nil {
		t.Fatal(err)
	}
}

func countReadableCaches(t *testing.T, v BibleVersion) int {
	t.Helper()
	n := 0
	paths := append([]string{cachePathForVersion(v.ID)}, supersededCachePaths(v)...)
	for _, p := range paths {
		if _, err := loadBibleFromCache(p); err == nil {
			n++
		}
	}
	return n
}

// --- the durability half of V1 ----------------------------------------------

// The currency check and the serve path must give the SAME answer for every
// shape of unusable file — this is what makes V1 unreachable rather than
// merely unlikely. Each case is a file that exists (so the old stat-based
// check called it current) and cannot be served.
func TestCurrencyCheckAgreesWithTheServePath(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	for _, tc := range []struct {
		name  string
		write func(t *testing.T, path string)
	}{
		{"zero length", func(t *testing.T, p string) {
			if err := os.WriteFile(p, nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{"truncated json", func(t *testing.T, p string) {
			if err := os.WriteFile(p, []byte(`{"version":1,"data":{"Books":["Gen`), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{"wrong schema version", func(t *testing.T, p string) {
			if err := os.WriteFile(p, []byte(`{"version":9999,"data":{}}`), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{"a directory", func(t *testing.T, p string) {
			if err := os.MkdirAll(p, 0o755); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("BIBLETEXT_CACHE_PATH", filepath.Join(dir, "bibletext-cache.json"))
			v := BibleVersion{
				ID: "vsdur", Name: "Durability", Abbrev: "DUR", PublicDomain: true,
				cacheEpoch: 2, source: &fakeVersionSource{avail: true, data: fullValidBible()},
			}
			withRegisteredVersion(t, v)
			path := cachePathForVersion("vsdur")
			tc.write(t, path)

			// The control: the file EXISTS, so the question is not trivial.
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("control: the file must exist, else the case proves nothing: %v", err)
			}
			servable := false
			if _, err := loadBibleFromCache(path); err == nil {
				servable = true
			}
			if servable {
				t.Fatal("control: this shape must not be servable")
			}
			if versionCacheIsCurrent(v) {
				t.Error("currency check says current for a cache that cannot be served — " +
					"the reader would be pinned on the previous epoch with no refresh and no notice (V1)")
			}
		})
	}
}

// The durability half: a cache write must be fsynced before the rename, so a
// power loss cannot leave the current-epoch name pointing at half a file.
// Asserted on the source, because the property is unobservable from inside a
// process that never loses power — the same shape as the release scripts
// being asserted rather than trusted (dev_links_guard_test.go).
func TestCacheWriteSyncsBeforeRename(t *testing.T) {
	src, err := os.ReadFile("cache.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)
	syncAt := strings.Index(body, ".Sync()")
	renameAt := strings.Index(body, "os.Rename(tmpPath, path)")
	if syncAt < 0 {
		t.Fatal("saveBibleToCache must fsync the temp file: without it the rename can " +
			"publish a name whose bytes never reached the disk (V2)")
	}
	if renameAt < 0 {
		t.Fatal("the tmp+rename activation is gone; re-read this test's premise")
	}
	if syncAt > renameAt {
		t.Error("the fsync must come BEFORE the rename, or it guarantees nothing")
	}
}

// --- the licensed lane's age axis -------------------------------------------

// A licensed version past its recency window must never be SERVED from cache —
// the §11 obligation. This walks the boundary from both sides through the real
// predicate rather than asserting the constant.
func TestLicensedRecencyWindowGovernsTheServe(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	for _, tc := range []struct {
		name      string
		age       time.Duration
		wantStale bool
	}{
		{"fresh", time.Hour, false},
		{"just inside", licensedRecencyWindow - time.Hour, false},
		{"just outside", licensedRecencyWindow + time.Hour, true},
		{"ancient", 10 * licensedRecencyWindow, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("BIBLETEXT_CACHE_PATH", filepath.Join(dir, "bibletext-cache.json"))
			path := filepath.Join(dir, "licensed.json")
			if err := saveBibleToCache(path, fullValidBible(), func() time.Time {
				return time.Now().UTC().Add(-tc.age)
			}); err != nil {
				t.Fatal(err)
			}
			if got := licensedCacheStale(path); got != tc.wantStale {
				t.Errorf("licensedCacheStale(age=%v) = %v, want %v", tc.age, got, tc.wantStale)
			}
		})
	}
}
