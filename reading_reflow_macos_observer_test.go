//go:build darwin && !ios

package bibletext

import (
	"strings"
	"testing"
)

func macReadingObserverProgram(t *testing.T) string {
	t.Helper()
	native := readNativeSource(t, "reading_macos.go")
	section := func(begin, end string) string {
		start := strings.Index(native, begin+"\n")
		if start < 0 {
			t.Fatal("native geometry section is missing: " + begin)
		}
		length := strings.Index(native[start:], end)
		if length < 0 {
			t.Fatal("native geometry section has no end: " + begin)
		}
		return native[start : start+length]
	}
	var source strings.Builder
	source.WriteString(`#import <AppKit/AppKit.h>
#include <math.h>
#include <string.h>
static NSScrollView *gScroll;
static NSTextView *gTextView;
`)
	source.WriteString(section("// BEGIN MAC GEOMETRY SCROLL OWNERSHIP", "// END MAC GEOMETRY SCROLL OWNERSHIP"))
	source.WriteString("\n@interface HBReadingTextView : NSTextView\n@end\n@implementation HBReadingTextView\n")
	source.WriteString(section("// BEGIN MAC TEXT GEOMETRY SCROLL OWNERSHIP", "// END MAC TEXT GEOMETRY SCROLL OWNERSHIP"))
	source.WriteString("\n@end\n")
	source.WriteString(section("// BEGIN MAC TEXT REFLOW ANCHOR", "// END MAC TEXT REFLOW ANCHOR"))
	source.WriteString(`
static BOOL gMacHasRestore=NO;
static int gMacNoteArrival=0, gMacNoteArrivalVerse=0;
enum { kMacArriveNothing=0 };
static unsigned gMacScrollSettleGen=0;
static BOOL gReadAlongActive=NO, gReadAlongOwnScroll=NO, gReadAlongUserLatch=NO;
static int readAlongUserCalls=0, settledCalls=0;
static void bibleTextReadingScrolled(void) { settledCalls++; }
static void bibleTextReadAlongUserScrolled(void) { readAlongUserCalls++; }
static int gBodyGenPending=1;
static BOOL gMacPreserveSelectionForPush=NO;
static NSString *gMacSelectionTargetBook=nil;
static int gMacSelectionTargetChapter=0;
`)
	for _, signature := range []string{
		"static void btMacUserScrolled(void) {",
		"static BOOL btMacReadingOriginMoved(NSClipView *clip, NSRect *lastBounds) {",
		"void bibleTextMacPreserveSelectionForPush(int gen, int preserve, const char *book, int chapter) {",
	} {
		source.WriteString(nativeFunctionSource(t, "reading_macos.go", signature))
		source.WriteString("\n}\n")
	}
	source.WriteString(`
static void Layout(void) {
 [gTextView.layoutManager ensureLayoutForTextContainer:gTextView.textContainer];
}
static CGFloat Origin(void) { return gScroll.contentView.bounds.origin.y-gTextView.frame.origin.y; }
static void OwnPut(void) { gMacOwnScroll++; btMacPutTextReflow(); gMacOwnScroll--; }
static void Move(CGFloat y, BOOL owned) {
 if(owned) gMacOwnScroll++;
 [gScroll.contentView scrollToPoint:NSMakePoint(0,y+gTextView.frame.origin.y)];
 [gScroll reflectScrolledClipView:gScroll.contentView];
 if(owned) gMacOwnScroll--;
}
static BOOL SameAnchor(BTMacTextReflowAnchor a) {
 return gMacTextReflowAnchor.valid && gMacTextReflowAnchor.character==a.character &&
        fabs(gMacTextReflowAnchor.lineFraction-a.lineFraction)<0.000001;
}
int main(int argc,char **argv) { @autoreleasepool {
 BOOL legacy=argc>1 && strcmp(argv[1],"legacy-observer")==0;
 NSView *parent=[[NSView alloc] initWithFrame:NSMakeRect(0,0,1000,15000)];
 gScroll=[[HBReadingScrollView alloc] initWithFrame:NSMakeRect(0,0,600,260)];
 gScroll.hasVerticalScroller=YES; gScroll.hasHorizontalScroller=NO;
 gScroll.autohidesScrollers=NO; [parent addSubview:gScroll];
 NSSize content=gScroll.contentSize;
 gTextView=[[HBReadingTextView alloc] initWithFrame:NSMakeRect(0,0,content.width,content.height)];
 gTextView.minSize=NSZeroSize;gTextView.maxSize=NSMakeSize(CGFLOAT_MAX,CGFLOAT_MAX);
 gTextView.verticallyResizable=YES;gTextView.horizontallyResizable=NO;
 gTextView.autoresizingMask=NSViewWidthSizable;
 gTextView.textContainer.containerSize=NSMakeSize(content.width,CGFLOAT_MAX);
 gTextView.textContainer.widthTracksTextView=YES;
 gTextView.textContainerInset=NSMakeSize(10,14);gScroll.documentView=gTextView;
 NSMutableString *words=[NSMutableString string];
 for(int i=0;i<220;i++) [words appendFormat:@"Synthetic%03d alpha beta gamma delta epsilon zeta eta theta. ",i];
 [gTextView.textStorage setAttributedString:[[NSAttributedString alloc] initWithString:words attributes:@{NSFontAttributeName:[NSFont systemFontOfSize:20]}]];
 Layout();NSSize docSize=gTextView.frame.size;
 CGFloat used=[gTextView.layoutManager usedRectForTextContainer:gTextView.textContainer].size.height+28;
 if(docSize.height<used) {docSize.height=ceil(used);[gTextView setFrameSize:docSize];}
 Layout();
 NSClipView *clip=gScroll.contentView;
 clip.postsBoundsChangedNotifications=YES;
 NSScrollView *sv=gScroll;
 __block NSRect lastClipBounds=clip.bounds;
 __block int notifications=0;
 id observer=[[NSNotificationCenter defaultCenter] addObserverForName:NSViewBoundsDidChangeNotification object:clip queue:nil usingBlock:^(NSNotification *n) {
  notifications++;
  if(legacy) {
   if(gReadAlongActive && !gReadAlongOwnScroll && gMacOwnScroll==0 && !gReadAlongUserLatch) {gReadAlongUserLatch=YES;bibleTextReadAlongUserScrolled();}
   if(gMacOwnScroll==0 && !gReadAlongOwnScroll) btMacUserScrolled();
  } else {
`)
	source.WriteString(section("// BEGIN MAC READING BOUNDS OBSERVER", "// END MAC READING BOUNDS OBSERVER"))
	source.WriteString(`
  }
 }];
 NSLayoutManager *lm=gTextView.layoutManager;
 NSUInteger glyph=[lm glyphIndexForCharacterAtIndex:gTextView.string.length-900];NSRange lineRange;
 NSRect line=[lm lineFragmentRectForGlyphAtIndex:glyph effectiveRange:&lineRange];
 NSUInteger character=[lm characterIndexForGlyphAtIndex:lineRange.location];
 CGFloat wanted=line.origin.y+14+.37*line.size.height;
 CGFloat maximum=MAX(0,gTextView.frame.size.height-clip.bounds.size.height);
 if(wanted<=300 || wanted>=maximum-1) return 1;
 Move(wanted,YES);gTextView.selectedRange=NSMakeRange(character+4,24);
 btMacClearTextReflow();btMacKeepTextReflow();BTMacTextReflowAnchor original=gMacTextReflowAnchor;
 if(!original.valid) return 2;
 unsigned generation=gMacScrollSettleGen;
 gReadAlongActive=YES;gReadAlongUserLatch=NO;readAlongUserCalls=0;
 int beforeNotifications=notifications;
 NSSize oldBounds=clip.bounds.size;
 [clip setBoundsSize:NSMakeSize(oldBounds.width,oldBounds.height+1)];
 if(notifications<=beforeNotifications) return 3; // actual NSView notification required
 if(legacy) {
  if(gMacTextReflowAnchor.valid || gMacScrollSettleGen==generation || !gReadAlongUserLatch || readAlongUserCalls!=1) return 4;
  [[NSNotificationCenter defaultCenter] removeObserver:observer];return 0;
 }
 if(!SameAnchor(original) || gMacScrollSettleGen!=generation || gReadAlongUserLatch || readAlongUserCalls!=0) return 5;
 [clip setBoundsSize:oldBounds];
 // Enlarge the actual viewport until AppKit clamps the origin. Its geometry
 // may emit the origin movement separately from the size notification.
 NSSize originalScroll=gScroll.frame.size;
 [gScroll setFrameSize:NSMakeSize(originalScroll.width,originalScroll.height+600)];
 Layout();
 if(Origin()>=wanted-1) return 6; // a real automatic clamp must have happened
 if(!SameAnchor(original) || gMacScrollSettleGen!=generation || gReadAlongUserLatch) return 7;
 [gScroll setFrameSize:originalScroll];Layout();OwnPut();
 if(!SameAnchor(original) || fabs(Origin()-wanted)>.5) return 8;
 if(!NSEqualRanges(gTextView.selectedRange,NSMakeRange(character+4,24))) return 9;
 // Shrinking the real document is the other source of automatic origin
 // clamps: it must not discard the original semantic target either.
 NSSize originalDocument=gTextView.frame.size;
 NSSize shorter=NSMakeSize(originalDocument.width,clip.bounds.size.height+wanted-100);
 [gTextView setFrameSize:shorter];
 [gScroll reflectScrolledClipView:clip];
 if(Origin()>=wanted-1) return 19; // require an actual document-height clamp
 if(!SameAnchor(original) || gMacScrollSettleGen!=generation) return 10;
 [gTextView setFrameSize:originalDocument];Layout();OwnPut();
 if(!SameAnchor(original) || fabs(Origin()-wanted)>.5) return 11;
 // A reader's unowned origin-only movement immediately wins, including the
 // event immediately following a resize: no timer/grace period is involved.
 gMacHasRestore=YES;gMacNoteArrival=1;gMacNoteArrivalVerse=1;
 CGFloat actual=Origin();Move(actual-30,NO);
 if(fabs(Origin()-actual)<1 || gMacTextReflowAnchor.valid || gMacHasRestore || gMacNoteArrival!=0 || gMacScrollSettleGen!=generation+1 || !gReadAlongUserLatch || readAlongUserCalls!=1) return 12;
 // Updating the observed origin during an owned move must not hide the next
 // genuine movement. Narration's own latch has the same existing contract.
 btMacKeepTextReflow();generation=gMacScrollSettleGen;gReadAlongUserLatch=NO;
 Move(Origin()-20,YES);
 if(!gMacTextReflowAnchor.valid || gMacScrollSettleGen!=generation || gReadAlongUserLatch) return 13;
 Move(Origin()-20,NO);
 if(gMacTextReflowAnchor.valid || gMacScrollSettleGen!=generation+1 || !gReadAlongUserLatch) return 14;
 btMacKeepTextReflow();generation=gMacScrollSettleGen;gReadAlongUserLatch=NO;gReadAlongOwnScroll=YES;
 Move(Origin()-20,NO);
 if(!gMacTextReflowAnchor.valid || gMacScrollSettleGen!=generation || gReadAlongUserLatch) return 15;
 gReadAlongOwnScroll=NO;Move(Origin()-20,NO);
 if(gMacTextReflowAnchor.valid || gMacScrollSettleGen!=generation+1 || !gReadAlongUserLatch) return 16;
 // The actual non-preserving push path still clears a transient target; this
 // is what explicit navigation hands to the native installation boundary.
 btMacKeepTextReflow();
 bibleTextMacPreserveSelectionForPush(1,0,"Synthetic",1);
 if(gMacTextReflowAnchor.valid) return 17;
 if(gMacOwnScroll!=0) return 18;
 [[NSNotificationCenter defaultCenter] removeObserver:observer];
 return 0;
} }
`)
	return source.String()
}

func TestMacReadingObserverSeparatesGeometryFromReaderScroll(t *testing.T) {
	program := macReadingObserverProgram(t)
	t.Run("geometry clamps and immediate reader takeover", func(t *testing.T) { runMacNativeProgram(t, "native reading geometry ownership", program) })
	t.Run("old observer loses target on a real size-only notification", func(t *testing.T) {
		runMacNativeProgram(t, "legacy observer negative control", program, "legacy-observer")
	})
}
