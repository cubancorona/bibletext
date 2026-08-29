package bibletext

// M2, the credential and licence machine, enumerated. See
// docs/VERSION_STATES.md.
//
// WHY CREDENTIALS GET THEIR OWN ENUMERATION. Every other machine in the model
// describes a fact about the world — which files exist, which version is
// active, what has been downloaded. This one has a state that is a fact about
// our KNOWLEDGE of the world: a credential store can fail, and "we cannot
// tell whether there is a key" is not "there is no key". The secretStore
// interface says so in its own contract ("ok=false means the credential store
// itself failed — CALLERS MUST NOT treat that as 'no key'"), and the key-read
// path is written to honour it. The question this enumeration exists to ask
// is whether every OTHER consumer honours it too — especially the ones that
// act irreversibly.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"fyne.io/fyne/v2/test"
)

// --- a credential store that can FAIL, which no existing fake can do --------

// flakySecretStore returns ok=false — the transient-failure answer a Keychain
// gives before first unlock, or on any error other than item-not-found.
type flakySecretStore struct{ reads *int }

func (s flakySecretStore) Read(string) (string, bool, bool) {
	if s.reads != nil {
		*s.reads++
	}
	return "", false, false // value, found, ok — the store itself failed
}
func (s flakySecretStore) Write(string, string) bool { return false }

// emptySecretStore is the definitive negative: the store works and there is
// no key. ok=true, found=false.
type emptySecretStore struct{}

func (emptySecretStore) Read(string) (string, bool, bool) { return "", false, true }
func (emptySecretStore) Write(string, string) bool        { return true }

// holdingSecretStore has the reader's key.
type holdingSecretStore struct{ key string }

func (s holdingSecretStore) Read(string) (string, bool, bool) { return s.key, true, true }
func (s holdingSecretStore) Write(string, string) bool        { return true }

// --- the axis ---------------------------------------------------------------

// credState is what the app can know about the reader's credential.
type credState int

const (
	credAbsent           credState = iota // the store works and holds nothing
	credHeld                              // the store works and holds the reader's key
	credUnreadable                        // THE STORE FAILED — we cannot tell
	credLegacyOnly                        // nothing in the secure store, a pre-1.1.6 key in prefs
	credUnreadableLegacy                  // the store failed AND a legacy key exists
)

func (c credState) String() string {
	return [...]string{"absent", "held", "unreadable", "legacy-only", "unreadable+legacy"}[c]
}

// credEvent is what the app does with that knowledge.
type credEvent int

const (
	evReadKey      credEvent = iota // keyStore.bibleAPIKey — the effective read
	evPurgeUnavail                  // purgeUnavailableLicensedCaches — IRREVERSIBLE
)

func (e credEvent) String() string {
	return [...]string{"read-key", "purge-unavailable"}[e]
}

type credObs struct {
	state credState
	event credEvent

	key           string // what the effective read returned
	definitive    bool   // could the app actually TELL? (store ok, or a key in hand)
	cacheSurvived bool   // after an irreversible event
}

type pinnedCredDefect struct {
	name   string
	what   string
	covers func(o credObs) bool
}

// knownCredIncoherent — every incoherent state of the credential machine
// reachable TODAY, by the name docs/VERSION_STATES.md gives it. Set-equality asserted,
// so a fix that leaves a pin behind fails the suite.
// D1 was struck when purgeUnavailableLicensedCaches was made to require a
// DEFINITIVE negative (keyStore.bibleKeyKnownAbsent) before deleting. No cell
// reaches an incoherent state today.
var knownCredIncoherent = []pinnedCredDefect{}

// The invariants of the credential machine.
//
//	C-A  A non-definitive answer never triggers an irreversible act.
//	C-B  A store failure never reads as an absent key when a key is recoverable.
func checkCredInvariants(o credObs) []string {
	var bad []string
	switch o.event {
	case evReadKey:
		// The legacy fallback exists precisely so a store failure cannot
		// strand a key the reader entered.
		if o.state == credUnreadableLegacy && o.key == "" {
			bad = append(bad, "C-B: a recoverable legacy key was lost to a store failure")
		}
	case evPurgeUnavail:
		if !o.definitive && !o.cacheSurvived {
			bad = append(bad, "C-A: deleted the reader's only copy on an answer the app could not verify")
		}
	}
	return bad
}

