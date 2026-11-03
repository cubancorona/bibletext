//go:build darwin && !ios

package bibletext

import (
	"strings"
	"testing"
)

func TestMacWidthReflowKeepsTheSameReadingCharacter(t *testing.T) {
	var source strings.Builder
	native := readNativeSource(t, "reading_macos.go")
	if start := strings.Index(native, "// BEGIN MAC TEXT REFLOW ANCHOR\n"); start >= 0 {
		end := strings.Index(native[start:], "// END MAC TEXT REFLOW ANCHOR")
		if end < 0 {
			t.Fatal("text reflow anchor source has no end")
		}
		source.WriteString(native[start : start+end])
	}
	for _, signature := range []string{
		"static CGFloat btMacVerseFontThreshold(NSTextStorage *ts) {",
		"static NSUInteger btMacLocForVerse(NSTextStorage *ts, NSInteger verse) {",
		"static NSInteger btMacVerseAtIndex(NSTextStorage *ts, NSUInteger ci, NSUInteger *outLoc) {",
		"static void btMacTopOfView(NSInteger *verse, CGFloat *delta) {",
		"static void btMacPutTopOfView(NSInteger verse, CGFloat delta) {",
		"static BOOL btMacApplyInsets(CGFloat w) {",
		"static void btMacApplyFrame(double x, double y, double w, double h) {",
	} {
		source.WriteString(nativeFunctionSource(t, "reading_macos.go", signature))
		source.WriteString("\n}\n")
	}
	program := `#import <AppKit/AppKit.h>
#include <math.h>
static NSScrollView *gScroll;
static NSTextView *gTextView;
static CGFloat gMacReadingMeasure=0, gMacReadingSideMin=15;
static BOOL gMacHasRestore=NO;
static int gMacNoteArrival=0;
enum { kMacArriveNothing=0 };
static NSRange gMacHighlightRange;
static void bibleTextMacEnsureTV(void) {}
static void btMacReadingWidthChanged(double w) {}
static void btMacLayoutFollowBtn(void) {}
static void btMacReassertPlacement(void) {}
static void btMacRefreshNote(void) {
    [gTextView.layoutManager ensureLayoutForTextContainer:gTextView.textContainer];
    [gTextView sizeToFit];
}
` + source.String() + `
static NSRect LineForCharacter(NSUInteger location) {
    NSLayoutManager *lm=gTextView.layoutManager;
    [lm ensureLayoutForTextContainer:gTextView.textContainer];
    NSUInteger glyph=[lm glyphIndexForCharacterAtIndex:location];
    return [lm lineFragmentRectForGlyphAtIndex:glyph effectiveRange:NULL];
}
int main(void) { @autoreleasepool {
    NSView *parent=[[NSView alloc] initWithFrame:NSMakeRect(0,0,1000,1000)];
    gScroll=[[NSScrollView alloc] initWithFrame:NSMakeRect(0,0,820,300)];
    gScroll.hasVerticalScroller=YES;
    gScroll.hasHorizontalScroller=NO;
    gScroll.autohidesScrollers=NO;
    [parent addSubview:gScroll];
    NSSize size=gScroll.contentSize;
    gTextView=[[NSTextView alloc] initWithFrame:NSMakeRect(0,0,size.width,size.height)];
    gTextView.minSize=NSZeroSize;
    gTextView.maxSize=NSMakeSize(CGFLOAT_MAX,CGFLOAT_MAX);
    gTextView.verticallyResizable=YES;
    gTextView.horizontallyResizable=NO;
    gTextView.autoresizingMask=NSViewWidthSizable;
    gTextView.textContainer.containerSize=NSMakeSize(size.width,CGFLOAT_MAX);
    gTextView.textContainer.widthTracksTextView=YES;
    gTextView.textContainerInset=NSMakeSize(10,14);
    gScroll.documentView=gTextView;
    NSMutableString *words=[NSMutableString stringWithString:@"1 "];
    for(int i=0;i<100;i++) [words appendString:@"alpha beta gamma delta epsilon zeta eta theta iota kappa lambda mu "];
    NSMutableAttributedString *as=[[NSMutableAttributedString alloc] initWithString:words
        attributes:@{NSFontAttributeName:[NSFont systemFontOfSize:20]}];
    [as addAttribute:NSFontAttributeName value:[NSFont systemFontOfSize:13] range:NSMakeRange(0,1)];
    [gTextView.textStorage setAttributedString:as];
    gMacHighlightRange=NSMakeRange(NSNotFound,0);
    btMacRefreshNote();
    // The measured character is the actual first glyph of a later line,
    // not a verse-relative pixel offset or a repeated-word search.
    NSUInteger location=500;
    NSLayoutManager *lm=gTextView.layoutManager;
    NSRange glyphLine;
    [lm lineFragmentRectForGlyphAtIndex:[lm glyphIndexForCharacterAtIndex:location] effectiveRange:&glyphLine];
    location=[lm characterIndexForGlyphAtIndex:glyphLine.location];
    NSRect first=LineForCharacter(location);
    CGFloat fraction=.37;
    CGFloat start=first.origin.y+fraction*first.size.height+gTextView.textContainerInset.height;
    [gScroll.contentView scrollToPoint:NSMakePoint(0,start)];
    [gScroll reflectScrolledClipView:gScroll.contentView];
    CGFloat widths[]={440,300,670,360,820,490,820};
    for(int i=0;i<7;i++) {
        btMacApplyFrame(0,0,widths[i],300);
        NSRect line=LineForCharacter(location);
        CGFloat want=line.origin.y+fraction*line.size.height+gTextView.textContainerInset.height;
        CGFloat got=gScroll.contentView.bounds.origin.y-gTextView.frame.origin.y;
        if(fabs(got-want)>.5) {
            fprintf(stderr,"width %.0f: scroll %.3f, want %.3f for character %lu\n",widths[i],got,want,(unsigned long)location);
            return 1;
        }
    }
    if(fabs(gScroll.contentView.bounds.origin.y-start)>.5) return 2;
    return 0;
} }
`
	runMacNativeProgram(t, "native frame reading-character retention", program)
}

