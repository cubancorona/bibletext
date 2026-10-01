//go:build bibletextdev

package bibletext

// EVERY NAME BIBLETEXT_DEV_OPEN TAKES OPENS ITS SHEET, IN ITS STATE.
//
// Each name is opened on a phone's window (devOpenPhoneWindow, an iPhone held
// upright), and the sheet on top is read for the words only that sheet, in
// that state, shows: the cross-references waiting say they are finding
// passages and show no list, listed they show the list and not the waiting
// words. The checks are shown to fail on a name that opens nothing, and on a
// sheet opened in the wrong state. Each name BIBLETEXT_DEV_TAB takes selects
// its tab and builds the window on it. The release build's twin, which opens
// nothing and selects nothing for any name, is dev_open_guard_test.go.

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// devOpenWant is what a name's sheet must show: every text in has, none in
// hasnt (each matched within the sheet's texts), and, where list is not
// zero, a list that is showing (1) or not (-1).
type devOpenWant struct {
	has, hasnt []string
	list       int
}

// devOpenWants is what each name opens, by the words on its sheet.
var devOpenWants = map[string]devOpenWant{
	"settings":        {has: []string{"Settings", "Changes save automatically."}},
	"goto":            {has: []string{"Go to", "Book"}},
	"chapters":        {has: []string{"Go to", " chapters"}, hasnt: []string{"Book"}},
	"versions":        {has: []string{"Translation", "Choose a Bible version."}, hasnt: []string{"Sample Translation"}},
	"votd":            {has: []string{"Verse of the day", "Read in context"}},
	"votd-one":        {has: []string{"Verse of the day", "John 3:16"}},
	"votd-long":       {has: []string{"Verse of the day", "Psalms 119:1–16"}},
	"audio":           {has: []string{"Audio source"}},
	"note":            {has: []string{"Add a note"}},
	"ask":             {has: []string{"Ask about this passage"}},
	"ai-waiting":      {has: []string{"Explanation", "Reading the passage…", "Cancel"}},
	"xrefs-waiting":   {has: []string{"Cross-references", "Finding related passages…"}, list: -1},
	"xrefs":           {has: []string{"Cross-references", "Matthew 5:3"}, hasnt: []string{"Finding related passages…", "No cross-references"}, list: 1},
	"share-image":     {has: []string{"Share as image", "John 3:16"}},
	"note-offer":      {has: []string{"Someone added a note", "John 3:16"}},
	"link-notice":     {has: []string{"Shared in New King James Version"}},
	"version-loading": {has: []string{"Downloading "}},
	"version-error":   {has: []string{"Couldn't download "}},
	"versions-more": {has: []string{"Choose a Bible version.", "Sample Translation H  (SAMPLE-H)",
		"could not be opened this time", "is updating to its latest edition", "are showing a previous edition",
		"are under evaluation"}},
}

// devOpenPhone is the phone the names are opened on: an iPhone held upright,
// its canvas 393 by 852 points, the Dynamic Island's 59 points above its safe
// area and the home indicator's 34 below.
var devOpenPhone = struct{ w, h, top, bottom float32 }{393, 852, 59, 34}

// devOpenInsetLayout places the window's content as the mobile driver does
// (sizeContent): inside the safe area, less the theme's padding on every
// edge, a mobile window being padded.
type devOpenInsetLayout struct{ top, bottom, pad float32 }

func (devOpenInsetLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.Size{} }

func (l devOpenInsetLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objs {
		o.Move(fyne.NewPos(l.pad, l.top+l.pad))
		o.Resize(size.SubtractWidthHeight(2*l.pad, l.top+l.bottom+2*l.pad))
	}
}

