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
	return bad
}

// --- enumeration D: the launch cross-product ---------------------------------

func TestVersionLaunchStateSpace(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	var unexplained []string
	seen := map[string]bool{}
	cells := 0

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
	// Set equality: a fix that leaves its pin behind fails here.
	for _, d := range knownLaunchIncoherent {
		if !seen[d.name] {
			t.Errorf("%s is pinned as reachable but no cell reached it — if it is fixed, strike it from knownLaunchIncoherent and from docs/VERSION_STATES.md: %s", d.name, d.what)
		}
	}
	t.Logf("%d launch cells enumerated; %d pinned incoherent states reached", cells, len(seen))
}

// runLaunchCell drives ONE launch through the app's real restore, then through
// the exact tail loadStateData runs when the restore declines (app.go) — the
// two together are what the reader actually experiences, and the erasure lives
// in the seam between them.
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

	obs.onScreen = state.CurrentVersion
	obs.wideData = state.Bible.GetChaptersForBook("Tobit") > 0
	obs.keptHist = len(state.RecentChapters)
	// What the next navigation would write — the only record of the choice.
	obs.persists = snapshotReadingState(state, 0, 0, 0, 0, 0).Version
	// Every surface that could carry the news. There is exactly one.
	obs.told = fullPendingNotice(state) != ""
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
