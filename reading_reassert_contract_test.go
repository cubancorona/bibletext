package bibletext

// A DEFERRED RE-ASSERT RE-PLACES; IT NEVER PINS THE VIEW TO THE TOP.
//
// The Apple panes re-assert the scroll position after an import settles (a turn
// later and 200ms later) and after a width change. A re-assert reads the arrival
// class when it RUNS, not when it was scheduled, and every render the reader did
// not ask for pushes "nothing" for that class — only the render a link asked for
// is explicit. When such a render landed between a link's import and its
// re-asserts (iOS flipping the appearance to snapshot the app for the switcher
// rebuilds the window; so does the push that follows a translation switch), the
// re-assert found nothing to place and pinned the view to the TOP, over the
// note the link had just placed it on. Seen on the simulator: "landed on
// highlight", then "arrival=nothing", then "pinned to TOP" twice. A link that
// opened the chapter and stayed at the top of it, once in several tries, is the
// report this answers; "nothing" was never meant to mean the top (arriveNothing,
// notes_arrival.go).
//
// The synchronous resolver keeps its top-pin — that is how a plain entry opens a
// new chapter — and the tests below hold the controls as well as the change.

import (
	"strings"
	"testing"
)

func TestDeferredReassertsNeverPinToTheTop(t *testing.T) {
	// iOS: the re-assert asks the restore, then the arrival, and nothing else.
	reassert := nativeFunctionSource(t, "reading_ios.go", "static void btIOSReassertPlacement(void) {")
	for _, want := range []string{"btIOSApplyRestore()", "btIOSScrollToHighlight()"} {
		if !strings.Contains(reassert, want) {
			t.Errorf("btIOSReassertPlacement does not call %s", want)
		}
	}
	// By what it does, not only by what it names: no path out of the re-assert
	// may move the view except the two placements it asks for.
	for _, bad := range []string{"bibleTextScrollReadingTV", "contentOffset", "scrollRangeToVisible", "scrollToPoint"} {
		if strings.Contains(strings.ToLower(reassert), strings.ToLower(bad)) {
			t.Errorf("btIOSReassertPlacement reaches %q — a re-assert with nothing to place must leave the view where it is", bad)
		}
	}
	// The restore moves the view only when it resolved a target.
	restore := nativeFunctionSource(t, "reading_ios.go", "static BOOL btIOSApplyRestore(void) {")
	if k := strings.Index(restore, "if (target >= 0) {"); k < 0 || strings.Count(restore, "contentOffset = ") != 1 ||
		strings.Index(restore, "contentOffset = ") < k {
		t.Error("btIOSApplyRestore must move the view once, and only inside its resolved-target branch")
	}

	// The import: the synchronous call keeps the top-pin (the control), and
	// everything scheduled after it is a re-assert.
	apply := nativeFunctionSource(t, "reading_ios.go", "static BOOL bibleTextApplyHTML(NSData *data) {")
	const gate = "if (gReadingHighlightRange.location != NSNotFound || gReadingHasRestore) {"
	i := strings.LastIndex(apply, gate)
	if i < 0 {
		t.Fatal("bibleTextApplyHTML no longer schedules its deferred re-asserts behind the highlight-or-restore gate")
	}
	if !strings.Contains(apply[:i], "bibleTextScrollReadingTV();") {
		t.Error("the import's synchronous placement no longer calls bibleTextScrollReadingTV — a plain entry would not open at the top")
	}
	deferred := apply[i:]
	if n := strings.Count(deferred, "btIOSReassertPlacement();"); n != 2 {
		t.Errorf("the import schedules %d re-asserts through btIOSReassertPlacement, want 2", n)
	}
	if strings.Contains(deferred, "bibleTextScrollReadingTV();") {
		t.Error("a deferred import re-assert calls bibleTextScrollReadingTV, which pins the view to the top when the class has gone to nothing")
	}

	// The width change: both of its re-asserts.
	frame := nativeFunctionSource(t, "reading_ios.go", "void bibleTextTVSetFrame(float x, float y, float w, float h) {")
	if strings.Contains(frame, "bibleTextScrollReadingTV();") {
		t.Error("a width change re-asserts through bibleTextScrollReadingTV, which throws a reader who scrolled away from a lit wash to the top")
	}
	if n := strings.Count(frame, "btIOSReassertPlacement();"); n != 2 {
		t.Errorf("the width change re-asserts %d times through btIOSReassertPlacement, want 2", n)
	}

	// The control: the synchronous resolver still pins, or a plain entry into a
	// new chapter would keep the previous chapter's offset.
	resolver := nativeFunctionSource(t, "reading_ios.go", "static void bibleTextScrollReadingTV(void) {")
	if !strings.Contains(resolver, "contentOffset = CGPointMake(0, -gReadingTV.adjustedContentInset.top)") {
		t.Error("bibleTextScrollReadingTV no longer pins to the top when nothing is placed")
	}

	// macOS has one re-assert, the frame change.
	macFrame := nativeFunctionSource(t, "reading_macos.go", "static void btMacApplyFrame(double x, double y, double w, double h) {")
	if !strings.Contains(macFrame, "btMacReassertPlacement();") || strings.Contains(macFrame, "bibleTextMacScrollTV();") {
		t.Error("the macOS frame change must re-assert through btMacReassertPlacement, not the pinning resolver")
	}
	macReassert := nativeFunctionSource(t, "reading_macos.go", "static void btMacReassertPlacement(void) {")
	if !strings.Contains(macReassert, "btMacScrollTVLatched(NO);") {
		t.Error("btMacReassertPlacement must ask the resolver not to pin")
	}
	latched := nativeFunctionSource(t, "reading_macos.go", "static void btMacScrollTVLatched(BOOL pinTop) {")
	landed := strings.Index(latched, "if (btMacScrollToHighlight())")
	stop, pin := strings.Index(latched, "if (!pinTop) {"), strings.Index(latched, "pinned to TOP")
	if landed < 0 || stop < 0 || pin < 0 || !(landed < stop && stop < pin) {
		t.Error("btMacScrollTVLatched must try the arrival, then return before the top-pin when pinTop is NO")
	} else if block := latched[stop:pin]; !strings.Contains(block, "return;") {
		t.Error("the !pinTop branch of btMacScrollTVLatched no longer returns, so the frame re-assert pins to the top again")
	}
	pinning := nativeFunctionSource(t, "reading_macos.go", "static void bibleTextMacScrollTV(void) {")
	if !strings.Contains(pinning, "btMacScrollTVLatched(YES);") {
		t.Error("bibleTextMacScrollTV no longer pins — a plain entry would keep the previous chapter's offset")
	}
}