// devOpenPhoneWindow lays the real window out on devOpenPhone, on the Read
// tab, inside a root widget like the layout watcher a phone's window holds
// (CreateMainUI), on a device that answers mobile, and gives the sheets the
// safe area the phone reports (canvasArea), with no keyboard up.
func devOpenPhoneWindow(t *testing.T) (*AppState, fyne.Window) {
	t.Helper()
	p := devOpenPhone
	app := test.NewTempApp(t)
	th := &bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()}
	app.Settings().SetTheme(th)
	fyne.SetCurrentApp(phoneTestApp{app})
	prevArea, prevFoot := canvasArea, keyboardFreeFoot
	keyboardFreeFoot.known, keyboardFreeFoot.foot = false, 0
	canvasArea = func(c fyne.Canvas) (fyne.Position, fyne.Size) {
		return fyne.NewPos(0, p.top), c.Size().SubtractWidthHeight(0, p.top+p.bottom)
	}
	t.Cleanup(func() { canvasArea, keyboardFreeFoot = prevArea, prevFoot })
	softKeyboard(t, false)

	st := sampleState()
	w := app.NewWindow("phone")
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(p.w, p.h))
	st.window, st.app, st.theme = w, app, th
	// No assistant key: Settings would fetch the provider's model list for one.
	st.aiKeys = newKeyStoreWith(newFakePrefs())
	w.SetPadded(false)
	root := &wrappingRoot{phoneRoot{content: buildCompactUI(st)}}
	root.ExtendBaseWidget(root)
	w.SetContent(container.New(devOpenInsetLayout{p.top, p.bottom, th.Size(theme.SizeNamePadding)}, root))
	w.Resize(fyne.NewSize(p.w, p.h))
	if st.header == nil {
		t.Fatal("the window's header was not recorded")
	}
	return st, w
}

// sheetShows reports whether one of the texts on o contains s.
func sheetShows(o fyne.CanvasObject, s string) bool {
	for _, text := range treeTexts(o) {
		if strings.Contains(text, s) {
			return true
		}
	}
	return false
}

// checkDevOpen opens a sheet on w with open and says what is wrong with it
// against want, or nil.
func checkDevOpen(st *AppState, w fyne.Window, open func(*AppState) bool, want devOpenWant) error {
	if !open(st) {
		return fmt.Errorf("the name opens nothing")
	}
	popup, ok := w.Canvas().Overlays().Top().(*widget.PopUp)
	if !ok || popup == nil || !popup.Visible() {
		return fmt.Errorf("no sheet is showing; the top overlay is %T", w.Canvas().Overlays().Top())
	}
	for _, s := range want.has {
		if !sheetShows(popup, s) {
			return fmt.Errorf("the sheet does not show %q; it shows %q", s, treeTexts(popup))
		}
	}
	for _, s := range want.hasnt {
		if sheetShows(popup, s) {
			return fmt.Errorf("the sheet shows %q", s)
		}
	}
	if want.list != 0 {
		list := findScroll(popup)
		showing := list != nil && list.Visible()
		if showing != (want.list > 0) {
			return fmt.Errorf("the sheet's list is showing: %v, want %v", showing, want.list > 0)
		}
	}
	return nil
}

// devOpenWindow is a fresh phone window for one name, with the sheets'
// timers held as the sheet tests hold them, and Matthew 5:3 and Luke 6:20
// in its Bible, which a reader's Bible has and the sample does not.
func devOpenWindow(t *testing.T) (*AppState, fyne.Window) {
	t.Helper()
	holdSheetTimers(t)
	holdNoteSheetTimers(t)
	votdSynchronousRemeasure(t)
	st, w := devOpenPhoneWindow(t)
	withBeatitudes(st)
	st.CurrentBook, st.CurrentChapter = "John", 3
	return st, w
}

// parkDevStudy holds the study request the ai-waiting name makes for good, as
// the suite's study stubs do, and puts the request back once the parked one
// has started.
func parkDevStudy(t *testing.T) (started chan struct{}) {
	t.Helper()
	prevRun, prevWait := aiActionRun, devStudyWait
	started = make(chan struct{}, 4)
	devStudyWait = func(context.Context) {
		started <- struct{}{}
		select {} // never answered, so no completion touches the panel
	}
	t.Cleanup(func() { aiActionRun, devStudyWait = prevRun, prevWait })
	return started
}

