package bibletext

import (
	"strings"
	"testing"
)

// The Android bridge is compiled into the mobile dex and is not exercised by
// host Go tests. Keep the cold-start liveness wiring under a source contract so
// a later scroll refactor cannot quietly return to a bounded pre-layout retry.
func TestAndroidColdStartArrivalWaitsForFirstLayout(t *testing.T) {
	src := readNativeSource(t, "android/BtBridge.java")

	listenerStart := strings.Index(src, "text.addOnLayoutChangeListener")
	if listenerStart < 0 {
		t.Fatal("Android reader must retry a pending verse from the TextView layout callback")
	}
	listenerEnd := strings.Index(src[listenerStart:], "// A real (or changed) WIDTH")
	if listenerEnd < 0 {
		t.Fatal("Android TextView layout callback has no stable boundary")
	}
	listener := src[listenerStart : listenerStart+listenerEnd]
	if !strings.Contains(listener, "if (pendingVerse > 0) applyPendingScroll();") {
		t.Fatal("the first real TextView layout does not re-apply the pending arrival verse")
	}
	scrollListenerStart := strings.Index(src, "scroll.getViewTreeObserver().addOnScrollChangedListener")
	if scrollListenerStart < 0 {
		t.Fatal("Android scroll listener not found")
	}
	scrollListenerEnd := strings.Index(src[scrollListenerStart:], "// The Dialog is not shown here")
	if scrollListenerEnd < 0 {
		t.Fatal("Android scroll listener has no stable boundary")
	}
	scrollListener := src[scrollListenerStart : scrollListenerStart+scrollListenerEnd]
	protectArrival := strings.Index(scrollListener, "if (pendingVerse > 0) return;")
	disarmArrival := strings.Index(scrollListener, "pendingVerse = 0;")
	if protectArrival < 0 || disarmArrival < 0 || protectArrival > disarmArrival {
		t.Fatal("a cold first-layout scroll callback can disarm the arrival as a reader gesture")
	}

	methodStart := strings.Index(src, "private static void applyPendingScroll()")
	if methodStart < 0 {
		t.Fatal("Android pending-scroll method not found")
	}
	methodEnd := strings.Index(src[methodStart:], "/** armRestore")
	if methodEnd < 0 {
		t.Fatal("Android pending-scroll method has no stable boundary")
	}
	method := src[methodStart : methodStart+methodEnd]
	if !strings.Contains(method, "if (layout == null || verseNums.length == 0) {") {
		t.Fatal("pending verse must distinguish unavailable geometry from a missing verse")
	}
	wait := strings.Index(method, "if (layout == null || verseNums.length == 0) {")
	waitReturn := strings.Index(method[wait:], "return;")
	fallback := strings.Index(method, "if (!pendingPlace)")
	if waitReturn < 0 || fallback < 0 || wait+waitReturn > fallback {
		t.Fatal("an unresolved arrival can fall through to top/restore before first layout")
	}
	if strings.Contains(src, "SCROLL_MAX_RETRIES") {
		t.Fatal("cold-start arrival liveness must be layout-driven, not a bounded Handler retry")
	}
}

// A first install renders from the embedded seed while the full Bible is
// fetched. The second same-chapter import must not capture the still-unplaced
// top as a restore and erase the link arrival.
func TestAndroidColdStartArrivalSurvivesSeedDataSwap(t *testing.T) {
	java := readNativeSource(t, "android/BtBridge.java")
	if !strings.Contains(java, "setHtml(final String html, final float frac, final int arrivalVerse)") ||
		!strings.Contains(java, "pendingVerse = arrivalVerse > 0 ? arrivalVerse : 0;") {
		t.Fatal("Android chapter content and its arrival verse must be one UI operation")
	}
	if strings.Contains(java, "public static void scrollToVerse") {
		t.Fatal("a separate Java arrival operation can race a following chapter import")
	}

	goSrc := readNativeSource(t, "reading_android.go")
	for _, want := range []string{
		`"setHtml", "(Ljava/lang/String;FI)V"`,
		"explicitArrival := state.forceReposition || carryDataSwapArrival",
		// The capture asks the SHARED predicate now, so the arrival clause cannot
		// be omitted here without being omitted for every pane at once.
		"shouldCaptureScrollRestore(state, bc == lastPushedBookChapter",
		"armAndroidDataSwapArrival(state, arrivalVerse)",
		"C.btaSetHtml(C.uintptr_t(env), ch, C.float(frac), C.int(arrivalVerse))",
	} {
		if !strings.Contains(goSrc, want) {
			t.Fatalf("Android seed-to-full arrival contract is missing %q", want)
		}
	}

	exportSrc := readNativeSource(t, "reading_android_export.go")
	if !strings.Contains(exportSrc, "clearAndroidDataSwapArrival(state)") {
		t.Fatal("a genuine reader scroll must cancel the carried seed-data arrival")
	}
	nextStart := strings.Index(exportSrc, "func btaNoteNextTapped()")
	if nextStart < 0 {
		t.Fatal("Android note-cycle callback not found")
	}
	nextEnd := strings.Index(exportSrc[nextStart:], "\n}\n")
	if nextEnd < 0 {
		t.Fatal("Android note-cycle callback has no stable boundary")
	}
	next := exportSrc[nextStart : nextStart+nextEnd]
	// The advance is reached through the shared verb entry point now
	// (performNoteAction, notes_action.go) rather than called directly — the
	// same function, named once, so a press can say WHICH note it means. What
	// this contract is about is unchanged: Android must not grow its own
	// advance or its own render path.
	if !strings.Contains(next, "performNoteAction(state, noteActionNext, noteKeyFocused)") ||
		!strings.Contains(next, "state.refreshReadingOnly()") {
		t.Fatal("Android note-cycle callback must preserve the shared advance and render path")
	}
	if strings.Contains(next, "forceReposition") {
		t.Fatal("Android callback must not override advanceNoteFocus's conditional placement")
	}
	if !strings.Contains(goSrc, "arrivalVerse = sp.Lo") {
		t.Fatal("Android explicit placement does not pass the selected note's anchor to the bridge")
	}
	if !strings.Contains(goSrc, "preserveTop = true") ||
		!strings.Contains(goSrc, "else if preserveTop") ||
		!strings.Contains(goSrc, "!preserveTop && here") {
		t.Fatal("Android same-anchor replacement must preserve a reader parked at chapter top")
	}
	if !strings.Contains(java, `setContentDescription("Next note")`) {
		t.Fatal("Android note-cycle control has no deterministic accessible name")
	}
}

