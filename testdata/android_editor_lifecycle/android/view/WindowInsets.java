package android.view;
public class WindowInsets {
    public int bottom;
    public android.graphics.Insets getInsets(int type) { return new android.graphics.Insets(bottom); }
    public static class Type { public static int ime() { return 1; } }
}
