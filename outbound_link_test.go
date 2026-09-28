package bibletext

// THE "LEAVES THE APP" ARROW IS DRAWN IN THE LINK'S OWN COLOUR.
//
// "Get a key ↗" and "Privacy Policy ↗" drew their arrow as a colour-emoji
// tile on every platform, because the only face Fyne consults that has U+2197
// is its bundled colour-emoji font (outboundLink explains the order). These
// look at the pixels, which is where the defect was: every pixel a link
// paints must be a blend of the ground it sits on and the link colour — the
// words are, and an emoji tile, with its own blue and its white arrow, is not.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"image"
	"image/color"
	"math"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
)

// offLinkColour measures how far c is from the line between ground and link
// in RGB, in 0-255 units, and how far along that line it lies (0 = ground,
// 1 = link). A pixel of antialiased link-coloured ink is close to the line
// wherever along it it falls.
func offLinkColour(c, ground, link color.Color) (off, along float64) {
	f := func(c color.Color) [3]float64 {
		n := color.NRGBAModel.Convert(c).(color.NRGBA)
		return [3]float64{float64(n.R), float64(n.G), float64(n.B)}
	}
	p, a, b := f(c), f(ground), f(link)
	var ab, ap [3]float64
	var abab, apab float64
	for i := range p {
		ab[i], ap[i] = b[i]-a[i], p[i]-a[i]
		abab += ab[i] * ab[i]
		apab += ap[i] * ab[i]
	}
	t := apab / abab
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	var d float64
	for i := range p {
		q := a[i] + t*ab[i] - p[i]
		d += q * q
	}
	return math.Sqrt(d), t
}

// linkInk checks every pixel of r in img against the ground→link line and
// returns how many lie off it, and how many strongly inked pixels (at least
// 60% of the way to the link colour) lie at or right of arrowX.
func linkInk(img image.Image, r image.Rectangle, ground, link color.Color, arrowX int) (off, arrowInk int, worst color.Color) {
	const tolerance = 24 // antialiasing and rounding; an emoji's white is ~200 off
	var worstOff float64
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			c := img.At(x, y)
			d, along := offLinkColour(c, ground, link)
			if d > tolerance {
				off++
				if d > worstOff {
					worstOff, worst = d, c
				}
			}
			if x >= arrowX && along >= 0.6 && d <= tolerance {
				arrowInk++
			}
		}
	}
	return off, arrowInk, worst
}

// dominant is the most common colour in r: the ground a link is drawn on.
func dominant(img image.Image, r image.Rectangle) color.Color {
	counts := map[color.NRGBA]int{}
	var best color.NRGBA
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			counts[c]++
			if counts[c] > counts[best] {
				best = c
			}
		}
	}
	return best
}

// arrowStartX is the pixel column where the arrow may begin in a link drawn at
// origin x0: one inner padding, then the words.
func arrowStartX(l *outboundLink, x0 float32, scale float32) int {
	th := l.Theme()
	words := fyne.MeasureText(l.Text, th.Size(theme.SizeNameText), l.TextStyle).Width
	return int((x0 + th.Size(theme.SizeNameInnerPadding) + words + 1) * scale)
}

func TestOutboundLinkArrowIsTheLinkColour(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Settings().SetTheme(&bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()})
	link := theme.Color(theme.ColorNameHyperlink)
	ground := theme.Color(theme.ColorNameBackground)
	const scale = 2

	for _, label := range []string{"Get a key", "Privacy Policy"} {
		t.Run(label, func(t *testing.T) {
			l := newOutboundLink(label, nil)
			w := test.NewWindow(nil)
			defer w.Close()
			w.SetPadded(false)
			w.Canvas().(test.WindowlessCanvas).SetScale(scale) // before the content, so the arrow rasterizes at 2x
			w.SetContent(l)
			w.Resize(l.MinSize())
			img := w.Canvas().Capture()

			off, arrow, worst := linkInk(img, img.Bounds(), ground, link, arrowStartX(l, 0, scale))
			if off > 0 {
				t.Errorf("%q paints %d pixels that are not the link colour on its ground (worst %v) — "+
					"the arrow is not a plain glyph in the link colour", label, off, worst)
			}
			// Absence would pass the check above; the arrow has to be there.
			if arrow < 30 {
				t.Errorf("%q has %d pixels of link-coloured ink right of its words; the arrow is missing", label, arrow)
			}
		})
	}
}