// Go-to is an explicit arrival even when it names the chapter already on
// screen. Both Android carry cases — a nonzero saved fraction and the special
// all-zero top position — live behind !explicitArrival, so neither may outrank
// the verse requested by goToVerseRange.
func TestAndroidGoToOutranksSameChapterTopAndMidCarry(t *testing.T) {
	goToSrc := readNativeSource(t, "verse_of_day.go")
	goToStart := strings.Index(goToSrc, "func goToVerseRange(")
	if goToStart < 0 {
		t.Fatal("goToVerseRange not found")
	}
	goToEnd := strings.Index(goToSrc[goToStart:], "\n}\n")
	if goToEnd < 0 {
		t.Fatal("goToVerseRange has no stable function boundary")
	}
	goTo := goToSrc[goToStart : goToStart+goToEnd]
	if !strings.Contains(goTo, "\n\tstate.forceReposition = true\n\tstate.refresh()") {
		t.Fatal("Go-to does not declare unconditional explicit placement before refreshing")
	}

	androidSrc := readNativeSource(t, "reading_android.go")
	pushStart := strings.Index(androidSrc, "func pushChapterHTML(")
	if pushStart < 0 {
		t.Fatal("Android pushChapterHTML not found")
	}
	pushEnd := strings.Index(androidSrc[pushStart:], "\n}\n\n// --- Reading-position persistence")
	if pushEnd < 0 {
		t.Fatal("Android pushChapterHTML has no stable function boundary")
	}
	push := androidSrc[pushStart : pushStart+pushEnd]
	if !strings.Contains(push, "explicitArrival := state.forceReposition || carryDataSwapArrival") {
		t.Fatal("Android does not classify Go-to's forceReposition as an explicit arrival")
	}
	capture := strings.Index(push, "if shouldCaptureScrollRestore(state, bc == lastPushedBookChapter")
	arm := strings.Index(push, "\n\tarmPendingRestore(state)")
	if capture < 0 || arm < 0 || capture > arm {
		t.Fatal("Android same-chapter carry is not guarded by !explicitArrival")
	}
	carry := push[capture:arm]
	if !strings.Contains(carry, "state.restore = &restoreAnchor{") {
		t.Fatal("test contract lost the mid-chapter carry branch")
	}
	if !strings.Contains(carry, "preserveTop = true") {
		t.Fatal("test contract lost the top-of-chapter carry branch")
	}
	if !strings.Contains(push, "if state.restore == nil && !preserveTop && here {") ||
		!strings.Contains(push, "arrivalVerse = sp.Lo") {
		t.Fatal("Android explicit placement is not handed to the requested verse")
	}
}

func TestAndroidRotationReappliesScrollAfterWidthReflow(t *testing.T) {
	src := readNativeSource(t, "android/BtBridge.java")

	setFrameStart := strings.Index(src, "public static void setFrame")
	if setFrameStart < 0 {
		t.Fatal("Android setFrame method not found")
	}
	setFrameEnd := strings.Index(src[setFrameStart:], "private static void applyPendingReflow")
	if setFrameEnd < 0 {
		t.Fatal("Android frame/reflow methods not found")
	}
	setFrame := src[setFrameStart : setFrameStart+setFrameEnd]
	capture := strings.Index(setFrame, "pendingReflowFrac = range > 0")
	resize := strings.Index(setFrame, "frameW = w;")
	if capture < 0 || resize < 0 || capture > resize {
		t.Fatal("the live scroll fraction must be captured before a width-changing frame is applied")
	}

	if !strings.Contains(src, "refreshNoteSticker();\n                    applyPendingReflow();") {
		t.Fatal("the new content width must reapply the pre-rotation scroll fraction")
	}
	if !strings.Contains(src, "ownScrollTo(Math.round(frac * scrollRange()));") {
		t.Fatal("rotation restore must use the post-reflow scroll range")
	}
	if !strings.Contains(src, "if (pendingReflowFrac >= 0) return;") {
		t.Fatal("a width-clamp callback must not be persisted as a reader scroll")
	}

	setHTMLStart := strings.Index(src, "public static void setHtml")
	if setHTMLStart < 0 {
		t.Fatal("Android setHtml method not found")
	}
	setHTMLEnd := strings.Index(src[setHTMLStart:], "private static int pendingVerse")
	if setHTMLEnd < 0 ||
		!strings.Contains(src[setHTMLStart:setHTMLStart+setHTMLEnd], "pendingReflowFrac = -1f;") {
		t.Fatal("a new chapter placement must cancel an unfinished width restore")
	}
}
