package bibletext

// The size to ask for at launch, and why asking for the wrong one is not a
// cosmetic matter.
//
// Fyne's Window.Resize sets the canvas size SYNCHRONOUSLY, before any window
// exists (internal/driver/glfw/window.go: `w.canvas.size = size`), and when the
// window is finally created it calls processResized with the size that was
// REQUESTED rather than the size the desktop granted. So a request the desktop
// cannot honour leaves the canvas holding a size nothing on screen has: the
// content is laid out for that phantom canvas and the window shows its
// top-left corner. On a 1024x768 screen asking for 1280x860 puts a dead gutter
// down the left of the reading pane and runs the text off the right edge, and
// it stays that way until some later resize happens to correct it.
//
// docs/windows-store-smoke-john3.jpg is that failure, captured on a runner and
// committed without anyone noticing. So the rule here is simply: never ask for
// a window the desktop cannot give.
//
// This file holds the arithmetic and nothing else -- the work area arrives as a
// parameter, so the decision is testable on a host with any screen at all, or
// none. Measuring the work area is platform work and lives in
// window_workarea_desktop.go.

import (
	"math"

	"fyne.io/fyne/v2"
)

// The window to ask for when there is room for it.
const (
	preferredWindowWidth  float32 = 1280
	preferredWindowHeight float32 = 860
)

// The desktop furniture around the canvas. Fyne sizes the CLIENT area; the
// title bar and resize borders sit outside it, so a client area exactly the
// size of the work area still overhangs by its own caption. These are
// allowances rather than measurements, because the frame cannot be measured
// before the window it belongs to exists. The height covers a Windows 11
// caption plus borders and a macOS title bar; the width covers two borders.
const (
	startupWindowFrameWidth  float32 = 4
	startupWindowFrameHeight float32 = 40
)

// startupWindowSize is the size to ask for: the preferred window, reduced to
// what the work area can hold once its frame is allowed for.
//
// A zero or negative work area means no answer was available -- no display, a
// sleeping screen, a headless runner, GLFW declining to start. There is then
// nothing to clamp against, so it asks for the preferred size and behaves
// exactly as it did before this existed.
func startupWindowSize(work fyne.Size) fyne.Size {
	want := fyne.NewSize(preferredWindowWidth, preferredWindowHeight)
	if work.Width <= 0 || work.Height <= 0 {
		return want
	}
	if maxW := work.Width - startupWindowFrameWidth; maxW > 0 && want.Width > maxW {
		want.Width = maxW
	}
	if maxH := work.Height - startupWindowFrameHeight; maxH > 0 && want.Height > maxH {
		want.Height = maxH
	}
	return want
}

// fyneScale reproduces the scale Fyne will multiply this request back by.
//
// This is the whole correctness argument for clamping in points, so it is
// written out rather than guessed. Fyne computes
// calculateScale(userScale, SystemScaleForWindow, detectScale) as
// round(system*user*10)/10 (internal/driver/glfw/scale.go), and
// SystemScaleForWindow is per-platform (internal/driver/glfw/device_desktop.go):
// a hard 1.0 on macOS, because macOS scaling happens at the texture level, and
// the monitor's content scale on Windows.
//
// A wrong answer here is not symmetric. Too HIGH and the point budget is too
// small, so the window opens smaller than it needed to -- untidy. Too LOW and
// the budget is too generous and the defect comes back. Callers that are unsure
// should therefore err high.
func fyneScale(systemScale, userScale float32) float32 {
	if userScale <= 0 {
		userScale = 1.0
	}
	if systemScale <= 0 {
		systemScale = 1.0
	}
	raw := float64(systemScale * userScale)
	scaled := float32(math.Round(raw*10.0) / 10.0)
	if scaled <= 0 {
		return 1.0
	}
	return scaled
}

// workAreaPoints converts a work area measured in the screen coordinates GLFW
// reports into the Fyne points that Window.Resize expects.
//
// The two are not the same thing on every platform, and that is the trap. On
// macOS GLFW screen coordinates are already points and Fyne's system scale is
// 1.0, so the pair agree and nothing is divided. On Windows, GLFW runs
// per-monitor DPI aware so its screen coordinates are physical pixels, and
// Fyne multiplies points by the monitor's content scale -- so the work area has
// to be divided by that same scale before it can be compared with a request in
// points. Dividing by the wrong one silently produces a request in the wrong
// unit, which is how a clamp can appear to work and still overflow.
func workAreaPoints(screenWidth, screenHeight int, systemScale, userScale float32) fyne.Size {
	if screenWidth <= 0 || screenHeight <= 0 {
		return fyne.Size{}
	}
	s := fyneScale(systemScale, userScale)
	return fyne.NewSize(float32(screenWidth)/s, float32(screenHeight)/s)
}
