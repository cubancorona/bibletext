package bibletext

// A SHARE THAT CANNOT FINISH SAYS SO ON IPHONE, IPAD AND ANDROID
// (share_failed.go). Neither bridge runs on the host, so both are held at
// the source, each check shown failing on a copy with the step it holds
// taken out. The desktop's are driven (share_failed_desktop_test.go).

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// iosPresentJudgesLate reports whether btIOSPresentTry's body presents from
// the top view controller once its transition has ended, counts the
// presentation taken when its completion runs with its view in a window, and
// judges it only btIOSPresentSettle seconds after presenting it: taken, it is
// let go; refused once, it is tried again; refused twice, it is let go and
// refused runs.
func iosPresentJudgesLate(body string) bool {
	return inSequence(body,
		"UIViewController *top = bibleTextTopVC();",
		"if (top == nil) {",
		"id<UIViewControllerTransitionCoordinator> tc = top.transitionCoordinator;",
		"if (tc != nil && waits > 0) {",
		"[tc animateAlongsideTransition:nil",
		"dispatch_async(dispatch_get_main_queue(), again);",
		"return;",
		"UIViewController *vc = p.make(top);",
		"[p.tried addObject:vc];",
		"[top presentViewController:vc animated:YES completion:^{",
		"if (shown.viewIfLoaded.window != nil) {",
		"p.taken = YES;",
		"btIOSPresentLetGo(p);",
		"dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(btIOSPresentSettle * NSEC_PER_SEC)),",
		"if (btIOSPresentTaken(p)) {",
		"btIOSPresentLetGo(p);",
		"return;",
		"if (p.tried.count < 2) {",
		"btIOSPresentTry(p, btIOSPresentWaits);",
		"return;",
		"btIOSPresentLetGo(p);",
		"if (p.refused != nil) p.refused();",
	)
}

// iosPresentTakenLooksEverywhere reports whether btIOSPresentTaken's body
// counts a presentation taken by its completion, and, for every view
// controller presented for it, by its presenter, its view in a window or its
// place among the view controllers presented from the window's root.
func iosPresentTakenLooksEverywhere(body string) bool {
	return inSequence(body,
		"if (p.taken) return YES;",
		"for (UIViewController *vc in p.tried) {",
		"if (vc.presentingViewController != nil) return YES;",
		"if (vc.viewIfLoaded.window != nil) return YES;",
		"up = up.presentedViewController",
		"if (up == vc) return YES;",
	)
}

// iosPresentOneAtATime reports whether btIOSPresent's body makes no
// presentation while another is on its way, unless that one is stale.
func iosPresentOneAtATime(body string) bool {
	return inSequence(body,
		"if (gBTIOSPresenting != nil && now - gBTIOSPresenting.asked < btIOSPresentStale) {",
		"return;",
		"gBTIOSPresenting = p;",
		"btIOSPresentTry(p, btIOSPresentWaits);",
	)
}

// iosPresentSettle is btIOSPresentSettle as reading_ios.go defines it, in
// seconds; -1 where it is not defined.
func iosPresentSettle(src string) float64 {
	m := regexp.MustCompile(`static const double btIOSPresentSettle = ([0-9.]+);`).FindStringSubmatch(src)
	if m == nil {
		return -1
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return -1
	}
	return v
}

