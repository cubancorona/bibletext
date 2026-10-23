package bibletext

// A window lays out by its own size from its first frame (launch_window.go).
//
// The reader's first layout can be built before the canvas has a size: the
// data loads while UIKit connects the window scene, and the canvas takes its
// size only at the first paint. Answered by the device, that layout was an
// iPhone's bottom bar and an iPad's rail, and the first paint of an iPhone
// opened sideways, or of an iPad opened upright, found the other wanted and
// rebuilt, with the wrong one on screen for a few frames. An Android window's
// first surface can be a single pixel, a third of a unit each way: square, so
// "wide", and a width the reading page was chosen from. Before the canvas has
// a size the layout now decides by the size the platform reports for the
// window (layoutSize), and the iOS read of it never holds up the layout.

import (
	"sync"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// pinLaunchSize stands in for the size the platform reports for the window
// before its canvas has one; 0x0 is no report. Each pin is a launch of its
// own, so nothing a previous one held carries over.
func pinLaunchSize(t *testing.T, w, h float32) {
	t.Helper()
	prev := launchSize
	launchSize = func() (float32, float32, bool) { return w, h, w > 0 && h > 0 }
	releaseLaunchSize()
	t.Cleanup(func() {
		launchSize = prev
		releaseLaunchSize()
	})
}

// A build decides its tree at its start and the layout watcher records the
// decision at its end, so the report the layout reads cannot change in
// between: Android's configuration is read live, and a rotation in the middle
// of a build would have left the watcher recording a layout the tree does not
// have, which no later resize sees as a change. The first question in a build
// asks the platform and the rest get its answer; the next build, or a newly
// kept iOS report, asks again.
func TestAReportedSizeIsHeldThroughABuild(t *testing.T) {
	resetWindowReport(t)
	launchReportRelayout = func() {}
	w, h := float32(914), float32(411)
	asked := 0
	prev := launchSize
	launchSize = func() (float32, float32, bool) { asked++; return w, h, true }
	t.Cleanup(func() {
		launchSize = prev
		releaseLaunchSize()
	})

	releaseLaunchSize() // a build begins
	if gw, gh := layoutSize(0, 0); gw != 914 || gh != 411 {
		t.Fatalf("the build's first question answered %vx%v, want the platform's 914x411", gw, gh)
	}
	w, h = 411, 914 // the device turns while the build runs
	for i := 0; i < 3; i++ {
		if gw, gh := layoutSize(0, 0); gw != 914 || gh != 411 {
			t.Fatalf("a later question in the same build answered %vx%v, want the 914x411 the build began with", gw, gh)
		}
	}
	if asked != 1 {
		t.Errorf("the platform was asked %d times in one build, want once", asked)
	}
	if gw, gh := layoutSize(402, 874); gw != 402 || gh != 874 {
		t.Errorf("control: a sized canvas answers %vx%v, want its own 402x874", gw, gh)
	}

	releaseLaunchSize() // the next build
	if gw, gh := layoutSize(0, 0); gw != 411 || gh != 914 {
		t.Errorf("the next build answered %vx%v, want the platform's 411x914 now", gw, gh)
	}

	w, h = 914, 411
	keepWindowReport(windowReport{w: 874, h: 402})
	if gw, gh := layoutSize(0, 0); gw != 914 || gh != 411 {
		t.Errorf("after a report was kept the layout answered %vx%v, want the platform asked again (914x411)", gw, gh)
	}
}

// phoneFrame is the layout an iPhone renders on tab, its terms composed as
// renderedLayout composes them on a phone: there compactNavRail is
// railForWindow (ui_mobile.go) and the presentation is
// phoneLandscapeReadingActive's decision for the live window, with the gate on
// as it is by default. The host build's compactNavRail is the desktop's and
// its presentation is off, so the phone's are spelled out here from the same
// functions.
func phoneFrame(st *AppState, tab int) renderedLayout {
	sz := st.window.Canvas().Size()
	w, h := layoutSize(sz.Width, sz.Height)
	landscape := phoneLandscapeReadingWanted(true, true, w, h)
	full := tab == 0 && landscape
	rail := railForWindow(st)
	return renderedLayout{
		class:     classifyLayout(sz.Width, false),
		rail:      !full && rail,
		landscape: landscape,
		centred:   !full && !rail && tabBarStyleFor(st) == tabBarEdgeCentred,
	}
}

func unsizedWindow(t *testing.T) (*AppState, fyne.Window) {
	t.Helper()
	test.NewTempApp(t)
	st := sampleState()
	w := test.NewWindow(nil)
	t.Cleanup(w.Close)
	st.window = w
	return st, w
}

func TestASidewaysLaunchLaysOutSidewaysFromTheFirstFrame(t *testing.T) {
	st, w := unsizedWindow(t)

	sideways := func(rail bool) renderedLayout {
		return renderedLayout{class: layoutCompact, rail: rail, landscape: true}
	}
	upright := renderedLayout{class: layoutCompact}
	for _, tc := range []struct {
		name  string
		w, h  float32 // the window the scene reports, and the canvas the first paint gives it
		tab   int
		first renderedLayout
	}{
		{"an iPhone 16 Pro opened sideways on Books", 874, 402, 1, sideways(true)},
		{"an iPhone 16 Pro opened sideways on Read", 874, 402, 0, sideways(false)},
		{"an iPhone 17 Pro Max opened sideways on Search", 956, 440, 2, sideways(true)},
		{"an iPhone SE opened sideways on Read", 667, 375, 0, sideways(false)},
		{"an iPhone 16 Pro opened upright on Books", 402, 874, 1, upright},
		{"an iPhone 16 Pro opened upright on Read", 402, 874, 0, upright},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pinLaunchSize(t, tc.w, tc.h)
			w.Resize(fyne.NewSize(0, 0))
			if sz := w.Canvas().Size(); sz.Width != 0 || sz.Height != 0 {
				t.Fatalf("control: the canvas is %vx%v, not unsized", sz.Width, sz.Height)
			}
			first := phoneFrame(st, tc.tab)
			if first != tc.first {
				t.Errorf("the first frame is %+v, want %+v", first, tc.first)
			}
			w.Resize(fyne.NewSize(tc.w, tc.h))
			if sized := phoneFrame(st, tc.tab); layoutWatcherNeedsRebuild(first, sized) {
				t.Errorf("the first paint, at %vx%v, rebuilds %+v as %+v", tc.w, tc.h, first, sized)
			}
		})
	}

	// Control: with no report, as before the window scene has been read, the
	// unsized first frame is the bar, and the sideways first paint rebuilds
	// it. This is the flash the comparison above has to be able to see.
	t.Run("no report", func(t *testing.T) {
		pinLaunchSize(t, 0, 0)
		w.Resize(fyne.NewSize(0, 0))
		first := phoneFrame(st, 1)
		if first != upright {
			t.Fatalf("control: with no report the first frame is %+v, want the bar", first)
		}
		w.Resize(fyne.NewSize(874, 402))
		if !layoutWatcherNeedsRebuild(first, phoneFrame(st, 1)) {
			t.Fatal("control: with no report the sideways first paint does not rebuild, so the comparison cannot see the flash")
		}
	})
}

