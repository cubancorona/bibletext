//go:build !race

package bibletext

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
)

func TestCrossRefExcerptEmphasizesOnlyItsSourceMarkers(t *testing.T) {
	th := appThemeForTest(t)
	t.Cleanup(func() { setReadingTextSizeID("normal") })
	if readingMarkScale != styledNumRatio || readingMarkLift != styledNumLift {
		t.Fatal("ordinary card markers no longer match the reading page's scale and lift")
	}
	units := []passageUnit{
		{Number: 99, Runs: []verseRun{{Text: "“A synthetic passage” keeps its words and exact anchors."}}, Marks: []passageMark{{At: 0, Letter: "aa"}}},
		{Number: 100, Runs: []verseRun{{Text: "Another "}, {Text: "supplied", Italic: true}, {Text: " word is followed by a reference."}}, Marks: []passageMark{{At: 17, Letter: "ww"}}},
	}
	runs := passageRuns(units)
	wantText := "“A synthetic passage” keeps its words and exact anchors.\nAnother supplied word is followed by a reference."
	if got := passagePlainText(runs); got != wantText {
		t.Fatalf("fixture text changed: %q", got)
	}
	out := os.Getenv("BIBLETEXT_EXCERPT_RENDER_DIR")
	if out != "" {
		if err := os.MkdirAll(out, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, size := range []string{"normal", "large", "xl"} {
		for _, dark := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s-dark-%v", size, dark), func(t *testing.T) {
				setReadingTextSizeID(size)
				variant, pal := theme.VariantLight, lightPalette
				if dark {
					variant, pal = theme.VariantDark, darkPalette
				}
				fyne.CurrentApp().Settings().SetTheme(forcedVariant{Theme: th, v: variant})
				st := &AppState{Bible: NewBibleData(), CurrentVersion: "web"}
				makePassage := func() *readingParagraph {
					p := passageIn([]fyne.CanvasObject{newCrossRefPassage(st, "Psalms", 119, nil, nil, pal)})
					p.runs = runs
					return p
				}
				p := makePassage()
				sz := crossRefPanelSizes()
				if p.size != readingGlyphSize(sz.passage) || p.marks.size != sz.printed || p.number != pal.Accent || p.mark != pal.Accent {
					t.Fatal("excerpt changed Scripture size or does not use the citation size and matching accent for its markers")
				}
				if p.marks.number != crossRefPlainZeroBold() || p.marks.letter != th.uiFonts.boldItalic {
					t.Fatal("excerpt markers do not match the upright plain-zero and bold-italic group faces")
				}
				for _, width := range []float32{216, 260, 283} {
					p.Resize(fyne.NewSize(width, 600))
					r := test.WidgetRenderer(p).(*readingParagraphRenderer)
					r.Layout(p.Size())
					marks := 0
					for ri, row := range r.rows {
						for _, tx := range row {
							if tx.Position().X < -0.01 || tx.Position().X+tx.Size().Width > width+0.5 {
								t.Errorf("%q overflows width %.1f", tx.Text, width)
							}
							if tx.Color != pal.Accent {
								if tx.TextSize != p.size {
									t.Errorf("Scripture fragment %q changed size", tx.Text)
								}
								continue
							}
							marks++
							if tx.TextSize != sz.printed {
								t.Errorf("marker %q was shrunk: %.2f", tx.Text, tx.TextSize)
							}
							top := tx.Position().Y - float32(ri)*r.rowH
							if top < -0.5 || top+tx.Size().Height > r.rowH+0.5 {
								t.Errorf("marker %q lies outside row: top %.2f, height %.2f, row %.2f", tx.Text, top, tx.Size().Height, r.rowH)
							}
							_, base := fyne.CurrentApp().Driver().RenderedTextSize(tx.Text, tx.TextSize, fyne.TextStyle{}, tx.FontSource)
							if diff := float64(r.bodyBase - (top + base) - p.marks.lift); math.Abs(diff) > 0.01 {
								t.Errorf("marker %q has incorrect baseline lift", tx.Text)
							}
						}
						if len(row) > 0 && row[len(row)-1].Color == pal.Accent {
							t.Errorf("row %d strands its final marker from its word", ri)
						}
					}
					if marks != 4 || passagePlainText(p.runs) != wantText {
						t.Fatal("layout lost a marker or changed Scripture")
					}
				}
				ordinary := newReadingParagraph(runs, p.size, p.color, p.red, p.regular, p.italic)
				ordinary.bold = passageNumeralFace()
				ordinary.Resize(fyne.NewSize(260, 600))
				or := test.WidgetRenderer(ordinary).(*readingParagraphRenderer)
				or.Layout(ordinary.Size())
				for _, f := range ordinary.fragments() {
					if !f.mark {
						continue
					}
					face := ordinary.regular
					if f.dim {
						face = ordinary.bold
					}
					if ordinary.marks != nil || or.fragmentSize(f) != ordinary.size*readingMarkScale || or.fragmentFace(f) != face {
						t.Fatal("ordinary card markers changed")
					}
				}
				for tx, top := range or.lift {
					_, base := fyne.CurrentApp().Driver().RenderedTextSize(tx.Text, tx.TextSize, fyne.TextStyle{}, tx.FontSource)
					if diff := float64(or.bodyBase - (top + base) - ordinary.size*readingMarkLift); math.Abs(diff) > 0.01 {
						t.Fatal("ordinary card marker lift changed")
					}
				}
				if out != "" {
					for _, old := range []bool{true, false} {
						excerpt := makePassage()
						name := "after"
						if old {
							name = "before"
							excerpt.marks, excerpt.number = nil, pal.VerseNumber
						}
						excerpt.guess = 260
						key := canvas.NewText("99", pal.Accent)
						key.TextSize, key.FontSource = sz.reference, crossRefPlainZeroBold()
						body := container.New(layout.NewCustomPaddedVBoxLayout(12), excerpt, container.NewHBox(key, crossRefLetter("aa", pal)))
						w := test.NewWindow(container.NewStack(canvas.NewRectangle(pal.Background), container.New(layout.NewCustomPaddedLayout(16, 16, 30, 30), body)))
						w.Resize(fyne.NewSize(320, 290))
						writeRenderPNG(t, filepath.Join(out, fmt.Sprintf("excerpt-%s-%s-dark-%v.png", name, size, dark)), w.Canvas().Capture())
						w.Close()
					}
				}
			})
		}
	}
}

