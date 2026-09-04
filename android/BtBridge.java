package org.bibletext;

import android.app.Activity;
import android.app.Application;
import android.app.Dialog;
import android.content.Intent;
import android.graphics.Color;
import android.graphics.drawable.ColorDrawable;
import android.graphics.drawable.GradientDrawable;
import android.graphics.Rect;
import android.os.Handler;
import android.os.Looper;
import android.text.Html;
import android.text.Layout;
import android.text.Selection;
import android.text.Spannable;
import android.text.Spanned;
import android.text.style.BackgroundColorSpan;
import android.text.style.SuperscriptSpan;
import android.provider.MediaStore;
import android.view.ActionMode;
import android.view.Gravity;
import android.view.KeyEvent;
import android.view.Menu;
import android.view.MenuItem;
import android.view.SubMenu;
import android.view.View;
import android.view.Window;
import android.view.WindowManager;
import android.widget.Button;
import android.widget.FrameLayout;
import android.widget.ScrollView;
import android.widget.TextView;
import java.util.Arrays;

/**
 * BtBridge is BibleText's native Android reading overlay — the twin of the iOS
 * UITextView in reading_ios.go. It owns a selectable TextView (inside a
 * ScrollView) floated ABOVE the Fyne GL surface, so the reader gets real
 * Android text selection: long-press handles, the floating toolbar, and our
 * study actions injected into it.
 *
 * The overlay is its OWN WINDOW (a Dialog), NOT a child view: NativeActivity
 * calls Window.takeSurface(), after which the activity's view hierarchy still
 * does layout and input but NEVER DRAWS — a plain addContentView child would be
 * laid out, hit-testable, and invisible. A separate window brings its own
 * surface, composited above the GL.
 *
 * It must be a DIALOG (not a bare WindowManager.addView) for TWO reasons:
 *  1. A Dialog window is TYPE_APPLICATION (2), outside the sub-window range
 *     (1000–1999) where the Editor force-disables selection handles
 *     (`windowSupportsHandles`).
 *  2. A Dialog has a real DecorView. Text selection's floating toolbar is
 *     created by DecorView.startActionModeForChild; a bare WindowManager root
 *     bubbles that call up to ViewRootImpl, which returns null — so the word
 *     selects but no toolbar ever appears.
 * The Dialog is non-dimming, non-modal (FLAG_NOT_TOUCH_MODAL — touches on the
 * Fyne header/tabs fall through), animation-free, and positioned to the Fyne
 * reading rect. BACK is forwarded to the activity so navigation still works.
 *
 * Ships as classes2.dex (ART auto-loads classesN.dex on API 21+), compiled by
 * scripts/build-android.sh. The Go side (reading_android.go) calls the static
 * methods below from a JNI-attached background thread (fyne driver.RunNative);
 * every UI mutation hops to the main thread through the UI handler. Callbacks
 * into Go are the two `native` methods, resolved by name against the app .so
 * (System.loadLibrary was already called by GoNativeActivity), implemented in
 * reading_android_export.go.
 */
public final class BtBridge {
    private static final Handler UI = new Handler(Looper.getMainLooper());

    private static Activity activity;
    private static Application.ActivityLifecycleCallbacks lifecycleCallbacks;
    private static Dialog dialog;
    private static FrameLayout root;   // hosts the ScrollView + the floating pill
    private static ScrollView scroll;
    private static FrameLayout content; // the scroll's one child: the TextView + the note sticker
    private static TextView text;

    // --- Shared-note sticker (full-screen reading) -------------------------
    // The Android twin of the iOS in-text note bubble (reading_ios.go
    // bibleTextSetNote): the Go side pushes the WHOLE presentation on every
    // chapter render — the sender's words, the WHO line (byline + honest
    // counts, the app's own chrome), pill vs bubble, whether the count region
    // is a next-tap control, the anchor verse, and the five palette colors —
    // and this side compares the tuple and re-derives the views only when it
    // changed (the iOS `changed` gate), so a presentation flip renders even
    // when the chapter Spanned was not re-pushed. The sticker lives INSIDE
    // the scroll content (a sibling of the TextView in `content`), so it
    // scrolls with the verse it is anchored to — no per-scroll tracking.
    //
    // Geometry, honestly: iOS reserves the band with paragraphSpacingBefore
    // on the anchor paragraph; a TextView has no per-paragraph spacing, so
    // the band here is a one-character LineHeightSpan (NoteBandSpan) on the
    // anchor verse's first character that raises that line's top by the
    // measured sticker height + gap, and the sticker view is positioned into
    // the reserved gap from the Layout's line geometry. Verse 0 (an
    // unplaced-only chapter) reserves nothing and parks the pill at the top
    // of the text — the only honest place for notes with no verses here.
    private static View noteView;                        // the expanded bubble
    private static TextView notePillView;                // the collapsed pill
    private static String noteText = null;   // sender's words; null = none
    private static String noteWho = null;    // the app's chrome line; null = none
    // noteTail is the PUSHED decision "does this card point at a passage". It is
    // not "is it collapsed": a note parked at chapter scope points at nothing,
    // and a tail there claims verse 1.
    // The verb sets, in noteChrome.verbs()'s own order (notes_chrome.go). WHICH
    // CONTROLS the card carries is decided in Go and pushed; this pane used to
    // decide it again from noteOwn, which is the same decision made twice.
    static final int VERBS_NONE = 0, VERBS_RECEIVED = 1, VERBS_OWN = 2;
    private static int noteVerbs = VERBS_RECEIVED;
    // noteCounts is the SUBSTRING of noteWho that is a control — the counts
    // phrase and its chevron, exactly as they appear in the line. This pane had
    // no split at all: the whole who line was painted accent when it was
    // nextable, so the sender's byline was coloured as though it too were
    // pressable. Composed in Go now (noteCountsSpan) and found here by
    // lastIndexOf, which still works after fitWho has ellipsised the sender.
    private static String noteCounts;
    // The arrival classes, in noteArrival's own order (notes_arrival.go), and
    // the class this render was pushed. WHERE the view goes is decided in Go;
    // this pane used to decide it with `noteAnchorVerse == pendingVerse` — same
    // VERSE — under a comment that said "when the band belongs to this verse's
    // paragraph". Bible paragraphs run to many verses, so a link to any other
    // verse of the note's own paragraph scrolled to the verse's line while the
    // band pushed the card above the fold: exactly the failure the comment
    // claimed to prevent.
    static final int ARRIVE_NOTHING = 0, ARRIVE_VERSE = 1, ARRIVE_BAND = 2;
    private static int noteArrival = ARRIVE_NOTHING;
    private static boolean noteTail = true;
    private static boolean notePill = false;
    private static boolean noteNextable = false;
    private static int noteAnchorVerse = 0;
    // Palette, pushed with every tuple (never hardcoded past these first-paint
    // fallbacks, which match the app's light parchment).
    private static int noteBg = 0xFFF7F3EA, noteFg = 0xFF262119, noteMuted = 0xFF6B6455,
                       noteAccent = 0xFF2E4C87, noteBorder = 0xFFBDB49E;
    // NOTE_DEBUG gates the sticker's own trace (what Go pushed, whether the
    // pane was laid out, what got built). Off in normal builds; flip it when
    // the sticker misbehaves — it is what found the arrival delivering the
    // WRONG note's tuple.
    static final boolean NOTE_DEBUG = false;
    // SCROLL_DEBUG is the Android twin of iOS's BT_SCROLL_DEBUG (reading_ios.go):
    // it traces WHERE the pane decided to land and why — the arrival verse, the
    // restore fraction, or the top. Off in normal builds. The cold-start arrival
    // is invisible without it: the sticker can be placed perfectly while the
    // scroll silently falls through to the top, and no screenshot can tell you
    // which of the three branches ran.
    static final boolean SCROLL_DEBUG = false;
    // noteOwn: the live note is one the READER WROTE, shown because they asked
    // NOTHING BRANCHES ON IT ANY MORE — noteVerbs carries that decision. It
    // stays on the wire, and in the changed-test, only so this bridge grows by
    // one appended parameter at a time (reading_ios.go says why).
    //
    // for it. It picks the closing control's mark and joins setNote's compare.
    private static boolean noteOwn;
    private static boolean noteRetryPending;
    private static NoteBandSpan noteBandSpan; // the STICKER's band, so placement can read it
    // EVERY applied band span, the sticker's included. The take-back sweeps BY
    // CLASS over the whole Spannable rather than through this list, because a
    // span whose handle is overwritten is orphaned on the live text with no
    // reference left to take it back by — a permanent phantom gap no host test
    // can see (this file does not compile on the dev host). The list exists for
    // PLACEMENT lookups: which paragraph carries which reservation.
    private static final java.util.ArrayList<NoteBandSpan> noteBandSpans =
            new java.util.ArrayList<NoteBandSpan>();

    // THE PUSHED BAND SPECS — the Apple twins' shape: which groups need a
    // reservation, where each hangs, and the pill's whole label, composed in
    // Go. Empty until the per-paragraph gate flips.
    private static int[] noteSpecKeys = new int[0];
    private static int[] noteSpecVerses = new int[0];
    private static String[] noteSpecLabels = new String[0];
    private static final java.util.ArrayList<TextView> notePillChips =
            new java.util.ArrayList<TextView>();

    /**
     * NoteBandSpan reserves the sticker's band: applied to ONE character (the
     * anchor verse's first), it raises that single line's top/ascent by the
     * band height, opening a gap above the verse for the sticker to sit in.
     * UpdateLayout is what makes DynamicLayout reflow when the span is added
     * or removed on the live Spannable.
     */
    // The band above the note's anchor verse — and two Android layout traps
    // that only the emulator could show (both measured, 19 Aug):
    //
    // 1. LineHeightSpan is a PARAGRAPH span: chooseHeight runs for EVERY line
    //    of the paragraph, not just the line the span covers. Adjusting
    //    unconditionally raised every following line by the sticker's whole
    //    height and tore the passage apart.
    // 2. Android reuses ONE FontMetricsInt across the paragraph's lines, so
    //    simply RETURNING for the other lines is not neutral: our subtraction
    //    is still sitting in the object, and the next line inherits it (traced:
    //    the anchor line and the one after it both came out with ascent -318,
    //    a 345px box where 85 was natural). A span that reserves space for one
    //    line must therefore put the metrics BACK on the next call.
    private static final class NoteBandSpan
            implements android.text.style.LineHeightSpan, android.text.style.UpdateLayout {
        final int band;
        final int at;
        // below: reserve the band under the line at `at` (the line BEFORE the
        // anchor verse) rather than above it. This is what keeps the verse's
        // WASH off the band: Android paints a character's background across
        // the whole line box, so inflating the ANCHOR line's ascent stretched
        // the highlight up into the reserved space and slid it under the
        // sticker. Reserving in the
        // previous line's descent puts the gap outside every washed character.
        final boolean below;
        // Which reservations this span carries: a span is ONE PER PARAGRAPH
        // with the heights SUMMED, because chooseHeight runs per paragraph over
        // one shared FontMetricsInt and two live spans there walk straight into
        // the traps documented above. pillPart is how much of `band` belongs to
        // pill reservations stacked ABOVE the sticker's own share (the styled
        // pane's ordering when a paragraph carries both).
        final int paraStart;
        final int pillPart;
        // The end offset of the line we adjusted, this layout pass. The next
        // call in the pass is the line that starts exactly there — the one
        // carrying our leftover metrics.
        private int inflatedTo = -1;
        NoteBandSpan(int band, int at, boolean below, int paraStart, int pillPart) {
            this.band = band; this.at = at; this.below = below;
            this.paraStart = paraStart; this.pillPart = pillPart;
        }
        @Override public void chooseHeight(CharSequence t, int start, int end,
                int spanstartv, int lineHeight, android.graphics.Paint.FontMetricsInt fm) {
            if (start <= at && at < end) {
                if (below) {
                    fm.descent += band;
                    fm.bottom += band;
                } else {
                    fm.ascent -= band;
                    fm.top -= band;
                }
                inflatedTo = end;
                return;
            }
            if (start == inflatedTo) {
                if (below) {
                    fm.descent -= band;
                    fm.bottom -= band;
                } else {
                    fm.ascent += band;
                    fm.top += band;
                }
                inflatedTo = -1;
            }
        }
    }

    // --- Read-along (audio) state ------------------------------------------
    // The floating "Follow narration" pill, a child of the overlay window (the
    // reading text paints ABOVE the Fyne canvas, so only a native view in this
    // same window can float over the verses). Shown/hidden by the Go controller.
    private static Button followBtn;
    private static boolean followWanted = false;

    // Selection-menu AI gate, mirroring the app's Settings → Assistant choice
    // ("None" turns AI off). Pushed from Go via setAIEnabled on init and whenever
    // the setting changes; onCreateActionMode reads it (on the UI thread) to
    // include or omit the "Study with AI" submenu. Defaults to on, matching the
    // preference's default. volatile: the write arrives on the Go/JNI-attached
    // thread, not the UI thread, so this is what formally guarantees the UI
    // thread sees the new value on the next selection.
    private static volatile boolean aiEnabled = true;

    // Verse index for the current chapter's Spanned, built in setHtml by scanning
    // the verse-number SuperscriptSpans (Html.fromHtml turns <sup> into one).
    // verseNums[i] is the verse number; [verseStarts[i], verseEnds[i]) is its whole
    // span (number + words), so highlighting matches the iOS read-along range.
    private static int[] verseNums = new int[0];
    private static int[] verseStarts = new int[0];
    private static int[] verseEnds = new int[0];
    // Where scripture ends and the appended translators'-footnote section
    // starts (== the Spanned's length when there is none). The section opens
    // with a sentinel <sup> holding a no-break space; buildVerseIndex records
    // its start here, bounding the last verse's span (and with it read-along
    // tint and scroll anchors) automatically, while the selection verbs clamp
    // to it below — the Android twin of the Apple panes' content-end.
    private static int contentEnd;
    // Where scripture STARTS: the first verse-number sup's offset. Characters
    // before it are the Psalm superscription — text with no verse identity —
    // so the app verbs clamp to [contentStart, contentEnd): title words can
    // never be dispatched or cited under verse 1's number.
    private static int contentStart;

    // The verse currently tinted and the span painting it (kept so each tick can
    // clear the previous cheaply — and so we remove OUR span, never the search
    // highlight's BackgroundColorSpan). raActive marks a live read-along;
    // raFollowing mirrors iOS's !gReadAlongUserLatch (auto-scroll armed vs the
    // reader has taken the scroll over).
    private static int raVerse = 0;
    private static BackgroundColorSpan raSpan;

    // The chapter wash, drawn in the LINE-BACKGROUND pass rather than as the
    // BackgroundColorSpan Html.fromHtml makes from the tint table's
    // background-color. Layout.draw paints line backgrounds first, then the
    // selection path, then the text pass — where TextLine fills a
    // BackgroundColorSpan's rect ABOVE the selection. An opaque wash written
    // as that span therefore buried the reader's selection: inside a washed
    // verse only the handles showed (measured on API 33 and 35 against an
    // unwashed control; the same defect UIKit had, for the same order). Here
    // the wash goes under the selection and the 0x33 text-colour highlight
    // shows through it, as it does on bare paper. The narration span stays a
    // BackgroundColorSpan, translucent, and composites over this — the same
    // "narration over the chapter wash" the other panes draw.
    //
    // Glyph-tight per line, like the fill it replaces: a line's painted run
    // starts at the first washed character and ends at the last, clamped to
    // the line's own extent so a wrapped line's trailing space and the break
    // between paragraphs stay bare (the shape every other pane paints).
    private static final class WashSpan implements android.text.style.LineBackgroundSpan {
        private final android.graphics.Paint fill = new android.graphics.Paint();
        WashSpan(int color) {
            fill.setColor(color);
            fill.setStyle(android.graphics.Paint.Style.FILL);
        }
        @Override public void drawBackground(android.graphics.Canvas c, android.graphics.Paint p,
                                             int left, int right, int top, int baseline, int bottom,
                                             CharSequence cs, int start, int end, int lnum) {
            if (text == null || !(cs instanceof Spanned)) return;
            Layout lay = text.getLayout();
            if (lay == null || lnum < 0 || lnum >= lay.getLineCount()) return;
            Spanned sp = (Spanned) cs;
            int lo = Math.max(start, sp.getSpanStart(this)), hi = Math.min(end, sp.getSpanEnd(this));
            if (lo >= hi) return;
            int lineStart = lay.getLineStart(lnum), lineEnd = lay.getLineEnd(lnum);
            float x0 = lo <= lineStart ? lay.getLineLeft(lnum) : lay.getPrimaryHorizontal(lo);
            // An offset AT the line end already belongs to the next line for
            // getPrimaryHorizontal; the line's own right extent is the
            // glyph-tight edge (it excludes trailing whitespace).
            float x1 = hi >= lineEnd ? lay.getLineRight(lnum) : lay.getPrimaryHorizontal(hi);
            if (x1 <= x0) return;
            // A NOTE'S RESERVED AIR IS NOT PART OF THE LINE. applyNoteBand
            // reserves a note's band by growing a line's descent (or, for a
            // chapter's first paragraph, its ascent), and this rect is drawn to
            // the line's FULL box — so a washed line carrying a band would fill
            // the gap the note is about to be drawn in.
            //
            // It never showed on the phone page because the band is reserved on
            // the BLANK line between paragraphs, which has no wash to fill it.
            // The reporter page has no blank line (imported COMPACT), so the
            // band lands on the previous paragraph's last INK line — and that
            // line is washed whenever its verse is.
            int t = top, b = bottom;
            for (NoteBandSpan ns : noteBandSpans) {
                if (ns.below && ns.at == lineEnd - 1) {
                    b -= ns.band;
                } else if (!ns.below && ns.at == lineStart) {
                    t += ns.band;
                }
            }
            if (b <= t) return;
            c.drawRect(x0, t, x1, b, fill);
        }
    }

