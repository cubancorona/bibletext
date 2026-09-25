package bibletext

// M5 x M6 x M7 — launch, reading position and canon shape, enumerated
// TOGETHER. See docs/VERSION_STATES.md.
//
// WHY THESE THREE ARE ONE ENUMERATION. Every other machine in the model is
// enumerable alone. These three are not, because the failure that motivated
// the document lives in their intersection and in none of them separately:
// the launch decides WHICH canon is loaded (M5), the saved position names a
// book that may only exist in ANOTHER canon (M6), and whether that book still
// exists depends on which canon answered (M7). Each machine is individually
// correct at every step; the reader loses their history in the composition.
//
// The property under test is DURABILITY, which is what makes this the most
// dangerous machine in the app. Every other defect in this document costs the
// reader a session. A defect here rewrites the only copy of something they
// cannot re-derive — where they were reading, and which translation they
// chose — and it does it on the next navigation, silently, so there is no
// moment at which they could have intervened.

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
)

// widerCanonBible is the 73-book shape (WEBC): everything in the base canon
// plus the deuterocanon. Built from the app's own catholicBooks list so it
// cannot drift from the real thing.
func widerCanonBible() *BibleData {
	bd := NewBibleData()
	bd.Books = append([]string(nil), catholicBooks...)
	bd.Verses = map[string]map[int][]Verse{}
	for _, book := range bd.Books {
		bd.Verses[book] = map[int][]Verse{
			1: {{BookName: book, Book: book, Chapter: 1, Verse: 1, Text: "wide " + book + " 1:1 sample."}},
		}
	}
	bd.PrepareSearchIndex()
	return bd
}

// --- the axes ---------------------------------------------------------------

// savedChoice is the translation named in the persisted reading state — the
// reader's own choice, and the only record of it that exists anywhere.
type savedChoice int

const (
	savedDefault    savedChoice = iota // the one the launch already loaded
	savedWiderCanon                    // 73 books: the saved book may not exist elsewhere
	savedLicensed                      // can stop being selectable between launches
)

func (c savedChoice) String() string {
	return [...]string{"saved-default", "saved-wider", "saved-licensed"}[c]
}

func (c savedChoice) id() string {
	return [...]string{defaultVersionID, "webc", "nkjv"}[c]
}

// choiceFate is what the app can do about that translation at launch.
type choiceFate int

const (
	fateLoads          choiceFate = iota // it loads normally
	fateLoadFails                        // offline, no usable cache
	fateSupersededOnly                   // current epoch gone, previous epoch on disk
	fateUnselectable                     // canSelect() is false this launch
)

func (f choiceFate) String() string {
	return [...]string{"loads", "load-fails", "superseded-only", "unselectable"}[f]
}

// --- what the launch produced, and whether it told the truth -----------------

type launchObs struct {
	choice savedChoice
	fate   choiceFate
	book   string

	aborted  bool   // restore returned an error: the Retry screen, history untouched
	onScreen string // state.CurrentVersion — what every surface will name
	wideData bool   // is the data actually in state.Bible the wider canon?
	persists string // what the NEXT save would write into the Version field
	keptHist int    // history entries that survived
	wantHist int    // entries that were valid in the reader's OWN canon
	told     bool   // does any surface say the chosen translation is not the one shown?
	previous bool   // is the text on screen a previous edition of the chosen translation?
	saidEd   bool   // does the picker footer say the edition on screen is a previous one?
	owed     bool   // does the refresh owe the translation on screen its upgrade?
}

type pinnedLaunchDefect struct {
	name   string
	what   string
	covers func(o launchObs) bool
}

// knownLaunchIncoherent — every incoherent state of the launch machine
// reachable TODAY, by the name docs/VERSION_STATES.md gives it. Set equality is
// asserted, so a fix that leaves a pin behind fails the suite just as loudly as
// a new defect.
// D9 was struck when a translation that is merely unselectable this launch
// started recording the reader's choice, and D10 when the picker footer
// learned to say the substitution out loud. No cell reaches an incoherent
// state today.
var knownLaunchIncoherent = []pinnedLaunchDefect{}

