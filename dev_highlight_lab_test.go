//go:build bibletextdev

package bibletext

import (
	"image/color"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
)

func resetDevHighlightLabForTest(t *testing.T) {
	t.Helper()
	originalColors := devHighlightColors
	originalActive := devHighlightActive
	devHighlightColors = [devHighlightCaseCount]color.NRGBA{
		lightPalette.Highlight,
		lightPalette.Highlight,
		darkPalette.Highlight,
		darkPalette.Highlight,
	}
	devHighlightActive = 0
	t.Cleanup(func() {
		devHighlightColors = originalColors
		devHighlightActive = originalActive
	})
}

func testNRGBA(t *testing.T, c color.Color) color.NRGBA {
	t.Helper()
	got, ok := color.NRGBAModel.Convert(c).(color.NRGBA)
	if !ok {
		t.Fatalf("converted %T is not color.NRGBA", c)
	}
	return got
}

func TestDevHighlightCasesCoverFourIndependentReadingModes(t *testing.T) {
	resetDevHighlightLabForTest(t)
	want := []struct {
		label        string
		summaryLabel string
		pal          palette
		red          bool
	}{
		{"Day / regular text", "Day / regular", lightPalette, false},
		{"Day / red text", "Day / red", lightPalette, true},
		{"Night / regular text", "Night / regular", darkPalette, false},
		{"Night / red text", "Night / red", darkPalette, true},
	}
	if len(devHighlightCases) != len(want) {
		t.Fatalf("cases = %d, want %d", len(devHighlightCases), len(want))
	}
	for i, w := range want {
		got := devHighlightCases[i]
		if got.label != w.label || got.summaryLabel != w.summaryLabel || got.pal != w.pal || got.red != w.red {
			t.Errorf("case %d = {%q red=%v}, want {%q red=%v}", i, got.label, got.red, w.label, w.red)
		}
		if devHighlightColors[i] != w.pal.Highlight {
			t.Errorf("case %q default = %s, want palette highlight %s",
				w.label, devHighlightHex(devHighlightColors[i]), devHighlightHex(w.pal.Highlight))
		}
	}

	// Equal defaults for the two day cases must not alias their later choices.
	devHighlightColors[0].R = 1
	if devHighlightColors[1] != lightPalette.Highlight {
		t.Error("changing Day / regular also changed Day / red")
	}
}

func TestDevHighlightSlidersUpdatePreviewAndCodesImmediately(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	resetDevHighlightLabForTest(t)

	lab := newDevHighlightLab(&AppState{})
	dayRegularBefore := devHighlightColors[0]
	nightRegularBefore := devHighlightColors[2]
	lab.selectCase(1)
	lab.sliders[0].SetValue(11)
	lab.sliders[1].SetValue(22)
	lab.sliders[2].SetValue(33)

	want := color.NRGBA{R: 11, G: 22, B: 33, A: 255}
	if got := devHighlightColors[1]; got != want {
		t.Fatalf("live colour = %#v, want %#v", got, want)
	}
	if got := testNRGBA(t, lab.previews[1].wash.FillColor); got != want {
		t.Errorf("preview wash = %#v, want %#v", got, want)
	}
	if got := lab.codes[1].Text; got != "#0B1621" {
		t.Errorf("case code = %q, want #0B1621", got)
	}
	if got := lab.editorCode.Text; got != "#0B1621" {
		t.Errorf("editor code = %q, want #0B1621", got)
	}
	if got := lab.summary.Text; !strings.Contains(got, "Day / red       #0B1621") {
		t.Errorf("summary did not update synchronously:\n%s", got)
	}
	if devHighlightColors[0] != dayRegularBefore || devHighlightColors[2] != nightRegularBefore {
		t.Error("editing Day / red changed another case")
	}
	if got := strings.Count(lab.summary.Text, "\n"); got != devHighlightCaseCount-1 {
		t.Errorf("summary has %d line breaks, want %d", got, devHighlightCaseCount-1)
	}
}