    // The import's BackgroundColorSpans become WashSpans: every background the
    // Apple/Android stylesheet emits is a chapter wash (the tint table is the
    // only emitter; a contract test on the Go side holds that), and the
    // narration span is not in a freshly imported Spanned. Adjacent pieces of
    // one colour merge first — the table paints a verse as several spans
    // (number, body, join space) whose rects would meet at fractional x and
    // draw a hairline seam if filled one by one.
    private static void liftWashToLineBackground(Spannable sp) {
        BackgroundColorSpan[] spans = sp.getSpans(0, sp.length(), BackgroundColorSpan.class);
        if (spans.length == 0) return;
        java.util.Arrays.sort(spans, new java.util.Comparator<BackgroundColorSpan>() {
            @Override public int compare(BackgroundColorSpan a, BackgroundColorSpan b) {
                return sp.getSpanStart(a) - sp.getSpanStart(b);
            }
        });
        int runLo = -1, runHi = -1, runColor = 0;
        for (BackgroundColorSpan b : spans) {
            int lo = sp.getSpanStart(b), hi = sp.getSpanEnd(b), color = b.getBackgroundColor();
            sp.removeSpan(b);
            if (runLo >= 0 && color == runColor && lo <= runHi) {
                runHi = Math.max(runHi, hi);
                continue;
            }
            if (runLo >= 0) sp.setSpan(new WashSpan(runColor), runLo, runHi, Spannable.SPAN_EXCLUSIVE_EXCLUSIVE);
            runLo = lo; runHi = hi; runColor = color;
        }
        if (runLo >= 0) sp.setSpan(new WashSpan(runColor), runLo, runHi, Spannable.SPAN_EXCLUSIVE_EXCLUSIVE);
    }
    private static boolean raActive = false;
    private static boolean raFollowing = true;

    // Colors pushed from the app palette (setReadAlongColors): the amber verse
    // wash, and the pill's semi-transparent ground + label. Sensible fallbacks.
    private static int raHighlightColor = 0x52FFCC4D;   // amber @ ~0.32 alpha
    private static int followBgColor = 0xC72E4C87;       // lapis @ ~0.78 alpha
    private static int followFgColor = 0xFFF5F7FC;

    // Pending frame: RAW Fyne pixel coords relative to the activity window (the
    // decor's on-screen origin is added late, in applyFrame) + size, and whether
    // the overlay should be visible. The Dialog is only shown once a real frame
    // has arrived; wantShown remembers a show() that landed before that. UI-thread.
    private static int frameX, frameY, frameW = 1, frameH = 1;
    private static boolean wantShown = false;

    // Suppress latch, same contract as iOS gReadingSuppressed: while a Fyne
    // modal is up, ANY show() is a no-op — a stray layout pass behind the modal
    // must not repaint the verses over it. Touched only on the UI thread.
    private static boolean suppressed = false;

    // One-shot scroll restore (fraction of the scrollable height), applied after
    // the next layout pass that follows setHtml. Dropped on the first user
    // scroll. Guarded by the UI thread.
    private static float pendingFrac = -1f;

    // A width change reflows every line and changes the scroll range. Capture
    // the live fraction before the Dialog is resized, then reapply it after the
    // TextView has laid out at the new width.
    private static float pendingReflowFrac = -1f;

    // Last known scroll fraction, updated on every scroll change; read (racily,
    // but it is just a float) by the Go side when persisting reading position.
    private static volatile float lastFrac = 0f;

    // Set while OUR code drives scrollTo, so the scroll listener can tell a
    // restore/follow-scroll from a reader gesture (a reader gesture disarms
    // pendingFrac and schedules a position save). NOTE: ViewTreeObserver's
    // OnScrollChangedListener fires on the NEXT traversal, not synchronously
    // inside scrollTo — so ownScroll must stay latched for a short window AFTER a
    // programmatic scroll (endOwnScroll), or the async callback lands with
    // ownScroll already cleared and our own follow-scroll is misread as a reader
    // gesture (spuriously raising the "Follow narration" pill).
    private static boolean ownScroll = false;
    private static final Runnable clearOwnScroll = new Runnable() {
        @Override public void run() { ownScroll = false; }
    };
    // The Y of the last programmatic scrollTo. The 200ms ownScroll window can
    // expire before the async listener fires when the UI thread is throttled
    // (screen off / doze) — verified live: a lock-screen +15 skip's follow-scroll
    // was misread as a reader gesture and raised the pill. Position-compare is
    // timing-independent: scrollTo lands exactly on its target, so a callback at
    // that Y is ours. (A reader gesture landing on the same pixel just skips one
    // suspend — harmless.)
    private static int lastOwnScrollY = -1;

    // Quiet-timer for "scroll ended": Android has no deceleration-end callback
    // on ScrollView, so each scroll change re-arms a short timer and the save
    // fires when the view has been still for a beat.
    private static final Runnable scrollIdle = new Runnable() {
        @Override public void run() { nativeScrolled(lastFrac); }
    };

    // Called from native (any thread). Selection/menu action chosen by the
    // reader; text is the exact selected substring, lo/hi its verse span
    // resolved against the Spanned's verse index (verseAtOffset; 0,0 when the
    // index has nothing for it) — position, not text, decides which verses a
    // share or cross-reference cites.
    private static native void nativeSelectionAction(String action, String text, int lo, int hi);
    // Called on the UI thread after the reader's scroll has been idle ~200ms.
    private static native void nativeScrolled(float frac);

    // The soft keyboard's on-screen overlap in RAW PIXELS, 0 when hidden. Not dp
    // and not Fyne units — those two differ (a dp is dpi/160, a Fyne unit is a
    // point at dpi/72), and conflating them is what made the lift overshoot; the
    // Go side converts with the live canvas scale.
    // Feeds the Go-side goto picker so its bottom verse row lifts to sit on the
    // keyboard — the Android twin of iOS's bibleTextKeyboardChanged. Observed
    // rather than adjustResize'd ON PURPOSE: resizing the canvas under the IME
    // flips the live smallest-dimension tablet test on landscape tablets and
    // layoutWatcher thrashes the whole window mid-keystroke (reviewed, reverted
    // 2026-08-11). This feed touches nothing but a number.
    private static native void nativeKeyboardChanged(float overlapPx);
    // Called on the UI thread when the reader scrolls by hand during read-along
    // (suspends follow) and when they tap the "Follow narration" pill (resumes it).
    private static native void nativeReadAlongUserScrolled();
    private static native void nativeReadAlongFollowTapped();
    // The note sticker's verbs, all fired on the UI thread by the
    // sticker's own controls; each dispatches to the SAME Go verb the iOS
    // sticker calls (reading_android_export.go / reading_jni_android.c).
    private static native void nativeNoteNextTapped();
    private static native void nativeNoteHidden();
    private static native void nativeNoteDeleted();
    private static native void nativeNoteRestored();
    // The KEYED verb (notes_action.go): verb 1 = Restore; key = the group the
    // pressed band pill belongs to. The un-keyed natives above stay for the
    // sticker's own controls.
    private static native void nativeNoteAction(int verb, int key);
    // Called on the UI thread with the full URL of a shared bibletext.co.uk
    // link the user tapped (App Links). Go decides whether it is a passage
    // link and ignores anything else.
    private static native void nativeOpenedLink(String url);

    // deliverLaunchLink hands Go the shared link this activity was launched
    // from, if any. Android delivers an App Link as the launch Intent, so a
    // COLD tap (the common case — the app usually isn't running) is read here.
    //
    // consumed guards against re-delivery: init() runs again on things like a
    // configuration change, and re-reading the same Intent would yank the
    // reader back to the shared verse after they had navigated away.
    //
    // A tap while the app IS running arrives at onNewIntent, which the patched
    // GoNativeActivity (patches/fyne-2.7.4-android-newintent.patch) forwards to
    // onNewIntent below by reflection — WARM links are honoured now, not just
    // cold launches. singleTop (see the manifest) is what makes the running
    // activity receive it instead of a second instance being created.
    // Whether to offer writing a note. Set from Go on every reading-view build,
    // the same way aiOn gates the Study with AI item.
    // openInBrowser hands a shared link back out. A bare ACTION_VIEW would
    // resolve straight back to THIS app — we claim these links — so the default
    // browser is resolved and named explicitly. The probe URL is a generic https
    // one so the resolve cannot match our own filters.
    public static void openInBrowser(final String url) {
        final Activity act = activity;
        if (act == null || url == null || url.isEmpty()) return;
        act.runOnUiThread(new Runnable() { public void run() {
            try {
                android.content.pm.PackageManager pm = act.getPackageManager();
                android.content.Intent probe = new android.content.Intent(
                        android.content.Intent.ACTION_VIEW,
                        android.net.Uri.parse("https://example.invalid"));
                probe.addCategory(android.content.Intent.CATEGORY_BROWSABLE);
                android.content.pm.ResolveInfo ri = pm.resolveActivity(
                        probe, android.content.pm.PackageManager.MATCH_DEFAULT_ONLY);

                android.content.Intent go = new android.content.Intent(
                        android.content.Intent.ACTION_VIEW, android.net.Uri.parse(url));
                go.addCategory(android.content.Intent.CATEGORY_BROWSABLE);
                go.addFlags(android.content.Intent.FLAG_ACTIVITY_NEW_TASK);
                if (ri != null && ri.activityInfo != null
                        && !act.getPackageName().equals(ri.activityInfo.packageName)) {
                    go.setPackage(ri.activityInfo.packageName);
                }
                act.startActivity(go);
            } catch (Throwable t) {
                // No browser, or the chooser refused: leaving the reader where
                // they are beats crashing over a link we chose not to open.
            }
        }});
    }

    public static boolean notesEnabled = true;
    public static void setNotesEnabled(boolean on) { notesEnabled = on; }

    private static boolean launchLinkConsumed = false;

    private static void deliverLaunchLink(final Activity act) {
        if (act == null || launchLinkConsumed) return;
        try {
            android.content.Intent it = act.getIntent();
            if (it == null || !android.content.Intent.ACTION_VIEW.equals(it.getAction())) return;
            android.net.Uri data = it.getData();
            if (data == null) return;
            launchLinkConsumed = true;
            nativeOpenedLink(data.toString());
        } catch (Throwable ignored) {
            // A malformed intent must never stop the reader from opening.
        }
    }

    /** onNewIntent receives a shared link tapped while the app is RUNNING —
     *  the warm half of link delivery, forwarded from the patched
     *  GoNativeActivity by reflection (so fyne's Java never needs a compile-
     *  time reference to this class). Same delivery as deliverLaunchLink, but
     *  no consumed latch: every distinct tap deserves a delivery. */
    public static void onNewIntent(final android.content.Intent it) {
        try {
            if (it == null || !android.content.Intent.ACTION_VIEW.equals(it.getAction())) return;
            android.net.Uri data = it.getData();
            if (data == null) return;
            nativeOpenedLink(data.toString());
        } catch (Throwable ignored) {
            // A malformed intent must never disturb the running reader.
        }
    }

    private BtBridge() {}

    // setAIEnabled mirrors the app's Settings → Assistant choice; called from Go
    // on init and on change (from the Go/JNI thread — the volatile on aiEnabled
    // makes the write visible to the UI thread's onCreateActionMode read).
    public static void setAIEnabled(boolean on) { aiEnabled = on; }

    // The reading palette, as last pushed by setStyle — the study popup below
    // draws itself in it, so it belongs to the page it floats over.
    private static int lastTextColor = 0xFF000000, lastPaperColor = 0xFFFFFFFF;
    // Material list-item metrics for the study popup, in dp.
    private static final int STUDY_ROW_H = 48, STUDY_ROW_PAD_X = 16, STUDY_ROW_MIN_W = 196,
                             STUDY_CARD_PAD_Y = 8, STUDY_EDGE = 8, STUDY_GAP = 4;

    // showStudyPopup presents Explain / Analyze context / Analyze translation as
    // a popup at the selection — the inline "Study with AI" bar item's second
    // level (a floating toolbar cannot nest a real SubMenu inline). It used to
    // be a PopupMenu on a 1px anchor, which went wrong twice: PopupMenu shrinks
    // to the space BELOW its anchor and scrolls rather than flipping above, and
    // measures that space against the overlay window — which ends at the tab
    // bar — so a selection in the lower half clipped the last item; and it
    // took the overlay Dialog's theme, a white card over the dark reading
    // page. This popup is a Material-shaped list drawn in the reading palette,
    // placed below the selection's line when it fits and above it when it
    // does not, and kept inside the overlay either way.
    private static void showStudyPopup(final ActionMode mode, final String sel, int selEnd,
                                       final int selLo, final int selHi) {
        if (activity == null || root == null || text == null) return;
        float ax = 0; int lineTop = 0, lineBottom = 0;
        Layout lay = text.getLayout();
        if (lay != null && selEnd >= 0) {
            int line = lay.getLineForOffset(selEnd);
            // Layout coordinates exclude the view's padding; the view draws the
            // layout shifted by it. With the phone page's 10dp that error was
            // invisible, but the reporter page's centred column insets the text
            // by a large fraction of the width (applyReadingPadding) and the
            // popup would open that far to the left of the selection.
            ax = lay.getPrimaryHorizontal(selEnd) + text.getTotalPaddingLeft()
                    + text.getLeft() - scroll.getScrollX();
            lineTop = lay.getLineTop(line) + text.getTotalPaddingTop()
                    + text.getTop() - scroll.getScrollY();
            lineBottom = lay.getLineBottom(line) + text.getTotalPaddingTop()
                    + text.getTop() - scroll.getScrollY();
        }

        final android.widget.LinearLayout list = new android.widget.LinearLayout(activity);
        list.setOrientation(android.widget.LinearLayout.VERTICAL);
        android.graphics.drawable.GradientDrawable card = new android.graphics.drawable.GradientDrawable();
        card.setColor(lastPaperColor);
        card.setCornerRadius(dp(4));
        list.setBackground(card);
        list.setElevation(dp(8));
        list.setPadding(0, dp(STUDY_CARD_PAD_Y), 0, dp(STUDY_CARD_PAD_Y));

        final android.widget.PopupWindow pw = new android.widget.PopupWindow(list,
                FrameLayout.LayoutParams.WRAP_CONTENT, FrameLayout.LayoutParams.WRAP_CONTENT, true);
        pw.setOutsideTouchable(true);
        // A transparent window background so outside taps dismiss; the card
        // itself carries the paper colour and the shadow.
        pw.setBackgroundDrawable(new android.graphics.drawable.ColorDrawable(0));
        pw.setElevation(dp(8));

        android.util.TypedValue ripple = new android.util.TypedValue();
        activity.getTheme().resolveAttribute(android.R.attr.selectableItemBackground, ripple, true);
        String[][] items = {{"Explain", "explain"}, {"Analyze context", "context"},
                            {"Analyze translation", "translation"}};
        for (String[] it : items) {
            final String action = it[1];
            TextView row = new TextView(activity);
            row.setText(it[0]);
            row.setTextColor(lastTextColor);
            row.setTextSize(android.util.TypedValue.COMPLEX_UNIT_SP, 16);
            row.setPadding(dp(STUDY_ROW_PAD_X), 0, dp(STUDY_ROW_PAD_X), 0);
            row.setMinHeight(dp(STUDY_ROW_H));
            row.setMinWidth(dp(STUDY_ROW_MIN_W));
            row.setGravity(android.view.Gravity.CENTER_VERTICAL);
            if (ripple.resourceId != 0) row.setBackgroundResource(ripple.resourceId);
            row.setOnClickListener(new View.OnClickListener() {
                @Override public void onClick(View v) {
                    pw.dismiss();
                    try { mode.finish(); } catch (Throwable ignored) {}
                    nativeSelectionAction(action, sel, selLo, selHi);
                }
            });
            list.addView(row, new android.widget.LinearLayout.LayoutParams(
                    android.widget.LinearLayout.LayoutParams.MATCH_PARENT,
                    android.widget.LinearLayout.LayoutParams.WRAP_CONTENT));
        }

        list.measure(View.MeasureSpec.makeMeasureSpec(0, View.MeasureSpec.UNSPECIFIED),
                     View.MeasureSpec.makeMeasureSpec(0, View.MeasureSpec.UNSPECIFIED));
        int w = list.getMeasuredWidth(), h = list.getMeasuredHeight();
        int rootW = root.getWidth(), rootH = root.getHeight();
        int edge = dp(STUDY_EDGE), gap = dp(STUDY_GAP);
        int x = Math.max(edge, Math.min((int) ax, rootW - w - edge));
        int y = lineBottom + gap;
        if (y + h > rootH - edge) {
            y = lineTop - gap - h;                       // above the selection's line
            if (y < edge) y = Math.max(edge, rootH - h - edge); // nowhere fits: pin inside
        }
        pw.showAtLocation(root, android.view.Gravity.NO_GRAVITY, x, y);
    }

