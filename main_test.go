package bibletext

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"fyne.io/fyne/v2"
)

// NO TEST IN THIS PACKAGE OPENS ANYTHING FOR REAL.
//
// The opener is substituted for the WHOLE test binary here, not per test. A
// per-test rule was tried and is not enough: it only binds tests that call the
// opener directly, and the one that actually opened browsers reached it three
// calls deep. TestNoteLinkIsDeclinedWhenNotesAreOff hands a note-bearing link
// to HandleShareLink with notes off and no window, and offerNoteLinkChoice's
// no-window fallback is — correctly, for a reader on a cold start — to hand the
// link to the browser. Every run of the suite opened a real tab, and on desktop
// macOS that is -[NSWorkspace openURL:], which Fyne's test app cannot stub.
//
// Nothing in CI shows it. It is visible only as tabs piling up on the machine
// of whoever ran the tests, which is how it survived.
//
// A test that wants to assert routing substitutes its own opener over this one
// and restores it; that still works, and openedInTests records what would have
// been opened for any test that wants to check.
var (
	openedMu sync.Mutex
	opened   []string
)

// openedInTests returns, and clears, the URLs the suite would have opened.
func openedInTests() []string {
	openedMu.Lock()
	defer openedMu.Unlock()
	out := opened
	opened = nil
	return out
}

// NOR DOES ANY TEST READ OR WRITE THE MACHINE'S TRANSLATION CACHE.
//
// Every translation cache path the app uses hangs off defaultCachePath, which
// is BIBLETEXT_CACHE_PATH when set and the user's cache directory otherwise.
// So the whole binary gets a directory of its own, made here, and a test that
// needs a particular disk builds it in a directory of its own — t.Setenv the
// variable to a t.TempDir, then mustCache — as the version harnesses do, so no
// test leaves a cache behind for the next.
//
// Before this, sixteen tests that switch translation read whatever the machine
// running them had downloaded: applyLoadedVersion asks versionCacheIsCurrent,
// which decodes the whole 7 MB file. On a machine that had opened the BSB and
// the Catholic WEB, the arrivals journeys decoded one 849 times — ten minutes
// under the race detector, most of the race run — and walked a different disk
// from CI's, which has none. The paths that delete and write the cache
// (purgeSupersededCaches, saveBibleToCache, the dev tab's Clear) hang off the
// same function, and only convention kept a test from reaching them.
//
// A test that reads real downloaded text on purpose, and says so, reaches the
// machine's directory through realCachePath: read-only, and never by default.
var realCacheDir string

// realCachePath is the machine's copy of the file the app keeps at p — for the
// opt-in tests that check real downloaded text. BIBLETEXT_CACHE_PATH set when
// the suite starts names the machine's cache for them (a simulator's, say).
func realCachePath(p string) string { return filepath.Join(realCacheDir, filepath.Base(p)) }

