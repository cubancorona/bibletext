//go:build darwin && !ios

package bibletext

// Opening a link on desktop macOS through AppKit rather than a subprocess.
//
// -[NSWorkspace openURL:] is the supported API: it works inside the App
// Sandbox with no entitlement, returns immediately (the receiving app is
// launched by the window server, not by us), and reports whether the URL was
// accepted. Fyne's own darwin implementation runs `open` through os/exec and
// blocks on it, which the sandbox is entitled to refuse and which stalls
// whichever goroutine tapped the link.
//
// ARC is on, matching the other AppKit files here (reading_macos.go,
// audio_macos.go).

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework AppKit -framework Foundation

#import <AppKit/AppKit.h>

// Returns 1 when AppKit accepted the URL, 0 when it refused or the string is
// not a URL it can parse.
static int btOpenExternalURL(const char *raw) {
    if (raw == NULL) return 0;
    @autoreleasepool {
        NSString *s = [NSString stringWithUTF8String:raw];
        if (s == nil) return 0;
        NSURL *u = [NSURL URLWithString:s];
        if (u == nil) return 0;
        return [[NSWorkspace sharedWorkspace] openURL:u] ? 1 : 0;
    }
}
*/
import "C"

import (
	"errors"
	"net/url"
	"unsafe"
)

func openExternalURLPlatform(u *url.URL) error {
	raw := u.String()
	c := C.CString(raw)
	defer C.free(unsafe.Pointer(c))
	if C.btOpenExternalURL(c) == 1 {
		return nil
	}
	// AppKit refuses when nothing is registered for the scheme — a Mac with no
	// mail client and a mailto: link, say. That is the reader's situation to
	// know about, not a bug to swallow.
	return errors.New("macOS declined to open the link: " + u.Scheme)
}