    /**
     * init stores the activity and builds the view lazily on the UI thread.
     * Called on every RunNative init AND whenever the activity changes: Android
     * recreates the activity (rotation, background→foreground), and a Dialog
     * cached against the destroyed activity's window token throws
     * BadTokenException on show(). So when a new activity arrives, tear the old
     * Dialog down and rebuild against the live one; the Go afterRebuild re-pushes
     * the frame + visibility, so we don't auto-show here.
     */
    // Last keyboard overlap reported, to fire the native callback only on change.
    private static float lastImePx = -1f;

    /**
     * Watch the soft keyboard's height on the ACTIVITY window (the window the
     * IME attaches to when a Fyne field focuses) and report its overlap in RAW
     * PIXELS — see the body comment: converting to dp here is exactly the bug.
     * API 30+ only: that is where WindowInsets delivers the IME type regardless
     * of softInputMode. Older releases keep the status quo (no lift) — the
     * alternative, adjustResize, breaks landscape tablets (see
     * nativeKeyboardChanged). The listener passes the insets through to the
     * platform handler, so NativeActivity's own inset processing is untouched.
     */
    private static void installKeyboardWatcher(final Activity act) {
        if (act == null || android.os.Build.VERSION.SDK_INT < 30) return;
        final android.view.View decor = act.getWindow().getDecorView();
        decor.setOnApplyWindowInsetsListener(new android.view.View.OnApplyWindowInsetsListener() {
            @Override
            public android.view.WindowInsets onApplyWindowInsets(android.view.View v, android.view.WindowInsets insets) {
                try {
                    // RAW PIXELS. Do not convert to dp here: Fyne's unit is a
                    // POINT (pixelsPerPt = dpi/72), not an Android dp (dpi/160),
                    // so a dp value read as Fyne units overstates the lift by
                    // ~2.2x. The Go side divides by the live canvas scale, the
                    // same px<->unit conversion pushChapterHTML already uses.
                    float px = insets.getInsets(android.view.WindowInsets.Type.ime()).bottom;
                    if (px != lastImePx) {
                        lastImePx = px;
                        nativeKeyboardChanged(px);
                    }
                } catch (Throwable ignored) {}
                return v.onApplyWindowInsets(insets);
            }
        });
    }

    public static void init(final Activity act) {
        UI.post(new Runnable() {
            @Override public void run() {
                if (activity == act && dialog != null) return;
                if (dialog != null) {
                    try { dialog.dismiss(); } catch (Throwable ignored) {}
                }
                dialog = null;
                root = null;
                scroll = null;
                content = null;
                text = null;
                followBtn = null;
                // The note sticker's views and band belonged to the destroyed
                // Dialog; reset the CACHED TUPLE too, or the re-pushed setNote
                // after recreation would compare equal and never rebuild them
                // (the read-along reset's lesson).
                noteView = null;
                notePillView = null;
                noteBandSpan = null;
                noteText = null;
                noteWho = null;
                notePill = false;
                noteNextable = false;
                noteAnchorVerse = 0;
                frameW = 1;
                frameH = 1;
                // wantShown/suppressed are re-asserted by the Go side after the
                // rebuild (notifyReadingOverlay / show/hide), so start neutral.
                wantShown = false;
                suppressed = false;
                // Read-along spans/index belonged to the destroyed view; reset. The
                // Go controller re-drives highlight/pill on the next audio tick.
                raSpan = null;
                raVerse = 0;
                raActive = false;
                raFollowing = true;
                followWanted = false;
                verseNums = new int[0];
                verseStarts = new int[0];
                verseEnds = new int[0];
                contentEnd = 0;
                contentStart = 0;
                pendingReflowFrac = -1f;
                activity = act;
                installKeyboardWatcher(act);
                deliverLaunchLink(act);

                if (lifecycleCallbacks == null && act != null) {
                    lifecycleCallbacks = new Application.ActivityLifecycleCallbacks() {
                        @Override public void onActivityCreated(Activity a, android.os.Bundle b) {}
                        @Override public void onActivityStarted(Activity a) {}
                        @Override public void onActivityResumed(Activity a) {}
                        @Override public void onActivityPaused(Activity a) {}
                        @Override public void onActivityStopped(Activity a) {}
                        @Override public void onActivitySaveInstanceState(Activity a, android.os.Bundle b) {}
                        @Override public void onActivityDestroyed(final Activity a) {
                            UI.post(new Runnable() {
                                @Override public void run() {
                                    if (activity == a) {
                                        if (dialog != null) {
                                            try { dialog.dismiss(); } catch (Throwable ignored) {}
                                        }
                                        dialog = null;
                                        root = null;
                                        scroll = null;
                                        content = null;
                                        text = null;
                                        followBtn = null;
                                        raSpan = null;
                                        noteView = null;
                                        notePillView = null;
                                        noteBandSpan = null;
                                        activity = null;
                                    }
                                }
                            });
                        }
                    };
                    act.getApplication().registerActivityLifecycleCallbacks(lifecycleCallbacks);
                }

                ensureView();
            }
        });
    }

    private static void ensureView() {
        if (text != null) return;

        text = new TextView(activity);
        text.setFocusable(true);
        text.setFocusableInTouchMode(true);
        text.setText(" ", TextView.BufferType.SPANNABLE); // Editor needs content before selectable arms
        text.setTextIsSelectable(true);
        text.setLongClickable(true);
        text.setVerticalScrollBarEnabled(false);
        // NO_OP TextClassifier: with the default (async, network-y) classifier
        // the smart-selection round-trip must complete before the floating
        // toolbar is created, and that async continuation fails to raise the
        // toolbar in a WindowManager-added overlay window — the word selects
        // but no menu appears. NO_OP keeps selection synchronous so the toolbar
        // shows immediately. We don't need smart-selection for scripture anyway.
        if (android.os.Build.VERSION.SDK_INT >= 26) {
            text.setTextClassifier(android.view.textclassifier.TextClassifier.NO_OP);
        }
        // Reading inset mirroring the iOS textContainerInset (14,10,14,10),
        // scaled by density on the Go side via setStyle padding args instead —
        // keep a sane default here.
        text.setCustomSelectionActionModeCallback(new ActionMode.Callback2() {
            @Override public boolean onCreateActionMode(ActionMode mode, Menu menu) {
                // Mirror the iOS selection menu: Study with AI leads, then the
                // Share pair, then Cross-references. Android's floating toolbar
                // can't show a SubMenu inline (only plain leaf items are eligible
                // for the visible bar) and it flattens SubMenus in the overflow —
                // so "Study with AI" is a PLAIN item pinned to the bar that opens
                // our own popup with the three actions (onActionItemClicked),
                // giving it the same lead position the iOS menu has. Keep Copy /
                // Select all; drop the system plain-text Share (ours, with the
                // reference, supersedes it) so there aren't two Shares.
                menu.removeItem(android.R.id.shareText);
                // Snapshot the volatile gate once so the menu is structurally
                // consistent even if it flips mid-build (read twice below).
                final boolean aiOn = aiEnabled;
                if (aiOn) {
                    MenuItem ai = menu.add(0, 200, 100, "Study with AI");
                    ai.setShowAsAction(MenuItem.SHOW_AS_ACTION_ALWAYS);
                }
                SubMenu sh = menu.addSubMenu(0, 201, 101, "Share");
                sh.add(0, 106, 0, "Share with citation");
                sh.add(0, 107, 1, "Share as image");
                sh.add(0, 108, 2, "Share as link");
                if (notesEnabled) sh.add(0, 109, 3, "Share with note");
                // Cross-references stays a plain root item: the toolbar may hoist
                // it inline after Study with AI when there's room (a bonus slot on
                // tablets), and it leads the custom items when AI is off — both
                // consistent with the iOS ordering.
                menu.add(0, 105, aiOn ? 102 : 100, "Cross-references");
                return true;
            }
            @Override public boolean onPrepareActionMode(ActionMode mode, Menu menu) { return false; }
            @Override public boolean onActionItemClicked(ActionMode mode, MenuItem item) {
                int a = text.getSelectionStart(), b = text.getSelectionEnd();
                if (a < 0 || b < 0) return true;
                int s0 = Math.min(a, b), s1 = Math.max(a, b);
                // ONLY the app's own verbs from here down. System items
                // (Copy, Select all) and the Share submenu header must fall
                // through to the TextView's default handling FIRST — a
                // `return true` on any of them consumes the click and the
                // verb silently dies. The system Copy therefore copies the
                // RAW selection, apparatus included, exactly as the Apple
                // panes leave the system verbs unclamped.
                int id = item.getItemId();
                boolean appItem = id == 200 || (id >= 105 && id <= 109);
                if (!appItem) return false;
                // Clamp the app's verbs to scripture (the Apple panes'
                // content-end contract): a selection wholly inside the
                // appended footnote section gets none of them — the
                // translators' words must never be dispatched or attributed
                // as scripture — and a straddling selection is cut at the
                // boundary. Clamped HERE, at click time: onPrepareActionMode
                // returns false, so handle drags and Select all move the
                // offsets without rebuilding the menu.
                int ce = contentEnd > 0 ? Math.min(contentEnd, text.getText().length()) : text.getText().length();
                if (s0 >= ce) {
                    mode.finish();
                    return true;
                }
                s1 = Math.min(s1, ce);
                // The same contract at the chapter's HEAD: a selection wholly
                // inside the Psalm superscription gets no app verbs, and one
                // straddling title and verse 1 is cut to the scripture half.
                if (s1 <= contentStart) { mode.finish(); return true; }
                s0 = Math.max(s0, contentStart);
                if (s1 <= s0) { mode.finish(); return true; }
                final String sel = text.getText().subSequence(s0, s1).toString();
                // The verse span, resolved NOW from the same offsets the text was
                // captured from (the popup below may collapse the selection).
                final int selLo = verseAtOffset(s0);
                final int selHi = s1 > s0 ? verseAtOffset(s1 - 1) : selLo;
                if (item.getItemId() == 200) {
                    // The inline "Study with AI": open our popup of the three
                    // actions, anchored at the selection. The selected text is
                    // captured NOW — opening the popup may collapse the selection
                    // and end the action mode.
                    showStudyPopup(mode, sel, s1, selLo, selHi);
                    return true;
                }
                String action;
                switch (item.getItemId()) {
                    case 105: action = "crossref"; break;
                    case 106: action = "share-cite"; break;
                    case 107: action = "share-image"; break;
                    case 108: action = "share-link"; break;
                    case 109: action = "share-link-note"; break;
                    default: return false; // submenu header (201) or system item
                }
                mode.finish();
                nativeSelectionAction(action, sel, selLo, selHi);
                return true;
            }
            @Override public void onDestroyActionMode(ActionMode mode) {}
            @Override public void onGetContentRect(ActionMode mode, View view, Rect outRect) {
                super.onGetContentRect(mode, view, outRect);
            }
        });

        // requestChildRectangleOnScreen is how a focused selectable TextView
        // drags its cursor into view — and after setText the cursor sits at the
        // END, so the first window-focus pass could yank a freshly positioned
        // chapter to its last verse. The scroll listener would then interpret
        // the jump as a reader gesture and persist the end position. Every
        // intentional scroll uses scrollTo, so automatic bring-into-view must
        // be disabled.
        scroll = new ScrollView(activity) {
            @Override
            public boolean requestChildRectangleOnScreen(View child, Rect rectangle, boolean immediate) {
                return false;
            }
        };
        scroll.setFillViewport(true);
        scroll.setVerticalScrollBarEnabled(true);
        // The scroll's one child is a FrameLayout holding the TextView AND the
        // note sticker: a sticker inside the scrolled content rides
        // its anchor verse natively, with no per-scroll repositioning. The
        // TextView's own geometry (getLeft/getTop = 0) is unchanged, so the
        // study-popup anchor math and scrollRange() still hold.
        content = new FrameLayout(activity);
        content.addView(text, new FrameLayout.LayoutParams(
                FrameLayout.LayoutParams.MATCH_PARENT, FrameLayout.LayoutParams.WRAP_CONTENT));
        // A cold Dialog can remain unattached until its Fyne frame arrives. Any
        // Handler/View.post retry queued before then may run before the first
        // measure, so the first real TextView layout is the durable liveness
        // signal for an arrival that is still waiting for verse geometry.
        text.addOnLayoutChangeListener(new View.OnLayoutChangeListener() {
            @Override public void onLayoutChange(View v, int l, int t, int r, int b,
                    int ol, int ot, int orr, int ob) {
                if (pendingVerse > 0) applyPendingScroll();
            }
        });
        // A real (or changed) WIDTH re-derives the sticker: the bubble wraps
        // its text at the content width, and the first refresh may have run
        // before any layout existed. Width only — the band itself changes the
        // HEIGHT on every refresh, and reacting to that would loop.
        content.addOnLayoutChangeListener(new View.OnLayoutChangeListener() {
            @Override public void onLayoutChange(View v, int l, int t, int r, int b,
                    int ol, int ot, int orr, int ob) {
                if ((r - l) != (orr - ol)) {
                    // The reporter column is centred against this width, so it
                    // is re-derived here too — the same first-layout and
                    // rotation cases the sticker needs.
                    applyReadingPadding();
                    refreshNoteSticker();
                    applyPendingReflow();
                }
            }
        });
        scroll.addView(content, new FrameLayout.LayoutParams(
                FrameLayout.LayoutParams.MATCH_PARENT, FrameLayout.LayoutParams.WRAP_CONTENT));

        // The floating "Follow narration" pill, laid out bottom-centre over the
        // verses, hidden until read-along suspends follow. Semi-transparent rounded
        // ground, tap resumes following. Built once; colors set by setReadAlongColors.
        followBtn = new Button(activity);
        followBtn.setText("Follow narration");
        followBtn.setAllCaps(false);
        followBtn.setTextSize(android.util.TypedValue.COMPLEX_UNIT_SP, 15f);
        followBtn.setVisibility(View.GONE);
        followBtn.setElevation(8f);
        styleFollowBtn();
        followBtn.setOnClickListener(new View.OnClickListener() {
            @Override public void onClick(View v) { nativeReadAlongFollowTapped(); }
        });
        FrameLayout.LayoutParams fp = new FrameLayout.LayoutParams(
                FrameLayout.LayoutParams.WRAP_CONTENT, FrameLayout.LayoutParams.WRAP_CONTENT);
        fp.gravity = Gravity.BOTTOM | Gravity.CENTER_HORIZONTAL;
        fp.bottomMargin = dp(18);

        root = new FrameLayout(activity);
        root.addView(scroll, new FrameLayout.LayoutParams(
                FrameLayout.LayoutParams.MATCH_PARENT, FrameLayout.LayoutParams.MATCH_PARENT));
        root.addView(followBtn, fp);

        dialog = new Dialog(activity, android.R.style.Theme_Translucent_NoTitleBar);
        dialog.setContentView(root);
        dialog.setCancelable(false); // BACK is forwarded to the activity, not consumed
        Window w = dialog.getWindow();
        w.setBackgroundDrawable(new ColorDrawable(Color.TRANSPARENT));
        w.clearFlags(WindowManager.LayoutParams.FLAG_DIM_BEHIND);
        w.addFlags(WindowManager.LayoutParams.FLAG_NOT_TOUCH_MODAL
                | WindowManager.LayoutParams.FLAG_LAYOUT_IN_SCREEN);
        WindowManager.LayoutParams lp = w.getAttributes();
        lp.gravity = Gravity.TOP | Gravity.START;
        lp.windowAnimations = 0;
        w.setAttributes(lp);
        // BACK on the overlay window drives the Fyne activity's back (nav/exit)
        // instead of dismissing the dialog.
        dialog.setOnKeyListener(new android.content.DialogInterface.OnKeyListener() {
            @Override public boolean onKey(android.content.DialogInterface d, int keyCode, KeyEvent e) {
                if (keyCode == KeyEvent.KEYCODE_BACK && e.getAction() == KeyEvent.ACTION_UP) {
                    activity.onBackPressed();
                    return true;
                }
                return false;
            }
        });

        scroll.getViewTreeObserver().addOnScrollChangedListener(
            new android.view.ViewTreeObserver.OnScrollChangedListener() {
                @Override public void onScrollChanged() {
                    int range = Math.max(1, scrollRange());
                    int y = scroll.getScrollY();
                    lastFrac = clamp01((float) y / range);
                    // Attaching and measuring a cold ScrollView can dispatch a
                    // scroll-change at y=0 even though no reader gesture occurred.
                    // An explicit arrival owns the first placement; keep it armed
                    // until applyPendingScroll resolves and consumes it.
                    if (pendingVerse > 0) return;
                    // Resizing the Dialog can clamp the old absolute scrollY
                    // before the new-width restore runs. That is reflow, not a
                    // reader gesture; applyPendingReflow owns this traversal.
                    if (pendingReflowFrac >= 0) return;
                    if (ownScroll) return;
                    if (lastOwnScrollY >= 0 && Math.abs(y - lastOwnScrollY) <= 2) {
                        return; // our own scrollTo arriving after the timed latch expired
                    }
                    // A reader gesture obsoletes any pending restore (frac and
                    // arrival verse alike), and the idle timer persists the new
                    // position once still.
                    lastOwnScrollY = -1;
                    pendingFrac = -1f;
                    pendingVerse = 0;
                    UI.removeCallbacks(scrollIdle);
                    UI.postDelayed(scrollIdle, 200);
                    // Read-along: a hand scroll while following suspends the
                    // follow (the highlight keeps tracking; the pill offers a way
                    // back). Fire once per suspension — iOS's gReadAlongUserLatch.
                    if (raActive && raFollowing) {
                        raFollowing = false;
                        nativeReadAlongUserScrolled();
                    }
                }
            });

        // The Dialog is not shown here — applyShow() shows it once a real frame
        // has arrived, so it never flashes at a 1×1 stub size.
    }

