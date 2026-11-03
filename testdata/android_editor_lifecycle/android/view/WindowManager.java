package android.view;
public interface WindowManager {
    class BadTokenException extends RuntimeException {}
    class LayoutParams {
        public static final int FLAG_DIM_BEHIND=1,FLAG_NOT_FOCUSABLE=2,FLAG_ALT_FOCUSABLE_IM=4,FLAG_NOT_TOUCH_MODAL=8,FLAG_LAYOUT_IN_SCREEN=16,SOFT_INPUT_ADJUST_NOTHING=32;
        public int x,y,width,height,gravity,windowAnimations;
    }
}
