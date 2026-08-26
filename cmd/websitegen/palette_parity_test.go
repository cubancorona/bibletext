package main

import (
	"fmt"
	"image/color"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"

	bibletext "bibletext"
)

func TestWebReaderPaletteValues(t *testing.T) {
	wantLight := bibletext.WebReaderPalette{
		Background:       color.NRGBA{R: 237, G: 233, B: 224, A: 255},
		Surface:          color.NRGBA{R: 253, G: 252, B: 248, A: 255},
		Text:             color.NRGBA{R: 37, G: 34, B: 29, A: 255},
		TextMuted:        color.NRGBA{R: 107, G: 100, B: 86, A: 255},
		Accent:           color.NRGBA{R: 47, G: 76, B: 134, A: 255},
		Border:           color.NRGBA{R: 189, G: 178, B: 159, A: 255},
		VerseNumber:      color.NRGBA{R: 83, G: 104, B: 143, A: 255},
		RedLetter:        color.NRGBA{R: 178, G: 58, B: 46, A: 255},
		Highlight:        color.NRGBA{R: 255, G: 224, B: 138, A: 255},
		ControlHover:     color.NRGBA{R: 47, G: 76, B: 134, A: 28},
		ControlSelection: color.NRGBA{R: 47, G: 76, B: 134, A: 40},
	}
	wantDark := bibletext.WebReaderPalette{
		Background:       color.NRGBA{R: 25, G: 23, B: 21, A: 255},
		Surface:          color.NRGBA{R: 34, G: 31, B: 28, A: 255},
		Text:             color.NRGBA{R: 233, G: 227, B: 217, A: 255},
		TextMuted:        color.NRGBA{R: 157, G: 148, B: 135, A: 255},
		Accent:           color.NRGBA{R: 124, G: 160, B: 228, A: 255},
		Border:           color.NRGBA{R: 57, G: 52, B: 46, A: 255},
		VerseNumber:      color.NRGBA{R: 140, G: 168, B: 216, A: 255},
		RedLetter:        color.NRGBA{R: 229, G: 115, B: 115, A: 255},
		Highlight:        color.NRGBA{R: 58, G: 50, B: 111, A: 255},
		ControlHover:     color.NRGBA{R: 124, G: 160, B: 228, A: 28},
		ControlSelection: color.NRGBA{R: 124, G: 160, B: 228, A: 40},
	}

	gotLight, gotDark := bibletext.WebReaderPalettes()
	if gotLight != wantLight {
		t.Errorf("light web palette = %#v, want the app palette %#v", gotLight, wantLight)
	}
	if gotDark != wantDark {
		t.Errorf("dark web palette = %#v, want the app palette %#v", gotDark, wantDark)
	}
}

func TestReaderCSSUsesExportedWebPalettes(t *testing.T) {
	light, dark := bibletext.WebReaderPalettes()
	blocks := cssRootBlocks(t, readerCSS("regular.woff2", "bold.woff2"))
	if len(blocks) != 2 {
		t.Fatalf("reader CSS has %d :root blocks, want the light root and one dark override", len(blocks))
	}

	tests := []struct {
		name    string
		block   string
		palette bibletext.WebReaderPalette
	}{
		{name: "light", block: blocks[0], palette: light},
		{name: "dark", block: blocks[1], palette: dark},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			roles := []struct {
				property string
				want     color.NRGBA
			}{
				{property: "--bg", want: tc.palette.Background},
				{property: "--surface", want: tc.palette.Surface},
				{property: "--text", want: tc.palette.Text},
				{property: "--muted", want: tc.palette.TextMuted},
				{property: "--accent", want: tc.palette.Accent},
				{property: "--border", want: tc.palette.Border},
				{property: "--verse", want: tc.palette.VerseNumber},
				{property: "--red", want: tc.palette.RedLetter},
				{property: "--verse-hl", want: tc.palette.Highlight},
				{property: "--control-hover", want: tc.palette.ControlHover},
				{property: "--control-selected", want: tc.palette.ControlSelection},
			}
			for _, role := range roles {
				raw := cssCustomProperty(t, tc.block, role.property)
				got, err := parseCSSColor(raw)
				if err != nil {
					t.Errorf("%s: parse %s value %q: %v", tc.name, role.property, raw, err)
					continue
				}
				if got != role.want {
					t.Errorf("%s %s = %#v, want exported app colour %#v", tc.name, role.property, got, role.want)
				}
			}
		})
	}
}

