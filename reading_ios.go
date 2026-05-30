//go:build ios

package holybible

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

// One persistent UITextView attached to the app's main window. We never
// destroy it during the app lifetime — easier to manage than re-attaching,
// and the iOS selection state stays alive across chapter changes.
static UITextView *gReadingTV = nil;

// Look up the foreground UIWindow that Fyne renders into. iOS 13+ uses scenes;
// pre-13 we fall back to the deprecated keyWindow. Fyne's mobile driver creates
// exactly one window, so the first one we find is the right one.
static UIWindow *holyBibleFindWindow(void) {
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
static void holyBibleEnsureTV(void) {
    dispatch_block_t block = ^{
        UIWindow *win = holyBibleFindWindow();
        if (win == nil) {
            NSLog(@"holybible: ensureTV — no UIWindow yet");
            return;
        }
        if (gReadingTV == nil) {
            UITextView *tv = [[UITextView alloc] init];
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
            NSLog(@"holybible: ensureTV — created UITextView, attaching to window %@", win);
        }
        if (gReadingTV.superview != win) {
            [gReadingTV removeFromSuperview];
            [win addSubview:gReadingTV];
        }
        [win bringSubviewToFront:gReadingTV];
    };
    if ([NSThread isMainThread]) {
        block();
    } else {
        dispatch_sync(dispatch_get_main_queue(), block);
    }
}

void holyBibleTVSetHTML(const char *html) {
    if (html == NULL) return;
    NSString *s = [NSString stringWithUTF8String:html];
    NSData *data = [s dataUsingEncoding:NSUTF8StringEncoding];
    NSUInteger len = data.length;
    dispatch_async(dispatch_get_main_queue(), ^{
        holyBibleEnsureTV();
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
            NSLog(@"holybible: HTML parse failed (input %lu bytes): %@", (unsigned long)len, err);
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
        gReadingTV.attributedText = as;
        // Aggressive scroll-to-top: UITextView may relayout after attributedText
        // is set, the Fyne side pushes a new frame just after, and either can
        // shift the offset. Reset on this tick, the next runloop tick, and a
        // ~200ms tick to outlast the slowest re-layout we've seen in practice.
        [gReadingTV layoutIfNeeded];
        gReadingTV.contentOffset = CGPointZero;
        dispatch_async(dispatch_get_main_queue(), ^{
            if (gReadingTV != nil) {
                gReadingTV.contentOffset = CGPointMake(0, -gReadingTV.adjustedContentInset.top);
            }
        });
        dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(0.2 * NSEC_PER_SEC)),
                       dispatch_get_main_queue(), ^{
            if (gReadingTV != nil) {
                gReadingTV.contentOffset = CGPointMake(0, -gReadingTV.adjustedContentInset.top);
            }
        });
    });
}

// holyBibleTVScrollToFraction scrolls the text view so the given normalised
// position (0.0 = top, 1.0 = bottom) is at the visible top. Used to jump to a
// highlighted verse from search results.
void holyBibleTVScrollToFraction(float fraction) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (gReadingTV == nil) return;
        CGFloat contentH = gReadingTV.contentSize.height;
        CGFloat viewportH = gReadingTV.bounds.size.height;
        if (contentH <= viewportH) return;
        CGFloat maxY = contentH - viewportH;
        CGFloat y = fraction * contentH;
        if (y > maxY) y = maxY;
        if (y < 0)    y = 0;
        gReadingTV.contentOffset = CGPointMake(0, y);
    });
}

void holyBibleTVSetFrame(float x, float y, float w, float h) {
    dispatch_async(dispatch_get_main_queue(), ^{
        holyBibleEnsureTV();
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
        // carry over and land in the middle of the new chapter. Pin to the
        // top on every frame change for predictability — we use both APIs:
        // contentOffset sets the raw scroll; scrollRangeToVisible asks UIKit
        // to make the first character of the text visible, which it honours
        // even when HTML→NSAttributedString conversion has inserted phantom
        // paragraph-spacing before the first <p>.
        if (changed) {
            gReadingTV.contentOffset = CGPointZero;
            if (gReadingTV.attributedText.length > 0) {
                [gReadingTV scrollRangeToVisible:NSMakeRange(0, 1)];
            }
            dispatch_async(dispatch_get_main_queue(), ^{
                if (gReadingTV != nil) {
                    gReadingTV.contentOffset = CGPointZero;
                    if (gReadingTV.attributedText.length > 0) {
                        [gReadingTV scrollRangeToVisible:NSMakeRange(0, 1)];
                    }
                }
            });
        }
    });
}

