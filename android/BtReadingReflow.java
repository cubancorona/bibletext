package org.bibletext;

import java.util.ArrayList;

// A page's semantic UTF-16 offsets, without the importer's paragraph air,
// heading spaces or paragraph direction marks. Word boundaries remain spaces;
// case, punctuation and surrogate pairs are never rewritten.
final class BtReadingReflow {
    private BtReadingReflow() {}

    static boolean gap(char c) {
        return c == ' ' || c == '\n' || c == '\r' || c == '\t'
                || c == '\u00a0' || c == '\u2005';
    }

    static boolean boundary(String text, int at) {
        return at >= 0 && at <= text.length()
                && !(at > 0 && at < text.length()
                    && Character.isHighSurrogate(text.charAt(at - 1))
                    && Character.isLowSurrogate(text.charAt(at)));
    }

    static final class Page {
        final String raw, text;
        final int[] starts, ends;
        Page(CharSequence input) {
            raw = input.toString();
            StringBuilder value = new StringBuilder();
            ArrayList<Integer> lo = new ArrayList<Integer>();
            ArrayList<Integer> hi = new ArrayList<Integer>();
            for (int at = 0; at < raw.length();) {
                char c = raw.charAt(at);
                if (c == '\u200e' && (at == 0 || raw.charAt(at - 1) == '\n')) {
                    at++;
                    continue;
                }
                if (gap(c)) {
                    int first = at++;
                    while (at < raw.length() && gap(raw.charAt(at))) at++;
                    if (value.length() > 0 && at < raw.length()) {
                        value.append(' '); lo.add(first); hi.add(at);
                    }
                    continue;
                }
                value.append(c); lo.add(at); hi.add(++at);
            }
            text = value.toString();
            starts = new int[lo.size()]; ends = new int[hi.size()];
            for (int n = 0; n < starts.length; n++) {
                starts[n] = lo.get(n); ends[n] = hi.get(n);
            }
        }

        int canonical(int offset, boolean end) {
            if (!boundary(raw, offset)) return -1;
            for (int n = 0; n < starts.length; n++) {
                if (offset <= starts[n]) return n;
                if (offset < ends[n]) return end ? n + 1 : n;
            }
            return starts.length;
        }

        int raw(int offset, boolean end) {
            if (!boundary(text, offset)) return -1;
            if (starts.length == 0) return 0;
            if (offset == 0) return starts[0];
            if (offset == starts.length) return ends[ends.length - 1];
            return end ? ends[offset - 1] : starts[offset];
        }
    }

    // A nonempty range is carried only when the requested chapter is the one
    // actually displayed and its whole semantic page is unchanged. If only the
    // appended apparatus differs, both ranges must lie in identical Scripture.
    static int[] selection(boolean requested, boolean sameChapter,
                           CharSequence before, int beforeEnd, int start, int end,
                           CharSequence after, int afterEnd) {
        if (!requested || !sameChapter || before == null || after == null || start == end) return null;
        Page a = new Page(before), b = new Page(after);
        int low = Math.min(start, end), high = Math.max(start, end);
        if (!boundary(a.raw, low) || !boundary(a.raw, high)) return null;
        int s = a.canonical(low, false), e = a.canonical(high, true);
        if (s < 0 || e <= s) return null;
        if (!a.text.equals(b.text)) {
            if (!boundary(a.raw, beforeEnd) || !boundary(b.raw, afterEnd) || high > beforeEnd) return null;
            Page ap = new Page(a.raw.substring(0, beforeEnd));
            Page bp = new Page(b.raw.substring(0, afterEnd));
            if (!ap.text.equals(bp.text) || e > ap.text.length()) return null;
        }
        int mappedStart = b.raw(s, false), mappedEnd = b.raw(e, true);
        if (mappedStart < 0 || mappedEnd <= mappedStart) return null;
        return start <= end ? new int[]{mappedStart, mappedEnd} : new int[]{mappedEnd, mappedStart};
    }

    static final class Anchor {
        final Page page;
        final int offset;
        final float lineFraction;
        final boolean top;
        Anchor(CharSequence text, int at, float fraction, boolean atTop) {
            page = new Page(text);
            offset = page.canonical(at, false);
            lineFraction = Float.isNaN(fraction) || Float.isInfinite(fraction) ? 0f : fraction;
            top = atTop;
        }

        int map(CharSequence after, int beforeEnd, int afterEnd) {
            Page next = new Page(after);
            if (offset < 0) return -1;
            if (!page.text.equals(next.text)) {
                if (!boundary(page.raw, beforeEnd) || !boundary(next.raw, afterEnd)) return -1;
                Page a = new Page(page.raw.substring(0, beforeEnd));
                Page b = new Page(next.raw.substring(0, afterEnd));
                if (!a.text.equals(b.text) || offset >= a.text.length()) return -1;
            }
            return next.raw(offset, false);
        }
    }
}
