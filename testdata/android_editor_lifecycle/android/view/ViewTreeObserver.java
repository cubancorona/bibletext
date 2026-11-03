package android.view;
public class ViewTreeObserver {
    public interface OnGlobalLayoutListener { void onGlobalLayout(); }
    public interface OnWindowFocusChangeListener { void onWindowFocusChanged(boolean focused); }
    public void addOnGlobalLayoutListener(OnGlobalLayoutListener listener) {}
    public void removeOnGlobalLayoutListener(OnGlobalLayoutListener listener) {}
    public void addOnWindowFocusChangeListener(OnWindowFocusChangeListener listener) {}
}
