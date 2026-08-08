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
// Posted when the reader scrolls by hand while read-along is live (audio_export_apple.go).
extern void bibleTextReadAlongUserScrolled(void);
// Posted when the floating "Follow narration" button is clicked (audio_export_apple.go).
extern void bibleTextReadAlongFollowTapped(void);

// Selection-menu AI gate, mirroring the Settings → Assistant choice ("None" turns
// AI off). Set from Go (btMacSetAIEnabled) when the reading host is built and
// whenever the setting changes; menuForEvent: reads it to include or omit the
// "Study with AI" submenu. Defaults to on, matching the preference's default.
static int gBTAIEnabled = 1;
void btMacSetAIEnabled(int on) { gBTAIEnabled = on; }

// HBReadingTextView adds a "Study with AI" submenu (Explain /
// Analyze context / Analyze translation) to the right-click selection menu and
// hands the selected text to Go.
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

    // Snapshot the AI gate once so the menu is structurally consistent even if
    // the flag flips mid-build (it's read twice below: the AI-vs-xref slot and
    // the trailing xref add).
    BOOL aiOn = gBTAIEnabled != 0;

    // Curate the default selection menu down to the essentials. macOS auto-adds a
    // long list a Bible reader doesn't need — Translate, Search-the-web, Speech,
    // Font/Spelling/Substitutions, and its own Share (which duplicates ours below).
    // Keep only Copy and Look Up; drop the rest (and the now-stray separators), then
    // append our group. (The "Services" submenu is injected later, at display time,
    // so it can't be removed here — validRequestorForSendType: below suppresses it.)
    NSMutableArray<NSMenuItem *> *drop = [NSMutableArray array];
    for (NSMenuItem *it in menu.itemArray) {
        if (it.action == @selector(copy:) || [it.title hasPrefix:@"Look Up"]) continue;
        [drop addObject:it];
    }
    for (NSMenuItem *it in drop) [menu removeItem:it];

    // Our group below — set off by a separator, but only if Copy / Look Up survived
    // the curation above. With AI on (an assistant chosen in Settings): Study with
    // AI, Share, Cross-references (same order as iOS). With AI off ("None"):
    // Cross-references takes the study slot at the top, then Share.
    if (menu.numberOfItems > 0) [menu addItem:[NSMenuItem separatorItem]];

    NSMenuItem *xref = [[NSMenuItem alloc] initWithTitle:@"Cross-references" action:@selector(hbCrossRefs:) keyEquivalent:@""];
    xref.target = self;

    if (aiOn) {
        NSMenu *ai = [[NSMenu alloc] initWithTitle:@"Study with AI"];
        for (NSArray *pair in @[@[@"Explain", @"explain"],
                                @[@"Analyze context", @"context"],
                                @[@"Analyze translation", @"translation"]]) {
            SEL action = NSSelectorFromString([NSString stringWithFormat:@"hbAI_%@:", pair[1]]);
            NSMenuItem *it = [[NSMenuItem alloc] initWithTitle:pair[0] action:action keyEquivalent:@""];
            it.target = self;
            [ai addItem:it];
        }
        NSMenuItem *aiItem = [[NSMenuItem alloc] initWithTitle:@"Study with AI" action:nil keyEquivalent:@""];
        aiItem.submenu = ai;
        [menu addItem:aiItem];
    } else {
        [menu addItem:xref];
    }

    // Share → with citation / as image / as link (all go to the macOS share sheet).
    NSMenu *share = [[NSMenu alloc] initWithTitle:@"Share"];
    NSMenuItem *sc = [[NSMenuItem alloc] initWithTitle:@"Share with citation" action:@selector(hbShare_cite:) keyEquivalent:@""];
    sc.target = self;
    [share addItem:sc];
    NSMenuItem *si = [[NSMenuItem alloc] initWithTitle:@"Share as image" action:@selector(hbShare_image:) keyEquivalent:@""];
    si.target = self;
    [share addItem:si];
    NSMenuItem *sl = [[NSMenuItem alloc] initWithTitle:@"Share as link" action:@selector(hbShare_link:) keyEquivalent:@""];
    sl.target = self;
    [share addItem:sl];
    NSMenuItem *shareItem = [[NSMenuItem alloc] initWithTitle:@"Share" action:nil keyEquivalent:@""];
    shareItem.submenu = share;
    [menu addItem:shareItem];

    // Cross-references sits last (below AI and Share) when AI is on — its usual
    // spot; with AI off it was already added in the study slot above.
    if (aiOn) [menu addItem:xref];
    return menu;
}

