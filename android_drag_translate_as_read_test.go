package bibletext

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// ON ANDROID, A DRAG OUT OF THE READING PANE, AND THE TEXT HANDED TO
// TRANSLATE, CARRY THE SELECTION AS COPY CARRIES IT (decided 15 October
// 2026; Translate added to the reading toolbar 21 October 2026).
//
// THE DEFECT THIS EXISTS FOR. The reading view holds characters that are
// there for its layout alone: a heading's and the Psalm title's gaps are
// four-per-em spaces on the justified page (HEADING_GAP), and a heading or a
// title that opens on a right-to-left letter opens on a left-to-right mark
// (PARA_LTR_MARK). Copy gives the reader the text as read (copyAsRead). A
// drag, which the platform starts on a long press inside the selection
// (Editor.performLongClick -> startDragAndDrop), took the view's own text,
// helpers and all, and so does an item that hands the selection to another
// app (Intent.ACTION_PROCESS_TEXT, such as Translate), which the platform
// fills with the view's own text (TextView.getSelectedText).
//
// Now Copy, the drag a long-click listener starts (dragAsRead) and the
// click of such an item, which a listener set on each in
// onPrepareActionMode takes over (processTextAsRead), take their text from
// one place, selectionAsRead (through selectionTextAsRead for the item). The
// drag hands the platform back its own drag, untouched, for a selection that
// has nothing to give back; the item takes every click, so each app gets its
// text one way, and says the text cannot be replaced. That the toolbar
// offers such items at all, and the accessibility actions that do the same,
// are TestAndroidReadingToolbarOffersOtherAppsTextActions's.
//
// The pane is Java behind JNI, so this holds the bridge's source to the rule
// line by line and proves on copies that each check can fail. On the bridge
// before the drag was cleaned every check fails, since none of these paths
// existed; on the bridge before Translate was offered the item's checks
// fail, since it handed the click to the platform, with the view's text,
// for a selection with nothing to clean, and set nothing read-only.
func TestAndroidDragAndTranslateCarryTheTextAsRead(t *testing.T) {
	java := readNativeSource(t, "android/BtBridge.java")

	held := func(src, head string, body ...string) bool {
		return strings.Contains(src, head) && slices.Equal(codeLines(javaBlockAfter(t, src, head)), body)
	}
	block := func(src, head string) string {
		if !strings.Contains(src, head) {
			return ""
		}
		return javaBlockAfter(t, src, head)
	}

	// The drag: the clip is selectionAsRead's text, and nothing in the path
	// reads the view's text itself. It is started where the platform starts
	// its own (a long press inside the selection while the toolbar is up),
	// with the platform's global flag, under a shadow the builder holds, and
	// the toolbar is closed after, as the platform closes it. Every other
	// long press, and a selection with nothing to give back, is left to the
	// platform (false).
	dragFn := []string{
		"private static boolean dragAsRead() {",
		"if (text == null || selectionMode == null) return false;",
		"int a = text.getSelectionStart(), b = text.getSelectionEnd();",
		"if (a < 0 || b < 0 || a == b) return false;",
		"if (touchMinOffset < Math.min(a, b) || touchMaxOffset >= Math.max(a, b)) return false;",
		"resetTouchOffsets();",
		"CharSequence read = selectionAsRead();",
		"if (read == null) return false;",
		"TextView shadow = dragShadowView(read);",
		"if (shadow == null) return true;",
		"text.startDragAndDrop(android.content.ClipData.newPlainText(null, read),",
		"new HeldShadow(shadow), null, View.DRAG_FLAG_GLOBAL);",
		"if (selectionMode != null) selectionMode.finish();",
		"return true;",
		"}",
	}
	// The shadow is the platform's: the first twenty characters on one
	// line, never a pair of surrogates cut in two, and no shadow where it
	// measures nothing, which startDragAndDrop throws for.
	shadowFn := []string{
		"private static TextView dragShadowView(CharSequence read) {",
		"int end = Math.min(read.length(), DRAG_SHADOW_MAX_CHARS);",
		"if (end < read.length() && Character.isHighSurrogate(read.charAt(end - 1))) end--;",
		"TextView shadow = new TextView(text.getContext());",
		"shadow.setTextAppearance(android.R.style.TextAppearance_Large);",
		"shadow.setTextColor(text.getTextColors());",
		"shadow.setSingleLine(true);",
		"shadow.setGravity(Gravity.CENTER);",
		"shadow.setText(read.subSequence(0, end));",
		"int any = View.MeasureSpec.makeMeasureSpec(0, View.MeasureSpec.UNSPECIFIED);",
		"shadow.measure(any, any);",
		"int w = shadow.getMeasuredWidth(), h = shadow.getMeasuredHeight();",
		"if (w <= 0 || h <= 0) return null;",
		"shadow.layout(0, 0, w, h);",
		"return shadow;",
		"}",
	}
	// The builder keeps the shadow: the platform's holds its view weakly.
	heldShadowFn := []string{
		"private static final class HeldShadow extends View.DragShadowBuilder {",
		"private final View held;",
		"HeldShadow(View v) {",
		"super(v);",
		"held = v;",
		"}",
		"}",
	}
	// Inside the selection is the platform's test on the platform's own
	// record of the touch (Editor.SelectionModifierCursorController): the
	// lowest and the highest offset under its fingers, set where the first
	// goes down, widened by each finger that goes down or lifts on a screen
	// that tells fingers apart, and cleared where the platform clears its
	// own: after the drag (dragFn above), when the view loses focus, when its
	// text changes, and when it is attached to a window. A record that
	// outlived its touch would let a later long press with no new touch, from
	// a keyboard or an accessibility service, drag where the platform would
	// not.
	noteFn := []string{
		"private static void noteTouch(TextView v, android.view.MotionEvent ev) {",
		"switch (ev.getActionMasked()) {",
		"case android.view.MotionEvent.ACTION_DOWN:",
		"touchMinOffset = touchMaxOffset = v.getOffsetForPosition(ev.getX(), ev.getY());",
		"break;",
		"case android.view.MotionEvent.ACTION_POINTER_DOWN:",
		"case android.view.MotionEvent.ACTION_POINTER_UP:",
		"if (!v.getContext().getPackageManager().hasSystemFeature(",
		"PackageManager.FEATURE_TOUCHSCREEN_MULTITOUCH_DISTINCT)) break;",
		"for (int i = 0; i < ev.getPointerCount(); i++) {",
		"int o = v.getOffsetForPosition(ev.getX(i), ev.getY(i));",
		"if (o < touchMinOffset) touchMinOffset = o;",
		"if (o > touchMaxOffset) touchMaxOffset = o;",
		"}",
		"break;",
		"default:",
		"break;",
		"}",
		"}",
	}
	resetFn := []string{
		"private static void resetTouchOffsets() {",
		"touchMinOffset = touchMaxOffset = -1;",
		"}",
	}
	// The reading view's own overrides, each found inside the view's
	// anonymous class.
	touches := func(src string) bool {
		v := block(src, "text = new TextView(activity)")
		return held(src, "private static void noteTouch(", noteFn...) &&
			held(src, "private static void resetTouchOffsets()", resetFn...) &&
			hasLinesInOrder(src, "private static int touchMinOffset = -1, touchMaxOffset = -1;") &&
			held(v, "@Override public boolean onTouchEvent(android.view.MotionEvent ev)",
				"@Override public boolean onTouchEvent(android.view.MotionEvent ev) {",
				"noteTouch(this, ev);",
				"return super.onTouchEvent(ev);",
				"}") &&
			held(v, "@Override protected void onFocusChanged(",
				"@Override protected void onFocusChanged(boolean focused, int direction, Rect previous) {",
				"super.onFocusChanged(focused, direction, previous);",
				"if (!focused) resetTouchOffsets();",
				"}") &&
			held(v, "@Override protected void onTextChanged(",
				"@Override protected void onTextChanged(CharSequence t, int start, int before, int after) {",
				"super.onTextChanged(t, start, before, after);",
				"resetTouchOffsets();",
				"}") &&
			held(v, "@Override protected void onAttachedToWindow()",
				"@Override protected void onAttachedToWindow() {",
				"super.onAttachedToWindow();",
				"resetTouchOffsets();",
				"}") &&
			// The four clears are the drag's and the view's three, and the
			// record is read from the view's touches alone.
			strings.Count(src, "resetTouchOffsets();") == 4 &&
			strings.Count(src, "noteTouch(") == 2
	}
	drags := func(src string) bool {
		listener := block(src, "text.setOnLongClickListener(new View.OnLongClickListener()")
		return held(src, "private static boolean dragAsRead()", dragFn...) &&
			held(src, "private static TextView dragShadowView(", shadowFn...) &&
			held(src, "private static final class HeldShadow", heldShadowFn...) &&
			hasLinesInOrder(src, "private static final int DRAG_SHADOW_MAX_CHARS = 20;") &&
			touches(src) &&
			hasLinesInOrder(listener,
				"@Override public boolean onLongClick(View v) {",
				"return dragAsRead();") &&
			// The listener is on the reading view, set once.
			strings.Count(src, "setOnLongClickListener(") == 1 &&
			strings.Count(src, "dragAsRead()") == 2 // the declaration and the listener's call
	}
	if !drags(java) {
		t.Error("a long press inside the selection must drag selectionAsRead's text: the reading " +
			"view's long-click listener answers dragAsRead, which tests the offsets under the " +
			"touch's fingers (noteTouch) as the platform does, clears them where the platform " +
			"clears its own (resetTouchOffsets), and starts the drag with that text and the " +
			"shadow held by HeldShadow")
	}

	// An app that offers to act on selected text, such as Translate: each
	// such item gets a listener when the menu is prepared, and the listener
	// sends the selection as Copy gives it (selectionTextAsRead:
	// selectionAsRead's text, or the view's own where that reads the same) in
	// a copy of the item's own intent, marked read-only, started for a
	// result. It takes every such click, so no app is ever handed the view's
	// text by the platform, and one with nothing to send ends there.
	handFn := []string{
		"private static void handProcessTextAsRead(Menu menu) {",
		"for (int i = 0; i < menu.size(); i++) {",
		"MenuItem item = menu.getItem(i);",
		"Intent in = item.getIntent();",
		"if (in == null || !Intent.ACTION_PROCESS_TEXT.equals(in.getAction())) continue;",
		"item.setOnMenuItemClickListener(new MenuItem.OnMenuItemClickListener() {",
		"@Override public boolean onMenuItemClick(MenuItem it) {",
		"return processTextAsRead(it.getIntent());",
		"}",
		"});",
		"}",
		"}",
	}
	textFn := []string{
		"private static String selectionTextAsRead() {",
		"if (text == null) return null;",
		"int a = text.getSelectionStart(), b = text.getSelectionEnd();",
		"if (a < 0 || b < 0 || a == b) return null;",
		"CharSequence read = selectionAsRead();",
		"if (read == null) read = text.getText().subSequence(Math.min(a, b), Math.max(a, b));",
		"return read.toString();",
		"}",
	}
	processFn := []string{
		"private static boolean processTextAsRead(Intent in) {",
		"if (in == null || !Intent.ACTION_PROCESS_TEXT.equals(in.getAction())) return false;",
		"String sent = selectionTextAsRead();",
		"if (sent == null || !(text.getContext() instanceof Activity)) return true;",
		"if (sent.length() > PARCEL_SAFE_TEXT_LENGTH) {",
		"int end = PARCEL_SAFE_TEXT_LENGTH;",
		"if (Character.isHighSurrogate(sent.charAt(end - 1))) end--;",
		"sent = sent.substring(0, end);",
		"}",
		"Intent out = new Intent(in);",
		"out.putExtra(Intent.EXTRA_PROCESS_TEXT_READONLY, true);",
		"out.putExtra(Intent.EXTRA_PROCESS_TEXT, sent);",
		"try {",
		"((Activity) text.getContext()).startActivityForResult(out, PROCESS_TEXT_REQUEST);",
		"} catch (Throwable ignored) {",
		"}",
		"return true;",
		"}",
	}
	// Calls are counted in code, not in comments, which name the functions.
	code := func(src string) string { return strings.Join(codeLines(src), "\n") }
	sends := func(src string) bool {
		// The listeners are set first in onPrepareActionMode, before
		// anything there that returns, so every preparing of the toolbar
		// sets them, whatever Study with AI does there.
		prepare := codeLines(block(src, "@Override public boolean onPrepareActionMode(ActionMode mode, Menu menu)"))
		return held(src, "private static void handProcessTextAsRead(", handFn...) &&
			held(src, "private static String selectionTextAsRead()", textFn...) &&
			held(src, "private static boolean processTextAsRead(", processFn...) &&
			hasLinesInOrder(src, "private static final int PARCEL_SAFE_TEXT_LENGTH = 100000;") &&
			len(prepare) >= 4 &&
			slices.Equal(prepare[:2], []string{
				"@Override public boolean onPrepareActionMode(ActionMode mode, Menu menu) {",
				"handProcessTextAsRead(menu);",
			}) &&
			strings.Count(code(src), "handProcessTextAsRead(") == 2 && // the declaration and its one call
			strings.Count(code(src), "selectionTextAsRead()") == 2 && // the declaration and processTextAsRead's call
			// The declaration, the toolbar listener's call and the
			// accessibility action's.
			strings.Count(code(src), "processTextAsRead(") == 3
	}
	if !sends(java) {
		t.Error("an item that hands the selection to another app (ACTION_PROCESS_TEXT, Translate) " +
			"must send the selection as Copy gives it: onPrepareActionMode gives each such item a " +
			"listener (handProcessTextAsRead) that starts the item's intent, marked read-only, with " +
			"selectionTextAsRead's text (processTextAsRead), for every selection")
	}

	// The request code the app is started with is not one Fyne's activity
	// reads as its file pickers' answer. The generated toolkit is not
	// tracked, so this half runs where scripts/setup-fyne-patch.sh has made it.
	m := regexp.MustCompile(`private static final int PROCESS_TEXT_REQUEST = (\d+);`).FindStringSubmatch(java)
	if m == nil {
		t.Error("the bridge declares no PROCESS_TEXT_REQUEST")
	} else if act, err := os.ReadFile(filepath.Join(repoRoot(t), "third_party", "fyne", "internal",
		"driver", "mobile", "app", "GoNativeActivity.java")); err == nil {
		codes := regexp.MustCompile(`private static final int (FILE_\w+_CODE) = (\d+);`).FindAllStringSubmatch(string(act), -1)
		if len(codes) < 2 {
			t.Errorf("the generated GoNativeActivity declares %d file picker codes, not 2; read what its "+
				"onActivityResult takes before trusting PROCESS_TEXT_REQUEST", len(codes))
		}
		for _, c := range codes {
			if c[2] == m[1] {
				t.Errorf("PROCESS_TEXT_REQUEST is %s, the activity's %s: a result from the app the "+
					"selection was sent to would be read as a file picker's", m[1], c[1])
			}
		}
	} else {
		t.Log("third_party/fyne is absent (scripts/setup-fyne-patch.sh makes it); the request code " +
			"is not checked against the activity's own")
	}
	if m != nil {
		if n, _ := strconv.Atoi(m[1]); n == 1 || n == 2 {
			t.Errorf("PROCESS_TEXT_REQUEST is %d, a code Fyne's activity reads for its file pickers", n)
		}
	}

	// The one source: neither path reads the view's text for itself, so the
	// text that leaves cannot be anything but selectionAsRead's, or, for the
	// item, the view's own only where selectionAsRead says it reads the same,
	// as the platform's Copy takes it then.
	oneSource := func(src string) bool {
		clean := func(b string) bool {
			return !strings.Contains(b, "getSelectedText") && !strings.Contains(b, "readerText(")
		}
		drag := block(src, "private static boolean dragAsRead()")
		item := block(src, "private static boolean processTextAsRead(")
		asText := block(src, "private static String selectionTextAsRead()")
		if drag == "" || item == "" || asText == "" {
			return false
		}
		var fromView []string
		for _, l := range codeLines(asText) {
			if strings.Contains(l, "getText()") {
				fromView = append(fromView, l)
			}
		}
		return clean(drag) && clean(item) && clean(asText) &&
			strings.Contains(drag, "CharSequence read = selectionAsRead();") && !strings.Contains(drag, "getText()") &&
			strings.Contains(item, "String sent = selectionTextAsRead();") && !strings.Contains(item, "getText()") &&
			hasLinesInOrder(asText, "CharSequence read = selectionAsRead();",
				"if (read == null) read = text.getText().subSequence(Math.min(a, b), Math.max(a, b));",
				"return read.toString();") &&
			slices.Equal(fromView, []string{"if (read == null) read = text.getText().subSequence(Math.min(a, b), Math.max(a, b));"})
	}
	if !oneSource(java) {
		t.Error("the drag and the item that hands the selection to another app must take their text " +
			"from selectionAsRead, the item through selectionTextAsRead, which takes the view's own " +
			"only where selectionAsRead reads the same, and from nothing else")
	}

	for _, c := range []struct {
		name, from, to string
		check          func(string) bool
	}{
		// The drag.
		{"the drag's clip from the view", "android.content.ClipData.newPlainText(null, read),\n",
			"android.content.ClipData.newPlainText(null, text.getText().subSequence(Math.min(a, b), Math.max(a, b))),\n", oneSource},
		{"the drag's text not selectionAsRead's", "        CharSequence read = selectionAsRead();\n        if (read == null) return false;\n        TextView shadow",
			"        CharSequence read = readerText(text.getText(), Math.min(a, b), Math.max(a, b));\n        if (read == null) return false;\n        TextView shadow", oneSource},
		{"no long-click listener", "                return dragAsRead();\n", "                return false;\n", drags},
		{"a drag from outside the selection", "        if (touchMinOffset < Math.min(a, b) || touchMaxOffset >= Math.max(a, b)) return false;\n", "", drags},
		{"a drag from the selection's end", "touchMaxOffset >= Math.max(a, b)", "touchMaxOffset > Math.max(a, b)", drags},
		{"a drag with one finger outside", "if (touchMinOffset < Math.min(a, b) || touchMaxOffset >= Math.max(a, b)) return false;",
			"if (touchMinOffset < Math.min(a, b) || touchMinOffset >= Math.max(a, b)) return false;", drags},
		{"a drag with no toolbar up", "if (text == null || selectionMode == null) return false;",
			"if (text == null) return false;", drags},
		{"the platform's drag as well", "        if (selectionMode != null) selectionMode.finish();\n        return true;\n    }\n\n    // dragShadowView",
			"        if (selectionMode != null) selectionMode.finish();\n        return false;\n    }\n\n    // dragShadowView", drags},
		{"the toolbar left up", "        if (selectionMode != null) selectionMode.finish();\n        return true;\n    }\n\n    // dragShadowView",
			"        return true;\n    }\n\n    // dragShadowView", drags},
		{"a drag only this app may take", "new HeldShadow(shadow), null, View.DRAG_FLAG_GLOBAL);",
			"new HeldShadow(shadow), null, 0);", drags},
		{"the platform's weak shadow", "new HeldShadow(shadow), null,", "new View.DragShadowBuilder(shadow), null,", drags},
		{"a shadow that keeps nothing", "            held = v;\n", "", drags},
		{"a shadow that measures nothing", "        if (w <= 0 || h <= 0) return null;\n", "", drags},
		{"the touch never read", "                noteTouch(this, ev);\n", "", drags},
		{"the offsets taken again on every move", "            case android.view.MotionEvent.ACTION_DOWN:\n",
			"            case android.view.MotionEvent.ACTION_DOWN:\n            case android.view.MotionEvent.ACTION_MOVE:\n", drags},
		{"the first finger alone", "            case android.view.MotionEvent.ACTION_POINTER_DOWN:\n            case android.view.MotionEvent.ACTION_POINTER_UP:\n",
			"", drags},
		{"every finger on a screen that cannot tell them apart", "                if (!v.getContext().getPackageManager().hasSystemFeature(\n                        PackageManager.FEATURE_TOUCHSCREEN_MULTITOUCH_DISTINCT)) break;\n",
			"", drags},
		{"the offsets kept after a drag", "        resetTouchOffsets();\n        CharSequence read = selectionAsRead();\n        if (read == null) return false;\n        TextView shadow",
			"        CharSequence read = selectionAsRead();\n        if (read == null) return false;\n        TextView shadow", drags},
		{"the offsets kept for the platform's drag", "        resetTouchOffsets();\n        CharSequence read = selectionAsRead();\n        if (read == null) return false;\n        TextView shadow",
			"        CharSequence read = selectionAsRead();\n        if (read == null) return false;\n        resetTouchOffsets();\n        TextView shadow", drags},
		{"the offsets kept when the view loses focus", "                if (!focused) resetTouchOffsets();\n", "", drags},
		{"the offsets kept when the text changes", "super.onTextChanged(t, start, before, after);\n                resetTouchOffsets();\n",
			"super.onTextChanged(t, start, before, after);\n", drags},
		{"the offsets kept when the view is attached", "super.onAttachedToWindow();\n                resetTouchOffsets();\n",
			"super.onAttachedToWindow();\n", drags},
		{"the offsets cleared by half", "        touchMinOffset = touchMaxOffset = -1;\n", "        touchMinOffset = -1;\n", drags},
		{"the offsets read on another view", "text = new TextView(activity) {\n            @Override public boolean performLongClick() {",
			"text = new TextView(activity) {\n        };\n        new TextView(activity) {\n            @Override public boolean performLongClick() {", drags},
		// Translate and its like.
		{"the item's text from the view", "out.putExtra(Intent.EXTRA_PROCESS_TEXT, sent);",
			"out.putExtra(Intent.EXTRA_PROCESS_TEXT, text.getText().toString());", oneSource},
		{"the item's text not Copy's", "        String sent = selectionTextAsRead();\n",
			"        String sent = text.getText().toString();\n", oneSource},
		{"the view's text first", "        CharSequence read = selectionAsRead();\n        if (read == null) read = text.getText()",
			"        CharSequence read = text.getText().subSequence(0, 0);\n        if (read != null) read = text.getText()", oneSource},
		{"the view's text, helpers and all", "        if (read == null) read = text.getText().subSequence(Math.min(a, b), Math.max(a, b));\n        return read.toString();",
			"        read = text.getText().subSequence(Math.min(a, b), Math.max(a, b));\n        return read.toString();", oneSource},
		{"no listener set", "                handProcessTextAsRead(menu);\n", "", sends},
		{"onPrepareActionMode as it was", "            @Override public boolean onPrepareActionMode(ActionMode mode, Menu menu) {\n                handProcessTextAsRead(menu);\n                MenuItem ai = menu.findItem(200);\n                if (ai == null) return false;\n                boolean show = studyOffered();\n                if (ai.isVisible() == show) return false;\n                ai.setVisible(show);\n                return true;\n            }",
			"            @Override public boolean onPrepareActionMode(ActionMode mode, Menu menu) { return false; }", sends},
		{"the listeners set only where Study with AI changes",
			"                handProcessTextAsRead(menu);\n                MenuItem ai = menu.findItem(200);\n                if (ai == null) return false;\n                boolean show = studyOffered();\n                if (ai.isVisible() == show) return false;\n",
			"                MenuItem ai = menu.findItem(200);\n                if (ai == null) return false;\n                boolean show = studyOffered();\n                if (ai.isVisible() == show) return false;\n                handProcessTextAsRead(menu);\n", sends},
		{"the listeners set only with the assistant on",
			"                handProcessTextAsRead(menu);\n                MenuItem ai = menu.findItem(200);\n                if (ai == null) return false;\n",
			"                MenuItem ai = menu.findItem(200);\n                if (ai == null) return false;\n                handProcessTextAsRead(menu);\n", sends},
		{"the platform's send as well", "            // Nothing started; the click ends here.\n        }\n        return true;",
			"            // Nothing started; the click ends here.\n        }\n        return false;", sends},
		{"the platform's send for a selection with nothing to clean",
			"        String sent = selectionTextAsRead();\n        if (sent == null || !(text.getContext() instanceof Activity)) return true;\n",
			"        CharSequence read = selectionAsRead();\n        if (read == null || !(text.getContext() instanceof Activity)) return false;\n        String sent = read.toString();\n", sends},
		{"the platform's send with nothing selected", "if (sent == null || !(text.getContext() instanceof Activity)) return true;",
			"if (sent == null || !(text.getContext() instanceof Activity)) return false;", sends},
		{"every item taken over", "if (in == null || !Intent.ACTION_PROCESS_TEXT.equals(in.getAction())) continue;",
			"if (in == null) continue;", sends},
		{"the item's intent not copied", "Intent out = new Intent(in);", "Intent out = new Intent(Intent.ACTION_PROCESS_TEXT);", sends},
		{"not said to be read-only", "        out.putExtra(Intent.EXTRA_PROCESS_TEXT_READONLY, true);\n", "", sends},
		{"said to be replaceable", "out.putExtra(Intent.EXTRA_PROCESS_TEXT_READONLY, true);",
			"out.putExtra(Intent.EXTRA_PROCESS_TEXT_READONLY, false);", sends},
		{"started without a result", "startActivityForResult(out, PROCESS_TEXT_REQUEST);", "startActivity(out);", sends},
		{"no cut for a long selection", "        if (sent.length() > PARCEL_SAFE_TEXT_LENGTH) {\n            int end = PARCEL_SAFE_TEXT_LENGTH;\n            if (Character.isHighSurrogate(sent.charAt(end - 1))) end--;\n            sent = sent.substring(0, end);\n        }\n",
			"", sends},
		{"the listener not sending the clicked item's intent", "return processTextAsRead(it.getIntent());",
			"return processTextAsRead(new Intent(Intent.ACTION_PROCESS_TEXT));", sends},
	} {
		if k := strings.Count(java, c.from); k != 1 {
			t.Fatalf("control %q: the bridge contains %q %d times, not once", c.name, c.from, k)
		}
		if c.check(strings.Replace(java, c.from, c.to, 1)) {
			t.Errorf("control %q: the check passes a bridge without it, so it proves nothing", c.name)
		}
	}
}

