package bibletext

import (
	"slices"
	"testing"
)

func TestGeminiDocumentEffortsMatchOnlyPublishedModelLevels(t *testing.T) {
	rows := []byte("## Previous section\n" +
		"| Model | Default Thinking | Levels Supported |\n" +
		"| gemini-4-carbon | On | minimal |\n" +
		"## Controlling thinking  {% id=\"thinking-levels\" %}\n" +
		"| Model | Default Thinking | Levels Supported |\n" +
		"| --- | --- | --- |\n" +
		"| gemini-4-carbon | On (medium) | low, medium, high |\n" +
		"| `gemini-9-synthetic` | On (medium) | `LOW`, `HIGH`, experimental, HIGH |\n" +
		"| gemini-9-synthetic-image | On | low, high |\n" +
		"| gemini-9-synthetic-live | On | low, high |\n" +
		"| gemini-2.5-flash | On | low, medium, high |\n" +
		"### Understanding thinking levels\n" +
		"| Model | Explanation | Levels Supported |\n" +
		"| gemini-9-unlisted | On | low, high |\n")
	for _, tc := range []struct {
		model string
		want  []string
	}{
		{"gemini-4-carbon", []string{"low", "medium", "high"}},
		{"gemini-9-synthetic", []string{"low", "high"}},
		{"gemini-9-synth", nil},
		{"gemini-9-unlisted", nil},
		{"gemini-9-synthetic-image", nil},
		{"gemini-9-synthetic-live", nil},
		{"gemini-2.5-flash", nil},
	} {
		if got := geminiDocumentEfforts(rows, tc.model); !slices.Equal(got, tc.want) {
			t.Errorf("%s: levels %v, want %v", tc.model, got, tc.want)
		}
	}
}
