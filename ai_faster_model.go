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
// reader is already on that model or a lighter pinned override — so an economy
// model is offered only when there is genuinely something faster to move to.
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
	if override := store.overrideModel(id); override != "" && !isCapableOverrideForFasterOffer(id, override, info) {
		return "", "", "", false
	}
	// "Switch", not "Use": this writes the same per-provider override the
	// Settings model picker does, so it persists until the reader changes it
	// back. A label implying a one-off would misrepresent that.
	return id, info.FastModel, "Switch to a faster model", true
}

func hasLightModelToken(low string) bool {
	parts := strings.FieldsFunc(low, func(r rune) bool {
		return r == '-' || r == '_' || r == '.' || r == '/' || r == ' '
	})
	for _, p := range parts {
		switch p {
		case "flash", "lite", "nano", "mini", "haiku", "small", "micro", "instant", "gemma", "fast":
			return true
		}
	}
	return containsAnySubstr(low, []string{"non-reasoning", "no-reasoning", "non-thinking"})
}

// isCapableOverrideForFasterOffer reports whether an explicit model override
// sits in a capable tier above info.FastModel. Lighter or economy overrides
// (flash-lite, flash, nano, mini, haiku, non-reasoning variants) must not be
// offered info.FastModel as "faster".
func isCapableOverrideForFasterOffer(providerID, model string, info providerInfo) bool {
	low := strings.ToLower(strings.TrimSpace(model))
	if low == "" || strings.EqualFold(low, info.FastModel) {
		return false
	}
	if hasLightModelToken(low) {
		return false
	}
	if providerID == providerGrok {
		if curRank, fastRank := parseVersionRank(low), parseVersionRank(strings.ToLower(info.FastModel)); curRank > 0 && fastRank > 0 && curRank < fastRank {
			return false
		}
	}
	return true
}

// fasterModelControl renders a secondary action as a caption-size stock
// Hyperlink in the palette accent via ColorNameHyperlink, used on sheets such
// as the shared-notes offer and Settings' key-test recovery where a full button
// would compete with the primary controls. Standard size names only — a custom
// ThemeSizeName measures as ZERO under any theme that does not define it,
// collapsing the link's tap hit-band.
func fasterModelControl(label string, onTapped func()) fyne.CanvasObject {
	return container.NewCenter(fasterModelLink(label, onTapped))
}

func fasterModelLink(label string, onTapped func()) *widget.Hyperlink {
	hl := widget.NewHyperlink(label, nil)
	hl.OnTapped = onTapped
	hl.SizeName = theme.SizeNameCaptionText
	return hl
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
