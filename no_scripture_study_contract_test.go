package bibletext

// The native panes' half of no study verb for a selection that holds no
// word of Scripture (no_scripture_study_test.go holds Go's). A host test
// runs neither the Objective-C nor the Java, but it reads both, which are
// source here. Each pane keeps one rule over the page it draws: a selection,
// cut to the scripture its verbs may take, holds Scripture when it holds a
// letter or figure of a verse's words, after the verse's number and before
// its end, and not in a heading or an omitted verse's mark, so punctuation
// alone holds none; a selection that crosses a heading, with no letter or
// figure of a verse's words before the heading's first selected word,
// does not count a word it ends inside, back to the white space or the
// closed-up dash before it. With no verse index the pane asks for a letter
// or figure anywhere in the selection. Each pane builds its menu with Study
// with AI only when the rule finds Scripture, Share and Cross-references
// keeping their places, and Android, whose toolbar is built once per
// selection, shows or hides the item each time the toolbar is prepared
// again and asks again when it is chosen.

import (
	"strings"
	"testing"
)

// studyGateCheck is one piece of a pane's rule as source: a function or a
// method, what ends it, and what it must hold, in order.
type studyGateCheck struct {
	name, file string
	start      string   // the function's or method's first line
	end        string   // what ends it
	sequence   []string // what it must hold, in this order
}

const (
	objcEnd       = "\n}\n"
	javaMethodEnd = "\n    }\n"
	javaInnerEnd  = "\n            }\n"
)

// The step back from a word a selection with a heading's words ends inside:
// to the white space or the closed-up dash before it, as Go's end repair
// cuts back, so an apostrophe or a hyphen inside the word is stepped over
// too.
const (
	appleStepBack   = "while (b > h && ![blank characterIsMember:[s characterAtIndex:b - 1]] && [s characterAtIndex:b - 1] != dash) b--;"
	androidStepBack = "while (end > h && !isBlankBesideHeading(cs.charAt(end - 1)) && cs.charAt(end - 1) != EM_DASH) end--;"
)

