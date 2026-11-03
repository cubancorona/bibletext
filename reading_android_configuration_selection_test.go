package bibletext

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Execute the reading TextView's production callback and offset mapper against
// a framework protocol that collapses selection during configuration changes.
// Android traversal, floating toolbar and window timing remain native checks.
func TestAndroidConfigurationChangeKeepsOnlyTheVisibleReadersSelection(t *testing.T) {
	const declaration = "@Override protected void onConfigurationChanged(android.content.res.Configuration config)"
	source := readNativeSource(t, "android/BtBridge.java")
	view := javaBlockAfter(t, source, "text = new TextView(activity)")
	member := javaBlockAfter(t, view, declaration)
	member = strings.ReplaceAll(member, "android.content.res.Configuration", "Configuration")
	mapper, err := os.ReadFile("android/BtReadingReflow.java")
	if err != nil {
		t.Fatal(err)
	}
	javac := jdkTool(t)
	java := filepath.Join(filepath.Dir(javac), "java")
	compile := func(t *testing.T, callback string) string {
		t.Helper()
		dir := t.TempDir()
		program := strings.Replace(androidConfigurationSelectionHarness, "CONFIGURATION_CALLBACK", callback, 1)
		for name, data := range map[string][]byte{
			"ConfigurationSelectionHarness.java": []byte(program),
			"BtReadingReflow.java":               mapper,
		} {
			if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
				t.Fatal(err)
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, javac, "--release", "8", "-d", dir,
			filepath.Join(dir, "BtReadingReflow.java"), filepath.Join(dir, "ConfigurationSelectionHarness.java")).CombinedOutput()
		if err != nil {
			t.Fatalf("compile production configuration callback: %v\n%s", err, out)
		}
		return dir
	}
	run := func(t *testing.T, dir, scenario, rejected string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, java, "-cp", dir, "org.bibletext.ConfigurationSelectionHarness", scenario).CombinedOutput()
		if rejected != "" {
			if err == nil || !strings.Contains(string(out), rejected) {
				t.Fatalf("negative control did not fail for its intended guard: %v\n%s", err, out)
			}
			return
		}
		if err != nil || strings.TrimSpace(string(out)) != "PASS "+scenario {
			t.Fatalf("production configuration callback: %v\n%s", err, out)
		}
	}
	dir := compile(t, member)
	for _, scenario := range []string{
		"visible", "reverse", "unicode", "subsequent-page-reflow",
		"hidden", "suppressed", "null-dialog", "stale-view",
		"hidden-during", "suppressed-during", "null-dialog-during", "view-replaced",
		"book-changed", "chapter-changed", "content-changed", "collapsed",
		"user-deselected", "user-selected-other-word",
	} {
		t.Run(scenario, func(t *testing.T) { run(t, dir, scenario, "") })
	}
	t.Run("former-super-only-control", func(t *testing.T) {
		old := "@Override protected void onConfigurationChanged(Configuration config) { super.onConfigurationChanged(config); }"
		run(t, compile(t, old), "visible", "configuration selection was not retained")
	})
	t.Run("visibility-guard-control", func(t *testing.T) {
		mutant := strings.ReplaceAll(member, "dialog.isShowing()", "true")
		if mutant == member {
			t.Fatal("visibility control changed no production guard")
		}
		run(t, compile(t, mutant), "hidden", "inactive configuration restored a selection")
	})
	t.Run("current-view-guard-control", func(t *testing.T) {
		mutant := strings.ReplaceAll(member, "this == text", "true")
		if mutant == member {
			t.Fatal("current-view control changed no production guard")
		}
		run(t, compile(t, mutant), "view-replaced", "inactive configuration restored a selection")
	})
}

