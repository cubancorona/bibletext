package bibletext

// The fallback page's contract on the native panes, pinned at the source: the
// host cannot execute either cgo preamble, so the contract is held the way
// the house holds every native contract (readalong_title_native_contract_test.go,
// reading_native_scroll_contract_test.go), by reading the source, with each
// check carrying a control that proves it can fail.
//
// When every import of a chapter fails, each pane shows the page Go built
// from the same HTML (reading_apple_fallback.go). The page used to be set as
// untyped text, after which the pane re-derived, or cleared, each cached range
// in a list of its own that had to follow the import's order by hand, and
// nothing could find a verse on it: every verb sent no span. Now the pane sets
// the page in the faces the importer gives the same markup and installs it
// through the very function the import installs through, so every range is
// derived from it in the import's order, and the selection verbs read the
// same geometry from it as from an imported page. reading_native_fallback_page_test.go
// and reading_apple_fallback_test.go hold the page and what a selection on
// it is answered with.

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// inSequence reports whether every step occurs in body, each after the end of
// the one before it. Unlike a chain of inOrder calls, a step is searched only
// past the previous step's match, so one occurrence cannot satisfy two links.
func inSequence(body string, steps ...string) bool {
	pos := 0
	for _, s := range steps {
		i := strings.Index(body[pos:], s)
		if i < 0 {
			return false
		}
		pos += i + len(s)
	}
	return true
}

// swapOnce exchanges the first occurrence of a and b in body.
func swapOnce(body, a, b string) string {
	body = strings.Replace(body, a, "\x00", 1)
	body = strings.Replace(body, b, a, 1)
	return strings.Replace(body, "\x00", b, 1)
}

// withoutLineComments is src with every // comment taken out, string
// literals passed over, so a statement quoted in a comment is not counted as
// one.
func withoutLineComments(src string) string {
	var b strings.Builder
	inString := false
	for i := 0; i < len(src); i++ {
		c := src[i]
		switch {
		case inString:
			b.WriteByte(c)
			if c == '\\' && i+1 < len(src) {
				i++
				b.WriteByte(src[i])
			} else if c == '"' || c == '\n' {
				inString = false
			}
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			for i < len(src) && src[i] != '\n' {
				i++
			}
			b.WriteByte('\n')
		default:
			if c == '"' {
				inString = true
			}
			b.WriteByte(c)
		}
	}
	return b.String()
}

// appleFallbackPane names the functions each pane's import and fallback are
// made of, and the statements that join them.
type appleFallbackPane struct {
	path       string
	setHTML    string   // the push's entry, with the retry ladder
	apply      string   // one import attempt
	importHTML string   // the import proper
	install    string   // the one install, for both pages
	page       string   // Go's fallback page set in its faces
	font       string   // the face maker the page uses
	tryImport  string   // an attempt, as the ladder writes it
	importOK   []string // the attempt: import, refuse on nil, install
	fallback   string   // the ladder's last step
	textWrite  *regexp.Regexp
	derive     []string // the install's derivations, in the import's order
	menu       []string // where the selection verbs read the span and lead
}

