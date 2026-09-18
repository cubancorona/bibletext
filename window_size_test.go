package bibletext

// These hold the launch-size rule in place. The defect they exist for is in
// window_size.go's header: Fyne keeps the size that was REQUESTED rather than
// the one the desktop granted, so a request that cannot be honoured lays the
// content out for a canvas nothing on screen has.
//
// The end-to-end proof is a screenshot check -- scripts/check-reading-centred.py,
// run against a real capture of the failure in .github/workflows/msstore.yml.
// What these tests add is the arithmetic that check cannot see: that the size
// asked for fits, and that the points-to-pixels conversion uses the same scale
// Fyne multiplies back by. That conversion is the whole correctness argument
// for clamping in points and it had no test at all in the original design.

import (
	"math"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
)

func closeEnough(a, b float32) bool { return math.Abs(float64(a-b)) < 0.05 }

func TestStartupWindowSizeAsksForSomethingThatFits(t *testing.T) {
	// Real work areas, including the ones that made this bug invisible and the
	// ones where it bites. A work area is the desktop minus the taskbar or
	// menu bar, in points.
	cases := []struct {
		name       string
		work       fyne.Size
		wantW      float32
		wantH      float32
		whyItBites string
	}{
		{
			name: "a large desktop grants the preferred size untouched",
			work: fyne.NewSize(1920, 1040), wantW: 1280, wantH: 860,
		},
		{
			name: "this development Mac clears it, which is why nobody saw this",
			work: fyne.NewSize(1728, 1022), wantW: 1280, wantH: 860,
			whyItBites: "measured with GLFW on the machine: videomode 1728x1117, work area 1728x1022 at y=33",
		},
		{
			name: "a display that clears the width but not the height",
			work: fyne.NewSize(1372, 854), wantW: 1280, wantH: 814,
			whyItBites: "only the height is clamped; the milder half of the defect",
		},
		{
			name: "the CI runner, where the committed failure was captured",
			work: fyne.NewSize(1024, 728), wantW: 1020, wantH: 688,
			whyItBites: "1024x768 minus the taskbar",
		},
		{
			name: "the Windows guest that reproduces it by hand",
			work: fyne.NewSize(800, 552), wantW: 796, wantH: 512,
			whyItBites: "800x600 minus the taskbar",
		},
		{
			name: "a 1366x768 laptop at 125% scaling, an ordinary machine",
			work: fyne.NewSize(1093, 566), wantW: 1089, wantH: 526,
			whyItBites: "1093x614 logical minus the taskbar; width is well under 1280",
		},
		{
			name: "1920x1080 at 150%, short only in height",
			work: fyne.NewSize(1280, 672), wantW: 1276, wantH: 632,
			whyItBites: "1280x720 logical; width exactly clears, height does not",
		},
	}

	for _, c := range cases {
		got := startupWindowSize(c.work)
		if !closeEnough(got.Width, c.wantW) || !closeEnough(got.Height, c.wantH) {
			t.Errorf("%s: startupWindowSize(%v) = %v, want %vx%v (%s)",
				c.name, c.work, got, c.wantW, c.wantH, c.whyItBites)
		}
		// The point of the whole exercise: whatever it returns must fit, frame
		// included. An assertion on the numbers above could be satisfied by a
		// table that happens to match; this cannot.
		if got.Width+startupWindowFrameWidth > c.work.Width+0.05 {
			t.Errorf("%s: asked for %vpt wide in a %vpt work area -- this is the defect, not a fix",
				c.name, got.Width, c.work.Width)
		}
		if got.Height+startupWindowFrameHeight > c.work.Height+0.05 {
			t.Errorf("%s: asked for %vpt tall in a %vpt work area -- this is the defect, not a fix",
				c.name, got.Height, c.work.Height)
		}
	}
}

