package android.view;
public class View {
    public static final int IMPORTANT_FOR_AUTOFILL_NO=2,IMPORTANT_FOR_CONTENT_CAPTURE_NO=2;
    public final WindowInsets insets = new WindowInsets();
    private final ViewTreeObserver observer = new ViewTreeObserver();
    public ViewTreeObserver getViewTreeObserver() { return observer; }
    public WindowInsets getRootWindowInsets() { return insets; }
    public void getWindowVisibleDisplayFrame(android.graphics.Rect rect) { rect.bottom=1000; }
    public void requestApplyInsets() {}
    public Object getWindowToken() { return this; }
    public void setBackground(Object drawable) {}
    public boolean post(Runnable run) { run.run(); return true; }
}