func TestMacTextReflowCanonicalCoordinatesAndModelGuard(t *testing.T) {
	native := readNativeSource(t, "reading_macos.go")
	start := strings.Index(native, "// BEGIN MAC TEXT REFLOW ANCHOR\n")
	if start < 0 {
		t.Fatal("native text anchor is absent")
	}
	end := strings.Index(native[start:], "// END MAC TEXT REFLOW ANCHOR")
	if end < 0 {
		t.Fatal("native text anchor has no end")
	}
	program := `#import <AppKit/AppKit.h>
#include <math.h>
static NSScrollView *gScroll;
static NSTextView *gTextView;
` + native[start:start+end] + `
int main(void) { @autoreleasepool {
    NSMutableArray *one=[NSMutableArray array], *two=[NSMutableArray array];
    NSString *a=btMacReflowText(@"  alpha\nβeta  🙂 gamma  ",one);
    NSString *b=btMacReflowText(@"alpha βeta\n\n🙂 gamma",two);
    if(![a isEqualToString:b] || ![a isEqualToString:@"alpha βeta 🙂 gamma"]) return 1;
    if(one.count!=a.length || two.count!=b.length) return 2;
    for(NSUInteger i=0;i<a.length;i++) {
        unichar expected=[a characterAtIndex:i];
        if(expected==' ') continue;
        NSString *left=@"  alpha\nβeta  🙂 gamma  ", *right=@"alpha βeta\n\n🙂 gamma";
        if([left characterAtIndex:[one[i] unsignedIntegerValue]]!=expected ||
           [right characterAtIndex:[two[i] unsignedIntegerValue]]!=expected) return 3;
    }
    gScroll=[[NSScrollView alloc] initWithFrame:NSMakeRect(0,0,350,200)];
    gTextView=[[NSTextView alloc] initWithFrame:NSMakeRect(0,0,350,200)];
    gTextView.string=@"alpha beta gamma";
    gScroll.documentView=gTextView;
    NSString *model=nil;
    BTMacTextReflowAnchor anchor=btMacCaptureTextReflow(gTextView,gScroll.contentView,&model);
    CGFloat y=99;
    if(!btMacResolveTextReflow(gTextView,anchor,model,&y) || y!=0) return 4;
    gTextView.string=@"alpha beta sigma";
    if(btMacResolveTextReflow(gTextView,anchor,model,&y)) return 5;
    gMacTextReflowAnchor=anchor; gMacTextReflowModel=model;
    btMacClearTextReflow();
    if(gMacTextReflowAnchor.valid || gMacTextReflowModel!=nil) return 6;
    return 0;
} }
`
	runMacNativeProgram(t, "native canonical reading anchor guards", program)
}

func TestMacReadingReflowYieldsToExplicitRestores(t *testing.T) {
	for _, test := range []struct {
		name          string
		same, arrival bool
		restore       *restoreAnchor
		want          bool
	}{
		{"live width change", true, false, nil, true},
		{"captured presentation", true, false, &restoreAnchor{Reflow: true}, true},
		{"same chapter history", true, false, &restoreAnchor{Verse: 8, Delta: 17}, false},
		{"same chapter top visit", true, false, &restoreAnchor{}, false},
		{"explicit arrival", true, true, &restoreAnchor{Reflow: true}, false},
		{"another chapter", false, false, &restoreAnchor{Reflow: true}, false},
	} {
		if got := readingReflowWanted(test.same, test.arrival, test.restore); got != test.want {
			t.Errorf("%s: carry%v,want%v", test.name, got, test.want)
		}
	}
}
