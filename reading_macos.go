//go:build darwin && !ios

package bibletext

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
extern void bibleTextAIMenuTapped(char *action, char *text);
// Sibling callback for the non-AI selection-menu actions (Share verse, …).
extern void bibleTextStudyMenuTapped(char *action, char *text);

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

    // Cross-references → related passages for the selection.
    NSMenuItem *xref = [[NSMenuItem alloc] initWithTitle:@"Cross-references" action:@selector(hbCrossRefs:) keyEquivalent:@""];
    xref.target = self;
    [menu addItem:xref];

    // Share verse → with citation / as image (both go to the macOS share sheet).
    NSMenu *share = [[NSMenu alloc] initWithTitle:@"Share verse"];
    NSMenuItem *sc = [[NSMenuItem alloc] initWithTitle:@"Share with citation" action:@selector(hbShare_cite:) keyEquivalent:@""];
    sc.target = self;
    [share addItem:sc];
    NSMenuItem *si = [[NSMenuItem alloc] initWithTitle:@"Share as image" action:@selector(hbShare_image:) keyEquivalent:@""];
    si.target = self;
    [share addItem:si];
    NSMenuItem *shareItem = [[NSMenuItem alloc] initWithTitle:@"Share verse" action:nil keyEquivalent:@""];
    shareItem.submenu = share;
    [menu addItem:shareItem];
    return menu;
}

- (void)hbAI_explain:(id)sender {
    bibleTextAIMenuTapped((char *)"explain", (char *)self.hbSelectedText.UTF8String);
}
- (void)hbAI_context:(id)sender {
    bibleTextAIMenuTapped((char *)"context", (char *)self.hbSelectedText.UTF8String);
}
- (void)hbAI_translation:(id)sender {
    bibleTextAIMenuTapped((char *)"translation", (char *)self.hbSelectedText.UTF8String);
}
- (void)hbCrossRefs:(id)sender {
    bibleTextStudyMenuTapped((char *)"crossref", (char *)self.hbSelectedText.UTF8String);
}
- (void)hbShare_cite:(id)sender {
    bibleTextStudyMenuTapped((char *)"share-cite", (char *)self.hbSelectedText.UTF8String);
}
- (void)hbShare_image:(id)sender {
    bibleTextStudyMenuTapped((char *)"share-image", (char *)self.hbSelectedText.UTF8String);
}

@end

static NSScrollView *gScroll = nil;
static NSTextView   *gTextView = nil;

// Character range of the highlighted verse (set when arriving from a search
// result), or {NSNotFound, 0} for a plain chapter. bibleTextMacScrollTV uses it
// to land the highlighted verse near the top instead of pinning to verse 1.
static NSRange gMacHighlightRange = {NSNotFound, 0};

// gReadingSuppressed is raised while a Fyne modal (chapter picker, AI panel, AI
// settings) is open. The native NSTextView floats above the whole Fyne canvas,
// so it must stay down for the duration of the modal — not just be hidden once.
// A layout pass behind the modal can call bibleTextMacTVShow again (e.g. a scroll
// re-pins the overlay), which would paint the verses back over the popup and
// steal its clicks. While suppressed, Show is a no-op; only Unsuppress clears it.
static BOOL gReadingSuppressed = NO;

// --- Reading-position restore -------------------------------------------------
// A one-shot scroll target applied when reopening into the last-read chapter
// (see reading_state.go). Verse numbers render as <sup> runs (buildChapterHTML),
// so we map between a verse number and its character location by enumerating the
// superscript runs — the trailing &nbsp; sits outside the <sup>, so each
// superscript run is exactly the verse digits.

// `ok` distinguishes "read the live scroll" (1, even at the top) from "couldn't
// read it — view gone" (0).
typedef struct { int verse; double delta; double frac; int ok; } BTAnchor;

static NSInteger gMacRestoreVerse = 0;
static CGFloat   gMacRestoreDelta = 0;
static CGFloat   gMacRestoreFrac  = 0;
static BOOL      gMacHasRestore   = NO;

// Verse numbers are the only small-font runs: buildChapterHTML renders them as
// <sup class="v"> at font-size 0.66em (~12.5px) while body text is 19px.
// Detecting them by font size (rather than a superscript attribute) matches the
// iOS overlay and reads the run's digits directly.
#define BT_VERSE_FONT_MAX 15.0

// btMacLocForVerse returns the character location of `verse`'s number run, or
// NSNotFound.
static NSUInteger btMacLocForVerse(NSTextStorage *ts, NSInteger verse) {
    __block NSUInteger found = NSNotFound;
    [ts enumerateAttribute:NSFontAttributeName
                   inRange:NSMakeRange(0, ts.length)
                   options:0
                usingBlock:^(id val, NSRange r, BOOL *stop) {
        if (val == nil || r.length == 0 || ((NSFont *)val).pointSize >= BT_VERSE_FONT_MAX) return;
        if ([[ts.string substringWithRange:r] integerValue] == verse) {
            found = r.location;
            *stop = YES;
        }
    }];
    return found;
}

