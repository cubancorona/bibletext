package bibletext

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
)

// Shared type scale, used by both the desktop and mobile layouts so headings
// and sub-labels stay the same size across platforms.
const (
	headingTextSize    = 26 // primary page heading: "Book N", "Results for …"
	subheadingTextSize = 13 // chapter line ("Chapter N of M"), search sub-label
)

// Custom theme color names. ThemeColorName is just a string, so we can register
// our own and resolve them through bibleTheme.Color. RichText segments reference
// these names to stay in sync with light/dark mode automatically.
const (
	colorNameVerseNumber fyne.ThemeColorName = "bibleTextVerseNumber"
	colorNameVerseText   fyne.ThemeColorName = "bibleTextVerseText"
	colorNameHighlight   fyne.ThemeColorName = "bibleTextHighlight"
	colorNameHighlightHi fyne.ThemeColorName = "bibleTextHighlightText"
	// colorNameHighlightMulti is HighlightMulti by name, for the one surface
	// (the RichText fallback) that can only reference palette colours through
	// the theme. Wired now so the multi-note wash reaches every surface the way
	// Highlight does; nothing resolves it yet (tintMulti is unreachable — see
	// tint_multi_guard_test.go).
	colorNameHighlightMulti fyne.ThemeColorName = "bibleTextHighlightMulti"
	colorNameMuted          fyne.ThemeColorName = "bibleTextMuted"
	// The native panes write pal.RedLetter into their markup as a hex literal,
	// but a RichText segment can only NAME a theme colour — so the Fyne fallback
	// pane needs the same palette entry reachable by name.
	colorNameRedLetter fyne.ThemeColorName = "bibleTextRedLetter"
)

// palette is the single source of truth for every colour in the UI. Routing all
// colours through here keeps the design consistent and makes dark mode a swap.
type palette struct {
	Background    color.NRGBA // window backdrop
	Surface       color.NRGBA // reading paper / cards
	SurfaceAlt    color.NRGBA // sidebar, header, chips
	Border        color.NRGBA
	Text          color.NRGBA
	TextMuted     color.NRGBA
	Accent        color.NRGBA // primary / interactive
	AccentText    color.NRGBA // text drawn on Accent
	Highlight     color.NRGBA // faint wash behind the highlighted verse
	HighlightText color.NRGBA // the highlighted verse's own text
	// HighlightMulti is the wash where MORE THAN ONE note covers a verse
	// (tintMulti, tint.go) — separated from Highlight by hue, not brightness.
	// Two nearby washes of one hue would read as different strengths of the
	// same mark, while different hue families read as different facts. Nothing
	// draws it yet — one lit span at a time is the recorded invariant, held by
	// tint_multi_guard_test.go until a multi-wash is requested.
	HighlightMulti color.NRGBA
	VerseNumber    color.NRGBA // superscript verse numbers
	RedLetter      color.NRGBA // words of Christ (red-letter mode)
	Input          color.NRGBA

	// ControlOutline draws the OFF state of a checkbox and the ring of an
	// unselected radio — the only things ColorNameInputBorder reaches here. It is
	// separate from Border because Border is a CARD EDGE: an edge may whisper, but
	// a control that can be switched on has to look switchable, which WCAG puts at
	// 3:1 against its own ground.
	ControlOutline color.NRGBA

	// Disabled / DisabledButton are the label+fill of a button that cannot be
	// pressed. Unmapped, Fyne answered with its own greys — cool, off-palette, and
	// 1.26:1 in dark, which made "Delete all notes" (disabled whenever there are
	// none, i.e. on every fresh install) a blank slab. They stay quieter than the
	// enabled pair on purpose: disabled must READ as disabled, just not as absent.
	Disabled       color.NRGBA
	DisabledButton color.NRGBA
}