// NOR DOES ANY TEST SEE THE MACHINE'S CREDENTIALS.
//
// The app reads these from the environment, ahead of anything the reader has
// saved, and a developer's shell can hold them: ai_live_test.go says to
// `set -a; source ./.env.local` before a live run.
//
//	variable                    what the app takes it for               read by
//	BIBLE_API_KEY               the API.Bible key                       apiKey, versions.go
//	BIBLETEXT_LICENSE_<ID>      the operator's licence for <ID>         licensed, versions.go
//	BIBLETEXT_PROVIDER_ID_<ID>  <ID>'s id at the provider               providerVersionID, versions.go
//	BIBLETEXT_ENABLE_TESTING    makes unlicensed versions selectable    testingVersionsEnabled, versions.go
//	ANTHROPIC_API_KEY           each AI provider's key, as envVarFor    providerAPIKey, ai_providers.go
//	GEMINI_API_KEY              names them
//	OPENAI_API_KEY
//	XAI_API_KEY
//	GITHUB_TOKEN                the audio host's token (tests only)     audio_live_test.go
//
// A test run from that shell tests a different app. With the NKJV's three set
// the NKJV is licensed and available, so a test that opens a link to it or
// switches to it starts the real loader, which fetches the whole canon from
// API.Bible with the machine's key, about two hundred requests of a monthly
// quota of 5,000, and the tests that expect it locked fail. The LSB's pair
// unlocks the unconfigured licensed source the version tests build, and the
// placeholder test then fetches the LSB instead; an AI key hands the Find pane
// the key a test built it without. Exported with dummy values they failed
// thirteen tests of a plain run, six of them live probes that took a key in
// the environment as their cue to call the real service. CI sets none of
// them, so nothing there showed it.
//
// BIBLETEXT_ENABLE_TESTING is no credential, and .env.local does not set it,
// but it changes which translations the app offers as the licences do: it
// makes every unlicensed one selectable, for internal QA. Exported, it failed
// five tests that expect them locked, so it is withheld with the rest.
//
// So TestMain takes every one of them out of the environment before any test
// runs, and keeps what it found. They are matched by name, the two
// per-translation families by prefix and the AI keys through envVarFor, so a
// translation or provider added later is covered. A test that wants a
// credential sets its own with t.Setenv, as before, and it is gone when the
// test ends. The opt-in live tests, which exist to call the real services,
// read what the suite found through liveEnv, or put it back for their own run
// with useLiveEnv when they reach it through the app's lookup. A key alone no
// longer runs them: the probes that ran whenever BIBLE_API_KEY or an AI key
// was set now also want BIBLETEXT_LIVE=1, and the rest already had a switch
// of their own.
var withheldCredentials map[string]string

// isCredential reports whether name is one of the variables in the table.
func isCredential(name string) bool {
	switch {
	case name == "BIBLE_API_KEY", name == "GITHUB_TOKEN", name == "BIBLETEXT_ENABLE_TESTING",
		strings.HasPrefix(name, "BIBLETEXT_LICENSE_"),
		strings.HasPrefix(name, "BIBLETEXT_PROVIDER_ID_"):
		return true
	}
	for _, p := range aiProviders() {
		if name == envVarFor(p.ID) {
			return true
		}
	}
	return false
}

// withholdCredentials takes every credential out of the environment and
// returns what it took, by name.
func withholdCredentials() map[string]string {
	took := map[string]string{}
	for _, kv := range os.Environ() {
		name, value, _ := strings.Cut(kv, "=")
		if isCredential(name) {
			took[name] = value
			os.Unsetenv(name)
		}
	}
	return took
}

// liveEnv is what the credential name held when the suite started, for the
// opt-in live tests: every other test finds it unset.
//
// It also looks the name up in the environment and throws the answer away,
// since the credential is withheld and there is nothing there to find. The
// lookup is for go test's cache. A passing run is replayed from the cache until
// a variable the run read through os.Getenv or os.LookupEnv changes in the
// shell, and a read of the map records nothing. Without the lookup, a run with
// no key, which skips, was cached, and the same command with the key and the
// switch exported replayed the skip instead of making the call.
func liveEnv(name string) string {
	os.LookupEnv(name)
	return withheldCredentials[name]
}

// useLiveEnv puts the named credentials back, as the suite found them, for the
// rest of t — for a live test that reaches one through the app's own lookup
// (providerAPIKey) rather than by name.
func useLiveEnv(t *testing.T, names ...string) {
	t.Helper()
	for _, name := range names {
		if v, ok := withheldCredentials[name]; ok {
			t.Setenv(name, v)
		}
	}
}

// NOR DOES ANY TEST WRITE THE IMAGES THE APP HANDS TO THE SHARE SHEET.
//
// The share card and the lock-screen artwork are written under fixed names in
// the system temp directory — bibletext-verse-<variant>.png per Regenerate,
// bibletext-artwork-<title>.png per chapter — and the share sheet and Now
// Playing are handed that path. Every test that rendered one wrote those same
// files — the card tests directly, the audio controller's through the artwork
// startChapter draws — so a run left twelve cards and an artwork behind, and
// the two card tests deleted the default card after. On a desktop, where the
// app and the test binary share the user's temp directory, a run could replace
// or remove the image a running copy of the app was about to share.
// imageRenderDir now sends every render in the binary to a directory made here
// and removed after the run; the names inside it are the app's own.
//
// realTempDir is where the app itself renders, recorded before the redirect,
// so the guard below can say where a test's render must not land.
var realTempDir string

