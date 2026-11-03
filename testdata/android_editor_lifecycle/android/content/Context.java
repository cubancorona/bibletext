package android.content;
public class Context {
    public static final String INPUT_METHOD_SERVICE = "input";
    public final android.view.inputmethod.InputMethodManager ime = new android.view.inputmethod.InputMethodManager();
    public Object getSystemService(String name) { return ime; }
}
