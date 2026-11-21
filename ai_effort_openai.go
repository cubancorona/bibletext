package bibletext

// Offline choices for OpenAI models whose Chat Completions support and effort
// levels are documented. Snapshot IDs are separate models in the API. A new
// variant can differ from a similarly named model even in the same generation.
// https://developers.openai.com/api/docs/models
// https://developers.openai.com/api/docs/guides/reasoning
// https://developers.openai.com/api/docs/guides/deployment-checklist
func openAIEfforts(model string) []string {
	switch model {
	case "gpt-5", "gpt-5-2025-08-07", "gpt-5-mini", "gpt-5-mini-2025-08-07",
		"gpt-5-nano", "gpt-5-nano-2025-08-07":
		return []string{"minimal", "low", "medium", "high"}
	case "gpt-5.1", "gpt-5.1-2025-11-13":
		return []string{"none", "low", "medium", "high"}
	case "gpt-5.2", "gpt-5.2-2025-12-11", "gpt-5.4", "gpt-5.4-2026-03-05",
		"gpt-5.4-mini", "gpt-5.4-mini-2026-03-17", "gpt-5.4-nano", "gpt-5.4-nano-2026-03-17",
		"gpt-5.5", "gpt-5.5-2026-04-23":
		return []string{"none", "low", "medium", "high", "xhigh"}
	case "gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.6-luna",
		"gpt-6-sol", "gpt-6-luna", "gpt-6.1-terra":
		return []string{"none", "low", "medium", "high", "xhigh", "max"}
	case "gpt-6-astra", "gpt-6.1-sol", "gpt-6.2-astra", "gpt-6.2-sol":
		return []string{"low", "medium", "high", "xhigh", "max"}
	case "o3", "o3-2025-04-16":
		return []string{"low", "medium", "high"}
	}
	return nil
}
