package bibletext

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"fyne.io/fyne/v2"
)

func TestEffortCatalogNewModelSurvivesReopenAndDropsRetiredChoice(t *testing.T) {
	const model = "gpt-synthetic-study"
	prefs := newFakePrefs()
	if choices := modelEffortChoices(prefs, providerOpenAI, model); len(choices) != 0 {
		t.Fatalf("model already has offline levels %v", choices)
	}
	rememberModelEfforts(prefs, providerOpenAI, []discoveredModel{
		{id: model, efforts: []string{"low", "high"}},
	})
	setAIReasoningEffort(prefs, providerOpenAI, model, "high")
	setAIReasoningEffort(prefs, providerOpenAI, model, "invalid")

	// Recreate the preference store from its persisted values, without
	// retaining the fetched model objects or the original preference store.
	reopened := newFakePrefs()
	for _, key := range []string{modelEffortCatalogKey(providerOpenAI), reasoningEffortKey(providerOpenAI, model)} {
		reopened.SetString(key, prefs.String(key))
	}
	if got := modelEffortChoices(reopened, providerOpenAI, model); !slices.Equal(got, []string{"low", "high"}) {
		t.Fatalf("reopened choices = %v", got)
	}
	if got := modelEffortChoices(reopened, providerGrok, model); len(got) != 0 {
		t.Fatalf("catalog escaped its provider: %v", got)
	}
	store := newKeyStoreWith(reopened)
	store.setOverrideModel(providerOpenAI, model)
	checkWire := func(want string) {
		t.Helper()
		mock := &mockHTTP{statusCode: 200, body: `{"choices":[{"message":{"content":"OK"}}]}`}
		resolver := effortTestResolver(t, store, providerOpenAI, map[string]*mockHTTP{model: mock})
		if got, err := resolver.generate(context.Background(), "A synthetic question."); got != "OK" || err != nil {
			t.Fatalf("generate = %q, %v", got, err)
		}
		if checkEffortInRequest(t, mock, model, want) {
			t.Error("newly discovered OpenAI reasoning model received a temperature override")
		}
	}
	checkWire("high")
	rememberModelEfforts(reopened, providerOpenAI, []discoveredModel{
		{id: model, efforts: []string{"low"}},
	})
	if got := aiReasoningEffort(reopened, providerOpenAI, model); got != "" {
		t.Fatalf("retired saved choice is still selectable: %q", got)
	}
	checkWire("")
}

