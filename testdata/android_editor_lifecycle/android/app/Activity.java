package android.app;
public class Activity extends android.content.Context {
    public boolean finishing;
    private final android.view.Window window = new android.view.Window();
    private final android.content.res.Resources resources = new android.content.res.Resources();
    public boolean isFinishing() { return finishing; }
    public android.view.Window getWindow() { return window; }
    public android.content.res.Resources getResources() { return resources; }
}