func TestCrossRefExcerptSeparatesAdjacentNumberAndLetter(t *testing.T) {
	appThemeForTest(t)
	setReadingTextSizeID("normal")
	t.Cleanup(func() { setReadingTextSizeID("normal") })
	makePassage := func(runs []cardRun) *readingParagraph {
		st := &AppState{Bible: NewBibleData(), CurrentVersion: "web"}
		p := passageIn([]fyne.CanvasObject{newCrossRefPassage(st, "Psalms", 119, nil, nil, lightPalette)})
		p.runs = runs
		return p
	}
	gapBetween := func(left, right *canvas.Text) float32 {
		return right.Position().X - left.Position().X - left.Size().Width
	}
	checkGap := func(t *testing.T, left, right *canvas.Text, want float32) {
		t.Helper()
		if got := gapBetween(left, right); math.Abs(float64(got-want)) > 0.01 {
			t.Errorf("gap %q to %q = %.2f, want %.2f", left.Text, right.Text, got, want)
		}
	}
	for _, tc := range []struct {
		name   string
		unit   passageUnit
		first  []string
		letter bool
	}{
		{"single", passageUnit{Number: 2, Runs: []verseRun{{Text: "Sample passage."}}, Marks: []passageMark{{At: 0, Letter: "a"}}}, []string{"2", "a", "Sample"}, true},
		{"wide", passageUnit{Number: 100, Runs: []verseRun{{Text: "Sample passage."}}, Marks: []passageMark{{At: 0, Letter: "aa"}}}, []string{"100", "aa", "Sample"}, true},
		{"multiple", passageUnit{Number: 2, Runs: []verseRun{{Text: "Sample passage."}}, Marks: []passageMark{{At: 0, Letter: "a"}, {At: 0, Letter: "b"}}}, []string{"2", "a", "b", "Sample"}, true},
		{"number-only", passageUnit{Number: 2, Runs: []verseRun{{Text: "Sample passage."}}}, []string{"2", "Sample"}, false},
		{"quote", passageUnit{Number: 2, Runs: []verseRun{{Text: "“Sample passage.”"}}, Marks: []passageMark{{At: 0, Letter: "a"}}}, []string{"2", "a", "“Sample"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runs := passageRuns([]passageUnit{tc.unit})
			p := makePassage(runs)
			p.Resize(fyne.NewSize(216, 200))
			r := test.WidgetRenderer(p).(*readingParagraphRenderer)
			r.Layout(p.Size())
			if len(r.rows) == 0 || len(r.rows[0]) < len(tc.first) {
				t.Fatal("number, letter and first word did not stay on one row")
			}
			row := r.rows[0]
			for i, want := range tc.first {
				if row[i].Text != want {
					t.Fatalf("fragment %d = %q, want %q", i, row[i].Text, want)
				}
				if i > 0 {
					gap := readingMarkGap
					if i == 1 && tc.letter {
						gap = p.marks.numberLetterGap
						if gap <= readingMarkGap {
							t.Fatal("verse-to-letter gap is not distinct from letter-to-word spacing")
						}
					}
					checkGap(t, row[i-1], row[i], gap)
				}
			}
			if got := passagePlainText(p.runs); got != tc.unit.text() {
				t.Fatalf("layout changed passage text: %q", got)
			}
			ordinary := newReadingParagraph(runs, p.size, p.color, p.red, p.regular, p.italic)
			ordinary.Resize(fyne.NewSize(216, 200))
			or := test.WidgetRenderer(ordinary).(*readingParagraphRenderer)
			or.Layout(ordinary.Size())
			for i := 1; i < len(tc.first); i++ {
				checkGap(t, or.rows[0][i-1], or.rows[0][i], readingMarkGap)
			}
		})
	}

	t.Run("explicit-break", func(t *testing.T) {
		p := makePassage([]cardRun{{Text: "2", Mark: true, Dim: true}, {Text: "a", Mark: true, Break: true}, {Text: "Sample"}})
		p.Resize(fyne.NewSize(216, 200))
		r := test.WidgetRenderer(p).(*readingParagraphRenderer)
		r.Layout(p.Size())
		if len(r.rows) != 2 || len(r.rows[1]) != 2 || r.rows[1][0].Text != "a" || r.rows[1][0].Position().X != 0 {
			t.Fatal("explicit break retained a gap from the preceding row's number")
		}
		checkGap(t, r.rows[1][0], r.rows[1][1], readingMarkGap)
	})

	for _, holdFlush := range []bool{false, true} {
		t.Run(fmt.Sprintf("lookahead-holdFlush-%v", holdFlush), func(t *testing.T) {
			p := makePassage([]cardRun{{Text: "Prefix "}, {Text: "2", Mark: true, Dim: true}, {Text: "a", Mark: true}, {Text: "word"}})
			p.holdFlush = holdFlush
			p.Resize(fyne.NewSize(500, 200))
			r := test.WidgetRenderer(p).(*readingParagraphRenderer)
			r.Layout(p.Size())
			if len(r.rows) != 1 || len(r.rows[0]) != 4 {
				t.Fatal("wide fixture should fit one row")
			}
			last := r.rows[0][3]
			// This width fits the old 1.5-point gap but not the new gap.
			// Lookahead must move the whole marker/word group to the next row.
			width := last.Position().X + last.Size().Width - (p.marks.numberLetterGap-readingMarkGap)/2
			p.Resize(fyne.NewSize(width, 200))
			r.Layout(p.Size())
			if len(r.rows) != 2 || len(r.rows[0]) != 1 || len(r.rows[1]) != 3 || r.rows[1][0].Text != "2" || r.rows[1][2].Text != "word" {
				t.Fatal("lookahead did not reserve the larger gap while keeping number, letter and word together")
			}
			checkGap(t, r.rows[1][0], r.rows[1][1], p.marks.numberLetterGap)
			for _, row := range r.rows {
				for _, tx := range row {
					if tx.Position().X+tx.Size().Width > width+0.01 {
						t.Errorf("fragment %q exceeds the narrow row", tx.Text)
					}
				}
			}
		})
	}

	if out := os.Getenv("BIBLETEXT_EXCERPT_GAP_RENDER_DIR"); out != "" {
		if err := os.MkdirAll(out, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, before := range []bool{true, false} {
			p := makePassage(passageRuns([]passageUnit{{Number: 2, Runs: []verseRun{{Text: "Sample passage for checking the reference markers."}}, Marks: []passageMark{{At: 0, Letter: "a"}}}}))
			name := "after"
			if before {
				name, p.marks.numberLetterGap = "before", readingMarkGap
			}
			p.guess = 260
			w := test.NewWindow(container.NewStack(canvas.NewRectangle(lightPalette.Background), container.New(layout.NewCustomPaddedLayout(16, 16, 30, 30), p)))
			w.Resize(fyne.NewSize(320, 130))
			writeRenderPNG(t, filepath.Join(out, "marker-gap-"+name+".png"), w.Canvas().Capture())
			w.Close()
		}
	}
}