// btMacVerseAtIndex returns the verse whose number run is the last at or before
// character index ci (the top-visible verse), writing its location to *outLoc.
static NSInteger btMacVerseAtIndex(NSTextStorage *ts, NSUInteger ci, NSUInteger *outLoc) {
    __block NSInteger verse = 0;
    __block NSUInteger loc = 0;
    [ts enumerateAttribute:NSFontAttributeName
                   inRange:NSMakeRange(0, ts.length)
                   options:0
                usingBlock:^(id val, NSRange r, BOOL *stop) {
        if (r.location > ci) { *stop = YES; return; }
        if (val == nil || r.length == 0 || ((NSFont *)val).pointSize >= BT_VERSE_FONT_MAX) return;
        NSInteger n = [[ts.string substringWithRange:r] integerValue];
        if (n > 0) { verse = n; loc = r.location; }
    }];
    if (outLoc) *outLoc = loc;
    return verse;
}

// bibleTextMacScrollTV positions the chapter, in priority order: the highlighted
// verse (a search jump), then a one-shot restore target (reopening where the
// reader left off), otherwise the very top. NSTextView is flipped, so larger y is
// further down; we scroll the clip view to the target glyph rect.
static void bibleTextMacScrollTV(void) {
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
    if (gMacHasRestore) {
        NSLayoutManager *lm = gTextView.layoutManager;
        NSTextStorage *ts = gTextView.textStorage;
        CGFloat insetH = gTextView.textContainerInset.height;
        CGFloat y = -1;
        if (gMacRestoreVerse > 0) {
            NSUInteger loc = btMacLocForVerse(ts, gMacRestoreVerse);
            if (loc != NSNotFound) {
                NSRange g = [lm glyphRangeForCharacterRange:NSMakeRange(loc, 1)
                                       actualCharacterRange:NULL];
                NSRect rr = [lm boundingRectForGlyphRange:g inTextContainer:gTextView.textContainer];
                y = rr.origin.y + insetH + gMacRestoreDelta;
            }
        }
        if (y < 0 && gMacRestoreFrac > 0) {
            CGFloat docH = [lm usedRectForTextContainer:gTextView.textContainer].size.height + insetH * 2;
            CGFloat viewH = gScroll.contentView.bounds.size.height;
            CGFloat scrollable = docH - viewH;
            if (scrollable > 0) y = gMacRestoreFrac * scrollable;
        }
        if (y >= 0) {
            CGFloat maxY = gTextView.frame.size.height - gScroll.contentView.bounds.size.height;
            if (maxY < 0) maxY = 0;
            if (y > maxY) y = maxY;
            if (y < 0) y = 0;
            [[gScroll contentView] scrollToPoint:NSMakePoint(0, y)];
            [gScroll reflectScrolledClipView:gScroll.contentView];
            return;
        }
    }
    [gTextView scrollRangeToVisible:NSMakeRange(0, 0)];
    [[gScroll contentView] scrollToPoint:NSZeroPoint];
    [gScroll reflectScrolledClipView:gScroll.contentView];
}

// Find the Fyne window. Fyne (via glfw) creates one standard NSWindow; prefer
// the key window, fall back to the first window.
static NSWindow *bibleTextMacWindow(void) {
    NSWindow *w = NSApp.keyWindow;
    if (w == nil) w = NSApp.mainWindow;
    if (w == nil && NSApp.windows.count > 0) w = NSApp.windows.firstObject;
    return w;
}