func TestDevOpenNamesOpenTheirSheets(t *testing.T) {
	var names []string
	for _, sh := range devSheets() {
		names = append(names, sh.name)
	}
	if !slices.Equal(names, devOpenNames) {
		t.Fatalf("the names the table opens, %v, are not the names the release guard checks, %v", names, devOpenNames)
	}
	for _, name := range names {
		if _, ok := devOpenWants[name]; !ok {
			t.Errorf("%s: nothing says what it opens", name)
		}
	}
	if len(devOpenWants) != len(names) {
		t.Errorf("%d names are checked, and the table has %d", len(devOpenWants), len(names))
	}

	for _, sh := range devSheets() {
		name := sh.name
		t.Run(name, func(t *testing.T) {
			st, w := devOpenWindow(t)
			var started chan struct{}
			if name == "ai-waiting" {
				started = parkDevStudy(t)
			}
			// The cross-references load is the test's own, and it must still be
			// in place, never called, after either cross-references name.
			prevRun, prevLoad := crossRefsRun, crossRefsLoad
			t.Cleanup(func() { crossRefsRun, crossRefsLoad = prevRun, prevLoad })
			fetched := false
			crossRefsLoad = func() error { fetched = true; return nil }
			load, run := reflect.ValueOf(crossRefsLoad).Pointer(), reflect.ValueOf(crossRefsRun).Pointer()
			versions := slices.Clone(registeredVersions)
			cur, pref, stale := st.CurrentVersion, st.preferredVersion, st.staleVersions
			pending, downloading, seed := st.fullPending, st.fullDownloading, st.seedOnly

			if err := checkDevOpen(st, w, func(s *AppState) bool { return devOpenSheet(s, name) }, devOpenWants[name]); err != nil {
				t.Fatalf("%s, %s: %v", name, sh.opens, err)
			}

			if started != nil {
				waitParked(t, started, "the parked study request")
			}
			if fetched {
				t.Error("the cross-references were loaded")
			}
			if reflect.ValueOf(crossRefsLoad).Pointer() != load || reflect.ValueOf(crossRefsRun).Pointer() != run {
				t.Error("the cross-references load was left replaced")
			}
			if !slices.EqualFunc(registeredVersions, versions, func(a, b BibleVersion) bool { return a.ID == b.ID }) {
				t.Errorf("the registry was left with %d translations, not %d", len(registeredVersions), len(versions))
			}
			if st.CurrentVersion != cur || st.preferredVersion != pref || !reflect.DeepEqual(st.staleVersions, stale) ||
				st.fullPending != pending || st.fullDownloading != downloading || st.seedOnly != seed {
				t.Error("the translation state the notice was read from was left changed")
			}
		})
	}
}

// THE CHECKS CAN FAIL. A name that opens nothing, one that says it opened a
// sheet and did not, and a sheet opened in the wrong state or form, are each
// refused.
func TestDevOpenChecksRefuseTheWrongSheet(t *testing.T) {
	st, w := devOpenWindow(t)
	if devOpenSheet(st, "no-such-sheet") {
		t.Fatal("a name that is not in the table opened a sheet")
	}
	if top := w.Canvas().Overlays().Top(); top != nil {
		t.Fatalf("a name that is not in the table left %T on the window", top)
	}
	if err := checkDevOpen(st, w, func(*AppState) bool { return true }, devOpenWants["audio"]); err == nil {
		t.Error("control: an opener that opens nothing passed")
	}

	for _, tc := range []struct{ open, as string }{
		{"xrefs-waiting", "xrefs"},
		{"xrefs", "xrefs-waiting"},
		{"versions", "versions-more"},
		{"votd-one", "votd-long"},
		{"goto", "chapters"},
		{"note", "audio"},
	} {
		st, w := devOpenWindow(t)
		if err := checkDevOpen(st, w, func(s *AppState) bool { return devOpenSheet(s, tc.open) }, devOpenWants[tc.as]); err == nil {
			t.Errorf("control: %s passed as %s", tc.open, tc.as)
		}
	}
}

