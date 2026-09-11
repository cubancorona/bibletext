package bibletext

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// S18 on the native panes: the read-along's verse 0 is the Psalm's title.
//
// Nothing on this host can execute the Objective-C in reading_ios.go /
// reading_macos.go or the Java in android/BtBridge.java, so the contract is
// held the way the house holds every native contract
// (reading_native_scroll_contract_test.go, reading_ios_wash_guard_test.go): by
// parsing the source, with each check carrying a control that proves it can
// fail. The one premise the native finders rest on — the title is the first
// paragraph, italic, and a heading above verse 1 is bold — is pinned in the
// Go the host CAN run (TestTitleParagraphLeadsBothDialectsEvenUnderAHeading).

// nativeIntConst reads `name = <int>` from a native source (a `static const
// NSInteger` or a `private static final int`). Signed, unlike constValue in
// notes_spacing_spec_test.go, because the none value is negative.
func nativeIntConst(src, name string) (int, bool) {
	m := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\s*=\s*(-?[0-9]+)`).FindStringSubmatch(src)
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, false
	}
	return n, true
}

// inOrder reports whether a occurs in body and b occurs after it.
func inOrder(body, a, b string) bool {
	i := strings.Index(body, a)
	if i < 0 {
		return false
	}
	return strings.Index(body[i+len(a):], b) >= 0
}

// javaMethodSource is nativeFunctionSource for a method indented one level
// inside the class: its body ends at the first `\n    }\n`.
func javaMethodSource(t *testing.T, path, signature string) string {
	t.Helper()
	src := readNativeSource(t, path)
	start := strings.Index(src, signature)
	if start < 0 {
		t.Fatalf("%s is missing %s", path, signature)
	}
	end := strings.Index(src[start:], "\n    }\n")
	if end < 0 {
		t.Fatalf("%s has no stable method boundary for %s", path, signature)
	}
	return src[start : start+end]
}

// T1: the three files carry Go's own values, so the wire cannot drift.
func TestNativeReadAlongStatesAgreeWithGo(t *testing.T) {
	for _, tc := range []struct{ path, none, title string }{
		{"reading_ios.go", "kBTReadAlongNone", "kBTReadAlongTitle"},
		{"reading_macos.go", "kBTReadAlongNone", "kBTReadAlongTitle"},
		{"android/BtBridge.java", "RA_NONE", "RA_TITLE"},
	} {
		src := readNativeSource(t, tc.path)
		if got, ok := nativeIntConst(src, tc.none); !ok || got != readAlongNone {
			t.Errorf("%s: %s = %d,%v; Go's readAlongNone is %d", tc.path, tc.none, got, ok, readAlongNone)
		}
		if got, ok := nativeIntConst(src, tc.title); !ok || got != readAlongTitle {
			t.Errorf("%s: %s = %d,%v; Go's readAlongTitle is %d", tc.path, tc.title, got, ok, readAlongTitle)
		}
		// CONTROL: a name the file does not define is reported absent.
		if _, ok := nativeIntConst(src, "kBTReadAlongNope"); ok {
			t.Fatalf("%s: the reader found a constant that does not exist; it proves nothing", tc.path)
		}
	}
	if v, ok := nativeIntConst("static const NSInteger kBTReadAlongNone = -1;", "kBTReadAlongNone"); !ok || v != -1 {
		t.Fatalf("the reader cannot read a negative constant (%d,%v)", v, ok)
	}
}

// T2: each pane resolves verse 0 to the title range AHEAD of its verse lookup,
// and the read-along's two Android callers use the resolver that knows it.
func TestNativePanesResolveVerseZeroToTheTitle(t *testing.T) {
	ios := nativeFunctionSource(t, "reading_ios.go", "static NSRange btIOSReadAlongRange(NSTextStorage *ts, NSInteger verse) {")
	if !inOrder(ios, "if (verse == kBTReadAlongTitle) return btIOSTitleRange(ts);", "while (lo < hi)") {
		t.Error("iOS: the title branch must come before the binary search over the verse index")
	}
	mac := nativeFunctionSource(t, "reading_macos.go", "static NSRange btMacReadAlongRange(NSTextStorage *ts, NSInteger verse) {")
	if !inOrder(mac, "if (verse == kBTReadAlongTitle) return btMacTitleRange(ts);", "btMacLocForVerse(ts, verse)") {
		t.Error("macOS: the title branch must come before btMacLocForVerse, which would accept a non-numeric run for 0")
	}
	rng := javaMethodSource(t, "android/BtBridge.java", "private static int[] readAlongRange(int verse) {")
	if !strings.Contains(rng, "verse == RA_TITLE") || !strings.Contains(rng, "verseRange(verse)") {
		t.Error("Android: readAlongRange must resolve RA_TITLE itself and defer the rest to verseRange")
	}
	for _, sig := range []string{
		"public static void readAlongHighlight(final int verse, final boolean follow) {",
		"private static void followScrollTo(int verse) {",
	} {
		body := javaMethodSource(t, "android/BtBridge.java", sig)
		if !strings.Contains(body, "readAlongRange(") || strings.Contains(body, "verseRange(") {
			t.Errorf("Android: %s must resolve through readAlongRange, never verseRange directly", sig)
		}
	}
	// CONTROL: the order check fails when the branch is taken out.
	if inOrder(strings.Replace(ios, "if (verse == kBTReadAlongTitle) return btIOSTitleRange(ts);", "", 1),
		"if (verse == kBTReadAlongTitle) return btIOSTitleRange(ts);", "while (lo < hi)") {
		t.Fatal("inOrder still passes with the branch removed; the check proves nothing")
	}
}

// T3: the finders look for the italic paragraph before the content start, run
// right after the index that defines that start, and are cleared with it.
func TestNativePanesFindTheTitleAsTheItalicParagraphBeforeVerseOne(t *testing.T) {
	ios := nativeFunctionSource(t, "reading_ios.go", "static void btIOSFindTitleRange(NSTextStorage *ts) {")
	for _, want := range []string{"btIOSContentStart()", "paragraphRangeForRange:", "kCTFontTraitItalic", "NSMaxRange(para) > start"} {
		if !strings.Contains(ios, want) {
			t.Errorf("iOS finder lacks %q", want)
		}
	}
	apply := nativeFunctionSource(t, "reading_ios.go", "static BOOL bibleTextApplyHTML(NSData *data) {")
	if !inOrder(apply, "btIOSBuildVerseIndex(gReadingTV.textStorage);", "btIOSFindTitleRange(gReadingTV.textStorage);") {
		t.Error("iOS: the title range must be found after the verse index that bounds it")
	}
	iosFallback := nativeFunctionSource(t, "reading_ios.go", "void bibleTextTVSetHTML(const char *html) {")
	if !inOrder(iosFallback, "btIOSBuildVerseIndex(nil);", "btIOSFindTitleRange(nil);") {
		t.Error("iOS: the plain-text fallback must clear the title range with the verse index")
	}

	mac := nativeFunctionSource(t, "reading_macos.go", "static void btMacFindTitleRange(NSTextStorage *ts) {")
	for _, want := range []string{"gMacContentStart", "paragraphRangeForRange:", "kCTFontTraitItalic", "NSMaxRange(para) > start"} {
		if !strings.Contains(mac, want) {
			t.Errorf("macOS finder lacks %q", want)
		}
	}
	latched := nativeFunctionSource(t, "reading_macos.go", "static BOOL btMacApplyHTMLLatched(NSData *data) {")
	if !inOrder(latched, "btMacFindContentStart(gTextView.textStorage);", "btMacFindTitleRange(gTextView.textStorage);") {
		t.Error("macOS: the title range must be found after the content start that bounds it")
	}
	// The same property as the iOS fallback check above. The latched import
	// returns NO before writing any geometry global when the import fails, so
	// the fallback is the only place the previous chapter's title range can be
	// dropped — and btMacTitleRange clamps on length alone, which the LONGER
	// plain string passes. macOS has no verse table to clear beside it
	// (btMacLocForVerse walks the font runs live), so the reset rides on the
	// content-start reset that bounds the finder instead.
	macFallback := nativeFunctionSource(t, "reading_macos.go", "void bibleTextMacTVSetHTML(const char *html) {")
	if !inOrder(macFallback, "gMacContentStart = 0;", "btMacFindTitleRange(nil);") {
		t.Error("macOS: the plain-text fallback must clear the title range with the content start")
	}

	java := readNativeSource(t, "android/BtBridge.java")
	finder := javaMethodSource(t, "android/BtBridge.java", "private static void findTitleRange(CharSequence cs) {")
	for _, want := range []string{"contentStart <= 0", "StyleSpan.class", "Typeface.ITALIC", "getSpanStart(", "<= contentStart"} {
		if !strings.Contains(finder, want) {
			t.Errorf("Android finder lacks %q", want)
		}
	}
	if !inOrder(java, "buildVerseIndex(text.getText());", "findTitleRange(text.getText());") {
		t.Error("Android: setHtml must find the title range after the verse index")
	}
	if n := strings.Count(java, "titleStart = titleEnd = -1;"); n < 2 {
		t.Errorf("Android: the title span is reset %d time(s); want the finder's reset AND the activity-recreate reset", n)
	}
	// CONTROL: swapping the two calls fails the order check.
	swapped := strings.Replace(strings.Replace(strings.Replace(apply,
		"btIOSBuildVerseIndex(gReadingTV.textStorage);", "\x00", 1),
		"btIOSFindTitleRange(gReadingTV.textStorage);", "btIOSBuildVerseIndex(gReadingTV.textStorage);", 1),
		"\x00", "btIOSFindTitleRange(gReadingTV.textStorage);", 1)
	if inOrder(swapped, "btIOSBuildVerseIndex(gReadingTV.textStorage);", "btIOSFindTitleRange(gReadingTV.textStorage);") {
		t.Fatal("inOrder still passes with the calls swapped; the check proves nothing")
	}
	// CONTROL: both fallback checks fail when their reset is taken out.
	if inOrder(strings.Replace(iosFallback, "btIOSFindTitleRange(nil);", "", 1), "btIOSBuildVerseIndex(nil);", "btIOSFindTitleRange(nil);") {
		t.Fatal("inOrder still passes with the iOS fallback reset removed; the check proves nothing")
	}
	if inOrder(strings.Replace(macFallback, "btMacFindTitleRange(nil);", "", 1), "gMacContentStart = 0;", "btMacFindTitleRange(nil);") {
		t.Fatal("inOrder still passes with the macOS fallback reset removed; the check proves nothing")
	}
}

// T4: no pane treats 0 as "nothing painted" any more, and "active" is
// painted-ness rather than the argument.
func TestNativeReadAlongNoneIsNeverZero(t *testing.T) {
	count := func(src string, needles ...string) int {
		n := 0
		for _, s := range needles {
			n += strings.Count(src, s)
		}
		return n
	}
	for _, path := range []string{"reading_ios.go", "reading_macos.go"} {
		src := readNativeSource(t, path)
		if n := count(src, "gReadAlongVerse > 0", "gPendingReadAlongVerse > 0", "(verse > 0)"); n != 0 {
			t.Errorf("%s: %d zero-means-none comparison(s) remain", path, n)
		}
		// Statements only (indented): the static declaration is checked below,
		// and gPendingReadAlongVerse's tail would otherwise match too.
		if n := len(regexp.MustCompile(`(?m)^\s+gReadAlongVerse = kBTReadAlongNone;`).FindAllString(src, -1)); n != 3 {
			t.Errorf("%s: gReadAlongVerse is reset to none %d times; want 3 (clear, highlight, import)", path, n)
		}
		for _, want := range []string{
			"static NSInteger gReadAlongVerse = kBTReadAlongNone;",
			"static NSInteger gPendingReadAlongVerse = kBTReadAlongNone;",
		} {
			if !strings.Contains(src, want) {
				t.Errorf("%s lacks %q", path, want)
			}
		}
		// CONTROL: the counter sees a comparison when one is added back.
		if count(src+"\nif (gReadAlongVerse > 0) {", "gReadAlongVerse > 0") != 1 {
			t.Fatalf("%s: the counter missed a planted comparison; it proves nothing", path)
		}
	}
	iosHL := nativeFunctionSource(t, "reading_ios.go", "void bibleTextIOSHighlightVerse(int verse, int follow) {")
	macHL := nativeFunctionSource(t, "reading_macos.go", "void bibleTextMacHighlightVerse(int verse, int follow) {")
	for path, body := range map[string]string{"reading_ios.go": iosHL, "reading_macos.go": macHL} {
		if !strings.Contains(body, "gReadAlongActive = (gReadAlongVerse != kBTReadAlongNone);") {
			t.Errorf("%s: the highlight must set active from what was painted", path)
		}
	}
	iosClear := nativeFunctionSource(t, "reading_ios.go", "void bibleTextIOSReadAlongClear(void) {")
	macClear := nativeFunctionSource(t, "reading_macos.go", "void bibleTextMacReadAlongClear(void) {")
	for path, body := range map[string]string{"reading_ios.go": iosClear, "reading_macos.go": macClear} {
		if strings.Contains(body, "gReadAlongActive = (gReadAlongVerse != kBTReadAlongNone);") || !strings.Contains(body, "gReadAlongActive = NO;") {
			t.Errorf("%s: the clear must set active to NO outright", path)
		}
	}

	java := readNativeSource(t, "android/BtBridge.java")
	if n := count(java, "raVerse == 0", "raVerse = 0;", "(verse > 0)"); n != 0 {
		t.Errorf("Android: %d zero-means-none use(s) remain", n)
	}
	if n := len(regexp.MustCompile(`(?m)^\s+raVerse = RA_NONE;`).FindAllString(java, -1)); n != 4 {
		t.Errorf("Android: raVerse is reset to none %d times; want 4 (recreate, highlight, clear, setHtml)", n)
	}
	if !strings.Contains(java, "private static int raVerse = RA_NONE;") {
		t.Error("Android: raVerse must be declared as none")
	}
	hl := javaMethodSource(t, "android/BtBridge.java", "public static void readAlongHighlight(final int verse, final boolean follow) {")
	if !strings.Contains(hl, "raActive = (raVerse != RA_NONE);") {
		t.Error("Android: the highlight must set raActive from what was painted")
	}
}

// T5: the premise the three finders rest on, in the Go the host can run — the
// title is the FIRST paragraph of both dialects, italic, and a heading standing
// above verse 1 comes after it and is bold, never italic. CONTROL: with no
// title, the first paragraph is the heading and nothing italic precedes verse 1,
// which is exactly what makes verse 0 a no-op there.
func TestTitleParagraphLeadsBothDialectsEvenUnderAHeading(t *testing.T) {
	heading := map[string]map[int][]Heading{"John": {3: {{Text: "A heading probe", BeforeVerse: 16}}}}
	for _, reporter := range []bool{false, true} {
		withReporterLayout(reporter, func() {
			st := superFixtureState(footnoteFixtureVerses())
			st.Bible.Headings = heading
			verses := st.Bible.GetChapter("John", 3)

			apple := buildChapterHTML(st, verses)
			body := apple[strings.Index(apple, "<body>")+len("<body>"):]
			// The title's class may carry pre-sec — a title standing before a
			// heading carries the heading's lead (reading.go) — so match the
			// class's opening, not the whole attribute.
			if !strings.HasPrefix(body, `<p class="pst`) {
				t.Errorf("reporter=%v: the Apple body does not open with the title: %.80q", reporter, body)
			}
			if !inOrder(body, `<p class="pst`, `<p class="sec">A heading probe</p>`) ||
				!inOrder(body, `<p class="sec">A heading probe</p>`, `<sup class="v">16</sup>`) {
				t.Errorf("reporter=%v: Apple order is not title, heading, verse 1: %.200q", reporter, body)
			}
			css := apple[:strings.Index(apple, "<body>")]
			if strings.Count(css, "font-style: italic") != 1 {
				t.Errorf("reporter=%v: %d italic rules in the Apple stylesheet; want exactly the title's", reporter, strings.Count(css, "font-style: italic"))
			}
			pst := css[strings.Index(css, "p.pst {"):]
			pst = pst[:strings.Index(pst, "}")]
			if !strings.Contains(pst, "font-style: italic") {
				t.Errorf("reporter=%v: the p.pst rule is not italic: %q", reporter, pst)
			}
			sec := css[strings.Index(css, "p.sec {"):]
			sec = sec[:strings.Index(sec, "}")]
			if strings.Contains(sec, "italic") || !strings.Contains(sec, "font-weight: 700") {
				t.Errorf("reporter=%v: the p.sec rule must be bold and not italic: %q", reporter, sec)
			}

			android := buildChapterHTMLAndroid(st, verses)
			if !strings.HasPrefix(android, "<p><i>") {
				t.Errorf("reporter=%v: the Android page does not open with the italic title: %.80q", reporter, android)
			}
			firstSup := strings.Index(android, "<sup")
			if firstSup < 0 {
				t.Fatalf("reporter=%v: no verse number on the Android page", reporter)
			}
			if !inOrder(android[:firstSup], "</i>", "<p><b>A heading probe</b></p>") {
				t.Errorf("reporter=%v: Android order is not title, heading, verse 1: %.200q", reporter, android)
			}
			between := android[strings.Index(android, "</i>")+4 : firstSup]
			if strings.Contains(between, "<i>") {
				t.Errorf("reporter=%v: a second italic stands between the title and verse 1: %q", reporter, between)
			}

			// CONTROL: no title.
			plain := footnoteFixtureState(footnoteFixtureVerses())
			plain.Bible.Headings = heading
			apple = buildChapterHTML(plain, verses)
			body = apple[strings.Index(apple, "<body>")+len("<body>"):]
			if !strings.HasPrefix(body, `<p class="sec">`) || strings.Contains(apple, "font-style: italic") {
				t.Errorf("reporter=%v: without a title the Apple page must open with the bold heading and carry no italic rule: %.80q", reporter, body)
			}
			android = buildChapterHTMLAndroid(plain, verses)
			if !strings.HasPrefix(android, "<p><b>") || strings.Contains(android[:strings.Index(android, "<sup")], "<i>") {
				t.Errorf("reporter=%v: without a title the Android page must open with the bold heading and nothing italic before verse 1: %.80q", reporter, android)
			}
		})
	}
}

// The verse-index guard in verse_gaps_test.go still holds: nothing here moved
// the scans off integerValue / SuperscriptSpan.
func TestTitleRangeIsNotPartOfTheVerseIndex(t *testing.T) {
	for path, body := range map[string]string{
		"reading_ios.go":        nativeFunctionSource(t, "reading_ios.go", "static void btIOSBuildVerseIndex(NSTextStorage *ts) {"),
		"reading_macos.go":      nativeFunctionSource(t, "reading_macos.go", "static void btMacFindContentStart(NSTextStorage *ts) {"),
		"android/BtBridge.java": javaMethodSource(t, "android/BtBridge.java", "private static void buildVerseIndex(CharSequence cs) {"),
	} {
		if strings.Contains(body, "TitleRange") || strings.Contains(body, "titleStart") {
			t.Errorf("%s: the verse index consults the title range; the two must stay independent", path)
		}
	}
	if _, err := os.Stat("android/BtBridge.java"); err != nil {
		t.Fatal(err)
	}
}
