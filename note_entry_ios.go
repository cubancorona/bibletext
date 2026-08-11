//go:build ios

package bibletext

// The native compose field for Share-with-note.
//
// WHY NATIVE. The Fyne Entry works, but it is a canvas-drawn imitation of a
// text field: no dictation, no autocorrect or predictive bar, no system
// selection loupe, no undo gesture, no VoiceOver text semantics — and its
// emoji come from whatever font we bundle rather than the system set. For the
// one field in the app where a person writes a message to another person,
// those absences are the difference between typing and composing.
//
// HOW. The same trick as the reading pane and the note sticker: a real UIKit
// view floating ABOVE the Fyne canvas. The sheet keeps a transparent
// placeholder rectangle where the field belongs; once the popup lays out, the
// placeholder's absolute rect is pushed here and a UITextView is parked over
// it. Fyne never knows the difference; the sheet's counter and Share button
// read the text back across cgo.
//
// The view is added to the root view controller's view, NOT the bare window —
// same rule as the reading overlay (CLAUDE.md): system edit actions walk the
// responder chain for a view controller and misbehave without one.

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework UIKit

#import <UIKit/UIKit.h>
#include <stdlib.h>
#include <string.h>

extern void bibleTextNoteEntryChanged(void);

// A delegate whose only job is to forward "the text changed" to Go (for the
// live character counter) and keep the placeholder label honest.
@interface BTNoteEntryDelegate : NSObject <UITextViewDelegate>
@end

static UITextView *gNoteEntryTV = nil;
static UILabel *gNoteEntryPH = nil;          // placeholder: UITextView has none of its own
static BTNoteEntryDelegate *gNoteEntryDelegate = nil;

static void btNoteEntryUpdatePH(void) {
    if (gNoteEntryPH == nil || gNoteEntryTV == nil) return;
    gNoteEntryPH.hidden = gNoteEntryTV.text.length > 0;
}

@implementation BTNoteEntryDelegate
- (void)textViewDidChange:(UITextView *)textView {
    btNoteEntryUpdatePH();
    bibleTextNoteEntryChanged();
}
@end

static UIWindow *btNoteEntryWindow(void) {
    UIWindow *fallback = nil;
    for (UIScene *scene in UIApplication.sharedApplication.connectedScenes) {
        if (![scene isKindOfClass:[UIWindowScene class]] ||
            scene.activationState == UISceneActivationStateUnattached) continue;
        for (UIWindow *w in ((UIWindowScene *)scene).windows) {
            if (w.isKeyWindow) return w;
            if (fallback == nil && !w.hidden) fallback = w;
        }
    }
    return fallback;
}

// Show creates the view (idempotently — a second call replaces the first) and
// brings up the keyboard. Colours arrive as sRGB components from the app
// palette so light/dark at open time matches the sheet around it.
// EVERY entry point hops to the main queue — Fyne's UI goroutine is NOT the
// iOS main thread, and UIKit asserts (and eventually kills the app) when views
// are built or laid out anywhere else. Show/SetFrame/Hide are fire-and-forget
// so they dispatch_async; Text must return a value so it uses a
// main-thread-guarded dispatch_sync.
static void bibleTextNoteEntryShow(const char *initial, const char *placeholder,
                                   double textR, double textG, double textB,
                                   double phR, double phG, double phB,
                                   double bgR, double bgG, double bgB,
                                   double bdR, double bdG, double bdB) {
    NSString *initialS = (initial != NULL && *initial) ? [NSString stringWithUTF8String:initial] : nil;
    NSString *phS = (placeholder != NULL) ? [NSString stringWithUTF8String:placeholder] : @"";
    dispatch_async(dispatch_get_main_queue(), ^{
    UIWindow *win = btNoteEntryWindow();
    if (win == nil || win.rootViewController == nil) return;
    if (gNoteEntryTV != nil) { [gNoteEntryTV removeFromSuperview]; gNoteEntryTV = nil; gNoteEntryPH = nil; }

    UITextView *tv = [[UITextView alloc] initWithFrame:CGRectZero];
    tv.font = [UIFont systemFontOfSize:18];
    tv.textColor = [UIColor colorWithRed:textR green:textG blue:textB alpha:1];
    tv.backgroundColor = [UIColor colorWithRed:bgR green:bgG blue:bgB alpha:1];
    tv.layer.cornerRadius = 10;
    tv.layer.borderWidth = 1;
    tv.layer.borderColor = [UIColor colorWithRed:bdR green:bdG blue:bdB alpha:1].CGColor;
    tv.textContainerInset = UIEdgeInsetsMake(10, 6, 10, 6);
    tv.keyboardType = UIKeyboardTypeDefault;
    tv.hidden = YES; // stays hidden until the first real frame arrives — no (0,0) flash
    if (initialS != nil) tv.text = initialS;

    if (gNoteEntryDelegate == nil) gNoteEntryDelegate = [[BTNoteEntryDelegate alloc] init];
    tv.delegate = gNoteEntryDelegate;

    UILabel *ph = [[UILabel alloc] initWithFrame:CGRectZero];
    ph.text = phS;
    ph.font = tv.font;
    ph.textColor = [UIColor colorWithRed:phR green:phG blue:phB alpha:1];
    ph.userInteractionEnabled = NO;
    ph.adjustsFontSizeToFitWidth = YES;
    ph.minimumScaleFactor = 0.7;
    [tv addSubview:ph];

    [win.rootViewController.view addSubview:tv];
    gNoteEntryTV = tv;
    gNoteEntryPH = ph;
    btNoteEntryUpdatePH();
    [tv becomeFirstResponder];
    });
}