// An iPad's navigation, as railForWindow composes it with the idiom: the
// reported window decides an unsized canvas, so an iPad opened upright, or a
// Split View or Stage Manager window taller than wide, draws the bar from its
// first frame, where the idiom alone drew the rail.
func TestAnUprightIPadLaunchDrawsTheBarFromTheFirstFrame(t *testing.T) {
	for _, tc := range []struct {
		name string
		w, h float32
		rail bool
	}{
		{"an iPad Pro 11-inch opened upright", 834, 1210, false},
		{"an iPad Pro 11-inch opened sideways", 1210, 834, true},
		{"a Split View third of an iPad held sideways", 375, 834, false},
		{"a Stage Manager window wider than tall", 900, 600, true},
		// Control: before the scene has been read the idiom answers, the rail.
		{"no report", 0, 0, true},
	} {
		pinLaunchSize(t, tc.w, tc.h)
		w, h := layoutSize(0, 0)
		if got := mobileRailWanted(true, w, h); got != tc.rail {
			t.Errorf("%s: an unsized iPad window's rail = %v, want %v", tc.name, got, tc.rail)
		}
	}
}

// The bar's dress, as tabBarStyleFor reads it: the reported width decides an
// unsized window, so an upright tablet's bar is dressed for its width from the
// first frame, and so is a Split View or split-screen window too narrow for
// the centred dress, where the idiom alone gave every tablet the centred one.
func TestTheBarsDressTakesTheReportedWidth(t *testing.T) {
	st, w := unsizedWindow(t)
	for _, tc := range []struct {
		name string
		w, h float32
		want tabBarStyle
	}{
		{"an iPad Pro 11-inch opened upright", 834, 1210, tabBarEdgeCentred},
		{"a Split View half of an upright iPad", 414, 1210, tabBarEdgeSpread},
		{"an iPhone opened upright", 402, 874, tabBarEdgeSpread},
	} {
		pinLaunchSize(t, tc.w, tc.h)
		w.Resize(fyne.NewSize(0, 0))
		first := tabBarStyleFor(st)
		w.Resize(fyne.NewSize(tc.w, tc.h))
		if sized := tabBarStyleFor(st); first != tc.want || sized != tc.want {
			t.Errorf("%s: the bar is dressed %v unsized and %v at its size, want %v both", tc.name, first, sized, tc.want)
		}
	}
}

