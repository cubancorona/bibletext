package org.bibletext;

import android.app.Activity;
import android.app.Dialog;
import android.content.Context;
import android.graphics.Color;
import android.graphics.Rect;
import android.graphics.drawable.ColorDrawable;
import android.graphics.drawable.GradientDrawable;
import android.os.Build;
import android.os.Handler;
import android.os.Looper;
import android.text.Editable;
import android.text.InputType;
import android.text.TextWatcher;
import android.util.TypedValue;
import android.view.Gravity;
import android.view.KeyEvent;
import android.view.View;
import android.view.Window;
import android.view.WindowManager;
import android.view.inputmethod.InputMethodManager;
import android.widget.EditText;
import java.nio.charset.StandardCharsets;

/** The shared Note/Ask editor occupies only its existing Fyne placeholder. */
final class BtNoteEntry {
    private static final Handler UI = new Handler(Looper.getMainLooper());
    private static Activity activity;
    private static Dialog dialog;
    private static EditText editor;
    private static View keyboardHost;
    private static android.view.ViewTreeObserver.OnGlobalLayoutListener keyboardLayout;
    private static long owner;
    private static boolean wanted, hasFrame, focusWanted;
    private static String draft = "", placeholder = "";
    private static int x, y, width, height, textColor, hintColor, background, border;
    private static float textPx, scale = 1, keyboardPx;
    private static int selectionStart, selectionEnd;

    static void init(Activity next) {
        if (activity == next) return;
        detach();
        activity = next;
        hasFrame = false;
    }

    static void destroy(Activity gone) {
        if (activity != gone) return;
        detach();
        activity = null;
        hasFrame = false;
    }

    private static void detach() {
        if (keyboardHost != null && keyboardLayout != null) {
            keyboardHost.getViewTreeObserver().removeOnGlobalLayoutListener(keyboardLayout);
        }
        keyboardHost = null;
        keyboardLayout = null;
        if (editor != null) {
            draft = editor.getText().toString();
            selectionStart = Math.max(0, editor.getSelectionStart());
            selectionEnd = Math.max(0, editor.getSelectionEnd());
        }
        if (dialog != null) {
            try { dialog.dismiss(); } catch (Throwable ignored) {}
        }
        editor = null;
        dialog = null;
    }

    // A card reopened over the one a rebuild drained (a rotation or a
    // light/dark change takes the card down and puts a new one up with the
    // draft) takes over the editor window that is showing, rather than
    // removing it and adding another. While the keyboard is up that window is
    // the input method's target, and the system layers the keyboard's
    // container relative to it: a window removed and another added in its
    // place while the display rotated left the keyboard's container layered
    // to the removed window, and the new window layered to the container, so
    // neither was composited. The new field was not drawn, was never given
    // input focus, and the next key ended in an input-dispatch ANR. A window
    // that stays hands nothing over. The text is set again only when the new
    // card brings other words; otherwise the reader's caret and any
    // composition in progress are kept.
    static void show(final long nextOwner, final byte[] initial, final byte[] hint,
                     final int text, final int hintText, final int paper, final int line,
                     final float sizePx, final float canvasScale) {
        final String value = new String(initial, StandardCharsets.UTF_8);
        final String caption = new String(hint, StandardCharsets.UTF_8);
        UI.post(new Runnable() { public void run() {
            if (nextOwner < owner) return;
            boolean live = wanted && editor != null && dialog != null && dialog.isShowing()
                    && activity != null && !activity.isFinishing();
            if (!live) detach();
            owner = nextOwner;
            placeholder = caption;
            textColor = text; hintColor = hintText; background = paper; border = line;
            textPx = sizePx; scale = canvasScale;
            wanted = focusWanted = true;
            keyboardPx = 0;
            if (!live) {
                draft = value;
                selectionStart = selectionEnd = draft.length();
                hasFrame = false;
                return;
            }
            style(editor);
            if (!editor.getText().toString().equals(value)) {
                editor.setText(value);
                editor.setSelection(value.length());
            }
            draft = editor.getText().toString();
            refreshKeyboard();
            focusNow(true);
        }});
    }

    static void frame(final long expected, final int left, final int top, final int w, final int h) {
        UI.post(new Runnable() { public void run() {
            if (expected != owner || !wanted || w <= 0 || h <= 0) return;
            x = left; y = top; width = w; height = h; hasFrame = true;
            boolean fresh = editor == null;
            if (!ensureView()) return;
            applyFrame();
            refreshKeyboard();
            if (fresh && focusWanted) focusNow(true);
        }});
    }

    static void focus(final long expected, final boolean focus) {
        UI.post(new Runnable() { public void run() {
            if (expected != owner || !wanted) return;
            focusWanted = focus;
            focusNow(focus);
        }});
    }

