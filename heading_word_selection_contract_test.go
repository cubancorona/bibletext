package bibletext

// The native panes' half of a word selected inside a publisher's heading
// (selSpan.heading; heading_word_selection_test.go holds Go's). A host test
// runs neither the Objective-C nor the Java, but it reads both, which are
// source here. Each pane tests the selection it hands over against the
// heading range it keeps beside its verse numbers, after it has cut the
// selection to the scripture it may dispatch, and hands a selection wholly
// inside a heading over as the verse below, with the flag and no lead,
// through every call.

import (
	"strings"
	"testing"
)

// headingSelectionCheck is one pane's rule as source: the function that
// tests a selection against the heading range, the method that hands its
// answer over, and the calls carrying it.
type headingSelectionCheck struct {
	name, file string
	java       bool
	fn         string   // the heading test's signature
	body       []string // what its body must hold
	within     string   // the method that dispatches the selection
	sequence   []string // what that method must do, in this order
	dispatch   []string // the calls that hand the selection to Go
	args       []string // their third to fifth arguments
}

var headingSelectionChecks = []headingSelectionCheck{
	{
		name: "iOS", file: "reading_ios.go",
		fn: "static NSInteger btIOSHeadingSelectionVerse(NSTextStorage *ts, NSRange r) {",
		body: []string{
			"while (a < b && [blank characterIsMember:[s characterAtIndex:a]]) a++;",
			"while (b > a && [blank characterIsMember:[s characterAtIndex:b - 1]]) b--;",
			"if (gVerseIndex[mid].loc <= a) lo = mid + 1; else hi = mid;",
			"if (lo == 0 || lo >= gVerseIndexCount) return 0;",
			"if (e.tail <= e.end || a < e.end || b > e.tail) return 0;",
			"return gVerseIndex[lo].verse;",
		},
		within: "editMenuForTextInRange:(NSRange)range",
		sequence: []string{
			"range.location = contentStart;",
			"NSString *captured = [[textView.text substringWithRange:range] copy];",
			"int headVerse = (int)btIOSHeadingSelectionVerse(textView.textStorage, range);",
			"int heading = headVerse > 0 ? 1 : 0;",
			"spanLo = spanHi = headVerse;",
			"lead = nil;",
		},
		dispatch: []string{"bibleTextAIMenuTapped", "bibleTextStudyMenuTapped"},
		args:     []string{"spanLo", "spanHi", "heading"},
	},
	{
		name: "Mac AI", file: "reading_macos.go",
		fn: "static NSInteger btMacHeadingSelectionVerse(NSTextStorage *ts, NSRange r) {",
		body: []string{
			"while (a < b && [blank characterIsMember:[s characterAtIndex:a]]) a++;",
			"while (b > a && [blank characterIsMember:[s characterAtIndex:b - 1]]) b--;",
			"NSRange para = [s paragraphRangeForRange:NSMakeRange(a, 0)];",
			"NSUInteger end = btMacHeadingBlockEndAt(ts, para.location, contentEnd, thr);",
			"if (end == NSNotFound || b > end || end >= contentEnd) return 0;",
			"inRange:NSMakeRange(end, contentEnd - end)",
			"if (v > 0) { below = v; *stop = YES; }",
		},
		within: "- (void)hbSendAI:(const char *)action {",
		sequence: []string{
			"NSRange sel = [self hbScriptureSelection];",
			"NSString *lead = btMacSelectionLead(self.textStorage, sel, lo, loLoc);",
			"NSInteger headVerse = btMacHeadingSelectionVerse(self.textStorage, sel);",
			"if (headVerse > 0) { lo = hi = (int)headVerse; lead = nil; heading = 1; }",
		},
		dispatch: []string{"bibleTextAIMenuTapped"},
		args:     []string{"lo", "hi", "heading"},
	},
	{
		name: "Mac study", file: "reading_macos.go",
		within: "- (void)hbSendStudy:(const char *)action {",
		sequence: []string{
			"NSRange sel = [self hbScriptureSelection];",
			"NSString *lead = btMacSelectionLead(self.textStorage, sel, lo, loLoc);",
			"NSInteger headVerse = btMacHeadingSelectionVerse(self.textStorage, sel);",
			"if (headVerse > 0) { lo = hi = (int)headVerse; lead = nil; heading = 1; }",
		},
		dispatch: []string{"bibleTextStudyMenuTapped"},
		args:     []string{"lo", "hi", "heading"},
	},
	{
		name: "Android", file: "android/BtBridge.java", java: true,
		fn: "private static int headingSelectionVerse(CharSequence cs, int s0, int s1) {",
		body: []string{
			"while (a < b && isBlankBesideHeading(cs.charAt(a))) a++;",
			"while (b > a && isBlankBesideHeading(cs.charAt(b - 1))) b--;",
			"if (verseEnds[k] >= verseStarts[k + 1] || a < verseEnds[k] || b > verseStarts[k + 1]) return 0;",
			"if (!blank && !isHeadingParagraph(sp, p, pe)) return 0;",
			"return verseNums[k + 1];",
		},
		within: "@Override public boolean onActionItemClicked(ActionMode mode, MenuItem item) {",
		sequence: []string{
			"s0 = Math.max(s0, contentStart);",
			"final String sel = readerText(text.getText(), s0, s1).toString();",
			"final int headVerse = headingSelectionVerse(text.getText(), s0, s1);",
			"final boolean selHeading = headVerse > 0;",
			"final int selLo = selHeading ? headVerse : verseAtOffset(s0);",
			"final int selHi = selHeading ? headVerse : s1 > s0 ? verseAtOffset(s1 - 1) : selLo;",
			"final String selLead = selHeading ? null : selectionLead(text.getText(), s0);",
		},
		dispatch: []string{"nativeSelectionAction", "showStudyPopup"},
		args:     []string{"selLo", "selHi", "selHeading"},
	},
}