void holyBibleTVShow(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        holyBibleEnsureTV();
        if (gReadingTV == nil) return;
        gReadingTV.hidden = NO;
        [gReadingTV.superview bringSubviewToFront:gReadingTV];
    });
}

void holyBibleTVHide(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (gReadingTV == nil) return;
        gReadingTV.hidden = YES;
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
	state.hideReadingOverlay = hideNativeReadingOverlay
	state.showReadingOverlay = showNativeReadingOverlay

	chapterNumbers := state.Bible.GetChapterNumbersForBook(state.CurrentBook)
	normalizeCurrentChapter(state, chapterNumbers)
	verses := state.Bible.GetChapter(state.CurrentBook, state.CurrentChapter)

	host := newNativeReadingHost(state, verses)

	paper := canvas.NewRectangle(pal.Surface)
	paper.StrokeColor = pal.Border
	paper.StrokeWidth = 1
	paper.CornerRadius = 8

	// Full-screen reading: no chrome at all except a small exit affordance.
	// Tabs and the top "Holy Bible" header are skipped in ui_mobile.go for this
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

	// Heading reflects the chapter: "John 1".
	title := canvas.NewText(fmt.Sprintf("%s %d", state.CurrentBook, state.CurrentChapter), pal.Text)
	title.TextSize = headingTextSize
	title.TextStyle = fyne.TextStyle{Bold: true}

	// Small copy icon tucked right after the book name — closer to the text
	// it copies, and visually lighter than the chapter-nav arrows.
	copyBtn := newIconTapButton(state, theme.ContentCopyIcon(), 15, 30, func() {
		copyChapter(state)
	})
	titleRow := container.NewHBox(title, hgap(4), copyBtn)

	var chapterLine fyne.CanvasObject
	if total > 1 {
		chapterLine = newChapterPickerAnchor(state,
			fmt.Sprintf("Chapter %d of %d  ▾", state.CurrentChapter, total),
			pal.TextMuted, subheadingTextSize)
	} else {
		lbl := canvas.NewText(fmt.Sprintf("Chapter %d", state.CurrentChapter), pal.TextMuted)
		lbl.TextSize = subheadingTextSize
		chapterLine = container.NewHBox(lbl)
	}

	idx := indexOf(chapterNumbers, state.CurrentChapter)

	// Prev/next as compact icon buttons sitting next to the chapter line, so
	// they're close to the book + chapter text rather than floating far right.
	prev := newIconTapButton(state, theme.NavigateBackIcon(), 18, 22, func() {
		if moveChapter(state, -1) {
			state.refresh()
		}
	})
	prev.disabled = idx <= 0

	next := newIconTapButton(state, theme.NavigateNextIcon(), 18, 22, func() {
		if moveChapter(state, 1) {
			state.refresh()
		}
	})
	next.disabled = idx < 0 || idx >= total-1

	chapterRow := container.NewHBox(
		container.NewVBox(layout.NewSpacer(), chapterLine, layout.NewSpacer()),
		hgap(10),
		prev, next,
	)

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
			if currentHost != nil {
				setFrameFromObject(currentHost)
			}
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
	C.holyBibleTVSetFrame(
		C.float(pos.X), C.float(pos.Y),
		C.float(sz.Width), C.float(sz.Height),
	)
}

// Show / Hide are hooked into the tab-switching logic from ui_mobile.go.
func (h *nativeReadingHost) Show() {
	h.BaseWidget.Show()
	C.holyBibleTVShow()
}

func (h *nativeReadingHost) Hide() {
	h.BaseWidget.Hide()
	C.holyBibleTVHide()
}

// showNativeReadingOverlay / hideNativeReadingOverlay are package-level so
// ui_mobile.go's tab-change handler can drive visibility without holding a
// reference to the host widget (which would force a circular dependency for
// every tab change).
func showNativeReadingOverlay() { C.holyBibleTVShow() }
func hideNativeReadingOverlay() { C.holyBibleTVHide() }

// pushChapterHTML builds the chapter as HTML (so NSAttributedString gets nice
// inline styling — superscript verse numbers, accent color, serif font) and
// sends it across the CGO boundary.
func pushChapterHTML(state *AppState, verses []Verse) {
	html := buildChapterHTML(state, verses)
	c := C.CString(html)
	defer C.free(unsafe.Pointer(c))
	C.holyBibleTVSetHTML(c)
}

// buildChapterHTML, nrgbaToHex and htmlEscape moved to reading.go so the macOS
// NSTextView overlay shares the exact same chapter HTML as the iOS UITextView.
