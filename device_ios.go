//go:build ios

package bibletext

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework UIKit

#import <UIKit/UIKit.h>

// bibleTextIsPad reports whether the current device's interface idiom is iPad.
// The idiom is fixed for the process and available immediately at launch, unlike
// the Fyne canvas size (0 until the first layout pass) — so the very first UI
// build can already choose the tablet layout with no phone-layout flash.
static int bibleTextIsPad(void) {
    return (int)([[UIDevice currentDevice] userInterfaceIdiom] == UIUserInterfaceIdiomPad);
}
*/
import "C"

// deviceIsTablet reports whether we're running on an iPad. See classifyLayout
// (layout.go) for how it combines with the live canvas width to pick between the
// compact (phone) and regular (sidebar+split) layouts.
func deviceIsTablet() bool {
	return C.bibleTextIsPad() != 0
}

// layoutMayChange gates installing the layoutWatcher. On iOS the idiom is
// static and known at launch, so only actual tablets need watching.
func layoutMayChange() bool { return deviceIsTablet() }
