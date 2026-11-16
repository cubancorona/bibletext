//go:build !race

package bibletext

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/test"
)

func TestCrossRefExcerptKeepsClosingQuotesWithTheirWords(t *testing.T) {
	appThemeForTest(t)
	t.Cleanup(func() { setReadingTextSizeID("normal") })
	cases := []struct {
		name string
		runs []cardRun
	}{
		{"nested", []cardRun{{Text: "A synthetic sentence ends here!’ ”"}}},
		{"styled", []cardRun{{Text: "A synthetic sentence ends "}, {Text: "here!", Italic: true}, {Text: "’ "}, {Text: "”", Red: true}}},
		{"separate-space", []cardRun{{Text: "A synthetic sentence ends here!’"}, {Text: " "}, {Text: "”"}}},
		{"closing-chain", []cardRun{{Text: "A synthetic sentence ends here! ’ ”"}}},
	}
	for _, size := range []string{"normal", "large", "xl"} {
		setReadingTextSizeID(size)
		for _, tc := range cases {
			t.Run(fmt.Sprintf("%s/%s", size, tc.name), func(t *testing.T) {
				p := passageIn([]fyne.CanvasObject{newCrossRefPassage(&AppState{Bible: NewBibleData(), CurrentVersion: "web"}, "Genesis", 1, nil, nil, lightPalette)})
				p.runs = tc.runs
				r := test.WidgetRenderer(p).(*readingParagraphRenderer)
				r.Layout(fyne.NewSize(1000, 600))
				if len(r.rows) != 1 {
					t.Fatal("wide synthetic fixture must fit on one row")
				}
				row := r.rows[0]
				quote := row[len(row)-1]
				if quote.Text != "”" {
					t.Fatal("fixture must end with a separate closing quote")
				}
				// The last word fits this width, but its closing quote does not.
				// Both must move together instead of orphaning or overflowing.
				threshold := quote.Position().X + quote.Size().Width/2
				for _, width := range []float32{threshold, 216, 260, 283} {
					r.Layout(fyne.NewSize(width, 600))
					var drawn strings.Builder
					for ri, line := range r.rows {
						for i, tx := range line {
							if (tx.Text == "”" || tx.Text == "’") && i == 0 {
								t.Errorf("closing quote stranded on row %d at width %.2f", ri, width)
							}
							if tx.Position().X+tx.Size().Width > width+0.01 {
								t.Errorf("fragment %q overflows width %.2f", tx.Text, width)
							}
							drawn.WriteString(tx.Text)
						}
					}
					if drawn.String() != strings.Join(strings.Fields(runsText(tc.runs)), "") {
						t.Fatal("wrapping changed the synthetic text")
					}
				}
				if out := os.Getenv("BIBLETEXT_SELECTION_EXCERPT_RENDER_DIR"); out != "" && size == "normal" && tc.name == "nested" {
					if err := os.MkdirAll(out, 0o755); err != nil {
						t.Fatal(err)
					}
					for _, hold := range []bool{true, false} {
						preview := passageIn([]fyne.CanvasObject{newCrossRefPassage(&AppState{Bible: NewBibleData(), CurrentVersion: "web"}, "Genesis", 1, nil, nil, lightPalette)})
						preview.runs, preview.holdFlush = tc.runs, hold
						name := "quote-before"
						if hold {
							name = "quote-after"
						}
						w := test.NewWindow(container.NewStack(canvas.NewRectangle(lightPalette.Background), container.New(layout.NewCustomPaddedLayout(20, 20, 30, 30), container.NewVBox(preview))))
						w.Resize(fyne.NewSize(328, 160))
						w.Canvas().Capture()
						writeRenderPNG(t, filepath.Join(out, name+".png"), w.Canvas().Capture())
						w.Close()
					}
				}
				// Ordinary cards keep their existing layout; only the study
				// excerpt opts into keeping these fragment chains together.
				p.holdFlush = false
				r.width = 0
				r.Layout(fyne.NewSize(threshold, 600))
				if len(r.rows) < 2 || r.rows[len(r.rows)-1][0].Text != "”" {
					t.Fatal("ordinary-card control no longer exercises the previous wrapping")
				}
			})
		}
	}
	p := newReadingParagraph([]cardRun{{Text: "Synthetic line."}, {Text: "”", Break: true}}, 17, lightPalette.Text, lightPalette.RedLetter, styledPaneFont(), cardItalicFont())
	p.holdFlush = true
	r := test.WidgetRenderer(p).(*readingParagraphRenderer)
	r.Layout(fyne.NewSize(283, 100))
	if len(r.rows) != 2 || r.rows[1][0].Text != "”" {
		t.Fatal("explicit source line break was removed")
	}
}
