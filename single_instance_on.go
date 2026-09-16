//go:build windows || linux || bibletextdev

package bibletext

// The desktops that need a single instance, plus dev builds so the handoff can
// be rehearsed on a Mac under BIBLETEXT_MIMIC. A darwin release build takes
// single_instance_off.go: LaunchServices already single-instances it, and the
// Mac App Store sandbox (appstore/mac/BibleText.entitlements) forbids a
// listener. TestReleaseScriptsNeverPassTheDevTag keeps the tag out of every
// release script and workflow, and TestSingleInstanceAbsentFromDarwinReleaseBuilds
// holds the gate itself.

import (
	"errors"
	"sync/atomic"

	"fyne.io/fyne/v2"
)

var singleInstanceActive atomic.Bool

// forwardToRunningInstance hands the startup link (or a bare raise) to a
// primary that is already running. true means this process should exit. The
// fast path, before the toolkit's app exists; claimSingleInstance repeats it
// for the race it cannot see.
func forwardToRunningInstance(startup string) bool {
	path, err := singleInstanceRecordPath()
	if err != nil {
		return false
	}
	return forwardShareLink(path, startup, allowSetForeground)
}

// claimSingleInstance makes this process the primary, or forwards to the one
// that became primary in the meantime. Called as soon as the app and the
// loading state exist — before the window, so the window of two primaries is
// the microseconds between a failed forward and an exclusive create, which
// the create itself closes: losing it means forwarding. The listener's
// callback is queued with fyne.Do, so a link that lands before the window
// exists parks (loadPhase is loadPending) and raises once the loop runs.
// The returned stop is deferred by Run; forwarded means exit now.
func claimSingleInstance(state *AppState, startup string) (stop func(), forwarded bool) {
	path, err := singleInstanceRecordPath()
	if err != nil {
		return func() {}, false
	}
	deliver := func(raw string) {
		fyne.Do(func() { singleInstanceDeliver(state, raw) })
	}
	for attempt := 0; attempt < 2; attempt++ {
		if forwardShareLink(path, startup, allowSetForeground) {
			return nil, true
		}
		stop, err := serveShareLinks(path, deliver)
		if err == nil {
			singleInstanceActive.Store(true)
			return func() {
				singleInstanceActive.Store(false)
				stop()
			}, false
		}
		if !errors.Is(err, errRecordExists) {
			return func() {}, false // cannot listen: run anyway, alone
		}
		// Lost the race: someone created the record between the forward and
		// the create. Round again: forward to them.
	}
	return func() {}, false
}

func singleInstanceListening() bool { return singleInstanceActive.Load() }