var appleFallbackPanes = []appleFallbackPane{
	{
		path:       "reading_ios.go",
		setHTML:    "void bibleTextTVSetHTML(const char *html, const char *fallbackText, const int *fallbackRuns,",
		apply:      "static BOOL bibleTextApplyHTML(NSData *data) {",
		importHTML: "static NSAttributedString *btIOSImportHTML(NSData *data) {",
		install:    "static void btIOSInstallPage(NSAttributedString *as) {",
		page:       "static NSAttributedString *btIOSFallbackPage(NSString *text, NSData *runs, BTFallbackStyle st) {",
		font:       "btIOSFallbackFont",
		tryImport:  "if (bibleTextApplyHTML(data)) { gTVBook = book; gTVChapter = chapter; return; }",
		importOK:   []string{"NSAttributedString *as = btIOSImportHTML(data);", "if (as == nil) return NO;", "btIOSInstallPage(as);", "return YES;"},
		fallback:   "btIOSInstallPage(btIOSFallbackPage(fbText, fbRuns, fallbackStyle));",
		textWrite:  regexp.MustCompile(`gReadingTV\.(?:text|attributedText)\s*=[^=]|\[gReadingTV(?:\.textStorage)? (?:setAttributedString|replaceCharactersInRange|setText|setAttributedText):`),
		derive: []string{
			"gReadingTV.attributedText = as;",
			"btIOSFindContentEnd(gReadingTV.textStorage);",
			"btIOSBuildVerseIndex(gReadingTV.textStorage);",
			"btIOSFindTitleRange(gReadingTV.textStorage);",
			"gBodyGenApplied = gBodyGenPending;",
			"btIOSRefreshHighlightRange(gReadingTV.textStorage);",
			"btIOSInstallNote();",
			"btIOSLayoutPillViews();",
			"btIOSRebuildWash();",
			"gHasLastTouch = NO;",
			"gMarkerApplied = NO;",
			"btIOSApplyMarker();",
			"bibleTextScrollReadingTV();",
		},
		menu: []string{
			"NSUInteger contentStart = btIOSContentStart();",
			"int spanLo = (int)btIOSVerseAtIndex(textView.textStorage, range.location, &loLoc);",
			"int spanHi = (int)btIOSVerseAtIndex(textView.textStorage, NSMaxRange(range) - 1, NULL);",
			"? [[textView.text substringWithRange:NSMakeRange(loLoc, range.location - loLoc)] copy]",
			"bibleTextAIMenuTapped((char *)act.UTF8String, (char *)captured.UTF8String, spanLo, spanHi,",
			"(char *)lead.UTF8String, (char *)book.UTF8String, chapter);",
			"bibleTextStudyMenuTapped((char *)act.UTF8String, (char *)captured.UTF8String, spanLo, spanHi,",
			"(char *)lead.UTF8String, (char *)book.UTF8String, chapter);",
		},
	},
	{
		path:       "reading_macos.go",
		setHTML:    "void bibleTextMacTVSetHTML(const char *html, const char *fallbackText, const int *fallbackRuns,",
		apply:      "static BOOL bibleTextMacApplyHTML(NSData *data) {",
		importHTML: "static NSMutableAttributedString *btMacImportHTML(NSData *data) {",
		install:    "static void btMacInstallPageLatched(NSAttributedString *as) {",
		page:       "static NSAttributedString *btMacFallbackPage(NSString *text, NSData *runs, BTFallbackStyle st) {",
		font:       "btMacFallbackFont",
		tryImport:  "if (bibleTextMacApplyHTML(data)) { gMacTextBook = book; gMacTextChapter = chapter; return; }",
		importOK:   []string{"NSMutableAttributedString *as = btMacImportHTML(data);", "if (as == nil) return NO;", "btMacInstallPage(as);", "return YES;"},
		fallback:   "btMacInstallPage(btMacFallbackPage(fbText, fbRuns, fallbackStyle));",
		textWrite:  regexp.MustCompile(`\[gTextView(?:\.textStorage)? (?:setString|setAttributedString|replaceCharactersInRange|insertText):|gTextView\.string\s*=[^=]`),
		derive: []string{
			"[gTextView.textStorage setAttributedString:as];",
			"btMacFindContentEnd(gTextView.textStorage);",
			"btMacFindContentStart(gTextView.textStorage);",
			"btMacFindTitleRange(gTextView.textStorage);",
			"gBodyGenApplied = gBodyGenPending;",
			"btMacRefreshHighlightRange(gTextView.textStorage);",
			"btMacRefreshNote();",
			"bibleTextMacScrollTV();",
		},
		menu: []string{
			"btMacVerseSpanForRange(self.textStorage, sel, &lo, &hi, &loLoc);",
			"NSString *lead = btMacSelectionLead(self.textStorage, sel, lo, loLoc);",
			"bibleTextStudyMenuTapped((char *)action, (char *)text.UTF8String, lo, hi, heading, (char *)lead.UTF8String,",
			"(char *)gMacTextBook.UTF8String, gMacTextChapter);",
		},
	},
}

