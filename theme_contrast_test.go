package bibletext

import (
	"image/color"
	"math"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Contrast maths, so these tests argue in the same units the palette comments do.
func relLuminance(c color.NRGBA) float64 {
	f := func(v uint8) float64 {
		x := float64(v) / 255
		if x <= 0.03928 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*f(c.R) + 0.7152*f(c.G) + 0.0722*f(c.B)
}

func contrastRatio(a, b color.NRGBA) float64 {
	la, lb := relLuminance(a), relLuminance(b)
	hi, lo := math.Max(la, lb), math.Min(la, lb)
	return (hi + 0.05) / (lo + 0.05)
}

func asNRGBA(t *testing.T, c color.Color) color.NRGBA {
	t.Helper()
	r, g, b, a := c.RGBA()
	if a == 0 {
		return color.NRGBA{}
	}
	return color.NRGBA{
		R: uint8(r * 0xff / a),
		G: uint8(g * 0xff / a),
		B: uint8(b * 0xff / a),
		A: uint8(a >> 8),
	}
}

// A checkbox that is OFF, and a radio that is UNSELECTED, are drawn by Fyne
// entirely from ColorNameInputBorder (outline) and ColorNameInputBackground
// (fill). Those glyphs are hairlines, so if the outline does not separate from
// the card behind it the control is simply not on screen — which is what the
// dark Settings sheet looked like: labels with empty gaps, and the ASSISTANT
// list reading as inert text with one accent dot floating in it.
//
// 3:1 is WCAG's minimum for a UI component boundary, and it is the number the
// palette comment now claims.
func TestUncheckedControlsAreVisibleOnTheirCards(t *testing.T) {
	th := &bibleTheme{}
	for _, variant := range []struct {
		name string
		v    fyne.ThemeVariant
		pal  palette
	}{
		{"light", theme.VariantLight, lightPalette},
		{"dark", theme.VariantDark, darkPalette},
	} {
		outline := asNRGBA(t, th.Color(theme.ColorNameInputBorder, variant.v))
		fill := asNRGBA(t, th.Color(theme.ColorNameInputBackground, variant.v))
		// The two grounds these controls actually land on: settingsGroup draws its
		// cards on SurfaceAlt (every checkbox and radio in the app), and the page
		// itself is Background. Surface is the reading paper and carries no
		// checkboxes — asserting against it would fail light mode over a control
		// that is never drawn there.
		for _, ground := range []struct {
			name string
			c    color.NRGBA
		}{
			{"SurfaceAlt (settings card)", variant.pal.SurfaceAlt},
			{"Background (page)", variant.pal.Background},
		} {
			// The two palettes make the control readable by DIFFERENT means, so
			// asserting one number across both would either fail a shipped light
			// palette that has no defect, or pass a dark one that does.
			//
			// Light: the unchecked FILL is lighter than every ground, so the box is
			// legible as a pale tile whatever its hairline outline measures (it is
			// 1.73:1 on Background, and always was). That is the property to pin.
			//
			// Dark: the fill is DARKER than the card by a hair (1.04:1), so the
			// outline is the only thing there is — hence WCAG's 3:1 for a component
			// boundary, and hence the bug when it sat at 1.22:1.
			if variant.v == theme.VariantLight {
				if relLuminance(fill) <= relLuminance(ground.c) {
					t.Errorf("light: unchecked control fill is no lighter than %s — "+
						"nothing marks the box, and light mode has no outline contrast to fall back on",
						ground.name)
				}
				continue
			}
			if got := contrastRatio(outline, ground.c); got < 3.0 {
				t.Errorf("dark: unchecked control outline on %s = %.2f:1, want >= 3:1 "+
					"(the fill is only %.2f:1, so the outline is all there is)",
					ground.name, got, contrastRatio(fill, ground.c))
			}
		}
	}
}

// A disabled button must read as disabled, not as blank. Unmapped, these two
// names fell through to Fyne's own greys (#39393a on #28292e = 1.26:1 in dark),
// so "Delete all notes" — disabled whenever there are none, i.e. on every fresh
// install — was a cool-grey pill with no words on it.
func TestDisabledButtonsAreLegibleAndOnPalette(t *testing.T) {
	th := &bibleTheme{}
	for _, variant := range []struct {
		name string
		v    fyne.ThemeVariant
		pal  palette
	}{
		{"light", theme.VariantLight, lightPalette},
		{"dark", theme.VariantDark, darkPalette},
	} {
		fg := asNRGBA(t, th.Color(theme.ColorNameDisabled, variant.v))
		bg := asNRGBA(t, th.Color(theme.ColorNameDisabledButton, variant.v))

		if got := contrastRatio(fg, bg); got < 2.5 {
			t.Errorf("%s: disabled label on its fill = %.2f:1, want >= 2.5:1 (it must be readable)", variant.name, got)
		}
		// ...but still quieter than an enabled button, or "disabled" stops meaning
		// anything. Enabled is Foreground on Button.
		enabledFg := asNRGBA(t, th.Color(theme.ColorNameForeground, variant.v))
		enabledBg := asNRGBA(t, th.Color(theme.ColorNameButton, variant.v))
		if contrastRatio(fg, bg) >= contrastRatio(enabledFg, enabledBg) {
			t.Errorf("%s: disabled (%.2f:1) is not quieter than enabled (%.2f:1)",
				variant.name, contrastRatio(fg, bg), contrastRatio(enabledFg, enabledBg))
		}
		// And it must be OUR grey, not Fyne's cool one. The palette is warm
		// throughout: red >= blue on every surface in it.
		if bg.B > bg.R {
			t.Errorf("%s: disabled fill rgb(%d,%d,%d) is cooler than the warm palette around it",
				variant.name, bg.R, bg.G, bg.B)
		}
	}
}

// Colour names the app never mapped fell through to Fyne's defaults, which are
// off-palette in both variants — and ColorNameFocus additionally follows the
// user's GLOBAL Fyne primary preference, so someone whose Fyne is green got
// green focus rings inside a sapphire app.
func TestPreviouslyUnmappedColourNamesComeFromThePalette(t *testing.T) {
	th := &bibleTheme{}
	stock := theme.DefaultTheme()
	for _, name := range []fyne.ThemeColorName{
		theme.ColorNameFocus,
		theme.ColorNameMenuBackground,
		theme.ColorNameDisabled,
		theme.ColorNameDisabledButton,
		theme.ColorNameInputBorder,
	} {
		for _, v := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
			ours := asNRGBA(t, th.Color(name, v))
			theirs := asNRGBA(t, stock.Color(name, v))
			if ours == theirs {
				t.Errorf("%s (variant %d) still resolves to Fyne's stock rgba(%d,%d,%d,%d) — "+
					"it is not coming from the palette", name, v, theirs.R, theirs.G, theirs.B, theirs.A)
			}
		}
	}
}
