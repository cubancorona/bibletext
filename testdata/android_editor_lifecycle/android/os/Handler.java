package android.os;
public class Handler { public Handler(Looper looper) {} public boolean post(Runnable run) { run.run(); return true; } }
