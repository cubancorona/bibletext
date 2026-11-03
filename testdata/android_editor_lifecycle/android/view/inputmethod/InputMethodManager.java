package android.view.inputmethod;
public class InputMethodManager {
    public static final int SHOW_IMPLICIT=1;
    public int showCalls,hideCalls;
    public boolean hideSoftInputFromWindow(Object token,int flags) { hideCalls++; return true; }
    public boolean showSoftInput(android.view.View view,int flags) { showCalls++; return true; }
}