// The navigation alone, as railForWindow answers it: an unsized canvas takes
// the reported window over the device, and a sized canvas its own shape over
// the report.
func TestRailForAnUnsizedWindowTakesTheReportedSize(t *testing.T) {
	st, w := unsizedWindow(t)
	for _, tc := range []struct {
		launchW, launchH float32
		w, h             float32
		rail             bool
	}{
		{874, 402, 0, 0, true},
		{402, 874, 0, 0, false},
		{0, 0, 0, 0, false},
		{874, 402, 402, 874, false},
		{402, 874, 874, 402, true},
	} {
		pinLaunchSize(t, tc.launchW, tc.launchH)
		w.Resize(fyne.NewSize(tc.w, tc.h))
		if got := railForWindow(st); got != tc.rail {
			t.Errorf("report %vx%v, canvas %vx%v: rail = %v, want %v", tc.launchW, tc.launchH, tc.w, tc.h, got, tc.rail)
		}
	}
}

// Android's first surface can be a single pixel, a third of a unit each way
// at three pixels to the unit. That is not a size: the layout decides by the
// activity's configured size until the real one arrives, and the reading page
// is chosen from the configured width, not a third of a unit. Before, the
// square third of a unit read as wide (a rail, and the full-screen page on a
// phone held upright), and its width pushed the phone page to a pane that
// then imported the chapter again as the book page.
func TestASinglePixelCanvasIsNotASize(t *testing.T) {
	st, w := unsizedWindow(t)
	const third = float32(1.0 / 3)
	w.Resize(fyne.NewSize(third, third))
	if sz := w.Canvas().Size(); sz.Width != third || sz.Height != third {
		t.Fatalf("control: the canvas is %vx%v, not a third of a unit each way", sz.Width, sz.Height)
	}

	if canvasSized(third, third) || canvasSized(1, 1) || canvasSized(0, 0) || canvasSized(402, 0) {
		t.Error("a canvas a unit or less on a side counts as sized")
	}
	if !canvasSized(402, 874) || !canvasSized(1.5, 1.5) {
		t.Error("control: a real canvas does not count as sized")
	}

	t.Run("with no report", func(t *testing.T) {
		pinLaunchSize(t, 0, 0)
		if railForWindow(st) {
			t.Error("a third of a unit each way drew the rail; a phone with no report keeps the bar")
		}
		if phoneLandscapeReadingWanted(true, true, third, third) {
			t.Error("a third of a unit each way took the full-screen page")
		}
	})
	t.Run("a phone held upright, by its configuration", func(t *testing.T) {
		pinLaunchSize(t, 411, 914)
		w, h := layoutSize(third, third)
		if railForWindow(st) || phoneLandscapeReadingWanted(true, true, w, h) {
			t.Errorf("an upright phone's configured %vx%v drew the rail or the full-screen page", w, h)
		}
	})
	t.Run("a phone held sideways, by its configuration", func(t *testing.T) {
		pinLaunchSize(t, 914, 380)
		w, h := layoutSize(third, third)
		if !railForWindow(st) || !phoneLandscapeReadingWanted(true, true, w, h) {
			t.Errorf("a sideways phone's configured %vx%v did not draw sideways", w, h)
		}
	})

	t.Run("the page", func(t *testing.T) {
		saveReadingPaneWidth(t)
		readingPaneWidth, readingPaneWindow = 0, 0
		readingPageOverride = nil
		readingWindowWidth = widestWindowWidth
		readingUnsizedPage = func() readingPageKind { return readingPagePhone }
		// A phone held sideways: 914 dp configured.
		readingColdWidth = func() float64 { return 914 }
		// The test driver keeps a sized dummy window of its own, which would
		// answer for the phone's: it gets the single pixel too.
		for _, other := range fyne.CurrentApp().Driver().AllWindows() {
			other.Resize(fyne.NewSize(third, third))
		}
		if got := widestWindowWidth(); got != 0 {
			t.Errorf("a third of a unit is a window width of %v; want none", got)
		}
		if p := currentReadingPage(); !p.Book() {
			t.Errorf("a phone configured 914 dp wide, on its single-pixel surface, pushed %+v, want the book page", p)
		}
		w.Resize(fyne.NewSize(402, 874))
		if got := widestWindowWidth(); got != 402 {
			t.Errorf("control: a sized window's width is %v, want 402", got)
		}
	})
}