func TestMain(m *testing.M) {
	withheldCredentials = withholdCredentials()
	externalOpener = func(u *url.URL) error {
		openedMu.Lock()
		opened = append(opened, u.String())
		openedMu.Unlock()
		return nil
	}
	realCacheDir = filepath.Dir(defaultCachePath())
	suiteCache, err := os.MkdirTemp("", "bibletext-test-cache-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "no directory for the suite's translation cache:", err)
		os.Exit(2)
	}
	os.Setenv("BIBLETEXT_CACHE_PATH", filepath.Join(suiteCache, cacheFileName))
	realTempDir = imageRenderDir()
	suiteRenders, err := os.MkdirTemp("", "bibletext-test-renders-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "no directory for the suite's rendered images:", err)
		os.RemoveAll(suiteCache)
		os.Exit(2)
	}
	imageRenderDir = func() string { return suiteRenders }
	code := m.Run()
	os.RemoveAll(suiteCache)
	os.RemoveAll(suiteRenders)
	if left := openedInTests(); len(left) > 0 && os.Getenv("BT_SHOW_OPENS") != "" {
		fmt.Fprintf(os.Stderr, "tests would have opened %d URL(s); first: %s\n", len(left), left[0])
	}
	os.Exit(code)
}

// The suite's translation cache is its own: every path the app reads, writes
// or deletes a translation through resolves inside the directory TestMain
// made, and none in the machine's. The control clears the redirect and shows
// the same check would fire — it resolves paths and reads nothing.
func TestTheSuiteNeverTouchesTheMachinesTranslationCache(t *testing.T) {
	suite := filepath.Dir(defaultCachePath())
	if suite == realCacheDir {
		t.Fatalf("the suite's translation cache is the machine's: %s", suite)
	}
	check := func() []string {
		var stray []string
		paths := []string{defaultCachePath(), crossRefCachePath()}
		for _, v := range bibleVersions() {
			paths = append(paths, cachePathForVersion(v.ID))
			paths = append(paths, supersededCachePaths(v)...)
		}
		for _, p := range paths {
			if filepath.Dir(p) != suite {
				stray = append(stray, p)
			}
		}
		return stray
	}
	if stray := check(); len(stray) > 0 {
		t.Fatalf("%d cache paths lie outside the suite's directory %s, first %s", len(stray), suite, stray[0])
	}
	t.Setenv("BIBLETEXT_CACHE_PATH", "")
	if len(check()) == 0 {
		t.Fatal("control: with the redirect cleared every path still lies in the suite's directory, so the check above proves nothing")
	}
}

// No test sees the machine's credentials: TestMain withheld them, none is in
// the environment, and withholding takes every kind in the table out and keeps
// its value. The control sets one of each kind and shows the check sees them
// all.
func TestNoTestSeesTheMachinesCredentials(t *testing.T) {
	if withheldCredentials == nil {
		t.Fatal("TestMain withheld nothing, so every test sees the credentials of the machine that runs it")
	}
	visible := func() []string {
		var names []string
		for _, kv := range os.Environ() {
			if name, _, _ := strings.Cut(kv, "="); isCredential(name) {
				names = append(names, name)
			}
		}
		sort.Strings(names)
		return names
	}
	if names := visible(); len(names) > 0 {
		t.Fatalf("tests can see %v, which TestMain should have withheld", names)
	}

	kinds := []string{"BIBLE_API_KEY", "GITHUB_TOKEN", "BIBLETEXT_ENABLE_TESTING",
		"BIBLETEXT_LICENSE_NKJV", "BIBLETEXT_PROVIDER_ID_NKJV",
		"BIBLETEXT_LICENSE_LSB", "BIBLETEXT_PROVIDER_ID_LSB"}
	for _, p := range aiProviders() {
		kinds = append(kinds, envVarFor(p.ID))
	}
	for _, name := range kinds {
		t.Setenv(name, "dummy-"+name)
	}
	if got := visible(); len(got) != len(kinds) {
		t.Fatalf("control: with %d credentials set the check sees %v, so its pass above proves nothing", len(kinds), got)
	}
	took := withholdCredentials()
	if names := visible(); len(names) > 0 {
		t.Errorf("withholding left %v in the environment", names)
	}
	for _, name := range kinds {
		if took[name] != "dummy-"+name {
			t.Errorf("withholding kept %q for %s, want what was set", took[name], name)
		}
	}
}

