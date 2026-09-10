package bibletext

// THE PLAIN-TEXT FALLBACK'S CONTRACT on the native panes, pinned at the
// source: the host cannot execute either cgo preamble, so the contract is held
// the way the house holds every native contract
// (readalong_title_native_contract_test.go, reading_native_scroll_contract_test.go)
// — by parsing the source, with each check carrying a control that proves it
// can fail.
//
// The latched import derives every cached range from the storage it has just
// installed, each after the one it depends on: the content boundaries; the
// title range, bounded by the content start; the highlight union, which
// resolves verse 0 through the title range; and the note, whose anchor falls
// back to the highlight. A failed import returns before any of that, and the
// fallback that then replaces the text is the ONE path where those
// derivations are skipped — so each range has to be re-derived (or cleared)
// there, in the import's own order, or the previous chapter's value survives
// under the plain text. Every reader clamps on length alone, and the
// tag-stripped string is longer than the styled one it replaces, so a stale
// range passes the clamp: the read-along painted verse 0 over the wrong text,
// a resize or a reposition scrolled to the old highlight and anchored the
// note card on it, and the pills were placed from paragraph ranges of a
// storage that no longer existed.

import (
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

// The fallback's steps on each pane, in the order the import derives them:
// the new text, then the content boundaries, the title range, the highlight
// union, the note — and on iOS the resume marker, the one cached run a scroll
// writes BACK onto the storage, forgotten as the import forgets it.
var (
	macFallbackSteps = []string{
		"[gTextView setString:bibleTextMacPlainFromHTML(plainSrc)];",
		"gMacContentEnd = gTextView.textStorage.length;",
		"gMacContentStart = 0;",
		"btMacFindTitleRange(nil);",
		"btMacRefreshHighlightRange(gTextView.textStorage);",
		"btMacRefreshNote();",
	}
	iosFallbackSteps = []string{
		"gReadingTV.text = bibleTextPlainFromHTML(plainSrc);",
		"gContentEnd = gReadingTV.textStorage.length;",
		"btIOSBuildVerseIndex(nil);",
		"btIOSFindTitleRange(nil);",
		"btIOSRefreshHighlightRange(gReadingTV.textStorage);",
		"btIOSRefreshNote();",
		"gHasLastTouch = NO;",
		"gMarkerApplied = NO;",
	}
)

// F1: each fallback re-derives (or clears) every cached range, in the import's
// order, after the text that invalidated them is in place.
func TestNativeFallbacksRederiveEveryCachedRangeInImportOrder(t *testing.T) {
	for _, tc := range []struct {
		path, fn string
		steps    []string
	}{
		{"reading_macos.go", "void bibleTextMacTVSetHTML(const char *html) {", macFallbackSteps},
		{"reading_ios.go", "void bibleTextTVSetHTML(const char *html) {", iosFallbackSteps},
	} {
		body := nativeFunctionSource(t, tc.path, tc.fn)
		if !inSequence(body, tc.steps...) {
			t.Errorf("%s: the plain-text fallback does not run, in this order:\n  %s", tc.path, strings.Join(tc.steps, "\n  "))
		}
		// CONTROL: taking any one step out fails the check, so every step is
		// held — not only the first and last.
		for _, s := range tc.steps {
			if inSequence(strings.Replace(body, s, "", 1), tc.steps...) {
				t.Fatalf("%s: the sequence check still passes with %q removed; it proves nothing", tc.path, s)
			}
		}
		// CONTROL: exchanging any two neighbours fails the check, so the ORDER
		// is held, not only the presence.
		for i := 1; i < len(tc.steps); i++ {
			if inSequence(swapOnce(body, tc.steps[i-1], tc.steps[i]), tc.steps...) {
				t.Fatalf("%s: the sequence check still passes with %q and %q exchanged; it proves nothing", tc.path, tc.steps[i-1], tc.steps[i])
			}
		}
	}
}

// F2: the fallback's order is the IMPORT's order, on both panes — the
// reference the fallback mirrors, not an order of its own. The iOS import
// inlines the note's steps; its fallback reaches the same steps through
// btIOSRefreshNote, whose body is pinned to run them in the import's order.
func TestNativeImportsDeriveTheCachedRangesInTheOrderTheFallbacksMirror(t *testing.T) {
	latched := nativeFunctionSource(t, "reading_macos.go", "static BOOL btMacApplyHTMLLatched(NSData *data) {")
	macImport := []string{
		"[gTextView.textStorage setAttributedString:as];",
		"btMacFindContentEnd(gTextView.textStorage);",
		"btMacFindContentStart(gTextView.textStorage);",
		"btMacFindTitleRange(gTextView.textStorage);",
		"btMacRefreshHighlightRange(gTextView.textStorage);",
		"btMacRefreshNote();",
	}
	if !inSequence(latched, macImport...) {
		t.Errorf("macOS: the latched import does not derive, in this order:\n  %s", strings.Join(macImport, "\n  "))
	}
	apply := nativeFunctionSource(t, "reading_ios.go", "static BOOL bibleTextApplyHTML(NSData *data) {")
	iosImport := []string{
		"btIOSFindContentEnd(gReadingTV.textStorage);",
		"btIOSBuildVerseIndex(gReadingTV.textStorage);",
		"btIOSFindTitleRange(gReadingTV.textStorage);",
		"btIOSRefreshHighlightRange(gReadingTV.textStorage);",
		"btIOSInstallNote();",
		"btIOSLayoutPillViews();",
		"gHasLastTouch = NO;",
		"gMarkerApplied = NO;",
	}
	if !inSequence(apply, iosImport...) {
		t.Errorf("iOS: the import does not derive, in this order:\n  %s", strings.Join(iosImport, "\n  "))
	}
	refresh := nativeFunctionSource(t, "reading_ios.go", "static void btIOSRefreshNote(void) {")
	if !inSequence(refresh, "btIOSInstallNote();", "btIOSLayoutNote();", "btIOSLayoutPillViews();") {
		t.Error("iOS: btIOSRefreshNote must install the bands, then place the card, then the pills — the import's own order")
	}
	// CONTROL: a step out of place fails.
	if inSequence(swapOnce(latched, macImport[2], macImport[3]), macImport...) {
		t.Fatal("inSequence still passes with the macOS content start and title finders exchanged; the check proves nothing")
	}
}

// F3: the highlight union has ONE writer on each pane — the refresh that
// derives it from the model — and the fallback goes through that writer rather
// than assigning the range itself. A second writer is how a range and its
// model drift; the fallback's clear is a derivation that resolves to nothing,
// so the same code is right the day the fallback gains geometry.
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

// F4: the premise that makes the fallback's derivation resolve to nothing.
// Against the plain string every verse lookup has to fail, or the union would
// be built from whatever the lookup happened to accept: iOS resolves verses
// from the index alone, which the fallback has just emptied; macOS walks the
// font runs and skips every run at or above the largest font's threshold,
// which a uniform-font string never falls below.
func TestVerseLookupsFindNothingInPlainText(t *testing.T) {
	iosIndex := nativeFunctionSource(t, "reading_ios.go", "static void btIOSBuildVerseIndex(NSTextStorage *ts) {")
	if !strings.Contains(iosIndex, "if (ts == nil) { gVerseIndexCount = 0; return; }") {
		t.Error("iOS: btIOSBuildVerseIndex(nil) must empty the index")
	}
	for _, fn := range []string{
		"static NSUInteger btIOSLocForVerse(NSTextStorage *ts, NSInteger verse) {",
		"static NSRange btIOSReadAlongRange(NSTextStorage *ts, NSInteger verse) {",
	} {
		body := nativeFunctionSource(t, "reading_ios.go", fn)
		if !strings.Contains(body, "gVerseIndexCount") || strings.Contains(body, "enumerateAttribute") {
			t.Errorf("iOS: %s must resolve from the index alone", fn)
		}
	}
	macLoc := nativeFunctionSource(t, "reading_macos.go", "static NSUInteger btMacLocForVerse(NSTextStorage *ts, NSInteger verse) {")
	if !strings.Contains(macLoc, "((NSFont *)val).pointSize >= thr) return;") {
		t.Error("macOS: btMacLocForVerse must skip every run at or above the threshold")
	}
	// The premise, not the literal: the threshold is SOME fraction of the
	// largest font strictly inside (0, 1), so a uniform font is never below it.
	thr := nativeFunctionSource(t, "reading_macos.go", "static CGFloat btMacVerseFontThreshold(NSTextStorage *ts) {")
	fraction := regexp.MustCompile(`return maxSize > 0 \? maxSize \* ([0-9.]+) :`)
	m := fraction.FindStringSubmatch(thr)
	if m == nil {
		t.Fatal("macOS: the threshold must be the largest font scaled by a fraction")
	}
	if f, err := strconv.ParseFloat(m[1], 64); err != nil || f <= 0 || f >= 1 {
		t.Errorf("macOS: the threshold fraction %q must lie strictly between 0 and 1", m[1])
	}
	// CONTROLS: the skip is seen to be missing when it is taken out, and a
	// fraction that would let a uniform font through is seen for what it is.
	if strings.Contains(strings.Replace(macLoc, "((NSFont *)val).pointSize >= thr) return;", "", 1), "pointSize >= thr) return;") {
		t.Fatal("the skip check still passes with the skip removed; the check proves nothing")
	}
	if p := fraction.FindStringSubmatch("return maxSize > 0 ? maxSize * 1.0 : 15.0;"); p == nil || p[1] != "1.0" {
		t.Fatal("the fraction reader cannot read a planted fraction; the check proves nothing")
	} else if f, _ := strconv.ParseFloat(p[1], 64); !(f <= 0 || f >= 1) {
		t.Fatal("a fraction of 1.0 passed the bound; the check proves nothing")
	}
}
