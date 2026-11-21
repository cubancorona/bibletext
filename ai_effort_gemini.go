package bibletext

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// generateContent accepts named levels on Gemini 3 and 4. Gemini 2.5
// accepts token budgets instead. Only the Flash editions accept zero.
// https://aistudio.google.com/docs/thinking#thinking-levels
// https://docs.cloud.google.com/gemini-enterprise-agent-platform/models/thinking
func geminiEfforts(model string) []string {
	return geminiEffortsAt(model, time.Now())
}

func geminiEffortsAt(model string, now time.Time) []string {
	// Google forwards these two names to Gemini 3 models on December 1
	// 2026. Their named levels replace the integer budgets for Gemini 2.5.
	// https://aistudio.google.com/docs/deprecations#gemini-2.5-pro-models
	if !now.Before(time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)) {
		switch model {
		case "gemini-2.5-pro":
			model = "gemini-3.8-flash"
		case "gemini-2.5-flash":
			model = "gemini-3.5-flash-lite"
		}
	}
	switch model {
	case "gemini-4-neon", "gemini-4-hydrogen":
		return []string{"low", "medium", "high", "xhigh"}
	case "gemini-4-carbon", "gemini-4-argon", "gemini-4-carbon-cyber-permissive",
		"gemini-4-argon-cyber-permissive", "gemini-3.8-flash", "gemini-3.8-flash-cyber",
		"gemini-3.7-flash", "gemini-3.1-pro-preview", "gemini-3.1-pro-preview-customtools":
		return []string{"low", "medium", "high"}
	case "gemini-3.6-flash", "gemini-3.5-flash", "gemini-3.5-flash-lite",
		"gemini-3.1-flash-lite", "gemini-3-flash-preview", "gemini-robotics-er-2-preview":
		return []string{"minimal", "low", "medium", "high"}
	case "gemini-3-pro-preview":
		return []string{"low", "high"}
	case "gemini-2.5-flash", "gemini-2.5-flash-lite":
		return []string{"0", "1024", "8192", "24576"}
	case "gemini-2.5-pro":
		// The answer shares the output-token allowance with thinking.
		// Leave room for visible text below the app's 32,768-token cap.
		return []string{"1024", "8192", "24576"}
	}
	return nil
}

// Gemini's list normally reports a thinking flag, not the model's levels.
// A concrete version or baseModelId can identify the model behind a rolling
// alias. A generic version number or true flag alone cannot do so.
// https://aistudio.google.com/api-reference/models#Resource:Model
type geminiCatalogModel struct {
	Name     string   `json:"name"`
	Base     string   `json:"baseModelId"`
	Version  string   `json:"version"`
	Methods  []string `json:"supportedGenerationMethods"`
	Thinking *bool    `json:"thinking"`
	Levels   []string `json:"supportedThinkingLevels"`
}

var geminiVersionedModelID = regexp.MustCompile(`^gemini-[0-9]+(\.[0-9]+)*-[a-z0-9]+(-[a-z0-9]+)*$`)
var geminiLevelID = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)

// A version may name a newer model than the base ID. If that model is new
// to BibleText, consult its own documentation instead of inheriting the
// base model's choices.
func (m geminiCatalogModel) documentModel() string {
	name := strings.TrimPrefix(m.Name, "models/")
	for _, value := range []string{m.Version, m.Base} {
		id := strings.TrimPrefix(value, "models/")
		if !strings.HasSuffix(id, "-latest") && geminiVersionedModelID.MatchString(id) {
			if id == name {
				return ""
			}
			return id
		}
	}
	return ""
}

func (m geminiCatalogModel) efforts() []string {
	if m.Thinking != nil && !*m.Thinking {
		return []string{}
	}
	if m.Levels != nil {
		result := []string{}
		for _, level := range m.Levels {
			value := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(level)), "thinking_level_")
			if value != "unspecified" && geminiLevelID.MatchString(value) && !slices.Contains(result, value) {
				result = append(result, value)
			}
		}
		return result
	}
	return nil
}

func geminiThinkingOptions(effort string) *geminiThinkingConfig {
	if effort == "" {
		return nil
	}
	if budget, err := strconv.Atoi(effort); err == nil {
		return &geminiThinkingConfig{ThinkingBudget: &budget}
	}
	return &geminiThinkingConfig{ThinkingLevel: strings.ToUpper(effort)}
}