// THE DRAG'S LISTENER LEAVES THE READING TEXT'S ACCESSIBILITY NODE AS THE
// PLATFORM SETS IT. From Android 13 the platform tells accessibility services
// that selectable text it cannot edit takes no long click, unless a
// long-click listener is set (TextView.onInitializeAccessibilityNodeInfoInternal).
// The listener the drag is taken from changed that: on the API 35 emulator
// the reading view's node read long-clickable with the listener and not
// without it. The view's onInitializeAccessibilityNodeInfo takes the long
// click back out where the platform would, on Android 13 and later, and
// leaves Android 12 and earlier, where the platform offers it, as they were.
// The node's last line puts the view's own actions for other apps' text
// actions in it, which TestAndroidReadingToolbarOffersOtherAppsTextActions
// holds.
func TestAndroidDragListenerKeepsThePlatformsAccessibilityNode(t *testing.T) {
	java := readNativeSource(t, "android/BtBridge.java")
	const head = "@Override public void onInitializeAccessibilityNodeInfo("
	node := []string{
		head,
		"android.view.accessibility.AccessibilityNodeInfo info) {",
		"super.onInitializeAccessibilityNodeInfo(info);",
		"if (Build.VERSION.SDK_INT >= 33 && isTextSelectable()) {",
		"info.setLongClickable(false);",
		"info.removeAction(android.view.accessibility.AccessibilityNodeInfo.AccessibilityAction.ACTION_LONG_CLICK);",
		"}",
		"processTextActions(this, info);",
		"}",
	}
	hides := func(src string) bool {
		if !strings.Contains(src, "text = new TextView(activity)") {
			return false
		}
		// Inside the reading view's own class, and nowhere else.
		v := javaBlockAfter(t, src, "text = new TextView(activity)")
		return strings.Contains(v, head) &&
			slices.Equal(codeLines(javaBlockAfter(t, v, head)), node) &&
			strings.Count(src, head) == 1
	}
	if !hides(java) {
		t.Error("the reading view must override onInitializeAccessibilityNodeInfo to take the long " +
			"click out of its node on Android 13 and later, as the platform does for selectable text " +
			"with no long-click listener")
	}
	for _, c := range []struct{ name, from, to string }{
		{"no override", head, "public void unusedNodeInfo("},
		{"the platform's node not built first", "                super.onInitializeAccessibilityNodeInfo(info);\n", ""},
		{"from Android 12", "Build.VERSION.SDK_INT >= 33 && isTextSelectable()", "Build.VERSION.SDK_INT >= 31 && isTextSelectable()"},
		{"on every Android", "Build.VERSION.SDK_INT >= 33 && isTextSelectable()", "isTextSelectable()"},
		{"the flag left as the listener sets it", "                    info.setLongClickable(false);\n", ""},
		{"the action left in", "                    info.removeAction(android.view.accessibility.AccessibilityNodeInfo.AccessibilityAction.ACTION_LONG_CLICK);\n", ""},
		{"on another view", "text = new TextView(activity) {\n            @Override public boolean performLongClick() {",
			"text = new TextView(activity) {\n        };\n        new TextView(activity) {\n            @Override public boolean performLongClick() {"},
	} {
		if k := strings.Count(java, c.from); k != 1 {
			t.Fatalf("control %q: the bridge contains %q %d times, not once", c.name, c.from, k)
		}
		if hides(strings.Replace(java, c.from, c.to, 1)) {
			t.Errorf("control %q: the check passes a bridge without it, so it proves nothing", c.name)
		}
	}
}