// The launch machine's invariants. These are the durability ones.
//
//	L-A  The reader's chosen translation survives a launch that could not open
//	     it. It is recorded in exactly one place, and a fallback must not be
//	     what rewrites it.
//	L-B  A fallback never prunes history that is valid in the reader's own
//	     canon. Validating a 73-book trail against 66 books is the erasure.
//	L-C  The version named on screen is the version whose text is on screen.
//	L-D  A reader who did not get the translation they chose is told so.
//	L-E  A previous edition on screen at launch is said (D3's launch site).
//	L-F  ...and the refresh owes it its upgrade, so it is not the text for
//	     the rest of the session (D17).
func checkLaunchInvariants(o launchObs) []string {
	var bad []string
	if o.aborted {
		return nil // the Retry screen: nothing was written, nothing can be wrong
	}
	if o.persists != o.choice.id() {
		bad = append(bad, fmt.Sprintf("L-A: the reader chose %s and the next save would write %q", o.choice.id(), o.persists))
	}
	if o.keptHist < o.wantHist {
		bad = append(bad, fmt.Sprintf("L-B: %d of %d history entries valid in the reader's canon were pruned", o.wantHist-o.keptHist, o.wantHist))
	}
	if (o.onScreen == "webc") != o.wideData {
		bad = append(bad, "L-C: the version named on screen is not the canon in hand")
	}
	if o.onScreen != o.choice.id() && !o.told {
		bad = append(bad, "L-D: the chosen translation was not opened and nothing says so")
	}
	if o.previous && !o.saidEd {
		bad = append(bad, "L-E: a previous edition is on screen at launch and the picker does not say so")
	}
	if o.previous && !o.owed {
		bad = append(bad, "L-F: a previous edition is on screen at launch and the refresh does not owe it an upgrade")
	}
	return bad
}

// --- enumeration D: the launch cross-product ---------------------------------