// A work area nobody could measure must not become a tiny window. No answer
// means behave exactly as the app did before the clamp existed.
func TestAnUnknownWorkAreaAsksForThePreferredSize(t *testing.T) {
	for _, work := range []fyne.Size{
		{}, // no display, a sleeping screen, a headless runner
		fyne.NewSize(0, 900),
		fyne.NewSize(1200, 0),
		fyne.NewSize(-1, -1),
	} {
		got := startupWindowSize(work)
		if !closeEnough(got.Width, preferredWindowWidth) || !closeEnough(got.Height, preferredWindowHeight) {
			t.Errorf("startupWindowSize(%v) = %v; an unmeasurable work area must fall back to the preferred %vx%v, not shrink",
				work, got, preferredWindowWidth, preferredWindowHeight)
		}
	}
}

// Swept rather than tabulated, so a rule that only fits the sizes someone
// thought of is caught. This is the property the pixel check cannot express.
func TestNoWorkAreaProducesAWindowThatOverflowsIt(t *testing.T) {
	for w := 320; w <= 3840; w += 37 {
		for h := 240; h <= 2160; h += 41 {
			work := fyne.NewSize(float32(w), float32(h))
			got := startupWindowSize(work)
			if got.Width+startupWindowFrameWidth > work.Width+0.05 {
				t.Fatalf("work %v: asked %vpt wide, overflows by %vpt",
					work, got.Width, got.Width+startupWindowFrameWidth-work.Width)
			}
			if got.Height+startupWindowFrameHeight > work.Height+0.05 {
				t.Fatalf("work %v: asked %vpt tall, overflows by %vpt",
					work, got.Height, got.Height+startupWindowFrameHeight-work.Height)
			}
			// Never larger than asked for either: the clamp reduces, it does
			// not invent a bigger window on a big screen.
			if got.Width > preferredWindowWidth+0.05 || got.Height > preferredWindowHeight+0.05 {
				t.Fatalf("work %v: returned %v, larger than the preferred %vx%v",
					work, got, preferredWindowWidth, preferredWindowHeight)
			}
		}
	}
}

// The scale term. The original design had no test with a scale in it at all,
// which would have let a regression here pass silently on exactly the 125% and
// 150% machines the fix exists for.
func TestWorkAreaPointsUsesTheScaleFyneWillMultiplyBackBy(t *testing.T) {
	cases := []struct {
		name         string
		pxW, pxH     int
		system, user float32
		wantW, wantH float32
	}{
		{
			name: "macOS: GLFW screen coordinates are already points and Fyne's system scale is 1.0",
			pxW:  1372, pxH: 854, system: 1.0, user: 1.0, wantW: 1372, wantH: 854,
		},
		{
			name: "Windows 1920x1080 at 150%: pixels divided by the content scale",
			pxW:  1920, pxH: 1080, system: 1.5, user: 1.0, wantW: 1280, wantH: 720,
		},
		{
			// NOT 1366/1.25. Fyne rounds the scale to one decimal and Go rounds
			// a half away from zero, so 1.25 becomes 1.3 and the toolkit will
			// multiply points back by 1.3. Dividing by 1.25 here would leave the
			// budget about 4% too generous -- and 4% of overflow is the whole
			// defect, just smaller.
			name: "Windows 1366x768 at 125%, which Fyne rounds to a scale of 1.3",
			pxW:  1366, pxH: 768, system: 1.25, user: 1.0, wantW: 1050.77, wantH: 590.77,
		},
		{
			name: "Windows at 100%: nothing to divide",
			pxW:  1024, pxH: 768, system: 1.0, user: 1.0, wantW: 1024, wantH: 768,
		},
		{
			name: "a user scale multiplies in, and Fyne rounds the product to one decimal",
			pxW:  1920, pxH: 1080, system: 1.25, user: 1.2, wantW: 1280, wantH: 720, // round(1.5*10)/10 = 1.5
		},
		{
			name: "a nonsense scale is treated as 1.0 rather than dividing by zero",
			pxW:  1024, pxH: 768, system: 0, user: 0, wantW: 1024, wantH: 768,
		},
	}
	for _, c := range cases {
		got := workAreaPoints(c.pxW, c.pxH, c.system, c.user)
		if !closeEnough(got.Width, c.wantW) || !closeEnough(got.Height, c.wantH) {
			t.Errorf("%s: workAreaPoints(%d, %d, system=%v, user=%v) = %v, want %vx%v",
				c.name, c.pxW, c.pxH, c.system, c.user, got, c.wantW, c.wantH)
		}
	}

	// An unmeasurable screen must produce an unmeasurable work area, so that
	// startupWindowSize falls back rather than clamping to nearly nothing.
	if got := workAreaPoints(0, 0, 1, 1); got.Width != 0 || got.Height != 0 {
		t.Errorf("workAreaPoints(0,0,...) = %v, want a zero size so the caller falls back", got)
	}
}