var studyGateChecks = []studyGateCheck{
	{
		name: "iOS rule", file: "reading_ios.go", end: objcEnd,
		start: "static BOOL btIOSSelectionHoldsScripture(NSTextStorage *ts, NSRange r) {",
		sequence: []string{
			"if (gVerseIndexCount == 0) return [s rangeOfCharacterFromSet:word options:0 range:r].location != NSNotFound;",
			"const unichar dash = 0x2014;",
			"BOOL above = NO;",
			"if ([word characterIsMember:[s characterAtIndex:i]]) above = YES;",
			"to = gVerseIndex[k].tail < b ? gVerseIndex[k].tail : b;",
			"if (![blank characterIsMember:[s characterAtIndex:i]]) { h = i; break; }",
			"if (h != NSNotFound && !above && b < s.length && [word characterIsMember:[s characterAtIndex:b]]) {",
			appleStepBack,
			"NSUInteger from = gVerseIndex[k].loc + gVerseIndex[k].len, to = gVerseIndex[k].end;",
			"if ([word characterIsMember:[s characterAtIndex:i]]) return YES;",
			"return NO;",
		},
	},
	{
		name: "iOS menu", file: "reading_ios.go", end: objcEnd,
		start: "editMenuForTextInRange:(NSRange)range",
		sequence: []string{
			"range.length = contentEnd - range.location;",
			"range.location = contentStart;",
			"BOOL offerStudy = gBTAIEnabled && btIOSSelectionHoldsScripture(textView.textStorage, range);",
			"[children addObjectsFromArray:(offerStudy ? @[ai, share, xref] : gBTAIEnabled ? @[share, xref] : @[xref, share])];",
		},
	},
	{
		name: "Mac rule", file: "reading_macos.go", end: objcEnd,
		start: "static BOOL btMacSelectionHoldsScripture(NSTextStorage *ts, NSRange r) {",
		sequence: []string{
			"const unichar dash = 0x2014;",
			"BOOL above = NO;",
			"if (btMacIsHeadingParagraph(ts, para, thr)) {",
			"if (![blank characterIsMember:[s characterAtIndex:i]]) { h = i; break; }",
			"} else if (!above) {",
			"above = btMacVerseWordsHold(ts, NSMakeRange(p, pe - p), thr, word);",
			"if (h != NSNotFound && !above && b < s.length && [word characterIsMember:[s characterAtIndex:b]]) {",
			appleStepBack,
			"if (!btMacIsHeadingParagraph(ts, para, thr) &&",
			"btMacVerseWordsHold(ts, NSMakeRange(p, pe - p), thr, word)) return YES;",
			"return NO;",
		},
	},
	{
		name: "Mac words", file: "reading_macos.go", end: objcEnd,
		start: "static BOOL btMacVerseWordsHold(NSTextStorage *ts, NSRange r, CGFloat thr, NSCharacterSet *set) {",
		sequence: []string{
			"if (val != nil && ((NSFont *)val).pointSize < thr) return;",
			"if ([set characterIsMember:[s characterAtIndex:i]]) { found = YES; *stop = YES; return; }",
			"return found;",
		},
	},
	{
		name: "Mac menu", file: "reading_macos.go", end: objcEnd,
		start: "- (NSMenu *)menuForEvent:(NSEvent *)event {",
		sequence: []string{
			"NSRange scripture = [self hbScriptureSelection];",
			"BOOL study = aiOn && scripture.location != NSNotFound &&",
			"btMacSelectionHoldsScripture(self.textStorage, scripture);",
			"if (study) {",
			"[menu addItem:aiItem];",
			"} else if (!aiOn) {",
			"[menu addItem:shareItem];",
			"if (aiOn) [menu addItem:xref];",
		},
	},
	{
		name: "Android rule", file: "android/BtBridge.java", end: javaMethodEnd,
		start: "private static boolean selectionHoldsScripture(CharSequence cs, int s0, int s1) {",
		sequence: []string{
			"if (verseNums.length == 0) return holdsLetterOrDigit(cs, s0, s1);",
			"int h = s1;",
			"boolean above = false;",
			"if (!above) above = verseWordsHold(sp, k, s0, s1, true);",
			"int a = Math.max(verseEnds[k], s0), b = Math.min(verseStarts[k + 1], s1);",
			"if (isHeadingParagraph(sp, ls, le)) {",
			"h = i;",
			"if (h < s1 && !above && end < cs.length() && Character.isLetterOrDigit(cs.charAt(end))) {",
			androidStepBack,
			"if (verseWordsHold(sp, k, s0, end, true)) return true;",
			"return false;",
		},
	},
	{
		name: "Android words", file: "android/BtBridge.java", end: javaMethodEnd,
		start: "private static boolean verseWordsHold(Spanned sp, int k, int s0, int s1, boolean letters) {",
		sequence: []string{
			"if (verseEnds[k] <= s0 || verseStarts[k] >= s1) return false;",
			"int a = Math.max(numberEnd(sp, verseStarts[k]), s0), b = Math.min(verseEnds[k], s1);",
			"if (letters ? Character.isLetterOrDigit(c) : !isBlankBesideHeading(c)) return true;",
			"return false;",
		},
	},
	{
		name: "Android letters", file: "android/BtBridge.java", end: javaMethodEnd,
		start: "private static boolean holdsLetterOrDigit(CharSequence cs, int s0, int s1) {",
		sequence: []string{
			"if (Character.isLetterOrDigit(cs.charAt(i))) return true;",
			"return false;",
		},
	},
	{
		name: "Android number", file: "android/BtBridge.java", end: javaMethodEnd,
		start: "private static int numberEnd(Spanned sp, int st) {",
		sequence: []string{
			"sp.getSpans(st, st + 1, SuperscriptSpan.class)",
			"if (sp.getSpanStart(s) == st && sp.getSpanEnd(s) > end) end = sp.getSpanEnd(s);",
		},
	},
	{
		name: "Android selection", file: "android/BtBridge.java", end: javaMethodEnd,
		start: "private static boolean studyOffered() {",
		sequence: []string{
			"int a = text.getSelectionStart(), b = text.getSelectionEnd();",
			"int s0 = Math.max(Math.min(a, b), contentStart);",
			"int ce = contentEnd > 0 ? Math.min(contentEnd, cs.length()) : cs.length();",
			"int s1 = Math.min(Math.max(a, b), ce);",
			"return s1 > s0 && selectionHoldsScripture(cs, s0, s1);",
		},
	},
	{
		name: "Android toolbar", file: "android/BtBridge.java", end: javaInnerEnd,
		start: "@Override public boolean onCreateActionMode(ActionMode mode, Menu menu) {",
		sequence: []string{
			`MenuItem ai = menu.add(0, 200, 100, "Study with AI");`,
			"ai.setVisible(studyOffered());",
			`SubMenu sh = menu.addSubMenu(0, 201, 101, "Share");`,
			`menu.add(0, 105, aiOn ? 102 : 100, "Cross-references");`,
		},
	},
	{
		name: "Android toolbar again", file: "android/BtBridge.java", end: javaInnerEnd,
		start: "@Override public boolean onPrepareActionMode(ActionMode mode, Menu menu) {",
		sequence: []string{
			"MenuItem ai = menu.findItem(200);",
			"boolean show = studyOffered();",
			"ai.setVisible(show);",
			"return true;",
		},
	},
	{
		name: "Android chosen", file: "android/BtBridge.java", end: javaInnerEnd,
		start: "@Override public boolean onActionItemClicked(ActionMode mode, MenuItem item) {",
		sequence: []string{
			"s0 = Math.max(s0, contentStart);",
			"if (item.getItemId() == 200) {",
			"if (!selectionHoldsScripture(text.getText(), s0, s1)) {",
			"mode.finish();",
			"showStudyPopup(mode, sel, s1,",
		},
	},
}

