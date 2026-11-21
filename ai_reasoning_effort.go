package bibletext

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"
)

// aiReasoningEfforts supplies offline choices for documented model versions.
// A model list can supersede these choices even for a previously unknown ID.
func aiReasoningEfforts(provider, model string) []string {
	switch provider {
	case providerGemini:
		return geminiEfforts(model)
	case providerOpenAI:
		return openAIEfforts(model)
	case providerAnthropic:
		return anthropicEfforts(model)
	case providerGrok:
		return grokEfforts(model)
	}
	return nil
}

func modelEffortCatalogKey(provider string) string { return "ai.modelEffortCatalog." + provider }

type modelEffortCatalog struct {
	Levels  map[string][]string
	Targets map[string]string
}

func readEffortCatalog(prefs prefStore, provider string) (c modelEffortCatalog) {
	if prefs != nil {
		_ = json.Unmarshal([]byte(prefs.String(modelEffortCatalogKey(provider))), &c)
	}
	return c
}

func rememberModelEfforts(prefs prefStore, provider string, models []discoveredModel) {
	if prefs == nil {
		return
	}
	encoded, err := json.Marshal(effortCatalogFromModels(provider, models))
	if err == nil && prefs.String(modelEffortCatalogKey(provider)) != string(encoded) {
		prefs.SetString(modelEffortCatalogKey(provider), string(encoded))
	}
}

func effortCatalogFromModels(provider string, models []discoveredModel) modelEffortCatalog {
	c := modelEffortCatalog{map[string][]string{}, map[string]string{}}
	// Canonical entries take priority over another entry's aliases.
	for pass := 0; pass < 2; pass++ {
		for _, m := range models {
			if m.id == "" {
				continue
			}
			ids := m.aliases
			if pass == 1 {
				ids = []string{m.id}
			}
			target := m.docsModel
			if target == "" {
				target = m.id
			}
			var levels []string
			if m.efforts != nil {
				levels = checkedEffortValues(m.efforts)
				if provider == providerGrok {
					levels = grokCatalogEfforts(m.id, levels)
				}
			}
			for _, id := range ids {
				if id == "" {
					continue
				}
				if provider == providerGemini || provider == providerOpenAI {
					c.Targets[id] = target
				}
				if levels != nil {
					if provider == providerGrok {
						c.Levels[id] = grokCatalogEfforts(id, levels)
					} else {
						c.Levels[id] = levels
					}
				}
			}
		}
	}
	return c
}

// Preserve future provider-defined levels when their values are simple tokens.
// Numeric token budgets have a separate wire representation and UI label.
func checkedEffortValues(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.ToLower(strings.TrimSpace(v))
		if v == "" || len(v) > 32 || slices.Contains(out, v) {
			continue
		}
		valid := true
		for _, c := range v {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
				valid = false
				break
			}
		}
		if valid {
			out = append(out, v)
		}
	}
	return out
}

func effectiveEffortTarget(prefs prefStore, provider, model string, catalog modelEffortCatalog) string {
	if target := catalog.Targets[model]; target != "" && target != model {
		return target
	}
	if provider == providerGemini {
		if target := recentGeminiResponseTarget(prefs, model); target != "" {
			return target
		}
	}
	return model
}

func modelEffortChoices(prefs prefStore, provider, model string) []string {
	return readEffortCatalog(prefs, provider).choices(prefs, provider, model)
}

func (c modelEffortCatalog) choices(prefs prefStore, provider, model string) []string {
	if levels, found := c.Levels[model]; found {
		return levels
	}
	target := effectiveEffortTarget(prefs, provider, model, c)
	if levels, found := c.Levels[target]; found {
		return levels
	}
	if doc := savedEffortDocument(prefs, provider, target); doc.Levels != nil && doc.Model == target {
		return doc.Levels
	}
	return aiReasoningEfforts(provider, target)
}

func reasoningEffortKey(provider, model string) string {
	return "ai.reasoningEffort." + provider + "." + model
}

func aiReasoningEffort(prefs prefStore, provider, model string) string {
	return readEffortCatalog(prefs, provider).selected(prefs, provider, model)
}

func (c modelEffortCatalog) selected(prefs prefStore, provider, model string) string {
	if prefs == nil {
		return ""
	}
	effort := prefs.String(reasoningEffortKey(provider, model))
	if effort != "" && slices.Contains(c.choices(prefs, provider, model), effort) {
		return effort
	}
	return ""
}

func setAIReasoningEffort(prefs prefStore, provider, model, effort string) {
	if prefs != nil && (effort == "" || slices.Contains(modelEffortChoices(prefs, provider, model), effort)) {
		prefs.SetString(reasoningEffortKey(provider, model), effort)
	}
}

func effortSettingLabel(levels []string) string {
	if len(levels) > 0 {
		if _, err := strconv.Atoi(levels[0]); err == nil {
			return "Thinking budget"
		}
	}
	return "Reasoning effort"
}

func effortDisplayName(value string) string {
	switch value {
	case "":
		return "Recommended"
	case "xhigh":
		return "Extra high"
	case "0":
		return "Off"
	}
	if n, err := strconv.Atoi(value); err == nil && n > 0 {
		digits := strconv.Itoa(n)
		for i := len(digits) - 3; i > 0; i -= 3 {
			digits = digits[:i] + "," + digits[i:]
		}
		return digits + " tokens"
	}
	value = strings.NewReplacer("_", " ", "-", " ").Replace(value)
	return strings.ToUpper(value[:1]) + value[1:]
}
