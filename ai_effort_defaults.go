package bibletext

import (
	"slices"
	"strconv"
	"time"
)

// Lowering effort changes only the next request. Follow documented level
// ordering or the numeric budget, even if a provider lists choices unsorted.
// Unknown names have no ordering, and disabling thinking is an explicit choice.
func lowerReasoningEffort(current string, levels []string) string {
	if current == "" || !slices.Contains(levels, current) {
		return ""
	}
	order := []string{"minimal", "low", "medium", "high", "xhigh", "max"}
	for i := slices.Index(order, current) - 1; i >= 0; i-- {
		if slices.Contains(levels, order[i]) {
			return order[i]
		}
	}
	budget, err := strconv.Atoi(current)
	if err != nil || budget <= 0 || strconv.Itoa(budget) != current {
		return ""
	}
	best := 0
	for _, level := range levels {
		n, err := strconv.Atoi(level)
		if err == nil && n > best && n < budget && strconv.Itoa(n) == level {
			best = n
		}
	}
	if best == 0 {
		return ""
	}
	return strconv.Itoa(best)
}

func freshEffortEvidence(checked int64) bool {
	age := time.Since(time.Unix(checked, 0))
	return age >= 0 && age < effortDocAge
}

// A published default describes an omitted setting. Only use evidence for
// the exact model serving the request and a value the model still supports.
func verifiedDefaultEffort(prefs prefStore, provider, model string, catalog modelEffortCatalog, levels []string) string {
	target := effectiveEffortTarget(prefs, provider, model, catalog)
	if target != model && catalog.Targets[model] == target {
		recent := ""
		if provider == providerGemini {
			recent = recentGeminiResponseTarget(prefs, model)
		}
		if recent != target && (recent != "" || !freshEffortEvidence(catalog.Checked)) {
			return ""
		}
	}
	value, reported := catalog.Defaults[model]
	if !reported {
		value, reported = catalog.Defaults[target]
	}
	if reported {
		if freshEffortEvidence(catalog.Checked) && slices.Contains(levels, value) {
			return value
		}
		return ""
	}
	d := savedEffortDocument(prefs, provider, target)
	if d.Model == target && d.Default != nil {
		if !freshEffortEvidence(d.Checked) {
			return ""
		}
		value = *d.Default
	} else {
		value = documentedEffortDefault(provider, target)
	}
	if slices.Contains(levels, value) {
		return value
	}
	return ""
}

// These exact model IDs have published defaults. A fresh model document
// can replace the bundled value. Rolling aliases require separate evidence.
func documentedEffortDefault(provider, model string) string {
	switch provider {
	case providerGemini:
		// https://aistudio.google.com/docs/thinking#thinking-levels
		switch model {
		case "gemini-4-carbon", "gemini-4-argon", "gemini-3.8-flash", "gemini-3.6-flash":
			return "medium"
		case "gemini-4-neon", "gemini-4-hydrogen", "gemini-3.1-pro-preview", "gemini-3-pro-preview", "gemini-3-flash-preview", "gemini-robotics-er-2-preview":
			return "high"
		case "gemini-3.5-flash-lite", "gemini-3.1-flash-lite":
			return "minimal"
		}
	case providerOpenAI:
		// https://developers.openai.com/api/docs/guides/latest-model?model=gpt-5
		// https://developers.openai.com/api/docs/guides/latest-model?model=gpt-5.1
		// https://developers.openai.com/api/docs/models/gpt-5.4.md
		switch model {
		case "gpt-5", "gpt-5-mini", "gpt-5-nano", "gpt-5-2025-08-07", "gpt-5-mini-2025-08-07", "gpt-5-nano-2025-08-07":
			return "medium"
		case "gpt-5.1", "gpt-5.1-2025-11-13", "gpt-5.2", "gpt-5.2-2025-12-11", "gpt-5.4", "gpt-5.4-2026-03-05":
			return "none"
		}
	case providerAnthropic:
		// https://platform.claude.com/docs/en/build-with-claude/effort
		switch model {
		case "claude-opus-5-5", "claude-haiku-5-5":
			return "medium"
		case "claude-fable-5-2", "claude-fable-5-1", "claude-fable-5", "claude-mythos-5-2", "claude-mythos-5-1", "claude-mythos-5",
			"claude-opus-5", "claude-opus-4-8", "claude-opus-4-7", "claude-opus-4-6", "claude-opus-4-5", "claude-opus-4-5-20251101",
			"claude-sonnet-5-5", "claude-sonnet-5", "claude-sonnet-4-6", "claude-mythos-preview":
			return "high"
		}
	case providerGrok:
		// https://docs.x.ai/developers/model-capabilities/text/reasoning
		// https://docs.x.ai/developers/models/grok-4.3
		switch model {
		case "grok-4.5", "grok-4.6", "grok-4.7":
			return "high"
		case "grok-4.3":
			return "low"
		}
	}
	return ""
}
