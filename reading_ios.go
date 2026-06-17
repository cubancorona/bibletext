//go:build ios

package bibletext

// Native-iOS reading pane: a real UITextView (isEditable=NO, isSelectable=YES)
// is attached to the Fyne app's UIWindow as an overlay. The user gets the full
// native iOS reading experience — character-level drag selection across paragraphs,
// the loupe magnifier, the system context menu with Copy / Look Up / Share /
// Translate, and inertial scrolling — none of which Fyne's RichText or Entry
// widgets can provide.
//
// The Fyne side keeps a transparent placeholder widget so the layout reserves
// the right rectangle; every Resize/Move pushes that rectangle to the
// UITextView frame. Tab switches show/hide the overlay (see ui_mobile.go).

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework UIKit -framework Foundation -framework CoreGraphics

#import <UIKit/UIKit.h>

// Implemented in Go (ai_menu_darwin.go, //export). Called when the reader picks
// an AI study action; it copies both strings immediately, so passing the
// transient UTF8String pointers is safe.
extern void bibleTextAIMenuTapped(char *action, char *text);
// Sibling callback for the non-AI selection-menu actions (Share verse, …).
extern void bibleTextStudyMenuTapped(char *action, char *text);
// Called when the reader finishes scrolling, to persist the live scroll position.
// iOS's app-background lifecycle hook is unreliable, so we save continuously
// (on scroll-end) instead, keeping the saved position current even on a hard kill.
extern void bibleTextReadingScrolled(void);

// --- Reading-position restore -------------------------------------------------
// A one-shot scroll target applied when reopening into the last-read chapter
// (see reading_state.go). Declared before the text-view class so its
// scrollViewDidScroll can disarm the restore the moment the user takes over.
// `ok` distinguishes "read the live scroll" (1, even when at the top) from
// "couldn't read it — view gone" (0).
typedef struct { int verse; double delta; double frac; int ok; } BTAnchor;

static NSInteger gReadingRestoreVerse = 0;
static CGFloat   gReadingRestoreDelta = 0;
static CGFloat   gReadingRestoreFrac  = 0;
static BOOL      gReadingHasRestore   = NO;

// HBReadingTextView adds a "Study with AI" submenu (Explain / Analyze context /
// Analyze translation) to the standard selection menu and hands the selected
// text to Go. It's its own delegate so it can implement the iOS 16+ menu hook.
@interface HBReadingTextView : UITextView <UITextViewDelegate>
@end

@implementation HBReadingTextView

- (UIMenu *)textView:(UITextView *)textView
    editMenuForTextInRange:(NSRange)range
          suggestedActions:(NSArray<UIMenuElement *> *)suggestedActions API_AVAILABLE(ios(16.0)) {
    if (range.length == 0 || NSMaxRange(range) > textView.text.length) {
        return [UIMenu menuWithChildren:suggestedActions];
    }
    NSString *captured = [[textView.text substringWithRange:range] copy];

    UIAction * (^make)(NSString *, NSString *) = ^UIAction *(NSString *title, NSString *act) {
        return [UIAction actionWithTitle:title image:nil identifier:nil
                                 handler:^(__kindof UIAction *_Nonnull a) {
            bibleTextAIMenuTapped((char *)act.UTF8String, (char *)captured.UTF8String);
        }];
    };

    UIMenu *ai = [UIMenu menuWithTitle:@"Study with AI" image:nil identifier:nil options:0
                              children:@[
                                  make(@"Explain", @"explain"),
                                  make(@"Analyze context", @"context"),
                                  make(@"Analyze translation", @"translation"),
                              ]];

    // Share verse → with citation / as image (both go to the iOS share sheet).
    UIAction * (^study)(NSString *, NSString *) = ^UIAction *(NSString *title, NSString *act) {
        return [UIAction actionWithTitle:title image:nil identifier:nil
                                 handler:^(__kindof UIAction *_Nonnull a) {
            bibleTextStudyMenuTapped((char *)act.UTF8String, (char *)captured.UTF8String);
        }];
    };
    UIMenu *share = [UIMenu menuWithTitle:@"Share verse" image:nil identifier:nil options:0
                                 children:@[
                                     study(@"Share with citation", @"share-cite"),
                                     study(@"Share as image", @"share-image"),
                                 ]];
    UIAction *xref = study(@"Cross-references", @"crossref");

    // Now that the text view is hosted in a view controller (see bibleTextEnsureTV),
    // the system's suggestedActions — Copy, Look Up, Translate, Define — present
    // correctly instead of crashing, and the ▸ overflow + our submenus navigate
    // properly. Lead with "Study with AI" (the flagship), then the standard system
    // actions, then our secondary study/share actions.
    NSArray *tail = [suggestedActions arrayByAddingObjectsFromArray:@[xref, share]];
    return [UIMenu menuWithChildren:[@[ai] arrayByAddingObjectsFromArray:tail]];
}

