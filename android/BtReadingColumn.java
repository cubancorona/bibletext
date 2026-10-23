package org.bibletext;

/**
 * Where the book page's column stands across the reading overlay: the side
 * padding BtBridge.applyReadingPadding gives the text view. Plain arithmetic
 * over pixel counts the bridge reads from the window, with no platform types,
 * so the host test suite compiles this file and runs it as it ships
 * (android_column_centre_test.go).
 *
 * The column is centred on the pane, as the iPad centres it in its text view
 * (btIOSApplyInsets), unless the pane stands evenly in the window less the
 * system insets at its sides; then it is centred on the window. On a phone
 * held sideways the toolkit keeps the pane out of the camera cutout, so the
 * pane's centre is half the cutout's inset off the screen's, and the column
 * centred on the window clears the cutout evenly, as the iPhone's does. Beside
 * a rail the pane does not stand evenly in the window, and its column is
 * centred on it; centred on the window, a tablet's column stood about half
 * the rail's width left of the pane's centre.
 */
final class BtReadingColumn {
    private BtReadingColumn() {}

    /**
     * The most the pane's two margins in the window may differ by and still
     * count as even. The frame's position and width are each rounded to a
     * pixel from the toolkit's units, so a pane laid out evenly can read a
     * pixel uneven; a rail is tens of pixels.
     */
    static final int EVEN_SLACK_PX = 2;

    /**
     * sides answers the text view's left and right padding for a column
     * measurePx wide, or null where the padding Go pushed, sideMin each side,
     * stands: the phone page, or a column with no room for a wider margin.
     *
     * All in pixels. viewW is the text view's width. windowW is the activity
     * window's width, 0 when it is not known, and insetL and insetR are its
     * system insets at the sides. frameX and frameW are the overlay's frame as
     * Go pushed it, measured from the left edge of the toolkit's content area,
     * which starts after insetL.
     */
    static int[] sides(int viewW, int measurePx, int sideMin, int windowW,
                       int insetL, int insetR, int frameX, int frameW) {
        if (measurePx <= 0 || viewW <= 0) return null;
        if (windowW > 0 && evenInWindow(windowW, insetL, insetR, frameX, frameW)) {
            // The column's left edge at the window's centre less half the
            // measure, in window coordinates, less the view's own place in
            // the window, which is taken from the frame and not from the
            // view's location on screen (BtBridge.applyReadingPadding says
            // why).
            int viewLeft = frameX + insetL;
            int left = (windowW - measurePx) / 2 - viewLeft;
            int right = viewW - left - measurePx;
            // Only when the column fits with at least the phone page's margin
            // on both sides; otherwise the pane's centre below.
            if (left >= sideMin && right >= sideMin) return new int[]{left, right};
        }
        int side = (viewW - measurePx) / 2;
        if (side > sideMin) return new int[]{side, side};
        return null;
    }

    /**
     * evenInWindow reports whether the pane's margins in the window less its
     * side insets are equal: the pane spans that width, less margins of its
     * own that are the same on both sides.
     */
    static boolean evenInWindow(int windowW, int insetL, int insetR, int frameX, int frameW) {
        int left = frameX;
        int right = windowW - insetL - insetR - frameX - frameW;
        return Math.abs(left - right) <= EVEN_SLACK_PX;
    }
}
