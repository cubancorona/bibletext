//go:build !ios && !android

package bibletext

import (
	"math"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// The test driver runs fyne.Do on a timer's goroutine, without a UI queue.
// Host tests hold scheduling; the resize tests below advance their own clock.
func init() {
	desktopNavigationAfter = func(time.Duration, func()) func() { return func() {} }
}

// uiMarshalRecordingApp counts what reaches the driver's UI marshal. fyne.Do
// resolves to CurrentApp().Driver().DoFromGoroutine, so a schedule that ran
// its callback on the timer's own goroutine would leave the count at zero.
type uiMarshalRecordingApp struct {
	fyne.App
	marshalled *atomic.Int32
}

func (a uiMarshalRecordingApp) Driver() fyne.Driver {
	return uiMarshalRecordingDriver{a.App.Driver(), a.marshalled}
}

type uiMarshalRecordingDriver struct {
	fyne.Driver
	marshalled *atomic.Int32
}

func (d uiMarshalRecordingDriver) DoFromGoroutine(fn func(), wait bool) {
	d.marshalled.Add(1)
	d.Driver.DoFromGoroutine(fn, wait)
}

// The production schedule, which the init above keeps out of the other tests:
// the callback reaches the UI goroutine through the driver's marshal once the
// delay has passed, and a cancellation before the delay keeps it from running.
func TestDesktopNavigationSettleHandsItsCallbackToTheUIMarshal(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	var marshalled atomic.Int32
	fyne.SetCurrentApp(uiMarshalRecordingApp{app, &marshalled})
	t.Cleanup(func() { fyne.SetCurrentApp(app) })
	ran := make(chan struct{}, 2)
	scheduleDesktopNavigationSettle(time.Millisecond, func() { ran <- struct{}{} })
	select {
	case <-ran:
	case <-time.After(10 * time.Second):
		t.Fatal("the settle never ran its callback")
	}
	if got := marshalled.Load(); got != 1 {
		t.Fatalf("the settled callback reached the UI marshal %d times, want 1", got)
	}
	cancel := scheduleDesktopNavigationSettle(50*time.Millisecond, func() { ran <- struct{}{} })
	cancel()
	select {
	case <-ran:
		t.Fatal("a cancelled settle ran its callback")
	case <-time.After(250 * time.Millisecond):
	}
	if got := marshalled.Load(); got != 1 {
		t.Fatalf("a cancelled settle reached the UI marshal: %d calls in all, want 1", got)
	}
}

type desktopNavigationTimer struct {
	at      time.Duration
	run     func()
	stopped bool
}

type desktopNavigationClock struct {
	now    time.Duration
	timers []*desktopNavigationTimer
}

type unsizedDesktopCanvas struct{ fyne.Canvas }

func (unsizedDesktopCanvas) Size() fyne.Size { return fyne.Size{} }

type unsizedDesktopWindow struct{ fyne.Window }

func (w unsizedDesktopWindow) Canvas() fyne.Canvas { return unsizedDesktopCanvas{w.Window.Canvas()} }

func holdDesktopNavigationClock(t *testing.T) *desktopNavigationClock {
	t.Helper()
	clock := &desktopNavigationClock{}
	previous := desktopNavigationAfter
	desktopNavigationAfter = func(delay time.Duration, run func()) func() {
		timer := &desktopNavigationTimer{at: clock.now + delay, run: run}
		clock.timers = append(clock.timers, timer)
		return func() { timer.stopped = true }
	}
	t.Cleanup(func() { desktopNavigationAfter = previous })
	return clock
}

func (c *desktopNavigationClock) advance(delta time.Duration) {
	c.now += delta
	for _, timer := range c.timers {
		if !timer.stopped && timer.at <= c.now {
			timer.stopped = true
			timer.run()
		}
	}
}

func desktopNavigationIn(root fyne.CanvasObject) *desktopNavigation {
	if box, ok := root.(*fyne.Container); ok {
		if watcher, ok := box.Layout.(*desktopNavigation); ok {
			return watcher
		}
		for _, child := range box.Objects {
			if watcher := desktopNavigationIn(child); watcher != nil {
				return watcher
			}
		}
	}
	return nil
}

func desktopNavigationFixture(t *testing.T, tab int) (*AppState, *desktopNavigation, *desktopNavigationClock) {
	t.Helper()
	t.Setenv("BIBLETEXT_DESKTOP_TABS", "")
	clock := holdDesktopNavigationClock(t)
	app := test.NewApp()
	t.Cleanup(app.Quit)
	state := longPsalmState()
	state.CurrentTab, state.loadPhase = tab, loadReady
	window := app.NewWindow("Adaptive navigation")
	t.Cleanup(window.Close)
	window.Resize(fyne.NewSize(1100, 760))
	previous := useStyledPane
	useStyledPane = func() bool { return true }
	t.Cleanup(func() { useStyledPane = previous; resetStyledWiring() })
	window.SetContent(CreateMainUI(app, state, window))
	watcher := desktopNavigationIn(window.Content())
	if watcher == nil {
		t.Fatal("the desktop entry point did not install adaptive navigation")
	}
	return state, watcher, clock
}

func TestDesktopNavFollowsSharedWindowRuleAndOverrides(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	state := sampleState()
	window := app.NewWindow("Navigation rule")
	t.Cleanup(window.Close)
	state.window = window
	for _, override := range []string{"", "nonsense", "rail", "bar", "1", "sidebar", "0"} {
		t.Setenv("BIBLETEXT_DESKTOP_TABS", override)
		for _, size := range []fyne.Size{{Width: 640, Height: 900}, {Width: 900, Height: 640}, {Width: 640, Height: 640}, {Width: 641, Height: 640}, {Width: 639, Height: 640}} {
			window.Resize(size)
			want := mobileRailWanted(false, size.Width, size.Height)
			switch override {
			case "rail":
				want = true
			case "bar", "1", "sidebar", "0":
				want = false
			}
			if got := compactNavRail(state); got != want {
				t.Errorf("override %q, window %v: rail=%v, want %v", override, size, got, want)
			}
		}
	}
}

func TestDesktopAdaptiveNavigationStartsWithUnsizedRail(t *testing.T) {
	t.Setenv("BIBLETEXT_DESKTOP_TABS", "")
	clock := holdDesktopNavigationClock(t)
	app := test.NewApp()
	t.Cleanup(app.Quit)
	window := app.NewWindow("Initial navigation")
	t.Cleanup(window.Close)
	state := sampleState()
	state.CurrentTab, state.loadPhase = 1, loadReady
	root := CreateMainUI(app, state, unsizedDesktopWindow{window})
	watcher := desktopNavigationIn(root)
	if watcher == nil || !watcher.built.rail {
		t.Fatal("an unsized desktop did not start with its initial rail")
	}
	state.window = window
	window.SetContent(root)
	window.Resize(fyne.NewSize(620, 900))
	clock.advance(desktopNavigationSettle)
	if watcher.built.rail {
		t.Fatal("the first real tall canvas did not move to the bottom bar")
	}
}

func TestDesktopAdaptiveNavigationSettlesAfterLastResize(t *testing.T) {
	state, watcher, clock := desktopNavigationFixture(t, 1)
	for _, size := range []fyne.Size{{Width: 620, Height: 900}, {Width: 621, Height: 900}, {Width: 622, Height: 900}} {
		state.window.Resize(size)
		clock.advance(200 * time.Millisecond)
		if !watcher.built.rail {
			t.Fatal("the rail moved during a live resize")
		}
	}
	clock.advance(49 * time.Millisecond)
	if !watcher.built.rail {
		t.Fatal("the rail moved before 250ms after the last resize")
	}
	clock.advance(time.Millisecond)
	if watcher.built.rail || !watcher.built.centred {
		t.Fatal("the settled tall window did not receive the shared centred bottom bar")
	}
	state.window.Resize(fyne.NewSize(900, 640))
	clock.advance(desktopNavigationSettle - time.Millisecond)
	if watcher.built.rail {
		t.Fatal("the bottom bar moved before the second resize settled")
	}
	clock.advance(time.Millisecond)
	if !watcher.built.rail {
		t.Fatal("the settled wide window did not receive the rail")
	}
	// No dead zone: one point across square changes the settled result.
	for _, width := range []float32{639, 640, 639, 641} {
		state.window.Resize(fyne.NewSize(width, 640))
		clock.advance(desktopNavigationSettle)
		if watcher.built.rail != (width >= 640) {
			t.Errorf("settled width %v kept the wrong edge", width)
		}
	}
}

func TestDesktopAdaptiveNavigationIgnoresStaleResizeCallbacks(t *testing.T) {
	state, watcher, clock := desktopNavigationFixture(t, 1)
	state.window.Resize(fyne.NewSize(620, 900))
	if len(clock.timers) == 0 {
		t.Fatal("the placement change scheduled no settle")
	}
	queued := clock.timers[len(clock.timers)-1].run
	state.window.Resize(fyne.NewSize(1100, 760))
	queued() // a stopped timer can already have queued its callback
	clock.advance(desktopNavigationSettle)
	if !watcher.built.rail {
		t.Fatal("a stale callback changed navigation after resizing back")
	}
	state.window.Resize(fyne.NewSize(620, 900))
	queued = clock.timers[len(clock.timers)-1].run
	state.CurrentTab = 2
	rebuildWindow(state)
	queued()
	if !watcher.built.rail || desktopNavigationIn(state.window.Content()) == watcher {
		t.Fatal("a callback from the old tab changed its detached tree")
	}
	watcher = desktopNavigationIn(state.window.Content())
	state.window.Resize(fyne.NewSize(1100, 760))
	queued = clock.timers[len(clock.timers)-1].run
	state.IsFullScreen = true
	rebuildWindow(state)
	queued()
	if watcher.built.rail || desktopNavigationIn(state.window.Content()) != nil || countTabCells(state.window.Content()) != 0 {
		t.Fatal("a stale resize disturbed chosen full-screen reading")
	}
}

// A settle timer stopped by a later resize can already have queued its
// callback. When the later shape still wants the same move, the callback's own
// reading of the live canvas agrees with it, so only its generation keeps the
// move waiting for the re-armed settle.
func TestDesktopAdaptiveNavigationStaleCallbackWaitsForTheReArmedSettle(t *testing.T) {
	state, watcher, clock := desktopNavigationFixture(t, 1)
	state.window.Resize(fyne.NewSize(620, 900))
	if len(clock.timers) == 0 {
		t.Fatal("the placement change scheduled no settle")
	}
	stale := clock.timers[len(clock.timers)-1].run
	state.window.Resize(fyne.NewSize(621, 900))
	stale()
	if !watcher.built.rail {
		t.Fatal("a stopped timer's callback moved the navigation before the last resize settled")
	}
	clock.advance(desktopNavigationSettle)
	if watcher.built.rail {
		t.Fatal("the re-armed settle did not move the navigation")
	}
}

func TestDesktopAdaptiveNavigationTabCallbacksFollowMovedChrome(t *testing.T) {
	state, _, clock := desktopNavigationFixture(t, 1)
	for _, target := range []struct {
		tab  int
		size fyne.Size
	}{
		{2, fyne.NewSize(620, 900)},
		{0, fyne.NewSize(1100, 760)},
		{1, fyne.NewSize(620, 900)},
	} {
		state.window.Resize(target.size)
		clock.advance(desktopNavigationSettle)
		var button *tabCell
		walkTree(state.window.Content(), func(object fyne.CanvasObject) {
			if cell, ok := object.(*tabCell); ok && cell.label == tabDestinations()[target.tab].label {
				button = cell
			}
		})
		if button == nil {
			t.Fatal("the moved navigation lost a destination")
		}
		test.Tap(button)
		if state.CurrentTab != target.tab {
			t.Fatal("the moved destination's callback did not select its tab")
		}
		walkTree(state.window.Content(), func(object fyne.CanvasObject) {
			if cell, ok := object.(*tabCell); ok && cell.active != (cell.label == button.label) {
				t.Error("the rebuilt tab did not mark its active destination")
			}
		})
	}
}

func TestDesktopAdaptiveNavigationPreservesActivePageAndSheet(t *testing.T) {
	for _, tab := range []int{0, 1, 2} {
		t.Run(tabDestinations()[tab].label, func(t *testing.T) {
			state, watcher, clock := desktopNavigationFixture(t, tab)
			root, header, content := state.window.Content(), watcher.header, watcher.content
			generation := windowRebuildGen
			controls := desktopControlMinimums(header)
			for control, size := range desktopControlMinimums(content) {
				controls[control] = size
			}
			fields := make(map[pageField]fyne.Focusable, len(state.pageFields))
			for key, field := range state.pageFields {
				fields[key] = field
			}
			pane, scroll := styledPane, styledScroll
			var selected string
			if tab == 0 {
				pane.selAnchor, pane.selStart, pane.selEnd = 8, 8, 25
				selected = pane.copySelected()
				scroll.ScrollToOffset(fyne.NewPos(0, 500))
			}
			entry := widget.NewEntry()
			entry.SetText("A synthetic unfinished draft")
			popup := widget.NewModalPopUp(container.NewPadded(entry), state.window.Canvas())
			popup.Show()
			popup.Resize(fyne.NewSize(340, 180))
			state.window.Canvas().Focus(entry)
			for _, size := range []fyne.Size{{Width: 620, Height: 900}, {Width: 1100, Height: 760}} {
				verse, delta, _, _ := captureStyledAnchor()
				state.window.Resize(size)
				clock.advance(desktopNavigationSettle)
				for pass := 0; pass < 3; pass++ {
					state.window.Content().Refresh()
				}
				if state.window.Content() != root || watcher.header != header || watcher.content != content || windowRebuildGen != generation {
					t.Fatal("moving navigation replaced the page or app header")
				}
				if state.window.Canvas().Overlays().Top() != popup || !popup.Visible() || state.window.Canvas().Focused() != entry || entry.Text != "A synthetic unfinished draft" {
					t.Fatal("moving navigation disturbed the same open sheet, draft or focus")
				}
				if tab == 0 && (styledPane != pane || styledScroll != scroll || pane.copySelected() != selected) {
					t.Fatal("moving navigation replaced the reading pane or lost its selection")
				}
				if tab == 0 {
					gotVerse, gotDelta, _, _ := captureStyledAnchor()
					if gotVerse != verse || math.Abs(gotDelta-delta) > 1 {
						t.Errorf("moving navigation changed the reading anchor: verse %d+%.1f became %d+%.1f", verse, delta, gotVerse, gotDelta)
					}
				}
				for control, minimum := range controls {
					if got := control.MinSize(); got != minimum {
						t.Errorf("moving navigation changed a %T control's minimum: %v became %v", control, minimum, got)
					}
				}
				for key, field := range fields {
					if state.pageFields[key] != field {
						t.Error("moving navigation rebound a page field to a new widget")
					}
				}
				cells := 0
				walkTree(watcher.body, func(object fyne.CanvasObject) {
					if cell, ok := object.(*tabCell); ok {
						cells++
						if cell.active != (cell.label == tabDestinations()[tab].label) {
							t.Error("the moved navigation lost its selected destination")
						}
						wantPad := tabBarEdgePadY
						if watcher.built.rail {
							wantPad = 0
						}
						if cell.padY != wantPad || cell.iconImg.MinSize() != fyne.NewSquareSize(tabCellIconSize) || cell.text.TextSize != tabCellLabelSize {
							t.Error("moving navigation changed the shared cell/icon/label metrics")
						}
					}
				})
				if cells != len(tabDestinations()) {
					t.Errorf("the moved edge carries %d cells, want %d", cells, len(tabDestinations()))
				}
				var expected fyne.CanvasObject
				if watcher.built.rail {
					expected = buildTabRail(state)
				} else {
					expected = buildMobileTabBar(state)
				}
				minimums := desktopTabMinimums(expected)
				for label, minimum := range desktopTabMinimums(watcher.body) {
					if minimums[label] != minimum {
						t.Errorf("%s minimum differs from unchanged shared chrome: %v, want %v", label, minimum, minimums[label])
					}
				}
			}
			popup.Hide()
			if tab == 2 {
				state.focusSearch()
				if state.window.Canvas().Focused() != fields[pageFieldSearch] {
					t.Error("the Search hook no longer focuses the retained search field")
				}
			}
		})
	}
}

func desktopControlMinimums(root fyne.CanvasObject) map[fyne.CanvasObject]fyne.Size {
	minimums := map[fyne.CanvasObject]fyne.Size{}
	walkTree(root, func(object fyne.CanvasObject) {
		switch object.(type) {
		case *widget.Button, *iconTapButton, *tapText, *referenceButton, *coverButton:
			minimums[object] = object.MinSize()
		}
	})
	return minimums
}

func desktopTabMinimums(root fyne.CanvasObject) map[string]fyne.Size {
	minimums := map[string]fyne.Size{}
	walkTree(root, func(object fyne.CanvasObject) {
		if cell, ok := object.(*tabCell); ok {
			minimums[cell.label] = cell.MinSize()
		}
	})
	return minimums
}

func TestDesktopAdaptiveNavigationLeavesForcedAndFullScreenLayoutsAlone(t *testing.T) {
	state, _, clock := desktopNavigationFixture(t, 1)
	for _, override := range []string{"rail", "bar", "sidebar"} {
		t.Setenv("BIBLETEXT_DESKTOP_TABS", override)
		state.window.SetContent(CreateMainUI(state.app, state, state.window))
		if desktopNavigationIn(state.window.Content()) != nil {
			t.Fatalf("forced %s installed adaptive navigation", override)
		}
		root := state.window.Content()
		state.window.Resize(fyne.NewSize(620, 900))
		clock.advance(desktopNavigationSettle)
		state.window.Resize(fyne.NewSize(1100, 760))
		clock.advance(desktopNavigationSettle)
		if state.window.Content() != root {
			t.Error("a forced layout was rebuilt by adaptive navigation")
		}
	}
	t.Setenv("BIBLETEXT_DESKTOP_TABS", "")
	state.CurrentTab, state.IsFullScreen = 0, true
	state.window.SetContent(CreateMainUI(state.app, state, state.window))
	if desktopNavigationIn(state.window.Content()) != nil || countTabCells(state.window.Content()) != 0 {
		t.Fatal("chosen full-screen reading installed navigation")
	}
}

func TestDesktopAdaptiveNavigationRetainsStyledSelectionAcrossReflow(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	pane := newTestPane(t, psalm23State(), 420)
	pane.selAnchor, pane.selStart, pane.selEnd = 8, 8, 35
	want := pane.copySelected()
	for _, width := range []float32{260, 720, 420} {
		pane.Resize(fyne.NewSize(width, 300))
		if pane.copySelected() != want || pane.selAnchor != 8 || len(pane.selectionSpans()) == 0 {
			t.Fatalf("width %v lost the live selection", width)
		}
	}
	pane.verses[0].Text = strings.Repeat("Changed synthetic words. ", 4)
	pane.relayout(300)
	if pane.selAnchor != -1 || pane.selStart != -1 || pane.selEnd != -1 {
		t.Fatal("a changed text model retained selection offsets from the previous words")
	}
}