// Light: warm parchment ground so the crisp near-white "page" appears to glow.
var lightPalette = palette{
	Background: color.NRGBA{R: 237, G: 233, B: 224, A: 255},
	Surface:    color.NRGBA{R: 253, G: 252, B: 248, A: 255},
	SurfaceAlt: color.NRGBA{R: 244, G: 240, B: 232, A: 255},
	// Border/TextMuted/VerseNumber are tuned for older eyes: TextMuted holds ≥4.5:1
	// (WCAG AA for normal-size text) against ALL three grounds it appears on (4.8 on
	// Background, 5.2 on SurfaceAlt, 5.7 on Surface — was 3.4–4.0:1), VerseNumber is
	// 5.5:1 on the reading paper (was a borderline 4.55), and Border is dark enough
	// (~1.8:1) that chips and inputs read as controls instead of dissolving into the
	// parchment (was 1.16:1). The dark palette already passed AA and is untouched.
	Border:     color.NRGBA{R: 189, G: 178, B: 159, A: 255},
	Text:       color.NRGBA{R: 37, G: 34, B: 29, A: 255},
	TextMuted:  color.NRGBA{R: 107, G: 100, B: 86, A: 255},
	Accent:     color.NRGBA{R: 47, G: 76, B: 134, A: 255}, // lapis / sapphire — the sacred manuscript blue
	AccentText: color.NRGBA{R: 244, G: 247, B: 252, A: 255},
	// A marker-pen amber, not a lapis wash. The wash was too close in weight to
	// the parchment to read as a highlight at all, and it sat behind RED LETTERS,
	// where a faint blue tint just muddied them. Amber is the one hue that says
	// "highlighted" while leaving red as red.
	Highlight:     color.NRGBA{R: 255, G: 224, B: 138, A: 255}, // marker-pen amber
	HighlightText: color.NRGBA{R: 36, G: 60, B: 112, A: 255},   // unused by the panes; see reading.go .hl
	// A cool lapis-family wash beside the amber: hue separation, so "several
	// notes here" cannot be misread as "one strong note". Red letters hold
	// 4.2:1 on it (theme_contrast_test.go pins >= 3.0). Deliberately chosen as
	// a pair with its dark twin below; unreachable until requested (tintMulti).
	HighlightMulti: color.NRGBA{R: 199, G: 219, B: 245, A: 255}, // #C7DBF5
	VerseNumber:    color.NRGBA{R: 83, G: 104, B: 143, A: 255},  // muted slate-blue superscripts
	RedLetter:      color.NRGBA{R: 178, G: 58, B: 46, A: 255},   // deep crimson on parchment
	Input:          color.NRGBA{R: 252, G: 251, B: 247, A: 255},
	// Unchanged from what Border already drew here: light-mode boxes and rings
	// were never the problem (the fill is lighter than the card, so the control
	// reads even at a 1.84:1 outline), and this palette is tuned and shipped.
	ControlOutline: color.NRGBA{R: 189, G: 178, B: 159, A: 255},
	// 2.9:1 — clearly quieter than the 13.9:1 enabled pair, and warm, where
	// Fyne's stock disabled grey read as a cool foreign chip on the parchment.
	Disabled:       color.NRGBA{R: 142, G: 135, B: 122, A: 255},
	DisabledButton: color.NRGBA{R: 236, G: 232, B: 224, A: 255},
}

