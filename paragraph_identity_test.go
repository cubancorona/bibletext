package bibletext

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// PARAGRAPHS ARE THE SHARED FACT, AND NOTHING CHECKED IT.
//
// Every note band is reserved above a PARAGRAPH, and every surface's arrival
// scroll asks, in its own dialect, "does this note's paragraph hold the verse I
// am going to?" That question is only answerable in one place — Go — while all
// five bodies really do break paragraphs where groupVersesIntoParagraphs breaks
// them. Four call sites do (reading.go, android_chapter_html.go,
// reading_styled_layout.go, web_api.go), and until now that was a convention.
//
// A heading, a footnote line, a runtime re-wrap or one builder's own <br> rule
// shifts a body's paragraph boundaries away from the model, and then a band
// reserved above paragraph 3 sits above a different paragraph 3. Nothing fails;
// the reader simply lands somewhere the note is not. So the map from verse to
// paragraph index is extracted from each RENDERED BODY and held against the
// model, on real chapters, with no device and no cgo.
func paragraphIndexByVerse(paras [][]Verse) map[int]int {
	out := map[int]int{}
	for i, para := range paras {
		for _, v := range para {
			out[v.Verse] = i
		}
	}
	return out
}

// htmlParagraphIndexByVerse reads a generated chapter body and reports which
// <p> each verse number landed in. Verse numbers are the <sup> markers both
// HTML dialects emit; a <br> inside a <p> is a poetic line break and NOT a
// paragraph, which is exactly the distinction that can drift.
// The two dialects nest the number differently — the Apple body writes it
// bare, Android wraps it in <small><font><b> — so the marker is matched as a
// <sup>…</sup> SPAN and the first digit run inside it is the verse.
var supRe = regexp.MustCompile(`(?s)<sup[^>]*>(.*?)</sup>`)
var digitsRe = regexp.MustCompile(`\d+`)

// tagRe strips the nested markup before digits are looked for. Without it the
// first digit run inside an Android marker is the "53688" of its colour, which
// parses cleanly, matches no verse, and makes the comparison silently empty —
// the exact "passes by comparing nothing" failure this test guards against
// elsewhere.
var tagRe = regexp.MustCompile(`<[^>]*>`)

func htmlParagraphIndexByVerse(t *testing.T, html string) map[int]int {
	t.Helper()
	out := map[int]int{}
	body := html
	if i := strings.Index(body, "<body>"); i >= 0 {
		body = body[i:]
	}
	for pi, chunk := range strings.Split(body, "<p")[1:] {
		end := strings.Index(chunk, "</p>")
		if end < 0 {
			end = len(chunk)
		}
		for _, m := range supRe.FindAllStringSubmatch(chunk[:end], -1) {
			d := digitsRe.FindString(tagRe.ReplaceAllString(m[1], ""))
			if d == "" {
				continue
			}
			n, err := strconv.Atoi(d)
			if err != nil {
				continue
			}
			if _, seen := out[n]; !seen {
				out[n] = pi
			}
		}
	}
	return out
}

// The paragraph indices must MATCH AS A PARTITION, not as raw numbers: a body
// that emits a superscription paragraph ahead of verse 1 shifts every index by
// one without moving a single boundary. What has to agree is which verses share
// a paragraph with which.
func sameParagraphPartition(model, body map[int]int) (string, bool) {
	// Only verses the body actually rendered can be compared.
	var shared []int
	for v := range body {
		if _, ok := model[v]; ok {
			shared = append(shared, v)
		}
	}
	if len(shared) < 2 {
		return fmt.Sprintf("only %d verses in common — nothing is being compared", len(shared)), false
	}
	for _, a := range shared {
		for _, b := range shared {
			if a >= b {
				continue
			}
			if (model[a] == model[b]) != (body[a] == body[b]) {
				return fmt.Sprintf("verses %d and %d: the model says %s, the body says %s",
					a, b,
					map[bool]string{true: "one paragraph", false: "two"}[model[a] == model[b]],
					map[bool]string{true: "one paragraph", false: "two"}[body[a] == body[b]]), false
			}
		}
	}
	return "", true
}

