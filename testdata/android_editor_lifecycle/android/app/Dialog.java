package android.app;
public class Dialog implements android.content.DialogInterface {
    public static boolean throwOnDismiss;
    public boolean shown, dismissed;
    private final android.view.Window window = new android.view.Window();
    private android.content.DialogInterface.OnKeyListener listener;
    public Dialog(Activity activity,int theme) {}
    public boolean isShowing() { return shown; }
    public void dismiss() { if(throwOnDismiss) throw new IllegalStateException("fixture window already removed"); dismissed=true; shown=false; }
    public void setCancelable(boolean value) {}
    public void setCanceledOnTouchOutside(boolean value) {}
    public void setContentView(android.view.View view) {}
    public android.view.Window getWindow() { return window; }
    public void setOnKeyListener(android.content.DialogInterface.OnKeyListener next) { listener=next; }
    public boolean key(int code,android.view.KeyEvent event) { return listener.onKey(this,code,event); }
    public void show() { shown=true; }
}
