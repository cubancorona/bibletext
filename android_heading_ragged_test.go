package bibletext

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// A HEADING IS SET RAGGED ON ANDROID'S JUSTIFIED PAGE, as the web, the Apple
// panes and the Windows and Linux pane set it.
//
// THE DEFECT THIS EXISTS FOR. From Android 15 the pane justifies INTER_WORD,
// and that is a property of the whole TextView: the layout spreads every line
// that does not end at a hard break, whatever its paragraph. A heading that
// wraps — the NKJV's over Psalm 3, on a phone — had its first line spread to
// the measure, its words pushed apart. No span can exempt a line
// (TextLine.justify sets the word spacing after the spans have set the
// paint); a line with no U+0020 is the one it leaves alone. So the bridge sets
// a heading's gaps as another space in the view and gives the reader the
// spaces back in the text that leaves the page.
//
// The pane is Java behind JNI, so this holds the bridge's source to the rule,
// checks the gap character against the platform's rules and the reading face,
// and proves on copies that each check can fail.
func TestAndroidSetsAHeadingRaggedOnTheJustifiedPage(t *testing.T) {
	java := readNativeSource(t, "android/BtBridge.java")

	// The gap character, read from the bridge.
	m := regexp.MustCompile(`static final char HEADING_GAP = '\\u([0-9A-Fa-f]{4})';`).FindStringSubmatch(java)
	if m == nil {
		t.Fatal("the bridge declares no HEADING_GAP; a heading's gaps are U+0020 in the view, " +
			"and on the justified page a heading that wraps has its first line spread")
	}
	n, _ := strconv.ParseUint(m[1], 16, 32)
	gap := rune(n)
	switch {
	case gap == ' ':
		t.Errorf("HEADING_GAP is U+0020, the one space INTER_WORD stretches " +
			"(TextLine.isStretchableWhitespace) and Minikin lets a justified line shrink")
	case !unicode.Is(unicode.Zs, gap):
		t.Errorf("HEADING_GAP U+%04X is not a space separator", gap)
	case gap < 0x2000 || gap > 0x200A || gap == 0x2007:
		// U+2000-U+200A less the figure space: a line may break after one
		// (line-break class BA), and TextLine.isLineEndSpace and Minikin's
		// isLineEndSpace drop one that ends a line, as they drop U+0020. The
		// no-break spaces (U+00A0, U+2007, U+202F) would keep a heading from
		// wrapping at all.
		t.Errorf("HEADING_GAP U+%04X is not a breaking space the layout drops at a line end", gap)
	}
	// It must be as wide as the space it stands for, in the face that draws a
	// heading (the reading family's bold cut), or the heading's words move.
	f, err := sfnt.Parse(readingFontBold)
	if err != nil {
		t.Fatalf("parse the bold reading face: %v", err)
	}
	var buf sfnt.Buffer
	upm := fixed.Int26_6(f.UnitsPerEm()) << 6
	advance := func(r rune) (fixed.Int26_6, bool) {
		i, err := f.GlyphIndex(&buf, r)
		if err != nil || i == 0 {
			return 0, false
		}
		a, err := f.GlyphAdvance(&buf, i, upm, 0)
		return a, err == nil
	}
	space, _ := advance(' ')
	if w, ok := advance(gap); !ok {
		t.Errorf("the bold reading face has no U+%04X, so a heading's gaps would come from a fallback face", gap)
	} else if d := w - space; d > upm/100 || d < -upm/100 {
		t.Errorf("U+%04X is %d units wide in the bold reading face and the space %d; more than "+
			"a hundredth of an em apart, a heading's words would visibly move",
			gap, w>>6, space>>6)
	}

	// The swap, on the justified page only, before the text reaches the view,
	// and in a heading's paragraph only. The guard is read as it stands: the
	// same pass with its test turned round sets every verse's spaces as gaps,
	// and the justified page then justifies no verse at all.
	swaps := func(src string) bool {
		set := javaBlockAfter(t, src, "public static void setHtml(")
		keep := javaBlockAfter(t, src, "private static void keepHeadingsRagged(")
		style := javaBlockAfter(t, src, "private static boolean justifiesInterWord()")
		return inSequence(set, "justifiesInterWord()", "keepHeadingsRagged(", "text.setText(s") &&
			inSequence(keep,
				"if (isHeadingParagraph(ssb, ps, i)) {",
				"for (int j = ps; j < i; j++) {",
				"if (ssb.charAt(j) == ' ') ssb.replace(j, j + 1, String.valueOf(HEADING_GAP));") &&
			strings.Contains(style, "android.os.Build.VERSION.SDK_INT >= 35") &&
			strings.Contains(src, "text.setJustificationMode(justifiesInterWord()")
	}
	if !swaps(java) {
		t.Error("setHtml must swap a heading's spaces for HEADING_GAP (keepHeadingsRagged) " +
			"before setText, on the page setStyle justifies — one predicate, justifiesInterWord, for both")
	}

	// And the reader gets the spaces back: the app's own verbs and Copy.
	givesBack := func(src string) bool {
		read := javaBlockAfter(t, src, "private static String readerText(")
		copyAs := javaBlockAfter(t, src, "private static boolean copyAsRead()")
		menu := javaBlockAfter(t, src, "@Override public boolean onTextContextMenuItem(int id)")
		return inSequence(read, "isHeadingParagraph(", "== HEADING_GAP", "' '") &&
			inSequence(copyAs, "readerText(", "setPrimaryClip(") &&
			inSequence(menu, "android.R.id.copy", "copyAsRead()", "super.onTextContextMenuItem(id)") &&
			strings.Contains(src, "final String sel = readerText(text.getText(), s0, s1);") &&
			!strings.Contains(src, "getText().subSequence(s0, s1).toString()")
	}
	if !givesBack(java) {
		t.Error("a selection's text must come through readerText — the app's verbs (sel) and " +
			"Copy (onTextContextMenuItem -> copyAsRead) — or a heading leaves the page with " +
			"four-per-em spaces in it")
	}

	// copyAsRead says false only where it has not copied, and true once it
	// has. A copyAsRead that set the reader's text on the clipboard and still
	// said false would hand Copy on to the platform, which sets the view's
	// text over it; one whose comparison was turned round would take Copy
	// over only for a selection with nothing to give back. So its ways out
	// are read as they stand, in order, with the one that says true after
	// the clipboard is set.
	copies := func(src string) bool {
		copyAs := javaBlockAfter(t, src, "private static boolean copyAsRead()")
		var exits []string
		for _, l := range strings.Split(copyAs, "\n") {
			if l = strings.TrimSpace(l); strings.Contains(l, "return ") && !strings.HasPrefix(l, "//") {
				exits = append(exits, l)
			}
		}
		return slices.Equal(exits, []string{
			"if (text == null) return false;",
			"if (a < 0 || b < 0 || a == b) return false;",
			"if (read.contentEquals(cs.subSequence(s0, s1))) return false;",
			"if (cm == null) return false;",
			"return false;", // the clipboard threw
			"return true;",
		}) && inSequence(copyAs,
			"String read = readerText(cs, s0, s1);",
			"if (read.contentEquals(cs.subSequence(s0, s1))) return false;",
			"cm.setPrimaryClip(android.content.ClipData.newPlainText(null, read));",
			"} catch (Throwable t) {", "return false;", "}",
			"return true;")
	}
	if !copies(java) {
		t.Error("copyAsRead must say true once it has set the reader's text on the clipboard, and " +
			"false only where it has not — no view, no selection, nothing readerText changes, no " +
			"clipboard, a clipboard that threw — or the platform's Copy runs after it and the " +
			"view's text is what pastes")
	}

	// The controls: each check fails a bridge that lost the piece it holds.
	for _, c := range []struct {
		name, from, to string
		check          func(string) bool
	}{
		{"no swap in setHtml", "keepHeadingsRagged((android.text.SpannableStringBuilder) s);", "", swaps},
		{"justified on its own predicate", "text.setJustificationMode(justifiesInterWord()",
			"text.setJustificationMode(android.os.Build.VERSION.SDK_INT >= 35", swaps},
		{"every paragraph but a heading", "if (isHeadingParagraph(ssb, ps, i)) {",
			"if (!isHeadingParagraph(ssb, ps, i)) {", swaps},
		{"raw selection text", "final String sel = readerText(text.getText(), s0, s1);",
			"final String sel = text.getText().subSequence(s0, s1).toString();", givesBack},
		{"the platform's Copy", "if (id == android.R.id.copy && copyAsRead()) return true;", "", givesBack},
		{"false once copied", "selectionMode.finish();\n        return true;",
			"selectionMode.finish();\n        return false;", copies},
		{"false before copying", "private static boolean copyAsRead() {",
			"private static boolean copyAsRead() {\n        if (true) return false;", copies},
		{"the comparison turned round", "if (read.contentEquals(cs.subSequence(s0, s1))) return false;",
			"if (!read.contentEquals(cs.subSequence(s0, s1))) return false;", copies},
	} {
		mutated := strings.Replace(java, c.from, c.to, 1)
		if mutated == java {
			t.Fatalf("control %q: the bridge no longer contains %q", c.name, c.from)
		}
		if c.check(mutated) {
			t.Errorf("control %q: the check passes a bridge without it, so it proves nothing", c.name)
		}
	}
}