// Ensure the scroll view + text view exist and are parented to the current
// window's content view.
static void bibleTextMacEnsureTV(void) {
    dispatch_block_t block = ^{
        NSWindow *win = bibleTextMacWindow();
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

void bibleTextMacTVSetHTML(const char *html) {
    if (html == NULL) return;
    NSString *s = [NSString stringWithUTF8String:html];
    NSData *data = [s dataUsingEncoding:NSUTF8StringEncoding];
    dispatch_async(dispatch_get_main_queue(), ^{
        bibleTextMacEnsureTV();
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
            NSLog(@"bibletext(mac): HTML parse failed: %@", err);
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
        bibleTextMacScrollTV();
    });
}

// bibleTextMacTVSetFrame positions the overlay. Inputs are Fyne coordinates
// (top-left origin, points). AppKit content views are non-flipped (bottom-left
// origin), so we flip Y using the content view height.
void bibleTextMacTVSetFrame(double x, double y, double w, double h) {
    dispatch_async(dispatch_get_main_queue(), ^{
        bibleTextMacEnsureTV();
        if (gScroll == nil) return;
        NSView *parent = gScroll.superview;
        if (parent == nil) return;
        CGFloat ph = parent.bounds.size.height;
        NSRect r = NSMakeRect(x, ph - y - h, w, h);
        BOOL changed = !NSEqualRects(r, gScroll.frame);
        gScroll.frame = r;
        // SetHTML may have scrolled to the highlighted verse / restore target
        // while the overlay was still at its initial width; once the real frame
        // lands the text rewraps, so re-assert that position. Only when a
        // highlight or a pending restore is active — otherwise leave the reader's
        // scroll position untouched.
        if (changed && (gMacHighlightRange.location != NSNotFound || gMacHasRestore)) {
            BOOL wasRestore = gMacHasRestore;
            bibleTextMacScrollTV();
            // One-shot: once the real frame has landed and the restore scroll has
            // been re-applied at the correct width, disarm — so later user resizes
            // don't snap the reader back to the restored position.
            if (wasRestore) gMacHasRestore = NO;
        }
    });
}

void bibleTextMacTVShow(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (gReadingSuppressed) return; // a modal is up; stay down until released
        bibleTextMacEnsureTV();
        if (gScroll == nil) return;
        gScroll.hidden = NO;
        [gScroll.superview addSubview:gScroll positioned:NSWindowAbove relativeTo:nil];
    });
}

void bibleTextMacTVHide(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (gScroll == nil) return;
        gScroll.hidden = YES;
    });
}

// bibleTextMacTVSuppress hides the overlay and latches it down so that any
// stray bibleTextMacTVShow from a layout pass behind a modal is ignored.
void bibleTextMacTVSuppress(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        gReadingSuppressed = YES;
        if (gScroll == nil) return;
        gScroll.hidden = YES;
    });
}

// bibleTextMacTVUnsuppress clears the latch. It does not show the overlay on its
// own — the caller decides whether to show (reading) or keep hidden (search).
void bibleTextMacTVUnsuppress(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        gReadingSuppressed = NO;
    });
}

// --- Share -----------------------------------------------------------------
// Present the macOS share sheet (NSSharingServicePicker) anchored at the current
// selection, so "Share with citation" / "Share as image" reach Messages, Mail,
// Notes, AirDrop, etc. — the same destinations Copy/Share would.
static NSRect bibleTextMacSelectionRect(void) {
    if (gTextView == nil) return NSZeroRect;
    NSRange sel = gTextView.selectedRange;
    if (sel.length == 0) {
        NSRect b = gTextView.visibleRect;
        return NSMakeRect(NSMidX(b), NSMidY(b), 1, 1);
    }
    NSLayoutManager *lm = gTextView.layoutManager;
    NSRange g = [lm glyphRangeForCharacterRange:sel actualCharacterRange:NULL];
    NSRect r = [lm boundingRectForGlyphRange:g inTextContainer:gTextView.textContainer];
    r.origin.x += gTextView.textContainerInset.width;
    r.origin.y += gTextView.textContainerInset.height;
    return r;
}

void bibleTextShareText(const char *text) {
    if (text == NULL) return;
    NSString *s = [NSString stringWithUTF8String:text];
    dispatch_async(dispatch_get_main_queue(), ^{
        if (gTextView == nil || s.length == 0) return;
        NSSharingServicePicker *p = [[NSSharingServicePicker alloc] initWithItems:@[s]];
        [p showRelativeToRect:bibleTextMacSelectionRect() ofView:gTextView preferredEdge:NSMaxYEdge];
    });
}

void bibleTextShareImageFile(const char *path) {
    if (path == NULL) return;
    NSString *p = [NSString stringWithUTF8String:path];
    dispatch_async(dispatch_get_main_queue(), ^{
        if (gTextView == nil) return;
        NSImage *img = [[NSImage alloc] initWithContentsOfFile:p];
        NSArray *items = img ? @[img] : @[[NSURL fileURLWithPath:p]];
        NSSharingServicePicker *sp = [[NSSharingServicePicker alloc] initWithItems:items];
        [sp showRelativeToRect:bibleTextMacSelectionRect() ofView:gTextView preferredEdge:NSMaxYEdge];
    });
}

// --- Reading-position capture / restore (Go bridge) -------------------------

