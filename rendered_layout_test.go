package bibletext

// THE WATCHER'S DECISION CARRIES THE LANDSCAPE PRESENTATION AS ITS OWN TERM.
//
// A phone that had chosen full-screen in portrait still rebuilds when the
// landscape presentation flips (that rebuild re-reads the typography gate and
// pushes the measure), while an iPad in chosen full-screen never rebuilds on
// rotation (its landscape term is constant and its rail term is zeroed).

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func TestRenderedLayoutFollowsThePhoneLandscapePresentation(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	st := sampleState()
	w := app.NewWindow("rotate")
	defer w.Close()
	st.window = w

	// The decision, read from the live canvas the way the phone would read
	// it; the host is not a phone, so the seam stands in for the policy.
	orig := phoneLandscapeReading
	phoneLandscapeReading = func() bool {
		sz := w.Canvas().Size()
		return sz.Width > 0 && sz.Width >= sz.Height
	}
	defer func() { phoneLandscapeReading = orig }()

	for _, chosen := range []bool{false, true} {
		st.IsFullScreen = chosen
		w.Resize(fyne.NewSize(402, 874))
		portrait := st.renderedLayout(402)
		w.Resize(fyne.NewSize(874, 402))
		landscape := st.renderedLayout(874)
		if portrait.landscape || !landscape.landscape {
			t.Fatalf("chosen=%v: landscape term portrait=%v landscape=%v", chosen, portrait.landscape, landscape.landscape)
		}
		if landscape.rail {
			t.Fatalf("chosen=%v: the full-screen tree reports a rail", chosen)
		}
		if !layoutWatcherNeedsRebuild(portrait, landscape) || !layoutWatcherNeedsRebuild(landscape, portrait) {
			t.Fatalf("chosen=%v: a rotation into or out of the presentation does not rebuild", chosen)
		}
	}

	// An iPad in chosen full-screen: the presentation never applies, the
	// rail is zeroed, so the two orientations render the same.
	phoneLandscapeReading = func() bool { return false }
	st.IsFullScreen = true
	w.Resize(fyne.NewSize(1024, 1366))
	a := st.renderedLayout(1024)
	w.Resize(fyne.NewSize(1366, 1024))
	b := st.renderedLayout(1366)
	if layoutWatcherNeedsRebuild(a, b) {
		t.Fatalf("chosen full-screen off a phone rebuilt on rotation: %+v -> %+v", a, b)
	}
}
