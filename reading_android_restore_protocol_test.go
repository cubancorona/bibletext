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

// Execute the production saved-placement tail, readiness/capture/arm methods,
// scroll listener and layout hook under deterministic framework protocols.
// Verse geometry is supplied by the harness; real Android traversal and arrival
// rendering remain native checks and the separate arrival contracts.
func TestAndroidSavedRestoreWaitsForLayoutAndYieldsToLiveReflow(t *testing.T) {
	source := readNativeSource(t, "android/BtBridge.java")
	var members strings.Builder
	for _, name := range []string{
		"private static void clearSemanticReflow()", "private static void captureSemanticReflow()",
		"private static boolean applySemanticReflow()", "private static boolean placementLayoutReady()",
		"private static int scrollRange()", "private static float clamp01(float f)",
		"private static void beginOwnScroll()", "private static void endOwnScroll()", "private static void ownScrollTo(int y)",
		"public static void armRestore(", "public static void setReadingIntent(",
	} {
		members.WriteString(javaBlockAfter(t, source, name))
		members.WriteByte('\n')
	}
	apply := javaBlockAfter(t, source, "private static void applyPendingScroll()")
	start, end := strings.Index(apply, "if (!pendingPlace)"), strings.LastIndex(apply, "\n            }")
	if start < 0 || end < start {
		t.Fatal("production saved-placement tail is missing")
	}
	tail := apply[start:end]
	view := javaBlockAfter(t, source, "text = new TextView(activity)")
	onLayout := javaBlockAfter(t, view, "@Override protected void onLayout(")
	onScroll := javaBlockAfter(t, source, "public void onScrollChanged()")
	idle := javaBlockAfter(t, source, "private static final Runnable scrollIdle = new Runnable()") + ";\n"
	program := strings.NewReplacer(
		"PRODUCTION_MEMBERS", members.String()+idle,
		"PRODUCTION_SAVED_TAIL", tail,
		"PRODUCTION_LAYOUT", onLayout,
		"PRODUCTION_SCROLL", onScroll,
	).Replace(androidRestoreProtocolHarness)
	program = strings.NewReplacer("android.text.TextUtils", "TextUtils", "android.util.Log", "Log").Replace(program)
	mapper, err := os.ReadFile("android/BtReadingReflow.java")
	if err != nil {
		t.Fatal(err)
	}
	javac := jdkTool(t)
	java := filepath.Join(filepath.Dir(javac), "java")
	compile := func(t *testing.T, program string) string {
		t.Helper()
		dir := t.TempDir()
		for name, data := range map[string][]byte{"RestoreProtocolHarness.java": []byte(program), "BtReadingReflow.java": mapper} {
			if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
				t.Fatal(err)
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, javac, "--release", "8", "-d", dir,
			filepath.Join(dir, "RestoreProtocolHarness.java"), filepath.Join(dir, "BtReadingReflow.java")).CombinedOutput()
		if err != nil {
			t.Fatalf("compile production saved-restore protocol: %v\n%s", err, out)
		}
		return dir
	}
	run := func(t *testing.T, dir, scenario, rejected string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, java, "-cp", dir, "org.bibletext.RestoreProtocolHarness", scenario).CombinedOutput()
		if rejected != "" {
			if err == nil || !strings.Contains(string(out), rejected) {
				t.Fatalf("negative control failed for the wrong reason: %v\n%s", err, out)
			}
			return
		}
		if err != nil || strings.TrimSpace(string(out)) != "PASS "+scenario {
			t.Fatalf("production saved-restore protocol: %v\n%s", err, out)
		}
	}
	dir := compile(t, program)
	for _, scenario := range []string{"cold-unready", "same-bounds", "stale-layout", "unindexed", "missing-verse", "live-arm", "fresh-arm", "cold-clamp-idle", "idle-owner", "clamped-own-scroll"} {
		t.Run(scenario, func(t *testing.T) { run(t, dir, scenario, "") })
	}
	for _, control := range []struct{ name, before, after, scenario, rejected string }{
		{"premature-consumption-control", "if (!placementLayoutReady() || (pendingRestoreVerse > 0 && verseNums.length == 0)) return;", "", "cold-unready", "cold placement was consumed before usable geometry"},
		{"same-bounds-liveness-control", "if (pendingPlace) applyPendingScroll();", "", "same-bounds", "same-bounds layout left restore pending"},
		{"eager-reflow-arm-control", "if (text == null || reflow) return;", "if (text == null || (reflow && semanticReflow != null)) return;", "live-arm", "reflow arm moved the live point"},
		{"cold-clamp-control", "if (pendingPlace) return;", "", "cold-clamp-idle", "cold clamp retired pending target"},
		{"requested-own-scroll-control", "lastOwnScrollY = scroll.getScrollY();", "lastOwnScrollY = y;", "clamped-own-scroll", "clamped own scroll became reader input"},
	} {
		t.Run(control.name, func(t *testing.T) {
			if strings.Count(program, control.before) != 1 {
				t.Fatal("negative-control production anchor is no longer unique")
			}
			mutant := strings.Replace(program, control.before, control.after, 1)
			run(t, compile(t, mutant), control.scenario, control.rejected)
		})
	}
}

