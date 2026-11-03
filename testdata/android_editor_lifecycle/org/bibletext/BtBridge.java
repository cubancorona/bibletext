package org.bibletext;
public class BtBridge {
    public static long changedOwner,dismissedOwner,keyboardOwner;
    public static String changedText;
    public static float keyboardPixels;
    public static void composeChanged(long owner,byte[] text) { changedOwner=owner; changedText=new String(text,java.nio.charset.StandardCharsets.UTF_8); }
    public static void composeDismiss(long owner) { dismissedOwner=owner; }
    public static void composeKeyboard(long owner,float pixels) { keyboardOwner=owner; keyboardPixels=pixels; }
    public static int[] composeOrigin() { return new int[]{0,0}; }
}
