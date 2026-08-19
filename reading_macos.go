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
#cgo LDFLAGS: -framework AppKit -framework Foundation -framework QuartzCore

#import <AppKit/AppKit.h>
#import <QuartzCore/QuartzCore.h>   // CAShapeLayer, for the note bubble's outline
#import <stdlib.h>
#import <time.h>

// The note sticker, defined far below beside the rest of its machinery. Declared
// up here because bibleTextMacApplyHTML has to install the band right after it
// sets the text, and bibleTextMacScrollTV has to keep the sticker in view; both
// come first.
static void btMacRefreshNote(void);
static CGFloat btMacNoteTopY(void);

// Implemented in Go (ai_menu_darwin.go, //export). Called when the reader picks
// an AI study action; it copies both strings immediately. lo/hi is the
// selection's verse span, resolved against the verse-number runs of the text
// storage at action time (0,0 = unresolved) — position, not text, decides which
// verses a share or cross-reference cites.
extern void bibleTextAIMenuTapped(char *action, char *text, int lo, int hi);
// Sibling callback for the non-AI selection-menu actions (Share verse, …).
extern void bibleTextStudyMenuTapped(char *action, char *text, int lo, int hi);
// Defined below with the rest of the verse-number machinery (it needs the font
// threshold); declared here because the menu handlers above it use it.
static void btMacVerseSpanForRange(NSTextStorage *ts, NSRange sel, int *outLo, int *outHi);
// Posted when the reader scrolls by hand while read-along is live (audio_export_apple.go).
extern void bibleTextReadAlongUserScrolled(void);
// Posted when the floating "Follow narration" button is clicked (audio_export_apple.go).
extern void bibleTextReadAlongFollowTapped(void);
// The note sticker's three verbs (ai_menu_darwin.go, //go:build darwin — so the
// same exports already serve iOS's bubble; macOS calls them from the same kind
// of buttons for the same reasons).
extern void bibleTextNoteHidden(void);
extern void bibleTextNoteDeleted(void);
extern void bibleTextNoteRestored(void);
// The expanded sticker's count region ("2 of 3 on this passage ›") — advances
// focus to the next note on the passage, wrapping.
extern void bibleTextNoteNextTapped(void);

// Selection-menu AI gate, mirroring the Settings → Assistant choice ("None" turns
// AI off). Set from Go (btMacSetAIEnabled) when the reading host is built and
// whenever the setting changes; menuForEvent: reads it to include or omit the
// "Study with AI" submenu. Defaults to on, matching the preference's default.
// Whether to offer writing a note in the contextual menu. Pushed from Go on
// every reading-view build, like the iOS twin.
static int gBTMacNotesEnabled = 1;
void btMacSetNotesEnabled(int on) { gBTMacNotesEnabled = on; }

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
    if (gBTMacNotesEnabled) {
        NSMenuItem *sn = [[NSMenuItem alloc] initWithTitle:@"Share with note" action:@selector(hbShare_link_note:) keyEquivalent:@""];
        sn.target = self;
        [share addItem:sn];
    }
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

// hbSendAI / hbSendStudy read the selection ONCE — text and verse span from the
// same selectedRange, at action time — so the words dispatched and the span
// attributing them can never come from different selections.
- (void)hbSendAI:(const char *)action {
    int lo = 0, hi = 0;
    btMacVerseSpanForRange(self.textStorage, self.selectedRange, &lo, &hi);
    bibleTextAIMenuTapped((char *)action, (char *)self.hbSelectedText.UTF8String, lo, hi);
}
- (void)hbSendStudy:(const char *)action {
    int lo = 0, hi = 0;
    btMacVerseSpanForRange(self.textStorage, self.selectedRange, &lo, &hi);
    bibleTextStudyMenuTapped((char *)action, (char *)self.hbSelectedText.UTF8String, lo, hi);
}
- (void)hbAI_explain:(id)sender     { [self hbSendAI:"explain"]; }
- (void)hbAI_context:(id)sender     { [self hbSendAI:"context"]; }
- (void)hbAI_translation:(id)sender { [self hbSendAI:"translation"]; }
- (void)hbCrossRefs:(id)sender      { [self hbSendStudy:"crossref"]; }
- (void)hbShare_cite:(id)sender     { [self hbSendStudy:"share-cite"]; }
- (void)hbShare_image:(id)sender    { [self hbSendStudy:"share-image"]; }
- (void)hbShare_link_note:(id)sender { [self hbSendStudy:"share-link-note"]; }
- (void)hbShare_link:(id)sender     { [self hbSendStudy:"share-link"]; }
// Target of the floating "Follow narration" button (btMacEnsureFollowBtn) — the
// text view doubles as its action target so no extra controller object is needed.
- (void)hbFollowTapped:(id)sender {
    bibleTextReadAlongFollowTapped();
}
// Targets of the note sticker's buttons — same arrangement as the follow pill,
// and the same three verbs the iOS bubble posts.
- (void)btNoteHide:(id)sender    { bibleTextNoteHidden(); }
- (void)btNoteDelete:(id)sender  { bibleTextNoteDeleted(); }
- (void)btNoteRestore:(id)sender { bibleTextNoteRestored(); }
- (void)btNoteNext:(id)sender    { bibleTextNoteNextTapped(); }

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