// F1: the fallback installs Go's page, set in its faces, through the import's
// own install, as the ladder's last step; it copies the page before the push
// returns; and nothing on the way writes the text view's text. The text is
// written in one place in the whole pane, the install.
func TestTheFallbackInstallsGosPageAsTheImportInstallsItsOwn(t *testing.T) {
	for _, p := range appleFallbackPanes {
		set := nativeFunctionSource(t, p.path, p.setHTML)
		ladder := []string{p.tryImport, p.tryImport, p.tryImport, p.fallback}
		if !inSequence(set, ladder...) {
			t.Errorf("%s: the push does not try the import three times and then install the fallback page:\n  %s", p.path, strings.Join(ladder, "\n  "))
		}
		copies := []string{
			"NSString *fbText = fallbackText != NULL ? [NSString stringWithUTF8String:fallbackText] : nil;",
			"[NSData dataWithBytes:fallbackRuns length:(NSUInteger)fallbackRunCount * 3 * sizeof(int)]",
			"dispatch_async(dispatch_get_main_queue(), ^{",
		}
		if !inSequence(set, copies...) {
			t.Errorf("%s: the push does not copy the fallback page before it leaves the caller's thread", p.path)
		}
		if n := len(p.textWrite.FindAllString(withoutLineComments(set), -1)); n != 0 {
			t.Errorf("%s: the push writes the text view's text %d time(s) itself", p.path, n)
		}
		src := withoutLineComments(readNativeSource(t, p.path))
		install := withoutLineComments(nativeFunctionSource(t, p.path, p.install))
		if n, m := len(p.textWrite.FindAllString(src, -1)), len(p.textWrite.FindAllString(install, -1)); n != 1 || m != 1 {
			t.Errorf("%s: the text view's text is written %d time(s), %d of them in the install; want once, there", p.path, n, m)
		}
		// The Mac's wash is an attribute the importer paints from the
		// stylesheet, and the fallback page has none: the install must paint
		// it from the model. iOS draws it from the model on every install.
		if p.path == "reading_macos.go" {
			unpainted := []string{"gTintUnpainted = YES;", p.fallback}
			if !inSequence(set, unpainted...) {
				t.Errorf("%s: the fallback page is not marked unpainted before the install, so the chapter's wash is not on it", p.path)
			}
			install := nativeFunctionSource(t, p.path, p.install)
			if !inSequence(install, "gBodyGenApplied = gBodyGenPending;", "if (gTintUnpainted) { btMacRepaintChapterWashFromModel(); gTintUnpainted = NO; }") {
				t.Errorf("%s: the install does not paint an unpainted wash from the model once the chapter is confirmed", p.path)
			}
			if inSequence(strings.Replace(set, "gTintUnpainted = YES;", "", 1), unpainted...) {
				t.Fatalf("%s: the wash check passes with the mark taken out; it proves nothing", p.path)
			}
		}
		// CONTROLS: the ladder without its fallback, the copies after the
		// dispatch, and a second write of the text are each seen.
		if inSequence(strings.Replace(set, p.fallback, "", 1), ladder...) {
			t.Fatalf("%s: the ladder check passes with the fallback taken out; it proves nothing", p.path)
		}
		if inSequence(swapOnce(set, copies[0], copies[2]), copies...) {
			t.Fatalf("%s: the copy check passes with the copy after the dispatch; it proves nothing", p.path)
		}
		planted := strings.Replace(src, p.fallback, p.fallback+"\n"+"[gTextView setString:@\"\"]; gReadingTV.text = @\"\";", 1)
		if len(p.textWrite.FindAllString(planted, -1)) == 1 {
			t.Fatalf("%s: the write counter misses a planted write; it proves nothing", p.path)
		}
	}
}

// F2: an import attempt installs through the same function, after the
// import succeeds; and that install derives every cached range in the
// import's order: the text, the content boundaries, the title, the
// generation that confirms the chapter, the highlight union, the note, the
// wash and the resume marker, then the placement.
func TestTheInstallDerivesTheCachedRangesInTheImportsOrder(t *testing.T) {
	for _, p := range appleFallbackPanes {
		apply := nativeFunctionSource(t, p.path, p.apply)
		if !inSequence(apply, p.importOK...) {
			t.Errorf("%s: an import attempt does not import, refuse a failure and install, in that order:\n  %s", p.path, strings.Join(p.importOK, "\n  "))
		}
		install := nativeFunctionSource(t, p.path, p.install)
		if !inSequence(install, p.derive...) {
			t.Errorf("%s: the install does not derive, in this order:\n  %s", p.path, strings.Join(p.derive, "\n  "))
		}
		// CONTROLS: any step taken out, or two neighbours exchanged, fails.
		for _, s := range p.derive {
			if inSequence(strings.Replace(install, s, "", 1), p.derive...) {
				t.Fatalf("%s: the sequence check still passes with %q removed; it proves nothing", p.path, s)
			}
		}
		for i := 1; i < len(p.derive); i++ {
			if inSequence(swapOnce(install, p.derive[i-1], p.derive[i]), p.derive...) {
				t.Fatalf("%s: the sequence check still passes with %q and %q exchanged; it proves nothing", p.path, p.derive[i-1], p.derive[i])
			}
		}
	}
	refresh := nativeFunctionSource(t, "reading_ios.go", "static void btIOSRefreshNote(void) {")
	if !inSequence(refresh, "btIOSInstallNote();", "btIOSLayoutNote();", "btIOSLayoutPillViews();") {
		t.Error("iOS: btIOSRefreshNote must install the bands, then place the card, then the pills — the install's own order")
	}
}

