package main

// The chapter page is the app's reading page (reading_page.go), in CSS: the
// column is the measure with the side minimum each side, the book page is on
// whenever the column is the measure wide, and the sizes inside the page are
// the spec's. The numbers are written out here rather than read back from the
// placeholders' sources, so a changed constant cannot agree with itself.

import (
	"strings"
	"testing"

	bibletext "github.com/cubancorona/bibletext"
)

// cssRule is the body of the first rule that opens with sel.
func cssRule(t *testing.T, css, sel string) string {
	t.Helper()
	i := strings.Index(css, sel)
	if i < 0 {
		t.Fatalf("the stylesheet has no %q rule", sel)
	}
	return css[i : i+strings.Index(css[i:], "}")]
}

func TestTheWebPageIsTheSpecsPage(t *testing.T) {
	css := testCSS()
	if strings.Contains(css, "__") {
		i := strings.Index(css, "__")
		t.Fatalf("a placeholder was left unfilled: …%s…", css[max(0, i-40):min(len(css), i+40)])
	}

	// The column: 577.5px of measure (27.5 × the 21px reference) and 15px a
	// side, so the page tops out at 607.5px and switches exactly where the
	// app panes do.
	page := cssRule(t, css, ".wrap.page{")
	for _, want := range []string{"max-width:37.96875rem;", "padding-left:0.9375rem;", "padding-right:0.9375rem;", "container:page / inline-size"} {
		if !strings.Contains(page, want) {
			t.Errorf("the chapter's column does not say %q: %s", want, page)
		}
	}
	// The other pages keep the plain column.
	if !strings.Contains(css, ".wrap{max-width:40rem;") {
		t.Error("the plain .wrap lost its 40rem; only chapter pages take the reading page")
	}

	// The switch is the column's width, not the viewport's, and not the old
	// 46rem tablet guess.
	book := cssRule(t, css, "@container page (min-width:36.09375rem){")
	if !strings.Contains(book, ".text{--pgap:0rem; line-height:1.2222") {
		t.Errorf("the book page does not close the paragraph gap at the spec's pitch: %s", book)
	}
	if strings.Contains(css, "46rem") {
		t.Error("the old 46rem switch is still in the stylesheet")
	}

	// Every prose paragraph is indented on the book page, the first included,
	// as every app pane indents it; a poem-opening paragraph is not.
	i := strings.Index(css, "@container page (")
	block := css[i : i+strings.Index(css[i:], "\n}\n")]
	if !strings.Contains(block, ".text p{margin:0; text-indent:"+bibletext.EmCSS(bibletext.ReadingReporterIndentEm())+"}") {
		t.Errorf("the book page's paragraphs are not indented: %s", block)
	}
	if !strings.Contains(block, ".text p.pm{text-indent:0}") {
		t.Errorf("a poem-opening paragraph is indented: %s", block)
	}
	for _, bad := range []string{"p:first-child", "p.pst + p", "p.pm + p", ".55rem"} {
		if strings.Contains(block, bad) {
			t.Errorf("the book page still carries the web's own paragraph rule %q", bad)
		}
	}

	// The verse number: .66 of the body raised a third of it, which is
	// .5051 of the numeral's own size.
	n := cssRule(t, css, ".n{")
	for _, want := range []string{"font-size:0.66em;", "vertical-align:0.5051em;", "font-weight:600;"} {
		if !strings.Contains(n, want) {
			t.Errorf("the verse number does not say %q: %s", want, n)
		}
	}
	if vg := cssRule(t, css, ".vg{"); !strings.Contains(vg, "font-size:0.66em;") || !strings.Contains(vg, "color:var(--muted)") {
		t.Errorf("the omitted-verse mark is not the number's size in the muted ink: %s", vg)
	}

	// The footnotes: the scripture face at .85 of the set body, the page's
	// pitch, a fifth of the body between entries, the key bold.
	notes := cssRule(t, css, ".notes{")
	for _, want := range []string{"font-family:var(--scripture);", "font-size:1.2850rem;", "line-height:1.2222;", "text-align:justify;"} {
		if !strings.Contains(notes, want) {
			t.Errorf("the footnote section does not say %q: %s", want, notes)
		}
	}
	if dl := cssRule(t, css, ".notes dl{"); !strings.Contains(dl, "gap:0.3023rem .7rem") {
		t.Errorf("the entries do not stand a fifth of the body apart: %s", dl)
	}
	if dt := cssRule(t, css, ".notes dt{"); !strings.Contains(dt, "font-weight:700") || strings.Contains(dt, "opacity") {
		t.Errorf("the footnote key is not the bold cut in the muted ink: %s", dt)
	}
	if h2 := cssRule(t, css, ".notes h2{"); !strings.Contains(h2, "font-family:var(--ui);") || !strings.Contains(h2, "font-size:.7396rem;") {
		t.Errorf("the Notes label left its interface face and size: %s", h2)
	}

	// The note card's text is the app's furniture: 15 and 11, not following
	// the text size.
	for sel, want := range map[string]string{
		".notetext{":   "font-size:0.9375rem;",
		".notewho{":    "font-size:0.6875rem; font-weight:600;",
		".notechip{":   "font-size:0.6875rem; font-weight:600;",
		".notenotice{": "font-size:0.9375rem;",
	} {
		if r := cssRule(t, css, sel); !strings.Contains(r, want) {
			t.Errorf("%s does not say %q: %s", sel, want, r)
		}
	}
}