// When the user drags the chapter, drop any pending restore target so the
// reopen-position logic stops re-pinning and the reader scrolls freely. A
// programmatic contentOffset change (our own restore) sets neither flag.
- (void)scrollViewDidScroll:(UIScrollView *)scrollView {
    if (scrollView.dragging || scrollView.decelerating) {
        gReadingHasRestore = NO;
    }
}

// Persist the reading position whenever the user finishes scrolling, so the
// saved spot is always current (the iOS background lifecycle hook is unreliable).
- (void)scrollViewDidEndDecelerating:(UIScrollView *)scrollView {
    bibleTextReadingScrolled();
}
- (void)scrollViewDidEndDragging:(UIScrollView *)scrollView willDecelerate:(BOOL)decelerate {
    if (!decelerate) bibleTextReadingScrolled();
}

@end

// One persistent UITextView attached to the app's main window. We never
// destroy it during the app lifetime — easier to manage than re-attaching,
// and the iOS selection state stays alive across chapter changes.
static UITextView *gReadingTV = nil;

// Character range of the highlighted verse (set when arriving from a search
// result), or {NSNotFound, 0} for a plain chapter view. bibleTextScrollReadingTV
// uses it to land the highlighted verse near the top instead of scrolling to
// the chapter's first verse.
static NSRange gReadingHighlightRange = {NSNotFound, 0};

// gReadingSuppressed is raised while a Fyne modal (chapter picker, AI panel, AI
// settings) is open. The UITextView floats above the whole Fyne canvas, so it
// must stay down for the duration of the modal — not merely be hidden once. A
// layout pass behind the modal can call bibleTextTVShow again, which would paint
// the verses back over the popup and steal its touches. While suppressed, Show
// is a no-op; only Unsuppress clears it.
static BOOL gReadingSuppressed = NO;

// Verse numbers are the only small-font runs in the chapter: buildChapterHTML
// renders them as <sup class="v"> at font-size 0.66em (~12.5px) while body text
// is 19px. Detecting them by font size (rather than a superscript attribute,
// which UIKit doesn't expose) works uniformly and the run's text is the digits.
#define BT_VERSE_FONT_MAX 15.0

// btIOSLocForVerse returns the character location of `verse`'s number run, or
// NSNotFound.
static NSUInteger btIOSLocForVerse(NSTextStorage *ts, NSInteger verse) {
    __block NSUInteger found = NSNotFound;
    [ts enumerateAttribute:NSFontAttributeName
                   inRange:NSMakeRange(0, ts.length)
                   options:0
                usingBlock:^(id val, NSRange r, BOOL *stop) {
        if (val == nil || r.length == 0 || ((UIFont *)val).pointSize >= BT_VERSE_FONT_MAX) return;
        if ([[ts.string substringWithRange:r] integerValue] == verse) {
            found = r.location;
            *stop = YES;
        }
    }];
    return found;
}

// btIOSVerseAtIndex returns the verse whose number run is the last at or before
// character index ci (the top-visible verse), writing its location to *outLoc.
static NSInteger btIOSVerseAtIndex(NSTextStorage *ts, NSUInteger ci, NSUInteger *outLoc) {
    __block NSInteger verse = 0;
    __block NSUInteger loc = 0;
    [ts enumerateAttribute:NSFontAttributeName
                   inRange:NSMakeRange(0, ts.length)
                   options:0
                usingBlock:^(id val, NSRange r, BOOL *stop) {
        if (r.location > ci) { *stop = YES; return; }
        if (val == nil || r.length == 0 || ((UIFont *)val).pointSize >= BT_VERSE_FONT_MAX) return;
        NSInteger n = [[ts.string substringWithRange:r] integerValue];
        if (n > 0) { verse = n; loc = r.location; }
    }];
    if (outLoc) *outLoc = loc;
    return verse;
}

