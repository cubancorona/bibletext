package bibletext

import (
	"encoding/xml"
	"os"
	"slices"
	"strings"
	"testing"
)

// ANDROID'S READING TOOLBAR OFFERS OTHER APPS' TEXT ACTIONS, TRANSLATE AMONG
// THEM, AND EACH IS HANDED THE TEXT AS READ (added 21 October 2026).
//
// THE DEFECT THIS EXISTS FOR. The iPhone's and the iPad's selection menus
// offer the system's Translate; Android's reading toolbar offered no
// Translate and no other app's text action at all. The platform adds those
// items (Intent.ACTION_PROCESS_TEXT) to a selection toolbar, and offers them
// as accessibility actions, only on a view with an id
// (TextView.canProcessText, the same from Android 7 to 16), and the reading
// view had none; and from Android 11 an app sees another app's text actions
// only through a queries entry for ACTION_PROCESS_TEXT in its manifest,
// which it did not declare.
//
// Now the reading view has an id, the manifest declares the entry, and
// neither the toolbar's items nor the accessibility actions reach the
// platform's own send, which fills the intent with the view's text, layout
// characters and all: the toolbar's items through a listener on each
// (handProcessTextAsRead), the accessibility actions by the view's own
// actions in their place (processTextActions, performAccessibilityAction),
// both into processTextAsRead, which
// TestAndroidDragAndTranslateCarryTheTextAsRead holds to the text Copy
// gives. The reading text is never edited: the intent says so
// (EXTRA_PROCESS_TEXT_READONLY), and nothing an app returns is read, so a
// replacement an app offers cannot reach the page. The one other way the
// platform offers a Translate on a selection, a text classifier's suggested
// actions (Editor's updateAssistMenuItems, and the accessibility actions it
// makes of them from 0x10001000, past the range processTextActions takes),
// stays shut: the platform starts those with intents the classifier builds
// from the view's own text, past processTextAsRead, and the reading view is
// given the classifier that suggests none (TextClassifier.NO_OP).
//
// The pane is Java behind JNI and the manifest is read by the packager, so
// this holds both sources to the rule and proves on copies that each check
// can fail. On the tree before the change the id, the queries entry and the
// accessibility actions are all missing.
func TestAndroidReadingToolbarOffersOtherAppsTextActions(t *testing.T) {
	java := readNativeSource(t, "android/BtBridge.java")
	block := func(src, head string) string {
		if !strings.Contains(src, head) {
			return ""
		}
		return javaBlockAfter(t, src, head)
	}
	code := func(src string) string { return strings.Join(codeLines(src), "\n") }

	// The toolbar: the reading view, and no other, is given an id where it
	// is made (the platform asks for it when the toolbar is built, on each
	// selection), and the menu callback takes no item out but the platform's
	// plain-text Share, hides none but its own Study with AI, for a
	// selection that holds no Scripture (studyOffered), and keeps the menu.
	offers := func(src string) bool {
		ensure := block(src, "private static void ensureView()")
		create := block(src, "@Override public boolean onCreateActionMode(ActionMode mode, Menu menu)")
		if ensure == "" || create == "" {
			return false
		}
		var removes, returns []string
		for _, l := range codeLines(create) {
			if strings.Contains(l, "remove") || strings.Contains(l, ".clear()") || strings.Contains(l, "setVisible(") ||
				strings.Contains(l, "setEnabled(") {
				removes = append(removes, l)
			}
			if strings.Contains(l, "return ") {
				returns = append(returns, l)
			}
		}
		return strings.Count(code(src), "setId(") == 1 &&
			hasLinesInOrder(ensure, "text = new TextView(activity) {", "text.setId(View.generateViewId());") &&
			slices.Equal(removes, []string{"menu.removeItem(android.R.id.shareText);", "ai.setVisible(studyOffered());"}) &&
			slices.Equal(returns, []string{"return true;"})
	}
	if !offers(java) {
		t.Error("the reading view must be given an id (text.setId(View.generateViewId())) where it is " +
			"made, so the platform adds other apps' text actions to its toolbar, and the menu callback " +
			"must take no item out but the platform's plain-text Share, and hide none but its own " +
			"Study with AI")
	}

	// The accessibility actions: where the platform's node offers other
	// apps' text actions, the view takes them out and offers its own, for
	// the apps the platform lists, under ids of the platform's range, and
	// performs every id of that range itself, never handing one to the
	// platform.
	actionsFn := []string{
		"private static void processTextActions(TextView v, android.view.accessibility.AccessibilityNodeInfo info) {",
		"boolean offered = false;",
		"for (android.view.accessibility.AccessibilityNodeInfo.AccessibilityAction a",
		": new java.util.ArrayList<>(info.getActionList())) {",
		"if (a.getId() < A11Y_PROCESS_TEXT_FIRST || a.getId() >= A11Y_PROCESS_TEXT_END) continue;",
		"info.removeAction(a);",
		"offered = true;",
		"}",
		"a11yProcessText.clear();",
		"if (!offered) return;",
		"android.content.Context c = v.getContext();",
		"PackageManager pm = c.getPackageManager();",
		`Intent query = new Intent(Intent.ACTION_PROCESS_TEXT).setType("text/plain");`,
		"int id = A11Y_PROCESS_TEXT_FIRST;",
		"for (android.content.pm.ResolveInfo ri : pm.queryIntentActivities(query, 0)) {",
		"android.content.pm.ActivityInfo ai = ri.activityInfo;",
		"if (!c.getPackageName().equals(ai.packageName) && !(ai.exported && (ai.permission == null",
		"|| c.checkSelfPermission(ai.permission) == PackageManager.PERMISSION_GRANTED))) continue;",
		"if (id >= A11Y_PROCESS_TEXT_END) break;",
		"a11yProcessText.put(id, new Intent(query).setClassName(ai.packageName, ai.name));",
		"info.addAction(new android.view.accessibility.AccessibilityNodeInfo.AccessibilityAction(id, ri.loadLabel(pm)));",
		"id++;",
		"}",
		"}",
	}
	performFn := []string{
		"@Override public boolean performAccessibilityAction(int action, android.os.Bundle arguments) {",
		"if (action >= A11Y_PROCESS_TEXT_FIRST && action < A11Y_PROCESS_TEXT_END) {",
		"return processTextAsRead(a11yProcessText.get(action));",
		"}",
		"return super.performAccessibilityAction(action, arguments);",
		"}",
	}
	actions := func(src string) bool {
		view := block(src, "text = new TextView(activity)")
		node := block(view, "@Override public void onInitializeAccessibilityNodeInfo(")
		perform := block(view, "@Override public boolean performAccessibilityAction(")
		lines := codeLines(node)
		return slices.Equal(codeLines(block(src, "private static void processTextActions(")), actionsFn) &&
			slices.Equal(codeLines(perform), performFn) &&
			len(lines) >= 2 && lines[len(lines)-2] == "processTextActions(this, info);" &&
			hasLinesInOrder(src,
				"private static final int A11Y_PROCESS_TEXT_FIRST = 0x10000100;",
				"private static final int A11Y_PROCESS_TEXT_END = 0x10001000;",
				"private static final android.util.SparseArray<Intent> a11yProcessText = new android.util.SparseArray<>();") &&
			strings.Count(code(src), "processTextActions(") == 2 && // the declaration and the node's call
			strings.Count(code(src), "performAccessibilityAction(") == 2 // the override and its super call
	}
	if !actions(java) {
		t.Error("the reading view's accessibility actions for other apps' text actions must be its own: " +
			"onInitializeAccessibilityNodeInfo ends with processTextActions, which replaces the platform's, " +
			"and performAccessibilityAction performs every id of the platform's range with processTextAsRead")
	}

	// Scripture is never edited: nothing in the bridge reads what an app
	// returns, the only lines naming the extras are the two that send, the
	// text read-only, and the request code is used where it is declared and
	// where the app is started, nowhere else.
	keeps := func(src string) bool {
		c := code(src)
		var extras []string
		for _, l := range codeLines(src) {
			if strings.Contains(l, "EXTRA_PROCESS_TEXT") {
				extras = append(extras, l)
			}
		}
		return !strings.Contains(c, "onActivityResult") &&
			slices.Equal(extras, []string{
				"out.putExtra(Intent.EXTRA_PROCESS_TEXT_READONLY, true);",
				"out.putExtra(Intent.EXTRA_PROCESS_TEXT, sent);",
			}) &&
			strings.Count(c, "PROCESS_TEXT_REQUEST") == 2
	}
	if !keeps(java) {
		t.Error("nothing an app returns may reach the reading text: the bridge must read no result " +
			"(no onActivityResult, no EXTRA_PROCESS_TEXT read back) and send the text read-only")
	}

	// No suggested actions: where the view is made it is given the NO_OP
	// classifier, on every API that has classifiers (Android 8, API 26, and
	// later), and no other classifier is set anywhere in the bridge.
	const noOp = "text.setTextClassifier(android.view.textclassifier.TextClassifier.NO_OP);"
	smartOff := func(src string) bool {
		ensure := codeLines(block(src, "private static void ensureView()"))
		made := slices.Index(ensure, "text = new TextView(activity) {")
		set := slices.Index(ensure, noOp)
		return strings.Count(code(src), "setTextClassifier(") == 1 &&
			made >= 0 && set > made+1 && set+1 < len(ensure) &&
			ensure[set-1] == "if (android.os.Build.VERSION.SDK_INT >= 26) {" && ensure[set+1] == "}"
	}
	if !smartOff(java) {
		t.Error("the reading view must be given the NO_OP text classifier where it is made, from API 26, " +
			"and no other: a classifier's suggested actions, a Translate among them, hand the platform's " +
			"own send the view's text, past processTextAsRead")
	}

	// The manifest lets the app see the apps that act on selected text, as
	// the platform's query for them asks (an action and a plain-text type),
	// in an entry that is live, not commented out.
	manifest, err := os.ReadFile("cmd/mobile/AndroidManifest.xml")
	if err != nil {
		t.Fatal(err)
	}
	declares := func(raw string) bool {
		var m struct {
			Queries []struct {
				Intents []struct {
					Actions []struct {
						Name string `xml:"http://schemas.android.com/apk/res/android name,attr"`
					} `xml:"action"`
					Data []struct {
						MimeType string `xml:"http://schemas.android.com/apk/res/android mimeType,attr"`
					} `xml:"data"`
				} `xml:"intent"`
			} `xml:"queries"`
		}
		if xml.Unmarshal([]byte(raw), &m) != nil {
			return false
		}
		for _, q := range m.Queries {
			for _, in := range q.Intents {
				if len(in.Actions) == 1 && in.Actions[0].Name == "android.intent.action.PROCESS_TEXT" &&
					len(in.Data) == 1 && in.Data[0].MimeType == "text/plain" {
					return true
				}
			}
		}
		return false
	}
	if !declares(string(manifest)) {
		t.Error("cmd/mobile/AndroidManifest.xml must declare a queries entry for " +
			"android.intent.action.PROCESS_TEXT with the text/plain type: from Android 11 the toolbar " +
			"lists no other app's text action without it")
	}

	const entry = "\t\t<intent>\n\t\t\t<action android:name=\"android.intent.action.PROCESS_TEXT\" />\n" +
		"\t\t\t<data android:mimeType=\"text/plain\" />\n\t\t</intent>\n"
	for _, c := range []struct {
		name, from, to string
		check          func(string) bool
		src            string // what the control changes a copy of
	}{
		// The toolbar.
		{"no id", "        text.setId(View.generateViewId());\n", "", offers, java},
		{"the id on another view", "        text.setId(View.generateViewId());\n",
			"        new TextView(activity).setId(View.generateViewId());\n", offers, java},
		{"the id outside the view's making", "        text.setId(View.generateViewId());\n",
			"", offers, strings.Replace(java, "    private static void ensureView() {\n",
				"    private static void unusedId() {\n        text.setId(View.generateViewId());\n    }\n\n    private static void ensureView() {\n", 1)},
		{"the items taken out again", "                menu.removeItem(android.R.id.shareText);\n",
			"                menu.removeItem(android.R.id.shareText);\n                menu.removeGroup(0);\n", offers, java},
		{"a platform item hidden", "                menu.removeItem(android.R.id.shareText);\n",
			"                menu.removeItem(android.R.id.shareText);\n                menu.findItem(android.R.id.copy).setVisible(false);\n", offers, java},
		{"Study with AI hidden for every selection", "                    ai.setVisible(studyOffered());\n",
			"                    ai.setVisible(false);\n", offers, java},
		{"the menu refused", "                selectionMode = mode;\n                return true;\n",
			"                selectionMode = mode;\n                return false;\n", offers, java},
		// The accessibility actions.
		{"the platform's actions kept", "            info.removeAction(a);\n", "", actions, java},
		{"no actions of the view's own", "                processTextActions(this, info);\n", "", actions, java},
		{"the platform performs them", "                    return processTextAsRead(a11yProcessText.get(action));\n",
			"                    return super.performAccessibilityAction(action, arguments);\n", actions, java},
		{"no override of the action", "@Override public boolean performAccessibilityAction(int action, android.os.Bundle arguments) {",
			"public boolean unusedAction(int action, android.os.Bundle arguments) {", actions, java},
		{"apps the platform would not list", "            if (!c.getPackageName().equals(ai.packageName) && !(ai.exported && (ai.permission == null\n                    || c.checkSelfPermission(ai.permission) == PackageManager.PERMISSION_GRANTED))) continue;\n",
			"", actions, java},
		// No suggested actions.
		{"suggested actions back on", "            " + noOp + "\n", "", smartOff, java},
		{"the classifier on another view", noOp,
			"new TextView(activity).setTextClassifier(android.view.textclassifier.TextClassifier.NO_OP);", smartOff, java},
		{"a classifier set again after", "            " + noOp + "\n        }\n",
			"            " + noOp + "\n        }\n        text.setTextClassifier(null);\n", smartOff, java},
		{"from a later API", "        if (android.os.Build.VERSION.SDK_INT >= 26) {\n            " + noOp,
			"        if (android.os.Build.VERSION.SDK_INT >= 28) {\n            " + noOp, smartOff, java},
		{"a range of the platform's other actions", "private static final int A11Y_PROCESS_TEXT_END = 0x10001000;",
			"private static final int A11Y_PROCESS_TEXT_END = 0x10002000;", actions, java},
		// Scripture never edited.
		{"a result read back", "    private static final int PARCEL_SAFE_TEXT_LENGTH = 100000;\n",
			"    private static final int PARCEL_SAFE_TEXT_LENGTH = 100000;\n    static void onActivityResult(int code, int result, Intent data) {\n        if (code == PROCESS_TEXT_REQUEST) text.setText(data.getCharSequenceExtra(Intent.EXTRA_PROCESS_TEXT));\n    }\n",
			keeps, java},
		{"not sent read-only", "        out.putExtra(Intent.EXTRA_PROCESS_TEXT_READONLY, true);\n", "", keeps, java},
		{"sent as replaceable", "out.putExtra(Intent.EXTRA_PROCESS_TEXT_READONLY, true);",
			"out.putExtra(Intent.EXTRA_PROCESS_TEXT_READONLY, false);", keeps, java},
		// The manifest.
		{"no queries entry", entry, "", declares, string(manifest)},
		{"the entry commented out", entry, "<!--\n" + entry + "-->\n", declares, string(manifest)},
		{"another type", "<data android:mimeType=\"text/plain\" />", "<data android:mimeType=\"text/html\" />", declares, string(manifest)},
		{"another action", "<action android:name=\"android.intent.action.PROCESS_TEXT\" />",
			"<action android:name=\"android.intent.action.SEND\" />", declares, string(manifest)},
	} {
		// A control's source holds what it changes once; "the id outside
		// the view's making" holds the line twice, and takes out the one
		// where the view is made.
		k := strings.Count(c.src, c.from)
		if k != 1 && !(c.name == "the id outside the view's making" && k == 2) {
			t.Fatalf("control %q: the source contains %q %d times", c.name, c.from, k)
		}
		mutated := c.src
		if i := strings.LastIndex(mutated, c.from); i >= 0 {
			mutated = mutated[:i] + c.to + mutated[i+len(c.from):]
		}
		if c.check(mutated) {
			t.Errorf("control %q: the check passes a source without it, so it proves nothing", c.name)
		}
	}
}