// Dark: warm near-black with a luminous sapphire accent — illuminated, not stark.
var darkPalette = palette{
	Background: color.NRGBA{R: 25, G: 23, B: 21, A: 255},
	Surface:    color.NRGBA{R: 34, G: 31, B: 28, A: 255},
	SurfaceAlt: color.NRGBA{R: 42, G: 38, B: 34, A: 255},
	Border:     color.NRGBA{R: 57, G: 52, B: 46, A: 255},
	Text:       color.NRGBA{R: 233, G: 227, B: 217, A: 255},
	TextMuted:  color.NRGBA{R: 157, G: 148, B: 135, A: 255},
	Accent:     color.NRGBA{R: 124, G: 160, B: 228, A: 255}, // luminous sapphire on near-black
	AccentText: color.NRGBA{R: 17, G: 24, B: 40, A: 255},
	// A red-first violet. The brighter #544AAB direction is vivid against the
	// night page and keeps body text at 5.6:1, but red letters fall to 2.4:1.
	// That is not enough for Georgia's fine strokes, and a real highlighted
	// verse can carry cream narration and red speech on one continuous band.
	//
	// #3A326F keeps the violet unmistakable while restoring 3.8:1 for red and
	// 8.8:1 for body text. Its 1.6:1 luminance separation from #191715 is modest,
	// but the violet chroma keeps the glyph-tight band visible without making it
	// look like an electric selection. The primary-wash contrast test pins the
	// text side of that balance.
	Highlight:     color.NRGBA{R: 58, G: 50, B: 111, A: 255},   // #3A326F
	HighlightText: color.NRGBA{R: 182, G: 205, B: 240, A: 255}, // unused by the panes; see reading.go .hl
	// The violet band's cooler, lower-chroma slate-blue partner: primary leans
	// violet (R > G), while this leans blue (G > R), so the two facts do not read
	// as one mark at two strengths. The soft red above it measures 3.6:1 (pinned
	// >= 3.0 in theme_contrast_test.go). Unreachable until requested (tintMulti).
	HighlightMulti: color.NRGBA{R: 46, G: 62, B: 92, A: 255},    // #2E3E5C
	VerseNumber:    color.NRGBA{R: 140, G: 168, B: 216, A: 255}, // light slate-blue superscripts
	RedLetter:      color.NRGBA{R: 229, G: 115, B: 115, A: 255}, // soft red, legible on near-black
	Input:          color.NRGBA{R: 38, G: 35, B: 31, A: 255},
	// Lifted well clear of Border. Border sat 1.22:1 from SurfaceAlt, which is
	// fine for an edge and invisible for a hairline checkbox; this is 3.1:1 on
	// SurfaceAlt, 3.7:1 on Background and 3.4:1 on Surface, so an unticked box
	// reads as a box on all three grounds it can land on. Warm, to stay in the
	// palette's family rather than going neutral grey.
	ControlOutline: color.NRGBA{R: 120, G: 112, B: 101, A: 255},
	// 2.8:1 against the disabled fill, against 11.8:1 for the enabled pair.
	// Fyne's unmapped defaults were #39393a on #28292e — 1.26:1, i.e. a blank
	// cool-grey pill where the words should be.
	Disabled:       color.NRGBA{R: 118, G: 111, B: 101, A: 255},
	DisabledButton: color.NRGBA{R: 48, G: 44, B: 40, A: 255},
}

// bibleTheme is a Fyne theme whose colours come from the active palette. Light
// vs. dark is driven by the OS variant Fyne hands to Color() — there is no
// explicit in-app toggle; we follow the system setting.
type bibleTheme struct {
	fonts   *bookFonts // book-like serif (scripture / share images); nil → Fyne's bundled font
	uiFonts *bookFonts // chrome typeface (Atkinson Hyperlegible); nil → fall back to fonts
}

// isDark reports whether the app should currently render with the dark
// palette, derived from the current Fyne app's theme variant (which itself
// tracks the OS appearance setting).
func isDark() bool {
	app := fyne.CurrentApp()
	if app == nil {
		return false
	}
	return app.Settings().ThemeVariant() == theme.VariantDark
}

// palette returns the right palette for the current system appearance.
// Code that needs colours outside of Fyne's Color() callback (e.g. canvas
// rectangles, the HTML the iOS UITextView consumes) uses this.
func (t *bibleTheme) palette() palette {
	if isDark() {
		return darkPalette
	}
	return lightPalette
}

// paletteFor maps a Fyne theme variant to one of our palettes. Used inside
// Color(), where we get the variant for free.
func paletteFor(variant fyne.ThemeVariant) palette {
	if variant == theme.VariantDark {
		return darkPalette
	}
	return lightPalette
}

