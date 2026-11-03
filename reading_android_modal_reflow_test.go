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

// Execute the actual modal visibility, capture, parked-position and import
// guards against deterministic hidden-layout protocols. Framework traversal,
// keyboard behavior and rendered reading positions still need native checks.
func TestAndroidModalReflowKeepsPositionWithoutRevivingSelection(t *testing.T) {
	source := readNativeSource(t, "android/BtBridge.java")
	var members strings.Builder
	for _, declaration := range []string{
		"private static final class RecreatedReading", "private static void parkRecreatedReading()",
		"private static void clearSemanticReflow()", "private static void captureSemanticReflow()",
		"private static boolean applySemanticReflow()", "private static boolean readerScrollStarted()", "private static float clamp01(float f)",
		"public static void armRestore(", "public static void hide()", "public static void suppress()",
		"public static void unsuppress()", "private static void dismissSelection()",
	} {
		members.WriteString(javaBlockAfter(t, source, declaration))
		members.WriteByte('\n')
	}
	members.WriteString(javaBlockAfter(t, source, "private static final Runnable scrollIdle = new Runnable()"))
	members.WriteString(";\n")
	importer := javaBlockAfter(t, source, "public static void setHtml(")
	start, end := strings.Index(importer, "if (text == null) return;"), strings.Index(importer, "pendingPlace = true;")
	if start < 0 || end <= start {
		t.Fatal("production import carry prelude is missing")
	}
	prelude := importer[start:end]
	start, end = strings.Index(importer, "int[] keptSelection ="), strings.Index(importer, "findTitleRange(text.getText());")
	if start < 0 || end <= start {
		t.Fatal("production import selection tail is missing")
	}
	selection := importer[start:end]
	onScroll := javaBlockAfter(t, source, "public void onScrollChanged()")
	program := strings.NewReplacer("PRODUCTION_MEMBERS", members.String(), "PRODUCTION_IMPORT_PRELUDE", prelude,
		"PRODUCTION_IMPORT_SELECTION", selection, "PRODUCTION_SCROLL", onScroll).Replace(androidModalReflowHarness)
	mapper, err := os.ReadFile("android/BtReadingReflow.java")
	if err != nil {
		t.Fatal(err)
	}
	javac := jdkTool(t)
	java := filepath.Join(filepath.Dir(javac), "java")
	compile := func(t *testing.T, program string) string {
		t.Helper()
		dir := t.TempDir()
		for name, data := range map[string][]byte{"ModalReflowHarness.java": []byte(program), "BtReadingReflow.java": mapper} {
			if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
				t.Fatal(err)
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, javac, "-encoding", "UTF-8", "--release", "8", "-d", dir,
			filepath.Join(dir, "ModalReflowHarness.java"), filepath.Join(dir, "BtReadingReflow.java")).CombinedOutput()
		if err != nil {
			t.Fatalf("compile production modal carry protocol: %v\n%s", err, out)
		}
		return dir
	}
	run := func(t *testing.T, dir, scenario, rejected string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, java, "-cp", dir, "org.bibletext.ModalReflowHarness", scenario).CombinedOutput()
		if rejected != "" {
			if err == nil || !strings.Contains(string(out), rejected) {
				t.Fatalf("modal negative control failed for the wrong reason: %v\n%s", err, out)
			}
			return
		}
		if err != nil || strings.TrimSpace(string(out)) != "PASS "+scenario {
			t.Fatalf("production modal carry protocol: %v\n%s", err, out)
		}
	}
	dir := compile(t, program)
	for _, scenario := range []string{"modal-roundtrip", "hidden-clamp", "repeated-suppression", "recreation", "recreation-then-modal",
		"ordinary-hide", "fresh-arrival", "new-chapter", "changed-text", "empty-text", "reader-scroll",
		"hidden-without-origin", "pending-placement", "pending-arrival", "null-view"} {
		t.Run(scenario, func(t *testing.T) { run(t, dir, scenario, "") })
	}
	for _, control := range []struct{ name, before, after, scenario, rejected string }{
		{"missing-modal-capture", "if (!suppressed && dialog != null && dialog.isShowing()) captureSemanticReflow();", "", "modal-roundtrip", "modal opening lost original semantic point"},
		{"former-clearing-suppression", "suppressed = true;", "suppressed = true; clearSemanticReflow();", "repeated-suppression", "modal opening lost original semantic point"},
		{"hidden-hide-clears-position", "if (!suppressed) {", "if (true) {", "modal-roundtrip", "hidden hide lost modal semantic point"},
		{"hidden-clamp-is-reader-input", "if (suppressed) return;", "", "hidden-clamp", "hidden clamp changed cached reading position"},
		{"late-hidden-clamp-guard", "if (suppressed) return;", "", "late-cache-clamp", "hidden clamp changed cached reading position"},
		{"parked-selection-not-dismissed", "if (recreatedReading != null) recreatedReading = new RecreatedReading(recreatedReading);", "", "recreation-then-modal", "modal retained a parked selection"},
		{"ordinary-hide-retains-position", "if (!suppressed) {", "if (false) {", "ordinary-hide", "ordinary hide retained modal position"},
		{"capture-hidden-origin", "if (!suppressed && dialog != null && dialog.isShowing()) captureSemanticReflow();", "captureSemanticReflow();", "hidden-without-origin", "hidden pane manufactured modal target"},
	} {
		t.Run(control.name, func(t *testing.T) {
			if strings.Count(program, control.before) != 1 {
				t.Fatal("modal negative-control production anchor is no longer unique")
			}
			mutant := strings.Replace(program, control.before, control.after, 1)
			if control.name == "late-hidden-clamp-guard" {
				const cacheWrite = "lastAnchor = new float[]{a != null ? a[0] : 0f, a != null ? a[1] : 0f, lastFrac};"
				if strings.Count(mutant, cacheWrite) != 1 {
					t.Fatal("late cache guard mutation anchor is no longer unique")
				}
				mutant = strings.Replace(mutant, cacheWrite, cacheWrite+" if (suppressed) return;", 1)
			}
			run(t, compile(t, mutant), control.scenario, control.rejected)
		})
	}
}