// The live tests still get what the suite found. A credential withheld at the
// start reads back through liveEnv, and useLiveEnv puts it back for one test,
// where the app's own lookup finds it, and it is gone again when that test
// ends.
func TestTheLiveTestsStillGetTheCredentialsTheSuiteFound(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "dummy-not-a-key")
	found := withholdCredentials()
	saved := withheldCredentials
	withheldCredentials = found
	t.Cleanup(func() { withheldCredentials = saved })

	if got := liveEnv("OPENAI_API_KEY"); got != "dummy-not-a-key" {
		t.Fatalf("liveEnv = %q, want what the suite found", got)
	}
	store := newKeyStoreWith(newFakePrefs())
	if got := providerAPIKey(store, providerOpenAI); got != "" {
		t.Fatalf("an ordinary test's lookup found %q", got)
	}
	t.Run("live", func(t *testing.T) {
		useLiveEnv(t, "OPENAI_API_KEY")
		if got := providerAPIKey(store, providerOpenAI); got != "dummy-not-a-key" {
			t.Errorf("the live test's lookup found %q, want what the suite found", got)
		}
	})
	if got := providerAPIKey(store, providerOpenAI); got != "" {
		t.Errorf("the key outlived the live test that put it back: %q", got)
	}
}

// A key the suite found does not run a live test by itself: without
// BIBLETEXT_LIVE=1 the test skips before it calls anything, so a shell with
// keys in it runs a plain suite that spends nothing. The control sets the
// switch and shows the same test goes on to its call.
func TestALiveTestWantsItsSwitchAsWellAsTheKey(t *testing.T) {
	saved := withheldCredentials
	withheldCredentials = map[string]string{
		"BIBLE_API_KEY": "dummy-not-a-key", "OPENAI_API_KEY": "dummy-not-a-key",
	}
	t.Cleanup(func() { withheldCredentials = saved })
	openAI, _ := providerByID(providerOpenAI)
	store := newKeyStoreWith(newFakePrefs())

	for _, live := range []struct {
		name string
		gate func(t *testing.T)
	}{
		{"API.Bible", func(t *testing.T) { liveAPIBible(t, "BIBLETEXT_LIVE") }},
		{"AI provider", func(t *testing.T) { liveAIKey(t, store, openAI) }},
	} {
		reached := func(switched string) bool {
			called := false
			t.Setenv("BIBLETEXT_LIVE", switched)
			t.Run(live.name+" BIBLETEXT_LIVE="+switched, func(t *testing.T) { live.gate(t); called = true })
			return called
		}
		if reached("") {
			t.Errorf("%s: the key without BIBLETEXT_LIVE=1 ran the live test", live.name)
		}
		if !reached("1") {
			t.Errorf("control: %s: with the switch set the live test still skipped, so the skip above proves nothing", live.name)
		}
	}
}

// go test knows which credential a live test asked for, so exporting the key
// after a run that skipped for want of it runs the test instead of replaying
// the skip. go test learns what a run read from a log the test binary keeps
// for it, so this runs the binary again, for this test alone, with the log
// turned on, and reads it. The controls show the log has the child's own read
// of its switch, and not a read the child made of the map alone, which is all
// liveEnv once made.
func TestGoTestKnowsWhichCredentialALiveTestAskedFor(t *testing.T) {
	if os.Getenv("BIBLETEXT_TESTLOG_CHILD") == "1" {
		liveEnv("BIBLE_API_KEY")
		_ = withheldCredentials["GITHUB_TOKEN"]
		return
	}
	logFile := filepath.Join(t.TempDir(), "testlog")
	child := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$", "-test.testlogfile="+logFile)
	child.Env = append(os.Environ(), "BIBLETEXT_TESTLOG_CHILD=1")
	if out, err := child.CombinedOutput(); err != nil {
		t.Fatalf("the test binary did not run this test again: %v\n%s", err, out)
	}
	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}
	read := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		if name, ok := strings.CutPrefix(line, "getenv "); ok {
			read[name] = true
		}
	}
	if !read["BIBLETEXT_TESTLOG_CHILD"] {
		t.Fatalf("control: the log does not have the child's read of its own switch, so it records no reads:\n%s", data)
	}
	if read["GITHUB_TOKEN"] {
		t.Fatal("control: the log has a read the child made of the map alone, so the check below proves nothing")
	}
	if !read["BIBLE_API_KEY"] {
		t.Error("the log does not have the child's read of BIBLE_API_KEY through liveEnv, so go test replays a live test's skip after the key is exported")
	}
}

