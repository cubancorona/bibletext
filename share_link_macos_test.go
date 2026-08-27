package bibletext

import (
	"os"
	"strings"
	"testing"
)

// The macOS link category only works if it attaches to the delegate class GLFW
// actually installs, gates on the browsing activity type, and reports Go's
// real decision back to macOS. A silent mismatch has a silent cost: the
// category is discarded at load and every clicked link falls back to the
// browser, which looks exactly like the entitlement being absent. This pins
// the load-bearing spellings in our own file so an edit cannot drift them.
// (A GLFW delegate-class RENAME needs no test — the category's class
// reference is a link-time symbol, so that build fails loudly on its own;
// share_link_macos.go documents the genuinely silent case instead.)
func TestMacLinkCategoryTargetsGLFWsDelegate(t *testing.T) {
	src, err := os.ReadFile("share_link_macos.go")
	if err != nil {
		t.Fatalf("read share_link_macos.go: %v", err)
	}
	body := string(src)

	for _, want := range []string{
		// The class GLFW's cocoa_init.m installs, and the category on it.
		"@interface GLFWApplicationDelegate",
		"@implementation GLFWApplicationDelegate (BibleTextLinks)",
		// The Universal Link entry point, gated on the browsing activity type,
		// answering with Go's real decision rather than an unconditional YES.
		"continueUserActivity",
		"NSUserActivityTypeBrowsingWeb",
		"bibleTextOpenedLink((char *)url.absoluteString.UTF8String) ? YES : NO",
		// The scheme entry point, so a future scheme needs no delegate work.
		"openURLs:(NSArray<NSURL *> *)urls",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("share_link_macos.go is missing %q", want)
		}
	}

	// CONTROL: the assertion mechanism must be able to fail.
	if strings.Contains(body, "this string does not appear in the file") {
		t.Fatal("the control string matched; the sweep proves nothing")
	}
}
