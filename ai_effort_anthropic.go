package bibletext

import (
	"encoding/json"
	"slices"
)

// Anthropic's Messages API accepts named effort through output_config. The
// older manual-thinking models take a token budget instead. These offline
// defaults are superseded by the account's model capabilities when available.
// https://platform.claude.com/docs/en/build-with-claude/effort
// https://platform.claude.com/docs/en/build-with-claude/extended-thinking
func anthropicEfforts(model string) []string {
	switch model {
	case "claude-fable-5-2", "claude-fable-5-1", "claude-fable-5",
		"claude-mythos-5-2", "claude-mythos-5-1", "claude-mythos-5",
		"claude-opus-5-5", "claude-opus-5", "claude-opus-4-8", "claude-opus-4-7",
		"claude-sonnet-5-5", "claude-sonnet-5", "claude-haiku-5-5":
		return []string{"low", "medium", "high", "xhigh", "max"}
	case "claude-opus-4-6", "claude-sonnet-4-6", "claude-mythos-preview":
		return []string{"low", "medium", "high", "max"}
	case "claude-opus-4-5", "claude-opus-4-5-20251101":
		return []string{"low", "medium", "high"}
	case "claude-haiku-4-5", "claude-haiku-4-5-20251001",
		"claude-sonnet-4-5", "claude-sonnet-4-5-20250929",
		"claude-opus-4-1", "claude-opus-4-1-20250805",
		"claude-opus-4-0", "claude-opus-4-20250514",
		"claude-sonnet-4-0", "claude-sonnet-4-20250514",
		"claude-3-7-sonnet-latest", "claude-3-7-sonnet-20250219":
		return anthropicManualBudgets()
	}
	return nil
}

func anthropicManualBudgets() []string {
	return []string{"1024", "4096", "8192"}
}

type anthropicCapabilitySupport struct {
	Supported bool `json:"supported"`
}

type anthropicModelCapabilities struct {
	Effort   map[string]json.RawMessage `json:"effort"`
	Thinking *struct {
		Types struct {
			Enabled *anthropicCapabilitySupport `json:"enabled"`
		} `json:"types"`
	} `json:"thinking"`
}

// Missing metadata leaves the documented offline choices available. An
// explicitly unsupported effort and manual mode yields no adjustable choices.
func (c *anthropicModelCapabilities) effortChoices(model string) []string {
	if c == nil {
		return nil
	}
	var named *bool
	if json.Unmarshal(c.Effort["supported"], &named) != nil || named == nil {
		return nil
	}
	if !*named {
		if c.Thinking != nil && c.Thinking.Types.Enabled != nil {
			if c.Thinking.Types.Enabled.Supported {
				return anthropicManualBudgets()
			}
		} else if slices.Contains(anthropicEfforts(model), "1024") {
			return anthropicManualBudgets()
		}
		return []string{}
	}
	// Read the per-level flags, including levels introduced after this build.
	// Their keys are the string values the Messages API accepts.
	var offered []string
	for name, raw := range c.Effort {
		var option anthropicCapabilitySupport
		if len(name) > 0 && name[0] >= 'a' && name[0] <= 'z' && json.Unmarshal(raw, &option) == nil && option.Supported {
			offered = append(offered, name)
		}
	}
	choices := []string{}
	for _, level := range []string{"low", "medium", "high", "xhigh", "max"} {
		if i := slices.Index(offered, level); i >= 0 {
			choices = append(choices, level)
			offered = slices.Delete(offered, i, i+1)
		}
	}
	slices.Sort(offered)
	choices = append(choices, offered...)
	return choices
}
