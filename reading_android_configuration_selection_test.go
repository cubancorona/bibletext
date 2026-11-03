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
// The actual capture method also executes against measurement-before-configuration
// and configuration-only protocols that rewrap the old visible point inside
// super. Android traversal and timing remain separate native checks.
func TestAndroidConfigurationChangeKeepsOnlyTheVisibleReadersSelection(t *testing.T) {
	const declaration = "@Override protected void onConfigurationChanged(android.content.res.Configuration config)"
	source := readNativeSource(t, "android/BtBridge.java")
	view := javaBlockAfter(t, source, "text = new TextView(activity)")
	member := javaBlockAfter(t, view, declaration)
	member = strings.ReplaceAll(member, "android.content.res.Configuration", "Configuration")
	measure := javaBlockAfter(t, view, "@Override protected void onMeasure(int widthMeasureSpec, int heightMeasureSpec)")
	measure = strings.ReplaceAll(measure, "android.view.View.MeasureSpec", "MeasureSpec")
	capture := javaBlockAfter(t, source, "private static void captureSemanticReflow()")
	mapper, err := os.ReadFile("android/BtReadingReflow.java")
	if err != nil {
		t.Fatal(err)
	}
	javac := jdkTool(t)
	java := filepath.Join(filepath.Dir(javac), "java")
	compile := func(t *testing.T, callback, measureCallback string) string {
		t.Helper()
		dir := t.TempDir()
		program := strings.NewReplacer("CONFIGURATION_CALLBACK", callback, "MEASURE_CALLBACK", measureCallback, "SEMANTIC_CAPTURE", capture).Replace(androidConfigurationSelectionHarness)
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
		out, err := exec.CommandContext(ctx, javac, "-encoding", "UTF-8", "--release", "8", "-d", dir,
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
	dir := compile(t, member, measure)
	for _, scenario := range []string{
		"visible", "reverse", "unicode", "subsequent-page-reflow",
		"hidden", "suppressed", "null-dialog", "stale-view",
		"hidden-during", "suppressed-during", "null-dialog-during", "view-replaced",
		"book-changed", "chapter-changed", "content-changed", "collapsed",
		"user-deselected", "user-selected-other-word",
		"reflow-original-point", "reflow-held-anchor", "reflow-hidden", "reflow-suppressed",
		"reflow-null-dialog", "reflow-stale-view", "reflow-cold-pending", "reflow-arrival-pending",
		"measure-before-configuration", "measure-held-anchor", "measure-hidden", "measure-suppressed",
		"measure-null-dialog", "measure-stale-view", "measure-cold-pending", "measure-arrival-pending",
		"measure-same-width", "measure-cold-zero-width", "measure-zero-incoming-width",
	} {
		t.Run(scenario, func(t *testing.T) { run(t, dir, scenario, "") })
	}
	t.Run("missing-premeasure-capture-control", func(t *testing.T) {
		mutant := strings.Replace(measure, "captureSemanticReflow();", "", 1)
		if mutant == measure {
			t.Fatal("measure capture control changed no production call")
		}
		run(t, compile(t, member, mutant), "measure-before-configuration", "measure reflow lost original semantic point")
	})
	t.Run("capture-after-measure-super-control", func(t *testing.T) {
		mutant := strings.Replace(measure, "captureSemanticReflow();", "{}", 1)
		mutant = strings.Replace(mutant, "super.onMeasure(widthMeasureSpec, heightMeasureSpec);", "super.onMeasure(widthMeasureSpec, heightMeasureSpec); captureSemanticReflow();", 1)
		if mutant == measure {
			t.Fatal("late measure capture control changed no production call")
		}
		run(t, compile(t, member, mutant), "measure-before-configuration", "measure reflow lost original semantic point")
	})
	t.Run("same-width-measure-control", func(t *testing.T) {
		mutant := strings.Replace(measure, " && incomingWidth != previousWidth", "", 1)
		if mutant == measure {
			t.Fatal("same-width control changed no production guard")
		}
		run(t, compile(t, member, mutant), "measure-same-width", "inactive measure captured a semantic point")
	})
	t.Run("cold-zero-measure-control", func(t *testing.T) {
		mutant := strings.Replace(measure, " && previousWidth > 0", "", 1)
		if mutant == measure {
			t.Fatal("cold measure control changed no production guard")
		}
		run(t, compile(t, member, mutant), "measure-cold-zero-width", "inactive measure captured a semantic point")
	})
	t.Run("missing-preconfiguration-capture-control", func(t *testing.T) {
		mutant := strings.Replace(member, "if (active) captureSemanticReflow();", "", 1)
		if mutant == member {
			t.Fatal("capture control changed no production call")
		}
		run(t, compile(t, mutant, measure), "reflow-original-point", "configuration reflow lost original semantic point")
	})
	t.Run("capture-after-super-control", func(t *testing.T) {
		mutant := strings.Replace(member, "if (active) captureSemanticReflow();", "", 1)
		mutant = strings.Replace(mutant, "super.onConfigurationChanged(config);", "super.onConfigurationChanged(config); if (active) captureSemanticReflow();", 1)
		if mutant == member {
			t.Fatal("late-capture control changed no production call")
		}
		run(t, compile(t, mutant, measure), "reflow-original-point", "configuration reflow lost original semantic point")
	})
	t.Run("inactive-capture-control", func(t *testing.T) {
		mutant := strings.Replace(member, "if (active) captureSemanticReflow();", "captureSemanticReflow();", 1)
		if mutant == member {
			t.Fatal("inactive-capture control changed no production call")
		}
		run(t, compile(t, mutant, measure), "reflow-hidden", "inactive configuration captured a semantic point")
	})
	t.Run("former-super-only-control", func(t *testing.T) {
		old := "@Override protected void onConfigurationChanged(Configuration config) { super.onConfigurationChanged(config); }"
		run(t, compile(t, old, measure), "visible", "configuration selection was not retained")
	})
	t.Run("visibility-guard-control", func(t *testing.T) {
		mutant := strings.ReplaceAll(member, "dialog.isShowing()", "true")
		if mutant == member {
			t.Fatal("visibility control changed no production guard")
		}
		run(t, compile(t, mutant, measure), "hidden", "inactive configuration restored a selection")
	})
	t.Run("current-view-guard-control", func(t *testing.T) {
		mutant := strings.ReplaceAll(member, "this == text", "true")
		if mutant == member {
			t.Fatal("current-view control changed no production guard")
		}
		run(t, compile(t, mutant, measure), "view-replaced", "inactive configuration restored a selection")
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
    static BtReadingReflow.Anchor semanticReflow;
    static String semanticReflowBook;
    static int semanticReflowChapter, semanticReflowContentEnd, pendingVerse;
    static boolean pendingPlace, configurationReflow, measureReflow;
    static class MeasureSpec { static int getSize(int spec) { return spec & 0x3fffffff; } }
    static Scroll scroll = new Scroll();
    static class Scroll { int y; int getScrollY() { return y; } }
    static class Layout {
        final int[] starts;
        Layout(int... values) { starts = values; }
        int getLineForVertical(int y) { return Math.max(0, Math.min(starts.length-1, y/81)); }
        int getLineTop(int line) { return line*81; }
        int getLineBottom(int line) { return getLineTop(line)+81; }
        int getLineStart(int line) { return starts[line]; }
        int getLineForOffset(int offset) {
            for (int n=1; n<starts.length; n++) if (offset<starts[n]) return n-1;
            return starts.length-1;
        }
    }
    static Layout narrow() { return new Layout(0,27,54,81,108,135,162,189,216,246,275,304,335,364,395,424,455,484); }
    static Layout wide() { return new Layout(0,50,100,150,200,254,300,350,400,450); }
SEMANTIC_CAPTURE
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
        int frameworkCallbacks, frameworkMeasures, measuredWidth;
        int getMeasuredWidth() { return measuredWidth; }
        protected void onMeasure(int widthSpec, int heightSpec) {
            frameworkMeasures++;
            int width = MeasureSpec.getSize(widthSpec);
            if (measureReflow && width > 0 && width != measuredWidth) {
                layout = wide(); paddingTop = 57; scroll.y = 540;
            }
            measuredWidth = width;
        }
        Layout layout = new Layout(0);
        int paddingTop;
        int length() { return page.length(); }
        Layout getLayout() { return layout; }
        int getTop() { return 0; }
        int getTotalPaddingTop() { return paddingTop; }
        CharSequence getText() { return page; }
        int getSelectionStart() { return page.start; }
        int getSelectionEnd() { return page.finish; }
        protected void onConfigurationChanged(Configuration config) {
            frameworkCallbacks++;
            // The framework closes its toolbar and collapses the selection.
            page.start = page.finish;
            if (configurationReflow) {
                layout = wide(); paddingTop = 57; scroll.y = 540;
            }
            if (duringConfiguration != null) {
                Runnable action = duringConfiguration; duringConfiguration = null; action.run();
            }
        }
    }
    static TextView makeView(String page) {
        TextView current = new TextView() {
CONFIGURATION_CALLBACK
MEASURE_CALLBACK
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
    static void runReflow(String scenario) {
        StringBuilder raw = new StringBuilder();
        for (int n=0; n<500; n++) raw.append((char)('a'+n%26));
        String original = raw.toString();
        TextView current = makeView(original); text = current;
        textBook = "Example"; textChapter = 3; contentEnd = original.length();
        dialog = new Dialog(); current.layout = narrow(); current.paddingTop = 43; scroll.y = 931;
        current.page.start = 320; current.page.finish = 326; configurationReflow = true;
        current.measuredWidth = 1017; measureReflow = true;
        if (scenario.endsWith("-hidden")) dialog.showing = false;
        if (scenario.endsWith("-suppressed")) suppressed = true;
        if (scenario.endsWith("-null-dialog")) dialog = null;
        if (scenario.endsWith("-stale-view")) text = makeView(original);
        if (scenario.endsWith("-cold-pending")) pendingPlace = true;
        if (scenario.endsWith("-arrival-pending")) pendingVerse = 1;
        BtReadingReflow.Anchor held = null;
        if (scenario.endsWith("-held-anchor")) {
            held = new BtReadingReflow.Anchor(original,112,.5f,false); semanticReflow = held;
            semanticReflowBook = textBook; semanticReflowChapter = textChapter; semanticReflowContentEnd = contentEnd;
        }
        boolean measuring = scenario.startsWith("measure-");
        if (measuring) {
            int requestedWidth = 1988;
            if (scenario.equals("measure-same-width")) requestedWidth = 1017;
            if (scenario.equals("measure-cold-zero-width")) current.measuredWidth = 0;
            if (scenario.equals("measure-zero-incoming-width")) requestedWidth = 0;
            current.onMeasure(0x40000000 | requestedWidth, 0x40000000 | 700);
            need(current.frameworkMeasures == 1, "framework measure callback was skipped");
            if (!scenario.equals("measure-before-configuration") && !scenario.equals("measure-held-anchor")) {
                need(semanticReflow == null, "inactive measure captured a semantic point");
                System.out.println("PASS " + scenario); return;
            }
        }
        current.onConfigurationChanged(new Configuration());
        need(current.frameworkCallbacks == 1, "framework configuration callback was skipped");
        if (!scenario.equals("reflow-original-point") && !scenario.endsWith("-held-anchor")
                && !scenario.equals("measure-before-configuration")) {
            need(semanticReflow == null, "inactive configuration captured a semantic point");
            System.out.println("PASS " + scenario); return;
        }
        // Model the later Go style/frame callback: capturing only now stores
        // wide line start254 even though original275 lies on that same line.
        captureSemanticReflow();
        if (held != null) {
            need(semanticReflow == held && semanticReflow.offset == held.offset,
                    "configuration replaced an already-held semantic point");
        } else {
            need(semanticReflow != null, "configuration semantic point missing");
            int at = semanticReflow.map(current.getText(),contentEnd,contentEnd);
            int wideY = current.paddingTop + current.layout.getLineTop(current.layout.getLineForOffset(at))
                    + Math.round(semanticReflow.lineFraction*81);
            need(wideY == 540, "wide protocol did not preserve its visible line");
            current.layout = narrow(); current.paddingTop = 43;
            int returned = semanticReflow.map(current.getText(),contentEnd,contentEnd);
            int y = current.paddingTop + current.layout.getLineTop(current.layout.getLineForOffset(returned))
                    + Math.round(semanticReflow.lineFraction*81);
            need(at == 275 && returned == 275 && y == 931,
                    measuring ? "measure reflow lost original semantic point" : "configuration reflow lost original semantic point");
        }
        unchangedRange(current,320,326);
        System.out.println("PASS " + scenario);
    }
    public static void main(String[] args) {
        String scenario = args[0];
        if (scenario.startsWith("reflow-") || scenario.startsWith("measure-")) { runReflow(scenario); return; }
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
