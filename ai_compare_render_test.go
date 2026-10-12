package bibletext

import (
	"fmt"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/rivo/uniseg"
)

type compareDrawnText struct {
	text  string
	pos   fyne.Position
	size  fyne.Size
	style fyne.TextStyle
}

type compareRenderedFixture struct {
	answer *compareAnswer
	window fyne.Window
	scroll *container.Scroll
	leaves []*widget.RichText
	texts  []compareDrawnText
}

// Gather the actual wrapped text, including inline runs and their positions.
// Segment contents alone cannot detect a wrapped bullet losing its indentation
// or emphasized words in a quotation being placed on a separate line.
func collectCompareRendered(answer *compareAnswer) compareRenderedFixture {
	fixture := compareRenderedFixture{answer: answer}
	seen := make(map[fyne.CanvasObject]bool)
	var visit func(fyne.CanvasObject, fyne.Position)
	visit = func(object fyne.CanvasObject, parent fyne.Position) {
		if object == nil || !object.Visible() || seen[object] {
			return
		}
		seen[object] = true
		position := parent.Add(object.Position())
		if text, ok := object.(*canvas.Text); ok && strings.TrimSpace(text.Text) != "" {
			fixture.texts = append(fixture.texts, compareDrawnText{text.Text, position, text.Size(), text.TextStyle})
		}
		if rich, ok := object.(*widget.RichText); ok {
			fixture.leaves = append(fixture.leaves, rich)
		}
		switch object := object.(type) {
		case *fyne.Container:
			for _, child := range object.Objects {
				visit(child, position)
			}
		case fyne.Widget:
			for _, child := range test.WidgetRenderer(object).Objects() {
				visit(child, position)
			}
		}
	}
	visit(answer, fyne.NewPos(0, 0))
	sort.SliceStable(fixture.texts, func(i, j int) bool {
		a, b := fixture.texts[i], fixture.texts[j]
		if a.pos.Y != b.pos.Y {
			return a.pos.Y < b.pos.Y
		}
		return a.pos.X < b.pos.X
	})
	return fixture
}

func renderCompareFixture(t *testing.T, raw, name string, width float32) compareRenderedFixture {
	t.Helper()
	answer := newCompareAnswer()
	window := test.NewWindow(nil)
	t.Cleanup(window.Close)
	window.Resize(fyne.NewSize(width, 420))
	scroll := container.NewVScroll(answer)
	window.SetContent(container.NewStack(canvas.NewRectangle(lightPalette.Surface),
		container.NewThemeOverride(scroll, &bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()})))
	answer.setText(raw)
	scroll.Refresh()
	window.Canvas().Capture()
	if dir := os.Getenv("BIBLETEXT_PANE_SNAPSHOT_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		file, err := os.OpenFile(filepath.Join(dir, fmt.Sprintf("compare-%s-%.0f.png", name, width)), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(file, window.Canvas().Capture())
		closeErr := file.Close()
		if err != nil {
			t.Fatal(err)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
	}
	fixture := collectCompareRendered(answer)
	fixture.window, fixture.scroll = window, scroll
	return fixture
}

func (fixture compareRenderedFixture) content() string {
	var text []string
	for _, leaf := range fixture.leaves {
		text = append(text, segmentText(leaf.Segments))
	}
	return strings.Join(text, "\n")
}

func (fixture compareRenderedFixture) find(t *testing.T, content string) compareDrawnText {
	t.Helper()
	for _, text := range fixture.texts {
		if strings.Contains(text.text, content) {
			return text
		}
	}
	t.Fatalf("rendered comparison lost %q: %+v", content, fixture.texts)
	return compareDrawnText{}
}

