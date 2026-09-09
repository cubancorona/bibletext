package bibletext

// The iOS foreground recovery lives behind //go:build ios, so no host test can
// call it. What a host test CAN hold is the shape it has to keep — and the two
// things that would silently un-fix the bug are both visible in the source: the
// gate not being cleared, and it being cleared too late.
//
// This follows the idiom android_arrival_scroll_test.go already uses for the
// Android push, which reads that file as text for the same reason.

import (
	"os"
	"strings"
	"testing"
)

func TestTheIOSForegroundRecoveryClearsTheGateBeforeRebuilding(t *testing.T) {
	src, err := os.ReadFile("overlay_recovery_ios.go")
	if err != nil {
		t.Fatalf("the iOS foreground recovery is gone: %v", err)
	}
	text := string(src)

	// It must ASK the view rather than trust the Go side's belief — asking is
	// the whole point, since the two disagreeing is the bug.
	if !strings.Contains(text, "nativeReadingTextLength()") {
		t.Error("the recovery does not ask the native view how much text it holds, so it " +
			"cannot tell a drawn pane from an emptied one")
	}

	// And it must stand down when the pane is fine, or every foreground rebuilds
	// the window and the reader loses their place for nothing.
	if !strings.Contains(text, "if nativeReadingTextLength() > 0 {") {
		t.Error("the recovery does not return early on a populated pane; it would rebuild " +
			"the window on every single foreground")
	}

	// THE ORDER IS THE FIX. pushChapterHTML gates on lastPushedBodyFP, so a
	// rebuild that runs before the gate is cleared pushes nothing and the pane
	// stays blank — which is the defect, not the repair.
	body := strings.Index(text, `lastPushedBodyFP = ""`)
	chap := strings.Index(text, `lastPushedBookChapter = ""`)
	rebuild := strings.Index(text, "rebuildWindow(state)")
	if body < 0 {
		t.Fatal("the recovery never clears lastPushedBodyFP, so pushChapterHTML will " +
			"decide the chapter is already rendered and push nothing")
	}
	if chap < 0 {
		t.Fatal("the recovery never clears lastPushedBookChapter")
	}
	if rebuild < 0 {
		t.Fatal("the recovery never rebuilds, so clearing the gate achieves nothing")
	}
	if rebuild < body || rebuild < chap {
		t.Errorf("the rebuild at %d runs BEFORE the gate is cleared (body %d, chapter %d) — "+
			"the push would be gated out and the pane would stay blank", rebuild, body, chap)
	}

	// A cold start has a legitimately empty view. Recovering there would fight
	// the launch path instead of repairing anything.
	if !strings.Contains(text, `if lastPushedBookChapter == "" {`) {
		t.Error("the recovery does not exempt the state before the first push, so it can " +
			"fire during a cold start")
	}
}

// Exactly one implementation must compile for iOS. The stub used to claim every
// non-Android platform, which is what left iOS with no recovery at all.
func TestOnlyOneForegroundRecoveryCompilesForIOS(t *testing.T) {
	stub, err := os.ReadFile("overlay_recovery_other.go")
	if err != nil {
		t.Fatalf("cannot read the stub: %v", err)
	}
	head := string(stub)
	if i := strings.Index(head, "package "); i > 0 {
		head = head[:i]
	}
	if !strings.Contains(head, "//go:build !android && !ios") {
		t.Errorf("the stub's build tag is %q. It must exclude ios, or two "+
			"foregroundOverlayRecovery definitions compile for that platform — and if it "+
			"stops excluding ios while the real one is deleted, iOS silently loses the "+
			"recovery again, which is exactly how this bug shipped",
			strings.TrimSpace(head[strings.Index(head, "//go:build"):]))
	}

	// The control: prove the file really is the stub and not something that
	// happens to carry the tag, so this test cannot pass on an empty file.
	if !strings.Contains(string(stub), "func foregroundOverlayRecovery(state *AppState) {}") {
		t.Error("overlay_recovery_other.go no longer defines the no-op recovery, so the " +
			"tag assertion above is not guarding what it claims to")
	}
}

// The other half of the same defect, held at its source. bibleTextApplyHTML used
// to accept a non-nil but ZERO-LENGTH import as a success: it wrote the empty
// string into the view, satisfied the generation tripwire, returned YES, and so
// stopped the retry ladder that exists precisely because that importer fails
// intermittently on return to the foreground. That is the likeliest way the pane
// went blank in the first place; the foreground recovery above only repairs it
// afterwards.
func TestTheIOSHTMLImportTreatsAnEmptyResultAsFailure(t *testing.T) {
	src, err := os.ReadFile("reading_ios.go")
	if err != nil {
		t.Fatalf("cannot read the iOS pane: %v", err)
	}
	text := string(src)

	if !strings.Contains(text, "if (as == nil || as.length == 0) return NO;") {
		t.Error("the HTML import no longer rejects a zero-length result. A non-nil empty " +
			"string would be written into the view as a successful push, blanking the " +
			"pane and stopping the retry ladder that would otherwise recover it")
	}

	// The control: the bare nil-only form must be gone, or both could be present
	// and the check above would pass while the old path still ran.
	if strings.Contains(text, "\n    if (as == nil) return NO;") {
		t.Error("the old nil-only guard is still in the file")
	}
}
