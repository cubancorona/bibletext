package bibletext

import "context"

// Each request owns this record on its sending goroutine. Cache lookup uses
// the current settings. A retry on a replacement model updates the record
// before sending, so its answer is stored under the options that produced it.
type aiRequestIdentity struct {
	provider, model, effort string
}

type aiRequestIdentityKey struct{}

func trackAIRequestOptions(ctx context.Context, store *keyStore, id string) (context.Context, *aiRequestIdentity) {
	model := activeModelFor(store, id)
	options := &aiRequestIdentity{id, model, aiReasoningEffort(store.prefs, id, model)}
	return context.WithValue(ctx, aiRequestIdentityKey{}, options), options
}

func reportAIRequestOptions(ctx context.Context, provider, model, effort string) {
	if options, ok := ctx.Value(aiRequestIdentityKey{}).(*aiRequestIdentity); ok {
		*options = aiRequestIdentity{provider, model, effort}
	}
}

func (o aiRequestIdentity) cacheModel() string {
	if o.effort == "" {
		return o.model
	}
	return o.model + "|effort=" + o.effort
}