// AppKit appends a "Services" submenu to a text view's contextual menu at display
// time — after menuForEvent: returns — so the curation above can never remove it.
// That submenu is only inserted when the responder chain offers a valid Services
// requestor, and NSTextView claims to be one (so Services can act on the selection).
// This read-only reading pane has no use for Services, so we opt out by claiming we
// neither send nor receive anything; with no requestor in the chain, AppKit never
// inserts the submenu. Copy / Look Up / our own actions don't go through Services.
- (id)validRequestorForSendType:(NSPasteboardType)sendType returnType:(NSPasteboardType)returnType {
    return nil;
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
- (void)hbShare_link:(id)sender {
    bibleTextStudyMenuTapped((char *)"share-link", (char *)self.hbSelectedText.UTF8String);
}
// Target of the floating "Follow narration" button (btMacEnsureFollowBtn) — the
// text view doubles as its action target so no extra controller object is needed.
- (void)hbFollowTapped:(id)sender {
    bibleTextReadAlongFollowTapped();
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
// <sup class="v"> at font-size 0.66em of the body size (which the reader can scale).
// Detecting them by font size (rather than a superscript attribute) matches the
// iOS overlay and reads the run's digits directly. The threshold is DERIVED from
// the rendered text (80% of the largest font in the storage) rather than a
// constant, because the reader can now scale the scripture text: superscripts
// are 0.66× the body at every size, so 0.8× cleanly separates them.
static CGFloat btMacVerseFontThreshold(NSTextStorage *ts) {
    __block CGFloat maxSize = 0;
    [ts enumerateAttribute:NSFontAttributeName
                   inRange:NSMakeRange(0, ts.length)
                   options:0
                usingBlock:^(id val, NSRange r, BOOL *stop) {
        if (val != nil && ((NSFont *)val).pointSize > maxSize) maxSize = ((NSFont *)val).pointSize;
    }];
    return maxSize > 0 ? maxSize * 0.8 : 15.0;
}

// btMacLocForVerse returns the character location of `verse`'s number run, or
// NSNotFound.
static NSUInteger btMacLocForVerse(NSTextStorage *ts, NSInteger verse) {
    __block NSUInteger found = NSNotFound;
    CGFloat thr = btMacVerseFontThreshold(ts);
    [ts enumerateAttribute:NSFontAttributeName
                   inRange:NSMakeRange(0, ts.length)
                   options:0
                usingBlock:^(id val, NSRange r, BOOL *stop) {
        if (val == nil || r.length == 0 || ((NSFont *)val).pointSize >= thr) return;
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
    CGFloat thr = btMacVerseFontThreshold(ts);
    [ts enumerateAttribute:NSFontAttributeName
                   inRange:NSMakeRange(0, ts.length)
                   options:0
                usingBlock:^(id val, NSRange r, BOOL *stop) {
        if (r.location > ci) { *stop = YES; return; }
        if (val == nil || r.length == 0 || ((NSFont *)val).pointSize >= thr) return;
        NSInteger n = [[ts.string substringWithRange:r] integerValue];
        if (n > 0) { verse = n; loc = r.location; }
    }];
    if (outLoc) *outLoc = loc;
    return verse;
}

// ---- Read-along: highlight the verse being narrated + gently follow-scroll -------
// gReadAlongRange is the char range currently tinted (its number run through just
// before the next verse's), so each tick can clear the previous verse cheaply.
static NSRange  gReadAlongRange = {NSNotFound, 0};
static NSColor *gReadAlongColor = nil;
// gReadAlongActive marks a live read-along; gReadAlongUserLatch makes the
// user-scrolled callback one-shot (reset when following resumes or read-along ends);
// gReadAlongOwnScroll is raised around OUR OWN follow-scroll so the bounds-change
// observer below can tell the reader's scrolling from ours.
static BOOL gReadAlongActive = NO;
static BOOL gReadAlongUserLatch = NO;
static BOOL gReadAlongOwnScroll = NO;

// --- Floating "Follow narration" button -------------------------------------
// A semi-transparent pill floated bottom-centre over the reading pane while the
// reader has scrolled away mid-narration (follow suspended). Native (AppKit)
// because the NSTextView overlay paints ABOVE the whole Fyne canvas — a Fyne
// widget could never float over the verses. Shown/hidden by the Go controller
// via bibleTextMacFollowButton; its colours arrive from the app palette via
// bibleTextMacSetFollowButtonColors (re-pushed on every reading-view build, so
// a light/dark flip restyles it).
static NSButton *gMacFollowBtn = nil;
static BOOL      gMacFollowWanted = NO;
static CGFloat   gMacFollowBg[3] = {0.18, 0.30, 0.53}; // lapis fallback
static CGFloat   gMacFollowFg[3] = {0.96, 0.97, 0.99};

static void btMacStyleFollowBtn(void) {
    if (gMacFollowBtn == nil) return;
    // attributedTitle REPLACES the cell's default centred alignment — without an
    // explicit centred paragraph style the label renders left-aligned in the pill.
    NSMutableParagraphStyle *ps = [[NSMutableParagraphStyle alloc] init];
    ps.alignment = NSTextAlignmentCenter;
    NSDictionary *attrs = @{
        NSFontAttributeName: [NSFont systemFontOfSize:13 weight:NSFontWeightSemibold],
        NSForegroundColorAttributeName: [NSColor colorWithCalibratedRed:gMacFollowFg[0]
                                                                  green:gMacFollowFg[1]
                                                                   blue:gMacFollowFg[2] alpha:1.0],
        NSParagraphStyleAttributeName: ps,
    };
    gMacFollowBtn.attributedTitle =
        [[NSAttributedString alloc] initWithString:@"Follow narration" attributes:attrs];
    gMacFollowBtn.layer.backgroundColor =
        [NSColor colorWithCalibratedRed:gMacFollowBg[0] green:gMacFollowBg[1]
                                   blue:gMacFollowBg[2] alpha:0.78].CGColor; // semi-transparent
}

// btMacLayoutFollowBtn centres the pill over the reading pane's bottom edge.
// The superview is non-flipped (bottom-left origin), so "18pt above the pane's
// bottom" is frame.origin.y + 18.
static void btMacLayoutFollowBtn(void) {
    if (gMacFollowBtn == nil || gScroll == nil) return;
    NSSize sz = gMacFollowBtn.frame.size;
    NSRect sf = gScroll.frame;
    gMacFollowBtn.frame = NSMakeRect(NSMidX(sf) - sz.width / 2,
                                     sf.origin.y + 18, sz.width, sz.height);
}

static void btMacEnsureFollowBtn(void) {
    if (gScroll == nil || gScroll.superview == nil) return;
    if (gMacFollowBtn == nil) {
        NSButton *b = [[NSButton alloc] initWithFrame:NSZeroRect];
        b.bordered = NO;
        b.wantsLayer = YES;
        [b setButtonType:NSButtonTypeMomentaryChange];
        b.target = gTextView;
        b.action = @selector(hbFollowTapped:);
        b.layer.cornerRadius = 15;
        b.hidden = YES;
        gMacFollowBtn = b;
        btMacStyleFollowBtn();
        [b sizeToFit];
        // A roomier pill than sizeToFit's tight text box: ~14pt side padding, 30pt tall.
        b.frame = NSMakeRect(0, 0, b.frame.size.width + 28, 30);
    }
    if (gMacFollowBtn.superview != gScroll.superview) {
        [gMacFollowBtn removeFromSuperview];
        [gScroll.superview addSubview:gMacFollowBtn positioned:NSWindowAbove relativeTo:nil];
    }
}

// btMacFrontFollowBtn keeps the pill above the scroll view — EnsureTV/TVShow
// re-add gScroll at the top of the sibling order on every call.
static void btMacFrontFollowBtn(void) {
    if (gMacFollowBtn != nil && gMacFollowBtn.superview != nil) {
        [gMacFollowBtn.superview addSubview:gMacFollowBtn positioned:NSWindowAbove relativeTo:nil];
    }
}

// btMacApplyFollowVisibility resolves the pill's actual visibility: wanted by
// the controller AND the reading overlay itself is up (a modal or tab switch
// that hides the verses must take the pill down with them).
static void btMacApplyFollowVisibility(void) {
    if (gMacFollowBtn == nil) return;
    BOOL show = gMacFollowWanted && gScroll != nil && !gScroll.hidden && !gReadingSuppressed;
    gMacFollowBtn.hidden = !show;
    if (show) { btMacLayoutFollowBtn(); btMacFrontFollowBtn(); }
}

void bibleTextMacFollowButton(int show) {
    dispatch_async(dispatch_get_main_queue(), ^{
        gMacFollowWanted = (show != 0);
        if (gMacFollowWanted) btMacEnsureFollowBtn();
        btMacApplyFollowVisibility();
    });
}

void bibleTextMacSetFollowButtonColors(double bgR, double bgG, double bgB,
                                       double fgR, double fgG, double fgB) {
    dispatch_async(dispatch_get_main_queue(), ^{
        gMacFollowBg[0] = bgR; gMacFollowBg[1] = bgG; gMacFollowBg[2] = bgB;
        gMacFollowFg[0] = fgR; gMacFollowFg[1] = fgG; gMacFollowFg[2] = fgB;
        btMacStyleFollowBtn();
    });
}

// btMacReadAlongRange returns verse's number-run start through just before the next
// verse's number run (or end of text) — i.e. the whole verse, number + words.
static NSRange btMacReadAlongRange(NSTextStorage *ts, NSInteger verse) {
    NSUInteger start = btMacLocForVerse(ts, verse);
    if (start == NSNotFound) return NSMakeRange(NSNotFound, 0);
    __block NSUInteger nextLoc = ts.length;
    CGFloat thr = btMacVerseFontThreshold(ts);
    [ts enumerateAttribute:NSFontAttributeName inRange:NSMakeRange(start, ts.length - start)
                   options:0 usingBlock:^(id val, NSRange r, BOOL *stop) {
        if (r.location <= start || val == nil || r.length == 0 ||
            ((NSFont *)val).pointSize >= thr) return;
        if ([[ts.string substringWithRange:r] integerValue] > 0) { nextLoc = r.location; *stop = YES; }
    }];
    return NSMakeRange(start, nextLoc - start);
}

void bibleTextMacReadAlongClear(void) {
    // Reachable from the Fyne goroutine (main on macOS) but also from AVSpeechSynthesizer
    // delegate callbacks, whose thread is not documented — marshal to main like the iOS twin.
    if (![NSThread isMainThread]) {
        dispatch_async(dispatch_get_main_queue(), ^{ bibleTextMacReadAlongClear(); });
        return;
    }
    if (gTextView == nil) return;
    NSTextStorage *ts = gTextView.textStorage;
    if (gReadAlongRange.location != NSNotFound && NSMaxRange(gReadAlongRange) <= ts.length) {
        [ts beginEditing];
        [ts removeAttribute:NSBackgroundColorAttributeName range:gReadAlongRange];
        [ts endEditing];
        // Unlike the per-tick highlight (always mid-playback, window active), this
        // clear can fire after long idle — e.g. closing the audio card much later —
        // where a coalesced/napped display update can drop the attribute change's
        // invalidation. Force the repaint so the tint never visibly lingers.
        [gTextView setNeedsDisplayInRect:gTextView.visibleRect];
    }
    gReadAlongRange = NSMakeRange(NSNotFound, 0);
    gReadAlongActive = NO;
    gReadAlongUserLatch = NO;
}

// bibleTextMacHighlightVerse tints the narrated verse (clearing the previous one) and
// follow-scrolls only when the verse has drifted out of a comfortable band, so the
// text isn't yanked on every verse. verse<=0 just clears (recording's intro).
void bibleTextMacHighlightVerse(int verse, int follow) {
    if (![NSThread isMainThread]) {   // see bibleTextMacReadAlongClear
        dispatch_async(dispatch_get_main_queue(), ^{ bibleTextMacHighlightVerse(verse, follow); });
        return;
    }
    if (gTextView == nil) return;
    NSTextStorage *ts = gTextView.textStorage;
    [ts beginEditing];
    if (gReadAlongRange.location != NSNotFound && NSMaxRange(gReadAlongRange) <= ts.length)
        [ts removeAttribute:NSBackgroundColorAttributeName range:gReadAlongRange];
    gReadAlongRange = NSMakeRange(NSNotFound, 0);
    if (verse > 0) {
        NSRange r = btMacReadAlongRange(ts, verse);
        if (r.location != NSNotFound && NSMaxRange(r) <= ts.length) {
            if (gReadAlongColor == nil)
                gReadAlongColor = [NSColor colorWithCalibratedRed:1.0 green:0.80 blue:0.30 alpha:0.32];
            [ts addAttribute:NSBackgroundColorAttributeName value:gReadAlongColor range:r];
            gReadAlongRange = r;
        }
    }
    [ts endEditing];
    gReadAlongActive = (verse > 0);
    if (follow) gReadAlongUserLatch = NO;   // following again → re-arm the one-shot

    if (!follow) return;   // reader scrolled away; tint only, never yank the view

    // Follow-scroll: keep the narrated verse in a comfortable band. All geometry stays in
    // the text view's OWN bounds space — visibleRect for "where are we" and -scrollPoint:
    // for the move — so it's immune to the text view's frame origin, which the layout can
    // leave non-zero inside the clip view. (A raw clip-view scrollToPoint:0 assumes the
    // document sits at origin 0; when it doesn't, the content lands offset below the top,
    // leaving a stale gap above verse 1.) Only scrolls when the verse drifts above the top
    // or past 70% down, so the text isn't yanked on every verse.
    if (gReadAlongRange.location != NSNotFound) {
        NSLayoutManager *lm = gTextView.layoutManager;
        NSRange g = [lm glyphRangeForCharacterRange:gReadAlongRange actualCharacterRange:NULL];
        NSRect rect = [lm boundingRectForGlyphRange:g inTextContainer:gTextView.textContainer];
        CGFloat vTop = rect.origin.y + gTextView.textContainerInset.height;
        NSRect vis = gTextView.visibleRect;
        if (vTop < vis.origin.y || vTop > vis.origin.y + vis.size.height * 0.70) {
            CGFloat y = vTop - vis.size.height * 0.30;
            if (y < 0) y = 0;
            gReadAlongOwnScroll = YES;   // our scroll — not the reader taking over
            [gTextView scrollPoint:NSMakePoint(0, y)];
            gReadAlongOwnScroll = NO;
        }
    }
}

// bibleTextMacScrollTV positions the chapter, in priority order: the highlighted
// verse (a search jump), then a one-shot restore target (reopening where the
// reader left off), otherwise the very top. NSTextView is flipped, so larger y is
// further down; we scroll the clip view to the target glyph rect.
static void bibleTextMacScrollTV(void) {
    if (gTextView == nil || gScroll == nil) return;
    // Programmatic scrolling (e.g. read-along follow-scroll) can leave the
    // verticallyResizable text view's frame origin non-zero inside the clip view.
    // Every case below computes its target in frame space and scrolls the clip view,
    // which assumes the document sits at origin 0 — a stale origin would land the
    // content offset below the top (a gap above verse 1). Normalize it first.
    { NSRect tf = gTextView.frame; if (tf.origin.y != 0) { tf.origin.y = 0; [gTextView setFrame:tf]; } }
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

            // Any scroll moves the clip view's bounds. Our own follow-scroll raises
            // gReadAlongOwnScroll around its scrollPoint call, so a bounds change
            // WITHOUT that flag while read-along is live = the reader scrolling
            // (trackpad, mouse wheel, scroller drag — the live-scroll notification
            // alone misses discrete wheel events).
            sv.contentView.postsBoundsChangedNotifications = YES;
            [[NSNotificationCenter defaultCenter]
                addObserverForName:NSViewBoundsDidChangeNotification
                            object:sv.contentView queue:nil usingBlock:^(NSNotification *n) {
                if (gReadAlongActive && !gReadAlongOwnScroll && !gReadAlongUserLatch) {
                    gReadAlongUserLatch = YES;
                    bibleTextReadAlongUserScrolled();
                }
            }];

            gScroll = sv;
            gTextView = tv;
        }
        if (gScroll.superview != win.contentView) {
            [gScroll removeFromSuperview];
            [win.contentView addSubview:gScroll];
        }
        [gScroll.superview addSubview:gScroll positioned:NSWindowAbove relativeTo:nil];
        btMacFrontFollowBtn(); // keep the follow pill above the re-fronted overlay
    };
    if ([NSThread isMainThread]) block();
    else dispatch_sync(dispatch_get_main_queue(), block);
}

// bibleTextMacApplyHTML parses `data` as HTML and applies it to the text view,
// returning YES on success (NO without touching the view on failure, so the
// caller can retry). Main-thread only.
static BOOL bibleTextMacApplyHTML(NSData *data) {
    if (gTextView == nil || data == nil) return NO;
    NSDictionary *opts = @{
        NSDocumentTypeDocumentAttribute: NSHTMLTextDocumentType,
        NSCharacterEncodingDocumentAttribute: @(NSUTF8StringEncoding),
    };
    NSError *err = nil;
    NSMutableAttributedString *as =
        [[NSMutableAttributedString alloc] initWithData:data options:opts
                                     documentAttributes:nil error:&err];
    if (as == nil) return NO;
    // The HTML importer injects a phantom paragraphSpacingBefore on the first
    // paragraph; zero it so the chapter starts flush at the top.
    [as enumerateAttribute:NSParagraphStyleAttributeName
                   inRange:NSMakeRange(0, as.length) options:0
                usingBlock:^(id v, NSRange r, BOOL *stop) {
        if (v == nil) return;
        NSMutableParagraphStyle *ps = [(NSParagraphStyle *)v mutableCopy];
        ps.paragraphSpacingBefore = 0;
        [as addAttribute:NSParagraphStyleAttributeName value:ps range:r];
    }];
    gMacHighlightRange = (NSRange){NSNotFound, 0};
    [as enumerateAttribute:NSBackgroundColorAttributeName
                   inRange:NSMakeRange(0, as.length) options:0
                usingBlock:^(id value, NSRange r, BOOL *stop) {
        if (value != nil) { gMacHighlightRange = r; *stop = YES; }
    }];
    [gTextView.textStorage setAttributedString:as];
    bibleTextMacScrollTV();
    return YES;
}

static NSString *bibleTextMacPlainFromHTML(NSString *html) {
    // Poem-line <br> must become a real newline BEFORE the tag strip, or
    // poetry lines jam together and selections there can no longer locate.
    html = [html stringByReplacingOccurrencesOfString:@"<br>" withString:@"\n"];
    NSRegularExpression *re = [NSRegularExpression regularExpressionWithPattern:@"<[^>]+>" options:0 error:nil];
    NSString *t = [re stringByReplacingMatchesInString:html options:0
                                                 range:NSMakeRange(0, html.length) withTemplate:@""];
    t = [t stringByReplacingOccurrencesOfString:@"&nbsp;" withString:@" "];
    t = [t stringByReplacingOccurrencesOfString:@"&amp;" withString:@"&"];
    t = [t stringByReplacingOccurrencesOfString:@"&lt;" withString:@"<"];
    t = [t stringByReplacingOccurrencesOfString:@"&gt;" withString:@">"];
    return t;
}

void bibleTextMacTVSetHTML(const char *html) {
    if (html == NULL) return;
    NSString *s = [NSString stringWithUTF8String:html];
    NSData *data = [s dataUsingEncoding:NSUTF8StringEncoding];
    dispatch_async(dispatch_get_main_queue(), ^{
        bibleTextMacEnsureTV();
        if (gTextView == nil) return;
        if (bibleTextMacApplyHTML(data)) return;
        // Never drop raw markup into the view; retry, then fall back to plain text.
        dispatch_async(dispatch_get_main_queue(), ^{
            if (bibleTextMacApplyHTML(data)) return;
            dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(0.25 * NSEC_PER_SEC)),
                           dispatch_get_main_queue(), ^{
                if (bibleTextMacApplyHTML(data)) return;
                NSLog(@"bibletext(mac): HTML import failed after retries; showing plain text");
                if (gTextView != nil) [gTextView setString:bibleTextMacPlainFromHTML(s)];
            });
        });
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
        btMacLayoutFollowBtn(); // the pill floats relative to the pane's bottom edge
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
        btMacApplyFollowVisibility(); // pill returns with the verses (if still wanted)
    });
}