// F3: the highlight union has ONE writer on each pane — the refresh that
// derives it from the model — and the install goes through that writer
// rather than assigning the range itself. A second writer is how a range and
// its model drift.
func TestHighlightRangeHasOneWriterOnEachPane(t *testing.T) {
	for _, tc := range []struct{ path, global, refresh string }{
		{"reading_macos.go", "gMacHighlightRange", "static void btMacRefreshHighlightRange(NSTextStorage *ts) {"},
		{"reading_ios.go", "gReadingHighlightRange", "static void btIOSRefreshHighlightRange(NSTextStorage *ts) {"},
	} {
		src := readNativeSource(t, tc.path)
		// Every assignment anywhere in the file, line-leading or not, and never
		// a comparison: the declaration's initialiser and the refresh's write
		// are the two, and nothing may take the address to write through.
		writes := regexp.MustCompile(`\b` + tc.global + `\s*=[^=]`)
		if n := len(writes.FindAllString(src, -1)); n != 2 {
			t.Errorf("%s: %s is assigned %d time(s); want the declaration's initialiser and the refresh's write", tc.path, tc.global, n)
		}
		if !strings.Contains(src, "static NSRange "+tc.global+" = {NSNotFound, 0};") {
			t.Errorf("%s: %s must be declared as not-found", tc.path, tc.global)
		}
		if strings.Contains(src, "&"+tc.global) {
			t.Errorf("%s: something takes the address of %s; the refresh must stay its only writer", tc.path, tc.global)
		}
		if !strings.Contains(nativeFunctionSource(t, tc.path, tc.refresh), tc.global+" = u;") {
			t.Errorf("%s: the one write to %s is not the refresh's", tc.path, tc.global)
		}
		// CONTROLS: a planted assignment is counted wherever it sits on the
		// line, and a planted comparison is not.
		if n := len(writes.FindAllString(src+"\nfoo(); "+tc.global+" = NSMakeRange(NSNotFound, 0);\n", -1)); n != 3 {
			t.Fatalf("%s: the counter missed a planted assignment (%d); it proves nothing", tc.path, n)
		}
		if n := len(writes.FindAllString(src+"\n    if ("+tc.global+" == x) {}\n", -1)); n != 2 {
			t.Fatalf("%s: the counter took a planted comparison for a write (%d); it proves nothing", tc.path, n)
		}
	}
}

// fractionOf reads the number a pattern's first group captures in src.
func fractionOf(t *testing.T, src, pattern, what string) float64 {
	t.Helper()
	m := regexp.MustCompile(pattern).FindStringSubmatch(src)
	if m == nil {
		t.Fatalf("%s is not written as this test reads it", what)
	}
	f, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}
	return f
}

