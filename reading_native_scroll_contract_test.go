package bibletext

// THE NATIVE PANES' RESTORE CONTRACT, pinned at the source. Both are cgo
// preambles the host cannot execute, so what a test can hold is the shape of
// the source: that each pane tells Go when the reader scrolls, and that the
// macOS pane's restore is ended by that and by nothing else.
//
// The macOS pane had no scroll hook at all. Its consequence was a launch
// restore that was never consumed on the Go side, so every same-chapter
// rebuild — an appearance flip, a text-size change — re-armed the launch
// anchor and moved the reader back to it, however far they had read; and a
// native arm disarmed only by a changed frame, which a later window resize
// re-applied. Measured on the desktop app before and after.

import (
	"strings"
	"testing"
)

// Each native pane declares the Go export and calls it from its scroll hook:
// iOS from scrollViewDidScroll's drag/decelerate ends, macOS from the clip
// view's bounds observer, debounced to the end of the motion.
func TestEachNativePaneReportsTheReadersScroll(t *testing.T) {
	for _, path := range []string{"reading_ios.go", "reading_macos.go"} {
		src := readNativeSource(t, path)
		if !strings.Contains(src, "extern void bibleTextReadingScrolled(void);") {
			t.Errorf("%s does not declare bibleTextReadingScrolled", path)
		}
		if strings.Count(src, "bibleTextReadingScrolled();") == 0 {
			t.Errorf("%s never calls bibleTextReadingScrolled", path)
		}
	}
}

// The macOS restore ends on the reader's scroll and on a Go disarm, and
// nowhere else — in particular not on the first changed frame, whose one-shot
// re-applied a superseded width and then disarmed before the reader had done
// anything. (Definitions are matched with their opening brace: the helper is
// forward-declared near the top of the file, and the declaration would match
// first.)
func TestMacRestoreEndsOnlyOnTheReadersScroll(t *testing.T) {
	src := readNativeSource(t, "reading_macos.go")
	frame := nativeFunctionSource(t, "reading_macos.go", "static void btMacApplyFrame(")
	if strings.Contains(frame, "gMacHasRestore = NO") {
		t.Error("the frame path disarms the restore; a frame change must not end it")
	}
	disarm := nativeFunctionSource(t, "reading_macos.go", "static void btMacUserScrolled(void) {")
	if !strings.Contains(disarm, "gMacHasRestore = NO") || !strings.Contains(disarm, "bibleTextReadingScrolled();") {
		t.Error("btMacUserScrolled must disarm natively AND tell Go")
	}
	// Exactly two writers of NO: the user-scroll disarm and the Go disarm in
	// bibleTextMacArmRestore. A third is a new way for the restore to die.
	if got := strings.Count(src, "gMacHasRestore = NO"); got != 2 {
		t.Errorf("reading_macos.go writes gMacHasRestore = NO %d times, want 2 (the user scroll, the Go disarm)", got)
	}
}

// The hook is BOUNDS-shaped, not event-shaped: the clip view's bounds observer
// calls the disarm for any movement that is not one of the pane's own scrolls,
// which is what covers the wheel, the trackpad, the scroller, keyboard paging,
// a drag-select autoscroll and an accessibility scrollbar write alike. An
// event-shaped scrollWheel: override saw only the first two and, on the
// document view, switched off AppKit's responsive scrolling — so its absence
// is part of the contract. Every programmatic scroll runs under the latch.
func TestMacScrollHookIsBoundsShapedAndLatched(t *testing.T) {
	src := readNativeSource(t, "reading_macos.go")
	if strings.Contains(src, "- (void)scrollWheel:") {
		t.Error("reading_macos.go overrides scrollWheel:, which disables responsive scrolling and misses keyboard and scroller scrolls")
	}
	i := strings.Index(src, "addObserverForName:NSViewBoundsDidChangeNotification")
	if i < 0 {
		t.Fatal("no bounds observer on the clip view")
	}
	observer := src[i:]
	if j := strings.Index(observer, "}];"); j > 0 {
		observer = observer[:j]
	}
	if !strings.Contains(observer, "gMacOwnScroll == 0") || !strings.Contains(observer, "btMacUserScrolled();") {
		t.Error("the bounds observer must call btMacUserScrolled for a movement that is not the pane's own")
	}
	for _, fn := range []string{
		"static void bibleTextMacScrollTV(void) {",
		"static BOOL bibleTextMacApplyHTML(NSData *data) {",
		"void bibleTextMacTVSetFrame(double x, double y, double w, double h) {",
	} {
		body := nativeFunctionSource(t, "reading_macos.go", fn)
		if !strings.Contains(body, "gMacOwnScroll++") || !strings.Contains(body, "gMacOwnScroll--") {
			t.Errorf("%s does not run under the own-scroll latch", fn)
		}
	}
	// Narration carrying the reader ends the restore as their own hand would.
	follow := nativeFunctionSource(t, "reading_macos.go", "void bibleTextMacHighlightVerse(")
	if !strings.Contains(follow, "btMacUserScrolled();") {
		t.Error("the read-along follow scroll does not end the restore")
	}
}

// The note chrome follows layout completion on macOS: the card and the pills
// are subviews placed from the layout, the text can move after they are
// placed, and the watcher is what re-places them — both of them, every pass.
func TestMacNoteChromeFollowsLayoutCompletion(t *testing.T) {
	src := readNativeSource(t, "reading_macos.go")
	if !strings.Contains(src, "tv.layoutManager.delegate = gMacLayoutWatcher") {
		t.Error("the reading text view's layout manager has no HBLayoutWatcher delegate")
	}
	place := nativeFunctionSource(t, "reading_macos.go", "static void btMacPlaceNoteAfterLayout(void) {")
	if strings.Contains(place, "btMacInstallNote()") || strings.Contains(place, "btMacRefreshNote()") {
		t.Error("btMacPlaceNoteAfterLayout must only place, never install or refresh — an install from a layout callback is a layout loop")
	}
	for _, call := range []string{"btMacLayoutNote();", "btMacLayoutPillViews();"} {
		if !strings.Contains(place, call) {
			t.Errorf("btMacPlaceNoteAfterLayout does not call %s", call)
		}
	}
}
