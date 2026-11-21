package bibletext

import (
	"context"
	"testing"
)

func TestStudyAndFindCacheTheModelAndEffortThatAnswered(t *testing.T) {
	isolateAICache(t)
	store := newKeyStoreWith(newFakePrefs())
	store.setActiveProvider(providerOpenAI)
	store.setAPIKey(providerOpenAI, "synthetic-key")
	grantAIPermission(store, providerOpenAI)
	state := &AppState{aiKeys: store}
	p := aiPassage{Book: "Acts", Chapter: 2, Text: "A synthetic passage."}
	prevStudy, prevSearch := aiStudyGenerate, aiSearchGenerate
	t.Cleanup(func() { aiStudyGenerate, aiSearchGenerate = prevStudy, prevSearch })

	run := func(wantSends int) {
		t.Helper()
		sends := 0
		send := func(ctx context.Context, _ providerInfo, _ *keyStore, _, _ string) (string, error) {
			sends++
			// Simulate the resolver discovering a replacement on the first
			// request. Both actions must file its reply under the new model.
			reportAIRequestOptions(ctx, providerOpenAI, "gpt-5-mini", aiReasoningEffort(store.prefs, providerOpenAI, "gpt-5-mini"))
			return "Acts 2:1", nil
		}
		aiStudyGenerate, aiSearchGenerate = send, send
		if _, err := runAIActionForPassage(context.Background(), state, aiActionExplain, p, ""); err != nil {
			t.Fatal(err)
		}
		if _, err := runAISearch(context.Background(), state, "synthetic query"); err != nil {
			t.Fatal(err)
		}
		if sends != wantSends {
			t.Fatalf("sent %d requests, want %d", sends, wantSends)
		}
	}
	store.setOverrideModel(providerOpenAI, "gpt-5")
	run(2)
	store.setOverrideModel(providerOpenAI, "gpt-5-mini")
	run(0)
	setAIReasoningEffort(store.prefs, providerOpenAI, "gpt-5-mini", "high")
	run(2)
	run(0)
	setAIReasoningEffort(store.prefs, providerOpenAI, "gpt-5-mini", "")
	run(0)
	store.setOverrideModel(providerOpenAI, "gpt-5")
	run(2)
}
