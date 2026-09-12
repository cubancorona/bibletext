package bibletext

import (
	"regexp"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
)

// THE APPLE STYLESHEET IS FORMATTED FROM THE CONSTANTS. It used to carry a
// fixed 24px paragraph gap and a fixed 14px title gap, which did not scale
// with the reader's text size and matched no other surface. Mutation: a px
// literal back in any of the four rules.
func TestApplePanesTakeTheirAirFromTheConstants(t *testing.T) {
	st := headedChapterState()
	st.Bible.Superscriptions = map[string]map[int]Superscription{"Matthew": {5: {Text: "A title, for the test."}}}
	var css string
	// The gapped (phone) page: the reporter page has no paragraph gap and
	// indents natively instead, so the paragraph rule is checked on this one.
	withReporterLayout(false, func() {
		css = buildChapterHTML(st, st.Bible.GetChapter("Matthew", 5))
	})
	css = css[:strings.Index(css, "<body>")]
	// A rule starts after the previous one's brace (or the stylesheet's
	// start): "p {" alone also matches inside "sup {".
	rule := func(sel string) string {
		re := regexp.MustCompile(`(?:^|})\s*` + regexp.QuoteMeta(sel) + ` \{[^}]*`)
		m := re.FindString(css)
		if m == "" {
			t.Fatalf("no %s rule", sel)
		}
		return m
	}
	if r := rule("p"); !strings.Contains(r, "margin: 0 0 "+emCSS(readingParaGapEm)+" 0;") {
		t.Errorf("the paragraph gap is not %s: %s", emCSS(readingParaGapEm), r)
	}
	if r := rule("p.sec"); !strings.Contains(r, "margin: 0 0 "+emCSS(readingHeadTailEm)+" 0;") {
		t.Errorf("the heading tail is not %s: %s", emCSS(readingHeadTailEm), r)
	}
	if r := rule("p.pre-sec"); !strings.Contains(r, "margin-bottom: "+emCSS(readingHeadLeadEm)+";") {
		t.Errorf("the heading lead is not %s: %s", emCSS(readingHeadLeadEm), r)
	}
	if r := rule("p.pst"); !strings.Contains(r, "margin: 0 0 "+emCSS(readingTitleGapEm)+" 0;") {
		t.Errorf("the title gap is not %s: %s", emCSS(readingTitleGapEm), r)
	}
	if strings.Contains(css, "px 0;") {
		t.Errorf("a fixed-pixel vertical margin is back in the stylesheet")
	}
}

// THE FYNE TITLE GAP IS THE CONSTANT, in ems of the size. Mutation: the gap
// figured from the line height again.
func TestStyledTitleGapIsTheConstant(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	meas := func(s string) float32 { return float32(len(s)) * 8 }
	g := measureStyledSuperscription("A Psalm of David.", 400, 20, 40, meas)
	if !g.present || len(g.lines) != 1 {
		t.Fatalf("fixture: %d title lines", len(g.lines))
	}
	if gap := g.height - 40; gap < float32(readingTitleGapEm)*20-0.01 || gap > float32(readingTitleGapEm)*20+0.01 {
		t.Errorf("the title gap is %.2f; want %g × 20 = %.2f", gap, readingTitleGapEm, readingTitleGapEm*20)
	}
}
