package bibletext

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// codeLines is block's statements one per line, trimmed, with blank lines and
// comment lines left out, so a check can hold a line as it stands rather than
// as a substring a changed line still contains: "isX(a)" is a substring of
// "if (!isX(a)) {" and of "if (isX(a) && false) {", and neither line is
// "if (isX(a)) {".
func codeLines(block string) []string {
	var out []string
	for _, l := range strings.Split(block, "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "//") {
			continue
		}
		out = append(out, l)
	}
	return out
}

// hasLinesInOrder reports whether each of want is a whole line of block, in
// that order.
func hasLinesInOrder(block string, want ...string) bool {
	lines := codeLines(block)
	i := 0
	for _, l := range lines {
		if i < len(want) && l == want[i] {
			i++
		}
	}
	return i == len(want)
}

// A HEADING, AND THE PSALM TITLE, IS SET RAGGED ON ANDROID'S JUSTIFIED PAGE,
// as the web, the Apple panes and the Windows and Linux pane set them.
//
// THE DEFECT THIS EXISTS FOR. From Android 15 the pane justifies INTER_WORD,
// and that is a property of the whole TextView: the layout spreads every line
// that does not end at a hard break, whatever its paragraph. A heading that
// wraps — the NKJV's over Psalm 3, on a phone — had its first line spread to
// the measure, its words pushed apart, and a Psalm title that wraps had every
// line but its last spread the same way. No span can exempt a line
// (TextLine.justify sets the word spacing after the spans have set the
// paint); a line with no U+0020 is the one it leaves alone. So the bridge sets
// the gaps of a heading and of the title as another space in the view and
// gives the reader the spaces back in the text that leaves the page.
//
// The pane is Java behind JNI, so this holds the bridge's source to the rule
// line by line, checks the gap character against the platform's rules and the
// reading faces, and proves on copies that each check can fail.
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

	// It must be as wide as the space it stands for, in the faces that draw
	// it: the reading family's bold cut for a heading, its italic cut for the
	// title, or the words move.
	advance := func(f *sfnt.Font, r rune) (fixed.Int26_6, bool) {
		var buf sfnt.Buffer
		i, err := f.GlyphIndex(&buf, r)
		if err != nil || i == 0 {
			return 0, false
		}
		a, err := f.GlyphAdvance(&buf, i, fixed.Int26_6(f.UnitsPerEm())<<6, 0)
		return a, err == nil
	}
	parse := func(name string, data []byte) *sfnt.Font {
		f, err := sfnt.Parse(data)
		if err != nil {
			t.Fatalf("parse the %s reading face: %v", name, err)
		}
		return f
	}
	bold, italic := parse("bold", readingFontBold), parse("italic", readingFontItalic)
	abs := func(v fixed.Int26_6) fixed.Int26_6 {
		if v < 0 {
			return -v
		}
		return v
	}
	widthProblems := func(r rune) []string {
		var out []string
		// The bold cut: within a hundredth of an em of its space.
		upm := fixed.Int26_6(bold.UnitsPerEm()) << 6
		space, _ := advance(bold, ' ')
		if w, ok := advance(bold, r); !ok {
			out = append(out, fmt.Sprintf("the bold reading face has no U+%04X, so a heading's gaps "+
				"would come from a fallback face", r))
		} else if d := abs(w - space); d > upm/100 {
			out = append(out, fmt.Sprintf("U+%04X is %d units wide in the bold reading face and the "+
				"space %d; more than a hundredth of an em apart, a heading's words would visibly move",
				r, w>>6, space>>6))
		}
		// The italic cut's space is wider (278 units to the bold cut's 243)
		// and no breaking space comes within a hundredth of an em of it, so
		// the gap must be within a hundredth of an em of the nearest one
		// there: no other choice would set the title's words much closer to
		// where its spaces set them.
		upm = fixed.Int26_6(italic.UnitsPerEm()) << 6
		space, _ = advance(italic, ' ')
		w, ok := advance(italic, r)
		if !ok {
			return append(out, fmt.Sprintf("the italic reading face has no U+%04X, so a title's gaps "+
				"would come from a fallback face", r))
		}
		best := abs(w - space)
		for c := rune(0x2000); c <= 0x200A; c++ {
			if cw, ok := advance(italic, c); ok && c != 0x2007 && abs(cw-space) < best {
				best = abs(cw - space)
			}
		}
		if abs(w-space)-best > upm/100 {
			out = append(out, fmt.Sprintf("U+%04X is %d units wide in the italic reading face and the "+
				"space %d, more than a hundredth of an em further from it than the nearest breaking "+
				"space; a title's words would move further than they need to", r, w>>6, space>>6))
		}
		return out
	}
	for _, p := range widthProblems(gap) {
		t.Error(p)
	}
	// The control: a three-per-em space (333 units) is a breaking space too,
	// and it fails both faces.
	if len(widthProblems(0x2004)) < 2 {
		t.Fatal("control: the width check passes U+2004 in a face, so it proves nothing there")
	}

	// The paragraph the bridge takes for the title is the one the dialect
	// writes for it: the chapter's first, wholly in <i>, on the phone page and
	// on the book page. Anything written ahead of it would leave the title to
	// justify again.
	st := sampleState()
	st.CurrentBook, st.CurrentChapter = "Psalms", 150
	st.Bible.Verses["Psalms"] = map[int][]Verse{150: {{BookName: "Psalms", Chapter: 150, Verse: 1, Text: "A fixture verse."}}}
	st.Bible.Superscriptions = map[string]map[int]Superscription{"Psalms": {150: {Text: "A fixture title that runs on."}}}
	for _, book := range []bool{false, true} {
		var html string
		withReporterLayout(book, func() { html = buildChapterHTMLAndroid(st, st.Bible.GetChapter("Psalms", 150)) })
		want := "<p><i>A fixture title that runs on.</i></p>"
		if book {
			want = "<p><i>A fixture title that runs on.</i><br></p>"
		}
		if !strings.HasPrefix(html, want) {
			t.Errorf("the Psalm title must open the chapter as %q (book page %v), the first paragraph "+
				"isRaggedParagraph takes for the title; the page came out as:\n%s", want, book, html)
		}
	}

	// The swap, on the justified page only, before the text reaches the view,
	// and in a heading's or the title's paragraph only. Each line is read as
	// it stands: the same pass with its test turned round sets every verse's
	// spaces as gaps, and the justified page then justifies no verse at all.
	swaps := func(src string) bool {
		set := javaBlockAfter(t, src, "public static void setHtml(")
		keep := javaBlockAfter(t, src, "private static void keepHeadingsRagged(")
		ragged := javaBlockAfter(t, src, "private static boolean isRaggedParagraph(")
		title := javaBlockAfter(t, src, "private static boolean isTitleParagraph(")
		style := javaBlockAfter(t, src, "private static boolean justifiesInterWord()")
		return hasLinesInOrder(set,
			"if (s instanceof android.text.SpannableStringBuilder && justifiesInterWord()) {",
			"keepHeadingsRagged((android.text.SpannableStringBuilder) s);",
			"text.setText(s, TextView.BufferType.SPANNABLE);") &&
			hasLinesInOrder(keep,
				"if (isRaggedParagraph(ssb, ps, i)) {",
				"for (int j = ps; j < i; j++) {",
				"if (ssb.charAt(j) == ' ') ssb.replace(j, j + 1, String.valueOf(HEADING_GAP));") &&
			hasLinesInOrder(ragged,
				"return isHeadingParagraph(sp, ps, pe) || (ps == 0 && isTitleParagraph(sp, ps, pe));") &&
			hasLinesInOrder(title,
				"if (st.getStyle() == android.graphics.Typeface.ITALIC",
				"&& sp.getSpanStart(st) <= ps && sp.getSpanEnd(st) >= e) return true;") &&
			hasLinesInOrder(style, "return android.os.Build.VERSION.SDK_INT >= 35;") &&
			strings.Contains(src, "text.setJustificationMode(justifiesInterWord()")
	}
	if !swaps(java) {
		t.Error("setHtml must swap the spaces of a heading and of the title for HEADING_GAP " +
			"(keepHeadingsRagged, isRaggedParagraph) before setText, on the page setStyle " +
			"justifies — one predicate, justifiesInterWord, for both")
	}

	// And the reader gets the spaces back, from the paragraphs the swap set
	// them in: the app's own verbs, and Copy, which the view hands to
	// copyAsRead for every copy and which takes it over when readerText
	// changes anything. Each line is read as it stands, so a test turned
	// round, or a Copy line that never reaches copyAsRead's answer, fails
	// here and not only in a control.
	givesBack := func(src string) bool {
		read := javaBlockAfter(t, src, "readerText(CharSequence cs, int s0, int s1)")
		copyAs := javaBlockAfter(t, src, "private static boolean copyAsRead()")
		menu := javaBlockAfter(t, src, "@Override public boolean onTextContextMenuItem(int id)")
		return hasLinesInOrder(read,
			"boolean ragged = isRaggedParagraph(sp, p, pe);",
			"if (ragged && c == HEADING_GAP) c = ' ';") &&
			inSequence(copyAs, "readerText(", "setPrimaryClip(") &&
			slices.Equal(codeLines(menu), []string{
				"@Override public boolean onTextContextMenuItem(int id) {",
				"if (id == android.R.id.copy && copyAsRead()) return true;",
				"return super.onTextContextMenuItem(id);",
				"}",
			}) &&
			hasLinesInOrder(src, "final String sel = readerText(text.getText(), s0, s1);") &&
			!strings.Contains(src, "getText().subSequence(s0, s1).toString()")
	}
	if !givesBack(java) {
		t.Error("a selection's text must come through readerText — the app's verbs (sel) and " +
			"Copy (onTextContextMenuItem -> copyAsRead) — with the gaps of exactly the paragraphs " +
			"the swap set them in turned back, or a heading or a title leaves the page with " +
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
		for _, l := range codeLines(copyAs) {
			if strings.Contains(l, "return ") {
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

	// The controls: each check fails a bridge that lost the piece it holds,
	// on its own and not by the control failing to find its line.
	for _, c := range []struct {
		name, from, to string
		check          func(string) bool
	}{
		{"no swap in setHtml", "keepHeadingsRagged((android.text.SpannableStringBuilder) s);", "", swaps},
		{"swapped below API 35", "&& justifiesInterWord()) {", "&& !justifiesInterWord()) {", swaps},
		{"justified on its own predicate", "text.setJustificationMode(justifiesInterWord()",
			"text.setJustificationMode(android.os.Build.VERSION.SDK_INT >= 35", swaps},
		{"every paragraph but a heading or the title", "if (isRaggedParagraph(ssb, ps, i)) {",
			"if (!isRaggedParagraph(ssb, ps, i)) {", swaps},
		{"the title left to justify", " || (ps == 0 && isTitleParagraph(sp, ps, pe));", ";", swaps},
		{"any paragraph wholly in italic", "(ps == 0 && isTitleParagraph(sp, ps, pe))",
			"isTitleParagraph(sp, ps, pe)", swaps},
		{"readerText's test turned round", "boolean ragged = isRaggedParagraph(sp, p, pe);",
			"boolean ragged = !isRaggedParagraph(sp, p, pe);", givesBack},
		{"readerText for headings alone", "boolean ragged = isRaggedParagraph(sp, p, pe);",
			"boolean ragged = isHeadingParagraph(sp, p, pe);", givesBack},
		{"raw selection text", "final String sel = readerText(text.getText(), s0, s1);",
			"final String sel = text.getText().subSequence(s0, s1).toString();", givesBack},
		{"the platform's Copy", "if (id == android.R.id.copy && copyAsRead()) return true;", "", givesBack},
		{"a Copy never taken over", "copyAsRead()) return true;", "copyAsRead() && false) return true;", givesBack},
		{"the Copy test turned round", "if (id == android.R.id.copy && copyAsRead()) return true;",
			"if (id != android.R.id.copy && copyAsRead()) return true;", givesBack},
		{"false once copied", "selectionMode.finish();\n        return true;",
			"selectionMode.finish();\n        return false;", copies},
		{"false before copying", "private static boolean copyAsRead() {",
			"private static boolean copyAsRead() {\n        if (true) return false;", copies},
		{"the comparison turned round", "if (read.contentEquals(cs.subSequence(s0, s1))) return false;",
			"if (!read.contentEquals(cs.subSequence(s0, s1))) return false;", copies},
	} {
		if k := strings.Count(java, c.from); k != 1 {
			t.Fatalf("control %q: the bridge contains %q %d times, not once", c.name, c.from, k)
		}
		if c.check(strings.Replace(java, c.from, c.to, 1)) {
			t.Errorf("control %q: the check passes a bridge without it, so it proves nothing", c.name)
		}
	}
}

// THE DOCUMENTS SAY WHAT THE ANDROID BRIDGE DOES WITH THE PSALM TITLE.
// docs/READING_TYPOGRAPHY.md and docs/BACKLOG.md said Android from API 35
// justifies a Psalm title, which the bridge sets ragged. The claim is read
// against the bridge, so the documents move when it does.
func TestAndroidDocsSayWhatTheBridgeDoesWithTheTitle(t *testing.T) {
	java := readNativeSource(t, "android/BtBridge.java")
	if !strings.Contains(java, "|| (ps == 0 && isTitleParagraph(sp, ps, pe))") {
		t.Fatal("the bridge no longer sets the title ragged; the documents checked here say it " +
			"does, so they and this test change with it")
	}
	flat := func(path string) string { return strings.Join(strings.Fields(readRepoFile(t, path)), " ") }
	// The title.
	for _, c := range []struct{ path, stale string }{
		{"docs/READING_TYPOGRAPHY.md", "the rows of a wrapped poem line too, and a Psalm title that wraps"},
		{"docs/READING_TYPOGRAPHY.md", "Android from API 35 justifies it"},
		{"docs/BACKLOG.md", "Android from API 35 justifies a Psalm title"},
	} {
		if strings.Contains(flat(c.path), c.stale) {
			t.Errorf("%s still says %q; the bridge sets the Psalm title ragged (isRaggedParagraph)", c.path, c.stale)
		}
	}
	for _, path := range []string{"docs/READING_TYPOGRAPHY.md", "docs/ANDROID.md", "docs/BACKLOG.md"} {
		if !strings.Contains(flat(path), "isRaggedParagraph") {
			t.Errorf("%s does not name isRaggedParagraph, the one predicate for the paragraphs "+
				"Android sets ragged on its justified page", path)
		}
	}
}
