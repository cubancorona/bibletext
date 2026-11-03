package android.view;
public class KeyEvent {
    public static final int KEYCODE_BACK=4,ACTION_UP=1;
    private final int action;
    public KeyEvent(int value) { action=value; }
    public int getAction() { return action; }
    public boolean isCanceled() { return false; }
}
