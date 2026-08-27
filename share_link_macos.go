//go:build darwin && !ios

package bibletext

// Receiving a Universal Link on macOS.
//
// When someone clicks https://bibletext.co.uk/web/john/3/#v16 on a Mac that
// has the Store build installed, macOS hands the URL to the app delegate
// instead of the browser — but only when the app's signature claims the
// domain (release-mac-store.sh injects applinks: at signing time; see the
// entitlements file for why the claim cannot live in the tracked file) AND
// the association file at /.well-known/apple-app-site-association allow-lists
// the path. The direct-download build is unsigned, carries no claim, and
// keeps the paste-into-search route as its only way in.
//
// THE PROBLEM is the same one share_link_ios.go solves: the app delegate
// belongs to the toolkit. On desktop that is GLFW — its cocoa_init.m installs
// GLFWApplicationDelegate and implements no link callback — and Fyne exposes
// no Go-level hook for an incoming URL.
//
// THE FIX is the same too: an Objective-C CATEGORY, compiled into the app
// from this preamble, adds the methods to GLFW's delegate at load time
// without touching GLFW or Fyne. Load-time matters: a cold start FROM a link
// has the callback in place before the first event is delivered, and the Go
// side (share_link_open.go) parks the link while the Bible is still loading.
//
// GLFW is statically compiled in, so the category's class reference is a
// LINK-TIME symbol: a GLFW upgrade renaming its delegate class fails the
// build loudly (undefined _OBJC_CLASS_$_GLFWApplicationDelegate), not
// silently. The silent case to watch on a GLFW bump is different — the
// class keeping its name but no longer being installed as NSApp's
// delegate — which would leave clicked links opening the browser again.
// share_link_macos_test.go pins this file's own load-bearing spellings.
//
// Both entry points are covered, mirroring iOS:
//   - continueUserActivity — the Universal Link itself, warm or cold.
//   - openURLs — a custom scheme, which we do not register; implementing it
//     costs one method and a future scheme then needs no delegate work.

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework AppKit -framework Foundation

#import <AppKit/AppKit.h>

// Implemented in Go (share_link_export_apple.go, //export). It copies the
// string immediately, so the transient UTF8String pointer is safe to pass.
// Returns non-zero when the URL is one of our reader links; ParseShareLink
// runs synchronously inside it, so the answer handed back to macOS is real.
extern int bibleTextOpenedLink(char *url);

// The class GLFW installs as the NSApplicationDelegate (cocoa_init.m). It is
// declared (not defined) here so the category below can extend it.
@interface GLFWApplicationDelegate : NSObject <NSApplicationDelegate>
@end

@interface GLFWApplicationDelegate (BibleTextLinks)
@end

@implementation GLFWApplicationDelegate (BibleTextLinks)

// The Universal Link path. macOS calls this for a clicked https link the app
// has claimed, on both cold launch and while running.
- (BOOL)application:(NSApplication *)application
continueUserActivity:(NSUserActivity *)userActivity
 restorationHandler:(void (^)(NSArray<id<NSUserActivityRestoring>> *))restorationHandler {
    if (![userActivity.activityType isEqualToString:NSUserActivityTypeBrowsingWeb]) {
        return NO;
    }
    NSURL *url = userActivity.webpageURL;
    if (url == nil) {
        return NO;
    }
    // absoluteString keeps the fragment (#v16-18) — the verse and any note
    // ride there and never reach a server.
    //
    // REPORT WHAT GO DECIDED, exactly as the iOS twin learned to: answering
    // YES for a link Go refused (a book index, an unknown path) would stop
    // macOS from falling back to the browser, and the app would just
    // foreground on whatever it was already showing.
    return bibleTextOpenedLink((char *)url.absoluteString.UTF8String) ? YES : NO;
}

// Custom-scheme entry point. Unused today (no scheme is registered).
- (void)application:(NSApplication *)application openURLs:(NSArray<NSURL *> *)urls {
    for (NSURL *url in urls) {
        bibleTextOpenedLink((char *)url.absoluteString.UTF8String);
    }
}

@end
*/
import "C"