    private static int scrollRange() {
        View child = scroll.getChildAt(0);
        if (child == null) return 0;
        return Math.max(0, child.getHeight() - scroll.getHeight());
    }

    private static float clamp01(float f) { return f < 0 ? 0 : (f > 1 ? 1 : f); }

    // THE SHARED NOTE SPACING SPEC — noteMetrics in notes_bubble.go, in dp.
    // These are not this file's numbers to choose: notes_spacing_spec_test.go
    // parses this line and fails if any of them leaves the Go table behind. Read
    // that comment for what each one means. NOTE_WHO_H is a RULE in the table
    // (ceil(whoSize x 1.27)); 14 is its value at this pane's 11sp who font.
    private static final int NOTE_GAP_ABOVE = 10, NOTE_GAP_BELOW = 10, NOTE_PAD = 12,
                             NOTE_WHO_H = 14, NOTE_WHO_GAP = 4, NOTE_TAIL = 9,
                             NOTE_TAIL_W = 18, NOTE_TAIL_X = 24,
                             NOTE_RADIUS = 10, NOTE_PILL_H = 28;
    // The pill's side padding and width floor — spec too (noteMetrics PillPadX /
    // PillMinW), and NOT NOTE_PAD: the pill is one line of chrome and wants more
    // air beside its text than a paragraph does. This pane had NOTE_PAD and no
    // floor at all, so a short label ("Note") made a visibly smaller pill here
    // than on the phone beside it.
    private static final int NOTE_PILL_PAD_X = 14, NOTE_PILL_MIN_W = 86;
    // NOT spec: the verb button's size, this platform's own touch target. The
    // verbs used to sit IN the card's vertical flow, so an 18sp glyph plus its
    // padding — not the spec — set the who row's height and pushed the byline
    // about 8dp lower than on every other surface. They float over the card now,
    // exactly as they do on iOS, macOS and the styled pane.
    private static final int NOTE_BTN = 30;
    // NOT spec either: how big the DRAWN bin is inside that 30dp target. iOS
    // sets 12.5 inside its own 30pt button and macOS 12 inside its 24; this
    // pane shares iOS's 30, so it takes iOS's 12.5 rounded to a whole dp — the
    // mark is the same drawing on all three, sized to each one's own thumb.
    private static final int NOTE_TRASH = 13;
    // How far below the top of the viewport an arrival lands. SPEC (Lead): four
    // surfaces each had their own — 12 and 16 on the Apple panes, 24 on the
    // styled pane, 16 here — so the same arrival sat at four different heights.
    private static final int NOTE_LEAD = 16;

    private static int dp(int v) {
        float d = activity != null ? activity.getResources().getDisplayMetrics().density : 2f;
        return Math.round(v * d);
    }

    // beginOwnScroll/endOwnScroll bracket a programmatic scrollTo. endOwnScroll
    // keeps the latch up briefly so the async OnScrollChangedListener that the
    // scroll triggers still sees ownScroll==true (see the field comment).
    private static void beginOwnScroll() {
        UI.removeCallbacks(clearOwnScroll);
        ownScroll = true;
    }
    private static void endOwnScroll() {
        UI.removeCallbacks(clearOwnScroll);
        UI.postDelayed(clearOwnScroll, 200);
    }

    // ownScrollTo is the ONLY way our code scrolls the view: it records the
    // target (the timing-independent guard) and brackets the timed latch.
    private static void ownScrollTo(int y) {
        lastOwnScrollY = y;
        beginOwnScroll();
        scroll.scrollTo(0, y);
        endOwnScroll();
    }

    // ---- Read-along (audio): highlight the narrated verse + the "Follow" pill ----

    private static void styleFollowBtn() {
        if (followBtn == null) return;
        GradientDrawable bg = new GradientDrawable();
        bg.setShape(GradientDrawable.RECTANGLE);
        bg.setCornerRadius(dp(19));
        bg.setColor(followBgColor);
        followBtn.setBackground(bg);
        followBtn.setTextColor(followFgColor);
        int px = dp(18), py = dp(8);
        followBtn.setPadding(px, py, px, py);
    }

    // buildVerseIndex records each verse's number and char span for the current
    // chapter's Spanned by scanning the verse-number SuperscriptSpans (Html.fromHtml
    // turns each <sup> into one), in document order. A verse's span runs from its
    // number through just before the next verse's number (or end of text) — number +
    // words, so the tint covers the whole verse, matching the iOS read-along range.
    private static void buildVerseIndex(CharSequence cs) {
        verseNums = new int[0];
        verseStarts = new int[0];
        verseEnds = new int[0];
        contentEnd = cs != null ? cs.length() : 0;
        if (!(cs instanceof Spanned)) return;
        Spanned sp = (Spanned) cs;
        SuperscriptSpan[] sups = sp.getSpans(0, sp.length(), SuperscriptSpan.class);
        if (sups == null || sups.length == 0) return;
        int n = sups.length;
        int[] starts = new int[n];
        for (int i = 0; i < n; i++) starts[i] = sp.getSpanStart(sups[i]);
        Arrays.sort(starts);
        int[] nums = new int[n];
        int[] ends = new int[n];
        int count = 0;
        for (int i = 0; i < n; i++) {
            int st = starts[i];
            int en = (i + 1 < n) ? starts[i + 1] : sp.length();
            int num = parseLeadingInt(sp, st, en);
            if (num <= 0) {
                // The only non-digit sup in the dialect is the footnote
                // section's sentinel: its start is the scripture/apparatus
                // boundary. First one wins (there is only one; being
                // defensive about a second costs nothing).
                if (st < contentEnd) contentEnd = st;
                continue;
            }
            nums[count] = num;
            starts[count] = st;
            ends[count] = en;
            count++;
        }
        verseNums = Arrays.copyOf(nums, count);
        verseStarts = Arrays.copyOf(starts, count);
        verseEnds = Arrays.copyOf(ends, count);
        contentStart = count > 0 ? verseStarts[0] : 0;
    }

    // parseLeadingInt reads the run of digits starting at `from` (the verse number
    // sits at the very start of the superscript run); returns -1 if none.
    private static int parseLeadingInt(CharSequence cs, int from, int to) {
        int i = from, val = 0;
        boolean any = false;
        while (i < to) {
            char c = cs.charAt(i);
            if (c < '0' || c > '9') break;
            val = val * 10 + (c - '0');
            any = true;
            i++;
        }
        return any ? val : -1;
    }

    // verseAtOffset returns the verse whose number run is the last at or before
    // character offset `off` in the current Spanned (0 = above verse 1's number,
    // i.e. the chapter heading — the Go side clamps). verseStarts is in document
    // order, so the last start <= off wins; an offset inside a verse's NUMBER
    // resolves to that verse, matching the iOS btIOSVerseAtIndex contract.
    private static int verseAtOffset(int off) {
        int v = 0;
        for (int i = 0; i < verseNums.length; i++) {
            if (verseStarts[i] > off) break;
            v = verseNums[i];
        }
        return v;
    }

    /**
     * trimTrailingBlank pulls a verse range back off the whitespace that
     * follows its last word — the Apple panes' rule (btIOSRunWashRange), which
     * this side had never needed.
     *
     * A verse's span runs to the NEXT verse number, so it swallows the break
     * between two paragraphs. Painting that leaves a tag hanging off the end of
     * the passage, and on the reporter page the break is followed by the next
     * paragraph's em+en indent — two wide spaces that took the narration colour
     * as a block sitting alone at the head of a paragraph that is not being
     * read.
     */
    private static int[] trimTrailingBlank(CharSequence sp, int[] r) {
        if (r == null || r.length != 2) return r;
        int end = Math.min(r[1], sp.length());
        while (end > r[0]) {
            char c = sp.charAt(end - 1);
            if (Character.isWhitespace(c) || c == '\u00a0' || c == '\u2002' || c == '\u2003') end--;
            else break;
        }
        return new int[]{r[0], end};
    }

    // verseRange returns {start,end} for a verse number, or null if not indexed.
    private static int[] verseRange(int verse) {
        for (int i = 0; i < verseNums.length; i++) {
            if (verseNums[i] == verse) return new int[]{verseStarts[i], verseEnds[i]};
        }
        return null;
    }

    /** readAlongHighlight tints the narrated verse (clearing the previous) and, when
     *  follow is set, scrolls it into a comfortable band. verse<=0 just clears. */
    public static void readAlongHighlight(final int verse, final boolean follow) {
        UI.post(new Runnable() {
            @Override public void run() {
                if (text == null) return;
                CharSequence cs = text.getText();
                if (!(cs instanceof Spannable)) return;
                Spannable sp = (Spannable) cs;
                if (raSpan != null) {
                    try { sp.removeSpan(raSpan); } catch (Throwable ignored) {}
                    raSpan = null;
                }
                raVerse = 0;
                if (verse > 0) {
                    int[] r = verseRange(verse);
                    if (r != null) r = trimTrailingBlank(sp, r);
                    if (r != null && r[0] >= 0 && r[1] <= sp.length() && r[0] < r[1]) {
                        raSpan = new BackgroundColorSpan(raHighlightColor);
                        sp.setSpan(raSpan, r[0], r[1], Spannable.SPAN_INCLUSIVE_EXCLUSIVE);
                        raVerse = verse;
                    }
                }
                raActive = (verse > 0);
                if (follow) raFollowing = true;   // following again → re-arm the latch
                if (!follow || raVerse == 0) return;
                followScrollTo(raVerse);
            }
        });
    }

    // followScrollTo nudges the verse into view only when it has drifted out of a
    // comfortable band (above the fold, or past 70% down), so the text isn't yanked
    // on every verse. ownScroll marks it as our scroll so the listener won't treat
    // it as a reader gesture.
    private static void followScrollTo(int verse) {
        if (text == null || scroll == null) return;
        Layout layout = text.getLayout();
        if (layout == null) return;   // not laid out yet — the next tick catches it
        int[] r = verseRange(verse);
        if (r == null) return;
        int line = layout.getLineForOffset(r[0]);
        int verseY = text.getTotalPaddingTop() + layout.getLineTop(line);
        int scrollY = scroll.getScrollY();
        int visH = scroll.getHeight();
        if (visH <= 0) return;
        if (verseY < scrollY || verseY > scrollY + visH * 0.70f) {
            int target = Math.round(verseY - visH * 0.30f);
            int maxY = scrollRange();
            if (target < 0) target = 0;
            if (target > maxY) target = maxY;
            ownScrollTo(target);
        }
    }

    /** readAlongClear removes the tint (stop/nav). */
    public static void readAlongClear() {
        UI.post(new Runnable() {
            @Override public void run() {
                if (text != null && text.getText() instanceof Spannable && raSpan != null) {
                    try { ((Spannable) text.getText()).removeSpan(raSpan); } catch (Throwable ignored) {}
                }
                raSpan = null;
                raVerse = 0;
                raActive = false;
                raFollowing = true;
            }
        });
    }

    /** readAlongFollow shows/hides the floating "Follow narration" pill. */
    public static void readAlongFollow(final boolean show) {
        UI.post(new Runnable() {
            @Override public void run() {
                followWanted = show;
                applyFollowVisibility();
            }
        });
    }

    // applyFollowVisibility resolves the pill's real visibility: wanted by the
    // controller AND the reading overlay itself is up (a modal or tab switch that
    // hides the verses must take the pill down too).
    private static void applyFollowVisibility() {
        if (followBtn == null) return;
        boolean up = dialog != null && dialog.isShowing() && !suppressed;
        followBtn.setVisibility((followWanted && up) ? View.VISIBLE : View.GONE);
        if (followWanted && up) followBtn.bringToFront();
    }

    /** setReadAlongColors pushes the palette-derived amber wash + pill colors. */
    public static void setReadAlongColors(final int highlight, final int followBg, final int followFg) {
        UI.post(new Runnable() {
            @Override public void run() {
                raHighlightColor = highlight;
                followBgColor = followBg;
                followFgColor = followFg;
                styleFollowBtn();
            }
        });
    }

    // ---- Shared-note sticker (full-screen reading) -------------------------

