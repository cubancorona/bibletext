//go:build !ios && !android

package bibletext

// Measuring the desktop's usable area, which Fyne does not expose.
//
// There is no screen accessor anywhere in the public API: fyne.Window has no
// Size, fyne.Driver has no monitors, and fyne.Settings offers only Scale. The
// toolkit's own monitor calls are in internal/ and unimportable, and they ask
// for the full video mode -- taskbar included -- which is the wrong number for
// deciding how big a window may be.
//
// One layer down, GLFW has it. GLFW is what Fyne's desktop driver is built on;
// it is already compiled and linked into this binary, so this adds no new C
// library, only a direct import of a module that was already an indirect one.
// Monitor.GetWorkarea is the number the toolkit never asks for.
//
// Everything here is best effort. A machine with no display, a sleeping
// screen, a headless runner or a GLFW that declines to start all return a zero
// size, and startupWindowSize treats that as "ask for the preferred window and
// behave as before". Nothing about the app's startup depends on this
// succeeding.

import (
	"os"
	"runtime"
	"strconv"

	"fyne.io/fyne/v2"
	"github.com/go-gl/glfw/v3.3/glfw"
)

// startupWorkArea is the primary monitor's usable area in Fyne points, or a
// zero size if it cannot be determined.
//
// Safe to call before the Fyne driver starts. glfw.Init is idempotent -- it
// returns immediately when GLFW is already initialised -- and Fyne's own
// initGLFW sets no init hints beforehand, so there is nothing an early call
// can discard. It deliberately does NOT call glfw.Terminate: Fyne initialises
// GLFW again moments later and tears it down itself on exit.
//
// GLFW requires its calls on the main thread. The toolkit's glfw driver locks
// the main OS thread in a package init, and Run is called from main, so that
// holds by the time this runs.
func startupWorkArea(a fyne.App) (size fyne.Size) {
	// A cosmetic improvement must never be the reason the app fails to start.
	defer func() {
		if r := recover(); r != nil {
			size = fyne.Size{}
		}
	}()

	if err := glfw.Init(); err != nil {
		return fyne.Size{}
	}
	monitor := glfw.GetPrimaryMonitor()
	if monitor == nil {
		return fyne.Size{}
	}
	_, _, w, h := monitor.GetWorkarea()
	if w <= 0 || h <= 0 {
		// Observed for real, not hypothetically: while this machine's display
		// was asleep, GetMonitors returned nothing and the work area came back
		// as zero. "No answer" is a live runtime state.
		return fyne.Size{}
	}
	return workAreaPoints(w, h, startupSystemScale(monitor), startupUserScale(a))
}

// startupSystemScale mirrors the toolkit's SystemScaleForWindow
// (internal/driver/glfw/device_desktop.go) from a monitor rather than a window,
// because the window does not exist yet.
//
// The platform split is the toolkit's, not a choice made here: macOS returns a
// hard 1.0 because its scaling happens at the texture level, and Windows
// returns the monitor's content scale.
//
// KNOWN LIMIT, and it is why this errs high rather than low. Fyne takes the
// scale of the monitor the window actually lands on; this takes the primary
// monitor's, because Win32 has not placed the window yet. On a mixed-DPI
// multi-monitor desktop the two can differ. Guessing high costs a window
// slightly smaller than it needed to be; guessing low brings the defect back.
// Linux is the case this handles least exactly -- the toolkit derives its scale
// from the monitor's physical size and video mode, and content scale is used
// here as a close proxy rather than duplicating that heuristic.
func startupSystemScale(m *glfw.Monitor) float32 {
	if runtime.GOOS == "darwin" {
		return 1.0
	}
	x, y := m.GetContentScale()
	if y > x {
		x = y // err high: a smaller point budget is the safe direction
	}
	if x <= 0 {
		return 1.0
	}
	return x
}

// startupUserScale mirrors the toolkit's userScale (internal/driver/glfw/scale.go)
// exactly, including the special meaning of FYNE_SCALE=auto, so that a reader
// who has set a scale gets a window clamped with the same number the toolkit
// will multiply back by.
func startupUserScale(a fyne.App) float32 {
	env := os.Getenv("FYNE_SCALE")
	if env != "" && env != "auto" {
		if s, err := strconv.ParseFloat(env, 32); err == nil && s != 0 {
			return float32(s)
		}
	}
	if env != "auto" && a != nil {
		if setting := a.Settings().Scale(); setting > 0 {
			return setting
		}
	}
	return 1.0
}
