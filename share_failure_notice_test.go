package bibletext

// A SHARE THE READER STARTED DOES NOT END IN SILENCE, ON EITHER PHONE.
//
// Android's text share called startActivity with nothing around it, so a
// chooser that could not start threw on the UI thread and closed the app: on
// the Android 16 emulator with the system chooser disabled, 1.2.17 closed on
// Share with citation, where the fixed build shows "Could not share the
// passage." and stays open. Share as image beside it already caught the same
// failure and said "Could not share the card.". On iOS a card that cannot be
// read returned without a word. Neither bridge runs on the host, so both are
// held at the source, and each check is shown failing on a copy with its
// line taken out.

import (
	"regexp"
	"strings"
	"testing"
)

const (
	shareTextFailedNotice  = "Could not share the passage."
	shareImageFailedNotice = "Could not share the card."
)

// shareTextCatches reports whether an Android shareText body starts its
// chooser inside a try whose catch shows the notice, after the no-activity
// return.
func shareTextCatches(t *testing.T, body string) bool {
	t.Helper()
	noActivity := statementLine(t, "if (activity == null) return;").FindStringIndex(body)
	try := regexp.MustCompile(`(?m)^[ \t]*try \{[ \t]*$`).FindStringIndex(body)
	if noActivity == nil || try == nil || try[0] < noActivity[1] {
		return false
	}
	tryBlock := braceBlock(t, body[try[1]-1:])
	if !statementLine(t, "activity.startActivity(Intent.createChooser(i, null));").MatchString(tryBlock) {
		return false
	}
	rest := strings.TrimLeft(body[try[1]-1+len(tryBlock):], " \t")
	if !strings.HasPrefix(rest, "catch (Throwable t) {") {
		return false
	}
	catch := braceBlock(t, rest[strings.Index(rest, "{"):])
	return statementLine(t, `shareNotice("`+shareTextFailedNotice+`");`).MatchString(catch)
}

func TestAndroidTextShareSaysWhenTheChooserCannotStart(t *testing.T) {
	java := readNativeSource(t, "android/BtBridge.java")
	body := javaBlockAfter(t, java, "public static void shareText(final String body)")
	if !shareTextCatches(t, body) {
		t.Errorf("shareText must start the chooser inside a try whose catch says %q:\n%s",
			shareTextFailedNotice, body)
	}
	// CONTROLS: the notice taken out of the catch, and the try taken away.
	if shareTextCatches(t, strings.Replace(body, `shareNotice("`+shareTextFailedNotice+`");`, "// silence", 1)) {
		t.Fatal("control: the check passes a catch that says nothing")
	}
	if shareTextCatches(t, strings.Replace(body, "try {", "{", 1)) {
		t.Fatal("control: the check passes a chooser started outside a try")
	}

	// Share as image's catch keeps its words, which the iOS card matches.
	image := javaBlockAfter(t, java, "public static void shareImage(final String path)")
	if !statementLine(t, `shareNotice("`+shareImageFailedNotice+`");`).MatchString(image) {
		t.Errorf("shareImage's catch no longer says %q", shareImageFailedNotice)
	}
}

// iosCardNotice reports whether bibleTextShareImageFile's unreadable-image
// branch says the notice before returning.
func iosCardNotice(t *testing.T, body string) bool {
	t.Helper()
	i := strings.Index(body, "if (img == nil) {")
	if i < 0 {
		return false
	}
	branch := braceBlock(t, body[i+len("if (img == nil) "):])
	notice := statementLine(t, `bibleTextShareNotice(@"`+shareImageFailedNotice+`");`).FindStringIndex(branch)
	ret := statementLine(t, "return;").FindStringIndex(branch)
	return notice != nil && ret != nil && notice[1] <= ret[0]
}

func TestIOSImageShareSaysWhenTheCardCannotBeRead(t *testing.T) {
	src := readSourceFile(t, "reading_ios.go")

	body := javaBlockAfter(t, src, "void bibleTextShareImageFile(const char *path)")
	if !iosCardNotice(t, body) {
		t.Errorf("bibleTextShareImageFile must say %q when the card cannot be read:\n%s",
			shareImageFailedNotice, body)
	}
	// CONTROL: the bare return this replaced.
	if iosCardNotice(t, regexp.MustCompile(`(?s)if \(img == nil\) \{.*?\n\s*\}`).ReplaceAllString(body, "if (img == nil) return;")) {
		t.Fatal("control: the check passes the bare return")
	}

	// The notice is an alert presented from the view controller the share
	// sheet would have used, and says nothing with none (no window on screen).
	notice := javaBlockAfter(t, src, "static void bibleTextShareNotice(NSString *msg)")
	steps := []string{
		"UIViewController *top = bibleTextTopVC();",
		"if (top == nil) return;",
		"[UIAlertController alertControllerWithTitle:nil",
		"message:msg",
		`[ac addAction:[UIAlertAction actionWithTitle:@"OK"`,
		"[top presentViewController:ac animated:YES completion:nil];",
	}
	if !inSequence(notice, steps...) {
		t.Errorf("bibleTextShareNotice must present an alert from the top view controller:\n%s", notice)
	}
	// The notice is defined before its first use, as C requires.
	if strings.Index(src, "static void bibleTextShareNotice(") > strings.Index(src, "bibleTextShareNotice(@") {
		t.Error("bibleTextShareNotice is used before it is defined")
	}
}