// A share card and a lock-screen artwork rendered by a test land in the
// directory TestMain made, and nothing is written under their names where the
// app renders its own. The variant and the title are ones no reader reaches,
// so a file of that name in the system temp directory as new as the render
// can only be the render's, and the test removes it. The control applies the
// same freshness check to the suite's copies, which the render did just
// write, and shows it fires.
func TestTheSuiteNeverWritesTheAppsSharedImages(t *testing.T) {
	if realTempDir != os.TempDir() {
		t.Fatalf("the app renders into %s, not the system temp directory %s", realTempDir, os.TempDir())
	}
	// Both directories are compared clean, because the renderers build their
	// paths with filepath.Join and the directories arrive spelled as TMPDIR
	// spells them: one ending in two slashes, or holding a ".", is the same
	// directory written differently, and the check below would call a render
	// that landed in the right place stray.
	app, suite := filepath.Clean(realTempDir), filepath.Clean(imageRenderDir())
	if suite == app {
		t.Fatalf("the suite renders into the system temp directory %s", suite)
	}
	// Truncated so a file system that keeps whole seconds still counts a
	// write made during the test as made at or after its start.
	start := time.Now().Truncate(time.Second)
	fresh := func(p string) bool {
		fi, err := os.Stat(p)
		return err == nil && !fi.ModTime().Before(start)
	}

	const variant = 1 << 20 // far past any Regenerate a reader presses
	card, err := renderVerseImage(nil, "“Jesus wept.”", "John 11:35", "World English Bible", variant)
	if err != nil {
		t.Fatal(err)
	}
	reg, bold := serifFontBytes(nil, fyne.TextStyle{}), serifFontBytes(nil, fyne.TextStyle{Bold: true})
	art, err := renderChapterArtwork("Render Guard 1", "World English Bible", reg, bold)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{card, art} {
		if filepath.Dir(p) != suite {
			t.Errorf("%s was written outside the suite's directory %s", p, suite)
		}
		if !fresh(p) {
			t.Errorf("control: %s, which the render just wrote, does not count as new, so the check below proves nothing", p)
		}
		if twin := filepath.Join(app, filepath.Base(p)); fresh(twin) {
			os.Remove(twin)
			t.Errorf("%s was written into the system temp directory, where the app keeps its own", twin)
		}
	}
}

// The no-window fallback really does hand the link to the browser, and the
// stub above is really what stops it reaching the machine.
//
// This is the mechanism that opened a real tab on every run of the suite:
// TestNoteLinkIsDeclinedWhenNotesAreOff hands a note-bearing link to
// HandleShareLink with notes off and no window, and offerNoteLinkChoice's
// no-window arm hands it onward — correct for a reader on a cold start, and
// invisible in CI, where nobody sees a window open.
func TestTheNoWindowOfferHandsTheLinkOnwardButNotToTheMachine(t *testing.T) {
	openedInTests() // drain anything an earlier test recorded

	st := psalm23State()
	st.window = nil
	raw := "https://bibletext.co.uk/bsb/psalms/23/#v1-4&n=" + EncodeNote("a note")
	target, ok := ParseShareLink(raw)
	if !ok {
		t.Fatal("fixture URL does not parse as a share link")
	}
	offerNoteLinkChoice(st, raw, target)

	got := openedInTests()
	if len(got) != 1 || got[0] != raw {
		t.Fatalf("the no-window fallback routed %v, want exactly the raw link — if this "+
			"is empty the fallback is gone; if it is a real open, the suite is opening "+
			"browser tabs again", got)
	}
}
