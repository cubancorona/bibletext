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

// iPhone keeps its bottom navigation in landscape; only iPad uses the rail.
func phoneLandscapeNavRail() bool { return false }

// layoutMayChange gates the orientation watcher. On iOS the idiom is static and
// known at launch, so only iPads need watching for bar/rail rotation.
func layoutMayChange() bool { return deviceIsTablet() }