func TestReaderCSSTintTokensHaveOneRole(t *testing.T) {
	css := stripCSSComments(readerCSS("regular.woff2", "bold.woff2"))
	if regexp.MustCompile(`--hl[[:space:]]*:`).MatchString(css) ||
		regexp.MustCompile(`var\([[:space:]]*--hl[[:space:]]*\)`).MatchString(css) {
		t.Fatal("the ambiguous --hl token remains; scripture and control feedback need separate roles")
	}

	for _, selector := range []string{".v:target", ".v.hl"} {
		assertSelectorUsesToken(t, css, selector, "--verse-hl")
		assertSelectorDoesNotUseToken(t, css, selector, "--control-hover")
		assertSelectorDoesNotUseToken(t, css, selector, "--control-selected")
	}
	for _, selector := range []string{
		".goto:hover",
		".arrow:hover",
		".gbooks a:hover",
		".notebtn:hover",
	} {
		assertSelectorUsesToken(t, css, selector, "--control-hover")
		assertSelectorDoesNotUseToken(t, css, selector, "--verse-hl")
	}
	assertSelectorUsesToken(t, css, ".gchaps a.on", "--control-selected")
	assertSelectorDoesNotUseToken(t, css, ".gchaps a.on", "--verse-hl")

	uses := []struct {
		token string
		want  int
	}{
		{token: "--verse-hl", want: 2},
		{token: "--control-hover", want: 4},
		{token: "--control-selected", want: 1},
	}
	for _, use := range uses {
		pattern := regexp.MustCompile(`var\([[:space:]]*` + regexp.QuoteMeta(use.token) + `[[:space:]]*\)`)
		if got := len(pattern.FindAllString(css, -1)); got != use.want {
			t.Errorf("%s has %d uses, want %d; each visual role must stay confined to its intended selectors", use.token, got, use.want)
		}
	}
}

func TestWebReaderPaletteContrast(t *testing.T) {
	light, dark := bibletext.WebReaderPalettes()
	for _, tc := range []struct {
		name    string
		palette bibletext.WebReaderPalette
	}{
		{name: "light", palette: light},
		{name: "dark", palette: dark},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := webContrastRatio(tc.palette.Text, tc.palette.Highlight); got < 4.5 {
				t.Errorf("body text on the verse highlight is %.2f:1, want at least 4.5:1", got)
			}
			if got := webContrastRatio(tc.palette.RedLetter, tc.palette.Highlight); got < 3.0 {
				t.Errorf("red-letter text on the verse highlight is %.2f:1, want at least 3.0:1", got)
			}

			for _, ground := range []struct {
				name  string
				color color.NRGBA
			}{
				{name: "background", color: tc.palette.Background},
				{name: "surface", color: tc.palette.Surface},
			} {
				hover := compositeWebColor(tc.palette.ControlHover, ground.color)
				if got := webContrastRatio(tc.palette.Accent, hover); got < 4.5 {
					t.Errorf("accent text on the %s hover wash is %.2f:1, want at least 4.5:1", ground.name, got)
				}
			}

			selected := compositeWebColor(tc.palette.ControlSelection, tc.palette.Surface)
			if got := webContrastRatio(tc.palette.Accent, selected); got < 4.5 {
				t.Errorf("accent text on the selected control wash is %.2f:1, want at least 4.5:1", got)
			}
		})
	}
}

func cssRootBlocks(t *testing.T, css string) []string {
	t.Helper()
	const marker = ":root"
	var blocks []string
	for searchFrom := 0; ; {
		rel := strings.Index(css[searchFrom:], marker)
		if rel < 0 {
			break
		}
		start := searchFrom + rel
		openRel := strings.IndexByte(css[start+len(marker):], '{')
		if openRel < 0 {
			t.Fatalf("reader CSS has a :root selector without a block")
		}
		open := start + len(marker) + openRel
		depth := 0
		close := -1
		for i := open; i < len(css); i++ {
			switch css[i] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					close = i
				}
			}
			if close >= 0 {
				break
			}
		}
		if close < 0 {
			t.Fatalf("reader CSS has an unterminated :root block")
		}
		blocks = append(blocks, css[open+1:close])
		searchFrom = close + 1
	}
	return blocks
}

func cssCustomProperty(t *testing.T, block, property string) string {
	t.Helper()
	pattern := regexp.MustCompile(regexp.QuoteMeta(property) + `[[:space:]]*:[[:space:]]*([^;]+);`)
	match := pattern.FindStringSubmatch(block)
	if match == nil {
		t.Fatalf("reader CSS root is missing %s", property)
	}
	return strings.TrimSpace(match[1])
}

