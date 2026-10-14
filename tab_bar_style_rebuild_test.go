package bibletext

// THE BAR'S DRESS IS PART OF WHAT THE WATCHER COMPARES.
//
// tabBarStyleFor picks the bottom bar's dress, its tabs spread across it or
// centred, from the width the window has when the bar is built, at 560
// points (tabBarSpreadMaxWidth). The watcher rebuilt only when the class, the
// rail or the phone's landscape presentation changed, so an iPad window, or
// an Android tablet's, resized across 560 points while it stayed taller than
// wide kept the dress it was built in until something else rebuilt it.
// renderedLayout now carries the dress, and only where a bar is drawn: a
// rail, and the full-screen tree that draws no navigation, rebuild for no
// width.
//
// The host draws its navigation by BIBLETEXT_DESKTOP_TABS, which stands in
// here for the shape rule a phone or a tablet applies; what is under test is
// the watcher's comparison.

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func TestATallWindowResizedAcrossTheBarsWidthRebuildsTheBar(t *testing.T) {
	test.NewTempApp(t)
	st := sampleState()
	w := test.NewWindow(nil)
	t.Cleanup(w.Close)
	st.window = w

	at := func(width float32) (renderedLayout, tabBarStyle) {
		w.Resize(fyne.NewSize(width, 900))
		return st.renderedLayout(width), tabBarStyleFor(st)
	}

	t.Setenv("BIBLETEXT_DESKTOP_TABS", "bar")
	wide, wideStyle := at(600)
	narrow, narrowStyle := at(500)
	if wideStyle == narrowStyle {
		t.Fatalf("control: the bar's dress is %v at 600 and at 500 points, so the test cannot see a change", wideStyle)
	}
	if !layoutWatcherNeedsRebuild(wide, narrow) || !layoutWatcherNeedsRebuild(narrow, wide) {
		t.Errorf("a window taller than wide resized across %v points keeps the bar it was built with (%v at 600, %v at 500)",
			tabBarSpreadMaxWidth, wideStyle, narrowStyle)
	}
	if a, _ := at(700); layoutWatcherNeedsRebuild(wide, a) {
		t.Error("a resize that stays above the bar's width rebuilds")
	}

	// No bar, no dress: the rail, and the full-screen tree.
	t.Setenv("BIBLETEXT_DESKTOP_TABS", "rail")
	railWide, _ := at(600)
	railNarrow, _ := at(500)
	if layoutWatcherNeedsRebuild(railWide, railNarrow) {
		t.Error("the rail is rebuilt for a width the bar's dress would change at; no bar is drawn")
	}
	t.Setenv("BIBLETEXT_DESKTOP_TABS", "bar")
	st.IsFullScreen = true
	fullWide, _ := at(600)
	fullNarrow, _ := at(500)
	if layoutWatcherNeedsRebuild(fullWide, fullNarrow) {
		t.Error("the full-screen tree is rebuilt for a width the bar's dress would change at; it draws no navigation")
	}
}
