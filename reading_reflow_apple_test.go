//go:build darwin && !ios

package bibletext

import (
	"strings"
	"testing"
)

// The UIKit-independent guard is compiled from the iOS source against the
// equivalent Foundation/AppKit types. UIKit layout is validated separately.
func TestIOSReflowSelectionUsesActualTextAndValidUTF16Ranges(t *testing.T) {
	var guard strings.Builder
	for _, signature := range []string{
		"static NSUInteger btIOSPageContentEnd(NSAttributedString *ts) {",
		"static BOOL btIOSChapterMatches(NSString *beforeBook, NSInteger beforeChapter, NSString *afterBook, NSInteger afterChapter) {",
		"static BOOL btIOSRangeFits(NSRange range, NSUInteger limit) {",
		"static BOOL btIOSUTF16Boundary(NSString *text, NSUInteger offset) {",
		"static NSRange btIOSReflowCharacterRange(NSString *text, NSUInteger offset) {",
		"static BOOL btIOSTextAllowsRange(NSString *before, NSUInteger beforeEnd, NSString *after, NSUInteger afterEnd, NSRange range) {",
		"static BOOL btIOSCanPreserveSelection(BOOL requested, BOOL sameChapter, NSAttributedString *before, NSAttributedString *after, NSRange range) {",
	} {
		guard.WriteString(nativeFunctionSource(t, "reading_ios.go", signature))
		guard.WriteString("\n}\n")
	}
	program := `#import <AppKit/AppKit.h>
#define UIFont NSFont
` + guard.String() + `
static NSAttributedString *Page(NSString *body, NSString *note) {
    NSMutableAttributedString *p=[[NSMutableAttributedString alloc] initWithString:body attributes:@{NSFontAttributeName:[NSFont systemFontOfSize:20]}];
    if (note != nil) [p appendAttributedString:[[NSAttributedString alloc] initWithString:note attributes:@{NSFontAttributeName:[NSFont systemFontOfSize:17]}]];
    return p;
}
int main(void) { @autoreleasepool {
    NSString *body=@"First phrase. Second First phrase. 🌿 שלום";
    NSRange second=[body rangeOfString:@"First" options:0 range:NSMakeRange(8,body.length-8)];
    NSAttributedString *a=Page(body,@"\nFootnote A"), *b=Page(body,@"\n\nFootnote B");
    if (!btIOSCanPreserveSelection(YES,YES,a,b,second)) return 1;
    if (btIOSCanPreserveSelection(NO,YES,a,b,second)) return 2;
    if (btIOSCanPreserveSelection(YES,NO,a,b,second)) return 3;
    if (btIOSCanPreserveSelection(YES,YES,a,b,NSMakeRange(2,0))) return 4;
    if (btIOSCanPreserveSelection(YES,YES,a,b,NSMakeRange(0,a.length))) return 5;
    if (btIOSCanPreserveSelection(YES,YES,a,Page([body stringByReplacingOccurrencesOfString:@"Second" withString:@"Change"],nil),second)) return 6;
    NSRange emoji=[body rangeOfString:@"🌿"];
    if (!btIOSCanPreserveSelection(YES,YES,a,b,emoji)) return 7;
    if (!NSEqualRanges(btIOSReflowCharacterRange(body,emoji.location),emoji)) return 13;
    if (btIOSCanPreserveSelection(YES,YES,a,b,NSMakeRange(emoji.location,1))) return 8;
    if (!btIOSChapterMatches(@"John",3,@"John",3)) return 9;
    if (btIOSChapterMatches(@"John",3,@"John",4) || btIOSChapterMatches(@"John",3,@"Mark",3)) return 10;
    if (btIOSCanPreserveSelection(YES,YES,a,b,NSMakeRange(NSNotFound,2))) return 11;
    if (btIOSCanPreserveSelection(YES,YES,a,b,NSMakeRange(a.length-1,8))) return 12;
} return 0; }
`
	runMacNativeProgram(t, "iOS selection guard", program)
}