    static void hide(final long expected) {
        UI.post(new Runnable() { public void run() {
            if (expected != owner) return;
            wanted = focusWanted = false;
            focusNow(false);
            detach();
            keyboardPx = 0;
            BtBridge.composeKeyboard(expected, 0);
        }});
    }

    // The callbacks below act for the card that owns the editor when they
    // run, read from owner then, not for the one that made the view: a kept
    // window serves every card reopened over it.
    private static boolean ensureView() {
        if (editor != null) return true;
        if (!wanted || !hasFrame || activity == null || activity.isFinishing()) return false;
        final Activity act = activity;
        final EditText field = new EditText(act);
        field.setIncludeFontPadding(false);
        field.setGravity(Gravity.TOP | Gravity.START);
        field.setSingleLine(false);
        field.setInputType(InputType.TYPE_CLASS_TEXT | InputType.TYPE_TEXT_FLAG_MULTI_LINE
                | InputType.TYPE_TEXT_FLAG_CAP_SENTENCES | InputType.TYPE_TEXT_FLAG_AUTO_CORRECT);
        field.setImeOptions(android.view.inputmethod.EditorInfo.IME_FLAG_NO_EXTRACT_UI
                | android.view.inputmethod.EditorInfo.IME_FLAG_NO_PERSONALIZED_LEARNING);
        // The draft is the reader's own words: declared out of autofill and
        // content capture, rather than left to the heuristic that keeps a view
        // with no id and no autofill hints out of them.
        if (Build.VERSION.SDK_INT >= 26) field.setImportantForAutofill(View.IMPORTANT_FOR_AUTOFILL_NO);
        if (Build.VERSION.SDK_INT >= 29) field.setImportantForContentCapture(View.IMPORTANT_FOR_CONTENT_CAPTURE_NO);
        field.setHorizontallyScrolling(false);
        style(field);
        field.setText(draft);
        field.setSelection(Math.min(selectionStart, draft.length()), Math.min(selectionEnd, draft.length()));
        field.addTextChangedListener(new TextWatcher() {
            public void beforeTextChanged(CharSequence s, int start, int count, int after) {}
            public void onTextChanged(CharSequence s, int start, int before, int count) {}
            public void afterTextChanged(Editable value) {
                if (editor != field || activity != act || !wanted) return;
                draft = value.toString();
                BtBridge.composeChanged(owner, draft.getBytes(StandardCharsets.UTF_8));
            }
        });
        field.getViewTreeObserver().addOnWindowFocusChangeListener(new android.view.ViewTreeObserver.OnWindowFocusChangeListener() {
            public void onWindowFocusChanged(boolean focused) {
                if (focused && editor == field && activity == act && wanted && focusWanted) {
                    focusNow(true);
                }
            }
        });
        Dialog box = new Dialog(act, android.R.style.Theme_Translucent_NoTitleBar);
        box.setCancelable(false);
        box.setCanceledOnTouchOutside(false);
        box.setContentView(field);
        Window window = box.getWindow();
        window.setBackgroundDrawable(new ColorDrawable(Color.TRANSPARENT));
        window.clearFlags(WindowManager.LayoutParams.FLAG_DIM_BEHIND
                | WindowManager.LayoutParams.FLAG_NOT_FOCUSABLE | WindowManager.LayoutParams.FLAG_ALT_FOCUSABLE_IM);
        window.addFlags(WindowManager.LayoutParams.FLAG_NOT_TOUCH_MODAL | WindowManager.LayoutParams.FLAG_LAYOUT_IN_SCREEN);
        window.setSoftInputMode(WindowManager.LayoutParams.SOFT_INPUT_ADJUST_NOTHING);
        WindowManager.LayoutParams lp = window.getAttributes();
        lp.gravity = Gravity.TOP | Gravity.START;
        lp.windowAnimations = 0;
        window.setAttributes(lp);
        box.setOnKeyListener(new android.content.DialogInterface.OnKeyListener() {
            public boolean onKey(android.content.DialogInterface d, int code, KeyEvent event) {
                if (code == KeyEvent.KEYCODE_BACK) {
                    if (event.getAction() == KeyEvent.ACTION_UP && !event.isCanceled()) back(act);
                    return true;
                }
                return false;
            }
        });
        if (Build.VERSION.SDK_INT < 30) {
            keyboardHost = act.getWindow().getDecorView();
            keyboardLayout = new android.view.ViewTreeObserver.OnGlobalLayoutListener() {
                public void onGlobalLayout() {
                    if (activity == act && wanted) refreshKeyboard();
                }
            };
            keyboardHost.getViewTreeObserver().addOnGlobalLayoutListener(keyboardLayout);
        }
        editor = field;
        dialog = box;
        try {
            box.show();
            if (Build.VERSION.SDK_INT >= 33) {
                window.getOnBackInvokedDispatcher().registerOnBackInvokedCallback(
                        android.window.OnBackInvokedDispatcher.PRIORITY_DEFAULT,
                        new android.window.OnBackInvokedCallback() {
                            public void onBackInvoked() { back(act); }
                        });
            }
            window.getDecorView().requestApplyInsets();
            return true;
        } catch (WindowManager.BadTokenException stale) {
            detach();
            return false;
        }
    }

