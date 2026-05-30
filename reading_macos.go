//go:build darwin && !ios

package holybible

// Native-macOS reading pane: a real AppKit NSTextView (editable=NO,
// selectable=YES) inside an NSScrollView, attached to the Fyne window's
// content view as an overlay. The user gets the full native macOS reading
// experience — character-level drag selection, and the system context menu
// (Copy / Look Up / Translate / Search With… / Share / Speech) automatically
// on selection — none of which Fyne's widget.Entry can provide.
//
// This is the desktop twin of the iOS UITextView overlay (reading_ios.go); the
// two share buildChapterHTML so the typography and verse-number styling are
// identical. The Fyne side keeps a transparent placeholder widget that reserves
// the rectangle and, on every Resize/Move, pushes that rectangle (flipped into
// AppKit's bottom-left coordinate space) to the NSScrollView frame.

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework AppKit -framework Foundation

#import <AppKit/AppKit.h>
#import <stdlib.h>

static NSScrollView *gScroll = nil;
static NSTextView   *gTextView = nil;

// Find the Fyne window. Fyne (via glfw) creates one standard NSWindow; prefer
// the key window, fall back to the first window.
static NSWindow *holyBibleMacWindow(void) {
    NSWindow *w = NSApp.keyWindow;
    if (w == nil) w = NSApp.mainWindow;
    if (w == nil && NSApp.windows.count > 0) w = NSApp.windows.firstObject;
    return w;
}

// Ensure the scroll view + text view exist and are parented to the current
// window's content view.
static void holyBibleMacEnsureTV(void) {
    dispatch_block_t block = ^{
        NSWindow *win = holyBibleMacWindow();
        if (win == nil || win.contentView == nil) return;

        if (gScroll == nil) {
            NSScrollView *sv = [[NSScrollView alloc] initWithFrame:NSMakeRect(0, 0, 200, 200)];
            sv.borderType = NSNoBorder;
            sv.hasVerticalScroller = YES;
            sv.hasHorizontalScroller = NO;
            sv.autohidesScrollers = YES;
            sv.drawsBackground = NO;

            NSSize contentSize = [sv contentSize];
            NSTextView *tv = [[NSTextView alloc] initWithFrame:NSMakeRect(0, 0, contentSize.width, contentSize.height)];
            tv.editable = NO;
            tv.selectable = YES;
            tv.richText = YES;
            tv.drawsBackground = NO;
            tv.textContainerInset = NSMakeSize(16, 14);
            tv.minSize = NSMakeSize(0, 0);
            tv.maxSize = NSMakeSize(CGFLOAT_MAX, CGFLOAT_MAX);
            tv.verticallyResizable = YES;
            tv.horizontallyResizable = NO;
            tv.autoresizingMask = NSViewWidthSizable;
            tv.textContainer.containerSize = NSMakeSize(contentSize.width, CGFLOAT_MAX);
            tv.textContainer.widthTracksTextView = YES;

            sv.documentView = tv;
            sv.hidden = YES;

            gScroll = sv;
            gTextView = tv;
        }
        if (gScroll.superview != win.contentView) {
            [gScroll removeFromSuperview];
            [win.contentView addSubview:gScroll];
        }
        [gScroll.superview addSubview:gScroll positioned:NSWindowAbove relativeTo:nil];
    };
    if ([NSThread isMainThread]) block();
    else dispatch_sync(dispatch_get_main_queue(), block);
}

void holyBibleMacTVSetHTML(const char *html) {
    if (html == NULL) return;
    NSString *s = [NSString stringWithUTF8String:html];
    NSData *data = [s dataUsingEncoding:NSUTF8StringEncoding];
    dispatch_async(dispatch_get_main_queue(), ^{
        holyBibleMacEnsureTV();
        if (gTextView == nil) return;
        NSDictionary *opts = @{
            NSDocumentTypeDocumentAttribute: NSHTMLTextDocumentType,
            NSCharacterEncodingDocumentAttribute: @(NSUTF8StringEncoding),
        };
        NSError *err = nil;
        NSMutableAttributedString *as =
            [[NSMutableAttributedString alloc] initWithData:data options:opts
                                         documentAttributes:nil error:&err];
        if (as == nil) {
            NSLog(@"holybible(mac): HTML parse failed: %@", err);
            [gTextView setString:s];
            return;
        }
        // The HTML importer injects a phantom paragraphSpacingBefore on the
        // first paragraph; zero it so the chapter starts flush at the top.
        [as enumerateAttribute:NSParagraphStyleAttributeName
                       inRange:NSMakeRange(0, as.length)
                       options:0
                    usingBlock:^(id v, NSRange r, BOOL *stop) {
            if (v == nil) return;
            NSMutableParagraphStyle *ps = [(NSParagraphStyle *)v mutableCopy];
            ps.paragraphSpacingBefore = 0;
            [as addAttribute:NSParagraphStyleAttributeName value:ps range:r];
        }];
        [gTextView.textStorage setAttributedString:as];
        // Scroll to the top of the new chapter.
        [gTextView scrollRangeToVisible:NSMakeRange(0, 0)];
        [[gScroll contentView] scrollToPoint:NSZeroPoint];
        [gScroll reflectScrolledClipView:gScroll.contentView];
    });
}

