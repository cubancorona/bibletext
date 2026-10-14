package bibletext

import (
	"strings"
	"testing"
)

// A one-shot placement, an arrival or a restore, is made at whatever layout
// the text has when it runs, and the frame can already have left that width:
// the text then lays out again at the frame's width with the scroll position
// unchanged. Two things went wrong after that on Android. Nothing re-placed
// the reader, so the top of the view was wherever that scroll position falls
// in the new layout; and a frame arriving in between armed a reflow from the
// scroll position read against the new layout, which the reflow then
// restored. A restore of John 3:14 made at 1017 px with the text laying out
// at 954 px straight after put the reader at verse 13, in 2 of 3 forced runs
// on the emulator; with the bridge's change, at 14 in 3 of 3.
//
// The model is android_reading_position_test.go's: the bridge's verseBand,
// anchorAt and anchorY over a laid-out page. The bridge now re-places the
// placement at the new width (replaceAfterWidth) and a reflow armed
// meanwhile carries the placement (placementAwaitsReplace in setFrame). The
// scroll position read against the other layout, the code before, is the
// control: it must move the top verse somewhere, or the model no longer
// shows what the change fixes.
func TestAPlacementMadeAtOneWidthIsKeptWhenTheTextLaysOutAtAnother(t *testing.T) {
	java := readNativeSource(t, "android/BtBridge.java")
	setFrame := javaMethodBody(t, java, "public static void setFrame(")
	takes := strings.Index(setFrame, "if (placementAwaitsReplace()) {")
	reads := strings.Index(setFrame, "float[] a = anchorAt(scroll.getScrollY());")
	if takes < 0 || reads < 0 || takes > reads {
		t.Error("setFrame does not arm a reflow from a placement awaiting re-placement before it reads the scroll position")
	}
	for _, want := range []string{
		"pendingReflowVerse = placedVerse;",
		"pendingReflowWithin = placedWithin;",
		"placedAtMs = 0; // the reflow carries the placement from here",
	} {
		if !strings.Contains(setFrame, want) {
			t.Errorf("setFrame lacks %q", want)
		}
	}
	for _, want := range []string{
		"replaceAfterWidth(r - l);",
		"notePlacement(pendingRestoreVerse, pendingRestoreWithin, vy);",
		"return placedAtMs != 0 && content != null && content.getWidth() != placedContentW",
		"int vy = placedVerse > 0 ? anchorY(placedVerse, placedWithin) : -1;",
	} {
		if !strings.Contains(java, want) {
			t.Errorf("BtBridge.java lacks %q", want)
		}
	}
	// A new chapter, a new activity and a reader's scroll each end the
	// placement's hold.
	if n := strings.Count(java, "placedAtMs = 0;"); n < 4 {
		t.Errorf("placedAtMs = 0; appears %d times, want the scroll listener, setHtml, the activity's attach and the reflow's capture", n)
	}

	var verses []string
	lengths := []int{9, 23, 14, 31, 6, 18, 27, 11, 40, 8, 16, 22, 35, 12, 19, 7, 26, 15, 30, 10}
	words := strings.Fields("the people went out at first light to the fields and the hills " +
		"and they worked until the evening came and the lamps were lit in every house")
	for i, n := range lengths {
		var b []string
		for k := 0; k < n; k++ {
			b = append(b, words[(i*5+k)%len(words)])
		}
		verses = append(verses, strings.Join(b, " ")+".")
	}
	paras := map[int]bool{1: true, 6: true, 12: true, 17: true}
	// 34 characters to a line at the placement's width, 32 at the frame's: a
	// phone's pane and the same pane 63 px narrower.
	placed := layWrapPage(verses, paras, 34, 60, 1900)
	other := layWrapPage(verses, paras, 32, 60, 1900)

	var positions, keptNow, keptBack, movedNow, movedBack int
	for y := 1; y <= placed.scrollRange(); y += placed.pitch / 3 {
		v, within, ok := placed.anchorAt(y)
		if !ok {
			continue
		}
		y0 := placed.anchorY(v, within)
		if y0 < 0 || y0 >= placed.scrollRange() || y0 >= other.scrollRange() {
			continue
		}
		positions++
		want := placed.versesOnTopLine(y0)

		// The text lays out at the other width and stays there. Before: the
		// scroll position stands. After: the placement is put back.
		if !sharesAVerse(want, other.versesOnTopLine(y0)) {
			movedNow++
		}
		if yR := other.anchorY(v, within); yR < 0 || (yR < other.scrollRange() && !sharesAVerse(want, other.versesOnTopLine(yR))) {
			keptNow++
		}

		// A frame arrives before the re-placement and the text goes back to
		// the placement's width. Before: the reflow restores the anchor read
		// at y0 against the other layout. After: it restores the placement.
		if cv, cw, ok := other.anchorAt(y0); ok {
			if yB := placed.anchorY(cv, cw); yB >= 0 && yB < placed.scrollRange() && !sharesAVerse(want, placed.versesOnTopLine(yB)) {
				movedBack++
			}
		}
		if yB := placed.anchorY(v, within); yB != y0 {
			keptBack++
		}
	}
	if positions == 0 {
		t.Fatal("the model placed nothing")
	}
	if keptNow > 0 {
		t.Errorf("re-placed at the other width, the top verse moved at %d of %d positions", keptNow, positions)
	}
	if keptBack > 0 {
		t.Errorf("a reflow carrying the placement moved the reader at %d of %d positions", keptBack, positions)
	}
	if movedNow == 0 || movedBack == 0 {
		t.Errorf("the scroll position read against the other layout moved the top verse at %d and %d of %d positions; the model no longer shows what the change fixes",
			movedNow, movedBack, positions)
	}
	t.Logf("%d placements: the scroll position left standing moved the top verse at %d, the reflow read from it at %d; the placement carried, at none",
		positions, movedNow, movedBack)
}