// F4: the page is set in the faces the walks read. The run kinds cross as
// numbers and are equal on both sides; a verse number is set at the
// numeral's size, bold, a title italic and a heading bold, at the body's
// size, and the body face is the page's largest; and the numeral's size, as
// Go sends it, is below the fraction of the largest size under which each
// pane takes a run for a verse number, and outside the band by which it
// finds the translators' notes, so the page ends where its text does.
func TestTheFallbackPageIsSetInTheFacesTheWalksRead(t *testing.T) {
	kinds := regexp.MustCompile(`enum \{ kBTFallbackNumeral = (\d+), kBTFallbackTitle = (\d+), kBTFallbackHeading = (\d+) \};`)
	st := appleFallbackStyleFor(&AppState{})
	ratio := st.numeralPt / st.bodyPt
	for _, p := range appleFallbackPanes {
		src := readNativeSource(t, p.path)
		m := kinds.FindStringSubmatch(src)
		if m == nil {
			t.Fatalf("%s: the fallback run kinds are not declared as this test reads them", p.path)
		}
		for i, want := range []appleFallbackRunKind{appleFallbackNumeral, appleFallbackTitle, appleFallbackHeading} {
			if m[1+i] != strconv.Itoa(int(want)) {
				t.Errorf("%s: kind %d is %s in C and %d in Go", p.path, i, m[1+i], want)
			}
		}
		page := nativeFunctionSource(t, p.path, p.page)
		faces := []string{
			"NSFontAttributeName: " + p.font + "(st.bodyPt, 0),",
			"*numeral = " + p.font + "(st.numeralPt, kCTFontTraitBold);",
			"*italic = " + p.font + "(st.bodyPt, kCTFontTraitItalic);",
			"*bold = " + p.font + "(st.bodyPt, kCTFontTraitBold);",
			"case kBTFallbackNumeral:", "[as addAttribute:NSFontAttributeName value:numeral range:rr];",
			"case kBTFallbackTitle:", "[as addAttribute:NSFontAttributeName value:italic range:rr];",
			"case kBTFallbackHeading:", "[as addAttribute:NSFontAttributeName value:bold range:rr];",
		}
		if !inSequence(page, faces...) {
			t.Errorf("%s: the fallback page does not set its runs in these faces, in this order:\n  %s", p.path, strings.Join(faces, "\n  "))
		}
		if inSequence(swapOnce(page, "value:italic range:rr", "value:bold range:rr"), faces...) {
			t.Fatalf("%s: the face check passes with the title set bold and the heading italic; it proves nothing", p.path)
		}
		prefix := "btIOS"
		if p.path == "reading_macos.go" {
			prefix = "btMac"
		}
		thr := fractionOf(t, nativeFunctionSource(t, p.path, "static CGFloat "+prefix+"VerseFontThreshold(NSTextStorage *ts) {"),
			`return maxSize > 0 \? maxSize \* ([0-9.]+) :`, p.path+": the verse-number threshold")
		end := nativeFunctionSource(t, p.path, "static void "+prefix+"FindContentEnd(NSTextStorage *ts) {")
		lo := fractionOf(t, end, `p >= maxSize \* ([0-9.]+) && p < maxSize \* [0-9.]+`, p.path+": the notes' band")
		hi := fractionOf(t, end, `p >= maxSize \* [0-9.]+ && p < maxSize \* ([0-9.]+)`, p.path+": the notes' band")
		if !(ratio > 0 && ratio < thr) || (ratio >= lo && ratio < hi) || thr <= 0 || thr >= 1 {
			t.Errorf("%s: a number at %.3f of the body is not below the threshold %.2f and outside the notes' band [%.2f, %.2f)", p.path, ratio, thr, lo, hi)
		}
		// CONTROL: a number at the body's size would not be found.
		if 1.0 < thr {
			t.Fatalf("%s: a number at the body's size passes the threshold; the check proves nothing", p.path)
		}
	}
}

// F5: Go builds the fallback page from the very HTML it pushes, and hands
// it over with that HTML, so the two cannot be of different chapters.
func TestGoPushesTheFallbackPageOfTheHTMLItPushes(t *testing.T) {
	for _, tc := range []struct{ path, fn, call string }{
		{"reading_ios.go", "pushChapterHTML", "C.bibleTextTVSetHTML(c, fbText, fbRuns, C.int(len(fb.runs)), cFallbackStyle(appleFallbackStyleFor(state)), gen)"},
		{"reading_macos.go", "newMacReadingHost", "C.bibleTextMacTVSetHTML(c, fbText, fbRuns, C.int(len(fb.runs)), cFallbackStyle(appleFallbackStyleFor(state)), gen)"},
	} {
		body := funcBody(t, readSourceFile(t, tc.path), tc.fn)
		steps := []string{"html := buildChapterHTML(state, verses)", "c := C.CString(html)", "fb := appleFallbackPageOf(html)",
			"fbText := C.CString(fb.text)", "runs := fb.flatRuns()", "fbRuns = (*C.int)(unsafe.Pointer(&runs[0]))", tc.call}
		if !inSequence(body, steps...) {
			t.Errorf("%s: %s does not build the fallback page from the HTML it pushes and pass both:\n  %s", tc.path, tc.fn, strings.Join(steps, "\n  "))
		}
		if strings.Count(body, "buildChapterHTML(") != 1 {
			t.Errorf("%s: %s builds the chapter's HTML more than once", tc.path, tc.fn)
		}
		if inSequence(strings.Replace(body, "fb := appleFallbackPageOf(html)", "fb := appleFallbackPageOf(other)", 1), steps...) {
			t.Fatalf("%s: the check passes a page built from other HTML; it proves nothing", tc.path)
		}
	}
}

