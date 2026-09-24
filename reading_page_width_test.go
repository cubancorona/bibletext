package bibletext

import (
	"strings"
	"testing"
	"time"
)

// The native panes choose their page from the width they report
// (reading_page_width.go): a width that settles on the other page re-pushes the
// chapter once, and a drag that crosses the switch and comes back re-pushes
// nothing.
func TestReadingPaneWidthRePushesOnceAcrossTheSwitch(t *testing.T) {
	saveReadingPaneWidth(t)
	readingPageSettle = 20 * time.Millisecond
	fired := make(chan struct{}, 8)
	readingPageRun = func(f func()) { f(); fired <- struct{}{} }
	wait := func() {
		select {
		case <-fired:
		case <-time.After(time.Second):
			t.Fatal("the settle never fired")
		}
	}
	ref := readingReferencePx()
	wide := reporterMeasureEm*ref + 2*readingPageSideMin + 100
	narrow := wide - 200

	pushes := 0
	repush := func() { pushes++; markReadingPagePushed(currentReadingPage().Kind) }

	readingPaneWidth = 0
	noteReadingPaneWidth(wide, repush)
	markReadingPagePushed(readingPageBook)
	wait()
	if pushes != 0 {
		t.Fatalf("a width on the page already pushed re-pushed %d times", pushes)
	}

	// Across and back inside the settle: nothing.
	noteReadingPaneWidth(narrow, repush)
	noteReadingPaneWidth(wide, repush)
	wait()
	if pushes != 0 {
		t.Fatalf("a drag that came back re-pushed %d times", pushes)
	}

	// Across, and held: once.
	noteReadingPaneWidth(narrow, repush)
	wait()
	if pushes != 1 || readingPagePushed != readingPagePhone {
		t.Fatalf("a width held across the switch re-pushed %d times (pushed %v), want once at the phone page", pushes, readingPagePushed)
	}
	// The same width again is not news.
	noteReadingPaneWidth(narrow, repush)
	select {
	case <-fired:
		t.Fatal("an unchanged width armed a re-push")
	case <-time.After(60 * time.Millisecond):
	}
}

// saveReadingPaneWidth restores the width feed's state when the test ends.
func saveReadingPaneWidth(t *testing.T) {
	prevSettle, prevRun, prevW, prevWin := readingPageSettle, readingPageRun, readingPaneWidth, readingPaneWindow
	prevPushed, prevValid, prevEst := readingPagePushed, readingPagePushedValid, readingWindowWidth
	t.Cleanup(func() {
		readingPageSettle, readingPageRun, readingPaneWidth, readingPaneWindow = prevSettle, prevRun, prevW, prevWin
		readingPagePushed, readingPagePushedValid, readingWindowWidth = prevPushed, prevValid, prevEst
	})
}

// A rotation pushes the chapter into the new pane before the pane has
// reported, so the page is chosen from the last report moved by what the window
// has moved since — the rotated phone's first push is already the book page,
// and its own report, when it comes, changes nothing.
func TestReadingPaneWidthFollowsTheWindowUntilThePaneReports(t *testing.T) {
	saveReadingPaneWidth(t)
	readingPageSettle = time.Hour // no re-push fires inside this test
	window := 402.0
	readingWindowWidth = func() float64 { return window }
	readingPaneWidth, readingPaneWindow = 0, 0

	if got := readingPaneWidthNow(); got != 402 {
		t.Fatalf("before any report the window is the estimate: got %v", got)
	}
	// Portrait: the pane reports its width, a rail's worth narrower.
	noteReadingPaneWidth(372, func() {})
	if currentReadingPage().Book() {
		t.Fatal("a 372-wide pane took the book page")
	}
	// Landscape: the window has turned, the pane has not reported yet.
	window = 874
	if got := readingPaneWidthNow(); got != 844 {
		t.Fatalf("after the rotation the estimate is %v, want the report moved by the window's change (844)", got)
	}
	if !currentReadingPage().Book() {
		t.Fatal("the rotated pane's first push is not the book page")
	}
	// The report lands: it is the width from now on.
	noteReadingPaneWidth(750, func() {})
	if got := readingPaneWidthNow(); got != 750 {
		t.Fatalf("after the report the width is %v, want the report (750)", got)
	}
	// A platform with no window estimate (the Mac) reads its report as it stands.
	readingWindowWidth = func() float64 { return 0 }
	readingPaneWidth, readingPaneWindow = 0, 0
	noteReadingPaneWidth(640, func() {})
	if got := readingPaneWidthNow(); got != 640 {
		t.Fatalf("with no window estimate the width is %v, want the report (640)", got)
	}
}

