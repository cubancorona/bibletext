package bibletext

import (
	"slices"
	"strings"
	"testing"
)

func TestOpenAIDocumentEfforts(t *testing.T) {
	// These fixtures retain the provider's Markdown headings, identity fields
	// and endpoint row. Only a model's own description can supply its choices.
	card := func(model, description, status, snapshot string) string {
		return "# Model\n\nModel ID: `" + model + "`\n" + description +
			"\n## Model details\n- Default snapshot: `" + snapshot + "`" +
			"\n## Endpoints\n| Endpoint | Route | Support |\n| --- | --- | --- |" +
			"\n| Chat Completions | `v1/chat/completions` | " + status + " |" +
			"\n| Responses | `v1/responses` | Supported |\n" +
			"\n## Snapshots\n- `" + snapshot + "`\n"
	}
	const current = "gpt-5.2"
	const dated = "gpt-5.2-2025-12-11"
	const future = "gpt-synthetic-study"
	const futureDated = "gpt-synthetic-study-2029-01-31"
	currentCard := card(current, "Reasoning.effort supports: none (default), low, medium, high and xhigh.", "Supported", dated)
	futureCard := card(future, "`reasoning_effort` supports `low`, `high`, and `ultra`. Its latency tier may also be medium.", "Supported", futureDated)
	bareCard := card(future, "This model performs reasoning tasks.", "Supported", futureDated)
	testCases := []struct {
		name  string
		model string
		card  string
		want  []string
	}{
		{"published format", current, currentCard, []string{"none", "low", "medium", "high", "xhigh"}},
		{"listed dated snapshot", dated, currentCard, []string{"none", "low", "medium", "high", "xhigh"}},
		{"new model and supported list", future, futureCard, []string{"low", "high", "ultra"}},
		{"wrapped after comma", "gpt-6.1-terra", card("gpt-6.1-terra", "`reasoning.effort` supports `none`, `low`, `medium` (default), `high`,\n`xhigh`, and `max`.", "Supported", ""), []string{"none", "low", "medium", "high", "xhigh", "max"}},
		{"wrapped after conjunction", "gpt-6.1-sol", card("gpt-6.1-sol", "`reasoning.effort` supports `low`, `medium` (default), `high`, `xhigh`, and\n`max`.", "Supported", ""), []string{"low", "medium", "high", "xhigh", "max"}},
		{"truncated after comma", future, card(future, "Reasoning effort supports low, high,\n\nMax context is a separate setting.", "Supported", futureDated), nil},
		{"truncated after conjunction", future, card(future, "Reasoning effort supports low, high, and.", "Supported", futureDated), nil},
		{"wrong identity", future, currentCard, nil},
		{"recommendation mentions wrong identity", future, currentCard + "\nAlso consider `" + future + "`.\n", nil},
		{"unlisted date", "gpt-5.2-2025-12-12", currentCard, nil},
		{"no Chat Completions support", future, strings.Replace(futureCard, "`v1/chat/completions` | Supported", "`v1/chat/completions` | Not supported", 1), nil},
		{"unparseable prose", future, card(future, "Reasoning effort supports both routine and complex tasks.", "Supported", futureDated), nil},
		{"no explicit levels", future, bareCard, nil},
		{"unrelated section", future, bareCard + "\n## Other models\nReasoning effort supports low and high.\n", nil},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := openAIDocumentEfforts([]byte(tc.card), tc.model).Levels
			if (got == nil) != (tc.want == nil) || !slices.Equal(got, tc.want) {
				t.Fatalf("levels = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestEffortDocumentFollowsCurrentGeminiAlias(t *testing.T) {
	const alias = "gemini-synthetic-latest"
	const old = "gemini-9-synthetic"
	const current = "gemini-10-synthetic"
	prefs := newFakePrefs()
	rememberModelEfforts(prefs, providerGemini, []discoveredModel{{id: alias, docsModel: old}})
	effortDocument{Levels: []string{"low", "high"}}.save(prefs, providerGemini, alias, old)
	setAIReasoningEffort(prefs, providerGemini, alias, "high")
	if got := aiReasoningEffort(prefs, providerGemini, alias); got != "high" {
		t.Fatalf("current documented effort = %q", got)
	}
	rememberModelEfforts(prefs, providerGemini, []discoveredModel{{id: alias, docsModel: current}})
	if got := aiReasoningEffort(prefs, providerGemini, alias); got != "" {
		t.Fatalf("previous model's effort remains active after alias changed: %q", got)
	}
	effortDocument{Levels: []string{"low", "medium"}}.save(prefs, providerGemini, alias, current)
	// A slower lookup started before the catalog changed.
	effortDocument{Levels: []string{"low", "high"}}.save(prefs, providerGemini, alias, old)
	if got := modelEffortChoices(prefs, providerGemini, alias); !slices.Equal(got, []string{"low", "medium"}) {
		t.Fatalf("late document replaced current choices: %v", got)
	}
}