func TestVersionLaunchStateSpace(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	var unexplained []string
	seen := map[string]bool{}
	cells, previousCells := 0, 0

	for choice := savedDefault; choice <= savedLicensed; choice++ {
		for fate := fateLoads; fate <= fateUnselectable; fate++ {
			for _, book := range []string{"Genesis", "Tobit"} {
				// A reader can only have been reading Tobit under the canon
				// that contains it; the other pairings are not states the app
				// can have written.
				if book == "Tobit" && choice != savedWiderCanon {
					continue
				}
				name := fmt.Sprintf("%s/%s/%s", choice, fate, book)
				t.Run(name, func(t *testing.T) {
					obs := runLaunchCell(t, choice, fate, book)
					cells++
					if obs.previous {
						previousCells++
					}
					for _, bad := range checkLaunchInvariants(obs) {
						explained := false
						for _, d := range knownLaunchIncoherent {
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
	}

	if len(unexplained) > 0 {
		sort.Strings(unexplained)
		t.Errorf("%d launch cells; %d incoherent states with no entry in the register:\n  %v",
			cells, len(unexplained), unexplained)
	}
	// L-E and L-F ask about a previous edition on screen; a space with none
	// would pass them without asking anything.
	if previousCells == 0 {
		t.Error("control: no launch cell put a previous edition on screen, so L-E and L-F were never put to it")
	}
	// Set equality: a fix that leaves its pin behind fails here.
	for _, d := range knownLaunchIncoherent {
		if !seen[d.name] {
			t.Errorf("%s is pinned as reachable but no cell reached it — if it is fixed, strike it from knownLaunchIncoherent and from docs/VERSION_STATES.md: %s", d.name, d.what)
		}
	}
	t.Logf("%d launch cells enumerated, %d with a previous edition on screen; %d pinned incoherent states reached", cells, previousCells, len(seen))
}

// runLaunchCell drives ONE launch through the app's real restore, then through
// the exact tail loadStateData runs when the restore declines (app.go), then
// through the hand-off to the live state (adoptLaunch) — the three together are
// what the reader actually experiences. The erasure lived in the seam between
// the first two, and D18 in the seam the cells used to stop short of: they
// read the restore's own state, which the reader never sees.
func runLaunchCell(t *testing.T, choice savedChoice, fate choiceFate, book string) launchObs {
	t.Helper()
	base := fullValidBible() // the 66-book default canon, already loaded
	wide := widerCanonBible()

	// The reader's trail: one entry for where they are, one book both canons
	// share, and one that exists ONLY in the wider canon. All distinct, so a
	// de-duplicated entry can never be miscounted as a pruned one.
	// Chapter 1 throughout: the fixture canons carry one chapter per book, so
	// any other number would be pruned for a reason that has nothing to do
	// with the canon under test — the enumeration would invent its own defect.
	savedRecent := []ChapterVisit{
		{Book: book, Chapter: 1},
		{Book: "John", Chapter: 1},
		{Book: "Judith", Chapter: 1},
	}
	rs := readingState{
		Version: choice.id(),
		Book:    book,
		Chapter: 1,
		Recent:  savedRecent,
	}

	// How many of those entries are valid in the READER'S OWN canon — the
	// number that must survive, whatever the launch had to fall back to.
	readerCanon := base
	if choice == savedWiderCanon {
		readerCanon = wide
	}
	want := 0
	for _, v := range savedRecent {
		if chapterExists(readerCanon, v.Book, v.Chapter) {
			want++
		}
	}

	// The fate, applied to the real code paths.
	restoreVersionFate(t, choice, fate, wide)

	state := &AppState{
		Bible:          base,
		CurrentVersion: defaultVersionID,
		currentMode:    modeReal,
		loadedVersions: map[string]*BibleData{defaultVersionID: base},
		loadPhase:      loadReady,
	}

	obs := launchObs{choice: choice, fate: fate, book: book, wantHist: want}
	restored, err := restoreReadingState(state, rs, base)
	if err != nil {
		obs.aborted = true
		return obs
	}
	if !restored {
		// loadStateData's tail, verbatim (app.go): a saved book that is gone
		// falls back to the default start, and the REST of the history is
		// re-validated — against whichever canon answered.
		state.RecentChapters = restoreRecent(rs.Recent, base,
			defaultStartBook(base), clampChapter(base, defaultStartBook(base), 1))
		state.CurrentBook = defaultStartBook(base)
		state.CurrentChapter = 1
	}

	// THE STATE THE READER USES: StartBackgroundLoad hands the restore's state
	// to the live one through adoptLaunch, and every observation below is read
	// off what comes out of it.
	live := NewLoadingState()
	adoptLaunch(live, state)
	obs.onScreen = live.CurrentVersion
	obs.wideData = live.Bible.GetChaptersForBook("Tobit") > 0
	obs.keptHist = len(live.RecentChapters)
	// What the next navigation would write — the only record of the choice.
	obs.persists = snapshotReadingState(live, 0, 0, 0, 0, 0).Version
	// Every surface that could carry the news. There is exactly one.
	notice := fullPendingNotice(live)
	obs.told = notice != ""
	// A previous edition is on screen exactly when the fate left only the
	// superseded file and the restore served the chosen translation from it.
	// The default is left out: its restore has nothing to load, and its own
	// edition is the refresh machine's cells.
	obs.previous = fate == fateSupersededOnly && live.CurrentVersion == choice.id() && choice != savedDefault
	obs.saidEd = strings.Contains(notice, "previous edition")
	for _, v := range owedUpgrades(live) {
		obs.owed = obs.owed || v.ID == live.CurrentVersion
	}
	return obs
}

// restoreVersionFate makes the chosen translation meet the given fate, through
// the real switches the app reads: the credential environment for
// selectability, the indirected loader for the load result, and the on-disk
// cache for the superseded-epoch fallback.
func restoreVersionFate(t *testing.T, choice savedChoice, fate choiceFate, wide *BibleData) {
	t.Helper()
	cacheDir := t.TempDir()
	t.Setenv("BIBLETEXT_CACHE_PATH", cacheDir+"/bibletext-cache.json")

	// The licensed translation is selectable exactly while its licence
	// configuration reads back. fateUnselectable withdraws it.
	if choice == savedLicensed && fate != fateUnselectable {
		t.Setenv("BIBLE_API_KEY", "launch-cell-key")
		t.Setenv("BIBLETEXT_LICENSE_NKJV", "1")
		t.Setenv("BIBLETEXT_PROVIDER_ID_NKJV", "test-provider-id")
	} else {
		t.Setenv("BIBLE_API_KEY", "")
		t.Setenv("BIBLETEXT_LICENSE_NKJV", "")
		t.Setenv("BIBLETEXT_PROVIDER_ID_NKJV", "")
	}
	prev := sharedKeys
	ks := &keyStore{prefs: newFakePrefs(), secrets: emptySecretStore{}}
	sharedKeys = func() *keyStore { return ks }
	t.Cleanup(func() { sharedKeys = prev })

	data := wide
	if choice != savedWiderCanon {
		data = fullValidBible()
	}

	prevLoad := loadVersionForRestore
	switch fate {
	case fateLoads:
		loadVersionForRestore = func(v BibleVersion, base *BibleData) (*BibleData, dataMode, error) {
			return data, modeReal, nil
		}
	case fateLoadFails, fateUnselectable:
		loadVersionForRestore = func(v BibleVersion, base *BibleData) (*BibleData, dataMode, error) {
			return nil, modeReal, errors.New("offline")
		}
	case fateSupersededOnly:
		// The current epoch is missing and the PREVIOUS one is on disk — the
		// offline epoch-bump upgrade. loadVersionFromCacheOnly is not
		// indirected, so this is driven by writing the real file.
		loadVersionForRestore = func(v BibleVersion, base *BibleData) (*BibleData, dataMode, error) {
			return nil, modeReal, errors.New("offline")
		}
		if v, ok := versionByID(choice.id()); ok {
			paths := supersededCachePaths(v)
			if len(paths) > 0 {
				if err := saveBibleToCache(paths[0], data, currentUTCTime); err != nil {
					t.Fatalf("seed the superseded epoch: %v", err)
				}
			}
		}
	}
	t.Cleanup(func() { loadVersionForRestore = prevLoad })
}

// --- the two the enumeration found, each pinned on its own ------------------

// TestAnUnreadableLicenceDoesNotForgetTheReadersTranslation is D9, and it is
// the M2 x M6 coupling the map predicted: a credential store that FAILS
// answers exactly like one that is empty, so the licensed translation stops
// being selectable — and the launch used to treat that momentary blindness as
// the reader changing their mind, in the one place the decision is recorded.
func TestAnUnreadableLicenceDoesNotForgetTheReadersTranslation(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	t.Setenv("BIBLETEXT_CACHE_PATH", t.TempDir()+"/bibletext-cache.json")
	t.Setenv("BIBLE_API_KEY", "")
	t.Setenv("BIBLETEXT_LICENSE_NKJV", "")
	t.Setenv("BIBLETEXT_PROVIDER_ID_NKJV", "")

	// The store FAILS — it does not report an absent key. This is the iOS
	// before-first-unlock answer, and it is transient by definition.
	prev := sharedKeys
	sharedKeys = func() *keyStore {
		return &keyStore{prefs: newFakePrefs(), secrets: flakySecretStore{}}
	}
	t.Cleanup(func() { sharedKeys = prev })

	nk, ok := versionByID("nkjv")
	if !ok {
		t.Skip("nkjv not registered")
	}
	// CONTROL. The whole defect turns on this being false; if a future change
	// makes an unreadable store leave the version selectable, this test would
	// pass while proving nothing.
	if nk.canSelect() {
		t.Fatal("control: the licensed version must be unselectable when the store cannot be read, or this test proves nothing")
	}

	base := fullValidBible()
	state := &AppState{
		Bible:          base,
		CurrentVersion: defaultVersionID,
		currentMode:    modeReal,
		loadedVersions: map[string]*BibleData{defaultVersionID: base},
		loadPhase:      loadReady,
	}
	rs := readingState{Version: "nkjv", Book: "John", Chapter: 1}

	restored, err := restoreReadingState(state, rs, base)
	if err != nil || !restored {
		t.Fatalf("the app must still open: restored=%v err=%v", restored, err)
	}
	// The reader is on the fallback — that part is correct and unavoidable.
	if state.CurrentVersion != defaultVersionID {
		t.Fatalf("on screen = %q, want the default canon", state.CurrentVersion)
	}
	// What must NOT happen: the next navigation overwriting the only record of
	// the reader's translation with the fallback's id.
	if got := snapshotReadingState(state, 0, 0, 0, 0, 0).Version; got != "nkjv" {
		t.Fatalf("the next save would write %q — the reader's choice was erased by a condition that fixes itself", got)
	}
}

// TestAFallbackTranslationSaysSoOnThePicker is D10. A reader who asked for one
// translation and was given another must be able to find that out; the picker
// footer is the surface that answers "which translation am I on", and it is
// the same one D3 uses.
func TestAFallbackTranslationSaysSoOnThePicker(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	nk, ok := versionByID("nkjv")
	if !ok {
		t.Skip("nkjv not registered")
	}
	base := fullValidBible()
	state := &AppState{
		Bible:            base,
		CurrentVersion:   defaultVersionID,
		currentMode:      modeReal,
		loadedVersions:   map[string]*BibleData{defaultVersionID: base},
		loadPhase:        loadReady,
		preferredVersion: nk.ID,
	}

	notice := fullPendingNotice(state)
	if notice == "" {
		t.Fatal("the reader asked for the New King James Version, is being shown something else, and no surface says so")
	}
	if !strings.Contains(notice, nk.Name) {
		t.Fatalf("the notice must name the translation the reader chose; got %q", notice)
	}
	def, _ := versionByID(defaultVersionID)
	if !strings.Contains(notice, def.Name) {
		t.Fatalf("the notice must name what is actually on screen; got %q", notice)
	}

	// And it stops the moment the reader's own translation is what they are
	// reading — a notice that outlives its condition is D4 in another machine.
	state.preferredVersion = defaultVersionID
	if n := fullPendingNotice(state); n != "" {
		t.Fatalf("the notice outlived the substitution: %q", n)
	}
}

// TestAWiderCanonsTrailSurvivesReadingANarrowerOne is D16, and it is the
// original incident arriving by a door the launch enumeration above does not
// have. L-B proves a trail survives a launch that falls back; this proves it
// survives the reader simply CHANGING TRANSLATION, which is the ordinary thing
// the app is for.
//
// An evening in the WEBC leaves Tobit, Sirach and 1 Maccabees in the trail.
// Switching to the WEB persisted that trail under "web", and the next launch
// validated it against 66 books and deleted all three from the only copy. The
// reader's history, erased for reading a different translation, with nothing
// said and no way back.
func TestAWiderCanonsTrailSurvivesReadingANarrowerOne(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	wide := widerCanonBible()
	narrow := fullValidBible()

	// CONTROL: the narrow canon really lacks these, or nothing below is a test.
	for _, b := range []string{"Tobit", "Judith"} {
		if narrow.GetChaptersForBook(b) != 0 {
			t.Fatalf("control: the narrow canon must not contain %s", b)
		}
		if wide.GetChaptersForBook(b) == 0 {
			t.Fatalf("control: the wider canon must contain %s", b)
		}
	}

	trail := []ChapterVisit{
		{Book: "Tobit", Chapter: 1},
		{Book: "Judith", Chapter: 1},
		{Book: "John", Chapter: 1},
	}
	st := &AppState{
		Bible: wide, CurrentVersion: "webc", currentMode: modeReal,
		loadedVersions: map[string]*BibleData{"webc": wide},
		loadPhase:      loadReady,
		CurrentBook:    "Tobit", CurrentChapter: 1,
		RecentChapters: append([]ChapterVisit(nil), trail...),
	}

	// The reader switches to the narrower translation.
	web, _ := versionByID(defaultVersionID)
	applyLoadedVersion(st, web, narrow, modeReal, byReader)

	// The bar must not offer what this canon cannot open.
	for _, v := range recentJumpTargets(st, maxRecent) {
		if narrow.GetChaptersForBook(v.Book) == 0 {
			t.Fatalf("the history bar offers %s, which this translation does not contain", v.Book)
		}
	}
	// ...and a stale reference reaching the navigation anyway is refused.
	before := st.CurrentBook
	navigateToVisit(st, ChapterVisit{Book: "Tobit", Chapter: 1})
	if st.CurrentBook != before {
		t.Fatalf("navigated to %s, a book this translation does not contain", st.CurrentBook)
	}

	// THE DURABLE HALF. What the next launch keeps, out of what this session
	// would save.
	saved := snapshotReadingState(st, 0, 0, 0, 0, 0)
	kept := restoreRecent(saved.Recent, narrow, "Genesis", 1)
	have := map[string]bool{}
	for _, v := range kept {
		have[v.Book] = true
	}
	for _, b := range []string{"Tobit", "Judith"} {
		if !have[b] {
			t.Fatalf("%s was deleted from the reader's trail because they read a different translation", b)
		}
	}

	// And a book this app has never shipped is still dropped — the guard is
	// "dormant", not "keep everything forever".
	withGhost := append([]ChapterVisit{{Book: "Book of Eli", Chapter: 1}}, saved.Recent...)
	for _, v := range restoreRecent(withGhost, narrow, "Genesis", 1) {
		if v.Book == "Book of Eli" {
			t.Fatal("a book no canon in this build contains was carried forward")
		}
	}
}

// TestTheLaunchCarriesWhatTheRestoreRecords is D18's guard.
//
// The launch restores onto a state of its own on the load goroutine
// (loadStateData), and StartBackgroundLoad hands that to the live state the
// window closed over, through adoptLaunch. Every field the restore writes must
// reach the live state — above all the two records the restore makes when it
// cannot give the reader what they chose: the translation the reader chose,
// when the launch had to show another (D9, and the sentence D10 reads off
// it), and the mark that a restored translation is showing its previous
// edition (D3's launch site, and now the refresh's work list, D17). The copy
// used to leave both out, so on the state the reader was using the choice was
// gone and the next navigation saved the fallback over it, and the previous
// edition was shown with nothing said.
//
// Both halves are checked. The hand-off runs inside a goroutine a test cannot
// await, so it is read from the source: what the restore records is every
// field it writes on its state, directly or through a function it hands the
// state to, and a record reaches the screen if the launch either reads it off
// the restore's state or writes it on the live one, in every shape a fix could
// take — see launchReaches. And adoptLaunch, the function the launch calls,
// is run for real on what loadStateData returns: the live state holds both
// records, says what the restore's state says, and saves the reader's choice.
// The one change the source reading cannot see is one that rebuilds the
// records on the live state through more than two calls of new code, without
// reading them off the restore's; the behavioural half is the check on that.
func TestTheLaunchCarriesWhatTheRestoreRecords(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	// The launch this reads needs no network: the default translation's
	// current edition is on disk, no translation's source is online, and no
	// licence reads back, so the NKJV cannot be selected.
	t.Setenv("BIBLETEXT_CACHE_PATH", filepath.Join(t.TempDir(), cacheFileName))
	t.Setenv("BIBLE_API_KEY", "")
	t.Setenv("BIBLETEXT_LICENSE_NKJV", "")
	t.Setenv("BIBLETEXT_PROVIDER_ID_NKJV", "")
	prev := sharedKeys
	ks := &keyStore{prefs: newFakePrefs(), secrets: emptySecretStore{}}
	sharedKeys = func() *keyStore { return ks }
	t.Cleanup(func() { sharedKeys = prev })
	for _, id := range []string{defaultVersionID, "webc"} {
		withVersionSource(t, id, &arrivalSource{offline: true})
	}
	mustCache(t, cachePathForVersion(defaultVersionID), fullValidBible())
	webc, _ := versionByID("webc")
	mustCache(t, supersededCachePaths(webc)[0], widerCanonBible())

	// The restore records both, on the state loadStateData hands over, and the
	// live state holds them after the hand-off.
	for _, tc := range []struct {
		saved, onScreen string
		recorded        func(st *AppState) bool
	}{
		{"nkjv", defaultVersionID, func(st *AppState) bool { return st.preferredVersion == "nkjv" }},
		{"webc", "webc", func(st *AppState) bool { return st.staleVersions["webc"] }},
	} {
		writeReadingState(appPrefs(), readingState{Version: tc.saved, Book: "John", Chapter: 1})
		loaded, err := loadStateData()
		if err != nil {
			t.Fatalf("control: a launch with %s saved must open: %v", tc.saved, err)
		}
		if loaded.CurrentVersion != tc.onScreen || !tc.recorded(loaded) || fullPendingNotice(loaded) == "" {
			t.Fatalf("control: with %s saved the restore must show %s and record why, and say so; on %s, notice %q",
				tc.saved, tc.onScreen, loaded.CurrentVersion, fullPendingNotice(loaded))
		}
		live := NewLoadingState()
		adoptLaunch(live, loaded)
		if live.CurrentVersion != tc.onScreen || !tc.recorded(live) {
			t.Fatalf("with %s saved the live state does not hold what the restore recorded: on %s", tc.saved, live.CurrentVersion)
		}
		if n := fullPendingNotice(live); n != fullPendingNotice(loaded) {
			t.Fatalf("with %s saved the live state's footer reads %q, and the restore's %q", tc.saved, n, fullPendingNotice(loaded))
		}
		if saved := snapshotReadingState(live, 0, 0, 0, 0, 0).Version; saved != tc.saved {
			t.Fatalf("with %s saved the next save from the live state writes %q over the reader's choice", tc.saved, saved)
		}
	}

	src := parsePackageSource(t)
	restore := src.funcs["restoreReadingState"]
	if len(restore) != 1 {
		t.Fatalf("control: restoreReadingState is declared %d times, so this reading of the source is wrong", len(restore))
	}
	written := src.fieldsWritten(restore[0], paramNames(restore[0])[0], 1)
	readOff, writtenOn := launchReaches(t, src)
	for _, f := range []string{"preferredVersion", "staleVersions", "CurrentVersion"} {
		if !written[f] {
			t.Fatalf("control: the restore no longer writes %s, so this reading of the source is wrong", f)
		}
	}
	if !readOff["CurrentVersion"] {
		t.Fatal("control: the hand-off no longer copies CurrentVersion off the restore's state, so this reading of the source is wrong")
	}
	var dropped []string
	for f := range written {
		if !readOff[f] && !writtenOn[f] {
			dropped = append(dropped, f)
		}
	}
	sort.Strings(dropped)
	if got := strings.Join(dropped, " "); got != "" {
		t.Fatalf("the launch hand-off drops [%s] of what the restore records: the reader never sees it. "+
			"D18 was exactly preferredVersion and staleVersions; anything the restore records must reach "+
			"the live state, through adoptLaunch.", got)
	}
}

// launchReaches returns the fields of the restore's state that the launch
// carries to the live one, by each road a fix could take. A field is read
// off the restore's state if StartBackgroundLoad reads loaded.X anywhere at
// all — a copy, a multiple assignment, a clone, a range, a condition — or
// hands the restore's state, under any name, to a function or method that
// does, however deep. A field is written on the live state if
// StartBackgroundLoad assigns it there, or a function it hands the live state
// to does, or one that function hands it to. Two calls and no more, because
// three reach applyLoadedVersion through the dev builds' automatic switch, and
// a few more through a link consumed at launch, and it writes both records for
// reasons of its own. Counting a field that is only read, or written for
// another reason, errs the safe way: the pin fails, and a person looks.
func launchReaches(t *testing.T, src packageSource) (readOff, writtenOn map[string]bool) {
	t.Helper()
	launch := src.funcs["StartBackgroundLoad"]
	if len(launch) != 1 {
		t.Fatalf("control: StartBackgroundLoad is declared %d times, so this reading of the source is wrong", len(launch))
	}
	return src.fieldsRead(launch[0], "loaded", -1), src.fieldsWritten(launch[0], "state", 2)
}

// packageSource is the package's own Go files, tests excluded and every build
// tag included: its functions and methods by name, each name with every
// declaration of it, and AppState's field names. A call is resolved to every
// declaration its name could mean, which can only find more.
type packageSource struct {
	funcs   map[string][]*ast.FuncDecl
	methods map[string][]*ast.FuncDecl
	fields  map[string]bool
}

func parsePackageSource(t *testing.T) packageSource {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	src := packageSource{funcs: map[string][]*ast.FuncDecl{}, methods: map[string][]*ast.FuncDecl{}, fields: map[string]bool{}}
	fset := token.NewFileSet()
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if d.Body == nil {
					continue
				}
				if d.Recv == nil {
					src.funcs[d.Name.Name] = append(src.funcs[d.Name.Name], d)
				} else {
					src.methods[d.Name.Name] = append(src.methods[d.Name.Name], d)
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok || ts.Name.Name != "AppState" {
						continue
					}
					if st, ok := ts.Type.(*ast.StructType); ok {
						for _, field := range st.Fields.List {
							for _, n := range field.Names {
								src.fields[n.Name] = true
							}
						}
					}
				}
			}
		}
	}
	if len(src.fields) == 0 {
		t.Fatal("control: AppState's fields were not found, so this reading of the source is wrong")
	}
	return src
}

// fieldsRead is every AppState field read off name in fn, or in what fn hands
// it to, depth calls deep (negative: all the way).
func (src packageSource) fieldsRead(fn *ast.FuncDecl, name string, depth int) map[string]bool {
	out := map[string]bool{}
	src.follow(fn, name, depth, map[string]int{}, func(body ast.Node, names map[string]bool) {
		targets := assignmentTargets(body)
		ast.Inspect(body, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok && !targets[sel] {
				if f, ok := src.fieldOf(sel, names); ok {
					out[f] = true
				}
			}
			return true
		})
	})
	return out
}

// fieldsWritten is every AppState field written on name in fn, or in what fn
// hands it to, depth calls deep: name.X = …, name.X[k] = …, name.X++, and the
// map and slice builtins that change name.X in place.
func (src packageSource) fieldsWritten(fn *ast.FuncDecl, name string, depth int) map[string]bool {
	out := map[string]bool{}
	src.follow(fn, name, depth, map[string]int{}, func(body ast.Node, names map[string]bool) {
		for sel := range assignmentTargets(body) {
			if f, ok := src.fieldOf(sel, names); ok {
				out[f] = true
			}
		}
		ast.Inspect(body, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok && len(call.Args) > 0 {
				if b, ok := call.Fun.(*ast.Ident); ok && (b.Name == "delete" || b.Name == "clear") {
					if sel, ok := call.Args[0].(*ast.SelectorExpr); ok {
						if f, ok := src.fieldOf(sel, names); ok {
							out[f] = true
						}
					}
				}
			}
			return true
		})
	})
	return out
}

// fieldOf reports the AppState field sel selects, when sel is name.X for one
// of names and X is a field rather than a method.
func (src packageSource) fieldOf(sel *ast.SelectorExpr, names map[string]bool) (string, bool) {
	id, ok := sel.X.(*ast.Ident)
	if !ok || !names[id.Name] || !src.fields[sel.Sel.Name] {
		return "", false
	}
	return sel.Sel.Name, true
}

// follow visits fn's body with the names name goes by there — itself and
// anything assigned from it — and then every package function or method fn
// hands one of those names to, as an argument or as the receiver, under the
// name it has inside that function, depth calls deep (negative: all the way).
func (src packageSource) follow(fn *ast.FuncDecl, name string, depth int, seen map[string]int, visit func(ast.Node, map[string]bool)) {
	// A function met again with more calls left to follow is followed again:
	// the first meeting may have been at the edge.
	reach := depth
	if reach < 0 {
		reach = math.MaxInt
	}
	key := fmt.Sprintf("%p/%s", fn, name)
	if been, ok := seen[key]; name == "" || name == "_" || (ok && been >= reach) {
		return
	}
	seen[key] = reach
	names := aliasesOf(fn.Body, name)
	visit(fn.Body, names)
	if depth == 0 {
		return
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		var callees []*ast.FuncDecl
		switch f := call.Fun.(type) {
		case *ast.Ident:
			callees = src.funcs[f.Name]
		case *ast.SelectorExpr:
			callees = src.methods[f.Sel.Name]
			if recv, ok := f.X.(*ast.Ident); ok && names[recv.Name] {
				for _, m := range callees {
					if m.Recv != nil && len(m.Recv.List[0].Names) > 0 {
						src.follow(m, m.Recv.List[0].Names[0].Name, depth-1, seen, visit)
					}
				}
			}
		}
		for i, arg := range call.Args {
			if id, ok := arg.(*ast.Ident); !ok || !names[id.Name] {
				continue
			}
			for _, callee := range callees {
				if params := paramNames(callee); i < len(params) {
					src.follow(callee, params[i], depth-1, seen, visit)
				}
			}
		}
		return true
	})
}