// headingSelectionFaults is what src lacks of c's rule, nil when it holds it.
func headingSelectionFaults(src string, c headingSelectionCheck) []string {
	var faults []string
	end := "\n}\n"
	if c.java {
		end = "\n    }\n"
	}
	if c.fn != "" {
		i := strings.Index(src, c.fn)
		if i < 0 || strings.Index(src[i:], end) < 0 {
			faults = append(faults, "no "+c.fn)
		} else {
			body := src[i : i+strings.Index(src[i:], end)]
			for _, want := range c.body {
				if !strings.Contains(body, want) {
					faults = append(faults, "the heading test lacks "+want)
				}
			}
		}
	}
	i := strings.Index(src, c.within)
	if i < 0 {
		return append(faults, "no "+c.within)
	}
	method := src[i:]
	if c.java {
		method = method[:strings.Index(method, "\n            }\n")]
	} else {
		method = method[:strings.Index(method, end)]
	}
	if !inSequence(method, c.sequence...) {
		faults = append(faults, "the dispatch does not cut the selection, test it against the heading range and hand the verse below over with no lead, in that order")
	}
	for _, fn := range c.dispatch {
		calls := callArgLists(method, fn)
		for _, args := range calls {
			if !passesTheFlag(args, c.args) {
				faults = append(faults, fn+" does not pass ("+strings.Join(c.args, ", ")+")")
			}
		}
		if len(calls) != 1 {
			faults = append(faults, fn+" is not called once")
		}
	}
	return faults
}

// passesTheFlag reports whether args hold want, the span and the flag, one
// after another.
func passesTheFlag(args, want []string) bool {
	for i := 0; i+len(want) <= len(args); i++ {
		if strings.Join(args[i:i+len(want)], "|") == strings.Join(want, "|") {
			return true
		}
	}
	return false
}