    /**
     * setNote pushes the sticker tuple from Go (androidStickerPush — the same
     * composition as the iOS bibleTextSetNote push): sender text, WHO line,
     * pill / nextable presentation, anchor verse, and the five palette colors
     * as ARGB ints (surface, text, muted, accent, border). Compare-and-refresh:
     * the views are re-derived only when the tuple changed, so a presentation
     * flip renders without a chapter re-push, and an unchanged push is free.
     * Colors alone never change without a body change (the theme variant is
     * folded into the chapter fingerprint, and that re-push runs setHtml →
     * refresh), so they do not join the compare — the iOS rule.
     */
    // text/who arrive as UTF-8 BYTES, not jstrings: a note body with emoji is
    // 4-byte UTF-8, which is invalid modified UTF-8 and can abort NewStringUTF
    // under CheckJNI. Decoded here with the real charset; null/empty = absent.
    public static void setNote(final byte[] noteText_, final byte[] who_, final boolean pill,
                               final boolean nextable, final boolean own, final int anchorVerse,
                               final int bg, final int fg, final int muted,
                               final int accent, final int border, final boolean tail,
                               final int verbs, final byte[] counts_, final int arrival) {
        UI.post(new Runnable() {
            @Override public void run() {
                String t = (noteText_ == null || noteText_.length == 0)
                        ? null : new String(noteText_, java.nio.charset.StandardCharsets.UTF_8);
                String w = (who_ == null || who_.length == 0)
                        ? null : new String(who_, java.nio.charset.StandardCharsets.UTF_8);
                String counts = (counts_ == null || counts_.length == 0)
                        ? null : new String(counts_, java.nio.charset.StandardCharsets.UTF_8);
                boolean changed = !sameStr(t, noteText) || !sameStr(w, noteWho)
                        || notePill != pill || noteNextable != nextable
                        || noteTail != tail || noteVerbs != verbs
                        || !java.util.Objects.equals(noteCounts, counts)
                        || noteArrival != arrival
                        || noteOwn != own
                        || noteAnchorVerse != anchorVerse;
                noteText = t;
                noteWho = w;
                notePill = pill;
                noteNextable = nextable;
                noteTail = tail;
                noteVerbs = verbs;
                noteCounts = counts;
                noteArrival = arrival;
                noteOwn = own;
                noteAnchorVerse = anchorVerse;
                noteBg = bg;
                noteFg = fg;
                noteMuted = muted;
                noteAccent = accent;
                noteBorder = border;
                if (NOTE_DEBUG) android.util.Log.i("BtNote", "setNote t=" + (t == null ? "null" : t.length() + "ch")
                        + " w=" + w + " pill=" + pill + " anchor=" + anchorVerse + " changed=" + changed);
                if (changed) refreshNoteSticker();
            }
        });
    }

    private static boolean sameStr(String a, String b) { return a == null ? b == null : a.equals(b); }

    // setNoteBands pushes the per-paragraph band specs (the Apple panes'
    // bibleTextSetNoteBands): parallel key/verse arrays plus the pill labels
    // '\n'-joined — safe because sender names are sanitized newline-free
    // before they reach any label. Empty arrays turn the machinery off.
    // UI-thread hop and changed-compare mirror setNote.
    public static void setNoteBands(final int[] keys, final int[] verses, final byte[] labelsJoined) {
        UI.post(new Runnable() {
            @Override public void run() {
                int[] k = keys == null ? new int[0] : keys;
                int[] v = verses == null ? new int[0] : verses;
                String joined = (labelsJoined == null || labelsJoined.length == 0)
                        ? "" : new String(labelsJoined, java.nio.charset.StandardCharsets.UTF_8);
                String[] l = joined.isEmpty() ? new String[0] : joined.split("\n", -1);
                boolean changed = !java.util.Arrays.equals(k, noteSpecKeys)
                        || !java.util.Arrays.equals(v, noteSpecVerses)
                        || !java.util.Arrays.equals(l, noteSpecLabels);
                noteSpecKeys = k;
                noteSpecVerses = v;
                noteSpecLabels = l;
                if (NOTE_DEBUG) android.util.Log.i("BtNote", "setNoteBands: n=" + k.length + " changed=" + changed);
                if (changed) refreshNoteSticker();
            }
        });
    }

    // notePresent / notePillNow are the iOS btIOSNotePresent / btIOSNotePill
    // questions verbatim: the sticker exists whenever text OR who does, and
    // "who without text" (an unplaced-only chapter) collapses to the pill by
    // construction — no sender words exist, and an empty sender bubble must
    // never render.
    private static boolean notePresent() { return noteText != null || noteWho != null; }
    private static boolean notePillNow() { return notePill || noteText == null; }

    /**
     * refreshNoteSticker tears the sticker down and re-derives it — views,
     * band, placement — against the text already on screen. UI thread only.
     * Runs from setNote (tuple changed), setHtml (fresh Spanned + verse
     * index), and the content width listener (first layout, rotation).
     */
    // btPillStackInkTop is notePillSeparatorLift's mirror (notes_bubble.go owns
    // the rule): a collapsed stack whose bottom neighbour is the passage
    // centres in the VISIBLE inter-paragraph air — the previous paragraph's
    // ink bottom (its last baseline plus the paint's descent) to the noted
    // paragraph's first ink top (its first baseline minus the cap height).
    // Centring in BOX space read wrong: the layout's leading is not split
    // evenly around the glyphs, so a box-centred pill carried the slack under
    // it and visibly hugged the wrong side, worse at larger text sizes. The
    // separator here is the blank line Html.fromHtml puts between paragraphs;
    // a band hanging off an INK line (a poem line's '\n', which paraStartAt
    // also stops at) has no separator above it and stands down: the blank
    // line is the one whose text is only the newline, i.e. the line STARTS at
    // the span's own character. Returns -1 to stand down; the caller keeps
    // the band-top arithmetic.
    private static int btPillStackInkTop(Layout lay, NoteBandSpan span, int ps,
                                         int stackH, int capH) {
        if (lay == null || span == null || !span.below || ps <= 0) return -1;
        int sl = lay.getLineForOffset(ps - 1);
        if (lay.getLineStart(sl) != ps - 1) return -1; // an ink line: mid-poem, no separator
        if (sl < 1) return -1;                          // nothing above the blank line
        // OPTICAL edges, not metric extremes: a text block's bottom edge is
        // its BASELINE (descenders are sparse; centring to baseline+descent
        // sat the pill low by half the descent), and its top edge is the cap
        // line.
        float inkBottom = lay.getLineBaseline(sl - 1);
        float inkTop = lay.getLineBaseline(lay.getLineForOffset(ps)) - capH;
        float gap = inkTop - inkBottom;
        if (gap <= stackH) return -1;
        return Math.round(inkBottom + (gap - stackH) / 2f);
    }

    private static void refreshNoteSticker() {
        if (content == null || text == null) {
            if (NOTE_DEBUG) android.util.Log.i("BtNote", "refresh: no content/text yet");
            return;
        }
        if (noteView != null) { content.removeView(noteView); noteView = null; }
        if (notePillView != null) { content.removeView(notePillView); notePillView = null; }
        for (TextView c : notePillChips) content.removeView(c);
        notePillChips.clear();
        clearNoteBand();
        if (!notePresent()) {
            if (NOTE_DEBUG) android.util.Log.i("BtNote", "refresh: notePresent=false — nothing to draw");
            return;
        }
        // The card sits over the COLUMN, not the view. On the phone page the
        // text's own side padding is that same dp(10), so this is unchanged
        // there; on the reporter page the column is inset far more
        // (applyReadingPadding), and a full-width card over a narrow column
        // reads as a different surface's furniture.
        int side = Math.max(dp(10), text != null ? text.getPaddingLeft() : dp(10));
        int wpx = content.getWidth() - 2 * side;
        if (wpx < dp(60)) {
            // No real layout yet. The width LISTENER only fires when the width
            // CHANGES, so on an arrival into an already-sized pane it never
            // fires again and the sticker is simply never built — the band is
            // reserved and the card is missing, leaving the highlight too high
            // over an empty reservation. Ask again after this layout pass instead of
            // waiting for a change that will not come.
            if (NOTE_DEBUG) android.util.Log.i("BtNote", "refresh: content width too small (" + wpx + ") — retrying");
            if (!noteRetryPending) {
                noteRetryPending = true;
                content.post(new Runnable() {
                    @Override public void run() {
                        noteRetryPending = false;
                        refreshNoteSticker();
                    }
                });
            }
            return;
        }

        boolean pillNow = notePillNow();
        // THE STAND-DOWN (the Apple panes' rule): when per-paragraph specs
        // exist, the single sticker-as-pill retires — the specs carry every
        // count between them, so drawing both would say the same thing twice.
        // The expanded bubble still draws; the specs mark the paragraphs it
        // does not.
        boolean standDown = pillNow && noteSpecKeys.length > 0;

        final int gapAbove = dp(NOTE_GAP_ABOVE), gapBelow = dp(NOTE_GAP_BELOW);
        View sticker = null;
        FrameLayout.LayoutParams lp = null;
        int noteH = 0;
        if (!standDown) {
            sticker = pillNow ? buildNotePill() : buildNoteBubble();
            sticker.setElevation(6f);
            if (pillNow) {
                // The pill sizes to its label (capped at the content width).
                sticker.measure(View.MeasureSpec.makeMeasureSpec(wpx, View.MeasureSpec.AT_MOST),
                                View.MeasureSpec.makeMeasureSpec(0, View.MeasureSpec.UNSPECIFIED));
                lp = new FrameLayout.LayoutParams(FrameLayout.LayoutParams.WRAP_CONTENT,
                                                  FrameLayout.LayoutParams.WRAP_CONTENT);
            } else {
                // The bubble is a full-width card, wrapped at the content measure.
                sticker.measure(View.MeasureSpec.makeMeasureSpec(wpx, View.MeasureSpec.EXACTLY),
                                View.MeasureSpec.makeMeasureSpec(0, View.MeasureSpec.UNSPECIFIED));
                lp = new FrameLayout.LayoutParams(wpx, FrameLayout.LayoutParams.WRAP_CONTENT);
            }
            lp.leftMargin = side;
            noteH = sticker.getMeasuredHeight();
        }

        if (NOTE_DEBUG) android.util.Log.i("BtNote", "refresh: building, wpx=" + wpx + " pill=" + pillNow
                + " standDown=" + standDown + " specs=" + noteSpecKeys.length
                + " anchor=" + noteAnchorVerse + " verseIdx=" + verseNums.length);
        int[] r = (sticker != null && noteAnchorVerse > 0) ? verseRange(noteAnchorVerse) : null;
        if (sticker != null && r == null) {
            // Nothing to anchor to (verse 0 = unplaced-only, or a verse this
            // translation's index does not carry): park at the top of the
            // text with no band — the only honest place for notes with no
            // verses here (the iOS top-inset case, minus the reservation).
            lp.topMargin = dp(6);
            content.addView(sticker, lp);
            sticker = null; // parked; only the spec chips still need placing
        }
        // A gap on BOTH sides of the card — the styled pane's symmetry rule.
        // Reserving only below left the card butting against the line above
        // (0 against gap+tail), unlike the symmetric spacing on the other
        // platforms. Sticker offset -1 = "no sticker band" (stood down, parked,
        // or absent); the pushed specs still reserve theirs.
        applyNoteBand(sticker != null ? r[0] : -1,
                      sticker != null ? gapAbove + noteH + gapBelow : 0);

        // One chip per pushed spec, styled as the pill, pressed as the KEYED
        // Restore. Added hidden; the placement runnable reveals each in its
        // paragraph's reservation.
        for (int i = 0; i < noteSpecKeys.length; i++) {
            String label = i < noteSpecLabels.length ? noteSpecLabels[i] : null;
            TextView chip = buildNoteBandChip(noteSpecKeys[i], label);
            chip.setElevation(6f);
            chip.measure(View.MeasureSpec.makeMeasureSpec(wpx, View.MeasureSpec.AT_MOST),
                         View.MeasureSpec.makeMeasureSpec(0, View.MeasureSpec.UNSPECIFIED));
            FrameLayout.LayoutParams clp = new FrameLayout.LayoutParams(
                    FrameLayout.LayoutParams.WRAP_CONTENT, FrameLayout.LayoutParams.WRAP_CONTENT);
            clp.leftMargin = side;
            content.addView(chip, clp);
            chip.setVisibility(View.INVISIBLE); // never flash at 0,0 before placement
            notePillChips.add(chip);
        }

        // Place everything into the reserved gaps AFTER the reflow the bands
        // just caused, in ONE posted pass.
        final View vv = sticker;
        final boolean pillForm = pillNow;
        final int off = r != null ? r[0] : -1;
        if (vv != null) {
            content.addView(vv, lp);
            vv.setVisibility(View.INVISIBLE); // never flash at 0,0 before placement
        }
        if (vv == null && notePillChips.isEmpty()) return;
        text.post(new Runnable() {
            @Override public void run() {
                if (text == null) return;
                Layout lay = text.getLayout();
                if (lay == null) return; // hidden overlay: the next refresh places it
                int textTop = text.getTop() + text.getTotalPaddingTop();
                CharSequence cs3 = text.getText();
                // The body cap height, for the ink-centering rule: measured
                // off the live paint, so every text-size setting is honest.
                android.graphics.Rect capB = new android.graphics.Rect();
                text.getPaint().getTextBounds("N", 0, 1, capB);
                final int capH = capB.height();

                if (vv != null && vv.getParent() != null && off >= 0) {
                    // The band belongs to the paragraph, so the sticker hangs from
                    // the paragraph's first line — not the anchor verse's line.
                    int paraOff = paraStartAt(cs3, off);
                    int line = lay.getLineForOffset(paraOff);
                    // The reserved gap is [lineTop - band, lineTop): it lives at the
                    // bottom of the PREVIOUS line's box now (applyNoteBand), so the
                    // sticker hangs from there rather than from the anchor line's
                    // own top. The first-character case has no previous line and
                    // keeps the old ascent reservation, where lineTop IS the gap's
                    // top.
                    int lineTop = lay.getLineTop(line);
                    int gapTop = (noteBandSpan != null && noteBandSpan.below)
                            ? lineTop - noteBandSpan.band : lineTop;
                    // Below any pill share first: pills stack ABOVE the sticker
                    // when a paragraph carries both (the styled pane's ordering).
                    int pillPart = noteBandSpan != null ? noteBandSpan.pillPart : 0;
                    if (NOTE_DEBUG) {
                        int pl = line > 0 ? line - 1 : 0;
                        android.util.Log.i("BtNote", "geom: paraLine=" + line
                                + " top=" + lay.getLineTop(line)
                                + " bottom=" + lay.getLineBottom(line)
                                + " base=" + lay.getLineBaseline(line)
                                + " asc=" + lay.getLineAscent(line)
                                + " desc=" + lay.getLineDescent(line)
                                + " | prev top=" + lay.getLineTop(pl)
                                + " bottom=" + lay.getLineBottom(pl)
                                + " asc=" + lay.getLineAscent(pl)
                                + " desc=" + lay.getLineDescent(pl)
                                + " | band=" + (noteBandSpan != null ? noteBandSpan.band : -1)
                                + " below=" + (noteBandSpan != null && noteBandSpan.below)
                                + " pillPart=" + pillPart
                                + " paraOff=" + paraOff + " off=" + off);
                    }
                    // The card hangs gapAbove below its share's top edge — the
                    // same arithmetic the styled pane's place() and iOS's
                    // btIOSLayoutNote use.
                    int top = textTop + gapTop + pillPart + gapAbove;
                    // The single collapsed pill centres in the visible air
                    // (btPillStackInkTop, its stack being itself alone); the
                    // OPEN card never does — its tail's distance to the
                    // passage is the pinned invariant.
                    if (pillForm) {
                        int ink = btPillStackInkTop(lay, noteBandSpan, paraOff,
                                vv.getMeasuredHeight(), capH);
                        if (ink >= 0) top = textTop + ink;
                    }
                    FrameLayout.LayoutParams p = (FrameLayout.LayoutParams) vv.getLayoutParams();
                    p.topMargin = Math.max(0, top);
                    vv.setLayoutParams(p);
                    vv.setVisibility(View.VISIBLE);
                }

                // Each chip hangs in ITS paragraph's reservation, stacked in
                // spec order when two specs share one (verse-0 "unplaced" lands
                // on paragraph 0 beside a verse-1 group's).
                int pillBand = gapAbove + dp(NOTE_PILL_H) + gapBelow;
                java.util.HashMap<Integer, Integer> stacked = new java.util.HashMap<Integer, Integer>();
                for (int i = 0; i < notePillChips.size() && i < noteSpecVerses.length; i++) {
                    TextView chip = notePillChips.get(i);
                    if (chip.getParent() == null) continue;
                    int soff = 0;
                    if (noteSpecVerses[i] > 0) {
                        int[] sr = verseRange(noteSpecVerses[i]);
                        if (sr == null) continue; // never reserved; stays hidden
                        soff = sr[0];
                    }
                    int ps = paraStartAt(cs3, soff);
                    NoteBandSpan span = null;
                    for (NoteBandSpan b : noteBandSpans) {
                        if (b.paraStart == ps) { span = b; break; }
                    }
                    if (span == null) continue;
                    int lineTop = lay.getLineTop(lay.getLineForOffset(ps));
                    int bandTop = span.below ? lineTop - span.band : lineTop;
                    Integer prev = stacked.get(ps);
                    int slot = prev == null ? 0 : prev;
                    stacked.put(ps, slot + 1);
                    // The centering rule: the chip centres in the visible
                    // inter-paragraph air (btPillStackInkTop) — except beside
                    // a live OPEN card sharing this paragraph, which owns the
                    // bottom air (a live sticker with chips present is always
                    // the card: the pill form stands down when specs exist).
                    int ink = btPillStackInkTop(lay, span, ps, chip.getMeasuredHeight(), capH);
                    if (vv != null && vv.getParent() != null && off >= 0
                            && paraStartAt(cs3, off) == ps) ink = -1;
                    int ctop = ink >= 0 ? textTop + ink + slot * pillBand
                            : textTop + bandTop + slot * pillBand + gapAbove;
                    FrameLayout.LayoutParams cp = (FrameLayout.LayoutParams) chip.getLayoutParams();
                    cp.topMargin = Math.max(0, ctop);
                    chip.setLayoutParams(cp);
                    chip.setVisibility(View.VISIBLE);
                }
            }
        });
    }