func (t *bibleTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	p := paletteFor(variant)
	switch name {
	case theme.ColorNameBackground:
		return p.Background
	case theme.ColorNameOverlayBackground:
		// Fyne's popup renderer paints this rectangle a few px (SizeNameInnerPadding/2)
		// BEYOND the popup's content on every side. The default is near-white, which
		// shows as a thin white border around our parchment popup cards. Painting it
		// the page parchment makes that frame blend into the reading ground (and gives
		// the native verse menu — the one popup that uses this as its own fill — a
		// parchment background instead of white).
		return p.Background
	case theme.ColorNameHeaderBackground:
		return p.SurfaceAlt
	case theme.ColorNameForeground:
		return p.Text
	case theme.ColorNamePrimary:
		return p.Accent
	case theme.ColorNameHyperlink:
		// Links in the palette accent, not Fyne's default #296FF6 — without
		// this every stock Hyperlink (Settings' "Get a key" / "Privacy
		// Policy", the waiting screens' faster-model line) renders in a bright
		// off-palette blue in BOTH variants (and follows the user's global
		// fyne primaryColor setting rather than the app).
		return p.Accent
	case theme.ColorNameForegroundOnPrimary:
		// The label/icon on accent-filled (primary) buttons. Use the palette's
		// AccentText — a cool near-white on the light-mode lapis accent, deep navy on
		// the lighter dark-mode sapphire accent — rather than the default stark white
		// (which would be unreadable on the dark-mode accent).
		return p.AccentText
	case theme.ColorNameButton:
		return p.SurfaceAlt
	case theme.ColorNameInputBackground:
		return p.Input
	case theme.ColorNameInputBorder:
		// Not p.Border. In THIS app this name reaches exactly one kind of thing:
		// the OFF state of a checkbox and the ring of an unselected radio. Fyne
		// draws both entirely from theme icons (widget/check.go and
		// widget/radio_item.go ask for IconNameCheckButton / IconNameRadioButton
		// tinted with ColorNameInputBorder, and their fill with
		// ColorNameInputBackground). Entry outlines do NOT use it here, because
		// SizeNameInputBorder is 0 below — our own bordered surfaces draw those.
		//
		// p.Border is a CARD EDGE colour, and on the dark Settings cards it came
		// to 1.22:1 against SurfaceAlt. The OFF glyphs are hairlines, so at that
		// contrast the box and the ring simply were not there: "Show the words of
		// Christ in red" and the whole ASSISTANT list looked like inert text until
		// something was selected, at which point one accent dot appeared among
		// invisible siblings. Light mode never showed it (1.84:1 outline over a
		// fill LIGHTER than the card, so the box reads).
		return p.ControlOutline
	case theme.ColorNameFocus:
		// The last member of Fyne's primary-colour quartet we had not claimed.
		// Unmapped it fell through to the stock #006cff, which is off-palette in
		// both variants AND follows the user's global Fyne primary preference —
		// so a reader who had set Fyne to, say, green got green focus rings in a
		// sapphire app.
		return withAlpha(p.Accent, 96)
	case theme.ColorNameMenuBackground:
		// The verse study menu is the one popup that was not painted from the
		// palette: unmapped, it took Fyne's cool #28292e in dark and #f5f5f5 in
		// light, both of which read as a foreign slab over the warm page. Same
		// answer as ColorNameOverlayBackground above, for the same reason.
		return p.Background
	case theme.ColorNameDisabled:
		return p.Disabled
	case theme.ColorNameDisabledButton:
		return p.DisabledButton
	case theme.ColorNamePlaceHolder, colorNameMuted:
		return p.TextMuted
	case theme.ColorNameSeparator:
		return p.Border
	case theme.ColorNameScrollBar:
		return withAlpha(p.TextMuted, 120)
	case theme.ColorNameHover:
		return withAlpha(p.Accent, 28)
	case theme.ColorNamePressed:
		return withAlpha(p.Accent, 48)
	case theme.ColorNameSelection:
		return withAlpha(p.Accent, 40)
	case theme.ColorNameShadow:
		if variant == theme.VariantDark {
			return color.NRGBA{A: 90}
		}
		return color.NRGBA{A: 24}
	case colorNameVerseNumber:
		return p.VerseNumber
	case colorNameVerseText:
		return p.Text
	case colorNameHighlight:
		return p.Highlight
	case colorNameHighlightHi:
		return p.HighlightText
	case colorNameHighlightMulti:
		return p.HighlightMulti
	case colorNameRedLetter:
		return p.RedLetter
	}

	return theme.DefaultTheme().Color(name, variant)
}

