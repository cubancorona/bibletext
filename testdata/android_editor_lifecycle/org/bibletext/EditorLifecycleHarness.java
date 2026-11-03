package org.bibletext;

import android.app.Activity;
import android.app.Dialog;
import android.widget.EditText;
import java.lang.reflect.Field;
import java.nio.charset.StandardCharsets;

/** Executes the production editor against deterministic host framework protocols. */
public final class EditorLifecycleHarness {
    private static final String DRAFT="A synthetic draft";
    private static Activity activity;

    private static Object member(String name) throws Exception {
        Field field=BtNoteEntry.class.getDeclaredField(name); field.setAccessible(true); return field.get(null);
    }
    private static void require(boolean condition,String message) {
        if(!condition) throw new AssertionError(message);
    }
    private static void show(long owner,String text) {
        BtNoteEntry.show(owner,text.getBytes(StandardCharsets.UTF_8),"Fixture hint".getBytes(StandardCharsets.UTF_8),1,2,3,4,14,1);
    }
    private static EditText start() throws Exception {
        activity=new Activity(); BtNoteEntry.init(activity); show(1,DRAFT); BtNoteEntry.frame(1,5,6,300,112);
        EditText editor=(EditText)member("editor");
        require(editor!=null,"fixture did not create an editor");
        require(((Dialog)member("dialog")).isShowing(),"fixture dialog not shown");
        return editor;
    }
    public static void main(String[] args) throws Exception {
        String name=args[0]; EditText editor=start(); Dialog dialog=(Dialog)member("dialog");
        if(name.equals("finishing-activity")) {
            activity.finishing=true; show(2,DRAFT);
            require(member("editor")==null,"a finishing activity kept its editor");
            require(dialog.dismissed,"the finishing activity's old dialog was not detached");
        } else if(name.equals("same-text-selection")) {
            editor.setSelection(2,7); editor.composing=true; int writes=editor.setTextCalls;
            show(2,DRAFT);
            require(member("editor")==editor && member("dialog")==dialog,"reopening replaced the live editor window");
            require(editor.setTextCalls==writes,"unchanged draft was set again");
            require(editor.getSelectionStart()==2 && editor.getSelectionEnd()==7,"reopening moved the retained selection");
            require(editor.composing,"reopening discarded composition state");
        } else if(name.equals("keyboard-refresh")) {
            activity.getWindow().getDecorView().insets.bottom=144; show(2,DRAFT);
            require(BtBridge.keyboardOwner==2 && BtBridge.keyboardPixels==144,"reopened owner did not receive current keyboard overlap");
        } else if(name.equals("focus-refresh")) {
            int requests=editor.focusCalls,shows=activity.ime.showCalls; show(2,DRAFT);
            require(editor.focusCalls>requests && activity.ime.showCalls>shows,"reopened editor was not refocused");
        } else if(name.equals("current-owner-callbacks")) {
            show(2,DRAFT); editor.setText("An edited synthetic draft");
            require(BtBridge.changedOwner==2,"the kept field sent a change for its creator instead of its current owner");
            require("An edited synthetic draft".equals(BtBridge.changedText),"the current draft was not delivered");
            dialog.key(android.view.KeyEvent.KEYCODE_BACK,new android.view.KeyEvent(android.view.KeyEvent.ACTION_UP));
            require(BtBridge.dismissedOwner==2,"the kept dialog dismissed its creator instead of its current owner");
        } else if(name.equals("stale-field-callbacks")) {
            activity.finishing=true; show(2,DRAFT); activity.finishing=false;
            BtNoteEntry.frame(2,5,6,300,112); EditText replacement=(EditText)member("editor");
            require(replacement!=null && replacement!=editor,"fixture did not replace the stale field");
            BtBridge.changedOwner=0; editor.setText("A stale synthetic change");
            require(BtBridge.changedOwner==0,"a detached field changed the replacement card");
        } else if(name.equals("dismiss-already-removed")) {
            Dialog.throwOnDismiss=true; BtNoteEntry.hide(1);
            require(member("editor")==null && member("dialog")==null,"a failed platform dismiss left editor references alive");
        } else throw new AssertionError("unknown fixture case");
        System.out.println("PASS "+name);
    }
}