    /**
     * buildNoteBubble is the expanded sticker: a rounded card in the pushed
     * surface + border colors, the WHO line (byline + honest counts), the
     * sender's words below it, and the Hide / Delete verbs FLOATED over the
     * card's top right — out of its vertical flow, exactly as on iOS, macOS
     * and the styled pane. The card's own rhythm is the shared spec's
     * (noteMetrics, notes_bubble.go), not this file's.
     */
    // NoteBubbleDrawable paints the card and its speech TAIL as one shape — the
    // thing that makes a note read as somebody speaking rather than as a system
    // card. Geometry copied from the Apple panes (notes_bubble.go's
    // noteTailDepth/Width/Inset, btMacNoteBubblePath): nine deep, eighteen
    // wide, twenty-four in from the left, pointing DOWN at the passage. Drawn
    // as ONE path so the border never runs across the tail's mouth.
    private static final class NoteBubbleDrawable extends android.graphics.drawable.Drawable {
        private final int fill, stroke, tailDepth, tailWidth, tailInset, radius;
        NoteBubbleDrawable(int fill, int stroke, int tailDepth, int tailWidth, int tailInset, int radius) {
            this.fill = fill; this.stroke = stroke;
            this.tailDepth = tailDepth; this.tailWidth = tailWidth;
            this.tailInset = tailInset; this.radius = radius;
        }
        private android.graphics.Path shape() {
            android.graphics.Rect b = getBounds();
            float l = b.left + 0.5f, t = b.top + 0.5f, r = b.right - 0.5f;
            float bot = b.bottom - tailDepth - 0.5f;
            float x0 = l + tailInset, x1 = x0 + tailWidth, apex = x0 + tailWidth / 2f;
            android.graphics.Path p = new android.graphics.Path();
            p.moveTo(l + radius, t);
            p.lineTo(r - radius, t);
            p.quadTo(r, t, r, t + radius);
            p.lineTo(r, bot - radius);
            p.quadTo(r, bot, r - radius, bot);
            p.lineTo(x1, bot);
            p.lineTo(apex, bot + tailDepth);   // the tail, pointing at the verse
            p.lineTo(x0, bot);
            p.lineTo(l + radius, bot);
            p.quadTo(l, bot, l, bot - radius);
            p.lineTo(l, t + radius);
            p.quadTo(l, t, l + radius, t);
            p.close();
            return p;
        }
        @Override public void draw(android.graphics.Canvas c) {
            android.graphics.Path p = shape();
            android.graphics.Paint paint = new android.graphics.Paint(android.graphics.Paint.ANTI_ALIAS_FLAG);
            paint.setStyle(android.graphics.Paint.Style.FILL);
            paint.setColor(fill);
            c.drawPath(p, paint);
            paint.setStyle(android.graphics.Paint.Style.STROKE);
            paint.setStrokeWidth(Math.max(1f, dp(1)));
            paint.setColor(stroke);
            c.drawPath(p, paint);
        }
        @Override public void setAlpha(int a) {}
        @Override public void setColorFilter(android.graphics.ColorFilter f) {}
        @Override public int getOpacity() { return android.graphics.PixelFormat.TRANSLUCENT; }
    }


    /**
     * The who line with the counts span in the accent colour and everything
     * else muted — the app's chrome, painted the way the other three paint it.
     *
     * The span is FOUND, not cut: Go composed the line and said which substring
     * of it is the control. lastIndexOf, because fitWho may have ellipsised the
     * sender half and the span sits at the end. Not found (a fit so tight the
     * counts gave way) leaves the line plain, with nothing accented promising a
     * tap — the same fallback as the Apple panes.
     */
    private static CharSequence noteWhoSpanned(String line) {
        if (line == null) return "";
        if (!noteNextable || noteCounts == null || noteCounts.isEmpty()) return line;
        int i = line.lastIndexOf(noteCounts);
        if (i < 0) return line;
        android.text.SpannableString sp = new android.text.SpannableString(line);
        sp.setSpan(new android.text.style.ForegroundColorSpan(noteAccent),
                i, i + noteCounts.length(), android.text.Spanned.SPAN_EXCLUSIVE_EXCLUSIVE);
        return sp;
    }

    /**
     * fitWho is btIOSFitWho in Java: when the who line will not fit, the SENDER
     * half tail-truncates and the counts survive whole. Split at the first
     * " \u00b7 " — the same idiom iOS uses, and safe against a sender's name
     * because sanitizeSenderName maps the middle dot away (notes_byline.go).
     */
    private static String fitWho(TextView v, String who, int widthPx) {
        if (who == null || widthPx <= 0) return who;
        android.text.TextPaint tp = v.getPaint();
        if (tp.measureText(who) <= widthPx) return who;
        int sep = who.indexOf(" \u00b7 ");
        if (sep < 0) {
            return android.text.TextUtils.ellipsize(who, tp, widthPx,
                    android.text.TextUtils.TruncateAt.END).toString();
        }
        String sender = who.substring(0, sep), counts = who.substring(sep);
        float countsW = tp.measureText(counts);
        if (countsW >= widthPx) {
            // Even the counts alone overflow: keep their head rather than the
            // byline's, so "1 of 3" survives as far as it can.
            return android.text.TextUtils.ellipsize(counts, tp, widthPx,
                    android.text.TextUtils.TruncateAt.END).toString();
        }
        CharSequence head = android.text.TextUtils.ellipsize(sender, tp,
                widthPx - countsW, android.text.TextUtils.TruncateAt.END);
        return head + counts;
    }

    private static View buildNoteBubble() {
        // THE CARD IS THE SPEC'S CARD: pad / who row / who gap / message / pad,
        // with the tail's depth carried in the bottom padding so no child draws
        // into it. It used to be setPadding(12, 6, 4, 10+9) — a different
        // internal rhythm from the other three surfaces, with the right inset
        // patched back for the message alone by a rightMargin, so the byline and
        // its verbs sat 8dp further right than the words below them.
        android.widget.LinearLayout box = new android.widget.LinearLayout(activity);
        box.setOrientation(android.widget.LinearLayout.VERTICAL);
        box.setBackground(new NoteBubbleDrawable(noteBg, noteBorder,
                noteTail ? dp(NOTE_TAIL) : 0, dp(NOTE_TAIL_W), dp(NOTE_TAIL_X), dp(NOTE_RADIUS)));
        // The bottom pad reserves the tail; with no tail there is nothing to reserve.
        box.setPadding(dp(NOTE_PAD), dp(NOTE_PAD), dp(NOTE_PAD),
                dp(NOTE_PAD) + (noteTail ? dp(NOTE_TAIL) : 0));

        TextView who = new TextView(activity);
        // The WHO line is the app's chrome — byline + honest counts, composed
        // in Go — never the sender's words. When the passage holds more than
        // one note the whole line is the next-tap control: accent + chevron
        // say "press me", the same colour language the iOS counts span uses
        // (iOS accents only the counts; one TextView cannot split the tap, so
        // the line is the control — recorded simplification).
        // The chevron is already in this string: the Go side composes the whole
        // line (chapterNoteChrome), so no renderer appends its own any more.
        String whoLabel = noteWho != null ? noteWho : "Note from Friend";
        who.setText(noteWhoSpanned(whoLabel));
        who.setTag(whoLabel); // the unfitted string, for the width-aware fit below
        who.setTextSize(android.util.TypedValue.COMPLEX_UNIT_SP, 11f);
        who.setTypeface(android.graphics.Typeface.DEFAULT_BOLD);
        // MUTED, with the counts span alone in the accent — the same painting as
        // the other three. This pane used to colour the ENTIRE line accent when
        // it was nextable, which put the sender's byline in the app's "you can
        // press this" colour.
        who.setTextColor(noteMuted);
        // The spec's who height is a MINIMUM, not a fixed box: dp() scales with
        // display density and sp with the reader's font-size choice, so an
        // 11sp line inside a 14dp box clips from about fontScale 1.08 — and
        // Android's slider reaches 1.3, accessibility 2.0.
        // At the default scale the minimum IS the spec, so the geometry the
        // other three platforms hold to is unchanged.
        who.setMinHeight(dp(NOTE_WHO_H));
        who.setGravity(Gravity.CENTER_VERTICAL);
        who.setSingleLine(true);
        // NO ELLIPSIS. The who line is "<byline> · K of N in this chapter ›" and
        // the counts are at the END, so END-truncation drops exactly the half a
        // reader must never lose — and would leave the next-tap overlay
        // invisible but still tappable. iOS forbids this explicitly
        // (btIOSFitWho, reading_ios.go: "a reader must never lose '· 2 of 105 on
        // this passage' to an ellipsis while the constant byline survives"), and
        // fitWho below is that rule in Java: the SENDER half gives way, the
        // counts survive whole.
        // A FIXED box, spec height, with the verbs' width kept clear on the
        // right — not a flow row whose height a glyph decides.
        android.widget.LinearLayout.LayoutParams wlp = new android.widget.LinearLayout.LayoutParams(
                android.widget.LinearLayout.LayoutParams.MATCH_PARENT,
                android.widget.LinearLayout.LayoutParams.WRAP_CONTENT);
        // ONE slot for an own note, two for everyone else's — the count comes
        // from the verb set, not from a constant. This row reserved two either
        // way, so an own note's who line gave way a whole button sooner here
        // than on the Apple panes, which have always asked (own ? 1 : 2).
        wlp.rightMargin = noteVerbSlots() * dp(NOTE_BTN);
        box.addView(who, wlp);
        // Fit at LAYOUT time, where the row's real width is known — the same
        // moment iOS runs btIOSFitWho (btIOSLayoutNote). The unfitted string
        // lives on the tag, so every re-layout re-fits from the original rather
        // than from an already-truncated one; fit(fit(s)) == fit(s), and setText
        // only fires on a real change, so this settles in one extra pass.
        who.addOnLayoutChangeListener(new View.OnLayoutChangeListener() {
            @Override public void onLayoutChange(View v, int l, int t, int r, int b,
                    int ol, int ot, int oR, int ob) {
                if (!(v instanceof TextView) || !(v.getTag() instanceof String)) return;
                TextView tv = (TextView) v;
                int avail = (r - l) - tv.getPaddingLeft() - tv.getPaddingRight();
                if (avail <= 0) return;
                String fitted = fitWho(tv, (String) v.getTag(), avail);
                // Re-span after the fit: setText keeps only the spans it is
                // given, and the fitted string is a NEW one — a plain setText
                // here would silently drop the accent the moment the who line
                // was too wide to fit, which is exactly when it matters.
                if (!fitted.contentEquals(tv.getText())) tv.setText(noteWhoSpanned(fitted));
            }
        });

        TextView body = new TextView(activity);
        body.setText(noteText); // TEXT — nothing here parses markup
        body.setTextSize(android.util.TypedValue.COMPLEX_UNIT_SP, 15f);
        body.setTextColor(noteFg);
        android.widget.LinearLayout.LayoutParams blp = new android.widget.LinearLayout.LayoutParams(
                android.widget.LinearLayout.LayoutParams.MATCH_PARENT,
                android.widget.LinearLayout.LayoutParams.WRAP_CONTENT);
        blp.topMargin = dp(NOTE_WHO_GAP);
        box.addView(body, blp);

        // THE VERBS FLOAT OVER THE CARD, out of its vertical flow, at its top
        // right — the iOS/macOS/styled placement exactly (2 in from the top and
        // right edges). In flow they set the who row's height from an 18sp glyph
        // plus padding, which is the whole reason this card's rhythm never
        // matched anyone else's.
        FrameLayout wrap = new FrameLayout(activity);
        wrap.addView(box, new FrameLayout.LayoutParams(
                FrameLayout.LayoutParams.MATCH_PARENT,
                FrameLayout.LayoutParams.WRAP_CONTENT));
        // The next-tap control is a TRANSPARENT overlay over the who line, not
        // the who TextView itself. The who row is a fixed 14dp spec box now, and
        // a 14dp tap target is not one; iOS has always used a separate button of
        // exactly this height (kNotePad + kNoteWho + 2) over the same span, so
        // the target is the card's whole top strip rather than the glyphs.
        // Added BEFORE the verbs, so the verbs stay on top of it.
        if (noteNextable) {
            View nxt = new View(activity);
            nxt.setClickable(true);
            nxt.setContentDescription("Next note");
            nxt.setOnClickListener(new View.OnClickListener() {
                @Override public void onClick(View v) { nativeNoteNextTapped(); }
            });
            FrameLayout.LayoutParams np = new FrameLayout.LayoutParams(
                    FrameLayout.LayoutParams.MATCH_PARENT,
                    dp(NOTE_PAD + NOTE_WHO_H + 2), Gravity.TOP | Gravity.START);
            np.rightMargin = noteVerbSlots() * dp(NOTE_BTN);
            wrap.addView(nxt, np);
        }
        // Minimize first, delete second: the destructive one is never what a
        // thumb reaches by accident (the iOS ordering).
        //
        // NOT ON YOUR OWN NOTE. There − and ✕ do the same thing — both end in
        // focus-to-none, mark cleared, re-project (hideCurrentNote /
        // dropCurrentNote) — and − promises a pill an own note can never have,
        // since it enters the plan only while focus names it and is built Open.
        // Slot 1 is the rightmost, so omitting slot 2 leaves ✕ where it was.
        if (noteVerbs != VERBS_OWN) {
            wrap.addView(noteVerb("–", 18f, new View.OnClickListener() { // en dash: Hide
                @Override public void onClick(View v) { nativeNoteHidden(); }
            }), noteVerbParams(2));
        }
        // THE MARK SAYS WHAT THE PRESS DOES: a bin where it deletes someone
        // else's message, ✕ where it only puts your own note away. The bin is
        // DRAWN (noteTrashDrawable), like the Apple panes' and the history
        // bar's; ✕ stays a glyph, as it is on every surface.
        View del;
        if (noteVerbs == VERBS_OWN) {
            del = noteVerb("✕", 14f, new View.OnClickListener() {
                @Override public void onClick(View v) { nativeNoteDeleted(); }
            });
        } else {
            android.widget.ImageView bin = new android.widget.ImageView(activity);
            bin.setImageDrawable(noteTrashDrawable(dp(NOTE_TRASH), noteMuted));
            bin.setScaleType(android.widget.ImageView.ScaleType.CENTER);
            bin.setClickable(true);
            bin.setContentDescription("Delete note");
            bin.setOnClickListener(new View.OnClickListener() {
                @Override public void onClick(View v) { nativeNoteDeleted(); }
            });
            del = bin;
        }
        wrap.addView(del, noteVerbParams(1));
        noteView = wrap;
        return wrap;
    }

    // How many verb slots the card reserves. An own note carries ✕ alone, so it
    // keeps one; every other card carries minimize beside delete and keeps two.
    // One function, because the who row, the next-tap overlay and the verbs
    // themselves must never disagree about how wide the verb corner is.
    private static int noteVerbSlots() {
        return noteVerbs == VERBS_OWN ? 1 : 2;
    }

    // noteVerbParams places one floated verb from the card's top-right corner:
    // slot 1 is the rightmost. dp(2) in from both edges, as on the Apple panes.
    private static FrameLayout.LayoutParams noteVerbParams(int slotFromRight) {
        FrameLayout.LayoutParams p = new FrameLayout.LayoutParams(
                dp(NOTE_BTN), dp(NOTE_BTN), Gravity.TOP | Gravity.END);
        p.topMargin = dp(2);
        p.rightMargin = dp(2) + (slotFromRight - 1) * dp(NOTE_BTN);
        return p;
    }