    // What a card gives the field: its text size, colours, hint and frame. A
    // new field takes them as it is made, and a kept field takes the reopened
    // card's, which a light/dark change gives other colours.
    private static void style(EditText field) {
        field.setTextSize(TypedValue.COMPLEX_UNIT_PX, textPx);
        field.setTextColor(textColor);
        field.setHintTextColor(hintColor);
        // The hint labels the field, and TalkBack reads it for an empty field.
        // A content description on an editable view is spoken on input focus in
        // place of the text.
        field.setHint(placeholder);
        GradientDrawable shape = new GradientDrawable();
        shape.setColor(background);
        shape.setCornerRadius(10*scale);
        shape.setStroke(Math.max(1, Math.round(scale)), border);
        field.setBackground(shape);
        applyPadding(field, height);
    }

    private static void back(Activity act) {
        if (activity != act || !wanted) return;
        refreshKeyboard();
        if (keyboardPx > 0) {
            focusWanted = false;
            focusNow(false);
        } else {
            BtBridge.composeDismiss(owner);
        }
    }

    // The editor Dialog's insets describe its small frame, so they can be zero
    // while the IME covers the bottom of the activity. Use the activity window
    // for the canvas-wide overlap, including when a new owner reuses an open IME.
    static void activityKeyboard(Activity act, float pixels) {
        keyboard(owner, act, pixels);
    }

    private static void refreshKeyboard() {
        if (activity == null || !wanted) return;
        View decor = activity.getWindow().getDecorView();
        if (Build.VERSION.SDK_INT >= 30) {
            android.view.WindowInsets insets = decor.getRootWindowInsets();
            if (insets != null) keyboard(owner, activity,
                    insets.getInsets(android.view.WindowInsets.Type.ime()).bottom);
        } else {
            Rect visible = new Rect();
            decor.getWindowVisibleDisplayFrame(visible);
            int overlap = Math.max(0, activity.getResources().getDisplayMetrics().heightPixels-visible.bottom);
            if (overlap < activity.getResources().getDisplayMetrics().density*100) overlap = 0;
            keyboard(owner, activity, overlap);
        }
    }

    private static void keyboard(long token, Activity act, float pixels) {
        if (owner != token || activity != act || !wanted || keyboardPx == pixels) return;
        keyboardPx = pixels;
        BtBridge.composeKeyboard(token, pixels);
    }

    private static void applyFrame() {
        if (dialog == null || !dialog.isShowing()) return;
        int[] origin = BtBridge.composeOrigin();
        Window window = dialog.getWindow();
        WindowManager.LayoutParams lp = window.getAttributes();
        lp.x = x+origin[0]; lp.y = y+origin[1]; lp.width = width; lp.height = height;
        window.setAttributes(lp);
        applyPadding(editor, height);
    }

    // The Android field uses 6-unit side padding and up to 10 units vertically.
    // UIKit's text-container inset also has its own line-fragment padding, so
    // these values do not promise identical effective horizontal text insets.
    // Reduce vertical padding when the frame has less room than the usual
    // inset plus one font row. A frame shorter than the row can still clip it.
    private static void applyPadding(EditText field, int frameHeight) {
        android.graphics.Paint.FontMetrics fm = field.getPaint().getFontMetrics();
        int centred = Math.max(0, (int) Math.floor((frameHeight-(fm.descent-fm.ascent))/2));
        int vertical = Math.min(Math.round(10*scale), centred);
        field.setPadding(Math.round(6*scale), vertical, Math.round(6*scale), vertical);
    }

    private static void focusNow(boolean focus) {
        if (editor == null || activity == null) return;
        InputMethodManager ime = (InputMethodManager) activity.getSystemService(Context.INPUT_METHOD_SERVICE);
        if (ime == null) return;
        if (!focus) {
            ime.hideSoftInputFromWindow(editor.getWindowToken(), 0);
            return;
        }
        final EditText field = editor;
        final Activity act = activity;
        field.requestFocus();
        field.post(new Runnable() { public void run() {
            if (editor == field && activity == act && wanted && focusWanted) {
                InputMethodManager live = (InputMethodManager) act.getSystemService(Context.INPUT_METHOD_SERVICE);
                if (live != null) live.showSoftInput(field, InputMethodManager.SHOW_IMPLICIT);
            }
        }});
    }
}