func (t *bibleTheme) Font(style fyne.TextStyle) fyne.Resource {
	// Monospace/symbol stay on the default faces. Everything else is UI chrome — the
	// scripture text is a native overlay (iOS/macOS) or wrapped separately — so it
	// uses the chrome typeface (Atkinson Hyperlegible) when present, then the serif,
	// then Fyne's default.
	if !style.Monospace && !style.Symbol {
		if t.uiFonts != nil {
			return t.uiFonts.face(style)
		}
		if t.fonts != nil {
			return t.fonts.face(style)
		}
	}
	return theme.DefaultTheme().Font(style)
}

func (t *bibleTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (t *bibleTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameText: // body text (reading + UI); Entry forces one size
		return 18
	case theme.SizeNameInputBorder:
		// The read-only reading text is an Entry; its blinking caret is drawn at
		// this width. Zero removes the caret. Entry outlines are supplied by our
		// own bordered surfaces instead (see the search/filter fields).
		return 0
	case theme.SizeNameCaptionText:
		return 11
	case theme.SizeNameHeadingText:
		// Nothing in this app sets SizeNameHeadingText any more — the search
		// results heading that used it was removed. It is kept equal to the
		// canvas-text page headings ("Book N") so that anything picking it up
		// (a Fyne-internal consumer, or a heading added later) still matches.
		return headingTextSize
	case theme.SizeNameSubHeadingText:
		return 16
	case theme.SizeNamePadding:
		return 7
	case theme.SizeNameInnerPadding:
		return 8 // a little more breathing room inside buttons/fields
	case theme.SizeNameInputRadius:
		// Softer, more modern rounding on buttons (and input fields, which share
		// this radius) — the default is small and reads a touch dated. inputFrame's
		// outline matches this so the search/filter fields stay consistent.
		return 10
	case theme.SizeNameLineSpacing:
		return 10 // a touch airier for an unhurried, page-like read
	}
	return theme.DefaultTheme().Size(name)
}

func withAlpha(c color.NRGBA, a uint8) color.NRGBA {
	c.A = a
	return c
}

// surface draws content on a bordered, padded card in the given colours. It is
// the building block for the sidebar, reading paper, history bar and popups.
func surface(content fyne.CanvasObject, bg, border color.Color, minSize fyne.Size) fyne.CanvasObject {
	frame := canvas.NewRectangle(bg)
	frame.StrokeColor = border
	frame.StrokeWidth = 1
	frame.CornerRadius = 8
	if minSize.Width > 0 || minSize.Height > 0 {
		frame.SetMinSize(minSize)
	}
	return container.NewStack(frame, container.NewPadded(content))
}

// readingGround draws content on the flat reading ground: the page colour, no
// border, no corner radius, and the SAME padding surface() applies — so a caller
// swapping one for the other moves no glyph.
//
// WHY THIS EXISTS RATHER THAN surface(). The reading area originally used the
// app's rounded card surface. Flattening it to a parchment reading ground on
// the Fyne path changed only the FILL argument, from pal.Surface to
// pal.Background, and left the stroke and the 8pt radius in place. That removed
// the card's body and kept its outline, which is what a Windows or Linux reader
// has been looking at ever since: a stray rounded rectangle around scripture,
// while iOS, iPadOS, Android and macOS draw it straight onto the page.
//
// The styled pane shared that call and therefore inherited the outline. This
// helper keeps every reading surface flat by construction.
func readingGround(content fyne.CanvasObject, bg color.Color) fyne.CanvasObject {
	ground := canvas.NewRectangle(bg)
	return container.NewStack(ground, container.NewPadded(content))
}

// inputFrame draws a thin rounded outline around an input field without adding
// padding. We zero the theme's input-border size (to hide the read-only reading
// caret), so fields get their outline here instead. The rectangle is
// non-interactive, so clicks still reach the entry beneath it.
func inputFrame(content fyne.CanvasObject, border color.Color) fyne.CanvasObject {
	outline := canvas.NewRectangle(color.Transparent)
	outline.StrokeColor = border
	outline.StrokeWidth = 1
	outline.CornerRadius = 10 // match SizeNameInputRadius so the field reads as one shape
	return container.NewStack(content, outline)
}