func TestDevHighlightPreviewsMatchReadingTypographyAndColours(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	resetDevHighlightLabForTest(t)

	fontName := styledPaneFont().Name()
	textSize := devHighlightPreviewTextSize()
	for i, c := range devHighlightCases {
		preview := newDevHighlightPreview(c, devHighlightColors[i])
		wantText := c.pal.Text
		if c.red {
			wantText = c.pal.RedLetter
		}

		if got := testNRGBA(t, preview.ground.FillColor); got != c.pal.Background {
			t.Errorf("%s ground = %#v, want reading ground %#v", c.label, got, c.pal.Background)
		}
		if got := testNRGBA(t, preview.wash.FillColor); got != devHighlightColors[i] {
			t.Errorf("%s wash = %#v, want %#v", c.label, got, devHighlightColors[i])
		}
		for name, text := range map[string]*canvas.Text{
			"unhighlighted prefix":  preview.prefix,
			"highlighted selection": preview.selected,
			"unhighlighted suffix":  preview.suffix,
		} {
			if text.Text == "" {
				t.Errorf("%s %s is empty", c.label, name)
			}
			if text.FontSource == nil || text.FontSource.Name() != fontName {
				t.Errorf("%s %s font = %v, want reading font %q", c.label, name, text.FontSource, fontName)
			}
			if text.TextSize != textSize {
				t.Errorf("%s %s size = %.1f, want reading size %.1f", c.label, name, text.TextSize, textSize)
			}
			if got := testNRGBA(t, text.Color); got != wantText {
				t.Errorf("%s %s colour = %#v, want %#v", c.label, name, got, wantText)
			}
			if text.TextStyle.Bold {
				t.Errorf("%s %s became bold under the wash", c.label, name)
			}
		}
		if preview.wash.CornerRadius != 2 {
			t.Errorf("%s wash radius = %.1f, want native-reading 2px", c.label, preview.wash.CornerRadius)
		}
	}
}

func TestDevTabContainsScreenshotReadyHighlightLab(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	resetDevHighlightLabForTest(t)

	tab := buildDevLinksTab(&AppState{CurrentVersion: defaultVersionID}, nil)
	for _, want := range []string{
		"Highlight colour lab",
		"Day / regular text",
		"Day / red text",
		"Night / regular text",
		"Night / red text",
		"Chosen highlight colours (RGB hex)",
	} {
		if !treeHasText(tab, want) {
			t.Errorf("dev tab is missing %q", want)
		}
	}

	// Exercise the widest scripture setting against less than a 320pt phone's
	// width, allowing for the real tab's outer padding. Checking leaf MinSizes
	// catches non-wrapping content; checking only the resized parent would pass
	// even while a child overflowed invisibly.
	originalTextSize := readingTextSizeID()
	t.Cleanup(func() { setReadingTextSizeID(originalTextSize) })
	setReadingTextSizeID("xl")
	lab := newDevHighlightLab(&AppState{})
	const phoneContentWidth = float32(300)
	lab.root.Resize(fyne.NewSize(phoneContentWidth, lab.root.MinSize().Height))
	for i, preview := range lab.previews {
		if got := preview.root.MinSize().Width; got > phoneContentWidth {
			t.Errorf("preview %d MinSize width = %.1f, exceeds %.1f phone content", i, got, phoneContentWidth)
		}
	}
	if got := lab.summary.MinSize().Width; got > phoneContentWidth {
		t.Errorf("summary MinSize width = %.1f, exceeds %.1f phone content", got, phoneContentWidth)
	}
	if lab.editor.Visible() {
		t.Error("RGB editor starts visible; the default panel is not screenshot-ready")
	}
	lab.buttons[1].OnTapped()
	if !lab.editor.Visible() || lab.editorSample == nil {
		t.Error("Adjust did not reveal the live editor specimen and controls")
	}
	hide := findTreeButton(lab.editor, "Hide controls for screenshot")
	if hide == nil {
		t.Fatal("live editor has no screenshot-collapse control")
	}
	hide.OnTapped()
	if lab.editor.Visible() {
		t.Error("screenshot-collapse control left the RGB editor visible")
	}
}