// THE LAUNCH OPENS THE SHEET IT NAMES: the control for the release build's
// guard, which watches a launch the same way and must see nothing. The
// opening runs here, on the test's goroutine, rather than on the timer's,
// and the delay it was given is held under the time the guard watches for,
// so the guard would see the sheet a development build opens.
func TestDevOpenAtLaunchOpensTheNamedSheet(t *testing.T) {
	st, w := devOpenWindow(t)
	var delays []time.Duration
	prev := devOpenAfter
	devOpenAfter = func(d time.Duration, f func()) { delays = append(delays, d); f() }
	t.Cleanup(func() { devOpenAfter = prev })
	if !launchOpensASheet(t, st, w, []string{"chapters"}) {
		t.Fatal("control: BIBLETEXT_DEV_OPEN=chapters opened no sheet at launch")
	}
	if len(delays) != 1 || delays[0] <= 0 || delays[0] >= devOpenLaunchWait {
		t.Fatalf("the launch opened its sheet after %v; the release guard watches for %v", delays, devOpenLaunchWait)
	}
	if err := checkDevOpen(st, w, func(*AppState) bool { return true }, devOpenWants["chapters"]); err != nil {
		t.Fatal(err)
	}
}

// THE SIMULATOR SHOWS WHAT THE HOST TESTS MEASURE: the long verse-of-the-day
// passage and the synthetic translations are the sheet tests' own, and the
// one-verse passage is one verse, the one the sheets that quote a selection
// quote.
func TestDevOpenFixturesAreTheTestsOwn(t *testing.T) {
	if one := devOneDayVerse(); one.Lo != one.Hi || len(one.Verses) != 1 || one.Verses[0].Text != devQuote {
		t.Errorf("votd-one's passage is not the one verse the other sheets quote: %+v", one)
	}
	if !reflect.DeepEqual(devLongDayPassage(), longDayPassage()) {
		t.Error("votd-long's passage is not the sheet tests' long passage")
	}
	if devMoreTranslations != moreTranslations {
		t.Errorf("versions-more adds %d translations; the sheet tests add %d", devMoreTranslations, moreTranslations)
	}
	withMoreTranslations(t)
	for i, v := range devSampleVersions() {
		want, ok := versionByID(sampleTranslationID(i))
		if !ok {
			t.Fatalf("the sheet tests registered no %s", sampleTranslationID(i))
		}
		if v.ID != want.ID || v.Name != want.Name || v.Abbrev != want.Abbrev || v.Publisher != want.Publisher ||
			reflect.TypeOf(v.source) != reflect.TypeOf(want.source) || v.canSelect() != want.canSelect() {
			t.Errorf("versions-more's %s is not the sheet tests' %s", v.ID, want.ID)
		}
	}
	st, _ := devOpenWindow(t)
	if n := strings.Count(devEveryNotice(st), "\n"); n != 2 {
		t.Errorf("the notice versions-more opens with says %d facts, not three", n+1)
	}
}

// devTabShowing says what is wrong with st's window as the tab name names,
// or nil. Each build registers the page fields of the tab it builds and no
// other (registerPageField): Books its filter, Search its keyword field, and
// Read none, with the reading pane wired to the tree.
func devTabShowing(st *AppState, name string) error {
	books, search := st.pageFields[pageFieldBooks] != nil, st.pageFields[pageFieldSearch] != nil
	switch name {
	case "books":
		if !books || search {
			return fmt.Errorf("the window is not built on Books: its filter %v, the search field %v", books, search)
		}
	case "search":
		if !search || books {
			return fmt.Errorf("the window is not built on Search: its field %v, the books filter %v", search, books)
		}
	case "read":
		if len(st.pageFields) != 0 || st.showReading == nil {
			return fmt.Errorf("the window is not built on Read: %d page fields, the reading pane wired %v",
				len(st.pageFields), st.showReading != nil)
		}
	default:
		return fmt.Errorf("%s is no tab", name)
	}
	return nil
}

