package bibletext

import (
	"regexp"
	"strings"
	"testing"
)

// The Android Note and Ask editor is Java behind the JNI boundary. These
// source contracts complement the executable lifecycle cases in
// note_entry_android_lifecycle_test.go; neither replaces device rendering,
// input-method or accessibility checks.

// javaMember is the source of the member of src whose declaration begins
// with decl, up to the first line that closes a member at four spaces.
func javaMember(t *testing.T, src, file, decl string) string {
	t.Helper()
	i := strings.Index(src, decl)
	if i < 0 {
		t.Fatalf("%s has no %q", file, decl)
	}
	body := src[i:]
	end := strings.Index(body, "\n    }\n")
	if end < 0 {
		t.Fatalf("%s: %q has no end", file, decl)
	}
	return body[:end]
}

const noteEntryJava = "android/BtNoteEntry.java"

// Android uses 6-unit horizontal padding and at most 10 units vertically.
// UIKit has additional line-fragment padding, so this is not an effective
// horizontal-inset parity check. Vertical padding shrinks with the font and
// frame, but a frame shorter than a font row can still clip the glyphs. These
// structural checks do not measure pixels or guarantee a complete line.
func TestNativeAndroidEditorPaddingUsesFontAndFrame(t *testing.T) {
	java := readNativeSource(t, noteEntryJava)
	pad := javaMember(t, java, noteEntryJava, "private static void applyPadding(EditText field, int frameHeight)")
	for _, want := range []string{"getFontMetrics()", "frameHeight", "fm.descent", "fm.ascent", "setPadding("} {
		if !strings.Contains(pad, want) {
			t.Errorf("applyPadding does not use %s: the vertical inset is no longer measured from the font and the frame", want)
		}
	}
	if !regexp.MustCompile(`Math\.min\(\s*Math\.round\(10\s*\*\s*scale\)\s*,`).MatchString(pad) {
		t.Error("applyPadding does not bound its 10-unit vertical inset by the room a font row leaves")
	}
	if !regexp.MustCompile(`setPadding\(\s*Math\.round\(6\s*\*\s*scale\)`).MatchString(pad) {
		t.Error("applyPadding does not use its 6-unit side padding")
	}
	if n := strings.Count(java, "setPadding("); n != 1 {
		t.Errorf("%s sets padding in %d places; an inset beside applyPadding can again leave the line less room than it needs", noteEntryJava, n)
	}
	for _, caller := range []string{"private static void style(EditText field)", "private static void applyFrame()"} {
		if !strings.Contains(javaMember(t, java, noteEntryJava, caller), "applyPadding(") {
			t.Errorf("%s does not apply the padding, so the frame it gives the field is not the one the inset was measured for", caller)
		}
	}
	if !strings.Contains(javaMember(t, java, noteEntryJava, "private static boolean ensureView()"), "style(field);") {
		t.Error("ensureView does not style the field it makes")
	}
}

// A card reopened over the one a rebuild drained (a rotation, a light/dark
// change) takes over the editor window that is showing. Removing that window,
// the input method's target while the keyboard is up, and adding another in
// its place while the display rotated left the new window layered to the
// keyboard's container, which was still layered to the removed one, so
// neither was composited: the field was not drawn, never took input focus,
// and the next key ended in an input-dispatch ANR. So show detaches only a
// window that is no longer live, a kept field is styled for the new card, and
// the field's callbacks act for the owner when they run, not for the one that
// made the view.
func TestNativeAndroidEditorKeepsItsWindowForAReopenedCard(t *testing.T) {
	java := readNativeSource(t, noteEntryJava)
	show := javaMember(t, java, noteEntryJava, "static void show(")
	if !strings.Contains(show, "dialog.isShowing()") || !strings.Contains(show, "if (!live) detach();") {
		t.Error("show does not keep a live editor window: a reopened card removes the window and adds another")
	}
	if n := strings.Count(show, "detach();"); n != 1 {
		t.Errorf("show detaches the editor in %d places; only a window that is no longer live is let go", n)
	}
	if !strings.Contains(show, "style(editor);") {
		t.Error("a kept field is not styled for the card reopened over it")
	}
	view := javaMember(t, java, noteEntryJava, "private static boolean ensureView()")
	for _, stale := range []string{"final long token", "owner != token", "owner == token"} {
		if strings.Contains(view, stale) {
			t.Errorf("ensureView's callbacks hold %q: a kept window would drop the edits and the Back of the card reopened over it", stale)
		}
	}
}

// Native layout remains shared; Android installs only its keyboard-area and
// owner-bound focus seams. iOS leaves the focus seam as the shared no-op.
func TestAndroidEditorTakesTheSharedComposerRules(t *testing.T) {
	src := readNativeSource(t, "note_entry_android.go")
	i := strings.Index(src, "\nfunc init() {\n")
	if i < 0 {
		t.Fatal("note_entry_android.go has no init, so this test reads nothing")
	}
	body := src[i:]
	if end := strings.Index(body, "\n}\n"); end >= 0 {
		body = body[:end]
	}
	var assigned []string
	for _, m := range regexp.MustCompile(`(?m)^\t(\w+)\s*=`).FindAllStringSubmatch(body, -1) {
		assigned = append(assigned, m[1])
	}
	if strings.Join(assigned, ",") != "noteEntryKeyboard,nativeComposeArea" {
		t.Errorf("Android init assigns %v; only keyboard focus and area differ from the shared composer", assigned)
	}
	if !strings.Contains(body, "androidComposeDraft.read(owner)") || !strings.Contains(body, "androidNoteEntryFocus(owner, show)") {
		t.Error("the keyboard seam does not retain the requesting card's owner")
	}
	if strings.Contains(readNativeSource(t, "note_entry_ios.go"), "noteEntryKeyboard =") {
		t.Error("the Android keyboard policy also changes the iOS keyboard policy")
	}
}

