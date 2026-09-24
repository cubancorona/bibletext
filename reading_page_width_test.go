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
	prevSettle, prevRun, prevW := readingPageSettle, readingPageRun, readingPaneWidth
	prevPushed, prevValid := readingPagePushed, readingPagePushedValid
	t.Cleanup(func() {
		readingPageSettle, readingPageRun, readingPaneWidth = prevSettle, prevRun, prevW
		readingPagePushed, readingPagePushedValid = prevPushed, prevValid
	})
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

// No surface chooses its page by device, idiom or orientation: each asks the
// spec, from its width.
func TestNoSurfaceChoosesThePageByDevice(t *testing.T) {
	for _, path := range []string{"reporter_ios.go", "reporter_macos.go"} {
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
