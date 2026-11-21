package bibletext

import (
	"slices"
	"strings"
	"testing"
)

func TestOpenAIOfflineReasoningChoices(t *testing.T) {
	for _, tc := range []struct{ models, choices string }{
		{"gpt-5 gpt-5-mini-2025-08-07 gpt-5-nano", "minimal low medium high"},
		{"gpt-5.1 gpt-5.1-2025-11-13", "none low medium high"},
		{"gpt-5.2-2025-12-11 gpt-5.4 gpt-5.4-mini-2026-03-17 gpt-5.4-nano gpt-5.5-2026-04-23", "none low medium high xhigh"},
		{"gpt-5.6-luna gpt-5.6-terra gpt-5.6-sol gpt-6-sol gpt-6-luna gpt-6.1-terra", "none low medium high xhigh max"},
		{"gpt-6-astra gpt-6.1-sol gpt-6.2-astra gpt-6.2-sol", "low medium high xhigh max"},
		{"o3 o3-2025-04-16", "low medium high"},
		{"gpt-5.4-pro gpt-5.3-codex gpt-5.3-chat-latest o3-pro o3-mini o4-mini gpt-6.2-sol-synthetic gpt-5.4-mini-2099-01-01", ""},
	} {
		for _, id := range strings.Fields(tc.models) {
			if got := openAIEfforts(id); !slices.Equal(got, strings.Fields(tc.choices)) {
				t.Errorf("%s: offered %v, want %q", id, got, tc.choices)
			}
		}
	}
}
