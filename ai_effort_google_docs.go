package bibletext

import (
	"slices"
	"strings"
)

// Google publishes the supported levels in a Markdown table. Only the
// exact model's levels column describes usable choices. The default column
// and the surrounding prose also name levels used by other models.
// https://aistudio.google.com/docs/thinking.md
func geminiDocumentEfforts(body []byte, model string) []string {
	if !strings.HasPrefix(model, "gemini-") || strings.HasPrefix(model, "gemini-2.") || strings.HasPrefix(model, "gemini-1.") ||
		containsAnySubstr(model, modelExcludeSubstrings) || containsAnySubstr(model, []string{"-live", "-omni-", "-translate"}) {
		// The Interactions table lists symbolic levels even for Gemini 2.5,
		// where this app's generateContent endpoint takes a numeric budget.
		return nil
	}
	section, table := false, false
	for line := range strings.SplitSeq(string(body), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "## ") {
			section = strings.Contains(line, `id="thinking-levels"`) || line == "## Controlling thinking"
			table = false
		}
		if !section || !strings.HasPrefix(line, "|") {
			table = false
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		if len(cells) != 3 {
			table = false
			continue
		}
		for i := range cells {
			cells[i] = strings.Trim(strings.TrimSpace(cells[i]), "`")
		}
		if cells[0] == "Model" && cells[1] == "Default Thinking" && cells[2] == "Levels Supported" {
			table = true
			continue
		}
		if !table || cells[0] != model {
			continue
		}
		var levels []string
		for _, token := range strings.Split(cells[2], ",") {
			value := strings.ToLower(strings.Trim(strings.TrimSpace(token), "`"))
			switch value {
			case "minimal", "low", "medium", "high", "xhigh":
				if !slices.Contains(levels, value) {
					levels = append(levels, value)
				}
			}
		}
		return levels
	}
	return nil
}