const androidRestoreProtocolHarness = `
package org.bibletext;
import java.util.ArrayDeque;
import java.util.ArrayList;
public class RestoreProtocolHarness {
    static TextView text;
    static ScrollView scroll;
    static View content;
    static String textBook = "Example", semanticReflowBook;
    static int textChapter = 3, semanticReflowChapter, semanticReflowContentEnd, contentEnd;
    static BtReadingReflow.Anchor semanticReflow;
    static int pendingVerse, pendingRestoreVerse, lastOwnScrollY = -1, placements;
    static float pendingFrac, pendingRestoreWithin, pendingReflowFrac = -1, lastFrac;
    static boolean pendingPlace, ownScroll, raActive, raFollowing;
    static long placedAtMs, readingIntent, nextReadingIntent, scrollIdleIntent;
    static float[] lastAnchor = new float[]{4,.375f,.2f};
    static int[] verseNums = new int[]{1,2,3,4,5};
    static final boolean SCROLL_DEBUG = false;
    static final Handler UI = new Handler();
    static final Runnable clearOwnScroll = () -> ownScroll = false;
    static final ArrayList<Long> events = new ArrayList<Long>();
    static class Handler {
        final ArrayDeque<Runnable> queue = new ArrayDeque<Runnable>();
        final ArrayList<Runnable> delayed = new ArrayList<Runnable>();
        void post(Runnable r) { queue.addLast(r); }
        void postDelayed(Runnable r, long ms) { delayed.add(r); }
        void removeCallbacks(Runnable r) { queue.removeIf(x -> x == r); delayed.removeIf(x -> x == r); }
        void drain() { int limit=100; while(!queue.isEmpty()) { if(--limit==0)throw new AssertionError("unbounded layout retry"); queue.removeFirst().run(); } }
    }
    static class View {
        int width=100,height=500; boolean attached=true,requested;
        int getWidth(){return width;} int getHeight(){return height;}
        boolean isAttachedToWindow(){return attached;} boolean isLayoutRequested(){return requested;}
    }
    static class Layout {
        String value; int width=100;
        Layout(String s){value=s;} CharSequence getText(){return value;} int getWidth(){return width;}
        int getLineForVertical(int y){return Math.max(0,y/10);} int getLineForOffset(int o){return o/10;}
        int getLineStart(int line){return Math.min(value.length(),line*10);}
        int getLineTop(int line){return line*10;} int getLineBottom(int line){return line*10+10;}
    }
    static class TextView extends View {
        String value; Layout layout;
        CharSequence getText(){return value;} int length(){return value.length();} Layout getLayout(){return layout;}
        int getTop(){return 0;} int getTotalPaddingTop(){return 0;}
        int getCompoundPaddingLeft(){return 0;} int getCompoundPaddingRight(){return 0;}
        protected void onLayout(boolean changed,int l,int t,int r,int b){}
    }
    static class ScrollView extends View {
        int y,moves;
        int getScrollY(){return y;} View getChildAt(int i){return content;}
        void post(Runnable r){UI.post(r);}
        void scrollTo(int x,int target){moves++;y=Math.max(0,Math.min(target,scrollRange()));}
    }
    static class TextUtils { static boolean equals(CharSequence a,CharSequence b){return a.toString().equals(b.toString());} }
    static class Log { static void i(String a,String b){} }
    static class Observer { public void onScrollChanged(){} }
    static Observer observer = new Observer() { PRODUCTION_SCROLL };
    static int anchorY(int verse,float within){return text.layout!=null && verse>0 && verse<=5 ? 80 : -1;}
    static float[] anchorAt(int y){return new float[]{4,.375f};}
    static void notePlacement(int verse,float within,int y){placements++;}
    static void nativeScrolled(float frac,long intent){events.add(intent);}
    static void nativeReadAlongUserScrolled(){}
    static void applySaved(){ scroll.post(new Runnable(){ public void run(){ PRODUCTION_SAVED_TAIL }}); }
    static void applyPendingScroll(){applySaved();}
    PRODUCTION_MEMBERS
    static void need(boolean ok,String message){if(!ok)throw new AssertionError(message);}
    static void ready(){
        text.width=100;text.height=500;text.attached=true;text.requested=false;text.layout=new Layout(text.value);
        scroll.width=100;scroll.height=100;scroll.attached=true;scroll.requested=false;
        content.width=100;content.height=500;content.requested=false;
    }
    static void reset(){
        text = new TextView(){ PRODUCTION_LAYOUT };
        StringBuilder b=new StringBuilder();for(int n=0;n<300;n++)b.append((char)('a'+n%26));
        text.value=b.toString();contentEnd=text.value.length();
        scroll=new ScrollView();content=new View();ready();
        pendingPlace=true;pendingRestoreVerse=4;pendingRestoreWithin=.375f;pendingFrac=.2f;
        readingIntent=nextReadingIntent=11;scrollIdleIntent=11;
    }
    static void apply(){applySaved();UI.drain();}
    public static void main(String[] args){
        String s=args[0];reset();
        if(s.equals("cold-unready")||s.equals("same-bounds")){
            text.layout=null;text.attached=false;text.width=0;text.height=0;scroll.height=0;content.height=0;
            armRestore(4,.375f,.2f,true);UI.drain();captureSemanticReflow();apply();
            need(pendingPlace && placements==0 && semanticReflow==null,"cold placement was consumed before usable geometry");
            ready();text.onLayout(false,0,0,100,500);UI.drain();
            need(!pendingPlace && scroll.y==80,"same-bounds layout left restore pending");
        }else if(s.equals("stale-layout")){
            text.requested=true;apply();need(pendingPlace,"requested layout consumed restore");text.requested=false;
            scroll.requested=true;apply();need(pendingPlace,"requested parent consumed restore");scroll.requested=false;
            content.requested=true;apply();need(pendingPlace,"requested content consumed restore");content.requested=false;
            text.layout.width=90;apply();need(pendingPlace,"stale width consumed restore");text.layout.width=100;
            text.layout.value="different";apply();need(pendingPlace,"stale text consumed restore");text.layout.value=text.value;
            text.attached=false;apply();need(pendingPlace,"detached view consumed restore");ready();apply();need(!pendingPlace&&scroll.y==80,"ready placement missing");
        }else if(s.equals("unindexed")){
            verseNums=new int[0];apply();need(pendingPlace,"unindexed saved target consumed");verseNums=new int[]{1,2,3,4,5};text.onLayout(false,0,0,100,500);UI.drain();need(!pendingPlace&&scroll.y==80,"indexed target not retried");
        }else if(s.equals("missing-verse")){
            pendingRestoreVerse=99;apply();need(!pendingPlace&&scroll.y==80,"missing indexed verse did not use measured fraction fallback");
        }else if(s.equals("live-arm")){
            pendingPlace=false;scroll.y=43;armRestore(4,.375f,.2f,true);UI.drain();
            need(scroll.y==43,"reflow arm moved the live point");captureSemanticReflow();
            pendingPlace=true;apply();need(scroll.y==43 && pendingRestoreVerse==4,"live semantic point did not outrank unchanged saved fallback");
        }else if(s.equals("fresh-arm")){
            pendingPlace=false;scroll.y=43;captureSemanticReflow();armRestore(4,.375f,.2f,false);UI.drain();
            need(semanticReflow==null&&scroll.y==80,"fresh explicit saved request lost placement priority");
        }else if(s.equals("cold-clamp-idle")){
            scroll.y=0;observer.onScrollChanged();
            need(pendingRestoreVerse==4&&pendingFrac==.2f,"cold clamp retired pending target");
            scrollIdle.run();need(events.isEmpty(),"pending cold idle emitted a reader event");
        }else if(s.equals("idle-owner")){
            pendingPlace=false;setReadingIntent(22);UI.drain();
            need(readingIntent==11&&nextReadingIntent==22,"new intent claimed old installed page");
            scrollIdle.run();need(events.size()==1&&events.get(0)==11,"already dequeued old idle borrowed new intent");
            readingIntent=nextReadingIntent;pendingPlace=true;scrollIdle.run();need(events.size()==1,"pending new placement emitted scroll");
            pendingPlace=false;scroll.y=43;lastOwnScrollY=-1;observer.onScrollChanged();scrollIdle.run();
            need(events.size()==2&&events.get(1)==22,"new installed page did not own its genuine scroll");
        }else if(s.equals("clamped-own-scroll")){
            pendingPlace=false;ownScrollTo(800);clearOwnScroll.run();observer.onScrollChanged();
            need(scroll.y==400&&lastOwnScrollY==400&&!UI.delayed.contains(scrollIdle),"clamped own scroll became reader input");
        }else throw new AssertionError("unknown scenario");
        System.out.println("PASS "+s);
    }
}
`