// F6: every import step and the fallback act only for the latest push. The
// install confirms the chapter (gBodyGenApplied), so a step left over from a
// push that a later one has replaced must not put the older chapter in. The
// generation each step checks is the one the push's announcement issued and
// Go handed over with the push. Read on the main queue it is shared by two
// pushes made in one main-thread turn, where the announcement moves it at
// once and the push's blocks run later, so a retry of the first could install
// the older chapter over the second; that is the shape the controls plant.
func TestEveryImportStepActsOnlyForTheLatestPush(t *testing.T) {
	for _, tc := range []struct {
		pane              appleFallbackPane
		begin, goFn, goAt string
	}{
		{appleFallbackPanes[0], "int bibleTextIOSBeginChapterPush(int sameChapter) {", "pushChapterHTML", "gen := C.bibleTextIOSBeginChapterPush(sameChapter)"},
		{appleFallbackPanes[1], "int bibleTextMacBeginChapterPush(int sameChapter) {", "newMacReadingHost", "gen := C.bibleTextMacBeginChapterPush(sameChapter)"},
	} {
		p := tc.pane
		set := nativeFunctionSource(t, p.path, p.setHTML)
		const guard = "if (gen != gBodyGenPending) return;"
		steps := []string{"BTFallbackStyle fallbackStyle, int gen) {", guard, p.tryImport, guard, p.tryImport, guard, p.tryImport, p.fallback}
		// The generation is the push's own: nothing in the push reads the
		// pending one into it.
		readsPending := regexp.MustCompile(`\bgen\s*=\s*gBodyGenPending\b`)
		check := func(set string) error {
			if !inSequence(set, steps...) {
				return fmt.Errorf("the steps are not each behind the push's generation:\n  %s", strings.Join(steps, "\n  "))
			}
			if readsPending.MatchString(withoutLineComments(set)) {
				return fmt.Errorf("the push reads the pending generation for its own")
			}
			return nil
		}
		if err := check(set); err != nil {
			t.Errorf("%s: %v", p.path, err)
		}
		// The announcement issues the generation on the caller's thread,
		// moves the pending one only forward (an announcement made off the
		// main thread can reach the queue after a later one made on it), and
		// returns it.
		begin := nativeFunctionSource(t, p.path, tc.begin)
		issued := []string{"int gen = __atomic_add_fetch(&gBodyGenIssued, 1, __ATOMIC_SEQ_CST);",
			"if (gen <= gBodyGenPending) return;", "gBodyGenPending = gen;", "return gen;"}
		if !inSequence(begin, issued...) || strings.Contains(begin, "gBodyGenPending++") {
			t.Errorf("%s: the announcement does not issue the push's generation, move the pending one forward to it and return it:\n  %s", p.path, strings.Join(issued, "\n  "))
		}
		body := funcBody(t, readSourceFile(t, p.path), tc.goFn)
		if !inSequence(body, tc.goAt, "cFallbackStyle(appleFallbackStyleFor(state)), gen)") {
			t.Errorf("%s: %s does not hand the generation the announcement returned to the push", p.path, tc.goFn)
		}
		// CONTROLS: a guard taken out, the first attempt left unguarded with
		// the generation read on the main queue after it, as the ladder first
		// had it, and an announcement that only counts, are each seen.
		if check(strings.Replace(set, guard, "", 1)) == nil {
			t.Fatalf("%s: the check passes with a guard taken out; it proves nothing", p.path)
		}
		readInBlock := strings.Replace(strings.Replace(set, guard, "", 1), p.tryImport, p.tryImport+"\n        int gen = gBodyGenPending;\n        "+guard, 1)
		if check(readInBlock) == nil {
			t.Fatalf("%s: the check passes a generation read on the main queue; it proves nothing", p.path)
		}
		if inSequence(strings.Replace(begin, "gBodyGenPending = gen;", "gBodyGenPending++;", 1), issued...) {
			t.Fatalf("%s: the announcement check passes one that only counts; it proves nothing", p.path)
		}
		if inSequence(strings.Replace(begin, "if (gen <= gBodyGenPending) return;", "", 1), issued...) {
			t.Fatalf("%s: the announcement check passes one that can move the generation back; it proves nothing", p.path)
		}
	}
}

