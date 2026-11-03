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

// Execute the production intent helper, cancellation method and ScrollView
// overrides with deterministic framework protocols. Actual Android gesture
// interception, rendering and asynchronous traversal remain native checks.
func TestAndroidReaderScrollIntentYieldsBeforeProgrammaticSuppression(t *testing.T) {
	javac := jdkTool(t)
	java := filepath.Join(filepath.Dir(javac), "java")
	source := readNativeSource(t, "android/BtBridge.java")
	cancelMember := javaMember(t, source, "android/BtBridge.java", "private static boolean readerScrollStarted() {") + "\n    }\n"
	members := javaMember(t, source, "android/BtBridge.java", "private static final class ReaderScrollIntent {") + "\n    }\n" +
		cancelMember + javaMember(t, source, "android/BtBridge.java", "private static void clearSemanticReflow() {") + "\n    }\n"
	start := strings.Index(source, "scroll = new ScrollView(activity) {")
	if start < 0 {
		t.Fatal("production reader ScrollView missing")
	}
	end := strings.Index(source[start:], "\n        };")
	if end < 0 {
		t.Fatal("production reader ScrollView boundary missing")
	}
	view := source[start : start+end+len("\n        };")]
	program := scrollIntentHarness + members + "\n    static void makeView() {\n" + view + "\n    }\n}\n"
	program = strings.NewReplacer(
		"android.view.MotionEvent", "MotionEvent",
		"android.view.ViewConfiguration", "ViewConfiguration",
		"android.view.accessibility.AccessibilityNodeInfo", "AccessibilityNodeInfo",
		"android.os.Bundle", "Bundle",
	).Replace(program)
	run := func(t *testing.T, text, scenario string, expectFailure bool) {
		t.Helper()
		dir := t.TempDir()
		path := filepath.Join(dir, "ScrollIntentHarness.java")
		if err := os.WriteFile(path, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if out, err := exec.CommandContext(ctx, javac, "--release", "8", "-d", dir, path).CombinedOutput(); err != nil {
			t.Fatalf("compile production scroll protocols: %v\n%s", err, out)
		}
		out, err := exec.CommandContext(ctx, java, "-cp", dir, "ScrollIntentHarness", scenario).CombinedOutput()
		if expectFailure {
			if err == nil || !strings.Contains(string(out), "real drag retained programmatic suppression") {
				t.Fatalf("old-behavior control did not fail on the real drag: %v\n%s", err, out)
			}
			return
		}
		if err != nil || strings.TrimSpace(string(out)) != "PASS "+scenario {
			t.Fatalf("production scroll protocols: %v\n%s", err, out)
		}
	}
	for _, scenario := range []string{"drag", "child-drag", "tap-selection-clamp", "arrival", "deferred-placement", "pointer-cancel", "accessibility-wheel"} {
		t.Run(scenario, func(t *testing.T) { run(t, program, scenario, false) })
	}
	t.Run("old-behavior-control", func(t *testing.T) {
		// The former reader had no gesture cancellation before its grace-window
		// early return. Make cancellation a no-op; execute the same real-drag case.
		mutant := strings.Replace(program, cancelMember,
			"private static boolean readerScrollStarted() { return false; }\n", 1)
		if mutant == program {
			t.Fatal("old-behavior control changed no production call")
		}
		run(t, mutant, "drag", true)
	})
}

const scrollIntentHarness = `
public class ScrollIntentHarness {
    static Object activity = new Object(), semanticReflow;
    static String semanticReflowBook;
    static int semanticReflowChapter, pendingVerse, pendingReflowVerse, pendingRestoreVerse, lastOwnScrollY;
    static boolean pendingPlace, ownScroll;
    static float pendingReflowFrac, pendingFrac;
    static long placedAtMs;
    static ScrollView scroll;
    static final Runnable clearOwnScroll = () -> ownScroll = false;
    static final Handler UI = new Handler();
    static class Handler { int removed; void removeCallbacks(Runnable r) { removed++; } }
    static class View {} static class Rect {} static class Bundle {}
    static class ViewConfiguration {
        static ViewConfiguration get(Object c) { return new ViewConfiguration(); }
        int getScaledTouchSlop() { return 4; }
    }
    static class MotionEvent {
        static final int ACTION_DOWN=0, ACTION_UP=1, ACTION_MOVE=2, ACTION_CANCEL=3, ACTION_POINTER_UP=6, ACTION_SCROLL=8, AXIS_VSCROLL=9;
        final int action, index; final int[] ids; final float[] ys; float axis;
        MotionEvent(int a, int i, int[] p, float[] y) { action=a; index=i; ids=p; ys=y; }
        int getActionMasked() { return action; } int getActionIndex() { return index; }
        int getPointerCount() { return ids.length; } int getPointerId(int i) { return ids[i]; }
        float getY(int i) { return ys[i]; } float getAxisValue(int i) { return axis; }
        int findPointerIndex(int id) { for(int i=0;i<ids.length;i++) if(ids[i]==id)return i; return -1; }
    }
    static class AccessibilityNodeInfo {
        static final int ACTION_SCROLL_FORWARD=4096, ACTION_SCROLL_BACKWARD=8192;
        static class AccessibilityAction {
            static final AccessibilityAction ACTION_SCROLL_UP=new AccessibilityAction(44), ACTION_SCROLL_DOWN=new AccessibilityAction(46);
            final int id; AccessibilityAction(int i) { id=i; } int getId() { return id; }
        }
    }
    static class ScrollView {
        boolean childConsumes, interceptMove;
        ScrollView(Object a) {} Object getContext() { return activity; }
        boolean dispatchTouchEvent(MotionEvent e) {
            if(childConsumes && !(e.action==MotionEvent.ACTION_MOVE && interceptMove)) return true;
            return onTouchEvent(e);
        }
        boolean onTouchEvent(MotionEvent e) { return true; }
        boolean onGenericMotionEvent(MotionEvent e) { return true; }
        boolean performAccessibilityAction(int a, Bundle b) { return true; }
        boolean requestChildRectangleOnScreen(View v, Rect r, boolean i) { return true; }
    }
    static MotionEvent event(int a, float y) { return new MotionEvent(a,0,new int[]{7},new float[]{y}); }
    static void need(boolean b,String s) { if(!b)throw new AssertionError(s); }
    static void reset() {
        semanticReflow=new Object(); semanticReflowBook="Synthetic"; semanticReflowChapter=3;
        pendingVerse=0; pendingPlace=false; pendingReflowFrac=.4f; pendingReflowVerse=6;
        pendingFrac=.4f; pendingRestoreVerse=6; placedAtMs=123; ownScroll=true; lastOwnScrollY=50; UI.removed=0;
        makeView();
    }
    static void changed() {
        need(!ownScroll,"real drag retained programmatic suppression");
        need(semanticReflow==null && pendingReflowFrac<0 && pendingReflowVerse==0,"real reader scroll retained stale semantic/reflow target");
        need(placedAtMs==0 && pendingFrac<0 && pendingRestoreVerse==0 && lastOwnScrollY==-1 && UI.removed==1,"reader cancellation protocol incomplete");
    }
    static void kept() {
        need(ownScroll && semanticReflow!=null && pendingReflowFrac==.4f && UI.removed==0,"non-scroll or priority placement lost protection");
    }
    public static void main(String[] args) {
        String s=args[0]; reset();
        if(s.equals("drag") || s.equals("child-drag")) {
            scroll.childConsumes=s.equals("child-drag"); scroll.interceptMove=true;
            scroll.dispatchTouchEvent(event(0,10)); scroll.dispatchTouchEvent(event(2,30)); changed();
            scroll.dispatchTouchEvent(event(2,40)); need(UI.removed==1,"drag cancelled repeatedly");
            scroll.dispatchTouchEvent(event(1,40)); clearOwnScroll.run();
            need(semanticReflow==null,"grace expiry resurrected the prior target");
        } else if(s.equals("tap-selection-clamp")) {
            scroll.dispatchTouchEvent(event(0,10)); scroll.dispatchTouchEvent(event(2,12)); scroll.dispatchTouchEvent(event(1,12)); kept();
            scroll.childConsumes=true; scroll.interceptMove=false;
            scroll.dispatchTouchEvent(event(0,10)); scroll.dispatchTouchEvent(event(2,80)); scroll.dispatchTouchEvent(event(1,80)); kept();
            clearOwnScroll.run(); need(semanticReflow!=null,"delayed programmatic completion discarded semantic target");
        } else if(s.equals("arrival")) {
            for(int n=0;n<2;n++) {
                reset(); pendingVerse=n==0?14:0; pendingPlace=n==1;
                scroll.dispatchTouchEvent(event(0,10)); scroll.dispatchTouchEvent(event(2,30)); kept();
                need(pendingVerse==(n==0?14:0) && pendingPlace==(n==1),"explicit arrival priority changed");
            }
        } else if(s.equals("deferred-placement")) {
            for(int n=0;n<2;n++) {
                reset(); pendingVerse=n==0?14:0; pendingPlace=n==1;
                scroll.dispatchTouchEvent(event(0,10)); scroll.dispatchTouchEvent(event(2,30)); kept();
                pendingVerse=0; pendingPlace=false;
                scroll.dispatchTouchEvent(event(2,40)); changed();
                scroll.dispatchTouchEvent(event(2,50)); need(UI.removed==1,"deferred acquisition repeated");
            }
        } else if(s.equals("pointer-cancel")) {
            scroll.dispatchTouchEvent(event(0,10)); scroll.dispatchTouchEvent(event(3,10));
            scroll.onTouchEvent(event(2,70)); kept();
            scroll.dispatchTouchEvent(event(0,10));
            scroll.dispatchTouchEvent(new MotionEvent(6,0,new int[]{7,9},new float[]{10,60}));
            scroll.dispatchTouchEvent(new MotionEvent(2,0,new int[]{9},new float[]{62})); kept();
            scroll.dispatchTouchEvent(new MotionEvent(2,0,new int[]{9},new float[]{80})); changed();
        } else if(s.equals("accessibility-wheel")) {
            for(int a:new int[]{4096,8192,44,46}) { reset(); scroll.performAccessibilityAction(a,new Bundle()); changed(); }
            reset(); scroll.performAccessibilityAction(16,new Bundle()); kept();
            MotionEvent e=event(8,0); scroll.onGenericMotionEvent(e); kept();
            e.axis=1; scroll.onGenericMotionEvent(e); changed();
        } else throw new AssertionError("unknown case");
        System.out.println("PASS "+s);
    }
`
