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
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
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

// fasterModelControl renders the offer as a small link line — Fyne's stock
// Hyperlink, in the palette accent via the theme's ColorNameHyperlink mapping,
// at caption size — so it reads as a quiet aside under Cancel rather than a
// competing action. Cancel keeps the only real button on the screen. A stock
// widget rather than hand-rolled tappable text: it brings the hover underline,
// keyboard focus and Space activation for free, and re-resolves its colour on
// a theme flip. Standard size names only — a custom ThemeSizeName measures as
// ZERO under any theme that doesn't define it (the Fyne test theme, a future
// override), collapsing the link's tap hit-band to nothing. (A LowImportance
// button was tried first: a bold body-size label, louder than Cancel's own —
// and inside a chip-theme override it rendered washed-out and mis-measured.)
func fasterModelControl(label string, onTapped func()) fyne.CanvasObject {
	hl := widget.NewHyperlink(label, nil)
	hl.OnTapped = onTapped
	hl.SizeName = theme.SizeNameCaptionText
	return container.NewCenter(hl)
}

// applyFasterModel pins the provider to its economy model. It is the same
// mechanism as choosing that model in Settings, so the choice persists and the
// reader can undo it there (or pick "Recommended" to come back).
func applyFasterModel(state *AppState, providerID, model string) {
	if state == nil || providerID == "" || model == "" {
		return
	}
	state.keys().setOverrideModel(providerID, model)
}