// The rounding is load-bearing and surprising enough to pin on its own. Fyne
// does round(system*user*10)/10, and Go rounds a half away from zero, so the
// two commonest Windows scalings do not divide by the number on the tin.
func TestFyneScaleRoundsToOneDecimalTheWayTheToolkitDoes(t *testing.T) {
	cases := []struct {
		system, user, want float32
		note               string
	}{
		{1.0, 1.0, 1.0, "100%"},
		{1.25, 1.0, 1.3, "125% rounds UP to 1.3, not down to 1.2 and not 1.25"},
		{1.5, 1.0, 1.5, "150% is exact"},
		{1.75, 1.0, 1.8, "175% rounds to 1.8"},
		{2.0, 1.0, 2.0, "200% is exact"},
		{1.25, 1.2, 1.5, "a user scale multiplies in before the rounding"},
		{1.0, 0, 1.0, "a zero user scale is treated as 1.0"},
		{0, 1.0, 1.0, "a zero system scale is treated as 1.0"},
		{-2, -2, 1.0, "nonsense never yields a zero or negative divisor"},
	}
	for _, c := range cases {
		got := fyneScale(c.system, c.user)
		if !closeEnough(got, c.want) {
			t.Errorf("fyneScale(%v, %v) = %v, want %v (%s)", c.system, c.user, got, c.want, c.note)
		}
		if got <= 0 {
			t.Fatalf("fyneScale(%v, %v) returned %v; dividing a work area by this would be a disaster",
				c.system, c.user, got)
		}
	}
}

// The composition is what actually runs, and it is where a unit mix-up would
// show: a Windows machine at 150% whose work area is 1920x1032 PIXELS must end
// up asking for a window that fits 1280x688 POINTS.
func TestPixelsThroughToTheRequestedWindow(t *testing.T) {
	work := workAreaPoints(1920, 1032, 1.5, 1.0) // -> 1280x688 points
	if !closeEnough(work.Width, 1280) || !closeEnough(work.Height, 688) {
		t.Fatalf("work area came out as %v, want 1280x688 points", work)
	}
	got := startupWindowSize(work)
	if got.Width+startupWindowFrameWidth > work.Width+0.05 ||
		got.Height+startupWindowFrameHeight > work.Height+0.05 {
		t.Errorf("asked for %v in a %v work area: the clamp did not survive the unit conversion", got, work)
	}
	if !closeEnough(got.Height, 648) {
		t.Errorf("height came out %v, want 648 (688 minus the frame allowance)", got.Height)
	}
}

// Cheap insurance, and honest about what it is: not a behavioural check, just a
// guard that the unconditional request does not come back. It is the line that
// caused this, and it read as obviously harmless.
func TestRunDoesNotAskForAFixedWindowSizeAnyMore(t *testing.T) {
	body := readRepoFile(t, "app.go")
	if strings.Contains(body, "window.Resize(fyne.NewSize(1280, 860))") {
		t.Error("app.go still calls window.Resize(fyne.NewSize(1280, 860)) unconditionally; " +
			"on any desktop smaller than that the content is laid out for a window that does not exist")
	}
	if !strings.Contains(body, "startupWindowSize(") {
		t.Error("app.go no longer asks startupWindowSize for its launch size, so nothing clamps the request")
	}
}