// --- the enumeration --------------------------------------------------------

func TestVersionCredentialStateSpace(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	var unexplained []string
	seen := map[string]bool{}
	cells := 0

	for state := credAbsent; state <= credUnreadableLegacy; state++ {
		for event := evReadKey; event <= evPurgeUnavail; event++ {
			name := fmt.Sprintf("%s/%s", state, event)
			t.Run(name, func(t *testing.T) {
				dir := t.TempDir()
				t.Setenv("BIBLETEXT_CACHE_PATH", filepath.Join(dir, "bibletext-cache.json"))
				// The env trio must be empty or it short-circuits the store
				// entirely (apiKey reads BIBLE_API_KEY first).
				t.Setenv("BIBLE_API_KEY", "")
				t.Setenv("BIBLETEXT_LICENSE_NKJV", "")
				t.Setenv("BIBLETEXT_PROVIDER_ID_NKJV", "")

				prefs := newFakePrefs()
				ks := &keyStore{prefs: prefs}
				switch state {
				case credAbsent:
					ks.secrets = emptySecretStore{}
				case credHeld:
					ks.secrets = holdingSecretStore{key: "reader-key"}
				case credUnreadable:
					ks.secrets = flakySecretStore{}
				case credLegacyOnly:
					ks.secrets = emptySecretStore{}
					prefs.SetString(prefKeyPrefix+bibleKeyID, "legacy-key")
				case credUnreadableLegacy:
					ks.secrets = flakySecretStore{}
					prefs.SetString(prefKeyPrefix+bibleKeyID, "legacy-key")
				}
				prev := sharedKeys
				sharedKeys = func() *keyStore { return ks }
				t.Cleanup(func() { sharedKeys = prev })

				obs := credObs{state: state, event: event}
				obs.key = ks.bibleAPIKey()
				// DEFINITIVE means the app could actually tell: either it holds
				// a key, or the store answered without failing.
				_, _, ok := ks.secrets.Read(bibleKeyID)
				obs.definitive = ok || obs.key != ""

				switch event {
				case evReadKey:
					// the read itself is the observation
				case evPurgeUnavail:
					// A licensed version with a cache on disk, driven through
					// the REAL startup sweep.
					nk, found := versionByID("nkjv")
					if !found {
						t.Skip("nkjv not registered")
					}
					path := cachePathForVersion(nk.ID)
					if err := saveBibleToCache(path, fullValidBible(), currentUTCTime); err != nil {
						t.Fatal(err)
					}
					if _, err := loadBibleFromCache(path); err != nil {
						t.Fatalf("control: the cache must be readable before the purge, else the cell proves nothing: %v", err)
					}
					purgeUnavailableLicensedCaches()
					_, err := loadBibleFromCache(path)
					obs.cacheSurvived = err == nil
				}
				cells++

				for _, bad := range checkCredInvariants(obs) {
					explained := false
					for _, d := range knownCredIncoherent {
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
			"in knownCredIncoherent. If it is not, the invariant is wrong.", u)
	}
	for _, d := range knownCredIncoherent {
		if !seen[d.name] {
			t.Errorf("%s is pinned but no cell reaches it any more (%s).\n"+
				"Strike it from knownCredIncoherent AND from docs/VERSION_STATES.md.", d.name, d.what)
		}
	}
	t.Logf("credential space: %d cells, %d pinned defects reached", cells, len(seen))
}

// The contract the secretStore interface states in its own words, asserted
// against the consumer that acts irreversibly. Named separately because it is
// the single property D1 turns on.
func TestTransientStoreFailureIsNotADeauthorization(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	dir := t.TempDir()
	t.Setenv("BIBLETEXT_CACHE_PATH", filepath.Join(dir, "bibletext-cache.json"))
	t.Setenv("BIBLE_API_KEY", "")
	t.Setenv("BIBLETEXT_LICENSE_NKJV", "")
	t.Setenv("BIBLETEXT_PROVIDER_ID_NKJV", "")

	reads := 0
	ks := &keyStore{prefs: newFakePrefs(), secrets: flakySecretStore{reads: &reads}}
	prev := sharedKeys
	sharedKeys = func() *keyStore { return ks }
	defer func() { sharedKeys = prev }()

	nk, found := versionByID("nkjv")
	if !found {
		t.Skip("nkjv not registered")
	}
	path := cachePathForVersion(nk.ID)
	if err := saveBibleToCache(path, fullValidBible(), currentUTCTime); err != nil {
		t.Fatal(err)
	}
	// The control: the cache is genuinely there and readable.
	if _, err := loadBibleFromCache(path); err != nil {
		t.Fatalf("control: cache must be readable first: %v", err)
	}

	purgeUnavailableLicensedCaches()

	if reads == 0 {
		t.Fatal("control: the purge never consulted the credential store — the cell proves nothing")
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("D1: the reader's only local copy of a licensed translation was deleted " +
			"because the credential store failed to answer. 'Cannot tell' is not " +
			"'not licensed' — the secretStore contract says so, and every other " +
			"consumer of that read honours it.")
	}
}

// A licensed superseded epoch can never be served — the licensed branch of
// loadVersionFromCacheOnly returns before the superseded walk — and the §11
// recency machinery only ever age-checks the CURRENT epoch. So such a file is
// licensed text on the reader's device with an unbounded lifetime that
// nothing will ever read or check again. The startup sweep now removes them.
// See D2 in docs/VERSION_STATES.md.
func TestSupersededLicensedEpochsAreNotRetained(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	dir := t.TempDir()
	t.Setenv("BIBLETEXT_CACHE_PATH", filepath.Join(dir, "bibletext-cache.json"))
	setNKJVLicence(t) // configured, so the §10 purge is NOT what removes these
	withFakeSharedKeys(t)
	withNKJVEpoch(t, 2)

	nk, found := versionByID("nkjv")
	if !found {
		t.Skip("nkjv not registered")
	}
	current := cachePathForVersion(nk.ID)
	superseded := supersededCachePaths(nk)
	if len(superseded) == 0 {
		t.Fatal("control: the fixture must produce a superseded path")
	}
	for _, p := range append([]string{current}, superseded...) {
		if err := saveBibleToCache(p, fullValidBible(), currentUTCTime); err != nil {
			t.Fatal(err)
		}
	}

	// THE CONTROL that makes the deletion safe: a licensed superseded epoch is
	// not serveable in the first place, so removing it costs the reader
	// nothing. Prove it rather than assert it — delete the current epoch so
	// the fallback would be the only thing left to try.
	if err := os.Remove(current); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadVersionFromCacheOnly(nk); err == nil {
		t.Fatal("control: a licensed version must NOT serve a superseded epoch — " +
			"if it can, deleting these files is not safe and this test is wrong")
	}
	if err := saveBibleToCache(current, fullValidBible(), currentUTCTime); err != nil {
		t.Fatal(err)
	}

	purgeSupersededLicensedCaches()

	for _, p := range superseded {
		if _, err := os.Stat(p); err == nil {
			t.Errorf("D2: a superseded licensed epoch survived the startup sweep (%s). "+
				"It can never be served and is never age-checked, so nothing else "+
				"will ever remove it.", filepath.Base(p))
		}
	}
	// And the current epoch — the one the reader actually reads — is untouched.
	if _, err := loadBibleFromCache(current); err != nil {
		t.Errorf("the CURRENT epoch must survive: %v", err)
	}
}

// The public-domain lane is untouched by the licensed sweep: its superseded
// epochs are the epoch-migration fallback an offline upgrader depends on.
func TestSupersededPublicDomainEpochsAreKept(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	dir := t.TempDir()
	t.Setenv("BIBLETEXT_CACHE_PATH", filepath.Join(dir, "bibletext-cache.json"))
	web, _ := versionByID("web")
	paths := supersededCachePaths(web)
	if len(paths) == 0 {
		t.Fatal("control: web must have superseded paths")
	}
	if err := saveBibleToCache(paths[0], fullValidBible(), currentUTCTime); err != nil {
		t.Fatal(err)
	}
	purgeSupersededLicensedCaches()
	if _, err := loadBibleFromCache(paths[0]); err != nil {
		t.Error("a public-domain superseded epoch was removed — that file is the " +
			"offline upgrader's whole canon, and the fallback that exists to serve it")
	}
}