// The same, on the links the Settings sheet really builds — found by type, so
// a call site that went back to typing the character would fail here too.
func TestSettingsLinksDrawTheArrowInTheLinkColour(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	th := &bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()}
	app.Settings().SetTheme(th)
	t.Setenv("BIBLE_API_KEY", "")
	fake := withFakeSharedKeys(t)
	fake.setAIEnabled(true)
	const scale = 2
	win := app.NewWindow("Settings")
	win.Canvas().(test.WindowlessCanvas).SetScale(scale)
	win.Resize(fyne.NewSize(834, 2400)) // tall enough that nothing scrolls away
	st := sampleState()
	st.window = win
	st.theme = th
	st.aiKeys = fake
	popup := pickerPopup(t, st, showAISettings)
	test.WidgetRenderer(popup).Layout(popup.Size())

	var links []*outboundLink
	walkTree(popup, func(o fyne.CanvasObject) {
		if l, ok := o.(*outboundLink); ok {
			links = append(links, l)
		}
	})
	var labels []string
	for _, l := range links {
		labels = append(labels, l.Text)
	}
	// The assistant's key, API.Bible's key, the privacy policy.
	if got := strings.Join(labels, " | "); got != "Get a key | Privacy Policy | Get a key" {
		t.Fatalf("the sheet's outbound links are %q, want the assistant's Get a key, the Privacy Policy "+
			"and API.Bible's Get a key", got)
	}

	img := win.Canvas().Capture()
	link := theme.Color(theme.ColorNameHyperlink)
	drv := fyne.CurrentApp().Driver()
	for _, l := range links {
		p := drv.AbsolutePositionForObject(l)
		m := l.MinSize()
		r := image.Rect(int(p.X*scale), int(p.Y*scale), int((p.X+m.Width)*scale), int((p.Y+m.Height)*scale)).Intersect(img.Bounds())
		ground := dominant(img, r)
		off, arrow, worst := linkInk(img, r, ground, link, arrowStartX(l, p.X, scale))
		if off > 0 {
			t.Errorf("%q in Settings paints %d pixels off the link colour (worst %v)", l.Text, off, worst)
		}
		if arrow < 30 {
			t.Errorf("%q in Settings has %d pixels of arrow ink; the arrow is missing", l.Text, arrow)
		}
	}
}

// A tap on the arrow opens the link, as a tap on the character did.
func TestOutboundLinkArrowOpensTheLink(t *testing.T) {
	restore := externalOpener
	defer func() { externalOpener = restore }()
	var opened []string
	externalOpener = func(u *url.URL) error { opened = append(opened, u.String()); return nil }

	app := test.NewApp()
	defer app.Quit()
	app.Settings().SetTheme(&bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()})
	u, _ := url.Parse("https://example.invalid/key")
	l := newOutboundLink("Get a key", u)
	w := test.NewWindow(l)
	defer w.Close()
	w.Resize(l.MinSize().AddWidthHeight(20, 20))

	at, sz := l.arrowBox()
	test.TapAt(l, at.AddXY(sz.Width/2, sz.Height/2))
	if len(opened) != 1 || opened[0] != u.String() {
		t.Fatalf("a tap on the arrow opened %v, want the link target once", opened)
	}
	// And the words still open it, through the Hyperlink's own handling.
	th := l.Theme()
	inner := th.Size(theme.SizeNameInnerPadding)
	test.TapAt(l, fyne.NewPos(inner+4, inner+4))
	if len(opened) != 2 {
		t.Fatalf("a tap on the words opened %v, want the link target again", opened)
	}
}

// No string in the app's source spells U+2197 any more: drawn by Fyne it is
// the emoji tile, whatever follows it. Comments may name it; string literals
// may not. Parsed, not grepped, so a comment quoting the old label does not
// count and a literal hidden in a concatenation does.
func TestNoUIStringCarriesTheArrowCharacter(t *testing.T) {
	const arrow = "↗"
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	literalsWith := func(src string) (hits []string) {
		f, err := parser.ParseFile(fset, "x.go", src, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			// The value, not the source text: "\u2197" is the same string.
			if v, err := strconv.Unquote(lit.Value); err == nil && strings.Contains(v, arrow) {
				hits = append(hits, fset.Position(lit.Pos()).String()+" "+lit.Value)
			}
			return true
		})
		return hits
	}
	// CONTROL: the sweep finds the label as it used to be written, and as an
	// escape, and not a comment that quotes it.
	if len(literalsWith("package p\nvar s = \"Get a key "+arrow+"\"\n")) != 1 ||
		len(literalsWith("package p\nvar s = \"Get a key \\u2197\"\n")) != 1 ||
		len(literalsWith("package p\n// \"Get a key "+arrow+"\"\nvar s = 1\n")) != 0 {
		t.Fatal("the sweep cannot tell a string literal from a comment")
	}
	scanned := 0
	var offenders []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		scanned++
		for _, h := range literalsWith(string(src)) {
			offenders = append(offenders, strings.Replace(h, "x.go", name, 1))
		}
	}
	if scanned < 50 {
		t.Fatalf("scanned only %d files; the sweep is not reaching the package", scanned)
	}
	if len(offenders) > 0 {
		t.Errorf("U+2197 in a string, which Fyne draws as a colour emoji — use newOutboundLink:\n  %s",
			strings.Join(offenders, "\n  "))
	}
}
