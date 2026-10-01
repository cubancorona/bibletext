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
// uses it to keep the landscape presentation to phones (phone_landscape.go),
// for the reporter-width reading measure, and for the navigation's place
// before the canvas has a size (mobileRailWanted).
func deviceIsTablet() bool {
	return C.bibleTextIsPad() != 0
}

// The iPhone reads like the iPad in landscape (phone_landscape.go) — on the
// book page too, because a landscape iPhone's pane is wide enough for it
// (reading_page.go) — with a Go-side anchor captured before the rotation's
// frame lands, because the re-import under the new grammar would otherwise
// land the reader elsewhere.
func phoneLandscapeReadingSupported() bool { return true }

func rotationRestoreNeeded() bool { return true }