// The page a chapter pushed before the canvas has a size opens on, on iOS:
// the window scene's width less the side safe-area insets (launchPaneWidth),
// the full-screen pane's width on an iPhone held sideways. The page that
// width chooses is the existing cold-width rule, held here too; whether iOS
// gives the width at all is the contract test's
// (TestTheLayoutNeverWaitsForTheWindowScene).
func TestTheReportedPaneWidthIsTheWindowLessItsSideInsets(t *testing.T) {
	for _, tc := range []struct {
		name string
		r    windowReport
		want float64
	}{
		{"an iPhone 17 Pro held sideways", windowReport{w: 874, h: 402, sides: 124}, 750},
		{"an iPhone 17 Pro held upright", windowReport{w: 402, h: 874}, 402},
		{"an iPhone SE held sideways", windowReport{w: 667, h: 375}, 667},
		{"insets wider than the window", windowReport{w: 100, h: 100, sides: 140}, 0},
	} {
		if got := tc.r.paneWidth(); got != tc.want {
			t.Errorf("%s: pane width %v, want %v", tc.name, got, tc.want)
		}
	}

	saveReadingPaneWidth(t)
	readingPaneWidth, readingPaneWindow = 0, 0
	readingWindowWidth = func() float64 { return 0 }
	readingPageOverride = nil
	readingUnsizedPage = func() readingPageKind { return readingPagePhone }
	readingColdWidth = windowReport{w: 874, h: 402, sides: 124}.paneWidth
	if p := currentReadingPage(); !p.Book() {
		t.Fatalf("a 750-point pane before the canvas has a size pushed %+v, want the book page", p)
	}
	readingColdWidth = windowReport{w: 402, h: 874}.paneWidth
	if p := currentReadingPage(); p.Book() {
		t.Fatalf("a 402-point pane before the canvas has a size pushed %+v, want the phone page", p)
	}
}

// resetWindowReport gives a test a fresh read of the window, as a launch has.
func resetWindowReport(t *testing.T) {
	t.Helper()
	prevRead, prevHandOff, prevRetry, prevRelayout := windowReportRead, windowReportHandOff, windowReportRetry, launchReportRelayout
	windowReportStart = sync.Once{}
	windowReportKept.Store(nil)
	t.Cleanup(func() {
		windowReportRead, windowReportHandOff, windowReportRetry, launchReportRelayout = prevRead, prevHandOff, prevRetry, prevRelayout
		windowReportStart = sync.Once{}
		windowReportKept.Store(nil)
	})
}

