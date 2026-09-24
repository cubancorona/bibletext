package bibletext

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// The Apple stylesheet carries the reading page's numbers (reading_page.go)
// rather than its own: the body at the size the canvas pane and the web set it
// at, unrounded; the pitch; the numeral, gap-mark and footnote sizes; and the
// footnote air in ems, so it follows the reader's text size.
func TestAppleStylesheetCarriesTheReadingPage(t *testing.T) {
	st := tintState("Romans", 8, "web", proseChapter())
	for _, tc := range []struct {
		book  bool
		pitch float64
	}{{false, readingPhonePitchEm}, {true, readingBookPitchEm}} {
		var html string
		withReporterLayout(tc.book, func() { html = buildChapterHTML(st, proseChapter()) })

		if want := fmt.Sprintf("font-size: %.2fpx;", readingGlyphPx()); !strings.Contains(html, want) {
			t.Errorf("book=%v: the body is not set at the unrounded reading size %q", tc.book, want)
		}
		if strings.Contains(html, fmt.Sprintf("font-size: %dpx;", int(readingGlyphPx()+0.5))) {
			t.Errorf("book=%v: the body is rounded to a whole pixel, 0.8%% off the other surfaces at Normal", tc.book)
		}
		i := strings.Index(html, "body {")
		body := html[i : i+strings.Index(html[i:], "}")]
		if want := fmt.Sprintf("line-height: %g;", tc.pitch); !strings.Contains(body, want) {
			t.Errorf("book=%v: the body's line-height is not the spec's pitch %q:\n%s", tc.book, want, body)
		}
		if !strings.Contains(html, "font-size: "+emCSS(readingNumeralEm)+";") {
			t.Errorf("book=%v: the verse number's size is not the spec's", tc.book)
		}
	}

	// The footnote section.
	verses := footnoteFixtureVerses()
	fst := footnoteFixtureState(verses)
	setFootnotesEnabled(true)
	defer setFootnotesEnabled(false)
	fhtml := buildChapterHTML(fst, verses)
	for _, rule := range []string{"p.fnsep {", "p.fn {"} {
		i := strings.Index(fhtml, rule)
		if i < 0 {
			t.Fatalf("the footnote stylesheet has no %s rule", rule)
		}
		body := fhtml[i:]
		body = body[:strings.Index(body, "}")]
		if regexp.MustCompile(`margin:[^;]*px`).MatchString(body) {
			t.Errorf("%s sets its air in px, which does not follow the reader's text size:\n%s", rule, body)
		}
		if !strings.Contains(body, "font-size: "+emCSS(readingFootnoteEm)+";") {
			t.Errorf("%s is not at the spec's footnote size", rule)
		}
	}
}
