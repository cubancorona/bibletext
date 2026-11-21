package bibletext

import (
	"encoding/json"
	"testing"
	"time"
)

func TestLowerReasoningEffort(t *testing.T) {
	for _, tc := range []struct {
		current string
		levels  []string
		want    string
	}{
		{"high", []string{"high", "low", "medium"}, "medium"},
		{"max", []string{"low", "experimental", "high", "max"}, "high"},
		{"xhigh", []string{"low", "xhigh", "max"}, "low"},
		{"medium", []string{"minimal", "medium"}, "minimal"},
		{"low", []string{"none", "low", "medium"}, ""},
		{"medium", []string{"low", "high"}, ""},
		{"", []string{"low", "high"}, ""},
		{"auto", []string{"low", "auto"}, ""},
		{"off", []string{"off", "low"}, ""},
		{"8192", []string{"0", "8192", "1024", "4096"}, "4096"},
		{"1024", []string{"0", "001", "low", "1024"}, ""},
		{"99999999999999999999999999", []string{"2048", "99999999999999999999999999"}, ""},
	} {
		if got := lowerReasoningEffort(tc.current, tc.levels); got != tc.want {
			t.Errorf("lower(%q, %v) = %q, want %q", tc.current, tc.levels, got, tc.want)
		}
	}
}

func TestVerifiedEffortDefaultsMatchExactModelsAndCapabilities(t *testing.T) {
	for _, tc := range []struct{ provider, model, want string }{
		{providerGemini, "gemini-4-carbon", "medium"},
		{providerGemini, "gemini-3.1-pro-preview", "high"},
		{providerGemini, "gemini-2.5-pro", ""},
		{providerGemini, "gemini-4-carbon-unknown", ""},
		{providerOpenAI, "gpt-5-mini", "medium"},
		{providerOpenAI, "gpt-5.4-2026-03-05", "none"},
		{providerOpenAI, "gpt-5.4-2026-03-06", ""},
		{providerAnthropic, "claude-opus-5", "high"},
		{providerAnthropic, "claude-haiku-5-5", "medium"},
		{providerAnthropic, "claude-haiku-4-5", ""},
		{providerGrok, "grok-4.3", "low"},
		{providerGrok, "grok-4.5", "high"},
		{providerGrok, "grok-4.5-latest", ""},
	} {
		c := modelEffortCatalog{}
		if got := verifiedDefaultEffort(nil, tc.provider, tc.model, c, c.choices(nil, tc.provider, tc.model)); got != tc.want {
			t.Errorf("default for %s/%s = %q, want %q", tc.provider, tc.model, got, tc.want)
		}
	}
	if got := verifiedDefaultEffort(nil, providerGemini, "gemini-4-carbon", modelEffortCatalog{}, []string{"high"}); got != "" {
		t.Fatalf("default is unavailable in current options: %q", got)
	}
}

func TestVerifiedDefaultUsesFreshSourceForExactModel(t *testing.T) {
	prefs := newFakePrefs()
	const model = "gemini-4-carbon"
	const alias = "gemini-synthetic-latest"
	levels := []string{"low", "medium", "high"}
	c := effortCatalogFromModels(providerGemini, []discoveredModel{{id: alias, docsModel: model}})
	if got := verifiedDefaultEffort(prefs, providerGemini, alias, c, levels); got != "medium" {
		t.Fatalf("fresh alias default = %q", got)
	}
	legacy := modelEffortCatalog{Targets: c.Targets}
	if got := verifiedDefaultEffort(prefs, providerGemini, alias, legacy, levels); got != "" {
		t.Fatalf("undated alias yielded %q", got)
	}
	rememberGeminiResponseTarget(prefs, alias, model)
	if got := verifiedDefaultEffort(prefs, providerGemini, alias, legacy, levels); got != "medium" {
		t.Fatalf("recent matching modelVersion yielded %q", got)
	}
	rememberGeminiResponseTarget(prefs, alias, "gemini-4-neon")
	if got := verifiedDefaultEffort(prefs, providerGemini, alias, c, levels); got != "" {
		t.Fatalf("conflicting alias evidence yielded %q", got)
	}
	key := effortDocKey(providerGemini, model)
	// The previous schema has levels but no default field. The concrete model
	// can still use its bundled default, and the next lookup upgrades it.
	prefs.SetString(key, `{"model":"gemini-4-carbon","levels":["low","medium","high"]}`)
	if got := verifiedDefaultEffort(prefs, providerGemini, model, c, levels); got != "medium" {
		t.Fatalf("legacy levels-only document yielded %q", got)
	}
	high := "high"
	doc := effortDocument{Levels: levels, Default: &high}
	doc.save(prefs, providerGemini, model, model)
	if got := verifiedDefaultEffort(prefs, providerGemini, model, c, levels); got != high {
		t.Fatalf("updated documented default = %q", got)
	}
	for _, offset := range []time.Duration{-effortDocAge - time.Minute, time.Minute} {
		stored := savedEffortDocument(prefs, providerGemini, model)
		stored.Checked = time.Now().Add(offset).Unix()
		b, _ := json.Marshal(stored)
		prefs.SetString(key, string(b))
		if got := verifiedDefaultEffort(prefs, providerGemini, model, c, levels); got != "" {
			t.Fatalf("invalid observation age %s yielded %q", offset, got)
		}
	}
	dynamic := ""
	doc.Default = &dynamic
	doc.save(prefs, providerGemini, model, model)
	if got := verifiedDefaultEffort(prefs, providerGemini, model, c, levels); got != "" {
		t.Fatalf("provider stopped naming a level; displayed %q", got)
	}
}