func paragraphFixtureChapters(t *testing.T) []struct {
	name   string
	verses []Verse
} {
	t.Helper()
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	var out []struct {
		name   string
		verses []Verse
	}
	for _, bc := range []struct {
		book string
		ch   int
	}{{"John", 3}, {"John", 11}, {"Psalms", 23}, {"Matthew", 1}, {"Ruth", 1}} {
		if vs := bd.GetChapter(bc.book, bc.ch); len(vs) >= 2 {
			out = append(out, struct {
				name   string
				verses []Verse
			}{fmt.Sprintf("%s %d", bc.book, bc.ch), vs})
		}
	}
	// A synthetic long chapter guarantees several real paragraph breaks even if
	// the sample corpus is thin — without it this test can pass on chapters
	// that have exactly one paragraph, where any two partitions agree.
	out = append(out, struct {
		name   string
		verses []Verse
	}{"synthetic multi-paragraph", enumerationChapter()})

	// A POETIC chapter, because the two HTML dialects break poetry with <br>
	// INSIDE a paragraph while the model keeps those verses in one paragraph.
	// That is the exact distinction a naive parser gets wrong, and the reason
	// this test counts <p> rather than lines.
	poem := make([]Verse, 0, 16)
	for i := 1; i <= 16; i++ {
		txt := "A poetic line that ends without a full stop and runs on,"
		if i%4 == 0 {
			txt = "and here the sentence closes so the splitter may break. " +
				"This tail makes the paragraph long enough to reach the threshold."
		}
		poem = append(poem, Verse{BookName: "Psalms", Book: "Psalms", Chapter: 119, Verse: i, Text: txt})
	}
	out = append(out, struct {
		name   string
		verses []Verse
	}{"synthetic poetic", poem})
	return out
}

func TestEverySurfaceBreaksParagraphsWhereTheModelDoes(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := planTestState(t)
	chapters := paragraphFixtureChapters(t)
	if len(chapters) < 2 {
		t.Fatalf("only %d fixture chapters", len(chapters))
	}

	multi := 0
	for _, ch := range chapters {
		model := paragraphIndexByVerse(groupVersesIntoParagraphs(ch.verses))
		paras := len(groupVersesIntoParagraphs(ch.verses))
		if paras > 1 {
			multi++
		}
		t.Run(ch.name, func(t *testing.T) {
			for _, surface := range []struct {
				name string
				html func() string
			}{
				{"apple", func() string { return buildChapterHTML(st, ch.verses) }},
				{"android", func() string { return buildChapterHTMLAndroid(st, ch.verses) }},
			} {
				got := htmlParagraphIndexByVerse(t, surface.html())
				if why, ok := sameParagraphPartition(model, got); !ok {
					t.Errorf("%s body: %s.\nA note's band is reserved above a PARAGRAPH, so a "+
						"body that breaks them elsewhere puts the band above a different "+
						"passage than the one the model reserved it for.", surface.name, why)
				}
			}

			// The styled pane lays out rather than emitting HTML, so its
			// paragraph identity is read off the layout's own line spans.
			pane := newStyledReadingPane(st, ch.verses)
			pane.Resize(fyne.NewSize(360, 900))
			if lay := pane.lay; lay != nil && len(lay.Lines) > 0 {
				// ParaFirst is the layout's own paragraph boundary, so counting
				// it forward gives each line its paragraph index; the verses
				// come off the runs, where provenance lives.
				styled, para := map[int]int{}, -1
				for _, ln := range lay.Lines {
					if ln.ParaFirst {
						para++
					}
					for _, r := range ln.Runs {
						if r.Verse > 0 {
							if _, seen := styled[r.Verse]; !seen {
								styled[r.Verse] = para
							}
						}
					}
				}
				if len(styled) >= 2 {
					if why, ok := sameParagraphPartition(model, styled); !ok {
						t.Errorf("styled layout: %s", why)
					}
				}
			}

			// And the web reader consumes the exported wrapper, so its identity
			// is that the wrapper IS the model — stated, because a future
			// divergence there would be invisible from the generated page.
			web := paragraphIndexByVerse(GroupVersesIntoParagraphs(ch.verses))
			if why, ok := sameParagraphPartition(model, web); !ok {
				t.Errorf("web reader: %s", why)
			}
		})
	}
	if multi < 2 {
		t.Errorf("only %d fixture chapters have more than one paragraph; on a "+
			"single-paragraph chapter every partition agrees and this test "+
			"proves nothing", multi)
	}
}