const androidConfigurationSelectionHarness = `
package org.bibletext;
public class ConfigurationSelectionHarness {
    static TextView text;
    static Dialog dialog;
    static String textBook;
    static int textChapter, contentEnd;
    static boolean suppressed;
    static Runnable duringConfiguration;
    static class Configuration {}
    static class Dialog { boolean showing = true; boolean isShowing() { return showing; } }
    interface Spannable extends CharSequence {}
    static class Page implements Spannable {
        final String value;
        int start, finish;
        Page(String raw) { value = raw; }
        public int length() { return value.length(); }
        public char charAt(int at) { return value.charAt(at); }
        public CharSequence subSequence(int start, int end) { return value.subSequence(start, end); }
        public String toString() { return value; }
    }
    static class Selection {
        static int restorations;
        static void setSelection(Spannable value, int start, int end) {
            Page page = (Page)value;
            page.start = start; page.finish = end; restorations++;
        }
    }
    static class TextView {
        Page page;
        int frameworkCallbacks;
        CharSequence getText() { return page; }
        int getSelectionStart() { return page.start; }
        int getSelectionEnd() { return page.finish; }
        protected void onConfigurationChanged(Configuration config) {
            frameworkCallbacks++;
            // The framework closes its toolbar and collapses the selection.
            page.start = page.finish;
            if (duringConfiguration != null) {
                Runnable action = duringConfiguration; duringConfiguration = null; action.run();
            }
        }
    }
    static TextView makeView(String page) {
        TextView current = new TextView() {
CONFIGURATION_CALLBACK
        };
        current.page = new Page(page);
        return current;
    }
    static void need(boolean ok, String message) { if (!ok) throw new AssertionError(message); }
    static void unchangedRange(TextView view, int start, int finish) {
        need(view.getSelectionStart() == start && view.getSelectionEnd() == finish,
                "configuration selection was not retained");
    }
    static void notRestored() {
        need(Selection.restorations == 0, "inactive configuration restored a selection");
    }
    public static void main(String[] args) {
        String scenario = args[0];
        String original = "Alpha Beta.\n\nSecond Beta.\n\u200eText 🌿 end.";
        TextView current = makeView(original);
        text = current; textBook = "Example"; textChapter = 3; contentEnd = original.length();
        dialog = new Dialog();
        int start = original.lastIndexOf("Beta"), finish = start + 4;
        if (scenario.equals("reverse")) { int temp = start; start = finish; finish = temp; }
        if (scenario.equals("unicode")) { start = original.indexOf("🌿"); finish = start + 2; }
        current.page.start = start; current.page.finish = finish;
        if (scenario.equals("hidden")) dialog.showing = false;
        if (scenario.equals("suppressed")) suppressed = true;
        if (scenario.equals("null-dialog")) dialog = null;
        if (scenario.equals("stale-view")) text = makeView(original);
        if (scenario.equals("hidden-during")) duringConfiguration = () -> dialog.showing = false;
        if (scenario.equals("suppressed-during")) duringConfiguration = () -> suppressed = true;
        if (scenario.equals("null-dialog-during")) duringConfiguration = () -> dialog = null;
        if (scenario.equals("view-replaced")) duringConfiguration = () -> text = makeView(original);
        if (scenario.equals("book-changed")) duringConfiguration = () -> textBook = "Other";
        if (scenario.equals("chapter-changed")) duringConfiguration = () -> textChapter = 4;
        if (scenario.equals("content-changed")) duringConfiguration = () -> {
            current.page = new Page(original.replace("Second", "Different"));
            contentEnd = current.page.length();
        };
        if (scenario.equals("collapsed")) current.page.start = current.page.finish = 0;
        current.onConfigurationChanged(new Configuration());
        need(current.frameworkCallbacks == 1, "framework configuration callback was skipped");
        if (scenario.equals("visible") || scenario.equals("reverse") || scenario.equals("unicode")
                || scenario.equals("subsequent-page-reflow")) {
            unchangedRange(current, start, finish);
            need(Selection.restorations == 1, "visible selection was not restored exactly once");
            if (scenario.equals("subsequent-page-reflow")) {
                String next = original.replace("\n\n", "\n");
                int[] mapped = BtReadingReflow.selection(true, true, current.getText(), contentEnd,
                        current.getSelectionStart(), current.getSelectionEnd(), next, next.length());
                need(mapped != null && mapped[0] == next.lastIndexOf("Beta") && mapped[1] == mapped[0]+4,
                        "configuration carry did not reach the guarded page reflow");
            }
        } else if (scenario.equals("user-deselected")) {
            unchangedRange(current, start, finish);
            current.page.start = current.page.finish = 0;
            current.onConfigurationChanged(new Configuration());
            need(Selection.restorations == 1 && current.getSelectionStart() == 0 && current.getSelectionEnd() == 0,
                    "normal user deselection resurrected an old range");
        } else if (scenario.equals("user-selected-other-word")) {
            unchangedRange(current, start, finish);
            current.page.start = 0; current.page.finish = 5;
            current.onConfigurationChanged(new Configuration());
            unchangedRange(current, 0, 5);
            need(Selection.restorations == 2, "new user selection was replaced with a cached range");
        } else {
            notRestored();
        }
        System.out.println("PASS " + scenario);
    }
}
`
