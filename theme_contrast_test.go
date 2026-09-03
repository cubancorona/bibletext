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

// Every wash is a band scripture sits on, and the hardest thing scripture puts
// on one is the red letters. Pin the tokens where they can fail: red-on-wash
// must clear the palette's chosen 3:1 design floor in both themes, and body text
// must stay clearly readable on it. That red floor is enough for the app's
// Large and Extra large sizes; it is deliberately not a claim of WCAG AA for
// the regular-weight 21px Normal size, which would require 4.5:1. The primary
// band is live today; the multi-note band is wired but deliberately unreachable.
func assertWashKeepsScriptureLegible(t *testing.T, name string, pal palette, wash color.NRGBA) {
	t.Helper()
	if got := contrastRatio(pal.RedLetter, wash); got < 3.0 {
		t.Errorf("%s: red letters on wash = %.2f:1, want >= 3.0:1 — His words would sink into the band",
			name, got)
	}
	if got := contrastRatio(pal.Text, wash); got < 4.5 {
		t.Errorf("%s: body text on wash = %.2f:1, want >= 4.5:1 (AA)", name, got)
	}
}

func TestPrimaryWashKeepsScriptureLegible(t *testing.T) {
	for _, variant := range []struct {
		name string
		pal  palette
	}{
		{"light primary", lightPalette},
		{"dark primary", darkPalette},
	} {
		assertWashKeepsScriptureLegible(t, variant.name, variant.pal, variant.pal.Highlight)
	}
}

// selectionOverWash is what the reader sees on iOS when they select words
// inside a washed verse: UIKit's highlight tint source-over the wash. The wash
// is drawn beneath the highlight view (BTWashView, reading_ios.go), so this is
// the compositor's own arithmetic, and the tint is the one MEASURED on the
// running text view in each appearance — rgba(0, 84, 166) at 0.20 in light,
// the same colour at 0.35 in dark — not an assumed systemBlue. The device
// colour is an input recorded here so that a future wash token is checked
// against the composite a selection actually produces, not the bare band; the
// same source-over predicts the unwashed light control word on paper
// (#EDE9E0 → #BECBD4) to the pixel, which is what grounds the constants.
func selectionOverWash(wash color.NRGBA, dark bool) color.NRGBA {
	const tr, tg, tb = 0, 84, 166
	ta := 0.20
	if dark {
		ta = 0.35
	}
	mix := func(tint, base uint8) uint8 {
		return uint8(math.Round(float64(tint)*ta + float64(base)*(1-ta)))
	}
	return color.NRGBA{R: mix(tr, wash.R), G: mix(tg, wash.G), B: mix(tb, wash.B), A: 255}
}

// A selected, washed verse is still scripture, and the red letters are still
// the hardest thing on it. The composites are pinned to the values the
// simulator recipe samples (docs/VISUAL_TESTS.md), then held to the same floors
// as the bare wash.
func TestSelectedWashKeepsScriptureLegible(t *testing.T) {
	for _, variant := range []struct {
		name string
		pal  palette
		want color.NRGBA
	}{
		{"light", lightPalette, color.NRGBA{R: 0xCC, G: 0xC4, B: 0x90, A: 255}},
		{"dark", darkPalette, color.NRGBA{R: 0x26, G: 0x3E, B: 0x82, A: 255}},
	} {
		got := selectionOverWash(variant.pal.Highlight, variant.name == "dark")
		if got != variant.want {
			t.Errorf("%s: selection over wash = #%02X%02X%02X, want #%02X%02X%02X (the value the sim recipe samples)",
				variant.name, got.R, got.G, got.B, variant.want.R, variant.want.G, variant.want.B)
		}
		assertWashKeepsScriptureLegible(t, variant.name+" selected", variant.pal, got)
	}
}

func TestApprovedHighlightTokensStayPinned(t *testing.T) {
	for _, variant := range []struct {
		name        string
		pal         palette
		wantPrimary color.NRGBA
		wantMulti   color.NRGBA
	}{
		{"light", lightPalette, color.NRGBA{R: 255, G: 224, B: 138, A: 255}, color.NRGBA{R: 199, G: 219, B: 245, A: 255}},
		{"dark", darkPalette, color.NRGBA{R: 58, G: 50, B: 111, A: 255}, color.NRGBA{R: 46, G: 62, B: 92, A: 255}},
	} {
		if variant.pal.Highlight != variant.wantPrimary {
			t.Errorf("%s: primary highlight = rgba(%d,%d,%d,%d), want approved rgba(%d,%d,%d,%d)",
				variant.name,
				variant.pal.Highlight.R, variant.pal.Highlight.G, variant.pal.Highlight.B, variant.pal.Highlight.A,
				variant.wantPrimary.R, variant.wantPrimary.G, variant.wantPrimary.B, variant.wantPrimary.A)
		}
		if variant.pal.HighlightMulti != variant.wantMulti {
			t.Errorf("%s: multi highlight = rgba(%d,%d,%d,%d), want approved rgba(%d,%d,%d,%d)",
				variant.name,
				variant.pal.HighlightMulti.R, variant.pal.HighlightMulti.G, variant.pal.HighlightMulti.B, variant.pal.HighlightMulti.A,
				variant.wantMulti.R, variant.wantMulti.G, variant.wantMulti.B, variant.wantMulti.A)
		}
	}
}

// The multi-note washes are also pinned to their approval rationale:
// separation from Highlight by hue, not brightness. Light pairs amber with
// blue; dark pairs violet with slate-blue. Component ordering keeps those hue
// families distinct without assuming the primary must be warm in every theme.
func TestMultiNoteWashKeepsScriptureLegible(t *testing.T) {
	th := &bibleTheme{}
	for _, variant := range []struct {
		name string
		v    fyne.ThemeVariant
		pal  palette
	}{
		{"light", theme.VariantLight, lightPalette},
		{"dark", theme.VariantDark, darkPalette},
	} {
		wash := variant.pal.HighlightMulti
		assertWashKeepsScriptureLegible(t, variant.name+" multi", variant.pal, wash)
		hl := variant.pal.Highlight
		if variant.v == theme.VariantLight {
			if !(hl.R > hl.G && hl.G > hl.B) || !(wash.B > wash.G && wash.G > wash.R) {
				t.Errorf("light: approved wash hues collapsed; primary rgb(%d,%d,%d) must be amber and multi rgb(%d,%d,%d) blue",
					hl.R, hl.G, hl.B, wash.R, wash.G, wash.B)
			}
		} else {
			if !(hl.B > hl.R && hl.R > hl.G) || !(wash.B > wash.G && wash.G > wash.R) {
				t.Errorf("dark: approved wash hues collapsed; primary rgb(%d,%d,%d) must be violet and multi rgb(%d,%d,%d) slate-blue",
					hl.R, hl.G, hl.B, wash.R, wash.G, wash.B)
			}
		}
		if got := contrastRatio(hl, wash); got > 1.2 {
			t.Errorf("%s: primary and multi washes differ by %.2f:1 in luminance, want <= 1.2:1 — they must separate by hue, not look like one mark at two strengths",
				variant.name, got)
		}
		// And the token is reachable BY NAME, for the surface that can only ask
		// the theme (the RichText fallback).
		if got := asNRGBA(t, th.Color(colorNameHighlightMulti, variant.v)); got != wash {
			t.Errorf("%s: colorNameHighlightMulti resolves to rgba(%d,%d,%d,%d), not the palette token",
				variant.name, got.R, got.G, got.B, got.A)
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
