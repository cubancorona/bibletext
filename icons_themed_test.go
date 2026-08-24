package bibletext

import (
	"image/color"
	"regexp"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
)

// forcedVariant answers every colour query for one fixed variant, whatever
// variant it is handed. Fyne's own test helpers do exactly this
// (internal/test.DarkTheme); the test driver's Settings reports a constant
// variant and has no setter, so this is the only way to render both looks.
type forcedVariant struct {
	fyne.Theme
	v fyne.ThemeVariant
}

func (f forcedVariant) Color(n fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	return f.Theme.Color(n, f.v)
}

// Every asset in assets/icons is a single fill="#000000" path, so an icon that
// reaches a widget as a plain StaticResource renders BLACK whatever the theme
// says. On the dark page (rgb(25,23,21)) that is all but invisible — which is
// how the Search tab's notes bubble rendered before its resource was themed.
//
// Fyne recolours an icon only when it can see a fyne.ThemedResource
// (widget/button.go tints one when Importance is neither Medium nor Low, and
// otherwise leaves the resource to colour itself). So "is it themed" is the
// property that decides whether the glyph is visible at night, and it is what
// these tests pin.

// iconFill pulls the fill colour out of a resource's rendered SVG content. It
// reads Content() rather than the file on disk, which is the point: a
// ThemedResource colourises on Content(), so this sees what Fyne will draw.
func iconFill(t *testing.T, r fyne.Resource) string {
	t.Helper()
	m := regexp.MustCompile(`fill="(#[0-9a-fA-F]{6})"`).FindSubmatch(r.Content())
	if m == nil {
		t.Fatalf("%s: no fill= in rendered content", r.Name())
	}
	return strings.ToLower(string(m[1]))
}

// TestButtonIconsAreThemed covers the icons handed to widget.NewButtonWithIcon,
// which is the call that does NOT colour its argument for itself. Icons drawn
// through iconTapButton / labeledTapChip are excluded on purpose: those wrap
// whatever they are given in theme.NewColoredResource at the use site, so they
// are already safe and are not what broke.
func TestButtonIconsAreThemed(t *testing.T) {
	for _, tc := range []struct {
		name string
		res  fyne.Resource
		used string
	}{
		{"iconNoteBubble", iconNoteBubble, "search.go (mode row) + notes_banner.go (Show note)"},
		{"iconAudioWave", iconAudioWave, "audio_menu.go (read-aloud source row)"},
		{"iconSidebarLeft", iconSidebarLeft, "ui.go (tablet header)"},
	} {
		if _, ok := tc.res.(fyne.ThemedResource); !ok {
			t.Errorf("%s is not a fyne.ThemedResource, so Fyne will draw the asset's own "+
				"fill=#000000 — invisible on the dark page. Used by %s", tc.name, tc.used)
		}
	}
}

// TestThemedIconsFollowTheTheme is the behavioural half: the SAME icon must
// render a different fill under a light theme than under a dark one. A type
// assertion alone would still pass if the resource were themed to a fixed
// colour; this fails unless the colour actually tracks the theme.
func TestThemedIconsFollowTheTheme(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	th := &bibleTheme{fonts: loadBookFonts(), uiFonts: loadUIFonts()}
	app.Settings().SetTheme(th)

	for _, tc := range []struct {
		name string
		res  fyne.Resource
	}{
		{"iconNoteBubble", iconNoteBubble},
		{"iconAudioWave", iconAudioWave},
	} {
		// The test driver reports a fixed variant of its own (test/app.go returns 2
		// unconditionally), so the variant cannot be set through Settings. Forcing
		// it through the THEME is how Fyne's own tests do it: a wrapper that ignores
		// the variant it is handed and answers for the one we want.
		app.Settings().SetTheme(forcedVariant{Theme: th, v: theme.VariantLight})
		light := iconFill(t, tc.res)
		app.Settings().SetTheme(forcedVariant{Theme: th, v: theme.VariantDark})
		dark := iconFill(t, tc.res)

		if light == "#000000" {
			t.Errorf("%s renders the raw asset fill (#000000) under the light theme", tc.name)
		}
		if light == dark {
			t.Errorf("%s renders %s in BOTH variants — it does not follow the theme", tc.name, light)
		}
		// The dark-mode fill must be the light TEXT colour, not something near the
		// dark page it sits on.
		want := strings.ToLower(nrgbaToHex(darkPalette.Text))
		if dark != want {
			t.Errorf("%s dark fill = %s, want the dark palette's Text %s", tc.name, dark, want)
		}
	}
}