// ON IPHONE AND IPAD A SHARE SHEET IOS WILL NOT SHOW IS SHOWN WHEN IT CAN BE,
// OR SAID NOT TO BE, AND ONE THAT OPENS IS NEVER SAID NOT TO HAVE. UIKit
// refuses, and only logs, a presentation from a view controller still being
// presented or dismissed: a share chosen while the share sheet before it was
// closing showed nothing and said nothing. Every share sheet, and the
// notice, goes through btIOSPresent, which waits out the top view
// controller's transition, presents from the top view controller as it then
// stands, and, refused even so, tries once more and then says "Could not
// share the passage." or "Could not share the card.", as fits. On 1.2.22
// every share sheet was presented directly, once.
//
// The share sheet holds its own presentation back while its share services
// load, with no presenter meanwhile, and UIKit refuses it, when it does,
// about three seconds after it was asked for. Judged 0.35 seconds after
// presenting it by its presenter, a sheet still held back was taken for
// refused: a share chosen as the sheet before it closed presented a second
// sheet, which UIKit refused, and then said "Could not share the passage."
// over the first, which had opened. So a presentation is judged only
// btIOSPresentSettle seconds after it, longer than that wait, by its
// completion, its presenter, its window or its place among the presented
// view controllers, for every view controller presented for it; and a second
// presentation is not made while one is on its way.
func TestIOSSaysWhenItWillNotShowTheShareSheet(t *testing.T) {
	src := readSourceFile(t, "reading_ios.go")

	if s := iosPresentSettle(src); s <= 3 {
		t.Errorf("btIOSPresentSettle is %v seconds; a presentation must be judged after the share sheet's own wait of as long as three seconds", s)
	}
	if s := iosPresentSettle(strings.Replace(src, "btIOSPresentSettle = 5.0;", "btIOSPresentSettle = 0.35;", 1)); s > 3 {
		t.Fatal("control: the check passes a settle of 0.35 seconds")
	}

	try := javaBlockAfter(t, src, "static void btIOSPresentTry(BTIOSPresentation *p, int waits)")
	if !iosPresentJudgesLate(try) {
		t.Errorf("btIOSPresentTry must wait out a transition, present from the top view controller, count it taken by its completion with its view in a window, judge it btIOSPresentSettle seconds later, retry once and then let it go and run refused:\n%s", try)
	}
	// CONTROLS: each step taken out, and the judging done after the short
	// wait that was in use when a late sheet was said to have failed.
	for _, cut := range []struct{ from, to string }{
		{"[tc animateAlongsideTransition:nil", ""},
		{"[p.tried addObject:vc];", ""},
		{"if (shown.viewIfLoaded.window != nil) {", ""},
		{"p.taken = YES;", ""},
		{"(int64_t)(btIOSPresentSettle * NSEC_PER_SEC)", "(int64_t)(btIOSPresentRetry * NSEC_PER_SEC)"},
		{"if (btIOSPresentTaken(p)) {", ""},
		{"btIOSPresentTry(p, btIOSPresentWaits);", ""},
		{"if (p.refused != nil) p.refused();", ""},
	} {
		if iosPresentJudgesLate(strings.Replace(try, cut.from, cut.to, 1)) {
			t.Fatalf("control: the check passes btIOSPresentTry with %q made %q", cut.from, cut.to)
		}
	}

	taken := javaBlockAfter(t, src, "static BOOL btIOSPresentTaken(BTIOSPresentation *p)")
	if !iosPresentTakenLooksEverywhere(taken) {
		t.Errorf("btIOSPresentTaken must count a presentation taken by its completion, its presenter, its view in a window, or its place among the presented view controllers, for every view controller presented for it:\n%s", taken)
	}
	for _, cut := range []string{
		"if (p.taken) return YES;",
		"if (vc.presentingViewController != nil) return YES;",
		"if (vc.viewIfLoaded.window != nil) return YES;",
		"if (up == vc) return YES;",
	} {
		if iosPresentTakenLooksEverywhere(strings.Replace(taken, cut, "", 1)) {
			t.Fatalf("control: the check passes btIOSPresentTaken without %q", cut)
		}
	}

	entry := javaBlockAfter(t, src, "static void btIOSPresent(BTIOSMakePresented make, dispatch_block_t refused)")
	if !iosPresentOneAtATime(entry) {
		t.Errorf("btIOSPresent must make no presentation while another is on its way:\n%s", entry)
	}
	if iosPresentOneAtATime(strings.Replace(entry, "if (gBTIOSPresenting != nil && now - gBTIOSPresenting.asked < btIOSPresentStale) {", "", 1)) {
		t.Fatal("control: the check passes btIOSPresent with no wait for the presentation on its way")
	}

	// btIOSPresent is the one place anything is presented, and a share sheet
	// is built in one place, which says the share's words when refused.
	if n := strings.Count(src, "presentViewController:"); n != 1 {
		t.Errorf("reading_ios.go presents in %d places; every presentation must go through btIOSPresent", n)
	}
	if n := strings.Count(src, "initWithActivityItems:"); n != 1 {
		t.Errorf("reading_ios.go builds a share sheet in %d places, want one, in bibleTextPresentShareSaying", n)
	}
	saying := javaBlockAfter(t, src, "static void bibleTextPresentShareSaying(NSArray *items, NSString *failed, BTIOSAnchor anchor)")
	if !inSequence(saying, "btIOSPresent(", "initWithActivityItems:items", "bibleTextShareNotice(failed);") {
		t.Errorf("bibleTextPresentShareSaying must present the sheet through btIOSPresent and say failed when refused:\n%s", saying)
	}

	// Each share says the words for what it shares.
	passage := `@"` + shareFailedPassage + `"`
	card := `@"` + shareFailedCard + `"`
	for _, c := range []struct{ head, call string }{
		{"void bibleTextShareText(const char *text)", "bibleTextPresentShare(@[s], " + passage + ");"},
		{"void bibleTextShareTextAt(const char *text, double x, double y, double w, double h)", passage + ");"},
		{"static void btIOSPresentWordsShare(NSUInteger token) {", "bibleTextPresentShareFrom(@[s], tv, r, " + passage + ");"},
		{"static void btIOSPresentWordsShare(NSUInteger token) {", "bibleTextPresentShare(@[s], " + passage + ");"},
		{"void bibleTextShareTextAtWords(const char *text, int verse, int at, int len)", "bibleTextPresentShare(@[s], " + passage + ");"},
		{"void bibleTextShareImageFile(const char *path)", "bibleTextPresentShare(@[img], " + card + ");"},
	} {
		if body := javaBlockAfter(t, src, c.head); !strings.Contains(body, c.call) {
			t.Errorf("%s must present with %s:\n%s", c.head, c.call, body)
		}
	}
	if regexp.MustCompile(`bibleTextPresentShare(At|From)?\(@\[[a-z]+\]\)`).MatchString(src) {
		t.Error("a share sheet is presented with no words for its refusal")
	}
}