// btMacVerseSpanForRange maps a selected character range to its verse span:
// lo/hi are the last verse-number runs at or before the range's first and last
// characters. A selection that starts inside a verse's NUMBER run starts at or
// after that run's location, so it resolves to that verse; one that starts
// above verse 1's number (the chapter heading) reports lo=0 and the Go side
// clamps to verse 1. macOS has no prebuilt verse index (unlike iOS's
// gVerseIndex — see btIOSBuildVerseIndex, built to unhook an O(n) walk from
// scroll-settle); this runs once per MENU ACTION, where one enumeration is
// nothing.
static void btMacVerseSpanForRange(NSTextStorage *ts, NSRange sel, int *outLo, int *outHi) {
    *outLo = 0; *outHi = 0;
    if (ts == nil || sel.length == 0 || NSMaxRange(sel) > ts.length) return;
    CGFloat thr = btMacVerseFontThreshold(ts);
    NSUInteger start = sel.location, last = NSMaxRange(sel) - 1;
    __block NSInteger lo = 0, hi = 0;
    [ts enumerateAttribute:NSFontAttributeName
                   inRange:NSMakeRange(0, ts.length)
                   options:0
                usingBlock:^(id val, NSRange r, BOOL *stop) {
        if (r.location > last) { *stop = YES; return; }
        if (val == nil || r.length == 0 || ((NSFont *)val).pointSize >= thr) return;
        NSInteger v = [[ts.string substringWithRange:r] integerValue];
        if (v <= 0) return;
        if (r.location <= start) lo = v;
        hi = v;
    }];
    *outLo = (int)lo;
    *outHi = (int)hi;
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
// gReadAlongVerse is the verse the narration is on, and it is the ONLY thing
// remembered about it — see the iOS twin for why the char range that used to sit
// beside it was the half that goes stale.
static NSInteger gReadAlongVerse = 0;
static NSColor *gReadAlongColor = nil;

// ---- The chapter's wash model ------------------------------------------------
//
// The macOS twin of reading_ios.go's block of the same name, and deliberately
// line-for-line its shape: what each verse's background SHOULD BE, pushed from
// Go (reading_tint_apple.go) out of the one function that answers that for every
// surface. A character has exactly ONE NSBackgroundColorAttributeName and two
// things want it — the chapter's wash and the narration's — so the narration
// cannot be allowed to "clear" what it did not put there.
#define BT_MAX_TINT_RUNS 32
typedef struct { int lo; int hi; double r, g, b, a; } BTTintRun;
typedef struct { int n; BTTintRun r[BT_MAX_TINT_RUNS]; } BTTintModel;
static BTTintModel gTint = {0};

// Which chapter the STORAGE holds, and the verse the narration is on — see the
// iOS twin for both, which this mirrors line for line. The short version: the
// model crosses as VERSE NUMBERS, so a failed import (bibleTextMacApplyHTML
// returns NO and leaves the previous chapter in place) would have the next
// mutation light verse 3 of the wrong chapter; and a body rebuild of the SAME
// chapter mid-playback must carry the narration wash across its own re-import
// rather than drop it until the next verse tick.
static int  gBodyGenPending = 0;
static int  gBodyGenApplied = 0;
static BOOL gTintUnpainted  = NO;
static NSInteger gPendingReadAlongVerse = 0;

// --- Timing, for the two paths this seam exists to tell apart -----------------
// Off unless BIBLETEXT_PERF is set in the environment. It prints the two costs
// the design claim rests on: the NSAttributedString re-import a wash change USED
// to pay, and the live range mutation it pays now.
static int btMacPerfOn(void) {
    static int on = -1;
    if (on < 0) { const char *e = getenv("BIBLETEXT_PERF"); on = (e && *e && *e != '0') ? 1 : 0; }
    return on;
}
static uint64_t btMacPerfNow(void) { return btMacPerfOn() ? clock_gettime_nsec_np(CLOCK_MONOTONIC) : 0; }
static void btMacPerfLog(const char *what, uint64_t t0) {
    if (!btMacPerfOn() || t0 == 0) return;
    NSLog(@"bibletext-perf: %s %.3f ms", what,
          (double)(clock_gettime_nsec_np(CLOCK_MONOTONIC) - t0) / 1e6);
}

// btMacTintRunForVerse returns the index of the run washing `verse`, or -1.
static int btMacTintRunForVerse(NSInteger verse) {
    for (int i = 0; i < gTint.n; i++)
        if (verse >= gTint.r[i].lo && verse <= gTint.r[i].hi) return i;
    return -1;
}

// EVERY COLOUR IN THIS BLOCK IS sRGB, AND THAT IS NOT A STYLE CHOICE.
//
// The wash a verse carries can arrive by two roads: the HTML importer reads
// `background-color: #ffe08a` out of buildChapterHTML's stylesheet and resolves
// it as sRGB, and the live mutation below builds an NSColor from the SAME three
// numbers. -colorWithCalibratedRed: reads them as NSCalibratedRGB, which is a
// different space: (255,224,138) there lands on sRGB (255,229,155). So a verse
// the mutation repainted came back a paler, greener gold than the one beside it
// that the import had painted — one mark rendered in two colours, permanently,
// because nothing rebuilds after a repaint. iOS never had it (UIColor's
// -colorWithRed: is sRGB), which is exactly the twin divergence tint.go's shared
// wash table exists to stop. -colorWithSRGBRed: is what the rest of this file
// already uses for palette colours crossing this boundary (btMacNoteColor).
static NSColor *btMacTintColor(int i) {
    if (i < 0 || i >= gTint.n) return nil;
    return [NSColor colorWithSRGBRed:gTint.r[i].r green:gTint.r[i].g
                                blue:gTint.r[i].b alpha:gTint.r[i].a];
}

// The narration wash, ONCE — see the iOS twin. sRGB for the same reason as
// btMacTintColor, and additionally so that the two halves of one narrated verse
// cannot disagree: btMacOverlayColor's arithmetic treats these constants as sRGB
// and emits sRGB, while btMacPaintVerseWash paints the tail beyond the chapter
// wash with this colour directly.
static const CGFloat kBTReadAlong[4] = {1.0, 0.80, 0.30, 0.32};

static NSColor *btMacReadAlongColor(void) {
    if (gReadAlongColor == nil)
        gReadAlongColor = [NSColor colorWithSRGBRed:kBTReadAlong[0] green:kBTReadAlong[1]
                                               blue:kBTReadAlong[2] alpha:kBTReadAlong[3]];
    return gReadAlongColor;
}

// btMacOverlayColor composites the narration wash OVER a chapter wash.
//
// The narration tint is translucent BECAUSE it is meant to be seen through —
// that is how the styled pane draws it, on its own layer above the verse wash.
// TextKit has no second layer here: one attribute, one colour. So the layering
// is done in arithmetic instead, source-over, and the note's gold shows through
// the narration's amber exactly as it does on the desktop styled pane.
//
// The base arrives from btMacTintColor, which is sRGB, so the conversion below
// is an identity today — it stays as the guard that the arithmetic and the
// colour it composites over are read in the SAME space, whatever a future
// caller hands in.
static NSColor *btMacOverlayColor(NSColor *base) {
    NSColor *b = [base colorUsingColorSpace:[NSColorSpace sRGBColorSpace]];
    if (b == nil) return btMacReadAlongColor();
    CGFloat br = 0, bg = 0, bb = 0, ba = 0;
    [b getRed:&br green:&bg blue:&bb alpha:&ba];
    const CGFloat ar = kBTReadAlong[0], ag = kBTReadAlong[1],
                  ab = kBTReadAlong[2], aa = kBTReadAlong[3];
    CGFloat oa = aa + ba * (1 - aa);
    if (oa <= 0) return btMacReadAlongColor();
    return [NSColor colorWithSRGBRed:(ar * aa + br * ba * (1 - aa)) / oa
                               green:(ag * aa + bg * ba * (1 - aa)) / oa
                                blue:(ab * aa + bb * ba * (1 - aa)) / oa
                               alpha:oa];
}
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
        // sRGB, like every other palette colour crossing this boundary (and like
        // the iOS pill, whose -colorWithRed: is sRGB) — the numbers are the app
        // palette's, and NSCalibratedRGB would land them somewhere else.
        NSForegroundColorAttributeName: [NSColor colorWithSRGBRed:gMacFollowFg[0]
                                                            green:gMacFollowFg[1]
                                                             blue:gMacFollowFg[2] alpha:1.0],
        NSParagraphStyleAttributeName: ps,
    };
    gMacFollowBtn.attributedTitle =
        [[NSAttributedString alloc] initWithString:@"Follow narration" attributes:attrs];
    gMacFollowBtn.layer.backgroundColor =
        [NSColor colorWithSRGBRed:gMacFollowBg[0] green:gMacFollowBg[1]
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

// btMacRunSpanRange is the whole character span of verses lo..hi, untrimmed.
static NSRange btMacRunSpanRange(NSTextStorage *ts, int lo, int hi) {
    NSRange a = btMacReadAlongRange(ts, lo);
    if (a.location == NSNotFound) return NSMakeRange(NSNotFound, 0);
    NSRange b = (hi > lo) ? btMacReadAlongRange(ts, hi) : a;
    NSUInteger end = (b.location == NSNotFound) ? NSMaxRange(a) : NSMaxRange(b);
    if (end < NSMaxRange(a)) end = NSMaxRange(a);
    if (end > ts.length) end = ts.length;
    if (a.location >= end) return NSMakeRange(NSNotFound, 0);
    return NSMakeRange(a.location, end - a.location);
}

// btMacRunWashRange is run i's OUTER bound: its span, trimmed of trailing
// whitespace. Per RUN, not per verse — see the iOS twin for why that distinction
// is the band's shape (the join between two washed verses is inside the band;
// the space after the last one is not). What is actually painted inside it is
// btMacPaintRunWash's business, which also takes the inter-verse breaks back out.
static NSRange btMacRunWashRange(NSTextStorage *ts, int i) {
    if (i < 0 || i >= gTint.n) return NSMakeRange(NSNotFound, 0);
    NSRange r = btMacRunSpanRange(ts, gTint.r[i].lo, gTint.r[i].hi);
    if (r.location == NSNotFound) return r;
    NSCharacterSet *ws = [NSCharacterSet whitespaceAndNewlineCharacterSet];
    while (r.length > 0 &&
           [ws characterIsMember:[ts.string characterAtIndex:NSMaxRange(r) - 1]])
        r.length--;
    return r.length > 0 ? r : NSMakeRange(NSNotFound, 0);
}

// btMacUnwashBreaks removes the background from every BREAK CHARACTER in a
// range, which is the one rule that makes a painted band the same shape as an
// imported one.
//
// MEASURED, not assumed. Feeding the importer
//     <span class="hl">washed one<br>washed two</span>
// and asking for the background at each break gives:
//     idx 16  BREAK(U+2028)  background=NONE
//     idx 33  BREAK(U+000A)  background=NONE
// — so a break is bare even when it sits INSIDE the highlighted span. TextKit
// paints a background-attributed break out to the right margin, which is why an
// unwashed one is not a detail: a band that keeps it grows a full-width tail off
// every poem line and every paragraph end, permanently, because nothing rebuilds
// after a live repaint.
//
// This replaces a narrower rule that stripped only the break before the NEXT
// VERSE NUMBER. That was written from the HTML markup, where an intra-verse <br>
// really is nested inside the .hl span — but what the markup CONTAINS and what
// the importer PAINTS are different questions, and only the second one decides
// pixels. The narrow rule left every poetic verse the wrong shape, on a
// single-verse mark, on both panes.
static void btMacUnwashBreaks(NSTextStorage *ts, NSRange r) {
    if (r.location == NSNotFound || r.length == 0) return;
    NSString *s = ts.string;
    NSUInteger end = NSMaxRange(r);
    if (end > s.length) return;
    NSCharacterSet *ws = [NSCharacterSet whitespaceAndNewlineCharacterSet];
    for (NSUInteger i = r.location; i < end; i++) {
        unichar ch = [s characterAtIndex:i];
        if (ch != '\n' && ch != '\r' && ch != 0x2028 && ch != 0x2029) continue;
        // The WHOLE whitespace run around the break: the reporter layout's
        // first-line indent (EM SPACE + EN SPACE) is written by the <p> that
        // FOLLOWS the break and is outside the span as well. A join space whose
        // run holds no break is inside the band and stays.
        NSUInteger lo = i, hi = i;
        while (lo > r.location && [ws characterIsMember:[s characterAtIndex:lo - 1]]) lo--;
        while (hi < end && [ws characterIsMember:[s characterAtIndex:hi]]) hi++;
        [ts removeAttribute:NSBackgroundColorAttributeName range:NSMakeRange(lo, hi - lo)];
        i = hi;
    }
}

// btMacChapterWashRange is the part of `verse` the CHAPTER wash covers: its own
// span inside the run, minus the break the HTML leaves bare between it and the
// verse after it.
static NSRange btMacChapterWashRange(NSTextStorage *ts, NSInteger verse) {
    NSRange run = btMacRunWashRange(ts, btMacTintRunForVerse(verse));
    NSRange v = btMacReadAlongRange(ts, verse);
    if (run.location == NSNotFound || v.location == NSNotFound) return NSMakeRange(NSNotFound, 0);
    NSRange x = NSIntersectionRange(run, v);
    if (x.length == 0) return NSMakeRange(NSNotFound, 0);
    return x.length > 0 ? x : NSMakeRange(NSNotFound, 0);
}

// btMacPaintRunWash paints run i's chapter wash the shape buildChapterHTML gives
// it: the whole span, then every BREAK CHARACTER inside it taken back out. See
// the iOS twin for why the difference is visible.
static void btMacPaintRunWash(NSTextStorage *ts, int i, NSColor *c) {
    if (i < 0 || i >= gTint.n || c == nil) return;
    NSRange r = btMacRunWashRange(ts, i);
    if (r.location == NSNotFound) return;
    [ts addAttribute:NSBackgroundColorAttributeName value:c range:r];
    btMacUnwashBreaks(ts, r);
}

// btMacPaintVerseWash sets the one background attribute each character of
// `verse` may carry, FROM THE MODEL — the chapter wash underneath, the narration
// over it, or their composite where both apply.
//
// This is the restoreTint / applyTint pair, and deliberately one function:
// narrated==NO is "put back what should be here now", narrated==YES is "and lay
// the narration over it". Written as two functions they would be two lists of
// what a verse's background can be, and the erasing bug this replaces was
// exactly one of those lists being shorter than the other.
static void btMacPaintVerseWash(NSTextStorage *ts, NSInteger verse, BOOL narrated) {
    if (verse <= 0 || ts == nil) return;
    NSRange whole = btMacReadAlongRange(ts, verse);
    if (whole.location == NSNotFound || NSMaxRange(whole) > ts.length) return;
    [ts removeAttribute:NSBackgroundColorAttributeName range:whole];
    NSRange wash = btMacChapterWashRange(ts, verse);
    NSColor *base = btMacTintColor(btMacTintRunForVerse(verse));
    if (wash.location != NSNotFound && base != nil)
        [ts addAttribute:NSBackgroundColorAttributeName
                   value:(narrated ? btMacOverlayColor(base) : base) range:wash];
    if (!narrated) {
        // A break is bare however it came to be painted — see btMacUnwashBreaks.
        btMacUnwashBreaks(ts, whole);
        return;
    }
    NSUInteger from = (wash.location != NSNotFound) ? NSMaxRange(wash) : whole.location;
    if (from < NSMaxRange(whole))
        [ts addAttribute:NSBackgroundColorAttributeName value:btMacReadAlongColor()
                   range:NSMakeRange(from, NSMaxRange(whole) - from)];
    btMacUnwashBreaks(ts, whole);
}

// btMacRefreshHighlightRange re-derives the scroll/tap target from the model,
// for the live-mutation path. bibleTextMacApplyHTML derives the same thing from
// the freshly imported background runs; here there is no import to enumerate.
static void btMacRefreshHighlightRange(NSTextStorage *ts) {
    NSRange u = (NSRange){NSNotFound, 0};
    for (int i = 0; i < gTint.n; i++) {
        NSRange r = btMacRunWashRange(ts, i);
        if (r.location == NSNotFound) continue;
        u = (u.location == NSNotFound) ? r : NSUnionRange(u, r);
    }
    gMacHighlightRange = u;
}

// bibleTextMacSetTintRuns replaces the chapter's wash model.
//
// repaint==0 means "the HTML about to arrive already carries this wash" — the
// full-rebuild path, where painting now would wash the OUTGOING chapter's verses
// for a frame. repaint==1 is the whole point of the seam: the text on screen is
// already right and only the wash changed, so the change is an attribute over a
// known range instead of buildChapterHTML plus a complete NSAttributedString
// re-import.
void bibleTextMacSetTintRuns(const BTTintRun *runs, int n, int repaint) {
    BTTintModel m; m.n = 0;
    if (n > BT_MAX_TINT_RUNS) n = BT_MAX_TINT_RUNS;
    for (int i = 0; i < n && runs != NULL; i++) m.r[m.n++] = runs[i];
    BOOL paint = repaint != 0;
    void (^block)(void) = ^{
        BTTintModel old = gTint;
        gTint = m;
        if (!paint || gTextView == nil) return;
        // The storage has to be the string this model was computed for — see the
        // gBodyGenPending block above, and the iOS twin.
        if (gBodyGenApplied != gBodyGenPending) {
            gTintUnpainted = YES;
            NSLog(@"bibletext(mac): wash mutation deferred — storage is not the pushed chapter");
            return;
        }
        NSTextStorage *ts = gTextView.textStorage;
        if (ts.length == 0) return;
        uint64_t t0 = btMacPerfNow();
        [ts beginEditing];
        // Clear the OLD wash first: the model is the only writer of these ranges,
        // so what it painted last time is exactly what has to come off, and a
        // verse that has just LOST its wash is otherwise left lit.
        for (int i = 0; i < old.n; i++) {
            NSRange r = btMacRunSpanRange(ts, old.r[i].lo, old.r[i].hi);
            if (r.location != NSNotFound)
                [ts removeAttribute:NSBackgroundColorAttributeName range:r];
        }
        for (int i = 0; i < gTint.n; i++) btMacPaintRunWash(ts, i, btMacTintColor(i));
        // The narration keeps its place whatever the chapter wash just did —
        // including when the wash it was sitting on has just gone away.
        if (gReadAlongVerse > 0) btMacPaintVerseWash(ts, gReadAlongVerse, YES);
        [ts endEditing];
        btMacRefreshHighlightRange(ts);
        // A wash change can arrive with the window long idle (a note cleared from
        // a menu, a link handled while the app was in the background), where a
        // coalesced display update can drop an attribute change's invalidation.
        [gTextView setNeedsDisplayInRect:gTextView.visibleRect];
        btMacPerfLog("tint-mutate", t0);
    };
    if ([NSThread isMainThread]) block();
    else dispatch_async(dispatch_get_main_queue(), block);
}

// bibleTextMacBeginChapterPush announces the body push about to be sent.
// sameChapter is 1 when it re-renders the chapter already on screen and 0 when it
// replaces it; a same-chapter push carries the live narration verse across its own
// re-import. The iOS twin, and see gPendingReadAlongVerse there for the whole of it.
void bibleTextMacBeginChapterPush(int sameChapter) {
    dispatch_block_t block = ^{
        gBodyGenPending++;
        gPendingReadAlongVerse = sameChapter ? gReadAlongVerse : 0;
    };
    if ([NSThread isMainThread]) block();
    else dispatch_async(dispatch_get_main_queue(), block);
}

// btMacRepaintChapterWashFromModel re-asserts the whole model over a freshly
// imported string — the recovery for a mutation refused while the storage was not
// the model's. See the iOS twin.
static void btMacRepaintChapterWashFromModel(void) {
    if (gTextView == nil) return;
    NSTextStorage *ts = gTextView.textStorage;
    if (ts.length == 0) return;
    [ts beginEditing];
    [ts removeAttribute:NSBackgroundColorAttributeName range:NSMakeRange(0, ts.length)];
    for (int i = 0; i < gTint.n; i++) btMacPaintRunWash(ts, i, btMacTintColor(i));
    [ts endEditing];
}

// bibleTextMacReadAlongClear takes the narration wash off, PUTTING BACK whatever
// the chapter says belongs on that verse rather than removing the attribute.
void bibleTextMacReadAlongClear(void) {
    // Reachable from the Fyne goroutine (main on macOS) but also from AVSpeechSynthesizer
    // delegate callbacks, whose thread is not documented — marshal to main like the iOS twin.
    if (![NSThread isMainThread]) {
        dispatch_async(dispatch_get_main_queue(), ^{ bibleTextMacReadAlongClear(); });
        return;
    }
    if (gTextView == nil) return;
    NSTextStorage *ts = gTextView.textStorage;
    if (gReadAlongVerse > 0) {
        [ts beginEditing];
        btMacPaintVerseWash(ts, gReadAlongVerse, NO);
        [ts endEditing];
        // Unlike the per-tick highlight (always mid-playback, window active), this
        // clear can fire after long idle — e.g. closing the audio card much later —
        // where a coalesced/napped display update can drop the attribute change's
        // invalidation. Force the repaint so the tint never visibly lingers.
        [gTextView setNeedsDisplayInRect:gTextView.visibleRect];
    }
    gReadAlongVerse = 0;
    gReadAlongActive = NO;
    gReadAlongUserLatch = NO;
}

// bibleTextMacHighlightVerse tints the narrated verse (restoring the previous one
// to whatever the chapter says belongs on it) and follow-scrolls only when the
// verse has drifted out of a comfortable band, so the text isn't yanked on every
// verse. verse<=0 just clears (recording's intro).
//
// MOVING OFF A VERSE IS A REPAINT, NOT AN ERASE. This used to removeAttribute
// over the range it had tinted, which is only correct when nothing was
// underneath — over a verse carrying a note or a search hit it deleted the
// reader's own mark as the audio walked past. btMacPaintVerseWash asks the model.
void bibleTextMacHighlightVerse(int verse, int follow) {
    if (![NSThread isMainThread]) {   // see bibleTextMacReadAlongClear
        dispatch_async(dispatch_get_main_queue(), ^{ bibleTextMacHighlightVerse(verse, follow); });
        return;
    }
    if (gTextView == nil) return;
    NSTextStorage *ts = gTextView.textStorage;
    uint64_t t0 = btMacPerfNow();
    [ts beginEditing];
    if (gReadAlongVerse > 0) btMacPaintVerseWash(ts, gReadAlongVerse, NO);
    gReadAlongVerse = 0;
    NSRange painted = NSMakeRange(NSNotFound, 0);
    if (verse > 0) {
        NSRange r = btMacReadAlongRange(ts, verse);
        if (r.location != NSNotFound && NSMaxRange(r) <= ts.length) {
            btMacPaintVerseWash(ts, verse, YES);
            gReadAlongVerse = verse;
            painted = r;   // local: the follow-scroll below is its only reader
        }
    }
    [ts endEditing];
    if (btMacPerfOn()) NSLog(@"bibletext-perf: readalong-verse %d", verse);
    btMacPerfLog("readalong-move", t0);
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
    if (painted.location != NSNotFound) {
        NSLayoutManager *lm = gTextView.layoutManager;
        NSRange g = [lm glyphRangeForCharacterRange:painted actualCharacterRange:NULL];
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
// btMacScrollToHighlight lands the view on the chapter's wash, returning NO when
// there is none to land on. Factored out of bibleTextMacScrollTV so the
// reposition can be issued WITHOUT a re-import — see the iOS twin
// (btIOSScrollToHighlight) for why that is the whole point of the seam.
//
// The frame-origin normalisation stays with the CALLER, because both branches
// below need it and this one can also be reached on its own.
static BOOL btMacScrollToHighlight(void) {
    if (gTextView == nil || gScroll == nil) return NO;
    { NSRect tf = gTextView.frame; if (tf.origin.y != 0) { tf.origin.y = 0; [gTextView setFrame:tf]; } }
    if (gMacHighlightRange.location == NSNotFound ||
        gMacHighlightRange.length == 0 ||
        NSMaxRange(gMacHighlightRange) > gTextView.textStorage.length) return NO;
    NSLayoutManager *lm = gTextView.layoutManager;
    NSRange glyphs = [lm glyphRangeForCharacterRange:gMacHighlightRange
                                actualCharacterRange:NULL];
    NSRect rect = [lm boundingRectForGlyphRange:glyphs
                                inTextContainer:gTextView.textContainer];
    CGFloat y = rect.origin.y + gTextView.textContainerInset.height - 16;
    // WHEN THERE IS A NOTE, LAND ON THE NOTE. The sticker sits in a band ABOVE
    // the paragraph holding the highlighted verse, so scrolling to the verse
    // pushes the message off the top — and the message is why the link was sent.
    // Taken as a MINIMUM rather than a substitution, so it can only ever scroll
    // further up: nothing can put the note out of view. (The iOS twin does the
    // same in its scroll path; without it here the bubble was drawn correctly and
    // simply never seen.)
    CGFloat noteY = btMacNoteTopY();
    if (noteY >= 0 && noteY - 12 < y) y = noteY - 12;
    if (y < 0) y = 0;
    [[gScroll contentView] scrollToPoint:NSMakePoint(0, y)];
    [gScroll reflectScrolledClipView:gScroll.contentView];
    return YES;
}

// bibleTextMacScrollToHighlight is the REPOSITION half of an arrival, on its own
// — the macOS twin of bibleTextIOSScrollToHighlight, and the reason
// state.forceReposition no longer has to mean "re-import the chapter".
void bibleTextMacScrollToHighlight(void) {
    dispatch_block_t block = ^{ btMacScrollToHighlight(); };
    if ([NSThread isMainThread]) block();
    else dispatch_async(dispatch_get_main_queue(), block);
}

static void bibleTextMacScrollTV(void) {
    if (gTextView == nil || gScroll == nil) return;
    // Programmatic scrolling (e.g. read-along follow-scroll) can leave the
    // verticallyResizable text view's frame origin non-zero inside the clip view.
    // Every case below computes its target in frame space and scrolls the clip view,
    // which assumes the document sits at origin 0 — a stale origin would land the
    // content offset below the top (a gap above verse 1). Normalize it first.
    { NSRect tf = gTextView.frame; if (tf.origin.y != 0) { tf.origin.y = 0; [gTextView setFrame:tf]; } }
    // ORDER MATTERS, and it is restore-before-highlight — the same rule iOS
    // states and for the same reason (reading_ios.go, bibleTextIOSScrollTV).
    //
    // A pending restore only ever exists on a REOPEN: every explicit arrival —
    // a tapped link, a note, a search result — clears it (share_link_open.go,
    // openSearchResultRange) precisely so it falls through to the highlight
    // below. So an armed restore means "the reader is coming back", and coming
    // back should land where they stopped reading, not on whatever happens to
    // be highlighted there. This pane had the two the other way round, so a
    // chapter carrying a note or a search hit dragged the reader to it on every
    // reopen — the defect iOS records as fixed, still live here and on the
    // Windows/Linux pane until 19 Aug 2026.
    //
    // The restore only WINS if it resolves: the y >= 0 guard inside falls
    // through to the highlight when the verse is gone and no fraction is
    // usable, which is iOS's behaviour too.
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
    if (btMacScrollToHighlight()) return;
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
    uint64_t t0 = btMacPerfNow();
    NSMutableAttributedString *as =
        [[NSMutableAttributedString alloc] initWithData:data options:opts
                                     documentAttributes:nil error:&err];
    if (as == nil) return NO;
    btMacPerfLog("html-import", t0);
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
    // New chapter text: a narration wash from the previous chapter must not be
    // "restored" against the new storage (audio already stopped via stopAudioForNav).
    // ALL THREE, like the iOS twin: gReadAlongActive left YES with nothing painted
    // is a lie, and the bounds observer above acts on it — one scroll belonging to
    // no live narration would latch gReadAlongUserLatch and kill follow-scroll for
    // the rest of the recording.
    gReadAlongVerse = 0;
    gReadAlongActive = NO;
    gReadAlongUserLatch = NO;
    [gTextView.textStorage setAttributedString:as];
    // The storage now holds the chapter Go announced; anything refused while it
    // did not gets re-asserted. See gBodyGenPending.
    gBodyGenApplied = gBodyGenPending;
    if (gTintUnpainted) { btMacRepaintChapterWashFromModel(); gTintUnpainted = NO; }
    // WHERE THE HIGHLIGHT RANGE COMES FROM: the MODEL, on this path as well as on
    // the mutation path (btMacRefreshHighlightRange), so the import and the
    // mutation cannot give different scroll targets for the same chapter. It used
    // to take the FIRST imported background run and stop, which is the union's
    // answer only while there is one run — and this whole model exists for the
    // plural case. (The iOS twin unions on both paths for the same reason.)
    btMacRefreshHighlightRange(gTextView.textStorage);
    // Put the narration back if it is still live on this chapter — a body rebuild
    // mid-playback (hiding a note, a theme flip, a text-size change) otherwise
    // drops its wash until the next verse tick. Painted, not follow-scrolled: the
    // scroll below owns where the view lands.
    if (gPendingReadAlongVerse > 0) {
        NSTextStorage *rts = gTextView.textStorage;
        NSRange rr = btMacReadAlongRange(rts, gPendingReadAlongVerse);
        if (rr.location != NSNotFound && NSMaxRange(rr) <= rts.length) {
            [rts beginEditing];
            btMacPaintVerseWash(rts, gPendingReadAlongVerse, YES);
            [rts endEditing];
            gReadAlongVerse = gPendingReadAlongVerse;
            gReadAlongActive = YES;
        }
    }
    // AFTER the text and after the paragraphSpacingBefore-zeroing pass above —
    // the note's band is a spacing-before on the anchor paragraph, so installing
    // it any earlier would be wiped by that pass, and the anchor is a VERSE,
    // which needs the text present to resolve to a character location.
    btMacRefreshNote();
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
// The reporter measure (27.5em × body px), the NSTextView twin of the iPad's
// bibleTextSetReadingMeasure: cap the text column and centre it by growing the
// horizontal textContainerInset. NSTextView applies the inset width to BOTH
// sides and the container tracks the view width, so one number does the whole
// job. Floors at the legacy 16pt so a narrow window degrades to today's look.
static CGFloat gMacReadingMeasure = 0;
static void btMacApplyInsets(CGFloat w) {
    if (gTextView == nil || w <= 0) return;
    CGFloat side = 16;
    if (gMacReadingMeasure > 0) {
        side = floor((w - gMacReadingMeasure) / 2.0);
        if (side < 16) side = 16;
    }
    NSSize cur = gTextView.textContainerInset;
    if (fabs(cur.width - side) < 0.5) return;
    gTextView.textContainerInset = NSMakeSize(side, cur.height);
    [gTextView.layoutManager ensureLayoutForTextContainer:gTextView.textContainer];
}

void bibleTextMacSetReadingMeasure(double m) {
    dispatch_async(dispatch_get_main_queue(), ^{
        gMacReadingMeasure = (CGFloat)m;
        if (gScroll != nil) btMacApplyInsets(gScroll.contentSize.width);
    });
}

// ─── The note sticker ────────────────────────────────────────────────────────
//
// The AppKit twin of the iOS bubble (reading_ios.go). Ported because the Fyne
// BANNER that stood in for it here could not do the three things that make a
// note read as somebody talking about a passage: it sat above the pane so it
// never moved with the text, it was a rectangle with no tail pointing at
// anything, and the collapsed chip was frozen at the top of the pane whatever
// verse the note belonged to. All three are the same defect — the banner lives
// outside the text, and the note belongs inside it.
//
// The trick is the same one iOS uses: the sticker is a subview of the TEXT
// VIEW, whose frame is in content coordinates, so it scrolls with the passage
// for nothing. The band it sits in is reserved by giving the anchor paragraph a
// paragraphSpacingBefore, so the sticker is part of the layout rather than
// floated over it and no verse is ever covered.
//
// AppKit differences from the iOS original, all of which bit during the port:
//   · NSTextView IS flipped, so y-down arithmetic carries over unchanged — but
//     NSBezierPath's arc angles are DEGREES and are measured counter-clockwise
//     in the view's own (flipped) space, which inverts the sense of `clockwise`
//     relative to UIBezierPath. The path below is wound to match the iOS one
//     visually, not textually.
//   · textContainerInset is an NSSize, so its height applies to the top AND the
//     bottom. Reserving the first-paragraph band there costs an equal strip of
//     empty space after the last verse; harmless, and the alternative (a
//     spacing-before that collapses at the top of a container) is what the iOS
//     code had to work around too.
//   · NSTextField/NSButton replace UILabel/UIButton, and a plain NSView needs
//     wantsLayer before a CAShapeLayer will draw into it.
// The sticker's container is FLIPPED. AppKit views are y-up by default, and the
// whole bubble — the bezier outline, the label frames, the button frames — is
// ported from UIKit, which is y-down. Drawn in a stock NSView the port came out
// upside down in every particular: the speech tail pointed UP out of the card's
// shoulder instead of down at the passage, and "Note from Friend" sat BELOW the
// message it attributes. One override fixes all of it at the root, which is why
// this is a flipped container rather than a pile of h-minus-y arithmetic.
@interface BTFlippedView : NSView
@end
@implementation BTFlippedView
- (BOOL)isFlipped { return YES; }

// THE CURSOR. The sticker is a subview of an NSTextView, and a text view claims
// the I-beam across its whole area — so the pointer stayed an I-beam over the
// card and, worse, over its buttons and the collapsed chip, which read as "this
// is text you can select" when it is in fact a thing you press.
//
// Cursor rects are the AppKit answer and they resolve innermost-first, so a rect
// on this view wins over the text view's. The card itself takes the arrow (it is
// a message, not selectable text — the note is deliberately not part of the text
// model), and anything pressable takes the pointing hand.
- (void)resetCursorRects {
    [self addCursorRect:self.bounds cursor:[NSCursor arrowCursor]];
    for (NSView *sub in self.subviews) {
        if ([sub isKindOfClass:[NSButton class]]) {
            [self addCursorRect:sub.frame cursor:[NSCursor pointingHandCursor]];
        }
    }
}
@end

static NSView       *gMacNoteView = nil;
static CAShapeLayer *gMacNoteCard = nil;
static NSString     *gMacNoteText = nil;      // the sender's words, ALONE (S9)
static NSString     *gMacNoteWho = nil;       // the app's chrome: byline + counts, composed in Go
static BOOL          gMacNoteMinimized = NO;  // pushed pill presentation (minimize OR suppression)
static BOOL          gMacNoteNextable = NO;   // the count region is a control (S10 next-tap)
static NSInteger     gMacNoteAnchorVerse = 0;
static CGFloat       gMacNoteBandH = 0;
static CGFloat       gMacNoteTopInset = 0;
static CGFloat gMacNoteBg[3]     = {0.99, 0.98, 0.97};
static CGFloat gMacNoteFg[3]     = {0.15, 0.13, 0.11};
static CGFloat gMacNoteMuted[3]  = {0.42, 0.39, 0.34};
static CGFloat gMacNoteAccent[3] = {0.18, 0.30, 0.53};
static CGFloat gMacNoteBorder[3] = {0.74, 0.70, 0.62};

static NSColor *btMacNoteColor(CGFloat c[3]) {
    return [NSColor colorWithSRGBRed:c[0] green:c[1] blue:c[2] alpha:1.0];
}
static NSFont *btMacNoteBodyFont(void) { return [NSFont systemFontOfSize:13]; }
static NSFont *btMacNoteWhoFont(void)  { return [NSFont systemFontOfSize:10 weight:NSFontWeightSemibold]; }

// THE SHARED NOTE SPACING SPEC — noteMetrics in notes_bubble.go. These are not
// this file's numbers to choose: notes_spacing_spec_test.go parses these lines
// and fails if any of them leaves the Go table behind. kMacNoteWho is 13 rather
// than iOS's 14 because the spec states the who row as a RULE — ceil(whoSize ×
// 1.27) — and this pane's who font is 10pt, not 11; a flat 14 in the table would
// be the wrong box here.
static const CGFloat kMacNoteGapAbove = 10, kMacNoteGapBelow = 10, kMacNotePad = 12;
// The pill's side padding and width floor — spec (noteMetrics PillPadX /
// PillMinW). This pane had 12/76 of its own, which made the same "Notes · 3"
// visibly narrower here than on the phone beside it.
static const CGFloat kMacNotePillPadX = 14, kMacNotePillMinW = 86;
static const CGFloat kMacNoteWho = 13, kMacNoteWhoGap = 4, kMacNotePill = 28, kMacNoteRad = 10;
// kMacNoteBtn is NOT spec: it is the verb button's size, this platform's 24pt
// pointer target. The pill used to borrow it (24), which is how a pointer-target
// decision came to set the height of a piece of content.
static const CGFloat kMacNoteBtn = 24;
static const CGFloat kMacNoteTail = 9, kMacNoteTailW = 18, kMacNoteTailX = 24;
// THE ONE DELIBERATE RESIDUAL IN THE SPEC, and it lives here.
//
// Every other surface reserves exactly noteMetrics.GapAbove above the card.
// This one reserves max(GapAbove, one line of the anchor paragraph's own font),
// which at the reporter leading is roughly 24pt rather than 10 — so a macOS card
// carries about 14pt more air above it than an iOS, Android or styled-pane card
// does. It is kept because removing it reintroduces a defect that was measured
// on rendered pixels and reported from the field, not because the number is
// prettier.
//
// What it corrects: at the reporter layout's leading the preceding line's INK
// overhangs the bottom of its own line box by most of a line. Place the card
// 10pt below that box — which every rect the layout manager will sell you says
// is clear — and it still lands across the bottom half of the text above it.
// This is a MEASUREMENT CORRECTION for TextKit's fragment geometry, not a design
// gap, and reading it off the font keeps it right at all three text sizes
// instead of at whichever one it was tuned against.
//
// It is also why this pane places the sticker BOTTOM-UP off the passage
// (btMacNoteStickerY): the tail→passage distance is the pinned invariant, and
// any slack in the correction falls into the gap above, where there is already
// air.
static CGFloat btMacNoteTopGap(NSTextStorage *ts, NSRange para) {
    const CGFloat floorGap = kMacNoteGapAbove;   // the spec's own reservation
    if (ts == nil || para.length < 3) return floorGap;
    // A character near the END of the paragraph: its start is the verse number,
    // which is a superscript in a smaller font, and measuring that was what made
    // an earlier attempt at this compute a clearance of zero.
    NSFont *f = [ts attribute:NSFontAttributeName atIndex:NSMaxRange(para) - 2 effectiveRange:NULL];
    if (f == nil) return floorGap;
    CGFloat natural = ceil(f.ascender - f.descender + f.leading);
    return natural > floorGap ? natural : floorGap;
}

static void btMacInstallNote(void);
static void btMacLayoutNote(void);

// btMacNotePresent / btMacNotePill — the two questions every sticker function
// asks since S9, twins of the iOS pair. PRESENT: an unplaced-only chapter
// pushes NO text but a who sentence — the sticker exists whenever either
// does. PILL: the pushed minimized flag (a reader's minimize OR the plan's
// derived suppression), and ALSO an empty text — no sender words exist, and
// an empty sender bubble must never render, so "who without text" collapses
// to the pill by construction.
static BOOL btMacNotePresent(void) { return gMacNoteText != nil || gMacNoteWho != nil; }
static BOOL btMacNotePill(void)    { return gMacNoteMinimized || gMacNoteText == nil; }

static BOOL btMacSameStr(NSString *a, NSString *b) {
    return a == b || (a != nil && b != nil && [a isEqualToString:b]);
}

// btMacFitWho keeps the WHO line's counts whole when the label is too narrow:
// everything before the first " · " is the sender half, and it alone is
// tail-truncated — never the count (the iOS twin is btIOSFitWho; keep them
// matched). With no separator the field's own tail truncation stands; in the
// degenerate case the sender half becomes a bare ellipsis.
static NSString *btMacFitWho(NSString *who, CGFloat width, NSFont *font) {
    if (who == nil || width <= 0) return who;
    NSDictionary *attrs = @{NSFontAttributeName: font};
    if ([who sizeWithAttributes:attrs].width <= width) return who;
    NSRange sep = [who rangeOfString:@" · "];
    if (sep.location == NSNotFound) return who;
    NSString *counts = [who substringFromIndex:sep.location]; // " · 1 of 3 …"
    NSString *sender = [who substringToIndex:sep.location];
    CGFloat avail = width - ceil([counts sizeWithAttributes:attrs].width);
    while (sender.length > 0) {
        NSString *cand = [sender stringByAppendingString:@"…"];
        if (ceil([cand sizeWithAttributes:attrs].width) <= avail) {
            return [cand stringByAppendingString:counts];
        }
        NSRange last = [sender rangeOfComposedCharacterSequenceAtIndex:sender.length - 1];
        sender = [sender substringToIndex:last.location];
    }
    return [@"…" stringByAppendingString:counts];
}

// btMacWhoCountRange is the "K of N on this passage" span of a (fitted) WHO
// line — the iOS twin is btIOSWhoCountRange; keep them matched. The first-
// separator split is safe against a sender's name by construction:
// sanitizeSenderName maps the middle dot away (notes_byline.go).
static NSRange btMacWhoCountRange(NSString *who) {
    NSRange sep = [who rangeOfString:@" · "];
    if (sep.location == NSNotFound) return NSMakeRange(NSNotFound, 0);
    NSUInteger start = NSMaxRange(sep);
    NSRange rest = NSMakeRange(start, who.length - start);
    NSRange next = [who rangeOfString:@" · " options:0 range:rest];
    NSUInteger end = (next.location == NSNotFound) ? who.length : next.location;
    return NSMakeRange(start, end - start);
}

// The bubble's whole outline — card and speech tail — as ONE continuous path,
// for the reason spelled out on the iOS twin: drawn as two shapes, the card's
// bottom stroke runs straight across the mouth of the tail and no z-ordering
// removes it, because the card must be on top for its fill to hide the tail's
// base. Walking the bottom edge and detouring into the tail leaves no crossing
// line to hide.
static NSBezierPath *btMacNoteBubblePath(CGFloat w, CGFloat h) {
    const CGFloat r = kMacNoteRad;   // the card's corner radius (spec)
    const CGFloat in = 0.5;   // half the 1pt stroke, kept inside bounds
    CGFloat left = in, top = in, right = w - in, bottom = h - in;
    CGFloat tx0 = kMacNoteTailX, tx1 = kMacNoteTailX + kMacNoteTailW;
    // The apex is kMacNoteTail below the card's bottom EDGE. The "- 1" that used
    // to be here survived from the two-shape era, when the tail's top row had to
    // overlap the card's bottom border by a point to hide it; a single outline has
    // no border to cover, so it only made the drawn tail a point shorter than the
    // constant naming it — and a point shorter than the other three surfaces'.
    CGFloat apexX = kMacNoteTailX + kMacNoteTailW / 2, apexY = bottom + kMacNoteTail;

    NSBezierPath *p = [NSBezierPath bezierPath];
    [p moveToPoint:NSMakePoint(left + r, top)];
    [p lineToPoint:NSMakePoint(right - r, top)];
    [p appendBezierPathWithArcWithCenter:NSMakePoint(right - r, top + r) radius:r
                             startAngle:270 endAngle:360 clockwise:NO];
    [p lineToPoint:NSMakePoint(right, bottom - r)];
    [p appendBezierPathWithArcWithCenter:NSMakePoint(right - r, bottom - r) radius:r
                             startAngle:0 endAngle:90 clockwise:NO];
    [p lineToPoint:NSMakePoint(tx1, bottom)];
    [p lineToPoint:NSMakePoint(apexX, apexY)];   // the tail's point, aimed at the passage
    [p lineToPoint:NSMakePoint(tx0, bottom)];
    [p lineToPoint:NSMakePoint(left + r, bottom)];
    [p appendBezierPathWithArcWithCenter:NSMakePoint(left + r, bottom - r) radius:r
                             startAngle:90 endAngle:180 clockwise:NO];
    [p lineToPoint:NSMakePoint(left, top + r)];
    [p appendBezierPathWithArcWithCenter:NSMakePoint(left + r, top + r) radius:r
                             startAngle:180 endAngle:270 clockwise:NO];
    [p closePath];
    return p;
}

// The height of the CARD at this width. The view is this plus the tail, and the
// reserved band is the view plus the gap. Measured BEFORE the band is reserved,
// because the band's height IS this number.
static CGFloat btMacNoteHeightForWidth(CGFloat w) {
    if (!btMacNotePresent()) return 0;
    if (btMacNotePill()) return kMacNotePill;    // the collapsed pill has no tail
    CGFloat inner = w - 2 * kMacNotePad;
    if (inner < 40) inner = 40;
    NSRect r = [gMacNoteText boundingRectWithSize:NSMakeSize(inner, CGFLOAT_MAX)
                                          options:(NSStringDrawingUsesLineFragmentOrigin |
                                                   NSStringDrawingUsesFontLeading)
                                       attributes:@{NSFontAttributeName: btMacNoteBodyFont()}];
    return kMacNotePad + kMacNoteWho + kMacNoteWhoGap + ceil(r.size.height) + kMacNotePad;
}

// The character range the note is anchored to. The note carries its OWN verse
// rather than borrowing the highlight's, because minimizing CLEARS the
// highlight — and a sticker that loses its anchor jumps to the top of the
// chapter, which is precisely the "frozen at the top" the banner did always.
static NSRange btMacNoteAnchorRange(NSTextStorage *ts, NSUInteger len) {
    if (gMacNoteAnchorVerse > 0 && ts != nil) {
        NSUInteger loc = btMacLocForVerse(ts, gMacNoteAnchorVerse);
        if (loc != NSNotFound && loc < len) return NSMakeRange(loc, 0);
    }
    if (gMacHighlightRange.location != NSNotFound && NSMaxRange(gMacHighlightRange) <= len) {
        return gMacHighlightRange;
    }
    return NSMakeRange(0, 0);
}

// Apply (or clear) the first-paragraph reservation, which rides the container
// inset because paragraphSpacingBefore collapses at the top of a container.
static void btMacApplyNoteInset(void) {
    if (gTextView == nil) return;
    NSSize ins = gTextView.textContainerInset;
    CGFloat wanted = 14 + gMacNoteTopInset;   // 14 is the pane's own top inset
    if (fabs(ins.height - wanted) > 0.5) {
        gTextView.textContainerInset = NSMakeSize(ins.width, wanted);
    }
}

// Reserve the band in the text. Runs AFTER the text is in the view, because the
// anchor is a VERSE and the verse index is what turns that into a character
// location — and after the pass that zeroes paragraphSpacingBefore across the
// string (bibleTextMacApplyHTML), or the reservation would be wiped on every
// render.
// gMacNoteReservedPara: the paragraph currently carrying the band, or
// NSNotFound — the iOS twin's gNoteReservedPara, for the same reason: the
// next-tap (S10) can move the sticker to another verse of the string already
// on screen, and no re-import wipes the old reservation, so the install must
// take it back itself. On a fresh import the zeroing pass has already cleared
// every paragraphSpacingBefore, so the guarded clear is a no-op there.
static NSRange gMacNoteReservedPara = {NSNotFound, 0};

static void btMacClearReservedPara(NSTextStorage *ts) {
    NSRange old = gMacNoteReservedPara;
    gMacNoteReservedPara = NSMakeRange(NSNotFound, 0);
    if (old.location == NSNotFound || ts == nil || NSMaxRange(old) > ts.length) return;
    NSParagraphStyle *base = [ts attribute:NSParagraphStyleAttributeName atIndex:old.location
                            effectiveRange:NULL];
    if (base == nil || base.paragraphSpacingBefore == 0) return;
    NSMutableParagraphStyle *ps = [base mutableCopy];
    ps.paragraphSpacingBefore = 0;
    [ts beginEditing];
    [ts addAttribute:NSParagraphStyleAttributeName value:ps range:old];
    [ts endEditing];
}

static void btMacInstallNote(void) {
    gMacNoteBandH = 0;
    gMacNoteTopInset = 0;
    if (gTextView == nil) return;
    NSTextStorage *ts = gTextView.textStorage;
    btMacClearReservedPara(ts);   // the sticker may have MOVED (next-tap): take the old band back
    if (!btMacNotePresent() || ts.length == 0) { btMacApplyNoteInset(); return; }

    CGFloat w = gTextView.textContainer.size.width - 2 * gTextView.textContainer.lineFragmentPadding;
    CGFloat h = btMacNoteHeightForWidth(w);
    if (h <= 0) { btMacApplyNoteInset(); return; }
    // The paragraph first: the top gap is read off ITS font, so the band cannot
    // be sized before we know which paragraph it belongs to.
    NSRange anchor = btMacNoteAnchorRange(ts, ts.length);
    NSRange para = [ts.string paragraphRangeForRange:anchor];
    if (para.location == NSNotFound || NSMaxRange(para) > ts.length) {
        btMacApplyNoteInset();
        return;
    }
    gMacNoteBandH = btMacNoteTopGap(ts, para) + h + (btMacNotePill() ? 0 : kMacNoteTail) + kMacNoteGapBelow;
    if (getenv("BT_NOTE_GEOM")) fprintf(stderr, "[geom] install: w=%.1f h=%.1f topGap=%.1f bandH=%.1f para={%lu,%lu}\n",
        w, h, btMacNoteTopGap(ts, para), gMacNoteBandH, (unsigned long)para.location, (unsigned long)para.length);
    if (para.location == 0) {
        gMacNoteTopInset = gMacNoteBandH;
        btMacApplyNoteInset();
        return;
    }
    btMacApplyNoteInset();   // clears any inset left by a previous chapter

    NSParagraphStyle *base = [ts attribute:NSParagraphStyleAttributeName atIndex:para.location
                            effectiveRange:NULL];
    NSMutableParagraphStyle *ps = base ? [base mutableCopy] : [[NSMutableParagraphStyle alloc] init];
    ps.paragraphSpacingBefore = gMacNoteBandH;
    [ts beginEditing];
    [ts addAttribute:NSParagraphStyleAttributeName value:ps range:para];
    [ts endEditing];
    gMacNoteReservedPara = para;   // so a moved sticker can take this band back
}

// Build (or rebuild) the sticker's subviews for the current note and palette.
static void btMacEnsureNoteView(void) {
    if (gMacNoteView) { [gMacNoteView removeFromSuperview]; gMacNoteView = nil; }
    gMacNoteCard = nil;
    if (!btMacNotePresent() || gTextView == nil) return;

    NSView *box = [[BTFlippedView alloc] initWithFrame:NSZeroRect];
    box.wantsLayer = YES;                      // no layer, no CAShapeLayer
    // A flipped view's layer needs telling too, or the CAShapeLayer path (which
    // is authored y-down, like the rest of the port) draws mirrored inside it.
    box.layer.geometryFlipped = YES;
    box.layer.backgroundColor = NSColor.clearColor.CGColor;

    if (btMacNotePill()) {
        // The collapsed marker is a plain pill — no tail, exactly as on iOS and
        // on the web. Since S9 it carries the pushed WHO composition
        // ("Notes · 3", or the unplaced-only sentence), so minimizing the open
        // note does not make the rest of the set invisible. The press is the
        // Restore verb; with no note text behind it (unplaced-only) the Go
        // side is a no-op, so the pill is inert exactly when there is nothing
        // to restore.
        box.layer.backgroundColor = btMacNoteColor(gMacNoteBg).CGColor;
        box.layer.borderColor = btMacNoteColor(gMacNoteBorder).CGColor;
        box.layer.borderWidth = 1;
        box.layer.cornerRadius = kMacNotePill / 2;

        NSButton *chip = [NSButton buttonWithTitle:(gMacNoteWho ?: @"Note")
                                            target:gTextView action:@selector(btNoteRestore:)];
        chip.bezelStyle = NSBezelStyleInline;
        chip.bordered = NO;
        chip.font = btMacNoteWhoFont();
        chip.contentTintColor = btMacNoteColor(gMacNoteMuted);
        chip.tag = 901;
        [box addSubview:chip];
    } else {
        gMacNoteCard = [CAShapeLayer layer];
        gMacNoteCard.fillColor = btMacNoteColor(gMacNoteBg).CGColor;
        gMacNoteCard.strokeColor = btMacNoteColor(gMacNoteBorder).CGColor;
        gMacNoteCard.lineWidth = 1;
        [box.layer addSublayer:gMacNoteCard];

        // The WHO line is the app's chrome — the byline and the counts,
        // pushed from Go (appleStickerPush) — never the sender's words. The
        // fallback keeps the frame honest if a push ever missed.
        NSTextField *who = [NSTextField labelWithString:(gMacNoteWho ?: @"Note from Friend")]; // a person, never the app
        who.font = btMacNoteWhoFont();
        who.textColor = btMacNoteColor(gMacNoteMuted);
        who.lineBreakMode = NSLineBreakByTruncatingTail;
        who.tag = 902;
        [box addSubview:who];

        // TEXT, never markup — the note is somebody else's words and nothing
        // here parses them.
        NSTextField *body = [NSTextField labelWithString:gMacNoteText];
        body.font = btMacNoteBodyFont();
        body.textColor = btMacNoteColor(gMacNoteFg);
        body.lineBreakMode = NSLineBreakByWordWrapping;
        body.maximumNumberOfLines = 0;
        body.tag = 903;
        [box addSubview:body];

        // Minimize first, delete second: the destructive one is never what the
        // pointer reaches by accident.
        NSButton *hide = [NSButton buttonWithTitle:@"–" target:gTextView action:@selector(btNoteHide:)];
        hide.bordered = NO;
        hide.font = [NSFont systemFontOfSize:17 weight:NSFontWeightMedium];
        hide.contentTintColor = btMacNoteColor(gMacNoteMuted);
        hide.tag = 904;
        [box addSubview:hide];

        NSButton *del = [NSButton buttonWithTitle:@"✕" target:gTextView action:@selector(btNoteDelete:)];
        del.bordered = NO;
        del.font = [NSFont systemFontOfSize:13 weight:NSFontWeightMedium];
        del.contentTintColor = btMacNoteColor(gMacNoteMuted);
        del.tag = 905;
        [box addSubview:del];

        if (gMacNoteNextable) {
            // The WHO line's count region is a CONTROL when the passage holds
            // more than one note (S10) — the iOS twin's arrangement: a
            // transparent button over the counts span, whose affordance is
            // the span itself drawn in the accent with a trailing chevron
            // (btMacLayoutNote).
            NSButton *nxt = [NSButton buttonWithTitle:@"" target:gTextView action:@selector(btNoteNext:)];
            nxt.bordered = NO;
            nxt.transparent = YES;
            nxt.tag = 906;
            [box addSubview:nxt];
        }
    }

    // A subview of the TEXT VIEW — the scroll view's document view — so this
    // frame is in content coordinates and the sticker scrolls with the passage
    // for free. Layout is the only thing that has to move it.
    [gTextView addSubview:box];
    gMacNoteView = box;
}

// btMacNoteStickerY is where the sticker's top goes, in the text view's content
// coordinates. ONE function, used by both the layout and the scroll target, so
// the two cannot disagree about where the note is.
//
// It does not simply trust the anchor paragraph's line-fragment origin, which is
// what the iOS twin does and what the first cut here did. The reporter layout
// sets a line height tighter than this serif face's natural leading, so the
// PRECEDING line's glyphs overflow the bottom of their fragment box — the card
// was placed correctly with respect to the fragments and still landed across the
// bottom half of the line above it. Asking the layout manager for the glyphs'
// actual bounding rect is the measurement that matches what a reader sees.
static CGFloat btMacNoteStickerY(NSLayoutManager *lm, NSTextContainer *tc,
                                 NSTextStorage *ts, NSRange para, NSRange g) {
    CGFloat inset = gTextView.textContainerInset.height;
    if (gMacNoteTopInset > 0) return 14 + btMacNoteTopGap(ts, para);   // the container-inset reservation

    // HANG THE CARD OFF THE PASSAGE, not off the band's top edge.
    //
    // The obvious placement — the anchor paragraph's line-fragment origin plus a
    // gap, which is what the iOS twin uses — put the card across the bottom half
    // of the line ABOVE it here, twice, for two different reasons I chased in
    // turn (descender overflow, then the fragment origin itself). Both were
    // guesses about how TextKit distributes paragraphSpacingBefore inside the
    // fragment rect, and the fragment is simply not a reliable way to ask.
    //
    // The USED rect of the paragraph's first glyph is: it is where the reader
    // sees the passage start. Hanging the sticker a fixed gap above that puts
    // its tail a fixed distance from the words it points at — which is the thing
    // that should be constant — and lets the leftover band space fall above the
    // card, where the previous line already is.
    // The USED rect, not the fragment rect and not boundingRectForGlyphRange —
    // both of those report the FRAGMENT, which is precisely the box the reserved
    // spacing lives inside, so asking either where the text starts returns where
    // the BAND starts and the card lands a band's height too high. The used rect
    // is the sub-box the glyphs occupy.
    // The queries below are only as good as the layout behind them, and the
    // install (or its reconcile) has just EDITED this paragraph's style —
    // asked too soon, the used rect answers with pre-edit geometry, the card
    // sticks a band too high, and nothing after corrects it (verification:
    // the bubble covering the previous paragraph's last line with a dead gap
    // under its tail, on the desktop). Force the paragraph's layout current
    // before asking anything.
    [lm ensureLayoutForCharacterRange:para];
    NSRect used = [lm lineFragmentUsedRectForGlyphAtIndex:g.location effectiveRange:NULL];
    NSRect frag = [lm lineFragmentRectForGlyphAtIndex:g.location effectiveRange:NULL];
    NSParagraphStyle *eff = [ts attribute:NSParagraphStyleAttributeName atIndex:para.location
                           effectiveRange:NULL];
    // Cross-check the used rect against the band's own arithmetic: TextKit
    // lays paragraphSpacingBefore INSIDE the first line's fragment (measured:
    // used.y == frag.y + spacingBefore, exactly), so the passage can never
    // start above frag.y + spacingBefore. A used rect claiming otherwise is
    // the stale answer above — take the honest one.
    CGFloat textTopRaw = used.origin.y;
    if (eff != nil && eff.paragraphSpacingBefore > 0) {
        CGFloat floorY = frag.origin.y + eff.paragraphSpacingBefore;
        if (textTopRaw < floorY) textTopRaw = floorY;
    }
    CGFloat textTop = textTopRaw + inset;
    CGFloat stickerH = btMacNoteHeightForWidth(tc.size.width - 2 * tc.lineFragmentPadding)
                     + (btMacNotePill() ? 0 : kMacNoteTail);
    if (getenv("BT_NOTE_GEOM")) {
        fprintf(stderr, "[geom] layout: used.y=%.1f frag.y=%.1f frag.h=%.1f inset=%.1f stickerH=%.1f "
                        "bandH=%.1f spacingBefore=%.1f y=%.1f\n",
            used.origin.y, frag.origin.y, frag.size.height, inset, stickerH,
            gMacNoteBandH, eff ? eff.paragraphSpacingBefore : -1,
            textTop - kMacNoteGapBelow - stickerH);
    }
    return textTop - kMacNoteGapBelow - stickerH;
}

// Put the sticker in the band the text reserved. Runs after every layout.
static void btMacLayoutNote(void) {
    if (gMacNoteView == nil || gTextView == nil || !btMacNotePresent()) return;
    NSLayoutManager *lm = gTextView.layoutManager;
    NSTextContainer *tc = gTextView.textContainer;
    NSTextStorage *ts = gTextView.textStorage;
    if (lm == nil || tc == nil || ts == nil) return;

    NSRange anchor = btMacNoteAnchorRange(ts, ts.length);
    NSRange para = [ts.string paragraphRangeForRange:anchor];
    if (para.location == NSNotFound || NSMaxRange(para) > ts.length) return;
    NSRange g = [lm glyphRangeForCharacterRange:para actualCharacterRange:NULL];
    if (g.length == 0) return;

    CGFloat pad = tc.lineFragmentPadding;
    CGFloat w = tc.size.width - 2 * pad;
    CGFloat h = btMacNoteHeightForWidth(w);
    CGFloat x = gTextView.textContainerInset.width + pad;

    // RECONCILE THE BAND. The height is measured from the container width, and
    // on the first render after a link opens that width is not yet final — the
    // band comes out taller than the bubble and leaves a gap. If what we
    // reserved no longer matches what we need, reserve again and let the layout
    // settle; the flag stops that becoming a loop.
    static BOOL reconciling = NO;
    CGFloat want = btMacNoteTopGap(ts, para) + h + (btMacNotePill() ? 0 : kMacNoteTail) + kMacNoteGapBelow;
    if (!reconciling && fabs(want - gMacNoteBandH) > 1.0) {
        reconciling = YES;
        btMacInstallNote();
        reconciling = NO;
    }

    CGFloat y = btMacNoteStickerY(lm, tc, ts, para, g);

    if (btMacNotePill()) {
        // The pill sizes to its label (it carries the count now), never
        // narrower than the old fixed chip and never wider than the column.
        NSString *title = gMacNoteWho ?: @"Note";
        CGFloat tw = ceil([title sizeWithAttributes:@{NSFontAttributeName: btMacNoteWhoFont()}].width);
        CGFloat cw = tw + 2 * kMacNotePillPadX;
        if (cw < kMacNotePillMinW) cw = kMacNotePillMinW;
        if (cw > w) cw = w;
        gMacNoteView.frame = NSMakeRect(x, y, cw, kMacNotePill);
        NSView *chip = [gMacNoteView viewWithTag:901];
        chip.frame = gMacNoteView.bounds;
        // Cursor rects are cached against the frames they were built from, so a
        // moved sticker keeps pointing-hand areas where it USED to be until this
        // is called. Every layout ends here.
        [gMacNoteView.window invalidateCursorRectsForView:gMacNoteView];
        return;
    }

    gMacNoteView.frame = NSMakeRect(x, y, w, h + kMacNoteTail);
    gMacNoteCard.frame = gMacNoteView.bounds;
    gMacNoteCard.path = btMacNoteBubblePath(w, h).CGPath;

    NSView *who  = [gMacNoteView viewWithTag:902];
    NSView *body = [gMacNoteView viewWithTag:903];
    NSView *hide = [gMacNoteView viewWithTag:904];
    NSView *del  = [gMacNoteView viewWithTag:905];
    NSButton *nxt = (NSButton *)[gMacNoteView viewWithTag:906];
    CGFloat whoW = w - 2 * kMacNotePad - 2 * kMacNoteBtn;
    // The who row's box starts at the card's own padding — no "- 2" shim, which is
    // what used to make the stated 12 + 13 + 4 rhythm describe a card whose real top
    // padding was 10 and whose real who→body gap was 6.
    who.frame  = NSMakeRect(kMacNotePad, kMacNotePad, whoW, kMacNoteWho);
    // The fit runs HERE, where the label's real width is known: if the who
    // line cannot fit, the sender half is tail-truncated and the counts
    // survive whole (btMacFitWho).
    NSString *fitted = btMacFitWho(gMacNoteWho ?: @"Note from Friend", whoW, btMacNoteWhoFont());
    NSRange counts = (gMacNoteNextable && nxt != nil) ? btMacWhoCountRange(fitted)
                                                      : NSMakeRange(NSNotFound, 0);
    if (counts.location == NSNotFound) {
        ((NSTextField *)who).stringValue = fitted;
        nxt.hidden = YES;
    } else {
        // The counts span is the next-tap's control, so it must LOOK
        // pressable: the accent colour plus a trailing chevron — the app's
        // own chrome, no new words (the iOS twin paints identically).
        NSString *shown = [fitted stringByReplacingCharactersInRange:NSMakeRange(NSMaxRange(counts), 0)
                                                          withString:@" ›"];
        NSRange lit = NSMakeRange(counts.location, counts.length + 2);
        NSDictionary *attrs = @{NSFontAttributeName: btMacNoteWhoFont()};
        NSMutableAttributedString *a = [[NSMutableAttributedString alloc]
            initWithString:shown
                attributes:@{NSFontAttributeName: btMacNoteWhoFont(),
                             NSForegroundColorAttributeName: btMacNoteColor(gMacNoteMuted)}];
        [a addAttribute:NSForegroundColorAttributeName value:btMacNoteColor(gMacNoteAccent) range:lit];
        ((NSTextField *)who).attributedStringValue = a;
        CGFloat pre = ceil([[shown substringToIndex:lit.location] sizeWithAttributes:attrs].width);
        CGFloat lw  = ceil([[shown substringWithRange:lit] sizeWithAttributes:attrs].width);
        CGFloat bx = kMacNotePad + pre - 6;
        if (bx < kMacNotePad - 6) bx = kMacNotePad - 6;
        CGFloat bw = lw + 16;
        if (bx + bw > kMacNotePad + whoW + 6) bw = kMacNotePad + whoW + 6 - bx;
        nxt.hidden = NO;
        nxt.frame = NSMakeRect(bx, 0, bw, kMacNotePad + kMacNoteWho + 2);
    }
    body.frame = NSMakeRect(kMacNotePad, kMacNotePad + kMacNoteWho + kMacNoteWhoGap,
                            w - 2 * kMacNotePad,
                            h - kMacNotePad - kMacNoteWho - kMacNoteWhoGap - kMacNotePad);
    hide.frame = NSMakeRect(w - 2 * kMacNoteBtn - 4, 3, kMacNoteBtn, kMacNoteBtn);
    del.frame  = NSMakeRect(w - kMacNoteBtn - 4, 3, kMacNoteBtn, kMacNoteBtn);
    [gMacNoteView.window invalidateCursorRectsForView:gMacNoteView];
}

// btMacNoteTopY is where the TOP of the sticker sits in the text view's content
// coordinates, or -1 when there is no note to worry about.
//
// Deliberately the same arithmetic btMacLayoutNote uses to place it: the scroll
// target and the sticker must not be able to disagree about where the note is.
static CGFloat btMacNoteTopY(void) {
    if (!btMacNotePresent() || gTextView == nil) return -1;
    NSLayoutManager *lm = gTextView.layoutManager;
    NSTextContainer *tc = gTextView.textContainer;
    NSTextStorage *ts = gTextView.textStorage;
    if (lm == nil || tc == nil || ts == nil || ts.length == 0) return -1;
    NSRange anchor = btMacNoteAnchorRange(ts, ts.length);
    NSRange para = [ts.string paragraphRangeForRange:anchor];
    if (para.location == NSNotFound || NSMaxRange(para) > ts.length) return -1;
    NSRange g = [lm glyphRangeForCharacterRange:para actualCharacterRange:NULL];
    if (g.length == 0) return -1;
    return btMacNoteStickerY(lm, tc, ts, para, g);
}

// btMacRefreshNote is the one entry point: reserve the band, build the sticker,
// place it. Every caller that changes the text, the note or the geometry ends
// here, so the three can never disagree about where the note is.
static void btMacRefreshNote(void) {
    if (gTextView == nil) return;
    btMacInstallNote();
    btMacEnsureNoteView();
    [gTextView.layoutManager ensureLayoutForTextContainer:gTextView.textContainer];
    btMacLayoutNote();
}

// The note the pane should draw, pushed from Go on every chapter render — so a
// light/dark flip restyles it and a navigation replaces it. Since S9 the push
// carries WHO beside the text (the app's chrome, composed in Go by
// appleStickerPush), and the refresh runs only when the pushed tuple changed
// — the iOS twin's compare, mirrored — so a presentation flip (suppression,
// minimize, a count moving) repaints the sticker alone while an unchanged
// push costs nothing. Colours alone never change without a body change (the
// theme variant is folded there), so they do not join the compare; the apply
// path's own btMacRefreshNote (bibleTextMacApplyHTML) restyles after every
// import exactly as before.
void bibleTextMacSetNote(const char *text, const char *who, int minimized, int nextable, int anchorVerse,
                         double bgR, double bgG, double bgB,
                         double fgR, double fgG, double fgB,
                         double muR, double muG, double muB,
                         double acR, double acG, double acB,
                         double boR, double boG, double boB) {
    NSString *t = (text == NULL || *text == 0) ? nil : [NSString stringWithUTF8String:text];
    NSString *w = (who == NULL || *who == 0) ? nil : [NSString stringWithUTF8String:who];
    dispatch_async(dispatch_get_main_queue(), ^{
        BOOL changed = !btMacSameStr(t, gMacNoteText) || !btMacSameStr(w, gMacNoteWho) ||
                       gMacNoteMinimized != (minimized ? YES : NO) ||
                       gMacNoteNextable != (nextable ? YES : NO) ||
                       gMacNoteAnchorVerse != anchorVerse;
        gMacNoteText = t;
        gMacNoteWho = w;
        gMacNoteMinimized = minimized ? YES : NO;
        gMacNoteNextable = nextable ? YES : NO;
        gMacNoteAnchorVerse = anchorVerse;
        gMacNoteBg[0]=bgR; gMacNoteBg[1]=bgG; gMacNoteBg[2]=bgB;
        gMacNoteFg[0]=fgR; gMacNoteFg[1]=fgG; gMacNoteFg[2]=fgB;
        gMacNoteMuted[0]=muR; gMacNoteMuted[1]=muG; gMacNoteMuted[2]=muB;
        gMacNoteAccent[0]=acR; gMacNoteAccent[1]=acG; gMacNoteAccent[2]=acB;
        gMacNoteBorder[0]=boR; gMacNoteBorder[1]=boG; gMacNoteBorder[2]=boB;
        if (changed) btMacRefreshNote();
    });
}

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
        btMacApplyInsets(gScroll.contentSize.width); // recentre the reporter column at the new width
        btMacLayoutFollowBtn(); // the pill floats relative to the pane's bottom edge
        // The sticker's width and its reserved band both come from the container
        // width, so a resize has to redo both — not just move the view. Without
        // this a window drag left the bubble at its old width with the band still
        // sized for it, which is the gap the reconcile in btMacLayoutNote exists
        // to close.
        if (changed) btMacRefreshNote();
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
	"math"
	"sync"
	"time"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// readingScrollArea (macOS) returns a transparent host that reserves the
// reading rectangle; the native NSTextView paints the verses on top. A
// parchment rectangle sits behind it (the text view's background is clear).
func readingScrollArea(state *AppState, verses []Verse, pal palette) fyne.CanvasObject {
	// The Windows/Linux surface, when something asks for it. Default is the
	// build-tag constant — FALSE here — so the shipping macOS path below is
	// byte-identical and no release build can reach this branch.
	//
	// It exists because the two reading surfaces are mutually exclusive per
	// platform, so on any one machine only one of them can be looked at. The
	// verses on macOS are drawn by a native NSTextView above the canvas: there
	// is no Fyne text in the tree, so "can the reader SEE the verses" cannot be
	// asked here at all, and a view test on this machine could only ever check

	// separately — a dev mode where macOS follows the Windows/Linux paths.
	if useStyledPane() {
		return styledReadingScrollArea(state, verses, pal)
	}

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
		// The sheet the reader was inside has left the canvas: run the window
		// rebuild a background data swap deferred to spare it (no-op otherwise,
		// and non-recursive — rebuildWindow downs the flag before re-running
		// this closure).
		consumeDeferredFullRebuild(state)
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
	// When the styled pane is the reading surface (the mimic dev mode, or a test
	// pinning the seam), there is no native overlay to show or hide — and the
	// show path MUST not run: bibleTextMacTVShow → bibleTextMacEnsureTV would
	// CREATE an empty native scroll view floating over the canvas. Matches
	// reading_fyne.go's no-op. Release darwin: useStyledPane() is the false
	// platform constant, so this branch is dead and the path is byte-identical.
	if useStyledPane() {
		return
	}
	if visible {
		C.bibleTextMacTVShow()
	} else {
		C.bibleTextMacTVHide()
	}
}

func hideNativeReadingOverlayMac() { C.bibleTextMacTVHide() }

// nativeShareText / nativeShareImage present the macOS share sheet for the
// selection-menu Share actions (see share.go). Under the platform-mimic dev
// mode they take the Windows/Linux fallback bodies instead (share_fallback.go:
// clipboard + notice popup, save-to-Downloads + file-manager reveal) so the
// real desktop-other share UX can be eyeballed here. devMimicTarget() is a
// constant "" in release builds (dev_mimic_off.go) — the branch is dead code.
func nativeShareText(s string) {
	if devMimicTarget() != "" {
		fallbackShareText(s)
		return
	}
	c := C.CString(s)
	defer C.free(unsafe.Pointer(c))
	C.bibleTextShareText(c)
}

func nativeShareImage(path string) {
	if devMimicTarget() != "" {
		fallbackShareImage(path)
		return
	}
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
// from lastPushedBodyFP (which also folds in theme/red-letter/data identity), so
// a genuine chapter change (pin to top) is distinguishable from a same-chapter
// re-render (preserve the reader's scroll). Mirrors the iOS and Android twins;
// the combined lastPushedChapterFP is Android's alone now.
var lastPushedBookChapter string

func newMacReadingHost(state *AppState, verses []Verse) *macReadingHost {
	// Keep the contextual menu's note verb in step with the setting.
	on := C.int(0)
	if notesFeatureOn(state) {
		on = 1
	}
	C.btMacSetNotesEnabled(on)
	// Before the SetHTML below, so the sticker's globals are already current when
	// the apply block installs the band. Both hop through the main queue, which
	// is FIFO, so this ordering holds.
	pushNoteToPane(state)

	h := &macReadingHost{state: state}
	h.ExtendBaseWidget(h)
	macCurrentHost = h
	// Keep the native reporter column in sync with the text-size setting: the
	// measure is em-based (27.5 × body px), so Large/XL widen the column and
	// keep its character count at the reporter's ~59 (the iOS twin does the
	// same in pushChapterHTML).
	if reporterLayoutActive() {
		bodyPx := math.Round(21 * readingTextScale())
		C.bibleTextMacSetReadingMeasure(C.double(reporterMeasureEm * bodyPx))
	} else {
		C.bibleTextMacSetReadingMeasure(0)
	}
	syncNativeAIMenu(state) // the menu gate must match the setting before any selection
	// TWO fingerprints, because the two changes cost different things to apply —
	// the body only by a rebuild + re-import, the wash by one attribute over a
	// known range of the string already on screen (chapterBodyFingerprint,
	// reading.go). The iOS twin (pushChapterHTML) splits identically.
	body := chapterBodyFingerprint(state)
	tintFP := chapterTint(state).fingerprint()
	bc := fmt.Sprintf("%s|%d", state.CurrentBook, state.CurrentChapter)
	// Same-chapter RE-render — the fingerprint changed but the book+chapter did
	// not (a light/dark flip, red-letter toggle, or background data swap). The
	// SetHTML below replaces the text view's content, which snaps the scroll to
	// the TOP; capture the reader's live position first and arm it as a one-shot
	// restore, exactly as the iOS twin does (pushChapterHTML). Without this a
	// system dark→light switch yanked a mid-chapter desktop reader to the top —
	// and the next flush then persisted that top-of-chapter position.
	//
	// The BODY fingerprint, not the whole render's: a wash-only change no longer
	// replaces the text view's content, so there is no scroll snap to pre-empt.
	if state.restore == nil && bc == lastPushedBookChapter && body != lastPushedBodyFP {
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
	// already holds this chapter's TEXT (mirrors the iOS gate in
	// pushChapterHTML); a pending scroll restore forces the push. SetHTML consumes
	// the C string synchronously, so freeing right after the call is safe.
	//
	// THE WASH IS NOT A REASON TO REBUILD — a wash-only change is a live range
	// mutation on the attributed string already on screen. See pushChapterHTML.
	// Nor is forceReposition: "place the view" is a scroll
	// (bibleTextMacScrollToHighlight), not a re-import, and treating it as one made
	// the fast path unreachable for every mark that ARRIVES.
	if state.restore != nil || body != lastPushedBodyFP {
		// Read BEFORE lastPushedBookChapter moves — see the iOS twin: a
		// same-chapter re-render may carry a live narration wash across its own
		// re-import, a chapter change must not.
		sameChapter := C.int(0)
		if bc == lastPushedBookChapter {
			sameChapter = 1
		}
		state.forceReposition = false
		lastPushedBodyFP = body
		lastPushedTintFP = tintFP
		lastPushedBookChapter = bc
		// The model FIRST and unpainted: the HTML below carries this very wash.
		setNativeTint(state, verses)
		// Announce the push: the same-chapter answer above, plus the generation
		// that tells a landed chapter from a failed import (gBodyGenPending).
		C.bibleTextMacBeginChapterPush(sameChapter)
		html := buildChapterHTML(state, verses)
		c := C.CString(html)
		C.bibleTextMacTVSetHTML(c)
		C.free(unsafe.Pointer(c))
	} else {
		if tintFP != lastPushedTintFP {
			lastPushedTintFP = tintFP
			applyNativeTint(state, verses)
		}
		if state.forceReposition {
			state.forceReposition = false
			C.bibleTextMacScrollToHighlight()
		}
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
//
// The styled pane delegates first — the same two-line delegation
// reading_scroll_fyne.go carries. Unconditionally safe in release:
// styledScroll/styledPane are only ever assigned inside styledReadingScrollArea
// (reading_styled_area.go), which release macOS never builds, so
// styledAnchorActive() is constant-false there and the C machinery is reached
// exactly as before. Under the mimic dev mode it is what keeps position
// persistence and history restore alive — without it flushReadingState would
// read the dead NSTextView and silently save nothing.
func captureReadingAnchor() (verse int, delta, frac float64, ok bool) {
	if styledAnchorActive() {
		return captureStyledAnchor()
	}
	a := C.bibleTextMacCaptureAnchor()
	return int(a.verse), float64(a.delta), float64(a.frac), a.ok != 0
}

func armReadingRestore(verse int, delta, frac float64) {
	if styledAnchorActive() {
		armStyledRestore(verse, delta, frac)
		return
	}
	C.bibleTextMacArmRestore(C.int(verse), C.double(delta), C.double(frac))
}

// pushNoteToPane hands the native sticker its text, its WHO line, its
// presentation and the live palette — the macOS twin of the iOS function of
// the same name, called on every chapter render so a light/dark flip restyles
// it and a navigation replaces it.
//
// SINCE S9 THE PUSH IS THE FULL TUPLE (appleStickerPush, notes_plan.go),
// exactly as the iOS twin: the bubble's body is the sender's words ALONE, and
// everything the app says — the byline, "· 1 of 3 on this passage", "· 2 not
// shown here", the pill's count, the unplaced-only sentence — rides in the
// WHO parameter, the sticker's own chrome. This closes S8's recorded identity
// gap (the count used to ride inside the sender's bubble in the sender's
// style) with the one tuple change S9 exists for. The pushed pill
// flag folds the plan's derived suppression, so a foreign mark stands the
// sticker down to the pill and releases it — rendered by the native side's
// own compare-and-refresh (bibleTextMacSetNote), never by a chapter
// re-import.
func pushNoteToPane(state *AppState) {
	pal := state.pal()
	text, who, pill, next := appleStickerPush(state, buildChapterPlan(state, appPrefs(), state.Bible))
	cText := C.CString(text)
	defer C.free(unsafe.Pointer(cText))
	cWho := C.CString(who)
	defer C.free(unsafe.Pointer(cWho))
	min := C.int(0)
	if pill {
		min = 1
	}
	nx := C.int(0)
	if next {
		nx = 1
	}
	f := func(c color.NRGBA) (C.double, C.double, C.double) {
		return C.double(float64(c.R) / 255), C.double(float64(c.G) / 255), C.double(float64(c.B) / 255)
	}
	bgR, bgG, bgB := f(pal.SurfaceAlt)
	fgR, fgG, fgB := f(pal.Text)
	muR, muG, muB := f(pal.TextMuted)
	acR, acG, acB := f(pal.Accent)
	boR, boG, boB := f(pal.Border)
	// The note's OWN verse, not the highlight's — minimizing clears the
	// highlight, and a sticker without an anchor parks at the top of the
	// chapter (an unplaced-only chapter pushes verse 0 on purpose: the top is
	// the only honest place for notes with no verses here).
	C.bibleTextMacSetNote(cText, cWho, min, nx, C.int(state.NoteVerseLo),
		bgR, bgG, bgB, fgR, fgG, fgB, muR, muG, muB, acR, acG, acB, boR, boG, boB)
}

// setNativeTint / applyNativeTint hand the chapter's wash model
// (reading_tint_apple.go) to the NSTextView — the macOS half of the pair, kept
// name-for-name with the iOS one so the audio controller and the push gate call
// the same thing on both panes.
//
//   - setNativeTint records the model and paints nothing, because the HTML about
//     to be imported already carries the wash.
//   - applyNativeTint paints it onto the attributed string already on screen: no
//     buildChapterHTML, no re-import, no re-assertion of the scroll position.
//
// Passing &runs[0] is within the cgo pointer rules — the C side copies into its
// own fixed table before returning and never retains the Go memory.
func setNativeTint(state *AppState, verses []Verse) { pushNativeTint(state, verses, 0) }

func applyNativeTint(state *AppState, verses []Verse) { pushNativeTint(state, verses, 1) }

func pushNativeTint(state *AppState, verses []Verse, repaint C.int) {
	runs := nativeTintRuns(state, verses)
	if len(runs) == 0 {
		C.bibleTextMacSetTintRuns(nil, 0, repaint)
		return
	}
	c := make([]C.BTTintRun, len(runs))
	for i, r := range runs {
		c[i] = C.BTTintRun{
			lo: C.int(r.Lo), hi: C.int(r.Hi),
			r: C.double(float64(r.Wash.R) / 255), g: C.double(float64(r.Wash.G) / 255),
			b: C.double(float64(r.Wash.B) / 255), a: C.double(float64(r.Wash.A) / 255),
		}
	}
	C.bibleTextMacSetTintRuns(&c[0], C.int(len(c)), repaint)
}

// readAlongHighlight tints the verse being narrated (0 clears) and follow-scrolls it
// into view; readAlongClear removes the tint. Both run on the macOS main thread — the
// audio time-observer's main queue, or the Fyne UI goroutine (which is that thread).
//
// When the styled pane is the reading surface (the mimic dev mode) the wash is
// forwarded to the styled helpers on the UI goroutine, exactly as
// readalong_other.go does on Windows/Linux — the C calls below would paint an
// invisible native view while the styled pane showed nothing. Release darwin:
// useStyledPane() is the false platform constant; the branch is dead.
func readAlongHighlight(verse int, follow bool) {
	if useStyledPane() {
		fyne.Do(func() { styledReadAlongApply(verse, follow) })
		return
	}
	f := C.int(0)
	if follow {
		f = 1
	}
	C.bibleTextMacHighlightVerse(C.int(verse), f)
}
func readAlongClear() {
	if useStyledPane() {
		fyne.Do(styledReadAlongClearTint)
		return
	}
	C.bibleTextMacReadAlongClear()
}

// readAlongFollowButton shows/hides the native floating "Follow narration" pill
// over the reading pane (audio_controller drives it around follow suspension).
func readAlongFollowButton(show bool) {
	if useStyledPane() {
		fyne.Do(func() { styledReadAlongSetPill(show) })
		return
	}
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
//
// The trailing re-assert is ONE shared timer, not one per call: a layout pass
// is a burst of Resize+Move, and only the LAST geometry is worth re-asserting
// — every earlier tick would push a frame already superseded. The single
// timer also matters to the race detector: under the TEST driver, fyne.Do
// from a goroutine runs inline on that goroutine, so a burst's timers all
// firing ~60ms later used to walk the object tree concurrently
// (AbsolutePositionForObject → Fyne's renderer cache), a race the real
// drivers never see because they serialize fyne.Do onto the main thread.
// macFrameMu covers the timer handle AND the walk itself, so a re-assert
// mid-fire cannot overlap the immediate push of the next burst either.
var (
	macFrameMu    sync.Mutex
	macFrameTimer *time.Timer
)

func (h *macReadingHost) pushFrame() {
	setMacFrameFromObject(h)
	if _, real := fyne.CurrentApp().Driver().(desktop.Driver); !real {
		// The TEST driver: there is no visible native pane to re-assert for,
		// and a timer's fyne.Do runs on the timer's own goroutine there — an
		// off-main tree walk the harness's walks can race with. The immediate
		// push above (same goroutine as the test) is the whole job.
		return
	}
	macFrameMu.Lock()
	defer macFrameMu.Unlock()
	if macFrameTimer != nil {
		macFrameTimer.Reset(60 * time.Millisecond)
		return
	}
	macFrameTimer = time.AfterFunc(60*time.Millisecond, func() {
		fyne.Do(func() {
			// The CURRENT host, not a captured one: the timer is shared, and
			// by fire time a rebuild may have swapped hosts under it.
			if h := macCurrentHost; h != nil {
				setMacFrameFromObject(h)
			}
		})
	})
}

func setMacFrameFromObject(h *macReadingHost) {
	macFrameMu.Lock()
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(h)
	macFrameMu.Unlock()
	sz := h.Size()
	if sz.Width <= 0 || sz.Height <= 0 {
		return
	}
	C.bibleTextMacTVSetFrame(
		C.double(pos.X), C.double(pos.Y),
		C.double(sz.Width), C.double(sz.Height),
	)
}
