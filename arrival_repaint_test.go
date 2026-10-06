package bibletext

import (
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// refreshCount is window content that counts the repaints it is asked for.
type refreshCount struct {
	widget.BaseWidget
	n int
}

func newRefreshCount() *refreshCount {
	r := &refreshCount{}
	r.ExtendBaseWidget(r)
	return r
}

func (r *refreshCount) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(canvas.NewRectangle(nil))
}

func (r *refreshCount) Refresh() { r.n++ }

// A SEARCH ARRIVAL'S REPAINT GOES THROUGH ITS DOOR. Armed straight on a
// timer, the repaint laid out the arriving test's whole window 160ms later on
// the timer's goroutine, while a later test laid out its own, and the two
// shared Fyne's unlocked stack of override themes: the verse-of-the-day card
// test failed now and then with a nil theme (arrivalRepaintAfter). Through
// the door, TestMain only counts it, and this test runs it on its own
// goroutine. Mutation guarded: openSearchResultRange arming time.AfterFunc
// itself (nothing is held here, and the suite's count does not move).
func TestASearchArrivalRepaintsThroughItsDoor(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	win := app.NewWindow("arrival")
	content := newRefreshCount()
	win.SetContent(content)

	var held []func()
	var delays []time.Duration
	prev := arrivalRepaintAfter
	arrivalRepaintAfter = func(d time.Duration, f func()) {
		delays = append(delays, d)
		held = append(held, f)
	}
	t.Cleanup(func() { arrivalRepaintAfter = prev })

	state := sampleState()
	state.window = win
	openSearchResult(state, Verse{BookName: "John", Chapter: 3, Verse: 16})
	if len(held) != 1 {
		t.Fatalf("a search arrival with a window armed %d repaints through arrivalRepaintAfter, want 1", len(held))
	}
	if delays[0] != arrivalRepaintDelay {
		t.Errorf("the repaint is armed %v after the arrival, want %v", delays[0], arrivalRepaintDelay)
	}
	before := content.n
	held[0]()
	if content.n != before+1 {
		t.Errorf("the repaint refreshed the window's content %d times, want once", content.n-before)
	}

	// A window gone by the time the beat comes has nothing to repaint.
	state.window = nil
	before = content.n
	held[0]()
	if content.n != before {
		t.Error("the repaint refreshed a window the state no longer holds")
	}

	// An arrival with no window arms nothing.
	held = nil
	openSearchResult(sampleState(), Verse{BookName: "John", Chapter: 3, Verse: 16})
	if len(held) != 0 {
		t.Errorf("an arrival with no window armed %d repaints", len(held))
	}
}

// The suite shuts the door: an arrival made by any other test only counts.
// Control for the test above, which takes the door over for itself.
func TestTheSuiteHoldsEveryArrivalRepaint(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	win := app.NewWindow("arrival")
	content := newRefreshCount()
	win.SetContent(content)
	state := sampleState()
	state.window = win

	armed := arrivalRepaintsArmed.Load()
	openSearchResult(state, Verse{BookName: "John", Chapter: 3, Verse: 16})
	if got := arrivalRepaintsArmed.Load() - armed; got != 1 {
		t.Fatalf("the arrival armed %d repaints through the suite's door, want 1", got)
	}
	before := content.n
	time.Sleep(2 * arrivalRepaintDelay)
	if content.n != before {
		t.Errorf("the window was repainted %d times after the arrival, on a timer the suite "+
			"should have held", content.n-before)
	}
}