// bibleTextScrollReadingTV positions the chapter, in priority order: the
// highlighted verse (a search jump), then a one-shot restore target (reopening
// where the reader left off), otherwise pinned to the top. Centralised so the
// several places that re-assert the offset (after setText, after a frame push,
// and on deferred ticks) all agree.
static void bibleTextScrollReadingTV(void) {
    if (gReadingTV == nil) return;
    NSUInteger len = gReadingTV.textStorage.length;
    if (gReadingHighlightRange.location != NSNotFound &&
        gReadingHighlightRange.length > 0 &&
        NSMaxRange(gReadingHighlightRange) <= len) {
        NSLayoutManager *lm = gReadingTV.layoutManager;
        NSRange glyphs = [lm glyphRangeForCharacterRange:gReadingHighlightRange
                                    actualCharacterRange:NULL];
        CGRect rect = [lm boundingRectForGlyphRange:glyphs
                                    inTextContainer:gReadingTV.textContainer];
        // A little breathing room above the verse so it doesn't kiss the top.
        CGFloat target = rect.origin.y + gReadingTV.textContainerInset.top - 16;
        CGFloat maxY = gReadingTV.contentSize.height - gReadingTV.bounds.size.height;
        if (target > maxY) target = maxY;
        if (target < 0) target = 0;
        gReadingTV.contentOffset = CGPointMake(0, target);
        return;
    }
    if (gReadingHasRestore && len > 0) {
        UITextView *tv = gReadingTV;
        NSLayoutManager *lm = tv.layoutManager;
        CGFloat insetTop = tv.textContainerInset.top;
        CGFloat target = -1;
        if (gReadingRestoreVerse > 0) {
            NSUInteger loc = btIOSLocForVerse(tv.textStorage, gReadingRestoreVerse);
            if (loc != NSNotFound) {
                NSRange g = [lm glyphRangeForCharacterRange:NSMakeRange(loc, 1)
                                       actualCharacterRange:NULL];
                CGRect rr = [lm boundingRectForGlyphRange:g inTextContainer:tv.textContainer];
                target = rr.origin.y + insetTop + gReadingRestoreDelta;
            }
        }
        if (target < 0 && gReadingRestoreFrac > 0) {
            CGFloat scrollable = tv.contentSize.height - tv.bounds.size.height;
            if (scrollable > 0) target = gReadingRestoreFrac * scrollable;
        }
        if (target >= 0) {
            CGFloat maxY = tv.contentSize.height - tv.bounds.size.height;
            if (target > maxY) target = maxY;
            if (target < 0) target = 0;
            tv.contentOffset = CGPointMake(0, target);
            return;
        }
    }
    gReadingTV.contentOffset = CGPointMake(0, -gReadingTV.adjustedContentInset.top);
}

// Look up the foreground UIWindow that Fyne renders into. iOS 13+ uses scenes;
// pre-13 we fall back to the deprecated keyWindow. Fyne's mobile driver creates
// exactly one window, so the first one we find is the right one.
static UIWindow *bibleTextFindWindow(void) {
    if (@available(iOS 13.0, *)) {
        NSSet<UIScene*> *scenes = UIApplication.sharedApplication.connectedScenes;
        for (UIScene *scene in scenes) {
            if ([scene isKindOfClass:[UIWindowScene class]]) {
                UIWindowScene *ws = (UIWindowScene *)scene;
                if (ws.windows.count > 0) {
                    return ws.windows.firstObject;
                }
            }
        }
    }
    return UIApplication.sharedApplication.keyWindow;
}

// Ensure the UITextView exists and is parented to the current window. Called
// from every public entry point so we recover if iOS recreated the window
// (e.g. after backgrounding+foregrounding the app on a real device).
static void bibleTextEnsureTV(void) {
    dispatch_block_t block = ^{
        UIWindow *win = bibleTextFindWindow();
        if (win == nil) {
            NSLog(@"bibletext: ensureTV — no UIWindow yet");
            return;
        }
        if (gReadingTV == nil) {
            HBReadingTextView *tv = [[HBReadingTextView alloc] init];
            tv.delegate = tv; // its own delegate for the AI menu hook
            tv.editable = NO;
            tv.selectable = YES;
            tv.scrollEnabled = YES;
            tv.alwaysBounceVertical = YES;
            tv.backgroundColor = UIColor.clearColor;
            tv.textContainerInset = UIEdgeInsetsMake(14, 16, 14, 16);
            // Stop iOS from auto-adjusting the content inset for the safe
            // area — we already position the textView below it via the Fyne
            // layout, and the auto-adjust would push verse 1 off the top.
            tv.contentInsetAdjustmentBehavior = UIScrollViewContentInsetAdjustmentNever;
            // Start visible — the Read tab is selected at app launch and
            // AppTabs.OnSelected doesn't fire for the initial selection.
            tv.hidden = NO;
            gReadingTV = tv;
            NSLog(@"bibletext: ensureTV — created UITextView, attaching to window %@", win);
        }
        // Host the text view inside the root view controller's view, NOT the bare
        // window. The iOS edit menu walks the text view's responder chain to find a
        // UIViewController to present from — its overflow (▸) page, its submenus,
        // and the system actions (Look Up / Translate / Define) all need one. A bare
        // window subview has no VC in its chain, so the overflow and submenus do
        // nothing and the system actions CRASH. rootViewController.view is full
        // window, so the frame math (window coords + safe-area inset) is unchanged.
        UIView *host = win.rootViewController.view ?: win;
        if (gReadingTV.superview != host) {
            [gReadingTV removeFromSuperview];
            [host addSubview:gReadingTV];
        }
        [host bringSubviewToFront:gReadingTV];
    };
    if ([NSThread isMainThread]) {
        block();
    } else {
        dispatch_sync(dispatch_get_main_queue(), block);
    }
}