// The iOS read of the window scene waits for the main thread, which UIKit
// keeps busy for the first few hundred milliseconds of a launch. Made where
// the layout asks, it held back every launch's first build by that long,
// upright ones included. The read is made on a goroutine of its own: the
// layout's question is answered at once, with no report until the read
// lands; a read made before the scene is connected is made again; a landed
// read is kept on the Fyne goroutine, between two of its tasks, never by the
// reading goroutine, so the report cannot change in the middle of a build;
// keeping it asks the layout again, once; and the read is not made again for
// the launch.
func TestTheLayoutNeverWaitsForTheWindowRead(t *testing.T) {
	resetWindowReport(t)
	windowReportRetry = time.Millisecond

	release := make(chan bool) // true: a window is connected
	var mu sync.Mutex
	reads := 0
	windowReportRead = func() (windowReport, bool) {
		mu.Lock()
		reads++
		mu.Unlock()
		if <-release {
			return windowReport{w: 874, h: 402, sides: 124}, true
		}
		return windowReport{}, false
	}
	// The Fyne goroutine's queue: held here, and run when the test says.
	handedOff := make(chan func(), 4)
	windowReportHandOff = func(f func()) { handedOff <- f }
	relayouts := 0
	launchReportRelayout = func() { relayouts++ }

	ask := func() (windowReport, bool) {
		t.Helper()
		type answer struct {
			r  windowReport
			ok bool
		}
		got := make(chan answer, 1)
		go func() {
			r, ok := keptWindowReport()
			got <- answer{r, ok}
		}()
		select {
		case a := <-got:
			return a.r, a.ok
		case <-time.After(5 * time.Second):
			t.Fatal("the question waited for the platform's read")
			return windowReport{}, false
		}
	}

	if _, ok := ask(); ok {
		t.Fatal("a report before the read has landed")
	}
	release <- false // the scene is not connected yet
	if _, ok := ask(); ok {
		t.Fatal("a report from a read that found no window")
	}
	release <- true
	var keep func()
	select {
	case keep = <-handedOff:
	case <-time.After(5 * time.Second):
		t.Fatal("the read landed and nothing was handed to the Fyne goroutine")
	}
	if _, ok := ask(); ok {
		t.Fatal("the reading goroutine kept the report itself; a build in progress would see it change")
	}
	if relayouts != 0 {
		t.Fatal("the layout was asked again before the Fyne goroutine kept the report")
	}
	keep()
	r, ok := ask()
	if !ok || r.w != 874 || r.h != 402 || r.paneWidth() != 750 {
		t.Fatalf("the kept report is %+v (ok %v), want 874x402 less 124 at the sides", r, ok)
	}
	if relayouts != 1 {
		t.Fatalf("keeping the report asked the layout again %d times, want once", relayouts)
	}
	for i := 0; i < 3; i++ {
		ask()
	}
	select {
	case <-handedOff:
		t.Error("a second report was handed to the Fyne goroutine")
	case <-time.After(50 * time.Millisecond):
	}
	mu.Lock()
	defer mu.Unlock()
	if reads != 2 {
		t.Errorf("the platform's read was made %d times, want 2: once before the scene connected, once after", reads)
	}
}

// The hand-off is fyne.Do: a landed report is kept, and the layout watcher
// asked again, on the Fyne goroutine.
func TestTheReportsLandingAsksTheLayoutAgain(t *testing.T) {
	test.NewTempApp(t)
	resetWindowReport(t)
	asked := make(chan struct{}, 1)
	launchReportRelayout = func() { asked <- struct{}{} }
	windowReportLanded(windowReport{w: 834, h: 1210})
	select {
	case <-asked:
	case <-time.After(5 * time.Second):
		t.Fatal("the report landed and the layout was not asked again")
	}
	if r := windowReportKept.Load(); r == nil || r.w != 834 || r.h != 1210 {
		t.Fatalf("the landed report was not kept: %+v", r)
	}
}
