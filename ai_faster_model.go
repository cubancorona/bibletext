package bibletext

// The "faster model" offer shown on the waiting screens.
//
// The defaults are now each provider's capable model, because scripture study
// is worth the better answer (see ai_providers.go). The price is latency —
// tens of seconds rather than a few. This is the escape hatch: while a request
// is running, the reader can switch that provider to its economy model and have
// the same question answered again straight away.
//
// Deliberately an OFFER, never automatic. The app must not quietly downgrade
// what a reader is paying for; and it must not nag — the control only appears
// when there is genuinely something faster to move to.

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// fasterModelOffer describes the switch available for the active provider, if
// any. ok is false when AI is off, the provider has no economy model, or the
// reader is already on it (or on some other model of their own choosing).
func fasterModelOffer(state *AppState) (providerID, model, label string, ok bool) {
	if state == nil || !aiFeaturesEnabled(state) {
		return "", "", "", false
	}
	store := state.keys()
	id := store.activeProvider()
	info, found := providerByID(id)
	if !found || strings.TrimSpace(info.FastModel) == "" {
		return "", "", "", false
	}
	// What this provider would actually send right now: an explicit override,
	// else a self-healed model, else the default.
	current := activeModelFor(store, id)
	if strings.EqualFold(current, info.FastModel) {
		return "", "", "", false // already on the quick one — nothing to offer
	}
	// "Switch", not "Use": this writes the same per-provider override the
	// Settings model picker does, so it persists until the reader changes it
	// back. A label implying a one-off would misrepresent that.
	return id, info.FastModel, "Switch to a faster model", true
}

// fasterModelControl renders the offer as a small tappable text line — accent
// colour at caption size, no button chrome at all — so it reads as a quiet
// aside under Cancel rather than a competing action. Cancel keeps the only
// real button on the screen; this is the link-like alternative for the
// impatient. (A LowImportance button inside a smallChipTheme override was
// tried first: it rendered washed-out to the point of illegibility and
// mis-measured its label under the override.)
func fasterModelControl(state *AppState, label string, onTapped func()) fyne.CanvasObject {
	pal := lightPalette
	if state != nil {
		pal = state.pal()
	}
	return container.NewCenter(newTappableText(label, pal.Accent, onTapped))
}

// tappableText is a bare text line that acts on tap — link-like, no chrome.
type tappableText struct {
	widget.BaseWidget
	text     *canvas.Text
	onTapped func()
}

func newTappableText(label string, clr color.Color, onTapped func()) *tappableText {
	t := &tappableText{
		text:     canvas.NewText(label, clr),
		onTapped: onTapped,
	}
	t.text.TextSize = theme.CaptionTextSize() + 1
	t.ExtendBaseWidget(t)
	return t
}

func (t *tappableText) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewPadded(t.text))
}

func (t *tappableText) Tapped(*fyne.PointEvent) {
	if t.onTapped != nil {
		t.onTapped()
	}
}

func (t *tappableText) Cursor() desktop.Cursor { return desktop.PointerCursor }

var (
	_ fyne.Tappable      = (*tappableText)(nil)
	_ desktop.Cursorable = (*tappableText)(nil)
)

// applyFasterModel pins the provider to its economy model. It is the same
// mechanism as choosing that model in Settings, so the choice persists and the
// reader can undo it there (or pick "Recommended" to come back).
func applyFasterModel(state *AppState, providerID, model string) {
	if state == nil || providerID == "" || model == "" {
		return
	}
	state.keys().setOverrideModel(providerID, model)
}
