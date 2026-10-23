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

// BTWindowReport is the app's window as its window scene describes it, in
// points: the scene's size, and the left and right safe-area insets of the
// window the app's controller is in. ready is 0 until an application window
// scene with that window is connected; the numbers mean nothing until then.
typedef struct {
    int ready;
    double width, height;
    double insetLeft, insetRight;
} BTWindowReport;

// bibleTextWindowReport reads the app's window scene. UIKit's scene objects
// belong to the main thread, so the read runs there and the caller waits for
// it. The caller is a goroutine of its own (windowReportRead,
// launch_window.go), never the Fyne goroutine: during launch the main thread
// is busy with UIKit's own work for a few hundred milliseconds, and the Fyne
// goroutine builds the first frame meanwhile. Already on the main thread, the
// read runs inline, since a dispatch_sync to the queue it is running on would
// never return. Before UIApplicationMain there is no application, and no
// scene, and the report is not ready.
static BTWindowReport bibleTextWindowReport(void) {
    __block BTWindowReport out = {0, 0, 0, 0, 0};
    dispatch_block_t read = ^{
        UIApplication *app = [UIApplication sharedApplication];
        if (app == nil) {
            return;
        }
        for (UIScene *scene in app.connectedScenes) {
            if (![scene isKindOfClass:[UIWindowScene class]] ||
                ![scene.session.role isEqualToString:UIWindowSceneSessionRoleApplication]) {
                continue;
            }
            UIWindowScene *windowScene = (UIWindowScene *)scene;
            // The scene's own coordinateSpace is deprecated from iOS 26 for
            // the one its effective geometry has from then. Its bounds are the
            // window's size, in the orientation the interface has, and for a
            // Split View or Stage Manager window the window's own.
            CGRect bounds;
            if (@available(iOS 26.0, *)) {
                bounds = windowScene.effectiveGeometry.coordinateSpace.bounds;
            } else {
                bounds = windowScene.coordinateSpace.bounds;
            }
            if (bounds.size.width <= 0 || bounds.size.height <= 0) {
                return;
            }
            for (UIWindow *window in windowScene.windows) {
                if (window.rootViewController == nil) {
                    continue;
                }
                out.width = bounds.size.width;
                out.height = bounds.size.height;
                out.insetLeft = window.safeAreaInsets.left;
                out.insetRight = window.safeAreaInsets.right;
                out.ready = 1;
                return;
            }
            return;
        }
    };
    if ([NSThread isMainThread]) {
        read();
    } else {
        dispatch_sync(dispatch_get_main_queue(), read);
    }
    return out;
}
*/
import "C"

// deviceIsTablet reports whether we're running on an iPad. The shared layout
// uses it to keep the landscape presentation to phones (phone_landscape.go),
// for the reporter-width reading measure, and for the navigation's place
// before the canvas has a size while the window scene has not been read
// (mobileRailWanted).
func deviceIsTablet() bool {
	return C.bibleTextIsPad() != 0
}

// The window scene is read once, on a goroutine of its own, and kept for the
// launch (keptWindowReport, launch_window.go): nothing on the Fyne goroutine
// waits for the main thread.
func init() {
	windowReportRead = func() (windowReport, bool) {
		r := C.bibleTextWindowReport()
		if r.ready == 0 {
			return windowReport{}, false
		}
		return windowReport{
			w:     float32(r.width),
			h:     float32(r.height),
			sides: float32(r.insetLeft + r.insetRight),
		}, true
	}
}

// launchWindowSize is the window's size, in points, before the Fyne canvas
// has one: the window scene's, once the read has landed (layoutSize,
// launch_window.go). An iPhone opened while held sideways then lays out
// sideways from its first frame, and an iPad opened upright, or in a Split
// View or Stage Manager window taller than wide, draws the bar from its first
// frame rather than the rail. Until the read lands it reports nothing, and the
// device answers.
func launchWindowSize() (w, h float32, ok bool) {
	r, ok := keptWindowReport()
	return r.w, r.h, ok
}

// launchPaneWidth is the reading pane's width, in points, before the canvas
// has a size: the width the window lays the app's tree in, its scene's width
// less the side safe-area insets. It is the reading page's estimate for a
// chapter pushed then (readingColdWidth, reading_page_width.go), so a window
// opens on the page its pane reads: an iPhone held sideways on the book page
// of its full-screen pane, rather than the phone page followed by a second
// import once the pane reports. 0 until the read lands, when the device's
// resting page answers (readingUnsizedPage, reporter_ios.go).
func launchPaneWidth() float64 {
	r, ok := keptWindowReport()
	if !ok {
		return 0
	}
	return r.paneWidth()
}

// The iPhone reads like the iPad in landscape (phone_landscape.go) — on the
// book page too, because a landscape iPhone's pane is wide enough for it
// (reading_page.go) — with a Go-side anchor captured before the rotation's
// frame lands, because the re-import under the new grammar would otherwise
// land the reader elsewhere.
func phoneLandscapeReadingSupported() bool { return true }

func rotationRestoreNeeded() bool { return true }