// The chapter page carries the page class, and an omitted verse's hole is
// marked where the apps mark it: after the join, before the next verse's
// number, outside that verse's span, with a body-size space after it.
func TestTheChapterPageMarksAnOmittedVerse(t *testing.T) {
	verses := []bibletext.Verse{
		{BookName: "Matthew", Book: "Matthew", Chapter: 17, Verse: 20, Text: "Nothing will be impossible for you."},
		{BookName: "Matthew", Book: "Matthew", Chapter: 17, Verse: 22, Text: "While they were staying in Galilee."},
	}
	bd := &bibletext.BibleData{Books: []string{"Matthew"}, Verses: map[string]map[int][]bibletext.Verse{"Matthew": {17: verses}}}
	body := chapterBody(bd, "bsb", "Matthew", 17, verses)
	if !strings.Contains(body, `</span></span> <span class="vg">[21]</span> <span class="v" id="v22">`) {
		t.Errorf("the hole at verse 21 is not marked before verse 22:\n%s", body)
	}
	// The control: the WEB keeps verse 21, so it marks nothing.
	if other := chapterBody(bd, "web", "Matthew", 17, verses); strings.Contains(other, `class="vg"`) {
		t.Errorf("an edition without the omission marks one:\n%s", other)
	}
	// A mark is washed with the verse after it, as the app panes wash it: the
	// page's script lights the marks standing before a lit verse, the
	// stylesheet paints them, and clearing puts them back.
	for _, want := range []string{"n.classList.contains('vg')", "n.classList.add('hlmark')", "spaces.forEach(wrapGap)", "el.classList.remove('hlmark')"} {
		if !strings.Contains(readerJSTemplate, want) {
			t.Errorf("the page's script does not say %q", want)
		}
	}
	if !strings.Contains(testCSS(), ".v:target,.v.hl,.hlgap,.vg.hlmark{background:var(--verse-hl);") {
		t.Error("the stylesheet does not paint a lit range through a mark")
	}
	page := renderChapter(loadedVersion{webVersion: webVersion{ID: "bsb", Name: "Berean Standard Bible"}, bible: bd},
		nil, "Matthew", "matthew", 17, 16, 18)
	if !strings.Contains(page, `<div class="wrap page">`) {
		t.Error("the chapter page does not take the reading page's column")
	}
}
