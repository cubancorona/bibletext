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
// Called when the reader single-taps a highlighted verse and picks "Clear
// highlight" from the inline native menu. Go clears the highlight state and
// re-renders so the .hl background wash disappears.
extern void bibleTextHighlightCleared(void);
// Called from the soft-keyboard frame observer with the keyboard's on-screen overlap
// (height in points, 0 when hidden). The Goto verse picker lifts its bottom row by this
// to sit exactly above the keyboard.
extern void bibleTextKeyboardChanged(double height);

// --- Reading-position restore -------------------------------------------------
// A one-shot scroll target applied when reopening into the last-read chapter
// (see reading_state.go). Declared before the text-view class so its
// scrollViewDidScroll can disarm the restore the moment the user takes over.
// `ok` distinguishes "read the live scroll" (1, even when at the top) from
// "couldn't read it — view gone" (0).
typedef struct { int verse; double delta; double frac; int ok; } BTAnchor;
// The last scroll's initial-touch point, mapped to a verse. ok=1 only when a verse
// was resolved (the reader scrolled and we located the grabbed line).
typedef struct { int verse; double delta; int ok; } BTTouch;

static NSInteger gReadingRestoreVerse = 0;
static CGFloat   gReadingRestoreDelta = 0;
static CGFloat   gReadingRestoreFrac  = 0;
static BOOL      gReadingHasRestore   = NO;

// gLastTouch* records where the reader's finger first landed on the CURRENT scroll
// (content-y of the initial touch). Set once per gesture in
// scrollViewWillBeginDragging — NOT per frame — and read at scroll-end /
// lifecycle flush as "the last scroll's initial touch". Reset on each chapter
// render (bibleTextApplyHTML) so a touch never carries across chapters. The verse
// mapping is deferred to bibleTextTVCaptureTouch (one place); the content-y is
// stable through a gesture (no re-layout mid-scroll).
static CGFloat gLastTouchContentY = 0;
static BOOL    gHasLastTouch      = NO;

// The "you left off here" marker. On reopen, the last-touched verse's number is
// recoloured in the accent colour until the reader scrolls. gMarkerVerse is the
// intent; once applied, the run [gMarkerLoc, +gMarkerLen) is recoloured and its
// original colour saved (gMarkerOrig*) so it is restored live, with no re-render.
static NSInteger  gMarkerVerse = 0;
static CGFloat    gMarkerR = 0, gMarkerG = 0, gMarkerB = 0;
static NSUInteger gMarkerLoc = 0, gMarkerLen = 0;
static CGFloat    gMarkerOrigR = 0, gMarkerOrigG = 0, gMarkerOrigB = 0, gMarkerOrigA = 1;
static BOOL       gMarkerApplied = NO;
static void btIOSApplyMarker(void); // defined below; used in bibleTextApplyHTML + arm
static void btIOSClearMarker(void); // forward decl: scrollViewDidScroll clears on drag
static NSUInteger btIOSLocForVerse(NSTextStorage *ts, NSInteger verse); // used by btIOSApplyMarker

// Character range of the highlighted verse (set when arriving from a search
// result), or {NSNotFound, 0} for a plain chapter view. bibleTextScrollReadingTV
// lands it near the top; HBReadingTextView's btHighlightTap hit-tests a tap
// against it. Declared up here (before the class) so the methods inside
// @implementation can reference it — the @implementation precedes the rest of the
// statics in this file.
static NSRange gReadingHighlightRange = {NSNotFound, 0};

// The single-tap "clear highlight" recognizer (created in bibleTextEnsureTV). It is
// ENABLED only while a verse is highlighted; during ordinary reading it stays
// disabled, so it never participates in touch handling and can add no latency to
// scrolling. bibleTextApplyHTML toggles it to match gReadingHighlightRange.
static UITapGestureRecognizer *gHighlightTap = nil;

// HBReadingTextView adds a "Study with AI" submenu (Explain / Analyze context /
// Analyze translation) to the standard selection menu and hands the selected
// text to Go. It's its own delegate so it can implement the iOS 16+ menu hook.
@interface HBReadingTextView : UITextView <UITextViewDelegate,
    UIGestureRecognizerDelegate, UIEditMenuInteractionDelegate>
@property (nonatomic, strong) UIEditMenuInteraction *hlMenu API_AVAILABLE(ios(16.0));
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
    UIMenu *share = [UIMenu menuWithTitle:@"Share" image:nil identifier:nil options:0
                                 children:@[
                                     study(@"Share with citation", @"share-cite"),
                                     study(@"Share as image", @"share-image"),
                                 ]];
    UIAction *xref = study(@"Cross-references", @"crossref");

    // Keep the three BibleText actions together as ONE group instead of scattering
    // them before and after the system actions (the old layout led with "Study with
    // AI" but trailed Cross-references + Share after Copy/Look Up/Translate). Order:
    // the standard edit commands (Copy/Cut/Paste) first, then our cluster —
    // Study with AI, Share, Cross-references — then the remaining system actions
    // (Look Up, Translate, Define). If the system hands us no identifiable
    // standard-edit group, our cluster simply leads and the system actions follow
    // (still grouped, never scattered).
    NSMutableArray<UIMenuElement *> *editGroup = [NSMutableArray array];
    NSMutableArray<UIMenuElement *> *systemRest = [NSMutableArray array];
    for (UIMenuElement *el in suggestedActions) {
        if ([el isKindOfClass:[UIMenu class]] &&
            [((UIMenu *)el).identifier isEqualToString:UIMenuStandardEdit]) {
            [editGroup addObject:el];
        } else {
            [systemRest addObject:el];
        }
    }
    NSMutableArray<UIMenuElement *> *children = [NSMutableArray array];
    [children addObjectsFromArray:editGroup];
    [children addObjectsFromArray:@[ai, share, xref]];
    [children addObjectsFromArray:systemRest];
    return [UIMenu menuWithChildren:children];
}

