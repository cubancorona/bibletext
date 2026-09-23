package bibletext

// A PUBLISHER'S HEADING BELONGS TO NO VERSE — on the native panes.
//
// A verse's character range on the native panes ran from its own number to the
// next verse's number. A heading is its own paragraph standing between two
// verses, so it fell inside the range of the verse ABOVE it: a note on that
// verse washed the heading too, the narration lit it, and a tap on it counted as
// a tap on the mark. reading_tint_wash_shape_test.go holds the RULE, in Go,
// against a model of what the importer produces (endOf, bareRanges). That test
// compares Go with Go and would pass whatever the Objective-C and Java did, so
// these hold the native code to the same rule, and the premise test below holds
// the markup to what the native detectors read.

import (
	"strings"
	"testing"
)

func TestNativeVerseRangesStopAtAHeading(t *testing.T) {
	for _, tc := range []struct {
		name, path, sig string
		java            bool
		must            []string
		mustNot         []string
	}{
		// iOS: the verse index records each verse's end and the heading
		// block after it, once per import; every range reads that.
		// The walk reads each verse's own span for a paragraph separator
		// rather than asking for the paragraph around every number, which
		// re-read a one-paragraph psalm once per verse.
		{"iOS index", "reading_ios.go", "static void btIOSBuildVerseIndex(NSTextStorage *ts) {", false,
			[]string{"btIOSIsHeadingParagraph(ts, para, thr)", "gVerseIndex[k].end = para.location",
				"gVerseIndex[k].tail = NSMaxRange(para)", "rangeOfCharacterFromSet:seps"},
			[]string{"paragraphRangeForRange:NSMakeRange(gVerseIndex[k].loc, 0)"}},
		{"iOS verse range", "reading_ios.go", "static NSRange btIOSReadAlongRange(NSTextStorage *ts, NSInteger verse) {", false,
			[]string{"gVerseIndex[lo].end"}, []string{"gVerseIndex[lo + 1].loc"}},
		// The exit at the range's end is what stops the loop: a narrated verse
		// just above a heading ends exactly where the heading block starts, so
		// without it the block is found, clamped back to the end, and found
		// again — forever, on the main thread.
		{"iOS bare ranges", "reading_ios.go", "static void btIOSBareRanges(NSTextStorage *ts, NSRange r, void (^yield)(NSRange bare)) {", false,
			[]string{"btIOSHeadingTailAt(hi)", "if (tail == NSNotFound || hi >= end) break;"}, nil},
		{"iOS heading block", "reading_ios.go", "static NSUInteger btIOSHeadingTailAt(NSUInteger at) {", false,
			[]string{"e->end == at", "e->tail > e->end"}, nil},
		{"iOS heading test", "reading_ios.go", "static BOOL btIOSIsHeadingParagraph(NSTextStorage *ts, NSRange para, CGFloat thr) {", false,
			[]string{"kCTFontTraitBold", "f.pointSize < thr"}, nil},
		// The tap target is the painted pieces, so a heading left bare inside
		// a multi-verse mark is plain paper to a tap as well as to the eye.
		{"iOS tap target", "reading_ios.go", "static BOOL btIOSPointInChapterWash(CGPoint inContainer) {", false,
			[]string{"btIOSPaintedPieces(ts, r,"}, nil},
		// macOS has no verse index: the range and the bare ranges ask the
		// paragraph directly.
		{"macOS verse range", "reading_macos.go", "static NSRange btMacReadAlongRange(NSTextStorage *ts, NSInteger verse) {", false,
			[]string{"btMacIsHeadingParagraph(ts, para, thr)", "nextLoc = para.location"}, nil},
		{"macOS bare ranges", "reading_macos.go", "static void btMacUnwashBreaks(NSTextStorage *ts, NSRange r) {", false,
			[]string{"btMacHeadingBlockEndAt(ts, hi, end, thr)", "if (hi >= end) break;"}, nil},
		{"macOS heading block", "reading_macos.go", "static NSUInteger btMacHeadingBlockEndAt(NSTextStorage *ts, NSUInteger at, NSUInteger limit, CGFloat thr) {", false,
			[]string{"while (at < limit && at < s.length)", "para.location != at"}, nil},
		{"macOS heading test", "reading_macos.go", "static BOOL btMacIsHeadingParagraph(NSTextStorage *ts, NSRange para, CGFloat thr) {", false,
			[]string{"kCTFontTraitBold", "f.pointSize < thr", "integerValue"}, nil},
		// Android washes from the markup; its verse ranges are the narration's.
		{"Android index", "android/BtBridge.java", "private static void buildVerseIndex(CharSequence cs) {", true,
			[]string{"ends[count] = endBeforeHeading(sp, st, en)"}, nil},
		{"Android verse end", "android/BtBridge.java", "private static int endBeforeHeading(Spanned sp, int st, int en) {", true,
			[]string{"isHeadingParagraph(sp, ls, le)", "if (le > en) break;"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body string
			if tc.java {
				body = javaMethodSource(t, tc.path, tc.sig)
			} else {
				body = nativeFunctionSource(t, tc.path, tc.sig)
			}
			for _, want := range tc.must {
				if !strings.Contains(body, want) {
					t.Errorf("%s: %s no longer contains %q — a heading can fall back into the verse above it",
						tc.path, strings.TrimSuffix(tc.sig, " {"), want)
				}
			}
			for _, bad := range tc.mustNot {
				if strings.Contains(body, bad) {
					t.Errorf("%s: %s still reads %q — the verse ends at the next number again, heading and all",
						tc.path, strings.TrimSuffix(tc.sig, " {"), bad)
				}
			}
		})
	}
}

// The native detectors recognise a heading by what the markup makes of it: a
// paragraph set BOLD (the Apple stylesheet's p.sec at weight 700, kept through
// the reading-face swap as Junicode-Bold; Android's <p><b>…</b></p>, one
// StyleSpan(BOLD) over the paragraph). If a heading ever stopped being bold the
// detectors would stop finding it and the verse above would quietly swallow it
// again, with nothing else failing. So the premise is held here.
func TestHeadingsReachTheNativePanesBold(t *testing.T) {
	verses := proseChapter()
	st := tintState("Romans", 8, "web", verses)
	st.Bible.Headings = map[string]map[int][]Heading{"Romans": {8: {{Text: "A Heading", BeforeVerse: 4}}}}

	apple := buildChapterHTML(st, verses)
	if !strings.Contains(apple, `<p class="sec">A Heading</p>`) {
		t.Errorf("the Apple dialect no longer writes a heading as <p class=\"sec\">")
	}
	i := strings.Index(apple, "p.sec {")
	if i < 0 {
		t.Fatalf("the Apple stylesheet has no p.sec rule")
	}
	rule := apple[i:]
	if j := strings.Index(rule, "}"); j >= 0 {
		rule = rule[:j]
	}
	if !strings.Contains(rule, "font-weight: 700") {
		t.Errorf("p.sec is no longer bold (%q) — btIOSIsHeadingParagraph and btMacIsHeadingParagraph test for the bold trait", rule)
	}
	fonts := readNativeSource(t, "reading_fonts_apple.go")
	if !strings.Contains(fonts, `name = "Junicode-Bold"`) {
		t.Errorf("the reading-face swap no longer maps a bold run to Junicode-Bold — the heading's bold trait may not survive it")
	}

	android := buildChapterHTMLAndroid(st, verses)
	if !strings.Contains(android, "<p><b>A Heading</b></p>") {
		t.Errorf("the Android dialect no longer writes a heading as <p><b>…</b></p> — isHeadingParagraph reads one StyleSpan(BOLD) over the paragraph")
	}
}
