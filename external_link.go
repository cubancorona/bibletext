package bibletext

import (
	"math"
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Opening a link, and every link the app shows, in one place.
//
// WHY THIS EXISTS RATHER THAN widget.NewHyperlink(label, u). Fyne's desktop
// macOS OpenURL shells out to /usr/bin/open and blocks the calling goroutine
// until that process exits. Two problems follow. Spawning an executable from
// outside the bundle is the kind of thing the App Sandbox refuses, so a
// sandboxed build — the only kind the Mac App Store accepts — risks every
// link silently doing nothing; and Fyne's Hyperlink discards the error, so
// there is no log, no dialog, and no way for a reader to tell a dead link
// from a slow one.
//
// externalLink keeps the Hyperlink's appearance and focus behaviour exactly —
// it IS a Hyperlink — and only takes over what happens on tap, which
// Hyperlink.invokeAction lets us do by preferring OnTapped over its own
// openURL. openExternalURL then routes to the platform's supported API
// (NSWorkspace on macOS, external_link_darwin.go) instead of a subprocess.
//
// Use this for every link the app opens. A bare widget.NewHyperlink with a
// URL bypasses all of the above; external_link_test.go fails the build if one
// reappears.

// externalLink is a Hyperlink that opens through openExternalURL.
//
// A nil url yields a plain label-styled Hyperlink with no action, matching
// what NewHyperlink does with a nil target.
func externalLink(label string, u *url.URL) *widget.Hyperlink {
	hl := widget.NewHyperlink(label, nil)
	if u == nil {
		return hl
	}
	hl.OnTapped = func() { openExternalURL(u) }
	return hl
}

// outboundLink is an externalLink followed by the arrow that says the link
// leaves the app: "Get a key", "Privacy Policy".
//
// WHY THE ARROW IS AN ICON AND NOT THE CHARACTER. The labels used to end in
// U+2197, and every platform drew it as a colour emoji — a blue keycap tile, the
// only emoji in the interface. Fyne chooses a face for each character in a fixed
// order: the theme's font (Atkinson Hyperlegible), Fyne's own Noto Sans, the
// colour-emoji font it bundles (Noto Color Emoji in a shipped build, via
// patches/), and only then the system's fonts. Neither text face has U+2197 and
// the emoji face does, so the emoji face always wins, and it holds colour
// bitmaps only — there is no outline in it to fall back to. A text-presentation
// selector (U+FE0E) changes none of this: the face is chosen by which one has a
// glyph for the character, the selector itself resolves to no face at all, and
// the arrow still came out as the tile.
//
// So the label carries the words and the arrow is drawn from
// assets/icons/arrow_outward.svg in the link colour, where the character stood:
// a space after the words, standing on the baseline, cap height tall. Nothing
// about it depends on which fonts a platform has.
//
// It IS a Hyperlink, embedded, so focus, keyboard activation, the hover
// underline and the routing through openExternalURL are exactly externalLink's;
// a tap or a pointer on the arrow counts as one on the words.
type outboundLink struct {
	widget.Hyperlink
	overArrow bool
}

// outboundArrowScale is the arrow's side as a fraction of the text size:
// Atkinson Hyperlegible's cap height, 668/1000 em.
const outboundArrowScale = 0.668

// newOutboundLink is externalLink for a link that opens outside the app. A nil
// url yields an inert link, as externalLink's does.
func newOutboundLink(label string, u *url.URL) *outboundLink {
	l := &outboundLink{}
	l.Text = label
	l.ExtendBaseWidget(l)
	if u != nil {
		l.OnTapped = func() { openExternalURL(u) }
	}
	return l
}

// arrowBox is where the arrow is drawn, in the link's own coordinates. The
// Hyperlink sets its words at one inner padding in from its top-left corner.
func (l *outboundLink) arrowBox() (fyne.Position, fyne.Size) {
	th := l.Theme()
	sizeName := l.SizeName
	if sizeName == "" {
		sizeName = theme.SizeNameText
	}
	size := th.Size(sizeName)
	inner := th.Size(theme.SizeNameInnerPadding)
	drv := fyne.CurrentApp().Driver()
	words, baseline := drv.RenderedTextSize(l.Text, size, l.TextStyle, nil)
	space, _ := drv.RenderedTextSize(" ", size, l.TextStyle, nil)
	// Whole points: a box on the pixel grid rasterizes crisply at 2x and 3x,
	// where a fractional one is resampled and its edges go soft.
	side := float32(math.Round(float64(size * outboundArrowScale)))
	x := float32(math.Round(float64(inner + words.Width + space.Width)))
	y := float32(math.Round(float64(inner + baseline - side)))
	return fyne.NewPos(x, y), fyne.NewSquareSize(side)
}

// onArrow reports whether pos, in the link's coordinates, is on the arrow's
// part of the link: right of the words, full height.
func (l *outboundLink) onArrow(pos fyne.Position) bool {
	if iconLeavesApp == nil {
		return false
	}
	at, sz := l.arrowBox()
	inner := l.Theme().Size(theme.SizeNameInnerPadding)
	return pos.X >= at.X-inner/2 && pos.X <= at.X+sz.Width+inner/2 &&
		pos.Y >= 0 && pos.Y <= l.Size().Height
}

// Tapped opens the link from the arrow as well as from the words.
func (l *outboundLink) Tapped(e *fyne.PointEvent) {
	if l.onArrow(e.Position) {
		if l.OnTapped != nil {
			l.OnTapped()
		}
		return
	}
	l.Hyperlink.Tapped(e)
}

// MouseIn, MouseMoved, MouseOut and Cursor give the arrow the pointer cursor
// the words have.
func (l *outboundLink) MouseIn(e *desktop.MouseEvent) { l.MouseMoved(e) }

func (l *outboundLink) MouseMoved(e *desktop.MouseEvent) {
	l.overArrow = l.onArrow(e.Position)
	l.Hyperlink.MouseMoved(e)
}

func (l *outboundLink) MouseOut() {
	l.overArrow = false
	l.Hyperlink.MouseOut()
}

func (l *outboundLink) Cursor() desktop.Cursor {
	if l.overArrow {
		return desktop.PointerCursor
	}
	return l.Hyperlink.Cursor()
}

func (l *outboundLink) CreateRenderer() fyne.WidgetRenderer {
	l.ExtendBaseWidget(l)
	base := l.Hyperlink.CreateRenderer()
	r := &outboundLinkRenderer{WidgetRenderer: base, link: l}
	r.objects = append([]fyne.CanvasObject{}, base.Objects()...)
	if iconLeavesApp != nil {
		r.arrow = canvas.NewImageFromResource(iconLeavesApp)
		r.arrow.FillMode = canvas.ImageFillContain
		r.objects = append(r.objects, r.arrow)
	}
	return r
}

// outboundLinkRenderer is the Hyperlink's own renderer with the arrow added
// after the words.
type outboundLinkRenderer struct {
	fyne.WidgetRenderer
	link    *outboundLink
	arrow   *canvas.Image
	objects []fyne.CanvasObject
}

func (r *outboundLinkRenderer) Layout(size fyne.Size) {
	r.WidgetRenderer.Layout(size)
	if r.arrow != nil {
		at, sz := r.link.arrowBox()
		r.arrow.Move(at)
		r.arrow.Resize(sz)
	}
}

func (r *outboundLinkRenderer) MinSize() fyne.Size {
	min := r.WidgetRenderer.MinSize()
	if r.arrow == nil {
		return min
	}
	at, sz := r.link.arrowBox()
	inner := r.link.Theme().Size(theme.SizeNameInnerPadding)
	return fyne.NewSize(fyne.Max(min.Width, at.X+sz.Width+inner), min.Height)
}

func (r *outboundLinkRenderer) Objects() []fyne.CanvasObject { return r.objects }

func (r *outboundLinkRenderer) Refresh() {
	r.WidgetRenderer.Refresh()
	if r.arrow != nil {
		r.arrow.Refresh()
	}
}

// externalOpener is the seam the platform implementation is reached through.
// It is a variable rather than a direct call for one reason: a test that
// invokes the real one on macOS launches a browser or a mail composer on the
// machine running it. A test asserting where a tap is ROUTED has no business
// opening anything, so it substitutes this instead.
var externalOpener = openExternalURLPlatform

// openExternalURL hands a URL to the platform. The error is reported here
// rather than discarded, because a link that quietly fails is indistinguishable
// from an app that has hung.
func openExternalURL(u *url.URL) {
	if u == nil {
		return
	}
	if err := externalOpener(u); err != nil {
		fyne.LogError("could not open "+u.Scheme+" link", err)
	}
}

// openExternalURLDefault is the portable implementation: Fyne's own OpenURL.
// Platforms with a supported native API override it in their own file.
func openExternalURLDefault(u *url.URL) error {
	app := fyne.CurrentApp()
	if app == nil {
		return nil // no running app (tests): nothing to open, nothing to report
	}
	return app.OpenURL(u)
}