// A CARD THE PREVIEW CANNOT DRAW IS SAID ON THE PHONES as their share sheets
// say a card they cannot share: on iOS in the alert (bibleTextShareNotice),
// on Android in the toast (BtBridge.shareNotice), each reached from the Go
// side's sayShareFailed. Before, the preview opened on an empty box, and its
// Share closed it and handed nothing over. The JNI descriptor is held with
// every other (jni_descriptor_contract_test.go).
func TestThePhonesSayACardThePreviewCannotDraw(t *testing.T) {
	ios := readSourceFile(t, "reading_ios.go")
	goSide := javaBlockAfter(t, ios, "func sayShareFailed(_ *AppState, line string)")
	if !strings.Contains(goSide, "C.bibleTextShareFailed(c)") {
		t.Errorf("iOS's sayShareFailed must call bibleTextShareFailed:\n%s", goSide)
	}
	objc := javaBlockAfter(t, ios, "void bibleTextShareFailed(const char *msg)")
	if !inSequence(objc, "dispatch_async(dispatch_get_main_queue(), ^{", "bibleTextShareNotice(s);") {
		t.Errorf("bibleTextShareFailed must say the line on the main thread through bibleTextShareNotice:\n%s", objc)
	}

	android := readSourceFile(t, "reading_android.go")
	goSide = javaBlockAfter(t, android, "func sayShareFailed(_ *AppState, line string)")
	if !strings.Contains(goSide, "C.btaShareFailed(C.uintptr_t(env), cs)") {
		t.Errorf("Android's sayShareFailed must call btaShareFailed:\n%s", goSide)
	}
	if !strings.Contains(android, "btaShareFailedM == NULL ||") {
		t.Error("btaEnsureClass must treat a missing shareFailed as the bridge absent")
	}
	java := readNativeSource(t, "android/BtBridge.java")
	body := javaBlockAfter(t, java, "public static void shareFailed(final String msg)")
	sayer := func(b string) bool {
		return inSequence(b, "UI.post(new Runnable() {", "shareNotice(msg);")
	}
	if !sayer(body) {
		t.Errorf("BtBridge.shareFailed must say msg in the share's toast on the UI thread:\n%s", body)
	}
	if sayer(strings.Replace(body, "shareNotice(msg);", "", 1)) {
		t.Fatal("control: the check passes a shareFailed that says nothing")
	}

	// The preview asks for it, with the card's words, where the card cannot
	// be drawn, before anything opens.
	preview := javaBlockAfter(t, readSourceFile(t, "share_preview.go"), "func showShareImagePreview(state *AppState, quote, cite, version string)")
	if !inSequence(preview, "previewCardRender(state, quote, cite, version, variant)", "if err != nil {", "sayShareFailed(state, shareFailedCard)", "return", "widget.NewModalPopUp(") {
		t.Errorf("showShareImagePreview must say the card could not be shared, and open nothing, when it cannot draw it:\n%s", preview)
	}
}