void bibleTextTVSetHTML(const char *html) {
    if (html == NULL) return;
    NSString *s = [NSString stringWithUTF8String:html];
    NSData *data = [s dataUsingEncoding:NSUTF8StringEncoding];
    NSUInteger len = data.length;
    dispatch_async(dispatch_get_main_queue(), ^{
        bibleTextEnsureTV();
        if (gReadingTV == nil) return;
        NSError *err = nil;
        NSDictionary *opts = @{
            NSDocumentTypeDocumentAttribute: NSHTMLTextDocumentType,
            NSCharacterEncodingDocumentAttribute: @(NSUTF8StringEncoding),
        };
        NSAttributedString *as = [[NSAttributedString alloc]
                                    initWithData:data
                                         options:opts
                              documentAttributes:nil
                                           error:&err];
        if (as == nil) {
            NSLog(@"bibletext: HTML parse failed (input %lu bytes): %@", (unsigned long)len, err);
            gReadingTV.text = s;
            return;
        }
        // NSAttributedString's HTML importer routinely injects a non-zero
        // paragraphSpacingBefore (and sometimes a minimumLineHeight) on the
        // FIRST paragraph that no CSS can override — leaving an ugly
        // ~100pt empty band before verse 1. Walk the string and zero those
        // paragraph-style fields so the chapter actually starts where the
        // UITextView frame top is.
        NSMutableAttributedString *mas = [as mutableCopy];
        [mas enumerateAttribute:NSParagraphStyleAttributeName
                        inRange:NSMakeRange(0, mas.length)
                        options:0
                     usingBlock:^(id v, NSRange r, BOOL *stop) {
            if (v == nil) return;
            NSMutableParagraphStyle *ps = [(NSParagraphStyle*)v mutableCopy];
            ps.paragraphSpacingBefore = 0;
            [mas addAttribute:NSParagraphStyleAttributeName value:ps range:r];
        }];
        as = mas;
        // Find the highlighted verse (the .hl span becomes a background-coloured
        // run) so we can scroll to it rather than the top when arriving from a
        // search result. First background run wins — there's only ever one.
        gReadingHighlightRange = (NSRange){NSNotFound, 0};
        [as enumerateAttribute:NSBackgroundColorAttributeName
                       inRange:NSMakeRange(0, as.length)
                       options:0
                    usingBlock:^(id value, NSRange range, BOOL *stop) {
            if (value != nil) {
                gReadingHighlightRange = range;
                *stop = YES;
            }
        }];
        gReadingTV.attributedText = as;
        // Aggressive re-assert of the scroll position: UITextView may relayout
        // after attributedText is set, the Fyne side pushes a new frame just
        // after, and either can shift the offset. Land it on this tick, the next
        // runloop tick, and a ~200ms tick to outlast the slowest re-layout.
        [gReadingTV layoutIfNeeded];
        bibleTextScrollReadingTV();
        dispatch_async(dispatch_get_main_queue(), ^{
            bibleTextScrollReadingTV();
        });
        dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(0.2 * NSEC_PER_SEC)),
                       dispatch_get_main_queue(), ^{
            bibleTextScrollReadingTV();
        });
    });
}

void bibleTextTVSetFrame(float x, float y, float w, float h) {
    dispatch_async(dispatch_get_main_queue(), ^{
        bibleTextEnsureTV();
        if (gReadingTV == nil) return;
        // Fyne renders its canvas inset below the device safe area (Dynamic
        // Island / status bar on top, home indicator on the bottom), so a Fyne
        // coordinate of Y maps to window Y + safeAreaInsets.top. The UITextView
        // is a direct window subview using raw window coordinates, so we must
        // add the same insets or the text floats up over the chapter header.
        UIEdgeInsets safe = UIEdgeInsetsZero;
        if (gReadingTV.superview != nil) {
            safe = gReadingTV.superview.safeAreaInsets;
        }
        CGRect r = CGRectMake(x + safe.left, y + safe.top, w, h);
        BOOL changed = !CGRectEqualToRect(r, gReadingTV.frame);
        gReadingTV.frame = r;
        // When the frame changes, the UITextView re-lays out the text. If the
        // previous chapter happened to scroll mid-paragraph, the offset can
        // carry over and land in the middle of the new chapter. Re-assert the
        // intended position (top, or the highlighted verse on a search jump);
        // layoutIfNeeded first so the glyph geometry matches the new width.
        if (changed) {
            [gReadingTV layoutIfNeeded];
            bibleTextScrollReadingTV();
            dispatch_async(dispatch_get_main_queue(), ^{
                bibleTextScrollReadingTV();
            });
        }
    });
}

void bibleTextTVShow(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (gReadingSuppressed) return; // a modal is up; stay down until released
        bibleTextEnsureTV();
        if (gReadingTV == nil) return;
        gReadingTV.hidden = NO;
        [gReadingTV.superview bringSubviewToFront:gReadingTV];
    });
}