func TestPublishedDefaultComesFromTheExactLevelField(t *testing.T) {
	header := "## Controlling thinking\n| Model | Default Thinking | Levels Supported |\n| --- | --- | --- |\n"
	rows := header + "| gemini-9-synthetic | On (medium) | low, medium, high |\n| gemini-9-dynamic | On | low, high |\n"
	if d := geminiDocumentEfforts([]byte(rows), "gemini-9-synthetic"); d.Default == nil || *d.Default != "medium" {
		t.Fatal("missed the default on the selected row")
	}
	if d := geminiDocumentEfforts([]byte(rows), "gemini-9-dynamic"); d.Default == nil || *d.Default != "" {
		t.Fatal("dynamic thinking was reported as a named level")
	}
	if d := geminiDocumentEfforts([]byte(rows), "gemini-9-new"); d.Default != nil {
		t.Fatal("unlisted model inherited another row's default")
	}
	card := func(list string) []byte {
		return []byte("Model ID: `gpt-synthetic`\nReasoning effort supports " + list +
			".\n## Model details\n| Chat Completions | `v1/chat/completions` | Supported |\n## Other models\nReasoning effort supports high (default).")
	}
	for _, tc := range []struct{ list, want string }{
		{"`low`, `medium` (default), and `high`", "medium"},
		{"low, medium, and high", ""},
		{"low, high (default), medium (default)", ""},
		{"low, (default), medium and high", ""},
	} {
		d := openAIDocumentEfforts(card(tc.list), "gpt-synthetic")
		got := ""
		if d.Default != nil {
			got = *d.Default
		}
		if got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.list, got, tc.want)
		}
	}
}

func TestEffortDocumentDefaultRefreshWithCatalogLevels(t *testing.T) {
	const model = "gpt-synthetic-study"
	prefs := newFakePrefs()
	known := []discoveredModel{{id: model, efforts: []string{"low", "high"}}}
	rememberModelEfforts(prefs, providerOpenAI, known)
	if got := effortDocumentTarget(prefs, providerOpenAI, model, known); got != model {
		t.Fatalf("missing default for known options: lookup = %q", got)
	}
	v := "high"
	doc := effortDocument{Levels: []string{"high"}, Default: &v}
	doc.save(prefs, providerOpenAI, model, model)
	if got := effortDocumentTarget(prefs, providerOpenAI, model, known); got != "" {
		t.Fatalf("fresh document requested again: %q", got)
	}
	if got := modelEffortChoices(prefs, providerOpenAI, model); len(got) != 2 {
		t.Fatalf("document changed authoritative catalog options: %v", got)
	}
	if got := verifiedDefaultEffort(prefs, providerOpenAI, model, readEffortCatalog(prefs, providerOpenAI), modelEffortChoices(prefs, providerOpenAI, model)); got != v {
		t.Fatalf("current model’s explicit documented default = %q, want %q", got, v)
	}
	const alias = "gpt-synthetic-latest"
	unavailable := []discoveredModel{{id: alias, docsModel: model}, {id: model, efforts: []string{}}}
	rememberModelEfforts(prefs, providerOpenAI, unavailable)
	prefs.SetString(effortDocKey(providerOpenAI, model), "")
	if got := effortDocumentTarget(prefs, providerOpenAI, alias, unavailable); got != "" {
		t.Fatalf("explicitly unsupported target scheduled a lookup: %q", got)
	}
}

func TestProviderCatalogOwnsTheCurrentDefault(t *testing.T) {
	medium := "medium"
	opts := []string{"low", "medium", "high"}
	const model = "grok-4.5"
	prefs := newFakePrefs()
	models := []discoveredModel{{id: model, aliases: []string{"grok-synthetic-latest", "canonical"}, efforts: opts, defaultEffort: &medium}, {id: "canonical", efforts: opts}}
	rememberModelEfforts(prefs, providerGrok, models)
	c := readEffortCatalog(prefs, providerGrok)
	if got := verifiedDefaultEffort(prefs, providerGrok, model, c, opts); got != medium {
		t.Fatalf("persisted provider default lost to offline data: %q", got)
	}
	if got := verifiedDefaultEffort(prefs, providerGrok, "canonical", c, opts); got != "" {
		t.Fatalf("concrete id inherited an alias default: %q", got)
	}
	c.Checked = 0
	if got := verifiedDefaultEffort(prefs, providerGrok, "grok-synthetic-latest", c, opts); got != "" {
		t.Fatalf("stale alias default = %q", got)
	}
	c.Checked = time.Now().Unix()
	if got := verifiedDefaultEffort(prefs, providerGrok, model, c, []string{"high"}); got != "" {
		t.Fatalf("catalog default outside supported list = %q", got)
	}
}