static void bibleTextNoteEntrySetFrame(double x, double y, double w, double h) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (gNoteEntryTV == nil) return;
        // Fyne's canvas sits inset below the device safe area, so a Fyne Y maps
        // to superview Y + safeAreaInsets.top — the same correction
        // bibleTextTVSetFrame applies for the reading overlay. Without it the
        // field parks a status-bar-height too high, over the sheet's title.
        UIEdgeInsets safe = UIEdgeInsetsZero;
        if (gNoteEntryTV.superview != nil) safe = gNoteEntryTV.superview.safeAreaInsets;
        gNoteEntryTV.frame = CGRectMake(x + safe.left, y + safe.top, w, h);
        if (gNoteEntryPH != nil) gNoteEntryPH.frame = CGRectMake(11, 10, w - 22, 22);
        gNoteEntryTV.hidden = NO;
    });
}

// The current text, strdup'd: Go frees it. Synchronous by necessity, so it
// guards against already being on main before dispatch_sync (which would
// otherwise deadlock).
static char *bibleTextNoteEntryText(void) {
    __block char *out = NULL;
    void (^read)(void) = ^{
        out = strdup(gNoteEntryTV != nil ? ([gNoteEntryTV.text UTF8String] ?: "") : "");
    };
    if ([NSThread isMainThread]) read();
    else dispatch_sync(dispatch_get_main_queue(), read);
    return out;
}

static void bibleTextNoteEntryHide(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (gNoteEntryTV == nil) return;
        [gNoteEntryTV resignFirstResponder];
        [gNoteEntryTV removeFromSuperview];
        gNoteEntryTV = nil;
        gNoteEntryPH = nil;
    });
}
*/
import "C"

import (
	"unsafe"

	"fyne.io/fyne/v2"
)

func nativeNoteEntrySupported() bool { return true }

// col splits a palette colour into the 0..1 components the ObjC side wants.
func noteEntryCol(c interface{ RGBA() (r, g, b, a uint32) }) (float64, float64, float64) {
	r, g, b, _ := c.RGBA()
	return float64(r) / 65535, float64(g) / 65535, float64(b) / 65535
}

func showNativeNoteEntry(initial, placeholder string, pal palette) {
	ci := C.CString(initial)
	cp := C.CString(placeholder)
	defer C.free(unsafe.Pointer(ci))
	defer C.free(unsafe.Pointer(cp))
	tr, tg, tb := noteEntryCol(pal.Text)
	pr, pg, pb := noteEntryCol(pal.TextMuted)
	br, bg2, bb := noteEntryCol(pal.Background)
	dr, dg, db := noteEntryCol(pal.Border)
	C.bibleTextNoteEntryShow(ci, cp,
		C.double(tr), C.double(tg), C.double(tb),
		C.double(pr), C.double(pg), C.double(pb),
		C.double(br), C.double(bg2), C.double(bb),
		C.double(dr), C.double(dg), C.double(db))
}

// setNativeNoteEntryFrameFromObject parks the native view exactly over the
// sheet's placeholder rectangle — the same absolute-position transform the
// reading overlay uses (setFrameFromObject).
func setNativeNoteEntryFrameFromObject(obj fyne.CanvasObject) {
	if obj == nil {
		return
	}
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(obj)
	sz := obj.Size()
	if sz.Width <= 0 || sz.Height <= 0 {
		return
	}
	C.bibleTextNoteEntrySetFrame(C.double(pos.X), C.double(pos.Y),
		C.double(sz.Width), C.double(sz.Height))
}

func nativeNoteEntryText() string {
	cs := C.bibleTextNoteEntryText()
	defer C.free(unsafe.Pointer(cs))
	return C.GoString(cs)
}

func hideNativeNoteEntry() { C.bibleTextNoteEntryHide() }
