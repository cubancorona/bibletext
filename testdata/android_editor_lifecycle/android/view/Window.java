package android.view;
public class Window {
    private WindowManager.LayoutParams attributes = new WindowManager.LayoutParams();
    private final View decor = new View();
    public void setBackgroundDrawable(Object drawable) {}
    public void clearFlags(int value) {}
    public void addFlags(int value) {}
    public void setSoftInputMode(int value) {}
    public WindowManager.LayoutParams getAttributes() { return attributes; }
    public void setAttributes(WindowManager.LayoutParams value) { attributes=value; }
    public View getDecorView() { return decor; }
    public android.window.OnBackInvokedDispatcher getOnBackInvokedDispatcher() { return new android.window.OnBackInvokedDispatcher(); }
}