func TestEveryNativePaneHandsAHeadingsSelectionOverAsTheVerseBelow(t *testing.T) {
	for _, c := range headingSelectionChecks {
		src := readNativeSource(t, c.file)
		for _, f := range headingSelectionFaults(src, c) {
			t.Errorf("%s (%s): %s", c.name, c.file, f)
		}
	}

	// The controls: each mutation is seen.
	const iosTest = "    int headVerse = (int)btIOSHeadingSelectionVerse(textView.textStorage, range);\n"
	for _, m := range []struct {
		name, file string
		check      int
		mutate     func(string) string
	}{
		{"iOS without the end of the heading block", "reading_ios.go", 0, func(s string) string {
			return strings.Replace(s, "a < e.end || b > e.tail", "a < e.end", 1)
		}},
		{"iOS testing the selection before the clamp", "reading_ios.go", 0, func(s string) string {
			s = strings.Replace(s, iosTest, "", 1)
			return strings.Replace(s, "    if (range.location < contentStart) {\n", iosTest+"    if (range.location < contentStart) {\n", 1)
		}},
		{"iOS passing no flag", "reading_ios.go", 0, func(s string) string {
			return strings.Replace(s, "spanLo, spanHi, heading,", "spanLo, spanHi, 0,", 1)
		}},
		{"the Mac's AI verbs keeping the lead", "reading_macos.go", 1, func(s string) string {
			return strings.Replace(s, "lo = hi = (int)headVerse; lead = nil; heading = 1;", "lo = hi = (int)headVerse; heading = 1;", 1)
		}},
		{"Android naming the verse above", "android/BtBridge.java", 3, func(s string) string {
			return strings.Replace(s, "return verseNums[k + 1];", "return verseNums[k];", 1)
		}},
		{"Android's toolbar losing the flag", "android/BtBridge.java", 3, func(s string) string {
			return strings.Replace(s, "selLo, selHi, selHeading, selLead, selBook, selChapter);\n                return true;",
				"selLo, selHi, false, selLead, selBook, selChapter);\n                return true;", 1)
		}},
	} {
		src := readNativeSource(t, m.file)
		bad := m.mutate(src)
		if bad == src {
			t.Fatalf("control %q: the mutation did not apply to %s", m.name, m.file)
		}
		if len(headingSelectionFaults(bad, headingSelectionChecks[m.check])) == 0 {
			t.Errorf("control %q: the check passes the mutated %s; it proves nothing", m.name, m.file)
		}
	}
}

// The popup carries the flag to its rows, the JNI thunk hands it on, and the
// two Go exports mark the span with it (selSpan.inHeading).
func TestTheHeadingFlagCrossesEverySeam(t *testing.T) {
	java := readNativeSource(t, "android/BtBridge.java")
	for _, want := range []string{
		"private static native void nativeSelectionAction(String action, String text, int lo, int hi, boolean heading,",
		"final int selLo, final int selHi, final boolean selHeading,",
	} {
		if !strings.Contains(java, want) {
			t.Errorf("android/BtBridge.java lacks %q", want)
		}
	}
	if n := strings.Count(java, "nativeSelectionAction(action, sel, selLo, selHi, selHeading, selLead, selBook, selChapter);"); n != 2 {
		t.Errorf("android/BtBridge.java: %d dispatches pass the flag, want the popup's and the toolbar's", n)
	}
	thunk := readNativeSource(t, "reading_jni_android.c")
	if !strings.Contains(thunk, "jint jLo, jint jHi, jboolean jHeading, jstring jLead,") ||
		!strings.Contains(thunk, "(int)jLo, (int)jHi, jHeading ? 1 : 0, (char *)lead,") {
		t.Error("reading_jni_android.c: the thunk does not take the flag as a jboolean and hand it on as 1 or 0")
	}
	darwin := readNativeSource(t, "ai_menu_darwin.go")
	if n := strings.Count(darwin, "span := nativeSelSpan(lo, hi, heading, cLead, cBook, chapter)"); n != 2 ||
		!strings.Contains(darwin, "return paneSelSpan(int(lo), int(hi), lead, cLead != nil, book, int(chapter)).inHeading(heading != 0)") {
		t.Error("ai_menu_darwin.go: the exports do not mark the span with the pane's flag")
	}
	android := readNativeSource(t, "reading_android_export.go")
	if !strings.Contains(android, "span := paneSelSpan(int(lo), int(hi), lead, cLead != nil, book, int(chapter)).inHeading(heading != 0)") {
		t.Error("reading_android_export.go: btaSelectionAction does not mark the span with the pane's flag")
	}
}