// bibleTextMacCaptureAnchor reads the current scroll position as a verse anchor
// (top-visible verse + within-verse delta) plus a whole-chapter fraction
// fallback. Synchronous on the main thread; safe to call during shutdown (it
// null-checks the view and returns a zero anchor when the view is gone).
BTAnchor bibleTextMacCaptureAnchor(void) {
    __block BTAnchor out = {0, 0, 0, 0};
    dispatch_block_t block = ^{
        if (gTextView == nil || gScroll == nil) return;
        NSTextView *tv = gTextView;
        NSLayoutManager *lm = tv.layoutManager;
        NSTextStorage *ts = tv.textStorage;
        if (ts.length == 0) return;
        out.ok = 1; // the live scroll was readable (even if it's at the top)
        CGFloat offY = tv.visibleRect.origin.y;
        if (offY <= 0.5) return; // at the top → zero anchor
        CGFloat insetH = tv.textContainerInset.height;
        CGFloat docH = [lm usedRectForTextContainer:tv.textContainer].size.height + insetH * 2;
        CGFloat viewH = tv.visibleRect.size.height;
        CGFloat scrollable = docH - viewH;
        if (scrollable > 1) {
            CGFloat f = offY / scrollable;
            if (f < 0) f = 0;
            if (f > 1) f = 1;
            out.frac = f;
        }
        CGFloat tcY = offY - insetH + 2;
        if (tcY < 0) tcY = 0;
        NSUInteger gi = [lm glyphIndexForPoint:NSMakePoint(4, tcY) inTextContainer:tv.textContainer];
        NSUInteger ci = [lm characterIndexForGlyphAtIndex:gi];
        NSUInteger loc = 0;
        NSInteger verse = btMacVerseAtIndex(ts, ci, &loc);
        if (verse <= 0) return;
        NSRange g = [lm glyphRangeForCharacterRange:NSMakeRange(loc, 1) actualCharacterRange:NULL];
        NSRect rr = [lm boundingRectForGlyphRange:g inTextContainer:tv.textContainer];
        out.verse = (int)verse;
        out.delta = offY - (rr.origin.y + insetH);
    };
    if ([NSThread isMainThread]) block();
    else dispatch_sync(dispatch_get_main_queue(), block);
    return out;
}

// bibleTextMacArmRestore stashes a one-shot scroll target consumed by
// bibleTextMacScrollTV on the next layout. verse<=0 && frac<=0 disarms.
void bibleTextMacArmRestore(int verse, double delta, double frac) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (verse <= 0 && frac <= 0) {
            gMacHasRestore = NO;
            gMacRestoreVerse = 0; gMacRestoreDelta = 0; gMacRestoreFrac = 0;
            return;
        }
        gMacRestoreVerse = verse;
        gMacRestoreDelta = delta;
        gMacRestoreFrac = frac;
        gMacHasRestore = YES;
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
	state.hideReadingOverlay = func() { C.bibleTextMacTVSuppress() }
	state.showReadingOverlay = func() {
		C.bibleTextMacTVUnsuppress()
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
		C.bibleTextMacTVShow()
	} else {
		C.bibleTextMacTVHide()
	}
}

func hideNativeReadingOverlayMac() { C.bibleTextMacTVHide() }

// nativeShareText / nativeShareImage present the macOS share sheet for the
// selection-menu Share actions (see share.go).
func nativeShareText(s string) {
	c := C.CString(s)
	defer C.free(unsafe.Pointer(c))
	C.bibleTextShareText(c)
}

func nativeShareImage(path string) {
	c := C.CString(path)
	defer C.free(unsafe.Pointer(c))
	C.bibleTextShareImageFile(c)
}

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
	// Arm any pending one-shot scroll restore for this chapter (reopening where
	// the reader left off) before pushing the text, so bibleTextMacScrollTV lands
	// on the saved position rather than the top. A normal push disarms it.
	armPendingRestore(state)
	// Skip the HTML rebuild + NSAttributedString re-import when the NSTextView
	// already holds this exact chapter render (mirrors the iOS gate in
	// pushChapterHTML); a pending scroll restore forces the push. SetHTML consumes
	// the C string synchronously, so freeing right after the call is safe.
	fp := chapterRenderFingerprint(state)
	if state.restore != nil || fp != lastPushedChapterFP {
		lastPushedChapterFP = fp
		html := buildChapterHTML(state, verses)
		c := C.CString(html)
		C.bibleTextMacTVSetHTML(c)
		C.free(unsafe.Pointer(c))
	}
	// Push the frame so the (possibly already-populated) text view shows.
	C.bibleTextMacTVShow()
	return h
}

// captureReadingAnchor / armReadingRestore bridge the reading-position restore
// (reading_state.go) to the native NSTextView scroll machinery.
func captureReadingAnchor() (verse int, delta, frac float64, ok bool) {
	a := C.bibleTextMacCaptureAnchor()
	return int(a.verse), float64(a.delta), float64(a.frac), a.ok != 0
}

func armReadingRestore(verse int, delta, frac float64) {
	C.bibleTextMacArmRestore(C.int(verse), C.double(delta), C.double(frac))
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
	C.bibleTextMacTVSetFrame(
		C.double(pos.X), C.double(pos.Y),
		C.double(sz.Width), C.double(sz.Height),
	)
}