    // noteVerb is one small sticker control (Hide "–" / Delete "✕"): muted, in a
    // fixed touch box (its size is the LayoutParams' now, so the glyph is
    // centred rather than padded out to a height of its own).
    /**
     * The SAME bin the other surfaces draw, so the app has one delete mark
     * rather than two designs. Fyne's theme.DeleteIcon() is Material's
     * "delete": a straight-sided can with a flat lid bar and a small trapezoid
     * handle. This is its SVG transcribed onto a 24x24 box and scaled, exactly
     * as btIOSTrashImage does it (reading_ios.go) — the emoji this replaced
     * rendered at the button's font size in the system's own colours, which is
     * a loud mark on a quiet card and a different bin from the history bar's.
     *
     *   body: M6 19c0 1.1.9 2 2 2h8c1.1 0 2-.9 2-2V7H6v12z
     *   lid:  M19 4h-3.5l-1-1h-5l-1 1H5v2h14V4z
     */
    private static android.graphics.drawable.Drawable noteTrashDrawable(int px, int tint) {
        final float u = px / 24f;
        android.graphics.Path path = new android.graphics.Path();
        // The can.
        path.moveTo(6 * u, 19 * u);
        path.cubicTo(6 * u, 20.1f * u, 6.9f * u, 21 * u, 8 * u, 21 * u);
        path.lineTo(16 * u, 21 * u);
        path.cubicTo(17.1f * u, 21 * u, 18 * u, 20.1f * u, 18 * u, 19 * u);
        path.lineTo(18 * u, 7 * u);
        path.lineTo(6 * u, 7 * u);
        path.close();
        // The lid, with its handle.
        path.moveTo(19 * u, 4 * u);
        path.lineTo(15.5f * u, 4 * u);
        path.lineTo(14.5f * u, 3 * u);
        path.lineTo(9.5f * u, 3 * u);
        path.lineTo(8.5f * u, 4 * u);
        path.lineTo(5 * u, 4 * u);
        path.lineTo(5 * u, 6 * u);
        path.lineTo(19 * u, 6 * u);
        path.close();

        android.graphics.Bitmap bmp =
                android.graphics.Bitmap.createBitmap(px, px, android.graphics.Bitmap.Config.ARGB_8888);
        android.graphics.Canvas c = new android.graphics.Canvas(bmp);
        android.graphics.Paint paint = new android.graphics.Paint(android.graphics.Paint.ANTI_ALIAS_FLAG);
        paint.setStyle(android.graphics.Paint.Style.FILL);
        paint.setColor(tint);
        c.drawPath(path, paint);
        return new android.graphics.drawable.BitmapDrawable(activity.getResources(), bmp);
    }

    private static TextView noteVerb(String glyph, float sp, View.OnClickListener tap) {
        TextView b = new TextView(activity);
        b.setText(glyph);
        b.setTextSize(android.util.TypedValue.COMPLEX_UNIT_SP, sp);
        b.setTextColor(noteMuted);
        b.setGravity(Gravity.CENTER);
        b.setClickable(true);
        b.setOnClickListener(tap);
        return b;
    }

    /**
     * buildNotePill is the collapsed marker: quiet, small, obviously a thing
     * to press. It carries the pushed WHO composition ("Notes · 3", or the
     * unplaced-only sentence), so minimizing the open note does not make the
     * rest of the set invisible. The press is the Restore verb; with no note
     * text behind it (unplaced-only) the Go side is a no-op, so the pill is
     * inert exactly when there is nothing to restore (iOS parity).
     */
    private static View buildNotePill() {
        TextView chip = styleNotePillChip(noteWho != null ? noteWho : "Note");
        chip.setOnClickListener(new View.OnClickListener() {
            @Override public void onClick(View v) { nativeNoteRestored(); }
        });
        notePillView = chip;
        return chip;
    }

    // buildNoteBandChip is the pill's shape carrying a pushed spec's label; the
    // press is the KEYED Restore (verb 1), landing the browser on that
    // paragraph's own group rather than whatever the sticker anchors.
    private static TextView buildNoteBandChip(final int key, String label) {
        TextView chip = styleNotePillChip(label != null && !label.isEmpty() ? label : "Notes");
        chip.setOnClickListener(new View.OnClickListener() {
            @Override public void onClick(View v) { nativeNoteAction(1, key); }
        });
        return chip;
    }

    // The ONE pill styling, whoever the label belongs to.
    private static TextView styleNotePillChip(String label) {
        TextView chip = new TextView(activity);
        chip.setText(label);
        chip.setTextSize(android.util.TypedValue.COMPLEX_UNIT_SP, 11f);
        chip.setTypeface(android.graphics.Typeface.DEFAULT_BOLD);
        chip.setTextColor(noteMuted);
        chip.setSingleLine(true);
        chip.setEllipsize(android.text.TextUtils.TruncateAt.END);
        GradientDrawable bg = new GradientDrawable();
        bg.setShape(GradientDrawable.RECTANGLE);
        bg.setCornerRadius(dp(NOTE_PILL_H) / 2f);
        bg.setColor(noteBg);
        bg.setStroke(Math.max(1, dp(1)), noteBorder);
        chip.setBackground(bg);
        // The pill's height is SPEC (noteMetrics.PillH), not whatever the label
        // plus a vertical padding happened to wrap to — it was ~26dp here, 30 on
        // iOS and 24 on macOS, all three of them the platform's VERB BUTTON size
        // leaking into a piece of content.
        chip.setPadding(dp(NOTE_PILL_PAD_X), 0, dp(NOTE_PILL_PAD_X), 0);
        // CENTER, not CENTER_VERTICAL: with a width floor the box is wider than
        // a short label, and iOS centres its title inside the same box (a
        // UIButton whose frame is the pill's bounds). Left-aligned text in a
        // floored pill reads as a mistake.
        chip.setGravity(Gravity.CENTER);
        chip.setMinHeight(dp(NOTE_PILL_H)); // a minimum, not a box — see the who row
        chip.setMinWidth(dp(NOTE_PILL_MIN_W));
        return chip;
    }

    private static void clearNoteBand() {
        noteBandSpan = null;
        noteBandSpans.clear();
        CharSequence cs = text != null ? text.getText() : null;
        if (!(cs instanceof Spannable)) return;
        Spannable sp = (Spannable) cs;
        // BY CLASS, never by field: whatever this pane ever applied, this sweep
        // can find, so no reservation can be orphaned by its handle being
        // overwritten.
        try {
            NoteBandSpan[] all = sp.getSpans(0, sp.length(), NoteBandSpan.class);
            for (NoteBandSpan b : all) sp.removeSpan(b);
        } catch (Throwable ignored) {}
    }

    // applyNoteBand reserves the band above the anchor verse's PARAGRAPH: a
    // one-character span (NoteBandSpan) on the character BEFORE the paragraph,
    // growing that line's DESCENT. Placing the span on the verse's own first
    // character raised that line's top. Both the paragraph location and descent
    // matter: a note must not open a hole inside a paragraph, and Android paints
    // a character's background across the whole line box,
    // so reserving in the anchor's own ascent stretched the verse's highlight up
    // into the gap and slid it under the card. A paragraph that opens the text
    // has no preceding line and keeps the ascent reservation — nothing is above
    // it to wash.
    // applyNoteBand reserves the STICKER's band plus one band per pushed spec,
    // coalesced ONE SPAN PER PARAGRAPH with the heights summed — chooseHeight
    // runs per paragraph over one shared FontMetricsInt, and two live spans on
    // one paragraph walk straight into the traps NoteBandSpan documents. Pills
    // stack ABOVE the sticker's share when a paragraph carries both (the styled
    // pane's ordering), which is what pillPart records for the placement.
    //
    // off < 0 means "no sticker band" (the sticker is absent, stood down, or
    // its verse is not on this string); spec bands still apply.
    private static void applyNoteBand(int off, int band) {
        CharSequence cs = text.getText();
        if (!(cs instanceof Spannable)) return;
        Spannable sp = (Spannable) cs;

        // paraStart -> {sticker share, pill share}
        java.util.TreeMap<Integer, int[]> shares = new java.util.TreeMap<Integer, int[]>();
        if (off >= 0 && off + 1 <= sp.length() && band > 0) {
            int ps = paraStartAt(sp, off);
            shares.put(ps, new int[]{band, 0});
        }
        int pillBand = dp(NOTE_GAP_ABOVE) + dp(NOTE_PILL_H) + dp(NOTE_GAP_BELOW);
        for (int i = 0; i < noteSpecKeys.length && i < noteSpecVerses.length; i++) {
            int v = noteSpecVerses[i];
            // Verse 0 = the chapter top: paragraph 0's start.
            int soff = 0;
            if (v > 0) {
                int[] r = verseRange(v);
                if (r == null) continue; // not on this string; the browser's business
                soff = r[0];
            }
            if (soff < 0 || soff + 1 > sp.length()) continue;
            int ps = paraStartAt(sp, soff);
            int[] cur = shares.get(ps);
            if (cur == null) { cur = new int[]{0, 0}; shares.put(ps, cur); }
            cur[1] += pillBand;
        }

        for (java.util.Map.Entry<Integer, int[]> e : shares.entrySet()) {
            int ps = e.getKey();
            int stickerShare = e.getValue()[0], pillShare = e.getValue()[1];
            // Attach to the character BEFORE the paragraph and grow THAT line's
            // descent: the reserved gap then belongs to a line the paragraph's
            // own wash does not cover. A paragraph opening the chapter has no
            // preceding line and keeps the ascent reservation.
            boolean below = ps > 0;
            int at = below ? ps - 1 : ps;
            NoteBandSpan span = new NoteBandSpan(stickerShare + pillShare, at, below, ps, pillShare);
            if (stickerShare > 0) noteBandSpan = span; // the sticker's placement handle
            noteBandSpans.add(span);
            sp.setSpan(span, at, at + 1, Spannable.SPAN_EXCLUSIVE_EXCLUSIVE);
        }
        // UpdateLayout makes DynamicLayout reflow; the explicit pair is
        // belt-and-braces for the TextView's own wrap_content height.
        text.requestLayout();
        text.invalidate();
    }

    // THE PARAGRAPH RULE, in one place. The band opens above the whole
    // paragraph carrying the verse, never between two of its lines —
    // Html.fromHtml separates paragraphs with newlines, so the paragraph starts
    // after the last '\n' at or before the offset.
    private static int paraStartAt(CharSequence sp, int off) {
        int ps = off;
        while (ps > 0 && sp.charAt(ps - 1) != '\n') ps--;
        return ps;
    }

    // The last style push's padding and reporter measure, in dp. Kept because
    // the measure is centred against a LIVE view width, so the answer changes
    // without a new push (a rotation, a split-window resize) and the width
    // listener re-asks with these.
    private static int lastPadLDp = 10, lastPadTDp = 14, lastPadRDp = 10, lastPadBDp = 14;
    private static float lastMeasureDp = 0f;

    /**
     * applyReadingPadding centres the reporter column, the way iOS centres it
     * in textContainerInset (btIOSApplyInsets): Go pushes the WIDTH the column
     * should occupy and this side owns both the density and the live view
     * width, so a width change re-centres without re-importing the chapter.
     *
     * measureDp <= 0 is the phone page — the pushed padding stands. A measure
     * wider than the view (a large text size on a narrow window) would compute
     * a negative inset, so the pushed padding is also the floor. UI thread only.
     */
    private static void applyReadingPadding() {
        if (text == null) return;
        float density = activity != null
                ? activity.getResources().getDisplayMetrics().density : 2f;
        int padL = Math.round(lastPadLDp * density), padT = Math.round(lastPadTDp * density);
        int padR = Math.round(lastPadRDp * density), padB = Math.round(lastPadBDp * density);
        if (lastMeasureDp > 0f) {
            int w = text.getWidth();
            if (w > 0) {
                int side = (w - Math.round(lastMeasureDp * density)) / 2;
                if (side > padL) { padL = side; padR = side; }
            }
        }
        text.setPadding(padL, padT, padR, padB);
    }

    /**
     * setStyle pushes the palette + typography (all sizes in PIXELS — the Go
     * side multiplies Fyne units by the canvas scale). Re-sent on every render,
     * so light/dark flips restyle the view.
     */
    public static void setStyle(final int textColor, final int paperColor, final float textSizeDp,
                                final float lineMult, final int padLDp, final int padTDp,
                                final int padRDp, final int padBDp, final float measureDp) {
        UI.post(new Runnable() {
            @Override public void run() {
                if (text == null) return;
                // The pushed sizes are dp; THIS side owns the display density.
                // The Go side used to pre-multiply by the Fyne canvas scale,
                // which is bucketed by dpi, so the same "Normal" drew a
                // different size on every phone (reading_android.go says).
                float density = activity != null
                        ? activity.getResources().getDisplayMetrics().density : 2f;
                float textSizePx = textSizeDp * density;
                lastTextColor = textColor;
                lastPaperColor = paperColor;
                lastPadLDp = padLDp; lastPadTDp = padTDp;
                lastPadRDp = padRDp; lastPadBDp = padBDp;
                lastMeasureDp = measureDp;
                text.setTextColor(textColor);
                text.setBackgroundColor(paperColor);
                scroll.setBackgroundColor(paperColor);
                text.setTextSize(android.util.TypedValue.COMPLEX_UNIT_PX, textSizePx);
                // EXACT line height, not a multiplier, so this matches the Apple
                // pane by construction. iOS sets CSS line-height, which is a
                // multiple of the FONT SIZE; setLineSpacing's multiplier applies
                // to the font's NATURAL line height, which already carries the
                // ascent, descent and leading (~1.25-1.4x the em for this
                // serif). Passing the same number to both therefore made Android
                // noticeably looser than iOS.
                //
                // setLineHeight takes the pitch in pixels and is exactly CSS's
                // line-height. It needs API 28; below that fall back to the
                // multiplier, deriving it from the font's own metrics rather
                // than guessing, so the two paths agree as closely as the older
                // API allows.
                if (android.os.Build.VERSION.SDK_INT >= 28) {
                    text.setLineHeight(Math.round(lineMult * textSizePx));
                } else {
                    android.graphics.Paint.FontMetrics fm = text.getPaint().getFontMetrics();
                    float natural = fm.descent - fm.ascent + fm.leading;
                    float mult = natural > 0f ? (lineMult * textSizePx) / natural : lineMult;
                    text.setLineSpacing(0f, mult);
                }
                text.setTypeface(android.graphics.Typeface.SERIF);
                applyReadingPadding();
                // Say the break strategy and hyphenation out loud rather than
                // inheriting a release's default: Android 13 changed the
                // defaults.
                if (android.os.Build.VERSION.SDK_INT >= 23) {
                    text.setBreakStrategy(android.text.Layout.BREAK_STRATEGY_HIGH_QUALITY);
                    text.setHyphenationFrequency(android.text.Layout.HYPHENATION_FREQUENCY_NORMAL);
                }
                text.setHighlightColor((textColor & 0x00FFFFFF) | 0x33000000);
                // Justify only on Android 15+. A SELECTABLE TextView lays its
                // text out with a DynamicLayout, and before Android 15 that
                // layout never handed the justification mode to the Layout
                // that DRAWS: the line breaker still broke lines in justified
                // mode — which lets a line run past the width by the amount
                // its spaces could shrink — but nothing shrank them, so on 13
                // and 14 every such line spilled past the right edge and the
                // rest stayed ragged (stock API 33 emulator, measured: layout
                // width == view width, line width > both). Android 15's
                // DynamicLayout passes the mode through, and justified text
                // draws exactly as it breaks. Unjustified breaks never
                // overflow, so older releases read ragged but whole.
                if (android.os.Build.VERSION.SDK_INT >= 26) {
                    text.setJustificationMode(android.os.Build.VERSION.SDK_INT >= 35
                            ? android.text.Layout.JUSTIFICATION_MODE_INTER_WORD
                            : android.text.Layout.JUSTIFICATION_MODE_NONE);
                }
            }
        });
    }