// holyBibleMacTVSetFrame positions the overlay. Inputs are Fyne coordinates
// (top-left origin, points). AppKit content views are non-flipped (bottom-left
// origin), so we flip Y using the content view height.
void holyBibleMacTVSetFrame(double x, double y, double w, double h) {
    dispatch_async(dispatch_get_main_queue(), ^{
        holyBibleMacEnsureTV();
        if (gScroll == nil) return;
        NSView *parent = gScroll.superview;
        if (parent == nil) return;
        CGFloat ph = parent.bounds.size.height;
        NSRect r = NSMakeRect(x, ph - y - h, w, h);
        gScroll.frame = r;
    });
}

void holyBibleMacTVShow(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        holyBibleMacEnsureTV();
        if (gScroll == nil) return;
        gScroll.hidden = NO;
        [gScroll.superview addSubview:gScroll positioned:NSWindowAbove relativeTo:nil];
    });
}

void holyBibleMacTVHide(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (gScroll == nil) return;
        gScroll.hidden = YES;
    });
}
*/
import "C"

import (
	"image/color"
	"time"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// readingScrollArea (macOS) returns a transparent host that reserves the
// reading rectangle; the native NSTextView paints the verses on top. A
// parchment rectangle sits behind it (the text view's background is clear).
func readingScrollArea(state *AppState, verses []Verse, pal palette) fyne.CanvasObject {
	if len(verses) == 0 {
		msg := widget.NewLabel("No verses are available for this chapter yet.")
		msg.Wrapping = fyne.TextWrapWord
		hideNativeReadingOverlayMac()
		return surface(container.NewPadded(msg), pal.Surface, pal.Border, fyne.Size{})
	}

	host := newMacReadingHost(state, verses)

	paper := canvas.NewRectangle(pal.Surface)
	paper.StrokeColor = pal.Border
	paper.StrokeWidth = 1
	paper.CornerRadius = 8
	return container.NewStack(paper, host)
}

// setReadingOverlayVisible shows/hides the NSTextView (called from
// buildReadingPane when switching between reading and search results).
func setReadingOverlayVisible(visible bool) {
	if visible {
		C.holyBibleMacTVShow()
	} else {
		C.holyBibleMacTVHide()
	}
}

func hideNativeReadingOverlayMac() { C.holyBibleMacTVHide() }

// macReadingHost is the transparent Fyne widget that tracks the reading
// rectangle and pushes it to the NSScrollView frame.
type macReadingHost struct {
	widget.BaseWidget
	state *AppState
}

// macCurrentHost guards stale deferred re-pins after a window rebuild.
var macCurrentHost *macReadingHost

func newMacReadingHost(state *AppState, verses []Verse) *macReadingHost {
	h := &macReadingHost{state: state}
	h.ExtendBaseWidget(h)
	macCurrentHost = h
	// Push HTML now; the frame follows on the first Resize.
	html := buildChapterHTML(state, verses)
	c := C.CString(html)
	defer C.free(unsafe.Pointer(c))
	C.holyBibleMacTVSetHTML(c)
	C.holyBibleMacTVShow()
	return h
}

func (h *macReadingHost) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}

func (h *macReadingHost) Resize(size fyne.Size) {
	h.BaseWidget.Resize(size)
	h.pushFrame()
}

func (h *macReadingHost) Move(p fyne.Position) {
	h.BaseWidget.Move(p)
	h.pushFrame()
}

// pushFrame projects the host's absolute canvas rect to the NSScrollView frame,
// immediately and again on the next tick once the layout settles.
func (h *macReadingHost) pushFrame() {
	setMacFrameFromObject(h)
	time.AfterFunc(60*time.Millisecond, func() {
		fyne.Do(func() {
			if macCurrentHost == h {
				setMacFrameFromObject(h)
			}
		})
	})
}

func setMacFrameFromObject(h *macReadingHost) {
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(h)
	sz := h.Size()
	if sz.Width <= 0 || sz.Height <= 0 {
		return
	}
	C.holyBibleMacTVSetFrame(
		C.double(pos.X), C.double(pos.Y),
		C.double(sz.Width), C.double(sz.Height),
	)
}