// No surface chooses its page by device, idiom or orientation: each asks the
// spec, from its width.
func TestNoSurfaceChoosesThePageByDevice(t *testing.T) {
	for _, path := range []string{"reporter_ios.go", "reporter_macos.go", "reporter_android.go"} {
		src := readNativeSource(t, path)
		i := strings.Index(src, "func reporterLayoutActive() bool")
		if i < 0 {
			t.Fatalf("%s has no reporterLayoutActive", path)
		}
		body := src[i:]
		if j := strings.Index(body, "\n}"); j >= 0 && !strings.HasPrefix(body[strings.Index(body, "{"):], "{ return") {
			body = body[:j]
		} else if k := strings.Index(body, "\n"); k >= 0 {
			body = body[:k]
		}
		if !strings.Contains(body, "currentReadingPage()") {
			t.Errorf("%s: reporterLayoutActive does not ask the spec (%q)", path, body)
		}
		for _, bad := range []string{"deviceIsTablet", "phoneLandscape", "return true"} {
			if strings.Contains(body, bad) {
				t.Errorf("%s: reporterLayoutActive still reads %q", path, bad)
			}
		}
	}
}

// The Apple panes place the ink where the spec says: the inset is the ink side
// less the container's own padding, from the side minimum Go pushes, and the
// page push reads the spec's page and reports what it pushed.
func TestApplePanesTakeTheirPageFromTheSpec(t *testing.T) {
	for _, tc := range []struct{ path, insets, sideMin, pushSig, sidePush string }{
		{"reading_ios.go", "static void btIOSApplyInsets(CGFloat w) {", "gReadingSideMin", "func pushChapterHTML(state *AppState, verses []Verse) {", "C.bibleTextSetReadingSideMin("},
		{"reading_macos.go", "static BOOL btMacApplyInsets(CGFloat w) {", "gMacReadingSideMin", "func newMacReadingHost(", "C.bibleTextMacSetReadingSideMin("},
	} {
		insets := nativeFunctionSource(t, tc.path, tc.insets)
		for _, want := range []string{"CGFloat ink = " + tc.sideMin + ";", "if (ink < " + tc.sideMin + ") ink = " + tc.sideMin + ";", "lineFragmentPadding", "ink - pad"} {
			if !strings.Contains(insets, want) {
				t.Errorf("%s: the insets do not use %q", tc.path, want)
			}
		}
		for _, bad := range []string{"side = 10", "side = 16", "side < 12", "side < 16"} {
			if strings.Contains(insets, bad) {
				t.Errorf("%s: the insets still carry the literal %q", tc.path, bad)
			}
		}
		push := nativeFunctionSource(t, tc.path, tc.pushSig)
		for _, want := range []string{"currentReadingPage()", "markReadingPagePushed(page.Kind)", tc.sidePush, "page.PitchEm", "page.Measure"} {
			if !strings.Contains(push, want) {
				t.Errorf("%s: the page push does not read %q", tc.path, want)
			}
		}
	}
	// The macOS pane reports its width; iOS reports it with each frame.
	if !strings.Contains(nativeFunctionSource(t, "reading_macos.go", "static void btMacApplyFrame(double x, double y, double w, double h) {"), "btMacReadingWidthChanged(") {
		t.Error("the macOS frame change does not report the pane's width")
	}
	if !strings.Contains(nativeFunctionSource(t, "reading_ios.go", "func setFrameFromObject(h *nativeReadingHost) {"), "noteReadingPaneWidth(") {
		t.Error("the iOS frame push does not report the pane's width")
	}
}

// The Android pane takes its page from the spec by the same moves: the push
// reads the page and reports it, the side padding is the spec's minimum, the
// pitch and the book page's measure are the page's, and the bridge reports the
// overlay's width in dp on every change.
func TestAndroidPaneTakesItsPageFromTheSpec(t *testing.T) {
	push := nativeFunctionSource(t, "reading_android.go", "func pushChapterHTML(state *AppState, verses []Verse) {")
	for _, want := range []string{"page := currentReadingPage()", "markReadingPagePushed(page.Kind)",
		"padL, padT := int(readingPageSideMin), 14", "C.float(page.PitchEm)", "measureDp = float32(page.Measure)"} {
		if !strings.Contains(push, want) {
			t.Errorf("reading_android.go: the page push does not read %q", want)
		}
	}
	if strings.Contains(push, "padL, padT := 10") {
		t.Error("reading_android.go: the push still carries the 10dp side padding")
	}
	export := nativeFunctionSource(t, "reading_android_export.go", "func btaReadingWidthChanged(widthDp C.float) {")
	for _, want := range []string{"noteReadingPaneWidth(w,", "refreshReadingOnly()", "readingPaneUnit = k"} {
		if !strings.Contains(export, want) {
			t.Errorf("reading_android_export.go: the width report does not %q", want)
		}
	}
	java := readNativeSource(t, "android/BtBridge.java")
	i := strings.Index(java, "content.addOnLayoutChangeListener(")
	if i < 0 {
		t.Fatal("BtBridge.java: no content layout listener")
	}
	listener := java[i:]
	listener = listener[:strings.Index(listener, "scroll.addView(content")]
	for _, want := range []string{"if ((r - l) != (orr - ol))", "nativeReadingWidthChanged((r - l) / density)"} {
		if !strings.Contains(listener, want) {
			t.Errorf("BtBridge.java: the content width listener does not %q", want)
		}
	}
}