// A frame that changes the width and a second that brings it back before
// the text lays out leave nothing to reflow: applyPendingReflow runs only on
// a change of the text's width, and a reflow left armed makes the scroll
// listener drop every scroll of the reader's until the next chapter, so the
// saved position stays where the reader was when the frames came.
func TestAReflowArmedByAFrameThatComesBackIsDropped(t *testing.T) {
	java := readNativeSource(t, "android/BtBridge.java")
	setFrame := javaMethodBody(t, java, "public static void setFrame(")
	for _, want := range []string{
		"pendingReflowFromW = content != null ? content.getWidth() : 0;",
		"} else if (pendingReflowFrac >= 0 && w > 1 && content != null\n                        && w == pendingReflowFromW && content.getWidth() == pendingReflowFromW) {",
	} {
		if !strings.Contains(setFrame, want) {
			t.Errorf("setFrame lacks %q", want)
		}
	}
	i := strings.Index(setFrame, "w == pendingReflowFromW")
	if i < 0 {
		t.Fatal("setFrame never compares the frame with the width the reflow was armed at")
	}
	drop := setFrame[i:]
	if j := strings.Index(drop, "frameX = x;"); j >= 0 {
		drop = drop[:j]
	}
	if !strings.Contains(drop, "pendingReflowFrac = -1f;") {
		t.Errorf("a frame that brings the width back does not drop the reflow:\n%s", drop)
	}
	listener := java[strings.Index(java, "public void onScrollChanged() {"):]
	if i := strings.Index(listener, "if (pendingReflowFrac >= 0) return;"); i < 0 ||
		i > strings.Index(listener, "lastAnchor = new float[]") {
		t.Error("the scroll listener no longer stands down for an armed reflow before it updates lastAnchor; the drop guards that")
	}
}

// The two tests above hold BtBridge.java's source, and the first models the
// arithmetic; neither runs the bridge. The behaviour rests on the emulator
// runs that forced each case: the John 3:14 restore at 1017 px with the text
// laying out at 954 px, and the 1276 px frame pushed with the real one.
//
// The drop also compares a frame's width with the reading content's width,
// which is the same quantity only while the content fills the overlay's
// window: the window takes the frame's width (setFrame), the root fills the
// window, the scroll fills the root and the content fills the scroll, and
// none of the three has side padding. The reading page's padding is the
// TextView's own (applyReadingPadding). A margin or padding on any of the
// three would make every frame differ from the content by its size, and the
// comparison would never drop a reflow again; this holds the layout to that.
func TestTheReadingContentIsAsWideAsTheOverlaysFrame(t *testing.T) {
	java := readNativeSource(t, "android/BtBridge.java")
	for _, want := range []string{
		"dialog.setContentView(root);",
		"root.addView(scroll, new FrameLayout.LayoutParams(\n" +
			"                FrameLayout.LayoutParams.MATCH_PARENT, FrameLayout.LayoutParams.MATCH_PARENT));",
		"scroll.addView(content, new FrameLayout.LayoutParams(\n" +
			"                FrameLayout.LayoutParams.MATCH_PARENT, FrameLayout.LayoutParams.WRAP_CONTENT));",
		"lp.width = frameW;",
	} {
		if !strings.Contains(java, want) {
			t.Errorf("BtBridge.java lacks %q: the content's width is no longer the frame's", want)
		}
	}
	for _, view := range []string{"root", "scroll", "content"} {
		for _, call := range []string{".setPadding(", ".setPaddingRelative("} {
			if strings.Contains(java, view+call) {
				t.Errorf("BtBridge.java pads %s (%s%s): the content is then narrower than the frame, "+
					"and setFrame's width comparison never drops a reflow", view, view, call)
			}
		}
	}
	if !strings.Contains(javaMethodBody(t, java, "private static boolean applyReadingPadding("), "text.setPadding(") {
		t.Error("applyReadingPadding no longer pads the TextView; the reading page's side padding has moved, " +
			"so re-check that the content still spans the frame")
	}
}

// javaMethodBody is the source of the Java method whose declaration begins
// with decl, up to the first line that closes a method at four spaces.
func javaMethodBody(t *testing.T, java, decl string) string {
	t.Helper()
	i := strings.Index(java, decl)
	if i < 0 {
		t.Fatalf("BtBridge.java has no %q", decl)
	}
	body := java[i:]
	end := strings.Index(body, "\n    }\n")
	if end < 0 {
		t.Fatalf("%q has no end", decl)
	}
	return body[:end]
}
