package bibletext

import (
	"context"
	"slices"
)

// Each request owns this record on its sending goroutine. Cache lookup uses
// the current settings. A retry on a replacement model updates the record
// before sending, so its answer is stored under the options that produced it.
type aiRequestIdentity struct {
	provider, model, effort string
}

type aiRequestIdentityKey struct{}

type aiSpeedChoiceKey struct{}

func withAISpeedChoice(ctx context.Context, choice aiSpeedChoice) context.Context {
	if choice.model == "" {
		return ctx
	}
	return context.WithValue(ctx, aiSpeedChoiceKey{}, choice)
}

// A speed action belongs to this request and to the model named by its link.
// Recheck at cache lookup and at send, since capabilities or Settings can change.
func requestedModelEffort(ctx context.Context, prefs prefStore, provider, model string, levels []string) (string, error) {
	if choice, ok := ctx.Value(aiSpeedChoiceKey{}).(aiSpeedChoice); ok {
		if choice.provider != provider || choice.model != model {
			return "", context.Canceled
		}
		if choice.effort != "" {
			if !slices.Contains(levels, choice.effort) {
				return "", context.Canceled
			}
			return choice.effort, nil
		}
	}
	return savedModelEffort(prefs, provider, model, levels), nil
}

func trackAIRequestOptions(ctx context.Context, store *keyStore, id string) (context.Context, *aiRequestIdentity, error) {
	model := activeModelFor(store, id)
	effort, err := requestedModelEffort(ctx, store.prefs, id, model, modelEffortChoices(store.prefs, id, model))
	if err != nil {
		return ctx, nil, err
	}
	options := &aiRequestIdentity{id, model, effort}
	return context.WithValue(ctx, aiRequestIdentityKey{}, options), options, nil
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
