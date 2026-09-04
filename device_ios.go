//go:build ios

package bibletext

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework UIKit

#import <UIKit/UIKit.h>

// bibleTextIsPad reports whether the current device's interface idiom is iPad.
// The idiom is fixed for the process and available immediately at launch, unlike
// the Fyne canvas size (0 until the first layout pass) — so the very first UI
// build can apply tablet presentation (rail/measure) with no phone flash.
static int bibleTextIsPad(void) {
    return (int)([[UIDevice currentDevice] userInterfaceIdiom] == UIUserInterfaceIdiomPad);
}
*/
import "C"

// deviceIsTablet reports whether we're running on an iPad. The shared layout
// uses it for the landscape rail and reporter-width reading measure.
func deviceIsTablet() bool {
	return C.bibleTextIsPad() != 0
}

// iPhone keeps its bottom navigation in landscape where navigation is drawn
// (Books and Search; the Read tab reads full-screen there by default,
// phone_landscape.go); only iPad uses the rail.
func phoneLandscapeNavRail() bool { return false }

// The iPhone reads like the iPad in landscape (phone_landscape.go), with the
// reporter typography — the Apple HTML dialect sets it — and a Go-side anchor
// captured before the rotation's frame lands, because the re-import under the
// new grammar would otherwise land the reader elsewhere.
func phoneLandscapeReadingSupported() bool { return true }

var phoneLandscapeTypographySupported = func() bool { return true }

func rotationRestoreNeeded() bool { return true }

// layoutMayChange gates the orientation watcher. On iOS the idiom is static and
// known at launch, so iPads need watching for bar/rail rotation — and every
// iPhone for the landscape presentation's flip, unless the landscape reading
// preference (phone_landscape.go) has turned the mode off.
func layoutMayChange() bool { return deviceIsTablet() || phoneLandscapeReadingEnabled() }
