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
    private static TextView text;

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
    // reader; text is the exact selected substring.
    private static native void nativeSelectionAction(String action, String text);
    // Called on the UI thread after the reader's scroll has been idle ~200ms.
    private static native void nativeScrolled(float frac);
    // Called on the UI thread when the reader scrolls by hand during read-along
    // (suspends follow) and when they tap the "Follow narration" pill (resumes it).
    private static native void nativeReadAlongUserScrolled();
    private static native void nativeReadAlongFollowTapped();

    private BtBridge() {}

    // setAIEnabled mirrors the app's Settings → Assistant choice; called from Go
    // on init and on change (from the Go/JNI thread — the volatile on aiEnabled
    // makes the write visible to the UI thread's onCreateActionMode read).
    public static void setAIEnabled(boolean on) { aiEnabled = on; }

    /**
     * init stores the activity and builds the view lazily on the UI thread.
     * Called on every RunNative init AND whenever the activity changes: Android
     * recreates the activity (rotation, background→foreground), and a Dialog
     * cached against the destroyed activity's window token throws
     * BadTokenException on show(). So when a new activity arrives, tear the old
     * Dialog down and rebuild against the live one; the Go afterRebuild re-pushes
     * the frame + visibility, so we don't auto-show here.
     */
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
                text = null;
                followBtn = null;
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
                                        text = null;
                                        followBtn = null;
                                        raSpan = null;
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
                // Mirror the iOS selection menu: a "Study with AI" submenu, a
                // "Share" submenu (with-citation + as-image), and Cross-references.
                // Keep Copy/Select all; drop the system plain-text Share (ours,
                // with the reference, supersedes it) so there aren't two Shares.
                menu.removeItem(android.R.id.shareText);
                // Snapshot the volatile gate once so the menu is structurally
                // consistent even if it flips mid-build (read twice below).
                final boolean aiOn = aiEnabled;
                if (aiOn) {
                    SubMenu ai = menu.addSubMenu(0, 200, 100, "Study with AI");
                    ai.add(0, 102, 1, "Explain");
                    ai.add(0, 103, 2, "Analyze context");
                    ai.add(0, 104, 3, "Analyze translation");
                }
                SubMenu sh = menu.addSubMenu(0, 201, 101, "Share");
                sh.add(0, 106, 0, "Share with citation");
                sh.add(0, 107, 1, "Share as image");
                // Cross-references: below Share (its usual spot) when AI is on; in
                // the AI submenu's place, ahead of Share, when AI is off.
                menu.add(0, 105, aiOn ? 102 : 100, "Cross-references");
                return true;
            }
            @Override public boolean onPrepareActionMode(ActionMode mode, Menu menu) { return false; }
            @Override public boolean onActionItemClicked(ActionMode mode, MenuItem item) {
                String action;
                switch (item.getItemId()) {
                    case 102: action = "explain"; break;
                    case 103: action = "context"; break;
                    case 104: action = "translation"; break;
                    case 105: action = "crossref"; break;
                    case 106: action = "share-cite"; break;
                    case 107: action = "share-image"; break;
                    default: return false; // submenu header (200/201) or system item
                }
                int a = text.getSelectionStart(), b = text.getSelectionEnd();
                if (a < 0 || b < 0) return true;
                String sel = text.getText().subSequence(Math.min(a, b), Math.max(a, b)).toString();
                mode.finish();
                nativeSelectionAction(action, sel);
                return true;
            }
            @Override public void onDestroyActionMode(ActionMode mode) {}
            @Override public void onGetContentRect(ActionMode mode, View view, Rect outRect) {
                super.onGetContentRect(mode, view, outRect);
            }
        });

        scroll = new ScrollView(activity);
        scroll.setFillViewport(true);
        scroll.setVerticalScrollBarEnabled(true);
        scroll.addView(text, new FrameLayout.LayoutParams(
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
                    // A reader gesture obsoletes any pending restore, and the
                    // idle timer persists the new position once still.
                    lastOwnScrollY = -1;
                    pendingFrac = -1f;
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
                text.setLineSpacing(0f, lineMult);
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
                pendingFrac = frac >= 0 ? frac : -1f;
                // Apply top-pin / restore after the text has been laid out.
                scroll.post(new Runnable() {
                    @Override public void run() {
                        ownScrollTo(pendingFrac >= 0 ? Math.round(pendingFrac * scrollRange()) : 0);
                    }
                });
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