// studyGateFaults is what src lacks of c, nil when it holds it.
func studyGateFaults(src string, c studyGateCheck) []string {
	i := strings.Index(src, c.start)
	if i < 0 {
		return []string{"no " + c.start}
	}
	j := strings.Index(src[i:], c.end)
	if j < 0 {
		return []string{"no end to " + c.start}
	}
	body := src[i : i+j]
	if !inSequence(body, c.sequence...) {
		var missing []string
		for _, s := range c.sequence {
			if !strings.Contains(body, s) {
				missing = append(missing, s)
			}
		}
		if len(missing) == 0 {
			return []string{"holds every step but not in order"}
		}
		return []string{"lacks " + strings.Join(missing, " | ")}
	}
	return nil
}

func TestEveryNativePaneOffersNoStudyVerbForASelectionWithoutScripture(t *testing.T) {
	for _, c := range studyGateChecks {
		for _, f := range studyGateFaults(readNativeSource(t, c.file), c) {
			t.Errorf("%s (%s): %s", c.name, c.file, f)
		}
	}

	// The controls: each mutation is seen.
	const iosStudy = "    BOOL offerStudy = gBTAIEnabled && btIOSSelectionHoldsScripture(textView.textStorage, range);\n"
	const iosEnd = "    NSUInteger contentEnd = btIOSContentEnd(textView.textStorage);\n"
	for _, m := range []struct {
		name, check string
		mutate      func(string) string
	}{
		{"iOS offering Study for every selection", "iOS menu", func(s string) string {
			return strings.Replace(s, iosStudy, "    BOOL offerStudy = gBTAIEnabled;\n", 1)
		}},
		{"iOS asking the rule before the cut", "iOS menu", func(s string) string {
			s = strings.Replace(s, iosStudy, "", 1)
			return strings.Replace(s, iosEnd, iosEnd+iosStudy, 1)
		}},
		{"iOS counting a verse's number as its words", "iOS rule", func(s string) string {
			return strings.Replace(s, "gVerseIndex[k].loc + gVerseIndex[k].len, to", "gVerseIndex[k].loc, to", 1)
		}},
		{"iOS keeping a word a heading's drag ends inside", "iOS rule", func(s string) string {
			return strings.Replace(s, appleStepBack, "", 1)
		}},
		{"iOS stepping back over letters and figures alone", "iOS rule", func(s string) string {
			return strings.Replace(s, appleStepBack, "while (b > h && [word characterIsMember:[s characterAtIndex:b - 1]]) b--;", 1)
		}},
		{"iOS counting punctuation as Scripture", "iOS rule", func(s string) string {
			return strings.Replace(s, "if ([word characterIsMember:[s characterAtIndex:i]]) return YES;",
				"if ([nonblank characterIsMember:[s characterAtIndex:i]]) return YES;", 1)
		}},
		{"iOS stepping back from a word a drag begun above ends inside", "iOS rule", func(s string) string {
			return strings.Replace(s, "if (h != NSNotFound && !above && b < s.length", "if (h != NSNotFound && b < s.length", 1)
		}},
		{"iOS counting punctuation above a heading as a complete word", "iOS rule", func(s string) string {
			return strings.Replace(s, "if ([word characterIsMember:[s characterAtIndex:i]]) above = YES;",
				"if ([[blank invertedSet] characterIsMember:[s characterAtIndex:i]]) above = YES;", 1)
		}},
		{"the Mac counting punctuation above a heading as a complete word", "Mac rule", func(s string) string {
			return strings.Replace(s, "above = btMacVerseWordsHold(ts, NSMakeRange(p, pe - p), thr, word);",
				"above = btMacVerseWordsHold(ts, NSMakeRange(p, pe - p), thr, [blank invertedSet]);", 1)
		}},
		{"Android counting punctuation above a heading as a complete word", "Android rule", func(s string) string {
			return strings.Replace(s, "if (!above) above = verseWordsHold(sp, k, s0, s1, true);",
				"if (!above) above = verseWordsHold(sp, k, s0, s1, false);", 1)
		}},
		{"iOS offering the verbs with no verse index for punctuation alone", "iOS rule", func(s string) string {
			return strings.Replace(s, "if (gVerseIndexCount == 0) return [s rangeOfCharacterFromSet:word options:0 range:r].location != NSNotFound;",
				"if (gVerseIndexCount == 0) return YES;", 1)
		}},
		{"the Mac stepping back over letters and figures alone", "Mac rule", func(s string) string {
			i := strings.Index(s, "static BOOL btMacSelectionHoldsScripture(")
			return s[:i] + strings.Replace(s[i:], appleStepBack, "while (b > h && [word characterIsMember:[s characterAtIndex:b - 1]]) b--;", 1)
		}},
		{"the Mac counting punctuation as Scripture", "Mac rule", func(s string) string {
			return strings.Replace(s, "btMacVerseWordsHold(ts, NSMakeRange(p, pe - p), thr, word)) return YES;",
				"btMacVerseWordsHold(ts, NSMakeRange(p, pe - p), thr, nonblank)) return YES;", 1)
		}},
		{"the Mac stepping back from a word a drag begun above ends inside", "Mac rule", func(s string) string {
			return strings.Replace(s, "if (h != NSNotFound && !above && b < s.length", "if (h != NSNotFound && b < s.length", 1)
		}},
		{"Android stepping back over letters and figures alone", "Android rule", func(s string) string {
			return strings.Replace(s, androidStepBack, "while (end > h && Character.isLetterOrDigit(cs.charAt(end - 1))) end--;", 1)
		}},
		{"Android counting punctuation as Scripture", "Android rule", func(s string) string {
			return strings.Replace(s, "if (verseWordsHold(sp, k, s0, end, true)) return true;",
				"if (verseWordsHold(sp, k, s0, end, false)) return true;", 1)
		}},
		{"Android stepping back from a word a drag begun above ends inside", "Android rule", func(s string) string {
			return strings.Replace(s, "if (h < s1 && !above && end", "if (h < s1 && end", 1)
		}},
		{"Android offering the verbs with no verse index for punctuation alone", "Android rule", func(s string) string {
			return strings.Replace(s, "if (verseNums.length == 0) return holdsLetterOrDigit(cs, s0, s1);",
				"if (verseNums.length == 0) return true;", 1)
		}},
		{"Android counting any character with no verse index", "Android letters", func(s string) string {
			return strings.Replace(s, "if (Character.isLetterOrDigit(cs.charAt(i))) return true;",
				"if (!Character.isWhitespace(cs.charAt(i))) return true;", 1)
		}},
		{"the Mac offering Study for every selection", "Mac menu", func(s string) string {
			return strings.Replace(s, "    if (study) {\n", "    if (aiOn) {\n", 1)
		}},
		{"the Mac counting verse numbers as words", "Mac words", func(s string) string {
			return strings.Replace(s, "if (val != nil && ((NSFont *)val).pointSize < thr) return;", "", 1)
		}},
		{"Android building the item shown", "Android toolbar", func(s string) string {
			return strings.Replace(s, "ai.setVisible(studyOffered());", "", 1)
		}},
		{"Android not following the handles", "Android toolbar again", func(s string) string {
			return strings.Replace(s, "ai.setVisible(show);", "", 1)
		}},
		{"Android counting a verse's number as its words", "Android words", func(s string) string {
			return strings.Replace(s, "Math.max(numberEnd(sp, verseStarts[k]), s0)", "Math.max(verseStarts[k], s0)", 1)
		}},
		{"Android not telling a heading", "Android rule", func(s string) string {
			return strings.Replace(s, "if (isHeadingParagraph(sp, ls, le)) {", "if (false) {", 1)
		}},
		{"Android opening the study popup unasked", "Android chosen", func(s string) string {
			return strings.Replace(s, "if (!selectionHoldsScripture(text.getText(), s0, s1)) {", "if (false) {", 1)
		}},
	} {
		var c studyGateCheck
		for _, k := range studyGateChecks {
			if k.name == m.check {
				c = k
			}
		}
		src := readNativeSource(t, c.file)
		bad := m.mutate(src)
		if bad == src {
			t.Fatalf("control %q: the mutation did not apply to %s", m.name, c.file)
		}
		if len(studyGateFaults(bad, c)) == 0 {
			t.Errorf("control %q: the check passes the mutated %s; it proves nothing", m.name, c.file)
		}
	}
}
