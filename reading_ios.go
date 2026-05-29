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
        NSLog(@"holybible: HTML set (%lu bytes input → %lu attr chars)",
              (unsigned long)len, (unsigned long)as.length);
        gReadingTV.attributedText = as;
        gReadingTV.contentOffset = CGPointZero;
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
        NSLog(@"holybible: setFrame raw(%.1f,%.1f) safe.top=%.1f → (%.1f,%.1f) %.1fx%.1f hidden=%d",
              x, y, safe.top, x + safe.left, y + safe.top, w, h, gReadingTV.hidden);
        gReadingTV.frame = r;
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
	"strings"
	"time"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
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

	top := container.NewVBox()
	if bar := buildHistoryBar(state); bar != nil {
		top.Add(bar)
	}
	if state.CanReturnToSearchResults {
		top.Add(backToResultsBar(state))
	}
	top.Add(chapterHeader(state, chapterNumbers))

	// Draw a "paper" rectangle behind the UITextView so the reading area still
	// has the parchment look; the UITextView's backgroundColor is clear, so
	// this shows through.
	paper := canvas.NewRectangle(pal.Surface)
	paper.StrokeColor = pal.Border
	paper.StrokeWidth = 1
	paper.CornerRadius = 8

	body := container.NewBorder(top, nil, nil, nil, container.NewStack(paper, host))
	return container.NewPadded(body)
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
func (h *nativeReadingHost) pushFrame() {
	setFrameFromObject(h)
	// Re-read after layout settles. fyne.Do runs on the UI goroutine.
	time.AfterFunc(50*time.Millisecond, func() {
		fyne.Do(func() { setFrameFromObject(h) })
	})
}

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

// buildChapterHTML emits an HTML document that NSAttributedString's HTML
// importer turns into a richly-styled attributed string. We embed all colors
// inline so light/dark mode tracks the active palette without a re-parse.
func buildChapterHTML(state *AppState, verses []Verse) string {
	pal := state.pal()
	textHex := nrgbaToHex(pal.Text)
	numHex := nrgbaToHex(pal.VerseNumber)
	highlightTextHex := nrgbaToHex(pal.HighlightText)
	highlightBgHex := nrgbaToHex(pal.Highlight)

	var b strings.Builder
	b.WriteString("<html><head><style>")
	fmt.Fprintf(&b, `body { font-family: Georgia, "Times New Roman", serif; `+
		`font-size: 18px; color: %s; line-height: 1.55; margin: 0; padding: 0; -webkit-text-size-adjust: 100%%; }`,
		textHex)
	fmt.Fprintf(&b, `p { margin: 0 0 14px 0; }`)
	fmt.Fprintf(&b, `sup.v { color: %s; font-weight: bold; font-size: 0.62em; }`, numHex)
	fmt.Fprintf(&b, `.hl { color: %s; background-color: %s; font-weight: bold; }`,
		highlightTextHex, highlightBgHex)
	b.WriteString("</style></head><body>")

	for _, para := range groupVersesIntoParagraphs(verses) {
		b.WriteString("<p>")
		for i, v := range para {
			if i > 0 {
				b.WriteByte(' ')
			}
			fmt.Fprintf(&b, `<sup class="v">%d</sup>&nbsp;`, v.Verse)
			body := htmlEscape(strings.TrimSpace(strings.ReplaceAll(v.Text, "\n", " ")))
			if isVerseHighlighted(state, v) {
				fmt.Fprintf(&b, `<span class="hl">%s</span>`, body)
			} else {
				b.WriteString(body)
			}
		}
		b.WriteString("</p>")
	}
	b.WriteString("</body></html>")
	return b.String()
}

// nrgbaToHex formats an image/color.NRGBA as a #RRGGBB string suitable for CSS.
func nrgbaToHex(c color.NRGBA) string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

// htmlEscape inlines just the four characters that would break out of a
// content span; we don't expect <, >, & in verse text but be safe.
func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}