func TestCompareRenderSeparatesHeadingsAndBlocks(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	const raw = "## First reading\n\n- First point.\n- Second point.\n\n## Second reading\n\nA paragraph follows.\n\nAnother paragraph follows."
	for _, width := range []float32{310, 530} {
		fixture := renderCompareFixture(t, raw, "block-spacing", width)
		first := fixture.find(t, "First point.")
		second := fixture.find(t, "Second point.")
		heading := fixture.find(t, "Second reading")
		paragraph := fixture.find(t, "A paragraph follows.")
		last := fixture.find(t, "Another paragraph follows.")
		itemGap := second.pos.Y - first.pos.Y - first.size.Height
		headingGap := heading.pos.Y - second.pos.Y - second.size.Height
		if headingGap < theme.Padding() || headingGap <= itemGap {
			t.Errorf("heading after a tight list has insufficient separation at %.0fpt: heading gap %.2f, item gap %.2f", width, headingGap, itemGap)
		}
		if !heading.style.Bold || paragraph.pos.Y < heading.pos.Y+heading.size.Height+theme.Padding() {
			t.Errorf("heading is not distinct from the following paragraph at %.0fpt", width)
		}
		if gap := last.pos.Y - paragraph.pos.Y - paragraph.size.Height; gap < theme.Padding() {
			t.Errorf("paragraph separation is only %.2fpt at %.0fpt width", gap, width)
		}
	}
}

func TestCompareRenderListWrapsWithHangingIndent(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	const prose = "The reading begins with the field and follows a long path past the river, describing careful attention to the surrounding words and the wider passage."
	for _, marker := range []string{"- ", "7. "} {
		for _, width := range []float32{310, 530} {
			fixture := renderCompareFixture(t, marker+prose, "list-wrap", width)
			first := fixture.find(t, "The reading")
			rows := make(map[float32]float32)
			var bullet compareDrawnText
			for _, text := range fixture.texts {
				if strings.TrimSpace(text.text) == "•" || strings.TrimSpace(text.text) == "7." {
					bullet = text
					continue
				}
				if x, exists := rows[text.pos.Y]; !exists || text.pos.X < x {
					rows[text.pos.Y] = text.pos.X
				}
			}
			if bullet.text == "" || bullet.pos.X >= first.pos.X || math.Abs(float64(bullet.pos.Y-first.pos.Y)) > 1 {
				t.Fatalf("list marker is not beside its first line at %.0fpt: marker %+v, first %+v", width, bullet, first)
			}
			if len(rows) < 2 {
				t.Fatal("list fixture must wrap to verify continuation indentation")
			}
			for y, x := range rows {
				if math.Abs(float64(x-first.pos.X)) > 0.5 {
					t.Errorf("wrapped list row at %.2fpt loses indentation: X %.2f, want %.2f", y, x, first.pos.X)
				}
			}
		}
	}
}

func TestCompareRenderQuotesKeepInlineRunsTogether(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	const raw = "An opening paragraph.\n\n> Quiet **strong** and *gentle* words.\n\nA closing paragraph."
	for _, width := range []float32{310, 530} {
		fixture := renderCompareFixture(t, raw, "quote-inline", width)
		opening := fixture.find(t, "An opening")
		quiet := fixture.find(t, "Quiet")
		strong := fixture.find(t, "strong")
		gentle := fixture.find(t, "gentle")
		if quiet.pos.X <= opening.pos.X {
			t.Error("the quotation lost its inset")
		}
		if !strong.style.Bold || !gentle.style.Italic {
			t.Error("quotation emphasis was lost")
		}
		if math.Abs(float64(quiet.pos.Y-strong.pos.Y)) > 1 || math.Abs(float64(quiet.pos.Y-gentle.pos.Y)) > 1 {
			t.Errorf("inline emphasis split a short quotation across lines at %.0fpt: %+v, %+v, %+v", width, quiet, strong, gentle)
		}
	}
}

func TestCompareRenderBreaksAndNestedEmphasis(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	for _, width := range []float32{310, 530} {
		fixture := renderCompareFixture(t, "Soft one\nsoft two.\n\nHard one  \nhard two.\n\nA ***combined*** emphasis.", "breaks-emphasis", width)
		soft := fixture.find(t, "Soft one")
		softTail := fixture.find(t, "soft two.")
		hard := fixture.find(t, "Hard one")
		hardTail := fixture.find(t, "hard two.")
		combined := fixture.find(t, "combined")
		if math.Abs(float64(soft.pos.Y-softTail.pos.Y)) > 1 {
			t.Error("a soft Markdown newline became a visible line break")
		}
		if hardTail.pos.Y < hard.pos.Y+hard.size.Height {
			t.Error("an explicit Markdown line break became a space")
		}
		if !combined.style.Bold || !combined.style.Italic {
			t.Error("nested emphasis lost bold or italic")
		}
	}
}