const androidModalReflowHarness = `
package org.bibletext;
import java.util.ArrayDeque;
public class ModalReflowHarness {
    static TextView text;
    static Dialog dialog;
    static Scroll scroll;
    static String textBook, nextBook, semanticReflowBook;
    static int textChapter, nextChapter, contentEnd, semanticReflowChapter, semanticReflowContentEnd;
    static BtReadingReflow.Anchor semanticReflow;
    static RecreatedReading recreatedReading;
    static boolean suppressed, wantShown, pendingPlace, ownScroll, raActive, raFollowing;
    static int pendingVerse, pendingRestoreVerse, pendingReflowVerse, lastOwnScrollY = -1, range = 3000;
    static float pendingRestoreWithin, pendingFrac, pendingReflowFrac = -1, lastFrac;
    static long placedAtMs, readingIntent, scrollIdleIntent;
    static float[] lastAnchor;
    static int idleEvents;
    static float emittedFrac;
    static long emittedIntent;
    static final Handler UI = new Handler();
    static final Runnable clearOwnScroll = () -> ownScroll = false;
    static class Handler {
        final ArrayDeque<Runnable> queue = new ArrayDeque<Runnable>();
        final ArrayDeque<Runnable> delayed = new ArrayDeque<Runnable>();
        void post(Runnable r) { queue.addLast(r); }
        void postDelayed(Runnable r, long millis) { delayed.addLast(r); }
        void removeCallbacks(Runnable r) { queue.removeIf(x -> x==r);delayed.removeIf(x -> x==r); }
        void drainDelayed() { int n=100;while(!delayed.isEmpty()){if(--n==0)throw new AssertionError("unbounded idle protocol");delayed.removeFirst().run();} }
        void drain() { int n=100; while(!queue.isEmpty()) { if(--n==0)throw new AssertionError("unbounded protocol");queue.removeFirst().run(); } }
    }
    static class Dialog {
        boolean showing = true;
        boolean isShowing() { return showing; }
        void hide() { showing = false; }
    }
    interface Spannable extends CharSequence {}
    static class Page implements Spannable {
        final String raw; int start = -1, finish = -1;
        Page(String raw) { this.raw = raw; }
        public int length() { return raw.length(); }
        public char charAt(int at) { return raw.charAt(at); }
        public CharSequence subSequence(int start,int end) { return raw.subSequence(start,end); }
        public String toString() { return raw; }
    }
    static class Selection {
        static void removeSelection(Spannable value) { Page p=(Page)value;p.start=p.finish=-1; }
        static void setSelection(Spannable value,int start,int end) { Page p=(Page)value;p.start=start;p.finish=end; }
    }
    static class TextView {
        Page page; Layout layout; int padding;
        TextView(String value) { page=new Page(value);layout=narrow();padding=43; }
        int length() { return page.length(); }
        CharSequence getText() { return page; }
        Layout getLayout() { return layout; }
        int getTop() { return 0; }
        int getTotalPaddingTop() { return padding; }
        int getSelectionStart() { return page.start; }
        int getSelectionEnd() { return page.finish; }
    }
    static class Layout {
        final int[] starts;
        Layout(int... starts) { this.starts=starts; }
        int getLineForVertical(int y) { return Math.max(0,Math.min(starts.length-1,y/81)); }
        int getLineForOffset(int offset) { for(int n=1;n<starts.length;n++)if(offset<starts[n])return n-1;return starts.length-1; }
        int getLineStart(int line) { return starts[line]; }
        int getLineTop(int line) { return line*81; }
        int getLineBottom(int line) { return line*81+81; }
    }
    static Layout narrow() { return new Layout(0,27,54,81,108,135,162,189,216,246,275,304,335,364,395,424,455,484); }
    static Layout wide() { return new Layout(0,50,100,150,200,254,300,350,400,450,500); }
    static class Scroll { int y; int getScrollY() { return y; } }
    static int scrollRange() { return range; }
    static void ownScrollTo(int y) { scroll.y=Math.max(0,Math.min(y,range)); }
    static int anchorY(int verse,float within) { return 80; }
    static float[] anchorAt(int y) { return new float[]{4,.375f}; }
    static void dismissStudyPopup() {}
    static void nativeReadAlongUserScrolled() {}
    static void nativeScrolled(float frac,long intent) { idleEvents++;emittedFrac=frac;emittedIntent=intent; }
    static class Observer { void onScrollChanged() {} }
    static Observer observer = new Observer() { PRODUCTION_SCROLL };
    PRODUCTION_MEMBERS
    static void importPage(String value,boolean preservePosition,boolean preserveSelection,int arrivalVerse) {
        PRODUCTION_IMPORT_PRELUDE
        text.page=new Page(value);textBook=nextBook;textChapter=nextChapter;contentEnd=value.length();
        PRODUCTION_IMPORT_SELECTION
    }
    static String raw() { StringBuilder b=new StringBuilder();for(int n=0;n<700;n++)b.append((char)('a'+n%26));return b.toString(); }
    static void reset() {
        text=new TextView(raw());text.page.start=320;text.page.finish=326;
        scroll=new Scroll();scroll.y=931;dialog=new Dialog();textBook=nextBook="Synthetic";textChapter=nextChapter=3;contentEnd=text.length();
        wantShown=true;pendingPlace=false;pendingVerse=0;pendingReflowFrac=-1;pendingRestoreVerse=4;pendingRestoreWithin=.375f;
        lastFrac=.31f;lastAnchor=new float[]{4,.375f,lastFrac};readingIntent=scrollIdleIntent=11;
    }
    static void need(boolean ok,String message) { if(!ok)throw new AssertionError(message); }
    static void noSelection() {
        need(text==null || (text.getSelectionStart()==-1 && text.getSelectionEnd()==-1),"modal retained a live selection");
        need(recreatedReading==null || (recreatedReading.start==-1 && recreatedReading.finish==-1),"modal retained a parked selection");
    }
    static BtReadingReflow.Anchor openModal() {
        suppress();UI.drain();
        need(semanticReflow!=null && semanticReflow.map(text.getText(),contentEnd,contentEnd)==275,"modal opening lost original semantic point");
        noSelection();return semanticReflow;
    }
    static void hiddenWide() { text.layout=wide();text.padding=57;scroll.y=540; }
    static void returnNarrow(BtReadingReflow.Anchor held) {
        text.layout=narrow();text.padding=43;scroll.y=540;
        importPage(raw(),true,true,0);
        need(semanticReflow==held,"import replaced held modal point");
        need(applySemanticReflow() && scroll.y==931,"modal round trip lost original semantic point");
        unsuppress();UI.drain();dialog.showing=true;noSelection();
    }
    public static void main(String[] args) {
        String s=args[0];reset();
        if(s.equals("hidden-without-origin")||s.equals("pending-placement")||s.equals("pending-arrival")||s.equals("null-view")) {
            if(s.equals("hidden-without-origin"))dialog.showing=false;
            if(s.equals("pending-placement"))pendingPlace=true;
            if(s.equals("pending-arrival"))pendingVerse=4;
            if(s.equals("null-view")){text=null;dialog=null;}
            suppress();UI.drain();need(semanticReflow==null,"hidden pane manufactured modal target");noSelection();
        } else if(s.equals("recreation-then-modal")) {
            parkRecreatedReading();clearSemanticReflow();
            need(recreatedReading.start==320 && recreatedReading.finish==326,"visible recreation did not park its existing range");
            BtReadingReflow.Anchor held=recreatedReading.anchor;
            text=new TextView(" ");textBook=null;textChapter=0;contentEnd=0;dialog.showing=false;
            suppress();UI.drain();noSelection();
            need(recreatedReading!=null && recreatedReading.anchor==held,"modal lost parked original point");
            hide();UI.drain();need(recreatedReading!=null && recreatedReading.anchor==held,"hidden hide lost parked point");
            importPage(raw(),true,true,0);need(semanticReflow==held,"replacement import lost parked modal point");
            need(applySemanticReflow() && scroll.y==931,"replacement modal return lost original point");noSelection();
        } else {
            BtReadingReflow.Anchor held=openModal();hiddenWide();
            if(s.equals("hidden-clamp")||s.equals("late-cache-clamp")) {
                float[] originalCache=lastAnchor;float originalFrac=lastFrac;
                UI.postDelayed(scrollIdle,200);observer.onScrollChanged();
                need(lastAnchor==originalCache && lastFrac==originalFrac,"hidden clamp changed cached reading position");
                need(semanticReflow==held,"hidden clamp lost modal semantic point");
                UI.drainDelayed();
                need(idleEvents==1 && emittedFrac==originalFrac && emittedIntent==11,"queued reader idle emitted hidden clamp state");
            }
            hide();UI.drain();need(semanticReflow==held,"hidden hide lost modal semantic point");
            suppress();UI.drain();need(semanticReflow==held,"repeated suppression replaced modal point");noSelection();
            if(s.equals("recreation")) {
                parkRecreatedReading();clearSemanticReflow();
                text=new TextView(" ");textBook=null;textChapter=0;contentEnd=0;suppressed=false;dialog.showing=false;
                suppress();UI.drain();noSelection();
                importPage(raw(),true,true,0);need(semanticReflow==held,"recreated modal lost original semantic point");
                returnNarrow(held);
            } else if(s.equals("ordinary-hide")) {
                unsuppress();UI.drain();hide();UI.drain();need(semanticReflow==null && recreatedReading==null,"ordinary hide retained modal position");
            } else if(s.equals("fresh-arrival")) {
                armRestore(9,.3f,.2f,false);UI.drain();need(semanticReflow==null && scroll.y==80,"fresh arrival lost priority to modal point");
                importPage(raw(),false,false,9);need(semanticReflow==null,"arrival import retained modal point");noSelection();
            } else if(s.equals("new-chapter")) {
                nextChapter=4;importPage(raw(),true,true,0);need(semanticReflow==null,"different chapter retained modal point");noSelection();
            } else if(s.equals("changed-text")||s.equals("empty-text")) {
                importPage(s.equals("empty-text")?"":raw()+"X",true,true,0);
                need(!applySemanticReflow() && semanticReflow==null,"changed or empty text retained modal point");noSelection();
            } else if(s.equals("reader-scroll")) {
                returnNarrow(held);need(readerScrollStarted() && semanticReflow==null,"reader input retained modal point");
                scroll.y=603;suppress();UI.drain();need(semanticReflow!=null && semanticReflow!=held && semanticReflow.map(text.getText(),contentEnd,contentEnd)==162,"new modal reused point before genuine scroll");
            } else if(s.equals("modal-roundtrip")||s.equals("hidden-clamp")||s.equals("late-cache-clamp")||s.equals("repeated-suppression")) {
                range=250;need(applySemanticReflow() && scroll.y==250,"hidden clamp protocol did not clamp intermediate placement");
                range=3000;returnNarrow(held);
            } else throw new AssertionError("unknown scenario");
        }
        System.out.println("PASS "+s);
    }
}
`
