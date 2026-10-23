package bibletext

import (
	"slices"
	"strings"
	"testing"
)

// A PSALM TITLE IS NOT HYPHENATED ON ANDROID 15 AND LATER, as the web and the
// Windows and Linux pane leave its words whole (decided 6 October 2026).
//
// THE DEFECT THIS EXISTS FOR. The reading view hyphenates its whole text
// (HYPHENATION_FREQUENCY_NORMAL), so a title that wrapped could break a word:
// WEB Psalm 102's "over-whelmed" on the API 35 emulator, at the default size
// on a phone held upright. Hyphenation belongs to the whole TextView, and the
// one way to turn it off for one paragraph is the no-hyphenation span API 35
// brought (LineBreakConfigSpan), which the selectable view's layout honours
// there. Earlier releases have no such span: from API 23, where the view
// starts hyphenating, to 34 they keep hyphenating a title that wraps, a
// divergence docs/PLATFORM_MATRIX.md records.
//
// The span shapes the page, not the words, and it is a framework
// ParcelableSpan, which Copy would carry into another app's editor with the
// title's italic. So the text that leaves the page drops it: readerText for
// the app's own verbs and for Copy, and copyAsRead takes Copy over whenever
// the selection holds one, even where its characters are the view's own.
//
// The pane is Java behind JNI, so this holds the bridge's source to the rule
// line by line and proves on copies that each check can fail; the emulator
// shots are in the change that brought it.
func TestAndroidPsalmTitleIsNotHyphenatedFromAPI35(t *testing.T) {
	java := readNativeSource(t, "android/BtBridge.java")
	whole := func(src, head string, body ...string) bool {
		return slices.Equal(codeLines(javaBlockAfter(t, src, head)), body)
	}

	// The span, over the title's paragraph alone, only where it exists.
	sets := func(src string) bool {
		keep := javaBlockAfter(t, src, "private static void keepHeadingsRagged(")
		return whole(src, "private static void keepTitleUnhyphenated(",
			"private static void keepTitleUnhyphenated(android.text.SpannableStringBuilder ssb, int ps, int pe) {",
			"if (android.os.Build.VERSION.SDK_INT < 35 || pe <= ps) return;",
			"ssb.setSpan(android.text.style.LineBreakConfigSpan.createNoHyphenationSpan(), ps, pe,",
			"Spanned.SPAN_EXCLUSIVE_EXCLUSIVE);",
			"}") &&
			hasLinesInOrder(keep,
				"if (isRaggedParagraph(ssb, ps, i)) {",
				"if (ps == 0 && isTitleParagraph(ssb, ps, i)) keepTitleUnhyphenated(ssb, ps, i);",
				"}",
				"ps = i + 1;") &&
			strings.Count(src, "keepTitleUnhyphenated(") == 2 && // the declaration and its one call
			hasLinesInOrder(javaBlockAfter(t, src, "public static void setHtml("),
				"if (s instanceof android.text.SpannableStringBuilder && justifiesInterWord()) {",
				"keepHeadingsRagged((android.text.SpannableStringBuilder) s);",
				"text.setText(s, TextView.BufferType.SPANNABLE);")
	}
	// Every other paragraph keeps hyphenating as it did: the view's own
	// frequency is unchanged.
	bodyHyphenates := func(src string) bool {
		return strings.Contains(src, "text.setHyphenationFrequency(android.text.Layout.HYPHENATION_FREQUENCY_NORMAL);")
	}
	// And the text that leaves the page drops it.
	drops := func(src string) bool {
		read := javaBlockAfter(t, src, "readerText(CharSequence cs, int s0, int s1)")
		asRead := javaBlockAfter(t, src, "private static CharSequence selectionAsRead()")
		copyAs := javaBlockAfter(t, src, "private static boolean copyAsRead()")
		return whole(src, "private static boolean holdsPageOnlySpan(",
			"private static boolean holdsPageOnlySpan(CharSequence cs, int s0, int s1) {",
			"if (android.os.Build.VERSION.SDK_INT < 35 || !(cs instanceof Spanned)) return false;",
			"return ((Spanned) cs).getSpans(s0, s1, android.text.style.LineBreakConfigSpan.class).length > 0;",
			"}") &&
			whole(src, "private static void dropPageOnlySpans(",
				"private static void dropPageOnlySpans(android.text.SpannableStringBuilder sb) {",
				"if (android.os.Build.VERSION.SDK_INT < 35) return;",
				"for (Object o : sb.getSpans(0, sb.length(), android.text.style.LineBreakConfigSpan.class)) {",
				"sb.removeSpan(o);",
				"}",
				"}") &&
			hasLinesInOrder(read,
				"|| (raw.indexOf(HEADING_GAP) < 0 && raw.indexOf(PARA_LTR_MARK) < 0",
				"&& !holdsPageOnlySpan(cs, s0, s1))) return raw;",
				"android.text.SpannableStringBuilder sb = new android.text.SpannableStringBuilder(cs, s0, s1);",
				"dropPageOnlySpans(sb);",
				"return sb;") &&
			hasLinesInOrder(asRead,
				"CharSequence read = readerText(cs, s0, s1);",
				"if (android.text.TextUtils.equals(read, cs.subSequence(s0, s1))",
				"&& !holdsPageOnlySpan(cs, s0, s1)) return null;",
				"return read;") &&
			hasLinesInOrder(copyAs,
				"CharSequence read = selectionAsRead();",
				"if (read == null) return false;",
				"cm.setPrimaryClip(android.content.ClipData.newPlainText(null, read));")
	}
	if !sets(java) {
		t.Error("setHtml's pass must set the no-hyphenation span over the Psalm title's paragraph alone, " +
			"on API 35 and later (keepTitleUnhyphenated, called once from keepHeadingsRagged)")
	}
	if !bodyHyphenates(java) {
		t.Error("the view must keep hyphenating the rest of the page (HYPHENATION_FREQUENCY_NORMAL)")
	}
	if !drops(java) {
		t.Error("the title's no-hyphenation span must not leave the page: readerText drops it " +
			"(dropPageOnlySpans), and copyAsRead takes Copy over wherever a selection holds one " +
			"(holdsPageOnlySpan), or Copy carries it into another app with the title")
	}

	for _, c := range []struct {
		name, from, to string
		check          func(string) bool
	}{
		{"never set", "if (ps == 0 && isTitleParagraph(ssb, ps, i)) keepTitleUnhyphenated(ssb, ps, i);", "", sets},
		{"set on every heading too", "if (ps == 0 && isTitleParagraph(ssb, ps, i)) keepTitleUnhyphenated(ssb, ps, i);",
			"keepTitleUnhyphenated(ssb, ps, i);", sets},
		{"set below API 35, where the class does not exist", "if (android.os.Build.VERSION.SDK_INT < 35 || pe <= ps) return;",
			"if (pe <= ps) return;", sets},
		{"the no-break span instead", "LineBreakConfigSpan.createNoHyphenationSpan()",
			"LineBreakConfigSpan.createNoBreakSpan()", sets},
		{"hyphenation off for the whole page", "text.setHyphenationFrequency(android.text.Layout.HYPHENATION_FREQUENCY_NORMAL);",
			"text.setHyphenationFrequency(android.text.Layout.HYPHENATION_FREQUENCY_NONE);", bodyHyphenates},
		{"kept by readerText", "        dropPageOnlySpans(sb);\n", "", drops},
		{"returned as it stands with the span", "\n                    && !holdsPageOnlySpan(cs, s0, s1))) return raw;",
			")) return raw;", drops},
		{"the platform's Copy with the span", "\n                && !holdsPageOnlySpan(cs, s0, s1)) return null;",
			") return null;", drops},
		{"looked for below API 35", "if (android.os.Build.VERSION.SDK_INT < 35 || !(cs instanceof Spanned)) return false;",
			"if (!(cs instanceof Spanned)) return false;", drops},
		{"only some removed", "            sb.removeSpan(o);\n", "            sb.removeSpan(o);\n            break;\n", drops},
	} {
		if k := strings.Count(java, c.from); k != 1 {
			t.Fatalf("control %q: the bridge contains %q %d times, not once", c.name, c.from, k)
		}
		if c.check(strings.Replace(java, c.from, c.to, 1)) {
			t.Errorf("control %q: the check passes a bridge without it, so it proves nothing", c.name)
		}
	}
}