func TestCompareRenderDecodesEntitiesAndEscapesOnce(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	const raw = "An escaped entity \\&amp;, a numeric entity &#38;amp;, a letter &#065;, a symbol &copy;, and \\*literal\\* punctuation with `&amp;` code."
	const want = "An escaped entity &amp;, a numeric entity &amp;, a letter A, a symbol ©, and *literal* punctuation with &amp; code."
	for _, width := range []float32{310, 530} {
		fixture := renderCompareFixture(t, raw, "entity-decoding", width)
		if got := fixture.content(); got != want {
			t.Errorf("entity and punctuation decoding changed the intended text:\n got: %q\nwant: %q", got, want)
		}
		literalCode := false
		for _, leaf := range fixture.leaves {
			for _, segment := range leaf.Segments {
				if text, ok := segment.(*widget.TextSegment); ok && text.Style.TextStyle.Monospace && text.Text == "&amp;" {
					literalCode = true
				}
			}
		}
		if !literalCode {
			t.Error("the code span lost its literal entity spelling or monospace style")
		}
	}
}

func TestCompareRenderHardBreakAtEmphasisBoundary(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	for _, lineBreak := range []struct {
		name string
		text string
	}{
		{"spaces", "  \n"},
		{"backslash", "\\\n"},
	} {
		for _, width := range []float32{310, 530} {
			fixture := renderCompareFixture(t, "**Firm**"+lineBreak.text+"*gentle*.", "emphasis-break-"+lineBreak.name, width)
			first := fixture.find(t, "Firm")
			second := fixture.find(t, "gentle")
			if !first.style.Bold || !second.style.Italic {
				t.Error("emphasis around a line break lost its style")
			}
			if second.pos.Y < first.pos.Y+first.size.Height {
				t.Errorf("a %s Markdown hard break at an emphasis boundary collapsed onto one line", lineBreak.name)
			}
		}
	}
}

func TestCompareRenderOrderedListAlignsAcrossDoubleDigits(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	const raw = "9. Ninth point follows a long path through the field and beside the river, retaining its continuation indentation.\n10. Tenth point follows another long path through the surrounding words and the wider passage."
	for _, width := range []float32{310, 530} {
		fixture := renderCompareFixture(t, raw, "ordered-double-digits", width)
		ninth := fixture.find(t, "Ninth point")
		tenth := fixture.find(t, "Tenth point")
		if math.Abs(float64(ninth.pos.X-tenth.pos.X)) > 0.5 {
			t.Error("ordered items lost their shared text column at the transition to double digits")
		}
		for _, marker := range []string{"9.", "10."} {
			count := 0
			for _, text := range fixture.texts {
				if strings.TrimSpace(text.text) == marker {
					count++
					if text.pos.X >= ninth.pos.X {
						t.Errorf("ordered marker %q overlaps the text column", marker)
					}
				}
			}
			if count != 1 {
				t.Errorf("ordered list rendered marker %q %d times, want once", marker, count)
			}
		}
		rowStarts := make(map[float32]float32)
		for _, text := range fixture.texts {
			if value := strings.TrimSpace(text.text); value == "9." || value == "10." {
				continue
			}
			if x, exists := rowStarts[text.pos.Y]; !exists || text.pos.X < x {
				rowStarts[text.pos.Y] = text.pos.X
			}
		}
		if len(rowStarts) < 3 {
			t.Fatal("ordered fixture must include a continuation row")
		}
		for _, x := range rowStarts {
			if math.Abs(float64(x-ninth.pos.X)) > 0.5 {
				t.Error("ordered continuation lost the shared hanging indentation")
			}
		}
	}
}

