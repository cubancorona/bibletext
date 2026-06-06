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

// Implemented in Go (ai_menu_darwin.go, //export). Called when the reader picks
// an AI study action; it copies both strings immediately.
extern void holyBibleAIMenuTapped(char *action, char *text);

// HBReadingTextView adds a "Study with AI" submenu (Explain / Analyze context /
// Analyze translation) to the right-click selection menu and hands the selected
// text to Go.
@interface HBReadingTextView : NSTextView
@end

@implementation HBReadingTextView

- (NSString *)hbSelectedText {
    NSRange sel = self.selectedRange;
    if (sel.length == 0 || NSMaxRange(sel) > self.textStorage.length) return @"";
    return [self.textStorage.string substringWithRange:sel];
}

- (NSMenu *)menuForEvent:(NSEvent *)event {
    NSMenu *menu = [super menuForEvent:event];
    if (menu == nil || self.selectedRange.length == 0) return menu;

    NSMenu *ai = [[NSMenu alloc] initWithTitle:@"Study with AI"];
    for (NSArray *pair in @[@[@"Explain", @"explain"],
                            @[@"Analyze context", @"context"],
                            @[@"Analyze translation", @"translation"]]) {
        SEL action = NSSelectorFromString([NSString stringWithFormat:@"hbAI_%@:", pair[1]]);
        NSMenuItem *it = [[NSMenuItem alloc] initWithTitle:pair[0] action:action keyEquivalent:@""];
        it.target = self;
        [ai addItem:it];
    }

    [menu addItem:[NSMenuItem separatorItem]];
    NSMenuItem *aiItem = [[NSMenuItem alloc] initWithTitle:@"Study with AI" action:nil keyEquivalent:@""];
    aiItem.submenu = ai;
    [menu addItem:aiItem];
    return menu;
}

- (void)hbAI_explain:(id)sender {
    holyBibleAIMenuTapped((char *)"explain", (char *)self.hbSelectedText.UTF8String);
}
- (void)hbAI_context:(id)sender {
    holyBibleAIMenuTapped((char *)"context", (char *)self.hbSelectedText.UTF8String);
}
- (void)hbAI_translation:(id)sender {
    holyBibleAIMenuTapped((char *)"translation", (char *)self.hbSelectedText.UTF8String);
}

@end

static NSScrollView *gScroll = nil;
static NSTextView   *gTextView = nil;

// Character range of the highlighted verse (set when arriving from a search
// result), or {NSNotFound, 0} for a plain chapter. holyBibleMacScrollTV uses it
// to land the highlighted verse near the top instead of pinning to verse 1.
static NSRange gMacHighlightRange = {NSNotFound, 0};

// gReadingSuppressed is raised while a Fyne modal (chapter picker, AI panel, AI
// settings) is open. The native NSTextView floats above the whole Fyne canvas,
// so it must stay down for the duration of the modal — not just be hidden once.
// A layout pass behind the modal can call holyBibleMacTVShow again (e.g. a scroll
// re-pins the overlay), which would paint the verses back over the popup and
// steal its clicks. While suppressed, Show is a no-op; only Unsuppress clears it.
static BOOL gReadingSuppressed = NO;

// holyBibleMacScrollTV positions the chapter: at the highlighted verse when one
// is set (a search jump), otherwise at the very top. NSTextView is flipped, so
// larger y is further down; we scroll the clip view to the verse's glyph rect.
static void holyBibleMacScrollTV(void) {
    if (gTextView == nil || gScroll == nil) return;
    if (gMacHighlightRange.location != NSNotFound &&
        gMacHighlightRange.length > 0 &&
        NSMaxRange(gMacHighlightRange) <= gTextView.textStorage.length) {
        NSLayoutManager *lm = gTextView.layoutManager;
        NSRange glyphs = [lm glyphRangeForCharacterRange:gMacHighlightRange
                                    actualCharacterRange:NULL];
        NSRect rect = [lm boundingRectForGlyphRange:glyphs
                                    inTextContainer:gTextView.textContainer];
        CGFloat y = rect.origin.y + gTextView.textContainerInset.height - 16;
        if (y < 0) y = 0;
        [[gScroll contentView] scrollToPoint:NSMakePoint(0, y)];
        [gScroll reflectScrolledClipView:gScroll.contentView];
        return;
    }
    [gTextView scrollRangeToVisible:NSMakeRange(0, 0)];
    [[gScroll contentView] scrollToPoint:NSZeroPoint];
    [gScroll reflectScrolledClipView:gScroll.contentView];
}

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
            HBReadingTextView *tv = [[HBReadingTextView alloc] initWithFrame:NSMakeRect(0, 0, contentSize.width, contentSize.height)];
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
        // Find the highlighted verse (the .hl span becomes a background-coloured
        // run) so a search jump lands on it rather than the chapter's top.
        gMacHighlightRange = (NSRange){NSNotFound, 0};
        [as enumerateAttribute:NSBackgroundColorAttributeName
                       inRange:NSMakeRange(0, as.length)
                       options:0
                    usingBlock:^(id value, NSRange r, BOOL *stop) {
            if (value != nil) {
                gMacHighlightRange = r;
                *stop = YES;
            }
        }];
        [gTextView.textStorage setAttributedString:as];
        holyBibleMacScrollTV();
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
        BOOL changed = !NSEqualRects(r, gScroll.frame);
        gScroll.frame = r;
        // SetHTML may have scrolled to the highlighted verse while the overlay
        // was still at its initial width; once the real frame lands the text
        // rewraps, so re-assert the highlight position. Only when a highlight is
        // active — otherwise leave the reader's scroll position untouched.
        if (changed && gMacHighlightRange.location != NSNotFound) {
            holyBibleMacScrollTV();
        }
    });
}

void holyBibleMacTVShow(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (gReadingSuppressed) return; // a modal is up; stay down until released
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

// holyBibleMacTVSuppress hides the overlay and latches it down so that any
// stray holyBibleMacTVShow from a layout pass behind a modal is ignored.
void holyBibleMacTVSuppress(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        gReadingSuppressed = YES;
        if (gScroll == nil) return;
        gScroll.hidden = YES;
    });
}

// holyBibleMacTVUnsuppress clears the latch. It does not show the overlay on its
// own — the caller decides whether to show (reading) or keep hidden (search).
void holyBibleMacTVUnsuppress(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        gReadingSuppressed = NO;
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
	// The NSTextView floats above the Fyne canvas, so any Fyne popup (the
	// chapter picker) would render behind it. Let shared code hide/show the
	// overlay around such popups — showChapterPicker calls these.
	state.hideReadingOverlay = func() { C.holyBibleMacTVSuppress() }
	state.showReadingOverlay = func() {
		C.holyBibleMacTVUnsuppress()
		// Restore only the overlay that belongs to the current view: the reading
		// text when reading, nothing when search results are showing (so closing
		// settings mid-search doesn't paint verses over the results).
		setReadingOverlayVisible(!state.IsSearching)
	}

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