    /** setHtml atomically replaces a chapter and its initial placement: an
     *  arrival verse outranks frac, frac>=0 restores, otherwise the chapter
     *  starts at the top. */
    public static void setHtml(final String html, final float frac, final int arrivalVerse) {
        UI.post(new Runnable() {
            @Override public void run() {
                if (text == null) return;
                // A chapter import carries its own top/restore/arrival placement
                // and supersedes any unfinished placement for the prior width.
                pendingReflowFrac = -1f;
                // THE PARAGRAPH GAP IS THE IMPORTER'S. LEGACY mode separates two
                // blocks with a blank line, COMPACT with a single newline — so
                // the reporter page (a first-line indent and NO gap, the octavo
                // page's grammar) is imported COMPACT, and the phone page keeps
                // the blank line it has always had. The two are pushed together
                // and setStyle runs first, so lastMeasureDp is this chapter's
                // answer: a centred column and a gapless page are the same page.
                //
                // Downstream this only tightens the separator: paraStartAt scans
                // back to the last '\n' either way, and the verse index is built
                // from <sup> spans. The one thing it costs is the note pill's
                // stack-centring refinement, which centres IN the blank line and
                // stands down without one (btPillStackInkTop) — the pill still
                // places, one line higher.
                CharSequence s;
                if (android.os.Build.VERSION.SDK_INT >= 24) {
                    s = Html.fromHtml(html, lastMeasureDp > 0f
                            ? Html.FROM_HTML_MODE_COMPACT : Html.FROM_HTML_MODE_LEGACY);
                } else {
                    // The two-argument fromHtml is API 24. Below that the import
                    // is always LEGACY, so the reporter page would arrive with
                    // the indent AND the blank line it replaces — both paragraph
                    // markers at once, which is the one thing the grammar
                    // forbids. Collapse the blank lines instead; a
                    // SpannableStringBuilder moves its spans with the deletion.
                    s = Html.fromHtml(html);
                    if (lastMeasureDp > 0f && s instanceof android.text.SpannableStringBuilder) {
                        android.text.SpannableStringBuilder ssb = (android.text.SpannableStringBuilder) s;
                        for (int i = ssb.length() - 1; i > 0; i--) {
                            if (ssb.charAt(i) == '\n' && ssb.charAt(i - 1) == '\n') ssb.delete(i, i + 1);
                        }
                    }
                }
                // The wash moves off the importer's spans BEFORE the text is
                // set, so the view receives its final spans in one assignment
                // and no span mutation lands on a layout the arrival placement
                // is about to measure.
                if (s instanceof Spannable) liftWashToLineBackground((Spannable) s);
                text.setText(s, TextView.BufferType.SPANNABLE);
                // The prior chapter's highlight span belonged to the old text; drop
                // the reference (the span itself went with the replaced Spanned) and
                // re-index the verses for read-along on the fresh content.
                raSpan = null;
                raVerse = 0;
                buildVerseIndex(text.getText());
                // The note band went with the old Spanned too; re-derive the
                // sticker against the fresh text + verse index (the pushed
                // tuple for this chapter arrived just before this setHtml).
                noteBandSpan = null;
                refreshNoteSticker();
                pendingFrac = frac >= 0 ? frac : -1f;
                if (arrivalVerse > 0) UI.removeCallbacks(scrollIdle);
                if (SCROLL_DEBUG) android.util.Log.i("BtScroll", "setHtml: frac=" + frac
                        + " arrivalVerse=" + arrivalVerse + " replacing pendingVerse=" + pendingVerse);
                // Content and placement are one UI message. A non-arrival import
                // explicitly clears an obsolete target; an arrival cannot be
                // erased by a layout between separate setHtml/scroll messages.
                pendingVerse = arrivalVerse > 0 ? arrivalVerse : 0;
                pendingPlace = true; // this chapter is fresh: place it once
                applyPendingScroll();
            }
        });
    }

    // The one-shot arrival verse. Outranks pendingFrac in applyPendingScroll:
    // when both exist the reader ARRIVED somewhere specific; frac is the
    // "coming back" scroll. Consumed after successful placement, or disarmed
    // once a fully indexed chapter proves the verse is absent.
    private static int pendingVerse = 0;

    // pendingPlace is the FRAC-OR-TOP placement, and it is one-shot. Only the
    // apply that follows a setHtml may use it. A successful verse arrival
    // consumes it so a queued fallback cannot move the reader back to the top.
    private static boolean pendingPlace;

    /** applyPendingScroll positions the text AFTER the layout that follows a
     *  setText: verse first, else frac restore, else top. All
     *  callers run on the UI thread. An armed verse remains pending until both
     *  the TextView layout and verse index exist; the TextView layout listener
     *  re-enters this method after a cold Dialog's first real measure. */
    private static void applyPendingScroll() {
        if (scroll == null) return;
        scroll.post(new Runnable() {
            @Override public void run() {
                if (text == null || scroll == null) return;
                if (pendingVerse > 0) {
                    Layout layout = text.getLayout();
                    int[] r = (layout != null) ? verseRange(pendingVerse) : null;
                    if (SCROLL_DEBUG) android.util.Log.i("BtScroll", "apply: v" + pendingVerse
                            + " layout=" + (layout != null) + " range=" + (r != null)
                            + " attached=" + scroll.isAttachedToWindow()
                            + " indexed=" + verseNums.length + " verses");
                    // Geometry is not ready. Keep the arrival armed; the first
                    // real TextView layout above will try again. Do not consume
                    // pendingPlace here, because top/restore must not outrank an
                    // explicit verse arrival merely due to startup timing.
                    if (layout == null || verseNums.length == 0) {
                        return;
                    }
                    if (r != null) {
                        int line = layout.getLineForOffset(r[0]);
                        int top = layout.getLineTop(line);
                        // ARRIVING MUST SHOW THE NOTE. The band sits above this
                        // verse's paragraph, so scrolling to the verse's own
                        // line puts the bubble explaining it above the fold —
                        // clipped, on the one arrival where it matters most
                        // (a shared link's whole point).
                        //
                        // WHICH of those applies is decided in Go and pushed
                        // (notes_arrival.go). It used to be decided here, by
                        // comparing VERSES, which answered the question this
                        // comment describes only when the link happened to
                        // point at the note's own verse.
                        //
                        // A band that is not reserved yet (noteBandSpan null —
                        // refreshNoteSticker defers its measure below 60dp, and
                        // that runnable can land after this one) falls through
                        // to the verse's own line, never to nothing: silence
                        // here is indistinguishable from "already there" and
                        // leaves the reader wherever they were.
                        if (noteArrival == ARRIVE_BAND && noteBandSpan != null) {
                            int paraOff = r[0];
                            CharSequence cs = text.getText();
                            while (paraOff > 0 && cs.charAt(paraOff - 1) != '\n') paraOff--;
                            int paraLine = layout.getLineForOffset(paraOff);
                            top = layout.getLineTop(paraLine) - noteBandSpan.band;
                            // The drawn stack takes the centering lift and can
                            // sit ABOVE the band top; a target that ignored it
                            // ate the lead — nearly all of it at the largest
                            // text size — and the reader arrived with the pill
                            // against the screen edge. Land on whichever is
                            // higher: the band top or the lifted stack's top.
                            android.graphics.Rect ab = new android.graphics.Rect();
                            text.getPaint().getTextBounds("N", 0, 1, ab);
                            int ink = btPillStackInkTop(layout, noteBandSpan, paraOff,
                                    dp(NOTE_PILL_H), ab.height());
                            if (ink >= 0 && ink < top) top = ink;
                        }
                        int y = text.getTotalPaddingTop() + top - dp(NOTE_LEAD);
                        ownScrollTo(Math.max(y, 0));
                        // Consumed — BOTH of them. Clearing pendingVerse is what
                        // prevents later layout callbacks from applying it again.
                        // Clearing pendingPlace prevents the fresh chapter's
                        // top/restore fallback from undoing this placement.
                        pendingVerse = 0;
                        pendingPlace = false;
                        return;
                    }
                    // The text is fully indexed and this verse genuinely is not
                    // present. Disarm it and use the fresh chapter's fallback.
                    pendingVerse = 0;
                }
                if (!pendingPlace) {
                    // Nothing armed and no fresh chapter to place: leave the
                    // reader exactly where they are.
                    if (SCROLL_DEBUG) android.util.Log.i("BtScroll", "apply: nothing to do"
                            + " (pendingVerse=" + pendingVerse + ") — leaving the scroll alone");
                    return;
                }
                pendingPlace = false;
                if (pendingFrac >= 0) {
                    if (SCROLL_DEBUG) android.util.Log.i("BtScroll", "apply: placing at frac "
                            + pendingFrac);
                    ownScrollTo(Math.round(pendingFrac * scrollRange()));
                } else {
                    if (SCROLL_DEBUG) android.util.Log.i("BtScroll", "apply: placing at TOP");
                    ownScrollTo(0);
                }
            }
        });
    }

    /** armRestore arms/updates the one-shot scroll target without touching the text. */
    public static void armRestore(final float frac) {
        UI.post(new Runnable() {
            @Override public void run() {
                pendingFrac = frac;
                if (text == null || frac < 0) return;
                ownScrollTo(Math.round(frac * scrollRange()));
            }
        });
    }

    /** getScrollFrac is read by the Go side when persisting the reading position. */
    public static float getScrollFrac() { return lastFrac; }

    /** setFrame stores the RAW Fyne pixel frame; applyFrame adds the screen origin. */
    public static void setFrame(final int x, final int y, final int w, final int h) {
        UI.post(new Runnable() {
            @Override public void run() {
                if (dialog == null) return;
                if (frameW > 1 && w > 1 && frameW != w && pendingReflowFrac < 0
                        && pendingVerse <= 0 && !pendingPlace && scroll != null) {
                    int range = scrollRange();
                    pendingReflowFrac = range > 0
                            ? clamp01((float) scroll.getScrollY() / range) : lastFrac;
                }
                frameX = x;   // RAW Fyne pixel coords (relative to the activity window).
                frameY = y;   // The decor's on-screen origin is added in applyFrame.
                frameW = w;
                frameH = h;
                if (dialog.isShowing()) {
                    applyFrame();
                } else if (wantShown) {
                    applyShow(); // a show() arrived before the first real frame
                }
            }
        });
    }

    private static void applyPendingReflow() {
        if (scroll == null || pendingReflowFrac < 0 || pendingVerse > 0 || pendingPlace) return;
        final float frac = pendingReflowFrac;
        scroll.post(new Runnable() {
            @Override public void run() {
                if (scroll == null || pendingReflowFrac != frac
                        || pendingVerse > 0 || pendingPlace) return;
                pendingReflowFrac = -1f;
                ownScrollTo(Math.round(frac * scrollRange()));
            }
        });
    }

    private static void applyFrame() {
        if (dialog == null || !dialog.isShowing()) return;
        try {
            int[] o = windowContentOrigin();
            Window w = dialog.getWindow();
            WindowManager.LayoutParams lp = w.getAttributes();
            lp.x = frameX + o[0];
            lp.y = frameY + o[1];
            lp.width = frameW;
            lp.height = frameH;
            w.setAttributes(lp);
        } catch (Throwable ignored) {
            // Window torn down mid-update (activity recreation) — harmless.
        }
    }

    // windowContentOrigin returns the ON-SCREEN origin of the Fyne GL canvas, which
    // this screen-absolute overlay (FLAG_LAYOUT_IN_SCREEN) must add to the raw Fyne
    // frame coords. Fyne insets its canvas below the status bar + display cutout
    // (e.g. y=136). Computed as decorTop + systemWindowInsetTop so it's correct in
    // BOTH window modes seen on Android 15 / target 35:
    //   - inset window (decor at [0,136]): 136 + 0 = 136
    //   - edge-to-edge window (decor at [0,0], enforced by target 35): 0 + 136 = 136
    // Resolved at apply time (not cached in setFrame) + re-asserted from applyShow,
    // because on a COLD START both terms read 0 until the window settles — caching
    // then would freeze the overlay 136px too high (over the header / audio transport).
    private static int[] windowContentOrigin() {
        int[] loc = new int[2];
        int addTop = 0, addLeft = 0;
        try {
            View decor = activity.getWindow().getDecorView();
            decor.getLocationOnScreen(loc);
            android.view.WindowInsets wi = decor.getRootWindowInsets();
            if (wi != null) {
                addTop = wi.getSystemWindowInsetTop();
                addLeft = wi.getSystemWindowInsetLeft();
            }
        } catch (Throwable ignored) {}
        return new int[]{ loc[0] + addLeft, loc[1] + addTop };
    }

    // Re-assert the frame after the decor settles (see applyFrame's cold-start note).
    private static final Runnable reassertFrame = new Runnable() {
        @Override public void run() { applyFrame(); }
    };

    public static void show() {
        UI.post(new Runnable() {
            @Override public void run() {
                wantShown = true;
                applyShow();
            }
        });
    }

    private static void applyShow() {
        if (dialog == null || suppressed || !wantShown) return;
        if (frameW <= 1 || frameH <= 1) return; // no real frame yet — setFrame retries
        if (activity == null || activity.isFinishing()) return;
        try {
            if (!dialog.isShowing()) {
                dialog.show();
            }
            applyFrame();
            text.requestFocus(); // selection needs the view to hold focus in its window
            applyFollowVisibility(); // re-assert the pill after the overlay comes up
            // Cold start: the decor may not be at its final on-screen origin yet, so
            // the applyFrame above can resolve a too-high position; re-assert once the
            // window has settled so a later apply captures the real origin.
            UI.removeCallbacks(reassertFrame);
            UI.postDelayed(reassertFrame, 250);
            UI.postDelayed(reassertFrame, 700);
        } catch (Throwable t) {
            // Stale window token during an activity teardown — init() rebuilds
            // the Dialog against the next activity, and Go re-drives visibility.
        }
    }

    public static void hide() {
        UI.post(new Runnable() {
            @Override public void run() {
                wantShown = false;
                if (dialog == null || !dialog.isShowing()) return;
                dismissSelection();
                dialog.hide();
            }
        });
    }

    /** suppress hides AND latches (modal up); unsuppress only clears the latch. */
    public static void suppress() {
        UI.post(new Runnable() {
            @Override public void run() {
                suppressed = true;
                if (dialog == null || !dialog.isShowing()) return;
                dismissSelection();
                dialog.hide();
            }
        });
    }

    public static void unsuppress() {
        UI.post(new Runnable() {
            @Override public void run() { suppressed = false; }
        });
    }

    // Clearing the selection takes the floating toolbar down with it — the
    // Android analog of iOS resignFirstResponder before hiding, so the menu
    // never floats orphaned over a Fyne modal.
    private static void dismissSelection() {
        if (text == null) return;
        CharSequence cs = text.getText();
        if (cs instanceof Spannable) Selection.removeSelection((Spannable) cs);
    }

    /** shareText opens the system share sheet for "Share with citation". */
    public static void shareText(final String body) {
        UI.post(new Runnable() {
            @Override public void run() {
                if (activity == null) return;
                Intent i = new Intent(Intent.ACTION_SEND);
                i.setType("text/plain");
                i.putExtra(Intent.EXTRA_TEXT, body);
                activity.startActivity(Intent.createChooser(i, null));
            }
        });
    }

    /**
     * shareImage opens the system share sheet with the rendered verse card (a
     * PNG the Go side wrote to the app cache at `path`). Sharing a file needs a
     * content:// URI; rather than ship a FileProvider (which would need a custom
     * manifest + androidx we don't bundle), we publish the PNG through
     * MediaStore and share that URI. Side effect: the card also lands in the
     * user's Pictures — which is reasonable for an image the user is exporting.
     */
    public static void shareImage(final String path) {
        UI.post(new Runnable() {
            @Override public void run() {
                if (activity == null) return;
                android.net.Uri uri = null;
                try {
                    java.io.File f = new java.io.File(path);
                    android.content.ContentValues cv = new android.content.ContentValues();
                    cv.put(MediaStore.Images.Media.DISPLAY_NAME,
                            f.getName().isEmpty() ? "bibletext-verse.png" : f.getName());
                    cv.put(MediaStore.Images.Media.MIME_TYPE, "image/png");
                    android.content.ContentResolver cr = activity.getContentResolver();
                    uri = cr.insert(MediaStore.Images.Media.EXTERNAL_CONTENT_URI, cv);
                    if (uri == null) return;
                    java.io.InputStream in = new java.io.FileInputStream(f);
                    java.io.OutputStream out = cr.openOutputStream(uri);
                    byte[] buf = new byte[8192];
                    int n;
                    while ((n = in.read(buf)) > 0) out.write(buf, 0, n);
                    in.close();
                    out.close();
                    Intent i = new Intent(Intent.ACTION_SEND);
                    i.setType("image/png");
                    i.putExtra(Intent.EXTRA_STREAM, uri);
                    i.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION);
                    activity.startActivity(Intent.createChooser(i, null));
                } catch (Throwable t) {
                    android.util.Log.w("BtBridge", "shareImage failed", t);
                    if (uri != null) {
                        try { activity.getContentResolver().delete(uri, null, null); } catch (Throwable ignored) {}
                    }
                }
            }
        });
    }
}