// bibleTextDismissMenu takes down any active text-selection edit menu. The
// UITextView floats above the Fyne canvas, so its edit menu is a SEPARATE UIKit
// element: hiding/suppressing the text view does NOT remove the menu, which would
// otherwise keep floating on screen — orphaned — and stack up as the user selects
// again (a Fyne modal opening on an AI/cross-ref action is the common trigger).
// resignFirstResponder clears the selection and takes the menu down with it
// (UITextView exposes no editMenuInteraction to dismiss directly); the view
// becomes first responder again on the next tap when it's shown.
static void bibleTextDismissMenu(void) {
    if (gReadingTV == nil) return;
    [gReadingTV resignFirstResponder];
}

void bibleTextTVHide(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (gReadingTV == nil) return;
        bibleTextDismissMenu();
        gReadingTV.hidden = YES;
    });
}

// bibleTextTVSuppress hides the overlay and latches it down so any stray
// bibleTextTVShow from a layout pass behind a modal is ignored.
void bibleTextTVSuppress(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        gReadingSuppressed = YES;
        if (gReadingTV == nil) return;
        bibleTextDismissMenu();
        gReadingTV.hidden = YES;
    });
}

// bibleTextTVUnsuppress clears the latch. It does not show the overlay on its
// own — the caller decides whether to show (reading) or keep hidden (search).
void bibleTextTVUnsuppress(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        gReadingSuppressed = NO;
    });
}

// --- Share -----------------------------------------------------------------
// Present the iOS share sheet (UIActivityViewController) for "Share with
// citation" / "Share as image". On iPad the sheet is a popover, so anchor it at
// the current selection rect.
static UIViewController *bibleTextTopVC(void) {
    UIWindow *win = bibleTextFindWindow();
    UIViewController *vc = win.rootViewController;
    while (vc.presentedViewController != nil) vc = vc.presentedViewController;
    return vc;
}

static void bibleTextPresentShare(NSArray *items) {
    if (items.count == 0) return;
    UIViewController *top = bibleTextTopVC();
    if (top == nil) return;
    UIActivityViewController *av =
        [[UIActivityViewController alloc] initWithActivityItems:items applicationActivities:nil];
    if (av.popoverPresentationController != nil && gReadingTV != nil) {
        av.popoverPresentationController.sourceView = gReadingTV;
        CGRect r = CGRectZero;
        if (gReadingTV.selectedTextRange != nil) {
            r = [gReadingTV firstRectForRange:gReadingTV.selectedTextRange];
        }
        if (CGRectIsEmpty(r) || CGRectIsNull(r)) {
            r = CGRectMake(CGRectGetMidX(gReadingTV.bounds), CGRectGetMidY(gReadingTV.bounds), 1, 1);
        }
        av.popoverPresentationController.sourceRect = r;
    }
    [top presentViewController:av animated:YES completion:nil];
}

void bibleTextShareText(const char *text) {
    if (text == NULL) return;
    NSString *s = [NSString stringWithUTF8String:text];
    dispatch_async(dispatch_get_main_queue(), ^{
        if (s.length == 0) return;
        bibleTextPresentShare(@[s]);
    });
}

void bibleTextShareImageFile(const char *path) {
    if (path == NULL) return;
    NSString *p = [NSString stringWithUTF8String:path];
    dispatch_async(dispatch_get_main_queue(), ^{
        UIImage *img = [UIImage imageWithContentsOfFile:p];
        if (img == nil) return;
        bibleTextPresentShare(@[img]);
    });
}

// --- Reading-position capture / restore (Go bridge) -------------------------

// bibleTextTVCaptureAnchor reads the current scroll position as a verse anchor
// (top-visible verse + within-verse delta) plus a whole-chapter fraction
// fallback. Synchronous on the main thread; null-safe during teardown.
BTAnchor bibleTextTVCaptureAnchor(void) {
    __block BTAnchor out = {0, 0, 0, 0};
    dispatch_block_t block = ^{
        if (gReadingTV == nil) return;
        UITextView *tv = gReadingTV;
        NSLayoutManager *lm = tv.layoutManager;
        NSTextStorage *ts = tv.textStorage;
        if (ts.length == 0) return;
        out.ok = 1; // the live scroll was readable (even if it's at the top)
        CGFloat offY = tv.contentOffset.y;
        if (offY <= 0.5) return; // at the top → zero anchor
        CGFloat insetTop = tv.textContainerInset.top;
        CGFloat scrollable = tv.contentSize.height - tv.bounds.size.height;
        if (scrollable > 1) {
            CGFloat f = offY / scrollable;
            if (f < 0) f = 0;
            if (f > 1) f = 1;
            out.frac = f;
        }
        CGFloat tcY = offY - insetTop + 2;
        if (tcY < 0) tcY = 0;
        NSUInteger gi = [lm glyphIndexForPoint:CGPointMake(4, tcY) inTextContainer:tv.textContainer];
        NSUInteger ci = [lm characterIndexForGlyphAtIndex:gi];
        NSUInteger loc = 0;
        NSInteger verse = btIOSVerseAtIndex(ts, ci, &loc);
        if (verse <= 0) return;
        NSRange g = [lm glyphRangeForCharacterRange:NSMakeRange(loc, 1) actualCharacterRange:NULL];
        CGRect rr = [lm boundingRectForGlyphRange:g inTextContainer:tv.textContainer];
        out.verse = (int)verse;
        out.delta = offY - (rr.origin.y + insetTop);
    };
    if ([NSThread isMainThread]) block();
    else dispatch_sync(dispatch_get_main_queue(), block);
    return out;
}