void bibleTextMacTVHide(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (gScroll == nil) return;
        gScroll.hidden = YES;
        btMacApplyFollowVisibility(); // pill never floats over search results / other views
    });
}

// bibleTextMacTVSuppress hides the overlay and latches it down so that any
// stray bibleTextMacTVShow from a layout pass behind a modal is ignored.
void bibleTextMacTVSuppress(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        gReadingSuppressed = YES;
        if (gScroll == nil) return;
        gScroll.hidden = YES;
        btMacApplyFollowVisibility(); // pill goes down with the overlay behind modals
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
    // AppKit is main-thread-only, so this read must run on main. We deliberately do NOT
    // dispatch_sync to the main queue when off-main: Fyne runs the OnStopped reading-state
    // flush off the main thread during shutdown (runOnMainWithWait executes the callback
    // inline once the main func-queue is drained), and by then the main run loop is gone —
    // a dispatch_sync(main) would block forever and hang the process on quit. So read only
    // when already on main; otherwise return a zero anchor (ok=0) and let captureSnapshot
    // keep the previously-saved one. The normal window-close flush (SetCloseIntercept) runs
    // on the main thread, so it still records the exact scroll position.
    if ([NSThread isMainThread]) block();
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
	"fmt"
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

	// Flat parchment reading surface (matches iOS): the NSTextView floats over this
	// pal.Background rectangle (it has drawsBackground=NO) with no bordered card.
	paper := canvas.NewRectangle(pal.Background)
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

// syncNativeAIMenu mirrors the Settings → Assistant choice ("None" = off) into
// the native selection menu's AI gate (gBTAIEnabled in the C preamble), so the
// "Study with AI" submenu appears or disappears with the setting.
func syncNativeAIMenu(state *AppState) {
	on := C.int(0)
	if aiFeaturesEnabled(state) {
		on = 1
	}
	C.btMacSetAIEnabled(on)
}

// lastPushedBookChapter is the "book|chapter" held by the NSTextView — distinct
// from lastPushedChapterFP (which also folds in theme/red-letter/highlight/data
// identity), so a genuine chapter change (pin to top) is distinguishable from a
// same-chapter re-render (preserve the reader's scroll). Mirrors the iOS and
// Android twins.
var lastPushedBookChapter string

func newMacReadingHost(state *AppState, verses []Verse) *macReadingHost {
	h := &macReadingHost{state: state}
	h.ExtendBaseWidget(h)
	macCurrentHost = h
	syncNativeAIMenu(state) // the menu gate must match the setting before any selection
	fp := chapterRenderFingerprint(state)
	bc := fmt.Sprintf("%s|%d", state.CurrentBook, state.CurrentChapter)
	// Same-chapter RE-render — the fingerprint changed but the book+chapter did
	// not (a light/dark flip, red-letter toggle, or background data swap). The
	// SetHTML below replaces the text view's content, which snaps the scroll to
	// the TOP; capture the reader's live position first and arm it as a one-shot
	// restore, exactly as the iOS twin does (pushChapterHTML). Without this a
	// system dark→light switch yanked a mid-chapter desktop reader to the top —
	// and the next flush then persisted that top-of-chapter position.
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
	// Arm any pending one-shot scroll restore for this chapter (reopening where
	// the reader left off, or the position just captured above) before pushing
	// the text, so bibleTextMacScrollTV lands on the saved position rather than
	// the top. A normal push disarms it.
	armPendingRestore(state)
	// Skip the HTML rebuild + NSAttributedString re-import when the NSTextView
	// already holds this exact chapter render (mirrors the iOS gate in
	// pushChapterHTML); a pending scroll restore forces the push. SetHTML consumes
	// the C string synchronously, so freeing right after the call is safe.
	if state.restore != nil || fp != lastPushedChapterFP {
		lastPushedChapterFP = fp
		lastPushedBookChapter = bc
		html := buildChapterHTML(state, verses)
		c := C.CString(html)
		C.bibleTextMacTVSetHTML(c)
		C.free(unsafe.Pointer(c))
	}
	// Keep the floating "Follow narration" pill styled for the current palette
	// (this build runs on every theme flip).
	pushFollowButtonColors(state.pal())
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

// readAlongHighlight tints the verse being narrated (0 clears) and follow-scrolls it
// into view; readAlongClear removes the tint. Both run on the macOS main thread — the
// audio time-observer's main queue, or the Fyne UI goroutine (which is that thread).
func readAlongHighlight(verse int, follow bool) {
	f := C.int(0)
	if follow {
		f = 1
	}
	C.bibleTextMacHighlightVerse(C.int(verse), f)
}
func readAlongClear() { C.bibleTextMacReadAlongClear() }

// readAlongFollowButton shows/hides the native floating "Follow narration" pill
// over the reading pane (audio_controller drives it around follow suspension).
func readAlongFollowButton(show bool) {
	s := C.int(0)
	if show {
		s = 1
	}
	C.bibleTextMacFollowButton(s)
}

// pushFollowButtonColors styles the pill from the app palette (accent ground,
// accent-text label). Called on every reading-view build so theme flips restyle it.
func pushFollowButtonColors(p palette) {
	C.bibleTextMacSetFollowButtonColors(
		C.double(float64(p.Accent.R)/255), C.double(float64(p.Accent.G)/255), C.double(float64(p.Accent.B)/255),
		C.double(float64(p.AccentText.R)/255), C.double(float64(p.AccentText.G)/255), C.double(float64(p.AccentText.B)/255))
}

// captureLastTouch / armReadingMarker are the initial-touch ("where I left off")
// bridge — an iOS-only feature (it needs touch). Desktop has no touch gesture and
// no marker, so these are inert here: nothing is recorded, nothing is drawn.
func captureLastTouch() (verse int, delta float64, ok bool) { return 0, 0, false }

func armReadingMarker(verse int, r, g, b float64) {}

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