// When the user drags the chapter, drop any pending restore target so the
// reopen-position logic stops re-pinning and the reader scrolls freely. A
// programmatic contentOffset change (our own restore) sets neither flag.
// Record where the finger first grabbed the text at the START of a scroll — the
// content-y of the initial touch. Fires once per gesture (not per frame), so it is
// cheap; the verse mapping is deferred to scroll-end (bibleTextTVCaptureTouch).
- (void)scrollViewWillBeginDragging:(UIScrollView *)scrollView {
    gLastTouchContentY = [scrollView.panGestureRecognizer locationInView:scrollView].y;
    gHasLastTouch = YES;
}

- (void)scrollViewDidScroll:(UIScrollView *)scrollView {
    if (scrollView.dragging || scrollView.decelerating) {
        // Disarm a pending restore the moment the user takes over the scroll. (We do
        // NOT poke the edit menu here — it self-dismisses on scroll, and calling it
        // every scroll frame is needless main-thread work during the gesture.)
        gReadingHasRestore = NO;
        // The "you left off here" marker has served its purpose once the reader
        // starts scrolling — clear it. After the first drag frame gMarkerVerse is 0,
        // so this is a single cheap comparison per frame thereafter.
        if (gMarkerVerse != 0) btIOSClearMarker();
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

// --- Tap a highlighted verse to clear it ------------------------------------
// When the reader arrives at a verse via search "see in context" or verse-of-day,
// it's painted with a background wash (.hl run -> gReadingHighlightRange). A single
// tap on that wash offers a one-item "Clear highlight" menu at the tap point; a tap
// anywhere else (or a scroll) dismisses it without clearing. The menu is a native
// UIEditMenuInteraction, so it floats above the overlay and handles tap-away,
// scroll-dismiss, positioning and theming for us.

// The single item shown when our interaction presents. Returning only our action
// (ignoring suggestedActions) keeps this a clean, single-purpose menu.
- (UIMenu *)editMenuInteraction:(UIEditMenuInteraction *)interaction
           menuForConfiguration:(UIEditMenuConfiguration *)configuration
               suggestedActions:(NSArray<UIMenuElement *> *)suggestedActions
           API_AVAILABLE(ios(16.0)) {
    UIAction *clear = [UIAction actionWithTitle:@"Clear highlight" image:nil identifier:nil
                                        handler:^(__kindof UIAction *a) {
        bibleTextHighlightCleared(); // -> Go: clear + re-render (drops the .hl run)
    }];
    clear.attributes = UIMenuElementAttributesDestructive; // red — signals "removes"
    return [UIMenu menuWithChildren:@[clear]];
}

// Recognize our single tap ALONGSIDE the text view's built-in pan (scroll),
// double-tap (word select) and long-press (loupe) recognizers, so scrolling and
// selection are untouched.
- (BOOL)gestureRecognizer:(UIGestureRecognizer *)g
shouldRecognizeSimultaneouslyWithGestureRecognizer:(UIGestureRecognizer *)other {
    return YES;
}

// Single tap -> if it landed on the highlighted wash, present "Clear highlight" at
// the tap point. Otherwise do nothing (an outside tap also auto-dismisses any menu
// already up — that's the tap-away behaviour, free from UIKit).
- (void)btHighlightTap:(UITapGestureRecognizer *)g {
    if (g.state != UIGestureRecognizerStateEnded) return;
    if (gReadingHighlightRange.location == NSNotFound || gReadingHighlightRange.length == 0) return;
    // Guard a stale range outrunning the text (parity with bibleTextScrollReadingTV's
    // NSMaxRange check) before any glyph-geometry call.
    if (NSMaxRange(gReadingHighlightRange) > self.textStorage.length) return;
    // A live selection means this tap belongs to the system/AI selection menu.
    if (self.selectedTextRange != nil && !self.selectedTextRange.empty) return;

    // locationInView: on the (scroll-view) text view already yields content
    // coordinates (the bounds origin moves with the scroll). Subtract the text
    // container inset (14,16) to reach the layout manager's container space.
    CGPoint p = [g locationInView:self];
    CGPoint inContainer = CGPointMake(p.x - self.textContainerInset.left,
                                      p.y - self.textContainerInset.top);
    NSLayoutManager *lm = self.layoutManager;
    NSRange wg = [lm glyphRangeForCharacterRange:gReadingHighlightRange actualCharacterRange:NULL];
    // Test each line fragment's glyph-tight usedRect (not the union bounding rect),
    // so a multi-line verse never accepts taps in the blank ragged-right margin or
    // the indent gap beside a short final line. A small inset adds tap tolerance.
    __block BOOL hit = NO;
    [lm enumerateLineFragmentsForGlyphRange:wg
                                 usingBlock:^(CGRect rect, CGRect usedRect, NSTextContainer *tc,
                                              NSRange gr, BOOL *stop) {
        if (CGRectContainsPoint(CGRectInset(usedRect, -2, -2), inContainer)) { hit = YES; *stop = YES; }
    }];
    if (!hit) return;

    if (@available(iOS 16.0, *)) {
        UIEditMenuConfiguration *cfg =
            [UIEditMenuConfiguration configurationWithIdentifier:@"btClearHL" sourcePoint:p];
        [self.hlMenu presentEditMenuWithConfiguration:cfg];
    }
}

@end

// One persistent UITextView attached to the app's main window. We never
// destroy it during the app lifetime — easier to manage than re-attaching,
// and the iOS selection state stays alive across chapter changes.
static UITextView *gReadingTV = nil;

// Cached reading "paper" colour (pal.Surface as 0..1 components), so the OPAQUE
// background can be re-asserted right after EVERY attributedText assignment.
// Setting attributedText (and especially the WebKit HTML importer's retry path)
// can land after bibleTextTVSetReadingBG and revert the view toward a transparent,
// non-opaque state — which brings back the per-frame compositor blend and the
// scroll lag. Re-asserting from these cached values inside bibleTextApplyHTML makes
// opaque the LAST word after any content change, immune to the runloop ordering.
static CGFloat gReadingPaperR = 1.0, gReadingPaperG = 1.0, gReadingPaperB = 1.0;
static BOOL gReadingPaperSet = NO;
static void btIOSApplyReadingBG(void); // defined below; used in bibleTextApplyHTML above it

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

// The chapter's verse-number runs, captured once per render as a {verse, charLoc}
// table in document order (so it's sorted by both location and verse number). The
// scroll-end anchor capture binary-searches this instead of re-enumerating the whole
// text storage on every finger-lift — that O(n) main-thread walk landed exactly at
// scroll-settle and was the felt scroll lag (worst on long chapters).
typedef struct { NSInteger verse; NSUInteger loc; NSUInteger len; } BTVerseLoc;
static BTVerseLoc *gVerseIndex = NULL;
static NSUInteger  gVerseIndexCount = 0;

// btIOSBuildVerseIndex captures every verse-number run (the only sub-15pt runs) into
// gVerseIndex. Called on every text assignment; the single buffer is reused for the
// app's life. ts==nil (plain-text fallback) clears the table.
static void btIOSBuildVerseIndex(NSTextStorage *ts) {
    const NSUInteger CAP = 512; // far above any chapter's verse count (max ~176)
    if (gVerseIndex == NULL) gVerseIndex = malloc(CAP * sizeof(BTVerseLoc));
    if (ts == nil) { gVerseIndexCount = 0; return; }
    __block NSUInteger n = 0;
    [ts enumerateAttribute:NSFontAttributeName
                   inRange:NSMakeRange(0, ts.length)
                   options:0
                usingBlock:^(id val, NSRange r, BOOL *stop) {
        if (n >= CAP) { *stop = YES; return; }
        if (val == nil || r.length == 0 || ((UIFont *)val).pointSize >= BT_VERSE_FONT_MAX) return;
        NSInteger v = [[ts.string substringWithRange:r] integerValue];
        if (v > 0) { gVerseIndex[n].verse = v; gVerseIndex[n].loc = r.location; gVerseIndex[n].len = r.length; n++; }
    }];
    gVerseIndexCount = n;
}

// btIOSApplyMarker recolours the marked verse's number run in the accent colour,
// saving the original colour so it can be restored when the reader scrolls. Runs
// once per render (only when a marker is set), on the main thread — cheap. The
// run length comes from the verse index (built just before this).
static void btIOSApplyMarker(void) {
    if (gMarkerVerse <= 0 || gReadingTV == nil) return;
    NSTextStorage *ts = gReadingTV.textStorage;
    NSUInteger loc = btIOSLocForVerse(ts, gMarkerVerse);
    if (loc == NSNotFound) return;
    NSUInteger len = 0;
    for (NSUInteger i = 0; i < gVerseIndexCount; i++) {
        if (gVerseIndex[i].loc == loc) { len = gVerseIndex[i].len; break; }
    }
    if (len == 0 || loc + len > ts.length) return;
    UIColor *orig = [ts attribute:NSForegroundColorAttributeName atIndex:loc effectiveRange:NULL];
    if (orig) { [orig getRed:&gMarkerOrigR green:&gMarkerOrigG blue:&gMarkerOrigB alpha:&gMarkerOrigA]; }
    else { gMarkerOrigR = gMarkerOrigG = gMarkerOrigB = 0; gMarkerOrigA = 1; }
    gMarkerLoc = loc; gMarkerLen = len;
    UIColor *c = [UIColor colorWithRed:gMarkerR green:gMarkerG blue:gMarkerB alpha:1.0];
    [ts addAttribute:NSForegroundColorAttributeName value:c range:NSMakeRange(loc, len)];
    gMarkerApplied = YES;
}

// btIOSClearMarker restores the marked verse number's original colour and forgets
// the marker. Called when the reader takes over the scroll (the resume hint is
// done) — and is a no-op if no marker is currently applied.
//
// gMarkerApplied==YES implies gMarkerLoc/Len are valid for the CURRENT text storage:
// the only thing that replaces textStorage is bibleTextApplyHTML, which resets
// gMarkerApplied=NO before touching it. So the bounds check below never fails while
// applied; it is a crash-guard, not a real branch. The metadata is forgotten
// UNCONDITIONALLY (outside the guard) on purpose: dropping the marker is always the
// safe outcome, and keeping gMarkerVerse set on a bounds miss could let a later
// render re-apply the marker to the wrong place. Any theoretical orphaned tint is
// transient — the next render repaints verse-number colours from the CSS.
static void btIOSClearMarker(void) {
    if (gMarkerApplied && gReadingTV != nil) {
        NSTextStorage *ts = gReadingTV.textStorage;
        if (gMarkerLen > 0 && gMarkerLoc + gMarkerLen <= ts.length) {
            UIColor *c = [UIColor colorWithRed:gMarkerOrigR green:gMarkerOrigG
                                          blue:gMarkerOrigB alpha:gMarkerOrigA];
            [ts addAttribute:NSForegroundColorAttributeName value:c
                       range:NSMakeRange(gMarkerLoc, gMarkerLen)];
        }
    }
    gMarkerApplied = NO; gMarkerVerse = 0; gMarkerLoc = 0; gMarkerLen = 0;
}

// btIOSLocForVerse returns the character location of `verse`'s number run, or
// NSNotFound. Binary search by verse number over the prebuilt table.
static NSUInteger btIOSLocForVerse(NSTextStorage *ts, NSInteger verse) {
    (void)ts;
    NSUInteger lo = 0, hi = gVerseIndexCount;
    while (lo < hi) {
        NSUInteger mid = lo + (hi - lo) / 2;
        if (gVerseIndex[mid].verse < verse) lo = mid + 1; else hi = mid;
    }
    if (lo < gVerseIndexCount && gVerseIndex[lo].verse == verse) return gVerseIndex[lo].loc;
    return NSNotFound;
}

// btIOSVerseAtIndex returns the verse whose number run is the last at or before
// character index ci (the top-visible verse), writing its location to *outLoc.
// Binary search by location over the prebuilt table.
static NSInteger btIOSVerseAtIndex(NSTextStorage *ts, NSUInteger ci, NSUInteger *outLoc) {
    (void)ts;
    NSUInteger lo = 0, hi = gVerseIndexCount; // first entry with loc > ci
    while (lo < hi) {
        NSUInteger mid = lo + (hi - lo) / 2;
        if (gVerseIndex[mid].loc <= ci) lo = mid + 1; else hi = mid;
    }
    if (lo == 0) { if (outLoc) *outLoc = 0; return 0; } // above the first verse
    BTVerseLoc e = gVerseIndex[lo - 1];
    if (outLoc) *outLoc = e.loc;
    return e.verse;
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
            tv.textContainerInset = UIEdgeInsetsMake(14, 10, 14, 10);
            // Stop iOS from auto-adjusting the content inset for the safe
            // area — we already position the textView below it via the Fyne
            // layout, and the auto-adjust would push verse 1 off the top.
            tv.contentInsetAdjustmentBehavior = UIScrollViewContentInsetAdjustmentNever;
            // Smooth scrolling for long, richly-attributed chapters. UITextView on
            // iOS 16+ defaults to TextKit 2, which lays glyphs out LAZILY while you
            // scroll and visibly hitches on a long chapter of HTML-imported text.
            // Touching layoutManager drops the view to TextKit 1; setting
            // allowsNonContiguousLayout = NO lays the whole chapter out up front so
            // scrolling stays smooth (a small one-time cost when the chapter loads).
            tv.layoutManager.allowsNonContiguousLayout = NO;
            // Start visible — the Read tab is selected at app launch and
            // AppTabs.OnSelected doesn't fire for the initial selection.
            tv.hidden = NO;
            // Single-tap -> "Clear highlight" when the tap lands on the wash. The
            // recognizer never swallows a touch the text view / selection wants, and
            // recognizes simultaneously with the built-in pan/loupe gestures (see the
            // delegate methods). Created once here so it travels with the persistent
            // view across the re-parenting ensureTV does on every call.
            UITapGestureRecognizer *hlTap =
                [[UITapGestureRecognizer alloc] initWithTarget:tv action:@selector(btHighlightTap:)];
            hlTap.numberOfTapsRequired = 1;
            hlTap.cancelsTouchesInView = NO;
            hlTap.delaysTouchesBegan = NO;
            hlTap.delaysTouchesEnded = NO;
            hlTap.delegate = tv;
            [tv addGestureRecognizer:hlTap];
            hlTap.enabled = NO; // enabled only while a verse is highlighted (see applyHTML)
            gHighlightTap = hlTap;
            if (@available(iOS 16.0, *)) {
                // Created but NOT attached here — btIOSSetHighlightUIEnabled adds the
                // interaction only while a verse is highlighted, keeping it (and the
                // recognizers it installs) off the touch path during ordinary reading.
                tv.hlMenu = [[UIEditMenuInteraction alloc] initWithDelegate:tv];
            }
            gReadingTV = tv;
            // Soft-keyboard frame observer (registered once, with the persistent text
            // view): report the keyboard's on-screen overlap so the Goto verse picker
            // lifts its bottom row to sit EXACTLY above the keyboard (no estimate). The
            // end frame is in screen coords; for a full-screen app that equals the Fyne
            // canvas, so the overlap is the inset in points. WillChangeFrame covers
            // show/move; WillHide guarantees a 0 on dismissal.
            NSOperationQueue *mq = [NSOperationQueue mainQueue];
            [[NSNotificationCenter defaultCenter] addObserverForName:UIKeyboardWillChangeFrameNotification
                object:nil queue:mq usingBlock:^(NSNotification *note) {
                    CGRect end = [note.userInfo[UIKeyboardFrameEndUserInfoKey] CGRectValue];
                    CGFloat overlap = CGRectGetMaxY([UIScreen mainScreen].bounds) - end.origin.y;
                    bibleTextKeyboardChanged(overlap > 0 ? (double)overlap : 0.0);
                }];
            [[NSNotificationCenter defaultCenter] addObserverForName:UIKeyboardWillHideNotification
                object:nil queue:mq usingBlock:^(NSNotification *note) {
                    bibleTextKeyboardChanged(0.0);
                }];
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

// bibleTextApplyHTML parses `data` as HTML into an attributed string and applies
// it to the reading text view, returning YES on success. MUST run on the main
// thread. Returns NO (without touching the view) if the import fails so the
// caller can retry — see bibleTextTVSetHTML.
// btIOSSetHighlightUIEnabled attaches the clear-highlight tap recognizer + edit-menu
// interaction ONLY while a verse is highlighted. During ordinary reading both are
// fully off the text view's touch pipeline, so they add nothing to scrolling — a
// UIEditMenuInteraction installs its own gesture recognizers that otherwise process
// every touch (including scroll pans). Called from bibleTextApplyHTML.
static void btIOSSetHighlightUIEnabled(BOOL on) {
    if (gReadingTV == nil) return;
    if (gHighlightTap) gHighlightTap.enabled = on;
    if (@available(iOS 16.0, *)) {
        HBReadingTextView *tv = (HBReadingTextView *)gReadingTV;
        UIEditMenuInteraction *m = tv.hlMenu;
        if (m == nil) return;
        BOOL attached = [tv.interactions containsObject:m];
        if (on && !attached) [tv addInteraction:m];
        else if (!on && attached) [tv removeInteraction:m];
    }
}

static BOOL bibleTextApplyHTML(NSData *data) {
    if (gReadingTV == nil || data == nil) return NO;
    NSDictionary *opts = @{
        NSDocumentTypeDocumentAttribute: NSHTMLTextDocumentType,
        NSCharacterEncodingDocumentAttribute: @(NSUTF8StringEncoding),
    };
    NSError *err = nil;
    NSAttributedString *as = [[NSAttributedString alloc]
                                initWithData:data options:opts documentAttributes:nil error:&err];
    if (as == nil) return NO;
    // NSAttributedString's HTML importer routinely injects a non-zero
    // paragraphSpacingBefore (and sometimes a minimumLineHeight) on the FIRST
    // paragraph that no CSS can override — leaving an ugly ~100pt empty band
    // before verse 1. Zero those so the chapter starts at the frame top.
    NSMutableAttributedString *mas = [as mutableCopy];
    [mas enumerateAttribute:NSParagraphStyleAttributeName
                    inRange:NSMakeRange(0, mas.length) options:0
                 usingBlock:^(id v, NSRange r, BOOL *stop) {
        if (v == nil) return;
        NSMutableParagraphStyle *ps = [(NSParagraphStyle*)v mutableCopy];
        ps.paragraphSpacingBefore = 0;
        [mas addAttribute:NSParagraphStyleAttributeName value:ps range:r];
    }];
    as = mas;
    // Find the highlighted verse (the .hl span becomes a background-coloured run)
    // so we scroll to it rather than the top when arriving from a search result.
    gReadingHighlightRange = (NSRange){NSNotFound, 0};
    [as enumerateAttribute:NSBackgroundColorAttributeName
                   inRange:NSMakeRange(0, as.length) options:0
                usingBlock:^(id value, NSRange range, BOOL *stop) {
        if (value != nil) { gReadingHighlightRange = range; *stop = YES; }
    }];
    // Attach the clear-highlight tap recognizer + edit-menu interaction ONLY while a
    // verse is highlighted; during ordinary reading they're off the touch path
    // entirely, so they add nothing to scrolling.
    btIOSSetHighlightUIEnabled(gReadingHighlightRange.location != NSNotFound);
    gReadingTV.attributedText = as;
    // Re-assert the opaque paper background: assigning attributedText (HTML import)
    // can revert the view toward clearColor/non-opaque, which brings back the
    // per-frame compositor blend and the scroll lag. The NSLog fires ONLY when the
    // assignment actually dropped opaque, so the device console confirms (or rules
    // out) that this is the lag's cause.
    if (gReadingPaperSet && !gReadingTV.opaque) {
        NSLog(@"bibletext: attributedText assignment dropped opaque — re-asserting paper bg");
    }
    btIOSApplyReadingBG();
    btIOSBuildVerseIndex(gReadingTV.textStorage); // cache verse positions for cheap scroll-end anchoring
    // New text: the prior chapter's initial-touch is no longer valid, and the fresh
    // attributed string carries the original verse-number colours (the marker is not
    // applied to it yet). Reset both, then (re-)apply the marker if one is intended —
    // bibleTextApplyHTML can run several times during a restore, so re-applying here
    // keeps the "you left off here" tint pinned through the relayouts.
    gHasLastTouch = NO;
    gMarkerApplied = NO;
    btIOSApplyMarker();
    // Re-assert the scroll position across the relayout + frame push.
    [gReadingTV layoutIfNeeded];
    bibleTextScrollReadingTV();
    // The deferred re-asserts re-place a highlight / restore target after the relayout
    // settles. They are pointless for a plain top-pin (verse 1 is already at the top),
    // and must never fight a flick the reader started in the 200ms window — so only
    // schedule them when a target is armed, and skip if the user is already scrolling.
    if (gReadingHighlightRange.location != NSNotFound || gReadingHasRestore) {
        dispatch_async(dispatch_get_main_queue(), ^{
            if (!gReadingTV.dragging && !gReadingTV.decelerating) bibleTextScrollReadingTV();
        });
        dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(0.2 * NSEC_PER_SEC)),
                       dispatch_get_main_queue(), ^{
            if (!gReadingTV.dragging && !gReadingTV.decelerating) bibleTextScrollReadingTV();
        });
    }
    return YES;
}

// bibleTextPlainFromHTML strips tags + the few entities buildChapterHTML emits —
// a readable last resort so the reader never sees raw markup.
static NSString *bibleTextPlainFromHTML(NSString *html) {
    NSRegularExpression *re = [NSRegularExpression regularExpressionWithPattern:@"<[^>]+>" options:0 error:nil];
    NSString *t = [re stringByReplacingMatchesInString:html options:0
                                                 range:NSMakeRange(0, html.length) withTemplate:@""];
    t = [t stringByReplacingOccurrencesOfString:@"&nbsp;" withString:@" "];
    t = [t stringByReplacingOccurrencesOfString:@"&amp;" withString:@"&"];
    t = [t stringByReplacingOccurrencesOfString:@"&lt;" withString:@"<"];
    t = [t stringByReplacingOccurrencesOfString:@"&gt;" withString:@">"];
    return t;
}

// bibleTextTVSetReadingBG paints the reading view's background with the theme's
// "paper" colour and marks the view OPAQUE. The text view used to be clearColor so
// the Fyne-painted paper rectangle showed through — but a transparent, full-screen,
// SCROLLING view forces the compositor to alpha-blend it over the GL canvas on every
// frame, a constant scroll cost regardless of chapter length. Painting the same paper
// colour into an OPAQUE view lets the compositor skip the per-frame blend (and skip
// drawing the canvas beneath it entirely), which is the classic iOS scroll-perf win.
// btIOSApplyReadingBG paints the cached paper colour into the text view and marks
// it opaque. Called both from bibleTextTVSetReadingBG (when the theme pushes a new
// colour) AND from bibleTextApplyHTML right after attributedText is set, so the
// opaque state always survives the content assignment. Main-thread only.
static void btIOSApplyReadingBG(void) {
    if (gReadingTV == nil || !gReadingPaperSet) return;
    gReadingTV.backgroundColor = [UIColor colorWithRed:gReadingPaperR
                                                 green:gReadingPaperG
                                                  blue:gReadingPaperB alpha:1.0];
    gReadingTV.opaque = YES;
}

void bibleTextTVSetReadingBG(double r, double g, double b) {
    dispatch_async(dispatch_get_main_queue(), ^{
        bibleTextEnsureTV();
        if (gReadingTV == nil) return;
        gReadingPaperR = r;
        gReadingPaperG = g;
        gReadingPaperB = b;
        gReadingPaperSet = YES;
        btIOSApplyReadingBG();
    });
}

void bibleTextTVSetHTML(const char *html) {
    if (html == NULL) return;
    NSString *s = [NSString stringWithUTF8String:html];
    NSData *data = [s dataUsingEncoding:NSUTF8StringEncoding];
    dispatch_async(dispatch_get_main_queue(), ^{
        bibleTextEnsureTV();
        if (gReadingTV == nil) return;
        if (bibleTextApplyHTML(data)) return;
        // The WebKit-backed HTML importer fails intermittently — most often right
        // after the app returns to the foreground. NEVER drop raw markup into the
        // view (the old code did `gReadingTV.text = rawHTML`); retry on later
        // runloop turns and, only if every attempt fails, show tag-stripped text.
        dispatch_async(dispatch_get_main_queue(), ^{
            if (bibleTextApplyHTML(data)) return;
            dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(0.25 * NSEC_PER_SEC)),
                           dispatch_get_main_queue(), ^{
                if (bibleTextApplyHTML(data)) return;
                NSLog(@"bibletext: HTML import failed after retries; showing plain text");
                if (gReadingTV != nil) {
                    gReadingTV.text = bibleTextPlainFromHTML(s);
                    btIOSApplyReadingBG(); // keep the view opaque on the fallback path too
                    btIOSBuildVerseIndex(nil); // plain text has no verse runs — clear the stale table
                }
            });
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
        // Only re-resolve the scroll position when a highlight (search jump) or a
        // pending restore is armed: those were computed at the old width and must be
        // re-placed after the rewrap. Without a target, bibleTextScrollReadingTV
        // snaps to the top (line ~229), so re-asserting on every frame change yanked
        // a mid-chapter reader back to the top on any Resize/Move (rotation, keyboard,
        // a stray layout pass) and ran a full layoutIfNeeded + a doubled scroll each
        // time. Gating it (and dropping the redundant nested re-scroll) mirrors
        // bibleTextMacTVSetFrame and leaves a plain reader exactly where they are.
        if (changed && (gReadingHighlightRange.location != NSNotFound || gReadingHasRestore)) {
            [gReadingTV layoutIfNeeded];
            bibleTextScrollReadingTV();
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

// bibleTextTVCaptureTouch maps the last scroll's initial-touch content-y (recorded
// in scrollViewWillBeginDragging) to the verse the finger grabbed, plus the
// within-verse offset of the touch. ok=0 when no touch was recorded for the live
// chapter or it couldn't be mapped. Synchronous on the main thread.
BTTouch bibleTextTVCaptureTouch(void) {
    __block BTTouch out = {0, 0, 0};
    dispatch_block_t block = ^{
        if (!gHasLastTouch || gReadingTV == nil) return;
        UITextView *tv = gReadingTV;
        NSLayoutManager *lm = tv.layoutManager;
        NSTextStorage *ts = tv.textStorage;
        if (ts.length == 0) return;
        CGFloat tcY = gLastTouchContentY - tv.textContainerInset.top;
        if (tcY < 0) tcY = 0;
        NSUInteger gi = [lm glyphIndexForPoint:CGPointMake(4, tcY) inTextContainer:tv.textContainer];
        NSUInteger ci = [lm characterIndexForGlyphAtIndex:gi];
        NSUInteger loc = 0;
        NSInteger v = btIOSVerseAtIndex(ts, ci, &loc);
        if (v <= 0) return;
        NSRange g = [lm glyphRangeForCharacterRange:NSMakeRange(loc, 1) actualCharacterRange:NULL];
        CGRect rr = [lm boundingRectForGlyphRange:g inTextContainer:tv.textContainer];
        out.ok = 1;
        out.verse = (int)v;
        out.delta = tcY - rr.origin.y; // within-verse offset of the touch (content space)
    };
    if ([NSThread isMainThread]) block();
    else dispatch_sync(dispatch_get_main_queue(), block);
    return out;
}

// bibleTextTVArmMarker sets (verse>0) or clears (verse<=0) the "you left off here"
// marker on the given verse, in the accent colour (r,g,b in 0..1). Applied
// immediately if the chapter is already laid out, and re-applied by
// bibleTextApplyHTML across restore relayouts.
//
// SYNCHRONOUS on the main thread (mirrors bibleTextTVCaptureAnchor): pushChapterHTML
// arms the marker and THEN submits the chapter text (bibleTextTVSetHTML) in the same
// pass. Running the arm inline sets/clears gMarkerVerse BEFORE that text is queued,
// so bibleTextApplyHTML can never observe a stale marker from the previous chapter —
// correctness does not rely on main-queue ordering between the two async submissions.
void bibleTextTVArmMarker(int verse, double r, double g, double b) {
    dispatch_block_t block = ^{
        if (verse <= 0) { btIOSClearMarker(); return; }
        gMarkerVerse = verse; gMarkerR = r; gMarkerG = g; gMarkerB = b;
        btIOSApplyMarker();
    };
    if ([NSThread isMainThread]) block();
    else dispatch_sync(dispatch_get_main_queue(), block);
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

	// Flat reading surface: a plain parchment (pal.Background) fill — the native
	// UITextView paints the same colour — with no bordered "card" or rounded frame,
	// so the verse text sits directly on the warm parchment ground as one
	// continuous plane with the header.
	paper := canvas.NewRectangle(pal.Background)

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
		// Margin AROUND the recents card (not inside it) so it floats clear of the
		// app-header divider above and the chapter heading below, rather than sitting
		// cramped against them.
		top.Add(container.New(layout.NewCustomPaddedLayout(6, 4, 0, 0), bar))
	}
	if state.CanReturnToSearchResults {
		top.Add(backToResultsBar(state))
	}
	top.Add(chapterHeaderMobile(state, chapterNumbers))

	// Inset the whole header column (history bar, back-to-results bar, and the
	// chapter heading) by one theme pad on the LEFT so its text lines up directly
	// under the app header's "BibleText" / version column, which carries the same
	// left pad (ui.go buildHeader). This is applied to the header band only — the
	// body's own left pad stays 0 — so the native verse text below keeps its own
	// reading inset and isn't pushed in with the chrome.
	header := container.New(layout.NewCustomPaddedLayout(0, 0, theme.Padding(), 0), top)

	body := container.NewBorder(header, nil, nil, nil, container.NewStack(paper, host))
	// No top padding (the app header's rule + the border gap already separate it) and
	// no LEFT padding (the header band above is inset on its own; the verse column keeps
	// its native inset). Keep the RIGHT pad (so the fullscreen control lines up under the
	// gear) and a bottom margin above the tab bar.
	return container.New(layout.NewCustomPaddedLayout(0, theme.Padding(), 0, theme.Padding()), body)
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
	// One even box height for BOTH rows, so the title row and the chapter/nav row
	// share the same vertical rhythm and the toolbar stays compact. A slightly
	// smaller heading (vs the 26px page heading) keeps it closer in scale to the
	// chapter line below, so that line no longer floats in an over-tall box.
	const boxH = 30
	const headSize = 22
	ref := newReferenceButton(fmt.Sprintf("%s %d", state.CurrentBook, state.CurrentChapter), pal.Text, headSize, boxH, func() {
		showChapterPicker(state)
	})

	// Small copy icon tucked after the heading — lighter than the chapter-nav
	// arrows but still a full-height (finger-friendly) hit box.
	copyBtn := newIconTapButton(state, theme.ContentCopyIcon(), 16, boxH, func() {
		copyChapter(state)
	})
	titleRow := container.NewHBox(ref, hgap(6), copyBtn)

	// Quiet chapter context below the heading — also a picker target, so the
	// whole "Chapter N of M" line opens the picker too.
	chapText := fmt.Sprintf("Chapter %d of %d", state.CurrentChapter, total)
	if total <= 1 {
		chapText = fmt.Sprintf("Chapter %d", state.CurrentChapter)
	}
	chapterLine := newTapTextStyled(chapText, pal.TextMuted, subheadingTextSize, boxH, false, func() {
		showChapterPicker(state)
	})

	idx := indexOf(chapterNumbers, state.CurrentChapter)

	// Prev/next as compact icon buttons sitting next to the chapter line, so
	// they're close to the book + chapter text rather than floating far right.
	prev := newIconTapButton(state, theme.NavigateBackIcon(), 20, boxH, func() {
		if moveChapter(state, -1) {
			state.refresh()
		}
	})
	prev.disabled = idx <= 0

	next := newIconTapButton(state, theme.NavigateNextIcon(), 20, boxH, func() {
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

	// Tighter-than-default gap between the two rows so the book heading and the
	// chapter/nav line read as one compact block, not two airy lines.
	left := container.New(layout.NewCustomPaddedVBoxLayout(2), titleRow, chapterRow)
	right := container.NewVBox(layout.NewSpacer(), fullScreenBtn, layout.NewSpacer())
	row := container.NewBorder(nil, nil, left, right, nil)

	// No divider under the header — the flat reading surface separates the chapter
	// toolbar from the verses with whitespace (the text view's top inset) instead
	// of a hard rule.
	return row
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
	// Only the latest host may write the singleton overlay frame. A stale host
	// (e.g. the previous chapter's, swapped out by showReading) can still receive
	// a Resize/Move during the relayout and would otherwise clobber the current
	// frame — the immediate write needs the same guard the deferred one has.
	if currentHost != h {
		return
	}
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

// lastPushedBookChapter is the "book|chapter" of the chapter currently held by the
// native text view — distinct from lastPushedChapterFP (which also folds in theme,
// red-letter and highlight). It lets pushChapterHTML tell a genuine chapter change
// (pin to top) from a same-chapter re-render (preserve the reader's scroll).
var lastPushedBookChapter string

// pushChapterHTML builds the chapter as HTML (so NSAttributedString gets nice
// inline styling — superscript verse numbers, accent color, serif font) and
// sends it across the CGO boundary.
func pushChapterHTML(state *AppState, verses []Verse) {
	fp := chapterRenderFingerprint(state)
	bc := fmt.Sprintf("%s|%d", state.CurrentBook, state.CurrentChapter)

	// Same-chapter RE-render — the fingerprint changed but the book+chapter did not.
	// This fires on a light/dark flip, a red-letter toggle, and (the common one) when
	// iOS forces an appearance change to snapshot the BACKGROUNDED app for the app
	// switcher: the reader takes a screenshot, shares it, comes back, and the trait
	// flip rebuilds the view. The bibleTextTVSetHTML below replaces the text view's
	// content, which snaps the scroll to the TOP. If the reader is mid-chapter with
	// nothing else pending, capture their live position now and arm it as a one-shot
	// restore so the post-setHTML scroll cadence returns them to where they were
	// instead of the top. (This is also what keeps a manual dark-mode toggle from
	// jumping the reader to the top of the chapter.)
	if state.restore == nil && bc == lastPushedBookChapter && fp != lastPushedChapterFP {
		if v, d, f, ok := captureReadingAnchor(); ok && (v > 0 || f > 0) {
			state.restore = &restoreAnchor{
				Book:    state.CurrentBook,
				Chapter: state.CurrentChapter,
				Verse:   v,
				Delta:   d,
				Frac:    f,
			}
		}
	}

	// Arm any pending one-shot scroll restore for this chapter (reopening where the
	// reader left off, or the position just captured above) before pushing the text,
	// so bibleTextScrollReadingTV lands on it rather than the top. A normal push
	// disarms it. (Done before the skip check below so a pending restore always re-arms.)
	armPendingRestore(state)

	// Skip the costly HTML rebuild + NSAttributedString re-import when the
	// UITextView already holds this exact chapter render (e.g. switching to the
	// Books tab and back, or a refresh that didn't change the text). A pending
	// scroll restore forces the push so the scroll cadence runs. The fingerprint
	// includes highlight + theme so a search-jump or light/dark flip still pushes.
	if state.restore == nil && fp == lastPushedChapterFP {
		return
	}
	lastPushedChapterFP = fp
	lastPushedBookChapter = bc

	html := buildChapterHTML(state, verses)
	c := C.CString(html)
	defer C.free(unsafe.Pointer(c))
	C.bibleTextTVSetHTML(c)

	// Keep the native text view OPAQUE with the current theme's reading-ground colour
	// (the same pal.Background parchment the Fyne layer paints behind it) so scrolling
	// doesn't pay a per-frame alpha blend. Set on every real render, so a light/dark
	// switch (which changes the fingerprint above and thus reaches here) updates it too.
	bg := state.pal().Background
	C.bibleTextTVSetReadingBG(
		C.double(float64(bg.R)/255), C.double(float64(bg.G)/255), C.double(float64(bg.B)/255))
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

// captureLastTouch reports the verse the reader's finger first grabbed on the last
// scroll (+ the within-verse offset of the touch). ok is false when no touch was
// recorded for the live chapter. armReadingMarker sets/clears the reopen marker.
func captureLastTouch() (verse int, delta float64, ok bool) {
	t := C.bibleTextTVCaptureTouch()
	return int(t.verse), float64(t.delta), t.ok != 0
}

func armReadingMarker(verse int, r, g, b float64) {
	C.bibleTextTVArmMarker(C.int(verse), C.double(r), C.double(g), C.double(b))
}

// buildChapterHTML, nrgbaToHex and htmlEscape moved to reading.go so the macOS
// NSTextView overlay shares the exact same chapter HTML as the iOS UITextView.
