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

import "strings"

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
	current := strings.TrimSpace(store.overrideModel(id))
	if current == "" {
		current = strings.TrimSpace(store.resolvedModel(id))
	}
	if current == "" {
		current = info.Model
	}
	if strings.EqualFold(current, info.FastModel) {
		return "", "", "", false // already on the quick one — nothing to offer
	}
	return id, info.FastModel, "Use a faster model", true
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