func TestCompareRenderPreservesComplexListContents(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	const raw = "- Outer start.\n\n  Outer continuation.\n\n  - First nested point.\n  - Another nested point.\n\n  Between nested lists.\n\n  1. Later nested step.\n  2. Final nested step.\n\n- Outer finish."
	for _, width := range []float32{310, 530} {
		fixture := renderCompareFixture(t, raw, "complex-list", width)
		for _, want := range []string{"Outer start.", "Outer continuation.", "First nested point.", "Another nested point.", "Between nested lists.", "Later nested step.", "Final nested step.", "Outer finish."} {
			if count := strings.Count(fixture.content(), want); count != 1 {
				t.Errorf("complex list renders %q %d times, want once", want, count)
			}
		}
		outer := fixture.find(t, "Outer start.")
		for _, want := range []string{"First nested", "Later nested"} {
			if nested := fixture.find(t, want); nested.pos.X <= outer.pos.X {
				t.Errorf("nested list %q lost its deeper indentation", want)
			}
		}
	}
}

func TestCompareRenderCannotCreateNetworkSegments(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	for i, raw := range append([]string{hostile}, bypasses...) {
		fixture := renderCompareFixture(t, raw, fmt.Sprintf("safety-%02d", i), 310)
		for _, leaf := range fixture.leaves {
			if images, links := networkSegments(leaf.Segments); images+links != 0 {
				t.Errorf("comparison rendered %d images and %d links for safety fixture %d", images, links, i)
			}
		}
	}
}

func TestCompareRenderBoundsLongUnicodeSegments(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	prose := strings.TrimSpace(strings.Repeat("A synthetic café with e\u0301 and 漢字 👩🏽‍💻 follows the path. ", 45))
	for _, width := range []float32{310, 530} {
		fixture := renderCompareFixture(t, prose, "long-unicode", width)
		var joined strings.Builder
		segments := 0
		for _, leaf := range fixture.leaves {
			for _, segment := range leaf.Segments {
				text, ok := segment.(*widget.TextSegment)
				if !ok {
					continue
				}
				segments++
				if !utf8.ValidString(text.Text) || utf8.RuneCountInString(text.Text) > aiAnswerSegmentRunes {
					t.Errorf("a long comparison segment exceeds the Unicode layout budget: %d runes", utf8.RuneCountInString(text.Text))
				}
				joined.WriteString(text.Text)
			}
		}
		if segments < 2 || joined.String() != prose {
			t.Fatal("long comparison splitting lost or altered the answer")
		}
		boundaries := map[int]bool{0: true}
		graphemes := uniseg.NewGraphemes(prose)
		for graphemes.Next() {
			_, end := graphemes.Positions()
			boundaries[end] = true
		}
		end := 0
		for _, leaf := range fixture.leaves {
			for _, segment := range leaf.Segments {
				if text, ok := segment.(*widget.TextSegment); ok {
					end += len(text.Text)
					if !boundaries[end] {
						t.Error("comparison splitting cut a grapheme cluster")
					}
				}
			}
		}
	}
}

func TestCompareRenderRefitsWithoutLosingReadingContent(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	raw := "## A synthetic heading\n\n" + strings.Repeat("The passage follows a path past the field and the river. ", 80) + "\n\nFinal comparison words."
	fixture := renderCompareFixture(t, raw, "refit", 310)
	content := fixture.content()
	narrowHeight := fixture.answer.MinSize().Height
	fixture.window.Resize(fyne.NewSize(530, 420))
	fixture.scroll.Refresh()
	fixture.window.Canvas().Capture()
	wide := collectCompareRendered(fixture.answer)
	if wide.content() != content || fixture.answer.MinSize().Height >= narrowHeight {
		t.Fatal("wider comparison layout failed to rewrap the complete answer")
	}
	fixture.window.Resize(fyne.NewSize(310, 420))
	fixture.scroll.Refresh()
	fixture.window.Canvas().Capture()
	if got := collectCompareRendered(fixture.answer).content(); got != content {
		t.Error("returning to a narrow viewport changed the answer")
	}
	fixture.scroll.ScrollToBottom()
	last := collectCompareRendered(fixture.answer).find(t, "Final comparison words.")
	if last.pos.Y+last.size.Height-fixture.scroll.Offset.Y > fixture.scroll.Size().Height+0.5 {
		t.Error("the final comparison words are outside the scroll's reach")
	}
}