// aliasesOf is name and every identifier body assigns it to: x := name,
// x = name, var x = name, and the same position in a multiple assignment.
func aliasesOf(body ast.Node, name string) map[string]bool {
	names := map[string]bool{name: true}
	for grew := true; grew; {
		grew = false
		ast.Inspect(body, func(n ast.Node) bool {
			var lhs []ast.Expr
			var rhs []ast.Expr
			switch n := n.(type) {
			case *ast.AssignStmt:
				lhs, rhs = n.Lhs, n.Rhs
			case *ast.ValueSpec:
				for _, id := range n.Names {
					lhs = append(lhs, id)
				}
				rhs = n.Values
			default:
				return true
			}
			if len(lhs) != len(rhs) {
				return true
			}
			for i := range rhs {
				r, ok1 := rhs[i].(*ast.Ident)
				l, ok2 := lhs[i].(*ast.Ident)
				if ok1 && ok2 && names[r.Name] && !names[l.Name] && l.Name != "_" {
					names[l.Name] = true
					grew = true
				}
			}
			return true
		})
	}
	return names
}

// assignmentTargets is every selector body assigns to or increments, with or
// without an index: the X in X = …, X[k] = … and X++.
func assignmentTargets(body ast.Node) map[*ast.SelectorExpr]bool {
	out := map[*ast.SelectorExpr]bool{}
	add := func(e ast.Expr) {
		if ix, ok := e.(*ast.IndexExpr); ok {
			e = ix.X
		}
		if sel, ok := e.(*ast.SelectorExpr); ok {
			out[sel] = true
		}
	}
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			for _, l := range n.Lhs {
				add(l)
			}
		case *ast.IncDecStmt:
			add(n.X)
		}
		return true
	})
	return out
}

// paramNames is fn's parameter names in order, one per parameter; an unnamed
// parameter is "".
func paramNames(fn *ast.FuncDecl) []string {
	var out []string
	for _, field := range fn.Type.Params.List {
		if len(field.Names) == 0 {
			out = append(out, "")
			continue
		}
		for _, n := range field.Names {
			out = append(out, n.Name)
		}
	}
	return out
}
