package bibletext

import (
	"slices"
	"strings"
)

// Grok's offline choices cover its documented text models. A model omitted
// here can still advertise its choices in the authenticated model catalogue.
// https://docs.x.ai/developers/models/grok-4.3
// https://docs.x.ai/developers/model-capabilities/text/reasoning
func grokEfforts(model string) []string {
	switch strings.TrimSuffix(model, "-latest") {
	case "grok-4.3":
		return []string{"none", "low", "medium", "high"}
	case "grok-4.5":
		return []string{"low", "medium", "high"}
	case "grok-4.6", "grok-4.7":
		return []string{"low", "medium", "high", "xhigh"}
	}
	return nil
}

// Some catalogue entries accept an effort value without providing a distinct
// reasoning depth. Multi-agent effort selects a team size, and older Grok
// versions silently run xhigh as high.
func grokCatalogEfforts(model string, levels []string) []string {
	if strings.Contains(model, "-multi-agent") || strings.Contains(model, "-non-reasoning") ||
		strings.HasPrefix(model, "grok-voice-") || strings.HasPrefix(model, "grok-imagine-") {
		return []string{}
	}
	switch strings.TrimSuffix(model, "-latest") {
	case "grok-4.3", "grok-4.5":
		if slices.Contains(levels, "xhigh") {
			return slices.DeleteFunc(slices.Clone(levels), func(level string) bool { return level == "xhigh" })
		}
	}
	return levels
}