// THE LAUNCH SELECTS THE TAB IT NAMES, BEFORE IT RETURNS, and builds the
// window on it: the control for the release build's guard, which watches a
// launch the same way and must see the window stay where it was. The
// destination each name selects is the one its label names in the
// navigation, wherever that puts it.
func TestDevTabAtLaunchSelectsTheNamedTab(t *testing.T) {
	if !slices.Equal(devTabNames, devOpenTabNames) {
		t.Fatalf("the names the launch selects, %v, are not the names the release guard checks, %v", devTabNames, devOpenTabNames)
	}
	for _, name := range devOpenTabNames {
		t.Run(name, func(t *testing.T) {
			st, w := devOpenWindow(t)
			want, ok := devTabIndex(name)
			if !ok {
				t.Fatalf("%s names no destination", name)
			}
			if label := tabDestinations()[want].label; strings.ToLower(label) != name {
				t.Fatalf("%s selects tab %d, which is %s", name, want, label)
			}
			from := devTabLaunchFrom(name)
			if from == want {
				t.Fatalf("control: the launch naming %s is watched from the tab it names", name)
			}
			tab, rebuilt := launchSelectsATab(t, st, w, name, from)
			if tab != want || !rebuilt {
				t.Fatalf("BIBLETEXT_DEV_TAB=%s left the window on tab %d (built again: %v), want tab %d", name, tab, rebuilt, want)
			}
			if err := devTabShowing(st, name); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// THE TAB CHECKS CAN FAIL. A name that is not a tab, the dev Links page
// among them, selects nothing and builds nothing, and a window built on one
// tab is refused as each other.
func TestDevTabChecksRefuseTheWrongTab(t *testing.T) {
	st, w := devOpenWindow(t)
	for _, name := range []string{"links", "no-such-tab"} {
		if tab, rebuilt := launchSelectsATab(t, st, w, name, 0); tab != 0 || rebuilt {
			t.Errorf("BIBLETEXT_DEV_TAB=%s moved the window to tab %d (built again: %v)", name, tab, rebuilt)
		}
	}
	for _, name := range devOpenTabNames {
		launchSelectsATab(t, st, w, name, devTabLaunchFrom(name))
		if err := devTabShowing(st, name); err != nil {
			t.Fatalf("control: %v", err)
		}
		for _, other := range devOpenTabNames {
			if other != name && devTabShowing(st, other) == nil {
				t.Errorf("control: the window built on %s passed as %s", name, other)
			}
		}
	}
}

// A SHEET NAMED WITH A TAB OPENS OVER THAT TAB. The tab is selected first:
// building the window drains every sheet on it, so a tab selected after the
// sheet opened would leave no sheet.
func TestDevTabAndSheetAtLaunchOpenTheSheetOverTheTab(t *testing.T) {
	st, w := devOpenWindow(t)
	prev := devOpenAfter
	devOpenAfter = func(_ time.Duration, f func()) { f() }
	t.Cleanup(func() { devOpenAfter = prev })
	t.Setenv("BIBLETEXT_DEV_TAB", "books")
	t.Setenv("BIBLETEXT_DEV_OPEN", "chapters")
	devAutoOpenSheet(st)
	if err := devTabShowing(st, "books"); err != nil {
		t.Fatal(err)
	}
	if err := checkDevOpen(st, w, func(*AppState) bool { return true }, devOpenWants["chapters"]); err != nil {
		t.Fatal(err)
	}
}