// THE APPLE PANES CARRY NEITHER OF ANDROID'S LAYOUT CHARACTERS, so their
// native drag (the iPad's and the Mac's text views) and the system's
// Translate (the iPhone's and the iPad's menus) hand on the same text their
// Copy does, with nothing to clean. The iPhone, iPad and Mac panes import the
// Apple dialect, which sets a heading and the Psalm title ragged by its
// stylesheet and writes no character for the page's layout; the plain-text
// fallback page is built from the same HTML; and the panes' Objective-C
// writes no character into the page. The Android dialect is the control: it
// marks the same heading and title, so the fixture does reach the rule.
func TestTheApplePanesCarryNoAndroidLayoutCharacter(t *testing.T) {
	st := sampleState()
	ch := st.Bible.GetChapter(st.CurrentBook, st.CurrentChapter)
	st.Bible.Headings = map[string]map[int][]Heading{
		st.CurrentBook: {st.CurrentChapter: {
			{Text: "א Fixture", Style: "qa", BeforeVerse: ch[0].Verse},
			{Text: "A Fixture Heading That Runs On", Style: "s1", BeforeVerse: ch[len(ch)-1].Verse},
		}},
	}
	psalm := []Verse{{BookName: "Psalms", Chapter: 150, Verse: 1, Text: "A fixture verse."}}
	helpers := []string{"\u2005", "\u200e", "&#x2005;", "&#x200E;", "&#x200e;", "&#8197;", "&#8206;"}
	carries := func(s string) []string {
		var out []string
		for _, h := range helpers {
			if strings.Contains(s, h) {
				out = append(out, strconv.QuoteToASCII(h))
			}
		}
		return out
	}
	for _, book := range []bool{false, true} {
		var apple, android, applePsalm, androidPsalm string
		withReporterLayout(book, func() {
			apple = buildChapterHTML(st, ch)
			android = buildChapterHTMLAndroid(st, ch)
			ps := sampleState()
			ps.CurrentBook, ps.CurrentChapter = "Psalms", 150
			ps.Bible.Verses["Psalms"] = map[int][]Verse{150: psalm}
			ps.Bible.Superscriptions = map[string]map[int]Superscription{"Psalms": {150: {Text: "א fixture title."}}}
			applePsalm = buildChapterHTML(ps, ps.Bible.GetChapter("Psalms", 150))
			androidPsalm = buildChapterHTMLAndroid(ps, ps.Bible.GetChapter("Psalms", 150))
		})
		if len(carries(android)) == 0 || len(carries(androidPsalm)) == 0 {
			t.Fatalf("control (book page %v): the Android dialect gives the fixture's heading and title "+
				"no direction mark, so the fixture does not reach the rule", book)
		}
		for name, page := range map[string]string{
			"the imported page":         apple,
			"the imported psalm":        applePsalm,
			"the fallback page":         appleFallbackPageOf(apple).text,
			"the fallback page's psalm": appleFallbackPageOf(applePsalm).text,
		} {
			if got := carries(page); len(got) > 0 {
				t.Errorf("the Apple panes' %s (book page %v) carries %v; its drag and Translate would "+
					"hand them on uncleaned, where Android's drag and Copy give the text as read", name, book, got)
			}
		}
	}
	// And the native side writes no character into the page it shows: neither
	// character in any spelling its Objective-C or Go could give it (the
	// character itself, 0x2005, \u2005 or \U00002005, in either case, or the
	// decimal code), and no call that edits the page's text.
	writes := func(src string) []string {
		var out []string
		lower := strings.ToLower(src)
		for _, w := range []string{"\u2005", "\u200e", "0x2005", "0x200e", `\u2005`, `\u200e`, `\u00002005`, `\u0000200e`} {
			if strings.Contains(lower, w) {
				out = append(out, strconv.QuoteToASCII(w))
			}
		}
		if m := regexp.MustCompile(`\b(8197|8206)\b`).FindString(src); m != "" {
			out = append(out, m)
		}
		for _, w := range []string{"replaceCharactersInRange", "insertString", "insertAttributedString", "deleteCharactersInRange"} {
			if strings.Contains(src, w) {
				out = append(out, w)
			}
		}
		return out
	}
	ios := readNativeSource(t, "reading_ios.go")
	for path, src := range map[string]string{"reading_ios.go": ios, "reading_macos.go": readNativeSource(t, "reading_macos.go")} {
		if got := writes(src); len(got) > 0 {
			t.Errorf("%s holds %v: if it writes a layout character into the page, the native drag "+
				"and Translate hand it on, and the parity this test records no longer holds", path, got)
		}
	}
	// Controls: each spelling, planted in a copy of the iPhone and iPad
	// pane's source, is caught.
	for _, plant := range []string{
		`NSString *mark = @"\u200E";`,
		`NSString *mark = @"\U0000200e";`,
		`NSString *gap = @"\u2005";`,
		"NSString *mark = @\"\u200e\";",
		"NSString *gap = @\"\u2005\";",
		`unichar gap = 0x2005;`,
		`unichar mark = 0x200E;`,
		`unichar mark = 8206;`,
		`[storage replaceCharactersInRange:r withString:s];`,
	} {
		if len(writes(ios+"\n"+plant+"\n")) == 0 {
			t.Errorf("control: %s planted in reading_ios.go is not caught, so the check proves nothing for it",
				strconv.QuoteToASCII(plant))
		}
	}
}
