package bibletext

// A PHONE TURNED TO LANDSCAPE READS LIKE THE iPAD.
//
// The presentation half: rotating a phone to landscape on the Read tab enters
// the distraction-free tree (the reading pane alone, buildCompactUI) and
// rotating back restores the portrait layout together with whatever
// full-screen choice the reader had made, because the landscape half never
// writes IsFullScreen. Tablets are excluded by deviceIsTablet — the UIKit
// idiom on iOS, and on Android the live window's short side (600dp), so a
// tablet pane split or floated narrower than that reads as a phone and takes
// the presentation, which is the height problem the mode exists for — and
// keep their rail in landscape.
//
// The PAGE is not this file's business. A landscape phone reads the book page
// because its pane is wide enough for it (reading_page.go), by the one rule
// every surface uses; this mode used to carry a typography half of its own
// that turned the page on with the orientation, and the width rule retired it.
//
// The presentation is on by default on phones and reads from a preference, so
// a switch can turn it off (the dev Links tab carries one). The decision
// itself is one untagged rule over platform facts, the shape mobileRailWanted
// (layout.go) already has.

import "fyne.io/fyne/v2"

const prefPhoneLandscapeReading = "reading.phoneLandscape"

// phoneLandscapeReadingEnabled reports the presentation gate: on unless the
// preference turns it off. A dev build's launch seed outranks the preference
// in both directions.
func phoneLandscapeReadingEnabled() bool {
	if devPhoneLandscapeSeedOff() {
		return false
	}
	if devPhoneLandscapeSeedOn() {
		return true
	}
	if app := fyne.CurrentApp(); app != nil {
		return app.Preferences().BoolWithFallback(prefPhoneLandscapeReading, true)
	}
	return true
}

func setPhoneLandscapeReadingEnabled(v bool) {
	if app := fyne.CurrentApp(); app != nil {
		app.Preferences().SetBool(prefPhoneLandscapeReading, v)
	}
}

// phoneLandscapeReadingWanted is the decision with every input stated: a phone
// (not a tablet) on a platform that supports the mode, with the gate on and a
// canvas that is wider than tall. An unsized canvas — the first build, before
// Fyne's first layout — answers false: portrait is a phone's resting
// orientation, and the watcher asks again once the size lands. Landscape is
// `w >= h`, the same reading canvasIsLandscape and mobileRailWanted use.
func phoneLandscapeReadingWanted(phone, enabled bool, w, h float32) bool {
	return phone && enabled && w > 0 && h > 0 && w >= h
}

// phoneLandscapeReadingActive answers the decision for the live canvas. It
// reads the window rather than an AppState because reporterLayoutActive, one
// of its callers, has none.
func phoneLandscapeReadingActive() bool {
	if !phoneLandscapeReadingSupported() || deviceIsTablet() {
		return false
	}
	var w, h float32
	if app := fyne.CurrentApp(); app != nil {
		if wins := app.Driver().AllWindows(); len(wins) > 0 {
			sz := wins[0].Canvas().Size()
			w, h = sz.Width, sz.Height
		}
	}
	return phoneLandscapeReadingWanted(true, phoneLandscapeReadingEnabled(), w, h)
}

// phoneLandscapeReading is the seam over the decision, pinned by tests the way
// reporterLayout (reading.go) is.
var phoneLandscapeReading = phoneLandscapeReadingActive

// readingFullScreen is the PRESENTED distraction-free mode: the reader's own
// choice (IsFullScreen — the focus and exit buttons' bool, never persisted) or
// the phone-landscape presentation on the Read tab. It is a READING mode:
// Books and Search keep their ordinary landscape layout, so a rotation while
// browsing does not throw the reader into the text. The landscape half never
// writes IsFullScreen, which is what lets rotating back land on whatever the
// reader had chosen. Every reader of the mode asks this; only the buttons
// write the flag.
func (s *AppState) readingFullScreen() bool {
	return s != nil && (s.IsFullScreen || (s.CurrentTab == 0 && phoneLandscapeReading()))
}