func TestAndroidEditorKeyboardFocusIsOwnerBoundAcrossJNI(t *testing.T) {
	java := readNativeSource(t, noteEntryJava)
	focus := javaMember(t, java, noteEntryJava, "static void focus(")
	for _, want := range []string{"UI.post(", "expected != owner || !wanted", "focusWanted = focus;", "focusNow(focus);"} {
		if !strings.Contains(focus, want) {
			t.Errorf("native keyboard focus lacks %q", want)
		}
	}
	bridge := readNativeSource(t, "android/BtBridge.java")
	if !strings.Contains(bridge, "noteEntryFocus(long owner, boolean focus) { BtNoteEntry.focus(owner, focus); }") {
		t.Error("the bridge does not forward the requesting owner with keyboard focus")
	}
	jni := readNativeSource(t, "reading_android.go")
	for _, want := range []string{`"noteEntryFocus", "(JZ)V"`, "btaNoteEntryFocusM == NULL", "btaNoteEntryFocusM, (jlong)owner, (jboolean)focus", "func androidNoteEntryFocus(owner uint64, focus bool)"} {
		if !strings.Contains(jni, want) {
			t.Errorf("the JNI focus boundary lacks %q", want)
		}
	}
}

// An editable view is labelled by its hint, which TalkBack reads for an empty
// field; a content description on it is spoken on input focus in place of the
// text, and the platform's accessibility checks report one as an error. The
// hint is set wherever the field is styled, so a field a reopened card keeps
// takes that card's. The draft is a reader's own words, declared out of
// autofill and content capture rather than left to the heuristic that keeps a
// view with no id and no autofill hints out of them.
func TestNativeAndroidEditorIsLabelledByItsHintAlone(t *testing.T) {
	java := readNativeSource(t, noteEntryJava)
	view := javaMember(t, java, noteEntryJava, "private static boolean ensureView()")
	if !strings.Contains(javaMember(t, java, noteEntryJava, "private static void style(EditText field)"), "field.setHint(placeholder);") {
		t.Error("the field has no hint, so an empty field is announced as nothing")
	}
	if strings.Contains(java, "setContentDescription(") {
		t.Error("the field carries a content description, which is spoken in place of the draft on input focus")
	}
	for _, want := range []string{
		"if (Build.VERSION.SDK_INT >= 26) field.setImportantForAutofill(View.IMPORTANT_FOR_AUTOFILL_NO);",
		"if (Build.VERSION.SDK_INT >= 29) field.setImportantForContentCapture(View.IMPORTANT_FOR_CONTENT_CAPTURE_NO);",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("ensureView lacks %q: the draft's exclusion rests on a heuristic, not a declaration", want)
		}
	}
}

// The native keyboard can offer corrections without retaining the draft in
// its personalized history. These are separate input-method settings.
func TestNativeAndroidEditorRequestsNoPersonalizedLearning(t *testing.T) {
	java := readNativeSource(t, noteEntryJava)
	view := javaMember(t, java, noteEntryJava, "private static boolean ensureView()")
	ime := regexp.MustCompile(`(?s)field\.setImeOptions\((.*?)\);`).FindStringSubmatch(view)
	if len(ime) != 2 {
		t.Fatal("the editor has no explicit input-method options")
	}
	for _, flag := range []string{"IME_FLAG_NO_EXTRACT_UI", "IME_FLAG_NO_PERSONALIZED_LEARNING"} {
		if !strings.Contains(ime[1], flag) {
			t.Errorf("the editor's input-method options omit %s", flag)
		}
	}
	input := regexp.MustCompile(`(?s)field\.setInputType\((.*?)\);`).FindStringSubmatch(view)
	if len(input) != 2 || !strings.Contains(input[1], "TYPE_TEXT_FLAG_AUTO_CORRECT") {
		t.Error("the editor no longer requests ordinary autocorrection")
	}
	if len(input) == 2 && strings.Contains(input[1], "TYPE_TEXT_FLAG_NO_SUGGESTIONS") {
		t.Error("the learning opt-out also disables ordinary suggestions")
	}
}

// Dismissing the Dialog is guarded as the reading overlay's dismiss is in
// BtBridge.java: a window the system has already taken away throws from
// the dismiss by paths that differ by release, and detach runs on the UI
// thread, where an uncaught throw ends the app.
func TestNativeAndroidEditorDetachSurvivesADismissThatThrows(t *testing.T) {
	java := readNativeSource(t, noteEntryJava)
	const guarded = "try { dialog.dismiss(); } catch (Throwable ignored) {}"
	if !strings.Contains(javaMember(t, java, noteEntryJava, "private static void detach()"), guarded) {
		t.Errorf("detach does not guard the dismiss as %q, the form the reading overlay uses", guarded)
	}
	if !strings.Contains(readNativeSource(t, "android/BtBridge.java"), guarded) {
		t.Fatal("control: BtBridge.java no longer uses that form, so this test holds the editor to nothing")
	}
}
