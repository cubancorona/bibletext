//go:build ios

package bibletext

// Receiving a Universal Link on iOS.
//
// When someone taps https://bibletext.co.uk/web/john/3/#v16 on an iPhone that
// has BibleText installed, iOS hands the URL to the app delegate rather than
// opening Safari — but only if the app's entitlement claims the domain
// (release-ios.sh pins applinks:bibletext.co.uk) AND the association file we
// publish at /.well-known/apple-app-site-association allow-lists that path.
//
// THE PROBLEM: the app delegate belongs to Fyne. Its mobile driver installs
// GoAppAppDelegate and implements none of the link callbacks, and Fyne exposes
// no Go-level hook for an incoming URL. Forking Fyne for this would be absurd.
//
// THE FIX: an Objective-C CATEGORY on GoAppAppDelegate, compiled into the app
// from this file's cgo preamble. A category adds methods to an existing class
// at load time, so iOS finds our implementation on Fyne's delegate without
// Fyne knowing anything about it — and it keeps working across Fyne upgrades,
// because it never touches Fyne's source.
//
// Both entry points are covered:
//   - continueUserActivity — the Universal Link itself, on a warm or cold start.
//   - openURL — a custom scheme, which we do not register, but implementing it
//     costs one method and means a future scheme needs no delegate work.
//
// The Go side (share_link_open.go) parks the link when the Bible is still
// loading, which on a cold start it always is.

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework UIKit -framework Foundation

#import <UIKit/UIKit.h>

// Implemented in Go (share_link_export_apple.go, //export). It copies the
// string immediately, so the transient UTF8String pointer is safe to pass.
//
// Returns non-zero when the URL is one of our reader links. ParseShareLink runs
// synchronously inside it, so this answer is real and both entry points below
// hand it straight back to iOS — see deliverShareLink for why the handling can
// still be asynchronous while the ANSWER is not.
extern int bibleTextOpenedLink(char *url);

// The class Fyne's mobile driver installs as the UIApplicationDelegate. It is
// declared (not defined) here so the category below can extend it.
@interface GoAppAppDelegate : UIResponder <UIApplicationDelegate>
@end

// A category adds these methods to Fyne's delegate without modifying Fyne.
@interface GoAppAppDelegate (BibleTextLinks)
@end

@implementation GoAppAppDelegate (BibleTextLinks)

// The Universal Link path. iOS calls this for a tapped https link the app has
// claimed, on both cold launch and while running.
- (BOOL)application:(UIApplication *)application
continueUserActivity:(NSUserActivity *)userActivity
 restorationHandler:(void (^)(NSArray<id<UIUserActivityRestoring>> *))restorationHandler {
    if (![userActivity.activityType isEqualToString:NSUserActivityTypeBrowsingWeb]) {
        return NO;
    }
    NSURL *url = userActivity.webpageURL;
    if (url == nil) {
        return NO;
    }
    // absoluteString keeps the fragment (#v16-18) — which is the whole point,
    // since the verse rides there and never reaches a server.
    //
    // REPORT WHAT GO DECIDED. This returned YES unconditionally, so iOS believed
    // the app had handled links it refused — /web/john/ (a book index, matched by
    // the "/web/*" component of the association file), /web/psalm/23/,
    // /web/john/three/ — and never fell back to Safari. The app just foregrounded
    // on whatever chapter it was already showing.
    return bibleTextOpenedLink((char *)url.absoluteString.UTF8String) ? YES : NO;
}

// Custom-scheme entry point. Unused today (no scheme is registered), but free.
- (BOOL)application:(UIApplication *)app
            openURL:(NSURL *)url
            options:(NSDictionary<UIApplicationOpenURLOptionsKey, id> *)options {
    if (url == nil) {
        return NO;
    }
    // Same rule as the Universal Link path above: an unrecognised URL is not
    // ours to swallow.
    return bibleTextOpenedLink((char *)url.absoluteString.UTF8String) ? YES : NO;
}

@end
*/
import "C"
