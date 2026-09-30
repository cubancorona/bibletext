package bibletext

import (
	"regexp"
	"strings"
	"testing"
)

// THE BUG THIS PINS. Fyne's Android driver learns the window's insets in one
// place: GoNativeActivity reads them in a decor OnLayoutChangeListener and
// hands them to the canvas, which lays its content out inside them. The
// activity window is adjust-resize (NativeActivity sets that in onCreate),
// so the system-window insets the driver reads count a raised keyboard as
// the bottom inset — measured on the Android 15 emulator, 883 px up against
// 63 down — and the canvas is laid out above the keyboard by whichever decor
// layout pass runs while it is up. On that Android the decor keeps its full
// height under the keyboard (a window targeting API 35 or later is drawn
// edge-to-edge), so the keyboard's departure arrives as an insets dispatch
// and brings no layout pass of its own; the one pass near it, the driver
// hiding its EditText, races the IME's inset update, and when it won the
// driver read the keyboard still in, nothing read the insets again, and the
// app stayed laid out at keyboard height — the reading text in a band, the
// tab bar mid-screen, the bottom third of the screen empty — until the
// activity was shown afresh. The keyboard watcher sees every dispatch, so it
// asks the decor for the pass the driver needs, inside the same change guard
// as its report to Go.
//
// The Java is compiled into the mobile dex and no host test runs it, and the
// stale value lives in the driver's own inset, out of reach of the Go seams
// (canvasArea, noteSoftKeyboard), so the watcher is held under a source
// contract, as the arrival wiring is. Each statement the contract wants is
// matched as a whole line, so a statement commented out fails the contract
// as a missing one does.
func TestAndroidKeyboardInsetChangeAsksForALayoutPass(t *testing.T) {
	src := readNativeSource(t, "android/BtBridge.java")

	start := strings.Index(src, "private static void installKeyboardWatcher(")
	if start < 0 {
		t.Fatal("android/BtBridge.java has no installKeyboardWatcher")
	}
	end := strings.Index(src[start:], "return v.onApplyWindowInsets(insets);")
	if end < 0 {
		t.Fatal("the keyboard watcher no longer passes the insets through to the platform handler")
	}
	watcher := src[start : start+end]

	const guardOpen = "if (px != lastImePx) {"
	guardAt := strings.Index(watcher, guardOpen)
	if guardAt < 0 {
		t.Fatal("the keyboard watcher has no change guard on the keyboard's inset")
	}
	guard := braceBlock(t, watcher[guardAt+len(guardOpen)-1:])

	report := statementLine(t, "nativeKeyboardChanged(px);").FindStringIndex(guard)
	if report == nil {
		t.Fatal("the change guard does not report the keyboard's overlap to Go")
	}
	relayoutLine := statementLine(t, "v.requestLayout();")
	relayout := relayoutLine.FindStringIndex(guard)
	if relayout == nil {
		t.Fatal("a changed keyboard inset does not ask the decor for a layout pass, so the driver never re-reads the insets when the keyboard goes and the app can stay laid out at keyboard height")
	}
	if relayout[0] < report[0] {
		t.Fatal("the layout pass is asked for before the overlap is reported; Go must hear the change first")
	}
	if n := len(relayoutLine.FindAllStringIndex(watcher, -1)); n != 1 {
		t.Fatalf("the layout pass must be asked for inside the change guard only, or every insets dispatch would lay the window out again; the watcher asks %d times", n)
	}
}

// statementLine matches the Java statement as a line of its own: whitespace,
// the statement, whitespace, end of line. A statement behind a comment marker
// is not on such a line, so commenting it out fails a contract that wants it
// present, and a mention of it in a comment cannot stand in for it.
func statementLine(t *testing.T, statement string) *regexp.Regexp {
	t.Helper()
	return regexp.MustCompile(`(?m)^[ \t]*` + regexp.QuoteMeta(statement) + `[ \t]*$`)
}

// braceBlock returns the text of the block that opens at src[0], which must be
// an opening brace, up to and including its matching close.
func braceBlock(t *testing.T, src string) string {
	t.Helper()
	if src == "" || src[0] != '{' {
		t.Fatal("braceBlock: the text does not start at an opening brace")
	}
	depth := 0
	for i, r := range src {
		switch r {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return src[:i+1]
			}
		}
	}
	t.Fatal("braceBlock: the block never closes")
	return ""
}
