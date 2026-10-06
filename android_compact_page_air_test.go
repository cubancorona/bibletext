package bibletext

// ANDROID'S BOOK PAGE KEEPS THE AIR THE PHONE PAGE KEEPS, under a Psalm title
// and under a heading with a closed note below it.
//
// The book page (the compact page: a phone held sideways, a tablet) is
// imported COMPACT, with no blank line between blocks, where the phone page
// separates every block with one (BtBridge.setHtml). Two things rested on a
// blank line there that the book page did not have:
//
//   - The Psalm title's gap. The dialect wrote one <br> into the title's block
//     to make the blank line the import leaves out, and the import swallowed
//     it, so the title sat on verse 1 with no gap at all, where every other
//     page leaves the title gap (reading_spacing.go).
//   - A closed note's pill under a heading. The pill centres in the air the
//     page put above its paragraph (notePillSeparatorLift); on the phone page
//     the bridge measures it across the blank separator line, and on the book
//     page, where the heading's tail is reserved in the next line's ascent
//     instead, it found no separator and stood down, 7px higher in the gap
//     than on the phone page.
//
// The import is the platform's, behind JNI, so the first is held by a model
// of the import's newline rule, itself proved against what the emulator
// showed; the second by the bridge's source, line by line, with controls.

import (
	"html"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// androidImportedText is the text Html.fromHtml makes of the Android dialect,
// as far as its line breaks go: the newlines HtmlToSpannedConverter writes
// for <p> and <br>, with every other tag dropped and the text kept. The rule
// it models is AOSP's appendNewlines: a block's start and its end each add
// newlines only until the text ends in as many as the mode asks for — two in
// LEGACY, one in COMPACT, which sets every FROM_HTML_SEPARATOR_LINE_BREAK
// flag — counting the ones already there, and none at the very start; a
// <br> appends one. Whitespace is not collapsed: the dialect writes none at
// a block's edge.
func androidImportedText(markup string, compact bool) string {
	margin := 2
	if compact {
		margin = 1
	}
	var b strings.Builder
	appendNewlines := func(min int) {
		s := b.String()
		if s == "" {
			return
		}
		n := 0
		for i := len(s) - 1; i >= 0 && s[i] == '\n'; i-- {
			n++
		}
		for ; n < min; n++ {
			b.WriteByte('\n')
		}
	}
	for _, m := range regexp.MustCompile(`<(/?)([a-zA-Z][a-zA-Z0-9]*)[^>]*>|[^<]+`).FindAllStringSubmatch(markup, -1) {
		switch tag := strings.ToLower(m[2]); {
		case tag == "":
			b.WriteString(html.UnescapeString(m[0]))
		case tag == "p":
			appendNewlines(margin)
		case tag == "br" && m[1] == "":
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// The model, against the two pages as the API 35 emulator drew them before
// this change: the phone page left a blank line under the title, and the
// book page put verse 1 straight under it.
func TestTheAndroidImportModelMatchesTheEmulator(t *testing.T) {
	for _, c := range []struct {
		name    string
		markup  string
		compact bool
		want    string
	}{
		{"the phone page: a blank line between blocks", "<p><i>T</i></p><p>V</p>", false, "T\n\nV\n\n"},
		{"the book page: none", "<p>A</p><p>B</p>", true, "A\nB\n"},
		{"the book page's old title: its one <br> swallowed", "<p><i>T</i><br></p><p>V</p>", true, "T\nV\n"},
		{"a <br> inside a block is a line of its own", "<p>a<br>b</p>", true, "a\nb\n"},
		{"nothing before the first block", "<p>A</p>", false, "A\n\n"},
		{"entities read back", "<p>a&#160;b</p>", true, "a\u00a0b\n"},
	} {
		if got := androidImportedText(c.markup, c.compact); got != c.want {
			t.Errorf("%s: %q imports as %q, want %q", c.name, c.markup, got, c.want)
		}
	}
}

// THE TITLE KEEPS ITS GAP ON BOTH PAGES. The text the import makes is the
// title, a blank line and verse 1 on the phone page and on the book page
// alike, so the bridge sets the same title gap on that line on both
// (applyParagraphAir: the blank line after the title paragraph, read before
// the book page's own branch). The book page stays compact below the title:
// no other blank line between its blocks.
func TestAndroidPsalmTitleKeepsItsGapOnTheBookPage(t *testing.T) {
	st := psalm23State()
	st.Bible.Superscriptions = map[string]map[int]Superscription{"Psalms": {23: {Text: "A Psalm by David."}}}
	st.Bible.Verses["Psalms"][23][1].ParaStart = true
	lines := func(book bool) []string {
		var markup string
		withReporterLayout(book, func() { markup = buildChapterHTMLAndroid(st, st.Bible.GetChapter("Psalms", 23)) })
		// The text's own closing newlines aside.
		return strings.Split(strings.TrimRight(androidImportedText(markup, book), "\n"), "\n")
	}
	pages := map[bool]string{false: "phone", true: "book"}
	for _, book := range []bool{false, true} {
		l := lines(book)
		if len(l) < 3 || l[0] != "A Psalm by David." || l[1] != "" || !strings.Contains(l[2], "The LORD is my shepherd;") {
			t.Errorf("on the %s page the title must stand over a blank line, the line the bridge sets to the "+
				"title gap, and then verse 1; the import's lines open:\n%q", pages[book], l[:min(len(l), 4)])
		}
	}
	// The book page is still compact under the title: its two paragraphs
	// meet with no blank line, and the phone page's do not.
	for _, book := range []bool{false, true} {
		blank := 0
		l := lines(book)
		for i := 2; i < len(l); i++ {
			if l[i] == "" {
				blank++
			}
		}
		if want := map[bool]int{false: 1, true: 0}[book]; blank != want {
			t.Errorf("the %s page has %d blank lines between its verse paragraphs, want %d:\n%q", pages[book], blank, want, l)
		}
	}

	// The bridge gives the line after the title the title gap on both pages:
	// the blank-line branch runs before the book page's own.
	air := javaBlockAfter(t, readNativeSource(t, "android/BtBridge.java"), "private static void applyParagraphAir(")
	if !hasLinesInOrder(air,
		"if (pr[1] == pr[0]) {",
		"boolean titlePrev = k == 1 && isTitleParagraph(ssb, paras.get(0)[0], paras.get(0)[1]);",
		"int h = headNext ? lead : (headPrev ? tail : (titlePrev ? titleGap : gap));",
		"if (!compact) continue; // the blank lines carry the air on this page") {
		t.Error("applyParagraphAir must set the blank line after the title to the title gap, before the " +
			"book page's own branch, so both pages take it")
	}
}

// THE PILL UNDER A HEADING CENTRES ON THE BOOK PAGE. btPillStackInkTop reads
// the air above the noted paragraph off the span that reserves it when there
// is no separator line, and centres between the heading's last line and the
// noted line, exactly as it centres across a separator on the phone page.
// The book page's plain paragraphs, which meet with no air, and a poem's
// lines still stand it down.
func TestAndroidPillCentresUnderAHeadingOnTheBookPage(t *testing.T) {
	java := readNativeSource(t, "android/BtBridge.java")
	holds := func(src string) bool {
		ink := javaBlockAfter(t, src, "private static int btPillStackInkTop(")
		ascent := javaBlockAfter(t, src, "private static boolean airInAscentAt(int ps)")
		return slices.Equal(codeLines(ink), []string{
			"private static int btPillStackInkTop(Layout lay, NoteBandSpan span, int ps,",
			"int stackH, int capH) {",
			"if (lay == null || span == null || !span.below || ps <= 0) return -1;",
			"int sl = lay.getLineForOffset(ps - 1);",
			"int above;",
			"if (lay.getLineStart(sl) == ps - 1) {",
			"if (sl < 1) return -1;                      // nothing above the blank line",
			"above = sl - 1;                             // the line above the separator",
			"} else if (airInAscentAt(ps)) {",
			"above = sl;                                 // the compact page: the air is the noted line's",
			"} else {",
			"return -1;                                  // an ink line: mid-poem, or no air on the compact page",
			"}",
			"float inkBottom = lay.getLineBaseline(above);",
			"float inkTop = lay.getLineBaseline(lay.getLineForOffset(ps)) - capH;",
			"float gap = inkTop - inkBottom;",
			"if (gap <= stackH) return -1;",
			"return Math.round(inkBottom + (gap - stackH) / 2f);",
			"}",
		}) && slices.Equal(codeLines(ascent), []string{
			"private static boolean airInAscentAt(int ps) {",
			"for (AirSpan a : airSpans) {",
			"if (!a.exact && a.at == ps && a.px > 0) return true;",
			"}",
			"return false;",
			"}",
		})
	}
	// And the heading's tail is the air it finds: the book page reserves it
	// in the ascent of the first character of the paragraph after the
	// heading, the paragraph a note under the heading hangs its band above.
	tail := func(src string) bool {
		air := javaBlockAfter(t, src, "private static void applyParagraphAir(")
		return hasLinesInOrder(air,
			"if (!compact) continue; // the blank lines carry the air on this page",
			"if (!isHeadingParagraph(ssb, pr[0], pr[1])) continue;",
			"int q = paras.get(k + 1)[0];",
			"AirSpan a = new AirSpan(tail, false, q, add);",
			"airSpans.add(a);")
	}
	if !holds(java) {
		t.Error("btPillStackInkTop must centre across the compact page's ascent air (airInAscentAt) where " +
			"there is no separator line, and stand down where there is neither")
	}
	if !tail(java) {
		t.Error("the compact page must reserve a heading's tail in the ascent of the next paragraph's first " +
			"character, recorded in airSpans, which is the air the pill centres in")
	}
	for _, c := range []struct {
		name, from, to string
		check          func(string) bool
	}{
		{"the compact page stands down again", "} else if (airInAscentAt(ps)) {", "} else if (false) {", holds},
		{"the air read from the line above the heading", "above = sl;      ", "above = sl - 1;  ", holds},
		{"a separator's exact height taken for ascent air", "if (!a.exact && a.at == ps && a.px > 0) return true;",
			"if (a.at == ps && a.px > 0) return true;", holds},
		{"the air on any character", "if (!a.exact && a.at == ps && a.px > 0) return true;",
			"if (!a.exact && a.px > 0) return true;", holds},
		{"the tail never recorded", "AirSpan a = new AirSpan(tail, false, q, add);\n                ssb.setSpan(a, q, Math.min(q + 1, n), android.text.Spanned.SPAN_EXCLUSIVE_EXCLUSIVE);\n                airSpans.add(a);",
			"AirSpan a = new AirSpan(tail, false, q, add);\n                ssb.setSpan(a, q, Math.min(q + 1, n), android.text.Spanned.SPAN_EXCLUSIVE_EXCLUSIVE);", tail},
	} {
		if k := strings.Count(java, c.from); k != 1 {
			t.Fatalf("control %q: the bridge contains %q %d times, not once", c.name, c.from, k)
		}
		if c.check(strings.Replace(java, c.from, c.to, 1)) {
			t.Errorf("control %q: the check passes a bridge without it, so it proves nothing", c.name)
		}
	}
}
