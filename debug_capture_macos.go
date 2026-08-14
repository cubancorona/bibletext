//go:build bibletextdev && darwin && !ios

package bibletext

// In-process window capture, for looking at the REAL macOS app.
//
// WHY THIS EXISTS RATHER THAN screencapture(1). Grabbing the screen from a
// script needs the Screen Recording entitlement, which macOS caches at process
// start: granting it to the terminal mid-session does nothing until that app is
// relaunched, and relaunching it ends the session doing the looking. This draws
// the window's own view hierarchy into a bitmap from inside the process, which
// is not a screen read at all and needs no permission.
//
// It also captures MORE than a Fyne canvas grab would. The reading pane on
// macOS is a native NSTextView floating above the GL surface, and the note
// bubble, the follow pill and the selection are all AppKit — none of them exist
// in Fyne's canvas. cacheDisplayInRect: walks the real hierarchy, so what lands
// in the PNG is what the reader sees.
//
// DEVELOPMENT BUILDS ONLY (the bibletextdev tag, like dev_autoopen_on.go).
// Release builds get debug_capture_off.go, whose body is empty.

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

// btDebugCaptureWindow renders the key window's content view into a PNG at
// `path`. Returns 1 on success. Main-thread only work, dispatched here so the
// caller can be any goroutine.
static int btDebugCaptureWindow(const char *path) {
    if (path == NULL) return 0;
    NSString *p = [NSString stringWithUTF8String:path];
    __block int ok = 0;
    dispatch_block_t block = ^{
        NSWindow *win = [NSApp keyWindow];
        if (win == nil) win = [NSApp mainWindow];
        if (win == nil) win = [[NSApp windows] firstObject];
        if (win == nil) return;
        NSView *v = win.contentView;
        if (v == nil) return;

        // cacheDisplayInRect: re-draws the hierarchy into the rep, so it picks
        // up the AppKit overlays. It does NOT capture the GL surface Fyne draws
        // into — that comes out as the window's background — so the reading
        // pane (native) is what this is good for, and the Fyne chrome around it
        // reads as flat colour. That limit is the whole reason the styled-pane
        // snapshots (software canvas) exist alongside this.
        NSBitmapImageRep *rep = [v bitmapImageRepForCachingDisplayInRect:v.bounds];
        if (rep == nil) return;
        [v cacheDisplayInRect:v.bounds toBitmapImageRep:rep];
        NSData *png = [rep representationUsingType:NSBitmapImageFileTypePNG properties:@{}];
        if (png == nil) return;
        ok = [png writeToFile:p atomically:YES] ? 1 : 0;
    };
    if ([NSThread isMainThread]) block();
    else dispatch_sync(dispatch_get_main_queue(), block);
    return ok;
}
*/
import "C"

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"unsafe"
)

// debugCaptureWindow writes a PNG of the live window to path.
func debugCaptureWindow(path string) error {
	c := C.CString(path)
	defer C.free(unsafe.Pointer(c))
	if C.btDebugCaptureWindow(c) == 0 {
		return fmt.Errorf("capture failed (no window yet?)")
	}
	return nil
}

var debugShotSeq atomic.Int64

// InstallDebugCapture arms SIGUSR1 to write a numbered PNG into the directory
// named by BIBLETEXT_DEV_SHOTS. Signal-driven rather than timed because the
// interesting moments are the ones somebody drives — open a note, scroll, flip
// the theme — and a timer cannot know when those happened.
//
// Called from the desktop entry point; a no-op when the env var is unset.
func InstallDebugCapture() {
	dir := os.Getenv("BIBLETEXT_DEV_SHOTS")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "bibletext(dev): shots dir: %v\n", err)
		return
	}
	ch := make(chan os.Signal, 4)
	signal.Notify(ch, syscall.SIGUSR1)
	go func() {
		for range ch {
			n := debugShotSeq.Add(1)
			p := filepath.Join(dir, fmt.Sprintf("shot-%02d.png", n))
			if err := debugCaptureWindow(p); err != nil {
				fmt.Fprintf(os.Stderr, "bibletext(dev): %v\n", err)
				continue
			}
			fmt.Fprintf(os.Stderr, "bibletext(dev): wrote %s\n", p)
		}
	}()
	fmt.Fprintf(os.Stderr, "bibletext(dev): SIGUSR1 → PNG in %s (pid %d)\n", dir, os.Getpid())
}