// bibleTextTVArmRestore stashes a one-shot scroll target consumed by
// bibleTextScrollReadingTV on the next layout. verse<=0 && frac<=0 disarms.
void bibleTextTVArmRestore(int verse, double delta, double frac) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (verse <= 0 && frac <= 0) {
            gReadingHasRestore = NO;
            gReadingRestoreVerse = 0; gReadingRestoreDelta = 0; gReadingRestoreFrac = 0;
            return;
        }
        gReadingRestoreVerse = verse;
        gReadingRestoreDelta = delta;
        gReadingRestoreFrac = frac;
        gReadingHasRestore = YES;
    });
}
*/
import "C"

import (
	"fmt"
	"image/color"
	"time"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// buildReadingViewMobile constructs the iOS reading pane.
//
// The Fyne-managed part is small: the standard header (history bar, chapter
// header, back-to-results when applicable) plus a nativeReadingHost widget
// that reserves space for the UITextView. The verse text itself is rendered
// by the UITextView overlay — Fyne renders nothing in that rectangle.
func buildReadingViewMobile(state *AppState) fyne.CanvasObject {
	pal := state.pal()

	// Let shared popups (the chapter picker) hide/show the native overlay so it
	// doesn't float over them. Idempotent — safe to set on every rebuild.
	state.hideReadingOverlay = func() { C.bibleTextTVSuppress() }
	state.showReadingOverlay = func() {
		C.bibleTextTVUnsuppress()
		// Restore only the overlay that belongs to the current view (reading,
		// not search results or another tab) — same invariant as every other
		// visibility decision.
		if overlayShouldShow(state) {
			C.bibleTextTVShow()
		} else {
			C.bibleTextTVHide()
		}
	}

	chapterNumbers := state.Bible.GetChapterNumbersForBook(state.CurrentBook)
	normalizeCurrentChapter(state, chapterNumbers)
	verses := state.Bible.GetChapter(state.CurrentBook, state.CurrentChapter)

	host := newNativeReadingHost(state, verses)

	paper := canvas.NewRectangle(pal.Surface)
	paper.StrokeColor = pal.Border
	paper.StrokeWidth = 1
	paper.CornerRadius = 8

	// Full-screen reading: no chrome at all except a small exit affordance.
	// Tabs and the top "BibleText" header are skipped in ui_mobile.go for this
	// case, so the UITextView fills almost the whole device screen.
	if state.IsFullScreen {
		exit := widget.NewButtonWithIcon("", theme.ViewRestoreIcon(), func() {
			state.IsFullScreen = false
			rebuildWindow(state)
		})
		exit.Importance = widget.LowImportance
		exitRow := container.NewBorder(nil, nil, nil, exit, nil)
		body := container.NewBorder(exitRow, nil, nil, nil, container.NewStack(paper, host))
		return container.NewPadded(body)
	}

	top := container.NewVBox()
	if bar := buildHistoryBar(state); bar != nil {
		top.Add(bar)
	}
	if state.CanReturnToSearchResults {
		top.Add(backToResultsBar(state))
	}
	top.Add(chapterHeaderMobile(state, chapterNumbers))

	body := container.NewBorder(top, nil, nil, nil, container.NewStack(paper, host))
	return container.NewPadded(body)
}

// chapterHeaderMobile is a compact, low-chrome chapter toolbar tuned for the
// mobile reading view. The book heading carries the current chapter number
// ("John 1") with a small inline copy icon; the muted chapter line below it
// (tappable to open the picker) carries the prev/next chapter arrows, so all
// the chapter navigation clusters next to the book + chapter text. Full-screen
// is the lone control on the right.
//
//	┌─────────────────────────────────────────────────────┐
//	│ John 1 ⧉                                       ⤢    │
//	│ Chapter 1 of 21 ▾   ←  →                            │
//	└─────────────────────────────────────────────────────┘
func chapterHeaderMobile(state *AppState, chapterNumbers []int) fyne.CanvasObject {
	pal := state.pal()
	total := len(chapterNumbers)

	// "John 10 ⌄" — one cohesive tap target (text + a clear dropdown chevron) that
	// opens the combined reference picker (book list + chapter grid). A roomy box
	// height makes it a comfortable touch target.
	const titleBoxH = 40
	ref := newReferenceButton(fmt.Sprintf("%s %d", state.CurrentBook, state.CurrentChapter), pal.Text, headingTextSize, titleBoxH, func() {
		showReferencePicker(state)
	})

	// Small copy icon tucked after the heading — lighter than the chapter-nav
	// arrows but still a full-height (finger-friendly) hit box.
	copyBtn := newIconTapButton(state, theme.ContentCopyIcon(), 16, titleBoxH, func() {
		copyChapter(state)
	})
	titleRow := container.NewHBox(ref, hgap(6), copyBtn)

	// Chapter arrows get the full 44pt touch target so they're easy to tap.
	navBoxH := minTapTarget

	// Quiet chapter context below the heading — also a picker target, so the
	// whole "Chapter N of M" line opens the picker too.
	chapText := fmt.Sprintf("Chapter %d of %d", state.CurrentChapter, total)
	if total <= 1 {
		chapText = fmt.Sprintf("Chapter %d", state.CurrentChapter)
	}
	chapterLine := newTapTextStyled(chapText, pal.TextMuted, subheadingTextSize, navBoxH, false, func() {
		showReferencePicker(state)
	})

	idx := indexOf(chapterNumbers, state.CurrentChapter)

	// Prev/next as compact icon buttons sitting next to the chapter line, so
	// they're close to the book + chapter text rather than floating far right.
	prev := newIconTapButton(state, theme.NavigateBackIcon(), 20, navBoxH, func() {
		if moveChapter(state, -1) {
			state.refresh()
		}
	})
	prev.disabled = idx <= 0

	next := newIconTapButton(state, theme.NavigateNextIcon(), 20, navBoxH, func() {
		if moveChapter(state, 1) {
			state.refresh()
		}
	})
	next.disabled = idx < 0 || idx >= total-1

	// Controls sit directly in the HBox so the picker anchor keeps a first-class
	// hit box (a nested spacer-VBox left it unresponsive to taps on iOS).
	chapterRow := container.NewHBox(chapterLine, hgap(8), prev, next)

	// Full-screen is the lone control on the right.
	fullScreenBtn := widget.NewButtonWithIcon("", theme.ViewFullScreenIcon(), func() {
		state.IsFullScreen = true
		rebuildWindow(state)
	})
	fullScreenBtn.Importance = widget.LowImportance

	left := container.NewVBox(titleRow, chapterRow)
	right := container.NewVBox(layout.NewSpacer(), fullScreenBtn, layout.NewSpacer())
	row := container.NewBorder(nil, nil, left, right, nil)

	rule := canvas.NewLine(pal.Border)
	rule.StrokeWidth = 1
	return container.NewVBox(row, rule)
}

// afterRebuild (iOS) re-pins the native UITextView overlay after the window
// tree is swapped. Fyne re-lays out on its own schedule, and a few Resize/Move
// calls can fire with intermediate values before the tree settles, so we post
// a deferred re-push to land the overlay on the new host's settled rect —
// important across full-screen transitions where the rect changes a lot.
func afterRebuild(state *AppState) {
	time.AfterFunc(150*time.Millisecond, func() {
		fyne.Do(func() {
			// Re-pin the overlay frame only when the reading view is the content
			// on screen; pushing a frame for a stale host while another tab is up
			// would drag the (hidden) overlay to the wrong place.
			if overlayShouldShow(state) && currentHost != nil {
				setFrameFromObject(currentHost)
			}
			// Re-assert visibility LAST so it wins any async show/hide ordering
			// from the rebuild — this is what stops a stray show from leaving the
			// overlay stuck as a black rectangle over the Books/Search tabs.
			notifyReadingOverlay(overlayShouldShow(state))
		})
	})
}

// nativeReadingHost is a Fyne widget that holds no visible content of its own
// — it just reports a minimum size, and on every Resize/Move it projects its
// allocated rectangle into UIKit screen coordinates and pushes that frame to
// the persistent UITextView overlay.
type nativeReadingHost struct {
	widget.BaseWidget
	state *AppState
}

func newNativeReadingHost(state *AppState, verses []Verse) *nativeReadingHost {
	h := &nativeReadingHost{state: state}
	h.ExtendBaseWidget(h)
	currentHost = h
	// Push the HTML into the UITextView right away (it'll appear on the next
	// frame once Resize has set the frame). Doing this in the constructor
	// rather than in CreateRenderer means tab-switches that rebuild the
	// Read tab content also refresh the chapter text.
	pushChapterHTML(state, verses)
	return h
}

// CreateRenderer returns a transparent rectangle — visually we render
// nothing; the UITextView paints the text on top.
func (h *nativeReadingHost) CreateRenderer() fyne.WidgetRenderer {
	r := canvas.NewRectangle(color.Transparent)
	return widget.NewSimpleRenderer(r)
}

// Resize is called by the Fyne layout. We project the host's canvas-relative
// position to UIKit screen coordinates (they're the same on iOS — both use
// logical points starting at the top-left of the window) and push the frame.
func (h *nativeReadingHost) Resize(size fyne.Size) {
	h.BaseWidget.Resize(size)
	h.pushFrame()
}

// Move likewise: when the host's parent re-positions it (e.g. after history
// bar collapses), the UITextView frame must follow.
func (h *nativeReadingHost) Move(p fyne.Position) {
	h.BaseWidget.Move(p)
	h.pushFrame()
}

// pushFrame projects the host's absolute canvas rect to the UITextView frame.
//
// Resize/Move can fire mid-layout — before sibling widgets (the chapter header
// in the Border's `top` slot) have reached their final height — so the
// position we read can be too high, causing the verse text to overlap the
// header. We push immediately (responsive) AND again on the next event-loop
// tick once the whole tree has settled, which lands the final correct frame.
//
// We also have to defend against this race: when the window content is
// replaced (e.g. entering/leaving full-screen mode), Fyne creates a NEW
// nativeReadingHost. The OLD host may still have a scheduled re-push in flight
// that, by the time it fires, would read stale position/size from the now-
// detached widget and overwrite the new host's correct frame. currentHost is
// the latest one; only it gets to push.
func (h *nativeReadingHost) pushFrame() {
	setFrameFromObject(h)
	time.AfterFunc(50*time.Millisecond, func() {
		fyne.Do(func() {
			if currentHost == h {
				setFrameFromObject(h)
			}
		})
	})
}

// currentHost is the most recently constructed nativeReadingHost. Stale
// AfterFunc callbacks from previous hosts compare against this and bail.
var currentHost *nativeReadingHost

func setFrameFromObject(h *nativeReadingHost) {
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(h)
	sz := h.Size()
	if sz.Width <= 0 || sz.Height <= 0 {
		return
	}
	C.bibleTextTVSetFrame(
		C.float(pos.X), C.float(pos.Y),
		C.float(sz.Width), C.float(sz.Height),
	)
}

// Show / Hide are hooked into the tab-switching logic from ui_mobile.go.
func (h *nativeReadingHost) Show() {
	h.BaseWidget.Show()
	C.bibleTextTVShow()
}

func (h *nativeReadingHost) Hide() {
	h.BaseWidget.Hide()
	C.bibleTextTVHide()
}

// showNativeReadingOverlay / hideNativeReadingOverlay are package-level so
// ui_mobile.go's tab-change handler can drive visibility without holding a
// reference to the host widget (which would force a circular dependency for
// every tab change).
func showNativeReadingOverlay() { C.bibleTextTVShow() }
func hideNativeReadingOverlay() { C.bibleTextTVHide() }

// nativeShareText / nativeShareImage present the iOS share sheet for the
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

// pushChapterHTML builds the chapter as HTML (so NSAttributedString gets nice
// inline styling — superscript verse numbers, accent color, serif font) and
// sends it across the CGO boundary.
func pushChapterHTML(state *AppState, verses []Verse) {
	// Arm any pending one-shot scroll restore for this chapter (reopening where
	// the reader left off) before pushing the text, so bibleTextScrollReadingTV
	// lands on the saved position rather than the top. A normal push disarms it.
	// (Done before the skip check below so a pending restore always re-arms.)
	armPendingRestore(state)

	// Skip the costly HTML rebuild + NSAttributedString re-import when the
	// UITextView already holds this exact chapter render (e.g. switching to the
	// Books tab and back, or a refresh that didn't change the text). A pending
	// scroll restore forces the push so the scroll cadence runs. The fingerprint
	// includes highlight + theme so a search-jump or light/dark flip still pushes.
	fp := chapterRenderFingerprint(state)
	if state.restore == nil && fp == lastPushedChapterFP {
		return
	}
	lastPushedChapterFP = fp

	html := buildChapterHTML(state, verses)
	c := C.CString(html)
	defer C.free(unsafe.Pointer(c))
	C.bibleTextTVSetHTML(c)
}

// captureReadingAnchor / armReadingRestore bridge the reading-position restore
// (reading_state.go) to the native UITextView scroll machinery.
func captureReadingAnchor() (verse int, delta, frac float64, ok bool) {
	a := C.bibleTextTVCaptureAnchor()
	return int(a.verse), float64(a.delta), float64(a.frac), a.ok != 0
}

func armReadingRestore(verse int, delta, frac float64) {
	C.bibleTextTVArmRestore(C.int(verse), C.double(delta), C.double(frac))
}

// buildChapterHTML, nrgbaToHex and htmlEscape moved to reading.go so the macOS
// NSTextView overlay shares the exact same chapter HTML as the iOS UITextView.
