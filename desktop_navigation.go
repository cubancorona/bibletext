//go:build !ios && !android

package bibletext

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
)

const desktopNavigationSettle = 250 * time.Millisecond

// desktopNavigationAfter schedules the settled resize. The production schedule
// is scheduleDesktopNavigationSettle; tests hold the variable with a clock of
// their own.
var desktopNavigationAfter = scheduleDesktopNavigationSettle

// scheduleDesktopNavigationSettle runs apply on the UI goroutine once delay has
// passed, and returns a cancellation. Stopping the timer does not retract a
// callback it has already queued for that goroutine, which is why the caller
// pairs the cancellation with a generation check.
func scheduleDesktopNavigationSettle(delay time.Duration, apply func()) func() {
	timer := time.AfterFunc(delay, func() { fyne.Do(apply) })
	return func() { timer.Stop() }
}

// desktopNavigation keeps the active page and overlays intact while moving only
// navigation chrome. Rebuilding the window would discard an in-flight answer,
// entry focus and native selection even though the selected page has not changed.
type desktopNavigation struct {
	state           *AppState
	root            *fyne.Container
	body            *fyne.Container
	header, content fyne.CanvasObject
	built           renderedLayout
	size            fyne.Size
	generation      uint64
	cancel          func()
}

func watchCompactNavigation(state *AppState, body *fyne.Container, header, content fyne.CanvasObject) fyne.CanvasObject {
	if desktopNav() != desktopNavAdaptive || state == nil || state.window == nil {
		return body
	}
	w := &desktopNavigation{
		state: state, body: body, header: header, content: content,
		built: state.renderedLayout(state.canvasWidth()),
		size:  state.window.Canvas().Size(),
	}
	w.root = container.New(w, body)
	return w.root
}

func (w *desktopNavigation) MinSize([]fyne.CanvasObject) fyne.Size {
	return w.body.MinSize()
}

func (w *desktopNavigation) Layout(_ []fyne.CanvasObject, size fyne.Size) {
	w.body.Resize(size)
	w.body.Move(fyne.Position{})
	// The full canvas decides placement; a child can be shorter because a
	// sheet or an interactive-area inset occupies part of the window.
	canvasSize := w.state.window.Canvas().Size()
	if canvasSize == w.size {
		return
	}
	w.size = canvasSize
	w.generation++
	if w.cancel != nil {
		w.cancel()
		w.cancel = nil
	}
	if !canvasSized(canvasSize.Width, canvasSize.Height) ||
		!layoutWatcherNeedsRebuild(w.built, w.state.renderedLayout(canvasSize.Width)) {
		return
	}
	generation := w.generation
	w.cancel = desktopNavigationAfter(desktopNavigationSettle, func() {
		if generation != w.generation || desktopNav() != desktopNavAdaptive ||
			!w.attached() {
			return
		}
		w.cancel = nil
		want := w.state.renderedLayout(w.state.canvasWidth())
		if !layoutWatcherNeedsRebuild(w.built, want) {
			return
		}
		body := compactNavigationBody(w.state, w.header, w.content)
		w.body.Layout, w.body.Objects = body.Layout, body.Objects
		w.built = want
		w.body.Refresh()
	})
}

func (w *desktopNavigation) attached() bool {
	if w.state.app == nil {
		return false
	}
	for _, window := range w.state.app.Driver().AllWindows() {
		if window == w.state.window {
			return desktopNavigationAttached(window.Canvas().Content(), w)
		}
	}
	return false
}

// Only the installed tree may apply a delayed resize. A tab switch, appearance
// rebuild or full-screen transition can replace the tree before its timer lands.
func desktopNavigationAttached(root fyne.CanvasObject, target *desktopNavigation) bool {
	if root == target.root {
		return true
	}
	if box, ok := root.(*fyne.Container); ok {
		for _, child := range box.Objects {
			if desktopNavigationAttached(child, target) {
				return true
			}
		}
	}
	return false
}