func TestSettingsShowsEffortWhenNewModelCatalogArrives(t *testing.T) {
	const model = "gemini-9-synthetic"
	state := newAppearanceHarness(t, true).state
	state.aiKeys = newTestKeyStore()
	store := state.keys()
	store.setActiveProvider(providerGemini)
	store.setOverrideModel(providerGemini, model)
	store.setAPIKey(providerGemini, "synthetic")

	started := make(chan context.Context, 1)
	deliver := make(chan struct{})
	release := sync.OnceFunc(func() { close(deliver) })
	listModels := aiSettingsListModels
	aiSettingsListModels = func(ctx context.Context, _ providerInfo, _ string) ([]discoveredModel, error) {
		started <- ctx
		select {
		case <-deliver:
			return []discoveredModel{{id: model, efforts: []string{"medium", "high"}}}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	t.Cleanup(func() { aiSettingsListModels = listModels })
	t.Cleanup(release)
	showAISettings(state)
	var ctx context.Context
	select {
	case ctx = <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("opening Settings with a key did not fetch the model list")
	}
	var rows *modelSettingsLayout
	walkTree(settingsPopup(t, state), func(obj fyne.CanvasObject) {
		if c, ok := obj.(*fyne.Container); ok {
			if layout, ok := c.Layout.(*modelSettingsLayout); ok {
				rows = layout
			}
		}
	})
	hidden := rows != nil && !rows.effort.Visible()
	release()
	// The test driver executes the UI callback inline. The request context
	// is cancelled after that callback returns and the sheet is safe to read.
	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("catalog never landed in the open sheet")
	}
	if ctx.Err() != context.Canceled || !hidden || !rows.effort.Visible() {
		t.Fatalf("effort picker failed to appear after provider confirmation (initially hidden = %v)", hidden)
	}
	button := findTreeButton(rows.effort, "Recommended")
	if button == nil {
		t.Fatal("new model has no Recommended effort control")
	}
	button.OnTapped()
	list := settingsPickerList(t, state)
	if n := list.Length(); n != 3 {
		t.Fatalf("new model has %d picker rows, want Recommended, Medium, High", n)
	}
	list.Select(2)
	if button.Text != "High" || aiReasoningEffort(store.prefs, providerGemini, model) != "high" {
		t.Fatal("new model's High setting did not persist")
	}
}

func TestEffortCatalogControlsPrecedenceForAliasesAndFallbacks(t *testing.T) {
	const known = "gpt-5"
	base := discoveredModel{
		id: "synthetic-model", aliases: []string{known, "synthetic-alias", "synthetic-shared", "synthetic-bare"},
		efforts: []string{"LOW", "high", "low"},
	}
	for _, reversed := range []bool{false, true} {
		prefs := newFakePrefs()
		if len(modelEffortChoices(prefs, providerOpenAI, known)) == 0 {
			t.Fatal("test requires an offline model with levels")
		}
		rememberModelEfforts(prefs, providerOpenAI, []discoveredModel{{id: known}})
		if _, found := readEffortCatalog(prefs, providerOpenAI).Levels[known]; found {
			t.Fatal("missing capability metadata overrode the offline table")
		}
		saveEffortDocument(prefs, providerOpenAI, known, known, []string{"high"})
		if got := modelEffortChoices(prefs, providerOpenAI, known); !slices.Equal(got, []string{"high"}) {
			t.Fatalf("documented choices with no catalog evidence = %v", got)
		}
		models := []discoveredModel{
			base,
			{id: known, efforts: []string{}},
			{id: "synthetic-shared", efforts: []string{"medium"}},
			{id: "synthetic-bare"},
		}
		if reversed {
			slices.Reverse(models)
		}
		rememberModelEfforts(prefs, providerOpenAI, models)
		catalog := readEffortCatalog(prefs, providerOpenAI)
		for id, want := range map[string][]string{
			known: {}, "synthetic-shared": {"medium"}, "synthetic-bare": {"low", "high"},
			"synthetic-alias": {"low", "high"}, "synthetic-model": {"low", "high"},
		} {
			if got, found := catalog.Levels[id]; !found || !slices.Equal(got, want) {
				t.Errorf("reversed=%v catalog %s = %v, found %v, want %v", reversed, id, got, found, want)
			}
			if got := modelEffortChoices(prefs, providerOpenAI, id); !slices.Equal(got, want) {
				t.Errorf("reversed=%v settings %s = %v, want %v", reversed, id, got, want)
			}
		}
	}
}

func TestEffortUsesCurrentCatalogTarget(t *testing.T) {
	const alias = "gemini-4-carbon"
	const target = "gemini-10-synthetic"
	prefs := newFakePrefs()
	rememberModelEfforts(prefs, providerGemini, []discoveredModel{{id: alias, docsModel: target}})
	if got := modelEffortChoices(prefs, providerGemini, alias); len(got) != 0 {
		t.Fatalf("unknown replacement inherited obsolete choices: %v", got)
	}
	rememberModelEfforts(prefs, providerGemini, []discoveredModel{
		{id: alias, docsModel: target}, {id: target, efforts: []string{"low", "high"}},
	})
	if got := modelEffortChoices(prefs, providerGemini, alias); !slices.Equal(got, []string{"low", "high"}) {
		t.Fatalf("alias did not use current target's catalog choices: %v", got)
	}
}
