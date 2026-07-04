package org.bibletext;

import android.app.Activity;
import android.app.Dialog;
import android.content.Intent;
import android.graphics.Color;
import android.graphics.drawable.ColorDrawable;
import android.graphics.Rect;
import android.os.Handler;
import android.os.Looper;
import android.text.Html;
import android.text.Selection;
import android.text.Spannable;
import android.view.ActionMode;
import android.view.Gravity;
import android.view.KeyEvent;
import android.view.Menu;
import android.view.MenuItem;
import android.view.View;
import android.view.Window;
import android.view.WindowManager;
import android.widget.FrameLayout;
import android.widget.ScrollView;
import android.widget.TextView;

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
    private static Dialog dialog;
    private static ScrollView scroll;
    private static TextView text;

    // Pending frame (device pixels, Fyne coordinates) and whether the overlay
    // should be visible. The Dialog is only shown once a real frame has
    // arrived; wantShown remembers a show() that landed before that. UI-thread.
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
    // restore from a reader gesture (a reader gesture disarms pendingFrac and
    // schedules a position save).
    private static boolean ownScroll = false;

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

    private BtBridge() {}

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
                scroll = null;
                text = null;
                frameW = 1;
                frameH = 1;
                // wantShown/suppressed are re-asserted by the Go side after the
                // rebuild (notifyReadingOverlay / show/hide), so start neutral.
                wantShown = false;
                suppressed = false;
                activity = act;
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
                // Keep the system items (Copy / Select All / Share…) and add the
                // BibleText study cluster; the floating toolbar shows ours in
                // the overflow. Order values place them after the system's.
                menu.add(0, 101, 100, "Ask AI a question…");
                menu.add(0, 102, 101, "Explain");
                menu.add(0, 103, 102, "Analyze context");
                menu.add(0, 104, 103, "Analyze translation");
                menu.add(0, 105, 104, "Cross-references");
                menu.add(0, 106, 105, "Share with citation");
                return true;
            }
            @Override public boolean onPrepareActionMode(ActionMode mode, Menu menu) { return false; }
            @Override public boolean onActionItemClicked(ActionMode mode, MenuItem item) {
                String action;
                switch (item.getItemId()) {
                    case 101: action = "ask"; break;
                    case 102: action = "explain"; break;
                    case 103: action = "context"; break;
                    case 104: action = "translation"; break;
                    case 105: action = "crossref"; break;
                    case 106: action = "share-cite"; break;
                    default: return false; // system item — let Android handle it
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

        dialog = new Dialog(activity, android.R.style.Theme_Translucent_NoTitleBar);
        dialog.setContentView(scroll);
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
                    lastFrac = clamp01((float) scroll.getScrollY() / range);
                    if (ownScroll) return;
                    // A reader gesture obsoletes any pending restore, and the
                    // idle timer persists the new position once still.
                    pendingFrac = -1f;
                    UI.removeCallbacks(scrollIdle);
                    UI.postDelayed(scrollIdle, 200);
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
                pendingFrac = frac >= 0 ? frac : -1f;
                // Apply top-pin / restore after the text has been laid out.
                scroll.post(new Runnable() {
                    @Override public void run() {
                        ownScroll = true;
                        try {
                            if (pendingFrac >= 0) {
                                scroll.scrollTo(0, Math.round(pendingFrac * scrollRange()));
                            } else {
                                scroll.scrollTo(0, 0);
                            }
                        } finally { ownScroll = false; }
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
                ownScroll = true;
                try { scroll.scrollTo(0, Math.round(frac * scrollRange())); }
                finally { ownScroll = false; }
            }
        });
    }

    /** getScrollFrac is read by the Go side when persisting the reading position. */
    public static float getScrollFrac() { return lastFrac; }

    /** setFrame positions the overlay window (device pixels, window coordinates). */
    public static void setFrame(final int x, final int y, final int w, final int h) {
        UI.post(new Runnable() {
            @Override public void run() {
                if (dialog == null) return;
                // The Fyne canvas is laid out BELOW the system insets (status
                // bar) while this window's coordinates are screen-absolute
                // (FLAG_LAYOUT_IN_SCREEN) — add the insets back, the Android
                // analog of the iOS safe-area shift in bibleTextTVSetFrame.
                int insetTop = 0, insetLeft = 0;
                View decor = activity.getWindow().getDecorView();
                if (decor.getRootWindowInsets() != null) {
                    insetTop = decor.getRootWindowInsets().getSystemWindowInsetTop();
                    insetLeft = decor.getRootWindowInsets().getSystemWindowInsetLeft();
                }
                frameX = x + insetLeft;
                frameY = y + insetTop;
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
            Window w = dialog.getWindow();
            WindowManager.LayoutParams lp = w.getAttributes();
            lp.x = frameX;
            lp.y = frameY;
            lp.width = frameW;
            lp.height = frameH;
            w.setAttributes(lp);
        } catch (Throwable ignored) {
            // Window torn down mid-update (activity recreation) — harmless.
        }
    }

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
}