func parseCSSColor(value string) (color.NRGBA, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if strings.HasPrefix(value, "#") {
		hex := strings.TrimPrefix(value, "#")
		if len(hex) != 6 && len(hex) != 8 {
			return color.NRGBA{}, fmt.Errorf("hex colour has %d digits", len(hex))
		}
		parsed, err := strconv.ParseUint(hex, 16, 32)
		if err != nil {
			return color.NRGBA{}, err
		}
		if len(hex) == 6 {
			return color.NRGBA{
				R: uint8(parsed >> 16),
				G: uint8(parsed >> 8),
				B: uint8(parsed),
				A: 255,
			}, nil
		}
		return color.NRGBA{
			R: uint8(parsed >> 24),
			G: uint8(parsed >> 16),
			B: uint8(parsed >> 8),
			A: uint8(parsed),
		}, nil
	}

	if !strings.HasPrefix(value, "rgba(") || !strings.HasSuffix(value, ")") {
		return color.NRGBA{}, fmt.Errorf("unsupported CSS colour")
	}
	parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(value, "rgba("), ")"), ",")
	if len(parts) != 4 {
		return color.NRGBA{}, fmt.Errorf("rgba colour has %d components", len(parts))
	}
	channels := [3]uint8{}
	for i := range channels {
		component, err := strconv.ParseUint(strings.TrimSpace(parts[i]), 10, 8)
		if err != nil {
			return color.NRGBA{}, fmt.Errorf("channel %d: %w", i, err)
		}
		channels[i] = uint8(component)
	}
	alpha, err := strconv.ParseFloat(strings.TrimSpace(parts[3]), 64)
	if err != nil {
		return color.NRGBA{}, fmt.Errorf("alpha: %w", err)
	}
	if alpha < 0 || alpha > 1 {
		return color.NRGBA{}, fmt.Errorf("alpha %v is outside 0..1", alpha)
	}
	return color.NRGBA{
		R: channels[0],
		G: channels[1],
		B: channels[2],
		A: uint8(math.Round(alpha * 255)),
	}, nil
}

func stripCSSComments(css string) string {
	return regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(css, "")
}

func assertSelectorUsesToken(t *testing.T, css, selector, token string) {
	t.Helper()
	bodies := cssRuleBodies(css, selector)
	if len(bodies) == 0 {
		t.Errorf("reader CSS is missing selector %s", selector)
		return
	}
	needle := "var(" + token + ")"
	for _, body := range bodies {
		if strings.Contains(strings.ReplaceAll(body, " ", ""), needle) {
			return
		}
	}
	t.Errorf("selector %s does not use %s", selector, token)
}

func assertSelectorDoesNotUseToken(t *testing.T, css, selector, token string) {
	t.Helper()
	needle := "var(" + token + ")"
	for _, body := range cssRuleBodies(css, selector) {
		if strings.Contains(strings.ReplaceAll(body, " ", ""), needle) {
			t.Errorf("selector %s must not use %s", selector, token)
		}
	}
}

func cssRuleBodies(css, wantedSelector string) []string {
	pattern := regexp.MustCompile(`(?s)([^{}]+)\{([^{}]*)\}`)
	var bodies []string
	for _, match := range pattern.FindAllStringSubmatch(css, -1) {
		for _, selector := range strings.Split(match[1], ",") {
			if strings.TrimSpace(selector) == wantedSelector {
				bodies = append(bodies, match[2])
				break
			}
		}
	}
	return bodies
}

func compositeWebColor(over, under color.NRGBA) color.NRGBA {
	alpha := float64(over.A) / 255
	mix := func(foreground, background uint8) uint8 {
		return uint8(math.Round(float64(foreground)*alpha + float64(background)*(1-alpha)))
	}
	return color.NRGBA{
		R: mix(over.R, under.R),
		G: mix(over.G, under.G),
		B: mix(over.B, under.B),
		A: 255,
	}
}

func webContrastRatio(a, b color.NRGBA) float64 {
	lighter := webRelativeLuminance(a)
	darker := webRelativeLuminance(b)
	if lighter < darker {
		lighter, darker = darker, lighter
	}
	return (lighter + 0.05) / (darker + 0.05)
}

func webRelativeLuminance(c color.NRGBA) float64 {
	linear := func(component uint8) float64 {
		channel := float64(component) / 255
		if channel <= 0.04045 {
			return channel / 12.92
		}
		return math.Pow((channel+0.055)/1.055, 2.4)
	}
	return 0.2126*linear(c.R) + 0.7152*linear(c.G) + 0.0722*linear(c.B)
}