// F7: the selection verbs read the span and the lead from the geometry the
// install derives, the verse index on iOS and the numeral runs on the Mac,
// and hand both over, so a selection on the installed fallback page sends
// them as one on an imported page does.
func TestTheSelectionVerbsSendTheSpanAndLeadTheInstallGives(t *testing.T) {
	for _, p := range appleFallbackPanes {
		src := readNativeSource(t, p.path)
		if !inSequence(src, p.menu...) {
			t.Errorf("%s: the selection verbs do not read and send the span and lead as:\n  %s", p.path, strings.Join(p.menu, "\n  "))
		}
	}
	index := nativeFunctionSource(t, "reading_ios.go", "static NSInteger btIOSVerseAtIndex(NSTextStorage *ts, NSUInteger ci, NSUInteger *outLoc) {")
	if !strings.Contains(index, "gVerseIndex[mid].loc <= ci") {
		t.Error("iOS: btIOSVerseAtIndex must read the verse index the install builds")
	}
	span := nativeFunctionSource(t, "reading_macos.go", "static void btMacVerseSpanForRange(NSTextStorage *ts, NSRange sel, int *outLo, int *outHi, NSUInteger *outLoLoc) {")
	if !strings.Contains(span, "((NSFont *)val).pointSize >= thr) return;") || !strings.Contains(span, "btMacVerseFontThreshold(ts)") {
		t.Error("macOS: btMacVerseSpanForRange must find the numbers by the size the fallback page sets them at")
	}
	if inSequence(swapOnce(readNativeSource(t, "reading_ios.go"), appleFallbackPanes[0].menu[1], appleFallbackPanes[0].menu[2]), appleFallbackPanes[0].menu...) {
		t.Fatal("the menu check passes with the span's two ends exchanged; it proves nothing")
	}
}

// F8: the development switch only makes an import fail. The import refuses
// first thing while it is on; the setter is its only writer; and Go calls the
// setter only when a development build was asked to
// (devFailImportsRequested, false in every release build,
// TestDevFailImportIsInertInReleaseBuilds).
func TestTheFailImportSwitchOnlyRefusesTheImport(t *testing.T) {
	for _, tc := range []struct{ path, global, importHTML, setter, goCall, goFn string }{
		{"reading_ios.go", "gBTFailImports", appleFallbackPanes[0].importHTML, "bibleTextIOSFailImports", "C.bibleTextIOSFailImports(1)", "pushChapterHTML"},
		{"reading_macos.go", "gBTMacFailImports", appleFallbackPanes[1].importHTML, "bibleTextMacFailImports", "C.bibleTextMacFailImports(1)", "newMacReadingHost"},
	} {
		src := readNativeSource(t, tc.path)
		imp := nativeFunctionSource(t, tc.path, tc.importHTML)
		refusal := "if (" + tc.global + ") return nil;"
		refusesFirst := func(imp string) bool {
			return strings.HasPrefix(strings.TrimSpace(imp[strings.Index(imp, "{")+1:]), refusal)
		}
		if !refusesFirst(imp) {
			t.Errorf("%s: the import does not refuse first thing while the switch is on", tc.path)
		}
		// CONTROL: the refusal moved below the import is seen.
		if refusesFirst(strings.Replace(imp, refusal, "", 1)) {
			t.Fatalf("%s: the refusal check passes with the refusal taken out; it proves nothing", tc.path)
		}
		writes := regexp.MustCompile(`\b` + tc.global + `\s*=[^=]`)
		if got := writes.FindAllString(src, -1); len(got) != 2 || !strings.Contains(src, "static int "+tc.global+" = 0;") ||
			!strings.Contains(src, "^{ "+tc.global+" = on; }") {
			t.Errorf("%s: %s is written %d time(s); want its declaration and the setter's", tc.path, tc.global, len(got))
		}
		goSrc := readSourceFile(t, tc.path)
		if strings.Count(goSrc, tc.goCall) != 1 || !strings.Contains(funcBody(t, goSrc, tc.goFn), "if devFailImportsRequested() {\n\t\t"+tc.goCall) &&
			!strings.Contains(funcBody(t, goSrc, tc.goFn), "if devFailImportsRequested() {\n\t\t\t"+tc.goCall) {
			t.Errorf("%s: %s is not called only behind devFailImportsRequested", tc.path, tc.setter)
		}
	}
}
