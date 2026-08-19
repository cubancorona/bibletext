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
import android.widget.PopupMenu;
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

    // --- Shared-note sticker (full-screen reading, the implementation requirement) ----------------
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
    private static boolean noteRetryPending;
    private static NoteBandSpan noteBandSpan; // the live band, so a refresh can take it back

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
        // sticker (owner-reported from the screenshot). Reserving in the
        // previous line's descent puts the gap outside every washed character.
        final boolean below;
        // The end offset of the line we adjusted, this layout pass. The next
        // call in the pass is the line that starts exactly there — the one
        // carrying our leftover metrics.
        private int inflatedTo = -1;
        NoteBandSpan(int band, int at, boolean below) {
            this.band = band; this.at = at; this.below = below;
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

    // The verse currently tinted and the span painting it (kept so each tick can
    // clear the previous cheaply — and so we remove OUR span, never the search
    // highlight's BackgroundColorSpan). raActive marks a live read-along;
    // raFollowing mirrors iOS's !gReadAlongUserLatch (auto-scroll armed vs the
    // reader has taken the scroll over).
    private static int raVerse = 0;
    private static BackgroundColorSpan raSpan;
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
    // The note sticker's verbs (the implementation requirement), all fired on the UI thread by the
    // sticker's own controls; each dispatches to the SAME Go verb the iOS
    // sticker calls (reading_android_export.go / reading_jni_android.c).
    private static native void nativeNoteNextTapped();
    private static native void nativeNoteHidden();
    private static native void nativeNoteDeleted();
    private static native void nativeNoteRestored();
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

    /** scrollToVerse pins a verse near the top of the viewport — the ARRIVAL
     *  scroll for a shared link's highlighted passage (the Android twin of the
     *  highlight branch of iOS's bibleTextScrollReadingTV). It arms
     *  pendingVerse and runs the shared post-layout applier, so it composes
     *  with setHtml instead of racing it; a delayed re-assert catches a slow
     *  first layout. */
    public static void scrollToVerse(final int verse) {
        if (verse <= 0) return;
        UI.post(new Runnable() {
            @Override public void run() {
                pendingVerse = verse;
                applyPendingScroll();
            }
        });
        UI.postDelayed(new Runnable() {
            @Override public void run() {
                if (pendingVerse == verse) applyPendingScroll();
            }
        }, 250);
    }

    private BtBridge() {}

    // setAIEnabled mirrors the app's Settings → Assistant choice; called from Go
    // on init and on change (from the Go/JNI thread — the volatile on aiEnabled
    // makes the write visible to the UI thread's onCreateActionMode read).
    public static void setAIEnabled(boolean on) { aiEnabled = on; }

    // showStudyPopup presents Explain / Analyze context / Analyze translation as
    // a popup anchored at the selection — the inline "Study with AI" bar item's
    // second level (a floating toolbar cannot nest a real SubMenu inline). The
    // anchor is a zero-size view placed at the selection end's coordinates in
    // root, removed when the popup dismisses. sel was captured at tap time.
    private static void showStudyPopup(final ActionMode mode, final String sel, int selEnd,
                                       final int selLo, final int selHi) {
        if (activity == null || root == null || text == null) return;
        float ax = 0; int ay = 0;
        Layout lay = text.getLayout();
        if (lay != null && selEnd >= 0) {
            int line = lay.getLineForOffset(selEnd);
            ax = lay.getPrimaryHorizontal(selEnd) + text.getLeft() - scroll.getScrollX();
            ay = lay.getLineBottom(line) + text.getTop() - scroll.getScrollY();
        }
        final View anchor = new View(activity);
        FrameLayout.LayoutParams lp = new FrameLayout.LayoutParams(1, 1);
        lp.leftMargin = Math.max(0, (int) ax);
        lp.topMargin = Math.max(0, ay);
        root.addView(anchor, lp);

        PopupMenu pm = new PopupMenu(activity, anchor);
        pm.getMenu().add(0, 102, 0, "Explain");
        pm.getMenu().add(0, 103, 1, "Analyze context");
        pm.getMenu().add(0, 104, 2, "Analyze translation");
        pm.setOnMenuItemClickListener(new PopupMenu.OnMenuItemClickListener() {
            @Override public boolean onMenuItemClick(MenuItem mi) {
                String action;
                switch (mi.getItemId()) {
                    case 102: action = "explain"; break;
                    case 103: action = "context"; break;
                    case 104: action = "translation"; break;
                    default: return false;
                }
                try { mode.finish(); } catch (Throwable ignored) {}
                nativeSelectionAction(action, sel, selLo, selHi);
                return true;
            }
        });
        pm.setOnDismissListener(new PopupMenu.OnDismissListener() {
            @Override public void onDismiss(PopupMenu m) { root.removeView(anchor); }
        });
        pm.show();
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
        // chapter to its last verse (platform reproduction: the arrival scroll to a
        // shared verse randomly lost to this, and the scroll listener then
        // misread the jump as a reader gesture and PERSISTED the end position).
        // Every scroll we mean happens through scrollTo; the bring-into-view
        // path is never one we asked for, so refuse it wholesale.
        scroll = new ScrollView(activity) {
            @Override
            public boolean requestChildRectangleOnScreen(View child, Rect rectangle, boolean immediate) {
                return false;
            }
        };
        scroll.setFillViewport(true);
        scroll.setVerticalScrollBarEnabled(true);
        // The scroll's one child is a FrameLayout holding the TextView AND the
        // note sticker (the implementation requirement): a sticker inside the scrolled content rides
        // its anchor verse natively, with no per-scroll repositioning. The
        // TextView's own geometry (getLeft/getTop = 0) is unchanged, so the
        // study-popup anchor math and scrollRange() still hold.
        content = new FrameLayout(activity);
        content.addView(text, new FrameLayout.LayoutParams(
                FrameLayout.LayoutParams.MATCH_PARENT, FrameLayout.LayoutParams.WRAP_CONTENT));
        // A real (or changed) WIDTH re-derives the sticker: the bubble wraps
        // its text at the content width, and the first refresh may have run
        // before any layout existed. Width only — the band itself changes the
        // HEIGHT on every refresh, and reacting to that would loop.
        content.addOnLayoutChangeListener(new View.OnLayoutChangeListener() {
            @Override public void onLayoutChange(View v, int l, int t, int r, int b,
                    int ol, int ot, int orr, int ob) {
                if ((r - l) != (orr - ol)) refreshNoteSticker();
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
    // NOT spec: the verb button's size, this platform's own touch target. The
    // verbs used to sit IN the card's vertical flow, so an 18sp glyph plus its
    // padding — not the spec — set the who row's height and pushed the byline
    // about 8dp lower than on every other surface. They float over the card now,
    // exactly as they do on iOS, macOS and the styled pane.
    private static final int NOTE_BTN = 30;

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
            if (num <= 0) continue;
            nums[count] = num;
            starts[count] = st;
            ends[count] = en;
            count++;
        }
        verseNums = Arrays.copyOf(nums, count);
        verseStarts = Arrays.copyOf(starts, count);
        verseEnds = Arrays.copyOf(ends, count);
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

    // ---- Shared-note sticker (full-screen reading, the implementation requirement) ----------------

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
                               final boolean nextable, final int anchorVerse,
                               final int bg, final int fg, final int muted,
                               final int accent, final int border) {
        UI.post(new Runnable() {
            @Override public void run() {
                String t = (noteText_ == null || noteText_.length == 0)
                        ? null : new String(noteText_, java.nio.charset.StandardCharsets.UTF_8);
                String w = (who_ == null || who_.length == 0)
                        ? null : new String(who_, java.nio.charset.StandardCharsets.UTF_8);
                boolean changed = !sameStr(t, noteText) || !sameStr(w, noteWho)
                        || notePill != pill || noteNextable != nextable
                        || noteAnchorVerse != anchorVerse;
                noteText = t;
                noteWho = w;
                notePill = pill;
                noteNextable = nextable;
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
    private static void refreshNoteSticker() {
        if (content == null || text == null) {
            if (NOTE_DEBUG) android.util.Log.i("BtNote", "refresh: no content/text yet");
            return;
        }
        if (noteView != null) { content.removeView(noteView); noteView = null; }
        if (notePillView != null) { content.removeView(notePillView); notePillView = null; }
        clearNoteBand();
        if (!notePresent()) {
            if (NOTE_DEBUG) android.util.Log.i("BtNote", "refresh: notePresent=false — nothing to draw");
            return;
        }
        int side = dp(10);
        int wpx = content.getWidth() - 2 * side;
        if (wpx < dp(60)) {
            // No real layout yet. The width LISTENER only fires when the width
            // CHANGES, so on an arrival into an already-sized pane it never
            // fires again and the sticker is simply never built — the band is
            // reserved and the card is missing, which is exactly what the
            // owner saw ("the highlight… too high": an empty reservation with
            // nothing in it). Ask again after this layout pass instead of
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
        View v = pillNow ? buildNotePill() : buildNoteBubble();
        v.setElevation(6f);
        FrameLayout.LayoutParams lp;
        if (pillNow) {
            // The pill sizes to its label (capped at the content width).
            v.measure(View.MeasureSpec.makeMeasureSpec(wpx, View.MeasureSpec.AT_MOST),
                      View.MeasureSpec.makeMeasureSpec(0, View.MeasureSpec.UNSPECIFIED));
            lp = new FrameLayout.LayoutParams(FrameLayout.LayoutParams.WRAP_CONTENT,
                                              FrameLayout.LayoutParams.WRAP_CONTENT);
        } else {
            // The bubble is a full-width card, wrapped at the content measure.
            v.measure(View.MeasureSpec.makeMeasureSpec(wpx, View.MeasureSpec.EXACTLY),
                      View.MeasureSpec.makeMeasureSpec(0, View.MeasureSpec.UNSPECIFIED));
            lp = new FrameLayout.LayoutParams(wpx, FrameLayout.LayoutParams.WRAP_CONTENT);
        }
        lp.leftMargin = side;
        // BOTH gaps are SPEC (noteMetrics.GapAbove / GapBelow). They were dp(8)
        // here and 10 on the other three, chosen locally and never reconciled.
        final int gapAbove = dp(NOTE_GAP_ABOVE), gapBelow = dp(NOTE_GAP_BELOW);
        final int noteH = v.getMeasuredHeight();

        if (NOTE_DEBUG) android.util.Log.i("BtNote", "refresh: building, wpx=" + wpx + " pill=" + pillNow
                + " anchor=" + noteAnchorVerse + " verseIdx=" + verseNums.length);
        int[] r = noteAnchorVerse > 0 ? verseRange(noteAnchorVerse) : null;
        if (r == null) {
            // Nothing to anchor to (verse 0 = unplaced-only, or a verse this
            // translation's index does not carry): park at the top of the
            // text with no band — the only honest place for notes with no
            // verses here (the iOS top-inset case, minus the reservation).
            lp.topMargin = dp(6);
            content.addView(v, lp);
            return;
        }
        // A gap on BOTH sides of the card — the styled pane's symmetry rule.
        // Reserving only below left the card butting against the line above
        // (0 against gap+tail), which the owner spotted immediately on the
        // other platform.
        applyNoteBand(r[0], gapAbove + noteH + gapBelow);
        // Place the sticker into the reserved gap AFTER the reflow the band
        // just caused: its top is the anchor line's (raised) top.
        final View vv = v;
        final int off = r[0];
        content.addView(vv, lp);
        vv.setVisibility(View.INVISIBLE); // never flash at 0,0 before placement
        text.post(new Runnable() {
            @Override public void run() {
                if (vv.getParent() == null || text == null) return;
                Layout lay = text.getLayout();
                if (lay == null) return; // hidden overlay: the next refresh places it
                // The band belongs to the paragraph, so the sticker hangs from
                // the paragraph's first line — not the anchor verse's line.
                int paraOff = off;
                CharSequence cs3 = text.getText();
                while (paraOff > 0 && cs3.charAt(paraOff - 1) != '\n') paraOff--;
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
                // The card hangs gapAbove below the band's own top edge — the
                // same arithmetic the styled pane's place() and iOS's
                // btIOSLayoutNote use. (It was a bare dp(8) written out again
                // here, so the reservation and the placement each had their own
                // copy of the number.)
                int top = text.getTop() + text.getTotalPaddingTop() + gapTop + gapAbove;
                FrameLayout.LayoutParams p = (FrameLayout.LayoutParams) vv.getLayoutParams();
                p.topMargin = Math.max(0, top);
                vv.setLayoutParams(p);
                vv.setVisibility(View.VISIBLE);
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
                dp(NOTE_TAIL), dp(NOTE_TAIL_W), dp(NOTE_TAIL_X), dp(NOTE_RADIUS)));
        box.setPadding(dp(NOTE_PAD), dp(NOTE_PAD), dp(NOTE_PAD), dp(NOTE_PAD) + dp(NOTE_TAIL));

        TextView who = new TextView(activity);
        // The WHO line is the app's chrome — byline + honest counts, composed
        // in Go — never the sender's words. When the passage holds more than
        // one note the whole line is the next-tap control: accent + chevron
        // say "press me", the same colour language the iOS counts span uses
        // (iOS accents only the counts; one TextView cannot split the tap, so
        // the line is the control — recorded simplification).
        String whoLabel = noteWho != null ? noteWho : "Note from Friend";
        whoLabel = noteNextable ? whoLabel + "  ›" : whoLabel;
        who.setText(whoLabel);
        who.setTag(whoLabel); // the unfitted string, for the width-aware fit below
        who.setTextSize(android.util.TypedValue.COMPLEX_UNIT_SP, 11f);
        who.setTypeface(android.graphics.Typeface.DEFAULT_BOLD);
        who.setTextColor(noteNextable ? noteAccent : noteMuted);
        // The spec's who height is a MINIMUM, not a fixed box: dp() scales with
        // display density and sp with the reader's font-size choice, so an
        // 11sp line inside a 14dp box clips from about fontScale 1.08 — and
        // Android's slider reaches 1.3, accessibility 2.0 (verification finding).
        // At the default scale the minimum IS the spec, so the geometry the
        // other three platforms hold to is unchanged.
        who.setMinHeight(dp(NOTE_WHO_H));
        who.setGravity(Gravity.CENTER_VERTICAL);
        who.setSingleLine(true);
        // NO ELLIPSIS. The who line is "<byline> · K of N on this passage ›" and
        // the counts are at the END, so END-truncation drops exactly the half a
        // reader must never lose — and would leave the next-tap overlay
        // invisible but still tappable. iOS forbids this explicitly
        // (btIOSFitWho, reading_ios.go: "a reader must never lose '· 2 of 105 on
        // this passage' to an ellipsis while the constant byline survives"), and
        // fitWho below is that rule in Java: the SENDER half gives way, the
        // counts survive whole (verification finding).
        // A FIXED box, spec height, with the verbs' width kept clear on the
        // right — not a flow row whose height a glyph decides.
        android.widget.LinearLayout.LayoutParams wlp = new android.widget.LinearLayout.LayoutParams(
                android.widget.LinearLayout.LayoutParams.MATCH_PARENT,
                android.widget.LinearLayout.LayoutParams.WRAP_CONTENT);
        wlp.rightMargin = 2 * dp(NOTE_BTN);
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
                if (!fitted.contentEquals(tv.getText())) tv.setText(fitted);
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
            nxt.setOnClickListener(new View.OnClickListener() {
                @Override public void onClick(View v) { nativeNoteNextTapped(); }
            });
            FrameLayout.LayoutParams np = new FrameLayout.LayoutParams(
                    FrameLayout.LayoutParams.MATCH_PARENT,
                    dp(NOTE_PAD + NOTE_WHO_H + 2), Gravity.TOP | Gravity.START);
            np.rightMargin = 2 * dp(NOTE_BTN);
            wrap.addView(nxt, np);
        }
        // Minimize first, delete second: the destructive one is never what a
        // thumb reaches by accident (the iOS ordering).
        wrap.addView(noteVerb("–", 18f, new View.OnClickListener() { // en dash: Hide
            @Override public void onClick(View v) { nativeNoteHidden(); }
        }), noteVerbParams(2));
        wrap.addView(noteVerb("✕", 14f, new View.OnClickListener() { // multiplication x: Delete
            @Override public void onClick(View v) { nativeNoteDeleted(); }
        }), noteVerbParams(1));
        noteView = wrap;
        return wrap;
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
        TextView chip = new TextView(activity);
        chip.setText(noteWho != null ? noteWho : "Note");
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
        chip.setPadding(dp(NOTE_PAD), 0, dp(NOTE_PAD), 0);
        chip.setGravity(Gravity.CENTER_VERTICAL);
        chip.setMinHeight(dp(NOTE_PILL_H)); // a minimum, not a box — see the who row
        chip.setOnClickListener(new View.OnClickListener() {
            @Override public void onClick(View v) { nativeNoteRestored(); }
        });
        notePillView = chip;
        return chip;
    }

    private static void clearNoteBand() {
        if (noteBandSpan == null) return;
        CharSequence cs = text != null ? text.getText() : null;
        if (cs instanceof Spannable) {
            try { ((Spannable) cs).removeSpan(noteBandSpan); } catch (Throwable ignored) {}
        }
        noteBandSpan = null;
    }

    // applyNoteBand reserves the band above the anchor verse: a one-character
    // span on the verse's first char raising that line's top (NoteBandSpan).
    private static void applyNoteBand(int off, int band) {
        CharSequence cs = text.getText();
        if (!(cs instanceof Spannable)) return;
        Spannable sp = (Spannable) cs;
        if (off < 0 || off + 1 > sp.length()) return;

        // up paragraphs… No breaking up the Word of God"). The band opens
        // above the whole paragraph carrying the verse, never between two of
        // its lines — the rule iOS has always followed and the styled pane now
        // follows too. Html.fromHtml separates paragraphs with newlines, so the
        // paragraph starts after the last '\n' at or before the verse.
        int paraStart = off;
        while (paraStart > 0 && sp.charAt(paraStart - 1) != '\n') paraStart--;
        // Attach to the character BEFORE the paragraph and grow THAT line's
        // descent: the reserved gap then belongs to a line the paragraph's own
        // wash does not cover. A paragraph opening the chapter has no preceding
        // line and keeps the ascent reservation (nothing above it to wash).
        boolean below = paraStart > 0;
        int at = below ? paraStart - 1 : paraStart;
        noteBandSpan = new NoteBandSpan(band, at, below);
        sp.setSpan(noteBandSpan, at, at + 1, Spannable.SPAN_EXCLUSIVE_EXCLUSIVE);
        // UpdateLayout makes DynamicLayout reflow; the explicit pair is
        // belt-and-braces for the TextView's own wrap_content height.
        text.requestLayout();
        text.invalidate();
    }

    /**
     * setStyle pushes the palette + typography (all sizes in PIXELS — the Go
     * side multiplies Fyne units by the canvas scale). Re-sent on every render,
     * so light/dark flips restyle the view.
     */
    public static void setStyle(final int textColor, final int paperColor, final float textSizePx,
                                final float lineMult, final int padL, final int padT,
                                final int padR, final int padB) {
        UI.post(new Runnable() {
            @Override public void run() {
                if (text == null) return;
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
                // noticeably looser than iOS — owner-reported from the emulator.
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
                text.setPadding(padL, padT, padR, padB);
                text.setHighlightColor((textColor & 0x00FFFFFF) | 0x33000000);
                if (android.os.Build.VERSION.SDK_INT >= 26) {
                    text.setJustificationMode(android.text.Layout.JUSTIFICATION_MODE_INTER_WORD);
                }
            }
        });
    }

    /** setHtml replaces the chapter text; frac>=0 arms a one-shot scroll restore, else pins to top. */
    public static void setHtml(final String html, final float frac) {
        UI.post(new Runnable() {
            @Override public void run() {
                if (text == null) return;
                CharSequence s;
                if (android.os.Build.VERSION.SDK_INT >= 24) {
                    s = Html.fromHtml(html, Html.FROM_HTML_MODE_LEGACY);
                } else {
                    s = Html.fromHtml(html);
                }
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
                pendingVerse = 0; // a following scrollToVerse (queued next) re-arms it
                applyPendingScroll();
            }
        });
    }

    // The one-shot arrival verse. Outranks pendingFrac in applyPendingScroll:
    // when both exist the reader ARRIVED somewhere specific; frac is the
    // "coming back" scroll. Cleared by the first user scroll alongside it.
    private static int pendingVerse = 0;

    /** applyPendingScroll positions the text AFTER the layout that follows a
     *  setText/scrollToVerse: verse first, else frac restore, else top. All
     *  callers run on the UI thread, so the posts queue deterministically —
     *  this replaced a free-standing scrollToVerse that raced setHtml's own
     *  post-layout scroll and lost (platform reproduction: arrivals landed wherever
     *  the previous reader stopped). */
    private static void applyPendingScroll() {
        if (scroll == null) return;
        scroll.post(new Runnable() {
            @Override public void run() {
                if (text == null || scroll == null) return;
                if (pendingVerse > 0) {
                    Layout layout = text.getLayout();
                    int[] r = (layout != null) ? verseRange(pendingVerse) : null;
                    if (r != null) {
                        int line = layout.getLineForOffset(r[0]);
                        int top = layout.getLineTop(line);
                        // ARRIVING MUST SHOW THE NOTE. The band sits above this
                        // verse's paragraph, so scrolling to the verse's own
                        // line puts the bubble explaining it above the fold —
                        // clipped, on the one arrival where it matters most
                        // (a shared link's whole point). The styled pane has
                        // the same rule in highlightY; this is its Android
                        // twin: when the band belongs to this verse's
                        // paragraph, scroll to the BAND's top instead.
                        if (noteBandSpan != null && noteAnchorVerse == pendingVerse) {
                            int paraOff = r[0];
                            CharSequence cs = text.getText();
                            while (paraOff > 0 && cs.charAt(paraOff - 1) != '\n') paraOff--;
                            int paraLine = layout.getLineForOffset(paraOff);
                            top = layout.getLineTop(paraLine) - noteBandSpan.band;
                        }
                        int y = text.getTotalPaddingTop() + top - dp(16);
                        ownScrollTo(Math.max(y, 0));
                        return;
                    }
                }
                if (pendingFrac >= 0) {
                    ownScrollTo(Math.round(pendingFrac * scrollRange()));
                } else {
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
